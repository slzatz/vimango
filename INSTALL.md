# Installing / Building vimango

This is a step-by-step guide to building vimango from scratch on a new machine.
It focuses on the **full CGO build** on **macOS**, which is the most involved
path because several artifacts are deliberately *not* checked into the repo
(they differ per platform/architecture) and one dependency is a personal fork.

> **TL;DR of the gotchas.** A fresh `git clone` will **not** build until you:
> 1. Clone the **glamour fork** and create a **`go.work`** file (both untracked).
> 2. Install **hunspell** via Homebrew.
> 3. Build **`libvim.a`** locally (`scripts/build-libvim.sh`).
> 4. Build the **`heic_worker`** and **`webview_worker`** helper binaries.
>
> None of these come with the clone because they're `.gitignore`d or live in a
> separate repo.

---

## 0. Prerequisites

- **Go** — 1.24+ (the workspace file pins `go 1.26.x`; your installed Go must be
  ≥ that line in `go.work`). Check with `go version`.
- **Homebrew** — https://brew.sh
- **Xcode command line tools** — `xcode-select --install`

### Apple Silicon vs Intel

Several build flags reference the Homebrew prefix. **Confirm which you're on:**

```bash
brew --prefix
```

- `/opt/homebrew` → **Apple Silicon** (all the hardcoded paths below match).
- `/usr/local` → **Intel** — you must substitute `/usr/local` for `/opt/homebrew`
  in any hardcoded CFLAGS/LDFLAGS. (The `build-libvim.sh` script auto-detects
  this; the hunspell binding is hardcoded — see the hunspell section.)

---

## 1. Clone the repo

```bash
git clone git@github.com:slzatz/vimango.git
cd vimango
```

---

## 2. The glamour fork + go.work  (REQUIRED — clone will not compile without it)

vimango imports Kitty text-sizing helpers (e.g. `ansi.DecodeKittyTextSizeMarkers`,
`makeRaw`/`restoreTerminal`) that exist **only** in a personal fork of glamour,
not in upstream `github.com/charmbracelet/glamour`. The fork is wired in via a
Go **workspace** file (`go.work`), which is `.gitignore`d and therefore absent
from a fresh clone.

**Symptoms if you skip this:** `undefined: ansi.DecodeKittyTextSizeMarkers`,
`undefined: makeRaw`, `undefined: restoreTerminal`, etc.

### 2a. Clone the glamour fork as a sibling of vimango

```bash
cd ..                     # parent dir that contains vimango/
git clone git@github.com:slzatz/glamour.git
```

You should now have `.../vimango` and `.../glamour` side by side.

### 2b. Create go.work inside vimango

`go.work` is not committed, so create it by hand:

```bash
cd vimango
cat > go.work <<'EOF'
go 1.26.1

use (
	.
	../glamour
)
EOF
```

Adjust `../glamour` if you cloned the fork somewhere else — the path is relative
to the vimango directory. Make sure the `go 1.26.1` line is **≤** your installed
Go version, or the toolchain will complain.

> **Note on the fork itself:** the fork carries macOS-specific termios code
> (`ansi/kitty_text_sizing_darwin.go` using `TIOCGETA`, with the Linux/BSD file
> narrowed to `//go:build linux || freebsd || openbsd || netbsd` using `TCGETS`).
> If you ever see `undefined: unix.TCGETS` on macOS, your fork checkout is behind —
> `git -C ../glamour pull`.

---

## 3. Native dependencies (Homebrew)

```bash
brew install pkg-config hunspell ncurses gettext libheif
```

> **`pkg-config` is required for the `heic_worker` build** (§5). The go-libheif
> CGO binding invokes `pkg-config` to locate libheif's compiler/linker flags. It
> is often present as a transitive dependency of other formulae, but on a fresh
> machine it may be absent — its omission produces
> `exec: "pkg-config": executable file not found in $PATH`.

### hunspell — header path caveat

The Go binding hardcodes the **Apple Silicon** Homebrew paths in
`hunspell/hunspell.go`:

```go
// #cgo darwin LDFLAGS: -lhunspell-1.7 -L/opt/homebrew/lib
// #cgo darwin CFLAGS: -I/opt/homebrew/include
// #include <hunspell/hunspell.h>
```

**Symptom if hunspell is missing:** `hunspell/hunspell.h: file not found`.
Just `brew install hunspell` fixes it on Apple Silicon.

**On Intel** (`brew --prefix` = `/usr/local`), those hardcoded paths are wrong —
edit `hunspell/hunspell.go` to point the darwin `CFLAGS`/`LDFLAGS` at
`/usr/local/include` and `/usr/local/lib`.

> Runtime note: spell-check dictionaries are looked up at
> `/usr/share/hunspell/en_US.{aff,dic}` (`spellcheck_cgo.go`), a Linux path that
> won't exist on macOS. Spell check degrades gracefully if they're absent — the
> build and app are unaffected.

---

## 4. Build libvim.a  (REQUIRED for CGO build)

