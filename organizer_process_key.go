package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/slzatz/vimango/vim"
)

func (o *Organizer) organizerProcessKey(c int) {

	if c == '\x1b' {
		o.showMessage("")
		o.command = ""
		vim.SendKey("<esc>")
		o.last_mode = o.mode // not sure this is necessary
		o.mode = NORMAL
		
		// Get cursor position - now should be preserved correctly by the buffer
		pos := vim.GetCursorPosition()
		o.fc = pos[1]
		o.fr = pos[0] - 1
		
		o.tabCompletion.index = 0
		o.tabCompletion.list = nil
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
			o.writeTitle() // now updates ftsTitle if taskview == BY_FIND
			vim.SendKey("<esc>")
			o.mode = NORMAL
			row := &o.rows[o.fr]
			row.dirty = false
			o.bufferTick = o.vbuf.GetLastChangedTick()
			o.command = ""
			return
		}

		if z, found := termcodes[c]; found {
			vim.SendKey(z)
		} else {
			vim.SendInput(string(c))
		}
		s := o.vbuf.Lines()[o.fr]
		o.rows[o.fr].title = s
		pos := vim.GetCursorPosition()
		o.fc = pos[1]
		// need to prevent row from changing in INSERT mode; for instance, an when an up or down arrow is pressed
		if o.fr != pos[0]-1 {
			vim.SetCursorPosition(o.fr+1, o.fc)
		}
		row := &o.rows[o.fr]
		tick := o.vbuf.GetLastChangedTick()
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
				o.writeTitle() // now updates ftsTitle if taskview == BY_FIND
				vim.SendKey("<esc>")
				row.dirty = false
				o.bufferTick = o.vbuf.GetLastChangedTick()
				return
			}
		}

		if _, err := strconv.Atoi(string(c)); err != nil {
			o.command += string(c)
			//sess.showEdMessage(org.command)
		}

		if cmd, found := o.normalCmds[o.command]; found {
			cmd(o)
			o.command = ""
			vim.SendKey("<esc>")
			return
		}

		// in NORMAL mode don't want ' ' (leader), 'O', 'V', 'o' ctrl-V (22)
		// being passed to vim
		//if c == int([]byte(leader)[0]) || c == 'O' || c == 'V' || c == ctrlKey('v') || c == 'o' || c == 'J' {
		if _, ok := noopKeys[c]; ok {
			if c != int([]byte(leader)[0]) {
				o.showMessage("Ascii %d has no effect in Organizer NORMAL mode", c)
			}
			return
		}

		// Send the keystroke to vim
		if z, found := termcodes[c]; found {
			vim.SendKey(z)
			o.ShowMessage(BR, "%s", z)
		} else {
			vim.SendInput(string(c))
		}

		pos := vim.GetCursorPosition()
		o.fc = pos[1]
		// if move to a new row then draw task note preview or container info
		// and set cursor back to beginning of line
		if o.fr != pos[0]-1 {
			o.fr = pos[0] - 1
			o.fc = 0
			vim.SetCursorPosition(o.fr+1, 0)
			o.altRowoff = 0
			if o.view == TASK {
				o.drawPreview()
			} else {
				o.displayContainerInfo()
			}
		}
		s := o.vbuf.Lines()[o.fr]
		o.rows[o.fr].title = s
		//firstLine := vim.WindowGetTopLine() // doesn't seem to work
		row := &o.rows[o.fr]
		tick := o.vbuf.GetLastChangedTick()
		if tick > o.bufferTick {
			row.dirty = true
			o.bufferTick = tick
		}
		mode := vim.GetCurrentMode()

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
			pos := vim.GetVisualRange()
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
			vim.SendKey(z)
		} else {
			vim.SendInput(string(c))
		}

		s := o.vbuf.Lines()[o.fr]
		o.rows[o.fr].title = s
		pos := vim.GetCursorPosition()
		o.fc = pos[1]
		// need to prevent row from changing in INSERT mode; for instance, an when an up or down arrow is pressed
		// note can probably remove j,k,g,G from the above
		if o.fr != pos[0]-1 {
			vim.SetCursorPosition(o.fr+1, o.fc)
		}
		row := &o.rows[o.fr]
		tick := o.vbuf.GetLastChangedTick()
		if tick > o.bufferTick {
			row.dirty = true
			o.bufferTick = tick
		}
		mode := vim.GetCurrentMode()  // I think just a few possibilities - stay in VISUAL or something like 'x' switches to NORMAL and : to command
		o.mode = modeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)
		o.command = ""
		visPos := vim.GetVisualRange()
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
				if cmd, found = o.exCmds[s]; found {
					cmd(o, pos)
				}
			} else {
				s = o.command_line[:pos]
				if cmd, found = o.exCmds[s]; found {
					cmd(o, pos)
				} else {
					pos := strings.Index(o.command_line, " ")
					s = o.command_line[:pos]
					if cmd, found = o.exCmds[s]; found {
						cmd(o, pos)
					}
				}
			}
			o.tabCompletion.index = 0
			o.tabCompletion.list = nil

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
			if o.tabCompletion.list != nil {
				o.tabCompletion.index++
				if o.tabCompletion.index > len(o.tabCompletion.list)-1 {
					o.tabCompletion.index = 0
				}
			} else {
				o.ShowMessage(BR, "tab")
				o.tabCompletion.index = 0
				cmd := o.command_line[:pos]
				option := o.command_line[pos+1:]
				/*
					var filterMap = make(map[string]struct{})
					switch cmd {
					case "o", "oc", "c", "cd":
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
				*/
				//for k, _ := range o.tabCompletion.List {
				if !(cmd == "o" || cmd == "cd") {
					return
				}
				for _, k := range o.filterList {
					if strings.HasPrefix(k.Text, option) {
						//tabCompletion.list = append(tabCompletion.list, k.Text)
						o.tabCompletion.list = append(o.tabCompletion.list, FilterNames{Text: k.Text, Char: k.Char})
					}
				}

				if len(o.tabCompletion.list) == 0 {
					return
				}
				//sort.Strings(tabCompletion.list)
			}

			o.command_line = o.command_line[:pos+1] + o.tabCompletion.list[o.tabCompletion.index].Text
			o.showMessage(":%s (%c)", o.command_line, o.tabCompletion.list[o.tabCompletion.index].Char)
			return

		case DEL_KEY, BACKSPACE:
			length := len(o.command_line)
			if length > 0 {
				o.command_line = o.command_line[:length-1]
			}

		default:
			o.command_line += string(c)
		} // end switch c in COMMAND_LINE

		o.tabCompletion.index = 0
		o.tabCompletion.list = nil

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
			o.mark()
		}

	case PREVIEW_SYNC_LOG:

		switch c {
		case ':':
			o.exCmd()
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
