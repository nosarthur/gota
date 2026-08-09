package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Repo struct {
	Path  string
	Type  string
	Flags []string
}

// Repos: ordered map, preserves repos.csv file order (python dict order).
type Repos struct {
	names []string
	m     map[string]*Repo
}

func newRepos() *Repos { return &Repos{m: map[string]*Repo{}} }

func (r *Repos) Has(name string) bool  { _, ok := r.m[name]; return ok }
func (r *Repos) Get(name string) *Repo { return r.m[name] }
func (r *Repos) Names() []string       { return r.names }
func (r *Repos) Len() int              { return len(r.names) }

func (r *Repos) Set(name string, p *Repo) {
	if _, ok := r.m[name]; !ok {
		r.names = append(r.names, name)
	}
	r.m[name] = p
}

func (r *Repos) Delete(name string) {
	if _, ok := r.m[name]; !ok {
		return
	}
	delete(r.m, name)
	for i, n := range r.names {
		if n == name {
			r.names = append(r.names[:i], r.names[i+1:]...)
			break
		}
	}
}

func (r *Repos) SortedNames() []string {
	s := append([]string(nil), r.names...)
	sort.Strings(s)
	return s
}

var reposCache = map[bool]*Repos{}

// getRepos reads repos.csv (path,name,type,flags). skipValidation keeps
// entries whose path is not a git repo anymore.
func getRepos(skipValidation bool) *Repos {
	if r, ok := reposCache[skipValidation]; ok {
		return r
	}
	r := newRepos()
	rows := readCSV(configFname("repos.csv"), ',')
	for _, rec := range rows {
		rec = padRecord(rec, 4)
		path, name, typ, flags := rec[0], rec[1], rec[2], rec[3]
		if skipValidation || isGit(path, true, false) {
			r.Set(name, &Repo{Path: path, Type: typ, Flags: strings.Fields(flags)})
		}
	}
	reposCache[skipValidation] = r
	return r
}

func readCSV(fname string, comma rune) [][]string {
	f, err := os.Open(fname)
	if err != nil {
		return nil
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = comma
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		return nil
	}
	return rows
}

func padRecord(rec []string, n int) []string {
	for len(rec) < n {
		rec = append(rec, "")
	}
	return rec
}

const (
	fileWrite  = "w"
	fileAppend = "a+"
)

func openConfigFile(fname, mode string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(fname), 0o755); err != nil {
		return nil, err
	}
	flag := os.O_WRONLY | os.O_CREATE
	if mode == fileAppend {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	return os.OpenFile(fname, flag, 0o644)
}

