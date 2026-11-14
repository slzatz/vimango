# The editor as a package: what is left

Written 2026-11-13, at the end of the session that did steps 1 to 3;
brought up to date 2026-11-14, when steps 4a and 4b were done. This is the
plan for what remains: the move itself, 4c. When a part of it lands, record
it in [HISTORY.md](HISTORY.md), and delete this file when the move is done.

## Why

vimango is three programs in one `package main`: the TUI's organizer (about
5,500 lines, for emergencies and ssh), the editor (about 3,900, which
vimango_hybrid runs every day as `--editor`), and `--render-html` (about
1,600, hybrid only, already separate: it builds no `App`). The editor is where
most changes land, and until this week nothing kept it from reaching into the
TUI around it. That is how `:sync` typed in the TUI's editor came to lose
unsaved edits and to run a real sync for a dry run (HISTORY, 2026-11-13).

The aim is an editor that is given what it needs by whoever opens it, and
then a package boundary so that the compiler keeps it that way.

## Done (all on `dev`)

| Commit | What |
|---|---|
| `ab8e12d` | The editor refuses `:sync`; it never hands over to the organizer with a note open |
| `2b534d5` | Closing goes through `Editor.onClose`; `Session.editorMode` is gone, `Session.editor != nil` is the one answer |
| `dc1a53e` | The editor is given its layout (`editorLayout`, from `Screen.editorLayout()`); its files no longer mention `editorOnly` |
| `f75d084` | Misspellings are marked on the last line of the TUI's pane (found on the way) |
| `7a9817e` | 4a: the editor holds no `Session` and no `Screen` -- it is given the markdown style and the notice box (`noticeBox`, two methods), and draws itself after `:checkstale` (`Editor.draw`) |
| `1a4794e` | 4a: `editorHost`, `newEditor`, `openNote`, `editorCommands` -- the constructors are the editor's, not methods of `App` |
| `7e0f5a5` | 4a: `paste.go` split: the editor's half is `editor_paste.go` |

