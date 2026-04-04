package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/fatih/color"
)

// True Color ANSI helpers (24-bit RGB)
func fg(r, g, b int) func(string) string {
	return func(s string) string {
		if color.NoColor {
			return s
		}
		return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", r, g, b, s)
	}
}

func fgb(r, g, b int) func(string) string {
	return func(s string) string {
		if color.NoColor {
			return s
		}
		return fmt.Sprintf("\033[1;38;2;%d;%d;%dm%s\033[0m", r, g, b, s)
	}
}

// Xiaomi orange #FF6900 = rgb(255, 105, 0)
var (
	xiaomi = fgb(255, 105, 0)   // оранжевый Xiaomi bold
	white  = fgb(255, 255, 255) // белый bold
	dim    = fg(120, 120, 120)  // серый
	red    = fgb(255, 60, 60)   // красный bold
	green  = fg(80, 200, 120)   // зелёный
	cyan   = fg(80, 180, 255)   // голубой
)

// Xiaomi exported для использования в других пакетах
var Xiaomi = xiaomi
var White = white
var Dim = dim
var Red = red
var Green = green
var Cyan = cyan

type Theme struct {
	NoColor bool
	NoEmoji bool
}

func (t Theme) Emoji(s string) string {
	if t.NoEmoji {
		return ""
	}
	return s
}

func (t Theme) Bar(ratio float64, width int) string {
	if width < 0 {
		width = 0
	}
	ratio = math.Max(0, math.Min(1, ratio))
	count := int(math.Round(ratio * float64(width)))
	if count < 1 && ratio > 0 {
		count = 1
	}
	if count > width {
		count = width
	}
	filled := strings.Repeat("█", count)
	empty := strings.Repeat("░", width-count)

	if t.NoColor {
		return filled + empty
	}
	if ratio >= 0.66 {
		return red(filled) + dim(empty)
	}
	if ratio >= 0.33 {
		return xiaomi(filled) + dim(empty)
	}
	return cyan(filled) + dim(empty)
}

func Title(icon, text string) string {
	return xiaomi(icon) + " " + white(text)
}

func Line() string {
	return dim(strings.Repeat("─", 48))
}

func Info(key, val string) string {
	return fmt.Sprintf("   %s %s", dim(key), val)
}

func Row(num int, size, icon, path string) string {
	n := fmt.Sprintf("%2d.", num)
	return fmt.Sprintf(" %s %s %s %s", dim(n), size, icon, cyan(path))
}

func Size(bytes int64, human string) string {
	if bytes >= 2*1024*1024*1024 {
		return red(human)
	}
	if bytes >= 500*1024*1024 {
		return xiaomi(human)
	}
	return white(human)
}

func Node(branch, icon, name, size, bar string) string {
	return fmt.Sprintf(" %s %s %-18s %s  %s", dim(branch), icon, white(name), xiaomi(size), bar)
}

func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n >= unit*div && exp < 5 {
		div *= unit
		exp++
	}
	value := float64(n) / float64(div)
	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	return fmt.Sprintf("%.1f %s", value, units[exp])
}
