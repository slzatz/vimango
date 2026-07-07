# Setting up vimango on a brand-new Mac

This guide takes a **completely fresh macOS machine** — no developer tools, no
Homebrew, no Go, no git — to a fully working vimango with spell checking
(hunspell), inline images in the terminal (including HEIC), the native
WebKit preview window, and full-text search.

Every step includes a verification command so you can confirm it worked before
moving on. Follow the steps **in order** — later steps assume earlier ones.

Time estimate: 30–60 minutes, most of it waiting for downloads and compiles.

> **Already have a partially set-up machine?** Skim each step's *verify*
> command; skip steps that already pass. The build-from-source details live in
> [INSTALL.md](./INSTALL.md) — this guide covers the same ground plus the
> from-zero bootstrapping around it.

---

## 1. Xcode Command Line Tools (gives you git, clang, make)

macOS ships with *stubs* for `git` and `clang` that prompt you to install the
real tools. Kick that off directly:

```bash
xcode-select --install
```

A dialog appears — click **Install** and wait (several minutes; it's a ~1 GB
download). You do **not** need full Xcode.

**Verify:**

```bash
git --version        # e.g. git version 2.x
clang --version      # e.g. Apple clang ...
```

Both should print versions instead of prompting to install.

---

## 2. Homebrew (the package manager everything else comes from)

Install per https://brew.sh:

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

**Important:** at the end, the installer prints two `eval` lines under
*"Next steps"* — run them, then also add them to your shell profile so `brew`
is on PATH in every new terminal. On Apple Silicon that is:

```bash
echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
eval "$(/opt/homebrew/bin/brew shellenv)"
```

**Verify:**

```bash
brew --version
brew --prefix
```

`brew --prefix` matters for later steps:

- `/opt/homebrew` → **Apple Silicon** (M1/M2/M3/M4). All paths in this guide
  match as written.
- `/usr/local` → **Intel Mac**. Two files hardcode Apple Silicon paths and
  need a one-line edit each — flagged inline below at steps 7 and 9.

---

## 3. A terminal that can display images

vimango renders images **inside the terminal** using the kitty graphics
protocol (unicode placeholders), and its markdown headings use kitty's
text-sizing protocol. The built-in **Terminal.app does not support either** —
notes will work but images won't render.

Install one of:

```bash
brew install --cask kitty      # reference implementation of the protocol
# or
brew install --cask ghostty    # also supports kitty graphics; what the author uses
```

Open the one you installed (Cmd-Space, type its name) and **do the rest of
this guide inside it** — the final smoke test displays an image.

> First launch of an unsigned-by-App-Store app may show a Gatekeeper prompt;
> approve it in System Settings → Privacy & Security if needed.

---

## 4. Go

```bash
brew install go
```

**Verify:**

```bash
go version    # needs 1.26 or later (the workspace file pins go 1.26.1)
```

Homebrew installs the current release, which satisfies this.

---

## 5. Clone vimango and the glamour fork, create go.work

