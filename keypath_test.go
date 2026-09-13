package main

import (
	"strings"
	"testing"

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
