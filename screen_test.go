package main

import (
	"testing"

	"github.com/slzatz/vimango/internal/editor"
)

// The two layouts vimango gives an editor, on a terminal of 40 rows by 120
// columns: the TUI's right pane, and the whole of a host app's window.
func TestEditorLayouts(t *testing.T) {
	tui := &Screen{screenLines: 40, screenCols: 120, textLines: 37, divider: 48, Session: &Session{}}
	want := editor.Layout{
		TopMargin: 2, LeftMargin: 48, ScreenLines: 37, ScreenCols: 71,
		StatusBar: true, Corner: true,
		Message: editor.MessageLine{Row: 40, Col: 49, Width: 72},
		Aside:   editor.MessageLine{Row: 40, Col: 1, Width: 48},
	}
	if got := tui.editorLayout(); got != want {
		t.Errorf("the TUI's layout is\n%+v, want\n%+v", got, want)
	}

	host := &Screen{screenLines: 40, screenCols: 120, textLines: 37, divider: 1, Session: &Session{editorOnly: true}}
	want = editor.Layout{
		TopMargin: 1, LeftMargin: 1, ScreenLines: 38, ScreenCols: 118,
		Message: editor.MessageLine{Row: 40, Col: 2, Width: 119},
		Aside:   editor.MessageLine{Row: 40, Col: 2, Width: 119},
	}
	if got := host.editorLayout(); got != want {
		t.Errorf("the --editor layout is\n%+v, want\n%+v", got, want)
	}
}
