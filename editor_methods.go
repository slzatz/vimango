package main

import (
	//"bufio"
	"bytes"
	"fmt"
	"image"
	"os"
	"strings"
//	"time"
	"unicode/utf8"

	"github.com/slzatz/vimango/hunspell"
	"github.com/slzatz/vimango/vim"
)

func (e *Editor) highlightInfo() { // [2][4]int {
	e.highlight = vim.VisualGetRange() //[]line col []line col
}

// highlight matched braces in NORMAL and INSERT modes
func (e *Editor) drawHighlightedBraces() {

	if len(e.ss) == 0 || len(e.ss[e.fr]) == 0 {
		return
	}

	var b byte
	var back int
	//if below handles case when in insert mode and brace is last char
	//in a row and cursor is beyond that last char (which is a brace)
	if e.fc == len(e.ss[e.fr]) {
		b = e.ss[e.fr][e.fc-1]
		back = 1
	} else {
		b = e.ss[e.fr][e.fc]
		back = 0
	}

	if !strings.ContainsAny(string(b), "{}()") {
		return
	}

	pos := vim.SearchGetMatchingPair()
	if pos == [2]int{0, 0} {
		return
	}

	y := e.getScreenYFromRowColWW(pos[0]-1, pos[1]) - e.lineOffset
	if y >= e.screenlines {
		return
	}

	match := e.ss[pos[0]-1][pos[1]]

	x := e.getScreenXFromRowColWW(pos[0]-1, pos[1]) + e.left_margin + e.left_margin_offset + 1
	fmt.Printf("\x1b[%d;%dH\x1b[48;5;237m%s", y+e.top_margin, x, string(match))

	x = e.getScreenXFromRowColWW(e.fr, e.fc-back) + e.left_margin + e.left_margin_offset + 1
	y = e.getScreenYFromRowColWW(e.fr, e.fc-back) + e.top_margin - e.lineOffset // added line offset 12-25-2019
	fmt.Printf("\x1b[%d;%dH\x1b[48;5;237m%s\x1b[0m", y, x, string(b))
	return
}

func (e *Editor) setLinesMargins() { //also sets top margin

	if e.output != nil {
		if e.output.is_below {
			e.screenlines = sess.textLines - LINKED_NOTE_HEIGHT - 1
			e.top_margin = TOP_MARGIN + 1
		} else {
			e.screenlines = sess.textLines
			e.top_margin = TOP_MARGIN + 1
		}
	} else {
		e.screenlines = sess.textLines
		e.top_margin = TOP_MARGIN + 1
	}
}

// used by updateNote
func (e *Editor) bufferToString() string {
	return strings.Join(e.ss, "\n")
}

func (e *Editor) getScreenXFromRowColWW(r, c int) int {
	row := e.ss[r]
	row = strings.ReplaceAll(row, "\t", "$$$$")
	tabCount := strings.Count(e.ss[r][:c], "\t")
	c = c + 3*tabCount
	width := e.screencols - e.left_margin_offset
	if width >= len(row) {
		return c
	}
	start := 0
	end := 0
	for {
		if width >= len(row[start:]) {
			break
		}
		pos := strings.LastIndex(row[start:start+width], " ")
		if pos == -1 {
			end = start + width - 1
		} else {
			end = start + pos
		}
		if end >= c {
			break
		}
		start = end + 1
	}
	return c - start
}

func (e *Editor) getScreenYFromRowColWW(r, c int) int {
	screenLine := 0

	for n := 0; n < r; n++ {
		screenLine += e.getLinesInRowWW(n)
	}

	screenLine = screenLine + e.getLineInRowWW(r, c) - 1
	return screenLine
}

func (e *Editor) getLinesInRowWW(r int) int {
	row := e.ss[r]
	row = strings.ReplaceAll(row, "\t", "$$$$")
	width := e.screencols - e.left_margin_offset
	if width >= len(row) {
		return 1
	}
	lines := 0
	start := 0
	end := 0
	for {

		if width >= len(row[start:]) {
			lines++
			break
		}
		pos := strings.LastIndex(row[start:start+width], " ")
		if pos == -1 {
			end = start + width - 1
		} else {
			end = start + pos
		}
		lines++
		start = end + 1
	}
	return lines
}

