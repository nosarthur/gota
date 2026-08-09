package main

import (
	"encoding/csv"
	"sort"
	"strings"
)

type Group struct {
	Repos []string
	Path  string
}

// Groups: ordered map, preserves groups.csv file order.
type Groups struct {
	names []string
	m     map[string]*Group
}

func newGroups() *Groups { return &Groups{m: map[string]*Group{}} }

func (g *Groups) Has(name string) bool   { _, ok := g.m[name]; return ok }
func (g *Groups) Get(name string) *Group { return g.m[name] }
func (g *Groups) Names() []string        { return g.names }
func (g *Groups) Len() int               { return len(g.names) }

func (g *Groups) Set(name string, grp *Group) {
	if _, ok := g.m[name]; !ok {
		g.names = append(g.names, name)
	}
	g.m[name] = grp
}

func (g *Groups) Delete(name string) {
	if _, ok := g.m[name]; !ok {
		return
	}
	delete(g.m, name)
	for i, n := range g.names {
		if n == name {
			g.names = append(g.names[:i], g.names[i+1:]...)
			break
		}
	}
}

var groupsCache *Groups

// getGroups reads groups.csv; each line is name:repo1 repo2:path.
// Repos unknown to gita are filtered out.
func getGroups() *Groups {
	if groupsCache != nil {
		return groupsCache
	}
	groups := newGroups()
	repos := getRepos(false)
	rows := readCSV(configFname("groups.csv"), ':')
	for _, rec := range rows {
		rec = padRecord(rec, 3)
		var members []string
		for _, r := range strings.Fields(rec[1]) {
			if repos.Has(r) {
				members = append(members, r)
			}
		}
		groups.Set(rec[0], &Group{Repos: members, Path: rec[2]})
	}
	groupsCache = groups
	return groups
}

func writeToGroupsFile(groups *Groups, mode string) {
	fname := configFname("groups.csv")
	f, err := openConfigFile(fname, mode)
	if err != nil {
		die("cannot write %s: %v", fname, err)
	}
	defer f.Close()
	if groups.Len() > 0 {
		w := csv.NewWriter(f)
		w.Comma = ':'
		for _, name := range groups.Names() {
			g := groups.Get(name)
			if len(g.Repos) == 0 { // drop empty groups
				continue
			}
			w.Write([]string{name, strings.Join(g.Repos, " "), g.Path})
		}
		w.Flush()
	}
	clearConfigCaches()
}

func deleteRepoFromGroups(repo string, groups *Groups) bool {
	deleted := false
	for _, name := range groups.Names() {
		g := groups.Get(name)
		for i, r := range g.Repos {
			if r == repo {
				g.Repos = append(g.Repos[:i], g.Repos[i+1:]...)
				deleted = true
				break
			}
		}
	}
	return deleted
}

func sortedUnion(a, b []string) []string {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	for _, x := range b {
		set[x] = true
	}
	out := make([]string, 0, len(set))
	for x := range set {
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}

func clearConfigCaches() {
	reposCache = map[bool]*Repos{}
	groupsCache = nil
	contextCached = false
	contextCache = ""
}
