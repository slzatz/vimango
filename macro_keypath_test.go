package main

import (
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// The same macro as vim/cvim's TestMacroRecordAndPlayback, but driven through
// editorProcessKey so the whole Go interception layer is in the path. Only keys
// that reach vim land in the recording register, so this is the test that
// catches an interception regression: insert-mode keys in particular must keep
// falling straight through -- INSERT deliberately has no case in the mode
// switch -- or "A  <esc>" records as a bare "A<esc>" and the replay silently
// appends nothing.
func TestMacroThroughEditorProcessKey(t *testing.T) {
	e := newTestEditor(t, "alpha", "bravo", "charlie", "delta")

	// qa A <space> <space> <esc> j q
	for _, c := range []int{'q', 'a', 'A', ' ', ' ', '\x1b', 'j', 'q'} {
		e.editorProcessKey(c)
	}

	if got := vim.EvaluateExpression(`getreg('a')`); got != "A  \x1bj" {
		t.Fatalf("getreg('a') = %q, want %q", got, "A  \x1bj")
	}

	e.editorProcessKey('@')
	e.editorProcessKey('a')

	got := strings.Join(e.vbuf.Lines()[:4], "|")
	want := "alpha  |bravo  |charlie|delta"
	if got != want {
		t.Errorf("after @a: %q, want %q", got, want)
	}
}
