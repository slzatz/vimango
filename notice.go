package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
)

// The notice: a box drawn over the right pane for text that is not a note --
// :help, a sync report, a note's info, the list a context is picked from, the
// output of a vim command that lists something. It is the screen's, not the
// organizer's: the editor shows its :help in one too, and in --editor mode
// there is no organizer.
//
// A notice is markdown, rendered and wrapped when it is shown; noticeTop is
// the first of its lines on screen. Showing a notice starts it at the top.
// Whoever shows one is in a mode whose keys scroll it (the organizer's HELP,
// NAVIGATE_NOTICE and CONTAINER, the editor's HELP), and Escape redraws what
// was underneath.

// showNotice renders text and draws it. hang is how far a wrapped line is
// indented under its first: :help's entries are a name and a description, and
// the description continues under itself.
func (s *Screen) showNotice(text string, hang int) {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath(glamourStylePath(s.glamourStyle)),
		glamour.WithWordWrap(0),
	)
	note, _ := r.Render(text)
	// glamour seems to add a '\n' at the start
	note = strings.TrimSpace(note)
	// Decode any Kitty text sizing markers (OSC 66)
	note = ansi.DecodeKittyTextSizeMarkers(note)
	note = WordWrap(note, s.totaleditorcols-NOTICE_RIGHT_PADDING, hang)
	s.notice = strings.Split(note, "\n")
	s.noticeTop = 0
	s.drawNotice()
}

// drawNotice draws the notice as it stands, from noticeTop.
func (s *Screen) drawNotice() {
	if len(s.notice) == 0 {
		return
	}
	s.drawNoticeLayer()
	s.drawNoticeText()
}

// noticeRows is how many lines of a notice drawNoticeText shows: all of a
// notice that fits in its box, otherwise the box less two rows.
func (s *Screen) noticeRows() int {
	box := s.textLines - 10
	if len(s.notice) <= box {
		return len(s.notice)
	}
	return max(box-2, 1)
}

// The scrolling of a notice. Each returns what there is to say about it on the
// message line, or "" -- where that line is belongs to whoever is showing the
// notice.

const noticeEnd = "Reached end of rendered note"

func (s *Screen) noticePosition() string {
	return fmt.Sprintf("Line %d of %d", s.noticeTop, len(s.notice))
}

// scrollNoticeDown and scrollNoticeUp move a notice by a line.
func (s *Screen) scrollNoticeDown() string {
	if len(s.notice) == 0 {
		return ""
	}
	if s.noticeTop >= len(s.notice)-2 {
		return noticeEnd
	}
	s.noticeTop++
	s.drawNotice()
	return s.noticePosition()
}

func (s *Screen) scrollNoticeUp() string {
	if len(s.notice) == 0 || s.noticeTop == 0 {
		return ""
	}
	s.noticeTop--
	s.drawNotice()
	return ""
}

// pageNoticeDown and pageNoticeUp move a notice by half of what is showing;
// down stops once its last line is in the box.
func (s *Screen) pageNoticeDown() string {
	if len(s.notice) == 0 {
		return ""
	}
	last := len(s.notice) - s.noticeRows()
	if s.noticeTop >= last {
		return noticeEnd
	}
	s.noticeTop = min(s.noticeTop+max(s.noticeRows()/2, 1), last)
	s.drawNotice()
	return ""
}

func (s *Screen) pageNoticeUp() string {
	if len(s.notice) == 0 || s.noticeTop == 0 {
		return ""
	}
	s.noticeTop = max(s.noticeTop-max(s.noticeRows()/2, 1), 0)
	s.drawNotice()
	return ""
}

// scrollNoticeHome goes back to a notice's first line.
func (s *Screen) scrollNoticeHome() string {
	if len(s.notice) == 0 {
		return ""
	}
	s.noticeTop = 0
	s.drawNotice()
	return s.noticePosition()
}

// noticeKey is the keys that scroll a notice, the same wherever one is shown:
// Ctrl-J and Ctrl-K by a line, Space or PgDn and b or PgUp by half a page (as
// in the rendered note), Home to the top. handled is false for any other key.
func (s *Screen) noticeKey(c int) (message string, handled bool) {
	switch c {
	case ctrlKey('j'):
		return s.scrollNoticeDown(), true
	case ctrlKey('k'):
		return s.scrollNoticeUp(), true
	case HOME_KEY:
		return s.scrollNoticeHome(), true
	case ' ', PAGE_DOWN:
		return s.pageNoticeDown(), true
	case 'b', PAGE_UP:
		return s.pageNoticeUp(), true
	}
	return "", false
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

func (s *Screen) drawNoticeText() {
	length := len(s.notice)
	if length > s.textLines-10 {
		length = s.textLines - 10
	}
	start := s.noticeTop
	var end int
	// check if there are more lines than can fit on the screen
	if len(s.notice)-start > length {
		end = length + start - 2
	} else {
		end = len(s.notice)
	}
	fmt.Fprintf(os.Stdout, "\x1b[%d;%dH", TOP_MARGIN+6, s.divider+8)
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", s.divider+7)
	fmt.Print(strings.Join(s.notice[start:end], lf_ret))
	fmt.Print(RESET) //sometimes there is an unclosed escape sequence
}
