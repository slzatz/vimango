package editor

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// What stock vim does with the keys that scroll, and with the motions that
// make the view follow the cursor: the answers of vim 9.1.1752, taken with
// scripts/ask-vim.py, which types the keys into a vim whose window is the
// test editor's pane -- 30 lines of 50 columns, 'wrap', 'linebreak',
// 'scrolloff' 0 -- and asks it for line("w0"), line(".") and col(".").
//
// The view's rules (view.go) are vim's, and this is what holds them to it.
// The rows are sets of a known shape (vimRows), so that a row's screen lines
// are the same in vim as here. To add a case, ask vim:
//
//	scripts/ask-vim.py uneven '20Gzz'
//
// Left out, because the editor does not do what vim does: half a page that
// ends two or more screen lines into a wrapped row
// (TestHalfPageKeepsTheCursorsPlace), a count given to CTRL-U on the first
// line or CTRL-D on the last, which libvim refuses before the editor hears of
// it, and two things libvim gets wrong with the wanted column that have
// nothing to do with scrolling (docs/VIM_COMMANDS.md, part 3).
var vimAnswers = []struct {
	rows string // vimRows
	keys string // as vim writes them: <C-d>, <PageDown>, <CR>
	want string // the first line in the window, the cursor's line, its column
}{
	{"numbered", "<C-e>", "2 2 1"},
	{"numbered", "4l<C-e>", "2 2 5"},
	{"numbered", "<C-e>3<C-e><C-y>", "4 5 1"},
	{"numbered", "<C-e>3<C-e><C-y>L<C-y>", "3 32 1"},
	{"numbered", "<C-e>3<C-e><C-y>L<C-y>9<C-y>", "1 30 1"},
	{"numbered", "200<C-e>", "100 100 1"},
	{"numbered", "200<C-e><C-e>", "100 100 1"},
	{"numbered", "<C-d>", "16 16 1"},
	{"numbered", "<C-d>jj<C-d>", "31 33 1"},
	{"numbered", "<C-d>jj<C-d><C-u>", "16 18 1"},
	{"numbered", "<C-d>jj<C-d><C-u>5<C-d>", "21 23 1"},
	{"numbered", "<C-d>jj<C-d><C-u>5<C-d><C-d>", "26 28 1"},
	{"numbered", "<C-d>jj<C-d><C-u>5<C-d><C-d><C-u>", "21 23 1"},
	{"numbered", "10G<C-u>", "1 1 1"},
	{"numbered", "G", "71 100 1"},
	{"numbered", "G<C-u>", "56 85 1"},
	{"numbered", "G<C-u><C-d>", "71 100 1"},
	{"numbered", "G<C-u><C-d>5k<C-d>", "71 100 1"},
	{"numbered", "<PageDown>", "29 29 1"},
	{"numbered", "<PageDown><PageDown>", "57 57 1"},
	{"numbered", "<PageDown><PageDown><PageUp>", "29 58 1"},
	{"numbered", "<PageDown><PageDown><PageUp><PageUp>", "1 30 1"},
	{"numbered", "<PageDown><PageDown><PageUp><PageUp><PageUp>", "1 30 1"},
	{"numbered", "2<PageDown>", "57 57 1"},
	{"numbered", "G<PageDown>", "100 100 1"},
	{"numbered", "G<PageDown><PageDown>", "100 100 1"},
	{"numbered", "50G4lzt", "50 50 5"},
	{"numbered", "50G4lztzz", "36 50 5"},
	{"numbered", "50G4lztzzzb", "21 50 5"},
	{"numbered", "5zt", "5 5 1"},
	{"numbered", "60z<CR>", "60 60 1"},
	{"numbered", "60z.", "46 60 1"},
	{"numbered", "60z-", "31 60 1"},
	{"numbered", "98Gzz", "84 98 1"},
	{"numbered", "Gzt", "100 100 1"},
	{"numbered", "3Gzz", "1 3 1"},
	{"numbered", "3Gzb", "1 3 1"},
	{"numbered", "29j", "1 30 1"},
	{"numbered", "30j", "2 31 1"},
	{"numbered", "30jL10j", "12 41 1"},
	{"numbered", "G5k30k", "65 65 1"},
	{"numbered", "G5k30k10k", "55 55 1"},
	{"numbered", "G5k30k10k25G", "11 25 1"},
	{"numbered", "60G", "45 60 1"},
	{"numbered", "40G", "11 40 1"},
	{"numbered", "45G", "16 45 1"},
	{"numbered", "46G", "31 46 1"},
	{"numbered", "/row 77<CR>", "63 77 1"},
	{"numbered", "50G<C-e>", "36 50 1"},
	{"numbered", "50G<C-d>", "50 65 1"},
	{"numbered", "50G<C-u>", "20 35 1"},
	{"numbered", "50G<PageUp>", "7 36 1"},
	{"numbered", "50G<PageDown>", "63 63 1"},
	{"tall3", "<C-e>", "2 2 1"},
	{"tall3", "<C-d>", "6 6 1"},
	{"tall3", "<C-d><C-u>", "1 1 1"},
	{"tall3", "<PageDown>", "10 10 1"},
	{"tall3", "<PageDown><PageUp>", "1 10 1"},
	{"tall3", "20Gzz", "16 20 1"},
	{"tall3", "20Gzb", "11 20 1"},
	{"tall3", "20Gzt", "20 20 1"},
	{"tall3", "G", "31 40 1"},
	{"tall3", "15G", "6 15 1"},
	{"tall3", "16G", "11 16 1"},
	{"tall3", "17G", "12 17 1"},
	{"tall3", "18G", "13 18 1"},
	{"tall3", "25G", "20 25 1"},
	{"tall3", "G<C-u>", "26 35 1"},
	{"tall3", "G<PageUp>", "22 31 1"},
	{"tall3", "jj<C-d>", "6 8 1"},
	{"tall3", "60lgj<C-d>", "6 6 1"},
	{"tall3", "G<C-y>", "30 39 1"},
	{"tall3", "GH<C-e>", "32 32 1"},
	{"tall3", "5<C-d>", "2 2 1"},
	{"tall3", "5<C-d><C-d>", "3 3 1"},
	{"tall3", "5j60l<C-d>", "6 11 1"},
	{"tall3", "5j60l<C-d><C-u>", "1 6 1"},
	{"tall2", "<C-d>", "8 8 1"},
	{"tall2", "<C-d><C-d>", "15 15 1"},
	{"tall2", "<PageDown>", "15 15 1"},
	{"tall2", "<PageDown><PageUp>", "1 15 1"},
	{"tall2", "20Gzz", "13 20 1"},
	{"tall2", "G<C-u>", "19 33 1"},
	{"tall2", "G<C-u><C-d>", "26 40 1"},
	{"tall2", "3<C-d>", "2 2 1"},
	{"tall2", "3<C-d><C-d>", "3 3 1"},
	{"mixed", "21GztL", "21 46 1"},
	{"mixed", "21GztLj", "22 47 1"},
	{"mixed", "G24G60lk", "23 23 18"},
	{"mixed", "G24G60lkk", "22 22 61"},
	{"mixed", "<PageDown>", "25 25 1"},
	{"mixed", "<PageDown><PageUp>", "1 26 1"},
	{"mixed", "<C-d><C-d>", "27 27 1"},
	{"mixed", "30Gzz", "20 30 1"},
	{"mixed", "30Gzb", "5 30 1"},
	{"mixed", "20G<C-d>", "16 31 1"},
	{"mixed", "23G<C-u>", "1 10 1"},
	{"mixed", "22G60l<C-d>", "16 34 1"},
	{"mixed", "G<C-u><C-u>", "5 30 1"},
	{"uneven", "<C-d>", "7 7 1"},
	{"uneven", "<C-d><C-d>", "12 12 1"},
	{"uneven", "<C-d><C-d><C-u>", "6 6 1"},
	{"uneven", "<PageDown>", "12 12 1"},
	{"uneven", "<PageDown><PageDown>", "23 23 1"},
	{"uneven", "<PageDown><PageDown><PageUp>", "12 23 1"},
	{"uneven", "30Gzz", "24 30 1"},
	{"uneven", "30Gzb", "19 30 1"},
	{"uneven", "30Gzt", "30 30 1"},
	{"uneven", "G", "49 60 1"},
	{"uneven", "G<C-u>", "44 54 1"},
	{"uneven", "G<PageUp>", "39 50 1"},
	{"uneven", "G<PageUp><PageUp>", "28 39 1"},
	{"uneven", "<C-e><C-e><C-e>", "4 4 1"},
	{"uneven", "20G", "15 20 1"},
	{"uneven", "10G", "1 10 1"},
	{"uneven", "40G<C-y>", "34 40 1"},
	{"uneven", "3<C-d>", "3 3 1"},
	{"uneven", "7<C-d><C-d><C-u>", "4 4 1"},
	{"uneven", "8G12<C-d>", "6 12 1"},
	{"uneven", "8G13<C-d>", "7 12 1"},
	{"uneven", "8G14<C-d>", "7 12 1"},
	{"uneven", "8G16<C-d>", "8 14 1"},
	{"uneven", "8G17<C-d>", "8 14 1"},
	{"uneven", "11G17<C-d>", "8 16 1"},
	{"uneven", "4Gzt1<C-d>", "5 5 1"},
	{"uneven", "4Gzt2<C-d>", "5 5 1"},
	{"uneven", "4Gzt3<C-d>", "5 5 1"},
	{"uneven", "4Gzt4<C-d>", "5 5 1"},
	{"uneven", "4Gzt5<C-d>", "6 6 1"},
	{"uneven", "4Gzt4j1<C-d>", "5 9 1"},
	{"uneven", "4Gzt4j2<C-d>", "5 9 1"},
	{"uneven", "4Gzt4j3<C-d>", "5 9 1"},
	{"uneven", "4Gzt5j2<C-d>", "5 11 1"},
	{"uneven", "4Gzt6j3<C-d>", "5 11 1"},
	{"uneven", "20Gzt1<C-u>", "19 19 1"},
	{"uneven", "20Gzt2<C-u>", "19 19 1"},
	{"uneven", "20Gzt3<C-u>", "19 19 1"},
	{"uneven", "20Gzt4<C-u>", "19 19 1"},
	{"uneven", "20Gzt5<C-u>", "18 18 1"},
	{"uneven", "20Gzt6<C-u>", "17 17 1"},
	{"uneven", "20Gzt7<C-u>", "17 17 1"},
	{"uneven", "20Gzt8<C-u>", "17 17 1"},
	{"uneven", "20Gzt9<C-u>", "17 17 1"},
	{"uneven", "20Gzt10<C-u>", "16 16 1"},
	{"uneven", "20Gzt3j2<C-u>", "19 21 1"},
	{"uneven", "20Gzt3j3<C-u>", "19 21 1"},
	{"uneven", "20Gzt3j4<C-u>", "19 21 1"},
	{"uneven", "20Gzt3j5<C-u>", "18 20 1"},
	{"uneven", "20Gzt3j6<C-u>", "17 20 1"},
	{"uneven", "20Gzt3j7<C-u>", "17 20 1"},
	{"uneven", "20Gzt3j8<C-u>", "17 20 1"},
	{"uneven", "20Gzt3j9<C-u>", "17 20 1"},
	{"uneven", "20Gzt3j10<C-u>", "16 19 1"},
	{"uneven", "20Gzt5j6<C-u>", "17 23 1"},
	{"uneven", "20Gzt5j7<C-u>", "17 23 1"},
	{"uneven", "20Gzt5j8<C-u>", "17 23 1"},
	{"uneven", "20Gzt5j9<C-u>", "17 23 1"},
	{"uneven", "20Gzt6j9<C-u>", "17 23 1"},
	{"uneven", "20Gzt7j9<C-u>", "17 24 1"},
	{"mixed", "l0<PageDown>ztz-$zt31G$<PageUp>M", "1 15 1"},
	{"numbered", "l089G6G<PageUp>84GLj12j", "71 100 1"},
	{"numbered", "l03gjk<PageDown>", "29 29 1"},
	{"tall2", "l00gk2<PageDown>12k30G7<C-u>", "14 27 1"},
	{"uneven", "l03gkgk<C-y>12jGG<C-y>12jj", "49 60 1"},
	{"tall3", "l0<PageUp>5<C-d>12j37G", "31 37 1"},
	{"tall3", "l0<C-d>3<C-e>jgg3gjj<PageDown>$0", "10 10 1"},
	{"mixed", "l0H27Ggg7<C-u>jz-k", "1 1 1"},
	{"tall3", "l026GGM<PageUp>", "22 31 1"},
	{"tall2", "l0<C-d><C-u>12j", "1 13 1"},
	{"numbered", "l075Gj<C-d>48G", "34 48 1"},
	{"mixed", "l0<PageUp>z.<C-u>12G", "1 12 1"},
	{"tall3", "l015G011G0<PageUp>", "1 10 1"},
	{"uneven", "l05kGzt", "60 60 1"},
	{"tall2", "l03gk27G<C-y>$", "19 27 75"},
	{"uneven", "l0gkz<CR>42G", "36 42 1"},
	{"uneven", "l0gjgjk", "1 1 25"},
	{"numbered", "l0<PageUp>5k$<C-d>z.", "2 16 1"},
	{"tall3", "l0H<C-d>z-", "1 6 1"},
	{"tall3", "l0k<C-u>5j2G<PageUp>3<C-e>H", "4 4 1"},
	{"numbered", "l0zb5jH", "1 1 1"},
	{"mixed", "l012k<PageUp>12j7<C-u>k", "1 5 1"},
	{"mixed", "l0j<C-u>z-1G12k6G", "1 6 1"},
	{"numbered", "l0<C-d>z-<PageUp>zbzz", "2 16 1"},
	{"mixed", "l03<C-e>12kkgg32G2<PageDown>j", "59 60 1"},
	{"tall2", "l010Gzz<C-d>2<PageDown>3<C-y>", "35 38 1"},
	{"mixed", "l00<PageDown>0gjgk3<C-y>H2<PageDown>", "60 60 1"},
	{"tall2", "l012jL<PageUp><C-e>7<C-u>25G2<PageDown>21G3<C-y>", "11 21 1"},
	{"numbered", "l0<C-e>5j<C-e><C-u>40GkH<PageDown>k", "38 38 1"},
	{"mixed", "l03gjM5kggGzb42G<C-d>0", "31 57 1"},
	{"tall3", "l03gkjzb$<C-e>zbztLgk", "2 10 101"},
	{"numbered", "l0<C-u><PageUp>0jz<CR>73G", "59 73 1"},
	{"uneven", "l0<C-d>3<C-y>gj7Ggg3gj<C-u>3<C-e>3<C-y>", "1 4 1"},
	{"tall2", "l0<C-u>12k<PageUp>z<CR><C-e>k12j5j", "4 18 1"},
	{"tall3", "l0HMzz2<PageDown>12k<PageUp>z<CR>", "10 10 1"},
	{"tall2", "l036Gzzz<CR>3<C-e>25G3<C-e>zb<C-e>2<PageDown>", "40 40 1"},
	{"tall3", "l0ggj<C-u>zbzzzb", "1 1 1"},
	{"mixed", "l02<PageDown>Lzb7<C-u>5<C-d>3gj2<PageDown>", "60 60 1"},
	{"tall3", "l0<PageUp>3<C-y>027G3gk", "22 26 1"},
	{"tall3", "l05jMj3<C-y>3gj2<PageDown>2<PageDown>gg", "1 1 1"},
	{"mixed", "l03gk0zz<PageDown><C-u>7<C-u>zz", "1 7 1"},
	{"uneven", "l0gj56G35Gzz", "29 35 1"},
	{"tall3", "l035GH15G$21G00HH", "12 12 1"},
	{"uneven", "l0<C-u><PageDown>7<C-u>", "9 9 1"},
	{"uneven", "l0H5<C-d>k0<C-u><C-d>zt<C-d>0", "4 4 1"},
	{"mixed", "l0G5jz<CR>", "60 60 1"},
	{"uneven", "l0z.M$gjLG5k", "49 55 1"},
	{"tall3", "l012j3<C-y><PageDown>zt<PageDown>14Gj0", "14 15 1"},
	{"tall3", "l0<C-d>5<C-d>13GGj12k", "28 28 1"},
	{"mixed", "l046Gz.0z-3gkggL5<C-d><C-d>", "11 36 1"},
	{"uneven", "l0<PageDown>$gj$<C-d><PageUp>j<C-u>", "2 12 1"},
	{"uneven", "l0gk3<C-e>gj5<C-d>$12jzt", "18 18 75"},
	{"mixed", "l0gg5<C-d>k8G3gj<PageUp>zt0", "26 26 1"},
	{"uneven", "l0G3gk<C-d>zb7<C-u>z.", "51 56 1"},
	{"mixed", "l0<PageUp>52Gz<CR>", "52 52 1"},
	{"uneven", "l05jL<PageUp>2<PageDown>35G5k2<PageDown><C-y>", "45 46 1"},
	{"uneven", "l03<C-e>zz<PageUp><C-e><C-y><C-y>", "1 4 1"},
	{"numbered", "l03gj5jL", "1 30 1"},
	{"mixed", "l0<C-u>3<C-y>5<C-d>z-3<C-y>gk<PageUp>3gjzz", "1 8 1"},
	{"numbered", "l0<PageDown>3gjgg7G<PageDown>gj<C-u>M<C-d>", "29 43 1"},
	{"mixed", "l07<C-u>30G17G7<C-u>", "1 10 1"},
	{"tall2", "l0HH<PageUp>ztzt<C-u><C-e>z.0", "1 2 1"},
	{"uneven", "l03gj3gj33G05j", "28 38 1"},
	{"uneven", "l0<C-e><PageDown>2G", "2 2 1"},
	{"numbered", "l0zb12j12k", "1 1 1"},
	{"uneven", "l03gj<C-d><PageUp>gj", "1 12 51"},
	{"tall2", "l0Hgg29G<PageUp>3gk", "8 20 51"},
	{"mixed", "l0<C-d>10G54G<C-y>$28Gz-<PageDown>3<C-y>", "24 27 1"},
	{"numbered", "l0<PageDown>z.<PageDown>j3<C-e>28G3<C-e>", "17 28 1"},
	{"mixed", "l03<C-y>M<C-u>3<C-y>gg<PageUp>37G<C-e>", "13 37 1"},
}

