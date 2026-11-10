# CLAUDE.md

This file provides guidance to Claude when working with code in this repository.

## Build Commands
- **Linux/Unix with CGO**: `CGO_ENABLED=1 go build --tags=fts5` (includes libvim, sqlite3)
NOTE: When updating, fixing or adding to the code, the CGO build is the most comprehensive to ensure all features work as expected.
NOTE: `fts5` is the only tag this build needs. Do **not** add `cgo` to the tag list — `cgo` is one of Go's automatic build constraints, set by the toolchain whenever cgo is in use, so `CGO_ENABLED=1` already satisfies every `//go:build cgo` file. Passing `-tags=cgo` selects an identical file set (verified with `go list`) and is only misleading: with `CGO_ENABLED=0` it does not enable cgo, it just pulls in `//go:build cgo` files that don't import `"C"`.
NOTE: spell checking needs no build tag and no external library: it is vim's own, from libvim, and reads the dictionary in `spell/` (copied there with `libvim.a` by `scripts/build-libvim.sh`). See "Spell Checking" below. (hunspell and the `spell` tag were removed 2026-11-08.)
NOTE: CGO builds require `libvim.a` in the project root. See "Building libvim.a" below.
NOTE: The glamour fork comes in through a committed `replace` in `go.mod`; a fresh clone needs no extra checkout. See "Glamour fork" below.

### Glamour fork

