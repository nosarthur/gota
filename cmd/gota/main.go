package main

import (
	"github.com/alexflint/go-arg"

	"github.com/nosarthur/gota/ui"
)

type AddCmd struct {
	Path string `arg:"positional"`
}

type RmCmd struct {
}

type LlCmd struct {
	Colorless bool `arg:"-C"`
}

var args struct {
	Add *AddCmd `arg:"subcommand:add"`
	Rm  *RmCmd  `arg:"subcommand:rm"`
	Ll  *LlCmd  `arg:"subcommand:ll"`
}

func main() {
	arg.MustParse(&args)
	switch {
	case args.Ll != nil:
		println(args.Ll.Colorless)
		ui.ShowSummary()
	default:
		ui.ShowSummary()
	}
}
