package vim

import (
	"strings"
	"testing"
)

// SetLines is for text that is new, and libvim saves nothing for undo when
// lines are set: the history it had would go on describing the old lines.
// With "x" on line 1, the whole buffer replaced and then "u", line 1 of the
// old text came back over line 1 of the new. So SetLines forgets the history.
func TestSetLinesForgetsUndo(t *testing.T) {
	text := func(b *Buffer) string { return strings.Join(b.Lines(), "|") }
	start := func() *Buffer {
		InitializeVim(0)
		b := NewBuffer(0)
		SetCurrentBuffer(b)
		b.SetLines(0, -1, []string{"alpha one", "bravo two", "charlie three"})
		SendKey("<esc>")
		SendMultiInput("gg0")
		return b
	}

	t.Run("the whole buffer", func(t *testing.T) {
		b := start()
		SendMultiInput("x")
		b.SetLines(0, -1, []string{"new one", "new two", "new three", "new four"})
		for _, key := range []string{"u", "<c-r>", "U"} {
			if strings.HasPrefix(key, "<") {
				SendKey(key)
			} else {
				SendInput(key)
			}
			if got := text(b); got != "new one|new two|new three|new four" {
				t.Fatalf("after %s the buffer is %q: the old text is back", key, got)
			}
		}
	})

	// The organizer puts an empty title above the others for a new note.
	t.Run("a line put in above an edit", func(t *testing.T) {
		b := start()
		SendMultiInput("jx")
		b.SetLines(0, 0, []string{""})
		SendInput("u")
		if got := text(b); got != "|alpha one|ravo two|charlie three" {
			t.Errorf("after u the buffer is %q", got)
		}
	})

	// A buffer nothing was done to has no history; and a new note's buffer
	// is not emptied by u.
	t.Run("a buffer just filled", func(t *testing.T) {
		b := start()
		SendInput("u")
		if got := text(b); got != "alpha one|bravo two|charlie three" {
			t.Errorf("u on a buffer just filled left %q", got)
		}
	})

	t.Run("what is changed afterwards is undone", func(t *testing.T) {
		b := start()
		SendMultiInput("x")
		b.SetLines(0, -1, []string{"new one", "new two"})
		SendMultiInput("xjdd")
		if got := text(b); got != "ew one" {
			t.Fatalf("after x and dd the buffer is %q", got)
		}
		SendMultiInput("uu")
		if got := text(b); got != "new one|new two" {
			t.Errorf("after undoing both the buffer is %q", got)
		}
		SendInput("u") // and no further
		if got := text(b); got != "new one|new two" {
			t.Errorf("a third u went behind the text that was set: %q", got)
		}
		SendKey("<c-r>")
		if got := text(b); got != "ew one|new two" {
			t.Errorf("after a redo the buffer is %q", got)
		}
	})
}
