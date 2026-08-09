#!/bin/bash
# smoke test gota end-to-end against tmp repos
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
go build -o "$ROOT/gota" "$ROOT/cmd/gota"
BIN="$ROOT/gota"
WORK=$(mktemp -d)
export GOTA_PROJECT_HOME="$WORK/config"
cd "$WORK"

mkgit() {
  git init -q "$1"
  git -C "$1" config user.email t@t
  git -C "$1" config user.name t
  git -C "$1" commit -q --allow-empty -m "init $1"
}

mkdir -p src/frontend src/backend
mkgit src/frontend/app1
mkgit src/frontend/app2
mkgit src/backend/api

echo "--- add -a (auto-group recursive)"
"$BIN" add -a src
echo "--- ls"
"$BIN" ls
echo "--- group ll"
"$BIN" group ll
echo "--- ll"
"$BIN" ll
echo "--- ll -C no colors"
"$BIN" ll -C
echo "--- ll group filter"
"$BIN" ll src-frontend
echo "--- ll -g by group"
"$BIN" ll -g
echo "--- delegated: st"
"$BIN" st app1
echo "--- delegated async: lo (all repos)"
"$BIN" lo
echo "--- super"
"$BIN" super app1 log --oneline -1
echo "--- shell"
"$BIN" shell src-backend pwd
echo "--- freeze"
"$BIN" freeze || true
echo "--- flags"
"$BIN" flags set app1 -c core.pager=cat
"$BIN" flags
"$BIN" flags set app1
echo "--- context"
"$BIN" context src-frontend
"$BIN" context
"$BIN" ls   # unaffected by context
"$BIN" ll   # affected: only frontend repos
"$BIN" context none
"$BIN" context
echo "--- auto context"
"$BIN" context auto
(cd src/backend/api && "$BIN" context)
"$BIN" context none
echo "--- info"
"$BIN" info
"$BIN" info add path
"$BIN" info ll
"$BIN" info rm path
echo "--- color"
"$BIN" color set no_remote blue
"$BIN" color ll | tail -6
"$BIN" color reset
echo "--- rename"
"$BIN" rename app1 myapp
"$BIN" ls
"$BIN" group ll src-frontend
"$BIN" rename myapp app1
echo "--- group add/rmrepo/rename/rm"
"$BIN" group add app1 api -n mixed
"$BIN" group ll mixed
"$BIN" group rmrepo api -n mixed
"$BIN" group ll mixed
"$BIN" group rename mixed blend
"$BIN" group ls
"$BIN" group rm blend
"$BIN" group ls
echo "--- rm repo"
"$BIN" rm app2
"$BIN" ls
echo "--- clone from freeze file (local remotes)"
mkgit remote1
"$BIN" freeze > /dev/null  # repos have no remotes; add one with remote
git clone -q remote1 cloned1 2>/dev/null
"$BIN" add cloned1
"$BIN" freeze | tee freeze.csv
"$BIN" rm cloned1
rm -rf cloned1_copy
mkdir clone_dest
cd clone_dest
"$BIN" clone -n -f ../freeze.csv
"$BIN" clone -f ../freeze.csv
"$BIN" ls
cd "$WORK"
echo "--- clear"
"$BIN" clear
"$BIN" ls
echo "--- version/help"
"$BIN" -v
"$BIN" >/dev/null
echo "ALL SMOKE OK"
