package main

import (
	"cliproxyapi-desktop/internal/branding"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	out := flag.String("out", "build/icon.png", "Output PNG or ICO")
	flag.Parse()
	data := branding.PNG(1024, false, true)
	if filepath.Ext(*out) == ".ico" {
		data = branding.ICO(true)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
