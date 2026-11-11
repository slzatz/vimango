package main

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/slzatz/vimango/internal/store"
	//	"github.com/charmbracelet/glamour"
)

func (a *App) setOrganizerNormalCmds(organizer *Organizer) map[string]func(*Organizer) {
	registry := NewKeyRegistry[func(*Organizer)]()

	// Entry Actions commands
	registry.Register(string(rune(0x4)), (*Organizer).del, CommandInfo{
		Name:        keyToDisplayName(string(rune(0x4))),
		Description: "Toggle delete status of current note",
		Category:    "Entry Actions",
	})

	registry.Register(string(rune(0x1)), (*Organizer).star, CommandInfo{
		Name:        keyToDisplayName(string(rune(0x1))),
		Description: "Toggle star status of current note",
		Category:    "Entry Actions",
	})

	registry.Register(string(rune(0x18)), (*Organizer).archive, CommandInfo{
		Name:        keyToDisplayName(string(rune(0x18))),
		Description: "Toggle archive status of current note",
		Category:    "Entry Actions",
	})

	registry.Register("m", (*Organizer).mark, CommandInfo{
		Name:        keyToDisplayName("m"),
		Description: "Toggle mark on current note for batch operations",
		Category:    "Entry Actions",
	})

	// Navigation commands
	registry.Register(string(rune(ctrlKey('j'))), (*Organizer).scrollPreviewDown, CommandInfo{
		Name:        keyToDisplayName(string(rune(ctrlKey('j')))),
		Description: "Scroll rendered note down one line",
		Category:    "Navigation",
	})

	registry.Register(string(rune(ctrlKey('k'))), (*Organizer).scrollPreviewUp, CommandInfo{
		Name:        keyToDisplayName(string(rune(ctrlKey('k')))),
		Description: "Scroll rendered note up one line",
		Category:    "Navigation",
	})

	// Half-page paging, as in vimango_hybrid's preview. Space and b are plain
	// characters, so they survive ssh and rex, which often drop Ctrl-J/K.
	// Shift-Space is Space here: terminals send it as a bare space (and the
	// kitty protocol's CSI-u form decodes to one too), so the way back is b,
	// as in less. They take Space (vim's l) and b from title editing.
	registry.Register(" ", (*Organizer).pagePreviewDown, CommandInfo{
		Name:        "Space/PgDn",
		Aliases:     []string{string(rune(PAGE_DOWN))},
		Description: "Scroll rendered note down half a page",
		Category:    "Navigation",
	})

	registry.Register("b", (*Organizer).pagePreviewUp, CommandInfo{
		Name:        "b/PgUp",
		Aliases:     []string{string(rune(PAGE_UP))},
		Description: "Scroll rendered note up half a page",
		Category:    "Navigation",
	})

	registry.Register(string(rune(ctrlKey('f'))), (*Organizer).newFolderNormal, CommandInfo{
		Name:        keyToDisplayName(string(rune(ctrlKey('f')))),
		Description: "Assign a folder to the current note",
		Category:    "Entry Actions",
	})

	registry.Register(string(rune(ctrlKey('c'))), (*Organizer).newContextNormal, CommandInfo{
		Name:        keyToDisplayName(string(rune(ctrlKey('c')))),
		Description: "Assign a context to the current note",
		Category:    "Entry Actions",
	})

	registry.Register(string(rune(HOME_KEY)), (*Organizer).scrollPreviewHome, CommandInfo{
		Name:        "Home",
		Description: "Scroll rendered note to top",
		Category:    "Navigation",
	})

	// Information commands
	registry.Register(string(rune(ctrlKey('i'))), (*Organizer).info, CommandInfo{
		Name:        keyToDisplayName(string(rune(ctrlKey('i')))),
		Description: "Show detailed information about current note",
		Category:    "Information",
	})

	// Store registry in organizer for help command access
	organizer.normalCommandRegistry = registry

	return registry.GetFunctionMap()
}

