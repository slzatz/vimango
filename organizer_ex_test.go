package main

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/slzatz/vimango/internal/localdb"
	"github.com/slzatz/vimango/internal/store"
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
	db := newTestStore(t)
	sess := &Session{}
	scr := &Screen{screenCols: 120, screenLines: 40, textLines: 30, divider: 40, totaleditorcols: 78, Session: sess}
	o = &Organizer{Session: sess, Screen: scr, Store: db, view: TASK, taskview: BY_FOLDER, filter: "none", sort: "modified",
		mode: NORMAL, vbuf: vim.NewBuffer(0), marked_entries: map[int]struct{}{}}
	for _, title := range titles {
		id, err := db.Add(title, false, localdb.DefaultContextUUID, localdb.DefaultFolderUUID)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.SetNote(id, "text of "+title); err != nil {
			t.Fatal(err)
		}
		o.rows = append(o.rows, Row{id: id, title: title})
		ids = append(ids, id)
	}
	vim.SetCurrentBuffer(o.vbuf)
	o.readRowsIntoBuffer()
	vim.SetCursorPosition(1, 0)

	a := &App{Session: sess, Screen: scr, Store: db, Organizer: o, Run: true}
	a.RenderManager = NewRenderManager(a) // the preview of the current row
	t.Cleanup(a.RenderManager.Stop)
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
	if _, err := o.Store.DB().Exec("INSERT INTO context (title, tid, uuid) VALUES ('my context', 2, ?);", uuid); err != nil {
		t.Fatal(err)
	}

	o.ex(t, "set context my context")
	var got string
	if err := o.Store.DB().QueryRow("SELECT context_uuid FROM task WHERE id=?;", ids[0]).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != uuid {
		t.Errorf("after :set context my context the note's context is %s, want %s", got, uuid)
	}
}

// titles is the list as vim holds it and as the organizer holds it; the two
// must agree.
func (o *Organizer) listed(t *testing.T) []string {
	t.Helper()
	var titles []string
	for _, r := range o.rows {
		titles = append(titles, r.title)
	}
	if got := o.vbuf.Lines(); strings.Join(got, "\n") != strings.Join(titles, "\n") {
		t.Errorf("vim's buffer holds %q, the organizer's rows %q", got, titles)
	}
	return titles
}

// Every command that changes what is listed leaves the list in one state: the
// rows, vim's buffer of their titles, and the cursor on the first of them.
func TestCommandsThatChangeTheList(t *testing.T) {
	o, _ := newDBOrganizer(t, "alpha", "bravo", "charlie")
	atTop := func(after string) {
		t.Helper()
		if o.fr != 0 || o.fc != 0 || o.rowoff != 0 || vim.GetCursorPosition() != [2]int{1, 0} {
			t.Errorf("after :%s the cursor is at row %d col %d (vim %v), want the first row", after, o.fr, o.fc, vim.GetCursorPosition())
		}
		if o.mode != NORMAL {
			t.Errorf("after :%s the mode is %v", after, o.mode)
		}
	}
	sorted := func(titles []string) string {
		s := slices.Clone(titles)
		slices.Sort(s)
		return strings.Join(s, " ")
	}
	move := func() { // off the first row, so that a command has something to reset
		o.fr = len(o.rows) - 1
		vim.SetCursorPosition(o.fr+1, 0)
	}

	for _, cmd := range []string{"recent", "open none", "openfolder none", "opencontext none", "refresh", "showall", "showall", "sort added"} {
		move()
		o.ex(t, cmd)
		if got := sorted(o.listed(t)); got != "alpha bravo charlie" {
			t.Errorf(":%s lists %q, want the three notes", cmd, got)
		}
		if o.view != TASK {
			t.Errorf(":%s left view %v", cmd, o.view)
		}
		atTop(cmd)
	}

	move()
	o.ex(t, "find bravo")
	if got := o.listed(t); len(got) != 1 || got[0] != "bravo" {
		t.Errorf(":find bravo lists %q", got)
	}
	atTop("find bravo")
	o.ex(t, "refresh") // a refresh of a search runs the search again
	if got := o.listed(t); len(got) != 1 || got[0] != "bravo" || o.taskview != BY_FIND {
		t.Errorf(":refresh after :find lists %q (taskview %d)", got, o.taskview)
	}

	// containers, and back
	for cmd, want := range map[string]View{"contexts": CONTEXT, "folders": FOLDER} {
		o.ex(t, cmd)
		if got := o.listed(t); len(got) != 1 || got[0] != "none" || o.view != want {
			t.Errorf(":%s lists %q in view %v", cmd, got, o.view)
		}
		atTop(cmd)
		o.ex(t, "refresh")
		if got := o.listed(t); len(got) != 1 || got[0] != "none" || o.view != want {
			t.Errorf(":refresh in :%s lists %q in view %v", cmd, got, o.view)
		}
	}
	o.ex(t, "recent")
	if got := sorted(o.listed(t)); got != "alpha bravo charlie" {
		t.Errorf(":recent after a container view lists %q", got)
	}
}

