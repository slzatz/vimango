package main

import (
	"testing"
)

// The organizer takes a paste into the title being edited and onto its
// command line, as one line. Anywhere else the paste is dropped: as keys it
// ran as the organizer's commands.
func TestOrganizerPaste(t *testing.T) {
	o, _ := newDBOrganizer(t, "alpha", "bravo")

	// NORMAL: "dd" is not run, "m" does not mark
	captureStdout(t, func() { o.pasteText("ddm\rx") })
	if got := o.vbuf.Lines(); got[0] != "alpha" || got[1] != "bravo" || len(o.marked_entries) != 0 {
		t.Fatalf("a paste in NORMAL mode did something: %q, %d marked", got, len(o.marked_entries))
	}
	if o.mode != NORMAL {
		t.Fatalf("mode %v, want NORMAL", o.mode)
	}

	// the title: one line, where the cursor is, and the row stays the row
	captureStdout(t, func() {
		o.organizerProcessKey('A')
		if o.pasteText(" one\r two\r") != RedrawPartial {
			t.Error("a paste into the title did not ask for the row to be redrawn")
		}
	})
	if got := o.vbuf.Lines(); len(got) != 2 || got[0] != "alpha one  two" || o.rows[0].title != got[0] {
		t.Errorf("titles %q, row %q", got, o.rows[0].title)
	}
	if o.mode != INSERT || o.fr != 0 || !o.rows[0].dirty {
		t.Errorf("mode %v, row %d, dirty %v", o.mode, o.fr, o.rows[0].dirty)
	}
	captureStdout(t, func() { o.organizerProcessKey('\x1b') })

	// the command line: nothing is run by the paste's line end
	captureStdout(t, func() {
		o.organizerProcessKey(':')
		o.pasteText("find café\r")
	})
	if o.mode != COMMAND_LINE || o.command_line != "find café" {
		t.Errorf("mode %v, command line %q", o.mode, o.command_line)
	}
}
