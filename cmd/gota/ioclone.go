package main

import "strings"

type CloneRepo struct {
	URL    string
	Path   string
	Type   string
	Branch string
	Flags  []string
}

// parseCloneConfig reads a `gita freeze` file. Repo lines are
// url,name,path,type,flags,branch; group lines have empty url and
// |-separated repos in the 4th column.
func parseCloneConfig(fname string) ([]string, map[string]*CloneRepo, *Groups) {
	var names []string
	repos := map[string]*CloneRepo{}
	groups := newGroups()
	for _, rec := range readCSV(fname, ',') {
		rec = padRecord(rec, 6)
		url, name, path, typ, flags, branch := rec[0], rec[1], rec[2], rec[3], rec[4], rec[5]
		if url != "" {
			if _, ok := repos[name]; !ok {
				names = append(names, name)
			}
			repos[name] = &CloneRepo{
				URL: url, Path: path, Type: typ, Branch: branch,
				Flags: strings.Fields(flags),
			}
		} else {
			var members []string
			for _, r := range strings.Split(typ, "|") {
				if _, ok := repos[r]; ok {
					members = append(members, r)
				}
			}
			groups.Set(name, &Group{Repos: members, Path: path})
		}
	}
	return names, repos, groups
}
