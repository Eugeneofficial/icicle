package commands

import (
	"fmt"
	"os"

	"icicle/internal/meta"
	"icicle/internal/ui"
)

func Run(args []string) int {
	if len(args) < 2 {
		printRootUsage()
		return 2
	}

	sub := args[1]
	switch sub {
	case "help", "-h", "--help":
		printRootUsage()
		return 0
	case "watch":
		return runWatch(args[2:])
	case "heavy":
		return runHeavy(args[2:])
	case "tree":
		return runTree(args[2:])
	case "version", "-v", "--version":
		fmt.Println("icicle " + ui.Dim(meta.Version))
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", sub)
		printRootUsage()
		return 2
	}
}

func printRootUsage() {
	fmt.Println()
	fmt.Println(" " + ui.Xiaomi("icicle") + " " + ui.Dim(meta.Version))
	fmt.Println()
	fmt.Println(" " + ui.White("commands"))
	fmt.Println("   " + ui.Xiaomi("heavy") + "  [path]     top largest files")
	fmt.Println("   " + ui.Xiaomi("tree") + "   [path]     size tree map")
	fmt.Println("   " + ui.Xiaomi("watch") + "  [path]     auto-sort folder")
	fmt.Println("   " + ui.Xiaomi("version") + "            show version")
	fmt.Println()
	fmt.Println(" " + ui.White("flags"))
	fmt.Println("   " + ui.Dim("--no-color") + "         disable colors")
	fmt.Println("   " + ui.Dim("--no-emoji") + "         disable emoji")
	fmt.Println()
	fmt.Println(" " + ui.White("examples"))
	fmt.Println("   " + ui.Dim("icicle heavy --n 20 ~/Downloads"))
	fmt.Println("   " + ui.Dim("icicle tree ~/Documents"))
	fmt.Println("   " + ui.Dim("icicle watch ~/Downloads"))
	fmt.Println()
}
