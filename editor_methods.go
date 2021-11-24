package main

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/slzatz/vimango/vim"
	"go.lsp.dev/protocol"
)

func (e *Editor) highlightInfo() { // [2][4]int {
	pos := vim.VisualGetRange()
	e.vb_highlight[0] = [4]int{0, pos[0][0], pos[0][1] + 1, 0}
	e.vb_highlight[1] = [4]int{0, pos[1][0], pos[1][1] + 1, 0}
}

//'automatically' happens in NORMAL and INSERT mode
//return true -> redraw; false -> don't redraw
func (e *Editor) findMatchForLeftBrace(leftBrace byte, back bool) bool {
	r := e.fr
	c := e.fc + 1
	count := 1
	max := len(e.bb)
	var b int
	if back {
		b = 1
	}

	m := map[byte]byte{'{': '}', '(': ')', '[': ']'}
	rightBrace := m[leftBrace]

	for {

		row := e.bb[r]

		// need >= because brace could be at end of line and in INSERT mode
		// fc could be row.size() [ie beyond the last char in the line
		// and so doing fc + 1 above leads to c > row.size()
		if c >= len(row) {
			r++
			if r == max {
				sess.showEdMessage("Couldn't find matching brace")
				return false
			}
			c = 0
			continue
		}

		if row[c] == rightBrace {
			count -= 1
			if count == 0 {
				break
			}
		} else if row[c] == leftBrace {
			count += 1
		}

		c++
	}
	y := e.getScreenYFromRowColWW(r, c) - e.lineOffset
	if y >= e.screenlines {
		return false
	}

	x := e.getScreenXFromRowColWW(r, c) + e.left_margin + e.left_margin_offset + 1
	fmt.Printf("\x1b[%d;%dH\x1b[48;5;237m%s", y+e.top_margin, x, string(rightBrace))

	x = e.getScreenXFromRowColWW(e.fr, e.fc-b) + e.left_margin + e.left_margin_offset + 1
	y = e.getScreenYFromRowColWW(e.fr, e.fc-b) + e.top_margin - e.lineOffset // added line offset 12-25-2019
	fmt.Printf("\x1b[%d;%dH\x1b[48;5;237m%s\x1b[0m", y, x, string(leftBrace))
	//sess.showEdMessage("r = %d   c = %d", r, c)
	return true
}

//'automatically' happens in NORMAL and INSERT mode
func (e *Editor) findMatchForRightBrace(rightBrace byte, back bool) bool {
	var b int
	if back {
		b = 1
	}
	r := e.fr
	c := e.fc - 1 - b
	count := 1

	row := e.bb[r]

	m := map[byte]byte{'}': '{', ')': '(', ']': '['}
	leftBrace := m[rightBrace]

	for {

		if c == -1 { //fc + 1 can be greater than row.size on first pass from INSERT if { at end of line
			r--
			if r == -1 {
				sess.showEdMessage("Couldn't find matching brace")
				return false
			}
			row = e.bb[r]
			c = len(row) - 1
			continue
		}

		if row[c] == leftBrace {
			count -= 1
			if count == 0 {
				break
			}
		} else if row[c] == rightBrace {
			count += 1
		}

		c--
	}

	y := e.getScreenYFromRowColWW(r, c) - e.lineOffset
	if y < 0 {
		return false
	}

	x := e.getScreenXFromRowColWW(r, c) + e.left_margin + e.left_margin_offset + 1
	fmt.Printf("\x1b[%d;%dH\x1b[48;5;237m%s", y+e.top_margin, x, string(leftBrace))

	x = e.getScreenXFromRowColWW(e.fr, e.fc-b) + e.left_margin + e.left_margin_offset + 1
	y = e.getScreenYFromRowColWW(e.fr, e.fc-b) + e.top_margin - e.lineOffset // added line offset 12-25-2019
	fmt.Printf("\x1b[%d;%dH\x1b[48;5;237m%s\x1b[0m", y, x, string(rightBrace))
	sess.showEdMessage("r = %d   c = %d", r, c)
	return true
}

