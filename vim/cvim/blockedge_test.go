//go:build cgo && !windows

package cvim

import (
	"fmt"
	"testing"
)

// Edge cases for blockwise-visual "I"/"A", with the expectations taken from
// stock vim 9.1 (vim -u NONE -N -es -c 'exe "normal! ..."').
func TestBlockInsertEdgeCases(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		keys  []string
		want  []string
	}{
		// A line shorter than the block start column is skipped by "I"...
		{"short line, I at col 3",
			[]string{"alpha", "ab", "charlie", "delta"},
			[]string{"l", "l", "l", "\x16", "j", "j", "I", "-"},
			[]string{"alp-ha", "ab", "cha-rlie", "delta"}},
		// ...but "A" pads it out with spaces.
		{"short line, A at col 3",
			[]string{"alpha", "ab", "charlie", "delta"},
			[]string{"l", "l", "l", "\x16", "j", "j", "A", "-"},
			[]string{"alph-a", "ab  -", "char-lie", "delta"}},
		{"leading tabs",
			[]string{"\talpha", "\tbravo", "\tcharlie"},
			[]string{"\x16", "j", "j", "I", ">"},
			[]string{">\talpha", ">\tbravo", ">\tcharlie"}},
		{"count in insert",
			[]string{"alpha", "bravo", "charlie", "delta"},
			[]string{"\x16", "j", "j", "3", "I", "x"},
			[]string{"xxxalpha", "xxxbravo", "xxxcharlie", "delta"}},
	}

	for _, tc := range cases {
		VimInit(0)
		vbuf := CBufferNew(0)
		CBufferSetCurrent(vbuf)
		CBufferSetLines(vbuf, 0, 0, tc.lines, len(tc.lines))
		Key("<esc>")
		Input("g")
		Input("g")
		Input("0")
		for _, k := range tc.keys {
			Input(k)
		}
		Key("<esc>")

		got := CBufferLines(vbuf)
		fmt.Printf("%-22s -> %q\n", tc.name, got)
		for i := range tc.want {
			if i >= len(got) || got[i] != tc.want[i] {
				t.Errorf("%s: line %d = %q, want %q", tc.name, i, got[i], tc.want[i])
			}
		}
	}
}

// "." repeats a block insert on a following block of the same height.
func TestBlockInsertRepeat(t *testing.T) {
	VimInit(0)
	vbuf := CBufferNew(0)
	CBufferSetCurrent(vbuf)
	CBufferSetLines(vbuf, 0, 0, []string{"alpha", "bravo", "charlie", "delta"}, 4)
	Key("<esc>")
	Input("g")
	Input("g")
	Input("0")
	for _, k := range []string{"\x16", "j", "I", "-"} {
		Input(k)
	}
	Key("<esc>")
	Input("j")
	Input("j")
	Input(".")

	got := CBufferLines(vbuf)
	fmt.Printf("%-22s -> %q\n", "block insert then .", got)
	want := []string{"-alpha", "-bravo", "-charlie", "-delta"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}