// vimRows is a set of rows by name, the same as scripts/ask-vim.py makes.
func vimRows(name string) []string {
	switch name {
	case "numbered": // a hundred rows of one screen line
		return numberedRows(100)
	case "tall3": // forty of three
		return tallRows(40, 3)
	case "tall2": // forty of two
		return tallRows(40, 2)
	case "mixed": // sixty of one, but rows 22 and 24, of three
		rows := numberedRows(60)
		rows[21], rows[23] = threeScreenLines, threeScreenLines
		return rows
	case "uneven": // sixty of one, two, three and four screen lines, in turn
		rows := make([]string, 60)
		for i := range rows {
			rows[i] = strings.Repeat("word ", 10*(i%4+1)-5)
		}
		return rows
	}
	panic("no rows called " + name)
}

var vimKey = regexp.MustCompile(`<[^>]+>|.`)

var vimKeyCodes = map[string]int{
	"<C-d>": ctrlD, "<C-u>": ctrlU, "<C-e>": ctrlE, "<C-y>": ctrlY, "<CR>": '\r',
	"<PageDown>": terminal.KeyPageDown, "<PageUp>": terminal.KeyPageUp,
}

// typeVimKeys types keys written as vim writes them.
func (e *Editor) typeVimKeys(keys string) {
	for _, k := range vimKey.FindAllString(keys, -1) {
		if c, named := vimKeyCodes[k]; named {
			e.typeKeys(c)
		} else {
			e.typeString(k)
		}
	}
}

