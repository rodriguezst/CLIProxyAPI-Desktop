//go:build !darwin && !windows

package tray

import (
	"cliproxyapi-desktop/internal/bootstrap"
	"cliproxyapi-desktop/internal/service"
	"cliproxyapi-desktop/internal/settings"
	"fmt"
)

func Run(_ *service.Manager, _ settings.Paths, _ bootstrap.Vault, _ string) error {
	return fmt.Errorf("the desktop tray supports Windows and macOS; use --smoke-test for headless verification")
}
