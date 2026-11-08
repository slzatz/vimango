package terminal

import (
	"bufio"
	"strings"
	"testing"
)

// keys reads every key from input the way the app does: ErrNoInput (a
// sequence dropped whole) is skipped, anything else stops.
func keys(t *testing.T, input string) []Key {
	t.Helper()
	r := bufio.NewReader(strings.NewReader(input))
	var out []Key
	for {
		k, err := readKey(r)
		if err == ErrNoInput {
			continue
		}
		if err != nil {
			return out
		}
		out = append(out, k)
	}
}

func regular(rs ...rune) []Key {
	out := make([]Key, len(rs))
	for i, r := range rs {
		out[i] = Key{r, KeyNoSpl}
	}
	return out
}

func assertKeys(t *testing.T, input string, want []Key) {
	t.Helper()
	got := keys(t, input)
	if len(got) != len(want) {
		t.Fatalf("%q -> %v, want %v", input, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%q -> %v, want %v", input, got, want)
		}
	}
}

// Ghostty sends Ctrl-I as ESC [105;5u. It must arrive as the Tab byte vim
// knows Ctrl-I as -- and nothing of it may leak: the old reader returned ESC
// and left ";5u" to be typed, which vim ran as "undo five times".
func TestCSIUCtrlLetterIsItsControlByte(t *testing.T) {
	assertKeys(t, "\x1b[105;5u", regular('\t'))
	assertKeys(t, "\x1b[105;5ux", regular('\t', 'x'))
	assertKeys(t, "\x1b[98;5u", regular(2))      // Ctrl-B
	assertKeys(t, "\x1b[101;5u", regular(5))     // Ctrl-E
	assertKeys(t, "\x1b[109;5u", regular('\r'))  // Ctrl-M
	assertKeys(t, "\x1b[91;5u", regular(27))     // Ctrl-[ is Esc
	assertKeys(t, "\x1b[73;6u", regular('\t'))   // Ctrl-Shift-I
	assertKeys(t, "\x1b[105;69u", regular('\t')) // Ctrl-I with caps lock on
}

func TestCSIUPlainAndShiftedKeys(t *testing.T) {
	assertKeys(t, "\x1b[97u", regular('a'))
	assertKeys(t, "\x1b[97;1u", regular('a'))
	assertKeys(t, "\x1b[97;2u", regular('A'))
	assertKeys(t, "\x1b[27u", regular(27))
	assertKeys(t, "\x1b[105:73;5u", regular('\t')) // alternate-key sub-field ignored
}

// Combinations with no meaning here are dropped whole, never leaked.
func TestCSIUUnusableKeysAreDropped(t *testing.T) {
	assertKeys(t, "\x1b[105;3uz", regular('z')) // Alt-I
	assertKeys(t, "\x1b[105;9uz", regular('z')) // Super-I
	assertKeys(t, "\x1b[57399uz", regular('z')) // a private-use functional key
	assertKeys(t, "\x1b[32;5uz", regular('z'))  // Ctrl-Space (would be NUL)
	assertKeys(t, "\x1b[;5uz", regular('z'))    // malformed: no key code
}

// Sequences the reader does not know are consumed through their final byte
// and dropped.
func TestUnknownSequencesDoNotLeak(t *testing.T) {
	assertKeys(t, "\x1b[1;5Az", regular('z')) // Ctrl-Up
	assertKeys(t, "\x1b[200~z", regular('z')) // bracketed-paste start
	assertKeys(t, "\x1b[24~z", regular('z'))  // F12
	assertKeys(t, "\x1bOxz", regular('z'))    // unknown SS3
}

// Every sequence in specialKeys still decodes to its key.
func TestSpecialKeysStillDecode(t *testing.T) {
	for stack, key := range specialKeys {
		seq := strings.TrimRight(string(stack[:]), "\x00")
		assertKeys(t, "\x1b"+seq+"z", []Key{{0, key}, {'z', KeyNoSpl}})
	}
}

// ESC followed by anything but [ or O is a bare Esc with independent input
// behind it -- a host injecting "\x1b:checkstale\r" in one write.
func TestBareEscapeBeforeOtherInput(t *testing.T) {
	assertKeys(t, "\x1b", regular(27))
	assertKeys(t, "\x1b:q\r", regular(27, ':', 'q', '\r'))
}
