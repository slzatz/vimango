package frontmatter

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Split is a transcription of the rule chroma's markdown lexer uses, and
// that is only worth anything if it is checked against chroma rather than
// against the transcription: the two live in different repos and only one
// of them is ours to keep in step. So the test asks chroma -- did you lex
// this as YAML -- and requires the same answer.
func chromaSawFrontmatter(t *testing.T, src string) bool {
	t.Helper()
	it, err := lexers.Get("markdown").Tokenise(nil, src)
	if err != nil {
		t.Fatalf("chroma tokenise: %v", err)
	}
	// NameTag is the YAML lexer's token for the key half of a pair, and
	// the markdown rules have no rule that emits it. Its presence is
	// therefore exactly "chroma handed this to the YAML lexer".
	for _, tok := range it.Tokens() {
		if tok.Type == chroma.NameTag {
			return true
		}
	}
	return false
}

func TestSplitMatchesChroma(t *testing.T) {
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
			_, _, ok := Split(tc.src)
			if ok != tc.want {
				t.Errorf("Split ok = %v, want %v", ok, tc.want)
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
// it is still the right answer -- Parse is what declines an
// empty block, and it declines it as a fall-through.
func TestSplitEmptyBlock(t *testing.T) {
	if _, _, ok := Split("---\n---\n\nBody.\n"); !ok {
		t.Error("expected an empty fenced block to split")
	}
	if got := Parse("---\n---\n"); got != nil {
		t.Errorf("expected nil for an empty block, got %+v", got)
	}
}

func TestSplitBody(t *testing.T) {
	fm, body, ok := Split("---\nname: a\n---\n\nBody.\n")
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

func TestParse(t *testing.T) {
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
	got := Parse(src)
	want := []Entry{
		{Key: "name", Value: "app-bundle-rebuild", Depth: 0},
		{Key: "description", Value: "swift build alone doesn't update the app", Depth: 0},
		{Key: "metadata", Value: "", Depth: 0},
		{Key: "node_type", Value: "memory", Depth: 1},
		{Key: "type", Value: "project", Depth: 1},
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

func TestParseLists(t *testing.T) {
	got := Parse("---\ntags:\n  - one\n  - two\n---\n")
	if len(got) != 1 || got[0].Key != "tags" || got[0].Value != "one, two" {
		t.Errorf("got %+v, want one tags entry valued \"one, two\"", got)
	}
}

func TestParseQuoting(t *testing.T) {
	cases := map[string]string{
		`a: "plain"`:            "plain",
		`a: "an \"inner\" bit"`: `an "inner" bit`,
		`a: 'single'`:           "single",
		`a: 'it''s'`:            "it's",
		`a: bare "quotes" mid`:  `bare "quotes" mid`,
		`a: ""`:                 "",
	}
	for line, want := range cases {
		got := Parse("---\n" + line + "\n---\n")
		if len(got) != 1 {
			t.Errorf("%s: got %d entries", line, len(got))
			continue
		}
		if got[0].Value != want {
			t.Errorf("%s: value = %q, want %q", line, got[0].Value, want)
		}
	}
}

// The bail-out is the load-bearing case: anything this parser has no
// display for must render the note the way it renders today rather than
// silently drop the half it could not read.
func TestParseBailsOut(t *testing.T) {
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
		if got := Parse(src); got != nil {
			t.Errorf("%s: expected nil (fall through), got %+v", name, got)
		}
	}
}

// Rows answers for the note's rows what Split and Parse answer for its text,
// which is the rows joined with "\n": the editor asks Rows, the preview the
// other two, and they must agree.
func TestRowsAgreesWithSplitAndParse(t *testing.T) {
	for _, src := range []string{
		"---\nname: a\n---\n\nBody.",
		"---\nname: a\n---\nBody.",
		"---\nname: a\n---\n",
		"---\nname: a\n---", // the closer is the last row: no line break after it
		"---\r\nname: a\r\n---\r\nBody.",
		"---\nname: a\nmetadata:\n  type: user\n---\n# Heading",
		"---\ntags:\n  - one\n---\nx",
		"---\n# just a comment\n---\n# Heading", // Parse declines
		"---\njust prose\n---\nx",
		"---\n---\nx",
		"---\nname: a\n\nno closer",
		"---- Task Type: URL ----\nname: Appigo\n---- End ----\n",
		" ---\nname: a\n---\nx",
		"---\nname: a\n ---\nx",
		"Intro.\n\n---\nname: a\n---\n",
		"---\nname: a\n---\nb: c\n---\nx", // the first closer counts
		"---",
		"",
	} {
		want := 0
		if fm, _, ok := Split(src); ok && Parse(fm) != nil {
			want = strings.Count(fm, "\n")
		}
		if got := Rows(strings.Split(src, "\n")); got != want {
			t.Errorf("%q: Rows %d, Split and Parse %d", src, got, want)
		}
	}
}
