package vim

import (
	"strings"
	"testing"
)

// SetLines looks after what the buffer has (the fork's vimBufferSetLines,
// since 2026-11-21): the cursor and the marks go with their lines, and the
// cursor is then inside the text. Before that a cursor left on a line that
// was gone gave E315 on the next key -- in Insert mode a read past the
// buffer's memory -- and a mark there E19.
func TestSetLinesLooksAfterCursorAndMarks(t *testing.T) {
	five := []string{"alpha one", "bravo two", "charlie three", "delta four", "echo five"}
	start := func() *Buffer {
		InitializeVim(0)
		b := NewBuffer(0)
		SetCurrentBuffer(b)
		b.SetLines(0, -1, five)
		SendKey("<esc>")
		ExecuteCommand("delmarks!")
		SendMultiInput("gg0")
		return b
	}
	said := func(keys string) string {
		var all []string
		for _, m := range CaptureMessages(func() { SendMultiInput(keys) }) {
			all = append(all, m.Text)
		}
		return strings.Join(all, "; ")
	}

	t.Run("fewer lines under the cursor", func(t *testing.T) {
		b := start()
		SendMultiInput("G$")
		b.SetLines(0, -1, []string{"abc"})
		if pos := GetCursorPosition(); pos != [2]int{1, 2} {
			t.Errorf("the cursor is at %v, want the end of the one line", pos)
		}
		if msg := said("x"); msg != "" {
			t.Errorf("x after the lines were set said %q", msg)
		}
		if got := b.Line(1); got != "ab" {
			t.Errorf("line 1 is %q after x", got)
		}
	})

	t.Run("fewer lines in Insert mode", func(t *testing.T) {
		b := start()
		SendMultiInput("GAzz")
		b.SetLines(0, -1, []string{"x"})
		msg := said("y")
		SendKey("<esc>")
		if got := strings.Join(b.Lines(), "|"); got != "xy" || msg != "" {
			t.Errorf("the buffer is %q (vim said %q), want xy", got, msg)
		}
	})

	t.Run("the same number of lines", func(t *testing.T) {
		b := start()
		SendMultiInput("jmajwmbGmc")
		SendMultiInput("3G$")
		b.SetLines(0, -1, []string{"one", "two", "three", "four", "five"})
		if pos := GetCursorPosition(); pos != [2]int{3, 4} {
			t.Errorf("the cursor is at %v, want the end of line 3", pos)
		}
		for mark, line := range map[string]int{"a": 2, "b": 3, "c": 5} {
			SendMultiInput("gg'" + mark)
			if pos := GetCursorPosition(); pos[0] != line {
				t.Errorf("'%s goes to line %d, want %d", mark, pos[0], line)
			}
		}
	})

	t.Run("a mark on a line that went", func(t *testing.T) {
		b := start()
		SendMultiInput("jmaGmb")
		b.SetLines(0, -1, []string{"one", "two"})
		SendMultiInput("gg'a")
		if pos := GetCursorPosition(); pos[0] != 2 {
			t.Errorf("'a goes to line %d, want 2", pos[0])
		}
		SendMultiInput("gg")
		if msg := said("'b"); !strings.Contains(msg, "E20: Mark not set") {
			t.Errorf("'b, a mark on a line that is gone, said %q, want E20", msg)
		}
		if pos := GetCursorPosition(); pos[0] != 1 {
			t.Errorf("'b moved the cursor to line %d", pos[0])
		}
	})

	// The organizer puts a new note's empty title above the others.
	t.Run("a line put in above", func(t *testing.T) {
		b := start()
		SendMultiInput("jmajw")
		b.SetLines(0, 0, []string{""})
		if pos := GetCursorPosition(); pos != [2]int{4, 8} {
			t.Errorf("the cursor is at %v, want line 4 (charlie three), where it was in the text", pos)
		}
		SendMultiInput("gg'a")
		if pos := GetCursorPosition(); pos[0] != 3 || b.Line(3) != "bravo two" {
			t.Errorf("'a goes to line %d, want 3 (bravo two)", pos[0])
		}
	})
}
