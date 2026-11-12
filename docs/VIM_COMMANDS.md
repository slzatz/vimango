# Vim commands in the editor: what to try

A list for hands-on testing, in the vimango TUI and in vimango_hybrid's
pop-out editor (the same binary, `vimango --editor`). Three parts:

1. **What works** — vim's own commands, forwarded to libvim.
2. **What is blocked on purpose**, and the message you should get.
3. **What is still missing or wrong.**

It is not exhaustive. 138 of vim's 558 ex commands are forwarded; the
authoritative list is `exAllowed` in `ex_forward.go`, and the reasons for the
rest are `exReasons` in the same file. Written 2026-11-11, after the libvim
work of that day (multi-key mappings, listings, typed search, Ctrl-R, no more
freezes); `docs/HISTORY.md` has the account of each.

## Before you start

- **`libvim.a` must be current.** Several of these changes link against an old
  library without complaint and are simply missing — or, for `"=` and
  `:echo input('x')`, freeze the editor. Check:

  ```bash
  git -C ~/libvim merge-base --is-ancestor 76bc49547 HEAD && echo "libvim is new enough"
  ```

  If not: `./scripts/build-libvim.sh && CGO_ENABLED=1 go build --tags=fts5`.
- **Use a scratch note.** The expected results below are for a note holding
  exactly these five lines, with the cursor on line 1:

  ```
  alpha one
  bravo two
  charlie three
  delta four
  echo five
  ```

  `u` after each command puts it back.
- **Where output goes.** One line of output goes on the message line. More
  than that opens the overlay `:help` uses: Space / `b` page it, Esc closes it.
- **hybrid** drives the same editor. Two differences: `:sync` answers "sync is
  handled by the app — use the Sync menu", and `:q` / `:x` end the pop-out
  (the process exiting is how hybrid knows the note was saved).

## 1. What works

### Editing

| Try | Expect |
|---|---|
| `:s/alpha/ALPHA/` | line 1 changes; nothing is said for one line |
| `:%s/o/0/g` | `5 substitutions on 4 lines` |
| `:g/two/d` and `:v/a/d` | the matching (or non-matching) lines go |
| `:2,4d` | `3 fewer lines` |
| `:2m0`, `:1t$`, `:1,3j` | move line 2 to the top; copy line 1 to the end; join three lines |
| `:2,3>` | lines 2–3 indented |
| `:%sort!` / `:sort u` | reverse sort; sort and drop duplicates |
| `:%normal A;` | a `;` on the end of every line |
| `:2,3normal Ix` | an `x` at the start of lines 2 and 3 |
| `:u`, `:red`, `:earlier 10s`, `:later 10s` | undo, redo, and by time |
| `:retab`, `:center`, `:left`, `:right` | as in vim |
| `V` then `:` | the line starts with `'<,'>`; any command above takes it |

### Moving and marking

| Try | Expect |
|---|---|
| `:3`, `:$`, `:42` | the cursor goes to that line (the last, if there are fewer) |
| `:ka` or `:mark a`, then `:'a` | set a mark, return to it |
| `:delmarks a` | the mark is gone from `:marks` |
| `:noh` | search highlighting off until the next search |
| `gj`, `gk` on a row that wraps | down or up one screen line, in the same screen column; `3gj` three. A shorter screen line in between does not lose the column. After `$`, the end of each screen line. `dgj` deletes to there (before 2026-11-11 these did nothing) |
| `H`, `M`, `L` | the first, middle and last row on screen, counting only rows shown whole; `3H` the third from the top |

### Listings (silent before 2026-11-11)

| Try | Expect |
|---|---|
| `yy` then `:registers` (or `:reg a`, `:display`) | overlay: `--- Registers ---` and a line per register |
| `:marks`, `:jumps`, `:changes` | overlay |
| `:history`, `:history /` | overlay: the commands, or searches, typed so far |
| `:messages` | what vim has said; `:messages clear` empties it |
| `:version` | overlay: `VIM - Vi IMproved 8.1` and the feature list |
| `:map`, `:nmap`, `:imap`, `:abbreviate`, `:command` | what is defined, or `No mapping found` and the like |
| `:1print` / `:1,3print` | line 1 on the message line / three lines in the overlay |
| `:2,3#` and `:1list` | lines with numbers; a line with `$` at its end |
| `:g/a/p` | every line with an `a` |
| `:=` / `:3=` | `5` / `3` |
| `:ascii` (cursor on `a`) | `<a>  97,  Hex 61,  Octal 141` |
| `:set` / `:set ts?` | overlay of options that differ from the default / `tabstop=8` |
| `:let` / `:let g:x` (after `:let g:x = 5`) | overlay of every variable / `g:x  #5` — these used to freeze the editor |
| `:pwd`, `:ls`, `:undolist`, `:autocmd`, `:scriptnames` | as in vim |

