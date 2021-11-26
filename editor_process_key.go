package main

import (
	"fmt"
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/slzatz/vimango/vim"
)

var termcodes = map[int]string{
	ARROW_UP:    "\x80ku",
	ARROW_DOWN:  "\x80kd",
	ARROW_RIGHT: "\x80kr",
	ARROW_LEFT:  "\x80kl",
	BACKSPACE:   "\x80kb", //? also works "\x08"
	HOME_KEY:    "\x80kh",
	DEL_KEY:     "\x80kD",
	PAGE_UP:     "\x80kP",
	PAGE_DOWN:   "\x80kN",
}

//note that bool returned is whether to redraw which will freeze program
//in BufferLines if mode is blocking
func editorProcessKey(c int) bool { //bool returned is whether to redraw

	//No matter what mode you are in an escape puts you in NORMAL mode
	if c == '\x1b' {
		vim.Input("\x1b")
		p.command = ""
		p.command_line = ""

		if p.mode == PREVIEW {
			// don't need to check WindowCursor - no change in pos
			//delete any images
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

		//pos, _ := v.WindowCursor(w) //set screen cx and cy from pos
		pos := vim.CursorGetPosition() //set screen cx and cy from pos
		p.fr = pos[0] - 1
		//p.fc = pos[1]
		p.fc = utf8.RuneCount(p.bb[p.fr][:pos[1]])
		sess.showEdMessage("")
		return true
	}
	/*
		 there are a set of commands like ctrl-w that we are intercepting
		note any command that changes the UI like splits or tabs doesn't make sense
		Also note that the if below falls through if p.command is "" and the character isn't
		one of the one that starts a command
	*/

	// the switch below deals with intercepting c before sending the char to nvim
	switch p.mode {

	case PREVIEW:
		switch c {
		//case PAGE_DOWN, ARROW_DOWN, 'j':
		case ARROW_DOWN, ctrlKey('j'):
			p.previewLineOffset++
		//case PAGE_UP, ARROW_UP, 'k':
		case ARROW_UP, ctrlKey('k'):
			if p.previewLineOffset > 0 {
				p.previewLineOffset--
			}
		}
		p.drawPreview()
		return false

	case SPELLING, VIEW_LOG:
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
			vim.Input("ciw" + p.suggestions[num] + "\x1b")
			//p.bb, _ = v.BufferLines(p.vbuf, 0, -1, true) //reading updated buffer
			p.bb = vim.BufferLines(p.vbuf)
			//pos, _ := v.WindowCursor(w) //screen cx and cy set from pos
			pos := vim.CursorGetPosition() //set screen cx and cy from pos
			p.fr = pos[0] - 1
			p.fc = utf8.RuneCount(p.bb[p.fr][:pos[1]])
			p.mode = NORMAL
			sess.showOrgMessage(p.command_line)
			p.command_line = "" // necessary
			return true
		}
		if c == DEL_KEY || c == BACKSPACE {
			if len(p.command_line) > 0 {
				p.command_line = p.command_line[:len(p.command_line)-1]
			}
		} else {
			p.command_line += string(c)
		}
		return false

	case NORMAL:
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
				vim.Input("\x1b")
				//keep tripping over this
				//command should return a redraw bool
				if strings.Index(" m l c d xz= su", p.command) != -1 {
					p.command = ""
					return false
				}

				p.command = ""
				//p.bb, _ = v.BufferLines(p.vbuf, 0, -1, true) //reading updated buffer
				p.bb = vim.BufferLines(p.vbuf)
				//pos, _ := v.WindowCursor(w) //screen cx and cy set from pos
				pos := vim.CursorGetPosition() //set screen cx and cy from pos
				p.fr = pos[0] - 1
				//p.fc = pos[1]
				p.fc = utf8.RuneCount(p.bb[p.fr][:pos[1]])
				return true
			} else {
				return false
			}
		}

	case VISUAL:
		if strings.IndexAny(string(c), "\x02\x05\x09") != -1 { //ctrl-b,e,i
			p.decorateWordVisual(c)

			// switch from VISUAl to NORMAL
			vim.Input("\x1b")
			p.mode = NORMAL
			p.command = ""
			//p.bb, _ = v.BufferLines(p.vbuf, 0, -1, true) //reading updated buffer
			p.bb = vim.BufferLines(p.vbuf)
			//pos, _ := v.WindowCursor(w)                  //screen cx and cy set from pos
			pos := vim.CursorGetPosition() //set screen cx and cy from pos
			p.fr = pos[0] - 1
			//p.fc = pos[1]
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
				//p.bb, _ = v.BufferLines(p.vbuf, 0, -1, true) //reading updated buffer
				//pos, _ := v.WindowCursor(w)                  //screen cx and cy set from pos
				p.bb = vim.BufferLines(p.vbuf)
				pos := vim.CursorGetPosition() //set screen cx and cy from pos
				p.fr = pos[0] - 1
				//p.fc = pos[1]
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
	} //end switch

	/////////////////below keycode sent to nvim///////////////////////////////////

	if z, found := termcodes[c]; found {
		//v.FeedKeys(z, "t", true)
		vim.Input(z)
		// if c is a control character we don't want to send to nvim 07012021
		// except we do want to send carriage return (13), ctrl-v (22), tab (9) and escape (27)
		// escape is dealt with first thing
		//} else if c < 32 && !(c == 13 || c == 22) {
	} else if c < 32 && !(c == 13 || c == 22 || c == 9) {
		return false
	} else {
		// < is special since it allows keycodes like <CR>
		if c == '<' {
			vim.Key("<LT>")
		} else {
			vim.Input(string(c))
			if c == 9 {
				c = 35
			}
			sess.showOrgMessage(string(c)) /// debug
		}
	}

	//mode, _ := v.Mode()
	mode := vim.GetMode()

	/* debugging
	sess.showOrgMessage("blocking: %t; mode: %s", mode.Blocking, mode.Mode)
	Example of input that blocks is entering a number (eg, 4x) in NORMAL mode
	If blocked = true you can't retrieve buffer with v.BufferLines -
	app just locks up
	*/

	//if mode.Blocking {
	if mode == 4 { //OP_PENDING
		return false // don't draw rows - which calls v.BufferLines
	}
	// the only way to get into EX_COMMAND or SEARCH
	//if mode.Mode == "c" && p.mode != SEARCH { //note that "c" => SEARCH
	if mode == 8 && p.mode != SEARCH { //note that "c" => SEARCH
		p.command_line = ""
		p.command = ""
		if c == ':' {
			p.mode = EX_COMMAND
			/*
			 below will put nvim back in NORMAL mode but listmango will be
			 in COMMAND_LINE mode, ie 'park' nvim in NORMAL mode
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
		//} else if mode.Mode == "i" && p.mode != INSERT {
	} else if mode == 16 && p.mode != INSERT {
		sess.showEdMessage("\x1b[1m-- INSERT --\x1b[0m")
	}

	p.mode = newModeMap[mode] //note that "c" => SEARCH
	if mode == 2 {            //VISUAL_MODE
		vmode := vim.VisualGetType()
		p.mode = visualModeMap[vmode]
	} else {
		p.mode = newModeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)
	}
	switch p.mode {
	//case INSERT, REPLACE, NORMAL:
	case VISUAL, VISUAL_LINE, VISUAL_BLOCK:
		p.highlightInfo()
	//	p.highlight = vim.VisualGetRange() //[]line col []line col
	case SEARCH:
		// return puts nvim into normal mode so don't need to catch return
		if c == DEL_KEY || c == BACKSPACE {
			if len(p.command_line) > 0 {
				p.command_line = p.command_line[:len(p.command_line)-1]
			}
		} else {
			p.command_line += string(c)
		}

		sess.showEdMessage("%s%s", p.searchPrefix, p.command_line)
		return false
	} // end switch p.mode

	//below is done for everything except SEARCH and EX_COMMAND
	//p.bb, _ = v.BufferLines(p.vbuf, 0, -1, true) //reading updated buffer
	//pos, _ := v.WindowCursor(w)                  //set screen cx and cy from pos
	p.bb = vim.BufferLines(p.vbuf)
	pos := vim.CursorGetPosition() //set screen cx and cy from pos
	p.fr = pos[0] - 1
	//p.fc = pos[1]
	p.fc = utf8.RuneCount(p.bb[p.fr][:pos[1]])

	/*
		if (c == 'u' || c == '\x12') && p.mode == NORMAL {
			showLastVimMessage()
		}
	*/

	if p.mode == PENDING { // -> operator pending (eg. typed 'd')
		return false
	} else {
		return true
	}
}
