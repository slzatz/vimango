# CLAUDE.md

Guidance for working in this repository: the rules, and how to do things.

**What changed, when and why is in [docs/HISTORY.md](docs/HISTORY.md).** When a change lands, add a dated entry there and leave here only the rule it results in. Open work is at the end, under "Open items".

## What this is

vimango is a note-taking application in Go. Notes and their titles are rows in a local SQLite database (`vimango.db`), synced to a PostgreSQL server shared with the other clients (vimango_hybrid on macOS, an iOS app). Editing is vim's, from libvim through cgo.

- **Two ways in.** The TUI: an organizer (note titles, on the left) and an editor (on the right) in one terminal. And the modes a host app drives, `--editor` and `--render-html`. **vimango_hybrid is the daily driver**; the TUI is kept deliberately simple, for emergencies and ssh. PDF, printing and a richer preview belong in hybrid.
- **One note at a time.** `Session.editor` is the open note's editor or nil, and that is also where keys go: to the editor while there is one, to the organizer otherwise. No second flag records it. `:x`/`:q` close the note and return to the organizer, and in `--editor` mode end the process.
- **Every note is markdown.** Fenced code blocks inside a note are highlighted as part of markdown; old notes in the folder named `code` are markdown like the rest.
- **Layout.** The application is `package main` in the project root, around a global `app`: the TUI's organizer, the screen, the event loop, and `--render-html`. **The editor is `internal/editor`**, a package that does not import the application (see "The editor"). Beneath both: `vim/` is the libvim binding, `internal/store` is the database as the application uses it, `internal/localdb` is how the file is stored (opening it, the schema, migrations, the search index), `internal/sync` is the sync engine, `internal/atomicfile` writes the files several vimango processes share, and `internal/pane` is what the organizer and the editor both use and neither owns (the command registry, cutting text to columns, giving a key to vim, what a paste is), and `internal/frontmatter` is what counts as a note's YAML frontmatter, which the preview and the editor must agree on. `cmd/vimango-sync` and `cmd/heic_worker` are the other binaries.
- **Platforms.** macOS and Linux, cgo builds only. Windows and `CGO_ENABLED=0` are unsupported: libvim is the only vim engine and `mattn/go-sqlite3` the only SQLite driver.

## Build

```bash
./scripts/build-libvim.sh              # once, and after every libvim fork change
CGO_ENABLED=1 go build --tags=fts5
```

- **`fts5` is the only tag, and it is mandatory** for every binary that writes `vimango.db`: the search index's triggers run inside the writer. `fts5_guard.go` (project root, `cmd/vimango-sync`, `internal/localdb`) turns a tag-less build into a compile error (`undefined: thisBinaryRequiresBuildTagFts5`).
- **Do not add `cgo` to the tags.** It is one of Go's automatic constraints, set whenever cgo is in use; `-tags=cgo` selects the same files and with `CGO_ENABLED=0` only misleads.
- Spell checking needs no tag and no external library; see "Spell checking".
- Run `go` from the project root: the linker flags in `vim/vim.go` name `libvim.a` by bare filename.

### libvim.a

`libvim.a` is not in the repo (it differs per platform). `./scripts/build-libvim.sh` (`--clean`, `--tests`, `--no-pull`) builds it from **[slzatz/libvim](https://github.com/slzatz/libvim)** — not onivim/libvim — and puts it, with the spell dictionary, in the project root. It clones the fork to `../libvim` (or `$LIBVIM_DIR`) if needed, pulls `main`, builds with the fork's own `build-macos.sh` / `build-linux.sh`, and prints the libvim commit. It **refuses** a checkout whose `origin` is not the fork, that is not on `main`, or that has tracked changes — so a fork change must be committed and pushed before this script can build it. To build by hand, see the fork's `CLAUDE.md`.

- **The fork is permanent.** Upstream is unmaintained and nothing goes back to it; fixes are ordinary commits on the fork's `main`. Its `PATCHES.md` lists every divergence — read it before changing anything in that tree. The one that matters most here: Visual-mode `I`/`A` are vim's operators again, so `ctrl-v jjI- <esc>` prefixes every line of the block (tests: `vim/blockops_test.go`, `blockedge_test.go`).
- **A stale `libvim.a` can be invisible**: the Go build succeeds and the behavior is missing. If a libvim-level change does not seem to take effect, check the commit the library was built from. What each feature needs:

  | Feature | Fork commit | With an older `libvim.a` |
  |---|---|---|
  | vim mappings (`vimango.vim`) | `ed0f2e241` | link fails: `undefined: vimSetMappingsEnabled` |
  | spell checking, and the cap on `z=` | `b00827d76` | `undefined: vimSpellGetErrors`; or `timeout:` in `'spellsuggest'` is rejected and `z=` can freeze |
  | `'spelloptions'` (acronyms, mixed case) | `bca64b35e` | **links**; the `set spelloptions=` in `vimango.vim` fails silently and acronyms are flagged |
  | search highlighting | `a036ecafe` | `undefined: vimSearchGetHighlightEnabled` |
  | vim's reports on the message line | `77fc72a6d` | **links**; errors show, reports ("5 lines yanked") do not |
  | vim writes no file | `31301666c` | `undefined: vimSetFileWritesEnabled` |
  | multi-key mappings (`jk`) | `d93698bc8` | `undefined: vimInputPending` |
  | what a command lists (`:registers`, `:history`, `:print`) | `6fc194385` | **links**; those commands say nothing, and `:let` with no assignment is refused (it would hang) |
  | a typed `/pattern` is one search | `6bc47d62d` | **links**; the Enter warns of wraps that did not happen and not of ones that did, `/pat/e` reports `E486`, an empty `/` does not repeat |
  | Ctrl-R, and `"=` | `3674896fb` | **links**; Ctrl-R inserts `^R` in INSERT and types the register's name on a command line, and `"=` in NORMAL freezes the editor |
  | `input()` returns | `76bc49547` | **links**; `:echo input('x')`, or a mapping or function that asks, freezes the editor |
  | `g0`, `g^`, `gm`, `g$` by screen line | `928659d8f` | link fails: `undefined: vimSetCursorMoveScreenColumnCallback` |
  | text that is set forgets undo | `9c14a01ae` | link fails: `undefined: vimUndoClear` |
  | the column `j` and `k` want after `$` | `5838803e9` | **links**; `0$0j` goes to the end of the next line, and `$` as the first key in a note wants the end of that row only |
  | `:cq` and `<PasteStart>` end nothing | `d57f18c3c` | **links**; `:execute 'cq'` ends the process unsaved, and `<PasteStart>` in a mapping crashes it |
  | Ctrl-C stops a command that does not end | `97d397b37` | link fails: `undefined: vimInterrupt` |

- **If a header change in the fork does not seem to rebuild an object, use `--clean`.** Any object the fork adds to its build needs a line in the generated `### Dependencies:` section of `src/Makefile`, or header changes will not rebuild it (a stale `spellfile.o` once made `zg` segfault; `TestSpellZgWritesSpellfile` catches that).
- **The binding compiles against `vim/libvim_api.h`, vimango's own copy of the libvim declarations it uses** (about thirty functions, five types, a few constants) — not against the fork's headers, which the vimango build never sees. `TestHeaderMatchesLibvim` (`vim/header_test.go`) is what keeps the copy honest: it compiles `libvim_api.h` together with the fork's `libvim.h`, so a prototype that differs does not compile, and compares the size and layout of every type and the value of every constant. It needs the fork's checkout (`../libvim` or `$LIBVIM_DIR`) and is skipped without one; `build-libvim.sh` runs it after every build and fails if it does. To use something more of libvim, copy its declaration into `libvim_api.h` and add any new type or constant to the test's probe.

### The vim package

`vim/` is the whole binding: `vim.go`, `spell.go`, `search.go` and `messages.go` are the cgo, and the application calls them directly (`vim.SendKey` calls `vimKey`). A buffer is a `*vim.Buffer`. `vim/README.md` says how to add a binding.

- **libvim is one process-wide vim and is not thread-safe.** See "Threading".
- vim's modes have names: `vim.GetCurrentMode()` returns a `vim.Mode`, compared with `vim.ModeNormal`, `ModeVisual`, `ModeOpPending`, `ModeCmdline`, `ModeInsert`, `ModeNormalBusy` — never with `1`, `8` or `257`.
- **`Buffer.SetLines` is for text that is new, and forgets the buffer's undo history** (the fork's `vimUndoClear`). libvim saves nothing for undo when lines are set, so the history would go on describing the old lines: `u` after `:checkstale` had reloaded a note put a line of the old text over the new, and `:x` wrote the mixture. It is the binding that clears, so no caller can leave it out: the editor's `Open` and `checkStale`, the organizer's `readRowsIntoBuffer` and `newEntry`. Text that is to be an edit `u` undoes is typed into vim, not set. Tests: `vim/undo_test.go`, `TestCheckStaleReloadForgetsUndo` (`internal/editor/editor_save_test.go`), `TestRowsReadAgainForgetUndo` (`organizer_ex_test.go`).

