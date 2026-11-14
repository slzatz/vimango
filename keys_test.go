package main

import (
	"testing"

	"github.com/slzatz/vimango/internal/pane"
	"github.com/slzatz/vimango/terminal"
)

// main's key codes are aliases for the reader's, not a second hand-maintained
// copy -- the two blocks had already drifted once.
func TestKeyConstantsTrackTheReader(t *testing.T) {
	for _, tc := range []struct {
		name       string
		main, term int
	}{
		{"ARROW_LEFT", ARROW_LEFT, terminal.KeyArrowLeft},
		{"DEL_KEY", DEL_KEY, terminal.KeyDelete},
		{"HOME_KEY", HOME_KEY, terminal.KeyHome},
		{"END_KEY", END_KEY, terminal.KeyEnd},
		{"PAGE_DOWN", PAGE_DOWN, terminal.KeyPageDown},
	} {
		if tc.main != tc.term {
			t.Errorf("%s = %d but reader says %d", tc.name, tc.main, tc.term)
		}
	}
	if pane.IsSyntheticKey(BACKSPACE) {
		t.Error("BACKSPACE is a real byte and must not be treated as synthetic")
	}
	if !pane.IsSyntheticKey(terminal.KeyF1) {
		t.Error("F1 should be treated as synthetic")
	}
}