func (e *Editor) getLineInRowWW(r, c int) int {
	row := e.ss[r]
	tabCount := strings.Count(row[:c], "\t")
	if tabCount > 0 {
		row = strings.ReplaceAll(row, "\t", "$$$$")
		c += 3 * tabCount
	}
	width := e.screencols - e.left_margin_offset
	if width >= len(row) {
		return 1
	}
	lines := 0
	start := 0
	end := 0
	for {
		if width >= len(row[start:]) {
			lines++
			break
		}
		pos := strings.LastIndex(row[start:start+width], " ")
		if pos == -1 {
			end = start + width - 1
		} else {
			end = start + pos
		}
		lines++
		if end >= c { //+3
			break
		}
		start = end + 1
	}
	return lines
}

func (e *Editor) drawText() {
	var ab strings.Builder

	fmt.Fprintf(&ab, "\x1b[?25l\x1b[%d;%dH", e.top_margin, e.left_margin+1)
	// \x1b[NC moves cursor forward by n columns
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", e.left_margin)
	erase_chars := fmt.Sprintf("\x1b[%dX", e.screencols)
	for i := 0; i < e.screenlines; i++ {
		ab.WriteString(erase_chars)
		ab.WriteString(lf_ret)
	}
	if e.highlightSyntax {
		e.drawCodeRows(&ab)
		e.drawHighlights(&ab)
		e.drawVisual(&ab)
		fmt.Print(ab.String())
		//go e.drawHighlightedBraces() // this will produce data race
		e.drawHighlightedBraces() //has to come after drawing rows
		//e.drawDiagnostics() // if here would update with each text change
	} else {
		e.drawPlainRows(&ab)
		e.drawHighlights(&ab)
		e.drawVisual(&ab)
		fmt.Print(ab.String())
	}
}

