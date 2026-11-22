# HISTORY.md

What changed in vimango, when and why: the dated record that used to be spread
through CLAUDE.md, moved here on 2026-11-10 so that CLAUDE.md holds only the
rules and how-tos that result. Oldest first; add new entries at the bottom.
An entry says what landed, the commits involved where they are known, what was
measured, and anything learned the hard way. A lesson that should guard future
work is *also* a rule in CLAUDE.md.

The CLAUDE.md that this was taken from is in git at `2e8d4fc`. vimango_hybrid
keeps the same kind of log in its own `HISTORY.md`.

## Before 2026-07

- **Visual-block `I` / `A` (libvim fork, 2026-01).** Upstream libvim reported
  one cursor per block line through `cursorAddCallback` and did a single plain
  insert, leaving replication to the embedder (Onivim 2). vimango never
  registered that callback, so the block insert was silently dropped. The fork
  makes them vim's `OP_INSERT` / `OP_APPEND` operators again.
- **The glamour fork through `go.work` (2026-03-27, `54ccf51`).** A local-path
  `replace` (`../glamour`) gave way to a gitignored per-machine `go.work`, so
  that no local path was committed.

## 2026-07

- **Offline startup (2026-07-06, `debda68`).** `InitDatabases` stopped pinging
  PostgreSQL; the handle is opened lazily and reachability is checked when a
  sync is asked for.
- **libvim became the only vim engine (2026-07-07, `f537fe7`).** The pure-Go
  `govim` engine, the adapter layer that switched between the two, and the
  Windows and `CGO_ENABLED=0` stubs were removed. From then on cgo is required
  and Windows builds are unsupported.
- **One SQLite driver (2026-07-12).** `mattn/go-sqlite3` everywhere; the
  modernc.org/sqlite pure-Go driver and the `--go-sqlite` / `--cgo-sqlite`
  selection layer went. mattn's built-in 5000 ms busy timeout matters because
  several processes share `vimango.db`. This made the `fts5` build tag
  mandatory, and the `fts5_guard.go` files were added to turn a tag-less build
  into a compile error instead of a runtime "no such module: fts5".

## 2026-08

- **`vimango-sync --probe`, and a bounded connect (2026-08-02, `1ad16fe`).**
  The probe answers "how many changes would a sync apply?" without running
  one. `connect_timeout` went into the Postgres DSN at the same time: without
  it lib/pq falls back to OS TCP behavior — wifi-off, DNS failure and
  connection-refused fail fast, but a blackholed route retried SYNs for 75 s,
  measured on macOS. That stranded vimango_hybrid's quit path, which awaits an
  in-flight sync unbounded. Set to 10 s. Later `fetchAllChanges` gained
  `dropSeen`, so that the count a sync reports agrees with the probe's.

## 2026-09

- **No leader key (2026-09-13).** The `<leader>` prefix matcher and the
  `<C-w>>` / `<C-w><` pair were removed. `<Space>` is a real vim motion (`l`):
  the leader swallowed it, and swallowed the following key too when the pair
  did not match a registered command.

## 2026-10

