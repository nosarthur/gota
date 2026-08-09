package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// isolate points config at a temp dir and clears caches
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GOTA_PROJECT_HOME", dir)
	clearConfigCaches()
	t.Cleanup(clearConfigCaches)
	return filepath.Join(dir, "gota")
}

func gitInit(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", path).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
}

func TestFormatOutput(t *testing.T) {
	cases := []struct{ in, prefix, want string }{
		{"a\nb\n", "repo", "repo: a\nrepo: b\n"},
		{"a", "repo", "repo: a"},
		{"", "repo", ""},
	}
	for _, c := range cases {
		if got := formatOutput(c.in, c.prefix); got != c.want {
			t.Errorf("formatOutput(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestRelativePath(t *testing.T) {
	if _, ok := relativePath("/a/b/c", ""); ok {
		t.Error("empty parent should not match")
	}
	if _, ok := relativePath("/a/b", "/a/b/c"); ok {
		t.Error("kid above parent should not match")
	}
	rel, ok := relativePath("/a/b/c/d", "/a/b")
	if !ok || !reflect.DeepEqual(rel, []string{"c", "d"}) {
		t.Errorf("got %v %v", rel, ok)
	}
	rel, ok = relativePath("/a/b", "/a/b")
	if !ok || len(rel) != 0 {
		t.Errorf("same path: got %v %v", rel, ok)
	}
}

func TestMakeName(t *testing.T) {
	repos := newRepos()
	counts := map[string]int{"repo": 1}
	if got := makeName("/a/b/repo", repos, counts); got != "repo" {
		t.Errorf("got %q", got)
	}
	counts["repo"] = 2
	if got := makeName("/a/b/repo", repos, counts); got != "b/repo" {
		t.Errorf("collision: got %q", got)
	}
	counts["repo"] = 1
	repos.Set("repo", &Repo{Path: "/x/repo"})
	if got := makeName("/a/b/repo", repos, counts); got != "b/repo" {
		t.Errorf("existing name: got %q", got)
	}
}

func TestIsGit(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "r1")
	gitInit(t, repo)
	if !isGit(repo, false, false) {
		t.Error("plain repo not detected")
	}
	if isGit(filepath.Join(base, "nope"), true, false) {
		t.Error("missing path detected as repo")
	}
	plain := filepath.Join(base, "plaindir")
	os.MkdirAll(plain, 0o755)
	if isGit(plain, false, false) {
		t.Error("plain dir detected as repo")
	}
	// bare repo
	bare := filepath.Join(base, "bare.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v %s", err, out)
	}
	if isGit(bare, false, false) {
		t.Error("bare repo detected without includeBare")
	}
	if !isGit(bare, true, false) {
		t.Error("bare repo not detected with includeBare")
	}
}

func TestReposRoundTrip(t *testing.T) {
	isolate(t)
	repos := newRepos()
	repos.Set("r1", &Repo{Path: "/a/r1", Flags: []string{"--work-tree=/a"}})
	repos.Set("r2", &Repo{Path: "/b/r2", Type: "w"})
	writeToRepoFile(repos, fileWrite)
	got := getRepos(true)
	if !reflect.DeepEqual(got.Names(), []string{"r1", "r2"}) {
		t.Fatalf("names: %v", got.Names())
	}
	r1 := got.Get("r1")
	if r1.Path != "/a/r1" || !reflect.DeepEqual(r1.Flags, []string{"--work-tree=/a"}) {
		t.Errorf("r1: %+v", r1)
	}
	if got.Get("r2").Type != "w" {
		t.Errorf("r2 type: %+v", got.Get("r2"))
	}
}

func TestGroupsRoundTrip(t *testing.T) {
	base := t.TempDir()
	isolate(t)
	r1 := filepath.Join(base, "r1")
	r2 := filepath.Join(base, "r2")
	gitInit(t, r1)
	gitInit(t, r2)
	repos := newRepos()
	repos.Set("r1", &Repo{Path: r1})
	repos.Set("r2", &Repo{Path: r2})
	writeToRepoFile(repos, fileWrite)

	groups := newGroups()
	groups.Set("g1", &Group{Repos: []string{"r1", "r2"}, Path: base})
	groups.Set("g2", &Group{Repos: []string{"r2", "ghost"}, Path: ""})
	writeToGroupsFile(groups, fileWrite)

	got := getGroups()
	if !reflect.DeepEqual(got.Names(), []string{"g1", "g2"}) {
		t.Fatalf("names: %v", got.Names())
	}
	if !reflect.DeepEqual(got.Get("g1").Repos, []string{"r1", "r2"}) {
		t.Errorf("g1: %+v", got.Get("g1"))
	}
	if got.Get("g1").Path != base {
		t.Errorf("g1 path: %q", got.Get("g1").Path)
	}
	// unknown repo filtered on read
	if !reflect.DeepEqual(got.Get("g2").Repos, []string{"r2"}) {
		t.Errorf("g2: %+v", got.Get("g2"))
	}
}

func TestAddRepos(t *testing.T) {
	base := t.TempDir()
	isolate(t)
	r1 := filepath.Join(base, "r1")
	gitInit(t, r1)
	plain := filepath.Join(base, "plain")
	os.MkdirAll(plain, 0o755)

	added := addRepos(getRepos(false), []string{r1, plain}, false, false, false)
	if !reflect.DeepEqual(added.Names(), []string{"r1"}) {
		t.Fatalf("added: %v", added.Names())
	}
	// second add: no new repos
	added = addRepos(getRepos(false), []string{r1}, false, false, false)
	if added.Len() != 0 {
		t.Errorf("re-add: %v", added.Names())
	}
	if !getRepos(false).Has("r1") {
		t.Error("r1 not persisted")
	}
}

func TestAddReposNameCollision(t *testing.T) {
	base := t.TempDir()
	isolate(t)
	a := filepath.Join(base, "x", "repo")
	b := filepath.Join(base, "y", "repo")
	gitInit(t, a)
	gitInit(t, b)
	added := addRepos(getRepos(false), []string{a, b}, false, false, false)
	want := []string{"x/repo", "y/repo"}
	if !reflect.DeepEqual(added.SortedNames(), want) {
		t.Errorf("got %v want %v", added.SortedNames(), want)
	}
}

func TestGenerateDirHash(t *testing.T) {
	hash, head := generateDirHash("/a/b/c/d/here", []string{"/a/b"})
	if !reflect.DeepEqual(hash, []string{"b", "c", "d"}) || head != "/a" {
		t.Errorf("got %v %q", hash, head)
	}
	hash, _ = generateDirHash("/z/repo", []string{"/a/b"})
	if hash != nil {
		t.Errorf("non-relative: got %v", hash)
	}
	// first path not a parent: falls through to second (upstream crashed here)
	hash, head = generateDirHash("/a/b/c/repo", []string{"/zzz", "/a/b"})
	if !reflect.DeepEqual(hash, []string{"b", "c"}) || head != "/a" {
		t.Errorf("got %v %q", hash, head)
	}
}

func TestAutoGroup(t *testing.T) {
	repos := newRepos()
	repos.Set("r1", &Repo{Path: "/a/b/c/r1"})
	repos.Set("r2", &Repo{Path: "/a/b/r2"})
	groups := autoGroup(repos, []string{"/a/b"})
	if !reflect.DeepEqual(groups.Names(), []string{"b", "b-c"}) {
		t.Fatalf("groups: %v", groups.Names())
	}
	if !reflect.DeepEqual(groups.Get("b").Repos, []string{"r1", "r2"}) {
		t.Errorf("b: %+v", groups.Get("b"))
	}
	if !reflect.DeepEqual(groups.Get("b-c").Repos, []string{"r1"}) {
		t.Errorf("b-c: %+v", groups.Get("b-c"))
	}
	if groups.Get("b-c").Path != "/a/b/c" {
		t.Errorf("b-c path: %q", groups.Get("b-c").Path)
	}
}

func TestTruncate(t *testing.T) {
	tr := &truncator{widths: map[string]int{"branch": 5, "path": 0}}
	if got := tr.truncate("branch", "verylongbranch"); got != "ve..." {
		t.Errorf("got %q", got)
	}
	if got := tr.truncate("branch", "ab"); got != "ab   " {
		t.Errorf("pad: got %q", got)
	}
	if got := tr.truncate("path", "whatever/long"); got != "whatever/long" {
		t.Errorf("no limit: got %q", got)
	}
	if got := tr.truncate("missing", "xyz"); got != "xyz" {
		t.Errorf("missing field: got %q", got)
	}
}

func TestParseReposAndRest(t *testing.T) {
	base := t.TempDir()
	isolate(t)
	r1 := filepath.Join(base, "r1")
	r2 := filepath.Join(base, "r2")
	gitInit(t, r1)
	gitInit(t, r2)
	repos := newRepos()
	repos.Set("r1", &Repo{Path: r1})
	repos.Set("r2", &Repo{Path: r2})
	writeToRepoFile(repos, fileWrite)
	groups := newGroups()
	groups.Set("g1", &Group{Repos: []string{"r1", "r2"}})
	writeToGroupsFile(groups, fileWrite)

	// leading repo name, rest is command
	chosen, rest := parseReposAndRest([]string{"r1", "status", "-s"}, false)
	if !reflect.DeepEqual(chosen.Names(), []string{"r1"}) {
		t.Errorf("chosen: %v", chosen.Names())
	}
	if !reflect.DeepEqual(rest, []string{"status", "-s"}) {
		t.Errorf("rest: %v", rest)
	}

	// group expands
	chosen, rest = parseReposAndRest([]string{"g1", "pull"}, false)
	if !reflect.DeepEqual(chosen.Names(), []string{"r1", "r2"}) {
		t.Errorf("group chosen: %v", chosen.Names())
	}
	if !reflect.DeepEqual(rest, []string{"pull"}) {
		t.Errorf("group rest: %v", rest)
	}

	// no names: all repos
	chosen, rest = parseReposAndRest([]string{"pull"}, false)
	if chosen.Len() != 2 {
		t.Errorf("all chosen: %v", chosen.Names())
	}
	if !reflect.DeepEqual(rest, []string{"pull"}) {
		t.Errorf("all rest: %v", rest)
	}

	// all input are names → empty rest
	chosen, rest = parseReposAndRest([]string{"r1", "r2"}, false)
	if !reflect.DeepEqual(chosen.Names(), []string{"r1", "r2"}) || len(rest) != 0 {
		t.Errorf("names only: %v %v", chosen.Names(), rest)
	}
}

func TestParseCloneConfig(t *testing.T) {
	dir := t.TempDir()
	fname := filepath.Join(dir, "freeze.csv")
	content := strings.Join([]string{
		"git@host:u/r1.git,r1,/a/r1,,,main",
		"git@host:u/r2.git,r2,/a/r2,,--bare,",
		",g1,/a,r1|r2|ghost",
	}, "\n") + "\n"
	os.WriteFile(fname, []byte(content), 0o644)
	names, repos, groups := parseCloneConfig(fname)
	if !reflect.DeepEqual(names, []string{"r1", "r2"}) {
		t.Fatalf("names: %v", names)
	}
	if repos["r1"].Branch != "main" || repos["r1"].URL != "git@host:u/r1.git" {
		t.Errorf("r1: %+v", repos["r1"])
	}
	if !reflect.DeepEqual(repos["r2"].Flags, []string{"--bare"}) {
		t.Errorf("r2 flags: %+v", repos["r2"])
	}
	if !reflect.DeepEqual(groups.Get("g1").Repos, []string{"r1", "r2"}) {
		t.Errorf("g1: %+v", groups.Get("g1"))
	}
}

func TestDeleteRepoFromGroups(t *testing.T) {
	groups := newGroups()
	groups.Set("g1", &Group{Repos: []string{"r1", "r2"}})
	groups.Set("g2", &Group{Repos: []string{"r2"}})
	if deleteRepoFromGroups("nope", groups) {
		t.Error("deleted nonexistent")
	}
	if !deleteRepoFromGroups("r2", groups) {
		t.Error("r2 not deleted")
	}
	if !reflect.DeepEqual(groups.Get("g1").Repos, []string{"r1"}) {
		t.Errorf("g1: %+v", groups.Get("g1"))
	}
	if len(groups.Get("g2").Repos) != 0 {
		t.Errorf("g2: %+v", groups.Get("g2"))
	}
}

func TestRenameRepo(t *testing.T) {
	base := t.TempDir()
	isolate(t)
	r1 := filepath.Join(base, "r1")
	r2 := filepath.Join(base, "r2")
	gitInit(t, r1)
	gitInit(t, r2)
	repos := newRepos()
	repos.Set("r1", &Repo{Path: r1})
	repos.Set("r2", &Repo{Path: r2})
	writeToRepoFile(repos, fileWrite)
	groups := newGroups()
	groups.Set("g1", &Group{Repos: []string{"r1", "r2"}})
	writeToGroupsFile(groups, fileWrite)

	renameRepo(getRepos(false), "r2", "zz")
	got := getRepos(false)
	if got.Has("r2") || !got.Has("zz") {
		t.Errorf("repos: %v", got.Names())
	}
	if !reflect.DeepEqual(getGroups().Get("g1").Repos, []string{"r1", "zz"}) {
		t.Errorf("group members: %v", getGroups().Get("g1").Repos)
	}
}

func TestGetContextAuto(t *testing.T) {
	base, _ := filepath.EvalSymlinks(t.TempDir()) // cwd is symlink-resolved on macOS
	cfg := isolate(t)
	r1 := filepath.Join(base, "sub", "r1")
	gitInit(t, r1)
	repos := newRepos()
	repos.Set("r1", &Repo{Path: r1})
	writeToRepoFile(repos, fileWrite)
	groups := newGroups()
	groups.Set("g1", &Group{Repos: []string{"r1"}, Path: filepath.Join(base, "sub")})
	writeToGroupsFile(groups, fileWrite)

	os.MkdirAll(cfg, 0o755)
	os.WriteFile(filepath.Join(cfg, "auto.context"), nil, 0o644)
	clearConfigCaches()

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(r1)
	if ctx := getContext(); ctxStem(ctx) != "g1" {
		t.Errorf("auto ctx: %q", ctx)
	}
	clearConfigCaches()
	os.Chdir(base) // outside any group path... base is parent of sub; not relative? sub is under base? group path is base/sub; cwd=base not under it
	if ctx := getContext(); ctx != "" {
		t.Errorf("expected no ctx, got %q", ctx)
	}
}

func TestDescribeSmoke(t *testing.T) {
	base := t.TempDir()
	isolate(t)
	r1 := filepath.Join(base, "r1")
	gitInit(t, r1)
	run := func(args ...string) {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = r1
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v %s", args, err, out)
		}
	}
	run("git", "config", "user.email", "t@t")
	run("git", "config", "user.name", "t")
	run("git", "commit", "--allow-empty", "-m", "init commit")
	repos := newRepos()
	repos.Set("r1", &Repo{Path: r1})
	lines := describe(repos, true)
	if len(lines) != 1 {
		t.Fatalf("lines: %v", lines)
	}
	if !strings.HasPrefix(lines[0], "r1 ") {
		t.Errorf("line: %q", lines[0])
	}
	if !strings.Contains(lines[0], "init commit") {
		t.Errorf("missing commit msg: %q", lines[0])
	}
	if !strings.Contains(lines[0], "∅") { // no remote
		t.Errorf("missing no-remote symbol: %q", lines[0])
	}
}
