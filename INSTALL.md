# Installing / Building vimango

> **Starting from a completely fresh Mac** (no git, Homebrew, or Go installed)?
> Use [MACOS_SETUP.md](./MACOS_SETUP.md) — it bootstraps the machine from zero
> (developer tools, Homebrew, a terminal that can display images) and covers
> the same build steps as this file.

This is a step-by-step guide to building vimango from scratch on a new machine.
It focuses on the **full CGO build** on **macOS**, which is the most involved
path because several artifacts are deliberately *not* checked into the repo
(they differ per platform/architecture) and one dependency is a personal fork.

> **TL;DR of the gotchas.** A fresh `git clone` will **not** build until you
> build **`libvim.a`** locally (`scripts/build-libvim.sh`, §4), which also
> copies the spell-check dictionary into `spell/`. One more piece is optional,
> and vimango builds and runs without it: the **`heic_worker`** helper binary
> (§5), without which HEIC images don't display.
>
> None of these come with the clone because they're `.gitignore`d or live in a
> separate repo. (The glamour fork used to be on this list; `go.mod` now pins
> it, so `go build` fetches it.)

---

## 0. Prerequisites

- **Go** — 1.26+ (`go.mod` says `go 1.26.0`). Check with `go version`.
- **Homebrew** — https://brew.sh
- **Xcode command line tools** — `xcode-select --install`

### Apple Silicon vs Intel

Several build flags reference the Homebrew prefix. **Confirm which you're on:**

```bash
brew --prefix
```

- `/opt/homebrew` → **Apple Silicon** (all the hardcoded paths below match).
- `/usr/local` → **Intel** — you must substitute `/usr/local` for `/opt/homebrew`
  in any hardcoded CFLAGS/LDFLAGS. (The libvim build scripts auto-detect
  this.)

---

## 1. Clone the repo

```bash
git clone git@github.com:slzatz/vimango.git
cd vimango
```

---

## 2. The glamour fork (nothing to do)

