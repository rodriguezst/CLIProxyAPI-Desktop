// Package bootstrap initializes writable data without replacing user settings.
package bootstrap

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"cliproxyapi-desktop/internal/settings"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

func Initialize(p settings.Paths, prompt func() (string, error)) error {
	for _, dir := range []string{p.Data, p.Static, p.Logs, filepath.Join(p.Data, "auths")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create data directory: %w", err)
		}
	}
	if _, err := os.Stat(p.Config); errors.Is(err, os.ErrNotExist) {
		if err := createConfig(p, prompt); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return updatePanel(filepath.Join(p.Resources, "management.html"), filepath.Join(p.Static, "management.html"))
}

func createConfig(p settings.Paths, prompt func() (string, error)) error {
	data, err := os.ReadFile(filepath.Join(p.Resources, "config.example.yaml"))
	if err != nil {
		return fmt.Errorf("read bundled config template: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse config template: %w", err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("bundled config template is not a mapping")
	}
	key, err := prompt()
	if err != nil {
		return err
	}
	if err := ValidateManagementKey(key); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	apiKey, err := randomKey()
	if err != nil {
		return err
	}
	values := []struct {
		path  []string
		value any
	}{
		{[]string{"server", "host"}, "127.0.0.1"},
		{[]string{"server", "port"}, 8317},
		{[]string{"management", "allow-remote"}, false},
		{[]string{"management", "secret-key"}, string(hash)},
		{[]string{"management", "disable-control-panel"}, false},
		{[]string{"management", "disable-auto-update-panel"}, true},
		{[]string{"access", "api-keys"}, []string{apiKey}},
		{[]string{"oauth", "auth-dir"}, filepath.Join(p.Data, "auths")},
	}
	for _, value := range values {
		if err := settings.Set(doc.Content[0], value.value, value.path...); err != nil {
			return err
		}
	}
	data, err = yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	return writeNew(p.Config, data)
}

func randomKey() (string, error) {
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func writeNew(path string, data []byte) (errResult error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); errResult == nil {
			errResult = err
		}
		if errResult != nil {
			_ = os.Remove(path)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

func updatePanel(source, target string) (errResult error) {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open bundled management page: %w", err)
	}
	defer func() {
		if err := f.Close(); errResult == nil {
			errResult = err
		}
	}()
	data, err := io.ReadAll(io.LimitReader(f, (50<<20)+1))
	if err != nil {
		return err
	}
	if len(data) > 50<<20 {
		return fmt.Errorf("bundled management page exceeds size limit")
	}
	existing, err := os.ReadFile(target)
	if err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// The panel is a managed asset. Refresh it on app upgrades without touching config.
	temporary, err := os.CreateTemp(filepath.Dir(target), ".management-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temporary.Name()) }()
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), target)
}
