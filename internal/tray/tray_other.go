//go:build !darwin && !windows

package tray

import (
	"fmt"

	"cliproxyapi-desktop/internal/autostart"
	"cliproxyapi-desktop/internal/service"
	"cliproxyapi-desktop/internal/settings"
)

func Run(_ *service.Manager, _ settings.Paths, _ *autostart.Manager, _ string) error {
	return fmt.Errorf("the desktop tray supports Windows and macOS; use --smoke-test for headless verification")
}
