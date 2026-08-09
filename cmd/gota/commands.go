package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// groupNameCheck: valid new group name or exit
func groupNameCheck(name string, excludeOldNames bool) {
	if getRepos(false).Has(name) {
		die("Cannot use group name %s since it's a repo name.", name)
	}
	if excludeOldNames && getGroups().Has(name) {
		die("Cannot use group name %s since it's already in use.", name)
	}
	if name == "none" || name == "auto" {
		die("Cannot use group name %s since it's a reserved keyword.", name)
	}
}

func mustGroup(groups *Groups, name string) *Group {
	if !groups.Has(name) {
		argErr("argument group: invalid choice: %q", name)
	}
	return groups.Get(name)
}

func subsetRepos(repos *Repos, names []string) *Repos {
	out := newRepos()
	for _, k := range names {
		if repos.Has(k) {
			out.Set(k, repos.Get(k))
		}
	}
	return out
}

// parseReposAndRest splits input into chosen repos (leading repo/group names,
// groups expanded, context fallback) and the remaining words.
func parseReposAndRest(input []string, quoteMode bool) (*Repos, []string) {
	repos := getRepos(false)
	groups := getGroups()
	ctx := getContext()
	i := 0
	var names []string
	broke := false
	for idx, word := range input {
		i = idx
		if repos.Has(word) || groups.Has(word) {
			names = append(names, word)
		} else {
			broke = true
			break
		}
	}
	if !broke {
		i++
	}
	if i > len(input) {
		i = len(input)
	}
	if len(names) == 0 && ctx != "" {
		names = []string{ctxStem(ctx)}
	}
	if quoteMode && i+1 != len(input) {
		if i < len(input) {
			fmt.Println(input[i], "is not a repo or group")
		} else {
			fmt.Println("quote mode expects repo/group names followed by one quoted command")
		}
		os.Exit(2)
	}
	if len(names) > 0 {
		chosen := newRepos()
		for _, k := range names {
			if repos.Has(k) {
				chosen.Set(k, repos.Get(k))
			}
			if groups.Has(k) {
				for _, r := range groups.Get(k).Repos {
					chosen.Set(r, repos.Get(r))
				}
			}
		}
		repos = chosen
	}
	return repos, input[i:]
}

// walkDirs: every dir under paths (paths included), hidden dirs skipped
func walkDirs(paths []string) []string {
	var out []string
	for _, root := range paths {
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			if p != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			out = append(out, p)
			return nil
		})
	}
	return out
}

// addToGroupAfterAdd: `gita add -g` / `gita clone -g` group bookkeeping
func addToGroupAfterAdd(added *Repos, gname, gpath string) {
	if added.Len() == 0 || gname == "" {
		return
	}
	groups := getGroups()
	if !groups.Has(gname) {
		fmt.Printf("%s does not exists, creating it.\n", gname)
		names := append([]string(nil), added.Names()...)
		sort.Strings(names)
		ng := newGroups()
		ng.Set(gname, &Group{Repos: names, Path: gpath})
		writeToGroupsFile(ng, fileAppend)
	} else {
		g := groups.Get(gname)
		g.Repos = sortedUnion(g.Repos, added.Names())
		writeToGroupsFile(groups, fileWrite)
	}
	fmt.Printf("Added %d repos to the %s group\n", added.Len(), gname)
}

func fAdd(argv []string) {
	var dryRun, skipSub, recursive, auto, bare bool
	var group, gpath string
	pos, err := parseArgs(argv,
		map[string]*bool{
			"-n": &dryRun, "--dry-run": &dryRun,
			"-s": &skipSub, "--skip-submodule": &skipSub,
			"-r": &recursive, "--recursive": &recursive,
			"-a": &auto, "--auto-group": &auto,
			"-b": &bare, "--bare": &bare,
		},
		map[string]*string{"-g": &group, "--group": &group, "--group-path": &gpath})
	if err != nil {
		argErr("add: %v", err)
	}
	if len(pos) == 0 {
		argErr("add: the following arguments are required: paths")
	}
	n := 0
	for _, b := range []bool{recursive, auto, bare} {
		if b {
			n++
		}
	}
	if n > 1 {
		argErr("add: -r/-a/-b are mutually exclusive")
	}
	var paths []string
	for _, p := range pos {
		paths = append(paths, absPath(p))
	}
	gpath = absPath(gpath)
	repos := getRepos(false)
	scan := paths
	if recursive || auto {
		scan = walkDirs(paths)
	}
	added := addRepos(repos, scan, bare, skipSub, dryRun)
	if dryRun {
		return
	}
	if added.Len() > 0 && auto {
		ng := autoGroup(added, paths)
		if ng.Len() > 0 {
			fmt.Printf("Created %d new group(s).\n", ng.Len())
			writeToGroupsFile(ng, fileAppend)
		}
	}
	addToGroupAfterAdd(added, group, gpath)
}

