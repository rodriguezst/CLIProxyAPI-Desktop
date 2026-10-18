//go:build darwin || windows

// Package tray connects native menus to the desktop's shared controller.
package tray

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"cliproxyapi-desktop/internal/autostart"
	"cliproxyapi-desktop/internal/bootstrap"
	"cliproxyapi-desktop/internal/branding"
	"cliproxyapi-desktop/internal/platform"
	"cliproxyapi-desktop/internal/service"
	"cliproxyapi-desktop/internal/settings"
	"github.com/getlantern/systray"
)

func Run(manager *service.Manager, paths settings.Paths, startup *autostart.Manager, version string) error {
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
		key := systray.AddMenuItem("Reset Management Key…", "Choose a new key and update config.yaml")
		enabled, err := startup.Enabled()
		if err != nil {
			go platform.Alert(err.Error())
		}
		startAtLogin := systray.AddMenuItemCheckbox("Start at Sign-in", "Start the tray and proxy when you sign in", enabled)
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
			keyResult := make(chan error, 1)
			startupResult := make(chan error, 1)
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
					key.Disable()
					go func() {
						value, err := platform.ManagementKey(ctx, true)
						if err == nil {
							err = manager.UpdateConfig(ctx, func() error { return bootstrap.ResetManagementKey(paths.Config, value) })
						}
						keyResult <- err
					}()
				case err := <-keyResult:
					key.Enable()
					if err != nil && !errors.Is(err, bootstrap.ErrCanceled) {
						go platform.Alert(err.Error())
					}
				case <-startAtLogin.ClickedCh:
					startAtLogin.Disable()
					go func() {
						enabled, err := startup.Enabled()
						if err == nil {
							err = startup.Set(!enabled)
						}
						startupResult <- err
					}()
				case err := <-startupResult:
					startAtLogin.Enable()
					if err != nil {
						go platform.Alert(err.Error())
					}
					enabled, err := startup.Enabled()
					if err == nil {
						if enabled {
							startAtLogin.Check()
						} else {
							startAtLogin.Uncheck()
						}
					}
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
