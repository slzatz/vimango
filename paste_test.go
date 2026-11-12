package main

import (
	"strings"
	"testing"
	"time"

	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// Text pasted from the terminal (paste.go). The editors here have vimango.vim
// sourced: its <CR> mapping, and vim's 'autoindent', are what a paste typed
// as keys used to trip over.

func assertBuffer(t *testing.T, e *Editor, want ...string) {
	t.Helper()
	got := e.vbuf.Lines()
	if len(got) != len(want) {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
	assertLines(t, e, want...)
}

func assertPasteOptionOff(t *testing.T) {
	t.Helper()
	if vim.EvaluateExpression("&paste") != "0" || vim.EvaluateExpression("&autoindent") != "1" {
		t.Errorf("after the paste: 'paste' is %s and 'autoindent' %s, want 0 and 1",
			vim.EvaluateExpression("&paste"), vim.EvaluateExpression("&autoindent"))
	}
}

func TestPastedTextIsCleaned(t *testing.T) {
	for raw, want := range map[string]string{
		"a\rb\r":             "a\nb\n",
		"a\r\nb\nc":          "a\nb\nc",
		"\tcafé 日本":          "\tcafé 日本",
		"a\x1b:q!\rb":        "a:q!\nb",
		"a\x00\x07\x7fb":     "ab",
		"x\u0085y\u009bz":    "xyz", // C1 controls
		"":                   "",
		"\x1b[201~":          "[201~",
		"- one\r  - two\r\r": "- one\n  - two\n\n",
	} {
		if got := pastedText(raw); got != want {
			t.Errorf("pastedText(%q) = %q, want %q", raw, got, want)
		}
	}
	if got := onOneLine("\n  a\n\tb\n\n"); got != "  a  b" {
		t.Errorf("onOneLine = %q", got)
	}
}

// A list pasted in INSERT mode arrives as it was copied: the <CR> mapping
// gives no line a second marker, and the insert goes on with the mapping
// working.
func TestPasteAListInInsertMode(t *testing.T) {
	e := newVimrcEditor(t, "- foo")

	e.sendKeys("A")
	if !e.pasteText("\r- bar\r- baz") {
		t.Error("a paste that changed the text returned redraw=false")
	}
	assertBuffer(t, e, "- foo", "- bar", "- baz")
	if e.mode != INSERT || vim.GetCurrentMode() != vim.ModeInsert {
		t.Errorf("mode %v, want INSERT", e.mode)
	}
	if e.ss[2] != "- baz" || e.fr != 2 || e.fc != 5 {
		t.Errorf("the editor's copy: %q, cursor %d,%d", e.ss, e.fr, e.fc)
	}
	assertPasteOptionOff(t)

	e.sendKeys("\rqux\x1b")
	assertBuffer(t, e, "- foo", "- bar", "- baz", "- qux")
}

// Indented text keeps its own indentation: 'autoindent' adds none.
func TestPasteIndentedText(t *testing.T) {
	e := newVimrcEditor(t, "")
	text := []string{"func f() {", "    if x {", "        return", "    }", "\ttab", "}"}

	e.sendKeys("i")
	e.pasteText(strings.Join(text, "\r"))
	e.sendKeys("\x1b")
	assertBuffer(t, e, text...)
	assertPasteOptionOff(t)
}

// Neither a mapping nor an abbreviation sees pasted text, and a control
// character in it is not a key.
func TestPasteIsNotTyped(t *testing.T) {
	e := newVimrcEditor(t, "")
	vim.ExecuteCommand("inoremap a X")
	vim.ExecuteCommand("inoremap jk <Esc>")
	vim.ExecuteCommand("iabbrev teh the")
	t.Cleanup(func() { vim.ExecuteCommand("iunabbrev teh") })

	e.sendKeys("i")
	e.pasteText("banana jk teh cat\x1bdd\x12 end")
	if e.mode != INSERT {
		t.Fatalf("mode %v, want INSERT: something in the paste was a key", e.mode)
	}
	e.sendKeys("\x1b")
	assertBuffer(t, e, "banana jk teh catdd end")
}

// A key held for a mapping ("j" of "jk") goes in before the paste.
func TestPasteAfterAHeldKey(t *testing.T) {
	e := newMappingEditor(t, "", "inoremap jk <Esc>")

	e.sendKeys("ij")
	e.pasteText("kx")
	if e.mode != INSERT || vim.InputPending() != 0 {
		t.Fatalf("mode %v, %d bytes held", e.mode, vim.InputPending())
	}
	assertBuffer(t, e, "jkx")
}

// In NORMAL mode a paste is text put after the cursor -- before it in the
// first column -- and not commands. One u takes it back; . repeats it.
func TestPasteInNormalMode(t *testing.T) {
	e := newVimrcEditor(t, "alpha")

	e.pasteText("dd- one\r- two\r")
	assertBuffer(t, e, "dd- one", "- two", "alpha")
	if e.mode != NORMAL || vim.GetCurrentMode() != vim.ModeNormal {
		t.Errorf("mode %v, want NORMAL", e.mode)
	}
	assertPasteOptionOff(t)
	e.sendKeys("u")
	assertBuffer(t, e, "alpha")

	e.sendKeys("ll") // on the "p"
	e.pasteText("XY")
	assertBuffer(t, e, "alpXYha")
	e.sendKeys(".")
	assertBuffer(t, e, "alpXYXYha")
}

// Whatever was half-typed in NORMAL mode is dropped, not applied to the paste
// or fed by it.
func TestPasteDropsWhatWasPending(t *testing.T) {
	for _, pending := range []string{"d", "3", "r", "\"a", "f", "2d"} {
		e := newVimrcEditor(t, "alpha", "bravo")
		e.sendKeys(pending)
		e.pasteText("word ")
		assertBuffer(t, e, "word alpha", "bravo")
		if e.mode != NORMAL {
			t.Errorf("after %q and a paste: mode %v, want NORMAL", pending, e.mode)
		}
	}
}

// In VISUAL mode the paste replaces the selection.
func TestPasteOverASelection(t *testing.T) {
	e := newVimrcEditor(t, "alpha bravo charlie")
	e.sendKeys("wviw")
	if !e.pasteText("X\rY") {
		t.Error("redraw=false")
	}
	assertBuffer(t, e, "alpha X", "Y charlie")
	if e.mode != NORMAL {
		t.Errorf("mode %v, want NORMAL", e.mode)
	}

	e = newVimrcEditor(t, "  one", "  two", "three")
	e.sendKeys("Vj")
	e.pasteText("new")
	assertBuffer(t, e, "new", "three")
	assertPasteOptionOff(t)
}

// On a command line a paste is put on the line, as one line, and nothing is
// run -- not by a line end in it either.
func TestPasteOnACommandLine(t *testing.T) {
	e := newVimrcEditor(t, "alpha", "bravo")

	e.sendKeys(":")
	e.pasteText("%s/a/b/g\r")
	if e.mode != EX_COMMAND || e.command_line != "%s/a/b/g" {
		t.Fatalf("mode %v, line %q, want EX_COMMAND and %%s/a/b/g", e.mode, e.command_line)
	}
	assertBuffer(t, e, "alpha", "bravo")
	e.sendKeys("\r")
	assertBuffer(t, e, "blphb", "brbvo")

	e.sendKeys("/")
	e.pasteText("br\tb\rvo")
	if e.mode != SEARCH || e.command_line != "br b vo" || e.searchPrefix != "/" {
		t.Errorf("mode %v, line %q%q", e.mode, e.searchPrefix, e.command_line)
	}
	e.sendKeys("\x1b")
	assertBuffer(t, e, "blphb", "brbvo")
}

// With :paste already on, it stays on.
func TestPasteLeavesThePasteOptionAsItWas(t *testing.T) {
	e := newVimrcEditor(t, "")
	vim.ExecuteCommand("set paste")
	t.Cleanup(func() { vim.ExecuteCommand("set nopaste") })

	e.sendKeys("i")
	e.pasteText("  a\r  b")
	e.sendKeys("\x1b")
	assertBuffer(t, e, "  a", "  b")
	if vim.EvaluateExpression("&paste") != "1" {
		t.Error("the paste turned :paste off")
	}
}

// :help is up: there is nowhere for text to go.
func TestPasteWhileHelpIsShown(t *testing.T) {
	e := newVimrcEditor(t, "alpha")
	e.mode = HELP
	if e.pasteText("ddxyz") {
		t.Error("redraw=true")
	}
	assertBuffer(t, e, "alpha")
	if e.mode != HELP {
		t.Errorf("mode %v, want HELP", e.mode)
	}
}

// A long paste does not take long.
func TestPasteALot(t *testing.T) {
	e := newVimrcEditor(t, "")
	line := "    - the quick brown fox jumps over the lazy dog, café 日本"
	const n = 4000 // about 270 KB
	text := strings.Repeat(line+"\r", n)

	e.sendKeys("i")
	start := time.Now()
	e.pasteText(text)
	took := time.Since(start)
	e.sendKeys("\x1b")
	got := e.vbuf.Lines()
	if len(got) != n+1 || got[0] != line || got[n-1] != line || got[n] != "" {
		t.Fatalf("%d lines, first %q, last %q", len(got), got[0], got[len(got)-1])
	}
	t.Logf("%d KB in %v", len(text)/1024, took)
	if took > 5*time.Second {
		t.Errorf("pasting %d KB took %v", len(text)/1024, took)
	}
}

// The whole way: a paste from the key reader, through the event loop, into
// the note.
func TestEventLoopPastes(t *testing.T) {
	a, ids := editorOnlyApp(t, "- foo")
	a.keyEvents = make(chan terminal.Key, 64)
	vim.ExecuteCommand("mapclear | mapclear!")
	sourceVimrc()
	t.Cleanup(func() { vim.ExecuteCommand("mapclear | mapclear!") })

	captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		a.keyEvents <- terminal.Key{Regular: 'A'}
		a.keyEvents <- terminal.Key{Special: terminal.KeyPaste, Paste: "\r- bar\r  - baz"}
		for _, r := range "\x1b:x\r" {
			a.keyEvents <- terminal.Key{Regular: r}
		}
		a.handleEvents() // :x ends it: --editor has one note
	})

	if text, err := a.Store.Note(ids[0]); err != nil || text != "- foo\n- bar\n  - baz" {
		t.Errorf("the note is %q (%v)", text, err)
	}
}

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
