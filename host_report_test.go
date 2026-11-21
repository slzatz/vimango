package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/slzatz/vimango/terminal"
)

// What a host is told (host_report.go): the terminal's title, set with OSC 2.
var titleSet = regexp.MustCompile("\x1b\\]2;([^\x07]*)\x07")

func titlesSet(written string) (titles []string) {
	for _, m := range titleSet.FindAllStringSubmatch(written, -1) {
		titles = append(titles, m[1])
	}
	return titles
}

// The report is the note, this process (whose recovery file a host looks
// for when it ends), whether the buffer has unsaved changes, and
// whether :checkstale found the row changed under them. It is written when
// one of those changes and not otherwise: most keys change none.
func TestEditorReportsToItsHost(t *testing.T) {
	a, ids := editorOnlyApp(t, "alpha")
	e := openIn(t, a, ids[0])
	id := ids[0]
	reported := func() string {
		t.Helper()
		return strings.Join(titlesSet(captureStdout(t, a.reportToHost)), " | ")
	}
	want := func(when, pairs string) {
		t.Helper()
		line := ""
		if pairs != "" {
			line = "vimango id=" + strconv.Itoa(id) + " pid=" + strconv.Itoa(os.Getpid()) + " " + pairs
		}
		if got := reported(); got != line {
			t.Fatalf("%s: the host was told %q, want %q", when, got, line)
		}
	}

	want("the note opened", "modified=0 stale=0")
	want("nothing happened", "")
	captureStdout(t, func() { pressKeys(e, "jk$") })
	want("keys that change nothing", "")
	captureStdout(t, func() { pressKeys(e, "x") })
	want("an edit", "modified=1 stale=0")
	captureStdout(t, func() { pressKeys(e, "x") })
	want("another edit", "")
	captureStdout(t, func() { runEx(e, "w") })
	want("saved", "modified=0 stale=0")

	// modified is the text against the database's, not that keys were typed
	captureStdout(t, func() { pressKeys(e, "x") })
	want("an edit", "modified=1 stale=0")
	captureStdout(t, func() { pressKeys(e, "u") })
	want("the edit undone", "modified=0 stale=0")
	captureStdout(t, func() { pressKeys(e, "u") })
	want("undone past the save", "modified=1 stale=0")

	if err := a.Store.SetNote(id, "from a sync"); err != nil {
		t.Fatal(err)
	}
	captureStdout(t, func() { runEx(e, "checkstale") })
	want("the row changed under it", "modified=1 stale=1")
	captureStdout(t, func() { runEx(e, "w") })
	want("saved over it", "modified=0 stale=0")
}

// The TUI has no host, and the title of the terminal it runs in is not its
// to set.
func TestTUISetsNoTitle(t *testing.T) {
	a, ids := tuiApp(t, "alpha")
	e := openIn(t, a, ids[0])
	captureStdout(t, func() { pressKeys(e, "x") })
	if got := captureStdout(t, a.reportToHost); got != "" {
		t.Errorf("the TUI wrote %q", got)
	}
}

// The event loop is what reports: at the start, and after the event that
// changed something.
func TestEventLoopReportsToTheHost(t *testing.T) {
	a, ids := editorOnlyApp(t, "alpha")
	a.keyEvents = make(chan terminal.Key, 16)
	written := captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		for _, r := range "xj:w\r:q\r" {
			a.keyEvents <- terminal.Key{Regular: r}
		}
		a.handleEvents() // :q ends it: --editor has one note
	})
	id := "vimango id=" + strconv.Itoa(ids[0]) + " pid=" + strconv.Itoa(os.Getpid())
	want := []string{id + " modified=0 stale=0", id + " modified=1 stale=0", id + " modified=0 stale=0"}
	if got := titlesSet(written); strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Errorf("the host was told %q, want %q", got, want)
	}
}

// A question vim asks in the middle of a command (":s" with the c flag) can
// stand for a long time, and the changes accepted before it are unsaved
// text: the host is told so while the loop waits for the answer, not only
// when the command is over.
func TestHostIsToldOfChangesWhileSubstituteAsks(t *testing.T) {
	a, ids := editorOnlyApp(t, "foo\nfoo\nfoo")
	a.keyEvents = make(chan terminal.Key, 64)

	var reports []string
	captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		// what the host had been told when each answer was taken
		a.Session.editor.WaitKey = func() int {
			k := a.waitKey()
			reports = append(reports, a.hostReported)
			return k
		}
		for _, r := range ":%s/foo/X/c\ryq:q!\r" {
			a.keyEvents <- terminal.Key{Regular: r}
		}
		a.handleEvents() // :q! ends it: --editor has one note
	})

	if len(reports) != 2 {
		t.Fatalf("%d answers were taken, want 2: %q", len(reports), reports)
	}
	if !strings.Contains(reports[0], "modified=0") {
		t.Errorf("at the first question the host had been told %q, want modified=0", reports[0])
	}
	if !strings.Contains(reports[1], "modified=1") {
		t.Errorf("at the second question, the first change accepted, the host had been told %q, want modified=1", reports[1])
	}
}
