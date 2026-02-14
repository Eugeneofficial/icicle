package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"fmt"
	"icicle/internal/commands"
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
			if guiMode {
				// Friendly UX for double-click: quietly exit if app is already open.
				return
			}
			fmt.Fprintln(os.Stderr, "icicle is already running")
			os.Exit(1)
		}
		defer singleinstance.Release()
	}

	if guiMode {
		if exePath, err := os.Executable(); err == nil {
			desktopExe := filepath.Join(filepath.Dir(exePath), "icicle-desktop.exe")
			if launchDesktop(desktopExe) == nil {
				return
			}
			if tryBuildDesktop(filepath.Dir(exePath), desktopExe) == nil && launchDesktop(desktopExe) == nil {
				return
			}
		}
		fmt.Fprintln(os.Stderr, "desktop GUI not found. Build with: go build -tags \"wails,production\" -o icicle-desktop.exe ./cmd/icicle-wails")
		os.Exit(1)
	}
	os.Exit(commands.Run(os.Args))
}

func launchDesktop(desktopExe string) error {
	if _, err := os.Stat(desktopExe); err != nil {
		return err
	}
	cmd := exec.Command(desktopExe)
	cmd.Dir = filepath.Dir(desktopExe)
	cmd.Env = append(os.Environ(), "ICICLE_ALLOW_MULTI=1")
	return cmd.Start()
}

func tryBuildDesktop(rootDir, desktopExe string) error {
	if _, err := os.Stat(filepath.Join(rootDir, "cmd", "icicle-wails", "main_windows.go")); err != nil {
		return err
	}
	if _, err := exec.LookPath("go"); err != nil {
		return err
	}
	cmd := exec.Command("go", "build", "-tags", "wails,production", "-o", desktopExe, "./cmd/icicle-wails")
	cmd.Dir = rootDir
	cmd.Env = append(os.Environ(), "GOFLAGS=-trimpath -buildvcs=false")
	return cmd.Run()
}
