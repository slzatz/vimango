package main

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Drawing the note's rows, with markdown highlighting.
//
// The highlighter (chroma) is a lexer run over text, and until 2026-11-10 it
// was run on every redraw over everything from the first line of the note to
// the bottom of the window. Typing at the end of an 860-line note cost 45ms a
// keystroke, all of it re-highlighting text that had not changed and was not
// on screen.
//
// Now the note is cut into chunks that can be highlighted on their own -- a
// paragraph, a fenced code block, the frontmatter -- and each chunk's result
// is cached by its text. A redraw highlights the chunks in the window, and all
// but the one being edited come out of the cache. The cost of a keystroke no
// longer depends on how long the note is or where in it the cursor is.
//
// One thing changes on purpose. chroma's markdown lexer lets an unmatched
// backtick, "~~" or "[" run on across blank lines to the next one, however far
// away, colouring everything between. Markdown does not: those constructs end
// with the paragraph. Highlighting a paragraph at a time ends them there too.

// rowRange is rows [start, end) of the note. frontmatter marks the YAML block
// at the top of a note, which is highlighted as YAML.
type rowRange struct {
	start, end  int
	frontmatter bool
}

// maxChunkRows caps a run of text with no blank line in it. Without a cap, a
// 260-line list is one chunk, re-highlighted whole on every key typed in it.
const maxChunkRows = 60

// A fence opens with ``` and an optional language at the start of a row, and
// closes with a row that is exactly ```: chroma's rules, which decide what is
// highlighted as code.
var fenceOpener = regexp.MustCompile("^```\\w*$")

const fenceCloser = "```"

func rowIndex(rows []string, from int, match func(string) bool) int {
	for i := from; i < len(rows); i++ {
		if match(rows[i]) {
			return i
		}
	}
	return -1
}

// opensComment reports whether row starts an HTML comment that does not end on
// the same row. A "<!--" inside an inline code span is not one.
func opensComment(row string) bool {
	i := strings.LastIndex(row, "<!--")
	if i < 0 || strings.Contains(row[i:], "-->") {
		return false
	}
	return strings.Count(row[:i], "`")%2 == 0
}

// markdownChunks cuts the note into runs of rows that highlight the same alone
// as they do in the whole note:
//   - the YAML frontmatter, if the note starts with one
//   - each fenced code block, opener to closer
//   - each paragraph: rows up to and including the blank row that ends it. A
//     multi-line HTML comment stays in one piece, blank rows and all.
//
// A fence or comment with no closer is not one (to chroma either), and its
// rows are ordinary text.
func markdownChunks(rows []string) []rowRange {
	var chunks []rowRange
	start := 0
	flush := func(end int) {
		if end > start {
			chunks = append(chunks, rowRange{start: start, end: end})
		}
		start = end
	}

	r := 0
	if len(rows) > 0 && rows[0] == "---" {
		if end := rowIndex(rows, 1, func(s string) bool { return s == "---" }); end > 0 {
			flush(end + 1)
			chunks[0].frontmatter = true
			r = end + 1
		}
	}
	for r < len(rows) {
		row := rows[r]
		switch {
		case fenceOpener.MatchString(row):
			if end := rowIndex(rows, r+1, func(s string) bool { return s == fenceCloser }); end > 0 {
				flush(r)
				flush(end + 1)
				r = end + 1
				continue
			}
		case opensComment(row):
			if end := rowIndex(rows, r+1, func(s string) bool { return strings.Contains(s, "-->") }); end > 0 {
				r = end + 1 // the comment's rows stay with the paragraph it began in
				continue
			}
		}
		switch {
		case strings.TrimSpace(row) == "":
			flush(r + 1)
		case r+1-start >= maxChunkRows:
			flush(r + 1)
		}
		r++
	}
	flush(len(rows))
	return chunks
}

// wrappedRow returns buffer row raw as it goes on screen: tabs as four spaces,
// and a tab between the screen lines it wraps onto. (The tab is the separator
// because no real one is left; it rides through the highlighter, so colour
// that starts on one screen line carries onto the next.)
func (e *Editor) wrappedRow(raw string) string {
	row := strings.ReplaceAll(raw, "\t", "    ")
	segs := wrapRow(row, e.wrapWidth())
	if len(segs) == 1 {
		return row
	}
	var sb strings.Builder
	for i, seg := range segs {
		if i > 0 {
			sb.WriteByte('\t')
		}
		sb.WriteString(row[seg.start:seg.end])
	}
	return sb.String()
}

