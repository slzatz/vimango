package main

import (
	"fmt"
	"os"

	"github.com/slzatz/vimango/internal/editor"
)

// What an --editor process tells the app that hosts it.
//
// A host cannot see into the editor: whether the buffer has unsaved changes,
// or whether :checkstale found the note changed underneath them, is vim's to
// know. Until 2026-11-16 the host had one way to ask, which was to inject
// ":q" and watch whether the process ended. That is still the only answer a
// host may act on where text could be lost -- an exit after :q or :x means
// the text is in the database, and nothing said here means that. This is for
// what a host shows: a mark on a window with unsaved changes, a warning that
// a hidden editor is holding a note the sync has changed.
//
// The report is the terminal's title (OSC 2), which a terminal hands to
// whatever embeds it: the word vimango and then key=value pairs,
//
//	vimango id=5 pid=1234 modified=1 stale=0
//
// id is the note, pid this process, modified is Editor.Modified and stale is
// Editor.Stale. The pid names the editor's recovery file, recovery/<id>.<pid>.json,
// which the editor removes when it closes the note with :q or :x and at no
// other time: a host that finds it still there after the process has ended
// knows the editor stopped some other way, and that what it had unsaved is
// in that file (crash.go, and recovery.go in internal/editor). A
// host takes the pairs it knows and passes over the rest, so a pair can be
// added. It is written when the editor opens and whenever a value changes,
// by the event loop and nowhere else, in --editor mode only: the TUI has no
// host, and the title of a terminal someone is sitting at is theirs.

// hostReport is the report for an editor as it is now.
func hostReport(e *editor.Editor) string {
	flag := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	return fmt.Sprintf("vimango id=%d pid=%d modified=%d stale=%d", e.ID(), os.Getpid(), flag(e.Modified()), flag(e.Stale()))
}

// reportToHost writes the report if it is not the one last written.
func (a *App) reportToHost() {
	if !a.Session.editorOnly || a.Session.editor == nil {
		return
	}
	report := hostReport(a.Session.editor)
	if report == a.hostReported {
		return
	}
	a.hostReported = report
	fmt.Printf("\x1b]2;%s\x07", report)
}
