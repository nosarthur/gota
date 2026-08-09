package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	contextCache  string
	contextCached bool
)

// ctxStem: group name from a context file path
func ctxStem(ctxPath string) string {
	return strings.TrimSuffix(filepath.Base(ctxPath), ".context")
}

// getContext returns the context file path, or "" if unset. In auto mode the
// group with minimal path distance to cwd is resolved (may be none → "").
func getContext() string {
	if contextCached {
		return contextCache
	}
	dir := configDir()
	matches, _ := filepath.Glob(filepath.Join(dir, "*.context"))
	// python glob skips dotfiles; Go doesn't
	var files []string
	for _, m := range matches {
		if !strings.HasPrefix(filepath.Base(m), ".") {
			files = append(files, m)
		}
	}
	if len(files) > 1 {
		fmt.Println("Cannot have multiple .context file")
		os.Exit(1)
	}
	ctx := ""
	if len(files) == 1 {
		ctx = files[0]
		if ctxStem(ctx) == "auto" {
			cwd, _ := os.Getwd()
			candidate := ""
			minDist := int(^uint(0) >> 1)
			groups := getGroups()
			for _, gname := range groups.Names() {
				rel, ok := relativePath(cwd, groups.Get(gname).Path)
				if !ok {
					continue
				}
				if len(rel) < minDist {
					candidate = gname
					minDist = len(rel)
				}
			}
			if candidate == "" {
				ctx = ""
			} else {
				ctx = filepath.Join(dir, candidate+".context")
			}
		}
	}
	contextCache, contextCached = ctx, true
	return ctx
}

// replaceContext sets/renames/removes the context file. new "none" (or "")
// deletes the context.
func replaceContext(old, new string) {
	dir := configDir()
	os.MkdirAll(dir, 0o755)
	auto := filepath.Join(dir, "auto.context")
	if exists(auto) {
		old = auto
	}
	switch {
	case new == "none" || new == "":
		if old != "" {
			os.Remove(old)
		}
	case old != "":
		os.Rename(old, filepath.Join(dir, new+".context"))
	default:
		os.WriteFile(filepath.Join(dir, new+".context"), nil, 0o644)
	}
	contextCached = false
	contextCache = ""
}
