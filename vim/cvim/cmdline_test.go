//go:build cgo && !windows

package cvim

import "testing"

// Empirical probe of libvim's cmdline API ahead of the EX_COMMAND rework:
// verifies that keys fed while vim is in cmdline mode accumulate inertly
// (nothing executes until CR), that GetText/GetPosition track editing keys
// (<left>, <bs>, <c-w>, <c-u>), and that Esc cancels cleanly.
func TestCommandLineAPI(t *testing.T) {
	VimInit(0)
	vbuf := CBufferNew(0)
	CBufferSetCurrent(vbuf)
	CBufferSetLines(vbuf, 0, 0, []string{"alpha bravo"}, 1)

	// ':' should put vim in cmdline mode (mode 8), type ':'
	Input(":")
	if m := GetMode(); m != 8 {
		t.Fatalf("after ':' expected mode 8 (cmdline), got %d", m)
	}
	if ty := CommandLineGetType(); ty != ':' {
		t.Errorf("CommandLineGetType = %q, want ':'", ty)
	}

	Input2("open 123")
	if txt := CommandLineGetText(); txt != "open 123" {
		t.Errorf("GetText = %q, want %q", txt, "open 123")
	}
	posEnd := CommandLineGetPosition()
	t.Logf("position after typing 8 chars: %d", posEnd)

	// cursor movement + insert at cursor
	Key("<left>")
	Key("<left>")
	if pos := CommandLineGetPosition(); pos != posEnd-2 {
		t.Errorf("after 2x <left>: position = %d, want %d", pos, posEnd-2)
	}
	Input("X")
	if txt := CommandLineGetText(); txt != "open 1X23" {
		t.Errorf("insert at cursor: GetText = %q, want %q", txt, "open 1X23")
	}

	// backspace deletes before the cursor, not at end of line
	Key("<bs>")
	if txt := CommandLineGetText(); txt != "open 123" {
		t.Errorf("after <bs>: GetText = %q, want %q", txt, "open 123")
	}

	// <c-w> deletes the word before the cursor ("123" minus the 2 chars
	// right of the cursor -> "open " + "23"? log actual, assert loosely)
	Key("<c-w>")
	t.Logf("after <c-w>: GetText = %q, pos = %d", CommandLineGetText(), CommandLineGetPosition())

	// <c-u> clears the line
	Key("<c-u>")
	t.Logf("after <c-u>: GetText = %q, pos = %d", CommandLineGetText(), CommandLineGetPosition())

	// still in cmdline mode; buffer untouched (nothing executed without CR)
	if m := GetMode(); m != 8 {
		t.Errorf("still expected mode 8, got %d", m)
	}

	// type a real ex command that would modify the buffer, but never send CR
	Input2("s/alpha/omega/")
	if line := BufferGetLine(vbuf, 1); line != "alpha bravo" {
		t.Errorf("buffer modified without CR: %q", line)
	}

	// Esc cancels: back to normal mode, buffer still untouched
	Key("<esc>")
	if m := GetMode(); m == 8 {
		t.Errorf("after <esc> still in cmdline mode")
	}
	if line := BufferGetLine(vbuf, 1); line != "alpha bravo" {
		t.Errorf("buffer modified after <esc>: %q", line)
	}
	t.Logf("after <esc>: GetText = %q, type = %d", CommandLineGetText(), CommandLineGetType())

	// sanity: CR does execute when vim is allowed to see it
	Input(":")
	Input2("s/alpha/omega/")
	Key("<cr>")
	if line := BufferGetLine(vbuf, 1); line != "omega bravo" {
		t.Errorf("CR did not execute the command: %q", line)
	}

	// tab-completion write-back sequence: replace the whole cmdline even
	// when the cursor is mid-line (<c-e> end, <c-u> clear, re-input)
	Input(":")
	Input2("open foo")
	Key("<left>")
	Key("<left>")
	Key("<c-e>")
	if pos := CommandLineGetPosition(); pos != 8 {
		t.Errorf("<c-e> did not move to end: pos = %d", pos)
	}
	Key("<c-u>")
	Input2("open foobar")
	if txt := CommandLineGetText(); txt != "open foobar" {
		t.Errorf("write-back failed: GetText = %q", txt)
	}
	Key("<esc>")

	// history: histadd via Execute, then <up> in cmdline recalls it
	Execute("call histadd(':', 'open my note')")
	Input(":")
	Key("<up>")
	t.Logf("after histadd + <up>: GetText = %q", CommandLineGetText())
	if txt := CommandLineGetText(); txt != "open my note" {
		t.Errorf("history recall: GetText = %q, want %q", txt, "open my note")
	}
	Key("<esc>")
}
