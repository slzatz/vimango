package editor

import (
	"fmt"
	"strings"
	"testing"
)

// styleName spells a style as letters: Heading, Bold, Italic, Strike, Code.
func styleName(s mdStyle) string {
	var b strings.Builder
	for i, c := range "HBISC" {
		if s&(1<<i) != 0 {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// markup shows row with each styled run wrapped as {style:text}, so that
// "a {B:**b**} c" reads as the bold run and its markers.
func markup(row string, styles []mdStyle) string {
	var sb strings.Builder
	for i := 0; i < len(row); {
		j := i
		for j < len(row) && styles[j] == styles[i] {
			j++
		}
		if styles[i] == 0 {
			sb.WriteString(row[i:j])
		} else {
			fmt.Fprintf(&sb, "{%s:%s}", styleName(styles[i]), row[i:j])
		}
		i = j
	}
	return sb.String()
}

// markupRows styles rows as the editor does, from the top of a note, and
// returns each one marked up.
func markupRows(rows ...string) []string {
	var out []string
	var b mdBlock
	for _, r := range rows {
		var styles []mdStyle
		styles, b = rowStyles(r, b)
		out = append(out, markup(r, styles))
	}
	return out
}

func TestInlineMarkdown(t *testing.T) {
	for _, tc := range []struct{ row, want string }{
		{"plain words", "plain words"},

		// headings
		{"# Head", "{H:# Head}"},
		{"###### six", "{H:###### six}"},
		{"   # three spaces in", "{H:   # three spaces in}"},
		{"#", "{H:#}"},
		{"####### seven is too many", "####### seven is too many"},
		{"#hashtag", "#hashtag"},
		{"    # four spaces in", "    # four spaces in"},
		{"## Head with `code`", "{H:## Head with }{HC:`code`}"},
		{"# Head with **bold**", "{H:# Head with }{HB:**bold**}"},

		// bold and italic, markers included
		{"a **bold** b", "a {B:**bold**} b"},
		{"a __bold__ b", "a {B:__bold__} b"},
		{"a *it* b", "a {I:*it*} b"},
		{"a _it_ b", "a {I:_it_} b"},
		{"**bold first**", "{B:**bold first**}"},
		{"***both***", "{I:*}{BI:**both**}{I:*}"},
		{"**bold with *italic* inside**", "{B:**bold with }{BI:*italic*}{B: inside**}"},
		{"*it with **bold** inside*", "{I:*it with }{BI:**bold**}{I: inside*}"},
		{"a**b**c", "a{B:**b**}c"},
		{"*a* and *b*", "{I:*a*} and {I:*b*}"},

		// not emphasis
		{"snake_case_name", "snake_case_name"},
		{"a_b_c and _d_", "a_b_c and {I:_d_}"},
		{"2 * 3 * 4", "2 * 3 * 4"},
		{"**unclosed", "**unclosed"},
		{"closed** only", "closed** only"},
		{"\\*escaped\\*", "\\*escaped\\*"},
		{"* a list item", "* a list item"},
		{"- a *list* item", "- a {I:*list*} item"},

		// strikethrough: the tildes are not struck
		{"a ~~st~~ b", "a ~~{S:st}~~ b"},
		{"~~st~~", "~~{S:st}~~"},
		{"a ~~st **b**~~", "a ~~{S:st }{BS:**b**}~~"},
		{"about ~5 minutes ~ok", "about ~5 minutes ~ok"},
		{"a ~~~not~~~ b", "a ~~~not~~~ b"},
		{"~~ spaced ~~", "~~ spaced ~~"},

		// inline code, backticks included, with nothing styled inside it
		{"a `code` b", "a {C:`code`} b"},
		{"`code first`", "{C:`code first`}"},
		{"`` a ` b ``", "{C:`` a ` b ``}"},
		{"`**not bold**`", "{C:`**not bold**`}"},
		{"a stray ` backtick", "a stray ` backtick"},
		{"``two` mismatched", "``two` mismatched"},
		{"**bold `code` in it**", "{B:**bold }{BC:`code`}{B: in it**}"},

		// bytes, not runes, are styled; multi-byte text is no different
		{"é **ü** →", "é {B:**ü**} →"},
		{"«*fr*»", "«{I:*fr*}»"},
	} {
		if got := markupRows(tc.row)[0]; got != tc.want {
			t.Errorf("%q:\n got  %s\n want %s", tc.row, got, tc.want)
		}
	}
}

func TestFencedCode(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []string
		want []string
	}{
		{"a fence and its markers are code; what follows is not",
			[]string{"```go", "x := `y` **z**", "", "```", "**after**"},
			[]string{"{C:```go}", "{C:x := `y` **z**}", "", "{C:```}", "{B:**after**}"}},
		{"tildes",
			[]string{"~~~", "# not a heading", "~~~"},
			[]string{"{C:~~~}", "{C:# not a heading}", "{C:~~~}"}},
		{"any info string, c++ included",
			[]string{"```c++", "int x;", "```"},
			[]string{"{C:```c++}", "{C:int x;}", "{C:```}"}},
		{"an indented fence, as in a list item",
			[]string{"- item", "  ```", "  code", "  ```", "- next"},
			[]string{"- item", "{C:  ```}", "{C:  code}", "{C:  ```}", "- next"}},
		{"a shorter run, or the other character, does not close",
			[]string{"````", "```", "~~~", "````", "x"},
			[]string{"{C:````}", "{C:```}", "{C:~~~}", "{C:````}", "x"}},
		{"a closer has nothing after it",
			[]string{"```", "``` not a closer", "```"},
			[]string{"{C:```}", "{C:``` not a closer}", "{C:```}"}},
		{"a fence with no closer runs to the end of the note",
			[]string{"```", "code", "", "**more code**"},
			[]string{"{C:```}", "{C:code}", "", "{C:**more code**}"}},
		{"a backtick in the info string: not a fence",
			[]string{"``` a ` b", "*x*"},
			[]string{"``` a ` b", "{I:*x*}"}},
	} {
		got := markupRows(tc.rows...)
		if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
			t.Errorf("%s:\n got  %q\n want %q", tc.name, got, tc.want)
		}
	}
}

// The frontmatter is plain: a "# comment" in the YAML is not a heading. Only
// a block at the very top is frontmatter, and it needs both of its rules.
func TestFrontmatterIsPlain(t *testing.T) {
	for _, tc := range []struct {
		rows []string
		want int
	}{
		{[]string{"---", "title: x", "# yaml comment", "---", "# body"}, 4},
		{[]string{"---", "body with no second rule"}, 0},
		{[]string{"text", "---", "x: y", "---"}, 0},
		{[]string{}, 0},
	} {
		if got := frontmatterRows(tc.rows); got != tc.want {
			t.Errorf("%q: %d frontmatter rows, want %d", tc.rows, got, tc.want)
		}
	}

	e := highlightEditor(t, []string{"---", "# yaml comment", "---", "# Heading"})
	rows := drawnRows(e)
	plain := mdStyle(0).sgr() + "# yaml comment" + sgrReset
	if rows[1] != plain {
		t.Errorf("frontmatter row drawn %q, want it plain %q", rows[1], plain)
	}
	if !strings.HasPrefix(rows[3], mdHeading.sgr()) {
		t.Errorf("the heading after the frontmatter is drawn %q", rows[3])
	}
}

// A redraw that starts below a fence's opener knows it is inside the fence:
// the rows above the window are read for fences.
func TestFenceAboveTheWindow(t *testing.T) {
	e := highlightEditor(t, []string{"```", "one", "two", "```", "**three**"})
	e.firstVisibleRow = 2
	var got []string
	e.visibleRows(func(n int, row string) bool {
		got = append(got, row)
		return true
	})
	if want := mdCode.sgr() + "two" + sgrReset; got[0] != want {
		t.Errorf("row inside a fence drawn %q, want %q", got[0], want)
	}
	if !strings.HasPrefix(got[2], mdBold.sgr()) {
		t.Errorf("row after the fence drawn %q, want it bold", got[2])
	}
}
