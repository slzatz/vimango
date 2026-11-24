package main

import (
	"reflect"
	"strings"
	"testing"
)

// deckContent renders body as a Marp deck and returns what is inside the
// page's content, the frontmatter card included.
func deckContent(t *testing.T, body string) string {
	t.Helper()
	return renderedContent(t, "---\nmarp: true\n---\n"+body)
}

func renderedContent(t *testing.T, note string) string {
	t.Helper()
	out, err := htmlRenderer{}.render("t", note)
	if err != nil {
		t.Fatal(err)
	}
	return out[strings.Index(out, `<div id="content">`):]
}

func TestIsMarpDeck(t *testing.T) {
	for fm, want := range map[string]bool{
		"---\nmarp: true\n---\n":                           true,
		"---\ntheme: gaia\nmarp:  true  # a deck\n---\n":   true,
		"---\r\nmarp: true\r\n---\r\n":                     true,
		"---\nmarp: false\n---\n":                          false,
		"---\nname: x\nmetadata:\n  marp: true\n---\n":     false,
		"---\ndescription: say marp: true at top\n---\n":   false,
		"---\nmarp: truely\n---\n":                         false,
		"---\n# a comment Parse declines\nmarp: true\n---": true,
	} {
		if got := isMarpDeck(fm); got != want {
			t.Errorf("isMarpDeck(%q) = %v, want %v", fm, got, want)
		}
	}
}