vimango imports Kitty text-sizing helpers (e.g. `ansi.DecodeKittyTextSizeMarkers`,
`makeRaw`/`restoreTerminal`) that exist **only** in a personal fork of glamour,
[slzatz/glamour](https://github.com/slzatz/glamour), not in upstream
`github.com/charmbracelet/glamour`. A `replace` directive committed in `go.mod`
pins a commit of the fork, and `go build` downloads it like any other
dependency — no separate clone, no `go.work`.

**Machines set up before 2026-11-08** have a `go.work` next to `go.mod` and a
`../glamour` checkout. Delete the `go.work`: while it exists it silently
overrides the pin with whatever `../glamour` holds. (`go env GOWORK` prints
`off` or nothing once it's gone.)

**Symptoms of a stale `go.work`:** `undefined: ansi.DecodeKittyTextSizeMarkers`,
`undefined: makeRaw`, `undefined: unix.TCGETS` on macOS.

---

## 3. Native dependencies (Homebrew)

```bash
brew install pkg-config ncurses gettext libheif
```

> **`pkg-config` is required for the `heic_worker` build** (§5). The go-libheif
> CGO binding invokes `pkg-config` to locate libheif's compiler/linker flags. It
> is often present as a transitive dependency of other formulae, but on a fresh
> machine it may be absent — its omission produces
> `exec: "pkg-config": executable file not found in $PATH`.

---

## 4. Build libvim.a  (REQUIRED for CGO build)

`libvim.a` is a static library built from the
[slzatz/libvim](https://github.com/slzatz/libvim) fork — **never** upstream
[onivim/libvim](https://github.com/onivim/libvim), which is unmaintained and
lacks fixes vimango needs. It is architecture-specific and `.gitignore`d, so it
must be built on each machine (every Mac and every Linux box) and placed in the
project root (next to `go.mod`). **Rebuild it whenever the fork moves** — a
`libvim.a` older than the vimango you are building either fails to link
(e.g. `undefined: vimSetMappingsEnabled`) or builds fine with behavior silently
missing.

**Symptom if missing:** linker errors / `libvim.a` not found when building.

> **The fork's `master` branch was renamed `main` on 2026-11-09.** A
> `../libvim` still on `master` is switched to `main` by the script. One on
> the `spelling_restore` branch (from the hand-build steps this section used
> to give) is refused; that branch is merged, so switch first:
> `git -C ../libvim fetch origin && git -C ../libvim switch main`. (The spell
> code has been built and tested on macOS only so far, not yet with GCC on
> Linux.)

### Use the script (macOS and Linux)

```bash
./scripts/build-libvim.sh
```

It uses the fork checkout at `../libvim` (next to vimango; set `LIBVIM_DIR` to
keep it elsewhere), cloning it there if it doesn't exist. It refuses to build
from a checkout that isn't the fork, isn't on `main`, or has local changes,
and says how to fix each. Otherwise it pulls `main`, builds with the fork's
own `build-macos.sh` / `build-linux.sh` (installing the Homebrew ncurses and
gettext first on macOS), copies `libvim.a` into the project root, and prints the
libvim commit it was built from.

Options: `--clean` rebuilds every object, `--tests` also runs libvim's API
tests, `--no-pull` builds the checkout as it is (e.g. offline).

### Manual

The script is a thin wrapper; by hand it is:

```bash
cd ..
git clone https://github.com/slzatz/libvim.git    # or: git -C libvim pull
cd libvim
./build-macos.sh       # Linux: ./build-linux.sh   (--clean, --tests as above)
cp src/libvim.a ../vimango/
```

The fork's scripts hold the configure flags — including the `-Wno-error`
set newer clang and GCC 14+ need, because libvim builds with `-Werror` — and
reconfigure only when those flags change. The fork's `CLAUDE.md` and
`PATCHES.md` explain the flags and what the fork changes.

> The CGO linker directives in `vim/vim.go` reference `libvim.a` as a bare
> filename resolved from the build working directory, so it must sit in the
> project root and you must run `go build` from there. The checked-in
> libvim headers under `vim/` (including `auto/config.h`) work on both platforms and don't
> need regenerating.

---

## 5. Helper worker binary

`heic_worker` runs as a subprocess (so a native-library crash can't take down
vimango). It is `.gitignore`d and must be built locally, and is looked up next
to the main executable, then in the current directory.

### heic_worker (HEIC/HEIF image decoding — libheif)

```bash
cd cmd/heic_worker
CGO_ENABLED=1 go build -o ../../heic_worker
cd ../..
```

If missing / built for the wrong arch, `IsHEICAvailable()` returns false and
HEIC images silently fail (a pure-Go fallback covers most cases). Not fatal.

**Symptom if `pkg-config` or libheif is missing:**
`exec: "pkg-config": executable file not found in $PATH` (or
`Package libheif was not found in the pkg-config search path`). Install both
with `brew install pkg-config libheif` (§3). Note: `brew --prefix libheif`
prints an expected path even when libheif is *not* installed — confirm the
install with `brew list libheif` or `pkg-config --modversion libheif`.

**Symptom of a libheif version mismatch:** cgo type errors from the binding,
e.g. `cannot use uint32(compression) ... as _Ctype_heif_compression_format`.
The `github.com/strukturag/libheif` Go binding is versioned to track the
libheif **C library** release, but `github.com/klippa-app/go-libheif` (used by
`heic_worker`) pins an older binding (`v1.17.6`) that only compiles against
libheif 1.17's headers. Homebrew ships only the current libheif, so the pinned
binding won't build against it. Fix by overriding the binding to match your
installed libheif version:

```bash
# match the version reported by: pkg-config --modversion libheif
go get github.com/strukturag/libheif@v1.23.1
```

This adds a `require github.com/strukturag/libheif vX.Y.Z // indirect` line to
`go.mod`. Because that version must match the libheif installed on the build
machine, keep it in sync with Homebrew's libheif (bump it after a
`brew upgrade libheif` if the cgo type errors reappear). A benign
`ld: warning: ignoring duplicate libraries: '-lheif'` during the build is
expected and harmless.

---

## 6. Build vimango

From the project root:

```bash
CGO_ENABLED=1 go build --tags=fts5
```

`fts5` is the only tag needed. Don't add `cgo`: `CGO_ENABLED=1` already sets
it. Spell checking needs no tag or library: it is vim's own, built into
libvim, and reads the dictionary `scripts/build-libvim.sh` puts in `spell/`.

CGO is required — libvim is the only vim engine (pure-Go builds and Windows are unsupported).

---

## 7. First-run configuration

`config.json` is `.gitignore`d (it can hold secrets), so a fresh clone has none.
Generate a default one plus the local SQLite databases:

```bash
./vimango --init
```

This writes `config.json` (pointing at `vimango.db` / `fts5_vimango.db`) and
creates the databases. Style files `default.json` / `darkslz.json` **are** in the
repo and are required at startup.

Run it:

```bash
./vimango
```

---

## Troubleshooting quick reference

| Error | Cause | Fix |
|---|---|---|
| `undefined: ansi.DecodeKittyTextSizeMarkers` / `makeRaw` / `restoreTerminal`, or `undefined: unix.TCGETS` on macOS | a leftover `go.work` pointing at a stale `../glamour` | delete `go.work` (§2) |
| `libvim.a` not found / link errors | libvim.a not built or not in root | §4 |
| `:spell` says "Spell check unavailable" | no `spell/en.utf-8.spl` next to config.json | re-run `./scripts/build-libvim.sh` (§4) |
| `undefined: vimSetMappingsEnabled` (or another `vim…` symbol) at link time | `libvim.a` older than this vimango, or built from upstream onivim/libvim | re-run `./scripts/build-libvim.sh` (§4) |
| libvim build fails on a warning (e.g. `long long` to `double`) | newer clang promotes warnings under `-Werror` | `git -C ../libvim pull` — the fork's build scripts carry the `-Wno-error` flags; report it there if it persists |
| `exec: "pkg-config": executable file not found in $PATH` | `pkg-config` (and maybe libheif) not installed | `brew install pkg-config libheif` (§3) |
| `Package libheif was not found in the pkg-config search path` | libheif not actually installed (despite `brew --prefix` printing a path) | `brew install libheif`; verify with `brew list libheif` (§5) |
| `cannot use uint32(...) as _Ctype_heif_*` when building `heic_worker` | strukturag/libheif binding version ≠ installed libheif | `go get github.com/strukturag/libheif@<installed-version>` (§5) |
| HEIC images don't render | `heic_worker` missing/wrong arch | §5 (non-fatal) |
| `config.json` / database errors on first run | never initialized | `./vimango --init` (§7) |