vimango builds against **[slzatz/glamour](https://github.com/slzatz/glamour)**, not upstream `charmbracelet/glamour`; the fork adds kitty image and text-sizing support (`glamour.WithKittyImages`, `ansi.DecodeKittyTextSizeMarkers`, `ansi.SetKittyTextSizingEnabled`). `go.mod` still requires `github.com/charmbracelet/glamour` — the fork's own `go.mod` keeps that module path, so no import changes — and a committed `replace` redirects it to a pinned commit of the fork on GitHub:

```
replace github.com/charmbracelet/glamour => github.com/slzatz/glamour v0.3.1-0.20260704003224-42bdbc1bf339
```

`go build` downloads the fork like any other dependency, and the one `go.mod` covers the `cmd/*` builds too. History: a local-path `replace` (`../glamour`) gave way to a gitignored per-machine `go.work` in 54ccf51 so no local path was committed; on 2026-11-08 this remote `replace` superseded both, so new machines no longer clone the fork.

A fork change reaches vimango only once it is pushed and the pin moves:

```bash
go mod edit -replace github.com/charmbracelet/glamour=github.com/slzatz/glamour@<commit>
go mod tidy
```

To iterate on the fork without pushing, a local `go.work` (`go work init . ../glamour`, still gitignored) overrides the `replace` — the workspace copy wins, silently. Delete it afterwards: a leftover `go.work` builds against whatever `../glamour` holds, not the pin. `go env GOWORK` shows whether one is active; `go list -m -f '{{.Dir}}' github.com/charmbracelet/glamour` shows which copy a build uses.

### Building libvim.a (CGO Prerequisite)

The static library `libvim.a` is not checked into the repo (it differs per platform). Build it from **[slzatz/libvim](https://github.com/slzatz/libvim)** — *not* onivim/libvim — and place it in the project root (next to `go.mod`):

```bash
./scripts/build-libvim.sh     # macOS and Linux; --clean, --tests, --no-pull
```

That is the normal path on every machine, and what INSTALL.md, MACOS_SETUP.md and README point to. It clones the fork to `../libvim` (or `$LIBVIM_DIR`) if needed, **refuses** a checkout whose `origin` isn't slzatz/libvim, that isn't on `main` (the fork's `master` until 2026-11-09; the script moves an old `master` checkout across when it's a fast-forward), or that has tracked changes, pulls `main`, builds with the fork's own `build-macos.sh` / `build-linux.sh` (which own the configure flags and reconfigure only when they change; on macOS the script installs ncurses/gettext first), copies `src/libvim.a` in and prints the libvim commit. Until 2026-11-08 this script cloned upstream onivim/libvim and never pulled an existing checkout — a machine set up from the old INSTALL.md / MACOS_SETUP.md may have an upstream or stale `../libvim`, which the script now catches. Rebuild after every fork change: vimango as of 2026-11-08 does not link against a `libvim.a` older than fork `ed0f2e241` (`undefined: vimSetMappingsEnabled`), and the spell checking needs the fork's spell API and its suggestion-search cap (fork commit `b00827d76`; older builds fail with `undefined: vimSpellGetErrors`, or reject `timeout:` in `'spellsuggest'` so `z=` can freeze on long words). (`spelling_restore` was merged into the fork's `main` on 2026-11-09, so the script builds it; a `../libvim` still on that branch must `git fetch && git switch main` first.) vim's reports on the message line ("5 lines yanked") need fork `77fc72a6d` (2026-11-10): an older `libvim.a` links and shows errors only. Search highlighting needs fork `a036ecafe` (2026-11-10): older builds fail with `undefined: vimSearchGetHighlightEnabled`, which arrived in the same commit as the fix that makes libvim report every match, so a stale `libvim.a` cannot quietly highlight every other one. Ignoring acronyms and mixed-case words needs the fork's `'spelloptions'` (`bca64b35e`): an older `libvim.a` links fine, the `set spelloptions=noallcaps` in vimango.vim fails silently, and acronyms are flagged again.

**Rebuild once with `--clean` on any machine whose `libvim.a` was built between fork `e0553a273` (2026-11-09, `'spelloptions'`) and `532f73b8d`.** Until `532f73b8d` the fork's `src/Makefile` had no header dependencies for `spell.o` and `spellfile.o`, so an incremental build after the `structs.h` change in `e0553a273` kept a `spellfile.o` compiled against the old `synblock_T`. It links and spell checking mostly works, but `zg` (any word added to `'spellfile'`) segfaults in `spell_add_word`. Pulling `532f73b8d` doesn't repair such a checkout by itself — `structs.h` is no newer than the stale object — so run `./scripts/build-libvim.sh --clean`. `TestSpellZgWritesSpellfile` (`vim/spell_test.go`) crashes against a `libvim.a` built that way. More generally, any object the fork adds to its build needs a line in that Makefile's generated `### Dependencies:` section, or header changes won't rebuild it.

That repo is a permanent fork. Upstream onivim/libvim is unmaintained and nothing goes back to it, so fixes live as ordinary commits on the fork's `main`; there is no patch queue to apply. Building from a pristine upstream clone produces a `libvim.a` that is missing them. Its `PATCHES.md` lists what diverges — read it before changing anything in that tree, since vimango is not necessarily its only consumer.

The divergence that matters most here: Visual-mode `I`/`A` are vim's `OP_INSERT`/`OP_APPEND` operators again, so `ctrl-v jjI- <esc>` prefixes every line of the block. Upstream libvim instead reported one cursor per block line through `cursorAddCallback` and did a single plain insert, leaving replication to the embedder (Onivim 2) — vimango never registered that callback, so the block insert was silently dropped. Regression tests: `vim/blockops_test.go` and `blockedge_test.go` here, `src/apitest/visual_mode.c` in libvim.

NOTE: a stale `libvim.a` is invisible — the Go build succeeds and the behavior is simply missing. If a libvim-level fix does not appear to take effect, check that the checkout you built from is the fork and is current (`git log --oneline -1`), then rebuild `libvim.a` and copy it here.

**macOS by hand** (Apple Silicon / Homebrew; the fork's `build-macos.sh` does this):
```bash
brew install ncurses gettext
cd /path/to/libvim/src
./configure --disable-selinux --with-tlib=ncurses \
  CFLAGS="-I/opt/homebrew/opt/ncurses/include -I/opt/homebrew/include \
    -Wno-error=implicit-function-declaration -Wno-error=implicit-int \
    -Wno-error=int-conversion -Wno-error=incompatible-function-pointer-types \
    -Wno-error=unused-but-set-variable -Wno-error=deprecated-non-prototype \
    -Wno-error=implicit-int-float-conversion" \
  LDFLAGS="-L/opt/homebrew/opt/ncurses/lib -L/opt/homebrew/lib"
make libvim.a
cp libvim.a /path/to/vimango/
```

`-Wno-error=implicit-int-float-conversion` is required on current clang; without it the build stops at `evalfunc.c:3751` on `VARNUM_MAX` (`-Werror,-Wimplicit-const-int-float-conversion`). Verified by configuring and building a fresh clone from scratch — an existing checkout may already carry the flag in `auto/config.mk` and so not show the failure.

**Linux:** use the fork's build script, which configures with the right flags and builds `src/libvim.a`:
```bash
sudo apt-get install make libtinfo-dev libacl1-dev   # Debian/Ubuntu
sudo pacman -S --needed make ncurses acl             # Arch
cd /path/to/libvim
./build-linux.sh            # --clean to rebuild from scratch, --tests to run the API tests
cp src/libvim.a /path/to/vimango/
```

By hand, the equivalent is:
```bash
cd /path/to/libvim/src
./configure --disable-selinux \
  CFLAGS="-fPIC -Wno-error=implicit-int -Wno-error=implicit-function-declaration"
make libvim.a
```

The two `-Wno-error` flags are needed on GCC 14+ (verified on GCC 16.2, Arch, 2026-10-10). They are for `./configure`'s probe programs, which use implicit int and implicit declarations. With plain `CFLAGS=-fPIC`, configure stops at `checking for tgetent()... NOT FOUND!` even with ncurses installed. Builds from libvim older than the GCC 16 fixes (see its `PATCHES.md`) also fail to compile `ex_cmds.c` under the Makefile's `-Werror` and race under `make -j`, so update the checkout first. If configure complains that `CFLAGS` has changed since the previous run, `rm src/auto/config.cache` and re-run; the script does this itself.

**Notes:**
- The CGO linkage flags in `vim/vim.go` reference `libvim.a` as a bare filename resolved from the build working directory (project root).
- **The `vim` package is the whole binding**: `vim/vim.go` and `vim/spell.go` are the cgo, and the application calls them directly (`vim.SendKey` calls `vimKey`). Until 2026-11-10 it was three layers — `vim` forwarded to an `Engine` interface (`vim/interfaces`), implemented by a wrapper that called a separate `vim/cvim` package — left over from switching between libvim and the pure-Go engine. A buffer is a `*vim.Buffer`. See `vim/README.md` for adding a binding.
- `vim/src/`, `vim/proto/` and `vim/auto/` (`config.h`, `osdef.h`) are the libvim headers the bindings compile against, copied from the fork; they work on both platforms and do not need regenerating.
- CGO is required: libvim is the only vim engine (the pure-Go `govim` engine and its adapter layer were removed 2026-07; Windows and `CGO_ENABLED=0` builds are unsupported).

### Building heic_worker (CGO HEIC Prerequisite)

The CGO HEIC decoder shells out to a separate `heic_worker` subprocess (built from `cmd/heic_worker/main.go`) so libheif crashes can't take down vimango. Like `libvim.a`, this binary differs per platform and is excluded from version control via `.gitignore`.

Requires `libheif`, plus `pkg-config` to locate it (the build fails with `exec: "pkg-config": executable file not found` without it):
- **Arch Linux:** `sudo pacman -S libheif pkgconf`
- **Debian/Ubuntu:** `sudo apt-get install libheif-dev pkg-config`
- **macOS:** `brew install libheif`

Build:
```bash
cd cmd/heic_worker
CGO_ENABLED=1 go build -o ../../heic_worker
```

`heic_cgo.go` looks for `heic_worker` next to the main executable, then in the current directory. If it's missing or built for the wrong architecture, `IsHEICAvailable()` returns false and HEIC images fail to render; nothing else is affected, and vimango itself builds and runs without libheif either way (see "HEIC Image Support" below).

- Run: `go run main.go`

### Tests
NOTE: Generally we have not been running tests but have tested key functionality manually.
- Test: `go test ./...`
- Test single package: `go test ./path/to/package`
- Test single function: `go test -run TestFunctionName`
- Sync integration tests (`internal/sync/sync_integration_test.go`) simulate several clients against a throwaway Postgres and are skipped unless `VIMANGO_TEST_PG` is set to a DSN (no `dbname`) for a server they may create databases on — each test makes and drops its own. With Homebrew's `postgresql@17`:
  ```bash
  initdb -D /tmp/vtpg -U postgres --auth=trust
  pg_ctl -D /tmp/vtpg -o "-p 54329 -k '' -c listen_addresses=127.0.0.1 -c TimeZone=UTC" -l /tmp/vtpg.log start
  VIMANGO_TEST_PG='host=127.0.0.1 port=54329 user=postgres sslmode=disable' CGO_ENABLED=1 go test --tags=fts5 ./internal/sync/
  ```
  `-k ''` because the socket path under a long `$TMPDIR` exceeds the 103-byte limit. Never point it at the real server.

## Runtime Options
- `--help`, `-h`: Display help message with all available options and exit
- `--editor`: Boot directly into editor-only mode (no organizer). Sets an immutable `editorOnly` session flag that blocks all editor→organizer handoffs. Used by host apps (vimango_hybrid) that provide their own native organizer. One note per process, chosen by `--open <id>` at boot: there is no ex command to switch notes (the `:open <id>` the host once injected was removed 2026-11-07, once every host editor became a one-session cold spawn). The host drives the editor over the pty with ex commands such as `:checkstale`, `:q` and `:x`. **A process exit after `:q` or `:x` is the host's only "saved" signal**, so the editor must close only when the buffer's text is in the database (or on `:q!`): `:x` writes a modified buffer and stays open, buffer still modified, if the write fails — a database error, or an `UPDATE` that matched no row because the note was deleted underneath (`closeNote`, `Editor.save`, `updateNote`; tests in `editor_save_test.go`). Until 2026-11-10 `:x` reported a failed write and quit anyway.
- `--open <id>`: With `--editor`, open note `<id>` at boot (avoids racing pty injection against startup)
- `--render-html <id>`: Headless: render note `<id>` as a standalone HTML document on stdout and exit. No terminal probes, no vim, no raw mode — safe to spawn with stdout piped (drives vimango_hybrid's preview pane). Google Drive images are inlined as data URIs via vimango's own auth/cache when configured; otherwise image markdown passes through untouched.
- `--migrate`: Bring this machine's `vimango.db` up to date with how this build stores it, print what changed, and exit. Optional — every start does the same quietly; see "Local database migrations".
- Spell check: vim's own, via libvim — `:spell` toggles it; see "Spell Checking"

### Host App Integration (vimango_hybrid)
`RenderNoteAsHTML(title, markdown, linkNotes)` (`note_html.go`) renders one document style, for `--render-html` in an embedded host pane: no `<h1>` and left-aligned content — the host shows the title in its own native header (the title is metadata, not part of the note), and auto-centering inside a pane reads as a large wasted indent. The TUI's webview window, which wanted an `<h1>` and a centered column instead, was removed 2026-11-08 along with `webview_worker`, `third_party/webview_go` and the GTK/webkit2gtk build dependency on Linux. config.json's `webview.accent` / `webview.code_accent` keep their historical names and still style this output.

## Help System
The application supports standard `--help` and `-h` flags to display comprehensive usage information:
- Shows all command-line options with descriptions
- Includes platform-specific behavior notes
- Displays build instructions and examples
- Reports runtime platform and architecture information
- Implementation: `help.go` with early detection in `main.go`

## Project Structure
- Main application functionality in root directory
- Command-specific code in `cmd/` directory
- Database operations in db-related files

## Key Points
- The application is written in Go.
- The vimango application is a note taking application that stores notes and their titles in a local SQLite database.
- The application operates in two main modes: an editor mode for editing notes and an organizer mode for managing and viewing notes. On the terminal screen the Organizer with the titles of notes is on the left and the Editor for editing notes is on the right.
- **One note at a time.** The TUI once let several notes be open side by side (Ctrl-H/Ctrl-L moved between them and back to the organizer); that was removed 2026-11-08. `Session.editor` is the open note's editor or nil; `:x`/`:q` close it and return to the organizer, and in `--editor` mode end the process. The TUI is kept deliberately simple — for emergencies and ssh — and vimango_hybrid is the daily driver. Removed from the TUI on the same day for the same reason: the webview window (organizer `:webview`/`:closewebview`, Ctrl-W/Ctrl-Q; editor `:webview`), the editor's in-terminal `:preview` (and the never-entered VIEW_LOG mode that shared its scrolling), and `:vertical resize` in both panes. The divider still follows terminal resizes (`moveDividerPct`) and `preferences.json`'s `edPct`.
- The application supports markdown rendering of notes to enhance the readability of the content.
- **Every note is markdown.** Notes in the folder named `code` used to be treated as source in the language their context named (`Languages`): chroma-highlighted in the editor and organizer, mirrored to a hard-coded file on `:w` (`updateCodeFile`), with `:compile`/`:run`, `:style` (the chroma theme) and `:template` (`go.template`). All removed 2026-11-08; old code notes still in the database render and highlight as markdown, and fenced code blocks inside a note are still highlighted as part of markdown.
- The ability to manage and display images is an important feature of the application.
- SQLite driver: `mattn/go-sqlite3` (CGO) everywhere — the modernc.org/sqlite
  pure-Go driver and the `--go-sqlite`/`--cgo-sqlite` selection layer were
  removed 2026-07-12 (mattn's built-in 5000ms busy timeout matters: several
  processes share vimango.db — the TUI, `--editor`, `--render-html`,
  vimango-sync, and vimango_hybrid's GRDB side). This makes the `fts5` build
  tag mandatory for every binary that writes `vimango.db` (the search index's
  triggers run inside the writer — see "Search index"); `fts5_guard.go`
  files (project root, cmd/vimango-sync, internal/localdb) turn a tag-less build
  into a compile error (`undefined: thisBinaryRequiresBuildTagFts5`) instead
  of a runtime "no such module: fts5" failure.
- The application uses vim editor functionality (libvim via CGO) for editing notes.
- The application supports full-text search using the `fts5` extension of SQLite; see "Search index".
- The application uses a terminal-based user interface for interaction.
- The application spell checks with vim's own engine; see "Spell Checking".

## Local database migrations
`vimango.db` is local to each machine — sync carries rows, not the file — so a change to how it is stored has to happen once per machine. `internal/localdb.Migrate` is where such changes go. Each step checks the database's actual state and does nothing if it is already there, so it runs quietly at every start of vimango (`InitDatabases`: the TUI, `--editor`, `--render-html`) and of `vimango-sync` (not `--probe`, which writes nothing), and `--init` runs it on a new database. `./vimango --migrate` runs it by hand and reports. A step that cannot be done because another process is mid-transaction returns an error and leaves nothing half-done; startup ignores that and the next start retries. Tests: `internal/localdb/localdb_test.go`.

Steps so far:
- **Write-ahead logging (2026-11-10).** `PRAGMA journal_mode=WAL`, recorded in the file, so every program that opens it — including vimango_hybrid's GRDB connection, which needs no change — uses it from then on. In the default rollback-journal mode a reader blocked the writer's commit and a committing writer blocked readers, so a sync and a preview render waited on each other; in WAL readers see a snapshot and neither blocks the other. Writers still take turns, so the busy timeout still matters. While the database is open there are two more files beside it, `vimango.db-wal` and `vimango.db-shm` (gitignored): recent commits live in the `-wal` file until a checkpoint, so **never back up or copy `vimango.db` with `cp`** — use `sqlite3 vimango.db ".backup copy.db"`.
- **The search index moved into `vimango.db` (2026-11-10).** See "Search index" below. `fts5_vimango.db` is no longer read or written by anything; on a migrated machine it can be deleted once you are satisfied.

## Search index
Full-text search is the FTS5 table `task_fts` inside `vimango.db`: one row per note, `rowid = task.id`, columns `title`, `note`, `tag` (the note's keywords, comma-joined), searched with `bm25(task_fts, 2.0, 1.0, 5.0)`. **Triggers keep it current** — on `task` (insert, delete, update of `title`/`note`/`tid`), on `task_keyword`, and on `keyword` (rename, delete). Definitions, and the migration that builds the index from the notes: `internal/localdb/searchindex.go`.

What follows from that:
- **No code writes the index.** `updateNote`, `updateTitle`, the keyword commands and the whole sync engine just change rows; the index changes in the same transaction. Don't add index writes anywhere — fix the trigger instead. This holds for vimango_hybrid's GRDB writes and the `sqlite3` shell too, since triggers run in whichever process writes; that process's SQLite must have FTS5 (all of them do).
- **A note is searchable as soon as it exists**, before it has synced. Keywords still hang off the server `tid` (`task_keyword.task_tid`), so tags appear once the note has one.
- **The index is derived data.** `localdb.Migrate` compares the table and trigger definitions in `sqlite_master` with the ones in the code and rebuilds everything from the notes when they differ — so changing a trigger's SQL in `searchObjects` re-indexes every machine at its next start (about half a second for 5,900 notes), and a damaged index is repaired the same way. `INSERT INTO task_fts(task_fts) VALUES ('integrity-check')` verifies one.
- Anything that drops and recreates `task` loses the triggers on it; `Migrate` puts them back.

History: until 2026-11-10 the index was the table `fts` in a second file, `fts5_vimango.db`, keyed by the server `tid`, and each writer updated it by hand in a separate, non-atomic write. Hybrid never wrote it (so its title edits were unsearchable), a note had no row until its first sync, and the two files drifted: on this machine 13 indexed titles and 15 indexed bodies no longer matched their notes, and 74 rows belonged to notes that were gone. `errSearchIndex`, sync's FTS statements, `FTS_CLEANUP.md` and `vimango-sync --fts-db` (still accepted, ignored) all went with it. Search results on the migrated copy matched the old index's for fifteen test terms, plus the notes the old index had gone stale on.

## Markdown highlighting in the editor
The editor colours the note with chroma's markdown lexer (`highlight.go`), drawn by `drawRows` (`editor_rows.go`). **The note is highlighted a chunk at a time, and each chunk's result is cached by its text**: `markdownChunks` cuts the rows into the YAML frontmatter, each fenced code block, and each paragraph (up to and including its blank row; a multi-line `<!-- -->` comment stays whole; a run of more than 60 rows with no blank is cut). A redraw highlights the chunks in the window, and all but the one being edited come from the cache (`Editor.hlCache`, dropped when it passes 8 MB), so the cost of a keystroke does not depend on the note's length or the cursor's place in it.
- Until 2026-11-10 every redraw ran the lexer from the first line of the note to the bottom of the window: 45 ms a keystroke at the end of an 860-line note, measured in a pty; 2.6 ms now.
- A chunk's fence and comment boundaries follow chroma's own rules (a fence is ```` ``` ```` plus an optional `\w+` at column 0, closed by a row that is exactly ```` ``` ````; no closer, no fence), so a chunk highlights alone as it does in the whole note. Each goes to the lexer with a line break in front, because chroma's emphasis rules want whitespace before the marker; the frontmatter goes in bare so that it is taken for frontmatter.
- **One intended difference from whole-note highlighting**: chroma lets an unmatched backtick, `~~` or `[` run on across blank lines to the next one, however far, colouring everything between. A chunk ends them with the paragraph, as markdown does. Against the real database when this was written, 5,552 of 5,574 notes looked identical; the other 22 each had a stray backtick. A marker at the very start of a note (`**Title**` on row 1) is also highlighted now, which the whole-note pass missed.
- Tests: `editor_rows_test.go` — `TestChunkedHighlightingLooksLikeTheWholeNote` compares the two methods cell by cell; `BenchmarkRedrawAtBottomOfLongNote` and `BenchmarkWholeNote` are the after and before.

## Search highlighting
The matches of the last search are highlighted in the note editor, as with vim's `'hlsearch'` (`searchhl.go`; bindings in `vim/search.go`). vim keeps all the state: libvim says where the matches are (`Buffer.SearchMatches`, for the visible rows, on every redraw) and whether they should be showing (`vim.SearchHighlighting`, which is `v:hlsearch`) — on after `/` `?` `n` `N` `*` `#` and `:s`, hidden by `:noh` until the next of those, off with `:set nohlsearch`. No Go code tracks those commands.
- `main` sets `'hlsearch'` before sourcing `vimango.vim` (vim's default is off, and here there is no other way to see the matches); `set nohlsearch` in `vimango.vim` turns it off.
- `:noh` and `n` change what is highlighted without changing the text, so the tick check cannot see them: `editorProcessKey` compares `searchState()` (the pattern, or "" when hidden) with what `drawSearchMatches` last drew and asks for a redraw when they differ. The same comparison gives incremental highlighting for free — libvim searches as the pattern is typed.
- Matches are drawn with `highlightRowRange`, like the visual selection, so one that crosses a wrap or contains a tab lands correctly; a match is plain black-on-yellow, losing the markdown colors under it.
- **libvim had to be fixed first.** Its `vimSearchGetHighlights` skipped a match at the first column of the first line asked for and any match starting within two columns of the end of the previous one: the middle `foo` of `foo foo foo`. See the fork's `PATCHES.md`.
- A search that finds nothing says so (`E486: Pattern not found`), like every other message vim has for a key — see "vim's messages" below. Not done: the match under the cursor is not marked differently.
- Tests: `searchhl_test.go`, `vim/search_test.go`; `src/apitest/search_highlights.c` in libvim.

## vim's messages
libvim has no screen; what vim would write on its message line goes to a callback (`vimSetMessageCallback`), which `vim.CaptureMessages` collects around a call. The editor shows them on its message line in two places:
- **Every key typed in the editor** (`processKey` → `showKeyMessages`, `ex_forward.go`): errors (`E486: Pattern not found`, `E20: Mark not set`, `E353: Nothing in register`), reports (`5 lines yanked`, `4 fewer lines`, `5 more lines`, `3 lines >ed 1 time`, what `u` and Ctrl-R did, `ga`, `g Ctrl-G`'s word count) and warnings (`search hit BOTTOM, continuing at TOP`). When vim says nothing, which is most keys, the message line is left untouched, so `-- INSERT --` and vimango's own messages stay.
- **A forwarded ex command** (`forwardEx` → `showVimMessages`): the same, plus output. A report or two goes on the message line (`29 substitutions on 29 lines`); a listing (`:marks`, `:ls`) goes to the overlay `:help` uses.

Until 2026-11-10 none of this was shown, and nothing had decided to hide it. vimango had never registered the callback; and libvim itself sent only errors and a few converted commands, dropping everything that went through vim's `msg()`. The second half is fixed in the fork (`2da770b06`, `77fc72a6d`; see its `PATCHES.md`): **an older `libvim.a` links fine and shows errors only** — no new symbol marks this one, so check `libvim.a`'s commit if the reports are missing.

Two things to know:
- The wrap warning is dropped for the Enter that completes a typed `/pattern` (`endsSearch`). libvim's is wrong there — it searches again from where the incremental search left the cursor, so it reports a wrap that did not happen and misses one that did. `n`, `N`, `*` and `#` are right, and shown.
- vim says some things twice (a failed search); `showKeyMessages` shows each line once.

The organizer's title editing does not show vim's messages. Tests: `TestVimMessagesForKeysAreShown`, `TestVimReportsForKeysAreShown`, `TestForwardedCommandReports` (`ex_forward_test.go`); `src/apitest/messages_reports.c` in libvim.

## Spell Checking
## Spell Checking
Spell checking is vim's, from libvim (restored in the slzatz/libvim fork; see its CLAUDE.md for the C API). libvim finds the misspellings; vimango only draws them and shows the suggestion list. Code: `spellcheck.go`; bindings: `vim/spell.go`, `spell_callback.go`.
- **Dictionary**: `spell/en.utf-8.spl` (+ `.sug`) next to config.json, cwd-relative like vimango.vim, copied from the fork's `runtime/spell` by `scripts/build-libvim.sh`. `spell/` is gitignored. `initSpell()` (before `sourceVimrc()`, so vimango.vim can override) prepends the vimango root to `'runtimepath'` and sets `'spelllang'`, `'spellfile'` (`spell/en.utf-8.add`, where `zg` writes) and `'spellsuggest'` (`best,9,timeout:1000`: one digit picks a suggestion, and the search is capped at about a second -- it is exponential in the word's length, and before the cap `z=` on a word like `llllllllllll` froze the editor for minutes). If the dictionary is missing, `:spell` says spell check is unavailable.
- **Acronyms and mixed case**: vimango.vim sets `'spelloptions'` to `noallcaps,nomixedcase`, values only the libvim fork has: an unknown word with 2+ capitals and no lower-case letter (`NASA`, `MP3`), or with a capital after its first character and a lower-case letter (`iOS`, `NASA's`, `PDFs`), is not an error. A word capitalized only at its start is still checked (`zg` for names); skipping those mid-sentence was considered and deferred 2026-11-09 — it would leave Title Case headings and misspelled names unchecked. The rule is in libvim's `spell_check()`, so highlighting, `]s`/`[s` and `z=` agree — `]s` never stops on a word that isn't highlighted. Don't filter spell errors on the Go side for this kind of rule: `]s` would still land on the hidden word. There is no `:syntax` (`FEAT_SYN_HL` is off in libvim), so `@NoSpell` syntax groups can't do it either.
- **`:spell`** toggles `spellOn`; vim's `'spell'` follows it through `syncSpellOption()`. `'spell'` is window-local and vim restores a buffer's old window options when it becomes current again, so the sync runs before every use rather than once.
- **Highlighting**: `drawSpellErrors` (called from `drawText`) asks libvim for the errors in the visible rows on each redraw, so highlights follow edits; nothing is cached. Bad words get a background, capitalisation/rare/regional ones an underline, and in INSERT mode the word at the cursor is skipped, as in vim. Only the current buffer can be checked (libvim requirement), which is always the active editor's.
- **Normal-mode commands** go to vim untouched: `]s` `[s` `zg` `zw` `zG` `zW` `zug` and `{n}z=`. `zg` and friends change what is misspelled without changing the buffer, so the tick check cannot see them; `spellListKey` spots them from `Editor.spellKeys` and forces the redraw.
- **`z=` without a count** would block on vim's numbered prompt, so libvim calls a callback instead (`TakeSpellSuggestRequest`). `editorProcessKey` then calls `spellSuggest`, which shows the list on the message line and enters `SPELLING` mode; a digit sends vim `{n}z=` (undo, `.` and `:spellrepall` work), any other key cancels. `:suggest` is the same thing by name, and the ex dispatcher leaves `SPELLING` in place for it.
- **Tests**: `spellcheck_test.go` (editor key paths, drawing) and `vim/spell_test.go` (bindings). Both skip when `spell/en.utf-8.spl` is absent, and point `'spellfile'` at a temp file so `zg` never writes the real word list.

## Platforms
macOS and Linux, CGO builds only. Windows is unsupported — support went with the pure-Go vim engine, and the last Windows-only files (resize polling, raw mode, ANSI colors, the HEIC stub) were deleted 2026-11-10.

## Terminal Resize Handling
SIGWINCH is forwarded to `app.resizeCh` (`signals.go`); `MainLoop` then calls `handleResize()`, which re-reads the window size (`rawmode.GetWindowSize`) and redraws. See "Threading".

## Threading
**Only `MainLoop` (`app.go`) draws, changes editor/organizer/screen state, or calls libvim** — libvim is not thread-safe and the drawing code keeps no locks. Every other goroutine computes and posts an event to a channel that `MainLoop` selects on:
- key reader → `keyEvents`
- SIGWINCH → `resizeCh` → `handleResize()`
- image loading for the preview (`prepareImages`, `organizer_render.go`: Drive download, disk read, decode — nothing else) → `RenderManager.resultCh` → `applyResult()`, which transmits the images to the terminal, runs glamour and draws

New background work follows the same shape: do the slow part off the loop, hand back data, and let the loop touch the UI. Until 2026-11-10 the SIGWINCH goroutine redrew the screen itself (reaching libvim through brace matching and spell errors while a key might be in progress), and the render goroutines transmitted images, ran glamour and replaced `o.note` — which also blanked the editor pane when a note's images finished loading after it had been opened with `:e`. `applyResult` now drops a result the pane cannot show (editor open, a notice up, another row selected); every way back to the preview calls `displayNote`, which renders afresh.

## Command System
The command system is loosely based on the vim command system and the existence of modes. The "super" modes of Organizer and Editor each have normal commands and Ex Commands.  For editing both of Organizer note titles and Editor notes, the use of libvim provides a wide range of vim commands including essentially all vim normal mode commands. The application features a comprehensive command registry system with full discoverability for both ex commands and non-vim normal mode commands:

### Help System
- `:help` - Show all available ex commands organized by category
- `:help normal` - Show all normal mode commands organized by category
- `:help <command>` - Show detailed help for specific ex command with usage and examples
- `:help <key>` - Organizer only: detailed help for a normal mode command (e.g., `:help Ctrl-D`). In the editor, `:help normal` describes the `vimango.vim` mappings instead — normal mode there is plain vim
- `:help <category>` - Show all commands in a specific category (e.g., `:help Navigation`)
- `:h` - Short alias for help command

### Ex Command Organization
**Organizer Ex Commands (34 commands in 9 categories):**
- **Navigation**: open, opencontext, openfolder, openkeyword
- **Entry Management**: new, write, refresh, set context, set folder, add keyword, e (edit), copy, deletekeywords, deletemarks
- **Search & Filter**: find, recent
- **Container Management**: contexts, newc, newf, folders, keywords
- **View Management**: sort, showall
- **Images**: convertgdrive, toggleimages, showimageinfo, refreshimageinfo, imagescale, cachewidth, clearcache, imagereset
- **Data Management**: sync
- **System**: quit
- **Help**: help

**Editor Ex Commands (13 commands in 6 categories):**
- **File Operations**: write, read, checkstale
- **Data Management**: sync
- **Editing**: syntax, number, paste, nopaste
- **Utility**: spell, suggest
- **System**: quit, exit
- **Help**: help

Those are vimango's own. **Every other ex command typed in the editor is vim's, and is forwarded to libvim if it is on the list in `ex_forward.go`** (`exAllowed`, 138 of vim's 558 commands): `:s` `:g` `:v` `:d` `:y` `:pu` `:m` `:t` `:j` `:>` `:<` `:normal` `:sort` `:retab` `:u` `:red` `:noh` `:set` `:let` `:echo` `:call` `:execute` `:map` and its relatives, `:command`, `:autocmd`, `:marks`, `:source`, `:read`, a bare line number, and more, with ranges, modifiers such as `:silent` and `|` chaining. What vim says back — output or its own error — is captured (`vim.CaptureMessages`) and shown: one line on the message line, several in the overlay `:help` uses. `runExLine` (`editor_process_key.go`) is the dispatcher; in order:
1. **Save, close and help, in any spelling vim accepts** (`:w` `:wri` `:up` `:wa` `:wq` `:x` `:exi` `:q` `:q!` `:qa` `:clo` `:h`). A note is a database row, so these write the row or close the editor (`closeNote`) and never a file. With a file name or a range they are refused (`:w notes.txt`). `:wq` is `:x`: it writes only a modified buffer.
2. **vimango's own commands, by exact name** — the registry above.
3. **vim's**, forwarded or refused by `classifyEx`.

Why a list of what is allowed rather than of what is not: libvim is vim with no screen, one window and one buffer, and commands outside that do real harm, quietly. Measured: `:w name` writes the file; `:e x` and `:bn` leave the note's buffer, after which every command edits some other buffer; `:cd` moves the process's working directory, which vimango finds `config.json` and the database relative to; bare `:let` (list variables) never returns from libvim — it waits inside the C call for a key to page with, freezing the editor. So any built-in command not on the list is refused with a reason (files, windows, programs, needs a prompt). A name vim's table does not have — a user-defined `:Command`, a typo — is forwarded, and vim answers (`E492`).
- Names are resolved as vim resolves them: `vim.FindExCommand` is a port of vim's `find_command` over the command table in `vim/src/ex_cmds.h`, which is embedded in the binary, so `:wri`, `:sav`, `:bn` mean here what they mean in vim and the table cannot drift from the library.
- The check covers every command on a line joined with `|` and the command given to `:g`. It does not look inside vimscript: `:execute`, `:normal` and `:call` run what they are given. The list guards against habit, not intent.
- After every forwarded command `forwardEx` verifies vim is still on the note's buffer, in normal mode, and in the same working directory, and puts back whatever is not.
- **To forward another command**, add it to `exAllowed` and run the tests: `TestForwardedCommandsDoNotHang` (`ex_hang_test.go`) runs every forwarded command, bare and with arguments, each in its own process with a time limit — that is how the `:let` hang was found — and `TestForwardedListIsSafeInLibvim` checks buffer, mode, directory and disk afterwards. `TestExPolicyCoversVimsTable` fails if a command that takes file names lands on the list. Keep `editorVimExHelp` (the tail of the editor's `:help`) in step.
- Known gaps, all libvim's: `:registers`, `:history`, `:messages`, `:version` and a bare `:map` produce no output through the message callback (they do no harm); `:s` with the `c` flag is refused, having no prompt.

Removed 2026-11-08 as TUI holdovers (PDF and printing belong in vimango_hybrid): editor `:pdf`, `:pdf-goldmark`, `:print`, `:ha`, `:save`; organizer `:print`, `:ha`, `:printlist`, `:save` — and with them the `mdtopdf`, `goldmark-pdf` and `gofpdf` dependencies.

### Key Interception

The editor intercepts **no** normal-mode keys: every key reaches vim. Editing
shortcuts are vim mappings in `vimango.vim` (below); anything wanting an
argument or a name is an ex command. The organizer keeps its own single-byte
`normalCmds` table (no prefixes, no multi-key sequences, no leader — the
`<leader>` prefix matcher and the `<C-w>>`/`<C-w><` pair were removed
2026-09-13).

This matters because `<Space>` is a real vim motion (`l`): the old leader
swallowed it, and swallowed the following key too when the pair didn't match a
registered command.

### vim mappings: `vimango.vim`

New key bindings belong in `vimango.vim` (project root, cwd-relative like
`config.json`), not in `normalCmds`. It is sourced once at startup
(`sourceVimrc`, `vimrc.go`), so `--editor` processes get it too; a missing file
is fine and errors in it are silenced. It holds ordinary vimscript — mappings
and the functions they call:
- visual `-` / `#` toggle the selected lines as a bulleted or numbered (`1. ` on
  every line) markdown list, and switch a list of one kind to the other
  (`VimangoToggleList`)
- Ctrl-B / Ctrl-I / Ctrl-E toggle bold / italic / code on the word under the
  cursor (normal) or a one-line selection (visual); a different key swaps the
  style (`VimangoDecorate`). Ctrl-I is Tab to a terminal, so Tab does italic in
  normal and visual mode. Ghostty — and so vimango_hybrid — sends Ctrl-I as the
  kitty-protocol `ESC [105;5u` instead; `terminal.ReadKey` decodes CSI-u
  Ctrl+letter back to its control byte and drops any sequence it can't use
  whole. Before that, the unparsed tail `;5u` reached vim as keys: "undo five
  times"

Both are string edits on the line (`getline`/`setline`), never `iw`/`gv`: after
a decoration the text has moved, so vim's word motions and the `'<`/`'>` marks
no longer describe it. A decoration "word" runs to whitespace or sentence
punctuation with `*` and `` ` `` counted in, so the cursor anywhere on
`**word**` finds it and toggles it off.

Until 2026-11-08 `:map` did nothing here: upstream libvim's `vimInit` raised
`no_mapping` because Onivim 2 owned keybindings itself, so mappings were stored
(`maparg()` read them back) but never expanded. The fork's
`vimSetMappingsEnabled()` lifts that (see its `PATCHES.md`); an older
`libvim.a` now fails to *link* (`_vimSetMappingsEnabled` undefined) rather than
silently dropping mappings.

Expansion is on only while a key the user typed in the note editor is forwarded
(`vim.SendUserKey` around `sendToVim` in `editorProcessKey`). Every key vimango
sends itself — the `:` forwards, `<c-e><c-u>`, ex commands' keys — runs with it
off, so a mapping cannot rewrite vimango's internal key sequences; the organizer
never expands mappings. Limits, from libvim:
- **single-key left-hand sides only** — a two-key lhs (`jk`, `,b`) swallows its
  first key, because libvim inserts later keys ahead of the waiting one
- no `<C-r>` in an insert-mode rhs (inserts `^R`); call a function instead
- `<esc>` never reaches a mapping: vimango sends it to vim itself
- vim 8.1 wants a modifier before the range: `:<C-u>keeppatterns '<,'>s/…`,
  not `:keeppatterns s/…` from VISUAL (E488)

Ctrl-B/E/I were Go intercepts with `:bold`/`:italic`/`:code` ex forms until
2026-11-08; both moved into `vimango.vim` and the ex commands were dropped.
A range in front of a command (`'<,'>` is what vim writes when `:` is pressed in VISUAL) is
skipped the way vim skips it (`skipExRange`) to find the command name, and the whole line,
range included, is what reaches vim.

### Normal Mode Command Organization
**Editor:** none. The editor intercepts no normal-mode keys (`normalCmds`, `NormalModeKeyHandler` and the editor's normal-command registry were removed with Ctrl-H/Ctrl-L on 2026-11-08); every key goes to vim. Markup and list shortcuts are vim mappings in `vimango.vim`, and the editor's `:help normal` lists them (`editorNormalModeHelp` — keep it in step with that file).

**Organizer Normal Mode Commands (12 commands in 3 categories, plus `:`):**
- **Entry Actions**: m (mark), Ctrl-D (delete), Ctrl-A (star), Ctrl-X (archive), Ctrl-F (new folder), Ctrl-C (new context)
- **Navigation**: Space or PgDn / b or PgUp (page the rendered note down / up by half a pane, as Space/Shift-Space and PgDn/PgUp do in vimango_hybrid), Ctrl-J / Ctrl-K (one line down / up), Home (back to its top). The same keys scroll `:help` and the other notices, in the organizer and the editor. Space and b are there because ssh and rex often don't carry Ctrl-J/K. Shift-Space can't be the way back as it is in hybrid: terminals send it as a bare space (and `csiUKey` decodes the kitty protocol's form to one too), so it pages down like Space. Taking Space and b costs title editing vim's `l` and `b`.
- **Information**: Ctrl-I (show entry info)
- `:` opens the ex command line

### Enhanced Error Messages
- Smart command suggestions for typos using fuzzy matching from both ex and normal command registries
- "Did you mean" suggestions when commands are not found
- Helpful guidance to use `:help` for ex commands or `:help normal` for normal mode commands

### Implementation Details
- **File**: `command_registry.go` - Core command registry system with metadata and key display helpers
- **File**: `organizer_normal.go` - Organizer normal mode command registration (the editor has none)
- **Files**: `editor_cmd_line.go`, `organizer_cmd_line.go` - Enhanced help system integration
- **Backward Compatible**: All existing commands and key bindings work unchanged
- **Type Safe**: Uses Go generics for type-safe command function signatures
- **Self-Documenting**: Help text is co-located with command definitions for both ex and normal commands
- **Key Display**: Human-readable key representations in help (e.g., `\x04` displayed as `Ctrl-D`)
- **Extensible**: New commands require help metadata, ensuring documentation stays current

## Markdown Rendering Configuration
The application uses the glamour library to render markdown with custom styling for previews and note display.

### Glamour Style Configuration
Style files are configured in `config.json`:
```json
{
  "glamour": {
    "style": "darkslz.json"
  }
}
```

### Style File Requirements
- **Required Files**: At least one of the following must exist:
  - Configured style file (specified in config.json)
  - `default.json` (fallback style)
- **File Location**: Style files must be in the application's working directory
- **File Format**: JSON format compatible with glamour's style specification
- **Startup Validation**: Application validates style file existence at startup and exits gracefully if neither file is found

### Style File Fallback Logic
1. Try configured style from `config.json` (`glamour.style`)
2. Fall back to `default.json` if configured style is missing
3. Exit with clear error message if neither file exists

### Error Handling
If style files are missing, the application displays:
```
Error: glamour style files not found:
  Configured style: <filename>
  Fallback style: default.json
Please ensure at least one of these files exists
```

## Terminal Graphics Protocol Support

The application uses the kitty graphics protocol for inline image display. This protocol is supported by multiple terminal emulators:

### Supported Terminals
- **Kitty**: Full support including images, Unicode placeholders, and text sizing (OSC 66)
- **Ghostty**: Supports images and Unicode placeholders; text sizing not yet supported (falls back gracefully)
- **Rex** (superlogical's tmux-like multiplexer): images, treated like Ghostty. Rex sets `TERM=xterm-256color`, so it is detected by `TERM_PROGRAM=rex` (added 2026-11-09; before that it needed `TERM=xterm-ghostty ./vimango`)
- **Other terminals**: detection reads environment variables only and never queries the terminal, so a terminal that implements the kitty graphics protocol but isn't named in `IsTermKitty()` gets no images. Launching with `TERM=xterm-ghostty` turns them on (graphics without kitty text sizing); `TERM=xterm-kitty` also turns on text sizing and runs `kitty --version`

### Feature Detection
The application automatically detects terminal capabilities:
- `IsTermKitty()` in `term_misc.go` - Detects kitty graphics protocol support from `TERM`, `TERM_PROGRAM` and `GHOSTTY_RESOURCES_DIR` (kitty, ghostty, rex)
- `isActualKittyTerminal()` in `kitty_capabilities.go` - Distinguishes actual kitty from other compatible terminals
- Text sizing (OSC 66) is only enabled for actual kitty terminal; other terminals get standard ANSI heading styles

### Text Sizing Protocol
The kitty text sizing protocol (OSC 66, added in kitty 0.40.0) allows scaled headings in markdown rendering:
- Configured via glamour style files (e.g., `darkslz.json`) using `kitty_scale`, `kitty_numerator`, `kitty_denominator`, `kitty_valign` properties
- Automatically disabled for terminals that don't support it (like ghostty)
- Headings fall back to standard ANSI styling (bold, colors) when text sizing is unavailable

### Environment Variable Override
- `VIMANGO_DISABLE_KITTY_TEXT_SIZING` - Escape hatch to disable text sizing if it causes issues

### Image Commands
- `:toggleimages` (`:ti`) - Toggle inline image display on/off
- `:imagereset` (`:ir`) - Clear terminal image cache and rerender current note
- `:imagescale [+|-|N]` - Scale images up, down, or to specific column width
- `:clearcache` (`:clc`) - Clear disk image cache

### Implementation Files
- `kitty_capabilities.go` - Terminal detection and capability flags
- `term_misc.go` - `IsTermKitty()` function for protocol support detection
- `kitty_placeholders.go` - Kitty graphics protocol commands
- `organizer_display.go` - Image rendering and placeholder handling

## Image Management
- The markdown notes may contain image placeholders of the form ![text](imagepath) where typically that image path is the id of an image file in google drive.
- Google drive image files are represented, for example, as ![2072 in lily_2025](gdrive:19_FwuxjvgIwxn-b4Ia75DrXRGLZeB2fe) where the string after gdrive: is the file id in google drive.
- An important feature of the application is the caching of images since downloading from google drive introduces some delays.
- There are two levels of image caching:
  - **Disk cache** (`./image_cache/`): Stores images as base64 encoded PNG files. Persistent across sessions and terminal-agnostic.
  - **Terminal memory cache**: The kitty graphics protocol caches images in the terminal emulator for fast display. Reset on each application launch.
- The disk cache works with any kitty-graphics-compatible terminal (kitty, ghostty, etc.)

## HEIC Image Support
HEIC/HEIF images (the iPhone camera format) are decoded by libheif running in the separate **`heic_worker`** binary — build it as in "Building heic_worker" above. That is the only decoder in a supported build.

### How it works
- vimango never links libheif. `heic_cgo.go` uses go-libheif's client side, which runs `heic_worker` as a hashicorp/go-plugin RPC subprocess and sends it the image bytes; only `cmd/heic_worker` imports the C binding (`strukturag/libheif/go/heif`). So the vimango build needs neither libheif nor `pkg-config` — only the worker build does — and a libheif crash kills the worker, not vimango.
- The worker is started on the first HEIC image (`createHEICDecoder`, once per process), found next to the vimango executable or else in the current directory.
- It is **not** tied to its parent's lifetime: it outlives a clean exit holding ~136MB. `ShutdownHEICDecoder` (`heic.go`) kills it and must run before the process exits — this matters most for `--render-html`, a fresh process per hybrid preview. It is one-shot: go-libheif v1.3.0's `DeInit` leaves its initialized flag set, so the worker cannot be restarted in the same process.
- `decodeImageWithOrientation` (`term_misc.go`) checks for HEIC first; the decoded image then goes through the normal image path and is cached as base64 PNG in `./image_cache/`, so later displays need no HEIC decoding (and no worker).

### When it's unavailable
Missing worker, wrong architecture, or a corrupt file: `IsHEICAvailable()` is false or `Decode` returns an error, and that one image shows an error instead of rendering. Nothing else is affected.

### HEIC Detection
HEIC files are detected by magic bytes (ftyp box with heic/heix/hevc/hevx/mif1/msf1 brand identifiers), not by file extension, ensuring reliable detection for Google Drive images where extension may not be preserved.

### Implementation Files
- `heic.go` - Shared interface, magic-byte detection, `ShutdownHEICDecoder`
- `heic_cgo.go` (`cgo && !windows`) - the `heic_worker` decoder; the one every supported build uses
- `cmd/heic_worker/main.go` - the worker: go-libheif plugin over libheif

A Python pillow-heif decoder (`heic_pillow.go`, `heic_convert.py`, a `.venv` with Pillow and pillow-heif) was removed 2026-11-08: only a `CGO_ENABLED=0` build compiled it, and those have been unsupported since libvim became the only vim engine. An old `.venv` in the vimango directory can be deleted.

## Local-Only Operation (UUID-Based Containers)

The application supports full local-only operation without requiring PostgreSQL synchronization. This is achieved through UUID-based container identification.

### Offline Startup
Startup performs no network I/O even when PostgreSQL is configured: `InitDatabases` opens the Postgres handle lazily (`sql.Open` only validates the connection string) and does not ping. This means the TUI, `--editor`, and `--render-html` all work offline against the local SQLite databases. Reachability is checked in `(*App).Synchronize` (bounded 5s ping), which reports a friendly "sync unavailable" message when offline; sync recovers automatically once connectivity returns. The `vimango-sync` CLI still pings at boot and fails fast offline, which is intentional.

**`connect_timeout` is mandatory in the DSN** (`buildPGConnString`, `cmd/vimango-sync/main.go`). Without it lib/pq falls back to OS TCP behavior: wifi-off, DNS failure and connection-refused all fail fast, but a *blackholed* route (dropped VPN, captive portal) retries SYNs for **75s measured** on macOS. That is long enough to wedge a caller — it stranded vimango_hybrid's quit path, which awaits an in-flight sync unbounded. Set to 10s. Note this bounds the *connect* only; a server that accepts and then stalls needs a caller-side deadline.

### `vimango-sync` modes
Three, in ascending cost — pick the cheapest that answers the question:
- `--probe`: "how many changes would a sync apply?" One watermark read from SQLite (`sync` table, `machine='server'`), one Postgres `SELECT` of `(table, tid, modified, deleted)` stamps since it, and a point lookup per stamp in SQLite. Prints a single integer, applies nothing, writes to neither database, and does not run the local migrations. Cheap enough to run unattended — vimango_hybrid probes on window activation.
- `--report-only`: the full engine, reporting without applying. Use when you need the *breakdown*, not a count — it costs a dozen-plus round trips and pulls full note bodies over the wire. Its writes are client-database upkeep, not changes: adding the `server_modified` and `taskwarrior_uuid` columns to an older database, and `normalizeAdded` (below).
- (no flag): sync and apply.

The server watermark is taken two minutes *before* each sync (see the comment above `run` in `internal/sync/sync.go`), so a bare `modified > watermark` count would include this client's own recent pushes. `Probe` instead compares each stamp with the `server_modified` the local row holds and skips matches, as the iOS client's probe does. Tombstones still count, since deletions bump `modified`, except for rows this client doesn't have, because applying those is a no-op. `fetchAllChanges` drops the same rows (`dropSeen`), so "Number of changes" in the sync log — which hybrid shows as "· N changes" — agrees with `Probe`.

**Every local timestamp is SQLite's format, `yyyy-MM-dd HH:mm:ss` UTC** — `datetime('now')`/`CURRENT_TIMESTAMP` in every writer, and `to_char(added, 'YYYY-MM-DD HH24:MI:SS')` in the pull, because `ORDER BY added` and every `modified > watermark` compare the *text*. Until 2026-10-17 the pull selected a bare `added`, which database/sql turns into RFC 3339 (`…T…Z`, variable fraction); the space-vs-`T` at position 11 then sorted every pulled note after every local note from the same UTC day. `normalizeAdded`, run by every sync, rewrites those old values in place — `modified` untouched, so nothing pushes — and skips anything `strftime` can't parse, since `added` is NOT NULL and a failed UPDATE would stop every sync.

### How It Works
- **Containers** (contexts, folders, keywords) are identified by UUID rather than PostgreSQL-assigned `tid` values
- **Tasks** reference containers via `context_uuid`, `folder_uuid`, and `keyword_uuid` foreign keys
- **Local creation**: Containers created locally generate their own UUIDs immediately
- **Sync compatibility**: The `tid` column is preserved for backward compatibility with PostgreSQL sync

### Database Schema
Containers include both `tid` (for sync) and `uuid` (for local operations):
```sql
CREATE TABLE context (
    id INTEGER PRIMARY KEY,
    tid INTEGER,           -- PostgreSQL sync ID (may be NULL locally)
    uuid TEXT NOT NULL UNIQUE,  -- Primary identifier for local operations
    title TEXT NOT NULL,
    ...
);
```

Tasks reference containers by UUID:
```sql
CREATE TABLE task (
    ...
    context_uuid TEXT DEFAULT '00000000-0000-0000-0000-000000000001',
    folder_uuid TEXT DEFAULT '00000000-0000-0000-0000-000000000002',
    FOREIGN KEY(context_uuid) REFERENCES context (uuid),
    FOREIGN KEY(folder_uuid) REFERENCES folder (uuid)
);
```

### Default Containers
- Default context UUID: `00000000-0000-0000-0000-000000000001` (title: "none")
- Default folder UUID: `00000000-0000-0000-0000-000000000002` (title: "none")

### Migration
Databases created with `--init` have the UUID columns from the start, and `MigrateToUUID()` (`app.go`) runs at every startup: idempotent, a no-op once a database has them. Every machine's database and the PostgreSQL server were migrated long ago; the standalone tools for it (`cmd/migrate_local`, `cmd/fix_uuid_sync`, `postgres_migrate_uuid.sql`) and the other one-shot commands under `cmd/` were deleted 2026-11-10 and are in git history.

### PostgreSQL Synchronization with UUIDs

The sync system now supports full UUID synchronization between SQLite and PostgreSQL:

**Container Sync**:
- Client-generated UUIDs are sent to PostgreSQL when creating new containers
- PostgreSQL assigns `tid` values which are synced back to the client
- Both `tid` and `uuid` are maintained for bidirectional sync

**Task Sync**:
- Tasks sync both `context_tid`/`folder_tid` and `context_uuid`/`folder_uuid`
- When syncing tasks created in local-only mode, `tid` is resolved from `uuid`
- Fallback to default containers if resolution fails

**Keyword Sync**:
- `task_keyword` table includes `keyword_uuid` column
- Keywords sync with both `tid` and `uuid` references

### Implementation Files
- `init.go` - Schema definitions with UUID columns, UUID generation helper
- `dbfunc.go` - Container/task CRUD operations using UUID
- `app.go` - `MigrateToUUID()` migration function
- `common.go` - Container and Entry structs with uuid fields
- `organizer_cmd_line.go` - Container assignment commands
- `synchronize.go` - Full UUID sync support for containers, tasks, and keywords
- `schema/postgres.sql` - the PostgreSQL server schema (with the UUID columns); the sync integration tests load it

## Pending Follow-ups

- **Updating another Mac to the 2026-11-10 state** (the Linux machines are being retired). Three repositories changed together: vimango (`dev`; `main` does not have these commits until `dev` is merged into it), vimango_hybrid (`dev`) and the libvim fork (`main`). On the machine, with vimango_hybrid quit:
  1. `cd ~/vimango && git pull` on the branch that has the changes.
  2. `rm -f go.work go.work.sum` — a leftover workspace silently builds against `../glamour` instead of the pinned fork commit; `../glamour` itself is no longer needed.
  3. `./scripts/build-libvim.sh` — pulls the libvim fork and rebuilds `libvim.a`. It must be fork `77fc72a6d` or later: older ones stop the build at `undefined: vimSearchGetHighlightEnabled` (before `a036ecafe`) or `vimSetMappingsEnabled`, or build and show vim's errors but none of its reports. Add `--clean` if that machine's `libvim.a` was built between fork `e0553a273` and `532f73b8d` (see "Building libvim.a"); `--clean` is always safe. The script refuses a `../libvim` that is upstream or off `main`.
  4. `CGO_ENABLED=1 go build --tags=fts5`. (`heic_worker` did not change.)
  5. `./vimango --migrate` — switches `vimango.db` to write-ahead logging and builds the `task_fts` search index from the notes, printing both lines; a second run prints "already up to date". Optional in that any start of the new vimango or vimango-sync does it quietly, but this shows the result.
  6. `cd ~/vimango_hybrid && git pull && ./scripts/run.sh` — rebuilds the app and the `vimango-sync` it bundles. vimango and hybrid must be updated together: a hybrid from before 2026-11-10 searches the old `fts5_vimango.db`, which the new vimango no longer updates.
  7. Afterwards, at leisure: delete `fts5_vimango.db` (nothing reads it), any old `webview_worker` binary and `.venv`; `"fts_db"` and `"claude"` in `config.json` are ignored and can go. Copy `vimango.db` only with `sqlite3 vimango.db ".backup copy.db"` from now on.

  Nothing changes on the PostgreSQL server or in the iOS app, and the machines can be updated one at a time: each migrates its own database.
- **Open items from the 2026-11-10 review** (none started):
  - *Organizer and structure.* Everything is `package main` around a global `app`; the database layer (`dbfunc.go`) prints to the screen through it, and `--editor` still builds an organizer and loads its rows (there are about 20 `editorOnly` checks papering over that). The organizer's ex commands repeat the same reload code about eight times (`organizer_cmd_line.go`), `setContext`/`setContextNew` are near copies, and `help()` is nested six deep. Suggested split: a store package that returns errors, the HTML renderer, the editor, with the organizer as the optional layer.
  - *Organizer bug.* `:e 123` opens note 1: ex commands are called with the position of the space, and `editNote` takes that for an id. Giving ex commands parsed arguments fixes the class.
  - *Smaller correctness.* `PRAGMA foreign_keys=ON` reaches one pooled connection, not all (`InitDatabases`; use the DSN). A `--render-html` killed by the host's SIGTERM skips its deferred `ShutdownHEICDecoder` and can orphan a `heic_worker`. `registers.json` is written non-atomically, and two editors closing at once can interleave. Messages and status text are cut by byte length and can split a multi-byte character.
  - *Image cache.* It holds two formats: entries hybrid's `--render-html` wrote are JPEG data URIs without pixel dimensions (148 of 180 on this machine), which the TUI cannot use — it downloads the image from Drive again and overwrites the entry as PNG. Letting the TUI decode those entries would end the re-download.
  - *Dead weight still in.* `MigrateToUUID()` runs at every start and is a no-op on every database in use. The "Local-Only Operation" section above describes something nobody runs and that does not fully work (search was fixed by the in-database index, but keywords still need a server `tid`). The vendored libvim headers under `vim/` are far more than the dozen declarations the bindings use.
  - *libvim (the fork).* On the Enter that completes a typed `/pattern` the wrap warning is wrong, so vimango drops it there; a typed search that wraps says nothing. `:registers`, `:history`, `:messages`, `:version` and a bare `:map` produce no output. A bare `:let` never returns (vimango refuses it). `:s` with the `c` flag has no prompt (refused).
  - *Editor.* The match under the cursor is not marked differently from the others. chroma's markdown lexer does not take ```` ```c++ ```` or an indented fence as a fence. The organizer's title editing shows none of vim's messages. `:r !cmd` is forwarded and runs the command in a shell, the one way left to start a program from the editor; whether to refuse it has not been decided.
  - *Docs.* This file is about 60 KB, loaded every session, and mostly history; the history belongs in a `docs/HISTORY.md` as in vimango_hybrid, leaving the rules and how-tos here. "Generally we have not been running tests" (Build Commands) is no longer true: the suite is run on every change.
  - *Notes with an unmatched backtick, left as they are* (the other 24 were fixed 2026-11-10; originals in `backtick-fix-originals.json.bak`): 3285, 3469, 4414, 5062 are program output or code that quotes `` `like this' ``; 2792 and 3929 have a lone backtick line inside indented code; 4837 has fences indented inside a list; 5612 ends with a closing fence that has no opener; 5905 is read by a script and was not touched.

- **Move markdown→HTML into vimango_hybrid, together with PDF export (noted 2026-11-08).** `--render-html` (`render_html.go`, `note_html.go`, plus the `callout.go` / `frontmatter.go` / `wikilink.go` goldmark extensions — ~1500 lines and the goldmark dependency) now has exactly one user: hybrid's preview pane, which spawns `vimango --render-html <id>` per preview (`PreviewRenderer.swift`). The TUI never used it; it renders with glamour (`organizer_render.go`). It lives in vimango for history (it was written for the TUI's webview window, removed 2026-11-08) and because Google Drive images — OAuth token, download, the disk cache shared with the TUI's inline images, HEIC conversion — are Go-side. Decided: not now; do it as one piece of hybrid work with PDF export, which was removed from the TUI the same day as belonging in hybrid. Once hybrid makes its own HTML (`swift-markdown`, or client-side in the WKWebView with markdown-it so the CSS carries over), WebKit's own PDF export gives PDF output from the same HTML with no PDF library. Suggested split: markdown→HTML (and the three extensions and stylesheet) moves to Swift; images stay in vimango behind a narrow interface (a command that returns a Drive image's cached file, or hybrid reading `./image_cache/` directly) so Drive access stays single-sourced while the TUI still shows images. Gains: vimango sheds the render path and goldmark, no process spawn per preview, preview changes stop needing a vimango rebuild.
- **Taskwarrior link column: done 2026-10-16.** A task note's Taskwarrior uuid is `task.taskwarrior_uuid`, synced in both directions by `internal/sync`, and no longer in the note's text. It is written once and never cleared: push and pull both COALESCE it, and every sync starts with a backfill. vimango itself never reads it. The design, the decisions taken along the way, and how it rolled out are in `docs/taskwarrior-link-column.md`. The Linux machines never got that rebuild and are being retired (2026-11-10).
