// Package service supervises one bundled server without using upstream internals.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"cliproxyapi-desktop/internal/settings"
	"cliproxyapi-desktop/internal/supervisor"
)

type State string

const (
	Stopped  State = "Stopped"
	Starting State = "Starting"
	Running  State = "Running"
	Stopping State = "Stopping"
	Failed   State = "Failed"
)

type Snapshot struct {
	State          State
	URL, Error     string
	PID            int
	PanelAvailable bool
}

type Manager struct {
	paths     settings.Paths
	operation sync.Mutex
	mu        sync.Mutex
	snapshot  Snapshot
	control   io.WriteCloser
	done      chan struct{}
	expected  bool
	changes   chan Snapshot
}

func New(p settings.Paths) *Manager {
	return &Manager{paths: p, snapshot: Snapshot{State: Stopped}, changes: make(chan Snapshot, 1)}
}
func (m *Manager) Changes() <-chan Snapshot { return m.changes }
func (m *Manager) Snapshot() Snapshot       { m.mu.Lock(); defer m.mu.Unlock(); return m.snapshot }
func (m *Manager) publishLocked() {
	select {
	case <-m.changes:
	default:
	}
	m.changes <- m.snapshot
}
func (m *Manager) fail(err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshot.State, m.snapshot.Error = Failed, err.Error()
	m.publishLocked()
	return err
}

func (m *Manager) Start(ctx context.Context) error {
	m.operation.Lock()
	defer m.operation.Unlock()
	m.mu.Lock()
	if m.done != nil {
		m.mu.Unlock()
		return nil
	}
	m.snapshot = Snapshot{State: Starting}
	m.publishLocked()
	m.mu.Unlock()
	listener, err := settings.Read(m.paths.Config)
	if err != nil {
		return m.fail(err)
	}
	// Refuse an occupied port before launching. Never adopt an existing server.
	probe, err := net.Listen("tcp", listener.Address())
	if err != nil {
		return m.fail(fmt.Errorf("server address %s is unavailable: %w", listener.Address(), err))
	}
	if err := probe.Close(); err != nil {
		return m.fail(err)
	}
	logPath := filepath.Join(m.paths.Logs, "server.log")
	if info, err := os.Stat(logPath); err == nil && info.Size() > 10<<20 {
		if err := os.Remove(logPath + ".1"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return m.fail(err)
		}
		if err := os.Rename(logPath, logPath+".1"); err != nil {
			return m.fail(err)
		}
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return m.fail(err)
	}
	cmd := exec.Command(m.paths.Supervisor, "--core", m.paths.Core, "--config", m.paths.Config,
		"--data", m.paths.Data, "--static", m.paths.Static, "--log", logPath)
	cmd.Dir, cmd.Stderr = m.paths.Data, log
	configureSupervisor(cmd)
	control, err := cmd.StdinPipe()
	if err != nil {
		_ = log.Close()
		return m.fail(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		_ = control.Close()
		_ = log.Close()
		return m.fail(err)
	}
	if err := cmd.Start(); err != nil {
		_ = output.Close()
		_ = control.Close()
		_ = log.Close()
		return m.fail(fmt.Errorf("launch supervisor: %w", err))
	}
	done := make(chan struct{})
	events := make(chan supervisor.Event, 4)
	m.mu.Lock()
	m.control, m.done, m.expected = control, done, false
	m.snapshot.URL, m.snapshot.PanelAvailable = listener.BaseURL(), !listener.PanelDisabled
	m.mu.Unlock()
	go m.monitor(cmd, output, log, control, done, events)
	// Startup probes are local control-plane checks, never proxy request timeouts.
	startCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	select {
	case event := <-events:
		if event.Type != "started" {
			err = fmt.Errorf("server startup: %s", event.Error)
		}
	case <-done:
		err = fmt.Errorf("server exited before startup; see %s", logPath)
	case <-startCtx.Done():
		err = startCtx.Err()
	}
	if err == nil {
		err = waitReady(startCtx, listener.BaseURL(), done)
	}
	if err != nil {
		_ = m.stopLocked()
		return m.fail(fmt.Errorf("server did not become ready: %w (logs: %s)", err, logPath))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.done != done {
		return fmt.Errorf("server exited during startup; see %s", logPath)
	}
	m.snapshot.State, m.snapshot.Error = Running, ""
	m.publishLocked()
	return nil
}

func (m *Manager) monitor(cmd *exec.Cmd, output io.ReadCloser, log *os.File, control io.WriteCloser, done chan struct{}, events chan supervisor.Event) {
	decoder := json.NewDecoder(output)
	message := ""
	for {
		var event supervisor.Event
		if err := decoder.Decode(&event); err != nil {
			break
		}
		if event.Type == "error" {
			message = event.Error
		}
		if event.Type == "started" {
			m.mu.Lock()
			m.snapshot.PID = event.PID
			m.mu.Unlock()
		}
		select {
		case events <- event:
		default:
		}
	}
	err := cmd.Wait()
	_ = control.Close()
	_ = log.Close()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.done == done {
		m.done, m.control = nil, nil
		m.snapshot.PID = 0
		if m.expected {
			m.snapshot.State, m.snapshot.Error = Stopped, ""
		} else {
			if message == "" {
				message = "Server exited unexpectedly; open the logs folder for details."
				if err != nil {
					message += " " + err.Error()
				}
			}
			m.snapshot.State, m.snapshot.Error = Failed, message
		}
		m.publishLocked()
	}
	close(done)
}

func (m *Manager) Stop() error { m.operation.Lock(); defer m.operation.Unlock(); return m.stopLocked() }
func (m *Manager) stopLocked() error {
	m.mu.Lock()
	done, control := m.done, m.control
	if done == nil {
		m.mu.Unlock()
		return nil
	}
	m.expected = true
	m.snapshot.State = Stopping
	m.publishLocked()
	m.mu.Unlock()
	// EOF also requests shutdown and handles an unexpected launcher exit.
	if control != nil {
		_ = control.Close()
	}
	<-done
	return nil
}

func waitReady(ctx context.Context, base string, done <-chan struct{}) error {
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/healthz", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				select {
				case <-done:
					return fmt.Errorf("server exited during startup")
				default:
					return nil
				}
			}
		}
		select {
		case <-done:
			return fmt.Errorf("server exited before becoming ready")
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
