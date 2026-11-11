package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/slzatz/vimango/internal/store"
	"github.com/slzatz/vimango/vim"
)

func (a *App) setEditorExCmds(editor *Editor) map[string]func(*Editor) {
	registry := NewCommandRegistry[func(*Editor)]()

	// File Operations commands
	registry.Register("write", (*Editor).writeNote, CommandInfo{
		Aliases:     []string{"w"},
		Description: "Save current note to database",
		Usage:       "write",
		Category:    "File Operations",
		Examples:    []string{":write", ":w"},
	})

	registry.Register("sync", (*Editor).synchronize, CommandInfo{
		Aliases:     []string{"test", "sync?"},
		Description: "Synchronize with remote server (test or sync? for dry-run)",
		Usage:       "sync",
		Category:    "Data Management",
		Examples:    []string{":sync", ":test (dry-run)", ":sync? (dry-run)"},
	})

	registry.Register("read", (*Editor).readFile, CommandInfo{
		Aliases:     []string{"r"},
		Description: "Read contents from file into current note",
		Usage:       "read <filename>",
		Category:    "File Operations",
		Examples:    []string{":read todo.txt", ":r /tmp/notes.md"},
	})

	registry.Register("checkstale", (*Editor).checkStale, CommandInfo{
		Description: "Reload this note if the database changed underneath it (a clean buffer only; " +
			"warns instead when you have unsaved changes). Injected by the host app after a sync",
		Usage:    "checkstale",
		Category: "File Operations",
		Examples: []string{":checkstale"},
	})

	// Editing commands
	registry.Register("syntax", (*Editor).syntax, CommandInfo{
		Description: "Set syntax highlighting for current note",
		Usage:       "syntax <language>",
		Category:    "Editing",
		Examples:    []string{":syntax go", ":syntax python", ":syntax markdown"},
	})

	registry.Register("number", (*Editor).number, CommandInfo{
		Aliases:     []string{"num"},
		Description: "Toggle line numbers on/off",
		Usage:       "number",
		Category:    "Editing",
		Examples:    []string{":number", ":num"},
	})

	registry.Register("paste", (*Editor).paste, CommandInfo{
		//Aliases:     []string{"num"},
		Description: "Autoindent off - good for pasting content",
		Usage:       "paste",
		Category:    "Editing",
		Examples:    []string{":paste"},
	})

	registry.Register("nopaste", (*Editor).nopaste, CommandInfo{
		//Aliases:     []string{"num"},
		Description: "Autoindent on - not good for pasting content",
		Usage:       "nopaste",
		Category:    "Editing",
		Examples:    []string{":nopaste"},
	})
	// Utility commands
	registry.Register("spell", (*Editor).toggleSpell, CommandInfo{
		Description: "Toggle spell checking (misspellings are highlighted as you edit)",
		Usage:       "spell",
		Category:    "Utility",
		Examples:    []string{":spell"},
	})

	registry.Register("suggest", (*Editor).spellSuggest, CommandInfo{
		Description: "Pick a spelling correction for the word under the cursor (same as z=)",
		Usage:       "suggest",
		Category:    "Utility",
		Examples:    []string{":suggest"},
	})

	// System commands
	registry.Register("quit", (*Editor).quitActions, CommandInfo{
		Aliases:     []string{"q", "quit!", "q!"},
		Description: "Close current editor (q!/quit! forces without saving)",
		Usage:       "quit",
		Category:    "System",
		Examples:    []string{":quit", ":q", ":q!", ":x"},
	})

	registry.Register("exit", (*Editor).exit, CommandInfo{
		Aliases:     []string{"x"},
		Description: "Save and exit the current file",
		Usage:       "x",
		Category:    "System",
		Examples:    []string{":exit", ":x"},
	})

	// Help command
	registry.Register("help", (*Editor).help, CommandInfo{
		Aliases:     []string{"h"},
		Description: "Show help for editor commands",
		Usage:       "help [command|category]",
		Category:    "Help",
		Examples:    []string{":help", ":help write", ":help File Operations", ":h"},
	})

	// Store registry in editor for help command access
	editor.commandRegistry = registry

	return registry.GetFunctionMap()
}

// help displays help information for editor commands
func (e *Editor) help() {
	if e.commandRegistry == nil {
		e.ShowMessage(BR, "Help system not available")
		return
	}

	var helpText string
	pos := strings.Index(e.command_line, " ")

	if pos == -1 {
		// No arguments - show all ex commands
		helpText = e.commandRegistry.FormatAllHelp() + editorVimExHelp
	} else {
		arg := e.command_line[pos+1:]
		categories := e.commandRegistry.GetAllCommands()
		if arg == "normal" {
			helpText = editorNormalModeHelp
		} else if _, exists := e.commandRegistry.GetCommandInfo(arg); exists {
			helpText = e.commandRegistry.FormatCommandHelp(arg)
		} else if _, exists := categories[arg]; exists {
			helpText = e.commandRegistry.FormatCategoryHelp(arg)
		} else if suggestions := e.commandRegistry.SuggestCommand(arg); len(suggestions) > 0 {
			helpText = fmt.Sprintf("Command or category '%s' not found.\nDid you mean: %s", arg, strings.Join(suggestions, ", "))
		} else {
			helpText = fmt.Sprintf("Command or category '%s' not found.\nUse ':help' for ex commands or ':help normal' for normal mode.", arg)
		}
	}
	e.mode = HELP
	e.Screen.showNotice(helpText, 16)
	e.command_line = ""
}

