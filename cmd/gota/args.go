package main

import (
	"fmt"
	"strings"
)

// parseArgs splits argv into positionals; bool/string flags recognized at any
// position (argparse-like). Map keys are aliases ("-n", "--dry-run").
// Supports --flag=value for string flags.
func parseArgs(argv []string, bools map[string]*bool, strs map[string]*string) ([]string, error) {
	var pos []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if p, ok := bools[a]; ok {
			*p = true
			continue
		}
		if p, ok := strs[a]; ok {
			if i+1 >= len(argv) {
				return nil, fmt.Errorf("argument %s: expected a value", a)
			}
			i++
			*p = argv[i]
			continue
		}
		if name, val, found := strings.Cut(a, "="); found {
			if p, ok := strs[name]; ok {
				*p = val
				continue
			}
		}
		if len(a) > 1 && strings.HasPrefix(a, "-") {
			return nil, fmt.Errorf("unrecognized arguments: %s", a)
		}
		pos = append(pos, a)
	}
	return pos, nil
}

// stripLeadingFlags removes given flags from the front of argv (REMAINDER
// semantics: flags after the first positional stay in the remainder).
func stripLeadingFlags(argv []string, names ...string) ([]string, bool) {
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	found := false
	i := 0
	for i < len(argv) && set[argv[i]] {
		found = true
		i++
	}
	return argv[i:], found
}
