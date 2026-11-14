package editor

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// The editor's markdown highlighting: headings, **bold**, *italic*,
// ~~strike~~, `inline code` and fenced code blocks, and nothing else. The
// note is shown as typed, markers and all; the highlighting only styles it.
//
// A row is styled on its own. The only thing carried from one row to the
// next is whether it is inside a fenced code block, so a redraw styles just
// the rows in the window, after a quick pass down the rows above it for
// fences. The YAML frontmatter at the top of a note is drawn plain. Notes are soft-wrapped, so a paragraph is one row, and
// the inline rules below -- CommonMark's, simplified -- see all of it.

// mdStyle is how one byte of a row is drawn: a set of these bits.
type mdStyle uint8

const (
	mdHeading mdStyle = 1 << iota // an ATX heading's row: # to ######
	mdBold                        // **text** or __text__, markers included
	mdItalic                      // *text* or _text_, markers included
	mdStrike                      // between ~~ and ~~ (the tildes are not struck)
	mdCode                        // an inline code span, backticks included, or a fenced block's row
)

// The colours, as they were when chroma drew the note with gruvbox_mod.xml:
// prose and italic in grey, headings and bold a near-white, code red.
const (
	mdTextColour   = "\x1b[38;2;196;196;196m" // #c4c4c4
	mdStrongColour = "\x1b[38;2;232;236;241m" // #e8ecf1
	mdCodeColour   = "\x1b[38;2;255;95;95m"   // #ff5f5f
)

// sgr returns the escape sequence that draws in style s, from a reset state.
func (s mdStyle) sgr() string {
	var b strings.Builder
	if s&(mdHeading|mdBold) != 0 {
		b.WriteString("\x1b[1m")
	}
	if s&mdItalic != 0 {
		b.WriteString("\x1b[3m")
	}
	if s&mdStrike != 0 {
		b.WriteString("\x1b[9m")
	}
	switch {
	case s&mdCode != 0:
		b.WriteString(mdCodeColour)
	case s&(mdHeading|mdBold) != 0:
		b.WriteString(mdStrongColour)
	default:
		b.WriteString(mdTextColour)
	}
	return b.String()
}

// mdBlock is what a row inherits from the rows above it: whether it is in a
// fenced code block, and the fence that will close it.
type mdBlock struct {
	fenceChar byte // '`' or '~' while inside a fenced block, else 0
	fenceLen  int  // the length of the opening fence's run
}

// fenceRun returns the character and length of the run of three or more
// backticks or tildes that row starts with, after any indentation, and what
// follows it. A list item's fence is indented, so any indentation is taken.
func fenceRun(row string) (c byte, n int, rest string) {
	t := strings.TrimLeft(row, " \t")
	if len(t) < 3 || (t[0] != '`' && t[0] != '~') {
		return 0, 0, ""
	}
	for n < len(t) && t[n] == t[0] {
		n++
	}
	if n < 3 {
		return 0, 0, ""
	}
	return t[0], n, t[n:]
}

// next returns the block state for the row after row, which had state b.
func (b mdBlock) next(row string) mdBlock {
	c, n, rest := fenceRun(row)
	if b.fenceChar != 0 {
		// a closer is a run of the same character, at least as long, alone
		if c == b.fenceChar && n >= b.fenceLen && strings.TrimSpace(rest) == "" {
			return mdBlock{}
		}
		return b
	}
	// an opener: a backtick fence's info string has no backtick in it (```c++ is one)
	if c != 0 && !(c == '`' && strings.ContainsRune(rest, '`')) {
		return mdBlock{fenceChar: c, fenceLen: n}
	}
	return b
}

// frontmatterRows returns how many rows the YAML block at the top of the
// note takes, both "---" rows included, or 0 if the note does not start with
// one.
func frontmatterRows(rows []string) int {
	if len(rows) == 0 || rows[0] != "---" {
		return 0
	}
	for i, r := range rows[1:] {
		if r == "---" {
			return i + 2
		}
	}
	return 0
}

// rowStyles returns the style of each byte of row, which has state b, and the
// state of the row after it. A fenced block's rows, its fences included, are
// all code.
func rowStyles(row string, b mdBlock) ([]mdStyle, mdBlock) {
	styles := make([]mdStyle, len(row))
	after := b.next(row)
	if b.fenceChar != 0 || after.fenceChar != 0 { // in a block, or its opener
		for i := range styles {
			styles[i] = mdCode
		}
		return styles, after
	}
	if isATXHeading(row) {
		for i := range styles {
			styles[i] = mdHeading
		}
	}
	styleInline(row, styles)
	return styles, after
}

// isATXHeading reports whether row is a heading: up to three spaces, one to
// six #, then a space, a tab or the end of the row.
func isATXHeading(row string) bool {
	i := 0
	for i < len(row) && i < 3 && row[i] == ' ' {
		i++
	}
	n := 0
	for i+n < len(row) && row[i+n] == '#' {
		n++
	}
	if n < 1 || n > 6 {
		return false
	}
	return i+n == len(row) || row[i+n] == ' ' || row[i+n] == '\t'
}

// delim is a run of *, _ or ~ that may open or close emphasis.
type delim struct {
	pos, n   int // where the unused part of the run starts, and its length
	origN    int // the run's length before any of it was used
	c        byte
	canOpen  bool
	canClose bool
}