`:2,3number` is **not** vim's: `:number` is vimango's own command and toggles
line numbers. Use `:2,3#`.

### Settings, expressions, script

| Try | Expect |
|---|---|
| `:set ignorecase`, `:set nohlsearch`, `:set ts=4` | as in vim |
| `:let g:x = 5 \| echo g:x * 2` | `10` |
| `:echo strftime('%F')` | today's date |
| `:echo 'a' \| echo 'b'` | `a; b` |
| `:echomsg 'kept'` then `:messages` | `kept`, and it is in the list |
| `:call setline(1, 'set by call')` | line 1 replaced |
| `:execute 'normal ggdd'` | line 1 deleted |
| `:silent s/alpha/A/`, `:keeppatterns s/alpha/A/`, `:verbose set ts?` | the modifier is accepted |
| `:zzz` | `E492: Not an editor command: zzz` — vim answers for itself |

### Mappings, abbreviations, commands

Typed at the `:` line they last until the editor closes; put them in
`vimango.vim` to keep them.

| Try | Expect |
|---|---|
| `:nnoremap ,d dd` then `,d` | the line is deleted. `,` alone, then a pause of a second, is vim's `,` |
| `:inoremap jk <Esc>` then `ijk` | back in NORMAL. `j` shows at once; `j` then a pause, or another letter, is a `j` |
| `:inoremap ,d <C-r>=strftime('%F')<CR>` then `i,d` | today's date is inserted |
| `:nnoremap ,s :%s/` then `,s` | the `:` line opens holding `%s/` |
| `:iabbrev teh the` then type `teh ` in INSERT | `the ` |
| `i- one` Enter `two` Enter Enter (from `vimango.vim`) | `- one`, `- two`, then an empty line with no marker: the list is over. The same with `1. ` |
| Paste (Cmd-V) a list, or indented code, in INSERT | the text as it was copied: no second marker, no added indentation |
| Paste in NORMAL, with the cursor in a line | the text goes in after the cursor and vim is still in NORMAL; `u` takes it all back, `.` repeats it. Nothing in it runs as a command |
| `viw`, then paste | the text replaces the selection |
| `:` or `/`, then paste something with a line break | it is on the command line as one line, and nothing has run |
| `:command! Hi echo 'hi there'` then `:Hi` | `hi there` |
| `:source vimango.vim` | re-reads it |
| `:r somefile.txt` | the file's text is read *into* the note (reading is allowed; writing is not) |

### Keys that changed with the same work

| Try | Expect |
|---|---|
| `i` Ctrl-R `0` (after a `yy` or `yiw`) | the yanked text is inserted |
| `i` Ctrl-R `=6*7` Enter | an `=` line opens for the expression; `42` is inserted. Esc there cancels it and INSERT goes on |
| `:s/` Ctrl-R Ctrl-W | the word under the cursor is put on the line |
| `:echo '` Ctrl-R `=6*7` Enter `'` Enter | the `:` line comes back with `42` on it; then `42` |
| `/` Ctrl-R `0` | the yanked text becomes the pattern |
| `"=6*7` Enter `p` | `42` is put — `"=` used to freeze the editor |
| `/one` Enter from the last line | found on line 1, with `search hit BOTTOM, continuing at TOP`, once |
| `/alpha/e` Enter | the cursor on the last `a` of `alpha`; no error |
| `gg`, `/o` Enter, then `/` Enter | the first `o` of line 1, then the next one, on line 2: an empty pattern repeats the last |
| `/two` Enter, then ` `` ` | back where the search started |
| `:set nowrapscan`, `G`, `/alpha` Enter | `E385`, and the cursor has not moved |

### Saving and closing — vimango's meaning, in any spelling vim accepts

| Try | Expect |
|---|---|
| `:w`, `:wri`, `:up`, `:wa` | the note is written to the database (`:up` only if changed) |
| `:x`, `:wq`, `:exi` | written if changed, then closed |
| `:q`, `:clo`, `:qa` | closed; with unsaved changes, `No write since last change` and it stays open |
| `:q!` | closed without saving |

### vimango's own commands

`:spell`, `:suggest` (the same as `z=`), `:number`, `:syntax`, `:paste` /
`:nopaste`, `:checkstale`, `:sync` (TUI only), `:help`, `:help normal`.

## 2. Blocked on purpose

These are refused before vim sees them. The message is
`:<command> is not available in vimango: <reason>` unless shown otherwise.