// The highlight cache is dropped when it passes this many bytes of
// highlighted text. Its keys include the wrap width, so resizing the window
// leaves entries behind that this clears.
const maxHighlightCacheBytes = 8 << 20

// highlightChunk returns the rows of one chunk highlighted, a string per row
// with its screen lines separated by tabs. The result is cached by the
// chunk's wrapped text.
//
// The text goes to the highlighter with a line break in front. chroma's rules
// for *emphasis*, **strong** and ~~strike~~ want whitespace before the marker,
// and for a row that starts with one that is the line break ending the row
// above -- which a chunk's first row does not have. The same line break keeps
// a paragraph that happens to start and end with "---" from being taken for
// frontmatter; the real frontmatter goes in bare, so that it is.
func (e *Editor) highlightChunk(rows []string, frontmatter bool) []string {
	var sb strings.Builder
	lead := 0
	if !frontmatter {
		sb.WriteByte('\n')
		lead = 1
	}
	for _, raw := range rows {
		sb.WriteString(e.wrappedRow(raw))
		sb.WriteByte('\n')
	}
	text := sb.String()
	if out, ok := e.hlCache[text]; ok {
		return out
	}

	var buf bytes.Buffer
	_ = Highlight2(&buf, text, "markdown", MDFormatter, e.Session.markdown_style)
	out := strings.Split(buf.String(), "\n")
	if len(out) < lead+len(rows) {
		// the highlighter lost a line break: show the rows plain rather than misplaced
		out = strings.Split(text, "\n")
	}
	out = out[lead : lead+len(rows)]

	if e.hlCache == nil || e.hlCacheBytes > maxHighlightCacheBytes {
		e.hlCache = make(map[string][]string)
		e.hlCacheBytes = 0
	}
	e.hlCache[text] = out
	e.hlCacheBytes += len(text) + buf.Len()
	return out
}

// visibleRows calls yield for each buffer row from the first visible one down,
// with the row as it is to be drawn (its screen lines separated by tabs),
// until yield returns false.
func (e *Editor) visibleRows(yield func(n int, row string) bool) {
	first := e.firstVisibleRow
	if first < 0 || first >= len(e.ss) {
		return
	}
	if !e.highlightSyntax {
		for n := first; n < len(e.ss); n++ {
			if !yield(n, e.wrappedRow(e.ss[n])) {
				return
			}
		}
		return
	}

	chunks := markdownChunks(e.ss)
	i := sort.Search(len(chunks), func(i int) bool { return chunks[i].end > first })
	for ; i < len(chunks); i++ {
		c := chunks[i]
		highlighted := e.highlightChunk(e.ss[c.start:c.end], c.frontmatter)
		for n := max(c.start, first); n < c.end; n++ {
			if !yield(n, highlighted[n-c.start]) {
				return
			}
		}
	}
}

// drawRows writes the visible part of the note: each row's screen lines, and
// its number in the margin if line numbers are on.
func (e *Editor) drawRows(pab *strings.Builder) {
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", e.left_margin)
	fmt.Fprintf(pab, "\x1b[?25l\x1b[%d;%dH", e.top_margin, e.left_margin+1)

	// The numbers are collected separately and written after the text, in a
	// second pass down the margin.
	var numCols strings.Builder
	indent := ""
	if e.numberLines {
		// a rectangle of background for the number column
		fmt.Fprintf(&numCols, "\x1b[2*x\x1b[%d;%d;%d;%d;48;5;235$r\x1b[*x",
			e.top_margin,
			e.left_margin,
			e.top_margin+e.screenlines,
			e.left_margin+e.left_margin_offset)
		fmt.Fprintf(&numCols, "\x1b[?25l\x1b[%d;%dH", e.top_margin, e.left_margin+1)
		indent = fmt.Sprintf("\x1b[%dC", e.left_margin_offset)
	}

	y := 0
	e.visibleRows(func(n int, row string) bool {
		if e.numberLines {
			fmt.Fprintf(&numCols, "\x1b[48;5;235m\x1b[38;5;245m%3d \x1b[49m", n+1)
		}
		for _, line := range strings.Split(row, "\t") {
			if y == e.screenlines {
				return false // the row runs past the bottom of the pane
			}
			pab.WriteString(indent)
			pab.WriteString(line)
			pab.WriteString(lf_ret)
			if e.numberLines {
				numCols.WriteString(lf_ret)
			}
			y++
		}
		return y < e.screenlines
	})
	pab.WriteString(numCols.String())
}
