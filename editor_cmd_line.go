package main

import (
	"fmt"
	"strings"

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
		helpText = e.commandRegistry.FormatAllHelp()
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
	app.Organizer.drawNotice(helpText)
	app.Organizer.altRowoff = 0
	e.command_line = ""
}

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
	"Mappings live in vimango.vim, next to config.json.\n"

func (e *Editor) writeNote() {
	text := e.bufferToString()

	err := e.Database.updateNote(e.id, text)
	if err != nil {
		e.ShowMessage(BL, "Error in updating note (updateNote) for entry with id %d: %v", e.id, err)
		return
	}
	e.ShowMessage(BL, "Updated note and fts entry for entry %d", e.id) //////

	// The database now holds exactly this text, so it becomes the
	// baseline :checkstale compares against.
	e.dbText = text

	//explicitly writes note to set isModified to false
	//vim.Execute("w")
	e.saveTick = e.vbuf.GetLastChangedTick()

	e.drawStatusBar() //need this since now refresh won't do it unless redraw =true
}

func (e *Editor) readFile() {
	vim.SendInput(":" + e.command_line + "\r")
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

/*
// debugging
func (e *Editor) modified() {
	var result bool
	err := v.BufferOption(0, "modified", &result) //or e.vbuf
	if err != nil {
		e.ShowMessage(BL, "%s", err)
		return
	}
	e.ShowMessage(BL, "Modified = %t", result)
}
*/

func (e *Editor) exit() {
	e.quitActions()
}

func (e *Editor) quitActions() {
	cmd := e.command_line
	if cmd == "x" || cmd == "exit" {
		text := e.bufferToString()
		err := e.Database.updateNote(e.id, text)
		if err != nil {
			e.ShowMessage(BR, "Error in updateNote for entry with id %d: %v", e.id, err)
		}
		e.ShowMessage(BL, "Updated note and fts entry for entry %d", e.id) //////

	} else if cmd == "q!" || cmd == "quit!" {
		// do nothing = allow editor to be closed

	} else if e.isModified() {
		e.mode = NORMAL
		e.command_line = ""
		e.ShowMessage(BR, "No write since last change")
		return
	}

	if e.Session.editorOnly {
		// no organizer to return to: quitting the editor exits the app
		// (leave the buffer intact; the main loop finishes this iteration
		// before a.Run is checked)
		app.Run = false
		return
	}

	vim.ExecuteCommand("bw") // wipe out the buffer
	e.Session.editor = nil
	e.Session.editorMode = false
	vim.SetCurrentBuffer(app.Organizer.vbuf)
	e.Screen.eraseRightScreen()
	app.Organizer.displayNote()
	app.returnCursor() //because main while loop if started in editor_mode -- need this 09302020
}

func (e *Editor) checkStale() {
	if !e.Database.entryExists(e.id) {
		e.ShowMessage(BR, "Note %d was deleted elsewhere — :w will not bring it back", e.id)
		return
	}

	dbText := e.Database.readNoteIntoString(e.id)
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
	e.title = e.Database.getTitle(e.id)
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
	app.Organizer.synchronize(0)
	app.returnCursor()
}
