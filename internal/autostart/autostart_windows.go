package autostart

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows/registry"
	"strings"
	"syscall"
	"unicode/utf16"
)

func (m *Manager) Enabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, m.registryPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer func() { _ = key.Close() }()
	_, _, err = key.GetStringValue(m.id)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (m *Manager) Set(enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, m.registryPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = key.Close() }()
	if !enabled {
		err := key.DeleteValue(m.id)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	args := m.args()
	for i, arg := range args {
		args[i] = syscall.EscapeArg(arg)
	}
	command := strings.Join(args, " ")
	if len(utf16.Encode([]rune(command))) > 260 {
		return fmt.Errorf("the startup command exceeds Windows' 260-character limit; install the app in a shorter path")
	}
	return key.SetStringValue(m.id, command)
}
