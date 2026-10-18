package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cliproxyapi-desktop/internal/settings"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

func TestManagementKeyValidation(t *testing.T) {
	for _, tc := range []struct {
		key   string
		valid bool
	}{
		{"", false}, {" \t\n", false}, {" normal key ", true}, {strings.Repeat("é", 36), true}, {strings.Repeat("é", 37), false},
	} {
		if got := ValidateManagementKey(tc.key); (got == nil) != tc.valid {
			t.Errorf("valid=%v, want %v", got == nil, tc.valid)
		}
	}
}

func TestResetPreservesConfigAndHandlesBothLayouts(t *testing.T) {
	for _, tc := range []struct {
		name, yaml string
		paths      [][]string
	}{
		{"v8", "# user comment\nserver:\n  port: 9001\nmanagement:\n  secret-key: old # keep note\n  allow-remote: false\n", [][]string{{"management", "secret-key"}}},
		{"legacy", "# user comment\nport: 9001\nremote-management:\n  secret-key: old\n", [][]string{{"remote-management", "secret-key"}}},
		{"mixed", "# user comment\nport: 9001\nmanagement:\n  secret-key: newer\nremote-management:\n  secret-key: older\n", [][]string{{"management", "secret-key"}, {"remote-management", "secret-key"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := ResetManagementKey(path, "user chose this key"); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "# user comment") || strings.Contains(string(data), "user chose this key") {
				t.Fatal("comment lost or plaintext password persisted")
			}
			if tc.name == "v8" && !strings.Contains(string(data), "# keep note") {
				t.Fatal("inline comment lost")
			}
			var doc yaml.Node
			if err := yaml.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			for _, path := range tc.paths {
				hash := settings.Find(doc.Content[0], path...).Value
				if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("user chose this key")); err != nil {
					t.Fatal("new password not configured")
				}
			}
			listener, err := settings.Read(path)
			if err != nil || listener.Port != 9001 {
				t.Fatal("unrelated listener settings changed")
			}
		})
	}
}

func TestResetInvalidKeyDoesNotChangeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	before := []byte("management:\n  secret-key: old\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ResetManagementKey(path, ""); err == nil {
		t.Fatal("blank key accepted")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("failed reset changed configuration")
	}
}
