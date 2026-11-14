package editor

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// newSpellEditor is newTestEditor with spell checking on, using the dictionary
// in spell/ (copied there by scripts/build-libvim.sh). zg and friends write to
// a temp file, never to spell/en.utf-8.add.
func newSpellEditor(t *testing.T, lines ...string) *Editor {
	t.Helper()
	if _, err := os.Stat(spellDir + "/" + spellDictFile); err != nil {
		t.Skip("spell/en.utf-8.spl not found; run scripts/build-libvim.sh")
	}
	e := newTestEditor(t, lines...)
	InitSpell()
	vim.ExecuteCommand("let &spellfile = tempname() . '.utf-8.add'")
	spellOn = true
	syncSpellOption()
	t.Cleanup(func() {
		spellOn = false
		syncSpellOption()
	})
	return e
}

func suggestionNumber(e *Editor, word string) int {
	for i, s := range e.suggestions {
		if s == word {
			return i + 1
		}
	}
	return 0
}

// z= puts up the numbered list and a digit applies that suggestion through
// vim's own "{n}z=", so the editor's copy of the text is resynced and redrawn.
func TestZEqualsThenDigitReplacesWord(t *testing.T) {
	e := newSpellEditor(t, "Hello wrold here.")
	vim.SetCursorPosition(1, 6)

	e.sendKeys("z=")
	if e.mode != SPELLING {
		t.Fatalf("after z= mode = %v, want SPELLING", e.mode)
	}
	n := suggestionNumber(e, "would")
	if n < 2 {
		t.Fatalf("\"would\" not offered after the first suggestion: %v", e.suggestions)
	}

	if redraw := e.ProcessKey('0' + n); !redraw {
		t.Error("choosing a suggestion returned redraw=false")
	}
	if e.mode != NORMAL {
		t.Errorf("after choosing, mode = %v, want NORMAL", e.mode)
	}
	if got, want := e.ss[0], "Hello would here."; got != want {
		t.Errorf("e.ss[0] = %q, want %q", got, want)
	}

	e.sendKeys("u") // vim's undo covers the replacement
	if got, want := e.line(0), "Hello wrold here."; got != want {
		t.Errorf("after u: %q, want %q", got, want)
	}
}

// Any key that is not a listed number cancels, and is not passed on to vim.
func TestZEqualsOtherKeyCancels(t *testing.T) {
	e := newSpellEditor(t, "Hello wrold here.")
	vim.SetCursorPosition(1, 6)

	e.sendKeys("z=x")
	if e.mode != NORMAL {
		t.Errorf("mode = %v, want NORMAL", e.mode)
	}
	if got, want := e.line(0), "Hello wrold here."; got != want {
		t.Errorf("the cancelling x reached vim: %q, want %q", got, want)
	}
}

// With a count, z= is vim's: it replaces the word directly, no list.
func TestCountZEqualsAppliesDirectly(t *testing.T) {
	e := newSpellEditor(t, "Hello wrold here.")
	vim.SetCursorPosition(1, 6)

	e.sendKeys("1z=")
	// NORMAL_BUSY is what libvim reports after almost any normal command
	if e.mode != NORMAL && e.mode != NORMAL_BUSY {
		t.Errorf("mode = %v, want NORMAL", e.mode)
	}
	if got, want := e.ss[0], "Hello world here."; got != want {
		t.Errorf("e.ss[0] = %q, want %q", got, want)
	}
}

// :suggest is z= by name, and must not be dropped back to NORMAL by the ex
// dispatcher before the choice is made.
func TestSuggestExCommandWaitsForChoice(t *testing.T) {
	e := newSpellEditor(t, "Hello wrold here.")
	vim.SetCursorPosition(1, 6)

	e.sendKeys(":suggest\r")
	if e.mode != SPELLING {
		t.Fatalf("after :suggest mode = %v, want SPELLING", e.mode)
	}
	e.ProcessKey('1')
	if got, want := e.line(0), "Hello world here."; got != want {
		t.Errorf("after 1: %q, want %q", got, want)
	}
}

// zg changes what is misspelled but not the buffer, so the buffer tick cannot
// trigger the redraw that removes the highlight.
func TestZgRequestsRedraw(t *testing.T) {
	e := newSpellEditor(t, "Hello wrold here.")
	vim.SetCursorPosition(1, 6)

	e.ProcessKey('z')
	if redraw := e.ProcessKey('G'); !redraw {
		t.Error("zG returned redraw=false; the highlight would stay")
	}
	if errs := e.vbuf.SpellErrors(1, 1); len(errs) != 0 {
		t.Errorf("after zG still misspelled: %+v", errs)
	}
}

func TestSpellListKey(t *testing.T) {
	for _, tc := range []struct {
		before string
		c      rune
		want   bool
	}{
		{"z", 'g', true}, {"z", 'W', true}, {"zu", 'w', true}, {"az", 'G', true},
		{"", 'g', false}, {"d", 'w', false}, {"z", 'z', false}, {"zu", 'x', false},
	} {
		if got := spellListKey(tc.before, int(tc.c)); got != tc.want {
			t.Errorf("spellListKey(%q, %q) = %v, want %v", tc.before, tc.c, got, tc.want)
		}
	}
}

