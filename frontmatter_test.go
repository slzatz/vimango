package main

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// The claim frontmatter.go makes is that the preview and the editor agree
// on what frontmatter is, because splitFrontmatter is a transcription of
// the one chroma's markdown lexer uses. That claim is only worth anything
// if it is checked against chroma rather than against the transcription:
// the two live in different repos and only one of them is ours to keep in
// step. So the test asks chroma the question the editor asks it -- did you
// lex this as YAML -- and requires the same answer.
func chromaSawFrontmatter(t *testing.T, src string) bool {
	t.Helper()
	it, err := lexers.Get("markdown").Tokenise(nil, src)
	if err != nil {
		t.Fatalf("chroma tokenise: %v", err)
	}
	// NameTag is the YAML lexer's token for the key half of a pair, and
	// the markdown rules have no rule that emits it. Its presence is
	// therefore exactly "chroma handed this to the YAML lexer" -- which is
	// also why the keys come up colored in the editor.
	for _, tok := range it.Tokens() {
		if tok.Type == chroma.NameTag {
			return true
		}
	}
	return false
}

func TestSplitFrontmatterMatchesChroma(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{"memory doc", "---\nname: a\nmetadata:\n  type: user\n---\n\nBody.\n", true},
		{"crlf", "---\r\nname: a\r\n---\r\n\r\nBody.\r\n", true},
		// The note in the wild this protects: id 1206 opens with a
		// four-dash rule and follows it with name:/url: lines.
		{"four dashes", "---- Task Type: URL ----\nname: Appigo\n---- End ----\n", false},
		{"unterminated", "---\nname: a\n\nBody with no closing fence.\n", false},
		{"thematic break", "---\n\nJust a note that opens with a rule.\n", false},
		{"not at start", "Intro.\n\n---\nname: a\n---\n", false},
		{"indented fence", " ---\nname: a\n---\n", false},
		{"closing fence indented", "---\nname: a\n ---\nbody\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, ok := splitFrontmatter(tc.src)
			if ok != tc.want {
				t.Errorf("splitFrontmatter ok = %v, want %v", ok, tc.want)
			}
			if got := chromaSawFrontmatter(t, tc.src); got != tc.want {
				t.Errorf("chroma disagrees: saw frontmatter = %v, want %v", got, tc.want)
			}
		})
	}
}

// Checked apart from the chroma table above rather than in it: the probe
// there is "did chroma emit a NameTag", and a block with no keys has
// nothing to tag either way, so the comparison would be vacuous. Splitting
// it is still the right answer -- parseFrontmatter is what declines an
// empty block, and it declines it as a fall-through.
func TestSplitFrontmatterEmptyBlock(t *testing.T) {
	if _, _, ok := splitFrontmatter("---\n---\n\nBody.\n"); !ok {
		t.Error("expected an empty fenced block to split")
	}
	if got := parseFrontmatter("---\n---\n"); got != nil {
		t.Errorf("expected nil for an empty block, got %+v", got)
	}
}

func TestSplitFrontmatterBody(t *testing.T) {
	fm, body, ok := splitFrontmatter("---\nname: a\n---\n\nBody.\n")
	if !ok {
		t.Fatal("expected frontmatter")
	}
	if fm != "---\nname: a\n---\n" {
		t.Errorf("frontmatter = %q", fm)
	}
	if body != "\nBody.\n" {
		t.Errorf("body = %q", body)
	}
}

func TestParseFrontmatter(t *testing.T) {
	// The real shape, trailing space on "metadata: " included -- that is
	// what the memory-doc writer emits, and untrimmed it would make a
	// parent key look like a key with an empty scalar.
	src := "---\n" +
		"name: app-bundle-rebuild\n" +
		"description: \"swift build alone doesn't update the app\"\n" +
		"metadata: \n" +
		"  node_type: memory\n" +
		"  type: project\n" +
		"---\n"
	got := parseFrontmatter(src)
	want := []fmEntry{
		{key: "name", value: "app-bundle-rebuild", depth: 0},
		{key: "description", value: "swift build alone doesn't update the app", depth: 0},
		{key: "metadata", value: "", depth: 0},
		{key: "node_type", value: "memory", depth: 1},
		{key: "type", value: "project", depth: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseFrontmatterLists(t *testing.T) {
	got := parseFrontmatter("---\ntags:\n  - one\n  - two\n---\n")
	if len(got) != 1 || got[0].key != "tags" || got[0].value != "one, two" {
		t.Errorf("got %+v, want one tags entry valued \"one, two\"", got)
	}
}

func TestParseFrontmatterQuoting(t *testing.T) {
	cases := map[string]string{
		`a: "plain"`:            "plain",
		`a: "an \"inner\" bit"`: `an "inner" bit`,
		`a: 'single'`:           "single",
		`a: 'it''s'`:            "it's",
		`a: bare "quotes" mid`:  `bare "quotes" mid`,
		`a: ""`:                 "",
	}
	for line, want := range cases {
		got := parseFrontmatter("---\n" + line + "\n---\n")
		if len(got) != 1 {
			t.Errorf("%s: got %d entries", line, len(got))
			continue
		}
		if got[0].value != want {
			t.Errorf("%s: value = %q, want %q", line, got[0].value, want)
		}
	}
}

// The bail-out is the load-bearing case: anything this parser has no
// display for must render the note the way it renders today rather than
// silently drop the half it could not read.
func TestParseFrontmatterBailsOut(t *testing.T) {
	cases := map[string]string{
		"block scalar": "---\ndesc: |\n  line one\n  line two\n---\n",
		"flow json":    "---\na: 1\n{\"b\": 2}\n---\n",
		"bare prose":   "---\njust a sentence with no colon\n---\n",
		"leading list": "---\n- orphan item\n---\n",
		"empty block":  "---\n---\n",
		"only blanks":  "---\n\n\n---\n",
		"comment only": "---\n# just a comment\n---\n",
		"anchor":       "---\na: &anchor\nb: c\n---\n",
		"alias":        "---\na: x\nb: *anchor\n---\n",
		"tag":          "---\na: !!str 5\n---\n",
		"flow seq":     "---\ntags: [one, two]\n---\n",
		"flow map":     "---\na: {b: c}\n---\n",
	}
	for name, src := range cases {
		if got := parseFrontmatter(src); got != nil {
			t.Errorf("%s: expected nil (fall through), got %+v", name, got)
		}
	}
}

func TestFrontmatterHTMLEscapes(t *testing.T) {
	got := frontmatterHTML([]fmEntry{
		{key: "a<b", value: "x & <script>", depth: 0},
		{key: "n", value: "v", depth: 1},
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

// End to end, through the function the three render paths actually call.
func TestRenderNoteAsHTMLFrontmatter(t *testing.T) {
	out, err := RenderNoteAsHTML("t", "---\nname: a\nmetadata:\n  type: user\n---\n\nBody text.\n", false, false)
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
