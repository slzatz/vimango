package main

import (
	"strconv"
	"strings"

	"github.com/slzatz/vimango/vim"
)

func (o *Organizer) organizerProcessKey(c int) (redraw RedrawScope) {
	//o.ShowMessage(BR, "organizerProcessKey: %d -- mode %s", c, o.mode) //debug
	redraw = RedrawFull
	// Handle global escape key
	if c == '\x1b' {
		o.showMessage("")
		o.command = ""
		vim.SendKey("<esc>")
		//o.last_mode = o.mode // not sure this is necessary
		o.mode = NORMAL
		o.altRowoff = 0
		o.tabCompletion.index = 0
		o.tabCompletion.list = nil
		if o.view == TASK {
			o.displayNote()
		} else {
			o.Screen.eraseRightScreen()
			o.displayContainerInfo()
		}
		return
	}

	switch o.mode {
	case INSERT:
		redraw = o.InsertModeKeyHandler(c)
	case NORMAL, NORMAL_BUSY:
		redraw = o.NormalModeKeyHandler(c)
	case VISUAL:
		o.VisualModeKeyHandler(c)
		redraw = RedrawPartial
	case COMMAND_LINE:
		redraw = o.ExModeKeyHandler(c)
	case NAVIGATE_NOTICE, HELP:
		redraw = o.NavigateNoticeModeKeyHandler(c)
	case CONTAINER:
		redraw = o.NavigateContainerModeKeyHandler(c)
	default:
		return
	}
	return
}

func (o *Organizer) updateRowStatus() {
	row := &o.rows[o.fr]
	tick := o.vbuf.GetLastChangedTick()
	if tick > o.bufferTick {
		row.dirty = true
		o.bufferTick = tick
	}
}

func (o *Organizer) updateRowStatus2() bool {
	row := &o.rows[o.fr]
	tick := o.vbuf.GetLastChangedTick()
	if tick > o.bufferTick {
		row.dirty = true
		o.bufferTick = tick
		return true
	}
	return false
}

func (o *Organizer) InsertModeKeyHandler(c int) (redraw RedrawScope) {
	redraw = RedrawPartial
	if c == '\r' {
		o.writeTitle() // now updates ftsTitle if taskview == BY_FIND
		vim.SendKey("<esc>")
		o.mode = NORMAL
		row := &o.rows[o.fr]
		row.dirty = false
		o.bufferTick = o.vbuf.GetLastChangedTick()
		o.command = ""
		return RedrawPartial // was RedrawFull
	}

	sendToVim(c)

	s := o.vbuf.Lines()[o.fr]
	o.rows[o.fr].title = s
	pos := vim.GetCursorPosition()
	o.fc = pos[1]
	// need to prevent row from changing in INSERT mode; for instance, an when an up or down arrow is pressed
	if o.fr != pos[0]-1 {
		vim.SetCursorPosition(o.fr+1, o.fc)
	}
	o.updateRowStatus()
	return
}

