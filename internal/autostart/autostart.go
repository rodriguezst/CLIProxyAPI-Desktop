// Package autostart registers the tray app for the current user's next sign-in.
package autostart

import (
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Manager struct {
	Executable, DataDir         string
	id, directory, registryPath string
}

func New(dataDir string) (*Manager, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	identity := dataDir
	if runtime.GOOS == "windows" {
		identity = strings.ToLower(identity)
	}
	hash := sha256.Sum256([]byte(identity))
	return &Manager{Executable: executable, DataDir: dataDir,
		id:           fmt.Sprintf("io.github.cliproxyapi-desktop.tray-%x", hash[:8]),
		directory:    filepath.Join(home, "Library", "LaunchAgents"),
		registryPath: `Software\Microsoft\Windows\CurrentVersion\Run`}, nil
}

func (m *Manager) Refresh() error {
	enabled, err := m.Enabled()
	if err != nil || !enabled {
		return err
	}
	return m.Set(true)
}

func (m *Manager) args() []string { return []string{m.Executable, "--login", "--data-dir", m.DataDir} }

func (m *Manager) launchAgent() []byte {
	escape := func(s string) string {
		var out strings.Builder
		_ = xml.EscapeText(&out, []byte(s))
		return out.String()
	}
	var args strings.Builder
	for _, arg := range m.args() {
		fmt.Fprintf(&args, "<string>%s</string>", escape(arg))
	}
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array>%s</array>
<key>RunAtLoad</key><true/>
<key>KeepAlive</key><false/>
<key>LimitLoadToSessionType</key><string>Aqua</string>
<key>ProcessType</key><string>Interactive</string>
<key>StandardOutPath</key><string>%s</string>
<key>StandardErrorPath</key><string>%s</string>
</dict></plist>
`, escape(m.id), args.String(), escape(filepath.Join(m.DataDir, "logs", "desktop.log")), escape(filepath.Join(m.DataDir, "logs", "desktop.log"))))
}
