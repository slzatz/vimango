package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"strings"

	"github.com/slzatz/vimango/rawmode"
	"golang.org/x/sys/unix"
	"github.com/alecthomas/chroma/v2"
)

type Session struct {
	screenCols       int
	screenLines      int //total number of screen lines
	textLines        int // considering margins, bottom messages
	divider          int
	totaleditorcols  int
	initialFileRow   int
	temporaryTID     int
	run              bool
	editorMode       bool
	imagePreview     bool
	imgSizeY         int
	fts_search_terms string
	origTermCfg      []byte //from GoKilo
	//cfg              Config
	edPct      int // percent that editor space takes up of whole horiz screen real estate
	style      [8]string
  markdown_style *chroma.Style
	styleIndex int
	ws         unix.Winsize //Row,Col,Xpixel,Ypixel unint16
	//images           map[string]*image.Image
}

/*
type Config struct {
		user     string
		password string
		dbname   string
		hostaddr string
		port     int
	ed_pct int
}
*/

func (s *Session) numberOfEditors() int {
	i := 0
	for _, w := range appCtx.Windows {
		if _, ok := w.(*Editor); ok {
			i++
		}
	}
	return i
}

func (s *Session) editors() []*Editor {
	eds := []*Editor{}
	for _, w := range appCtx.Windows {
		if e, ok := w.(*Editor); ok {
			eds = append(eds, e)
		}
	}
	return eds
}

