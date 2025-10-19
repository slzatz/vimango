package main

import (
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/slzatz/vimango/vim"
)

// note that bool returned is whether to redraw
func (e *Editor) editorProcessKey(c int) (redraw bool) {

	//No matter what mode you are in an escape puts you in NORMAL mode
	if c == '\x1b' {
		vim.SendKey("<esc>")
		e.ss = e.vbuf.Lines() /// for commands like 4i
		//e.mode = NORMAL_BUSY
		prevMode := e.mode
		mode := vim.GetCurrentMode() ////////
		e.mode = modeMap[mode]
		e.ShowMessage(BL, "vim mode: %d | e.mode: %s", mode, e.mode) //////Debug
		e.command = ""
		e.command_line = ""
		pos := vim.GetCursorPosition() //set screen cx and cy from pos
		e.fr = pos[0] - 1
		e.fc = utf8.RuneCountInString(e.ss[e.fr][:pos[1]])
		e.ShowMessage(BR, "")
		//return false
		if prevMode == VISUAL { //need to redraw to remove highlight
			return true
		} else {
			return false
		}
	}

	// if exit is false, doesn't matter if redraw is false since redraw will be determined
	// by the mode processing below
	//	skip := false
	var exit bool

	// the switch is basically pre-processing before sending key to vim
	// Below don't  handle NORMAL_BUSY because just want that to drop through
	switch e.mode {
	case NORMAL, NORMAL_BUSY: //NORMAL only occurs after INSERT is escaped
		redraw, exit = e.NormalModeKeyHandler(c)
	case VISUAL:
		redraw, exit = e.VisualModeKeyHandler(c)
	case EX_COMMAND:
		redraw, exit = e.ExModeKeyHandler(c)
	case SEARCH:
		redraw, exit = e.SearchModeKeyHandler(c)
	case PREVIEW:
		redraw, exit = e.PreviewModeKeyHandler(c)
	case VIEW_LOG:
		redraw, exit = e.ViewLogModeKeyHandler(c)
	}

	// if exit true, don't process key any further
	if exit {
		return
	}
	// Process the key
	if z, found := termcodes[c]; found {
		vim.SendKey(z)
	} else {
		vim.SendInput(string(c))
	}

	tick := e.vbuf.GetLastChangedTick()
	if tick > e.bufferTick {
		e.bufferTick = tick
		redraw = true
	} else {
		redraw = false
	}
	mode := vim.GetCurrentMode()
	//submode := vim.GetSubMode() added this 9-22-25 but really only for obscure submodes

	switch mode {
	//case 1: //NORMAL
	//	e.mode = NORMAL
	case 4: //OP_PENDING delete, change, yank, etc
		e.mode = PENDING
		e.ShowMessage(BL, "vim mode: %d | e.mode: %s | char: %s", mode, e.mode, string(c)) //////Debug
		return false
	case 8: //SEARCH and EX_COMMAND
		// Note: will not hit this case if we are in e.mode == Ex_COMMAND because we
		// park vim in NORMAL mode and don't feed it keys
		// note that if e.mode has been set to SEARCH, this code does nothing
		if e.mode != SEARCH {
			e.command_line = ""
			e.command = ""
			if c == ':' {
				e.mode = EX_COMMAND
				vim.SendKey("<esc>") // park in NORMAL mode
				e.ShowMessage(BR, ":")
			} else {
				e.mode = SEARCH
				e.searchPrefix = string(c)
				e.ShowMessage(BR, e.searchPrefix)
			}
			e.ShowMessage(BL, "vim mode: %d | e.mode: %s | char: %s", mode, e.mode, string(c)) //////Debug
			return false
		}
	case 16: //INSERT
		if e.mode != INSERT {
			e.mode = INSERT
			e.ShowMessage(BR, "\x1b[1m-- INSERT --\x1b[0m")
		}
		//redraw = true
	case 2: //VISUAL_MODE
		vmode := vim.GetVisualType()
		e.vmode = visualModeMap[vmode]
		e.mode = VISUAL
		//e.ShowMessage(BR, "Current visualType from vim: %d, visual test: %v", vmode, mode == 118)
		e.highlightInfo()
		//e.ShowMessage(BL, "vim mode: %d | vmode: %s | e.mode: %s", mode, e.vmode, e.mode) //////Debug
		redraw = true
		//case 257: //NORMAL_BUSY
		//	e.mode = NORMAL_BUSY
	}
	//default:
	if m, ok := modeMap[mode]; ok { //note that 8 => SEARCH (8 is also COMMAND)
		e.mode = m
	} else {
		e.mode = OTHER // not sure this ever happens
	}
	//} // was end of switch mode
	e.ShowMessage(BL, "vim mode: %d | e.mode: %s | char: %s", mode, e.mode, string(c)) //////Debug
	//below is done for everything except SEARCH, EX_COMMAND and OP_PENDING
	e.ss = e.vbuf.Lines()
	// Add safety checks to prevent panic with empty buffers
	if len(e.ss) == 0 {
		e.ss = []string{""}
	}

	pos := vim.GetCursorPosition() //set screen cx and cy from pos
	e.fr = pos[0] - 1

	// Ensure fr is in bounds
	if e.fr < 0 {
		e.fr = 0
	}
	if e.fr >= len(e.ss) {
		e.fr = len(e.ss) - 1
	}

	// Ensure pos[1] (column) is valid
	if pos[1] > len(e.ss[e.fr]) {
		pos[1] = len(e.ss[e.fr])
	}

	e.fc = utf8.RuneCountInString(e.ss[e.fr][:pos[1]])

	return
}

