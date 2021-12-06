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

//note that bool returned is whether to redraw
func editorProcessKey(c int) bool { //bool returned is whether to redraw

	//No matter what mode you are in an escape puts you in NORMAL mode
	if c == '\x1b' {
		vim.Key("<esc>")
		p.command = ""
		p.command_line = ""

		if p.mode == PREVIEW {
			// don't need to call CursorGetPosition - no change in pos
			// delete any images
			fmt.Print("\x1b_Ga=d\x1b\\")
			sess.showEdMessage("")
			p.mode = NORMAL
			return true
		}

		p.mode = NORMAL

		/*
			if previously in visual mode some text may be highlighted so need to return true
			 also need the cursor position because for example going from INSERT -> NORMAL causes cursor to move back
			 note you could fall through to getting pos but that recalcs rows which is unnecessary
		*/

		pos := vim.CursorGetPosition() //set screen cx and cy from pos
		p.fr = pos[0] - 1
		p.fc = utf8.RuneCount(p.bb[p.fr][:pos[1]])
		sess.showEdMessage("")
		return true
	}

	// the switch below deals with intercepting c before sending the char to vim
	switch p.mode {

	case PREVIEW:
		switch c {
		case ARROW_DOWN, ctrlKey('j'):
			p.previewLineOffset++
		case ARROW_UP, ctrlKey('k'):
			if p.previewLineOffset > 0 {
				p.previewLineOffset--
			}
		}
		p.drawPreview()
		return false

	//case SPELLING, VIEW_LOG:
	case VIEW_LOG:
		switch c {
		case PAGE_DOWN, ARROW_DOWN, 'j':
			p.previewLineOffset++
			p.drawOverlay()
			return false
		case PAGE_UP, ARROW_UP, 'k':
			if p.previewLineOffset > 0 {
				p.previewLineOffset--
				p.drawOverlay()
				return false
			}
		}
		// enter a number and that's the selected replacement for a mispelling
		/*
			if c == '\r' && p.mode == SPELLING {
				num, err := strconv.Atoi(p.command_line)
				if err != nil {
					sess.showEdMessage("That wasn't a number!")
					p.mode = NORMAL
					p.command_line = ""
					return true
				}
				if num < 0 || num > len(p.suggestions)-1 {
					sess.showEdMessage("%d not in appropriate range", num)
					p.mode = NORMAL
					p.command_line = ""
					return true
				}
				vim.Input2("ciw" + p.suggestions[num] + "\x1b")
				p.bb = vim.BufferLines(p.vbuf)
				pos := vim.CursorGetPosition() //set screen cx and cy from pos
				p.fr = pos[0] - 1
				p.fc = utf8.RuneCount(p.bb[p.fr][:pos[1]])
				p.mode = NORMAL
				sess.showOrgMessage(p.command_line)
				p.command_line = "" // necessary
				return true
			}
		*/
		if c == DEL_KEY || c == BACKSPACE {
			if len(p.command_line) > 0 {
				p.command_line = p.command_line[:len(p.command_line)-1]
			}
		} else {
			p.command_line += string(c)
		}
		return false

	case NORMAL:
		// characters below make up first char of non-vim commands
		if len(p.command) == 0 {
			if strings.IndexAny(string(c), "\x17\x08\x0c\x02\x05\x09\x06\x0a\x0b z") != -1 {
				p.command = string(c)
			}
		} else {
			p.command += string(c)
		}

		if len(p.command) > 0 {
			if cmd, found := e_lookup2[p.command]; found {
				switch cmd := cmd.(type) {
				case func(*Editor):
					cmd(p)
				case func():
					cmd()
				case func(*Editor, int):
					cmd(p, c)
				case func(*Editor) bool:
					cmd(p)
				}
				// seems to be necessary at least for certain commands
				//vim.Input("\x1b")
				vim.Key("<esc>")
				//keep tripping over this
				//these commands should return a redraw bool = false
				if strings.Index(" m l c d xz= su", p.command) != -1 {
					p.command = ""
					return false
				}

				p.command = ""
				p.bb = vim.BufferLines(p.vbuf)
				pos := vim.CursorGetPosition() //set screen cx and cy from pos
				p.fr = pos[0] - 1
				p.fc = utf8.RuneCount(p.bb[p.fr][:pos[1]])
				return true
			} else {
				return false
			}
		}

	case VISUAL:
		// Special commands in visual mode to do markdown decoration: ctrl-b, e, i
		if strings.IndexAny(string(c), "\x02\x05\x09") != -1 {
			p.decorateWordVisual(c)
			// switch from VISUAl to NORMAL
			vim.Key("<esc>")
			p.mode = NORMAL
			p.command = ""
			p.bb = vim.BufferLines(p.vbuf)
			pos := vim.CursorGetPosition() //set screen cx and cy from pos
			p.fr = pos[0] - 1
			p.fc = utf8.RuneCount(p.bb[p.fr][:pos[1]])
			return true
		}

	case EX_COMMAND:
		if c == '\r' {
			// Index doesn't work for vert resize
			// and LastIndex doesn't work for run
			// so total kluge below
			//if p.command_line[0] == '%' {
			if strings.Index(p.command_line, "s/") != -1 {
				if strings.LastIndex(p.command_line, "/") < strings.LastIndex(p.command_line, "c") {
					sess.showEdMessage("We don't support [c]onfirm")
					p.mode = NORMAL
					return false
				}

				vim.Input(":" + p.command_line + "\r")
				p.mode = NORMAL
				p.command = ""
				p.bb = vim.BufferLines(p.vbuf)
				pos := vim.CursorGetPosition() //set screen cx and cy from pos
				p.fr = pos[0] - 1
				p.fc = utf8.RuneCount(p.bb[p.fr][:pos[1]])
				sess.showOrgMessage("search and replace: %s", p.command_line)
				return true
			}
			var pos int
			var cmd string
			if strings.HasPrefix(p.command_line, "vert") {
				pos = strings.LastIndex(p.command_line, " ")
			} else {
				pos = strings.Index(p.command_line, " ")
			}
			if pos != -1 {
				cmd = p.command_line[:pos]
			} else {
				cmd = p.command_line
			}

			if cmd0, found := e_lookup_C[cmd]; found {
				cmd0(p)
				p.command_line = ""
				p.mode = NORMAL
				tabCompletion.idx = 0
				tabCompletion.list = nil
				return false
			}

			sess.showEdMessage("\x1b[41mNot an editor command: %s\x1b[0m", cmd)
			p.mode = NORMAL
			p.command_line = ""
			return false
		} //end 'r'

		if c == '\t' {
			pos := strings.Index(p.command_line, " ")
			if tabCompletion.list == nil {
				sess.showOrgMessage("tab")
				var s string
				if pos != -1 {
					s = p.command_line[pos+1:]
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
					sess.showOrgMessage("dir: %s  base: %s", dir, partial)

					for _, path := range paths {
						if strings.HasPrefix(path.Name(), partial) {
							tabCompletion.list = append(tabCompletion.list, filepath.Join(dir, path.Name()))
						}
					}
				}
				if len(tabCompletion.list) == 0 {
					return false
				}
			} else {
				tabCompletion.idx++
				if tabCompletion.idx > len(tabCompletion.list)-1 {
					tabCompletion.idx = 0
				}
			}
			p.command_line = p.command_line[:pos+1] + tabCompletion.list[tabCompletion.idx]
			sess.showEdMessage(":%s", p.command_line)
			return false
		}

		if c == DEL_KEY || c == BACKSPACE {
			if len(p.command_line) > 0 {
				p.command_line = p.command_line[:len(p.command_line)-1]
			}
		} else {
			p.command_line += string(c)
		}

		tabCompletion.idx = 0
		tabCompletion.list = nil

		sess.showEdMessage(":%s", p.command_line)
		return false //end EX_COMMAND

	case SEARCH:
		if c == DEL_KEY || c == BACKSPACE {
			if len(p.command_line) > 0 {
				p.command_line = p.command_line[:len(p.command_line)-1]
			}
		} else {
			p.command_line += string(c)
		}
		sess.showEdMessage("%s%s", p.searchPrefix, p.command_line)
		//return false //end SEARCH
	} //end switch

	// Most control characters we don't want to send to vim
	// however, we do want to send carriage return (13), ctrl-v (22), tab (9) and escape (27)
	// escape is dealt with first thing
	// could we put this at the top
	if c < 32 && !(c == 13 || c == 22 || c == 9) {
		return false
	}

	// Process the key
	if z, found := termcodes[c]; found {
		vim.Key(z)
		//vim.Input(z)
	} else {
		vim.Input(string(c))
	}

	mode := vim.GetMode()

	if mode == 4 { //OP_PENDING
		return false // don't draw rows
	}
	// the only way to get into EX_COMMAND or SEARCH; 8 => SEARCH
	if mode == 8 && p.mode != SEARCH {
		p.command_line = ""
		p.command = ""
		if c == ':' {
			p.mode = EX_COMMAND
			/*
			 below will put nvim back in NORMAL mode but listmango will be
			 in COMMAND_LINE mode, ie 'park' vim in NORMAL mode
			 and don't feed it any keys while in listmango COMMAND_LINE mode
			*/
			vim.Input("\x1b")
			sess.showEdMessage(":")
		} else {
			p.mode = SEARCH
			p.searchPrefix = string(c)
			sess.showEdMessage(p.searchPrefix)
		}
		return false
	} else if mode == 16 && p.mode != INSERT {
		sess.showEdMessage("\x1b[1m-- INSERT --\x1b[0m")
	}

	if mode == 2 { //VISUAL_MODE
		vmode := vim.VisualGetType()
		p.mode = visualModeMap[vmode]
		p.highlightInfo()
	} else {
		p.mode = modeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)
	}

	// this is the 'new' p.mode based on vim processing input
	//switch p.mode {
	//case INSERT, REPLACE, NORMAL:
	//case VISUAL, VISUAL_LINE, VISUAL_BLOCK:
	//	p.highlightInfo()
	//case SEARCH:
	// return puts vim into normal mode so don't need to catch return
	/*
		if c == DEL_KEY || c == BACKSPACE {
			if len(p.command_line) > 0 {
				p.command_line = p.command_line[:len(p.command_line)-1]
			}
		} else {
			p.command_line += string(c)
		}

		sess.showEdMessage("%s%s", p.searchPrefix, p.command_line)
		return false
	*/
	//} // end switch p.mode

	//below is done for everything except SEARCH and EX_COMMAND
	p.bb = vim.BufferLines(p.vbuf)
	pos := vim.CursorGetPosition() //set screen cx and cy from pos
	p.fr = pos[0] - 1
	p.fc = utf8.RuneCount(p.bb[p.fr][:pos[1]])

	/*
		if p.mode == PENDING { // -> operator pending (eg. typed 'd')
			return false
		} else {
			return true
		}
	*/
	return true
}