func (e *Editor) drawVisual(pab *strings.Builder) {

	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", e.left_margin+e.left_margin_offset)

	if e.mode == VISUAL_LINE {
		startRow := e.highlight[0][0] - 1 // highlight line starts a 1
		endRow := e.highlight[1][0] - 1   //ditto - done differently for visual and v_block

		x := e.left_margin + e.left_margin_offset + 1
		y := e.getScreenYFromRowColWW(startRow, 0) - e.lineOffset

		if y >= 0 {
			fmt.Fprintf(pab, "\x1b[%d;%dH\x1b[48;5;237m", y+e.top_margin, x) //244
		} else {
			fmt.Fprintf(pab, "\x1b[%d;%dH\x1b[48;5;237m", e.top_margin, x)
		}

		for n := 0; n < (endRow - startRow + 1); n++ { //++n
			rowNum := startRow + n
			row := e.ss[rowNum]
			row = strings.ReplaceAll(row, "\t", "    ")
			pos := 0
			for line := 1; line <= e.getLinesInRowWW(rowNum); line++ { //++line
				if y < 0 {
					y += 1
					continue
				}
				if y == e.screenlines {
					break //out for should be done (theoretically) - 1
				}
				line_char_count := e.getLineCharCountWW(rowNum, line)
				//pab.WriteString(strings.ReplaceAll(e.ss[rowNum][pos:pos+line_char_count], "\t", "$$$$"))
				pab.WriteString(row[pos : pos+line_char_count])
				pab.WriteString(lf_ret)
				y += 1
				pos += line_char_count
			}
		}
	}

	if e.mode == VISUAL {
		startCol, endcol := e.highlight[0][1], e.highlight[1][1]

		// startRow always <= endRow and need to subtract 1 since counting starts at 1 not zero
		startRow, endRow := e.highlight[0][0]-1, e.highlight[1][0]-1 //startRow always <= endRow
		numrows := endRow - startRow + 1

		x := e.getScreenXFromRowColWW(startRow, startCol) + e.left_margin + e.left_margin_offset + 1
		y := e.getScreenYFromRowColWW(startRow, startCol) + e.top_margin - e.lineOffset // - 1

		pab.WriteString("\x1b[48;5;237m")
		for n := 0; n < numrows; n++ {
			// i think would check here to see if a row has multiple lines (ie wraps)
			if n == 0 {
				fmt.Fprintf(pab, "\x1b[%d;%dH", y+n, x)
			} else {
				fmt.Fprintf(pab, "\x1b[%d;%dH", y+n, 1+e.left_margin+e.left_margin_offset)
			}
			row := e.ss[startRow+n]

			// I do not know why this works!!
			row = strings.ReplaceAll(row, "\t", " ")

			if len(row) == 0 {
				continue
			}
			if numrows == 1 {
				// in VISUAL mode like INSERT mode, the cursor can go beyond end of row
				if len(row) == endcol {
					pab.WriteString(row[startCol:endcol])
				} else {
					pab.WriteString(row[startCol : endcol+1])
				}
			} else if n == 0 {
				pab.WriteString(row[startCol:])
			} else if n < numrows-1 {
				pab.WriteString(row)
			} else {
				if len(row) < endcol {
					pab.WriteString(row)
				} else {
					pab.WriteString(row[:endcol])
				}
			}
		}
	}

	if e.mode == VISUAL_BLOCK {

		var left, right int
		if e.highlight[1][1] > e.highlight[0][1] {
			right, left = e.highlight[1][1], e.highlight[0][1]
		} else {
			left, right = e.highlight[1][1], e.highlight[0][1]
		}
		x := e.getScreenXFromRowColWW(e.highlight[0][0]-1, left) + e.left_margin + e.left_margin_offset + 1 //-1
		y := e.getScreenYFromRowColWW(e.highlight[0][0]-1, left) + e.top_margin - e.lineOffset
		//sess.showOrgMessage("highlight = %v, right = %v, left = %v, x = %v, y = %v", e.highlight, right, left, x, y)

		pab.WriteString("\x1b[48;5;237m")
		for n := 0; n < (e.highlight[1][0] - e.highlight[0][0] + 1); n++ {
			fmt.Fprintf(pab, "\x1b[%d;%dH", y+n, x)
			row := e.ss[e.highlight[0][0]+n-1]
			rowLen := len(row)

			if rowLen == 0 || rowLen < left {
				continue
			}

			if rowLen < right+1 {
				pab.WriteString(row[left:rowLen])
			} else {
				pab.WriteString(row[left : right+1])
			}
		}
	}

	pab.WriteString(RESET)
}

func (e *Editor) getLineCharCountWW(r, line int) int {
	row := e.ss[r]
	row = strings.ReplaceAll(row, "\t", "$$$$")

	width := e.screencols - e.left_margin_offset

	if width >= len(row) {
		return len(row)
	}

	lines := 0
	pos := 0
	prev_pos := 0

	for {

		if width >= len(row[prev_pos:]) {
			return len(row[prev_pos:])
		}

		pos = strings.LastIndex(row[prev_pos:pos+width], " ")

		if pos == -1 {
			pos = prev_pos + width - 1
		} else {
			pos = pos + prev_pos
		}

		lines++

		if lines == line {
			break
		}

		prev_pos = pos + 1
	}

	return pos - prev_pos + 1
}

func (e *Editor) drawPlainRows(pab *strings.Builder) {
	note := e.generateWWStringFromBuffer() // need the \t for line num to be correct
	nnote := strings.Split(note, "\n")

	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", e.left_margin)
	fmt.Fprintf(pab, "\x1b[?25l\x1b[%d;%dH", e.top_margin, e.left_margin+1) //+1

	var s string
	if e.numberLines {
		var numCols strings.Builder
		// below draws the line number 'rectangle'
		// can be drawm to pab or &numCols
		fmt.Fprintf(&numCols, "\x1b[2*x\x1b[%d;%d;%d;%d;48;5;235$r\x1b[*x",
			e.top_margin,
			e.left_margin,
			e.top_margin+e.screenlines,
			e.left_margin+e.left_margin_offset)
		fmt.Fprintf(&numCols, "\x1b[?25l\x1b[%d;%dH", e.top_margin, e.left_margin+1)

		s = fmt.Sprintf("\x1b[%dC", e.left_margin_offset) + "%s" + lf_ret
		for n := e.firstVisibleRow; n < len(nnote); n++ {
			row := nnote[n]
			fmt.Fprintf(&numCols, "\x1b[48;5;235m\x1b[38;5;245m%3d \x1b[49m", n+1)
			line := strings.Split(row, "\t")
			for i := 0; i < len(line); i++ {
				fmt.Fprintf(pab, s, line[i])
				numCols.WriteString(lf_ret)
			}
		}
		pab.WriteString(numCols.String())
	} else {
		s = "%s" + lf_ret
		for n := e.firstVisibleRow; n < len(nnote); n++ {
			row := nnote[n]
			line := strings.Split(row, "\t")
			for i := 0; i < len(line); i++ {
				fmt.Fprintf(pab, s, line[i])
			}
		}
	}
}