// Which comments are directives. Each was given to marp-cli 4.5.1: a nil here
// is a comment it read as presenter notes.
func TestMarpDirectives(t *testing.T) {
	one := func(text string) []marpLabel { return []marpLabel{{text: text}} }
	for _, tc := range []struct {
		in   string
		want []marpLabel
	}{
		{"_backgroundPosition: top left", one("_backgroundPosition: top left")},
		{"backgroundPosition: 60% 10%", one("backgroundPosition: 60% 10%")},
		{"_backgroundPosition: -10px 0", one("_backgroundPosition: -10px 0")},
		{"_backgroundColor: #123456", one("_backgroundColor: #123456")},
		{`class: ""`, one(`class: ""`)},
		{`"_class": a`, one("_class: a")},
		{"_class: [a, b]", one("_class: [a, b]")},
		{"_class:\n    - a\n    - b", one("_class: a, b")},
		{"_footer: Note: hi", one("_footer: Note: hi")},
		{"transition: fade", one("transition: fade")},
		{"# a comment\n_class: a", one("_class: a")},
		{"_class: a\nFoo: bar\n_color: red", []marpLabel{{text: "_class: a"}, {text: "_color: red"}}},
		{"_class: a\r\n_color: red", []marpLabel{{text: "_class: a"}, {text: "_color: red"}}},
		{"style: |\n  h2 {\n    color: red;\n  }\n\n  h3 {}\ntheme: gaia", []marpLabel{
			{text: "style", tooltip: "h2 {\n  color: red;\n}\n\nh3 {}"}, {text: "theme: gaia"}}},

		{"Say hello.", nil},
		{"Talk about this.\n_class: a", nil},
		{"_class: a\nnonsense here", nil},
		{"_class: a\n  _color: red", nil},
		{"_class:a", nil},
		{"$class: a", nil},
		{"_footer: **hi**", nil},
		{"transition: a: b", nil},
		{"_class: a\n_class: b", nil},
		{"Foo: bar", nil},
		{"_backgroundPostion: top", nil}, // misspelled
		{"_theme: gaia", nil},            // the whole deck's, so no "_" form
		{"", nil},
	} {
		if got := marpDirectives(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("marpDirectives(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestDeckDirectivesAreLabels(t *testing.T) {
	got := deckContent(t, `# One

<!-- _backgroundPosition: top left -->
![bg right:65%](x.png)

## Two <!-- _class: a<b -->

<!--
_class: x
_color: red
-->

- item
  <!-- _footer: in a list -->
`)
	for _, want := range []string{
		`<div class="marp"><span class="marp-label">_backgroundPosition: top left</span></div>`,
		`<span class="marp-label">_class: a&lt;b</span></h2>`,
		`<div class="marp"><span class="marp-label">_class: x</span> <span class="marp-label">_color: red</span></div>`,
		`<span class="marp-label">_footer: in a list</span>`,
		`<img src="x.png"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the deck lacks %q:\n%s", want, got)
		}
	}
}

// Presenter notes stay out of sight, as they are on the slide.
func TestDeckPresenterNotesAreNotLabels(t *testing.T) {
	got := deckContent(t, "# One\n\n<!-- Thank everyone. -->\n\ntext <!-- and pause --> text\n")
	if strings.Contains(got, "marp-label") {
		t.Errorf("presenter notes became a label:\n%s", got)
	}
	if !strings.Contains(got, "<!-- Thank everyone. -->") || !strings.Contains(got, "<!-- and pause -->") {
		t.Errorf("a comment was changed:\n%s", got)
	}
}

func TestDeckStyleBlocksAreDropped(t *testing.T) {
	got := deckContent(t, `# One

<style scoped>
h1 { color: "red"; }

img { width: 1px; }
</style>
![bg](x.png)

<STYLE>p {}</STYLE> after

- item
  <style scoped>li {}</style>
`)
	if strings.Contains(strings.ToLower(got), "<style") {
		t.Errorf("a style block reached the page:\n%s", got)
	}
	for _, want := range []string{
		`<div class="marp"><span class="marp-label" title="h1 { color: &#34;red&#34;; }` + "\n\n" + `img { width: 1px; }">style scoped</span></div>`,
		`<span class="marp-label" title="p {}">style</span></div> after`,
		`title="li {}">style scoped</span>`,
		`<img src="x.png"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the deck lacks %q:\n%s", want, got)
		}
	}
}

// Where Marp does not take a style tag for CSS it shows it as text, and so
// does the preview: no text is lost, and no CSS gets in.
func TestDeckStyleTagsElsewhereAreText(t *testing.T) {
	for _, body := range []string{
		"text <style>h1 {}</style> text\n",
		"<div><style>h1 {}</style></div>\n",
		"<!-- _class: a --> <style>h1 {}</style>\n",
		"<style>\nh1 {}\n\n# never closed\n",
	} {
		got := deckContent(t, body)
		if strings.Contains(got, "<style") || !strings.Contains(got, "&lt;style>") && !strings.Contains(got, "&lt;style&gt;") {
			t.Errorf("%q: the tag is not shown as text:\n%s", body, got)
		}
		if !strings.Contains(got, "h1 {}") {
			t.Errorf("%q: text was lost:\n%s", body, got)
		}
	}
}

// A note that documents the syntax keeps it, and a note that is no deck keeps
// the HTML it has always passed through.
func TestMarpSyntaxInCodeAndInOtherNotes(t *testing.T) {
	got := deckContent(t, "`<!-- _class: a -->`\n\n```\n<style scoped>\nh1 {}\n</style>\n```\n")
	if strings.Contains(got, "marp-label") {
		t.Errorf("code became a label:\n%s", got)
	}
	if !strings.Contains(got, "&lt;!-- _class: a --&gt;") || !strings.Contains(got, "&lt;style scoped&gt;") {
		t.Errorf("code was changed:\n%s", got)
	}

	body := "<!-- _class: a -->\n\n<style>\nh1 {}\n</style>\n"
	for _, note := range []string{body, "---\nmarp: false\n---\n" + body} {
		got := renderedContent(t, note)
		if !strings.Contains(got, body[:18]) || !strings.Contains(got, "<style>\nh1 {}\n</style>") || strings.Contains(got, "marp-label") {
			t.Errorf("a note that is no deck changed:\n%s", got)
		}
	}
}

// --render-markdown is what Marp reads, so the instructions must reach it.
func TestRenderMarkdownKeepsMarpInstructions(t *testing.T) {
	note := "---\nmarp: true\n---\n<!-- _class: a -->\n\n<style scoped>\nh1 {}\n</style>\n"
	if got, err := (htmlRenderer{}).markdown(note); err != nil || got != note {
		t.Errorf("markdown() = %q, %v", got, err)
	}
}
