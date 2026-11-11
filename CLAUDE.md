# CLAUDE.md

Guidance for working in this repository: the rules, and how to do things.

**What changed, when and why is in [docs/HISTORY.md](docs/HISTORY.md).** When a change lands, add a dated entry there and leave here only the rule it results in. Open work is at the end, under "Open items".

## What this is

vimango is a note-taking application in Go. Notes and their titles are rows in a local SQLite database (`vimango.db`), synced to a PostgreSQL server shared with the other clients (vimango_hybrid on macOS, an iOS app). Editing is vim's, from libvim through cgo.

- **Two ways in.** The TUI: an organizer (note titles, on the left) and an editor (on the right) in one terminal. And the modes a host app drives, `--editor` and `--render-html`. **vimango_hybrid is the daily driver**; the TUI is kept deliberately simple, for emergencies and ssh. PDF, printing and a richer preview belong in hybrid.
- **One note at a time.** `Session.editor` is the open note's editor or nil; `:x`/`:q` close it and return to the organizer, and in `--editor` mode end the process.
- **Every note is markdown.** Fenced code blocks inside a note are highlighted as part of markdown; old notes in the folder named `code` are markdown like the rest.
- **Layout.** The application is `package main` in the project root, around a global `app`. Beneath it: `vim/` is the libvim binding, `internal/store` is the database as the application uses it, `internal/localdb` is how the file is stored (opening it, the schema, migrations, the search index), and `internal/sync` is the sync engine. `cmd/vimango-sync` and `cmd/heic_worker` are the other binaries.
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

- **If a header change in the fork does not seem to rebuild an object, use `--clean`.** Any object the fork adds to its build needs a line in the generated `### Dependencies:` section of `src/Makefile`, or header changes will not rebuild it (a stale `spellfile.o` once made `zg` segfault; `TestSpellZgWritesSpellfile` catches that).
- **The binding compiles against `vim/libvim_api.h`, vimango's own copy of the libvim declarations it uses** (about thirty functions, five types, a few constants) — not against the fork's headers, which the vimango build never sees. `TestHeaderMatchesLibvim` (`vim/header_test.go`) is what keeps the copy honest: it compiles `libvim_api.h` together with the fork's `libvim.h`, so a prototype that differs does not compile, and compares the size and layout of every type and the value of every constant. It needs the fork's checkout (`../libvim` or `$LIBVIM_DIR`) and is skipped without one; `build-libvim.sh` runs it after every build and fails if it does. To use something more of libvim, copy its declaration into `libvim_api.h` and add any new type or constant to the test's probe.

### The vim package

`vim/` is the whole binding: `vim.go`, `spell.go`, `search.go` and `messages.go` are the cgo, and the application calls them directly (`vim.SendKey` calls `vimKey`). A buffer is a `*vim.Buffer`. `vim/README.md` says how to add a binding.

- **libvim is one process-wide vim and is not thread-safe.** See "Threading".
- vim's modes have names: `vim.GetCurrentMode()` returns a `vim.Mode`, compared with `vim.ModeNormal`, `ModeVisual`, `ModeOpPending`, `ModeCmdline`, `ModeInsert`, `ModeNormalBusy` — never with `1`, `8` or `257`.

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

- Several tests start a process per case (`TestForwardedCommandsDoNotHang`, `TestKilledProcessTakesItsHEICWorkerAlong`); `-short` skips them.
- Spell tests skip when `spell/en.utf-8.spl` is absent.
- **Sync integration tests** (`internal/sync/sync_integration_test.go`) simulate several clients against a throwaway Postgres and are skipped unless `VIMANGO_TEST_PG` is a DSN (no `dbname`) for a server they may create databases on. Never point it at the real server. With Homebrew's `postgresql@17`:
  ```bash
  initdb -D /tmp/vtpg -U postgres --auth=trust
  pg_ctl -D /tmp/vtpg -o "-p 54329 -k '' -c listen_addresses=127.0.0.1 -c TimeZone=UTC" -l /tmp/vtpg.log start
  VIMANGO_TEST_PG='host=127.0.0.1 port=54329 user=postgres sslmode=disable' CGO_ENABLED=1 go test --tags=fts5 ./internal/sync/
  ```
  (`-k ''` because the socket path under a long `$TMPDIR` exceeds the 103-byte limit.)

