//go:build !windows

package commands

import (
	"os"
	"path/filepath"
	"strings"
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
	return userFolders{
		Home:      home,
		Downloads: filepath.Join(home, "Downloads"),
		Desktop:   filepath.Join(home, "Desktop"),
		Documents: filepath.Join(home, "Documents"),
	}
}