func fRm(argv []string) {
	pos, err := parseArgs(argv, nil, nil)
	if err != nil {
		argErr("rm: %v", err)
	}
	if len(pos) == 0 {
		argErr("rm: the following arguments are required: repo")
	}
	if !isFile(configFname("repos.csv")) {
		return
	}
	repos := getRepos(false)
	for _, r := range pos {
		if !repos.Has(r) {
			argErr("argument repo: invalid choice: %q", r)
		}
	}
	groups := getGroups()
	updated := false
	for _, r := range pos {
		repos.Delete(r)
		if deleteRepoFromGroups(r, groups) {
			updated = true
		}
	}
	if updated {
		writeToGroupsFile(groups, fileWrite)
	}
	writeToRepoFile(repos, fileWrite)
}

func fRename(argv []string) {
	pos, err := parseArgs(argv, nil, nil)
	if err != nil || len(pos) != 2 {
		argErr("rename: expected arguments: repo new_name")
	}
	repos := getRepos(false)
	if !repos.Has(pos[0]) {
		argErr("argument repo: invalid choice: %q", pos[0])
	}
	renameRepo(repos, pos[0], pos[1])
}

func firstRemoteURL(path string) string {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) == 0 {
		return ""
	}
	parts := strings.Fields(lines[0])
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

func fFreeze(argv []string) {
	var group string
	pos, err := parseArgs(argv, nil, map[string]*string{"-g": &group, "--group": &group})
	if err != nil || len(pos) > 0 {
		argErr("freeze: unexpected arguments")
	}
	groups := getGroups()
	if group != "" {
		mustGroup(groups, group)
	} else if ctx := getContext(); ctx != "" {
		group = ctxStem(ctx)
		if !groups.Has(group) {
			die("context group %s does not exist", group)
		}
	}
	repos := getRepos(false)
	var groupRepos []string
	if group != "" {
		groupRepos = groups.Get(group).Repos
		repos = subsetRepos(repos, groupRepos)
	}
	t := newTruncator()
	seen := map[string]bool{"": true}
	for _, name := range repos.Names() {
		p := repos.Get(name)
		url := firstRemoteURL(p.Path)
		if seen[url] { // repos without remote are skipped too
			continue
		}
		seen[url] = true
		branch := getRepoBranch(p, t)
		fmt.Printf("%s,%s,%s,%s,%s,%s\n",
			url, name, p.Path, p.Type, strings.Join(p.Flags, " "), branch)
	}
	if group != "" {
		fmt.Printf(",%s,%s,%s\n", group, groups.Get(group).Path, strings.Join(groupRepos, "|"))
	} else {
		for _, gname := range groups.Names() {
			g := groups.Get(gname)
			fmt.Printf(",%s,%s,%s\n", gname, g.Path, strings.Join(g.Repos, "|"))
		}
	}
}

