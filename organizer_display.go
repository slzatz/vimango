package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
)

var (
	googleDriveRegex  = regexp.MustCompile(`!\[([^\]]*)\]\((https://drive\.google\.com/file/d/[^)]+)\)`)
	timeKeywordsRegex = regexp.MustCompile(`seconds|minutes|hours|days`)
)

// should probably be named drawOrgRows
func (o *Organizer) refreshScreen() {
	var ab strings.Builder
	titlecols := o.Screen.divider - TIME_COL_WIDTH - LEFT_MARGIN

	ab.WriteString("\x1b[?25l") //hides the cursor

	//Below erase screen from middle to left - `1K` below is cursor to left erasing
	//Now erases time/sort column (+ 17 in line below)
	for j := TOP_MARGIN; j < o.Screen.textLines+1; j++ {
		// Use 1K to clear from cursor to start of line, preserving vertical lines
		fmt.Fprintf(&ab, "\x1b[%d;%dH\x1b[1K", j+TOP_MARGIN, titlecols+LEFT_MARGIN+17)
	}
	//	}
	// put cursor at upper left after erasing
	ab.WriteString(fmt.Sprintf("\x1b[%d;%dH", TOP_MARGIN+1, LEFT_MARGIN+1))
	fmt.Print(ab.String())
	if o.taskview == BY_FIND {
		o.drawSearchRows()
		//o.drawActive() //////////////////////////
	} else {
		o.drawRows()
	}
}

func (o *Organizer) drawActiveRow(ab *strings.Builder) {
	// When drawing the current row there are only two things
	// we need to deal with: 1) horizontal scrolling and 2) visual mode highlighting

	titlecols := o.Screen.divider - TIME_COL_WIDTH - LEFT_MARGIN
	y := o.fr - o.rowoff
	row := &o.rows[o.fr]

	if o.coloff > 0 {
		fmt.Fprintf(ab, "\x1b[%d;%dH\x1b[1K\x1b[%dG", y+TOP_MARGIN+1, titlecols+LEFT_MARGIN+1, LEFT_MARGIN+1)
		length := len(row.title) - o.coloff
		if length > titlecols {
			length = titlecols
		}
		if length < 0 {
			length = 0
		}
		beg := o.coloff
		if len(row.title[beg:]) > length {
			ab.WriteString(row.title[beg : beg+length])
		} else {
			ab.WriteString(row.title[beg:])
		}
	}

	if o.mode == VISUAL {
		var j, k int
		if o.highlight[1] > o.highlight[0] {
			j, k = 0, 1
		} else {
			k, j = 0, 1
		}

		fmt.Fprintf(ab, "\x1b[%d;%dH\x1b[1K\x1b[%dG", y+TOP_MARGIN+1, titlecols+LEFT_MARGIN+1, LEFT_MARGIN+1)
		ab.WriteString(row.title[o.coloff : o.highlight[j]-o.coloff])
		ab.WriteString(LIGHT_GRAY_BG)
		ab.WriteString(row.title[o.highlight[j] : o.highlight[k]-o.coloff])
		ab.WriteString(RESET)
		ab.WriteString(row.title[o.highlight[k]:])
	}
}

func (o *Organizer) appendStandardRow(ab *strings.Builder, fr, y, titlecols int) {
	row := &o.rows[fr]
	// position cursor -note that you don't use lf/cr to position lines
	fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, LEFT_MARGIN+1)

	note := o.Database.readNoteIntoString(row.id)
	if googleDriveRegex.MatchString(note) {
		ab.WriteString(BOLD)
	}
	if timeKeywordsRegex.MatchString(row.sort) {
		ab.WriteString(CYAN)
	} else {
		ab.WriteString(WHITE)
	}

	if row.archived && row.deleted {
		ab.WriteString(GREEN)
	} else if row.archived {
		ab.WriteString(YELLOW)
	} else if row.deleted {
		ab.WriteString(RED)
	}

	if row.dirty {
		ab.WriteString(BLACK + WHITE_BG)
	}
	if _, ok := o.marked_entries[row.id]; ok {
		ab.WriteString(BLACK + YELLOW_BG)
	}

	if len(row.title) > titlecols {
		ab.WriteString(row.title[:titlecols])
	} else {
		ab.WriteString(row.title)
		ab.WriteString(strings.Repeat(" ", titlecols-len(row.title)))
	}

	ab.WriteString(RESET)
	ab.WriteString(WHITE)
	sortX := o.Screen.divider - TIME_COL_WIDTH + 2
	width := o.Screen.divider - sortX
	if width > 0 {
		fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, sortX)
		ab.WriteString(strings.Repeat(" ", width))
		fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, sortX)
		if len(row.sort) > width {
			ab.WriteString(row.sort[:width])
		} else {
			ab.WriteString(row.sort)
		}
	}
	ab.WriteString(RESET)
}

