#!/usr/bin/env bash
#
# build-libvim.sh — build libvim.a for vimango and copy it into the project root.
#
# libvim.a must come from the slzatz/libvim fork, never upstream onivim/libvim:
# vimango needs the fork's fixes (Visual-block I/A, vimSetMappingsEnabled), and
# a libvim.a built from upstream -- or from a stale fork checkout -- either
# fails to link or builds fine with the behavior silently missing. It is
# platform- and architecture-specific and .gitignore'd, so every machine
# (each Mac, each Linux box) builds its own, and rebuilds it whenever the fork
# moves.
#
# This script:
#   1. Finds the fork checkout at ../libvim (next to vimango), or clones it there.
#   2. Refuses to build unless that checkout's origin is slzatz/libvim, it is on
#      main, and it has no local changes -- each failure says how to fix it.
#      A checkout still on master (the fork's branch was renamed to main on
#      2026-11-09) is switched over first, when that is a fast-forward.
#   3. Pulls main (skip with --no-pull, e.g. offline).
#   4. Builds with the fork's own script -- build-macos.sh or build-linux.sh --
#      which owns the configure flags and reconfigures only when they change.
#      On macOS it installs the Homebrew build deps (ncurses, gettext) first.
#   5. Copies libvim.a into the vimango root, the English spell dictionary
#      (runtime/spell/en.utf-8.{spl,sug}) into spell/, and vim's command table
#      (src/ex_cmds.h) into vim/, and reports the commit it was built from.
#   6. Checks that vim/libvim_api.h -- vimango's own copy of the libvim
#      declarations it uses -- still matches the fork's headers.
#
# Usage:
#   ./scripts/build-libvim.sh            # pull, build, copy
#   ./scripts/build-libvim.sh --clean    # rebuild every object from scratch
#   ./scripts/build-libvim.sh --tests    # also run libvim's API test suite
#   ./scripts/build-libvim.sh --no-pull  # build the checkout as it is
#
# Keep the fork somewhere else with LIBVIM_DIR:
#   LIBVIM_DIR=/path/to/libvim ./scripts/build-libvim.sh
#
set -euo pipefail

FORK_URL="https://github.com/slzatz/libvim.git"
FORK_RE='github\.com[:/]slzatz/libvim(\.git)?/?$'

PULL=1
PASS=()
for arg in "$@"; do
  case "$arg" in
    --no-pull) PULL=0 ;;
    --clean | --tests) PASS+=("$arg") ;;
    -h | --help) sed -n '2,36p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "error: unknown option: $arg (see --help)" >&2; exit 1 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VIMANGO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
LIBVIM_DIR="${LIBVIM_DIR:-$(cd "${VIMANGO_ROOT}/.." && pwd)/libvim}"

case "$(uname -s)" in
  Darwin) FORK_SCRIPT=build-macos.sh ;;
  Linux) FORK_SCRIPT=build-linux.sh ;;
  *) echo "error: unsupported platform $(uname -s) (macOS and Linux only)" >&2; exit 1 ;;
esac

# --- 1. Find or clone the fork.
if [[ ! -d "${LIBVIM_DIR}" ]]; then
  echo "==> Cloning ${FORK_URL} into ${LIBVIM_DIR}"
  git clone "${FORK_URL}" "${LIBVIM_DIR}"
fi
cd "${LIBVIM_DIR}"

# --- 2. It must be the fork, on main, unmodified.
origin="$(git remote get-url origin 2>/dev/null || true)"
if ! [[ "${origin}" =~ ${FORK_RE} ]]; then
  echo "error: ${LIBVIM_DIR} is not the slzatz/libvim fork (origin: ${origin:-none})." >&2
  echo "       A libvim.a built from it would lack vimango's fixes. Move it aside and" >&2
  echo "       re-run to clone the fork:  mv '${LIBVIM_DIR}' '${LIBVIM_DIR}.upstream'" >&2
  exit 1
