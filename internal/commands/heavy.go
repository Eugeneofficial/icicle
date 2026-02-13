package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"icicle/internal/scan"
	"icicle/internal/ui"
)

func runHeavy(args []string) int {
	fs := flag.NewFlagSet("heavy", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var common commonFlags
	addCommonFlags(fs, &common)
	limit := fs.Int("n", 20, "number of files to show")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "usage: icicle heavy [--n 20] [--no-color] [--no-emoji] [path]")
		return 2
	}
	applyCommonFlags(common)

	folders := detectUserFolders()
	pathArg := fs.Arg(0)
	if pathArg == "" {
		pathArg = folders.Home
	}
	root, err := expandPath(pathArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "path error: %v\n", err)
		return 1
	}

	stats, err := scan.ScanTopFiles(root, *limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
		return 1
	}

	fmt.Printf("TOP FILES in %s\n", root)
	if len(stats.TopFiles) == 0 {
		fmt.Println("No files found.")
		return 0
	}
	for _, file := range stats.TopFiles {
		rel, relErr := filepath.Rel(root, file.Path)
		if relErr != nil {
			rel = file.Path
		}
		tag := fileEmoji(file.Size, common.noEmoji)
		fmt.Printf("%s %8s  %s\n", tag, ui.HumanBytes(file.Size), rel)
	}
	return 0
}
