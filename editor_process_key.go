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
func (p *Editor) editorProcessKey(c int) bool { //bool returned is whether to redraw

	//No matter what mode you are in an escape puts you in NORMAL mode
	if c == '\x1b' {
		vim.Key("<esc>")
		p.command = ""
		p.command_line = ""

		if p.mode == PREVIEW {
			// don't need to call CursorGetPosition - no change in pos
			// delete any images
			fmt.Print("\x1b_Ga=d\x1b\\")
			p.ShowMessage(BR, "")
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
		p.fc = utf8.RuneCountInString(p.ss[p.fr][:pos[1]])
		p.ShowMessage(BR, "")
		return true
	}

	// the switch below deals with intercepting c before sending the char to vim
	switch p.mode {

	// in PREVIEW and VIEW_LOG don't send keys to vim
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
		if c == DEL_KEY || c == BACKSPACE {
			if len(p.command_line) > 0 {
				p.command_line = p.command_line[:len(p.command_line)-1]
			}
		} else {
			p.command_line += string(c)
		}
		return false
		// end case VIEW_LOG

	case NORMAL:
		// characters below make up first char of non-vim commands
		if len(p.command) == 0 {
			if strings.IndexAny(string(c), "\x17\x08\x0c\x02\x05\x09\x06\x0a\x0b"+leader) != -1 {
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
				vim.Key("<esc>")
				//keep tripping over this
				//these commands should return a redraw bool = false
				if strings.Index(" m l c d xz= su", p.command) != -1 {
					p.command = ""
					return false
				}

				p.command = ""
				p.ss = vim.BufferLines(p.vbuf)
				pos := vim.CursorGetPosition() //set screen cx and cy from pos
				p.fr = pos[0] - 1
				p.fc = utf8.RuneCountInString(p.ss[p.fr][:pos[1]])
				return true
			} else {
				return false
			}
		}
		// end case NORMAL

	case VISUAL:
		// Special commands in visual mode to do markdown decoration: ctrl-b, e, i
		if strings.IndexAny(string(c), "\x02\x05\x09") != -1 {
			p.decorateWordVisual(c)
			vim.Key("<esc>")
			p.mode = NORMAL
			p.command = ""
			p.ss = vim.BufferLines(p.vbuf)
			pos := vim.CursorGetPosition() //set screen cx and cy from pos
			p.fr = pos[0] - 1
			p.fc = utf8.RuneCountInString(p.ss[p.fr][:pos[1]])
			return true
		}

		// end case VISUAL
	case EX_COMMAND:
		if c == '\r' {
			// Index doesn't work for vert resize
			// and LastIndex doesn't work for run
			// so total kluge below
			//if p.command_line[0] == '%'
			if strings.Index(p.command_line, "s/") != -1 {
				if strings.LastIndex(p.command_line, "/") < strings.LastIndex(p.command_line, "c") {
					p.ShowMessage(BR, "We don't support [c]onfirm")
					p.mode = NORMAL
					return false
				}

				vim.Input(":" + p.command_line + "\r")
				p.mode = NORMAL
				p.command = ""
				p.ss = vim.BufferLines(p.vbuf)
				pos := vim.CursorGetPosition() //set screen cx and cy from pos
				p.fr = pos[0] - 1
				p.fc = utf8.RuneCountInString(p.ss[p.fr][:pos[1]])
				p.ShowMessage(BL, "search and replace: %s", p.command_line)
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

			p.ShowMessage(BR, "\x1b[41mNot an editor command: %s\x1b[0m", cmd)
			p.mode = NORMAL
			p.command_line = ""
			return false
		} //end 'r'

		if c == '\t' {
			pos := strings.Index(p.command_line, " ")
			if tabCompletion.list == nil {
				p.ShowMessage(BL, "tab")
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
					p.ShowMessage(BL, "dir: %s  base: %s", dir, partial)

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
			p.ShowMessage(BR, ":%s", p.command_line)
			return false
		}

		// process the key typed in COMMAND_LINE mode
		if c == DEL_KEY || c == BACKSPACE {
			if len(p.command_line) > 0 {
				p.command_line = p.command_line[:len(p.command_line)-1]
			}
		} else {
			p.command_line += string(c)
		}

		tabCompletion.idx = 0
		tabCompletion.list = nil

		p.ShowMessage(BR, ":%s", p.command_line)
		return false
		//end case EX_COMMAND

	case SEARCH:
		if c == DEL_KEY || c == BACKSPACE {
			if len(p.command_line) > 0 {
				p.command_line = p.command_line[:len(p.command_line)-1]
			}
		} else {
			p.command_line += string(c)
		}
		p.ShowMessage(BR, "%s%s", p.searchPrefix, p.command_line)
		//process the key in vim below so no return
		//end SEARCH
	} //end switch

	// Process the key
	if z, found := termcodes[c]; found {
		vim.Key(z)
	} else {
		vim.Input(string(c))
	}

	mode := vim.GetMode()

	//OP_PENDING
	if mode == 4 {
		return false
	}
	// the only way to get into EX_COMMAND or SEARCH; 8 => SEARCH
	if mode == 8 && p.mode != SEARCH {
		p.command_line = ""
		p.command = ""
		if c == ':' {
			p.mode = EX_COMMAND
			// 'park' vim in NORMAL mode and don't feed it keys
			vim.Key("<esc>")
			p.ShowMessage(BR, ":")
		} else {
			p.mode = SEARCH
			p.searchPrefix = string(c)
			p.ShowMessage(BL, p.searchPrefix)
		}
		return false
	} else if mode == 16 && p.mode != INSERT {
		p.ShowMessage(BR, "\x1b[1m-- INSERT --\x1b[0m")
	}

	if mode == 2 { //VISUAL_MODE
		vmode := vim.VisualGetType()
		p.mode = visualModeMap[vmode]
		p.highlightInfo()
	} else {
		p.mode = modeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)
	}

	//below is done for everything except SEARCH and EX_COMMAND
	p.ss = vim.BufferLines(p.vbuf)
	pos := vim.CursorGetPosition() //set screen cx and cy from pos
	p.fr = pos[0] - 1
	p.fc = utf8.RuneCountInString(p.ss[p.fr][:pos[1]])

	return true
}
