package main

import (
	"fmt"
	"sort"

	"github.com/slzatz/vimango/internal/store"
	"github.com/slzatz/vimango/vim"
)

// RedrawScope represents the extent of organizer redraw work required after a key event.
type RedrawScope int

const (
	RedrawNone RedrawScope = iota
	RedrawPartial
	RedrawFull
)

type Organizer struct {
	mode Mode
	//last_mode Mode

	cx, cy    int //cursor x and y position
	fc, fr    int // file x and y position
	rowoff    int //the number of rows scrolled (aka number of top rows now off-screen
	altRowoff int //the number of rows scrolled in the right window (aka number of top rows now off-screen)
	coloff    int //the number of display columns of the current title scrolled off the left

	rows           []Row
	filter         string
	sort           string
	sortPriority   bool
	command_line   string
	note           []string // the preview
	command        string
	show_deleted   bool
	view           View
	altView        View //int
	taskview       int
	marked_entries map[int]struct{} // map instead of list makes toggling a row easier
	highlight      [2]int
	vbuf           *vim.Buffer
	bufferTick     int
	//saveTick              int
	normalCmds            map[string]func(*Organizer)
	exCmds                map[string]func(*Organizer, exArgs)
	commandRegistry       *CommandRegistry[func(*Organizer, exArgs)]
	normalCommandRegistry *CommandRegistry[func(*Organizer)]
	filterList            []FilterNames
	containerList         []string
	tabCompletion         struct {
		list  []FilterNames
		index int
	}
	Store   *store.Store
	Session *Session
	Screen  *Screen
}

type FilterNames struct {
	Text string
	Char rune
}

// filterNames is every context and folder, by title: what Tab completes on
// the organizer's command line.
func (o *Organizer) filterNames() []FilterNames {
	fnlist := []FilterNames{}
	for _, title := range o.containerTitles(store.Context) {
		fnlist = append(fnlist, FilterNames{Text: title, Char: 'c'})
	}
	for _, title := range o.containerTitles(store.Folder) {
		fnlist = append(fnlist, FilterNames{Text: title, Char: 'f'})
	}
	sort.Slice(fnlist, func(i, j int) bool {
		return fnlist[i].Text < fnlist[j].Text
	})
	return fnlist
}

// setRows makes rows the organizer's list, with the first row current: the
// rows, vim's buffer of their titles, and the cursor in both. Everything else
// assumes there is a current row, so a list with nothing in it gets one blank
// row; empty reports that, for the caller to say what it wants to about it.
//
// Every way of changing what is listed ends here. Drawing the right pane for
// the new current row is the caller's.
func (o *Organizer) setRows(rows []Row) (empty bool) {
	o.rows = rows
	o.fc, o.fr, o.rowoff = 0, 0, 0
	empty = len(o.rows) == 0
	if empty {
		o.insertRow(0, "", true, false, false, BASE_DATE)
		o.rows[0].dirty = false
	}
	o.readRowsIntoBuffer()
	vim.SetCursorPosition(1, 0)
	o.bufferTick = o.vbuf.GetLastChangedTick()
	return empty
}

func (o *Organizer) getId() int {
	return o.rows[o.fr].id
}

func (o *Organizer) readRowsIntoBuffer() {
	// Create a new clean slice for titles
	numRows := len(o.rows)
	ss := make([]string, numRows)
	for i, row := range o.rows {
		ss[i] = row.title
	}

	if o.vbuf == nil {
		// This shouldn't happen, but handle it gracefully
		o.vbuf = vim.NewBuffer(0)
	}

	// Ensure we have at least an empty line in empty cases
	if len(ss) == 0 {
		ss = []string{""}
	}

	// Update the existing buffer's content
	o.vbuf.SetLines(0, -1, ss)
	vim.SetCurrentBuffer(o.vbuf)
}

func (o *Organizer) showMessage(format string, a ...interface{}) {
	fmt.Printf("\x1b[%d;%dH\x1b[1K\x1b[%d;1H", o.Screen.textLines+2+TOP_MARGIN, o.Screen.divider, o.Screen.textLines+2+TOP_MARGIN)
	str, _ := clipColumns(fmt.Sprintf(format, a...), 0, o.Screen.divider)
	fmt.Print(str)
}

func (o *Organizer) ShowMessage(loc Location, format string, a ...interface{}) {
	max_length := o.Screen.PositionMessage(loc)
	str := fmt.Sprintf(format, a...)
	// breakWord breaks strings (can be several words) into first segment
	// that fits taking into acount ANSI escape codes
	ss := breakWord(str, max_length)[0]
	str = ss + RESET
	fmt.Print(str)
}

// moveToContext puts the marked entries, or else the current one, in the
// context named input. moveToFolder is the same for a folder.
func (o *Organizer) moveToContext(input string) {
	var contextUUID string
	var ok bool
	//o.mode = NORMAL
	if contextUUID, ok = o.containerUUID(store.Context, input); !ok {
		o.ShowMessage(BL, "%s is not a valid context!", input)
		return
	}

	if len(o.marked_entries) > 0 {
		for id := range o.marked_entries {
			err := o.Store.SetContext(id, contextUUID)
			if err != nil {
				o.ShowMessage(BL, "Error updating context for entry %d: %v", id, err)
				return
			}
		}
		o.ShowMessage(BL, "Marked entries moved into context %q", input)
		return
	}
	id := o.rows[o.fr].id
	err := o.Store.SetContext(id, contextUUID)
	if err != nil {
		o.showMessage("Error updating context for entry %d: %v", id, err)
		return
	}
	o.ShowMessage(BL, "Moved current entry into context %q", input)
}

func (o *Organizer) moveToFolder(input string) {
	var folderUUID string
	var ok bool
	//o.mode = NORMAL
	if folderUUID, ok = o.containerUUID(store.Folder, input); !ok {
		o.ShowMessage(BL, "%s is not a valid folder!", input)
		return
	}

	if len(o.marked_entries) > 0 {
		for id := range o.marked_entries {
			err := o.Store.SetFolder(id, folderUUID)
			if err != nil {
				o.ShowMessage(BL, "Error updating folder for entry %d: %v", id, err)
				return
			}
		}
		o.ShowMessage(BL, "Marked entries moved into folder %q", input)
		return
	}
	id := o.rows[o.fr].id
	err := o.Store.SetFolder(id, folderUUID)
	if err != nil {
		o.showMessage("Error updating folder for entry %d: %v", id, err)
		return
	}
	o.ShowMessage(BL, "Moved current entry into folder %q", input)
}