func (e *Editor) drawHighlightedBraces() {

	// this guard is necessary
	if len(e.bb) == 0 || len(e.bb[e.fr]) == 0 {
		return
	}

	braces := "{}()" //? intentionally exclusing [] from auto drawing
	var c byte
	var back bool
	//if below handles case when in insert mode and brace is last char
	//in a row and cursor is beyond that last char (which is a brace)
	if e.fc == len(e.bb[e.fr]) {
		c = e.bb[e.fr][e.fc-1]
		back = true
	} else {
		c = e.bb[e.fr][e.fc]
		back = false
	}
	pos := strings.Index(braces, string(c))
	if pos != -1 {
		switch c {
		case '{', '(':
			e.findMatchForLeftBrace(c, back)
			return
		case '}', ')':
			e.findMatchForRightBrace(c, back)
			return
		//case '(':
		default: //should not need this
			return
		}
	} else if e.fc > 0 && e.mode == INSERT {
		c := e.bb[e.fr][e.fc-1]
		pos := strings.Index(braces, string(c))
		if pos != -1 {
			switch e.bb[e.fr][e.fc-1] {
			case '{', '(':
				e.findMatchForLeftBrace(c, true)
				return
			case '}', ')':
				e.findMatchForRightBrace(c, true)
				return
			//case '(':
			default: //should not need this
				return
			}
		}
	}
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

	numRows := len(e.bb)
	if numRows == 0 {
		return ""
	}

	var sb strings.Builder
	for i := 0; i < numRows-1; i++ {
		sb.Write(e.bb[i])
		sb.Write([]byte("\n"))
	}
	sb.Write(e.bb[numRows-1])
	return sb.String()
}