func (o *Organizer) mark() {
	if o.view != TASK {
		o.ShowMessage(BL, "You can only mark tasks")
		return
	}

	if _, found := o.marked_entries[o.rows[o.fr].id]; found {
		delete(o.marked_entries, o.rows[o.fr].id)
	} else {
		o.marked_entries[o.rows[o.fr].id] = struct{}{}
	}
	o.ShowMessage(BL, "Toggle mark for item %d", o.rows[o.fr].id)
}

func (o *Organizer) del() {
	id := o.rows[o.fr].id
	state := o.rows[o.fr].deleted
	err := o.Store.SetDeleted(o.view.kind(), id, !state)
	if err != nil {
		o.ShowMessage(BL, "Error toggling %s id %d to deleted: %v", o.view, id, err)
		return
	}
	o.rows[o.fr].deleted = !state
	o.ShowMessage(BL, "Toggle deleted for %s id %d succeeded (new)", o.view, id)
}

func (o *Organizer) star() {
	id := o.rows[o.fr].id
	state := o.rows[o.fr].star
	err := o.Store.SetStar(o.view.kind(), id, !state)
	if err != nil {
		o.showMessage("Error toggling %s id %d to star: %v", o.view, id, err)
		return
	}
	o.rows[o.fr].star = !state
	o.ShowMessage(BL, "Toggle star for %s id %d succeeded (new)", o.view, id)
}

func (o *Organizer) archive() {
	id := o.rows[o.fr].id
	state := o.rows[o.fr].archived
	err := o.Store.SetArchived(id, !state)
	if err != nil {
		o.ShowMessage(BL, "Error toggling %s id %d to archived: %v", o.view, id, err)
		return
	}
	o.rows[o.fr].archived = !state
	o.ShowMessage(BL, "Toggle archive for %s id %d succeeded (new)", o.view, id)
}

func (o *Organizer) info() {
	if o.view != TASK {
		return
	}
	e, err := o.Store.Info(o.getId())
	if err != nil {
		o.ShowMessage(BL, "Error in getEntryInfo for id %d: %v", o.getId(), err)
		return
	}
	info := o.displayEntryInfo(&e)
	o.drawNotice(info)
	o.altRowoff = 0
	o.mode = NAVIGATE_NOTICE
}

// for scrolling terminal markdown rendered note
func (o *Organizer) scrollPreviewDown() {
	if o.altRowoff == len(o.note)-1 {
		o.ShowMessage(BL, "Reached end of rendered note")
		return
	}
	o.altRowoff++
	o.ShowMessage(BL, "Line %d of %d", o.altRowoff, len(o.note))
	o.Screen.eraseRightScreen()
	o.drawRenderedNote()
}

// for scrolling terminal markdown rendered note
func (o *Organizer) scrollPreviewUp() {
	if o.altRowoff > 0 {
		o.altRowoff--
		o.Screen.eraseRightScreen()
		o.drawRenderedNote()
	}
}

// for paging the terminal markdown rendered note by half a screen; it stops
// once the note's last line reaches the bottom of the pane
func (o *Organizer) pagePreviewDown() {
	last := max(len(o.note)-o.Screen.textLines, 0)
	if o.altRowoff >= last {
		o.ShowMessage(BL, "Reached end of rendered note")
		return
	}
	o.altRowoff = min(o.altRowoff+max(o.Screen.textLines/2, 1), last)
	o.ShowMessage(BL, "Line %d of %d", o.altRowoff, len(o.note))
	o.Screen.eraseRightScreen()
	o.drawRenderedNote()
}

func (o *Organizer) pagePreviewUp() {
	if o.altRowoff > 0 {
		o.altRowoff = max(o.altRowoff-max(o.Screen.textLines/2, 1), 0)
		o.Screen.eraseRightScreen()
		o.drawRenderedNote()
	}
}

