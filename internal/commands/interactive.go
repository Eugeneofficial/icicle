package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func runInteractive() int {
	if !isInteractiveTerminal() {
		home, err := os.UserHomeDir()
		if err != nil {
			printRootUsage()
			return 2
		}
		return runTree([]string{"--top", "5", home})
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "home error: %v\n", err)
		return 1
	}
	downloads := home + `\Downloads`
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("icicle - quick start")
	fmt.Println("1) Tree view (size map)")
	fmt.Println("2) Heavy files (top-N)")
	fmt.Println("3) Watch folder (auto-sort)")
	fmt.Println("4) Help")
	fmt.Print("Select [1-4, default 1]: ")
	choice := readLineOrDefault(reader, "1")

	switch choice {
	case "1":
		fmt.Printf("Path [default: %s]: ", home)
		path := readLineOrDefault(reader, home)
		return runTree([]string{"--top", "5", path})
	case "2":
		fmt.Printf("Path [default: %s]: ", home)
		path := readLineOrDefault(reader, home)
		fmt.Print("Top N [default: 20]: ")
		n := readLineOrDefault(reader, "20")
		return runHeavy([]string{"--n", n, path})
	case "3":
		fmt.Printf("Path to watch [default: %s]: ", downloads)
		path := readLineOrDefault(reader, downloads)
		fmt.Print("Dry run? [y/N]: ")
		dry := strings.ToLower(readLineOrDefault(reader, "n"))
		if dry == "y" || dry == "yes" {
			return runWatch([]string{"--dry-run", path})
		}
		return runWatch([]string{path})
	case "4":
		printRootUsage()
		return 0
	default:
		fmt.Fprintln(os.Stderr, "invalid choice")
		return 2
	}
}

func isInteractiveTerminal() bool {
	stdinInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	stdoutInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stdinInfo.Mode()&os.ModeCharDevice) != 0 && (stdoutInfo.Mode()&os.ModeCharDevice) != 0
}

func readLineOrDefault(r *bufio.Reader, fallback string) string {
	line, err := r.ReadString('\n')
	if err != nil {
		return fallback
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return fallback
	}
	return line
}