| Try | Reason given |
|---|---|
| `:w notes.txt` | `:write takes no file name in vimango: it saves the note to the database` |
| `:1,5w` | `:write takes no range in vimango: it saves or closes the whole note` |
| `:noh \| w` | `:write cannot follow another command in vimango; give it on its own` |
| `:sav copy.md`, `:e other.md`, `:e!`, `:file x`, `:mksession`, `:redir > x`, `:wundo x`, `:recover` | it works on files, and a note lives in the database |
| `:sp`, `:vs`, `:new`, `:tabnew`, `:only`, `:bn`, `:b2`, `:bd`, `:wincmd w`, `:diffthis`, `:copen` | vimango shows one note in one window |
| `:vertical …`, `:tab …`, `:browse …`, `:confirm …` | the same: modifiers that ask for a window or a dialog |
| `:cd /tmp`, `:lcd ..` | vimango finds its database and settings relative to the working directory |
| `:!ls`, `:sh`, `:make`, `:grep`, `:vimgrep`, `:terminal` | running other programs from the editor is not supported |
| `:append`, `:insert`, `:change`, `:if 1`, `:while 1`, `:function Foo()`, `:debug …` | it needs more input than one command line |
| `:s/a/b/c`, `:%s/a/b/gc` | `:s with the c flag asks before each change, and vimango has no prompt for that` |
| `:startinsert`, `:colorscheme x`, `:highlight`, `:fold`, `:sign list`, `:menu`, `:lua …`, `:python …` | libvim has no use for it here |

**Two things vim itself refuses, by whatever route** — typed, in a mapping, in
a sourced file, inside `:execute`:

| Try | Expect |
|---|---|
| `:r !ls`, `:echo system('ls')`, `:execute '!ls'` | `E145: Shell commands and some functionality not allowed in rvim` — vim starts no program |
| `:execute 'w name.txt'`, `:call writefile(['x'], 'name.txt')` | `Writing files is not allowed here`, and no file appears — vim writes no file |

The one file vim does write is the spelling word list (`zg`, `zw`,
`:spellgood`).

### Not blocked, and that is intended

The check reads what you typed. It does not look inside vimscript: `:execute`,
`:normal` and `:call` run what they are given. It guards against habit
(`:w notes.txt`), not against intent. Behind it:

| Try | Expect |
|---|---|
| `:execute 'e other'` | `that command left the note's buffer; vimango went back to it` |
| `:execute 'cd /tmp'` | `that command changed the working directory; vimango changed it back` |
| Ctrl-^ | nothing visible: vim is brought straight back to the note |

## 3. Still missing or wrong

Worth confirming that these fail the way described, and not some other way.

| What | What happens now |
|---|---|
| `:s` with the `c` flag | refused; there is no confirm prompt |
| `:echo input('name: ')`, `inputlist()`, `inputsecret()` | returns at once with nothing (`''`, or `0`). It no longer freezes, but a script cannot ask you anything |
| Listings are laid out for 80 columns | `:registers` cuts long lines at 74 characters whatever the pane's width; `:version`'s columns may wrap in a narrow pane |
| `:filter /pat/ registers` | refused, with the "one window" reason, which is wrong: libvim could do it now |
| `:highlight`, `:function` (to list) | refused; libvim could list them now |
| `:2,3number` | toggles line numbering (vimango's `:number`) instead of printing lines |
| `:set` | each option arrives as its own message, so the overlay has a `--- Options ---` line followed by them |
| Ctrl-K (digraphs) in INSERT | inserts `^K`; digraphs are not compiled into libvim |
| Function keys (F1–F12) | never reach vim: a mapping on `<F5>` can be defined and cannot be triggered. Use a letter or a two-key sequence |
| Ctrl-\ Ctrl-N in INSERT | the two control characters are inserted into the text, instead of going to NORMAL |
| Ctrl-\ e on the `:` line | the Ctrl-\ is dropped and `e` is typed |
| `feedkeys('…', 'x')` in a script | does nothing; without the `x` the keys are run afterwards |
| A register name after Ctrl-R is not shown | vim draws a `"` while it waits; here nothing is drawn until the name is typed |
| After a search, `/pattern` stays on the message line | harmless; vim leaves it there too until something else is said |
| The search match under the cursor | highlighted like the others, not differently |
| `$` as the very first key after opening a note, then `gj`/`gk` | they keep the column instead of going to the end of each screen line (libvim forgets the "end" on the first command in a new buffer); any `$` after that is fine |
| `H`/`L` and `'scrolloff'` | `H` and `L` go to the top and bottom row exactly, as with `'scrolloff'` 0 |
| Editing a title in the organizer (TUI) | shows none of vim's messages, and expands no mappings |

If something on this page does not behave as written, that is the thing to
report: the command, what the note held, and what was shown.
