package main

import (
	"fmt"
	"os/exec"
	"strings"
)

var navigation = map[int]struct{}{
	ARROW_UP:    z0,
	ARROW_DOWN:  z0,
	ARROW_LEFT:  z0,
	ARROW_RIGHT: z0,
	'h':         z0,
	'j':         z0,
	'k':         z0,
	'l':         z0,
	//PAGE_UP:     z0, // navigate right pane
	//PAGE_DOWN:   z0, // navigate right pane
}

var tabCompletion struct {
	idx  int
	list []string
}

func organizerProcessKey(c int) {

	switch org.mode {

	case NO_ROWS:
		switch c {
		case ':':
			exCmd()
		case '\x1b':
			org.command = ""
			org.repeat = 0
		case 'i', 'I', 'a', 'A', 's':
			org.insertRow(0, "", true, false, false, BASE_DATE)
			org.mode = INSERT
			org.command = ""
			org.repeat = 0
		}

	case FIND:
		switch c {
		case ':':
			exCmd()
		case ARROW_UP, ARROW_DOWN, PAGE_UP, PAGE_DOWN:
			org.moveCursor(c)
		default:
			org.mode = NORMAL
			org.command = ""
			organizerProcessKey(c)
		}

	case INSERT:
		switch c {
		case '\r': //also does in effect an escape into NORMAL mode
			org.writeTitle()
		case ARROW_UP, ARROW_DOWN, ARROW_LEFT, ARROW_RIGHT, PAGE_UP, PAGE_DOWN:
			org.moveCursor(c)
		case '\x1b':
			org.command = ""
			org.mode = NORMAL
			if org.fc > 0 {
				org.fc--
			}
			sess.showOrgMessage("")
		case HOME_KEY:
			org.fc = 0
		case END_KEY:
			org.fc = len(org.rows[org.fr].title)
		case BACKSPACE:
			org.backspace()
		case DEL_KEY:
			org.delChar()
		case '\t':
			//do nothing
		default:
			org.insertChar(c)
		}

	case NORMAL:

		// escape, return, ctrl-l [in one circumstance], PAGE_DOWN, PAGE_UP

		if c == '\x1b' {
			if org.view == TASK {
				sess.imagePreview = false
				org.drawPreview()
			}
			sess.showOrgMessage("")
			org.command = ""
			org.repeat = 0
			return
		}

		if c == ctrlKey('l') && org.last_mode == ADD_CHANGE_FILTER {
			org.mode = ADD_CHANGE_FILTER
			sess.eraseRightScreen()
		}

		if c == '\r' { //also does escape into NORMAL mode
			row := &org.rows[org.fr]
			if row.dirty {
				org.writeTitle()
				return
			}
			switch org.view {
			case CONTEXT:
				org.taskview = BY_CONTEXT
			case FOLDER:
				org.taskview = BY_FOLDER
			case KEYWORD:
				org.taskview = BY_KEYWORD
			default: //org.view == TASK
				return
			}

			org.filter = row.title
			sess.showOrgMessage("'%s' will be opened", org.filter)

			org.clearMarkedEntries()
			org.view = TASK
			org.mode = NORMAL // can be changed to NO_ROWS below
			org.fc, org.fr, org.rowoff = 0, 0, 0
			org.rows = filterEntries(org.taskview, org.filter, org.show_deleted, org.sort, MAX)
			if len(org.rows) == 0 {
				sess.showOrgMessage("No results were returned")
				org.mode = NO_ROWS
			}
			//org.drawPreviewWindow()
			org.drawPreview()
			return
		}

		// these navigate the preview in org.view == TASK
		if c == PAGE_UP {
			if org.altRowoff > org.textLines {
				org.altRowoff -= org.textLines
			} else {
				org.altRowoff = 0
			}
			org.drawPreview()
		}

		if c == PAGE_DOWN {
			if len(org.note) > org.altRowoff+org.textLines {
				if len(org.note) < org.altRowoff+2*org.textLines {
					org.altRowoff = len(org.note) - org.textLines
				} else {
					org.altRowoff += org.textLines
				}
			}
			org.drawPreview()
		}

		/*leading digit is a multiplier*/

		if (c > 47 && c < 58) && len(org.command) == 0 {

			if org.repeat == 0 && c == 48 {
			} else if org.repeat == 0 {
				org.repeat = c - 48
				return
			} else {
				org.repeat = org.repeat*10 + c - 48
				return
			}
		}

		if org.repeat == 0 {
			org.repeat = 1
		}

		/* ? needs to be before navigation  - if supporting commands line 'dh' */
		org.command += string(c)

		if cmd, found := n_lookup[org.command]; found {
			cmd()
			org.command = ""
			org.repeat = 0
			return
		}

		// any key sequence ending in a navigation key will
		// be true if not caught by above

		//arrow keys + h,j,k,l
		if _, found := navigation[c]; found {
			for j := 0; j < org.repeat; j++ {
				org.moveCursor(c)
			}
			org.command = ""
			org.repeat = 0
			return
		}

		// end of case NORMAL

	case REPLACE:
		if org.repeat == 0 {
			org.repeat = 1
		}
		if c == '\x1b' {
			org.command = ""
			org.repeat = 0
			org.mode = NORMAL
			return
		}

		for i := 0; i < org.repeat; i++ {
			org.delChar()
			org.insertChar(c)
		}

		org.repeat = 0
		org.command = ""
		org.mode = NORMAL

		return

	case ADD_CHANGE_FILTER:

		switch c {

		case '\x1b':
			org.mode = NORMAL
			org.last_mode = ADD_CHANGE_FILTER
			org.command = ""
			org.command_line = ""
			org.repeat = 0

		case ARROW_UP, ARROW_DOWN, 'j', 'k':
			org.moveAltCursor(c)

		case '\r':
			altRow := &org.altRows[org.altFr] //currently highlighted container row
			row := &org.rows[org.fr]          //currently highlighted entry row
			if len(org.marked_entries) == 0 {
				switch org.altView {
				case KEYWORD:
					addTaskKeyword(altRow.id, row.id, true)
					sess.showOrgMessage("Added keyword %s to current entry", altRow.title)
				case FOLDER:
					updateTaskFolder(altRow.title, row.id)
					sess.showOrgMessage("Current entry folder changed to %s", altRow.title)
				case CONTEXT:
					updateTaskContext(altRow.title, row.id)
					sess.showOrgMessage("Current entry had context changed to %s", altRow.title)
				}
			} else {
				for id := range org.marked_entries {
					switch org.altView {
					case KEYWORD:
						addTaskKeyword(altRow.id, id, true)
					case FOLDER:
						updateTaskFolder(altRow.title, id)
					case CONTEXT:
						updateTaskContext(altRow.title, id)

					}
					sess.showOrgMessage("Marked entries' %d changed/added to %s", org.altView, altRow.title)
				}
			}
		}

	case COMMAND_LINE:
		if c == '\x1b' {
			//org.mode = NORMAL
			org.mode = org.last_mode
			sess.showOrgMessage("")
			tabCompletion.idx = 0
			tabCompletion.list = nil
			return
		}

		if c == '\r' {
			pos := strings.LastIndex(org.command_line, " ")
			var s string
			if pos != -1 {
				s = org.command_line[:pos]
			} else {
				s = org.command_line
			}
			if cmd, found := cmd_lookup[s]; found {
				cmd(&org, pos)
				tabCompletion.idx = 0
				tabCompletion.list = nil
				return
			}

			sess.showOrgMessage("\x1b[41mNot a recognized command: %s\x1b[0m", s)
			org.mode = org.last_mode
			return
		}

		if c == '\t' {
			pos := strings.Index(org.command_line, " ")
			if tabCompletion.list == nil {
				sess.showEdMessage("tab")
				var s string
				if pos != -1 {
					s = org.command_line[:pos]
					cl := org.command_line
					var filterMap = make(map[string]int)
					if s == "o" || s == "oc" || s == "c" {
						filterMap = org.context_map
					} else if s == "of" || s == "f" {
						filterMap = org.folder_map
					} else if s == "ok" || s == "k" {
						filterMap = org.keywordMap
					} else {
						return
					}
					for k, _ := range filterMap {
						if strings.HasPrefix(k, cl[pos+1:]) {
							tabCompletion.list = append(tabCompletion.list, k)
						}
					}
				}
				if len(tabCompletion.list) == 0 {
					return // don't want to hit if below
				}
			} else {
				tabCompletion.idx++
				if tabCompletion.idx > len(tabCompletion.list)-1 {
					tabCompletion.idx = 0
				}
			}
			org.command_line = org.command_line[:pos+1] + tabCompletion.list[tabCompletion.idx]
			sess.showOrgMessage(":%s", org.command_line)
			return // don't want to hit if below
		}

		if c == DEL_KEY || c == BACKSPACE {
			length := len(org.command_line)
			if length > 0 {
				org.command_line = org.command_line[:length-1]
			}
		} else {
			org.command_line += string(c)
		}
		tabCompletion.idx = 0
		tabCompletion.list = nil

		sess.showOrgMessage(":%s", org.command_line)

		//end of case COMMAND_LINE

		//probably should be a org.view not org.mode but
		// for the moment this kluge works
	case SYNC_LOG:
		switch c {
		case ARROW_UP, 'k':
			if org.fr == 0 {
				return
			}
			org.fr--
			sess.eraseRightScreen()
			org.altRowoff = 0
			note := readSyncLog(org.rows[org.fr].id)
			note = generateWWString(note, org.totaleditorcols)
			org.note = strings.Split(note, "\n")
			org.drawPreviewWithoutImages()
		case ARROW_DOWN, 'j':
			if org.fr == len(org.rows)-1 {
				return
			}
			org.fr++
			sess.eraseRightScreen()
			org.altRowoff = 0
			note := readSyncLog(org.rows[org.fr].id)
			note = generateWWString(note, org.totaleditorcols)
			org.note = strings.Split(note, "\n")
			org.drawPreviewWithoutImages()
		case ':':
			sess.showOrgMessage(":")
			org.command_line = ""
			org.last_mode = org.mode
			org.mode = COMMAND_LINE

		// the two below only handle logs < 2x textLines
		case PAGE_DOWN:
			if len(org.note) > org.altRowoff+org.textLines {
				if len(org.note) < org.altRowoff+2*org.textLines {
					org.altRowoff = len(org.note) - org.textLines
				} else {
					org.altRowoff += org.textLines
				}
			}
			sess.eraseRightScreen()
			org.drawPreviewWithoutImages()

			//org.altRowoff++
			//sess.eraseRightScreen()
			//org.drawPreviewWithoutImages()
		case PAGE_UP:
			if org.altRowoff > org.textLines {
				org.altRowoff -= org.textLines
			} else {
				org.altRowoff = 0
			}
			sess.eraseRightScreen()
			org.drawPreviewWithoutImages()

			//if org.altRowoff > 0 {
			//	org.altRowoff--
			//}
			//sess.eraseRightScreen()
			//org.drawPreviewWithoutImages()
		case ctrlKey('d'):
			if len(org.marked_entries) == 0 {
				deleteSyncItem(org.rows[org.fr].id)
			} else {
				for id := range org.marked_entries {
					deleteSyncItem(id)
				}
			}
			org.log(0)
		case 'm':
			mark()
		}
	// The log that appears when you sync
	case PREVIEW_SYNC_LOG:
		switch c {
		case '\x1b':
			//org.mode = org.last_mode // pickup NO_ROWS
			org.mode = org.getMode()
			org.command_line = ""
			org.repeat = 0
			org.drawPreview()
		case ':':
			exCmd()
			/*
				sess.showOrgMessage(":")
				org.command_line = ""
				org.last_mode = org.mode
				org.mode = COMMAND_LINE
			*/

		//case ARROW_DOWN, 'j':
		case ctrlKey('j'):
			org.altRowoff++
			sess.eraseRightScreen()
			org.drawPreviewWithoutImages()
		//case ARROW_UP, 'k':
		case ctrlKey('k'):
			if org.altRowoff > 0 {
				org.altRowoff--
			}
			sess.eraseRightScreen()
			org.drawPreviewWithoutImages()
		case PAGE_DOWN:
			if len(org.note) > org.altRowoff+org.textLines {
				if len(org.note) < org.altRowoff+2*org.textLines {
					org.altRowoff = len(org.note) - org.textLines
				} else {
					org.altRowoff += org.textLines
				}
			}
			sess.eraseRightScreen()
			org.drawPreviewWithoutImages()
		case PAGE_UP:
			if org.altRowoff > org.textLines {
				org.altRowoff -= org.textLines
			} else {
				org.altRowoff = 0
			}
			sess.eraseRightScreen()
			org.drawPreviewWithoutImages()
		}
	case LINKS:
		if c == '\x1b' {
			org.command = ""
			org.mode = NORMAL
			return
		}

		if c < 49 || c > 57 {
			sess.showOrgMessage("That's not a number between 1 and 9")
			org.mode = NORMAL
			return
		}
		//if c > 48 && c < 58 {
		linkNum := c - 48
		var found string
		pre := fmt.Sprintf("[%d]", linkNum)
		for _, line := range org.note {

			//if strings.HasPrefix(line, pre) { //not at beginning of line
			idx := strings.Index(line, pre)
			if idx != -1 {
				found = line
				break
			}
		}
		if found == "" {
			sess.showOrgMessage("There is no [%d]", linkNum)
			org.mode = NORMAL
			return
		}
		//sess.showOrgMessage("length = %d, string = %s", len(found), strings.TrimSpace(found[80:140]))
		beg := strings.Index(found, "http")
		end := strings.Index(found, "\x1b\\")
		url := found[beg:end]
		sess.showOrgMessage("Opening %q", url)
		cmd := exec.Command("qutebrowser", url)
		err := cmd.Start()
		if err != nil {
			sess.showOrgMessage("Problem opening url: %v", err)
		}
		org.mode = NORMAL

	} // end switch o.mode
} // end func organizerProcessKey(c int)
