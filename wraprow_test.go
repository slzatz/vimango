package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// the diagram from the note that motivated the display-width wrap: every glyph
// is three bytes, so byte-based wrapping broke these lines roughly three times
// too often and sliced some of them mid-rune.
var boxDiagram = []string{
	"┌─────────────┐     ┌──────────────────┐     ┌─────────────────────┐",
	"│  Telegram   │────▶│  Telegram Bridge │────▶│  claude -p          │",
	"│  (phone)    │◀────│  (Python bot)    │◀────│  (Claude Code CLI)  │",
	"└─────────────┘     └──────────────────┘     └────────┬────────────┘",
	"                              ┌────────────────────────┼────────────────────────┐",
	"                                                 │ SKILL.md×6│",
}

func segmentsText(row string, segs []wrapSegment) []string {
	out := make([]string, len(segs))
	for i, s := range segs {
		out[i] = row[s.start:s.end]
	}
	return out
}

func TestWrapRowIsLossless(t *testing.T) {
	rows := append([]string{
		"",
		"short",
		"a few plain ascii words that will need to wrap at some point",
		strings.Repeat("x", 200),
		"trailing spaces      ",
		"    leading spaces and then a long run of text to force a wrap here",
	}, boxDiagram...)

	for _, row := range rows {
		for width := 1; width <= 100; width++ {
			segs := wrapRow(row, width)
			if len(segs) == 0 {
				t.Fatalf("width %d: no segments for %q", width, row)
			}
			if got := strings.Join(segmentsText(row, segs), ""); got != row {
				t.Errorf("width %d: rejoined %q, want %q", width, got, row)
			}
			prevEnd := 0
			for i, s := range segs {
				if s.start != prevEnd {
					t.Fatalf("width %d: segment %d starts at %d, want %d", width, i, s.start, prevEnd)
				}
				if !utf8.ValidString(row[s.start:s.end]) {
					t.Errorf("width %d: segment %d split a rune: %q", width, i, row[s.start:s.end])
				}
				prevEnd = s.end
			}
			if prevEnd != len(row) {
				t.Errorf("width %d: segments end at %d, want %d", width, prevEnd, len(row))
			}
		}
	}
}

func TestWrapRowRespectsDisplayWidth(t *testing.T) {
	for _, row := range boxDiagram {
		for width := 20; width <= 100; width++ {
			for i, seg := range wrapRow(row, width) {
				// a trailing space may sit in the last column; measure without it
				w := runewidth.StringWidth(strings.TrimRight(row[seg.start:seg.end], " "))
				if w > width {
					t.Errorf("width %d: segment %d is %d columns wide: %q", width, i, w, row[seg.start:seg.end])
				}
			}
		}
	}
}

func TestWrapRowCountsColumnsNotBytes(t *testing.T) {
	// 68 display columns, 184 bytes: it must not wrap at a width of 68
	row := boxDiagram[0]
	if got := runewidth.StringWidth(row); got != 68 {
		t.Fatalf("fixture is %d columns wide, expected 68", got)
	}
	if segs := wrapRow(row, 68); len(segs) != 1 {
		t.Errorf("wrapped a 68-column row at width 68 into %d lines: %q", len(segs), segmentsText(row, segs))
	}
	if segs := wrapRow(row, 67); len(segs) != 2 {
		t.Errorf("wrapped a 68-column row at width 67 into %d lines, want 2", len(segs))
	}
}

func TestWrapRowBreaksAtSpaces(t *testing.T) {
	row := "the quick brown fox jumps over the lazy dog"
	got := segmentsText(row, wrapRow(row, 10))
	want := []string{"the quick ", "brown fox ", "jumps ", "over the ", "lazy dog"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWrapRowNarrowerThanARune(t *testing.T) {
	// width 1 with a 2-column rune must still make progress
	row := "世界"
	segs := wrapRow(row, 1)
	if len(segs) != 2 {
		t.Fatalf("got %d segments, want 2", len(segs))
	}
	if got := strings.Join(segmentsText(row, segs), ""); got != row {
		t.Errorf("rejoined %q, want %q", got, row)
	}
}
