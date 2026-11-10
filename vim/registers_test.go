package vim

import (
	"strings"
	"testing"
)

// Empirical probe of libvim's register eval functions ahead of register
// persistence (viminfo-style save/restore across vimango runs): verifies
// that getreg/getregtype/setreg are in the compiled eval table, that Eval
// returns multi-line register contents with embedded newlines intact, and
// that setreg round-trips charwise and linewise types.
func TestRegisterEvalRoundTrip(t *testing.T) {
	InitializeVim(0)
	vbuf := NewBuffer(0)
	SetCurrentBuffer(vbuf)
	vbuf.SetLines(0, 0, []string{"alpha", "bravo", "charlie"})

	// linewise: setreg with 'V' then read back content and type
	ExecuteCommand(`call setreg('a', "one\ntwo", 'V')`)
	got := EvaluateExpression(`getreg('a')`)
	if got != "one\ntwo\n" {
		t.Errorf("linewise getreg('a') = %q, want %q", got, "one\ntwo\n")
	}
	if ty := EvaluateExpression(`getregtype('a')`); ty != "V" {
		t.Errorf("getregtype('a') = %q, want %q", ty, "V")
	}

	// charwise: no trailing newline
	ExecuteCommand(`call setreg('b', "hello", 'v')`)
	if got := EvaluateExpression(`getreg('b')`); got != "hello" {
		t.Errorf("charwise getreg('b') = %q, want %q", got, "hello")
	}
	if ty := EvaluateExpression(`getregtype('b')`); ty != "v" {
		t.Errorf("getregtype('b') = %q, want %q", ty, "v")
	}

	// quotes and backslashes survive the double-quoted round trip
	ExecuteCommand(`call setreg('c', "he said \"hi\" \\ done", 'v')`)
	if got := EvaluateExpression(`getreg('c')`); got != `he said "hi" \ done` {
		t.Errorf("escaped getreg('c') = %q, want %q", got, `he said "hi" \ done`)
	}

	// a real yank lands in the unnamed register as linewise
	SetCursorPosition(1, 0)
	SendInput("y")
	SendInput("y")
	if got := EvaluateExpression(`getreg('"')`); got != "alpha\n" {
		t.Errorf("after yy: getreg('\"') = %q, want %q", got, "alpha\n")
	}
	if ty := EvaluateExpression(`getregtype('"')`); ty != "V" {
		t.Errorf("after yy: getregtype('\"') = %q, want %q", ty, "V")
	}

	// setreg on the unnamed register is what restore-on-startup does;
	// verify p pastes the restored content
	ExecuteCommand(`call setreg('"', "restored", 'v')`)
	SendInput("p")
	lines := vbuf.Lines()
	joined := strings.Join(lines, "|")
	if !strings.Contains(joined, "restored") {
		t.Errorf("after setreg + p: buffer = %q, want it to contain %q", joined, "restored")
	}

	// blockwise type string (CTRL-V + width) comes back readable
	ExecuteCommand("call setreg('d', \"ab\\ncd\", \"\\<C-v>2\")")
	ty := EvaluateExpression(`getregtype('d')`)
	if len(ty) == 0 || ty[0] != 0x16 {
		t.Errorf("blockwise getregtype('d') = %q, want leading CTRL-V (0x16)", ty)
	}
}
