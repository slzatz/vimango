package main

import (
	"fmt"
	"sort"

	"github.com/slzatz/vimango/vim"
	"github.com/slzatz/vimango/vim/interfaces"
)

// RedrawScope represents the extent of organizer redraw work required after a key event.
type RedrawScope int

const (
	RedrawNone RedrawScope = iota
	RedrawPartial
	RedrawFull
)

type Organizer struct {
	mode      Mode
	last_mode Mode

	cx, cy    int //cursor x and y position
	fc, fr    int // file x and y position
	rowoff    int //the number of rows scrolled (aka number of top rows now off-screen
	altRowoff int //the number of rows scrolled in the right window (aka number of top rows now off-screen)
	coloff    int //the number of columns scrolled (aka number of left rows now off-screen

	rows                []Row
	altRows             []AltRow
	altFr               int
	filter              string
	sort                string
	sortPriority        bool
	command_line        string
	message             string
	note                []string // the preview
	command             string
	show_deleted        bool
	show_completed      bool
	view                View
	altView             View //int
	taskview            int
	current_task_id     int
	string_buffer       string
	marked_entries      map[int]struct{} // map instead of list makes toggling a row easier
	title_search_string string
	highlight           [2]int
	vbuf                interfaces.VimBuffer
	bufferTick          int
	//saveTick              int
	normalCmds            map[string]func(*Organizer)
	exCmds                map[string]func(*Organizer, int)
	commandRegistry       *CommandRegistry[func(*Organizer, int)]
	normalCommandRegistry *CommandRegistry[func(*Organizer)]
	filterList            []FilterNames
	tabCompletion         struct {
		list  []FilterNames
		index int
	}
	Database *Database
	Session  *Session
	Screen   *Screen
}

type FilterNames struct {
	Text string
	Char rune
}

func (a *App) setFilterList() []FilterNames {
	fnlist := []FilterNames{}
	filterMap := a.Database.contextList()
	for v, _ := range filterMap {
		fnlist = append(fnlist, FilterNames{Text: v, Char: 'c'})
	}
	filterMap = a.Database.folderList()
	for v, _ := range filterMap {
		fnlist = append(fnlist, FilterNames{Text: v, Char: 'f'})
	}
	sort.Slice(fnlist, func(i, j int) bool {
		return fnlist[i].Text < fnlist[j].Text
	})
	return fnlist
}

func (o *Organizer) FilterEntries(max int) {
	var err error
	o.rows, err = o.Database.filterEntries(o.taskview, o.filter, o.show_deleted, o.sort, o.sortPriority, max)
	if err != nil {
		o.showMessage("Error filtering entries: %v", err)
	}
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

	// If we're using Go implementation and have titles
	if vim.GetActiveImplementation() == vim.ImplGo {
		// Create a completely new buffer for maximum safety with Go implementation
		buf := vim.NewBuffer(0)

		// Handle the special case where we have no rows
		if len(ss) == 0 {
			// Always ensure at least an empty line for consistent behavior
			ss = []string{""}
		}

		// Set the new buffer's lines - use a defensive approach
		deepCopy := make([]string, len(ss))
		for i, line := range ss {
			deepCopy[i] = line // Deep copy to ensure no shared references
		}

		// Set the lines in the buffer
		buf.SetLines(0, -1, deepCopy)

		// Make it the current buffer and store the reference
		buf.SetCurrent()
		o.vbuf = buf
	} else {
		// Standard approach for C implementation
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
}

func (o *Organizer) showMessage(format string, a ...interface{}) {
	fmt.Printf("\x1b[%d;%dH\x1b[1K\x1b[%d;1H", o.Screen.textLines+2+TOP_MARGIN, o.Screen.divider, o.Screen.textLines+2+TOP_MARGIN)
	str := fmt.Sprintf(format, a...)
	if len(str) > o.Screen.divider {
		str = str[:o.Screen.divider]
	}
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
