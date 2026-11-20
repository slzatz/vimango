package editor

import (
	"strings"
	"testing"
)

// Keys in INSERT that vim reads more after, or that do what libvim has not
// compiled in. Until 2026-11-20 (the fork's 8cca54f72) each of these put a
// control character in the note, which was saved with it: CTRL-K (digraphs),
// CTRL-\ CTRL-N, and CTRL-N, CTRL-P and CTRL-X (completion).

const (
	ctrlK   = "\x0b"
	ctrlBsl = "\x1c"
	ctrlN   = "\x0e"
	ctrlP   = "\x10"
	ctrlX   = "\x18"
)

func TestDigraphsInInsert(t *testing.T) {
	e := newRegisterEditor(t, "alpha")
	typeKeys(t, e, "i"+ctrlK+"a:"+ctrlK+"Eu")
	if e.mode != INSERT {
		t.Errorf("mode %v, want INSERT", e.mode)
	}
	assertLines(t, e, "ä€alpha")
	if e.ss[0] != "ä€alpha" {
		t.Errorf("the editor's copy is %q", e.ss[0])
	}

	// <esc> after CTRL-K cancels it, and INSERT goes on
	typeKeys(t, e, ctrlK+"\x1b")
	if e.mode != INSERT {
		t.Errorf("after CTRL-K <esc>: mode %v, want INSERT", e.mode)
	}
	typeKeys(t, e, "\x1b")
	if e.mode != NORMAL {
		t.Errorf("mode %v, want NORMAL", e.mode)
	}
}

func TestDigraphOnTheCommandLine(t *testing.T) {
	e := newRegisterEditor(t, "alpha")
	typeKeys(t, e, ":echo '"+ctrlK+"a:'")
	if e.mode != EX_COMMAND || e.command_line != "echo 'ä'" {
		t.Fatalf("mode %v, command line %q", e.mode, e.command_line)
	}
	said := stripANSI(captureStdout(t, func() { e.ProcessKey('\r') }))
	if !inNormal(e) || !strings.Contains(said, "ä") || strings.Contains(said, "E1") {
		t.Errorf("Enter: mode %v, said %q", e.mode, said)
	}
}

func TestCtrlBackslashCtrlNLeavesInsert(t *testing.T) {
	e := newRegisterEditor(t, "alpha")
	typeKeys(t, e, "ix"+ctrlBsl+ctrlN)
	if !inNormal(e) {
		t.Errorf("mode %v, want NORMAL", e.mode)
	}
	assertLines(t, e, "xalpha")
}

func TestCompletionKeysInsertNothing(t *testing.T) {
	e := newRegisterEditor(t, "alpha")
	typeKeys(t, e, "A"+ctrlN+ctrlP+ctrlX+ctrlN+"z")
	assertLines(t, e, "alphaz")
	out := stripANSI(captureStdout(t, func() { e.sendKeys(ctrlN) }))
	if !strings.Contains(out, "Completion is not available") {
		t.Errorf("CTRL-N said %q", out)
	}
	assertLines(t, e, "alphaz")
}
