package main
import (
  "fmt"

  "github.com/slzatz/vimango/vim"
) 

//import "github.com/neovim/go-client/nvim"

// note that there isn't a columnOffset because currently only word wrap supported

type Editor struct {
	cx, cy             int //screen cursor x and y position
	fc, fr             int // file cursor x and y position
	lineOffset         int //first row based on user scroll
	screenlines        int //number of lines for this Editor
	screencols         int //number of columns for this Editor
	left_margin        int //can vary (so could TOP_MARGIN - will do that later
	left_margin_offset int // 0 if no line numbers
	top_margin         int
	highlight          [2][2]int // [line col][line col] -> note line is 1-based not zero-based
	mode               Mode
	command_line       string //for commands on the command line; string doesn't include ':'
	command            string // right now includes normal mode commands and command line commands
	last_command       string
	firstVisibleRow    int
	highlightSyntax    bool
	numberLines        bool
	redraw             bool
	id                 int //db id of entry
	output             *Output
	vbuf               vim.Buffer
	ss                 []string
	searchPrefix       string
	renderedNote       string
	previewLineOffset  int
	overlay            []string // for suggest, showVimMessageLog
	highlightPositions []Position
	suggestions        []string //spelling suggestions
	bufferTick         int
	modified           bool     // tracks if the buffer has been modified
  title              string   // title of the note
  Database          *Database // pointer to the database
  AppUI             *Session  // pointer to the session
}

func (e *Editor) ShowMessage(max_length int, format string, a ...interface{}) { //Sesseion struct

	str := fmt.Sprintf(format, a...)
	if len(str) > max_length {
		str = str[:max_length]
	}
	fmt.Print(str)
}