// editorVimExHelp follows the list of vimango's own commands in ":help". What
// is and is not forwarded is decided in ex_forward.go; keep this in step.
const editorVimExHelp = "\n## vim's ex commands\n\n" +
	"Any other command is vim's and goes to vim, with ranges and `|` as usual:\n" +
	"`:s` `:g` `:v` `:d` `:m` `:t` `:j` `:>` `:<` `:normal` `:sort` `:u` `:red` `:noh`\n" +
	"`:set` `:let` `:echo` `:map` `:marks` `:source`, and a line number (`:42`).\n" +
	"What vim says back is shown on the message line.\n\n" +
	"`:w` `:up` `:wq` `:x` `:q` `:q!` save and close the note in the database, in\n" +
	"any spelling vim accepts (`:wri`, `:exi`, `:clo`).\n\n" +
	"Not available: commands that write or open files (`:w name`, `:sav`, `:e`,\n" +
	"`:cd`), that need another window or buffer (`:sp`, `:bn`, `:tabnew`), that\n" +
	"run programs (`:!`, `:r !cmd`, `system()`), or that need a prompt (`:s/a/b/c`,\n" +
	"`:append`).\n"

// editorNormalModeHelp is ":help normal" in the editor. The editor intercepts
// no normal-mode keys, so there is no registry to list: normal mode is vim's,
// plus the mappings in vimango.vim -- keep this list in step with that file.
const editorNormalModeHelp = "Normal Mode:\n\n" +
	"Normal and visual mode are vim's own: vimango intercepts no keys, so vim's\n" +
	"commands work as they do in vim.\n\n" +
	"## vimango.vim mappings:\n" +
	"`  Ctrl-B         ` - bold (**): the word under the cursor, or the selection\n" +
	"`  Ctrl-I         ` - italic (*); also Tab, the same key to a terminal\n" +
	"`  Ctrl-E         ` - code (`)\n" +
	"`  - (visual)     ` - toggle the selected lines as a bulleted list\n" +
	"`  # (visual)     ` - toggle the selected lines as a numbered list\n\n" +
	"The same key again removes a decoration or list; the other one swaps it.\n" +
	"\nSearch with `/` `?` `n` `N` `*` `#`: the matches are highlighted until `:noh`\n" +
	"(`set nohlsearch` in vimango.vim turns that off).\n" +
	"Mappings live in vimango.vim, next to config.json.\n"

// save writes the buffer to the database. Once the note is in, the buffer is
// clean (saveTick) and its text is the baseline :checkstale compares against
// (dbText). On an error the database does not hold this text, and the buffer
// stays modified so nothing treats it as safe to discard.
func (e *Editor) save() error {
	text := e.bufferToString()
	if err := e.Store.SetNote(e.id, text); err != nil {
		return err
	}
	e.dbText = text
	e.saveTick = e.vbuf.GetLastChangedTick()
	return nil
}

func (e *Editor) writeNote() {
	if err := e.save(); err != nil {
		e.ShowMessage(BL, "Note %d NOT saved: %v", e.id, err)
		return
	}
	e.ShowMessage(BL, "Saved note %d", e.id)
	e.drawStatusBar() //need this since now refresh won't do it unless redraw =true
}

// readFile is :read, vim's own: the lines of a file, inserted below the cursor.
func (e *Editor) readFile() {
	e.forwardEx(e.exLine)
}

func (e *Editor) paste() {
	vim.SendInput(":set paste\r")
}

func (e *Editor) nopaste() {
	vim.SendInput(":set nopaste\r")
}

func (e *Editor) syntax() {
	e.highlightSyntax = !e.highlightSyntax
	if e.highlightSyntax {
		e.left_margin_offset = LEFT_MARGIN_OFFSET
		//e.checkSpelling = false // can't syntax highlight(including markdown) and check spelling
	}
	e.drawText()
	// no need to call drawFrame or drawStatusBar
	e.ShowMessage(BR, "Syntax highlighting is %v", e.highlightSyntax)
}

// exit and quitActions are the registry's entries for :x and :q. runExLine
// reaches closeNote directly for every spelling of them, so these run only
// if something calls the registry's functions by name.
func (e *Editor) exit() {
	e.closeNote(true, false)
}

func (e *Editor) quitActions() {
	e.closeNote(false, strings.HasSuffix(e.command_line, "!"))
}

