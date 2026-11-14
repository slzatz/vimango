package main

import "testing"

// The two layouts vimango gives an editor, on a terminal of 40 rows by 120
// columns: the TUI's right pane, and the whole of a host app's window.
func TestEditorLayouts(t *testing.T) {
	tui := &Screen{screenLines: 40, screenCols: 120, textLines: 37, divider: 48, Session: &Session{}}
	want := editorLayout{
		top_margin: 2, left_margin: 48, screenlines: 37, screencols: 71,
		statusBar: true, corner: true,
		message: messageLine{row: 40, col: 49, width: 72},
		aside:   messageLine{row: 40, col: 1, width: 48},
	}
	if got := tui.editorLayout(); got != want {
		t.Errorf("the TUI's layout is\n%+v, want\n%+v", got, want)
	}

	host := &Screen{screenLines: 40, screenCols: 120, textLines: 37, divider: 1, Session: &Session{editorOnly: true}}
	want = editorLayout{
		top_margin: 1, left_margin: 1, screenlines: 38, screencols: 118,
		message: messageLine{row: 40, col: 2, width: 119},
		aside:   messageLine{row: 40, col: 2, width: 119},
	}
	if got := host.editorLayout(); got != want {
		t.Errorf("the --editor layout is\n%+v, want\n%+v", got, want)
	}
}
