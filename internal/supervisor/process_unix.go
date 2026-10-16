//go:build !windows

package supervisor

import (
	"os/exec"
	"syscall"
)

func configureChild(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func containChild(cmd *exec.Cmd) (func(), error) {
	return func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }, nil
}
func interruptChild(cmd *exec.Cmd) error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
func killChild(cmd *exec.Cmd) error      { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
