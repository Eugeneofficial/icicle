//go:build !windows

package gui

import "fmt"

func Run(appPath string) error {
	return fmt.Errorf("GUI is supported only on Windows")
}