- **libvim on GCC 14+ (2026-10-10).** Verified on GCC 16.2, Arch:
  `./configure`'s probe programs need `-Wno-error=implicit-int
  -Wno-error=implicit-function-declaration`; with plain `CFLAGS=-fPIC`
  configure stops at `checking for tgetent()... NOT FOUND!` even with ncurses
  installed. Older checkouts also failed to compile `ex_cmds.c` under the
  Makefile's `-Werror` and raced under `make -j`. The recipes are in the
  fork's CLAUDE.md now.
- **Taskwarrior link column (2026-10-16).** A task note's Taskwarrior uuid is
  `task.taskwarrior_uuid`, synced in both directions by `internal/sync`, and
  no longer in the note's text. Written once and never cleared: push and pull
  both COALESCE it, and every sync starts with a backfill. The design and how
  it rolled out are in `docs/taskwarrior-link-column.md`. The Linux machines
  never got that rebuild.
- **Local timestamps are SQLite's format (2026-10-17).** Until then the pull
  selected a bare `added`, which database/sql turns into RFC 3339 (`…T…Z`,
  variable fraction); the space-versus-`T` at position 11 sorted every pulled
  note after every local note from the same UTC day. The pull now uses
  `to_char`, and `normalizeAdded`, run by every sync, rewrites the old values
  in place — `modified` untouched, so nothing pushes — and skips anything
  `strftime` cannot parse, since `added` is NOT NULL and a failed UPDATE would
  stop every sync.

## 2026-11-07

- **`:open <id>` removed from the editor.** The host once injected it to
  switch notes in a warm editor; every host editor became a one-session cold
  spawn chosen by `--open <id>`, so there is no command to switch notes.

## 2026-11-08

A day of removing what the TUI no longer needs, now that vimango_hybrid is the
daily driver.

- **One note at a time.** The TUI once let several notes be open side by side
  (Ctrl-H / Ctrl-L moved between them and back to the organizer). Removed;
  `Session.editor` is the open note's editor or nil. With it went the editor's
  `normalCmds`, `NormalModeKeyHandler` and its normal-command registry: the
  editor intercepts no normal-mode keys.
- **The webview window.** Organizer `:webview` / `:closewebview`, Ctrl-W /
  Ctrl-Q, the editor's `:webview`, the `webview_worker` binary,
  `third_party/webview_go` and the GTK/webkit2gtk build dependency on Linux
  all went. `RenderNoteAsHTML` had rendered two styles — an `<h1>` and a
  centered column for the webview, neither for a host pane — and kept the
  second. `config.json`'s `webview.accent` / `webview.code_accent` keep their
  names and still style `--render-html`.
- **The editor's in-terminal `:preview`**, and the never-entered VIEW_LOG mode
  that shared its scrolling; `:vertical resize` in both panes. The divider
  still follows terminal resizes (`moveDividerPct`) and `preferences.json`'s
  `edPct`.
- **Code notes.** Notes in the folder named `code` used to be treated as
  source in the language their context named (`Languages`): chroma-highlighted
  in the editor and organizer, mirrored to a hard-coded file on `:w`
  (`updateCodeFile`), with `:compile` / `:run`, `:style` (the chroma theme)
  and `:template` (`go.template`). All removed; every note is markdown.
- **PDF and printing.** Editor `:pdf`, `:pdf-goldmark`, `:print`, `:ha`,
  `:save`; organizer `:print`, `:ha`, `:printlist`, `:save`; and the
  `mdtopdf`, `goldmark-pdf` and `gofpdf` dependencies. They belong in hybrid.
- **hunspell.** Spell checking became vim's own, restored in the libvim fork
  (fork `b00827d76`), and the `spell` build tag went. The suggestion search is
  exponential in the word's length; before the fork's cap, `z=` on a word like
  `llllllllllll` froze the editor for minutes.
- **The Python HEIC decoder.** `heic_pillow.go`, `heic_convert.py` and a
  `.venv` with Pillow and pillow-heif: only a `CGO_ENABLED=0` build compiled
  it. An old `.venv` in the vimango directory can be deleted.
- **vim mappings work.** Until then `:map` did nothing: upstream libvim's
  `vimInit` raised `no_mapping` because Onivim 2 owned keybindings itself, so
  mappings were stored (`maparg()` read them back) but never expanded. The
  fork's `vimSetMappingsEnabled()` lifts that (fork `ed0f2e241`). Ctrl-B / E /
  I, which had been Go intercepts with `:bold` / `:italic` / `:code` ex forms,
  moved into `vimango.vim` and the ex commands were dropped.
- **Ctrl-I under the kitty keyboard protocol (`3619010`).** Ghostty sends
  Ctrl-I as `ESC [105;5u`. Before `terminal.ReadKey` decoded CSI-u keys, the
  unparsed tail `;5u` reached vim as keys: "undo five times". It now drops any
  sequence it cannot use whole.
- **The glamour fork is a remote `replace`.** A committed `replace` pinned to
  a commit of slzatz/glamour superseded the `go.work`, so new machines no
  longer clone the fork.
- **`scripts/build-libvim.sh` builds the fork.** Until then it cloned upstream
  onivim/libvim and never pulled an existing checkout; a machine set up from
  the old INSTALL.md / MACOS_SETUP.md may have an upstream or stale
  `../libvim`, which the script now refuses.
- **Decided, not done:** moving markdown→HTML out of vimango into hybrid (see
  CLAUDE.md, "Open items").

## 2026-11-09

- **The fork's branch is `main`.** It was `master`; the build script moves an
  old `master` checkout across when that is a fast-forward. `spelling_restore`
  was merged into it the same day; a `../libvim` still on that branch must
  `git fetch && git switch main` first.
- **`'spelloptions'`** in the fork (`e0553a273` `noallcaps`, `bca64b35e`
  `nomixedcase`): acronyms and mixed-case words are not spelling errors.
  Skipping words capitalized only at their start, mid-sentence, was considered
  and deferred: it would leave Title Case headings and misspelled names
  unchecked.
- **A stale `spellfile.o`.** Until fork `532f73b8d` the fork's `src/Makefile`
  had no header dependencies for `spell.o` and `spellfile.o`, so an
  incremental build after the `structs.h` change in `e0553a273` kept a
  `spellfile.o` compiled against the old `synblock_T`. It linked and spell
  checking mostly worked, but `zg` segfaulted in `spell_add_word`. Pulling the
  fix did not repair such a checkout — `structs.h` was no newer than the stale
  object — so any machine whose `libvim.a` was built between those two commits
  needed `./scripts/build-libvim.sh --clean` once.
- **Rex is detected** by `TERM_PROGRAM=rex`; before that it needed
  `TERM=xterm-ghostty ./vimango`.
- **`--render-html --title`** (vimango `8205c8e`), and print styles, for
  vimango_hybrid's `:export` to PDF, which pages vimango's HTML with WebKit.

## 2026-11-10

The first part of the day's work was merged into `main` at `0556725`.

- **The `vim` package is the whole binding.** It had been three layers — `vim`
  forwarded to an `Engine` interface (`vim/interfaces`), implemented by a
  wrapper that called a separate `vim/cvim` package — left over from switching
  between libvim and the pure-Go engine.
- **Write-ahead logging**, as the first step in the new
  `internal/localdb.Migrate`. In the default rollback-journal mode a reader
  blocked the writer's commit and a committing writer blocked readers, so a
  sync and a preview render waited on each other.
- **The search index moved into `vimango.db`.** Until then it was the table
  `fts` in a second file, `fts5_vimango.db`, keyed by the server `tid`, and
  each writer updated it by hand in a separate, non-atomic write. Hybrid never
  wrote it (so its title edits were unsearchable), a note had no row until its
  first sync, and the two files drifted: on this machine 13 indexed titles and
  15 indexed bodies no longer matched their notes, and 74 rows belonged to
  notes that were gone. `errSearchIndex`, sync's FTS statements,
  `FTS_CLEANUP.md` and `vimango-sync --fts-db` (still accepted, ignored) went
  with it. Search results on the migrated copy matched the old index's for
  fifteen test terms, plus the notes the old index had gone stale on.
- **Markdown highlighting by chunk.** Every redraw had run the lexer from the
  first line of the note to the bottom of the window: 45 ms a keystroke at the
  end of an 860-line note, measured in a pty; 2.6 ms afterwards. Against the
  real database, 5,552 of 5,574 notes looked identical under the two methods;
  the other 22 each had a stray backtick, which chroma lets run on across
  blank lines. A marker at the very start of a note (`**Title**` on row 1) is
  also highlighted now, which the whole-note pass missed.
- **Stray backticks fixed in 24 notes**; the originals are in
  `backtick-fix-originals.json.bak`. Nine were left as they are (listed in
  CLAUDE.md's open items).
- **Search highlighting.** libvim had to be fixed first: its
  `vimSearchGetHighlights` skipped a match at the first column of the first
  line asked for and any match starting within two columns of the end of the
  previous one — the middle `foo` of `foo foo foo` (fork `a036ecafe`).
- **vim's messages are shown.** None had been, and nothing had decided to hide
  them: vimango had never registered the message callback, and libvim itself
  sent only errors and a few converted commands, dropping everything that went
  through vim's `msg()` (fork `2da770b06`, `77fc72a6d`).
- **vim's ex commands are forwarded**, from a list of what is safe
  (`exAllowed`). Measured before deciding on a list of the allowed rather than
  of the forbidden: `:w name` wrote the file; `:e x` and `:bn` left the note's
  buffer; `:cd` moved the process's working directory; bare `:let` never
  returned from libvim. `TestForwardedCommandsDoNotHang` is how the `:let`
  hang was found.
- **vim starts no program** (`8d1e246`): libvim runs in restricted mode. Until
  then only `:!` and its relatives were refused, by name, and the rest ran
  (`:r !touch x` made the file).
- **`:x` stays open when the write fails.** It had reported a failed write and
  quit anyway, and a process exit is the host's only "saved" signal.
- **Only `MainLoop` touches the UI and libvim.** The SIGWINCH goroutine had
  redrawn the screen itself (reaching libvim through brace matching and spell
  errors while a key might be in progress), and the render goroutines
  transmitted images, ran glamour and replaced `o.note` — which also blanked
  the editor pane when a note's images finished loading after it had been
  opened with `:e`.
- **Deleted:** the last Windows-only files (resize polling, raw mode, ANSI
  colors, the HEIC stub); the one-shot tools under `cmd/` (`migrate_local`,
  `fix_uuid_sync`, and others) and `postgres_migrate_uuid.sql`. Every
  machine's database and the PostgreSQL server were migrated to UUID
  containers long ago.
- **`docs/UPDATING_A_MAC.md`** written, for bringing another Mac to this
  state. The Linux machines are being retired.

Later the same day, after a review of the open items in all three
repositories:

- **Seven small fixes** (`022008d`):
  - Organizer ex commands take parsed arguments (`exArgs`). They had been
    handed the position of the last space in the line and sliced the line
    themselves, so `:e 123` opened note 1 (the position, taken for an id), and
    a name with a space in it worked for `:open` but not for `:set context`.
    A bare `:sort` had left the organizer in command-line mode.
    `setContext` / `setContextNew` and the folder pair became one function
    each.
  - Foreign keys are enforced on every pooled connection (`localdb.Open`).
    vimango and vimango-sync had run `PRAGMA foreign_keys=ON` once on the
    pool, which reached one connection; a statement that landed on another —
    a query made while a result set was still being read, anything from a
    second goroutine — was not checked.
  - A killed vimango takes its `heic_worker` along. Hybrid's SIGTERM to a
    `--render-html` it no longer wanted ran no defers and left the worker
    behind with its 136 MB.
  - `registers.json` was written in place, and `preferences.json` and the
    image cache through one fixed `.tmp` name that two processes could both be
    writing. All go through `writeFileAtomic` now.
  - The image cache's index was rewritten on every cache hit.
  - Status bars, messages and the info pane were cut by byte length, which
    could split a multi-byte character; Backspace on a command line removed a
    byte.
  - vim's modes were bare numbers (`1`, `8`, `257`).
- **vim writes no file** (`1fbaa0b`, fork `31301666c`). The ex list refused
  `:w name` when typed and did not look inside vimscript, where
  `:execute 'w name'`, `:saveas`, `:redir > name`, `:mksession` and
  `writefile()` all wrote. The fork's `vimSetFileWritesEnabled(0)` refuses at
  the point where vim opens a file. Found on the way: `:execute 'diffpatch f'`
  ran the `patch` program in restricted mode; `call_shell()` checks now.
- **vimango_ghostty is retired**; the references to it were removed (`2e8d4fc`
  here, and in vimango_hybrid).
- **This file.** CLAUDE.md had grown to 70 KB, loaded every session and mostly
  history; it was rewritten as rules and how-tos, and the history moved here.
- **Dead weight removed.**
  - `MigrateToUUID()`: about 160 lines that ran at every start of the TUI and
    `--editor` to move a pre-UUID database to UUID containers, and returned
    at once on every database in use.
  - `extension/` and `extension/ast/`: goldmark extensions for `==highlight==`
    and `~~strikethrough~~` that nothing imported.
  - Twelve struct fields in `internal/sync` that were never read or written.
  - The vendored libvim headers: `vim/src/`, `vim/proto/` and `vim/auto/`,
    109 files and about 1 MB, copied from the fork because `libvim.h`
    includes `vim.h`, which includes everything — and among them four `.c`
    files and twenty-four headers that not even that reached. The binding
    uses about thirty functions, five types and a few constants, now declared
    in `vim/libvim_api.h`. The vendored copy was never checked against the
    library either; the new one is, by `TestHeaderMatchesLibvim`, which
    `scripts/build-libvim.sh` runs after every build. `vim/ex_cmds.h` (the
    embedded command table) is the one file kept from the old tree, and the
    script now copies it from the fork.
  - Found by `deadcode` and `staticcheck -checks U1000`, which report nothing
    more in any package; `go mod tidy` had nothing to drop.
  - Left in on purpose: running with no PostgreSQL configured. It is a few
    lines in `InitDatabases` and `Synchronize`, and a state the installation
    guides and vimango_hybrid's README describe as supported (sync reports
    itself unavailable and everything else works; keywords are the gap).
- **The structural split, begun: `internal/store`.** The database layer was
  `dbfunc.go`, thirty methods on a `Database` type in `package main`, fifteen
  of which reported an error by calling `app.Organizer.ShowMessage` and then
  returning an empty answer, so nothing could use the database without an
  organizer and a terminal, and a caller could not tell "no text" from "could
  not read". It is now `internal/store`: every method returns data or an
  error and shows nothing, with types of its own (`Entry`, `Info`,
  `Container`), and `organizer_store.go` is where the organizer turns its
  answers into rows and messages. The schema and database creation moved from
  `init.go` to `internal/localdb` (`Schema`, `Create`). Done first, as
  warm-ups (`ca88e02`): one `setRows` for the eight places that changed the
  organizer's list, and a flat `helpFor` in place of a six-deep `help()`.
  Four near-identical "pick a container" functions became one.

  Bugs it brought out, each now with a test:
  - `:w` in the organizer worked on a copy of each row, so a new row kept
    id -1 in the list after its note was written: `:e` asked for the title to
    be saved first, and the next change to the title was written as a
    *second* note.
  - `:checkstale` could blank a clean buffer. A read that failed came back as
    empty text, which is "the row changed", and a clean buffer is reloaded
    with what was read. It now says it could not check and touches nothing.
  - A new note's context and folder were worked out in two places; one
    function now.
- **The structural split, second layer: the HTML renderer builds no `App`.**
  `--render-html` used to call `CreateApp()` -- an organizer, a screen, the
  terminal probe for kitty -- and `InitDatabases`, which also opened the
  Postgres handle, all to get at one note, two colours from `config.json` and
  the Drive sign-in; the renderer then read those back through the global
  `app`, with `if app == nil` guards for the tests. Now `renderNote` opens the
  store (`openStore`, shared with `InitDatabases`) and builds an
  `htmlRenderer`, a value that carries the colours, the flags, the
  `[[tid:N]]` title lookup, the Drive downloader (`driveImages`) and the
  image cache. The global stays nil in this mode. `loadConfig` and
  `readPreferences` are plain functions for the same reason.

  Checked against the previous build: 66 real notes -- with Drive images,
  tid links, frontmatter and callouts among them -- 15 more with
  `--wiki-links`, and five colour settings, all byte-identical. The colour
  comparison was added after a new unit test caught the refactor resolving
  the colours twice, which would have turned `"none"` back into the default
  blue.
- **The structural split, third layer: `--editor` has no organizer.** The
  mode vimango_hybrid edits in used to build the whole TUI behind the editor:
  an organizer, a query for its first list of notes, and a vim buffer of
  their titles. The editor needed it, because opening a note was an
  organizer method (`openInEditor`) and the box `:help` is shown in belonged
  to the organizer. Checks of `editorOnly` kept that hidden organizer from
  drawing, taking focus or being returned to.

  Now `InitApp` makes the organizer (and the render manager) only for the
  TUI, and in `--editor` mode `app.Organizer` is nil. What the editor shared
  with it moved to where both can reach it: `App.openEditor` reads the note
  from the store and opens it (the organizer calls it for its current row);
  `App.noteClosed` is what follows a close; the notice box is the screen's
  (`notice.go`). The `editorOnly` checks that remain are the mode's decision
  itself, the refusal of `:sync`, and five about layout.

  Checked by replaying scripted sessions in a pty against the previous build,
  each in its own copy of a small workspace, comparing every byte written to
  the terminal, the exit status and what was left on disk: a 45-step TUI
  session at two terminal sizes and nine `--editor` sessions (searching, substituting, spelling,
  listings, resizes, refused commands, `:w`, `:x`, `:q`, `:q!`, an empty note,
  a note with wide characters, no `--open`). All identical but for three
  things, each meant:
  - `:ls` lists one buffer fewer: there is no organizer's buffer of titles.
  - Scrolling `:help` in the editor said "Line 13 of 0", counting the lines of
    a preview that is not there; it counts the help's.
  - A notice opened at whatever line the *last* thing shown there had been
    scrolled to, and was reset only after being drawn. In the editor a second
    `:help` started part-way down. In the TUI, `:help find` after paging down
    a long note asked for lines 30 to 6 of its text and crashed (`slice bounds
    out of range`), in raw mode. A notice now starts at its top.

  Also: a note that cannot be read is no longer opened as an empty buffer;
  and with no `--open`, `--editor` opens the note modified last (it was the
  first row of the organizer's default list; hybrid always passes `--open`).
  The tests that exercised `--editor` did so with an organizer present; they
  now run without one.
- **Ctrl-^ no longer takes the editor off its note.** Found while comparing
  sessions for the change above: Ctrl-^ switches vim to its alternate buffer,
  which in the TUI is the organizer's list of titles and in `--editor` mode
  was the same hidden list (now vim's own empty buffer). The screen went on
  showing the note, and whatever was typed next went into the other buffer --
  in the TUI, into the titles -- until the next forwarded ex command noticed
  and went back. The editor now checks after every key.

## 2026-11-11

- **Mappings can have more than one key on the left** (fork `d93698bc8`).
  `inoremap jk <Esc>` and `nnoremap ,d dd` did not work: vim holds the first
  key of such a mapping in its typeahead buffer until the next key shows
  whether the mapping is meant, and libvim's `sm_execute()` put each new key
  *in front of* the one waiting. The pair never completed, and the first key
  was never seen again. The fork appends now, so keys are taken in the order
  sent, and it gained the two calls the rest needs: `vimInputPending()` and
  `vimInputTimeout()`.

  What vimango does with them:
  - **The timeout.** vim gives up on a half-typed mapping after
    `'timeoutlen'` and takes the key as typed. libvim cannot wait, so
    `MainLoop` keeps a timer (`mappingWait`, `editorMappingTimeout`). Without
    it a `j` typed at the end of a word would not appear until the next key.
  - **The held key is drawn at the cursor** in INSERT mode, as vim draws it,
    so typing does not seem to lag by a key (`drawAfterKey`).
  - **A mapping that ends on a command line** (`nnoremap ,s :%s/`) opens the
    ex line with its text on it. The editor went by the key typed and took
    anything but `:` for the start of a search; it asks vim now. This was
    wrong for single-key mappings too.
  - **Leaving INSERT by a mapping clears `-- INSERT --`.** Only a typed
    `<esc>` did.
  - `processKey` split in two for this: `forwardKey` is everything after the
    key has been given to vim, and a timeout runs it with no key. `MainLoop`
    is `startKeyReader`, `handleEvents`, `Cleanup`, so that a test can feed
    the loop keys (`TestEventLoopTimesOutAHalfTypedMapping`).

  Checked in the fork with ten new tests in `apitest/mapping.c` (eight of them
  fail with the old order), here with `mapping_test.go`, and by driving the
  real `--editor` in a pty: `j`, a second's pause, `jk`, `,d`, `:x`.

  Found on the way, in the fork (`cc8b7f53d`): ten of its sixty test programs
  ended with `return minunit_status;`, which is the status of the *last* test
  only, so `build-macos.sh --tests` printed PASS for them whatever the earlier
  tests had done. None was hiding a failure.

  One trap when testing mappings: `inoremap j X | inoremap jk Y` maps `j` to
  `X` and a space. vim ends a `:map` at the `|` and keeps the blank before
  it.

- **Commands that list something say it** (fork `6fc194385`). `:registers`,
  `:history`, `:messages`, `:version` and a bare `:map` were the known silent
  ones. A survey of 93 command lines in libvim found the gap much wider --
  `:display`, `:abbreviate`, `:command`, `:print`, `:number`, `:list`,
  `:g/pat/p`, `:autocmd`, `:tags`, `:echon` -- and found what the bare `:let`
  hang really was.

  One cause for all of it. vim writes a listing a piece at a time
  (`msg_puts()`, `msg_putchar()`, `msg_advance()`), and every piece ends in one
  function, where upstream libvim dropped it: no screen. The same deleted
  screen code was what advanced `msg_col`, and `msg_advance()` pads with blanks
  *until `msg_col` reaches a column* -- so `:let`, `:let g:var` and
  `:highlight` did not wait for a key to page with, as CLAUDE.md said; they
  looped. The fork keeps the column now and collects the text while an ex
  command runs, sending it as one message when the command ends.

  Here: nothing in the display path changed -- `showVimMessages` already put a
  multi-line message in the overlay. `:let` with no assignment is forwarded,
  but only when the linked library can list (`vim.ListsOutput()`): an older
  `libvim.a` links just the same and would freeze the editor on it.
  `TestForwardedCommandsDoNotHang` had been spending four 5-second timeouts on
  the refused `:let` forms; the package's tests went from 8 s to 3.4 s.

  Also in the fork: `:version` and `:messages` had been handed to callbacks
  vimango does not set, and fall back to vim's own output; `:echomsg` is kept
  in the message history again, so `:messages` has something to list;
  `:echoerr` sent its error twice; a bare `:history` is the command history
  whatever kind of line was typed last.

  Checked by the survey, before and after: the only differences are output
  that was missing, in vim's order, nothing said twice. Fork tests:
  `apitest/listings.c`. Here: `TestListingsAreShown`,
  `TestOldLibraryDoesNotListVariables`.

- **A typed search is one search** (fork `6bc47d62d`). The known problem was
  the wrap warning on the Enter of a typed `/pattern`, which vimango dropped
  because it was wrong. The cause was larger than the warning. While a pattern
  is typed, `'incsearch'` moves the cursor to the match; vim puts it back when
  the command line returns and then searches. libvim did not put it back, and
  made up for that by searching twice from wherever the cursor was, forward
  and then back. That lands on the right match for a plain pattern -- and:
  - warned for the two extra searches rather than the real one (both "hit
    BOTTOM" and "hit TOP" for a search that did not wrap; nothing for
    `?pattern` from the first line, which does);
  - reported `E486: Pattern not found` for `/pattern/e` and `/pattern/+1`,
    after going to the right place;
  - left the cursor on the match when a search failed under `'nowrapscan'`;
  - did not repeat the last pattern for an empty `/`, and set the `''` mark
    in the wrong place;
  - found the *previous* match with `'noincsearch'`.

  The fork restores the cursor and runs vim's one search. Here the
  `endsSearch` exception is gone, and `showKeyMessages` shows an error alone:
  vim warns that it is wrapping before it knows the search will fail, and on
  vim's screen the error overwrites the warning.

  Checked with 29 typed-search scenarios in libvim, cursor and messages,
  before and after; fork tests `apitest/search_typed.c`; here
  `TestTypedSearch` and the end of `TestVimReportsForKeysAreShown`.

- **Ctrl-R works, and `"=` no longer freezes the editor** (fork `3674896fb`).
  The known limit was that `<C-r>` in an insert-mode mapping inserted a
  literal `^R`. It was not about mappings: typed Ctrl-R did the same. vim
  reads the register's name by waiting for a key, and for `=` reads a whole
  command line; libvim is handed its keys and cannot wait, and upstream had
  taken that code out of Insert mode and of the command line -- where Ctrl-R
  was dropped and the register's *name* typed -- and left it in Normal mode,
  where **`"=` and `@=` never returned**. Typing `"=` in the editor froze it,
  with the note unsaved.

  In the fork each state now remembers that it is owed a key. Ctrl-R works in
  INSERT (`<C-r>a`, Ctrl-R Ctrl-R/Ctrl-O/Ctrl-P, `<C-r>=expr<CR>`) and on the
  command line (registers, Ctrl-R Ctrl-W for the word under the cursor,
  `<C-r>=`), and `"=` / `@=` ask for their expression. An expression is typed
  on a command line of its own, stacked on whatever asked for it.

  Here that meant the command line had to stop being tracked in Go. The
  search line mirrored each key typed into a string of its own, and the ex
  line assumed a line, once open, stayed the `:` line until Enter or Esc.
  Now `syncCmdline` asks vim after every key which kind of line is open and
  what is on it, so that: Ctrl-R on a search line shows what vim has; the `=`
  line shows as `=`; **Enter on the expression's line goes to vim** (vimango
  takes the Enter of a `:` line for itself, and would have run the expression
  as a command); and Esc that ends only the expression, or only a waiting
  Ctrl-R, leaves the line under it open. The cursor on a search line is now
  where vim has it, as on the ex line.

  Checked with 49 key sequences in libvim before and after (none hangs or
  crashes), fork tests `apitest/ctrl_r.c`, and `ctrlr_test.go` here.

  Found on the way, **not fixed**: `input()`, `inputsecret()` and
  `inputlist()` still call vim's waiting readers and never return --
  `:echo input('x')` freezes the editor. `getchar()` and `confirm()` return.

