package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func expandPath(in string) (string, error) {
	if in == "" {
		return "", fmt.Errorf("path is required")
	}
	if strings.HasPrefix(in, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if in == "~" {
			in = home
		} else if strings.HasPrefix(in, "~/") || strings.HasPrefix(in, "~\\") {
			in = filepath.Join(home, in[2:])
		}
	}
	abs, err := filepath.Abs(filepath.Clean(in))
	if err != nil {
		return "", err
	}
	return abs, nil
}

func fileEmoji(size int64, noEmoji bool) string {
	if noEmoji {
		return ""
	}
	if size >= 2*1024*1024*1024 {
		return "[HOT]"
	}
	return "[COLD]"
}
