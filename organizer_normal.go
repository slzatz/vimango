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
		Description: "Toggle delete status of current entry",
		Usage:       "Ctrl-D",
		Category:    "Entry Actions",
		Examples:    []string{"Ctrl-D - Mark/unmark entry as deleted"},
	})

	registry.Register(string(0x1), (*Organizer).star, CommandInfo{
		Name:        keyToDisplayName(string(0x1)),
		Description: "Toggle star status of current entry",
		Usage:       "Ctrl-A",
		Category:    "Entry Actions",
		Examples:    []string{"Ctrl-A - Mark/unmark entry as starred"},
	})

	registry.Register(string(0x18), (*Organizer).archive, CommandInfo{
		Name:        keyToDisplayName(string(0x18)),
		Description: "Toggle archive status of current entry",
		Usage:       "Ctrl-X",
		Category:    "Entry Actions",
		Examples:    []string{"Ctrl-X - Mark/unmark entry as archived"},
	})

	registry.Register("m", (*Organizer).mark, CommandInfo{
		Name:        keyToDisplayName("m"),
		Description: "Toggle mark on current entry for batch operations",
		Usage:       "m",
		Category:    "Entry Actions",
		Examples:    []string{"m - Mark/unmark entry for batch operations"},
	})

	// Navigation commands
	registry.Register(string(ctrlKey('j')), (*Organizer).scrollPreviewDown, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('j'))),
		Description: "Scroll rendered page down",
		Usage:       "Ctrl-J",
		Category:    "Navigation",
		Examples:    []string{"Ctrl-J - Scroll down in rendered page"},
	})

	registry.Register(string(ctrlKey('k')), (*Organizer).scrollPreviewUp, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('k'))),
		Description: "Scroll rendered page up",
		Usage:       "Ctrl-K",
		Category:    "Navigation",
		Examples:    []string{"Ctrl-K - Scroll up in rendered page"},
	})

	// Information commands
	registry.Register(string(ctrlKey('i')), (*Organizer).info, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('i'))),
		Description: "Show detailed information about current entry",
		Usage:       "Ctrl-I",
		Category:    "Information",
		Examples:    []string{"Ctrl-I - Display entry details (ID, context, folder, etc.)"},
	})

	// Mode Switching commands
	registry.Register(string(ctrlKey('l')), (*Organizer).switchToEditorMode, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('l'))),
		Description: "Move to editor to the left (if one is active)",
		Usage:       "Ctrl-L",
		Category:    "Mode Switching",
		Examples:    []string{"Ctrl-L - Switch to active editor if available"},
	})

	// Preview commands
	registry.Register(string(ctrlKey('w')), (*Organizer).showWebView_n, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('w'))),
		Description: "Show current note in web browser",
		Usage:       "Ctrl-W",
		Category:    "Preview",
		Examples:    []string{"Ctrl-W - Open current note in web browser"},
	})

	registry.Register(string(ctrlKey('q')), (*Organizer).closeWebView_n, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('q'))),
		Description: "Close webkit webview window",
		Usage:       "Ctrl-Q",
		Category:    "Preview",
		Examples:    []string{"Ctrl-Q - Close webview window"},
	})

	registry.Register(string(ctrlKey('y')), (*Organizer).showEditorWindows, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('y'))),
		Description: "Show open editor windows",
		Usage:       "Ctrl-Y",
		Category:    "Preview",
		Examples:    []string{"Ctrl-Y - Show active editor windows"},
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
	o.displayEntryInfo(&e)
	o.Screen.drawPreviewBox()
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