func fLl(argv []string) {
	var noColors, byGroup bool
	pos, err := parseArgs(argv,
		map[string]*bool{"-C": &noColors, "--no-colors": &noColors, "-g": &byGroup}, nil)
	if err != nil || len(pos) > 1 {
		argErr("ll: usage: gota ll [-C] [-g] [group]")
	}
	group := ""
	if len(pos) == 1 {
		group = pos[0]
	}
	if group == "" {
		if ctx := getContext(); ctx != "" {
			group = ctxStem(ctx)
		}
	}
	repos := getRepos(false)
	groups := getGroups()
	var groupRepos []string
	if group != "" {
		mustGroup(groups, group)
		groupRepos = groups.Get(group).Repos
		repos = subsetRepos(repos, groupRepos)
	}
	if byGroup {
		if len(groupRepos) > 0 {
			fmt.Printf("%s:\n", group)
			for _, line := range describe(repos, noColors) {
				fmt.Println("  ", line)
			}
		} else {
			for _, gname := range groups.Names() {
				fmt.Printf("%s:\n", gname)
				gRepos := subsetRepos(repos, groups.Get(gname).Repos)
				for _, line := range describe(gRepos, noColors) {
					fmt.Println("  ", line)
				}
			}
		}
	} else {
		for _, line := range describe(repos, noColors) {
			fmt.Println(line)
		}
	}
}

func fLs(argv []string) {
	pos, err := parseArgs(argv, nil, nil)
	if err != nil || len(pos) > 1 {
		argErr("ls: usage: gota ls [repo]")
	}
	repos := getRepos(false)
	if len(pos) == 1 {
		if !repos.Has(pos[0]) {
			argErr("argument repo: invalid choice: %q", pos[0])
		}
		fmt.Println(repos.Get(pos[0]).Path)
		return
	}
	fmt.Println(strings.Join(repos.Names(), " "))
}

// groupAddRepos: `group add` core; gpath nil keeps existing path
func groupAddRepos(gname string, repoNames []string, gpath *string) {
	groups := getGroups()
	if groups.Has(gname) {
		g := groups.Get(gname)
		g.Repos = sortedUnion(g.Repos, repoNames)
		if gpath != nil {
			g.Path = *gpath
		}
		writeToGroupsFile(groups, fileWrite)
	} else {
		p := ""
		if gpath != nil {
			p = *gpath
		}
		names := append([]string(nil), repoNames...)
		sort.Strings(names)
		ng := newGroups()
		ng.Set(gname, &Group{Repos: names, Path: p})
		writeToGroupsFile(ng, fileAppend)
	}
}

func fGroup(argv []string) {
	subs := map[string]bool{"ll": true, "ls": true, "rename": true, "rm": true, "add": true, "rmrepo": true}
	cmd, rest := "ll", argv
	if len(argv) > 0 {
		if !subs[argv[0]] {
			argErr("group: invalid choice: %q (choose from ll, ls, add, rmrepo, rename, rm)", argv[0])
		}
		cmd, rest = argv[0], argv[1:]
	}
	groups := getGroups()
	switch cmd {
	case "ll":
		pos, err := parseArgs(rest, nil, nil)
		if err != nil || len(pos) > 1 {
			argErr("group ll: usage: gota group ll [group]")
		}
		if len(pos) == 1 {
			g := mustGroup(groups, pos[0])
			fmt.Println(strings.Join(g.Repos, " "))
			return
		}
		for _, gname := range groups.Names() {
			g := groups.Get(gname)
			fmt.Printf("%s%s%s: %s\n", colorCodes["underline"], gname, colorCodes["end"], g.Path)
			for _, r := range g.Repos {
				fmt.Println("  -", r)
			}
		}
	case "ls":
		fmt.Println(strings.Join(groups.Names(), " "))
	case "rename":
		pos, err := parseArgs(rest, nil, nil)
		if err != nil || len(pos) != 2 {
			argErr("group rename: expected arguments: group-name new-name")
		}
		gname, newName := pos[0], pos[1]
		mustGroup(groups, gname)
		groupNameCheck(newName, true)
		g := groups.Get(gname)
		groups.Delete(gname)
		groups.Set(newName, g)
		writeToGroupsFile(groups, fileWrite)
		if ctx := getContext(); ctx != "" && ctxStem(ctx) == gname {
			replaceContext(ctx, newName)
		}
	case "rm":
		pos, err := parseArgs(rest, nil, nil)
		if err != nil || len(pos) == 0 {
			argErr("group rm: the following arguments are required: group(s)")
		}
		for _, name := range pos {
			mustGroup(groups, name)
		}
		ctx := getContext()
		for _, name := range pos {
			groups.Delete(name)
			if ctx != "" && ctxStem(ctx) == name {
				replaceContext(ctx, "none")
			}
		}
		writeToGroupsFile(groups, fileWrite)
	case "add":
		unset := "\x00"
		gname, gpath := "", unset
		pos, err := parseArgs(rest, nil,
			map[string]*string{"-n": &gname, "--name": &gname, "-p": &gpath, "--path": &gpath})
		if err != nil {
			argErr("group add: %v", err)
		}
		if gname == "" {
			argErr("group add: the following arguments are required: -n/--name")
		}
		if len(pos) == 0 {
			argErr("group add: the following arguments are required: repo")
		}
		repos := getRepos(false)
		for _, r := range pos {
			if !repos.Has(r) {
				argErr("argument repo: invalid choice: %q", r)
			}
		}
		groupNameCheck(gname, false)
		var gp *string
		if gpath != unset {
			a := absPath(gpath)
			gp = &a
		}
		groupAddRepos(gname, pos, gp)
	case "rmrepo":
		var gname string
		pos, err := parseArgs(rest, nil, map[string]*string{"-n": &gname, "--name": &gname})
		if err != nil {
			argErr("group rmrepo: %v", err)
		}
		if gname == "" {
			argErr("group rmrepo: the following arguments are required: -n/--name")
		}
		if len(pos) == 0 {
			argErr("group rmrepo: the following arguments are required: repo")
		}
		repos := getRepos(false)
		for _, r := range pos {
			if !repos.Has(r) {
				argErr("argument repo: invalid choice: %q", r)
			}
		}
		if groups.Has(gname) {
			for _, r := range pos {
				sub := newGroups()
				sub.Set(gname, groups.Get(gname))
				deleteRepoFromGroups(r, sub)
			}
			writeToGroupsFile(groups, fileWrite)
		}
	}
}

