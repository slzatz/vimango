package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/slzatz/vimango/rawmode"
)

type Screen struct {
	screenCols      int
	screenLines     int //total number of screen lines
	textLines       int // considering margins, bottom messages
	divider         int
	totaleditorcols int
	imgSizeY        int
	edPct           int             // percent that editor space takes up of whole horiz screen real estate
	ws              rawmode.Winsize //Row,Col,Xpixel,Ypixel unint16
	altRowoff       int             //the number of rows scrolled in notice window)
	notice          []string        // e.g., synch results, help test, research notification
	Session         *Session
	//images           map[string]*image.Image
}

func (s *Screen) eraseScreenRedrawLines() {
	fmt.Fprint(os.Stdout, "\x1b[2J") //Erase the screen
	fmt.Fprint(os.Stdout, "\x1b(0")  //Enter line drawing mode
	for j := 1; j < s.screenLines+1; j++ {
		fmt.Fprintf(os.Stdout, "\x1b[%d;%dH", TOP_MARGIN+j, s.divider)

		// x = 0x78 vertical line; q = 0x71 horizontal line
		// 37 = white; 1m = bold (note only need one 'm')
		fmt.Fprint(os.Stdout, "\x1b[37;1mx")
	}

	fmt.Fprint(os.Stdout, "\x1b[1;1H")
	for k := 1; k < s.screenCols; k++ {
		// cursor advances - same as char write
		fmt.Fprint(os.Stdout, "\x1b[37;1mq")
	}

	if s.divider > 10 {
		fmt.Fprintf(os.Stdout, "\x1b[%d;%dH", TOP_MARGIN, s.divider-TIME_COL_WIDTH+1)
		fmt.Fprint(os.Stdout, "\x1b[37;1mw") //'T' corner
	}

	// draw next column's 'T' corner - divider
	fmt.Fprintf(os.Stdout, "\x1b[%d;%dH", TOP_MARGIN, s.divider)
	fmt.Fprint(os.Stdout, "\x1b[37;1mw") //'T' corner

	fmt.Fprint(os.Stdout, "\x1b[0m") // return background to normal (? necessary)
	fmt.Fprint(os.Stdout, "\x1b(B")  //exit line drawing mode
}

func (s *Screen) eraseRightScreen() {
	var ab strings.Builder

	// NOTE: With Unicode placeholders (U=1), we DON'T delete placements!
	// The placeholders are part of the text and scroll naturally.
	// The virtual placement is just metadata that the placeholders reference.

	ab.WriteString("\x1b[?25l") //hides the cursor

	//below positions cursor such that top line is erased the first time through
	//for loop although ? could really start on second line since need to redraw
	//horizontal line anyway
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN, s.divider+1)

	//erase the screen
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", s.divider)
	for i := 0; i < s.screenLines-TOP_MARGIN; i++ {
		ab.WriteString("\x1b[K")
		ab.WriteString(lf_ret)
	}
	//ab.WriteString("\x1b[K") //added 09302020 to erase the last line (message line)

	// redraw top horizontal line which has t's and was erased above
	// ? if the individual editors draw top lines do we need to just
	// erase but not draw
	ab.WriteString("\x1b(0")                   // Enter line drawing mode
	for j := 1; j < s.totaleditorcols+1; j++ { //added +1 0906/2020
		fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN, s.divider+j)
		// below x = 0x78 vertical line (q = 0x71 is horizontal) 37 = white;
		// 1m = bold (note only need one 'm'
		ab.WriteString("\x1b[37;1mq")
	}

	//exit line drawing mode
	ab.WriteString("\x1b(B")

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+1, s.divider+2)
	ab.WriteString("\x1b[0m") // needed or else in bold mode from line drawing above

	fmt.Fprint(os.Stdout, ab.String())
}

func (s *Screen) drawRightScreen() {
	for _, w := range s.Session.Windows {
		w.drawText()
		w.drawFrame()
		w.drawStatusBar()
	}
}

func (s *Screen) positionWindows() {
	windowSlots := len(s.Session.Windows)

	cols := -1 + (s.screenCols-s.divider)/windowSlots
	i := -1 //i = number of columns of windows -1
	for _, w := range s.Session.Windows {
		i++
		w.left_margin = s.divider + i*cols + i
		w.screencols = cols
		w.setLinesMargins()
	}
}

func (s *Screen) GetWindowSize() error { //should be updateWindowDimensions

	ws, err := rawmode.GetWindowSize()
	if err != nil {
		return err
	}

	s.screenCols = int(ws.Col)
	s.screenLines = int(ws.Row)
	s.ws = *ws

	return nil
}

