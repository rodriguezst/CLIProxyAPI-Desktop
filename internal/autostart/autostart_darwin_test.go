package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchAgentEnableRefreshDisable(t *testing.T) {
	m := &Manager{Executable: "/Applications/old/App", DataDir: t.TempDir(), directory: t.TempDir(), id: "desktop.test"}
	if enabled, err := m.Enabled(); err != nil || enabled {
		t.Fatal("registration unexpectedly enabled")
	}
	if err := m.Set(true); err != nil {
		t.Fatal(err)
	}
	m.Executable = "/Applications/new/App"
	if err := m.Refresh(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(m.directory, m.id+".plist"))
	if err != nil || !strings.Contains(string(data), m.Executable) {
		t.Fatal("moved app registration was not updated")
	}
	if err := m.Set(false); err != nil {
		t.Fatal(err)
	}
	if enabled, err := m.Enabled(); err != nil || enabled {
		t.Fatal("registration not removed")
	}
	if err := m.Set(false); err != nil {
		t.Fatal("disable is not idempotent")
	}
}