func (o *Organizer) scrollPreviewHome() {
	o.altRowoff = 0
	o.Screen.eraseRightScreen()
	o.drawRenderedNote()
}

// drawNotice shows text in the notice box (notice.go) over the right pane.
func (o *Organizer) drawNotice(text string) {
	hang := 0
	if o.mode == HELP {
		hang = 16
	}
	o.Screen.showNotice(text, hang)
}

// The keys that scroll a notice, as normal-mode commands of the organizer.
// What the scrolling has to say goes on the organizer's message line.
func (o *Organizer) scrollNoticeDown() { o.noticeSays(o.Screen.scrollNoticeDown()) }
func (o *Organizer) scrollNoticeUp()   { o.noticeSays(o.Screen.scrollNoticeUp()) }
func (o *Organizer) scrollNoticeHome() { o.noticeSays(o.Screen.scrollNoticeHome()) }
func (o *Organizer) pageNoticeDown()   { o.noticeSays(o.Screen.pageNoticeDown()) }
func (o *Organizer) pageNoticeUp()     { o.noticeSays(o.Screen.pageNoticeUp()) }

func (o *Organizer) noticeSays(message string) {
	if message != "" {
		o.ShowMessage(BL, "%s", message)
	}
}

func (o *Organizer) displayEntryInfo(e *store.Info) string {
	width := o.Screen.totaleditorcols - 10
	var ab strings.Builder

	fmt.Fprintf(&ab, "id: %d%s", e.ID, "\n")
	fmt.Fprintf(&ab, "tid: %d%s", e.TID, "\n")

	title := fmt.Sprintf("title: %s", e.Title)
	if runewidth.StringWidth(title) > width {
		title, _ = clipColumns(title, 0, width-3)
		title += "..."
	}
	fmt.Fprintf(&ab, "%s%s", title, "\n")

	// Use taskContext/taskFolder which join on uuid
	context := o.contextOf(e.ID)
	fmt.Fprintf(&ab, "**context**: %s%s", context, "\n")

	folder := o.folderOf(e.ID)
	fmt.Fprintf(&ab, "folder: %s%s", folder, "\n")

	fmt.Fprintf(&ab, "star: %t%s", e.Star, "\n")
	fmt.Fprintf(&ab, "deleted: %t%s", e.Deleted, "\n")

	fmt.Fprintf(&ab, "completed: %t%s", e.Archived, "\n")
	fmt.Fprintf(&ab, "modified: %s%s", e.Modified, "\n")
	fmt.Fprintf(&ab, "added: %s%s", e.Added, "\n")

	fmt.Fprintf(&ab, "keywords: %s%s", o.keywordsOf(e.ID), "\n")

	return ab.String()
}

func (o *Organizer) displayContainerInfo() {

	id := o.rows[o.fr].id
	if id == -1 {
		return // a row not saved yet
	}
	c, err := o.Store.Container(o.view.kind(), id)
	if err != nil {
		o.ShowMessage(BL, "Error in getContainerInfo: %v", err)
		return
	}

	var ab strings.Builder

	fmt.Fprintf(&ab, "id: %d%s", c.ID, "\n")
	fmt.Fprintf(&ab, "tid: %d%s", c.TID, "\n")

	fmt.Fprintf(&ab, "title: %s%s", c.Title, "\n")

	fmt.Fprintf(&ab, "star: %t%s", c.Star, "\n")
	fmt.Fprintf(&ab, "deleted: %t%s", c.Deleted, "\n")

	fmt.Fprintf(&ab, "modified: %s%s", c.Modified, "\n")
	fmt.Fprintf(&ab, "note count: %d%s", c.Count, "\n")

	o.drawNotice(ab.String())
}

// Ctrl-C and Ctrl-F: pick a context or folder for the current entry.
func (o *Organizer) newContextNormal() { o.pickContainer(CONTEXT) }
func (o *Organizer) newFolderNormal()  { o.pickContainer(FOLDER) }
