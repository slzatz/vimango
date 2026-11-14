package main

import (
	"fmt"
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/slzatz/vimango/internal/pane"
	"github.com/slzatz/vimango/vim"
)

// editorProcessKey handles one key in the note editor and reports whether the
// text needs redrawing.
func (e *Editor) editorProcessKey(c int) (redraw bool) {
	return e.redrawAfter(e.processKey(c))
}

// editorMappingTimeout is 'timeoutlen' running out on a half-typed mapping:
// MainLoop calls it when no key has followed one that vim is holding (see
// mappingWait). vim takes the held keys as they are, and the editor follows
// as it does after a key.
func (e *Editor) editorMappingTimeout() (redraw bool) {
	if vim.InputPending() == 0 {
		return false
	}
	return e.redrawAfter(e.forwardKey(noKey, vim.InputTimeout))
}

// redrawAfter adds to a key's own answer the thing a key cannot see.
func (e *Editor) redrawAfter(redraw bool) bool {
	// Search highlighting can change with no change to the text: a new
	// pattern (each character typed after "/", n, *), :noh, :set nohlsearch.
	// Not while an overlay (:help, :marks) is up: drawing the text would wipe
	// it, and closing it redraws anyway.
	if e.mode != HELP && searchState() != e.searchDrawn {
		redraw = true
	}
	return redraw
}

// mappingWait reports whether vim is holding keys that begin a longer mapping
// ("j" when "jk" is mapped), and how long vim would wait for the rest:
// 'timeoutlen'. With 'notimeout' vim waits for ever, and so does vimango --
// the next key settles it.
func mappingWait() (time.Duration, bool) {
	if vim.InputPending() == 0 || vim.EvaluateExpression("&timeout") != "1" {
		return 0, false
	}
	ms, err := strconv.Atoi(vim.EvaluateExpression("&timeoutlen"))
	if err != nil || ms < 0 {
		ms = 1000 // vim's default
	}
	return time.Duration(ms) * time.Millisecond, true
}

// noKey is forwardKey's key when there is none: a timeout.
const noKey = -1

// note that bool returned is whether to redraw
func (e *Editor) processKey(c int) (redraw bool) {

	//No matter what mode you are in an escape puts you in NORMAL mode
	if c == '\x1b' {
		vim.SendKey("<esc>")
		e.ss = e.vbuf.Lines() /// for commands like 4i
		//e.mode = NORMAL_BUSY
		prevMode := e.mode
		mode := vim.GetCurrentMode() ////////
		// <esc> ends one thing, and on a command line that need not be the
		// line: it can be the register name CTRL-R was waiting for, or the
		// expression being typed after CTRL-R =. The line under it is still
		// open, as it was.
		if mode == vim.ModeCmdline {
			e.syncCmdline()
			return false
		}
		e.mode = modeMap[mode]
		//e.ShowAside("vim mode: %d | e.mode: %s", mode, e.mode) //////Debug
		e.command_line = ""
		pos := vim.GetCursorPosition() //set screen cx and cy from pos
		e.fr = pos[0] - 1
		e.fc = pos[1]
		// vim clears the message line on <esc>; it does not announce NORMAL
		e.ShowMessage("")
		// ...and the same in INSERT: the <esc> ended a CTRL-R, or the
		// expression for one, and the insert goes on
		if e.mode == INSERT {
			e.ShowMessage("\x1b[1m-- INSERT --\x1b[0m")
		}
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
	case HELP:
		redraw, exit = e.HelpModeKeyHandler(c)
	case SPELLING:
		redraw, exit = e.SpellingModeKeyHandler(c)
	}

	// if exit true, don't process key any further
	if exit {
		return
	}
	return e.forwardKey(c, func() { pane.SendToVim(c) })
}

// forwardKey gives vim a key the user typed -- send does that -- and brings
// the editor into line with whatever vim did: mode, text, cursor, messages.
// c is the key, or noKey when send is the timeout of a half-typed mapping.
// The bool is whether to redraw.
//
// vim may do nothing yet: a key that begins a longer mapping waits for the
// next (vim.InputPending). And it may do more than the key says: a mapping can
// end anywhere, on a command line for one, so what follows asks vim where it
// is and does not reason from c.
func (e *Editor) forwardKey(c int, send func()) (redraw bool) {
	// Process the key -- with vim's mapping expansion on, so vimango.vim's
	// mappings apply to what the user types (and only to that; see vimrc.go)
	return e.followVim(c, func() { vim.SendUserKey(send) })
}

