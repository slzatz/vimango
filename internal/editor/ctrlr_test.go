package editor

import (
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// CTRL-R: a register's contents, in INSERT and on a command line, and the
// expression register from NORMAL ("=). vim waits for the register's name,
// and for "=" a whole line; libvim cannot wait. It used to insert a literal
// ^R in INSERT, type the register's name on a command line, and never come
// back from "=.

const ctrlR = "\x12"

func newRegisterEditor(t *testing.T, lines ...string) *Editor {
	t.Helper()
	e := newTestEditor(t, lines...)
	vim.ExecuteCommand("mapclear | mapclear!")
	vim.ExecuteCommand("let @a = 'REG'")
	vim.ExecuteCommand(`let @b = "two\nlines\n"`)
	t.Cleanup(func() {
		vim.SendKey("<esc>")
		vim.SendKey("<esc>")
		vim.ExecuteCommand("mapclear | mapclear!")
	})
	return e
}

// inNormal: vim calls NORMAL "busy" for a moment after it comes back from a
// command line, and the editor treats the two alike.
func inNormal(e *Editor) bool { return e.mode == NORMAL || e.mode == NORMAL_BUSY }

// typeKeys is sendKeys with the terminal's output swallowed.
func typeKeys(t *testing.T, e *Editor, keys string) {
	t.Helper()
	captureStdout(t, func() { e.sendKeys(keys) })
}

func TestCtrlRInInsertMode(t *testing.T) {
	e := newRegisterEditor(t, "alpha")

	typeKeys(t, e, "i"+ctrlR+"a")
	if e.mode != INSERT {
		t.Errorf("mode %v, want INSERT", e.mode)
	}
	assertLines(t, e, "REGalpha")
	if e.fc != 3 {
		t.Errorf("cursor column %d, want 3", e.fc)
	}

	// a register of lines
	typeKeys(t, e, "\x1bu")
	typeKeys(t, e, "i"+ctrlR+"b")
	assertLines(t, e, "two", "lines", "alpha")

	// literally
	typeKeys(t, e, "\x1bu")
	typeKeys(t, e, "i"+ctrlR+ctrlR+"a\x1b")
	assertLines(t, e, "REGalpha")

	// <esc> in place of the name: no register, and INSERT goes on
	typeKeys(t, e, "u")
	out := captureStdout(t, func() { e.sendKeys("i" + ctrlR + "\x1b") })
	if e.mode != INSERT || vim.GetCurrentMode() != vim.ModeInsert {
		t.Errorf("after CTRL-R <esc>: mode %v, want INSERT", e.mode)
	}
	if !strings.Contains(out, "-- INSERT --") {
		t.Error("still in INSERT, and the message line does not say so")
	}
	typeKeys(t, e, "x")
	assertLines(t, e, "xalpha")
}

// CTRL-R = asks for an expression, on a command line of its own.
func TestCtrlREqualsInInsertMode(t *testing.T) {
	e := newRegisterEditor(t, "alpha")

	typeKeys(t, e, "i"+ctrlR+"=")
	if e.mode != SEARCH || e.searchPrefix != "=" {
		t.Fatalf("after CTRL-R =: mode %v, prefix %q; want the = line", e.mode, e.searchPrefix)
	}
	typeKeys(t, e, "6*7")
	if e.command_line != "6*7" {
		t.Errorf("the line shows %q, want \"6*7\"", e.command_line)
	}
	out := captureStdout(t, func() { e.sendKeys("\r") })
	if e.mode != INSERT {
		t.Errorf("after the expression: mode %v, want INSERT", e.mode)
	}
	if !strings.Contains(out, "-- INSERT --") {
		t.Error("back in INSERT, and the message line does not say so")
	}
	assertLines(t, e, "42alpha")

	// abandoned with <esc>: nothing inserted, still INSERT
	typeKeys(t, e, ctrlR+"=6*7\x1b")
	if e.mode != INSERT {
		t.Errorf("after an abandoned expression: mode %v, want INSERT", e.mode)
	}
	typeKeys(t, e, "x\x1b")
	assertLines(t, e, "42xalpha")
	if e.mode != NORMAL {
		t.Errorf("mode %v, want NORMAL", e.mode)
	}
}

// The reason to want it: a mapping that inserts something computed.
func TestCtrlRInAMapping(t *testing.T) {
	e := newRegisterEditor(t, "alpha")
	vim.ExecuteCommand("inoremap ,d <C-r>=toupper('date')<CR>!")
	vim.ExecuteCommand("inoremap Q <C-r>a")

	typeKeys(t, e, "iQ-,d")
	if e.mode != INSERT {
		t.Errorf("mode %v, want INSERT", e.mode)
	}
	assertLines(t, e, "REG-DATE!alpha")
}

func TestCtrlROnTheExLine(t *testing.T) {
	e := newRegisterEditor(t, "alpha bravo", "charlie")

	typeKeys(t, e, ":s/"+ctrlR+"\x17") // CTRL-R CTRL-W: the word under the cursor
	if e.mode != EX_COMMAND || e.command_line != "s/alpha" {
		t.Fatalf("mode %v, line %q; want EX_COMMAND, \"s/alpha\"", e.mode, e.command_line)
	}
	typeKeys(t, e, "/"+ctrlR+"a/\r")
	assertLines(t, e, "REG bravo", "charlie")
	if e.mode != NORMAL {
		t.Errorf("after the command: mode %v", e.mode)
	}

	// <esc> in place of the name leaves the line open, as it was
	typeKeys(t, e, ":echo "+ctrlR+"\x1b")
	if e.mode != EX_COMMAND || e.command_line != "echo " || vim.GetCurrentMode() != vim.ModeCmdline {
		t.Errorf("after CTRL-R <esc>: mode %v, line %q; want the line still open", e.mode, e.command_line)
	}
	typeKeys(t, e, "\x1b")
	if !inNormal(e) {
		t.Errorf("after a second <esc>: mode %v, want NORMAL", e.mode)
	}
}

// CTRL-R = on the ex line: the line is put aside for the expression's, whose
// <cr> is vim's -- vimango takes the <cr> of a ':' line for itself, and must
// not take this one.
func TestCtrlREqualsOnTheExLine(t *testing.T) {
	e := newRegisterEditor(t, "alpha")

	typeKeys(t, e, ":s/alpha/"+ctrlR+"=")
	if e.mode != SEARCH || e.searchPrefix != "=" {
		t.Fatalf("after CTRL-R =: mode %v, prefix %q; want the = line", e.mode, e.searchPrefix)
	}
	typeKeys(t, e, "6*7\r")
	if e.mode != EX_COMMAND || e.command_line != "s/alpha/42" {
		t.Fatalf("after the expression: mode %v, line %q; want EX_COMMAND, \"s/alpha/42\"", e.mode, e.command_line)
	}
	assertLines(t, e, "alpha") // nothing has run yet
	typeKeys(t, e, "/\r")
	assertLines(t, e, "42")

	// <esc> ends the expression only
	typeKeys(t, e, ":s/42/"+ctrlR+"=1+1\x1b")
	if e.mode != EX_COMMAND || e.command_line != "s/42/" {
		t.Errorf("after an abandoned expression: mode %v, line %q", e.mode, e.command_line)
	}
	typeKeys(t, e, "x/\r")
	assertLines(t, e, "x")
}

func TestCtrlRInASearch(t *testing.T) {
	e := newRegisterEditor(t, "alpha", "bravo REG", "charlie")

	typeKeys(t, e, "/"+ctrlR+"a")
	if e.mode != SEARCH || e.searchPrefix != "/" || e.command_line != "REG" {
		t.Fatalf("mode %v, prefix %q, line %q; want the search line with \"REG\"", e.mode, e.searchPrefix, e.command_line)
	}
	typeKeys(t, e, "\r")
	if !inNormal(e) || e.fr != 1 || e.fc != 6 {
		t.Errorf("mode %v, cursor %d,%d; want NORMAL at 1,6", e.mode, e.fr, e.fc)
	}
}

// "= never came back from libvim: the editor froze, the note unsaved.
func TestExpressionRegisterInNormalMode(t *testing.T) {
	e := newRegisterEditor(t, "alpha")

	typeKeys(t, e, `"=`)
	if e.mode != SEARCH || e.searchPrefix != "=" {
		t.Fatalf(`after "=: mode %v, prefix %q; want the = line`, e.mode, e.searchPrefix)
	}
	typeKeys(t, e, "6*7\r")
	if !inNormal(e) {
		t.Fatalf("after the expression: mode %v, want NORMAL", e.mode)
	}
	typeKeys(t, e, "P")
	assertLines(t, e, "42alpha")

	// abandoned: the next key is an ordinary command
	typeKeys(t, e, `"=1+1`+"\x1b")
	if !inNormal(e) {
		t.Errorf("after an abandoned expression: mode %v", e.mode)
	}
	typeKeys(t, e, "x")
	assertLines(t, e, "4alpha")

	// @= runs the value as keys: another x, on the a the cursor is now on
	typeKeys(t, e, "@='x'\r")
	assertLines(t, e, "4lpha")
}
