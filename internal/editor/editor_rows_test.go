package editor

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestMarkdownChunks(t *testing.T) {
	ranges := func(rows ...string) (out [][2]int) {
		for _, c := range markdownChunks(rows) {
			out = append(out, [2]int{c.start, c.end})
		}
		return out
	}
	for _, tc := range []struct {
		name string
		rows []string
		want [][2]int
	}{
		{"paragraphs end with their blank row",
			[]string{"one", "two", "", "three", "", "", "four"},
			[][2]int{{0, 3}, {3, 5}, {5, 6}, {6, 7}}},
		{"a fence is one chunk, blank rows and all",
			[]string{"text", "```go", "a := 1", "", "b := 2", "```", "after"},
			[][2]int{{0, 1}, {1, 6}, {6, 7}}},
		{"a fence with no closer is ordinary text",
			[]string{"text", "```go", "a := 1", "", "more"},
			[][2]int{{0, 4}, {4, 5}}},
		{"an opener that chroma would not take is not one",
			[]string{"```c++", "x", "", "```", "y", "```"},
			[][2]int{{0, 3}, {3, 6}}},
		{"frontmatter",
			[]string{"---", "title: x", "", "tags: y", "---", "body", "", "---", "not frontmatter"},
			[][2]int{{0, 5}, {5, 7}, {7, 9}}},
		{"a rule on the first row with no second one is not frontmatter",
			[]string{"---", "body", "", "more"},
			[][2]int{{0, 3}, {3, 4}}},
		{"a comment keeps its blank rows",
			[]string{"before <!-- note", "", "still comment", "--> after", "", "next"},
			[][2]int{{0, 5}, {5, 6}}},
		{"a comment closed on its own row, or opened inside code, splits nothing",
			[]string{"a <!-- b --> c", "", "use `<!--` to open", "", "d"},
			[][2]int{{0, 2}, {2, 4}, {4, 5}}},
		{"empty note", []string{""}, [][2]int{{0, 1}}},
	} {
		if got := ranges(tc.rows...); !slices.Equal(got, tc.want) {
			t.Errorf("%s: chunks %v, want %v", tc.name, got, tc.want)
		}
	}

	// a long run with no blank row is cut, so a key typed in it does not
	// re-highlight all of it
	long := make([]string, 150)
	for i := range long {
		long[i] = "- item"
	}
	if got := ranges(long...); !slices.Equal(got, [][2]int{{0, 60}, {60, 120}, {120, 150}}) {
		t.Errorf("150 rows with no blank row: chunks %v", got)
	}
}

// seen reduces highlighted text to what is on screen: each visible character
// with the style in force when it is written. Two strings that differ only in
// where a colour is re-applied, or in the colour of a blank line, are equal.
func seen(x string) string {
	var out strings.Builder
	style := ""
	for i := 0; i < len(x); {
		if x[i] == 0x1b {
			j := i + 1
			for j < len(x) && !(x[j] >= 'a' && x[j] <= 'z' || x[j] >= 'A' && x[j] <= 'Z') {
				j++
			}
			if seq := x[i:min(j+1, len(x))]; seq == sgrReset {
				style = ""
			} else {
				style += seq
			}
			i = j + 1
			continue
		}
		if x[i] != ' ' && x[i] != '\t' {
			out.WriteString(style + "|")
		}
		out.WriteByte(x[i])
		i++
	}
	return out.String()
}

func highlightEditor(t testing.TB, rows []string) *Editor {
	style, err := LoadMarkdownStyle("gruvbox_mod.xml")
	if err != nil {
		t.Skip(err)
	}
	e := &Editor{Host: Host{MarkdownStyle: style}}
	e.ss = rows
	e.ScreenCols = 60
	e.ScreenLines = 40
	e.highlightSyntax = true
	return e
}

func chunkedRows(e *Editor) (out []string) {
	for _, c := range markdownChunks(e.ss) {
		out = append(out, e.highlightChunk(e.ss[c.start:c.end], c.frontmatter)...)
	}
	return out
}