// case PREVIEW:
func (e *Editor) PreviewModeKeyHandler(c int) (redraw, exit bool) {
	switch c {
	case ARROW_DOWN, ctrlKey('j'):
		e.previewLineOffset++
	case ARROW_UP, ctrlKey('k'):
		if e.previewLineOffset > 0 {
			e.previewLineOffset--
		}
	}
	e.drawPreview()
	return false, true
}

// case VIEW_LOG:
func (e *Editor) ViewLogModeKeyHandler(c int) (redraw, skip bool) {
	switch c {
	case PAGE_DOWN, ARROW_DOWN, 'j':
		e.previewLineOffset++
		e.drawOverlay()
		return false, true
	case PAGE_UP, ARROW_UP, 'k':
		if e.previewLineOffset > 0 {
			e.previewLineOffset--
			e.drawOverlay()
			redraw = false
			skip = true
			return false, true
		}
	}
	if c == DEL_KEY || c == BACKSPACE {
		if len(e.command_line) > 0 {
			e.command_line = e.command_line[:len(e.command_line)-1]
		}
	} else {
		e.command_line += string(c)
	}
	return false, true
}

// case NORMAL, OTHER (257):
// note that Crtl-A and Ctrl-X are passed through and perform their usual weird function of incrementing and decrementing the number under the cursor and Ctrl-R also works to undo the last undone change.  All other vim built-in Ctrl commands appear to do nothing.
func (e *Editor) NormalModeKeyHandler(c int) (redraw, skip bool) {
	//leader := ' ' //vim.GetLeaderKey()
	if c == ' ' { //should become if c == leader
		e.command = string(c)
		return false, true
	}
	e.command += string(c)

	if len(e.command) > 0 {
		if e.command[0] != ' ' { //leader
			e.command = string(c)
		}
	}

	e.ShowMessage(BR, "e.command = %q", e.command) //Debug
	if cmd, found := e.normalCmds[e.command]; found {
		cmd(e, c)
		vim.SendKey("<esc>")
		// below is kludge becuse moveLeft and moveRight move you out of current editor
		if e.command == "\x08" || e.command == "\x0c" { //moveLeft or moveRight
			e.command = ""
			return true, true
		}
		e.command = ""
		e.ss = e.vbuf.Lines()
		pos := vim.GetCursorPosition() //set screen cx and cy from pos
		e.fr = pos[0] - 1
		e.fc = utf8.RuneCountInString(e.ss[e.fr][:pos[1]])
		redraw = true
		if e.mode == PREVIEW {
			redraw = false
		}
		return redraw, true
	} else {
		if e.command[0] == ' ' {
			return false, true // don't process key
		} else {
			return false, false // have vim process key
		}
	}
}

// case VISUAL:
func (e *Editor) VisualModeKeyHandler(c int) (redraw, skip bool) {
	// Special commands in visual mode to do markdown decoration: ctrl-b, e, i
	if strings.IndexAny(string(c), "\x02\x05\x09") != -1 { // this should define commmands like normalCmds, ie visualCmds
		e.decorateWordVisual(c)
		vim.SendKey("<esc>")
		e.mode = NORMAL
		e.command = ""
		e.ss = e.vbuf.Lines()
		pos := vim.GetCursorPosition() //set screen cx and cy from pos
		e.fr = pos[0] - 1
		e.fc = utf8.RuneCountInString(e.ss[e.fr][:pos[1]])
		return true, true
	}
	return false, false
}

