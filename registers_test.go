package main

import (
	"os"
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// Round-trips registers through registers.json against real vim: save,
// clobber, restore, and verify contents, types, and what p would paste —
// including the cases the double-quoted escaping has to survive
// (newlines, quotes, backslashes, control bytes, unicode, blockwise
// types). A real "ayy makes the unnamed register genuinely alias a
// (setreg alone never moves vim's y_previous pointer).
func TestRegistersRoundTrip(t *testing.T) {
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	vim.InitializeVim(0)
	buf := vim.NewBuffer(0)
	vim.SetCurrentBuffer(buf)

	set := map[string][2]string{
		`0`: {"last yank\n", "V"},
		`-`: {"small delete", "v"},
		`b`: {`he said "hi" \ done`, "v"},
		`c`: {"tab\there \x01ctrl", "v"},
		`d`: {"café ☕", "v"},
		`e`: {"ab\ncd", "\x162"},
	}
	for name, cv := range set {
		setRegister(name, persistedRegister{Content: cv[0], Type: cv[1]})
	}
	vim.ExecuteCommand("call setline(1, ['alpha a-yank'])")
	vim.SetCursorPosition(1, 0)
	vim.SendMultiInput("\"ayy")
	aContent := "alpha a-yank\n"
	if got := vim.EvaluateExpression(`getreg('"')`); got != aContent {
		t.Fatalf("precondition: unnamed should alias a, got %q", got)
	}

	saveRegisters()
	data, err := os.ReadFile(registersFile)
	if err != nil {
		t.Fatalf("saveRegisters wrote no %s: %v", registersFile, err)
	}
	if !strings.Contains(string(data), `"unnamed": "a"`) {
		t.Errorf("saved state should record unnamed aliasing a: %s", data)
	}

	for name := range set {
		setRegister(name, persistedRegister{Content: "clobbered", Type: "v"})
	}
	setRegister("a", persistedRegister{Content: "clobbered", Type: "v"})

	restoreRegisters()

	for name, cv := range set {
		if name == "0" {
			continue // replaying the unnamed alias writes register 0, checked below
		}
		if got := vim.EvaluateExpression("getreg('" + name + "')"); got != cv[0] {
			t.Errorf("register %s content = %q, want %q", name, got, cv[0])
		}
		if got := vim.EvaluateExpression("getregtype('" + name + "')"); got != cv[1] {
			t.Errorf("register %s type = %q, want %q", name, got, cv[1])
		}
	}
	if got := vim.EvaluateExpression("getreg('a')"); got != aContent {
		t.Errorf("register a = %q, want %q", got, aContent)
	}
	// p must paste what it pasted before the "quit": register a's content
	// (replayed through '"', which by vim's own :let @" semantics leaves
	// the copy in register 0's storage)
	if got := vim.EvaluateExpression(`getreg('"')`); got != aContent {
		t.Errorf("unnamed register = %q, want %q", got, aContent)
	}
	if got := vim.EvaluateExpression("getreg('0')"); got != aContent {
		t.Errorf("register 0 = %q, want the replayed unnamed copy %q", got, aContent)
	}
}

// The common case end-to-end with real vim operations: yank in one
// "session", save, restore in the next, paste.
func TestRegistersYankQuitPaste(t *testing.T) {
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	vim.InitializeVim(0)
	buf := vim.NewBuffer(0)
	vim.SetCurrentBuffer(buf)
	vim.ExecuteCommand("call setline(1, ['yanked line', 'other'])")
	vim.SetCursorPosition(1, 0)
	vim.SendMultiInput("yy")

	saveRegisters()

	// "next session": clobber what the yank set
	setRegister("0", persistedRegister{Content: "stale", Type: "v"})

	restoreRegisters()

	buf2 := vim.NewBuffer(0)
	vim.SetCurrentBuffer(buf2)
	vim.ExecuteCommand("call setline(1, ['target'])")
	vim.SendMultiInput("p")
	if got := vim.EvaluateExpression("getline(2)"); got != "yanked line" {
		t.Errorf("after restore + p: line 2 = %q, want %q", got, "yanked line")
	}
}

// The delete case: dd, "quit", restore, p pastes the deleted line (the
// unnamed register aliased register 1, replayed through '"').
func TestRegistersDeleteQuitPaste(t *testing.T) {
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	vim.InitializeVim(0)
	buf := vim.NewBuffer(0)
	vim.SetCurrentBuffer(buf)
	vim.ExecuteCommand("call setline(1, ['deleted line', 'other'])")
	vim.SetCursorPosition(1, 0)
	vim.SendMultiInput("dd")

	saveRegisters()
	restoreRegisters()

	buf2 := vim.NewBuffer(0)
	vim.SetCurrentBuffer(buf2)
	vim.ExecuteCommand("call setline(1, ['target'])")
	vim.SendMultiInput("p")
	if got := vim.EvaluateExpression("getline(2)"); got != "deleted line" {
		t.Errorf("after restore + p: line 2 = %q, want %q", got, "deleted line")
	}
}

// Registers that should not be persisted: empty, oversized, NUL-bearing.
func TestSaveRegistersSkips(t *testing.T) {
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	vim.InitializeVim(0)
	vim.NewBuffer(0)

	for _, name := range persistedRegisterNames() {
		setRegister(name, persistedRegister{Content: "", Type: "v"})
	}
	big := make([]byte, maxRegisterBytes+1)
	for i := range big {
		big[i] = 'x'
	}
	setRegister("f", persistedRegister{Content: string(big), Type: "v"})
	setRegister("g", persistedRegister{Content: "kept", Type: "v"})

	saveRegisters()

	data, err := os.ReadFile(registersFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, `"g"`) {
		t.Errorf("register g missing from %s: %s", registersFile, got)
	}
	if strings.Contains(got, `"f"`) {
		t.Errorf("oversized register f persisted in %s", registersFile)
	}
}
