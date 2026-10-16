package main

import (
	"cliproxyapi-desktop/internal/supervisor"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	var o supervisor.Options
	flag.StringVar(&o.Core, "core", "", "Bundled server executable")
	flag.StringVar(&o.Config, "config", "", "Server config file")
	flag.StringVar(&o.Data, "data", "", "Server working directory")
	flag.StringVar(&o.Static, "static", "", "Management asset directory")
	flag.StringVar(&o.Log, "log", "", "Server log file")
	flag.Parse()
	if o.Core == "" || o.Config == "" || o.Data == "" || o.Static == "" || o.Log == "" {
		fmt.Fprintln(os.Stderr, "The supervisor must be launched by CLIProxyAPI Desktop.")
		os.Exit(2)
	}
	if err := supervisor.Run(o, os.Stdin, os.Stdout); err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(supervisor.Event{Type: "error", Error: err.Error()})
		os.Exit(1)
	}
}