`libvim.a` is a static library built from
[onivim/libvim](https://github.com/onivim/libvim). It is architecture-specific
and `.gitignore`d, so it must be built on each machine and placed in the project
root (next to `go.mod`).

**Symptom if missing:** linker errors / `libvim.a` not found when building.

### Easiest: use the script

```bash
./scripts/build-libvim.sh
```

It auto-detects the Homebrew prefix (Apple Silicon vs Intel), installs
ncurses/gettext, clones onivim/libvim next to vimango if needed, builds
`libvim.a`, and copies it into the project root.

### Manual (Apple Silicon)

```bash
cd ..
git clone https://github.com/onivim/libvim.git
cd libvim/src
./configure --disable-selinux --with-tlib=ncurses \
  CFLAGS="-I/opt/homebrew/opt/ncurses/include -I/opt/homebrew/include \
    -Wno-error=implicit-function-declaration -Wno-error=implicit-int \
    -Wno-error=int-conversion -Wno-error=incompatible-function-pointer-types \
    -Wno-error=unused-but-set-variable -Wno-error=deprecated-non-prototype \
    -Wno-error=implicit-int-float-conversion -Wno-error" \
  LDFLAGS="-L/opt/homebrew/opt/ncurses/lib -L/opt/homebrew/lib"
make libvim.a
cp libvim.a /path/to/vimango/
```

> libvim builds with `-Werror`, and newer clang/Xcode versions promote more
> warnings to errors (e.g. `implicit conversion from 'long long' to 'double'`).
> The trailing blanket `-Wno-error` downgrades *all* warnings back to warnings so
> the build survives whatever your clang version flags. If you still hit a hard
> error, it's a real compile failure, not a warning.

> The CGO linker directives in `vim/cvim/cvim.go` reference `libvim.a` as a bare
> filename resolved from the build working directory, so it must sit in the
> project root and you must run `go build` from there. The checked-in
> `vim/cvim/auto/config.h` and `auto/pathdef.c` work on both platforms and don't
> need regenerating.

---

## 5. Helper worker binaries

These run as subprocesses (so native-library crashes can't take down vimango).
Both are `.gitignore`d and must be built locally. They're looked up next to the
main executable, then in the current directory.

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

### webview_worker (native WebKit note preview)

```bash
cd cmd/webview_worker
CGO_ENABLED=1 go build -o ../../webview_worker
cd ../..
```

If missing, `OpenNoteInWebview` falls back to opening rendered HTML in the system
browser (`open`). Not fatal. Worker logs: `$TMPDIR/vimango_webview_worker.log`.

---

## 6. HEIC Python fallback (optional)

Independent of `heic_worker`, there is a pure-Python HEIC path via pillow-heif:

```bash
python3 -m venv .venv
.venv/bin/pip install Pillow pillow-heif
```

`.venv/` and `heic_convert.py` must be next to the executable or in the CWD.
Degrades gracefully if absent.

---

## 7. Build vimango

From the project root:

```bash
CGO_ENABLED=1 go build --tags="fts5,cgo"
```

Pure-Go build (no libvim/hunspell/CGO-sqlite, no worker CGO needed):

```bash
CGO_ENABLED=0 go build --tags=fts5
```

---

## 8. First-run configuration

`config.json` is `.gitignore`d (it can hold secrets), so a fresh clone has none.
Generate a default one plus the local SQLite databases:

```bash
./vimango --init
```

This writes `config.json` (pointing at `vimango.db` / `fts5_vimango.db`) and
creates the databases. Style files `default.json` / `darkslz.json` **are** in the
repo and are required at startup.

Add your Claude API key to `config.json` if you want the deep-research feature
(or set `VIMANGO_CLAUDE_API_KEY`):

```json
{
  "claude": { "api_key": "your-claude-api-key" },
  "glamour": { "style": "darkslz.json" }
}
```

Run it:

```bash
./vimango
```

---

## Troubleshooting quick reference

| Error | Cause | Fix |
|---|---|---|
| `undefined: ansi.DecodeKittyTextSizeMarkers` | glamour fork / `go.work` missing | §2 |
| `undefined: makeRaw` / `restoreTerminal` | fork checkout behind (darwin file missing) | `git -C ../glamour pull` (§2) |
| `undefined: unix.TCGETS` on macOS | fork's darwin termios split not pulled | `git -C ../glamour pull` (§2) |
| `hunspell/hunspell.h: file not found` | hunspell not installed / Intel paths | §3 |
| `libvim.a` not found / link errors | libvim.a not built or not in root | §4 |
| libvim build fails on a warning (e.g. `long long` to `double`) | newer clang promotes warnings under `-Werror` | blanket `-Wno-error` in §4 CFLAGS |
| `exec: "pkg-config": executable file not found in $PATH` | `pkg-config` (and maybe libheif) not installed | `brew install pkg-config libheif` (§3) |
| `Package libheif was not found in the pkg-config search path` | libheif not actually installed (despite `brew --prefix` printing a path) | `brew install libheif`; verify with `brew list libheif` (§5) |
| `cannot use uint32(...) as _Ctype_heif_*` when building `heic_worker` | strukturag/libheif binding version ≠ installed libheif | `go get github.com/strukturag/libheif@<installed-version>` (§5) |
| HEIC images don't render | `heic_worker` missing/wrong arch | §5 (non-fatal) |
| Web view opens in browser instead of window | `webview_worker` missing | §5 (non-fatal) |
| `config.json` / database errors on first run | never initialized | `./vimango --init` (§8) |
