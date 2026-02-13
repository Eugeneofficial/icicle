package commands

import (
	"flag"

	"github.com/fatih/color"
)

type commonFlags struct {
	noColor bool
	noEmoji bool
}

func addCommonFlags(fs *flag.FlagSet, c *commonFlags) {
	fs.BoolVar(&c.noColor, "no-color", false, "disable ANSI colors")
	fs.BoolVar(&c.noEmoji, "no-emoji", false, "disable emoji in output")
}

func applyCommonFlags(c commonFlags) {
	if c.noColor {
		color.NoColor = true
	}
}