func (o *Organizer) displayEntryInfo(e *NewEntry) {
	var ab strings.Builder
	width := o.Screen.totaleditorcols - 10
	length := o.Screen.textLines - 10

	// \x1b[NC moves cursor forward by N columns
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", o.Screen.divider+6)

	//hide the cursor
	ab.WriteString("\x1b[?25l")
	// move the cursor
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, o.Screen.divider+7)

	//erase set number of chars on each line
	erase_chars := fmt.Sprintf("\x1b[%dX", o.Screen.totaleditorcols-10)
	for i := 0; i < length-1; i++ {
		ab.WriteString(erase_chars)
		ab.WriteString(lf_ret)
	}

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, o.Screen.divider+7)

	// \x1b[ 2*x is DECSACE to operate in rectable mode
	// \x1b[%d;%d;%d;%d;48;5;235$r is DECCARA to apply specified attributes (background color 235) to rectangle area
	// \x1b[ *x is DECSACE to exit rectangle mode
	fmt.Fprintf(&ab, "\x1b[2*x\x1b[%d;%d;%d;%d;48;5;235$r\x1b[*x",
		TOP_MARGIN+6, o.Screen.divider+7, TOP_MARGIN+4+length, o.Screen.divider+7+width)
	ab.WriteString("\x1b[48;5;235m") //draws the box lines with same background as above rectangle

	fmt.Fprintf(&ab, "id: %d%s", e.id, lf_ret)
	fmt.Fprintf(&ab, "tid: %d%s", e.tid, lf_ret)

	title := fmt.Sprintf("title: %s", e.title)
	if len(title) > width {
		title = title[:width-3] + "..."
	}
	//coloring labels will take some work b/o gray background
	//s.append(fmt::format("{}title:{} {}{}", COLOR_1, "\x1b[m", title, lf_ret));
	fmt.Fprintf(&ab, "%s%s", title, lf_ret)

	context := o.Database.filterTitle("context", e.context_tid)
	fmt.Fprintf(&ab, "context: %s%s", context, lf_ret)

	folder := o.Database.filterTitle("folder", e.folder_tid)
	fmt.Fprintf(&ab, "folder: %s%s", folder, lf_ret)

	fmt.Fprintf(&ab, "star: %t%s", e.star, lf_ret)
	fmt.Fprintf(&ab, "deleted: %t%s", e.deleted, lf_ret)

	fmt.Fprintf(&ab, "completed: %t%s", e.archived, lf_ret)
	fmt.Fprintf(&ab, "modified: %s%s", e.modified, lf_ret)
	fmt.Fprintf(&ab, "added: %s%s", e.added, lf_ret)

	fmt.Fprintf(&ab, "keywords: %s%s", app.Database.getTaskKeywords(e.id), lf_ret)

	fmt.Print(ab.String())
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
	width := o.Screen.totaleditorcols - 10
	length := o.Screen.textLines - 10

	// \x1b[NC moves cursor forward by N columns
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", o.Screen.divider+6)

	//hide the cursor
	ab.WriteString("\x1b[?25l")
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, o.Screen.divider+7)

	//erase set number of chars on each line
	erase_chars := fmt.Sprintf("\x1b[%dX", o.Screen.totaleditorcols-10)
	for i := 0; i < length-1; i++ {
		ab.WriteString(erase_chars)
		ab.WriteString(lf_ret)
	}

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, o.Screen.divider+7)

	fmt.Fprintf(&ab, "\x1b[2*x\x1b[%d;%d;%d;%d;48;5;235$r\x1b[*x",
		TOP_MARGIN+6, o.Screen.divider+7, TOP_MARGIN+4+length, o.Screen.divider+7+width)
	ab.WriteString("\x1b[48;5;235m") //draws the box lines with same background as above rectangle

	//ab.append(COLOR_6); // Blue depending on theme

	fmt.Fprintf(&ab, "id: %d%s", c.id, lf_ret)
	fmt.Fprintf(&ab, "tid: %d%s", c.tid, lf_ret)

	title := fmt.Sprintf("title: %s", c.title)
	if len(title) > width {
		title = title[:width-3] + "..."
	}

	fmt.Fprintf(&ab, "star: %t%s", c.star, lf_ret)
	fmt.Fprintf(&ab, "deleted: %t%s", c.deleted, lf_ret)

	fmt.Fprintf(&ab, "modified: %s%s", c.modified, lf_ret)
	fmt.Fprintf(&ab, "entry count: %d%s", c.count, lf_ret)

	fmt.Print(ab.String())
	o.Screen.drawPreviewBox()
}
