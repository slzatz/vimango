package main

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/slzatz/vimango/vim"
)

// A listing is shown in the notice box as a code block: it can take the box's
// width, where ShowNotice wraps, less what the glamour style indents a code
// block by (4 columns in the styles vimango has).
func TestListingWidthIsTheBoxLessTheCodeIndent(t *testing.T) {
	for _, style := range []string{"darkslz.json", "default.json"} {
		s := &Screen{totaleditorcols: 100, glamourStyle: style}
		if got, want := s.ListingWidth(), 100-NOTICE_RIGHT_PADDING-4; got != want {
			t.Errorf("%s: ListingWidth %d, want %d", style, got, want)
		}
		s.totaleditorcols = 60
		if got, want := s.ListingWidth(), 60-NOTICE_RIGHT_PADDING-4; got != want {
			t.Errorf("%s: after a resize, ListingWidth %d, want %d", style, got, want)
		}
	}
}

// Through the screen itself: what :digraphs lists is drawn one line of the
// box for each line vim wrote, none of them wrapped, and the box is filled.
func TestListingIsShownUnwrapped(t *testing.T) {
	t.Cleanup(func() { vim.ExecuteCommand("set columns=80") })
	a, ids := editorOnlyApp(t, "alpha")
	e := openIn(t, a, ids[0])
	captureStdout(t, func() { runEx(e, "digraphs") })
	lines := a.Screen.notice
	if len(lines) < 10 {
		t.Fatalf("the notice has %d lines", len(lines))
	}
	box := a.Screen.totaleditorcols - NOTICE_RIGHT_PADDING
	widest := 0
	for _, l := range lines {
		w := runewidth.StringWidth(stripANSI(l))
		widest = max(widest, w)
		if w > box {
			t.Errorf("a line %d wide in a box %d wide: %q", w, box, stripANSI(l))
		}
		if strings.HasPrefix(stripANSI(l), strings.Repeat(" ", 16)) && strings.TrimSpace(stripANSI(l)) != "" {
			t.Errorf("a line was wrapped: %q", stripANSI(l))
		}
	}
	if widest < box-16 {
		t.Errorf("the widest line is %d in a box %d wide: the listing does not fill it", widest, box)
	}
}
