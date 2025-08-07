package main

import (
	"strconv"
	"strings"

	"github.com/slzatz/vimango/vim"
)

func (o *Organizer) organizerProcessKey(c int) {
	// Handle global escape key
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

	switch o.mode {
	case INSERT:
		o.InsertModeKeyHandler(c)
	case NORMAL:
		o.NormalModeKeyHandler(c)
	case VISUAL:
		o.VisualModeKeyHandler(c)
	case COMMAND_LINE:
		o.ExModeKeyHandler(c)
		/*
			case ADD_CHANGE_FILTER:
				handler = NewAddChangeFilterModeHandler(o)
			case SYNC_LOG:
				handler = NewSyncLogModeHandler(o)
		*/
	case PREVIEW_SYNC_LOG, PREVIEW_HELP:
		o.PreviewSyncLogModeKeyHandler(c)
	//case LINKS:
	//	handler = NewLinksModeHandler(o)
	default:
		return
	}
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

func (o *Organizer) InsertModeKeyHandler(c int) {
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
}

func (o *Organizer) NormalModeKeyHandler(c int) {

	/*
		if c == ctrlKey('l') && o.last_mode == ADD_CHANGE_FILTER {
			o.mode = ADD_CHANGE_FILTER
			o.Screen.eraseRightScreen()
		}
	*/

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
	}

	if cmd, found := o.normalCmds[o.command]; found {
		cmd(o)
		o.command = ""
		vim.SendKey("<esc>")
		return
	}

	// in NORMAL mode don't want ' ' (leader), 'O', 'V', 'o' ctrl-V (22)
	// being passed to vim
	//if c == int([]byte(leader)[0]) || c == 'O' || c == 'V' || c == ctrlKey('v') || c == 'o' || c == 'J'
	if _, ok := noopKeys[c]; ok {
		if c != int([]byte(leader)[0]) {
			o.showMessage("Ascii %d has no effect in Organizer NORMAL mode", c)
		}
		return
	}
	sendToVim(c)
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
	o.updateRowStatus()
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
}

func (o *Organizer) VisualModeKeyHandler(c int) {
	if c == 'j' || c == 'k' || c == 'J' || c == 'V' || c == ctrlKey('v') || c == 'g' || c == 'G' {
		o.showMessage("Ascii %d has no effect in Organizer VISUAL mode", c)
		return
	}
	sendToVim(c)
	s := o.vbuf.Lines()[o.fr]
	o.rows[o.fr].title = s
	pos := vim.GetCursorPosition()
	o.fc = pos[1]
	// need to prevent row from changing in INSERT mode; for instance, an when an up or down arrow is pressed
	// note can probably remove j,k,g,G from the above
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

func (o *Organizer) ExModeKeyHandler(c int) {
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
		}
		// to catch find with more than one find term
		if !found && pos != -1 && strings.Count(o.command_line, " ") > 1 {
			pos := strings.IndexByte(o.command_line, ' ')
			if cmd, found = o.exCmds[o.command_line[:pos]]; found {
				// pass the position of the first space
				cmd(o, pos)
			}
		}
		o.tabCompletion.index = 0
		o.tabCompletion.list = nil

		if !found {
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
		}
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
}

// case PREVIEW_SYNC_LOG and PREVIEW_HELP:
func (o *Organizer) PreviewSyncLogModeKeyHandler(c int) {
	switch c {
	case ':':
		o.exCmd()
	case ctrlKey('j'), PAGE_DOWN:
		o.scrollPreviewDown()
	case ctrlKey('k'), PAGE_UP:
		o.scrollPreviewUp()
	}
}
