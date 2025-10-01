package main

import (
	"strconv"
	"strings"

	"github.com/slzatz/vimango/vim"
)

func (o *Organizer) organizerProcessKey(c int) (redraw RedrawScope) {
	redraw = RedrawFull
	// Handle global escape key
	if c == '\x1b' {
		o.showMessage("")
		o.command = ""
		vim.SendKey("<esc>")
		o.last_mode = o.mode // not sure this is necessary
		o.mode = NORMAL
		o.altRowoff = 0
		o.tabCompletion.index = 0
		o.tabCompletion.list = nil
		o.Session.imagePreview = false
		if o.view == TASK {
			o.drawPreview()
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
	case NAVIGATE_REPORT:
		redraw = o.NavigateReportModeKeyHandler(c)
	default:
		return
	}
	return
}

func sendToVim(c int) {
	if z, found := termcodes[c]; found {
		vim.SendKey(z)
	} else {
		vim.SendInput(string(c))
	}
}

func (o *Organizer) updateRowStatus() {
	row := &o.rows[o.fr]
	tick := o.vbuf.GetLastChangedTick()
	if tick > o.bufferTick {
		row.dirty = true
		o.bufferTick = tick
	}
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

	// commands are letters or control characters
	if _, err := strconv.Atoi(string(c)); err != nil {
		o.command += string(c) //note currently all are single characters although future could use leader+chars

		if cmd, found := o.normalCmds[o.command]; found {
			cmd(o)
			switch o.command {
			case string(ctrlKey('a')), string(ctrlKey('d')), string(ctrlKey('x')), "m":
				redraw = RedrawPartial
			default:
				redraw = RedrawNone
			}
			/*
				redraw_map := map[string]struct{}{
					string(ctrlKey('a')): {}, // have retired starring for the moment
					string(ctrlKey('d')): {},
					string(ctrlKey('x')): {},
					"m":                  {},
				}
				if _, ok := redraw_map[o.command]; ok {
					redraw = RedrawPartial
				} else {
					redraw = RedrawNone
				}
			*/
			o.command = ""
			vim.SendKey("<esc>")
			return
		}
	}

	// in NORMAL mode don't want ? leader, O, o, V, ctrl-V, J being passed to vim
	switch c {
	case 'J', 'V', ctrlKey('v'), 'o', 'O':
		o.showMessage("Ascii %d has no effect in Organizer NORMAL mode", c)
		return
	}

	// anything sent to vim should only require the active screen line to be redrawn
	// however if the row changes we need to erase the > from the previous row
	sendToVim(c)

	mode := vim.GetCurrentMode()
	if mode == 4 { // OP_PENDING like 4da
		return
	}

	if mode == 8 { // COMMAND or SEARCH
		o.ShowMessage(BL, ":")
		vim.SendKey("<esc>") // park in NORMAL mode
		o.command_line = ""
		o.last_mode = o.mode //Should probably be NORMAL
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
			o.drawPreview()
		} else {
			o.displayContainerInfo()
		}
	}
	s := o.vbuf.Lines()[o.fr]
	o.rows[o.fr].title = s
	//firstLine := vim.WindowGetTopLine() // doesn't seem to work
	o.updateRowStatus()
	o.command = ""
	if mode == 16 && o.mode != INSERT {
		o.showMessage("\x1b[1m-- INSERT --\x1b[0m")
	}
	o.mode = modeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)
	if o.mode == VISUAL {
		pos := vim.GetVisualRange()
		o.highlight[1] = pos[1][1] + 1
		o.highlight[0] = pos[0][1]
	}
	o.ShowMessage(BL, "%s", s)
	//could put this redraw inside if o.mode == VISUAL and then check GetLastChangedTick
	redraw = RedrawPartial
	return
}

func (o *Organizer) VisualModeKeyHandler(c int) {
	/*
		if c == 'J' || c == 'V' || c == ctrlKey('v') || c == ':' {
			o.showMessage("Ascii %d has no effect in Organizer VISUAL mode", c)
			return
		}
	*/

	// in VISUAL mode don't want J, V, ctrl-V, : being passed to vim
	// in vim : triggers visual selection range :'<,'>
	switch c {
	case 'J', 'V', ctrlKey('v'), ':':
		o.showMessage("Ascii %d has no effect in Organizer VISUAL mode", c)
		return
	}

	sendToVim(c)
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
	o.mode = modeMap[mode]       //note that 8 => SEARCH (8 is also COMMAND)
	o.command = ""
	visPos := vim.GetVisualRange()
	o.highlight[1] = visPos[1][1] + 1
	o.highlight[0] = visPos[0][1]
	o.showMessage("visual %s; %d %d", s, o.highlight[0], o.highlight[1])
}

func (o *Organizer) ExModeKeyHandler(c int) (redraw RedrawScope) {
	redraw = RedrawNone
	switch c {

	case '\r':
		var cmd func(*Organizer, int)
		var found bool
		var s string
		pos := strings.LastIndexByte(o.command_line, ' ')
		if pos == -1 {
			s = o.command_line
		} else {
			s = o.command_line[:pos]
		}
		if cmd, found = o.exCmds[s]; found {
			cmd(o, pos)
			redraw = RedrawFull
			//o.mode = o.last_mode - could be NAVIGATE_RENDER
			return
		}
		// to catch find with more than one find term
		if !found && pos != -1 && strings.Count(o.command_line, " ") > 1 {
			pos := strings.IndexByte(o.command_line, ' ')
			if cmd, found = o.exCmds[o.command_line[:pos]]; found {
				// pass the position of the first space
				cmd(o, pos)
				redraw = RedrawFull
				o.mode = o.last_mode
				return
			}
		}
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
		o.mode = o.last_mode
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
	return
}

// Used for viewing sync log and help
func (o *Organizer) NavigateReportModeKeyHandler(c int) RedrawScope {
	o.ShowMessage(BL, "NavigateRender mode")
	switch c {
	//case ':':
	//	o.exCmd()
	case ctrlKey('j'), PAGE_DOWN:
		o.scrollReportDown()
	case ctrlKey('k'), PAGE_UP:
		o.scrollReportUp()
	}
	return RedrawNone
}
