package main

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	//	"github.com/charmbracelet/glamour"
)

func (a *App) setOrganizerNormalCmds(organizer *Organizer) map[string]func(*Organizer) {
	registry := NewCommandRegistry[func(*Organizer)]()

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
	err := o.Database.toggleDeleted(id, state, o.view.String())
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
	err := o.Database.toggleStar(id, state, o.view.String())
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
	err := o.Database.toggleArchived(id, state, o.view.String())
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
	e := o.Database.getEntryInfo(o.getId())
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

// for scrolling reports (notices) like help
func (o *Organizer) scrollNoticeDown() {
	if len(o.notice) == 0 {
		return
	}
	if o.altRowoff == len(o.notice)-2 {
		o.ShowMessage(BL, "Reached end of rendered note")
		return
	}
	o.altRowoff++
	o.ShowMessage(BL, "Line %d of %d", o.altRowoff, len(o.note))
	o.drawNoticeLayer()
	o.drawNoticeText()
}

// noticeRows is how many lines of a notice drawNoticeText shows: all of a
// notice that fits in its box, otherwise the box less two rows.
func (o *Organizer) noticeRows() int {
	box := o.Screen.textLines - 10
	if len(o.notice) <= box {
		return len(o.notice)
	}
	return max(box-2, 1)
}

// for paging reports (notices) like help by half the box
func (o *Organizer) pageNoticeDown() {
	if len(o.notice) == 0 {
		return
	}
	last := len(o.notice) - o.noticeRows()
	if o.altRowoff >= last {
		o.ShowMessage(BL, "Reached end of rendered note")
		return
	}
	o.altRowoff = min(o.altRowoff+max(o.noticeRows()/2, 1), last)
	o.drawNoticeLayer()
	o.drawNoticeText()
}

func (o *Organizer) pageNoticeUp() {
	if len(o.notice) == 0 || o.altRowoff == 0 {
		return
	}
	o.altRowoff = max(o.altRowoff-max(o.noticeRows()/2, 1), 0)
	o.drawNoticeLayer()
	o.drawNoticeText()
}

func (o *Organizer) scrollNoticeHome() {
	if len(o.notice) == 0 {
		return
	}
	o.altRowoff = 0
	o.ShowMessage(BL, "Line %d of %d", o.altRowoff, len(o.note))
	o.drawNoticeLayer()
	o.drawNoticeText()
}

// for scrolling reports (notices) like help
func (o *Organizer) scrollNoticeUp() {
	if len(o.notice) == 0 {
		return
	}
	if o.altRowoff > 0 {
		o.altRowoff--
		o.drawNoticeLayer()
		o.drawNoticeText()
	}
}

func (o *Organizer) displayEntryInfo(e *NewEntry) string {
	width := o.Screen.totaleditorcols - 10
	var ab strings.Builder

	fmt.Fprintf(&ab, "id: %d%s", e.id, "\n")
	fmt.Fprintf(&ab, "tid: %d%s", e.tid, "\n")

	title := fmt.Sprintf("title: %s", e.title)
	if len(title) > width {
		title = title[:width-3] + "..."
	}
	fmt.Fprintf(&ab, "%s%s", title, "\n")

	// Use taskContext/taskFolder which join on uuid
	context := o.Database.taskContext(e.id)
	fmt.Fprintf(&ab, "**context**: %s%s", context, "\n")

	folder := o.Database.taskFolder(e.id)
	fmt.Fprintf(&ab, "folder: %s%s", folder, "\n")

	fmt.Fprintf(&ab, "star: %t%s", e.star, "\n")
	fmt.Fprintf(&ab, "deleted: %t%s", e.deleted, "\n")

	fmt.Fprintf(&ab, "completed: %t%s", e.archived, "\n")
	fmt.Fprintf(&ab, "modified: %s%s", e.modified, "\n")
	fmt.Fprintf(&ab, "added: %s%s", e.added, "\n")

	fmt.Fprintf(&ab, "keywords: %s%s", app.Database.getTaskKeywords(e.id), "\n")

	return ab.String()
}

func (o *Organizer) displayContainerInfo() {

	/*
		type Container struct {
			id       int
			tid      int
			title    string
			star     bool
			deleted  bool
			modified string
			count    int
		}
	*/
	c := o.Database.getContainerInfo(o.rows[o.fr].id, o.view)

	if c.id == 0 {
		return
	}

	var ab strings.Builder

	fmt.Fprintf(&ab, "id: %d%s", c.id, "\n")
	fmt.Fprintf(&ab, "tid: %d%s", c.tid, "\n")

	fmt.Fprintf(&ab, "title: %s%s", c.title, "\n")
	//title := fmt.Sprintf("**title**: %s", c.title)

	fmt.Fprintf(&ab, "star: %t%s", c.star, "\n")
	fmt.Fprintf(&ab, "deleted: %t%s", c.deleted, "\n")

	fmt.Fprintf(&ab, "modified: %s%s", c.modified, "\n")
	fmt.Fprintf(&ab, "note count: %d%s", c.count, "\n")

	o.drawNotice(ab.String())
}

func (o *Organizer) newContextNormal() {
	o.altView = CONTEXT
	o.command = ""
	o.command_line = ""

	context_map := o.Database.contextList()
	context_slice := slices.Collect(maps.Keys(context_map))
	sort.Strings(context_slice)
	o.containerList = context_slice
	var sb strings.Builder
	for i, k := range context_slice {
		fmt.Fprint(&sb, fmt.Sprintf("%d. %s\n", i+1, k))
	}
	//contexts := strings.Join(context_slice, "\n")
	o.mode = CONTAINER
	//o.drawNotice(contexts)
	o.drawNotice(sb.String())
	o.altRowoff = 0
	//o.command_line = "" // needed because not going into normal mode right away where it is cleared on typing ":"
}

func (o *Organizer) newFolderNormal() {
	o.altView = FOLDER
	o.command = ""
	o.command_line = ""

	folder_map := o.Database.folderList()
	folder_slice := slices.Collect(maps.Keys(folder_map))
	sort.Strings(folder_slice)
	o.containerList = folder_slice
	var sb strings.Builder
	for i, k := range folder_slice {
		fmt.Fprint(&sb, fmt.Sprintf("%d. %s\n", i+1, k))
	}
	//contexts := strings.Join(context_slice, "\n")
	o.mode = CONTAINER
	//o.drawNotice(contexts)
	o.drawNotice(sb.String())
	o.altRowoff = 0
	//o.command_line = "" // needed because not going into normal mode right away where it is cleared on typing ":"
}
