package main

import (
	"fmt"
	"os"
	"strings"

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

func (s *Screen) PositionMessage(loc Location) int { //Keep it Screen struct
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
