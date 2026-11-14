package main

import (
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// searchEditor is a test editor with 'hlsearch' on, as main sets it.
func searchEditor(t *testing.T, lines ...string) *Editor {
	t.Helper()
	e := newTestEditor(t, lines...)
	e.top_margin = 1
	vim.ExecuteCommand("set hlsearch")
	vim.ExecuteCommand("let @/ = ''")
	vim.ExecuteCommand("nohlsearch")
	t.Cleanup(func() { vim.ExecuteCommand("nohlsearch") })
	return e
}

// highlighted returns the pieces of text drawText draws in the search style,
// in order, each with the screen row it was put on.
func highlighted(t *testing.T, e *Editor) (texts []string, rows []int) {
	t.Helper()
	var ab strings.Builder
	e.scroll()
	e.drawSearchMatches(&ab)
	out := ab.String()
	if out == "" {
		return nil, nil
	}
	if !strings.HasPrefix(out, searchStyle) || !strings.HasSuffix(out, sgrReset) {
		t.Fatalf("search highlights not wrapped in the search style: %q", out)
	}
	for _, s := range parseSpans(t, strings.TrimPrefix(out, searchStyle)) {
		texts = append(texts, s.text)
		rows = append(rows, s.row)
	}
	return texts, rows
}

func TestSearchMatchesAreHighlighted(t *testing.T) {
	e := searchEditor(t, "foo foo foo", "nothing here", "a\tfoo é foofoo")

	if texts, _ := highlighted(t, e); texts != nil {
		t.Fatalf("highlights before any search: %q", texts)
	}

	e.sendKeys("/foo")
	e.editorProcessKey('\r')
	texts, rows := highlighted(t, e)
	if got, want := strings.Join(texts, "|"), "foo|foo|foo|foo|foo|foo"; got != want {
		t.Fatalf("highlighted %q, want %q", got, want)
	}
	if rows[0] != 1 || rows[2] != 1 || rows[3] != 3 || rows[5] != 3 {
		t.Errorf("highlights on screen rows %v, want the first three on row 1 and the rest on row 3", rows)
	}

	// they follow an edit
	e.sendKeys("ggx") // "oo foo foo"
	if texts, _ := highlighted(t, e); len(texts) != 5 {
		t.Errorf("after deleting the first f, %d matches are highlighted, want 5", len(texts))
	}
}

// A key that changes what should be highlighted, and not the text, has to ask
// for a redraw: the tick check sees nothing.
func TestSearchStateChangesRedraw(t *testing.T) {
	e := searchEditor(t, "foo bar", "bar foo", "bar bar")
	draw := func() {
		var ab strings.Builder
		e.drawSearchMatches(&ab)
	}
	draw()

	e.sendKeys("/foo")
	if !e.editorProcessKey('\r') {
		t.Fatal("finishing a search did not ask for a redraw")
	}
	draw()

	if e.editorProcessKey('l') {
		t.Error("a plain motion asked for a redraw")
	}

	e.sendKeys(":noh")
	if !e.editorProcessKey('\r') {
		t.Fatal(":noh did not ask for a redraw; the highlights would stay on screen")
	}
	draw()
	if texts, _ := highlighted(t, e); texts != nil {
		t.Errorf("after :noh, still highlighted: %q", texts)
	}

	if !e.editorProcessKey('n') {
		t.Fatal("n after :noh did not ask for a redraw")
	}
	if texts, _ := highlighted(t, e); len(texts) != 2 {
		t.Errorf("after n, %d matches highlighted, want 2", len(texts))
	}

	e.sendKeys("gg0w") // onto "bar"
	draw()
	if !e.editorProcessKey('*') {
		t.Fatal("* did not ask for a redraw")
	}
	if texts, _ := highlighted(t, e); len(texts) != 4 || texts[0] != "bar" {
		t.Errorf("after *, highlighted %q, want the four bars", texts)
	}

	e.exCommand("set nohlsearch")
	if texts, _ := highlighted(t, e); texts != nil {
		t.Errorf("with 'hlsearch' off, still highlighted: %q", texts)
	}
}

// A match that wraps onto the next screen line is drawn on both; one that is
// scrolled off is not drawn at all.
func TestSearchHighlightsWrapAndScroll(t *testing.T) {
	long := strings.Repeat("word ", 9) + "needle in the hay"
	e := searchEditor(t, long, "short needle")
	e.screencols = 50
	e.left_margin_offset = 0

	// the row wraps after the ninth "word ", so this match straddles the break
	e.sendKeys("/word needle")
	e.editorProcessKey('\r')
	texts, rows := highlighted(t, e)
	if strings.Join(texts, "") != "word needle" || len(texts) != 2 || rows[1] != rows[0]+1 {
		t.Errorf("a match across a wrap was drawn as %q on rows %v; want two pieces on consecutive rows", texts, rows)
	}

	e.sendKeys("/needle")
	e.editorProcessKey('\r')
	e.screenlines = 2 // only the first (wrapped) row fits
	if texts, _ := highlighted(t, e); len(texts) != 1 {
		t.Errorf("with one row visible, %d matches drawn, want 1", len(texts))
	}
}
