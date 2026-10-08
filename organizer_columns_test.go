package main

import (
	"strings"
	"testing"
)

func TestClipColumns(t *testing.T) {
	tests := []struct {
		s           string
		from, width int
		want        string
		cols        int
	}{
		{"alpha", 0, 3, "alp", 3},
		{"alpha", 3, 10, "ha", 2},
		// three-byte quotes are one column each; byte slicing tore them apart
		{"‘x“y’", 0, 3, "‘x“", 3},
		{"‘x“y’", 2, 2, "“y", 2},
		// a wide rune straddling an edge becomes spaces for its visible part
		{"a中b", 2, 2, " b", 2},
		{"a中b", 0, 2, "a ", 2},
		// fts highlight escapes take no columns and survive the clip
		{"\x1b[48;5;31mab\x1b[49mcd", 0, 1, "\x1b[48;5;31ma\x1b[49m", 1},
	}
	for _, tt := range tests {
		got, cols := clipColumns(tt.s, tt.from, tt.width)
		if got != tt.want || cols != tt.cols {
			t.Errorf("clipColumns(%q, %d, %d) = %q, %d; want %q, %d",
				tt.s, tt.from, tt.width, got, cols, tt.want, tt.cols)
		}
	}
}

func newTestOrganizer(title string) *Organizer {
	return &Organizer{
		rows:   []Row{{title: title}},
		Screen: &Screen{divider: 40, textLines: 30},
	}
}

// libvim reports the organizer cursor as a byte offset, which scroll() used as
// a display column, so smart quotes in a title put the cursor too far right.
func TestOrganizerScrollMultiByte(t *testing.T) {
	o := newTestOrganizer("‘x“y’")
	o.fc = 8 // byte offset of ’
	o.scroll()
	if o.cx != 4 || o.coloff != 0 {
		t.Errorf("cursor on ’: cx=%d coloff=%d, want cx=4 coloff=0", o.cx, o.coloff)
	}

	// a title wider than the column scrolls by columns, not bytes
	titlecols := o.titleColumnWidth()
	n := titlecols + 5
	o = newTestOrganizer(strings.Repeat("’", n))
	o.fc = 3 * (n - 1) // the last quote
	o.scroll()
	if o.coloff != n-titlecols || o.cx != titlecols-1 {
		t.Errorf("end of long title: cx=%d coloff=%d, want cx=%d coloff=%d",
			o.cx, o.coloff, titlecols-1, n-titlecols)
	}
}

func TestOrganizerVisualHighlightMultiByte(t *testing.T) {
	o := newTestOrganizer("‘x“y’")
	// selection made backwards: vim reports the anchor (’) then the cursor (“)
	o.setVisualHighlight(o.rows[0].title, [2][2]int{{1, 8}, {1, 4}})
	if o.highlight != [2]int{4, 11} {
		t.Fatalf("highlight = %v, want [4 11]", o.highlight)
	}

	o.mode = VISUAL
	var ab strings.Builder
	o.drawActiveRow(&ab)
	if want := LIGHT_GRAY_BG + "“y’" + RESET; !strings.Contains(ab.String(), want) {
		t.Errorf("drawActiveRow = %q, want it to contain %q", ab.String(), want)
	}
}
