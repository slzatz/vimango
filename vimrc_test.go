package main

import (
	"testing"

	"github.com/slzatz/vimango/vim"
)

// newVimrcEditor is newTestEditor with vimango.vim sourced, as startup does.
// vim is a process-wide singleton and vimInit does not clear mappings, so
// every test starts from mapclear.
func newVimrcEditor(t *testing.T, lines ...string) *Editor {
	t.Helper()
	e := newTestEditor(t, lines...)
	vim.ExecuteCommand("mapclear | mapclear!")
	sourceVimrc()
	return e
}

func assertLines(t *testing.T, e *Editor, want ...string) {
	t.Helper()
	got := e.vbuf.Lines()
	for i, w := range want {
		if i >= len(got) || got[i] != w {
			t.Fatalf("buffer = %q, want %q", got, want)
		}
	}
}

// The bullet mapping from vimango.vim, typed through editorProcessKey.
func TestVimrcBulletMappingLinewise(t *testing.T) {
	e := newVimrcEditor(t, "alpha", "  bravo", "", "charlie", "delta")

	e.sendKeys("Vjjj-")
	assertLines(t, e, "- alpha", "  - bravo", "", "- charlie", "delta")
	// the mapping's :s ends VISUAL; NORMAL_BUSY is what editorProcessKey
	// records for vim's normal mode after any forwarded key
	if e.mode != NORMAL && e.mode != NORMAL_BUSY {
		t.Errorf("mode after mapping = %v, want normal", e.mode)
	}
	// drawText renders e.ss, so the editor's copy must be current
	if e.ss[0] != "- alpha" {
		t.Errorf("e.ss[0] = %q, the change would not be drawn", e.ss[0])
	}

	// one undo takes the whole list back
	e.sendKeys("u")
	assertLines(t, e, "alpha", "  bravo", "", "charlie", "delta")
}

func TestVimrcBulletMappingBlockwise(t *testing.T) {
	e := newVimrcEditor(t, "alpha", "bravo", "charlie")

	e.sendKeys("\x16jj-") // Ctrl-V
	assertLines(t, e, "- alpha", "- bravo", "- charlie")
}

// Pressing - again on an all-bulleted selection takes the bullets off; a
// mixed selection is completed rather than double-bulleted.
func TestVimrcBulletMappingToggles(t *testing.T) {
	e := newVimrcEditor(t, "alpha", "  bravo", "", "charlie")

	e.sendKeys("Vjjj-")
	assertLines(t, e, "- alpha", "  - bravo", "", "- charlie")

	e.sendKeys("gv-")
	assertLines(t, e, "alpha", "  bravo", "", "charlie")

	// mixed: some lines already bulleted -> bullet the rest, nest nothing
	e2 := newVimrcEditor(t, "- alpha", "bravo", "  - charlie")
	e2.sendKeys("Vjj-")
	assertLines(t, e2, "- alpha", "- bravo", "  - charlie")

	// all blank: nothing to do
	e3 := newVimrcEditor(t, "", "  ", "alpha")
	e3.sendKeys("Vj-")
	assertLines(t, e3, "", "  ", "alpha")

	// a toggle off is one undo step too
	e4 := newVimrcEditor(t, "- alpha", "- bravo")
	e4.sendKeys("Vj-")
	assertLines(t, e4, "alpha", "bravo")
	e4.sendKeys("u")
	assertLines(t, e4, "- alpha", "- bravo")
}

// # is the numbered twin of -: every line gets "1. " (markdown numbers them
// when rendering), and pressing it again takes them off.
func TestVimrcNumberMappingToggles(t *testing.T) {
	e := newVimrcEditor(t, "alpha", "  bravo", "", "charlie")
	e.sendKeys("Vjjj#")
	assertLines(t, e, "1. alpha", "  1. bravo", "", "1. charlie")

	e.sendKeys("gv#")
	assertLines(t, e, "alpha", "  bravo", "", "charlie")

	// any existing number marker counts, not just "1."; mixed is completed
	e2 := newVimrcEditor(t, "3. alpha", "bravo", "12) charlie")
	e2.sendKeys("Vjj#")
	assertLines(t, e2, "3. alpha", "1. bravo", "12) charlie")
	e2.sendKeys("gv#")
	assertLines(t, e2, "alpha", "bravo", "charlie")

	e3 := newVimrcEditor(t, "alpha", "bravo")
	e3.sendKeys("\x16j#") // Ctrl-V
	assertLines(t, e3, "1. alpha", "1. bravo")
	e3.sendKeys("u")
	assertLines(t, e3, "alpha", "bravo")
}

