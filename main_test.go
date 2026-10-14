package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddCustomNameExtractsIconAndRemoves(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("HOME", filepath.Join(temporary, "home"))
	source := filepath.Join(temporary, "download.AppImage")
	// This tiny fake AppImage implements the extraction flag used by appbox.
	script := "#!/bin/sh\nmkdir -p squashfs-root/usr/share/icons\nprintf icon > squashfs-root/usr/share/icons/my-great-app.png\n"
	if err := os.WriteFile(source, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	if err := add(source, "My Great App"); err != nil {
		t.Fatal(err)
	}
	p, err := userPaths()
	if err != nil {
		t.Fatal(err)
	}
	recordFile := filepath.Join(p.records, "my-great-app.json")
	bytes, err := os.ReadFile(recordFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bytes), "My Great App") {
		t.Fatalf("custom name was not recorded: %s", bytes)
	}
	if _, err := os.Stat(filepath.Join(p.icons, "my-great-app.png")); err != nil {
		t.Fatal("icon was not extracted:", err)
	}
	if err := remove("my-great-app"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(recordFile); !os.IsNotExist(err) {
		t.Fatal("record should have been removed")
	}
}