- **A script that asks the user no longer freezes the editor** (fork
  `76bc49547`). `:echo input('x')` never came back, nor did a mapping or a
  function that called `input()`, `inputsecret()`, `inputdialog()` or
  `inputlist()`, nor `:execute 'debug …'`. All of them end in vim waiting for
  a key, and no key can reach libvim while it is running a command: keys are
  handed to it between calls. The wait was a loop with no end.

  vim has the same problem inside `:normal`, where the keys it was given can
  run out, and answers it by handing the reader the key that makes it give
  up. The fork now gives that answer whenever a key is asked for and none is
  waiting, so the question is answered as Esc would answer it: `input()`
  returns `''`, `inputlist()` 0, the debugger continues. The script goes on.

  This is a floor, not support -- the user is never actually asked. Checked
  with 36 commands and key sequences in libvim, each in its own process
  (11 hung before, none after); fork tests `apitest/no_waiting.c`; here they
  are in `TestForwardedCommandsDoNotHang`.

  Also learned: `feedkeys()` with the `x` flag does nothing in libvim, and
  libvim refuses `:s` with the `c` flag itself, with an error, so that one was
  never a hang.

- **`docs/VIM_COMMANDS.md`**: a list to test the editor against by hand, in the
  TUI and in hybrid -- what vim's commands now do, what is blocked on purpose
  and the message each gives, and what is still missing. Its expected results
  were taken from running some ninety commands through the editor. Writing it
  turned up three things not on any list: function keys never reach vim
  (`termcodes` does not name them), Ctrl-\ Ctrl-N in INSERT inserts its two
  control characters as text, and `go.work` was still active on this machine
  (`../glamour` was at the pinned commit, so nothing differed). `go.work` and
  `go.work.sum` were deleted the same day; the build uses the pinned glamour.

- **`gj`, `gk`, `H`, `M` and `L` work.** They did nothing: upstream libvim
  took the screen out of vim's `nv_screengo` and `nv_scroll` and left two
  callbacks (`vimSetCursorMoveScreenPositionCallback`,
  `vimSetCursorMoveScreenLineCallback`) for the embedder to answer, since only
  the embedder knows how lines are drawn. vimango never registered them, so
  `gj`/`gk` failed silently and `H`/`M`/`L` stayed on their line. Now the
  editor answers them (`screen_motion.go`) from the layout it draws -- word
  wrap by `wrapRow`, tabs four columns -- and the binding (`vim/screen.go`)
  turns its byte offsets into vim's virtual columns with vim's own `getvcol`,
  so 'tabstop' and wide characters land right. No change to the fork.
  Telling libvim the pane's width and letting vim compute instead would not
  have worked: vim wraps at the width, vimango at word boundaries. Tests:
  `screen_motion_test.go`.

  Learned: in libvim the first command in a new buffer ends with vim
  recomputing its wanted column (`w_set_curswant` is still set from the
  buffer switch, and `vimInput` ends in `update_curswant()`), so a `$` typed
  first forgets that it meant "the end". Harmless, but it confuses tests.

- **Enter continues a list.** In INSERT mode, Enter on a line that is a list
  item -- `- `, `* `, `+ `, `1. `, `1) `, after any indentation -- starts the
  next item with the same marker, and Enter on an item with nothing in it
  clears the line and ends the list. All of it is `vimango.vim`: an
  `inoremap <expr> <CR>` on `VimangoListEnter`, which uses the patterns the
  visual `-` / `#` toggles already had. No Go and no libvim change. Tests:
  `TestVimrcEnterContinuesAList`, `TestVimrcEnterEndingAListStaysInInsert`
  (`vimrc_test.go`).

  Choices made: the marker is repeated as written rather than counted up (a
  list numbered `1. ` throughout stays so); the new line's indentation is
  left to 'autoindent' when it is set, and written by the mapping when it is
  not; ending a list is `<C-o>0<C-o>D`, not `<C-u>`, which leaves the
  indentation of a nested item and depends on 'backspace' when the marker was
  not typed in this insert. vim's 'comments' with 'formatoptions' `ro` was
  the alternative: it continues bullets on Enter and on `o`, but knows
  nothing of numbers and has no way to end a list.

  Known: `o` in NORMAL mode on a list item opens a plain line. And at this
  commit (`2aa57d5`) pasted text was still typed keys, so a list pasted in
  INSERT mode got a second marker on every line unless `:paste` was on; the
  next entry ends that.

- **Pasted text is a paste, not keys (bracketed paste).** A terminal sends a
  paste down the same pipe as the keyboard, and vimango gave every character
  of it to vim as a typed key: 'autoindent' indented each line under the one
  before (4, 8, 12 spaces for three lines indented 4), a mapping fired on
  every key it was for -- the `<CR>` mapping above made `- b` into `- - b` --
  and a paste in NORMAL mode, or in the organizer, ran as commands. Now
  vimango asks the terminal for bracketed paste (`ESC [?2004h` before
  `MainLoop`, `l` in `quitApp`), the key reader returns everything between
  `ESC [200~` and `ESC [201~` as one `KeyPaste` event (it used to read the
  start marker and throw it away), and `MainLoop` hands the text to
  `Editor.pasteText` or `Organizer.pasteText` (`paste.go`).

  In the note the text is typed into vim with 'paste' set and mappings off,
  after dropping control characters other than tab and line end. Where it
  lands is what vim does with a bracketed paste: at the cursor in INSERT;
  after the cursor in NORMAL (before it in column 0), with a pending operator
  or count dropped; over the selection in VISUAL (`c`); on a command line as
  one line, not run. Being typed, a paste is one insert: `u` undoes it and
  `.` repeats it. The organizer takes it into the title being edited or its
  command line and drops it elsewhere, with a message. `forwardKey` was split
  so that a paste shares what follows a key (`followVim`) without the mapping
  expansion.

  Why not vim's own code: libvim still has `bracketed_paste()`, behind the
  `<PasteStart>` key, but sending that key segfaults (`STRLEN` of a termcode
  libvim does not define) and the function reads the paste with `vgetc()`
  from a terminal libvim does not have. Listed under "Open items"; vimango
  does not send the key.

  Measured: 250 KB (4,000 lines) pasted in 0.2 s. Checked with the real
  binary on a pseudo-terminal against a scratch database -- `--editor`
  (a nested list pasted in INSERT, saved with `:x`) and the TUI (a paste in
  NORMAL does nothing, one into a title and one onto the command line) --
  and that `ESC [?2004h` and `l` are both sent. Not checked by me: Cmd-V in
  kitty, Ghostty, Rex and hybrid's pop-out, which is the user's test. Hybrid
  needs no change of its own; its terminal is libghostty, which marks a paste
  once the program asks. No libvim rebuild. Tests: `paste_test.go`,
  `terminal/terminal_test.go`.

  Left as it was: a death by signal restores neither raw mode nor this (a
  shell sets bracketed paste for itself at its next prompt).

## 2026-11-13

- **The editor refuses `:sync`, in the TUI as in `--editor`.** In the TUI,
  `:sync` typed in the editor used to hand over to the organizer and run the
  sync there, with the note still open. Two things were wrong with that, both
  found by reading the code for what the editor still reaches outside itself:
  - **Unsaved edits could be lost without a word.** The handover cleared
    `Session.editorMode` and left the editor, modified, in `Session.editor`.
    Nothing led back to it; the organizer's `:q` asks only about unsaved
    *titles* and quit. Opening any note replaced it. (Confirmed with a test
    before the fix: buffer modified, `app.Run` false after `:q`.)
  - **`:test` and `:sync?`, the dry runs, ran a real sync.** The editor
    passed the organizer `exArgs{name: "sync"}` whatever was typed; the
    organizer has told a dry run by `ex.name` since commands took `exArgs`,
    and the line that copied the editor's command line across for it no
    longer did anything. Read, not run: there is no server in the tests.

  Now all three names answer "sync is the organizer's: close the note (:x or
  :q), then :sync" and nothing else happens (`noSync`, `editor_cmd_line.go`);
  in `--editor` the answer is the host's, as before. The names stay in the
  editor's registry so that the answer is this one and not vim's -- `:sync`
  is vim's `:syncbind`. A TUI change only, and Steve's decision: hybrid never
  ran the editor's sync and injects only `:checkstale`, `:q`, `:x` and `:q!`.

  With it the editor's uses of the global `app` are down to one,
  `app.noteClosed()`. Test: `TestSyncIsRefusedInTheEditor`
  (`session_test.go`), which fails on the old code.

- **The editor closes through `onClose`, and `Session.editorMode` is gone.**
  Two things, one commit, both about who knows a note is open.
  - `closeNote` called `app.noteClosed()`: the editor reaching the global for
    what follows its closing. It now calls `e.onClose`, a function the editor
    is given when it is made (`App.NewEditor` gives it `a.noteClosed`). The
    editor's files no longer use `app`. What the TUI and `--editor` do on
    a close is unchanged, and is still `App.noteClosed`.
  - "A note is open" was held twice, as `Session.editor` and as
    `Session.editorMode`, and the two were set in different places. The
    editor's `:sync` (above) was where they disagreed. `editorMode` is
    removed: `Session.editor != nil` is the one answer, to where a key goes
    (`handleEvents`), where the cursor is (`returnCursor`), what a resize
    redraws, and whether the preview may draw (`previewVisible`).

  No behaviour change intended. Tests: `TestEditorClosesThroughOnClose` closes
  an editor with the global `app` nil; the suite's other close tests go
  through `onClose` too. Checked with the real binary on a pseudo-terminal and
  a scratch database: `--editor` (`:q` clean exits; `:q` modified stays, with
  the message; `:x` writes and exits) and the TUI (`:e`, `:q` refused, `:q!`
  back to the organizer, the next key goes to the organizer, `:e` again,
  `:x`, `:q`).

  Still shared with the TUI, for later: the editor holds `Session` and
  `Screen`, asks `Session.editorOnly` for its layout in five places, and its
  notice box reads the glamour style through `app.Config`.