// Each key on a list of the other kind switches it rather than nesting
// markers, and a selection mixing plain, bulleted and numbered lines all ends
// up the requested kind.
func TestVimrcListKindSwitching(t *testing.T) {
	e := newVimrcEditor(t, "- alpha", "  - bravo", "", "- charlie")
	e.sendKeys("Vjjj#")
	assertLines(t, e, "1. alpha", "  1. bravo", "", "1. charlie")

	e.sendKeys("gv-")
	assertLines(t, e, "- alpha", "  - bravo", "", "- charlie")

	// "*" and "+" are bullets too; *italic* at the start of a line is not
	e2 := newVimrcEditor(t, "* alpha", "+ bravo", "*charlie* here", "2. delta")
	e2.sendKeys("Vjjj#")
	assertLines(t, e2, "1. alpha", "1. bravo", "1. *charlie* here", "2. delta")

	e3 := newVimrcEditor(t, "* alpha", "1. bravo", "charlie")
	e3.sendKeys("Vjj-")
	assertLines(t, e3, "* alpha", "- bravo", "- charlie")

	// task items keep their checkbox across a switch
	e4 := newVimrcEditor(t, "- [ ] alpha", "- [x] bravo")
	e4.sendKeys("Vj#")
	assertLines(t, e4, "1. [ ] alpha", "1. [x] bravo")
}

// bulleting must not replace the pattern n searches for.
func TestVimrcBulletMappingKeepsSearchPattern(t *testing.T) {
	e := newVimrcEditor(t, "alpha", "bravo")
	vim.ExecuteCommand("let @/ = 'bravo'")

	e.sendKeys("Vj-")
	if got := vim.EvaluateExpression("@/"); got != "bravo" {
		t.Errorf("@/ = %q after bulleting, want %q", got, "bravo")
	}
}

// Mappings apply to what the user types and to nothing vimango sends itself:
// with "s" mapped to something destructive, a typed s is remapped while an s
// vimango sends through the vim API keeps vim's meaning.
func TestVimrcMappingsDoNotTouchInternalKeys(t *testing.T) {
	e := newVimrcEditor(t, "first", "second")
	vim.ExecuteCommand("nnoremap s dd")

	e.sendKeys("s")
	assertLines(t, e, "second")

	// expansion is off again once the typed key has been handled: plain vim
	// "s" on "second" deletes the 's' and enters INSERT; the mapping would
	// have deleted the line
	vim.SendInput("s")
	vim.SendKey("<esc>")
	assertLines(t, e, "econd")
}

// ---- Ctrl-B / Ctrl-I / Ctrl-E (VimangoDecorate in vimango.vim) ----

// at puts vim's cursor and the editor's copy of it on line 1, byte col.
func (e *Editor) at(col int) *Editor {
	vim.SetCursorPosition(1, col)
	e.ss = e.vbuf.Lines()
	e.fr, e.fc = 0, col
	return e
}

func TestVimrcDecorateWordNormalMode(t *testing.T) {
	e := newVimrcEditor(t, "alpha bravo charlie").at(6) // on "bravo"

	e.editorProcessKey(ctrlKey('b'))
	assertLines(t, e, "alpha **bravo** charlie")

	// same key again toggles it off -- the cursor was parked on the word
	e.editorProcessKey(ctrlKey('b'))
	assertLines(t, e, "alpha bravo charlie")

	// a different style replaces rather than nests
	e.editorProcessKey(ctrlKey('b'))
	e.editorProcessKey(ctrlKey('i'))
	assertLines(t, e, "alpha *bravo* charlie")
	e.editorProcessKey(ctrlKey('e'))
	assertLines(t, e, "alpha `bravo` charlie")
	e.editorProcessKey(ctrlKey('e'))
	assertLines(t, e, "alpha bravo charlie")
}

// The cases the old Go version (ciw / <cword>) got wrong: the cursor sitting
// on a marker rather than on the word.
func TestVimrcDecorateCursorOnMarker(t *testing.T) {
	for _, col := range []int{6, 7, 13, 14} { // each * of "**bravo**"
		e := newVimrcEditor(t, "alpha **bravo** charlie").at(col)
		e.editorProcessKey(ctrlKey('b'))
		assertLines(t, e, "alpha bravo charlie")
	}

	e := newVimrcEditor(t, "say `x`, then").at(4) // on the opening `
	e.editorProcessKey(ctrlKey('e'))
	assertLines(t, e, "say x, then")
}

