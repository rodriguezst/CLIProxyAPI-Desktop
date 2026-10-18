package bootstrap

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"cliproxyapi-desktop/internal/fileutil"
	"cliproxyapi-desktop/internal/settings"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

var ErrCanceled = errors.New("management key entry canceled")

func ValidateManagementKey(key string) error {
	if !utf8.ValidString(key) || strings.TrimSpace(key) == "" {
		return fmt.Errorf("enter a non-empty management key")
	}
	if len(key) > 72 {
		return fmt.Errorf("the management key must be at most 72 UTF-8 bytes")
	}
	return nil
}

// ResetManagementKey keeps config.yaml as the only source of the password hash.
// The caller stops its server before invoking this to avoid racing the API watcher.
func ResetManagementKey(path, key string) error {
	if err := ValidateManagementKey(key); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("management key reset requires a regular config.yaml file")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(before, &doc); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("config must be a YAML mapping")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	root := doc.Content[0]
	current := settings.Find(root, "management", "secret-key") != nil
	legacy := settings.Find(root, "remote-management", "secret-key") != nil
	if !current && !legacy {
		legacy = settings.Find(root, "remote-management") != nil && settings.Find(root, "management") == nil
		current = !legacy
	}
	if current {
		if err := settings.Set(root, string(hash), "management", "secret-key"); err != nil {
			return err
		}
	}
	if legacy {
		if err := settings.Set(root, string(hash), "remote-management", "secret-key"); err != nil {
			return err
		}
	}
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		_ = encoder.Close()
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	latest, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(latest, before) {
		return fmt.Errorf("config.yaml changed during reset; retry after saving your edits")
	}
	return fileutil.Replace(path, out.Bytes(), info.Mode())
}
