package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// Tab on a ':' line completes a file name, for a command that takes one
// (:read, :source). It writes nothing but the line.

// tabDir is a directory of files to complete among.
func tabDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"alpha.txt", "alps.md", "bravo.txt", ".hidden"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "album"), 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// typeLine opens a ':' line and types s on it.
func typeLine(t *testing.T, e *Editor, s string) {
	t.Helper()
	captureStdout(t, func() { e.sendKeys(":" + s) })
	if e.mode != EX_COMMAND || e.command_line != s {
		t.Fatalf("typed %q: mode %v, line %q", s, e.mode, e.command_line)
	}
}

// tab presses Tab and returns what was written, without escape sequences.
func tab(t *testing.T, e *Editor) string {
	t.Helper()
	return stripANSI(captureStdout(t, func() { e.ProcessKey('\t') }))
}

// lineIs fails unless the editor's command line and vim's are both want.
func lineIs(t *testing.T, e *Editor, want string) {
	t.Helper()
	if e.command_line != want || vim.CommandLineGetText() != want {
		t.Errorf("the line is %q (vim's %q), want %q", e.command_line, vim.CommandLineGetText(), want)
	}
	if e.cmdLineCursor != len(want) || vim.CommandLineGetPosition() != len(want) {
		t.Errorf("the cursor is at %d (vim's %d), want the end, %d", e.cmdLineCursor, vim.CommandLineGetPosition(), len(want))
	}
}

// Until 2026-11-15 Tab wrote "tab", and "dir: . base: ..." after a command
// with an argument: debugging output, on the line the command is typed on
// wherever the editor has the whole window.
func TestTabWritesOnlyTheLine(t *testing.T) {
	for _, line := range []string{"sy", "read zzznope", "help zzz", ""} {
		e := newTestEditor(t, "alpha")
		e.Layout = hostAppLayout
		typeLine(t, e, line)
		if out := tab(t, e); out != "" && out != ":"+line {
			t.Errorf("Tab after %q wrote %q", ":"+line, out)
		}
		lineIs(t, e, line)
		captureStdout(t, func() { e.ProcessKey(esc) })
	}
}

func TestTabCompletesAFileName(t *testing.T) {
	dir := tabDir(t)
	for _, cmd := range []string{"read", "r", "$read", "source", "so"} {
		e := newTestEditor(t, "alpha")
		typeLine(t, e, cmd+" "+dir+"/al")
		// in name order, a directory with its slash, and round again
		for _, want := range []string{"album/", "alpha.txt", "alps.md", "album/"} {
			if out := tab(t, e); !strings.HasPrefix(out, ":"+cmd+" ") {
				t.Errorf(":%s: Tab wrote %q", cmd, out)
			}
			lineIs(t, e, cmd+" "+dir+"/"+want)
		}
		// a key that is not Tab ends the round: the next Tab completes what
		// is then on the line
		captureStdout(t, func() { e.sendKeys("b") }) // .../album/b
		tab(t, e)
		lineIs(t, e, cmd+" "+dir+"/album/b") // nothing in there
		captureStdout(t, func() { e.ProcessKey(esc) })
	}
}

// With nothing typed of the name, every file in the directory but the hidden
// ones; with a dot typed, those.
func TestTabCompletesInADirectory(t *testing.T) {
	dir := tabDir(t)
	e := newTestEditor(t, "alpha")
	typeLine(t, e, "read "+dir+"/")
	for _, want := range []string{"album/", "alpha.txt", "alps.md", "bravo.txt", "album/"} {
		tab(t, e)
		lineIs(t, e, "read "+dir+"/"+want)
	}
	captureStdout(t, func() { e.ProcessKey(esc) })

	typeLine(t, e, "read "+dir+"/.")
	tab(t, e)
	lineIs(t, e, "read "+dir+"/.hidden")
	captureStdout(t, func() { e.ProcessKey(esc) })
}

// A name relative to the working directory, and to the home directory.
func TestTabCompletesRelativeNames(t *testing.T) {
	e := newTestEditor(t, "alpha")
	typeLine(t, e, "read vimango.v") // the tests run in the project root
	tab(t, e)
	lineIs(t, e, "read vimango.vim")
	captureStdout(t, func() { e.ProcessKey(esc) })

	t.Setenv("HOME", tabDir(t))
	typeLine(t, e, "read ~/br")
	tab(t, e)
	lineIs(t, e, "read ~/bravo.txt") // the ~ stays: vim expands it
	captureStdout(t, func() { e.ProcessKey(esc) })
}

// A file name is completed only where a file name goes. Tab after ":help s"
// used to put the first file in the working directory that starts with an s
// there.
func TestTabCompletesOnlyFileNames(t *testing.T) {
	for _, line := range []string{"help s", "set s", "s/a/b/ s", "echo s", "edit s", "w s", "r !s", "normal s"} {
		e := newTestEditor(t, "alpha")
		typeLine(t, e, line)
		tab(t, e)
		lineIs(t, e, line)
		captureStdout(t, func() { e.ProcessKey(esc) })
	}
}
