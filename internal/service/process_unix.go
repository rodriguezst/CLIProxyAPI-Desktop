//go:build !windows

package service

import "os/exec"

func configureSupervisor(cmd *exec.Cmd) {}