func (s *Session) eraseScreenRedrawLines() {
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

func (s *Session) eraseRightScreen() {
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

func (s *Session) drawRightScreen() {
	for _, w := range appCtx.Windows {
		w.drawText()
		w.drawFrame()
		w.drawStatusBar()
	}
}

func (s *Session) positionWindows() {
	windowSlots := 0
	for _, w := range appCtx.Windows {
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
	for _, w := range appCtx.Windows {
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

func (s *Session) GetWindowSize() error {

	ws, err := rawmode.GetWindowSize()
	if err != nil {
		return err
	}

	s.screenCols = int(ws.Col)
	s.screenLines = int(ws.Row)
	s.ws = *ws

	return nil
}

func (s *Session) enableRawMode() ([]byte, error) {

	// Gets TermIOS data structure. From glibc, we find the cmd should be TCGETS
	// https://code.woboq.org/userspace/glibc/sysdeps/unix/sysv/linux/tcgetattr.c.html
	termios, err := unix.IoctlGetTermios(unix.Stdin, unix.TCGETS)
	if err != nil {
		return nil, fmt.Errorf("error fetching existing console settings: %w", err)
	}

	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(termios); err != nil {
		return nil, fmt.Errorf("error serializing existing console settings: %w", err)
	}

	// turn off echo & canonical mode by using a bitwise clear operator &^
	termios.Lflag = termios.Lflag &^ (unix.ECHO | unix.ICANON | unix.ISIG | unix.IEXTEN)
	termios.Iflag = termios.Iflag &^ (unix.IXON | unix.ICRNL | unix.BRKINT | unix.INPCK | unix.ISTRIP)
	termios.Oflag = termios.Oflag &^ (unix.OPOST)
	termios.Cflag = termios.Cflag | unix.CS8
	//termios.Cc[unix.VMIN] = 0
	//termios.Cc[unix.VTIME] = 1
	// from the code of tcsetattr in glibc, we find that for TCSAFLUSH,
	// the corresponding command is TCSETSF
	// https://code.woboq.org/userspace/glibc/sysdeps/unix/sysv/linux/tcsetattr.c.html
	if err := unix.IoctlSetTermios(unix.Stdin, unix.TCSETSF, termios); err != nil {
		return buf.Bytes(), err
	}

	return buf.Bytes(), nil
}

func Restore(original []byte) error {

	var termios unix.Termios

	if err := gob.NewDecoder(bytes.NewReader(original)).Decode(&termios); err != nil {
		return fmt.Errorf("error decoding terminal settings: %w", err)
	}

	if err := unix.IoctlSetTermios(unix.Stdin, unix.TCSETSF, &termios); err != nil {
		return fmt.Errorf("error restoring original console settings: %w", err)
	}
	return nil
}

func (s *Session) showOrgMessage(format string, a ...interface{}) {
	fmt.Printf("\x1b[%d;%dH\x1b[1K\x1b[%d;1H", s.textLines+2+TOP_MARGIN, s.divider, s.textLines+2+TOP_MARGIN)
	str := fmt.Sprintf(format, a...)
	if len(str) > s.divider {
		str = str[:s.divider]
	}
	fmt.Print(str)
}

func (s *Session) showEdMessage(format string, a ...interface{}) {
	fmt.Printf("\x1b[%d;%dH\x1b[K", s.textLines+2+TOP_MARGIN, s.divider+1)
	str := fmt.Sprintf(format, a...)

	cols := s.screenCols - s.divider
	if len(str) > cols {
		str = str[:cols]
	}
	fmt.Print(str)
}

func (s *Session) returnCursor() {
	var ab strings.Builder
	if s.editorMode {
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

func (s *Session) displayEntryInfo(e *NewEntry) {
	var ab strings.Builder
	width := s.totaleditorcols - 10
	length := s.textLines - 10

	// \x1b[NC moves cursor forward by N columns
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", s.divider+6)

	//hide the cursor
	ab.WriteString("\x1b[?25l")
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, s.divider+7)

	//erase set number of chars on each line
	erase_chars := fmt.Sprintf("\x1b[%dX", s.totaleditorcols-10)
	for i := 0; i < length-1; i++ {
		ab.WriteString(erase_chars)
		ab.WriteString(lf_ret)
	}

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, s.divider+7)

	fmt.Fprintf(&ab, "\x1b[2*x\x1b[%d;%d;%d;%d;48;5;235$r\x1b[*x",
		TOP_MARGIN+6, s.divider+7, TOP_MARGIN+4+length, s.divider+7+width)
	ab.WriteString("\x1b[48;5;235m") //draws the box lines with same background as above rectangle

	fmt.Fprintf(&ab, "id: %d%s", e.id, lf_ret)
	fmt.Fprintf(&ab, "tid: %d%s", e.tid, lf_ret)

	title := fmt.Sprintf("title: %s", e.title)
	if len(title) > width {
		title = title[:width-3] + "..."
	}
	//coloring labels will take some work b/o gray background
	//s.append(fmt::format("{}title:{} {}{}", COLOR_1, "\x1b[m", title, lf_ret));
	fmt.Fprintf(&ab, "%s%s", title, lf_ret)

	context := filterTitle("context", e.context_tid)
	fmt.Fprintf(&ab, "context: %s%s", context, lf_ret)

	folder := filterTitle("folder", e.folder_tid)
	fmt.Fprintf(&ab, "folder: %s%s", folder, lf_ret)

	fmt.Fprintf(&ab, "star: %t%s", e.star, lf_ret)
	fmt.Fprintf(&ab, "deleted: %t%s", e.deleted, lf_ret)

	fmt.Fprintf(&ab, "completed: %t%s", e.archived, lf_ret)
	fmt.Fprintf(&ab, "modified: %s%s", e.modified, lf_ret)
	fmt.Fprintf(&ab, "added: %s%s", e.added, lf_ret)

	//fmt.Fprintf(&ab, "keywords: %s%s", getTaskKeywords(getId()), lf_ret)
	fmt.Fprintf(&ab, "keywords: %s%s", getTaskKeywords(e.id), lf_ret)

	fmt.Print(ab.String())
}

// used by containers
func (s *Session) drawPreviewBox() {
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

func (s *Session) displayContainerInfo() {

	/*
		type Container struct {
			id       int
			tid      int
			title    string
			star     bool
			deleted  bool
			modified string
			count    int
		}
	*/
	c := getContainerInfo(org.rows[org.fr].id)

	if c.id == 0 {
		return
	}

	var ab strings.Builder
	width := s.totaleditorcols - 10
	length := s.textLines - 10

	// \x1b[NC moves cursor forward by N columns
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", s.divider+6)

	//hide the cursor
	ab.WriteString("\x1b[?25l")
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, s.divider+7)

	//erase set number of chars on each line
	erase_chars := fmt.Sprintf("\x1b[%dX", s.totaleditorcols-10)
	for i := 0; i < length-1; i++ {
		ab.WriteString(erase_chars)
		ab.WriteString(lf_ret)
	}

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, s.divider+7)

	fmt.Fprintf(&ab, "\x1b[2*x\x1b[%d;%d;%d;%d;48;5;235$r\x1b[*x",
		TOP_MARGIN+6, s.divider+7, TOP_MARGIN+4+length, s.divider+7+width)
	ab.WriteString("\x1b[48;5;235m") //draws the box lines with same background as above rectangle

	//ab.append(COLOR_6); // Blue depending on theme

	fmt.Fprintf(&ab, "id: %d%s", c.id, lf_ret)
	fmt.Fprintf(&ab, "tid: %d%s", c.tid, lf_ret)

	title := fmt.Sprintf("title: %s", c.title)
	if len(title) > width {
		title = title[:width-3] + "..."
	}

	fmt.Fprintf(&ab, "star: %t%s", c.star, lf_ret)
	fmt.Fprintf(&ab, "deleted: %t%s", c.deleted, lf_ret)

	fmt.Fprintf(&ab, "modified: %s%s", c.modified, lf_ret)
	fmt.Fprintf(&ab, "entry count: %d%s", c.count, lf_ret)

	fmt.Print(ab.String())
	sess.drawPreviewBox()
}

func (s *Session) quitApp() {
	//if lsp.name != "" {
	//	shutdownLsp()
	//}
	fmt.Print("\x1b[2J\x1b[H") //clears the screen and sends cursor home
	//sqlite3_close(S.db); //something should probably be done here
	//PQfinish(conn);
	//lsp_shutdown("all");

	if err := rawmode.Restore(s.origTermCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: disabling raw mode: %s\r\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func (s *Session) signalHandler() {
	err := s.GetWindowSize()
	if err != nil {
		//SafeExit(fmt.Errorf("couldn't get window size: %v", err))
		os.Exit(1)
	}
	moveDividerPct(s.edPct)
}