func (o *Organizer) appendSearchRow(ab *strings.Builder, fr, y, titlecols int) {
	row := &o.rows[fr]

	fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, LEFT_MARGIN+1)

	note := o.Database.readNoteIntoString(row.id)
	if googleDriveRegex.MatchString(note) {
		ab.WriteString(BOLD)
	}
	if timeKeywordsRegex.MatchString(row.sort) {
		ab.WriteString(CYAN)
	} else {
		ab.WriteString(WHITE)
	}

	if row.archived && row.deleted {
		ab.WriteString(GREEN)
	} else if row.archived {
		ab.WriteString(YELLOW)
	} else if row.deleted {
		ab.WriteString(RED)
	}

	if row.dirty {
		ab.WriteString(BLACK + WHITE_BG)
	}
	if _, ok := o.marked_entries[row.id]; ok {
		ab.WriteString(BLACK + YELLOW_BG)
	}

	if len(row.title) <= titlecols {
		ab.WriteString(row.ftsTitle)
	} else {
		pos := strings.Index(row.ftsTitle, "\x1b[49m")
		if pos > 0 && pos < titlecols+11 && len(row.ftsTitle) >= titlecols+15 {
			ab.WriteString(row.ftsTitle[:titlecols+15])
		} else {
			ab.WriteString(row.title[:titlecols])
		}
	}

	length := len(row.title)
	if length > titlecols {
		length = titlecols
	}
	if spaces := titlecols - length; spaces > 0 {
		ab.WriteString(strings.Repeat(" ", spaces))
	}
	ab.WriteString(RESET)

	sortX := o.Screen.divider - TIME_COL_WIDTH + 2
	width := o.Screen.divider - sortX
	if width > 0 {
		fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, sortX)
		ab.WriteString(strings.Repeat(" ", width))
		fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, sortX)
		if len(row.sort) > width {
			ab.WriteString(row.sort[:width])
		} else {
			ab.WriteString(row.sort)
		}
	}
	ab.WriteString(RESET)
}

func (o *Organizer) drawRows() {
	if len(o.rows) == 0 {
		return
	}
	var ab strings.Builder
	titlecols := o.Screen.divider - TIME_COL_WIDTH - LEFT_MARGIN

	for y := 0; y < o.Screen.textLines; y++ {
		fr := y + o.rowoff
		if fr > len(o.rows)-1 {
			break
		}
		o.appendStandardRow(&ab, fr, y, titlecols)
	}
	// instances that require a full redraw do not require separate drawing of active row
	//o.drawActiveRow(&ab)

	ab.WriteString(RESET)
	fmt.Print(ab.String())
}

// for drawing containers when making a selection
func (o *Organizer) drawAltRows() {

	if len(o.altRows) == 0 {
		return
	}

	var ab strings.Builder
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+1, o.Screen.divider+2)
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", o.Screen.divider+1)

	for y := 0; y < o.Screen.textLines; y++ {

		fr := y + o.altRowoff
		if fr > len(o.altRows)-1 {
			break
		}

		length := len(o.altRows[fr].title)
		if length > o.Screen.totaleditorcols {
			length = o.Screen.totaleditorcols
		}

		if o.altRows[fr].star {
			ab.WriteString("\x1b[1m") //bold
			ab.WriteString("\x1b[1;36m")
		}

		if fr == o.altFr {
			ab.WriteString("\x1b[48;5;236m") // 236 is a grey
		}

		ab.WriteString(o.altRows[fr].title[:length])
		ab.WriteString("\x1b[0m") // return background to normal
		ab.WriteString(lf_ret)
	}
	fmt.Print(ab.String())
}

