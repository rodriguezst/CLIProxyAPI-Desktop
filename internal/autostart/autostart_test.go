package autostart

import (
	"encoding/xml"
	"testing"
)

func TestLaunchAgentEscapesPathsAndStartsOnlyAtLogin(t *testing.T) {
	m := &Manager{Executable: `/Applications/Proxy & "tools"/App`, DataDir: `/Users/test/Library/Application Support/Proxy & data`, id: "test.id"}
	var doc struct {
		Dict struct {
			Keys  []string `xml:"key"`
			Array struct {
				Args []string `xml:"string"`
			} `xml:"array"`
			True  []struct{} `xml:"true"`
			False []struct{} `xml:"false"`
		} `xml:"dict"`
	}
	if err := xml.Unmarshal(m.launchAgent(), &doc); err != nil {
		t.Fatal(err)
	}
	args := doc.Dict.Array.Args
	if len(args) != 4 || args[0] != m.Executable || args[1] != "--login" || args[2] != "--data-dir" || args[3] != m.DataDir {
		t.Fatalf("incorrect program arguments: %v", args)
	}
	if len(doc.Dict.True) != 1 || len(doc.Dict.False) != 1 {
		t.Fatal("expected RunAtLoad enabled and KeepAlive disabled")
	}
}
