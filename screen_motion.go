package main

import (
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/slzatz/vimango/vim"
)

// The motions that move by what is on screen -- gj and gk, H, M and L. vim
// works them out from its own window, which in libvim draws nothing and
// wraps nothing, so libvim asks the embedder instead (vim.ScreenLayout), and
// the editor answers from the layout it draws: rows wrapped at word
// boundaries by wrapRow, tabs four columns wide. The editor is set as the
// layout when it opens (openEditor), and answers only while vim is on its
// buffer.

// screenWant is where the last gj or gk left the cursor, and the screen
// column it was after. A short screen line moves the cursor left of that
// column; the next gj or gk from the same place goes back to it, as vim's
// do.
type screenWant struct {
	line, col, curswant int // where gj/gk put the cursor, and the curswant it gave vim
	x                   int // the screen column wanted
}

func (e *Editor) onItsBuffer() bool {
	return e.vbuf != nil && vim.GetCurrentBuffer().GetID() == e.vbuf.GetID()
}

// screenLines returns line (1-based) of vim's buffer with its tabs expanded,
// as wrapRow measures it, and the screen lines it is drawn on.
func (e *Editor) screenLines(line int) (raw, text string, segs []wrapSegment) {
	raw = e.vbuf.Line(line)
	text = strings.ReplaceAll(raw, "\t", "$$$$")
	return raw, text, wrapRow(text, e.wrapWidth())
}

// ScreenPosition is gj (down) and gk: count screen lines from byte col of
// line, staying in the same screen column, or at the end of the screen line
// after $ (curswant vim.MaxCol). It moves as far as it can. The lines are
// read from vim, not e.ss: a mapping can change the text and move in one key.
func (e *Editor) ScreenPosition(down bool, count, line, col, curswant int) (int, int) {
	if !e.onItsBuffer() {
		return line, curswant
	}
	last := e.vbuf.LineCount()
	if line < 1 || line > last {
		return line, curswant
	}
	raw, text, segs := e.screenLines(line)
	c := expandedOffset(raw, col)
	si := segmentIndex(segs, c)

	atEnd := curswant == vim.MaxCol
	x := 0
	if w := e.screenWant; !atEnd && w.line == line && w.col == col && w.curswant == curswant {
		x = w.x
	} else if !atEnd {
		seg := segs[si]
		x = runewidth.StringWidth(text[seg.start:min(c, seg.end)])
		// On the last character vim can want a column past the end of the
		// line: where j or k came from, on a longer line.
		if raw != "" && col >= runeStart(raw, len(raw)-1) {
			if _, end := vim.VirtualColumns(line, col); curswant > end {
				x += curswant - end
			}
		}
	}

	for ; count > 0; count-- {
		switch {
		case down && si+1 < len(segs):
			si++
		case down && line < last:
			line++
			raw, text, segs = e.screenLines(line)
			si = 0
		case !down && si > 0:
			si--
		case !down && line > 1:
			line--
			raw, text, segs = e.screenLines(line)
			si = len(segs) - 1
		default:
			count = 1 // at the top or bottom: as far as it goes
		}
	}

	// the character in that screen line at the column wanted, or the last
	// one when the line is shorter
	seg := segs[si]
	part := text[seg.start:seg.end]
	b := len(part)
	if !atEnd {
		b = byteAtDisplayCol(part, x)
	}
	if b >= len(part) && part != "" {
		b = runeStart(part, len(part)-1)
	}
	dest := rawOffset(raw, seg.start+b)

	// vim moves to a virtual column, so the character's is what it is told
	// to want. After $ it goes on wanting the end (libvim puts MaxCol back).
	want, _ := vim.VirtualColumns(line, dest)
	e.screenWant = screenWant{line: line, col: dest, curswant: want, x: x}
	return line, want
}

// segmentIndex returns the index of the screen line holding byte offset c of
// a row, or the last when c is past the end (as wrapSegmentAt).
func segmentIndex(segs []wrapSegment, c int) int {
	for i, seg := range segs {
		if c < seg.end {
			return i
		}
	}
	return len(segs) - 1
}

// rawOffset is expandedOffset backwards: the byte of row that offset ec of
// its tab-expanded text falls in. Inside a tab's four columns is the tab.
func rawOffset(row string, ec int) int {
	e := 0
	for i := 0; i < len(row); i++ {
		w := 1
		if row[i] == '\t' {
			w = tabWidth
		}
		if ec < e+w {
			return i
		}
		e += w
	}
	return len(row)
}

// ScreenLine is H (vim.ScreenTop), M and L: the line count lines down from
// the first one on screen, up from the last, or the one in the middle of
// those drawn. It counts only rows drawn whole, as vim does; a row taller
// than the pane is one to itself.
func (e *Editor) ScreenLine(motion vim.ScreenLineMotion, count, line int) int {
	if !e.onItsBuffer() {
		return line
	}
	var rows []int // the rows on screen, 0-based
	height := 0    // the screen lines they take
	for n := max(e.firstVisibleRow, 0); n < len(e.ss); n++ {
		h := e.getLinesInRowWW(n)
		if height+h > e.screenlines && len(rows) > 0 {
			break
		}
		rows = append(rows, n)
		height += h
		if height >= e.screenlines {
			break
		}
	}
	if len(rows) == 0 {
		return line
	}
	count = max(count, 1)
	switch motion {
	case vim.ScreenTop:
		return rows[min(count, len(rows))-1] + 1
	case vim.ScreenBottom:
		return rows[max(len(rows)-count, 0)] + 1
	default: // M, as vim finds it: the row holding the middle screen line
		half := (height + 1) / 2
		used := 0
		for _, n := range rows {
			used += e.getLinesInRowWW(n)
			if used >= half {
				return n + 1
			}
		}
		return rows[len(rows)-1] + 1
	}
}
