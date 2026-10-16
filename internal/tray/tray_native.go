//go:build darwin || windows

// Package tray connects native menus to the desktop's shared controller.
package tray

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"cliproxyapi-desktop/internal/bootstrap"
	"cliproxyapi-desktop/internal/branding"
	"cliproxyapi-desktop/internal/platform"
	"cliproxyapi-desktop/internal/service"
	"cliproxyapi-desktop/internal/settings"
	"github.com/atotto/clipboard"
	"github.com/getlantern/systray"
)

func Run(manager *service.Manager, paths settings.Paths, vault bootstrap.Vault, version string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	systray.Run(func() {
		systray.SetTooltip("CLIProxyAPI Desktop " + version)
		status := systray.AddMenuItem("Status: Stopped", "Current server state")
		status.Disable()
		start := systray.AddMenuItem("Start Service", "Start the bundled CLIProxyAPI server")
		stop := systray.AddMenuItem("Stop Service", "Stop the server; active requests will be closed")
		stop.Disable()
		systray.AddSeparator()
		config := systray.AddMenuItem("Open Config Folder", "Show the folder containing config.yaml")
		management := systray.AddMenuItem("Open Management UI", "Open management.html in your default browser")
		management.Disable()
		key := systray.AddMenuItem("Copy Management Key", "Copy the saved management login key")
		logs := systray.AddMenuItem("Open Logs Folder", "Show server logs")
		lastError := systray.AddMenuItem("Show Last Error", "Show the last server failure")
		lastError.Disable()
		systray.AddSeparator()
		about := systray.AddMenuItem("About", "Show wrapper and bundled versions")
		quit := systray.AddMenuItem("Quit", "Stop the server and exit")
		apply := func(s service.Snapshot) {
			status.SetTitle("Status: " + string(s.State))
			start.Disable()
			stop.Disable()
			management.Disable()
			lastError.Disable()
			if s.State == service.Stopped || s.State == service.Failed {
				start.Enable()
			}
			if s.State == service.Running {
				stop.Enable()
				if s.PanelAvailable {
					management.Enable()
				}
			}
			if s.Error != "" {
				lastError.Enable()
			}
			if runtime.GOOS == "darwin" {
				systray.SetTemplateIcon(branding.PNG(44, true, false), branding.PNG(44, false, s.State == service.Running))
			} else {
				systray.SetIcon(branding.ICO(s.State == service.Running))
			}
		}
		apply(manager.Snapshot())
		run := func(action func() error) {
			go func() {
				if err := action(); err != nil {
					platform.Alert(err.Error())
				}
			}()
		}
		go func() {
			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(signals)
			for {
				select {
				case s := <-manager.Changes():
					apply(s)
				case <-start.ClickedCh:
					run(func() error { return manager.Start(ctx) })
				case <-stop.ClickedCh:
					run(manager.Stop)
				case <-config.ClickedCh:
					run(func() error { return platform.RevealConfig(paths.Config) })
				case <-management.ClickedCh:
					s := manager.Snapshot()
					if s.State == service.Running && s.PanelAvailable {
						run(func() error { return platform.OpenURL(s.URL + "/management.html") })
					}
				case <-key.ClickedCh:
					run(func() error {
						value, err := vault.Get()
						if err != nil {
							return fmt.Errorf("no saved management key for this config; set management.secret-key in config.yaml: %w", err)
						}
						return clipboard.WriteAll(value)
					})
				case <-logs.ClickedCh:
					run(func() error { return platform.OpenFolder(paths.Logs) })
				case <-lastError.ClickedCh:
					go platform.Alert(manager.Snapshot().Error)
				case <-about.ClickedCh:
					data, _ := os.ReadFile(paths.Resources + string(os.PathSeparator) + "build-info.txt")
					go platform.Alert("CLIProxyAPI Desktop " + version + "\n" + string(data))
				case <-quit.ClickedCh:
					cancel()
					systray.Quit()
					return
				case <-signals:
					cancel()
					systray.Quit()
					return
				case <-ctx.Done():
					return
				}
			}
		}()
		run(func() error { return manager.Start(ctx) })
	}, func() { cancel(); _ = manager.Stop() })
	return nil
}
