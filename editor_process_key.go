package main

import (
	"fmt"
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/slzatz/vimango/vim"
)

// editorProcessKey handles one key in the note editor and reports whether the
// text needs redrawing.
func (e *Editor) editorProcessKey(c int) (redraw bool) {
	redraw = e.processKey(c)
	// Search highlighting can change with no change to the text: a new
	// pattern (each character typed after "/", n, *), :noh, :set nohlsearch.
	// Not while an overlay (:help, :marks) is up: drawing the text would wipe
	// it, and closing it redraws anyway.
	if e.mode != HELP && searchState() != e.searchDrawn {
		redraw = true
	}
	return redraw
}

// note that bool returned is whether to redraw
func (e *Editor) processKey(c int) (redraw bool) {

	//No matter what mode you are in an escape puts you in NORMAL mode
	if c == '\x1b' {
		vim.SendKey("<esc>")
		e.ss = e.vbuf.Lines() /// for commands like 4i
		//e.mode = NORMAL_BUSY
		prevMode := e.mode
		mode := vim.GetCurrentMode() ////////
		e.mode = modeMap[mode]
		//e.ShowMessage(BL, "vim mode: %d | e.mode: %s", mode, e.mode) //////Debug
		e.command_line = ""
		pos := vim.GetCursorPosition() //set screen cx and cy from pos
		e.fr = pos[0] - 1
		e.fc = pos[1]
		// vim clears the message line on <esc>; it does not announce NORMAL
		e.ShowMessage(BR, "")
		//return false
		// INSERT is below because escaping from INSERT needs a redraw if previously in VISUAL BLOCK mode and an s, c or I was typed
		if prevMode == VISUAL || prevMode == HELP || prevMode == INSERT { //need to redraw to remove highlight or if leaving help
			return true
		} else {
			return false
		}
	}

	// if exit is false, doesn't matter if redraw is false since redraw will be determined
	// by the mode processing below
	//	skip := false
	var exit bool

	// the switch is pre-processing before sending the key to vim. NORMAL,
	// VISUAL and INSERT have none: vimango intercepts no keys there, so they
	// go straight to vim (and through vimango.vim's mappings)
	switch e.mode {
	case EX_COMMAND:
		redraw, exit = e.ExModeKeyHandler(c)
	case SEARCH:
		redraw, exit = e.SearchModeKeyHandler(c)
	case HELP:
		redraw, exit = e.HelpModeKeyHandler(c)
	case SPELLING:
		redraw, exit = e.SpellingModeKeyHandler(c)
	}

	// if exit true, don't process key any further
	if exit {
		return
	}
	// Process the key -- with vim's mapping expansion on, so vimango.vim's
	// mappings apply to what the user types (and only to that; see vimrc.go)
	wasVisual := e.mode == VISUAL
	vim.TakeSpellSuggestRequest() // drop a z= typed outside the editor (organizer)
	// What vim would have put on its message line for this key -- mostly
	// errors: "E486: Pattern not found", "E20: Mark not set". Shown last, when
	// this function returns, so it is not overwritten by a message of ours.
	endsSearch := e.mode == SEARCH && c == '\r'
	msgs := vim.CaptureMessages(func() {
		vim.SendUserKey(func() { sendToVim(c) })
	})
	// A key can take vim off the note's buffer: Ctrl-^ goes to the alternate
	// buffer, which here is the organizer's list of titles or vim's own
	// empty one. Every key after it would edit that buffer, unseen, while
	// the screen went on showing the note. Come straight back, and drop what
	// vim said about the buffer it visited.
	if vim.GetCurrentBuffer().GetID() != e.vbuf.GetID() {
		vim.SetCurrentBuffer(e.vbuf)
		msgs = nil
	}
	defer e.showKeyMessages(msgs, endsSearch)
	// zg, zw etc. change what is misspelled without changing the buffer
	spellRedraw := spellOn && spellListKey(e.spellKeys, c)
	e.spellKeys = (e.spellKeys + string(rune(c)))[max(0, len(e.spellKeys)+len(string(rune(c)))-2):]

	tick := e.vbuf.GetLastChangedTick()
	if tick > e.bufferTick {
		e.bufferTick = tick
		redraw = true
	} else {
		redraw = false
	}
	mode := vim.GetCurrentMode()
	//submode := vim.GetSubMode() added this 9-22-25 but really only for obscure submodes

	// The switch performs side effects only; e.mode is set once from modeMap
	// below, so no case should assign it. The two early returns are the
	// exceptions, and each says why it skips the buffer/cursor resync.
	switch mode {
	case vim.ModeOpPending: // delete, change, yank, etc
		// half of an operator (the "d" of "d3w"): the buffer has not changed
		// and the cursor has not settled, so there is nothing to resync yet
		e.mode = PENDING
		return false
	case vim.ModeCmdline: // SEARCH and EX_COMMAND
		// reached only on the ':' or '/' that opens the line: once e.mode is
		// EX_COMMAND or SEARCH those handlers send keys to vim themselves and
		// never fall through to here
		if e.mode != SEARCH && e.mode != EX_COMMAND {
			e.command_line = ""
			if c == ':' {
				// vim stays in cmdline mode: its cmdline buffer is the line
				// editor; ExModeKeyHandler intercepts Enter so vim never
				// executes anything
				e.mode = EX_COMMAND
				// vim may have put a range on the line already: "'<,'>"
				// when ':' is pressed in VISUAL, ".,.+2" after a count
				// (3:). Show it at once, with the cursor after it -- it
				// used to appear only when the next key was typed.
				e.command_line = vim.CommandLineGetText()
				e.cmdLineCursor = vim.CommandLineGetPosition()
				e.tabCompletion.index = 0
				e.tabCompletion.list = nil
				e.ShowMessage(BR, ":%s", e.command_line)
			} else {
				e.mode = SEARCH
				e.searchPrefix = string(rune(c))
				e.ShowMessage(BR, "%s", e.searchPrefix)
			}
			return false
		}
	case vim.ModeInsert:
		if e.mode != INSERT {
			e.ShowMessage(BR, "\x1b[1m-- INSERT --\x1b[0m")
		}
	case vim.ModeVisual:
		e.vmode = visualModeMap[vim.GetVisualType()]
		e.highlightInfo()
		redraw = true
	}

	if m, ok := modeMap[mode]; ok { // note that ModeCmdline => SEARCH (it is also COMMAND)
		e.mode = m
	} else {
		e.mode = OTHER // not sure this ever happens
	}

	// A key that ends VISUAL without changing text (y, a mapping that found
	// nothing to do) would otherwise leave the selection highlight on screen:
	// the tick check above saw no change, and only VISUAL itself redraws.
	if wasVisual && e.mode != VISUAL {
		redraw = true
	}

	//below is done for everything except SEARCH, EX_COMMAND and OP_PENDING
	e.ss = e.vbuf.Lines()
	// Add safety checks to prevent panic with empty buffers
	if len(e.ss) == 0 {
		e.ss = []string{""}
	}

	pos := vim.GetCursorPosition() //set screen cx and cy from pos
	e.fr = pos[0] - 1

	// Ensure fr is in bounds
	if e.fr < 0 {
		e.fr = 0
	}
	if e.fr >= len(e.ss) {
		e.fr = len(e.ss) - 1
	}

	// Ensure pos[1] (column) is valid
	if pos[1] > len(e.ss[e.fr]) {
		pos[1] = len(e.ss[e.fr])
	}

	e.fc = pos[1]

	if spellRedraw {
		redraw = true
	}
	// z= without a count: libvim asked for the suggestion list
	if vim.TakeSpellSuggestRequest() {
		e.spellSuggest()
	}

	return
}

