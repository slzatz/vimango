package store

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/slzatz/vimango/internal/localdb"
)

// newStore is a store over a new database, as `vimango --init` makes one.
func newStore(t *testing.T) *Store {
	t.Helper()
	db, err := localdb.Open(filepath.Join(t.TempDir(), "vimango.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := localdb.Create(db); err != nil {
		t.Fatal(err)
	}
	return New(db)
}

// add makes a note with a title and text in the default containers.
func add(t *testing.T, s *Store, title, text string) int {
	t.Helper()
	id, err := s.Add(title, false, localdb.DefaultContextUUID, localdb.DefaultFolderUUID)
	if err != nil {
		t.Fatal(err)
	}
	if text != "" {
		if err := s.SetNote(id, text); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

// synced gives a row of a kind the server id it would get from a sync.
func synced(t *testing.T, s *Store, kind Kind, id, tid int) {
	t.Helper()
	if _, err := s.DB().Exec("UPDATE "+string(kind)+" SET tid=? WHERE id=?;", tid, id); err != nil {
		t.Fatal(err)
	}
}

func titles(entries []Entry) string {
	var tt []string
	for _, e := range entries {
		tt = append(tt, e.Title)
	}
	return strings.Join(tt, " ")
}

var byModified = Listing{Sort: "modified", Limit: 100}

func TestANewDatabaseIsValid(t *testing.T) {
	s := newStore(t)
	if err := s.Validate(); err != nil {
		t.Errorf("a database made by Create: %v", err)
	}
	if _, err := s.DB().Exec("DROP TABLE task_keyword;"); err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "task_keyword") {
		t.Errorf("with task_keyword gone, Validate says %v", err)
	}
}

func TestNoteTextAndTitle(t *testing.T) {
	s := newStore(t)
	id := add(t, s, "a title", "")

	if text, err := s.Note(id); err != nil || text != "" {
		t.Errorf("a new note's text is %q, %v; want empty", text, err)
	}
	if err := s.SetNote(id, "some text"); err != nil {
		t.Fatal(err)
	}
	if text, err := s.Note(id); err != nil || text != "some text" {
		t.Errorf("Note = %q, %v", text, err)
	}
	if title, text, err := s.TitleAndNote(id); err != nil || title != "a title" || text != "some text" {
		t.Errorf("TitleAndNote = %q, %q, %v", title, text, err)
	}

	// an empty note is stored as NULL, and read back as empty
	if err := s.SetNote(id, ""); err != nil {
		t.Fatal(err)
	}
	var isNull bool
	if err := s.DB().QueryRow("SELECT note IS NULL FROM task WHERE id=?;", id).Scan(&isNull); err != nil || !isNull {
		t.Errorf("an empty note is stored as NULL: %v, %v", isNull, err)
	}
	if title, text, err := s.TitleAndNote(id); err != nil || title != "a title" || text != "" {
		t.Errorf("TitleAndNote of a note with no text = %q, %q, %v", title, text, err)
	}

	if err := s.SetTitle(id, "renamed"); err != nil {
		t.Fatal(err)
	}
	if title, err := s.Title(id); err != nil || title != "renamed" {
		t.Errorf("Title = %q, %v", title, err)
	}
	if ok, err := s.Exists(id); err != nil || !ok {
		t.Errorf("Exists(%d) = %v, %v", id, ok, err)
	}
}

// Asking about a note that is not there is ErrNotFound, by every route -- and
// writing its text is too: SQLite reports an UPDATE that matched no row as a
// success, and the editor must not take it for a save.
func TestANoteThatIsNotThere(t *testing.T) {
	s := newStore(t)
	const gone = 4242
	if ok, err := s.Exists(gone); err != nil || ok {
		t.Errorf("Exists = %v, %v", ok, err)
	}
	for what, err := range map[string]error{
		"Note":         second(s.Note(gone)),
		"Title":        second(s.Title(gone)),
		"TitleAndNote": third(s.TitleAndNote(gone)),
		"Info":         second(s.Info(gone)),
		"ContextOf":    second(s.ContextOf(gone)),
		"FolderOf":     second(s.FolderOf(gone)),
		"Copy":         second(s.Copy(gone)),
		"SetNote":      s.SetNote(gone, "text"),
		"TitleForTID":  second(s.TitleForTID(gone)),
	} {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s of a note that is not there: %v, want ErrNotFound", what, err)
		}
	}
	// a note with no keywords, or none at all, has no keywords
	if kk, err := s.KeywordsOf(gone); err != nil || len(kk) != 0 {
		t.Errorf("KeywordsOf = %v, %v", kk, err)
	}
}

func second[T any](_ T, err error) error        { return err }
func third[A, B any](_ A, _ B, err error) error { return err }

func TestInfoAndFlags(t *testing.T) {
	s := newStore(t)
	id := add(t, s, "flagged", "text")

	info, err := s.Info(id)
	if err != nil || info.ID != id || info.TID != 0 || info.Title != "flagged" || info.Star || info.Deleted || info.Archived || info.Added == "" {
		t.Fatalf("Info = %+v, %v", info, err)
	}
	synced(t, s, Task, id, 77)
	if err := errors.Join(s.SetStar(Task, id, true), s.SetDeleted(Task, id, true), s.SetArchived(id, true)); err != nil {
		t.Fatal(err)
	}
	info, _ = s.Info(id)
	if info.TID != 77 || !info.Star || !info.Deleted || !info.Archived {
		t.Errorf("after setting every flag, Info = %+v", info)
	}
	if err := errors.Join(s.SetStar(Task, id, false), s.SetDeleted(Task, id, false), s.SetArchived(id, false)); err != nil {
		t.Fatal(err)
	}
	if info, _ = s.Info(id); info.Star || info.Deleted || info.Archived {
		t.Errorf("after clearing every flag, Info = %+v", info)
	}
	if title, err := s.TitleForTID(77); err != nil || title != "flagged" {
		t.Errorf("TitleForTID(77) = %q, %v", title, err)
	}
	// a deleted note is not a link's target
	s.SetDeleted(Task, id, true)
	if _, err := s.TitleForTID(77); !errors.Is(err, ErrNotFound) {
		t.Errorf("TitleForTID of a deleted note: %v", err)
	}

	// a Kind that is not a table is refused, not formatted into the SQL
	if err := s.SetStar(Kind("task; DROP TABLE task"), id, true); err == nil {
		t.Error("SetStar accepted a kind that is not one")
	}
	if ok, _ := s.Exists(id); !ok {
		t.Error("the note is gone")
	}
}

func TestCopy(t *testing.T) {
	s := newStore(t)
	ctx, err := s.AddContainer(Context, "work", false)
	if err != nil {
		t.Fatal(err)
	}
	ctxUUID, _ := s.ContainerUUID(Context, "work")
	id, err := s.Add("original", true, ctxUUID, localdb.DefaultFolderUUID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetNote(id, "the text"); err != nil {
		t.Fatal(err)
	}
	synced(t, s, Task, id, 9)
	_ = ctx

	copyID, err := s.Copy(id)
	if err != nil || copyID == id {
		t.Fatalf("Copy = %d, %v", copyID, err)
	}
	info, _ := s.Info(copyID)
	text, _ := s.Note(copyID)
	context, _ := s.ContextOf(copyID)
	folder, _ := s.FolderOf(copyID)
	if info.Title != "Copy of original" || !info.Star || info.TID != 0 || text != "the text" || context != "work" || folder != "none" {
		t.Errorf("the copy: %+v, text %q, in %q / %q", info, text, context, folder)
	}
}

func TestContainers(t *testing.T) {
	s := newStore(t)
	for _, kind := range []Kind{Context, Folder, Keyword} {
		for _, title := range []string{"zebra", "Apple", "mango"} {
			if _, err := s.AddContainer(kind, title, title == "mango"); err != nil {
				t.Fatalf("AddContainer(%s, %s): %v", kind, title, err)
			}
		}
		cc, err := s.Containers(kind)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, c := range cc {
			got = append(got, c.Title)
		}
		want := "Apple mango zebra" // by title, whatever the case
		if kind != Keyword {
			want = "Apple mango none zebra"
		}
		if strings.Join(got, " ") != want {
			t.Errorf("Containers(%s) = %v, want %s", kind, got, want)
		}
		tt, err := s.ContainerTitles(kind)
		slices.Sort(tt)
		if err != nil || len(tt) != len(cc) {
			t.Errorf("ContainerTitles(%s) = %v, %v", kind, tt, err)
		}

		id, err := s.ContainerUUID(kind, "mango")
		if err != nil || len(id) != 36 {
			t.Errorf("ContainerUUID(%s, mango) = %q, %v; want a uuid", kind, id, err)
		}
		if _, err := s.ContainerUUID(kind, "nosuch"); !errors.Is(err, ErrNotFound) {
			t.Errorf("ContainerUUID(%s, nosuch): %v", kind, err)
		}
	}

	// the two default containers have their fixed uuids
	if id, _ := s.ContainerUUID(Context, "none"); id != localdb.DefaultContextUUID {
		t.Errorf("context none is %s", id)
	}
	if id, _ := s.ContainerUUID(Folder, "none"); id != localdb.DefaultFolderUUID {
		t.Errorf("folder none is %s", id)
	}

	// a container of notes is not a table of notes
	if _, err := s.Containers(Task); err == nil {
		t.Error("Containers(Task) was accepted")
	}
	if _, err := s.Container(Task, 1); err == nil {
		t.Error("Container(Task, 1) was accepted")
	}
}

func TestContainerCountsAndRename(t *testing.T) {
	s := newStore(t)
	workID, _ := s.AddContainer(Context, "work", false)
	work, _ := s.ContainerUUID(Context, "work")
	a, b := add(t, s, "a", ""), add(t, s, "b", "")
	add(t, s, "c", "")
	if err := errors.Join(s.SetContext(a, work), s.SetContext(b, work)); err != nil {
		t.Fatal(err)
	}

	c, err := s.Container(Context, workID)
	if err != nil || c.Title != "work" || c.Count != 2 || c.TID != 0 || c.Modified == "" {
		t.Errorf("Container(work) = %+v, %v", c, err)
	}
	if context, _ := s.ContextOf(a); context != "work" {
		t.Errorf("ContextOf(a) = %q", context)
	}
	if context, _ := s.ContextOf(add(t, s, "d", "")); context != "none" {
		t.Errorf("a new note's context is %q", context)
	}

	if err := s.RenameContainer(Context, workID, "office"); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.Container(Context, workID); c.Title != "office" || c.Count != 2 {
		t.Errorf("after the rename: %+v", c)
	}
	if err := s.SetStar(Context, workID, true); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.Container(Context, workID); !c.Star {
		t.Errorf("after SetStar: %+v", c)
	}
	if _, err := s.Container(Context, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("Container(999): %v", err)
	}

	folderID, _ := s.AddContainer(Folder, "projects", false)
	projects, _ := s.ContainerUUID(Folder, "projects")
	if err := s.SetFolder(a, projects); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.Container(Folder, folderID); c.Count != 1 {
		t.Errorf("Container(projects) = %+v", c)
	}
	if folder, _ := s.FolderOf(a); folder != "projects" {
		t.Errorf("FolderOf(a) = %q", folder)
	}
}

// Keywords hang off the server's ids, so neither a note nor a keyword that
// has not synced can be joined to the other.
func TestKeywords(t *testing.T) {
	s := newStore(t)
	note := add(t, s, "tagged", "")
	kwID, _ := s.AddContainer(Keyword, "urgent", false)
	urgent, _ := s.ContainerUUID(Keyword, "urgent")

	if err := s.AddKeyword(note, urgent); err == nil || !strings.Contains(err.Error(), "has not been synced") {
		t.Errorf("AddKeyword to a note that has not synced: %v", err)
	}
	synced(t, s, Task, note, 5)
	if err := s.AddKeyword(note, urgent); err == nil || !strings.Contains(err.Error(), "keyword has not been synced") {
		t.Errorf("AddKeyword of a keyword that has not synced: %v", err)
	}
	synced(t, s, Keyword, kwID, 3)

	if err := s.AddKeyword(note, urgent); err != nil {
		t.Fatal(err)
	}
	if err := s.AddKeyword(note, urgent); err != nil { // a second time changes nothing
		t.Fatal(err)
	}
	if kk, err := s.KeywordsOf(note); err != nil || strings.Join(kk, ",") != "urgent" {
		t.Errorf("KeywordsOf = %v, %v", kk, err)
	}
	if c, _ := s.Container(Keyword, kwID); c.Count != 1 {
		t.Errorf("the keyword's count is %d", c.Count)
	}
	if got, _ := s.Entries(ByKeyword, "urgent", byModified); titles(got) != "tagged" {
		t.Errorf("Entries(ByKeyword, urgent) = %q", titles(got))
	}

	if n, err := s.RemoveKeywords(note); err != nil || n != 1 {
		t.Errorf("RemoveKeywords = %d, %v", n, err)
	}
	if kk, _ := s.KeywordsOf(note); len(kk) != 0 {
		t.Errorf("after RemoveKeywords: %v", kk)
	}
	if n, err := s.RemoveKeywords(note); err != nil || n != 0 {
		t.Errorf("RemoveKeywords of a note with none = %d, %v", n, err)
	}
}

func TestEntries(t *testing.T) {
	s := newStore(t)
	s.AddContainer(Context, "work", false)
	work, _ := s.ContainerUUID(Context, "work")
	s.AddContainer(Folder, "projects", false)
	projects, _ := s.ContainerUUID(Folder, "projects")

	// modified a day apart, oldest first
	stamp := func(id int, day string) {
		t.Helper()
		if _, err := s.DB().Exec("UPDATE task SET modified=?, added=? WHERE id=?;", "2026-01-"+day+" 10:00:00", "2026-02-"+day+" 10:00:00", id); err != nil {
			t.Fatal(err)
		}
	}
	one, two, three, four := add(t, s, "one", "has text"), add(t, s, "two", ""), add(t, s, "three", ""), add(t, s, "four", "")
	s.SetContext(one, work)
	s.SetContext(two, work)
	s.SetFolder(three, projects)
	// every change above is a modification "now"; give the four their days
	restamp := func() {
		stamp(one, "01")
		stamp(two, "02")
		stamp(three, "03")
		stamp(four, "04")
	}
	restamp()

	list := func(by By, name string, l Listing) string {
		t.Helper()
		entries, err := s.Entries(by, name, l)
		if err != nil {
			t.Fatal(err)
		}
		return titles(entries)
	}

	if got := list(Recent, "", byModified); got != "four three two one" {
		t.Errorf("recent, newest first: %q", got)
	}
	if got := list(ByContext, "work", byModified); got != "two one" {
		t.Errorf("in context work: %q", got)
	}
	if got := list(ByFolder, "projects", byModified); got != "three" {
		t.Errorf("in folder projects: %q", got)
	}
	if got := list(ByContext, "nosuch", byModified); got != "" {
		t.Errorf("in a context that is not there: %q", got)
	}
	if got := list(Recent, "", Listing{Sort: "modified", Limit: 2}); got != "four three" {
		t.Errorf("recent, two of them: %q", got)
	}

	// the stamp is the column sorted on
	entries, _ := s.Entries(ByContext, "work", Listing{Sort: "added", Limit: 10})
	if len(entries) != 2 || entries[0].Stamp != "2026-02-02 10:00:00" || entries[1].Note != "has text" || entries[0].Note != "" {
		t.Errorf("sorted by added: %+v", entries)
	}

	s.SetStar(Task, one, true)
	restamp()
	if got := list(Recent, "", Listing{Sort: "modified", StarredFirst: true, Limit: 10}); got != "one four three two" {
		t.Errorf("starred first: %q", got)
	}

	s.SetDeleted(Task, four, true)
	s.SetArchived(three, true)
	restamp()
	if got := list(Recent, "", byModified); got != "two one" {
		t.Errorf("deleted and archived notes are left out: %q", got)
	}
	hidden, _ := s.Entries(Recent, "", Listing{Sort: "modified", ShowHidden: true, Limit: 10})
	if len(hidden) != 4 || !slices.ContainsFunc(hidden, func(e Entry) bool { return e.Title == "four" && e.Deleted }) ||
		!slices.ContainsFunc(hidden, func(e Entry) bool { return e.Title == "three" && e.Archived }) {
		t.Errorf("with hidden notes shown: %+v", hidden)
	}

	// what cannot be asked for is an error, not an empty list
	if _, err := s.Entries(Recent, "", Listing{Sort: "title; DROP TABLE task", Limit: 10}); err == nil {
		t.Error("a sort column that is not one was accepted")
	}
	if _, err := s.Entries(By(99), "", byModified); err == nil {
		t.Error("a way of choosing notes that is not one was accepted")
	}
	if ok, _ := s.Exists(one); !ok {
		t.Error("the notes are gone")
	}
}

func TestSearch(t *testing.T) {
	s := newStore(t)
	quokka := add(t, s, "Quokka sightings", "seen near the lighthouse")
	add(t, s, "Lighthouse keeping", "a quokka came by once")
	add(t, s, "Unrelated", "nothing here")
	const mark, unmark = "<<", ">>"

	entries, err := s.Search("quokka", byModified, mark, unmark)
	if err != nil {
		t.Fatal(err)
	}
	// a match in the title outranks one in the text
	if titles(entries) != "Quokka sightings Lighthouse keeping" {
		t.Errorf("search for quokka: %q", titles(entries))
	}
	if entries[0].Marked != "<<Quokka>> sightings" || entries[1].Marked != "Lighthouse keeping" {
		t.Errorf("marked titles: %q, %q", entries[0].Marked, entries[1].Marked)
	}
	if entries[0].Note != "seen near the lighthouse" || entries[0].Stamp == "" || entries[0].ID != quokka {
		t.Errorf("a search result: %+v", entries[0])
	}

	if got, _ := s.Search("quokka", Listing{Sort: "modified", Limit: 1}, mark, unmark); len(got) != 1 {
		t.Errorf("search limited to one returned %d", len(got))
	}
	if got, err := s.Search("zzzznothing", byModified, mark, unmark); err != nil || len(got) != 0 {
		t.Errorf("a search that matches nothing: %v, %v", got, err)
	}
	// a query FTS5 cannot parse is an error, not no results
	if _, err := s.Search(`"unbalanced`, byModified, mark, unmark); err == nil {
		t.Error("a malformed query returned no error")
	}

	s.SetDeleted(Task, quokka, true)
	if got, _ := s.Search("quokka", byModified, mark, unmark); titles(got) != "Lighthouse keeping" {
		t.Errorf("a deleted note is left out of a search: %q", titles(got))
	}
	if got, _ := s.Search("quokka", Listing{Sort: "modified", ShowHidden: true, Limit: 10}, mark, unmark); len(got) != 2 {
		t.Errorf("with hidden notes shown, %d results", len(got))
	}

	// one note's title and text, marked
	if title, err := s.MarkedTitle(quokka, "quokka", mark, unmark); err != nil || title != "<<Quokka>> sightings" {
		t.Errorf("MarkedTitle = %q, %v", title, err)
	}
	if text, err := s.MarkedNote(quokka, "lighthouse", mark, unmark); err != nil || text != "seen near the <<lighthouse>>" {
		t.Errorf("MarkedNote = %q, %v", text, err)
	}
	// the note was edited and no longer matches: no marked title, and the text as it is
	if err := s.SetTitle(quokka, "Wallaby sightings"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkedTitle(quokka, "quokka", mark, unmark); !errors.Is(err, ErrNotFound) {
		t.Errorf("MarkedTitle of a note that no longer matches: %v", err)
	}
	if text, err := s.MarkedNote(quokka, "quokka", mark, unmark); err != nil || text != "seen near the lighthouse" {
		t.Errorf("MarkedNote of a note that no longer matches = %q, %v", text, err)
	}
}

// A note is searchable from the moment it is written, before it has synced
// and so before it has a tid.
func TestSearchFindsANoteThatHasNotSynced(t *testing.T) {
	s := newStore(t)
	id := add(t, s, "Unsynced", "body text")
	for _, q := range []string{"unsynced", "body"} {
		got, err := s.Search(q, byModified, "", "")
		if err != nil || len(got) != 1 || got[0].ID != id || got[0].TID != 0 {
			t.Errorf("search %q: %+v, %v", q, got, err)
		}
	}
}
