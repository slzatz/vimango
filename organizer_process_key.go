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

func (o *Organizer) organizerProcessKey(c int) {

	if c == '\x1b' {
		o.showMessage("")
		o.command = ""
		vim.Key("<esc>")
		o.last_mode = o.mode // not sure this is necessary
		o.mode = NORMAL
		pos := vim.CursorGetPosition()
		o.fc = pos[1]
		o.fr = pos[0] - 1
		//org.fc = utf8.RuneCount(p.ss[org.fr][:pos[1]])
		tabCompletion.idx = 0
		tabCompletion.list = nil
		o.Session.imagePreview = false
		if o.view == TASK {
			o.drawPreview()
		}
		return
	}

	/* not sure this is necessary
	if c < 32 && !(c == 13 || c == ctrlKey('j') || c == ctrlKey('k') || c == ctrlKey('x') || c == ctrlKey('d')) {
		return
	}
	*/

	switch o.mode {

	case INSERT:

		if c == '\r' {
			o.writeTitle()
			vim.Key("<esc>")
			o.mode = NORMAL
			row := &o.rows[o.fr]
			row.dirty = false
			o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
			o.command = ""
			//o.showMessage("")
			return
		}

		if z, found := termcodes[c]; found {
			vim.Key(z)
		} else {
			vim.Input(string(c))
		}
		s := vim.BufferLines(o.vbuf)[o.fr]
		o.rows[o.fr].title = s
		pos := vim.CursorGetPosition()
		o.fc = pos[1]
		// need to prevent row from changing in INSERT mode; for instance, an when an up or down arrow is pressed
		if o.fr != pos[0]-1 {
			vim.CursorSetPosition(o.fr+1, o.fc)
		}
		row := &o.rows[o.fr]
		tick := vim.BufferGetLastChangedTick(o.vbuf)
		if tick > o.bufferTick {
			row.dirty = true
			o.bufferTick = tick
		}

	case NORMAL:

		if c == ctrlKey('l') && o.last_mode == ADD_CHANGE_FILTER {
			o.mode = ADD_CHANGE_FILTER
			o.Screen.eraseRightScreen()
		}

		if c == '\r' {

			o.command = ""
			row := &o.rows[o.fr]
			if row.dirty {
				o.writeTitle()
				vim.Key("<esc>")
				row.dirty = false
				o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
				return
			}
			// if not row.dirty nothing happens in TASK but if in a CONTAINER view open the entries with that container
			var tid int
			switch o.view {
			case TASK:
				return
			case CONTEXT:
				o.taskview = BY_CONTEXT
				tid, _ = o.Database.contextExists(row.title)
			case FOLDER:
				o.taskview = BY_FOLDER
				tid, _ = o.Database.folderExists(row.title)
			case KEYWORD:
				o.taskview = BY_KEYWORD
				// this guard to see if synced may not be necessary for keyword
				tid, _ = o.Database.keywordExists(row.title)
			}

			// if it's a new context|folder|keyword we can't filter tasks by it
			if tid < 1 {
				o.showMessage("You need to sync before you can use %q", row.title)
				return
			}
			o.filter = row.title
			o.ShowMessage(BL, "'%s' will be opened", o.filter)

			o.clearMarkedEntries()
			o.view = TASK
			o.fc, o.fr, o.rowoff = 0, 0, 0
			//o.rows = o.Database.filterEntries(o.taskview, o.filter, o.show_deleted, o.sort, o.sortPriority, MAX)
			o.FilterEntries(MAX)
			if len(o.rows) == 0 {
				o.insertRow(0, "", true, false, false, BASE_DATE)
				o.rows[0].dirty = false
				o.showMessage("No results were returned")
			}
			o.Session.imagePreview = false
			o.readRowsIntoBuffer()
			vim.CursorSetPosition(1, 0)
			o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
			o.drawPreview()
			return
		}

		if _, err := strconv.Atoi(string(c)); err != nil {
			o.command += string(c)
			//sess.showEdMessage(org.command)
		}

    if cmd, found := new_lookup[o.command]; found {
      cmd(o) 
			o.command = ""
			vim.Key("<esc>")
			return
		}

		if cmd, found := n_lookup[o.command]; found {
			 cmd()
			o.command = ""
			vim.Key("<esc>")
			return
		}

		// in NORMAL mode don't want ' ' (leader), 'O', 'V', 'o' ctrl-V (22)
		// being passed to vim
		if c == int([]byte(leader)[0]) || c == 'O' || c == 'V' || c == ctrlKey('v') || c == 'o' || c == 'J' {
			if c != int([]byte(leader)[0]) {
				o.showMessage("Ascii %d has no effect in Organizer NORMAL mode", c)
			}
			return
		}

		// Send the keystroke to vim
		if z, found := termcodes[c]; found {
			vim.Key(z)
			o.ShowMessage(BR, "%s", z)
		} else {
			vim.Input(string(c))
		}

		pos := vim.CursorGetPosition()
		o.fc = pos[1]
		// if move to a new row then draw task note preview or container info
		// and set cursor back to beginning of line
		if o.fr != pos[0]-1 {
			o.fr = pos[0] - 1
			o.fc = 0
			vim.CursorSetPosition(o.fr+1, 0)
			o.altRowoff = 0
			if o.view == TASK {
				o.drawPreview()
			} else {
				o.displayContainerInfo()
			}
		}
		s := vim.BufferLines(o.vbuf)[o.fr]
		o.rows[o.fr].title = s
		//firstLine := vim.WindowGetTopLine() // doesn't seem to work
		row := &o.rows[o.fr]
		tick := vim.BufferGetLastChangedTick(o.vbuf)
		if tick > o.bufferTick {
			row.dirty = true
			o.bufferTick = tick
		}
		mode := vim.GetMode()

		// OP_PENDING like 4da
		if mode == 4 {
			return
		}
		o.command = ""
		// the only way to get into EX_COMMAND or SEARCH
		//if mode.Mode == "c" && p.mode != SEARCH  //note that "c" => SEARCH
		//if mode == 8 && org.mode != SEARCH  //note that 8 => SEARCH
		if mode == 16 && o.mode != INSERT {
			o.showMessage("\x1b[1m-- INSERT --\x1b[0m")
		}
		o.mode = modeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)
		if o.mode == VISUAL {
			pos := vim.VisualGetRange()
			o.highlight[1] = pos[1][1] + 1
			o.highlight[0] = pos[0][1]
		}
		o.showMessage("%s", s)

		// end case NORMAL

	case VISUAL:

		if c == 'j' || c == 'k' || c == 'J' || c == 'V' || c == ctrlKey('v') || c == 'g' || c == 'G' {
			o.showMessage("Ascii %d has no effect in Organizer VISUAL mode", c)
			return
		}

		if z, found := termcodes[c]; found {
			vim.Key(z)
		} else {
			vim.Input(string(c))
		}

		s := vim.BufferLines(o.vbuf)[o.fr]
		o.rows[o.fr].title = s
		pos := vim.CursorGetPosition()
		o.fc = pos[1]
		// need to prevent row from changing in INSERT mode; for instance, an when an up or down arrow is pressed
		// note can probably remove j,k,g,G from the above
		if o.fr != pos[0]-1 {
			vim.CursorSetPosition(o.fr+1, o.fc)
		}
		row := &o.rows[o.fr]
		tick := vim.BufferGetLastChangedTick(o.vbuf)
		if tick > o.bufferTick {
			row.dirty = true
			o.bufferTick = tick
		}
		mode := vim.GetMode()    // I think just a few possibilities - stay in VISUAL or something like 'x' switches to NORMAL and : to command
		o.mode = modeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)
		o.command = ""
		visPos := vim.VisualGetRange()
		o.highlight[1] = visPos[1][1] + 1
		o.highlight[0] = visPos[0][1]
		o.showMessage("visual %s; %d %d", s, o.highlight[0], o.highlight[1])

		// end case VISUAL

	case COMMAND_LINE:

		switch c {

		case '\r':
			var cmd func(*Organizer, int)
			var found bool
			var s string
			pos := strings.LastIndex(o.command_line, " ")
			if pos == -1 {
				s = o.command_line
				if cmd, found = cmd_lookup[s]; found {
					cmd(o, pos)
				}
			} else {
				s = o.command_line[:pos]
				if cmd, found = cmd_lookup[s]; found {
					cmd(o, pos)
				} else {
					pos := strings.Index(o.command_line, " ")
					s = o.command_line[:pos]
					if cmd, found = cmd_lookup[s]; found {
						cmd(o, pos)
					}
				}
			}
			tabCompletion.idx = 0
			tabCompletion.list = nil

			if !found {
				o.showMessage("\x1b[41mNot a recognized command: %s\x1b[0m", s)
				o.showMessage("%sNot a recognized command: %s%s", RED_BG, s, RESET)
				o.mode = o.last_mode
			}
			return

		case '\t':
			pos := strings.Index(o.command_line, " ")
			if pos == -1 {
				return
			}
			if tabCompletion.list != nil {
				tabCompletion.idx++
				if tabCompletion.idx > len(tabCompletion.list)-1 {
					tabCompletion.idx = 0
				}
			} else {
				o.ShowMessage(BR, "tab")
				cmd := o.command_line[:pos]
				option := o.command_line[pos+1:]
				var filterMap = make(map[string]struct{})
				switch cmd {
				case "o", "oc", "c":
					filterMap = o.Database.contextList()
				case "of", "f":
					filterMap = o.Database.folderList()
				case "ok", "k":
					filterMap = o.Database.keywordList()
				case "sort":
					filterMap = sortColumns
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

			o.command_line = o.command_line[:pos+1] + tabCompletion.list[tabCompletion.idx]
			o.showMessage(":%s", o.command_line)
			return

		case DEL_KEY, BACKSPACE:
			length := len(o.command_line)
			if length > 0 {
				o.command_line = o.command_line[:length-1]
			}

		default:
			o.command_line += string(c)
		} // end switch c in COMMAND_LINE

		tabCompletion.idx = 0
		tabCompletion.list = nil

		o.showMessage(":%s", o.command_line)

		//end case COMMAND_LINE

	case ADD_CHANGE_FILTER:

		switch c {

		case ARROW_UP, ARROW_DOWN, 'j', 'k':
			o.moveAltCursor(c)

		case '\r':
			altRow := &o.altRows[o.altFr] //currently highlighted container row
			var tid int
			row := &o.rows[o.fr] //currently highlighted entry row
			switch o.altView {
			case KEYWORD:
				tid, _ = o.Database.keywordExists(altRow.title)
			case FOLDER:
				tid, _ = o.Database.folderExists(altRow.title)
			case CONTEXT:
				tid, _ = o.Database.contextExists(altRow.title)
			}
			if tid < 1 {
				o.showMessage("%q has not been synched yet - must do that before adding tasks", altRow.title)
				return
			}
			if len(o.marked_entries) == 0 {
				switch o.altView {
				case KEYWORD:
					o.Database.addTaskKeywordByTid(tid, row.id, true)
					o.showMessage("Added keyword %s to current entry", altRow.title)
				case FOLDER:
					o.Database.updateTaskFolderByTid(tid, row.id)
					o.showMessage("Current entry folder changed to %s", altRow.title)
				case CONTEXT:
					err := o.Database.updateTaskContextByTid(tid, row.id)
          if err != nil {
	          o.showMessage("Error updating context (updateTaskContextByTid) for entry %d to tid %d: %v", row.id, tid, err)
            return
          }
					o.showMessage("Current entry had context changed to %s", altRow.title)
				}
			} else {
				for id := range o.marked_entries {
					switch o.altView {
					case KEYWORD:
						o.Database.addTaskKeywordByTid(tid, id, true)
					case FOLDER:
						o.Database.updateTaskFolderByTid(tid, id)
					case CONTEXT:
						err := o.Database.updateTaskContextByTid(tid, id)
            if err != nil {
	            o.showMessage("Error updating context (updateTaskContextByTid) for entry %d to tid %d: %v", id, tid, err)
              return
            }
					}
					o.showMessage("Marked entries' %d changed/added to %s", o.altView, altRow.title)
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
			o.fc = pos[1]
			if o.fr != pos[0]-1 {
				o.fr = pos[0] - 1
				o.fc = 0
				vim.CursorSetPosition(o.fr+1, 0)
				o.drawPreview()
			}
		} else {
			o.mode = NORMAL
			o.command = ""
			o.organizerProcessKey(c)
		}
		//sess.showOrgMessage("Find: fr %d fc %d", org.fr, org.fc)

		//probably should be a org.view not org.mode but
		// for the moment this kluge works
	case SYNC_LOG:

		switch c {

		case ARROW_UP, 'k':
			if len(o.rows) == 0 {
				return
			}
			if o.fr == 0 {
				return
			}
			o.fr--
			o.Screen.eraseRightScreen()
			o.altRowoff = 0
			note := o.Database.readSyncLog(o.rows[o.fr].id)
			o.note = strings.Split(note, "\n")
			//note = generateWWString(note, org.totaleditorcols)
			o.drawPreviewWithoutImages()
		case ARROW_DOWN, 'j':
			if len(o.rows) == 0 {
				return
			}
			if o.fr == len(o.rows)-1 {
				return
			}
			o.fr++
			o.Screen.eraseRightScreen()
			o.altRowoff = 0
			note := o.Database.readSyncLog(o.rows[o.fr].id)
			//note = generateWWString(note, org.totaleditorcols)
			o.note = strings.Split(note, "\n")
			o.drawPreviewWithoutImages()
		case ':':
			o.showMessage(":")
			o.command_line = ""
			o.last_mode = o.mode
			o.mode = COMMAND_LINE

		// the two below only handle logs < 2x textLines
		case ctrlKey('j'):
			if len(o.rows) == 0 {
				return
			}
			if len(o.note) > o.altRowoff+o.Screen.textLines {
				if len(o.note) < o.altRowoff+2*o.Screen.textLines {
					o.altRowoff = len(o.note) - o.Screen.textLines
				} else {
					o.altRowoff += o.Screen.textLines
				}
			}
			o.Screen.eraseRightScreen()
			o.drawPreviewWithoutImages()

			//org.altRowoff++
			//sess.eraseRightScreen()
			//org.drawPreviewWithoutImages()
		//case PAGE_UP:
		case ctrlKey('k'):
			if len(o.rows) == 0 {
				return
			}
			if o.altRowoff > o.Screen.textLines {
				o.altRowoff -= o.Screen.textLines
			} else {
				o.altRowoff = 0
			}
			o.Screen.eraseRightScreen()
			o.drawPreviewWithoutImages()

			//if org.altRowoff > 0 {
			//	org.altRowoff--
			//}
			//sess.eraseRightScreen()
			//org.drawPreviewWithoutImages()
		case ctrlKey('d'):
			if len(o.rows) == 0 {
				return
			}
			if len(o.marked_entries) == 0 {
				o.Database.deleteSyncItem(o.rows[o.fr].id)
			} else {
				for id := range o.marked_entries {
				  o.Database.deleteSyncItem(id)
				}
			}
			o.log(0)
		case 'm':
			if len(o.rows) == 0 {
				return
			}
			mark()
		}

	case PREVIEW_SYNC_LOG:

		switch c {
		case ':':
			exCmd()
		case ctrlKey('j'):
			o.altRowoff++
			o.Screen.eraseRightScreen()
			o.drawPreviewWithoutImages()
		//case ARROW_UP, 'k':
		case ctrlKey('k'):
			if o.altRowoff > 0 {
				o.altRowoff--
			}
			o.Screen.eraseRightScreen()
			o.drawPreviewWithoutImages()
		case PAGE_DOWN:
			if len(o.note) > o.altRowoff+o.Screen.textLines {
				if len(o.note) < o.altRowoff+2*o.Screen.textLines {
					o.altRowoff = len(o.note) - o.Screen.textLines
				} else {
					o.altRowoff += o.Screen.textLines
				}
			}
			o.Screen.eraseRightScreen()
			o.drawPreviewWithoutImages()
		case PAGE_UP:
			if o.altRowoff > o.Screen.textLines {
				o.altRowoff -= o.Screen.textLines
			} else {
				o.altRowoff = 0
			}
			o.Screen.eraseRightScreen()
			o.drawPreviewWithoutImages()
		}

	case LINKS:

		if c < 49 || c > 57 {
			o.showMessage("That's not a number between 1 and 9")
			o.mode = NORMAL
			return
		}
		linkNum := c - 48
		var found string
		pre := fmt.Sprintf("[%d]", linkNum)
		for _, line := range o.note {

			idx := strings.Index(line, pre)
			if idx != -1 {
				found = line
				break
			}
		}
		if found == "" {
			o.showMessage("There is no [%d]", linkNum)
			o.mode = NORMAL
			return
		}
		beg := strings.Index(found, "http")
		end := strings.Index(found, "\x1b\\")
		url := found[beg:end]
		o.showMessage("Opening %q", url)
		cmd := exec.Command("qutebrowser", url)
		err := cmd.Start()
		if err != nil {
			o.showMessage("Problem opening url: %v", err)
		}
		o.mode = NORMAL

	} // end switch o.mode
} // end func organizerProcessKey(c int)