- **The editor is given its layout.** Its drawing code asked
  `Session.editorOnly` in five places -- where messages go, how many lines of
  text and from which row, whether to draw a status bar, whether to draw the
  frame's corner, what `:sync` answers -- and read the screen's size and
  divider besides. Every one of those was the editor working out which
  program it was in. Now `Screen.editorLayout()` works it out once, on the
  screen's side, and hands the editor an `editorLayout`: the text's
  rectangle, `statusBar`, `corner`, and two `messageLine`s (`message`, and
  `aside` for what the TUI shows under the organizer). `positionEditor` sets
  it at open and at every resize. The `:sync` answer is `App.syncAnswer()`,
  given to the editor when it is made.

  Moved with it: the editor's half of `App.returnCursor` is `Editor.cursor()`
  (the app was reading the editor's margins and the screen's rows to place a
  cursor on the editor's command line); `setLinesMargins` is gone; and
  `drawFrame`'s choice between two corner characters is gone -- it compared
  the frame's column with the screen's width, was left from when several
  editors sat side by side, and has been true since there was one.

  **One change to what is sent to the terminal**, none to what is seen: the
  editor clears a message line with "erase N characters" (`ESC [ N X`, which
  it already used for every line of text) from the line's start, in place of
  "erase to end of line" on the right and "erase to start of line" on the
  left. A layout that states a width has to clear that width.

  Checked by running the binary from before this change and the one after
  through the same keys on a pseudo-terminal, each on its own copy of the
  database, and comparing everything written to the terminal step by step:
  `--editor` (23 steps: edit, `:w`, undo, `:q` refused, `:help` and paging,
  search, visual, `:sync`, `:marks`, a resize, `:number`, `z=`, `:q!`) and the
  TUI (the same inside `:e`, then `:x`, the organizer, `:e`, `:q`). Apart from
  the erase above the two are byte for byte the same. Tests:
  `editor_layout_test.go` -- the two layouts, and that an editor given the
  other program's layout draws that one, whatever the session's flag says.

  Found on the way, not changed: `drawSpellErrors` (`spellcheck.go`) tests
  `y <= e.screenlines` with a `y` that already includes the top margin, so in
  the TUI a misspelling on the last line of the pane is not marked. In
  `--editor`, where the text starts on row 1, it is.

- **Misspellings are marked on the last line of the TUI's pane.** The one
  noted above. `drawSpellErrors` asked whether a word was on the pane with
  `y >= top_margin && y <= screenlines`, where `y` was a screen row with the
  top margin already added: right when the text starts on row 1, as in
  `--editor`, and one line short when it starts on row 2, as in the TUI --
  so hybrid never showed it. The test is now on the line within the pane,
  `0 <= line < screenlines`, and the margin is added only to draw. Test:
  `TestSpellErrorsAreMarkedToTheLastLineOfThePane`, for text starting on
  rows 1, 2 and 5, and for a word pushed below the pane by a row that wraps;
  it fails on the old code for rows 2 and 5.

- **What is left of the editor's move to a package** is written down in
  `docs/EDITOR_PACKAGE.md`, for the next session: what the editor still takes
  from outside itself (found by compiling its files alone), what it shares
  with the organizer, the order to do it in, and what comes after.
  `scripts/compare-drawing.py` is the check used for the layout change, kept:
  two builds, the same keys on a pseudo-terminal in both modes, every byte
  compared. Run on the build before the spell fix and the one after, it
  reports one difference -- a misspelling on row 38 of the TUI's pane, now
  marked -- and none in `--editor`.

## 2026-11-14

- **The editor holds no `Session` and no `Screen`.** Step 4a of
  `docs/EDITOR_PACKAGE.md`, the first half: what the editor took from those
  two it is now given when it is made.
  - **The markdown style.** `highlightChunk` read `e.Session.markdown_style`,
    the editor's only use of the session. It is `Editor.markdownStyle`, from
    `App.markdownStyle`; `Session.markdown_style` is gone (nothing else read
    it).
  - **The notice box.** `:help`, a vim listing and HELP mode's keys called
    `e.Screen.showNotice` and `e.Screen.noticeKey`. The box is still the
    screen's, and the editor is given it as `noticeBox`, an interface of
    those two methods.
  - **`:checkstale` redraws the editor itself.** After a reload it called the
    screen's `eraseRightScreen` and `drawRightScreen`, which is the screen
    erasing the pane and then asking the editor to draw. It calls
    `Editor.draw()` -- the text, the frame, the status bar -- which is also
    what `drawRightScreen` now calls. Nothing needs erasing first: `drawText`
    clears each line before it writes it.
  - **The notice's glamour style** came from `app.Config`, the global, on
    the editor's `:help` path. The screen is given the configured name at
    startup (`Screen.glamourStyle`) and `glamourStylePath` does the fallback
    to `default.json`; the organizer's preview still asks through
    `getGlamourStylePath`.

  Checked: the suite; `scripts/compare-drawing.py` on the builds before and
  after says `SAME` in `--editor` and in the TUI. `:checkstale` is not in
  that script, and its bytes are meant to differ (no erase), so the *screen*
  each build leaves after a reload was compared instead, with a terminal
  emulator (pyte) reading the pseudo-terminal: `--editor` and the TUI, with
  the cursor on the last line of a longer note, with `:help` up, in VISUAL,
  and with line numbers off. Every cell and attribute the same.

  That comparison caught a first attempt that left the frame out
  (`drawText`, `drawStatusBar`), and with it something older, **not
  changed**: the line numbers set a grey foreground (`ESC[38;5;245m`) and
  put back only the background, so whatever is drawn straight after
  `drawText` is grey. In the TUI the status bar is the terminal's own colour
  when a note is opened and grey from the first key that redraws; a message
  shown after a redraw in the same key (`:number`, `:syntax`) is grey.
  `drawFrame` ends with a reset, which is why a reload looked right. Which
  colour the bar should be is Steve's to say.

- **The editor has a constructor of its own.** The rest of step 4a.
  `(*App).NewEditor`, `(*App).setEditorExCmds` and the first half of
  `(*App).openEditor` were methods of the app that used nothing of it but
  five things to hand on. Those five are now a value, `editorHost` (the
  store, the notice box, the markdown style, `onClose`, the `:sync` answer),
  embedded in `Editor` beside `editorLayout`: between them, everything an
  editor knows of the program it is in.
  - `newEditor(host)` is an editor with no note; `openNote(host, id, title)`
    reads the note from the host's store into a vim buffer and returns the
    editor, or an error and no editor. `editorCommands()` is the registry of
    vimango's own ex commands.
  - `App.editorHost()` is what this app gives; `App.openEditor` is now
    `openNote`, then the app's part: `Session.editor`, `positionEditor`, and
    the first draw.
  - Two fields of `Editor` that nothing read, `redraw` and `modified`, are
    gone. (`NewEditor` set them, which is how they were found.)

  No behaviour change: `compare-drawing.py` says `SAME`, and the
  `:checkstale` screens match as above. Test: `TestEditorIsMadeFromItsHost`
  (`editor_host_test.go`) makes an editor with no `App`, `Screen` or
  `Session` and the global nil -- a notice box that only records, a store, a
  layout -- and opens, draws, shows `:help` and `:marks`, writes, reloads,
  and closes it.

- **`paste.go` is split.** The last of step 4a, and only a move:
  `Editor.pasteText` and `pasteIntoVim` are `editor_paste.go`, which goes
  with the editor; `Organizer.pasteText` is in `organizer_process_key.go`;
  `paste.go` keeps what both use, `pastedText` and `onOneLine`.

  With that, compiling the editor's files alone in a throwaway package (the
  command is in `docs/EDITOR_PACKAGE.md`) finds nothing of the host left
  undefined -- no `App`, `Session`, `Screen` or `app` -- only the 38 names
  the editor shares with the organizer: the modes, the key names,
  `sendToVim`, `clipColumns`, the wrap helpers, the command registry,
  `writeFileAtomic`. Giving those a home is step 4b, and one package or
  several is Steve's decision first.

- **Drawing the text leaves no colour set.** The leak noted in the first
  entry above, fixed where it starts: the line numbers put back the
  foreground as well as the background (`drawRows`). Steve's choice for the
  TUI's status bar: the terminal's own colour, always.
  - **What changes on screen:** in the TUI the status bar no longer turns
    grey at the first key that redraws; the messages of `:number` and
    `:syntax` are not grey (hybrid too).
  - **What is kept:** the visual selection and a matched pair of braces were
    grey text on a darker ground by the same accident -- only with line
    numbers on, and only when nothing drawn in between (a visible
    misspelling, a search match) had reset the colour. With numbers on and
    spell off, which is how an editor starts, that was grey, so grey is now
    said outright: `shadedStyle` (`editor_methods.go`), one constant to
    change if the selection's text should be the terminal's colour instead.
    In the states where the accident went the other way the selection is
    now grey too.

  Checked by comparing screens, not bytes: both builds through
  `compare-drawing.py`'s keys and more (selections with line numbers on and
  off, with a search highlighted, with `:spell` on; a matched brace), read
  by a terminal emulator, every cell and attribute compared after every
  step. The differences are those above and no others. Test:
  `TestDrawingTheTextLeavesNoColourSet`, which fails without the fix.

- **`internal/atomicfile`.** Step 4b begins: the things the editor shares
  with the rest get a home it can import. Steve's decision: two packages,
  this one and `internal/pane` (next). `writeFileAtomic` is
  `atomicfile.Write` and `removeStaleTempFiles` is `atomicfile.RemoveStale`;
  the callers are the registers (the editor's), the preferences and the image
  cache. Their two tests moved with them; the image cache's test that shared
  the file is `image_cache_test.go`. A move only.

- **`internal/pane`: what the organizer and the editor both use.** The
  second home (step 4b). Moved, and exported:
  - the command registry (`command_registry.go` is `pane/registry.go`):
    `CommandRegistry`, `CommandInfo`, `NewCommandRegistry`, `NewKeyRegistry`,
    and `KeyDisplayName` (was `keyToDisplayName`);
  - `ClipColumns` (from `organizer_display.go`), `RuneStart` and
    `InclusiveEnd` (from `word_wrap.go`);
  - how a key goes to vim (from `common.go`): `SendToVim`,
    `IsSyntheticKey`, and `VimKeyName(c)` in place of the `termcodes` map,
    which is now the package's own. The organizer's key names (`ARROW_UP`,
    …) stay in `common.go`; they are aliases of the reader's;
  - what a paste is (`paste.go` is `pane/paste.go`): `PastedText`,
    `OnOneLine`.

  The tests of those moved with them (`TestClipColumns`,
  `TestPastedTextIsCleaned`), and two are new (`TestRuneEdges`,
  `TestKeysVimUnderstands`). The registry's tests go through the organizer's
  commands and stay. A move only: `compare-drawing.py` says `SAME`.

  The package imports `vim` (to send a key), so like `vim`'s own tests its
  tests link `libvim.a`: run `go test` from the project root.

- **What is the editor's alone is in the editor's files.** Step 4b, the
  part that needed no package: things the editor used from files it shares
  a directory with but that nothing else used, or used differently.
  - **Two message methods.** `ShowMessage(BR, …)` and `ShowMessage(BL, …)`
    took the organizer's `Location` to choose between the editor's two
    lines. They are `ShowMessage(…)` and `ShowAside(…)`; `Location` is the
    organizer's again.
  - **The wrap helpers** only the editor calls -- `wrapRow`, `wrapSegment`,
    `expandedOffset`, `byteAtDisplayCol`, `byteAfterDisplayCol`, `tabWidth`
    -- are `editor_wrap.go`, out of `word_wrap.go`, which is now only the
    preview's `WordWrap`.
  - `LEFT_MARGIN_OFFSET` is `lineNumberCols`, in `editor.go`; the editor's
    three uses of `RESET` and one of `RED_BG` are its own `sgrReset` and a
    literal.

  A move and a rename: `compare-drawing.py` says `SAME`.

- **The editor's tests stand without the app.** Step 4c, the first half:
  the editor's tests are most of the root package's, and they made their
  editors on a real `Screen` and `Session`, and wrapped an `App` around one
  to ask whether it had closed (`a.Run`, `a.Session.editor`). None of that
  can come along into a package that does not import the application.
  - **`testHost`** (`editor_testhost_test.go`) is who opens a test's editor:
    a notice box that records what it is shown (and writes it out, so a test
    still reads a listing from the output), and a count of the times the
    editor said its note was closed. `newTestEditor` uses it, with a layout
    written out (`tuiLayout`, `hostAppLayout`: the two the screen makes), and
    counts the note as loaded as saved, as opening one does.
  - **The tests that asked the app now ask the host**: `h.open()` where they
    read `a.Run`. `TestExitStaysOpenWhenTheWriteFailsInTheTUI` is gone: to
    the editor there is one way to close, whoever opened it.
  - **The tests of the app with an editor in it are written as that**
    (`session_test.go`, `editor_only_test.go`): `tuiApp` / `editorOnlyApp`
    make an app over a store, `openIn` opens a note with `App.openEditor`,
    and what they read of the editor is what it says of itself -- `ID()`,
    `Title()`, `Mode()`, `Modified()`, new -- or what vim and the database
    hold. `TestFailedSaveKeepsTheNoteOpen` is the app's side of the removed
    test, for both ways of running. The two event-loop tests are
    `event_loop_test.go`, the organizer's paste test
    `organizer_paste_test.go`, the screen's layouts `screen_test.go`.

  Tests only, but for the four accessors. 218 tests before and 219 after:
  one removed, and two added -- `TestFailedSaveKeepsTheNoteOpen`, and
  `TestLongTitleIsCutAtACharacterBoundary`, the organizer's part of a test
  whose other part stays with the editor.

- **The editor is a package: `internal/editor`.** Step 4c, the move, and
  the end of the plan that was `docs/EDITOR_PACKAGE.md` (deleted; what
  comes after is under "Open items" in `CLAUDE.md`). The editor's fourteen
  files and twenty test files moved, with their names; the package imports
  `vim`, `internal/pane`, `internal/store`, `internal/atomicfile` and
  `terminal`, and nothing of the application. The compiler now keeps what
  the last three days arranged by hand: nothing in the editor can reach the
  organizer, the screen, the session or the global `app`.
  - **Its public face** is what the application already used: `Open(host,
    id, title)` (was `openNote`); `Host` and `Layout` (`editorHost`,
    `editorLayout`) with their fields exported, and `MessageLine`;
    `NoticeBox`, whose two methods are `ShowNotice` and `NoticeKey` -- the
    screen's are renamed to match; on an editor `ProcessKey`
    (`editorProcessKey`), `PasteText`, `MappingTimeout`, `Draw`,
    `DrawAfterKey`, `Cursor`, `ShowMessage`, `ShowAside`, `ID`, `Title`,
    `Mode`, `Modified`; and `InitSpell`, `SourceVimrc`, `RestoreRegisters`,
    `SaveRegisters`, `LoadMarkdownStyle` (`selectMDStyle`), `MappingWait`,
    `NoKey`. The application sets a layout with `e.Layout = …`.
    `Highlight2` and `MDFormatter`, which nothing outside used, are
    `highlightText` and `markdownFormatter`.
  - **The editor's modes are its own** (`editor.Mode`, `mode.go`), decided
    with Steve. In its own package it keeps the names it had, so none of
    its code changed for this. The organizer's `Mode` (`common.go`) lost
    what only the editor used -- `EX_COMMAND`, `VISUAL_LINE`,
    `VISUAL_BLOCK`, `SPELLING`, `OTHER` and `visualModeMap` -- and `LINKS`,
    which nothing used at all.
  - **Tests.** The editor's run in the project root, as they did
    (`TestMain` goes there: `vimango.vim`, `spell/` and the chroma style are
    found by working directory). `captureStdout`, `newTestStore`,
    `stripANSI` and `ctrlKey` exist on both sides now, a few lines each.
    `TestKeyConstantsTrackTheReader`, which is about the organizer's key
    names, stayed in the root package (`keys_test.go`). 211 tests in the two
    packages before and after, all run (counted from `go test -json`), with
    the same two helper "tests" skipped. `TestForwardedCommandsDoNotHang`
    still finds its own binary.

  No behaviour change. Against the build from before step 4b:
  `compare-drawing.py` says `SAME` for `--editor` and the TUI; the screen
  comparison (39 and 41 steps) differs only in the age vim gives an undo;
  the six `:checkstale` screens are the same.

  One thing for vimango_hybrid's `CLAUDE.md`, not changed from here: it
  names `~/vimango/highlight.go` for the `terminal16m-md` formatter, which
  is now `~/vimango/internal/editor/highlight.go`.
- **The editor's own markdown highlighter replaces chroma.** chroma's
  markdown lexer, run a chunk at a time with a cache (2026-11-10), gave way
  to `internal/editor/markdown.go`, which styles headings, bold, italic,
  strikethrough, inline code and fenced code and nothing else, one row at a
  time. Asked for by Steve: the notes are markdown, and syntax highlighting
  of programming languages was no longer wanted. What went: `highlight.go`
  (the `terminal16m-md` formatter that added SGR 9), the chunking and the
  highlight cache in `editor_rows.go`, `Host.MarkdownStyle`,
  `LoadMarkdownStyle`, `config.json`'s `chroma.style` (an old config still
  loads; the key is ignored) and `gruvbox_mod.xml`, whose colours are now
  constants. chroma stays a dependency: glamour uses it, and
  `frontmatter_test.go` checks `splitFrontmatter` against it.
  - **Measured**: a redraw at the bottom of a 900-row note, 308 µs with the
    chunk cache, 68 µs now with none (`BenchmarkRedrawAtBottomOfLongNote`);
    chroma over the whole note was 16.8 ms.
  - **Changed on purpose**: a fence may be indented and have any info string
    (```` ```c++ ```` was not a fence to chroma), and an unclosed fence runs
    to the end of the note (to chroma it was no fence). `***x***` is bold and
    italic. Frontmatter is plain, not coloured as YAML; links, list markers,
    quotes and HTML comments are no longer coloured. A stray backtick is
    literal and affects nothing, so the nine notes CLAUDE.md listed as left
    with one need nothing now.
  - **Checked against the real database** (4,560 notes, 97,893 rows): every
    row's text drawn unchanged, apart from 13 rows that hold raw terminal
    escapes (pasted compiler output), which the check's own escape-stripping
    cannot compare; the slowest whole note drew in 0.4 ms. Three notes end
    inside a fence by the new rule: 2414 (a row of `~` used as a rule), 5612
    (a closing fence with no opener, on its last row) and 5638 (a closing
    fence with a citation link stuck to it), each coloured as code from there
    to its end.
  - **`:syntax` no longer touches the margin.** Turning highlighting on set
    `left_margin_offset` to the line numbers' width whether or not they were
    on, so with `:number` off it left a blank four-column margin and wrapped
    the text four columns short (`TestSyntaxLeavesTheMarginAlone`).
  - vimango_hybrid's `CLAUDE.md` says the editor's colours come from
    `gruvbox_mod.xml` and its attributes from `highlight.go`, and that the
    editor lexes frontmatter as YAML; none of that is so now. Not changed
    from here.
- **Code in the editor is `#e04212`**, the rendered HTML's
  `webview.code_accent`, instead of chroma's `#ff5f5f`, so the editor and
  the preview show code in the same colour. It is a constant
  (`mdCodeColour`), not read from `config.json`.
- **The editor and the preview agree exactly on frontmatter.** The rule
  moved from `frontmatter.go` into a package of its own,
  `internal/frontmatter` (`Split`, `Parse`, and `Rows` for the editor),
  which both use; `frontmatter.go` keeps only the preview's card. Two
  differences went: the editor took a closing `---` on a note's last row,
  with no line break after it, where `Split` does not; and it drew plain
  any block `Split` found, where the preview shows a block as frontmatter
  only when `Parse` can display it (no `# comment`, anchor, alias or flow
  YAML) and as markdown otherwise. Over the real database's 4,560 notes the
  two now agree on all of them; 35 have frontmatter.
- **`:syntax on` / `:syntax off`, and `:md`.** `:syntax` was a bare
  toggle, so what it did depended on a state the screen does not show. It
  now takes vim's own `on` and `off` (a bare `:syntax` still toggles, and
  anything else gets a usage line), and answers to `:md` and `:markdown`,
  which say what it is for. `:highlight` was considered and left alone: it
  is vim's command for colours, refused here on purpose.