// Sentence punctuation ends the word; whitespace or punctuation under the
// cursor means there is nothing to decorate.
func TestVimrcDecorateWordBoundaries(t *testing.T) {
	e := newVimrcEditor(t, "(alpha), bravo.").at(2)
	e.editorProcessKey(ctrlKey('b'))
	assertLines(t, e, "(**alpha**), bravo.")

	e2 := newVimrcEditor(t, "alpha bravo").at(5) // the space
	e2.editorProcessKey(ctrlKey('b'))
	assertLines(t, e2, "alpha bravo")

	// non-ASCII letters are word characters
	e3 := newVimrcEditor(t, "un café noir").at(4)
	e3.editorProcessKey(ctrlKey('i'))
	assertLines(t, e3, "un *café* noir")
}

func TestVimrcDecorateVisualSelection(t *testing.T) {
	e := newVimrcEditor(t, "alpha bravo charlie").at(6)
	e.sendKeys("ve") // select "bravo"
	if e.mode != VISUAL {
		t.Fatalf("expected VISUAL, got %v", e.mode)
	}
	redraw := e.editorProcessKey(ctrlKey('b'))
	assertLines(t, e, "alpha **bravo** charlie")
	if !redraw {
		t.Error("visual Ctrl-B returned redraw=false")
	}
	if e.mode == VISUAL {
		t.Error("still VISUAL after the mapping")
	}

	// a phrase, then toggled off by reselecting just the inner text: the
	// markers outside the selection are found
	e2 := newVimrcEditor(t, "say hello world now").at(4)
	e2.sendKeys("vee")
	e2.editorProcessKey(ctrlKey('b'))
	assertLines(t, e2, "say **hello world** now")
	e2.at(6).sendKeys("vee") // "hello world" inside the markers
	e2.editorProcessKey(ctrlKey('b'))
	assertLines(t, e2, "say hello world now")

	// selecting the markers too works the same
	e3 := newVimrcEditor(t, "a *b c* d").at(2)
	e3.sendKeys("v4l")
	e3.editorProcessKey(ctrlKey('i'))
	assertLines(t, e3, "a b c d")

	// V decorates the line's text, not its indentation
	e4 := newVimrcEditor(t, "  whole line  ")
	e4.sendKeys("V")
	e4.editorProcessKey(ctrlKey('e'))
	assertLines(t, e4, "  `whole line`  ")

	// a selection over two lines is left alone
	e5 := newVimrcEditor(t, "alpha", "bravo")
	e5.sendKeys("vj")
	e5.editorProcessKey(ctrlKey('b'))
	assertLines(t, e5, "alpha", "bravo")
}

// One decoration is one undo step.
func TestVimrcDecorateUndo(t *testing.T) {
	e := newVimrcEditor(t, "alpha bravo").at(0)
	e.editorProcessKey(ctrlKey('b'))
	assertLines(t, e, "**alpha** bravo")
	e.sendKeys("u")
	assertLines(t, e, "alpha bravo")
}

// The editor intercepts no normal-mode keys. Ctrl-H and Ctrl-L were the last
// two (previous/next editor, from when several notes could be open); now they
// reach vim, where Ctrl-H moves left like h and Ctrl-L changes nothing.
func TestEditorInterceptsNoNormalKeys(t *testing.T) {
	e := newTestEditor(t, "alpha bravo")
	vim.SetCursorPosition(1, 3)

	e.editorProcessKey(ctrlKey('h'))
	if got := vim.GetCursorPosition(); got != [2]int{1, 2} {
		t.Errorf("after Ctrl-H: cursor = %v, want [1 2] (vim's ^H is h)", got)
	}
	e.editorProcessKey(ctrlKey('l'))
	assertLines(t, e, "alpha bravo")
	if !hostOf(e).open() {
		t.Error("a key in the editor closed the editor")
	}
}

// A key that ends VISUAL without changing text must still redraw, or the
// selection highlight stays on screen.
func TestLeavingVisualWithoutChangeRedraws(t *testing.T) {
	e := newVimrcEditor(t, "alpha", "bravo")
	e.sendKeys("vj")
	if redraw := e.editorProcessKey(ctrlKey('b')); !redraw { // two lines: no-op
		t.Error("no-op mapping out of VISUAL returned redraw=false")
	}

	e2 := newTestEditor(t, "alpha bravo")
	e2.sendKeys("ve")
	if redraw := e2.editorProcessKey('y'); !redraw {
		t.Error("visual y returned redraw=false")
	}
}

// A missing vimango.vim is normal and must not disturb anything.
func TestVimrcMissingFileIsSilent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	e := newVimrcEditor(t, "alpha")
	e.sendKeys("V-")
	assertLines(t, e, "alpha")
}

// ---- Enter in INSERT mode on a list item (VimangoListEnter in vimango.vim) ----