func fContext(argv []string) {
	pos, err := parseArgs(argv, nil, nil)
	if err != nil || len(pos) > 1 {
		argErr("context: usage: gota context [group|auto|none]")
	}
	if len(pos) == 0 {
		ctx := getContext()
		if ctx != "" {
			group := ctxStem(ctx)
			var repos []string
			if g := getGroups().Get(group); g != nil {
				repos = g.Repos
			}
			fmt.Printf("%s: %s\n", group, strings.Join(repos, " "))
		} else if exists(filepath.Join(configDir(), "auto.context")) {
			fmt.Println("auto: none detected!")
		} else {
			fmt.Println("Context is not set")
		}
		return
	}
	choice := pos[0]
	if choice != "none" && choice != "auto" && !getGroups().Has(choice) {
		argErr("argument choice: invalid choice: %q", choice)
	}
	replaceContext(getContext(), choice)
}

func fFlags(argv []string) {
	cmd, rest := "ll", argv
	if len(argv) > 0 {
		if argv[0] != "ll" && argv[0] != "set" {
			argErr("flags: invalid choice: %q (choose from ll, set)", argv[0])
		}
		cmd, rest = argv[0], argv[1:]
	}
	repos := getRepos(false)
	switch cmd {
	case "ll":
		for _, name := range repos.Names() {
			if flags := repos.Get(name).Flags; len(flags) > 0 {
				fmt.Printf("%s: %s\n", name, strings.Join(flags, " "))
			}
		}
	case "set": // REMAINDER: everything after repo is flags, verbatim
		if len(rest) == 0 {
			argErr("flags set: the following arguments are required: repo")
		}
		repo := rest[0]
		if !repos.Has(repo) {
			argErr("argument repo: invalid choice: %q", repo)
		}
		repos.Get(repo).Flags = rest[1:]
		writeToRepoFile(repos, fileWrite)
	}
}

