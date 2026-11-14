package pane

import (
	"testing"

	"github.com/slzatz/vimango/terminal"
)

// Only the keys vim has a name for are sent to it by name; a function key is
// a synthetic code with none, and Backspace is a real byte with one.
func TestKeysVimUnderstands(t *testing.T) {
	for name, code := range map[string]int{"Home": terminal.KeyHome, "PageUp": terminal.KeyPageUp, "ArrowLeft": terminal.KeyArrowLeft, "Backspace": 127} {
		if _, ok := VimKeyName(code); !ok {
			t.Errorf("%s (%d) has no name for vim", name, code)
		}
	}
	for name, code := range map[string]int{"F1": terminal.KeyF1, "F12": terminal.KeyF12, "Ins": terminal.KeyIns, "End": terminal.KeyEnd, "a": 'a'} {
		if _, ok := VimKeyName(code); ok {
			t.Errorf("%s (%d) has a name for vim; it is not sent by name", name, code)
		}
	}
	if IsSyntheticKey(127) {
		t.Error("Backspace is a real byte and must not be treated as synthetic")
	}
	if !IsSyntheticKey(terminal.KeyF1) || IsSyntheticKey('a') {
		t.Error("F1 is synthetic and 'a' is not")
	}
}