func TestViewAgreesWithVim(t *testing.T) {
	for _, c := range vimAnswers {
		e := newTestEditor(t, vimRows(c.rows)...)
		captureStdout(t, func() { e.typeVimKeys(c.keys) }) // a search writes its line
		pos := vim.GetCursorPosition()
		if got := fmt.Sprintf("%d %d %d", e.firstVisibleRow+1, pos[0], pos[1]+1); got != c.want {
			t.Errorf("%s, %s: top row, line and column %s; vim's are %s", c.rows, c.keys, got, c.want)
		}
		if e.cy < 0 || e.cy >= e.ScreenLines {
			t.Errorf("%s, %s: the cursor is on line %d of a pane of %d", c.rows, c.keys, e.cy, e.ScreenLines)
		}
	}
}

// Half a page moves the cursor as many screen lines as it moves the view, so
// the cursor stays on its line of the pane. vim 9.1 means to, and when the
// half page ends two screen lines or more into a wrapped row it moves the
// cursor one line too many (view.go); these are the cases it gets wrong,
// with what the editor does.
func TestHalfPageKeepsTheCursorsPlace(t *testing.T) {
	for _, c := range []struct {
		keys          string
		top, line, cy int
	}{
		// The cursor is on row 8, line 16 of the pane. Fifteen screen
		// lines on is two lines into row 7, of three: the view goes
		// thirteen, to row 7, and so does the cursor, onto row 12, whose
		// first non-blank is three lines up. (vim: row 13.)
		{"8G15<C-d>", 7, 12, 13},
		{"8G18<C-d>", 8, 14, 15},     // vim: row 15
		{"11G19<C-d>", 8, 16, 20},    // vim: row 17
		{"20Gzt3j1<C-u>", 19, 21, 7}, // a row of three comes on whole; vim: row 22
	} {
		e := newTestEditor(t, vimRows("uneven")...)
		e.typeVimKeys(c.keys)
		expectView(t, e, c.keys, c.top, c.line)
		if e.cy != c.cy {
			t.Errorf("after %s: the cursor is on line %d of the pane, want %d", c.keys, e.cy, c.cy)
		}
	}
}