func (e *Editor) getScreenXFromRowColWW(r, c int) int {
	row := e.bb[r]
	row = bytes.ReplaceAll(row, []byte("\t"), []byte("$$$$")) ////////////
	tabCount := bytes.Count(e.bb[r][:c], []byte("\t"))
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
		pos := bytes.LastIndex(row[start:start+width], []byte(" "))
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
	row := e.bb[r]
	row = bytes.ReplaceAll(row, []byte("\t"), []byte("$$$$")) ////////////
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
		pos := bytes.LastIndex(row[start:start+width], []byte(" "))
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
	row := e.bb[r]
	row = bytes.ReplaceAll(row, []byte("\t"), []byte("$$$$")) ////////////
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
		pos := bytes.LastIndex(row[start:start+width], []byte(" "))
		if pos == -1 {
			end = start + width - 1
		} else {
			end = start + pos
		}
		lines++
		if end >= c {
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

func (e *Editor) drawDiagnostics() {

	op := e.output
	op.rowOffset = 0
	var s string
	select {
	case dd := <-diagnostics:
		var ab strings.Builder
		ab.WriteString("\n-----------------------------------------------\n")
		for i, d := range dd {
			fmt.Fprintf(&ab, "[%d]Start.Line = %v\n", i, d.Range.Start.Line+1)         //uint32
			fmt.Fprintf(&ab, "[%d]Start.Character = %v\n", i, d.Range.Start.Character) //uint32
			fmt.Fprintf(&ab, "[%d]End.Line = %v\n", i, d.Range.End.Line+1)             //uint32
			fmt.Fprintf(&ab, "[%d]End.Character = %v\n", i, d.Range.End.Character)     //uint32
			fmt.Fprintf(&ab, "[%d]Message = %s\n", i, d.Message)                       //string
			fmt.Fprintf(&ab, "[%d]Severity = %v\n", i, d.Severity)                     //Stringer()
			ab.WriteString("\n-----------------------------------------------\n")
			startRow := int(d.Range.Start.Line)
			/*
				startCol := int(d.Range.Start.Character)
				endCol := int(d.Range.End.Character)
			*/
			//fmt.Fprintf(&ab, "%d %d %d %d", startLine, startChar, endLine, endChar)
			//x := e.getScreenXFromRowColWW(startRow, startCol) + e.left_margin + e.left_margin_offset
			//y := e.getScreenYFromRowColWW(startRow, startCol) + e.top_margin - e.lineOffset // - 1
			y := e.getScreenYFromRowColWW(startRow, 0) + e.top_margin - e.lineOffset // - 1
			x := e.left_margin + 1
			ab.WriteString("\x1b[48;5;244m")
			fmt.Fprintf(&ab, "\x1b[%d;%dH", y, x)
			/*
				row := e.bb[startRow]
				ab.Write(row[startCol-1 : endCol])
			*/
			fmt.Fprintf(&ab, "%*d", 3, startRow+1)
			ab.WriteString(RESET)
			break
		}
		if len(dd) == 0 {
			fmt.Fprintf(&ab, "->Diagnostics was []\n")
		}
		s = ab.String()
	case <-time.After(1 * time.Second):

		s = "Did not receive any new diagnostics"
	}
	op.rows = strings.Split(s, "\n")
	op.drawText()
	sess.returnCursor()
}
func (e *Editor) drawCompletionItems(completion protocol.CompletionList) {

	op := e.output
	op.rowOffset = 0
	var s string
	var ab strings.Builder
	for _, item := range completion.Items {
		//fmt.Fprintf(&ab, "%v: %v\n", item.Label, item.Documentation)
		fmt.Fprintf(&ab, "%v\n", item.Label)
	}
	s = ab.String()
	op.rows = strings.Split(s, "\n")
	op.drawText()
	sess.returnCursor()
}

func (e *Editor) drawHover(hover protocol.Hover) {

	op := e.output
	op.rowOffset = 0
	var s string
	var ab strings.Builder
	kind := hover.Contents.Kind
	value := hover.Contents.Value
	startPosition := hover.Range.Start
	endPosition := hover.Range.End
	fmt.Fprintf(&ab, "contents.kind = %v\n", kind)
	fmt.Fprintf(&ab, "contents.value = %v\n\n", value)
	fmt.Fprintf(&ab, "range.start = %+v\n\n", startPosition)
	fmt.Fprintf(&ab, "range.end = %+v\n\n", endPosition)
	s = ab.String()
	op.rows = strings.Split(s, "\n")
	op.drawText()
	sess.returnCursor()
}

func (e *Editor) drawSignatureHelp(signatureHelp protocol.SignatureHelp) {
	// commented out info doesn't add anything at least for go
	op := e.output
	op.rowOffset = 0
	var s string
	var ab strings.Builder
	for _, sig := range signatureHelp.Signatures {
		fmt.Fprintf(&ab, "Label: %s\n\n", sig.Label)
		fmt.Fprintf(&ab, "Documentation: %v\n", sig.Documentation)
		//fmt.Fprintf(&ab, "Parameters: %v\n", sig.Parameters)
		//fmt.Fprintf(&ab, "ActiveParameter: %v\n", sig.ActiveParameter)
	}
	//activeParameter := signatureHelp.ActiveParameter
	//activeSignature := signatureHelp.ActiveSignature
	//fmt.Fprintf(&ab, "ActiveParameter = %+v\n", activeParameter)
	//fmt.Fprintf(&ab, "ActiveSignature = %+v\n", activeSignature)
	s = ab.String()
	op.rows = strings.Split(s, "\n")
	op.drawText()
	sess.returnCursor()
}

// rename
func (e *Editor) applyWorkspaceEdit(wse protocol.WorkspaceEdit) {
	op := e.output
	op.rowOffset = 0
	var s string
	s = "Rename:\n\n"
	changes := wse.DocumentChanges
	for _, change := range changes {
		s += "Change->\n"
		s += fmt.Sprintf("TextDocument->%+v\n", change.TextDocument)
		for i, edit := range change.Edits {
			s += fmt.Sprintf("Edit Number: %d\n", i+1)
			s += fmt.Sprintf("Range.Start.Line->%+v\n", edit.Range.Start.Line)
			s += fmt.Sprintf("Range.Start.Character->%+v\n", edit.Range.Start.Character)
			s += fmt.Sprintf("Range.End.Line->%+v\n", edit.Range.End.Line)
			s += fmt.Sprintf("Range.End.Character->%+v\n", edit.Range.End.Character)
			s += fmt.Sprintf("NewText->%+v\n\n", edit.NewText)
			line := int(edit.Range.Start.Line)
			startChar := int(edit.Range.Start.Character)
			endChar := int(edit.Range.End.Character)
			row := string(e.bb[line])
			row = row[:startChar] + edit.NewText + row[endChar:]
			//v.SetBufferText(e.vbuf, line, startChar, line, endChar, [][]byte{[]byte(edit.NewText)})
			vim.Input(fmt.Sprintf("%dgg%dlc%dl%s\x1b", line, startChar, endChar-startChar, edit.NewText))
			//e.bb, _ = v.BufferLines(e.vbuf, 0, -1, true) //reading updated buffer
			e.bb = vim.BufferLines(e.vbuf) //reading updated buffer
			e.drawText()
		}
	}
	op.rows = strings.Split(s, "\n")
	op.drawText()
	sess.returnCursor()
}

func (e *Editor) drawDocumentHighlight(documentHighlights []protocol.DocumentHighlight) {
	/*
		op := e.output
		op.rowOffset = 0
		var s string
		s = "documentHighlights:\n\n"
	*/
	e.highlightPositions = nil
	for _, dh := range documentHighlights {
		/*
			s += fmt.Sprintf("Highlight Number: %d\n", i+1)
			s += fmt.Sprintf("Range.Start.Line->%+v\n", dh.Range.Start.Line)
			s += fmt.Sprintf("Range.Start.Character->%+v\n", dh.Range.Start.Character)
			//s += fmt.Sprintf("Range.End.Line->%+v\n", dh.Range.End.Line)
			s += fmt.Sprintf("Range.End.Character->%+v\n", dh.Range.End.Character)
			s += fmt.Sprintf("Kind->%+v\n\n", dh.Kind)
		*/
		rowNum := int(dh.Range.Start.Line)
		start := int(dh.Range.Start.Character)
		end := int(dh.Range.End.Character)
		e.highlightPositions = append(e.highlightPositions, Position{rowNum, start, end})
	}
	/*
		s += fmt.Sprintf("e.highlightPositions->%+v\n\n", e.highlightPositions)
		op.rows = strings.Split(s, "\n")
		op.drawText()
	*/

	var ab strings.Builder
	e.drawHighlights(&ab)
	fmt.Print(ab.String())

	sess.showEdMessage("Document Highlight Results: %d found", len(e.highlightPositions))

	sess.returnCursor()
}

//func (e *Editor) drawDefinition(definition []protocol.LocationLink) {
func (e *Editor) drawDefinition(definition []protocol.Location) {
	if len(definition) == 0 {
		return
	}
	/*
		var s string
		s = "definition:\n\n"
		for i, d := range definition {
			s += fmt.Sprintf("URI[%d]: %s\n", i, string(d.URI)[7:])
			s += fmt.Sprintf("Range.Start.Line->%+v\n", d.Range.Start.Line)
			s += fmt.Sprintf("Range.Start.Character->%+v\n", d.Range.Start.Character)
			s += fmt.Sprintf("Range.End.Line->%+v\n", d.Range.End.Line)
			s += fmt.Sprintf("Range.End.Character->%+v\n", d.Range.End.Character)
		}
	*/
	d := definition[0]
	filename := string(d.URI)[7:]
	rowNum := int(d.Range.Start.Line)
	//start := int(d.Range.Start.Character)
	r, err := os.Open(filename)
	if err != nil {
		sess.showEdMessage("Error opening file %s: %w", filename, err)
		return
	}
	defer r.Close()

	//var ss []string
	scanner := bufio.NewScanner(r)
	op := e.output
	op.rows = nil
	i := 0
	for scanner.Scan() {
		/*
				if i < rowNum {
					continue
				}
				if i > rowNum+10 {
					break
				}
			ss = append(ss, scanner.Text())
		*/
		op.rows = append(op.rows, scanner.Text())
		i++
	}
	sess.showEdMessage("len(op.rows) = %d; i = %d, rowNum = %d", len(op.rows), i, rowNum)
	op.rowOffset = rowNum
	/*
		op.rows = strings.Split(s, "\n")
		op.rows = append(op.rows, ss...)
	*/
	op.drawText()
}

func (e *Editor) drawReference(references []protocol.Location) {
	if len(references) == 0 {
		return
	}
	var s string
	s = "references:\n\n"
	for i, d := range references {
		s += fmt.Sprintf("URI[%d]: %s\n", i, string(d.URI)[7:])
		s += fmt.Sprintf("Range.Start.Line->%d\n", int(d.Range.Start.Line)+1)
		s += fmt.Sprintf("Range.Start.Character->%d\n", int(d.Range.Start.Character))
		//s += fmt.Sprintf("Range.End.Line->%+v\n", d.Range.End.Line)
		s += fmt.Sprintf("Range.End.Character->%d\n\n", int(d.Range.End.Character))
	}
	/*
		d := references[0]
		filename := string(d.URI)[7:]
		rowNum := int(d.Range.Start.Line)
		r, err := os.Open(filename)
		if err != nil {
			sess.showEdMessage("Error opening file %s: %w", filename, err)
			return
		}
		defer r.Close()

		scanner := bufio.NewScanner(r)
		op := e.output
		op.rows = nil
		i := 0
		for scanner.Scan() {
			op.rows = append(op.rows, scanner.Text())
			i++
		}
		sess.showEdMessage("len(op.rows) = %d; i = %d, rowNum = %d", len(op.rows), i, rowNum)
		op.rowOffset = rowNum
	*/
	op := e.output
	op.rows = strings.Split(s, "\n")
	op.rowOffset = 0
	op.drawText()
}

func (e *Editor) drawVisual(pab *strings.Builder) {

	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", e.left_margin+e.left_margin_offset)

	if e.mode == VISUAL_LINE {
		startRow := e.vb_highlight[0][1] - 1 // i think better to subtract one here
		endRow := e.vb_highlight[1][1] - 1   //ditto - done differently for visual and v_block

		x := e.left_margin + e.left_margin_offset + 1
		y := e.getScreenYFromRowColWW(startRow, 0) - e.lineOffset

		if y >= 0 {
			fmt.Fprintf(pab, "\x1b[%d;%dH\x1b[48;5;237m", y+e.top_margin, x) //244
		} else {
			fmt.Fprintf(pab, "\x1b[%d;%dH\x1b[48;5;237m", e.top_margin, x)
		}

		for n := 0; n < (endRow - startRow + 1); n++ { //++n
			rowNum := startRow + n
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
				//pab.Write(e.bb[rowNum][pos : pos+line_char_count])
				pab.Write(bytes.ReplaceAll(e.bb[rowNum][pos:pos+line_char_count], []byte("\t"), []byte("    ")))
				pab.WriteString(lf_ret)
				y += 1
				pos += line_char_count
			}
		}
	}

	if e.mode == VISUAL {
		startCol, endcol := e.vb_highlight[0][2], e.vb_highlight[1][2]

		// startRow always <= endRow and need to subtract 1 since counting starts at 1 not zero
		startRow, endRow := e.vb_highlight[0][1]-1, e.vb_highlight[1][1]-1 //startRow always <= endRow
		numrows := endRow - startRow + 1

		x := e.getScreenXFromRowColWW(startRow, startCol) + e.left_margin + e.left_margin_offset
		y := e.getScreenYFromRowColWW(startRow, startCol) + e.top_margin - e.lineOffset // - 1

		pab.WriteString("\x1b[48;5;237m")
		for n := 0; n < numrows; n++ {
			// i think would check here to see if a row has multiple lines (ie wraps)
			if n == 0 {
				fmt.Fprintf(pab, "\x1b[%d;%dH", y+n, x)
			} else {
				fmt.Fprintf(pab, "\x1b[%d;%dH", y+n, 1+e.left_margin+e.left_margin_offset)
			}
			row := e.bb[startRow+n]

			// I do not know why this works!!
			row = bytes.ReplaceAll(row, []byte("\t"), []byte(" "))

			if len(row) == 0 {
				continue
			}
			if numrows == 1 {
				//pab.Write(row[startCol-1 : endcol-1])
				pab.Write(row[startCol-1 : endcol])
			} else if n == 0 {
				pab.Write(row[startCol-1:])
			} else if n < numrows-1 {
				pab.Write(row)
			} else {
				if len(row) < endcol {
					pab.Write(row)
				} else {
					pab.Write(row[:endcol])
				}
			}
		}
	}

	if e.mode == VISUAL_BLOCK {

		var left, right int
		if e.vb_highlight[1][2] > e.vb_highlight[0][2] {
			right, left = e.vb_highlight[1][2], e.vb_highlight[0][2]
		} else {
			left, right = e.vb_highlight[1][2], e.vb_highlight[0][2]
		}

		x := e.getScreenXFromRowColWW(e.vb_highlight[0][1], left) + e.left_margin + e.left_margin_offset
		y := e.getScreenYFromRowColWW(e.vb_highlight[0][1], left) + e.top_margin - e.lineOffset - 1

		pab.WriteString("\x1b[48;5;237m")
		for n := 0; n < (e.vb_highlight[1][1] - e.vb_highlight[0][1] + 1); n++ {
			fmt.Fprintf(pab, "\x1b[%d;%dH", y+n, x)
			row := e.bb[e.vb_highlight[0][1]+n-1]
			rowLen := len(row)

			if rowLen == 0 || rowLen < left {
				continue
			}

			if rowLen < right {
				pab.Write(row[left-1 : rowLen])
			} else {
				pab.Write(row[left-1 : right])
			}
		}
	}

	pab.WriteString(RESET)
}

func (e *Editor) getLineCharCountWW(r, line int) int {
	row := e.bb[r]

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

		pos = bytes.LastIndex(row[prev_pos:pos+width], []byte(" "))

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
	e.highlightPositions = nil
	var rowNum int
	var start int
	var end int
	//curPos, _ := v.WindowCursor(w)
	curPos := vim.CursorGetPosition()
	//v.Command("set spell")
	vim.Execute("set spell")
	vim.Input("gg")
	vim.Input("]s")
	//first, _ := v.WindowCursor(w)
	first := vim.CursorGetPosition()
	rowNum = first[0] - 1
	start = first[1]
	vim.Execute("let length = strlen(expand('<cword>'))")
	ln_s := vim.Eval("length")
	ln, _ := strconv.Atoi(ln_s)
	end = start + ln
	e.highlightPositions = append(e.highlightPositions, Position{rowNum, start, end})
	var pos [2]int
	for {
		vim.Input("]s")
		pos = vim.CursorGetPosition()
		//pos, _ = v.WindowCursor(w)
		if pos == first {
			break
		}
		rowNum = pos[0] - 1
		// adjustment is made in drawHighlights
		//start = utf8.RuneCount(p.bb[rowNum][:pos[1]])
		start = pos[1]
		vim.Execute("let length = strlen(expand('<cword>'))")
		ln_s := vim.Eval("length")
		ln, _ := strconv.Atoi(ln_s)
		end = start + ln

		e.highlightPositions = append(e.highlightPositions, Position{rowNum, start, end})
	}
	//v.SetWindowCursor(w, curPos) //return cursor to where it was
	vim.CursorSetPosition(curPos) //return cursor to where it was

	// done here because no need to redraw text
	/*
		var ab strings.Builder
		e.drawHighlights(&ab)
		fmt.Print(ab.String())
		sess.showEdMessage("e.highlightPositions = %=v", e.highlightPositions)
	*/
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
		row := e.bb[p.rowNum]
		chars := "\x1b[48;5;31m" + string(row[p.start:p.end]) + "\x1b[0m"
		start := utf8.RuneCount(row[:p.start])
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
	numRows := len(e.bb)
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

		row := e.bb[filerow]

		if len(row) == 0 {
			ab.Write([]byte("\n"))
			filerow++
			y++
			continue
		}

		start := 0
		end := 0
		for {
			// if remainder of line is less than screen width
			if start+width > len(row)-1 {
				ab.Write(row[start:])
				ab.Write([]byte("\n"))
				y++
				filerow++
				break
			}

			pos := bytes.LastIndex(row[start:start+width], []byte(" "))
			if pos == -1 {
				end = start + width - 1
			} else {
				end = start + pos
			}

			ab.Write(row[start : end+1])
			ab.Write([]byte("\n"))
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
	numRows := len(e.bb)
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

		//row := e.bb[filerow]

		row := bytes.ReplaceAll(e.bb[filerow], []byte("\t"), []byte("    ")) ////////////

		if len(row) == 0 {
			ab.Write([]byte("\n"))
			filerow++
			y++
			continue
		}

		start := 0
		end := 0
		for {
			// if remainder of line is less than screen width
			if start+width > len(row)-1 {
				ab.Write(row[start:])
				ab.Write([]byte("\n"))
				y++
				filerow++
				break
			}

			pos := bytes.LastIndex(row[start:start+width], []byte(" "))
			if pos == -1 {
				end = start + width - 1
			} else {
				end = start + pos
			}

			ab.Write(row[start : end+1])
			if y == e.screenlines+e.lineOffset-1 {
				return ab.String()
			}
			ab.Write([]byte("\t"))
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

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("Error opening file %s: %w", filename, err)
	}
	e.bb = bytes.Split(b, []byte("\n"))
	vim.BufferSetLinesBBB(e.vbuf, e.bb)
	//e.bb = vim.BufferLines(e.vbuf)

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
	return vim.BufferGetModified(e.vbuf)
}
