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
func (e *Editor) editorProcessKey(c int) bool {

	//No matter what mode you are in an escape puts you in NORMAL mode
	if c == '\x1b' {
		vim.SendKey("<esc>")
		e.command = ""
		e.command_line = ""

		if e.mode == PREVIEW {
			// don't need to call CursorGetPosition - no change in pos
			// delete any images
			fmt.Print("\x1b_Ga=d\x1b\\")
			e.ShowMessage(BR, "")
			e.mode = NORMAL
			return true
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

	// the switch below deals with intercepting c before sending the char to vim
	switch e.mode {

	// in PREVIEW and VIEW_LOG don't send keys to vim
	case PREVIEW:
		switch c {
		case ARROW_DOWN, ctrlKey('j'):
			e.previewLineOffset++
		case ARROW_UP, ctrlKey('k'):
			if e.previewLineOffset > 0 {
				e.previewLineOffset--
			}
		}
		e.drawPreview()
		return false

	case VIEW_LOG:
		switch c {
		case PAGE_DOWN, ARROW_DOWN, 'j':
			e.previewLineOffset++
			e.drawOverlay()
			return false
		case PAGE_UP, ARROW_UP, 'k':
			if e.previewLineOffset > 0 {
				e.previewLineOffset--
				e.drawOverlay()
				return false
			}
		}
		if c == DEL_KEY || c == BACKSPACE {
			if len(e.command_line) > 0 {
				e.command_line = e.command_line[:len(e.command_line)-1]
			}
		} else {
			e.command_line += string(c)
		}
		return false
		// end case VIEW_LOG

	case NORMAL:
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
				if strings.IndexAny(e.command, "\x08\x0c") != -1 {
					return true
				}
				//keep tripping over this
				//these commands should return a redraw bool = false
				if strings.Index(" m l c d xz= su", e.command) != -1 {
					e.command = ""
					return false
				}

				e.command = ""
				e.ss = e.vbuf.Lines()
				pos := vim.GetCursorPosition() //set screen cx and cy from pos
				e.fr = pos[0] - 1
				e.fc = utf8.RuneCountInString(e.ss[e.fr][:pos[1]])
				return true
			} else {
				return false
			}
		}
		// end case NORMAL

	case VISUAL:
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
			return true
		}

		// end case VISUAL
	case EX_COMMAND:
		if c == '\r' {
			// Index doesn't work for vert resize
			// and LastIndex doesn't work for run
			// so total kluge below
			//if e.command_line[0] == '%'
			//if strings.Index(e.command_line, "s/") != -1 {
			if strings.HasPrefix(e.command_line, "s/") || strings.HasPrefix(e.command_line, "%s/") {
				if strings.HasSuffix(e.command_line, "/c") {
					//if strings.LastIndex(e.command_line, "/") < strings.LastIndex(e.command_line, "c") {
					e.ShowMessage(BR, "We don't support [c]onfirm")
					e.mode = NORMAL
					return false
				}

				vim.SendInput(":" + e.command_line + "\r")
				e.mode = NORMAL
				e.command = ""
				e.ss = e.vbuf.Lines()
				pos := vim.GetCursorPosition() //set screen cx and cy from pos
				e.fr = pos[0] - 1
				e.fc = utf8.RuneCountInString(e.ss[e.fr][:pos[1]])
				e.ShowMessage(BL, "search and replace: %s", e.command_line)
				return true
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

			//if cmd0, found := e_lookup_C[cmd]; found {
			if cmd0, found := e.exCmds[cmd]; found {
				cmd0(e)
				e.command_line = ""
				e.mode = NORMAL
				e.tabCompletion.index = 0
				e.tabCompletion.list = nil
				return false
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
			return false
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
					return false
				}
			} else {
				e.tabCompletion.index++
				if e.tabCompletion.index > len(e.tabCompletion.list)-1 {
					e.tabCompletion.index = 0
				}
			}
			e.command_line = e.command_line[:pos+1] + e.tabCompletion.list[e.tabCompletion.index]
			e.ShowMessage(BR, ":%s", e.command_line)
			return false
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
		return false
		//end case EX_COMMAND

	case SEARCH:
		if c == DEL_KEY || c == BACKSPACE {
			if len(e.command_line) > 0 {
				e.command_line = e.command_line[:len(e.command_line)-1]
			}
		} else {
			e.command_line += string(c)
		}
		e.ShowMessage(BR, "%s%s", e.searchPrefix, e.command_line)
		//process the key in vim below so no return
		//end SEARCH
	} //end switch

	// Process the key
	if z, found := termcodes[c]; found {
		vim.SendKey(z)
	} else {
		vim.SendInput(string(c))
	}

	mode := vim.GetCurrentMode()

	//OP_PENDING
	if mode == 4 {
		return false
	}
	// the only way to get into EX_COMMAND or SEARCH; 8 => SEARCH
	if mode == 8 && e.mode != SEARCH {
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
			e.ShowMessage(BL, e.searchPrefix)
		}
		return false
	} else if mode == 16 && e.mode != INSERT {
		e.ShowMessage(BR, "\x1b[1m-- INSERT --\x1b[0m")
	}

	//e.ShowMessage(BL, "Current mode from vim: %d, visual test: %v", mode, mode == 2) //////Debug

	if mode == 2 { //VISUAL_MODE
		vmode := vim.GetVisualType()
		e.mode = visualModeMap[vmode]
		//e.ShowMessage(BR, "Current visualType from vim: %d, visual test: %v", vmode, mode == 118)
		e.highlightInfo()
	} else {
		e.mode = modeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)
	}

	//below is done for everything except SEARCH and EX_COMMAND
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
