package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
)

// should probably be named drawOrgRows
func (o *Organizer) refreshScreen() {
	var ab strings.Builder
	titlecols := o.Screen.divider - TIME_COL_WIDTH - LEFT_MARGIN

	ab.WriteString("\x1b[?25l") //hides the cursor

	//Below erase screen from middle to left - `1K` below is cursor to left erasing
	//Now erases time/sort column (+ 17 in line below)
	//if (org.view != KEYWORD) {
	//	if o.mode != ADD_CHANGE_FILTER {
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
		/*
			} else if o.mode == ADD_CHANGE_FILTER {
				o.drawAltRows()
		*/
	} else {
		o.drawRows()
	}
}

func (o *Organizer) drawActiveRow(ab *strings.Builder, y int) {
	var j, k int //to swap highlight if org.highlight[1] < org.highlight[0]
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", LEFT_MARGIN)
	titlecols := o.Screen.divider - TIME_COL_WIDTH - LEFT_MARGIN
	length := len(o.rows[o.fr].title) - o.coloff
	if length > titlecols {
		length = titlecols
	}
	if o.rows[o.fr].star {
		ab.WriteString(CYAN_BOLD)
	}
	if o.rows[o.fr].archived && o.rows[o.fr].deleted {
		ab.WriteString(GREEN)
	} else if o.rows[o.fr].archived {
		ab.WriteString(YELLOW)
	} else if o.rows[o.fr].deleted {
		ab.WriteString(RED)
	}
	ab.WriteString(DARK_GRAY_BG)
	if o.rows[o.fr].dirty {
		ab.WriteString(BLACK + WHITE_BG)
	}
	if o.mode == VISUAL {
		// below in case org.highlight[1] < org.highlight[0]
		if o.highlight[1] > o.highlight[0] {
			j, k = 0, 1
		} else {
			k, j = 0, 1
		}

		ab.WriteString(o.rows[o.fr].title[o.coloff : o.highlight[j]-o.coloff])
		ab.WriteString(LIGHT_GRAY_BG)
		ab.WriteString(o.rows[o.fr].title[o.highlight[j] : o.highlight[k]-o.coloff])

		ab.WriteString(DARK_GRAY_BG)
		ab.WriteString(o.rows[o.fr].title[o.highlight[k]:])

	} else {
		// current row is only row that is scrolled if org.coloff != 0
		beg := o.coloff
		if len(o.rows[o.fr].title[beg:]) > length {
			ab.WriteString(o.rows[o.fr].title[beg : beg+length])
		} else {
			ab.WriteString(o.rows[o.fr].title[beg:])
		}
	}
	// the spaces make it look like the whole row is highlighted
	//note len can't be greater than titlecols so always positive
	ab.WriteString(strings.Repeat(" ", titlecols-length+1))

	// believe the +2 is just to give some space from the end of long titles
	fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, o.Screen.divider-TIME_COL_WIDTH+2)
	ab.WriteString(o.rows[o.fr].sort)
	ab.WriteString(RESET)
	ab.WriteString(lf_ret)
}

