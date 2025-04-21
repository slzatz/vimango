package main

import (
	"fmt"
	"strings"

	//	"github.com/charmbracelet/glamour"
	"github.com/slzatz/vimango/vim"
)

func (a *App) setOrganizerNormalCmds() map[string]func(*Organizer) {
	return map[string]func(*Organizer){
		//"dd":                 (*Organizer).del, //delete
		string(0x4):          (*Organizer).del,     //ctrl-d delete
		string(0x1):          (*Organizer).star,    //ctrl-b starEntry
		string(0x18):         (*Organizer).archive, //ctrl-x archive
		string(ctrlKey('i')): (*Organizer).info,    //{{0x9}} entryInfo
		"m":                  (*Organizer).mark,
		string(ctrlKey('l')): (*Organizer).switchToEditorMode,
		":":                  (*Organizer).exCmd,
		string(ctrlKey('j')): (*Organizer).scrollPreviewDown,
		string(ctrlKey('k')): (*Organizer).scrollPreviewUp,
		string(ctrlKey('n')): (*Organizer).previewWithImages,
	}
}

func (o *Organizer) exCmd() {
	o.ShowMessage(BL, ":")
	o.command_line = ""
	o.last_mode = o.mode //at the least picks up NORMAL and NO_ROWS
	o.mode = COMMAND_LINE
}

/*
func noop() {
	return
}
*/

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
	e := o.Database.getEntryInfo(o.getId())
	o.displayEntryInfo(&e)
	o.Screen.drawPreviewBox()
}

func (o *Organizer) switchToEditorMode() {
	if len(o.Session.Windows) == 0 {
		o.ShowMessage(BL, "There are no active editors")
		return
	}
	o.Screen.eraseRightScreen()
	o.Screen.drawRightScreen()
	o.Session.editorMode = true
	vim.BufferSetCurrent(app.Session.activeEditor.vbuf)
}

func (o *Organizer) scrollPreviewDown() {
	//if len(org.note) > org.altRowoff+org.textLines {
	o.altRowoff++
	o.drawPreview()
	//}
}

func (o *Organizer) scrollPreviewUp() {
	if o.altRowoff > 0 {
		o.altRowoff--
		o.drawPreview()
	}
}

/*
// this rendered links and I should go back to take a look at it
func controlZ() {
	id := org.rows[org.fr].id
	note := app.Database.readNoteIntoString(id)
	note = generateWWString(note, org.Screen.totaleditorcols)
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("darkslz.json"),
		glamour.WithWordWrap(0),
		//glamour.WithLinkNumbers(true), //12312023 -- trying to use standard glamour
	)
	note, _ = r.Render(note)
	// glamour seems to add a '\n' at the start
	note = strings.TrimSpace(note)
	note = strings.ReplaceAll(note, "^^^", "\n") ///////////////04052022
	org.note = strings.Split(note, "\n")
	app.Screen.eraseRightScreen()
	if !app.Session.imagePreview {
		org.drawPreviewWithoutImages()
	} else {
		org.drawPreviewWithImages()
	}
	org.mode = LINKS
	org.ShowMessage(BL, "\x1b[1mType a number to choose a link\x1b[0m")
}
*/

func (o *Organizer) previewWithImages() {
	o.Screen.eraseRightScreen()
	o.drawPreviewWithImages()
	o.Session.imagePreview = true
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
