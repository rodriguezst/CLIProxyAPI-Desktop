package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

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
	flag.Parse()
	if *printVersion {
		fmt.Println(version)
		return
	}
	if err := run(*data, *resources, *smoke); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if !*smoke {
			platform.Alert(err.Error())
		}
		os.Exit(1)
	}
}

func run(data, resources string, smoke bool) (errResult error) {
	if smoke && data == "" {
		return fmt.Errorf("--smoke-test requires an explicit temporary --data-dir")
	}
	p, err := settings.Resolve(data, resources)
	if err != nil {
		return err
	}
	release, err := instance.Acquire(p.Data)
	if err != nil {
		return err
	}
	defer func() {
		if err := release(); errResult == nil {
			errResult = err
		}
	}()
	var vault bootstrap.Vault = bootstrap.SystemVault{DataDir: p.Data}
	if smoke {
		vault = &bootstrap.MemoryVault{}
	}
	if err := bootstrap.Initialize(p, vault); err != nil {
		return err
	}
	manager := service.New(p)
	defer func() {
		if err := manager.Stop(); errResult == nil {
			errResult = err
		}
	}()
	if !smoke {
		return tray.Run(manager, p, vault, version)
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
		managementKey, err := vault.Get()
		if err != nil {
			return err
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
		if err := manager.Stop(); err != nil {
			return err
		}
	}
	fmt.Println("Packaged server start/stop, management page, and authenticated management smoke test passed.")
	return nil
}
