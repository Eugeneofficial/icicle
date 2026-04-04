package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"icicle/internal/ui"
)

func runInteractive() int {
	folders := detectUserFolders()
	home := folders.Home
	downloads := folders.Downloads

	if !isInteractiveTerminal() {
		return runTree([]string{"--top", "5", home})
	}
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println(" " + ui.Xiaomi("icicle") + " quick start")
	fmt.Println()
	fmt.Println("   " + ui.Xiaomi("1") + "   tree view")
	fmt.Println("   " + ui.Xiaomi("2") + "   heavy files")
	fmt.Println("   " + ui.Xiaomi("3") + "   watch folder")
	fmt.Println("   " + ui.Xiaomi("4") + "   help")
	fmt.Println()
	fmt.Print(ui.Dim("   select [1-4]: "))
	choice := readLineOrDefault(reader, "1")

	switch choice {
	case "1":
		fmt.Print(ui.Dim("   path [default: " + home + "]: "))
		path := readLineOrDefault(reader, home)
		return runTree([]string{"--top", "5", path})
	case "2":
		fmt.Print(ui.Dim("   path [default: " + home + "]: "))
		path := readLineOrDefault(reader, home)
		fmt.Print(ui.Dim("   top N [default: 20]: "))
		n := readLineOrDefault(reader, "20")
		return runHeavy([]string{"--n", n, path})
	case "3":
		fmt.Print(ui.Dim("   path [default: " + downloads + "]: "))
		path := readLineOrDefault(reader, downloads)
		fmt.Print(ui.Dim("   dry run? [y/N]: "))
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