// A list with nothing in it still has a current row: one blank, unsaved row
// that is not counted as an edit.
func TestAnEmptyListHasOneBlankRow(t *testing.T) {
	o, _ := newDBOrganizer(t, "alpha")
	for _, cmd := range []string{"keywords", "find zzzznothinghasthis"} {
		out := o.ex(t, cmd)
		if len(o.rows) != 1 || o.rows[0].id != -1 || o.rows[0].title != "" || o.rows[0].dirty {
			t.Errorf(":%s with nothing to list left rows %+v", cmd, o.rows)
		}
		o.listed(t)
		if cmd == "keywords" && !strings.Contains(out, "No results were returned") {
			t.Errorf(":%s did not say the list is empty: %q", cmd, out)
		}
		// the key that needs a current row
		captureStdout(t, func() { o.organizerProcessKey('j') })
	}
}

func TestOrganizerHelp(t *testing.T) {
	o, _ := newDBOrganizer(t, "alpha")
	a := &App{}
	a.setOrganizerNormalCmds(o) // fills o.normalCommandRegistry
	for _, tc := range []struct{ arg, want, not string }{
		{"", "open", "not found"},                          // every ex command
		{"normal", "Normal Mode Commands", "not found"},    // the normal-mode keys
		{"find", "find <search_terms>", "not found"},       // an ex command
		{"o", "open <name>", "not found"},                  // by an alias
		{"Ctrl-D", "Ctrl-D", "not found"},                  // a normal-mode key, by name
		{"Images", "imagescale", "not found"},              // a category of ex commands
		{"Entry Actions", "Ctrl-A", "not found"},           // a category of normal commands
		{"opn", "Did you mean: ", "Use ':help'"},           // a typo
		{"zzzzqqqq", "Use ':help' for ex commands", "Did"}, // nothing like it
	} {
		got := o.helpFor(tc.arg)
		if !strings.Contains(got, tc.want) || strings.Contains(got, tc.not) {
			t.Errorf(":help %s = %q\n  want it to contain %q and not %q", tc.arg, got, tc.want, tc.not)
		}
	}
	if got := o.helpFor("zzzzqqqq"); !strings.HasPrefix(got, "Command or category 'zzzzqqqq' not found.\n") {
		t.Errorf("help for an unknown name is %q", got)
	}

	// :help shows it and stays up; the command line is left behind
	o.ex(t, "help find")
	if o.mode != HELP || o.command_line != "" {
		t.Errorf("after :help find the mode is %v and the command line %q", o.mode, o.command_line)
	}
}

