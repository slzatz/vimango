package vim

import (
	"strings"
	"testing"
)

// libvim still has working q/@ macro record and playback: do_record (ops.c),
// the recording hook in gotchars (getchar.c) and the typebuf drain loop in
// sm_execute (state_machine.c) all have to stay intact for this to pass.
//
// Keys are fed one per SendInput() call because that is how vimango drives libvim
// — one vimInput() per keystroke — and the escape goes through SendKey(), which is
// the vimKey()/replace_termcodes() entry point vimango's editorProcessKey uses
// for <esc>. Playing the macro back is the opposite shape: the whole register
// is drained inside the single SendInput("a") call.
func TestMacroRecordAndPlayback(t *testing.T) {
	InitializeVim(0)
	vbuf := NewBuffer(0)
	SetCurrentBuffer(vbuf)
	vbuf.SetLines(0, 0, []string{"alpha", "bravo", "charlie", "delta"})
	SetCursorPosition(1, 0)

	// qa A <space> <space> <esc> j q — append a markdown hard line break and
	// step down a line
	for _, k := range []string{"q", "a", "A", " ", " "} {
		SendInput(k)
	}
	SendKey("<esc>")
	SendInput("j")
	SendInput("q")

	// the trailing q that stopped the recording is trimmed by get_recorded()
	if got := EvaluateExpression(`getreg('a')`); got != "A  \x1bj" {
		t.Fatalf("getreg('a') = %q, want %q", got, "A  \x1bj")
	}

	SendInput("@")
	SendInput("a")

	// and again with a count, which goes through do_execreg's repeat loop
	SendInput("2")
	SendInput("@")
	SendInput("a")

	got := strings.Join(vbuf.Lines()[:4], "|")
	want := "alpha  |bravo  |charlie  |delta  "
	if got != want {
		t.Errorf("after @a and 2@a: %q, want %q", got, want)
	}
}
