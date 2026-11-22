package vim

import (
	"reflect"
	"strings"
	"testing"
)

// clipboardBuffer is a buffer of three lines with the cursor on the first,
// and no callbacks left behind.
func clipboardBuffer(t *testing.T) *Buffer {
	t.Helper()
	InitializeVim(0)
	buf := NewBuffer(0)
	SetCurrentBuffer(buf)
	buf.SetLines(0, -1, []string{"alpha bravo", "charlie", "delta"})
	SetCursorPosition(1, 0)
	t.Cleanup(func() {
		SendKey("<esc>")
		SetYankCallback(nil)
		SetClipboardCallback(nil)
		ExecuteCommand("set clipboard=")
	})
	return buf
}

// Every yank and delete is reported, with the operator, the register named
// and how the text was taken.
func TestYankCallback(t *testing.T) {
	buf := clipboardBuffer(t)
	var got []Yank
	SetYankCallback(func(y Yank) { got = append(got, y) })
	// with no clipboard to ask, libvim makes "+dd a plain dd (adjust_clip_reg)
	SetClipboardCallback(func(byte) ([]string, bool, bool) { return nil, false, false })

	for _, c := range []struct {
		keys string
		want Yank
	}{
		{"yy", Yank{'y', 0, Linewise, []string{"alpha bravo"}}},
		{"yiw", Yank{'y', 0, Charwise, []string{"alpha"}}},
		{"yj", Yank{'y', 0, Linewise, []string{"alpha bravo", "charlie"}}},
		{"vjy", Yank{'y', 0, Charwise, []string{"alpha bravo", "c"}}},
		{"\"ayiw", Yank{'y', 'a', Charwise, []string{"alpha"}}},
		{"\"+yy", Yank{'y', '+', Linewise, []string{"alpha bravo"}}},
		{"\"*yiw", Yank{'y', '*', Charwise, []string{"alpha"}}},
		{"x", Yank{'d', 0, Charwise, []string{"a"}}},
		{"\"+dd", Yank{'d', '+', Linewise, []string{"alpha bravo"}}},
		{"cw", Yank{'c', 0, Charwise, []string{"alpha"}}},
	} {
		buf.SetLines(0, -1, []string{"alpha bravo", "charlie", "delta"})
		SetCursorPosition(1, 0)
		got = nil
		SendMultiInput(c.keys)
		SendKey("<esc>")
		if len(got) != 1 || !reflect.DeepEqual(got[0], c.want) {
			t.Errorf("%q: reported %+v, want %+v", c.keys, got, c.want)
		}
	}

	buf.SetLines(0, -1, []string{"alpha bravo", "charlie", "delta"})
	got = nil
	ExecuteCommand("2yank")
	if len(got) != 1 || !reflect.DeepEqual(got[0], Yank{'y', 0, Linewise, []string{"charlie"}}) {
		t.Errorf(":2yank reported %+v", got)
	}
}

// The clipboard registers are read from the callback, as characters or as
// lines, and no other register is.
func TestClipboardCallback(t *testing.T) {
	buf := clipboardBuffer(t)
	var asked []byte
	text, linewise, ok := []string{"one", "two"}, true, true
	SetClipboardCallback(func(reg byte) ([]string, bool, bool) {
		asked = append(asked, reg)
		return text, linewise, ok
	})

	SendMultiInput("\"+p")
	if got := strings.Join(buf.Lines(), "|"); got != "alpha bravo|one|two|charlie|delta" {
		t.Errorf("\"+p of two lines: %q", got)
	}
	if string(asked) != "+" {
		t.Errorf("asked for %q, want +", asked)
	}

	text, linewise = []string{"X", "Y"}, false
	SetCursorPosition(1, 0)
	SendMultiInput("\"*P")
	if got := strings.Join(buf.Lines(), "|"); got != "X|Yalpha bravo|one|two|charlie|delta" {
		t.Errorf("\"*P of characters: %q", got)
	}
	if got := EvaluateExpression("getregtype('+')"); got != "v" {
		t.Errorf("getregtype('+') = %q, want v", got)
	}
	if got := EvaluateExpression("@+"); got != "X\nY" {
		t.Errorf("@+ = %q", got)
	}

	// the other registers are vim's own: nobody is asked
	asked = nil
	SetCursorPosition(1, 0)
	SendMultiInput("yyp\"ayy\"ap")
	_ = EvaluateExpression("getreg('0') . getreg('a') . getreg('\"')")
	if len(asked) != 0 {
		t.Errorf("asked for %q at reads of other registers", asked)
	}

	// nothing on the clipboard: the register keeps what was yanked into it
	ok = false
	buf.SetLines(0, -1, []string{"kept"})
	SetCursorPosition(1, 0)
	SendMultiInput("\"+yy")
	if got := EvaluateExpression("@+"); got != "kept\n" {
		t.Errorf("@+ with an empty clipboard = %q, want what was yanked into it", got)
	}
}
