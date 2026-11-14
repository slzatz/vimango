package main

import (
	"fmt"
	"strings"
	"testing"
)

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

// The editor draws by the layout it was given and asks nothing else: not the
// session's mode, not the screen's size. Each editor here is given the layout
// of the *other* program, and draws that.
func TestEditorDrawsByItsLayout(t *testing.T) {
	inTUI := newTestEditor(t, "alpha")
	inTUI.id, inTUI.title = 7, "a note"
	inTUI.editorLayout = (&Screen{screenLines: 40, screenCols: 100, textLines: 37, divider: 1, Session: &Session{editorOnly: true}}).editorLayout()

	inHost := newTestEditor(t, "alpha")
	inHost.id, inHost.title = 7, "a note"
	inHost.Session.editorOnly = true // the flag alone changes nothing the editor draws
	inHost.editorLayout = (&Screen{screenLines: 40, screenCols: 100, textLines: 37, divider: 40, Session: &Session{}}).editorLayout()

	for _, c := range []struct {
		name          string
		e             *Editor
		bar, corner   bool
		aside, cursor string
	}{
		{"a TUI editor with the host's layout", inTUI, false, false, "\x1b[40;2H", "40;4"},
		{"a host's editor with the TUI's layout", inHost, true, true, "\x1b[40;1H", "40;43"},
	} {
		e := c.e
		if bar := captureStdout(t, e.drawStatusBar); strings.Contains(bar, "a note") != c.bar {
			t.Errorf("%s: status bar drawn %v, want %v", c.name, !c.bar, c.bar)
		}
		// the corner is on the row above the text, at the frame's column
		corner := fmt.Sprintf("\x1b[%d;%dH\x1b[37;1mk", e.top_margin-1, e.left_margin+e.screencols+1)
		if frame := captureStdout(t, e.drawFrame); strings.Contains(frame, corner) != c.corner {
			t.Errorf("%s: corner drawn %v, want %v", c.name, !c.corner, c.corner)
		}
		if out := captureStdout(t, func() { e.ShowMessage(BL, "saved") }); !strings.HasPrefix(out, c.aside) {
			t.Errorf("%s: what it says beside the note went to %q, want it to start %q", c.name, out, c.aside)
		}
		// the cursor on a command line: after the ':' and the one character typed
		captureStdout(t, func() { e.sendKeys(":s") })
		row, col, show := e.cursor()
		if got := fmt.Sprintf("%d;%d", row, col); !show || got != c.cursor {
			t.Errorf("%s: cursor on the command line at %s (shown %v), want %s", c.name, got, show, c.cursor)
		}
		captureStdout(t, func() { e.editorProcessKey('\x1b') })
	}
}