func (s *Screen) PositionMessage(loc Location) int { // positions message and returns max length
	var max_length int

	switch loc {
	case BL:
		fmt.Printf("\x1b[%d;%dH\x1b[1K\x1b[%d;1H", s.textLines+2+TOP_MARGIN, s.divider, s.textLines+2+TOP_MARGIN)
		max_length = s.divider
	case BR:
		fmt.Printf("\x1b[%d;%dH\x1b[K", s.textLines+2+TOP_MARGIN, s.divider+1)
		max_length = s.screenCols - s.divider
	}
	return max_length
}

func (s *Screen) drawNotice(str string, isMarkdown bool, loc Location) {
	if len(str) == 0 {
		return
	}
	if isMarkdown {
		s.renderNotice(str, loc)
	} else {
		var width int
		if loc == TR {
			width = s.totaleditorcols - NOTICE_RIGHT_PADDING
		} else {
			width = s.divider - 2
		}
		note := WordWrap(str, width, 0)
		s.notice = strings.Split(note, "\n")
	}
	if loc == TR {
		s.drawNoticeLayer()
		s.drawNoticeText()
	} else {
		s.drawNoticeLayerLeft()
		s.drawNoticeTextLeft()
	}
}

func (s *Screen) renderNotice(str string, loc Location) {
	var offset, width int
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath(getGlamourStylePath()),
		glamour.WithWordWrap(0),
	)
	note, _ := r.Render(str)
	// glamour seems to add a '\n' at the start
	note = strings.TrimSpace(note)
	// Decode any Kitty text sizing markers (OSC 66)
	note = ansi.DecodeKittyTextSizeMarkers(note)
	if app.Organizer.mode == HELP || (app.Session.activeEditor != nil && app.Session.activeEditor.mode == HELP) {
		offset = 16
	}
	// width = s.screenCols - s.divider - width
	if loc == TR {
		width = s.totaleditorcols - NOTICE_RIGHT_PADDING
	} else {
		width = s.divider - NOTICE_RIGHT_PADDING
	}
	note = WordWrap(note, width, offset)
	s.notice = strings.Split(note, "\n")
}

func (s *Screen) drawNoticeLayer() {
	var ab strings.Builder
	width := s.totaleditorcols - 10
	length := len(s.notice)
	if length > s.textLines-10 {
		length = s.textLines - 10
	}

	// \x1b[NC moves cursor forward by N columns
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", s.divider+6)

	//hide the cursor
	ab.WriteString("\x1b[?25l")
	// move the cursor
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, s.divider+7)

	//erase set number of chars on each line
	erase_chars := fmt.Sprintf("\x1b[%dX", s.totaleditorcols-10)
	for i := 0; i < length; i++ {
		ab.WriteString(erase_chars)
		ab.WriteString(lf_ret)
	}

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, s.divider+7)

	// \x1b[ 2*x is DECSACE to operate in rectable mode
	// \x1b[%d;%d;%d;%d;48;5;235$r is DECCARA to apply specified attributes (background color 235) to rectangle area
	// \x1b[ *x is DECSACE to exit rectangle mode
	fmt.Fprintf(&ab, "\x1b[2*x\x1b[%d;%d;%d;%d;48;5;235$r\x1b[*x",
		TOP_MARGIN+6, s.divider+7, TOP_MARGIN+4+length, s.divider+7+width)
	ab.WriteString("\x1b[48;5;235m") //draws the box lines with same background as above rectangle
	fmt.Print(ab.String())
	s.drawNoticeBox()
}

