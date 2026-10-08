package main

import (
	"strings"
	"testing"

	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// newTestEditor builds the smallest Editor editorProcessKey will drive: a real
// vim buffer plus both command tables, so the whole interception layer is in
// the path.
func newTestEditor(t *testing.T, lines ...string) *Editor {
	t.Helper()
	vim.InitializeVim(0)
	a := &App{}
	sess := &Session{}
	scr := &Screen{screenCols: 100, screenLines: 40, textLines: 30, divider: 40, Session: sess}
	e := &Editor{Session: sess, Screen: scr}
	e.screencols = 50
	e.screenlines = 30
	e.normalCmds = a.setEditorNormalCmds(e)
	e.exCmds = a.setEditorExCmds(e)
	// vim is a process-wide singleton, so a previous test may have left it in
	// INSERT or VISUAL; start every editor from NORMAL
	vim.SendKey("<esc>")
	e.vbuf = vim.NewBuffer(0)
	vim.SetCurrentBuffer(e.vbuf)
	e.vbuf.SetLines(0, -1, lines)
	vim.SetCursorPosition(1, 0)
	e.ss = e.vbuf.Lines()
	e.mode = NORMAL
	return e
}

func (e *Editor) sendKeys(s string) {
	for _, r := range s {
		e.editorProcessKey(int(r))
	}
}

func (e *Editor) line(n int) string { return e.vbuf.Lines()[n] }

// The leader used to swallow <space> unconditionally, so it never reached vim.
// vim defines <Space> as "l", and a vim-like editor should honour that.
func TestSpaceIsAMotionNotALeader(t *testing.T) {
	e := newTestEditor(t, "alpha bravo")

	e.editorProcessKey(' ')
	if got := vim.GetCursorPosition(); got != [2]int{1, 1} {
		t.Errorf("after <space>: cursor = %v, want [1 1]", got)
	}

	// the old matcher also swallowed the key *after* an unmatched leader
	e.editorProcessKey(' ')
	e.editorProcessKey('x') // delete char under cursor
	if got, want := e.line(0), "alha bravo"; got != want {
		t.Errorf("after <space><space>x: %q, want %q", got, want)
	}
}

func TestDecorateWordNormalMode(t *testing.T) {
	e := newTestEditor(t, "alpha bravo charlie")
	vim.SetCursorPosition(1, 6) // on "bravo"
	e.ss = e.vbuf.Lines()
	e.fr, e.fc = 0, 6

	e.editorProcessKey(ctrlKey('b'))
	if got, want := e.line(0), "alpha **bravo** charlie"; got != want {
		t.Fatalf("Ctrl-B: %q, want %q", got, want)
	}

	// same style again toggles it back off
	e.editorProcessKey(ctrlKey('b'))
	if got, want := e.line(0), "alpha bravo charlie"; got != want {
		t.Errorf("Ctrl-B twice: %q, want %q", got, want)
	}

	// a different style replaces rather than nests
	e.editorProcessKey(ctrlKey('b'))
	e.editorProcessKey(ctrlKey('i'))
	if got, want := e.line(0), "alpha *bravo* charlie"; got != want {
		t.Errorf("Ctrl-B then Ctrl-I: %q, want %q", got, want)
	}
}

func TestDecorateVisualSelection(t *testing.T) {
	e := newTestEditor(t, "alpha bravo charlie")
	vim.SetCursorPosition(1, 6)
	e.ss = e.vbuf.Lines()
	e.fr, e.fc = 0, 6

	e.sendKeys("ve") // select "bravo"
	if e.mode != VISUAL {
		t.Fatalf("expected VISUAL, got %v", e.mode)
	}
	e.editorProcessKey(ctrlKey('b'))
	if got, want := e.line(0), "alpha **bravo** charlie"; got != want {
		t.Errorf("visual Ctrl-B: %q, want %q", got, want)
	}
}

// :bold with no range decorates the word under the cursor; with the "'<,'>"
// range vim writes when ':' is pressed from VISUAL mode, it decorates the
// selection. The '< and '> marks survive the <esc> that cancels vim's cmdline.
func TestBoldExCommand(t *testing.T) {
	e := newTestEditor(t, "alpha bravo charlie")
	vim.SetCursorPosition(1, 6)
	e.ss = e.vbuf.Lines()
	e.fr, e.fc = 0, 6

	e.sendKeys(":bold")
	e.editorProcessKey('\r')
	if got, want := e.line(0), "alpha **bravo** charlie"; got != want {
		t.Fatalf(":bold with no range: %q, want %q", got, want)
	}

	e2 := newTestEditor(t, "alpha bravo charlie")
	vim.SetCursorPosition(1, 6)
	e2.ss = e2.vbuf.Lines()
	e2.fr, e2.fc = 0, 6
	e2.sendKeys("ve") // select "bravo"
	e2.editorProcessKey(':')
	if got := vim.CommandLineGetText(); got != "'<,'>" {
		t.Fatalf("after ':' from visual, cmdline = %q, want %q", got, "'<,'>")
	}
	e2.sendKeys("bold")
	e2.editorProcessKey('\r')
	if got, want := e2.line(0), "alpha **bravo** charlie"; got != want {
		t.Errorf(":'<,'>bold: %q, want %q", got, want)
	}
}

// A buffer-changing ex command must ask the caller to redraw. MainLoop only
// calls drawText when editorProcessKey returns true, and dispatch ends the
// keystroke, so returning false leaves the edit invisible until some later key
// trips the tick check -- ":bold" showed the old text until you pressed l.
func TestBufferChangingExCommandRequestsRedraw(t *testing.T) {
	e := newTestEditor(t, "alpha bravo charlie")
	vim.SetCursorPosition(1, 6)
	e.ss = e.vbuf.Lines()
	e.fr, e.fc = 0, 6

	e.sendKeys(":bold")
	if redraw := e.editorProcessKey('\r'); !redraw {
		t.Error(":bold returned redraw=false; the change would not be drawn")
	}
	// and the editor's own copy of the text must be current, since drawText
	// renders e.ss rather than re-reading the buffer
	if got, want := e.ss[0], "alpha **bravo** charlie"; got != want {
		t.Errorf("e.ss[0] = %q, want %q", got, want)
	}

	// a command that changes nothing should not force a redraw
	e2 := newTestEditor(t, "alpha bravo")
	e2.sendKeys(":number")
	if redraw := e2.editorProcessKey('\r'); redraw {
		t.Error(":number returned redraw=true but changed no text")
	}
}

// Keys with no character (F-keys, Ins) arrive as synthetic codes above any
// real rune. Only the ones in termcodes mean anything to vim; forwarding any
// other one types its code point as text, because string(rune(1011)) is the
// Greek letter that 1011 happens to name. F1 and F2 used to collide with
// constants named NOP and SHIFT_TAB, and F3 upward had no name here at all.
func TestSyntheticKeysAreNotForwarded(t *testing.T) {
	unmapped := []struct {
		name string
		code int
	}{
		{"F1", terminal.KeyF1},
		{"F3", terminal.KeyF3},
		{"F12", terminal.KeyF12},
		{"Ins", terminal.KeyIns},
	}

	for _, k := range unmapped {
		if _, mapped := termcodes[k.code]; mapped {
			t.Errorf("%s is in termcodes; this test assumes it is not", k.name)
		}

		e := newTestEditor(t, "alpha")
		e.editorProcessKey('i') // INSERT, where a forwarded key becomes text
		e.editorProcessKey(k.code)
		if got := e.vbuf.Lines()[0]; got != "alpha" {
			t.Errorf("%s in INSERT changed the buffer to %q", k.name, got)
		}

		e2 := newTestEditor(t, "alpha")
		e2.sendKeys(":")
		e2.editorProcessKey(k.code)
		if e2.command_line != "" {
			t.Errorf("%s in EX put %q on the command line", k.name, e2.command_line)
		}
	}

	// the keys vim does understand must still get through
	for _, k := range []struct {
		name string
		code int
	}{
		{"Home", HOME_KEY},
		{"PageUp", PAGE_UP},
		{"ArrowLeft", ARROW_LEFT},
	} {
		if _, mapped := termcodes[k.code]; !mapped {
			t.Errorf("%s (%d) is no longer in termcodes", k.name, k.code)
		}
	}
}

// main's key codes are aliases for the reader's, not a second hand-maintained
// copy -- the two blocks had already drifted once.
func TestKeyConstantsTrackTheReader(t *testing.T) {
	for _, tc := range []struct {
		name       string
		main, term int
	}{
		{"ARROW_LEFT", ARROW_LEFT, terminal.KeyArrowLeft},
		{"DEL_KEY", DEL_KEY, terminal.KeyDelete},
		{"HOME_KEY", HOME_KEY, terminal.KeyHome},
		{"END_KEY", END_KEY, terminal.KeyEnd},
		{"PAGE_DOWN", PAGE_DOWN, terminal.KeyPageDown},
	} {
		if tc.main != tc.term {
			t.Errorf("%s = %d but reader says %d", tc.name, tc.main, tc.term)
		}
	}
	if isSyntheticKey(BACKSPACE) {
		t.Error("BACKSPACE is a real byte and must not be treated as synthetic")
	}
	if !isSyntheticKey(terminal.KeyF1) {
		t.Error("F1 should be treated as synthetic")
	}
}

func TestSplitExRange(t *testing.T) {
	for _, tc := range []struct{ in, rng, rest string }{
		{"bold", "", "bold"},
		{"'<,'>bold", "'<,'>", "bold"},
		{"%s/a/b/", "%", "s/a/b/"},
		{"'<,'>s/a/b/", "'<,'>", "s/a/b/"},
		{"write", "", "write"},
	} {
		rng, rest := splitExRange(tc.in)
		if rng != tc.rng || rest != tc.rest {
			t.Errorf("splitExRange(%q) = (%q, %q), want (%q, %q)", tc.in, rng, rest, tc.rng, tc.rest)
		}
	}
}

func TestDecorateText(t *testing.T) {
	for _, tc := range []struct{ in, marker, want string }{
		{"word", markerBold, "**word**"},
		{"**word**", markerBold, "word"},
		{"*word*", markerBold, "**word**"},
		{"**word**", markerItalic, "*word*"},
		{"word", markerCode, "`word`"},
		{"`word`", markerCode, "word"},
	} {
		if got := decorateText(tc.in, tc.marker); got != tc.want {
			t.Errorf("decorateText(%q, %q) = %q, want %q", tc.in, tc.marker, got, tc.want)
		}
	}
}

// the ex table and the normal table must not both claim a key/name
func TestNoLeaderCommandsRemain(t *testing.T) {
	e := newTestEditor(t, "x")
	for k := range e.normalCmds {
		if strings.HasPrefix(k, " ") {
			t.Errorf("normalCmds still has a leader-prefixed key: %q", k)
		}
		if len(k) != 1 {
			t.Errorf("normalCmds key %q is not a single byte", k)
		}
	}
}

// libvim reports the cursor column as a byte offset, and the screen-position
// code takes one. e.fc used to be converted to a rune count in between, so on a
// line of three-byte smart quotes the drawn cursor fell behind vim's and edits
// landed to the right of it.
func TestCursorColumnOnMultiByteLine(t *testing.T) {
	e := newTestEditor(t, "‘x“y’") // each quote is 3 bytes, 1 column

	e.editorProcessKey('$')
	e.scroll()
	if e.fc != 8 || e.cx != 4 {
		t.Errorf("after $: fc=%d cx=%d, want fc=8 cx=4", e.fc, e.cx)
	}

	e.editorProcessKey('0')
	e.sendKeys("ll")
	e.scroll()
	if e.fc != 4 || e.cx != 2 {
		t.Errorf("after 0ll: fc=%d cx=%d, want fc=4 cx=2", e.fc, e.cx)
	}

	// x deletes the glyph the cursor is drawn on
	e.editorProcessKey('x')
	if got, want := e.line(0), "‘xy’"; got != want {
		t.Errorf("after x: %q, want %q", got, want)
	}
}
