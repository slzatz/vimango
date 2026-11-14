package pane

import "testing"

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
		got, cols := ClipColumns(tt.s, tt.from, tt.width)
		if got != tt.want || cols != tt.cols {
			t.Errorf("ClipColumns(%q, %d, %d) = %q, %d; want %q, %d",
				tt.s, tt.from, tt.width, got, cols, tt.want, tt.cols)
		}
	}
}

// A selection's edges arrive as byte offsets, which can land inside a
// character of several bytes.
func TestRuneEdges(t *testing.T) {
	row := "a‘b" // the quote is bytes 1 to 3
	for c, want := range map[int]int{-1: 0, 0: 0, 1: 1, 2: 1, 3: 1, 4: 4, 9: 5} {
		if got := RuneStart(row, c); got != want {
			t.Errorf("RuneStart(%q, %d) = %d, want %d", row, c, got, want)
		}
	}
	for c, want := range map[int]int{0: 1, 1: 4, 2: 4, 3: 4, 4: 5, 9: 5} {
		if got := InclusiveEnd(row, c); got != want {
			t.Errorf("InclusiveEnd(%q, %d) = %d, want %d", row, c, got, want)
		}
	}
}
