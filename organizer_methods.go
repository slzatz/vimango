package main

import (
  "fmt"
	"strings"
)

func (o *Organizer) getMode() Mode {
	if len(o.rows) > 0 {
		return NORMAL
	} else {
		return NO_ROWS
	}
}

func (o *Organizer) moveAltCursor(key int) {

	if len(o.altRows) == 0 {
		return
	}

	//o.fc = 0

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

//func (o *Organizer) insertRow(at int, s string, star bool, deleted bool, completed bool, modified string) {
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

	if o.mode == ADD_CHANGE_FILTER {
		o.altScroll()
		return
	}

	if len(o.rows) == 0 {
		o.fr, o.fc, o.coloff, o.rowoff, o.cx, o.cy = 0, 0, 0, 0, 0, 0
		return
	}
	titlecols := sess.divider - TIME_COL_WIDTH - LEFT_MARGIN

	if o.fr > sess.textLines+o.rowoff-1 {
		o.rowoff = o.fr - sess.textLines + 1
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

	if o.altFr > sess.textLines+o.altRowoff-1 {
		o.altRowoff = o.altFr - sess.textLines + 1
	}

	if o.altFr < o.altRowoff {
		o.altRowoff = o.altFr
	}

	if prevRowoff != o.altRowoff {
		sess.eraseRightScreen()
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
		sess.showOrgMessage("Row has not been changed")
		return
	}

	if o.view == TASK {
		err := o.Database.updateTitle(row)
		if err != nil {
      msg = fmt.Sprintf("Error inserting into DB: %v", err)
		} else {
      msg = fmt.Sprintf("New (new) entry written to db with id: %d", row.id)
    }
	} else {
    if !row.dirty {
      o.AppUI.showMessage(BL, "Row has not been changed")
      return
      }
		err := o.Database.updateContainerTitle(row)
		if err != nil {
      msg = fmt.Sprintf("Error inserting into DB: %v", err)
		} else {
      msg = fmt.Sprintf("New (new) container written to db with id: %d", row.id)
    }
	}

	o.command = ""

	//sess.showOrgMessage("Updated id %d to %s (+fts if Entry)", row.id, truncate(row.title, 15))
	o.refreshScreen()
	sess.showOrgMessage(msg)
}

func (o *Organizer) clearMarkedEntries() {
	for k, _ := range o.marked_entries {
		delete(o.marked_entries, k)
	}
}

