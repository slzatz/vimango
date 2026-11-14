# The editor as a package: what is left

Written 2026-11-13, at the end of the session that did steps 1 to 3. This is
the plan for the next one. When a part of it lands, record it in
[HISTORY.md](HISTORY.md), and delete this file when the move is done.

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

The editor's files now use neither the global `app` nor `Session.editorOnly`.

## What the editor still takes from outside

Found by compiling the editor's files alone in a throwaway package and
reading what was undefined. "The editor's files" are `editor.go`,
`editor_methods.go`, `editor_process_key.go`, `editor_cmd_line.go`,
`editor_rows.go`, `ex_forward.go`, `spellcheck.go`, `searchhl.go`,
`screen_motion.go`, `highlight.go`, the editor's half of `paste.go`, and the
two that set vim up at startup, `registers.go` and `vimrc.go`.

### From the host (to remove, step 4a)

| What | Where | What to do |
|---|---|---|
| `e.Session.markdown_style` | `editor_rows.go:174` | Give the editor the style when it is made. It is the only use of `Editor.Session`; the field goes with it |
| `e.Screen.eraseRightScreen()`, `drawRightScreen()` | `checkStale`, `editor_cmd_line.go` | The editor redraws itself (`drawText`, `drawFrame`, `drawStatusBar`); `drawText` already clears every line it draws |
| `e.Screen.showNotice`, `e.Screen.noticeKey` | `:help`, listings, HELP mode's keys | The notice box stays the screen's. Give the editor a two-method interface for it |
| `getGlamourStylePath()` reads `app.Config` | `notice.go`, on the editor's `:help` path | Give the screen the style path at startup; `Screen` then needs no global |
| `(*App).NewEditor`, `(*App).setEditorExCmds` | `app.go`, `editor_cmd_line.go` | Neither uses the `App` beyond `Store`, `Screen`, `Session`, `noteClosed`, `syncAnswer`. Make them the editor's constructor, taking those |
| `(*App).openEditor` | `app.go` | Split: reading the note and making the buffer is the editor's; `Session.editor = ae`, `positionEditor` and the first draw are the host's |
| `paste.go` | both `Editor.pasteText` and `Organizer.pasteText` | Split the file in two |

After 4a `Editor` holds no `Session` and no `Screen`, and what it needs is one
value: store, layout, `onClose`, `syncAnswer`, the markdown style, the notice
box.

### Shared with the organizer (to give a home, step 4b)

These are used by the editor and by code that stays in `package main`, so
they have to live somewhere both can import.

| What | Now in | Also used by |
|---|---|---|
| `Mode` and its constants, `modeMap`, `visualModeMap` | `common.go` | the organizer (its own modes are in the same type: `COMMAND_LINE`, `NAVIGATE_NOTICE`, `CONTAINER`, `LINKS`) |
| `termcodes`, `sendToVim`, `isSyntheticKey`, the key names (`ARROW_UP`, `END_KEY`, …) | `common.go` | the organizer's title editing |
| `clipColumns` | `organizer_display.go` | the organizer |
| `wrapRow`, `wrapSegment`, `expandedOffset`, `runeStart`, `inclusiveEnd`, `byteAtDisplayCol`, `byteAfterDisplayCol`, `tabWidth` | `word_wrap.go` | `runeStart` and `inclusiveEnd` by the organizer; the rest are the editor's alone and move with it |
| `CommandRegistry`, `CommandInfo` | `command_registry.go` | the organizer |
| `writeFileAtomic` | `atomicfile.go` | preferences, the image cache |
| `Location` (`BL`, `BR`), `RESET`, `RED_BG`, `LEFT_MARGIN_OFFSET` | `common.go`, `ansi_colors.go` | the organizer |

**To decide first, with Steve:** one small shared package or several. My
recommendation is two: `internal/atomicfile` (it has nothing to do with the
rest), and one package for text on a terminal that both panes use — the
mode and key names, `sendToVim`, `clipColumns`, the two rune helpers, the
command registry. `Location` need not be shared: the editor has exactly two
places to write, so `ShowMessage(BR, …)` and `ShowMessage(BL, …)` can become
two methods and the type stays the organizer's. The editor could also have a
`Mode` of its own, with only its modes; that is cleaner and costs a second
six-line `modeMap`.

### The move itself (step 4c)

- The editor's files and their tests go to `internal/editor`. The tests are
  most of the root package's: `keypath`, `macro_keypath`, `mapping`, `ctrlr`,
  `paste` (the editor's half), `ex_forward`, `ex_hang`, `searchhl`, `search`,
  `spellcheck`, `screen_motion`, `drawvisual`, `wraprow`, `highlight`,
  `editor_rows`, `editor_save`, `editor_layout`, `vimrc`, `registers`, and
  parts of `clip`, `session` and `editor_only`, which test the app with an
  editor in it and stay.
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
  copy of the database, and compares everything written. For 4a (apart from
  `:checkstale`, which it does not exercise), 4b and 4c the answer should be
  `SAME`: none of them is meant to change a byte. Build the "before" binary
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