// styleInline adds the inline styles to styles: code spans first, then
// emphasis and strikethrough outside them, by CommonMark's delimiter-run
// rules.
func styleInline(row string, styles []mdStyle) {
	var delims []delim
	for i := 0; i < len(row); {
		switch c := row[i]; {
		case c == '\\' && i+1 < len(row) && isASCIIPunct(row[i+1]):
			i += 2 // an escaped character is literal
		case c == '`':
			n := runLen(row, i)
			if end := closingBackticks(row, i+n, n); end >= 0 {
				for j := i; j < end+n; j++ {
					styles[j] |= mdCode
				}
				i = end + n
			} else {
				i += n // no closer: the backticks are literal
			}
		case c == '*' || c == '_' || c == '~':
			n := runLen(row, i)
			if c != '~' || n == 2 { // strikethrough is exactly ~~
				d := delim{pos: i, n: n, origN: n, c: c}
				d.canOpen, d.canClose = flanking(row, i, n)
				delims = append(delims, d)
			}
			i += n
		default:
			i++
		}
	}
	matchDelims(delims, styles)
}

// matchDelims pairs openers with closers, as CommonMark's "process emphasis"
// does, and styles what each pair encloses.
func matchDelims(delims []delim, styles []mdStyle) {
	for ci := 0; ci < len(delims); ci++ {
		for delims[ci].canClose && delims[ci].n > 0 {
			cl := &delims[ci]
			oi := ci - 1
			for ; oi >= 0; oi-- {
				o := &delims[oi]
				if o.c != cl.c || o.n == 0 || !o.canOpen {
					continue
				}
				// the rule of 3: in *foo**bar* the ** neither opens nor closes
				if o.c != '~' && (o.canClose || cl.canOpen) &&
					(o.origN+cl.origN)%3 == 0 && !(o.origN%3 == 0 && cl.origN%3 == 0) {
					continue
				}
				break
			}
			if oi < 0 {
				break // nothing to close
			}
			o := &delims[oi]
			use := 1
			if o.n >= 2 && cl.n >= 2 {
				use = 2
			}
			from, to := o.pos+o.n-use, cl.pos+use // the used markers and what they enclose
			switch {
			case o.c == '~':
				from, to = o.pos+o.n, cl.pos // the tildes are not struck
				for j := from; j < to; j++ {
					styles[j] |= mdStrike
				}
			case use == 2:
				for j := from; j < to; j++ {
					styles[j] |= mdBold
				}
			default:
				for j := from; j < to; j++ {
					styles[j] |= mdItalic
				}
			}
			o.n -= use
			cl.n -= use
			cl.pos += use
			for k := oi + 1; k < ci; k++ {
				delims[k].n = 0 // a run between them can no longer pair
			}
		}
	}
}

// flanking reports whether the run of n delimiters at row[i] can open and can
// close emphasis: CommonMark's left- and right-flanking rules, with _ not
// allowed to open or close inside a word (snake_case stays as it is).
func flanking(row string, i, n int) (canOpen, canClose bool) {
	before, after := ' ', ' ' // the start and end of the row count as space
	if i > 0 {
		before, _ = utf8.DecodeLastRuneInString(row[:i])
	}
	if i+n < len(row) {
		after, _ = utf8.DecodeRuneInString(row[i+n:])
	}
	spaceB, spaceA := unicode.IsSpace(before), unicode.IsSpace(after)
	punctB, punctA := isPunct(before), isPunct(after)

	left := !spaceA && (!punctA || spaceB || punctB)
	right := !spaceB && (!punctB || spaceA || punctA)
	if row[i] == '_' {
		return left && (!right || punctB), right && (!left || punctA)
	}
	return left, right
}

func isPunct(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) }

func isASCIIPunct(c byte) bool {
	return c >= '!' && c <= '/' || c >= ':' && c <= '@' || c >= '[' && c <= '`' || c >= '{' && c <= '~'
}

// runLen returns how many times row[i] repeats from i.
func runLen(row string, i int) int {
	n := 1
	for i+n < len(row) && row[i+n] == row[i] {
		n++
	}
	return n
}

// closingBackticks returns where the run of exactly n backticks that closes a
// code span opened before from starts, or -1 if there is none on the row.
func closingBackticks(row string, from, n int) int {
	for i := from; i < len(row); {
		if row[i] != '`' {
			i++
			continue
		}
		m := runLen(row, i)
		if m == n {
			return i
		}
		i += m
	}
	return -1
}

// styledSegment writes row[start:end] with each byte in its style, starting
// from a reset state and leaving one: drawRows positions every screen line
// itself, so none may rely on styling left open by the one above.
func styledSegment(sb *strings.Builder, row string, styles []mdStyle, start, end int) {
	if start == end {
		return
	}
	cur := styles[start]
	sb.WriteString(cur.sgr())
	from := start
	for i := start + 1; i < end; i++ {
		if styles[i] != cur {
			sb.WriteString(row[from:i])
			sb.WriteString(sgrReset)
			cur = styles[i]
			sb.WriteString(cur.sgr())
			from = i
		}
	}
	sb.WriteString(row[from:end])
	sb.WriteString(sgrReset)
}
