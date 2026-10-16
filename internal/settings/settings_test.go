package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadListenerLayouts(t *testing.T) {
	for _, tc := range []struct {
		name, yaml, want string
		invalid          bool
	}{
		{"v8", "server:\n  host: 127.0.0.1\n  port: 9000\n", "http://127.0.0.1:9000", false},
		{"legacy", "host: 0.0.0.0\nport: 8317\ntls:\n  enable: true\n", "https://127.0.0.1:8317", false},
		{"TLS hostname", "server:\n  host: localhost\n  tls:\n    enable: true\n", "https://localhost:8317", false},
		{"v8 wins", "host: wrong\nport: 42\nserver:\n  host: '::'\n  port: 9001\n", "http://[::1]:9001", false},
		{"invalid port", "server:\n  port: 0\n", "", true},
		{"wrong type", "server:\n  tls:\n    enable: garbage\n", "", true},
		{"not mapping", "- bad\n", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			listener, err := Read(path)
			if tc.invalid {
				if err == nil {
					t.Fatal("expected config error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := listener.BaseURL(); got != tc.want {
				t.Fatalf("URL = %s, want %s", got, tc.want)
			}
			data, _ := os.ReadFile(path)
			if string(data) != tc.yaml {
				t.Fatal("reading config modified the file")
			}
		})
	}
}
