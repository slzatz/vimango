package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/slzatz/vimango/internal/editor"
	"github.com/slzatz/vimango/rawmode"
)

type Screen struct {
	screenCols      int
	screenLines     int //total number of screen lines
	textLines       int // considering margins, bottom messages
	divider         int
	totaleditorcols int
	edPct           int             // percent that editor space takes up of whole horiz screen real estate
	ws              rawmode.Winsize //Row,Col,Xpixel,Ypixel unint16
	Session         *Session

	// the notice: text shown in a box over the right pane (notice.go)
	notice       []string // its lines, rendered and wrapped
	noticeTop    int      // the first of them on screen
	glamourStyle string   // the style file it is rendered with, as config.json names it
}

func (s *Screen) eraseScreenRedrawLines() {
	// editor-only mode: no top border row (the host's native title bar
	// sits directly above, and the text area starts at row 1) — just the
	// vertical left border, full height.
	if s.Session.editorOnly {
		fmt.Fprint(os.Stdout, "\x1b[2J")
		fmt.Fprint(os.Stdout, "\x1b(0")
		for j := 1; j <= s.screenLines; j++ {
			fmt.Fprintf(os.Stdout, "\x1b[%d;%dH\x1b[37;1mx", j, s.divider)
		}
		fmt.Fprint(os.Stdout, "\x1b[0m")
		fmt.Fprint(os.Stdout, "\x1b(B")
		return
	}

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
	// (skipped in editor-only mode: there is no top border row — the
	// text area starts at row 1 and this would overwrite it)
	if !s.Session.editorOnly {
		ab.WriteString("\x1b(0")                   // Enter line drawing mode
		for j := 1; j < s.totaleditorcols+1; j++ { //added +1 0906/2020
			fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN, s.divider+j)
			// below x = 0x78 vertical line (q = 0x71 is horizontal) 37 = white;
			// 1m = bold (note only need one 'm'
			ab.WriteString("\x1b[37;1mq")
		}

		//exit line drawing mode
		ab.WriteString("\x1b(B")
	}

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+1, s.divider+2)
	ab.WriteString("\x1b[0m") // needed or else in bold mode from line drawing above

	fmt.Fprint(os.Stdout, ab.String())
}

func (s *Screen) drawRightScreen() {
	if e := s.Session.editor; e != nil {
		e.Draw()
	}
}

// positionEditor gives the open editor its layout: when it is opened, and
// again whenever the terminal's size or the divider changes.
func (s *Screen) positionEditor() {
	if e := s.Session.editor; e != nil {
		e.Layout = s.editorLayout()
	}
}

// editorLayout is where an editor draws on this screen: from the divider to
// the right edge, with its message line on the last row. This is the one
// place that knows the two ways vimango shows an editor.
//
// In the TUI the editor is the right pane: a border along the top, the text,
// a status bar, the message line. What it says beside the note goes under
// the organizer.
//
// In --editor mode it has the whole of a host app's window (the divider is
// column 1), and the host has a title bar above and a status bar below. So
// there is no top border -- the text starts on row 1, and there is no row for
// a corner -- and no status bar: the row it would have is left blank, between
// the text and the message line, and the text gains the border's row. With
// nothing beside the editor, everything it says goes on its message line.
func (s *Screen) editorLayout() editor.Layout {
	message := editor.MessageLine{Row: s.textLines + 2 + TOP_MARGIN, Col: s.divider + 1, Width: s.screenCols - s.divider}
	l := editor.Layout{
		LeftMargin: s.divider,
		ScreenCols: s.screenCols - s.divider - 1,
		Message:    message,
	}
	if s.Session.editorOnly {
		l.TopMargin = 1
		l.ScreenLines = s.textLines + 1
		l.Aside = message
		return l
	}
	l.TopMargin = TOP_MARGIN + 1
	l.ScreenLines = s.textLines
	l.StatusBar = true
	l.Corner = true
	l.Aside = editor.MessageLine{Row: message.Row, Col: 1, Width: s.divider}
	return l
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
