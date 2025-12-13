package main

import (
	"fmt"
	"strings"

	"github.com/slzatz/vimango/vim"
	//	"github.com/charmbracelet/glamour"
)

func (a *App) setOrganizerNormalCmds(organizer *Organizer) map[string]func(*Organizer) {
	registry := NewCommandRegistry[func(*Organizer)]()

	// Entry Actions commands
	registry.Register(string(0x4), (*Organizer).del, CommandInfo{
		Name:        keyToDisplayName(string(0x4)),
		Description: "Toggle delete status of current note",
		Category:    "Entry Actions",
	})

	registry.Register(string(0x1), (*Organizer).star, CommandInfo{
		Name:        keyToDisplayName(string(0x1)),
		Description: "Toggle star status of current note",
		Category:    "Entry Actions",
	})

	registry.Register(string(0x18), (*Organizer).archive, CommandInfo{
		Name:        keyToDisplayName(string(0x18)),
		Description: "Toggle archive status of current note",
		Category:    "Entry Actions",
	})

	registry.Register("m", (*Organizer).mark, CommandInfo{
		Name:        keyToDisplayName("m"),
		Description: "Toggle mark on current note for batch operations",
		Category:    "Entry Actions",
	})

	// Navigation commands
	registry.Register(string(ctrlKey('j')), (*Organizer).scrollPreviewDown, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('j'))) + " or ↓",
		Aliases:     []string{string(ARROW_DOWN)},
		Description: "Scroll rendered note down",
		Category:    "Navigation",
	})

	registry.Register(string(ctrlKey('k')), (*Organizer).scrollPreviewUp, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('k'))) + " or ↑",
		Aliases:     []string{string(ARROW_UP)},
		Description: "Scroll rendered note up",
		Category:    "Navigation",
	})

	// Information commands
	registry.Register(string(ctrlKey('i')), (*Organizer).info, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('i'))),
		Description: "Show detailed information about current note",
		Category:    "Information",
	})

	// Mode Switching commands
	registry.Register(string(ctrlKey('l')), (*Organizer).switchToEditorMode, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('l'))),
		Description: "Switch to editor (if one is active)",
		Category:    "Mode Switching",
	})

	// Preview commands
	registry.Register(string(ctrlKey('w')), (*Organizer).showWebView_n, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('w'))),
		Description: "Show current note in web browser",
		Category:    "Preview",
	})

	registry.Register(string(ctrlKey('q')), (*Organizer).closeWebView_n, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('q'))),
		Description: "Close webkit webview window",
		Category:    "Preview",
	})

	registry.Register(string(ctrlKey('y')), (*Organizer).showEditorWindows, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('y'))),
		Description: "Show open editor windows",
		Category:    "Preview",
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

func (o *Organizer) showEditorWindows() {
	o.Screen.eraseRightScreen()
	o.Screen.drawRightScreen()
}

func (o *Organizer) switchToEditorMode() {
	if len(o.Session.Windows) == 0 {
		o.ShowMessage(BL, "%sThere are no active editors%s", RED_BG, RESET)
		return
	}
	o.Session.editorMode = true
	ae := app.Session.activeEditor
	vim.SetCurrentBuffer(ae.vbuf)
	// below necessary because libvim does not set cursor column correctly
	vim.SetCursorPosition(ae.fr+1, ae.fc) //ae.fr is 0-based, vim expects 1-based
	o.Screen.eraseRightScreen()
	o.Screen.drawRightScreen()
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

// for scrolling reports (notices) like help
func (o *Organizer) scrollNoticeDown() {
	if len(o.notice) == 0 {
		return
	}
	if o.altRowoff == len(o.note)-2 {
		o.ShowMessage(BL, "Reached end of rendered note")
		return
	}
	o.altRowoff++
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

func (o *Organizer) showWebView_n() {
	o.showWebView(0)
}

func (o *Organizer) closeWebView_n() {
	o.closeWebView(0)
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
