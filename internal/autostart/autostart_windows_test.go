package autostart

import (
	"golang.org/x/sys/windows/registry"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRegistrationEnableRefreshDisable(t *testing.T) {
	m := &Manager{Executable: `C:\Program Files\Proxy\App.exe`, DataDir: `C:\Users\test\Proxy Data`, id: "desktop.test", registryPath: `Software\CLIProxyAPIDesktopTests\` + filepath.Base(t.TempDir())}
	t.Cleanup(func() { _ = registry.DeleteKey(registry.CURRENT_USER, m.registryPath) })
	if enabled, err := m.Enabled(); err != nil || enabled {
		t.Fatal("registration unexpectedly enabled")
	}
	if err := m.Set(true); err != nil {
		t.Fatal(err)
	}
	m.Executable = `C:\Program Files\New Proxy\App.exe`
	if err := m.Refresh(); err != nil {
		t.Fatal(err)
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, m.registryPath, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	value, _, err := key.GetStringValue(m.id)
	_ = key.Close()
	if err != nil || !strings.HasPrefix(value, `"`+m.Executable+`" --login --data-dir "`+m.DataDir+`"`) {
		t.Fatalf("incorrect startup command: %q", value)
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