fi
branch="$(git branch --show-current)"
# The fork's master became main on 2026-11-09. Move a checkout made before
# that across, but only when its master is behind origin/main (nothing local
# would be lost); anything else is left for a person to sort out.
if [[ "${branch}" == "master" ]] && [[ "${PULL}" -eq 1 ]]; then
  echo "==> The fork's master is now main; switching ${LIBVIM_DIR} over"
  git fetch --prune origin
  if git merge-base --is-ancestor master origin/main; then
    if git show-ref --verify --quiet refs/heads/main; then
      git checkout main
      git branch -d master
    else
      git branch -m master main
    fi
    git branch -u origin/main main
    git remote set-head origin -a >/dev/null
    branch=main
  else
    echo "error: ${LIBVIM_DIR}'s master has commits that are not on origin/main." >&2
    echo "       Look at them (git -C '${LIBVIM_DIR}' log origin/main..master), then switch:" >&2
    echo "       git -C '${LIBVIM_DIR}' switch main" >&2
    exit 1
  fi
fi
if [[ "${branch}" != "main" ]]; then
  echo "error: ${LIBVIM_DIR} is on branch '${branch:-detached HEAD}', not main." >&2
  echo "       vimango is built from the fork's main:" >&2
  echo "       git -C '${LIBVIM_DIR}' fetch origin && git -C '${LIBVIM_DIR}' switch main" >&2
  exit 1
fi
if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
  echo "error: ${LIBVIM_DIR} has uncommitted changes to tracked files:" >&2
  git status --short --untracked-files=no >&2
  echo "       Commit or stash them; vimango is built from the fork's main as published." >&2
  exit 1
fi

# --- 3. Bring main up to date.
if [[ "${PULL}" -eq 1 ]]; then
  echo "==> Pulling ${LIBVIM_DIR}"
  git pull --ff-only origin main
fi
commit="$(git log -1 --format='%h %s')"
echo "==> Building libvim at ${commit}"

# --- 4. Build with the fork's own script.
if [[ "$(uname -s)" == "Darwin" ]]; then
  if ! command -v brew >/dev/null 2>&1; then
    echo "error: Homebrew not found. Install it from https://brew.sh first." >&2
    exit 1
  fi
  for pkg in ncurses gettext; do
    brew list --versions "${pkg}" >/dev/null 2>&1 || brew install "${pkg}"
  done
fi
./"${FORK_SCRIPT}" ${PASS[@]+"${PASS[@]}"}

# --- 5. Copy it in.
cp src/libvim.a "${VIMANGO_ROOT}/libvim.a"
# vim's spell dictionary, which libvim reads from spell/ (see spellcheck.go).
# The user's own word list (spell/en.utf-8.add, from zg) is left alone.
mkdir -p "${VIMANGO_ROOT}/spell"
cp runtime/spell/en.utf-8.spl runtime/spell/en.utf-8.sug "${VIMANGO_ROOT}/spell/"
# vim's table of ex commands, which vimango embeds to resolve command names as
# this library does (vim/excommands.go). A tracked file: if the fork changed
# it, commit the new copy.
cp src/ex_cmds.h "${VIMANGO_ROOT}/vim/ex_cmds.h"

# --- 6. vimango compiles against its own copy of the declarations it uses, not
# against these headers, so nothing in its build notices when one changes here.
if command -v go >/dev/null 2>&1; then
  echo "==> Checking vim/libvim_api.h against ${LIBVIM_DIR}/src"
  if ! (cd "${VIMANGO_ROOT}" && LIBVIM_DIR="${LIBVIM_DIR}" CGO_ENABLED=1 \
        go test --tags=fts5 -count=1 -run '^TestHeaderMatchesLibvim$' ./vim); then
    echo "error: vim/libvim_api.h no longer matches libvim's headers (see the test's output above)." >&2
    echo "       libvim.a was built and copied, but vimango must not be built against it until" >&2
    echo "       vim/libvim_api.h is brought into line with ${LIBVIM_DIR}/src/libvim.h." >&2
    exit 1
  fi
else
  echo "note: go is not installed yet, so vim/libvim_api.h was not checked against the fork;" >&2
  echo "      'go test ./vim' does that once it is." >&2
fi
echo
echo "Done. ${VIMANGO_ROOT}/libvim.a is libvim ${commit}"
echo "Now build vimango:"
echo "  cd ${VIMANGO_ROOT} && CGO_ENABLED=1 go build --tags=fts5"
