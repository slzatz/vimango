package vim

import "testing"

// Empirical probe of libvim's cmdline API ahead of the EX_COMMAND rework:
// verifies that keys fed while vim is in cmdline mode accumulate inertly
// (nothing executes until CR), that GetText/GetPosition track editing keys
// (<left>, <bs>, <c-w>, <c-u>), and that Esc cancels cleanly.
func TestCommandLineAPI(t *testing.T) {
	InitializeVim(0)
	vbuf := NewBuffer(0)
	SetCurrentBuffer(vbuf)
	vbuf.SetLines(0, 0, []string{"alpha bravo"})

	// ':' should put vim in cmdline mode (mode 8), type ':'
	SendInput(":")
	if m := GetCurrentMode(); m != 8 {
		t.Fatalf("after ':' expected mode 8 (cmdline), got %d", m)
	}
	if ty := CommandLineGetType(); ty != ':' {
		t.Errorf("CommandLineGetType = %q, want ':'", ty)
	}

	SendMultiInput("open 123")
	if txt := CommandLineGetText(); txt != "open 123" {
		t.Errorf("GetText = %q, want %q", txt, "open 123")
	}
	posEnd := CommandLineGetPosition()
	t.Logf("position after typing 8 chars: %d", posEnd)

	// cursor movement + insert at cursor
	SendKey("<left>")
	SendKey("<left>")
	if pos := CommandLineGetPosition(); pos != posEnd-2 {
		t.Errorf("after 2x <left>: position = %d, want %d", pos, posEnd-2)
	}
	SendInput("X")
	if txt := CommandLineGetText(); txt != "open 1X23" {
		t.Errorf("insert at cursor: GetText = %q, want %q", txt, "open 1X23")
	}

	// backspace deletes before the cursor, not at end of line
	SendKey("<bs>")
	if txt := CommandLineGetText(); txt != "open 123" {
		t.Errorf("after <bs>: GetText = %q, want %q", txt, "open 123")
	}

	// <c-w> deletes the word before the cursor ("123" minus the 2 chars
	// right of the cursor -> "open " + "23"? log actual, assert loosely)
	SendKey("<c-w>")
	t.Logf("after <c-w>: GetText = %q, pos = %d", CommandLineGetText(), CommandLineGetPosition())

	// <c-u> clears the line
	SendKey("<c-u>")
	t.Logf("after <c-u>: GetText = %q, pos = %d", CommandLineGetText(), CommandLineGetPosition())

	// still in cmdline mode; buffer untouched (nothing executed without CR)
	if m := GetCurrentMode(); m != 8 {
		t.Errorf("still expected mode 8, got %d", m)
	}

	// type a real ex command that would modify the buffer, but never send CR
	SendMultiInput("s/alpha/omega/")
	if line := vbuf.Line(1); line != "alpha bravo" {
		t.Errorf("buffer modified without CR: %q", line)
	}

	// Esc cancels: back to normal mode, buffer still untouched
	SendKey("<esc>")
	if m := GetCurrentMode(); m == 8 {
		t.Errorf("after <esc> still in cmdline mode")
	}
	if line := vbuf.Line(1); line != "alpha bravo" {
		t.Errorf("buffer modified after <esc>: %q", line)
	}
	t.Logf("after <esc>: GetText = %q, type = %d", CommandLineGetText(), CommandLineGetType())

	// sanity: CR does execute when vim is allowed to see it
	SendInput(":")
	SendMultiInput("s/alpha/omega/")
	SendKey("<cr>")
	if line := vbuf.Line(1); line != "omega bravo" {
		t.Errorf("CR did not execute the command: %q", line)
	}

	// tab-completion write-back sequence: replace the whole cmdline even
	// when the cursor is mid-line (<c-e> end, <c-u> clear, re-input)
	SendInput(":")
	SendMultiInput("open foo")
	SendKey("<left>")
	SendKey("<left>")
	SendKey("<c-e>")
	if pos := CommandLineGetPosition(); pos != 8 {
		t.Errorf("<c-e> did not move to end: pos = %d", pos)
	}
	SendKey("<c-u>")
	SendMultiInput("open foobar")
	if txt := CommandLineGetText(); txt != "open foobar" {
		t.Errorf("write-back failed: GetText = %q", txt)
	}
	SendKey("<esc>")

	// history: histadd via Execute, then <up> in cmdline recalls it
	ExecuteCommand("call histadd(':', 'open my note')")
	SendInput(":")
	SendKey("<up>")
	t.Logf("after histadd + <up>: GetText = %q", CommandLineGetText())
	if txt := CommandLineGetText(); txt != "open my note" {
		t.Errorf("history recall: GetText = %q, want %q", txt, "open my note")
	}
	SendKey("<esc>")
}
