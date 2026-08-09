// Command gota manages multiple git repos: display side-by-side status and
// delegate git commands from any working directory.
// Go rewrite of https://github.com/nosarthur/gita (v0.16.8.2).
package main

import (
	"fmt"
	"os"
	"sort"
)

const version = "0.16.8.2-go"

var builtinHelp = []struct{ name, help string }{
	{"add", "add repo(s)"},
	{"rm", "remove repo(s)"},
	{"freeze", "print all repo information"},
	{"clone", "clone repos"},
	{"rename", "rename a repo"},
	{"flags", "git flags configuration"},
	{"color", "color configuration"},
	{"info", "information setting"},
	{"ll", "display summary of all repos"},
	{"context", "set context"},
	{"ls", "show repo(s) or repo path"},
	{"group", "group repos"},
	{"super", "run any git command/alias"},
	{"shell", "run any shell command"},
	{"clear", "removes all groups and repositories"},
}

func printHelp() {
	fmt.Println(`gota manages multiple git repos. It has two functionalities

   1. display the status of multiple repos side by side
   2. delegate git commands/aliases from any working directory

Examples:
    gota ls
    gota fetch
    gota stat myrepo2
    gota super myrepo1 commit -am 'add some cool feature'

usage: gota [-v] <sub-command> [args]

sub-commands:`)
	for _, c := range builtinHelp {
		fmt.Printf("    %-10s %s\n", c.name, c.help)
	}
	cmds := getCmds()
	names := make([]string, 0, len(cmds))
	for n := range cmds {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Println("\ndelegated git sub-commands (custom ones via ~/.config/gota/cmds.json):")
	for _, n := range names {
		fmt.Printf("    %-10s %s\n", n, cmds[n].Help)
	}
	fmt.Println(`
ll status symbols:
    +: staged changes  *: unstaged changes  ?: untracked files/folders  $: stashed changes

ll branch colors:
    white: no remote  green: in sync  red: diverged  purple: ahead  yellow: behind`)
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printHelp()
		return
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "-v", "--version":
		fmt.Println("gota " + version)
	case "-h", "--help":
		printHelp()
	case "add":
		fAdd(rest)
	case "rm":
		fRm(rest)
	case "freeze":
		fFreeze(rest)
	case "clone":
		fClone(rest)
	case "rename":
		fRename(rest)
	case "flags":
		fFlags(rest)
	case "color":
		fColor(rest)
	case "info":
		fInfo(rest)
	case "ll":
		fLl(rest)
	case "context":
		fContext(rest)
	case "ls":
		fLs(rest)
	case "group":
		fGroup(rest)
	case "super":
		fSuper(rest)
	case "shell":
		fShell(rest)
	case "clear":
		fClear(rest)
	default:
		if def, ok := getCmds()[sub]; ok {
			fDelegated(sub, def, rest)
		} else {
			fmt.Fprintf(os.Stderr, "gota: invalid sub-command: %q\n", sub)
			os.Exit(2)
		}
	}
}
