package main

import (
	"fmt"
	"strings"
)

func (o *Organizer) moveAltCursor(key int) {

	if len(o.altRows) == 0 {
		return
	}

	switch key {

	case ARROW_UP, 'k':
		if o.altFr > 0 {
			o.altFr--
		}

	case ARROW_DOWN, 'j':
		if o.altFr < len(o.altRows)-1 {
			o.altFr++
		}
	}
}

func (o *Organizer) getWordUnderCursor() {
	t := &o.rows[o.fr].title
	delimiters := " ,.;?:()[]{}&#"
	if strings.IndexAny(string((*t)[o.fc]), delimiters) != -1 {
		return
	}

	var beg int
	if o.fc != 0 {
		beg = strings.LastIndexAny((*t)[:o.fc], delimiters)
		if beg == -1 {
			beg = 0
		} else {
			beg++
		}
	}
	end := strings.IndexAny((*t)[o.fc:], delimiters)
	if end == -1 {
		end = len(*t) - 1
	} else {
		end = end + o.fc - 1
	}
	o.title_search_string = (*t)[beg : end+1]
}

func (o *Organizer) findNextWord() {
	var n int
	if o.fr < len(o.rows)-1 {
		n = o.fr + 1
	} else {
		n = 0
	}

	for {
		if n == len(o.rows) {
			n = 0
		}
		pos := strings.Index(o.rows[n].title, o.title_search_string)
		if pos == -1 {
			continue
		} else {
			o.fr = n
			o.fc = pos
			return
		}
		n++
	}
}

/*
func (o *Organizer) changeCase() {
	t := &o.rows[o.fr].title
	char := rune((*t)[o.fc])
	if unicode.IsLower(char) {
		char = unicode.ToUpper(char)
	} else {
		char = unicode.ToLower(char)
	}
	*t = (*t)[:o.fc] + string(char) + (*t)[o.fc+1:]
	o.rows[o.fr].dirty = true
}
*/

// func (o *Organizer) insertRow(at int, s string, star bool, deleted bool, completed bool, modified string) {
func (o *Organizer) insertRow(at int, s string, star bool, deleted bool, archived bool, sort string) {
	/* note since only inserting blank line at top, don't really need at, s and also don't need size_t*/

	var row Row
	o.rows = append(o.rows, row)     //make sure there is room to expand o.rows
	copy(o.rows[at+1:], o.rows[at:]) // move everything one over that will be to the right of new entry

	row.title = s
	row.id = -1
	row.star = star
	row.deleted = deleted
	row.archived = archived
	row.dirty = true
	//row.modified = modified
	row.sort = sort

	row.marked = false

	o.rows[at] = row
}

func (o *Organizer) scroll() {

	if len(o.rows) == 0 {
		o.fr, o.fc, o.coloff, o.rowoff, o.cx, o.cy = 0, 0, 0, 0, 0, 0
		return
	}
	titlecols := o.Screen.divider - TIME_COL_WIDTH - LEFT_MARGIN

	if o.fr > o.Screen.textLines+o.rowoff-1 {
		o.rowoff = o.fr - o.Screen.textLines + 1
	}

	if o.fr < o.rowoff {
		o.rowoff = o.fr
	}

	if o.fc > titlecols+o.coloff-1 {
		o.coloff = o.fc - titlecols + 1
	}

	if o.fc < o.coloff {
		o.coloff = o.fc
	}

	o.cx = o.fc - o.coloff
	o.cy = o.fr - o.rowoff
}

func (o *Organizer) altScroll() {

	if len(o.altRows) == 0 {
		return
	}
	prevRowoff := o.altRowoff

	if o.altFr > o.Screen.textLines+o.altRowoff-1 {
		o.altRowoff = o.altFr - o.Screen.textLines + 1
	}

	if o.altFr < o.altRowoff {
		o.altRowoff = o.altFr
	}

	if prevRowoff != o.altRowoff {
		o.Screen.eraseRightScreen()
	}
}

func (o *Organizer) insertChar(c int) {
	if len(o.rows) == 0 {
		return
	}

	t := &o.rows[o.fr].title
	if *t == "" {
		*t = string(c)
	} else {
		*t = (*t)[:o.fc] + string(c) + (*t)[o.fc:]
	}
	o.fc++
	o.rows[o.fr].dirty = true
}

func (o *Organizer) writeTitle() {
	row := &o.rows[o.fr]
	var msg string
	if !row.dirty {
		o.ShowMessage(BR, "Row has not been changed")
		return
	}

	if o.view == TASK {
		if row.id != -1 {
			err := o.Database.updateTitle(row)
			if err != nil {
				o.ShowMessage(BL, "Error updating title: id %d: %v", row.id, err)
			} else {
				o.ShowMessage(BL, "Updated title for id: %d", row.id)
			}
			// update fts title
			if o.taskview == BY_FIND {
				err := o.Database.updateFtsTitle(o.Session.fts_search_terms, row)
				if err != nil {
					row.ftsTitle = row.title
					o.ShowMessage(BL, "Error updating fts title: id %d: %v", row.id, err)
				} else {
					o.ShowMessage(BL, "Updated fts title for id: %d", row.id)
				}
			}
		} else {
			var context_tid, folder_tid int
			switch o.taskview {
			case BY_CONTEXT:
				context_tid, _ = o.Database.contextExists(o.filter)
				folder_tid = 1
			case BY_FOLDER:
				folder_tid, _ = o.Database.folderExists(o.filter)
				context_tid = 1
			default:
				context_tid = 1
				folder_tid = 1
			}
			err := o.Database.insertTitle(row, context_tid, folder_tid)
			if err != nil {
				o.ShowMessage(BL, "Error inserting new title id %d: %v", row.id, err)
			} else {
				o.ShowMessage(BL, "New (new) title written to db with id: %d", row.id)
			}
			if o.taskview == BY_FIND {
				err := o.Database.updateFtsTitle(o.Session.fts_search_terms, row)
				if err != nil {
					row.ftsTitle = row.title
					o.ShowMessage(BL, "Error updating fts title: id %d: %v", row.id, err)
				} else {
					o.ShowMessage(BL, "Updated fts title for id: %d", row.id)
				}
			}
		}
	} else {
		if !row.dirty {
			o.ShowMessage(BL, "Row has not been changed")
			return
		}
		err := o.Database.updateContainerTitle(row, o.view)
		if err != nil {
			msg = fmt.Sprintf("Error inserting into DB: %v", err)
		} else {
			msg = fmt.Sprintf("New (new) container written to db with id: %d", row.id)
		}
	}

	o.command = ""

	//sess.showOrgMessage("Updated id %d to %s (+fts if Entry)", row.id, truncate(row.title, 15))
	o.refreshScreen()
	o.ShowMessage(BR, msg)
}

func (o *Organizer) clearMarkedEntries() {
	for k, _ := range o.marked_entries {
		delete(o.marked_entries, k)
	}
}