### Glamour fork

vimango renders markdown with **[slzatz/glamour](https://github.com/slzatz/glamour)**, which adds kitty image and text-sizing support. `go.mod` requires `github.com/charmbracelet/glamour` (the fork keeps that module path, so imports are unchanged) and a committed `replace` pins a commit of the fork, so a fresh clone needs no extra checkout. A fork change reaches vimango only once it is pushed and the pin moves:

```bash
go mod edit -replace github.com/charmbracelet/glamour=github.com/slzatz/glamour@<commit>
go mod tidy
```

To iterate on the fork without pushing, a local `go.work` (`go work init . ../glamour`, gitignored) overrides the `replace` — silently. **Delete it afterwards**: a leftover `go.work` builds against whatever `../glamour` holds. `go env GOWORK` shows whether one is active.

### heic_worker

HEIC images are decoded by libheif in a separate `heic_worker` binary, so a libheif crash cannot take vimango down. It is not in the repo. It needs `libheif` and `pkg-config` (macOS: `brew install libheif`; Arch: `libheif pkgconf`; Debian: `libheif-dev pkg-config`):

```bash
cd cmd/heic_worker && CGO_ENABLED=1 go build -o ../../heic_worker
```

vimango itself builds and runs without it; see "HEIC".

### Tests

**Run the suite on every change**: `CGO_ENABLED=1 go test --tags=fts5 ./...` (`-run TestName` for one). libvim's own tests are `./build-macos.sh --tests` in the fork.

- **Always from the project root**: the `vim` package's linker flags name `libvim.a` by bare filename, so `go test` inside `internal/editor`, `internal/pane` or `vim` fails to link. `CGO_ENABLED=1 go test --tags=fts5 ./internal/editor/` is the editor alone.
- **After `libvim.a` changes, add `-count=1`.** `go test` keeps a package's last result for as long as its Go inputs are the same, and the library is not one of them: with an older `libvim.a` copied in, `go test ./vim/` answered `ok (cached)` for a test that fails against it (2026-11-16). `go build` does link again. `build-libvim.sh` runs its header test with `-count=1` for the same reason.
- Several tests start a process per case (`TestForwardedCommandsDoNotHang`, `TestKilledProcessTakesItsHEICWorkerAlong`); `-short` skips them.
- Spell tests skip when `spell/en.utf-8.spl` is absent.
- **A change that is not meant to alter what is drawn** (moving code, the editor's layout): `scripts/compare-drawing.py before after` runs two builds through the same keys on a pseudo-terminal, in `--editor` and in the TUI, on a scratch copy of the database, and compares every byte written. Build the "before" binary first.
- **Sync integration tests** (`internal/sync/sync_integration_test.go`) simulate several clients against a throwaway Postgres and are skipped unless `VIMANGO_TEST_PG` is a DSN (no `dbname`) for a server they may create databases on. Never point it at the real server. With Homebrew's `postgresql@17`:
  ```bash
  initdb -D /tmp/vtpg -U postgres --auth=trust
  pg_ctl -D /tmp/vtpg -o "-p 54329 -k '' -c listen_addresses=127.0.0.1 -c TimeZone=UTC" -l /tmp/vtpg.log start
  VIMANGO_TEST_PG='host=127.0.0.1 port=54329 user=postgres sslmode=disable' CGO_ENABLED=1 go test --tags=fts5 ./internal/sync/
  ```
  (`-k ''` because the socket path under a long `$TMPDIR` exceeds the 103-byte limit.)

## Runtime options

`help.go` holds the list (`validFlags`) and the `--help` text; an unknown flag is an error.

- `--editor`: boot straight into the editor; for a host app with its own list of notes. **There is no organizer in this mode: `app.Organizer` is nil**, and so is `app.RenderManager`. `InitApp` makes the organizer only for the TUI, and the editor opens its note from the store itself (`editor.Open`; `App.openEditor` is that and then showing it). So nothing on the editor's path may reach for the organizer, and nothing can: **the editor is a package of its own, `internal/editor`, which does not import the application.** An editor is made from an `editor.Host` — the store, the notice box (the screen's, `notice.go`), `OnClose` (here `App.noteClosed`) and what `:sync` answers; `App.editorHost()` is what this app gives — and an `editor.Layout`, and that is all it knows of the program around it. Something more the editor needs from outside is a field of `Host` (test: `TestEditorIsMadeFromItsHost`). `Session.editorOnly`, set once at startup, is the host's to ask, not the editor's: the screen turns it into the editor's layout (full width, no status bar, no top border; see "Layout" under "The editor") and the app into what `:sync` answers. `:sync` is refused there because the host owns sync — and refused in the TUI's editor too, where it is the organizer's once the note is closed (`noSync`): **the editor never hands over to the organizer with its note open.** One note per process, chosen by `--open <id>`, or else the note modified last; there is no command to switch notes. The host drives the editor over the pty with ex commands (`:checkstale`, `:q`, `:x`). Tests run this mode as it is, with no organizer: `editorOnlyApp` (`session_test.go`), used in `editor_only_test.go`.
  **A process exit after `:q` or `:x` is the host's only "saved" signal**, so the editor closes only when the buffer's text is in the database, or on `:q!`. `:x` on a modified buffer writes it, and stays open with the buffer still modified if the write fails — a database error, or an `UPDATE` that matched no row because the note was deleted underneath (`closeNote`, `Editor.save`, `updateNote`; tests in `internal/editor/editor_save_test.go`, and `TestFailedSaveKeepsTheNoteOpen` for the app's side).
  **What else the editor tells its host is `host_report.go`**: the terminal's title (OSC 2), `vimango id=5 modified=1 stale=0`, written by the event loop when the note opens and whenever a value changes, in `--editor` only — the TUI sets no title. `modified` is `Editor.Modified`; `stale` is `Editor.Stale`, what `:checkstale` found and could not settle (the row changed under unsaved changes, or was deleted), ended by a save that goes through, a reload, or the row back as it was read. A host takes the pairs it knows, so one can be added. **It is for what a host shows** — a mark on a window with unsaved changes, a warning for a hidden editor — **and the exit stays the only thing a host may act on where text could be lost**: the report arrives after the key that caused it. Tests: `host_report_test.go`, `TestStaleIsWhatCheckStaleCouldNotSettle` (`internal/editor/editor_save_test.go`).
- `--open <id>`: with `--editor`, the note to open.
- `--render-html <id>`: headless — print the note as a standalone HTML document and exit. No terminal probes, no vim, no raw mode; safe with stdout piped. Google Drive images are inlined as data URIs through vimango's own auth and cache when configured; otherwise the image markdown passes through.
  - `--title`: put the note's title at the top as an `<h1>`. Off for hybrid's preview pane, which has its own title bar; on for its PDF export.
  - `--wiki-links`: emit `[[refs]]` as `vimango://note/` anchors, for a host that intercepts the clicks.
- `--migrate`: bring this machine's `vimango.db` up to date, print what changed, exit. Optional; every start does the same quietly.
- `--init`: first-time setup (`config.json`, a new database). `--gdrive-auth`: sign in to Google Drive and exit.

**vimango_hybrid uses both host modes**: an `--editor --open <id>` per editing session in a pop-out, and a `--render-html` per preview and per PDF export.

**`--render-html` builds no `App`** — no organizer, no terminal detection, and the global `app` stays nil. `renderNote` (`render_html.go`) opens the store and makes an `htmlRenderer` (`note_html.go`), a value holding everything rendering takes from outside: the two colours from `config.json` (`webview.accent` / `webview.code_accent`; the names are historical), the flags, a title lookup for `[[tid:N]]` links, and where Drive images come from (`driveImages`, and the disk cache). Anything new the renderer needs goes in that struct, not through a global. It renders one style: left-aligned content, no centered column, since it is shown in a pane. Tests: `render_html_test.go`.

## Rules that cut across

### Threading

**Only `MainLoop` (`app.go`) draws, changes editor/organizer/screen state, or calls libvim.** libvim is not thread-safe and the drawing code keeps no locks. Every other goroutine computes and posts an event to a channel `MainLoop` selects on:

- key reader → `keyEvents` (a paste is one event, however long)
- SIGWINCH → `resizeCh` → `handleResize()` (`signals.go`), which re-reads the window size and redraws
- image loading for the preview (`prepareImages`, `organizer_render.go`: Drive download, disk read, decode — nothing else) → `RenderManager.resultCh` → `applyResult()`, which transmits the images, runs glamour and draws

**One exception: `vim.Interrupt`**, which the key reader calls itself on a Ctrl-C while vim is running a command (`App.readerTakes`). A command can run for ever (`:while 1 | endwhile`, a mapping that types itself again), and `MainLoop` is the goroutine inside it, so the key would wait behind it for good. libvim's `vimInterrupt` only sets a flag, as a signal handler would; nothing else in `vim/` may be called from another goroutine.

New background work follows the same shape: do the slow part off the loop, hand back data, let the loop touch the UI. `applyResult` drops a result the pane cannot show (editor open, a notice up, another row selected); every way back to the preview calls `displayNote`, which renders afresh.

### Files and the database

- **The application reads and writes `vimango.db` through `internal/store`, and the store shows nothing.** Each method returns data or an error; the caller decides what the user is told. The organizer's side of that is `organizer_store.go`: small functions that turn the store's answers into rows and an error into a message on the status line. Do not put SQL in the application, and do not reach for the screen (or `app`) from the store — the editor and `--render-html` use it with no organizer. A row that is not there is `store.ErrNotFound`. Sync and the migrations take the handle (`Store.DB()`) and do not go through the store. Tests: `internal/store/store_test.go`.
- **Open the database with `localdb.Open(path)`, never `sql.Open`.** It asks for foreign keys in the DSN, so every connection in the pool enforces them; a `PRAGMA` run on the pool reaches one connection.
- **Several processes share `vimango.db`** — the TUI, an `--editor` per pop-out, a `--render-html` per preview, `vimango-sync`, hybrid's GRDB connection. Writers take turns; mattn's built-in 5000 ms busy timeout is what makes that work.
- **Files that several vimango processes write** (`registers.json`, `preferences.json`, the image cache's index and images) go through `atomicfile.Write` (`internal/atomicfile`): a temporary file with a name of its own, renamed into place.
- `config.json`, `vimango.vim`, `spell/`, `registers.json`, `image_cache/` and the glamour style are found **relative to the working directory**. Nothing may change it; see "vim's ex commands".

### Text on screen

**The notice box is the screen's** (`notice.go`): the overlay on the right pane for `:help`, a sync report, a note's info, the list a context is picked from, and the output of a vim command that lists something. `Screen.ShowNotice(text, hang)` renders markdown into it and starts it at its top; `Screen.NoticeKey` is the keys that scroll it, the same in the organizer's notice modes and the editor's HELP mode. Whoever shows a notice says what the scrolling reports, on its own message line.

**Cut text to fit by screen columns, with `pane.ClipColumns`** (`internal/pane`), never by slicing bytes: a title or message can hold multi-byte and double-width characters, and escape sequences that take no room. Backspace on a command line is `dropLastRune`.

## The database

### Migrations

`vimango.db` is local to each machine — sync carries rows, not the file — so a change to how it is stored has to happen once per machine. **Such changes go in `internal/localdb.Migrate`.** Each step checks the database's actual state and does nothing if it is already there, so it runs quietly at every start of vimango (`InitDatabases`) and of `vimango-sync` (not `--probe`, which writes nothing); `--init` runs it on a new database and `--migrate` runs it by hand and reports. A step that cannot be done because another process is mid-transaction returns an error and leaves nothing half-done; startup ignores that and the next start retries. Tests: `internal/localdb/localdb_test.go`.

Steps so far:
- **Write-ahead logging.** Recorded in the file, so every program that opens it uses it. Readers see a snapshot and neither block nor wait for the writer. While the database is open there are two more files beside it, `vimango.db-wal` and `-shm`; recent commits live in the `-wal` file until a checkpoint, so **never back up or copy `vimango.db` with `cp`** — use `sqlite3 vimango.db ".backup copy.db"`.
- **The search index**, below.

### Search index

Full-text search is the FTS5 table `task_fts` inside `vimango.db`: one row per note, `rowid = task.id`, columns `title`, `note`, `tag` (the note's keywords, comma-joined), searched with `bm25(task_fts, 2.0, 1.0, 5.0)`. **Triggers keep it current** — on `task` (insert, delete, update of `title`/`note`/`tid`), on `task_keyword`, and on `keyword` (rename, delete). Definitions and the migration that builds the index: `internal/localdb/searchindex.go`.

- **No code writes the index.** The store's `SetNote` and `SetTitle`, the keyword commands and the sync engine just change rows; the index changes in the same transaction. Do not add index writes — fix the trigger. This holds for hybrid's GRDB writes and the `sqlite3` shell too, since triggers run in whichever process writes.
- A note is searchable as soon as it exists. Keywords hang off the server `tid` (`task_keyword.task_tid`), so tags appear once the note has synced.
- **The index is derived data.** `Migrate` compares the table and trigger definitions in `sqlite_master` with those in the code and rebuilds everything from the notes when they differ — so changing a trigger's SQL in `searchObjects` re-indexes every machine at its next start (about half a second for 5,900 notes), and a damaged index is repaired the same way. `INSERT INTO task_fts(task_fts) VALUES ('integrity-check')` verifies one.
- Anything that drops and recreates `task` loses the triggers on it; `Migrate` puts them back.

### Containers and UUIDs

Contexts, folders and keywords each have a `tid` (the PostgreSQL id, NULL until synced) and a `uuid` (generated locally, the identifier for local operations). Tasks point at containers by `context_uuid` / `folder_uuid`; `task_keyword` carries `keyword_uuid`. Sync sends both, and resolves a `tid` from the `uuid` for rows created before their container had one. The defaults, both titled "none": context `00000000-0000-0000-0000-000000000001`, folder `00000000-0000-0000-0000-000000000002`. A database with no server configured works, with one gap: keywords, since `task_keyword` hangs off the server `tid`. Schemas: `internal/localdb/create.go` (`Schema`, and `Create`, which `--init` and the tests make a database with), `schema/postgres.sql`, and `DATABASE_SCHEMA.md`.

## Sync

Startup does no network I/O, even with PostgreSQL configured: `InitDatabases` opens the Postgres handle lazily and does not ping, so the TUI, `--editor` and `--render-html` work offline. Reachability is checked in `(*App).Synchronize` (a 5 s ping), which reports "sync unavailable" when offline. `vimango-sync` pings at boot and fails fast, on purpose.

- **`connect_timeout` is mandatory in the DSN** (`buildPGConnString`, `cmd/vimango-sync/main.go`; 10 s). Without it a *blackholed* route (dropped VPN, captive portal) retries SYNs for 75 s measured on macOS — long enough to strand hybrid's quit path, which awaits an in-flight sync. It bounds the connect only; a server that accepts and then stalls needs a caller-side deadline.
- **Every local timestamp is SQLite's format, `yyyy-MM-dd HH:mm:ss` UTC** — `datetime('now')` in every writer and `to_char(added, 'YYYY-MM-DD HH24:MI:SS')` in the pull, because `ORDER BY added` and every `modified > watermark` compare the *text*. A bare timestamp column comes through database/sql as RFC 3339 and sorts wrongly. `normalizeAdded`, run by every sync, rewrites old values in place.

**`vimango-sync` has three modes; pick the cheapest that answers the question.**
- `--probe`: "how many changes would a sync apply?" One watermark read, one Postgres `SELECT` of `(table, tid, modified, deleted)` stamps since it, a point lookup per stamp. Prints one integer, writes to neither database, runs no migrations. Cheap enough to run unattended; hybrid probes on window activation.
- `--report-only`: the full engine, reporting without applying. For the *breakdown*, not a count — a dozen-plus round trips and full note bodies over the wire. Its only writes are client-database upkeep.
- no flag: sync and apply.

The server watermark is taken two minutes *before* each sync (see the comment above `run` in `internal/sync/sync.go`), so a bare `modified > watermark` count would include this client's own recent pushes. `Probe` compares each stamp with the `server_modified` the local row holds and skips matches; `fetchAllChanges` drops the same rows (`dropSeen`), so "Number of changes" in the sync log agrees with `Probe`. Tombstones count, except for rows this client does not have.

A task note's Taskwarrior uuid is `task.taskwarrior_uuid`: synced both ways, written once and never cleared, never read by vimango itself. Design: `docs/taskwarrior-link-column.md`. Known sync problems and their analysis: `SYNC_ISSUES.md`. Server backup: `BACKUP_RESTORE.md`.

## The editor

**The editor is the package `internal/editor`, and every file this section names is in it** unless it says otherwise. The package does not import the application: what it takes from outside is an `editor.Host`, an `editor.Layout`, and the packages beneath both (`vim`, `internal/pane`, `internal/store`, `internal/atomicfile`, `internal/frontmatter`, `terminal`). Its public face is small — `Open`, `Editor.ProcessKey` / `PasteText` / `MappingTimeout`, `Draw` / `DrawAfterKey` / `Cursor`, `ShowMessage` / `ShowAside`, what an editor says of itself (`ID`, `Title`, `Mode`, `Modified`, `Stale`), and the calls that set the one vim up at startup (`InitSpell`, `SourceVimrc`, `RestoreRegisters`, `SaveRegisters`, `MappingWait`). **Keep it that way: an editor that needs something of the app is given it, as a field of `Host` or `Layout`.**

- **What it draws is a copy of vim, made in one place**: `resyncFromVim` (`editor_process_key.go`) copies vim's text and cursor into the editor (`e.ss`, `e.fr`, `e.fc`) and reports whether the text changed since it last did (vim's change tick). Call it after anything that touches vim, and write to that copy nowhere else — a copy made by hand, or skipped because "this key cannot have changed anything", is what most of the editor's display bugs have been. Keys go through `followVim`, which calls it, and should not go round: it also captures vim's messages and brings vim back to the note's buffer. The one other maker of the copy is `copyFromVim`, the same copy without the report, for `Scroll` alone: libvim calls that in the middle of a key, and the question of whether the text changed is left for the `resyncFromVim` after the key. Tests: `resync_test.go`.
- **Modified is the buffer's text against the database's**, not whether vim has changed the buffer since the save (`isModified`, `editor_methods.go`). It is compared with `dbText`, the note as the editor last read or wrote it, once for each value of vim's change tick: `x` then `u` is a clean note, so is a word deleted and typed again, and undo past a save is a modified one, because the database holds the later text. The quit guard, what `:x` writes and the host's `modified` all rest on it. **`noteIs` is the one place `dbText` and `saveTick` are set**, after a note is read into the buffer and after the buffer is written; set either anywhere else and a note can be called clean that is not in the database. vim's own `'modified'` is not asked: it is about a file vim never writes. Tests: `TestUndoneChangeIsNotAModifiedNote`, `TestUndoPastASaveIsAModifiedNote` (`editor_save_test.go`).
- **A notice and HELP mode go together.** `showNotice` (`editor.go`) is the one place the editor puts up a notice: it sets HELP, whose keys scroll the box, and records that a notice is up (`Editor.notice`). Drawing the text ends it (`drawText`). `:` typed over a notice leaves it up while the command is typed; when that line ends in anything but another notice, `redrawAfter` has the note drawn. `Draw` in HELP mode shows the notice again, so a resize keeps it. Do not call `Notices.ShowNotice` directly, and do not set HELP anywhere else. Tests: `notice_test.go`, and `TestResizeKeepsTheNoticeUp` in the root package.
- **Its modes are its own**: `editor.Mode` (`mode.go`: `NORMAL`, `INSERT`, `EX_COMMAND`, `VISUAL`…, `SEARCH`, `SPELLING`, `HELP`). The organizer's `Mode` (`common.go`) is a different type, with the same names where they mean the same thing.
- **Its tests run in the project root**, not in the package's directory (`TestMain`, `helpers_test.go`), because the editor finds `vimango.vim` and `spell/` relative to the working directory. A test's editor is opened by a `testHost` (`editor_testhost_test.go`): a notice box that records what it is shown, and a count of the times the editor said it closed. Tests of the *app* with an editor in it — what a close comes to, the event loop, the screen's two layouts — are in the root package (`session_test.go`, `editor_only_test.go`, `event_loop_test.go`, `screen_test.go`), and read the editor only through its public face.

### Layout

**The editor draws by the layout it is given and asks nothing else** — not `Session.editorOnly`, not the screen's size. `editor.Layout` (`editor.go`) is the text's rectangle, whether there is a status bar and a top right corner, and two places for messages: `Message`, the editor's message line and command line (`ShowMessage`), and `Aside`, for what is not about the text under the cursor (`ShowAside`: "Saved note 5", how far a notice has scrolled). `Screen.editorLayout()` (`screen.go`, in the root package) is the one place that knows the two ways an editor is shown — the TUI's right pane, and the whole of a host's window, where `Aside` is the message line again — and `Screen.positionEditor` gives the result to the editor when it is opened and at every resize. `Editor.Cursor()` is where the terminal's cursor belongs; `App.returnCursor` asks it.

So a new difference between the TUI's editor and hybrid's is a field of the layout, set in `Screen.editorLayout()`; a check of the mode in the editor's files is the thing this replaced. Of the screen the editor is given only the notice box, as `editor.NoticeBox` (two methods), and it draws the whole of itself with `Editor.Draw()`, which is also what a resize and `:checkstale`'s reload call. **A resize clears the screen first, so `Draw` has to put back everything the editor has on it**: the text, the frame, the status bar, a notice if one is up, and what its mode keeps on the message line (`drawModeLine`: the command line being typed, "-- INSERT --", the spelling suggestions). It works the view out again first (`followCursor`): a pane of another width wraps the rows above the cursor onto other screen lines. Anything new that stays on screen for the length of a mode belongs there too. Tests: `editor_layout_test.go`, and `screen_test.go` in the root package for the two layouts the screen makes.

### Keys

**The editor intercepts no normal-mode keys: every key reaches vim.** Editing shortcuts are vim mappings in `vimango.vim`; anything wanting an argument or a name is an ex command. (`<Space>` is a real vim motion; a leader key would swallow it.)

**`<esc>` is a key like the rest.** `handleKey` sends it whatever the editor's mode — it ends `:help` and the spelling list as it ends an insert — and with `vim.SendKey`, so no mapping sees it; what vim did is then followed by `followVim`, as for any key. Do not give it a path of its own: vim can do more with an `<esc>` than end something (a key it was holding for a mapping goes in first; an insert with a count is carried out by it), and vimango_hybrid begins every command it injects with one. Tests: `esc_test.go`.

**After every key, vim is on the note's buffer.** A key can take it elsewhere — Ctrl-^ goes to the alternate buffer — and `ProcessKey` brings it straight back, as `forwardEx` does after an ex command. Otherwise the keys that follow edit a buffer nobody is looking at. Test: `TestKeyThatLeavesTheNoteComesBack`.

**The command line is vim's, and the editor follows it** (`syncCmdline`, after every key typed on one): which kind of line it is (`vim.CommandLineGetType()`), what is on it, where the cursor is. A `:` line is `EX_COMMAND`, whose Enter vimango takes for itself (`runExLine`); any other is `SEARCH`, whose keys all go to vim — `/` and `?`, and `=`, the line an expression is typed on. Do not track the line in Go: Ctrl-R puts a register on it, Ctrl-R Ctrl-W the word under the cursor, and **Ctrl-R = puts the whole line aside for an expression's and brings it back** — so the kind of line can change under a key, and `<esc>` can end the expression (or a Ctrl-R waiting for its register) and leave the line open. The same Ctrl-R works in INSERT (`<C-r>a`, `<C-r>=strftime('%F')<CR>`, in a mapping too), and `"=` in NORMAL asks for an expression the same way. Tests: `ctrlr_test.go`.

**Ctrl-C stops a command that does not end** — `:while 1 | endwhile` (through `:execute` or a function: typed, `:while` is refused), a mapping that types itself again (`nmap Q jkQ`), a function that never returns — as in vim. libvim reads no keys while it runs one and `MainLoop` is inside it, so the key reader takes the Ctrl-C itself and calls `vim.Interrupt` (`App.readerTakes`, `app.go`; see "Threading"). The call returns, keys still waiting in vim are dropped, Insert mode and a command line end, and the message line says `Interrupted` (`vim.CaptureMessages` adds it: vim says nothing for a `:while`). A Ctrl-C with nothing running is a key like any other. Keys typed meanwhile wait in `keyEvents`, which has room for 1,024: the reader blocks when it is full, and cannot see a Ctrl-C then. A loop in C that never looks at vim's `got_int` is not stopped; vim's own loops look. Tests: `TestCtrlCInterruptsACommandThatNeverEnds` (`event_loop_test.go`), `vim/interrupt_test.go`, and the fork's `apitest/interrupt.c`.

**Pasted text is not keys.** vimango turns on bracketed paste (`terminal.BracketedPasteOn`, just before `MainLoop`; off again in `quitApp`), so the terminal marks a paste and `terminal.ReadKey` returns the whole of it as one `KeyPaste` event, which the app's `MainLoop` gives to `Editor.PasteText` (`editor_paste.go`) or `Organizer.pasteText` (`organizer_process_key.go`, root package) instead of to the key path; what both make of the raw text is `pane.PastedText` and `pane.OnOneLine`. In the note it is typed into vim with `'paste'` set and mapping expansion off, so `'autoindent'`, `vimango.vim`'s `<CR>` mapping and abbreviations leave it alone, and control characters other than tab and line end are dropped first (an `<esc>` in a paste would end the insert and run the rest as commands). Where it goes is vim's own rule: at the cursor in INSERT; after the cursor in NORMAL (before it in the first column), dropping whatever was half-typed; over the selection in VISUAL; onto a command line as one line, never run. The organizer takes a paste into the title being edited or its command line, and drops it anywhere else. Anything new that accepts text must say what a paste does there. `:paste` / `:nopaste` remain for a terminal without bracketed paste. libvim's own `bracketed_paste` is never reached: the fork makes `<PasteStart>` and `<PasteEnd>` keys that do nothing (`d57f18c3c`). Tests: `paste_test.go`; in the root package `event_loop_test.go` and `organizer_paste_test.go`; `internal/pane/paste_test.go`; `terminal/terminal_test.go`.

**Motions by screen line are answered by the editor.** libvim draws nothing, so for `gj`/`gk`, `H`/`M`/`L` and `g0`/`g^`/`gm`/`g$` it calls back to ask where the screen lines are; the editor is that `vim.ScreenLayout` while it is open (`vim.SetScreenLayout` in `editor.Open`, nil in the app's `noteClosed`) and answers from the layout `drawRows` draws (`screen_motion.go`): `ScreenPosition` for `gj`/`gk`, `ScreenLine` for `H`/`M`/`L`, `ScreenColumn` for the start, middle and end of the screen line the cursor is on. Anything that changes how rows wrap must change all three. What is libvim's and not asked: `g^` goes on from the start to the first non-blank, and `g$` with a count is `ScreenPosition`, `count-1` lines down after `$`. `gm` is half the pane's width along the screen line, or its end when it is shorter. **With no layout set, `g0`/`g^`/`gm`/`g$` are libvim's own again** (`SetScreenLayout(nil)` takes the callback away), so on the organizer's title, one line, they are `0`, `^` and `$`; `gj`, `gk`, `H`, `M` and `L` do nothing there, and neither do the commands that scroll.

**The view is the editor's, and so are the commands that scroll** (`view.go`). The view is a row, the first one on screen (`firstVisibleRow`, vim's `'topline'`), and the rows after it for as far as the pane goes; a row is always shown from its first screen line, except one taller than the whole pane with the cursor in it (`topSkip`). `setView` is the one place it is set, and `lineOffset`, which everything drawn is placed by, is worked out there. Two things move it:

- **The cursor, when it leaves** (`followCursor`, after every key and in `Draw`): its row is shown whole; just above the view it becomes the top row and just below it the bottom one, so `j` and `k` scroll a row at a time; further off — about half a pane: `G`, a search, a mark — it goes to the middle. vim's rule (`update_topline`).
- **The commands that scroll**, none of which libvim carries out: it hands each to `ScreenLayout.Scroll`, in the middle of the key, and the editor moves the view and then the cursor (`vim.ScrollCursorTo`, which keeps the column vim wants, or goes to the first non-blank, as each command does in vim). `zt` `zz` `zb` (and `z<CR>` `z.` `z-`) put the cursor's row at the top, middle or bottom. Ctrl-Y and Ctrl-E move the view by rows and the cursor only if its row leaves. Ctrl-D and Ctrl-U move it half a pane of screen lines, or the count, which is kept, and the cursor as many screen lines, so it stays where it was in the pane. Ctrl-F, Ctrl-B, `<PageDown>` and `<PageUp>` move it a pane less two screen lines. The view moves by whole rows: as many as the distance covers, and one at least.

**The rules are vim 9.1's, not those of the vim 8.1 that libvim is**, and they are held to it by a table of what stock vim answered (`vimAnswers`, `view_vim_test.go`: 229 key sequences over rows that wrap in different ways). The two vims agree where rows take one screen line; where rows wrap, which in a note is everywhere, 9.1 counts screen lines and 8.1 lines, and 9.1 is the one there is to compare with. **`scripts/ask-vim.py uneven '20Gzz'` asks the vim on this machine** and prints a line of that table: use it before changing a rule, not memory of what vim does. One thing of 9.1's is left out on purpose: when half a page ends two or more screen lines into a wrapped row, vim moves the cursor a screen line further than the view (an off-by-one in its `scroll_with_sms`); here the cursor goes exactly as far as the view (`TestHalfPageKeepsTheCursorsPlace`). Tests: `view_test.go`, `view_vim_test.go`, `vim/scroll_test.go`.

**The screen is the editor's, and libvim asks.** That is the rule for anything in vim that depends on where text is drawn, decided 2026-11-15: a callback the editor answers from the layout it draws, as for `gj`, or one that hands the editor the whole command, as for Ctrl-D. Do not instead make libvim's window match the pane and trust vim's arithmetic. libvim believes in a window 80 columns by 40 lines, unwrapped, with `'tabstop'` 8 and its first line always on top (`winwidth(0)`, `winheight(0)`, `line('w0')`); the editor wraps at word boundaries, draws a tab as four columns, and scrolls by its own rule. Two layouts that have to agree to the column would be right most of the time and a few characters out the rest. The pane's size can still be given to libvim as numbers (`vimWindowSetWidth`, `vimWindowSetHeight`, both unused here) for what needs only a number — how wide a listing is laid out — but never as the layout. The wanted column handed back is vim's virtual column (`vim.VirtualColumns`, vim's `getvcol`), not a screen column: vim counts a tab by `'tabstop'`, the screen as four. Tests: `screen_motion_test.go`.

### vim mappings: `vimango.vim`

New key bindings belong in `vimango.vim` (project root, cwd-relative), sourced once at startup (`editor.SourceVimrc`, `vimrc.go`), so `--editor` processes get it too. A missing file is fine and errors in it are silenced. It holds ordinary vimscript:

- visual `-` / `#` toggle the selected lines as a bulleted or numbered markdown list, and switch one kind to the other (`VimangoToggleList`)
- Enter in INSERT mode on a list item (`- `, `* `, `+ `, `1. `, `1) `) starts the next item with the same marker, and on an empty item ends the list by clearing the line (`VimangoListEnter`, an `<expr>` mapping on `<CR>`). The marker is repeated as written, never counted up
- `let mapleader = " "`, at the top: `<Leader>` is replaced when a mapping is defined, so it has to come before every mapping that uses it. Leader mappings belong in normal and visual mode only; in INSERT every typed space would wait `'timeoutlen'`
- `<Leader>b` / `<Leader>i` / `<Leader>c` toggle bold / italic / code on the word under the cursor (normal) or a one-line selection (visual); a different one swaps the style (`VimangoDecorate`). Ctrl-B, Ctrl-E and Ctrl-I are vim's (page back, scroll a row, jump forward); `TestVimrcLeavesCtrlBEIToVim` keeps them so
- `set spelloptions=noallcaps,nomixedcase`, and anything else a vimrc would hold

`VimangoToggleList` and `VimangoDecorate` are string edits on the line (`getline`/`setline`), never `iw`/`gv`: after a decoration the text has moved, so vim's word motions and the `'<`/`'>` marks no longer describe it. Keep `editorNormalModeHelp` (the editor's `:help normal`) in step with the file.

Ctrl-I is Tab to a terminal, so Tab is vim's jump forward too. Ghostty — and so hybrid — sends Ctrl-I as the kitty-protocol `ESC [105;5u`; `terminal.ReadKey` decodes CSI-u Ctrl+letter back to its control byte and **drops any sequence it cannot use whole** (an unparsed tail such as `;5u` would reach vim as keys: "undo five times").

Mapping expansion is on only while a key the user typed in the note editor is forwarded (`vim.SendUserKey` around `pane.SendToVim` in `ProcessKey`). Every key vimango sends itself runs with it off, so a mapping cannot rewrite vimango's internal key sequences; the organizer never expands mappings.

**A left-hand side can be several keys** (`inoremap jk <Esc>`, `nnoremap ,d dd`). vim holds the first key until the next one shows whether the mapping is meant — so after a key, vim may have done nothing yet (`vim.InputPending()`), and the editor's state must come from vim, not from the key. vim gives up waiting after `'timeoutlen'` (1000 ms; `'notimeout'` waits for ever); libvim has no clock, so the app's `MainLoop` keeps the time (`editor.MappingWait`) and calls `Editor.MappingTimeout`, which is `vim.InputTimeout()`. In INSERT mode the held key is drawn at the cursor meanwhile, as vim draws it (`DrawAfterKey`). A key vimango sends itself (`<esc>`) is never held and takes a held key with it, in front. A mapping can end on a command line (`nnoremap ,s :%s/`): `forwardKey` asks vim which kind of line it is and what is on it. Tests: `mapping_test.go`, and `event_loop_test.go` in the root package for the loop's timer.

Limits, from libvim:
- `<esc>` never reaches a mapping: vimango sends it to vim itself
- vim 8.1 wants a modifier before the range: `:<C-u>keeppatterns '<,'>s/…`, not `:keeppatterns s/…` from VISUAL (E488)

### Ex commands

`runExLine` (`editor_process_key.go`) is the dispatcher. In order:

1. **Save, close and help, in any spelling vim accepts** (`:w` `:wri` `:up` `:wa` `:wq` `:x` `:exi` `:q` `:q!` `:qa` `:clo` `:h`). A note is a database row, so these write the row or close the editor (`closeNote`) and never a file. With a file name or a range they are refused. `:wq` is `:x`: it writes only a modified buffer.
2. **vimango's own commands, by exact name** — 13, registered in `editor_cmd_line.go`: write, read, checkstale, sync (which only says where sync is run from), syntax, number, paste, nopaste, spell, suggest, quit, exit, help.
3. **vim's**, forwarded to libvim or refused by `classifyEx`.

A range in front of a command (`'<,'>` is what vim writes when `:` is pressed in VISUAL) is skipped as vim skips it (`skipExRange`) to find the name, and the whole line, range included, is what reaches vim.

**Tab on a `:` line completes a file name, and only where one goes**: for a command whose arguments vim's table says are file names and which vimango does not refuse — `:read`, `:source` (`completeFileName`, `editor_process_key.go`). It goes through the matching files in name order, a `/` after a directory, hidden files only after a dot; any other key ends the round. Anywhere else Tab does nothing: there is no completion of command names. The line is typed into vim and read back with `syncCmdline`, like every change to it. Tests: `tab_test.go`.

### vim's ex commands

**Every other ex command is vim's, and is forwarded to libvim only if it is on the list in `ex_forward.go`** (`exAllowed`, 138 of vim's 558): `:s` `:g` `:v` `:d` `:y` `:pu` `:m` `:t` `:j` `:>` `:<` `:normal` `:sort` `:retab` `:u` `:red` `:noh` `:set` `:let` `:echo` `:call` `:execute` `:map` and its relatives, `:command`, `:autocmd`, `:marks`, `:source`, `:read`, a bare line number, and more — with ranges, modifiers such as `:silent`, and `|` chaining.

