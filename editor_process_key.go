package main

import (
	"fmt"
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
		e.mode = NORMAL
		e.ShowMessage(BL, "e.mode: %s", e.mode) //////Debug
		e.command = ""
		e.command_line = ""

		if e.mode == PREVIEW {
			// don't need to call CursorGetPosition - no change in pos
			// delete any images
			fmt.Print("\x1b_Ga=d\x1b\\")
			e.ShowMessage(BR, "")
			//e.mode = NORMAL
			redraw = true
			return
		}

		e.mode = NORMAL

		/*
			if previously in visual mode some text may be highlighted so need to return true
			 also need the cursor position because for example going from INSERT -> NORMAL causes cursor to move back
			 note you could fall through to getting pos but that recalcs rows which is unnecessary
		*/

		pos := vim.GetCursorPosition() //set screen cx and cy from pos
		e.fr = pos[0] - 1
		e.fc = utf8.RuneCountInString(e.ss[e.fr][:pos[1]])
		e.ShowMessage(BR, "")
		return true
	}
	skip := false

	switch e.mode {
	case INSERT:
		redraw = false // doesn't matter when skip is false
		skip = false
	case NORMAL, OTHER:
		redraw, skip = e.NormalModeKeyHandler(c)
	case VISUAL:
		redraw, skip = e.VisualModeKeyHandler(c)
	case EX_COMMAND:
		redraw, skip = e.ExModeKeyHandler(c)
	case SEARCH:
		redraw, skip = e.SearchModeKeyHandler(c)
	case PREVIEW:
		redraw, skip = e.PreviewModeKeyHandler(c)
	case VIEW_LOG:
		redraw, skip = e.ViewLogModeKeyHandler(c)
	}
	if skip {
		return
	}
	// Process the key
	if z, found := termcodes[c]; found {
		vim.SendKey(z)
	} else {
		vim.SendInput(string(c))
	}

	mode := vim.GetCurrentMode()

	switch mode {
	case 4: //OP_PENDING
		return false
	case 8: //SEARCH and EX_COMMAND
		if e.mode != SEARCH {
			e.command_line = ""
			e.command = ""
			if c == ':' {
				e.mode = EX_COMMAND
				// 'park' vim in NORMAL mode and don't feed it keys
				vim.SendKey("<esc>")
				e.ShowMessage(BR, ":")
			} else {
				e.mode = SEARCH
				e.searchPrefix = string(c)
				e.ShowMessage(BR, e.searchPrefix)
			}
			return false
		}
	case 16: //INSERT
		if e.mode != INSERT {
			e.mode = INSERT
			e.ShowMessage(BR, "\x1b[1m-- INSERT --\x1b[0m")
		}
	case 2: //VISUAL_MODE
		vmode := vim.GetVisualType()
		e.mode = visualModeMap[vmode]
		//e.ShowMessage(BR, "Current visualType from vim: %d, visual test: %v", vmode, mode == 118)
		e.highlightInfo()
		//necessary for recognizing return after typing search term
	default:
		m, ok := modeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)yy
		if ok {
			e.mode = m
		} else {
			e.mode = OTHER //257 appears to be the vim number - seems to be a variant of NORMAL
		}
		e.ShowMessage(BL, "mode: %d  e.mode: %s", mode, e.mode) //////Debug
	}
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

	return true
}

// the switch below deals with intercepting c before sending the char to vim
//	switch e.mode {

// in PREVIEW and VIEW_LOG don't send keys to vim
// case PREVIEW:
func (e *Editor) PreviewModeKeyHandler(c int) (bool, bool) {
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

// end case VIEW_LOG

// case NORMAL, OTHER (257):
func (e *Editor) NormalModeKeyHandler(c int) (redraw, skip bool) {
	// characters below make up first char of non-vim commands
	if len(e.command) == 0 {
		if strings.IndexAny(string(c), "\x17\x08\x0c\x02\x05\x09\x06\x0a\x0b"+leader) != -1 {
			e.command = string(c)
		}
	} else {
		e.command += string(c)
	}

	if len(e.command) > 0 {
		if cmd, found := e.normalCmds[e.command]; found {
			cmd(e, c)
			vim.SendKey("<esc>")
			if strings.IndexAny(e.command, "\x08\x0c") != -1 { //Ctrl H, Ctrl L
				return true, true
			}
			//keep tripping over this
			//these commands should return a redraw bool = false
			if strings.Index(" m l c d xz= su", e.command) != -1 {
				e.command = ""
				return false, true
			}

			e.command = ""
			e.ss = e.vbuf.Lines()
			pos := vim.GetCursorPosition() //set screen cx and cy from pos
			e.fr = pos[0] - 1
			e.fc = utf8.RuneCountInString(e.ss[e.fr][:pos[1]])
			redraw = true
		} else {
			redraw = false
		}
		skip = true
	}
	skip = false
	return redraw, skip
}

// end case NORMAL

// case VISUAL:
func (e *Editor) VisualModeKeyHandler(c int) (redraw, skip bool) {
	// Special commands in visual mode to do markdown decoration: ctrl-b, e, i
	if strings.IndexAny(string(c), "\x02\x05\x09") != -1 {
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

// end case VISUAL

// case EX_COMMAND:
func (e *Editor) ExModeKeyHandler(c int) (redraw, skip bool) {
	if c == '\r' {
		// Index doesn't work for vert resize
		// and LastIndex doesn't work for run
		// so total kluge below
		//if e.command_line[0] == '%'
		//if strings.Index(e.command_line, "s/") != -1
		if strings.HasPrefix(e.command_line, "s/") || strings.HasPrefix(e.command_line, "%s/") {
			if strings.HasSuffix(e.command_line, "/c") {
				//if strings.LastIndex(e.command_line, "/") < strings.LastIndex(e.command_line, "c")
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
	//end case EX_COMMAND
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
	//process the key in vim below so no return
	//end SEARCH
} //end switch