// Highlighting a chunk at a time must look the same as highlighting the whole
// note at once, which is what it replaced. (Run over the 5,574 notes of a real
// database when this was written: 5,552 came out the same; the other 22 each
// had an unmatched backtick, covered by the next test.)
func TestChunkedHighlightingLooksLikeTheWholeNote(t *testing.T) {
	note := strings.Split(strings.Join([]string{
		"---", "title: a note", "tags: [one, two]", "---",
		"# Heading", "",
		"**Bold starts this paragraph** and *italic* follows, with `code` and a [link](http://example.com).",
		"A second line of the same paragraph with ~~strike~~ and @mention #tag.", "",
		"*Italic starts this one*", "",
		"- a list item with **bold**", "- [x] a done task", "  - nested *item*", "1. numbered", "",
		"> a quotation", "",
		"```go", "func main() {", "", "\tfmt.Println(\"hi\") // comment", "}", "```", "",
		"Text <!-- a comment", "", "that runs over a blank row --> and ends.", "",
		"```", "plain fence", "```",
		"A long line that has to wrap because it is quite a bit wider than the sixty columns this editor has, twice over if need be.", "",
		"## Subheading", "last line",
	}, "\n"), "\n")
	e := highlightEditor(t, note)

	var sb strings.Builder
	for _, raw := range e.ss {
		sb.WriteString(e.wrappedRow(raw))
		sb.WriteByte('\n')
	}
	var buf bytes.Buffer
	if err := highlightText(&buf, sb.String(), "markdown", markdownFormatter, e.MarkdownStyle); err != nil {
		t.Fatal(err)
	}
	whole := strings.Split(buf.String(), "\n")

	chunked := chunkedRows(e)
	if len(chunked) != len(note) {
		t.Fatalf("%d rows in, %d highlighted rows out", len(note), len(chunked))
	}
	for i := range note {
		if seen(whole[i]) != seen(chunked[i]) {
			t.Errorf("row %d %q\n whole note: %q\n by chunk:   %q", i+1, note[i], whole[i], chunked[i])
		}
	}
	// and the text itself is all there, in order
	for i, row := range chunked {
		if got, want := stripANSI(row), e.wrappedRow(note[i]); got != want {
			t.Errorf("row %d: highlighted text %q, want %q", i+1, got, want)
		}
	}
}

// The one intended difference. chroma lets an unmatched backtick run on to the
// next backtick however many paragraphs away, colouring everything between as
// code; markdown ends a code span with its paragraph, and so does a chunk.
func TestUnmatchedBacktickStopsAtTheParagraph(t *testing.T) {
	e := highlightEditor(t, []string{"a stray ` backtick", "", "an ordinary paragraph", "", "and `code` here"})
	rows := chunkedRows(e)
	plain := seen(e.highlightChunk([]string{"an ordinary paragraph"}, false)[0])
	if seen(rows[2]) != plain {
		t.Errorf("the paragraph after a stray backtick is coloured %q, want it plain %q", rows[2], plain)
	}
}

// A paragraph that begins with a marker is highlighted, including the very
// first row of the note -- which whole-note highlighting missed, the lexer
// wanting whitespace before the marker.
func TestMarkerAtTheStartOfAChunk(t *testing.T) {
	e := highlightEditor(t, []string{"**bold first row**", "", "**bold after a blank row**"})
	rows := chunkedRows(e)
	plain := seen(e.highlightChunk([]string{"x"}, false)[0])
	for i := range []int{0, 2} {
		if strings.HasPrefix(seen(rows[i*2]), plain[:len(plain)-1]) {
			t.Errorf("row %d is not highlighted as bold: %q", i*2+1, rows[i*2])
		}
	}
}

// Only the chunk that changed is highlighted again.
func TestHighlightCache(t *testing.T) {
	rows := []string{"# Title", "", "first paragraph", "", "second paragraph", "", "```", "code", "```"}
	e := highlightEditor(t, rows)
	draw := func() {
		var ab strings.Builder
		e.drawRows(&ab)
	}
	draw()
	cached := len(e.hlCache)
	if cached == 0 {
		t.Fatal("nothing was cached")
	}
	draw()
	if len(e.hlCache) != cached {
		t.Errorf("redrawing unchanged text added %d cache entries", len(e.hlCache)-cached)
	}
	e.ss[4] = "second paragraph, edited"
	draw()
	if got := len(e.hlCache) - cached; got != 1 {
		t.Errorf("editing one paragraph highlighted %d chunks, want 1", got)
	}
}

