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
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "usage: icicle tree [--n 20] [--w 24] [--top 5] [--no-color] [--no-emoji] [path]")
		return 2
	}
	if *limit < 0 || *width < 0 || *top < 0 {
		fmt.Fprintln(os.Stderr, "flag error: values must be >= 0")
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

	stats, err := scan.ScanTree(root, *top, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
		return 1
	}

	theme := ui.Theme{NoColor: common.noColor, NoEmoji: common.noEmoji}

	fmt.Println()
	fmt.Println(ui.Title("●", "DISK TREE"))
	fmt.Println(ui.Info("path", ui.Cyan(root)))
	fmt.Println(ui.Info("total", ui.HumanBytes(stats.Total)))
	fmt.Println(ui.Line())

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
		branch := "├─"
		if shown == childCount-1 && stats.RootFiles == 0 {
			branch = "└─"
		}
		fmt.Println(ui.Node(branch, "📁", name, ui.HumanBytes(size), theme.Bar(ratio, *width)))
		shown++
	}
	if stats.RootFiles > 0 {
		ratio := 0.0
		if stats.Total > 0 {
			ratio = float64(stats.RootFiles) / float64(stats.Total)
		}
		fmt.Println(ui.Node("└─", "📄", "(root)", ui.HumanBytes(stats.RootFiles), theme.Bar(ratio, *width)))
	}

	fmt.Println()
	for i, file := range stats.TopFiles {
		rel, relErr := filepath.Rel(root, file.Path)
		if relErr != nil {
			rel = file.Path
		}
		tag := fileEmoji(file.Size, common.noEmoji)
		fmt.Println(ui.Row(i+1, ui.Size(file.Size, ui.HumanBytes(file.Size)), tag, rel))
	}

	fmt.Println()
	return 0
}
