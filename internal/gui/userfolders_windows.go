//go:build windows

package gui

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

type userFolders struct {
	Home      string
	Downloads string
	Desktop   string
	Documents string
}

func detectUserFolders() userFolders {
	home, _ := os.UserHomeDir()
	if strings.TrimSpace(home) == "" {
		home = "."
	}
	f := userFolders{
		Home:      home,
		Downloads: filepath.Join(home, "Downloads"),
		Desktop:   filepath.Join(home, "Desktop"),
		Documents: filepath.Join(home, "Documents"),
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`, registry.QUERY_VALUE)
	if err != nil {
		return f
	}
	defer k.Close()

	read := func(name, fallback string) string {
		v, _, err := k.GetStringValue(name)
		if err != nil || strings.TrimSpace(v) == "" {
			return fallback
		}
		v = os.ExpandEnv(v)
		if strings.TrimSpace(v) == "" {
			return fallback
		}
		return filepath.Clean(v)
	}

	f.Desktop = read("Desktop", f.Desktop)
	f.Documents = read("Personal", f.Documents)
	f.Downloads = read("{374DE290-123F-4565-9164-39C4925E467B}", f.Downloads)
	return f
}