// case HELP: the :help text is drawn in a notice (notice.go) over the editor
// pane; the notice's keys scroll it, and ':' starts a new command line.
func (e *Editor) HelpModeKeyHandler(c int) (redraw, exit bool) {
	if message, scrolled := e.Screen.noticeKey(c); scrolled {
		if message != "" {
			e.ShowMessage(BL, "%s", message)
		}
		return false, true
	}
	switch c {
	case ':': // COMMAND or SEARCH
		e.ShowMessage(BR, ":")
		vim.SendKey("<esc>")
		vim.SendInput(":") // put vim in cmdline mode (the line editor)
		e.command_line = ""
		e.cmdLineCursor = 0
		e.mode = EX_COMMAND
		e.tabCompletion.index = 0
		e.tabCompletion.list = nil
	}
	return false, true
}

// case EX_COMMAND:
// vim sits in cmdline mode acting purely as a line editor: keys are forwarded
// and the line + cursor are mirrored from vim's cmdline buffer. vim executes
// nothing until it sees a CR, and Enter is intercepted below, so dispatch
// stays vimango's and custom commands like :checkstale never collide with vim's.
func (e *Editor) ExModeKeyHandler(c int) (redraw, skip bool) {
	if c == '\r' {
		// vim's cmdline buffer is authoritative; read it, then cancel vim's
		// cmdline before dispatching
		e.command_line = vim.CommandLineGetText()
		vim.SendKey("<esc>")
		if e.command_line == "" {
			e.mode = NORMAL
			e.ShowMessage(BR, "")
			return false, true
		}
		// make the line recallable with <up>/<down> next time (vim only adds
		// history on a concluded cmdline, and we never forward the CR)
		vim.ExecuteCommand("call histadd(':', '" + strings.ReplaceAll(e.command_line, "'", "''") + "')")

		e.tabCompletion.index = 0
		e.tabCompletion.list = nil
		return e.runExLine(e.command_line), true
	} //end 'r'

	if c == '\t' {
		pos := strings.Index(e.command_line, " ")
		if e.tabCompletion.list == nil {
			e.ShowMessage(BL, "tab")
			var s string
			if pos != -1 {
				s = e.command_line[pos+1:]
				//cl := p.command_line
				dir := filepath.Dir(s)
				if dir == "~" {
					usr, _ := user.Current()
					dir = usr.HomeDir
				} else if strings.HasPrefix(dir, "~/") {
					usr, _ := user.Current()
					dir = filepath.Join(usr.HomeDir, dir[2:])
				}

				partial := filepath.Base(s)
				paths, _ := ioutil.ReadDir(dir)
				e.ShowMessage(BL, "dir: %s  base: %s", dir, partial)

				for _, path := range paths {
					if strings.HasPrefix(path.Name(), partial) {
						e.tabCompletion.list = append(e.tabCompletion.list, filepath.Join(dir, path.Name()))
					}
				}
			}
			if len(e.tabCompletion.list) == 0 {
				return false, true
			}
		} else {
			e.tabCompletion.index++
			if e.tabCompletion.index > len(e.tabCompletion.list)-1 {
				e.tabCompletion.index = 0
			}
		}
		e.command_line = e.command_line[:pos+1] + e.tabCompletion.list[e.tabCompletion.index]
		// write the completed line back into vim's cmdline buffer so vim and
		// the display stay in sync (end, clear, retype)
		vim.SendKey("<c-e>")
		vim.SendKey("<c-u>")
		vim.SendMultiInput(e.command_line)
		e.cmdLineCursor = len(e.command_line)
		e.ShowMessage(BR, ":%s", e.command_line)
		return false, true
	}

	// forward the key to vim's cmdline
	if z, found := termcodes[c]; found {
		vim.SendKey(z)
	} else if c == END_KEY {
		vim.SendKey("<end>")
	} else if c >= 1 && c <= 26 { // Ctrl-A..Ctrl-Z (Enter/Tab intercepted above)
		vim.SendKey(fmt.Sprintf("<c-%c>", rune('a'+c-1)))
	} else if c < 32 || isSyntheticKey(c) {
		// other control chars, and reader codes vim has no binding for --
		// forwarding one of those would type its code point into the cmdline
		return false, true
	} else {
		vim.SendInput(string(rune(c)))
	}

	// backspacing over the ':' (or Ctrl-C) makes vim leave cmdline mode
	if vim.GetCurrentMode() != vim.ModeCmdline {
		e.command_line = ""
		e.cmdLineCursor = 0
		e.mode = NORMAL
		e.ShowMessage(BR, "")
		return false, true
	}

	// mirror line and cursor from vim for display
	e.command_line = vim.CommandLineGetText()
	e.cmdLineCursor = vim.CommandLineGetPosition()

	e.tabCompletion.index = 0
	e.tabCompletion.list = nil

	e.ShowMessage(BR, ":%s", e.command_line)
	return false, true
}

