package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

var colorCodes = map[string]string{
	"black":     "\x1b[30m",
	"red":       "\x1b[31m", // local diverges from remote
	"green":     "\x1b[32m", // local == remote
	"yellow":    "\x1b[33m", // local is behind
	"blue":      "\x1b[34m",
	"purple":    "\x1b[35m", // local is ahead
	"cyan":      "\x1b[36m",
	"white":     "\x1b[37m", // no remote branch
	"end":       "\x1b[0m",
	"b_black":   "\x1b[30;1m",
	"b_red":     "\x1b[31;1m",
	"b_green":   "\x1b[32;1m",
	"b_yellow":  "\x1b[33;1m",
	"b_blue":    "\x1b[34;1m",
	"b_purple":  "\x1b[35;1m",
	"b_cyan":    "\x1b[36;1m",
	"b_white":   "\x1b[37;1m",
	"underline": "\x1b[4m",
}

var colorEnumOrder = []string{
	"black", "red", "green", "yellow", "blue", "purple", "cyan", "white", "end",
	"b_black", "b_red", "b_green", "b_yellow", "b_blue", "b_purple", "b_cyan",
	"b_white", "underline",
}

var defaultColorSituations = []string{
	"no_remote", "in_sync", "diverged", "local_ahead", "remote_ahead",
}

var defaultColors = map[string]string{
	"no_remote":    "white",
	"in_sync":      "green",
	"diverged":     "red",
	"local_ahead":  "purple",
	"remote_ahead": "yellow",
}

var (
	colorEncOnce  sync.Once
	colorEncKeys  []string
	colorEncoding map[string]string
)

// getColorEncoding: situation → color name; keys keep header order for rewrite.
func getColorEncoding() ([]string, map[string]string) {
	colorEncOnce.Do(func() {
		rows := readCSV(configFname("color.csv"), ',')
		if len(rows) >= 2 {
			colorEncoding = map[string]string{}
			for i, k := range rows[0] {
				if i < len(rows[1]) {
					colorEncKeys = append(colorEncKeys, k)
					colorEncoding[k] = rows[1][i]
				}
			}
		} else {
			colorEncKeys = defaultColorSituations
			colorEncoding = defaultColors
		}
	})
	return colorEncKeys, colorEncoding
}

func showColors() {
	for i, name := range colorEnumOrder {
		if name != "end" && name != "underline" {
			fmt.Printf("%s%-8s ", colorCodes[name], name)
		}
		if (i+1)%9 == 0 {
			fmt.Println()
		}
	}
	fmt.Println(colorCodes["end"])
	keys, enc := getColorEncoding()
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	for _, situ := range sorted {
		c := enc[situ]
		fmt.Printf("%-12s: %s%-8s%s \n", situ, colorCodes[c], c, colorCodes["end"])
	}
}

var defaultSymbols = map[string]string{
	"dirty":        "*",
	"staged":       "+",
	"untracked":    "?",
	"stashed":      "$",
	"local_ahead":  "↑",
	"remote_ahead": "↓",
	"diverged":     "⇕",
	"in_sync":      "",
	"no_remote":    "∅",
	"":             "",
}

var (
	symbolsOnce sync.Once
	symbols     map[string]string
)

func getSymbols() map[string]string {
	symbolsOnce.Do(func() {
		symbols = map[string]string{}
		for k, v := range defaultSymbols {
			symbols[k] = v
		}
		rows := readCSV(configFname("symbols.csv"), ',')
		if len(rows) >= 2 {
			for i, k := range rows[0] {
				if i < len(rows[1]) {
					symbols[k] = rows[1][i]
				}
			}
		}
	})
	return symbols
}

// truncator applies column widths from layout.csv; 0/missing = no limit.
type truncator struct {
	widths map[string]int
}

func newTruncator() *truncator {
	t := &truncator{widths: map[string]int{}}
	rows := readCSV(configFname("layout.csv"), ',')
	if len(rows) >= 2 {
		for i, k := range rows[0] {
			if i < len(rows[1]) {
				w, err := strconv.Atoi(strings.TrimSpace(rows[1][i]))
				if err == nil {
					t.widths[k] = w
				}
			}
		}
	}
	return t
}

func (t *truncator) truncate(field, message string) string {
	w := t.widths[field]
	if w == 0 {
		return message
	}
	if w < 3 {
		w = 3
	}
	r := []rune(message)
	if len(r) > w {
		return string(r[:w-3]) + "..."
	}
	return padRight(message, w)
}