func (s *Screen) drawNoticeLayerLeft() {
	var ab strings.Builder
	//width := s.totaleditorcols - 10
	width := s.divider - 10 /////////////
	length := len(s.notice)
	if length > s.textLines-10 {
		length = s.textLines - 10
	}

	// \x1b[NC moves cursor forward by N columns
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", 2) //////////

	//hide the cursor
	ab.WriteString("\x1b[?25l")
	// move the cursor
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, 3) //////

	//erase set number of chars on each line
	erase_chars := fmt.Sprintf("\x1b[%dX", s.divider-4) ///////
	for i := 0; i < length; i++ {
		ab.WriteString(erase_chars)
		ab.WriteString(lf_ret)
	}

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, 3) /////

	// \x1b[ 2*x is DECSACE to operate in rectable mode
	// \x1b[%d;%d;%d;%d;48;5;235$r is DECCARA to apply specified attributes (background color 235) to rectangle area
	// \x1b[ *x is DECSACE to exit rectangle mode
	fmt.Fprintf(&ab, "\x1b[2*x\x1b[%d;%d;%d;%d;48;5;235$r\x1b[*x",
		TOP_MARGIN+6, 7, TOP_MARGIN+4+length, 7+width)
	ab.WriteString("\x1b[48;5;235m") //draws the box lines with same background as above rectangle
	fmt.Print(ab.String())
	s.drawNoticeBoxLeft()
}
func (s *Screen) drawNoticeBox() {
	width := s.totaleditorcols - 10
	length := len(s.notice) + 1
	if length > s.textLines-9 {
		length = s.textLines - 9
	}
	var ab strings.Builder
	move_cursor := fmt.Sprintf("\x1b[%dC", width)

	ab.WriteString("\x1b(0") // Enter line drawing mode
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5, s.divider+6)
	ab.WriteString("\x1b[37;1ml") //upper left corner

	for i := 1; i < length; i++ {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5+i, s.divider+6)
		// x=0x78 vertical line (q=0x71 is horizontal) 37=white; 1m=bold (only need 1 m)
		ab.WriteString("\x1b[37;1mx")
		ab.WriteString(move_cursor)
		ab.WriteString("\x1b[37;1mx")
	}

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+4+length, s.divider+6)
	ab.WriteString("\x1b[1B")
	ab.WriteString("\x1b[37;1mm") //lower left corner

	move_cursor = fmt.Sprintf("\x1b[1D\x1b[%dB", length)

	for i := 1; i < width+1; i++ {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5, s.divider+6+i)
		ab.WriteString("\x1b[37;1mq")
		ab.WriteString(move_cursor)
		ab.WriteString("\x1b[37;1mq")
	}

	ab.WriteString("\x1b[37;1mj") //lower right corner
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5, s.divider+7+width)
	ab.WriteString("\x1b[37;1mk") //upper right corner

	//exit line drawing mode
	ab.WriteString("\x1b(B")
	ab.WriteString("\x1b[0m")
	ab.WriteString("\x1b[?25h")
	fmt.Print(ab.String())
}

func (s *Screen) drawNoticeBoxLeft() {
	//width := s.totaleditorcols - 10
	width := s.divider - 4 /////////////
	length := len(s.notice) + 1
	if length > s.textLines-9 {
		length = s.textLines - 9
	}
	var ab strings.Builder
	move_cursor := fmt.Sprintf("\x1b[%dC", width)

	ab.WriteString("\x1b(0") // Enter line drawing mode
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5, 2)
	ab.WriteString("\x1b[37;1ml") //upper left corner

	for i := 1; i < length; i++ {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5+i, 2)
		// x=0x78 vertical line (q=0x71 is horizontal) 37=white; 1m=bold (only need 1 m)
		ab.WriteString("\x1b[37;1mx")
		ab.WriteString(move_cursor)
		ab.WriteString("\x1b[37;1mx")
	}

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+4+length, 2)
	ab.WriteString("\x1b[1B")
	ab.WriteString("\x1b[37;1mm") //lower left corner

	move_cursor = fmt.Sprintf("\x1b[1D\x1b[%dB", length)

	for i := 1; i < width+1; i++ {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5, 2+i) //3
		ab.WriteString("\x1b[37;1mq")
		ab.WriteString(move_cursor)
		ab.WriteString("\x1b[37;1mq")
	}

	ab.WriteString("\x1b[37;1mj")                          //lower right corner
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5, 3+width) //3
	ab.WriteString("\x1b[37;1mk")                          //upper right corner

	//exit line drawing mode
	ab.WriteString("\x1b(B")
	ab.WriteString("\x1b[0m")
	ab.WriteString("\x1b[?25h")
	fmt.Print(ab.String())
}

func (s *Screen) drawNoticeText() {
	length := len(s.notice)
	if length > s.textLines-10 {
		length = s.textLines - 10
	}
	start := s.altRowoff
	var end int
	// check if there are more lines than can fit on the screen
	if len(s.notice)-start > length {
		end = length + start - 2
	} else {
		//end = len(o.note) - 1
		end = len(s.notice)
	}
	fmt.Fprintf(os.Stdout, "\x1b[%d;%dH", TOP_MARGIN+6, s.divider+8)
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", s.divider+7)
	fmt.Print(strings.Join(s.notice[start:end], lf_ret))
	fmt.Print(RESET) //sometimes there is an unclosed escape sequence
}

func (s *Screen) drawNoticeTextLeft() {
	length := len(s.notice)
	if length > s.textLines-10 {
		length = s.textLines - 10
	}
	start := s.altRowoff
	var end int
	// check if there are more lines than can fit on the screen
	if len(s.notice)-start > length {
		end = length + start - 2
	} else {
		//end = len(o.note) - 1
		end = len(s.notice)
	}
	fmt.Fprintf(os.Stdout, "\x1b[%d;%dH", TOP_MARGIN+6, 4)
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", 3)
	fmt.Print(strings.Join(s.notice[start:end], lf_ret))
	fmt.Print(RESET) //sometimes there is an unclosed escape sequence
}