func TestVimrcEnterContinuesAList(t *testing.T) {
	const esc = "\x1b"
	cases := []struct {
		name  string
		setup string // an ex command to run first
		lines []string
		keys  string
		want  []string
	}{
		{"bullets", "", []string{"- foo"}, "A\rbar\rbaz" + esc, []string{"- foo", "- bar", "- baz"}},
		{"numbers", "", []string{"1. foo"}, "A\rbar\rbaz" + esc, []string{"1. foo", "1. bar", "1. baz"}},
		{"the marker as written", "", []string{"2) foo"}, "A\rbar" + esc, []string{"2) foo", "2) bar"}},
		{"star bullet", "", []string{"* foo"}, "A\rbar" + esc, []string{"* foo", "* bar"}},
		{"a list typed from nothing", "", []string{""}, "i- one\rtwo\r\rafter" + esc, []string{"- one", "- two", "after"}},
		{"nested", "", []string{"- top", "  - foo"}, "jA\rbar" + esc, []string{"- top", "  - foo", "  - bar"}},
		{"nested, noautoindent", "set noautoindent", []string{"  1. foo"}, "A\rbar" + esc, []string{"  1. foo", "  1. bar"}},
		{"tab indent", "", []string{"\t- foo"}, "A\rbar" + esc, []string{"\t- foo", "\t- bar"}},
		{"an empty item ends the list", "", []string{"- foo"}, "A\rbar\r\rplain\rnext" + esc, []string{"- foo", "- bar", "plain", "next"}},
		{"an empty nested item ends the list", "", []string{"  - foo"}, "A\r\rplain" + esc, []string{"  - foo", "plain"}},
		{"an empty item that was already there", "", []string{"- foo", "- "}, "jA\rplain" + esc, []string{"- foo", "plain"}},
		{"an empty item with trailing blanks", "", []string{"1.   "}, "A\rplain" + esc, []string{"plain"}},
		{"in the middle of an item", "", []string{"- alpha bravo"}, "wwi\r" + esc, []string{"- alpha ", "- bravo"}},
		{"inside the marker", "", []string{"- foo"}, "i\r" + esc, []string{"", "- foo"}},
		// noautoindent: with it on, vim itself takes the indentation off text
		// moved down from column 0
		{"inside the indentation", "set noautoindent", []string{"  - foo"}, "i\r" + esc, []string{"", "  - foo"}},
		{"not a list", "", []string{"plain"}, "A\rtext" + esc, []string{"plain", "text"}},
		{"italic is not a bullet", "", []string{"*word* more"}, "A\rtext" + esc, []string{"*word* more", "text"}},
		{"a rule is not a bullet", "", []string{"---"}, "A\rtext" + esc, []string{"---", "text"}},
		{"a number without a space", "", []string{"3.14 is pi"}, "A\rtext" + esc, []string{"3.14 is pi", "text"}},
		{":paste turns it off", "set paste", []string{"- foo"}, "A\r- bar" + esc, []string{"- foo", "- bar"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newVimrcEditor(t, c.lines...)
			t.Cleanup(func() {
				vim.SendKey("<esc>")
				vim.ExecuteCommand("set nopaste autoindent")
			})
			if c.setup != "" {
				vim.ExecuteCommand(c.setup)
			}
			e.sendKeys(c.keys)
			got := e.vbuf.Lines()
			if len(got) != len(c.want) {
				t.Fatalf("buffer = %q, want %q", got, c.want)
			}
			assertLines(t, e, c.want...)
		})
	}
}

// Ending a list leaves the editor in INSERT mode on the emptied line, with
// vim on the note's buffer, and what was typed can be undone.
func TestVimrcEnterEndingAListStaysInInsert(t *testing.T) {
	e := newVimrcEditor(t, "- foo")

	e.sendKeys("A\rbar\r")
	assertLines(t, e, "- foo", "- bar", "- ")
	e.sendKeys("\r")
	if e.mode != INSERT || vim.GetCurrentMode() != vim.ModeInsert {
		t.Errorf("after ending the list: mode %v, vim %v, want INSERT", e.mode, vim.GetCurrentMode())
	}
	if got := vim.GetCursorPosition(); got != [2]int{3, 0} {
		t.Errorf("cursor = %v, want [3 0]", got)
	}
	if got := e.vbuf.Lines(); len(got) != 3 || got[2] != "" {
		t.Fatalf("buffer = %q, want the third line empty", got)
	}
	e.sendKeys("x\x1b")
	assertLines(t, e, "- foo", "- bar", "x")

	for i := 0; i < 5 && len(e.vbuf.Lines()) > 1; i++ {
		e.sendKeys("u")
	}
	if got := e.vbuf.Lines(); len(got) != 1 || got[0] != "- foo" {
		t.Errorf("after undo: %q, want only the first line", got)
	}
}
