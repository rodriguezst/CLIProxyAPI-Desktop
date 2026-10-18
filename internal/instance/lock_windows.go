package instance

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"strings"
)

func Acquire(dir string) (func() error, error) {
	hash := sha256.Sum256([]byte(strings.ToLower(dir)))
	name, err := windows.UTF16PtrFromString(fmt.Sprintf("Local\\CLIProxyAPI-Desktop-%x", hash[:16]))
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if handle != 0 {
			_ = windows.CloseHandle(handle)
		}
		return nil, ErrAlreadyRunning
	}
	if err != nil {
		return nil, err
	}
	installerName, _ := windows.UTF16PtrFromString(`Local\CLIProxyAPI-Desktop-Installer`)
	installer, err := windows.CreateMutex(nil, false, installerName)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.CloseHandle(handle)
		return nil, err
	}
	return func() error { return errors.Join(windows.CloseHandle(installer), windows.CloseHandle(handle)) }, nil
}