func (o *Organizer) NormalModeKeyHandler(c int) (redraw RedrawScope) {
	//o.ShowMessage(BR, "key pressed: %d", c) ///debug
	redraw = RedrawNone
	if c == '\r' {

		o.command = ""
		row := &o.rows[o.fr]
		if row.dirty {
			o.writeTitle() // now updates ftsTitle if taskview == BY_FIND
			vim.SendKey("<esc>")
			row.dirty = false
			o.bufferTick = o.vbuf.GetLastChangedTick()
			redraw = RedrawPartial
			return
		}
	}

	// commands are letters or control characters, and always a single byte --
	// like the editor, the organizer has no prefixes and no leader
	if _, err := strconv.Atoi(string(rune(c))); err != nil {
		key := string(rune(c))
		if cmd, found := o.normalCmds[key]; found {
			//o.ShowMessage(BR, "key pressed: %T", cmd) ///debug
			cmd(o)
			switch key {
			case string(rune(ctrlKey('a'))), string(rune(ctrlKey('d'))), string(rune(ctrlKey('x'))), "m":
				redraw = RedrawPartial
			default:
				redraw = RedrawNone
			}
			o.command = ""
			vim.SendKey("<esc>")
			return
		}
	}

	// in NORMAL mode don't want ?, O, o, V, ctrl-V, J being passed to vim
	switch c {
	case 'J', 'V', ctrlKey('v'), 'o', 'O':
		o.showMessage("Ascii %d has no effect in Organizer NORMAL mode", c)
		return
	}

	// anything sent to vim should only require the active screen line to be redrawn
	// however if the row changes we need to erase the > from the previous row
	sendToVim(c)
	if o.updateRowStatus2() {
		redraw = RedrawPartial
	} else {
		redraw = RedrawNone
	}

	mode := vim.GetCurrentMode()
	if mode == vim.ModeOpPending { // like 4da
		return
	}

	if mode == vim.ModeCmdline { // COMMAND or SEARCH
		o.ShowMessage(BL, ":")
		vim.SendKey("<esc>") // park in NORMAL mode
		o.command_line = ""
		//o.last_mode = o.mode //Should probably be NORMAL
		o.mode = COMMAND_LINE
		o.tabCompletion.index = 0
		o.tabCompletion.list = nil
		return
	}

	prevRow := o.fr
	pos := vim.GetCursorPosition()
	o.fc = pos[1]
	newRow := pos[0] - 1
	// if move to a new row then draw task note preview or container info
	// and set cursor back to beginning of line and erase > from previous row
	if newRow != prevRow {
		o.fr = newRow
		o.fc = 0
		vim.SetCursorPosition(o.fr+1, 0)
		o.altRowoff = 0 // reset scroll in right window
		// need to erase > from previous row
		o.erasePreviousRowMarker(prevRow)
		if o.view == TASK {
			o.displayNote()
		} else {
			o.displayContainerInfo()
		}
	}
	s := o.vbuf.Lines()[o.fr]
	o.rows[o.fr].title = s
	//firstLine := vim.WindowGetTopLine() // doesn't seem to work
	//o.updateRowStatus()
	o.command = ""
	if mode == vim.ModeInsert && o.mode != INSERT {
		o.showMessage("\x1b[1m-- INSERT --\x1b[0m")
	}
	o.mode = modeMap[mode] // note that ModeCmdline => SEARCH (it is also COMMAND)
	if o.mode == VISUAL {
		o.setVisualHighlight(s, vim.GetVisualRange())
		redraw = RedrawPartial
	}
	o.ShowMessage(BL, "%s", s)
	//could put this redraw inside if o.mode == VISUAL and then check GetLastChangedTick
	//redraw = RedrawPartial
	return
}

func (o *Organizer) VisualModeKeyHandler(c int) {

	// in VISUAL mode don't want J, V, ctrl-V, : being passed to vim
	// in vim : triggers visual selection range :'<,'>
	switch c {
	case 'J', 'V', ctrlKey('v'), ':':
		o.showMessage("Ascii %d has no effect in Organizer VISUAL mode", c)
		return
	}

	sendToVim(c)
	o.bufferTick = o.vbuf.GetLastChangedTick()
	s := o.vbuf.Lines()[o.fr]
	o.rows[o.fr].title = s
	pos := vim.GetCursorPosition()
	o.fc = pos[1]
	// need to prevent row from changing in VISUAL mode; this catches j,k,g,G and arrow keys
	if o.fr != pos[0]-1 {
		vim.SetCursorPosition(o.fr+1, o.fc)
	}
	o.updateRowStatus()
	mode := vim.GetCurrentMode() // I think just a few possibilities - stay in VISUAL or something like 'x' switches to NORMAL and : to command
	o.mode = modeMap[mode]       // note that ModeCmdline => SEARCH (it is also COMMAND)
	o.command = ""
	o.setVisualHighlight(s, vim.GetVisualRange())
	o.showMessage("visual %s; %d %d", s, o.highlight[0], o.highlight[1])
}

// setVisualHighlight records the selection on title as a byte range [lo, hi).
// vim reports the anchor and the cursor, in either order, each as the byte
// offset of a character the selection includes, so the end steps over the whole
// rune rather than one byte.
func (o *Organizer) setVisualHighlight(title string, pos [2][2]int) {
	lo, hi := pos[0][1], pos[1][1]
	if hi < lo {
		lo, hi = hi, lo
	}
	o.highlight = [2]int{min(lo, len(title)), inclusiveEnd(title, hi)}
}