func (e *Editor) drawCodeRows(pab *strings.Builder) {
	var lang string
	if taskFolder(e.id) == "code" {
		c := taskContext(e.id)
		var ok bool
		if lang, ok = Languages[c]; !ok {
			lang = "markdown"
		}
	} else {
		lang = "markdown"
	}

	note := e.generateWWStringFromBuffer()
	var buf bytes.Buffer
	_ = Highlight(&buf, note, lang, "terminal16m", sess.style[sess.styleIndex])
	note = buf.String()
	nnote := strings.Split(note, "\n")
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", e.left_margin)
	fmt.Fprintf(pab, "\x1b[?25l\x1b[%d;%dH", e.top_margin, e.left_margin+1)

	if e.numberLines {
		var numCols strings.Builder
		// below draws the line number 'rectangle'
		// cam be drawm to pab or &numCols
		fmt.Fprintf(&numCols, "\x1b[2*x\x1b[%d;%d;%d;%d;48;5;235$r\x1b[*x",
			e.top_margin,
			e.left_margin,
			e.top_margin+e.screenlines,
			e.left_margin+e.left_margin_offset)
		fmt.Fprintf(&numCols, "\x1b[?25l\x1b[%d;%dH", e.top_margin, e.left_margin+1)

		s := fmt.Sprintf("\x1b[%dC", e.left_margin_offset) + "%s" + lf_ret
		for n := e.firstVisibleRow; n < len(nnote); n++ {
			row := nnote[n]
			fmt.Fprintf(&numCols, "\x1b[48;5;235m\x1b[38;5;245m%3d \x1b[49m", n+1)
			line := strings.Split(row, "\t")
			for i := 0; i < len(line); i++ {
				fmt.Fprintf(pab, s, line[i])
				numCols.WriteString(lf_ret)
			}
		}
		pab.WriteString(numCols.String())
	} else {
		s := "%s" + lf_ret
		for n := e.firstVisibleRow; n < len(nnote); n++ {
			row := nnote[n]
			line := strings.Split(row, "\t")
			for i := 0; i < len(line); i++ {
				fmt.Fprintf(pab, s, line[i])
			}
		}
	}
}

func (e *Editor) highlightMispelledWords() {
	h := hunspell.Hunspell("/usr/share/hunspell/en_US.aff", "/usr/share/hunspell/en_US.dic")
	e.highlightPositions = nil
	curPos := vim.CursorGetPosition()
	vim.Input2("gg^")
	var pos, prevPos [2]int
	for {
		vim.Input("w")
		prevPos = pos
		pos = vim.CursorGetPosition()
		if pos == prevPos {
			break
		}
		w := vim.Eval("expand('<cword>')")
		if ok := h.Spell(w); ok {
			continue
		}
		e.highlightPositions = append(e.highlightPositions, Position{pos[0] - 1, pos[1], pos[1] + len(w)})
	}
	vim.CursorSetPosition(curPos[0], curPos[1]) //return cursor to where it was
}

