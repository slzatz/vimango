package main

import "github.com/slzatz/vimango/vim"

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
	code               string    //used by lsp thread and intended to avoid unnecessary calls to editorRowsToString
	vb_highlight       [2][4]int // holds various visual modes highlight coordinates
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
	//vbuf               nvim.Buffer
	vbuf               vim.Buffer
	bb                 [][]byte
	searchPrefix       string
	renderedNote       string
	previewLineOffset  int
	overlay            []string // for suggest, showVimMessageLog
	highlightPositions []Position
	suggestions        []string //spelling suggestions
}

func NewEditor() *Editor {
	return &Editor{
		cx:                 0, //actual cursor x position (takes into account any scroll/offset)
		cy:                 0, //actual cursor y position ""
		fc:                 0, //'file' x position as defined by reading sqlite text into rows vector
		fr:                 0, //'file' y position ""
		lineOffset:         0, //the number of lines of text at the top scrolled off the screen
		mode:               NORMAL,
		command:            "",
		command_line:       "",
		firstVisibleRow:    0,
		highlightSyntax:    true, // applies to golang, c++ etc. and markdown
		numberLines:        true,
		redraw:             false,
		output:             nil,
		left_margin_offset: LEFT_MARGIN_OFFSET, // 0 if not syntax highlighting b/o synt high =>line numbers

	}
}
