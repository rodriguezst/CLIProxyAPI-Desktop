//go:build !windows

// Package instance enforces one launcher per data directory.
package instance

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

func Acquire(dir string) (func() error, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "desktop.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another CLIProxyAPI Desktop instance is using this data directory: %w", err)
	}
	return f.Close, nil
}
