package platform

import (
	"golang.org/x/sys/windows"
	"path/filepath"
	"unsafe"
)

func open(target string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	path, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, path, nil, nil, windows.SW_SHOWNORMAL)
}
func OpenURL(url string) error       { return open(url) }
func RevealConfig(path string) error { return open(filepath.Dir(path)) }
func OpenFolder(path string) error   { return open(path) }
func Alert(message string) {
	text, err := windows.UTF16PtrFromString(message)
	if err != nil {
		return
	}
	title, _ := windows.UTF16PtrFromString("CLIProxyAPI Desktop")
	proc := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
	_, _, _ = proc.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
