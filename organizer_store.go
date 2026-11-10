package main

import (
	"errors"
	"slices"
	"strings"

	"github.com/slzatz/vimango/internal/localdb"
	"github.com/slzatz/vimango/internal/store"
)

// What the organizer asks of the store, in the organizer's terms.
//
// The store returns data and errors and shows nothing. These are where an
// error becomes a message on the organizer's status line and an answer the
// caller can still draw with: no rows, no text. A function here that returns
// an error instead leaves the saying to a caller with more to say.

// The escape sequences a matched search term is wrapped in, in the title
// column of a search result. In a note's text the terms are marked qx…qy,
// which the renderer turns into colour.
const ftsMark, ftsUnmark = "\x1b[48;5;31m", "\x1b[49m"

const noteMark, noteUnmark = "qx", "qy"

// kind is the store's name for what a view lists.
func (v View) kind() store.Kind { return store.Kind(v.String()) }

// listing is how the organizer's lists are cut and ordered at the moment.
func (o *Organizer) listing() store.Listing {
	return store.Listing{Sort: o.sort, StarredFirst: o.sortPriority, ShowHidden: o.show_deleted, Limit: MAX}
}

func rowsFromEntries(entries []store.Entry) []Row {
	rows := make([]Row, 0, len(entries))
	for _, e := range entries {
		row := Row{id: e.ID, tid: e.TID, title: e.Title, ftsTitle: e.Marked, star: e.Star, deleted: e.Deleted, archived: e.Archived}
		row.hasImage = e.Note != "" && containsGoogleDriveImage(e.Note)
		if e.Stamp != "" {
			row.sort = timeDelta(e.Stamp)
		}
		rows = append(rows, row)
	}
	return rows
}

// entries returns the notes the current filter asks for -- a context's, a
// folder's, a keyword's, or the recent ones -- and says so if the query fails.
func (o *Organizer) entries() []Row {
	by, known := map[int]store.By{
		BY_CONTEXT: store.ByContext, BY_FOLDER: store.ByFolder, BY_KEYWORD: store.ByKeyword, BY_RECENT: store.Recent,
	}[o.taskview]
	if !known {
		o.ShowMessage(BL, "You asked for an unsupported db query")
		return nil
	}
	entries, err := o.Store.Entries(by, o.filter, o.listing())
	if err != nil {
		o.showMessage("Error filtering entries: %v", err)
		return nil
	}
	return rowsFromEntries(entries)
}

// search returns the notes matching a full-text query, best match first. The
// error is the caller's to show: a failed search leaves the list as it was.
func (o *Organizer) search(terms string) ([]Row, error) {
	entries, err := o.Store.Search(terms, o.listing(), ftsMark, ftsUnmark)
	return rowsFromEntries(entries), err
}

// containerRows lists the contexts, folders or keywords the view is of.
func (o *Organizer) containerRows() []Row {
	containers, err := o.Store.Containers(o.view.kind())
	if err != nil {
		o.ShowMessage(BL, "Error listing %ss: %v", o.view, err)
		return nil
	}
	rows := make([]Row, 0, len(containers))
	for _, c := range containers {
		rows = append(rows, Row{id: c.ID, title: c.Title, star: c.Star, deleted: c.Deleted, sort: timeDelta(c.Modified)})
	}
	return rows
}

// containerTitles is the titles of every context, folder or keyword, sorted.
func (o *Organizer) containerTitles(kind store.Kind) []string {
	titles, err := o.Store.ContainerTitles(kind)
	if err != nil {
		o.ShowMessage(BL, "Error listing %ss: %v", kind, err)
		return nil
	}
	slices.Sort(titles)
	return titles
}

// containerUUID is the uuid of the context, folder or keyword with a title,
// and whether there is one.
func (o *Organizer) containerUUID(kind store.Kind, title string) (string, bool) {
	uuid, err := o.Store.ContainerUUID(kind, title)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			o.ShowMessage(BL, "Error looking up %s %q: %v", kind, title, err)
		}
		return "", false
	}
	return uuid, true
}

// noteText is a note's text, for the preview and the commands that work on
// it. The row of a note not yet saved (id -1) has none.
func (o *Organizer) noteText(id int) string {
	if id == -1 {
		return ""
	}
	text, err := o.Store.Note(id)
	if err != nil {
		o.ShowMessage(BL, "Error retrieving note for id %d: %v", id, err)
	}
	return text
}

// markedNoteText is a note's text with the terms of the current search
// marked, for the preview of a search result.
func (o *Organizer) markedNoteText(id int) string {
	if id == -1 {
		return ""
	}
	text, err := o.Store.MarkedNote(id, o.Session.fts_search_terms, noteMark, noteUnmark)
	if err != nil {
		o.ShowMessage(BL, "Error highlighting search terms: %v", err)
	}
	return text
}

// contextOf, folderOf and keywordsOf are what the status bar and the info
// pane show for a note: drawn on every key, so an error is just a blank.
func (o *Organizer) contextOf(id int) string {
	title, _ := o.Store.ContextOf(id)
	return title
}

func (o *Organizer) folderOf(id int) string {
	title, _ := o.Store.FolderOf(id)
	return title
}

func (o *Organizer) keywordsOf(id int) string {
	keywords, err := o.Store.KeywordsOf(id)
	if err != nil {
		o.ShowMessage(BL, "Error in getTaskKeywords for entry id %d: %v", id, err)
		return ""
	}
	return strings.Join(keywords, ",")
}

// refreshMarkedTitle brings a search result's highlighted title up to date
// after its title was edited; plain if the new title no longer matches.
func (o *Organizer) refreshMarkedTitle(row *Row) {
	marked, err := o.Store.MarkedTitle(row.id, o.Session.fts_search_terms, ftsMark, ftsUnmark)
	if err != nil {
		marked = row.title
	}
	row.ftsTitle = marked
}

// newEntryContainers is where a new note goes: the context or the folder the
// list is of, and the default "none" for the other.
func (o *Organizer) newEntryContainers() (contextUUID, folderUUID string) {
	contextUUID, folderUUID = localdb.DefaultContextUUID, localdb.DefaultFolderUUID
	switch o.taskview {
	case BY_CONTEXT:
		if uuid, ok := o.containerUUID(store.Context, o.filter); ok {
			contextUUID = uuid
		}
	case BY_FOLDER:
		if uuid, ok := o.containerUUID(store.Folder, o.filter); ok {
			folderUUID = uuid
		}
	}
	return contextUUID, folderUUID
}

// addEntry writes a row that is not yet in the database as a new note, in the
// context or folder the list is of, and gives the row its id.
func (o *Organizer) addEntry(row *Row, contextUUID, folderUUID string) error {
	id, err := o.Store.Add(row.title, row.star, contextUUID, folderUUID)
	if err != nil {
		return err
	}
	row.id = id
	row.dirty = false
	return nil
}

// saveContainerTitle writes a container row's title: a rename, or a new
// container if the row is not in the database yet.
func (o *Organizer) saveContainerTitle(row *Row) error {
	if row.id != -1 {
		return o.Store.RenameContainer(o.view.kind(), row.id, row.title)
	}
	id, err := o.Store.AddContainer(o.view.kind(), row.title, row.star)
	if err != nil {
		return err
	}
	row.id = id
	row.dirty = false
	return nil
}
