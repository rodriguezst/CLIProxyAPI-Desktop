package autostart

import (
	"cliproxyapi-desktop/internal/fileutil"
	"errors"
	"os"
	"path/filepath"
)

func (m *Manager) Enabled() (bool, error) {
	_, err := os.Stat(filepath.Join(m.directory, m.id+".plist"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (m *Manager) Set(enabled bool) error {
	path := filepath.Join(m.directory, m.id+".plist")
	if !enabled {
		err := os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(m.directory, 0o700); err != nil {
		return err
	}
	// launchd reads this at the next sign-in. Do not bootstrap/bootout the live tray.
	return fileutil.Replace(path, m.launchAgent(), 0o600)
}