Why a list of what is allowed, not of what is forbidden: libvim is vim with no screen, one window and one buffer, and commands outside that do real harm, quietly. `:e x` and `:bn` leave the note's buffer, after which every command edits some other buffer; `:cd` moves the working directory vimango finds its files relative to; a command that waits for a screen or a key never returns from the C call. So a built-in command not on the list is refused with a reason. A name vim's table does not have (a user-defined `:Command`, a typo) is forwarded, and vim answers (`E492`).

- Names are resolved as vim resolves them: `vim.FindExCommand` is a port of vim's `find_command` over the command table in `vim/ex_cmds.h` (the fork's file, copied in by `build-libvim.sh`), embedded in the binary, so `:wri`, `:sav`, `:bn` mean here what they mean in vim.
- The check covers every command on a line joined with `|` and the command given to `:g`. **It does not look inside vimscript**: `:execute`, `:normal` and `:call` run what they are given. The list guards against habit, not intent.
- **Two things are enforced by libvim instead, so they hold by every route** — typed, in a mapping, a sourced script, `:execute`. `vim.InitializeVim` sets both:
  - **vim starts no program** (restricted mode, `restricted = 1`, what `vim -Z` sets): `:r !cmd`, `` :r `cmd` ``, `system()`, `:!cmd`, `:diffpatch` each get `E145`. Restricted mode also turns off vimscript's `delete()`, `rename()` and `mkdir()`.
  - **vim writes no file** (`vimSetFileWritesEnabled(0)`, the fork's): `:w name`, `:saveas`, `:redir > name`, `:mksession`, `writefile()`, a swap file after `:set swapfile` all fail, and an existing file is not touched. The exception is the spell word list: `zg`, `zw` and `:spellgood` write `'spellfile'` and its `.spl`.

  Tests: `TestVimStartsNoProgram`, `TestVimWritesNoFile` (`ex_forward_test.go`).
- After every forwarded command `forwardEx` verifies vim is still on the note's buffer, in normal mode, and in the same working directory, and puts back whatever is not.
- **To forward another command**, add it to `exAllowed` and run the tests. `TestForwardedCommandsDoNotHang` (`ex_hang_test.go`) runs every forwarded command, bare and with arguments, each in its own process with a time limit; `TestForwardedListIsSafeInLibvim` checks buffer, mode, directory and disk afterwards; `TestExPolicyCoversVimsTable` fails if a command that takes file names lands on the list. Keep `editorVimExHelp` (the tail of the editor's `:help`) in step.
- Known gaps, libvim's: `:s` with the `c` flag is refused, having no prompt; and **vimscript cannot ask the user** — `input()`, `inputsecret()`, `inputdialog()` return `''` and `inputlist()` 0, at once, because no key can reach vim while it is running a command (`TestForwardedCommandsDoNotHang` runs them).
- **`:let` with no assignment lists variables, and is forwarded only if the linked `libvim.a` can list** (`vim.ListsOutput()`, probed once in `InitializeVim`): a library from before the fork's `6fc194385` links without complaint and never comes back from that listing.

### vim's messages

libvim has no screen; what vim would write on its message line goes to a callback, which `vim.CaptureMessages` collects around a call. The editor shows them in two places:

- **Every key typed in the editor** (`ProcessKey` → `showKeyMessages`, `ex_forward.go`): errors (`E486: Pattern not found`), reports (`5 lines yanked`, what `u` did, `g Ctrl-G`'s word count) and warnings (`search hit BOTTOM, continuing at TOP`). When vim says nothing, which is most keys, the message line is left untouched, so `-- INSERT --` and vimango's own messages stay.
- **A forwarded ex command** (`forwardEx` → `showVimMessages`): the same, plus output. A report or two goes on the message line; a listing goes to the overlay `:help` uses. **What a command lists arrives as one message with line breaks in it** — `:registers`, `:history`, `:messages`, `:version`, `:map`, `:print`, `:let` — which libvim collects while the command runs (the fork's PATCHES.md, "What a command lists"); the few commands upstream converted (`:marks`, `:set`) send a message per item, some with a title. vim lays a listing out for its own `'columns'` (80), not for the pane. `:history` lists what was typed because `ExModeKeyHandler` gives each line to `histadd()`. Tests: `TestListingsAreShown`.

Two things to know:
- **An error is shown alone.** vim warns that a search is going round the end before it knows whether it will find anything, and then writes `E486: Pattern not found` over the warning; here both arrive, and `showKeyMessages` shows the error.
- vim says some things twice; `showKeyMessages` shows each line once.
- A typed `/pattern` is vim's, wrap warning included: libvim runs one search for it, from where it started (the fork's `6bc47d62d`; before that it ran two, and vimango dropped the warning for that key).

The organizer's title editing does not show vim's messages. Tests: `TestVimMessagesForKeysAreShown`, `TestVimReportsForKeysAreShown`, `TestTypedSearch`, `TestForwardedCommandReports` (`ex_forward_test.go`).

### Markdown highlighting

The editor highlights six things in the note, and nothing else: headings (`#` to `######`), `**bold**` / `__bold__`, `*italic*` / `_italic_`, `~~strike~~`, `` `inline code` `` and fenced code blocks (`markdown.go`, drawn by `drawRows` in `editor_rows.go`). The markers stay on screen; the highlighting only styles them. `:syntax off` and `:syntax on` turn it off and on, as in vim, and a bare `:syntax` toggles it; `:md` and `:markdown` are the same command. **Off, every row is drawn as body text** (`#c4c4c4`), never in no colour: the terminal's default foreground is a brighter white, and the whole note looked bold. The colours are constants in `markdown.go`: prose and italic `#c4c4c4` and headings and bold `#e8ecf1`, as `gruvbox_mod.xml` gave chroma before this replaced it, and code `#e04212`, the `webview.code_accent` of the rendered HTML, so the two agree. The constant does not follow `config.json`: a change of `code_accent` there needs one here.

- **A row is styled on its own** (`rowStyles`): every byte gets an `mdStyle`, a set of bits, and `styledRow` writes the row with an escape wherever the style changes, wrapped as `wrappedRow` wraps it and closed at the end of each screen line. Notes are soft-wrapped, so a paragraph is one row and the inline rules see all of it. **The one thing carried from row to row is the fence** (`mdBlock`): a redraw reads the rows above the window for fences only, then styles the visible rows. Nothing is cached; at the bottom of a 900-row note a redraw takes about 70 µs.
- **The rules are CommonMark's, simplified.** Code spans first: a run of n backticks to the next run of exactly n on the row, or literal; nothing inside one is styled. Then `*`, `_` and `~~` by the delimiter-run rules (left/right flanking, the rule of 3), so `snake_case` and `2 * 3` stay plain, and `***x***` is bold and italic. `~~` strikes what is between the tildes, not the tildes. A backslash escapes a marker. A heading is up to three spaces, one to six `#`, then a space or the end of the row.
- **Fences**: three or more backticks or tildes, **at any indentation** (a list item's fence is indented), with any info string (```` ```c++ ````) unless a backtick fence's has a backtick in it. Closed by a run of the same character at least as long with nothing after it. **An unclosed fence runs to the end of the note**, as in CommonMark and vim: typing an opener colours everything below until the closer is typed. All of a fence, its markers included, is code colour.
- **Frontmatter is drawn plain exactly when the preview shows it as a card of keys** (`frontmatter.Rows`, `internal/frontmatter`): `---` on the first row, to the next `---` row with a line break after it, holding `key: value` lines the preview can display. A block the preview declines — a `# comment` in it, an anchor, flow YAML — is markdown in both panes. Both use the one package, so they cannot drift apart; `TestRowsAgreesWithSplitAndParse` checks that `Rows` on the editor's rows gives what `Split` and `Parse` give on the note's text.
- Not highlighted: links, lists, quotes, HTML comments, setext headings (a row underlined with `===` or `---`).
- Tests: `markdown_test.go` (the rules, as `{B:**bold**}` markup), `editor_rows_test.go` (drawing: the text is unchanged, each screen line closes its style, `:syntax` leaves the margin alone); `BenchmarkRedrawAtBottomOfLongNote`.

### Search highlighting

The matches of the last search are highlighted, as with vim's `'hlsearch'` (`searchhl.go`; bindings in `vim/search.go`). **vim keeps all the state**: libvim says where the matches are (`Buffer.SearchMatches`, for the visible rows, on every redraw) and whether they should be showing (`vim.SearchHighlighting`, which is `v:hlsearch`) — on after `/` `?` `n` `N` `*` `#` and `:s`, hidden by `:noh` until the next of those, off with `:set nohlsearch`. No Go code tracks those commands.

- `main` (the app's) sets `'hlsearch'` before sourcing `vimango.vim`; `set nohlsearch` there turns it off.
- `:noh` and `n` change what is highlighted without changing the text, so the tick check cannot see them: `ProcessKey` compares `searchState()` with what `drawSearchMatches` last drew and asks for a redraw when they differ. The same comparison gives incremental highlighting — libvim searches as the pattern is typed.
- Matches are drawn with `highlightRowRange`, like the visual selection, so one that crosses a wrap or contains a tab lands correctly; a match is plain black-on-yellow.
- Tests: `searchhl_test.go`, `vim/search_test.go`.

### Spell checking

Spell checking is vim's, from libvim. libvim finds the misspellings; vimango only draws them and shows the suggestion list. Code: `spellcheck.go`; bindings: `vim/spell.go`, `spell_callback.go`.

- **Dictionary**: `spell/en.utf-8.spl` (+ `.sug`) in the working directory, copied from the fork by `scripts/build-libvim.sh`; `spell/` is gitignored. `editor.InitSpell()` (before `editor.SourceVimrc()`, so `vimango.vim` can override) prepends the vimango root to `'runtimepath'` and sets `'spelllang'`, `'spellfile'` (`spell/en.utf-8.add`, where `zg` writes) and `'spellsuggest'` (`best,9,timeout:1000`: one digit picks a suggestion, and the search is capped at about a second — it is exponential in the word's length). If the dictionary is missing, `:spell` says spell check is unavailable.
- **Acronyms and mixed case**: `vimango.vim` sets `'spelloptions'` to `noallcaps,nomixedcase`, values only the fork has: an unknown word with 2+ capitals and no lower-case letter (`NASA`, `MP3`), or with a capital after its first character and a lower-case letter (`iOS`, `NASA's`), is not an error. A word capitalized only at its start is still checked (`zg` for names); skipping those was considered and deferred — it would leave Title Case headings and misspelled names unchecked.
- **A "don't flag this kind of word" rule belongs in libvim's `spell_check()`**, so highlighting, `]s`/`[s` and `z=` agree. Do not filter spell errors on the Go side: `]s` would still land on the hidden word. There is no `:syntax` in libvim, so `@NoSpell` groups cannot do it either.
- **`:spell`** toggles `spellOn`; vim's `'spell'` follows it through `syncSpellOption()`, which runs before every use rather than once: `'spell'` is window-local and vim restores a buffer's old window options when it becomes current again.
- **Highlighting**: `drawSpellErrors` (from `drawText`) asks libvim for the errors in the visible rows on each redraw; nothing is cached. Bad words get a background, capitalisation/rare/regional ones an underline, and in INSERT mode the word at the cursor is skipped, as in vim. Only the current buffer can be checked.
- **Normal-mode commands** go to vim untouched: `]s` `[s` `zg` `zw` `zG` `zW` `zug` and `{n}z=`. `zg` and friends change what is misspelled without changing the buffer; `spellListKey` spots them from `Editor.spellKeys` and forces the redraw.
- **`z=` without a count** would block on vim's numbered prompt, so libvim calls a callback instead (`TakeSpellSuggestRequest`). `ProcessKey` then calls `spellSuggest`, which shows the list on the message line and enters `SPELLING` mode; a digit sends vim `{n}z=` (undo, `.` and `:spellrepall` work), any other key cancels. `:suggest` is the same thing by name.
- **Tests**: `spellcheck_test.go` and `vim/spell_test.go`. Both point `'spellfile'` at a temp file so `zg` never writes the real word list.

## The organizer

### Ex commands

34 commands in 9 categories, registered in `organizer_cmd_line.go`:
- **Navigation**: open, opencontext, openfolder, openkeyword
- **Entry Management**: new, write, refresh, set context, set folder, add keyword, e (the current note, or `:e <id>`), copy, deletekeywords, deletemarks
- **Search & Filter**: find, recent
- **Container Management**: contexts, newc, newf, folders, keywords
- **View Management**: sort, showall
- **Images**: convertgdrive, toggleimages (`:ti`), showimageinfo, refreshimageinfo, imagescale `[+|-|N]`, cachewidth, clearcache (`:clc`), imagereset (`:ir`)
- **Data Management**: sync
- **System**: quit
- **Help**: help

**A command is a `func(*Organizer, exArgs)`**, and `exArgs` is `{name, bang, arg}` from `parseEx`: `:set context my ctx` is `{"set context", false, "my ctx"}`, `:rii!` is `{"rii", true, ""}`. Commands never slice `o.command_line` themselves. A command that does not set a mode of its own leaves the command line. Tests: `organizer_ex_test.go`.

### Normal mode

The organizer keeps its own single-byte `normalCmds` table — no prefixes, no multi-key sequences, no leader. 12 commands in 3 categories, plus `:`:
- **Entry Actions**: m (mark), Ctrl-D (delete), Ctrl-A (star), Ctrl-X (archive), Ctrl-F (new folder), Ctrl-C (new context)
- **Navigation**: Space or PgDn / b or PgUp page the rendered note down / up by half a pane (as in hybrid), Ctrl-J / Ctrl-K one line, Home back to its top. The same keys scroll `:help` and other notices, in the organizer and the editor. Space and b are there because ssh and rex often do not carry Ctrl-J/K; they cost title editing vim's `l` and `b`. Shift-Space cannot be the way back as it is in hybrid: terminals send it as a bare space.
- **Information**: Ctrl-I (entry info)

Other keys go to vim, which edits the title of the current row.

### Help and the command registry

`pane.CommandRegistry` (`internal/pane/registry.go`) is the registry both panes use: a command is registered with its aliases, description, usage, category and examples, so help cannot drift from the commands. Normal-mode keys are shown by name (`\x04` as `Ctrl-D`).

- `:help` — all ex commands by category; `:help <command>` or `:help <category>` for one
- `:help normal` — the organizer's normal-mode commands; in the editor, the `vimango.vim` mappings
- `:help <key>` — organizer only (`:help Ctrl-D`)
- A mistyped command gets "did you mean" suggestions from both registries.

A new command needs its help metadata.

## Rendering and images

### Markdown preview

The organizer's preview renders markdown with glamour. The style file is named in `config.json` (`"glamour": {"style": "darkslz.json"}`) and found in the working directory; if it is missing `default.json` is used, and if neither exists vimango exits at startup with an error naming both.

### Terminal graphics

Images are shown with the kitty graphics protocol. **Detection reads environment variables only and never queries the terminal** (`IsTermKitty()`, `term_misc.go`: `TERM`, `TERM_PROGRAM`, `GHOSTTY_RESOURCES_DIR`):

- **kitty**: images, Unicode placeholders, and text sizing (OSC 66, kitty 0.40+) for scaled headings
- **Ghostty**: images and placeholders; no text sizing, headings fall back to ANSI styles
- **Rex** (sets `TERM=xterm-256color`; detected by `TERM_PROGRAM=rex`): treated like Ghostty
- anything else gets no images. `TERM=xterm-ghostty ./vimango` turns them on; `TERM=xterm-kitty` also turns on text sizing.

Text sizing is enabled only for actual kitty (`isActualKittyTerminal()`, `kitty_capabilities.go`), configured in the glamour style (`kitty_scale`, `kitty_numerator`, `kitty_denominator`, `kitty_valign`); `VIMANGO_DISABLE_KITTY_TEXT_SIZING` turns it off. Protocol commands: `kitty_placeholders.go`; drawing: `organizer_display.go`.

### Image cache

A note's images are `![text](gdrive:<file id>)`, fetched from Google Drive. Two caches:

- **Disk** (`./image_cache/`, `image_cache.go`): base64 files and an index, shared by the TUI and `--render-html`, persistent and terminal-agnostic. Evicted least-recently-used; a cache hit writes the index only when the last use on record is over an hour old (`accessGranularity`).
- **Terminal**: the kitty protocol keeps transmitted images in the terminal; reset at each launch, and by `:imagereset`.

### HEIC

HEIC/HEIF (the iPhone camera format) is decoded by libheif in the `heic_worker` binary, the only decoder.

- vimango never links libheif. `heic_cgo.go` uses go-libheif's client side, which runs `heic_worker` as a hashicorp/go-plugin RPC subprocess; only `cmd/heic_worker` imports the C binding. A libheif crash kills the worker, not vimango.
- The worker is started on the first HEIC image (`createHEICDecoder`, once per process), found next to the vimango executable or else in the current directory.
- **It is not tied to its parent's lifetime**: it outlives the process, holding ~136 MB, unless killed. `ShutdownHEICDecoder` (`heic.go`) must run before a normal exit, and `killHEICWorkerOnSignal` (`signals.go`; the TUI and `--render-html` install it) does the same on SIGTERM, SIGHUP and SIGINT before letting the signal kill the process. Hybrid stops a render it no longer wants with SIGTERM. Shutdown is one-shot: go-libheif v1.3.0's `DeInit` leaves its initialized flag set, so the worker cannot be restarted in the same process.
- Detection is by magic bytes (an ftyp box with a heic/heix/hevc/hevx/mif1/msf1 brand), not by extension, which Drive may not preserve. `decodeImageWithOrientation` (`term_misc.go`) checks for HEIC first; the decoded image is cached as PNG like any other, so later displays need no worker.
- Missing worker, wrong architecture or a corrupt file: `IsHEICAvailable()` is false or `Decode` errors, and that one image shows an error. Nothing else is affected.

## Open items

Listed 2026-11-10, the libvim item rewritten 2026-11-11, the structure item 2026-11-14 and 2026-11-15 (`g0`/`g$` and the scrolling commands, added 2026-11-15, were done that day, and what scrolling left is in the libvim item); nothing here is started unless it says so. File names in the editor's and libvim's items are in `internal/editor` unless they say otherwise.

- **To update another Mac, follow [docs/UPDATING_A_MAC.md](docs/UPDATING_A_MAC.md)**, and keep it current when a change adds a per-machine step. vimango and hybrid must be updated together, and `libvim.a` rebuilt.
- **Structure: the editor is a package; what follows it.** The move was done 2026-11-14 (HISTORY): the editor is `internal/editor` and imports nothing of the application. In the order I would do the rest:
  - **The TUI's weight** is Steve's call, not a refactor: about 2,000 of the organizer's lines show images in a terminal that is used for emergencies.
  - The editor's files keep their `editor_` prefix inside `internal/editor` (`editor_rows.go`), so that the move was a move and this file's names stayed true. Dropping it is a rename for a quiet day.
- **Image cache.** It holds two formats: entries `--render-html` wrote are JPEG data URIs without pixel dimensions, which the TUI cannot use — it downloads the image again and overwrites the entry as PNG. Letting the TUI decode those would end the re-download. Separately, each process keeps its own copy of the index and writes the whole of it, so the last writer wins: a long-running TUI drops the entries a `--render-html` added in the meantime (the image file stays, unindexed, and is downloaded again).
- **libvim (the fork): what is left**, as of 2026-11-11. Done that day, each with a HISTORY entry: multi-key mappings, listings, the typed search, Ctrl-R and `"=`, and the waiting readers that froze the editor. [docs/VIM_COMMANDS.md](docs/VIM_COMMANDS.md) is the list to test against — what works, what is blocked on purpose, what is still wrong — and should be kept in step with this item. Still open, roughly in the order worth doing:
  - **A script cannot ask the user.** `input()`, `inputsecret()`, `inputdialog()`, `inputlist()` return `''` or 0 at once (a floor: they used to hang). Real support needs a callback from libvim to the embedder, and a prompt in vimango to answer it with.
  - **`:s` with the `c` flag has no prompt.** libvim refuses it itself, with an error; vimango refuses it first. Same missing piece as above: a way to ask.
  - **A reload leaves the marks where they were** (the fork's `TODO.md`, entry 2; seen there, not tried here). `vimBufferSetLines()` does not adjust them, so `'a` after `:checkstale` has reloaded a shorter note can give `E19: Mark has invalid line number`. Its undo item was done 2026-11-16; the cursor vimango sets itself (`checkStale`).
  - **What scrolling left**, 2026-11-15. `z+` and `z^` (the page after and before, by the line under or over the window) go by libvim's own window, 40 lines with line 1 on top: `z+` puts line 41 at the top. The fix is the fork's: ask the layout for the window's last and first line, as `L` and `H` do. A count given to Ctrl-U on the first line, or to Ctrl-D on the last, is not kept: libvim refuses the key before the editor hears of it. `:set scroll=` and `'scrolloff'` are not consulted: the count of Ctrl-D is the editor's, and the cursor may go to the pane's first and last rows.
  - **Function keys never reach vim.** Not libvim: `termcodes` (`internal/pane/keys.go`) names only the arrows, Backspace, Home, Del, PgUp, PgDn, and every other reader code is dropped, so a mapping on `<F5>` cannot be triggered. `terminal.ReadKey` already decodes F1–F12.
  - **Listings are laid out for vim's `'columns'` (80)**, not the pane: `:registers` cuts lines at 74 characters. Setting `'columns'` from the pane's width is untried.
  - **Refused though libvim could now do them**: `:filter /pat/ {cmd}` (and its refusal gives the wrong reason, "one window"), `:highlight`, `:function` with no body (to list). Each needs adding to `exAllowed` / `exModifiers` and a run of the hang test.
  - **Keys upstream commented out**: Ctrl-\ Ctrl-N in INSERT inserts the two control characters as text; Ctrl-\ e on a command line types `e`. Ctrl-K inserts `^K` (digraphs are not compiled in).
  - **`feedkeys()` with the `x` flag does nothing** (a `TODO` in the fork's `getchar.c`).
  - **`` :runtime `cmd` `` segfaults** outside restricted mode; vimango runs restricted, where vim refuses the backtick first.
  - **Small**: nothing is drawn while Ctrl-R waits for its register (vim draws a `"`); `:execute 'sp'` is neither refused nor reported; `vimSetCursorAddCallback()` is dead wiring in the fork.
  - The fork's own list is "Known gaps" in its `CLAUDE.md`.
- **Editor.** The search match under the cursor is not marked differently from the others. The organizer's title editing shows none of vim's messages.
- **Move markdown→HTML into vimango_hybrid.** `--render-html` (`render_html.go`, `note_html.go`, and the `callout.go` / `frontmatter.go` / `wikilink.go` goldmark extensions; `internal/frontmatter`, the rule they share with the editor, would have to be kept in step on both sides — about 1,500 lines and the goldmark dependency) serves only hybrid: its preview pane and, since 2026-11-09, its PDF export, which pages this HTML with WebKit. The TUI renders with glamour and never used it. It lives here because Google Drive images — OAuth token, download, the disk cache, HEIC conversion — are Go-side. Decided 2026-11-08: not now. Suggested split when it is done: markdown→HTML, the three extensions and the stylesheet move to Swift (`swift-markdown`, or markdown-it in the WKWebView so the CSS carries over); images stay here behind a narrow interface (a command that returns a Drive image's cached file, or hybrid reading `./image_cache/`). Gains: no process spawn per preview, and preview changes stop needing a vimango rebuild.
- **vimango_hybrid: what the report could carry next.** Since 2026-11-16 the editor tells its host whether it has unsaved changes and whether its note went stale (`host_report.go`). Hybrid still asks with `:q` before a quit or a close, on purpose. Not reported, and each a pair to add if hybrid finds a use: the mode, the title, a failed `:w`.
