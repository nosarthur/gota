# gota: manage multiple git repos with sanity

This is a remake of the [gita](https://github.com/nosarthur/gita) project with golang
(ported from gita v0.16.8.2). The motivations are

- speed improvements
    - eliminate the slow Python startup time
    - concurrent status/command execution via goroutines
- ease of maintenance
    - single static binary, stdlib only

Display side-by-side status of multiple repos and delegate git commands/aliases
from any working directory:

```
gota ls
gota fetch
gota ll
gota stat myrepo2
gota super myrepo1 commit -am 'add some cool feature'
```

## Installation

1. clone the repo
2. `make`
3. run `gota` command

## Compatibility

Config-compatible with python gita. Config root is `$GOTA_PROJECT_HOME` > `$GITA_PROJECT_HOME` > `$XDG_CONFIG_HOME` > `~/.config`, using the `gota/` subdir — or falling back to an existing `gita/` subdir, so it picks up a python-gita setup as-is:

- `repos.csv` (path,name,type,flags), `groups.csv` (name:repos:path)
- `<group>.context` / `auto.context`
- `color.csv`, `info.csv`, `layout.csv`, `symbols.csv`
- `cmds.json` (custom delegated commands, shadow the built-in ones)

All sub-commands ported: `add rm freeze clone rename flags color info ll context ls group super shell clear` plus the delegated git commands from `cmds.json` (`fetch pull push st ll lo br ...`) with per-repo flags, `allow_all`, `disable_async`, and shell-mode semantics. Async execution runs detached from the tty; failed repos are re-run synchronously so credential prompts still work.

## Deviations from python gita

Upstream bugs fixed rather than ported:

- `clone -n -f` (dry-run from file) printed a traceback upstream; prints the clone commands here.
- `add -a` crashed when the first given path was not a parent of a found repo; non-matching paths are skipped.
- sync re-run after a failed async command dropped per-repo flags upstream; kept here.
- `group add -n <existing>` without `-p` silently reset the group path; path now only changes when `-p` is given.
- delegated custom commands with `"shell": true` crashed on >1 repo upstream; run via `sh -c` here.
- `group rm` of the current context left a stray `.context` file; the context is removed instead.
- `flags ll` printed the python list repr (`['-c', ...]`); prints space-joined flags.

Behavior differences:

- `-s/--shell` on delegated commands is a plain boolean flag (upstream needed a throwaway value: `--shell x`).
- `gota <cmd> -h` per-sub-command help is minimal; shell completion (argcomplete) not ported.
- new repos found by `add` are processed in sorted path order (upstream: undefined set order).

## Test

```
make test
bash scripts/smoke.sh   # end-to-end exercise of all sub-commands
```
