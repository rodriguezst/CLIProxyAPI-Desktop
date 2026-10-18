package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"cliproxyapi-desktop/internal/autostart"
	"cliproxyapi-desktop/internal/bootstrap"
	"cliproxyapi-desktop/internal/instance"
	"cliproxyapi-desktop/internal/platform"
	"cliproxyapi-desktop/internal/service"
	"cliproxyapi-desktop/internal/settings"
	"cliproxyapi-desktop/internal/tray"
)

var version = "dev"

func main() {
	smoke := flag.Bool("smoke-test", false, "Start the bundled server, verify management.html, stop, and exit")
	data := flag.String("data-dir", "", "Override the per-user writable data directory")
	resources := flag.String("resources", "", "Override bundled resources for development")
	printVersion := flag.Bool("version", false, "Print wrapper version")
	login := flag.Bool("login", false, "Launch from the current user's sign-in startup entry")
	disableAutostart := flag.Bool("disable-autostart", false, "Remove this profile's sign-in startup entry and exit")
	flag.Parse()
	if *printVersion {
		fmt.Println(version)
		return
	}
	if err := run(*data, *resources, *smoke, *login, *disableAutostart); err != nil && !errors.Is(err, bootstrap.ErrCanceled) {
		fmt.Fprintln(os.Stderr, err)
		if !*smoke {
			platform.Alert(err.Error())
		}
		os.Exit(1)
	}
}

func run(data, resources string, smoke, login, disableAutostart bool) (errResult error) {
	if smoke && data == "" {
		return fmt.Errorf("--smoke-test requires an explicit temporary --data-dir")
	}
	p, err := settings.Resolve(data, resources)
	if err != nil {
		return err
	}
	startup, err := autostart.New(p.Data)
	if err != nil {
		return err
	}
	if disableAutostart {
		return startup.Set(false)
	}
	release, err := instance.Acquire(p.Data)
	if err != nil {
		if login && errors.Is(err, instance.ErrAlreadyRunning) {
			return nil
		}
		return err
	}
	defer func() {
		if err := release(); errResult == nil {
			errResult = err
		}
	}()
	managementKey := ""
	prompt := func() (string, error) { return platform.ManagementKey(context.Background(), false) }
	if smoke {
		managementKey = rand.Text()
		prompt = func() (string, error) { return managementKey, nil }
	}
	if err := bootstrap.Initialize(p, prompt); err != nil {
		return err
	}
	manager := service.New(p)
	defer func() {
		if err := manager.Stop(); errResult == nil {
			errResult = err
		}
	}()
	if !smoke {
		if err := startup.Refresh(); err != nil {
			platform.Alert("Could not refresh sign-in startup: " + err.Error())
		}
		return tray.Run(manager, p, startup, version)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	// Two complete cycles also catch retained handles and unreleased listeners.
	for i := 0; i < 2; i++ {
		if err := manager.Start(ctx); err != nil {
			return err
		}
		url := manager.Snapshot().URL + "/management.html"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}}
		resp, err := client.Do(req)
		if err != nil {
			client.CloseIdleConnections()
			return err
		}
		n, errRead := io.Copy(io.Discard, resp.Body)
		errClose := resp.Body.Close()
		client.CloseIdleConnections()
		if resp.StatusCode != http.StatusOK || n == 0 || errRead != nil || errClose != nil {
			return fmt.Errorf("management page verification failed (HTTP %d)", resp.StatusCode)
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, manager.Snapshot().URL+"/v8/management/config", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+managementKey)
		resp, err = client.Do(req)
		if err != nil {
			client.CloseIdleConnections()
			return err
		}
		_, errRead = io.Copy(io.Discard, io.LimitReader(resp.Body, 2<<20))
		errClose = resp.Body.Close()
		client.CloseIdleConnections()
		if resp.StatusCode != http.StatusOK || errRead != nil || errClose != nil {
			return fmt.Errorf("management login verification failed (HTTP %d)", resp.StatusCode)
		}
		if i == 0 {
			oldKey := managementKey
			managementKey = rand.Text()
			if err := manager.UpdateConfig(ctx, func() error { return bootstrap.ResetManagementKey(p.Config, managementKey) }); err != nil {
				return err
			}
			for _, check := range []struct {
				key    string
				status int
			}{{managementKey, http.StatusOK}, {oldKey, http.StatusUnauthorized}} {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, manager.Snapshot().URL+"/v8/management/config", nil)
				if err != nil {
					return err
				}
				req.Header.Set("Authorization", "Bearer "+check.key)
				resp, err := client.Do(req)
				if err != nil {
					client.CloseIdleConnections()
					return err
				}
				_, errRead := io.Copy(io.Discard, io.LimitReader(resp.Body, 2<<20))
				errClose := resp.Body.Close()
				client.CloseIdleConnections()
				if resp.StatusCode != check.status || errRead != nil || errClose != nil {
					return fmt.Errorf("management key reset verification failed (HTTP %d)", resp.StatusCode)
				}
			}
		}
		if err := manager.Stop(); err != nil {
			return err
		}
	}
	fmt.Println("Packaged server start/stop, management page, authentication, and key reset smoke test passed.")
	return nil
}
