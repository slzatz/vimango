package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/slzatz/vimango/vim"
	"github.com/slzatz/vimango/vim/cvim"
)

// Spell checking is vim's own, from libvim (restored in the slzatz/libvim
// fork): libvim finds the misspellings and vimango only draws them. vim's
// normal-mode commands work as typed -- ]s [s zg zw zG zW zug {n}z= -- and z=
// without a count shows a numbered list on the message line (SPELLING mode).
//
// The dictionary lives in spell/ next to config.json, like vimango.vim;
// scripts/build-libvim.sh copies en.utf-8.spl/.sug there from the fork's
// runtime/spell. zg/zw add words to spell/en.utf-8.add, which vim also
// compiles to en.utf-8.add.spl beside it.
const (
	spellDir      = "spell"
	spellDictFile = "en.utf-8.spl"
	spellAddFile  = "en.utf-8.add"
)

var (
	spellAvailable bool // spell/en.utf-8.spl was found at startup
	spellOn        bool // :spell toggles; vim's 'spell' follows (see syncSpellOption)
)

// initSpell points libvim at spell/ and sets the spell options. Called before
// vimango.vim is sourced, so the user can override any of them there.
func initSpell() {
	root, err := filepath.Abs(".")
	if err != nil {
		return
	}
	if _, err := os.Stat(filepath.Join(root, spellDir, spellDictFile)); err != nil {
		return
	}
	spellAvailable = true

	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	vim.ExecuteCommand("let &runtimepath = " + quote(root) + " . ',' . &runtimepath")
	vim.ExecuteCommand("let &spellfile = " + quote(filepath.Join(root, spellDir, spellAddFile)))
	// 9 suggestions: each is picked with a single digit in SPELLING mode
	vim.ExecuteCommand("set spelllang=en spellsuggest=best,9")
}

func spellUnavailableMessage() string {
	return fmt.Sprintf("%sSpell check unavailable: %s/%s not found (scripts/build-libvim.sh copies it)%s",
		RED_BG, spellDir, spellDictFile, RESET)
}

// syncSpellOption makes vim's 'spell' match spellOn. 'spell' is window-local
// and vim restores a buffer's old window options when it becomes current
// again, so it can drift from spellOn when buffers are switched.
func syncSpellOption() {
	want := "0"
	if spellOn {
		want = "1"
	}
	if vim.EvaluateExpression("&spell") == want {
		return
	}
	if spellOn {
		vim.ExecuteCommand("set spell")
	} else {
		vim.ExecuteCommand("set nospell")
	}
}

// :spell toggles spell checking. While it is on, misspellings are highlighted
// as the note is edited.
func (e *Editor) toggleSpell() {
	if !spellAvailable {
		e.ShowMessage(BR, "%s", spellUnavailableMessage())
		return
	}
	spellOn = !spellOn
	syncSpellOption()
	e.drawText()
	if spellOn {
		e.ShowMessage(BR, "Spell checking is on")
	} else {
		e.ShowMessage(BR, "Spell checking is off")
	}
}

// drawSpellErrors highlights the misspellings in the visible rows: bad words
// with a background, missing capitals and rare or regional words underlined.
// In INSERT mode the word being typed is left alone, as vim does.
func (e *Editor) drawSpellErrors(pab *strings.Builder) {
	if !spellOn || len(e.ss) == 0 {
		return
	}
	syncSpellOption()

	first := e.firstVisibleRow
	last := first + e.screenlines // every row takes at least one screen line
	if last > len(e.ss) {
		last = len(e.ss)
	}
	for _, se := range e.vbuf.SpellErrors(first+1, last) {
		r := se.Line - 1
		if r < 0 || r >= len(e.ss) {
			continue
		}
		row := e.ss[r]
		start, end := se.Col, se.Col+se.Len
		if start < 0 || end > len(row) {
			continue // e.ss out of step with vim's buffer
		}
		if e.mode == INSERT && r == e.fr && e.fc >= start && e.fc <= end {
			continue
		}
		style := "\x1b[48;5;31m"
		if se.Type != cvim.SpellBad {
			style = "\x1b[4m"
		}
		y := e.getScreenYFromRowColWW(r, start) + e.top_margin - e.lineOffset
		x := e.getScreenXFromRowColWW(r, start) + e.left_margin + e.left_margin_offset
		if y >= e.top_margin && y <= e.screenlines {
			fmt.Fprintf(pab, "\x1b[%d;%dH\x1b[0m", y, x+1)
			fmt.Fprint(pab, style+row[start:end]+"\x1b[0m")
		}
	}
}

// spellSuggest lists the suggestions for the word z= would change -- the
// misspelling under or after the cursor, else the word under it -- and enters
// SPELLING mode, where a digit picks one. Reached from z= and :suggest.
func (e *Editor) spellSuggest() {
	if !spellAvailable {
		e.ShowMessage(BR, "%s", spellUnavailableMessage())
		return
	}
	if !spellOn {
		e.ShowMessage(BR, "Spell checking is off; :spell turns it on")
		return
	}
	syncSpellOption()

	word, suggestions, ok := vim.GetSpellSuggestions()
	if !ok {
		e.ShowMessage(BR, "No word under the cursor")
		return
	}
	if len(suggestions) == 0 {
		e.ShowMessage(BR, "No suggestions for %q", word)
		return
	}

	e.suggestions = suggestions
	e.mode = SPELLING
	var b strings.Builder
	fmt.Fprintf(&b, "%q ->", word)
	for i, s := range suggestions {
		fmt.Fprintf(&b, "  \x1b[1m%d\x1b[0m %s", i+1, s)
	}
	b.WriteString("   (any other key cancels)")
	e.ShowMessage(BR, "%s", b.String())
}

// case SPELLING: a digit replaces the word with that suggestion, by sending
// vim "{n}z=" (so undo, . and :spellrepall work); any other key cancels.
// <esc> never gets here: editorProcessKey handles it first.
func (e *Editor) SpellingModeKeyHandler(c int) (redraw, exit bool) {
	e.mode = NORMAL
	e.ShowMessage(BR, "")
	n := c - '0'
	if n < 1 || n > len(e.suggestions) {
		return false, true
	}

	vim.SendInput(strconv.Itoa(n))
	vim.SendInput("z")
	vim.SendInput("=")

	e.bufferTick = e.vbuf.GetLastChangedTick()
	e.ss = e.vbuf.Lines()
	if len(e.ss) == 0 {
		e.ss = []string{""}
	}
	pos := vim.GetCursorPosition()
	e.fr = min(max(pos[0]-1, 0), len(e.ss)-1)
	e.fc = min(pos[1], len(e.ss[e.fr]))
	return true, true
}

// spellListKey reports whether key c completes a command that changes the
// word lists (zg zw zG zW, or zu followed by one of those), given the keys
// typed before it. Those change what is misspelled without changing the
// buffer, so editorProcessKey has to redraw for them itself.
func spellListKey(before string, c int) bool {
	if !strings.ContainsRune("gwGW", rune(c)) {
		return false
	}
	return strings.HasSuffix(before, "z") || strings.HasSuffix(before, "zu")
}
