package editor

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/slzatz/vimango/vim"
)

// vim lays out what a command lists for the width of the box it is shown in
// (fitListings), not for the 80 columns libvim believed in: a wide pane is
// filled, and a narrow one is not wrapped. Until 2026-11-20 :registers cut
// its lines at 74 characters whatever the width, and :digraphs put six to a
// line.

func widestLine(text string) int {
	w := 0
	for _, l := range strings.Split(text, "\n") {
		w = max(w, runewidth.StringWidth(l))
	}
	return w
}

func TestListingsFitTheBox(t *testing.T) {
	t.Cleanup(func() {
		vim.ExecuteCommand("set columns=80")
	})
	e := newTestEditor(t, "alpha")
	vim.ExecuteCommand("let @a = repeat('x', 150)")
	h := hostOf(e)

	for _, width := range []int{50, 120} {
		h.listingWidth = width
		h.shown = nil
		captureStdout(t, func() {
			e.exCommand("registers a")
			e.ProcessKey(0x1b)
			e.exCommand("digraphs")
			e.ProcessKey(0x1b)
		})
		if len(h.shown) != 2 {
			t.Fatalf("width %d: %d listings shown, want 2", width, len(h.shown))
		}
		registers, digraphs := h.shown[0], h.shown[1]
		if got := widestLine(registers); got > width || got < width-6 {
			t.Errorf("width %d: :registers is %d columns wide", width, got)
		}
		if got := widestLine(digraphs); got > width+2 || got < width-14 {
			// vim pads each entry to a fixed width, and counts some
			// characters' width as the terminal does not
			t.Errorf("width %d: :digraphs is %d columns wide", width, got)
		}
		if said := exMessage(t, e, "echo &columns"); !strings.Contains(said, itoa(width)) {
			t.Errorf("width %d: &columns is %q", width, said)
		}
	}
}

// A box with no width to give, or less than vim takes, leaves vim alone.
func TestListingWidthTooSmallIsLeftAlone(t *testing.T) {
	t.Cleanup(func() {
		vim.ExecuteCommand("set columns=80")
	})
	e := newTestEditor(t, "alpha")
	hostOf(e).listingWidth = 5
	if said := exMessage(t, e, "echo &columns"); !strings.Contains(said, "80") {
		t.Errorf("&columns is %q, want 80", said)
	}
}

func itoa(n int) string {
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}