func (e *Editor) drawHighlights(pab *strings.Builder) {
	if e.highlightPositions == nil {
		return
	}
	if e.isModified() {
		e.highlightPositions = nil
		return
	}
	for _, p := range e.highlightPositions {
		row := e.ss[p.rowNum]
		chars := "\x1b[48;5;31m" + string(row[p.start:p.end]) + "\x1b[0m"
		start := utf8.RuneCountInString(row[:p.start])
		y := e.getScreenYFromRowColWW(p.rowNum, start) + e.top_margin - e.lineOffset          // - 1
		x := e.getScreenXFromRowColWW(p.rowNum, start) + e.left_margin + e.left_margin_offset // - 1
		if y >= e.top_margin && y <= e.screenlines {
			fmt.Fprintf(pab, "\x1b[%d;%dH\x1b[0m", y, x+1) //not sure why the +1
			fmt.Fprint(pab, chars)
		}
	}
}

/*
* simplified version of generateWWStringFromBuffer
* used by editor.showMarkdown and editor.spellCheck in editor_normal
* we know we want the whole buffer not just what is visible
* unlike the situation with syntax highlighting for code
* we don't have to handle word-wrapped lines in a special way
 */
func (e *Editor) generateWWStringFromBuffer2() string {
	numRows := len(e.ss)
	if numRows == 0 {
		return ""
	}

	var ab strings.Builder
	y := 0
	filerow := 0
	//width := e.screencols - e.left_margin_offset
	width := e.screencols //05042021

	for {
		if filerow == numRows {
			return ab.String()
		}

		row := e.ss[filerow]

		if len(row) == 0 {
			ab.WriteString("\n")
			filerow++
			y++
			continue
		}

		// added 04052022 so urls that word wrap handled correctly`
		// question if might cause issue for spellcheck
		if strings.Index(row, "](http") != -1 {
			ab.WriteString(row)
			ab.WriteString("\n")
			filerow++
			y++
			continue
		}

		start := 0
		end := 0
		for {
			// if remainder of line is less than screen width
			if start+width > len(row)-1 {
				//ab.WriteString(row[start:])
				//ab.WriteString("\n")
				fmt.Fprintf(&ab, "%s%s", row[start:], "\n")
				y++
				filerow++
				break
			}

			pos := strings.LastIndex(row[start:start+width], " ")
			if pos == -1 {
				end = start + width - 1
			} else {
				end = start + pos
			}

			//ab.WriteString(row[start : end+1])
			//ab.WriteString("\n")
			//fmt.Fprintf(&ab, "%s%s", row[start:end+1], "\n")
			fmt.Fprintf(&ab, "%s%s", row[start:end+1], "^^^") //04052022
			y++
			start = end + 1
		}
	}
}

/* below exists to create a string that has the proper
 * line breaks based on screen width for syntax highlighting
 * being done in drawcoderows
 * produces a text string that starts at the first line of the
 * file (need to deal with comments where start of comment might be scrolled
 * and ends on the last visible line. Word-wrapped rows are terminated by \t
 * so highlighter deals with them correctly and converted to \n in drawcoderows
 * very similar to dbfunc generateWWString except this uses buffer
 * and only returns as much file as fits the screen
 * and deals with the issue of multi-line comments
 */
func (e *Editor) generateWWStringFromBuffer() string {
	numRows := len(e.ss)
	if numRows == 0 {
		return ""
	}

	var ab strings.Builder
	y := 0
	filerow := 0
	width := e.screencols - e.left_margin_offset

	for {
		if filerow == numRows || y == e.screenlines+e.lineOffset {
			return ab.String()[:ab.Len()-1] // delete last \n
		}

		row := strings.ReplaceAll(e.ss[filerow], "\t", "    ")

		if len(row) == 0 {
			ab.WriteString("\n")
			filerow++
			y++
			continue
		}

		start := 0
		end := 0
		for {
			// if remainder of line is less than screen width
			if start+width > len(row)-1 {
				//ab.WriteString(row[start:])
				//ab.WriteString("\n")
				fmt.Fprintf(&ab, "%s%s", row[start:], "\n")
				y++
				filerow++
				break
			}

			pos := strings.LastIndex(row[start:start+width], " ")
			if pos == -1 {
				end = start + width - 1
			} else {
				end = start + pos
			}

			ab.WriteString(row[start : end+1])
			if y == e.screenlines+e.lineOffset-1 {
				return ab.String()
			}
			ab.WriteString("\t")
			y++
			start = end + 1
		}
	}
}