func padRight(s string, w int) string {
	n := utf8.RuneCountInString(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

var allInfoItems = []string{"branch", "branch_name", "commit_msg", "commit_time", "path"}

func isInfoItem(x string) bool {
	for _, i := range allInfoItems {
		if i == x {
			return true
		}
	}
	return false
}

// getInfoItems: items shown by `gita ll`, from info.csv or defaults.
func getInfoItems() []string {
	rows := readCSV(configFname("info.csv"), ',')
	if len(rows) >= 1 {
		var items []string
		for _, x := range rows[0] {
			if isInfoItem(x) {
				items = append(items, x)
			}
		}
		return items
	}
	return []string{"branch", "commit_msg", "commit_time"}
}

type infoFunc func(*Repo, *truncator) string

func getInfoFuncs(noColors bool) []infoFunc {
	m := map[string]infoFunc{
		"branch": func(p *Repo, t *truncator) string {
			return getRepoStatus(p, t, noColors)
		},
		"branch_name": getRepoBranch,
		"commit_msg":  getCommitMsg,
		"commit_time": getCommitTime,
		"path":        getPath,
	}
	var funcs []infoFunc
	for _, k := range getInfoItems() {
		funcs = append(funcs, m[k])
	}
	return funcs
}

func getPath(p *Repo, t *truncator) string {
	return colorCodes["cyan"] + t.truncate("path", p.Path) + colorCodes["end"]
}

// gitStdout runs git in dir, returns trimmed stdout ("" on error).
func gitStdout(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

// getHead: current branch, or exact tag on detached HEAD
func getHead(path string) string {
	head := gitStdout(path, "symbolic-ref", "-q", "--short", "HEAD")
	if head == "" {
		head = gitStdout(path, "describe", "--tags", "--exact-match")
	}
	return head
}

// runQuietDiff: exit code of git <flags> diff --quiet <args>
func runQuietDiff(flags, args []string, path string) int {
	all := append(append(append([]string{}, flags...), "diff", "--quiet"), args...)
	cmd := exec.Command("git", all...)
	cmd.Dir = path
	err := cmd.Run()
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 1
}

func getCommonCommit(path string) string {
	return gitStdout(path, "merge-base", "@{0}", "@{u}")
}

func hasUntracked(flags []string, path string) bool {
	all := append(append([]string{}, flags...), "ls-files", "-zo", "--exclude-standard")
	cmd := exec.Command("git", all...)
	cmd.Dir = path
	out, _ := cmd.Output()
	return len(out) > 0
}

func hasStashed(path string) bool {
	return isFile(filepath.Join(path, ".git", "logs", "refs", "stash"))
}

func getCommitMsg(p *Repo, t *truncator) string {
	all := append(append([]string{}, p.Flags...), "show-branch", "--no-name", "HEAD")
	return t.truncate("commit_msg", gitStdout(p.Path, all...))
}

func getCommitTime(p *Repo, t *truncator) string {
	all := append(append([]string{}, p.Flags...), "log", "-1", "--format=%cd", "--date=relative")
	return t.truncate("commit_time", "("+gitStdout(p.Path, all...)+")")
}

func getRepoBranch(p *Repo, t *truncator) string {
	return t.truncate("branch_name", getHead(p.Path))
}

// repoStatus returns symbol keys: dirty staged untracked stashed situation
func repoStatus(p *Repo) (string, string, string, string, string) {
	path, flags := p.Path, p.Flags
	dirty, staged, untracked, stashed := "", "", "", ""
	if runQuietDiff(flags, nil, path) != 0 {
		dirty = "dirty"
	}
	if runQuietDiff(flags, []string{"--cached"}, path) != 0 {
		staged = "staged"
	}
	if hasUntracked(flags, path) {
		untracked = "untracked"
	}
	if hasStashed(path) {
		stashed = "stashed"
	}
	var situ string
	switch rc := runQuietDiff(flags, []string{"@{u}", "@{0}"}, path); rc {
	case 128:
		situ = "no_remote"
	case 0:
		situ = "in_sync"
	default:
		commonCommit := getCommonCommit(path)
		if runQuietDiff(flags, []string{"@{u}", commonCommit}, path) != 0 {
			if runQuietDiff(flags, []string{"@{0}", commonCommit}, path) != 0 {
				situ = "diverged"
			} else {
				situ = "remote_ahead"
			}
		} else {
			situ = "local_ahead"
		}
	}
	return dirty, staged, untracked, stashed, situ
}

func getRepoStatus(p *Repo, t *truncator, noColors bool) string {
	branch := t.truncate("branch", getHead(p.Path))
	dirty, staged, untracked, stashed, situ := repoStatus(p)
	sym := getSymbols()
	symField := t.truncate("symbols",
		"["+sym[dirty]+sym[staged]+sym[stashed]+sym[untracked]+sym[situ]+"]")
	info := padRight(branch, 10) + " " + symField
	if noColors {
		return padRight(info, 18)
	}
	_, enc := getColorEncoding()
	return colorCodes[enc[situ]] + padRight(info, 18) + colorCodes["end"]
}

// describe renders one ll line per repo, computed concurrently, sorted by name.
func describe(repos *Repos, noColors bool) []string {
	names := repos.SortedNames()
	if len(names) == 0 {
		return nil
	}
	t := newTruncator()
	getSymbols()
	getColorEncoding()
	width := 0
	for _, n := range names {
		if w := utf8.RuneCountInString(n); w > width {
			width = w
		}
	}
	width++
	funcs := getInfoFuncs(noColors)
	lines := make([]string, len(names))
	sem := make(chan struct{}, max(1, min(runtime.NumCPU(), len(names))))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			parts := make([]string, len(funcs))
			for j, f := range funcs {
				parts[j] = f(repos.Get(name), t)
			}
			lines[i] = padRight(name, width) + strings.Join(parts, " ")
		}(i, name)
	}
	wg.Wait()
	return lines
}
