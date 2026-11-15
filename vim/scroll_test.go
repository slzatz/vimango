package vim

import (
	"fmt"
	"reflect"
	"testing"
)

// scrollRecorder is a layout that notes what vim asks it to scroll, and on
// request moves the cursor as a layout does.
type scrollRecorder struct {
	asked  []string
	toLine int  // where Scroll sends the cursor, 0 for nowhere
	sol    bool // ...and whether to the start of that line
}

func (r *scrollRecorder) ScreenPosition(down bool, count, line, col, curswant int) (int, int) {
	return line, curswant
}
func (r *scrollRecorder) ScreenLine(motion ScreenLineMotion, count, line int) int   { return line }
func (r *scrollRecorder) ScreenColumn(motion ScreenColumnMotion, line, col int) int { return col }
func (r *scrollRecorder) Scroll(s Scroll, count int) {
	r.asked = append(r.asked, fmt.Sprintf("%s %d", scrollNames[s], count))
	if r.toLine > 0 {
		ScrollCursorTo(r.toLine, r.sol)
	}
}

var scrollNames = map[Scroll]string{
	ScrollCursorTop: "top", ScrollCursorMiddle: "middle", ScrollCursorBottom: "bottom",
	ScrollLineUp: "line up", ScrollLineDown: "line down",
	ScrollHalfPageDown: "half down", ScrollHalfPageUp: "half up",
	ScrollPageDown: "page down", ScrollPageUp: "page up",
}

func scrollBuffer(t *testing.T, lines ...string) *scrollRecorder {
	t.Helper()
	InitializeVim(0)
	vbuf := NewBuffer(0)
	SetCurrentBuffer(vbuf)
	vbuf.SetLines(0, -1, lines)
	SendKey("<esc>")
	SetCursorPosition(1, 0)
	r := &scrollRecorder{}
	SetScreenLayout(r)
	t.Cleanup(func() { SetScreenLayout(nil) })
	return r
}

// Every command that scrolls is handed to the layout, with the count typed.
func TestScrollCommandsReachTheLayout(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	for _, c := range []struct {
		keys []string // SendInput for a string, SendKey for a <name>
		want string
	}{
		{[]string{"\x05"}, "line up 1"},        // CTRL-E
		{[]string{"3", "\x05"}, "line up 3"},   //
		{[]string{"\x19"}, "line down 1"},      // CTRL-Y
		{[]string{"\x04"}, "half down 0"},      // CTRL-D: no count is 0
		{[]string{"7", "\x04"}, "half down 7"}, //
		{[]string{"\x15"}, "half up 0"},        // CTRL-U
		{[]string{"\x06"}, "page down 1"},      // CTRL-F
		{[]string{"2", "\x06"}, "page down 2"}, //
		{[]string{"\x02"}, "page up 1"},        // CTRL-B
		{[]string{"<pagedown>"}, "page down 1"},
		{[]string{"<pageup>"}, "page up 1"},
		{[]string{"z", "t"}, "top 1"},
		{[]string{"z", "<cr>"}, "top 1"},
		{[]string{"z", "z"}, "middle 1"},
		{[]string{"z", "."}, "middle 1"},
		{[]string{"z", "b"}, "bottom 1"},
		{[]string{"z", "-"}, "bottom 1"},
	} {
		r := scrollBuffer(t, lines...)
		SetCursorPosition(20, 0) // CTRL-U and CTRL-D are refused on the first and last lines
		for _, k := range c.keys {
			if k[0] == '<' {
				SendKey(k)
			} else {
				SendInput(k)
			}
		}
		if want := []string{c.want}; !reflect.DeepEqual(r.asked, want) {
			t.Errorf("%q: the layout was asked %q, want %q", c.keys, r.asked, want)
		}
	}
}

// A count before zt is a line to go to, which vim does before it asks; and
// the sideways commands, for a window that does not wrap, are not passed on.
func TestScrollCountAndSideways(t *testing.T) {
	r := scrollBuffer(t, "one", "two", "  three", "four")
	SendInput("3")
	SendInput("z")
	SendKey("<cr>")
	if got := GetCursorPosition(); got != [2]int{3, 2} {
		t.Errorf("3z<CR>: cursor at %v, want the first non-blank of line 3", got)
	}
	r.asked = nil
	for _, k := range []string{"zh", "zl", "zH", "zL", "zs", "ze"} {
		SendInput(k[:1])
		SendInput(k[1:])
	}
	if len(r.asked) != 0 {
		t.Errorf("the sideways commands asked the layout %q", r.asked)
	}
}

// With no layout the commands do nothing, as before: on the organizer's
// title, say.
func TestScrollWithNoLayout(t *testing.T) {
	scrollBuffer(t, "one", "two", "three")
	SetScreenLayout(nil)
	for _, k := range []string{"\x04", "\x05", "\x06"} {
		SendInput(k)
	}
	SendInput("z")
	SendInput("z")
	if got := GetCursorPosition(); got != [2]int{1, 0} {
		t.Errorf("cursor at %v, want it where it was", got)
	}
}

// ScrollCursorTo is how a layout moves the cursor from inside Scroll: to the
// column vim wants, or to the line's first non-blank unless 'nostartofline'.
func TestScrollCursorTo(t *testing.T) {
	lines := []string{"a long first line", "  second", "x", "  the fourth line"}

	// the wanted column: where the cursor is...
	r := scrollBuffer(t, lines...)
	r.toLine = 4
	SendInput("5")
	SendInput("l")
	SendInput("\x05")
	if got := GetCursorPosition(); got != [2]int{4, 5} {
		t.Errorf("CTRL-E from column 5: cursor at %v, want [4 5]", got)
	}
	// ...and it is still wanted after a shorter line
	r.toLine = 3
	SendInput("\x05")
	r.toLine = 1
	SendInput("\x05")
	if got := GetCursorPosition(); got != [2]int{1, 5} {
		t.Errorf("by way of a short line: cursor at %v, want [1 5]", got)
	}
	// after $ it is the end of the line
	r.toLine = 4
	SendInput("$")
	SendInput("\x05")
	if got := GetCursorPosition(); got != [2]int{4, len(lines[3]) - 1} {
		t.Errorf("CTRL-E after $: cursor at %v, want the end of line 4", got)
	}
	// a line the buffer does not have is its last
	r.toLine = 99
	SendInput("\x05")
	if got := GetCursorPosition(); got[0] != 4 {
		t.Errorf("to line 99: cursor at %v, want line 4", got)
	}

	// the first non-blank, for CTRL-D and its kind...
	r = scrollBuffer(t, lines...)
	r.toLine, r.sol = 4, true
	SendInput("5")
	SendInput("l")
	SendInput("\x04")
	if got := GetCursorPosition(); got != [2]int{4, 2} {
		t.Errorf("CTRL-D: cursor at %v, want [4 2]", got)
	}
	// ...unless 'nostartofline': then the column
	r = scrollBuffer(t, lines...)
	ExecuteCommand("set nostartofline")
	defer ExecuteCommand("set startofline")
	r.toLine, r.sol = 4, true
	SendInput("5")
	SendInput("l")
	SendInput("\x04")
	if got := GetCursorPosition(); got != [2]int{4, 5} {
		t.Errorf("CTRL-D with nostartofline: cursor at %v, want [4 5]", got)
	}
}
