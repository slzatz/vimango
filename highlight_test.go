package main

import (
	"strings"
	"testing"
)

// The markdown formatter has to earn SGR 9 without disturbing anything else,
// so these compare it against chroma's stock terminal16m over the same style.
const strikeOn = "\x1b[9m"

func mdStyle(t *testing.T) string {
	t.Helper()
	if _, err := selectMDStyle("gruvbox_mod.xml"); err != nil {
		t.Fatalf("gruvbox_mod.xml: %v", err)
	}
	return "gruvbox_mod.xml"
}

func highlightMD(t *testing.T, source, formatter string) string {
	t.Helper()
	style, err := selectMDStyle(mdStyle(t))
	if err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	if err := Highlight2(&buf, source, "markdown", formatter, style); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// ~~text~~ must come out struck, and the tildes are struck with it: the
// editor shows raw markdown, so the delimiters stay on screen.
func TestMarkdownFormatterStrikesDeleted(t *testing.T) {
	const source = "plain line\n\n~~struck text~~\n\nplain again\n"

	got := highlightMD(t, source, MDFormatter)
	if !strings.Contains(got, strikeOn) {
		t.Fatalf("no SGR 9 in output for %q", source)
	}

	// The struck run must cover the whole token, tildes included, and stop
	// there -- the reset chroma writes at each token end closes it.
	start := strings.Index(got, strikeOn) + len(strikeOn)
	end := strings.Index(got[start:], "\x1b[0m")
	if end < 0 {
		t.Fatal("SGR 9 never reset")
	}
	run := stripSGR(got[start : start+end])
	if run != "~~struck text~~" {
		t.Errorf("struck run = %q, want %q", run, "~~struck text~~")
	}

	// Nothing else may pick up the attribute.
	if n := strings.Count(got, strikeOn); n != 1 {
		t.Errorf("SGR 9 appears %d times, want 1", n)
	}
	if stripSGR(got) != source {
		t.Errorf("formatter altered the text itself:\n got %q\nwant %q", stripSGR(got), source)
	}
}

// Text with no strikethrough must format exactly as chroma's terminal16m
// would, so the only behavioural difference is SGR 9.
func TestMarkdownFormatterMatchesTerminal16mWithoutStrikethrough(t *testing.T) {
	const source = "# Heading\n\n**bold** and *emph* and `code`\n\n- a list item\n\n> a quote\n"

	got := highlightMD(t, source, MDFormatter)
	want := highlightMD(t, source, "terminal16m")
	if got != want {
		t.Errorf("diverged from terminal16m on strikethrough-free markdown:\n got %q\nwant %q", got, want)
	}
}

// Strikethrough must not leak across a line boundary: drawCodeRows splits the
// formatted note on "\n" and draws each line with its own cursor positioning,
// so an unclosed SGR 9 would strike the rest of the screen.
func TestMarkdownFormatterClosesStrikeOnEveryLine(t *testing.T) {
	const source = "before\n\n~~struck~~\n\nafter\n"

	for i, line := range strings.Split(highlightMD(t, source, MDFormatter), "\n") {
		opens := strings.Count(line, strikeOn)
		if opens == 0 {
			continue
		}
		if resets := strings.Count(line, "\x1b[0m"); resets < opens {
			t.Errorf("line %d opens SGR 9 %d time(s) but resets %d time(s): %q", i, opens, resets, line)
		}
	}
}

// stripSGR removes CSI ... m sequences, leaving the text the terminal shows.
func stripSGR(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				i = j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