func (o *Organizer) drawRenderedNote() {
	if len(o.note) == 0 {
		return
	}
	start := o.altRowoff
	var end int
	// check if there are more lines than can fit on the screen
	if len(o.note)-start > o.Screen.textLines-1 {
		end = o.Screen.textLines + start
	} else {
		//end = len(o.note) - 1
		end = len(o.note)
	}
	fmt.Fprintf(os.Stdout, "\x1b[%d;%dH", TOP_MARGIN+1, o.Screen.divider+1)
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", o.Screen.divider+0)
	fmt.Print(strings.Join(o.note[start:end], lf_ret))
	fmt.Print(RESET) //sometimes there is an unclosed escape sequence
}

func (o *Organizer) drawStatusBar() {

	var ab strings.Builder
	//position cursor and erase - and yes you do have to reposition cursor after erase
	fmt.Fprintf(&ab, "\x1b[%d;%dH\x1b[1K\x1b[%d;1H", o.Screen.textLines+TOP_MARGIN+1, o.Screen.divider, o.Screen.textLines+TOP_MARGIN+1)
	ab.WriteString("\x1b[7m") //switches to reversed colors

	var str string
	var id int
	var title string
	var keywords string
	if len(o.rows) > 0 {
		switch o.view {
		case TASK:
			id = o.getId()
			switch o.taskview {
			case BY_FIND:
				str = "search - " + o.Session.fts_search_terms
			case BY_FOLDER:
				str = fmt.Sprintf("%s[f] (%s[c])", o.filter, o.Database.taskContext(id))
			case BY_CONTEXT:
				str = fmt.Sprintf("%s[c] (%s[f])", o.filter, o.Database.taskFolder(id))
			case BY_RECENT:
				str = fmt.Sprintf("Recent: %s[c] %s[f]",
					o.Database.taskContext(id), o.Database.taskFolder(id))
			case BY_KEYWORD:
				str = o.filter + "[k]"
			}
		case CONTEXT:
			str = "Contexts"
		case FOLDER:
			str = "Folders"
		case KEYWORD:
			str = "Keywords"
		//case SYNC_LOG_VIEW:
		//	str = "Sync Log"
		default:
			str = "Other"
		}

		row := &o.rows[o.fr]

		if len(row.title) > 16 {
			title = row.title[:12] + "..."
		} else {
			title = row.title
		}

		id = row.id

		if o.view == TASK {
			keywords = o.Database.getTaskKeywords(row.id)
		}
	} else {
		title = "   No Results   "
		id = -1

	}

	// [49m - revert background to normal
	// 7m - reverses video
	// because video is reversted [42 sets text to green and 49 undoes it
	// also [0;35;7m -> because of 7m it reverses background and foreground
	// [0;7m is revert text to normal and reverse video
	status := fmt.Sprintf("\x1b[1m%s\x1b[0;7m %s \x1b[0;35;7m%s\x1b[0;7m %d %d/%d \x1b[1;42m%%s\x1b[0;7m sort: %s ",
		str, title, keywords, id, o.fr+1, len(o.rows), o.sort)

	// klugy way of finding length of string without the escape characters
	plain := fmt.Sprintf("%s %s %s %d %d/%d   sort: %s ",
		str, title, keywords, id, o.fr+1, len(o.rows), o.sort)
	length := len(plain)

	if length+len(fmt.Sprintf("%s", o.mode)) <= o.Screen.divider {
		/*
			s := fmt.Sprintf("%%-%ds", o.divider-length) // produces "%-25s"
			t := fmt.Sprintf(s, o.mode)
			fmt.Fprintf(&ab, status, t)
		*/
		fmt.Fprintf(&ab, status, fmt.Sprintf(fmt.Sprintf("%%-%ds", o.Screen.divider-length), o.mode))
	} else {
		status = fmt.Sprintf("\x1b[1m%s\x1b[0;7m %s \x1b[0;35;7m%s\x1b[0;7m %d %d/%d\x1b[49m",
			str, title, keywords, id, o.fr+1, len(o.rows))
		plain = fmt.Sprintf("%s %s %s %d %d/%d",
			str, title, keywords, id, o.fr+1, len(o.rows))
		length := len(plain)
		if length < o.Screen.divider {
			fmt.Fprintf(&ab, "%s%-*s", status, o.Screen.divider-length, " ")
		} else {
			status = fmt.Sprintf("\x1b[1m%s\x1b[0;7m %s %s %d %d/%d",
				str, title, keywords, id, o.fr+1, len(o.rows))
			ab.WriteString(status[:o.Screen.divider+10])
		}
	}
	ab.WriteString("\x1b[0m") //switches back to normal formatting
	fmt.Print(ab.String())
}

