package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/fatih/color"
)

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
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	count := int(math.Round(ratio * float64(width)))
	if count < 1 && ratio > 0 {
		count = 1
	}
	bar := strings.Repeat("█", count)
	pad := strings.Repeat(" ", width-count)

	if t.NoColor {
		return bar + pad
	}
	if ratio >= 0.66 {
		return color.New(color.FgRed, color.Bold).Sprint(bar) + pad
	}
	if ratio >= 0.33 {
		return color.New(color.FgYellow, color.Bold).Sprint(bar) + pad
	}
	return color.New(color.FgBlue, color.Bold).Sprint(bar) + pad
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
