package main

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/slzatz/vimango/vim"
)

var tabCompletion struct {
	idx  int
	list []string
}

func organizerProcessKey(c int) {

	if c == '\x1b' {
		sess.showOrgMessage("")
		org.command = ""
		vim.Key("<esc>")
		org.last_mode = org.mode // not sure this is necessary
		org.mode = NORMAL
		pos := vim.CursorGetPosition()
		org.fc = pos[1]
		org.fr = pos[0] - 1
		//org.fc = utf8.RuneCount(p.bb[org.fr][:pos[1]])
		tabCompletion.idx = 0
		tabCompletion.list = nil
		sess.imagePreview = false
		if org.view == TASK {
			org.drawPreview()
		}
		return
	}

	/* not sure this is necessary
	if c < 32 && !(c == 13 || c == ctrlKey('j') || c == ctrlKey('k') || c == ctrlKey('x') || c == ctrlKey('d')) {
		return
	}
	*/

	switch org.mode {

	case INSERT:

		if c == '\r' {
			org.writeTitle()
			vim.Key("<esc>")
			org.mode = NORMAL
			row := &org.rows[org.fr]
			row.dirty = false
			org.bufferTick = vim.BufferGetLastChangedTick(org.vbuf)
			org.command = ""
			return
		}

		if z, found := termcodes[c]; found {
			vim.Key(z)
		} else {
			vim.Input(string(c))
		}
		s := vim.BufferLinesS(org.vbuf)[org.fr]
		org.rows[org.fr].title = s
		pos := vim.CursorGetPosition()
		org.fc = pos[1]
		// need to prevent row from changing in INSERT mode; for instance, an when an up or down arrow is pressed
		if org.fr != pos[0]-1 {
			vim.CursorSetPosition(org.fr+1, org.fc)
		}
		row := &org.rows[org.fr]
		tick := vim.BufferGetLastChangedTick(org.vbuf)
		if tick > org.bufferTick {
			row.dirty = true
			org.bufferTick = tick
		}

	case NORMAL:

		if c == ctrlKey('l') && org.last_mode == ADD_CHANGE_FILTER {
			org.mode = ADD_CHANGE_FILTER
			sess.eraseRightScreen()
		}

		if c == '\r' {

			org.command = ""
			row := &org.rows[org.fr]
			if row.dirty {
				org.writeTitle()
				vim.Key("<esc>")
				row.dirty = false
				org.bufferTick = vim.BufferGetLastChangedTick(org.vbuf)
				return
			}
			// if not row.dirty nothing happens in TASK but if in a CONTAINER view open the entries with that container
			switch org.view {
			case TASK:
				return
			case CONTEXT:
				org.taskview = BY_CONTEXT
			case FOLDER:
				org.taskview = BY_FOLDER
			case KEYWORD:
				org.taskview = BY_KEYWORD
			}

			org.filter = row.title
			sess.showOrgMessage("'%s' will be opened", org.filter)

			org.clearMarkedEntries()
			org.view = TASK
			org.fc, org.fr, org.rowoff = 0, 0, 0
			org.rows = filterEntries(org.taskview, org.filter, org.show_deleted, org.sort, MAX)
			if len(org.rows) == 0 {
				org.insertRow(0, "", true, false, false, BASE_DATE)
				org.rows[0].dirty = false
				sess.showOrgMessage("No results were returned")
			}
			sess.imagePreview = false
			org.readRowsIntoBuffer()
			vim.CursorSetPosition(1, 0)
			org.bufferTick = vim.BufferGetLastChangedTick(org.vbuf)
			org.drawPreview()
			return
		}

		if _, err := strconv.Atoi(string(c)); err != nil {
			org.command += string(c)
			//sess.showEdMessage(org.command)
		}

		if cmd, found := n_lookup[org.command]; found {
			cmd()
			org.command = ""
			vim.Key("<esc>")
			return
		}

		// in NORMAL mode don't want ' ' (leader), 'O', 'V', 'o' ctrl-V (22)
		// being passed to vim
		if c == int([]byte(leader)[0]) || c == 'O' || c == 'V' || c == ctrlKey('v') || c == 'o' || c == 'J' {
			if c != int([]byte(leader)[0]) {
				sess.showOrgMessage("Ascii %d has no effect in Organizer NORMAL mode", c)
			}
			return
		}

		// Send the keystroke to vim
		if z, found := termcodes[c]; found {
			vim.Key(z)
			sess.showEdMessage("%s", z)
		} else {
			vim.Input(string(c))
		}

		pos := vim.CursorGetPosition()
		org.fc = pos[1]
		// if move to a new row then draw task note preview or container info
		// and set cursor back to beginning of line
		if org.fr != pos[0]-1 {
			org.fr = pos[0] - 1
			org.fc = 0
			vim.CursorSetPosition(org.fr+1, 0)
			org.altRowoff = 0
			if org.view == TASK {
				org.drawPreview()
			} else {
				sess.displayContainerInfo()
			}
		}
		s := vim.BufferLinesS(org.vbuf)[org.fr]
		org.rows[org.fr].title = s
		//firstLine := vim.WindowGetTopLine() // doesn't seem to work
		row := &org.rows[org.fr]
		tick := vim.BufferGetLastChangedTick(org.vbuf)
		if tick > org.bufferTick {
			row.dirty = true
			org.bufferTick = tick
		}
		mode := vim.GetMode()

		// OP_PENDING like 4da
		if mode == 4 {
			return
		}
		org.command = ""
		// the only way to get into EX_COMMAND or SEARCH
		//if mode.Mode == "c" && p.mode != SEARCH  //note that "c" => SEARCH
		//if mode == 8 && org.mode != SEARCH  //note that 8 => SEARCH
		if mode == 16 && org.mode != INSERT {
			sess.showOrgMessage("\x1b[1m-- INSERT --\x1b[0m")
		}
		org.mode = modeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)
		if org.mode == VISUAL {
			pos := vim.VisualGetRange()
			org.highlight[1] = pos[1][1] + 1
			org.highlight[0] = pos[0][1]
		}
		sess.showOrgMessage("%s", s)

		// end case NORMAL

	case VISUAL:

		if c == 'j' || c == 'k' || c == 'J' || c == 'V' || c == ctrlKey('v') || c == 'g' || c == 'G' {
			sess.showOrgMessage("Ascii %d has no effect in Organizer VISUAL mode", c)
			return
		}

		if z, found := termcodes[c]; found {
			vim.Key(z)
		} else {
			vim.Input(string(c))
		}

		s := vim.BufferLinesS(org.vbuf)[org.fr]
		org.rows[org.fr].title = s
		pos := vim.CursorGetPosition()
		org.fc = pos[1]
		// need to prevent row from changing in INSERT mode; for instance, an when an up or down arrow is pressed
		// note can probably remove j,k,g,G from the above
		if org.fr != pos[0]-1 {
			vim.CursorSetPosition(org.fr+1, org.fc)
		}
		row := &org.rows[org.fr]
		tick := vim.BufferGetLastChangedTick(org.vbuf)
		if tick > org.bufferTick {
			row.dirty = true
			org.bufferTick = tick
		}
		mode := vim.GetMode()    // I think just a few possibilities - stay in VISUAL or something like 'x' switches to NORMAL and : to command
		org.mode = modeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)
		org.command = ""
		visPos := vim.VisualGetRange()
		org.highlight[1] = visPos[1][1] + 1
		org.highlight[0] = visPos[0][1]
		sess.showOrgMessage("visual %s; %d %d", s, org.highlight[0], org.highlight[1])

		// end case VISUAL

	case COMMAND_LINE:

		switch c {

		case '\r':
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

		case '\t':
			pos := strings.Index(org.command_line, " ")
			if pos == -1 {
				return
			}
			if tabCompletion.list != nil {
				tabCompletion.idx++
				if tabCompletion.idx > len(tabCompletion.list)-1 {
					tabCompletion.idx = 0
				}
			} else {
				sess.showEdMessage("tab")
				cmd := org.command_line[:pos]
				option := org.command_line[pos+1:]
				var filterMap = make(map[string]int)
				switch cmd {
				case "o", "oc", "c":
					filterMap = org.contextMap
				case "of", "f":
					filterMap = org.folderMap
				case "ok", "k":
					filterMap = org.keywordMap
				case "sort":
					filterMap = map[string]int{"added": 0, "created": 0, "modified": 0}
				default:
					return
				}

				for k, _ := range filterMap {
					if strings.HasPrefix(k, option) {
						tabCompletion.list = append(tabCompletion.list, k)
					}
				}
				if len(tabCompletion.list) == 0 {
					return
				}
				sort.Strings(tabCompletion.list)
			}

			org.command_line = org.command_line[:pos+1] + tabCompletion.list[tabCompletion.idx]
			sess.showOrgMessage(":%s", org.command_line)
			return

		case DEL_KEY, BACKSPACE:
			length := len(org.command_line)
			if length > 0 {
				org.command_line = org.command_line[:length-1]
			}

		default:
			org.command_line += string(c)
		} // end switch c in COMMAND_LINE

		tabCompletion.idx = 0
		tabCompletion.list = nil

		sess.showOrgMessage(":%s", org.command_line)

		//end case COMMAND_LINE

	case ADD_CHANGE_FILTER:

		switch c {

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

	case FIND:
		if c == ':' {
			exCmd()
			return
		}

		if c >= '0' && c <= '9' {
			vim.Input(string(c))
			return
		}

		if _, found := navKeys[c]; found {
			if z, found := termcodes[c]; found {
				vim.Key(z)
			} else {
				vim.Input(string(c))
			}
			pos := vim.CursorGetPosition()
			org.fc = pos[1]
			if org.fr != pos[0]-1 {
				org.fr = pos[0] - 1
				org.fc = 0
				vim.CursorSetPosition(org.fr+1, 0)
				org.drawPreview()
			}
		} else {
			org.mode = NORMAL
			org.command = ""
			organizerProcessKey(c)
		}
		sess.showOrgMessage("Find: fr %d fc %d", org.fr, org.fc)

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

	case PREVIEW_SYNC_LOG:

		switch c {
		case ':':
			exCmd()
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

		if c < 49 || c > 57 {
			sess.showOrgMessage("That's not a number between 1 and 9")
			org.mode = NORMAL
			return
		}
		linkNum := c - 48
		var found string
		pre := fmt.Sprintf("[%d]", linkNum)
		for _, line := range org.note {

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
