package editor

import (
	"strings"

	"github.com/slzatz/vimango/internal/pane"
	"github.com/slzatz/vimango/vim"
)

// ":s/old/new/c" asks before each change, and it is vim that asks: its own
// substitute loop, in libvim, which stops at every match for a key. vim
// would show the match and wait at its keyboard. libvim has neither, so it
// asks whoever it was told to (vim.SetSubstituteConfirm; Open says the
// editor), from the middle of the command -- the call that ran it has not
// returned, and does not until the last match is answered.
//
// So this is the one place the editor waits for a key itself instead of
// being handed one: it shows the match, writes the question, and asks its
// host for the next key (Host.WaitKey). Everything else about the command is
// vim's -- which text matches, what "&" and "\=" come to, 'a' and 'l', the
// one undo step for all of it -- and works by whatever route the command
// came: typed, from a mapping, under :g, in a function.
//
// What is on screen meanwhile is what vim would show: the changes accepted
// so far, the match marked, the cursor on it.

// askedStyle marks the match being asked about: black on orange, over the
// yellow the other matches of the pattern have (searchStyle).
const askedStyle = "\x1b[30;48;5;208m"

// confirmSubstitute is asked by vim, for one match of ":s" with the c flag,
// and returns the key the user answers with. vim takes y, n, a, l, q, <esc>
// and CTRL-C, and asks again for anything else.
//
// It runs inside a call into vim, so it reads from vim and sends it nothing:
// no key, no command.
func (e *Editor) confirmSubstitute(m vim.SubstituteMatch) int {
	// Nobody to ask -- no host that can wait for a key, or vim is on some
	// buffer that is not the note's -- is <esc>: nothing more is changed.
	if e.WaitKey == nil || !e.onItsBuffer() {
		return '\x1b'
	}
	was := e.mode
	e.mode = CONFIRM
	e.asked = &m
	e.askedShown = true // the mark is on screen: the text is drawn again when the command is over (redrawAfter)
	defer func() {
		e.mode, e.asked = was, nil
		e.ShowMessage("") // the question; what vim says of the command comes after
	}()

	// As for Scroll: vim's text and cursor as they are in the middle of the
	// command, and whether the text changed left for the resync after it.
	e.copyFromVim()
	e.followCursor()
	e.drawText()
	e.drawStatusBar()
	e.drawModeLine()

	for {
		switch c := e.WaitKey(); {
		case c == NoKey:
			return '\x1b' // no key is coming
		case c > 0 && !pane.IsSyntheticKey(c):
			return c
		}
		// an arrow, a function key: not an answer, and not vim's to hear of
	}
}

// question is what vim writes while it waits: the replacement as it was
// typed, and the keys that answer. (vim lists ^E and ^Y too, which scroll;
// here they are not taken.) "" when nothing is being asked.
func (e *Editor) question() string {
	if e.asked == nil {
		return ""
	}
	return "replace with " + expandRow(e.asked.Replacement, "    ") + " (y/n/a/q/l)?"
}

// showQuestion writes the question on the message line.
func (e *Editor) showQuestion() {
	if q := e.question(); q != "" {
		e.ShowMessage("%s", q)
	}
}

// questionCursor is where the terminal's cursor waits while the question
// stands: after it, on the message line, as in vim -- the match is marked in
// the text, and the answer is typed here.
func (e *Editor) questionCursor() (row, col int) {
	_, cols := pane.ClipColumns(e.question(), 0, e.Message.Width)
	return e.Message.Row, e.Message.Col + min(cols, max(e.Message.Width-1, 0))
}

// drawAskedMatch marks the match being asked about. From drawText, after the
// search matches, which it is one of.
func (e *Editor) drawAskedMatch(pab *strings.Builder) {
	if e.asked == nil || len(e.ss) == 0 {
		return
	}
	first, last, ys := e.visibleRowStarts()
	if first >= last {
		return
	}
	pab.WriteString(askedStyle)
	e.highlightSpan(pab, e.asked.Line, e.asked.Col, e.asked.EndLine, e.asked.EndCol, first, last, ys)
	pab.WriteString(sgrReset)
}
