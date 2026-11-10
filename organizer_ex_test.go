package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// newDBOrganizer is an organizer over a throwaway database holding one note
// per title, listed in that order, behind the global app as the ex commands
// expect it. ids[i] is the id of titles[i]; each note's text is "text of
// <title>".
func newDBOrganizer(t *testing.T, titles ...string) (o *Organizer, ids []int) {
	t.Helper()
	vim.InitializeVim(0)
	vim.SendKey("<esc>")
	db := newTestDatabase(t)
	sess := &Session{}
	scr := &Screen{screenCols: 120, screenLines: 40, textLines: 30, divider: 40, totaleditorcols: 78, Session: sess}
	o = &Organizer{Session: sess, Screen: scr, Database: db, view: TASK, taskview: BY_FOLDER, filter: "none", sort: "modified",
		mode: NORMAL, vbuf: vim.NewBuffer(0), marked_entries: map[int]struct{}{}}
	for _, title := range titles {
		row := &Row{title: title}
		captureStdout(t, func() {
			if err := db.insertTitle(row, DefaultContextUUID, DefaultFolderUUID); err != nil {
				t.Fatal(err)
			}
		})
		if err := db.updateNote(row.id, "text of "+title); err != nil {
			t.Fatal(err)
		}
		o.rows = append(o.rows, Row{id: row.id, title: title})
		ids = append(ids, row.id)
	}
	vim.SetCurrentBuffer(o.vbuf)
	o.readRowsIntoBuffer()
	vim.SetCursorPosition(1, 0)

	a := &App{Session: sess, Screen: scr, Database: db, Organizer: o, Run: true}
	o.exCmds = a.setOrganizerExCmds(o)
	o.normalCmds = a.setOrganizerNormalCmds(o)
	prev := app
	app = a
	t.Cleanup(func() { app = prev })
	return o, ids
}

// ex types ":line<cr>" in the organizer and returns what it wrote to the
// screen.
func (o *Organizer) ex(t *testing.T, line string) string {
	t.Helper()
	return captureStdout(t, func() {
		for _, r := range ":" + line + "\r" {
			o.organizerProcessKey(int(r))
		}
	})
}

func TestParseEx(t *testing.T) {
	o, _ := newDBOrganizer(t, "one")
	for _, tc := range []struct {
		line  string
		want  exArgs
		found bool
	}{
		{"e", exArgs{name: "e"}, true},
		{"e 123", exArgs{name: "e", arg: "123"}, true},
		{"q!", exArgs{name: "q", bang: true}, true},
		{"rii!", exArgs{name: "rii", bang: true}, true},
		{"sync?", exArgs{name: "sync?"}, true},
		{"o work", exArgs{name: "o", arg: "work"}, true},
		{"open my folder", exArgs{name: "open", arg: "my folder"}, true},
		{"find two  words here", exArgs{name: "find", arg: "two  words here"}, true},
		{"set context work", exArgs{name: "set context", arg: "work"}, true},
		{"set context", exArgs{name: "set context"}, true},
		{"set c my context", exArgs{name: "set c", arg: "my context"}, true},
		{"add k urgent", exArgs{name: "add k", arg: "urgent"}, true},
		{"help set context", exArgs{name: "help", arg: "set context"}, true},
		{"  imagescale   30 ", exArgs{name: "imagescale", arg: "30"}, true},
		{"nosuch thing", exArgs{name: "nosuch thing"}, false},
		{"", exArgs{}, false},
	} {
		got, cmd, found := o.parseEx(tc.line)
		if got != tc.want || found != tc.found || (cmd != nil) != tc.found {
			t.Errorf("parseEx(%q) = %+v, found %v; want %+v, found %v", tc.line, got, found, tc.want, tc.found)
		}
	}
}

// ":e 123" opens note 123. The ex commands used to be handed the position of
// the space in the line, and editNote took that for an id: ":e 123" opened
// note 1.
func TestEditByIdOpensThatNote(t *testing.T) {
	o, ids := newDBOrganizer(t, "one", "two", "three")
	if ids[2] == 1 {
		t.Fatal("the premise of this test is wrong: the third note has id 1")
	}

	o.ex(t, "e "+strconv.Itoa(ids[2]))
	e := o.Session.editor
	if e == nil {
		t.Fatal(":e <id> opened no editor")
	}
	if e.id != ids[2] || e.title != "three" || strings.Join(e.vbuf.Lines(), "\n") != "text of three" {
		t.Errorf(":e %d opened note %d (%q, %q)", ids[2], e.id, e.title, strings.Join(e.vbuf.Lines(), "\n"))
	}
}

// ":e" alone opens the note of the current row.
func TestEditOpensTheCurrentNote(t *testing.T) {
	o, ids := newDBOrganizer(t, "one", "two", "three")
	o.fr = 1
	vim.SetCursorPosition(2, 0)

	o.ex(t, "e")
	if e := o.Session.editor; e == nil || e.id != ids[1] {
		t.Errorf(":e with the second row current opened %+v, want note %d", e, ids[1])
	}
}

// An id that is not a note's opens nothing and says so.
func TestEditRefusesAnIdThatIsNotANote(t *testing.T) {
	o, _ := newDBOrganizer(t, "one")
	for _, arg := range []string{"99999", "abc", "-1", "1 2"} {
		out := o.ex(t, "e "+arg)
		if o.Session.editor != nil || o.Session.editorMode {
			t.Fatalf(":e %s opened an editor", arg)
		}
		if !strings.Contains(out, "There is no note with id") {
			t.Errorf(":e %s said %q", arg, out)
		}
		if o.mode != NORMAL {
			t.Errorf(":e %s left the organizer in mode %v", arg, o.mode)
		}
	}
}

// A command that is finished leaves the command line. ":sort" without a column
// only showed its message and stayed in COMMAND_LINE mode, where the next keys
// were added to the line.
func TestExCommandLeavesTheCommandLine(t *testing.T) {
	o, _ := newDBOrganizer(t, "one")
	for _, line := range []string{"sort", "sort nosuchcolumn", "nosuchcommand"} {
		o.ex(t, line)
		if o.mode != NORMAL {
			t.Errorf(":%s left the organizer in mode %v, want NORMAL", line, o.mode)
		}
	}
	// and one that opens something else keeps its own mode
	o.ex(t, "help set context")
	if o.mode != HELP {
		t.Errorf(":help set context left mode %v, want HELP", o.mode)
	}
}

// ":set context" takes a name with a space in it, as ":open" does. The line
// was split at its last space, so "set context my" was looked up as a command.
func TestSetContextWithASpaceInTheName(t *testing.T) {
	o, ids := newDBOrganizer(t, "one")
	const uuid = "11111111-1111-1111-1111-111111111111"
	if _, err := o.Database.MainDB.Exec("INSERT INTO context (title, tid, uuid) VALUES ('my context', 2, ?);", uuid); err != nil {
		t.Fatal(err)
	}

	o.ex(t, "set context my context")
	var got string
	if err := o.Database.MainDB.QueryRow("SELECT context_uuid FROM task WHERE id=?;", ids[0]).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != uuid {
		t.Errorf("after :set context my context the note's context is %s, want %s", got, uuid)
	}
}
