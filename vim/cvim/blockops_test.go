//go:build cgo && !windows

package cvim

import (
	"fmt"
	"testing"
)

// Blockwise-visual operator behavior, checked against stock vim.
// Fixture buffer is alpha/bravo/charlie/delta with the cursor on line 1, col 0.
func TestBlockOperators(t *testing.T) {
	cases := []struct {
		name string
		keys []string
		want []string
	}{
		{"C-v jj I'- '", []string{"\x16", "j", "j", "I", "-", " "},
			[]string{"- alpha", "- bravo", "- charlie", "delta"}},
		{"C-v jj A'!'", []string{"\x16", "j", "j", "A", "!"},
			[]string{"a!lpha", "b!ravo", "c!harlie", "delta"}},
		{"C-v jj $A'!'", []string{"\x16", "j", "j", "$", "A", "!"},
			[]string{"alpha!", "bravo!", "charlie!", "delta"}},
		{"C-v jj ll I'>'", []string{"\x16", "j", "j", "l", "l", "I", ">"},
			[]string{">alpha", ">bravo", ">charlie", "delta"}},
		{"C-v jj cX", []string{"\x16", "j", "j", "c", "X"},
			[]string{"Xlpha", "Xravo", "Xharlie", "delta"}},
		{"C-v jj d", []string{"\x16", "j", "j", "d"},
			[]string{"lpha", "ravo", "harlie", "delta"}},
		{"C-v jj D", []string{"\x16", "j", "j", "D"},
			[]string{"", "", "", "delta"}},
		{"C-v jj l r-", []string{"\x16", "j", "j", "l", "r", "-"},
			[]string{"--pha", "--avo", "--arlie", "delta"}},
	}

	for _, tc := range cases {
		VimInit(0)
		vbuf := CBufferNew(0)
		CBufferSetCurrent(vbuf)
		CBufferSetLines(vbuf, 0, 0, []string{"alpha", "bravo", "charlie", "delta"}, 4)
		Key("<esc>")
		Input("g")
		Input("g")
		Input("0")
		for _, k := range tc.keys {
			Input(k)
		}
		Key("<esc>")

		got := CBufferLines(vbuf)
		fmt.Printf("%-18s -> %q\n", tc.name, got)
		for i := range tc.want {
			if i >= len(got) || got[i] != tc.want[i] {
				t.Errorf("%s: line %d = %q, want %q", tc.name, i, got[i], tc.want[i])
			}
		}
	}
}

// Charwise/linewise visual I and A also go through v_visop upstream.
func TestNonBlockVisualInsert(t *testing.T) {
	for _, keys := range [][]string{
		{"v", "j", "I", "X"},
		{"v", "j", "A", "X"},
		{"V", "j", "I", "X"},
	} {
		VimInit(0)
		vbuf := CBufferNew(0)
		CBufferSetCurrent(vbuf)
		CBufferSetLines(vbuf, 0, 0, []string{"alpha", "bravo", "charlie"}, 3)
		Key("<esc>")
		Input("g")
		Input("g")
		Input("0")
		for _, k := range keys {
			Input(k)
		}
		Key("<esc>")
		fmt.Printf("%-14v -> %q\n", keys, CBufferLines(vbuf))
	}
}

// Counts, undo and repeat on top of a block insert.
func TestBlockInsertUndoAndRepeat(t *testing.T) {
	VimInit(0)
	vbuf := CBufferNew(0)
	CBufferSetCurrent(vbuf)
	CBufferSetLines(vbuf, 0, 0, []string{"alpha", "bravo", "charlie", "delta"}, 4)
	Key("<esc>")
	Input("g")
	Input("g")
	Input("0")
	for _, k := range []string{"\x16", "j", "j", "I", "-", " "} {
		Input(k)
	}
	Key("<esc>")
	fmt.Printf("%-18s -> %q\n", "after insert", CBufferLines(vbuf))
	Input("u")
	fmt.Printf("%-18s -> %q\n", "after undo", CBufferLines(vbuf))
	if got := CBufferLines(vbuf); got[0] != "alpha" || got[1] != "bravo" || got[2] != "charlie" {
		t.Errorf("undo did not restore buffer: %q", got)
	}
	Key("<c-r>")
	fmt.Printf("%-18s -> %q\n", "after redo", CBufferLines(vbuf))
	if got := CBufferLines(vbuf); got[0] != "- alpha" || got[2] != "- charlie" {
		t.Errorf("redo did not reapply block insert: %q", got)
	}
}
