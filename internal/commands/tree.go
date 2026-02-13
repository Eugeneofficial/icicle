package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"icicle/internal/scan"
	"icicle/internal/ui"
)

func runTree(args []string) int {
	fs := flag.NewFlagSet("tree", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var common commonFlags
	addCommonFlags(fs, &common)
	limit := fs.Int("n", 20, "number of child entries to show")
	width := fs.Int("w", 24, "bar width")
	top := fs.Int("top", 5, "show top N files under tree")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: icicle tree [--n 20] [--w 24] [--top 5] [--no-color] [--no-emoji] <path>")
		return 2
	}
	applyCommonFlags(common)

	root, err := expandPath(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "path error: %v\n", err)
		return 1
	}

	stats, err := scan.ScanTree(root, *top)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
		return 1
	}

	theme := ui.Theme{NoColor: common.noColor, NoEmoji: common.noEmoji}
	fmt.Printf("%s  (total: %s)\n", root, ui.HumanBytes(stats.Total))

	shown := 0
	childCount := len(stats.ChildNames)
	if childCount > *limit {
		childCount = *limit
	}
	for _, name := range stats.ChildNames {
		if shown >= *limit {
			break
		}
		size := stats.ByChild[name]
		ratio := 0.0
		if stats.Total > 0 {
			ratio = float64(size) / float64(stats.Total)
		}
		prefix := "|-"
		isLastChild := shown == childCount-1
		if isLastChild && stats.RootFiles == 0 {
			prefix = "`-"
		}
		fmt.Printf("%s %s %-20s %8s  %s\n", prefix, "[DIR]", name, ui.HumanBytes(size), theme.Bar(ratio, *width))
		shown++
	}
	if stats.RootFiles > 0 {
		ratio := 0.0
		if stats.Total > 0 {
			ratio = float64(stats.RootFiles) / float64(stats.Total)
		}
		fmt.Printf("`- [FILES] %-18s %8s  %s\n", "(root)", ui.HumanBytes(stats.RootFiles), theme.Bar(ratio, *width))
	}

	fmt.Println()
	fmt.Println("TOP FILES:")
	for _, file := range stats.TopFiles {
		rel, relErr := filepath.Rel(root, file.Path)
		if relErr != nil {
			rel = file.Path
		}
		tag := fileEmoji(file.Size, common.noEmoji)
		fmt.Printf("%s %8s  %s\n", tag, ui.HumanBytes(file.Size), rel)
	}

	// Tiny easter egg for huge folders.
	if stats.Total >= 500*1024*1024*1024 {
		fmt.Println("\nice alert: this path is glacier-class heavy")
	}

	return 0
}