- **With highlighting off the note is body text, not bold.** Off, the rows
  were sent with no colour at all and took the terminal's default
  foreground -- in Ghostty and libghostty a brighter white than the body
  text's `#c4c4c4` -- so the whole note looked bold beside it (traced: no
  bold attribute was ever set). Now every row is drawn as unstyled body
  text, in the same grey as with highlighting on, whatever the terminal's
  default (`TestHighlightingOffIsBodyText`). `wrappedRow`, which drew the
  rows uncoloured, is now only the tests' reference for the text.

## 2026-11-15

- **`<esc>` takes the path every key takes.** `handleKey` sent `<esc>` to
  vim itself and returned, having copied back what an `<esc>` was expected
  to change: the text, the cursor (unclamped), the mode. What vim did beyond
  that was not followed -- its messages were not captured, the tick was not
  checked, and nothing looked at which buffer vim was on. It matters more
  than it reads: vimango_hybrid starts every command it injects with `<esc>`
  (`:checkstale` after each sync, the `:q` that asks about unsaved changes),
  so this was the first key of the host's whole control channel. Now
  `handleKey` hands it to `followVim` like any key, sent with `vim.SendKey`
  so that no mapping sees it, and `followVim` knows the two things that are
  an `<esc>`'s own: it clears the message line, and it can leave a command
  line open (it ended a Ctrl-R, or the expression for one).

  Four things were wrong, each confirmed with a test that fails on the old
  code (`esc_test.go`):
  - **A key vim was holding went in ahead of the `<esc>` and was not
    drawn.** With `nnoremap xx dd`, `x` then `<esc>` deleted a character
    and the screen kept it until some later key redrew. With a held `u` the
    undo happened unseen, and vim's "1 change; before #1" was never shown.
  - **A held Ctrl-^ left vim on another buffer**, where the next keys were
    typed: the check that brings vim back to the note (2026-11-11) ran for
    every key but this one.
  - **An `<esc>` that edits** -- `3ix<esc>`, an insert on a block -- was
    drawn, but the tick was not recorded, so the next key redrew the whole
    text for nothing.
  - **The selection outlived VISUAL.** `v`, a motion, `:` and then `<esc>`
    left the selection drawn with vim in NORMAL and nothing selected; moving
    the cursor did not clear it. The cause was one step earlier: vim ends
    VISUAL at the `:` (the line opens on `'<,'>`), and a key that opened a
    command line returned before the redraw. Now the text is drawn without
    the selection when the `:` is typed, **as vim draws it** -- the one
    change here to what is seen in ordinary use.

  With it, two smaller things:
  - **Leaving INSERT redraws however it is left.** `<esc>` always did; a
    mapping (`inoremap jk <Esc>`) did not, so with `:spell` on the word just
    typed stayed unmarked until something else redrew.
  - **The editor has one name for NORMAL.** libvim reports `NORMAL_BUSY`
    after almost every key typed in NORMAL and `NORMAL` only after an
    `<esc>`; the editor copied both and nothing told them apart, except the
    tests, which each accepted either. `editor.NORMAL_BUSY` is gone. (The
    organizer's `Mode` keeps its own.)

  Checked with the binaries from before and after on a pseudo-terminal:
  `compare-drawing.py` says `SAME` for `--editor` and the TUI, and a second
  run through the cases above differs in those steps and no others. What
  hybrid injects -- `<esc>:checkstale` from INSERT, with a notice up and on
  a search line, `<esc>:q` from VISUAL -- is byte for byte the same.

  Found on the way, not changed here: a notice can outlive HELP mode.
  `:help`, then `:` and `<esc>` (or a command that draws nothing, `:noh`)
  leaves the notice on screen with the editor in NORMAL, and the keys go to
  the note under it.

- **One copy of vim, made in one place.** The editor draws from a copy of
  vim's text and cursor (`e.ss`, `e.fr`, `e.fc`), and decides whether to
  draw from vim's change tick. The copy was made in five places, each
  clamping differently -- four after the `<esc>` change above -- and the tick
  compared in four. `resyncFromVim` now does both, and reports whether the
  text changed; it is called after anything that touches vim: a key
  (`followVim`), an ex command (`forwardEx`, `runExLine`), a note read into
  the buffer (`Open`, `checkStale`). What changed with it:
  - **`followVim` has no exceptions.** A key that left vim half way through
    an operator returned before the copy, on the reasoning that an operator
    has changed nothing yet. A mapping can: with `nnoremap ,x xd`, `,x`
    deleted a character that was not drawn, and the tick that would have
    drawn it later was already spent
    (`TestMappingThatEndsInAnOperatorIsDrawn`, which fails on the old code).
  - **`:checkstale` takes its cursor from vim.** It kept a cursor of its
    own, clamped to one past the end of a row, and told vim -- which, in
    NORMAL, put it on the last character. After a reload onto a shorter note
    the cursor was drawn one column right of where the next key landed
    (`TestCheckStaleKeepsTheCursorInTheText`, likewise). Seen with the two
    binaries on a pseudo-terminal, a row shortened underneath by `sqlite3`:
    the reload differs in the cursor's column and in nothing else. Learned
    on the way: replacing a buffer's lines leaves vim's cursor where it was,
    on a row that may no longer exist, until it is set -- so the
    `vim.SetCursorPosition` there is not optional.
  - **A spelling suggestion is followed like a key.** The digit's `{n}z=`
    was sent and the copy made by hand; it goes through `followVim`, so
    vim's messages and the buffer check apply to it too.
  - **`:w` writes vim's text**, not the editor's copy of it
    (`bufferToString` is gone): what is saved is what the tick recorded
    beside it says is saved.
  - **The mode is set in fewer places**: `runExLine` says NORMAL once, when
    it starts, where three branches and `closeNote` each said it; `:` typed
    over a notice asks vim what line is open (`syncCmdline`) in place of six
    assignments.

  No other change to what is drawn: `compare-drawing.py` says `SAME`
  against the build from before both changes, and the run through the
  `<esc>` cases says `SAME` against the build between them. Tests:
  `resync_test.go`.

- **The sync integration tests pass against `localdb.Open`.** The open
  item from 2026-11-10, when foreign keys became per-connection and there
  was no Postgres on the machine to run them. Homebrew's `postgresql@17`
  is installed now: run against a throwaway server on port 54329, as
  CLAUDE.md's "Tests" describes, all 25 pass. No code changed. For scale:
  with the server the tests cover 80.2% of `internal/sync`'s statements;
  without it, which is what a plain `go test ./...` runs, 0.6%.

- **A notice and HELP mode go together.** "A notice is up" was kept twice:
  as the editor's mode, and as what the screen had drawn. They came apart
  both ways.
  - **The notice outlived the mode** (noted in the `<esc>` entry above).
    `:` typed over a notice opens a command line and leaves the notice up,
    which is wanted: `:help` lists the commands, and one is typed while
    reading it. But when that line ended in anything but another notice --
    `<esc>`, an empty Enter, Backspace over the colon, `:noh`, `:paste`, a
    refused command, a typo -- the notice stayed, with the editor in NORMAL
    and the keys going to the note under it.
  - **The mode outlived the notice**, found while fixing the first. A resize
    draws the whole editor, text over the notice, and the mode was still
    HELP: the screen showed only the note, and every key but the ones that
    scroll did nothing. vimango_hybrid resizes the editor whenever its
    window changes.

  Now the editor knows a notice is up apart from its mode (`Editor.notice`,
  set by `showNotice`, the one place a notice starts; cleared by `drawText`,
  which is what ends one). After a key, a notice that is up with the mode
  neither HELP nor a command line means the note is drawn (`redrawAfter`).
  And `Draw` in HELP mode shows the notice again -- from its top, and
  wrapped for the new width, since the box rewraps what it is shown.

  Tests, each failing on the old code: `notice_test.go` (seven ways a line
  typed over a notice can end; one notice after another draws no text
  between; `Draw` puts the notice back), and `TestResizeKeepsTheNoticeUp`
  in the root package, on the real screen. On a pseudo-terminal the two
  binaries differ at the resize, the page after it, and the `<esc>` and
  `:noh` that end a line over a notice, and nowhere else;
  `compare-drawing.py` says `SAME`.

  Found on the way, not changed: a resize clears the screen
  (`eraseScreenRedrawLines`) and `Draw` does not write the message line, so
  a command line being typed disappears until the next key, and so does
  "-- INSERT --". Confirmed for `--editor` and the TUI; now under "Open
  items".

- **Tab on the editor's command line writes only the line.** It wrote
  "tab", and after a command with an argument "dir: . base: ...": two
  debugging messages left in `ExModeKeyHandler`, sent to the aside line.
  In the TUI that is under the organizer; where the editor has the whole
  window -- vimango_hybrid -- it is the line the command is being typed on,
  so `:sy` and Tab left "tab" there. Looking at what the code did besides:
  - **It completed a file name after any command.** `:help s` and Tab put
    the first file in the working directory that starts with an s on the
    line (`:help schema`). Completion is now for a command whose arguments
    vim's own table says are file names and which vimango forwards: `:read`
    and `:source`, in any spelling, with a range. Anywhere else Tab does
    nothing.
  - **A directory could not be gone into.** `:read docs/` and Tab found
    nothing: the name was split with `filepath.Dir` and `Base`, which take
    `docs/` for a file called `docs`. A directory is now completed with its
    `/`, so the next letter typed is inside it, and a name that ends in `/`
    lists what is there. Hidden files are offered only after a dot.
  - `~/` is kept on the line as typed (vim expands it) and found through
    `$HOME`; `ioutil.ReadDir` and `user.Current` are gone.

  The line is typed into vim and read back with `syncCmdline`, in place of
  a copy set by hand. Tests, failing on the old code: `tab_test.go`. With
  the two binaries on a pseudo-terminal: `:sy`, `:help s`, `:read
  vimango.v` and `:read zzznope`, each with Tab, differ as described and
  nothing else does; `compare-drawing.py` says `SAME`.

- **A resize keeps the message line.** The one noted in the entry on
  notices. A resize clears the screen and has the editor draw itself, and
  `Draw` wrote the text, the frame and the status bar but not the message
  line: a command half typed when the window changed size was gone from the
  screen until the next key (vim still had it, and the cursor sat after
  nothing), and so was "-- INSERT --". `Draw` now ends with `drawModeLine`,
  which writes what the editor's mode keeps on that line -- the command
  line (`:`, `/`, `?`, an expression's), "-- INSERT --", the list a
  spelling suggestion is chosen from -- and leaves it alone in any other
  mode, where what was there was a report already made.

  Tests, failing on the old code: `draw_message_test.go`, and
  `TestResizeKeepsTheMessageLine` in the root package, with and without
  an organizer. With the binaries from before and after on a
  pseudo-terminal, resized on a `:` line, on a search line, in INSERT, in
  VISUAL, with `:help` up and in NORMAL: the first three differ, by the
  line now written, and the others are byte for byte the same.
  `compare-drawing.py` says `SAME`.

- **`g0`, `g^`, `gm` and `g$` go by the screen line.** The open item of
  this morning, done on both sides. `gj`, `gk`, `H`, `M` and `L` asked the
  editor where its screen lines are; these four were worked out from
  libvim's own window, 80 columns and unwrapped, and went to columns that
  have nothing to do with what is drawn.
  - **The fork** (`928659d8f`): a third callback, opt-in,
    `vimSetCursorMoveScreenColumnCallback()`. libvim gives a position and
    says which character of the screen line holding it is wanted -- its
    first, its last, or the middle one -- and is answered with a byte of
    the same line. It keeps what is vim's: `g^` goes on from the start to
    the first non-blank, `g$` leaves the column wanted at that character
    (so `j` keeps it), and `g$` with a count is `gj` after `$`, the
    callback there already was. An answer outside the line, or inside a
    multi-byte character, is brought to a character. The fork's `TODO.md`
    proposed "the first and last byte of the screen line"; that was not
    enough for **`gm`, which is half the window's width along the screen
    line and not the middle of its text**, and only the editor knows which
    byte is at a screen column (a tab is four columns here and up to eight
    in vim's count). So the middle is asked for, not derived.
  - **Here**: `ScreenLayout` has a third method, `ScreenColumn`, which the
    editor answers from `wrapRow` as it answers the other two
    (`screen_motion.go`). The end of a screen line is the last character
    drawn on it, which is the space the row was wrapped after -- where
    `$ gk` already went, and where vim's `g$` goes with `'linebreak'`.
  - **The callback is set only while there is a layout**
    (`vim.SetScreenLayout`), not once at startup like the other two. With
    it set and nobody to answer, the four would do nothing, and in the
    organizer's title they have always been `0`, `^` and `$`, which on one
    line is right. So with no note open they are libvim's own, as before.

  Tests: `apitest/motion_screen.c` in the fork (nine new cases; all 67 of
  its test programs pass), and here `screen_motion_test.go`: each motion
  on each of a row's three screen lines, a screen line that starts with
  blanks, the count, the column kept afterwards, `dg$` `dg0` `vg$d`, rows
  with a tab and with two-byte characters, and no layout. With the binary
  in `--editor` on a pseudo-terminal 60 wide, on a row of three screen
  lines: from the second, `g0` puts the cursor in the first text column,
  `g$` on the last character of that line, `gm` 27 columns in, and `2g$`
  at the end of the third. `vim/libvim_api.h` has the declaration and the
  header test's probe the enum. **`libvim.a` must be rebuilt**: an older
  one does not link (`undefined: vimSetCursorMoveScreenColumnCallback`);
  `docs/UPDATING_A_MAC.md` names the commit.

- **The scrolling commands work, and the view follows the cursor as vim's
  does.** Ctrl-D, Ctrl-U, Ctrl-F, Ctrl-Y, `zt`, `zz`, `zb` (and `z<CR>`,
  `z.`, `z-`, `<PageDown>`, `<PageUp>`) did nothing: libvim carries none of
  them out and hands each to the embedder (`vimSetScrollCallback`), and
  vimango registered no callback. All of it is on this side; the fork is
  unchanged and `libvim.a` need not be rebuilt.
  - **The binding** (`vim/screen.go`): `ScreenLayout` has a fourth method,
    `Scroll(s, count)`, set and cleared with the layout like the column
    callback, and `vim.ScrollCursorTo(line, startOfLine)` moves the cursor
    from inside it the way vim's own scrolling does -- to the column vim
    wants (`update_curswant`, `coladvance`), or to the first non-blank
    unless `'nostartofline'` (`beginline`). Those three are vim's
    internals, declared in `libvim_api.h` like `getvcol`. The sideways
    commands (`zh`, `zl`, `zs`, `ze`) are for a window that does not wrap
    and are not passed on.
  - **The view** (`internal/editor/view.go`, new). `scroll()` and
    `adjustFirstVisibleRow()` went. The view is now a row, the first on
    screen, set in one place (`setView`); `followCursor` moves it when the
    cursor leaves, and `Scroll` when vim asks. `Scroll` runs in the middle
    of the key, from vim's text and cursor as they are then
    (`copyFromVim`), so a mapping or `:normal` can move, scroll and edit in
    one go: `:execute "normal \<C-d>dd"` deletes the line Ctrl-D went to.
  - **vim 9.1's rules, not 8.1's.** The first port was of vim 8.1's
    `halfpage()` and `onepage()`, from memory of the source, and it agreed
    with the vim on this machine (9.1.1752) for every case with rows of
    one screen line and differed by a row where rows wrap: 9.1 pages and
    moves the cursor by screen lines, 8.1 by lines. A note is wrapped
    prose, and 9.1 is the vim there is to compare with, so the rules are
    9.1's, read from its `move.c` (`pagescroll`, `scroll_with_sms`,
    `get_scroll_overlap`, `scroll_cursor_halfway` with `prefer_above`).
  - **How it was checked.** `scripts/ask-vim.py` (new) types keys into
    stock vim on a pseudo-terminal, in a window the size of the test
    editor's pane, and prints the top line, the cursor's line and its
    column. Keys have to be typed one at a time: given to `:normal`, vim
    scrolls from a window it has not yet brought up to date. 400 random
    sequences of 3 to 10 motions and scrolls over five sets of rows (one
    screen line each; two; three; mixed; one to four in turn), run with
    vim's off-by-one, below, copied for the comparison: 394 the same as
    the editor, and the 6 others not the view's -- 5 a count given to
    Ctrl-U on the first line or Ctrl-D on the last, which libvim refuses
    before the editor hears of it, 1 the wanted column after `gj`, below.
    As built, of 285 sequences 277 are the same, 6 are that off-by-one and
    2 the wanted column after `$`. 229 sequences and vim's answers are now
    a test (`vimAnswers`, `view_vim_test.go`).
  - **One thing of vim's left out.** When half a page ends two or more
    screen lines into a wrapped row, vim 9.1 brings the view back to the
    start of that row and the cursor back one screen line fewer:
    `count = 1 + (w_skipcol - width1 - 1) / width2` in `scroll_with_sms` is
    one short for every row but the first. With that off-by-one copied the
    editor matched vim on all 47 cases written to find it; without it, 6
    differ by a row. It is not copied: the cursor goes as far as the view,
    which is what vim's comment says it means
    (`TestHalfPageKeepsTheCursorsPlace`).
  - **What changes for a note already open.** `G`, a search or a mark that
    lands more than about half a pane away now puts the cursor's row in
    the middle, not on the last line of the pane with nothing under it.
    The cursor's row is shown whole: `G` onto a last paragraph of ten
    screen lines used to show its first line only. `Draw` works the view
    out again, so a resize places the cursor for the new width at once and
    not at the next key. `compare-drawing.py` differs in exactly those
    steps (the resize; `G` in the TUI and what follows it).
  - **Two bugs it fixed on the way.** Going up with `k` into the second
    screen line of a wrapped row above the pane left the cursor drawn
    above the pane and the row not drawn: the old code set the offset to
    the cursor's screen line and then rounded it up to the next row. And a
    row taller than the whole pane with the cursor far into it had the
    same end; it is now shown from the screen line that keeps the cursor
    on screen (`topSkip`). 60 of 5,922 notes have a paragraph over 1,000
    characters, 3 over 2,000.
  - **Found and not fixed** (CLAUDE.md, "Open items"): after `$`, `0` then
    `j` goes to the end of the next line -- libvim's, with or without the
    editor, and there before this change (the same keys give the same
    answer at `60c0539`); after `gj` along a wrapped row, `gg` then `j`
    goes back to `gj`'s column, probably the same fault; `z+` and `z^` go
    by libvim's own window; and in a note Ctrl-E and Ctrl-B are
    `vimango.vim`'s code and bold, so of vim's scrolling keys those two
    are not available.

  Tests: `vim/scroll_test.go` (every command reaches the layout with its
  count; `ScrollCursorTo`), `internal/editor/view_test.go` (each command,
  rows that wrap, the view following the cursor, a row taller than the
  pane, a scroll inside a mapping and in VISUAL, the redraw, a resize) and
  `view_vim_test.go`. With the binary in `--editor` on a pseudo-terminal of
  40 lines, on the longest note: `zt`, `zz` and `zb` put the cursor on
  lines 1, 19 and 38 of the pane, `<PageUp>` on 38, and Ctrl-D leaves it
  on the line it was on. `BenchmarkRedrawAtBottomOfLongNote`: 84 µs.

## 2026-11-16

- **Text that is set forgets undo (libvim fork `9c14a01ae`, and the binding).**
  Found while looking for what to do next, by trying it: a note open in the
  editor, `x` and `:w`, the row changed as a sync changes it, `:checkstale`,
  then `u` — and line 1 of the old text was back over line 1 of the new, the
  buffer modified. `:x` wrote that to the database, from where the next sync
  would have sent it to every client as the newest version. In hybrid that is
  a pop-out left open across a sync, and one `u`.
  - **Why.** `vimBufferSetLines()` saves nothing for undo and leaves the
    history as it was, describing lines that are gone or have moved (entry 2
    of the fork's `TODO.md`, written 2026-11-12). The organizer had the same
    fault: a title edited, the rows read again for another context, and `u`
    gave the note now in the first row the title of the one that had been
    there, as an unsaved change.
  - **What was decided.** A reload is not an edit: `u` after it does nothing
    (Steve, 2026-11-16). The other choice, a reload that `u` undoes, brings
    back text from before the sync for `:w` to put over the synced version.
  - **The fork.** `vimUndoClear(buf)`, which is what vim's own `buf_reload()`
    does (`u_blockfree`, `u_clearall`). **The first attempt cleared inside
    `vimBufferSetLines()`, and upstream's `test_undo_save` failed**: an
    embedder can call `vimUndoSaveRegion()` before setting lines, which makes
    the set a change `u` undoes, and clearing threw away what that had just
    saved. So it is a call of its own, and an embedder that does not make it
    sees no change, which is the fork's rule anyway.
  - **Here.** `Buffer.SetLines` (`vim/vim.go`) calls it after setting the
    lines, so all four callers have it and a fifth cannot leave it out.
    vimango has no binding for `vimUndoSaveRegion()` and no use for one:
    what should be undoable is typed into vim.
  - **Checked** with the binary in `--editor` on a pseudo-terminal, on a copy
    of the database, with the keys hybrid injects: before, the row held
    `alpha one / second / third / fourth` after `:x`; after, the four lines
    the sync wrote. The process exited after `:x` both times.
  - **Not done**: marks are not adjusted either (the same `TODO.md` entry);
    CLAUDE.md, "Open items".

  Tests: the fork's `apitest/undo_redo.c`; `vim/undo_test.go`,
  `TestCheckStaleReloadForgetsUndo`, `TestRowsReadAgainForgetUndo`. Each of
  the three fails with the call taken out of `SetLines`.

- **The editor tells its host its state (`host_report.go`).** vimango_hybrid
  could learn one thing from an editor, by injecting `:q` and watching
  whether the process ended. So its export said "as last saved" whenever an
  editor was open, changed or not; a pop-out with unsaved changes looked like
  any other; and `:checkstale`'s warning, on the editor's message line, was
  seen only by someone looking at that editor.
  - **What is said.** The terminal's title, `vimango id=5 modified=1
    stale=0`: the note, `Editor.Modified`, and `Editor.Stale`, which is new —
    what `:checkstale` found and could not settle (the row changed under
    unsaved changes, or was deleted), ended by a save that goes through, a
    reload, or the row back as it was read. Key=value pairs so that one can
    be added and an older host passes over it.
  - **Who says it.** The app's event loop, before it waits for each event,
    when the report differs from the last one written; in `--editor` only.
    The editor package knows nothing of it: it answers `Modified` and
    `Stale`, as it answers `ID`.
  - **What it is not.** The "saved" signal. That is still the process ending
    after `:q` or `:x`, because the report arrives after the key that caused
    it: a host that quit on "modified=0" could do it between a keystroke and
    the report of it. Hybrid's rule against mirroring the editor's state
    (its CLAUDE.md, from a desync on 2026-07-12) was about a host guessing
    and then acting on the guess; this is the editor's word, shown.
  - **Checked** with the binary in `--editor` on a pseudo-terminal, on a copy
    of the database: one title when the note opens, one at the first `x` and
    none at the second, one at `:w`, one with `stale=1` when `:checkstale`
    follows a change to the row under an unsaved edit, one at the `:w` over
    it, and none for keys that only move, or for a reload.
  - **Seen on the way**: `x` then `u` leaves the buffer modified, since
    `isModified` goes by vim's change tick. CLAUDE.md, "Open items".

  Tests: `host_report_test.go`, `TestStaleIsWhatCheckStaleCouldNotSettle`.

- **The column `j` and `k` want (libvim fork `fd63ffc37`, `5838803e9`).**
  Three rows of `docs/VIM_COMMANDS.md`, part 3, and the first of the libvim
  items under "Open items", found 2026-11-15 by comparing with vim and put
  down as probably one fault. They were two, both the fork's, and no Go
  changed.
  - **`0$0j` went to the end of the next line** (`fd63ffc37`). Also `$^j`,
    `$ggj`, `l$hj` from column 1, and `gj` along a wrapped row, then `gg`,
    `H` or `0`, then `j`: any motion that brought the cursor back to the
    column where a motion had last left it. libvim made `update_topline()`
    a function that does nothing, since the embedder scrolls; in vim it
    begins with `check_cursor_moved()`, and `curs_columns()` counts on that
    when it marks the cursor's column as worked out. Without it the column
    of the line's end stayed "worked out" for a cursor recorded where it had
    been before the `$`, a motion back there looked like no motion, and that
    stale column became the one wanted. The function now makes the check
    and nothing else.
  - **Why it was written down as rarer than it is.** After `$` it shows only
    on a line of 80 characters or fewer: a longer one scrolls libvim's own
    window sideways, 80 columns that nobody sees, and that put the column
    right by another route. Measured: `$0j` wrong at 40, 78, 79 and 80
    characters, right at 81, 82, 90 and 125. The editor's tests use rows of
    125. A paragraph kept on one row was right; a heading, a list item, a
    line of code were not.
  - **`$` as the first key in a note wanted the end of that row only**
    (`5838803e9`): `$j` went to that column of the next row, not to its end,
    and `$gj` kept the column. A buffer just entered has a wanted column
    still to be worked out; vim does that before the next command
    (`main_loop`), libvim did it only after the command, over what `$` had
    just asked for. `vimInputCore()` now does it before the key as well.
    `TestScreenLineMotionsAfterDollar` had a `0` in front of its `$` for
    this since 2026-11-11, and many rows of `vimAnswers` begin with `l0`.
  - **Checked against vim**, not memory: `scripts/ask-vim.py` answered 31
    sequences over the table's four sets of rows, and they are in
    `vimAnswers` now. Against the library as it was 19 fail, with the first
    fix alone 6, with both none. In the fork, 60 sequences in
    `apitest/compare/cases/curswant.txt`: 21 of the first 56 differed from
    vim, none do, and the five older case files differ where they did.
  - **Checked with the binary** in `--editor` on a pseudo-terminal, on a copy
    of the database, a note of three short lines: the keys, `iX`, `:x`, and
    the row read back. Line 2 after `0$0j`, after `$^j` and after `$j` as
    the first keys was `bravo twXo x` with the old library; with the new one
    `Xbravo two x`, `Xbravo two x` and `bravo two Xx`.
  - **Learned the hard way: `go test` does not know `libvim.a` changed.**
    Two of the comparisons made on the way, old library against new, showed
    no difference at all, and were the first run's results printed twice:
    `ok (cached)`. The library is named in a linker flag and is not an
    input `go test` keeps track of. With the old one copied in,
    `go test ./vim/` said `ok (cached)` for a test that fails against it,
    and `-count=1` said `FAIL`. `go build` does link again. The full suite
    run after the first fix had `vim (cached)` in it, so it had not tested
    the new library there. Now a rule in CLAUDE.md, "Tests", and
    `docs/UPDATING_A_MAC.md` runs the suite with `-count=1`.
  - **Looked at and left.** `vim.SetCursorPosition` leaves the wanted column
    as it was, where vim's `cursor()` makes it the column given: `7l`, the
    cursor set to column 2, `j` goes to column 7. The editor's one caller,
    `:checkstale`'s reload, puts the cursor back where it was, so the column
    wanted before is the right one unless the row came back shorter; and
    `ScrollCursorTo` (`vim/screen.go`) counts on it staying.

  Tests: the fork's `apitest/normal_mode_curswant.c`; `vim/curswant_test.go`
  (`TestWantedColumnAfterDollar`, which is also what says a `libvim.a` is
  too old, since an old one links), the 31 rows of `vimAnswers`, and
  `TestScreenLineMotionsAfterDollar` without its `0`.

- **A change undone is not a modified note.** Seen earlier the same day
  while writing the host's report: `x`, `u`, and `:q` was refused, "No
  write since last change". `isModified` compared vim's change tick with
  the tick at the last save, and the tick only goes up.
  - **What it cost**, tried in the `--editor` binary on a pseudo-terminal
    over a copy of the database: `x` `u` `:q` left the process running,
    which to vimango_hybrid is "not saved" and a window kept open; `x` `u`
    `:x` wrote the row though its text had not changed, with a new
    modified time, so the note moved in every list and the next sync
    pushed it; and the host was told `modified=1`.
  - **What modified is now**: the buffer's text is not the text the
    database holds, as the editor last read or wrote it (`dbText`), found
    by comparing the two. The tick only says when to look: once for each
    value of it, and not at all while it is the tick of the save. `noteIs`
    sets `dbText` and `saveTick` together, where `Open`, `save` and
    `checkStale` each set them before.
  - **Why not ask vim**, which "Open items" proposed (`'modified'`, cleared
    by an undo that reaches the saved state; the fork has
    `vimBufferGetModified`). vim's flag is about a file it wrote, and it
    writes none here. Tried: `vimBufferSetLines()` sets it and nothing
    clears it; and with `:set nomodified` standing in for the save, `x`,
    the "save", `u` gives `&modified` 0 with the text from before the save
    in the buffer -- a clean note holding what the database does not, for
    `:q` to close on. Doing it properly is what `buf_write()` does
    (`u_unchanged()`, the save number), a new call in the fork, to get an
    answer that comparing the text gives directly. And the text's answer
    is the one the host's rule is written in: the editor closes only when
    the buffer's text is in the database. It differs from vim in one way,
    on purpose: a word deleted and typed again is a clean note.
  - **Measured** on a note of 900 rows: 41 µs to read the text from vim and
    compare it, once for a key that changes the text (a redraw is 71 µs),
    and 16 ns when asked again with nothing changed.
  - **Checked with the binary**, as above: `x` `u` `:q` ends the process and
    the row is not written; `x` `u` `:x` the same; `x` `:w` `u` `:q` stays
    open, and after Ctrl-R `:q` ends it. The titles written: `modified=`
    0, 1, 0 for the first, and 0, 1, 0, 1, 0 for the last.

  Tests: `TestUndoneChangeIsNotAModifiedNote`, `TestQuitAfterUndoCloses`,
  `TestUndoPastASaveIsAModifiedNote`,
  `TestReloadedNoteUndoneChangeIsNotModified` (`editor_save_test.go`), each
  of which fails on the tick alone; `TestEditorReportsToItsHost` for the
  report; `BenchmarkModifiedAfterAChange`.

## 2026-11-17

- **The last of the old search index.** `fts5_vimango.db` and its two
  backups were deleted on this machine; every Mac now runs a hybrid that
  does not pass `--fts-db`, so `vimango-sync` no longer accepts it (an old
  host passing it would now fail with an unknown flag). The comment in
  `common.go` about an ignored `"fts_db"` key went too: an unknown key in
  `config.json` is ignored by the decoder anyway.
- **chroma is no longer used directly.** The editor stopped highlighting with
  it on 2026-11-14; the one direct use left was `TestSplitMatchesChroma`,
  which checked `frontmatter.Split` against chroma's markdown lexer. That
  agreement stopped mattering when the editor and the preview both came to
  call `Split`, so the test's cases now check `Split` alone (`TestSplit`)
  and chroma is `// indirect` in `go.mod`. It stays in the build: glamour
  colours code blocks in the TUI's preview with it, and the `"chroma"`
  sections of `darkslz.json` and `default.json` are those colours.

## 2026-11-18

- **Ctrl-B, Ctrl-E and Ctrl-I are vim's again.** On 2026-11-17 (`e1e2a5a`)
  bold, italic and code moved to `<Leader>b`, `<Leader>i` and `<Leader>c`,
  and the old Ctrl mappings were kept in a temporary block of `vimango.vim`
  while getting used to the leader. That block is gone, with its line in
  `:help normal`, so in a note Ctrl-B pages back, Ctrl-E scrolls a row (both
  through the editor's `Scroll`, like Ctrl-F and Ctrl-Y) and Ctrl-I -- Tab,
  to a terminal -- jumps forward in the jump list.
  `TestVimrcDecorateCtrlKeysForNow` became `TestVimrcLeavesCtrlBEIToVim`,
  which sources `vimango.vim` and checks each key does vim's thing and
  leaves the text alone; it fails on all four counts with the old file.
- **The server backs itself up.** The Postgres server had never been backed
  up; the local SQLite databases were its copies. They cover losing the
  server, not a mass delete or bad edit that sync carries to every client,
  since they keep no deleted notes and no history. A systemd user timer on
  the server now runs `pg_dump -Fc` daily at about 09:30 UTC and keeps 30
  dumps in `~/backups/vimango` (cron is not installed on Arch; lingering was
  already on, so no sudo was needed). Peer authentication over the local
  socket, so no password is stored. The first dump was checked table by
  table against the live database: 6,032 tasks, 849 task keywords, 139
  keywords, 33 contexts, 19 folders. The files are in `scripts/server/`;
  `BACKUP_RESTORE.md` says how to install and check them.
- **Ctrl-C stops a command that does not end; `:cq` and `<PasteStart>` end
  nothing.** Two libvim fork commits and their vimango side.
  - **The interrupt** (fork `97d397b37`, its TODO.md entry 6). `:while 1 |
    endwhile`, `nmap Q jkQ` then `Q`, a function that never returns: libvim
    reads no keys while it runs one, so the editor froze for good -- in
    hybrid, a window to kill, losing what had not been saved. vim's
    `got_int` stops such loops, but nothing could set it, and set from
    outside it was never cleared: every later key was answered with
    Ctrl-C. The fork adds `vimInterrupt()`, which only sets it; `sm_run()`
    stops after the one key `vgetorpeek()` gives for it (dropping it in
    Normal mode, where it said "Type :qa and press <Enter>"), and each call
    that runs commands clears it and flushes the typeahead on the way in and
    out, as vim's `main_loop()` does. Here the key reader, which reads on
    while `MainLoop` is stuck in C, calls `vim.Interrupt` on a Ctrl-C while
    a command runs (`App.readerTakes`) and passes the key on otherwise;
    `vim/` counts the calls that run commands, so it knows. vim says
    nothing for a stopped `:while`, so `CaptureMessages` adds
    `Interrupted`. `keyEvents` went from 16 to 1,024 keys, so a reader with
    keys typed into a frozen editor is still reading when Ctrl-C comes.
    The fork's `apitest/compare` gives the same answers for all 2,692 cases
    as before.
  - **`:cq` and `<PasteStart>`** (fork `d57f18c3c`, entries 4 and 5).
    vimango refused a typed `:cq`, but `:execute 'cq'` or a mapping reached
    libvim's `exit(1)`: the process ended without saving, which hybrid
    would have read as a close. It now calls the quit callback as `:qa!`
    does, and vimango registers none, so it does nothing, like
    `:execute 'q'`. `<PasteStart>` in a mapping or `:normal` crashed; it is
    now a key that does nothing.

  - **Ctrl-C says nothing** (fork `126388a93`). vim answers a Ctrl-C in
    Normal mode with nothing to cancel with "Type :qa! and press <Enter>
    to abandon all changes and exit Vim"; in vimango `:qa!` closes the
    note, so it told whoever pressed Ctrl-C that their changes were about
    to go. The fork's `nv_esc()` no longer says it. With nothing to avoid,
    the CTRL-C an interrupt leaves in Normal mode goes to vim as any key
    would, and the fork's first version of the interrupt, which dropped it,
    lost that special case.

  Tests: `TestCtrlCInterruptsACommandThatNeverEnds` (`event_loop_test.go`),
  `vim/interrupt_test.go`, Ctrl-C among the quiet keys of
  `TestVimMessagesForKeysAreShown`; in the fork `apitest/interrupt.c`,
  `apitest/paste_start.c`, `test_cquit` in `apitest/quit.c` and
  `test_ctrl_c_says_nothing` in `apitest/messages_reports.c`.

## 2026-11-20

- **The recovery file: no way of ending loses unsaved text.** Until now a
  note's unsaved text lived only in vim's buffer. A panic, a crash in
  libvim, a kill, or hybrid itself going away (its pty closing sends
  SIGHUP) took it with the process -- and hybrid could not tell any of
  those from `:q`: GhosttyTerminal's close callback carries no exit
  status, so the window went back to the preview as if the note had been
  closed, the panic's message vanished with the terminal, and nothing was
  said. Now the editor keeps `recovery/<id>.<pid>.json` while its note is
  open (`internal/editor/recovery.go`): written when the note opens, after
  a save, and a second after an edit (the event loop's timer), with the
  buffer's text whenever it differs from the database's; removed by
  `closeNote` and by nothing else. The next editor on the note takes the
  newest file a dead process left, with a notice (`:w` keeps, `:q!`
  drops), stale if a sync has changed the note since. A panic in
  `MainLoop` writes the file and `recovery/crash.<pid>.log` and exits 2;
  SIGHUP, SIGTERM and SIGINT have `MainLoop` write the file before the
  process dies (`crash.go`). The host report gains `pid=`, so hybrid
  finds its editor's file after the exit and says the editor stopped.
  - **Why the file and not a "closed" pair in the title**, which was the
    first plan: the title is written to the pty just before the exit, and
    nothing orders the terminal reading it before it notices the process
    has ended. The file is removed before the exit, so its absence is
    certain by the time anyone looks.
  - Checked end to end with the built binary on a scratch copy of the
    database: an edit, `kill -9` a second later, the file held it;
    reopened, the text came back with the notice; an edit and SIGHUP at
    once, kept; `:x` saved it and the file went.
  - Tests: `internal/editor/recovery_test.go`, `crash_test.go`, and the
    `pid` in `host_report_test.go`.

- **Keys in INSERT that typed a control character into the note**
  (fork `8cca54f72`). Found by the review of this day: Ctrl-N put an
  invisible 0x0E in the text, saved and synced with it. Stock vim 9.1 was
  asked what each Ctrl key does in INSERT; libvim differed on five, all by
  inserting the key: Ctrl-N and Ctrl-P (completion), Ctrl-X (its submode),
  Ctrl-K (digraphs) and Ctrl-\ Ctrl-N. (^B, ^F, ^L, ^S, ^Z and ^_ vim
  inserts too, and they are left as vim has them.) Digraphs are compiled in
  again, with Ctrl-K reading its two characters a call at a time in INSERT
  and on the `:` line, as Ctrl-R reads its register; Ctrl-\ Ctrl-N, Ctrl-G
  and Ctrl-O leave INSERT. Completion is a few thousand lines of vim the
  fork no longer has, and would need a menu drawn by the editor: Ctrl-N,
  Ctrl-P, Ctrl-X and the Ctrl-X command after it insert nothing and say
  `Completion is not available`. The fork's comparison with vim: 38 INSERT
  cases differ (58 on 2026-11-12), the rest of them completion. Tests: the
  fork's `apitest/digraph.c` and `insert_ctrl_bsl.c`; here
  `internal/editor/insert_keys_test.go`.

- **Control characters in a note are drawn as ^X.** Steve, trying the
  digraphs: Ctrl-K, a key or two, then Ctrl-K again "deletes the lines
  above", and `u` brings them back. Nothing was deleted: Ctrl-K a Ctrl-K
  is no digraph, so vim inserts its second key, a ^K (stock vim does the
  same, and shows `^K`), and the editor wrote the ^K to the terminal as it
  was -- a vertical tab, which moves the cursor down a line -- so what was
  drawn after it was drawn in the wrong place; `u` took the ^K out and the
  redraw was right again. Any control character already in a note (a ^N
  from before the fork's `8cca54f72`, say) did the same. The layout now has
  one place, `expandRow` (`editor_wrap.go`): a tab is four columns, a
  control character two, `^` and its letter in a colour of its own, and the
  seven places that expanded tabs each by hand -- drawing, the cursor, the
  selection and search highlights, wrapping, `gj`/`g$` -- use it.
  `scripts/compare-drawing.py` (which now masks the report's `pid=`) found
  nothing else drawn differently, in either mode. Tests:
  `internal/editor/control_chars_test.go`.

- **Listings fit the pane** (`'columns'`, the open item from 2026-11-11).
  Steve, trying `:digraphs`: the formatting is not great. libvim believed
  in 80 columns whatever the pane, so vim put six digraphs to a line and
  `:registers` cut its lines at 74 characters; a wide window was mostly
  empty, a narrow one wrapped every line. The overlay now says how wide a
  listing may be in it (`Screen.ListingWidth`: the pane less its padding and
  the 4 columns the glamour style indents a code block by, measured from the
  style), and the editor sets vim's `'columns'` to that (`fitListings`)
  when a note opens and before each forwarded command -- only then: setting
  it runs a command, which drops the keys vim holds for a mapping.
  `vimWindowSetWidth` was not used: it can only widen vim's screen. What is
  left is vim's own: entries padded by vim's count of a character's width,
  which drifts where the terminal draws a wide or combining one otherwise.
  Tests: `internal/editor/listing_width_test.go`, `notice_listing_test.go`.

## 2026-11-21

- **libvim run instrumented: nothing found** (`scripts/test-sanitized.sh`, the
  first item left by the review of 2026-11-20). The library vimango links is
  not instrumented, so a bad read inside vim showed only if it crashed, and
  the fork's own tests instrument the test file and not the library. The
  script builds libvim with AddressSanitizer, or with
  UndefinedBehaviorSanitizer, from a copy of the fork's checkout and runs
  against it vimango's suite (in a copy of this tree, since the linker takes
  `libvim.a` from the directory `go` is run in), the fork's 70 API test
  programs, and its nine case files: 2,626 key sequences, ex commands and
  vimscript functions, a process each. On fork `8cca54f72`:
  - **AddressSanitizer: no report** from any of them.
  - **UndefinedBehaviorSanitizer: no report** either, where the fork's notes
    expected vim's older code to have a good deal to say. (It is not silent
    about everything: `:echo 9223372036854775807 + 1` is a signed overflow,
    as in every vim, and no case does that.)
  - **`api_edges.exe`**, the API called wrongly, gives three, all in the
    fork's `TODO.md` already: a key typed in Insert mode after lines were set
    under a cursor now past the end (entry 2: a read past the buffer's block,
    in `ins_char_bytes`), a negative first line (entry 2), and
    `vimSearchGetHighlights` with no buffer (entry 14). vimango makes none of
    those calls: a caller of `SetLines` that can shorten the text sets the
    cursor next (`checkStale`, the organizer's `setRows`). That is a rule
    the callers keep, and the open item says so.
  - Each checker was shown to be awake first: a call that makes libvim read
    one pointer past a malloc'd array, and the overflow above, from a Go
    test, each wrote its report and failed the script.
  - Go's `-asan` is "not supported on darwin/arm64" (go 1.27); linking the
    runtime with `CGO_LDFLAGS=-fsanitize=address` is enough, since libvim is
    the code to check. Not run on Linux yet.

- **`SetLines` looks after the cursor and the marks** (fork `7f2ad2af7`; its
  `TODO.md` entry 2, the one memory error the instrumented run knew of).
  libvim's `vimBufferSetLines()` appended and deleted lines and looked at
  nothing else: a cursor on a row that was gone gave `E315` at the next
  key, and in INSERT read past the buffer's memory; `'a` after a note was
  reloaded shorter gave `E19: Mark has invalid line number`. vimango got
  away with it because each caller put the cursor back at once. Now the
  cursor and the marks go with their lines, as for vim's `setbufline()`
  and `deletebufline()`: a row that is replaced keeps what is on it, rows
  below the range move with theirs, a mark on a row that went is not set
  (`E20`), and the cursor is brought inside the text. So a mark set in a
  note is still on its row after a sync reloads the note.
  - **A stress run found two more holders of stale positions**, and they
    are fixed in the same commit. The fork's new `setlines_stress` types
    random keys and sets random lines in between, a seed to a process,
    under AddressSanitizer. `d` Ctrl-V, lines set, then a motion: the
    operator kept the position it was typed at, and read past the end of a
    row that had become shorter. A command half typed in NORMAL is now
    forgotten when lines are set. And Ctrl-V `jj` `I`, lines set, `<esc>`:
    the insert was copied to the block's other rows by their old numbers,
    `E315` for each one that was gone. It is no longer copied. 5,000 seeds
    of 400 steps pass, and `scripts/test-sanitized.sh` runs 1,000.
  - **And one thing that is not about lines being set**: Ctrl-V Ctrl-O in
    INSERT, then `.`, then `<esc>` and `.` never returns, in libvim with no
    line set at all; vim inserts the two characters again. Not fixed; in
    the open items and the fork's `TODO.md` (entry 4).
  - **One behavior changes**: a line put in above the cursor moves the
    cursor down with its text, as vim's `append()` does. `TestCommandLineAPI`
    relied on the opposite, by putting its one line above the empty
    buffer's instead of over it.
  - `checkStale` still sets the cursor after the reload, for a `libvim.a`
    from before this commit, which links without complaint.
  - Tests: `vim/setlines_test.go`, `TestCheckStaleReloadKeepsMarks`; in
    the fork `apitest/buffer_set_lines.c` and `apitest/compare/setlines_stress.c`.
    Its comparison with vim gives the same numbers as before for every case
    file.

- **`:%s/old/new/gc` works: vim asks about each match, and the editor
  answers** (fork `44d4a6c10`). It was the one everyday vim command the
  editor refused: vim shows each match and waits for a key, and libvim has
  no window and no keyboard, so upstream turned the flag down with an error
  and vimango refused it first. Three ways were tried in a scratch copy
  before any was chosen:
  - *The editor walks the matches* and has vim change one at a time. Not
    robust: a pattern cannot be pinned to one match. `\%9cfoo\|bar`, meant
    for the `foo` at column 9, changed a `bar`, the prefix belonging to the
    first branch only; `\%(…\)` round the pattern fixes that and fails
    (`E53`) when the pattern has `\v` in it. It would also have been a Go
    copy of how `:s` reads its arguments, and typed commands only.
  - *libvim's `do_sub()` as a state that returns for each key*, which is
    what upstream meant to do. Not tried, on reading it: whatever ran the
    command — the rest of a line after `|`, `:g`, a function, a mapping —
    goes on when `do_sub()` returns, before the answers, so it could only
    ever be a `:s` typed by itself; and the loop's thirty-odd locals, the
    replacement (which points into the command line) and the caller's
    `eap` would have to outlive the call.
  - *vim's loop, asking a callback where it read the key.* Two lines
    changed in the scratch copy and twelve cases behaved as in vim. Chosen.
    Its cost is that a call into libvim now waits for the user, which
    nothing did: the fork's rule was that `vimInput()` must return. Steve
    agreed to that on the day.

  So libvim calls `vimSetSubstituteConfirmCallback()`'s callback with each
  match; the binding (`vim.SetSubstituteConfirm`) hands it to the editor
  (`confirm.go`), which draws what vim would — the changes accepted so far,
  the match marked, `replace with X (y/n/a/q/l)?` — and asks its host for
  the next key (`Host.WaitKey`); the app's `waitKey` is the event loop's
  own wait again, on `MainLoop`'s goroutine, with the resize, the recovery
  file and the signals still served.
  - **Checked against vim.** Thirteen key sequences (answers in every
    order, a replacement that lengthens the line, `\r`, `:g`, `:&&`) leave
    the cursor and the text exactly as vim 9.1 does. The fork's
    `compare.sh` could not say so: its vim runs in silent Ex mode, where
    `:s///c` takes a *line* for each answer, so every case with more than
    one answer differs there. Run with `--not-a-term` instead of `-es`
    they agree.
  - **Keys already typed answer first**, as in vim: the rest of a mapping
    or of a `:normal`. Inside `:normal` they are the only answer.
  - **Ctrl-C at the question is an answer**, not an interrupt: the key
    reader used to take every Ctrl-C typed while a call into libvim was in
    progress, and one typed at the question would have set vim's interrupt
    flag and never reached the wait. `vim.Interrupt` now says nothing is
    running while a question is asked.
  - **The recovery file is kept during a question**, which can stand for as
    long as the user likes: the timer moved from a local of `handleEvents`
    to the `App` so that both waits serve it.
  - The terminal's cursor waits after the question, as in vim, not on the
    match.
  - Left out: Ctrl-E and Ctrl-Y at the question (vim scrolls), and
    recording an answer into a register with `q`. `input()` is still not
    asked, but the way to ask now exists.
  - Tests: the fork's `apitest/substitute_confirm.c` (14, under
    AddressSanitizer); `vim/substitute_test.go`; `confirm_test.go` in the
    editor; and through the event loop,
    `TestEventLoopAnswersSubstituteConfirm`,
    `TestCtrlCAnswersSubstituteConfirm` and
    `TestRecoveryFileIsKeptWhileSubstituteAsks`. Not tried in
    vimango_hybrid before it was committed.
  - **The host is told of changes while a question stands** (the commit
    after): the report to the host was written only by `handleEvents`, so
    hybrid's mark for unsaved changes waited for the whole command to end.
    `waitKey` writes it too. Found when Steve asked what the wait cost.
    Test: `TestHostIsToldOfChangesWhileSubstituteAsks`.
  - **Nothing can be sent to vim while it asks** (the commit after that).
    That the editor only reads from vim during a question was a rule kept
    by hand: its drawing, and a resize's, run inside vim's substitute loop,
    and one `vim.ExecuteCommand` added to either would have run there. Now
    the binding refuses: keys, commands, expressions, a capture of
    messages, lines set, the current buffer and the cursor changed all do
    nothing while a question is asked, and the editor says
    `vimango note: refused … while vim was asking` when the command ends
    (worded "vimango bug" at first; Steve found that odd on a message
    line, and it was changed the same day).
    Steve asked for it after hearing that this was the weakest part. The
    test that draws during a question with spell checking on was shown to
    fail when the editor's one such call (`syncSpellOption`) was put back.

## 2026-11-22

- **`--render-markdown <id>`: a note's markdown with its Drive images
  inlined.** For vimango_hybrid's slides, which are made by marp-cli: Marp
  cannot read `gdrive:ID`, and nothing of Drive — the token, the download,
  the cache, HEIC — can be given to it. It did not have to be.
  `inlineImages` was already a pass from markdown to markdown, run before
  goldmark, so the new mode prints what that pass returns: the note as
  stored, with `![alt](gdrive:ID)` become `![alt](data:image/…)`. The alt
  text is untouched, which is what lets Marp's image keywords
  (`![bg right:40%](gdrive:…)`) work on a Drive image. Tried by hand first,
  with entries from `./image_cache` and marp-cli 4.5.1: an image in the
  flow, a sized one and a split background all rendered; markdown-it
  accepts `data:image/` gif, png, jpeg and webp, which is all this writes.
  The placeholder became an argument of `inlineImages`: HTML for the page
  as before, plain text for markdown, because Marp escapes raw HTML and
  reads an HTML comment as presenter notes. `renderNote` and the new
  `renderNoteMarkdown` share `driveForRender`. Measured on a real note
  with two cached images: 33ms. Tests: `TestRenderMarkdown`,
  `TestRenderMarkdownFailures`, `TestRenderFlagsNameTheirNote`.
  Not done: images are still cached at `image_cache_max_width` (800px), so
  one used as a full slide background is scaled up; the cache is keyed by
  URL, so a second size would be a second key.
- **Frontmatter takes block scalars.** A Marp deck that styles itself has
  `style: |` in its frontmatter with CSS on the lines below, and `Parse`
  declined a block scalar, as it declines whatever it has no display for:
  the preview showed such a deck's frontmatter as a rule, a paragraph of
  CSS and an h2, and the editor highlighted it as markdown. Steve met it
  with the first deck that set a layout. Now the lines under `key: |` or
  `key: >` (with `-` or `+`) are the key's value (`blockScalar`):
  those indented further than the key, less the indentation of the first,
  blank lines at the ends dropped; a `>` one folded the way YAML folds.
  `Entry.Literal` marks a `|` one, and the card shows it as written, in
  the code font (`dd.fm-literal`). Still declined: an indentation digit
  (`|2`), a comment after the mark, and a line that steps back to
  between the key's indentation and the block's. The editor needed no
  change: it asks `frontmatter.Rows`, and draws those rows plain. Tests:
  `TestParseBlockScalars`, `TestFrontmatterHTMLBlockScalars`,
  `TestRenderNoteAsHTMLDeckFrontmatter`, and cases in
  `TestRowsAgreesWithSplitAndParse` and `TestFrontmatterIsPlain`.
