// Package supervisor owns the official server process and stops it on parent EOF.
package supervisor

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Event struct {
	Type  string `json:"type"`
	PID   int    `json:"pid,omitempty"`
	Error string `json:"error,omitempty"`
}

type Options struct{ Core, Config, Data, Static, Log string }

func Run(o Options, input io.Reader, output io.Writer) (errResult error) {
	log, err := os.OpenFile(o.Log, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open server log: %w", err)
	}
	defer func() {
		if err := log.Close(); errResult == nil {
			errResult = err
		}
	}()
	cmd := exec.Command(o.Core, "--config", o.Config)
	cmd.Dir = o.Data
	cmd.Env = ChildEnvironment(os.Environ(), o.Data, o.Static)
	cmd.Stdout, cmd.Stderr = log, log
	configureChild(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start server: %w", err)
	}
	cleanup, err := containChild(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("contain server process: %w", err)
	}
	defer cleanup()
	encoder := json.NewEncoder(output)
	if err := encoder.Encode(Event{Type: "started", PID: cmd.Process.Pid}); err != nil {
		_ = killChild(cmd)
		_ = cmd.Wait()
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stop := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == "stop" {
				break
			}
		}
		close(stop)
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case err := <-done:
		message := "server exited unexpectedly"
		if err != nil {
			message += ": " + err.Error()
		}
		_ = encoder.Encode(Event{Type: "error", Error: message})
		return errors.New(message)
	case <-stop:
	case <-signals:
	}
	if err := interruptChild(cmd); err != nil && !errors.Is(err, os.ErrProcessDone) {
		_, _ = fmt.Fprintf(log, "desktop supervisor: interrupt failed: %v\n", err)
	}
	// This bounds local process cleanup, not upstream network requests.
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		_, _ = fmt.Fprintln(log, "desktop supervisor: forcing server termination after shutdown grace period")
		_ = killChild(cmd)
		<-done
	}
	return encoder.Encode(Event{Type: "stopped"})
}

// A desktop instance always uses its explicit local config and data directory.
// Shell deployment credentials must not silently select a different config store.
func ChildEnvironment(in []string, data, static string) []string {
	out := make([]string, 0, len(in)+2)
	for _, item := range in {
		name, _, _ := strings.Cut(item, "=")
		name = strings.ToUpper(name)
		if name == "WRITABLE_PATH" || name == "MANAGEMENT_STATIC_PATH" || name == "HOME_JWT" || name == "MANAGEMENT_PASSWORD" || name == "DEPLOY" || strings.HasPrefix(name, "PGSTORE_") || strings.HasPrefix(name, "GITSTORE_") || strings.HasPrefix(name, "OBJECTSTORE_") {
			continue
		}
		out = append(out, item)
	}
	return append(out, "WRITABLE_PATH="+data, "MANAGEMENT_STATIC_PATH="+static)
}
