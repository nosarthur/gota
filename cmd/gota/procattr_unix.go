//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// detachFromTTY: new session so async git can't grab the terminal for
// credential prompts; failures are re-run synchronously instead
func detachFromTTY(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