## Runtime options

`help.go` holds the list (`validFlags`) and the `--help` text; an unknown flag is an error.

- `--editor`: boot straight into the editor; for a host app with its own list of notes. **There is no organizer in this mode: `app.Organizer` is nil**, and so is `app.RenderManager`. `InitApp` makes the organizer only for the TUI, and the editor opens its note from the store itself (`App.openEditor`). So nothing on the editor's path may reach for the organizer: what the editor shares with it is the screen's (the notice box, `notice.go`) or the app's (`openEditor`, `noteClosed`). `Session.editorOnly`, set once at startup, is what the layout asks (full width, no status bar, no top border) and why `:sync` is refused (the host owns sync). One note per process, chosen by `--open <id>`, or else the note modified last; there is no command to switch notes. The host drives the editor over the pty with ex commands (`:checkstale`, `:q`, `:x`). Tests run this mode as it is, with no organizer: the `editorOnly` and `editorOnlyApp` helpers (`session_test.go`, `editor_only_test.go`).
  **A process exit after `:q` or `:x` is the host's only "saved" signal**, so the editor closes only when the buffer's text is in the database, or on `:q!`. `:x` on a modified buffer writes it, and stays open with the buffer still modified if the write fails — a database error, or an `UPDATE` that matched no row because the note was deleted underneath (`closeNote`, `Editor.save`, `updateNote`; tests in `editor_save_test.go`).
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

- key reader → `keyEvents`
- SIGWINCH → `resizeCh` → `handleResize()` (`signals.go`), which re-reads the window size and redraws
- image loading for the preview (`prepareImages`, `organizer_render.go`: Drive download, disk read, decode — nothing else) → `RenderManager.resultCh` → `applyResult()`, which transmits the images, runs glamour and draws

New background work follows the same shape: do the slow part off the loop, hand back data, let the loop touch the UI. `applyResult` drops a result the pane cannot show (editor open, a notice up, another row selected); every way back to the preview calls `displayNote`, which renders afresh.

### Files and the database

- **The application reads and writes `vimango.db` through `internal/store`, and the store shows nothing.** Each method returns data or an error; the caller decides what the user is told. The organizer's side of that is `organizer_store.go`: small functions that turn the store's answers into rows and an error into a message on the status line. Do not put SQL in the application, and do not reach for the screen (or `app`) from the store — the editor and `--render-html` use it with no organizer. A row that is not there is `store.ErrNotFound`. Sync and the migrations take the handle (`Store.DB()`) and do not go through the store. Tests: `internal/store/store_test.go`.
- **Open the database with `localdb.Open(path)`, never `sql.Open`.** It asks for foreign keys in the DSN, so every connection in the pool enforces them; a `PRAGMA` run on the pool reaches one connection.
- **Several processes share `vimango.db`** — the TUI, an `--editor` per pop-out, a `--render-html` per preview, `vimango-sync`, hybrid's GRDB connection. Writers take turns; mattn's built-in 5000 ms busy timeout is what makes that work.
- **Files that several vimango processes write** (`registers.json`, `preferences.json`, the image cache's index and images) go through `writeFileAtomic` (`atomicfile.go`): a temporary file with a name of its own, renamed into place.
- `config.json`, `vimango.vim`, `spell/`, `registers.json`, `image_cache/` and the glamour style are found **relative to the working directory**. Nothing may change it; see "vim's ex commands".

### Text on screen

**The notice box is the screen's** (`notice.go`): the overlay on the right pane for `:help`, a sync report, a note's info, the list a context is picked from, and the output of a vim command that lists something. `Screen.showNotice(text, hang)` renders markdown into it and starts it at its top; `Screen.noticeKey` is the keys that scroll it, the same in the organizer's notice modes and the editor's HELP mode. Whoever shows a notice says what the scrolling reports, on its own message line.

**Cut text to fit by screen columns, with `clipColumns`** (`organizer_display.go`), never by slicing bytes: a title or message can hold multi-byte and double-width characters, and escape sequences that take no room. Backspace on a command line is `dropLastRune`.

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
- `fts5_vimango.db` is no longer read or written by anything.

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

### Keys

**The editor intercepts no normal-mode keys: every key reaches vim.** Editing shortcuts are vim mappings in `vimango.vim`; anything wanting an argument or a name is an ex command. (`<Space>` is a real vim motion; a leader key would swallow it.)

**After every key, vim is on the note's buffer.** A key can take it elsewhere — Ctrl-^ goes to the alternate buffer — and `editorProcessKey` brings it straight back, as `forwardEx` does after an ex command. Otherwise the keys that follow edit a buffer nobody is looking at. Test: `TestKeyThatLeavesTheNoteComesBack`.

### vim mappings: `vimango.vim`

New key bindings belong in `vimango.vim` (project root, cwd-relative), sourced once at startup (`sourceVimrc`, `vimrc.go`), so `--editor` processes get it too. A missing file is fine and errors in it are silenced. It holds ordinary vimscript:

- visual `-` / `#` toggle the selected lines as a bulleted or numbered markdown list, and switch one kind to the other (`VimangoToggleList`)
- Ctrl-B / Ctrl-I / Ctrl-E toggle bold / italic / code on the word under the cursor (normal) or a one-line selection (visual); a different key swaps the style (`VimangoDecorate`)
- `set spelloptions=noallcaps,nomixedcase`, and anything else a vimrc would hold

Both functions are string edits on the line (`getline`/`setline`), never `iw`/`gv`: after a decoration the text has moved, so vim's word motions and the `'<`/`'>` marks no longer describe it. Keep `editorNormalModeHelp` (the editor's `:help normal`) in step with the file.

