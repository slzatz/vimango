package main

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// a highlighted span as drawVisual emits it: the cursor is parked at (row, col)
// and the selected text is written there over the reverse-video background.
type span struct {
	row, col int
	text     string
}

var cupRe = regexp.MustCompile(`\x1b\[(\d+);(\d+)H`)
var cufRe = regexp.MustCompile(`\x1b\[\d+C`)

func parseSpans(t *testing.T, out string) []span {
	t.Helper()
	var spans []span
	locs := cupRe.FindAllStringSubmatchIndex(out, -1)
	for i, loc := range locs {
		end := len(out)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		text := out[loc[1]:end]
		text = strings.ReplaceAll(text, "\x1b[48;5;237m", "")
		text = strings.ReplaceAll(text, RESET, "")
		text = cufRe.ReplaceAllString(text, "") // the \r\n + cursor-forward that ends a VISUAL_LINE row
		if !utf8.ValidString(text) {
			t.Errorf("span at %s;%s is not valid UTF-8: %q", out[loc[2]:loc[3]], out[loc[4]:loc[5]], text)
		}
		spans = append(spans, span{atoi(out[loc[2]:loc[3]]), atoi(out[loc[4]:loc[5]]), text})
	}
	return spans
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

// visualEditor builds the smallest Editor that drawVisual will read from: no
// left margin or line numbers, and a pane wide enough that nothing wraps.
func visualEditor(rows []string, vmode Mode, highlight [2][2]int) *Editor {
	e := &Editor{}
	e.ss = rows
	e.vmode = vmode
	e.highlight = highlight
	e.screencols = 100
	e.screenlines = 40
	e.left_margin = 0
	e.left_margin_offset = 0
	e.top_margin = 1
	e.lineOffset = 0
	return e
}

func drawVisualSpans(t *testing.T, rows []string, vmode Mode, highlight [2][2]int) []span {
	t.Helper()
	var sb strings.Builder
	visualEditor(rows, vmode, highlight).drawVisual(&sb)
	return parseSpans(t, sb.String())
}

func wantSpans(t *testing.T, got []span, want []span) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d spans %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("span %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// "héllo wörld" — é and ö are two bytes each, so a byte offset is not a column
const accented = "héllo wörld"

func TestDrawVisualSingleRowMultiByteEnd(t *testing.T) {
	// select "héllo": start byte 0, end byte 5 (the 'o', since é is two bytes)
	got := drawVisualSpans(t, []string{accented}, VISUAL, [2][2]int{{1, 0}, {1, 5}})
	wantSpans(t, got, []span{{1, 1, "héllo"}})
}

func TestDrawVisualEndsOnMultiByteRune(t *testing.T) {
	// select "wö": bytes 7..8, where 8 is the first byte of ö. Slicing at
	// endcol+1 would cut ö in half.
	got := drawVisualSpans(t, []string{accented}, VISUAL, [2][2]int{{1, 7}, {1, 8}})
	wantSpans(t, got, []span{{1, 7, "wö"}})
}

func TestDrawVisualStartsAfterMultiByteRune(t *testing.T) {
	// "llo" starts at byte 3 but at display column 2
	got := drawVisualSpans(t, []string{accented}, VISUAL, [2][2]int{{1, 3}, {1, 5}})
	wantSpans(t, got, []span{{1, 3, "llo"}})
}

func TestDrawVisualMultiRowLastRowIsInclusive(t *testing.T) {
	rows := []string{"aé b", "xyz", "pqrs"}
	// from row 1 byte 3 (the space after the two-byte é) through row 3 byte 1
	// ('q'), inclusive
	got := drawVisualSpans(t, rows, VISUAL, [2][2]int{{1, 3}, {3, 1}})
	wantSpans(t, got, []span{
		{1, 3, " b"}, // é is one column, so byte 3 is column 2 -> screen col 3
		{2, 1, "xyz"},
		{3, 1, "pq"},
	})
}

func TestDrawVisualCursorPastEndOfRow(t *testing.T) {
	got := drawVisualSpans(t, []string{accented}, VISUAL, [2][2]int{{1, 0}, {1, len(accented)}})
	wantSpans(t, got, []span{{1, 1, accented}})
}

func TestDrawVisualTabsExpandToFourColumns(t *testing.T) {
	rows := []string{"a\tb"}
	got := drawVisualSpans(t, rows, VISUAL, [2][2]int{{1, 0}, {1, 2}})
	wantSpans(t, got, []span{{1, 1, "a    b"}})
}

func TestDrawVisualBlockIsRectangularOverMultiByte(t *testing.T) {
	// every row must be highlighted at the same screen columns even though the
	// accented rows reach those columns at different byte offsets
	rows := []string{
		"ααααtargetαααα", // α is two bytes, one column
		"xxxxtargetxxxx",
		"éééétargetéééé",
	}
	// top-left corner is the start of "target" on row 1, bottom-right the last
	// byte of "target" on row 3 — different byte offsets, same screen columns
	left := strings.Index(rows[0], "target")
	right := strings.Index(rows[2], "target") + 5
	got := drawVisualSpans(t, rows, VISUAL_BLOCK, [2][2]int{{1, left}, {3, right}})
	wantSpans(t, got, []span{
		{1, 5, "target"},
		{2, 5, "target"},
		{3, 5, "target"},
	})
}

func TestDrawVisualBlockSkipsShortRows(t *testing.T) {
	rows := []string{"ααααtarget", "hi", "éééétarget"}
	left := strings.Index(rows[0], "target")
	right := strings.Index(rows[2], "target") + 5
	got := drawVisualSpans(t, rows, VISUAL_BLOCK, [2][2]int{{1, left}, {3, right}})
	wantSpans(t, got, []span{
		{1, 5, "target"},
		{3, 5, "target"},
	})
}

func TestDrawVisualBlockCornersInEitherOrder(t *testing.T) {
	rows := []string{"ααααtargetαααα", "xxxxtargetxxxx"}
	left := strings.Index(rows[0], "target")
	right := strings.Index(rows[1], "target") + 5
	forward := drawVisualSpans(t, rows, VISUAL_BLOCK, [2][2]int{{1, left}, {2, right}})
	// the same rectangle with the columns reported the other way round: the
	// right edge on the top row, the left edge on the bottom one
	backward := drawVisualSpans(t, rows, VISUAL_BLOCK, [2][2]int{{1, left + 5}, {2, 4}})
	wantSpans(t, forward, backward)
}

func TestDrawVisualLineCoversWholeRows(t *testing.T) {
	rows := []string{"aé b", "xyz"}
	got := drawVisualSpans(t, rows, VISUAL_LINE, [2][2]int{{1, 0}, {2, 0}})
	if len(got) != 1 {
		t.Fatalf("got %d spans %v, want 1", len(got), got)
	}
	// VISUAL_LINE writes every row in one run separated by carriage returns
	lines := strings.Split(got[0].text, "\r\n")
	if len(lines) < 2 || lines[0] != "aé b" || !strings.HasPrefix(lines[1], "xyz") {
		t.Errorf("got %q, want the two whole rows", got[0].text)
	}
}

// vim reports byte offsets that sit on rune boundaries, but nothing downstream
// should tear a rune if one ever arrives mid-sequence.
func TestDrawVisualNeverEmitsTornRunes(t *testing.T) {
	rows := []string{"αβγ target δεζ", "héllo wörld", "плайн", "ascii only"}
	modes := []Mode{VISUAL, VISUAL_BLOCK, VISUAL_LINE}
	for _, vmode := range modes {
		for r0 := 1; r0 <= len(rows); r0++ {
			for r1 := r0; r1 <= len(rows); r1++ {
				for c0 := 0; c0 <= len(rows[r0-1]); c0++ {
					for c1 := 0; c1 <= len(rows[r1-1]); c1++ {
						// parseSpans fails the test on invalid UTF-8
						drawVisualSpans(t, rows, vmode, [2][2]int{{r0, c0}, {r1, c1}})
					}
				}
			}
		}
	}
}
