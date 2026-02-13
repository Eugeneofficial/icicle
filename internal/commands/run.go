package commands

import (
	"fmt"
	"os"

	"icicle/internal/meta"
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
		fmt.Println("icicle " + meta.Version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", sub)
		printRootUsage()
		return 2
	}
}

func printRootUsage() {
	fmt.Println("icicle")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  icicle watch [path]   Watch a folder and auto-sort new files")
	fmt.Println("  icicle heavy [path]   Show top largest files")
	fmt.Println("  icicle tree [path]    Visualize size tree")
	fmt.Println("")
	fmt.Println("Default paths:")
	fmt.Println("  watch -> Windows Downloads folder")
	fmt.Println("  heavy/tree -> Windows Home folder")
	fmt.Println("")
	fmt.Println("Shared per-command flags:")
	fmt.Println("  --no-color           Disable ANSI colors")
	fmt.Println("  --no-emoji           Disable emoji in output")
	fmt.Println("")
}