func (o *Organizer) drawRows() {

	googleDriveRegex := regexp.MustCompile(`!\[([^\]]*)\]\((https://drive\.google\.com/file/d/[^)]+)\)`)
	timeKeywordsRegex := regexp.MustCompile(`seconds|minutes|hours|days`)
	if len(o.rows) == 0 {
		return
	}

	var j, k int //to swap highlight if org.highlight[1] < org.highlight[0]
	var ab strings.Builder
	titlecols := o.Screen.divider - TIME_COL_WIDTH - LEFT_MARGIN

	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", LEFT_MARGIN)

	for y := 0; y < o.Screen.textLines; y++ {
		fr := y + o.rowoff
		if fr > len(o.rows)-1 {
			break
		}

		// if a line is long you only draw what fits on the screen
		//below solves problem when deleting chars from a scrolled long line
		var length int
		if fr == o.fr {
			length = len(o.rows[fr].title) - o.coloff
		} else {
			length = len(o.rows[fr].title)
		}

		if length > titlecols {
			length = titlecols
		}

		id := o.rows[fr].id
		note := o.Database.readNoteIntoString(id)
		if googleDriveRegex.MatchString(note) {
			ab.WriteString(BOLD)
		}
		//} else if timeKeywordsRegex.MatchString(o.rows[fr].sort) {
		if timeKeywordsRegex.MatchString(o.rows[fr].sort) {
			ab.WriteString(CYAN)
		} else {
			ab.WriteString(WHITE)
		}

		if o.rows[fr].archived && o.rows[fr].deleted {
			ab.WriteString(GREEN)
		} else if o.rows[fr].archived {
			ab.WriteString(YELLOW)
		} else if o.rows[fr].deleted {
			ab.WriteString(RED)
		}

		if fr == o.fr {
			ab.WriteString(DARK_GRAY_BG)
		}
		if o.rows[fr].dirty {
			ab.WriteString(BLACK + WHITE_BG)
		}
		if _, ok := o.marked_entries[o.rows[fr].id]; ok {
			ab.WriteString(BLACK + YELLOW_BG)
		}

		// below - only will get visual highlighting if it's the active
		// then also deals with column offset
		if o.mode == VISUAL && fr == o.fr {

			// below in case org.highlight[1] < org.highlight[0]
			if o.highlight[1] > o.highlight[0] {
				j, k = 0, 1
			} else {
				k, j = 0, 1
			}

			ab.WriteString(o.rows[fr].title[o.coloff : o.highlight[j]-o.coloff])
			ab.WriteString(LIGHT_GRAY_BG)
			ab.WriteString(o.rows[fr].title[o.highlight[j] : o.highlight[k]-o.coloff])

			ab.WriteString(DARK_GRAY_BG)
			ab.WriteString(o.rows[fr].title[o.highlight[k]:])

		} else {
			// current row is only row that is scrolled if org.coloff != 0
			var beg int
			if fr == o.fr {
				beg = o.coloff
			}
			if len(o.rows[fr].title[beg:]) > length {
				ab.WriteString(o.rows[fr].title[beg : beg+length])
			} else {
				ab.WriteString(o.rows[fr].title[beg:])
			}
		}
		// the spaces make it look like the whole row is highlighted
		//note len can't be greater than titlecols so always positive
		ab.WriteString(strings.Repeat(" ", titlecols-length+1))

		// believe the +2 is just to give some space from the end of long titles
		fmt.Fprintf(&ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, o.Screen.divider-TIME_COL_WIDTH+2)
		//ab.WriteString(o.rows[fr].modified)
		ab.WriteString(o.rows[fr].sort)
		ab.WriteString(RESET)
		ab.WriteString(lf_ret)
	}
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
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", LEFT_MARGIN)

	for y := 0; y < o.Screen.textLines; y++ {
		fr := y + o.rowoff
		if fr > len(o.rows)-1 {
			break
		}
		if fr == o.fr {
			o.drawActiveRow(&ab, y)
			continue
		}

		if o.rows[fr].star {
			ab.WriteString("\x1b[1m") //bold
			ab.WriteString("\x1b[1;36m")
		}

		if o.rows[fr].archived && o.rows[fr].deleted {
			ab.WriteString(GREEN) //green foreground
		} else if o.rows[fr].archived {
			ab.WriteString(YELLOW) //yellow foreground
		} else if o.rows[fr].deleted {
			ab.WriteString(RED) //red foreground
		}
		if len(o.rows[fr].title) <= titlecols { // we know it fits
			ab.WriteString(o.rows[fr].ftsTitle)
			// note below doesn't handle two highlighted terms in same line
			// and it might cause display issues if second highlight isn't fully escaped
			// need to come back and deal with this
			// coud check if LastIndex"\x1b[49m" or Index(fts_title[pos+1:titlecols+15] contained another escape
		} else {
			pos := strings.Index(o.rows[fr].ftsTitle, "\x1b[49m")                          //\x1b[48;5;31m', '\x1b[49m'
			if pos > 0 && pos < titlecols+11 && len(o.rows[fr].ftsTitle) >= titlecols+15 { //length of highlight escape last check ? shouldn't be necessary added 04032021
				ab.WriteString(o.rows[fr].ftsTitle[:titlecols+15]) //titlecols + 15); // length of highlight escape + remove formatting escape
			} else {
				ab.WriteString(o.rows[fr].title[:titlecols])
			}
		}
		var length int
		if len(o.rows[fr].title) <= titlecols {
			length = len(o.rows[fr].title)
		} else {
			length = titlecols
		}

		spaces := titlecols - length
		ab.WriteString(strings.Repeat(" ", spaces))

		ab.WriteString("\x1b[0m") // return background to normal
		fmt.Fprintf(&ab, "\x1b[%d;%dH", y+2, o.Screen.divider-TIME_COL_WIDTH+2)
		//ab.WriteString(o.rows[fr].modified)
		ab.WriteString(o.rows[fr].sort)
		ab.WriteString(lf_ret)

		//fmt.Print(ab.String())
	}
	fmt.Print(ab.String())
}

// change function name to displayRenderedNote
func (o *Organizer) drawPreview() {
	if len(o.rows) == 0 {
		o.Screen.eraseRightScreen()
		return
	}
	id := o.rows[o.fr].id ///it appears that coming out of a sync there is a problem - maybe o.rows is not what is expected
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
	note = WordWrap(note, o.Screen.totaleditorcols)
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
