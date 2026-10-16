package service

import (
	"cliproxyapi-desktop/internal/settings"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

var fixture, helper string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "CPA native process fixtures ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	fixture, helper = filepath.Join(dir, "core"+suffix), filepath.Join(dir, "supervisor"+suffix)
	for _, pair := range [][2]string{{fixture, "./testdata/core"}, {helper, "../../cmd/supervisor"}} {
		cmd := exec.Command("go", "build", "-o", pair[0], pair[1])
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintln(os.Stderr, string(out), err)
			_ = os.RemoveAll(dir)
			os.Exit(1)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func testPaths(t *testing.T) settings.Paths {
	t.Helper()
	dir := t.TempDir()
	port, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := port.Addr().(*net.TCPAddr).Port
	if err := port.Close(); err != nil {
		t.Fatal(err)
	}
	p := settings.Paths{Data: dir, Config: filepath.Join(dir, "config.yaml"), Logs: filepath.Join(dir, "logs"), Static: filepath.Join(dir, "static"), Core: fixture, Supervisor: helper}
	for _, dir := range []string{p.Logs, p.Static} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(p.Config, []byte(fmt.Sprintf("server:\n  host: 127.0.0.1\n  port: %d\n", address)), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStartStopRestartsAndDeliversInterrupt(t *testing.T) {
	p := testPaths(t)
	marker := filepath.Join(p.Data, "signal.txt")
	t.Setenv("CPA_FIXTURE_MARKER", marker)
	m := New(p)
	t.Cleanup(func() { _ = m.Stop() })
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for i := 0; i < 2; i++ {
		if err := m.Start(ctx); err != nil {
			t.Fatal(err)
		}
		if s := m.Snapshot(); s.State != Running || s.PID == 0 {
			t.Fatalf("not running: %+v", s)
		}
		if err := m.Stop(); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(marker); err != nil || string(data) != "interrupted" {
			t.Fatalf("normal interrupt handler was not reached: %s, %v", data, err)
		}
		if s := m.Snapshot(); s.State != Stopped || s.PID != 0 {
			t.Fatalf("not stopped: %+v", s)
		}
		_ = os.Remove(marker)
	}
}

func TestZeroExitBeforeReadinessIsFailure(t *testing.T) {
	t.Setenv("CPA_FIXTURE_EXIT", "1")
	m := New(testPaths(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.Start(ctx); err == nil {
		_ = m.Stop()
		t.Fatal("early successful exit was reported as a running server")
	}
	if m.Snapshot().State != Failed {
		t.Fatal("expected failed state")
	}
}

func TestOccupiedPortDoesNotAdoptOrStopExistingServer(t *testing.T) {
	p := testPaths(t)
	l, err := settings.Read(p.Config)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", l.Address())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	m := New(p)
	if err := m.Start(context.Background()); err == nil {
		_ = m.Stop()
		t.Fatal("occupied port accepted")
	}
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	if listener.Addr() == nil {
		t.Fatal("existing listener was disturbed")
	}
}
