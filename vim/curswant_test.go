package vim

import "testing"

// j and k go to the column the cursor was last taken to along its line, and
// "$" means the end of every line until something else is asked for. libvim
// lost track of that when a motion brought the cursor back to the column a
// motion had last left it in: "0$0j" went to the end of the next line. On
// lines of 80 characters or fewer only; a longer one scrolled libvim's own
// window sideways, which put it right by another route. The fork's PATCHES.md,
// "The column wanted after $".
//
// And "$" as the first key in a buffer wanted the end of that line only: the
// column a new buffer wants was settled after the first command, not before
// it. PATCHES.md, "What a buffer just entered wants is settled before the
// first key".
//
// A libvim.a from before those fixes links without complaint, so this is
// also what says the library is too old.
func TestWantedColumnAfterDollar(t *testing.T) {
	for _, c := range []struct {
		keys      string
		line, col int
	}{
		{"0$0j", 2, 0},
		{"0$^j", 2, 0},
		{"0$ggj", 2, 0},
		{"0$0jj", 3, 0},
		{"l$hj", 2, 7},   // back to column 7, where h ends...
		{"7l$hj", 2, 7},  // ...and where 7l had been
		{"jj0$0k", 2, 0}, // going up, from a line that is not the first
		{"0$j", 2, 10},   // the end is still wanted,
		{"0$0$j", 2, 10}, // and wanted again
		{"0$jk", 1, 8},
		// "$" as the first key in a buffer: the column wanted was worked out
		// after the first command, which made "the end" the end of line 1
		{"$j", 2, 10},
		{"$jj", 3, 12},
	} {
		InitializeVim(0)
		// before the buffer is made, so that the first key of the case is
		// the first key the buffer gets
		SendKey("<esc>")
		b := NewBuffer(0)
		SetCurrentBuffer(b)
		b.SetLines(0, -1, []string{"alpha one", "bravo two x", "charlie three"})
		for _, k := range c.keys {
			SendInput(string(k))
		}
		if got := GetCursorPosition(); got != [2]int{c.line, c.col} {
			t.Errorf("%s: cursor at %v, want [%d %d]", c.keys, got, c.line, c.col)
		}
	}
}
