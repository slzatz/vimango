// I think this is for windows that display the output of a compiled program
package main

import (
	"fmt"
	"strings"
)

type Output struct {
	rowOffset   int //first row based on user scroll
	screenlines int //number of lines for this Window
	screencols  int //number of columns for this Window
	left_margin int
	top_margin  int
	is_below    bool
	rows        []string
	id          int // db id of related entry
}

/*
func NewOutput() *Output {
	return &Output{
		rowOffset: 0, //the number of lines of text at the top scrolled off the screen
		is_below:  false,
	}
}
*/

func (o *Output) drawText() {
	// probably unnecessary
	if len(o.rows) == 0 {
		o.drawStatusBar()
		return
	}
	var ab strings.Builder

	fmt.Fprintf(&ab, "\x1b[?25l\x1b[%d;%dH", o.top_margin, o.left_margin+1)
	// \x1b[NC moves cursor forward by n columns
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", o.left_margin)
	erase_chars := fmt.Sprintf("\x1b[%dX", o.screencols)
	for i := 0; i < o.screenlines; i++ {
		ab.WriteString(erase_chars)
		ab.WriteString(lf_ret)
	}

	// format for positioning cursor is "\x1b[%d;%dh"
	fmt.Fprintf(&ab, "\x1b[%d;%dH", o.top_margin, o.left_margin+1)

	y := 0
	filerow := o.rowOffset

	for {

		if y == o.screenlines-1 {
			break
		}

		if filerow == len(o.rows) {
			break
		}

		row := o.rows[filerow]

		if len(row) == 0 {
			ab.WriteString(lf_ret)
			filerow++
			y++
			continue
		}

		start := 0
		end := 0
		for {
			if start+o.screencols > len(row)-1 {
				ab.WriteString(row[start:])
				if y == o.screenlines-1 {
					break
				}
				ab.WriteString(lf_ret)
				y++
				filerow++
				break
			}

			pos := strings.LastIndex(row[start:start+o.screencols], " ")

			if pos == -1 {
				end = start + o.screencols - 1
			} else {
				end = start + pos
			}

			ab.WriteString(row[start : end+1])
			if y == o.screenlines-1 {
				break
			}
			ab.WriteString(lf_ret)
			start = end + 1
			y++
		}
	}
	fmt.Print(ab.String())
}

func (o *Output) drawStatusBar() {
	var ab strings.Builder
	fmt.Fprintf(&ab, "\x1b[%d;%dH", o.screenlines+o.top_margin, o.left_margin+1)

	//erase from start of an Editor's status bar to the end of the Editor's status bar
	fmt.Fprintf(&ab, "\x1b[%dX", o.screencols)

	ab.WriteString("\x1b[7m ") //switches to inverted colors

	status := fmt.Sprintf("%d Output", o.id)

	if len(status) > o.screencols-1 {
		status = status[:o.screencols-1]
	}
	fmt.Fprintf(&ab, "%-*s", o.screencols, status)
	ab.WriteString("\x1b[0m") //switches back to normal formatting
	fmt.Print(ab.String())
}

func (o *Output) setLinesMargins() { //also sets top margin

	if o.is_below {
		o.screenlines = LINKED_NOTE_HEIGHT
		o.top_margin = sess.textLines - LINKED_NOTE_HEIGHT + 2
	} else {
		o.screenlines = sess.textLines
		o.top_margin = TOP_MARGIN + 1
	}
}

func (o *Output) drawFrame() {
	var ab strings.Builder
	ab.WriteString("\x1b(0") // Enter line drawing mode

	for j := 1; j < o.screenlines+1; j++ {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", o.top_margin-1+j, o.left_margin+o.screencols+1)
		// below x = 0x78 vertical line (q = 0x71 is horizontal) 37 = white; 1m = bold (note
		// only need one 'm'
		ab.WriteString("\x1b[37;1mx")
	}

	//'T' corner = w or right top corner = k
	fmt.Fprintf(&ab, "\x1b[%d;%dH", o.top_margin-1, o.left_margin+o.screencols+1)

	if o.left_margin+o.screencols > sess.screenCols-4 {
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