| `d541698` | Drawing the text leaves no colour set (found on the way: the line numbers' grey) |
| `3fc9906` | 4b: `internal/atomicfile` |
| `702719f` | 4b: `internal/pane` -- the command registry, `ClipColumns`, `RuneStart`, `InclusiveEnd`, `SendToVim`, `IsSyntheticKey`, `VimKeyName`, `PastedText`, `OnOneLine` |
| `04524aa` | 4b: what is the editor's alone is in its files -- `ShowMessage`/`ShowAside`, `editor_wrap.go`, `lineNumberCols`, `sgrReset` |

An editor is now made from an `editorHost` (the store, the notice box, the
markdown style, `onClose`, the `:sync` answer) and an `editorLayout`, and
holds nothing else of the program around it.
`TestEditorIsMadeFromItsHost` makes one with no `App`, `Screen` or `Session`.

## What the editor still takes from outside

Found by compiling the editor's files alone in a throwaway package and
reading what was undefined. "The editor's files" are `editor.go`,
`editor_methods.go`, `editor_process_key.go`, `editor_cmd_line.go`,
`editor_rows.go`, `editor_paste.go`, `editor_wrap.go`, `ex_forward.go`,
`spellcheck.go`, `searchhl.go`, `screen_motion.go`, `highlight.go`, and the
two that set vim up at startup, `registers.go` and `vimrc.go`.

```bash
mkdir zz && for f in editor*.go ex_forward.go spellcheck.go searchhl.go \
    screen_motion.go highlight.go registers.go vimrc.go fts5_guard.go; do
  case $f in *_test.go) continue;; esac
  sed 's/^package main/package zz/' $f > zz/$f; done
CGO_ENABLED=1 go build --tags=fts5 -gcflags=-e ./zz/ 2>&1 |
  grep -o "undefined: [A-Za-z_0-9.]*" | sort | uniq -c | sort -rn; rm -rf zz
```

After 4b (2026-11-14) that prints two things only:

- **The mode names** -- `Mode`, `NORMAL`, `INSERT`, `VISUAL`, `VISUAL_BLOCK`,
  `EX_COMMAND`, `SEARCH`, `SPELLING`, `HELP`, `PENDING`, `OTHER`, `modeMap`,
  `visualModeMap` -- which are in `common.go`, in one type with the
  organizer's (`COMMAND_LINE`, `NAVIGATE_NOTICE`, `CONTAINER`, `LINKS`).
  Decided with Steve: the editor gets a `Mode` of its own. It is done *with*
  the move, not before: in a package of its own the editor can keep these
  names, so its files need no renaming, where two types in `package main`
  could not share them.
- `END_KEY`, once: the organizer's name for `terminal.KeyEnd`, which the
  editor can use directly.

### The move itself (step 4c)

- The editor's files and their tests go to `internal/editor`. The tests are
  most of the root package's: `keypath`, `macro_keypath`, `mapping`, `ctrlr`,
  `paste` (the editor's half), `ex_forward`, `ex_hang`, `searchhl`, `search`,
  `spellcheck`, `screen_motion`, `drawvisual`, `wraprow`, `highlight`,
  `editor_rows`, `editor_save`, `editor_layout`, `editor_host`, `vimrc`,
  `registers`, and parts of `clip`, `session` and `editor_only`, which test
  the app with an editor in it and stay. The test helpers reach the screen
  behind a test's editor with `screenOf(e)` (`e.notices.(*Screen)`); the
  tests that move need a notice box of their own (`recordedNotices`, in
  `editor_host_test.go`, is one).
- The notice box's two methods and `editorHost`'s fields become exported, so
  `Screen.showNotice` and `Screen.noticeKey` are renamed with them.
- What becomes exported is small: the editor type, its constructor, its
  layout, and what `MainLoop` calls (`editorProcessKey`, `pasteText`,
  `drawAfterKey`, `editorMappingTimeout`, `cursor`, `ShowMessage`), plus the
  startup calls `main` makes (`initSpell`, `sourceVimrc`, `restoreRegisters`,
  `saveRegisters`) and `mappingWait`.
- `spellOn` and `spellAvailable` are package variables. They can stay so:
  there is one vim per process.
- `TestForwardedCommandsDoNotHang` starts a process per case; check that it
  still finds its own test binary from the new package.
- `CLAUDE.md` names files throughout. It needs a pass.

## How to check each commit

- `CGO_ENABLED=1 go test --tags=fts5 ./...`
- **`scripts/compare-drawing.py before after`** runs two builds through the
  same keys on a pseudo-terminal, in `--editor` and in the TUI, on a scratch
  copy of the database, and compares everything written. For 4b and 4c the
  answer should be `SAME`, as it was for 4a: none of them is meant to change
  a byte. (`:checkstale` is not in it. For 4a, which changed what a reload
  sends, the screens the two builds left were compared with a terminal
  emulator; HISTORY, 2026-11-14.) Build the "before" binary
  first: `CGO_ENABLED=1 go build --tags=fts5 -o /tmp/vimango.before .`
- Steve's own pass in hybrid and the TUI before each push, as always.

## After the move, in the order I would do them

1. **One resync from vim.** vim's buffer and cursor are copied into the
   editor in five places, each clamping differently: the Esc branch of
   `processKey` (no clamping at all), the end of `followVim`,
   `resyncFromVim`, `SpellingModeKeyHandler`, and `checkStale`. `e.mode` is
   assigned in about twenty. Most of the fixes of 2026-11-11 were a copy that
   had gone stale. One function, called after anything that touches vim.
2. **Esc goes round `followVim`.** `processKey` sends `<esc>` itself and
   returns: vim's messages for it are not captured, and nothing checks that
   vim is still on the note's buffer. It should take the same path as every
   other key.
3. **The TUI's weight** is Steve's call, not a refactor: about 2,000 of the
   organizer's lines show images in a terminal that is used for emergencies.
4. `--render-html` to Swift is already an open item in `CLAUDE.md`.