func fColor(argv []string) {
	cmd, rest := "ll", argv
	if len(argv) > 0 {
		if argv[0] != "ll" && argv[0] != "set" && argv[0] != "reset" {
			argErr("color: invalid choice: %q (choose from ll, set, reset)", argv[0])
		}
		cmd, rest = argv[0], argv[1:]
	}
	switch cmd {
	case "ll":
		showColors()
	case "set":
		pos, err := parseArgs(rest, nil, nil)
		if err != nil || len(pos) != 2 {
			argErr("color set: expected arguments: situation color")
		}
		situation, color := pos[0], pos[1]
		keys, enc := getColorEncoding()
		if _, ok := enc[situation]; !ok {
			argErr("argument situation: invalid choice: %q", situation)
		}
		if _, ok := colorCodes[color]; !ok {
			argErr("argument color: invalid choice: %q", color)
		}
		enc[situation] = color
		f, err := openConfigFile(configFname("color.csv"), fileWrite)
		if err != nil {
			die("cannot write color.csv: %v", err)
		}
		defer f.Close()
		var vals []string
		for _, k := range keys {
			vals = append(vals, enc[k])
		}
		fmt.Fprintln(f, strings.Join(keys, ","))
		fmt.Fprintln(f, strings.Join(vals, ","))
	case "reset":
		os.Remove(configFname("color.csv"))
	}
}

func fInfo(argv []string) {
	cmd, rest := "ll", argv
	subs := map[string]bool{"ll": true, "add": true, "rm": true, "set-length": true}
	if len(argv) > 0 {
		if !subs[argv[0]] {
			argErr("info: invalid choice: %q (choose from ll, add, rm, set-length)", argv[0])
		}
		cmd, rest = argv[0], argv[1:]
	}
	toDisplay := getInfoItems()
	writeItems := func(items []string) {
		f, err := openConfigFile(configFname("info.csv"), fileWrite)
		if err != nil {
			die("cannot write info.csv: %v", err)
		}
		defer f.Close()
		fmt.Fprintln(f, strings.Join(items, ","))
	}
	switch cmd {
	case "ll":
		fmt.Println("In use:", strings.Join(toDisplay, ","))
		var unused []string
		for _, x := range allInfoItems {
			found := false
			for _, y := range toDisplay {
				if x == y {
					found = true
					break
				}
			}
			if !found {
				unused = append(unused, x)
			}
		}
		sort.Strings(unused)
		if len(unused) > 0 {
			fmt.Println("Unused:", strings.Join(unused, ","))
		}
	case "add", "rm":
		if len(rest) != 1 {
			argErr("info %s: expected argument: info_item", cmd)
		}
		item := rest[0]
		if !isInfoItem(item) {
			argErr("argument info_item: invalid choice: %q", item)
		}
		idx := -1
		for i, x := range toDisplay {
			if x == item {
				idx = i
				break
			}
		}
		if cmd == "add" && idx < 0 {
			writeItems(append(toDisplay, item))
		} else if cmd == "rm" && idx >= 0 {
			writeItems(append(toDisplay[:idx], toDisplay[idx+1:]...))
		}
	case "set-length":
		fname := configFname("layout.csv")
		fmt.Printf("Settings are in %s\n", fname)
		f, err := openConfigFile(fname, fileWrite)
		if err != nil {
			die("cannot write layout.csv: %v", err)
		}
		defer f.Close()
		fmt.Fprintln(f, "branch,symbols,branch_name,commit_msg,commit_time,path")
		fmt.Fprintln(f, "19,5,27,0,0,30")
	}
}

func nameFromCloneURL(url string) string {
	parts := strings.Split(url, "/")
	base := parts[len(parts)-1]
	return strings.SplitN(base, ".", 2)[0]
}

