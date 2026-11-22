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

// A block scalar's dd says how its lines are to be shown, and its text is
// escaped like any other value.
func TestFrontmatterHTMLBlockScalars(t *testing.T) {
	got := frontmatterHTML([]frontmatter.Entry{
		{Key: "style", Value: "section > h1 {\n  color: red;\n}", Literal: true},
		{Key: "css", Value: "a {}", Depth: 1, Literal: true},
		{Key: "description", Value: "one\ntwo"},
		{Key: "plain", Value: "one"},
	})
	for _, want := range []string{
		"<dt>style</dt><dd class=\"fm-literal\">section &gt; h1 {\n  color: red;\n}</dd>",
		`<dt class="fm-l1">css</dt><dd class="fm-l1 fm-literal">a {}</dd>`,
		"<dt>description</dt><dd class=\"fm-lines\">one\ntwo</dd>",
		`<dt>plain</dt><dd>one</dd>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the card lacks %q:\n%s", want, got)
		}
	}
}

// A Marp deck's frontmatter, "style: |" and all, is a card in the preview
// and none of it is read as markdown.
func TestRenderNoteAsHTMLDeckFrontmatter(t *testing.T) {
	out, err := htmlRenderer{}.render("t", "---\nmarp: true\nstyle: |\n  section {\n    display: flex;\n  }\n---\n\n# A slide\n")
	if err != nil {
		t.Fatal(err)
	}
	content := out[strings.Index(out, `<div id="content">`):]
	if !strings.Contains(content, "<dd class=\"fm-literal\">section {\n  display: flex;\n}</dd>") {
		t.Errorf("the style is not in the card as written:\n%s", content)
	}
	if strings.Contains(content, "<h2") || strings.Contains(content, "<hr>") {
		t.Errorf("frontmatter still parsed as markdown:\n%s", content)
	}
	if !strings.Contains(content, `<h1 id="a-slide">A slide</h1>`) {
		t.Errorf("body lost:\n%s", content)
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