// case EX_COMMAND:
func (e *Editor) ExModeKeyHandler(c int) (redraw, skip bool) {
	if c == '\r' {
		// Index doesn't work for vert resize
		// and LastIndex doesn't work for run
		// so total kluge below
		//if e.command_line[0] == '%'
		//if strings.Index(e.command_line, "s/") != -1

		// we want libvim to handle the following Ex-Commands:
		use_vim := []string{"s/", "%s/", "g/", "g!/", "v/"}
		for _, p := range use_vim {
			if strings.HasPrefix(e.command_line, p) {
				if strings.HasSuffix(e.command_line, "/c") {
					e.ShowMessage(BR, "We don't support [c]onfirm")
					e.mode = NORMAL
					return false, true
				}

				vim.SendInput(":" + e.command_line + "\r")
				e.mode = NORMAL
				e.command = ""
				e.ss = e.vbuf.Lines()
				pos := vim.GetCursorPosition() //set screen cx and cy from pos
				e.fr = pos[0] - 1
				e.fc = utf8.RuneCountInString(e.ss[e.fr][:pos[1]])
				e.ShowMessage(BL, "search and replace: %s", e.command_line)
				return true, true
			}
		}

		var pos int
		var cmd string
		if strings.HasPrefix(e.command_line, "vert") {
			pos = strings.LastIndex(e.command_line, " ")
		} else {
			pos = strings.Index(e.command_line, " ")
		}
		if pos != -1 {
			cmd = e.command_line[:pos]
		} else {
			cmd = e.command_line
		}

		//if cmd0, found := e_lookup_C[cmd]; found
		if cmd0, found := e.exCmds[cmd]; found {
			cmd0(e)
			e.command_line = ""
			e.mode = NORMAL
			e.tabCompletion.index = 0
			e.tabCompletion.list = nil
			return false, true
		}

		// Try to provide helpful suggestions using command registry
		if e.commandRegistry != nil {
			suggestions := e.commandRegistry.SuggestCommand(cmd)
			if len(suggestions) > 0 {
				e.ShowMessage(BR, "\x1b[41mCommand '%s' not found. Did you mean: %s?\x1b[0m", cmd, strings.Join(suggestions, ", "))
			} else {
				e.ShowMessage(BR, "\x1b[41mCommand '%s' not found. Use ':help' to see available commands.\x1b[0m", cmd)
			}
		} else {
			// Fallback if registry not available
			e.ShowMessage(BR, "\x1b[41mNot an editor command: %s\x1b[0m", cmd)
		}
		e.mode = NORMAL
		e.command_line = ""
		return false, true
	} //end 'r'

	if c == '\t' {
		pos := strings.Index(e.command_line, " ")
		if e.tabCompletion.list == nil {
			e.ShowMessage(BL, "tab")
			var s string
			if pos != -1 {
				s = e.command_line[pos+1:]
				//cl := p.command_line
				dir := filepath.Dir(s)
				if dir == "~" {
					usr, _ := user.Current()
					dir = usr.HomeDir
				} else if strings.HasPrefix(dir, "~/") {
					usr, _ := user.Current()
					dir = filepath.Join(usr.HomeDir, dir[2:])
				}

				partial := filepath.Base(s)
				paths, _ := ioutil.ReadDir(dir)
				e.ShowMessage(BL, "dir: %s  base: %s", dir, partial)

				for _, path := range paths {
					if strings.HasPrefix(path.Name(), partial) {
						e.tabCompletion.list = append(e.tabCompletion.list, filepath.Join(dir, path.Name()))
					}
				}
			}
			if len(e.tabCompletion.list) == 0 {
				return false, true
			}
		} else {
			e.tabCompletion.index++
			if e.tabCompletion.index > len(e.tabCompletion.list)-1 {
				e.tabCompletion.index = 0
			}
		}
		e.command_line = e.command_line[:pos+1] + e.tabCompletion.list[e.tabCompletion.index]
		e.ShowMessage(BR, ":%s", e.command_line)
		return false, true
	}

	// process the key typed in COMMAND_LINE mode
	if c == DEL_KEY || c == BACKSPACE {
		if len(e.command_line) > 0 {
			e.command_line = e.command_line[:len(e.command_line)-1]
		}
	} else {
		e.command_line += string(c)
	}

	e.tabCompletion.index = 0
	e.tabCompletion.list = nil

	e.ShowMessage(BR, ":%s", e.command_line)
	return false, true
}

// case SEARCH:
func (e *Editor) SearchModeKeyHandler(c int) (bool, bool) {
	// Below is purely for displaying command line correctly
	// vim is actually handling the keystrokes whether we do this or not
	if c == DEL_KEY || c == BACKSPACE {
		if len(e.command_line) > 0 {
			e.command_line = e.command_line[:len(e.command_line)-1]
		}
	} else {
		e.command_line += string(c)
	}
	e.ShowMessage(BR, "%s%s", e.searchPrefix, e.command_line)
	return false, false
}
