package editor

import (
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// fakeClipboard stands in for the system's, which is the user's.
type fakeClipboard struct {
	text   string
	has    bool
	writes int
}

func (c *fakeClipboard) Text() (string, bool) { return c.text, c.has }
func (c *fakeClipboard) SetText(text string) bool {
	c.text, c.has = text, true
	c.writes++
	return true
}

func newClipboardEditor(t *testing.T, lines ...string) (*Editor, *fakeClipboard) {
	t.Helper()
	e := newTestEditor(t, lines...)
	vim.ExecuteCommand("mapclear | mapclear!")
	c := &fakeClipboard{}
	UseClipboard(c)
	t.Cleanup(func() {
		vim.SendKey("<esc>")
		vim.SendKey("<esc>")
		UseClipboard(nil)
	})
	return e, c
}

// A yank is on the clipboard, as the text another program would paste: whole
// lines end in a line end, characters do not.
func TestYankGoesToClipboard(t *testing.T) {
	for _, c := range []struct {
		keys, want string
	}{
		{"yy", "alpha bravo\n"},
		{"Y", "alpha bravo\n"},
		{"yj", "alpha bravo\ncharlie\n"},
		{"yiw", "alpha"},
		{"wy$", "bravo"},
		{"vjy", "alpha bravo\nc"},
		{"Vjy", "alpha bravo\ncharlie\n"},
		{"\x16jly", "al\nch"}, // a block, as its rows
		{"\"\"yiw", "alpha"},
		{"\"+yy", "alpha bravo\n"},
		{"\"*yiw", "alpha"},
		{"\"+dd", "alpha bravo\n"}, // a cut, when the clipboard is named
		{"\"+x", "a"},
		{"\"*ciw", "alpha"},
	} {
		e, clip := newClipboardEditor(t, "alpha bravo", "charlie", "delta")
		typeKeys(t, e, c.keys)
		if !clip.has || clip.text != c.want {
			t.Errorf("%q: the clipboard holds %q (%v), want %q", c.keys, clip.text, clip.has, c.want)
		}
	}

	e, clip := newClipboardEditor(t, "alpha bravo", "charlie", "delta")
	captureStdout(t, func() { e.exCommand("2,3yank") })
	if clip.text != "charlie\ndelta\n" {
		t.Errorf(":2,3yank: the clipboard holds %q", clip.text)
	}
}

// What was copied on purpose stays there: a delete does not replace it, and
// neither does a yank into a register that was named.
func TestDeletesAndNamedRegistersLeaveTheClipboard(t *testing.T) {
	for _, keys := range []string{"x", "dd", "dw", "D", "cwnew\x1b", "s!\x1b", "Vjd", "\"ayy", "\"add", "\"Ayy", "\"_dd", "p", "P", "J", ">>"} {
		e, clip := newClipboardEditor(t, "alpha bravo", "charlie", "delta")
		clip.text, clip.has = "copied on purpose", true
		typeKeys(t, e, keys)
		if clip.writes != 0 || clip.text != "copied on purpose" {
			t.Errorf("%q wrote %q to the clipboard", keys, clip.text)
		}
	}

	// ...and a macro being recorded is no yank
	e, clip := newClipboardEditor(t, "alpha bravo", "charlie")
	typeKeys(t, e, "qajq")
	captureStdout(t, func() { e.exCommand("let @b = 'set'") })
	if clip.writes != 0 {
		t.Errorf("recording a macro, or :let @b, wrote %q to the clipboard", clip.text)
	}
}

// "+ and "* are the clipboard: whatever is there is put, as lines when it
// ends in a line end and as characters when it does not.
func TestClipboardRegisterPuts(t *testing.T) {
	for _, c := range []struct {
		clip, keys, want string
	}{
		{"one\ntwo\n", "\"+p", "alpha bravo|one|two|charlie"},
		{"one\ntwo\n", "\"+P", "one|two|alpha bravo|charlie"},
		{"one\ntwo\n", "\"*p", "alpha bravo|one|two|charlie"},
		{"X", "\"+p", "aXlpha bravo|charlie"},
		{"X", "\"+P", "Xalpha bravo|charlie"},
		{"X\nY", "\"+P", "X|Yalpha bravo|charlie"},
		{"X", "\"+3p", "aXXXlpha bravo|charlie"},
		{"one\r\ntwo\r\n", "\"+p", "alpha bravo|one|two|charlie"}, // as Windows wrote it
		{"a\x1bddb", "\"+P", "addbalpha bravo|charlie"},           // an <esc> is not a key
		{"X", "viw\"+p", "X bravo|charlie"},                       // over a selection
		{"X", "i\x12+\x1b", "Xalpha bravo|charlie"},               // CTRL-R + in INSERT
		{"\n", "\"+p", "alpha bravo||charlie"},                    // an empty line is a line
	} {
		e, clip := newClipboardEditor(t, "alpha bravo", "charlie")
		clip.text, clip.has = c.clip, true
		typeKeys(t, e, c.keys)
		if got := strings.Join(e.vbuf.Lines(), "|"); got != c.want {
			t.Errorf("%q with %q on the clipboard: %q, want %q", c.keys, c.clip, got, c.want)
		}
		if e.mode != NORMAL {
			t.Errorf("%q left the editor in %v", c.keys, e.mode)
		}
		if strings.Join(e.ss, "|") != strings.Join(e.vbuf.Lines(), "|") {
			t.Errorf("%q: the editor's copy of the text is not vim's", c.keys)
		}
	}
}

// The round trip this is for: yanked in one note, put in another. Two
// editors in two processes share nothing but the clipboard, and neither do
// these two, one after the other with the registers emptied between.
func TestYankInOneNotePutInAnother(t *testing.T) {
	e, clip := newClipboardEditor(t, "# From", "", "- one", "- two", "after")
	typeKeys(t, e, "jjyj")
	if clip.text != "- one\n- two\n" {
		t.Fatalf("the clipboard holds %q", clip.text)
	}

	other := newTestEditor(t, "# To", "end")
	vim.ExecuteCommand("call setreg('\"', '') | call setreg('0', '') | call setreg('+', '')")
	typeKeys(t, other, "\"+p")
	if got := strings.Join(other.vbuf.Lines(), "|"); got != "# To|- one|- two|end" {
		t.Errorf("put in the other note: %q", got)
	}
}

// "p" is vim's: the register written last, whatever another program has put
// on the clipboard since.
func TestPlainPutIsNotTheClipboard(t *testing.T) {
	e, clip := newClipboardEditor(t, "alpha bravo", "charlie")
	typeKeys(t, e, "yiw")
	clip.text = "from another program"
	typeKeys(t, e, "$p")
	if got := e.line(0); got != "alpha bravoalpha" {
		t.Errorf("p put %q", got)
	}
}

// With nothing on the clipboard -- empty, or an image -- "+ is what was last
// yanked into it here.
func TestEmptyClipboard(t *testing.T) {
	e, clip := newClipboardEditor(t, "alpha bravo", "charlie")
	typeKeys(t, e, "\"+yiw")
	clip.has = false
	typeKeys(t, e, "$\"+p")
	if got := e.line(0); got != "alpha bravoalpha" {
		t.Errorf("\"+p with an empty clipboard put %q", got)
	}

	clip.text, clip.has = "", true // text, and none of it
	typeKeys(t, e, "\"+p")
	if got := e.line(0); got != "alpha bravoalphaalpha" {
		t.Errorf("\"+p with no text on the clipboard put %q", got)
	}
}

// Without a clipboard (Linux, for now) nothing is asked of one.
func TestNoClipboard(t *testing.T) {
	e, clip := newClipboardEditor(t, "alpha bravo", "charlie")
	UseClipboard(nil)
	typeKeys(t, e, "yy\"+yyp")
	if clip.writes != 0 {
		t.Errorf("a yank reached a clipboard that is not connected: %q", clip.text)
	}
	if got := strings.Join(e.vbuf.Lines(), "|"); got != "alpha bravo|alpha bravo|charlie" {
		t.Errorf("yy and p: %q", got)
	}
}

func TestClipboardLines(t *testing.T) {
	for _, c := range []struct {
		text     string
		lines    []string
		linewise bool
		ok       bool
	}{
		{"", nil, false, false},
		{"\x1b", nil, false, false},
		{"a", []string{"a"}, false, true},
		{"a\n", []string{"a"}, true, true},
		{"a\nb", []string{"a", "b"}, false, true},
		{"a\n\nb\n", []string{"a", "", "b"}, true, true},
		{"a\n\n", []string{"a", ""}, true, true},
		{"\n", []string{""}, true, true},
		{"a\r\nb\r", []string{"a", "b"}, true, true},
	} {
		lines, linewise, ok := clipboardLines(c.text)
		if strings.Join(lines, "|") != strings.Join(c.lines, "|") || len(lines) != len(c.lines) || linewise != c.linewise || ok != c.ok {
			t.Errorf("%q: %q, linewise %v, ok %v", c.text, lines, linewise, ok)
		}
	}
}
