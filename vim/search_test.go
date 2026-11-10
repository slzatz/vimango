package vim

import (
	"slices"
	"testing"
)

func searchSetup(t *testing.T, lines ...string) *Buffer {
	t.Helper()
	InitializeVim(0)
	SendKey("<esc>")
	buf := NewBuffer(0)
	SetCurrentBuffer(buf)
	buf.SetLines(0, -1, lines)
	SetCursorPosition(1, 0)
	ExecuteCommand("set hlsearch")
	return buf
}

func search(pattern string) {
	SendInput("/")
	SendMultiInput(pattern)
	SendKey("<cr>")
}

func starts(matches []SearchMatch) (cols [][2]int) {
	for _, m := range matches {
		cols = append(cols, [2]int{m.Line, m.Col})
	}
	return cols
}

// Every match comes back, including the ones libvim used to skip: at the first
// column of the first line asked for, and starting within two columns of the
// end of the previous match.
func TestSearchMatches(t *testing.T) {
	buf := searchSetup(t, "foo foo foo", "foofoofoo", "nothing", "a\tfoo é foo")
	search("foo")

	want := [][2]int{{1, 0}, {1, 4}, {1, 8}, {2, 0}, {2, 3}, {2, 6}, {4, 2}, {4, 9}}
	got := buf.SearchMatches(1, 4)
	if !slices.Equal(starts(got), want) {
		t.Fatalf("matches start at %v, want %v", starts(got), want)
	}
	for _, m := range got {
		if m.EndLine != m.Line || m.EndCol != m.Col+3 {
			t.Errorf("match %+v does not end three bytes after it starts", m)
		}
	}
	if got := starts(buf.SearchMatches(2, 3)); !slices.Equal(got, [][2]int{{2, 0}, {2, 3}, {2, 6}}) {
		t.Errorf("lines 2..3: matches start at %v", got)
	}
	if got := buf.SearchMatches(3, 3); len(got) != 0 {
		t.Errorf("a line with no match returned %v", got)
	}
	if SearchPattern() != "foo" {
		t.Errorf("SearchPattern() = %q", SearchPattern())
	}

	search("o")
	if got := len(buf.SearchMatches(1, 2)); got != 12 {
		t.Errorf("/o found %d of the 12 o's in the first two lines", got)
	}
	search("^") // empty matches: one per line, and no endless loop
	if got := len(buf.SearchMatches(1, 4)); got != 4 {
		t.Errorf("/^ found %d matches in 4 lines", got)
	}
}

// vim keeps the on/off state: a search turns highlighting on, :noh hides it
// until the next search, 'hlsearch' switches it off altogether.
func TestSearchHighlighting(t *testing.T) {
	searchSetup(t, "foo bar", "bar foo")
	search("foo")
	if !SearchHighlighting() {
		t.Fatal("highlighting is off after a search with 'hlsearch' set")
	}
	ExecuteCommand("nohlsearch")
	if SearchHighlighting() {
		t.Error(":nohlsearch left highlighting on")
	}
	SendInput("n")
	if !SearchHighlighting() {
		t.Error("n after :nohlsearch did not turn highlighting back on")
	}
	ExecuteCommand("nohlsearch")
	SendInput("*")
	if !SearchHighlighting() {
		t.Error("* after :nohlsearch did not turn highlighting back on")
	}
	ExecuteCommand("set nohlsearch")
	SendInput("n")
	if SearchHighlighting() {
		t.Error("highlighting is on with 'hlsearch' off")
	}
	ExecuteCommand("set hlsearch")
}
