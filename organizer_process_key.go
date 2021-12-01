package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/slzatz/vimango/vim"
)

var navigation = map[int]struct{}{
	ARROW_UP:   z0,
	ARROW_DOWN: z0,
	//ARROW_LEFT:  z0,
	//ARROW_RIGHT: z0,
	//'h':         z0,
	'j': z0,
	'k': z0,
	//'l':         z0,
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
		case 'i', 'I', 'a', 'A', 's':
			org.insertRow(0, "", true, false, false, BASE_DATE)
			vim.BufferSetLine(org.vbuf, []byte(""))
			vim.Execute("w")
			s := vim.BufferLinesS(org.vbuf)[0]
			vim.Key("<esc>")
			vim.Input("i")
			sess.showOrgMessage(s)
			org.mode = INSERT
			org.command = ""
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
		case '\r':
			org.writeTitle()
			vim.Key("<esc>")
			org.mode = NORMAL
			row := &org.rows[org.fr]
			row.dirty = false
		case ARROW_UP, ARROW_DOWN, PAGE_UP, PAGE_DOWN:
			//org.moveCursor(c)
			sess.showOrgMessage("Can't leave row while in INSERT mode")
		case '\x1b':
			org.command = ""
			org.mode = NORMAL
			vim.Key("<esc>")
			pos := vim.CursorGetPosition()
			org.fc = pos[1]
			sess.showOrgMessage("")
		default:
			if c < 32 {
				return
			}
			if z, found := termcodes[c]; found {
				vim.Input(z)
			} else {
				vim.Input(string(c))
			}
			s := vim.BufferLinesS(org.vbuf)[0]
			org.rows[org.fr].title = s
			pos := vim.CursorGetPosition()
			org.fc = pos[1]
			row := &org.rows[org.fr]
			row.dirty = vim.BufferGetModified(org.vbuf)
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
			vim.Key("<esc>")
			// shouldn't need to check cursor position
			return
		}

		if c == ctrlKey('l') && org.last_mode == ADD_CHANGE_FILTER {
			org.mode = ADD_CHANGE_FILTER
			sess.eraseRightScreen()
		}

		if c == '\r' {

			org.command = ""
			switch org.view {
			case TASK:
				org.writeTitle()
				vim.Execute("w")
				row := &org.rows[org.fr]
				row.dirty = false
				return
			case CONTEXT:
				org.taskview = BY_CONTEXT
			case FOLDER:
				org.taskview = BY_FOLDER
			case KEYWORD:
				org.taskview = BY_KEYWORD
			}

			row := &org.rows[org.fr]
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

		if _, found := navigation[c]; found {
			org.moveCursor(c)
			org.command = ""
			return
		}

		org.command += string(c)
		sess.showEdMessage(org.command)

		if cmd, found := n_lookup[org.command]; found {
			cmd()
			org.command = ""
			return
		}

		if c < 33 || c == 79 || c == 86 || c == 111 || c == 103 {
			return
		}

		// We don't want control characters
		// escape is dealt with above
		if z, found := termcodes[c]; found {
			vim.Input(z)
		} else {
			vim.Input(string(c))
		}
		//sess.showOrgMessage(string(c)) /// debug

		s := vim.BufferLinesS(org.vbuf)[0]
		org.rows[org.fr].title = s
		pos := vim.CursorGetPosition()
		org.fc = pos[1]
		row := &org.rows[org.fr]
		row.dirty = vim.BufferGetModified(org.vbuf)
		mode := vim.GetMode()
		org.command = ""

		//if mode.Blocking {
		if mode == 4 { //OP_PENDING
			return // don't draw rows - which calls v.BufferLines
		}
		// the only way to get into EX_COMMAND or SEARCH
		//if mode.Mode == "c" && p.mode != SEARCH { //note that "c" => SEARCH
		if org.mode == 16 && org.mode != INSERT {
			sess.showOrgMessage("\x1b[1m-- INSERT --\x1b[0m")
		}
		org.mode = newModeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)
		if org.mode == VISUAL {
			pos := vim.VisualGetRange()
			org.highlight[1] = pos[1][1] + 1
			org.highlight[0] = pos[0][1]
		}
		sess.showOrgMessage(s)

		// end of case NORMAL

	case VISUAL:

		// escape, return, ctrl-l [in one circumstance], PAGE_DOWN, PAGE_UP

		if c == '\x1b' {
			if org.view == TASK {
				sess.imagePreview = false
				org.drawPreview()
			}
			sess.showOrgMessage("")
			org.command = ""
			org.mode = NORMAL
			vim.Key("<esc>")
			// shouldn't need to check cursor position
			return
		}

		if c < 33 {
			return
		}

		// We don't want control characters
		// escape is dealt with above
		if z, found := termcodes[c]; found {
			vim.Input(z)
		} else {
			vim.Input(string(c))
		}
		//sess.showOrgMessage(string(c)) /// debug

		s := vim.BufferLinesS(org.vbuf)[0]
		org.rows[org.fr].title = s
		pos := vim.CursorGetPosition()
		org.fc = pos[1]
		row := &org.rows[org.fr]
		row.dirty = vim.BufferGetModified(org.vbuf)
		mode := vim.GetMode()
		org.mode = newModeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)
		org.command = ""
		visPos := vim.VisualGetRange()
		org.highlight[1] = visPos[1][1] + 1
		org.highlight[0] = visPos[0][1]
		sess.showOrgMessage("visual %s; %d %d", s, org.highlight[0], org.highlight[1])

		// end of case VISUAL

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
			//org.readTitleIntoBuffer() //not necessary here
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
