package main

import (
	"os"

	"fmt"
	"icicle/internal/commands"
	"icicle/internal/gui"
)

func main() {
	if len(os.Args) < 2 {
		if err := gui.Run(os.Args[0]); err == nil {
			return
		} else {
			fmt.Fprintf(os.Stderr, "GUI start failed: %v\n", err)
			os.Exit(1)
		}
	}
	if len(os.Args) >= 2 && os.Args[1] == "gui" {
		if err := gui.Run(os.Args[0]); err == nil {
			return
		} else {
			fmt.Fprintf(os.Stderr, "GUI start failed: %v\n", err)
			os.Exit(1)
		}
	}
	os.Exit(commands.Run(os.Args))
}