// drawRows fills the pane and no more, starts at the first visible row, and
// numbers the rows by their place in the note.
func TestDrawRowsFillsThePane(t *testing.T) {
	rows := make([]string, 200)
	for i := range rows {
		rows[i] = fmt.Sprintf("row number %d", i+1)
		if i%5 == 4 {
			rows[i] = ""
		}
	}
	rows[101] = strings.Repeat("wrapping ", 20) // four screen lines at this width
	for _, highlight := range []bool{true, false} {
		e := highlightEditor(t, rows)
		e.highlightSyntax = highlight
		e.numberLines = true
		e.left_margin_offset = lineNumberCols
		e.ScreenLines = 10
		e.firstVisibleRow = 100

		var ab strings.Builder
		e.drawRows(&ab)
		out := stripANSI(ab.String())
		if n := strings.Count(ab.String(), "\r\n") / 2; n != e.ScreenLines {
			t.Errorf("highlight=%v: %d screen lines written, want %d", highlight, n, e.ScreenLines)
		}
		// rows 101..107 fill the ten lines: 102 takes four of them
		for _, want := range []string{"row number 101", "wrapping wrapping", "row number 103", "row number 107", "101 ", "102 ", "107 "} {
			if !strings.Contains(out, want) {
				t.Errorf("highlight=%v: output lacks %q", highlight, want)
			}
		}
		for _, not := range []string{"row number 99", "row number 108", "108 "} {
			if strings.Contains(out, not) {
				t.Errorf("highlight=%v: output has %q, which is outside the pane", highlight, not)
			}
		}
	}
}

// longNote is 900 rows of ordinary markdown in paragraphs, the shape of a long
// real note.
func longNote() []string {
	var rows []string
	for len(rows) < 900 {
		rows = append(rows,
			fmt.Sprintf("## Section %d", len(rows)),
			"",
			"A paragraph with **bold**, *italic*, `code` and a [link](http://example.com/page) in it, long enough to wrap.",
			"A second line of it.",
			"",
			"- an item", "- another item with `code`", "- [x] a done one",
			"",
		)
	}
	return rows
}

// The cost of redrawing after a key, with the cursor at the bottom of a long
// note and the edit changing one paragraph each time. Highlighting everything
// from the top of the note, as before, took 45ms here; see BenchmarkWholeNote.
func BenchmarkRedrawAtBottomOfLongNote(b *testing.B) {
	e := highlightEditor(b, longNote())
	e.fr = len(e.ss) - 1
	e.scroll()
	var ab strings.Builder
	e.drawRows(&ab)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ss[len(e.ss)-5] = fmt.Sprintf("A second line of it, as typed %d.", i)
		e.scroll()
		ab.Reset()
		e.drawRows(&ab)
	}
}

func BenchmarkWholeNote(b *testing.B) {
	e := highlightEditor(b, longNote())
	var sb strings.Builder
	for _, raw := range e.ss {
		sb.WriteString(e.wrappedRow(raw))
		sb.WriteByte('\n')
	}
	text := sb.String()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		highlightText(&buf, text, "markdown", markdownFormatter, e.MarkdownStyle)
	}
}

// foregroundLeftSet reports whether out ends with a foreground colour still
// in effect: an SGR that sets one (30-37, 38;..., 90-97) with no reset (0, or
// 39) after it.
func foregroundLeftSet(out string) bool {
	set := false
	for _, m := range regexp.MustCompile(`\x1b\[([0-9;]*)m`).FindAllStringSubmatch(out, -1) {
		params := strings.Split(m[1], ";")
		for i := 0; i < len(params); i++ {
			n, _ := strconv.Atoi(params[i]) // "" is 0
			switch {
			case n == 0 || n == 39:
				set = false
			case n >= 30 && n <= 37, n >= 90 && n <= 97:
				set = true
			case n == 38:
				set = true
				i = len(params) // the rest are the colour's own numbers
			case n == 48:
				i = len(params)
			}
		}
	}
	return set
}

// Drawing the text leaves no colour behind for whatever is drawn next. The
// line numbers set a grey foreground and put back only the background, so
// the TUI's status bar turned grey at the first key that redrew the text, as
// did a message shown after a redraw (:number, :syntax) -- and the visual
// selection was grey or not by the same accident. The selection's grey is
// now its own (shadedStyle).
func TestDrawingTheTextLeavesNoColourSet(t *testing.T) {
	e := newTestEditor(t, "alpha (bravo)", "charlie", "delta")
	e.numberLines = true
	e.left_margin_offset = lineNumberCols

	if out := captureStdout(t, e.drawText); !strings.Contains(out, "  1 ") || foregroundLeftSet(out) {
		t.Errorf("drawText with line numbers left a foreground colour set: %q", out)
	}

	// a selection is drawn in its own grey, with the numbers on or off
	for _, numbers := range []bool{true, false} {
		e.numberLines = numbers
		captureStdout(t, func() { e.sendKeys("vj") })
		out := captureStdout(t, e.drawText)
		if !strings.Contains(out, shadedStyle) || foregroundLeftSet(out) {
			t.Errorf("a selection, line numbers %v: drawn %q", numbers, out)
		}
		captureStdout(t, func() { e.ProcessKey('\x1b') })
	}
}