vimango depends on a **personal fork** of the glamour markdown renderer (it
carries kitty text-sizing helpers that upstream doesn't have). The fork must
sit **next to** vimango, wired in by a `go.work` file that is deliberately not
checked in. **A fresh clone will not compile without this step.**

```bash
mkdir -p ~/code && cd ~/code       # or wherever you keep source
git clone https://github.com/slzatz/vimango.git
git clone https://github.com/slzatz/glamour.git
```

Create the workspace file inside vimango:

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

**Verify:**

```bash
ls ../glamour/go.mod go.work    # both files must exist
```

> If you later see `undefined: ansi.DecodeKittyTextSizeMarkers` or
> `undefined: makeRaw` when building, this step was skipped or `go.work`
> points at the wrong path.

---

## 6. Native libraries from Homebrew

```bash
brew install pkg-config hunspell ncurses gettext libheif
```

What each is for:

| Package | Used by |
|---|---|
| `pkg-config` | locating libheif when building the HEIC worker |
| `hunspell` | spell checking (the library; dictionaries are step 7) |
| `ncurses`, `gettext` | linking libvim and the vimango binary |
| `libheif` | decoding HEIC/HEIF images (iPhone photos) |

**Verify:**

```bash
pkg-config --modversion libheif     # prints a version, e.g. 1.23.x
brew list hunspell >/dev/null && echo hunspell OK
```

---

## 7. Hunspell dictionaries (spell check won't work without them)

Homebrew installs the hunspell *library* but **no dictionaries**, and vimango
looks for them at a fixed path: `/usr/share/hunspell/en_US.aff` and
`/usr/share/hunspell/en_US.dic`. Fetch the standard US-English dictionary from
the LibreOffice project and put it there:

```bash
sudo mkdir -p /usr/share/hunspell
sudo curl -fsSL -o /usr/share/hunspell/en_US.aff \
  https://raw.githubusercontent.com/LibreOffice/dictionaries/master/en/en_US.aff
sudo curl -fsSL -o /usr/share/hunspell/en_US.dic \
  https://raw.githubusercontent.com/LibreOffice/dictionaries/master/en/en_US.dic
```

**Verify:**

```bash
ls -la /usr/share/hunspell/    # both files present and non-trivial in size
```

If the dictionaries are absent vimango still builds and runs — spell check
just silently reports every word as correct.

> **Intel Macs only:** the Go binding hardcodes Apple Silicon include/lib
> paths. Edit `hunspell/hunspell.go` and change `/opt/homebrew/include` →
> `/usr/local/include` and `/opt/homebrew/lib` → `/usr/local/lib` in the two
> `#cgo darwin` lines.

---

## 8. Build libvim.a (the vim engine)

vimango's editor **is** vim, provided by a static library built from
[onivim/libvim](https://github.com/onivim/libvim). It's architecture-specific,
so each machine builds its own copy. A script does everything (detects your
Homebrew prefix, clones libvim next to vimango, builds, and copies the result
into the vimango root):

```bash
cd ~/code/vimango       # adjust if you cloned elsewhere
./scripts/build-libvim.sh
```

This takes a few minutes of C compilation.

**Verify:**

```bash
ls -la libvim.a         # sits in the vimango root, next to go.mod
```

> If the script fails partway, the manual recipe and an explanation of the
> `-Wno-error` workaround for newer clang versions are in
> [INSTALL.md §4](./INSTALL.md).

---

## 9. Build the two helper binaries

These run as subprocesses so that crashes in native image/WebKit libraries
can't take down vimango. Both are optional-but-recommended; both land in the
vimango root.

### heic_worker — HEIC/HEIF image decoding

```bash
cd cmd/heic_worker
CGO_ENABLED=1 go build -o ../../heic_worker
cd ../..
```

> **If you get cgo type errors** like
> `cannot use uint32(compression) ... as _Ctype_heif_compression_format`:
> the Go binding version must match the libheif Homebrew installed. Fix with
> `go get github.com/strukturag/libheif@v<version>` where `<version>` is what
> `pkg-config --modversion libheif` prints, then rebuild. A warning about
> `duplicate libraries: '-lheif'` is harmless.

### webview_worker — native WebKit preview window

```bash
cd cmd/webview_worker
CGO_ENABLED=1 go build -o ../../webview_worker
cd ../..
```

**Verify:**

```bash
ls -la heic_worker webview_worker
```

If either is missing vimango still works: HEIC images fall back to a Python
path (step 10) or don't render; the web preview opens in your browser instead
of its own window.

---

## 10. Optional: Python HEIC fallback

A second, pure-Python HEIC decode path used when `heic_worker` can't handle a
file. macOS's `python3` (installed with the Command Line Tools) is fine:

```bash
python3 -m venv .venv
.venv/bin/pip install Pillow pillow-heif
```

Skip freely — it degrades gracefully.

---

## 11. Build vimango itself

From the vimango root:

```bash
CGO_ENABLED=1 go build --tags="fts5,cgo"
```

**The tags are not optional.** `fts5` compiles SQLite's full-text-search
module into the binary — without it vimango starts but **fails on every note
save** with `no such module: fts5`. CGO is required because libvim is the only
vim engine.

**Verify:**

```bash
ls -la vimango
./vimango --help | head -5
```

---

## 12. First run

Generate the config file and local databases:

```bash
./vimango --init
```

This writes `config.json` and creates `vimango.db` (notes) and
`fts5_vimango.db` (search index). Then, **inside kitty or Ghostty**, from the
vimango directory:

```bash
./vimango
```

You should see the organizer (note list) on the left and a note pane on the
right. Basics to try:

- `a` — create a new note title, type it, press Enter, then Enter again to
  edit the note body in vim; `Esc` then `:x` saves and returns.
- `:help` — command list. `/` — search. `:q` — quit.

**Image smoke test:** create a note containing
`![screenshot](images/vimango_screenshot.png)` (that file ships in the repo)
and view it — the image should render inline in the terminal.

> **Always run vimango from its own directory.** It resolves `config.json`,
> the style files, both databases, `.venv`, and the worker binaries relative
> to the current directory. A shell alias keeps that convenient:
>
> ```bash
> echo 'alias vimango="cd ~/code/vimango && ./vimango"' >> ~/.zshrc
> ```

---

## 13. Optional integrations

Configured in `config.json` (created by `--init`; it's `.gitignore`d because
it can hold secrets):

- **Google Drive images** — render `![alt](gdrive:<file id>)` markdown from
  your Drive. Needs a Google Cloud OAuth credential; full walkthrough in
  [README.md → Google Drive Setup](./README.md#google-drive-setup-optional).
- **Remote sync (PostgreSQL)** — sync notes across machines. Fill in the
  `postgres` block; leave it empty to stay local-only. vimango works fully
  offline; connectivity is only checked when you run a sync.
- **Claude deep research** — stores research results as notes. Add
  `"claude": { "api_key": "..." }` or set `VIMANGO_CLAUDE_API_KEY`.

---

## 14. Updating later

```bash
cd ~/code/vimango
git pull
git -C ../glamour pull                          # keep the fork in step
CGO_ENABLED=1 go build --tags="fts5,cgo"        # ALWAYS with the tags
```

`libvim.a` and the workers only need rebuilding if libvim or their
dependencies change (e.g. after `brew upgrade libheif`, rebuild
`heic_worker`; bump the binding version first if cgo type errors appear —
see step 9).

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `xcrun: error: invalid active developer path` | Command Line Tools missing | step 1 |
| `brew: command not found` in a new terminal | shellenv not in `~/.zprofile` | step 2 |
| `undefined: ansi.DecodeKittyTextSizeMarkers` / `undefined: makeRaw` | glamour fork or `go.work` missing | step 5 |
| `hunspell/hunspell.h: file not found` | hunspell not installed (or Intel paths) | steps 6–7 |
| Spell check accepts everything | dictionaries missing from `/usr/share/hunspell` | step 7 |
| linker can't find `libvim.a` | not built, or `go build` not run from vimango root | step 8 |
| `exec: "pkg-config": executable file not found` | pkg-config missing | step 6 |
| `cannot use uint32(...) as _Ctype_heif_*` | libheif binding/library version mismatch | step 9 note |
| `no such module: fts5` when saving a note | built without `--tags="fts5,cgo"` | step 11 |
| Images don't render, notes otherwise fine | terminal lacks kitty graphics (e.g. Terminal.app) | step 3 |
| HEIC images specifically don't render | `heic_worker` missing/wrong arch | step 9 (non-fatal) |
| Web preview opens in browser, not a window | `webview_worker` missing | step 9 (non-fatal) |
| `config.json` / database errors at startup | first-run init never done, or wrong CWD | step 12 |