Ctrl-I is Tab to a terminal, so Tab does italic. Ghostty — and so hybrid — sends Ctrl-I as the kitty-protocol `ESC [105;5u`; `terminal.ReadKey` decodes CSI-u Ctrl+letter back to its control byte and **drops any sequence it cannot use whole** (an unparsed tail such as `;5u` would reach vim as keys: "undo five times").

Mapping expansion is on only while a key the user typed in the note editor is forwarded (`vim.SendUserKey` around `sendToVim` in `editorProcessKey`). Every key vimango sends itself runs with it off, so a mapping cannot rewrite vimango's internal key sequences; the organizer never expands mappings. Limits, from libvim:
- **single-key left-hand sides only** — a two-key lhs (`jk`, `,b`) swallows its first key
- no `<C-r>` in an insert-mode rhs (it inserts `^R`); call a function instead
- `<esc>` never reaches a mapping: vimango sends it to vim itself
- vim 8.1 wants a modifier before the range: `:<C-u>keeppatterns '<,'>s/…`, not `:keeppatterns s/…` from VISUAL (E488)

### Ex commands

`runExLine` (`editor_process_key.go`) is the dispatcher. In order:

1. **Save, close and help, in any spelling vim accepts** (`:w` `:wri` `:up` `:wa` `:wq` `:x` `:exi` `:q` `:q!` `:qa` `:clo` `:h`). A note is a database row, so these write the row or close the editor (`closeNote`) and never a file. With a file name or a range they are refused. `:wq` is `:x`: it writes only a modified buffer.
2. **vimango's own commands, by exact name** — 13, registered in `editor_cmd_line.go`: write, read, checkstale, sync, syntax, number, paste, nopaste, spell, suggest, quit, exit, help.
3. **vim's**, forwarded to libvim or refused by `classifyEx`.

A range in front of a command (`'<,'>` is what vim writes when `:` is pressed in VISUAL) is skipped as vim skips it (`skipExRange`) to find the name, and the whole line, range included, is what reaches vim.

### vim's ex commands

**Every other ex command is vim's, and is forwarded to libvim only if it is on the list in `ex_forward.go`** (`exAllowed`, 138 of vim's 558): `:s` `:g` `:v` `:d` `:y` `:pu` `:m` `:t` `:j` `:>` `:<` `:normal` `:sort` `:retab` `:u` `:red` `:noh` `:set` `:let` `:echo` `:call` `:execute` `:map` and its relatives, `:command`, `:autocmd`, `:marks`, `:source`, `:read`, a bare line number, and more — with ranges, modifiers such as `:silent`, and `|` chaining.

