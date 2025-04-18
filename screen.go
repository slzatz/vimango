package main

import (
//	"bytes"
//	"encoding/gob"
	"fmt"
	"os"
	"strings"

	"github.com/slzatz/vimango/rawmode"
	"golang.org/x/sys/unix"
)

type Screen struct {
	screenCols       int
	screenLines      int //total number of screen lines
	textLines        int // considering margins, bottom messages
	divider          int
	totaleditorcols  int
	imgSizeY         int
	edPct            int // percent that editor space takes up of whole horiz screen real estate
	ws               unix.Winsize //Row,Col,Xpixel,Ypixel unint16
	//images           map[string]*image.Image
}


func (s *Screen) numberOfEditors() int {
	i := 0
	for _, w := range app.Windows {
		if _, ok := w.(*Editor); ok {
			i++
		}
	}
	return i
}

func (s *Screen) editors() []*Editor {
	eds := []*Editor{}
	for _, w := range app.Windows {
		if e, ok := w.(*Editor); ok {
			eds = append(eds, e)
		}
	}
	return eds
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

	fmt.Print("\x1b_Ga=d\x1b\\") //delete any images
	ab.WriteString("\x1b[?25l")  //hides the cursor

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
	ab.WriteString("\x1b[K") //added 09302020 to erase the last line (message line)

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
	for _, w := range app.Windows {
		w.drawText()
		w.drawFrame()
		w.drawStatusBar() 
	}
}

func (s *Screen) positionWindows() {
	windowSlots := 0
	for _, w := range app.Windows {
		switch v := w.(type) {
		case *Output:
			if !v.is_below {
				windowSlots++
			}
			// or default
		case *Editor:
			windowSlots++
		}
	}

	cols := -1 + (s.screenCols-s.divider)/windowSlots
	i := -1 //i = number of columns of windows -1
	for _, w := range app.Windows {
		switch v := w.(type) {
		case *Output:
			if !v.is_below {
				i++
			}
			v.left_margin = s.divider + i*cols + i
			v.screencols = cols
			v.setLinesMargins()
		case *Editor:
			i++
			v.left_margin = s.divider + i*cols + i
			v.screencols = cols
			v.setLinesMargins()
		}
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
/*
func (s *Screen) returnCursor() {
	var ab strings.Builder
	if sess.editorMode {
		switch p.mode {
		case PREVIEW, SPELLING, VIEW_LOG:
			// we don't need to position cursor and don't want cursor visible
			fmt.Print(ab.String())
			return
		case EX_COMMAND, SEARCH:
			fmt.Fprintf(&ab, "\x1b[%d;%dH", s.textLines+TOP_MARGIN+2, len(p.command_line)+s.divider+2)
		default:
			fmt.Fprintf(&ab, "\x1b[%d;%dH", p.cy+p.top_margin, p.cx+p.left_margin+p.left_margin_offset+1)
		}
	} else {
		switch org.mode {
		case FIND:
			fmt.Fprintf(&ab, "\x1b[%d;%dH\x1b[1;34m>", org.cy+TOP_MARGIN+1, LEFT_MARGIN) //blue
			fmt.Fprintf(&ab, "\x1b[%d;%dH", org.cy+TOP_MARGIN+1, org.cx+LEFT_MARGIN+1)
		case COMMAND_LINE:
			fmt.Fprintf(&ab, "\x1b[%d;%dH", s.textLines+2+TOP_MARGIN, len(org.command_line)+LEFT_MARGIN+1)

		default:
			fmt.Fprintf(&ab, "\x1b[%d;%dH\x1b[1;31m>", org.cy+TOP_MARGIN+1, LEFT_MARGIN)
			// below restores the cursor position based on org.cx and org.cy + margin
			fmt.Fprintf(&ab, "\x1b[%d;%dH", org.cy+TOP_MARGIN+1, org.cx+LEFT_MARGIN+1)
		}
	}

	ab.WriteString("\x1b[0m")   //return to default fg/bg
	ab.WriteString("\x1b[?25h") //shows the cursor
	fmt.Print(ab.String())
}
*/

// used by containers
func (s *Screen) drawPreviewBox() {
	width := s.totaleditorcols - 10
	length := s.textLines - 10
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

/*
func (s *Screen) moveDividerPct(pct int) {
	// note below only necessary if window resized or font size changed
	s.textLines = s.screenLines - 2 - TOP_MARGIN

	if pct == 100 {
		s.divider = 1
	} else {
		s.divider = s.screenCols - pct*s.screenCols/100
	}
	s.totaleditorcols = s.screenCols - s.divider - 2
	s.eraseScreenRedrawLines()

	if s.divider > 10 {
		org.refreshScreen()
		org.drawStatusBar()
	}

	if sess.editorMode {
		s.positionWindows()
		s.eraseRightScreen() //erases editor area + statusbar + msg
		s.drawRightScreen()
	} else if org.view == TASK {
		org.drawPreview()
	}
	sess.showOrgMessage("rows: %d  cols: %d  divider: %d", s.screenLines, s.screenCols, s.divider)

	s.returnCursor()
}
*/

func (s *Screen) moveDividerAbs(num int) {
	if num >= s.screenCols {
		s.divider = 1
	} else if num < 20 {
		s.divider = s.screenCols - 20
	} else {
		s.divider = s.screenCols - num
	}

	s.edPct = 100 - 100*s.divider/s.screenCols
	s.totaleditorcols = s.screenCols - s.divider - 2
	s.eraseScreenRedrawLines()

	if s.divider > 10 {
		org.refreshScreen()
		org.drawStatusBar()
	}

	if sess.editorMode {
		s.positionWindows()
		s.eraseRightScreen() //erases editor area + statusbar + msg
		s.drawRightScreen()
	} else if org.view == TASK {
		org.drawPreview()
	}
	app.Organizer.ShowMessage(BL, "rows: %d  cols: %d  divider: %d edPct: %d", s.screenLines, s.screenCols, s.divider, s.edPct)

	app.returnCursor()
}


func (s *Screen) signalHandler() {
	err := s.GetWindowSize()
	if err != nil {
		//SafeExit(fmt.Errorf("couldn't get window size: %v", err))
		os.Exit(1)
	}
	app.moveDividerPct(s.edPct)
}