// Bad words get a background and the rest an underline; in INSERT mode the
// word at the cursor is the one being typed and is left alone.
func TestDrawSpellErrors(t *testing.T) {
	e := newSpellEditor(t, "Hello wrold here.", "next line lowercase")

	var b strings.Builder
	e.drawSpellErrors(&b)
	out := b.String()
	if !strings.Contains(out, "\x1b[48;5;31mwrold\x1b[0m") {
		t.Errorf("bad word not highlighted: %q", out)
	}
	if !strings.Contains(out, "\x1b[4mnext\x1b[0m") {
		t.Errorf("missing capital not underlined: %q", out)
	}

	e.mode = INSERT
	e.fr, e.fc = 0, 8
	b.Reset()
	e.drawSpellErrors(&b)
	if strings.Contains(b.String(), "wrold") {
		t.Errorf("word under the cursor highlighted in INSERT mode: %q", b.String())
	}

	spellOn = false
	b.Reset()
	e.drawSpellErrors(&b)
	if b.Len() != 0 {
		t.Errorf("drew with spell off: %q", b.String())
	}
}

// A misspelling is marked on every line of the pane and on none below it,
// wherever the pane starts. The test for "on the pane" compared a screen row,
// top margin included, with the number of lines: right when the text starts
// on row 1 (--editor), one line short when it starts on row 2 (the TUI),
// where a misspelling on the pane's last line went unmarked.
func TestSpellErrorsAreMarkedToTheLastLineOfThePane(t *testing.T) {
	const marked = "\x1b[48;5;31mwrold\x1b[0m"
	for _, top := range []int{1, 2, 5} {
		e := newSpellEditor(t, "one wrold", "two wrold", "three wrold", "four wrold")
		e.TopMargin = top
		e.ScreenLines = 3

		var b strings.Builder
		e.drawSpellErrors(&b)
		out := b.String()
		if n := strings.Count(out, marked); n != 3 {
			t.Errorf("text from row %d: %d of the pane's 3 lines marked: %q", top, n, out)
		}
		if last := fmt.Sprintf("\x1b[%d;", top+2); !strings.Contains(out, last) {
			t.Errorf("text from row %d: nothing marked on the pane's last line, row %d: %q", top, top+2, out)
		}

		// a row that wraps onto all three lines pushes the next one off the pane
		e = newSpellEditor(t, strings.TrimSpace(strings.Repeat("some plain words ", 7)), "five wrold")
		e.TopMargin = top
		e.ScreenLines = 3
		if n := e.getLinesInRowWW(0); n != 3 {
			t.Fatalf("the long row takes %d lines, want 3", n)
		}
		b.Reset()
		e.drawSpellErrors(&b)
		if strings.Contains(b.String(), "wrold") {
			t.Errorf("text from row %d: marked a word below the pane: %q", top, b.String())
		}
	}
}

// 'spelloptions' noallcaps (the libvim fork's, set in vimango.vim): an unknown
// word in capitals only is neither highlighted nor a stop for ]s.
func TestNoAllCapsSkipsAcronyms(t *testing.T) {
	e := newSpellEditor(t, "The ZXQV report wrold ZXQV's.")
	vim.ExecuteCommand("set spelloptions=noallcaps") // not nomixedcase: ZXQV's stays bad
	t.Cleanup(func() { vim.ExecuteCommand("set spelloptions=") })

	var b strings.Builder
	e.drawSpellErrors(&b)
	out := b.String()
	if strings.Contains(out, "mZXQV\x1b[0m") {
		t.Errorf("acronym highlighted: %q", out)
	}
	if !strings.Contains(out, "\x1b[48;5;31mwrold\x1b[0m") {
		t.Errorf("bad word not highlighted: %q", out)
	}
	if !strings.Contains(out, "\x1b[48;5;31mZXQV's\x1b[0m") {
		t.Errorf("word with a lower-case letter not highlighted: %q", out)
	}

	vim.SetCursorPosition(1, 0)
	e.sendKeys("]s")
	if got := vim.GetCursorPosition(); got != [2]int{1, 16} {
		t.Errorf("]s went to %v, want [1 16] (wrold)", got)
	}
}

// nomixedcase: a capital after the first letter (iOS, NASA's) is a name.
func TestNoMixedCaseSkipsMixedCaseWords(t *testing.T) {
	e := newSpellEditor(t, "The zXqv ZXQV's report Zxqv wrold.")
	vim.ExecuteCommand("set spelloptions=nomixedcase")
	t.Cleanup(func() { vim.ExecuteCommand("set spelloptions=") })

	var words []string
	for _, se := range e.vbuf.SpellErrors(1, 1) {
		words = append(words, e.ss[0][se.Col:se.Col+se.Len])
	}
	if got, want := strings.Join(words, " "), "Zxqv wrold"; got != want {
		t.Errorf("misspelled = %q, want %q", got, want)
	}

	vim.SetCursorPosition(1, 0)
	e.sendKeys("]s")
	if got := vim.GetCursorPosition(); got != [2]int{1, 23} {
		t.Errorf("]s went to %v, want [1 23] (Zxqv)", got)
	}
}