// closeNote closes the editor: :q (save and force both false), :q! (force),
// :x and :wq (save). The rule for all of them: the editor closes only when
// the buffer's text is in the database or the user has said to throw it away.
//
// A host app leans on this. vimango_hybrid cannot see the buffer, so it
// injects ":q" or ":x" and watches whether the process exits: an editor still
// running means "not saved", and the host keeps the window open on it. Until
// 2026-11-10 :x reported a failed write and quit anyway, which the host read
// as "saved" -- the edits were gone with nothing to show for it.
func (e *Editor) closeNote(save, force bool) {
	switch {
	case save:
		// As in vim, :x writes only a modified buffer. Writing a clean one
		// would bump the note's modified time -- reordering the list and
		// queuing a sync push for nothing -- and, if a sync had changed the
		// row since it was read, overwrite the newer text with the old.
		// :wq does the same here, for the same reason.
		if e.isModified() {
			if err := e.save(); err != nil {
				e.mode = NORMAL
				e.command_line = ""
				e.ShowMessage(BR, "\x1b[41mNote %d NOT saved: %v\x1b[0m", e.id, err)
				return
			}
		}
	case force:
		// the user's call: close without saving
	case e.isModified():
		e.mode = NORMAL
		e.command_line = ""
		e.ShowMessage(BR, "No write since last change")
		return
	}

	// back to the organizer in the TUI; in --editor mode, the end of the process
	app.noteClosed()
}

func (e *Editor) checkStale() {
	title, dbText, err := e.Store.TitleAndNote(e.id)
	if errors.Is(err, store.ErrNotFound) {
		e.ShowMessage(BR, "Note %d was deleted elsewhere — :w will not bring it back", e.id)
		return
	}
	if err != nil {
		// Not knowing is not the same as "it changed": a clean buffer is
		// reloaded with what was read, and nothing was.
		e.ShowMessage(BR, "Could not check note %d against the database: %v", e.id, err)
		return
	}
	if dbText == e.dbText {
		// Silence is the point: this runs after every sync, and most
		// syncs don't touch the note you happen to have open.
		return
	}

	if e.isModified() {
		// Both sides changed. Reloading would discard the user's edits and
		// `:w` will discard the database's — only they can choose, so
		// say so and touch nothing.
		e.ShowMessage(BR, "Note %d changed in the database and you have unsaved changes — :w will overwrite it", e.id)
		return
	}

	e.dbText = dbText
	e.title = title
	e.ss = strings.Split(dbText, "\n")
	if len(e.ss) == 0 {
		e.ss = []string{""}
	}
	e.vbuf.SetLines(0, -1, e.ss)
	// The reload is not an edit: keep the buffer clean so the quit guard
	// and `:w` see it for what it is.
	e.bufferTick = e.vbuf.GetLastChangedTick()
	e.saveTick = e.bufferTick

	// Hold the cursor where the user left it, clamped into the new text
	// (the synced version can be shorter).
	if e.fr >= len(e.ss) {
		e.fr = len(e.ss) - 1
	}
	if e.fc > len(e.ss[e.fr]) {
		e.fc = len(e.ss[e.fr])
	}
	vim.SetCursorPosition(e.fr+1, e.fc)
	e.scroll()

	e.Screen.eraseRightScreen()
	e.Screen.drawRightScreen()
	e.ShowMessage(BR, "Note %d reloaded — it changed in the database since you opened it", e.id)
}

func (e *Editor) number() {
	e.numberLines = !e.numberLines
	if e.numberLines {
		e.left_margin_offset = LEFT_MARGIN_OFFSET
	} else {
		e.left_margin_offset = 0
	}
	e.drawText()
	e.ShowMessage(BR, "Line numbering is %t", e.numberLines)
}

// synchronize makes :sync work from the editor pane (the macOS host
// app's Sync menu feeds ":sync\r" into the pty without knowing which
// pane has focus). It returns focus to the organizer — same steps as
// moveLeft's return path — and delegates to the organizer's synchronize
// so the full report renders in the notice view exactly as it does when
// syncing from the organizer.
func (e *Editor) synchronize() {
	if e.Session.editorOnly {
		// --editor runs inside a host app that owns sync (it spawns
		// vimango-sync itself and refreshes its list, status bar, and
		// :synclog afterwards). An in-process sync here would run behind
		// the host's back — and could race a host-initiated sync — so
		// redirect instead of syncing.
		e.ShowMessage(BR, "sync is handled by the app — use the Sync menu")
		return
	}

	if e.Screen.divider < 10 {
		e.Screen.edPct = 80
		app.moveDividerPct(80)
	}
	e.Session.editorMode = false
	vim.SetCurrentBuffer(app.Organizer.vbuf)
	app.Organizer.command_line = e.command_line // dry-run detection (:test / :sync?)
	app.Organizer.synchronize(exArgs{name: "sync"})
	app.returnCursor()
}
