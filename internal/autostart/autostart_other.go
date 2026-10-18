//go:build !windows && !darwin

package autostart

import "fmt"

func (m *Manager) Enabled() (bool, error) { return false, nil }
func (m *Manager) Set(enabled bool) error {
	if !enabled {
		return nil
	}
	return fmt.Errorf("automatic sign-in startup is supported on Windows and macOS")
}
