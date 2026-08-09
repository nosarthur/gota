package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// configDir: $GOTA_PROJECT_HOME > $GITA_PROJECT_HOME > $XDG_CONFIG_HOME >
// ~/.config, plus /gota; falls back to an existing /gita dir (python-gita compat)
func configDir() string {
	root := os.Getenv("GOTA_PROJECT_HOME")
	if root == "" {
		root = os.Getenv("GITA_PROJECT_HOME")
	}
	if root == "" {
		root = os.Getenv("XDG_CONFIG_HOME")
	}
	if root == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, ".config")
	}
	gota := filepath.Join(root, "gota")
	if !exists(gota) {
		if gita := filepath.Join(root, "gita"); exists(gita) {
			return gita
		}
	}
	return gota
}

func configFname(name string) string { return filepath.Join(configDir(), name) }

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

// die: app-level error, exit 1 (python style: print + sys.exit(1))
func die(format string, a ...any) {
	fmt.Printf(format+"\n", a...)
	os.Exit(1)
}

// argErr: bad CLI usage, exit 2 (argparse style)
func argErr(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "gota: error: "+format+"\n", a...)
	os.Exit(2)
}

func absPath(p string) string {
	if p == "" {
		return ""
	}
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}