func (e *Editor) drawStatusBar() {
	var ab strings.Builder
	fmt.Fprintf(&ab, "\x1b[%d;%dH", e.screenlines+e.top_margin, e.left_margin+1)

	//erase from start of an Editor's status bar to the end of the Editor's status bar
	fmt.Fprintf(&ab, "\x1b[%dX", e.screencols)

	ab.WriteString("\x1b[7m ") //switches to inverted colors
	title := getTitle(e.id)
	if len(title) > 30 {
		title = title[:30]
	}
	if e.isModified() {
		title += "[+]"
	}
	status := fmt.Sprintf("%d - %s ...", e.id, title)

	if len(status) > e.screencols-1 {
		status = status[:e.screencols-1]
	}
	fmt.Fprintf(&ab, "%-*s", e.screencols, status)
	ab.WriteString("\x1b[0m") //switches back to normal formatting
	fmt.Print(ab.String())
}

func (e *Editor) drawFrame() {
	var ab strings.Builder
	ab.WriteString("\x1b(0") // Enter line drawing mode

	for j := 1; j < e.screenlines+1; j++ {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", e.top_margin-1+j, e.left_margin+e.screencols+1)
		// below x = 0x78 vertical line (q = 0x71 is horizontal) 37 = white; 1m = bold (note
		// only need one 'm'
		ab.WriteString("\x1b[37;1mx")
	}

	//'T' corner = w or right top corner = k
	fmt.Fprintf(&ab, "\x1b[%d;%dH", e.top_margin-1, e.left_margin+e.screencols+1)

	if e.left_margin+e.screencols > sess.screenCols-4 {
		ab.WriteString("\x1b[37;1mk") //draw corner
	} else {
		ab.WriteString("\x1b[37;1mw")
	}

	//exit line drawing mode
	ab.WriteString("\x1b(B")
	ab.WriteString("\x1b[?25h") //shows the cursor
	ab.WriteString("\x1b[0m")   //or else subsequent editors are bold
	fmt.Print(ab.String())
}

func (e *Editor) scroll() {

	if e.fc == 0 && e.fr == 0 {
		e.cy, e.cx, e.lineOffset, e.firstVisibleRow = 0, 0, 0, 0
		return
	}

	e.cx = e.getScreenXFromRowColWW(e.fr, e.fc)
	cy := e.getScreenYFromRowColWW(e.fr, e.fc)

	//deal with scroll insufficient to include the current line
	if cy > e.screenlines+e.lineOffset-1 {
		e.lineOffset = cy - e.screenlines + 1 ////
		e.adjustFirstVisibleRow()             //can also change e.lineOffset
	}

	if cy < e.lineOffset {
		e.lineOffset = cy
		e.adjustFirstVisibleRow()
	}

	if e.lineOffset == 0 {
		e.firstVisibleRow = 0
	}

	e.cy = cy - e.lineOffset
}

// e.lineOffset determines the first
// visible row but we want the whole row
// visible so that can change e.lineOffset
func (e *Editor) adjustFirstVisibleRow() {

	if e.lineOffset == 0 {
		e.firstVisibleRow = 0
		return
	}

	rowNum := 0
	lines := 0

	for {
		lines += e.getLinesInRowWW(rowNum)
		rowNum++

		/*
			there is no need to adjust line_offset
			if it happens that we start
			on the first line of the first visible row
		*/
		if lines == e.lineOffset {
			break
		}

		/*
			need to adjust line_offset
			so we can start on the first
			line of the top row
		*/
		if lines > e.lineOffset {
			e.lineOffset = lines
			break
		}
	}
	e.firstVisibleRow = rowNum
}

func (e *Editor) readFileIntoNote(filename string) error {

	b, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("Error opening file %s: %w", filename, err)
	}
	e.ss = strings.Split(string(b), "\n")
	vim.BufferSetLines(e.vbuf, 0, -1, e.ss, len(e.ss))

	e.fr, e.fc, e.cy, e.cx, e.lineOffset, e.firstVisibleRow = 0, 0, 0, 0, 0, 0

	e.drawText()
	e.drawStatusBar() // not sure what state of isModified would be so not sure need to draw statubBar
	return nil
}

