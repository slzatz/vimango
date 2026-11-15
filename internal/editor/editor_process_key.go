package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/slzatz/vimango/internal/pane"
	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// ProcessKey handles one key in the note editor and reports whether the
// text needs redrawing.
func (e *Editor) ProcessKey(c int) (redraw bool) {
	return e.redrawAfter(e.handleKey(c))
}

// MappingTimeout is 'timeoutlen' running out on a half-typed mapping:
// MainLoop calls it when no key has followed one that vim is holding (see
// MappingWait). vim takes the held keys as they are, and the editor follows
// as it does after a key.
func (e *Editor) MappingTimeout() (redraw bool) {
	if vim.InputPending() == 0 {
		return false
	}
	return e.redrawAfter(e.forwardKey(NoKey, vim.InputTimeout))
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
	// A notice is up and the keys are no longer its own, nor a command
	// line's typed over it: that line ended -- <esc>, an empty Enter, a
	// command that showed nothing -- and the keys are the note's again.
	// Draw the note, which is the end of the notice.
	if e.notice.up && e.mode != HELP && e.mode != EX_COMMAND && e.mode != SEARCH {
		redraw = true
	}
	return redraw
}

// MappingWait reports whether vim is holding keys that begin a longer mapping
// ("j" when "jk" is mapped), and how long vim would wait for the rest:
// 'timeoutlen'. With 'notimeout' vim waits for ever, and so does vimango --
// the next key settles it.
func MappingWait() (time.Duration, bool) {
	if vim.InputPending() == 0 || vim.EvaluateExpression("&timeout") != "1" {
		return 0, false
	}
	ms, err := strconv.Atoi(vim.EvaluateExpression("&timeoutlen"))
	if err != nil || ms < 0 {
		ms = 1000 // vim's default
	}
	return time.Duration(ms) * time.Millisecond, true
}

// NoKey is forwardKey's key when there is none: a timeout.
const NoKey = -1

// handleKey is ProcessKey without what a key cannot see (redrawAfter). The
// bool is whether to redraw.
func (e *Editor) handleKey(c int) (redraw bool) {
	// <esc> goes to vim whatever the editor is doing: it ends :help and the
	// list of spelling suggestions as it ends an insert, a selection or a
	// command line. vimango sends it itself, so it never reaches a mapping.
	// What vim then did is followed as it is for any key -- it can be more
	// than ending something: a key vim was holding for a mapping goes in
	// ahead of the <esc>, and an insert with a count is carried out by it.
	if c == '\x1b' {
		return e.followVim(c, func() { vim.SendKey("<esc>") })
	}

	// What the editor does with a key before vim sees it. NORMAL, VISUAL and
	// INSERT have nothing here: vimango intercepts no keys there, so they go
	// straight to vim (and through vimango.vim's mappings). exit means the
	// key went no further, and redraw is then the answer.
	var exit bool
	switch e.mode {
	case EX_COMMAND:
		redraw, exit = e.ExModeKeyHandler(c)
	case HELP:
		redraw, exit = e.HelpModeKeyHandler(c)
	case SPELLING:
		redraw, exit = e.SpellingModeKeyHandler(c)
	}
	if exit {
		return
	}
	return e.forwardKey(c, func() { pane.SendToVim(c) })
}

