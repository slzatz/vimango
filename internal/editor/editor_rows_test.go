package editor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func highlightEditor(t testing.TB, rows []string) *Editor {
	e := &Editor{}
	e.ss = rows
	e.ScreenCols = 60
	e.ScreenLines = 40
	e.highlightSyntax = true
	return e
}

// drawnRows returns every row of the note as visibleRows draws it.
func drawnRows(e *Editor) (out []string) {
	e.firstVisibleRow = 0
	e.visibleRows(func(n int, row string) bool {
		out = append(out, row)
		return true
	})
	return out
}

// The highlighting changes how the text is drawn, never the text: with the
// escapes taken out, every row is what wrappedRow draws with highlighting off
// -- tabs widened and wrapped at the same places.
func TestHighlightingKeepsTheText(t *testing.T) {
	note := []string{
		"---", "title: a **note**", "---",
		"# Heading", "",
		"**Bold starts this paragraph** and *italic* follows, with `code`, ~~strike~~ and a long tail so that it has to wrap at sixty columns, twice over.",
		"\tan indented **bold** row\twith a tab in it",
		"```go", "func main() {", "\tfmt.Println(\"hi\")", "}", "```",
		"é **ü** and → arrows", "",
	}
	e := highlightEditor(t, note)
	rows := drawnRows(e)
	if len(rows) != len(note) {
		t.Fatalf("%d rows in, %d drawn", len(note), len(rows))
	}
	for i, row := range rows {
		if got, want := stripANSI(row), e.wrappedRow(note[i]); got != want {
			t.Errorf("row %d: drawn text %q, want %q", i+1, got, want)
		}
		// each screen line of it starts from a reset state and leaves one
		for _, line := range strings.Split(row, "\t") {
			if line != "" && !strings.HasSuffix(line, sgrReset) {
				t.Errorf("row %d: screen line %q leaves its style open", i+1, line)
			}
		}
	}
}

// Each style's escape: its attributes, and the one colour that wins.
func TestMarkdownColours(t *testing.T) {
	for _, tc := range []struct {
		s    mdStyle
		want string
	}{
		{0, mdTextColour},
		{mdHeading, "\x1b[1m" + mdStrongColour},
		{mdBold, "\x1b[1m" + mdStrongColour},
		{mdItalic, "\x1b[3m" + mdTextColour},
		{mdStrike, "\x1b[9m" + mdTextColour},
		{mdCode, mdCodeColour},
		{mdHeading | mdCode, "\x1b[1m" + mdCodeColour},
	} {
		if got := tc.s.sgr(); got != tc.want {
			t.Errorf("style %s: %q, want %q", styleName(tc.s), got, tc.want)
		}
	}
}

// :syntax turns the highlighting on and off and leaves the margin alone. It
// used to make room for line numbers whenever it turned highlighting on, so
// with the numbers off it left a blank margin and wrapped the text short.
func TestSyntaxLeavesTheMarginAlone(t *testing.T) {
	e := newTestEditor(t, "# a note")
	for _, numbers := range []bool{false, true} {
		e.numberLines = numbers
		want := 0
		if numbers {
			want = lineNumberCols
		}
		e.left_margin_offset = want
		for range 2 { // off, then on again
			captureStdout(t, e.syntax)
			if e.left_margin_offset != want {
				t.Errorf("numbers %v, highlighting %v: margin %d, want %d", numbers, e.highlightSyntax, e.left_margin_offset, want)
			}
		}
	}
}

// :syntax and its aliases :md and :markdown turn the highlighting on and off,
// or toggle it with no argument, typed as the user types them. Anything else
// changes nothing.
func TestSyntaxOnOffAndAliases(t *testing.T) {
	e := newTestEditor(t, "# a note")
	for _, tc := range []struct {
		line string
		want bool
	}{
		{"syntax off", false},
		{"syntax on", true},
		{"md off", false},
		{"md", true},
		{"markdown", false},
		{"markdown on", true},
		{"syntax on", true}, // on when on stays on
		{"md sideways", true},
		{"syntax", false},
	} {
		out := captureStdout(t, func() { e.runExLine(tc.line) })
		if e.highlightSyntax != tc.want {
			t.Errorf(":%s: highlighting %v, want %v", tc.line, e.highlightSyntax, tc.want)
		}
		if tc.line == "md sideways" && !strings.Contains(out, "Usage: :md [on|off]") {
			t.Errorf(":%s: no usage message in %q", tc.line, out)
		}
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
// note and the edit changing one paragraph each time. chroma, run over the
// whole note, took 45ms here.
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
