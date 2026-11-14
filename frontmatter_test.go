package main

import (
	"strings"
	"testing"

	"github.com/slzatz/vimango/internal/frontmatter"
)

func TestFrontmatterHTMLEscapes(t *testing.T) {
	got := frontmatterHTML([]frontmatter.Entry{
		{Key: "a<b", Value: "x & <script>", Depth: 0},
		{Key: "n", Value: "v", Depth: 1},
	})
	if strings.Contains(got, "<script>") {
		t.Errorf("unescaped markup in output: %s", got)
	}
	if !strings.Contains(got, `<dt class="fm-l1">n</dt><dd class="fm-l1">v</dd>`) {
		t.Errorf("nesting class missing: %s", got)
	}
	if !strings.Contains(got, "a&lt;b") {
		t.Errorf("key not escaped: %s", got)
	}
}

// End to end, through the function --render-html calls.
func TestRenderNoteAsHTMLFrontmatter(t *testing.T) {
	out, err := htmlRenderer{}.render("t", "---\nname: a\nmetadata:\n  type: user\n---\n\nBody text.\n")
	if err != nil {
		t.Fatal(err)
	}
	content := out[strings.Index(out, `<div id="content">`):]
	if !strings.Contains(content, `<dl class="frontmatter">`) {
		t.Errorf("no frontmatter card:\n%s", content)
	}
	// The bug this whole file exists for: the closing fence read as a
	// setext underline and swallowed the block into a heading.
	if strings.Contains(content, "<h2") || strings.Contains(content, "<hr>") {
		t.Errorf("frontmatter still parsed as markdown:\n%s", content)
	}
	if !strings.Contains(content, "<p>Body text.</p>") {
		t.Errorf("body lost:\n%s", content)
	}
}
