package editor

import (
	"slices"
	"strings"
	"testing"

	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// ":s/old/new/c" asks about each match, and the editor is who vim asks: it
// shows the match, writes the question and waits for its host to give it a
// key. The command is vim's own in every other way.

// The answers do what they do in vim, and while each is waited for the
// editor shows what vim would: the changes so far, the cursor on the match,
// and a mode of its own.
func TestConfirmAsksAboutEachMatch(t *testing.T) {
	e := newTestEditor(t, "foo bar foo", "foo", "bar foo")
	h := hostOf(e)
	h.answer("ynyq")
	var seen []string
	h.whileAsked = func(int) {
		if e.Mode() != CONFIRM {
			t.Errorf("mode %v while asked, want CONFIRM", e.Mode())
		}
		// the terminal's cursor waits after the question (tuiLayout's
		// message line starts at row 33, column 41), as in vim
		if row, col, show := e.Cursor(); row != 33 || col != 41+len("replace with X (y/n/a/q/l)?") || !show {
			t.Errorf("the cursor while asked is at %d,%d (shown %v), want after the question", row, col, show)
		}
		seen = append(seen, e.ss[e.fr]+"|"+e.ss[e.fr][e.fc:])
	}

	out := captureStdout(t, func() { e.exCommand("%s/foo/X/gc") })

	if got, want := e.vbuf.Lines(), []string{"X bar foo", "X", "bar foo"}; !slices.Equal(got, want) {
		t.Errorf("the note is %q, want %q", got, want)
	}
	// the cursor's row as the editor has it, and the row from the cursor on
	want := []string{"foo bar foo|foo bar foo", "X bar foo|foo", "foo|foo", "bar foo|foo"}
	if !slices.Equal(seen, want) {
		t.Errorf("shown while asked:\n%q\nwant:\n%q", seen, want)
	}
	if h.waits != 4 {
		t.Errorf("waited for a key %d times, want 4", h.waits)
	}
	if e.Mode() != NORMAL || e.asked != nil {
		t.Errorf("afterwards: mode %v, asked %v; want NORMAL and nothing asked", e.Mode(), e.asked)
	}
	if n := strings.Count(out, "replace with X (y/n/a/q/l)?"); n != 4 {
		t.Errorf("the question was written %d times, want 4", n)
	}
	// each match was marked: its text, in the style of the one asked about
	if n := strings.Count(out, askedStyle); n != 4 {
		t.Errorf("a match was marked %d times, want 4", n)
	}
	if !slices.Equal(e.ss, e.vbuf.Lines()) {
		t.Errorf("the editor's copy is %q, vim's %q", e.ss, e.vbuf.Lines())
	}
}

// Every answer "no" leaves the text as it was, so nothing says it changed,
// and the last match is still marked on screen: the key that ran the command
// asks for the text to be drawn. And the question does not stay on the
// message line.
func TestConfirmLeavesNoMarkBehind(t *testing.T) {
	e := newTestEditor(t, "foo", "foo")
	hostOf(e).answer("nn")
	e.sendKeys(":%s/foo/X/c")

	var redraw bool
	out := captureStdout(t, func() { redraw = e.ProcessKey('\r') })

	if !redraw {
		t.Error("no redraw after a command that marked a match and changed nothing")
	}
	if got := e.vbuf.Lines(); !slices.Equal(got, []string{"foo", "foo"}) {
		t.Errorf("the note is %q", got)
	}
	last := strings.LastIndex(out, "replace with")
	if last < 0 {
		t.Fatal("the question was not written")
	}
	// the message line (row 33, from column 41, 60 wide: tuiLayout) is
	// written again after the last question, which erases it
	if !strings.Contains(out[last:], "\x1b[33;41H\x1b[60X") {
		t.Errorf("the question was left on the message line: %q", out[last:])
	}
	if e.askedShown {
		t.Error("the mark is still recorded as shown")
	}
}

// By every route the command can come: a mapping, under :g, with a command
// after it, and "&&", which keeps the flag.
func TestConfirmByEveryRoute(t *testing.T) {
	t.Cleanup(func() { vim.ExecuteCommand("mapclear") })
	lines := func(e *Editor) string { return strings.Join(e.vbuf.Lines(), "|") }

	e := newTestEditor(t, "foo bar", "x", "foo")
	vim.ExecuteCommand("nnoremap Q :%s/foo/X/gc<CR>")
	hostOf(e).answer("yn")
	captureStdout(t, func() { e.sendKeys("Q") })
	if got := lines(e); got != "X bar|x|foo" {
		t.Errorf("from a mapping: %q", got)
	}
	if e.Mode() != NORMAL {
		t.Errorf("from a mapping: mode %v afterwards", e.Mode())
	}

	e = newTestEditor(t, "foo bar", "x", "foo")
	hostOf(e).answer("ny")
	captureStdout(t, func() { e.exCommand("g/foo/s//X/c") })
	if got := lines(e); got != "foo bar|x|X" {
		t.Errorf("under :g: %q", got)
	}

	e = newTestEditor(t, "foo bar", "x", "foo")
	hostOf(e).answer("yy")
	captureStdout(t, func() { e.exCommand("%s/foo/X/gc | let g:confirm_after = 'ran'") })
	if got := lines(e); got != "X bar|x|X" {
		t.Errorf("with a command after it: %q", got)
	}
	if got := vim.EvaluateExpression("g:confirm_after"); got != "ran" {
		t.Errorf("the command after it: g:confirm_after is %q", got)
	}

	// the cursor is on the last line changed; "&&" there asks again
	e = newTestEditor(t, "foo foo", "x")
	hostOf(e).answer("yq" + "y")
	captureStdout(t, func() { e.exCommand("s/foo/X/gc"); e.exCommand("&&") })
	if got := lines(e); got != "X X|x" {
		t.Errorf("&&: %q", got)
	}
}

// One u undoes everything that was accepted, and the note is then not
// modified.
func TestConfirmIsOneUndo(t *testing.T) {
	e := newTestEditor(t, "foo bar foo", "foo")
	hostOf(e).answer("yny")
	captureStdout(t, func() { e.exCommand("%s/foo/X/gc") })
	if got := e.vbuf.Lines(); !slices.Equal(got, []string{"X bar foo", "X"}) {
		t.Fatalf("the note is %q", got)
	}
	if !e.Modified() {
		t.Error("not modified after changes were accepted")
	}
	captureStdout(t, func() { e.sendKeys("u") })
	if got := e.vbuf.Lines(); !slices.Equal(got, []string{"foo bar foo", "foo"}) {
		t.Errorf("after u the note is %q", got)
	}
	if e.Modified() {
		t.Error("modified after the changes were undone")
	}
}

// Keys that are not characters -- an arrow -- are not answers and vim does
// not hear of them; a key vim does not take is asked about again; and when
// no key can come, or the host cannot wait for one, the question is answered
// as <esc> answers it: nothing more is changed, and nothing hangs.
func TestConfirmKeysThatAreNotAnswers(t *testing.T) {
	e := newTestEditor(t, "foo", "foo")
	h := hostOf(e)
	h.answers = []int{terminal.KeyArrowUp, 'x', 'y'}
	out := captureStdout(t, func() { e.exCommand("%s/foo/X/c") })
	questions := strings.Count(out, "replace with")
	if got := e.vbuf.Lines(); !slices.Equal(got, []string{"X", "foo"}) {
		t.Errorf("the note is %q, want the first changed and no more: the keys ran out", got)
	}
	// asked about the first match, asked again after "x", and about the
	// second, which no key answered
	if questions != 3 {
		t.Errorf("the question was written %d times, want 3", questions)
	}

	e = newTestEditor(t, "foo", "foo")
	e.WaitKey = nil
	captureStdout(t, func() { e.exCommand("%s/foo/X/c") })
	if got := e.vbuf.Lines(); !slices.Equal(got, []string{"foo", "foo"}) {
		t.Errorf("with a host that cannot wait, the note is %q", got)
	}
	if e.Mode() != NORMAL {
		t.Errorf("with a host that cannot wait, mode %v afterwards", e.Mode())
	}
}

// A resize while the question stands clears the screen, and Draw puts all of
// it back: the text with the match marked, and the question.
func TestDrawPutsTheQuestionBack(t *testing.T) {
	e := newTestEditor(t, "foo", "foo")
	h := hostOf(e)
	h.answer("q")
	h.whileAsked = func(int) { e.Draw() }
	out := captureStdout(t, func() { e.exCommand("%s/foo/X/c") })
	if h.waits != 1 {
		t.Fatalf("the host was asked for a key %d times, want once", h.waits)
	}
	if n := strings.Count(out, "replace with X (y/n/a/q/l)?"); n != 2 {
		t.Errorf("the question was written %d times, want 2: when asked, and by Draw", n)
	}
	if n := strings.Count(out, askedStyle); n != 2 {
		t.Errorf("the match was marked %d times, want 2", n)
	}
}

// The replacement is shown as it was typed, with a control character in it
// drawn as vim draws one: nothing typed is written to the terminal raw.
func TestQuestionShowsTheReplacementSafely(t *testing.T) {
	e := newTestEditor(t, "foo")
	e.asked = &vim.SubstituteMatch{Line: 1, Col: 0, EndLine: 1, EndCol: 3, Replacement: "a\x0bb&"}
	out := captureStdout(t, e.showQuestion)
	if !strings.Contains(out, "replace with a^Kb& (y/n/a/q/l)?") {
		t.Errorf("the question is %q", out)
	}
	e.asked = nil
}
