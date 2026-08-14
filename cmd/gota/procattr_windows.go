//go:build windows

package main

import "os/exec"

// no tty detach on windows; stdin is already /dev/null-equivalent
func detachFromTTY(c *exec.Cmd) {}