// forwardKey gives vim a key the user typed -- send does that -- and brings
// the editor into line with whatever vim did: mode, text, cursor, messages.
// c is the key, or NoKey when send is the timeout of a half-typed mapping.
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
// mappings (forwardKey); <esc>, which vimango sends itself (handleKey); or
// text that is not keys at all, a paste (editor_paste.go).
func (e *Editor) followVim(c int, run func()) (redraw bool) {
	was := e.mode
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
	if c != NoKey {
		spellRedraw = spellOn && spellListKey(e.spellKeys, c)
		e.spellKeys = (e.spellKeys + string(rune(c)))[max(0, len(e.spellKeys)+len(string(rune(c)))-2):]
	}

	// The text and the cursor, then the mode: all three are vim's, and are
	// asked for after every key rather than worked out from it. A key that
	// begins a longer mapping has done nothing yet; a mapping can do
	// anything.
	redraw = e.resyncFromVim()
	mode := vim.GetCurrentMode()

	// The switch performs side effects only; e.mode is set once from modeMap
	// below, so no case should assign it.
	switch mode {
	case vim.ModeCmdline: // SEARCH and EX_COMMAND
		// A command line is open in vim. This key may have opened it -- ':'
		// '/' '?', a mapping that ends on one ("nnoremap ,s :%s/"), or
		// CTRL-R = and "=, which ask for an expression on a line of their
		// own -- or have been typed on it. Either way vim is asked which
		// kind of line it is and what is on it. (Keys typed on a ':' line
		// do not come this way: ExModeKeyHandler sends them itself.)
		//
		// <esc> can leave one open, too: it ends one thing, and that need
		// not be the line. It can be the register name CTRL-R was waiting
		// for, or the expression being typed after CTRL-R =, and the line
		// under it is still there.
		e.syncCmdline()
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

	// The message line. vim says what mode it is in only for INSERT, and an
	// <esc> clears the line whatever was on it. "-- INSERT --" must not stay
	// behind when the insert ends some other way, either: a mapping
	// ("inoremap jk <Esc>") ends it too.
	switch {
	case mode == vim.ModeCmdline:
		// the command line is on it (syncCmdline)
	case c == '\x1b' || (was == INSERT && e.mode != INSERT):
		e.command_line = ""
		e.ShowMessage("")
		// an <esc> can leave the insert going: it ended a CTRL-R, or the
		// expression typed for one
		if e.mode == INSERT {
			e.ShowMessage(insertMessage)
		}
	case e.mode == INSERT && was != INSERT:
		e.ShowMessage(insertMessage)
	}

	// What has to be drawn though the text is the same -- the tick check
	// above saw no change:
	//   - the selection, when VISUAL ends without an edit (y, <esc>, a
	//     mapping that found nothing to do; only VISUAL itself redraws).
	//     ':' ends it too, as it does in vim: the line opens on "'<,'>" and
	//     the selection is no longer shown;
	//   - the word under the cursor, when INSERT ends: it is not marked as
	//     misspelled while it is being typed, and is once it is not;
	//   - the text itself, when <esc> closes a notice that was over it.
	if was != e.mode && (was == VISUAL || was == INSERT || was == HELP) {
		redraw = true
	}

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
	if message, scrolled := e.Notices.NoticeKey(c); scrolled {
		if message != "" {
			e.ShowAside("%s", message)
		}
		return false, true
	}
	switch c {
	case ':':
		vim.SendKey("<esc>")
		vim.SendInput(":") // put vim in cmdline mode (the line editor)
		e.syncCmdline()
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

		e.tabCompletion.list = nil
		return e.runExLine(e.command_line), true
	} //end 'r'

	if c == '\t' {
		e.completeFileName()
		return false, true
	}

	// forward the key to vim's cmdline
	if z, found := pane.VimKeyName(c); found {
		vim.SendKey(z)
	} else if c == terminal.KeyEnd {
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

// completeFileName is Tab on a ':' line. Where the line is a command that
// takes a file name (:read, :source) it puts on the line the first file whose
// name starts with what is typed of one, and each Tab after it the next,
// round again from the last. Anywhere else, and with no such file, it does
// nothing. A key that is not Tab ends the round (syncCmdline).
func (e *Editor) completeFileName() {
	round := e.tabCompletion
	if round.list == nil {
		prefix, name, ok := fileNameArgument(e.command_line)
		if !ok {
			return
		}
		round.prefix, round.list, round.index = prefix, fileNamesStarting(name), 0
		if len(round.list) == 0 {
			return
		}
	} else {
		round.index = (round.index + 1) % len(round.list)
	}
	// the line is vim's: clear it there (to its end, then back to its start),
	// type the new one, and read it back
	vim.SendKey("<c-e>")
	vim.SendKey("<c-u>")
	vim.SendMultiInput(round.prefix + round.list[round.index])
	e.syncCmdline()
	e.tabCompletion = round
}

// fileNameArgument splits a ':' line being typed into what is typed of a file
// name, at its end, and everything before that. ok is false when the line is
// not a command that takes a file name, is one vimango refuses (":e", ":w
// name"), or has no blank after the command yet.
func fileNameArgument(line string) (prefix, name string, ok bool) {
	cmd, rest, found := vim.FindExCommand(skipExRange(line))
	if !found || cmd.Flags&vim.ExXFile == 0 || classifyEx(line).action == exRefuse {
		return "", "", false
	}
	name = strings.TrimLeft(rest, " \t")
	// ":r !cmd" is a program, not a file; and a blank or a bar in what is
	// left means the file name is not what is being typed
	if name == rest || strings.HasPrefix(name, "!") || strings.ContainsAny(name, " \t|`") {
		return "", "", false
	}
	return line[:len(line)-len(name)], name, true
}

// fileNamesStarting is the files whose names start with typed, in name order
// and as they would be typed: with the directory typed in front, "~/"
// included, and a "/" after a directory. Hidden files only when the name typed
// starts with a dot.
func fileNamesStarting(typed string) []string {
	dir, start := filepath.Split(typed)
	look := dir
	switch {
	case dir == "":
		look = "."
	case strings.HasPrefix(dir, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		look = filepath.Join(home, dir[2:])
	}
	entries, err := os.ReadDir(look)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, start) || (strings.HasPrefix(name, ".") && !strings.HasPrefix(start, ".")) {
			continue
		}
		if entry.IsDir() {
			name += "/"
		}
		names = append(names, dir+name)
	}
	return names
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
	e.tabCompletion.list = nil // a round of file names ends with any change to the line
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
	// vim left its command line when the Enter was taken (ExModeKeyHandler).
	// A command that puts up a notice (:help, a listing) or waits for a
	// choice (:suggest) sets the mode for that itself.
	e.mode = NORMAL
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
		// A command that changed the buffer has to be drawn now: dispatch
		// ends the keystroke, so otherwise the edit sits invisible until the
		// next key. Not over a notice the command put up: closing that draws
		// the text.
		return e.resyncFromVim() && e.mode != HELP
	}

	e.command_line = ""
	if decision.action == exRefuse {
		e.ShowMessage("\x1b[41m%s\x1b[0m", decision.reason)
		return false
	}
	redraw = e.forwardEx(line)
	return redraw && e.mode != HELP // an overlay (:marks) is up: draw the text when it closes
}

// resyncFromVim brings the editor's copy of the note into line with vim's --
// the text, and the cursor, kept inside it -- and reports whether the text
// has changed since the last time it was asked: whether there is something
// to draw.
//
// It is the one place that copy is made, and it is called after anything
// that touches vim: a key (followVim), an ex command (forwardEx, runExLine),
// a note read into the buffer (Open, checkStale). The copy is what is drawn;
// nothing may change it but this.
func (e *Editor) resyncFromVim() (changed bool) {
	e.ss = e.vbuf.Lines()
	if len(e.ss) == 0 {
		e.ss = []string{""}
	}
	pos := vim.GetCursorPosition()
	e.fr = min(max(pos[0]-1, 0), len(e.ss)-1)
	e.fc = min(max(pos[1], 0), len(e.ss[e.fr]))
	if tick := e.vbuf.GetLastChangedTick(); tick > e.bufferTick {
		e.bufferTick = tick
		return true
	}
	return false
}
