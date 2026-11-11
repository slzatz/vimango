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

