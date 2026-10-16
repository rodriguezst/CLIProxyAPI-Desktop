// This fixture exercises real process and signal behavior on each native runner.
package main

import (
	"cliproxyapi-desktop/internal/settings"
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	path := flag.String("config", "", "config")
	flag.Parse()
	if os.Getenv("CPA_FIXTURE_EXIT") == "1" {
		return
	}
	listener, err := settings.Read(*path)
	if err != nil {
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"status":"ok"}`)) })
	mux.HandleFunc("/management.html", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>fixture</html>")) })
	server := &http.Server{Addr: listener.Address(), Handler: mux}
	go func() {
		<-ctx.Done()
		if marker := os.Getenv("CPA_FIXTURE_MARKER"); marker != "" {
			_ = os.WriteFile(marker, []byte("interrupted"), 0o600)
		}
		_ = server.Close()
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		os.Exit(3)
	}
}
