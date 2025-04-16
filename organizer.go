package main

import (
  "fmt"

  "github.com/slzatz/vimango/vim"
)

type Organizer struct {
	mode      Mode
	last_mode Mode

	cx, cy    int //cursor x and y position
	fc, fr    int // file x and y position
	rowoff    int //the number of rows scrolled (aka number of top rows now off-screen
	altRowoff int
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
	vbuf                vim.Buffer
	bufferTick          int
  Database            *Database
	AppUI               *Session
  //*Database
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
	var ss []string
	for _, row := range o.rows {
		ss = append(ss, row.title)
	}
	vim.BufferSetLines(o.vbuf, 0, -1, ss, len(ss))
	vim.BufferSetCurrent(o.vbuf)
}

func (o *Organizer) showMessage(format string, a ...interface{}) {
	fmt.Printf("\x1b[%d;%dH\x1b[1K\x1b[%d;1H", o.AppUI.textLines+2+TOP_MARGIN, o.AppUI.divider, o.AppUI.textLines+2+TOP_MARGIN)
	str := fmt.Sprintf(format, a...)
	if len(str) > o.AppUI.divider {
		str = str[:o.AppUI.divider]
	}
	fmt.Print(str)
}