// :w gives a new row the id of the note it became. It worked on a copy of
// the row, so the list kept id -1: the note could not be opened (":e" asked
// for the title to be saved first), and the next change to its title was
// written as a second note.
func TestWriteGivesANewRowItsNote(t *testing.T) {
	o, _ := newDBOrganizer(t, "alpha")
	count := func() (n int) {
		t.Helper()
		if err := o.Store.DB().QueryRow("SELECT count(*) FROM task;").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	o.rows = append([]Row{{id: -1, title: "brand new", dirty: true}}, o.rows...)
	o.readRowsIntoBuffer()

	o.ex(t, "w")
	id := o.rows[0].id
	if id == -1 || o.rows[0].dirty || count() != 2 {
		t.Fatalf("after :w the new row has id %d (dirty %v) and there are %d notes", id, o.rows[0].dirty, count())
	}
	if title, err := o.Store.Title(id); err != nil || title != "brand new" {
		t.Errorf("note %d is titled %q (%v)", id, title, err)
	}
	if context, folder := o.contextOf(id), o.folderOf(id); context != "none" || folder != "none" {
		t.Errorf("the new note is in %q / %q; the list is of folder none", context, folder)
	}

	// changing the title again renames that note
	o.rows[0].title, o.rows[0].dirty = "renamed", true
	o.ex(t, "w")
	if title, _ := o.Store.Title(id); title != "renamed" || count() != 2 || o.rows[0].id != id {
		t.Errorf("after a second :w: note %d is %q, %d notes, the row's id %d", id, title, count(), o.rows[0].id)
	}
}

// A new note goes in the context or folder the list is of.
func TestNewNoteGoesWhereTheListIs(t *testing.T) {
	o, _ := newDBOrganizer(t, "alpha")
	if _, err := o.Store.AddContainer(store.Context, "work", false); err != nil {
		t.Fatal(err)
	}
	o.ex(t, "opencontext work")
	o.rows = []Row{{id: -1, title: "at work", dirty: true}}
	o.readRowsIntoBuffer()
	o.ex(t, "w")
	if id := o.rows[0].id; id == -1 || o.contextOf(id) != "work" || o.folderOf(id) != "none" {
		t.Errorf("the new note (id %d) is in %q / %q, want work / none", id, o.contextOf(id), o.folderOf(id))
	}
}

// :newc and :newf (and Ctrl-C, Ctrl-F) list the contexts or folders by
// number, in order, and wait for one.
func TestPickingAContainer(t *testing.T) {
	o, ids := newDBOrganizer(t, "alpha")
	for _, title := range []string{"zebra", "apple"} {
		if _, err := o.Store.AddContainer(store.Context, title, false); err != nil {
			t.Fatal(err)
		}
	}
	for name, pick := range map[string]func(){
		":newc":  func() { o.ex(t, "newc") },
		"Ctrl-C": func() { captureStdout(t, func() { o.organizerProcessKey(ctrlKey('c')) }) },
	} {
		o.mode = NORMAL
		pick()
		if o.mode != CONTAINER || o.altView != CONTEXT || strings.Join(o.containerList, " ") != "apple none zebra" {
			t.Errorf("%s: mode %v, picking a %v from %v", name, o.mode, o.altView, o.containerList)
		}
		captureStdout(t, func() { o.organizerProcessKey('\x1b') })
	}

	// and picking one moves the current entry there
	o.ex(t, "newc")
	captureStdout(t, func() {
		o.organizerProcessKey('3') // zebra
		o.organizerProcessKey('\r')
	})
	if got := o.contextOf(ids[0]); got != "zebra" {
		t.Errorf("after picking 3 the note's context is %q, want zebra", got)
	}
}

// A notice opens at its top. It used to be drawn first from wherever the
// preview had been scrolled to, and only then reset: a short :help after
// paging down a long note asked for lines 15 to 5 of its text, and that
// took the app down.
func TestNoticeOpensAtItsTopAfterThePreviewWasScrolled(t *testing.T) {
	o, _ := newDBOrganizer(t, "alpha")
	for i := range 80 {
		o.note = append(o.note, "preview line "+strconv.Itoa(i))
	}
	captureStdout(t, func() {
		o.organizerProcessKey(' ') // page the preview down
		o.organizerProcessKey(' ')
	})
	if o.altRowoff == 0 {
		t.Fatal("the premise of this test is wrong: Space did not scroll the preview")
	}

	o.ex(t, "help find") // five lines
	if o.mode != HELP || o.Screen.noticeTop != 0 || len(o.Screen.notice) == 0 {
		t.Errorf(":help find: mode %v, %d lines, from line %d", o.mode, len(o.Screen.notice), o.Screen.noticeTop)
	}
	captureStdout(t, func() { o.organizerProcessKey('\x1b') })

	captureStdout(t, func() { o.organizerProcessKey(ctrlKey('i')) }) // entry info, another notice
	if o.Screen.noticeTop != 0 || len(o.Screen.notice) == 0 {
		t.Errorf("entry info: %d lines, from line %d", len(o.Screen.notice), o.Screen.noticeTop)
	}
}
