//go:build !windows && !darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func OpenURL(url string) error       { return exec.Command("xdg-open", url).Run() }
func RevealConfig(path string) error { return OpenFolder(filepath.Dir(path)) }
func OpenFolder(path string) error   { return exec.Command("xdg-open", path).Run() }
func Alert(message string)           { _, _ = fmt.Fprintln(os.Stderr, message) }