Why a list of what is allowed, not of what is forbidden: libvim is vim with no screen, one window and one buffer, and commands outside that do real harm, quietly. `:e x` and `:bn` leave the note's buffer, after which every command edits some other buffer; `:cd` moves the working directory vimango finds its files relative to; bare `:let` never returns — it waits inside the C call for a key to page with. So a built-in command not on the list is refused with a reason. A name vim's table does not have (a user-defined `:Command`, a typo) is forwarded, and vim answers (`E492`).

- Names are resolved as vim resolves them: `vim.FindExCommand` is a port of vim's `find_command` over the command table in `vim/ex_cmds.h` (the fork's file, copied in by `build-libvim.sh`), embedded in the binary, so `:wri`, `:sav`, `:bn` mean here what they mean in vim.
- The check covers every command on a line joined with `|` and the command given to `:g`. **It does not look inside vimscript**: `:execute`, `:normal` and `:call` run what they are given. The list guards against habit, not intent.
- **Two things are enforced by libvim instead, so they hold by every route** — typed, in a mapping, a sourced script, `:execute`. `vim.InitializeVim` sets both:
  - **vim starts no program** (restricted mode, `restricted = 1`, what `vim -Z` sets): `:r !cmd`, `` :r `cmd` ``, `system()`, `:!cmd`, `:diffpatch` each get `E145`. Restricted mode also turns off vimscript's `delete()`, `rename()` and `mkdir()`.
  - **vim writes no file** (`vimSetFileWritesEnabled(0)`, the fork's): `:w name`, `:saveas`, `:redir > name`, `:mksession`, `writefile()`, a swap file after `:set swapfile` all fail, and an existing file is not touched. The exception is the spell word list: `zg`, `zw` and `:spellgood` write `'spellfile'` and its `.spl`.

  Tests: `TestVimStartsNoProgram`, `TestVimWritesNoFile` (`ex_forward_test.go`).
- After every forwarded command `forwardEx` verifies vim is still on the note's buffer, in normal mode, and in the same working directory, and puts back whatever is not.
- **To forward another command**, add it to `exAllowed` and run the tests. `TestForwardedCommandsDoNotHang` (`ex_hang_test.go`) runs every forwarded command, bare and with arguments, each in its own process with a time limit; `TestForwardedListIsSafeInLibvim` checks buffer, mode, directory and disk afterwards; `TestExPolicyCoversVimsTable` fails if a command that takes file names lands on the list. Keep `editorVimExHelp` (the tail of the editor's `:help`) in step.
- Known gaps, all libvim's: `:registers`, `:history`, `:messages`, `:version` and a bare `:map` produce no output (they do no harm); `:s` with the `c` flag is refused, having no prompt.

### vim's messages

libvim has no screen; what vim would write on its message line goes to a callback, which `vim.CaptureMessages` collects around a call. The editor shows them in two places:

- **Every key typed in the editor** (`processKey` → `showKeyMessages`, `ex_forward.go`): errors (`E486: Pattern not found`), reports (`5 lines yanked`, what `u` did, `g Ctrl-G`'s word count) and warnings (`search hit BOTTOM, continuing at TOP`). When vim says nothing, which is most keys, the message line is left untouched, so `-- INSERT --` and vimango's own messages stay.
- **A forwarded ex command** (`forwardEx` → `showVimMessages`): the same, plus output. A report or two goes on the message line; a listing (`:marks`, `:ls`) goes to the overlay `:help` uses.

Two things to know:
- The wrap warning is dropped for the Enter that completes a typed `/pattern` (`endsSearch`): libvim's is wrong there. `n`, `N`, `*` and `#` are right, and shown.
- vim says some things twice (a failed search); `showKeyMessages` shows each line once.

The organizer's title editing does not show vim's messages. Tests: `TestVimMessagesForKeysAreShown`, `TestVimReportsForKeysAreShown`, `TestForwardedCommandReports` (`ex_forward_test.go`).

### Markdown highlighting

The editor colours the note with chroma's markdown lexer (`highlight.go`), drawn by `drawRows` (`editor_rows.go`). **The note is highlighted a chunk at a time, and each chunk's result is cached by its text**: `markdownChunks` cuts the rows into the YAML frontmatter, each fenced code block, and each paragraph (up to and including its blank row; a multi-line `<!-- -->` comment stays whole; a run of more than 60 rows with no blank is cut). A redraw highlights the chunks in the window, and all but the one being edited come from the cache (`Editor.hlCache`, dropped when it passes 8 MB), so the cost of a keystroke does not depend on the note's length or the cursor's place in it.

- A chunk's fence and comment boundaries follow chroma's own rules (a fence is ```` ``` ```` plus an optional `\w+` at column 0, closed by a row that is exactly ```` ``` ````; no closer, no fence), so a chunk highlights alone as it does in the whole note. Each goes to the lexer with a line break in front, because chroma's emphasis rules want whitespace before the marker; the frontmatter goes in bare so that it is taken for frontmatter.
- **One intended difference from whole-note highlighting**: chroma lets an unmatched backtick, `~~` or `[` run on across blank lines to the next one, however far. A chunk ends them with the paragraph, as markdown does.
- Tests: `editor_rows_test.go` — `TestChunkedHighlightingLooksLikeTheWholeNote` compares the two methods cell by cell; `BenchmarkRedrawAtBottomOfLongNote` and `BenchmarkWholeNote` are the after and before.

### Search highlighting

The matches of the last search are highlighted, as with vim's `'hlsearch'` (`searchhl.go`; bindings in `vim/search.go`). **vim keeps all the state**: libvim says where the matches are (`Buffer.SearchMatches`, for the visible rows, on every redraw) and whether they should be showing (`vim.SearchHighlighting`, which is `v:hlsearch`) — on after `/` `?` `n` `N` `*` `#` and `:s`, hidden by `:noh` until the next of those, off with `:set nohlsearch`. No Go code tracks those commands.

- `main` sets `'hlsearch'` before sourcing `vimango.vim`; `set nohlsearch` there turns it off.
- `:noh` and `n` change what is highlighted without changing the text, so the tick check cannot see them: `editorProcessKey` compares `searchState()` with what `drawSearchMatches` last drew and asks for a redraw when they differ. The same comparison gives incremental highlighting — libvim searches as the pattern is typed.
- Matches are drawn with `highlightRowRange`, like the visual selection, so one that crosses a wrap or contains a tab lands correctly; a match is plain black-on-yellow.
- Tests: `searchhl_test.go`, `vim/search_test.go`.

### Spell checking

Spell checking is vim's, from libvim. libvim finds the misspellings; vimango only draws them and shows the suggestion list. Code: `spellcheck.go`; bindings: `vim/spell.go`, `spell_callback.go`.

- **Dictionary**: `spell/en.utf-8.spl` (+ `.sug`) in the working directory, copied from the fork by `scripts/build-libvim.sh`; `spell/` is gitignored. `initSpell()` (before `sourceVimrc()`, so `vimango.vim` can override) prepends the vimango root to `'runtimepath'` and sets `'spelllang'`, `'spellfile'` (`spell/en.utf-8.add`, where `zg` writes) and `'spellsuggest'` (`best,9,timeout:1000`: one digit picks a suggestion, and the search is capped at about a second — it is exponential in the word's length). If the dictionary is missing, `:spell` says spell check is unavailable.
- **Acronyms and mixed case**: `vimango.vim` sets `'spelloptions'` to `noallcaps,nomixedcase`, values only the fork has: an unknown word with 2+ capitals and no lower-case letter (`NASA`, `MP3`), or with a capital after its first character and a lower-case letter (`iOS`, `NASA's`), is not an error. A word capitalized only at its start is still checked (`zg` for names); skipping those was considered and deferred — it would leave Title Case headings and misspelled names unchecked.
- **A "don't flag this kind of word" rule belongs in libvim's `spell_check()`**, so highlighting, `]s`/`[s` and `z=` agree. Do not filter spell errors on the Go side: `]s` would still land on the hidden word. There is no `:syntax` in libvim, so `@NoSpell` groups cannot do it either.
- **`:spell`** toggles `spellOn`; vim's `'spell'` follows it through `syncSpellOption()`, which runs before every use rather than once: `'spell'` is window-local and vim restores a buffer's old window options when it becomes current again.
- **Highlighting**: `drawSpellErrors` (from `drawText`) asks libvim for the errors in the visible rows on each redraw; nothing is cached. Bad words get a background, capitalisation/rare/regional ones an underline, and in INSERT mode the word at the cursor is skipped, as in vim. Only the current buffer can be checked.
- **Normal-mode commands** go to vim untouched: `]s` `[s` `zg` `zw` `zG` `zW` `zug` and `{n}z=`. `zg` and friends change what is misspelled without changing the buffer; `spellListKey` spots them from `Editor.spellKeys` and forces the redraw.
- **`z=` without a count** would block on vim's numbered prompt, so libvim calls a callback instead (`TakeSpellSuggestRequest`). `editorProcessKey` then calls `spellSuggest`, which shows the list on the message line and enters `SPELLING` mode; a digit sends vim `{n}z=` (undo, `.` and `:spellrepall` work), any other key cancels. `:suggest` is the same thing by name.
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

`command_registry.go` is the registry both panes use: a command is registered with its aliases, description, usage, category and examples, so help cannot drift from the commands. Normal-mode keys are shown by name (`\x04` as `Ctrl-D`).

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

Listed 2026-11-10; nothing here is started unless it says so.

- **To update another Mac, follow [docs/UPDATING_A_MAC.md](docs/UPDATING_A_MAC.md)**, and keep it current when a change adds a per-machine step. vimango and hybrid must be updated together, and `libvim.a` rebuilt.
- **Run the sync integration tests once against `localdb.Open`.** Foreign keys became per-connection on 2026-11-10 with no Postgres on the machine to run them. They have always run on a single connection with enforcement on, so no change is expected.
- **Structure: what could still be done.** The split is made (2026-11-10): the store is `internal/store`, the HTML renderer builds no `App`, and `--editor` has no organizer. The three still share `package main` and the editor still reaches the global `app` for what is the app's (`Run`, `returnCursor`, `moveDividerPct`, `noteClosed`). Moving the editor and the renderer into packages of their own would make those boundaries the compiler's; nothing needs it yet.
- **Image cache.** It holds two formats: entries `--render-html` wrote are JPEG data URIs without pixel dimensions, which the TUI cannot use — it downloads the image again and overwrites the entry as PNG. Letting the TUI decode those would end the re-download. Separately, each process keeps its own copy of the index and writes the whole of it, so the last writer wins: a long-running TUI drops the entries a `--render-html` added in the meantime (the image file stays, unindexed, and is downloaded again).
- **libvim (the fork).** On the Enter that completes a typed `/pattern` the wrap warning is wrong, so vimango drops it there. `:registers`, `:history`, `:messages`, `:version` and a bare `:map` produce no output. A bare `:let` never returns (vimango refuses it). `` :runtime `cmd` `` segfaults (in restricted mode vim refuses the backtick first). `:s` with the `c` flag has no prompt. A multi-key mapping left-hand side loses its first key.
- **Editor.** The search match under the cursor is not marked differently from the others. chroma's markdown lexer does not take ```` ```c++ ```` or an indented fence as a fence. The organizer's title editing shows none of vim's messages.
- **Move markdown→HTML into vimango_hybrid.** `--render-html` (`render_html.go`, `note_html.go`, and the `callout.go` / `frontmatter.go` / `wikilink.go` goldmark extensions — about 1,500 lines and the goldmark dependency) serves only hybrid: its preview pane and, since 2026-11-09, its PDF export, which pages this HTML with WebKit. The TUI renders with glamour and never used it. It lives here because Google Drive images — OAuth token, download, the disk cache, HEIC conversion — are Go-side. Decided 2026-11-08: not now. Suggested split when it is done: markdown→HTML, the three extensions and the stylesheet move to Swift (`swift-markdown`, or markdown-it in the WKWebView so the CSS carries over); images stay here behind a narrow interface (a command that returns a Drive image's cached file, or hybrid reading `./image_cache/`). Gains: no process spawn per preview, and preview changes stop needing a vimango rebuild.
- **vimango_hybrid, an idea only.** Hybrid learns whether the editor has unsaved changes by probing with `:q`; the editor could put that in the terminal title instead.
- **Notes with an unmatched backtick, left as they are** (see HISTORY, 2026-11-10): 3285, 3469, 4414, 5062 are program output or code that quotes `` `like this' ``; 2792 and 3929 have a lone backtick line inside indented code; 4837 has fences indented inside a list; 5612 ends with a closing fence that has no opener; 5905 is read by a script and was not touched.
