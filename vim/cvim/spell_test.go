//go:build cgo && !windows

package cvim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// spellSetup starts a fresh vim with "lines" in the current buffer and spell
// checking on, using the dictionary in vimango's spell/ directory (copied
// there by scripts/build-libvim.sh). Skips the test when it is missing.
func spellSetup(t *testing.T, lines []string) Buffer {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "spell", "en.utf-8.spl")); err != nil {
		t.Skip("spell/en.utf-8.spl not found; run scripts/build-libvim.sh")
	}

	VimInit(0)
	EnableSpellSuggestCallback()
	vbuf := CBufferNew(0)
	CBufferSetCurrent(vbuf)
	CBufferSetLines(vbuf, 0, -1, lines, len(lines))
	Execute("let &runtimepath = '" + strings.ReplaceAll(root, "'", "''") + ",' . &runtimepath")
	Execute("let &spellfile = tempname() . '.utf-8.add'")
	Execute("set spelllang=en spellsuggest=best,9,timeout:1000 spell")
	TakeSpellSuggestRequest() // clear anything left by an earlier test
	return vbuf
}

func input(keys ...string) {
	for _, k := range keys {
		Input(k)
	}
}

func TestSpellGetErrors(t *testing.T) {
	vbuf := spellSetup(t, []string{"Hello wrold here.", "this starts lowercase"})

	errs := SpellGetErrors(vbuf, 1, 2)
	want := []SpellError{
		{Line: 1, Col: 6, Len: 5, Type: SpellBad},
		{Line: 2, Col: 0, Len: 4, Type: SpellCaps},
	}
	if len(errs) != len(want) {
		t.Fatalf("got %d errors %+v, want %+v", len(errs), errs, want)
	}
	for i := range want {
		if errs[i] != want[i] {
			t.Errorf("error %d: got %+v, want %+v", i, errs[i], want[i])
		}
	}

	Execute("set nospell")
	if errs := SpellGetErrors(vbuf, 1, 2); len(errs) != 0 {
		t.Errorf("with nospell got %+v, want none", errs)
	}
}

func TestSpellSuggestionsAndApply(t *testing.T) {
	vbuf := spellSetup(t, []string{"Hello wrold here."})
	CursorSetPosition(1, 6)

	word, sugs, ok := SpellGetSuggestions()
	if !ok || word != "wrold" || len(sugs) == 0 || len(sugs) > 9 {
		t.Fatalf("got %q %v ok=%v, want wrold with 1-9 suggestions", word, sugs, ok)
	}
	n := -1
	for i, s := range sugs {
		if s == "would" {
			n = i + 1
		}
	}
	if n < 2 {
		t.Fatalf("\"would\" not among later suggestions: %v", sugs)
	}

	input(string(rune('0'+n)), "z", "=")
	if got := BufferGetLine(vbuf, 1); got != "Hello would here." {
		t.Errorf("after %dz= got %q", n, got)
	}
}

func TestSpellSuggestCallback(t *testing.T) {
	vbuf := spellSetup(t, []string{"Hello wrold here."})
	CursorSetPosition(1, 6)

	input("z", "=")
	if !TakeSpellSuggestRequest() {
		t.Error("z= did not request suggestions")
	}
	if TakeSpellSuggestRequest() {
		t.Error("request was not cleared")
	}
	if got := BufferGetLine(vbuf, 1); got != "Hello wrold here." {
		t.Errorf("plain z= changed the line: %q", got)
	}
}

func TestSpellNormalCommands(t *testing.T) {
	vbuf := spellSetup(t, []string{"Hello wrold and", "more wurds here."})
	CursorSetPosition(1, 0)

	input("]", "s")
	if pos := CursorGetPosition(); pos != [2]int{1, 6} {
		t.Errorf("]s moved to %v, want [1 6]", pos)
	}
	input("z", "G") // accept "wrold" for this session
	if errs := SpellGetErrors(vbuf, 1, 2); len(errs) != 1 || errs[0].Line != 2 {
		t.Errorf("after zG got %+v, want only the line 2 error", errs)
	}
	input("z", "u", "G")
	if errs := SpellGetErrors(vbuf, 1, 2); len(errs) != 2 {
		t.Errorf("after zuG got %+v, want 2 errors", errs)
	}
}

// zg writes the word to 'spellfile' (zG, above, only to an internal list).
// It crashed when libvim's Makefile had no header dependencies for spellfile.o:
// a structs.h change left it compiled against the old synblock_T, so it read
// 'spellfile' from the wrong field and dereferenced NULL.
func TestSpellZgWritesSpellfile(t *testing.T) {
	vbuf := spellSetup(t, []string{"Hello wrold and", "more wurds here."})
	CursorSetPosition(1, 0)

	input("]", "s", "z", "g")
	if errs := SpellGetErrors(vbuf, 1, 2); len(errs) != 1 || errs[0].Line != 2 {
		t.Errorf("after zg got %+v, want only the line 2 error", errs)
	}
	data, err := os.ReadFile(Eval("&spellfile"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(data)); got != "wrold" {
		t.Errorf("spellfile holds %q, want %q", got, "wrold")
	}
}

// z= on a long run of one letter used to freeze the editor: the suggestion
// search is exponential in the word's length (12 l's took over two minutes).
// timeout:1000 bounds it, and asking twice must give the same list, since the
// digit chosen from the list is applied by "{n}z=", which searches again.
func TestSpellSuggestionsAreBounded(t *testing.T) {
	spellSetup(t, []string{"Hello " + strings.Repeat("l", 14) + " here."})
	CursorSetPosition(1, 6)

	start := time.Now()
	_, first, ok1 := SpellGetSuggestions()
	_, second, ok2 := SpellGetSuggestions()
	elapsed := time.Since(start)

	if !ok1 || !ok2 {
		t.Fatal("no suggestions")
	}
	if strings.Join(first, "|") != strings.Join(second, "|") {
		t.Errorf("repeated search differs:\n %v\n %v", first, second)
	}
	// about 2 x 1s on an Apple M-series Mac; generous for slower machines
	if elapsed > 30*time.Second {
		t.Errorf("two searches took %v", elapsed)
	}
	t.Logf("two searches: %v", elapsed)
}