func (o *Organizer) ExModeKeyHandler(c int) (redraw RedrawScope) {
	redraw = RedrawNone
	switch c {

	case '\r':
		o.command = "" ////////there must be a better place!
		ex, cmd, found := o.parseEx(o.command_line)
		if found {
			cmd(o, ex)
			// a command that changes the mode has said so (:help, :new);
			// the rest are finished, and the command line with them
			if o.mode == COMMAND_LINE {
				o.mode = NORMAL
			}
			redraw = RedrawFull
			return
		}
		s := ex.name
		o.tabCompletion.index = 0
		o.tabCompletion.list = nil

		// Try to provide helpful suggestions using command registry
		if o.commandRegistry != nil {
			suggestions := o.commandRegistry.SuggestCommand(s)
			if len(suggestions) > 0 {
				o.showMessage("%sCommand '%s' not found. Did you mean: %s?%s", RED_BG, s, strings.Join(suggestions, ", "), RESET)
			} else {
				o.showMessage("%sCommand '%s' not found. Use ':help' to see available commands.%s", RED_BG, s, RESET)
			}
		} else {
			// Fallback if registry not available
			o.showMessage("%sNot a recognized command: %s%s", RED_BG, s, RESET)
		}
		//o.mode = o.last_mode
		o.mode = NORMAL
		return

	case '\t':
		pos := strings.LastIndex(o.command_line, " ")
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
			option := o.command_line[pos+1:]
			/* NOTE: there may be some commands we do want to exclude from tab completion
			cmd := o.command_line[:pos]
			if !(cmd == "o" || cmd == "cd" ) {
				return
			}
			*/
			for _, k := range o.filterList {
				if strings.HasPrefix(k.Text, option) {
					o.tabCompletion.list = append(o.tabCompletion.list, FilterNames{Text: k.Text, Char: k.Char})
				}
			}

			if len(o.tabCompletion.list) == 0 {
				return
			}
			//sort.Strings(tabCompletion.list)
		}

		o.command_line = o.command_line[:pos+1] + o.tabCompletion.list[o.tabCompletion.index].Text
		//o.showMessage(":%s (%c)", o.command_line, o.tabCompletion.list[o.tabCompletion.index].Char)
		o.showMessage(":%s", o.command_line)
		return

	case DEL_KEY, BACKSPACE:
		o.command_line = dropLastRune(o.command_line)

	default:
		// the organizer's command line is its own buffer, not a mirror of
		// vim's, so a reader code with no meaning here is simply dropped
		if !isSyntheticKey(c) {
			o.command_line += string(rune(c))
		}
	} // end switch c in COMMAND_LINE

	o.tabCompletion.index = 0
	o.tabCompletion.list = nil

	o.showMessage(":%s", o.command_line)
	return
}

// Used for viewing sync log and help
func (o *Organizer) NavigateNoticeModeKeyHandler(c int) RedrawScope {
	o.ShowMessage(BL, "NavigateRender mode")
	switch c {
	case ctrlKey('j'):
		o.scrollNoticeDown()
	case ctrlKey('k'):
		o.scrollNoticeUp()
	case HOME_KEY:
		o.scrollNoticeHome()
	case ' ', PAGE_DOWN: // half a page, as in the rendered note
		o.pageNoticeDown()
	case 'b', PAGE_UP:
		o.pageNoticeUp()
	case ':': // COMMAND or SEARCH
		o.ShowMessage(BL, ":")
		vim.SendKey("<esc>") // park in NORMAL mode
		o.command_line = ""
		//o.last_mode = o.mode //Should probably be NORMAL
		o.mode = COMMAND_LINE
		o.tabCompletion.index = 0
		o.tabCompletion.list = nil
	}
	return RedrawNone
}

func (o *Organizer) NavigateContainerModeKeyHandler(c int) RedrawScope {
	if _, err := strconv.Atoi(string(rune(c))); err == nil {
		o.command += string(rune(c))
	}
	o.ShowMessage(BR, "%s", o.command) //debug
	if c == '\r' {
		index, _ := strconv.Atoi(o.command)
		index-- // to convert to 0 based
		if o.altView == CONTEXT {
			o.moveToContext(o.containerList[index])
		}
		if o.altView == FOLDER {
			o.moveToFolder(o.containerList[index])
		}
		o.command = ""
		vim.SendKey("<esc>")
		o.mode = NORMAL
		o.altRowoff = 0
		o.displayNote()
		return RedrawFull // needed to redraw status line
	}
	switch c {
	case ctrlKey('j'):
		o.scrollNoticeDown()
	case ctrlKey('k'):
		o.scrollNoticeUp()
	case HOME_KEY:
		o.scrollNoticeHome()
	case ' ', PAGE_DOWN: // half a page, as in the rendered note
		o.pageNoticeDown()
	case 'b', PAGE_UP:
		o.pageNoticeUp()
	case ':': // COMMAND or SEARCH
		o.ShowMessage(BL, ":")
		vim.SendKey("<esc>") // park in NORMAL mode
		o.command_line = ""
		//o.last_mode = o.mode //Should probably be NORMAL
		o.mode = COMMAND_LINE
		o.tabCompletion.index = 0
		o.tabCompletion.list = nil
	}
	return RedrawNone
}
