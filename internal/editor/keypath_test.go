package editor

import (
	"strings"
	"testing"

	"github.com/slzatz/vimango/internal/pane"
	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// The leader used to swallow <space> unconditionally, so it never reached vim.
// vim defines <Space> as "l", and a vim-like editor should honour that.
func TestSpaceIsAMotionNotALeader(t *testing.T) {
	e := newTestEditor(t, "alpha bravo")

	e.ProcessKey(' ')
	if got := vim.GetCursorPosition(); got != [2]int{1, 1} {
		t.Errorf("after <space>: cursor = %v, want [1 1]", got)
	}

	// the old matcher also swallowed the key *after* an unmatched leader
	e.ProcessKey(' ')
	e.ProcessKey('x') // delete char under cursor
	if got, want := e.line(0), "alha bravo"; got != want {
		t.Errorf("after <space><space>x: %q, want %q", got, want)
	}
}

// A buffer-changing ex command must ask the caller to redraw. MainLoop only
// calls drawText when ProcessKey returns true, and dispatch ends the
// keystroke, so returning false leaves the edit invisible until some later key
// trips the tick check -- the old :bold showed the old text until you pressed
// l. The dispatch is generic, so a stand-in command that edits the buffer the
// way vimango's own commands do (keys sent to vim) exercises it.
func TestBufferChangingExCommandRequestsRedraw(t *testing.T) {
	e := newTestEditor(t, "alpha bravo charlie")
	e.exCmds["zzedit"] = func(*Editor) { vim.SendInput("x") }

	e.sendKeys(":zzedit")
	if redraw := e.ProcessKey('\r'); !redraw {
		t.Error(":zzedit returned redraw=false; the change would not be drawn")
	}
	// and the editor's own copy of the text must be current, since drawText
	// renders e.ss rather than re-reading the buffer
	if got, want := e.ss[0], "lpha bravo charlie"; got != want {
		t.Errorf("e.ss[0] = %q, want %q", got, want)
	}

	// a command that changes nothing should not force a redraw
	e2 := newTestEditor(t, "alpha bravo")
	e2.sendKeys(":number")
	if redraw := e2.ProcessKey('\r'); redraw {
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
		if _, mapped := pane.VimKeyName(k.code); mapped {
			t.Errorf("%s is in termcodes; this test assumes it is not", k.name)
		}

		e := newTestEditor(t, "alpha")
		e.ProcessKey('i') // INSERT, where a forwarded key becomes text
		e.ProcessKey(k.code)
		if got := e.vbuf.Lines()[0]; got != "alpha" {
			t.Errorf("%s in INSERT changed the buffer to %q", k.name, got)
		}

		e2 := newTestEditor(t, "alpha")
		e2.sendKeys(":")
		e2.ProcessKey(k.code)
		if e2.command_line != "" {
			t.Errorf("%s in EX put %q on the command line", k.name, e2.command_line)
		}
	}

	// the keys vim does understand must still get through
	for _, k := range []struct {
		name string
		code int
	}{
		{"Home", terminal.KeyHome},
		{"PageUp", terminal.KeyPageUp},
		{"ArrowLeft", terminal.KeyArrowLeft},
	} {
		if _, mapped := pane.VimKeyName(k.code); !mapped {
			t.Errorf("%s (%d) is no longer in termcodes", k.name, k.code)
		}
	}
}

// The range in front of a command is skipped the way vim skips it, so the
// command name is found where vim will find it.
func TestSkipExRange(t *testing.T) {
	for _, tc := range []struct{ in, rest string }{
		{"write", "write"},
		{"'<,'>sort", "sort"},
		{"%s/a/b/", "s/a/b/"},
		{"1,5d", "d"},
		{".,$j", "j"},
		{".,+3>", ">"},
		{"'a,'bm0", "m0"},
		{"/start/,/end/d", "d"},
		{`/a\/b/d`, "d"},
		{"?back?d", "d"},
		{`\/,\?d`, "d"},
		{"42", ""},
		{"$", ""},
		{"3 d", "d"},
		{"1,3 : d", "d"},
		{"1;/x/normal dd", "normal dd"},
	} {
		if got := skipExRange(tc.in); got != tc.rest {
			t.Errorf("skipExRange(%q) = %q, want %q", tc.in, got, tc.rest)
		}
	}
}

// libvim reports the cursor column as a byte offset, and the screen-position
// code takes one. e.fc used to be converted to a rune count in between, so on a
// line of three-byte smart quotes the drawn cursor fell behind vim's and edits
// landed to the right of it.
func TestCursorColumnOnMultiByteLine(t *testing.T) {
	e := newTestEditor(t, "‘x“y’") // each quote is 3 bytes, 1 column

	e.ProcessKey('$')
	e.scroll()
	if e.fc != 8 || e.cx != 4 {
		t.Errorf("after $: fc=%d cx=%d, want fc=8 cx=4", e.fc, e.cx)
	}

	e.ProcessKey('0')
	e.sendKeys("ll")
	e.scroll()
	if e.fc != 4 || e.cx != 2 {
		t.Errorf("after 0ll: fc=%d cx=%d, want fc=4 cx=2", e.fc, e.cx)
	}

	// x deletes the glyph the cursor is drawn on
	e.ProcessKey('x')
	if got, want := e.line(0), "‘xy’"; got != want {
		t.Errorf("after x: %q, want %q", got, want)
	}
}

// A key that takes vim to another buffer does not leave it there. Ctrl-^
// switches to the alternate buffer -- the organizer's list of titles, in the
// TUI -- and what was typed next went into that buffer: titles were edited,
// and the note on screen did not change.
func TestKeyThatLeavesTheNoteComesBack(t *testing.T) {
	e := newTestEditor(t, "alpha", "bravo")
	// as when a note is opened from the organizer: the organizer's buffer
	// was current, then the note's
	titles := vim.NewBuffer(0)
	vim.SetCurrentBuffer(titles)
	titles.SetLines(0, -1, []string{"a title", "another title"})
	vim.SetCurrentBuffer(e.vbuf)

	out := captureStdout(t, func() {
		e.ProcessKey(ctrlKey('^'))
		e.sendKeys("ix")
		e.ProcessKey('\x1b')
	})

	if vim.GetCurrentBuffer().GetID() != e.vbuf.GetID() {
		t.Fatal("vim is not on the note's buffer")
	}
	if got := e.line(0); got != "xalpha" {
		t.Errorf("after Ctrl-^ and ix the note's first line is %q, want the x in it", got)
	}
	if got := titles.Lines(); got[0] != "a title" || got[1] != "another title" {
		t.Errorf("the organizer's titles are now %q", got)
	}
	if strings.Contains(out, "line 1 of 2") {
		t.Errorf("vim's report on the other buffer was shown: %q", out)
	}
}
