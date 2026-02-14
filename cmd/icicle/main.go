package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"fmt"
	"icicle/internal/commands"
	"icicle/internal/gui"
	"icicle/internal/singleinstance"
)

func main() {
	guiMode := len(os.Args) < 2 || (len(os.Args) >= 2 && os.Args[1] == "gui")
	if os.Getenv("ICICLE_ALLOW_MULTI") != "1" {
		ok, err := singleinstance.Acquire("icicle_single_instance_v1")
		if err != nil {
			fmt.Fprintf(os.Stderr, "instance guard failed: %v\n", err)
			os.Exit(1)
		}
		if !ok {
			fmt.Fprintln(os.Stderr, "icicle is already running")
			os.Exit(1)
		}
		defer singleinstance.Release()
	}

	if guiMode {
		// If desktop build exists, use it instead of browser GUI.
		if exePath, err := os.Executable(); err == nil {
			desktopExe := filepath.Join(filepath.Dir(exePath), "icicle-desktop.exe")
			if _, err := os.Stat(desktopExe); err == nil {
				cmd := exec.Command(desktopExe)
				cmd.Dir = filepath.Dir(desktopExe)
				cmd.Env = append(os.Environ(), "ICICLE_ALLOW_MULTI=1")
				if err := cmd.Start(); err == nil {
					return
				}
			}
		}
		if err := gui.Run(os.Args[0]); err == nil {
			return
		} else {
			fmt.Fprintf(os.Stderr, "GUI start failed: %v\n", err)
			os.Exit(1)
		}
	}
	os.Exit(commands.Run(os.Args))
}