// runExLine carries out an ex command line the user has entered (without its
// ':') and reports whether the text needs redrawing. Three kinds of command,
// tried in this order:
//
//  1. Saving, closing and help, in any spelling vim accepts (:w :wri :up :wq
//     :x :exi :q :q! :clo :h ...). They are vimango's: a note is a database
//     row, so nothing here writes a file.
//  2. vimango's own commands, by exact name (:sync, :spell, :number, ...).
//  3. Everything else is vim's. classifyEx (ex_forward.go) decides whether it
//     is forwarded or refused.
func (e *Editor) runExLine(line string) (redraw bool) {
	e.exLine = line
	decision := classifyEx(line)

	switch decision.action {
	case exSave:
		e.writeNote()
	case exSaveIfModified:
		if e.isModified() {
			e.writeNote()
		} else {
			e.ShowMessage(BR, "No changes to save")
		}
	case exSaveQuit:
		e.closeNote(true, false)
	case exQuit:
		e.closeNote(false, false)
	case exQuitForce:
		e.closeNote(false, true)
	case exHelp:
		e.command_line = strings.TrimSpace("help " + decision.arg)
		e.help()
	}
	if decision.action != exForward && decision.action != exRefuse {
		e.command_line = ""
		if e.mode != HELP {
			e.mode = NORMAL
		}
		return false
	}

	// vimango's own commands. The lookup is on the first word after any range
	// ("'<,'>" is what vim puts there when ':' is pressed in VISUAL mode).
	// Not for a refused spelling of save or close (":w notes.txt"): "w" is in
	// the registry too, and would save while ignoring the file name.
	e.command_line = skipExRange(line)
	name, _, _ := strings.Cut(e.command_line, " ")
	if cmd, found := e.exCmds[name]; found && !decision.owned {
		cmd(e)
		e.command_line = ""
		// a command that put up its own screen (:help) or is waiting for a
		// choice (:suggest) owns the mode from here; everything else lands
		// back in NORMAL
		if e.mode != HELP && e.mode != SPELLING {
			e.mode = NORMAL
		}
		// A command that changed the buffer has to be drawn now: dispatch
		// ends the keystroke, so otherwise the edit sits invisible until the
		// next key happens to trip the tick check in editorProcessKey.
		if tick := e.vbuf.GetLastChangedTick(); tick > e.bufferTick && e.mode != HELP {
			e.bufferTick = tick
			e.resyncFromVim()
			return true
		}
		return false
	}

	e.command_line = ""
	e.mode = NORMAL
	if decision.action == exRefuse {
		e.ShowMessage(BR, "\x1b[41m%s\x1b[0m", decision.reason)
		return false
	}
	redraw = e.forwardEx(line)
	return redraw && e.mode != HELP // an overlay (:marks) is up: draw the text when it closes
}

// resyncFromVim copies vim's buffer and cursor into the editor, clamped.
func (e *Editor) resyncFromVim() {
	e.ss = e.vbuf.Lines()
	if len(e.ss) == 0 {
		e.ss = []string{""}
	}
	pos := vim.GetCursorPosition()
	e.fr = min(max(pos[0]-1, 0), len(e.ss)-1)
	e.fc = min(max(pos[1], 0), len(e.ss[e.fr]))
}

// case SEARCH:
func (e *Editor) SearchModeKeyHandler(c int) (bool, bool) {
	// Below is purely for displaying command line correctly
	// vim is actually handling the keystrokes whether we do this or not
	if c == DEL_KEY || c == BACKSPACE {
		e.command_line = dropLastRune(e.command_line)
	} else if !isSyntheticKey(c) {
		// sendToVim drops the other reader codes, so mirroring one here would
		// show a character vim never received
		e.command_line += string(rune(c))
	}
	e.ShowMessage(BR, "%s%s", e.searchPrefix, e.command_line)
	return false, false
}