func fClone(argv []string) {
	var dryRun, preservePath, fromFile bool
	var directory, group string
	pos, err := parseArgs(argv,
		map[string]*bool{
			"-n": &dryRun, "--dry-run": &dryRun,
			"-p": &preservePath, "--preserve-path": &preservePath,
			"-f": &fromFile, "--from-file": &fromFile,
		},
		map[string]*string{
			"-C": &directory, "--directory": &directory,
			"-g": &group, "--group": &group,
		})
	if err != nil {
		argErr("clone: %v", err)
	}
	if len(pos) != 1 {
		argErr("clone: expected a single argument: clonee (a URL or config file)")
	}
	if group != "" && fromFile {
		argErr("clone: -g and -f are mutually exclusive")
	}
	clonee := pos[0]

	if dryRun {
		if fromFile {
			names, cloneRepos, _ := parseCloneConfig(clonee)
			for _, n := range names {
				r := cloneRepos[n]
				fmt.Printf("git clone %s %s\n", r.URL, r.Path)
			}
		} else {
			fmt.Printf("git clone %s\n", clonee)
		}
		return
	}

	path := directory
	if path == "" {
		path, _ = os.Getwd()
	}
	currentPaths := map[string]bool{}
	{
		all := getRepos(true)
		for _, n := range all.Names() {
			currentPaths[all.Get(n).Path] = true
		}
	}

	if !fromFile {
		runSyncInteractive([]string{"git", "clone", clonee}, path, false)
		clonedPath := filepath.Join(path, nameFromCloneURL(clonee))
		if currentPaths[clonedPath] {
			fmt.Printf("%s already in gota.\n", clonee)
			return
		}
		added := addRepos(getRepos(false), []string{clonedPath}, false, false, false)
		addToGroupAfterAdd(added, group, "")
		return
	}

	names, cloneRepos, cloneGroups := parseCloneConfig(clonee)
	var tasks []repoTask
	for _, name := range names {
		r := cloneRepos[name]
		c := []string{"git", "clone", r.URL}
		if preservePath {
			c = append(c, r.Path)
		}
		tasks = append(tasks, repoTask{name: name, dir: path, cmd: c})
	}
	runTasks(tasks)

	// checkout requested branches, only after cloning
	var checkouts []repoTask
	for _, name := range names {
		r := cloneRepos[name]
		if r.Branch == "" {
			continue
		}
		gitPath := name
		if preservePath {
			gitPath = r.Path
		}
		checkouts = append(checkouts, repoTask{name: name, dir: path, cmd: []string{
			"git", "--git-dir=" + gitPath + "/.git", "--work-tree=" + gitPath,
			"checkout", r.Branch,
		}})
	}
	runTasks(checkouts)

	// register new repos, skipping already-known paths
	newR := newRepos()
	for _, name := range names {
		r := cloneRepos[name]
		if !currentPaths[r.Path] {
			newR.Set(name, &Repo{Path: r.Path, Type: r.Type, Flags: r.Flags})
		}
	}
	writeToRepoFile(newR, fileAppend)

	for _, gname := range cloneGroups.Names() {
		groupAddRepos(gname, cloneGroups.Get(gname).Repos, nil)
	}
}

func fSuper(argv []string) {
	man, quote := stripLeadingFlags(argv, "-q", "--quote-mode")
	repos, rest := parseReposAndRest(man, quote)
	if len(rest) == 0 {
		fmt.Println("Missing commands")
		os.Exit(2)
	}
	gitCmd(repos, append([]string{"git"}, rest...), false)
}

func fShell(argv []string) {
	man, quote := stripLeadingFlags(argv, "-q", "--quote-mode")
	repos, rest := parseReposAndRest(man, quote)
	if len(rest) == 0 {
		fmt.Println("Missing commands")
		os.Exit(2)
	}
	cmdStr := strings.Join(rest, " ")
	for _, name := range repos.Names() {
		c := exec.Command("sh", "-c", cmdStr)
		c.Dir = repos.Get(name).Path
		out, _ := c.CombinedOutput()
		fmt.Println(formatOutput(string(out), name))
	}
}

func fClear(argv []string) {
	writeToGroupsFile(newGroups(), fileWrite)
	writeToRepoFile(newRepos(), fileWrite)
}

// fDelegated: cmds.json-defined git sub-command over repos/groups
func fDelegated(name string, def CmdDef, argv []string) {
	shellMode := def.Shell
	pos, err := parseArgs(argv, map[string]*bool{"-s": &shellMode, "--shell": &shellMode}, nil)
	if err != nil {
		argErr("%s: %v", name, err)
	}
	if def.Cmd == "" {
		die("cmds.json: %s has no cmd", name)
	}
	repos := getRepos(false)
	groups := getGroups()
	for _, p := range pos {
		if !repos.Has(p) && !groups.Has(p) {
			argErr("argument repo: invalid choice: %q", p)
		}
	}
	if !def.AllowAll && len(pos) == 0 {
		argErr("%s: the following arguments are required: repo", name)
	}
	chosen, _ := parseReposAndRest(pos, false)
	var cmd []string
	if shellMode {
		cmd = []string{def.Cmd}
	} else {
		cmd = strings.Fields(def.Cmd)
	}
	gitCmd(chosen, cmd, shellMode)
}