func (o *Organizer) drawSearchRows() {
	if len(o.rows) == 0 {
		return
	}
	var ab strings.Builder
	titlecols := o.Screen.divider - TIME_COL_WIDTH - LEFT_MARGIN

	for y := 0; y < o.Screen.textLines; y++ {
		fr := y + o.rowoff
		if fr > len(o.rows)-1 {
			break
		}
		o.appendSearchRow(&ab, fr, y, titlecols)
	}
	fmt.Print(ab.String())
}

// should be removed after more testing
func (o *Organizer) drawRowAt(fr int) {
	if fr < 0 || fr >= len(o.rows) {
		return
	}
	if fr < o.rowoff || fr >= o.rowoff+o.Screen.textLines {
		return
	}
	titlecols := o.Screen.divider - TIME_COL_WIDTH - LEFT_MARGIN
	y := fr - o.rowoff
	var ab strings.Builder
	if o.taskview == BY_FIND {
		o.appendSearchRow(&ab, fr, y, titlecols)
	} else {
		o.appendStandardRow(&ab, fr, y, titlecols)
	}
	fmt.Print(ab.String())
}

func (o *Organizer) drawActive() {
	// When doing a partial redraw, we first draw the standard row, then
	// address horizontal scrolling and visual mode highlighting
	titlecols := o.Screen.divider - TIME_COL_WIDTH - LEFT_MARGIN
	y := o.fr - o.rowoff
	var ab strings.Builder
	o.appendStandardRow(&ab, o.fr, y, titlecols)
	o.drawActiveRow(&ab)
	fmt.Print(ab.String())
}

func (o *Organizer) erasePreviousRowMarker(prevRow int) {
	y := prevRow - o.rowoff
	fmt.Fprintf(os.Stdout, "\x1b[%d;%dH ", y+TOP_MARGIN+1, 0)
}

// change function name to displayRenderedNote
func (o *Organizer) drawPreview() {
	if len(o.rows) == 0 {
		o.Screen.eraseRightScreen()
		return
	}
	id := o.rows[o.fr].id
	var note string
	if o.taskview != BY_FIND {
		note = o.Database.readNoteIntoString(id)
	} else {
		note = o.Database.highlightTerms2(id)
	}
	o.Screen.eraseRightScreen()

	var lang string
	if o.Database.taskFolder(id) == "code" {
		c := o.Database.taskContext(id)
		var ok bool
		if lang, ok = Languages[c]; !ok {
			lang = "markdown"
		}
	} else {
		lang = "markdown"
	}

	if lang == "markdown" {
		o.renderMarkdown(note)
	} else {
		o.renderCode(note, lang)
	}
	o.drawRenderedNote()
}

func (o *Organizer) renderMarkdown(s string) {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("darkslz.json"),
		glamour.WithWordWrap(0),
	)
	note, _ := r.Render(s)
	// glamour seems to add a '\n' at the start
	note = strings.TrimSpace(note)

	if o.taskview == BY_FIND {
		// could use strings.Count to make sure they are balanced
		note = strings.ReplaceAll(note, "qx", "\x1b[48;5;31m") //^^
		note = strings.ReplaceAll(note, "qy", "\x1b[0m")       // %%
	}
	note = WordWrap(note, o.Screen.totaleditorcols-20)
	o.note = strings.Split(note, "\n")
}

func (o *Organizer) renderCode(s string, lang string) {
	var buf bytes.Buffer
	_ = Highlight(&buf, s, lang, "terminal16m", o.Session.style[o.Session.styleIndex])
	note := buf.String()

	if o.taskview == BY_FIND {
		// could use strings.Count to make sure they are balanced
		note = strings.ReplaceAll(note, "qx", "\x1b[48;5;31m") //^^
		note = strings.ReplaceAll(note, "qy", "\x1b[0m")       // %%
	}
	note = WordWrap(note, o.Screen.totaleditorcols)
	o.note = strings.Split(note, "\n")
}

func (o *Organizer) drawNotice(s string) {
	o.renderNotice(s)
	if len(o.notice) == 0 {
		return
	}
	o.drawNoticeLayer()
	o.drawNoticeText()
}

