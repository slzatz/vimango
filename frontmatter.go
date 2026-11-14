package main

import (
	"html"
	"strings"

	"github.com/slzatz/vimango/internal/frontmatter"
)

// The preview's half of YAML frontmatter: the card it shows the block as.
// What counts as frontmatter, and its rows, are internal/frontmatter's,
// which the editor asks too, so the two panes agree.

// frontmatterHTML renders the rows as a definition list. A dl because that
// is what this is, and because the stylesheet already colors dt with the
// preview accent -- the sheet's standing answer for the term half of a
// key/value pair, shared with list markers and the task checkbox. The
// class carries geometry only.
//
// Values are text, not markdown: a YAML scalar is a string, so backticks
// and underscores in a description stay on screen instead of turning into
// code spans and emphasis.
func frontmatterHTML(entries []frontmatter.Entry) string {
	var b strings.Builder
	b.WriteString("<dl class=\"frontmatter\">\n")
	for _, e := range entries {
		cls := ""
		if e.Depth > 0 {
			cls = " class=\"fm-l" + string(rune('0'+e.Depth)) + "\""
		}
		b.WriteString("<dt" + cls + ">")
		b.WriteString(html.EscapeString(e.Key))
		b.WriteString("</dt><dd" + cls + ">")
		b.WriteString(html.EscapeString(e.Value))
		b.WriteString("</dd>\n")
	}
	b.WriteString("</dl>\n")
	return b.String()
}
