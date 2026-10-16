// Package settings manages desktop paths and reads the server listener config.
package settings

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Paths struct {
	Data, Config, Static, Logs, Core, Supervisor, Resources string
}

func Resolve(dataOverride, resourcesOverride string) (Paths, error) {
	executable, err := os.Executable()
	if err != nil {
		return Paths{}, fmt.Errorf("locate application: %w", err)
	}
	bin := filepath.Dir(executable)
	resources := filepath.Join(bin, "resources")
	if runtime.GOOS == "darwin" {
		resources = filepath.Join(bin, "..", "Resources")
	}
	if resourcesOverride != "" {
		resources = resourcesOverride
	}
	data := dataOverride
	if data == "" {
		if runtime.GOOS == "windows" {
			data = os.Getenv("LOCALAPPDATA")
			if data == "" {
				return Paths{}, fmt.Errorf("LOCALAPPDATA is not set")
			}
		} else {
			data, err = os.UserConfigDir()
			if err != nil {
				return Paths{}, fmt.Errorf("locate user configuration directory: %w", err)
			}
		}
		data = filepath.Join(data, "CLIProxyAPI")
	}
	data, err = filepath.Abs(data)
	if err != nil {
		return Paths{}, err
	}
	resources, err = filepath.Abs(resources)
	if err != nil {
		return Paths{}, err
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	core := filepath.Join(resources, "cli-proxy-api"+suffix)
	if runtime.GOOS == "darwin" && resourcesOverride == "" {
		core = filepath.Join(bin, "cli-proxy-api")
	}
	return Paths{
		Data: data, Config: filepath.Join(data, "config.yaml"),
		Static: filepath.Join(data, "static"), Logs: filepath.Join(data, "logs"),
		Core: core, Supervisor: filepath.Join(bin, "cli-proxy-supervisor"+suffix),
		Resources: resources,
	}, nil
}

type Listener struct {
	Host                     string
	Port                     int
	TLS, PanelDisabled, Home bool
}

// Read only decodes YAML. It never rewrites the user's configuration.
func Read(path string) (Listener, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Listener{}, fmt.Errorf("read config: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Listener{}, fmt.Errorf("parse config: %w", err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return Listener{}, fmt.Errorf("config must be a YAML mapping")
	}
	root := doc.Content[0]
	l := Listener{Port: 8317}
	fields := []struct {
		current, legacy string
		target          any
	}{
		{"server.host", "host", &l.Host}, {"server.port", "port", &l.Port},
		{"server.tls.enable", "tls.enable", &l.TLS},
		{"management.disable-control-panel", "remote-management.disable-control-panel", &l.PanelDisabled},
		{"home.enabled", "home.enabled", &l.Home},
	}
	for _, f := range fields {
		n := Find(root, strings.Split(f.current, ".")...)
		if n == nil {
			n = Find(root, strings.Split(f.legacy, ".")...)
		}
		if n != nil {
			if err := n.Decode(f.target); err != nil {
				return Listener{}, fmt.Errorf("invalid %s: %w", f.current, err)
			}
		}
	}
	if l.Port < 1 || l.Port > 65535 {
		return Listener{}, fmt.Errorf("server.port must be between 1 and 65535")
	}
	if l.Home {
		return Listener{}, fmt.Errorf("Home mode is not supported by this local desktop launcher")
	}
	return l, nil
}

func (l Listener) Address() string { return net.JoinHostPort(l.Host, strconv.Itoa(l.Port)) }

func (l Listener) BaseURL() string {
	host := l.Host
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	} else if host == "::" {
		host = "::1"
	}
	scheme := "http"
	if l.TLS {
		scheme = "https"
	}
	return (&url.URL{Scheme: scheme, Host: net.JoinHostPort(host, strconv.Itoa(l.Port))}).String()
}

func Find(root *yaml.Node, path ...string) *yaml.Node {
	if len(path) == 0 {
		return root
	}
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == path[0] {
			return Find(root.Content[i+1], path[1:]...)
		}
	}
	return nil
}

func Set(root *yaml.Node, value any, path ...string) error {
	if len(path) == 0 {
		return fmt.Errorf("empty config path")
	}
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("config parent is not a mapping")
	}
	var child *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == path[0] {
			child = root.Content[i+1]
			break
		}
	}
	if child == nil {
		child = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: path[0]}, child)
	}
	if len(path) == 1 {
		return child.Encode(value)
	}
	return Set(child, value, path[1:]...)
}
