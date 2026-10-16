package service

import (
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
)

func configureSupervisor(cmd *exec.Cmd) {
	// CREATE_NO_WINDOW would prevent delivery of console control events.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
}
