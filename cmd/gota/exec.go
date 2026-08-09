package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
)

var printMu sync.Mutex

// formatOutput prepends "prefix: " to every line, keeping line ends.
func formatOutput(s, prefix string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	rest := s
	for len(rest) > 0 {
		i := strings.IndexByte(rest, '\n')
		var line string
		if i < 0 {
			line, rest = rest, ""
		} else {
			line, rest = rest[:i+1], rest[i+1:]
		}
		b.WriteString(prefix + ": " + line)
	}
	return b.String()
}

func buildCmd(cmdArgs []string, dir string, shellMode bool) *exec.Cmd {
	var c *exec.Cmd
	if shellMode {
		c = exec.Command("sh", "-c", strings.Join(cmdArgs, " "))
	} else {
		c = exec.Command(cmdArgs[0], cmdArgs[1:]...)
	}
	c.Dir = dir
	return c
}

// runSyncInteractive runs cmd in dir with inherited stdio.
func runSyncInteractive(cmdArgs []string, dir string, shellMode bool) {
	c := buildCmd(cmdArgs, dir, shellMode)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Run()
}

type repoTask struct {
	name      string
	dir       string
	cmd       []string
	shellMode bool
}

// runTask runs one task detached from the tty (no credential prompts),
// prints its output prefixed with the repo name; reports failure.
func runTask(t repoTask) bool {
	c := buildCmd(t.cmd, t.dir, t.shellMode)
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	var out, errb bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errb
	err := c.Run()
	printMu.Lock()
	if out.Len() > 0 {
		fmt.Println(formatOutput(out.String(), t.name))
	}
	if errb.Len() > 0 {
		fmt.Println(formatOutput(errb.String(), t.name))
	}
	printMu.Unlock()
	// stderr alone is not a failure signal (e.g. git fetch)
	return err != nil
}

// runTasks runs tasks concurrently; returns failed flags per task index.
func runTasks(tasks []repoTask) []bool {
	failed := make([]bool, len(tasks))
	sem := make(chan struct{}, max(2, runtime.NumCPU()*4))
	var wg sync.WaitGroup
	for i := range tasks {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			failed[i] = runTask(tasks[i])
		}(i)
	}
	wg.Wait()
	return failed
}

// gitCmd delegates cmd to each repo. Per-repo flags are inserted after "git".
// Async unless a single repo or cmd[1] is async-blacklisted; failed async
// runs are re-run synchronously (with user interaction possible).
func gitCmd(repos *Repos, cmd []string, shellMode bool) {
	names := repos.Names()
	if len(names) == 0 {
		return
	}
	perRepo := make([][]string, len(names))
	for i, n := range names {
		c := append([]string(nil), cmd...)
		p := repos.Get(n)
		if c[0] == "git" && len(p.Flags) > 0 {
			c = append(c[:1:1], append(append([]string(nil), p.Flags...), c[1:]...)...)
		}
		perRepo[i] = c
	}
	blacklisted := len(cmd) > 1 && asyncBlacklist()[cmd[1]]
	if len(names) == 1 || blacklisted {
		for i, n := range names {
			path := repos.Get(n).Path
			fmt.Println(path)
			runSyncInteractive(perRepo[i], path, shellMode)
		}
		return
	}
	tasks := make([]repoTask, len(names))
	for i, n := range names {
		tasks[i] = repoTask{name: n, dir: repos.Get(n).Path, cmd: perRepo[i], shellMode: shellMode}
	}
	failed := runTasks(tasks)
	for i, f := range failed {
		if f {
			fmt.Println(tasks[i].dir)
			runSyncInteractive(perRepo[i], tasks[i].dir, shellMode)
		}
	}
}