func writeToRepoFile(repos *Repos, mode string) {
	fname := configFname("repos.csv")
	f, err := openConfigFile(fname, mode)
	if err != nil {
		die("cannot write %s: %v", fname, err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	for _, name := range repos.Names() {
		p := repos.Get(name)
		w.Write([]string{p.Path, name, p.Type, strings.Join(p.Flags, " ")})
	}
	w.Flush()
	clearConfigCaches()
}

// isSubmoduleRepo: .git file pointing into a superproject's .git/modules
func isSubmoduleRepo(gitPath string) bool {
	data, err := os.ReadFile(gitPath)
	return err == nil && strings.Contains(string(data), ".git/modules")
}

// isGit reports whether path is a git repo (.git dir or file; optionally bare).
func isGit(path string, includeBare, excludeSubmodule bool) bool {
	if !exists(path) {
		return false
	}
	loc := filepath.Join(path, ".git")
	if fi, err := os.Stat(loc); err == nil {
		if excludeSubmodule && fi.Mode().IsRegular() && isSubmoduleRepo(loc) {
			return false
		}
		return true
	}
	if !includeBare {
		return false
	}
	cmd := exec.Command("git", "rev-parse", "--is-bare-repository")
	cmd.Dir = path
	out, err := cmd.Output()
	return err == nil && string(out) == "true\n"
}

// makeName: basename; on collision include parent dir name
func makeName(path string, repos *Repos, nameCounts map[string]int) string {
	name := filepath.Base(filepath.Clean(path))
	if repos.Has(name) || nameCounts[name] > 1 {
		parName := filepath.Base(filepath.Dir(filepath.Clean(path)))
		return filepath.Join(parName, name)
	}
	return name
}

// addRepos appends new repo paths to repos.csv; returns the added repos.
func addRepos(repos *Repos, newPaths []string, includeBare, excludeSubmodule, dryRun bool) *Repos {
	existing := map[string]bool{}
	for _, n := range repos.Names() {
		existing[repos.Get(n).Path] = true
	}
	seen := map[string]bool{}
	var fresh []string
	for _, p := range newPaths {
		if seen[p] || existing[p] || !isGit(p, includeBare, excludeSubmodule) {
			continue
		}
		seen[p] = true
		fresh = append(fresh, p)
	}
	sort.Strings(fresh)
	added := newRepos()
	if len(fresh) == 0 {
		fmt.Println("No new repos found!")
		return added
	}
	fmt.Printf("Found %d new repo(s).\n", len(fresh))
	if dryRun {
		for _, p := range fresh {
			fmt.Println(p)
		}
		return newRepos()
	}
	nameCounts := map[string]int{}
	for _, p := range fresh {
		nameCounts[filepath.Base(filepath.Clean(p))]++
	}
	for _, p := range fresh {
		added.Set(makeName(p, repos, nameCounts), &Repo{Path: p})
	}
	writeToRepoFile(added, fileAppend)
	return added
}

func renameRepo(repos *Repos, repo, newName string) {
	if repos.Has(newName) {
		fmt.Printf("%s is already in use!\n", newName)
		return
	}
	groups := getGroups() // read before cache invalidation
	prop := repos.Get(repo)
	repos.Delete(repo)
	repos.Set(newName, prop)
	writeToRepoFile(repos, fileWrite)

	for _, g := range groups.Names() {
		members := groups.Get(g).Repos
		for i, m := range members {
			if m == repo {
				members[i] = newName
				sort.Strings(members)
				groups.Get(g).Repos = members
				break
			}
		}
	}
	writeToGroupsFile(groups, fileWrite)
}

// relativePath returns path components of kid relative to parent, or ok=false.
func relativePath(kid, parent string) ([]string, bool) {
	if parent == "" {
		return nil, false
	}
	rel, err := filepath.Rel(parent, kid)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, false
	}
	if rel == "." {
		return []string{}, true
	}
	return strings.Split(rel, string(os.PathSeparator)), true
}

// generateDirHash: basename of the matching parent path + relative dir
// components (repo dir itself excluded); head is the parent path's dir.
func generateDirHash(repoPath string, paths []string) ([]string, string) {
	var rel []string
	matched := ""
	for _, p := range paths {
		r, ok := relativePath(repoPath, p)
		if ok {
			if len(r) > 0 {
				r = r[:len(r)-1]
			}
			rel = r
			matched = p
			break
		}
	}
	if matched == "" {
		return nil, ""
	}
	head := filepath.Dir(filepath.Clean(matched))
	tail := filepath.Base(filepath.Clean(matched))
	return append([]string{tail}, rel...), head
}

// autoGroup creates hierarchical groups from folder structure.
func autoGroup(repos *Repos, paths []string) *Groups {
	groups := newGroups()
	for _, repoName := range repos.Names() {
		hash, head := generateDirHash(repos.Get(repoName).Path, paths)
		if len(hash) == 0 {
			continue
		}
		for i := 1; i <= len(hash); i++ {
			groupName := strings.Join(hash[:i], "-")
			g := groups.Get(groupName)
			if g == nil {
				g = &Group{}
				groups.Set(groupName, g)
			}
			g.Path = filepath.Join(append([]string{head}, hash[:i]...)...)
			g.Repos = append(g.Repos, repoName)
		}
	}
	return groups
}
