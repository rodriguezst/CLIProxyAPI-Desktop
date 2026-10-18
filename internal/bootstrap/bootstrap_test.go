package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"cliproxyapi-desktop/internal/settings"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

func TestInitializeProtectsCredentialsAndPreservesConfig(t *testing.T) {
	base := t.TempDir()
	p := settings.Paths{Data: filepath.Join(base, "user data"), Resources: filepath.Join(base, "resources")}
	p.Config, p.Static, p.Logs = filepath.Join(p.Data, "config.yaml"), filepath.Join(p.Data, "static"), filepath.Join(p.Data, "logs")
	if err := os.MkdirAll(p.Resources, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{"config.example.yaml": "config-version: 8\nserver:\n  host: ''\naccess:\n  api-keys: [your-api-key-1]\n", "management.html": "<html>bundled</html>"} {
		if err := os.WriteFile(filepath.Join(p.Resources, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	key := "first user supplied management key"
	prompts := 0
	prompt := func() (string, error) { prompts++; return key, nil }
	if err := Initialize(p, prompt); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(p.Config)
	if err != nil {
		t.Fatal(err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(original, &doc); err != nil {
		t.Fatal(err)
	}
	secret := settings.Find(doc.Content[0], "management", "secret-key").Value
	if err := bcrypt.CompareHashAndPassword([]byte(secret), []byte(key)); err != nil {
		t.Fatal("config does not contain the matching password hash")
	}
	if settings.Find(doc.Content[0], "access", "api-keys").Content[0].Value == "your-api-key-1" {
		t.Fatal("template API key retained")
	}
	if l, err := settings.Read(p.Config); err != nil || l.Host != "127.0.0.1" {
		t.Fatalf("desktop listener is not loopback: %+v, %v", l, err)
	}
	if err := Initialize(p, prompt); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(p.Config)
	if string(original) != string(after) || prompts != 1 {
		t.Fatal("existing config or credentials changed")
	}
	if err := os.WriteFile(filepath.Join(p.Resources, "management.html"), []byte("<html>new bundled version</html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Initialize(p, prompt); err != nil {
		t.Fatal(err)
	}
	panel, err := os.ReadFile(filepath.Join(p.Static, "management.html"))
	if err != nil || string(panel) != "<html>new bundled version</html>" {
		t.Fatal("app upgrade did not refresh managed panel")
	}
	after, _ = os.ReadFile(p.Config)
	if string(original) != string(after) {
		t.Fatal("panel upgrade modified user config")
	}
}

func TestFirstRunCancellationDoesNotCreateConfig(t *testing.T) {
	base := t.TempDir()
	p := settings.Paths{Data: base, Config: filepath.Join(base, "config.yaml"), Static: filepath.Join(base, "static"), Logs: filepath.Join(base, "logs"), Resources: base}
	if err := os.WriteFile(filepath.Join(base, "config.example.yaml"), []byte("server: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Initialize(p, func() (string, error) { return "", ErrCanceled }); err != ErrCanceled {
		t.Fatal("cancellation was not returned:", err)
	}
	if _, err := os.Stat(p.Config); !os.IsNotExist(err) {
		t.Fatal("cancellation created config.yaml")
	}
}