func (o *Organizer) renderNotice(s string) {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("darkslz.json"),
		glamour.WithWordWrap(0),
	)
	note, _ := r.Render(s)
	// glamour seems to add a '\n' at the start
	note = strings.TrimSpace(note)

	note = WordWrap(note, o.Screen.totaleditorcols-20)
	o.notice = strings.Split(note, "\n")
}

func (o *Organizer) drawNoticeLayer() {
	var ab strings.Builder
	width := o.Screen.totaleditorcols - 10
	length := len(o.notice)
	if length > o.Screen.textLines-10 {
		length = o.Screen.textLines - 10
	}

	// \x1b[NC moves cursor forward by N columns
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", o.Screen.divider+6)

	//hide the cursor
	ab.WriteString("\x1b[?25l")
	// move the cursor
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, o.Screen.divider+7)

	//erase set number of chars on each line
	erase_chars := fmt.Sprintf("\x1b[%dX", o.Screen.totaleditorcols-10)
	for i := 0; i < length; i++ {
		ab.WriteString(erase_chars)
		ab.WriteString(lf_ret)
	}

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, o.Screen.divider+7)

	// \x1b[ 2*x is DECSACE to operate in rectable mode
	// \x1b[%d;%d;%d;%d;48;5;235$r is DECCARA to apply specified attributes (background color 235) to rectangle area
	// \x1b[ *x is DECSACE to exit rectangle mode
	fmt.Fprintf(&ab, "\x1b[2*x\x1b[%d;%d;%d;%d;48;5;235$r\x1b[*x",
		TOP_MARGIN+6, o.Screen.divider+7, TOP_MARGIN+4+length, o.Screen.divider+7+width)
	ab.WriteString("\x1b[48;5;235m") //draws the box lines with same background as above rectangle
	fmt.Print(ab.String())
	o.drawNoticeBox()
}

func (o *Organizer) drawNoticeBox() {
	width := o.Screen.totaleditorcols - 10
	length := len(o.notice) + 1
	if length > o.Screen.textLines-9 {
		length = o.Screen.textLines - 9
	}
	var ab strings.Builder
	move_cursor := fmt.Sprintf("\x1b[%dC", width)

	ab.WriteString("\x1b(0") // Enter line drawing mode
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5, o.Screen.divider+6)
	ab.WriteString("\x1b[37;1ml") //upper left corner

	for i := 1; i < length; i++ {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5+i, o.Screen.divider+6)
		// x=0x78 vertical line (q=0x71 is horizontal) 37=white; 1m=bold (only need 1 m)
		ab.WriteString("\x1b[37;1mx")
		ab.WriteString(move_cursor)
		ab.WriteString("\x1b[37;1mx")
	}

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+4+length, o.Screen.divider+6)
	ab.WriteString("\x1b[1B")
	ab.WriteString("\x1b[37;1mm") //lower left corner

	move_cursor = fmt.Sprintf("\x1b[1D\x1b[%dB", length)

	for i := 1; i < width+1; i++ {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5, o.Screen.divider+6+i)
		ab.WriteString("\x1b[37;1mq")
		ab.WriteString(move_cursor)
		ab.WriteString("\x1b[37;1mq")
	}

	ab.WriteString("\x1b[37;1mj") //lower right corner
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5, o.Screen.divider+7+width)
	ab.WriteString("\x1b[37;1mk") //upper right corner

	//exit line drawing mode
	ab.WriteString("\x1b(B")
	ab.WriteString("\x1b[0m")
	ab.WriteString("\x1b[?25h")
	fmt.Print(ab.String())
}

func (o *Organizer) drawNoticeText() {
	length := len(o.notice)
	if length > o.Screen.textLines-10 {
		length = o.Screen.textLines - 10
	}
	start := o.altRowoff
	var end int
	// check if there are more lines than can fit on the screen
	if len(o.notice)-start > length {
		end = length + start - 2
	} else {
		//end = len(o.note) - 1
		end = len(o.notice)
	}
	fmt.Fprintf(os.Stdout, "\x1b[%d;%dH", TOP_MARGIN+6, o.Screen.divider+7)
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", o.Screen.divider+7)
	fmt.Print(strings.Join(o.notice[start:end], lf_ret))
	fmt.Print(RESET) //sometimes there is an unclosed escape sequence
}
