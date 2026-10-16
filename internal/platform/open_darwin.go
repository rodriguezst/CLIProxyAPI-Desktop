package platform

import (
	"os/exec"
	"path/filepath"
)

func OpenURL(url string) error       { return exec.Command("/usr/bin/open", url).Run() }
func RevealConfig(path string) error { return exec.Command("/usr/bin/open", "-R", path).Run() }
func OpenFolder(path string) error   { return exec.Command("/usr/bin/open", filepath.Clean(path)).Run() }
func Alert(message string) {
	_ = exec.Command("/usr/bin/osascript", "-e", `on run argv
display alert "CLIProxyAPI Desktop" message (item 1 of argv)
end run`, message).Run()
}