// followVim runs something that sends vim keys and brings the editor into
// line with whatever vim did, as forwardKey says. run is a typed key, with
// mappings (forwardKey), or text that is not keys at all: a paste (paste.go).
func (e *Editor) followVim(c int, run func()) (redraw bool) {
	wasVisual := e.mode == VISUAL
	wasInsert := e.mode == INSERT
	vim.TakeSpellSuggestRequest() // drop a z= typed outside the editor (organizer)
	// What vim would have put on its message line for this key -- mostly
	// errors: "E486: Pattern not found", "E20: Mark not set". Shown last, when
	// this function returns, so it is not overwritten by a message of ours.
	msgs := vim.CaptureMessages(run)
	// A key can take vim off the note's buffer: Ctrl-^ goes to the alternate
	// buffer, which here is the organizer's list of titles or vim's own
	// empty one. Every key after it would edit that buffer, unseen, while
	// the screen went on showing the note. Come straight back, and drop what
	// vim said about the buffer it visited.
	if vim.GetCurrentBuffer().GetID() != e.vbuf.GetID() {
		vim.SetCurrentBuffer(e.vbuf)
		msgs = nil
	}
	defer e.showKeyMessages(msgs)
	// zg, zw etc. change what is misspelled without changing the buffer
	spellRedraw := false
	if c != noKey {
		spellRedraw = spellOn && spellListKey(e.spellKeys, c)
		e.spellKeys = (e.spellKeys + string(rune(c)))[max(0, len(e.spellKeys)+len(string(rune(c)))-2):]
	}

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
		// A command line is open in vim. This key may have opened it -- ':'
		// '/' '?', a mapping that ends on one ("nnoremap ,s :%s/"), or
		// CTRL-R = and "=, which ask for an expression on a line of their
		// own -- or have been typed on it. Either way vim is asked which
		// kind of line it is and what is on it. (Keys typed on a ':' line
		// do not come this way: ExModeKeyHandler sends them itself.)
		opened := e.mode != SEARCH && e.mode != EX_COMMAND
		e.syncCmdline()
		if opened {
			return false
		}
	case vim.ModeInsert:
		if e.mode != INSERT {
			e.ShowMessage("\x1b[1m-- INSERT --\x1b[0m")
		}
	case vim.ModeVisual:
		e.vmode = visualModeMap[vim.GetVisualType()]
		e.highlightInfo()
		redraw = true
	}

	if mode == vim.ModeCmdline {
		// SEARCH or EX_COMMAND: syncCmdline said which
	} else if m, ok := modeMap[mode]; ok {
		e.mode = m
	} else {
		e.mode = OTHER // not sure this ever happens
	}

	// <esc> is not the only way out of INSERT: a mapping ("inoremap jk
	// <Esc>") leaves it too, and "-- INSERT --" must not stay behind.
	if wasInsert && e.mode != INSERT {
		e.ShowMessage("")
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
	if message, scrolled := e.notices.noticeKey(c); scrolled {
		if message != "" {
			e.ShowAside("%s", message)
		}
		return false, true
	}
	switch c {
	case ':': // COMMAND or SEARCH
		e.ShowMessage(":")
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
			e.ShowMessage("")
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
			e.ShowAside("tab")
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
				e.ShowAside("dir: %s  base: %s", dir, partial)

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
		e.ShowMessage(":%s", e.command_line)
		return false, true
	}

	// forward the key to vim's cmdline
	if z, found := pane.VimKeyName(c); found {
		vim.SendKey(z)
	} else if c == END_KEY {
		vim.SendKey("<end>")
	} else if c >= 1 && c <= 26 { // Ctrl-A..Ctrl-Z (Enter/Tab intercepted above)
		vim.SendKey(fmt.Sprintf("<c-%c>", rune('a'+c-1)))
	} else if c < 32 || pane.IsSyntheticKey(c) {
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
		e.ShowMessage("")
		return false, true
	}

	// mirror the line from vim for display. It may not be the ':' line any
	// more: CTRL-R = puts that aside for a line the expression is typed on,
	// and then the keys are that line's, <cr> included
	e.syncCmdline()
	return false, true
}

// syncCmdline brings the editor's idea of the command line into step with
// vim's, where it is typed: which kind of line it is, what is on it, where
// the cursor is. A ':' line is EX_COMMAND, whose <cr> vimango takes for
// itself (runExLine). Any other is SEARCH, whose keys all go to vim: '/' and
// '?', and '=' -- the line an expression is typed on after CTRL-R = (in
// INSERT, or on another command line) and after "= and @=.
func (e *Editor) syncCmdline() {
	kind := vim.CommandLineGetType()
	if kind == ':' {
		e.mode = EX_COMMAND
	} else {
		e.mode = SEARCH
	}
	e.searchPrefix = string(rune(kind))
	e.command_line = vim.CommandLineGetText()
	e.cmdLineCursor = vim.CommandLineGetPosition()
	e.tabCompletion.index = 0
	e.tabCompletion.list = nil
	e.ShowMessage("%s%s", e.searchPrefix, e.command_line)
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
			e.ShowMessage("No changes to save")
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
		e.ShowMessage("\x1b[41m%s\x1b[0m", decision.reason)
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