func (e *Editor) drawPreview() {
	//delete any images
	fmt.Print("\x1b_Ga=d\x1b\\") //delete any images

	rows := strings.Split(e.renderedNote, "\n")
	fmt.Printf("\x1b[?25l\x1b[%d;%dH", e.top_margin, e.left_margin+1)
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", e.left_margin)

	// erase specific editors 'window'
	erase_chars := fmt.Sprintf("\x1b[%dX", e.screencols)
	for i := 0; i < e.screenlines; i++ {
		fmt.Printf("%s%s", erase_chars, lf_ret)
	}

	fmt.Printf("\x1b[%d;%dH", e.top_margin, e.left_margin+1)

	fr := e.previewLineOffset - 1
	y := 0
	for {
		fr++
		if fr > len(rows)-1 || y > e.screenlines-1 {
			break
		}
		if strings.Contains(rows[fr], "Im@ge") {
			fmt.Printf("Loading Image ... \x1b[%dG", e.left_margin+1)
			prevY := y
			path := getStringInBetween(rows[fr], "|", "|")
			var img image.Image
			var err error
			if strings.Contains(path, "http") {
				img, _, err = loadWebImage(path)
				if err != nil {
					fmt.Printf("%sError:%s %s%s", BOLD, RESET, rows[fr], lf_ret)
					y++
					continue
				}
			} else {
				maxWidth := e.screencols * int(sess.ws.Xpixel) / sess.screenCols
				maxHeight := e.screenlines * int(sess.ws.Ypixel) / sess.screenLines
				img, _, err = loadImage(path, maxWidth-5, maxHeight-150)
				if err != nil {
					fmt.Printf("%sError:%s %s%s", BOLD, RESET, rows[fr], lf_ret)
					y++
					continue
				}
			}
			height := img.Bounds().Max.Y / (int(sess.ws.Ypixel) / sess.screenLines)
			y += height
			if y > e.screenlines-1 {
				fmt.Printf("\x1b[3m\x1b[4mImage %s doesn't fit!\x1b[0m \x1b[%dG", path, e.left_margin+1)
				y = y - height + 1
				fmt.Printf("\x1b[%d;%dH", TOP_MARGIN+1+y, e.left_margin+1)
				continue
			}
			displayImage(img)
			// erases "Loading image ..."
			fmt.Printf("\x1b[%d;%dH\x1b[0K", e.top_margin+prevY, e.left_margin+1)
			fmt.Printf("\x1b[%d;%dH", e.top_margin+y, e.left_margin+1)
		} else {
			fmt.Printf("%s%s", rows[fr], lf_ret)
			y++
		}
	}
}

func (e *Editor) drawOverlay() {
	fmt.Print("\x1b_Ga=d\x1b\\") //delete any images

	//rows := strings.Split(s, "\n")
	fmt.Printf("\x1b[?25l\x1b[%d;%dH", e.top_margin, e.left_margin+1)
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", e.left_margin)

	// erase specific editors 'window'
	erase_chars := fmt.Sprintf("\x1b[%dX", e.screencols)
	for i := 0; i < e.screenlines; i++ {
		fmt.Printf("%s%s", erase_chars, lf_ret)
	}

	fmt.Printf("\x1b[%d;%dH", e.top_margin, e.left_margin+1)

	fr := e.previewLineOffset - 1
	y := 0
	rows := e.overlay
	for {
		fr++
		if fr > len(rows)-1 || y > e.screenlines-1 {
			break
		}
		fmt.Printf("%s%s", rows[fr], lf_ret)
		y++
	}
}

// this func is reason that we are writing notes to file
// allows easy testing if a file is modified with BufferOption
func (e *Editor) isModified() bool {
	//return vim.BufferGetModified(e.vbuf)

	tick := vim.BufferGetLastChangedTick(e.vbuf)
	if tick > e.bufferTick {
		//e.bufferTick = tick
		return true
	}
	return false
}
