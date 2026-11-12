package terminal

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// Special keys
const (
	KeyNoSpl = iota
	KeyArrowLeft = iota + 999//ARROW_LEFT //KeyArrowLeft
	KeyArrowRight //ARROW_RIGHT //KeyArrowRight
	KeyArrowUp //ARROW_UP //KeyArrowUp
	KeyArrowDown //ARROW_DOWN //KeyArrowDown
	KeyDelete //DEL_KEY //KeyDelete
	KeyHome //HOME_KEY //KeyHome
	KeyEnd //END_KEY //KeyEnd
	KeyPageUp //PAGE_UP //KeyPageUp
	KeyPageDown //PAGE_DOWN //KeyPageDown
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyIns
	KeyPaste // text pasted from the terminal; it is in Key.Paste
)

var specialKeys = map[[4]byte]int{
  [4]byte{91, 65, 0, 0}    : KeyArrowUp,  //\x1b[A
  [4]byte{91, 66, 0, 0}    : KeyArrowDown, //\x1b[B
  [4]byte{91, 68, 0, 0}	   : KeyArrowLeft,
  [4]byte{91, 67, 0, 0}    : KeyArrowRight,
  [4]byte{91, 53, 126, 0}  : KeyPageUp, //126 = ~; 91 = [; 53 = 5 /x1b[5~
  [4]byte{91, 54, 126, 0}  : KeyPageDown, // \x1b[6~
  [4]byte{91, 72, 0, 0}    : KeyHome,
  [4]byte{91, 70, 0, 0}    : KeyEnd,
  [4]byte{91, 51, 126}     : KeyDelete,
  [4]byte{79, 80, 0, 0}    : KeyF1,
  [4]byte{79, 81, 0, 0}    : KeyF2,
  [4]byte{79, 82, 0, 0}    : KeyF3,
  [4]byte{79, 83, 0, 0}    : KeyF4,
  [4]byte{91, 49, 53, 126} : KeyF5,
  [4]byte{91, 49, 55, 126} : KeyF6,
  [4]byte{91, 49, 56, 126} : KeyF7,
  [4]byte{91, 49, 57, 126} : KeyF8,
  [4]byte{91, 50, 48, 126} : KeyF9,
  [4]byte{91, 50, 126, 0}  : KeyIns,
}
// ErrNoInput indicates that there is no input when reading from keyboard
// in raw mode. This happens when timeout is set to a low number
var ErrNoInput = fmt.Errorf("no input")

// Key represents the key entered by the user
type Key struct {
	Regular rune
	Special int
	Paste   string // with Special == KeyPaste: the pasted text
}

// Bracketed paste (xterm's mode 2004): while it is on, the terminal sends
// pasted text between two markers, ESC [200~ and ESC [201~, so it can be told
// from typed keys. The program turns it on and must turn it off again.
const (
	BracketedPasteOn  = "\x1b[?2004h"
	BracketedPasteOff = "\x1b[?2004l"
	pasteEnd          = "\x1b[201~"
)

var bufr = bufio.NewReader(os.Stdin)

// ReadKey reads a key from Stdin processing it for VT100 sequences.
// Stdin should be put in raw mode with
// VT100 processing enabled prior to using RawReadKey. If terminal read is set to
// timeout mode and no key is pressed, then ErrNoInput will be returned.
// ErrNoInput is also returned for an escape sequence that was read whole but
// means nothing here; callers skip it like any other non-key.
func ReadKey() (Key, error) {
	return readKey(bufr)
}

// maxSeqLen bounds how far readKey reads looking for a sequence's final byte,
// so a stray ESC [ followed by junk cannot swallow input indefinitely.
const maxSeqLen = 64

func readKey(r *bufio.Reader) (Key, error) {
	ch, n, err := r.ReadRune()
	if err != nil {
		return Key{}, err
	}

	// this code handles situation where a timeout has been set
	// but no key was pressed
	if n == 0 {
		return Key{}, ErrNoInput
	}

	//ascii escape is decimal 27
	if ch != 27 {
		return Key{Regular: ch, Special: KeyNoSpl}, nil
	}

	// nothing has been buffered, probably plain escape
	if r.Buffered() == 0 {
		return Key{Regular: 27, Special: KeyNoSpl}, nil
	}

	// Every escape sequence we recognize is introduced by '[' (CSI) or 'O'
	// (SS3). Anything else buffered behind ESC is independent input that
	// happened to arrive in the same read -- e.g. a host app injecting
	// "\x1b:checkstale\r" through the pty in one write. Return the bare ESC
	// and leave those bytes for later reads.
	next, err := r.Peek(1)
	if err != nil || (next[0] != '[' && next[0] != 'O') {
		return Key{Regular: 27, Special: KeyNoSpl}, nil
	}

	seq, err := readSequence(r)
	if err != nil {
		return Key{}, err
	}

	if string(seq) == "[200~" {
		return readPaste(r)
	}
	if seq[len(seq)-1] == 'u' && seq[0] == '[' {
		return csiUKey(seq[1 : len(seq)-1])
	}
	if len(seq) <= 4 {
		var stack [4]byte
		copy(stack[:], seq)
		if key, found := specialKeys[stack]; found {
			return Key{Regular: 0, Special: key}, nil
		}
	}
	// A sequence we don't know, consumed whole and dropped. The reader used
	// to return ESC and leave the unread tail to be typed as keys: Ghostty's
	// Ctrl-I, ESC [105;5u, reached vim as <esc> ; 5 u -- "undo five times".
	return Key{}, ErrNoInput
}

// readPaste reads what follows a paste's start marker up to its end marker,
// and returns the text between as one key. Nothing in it is a key: an escape
// sequence in pasted text is text. Line ends come as the terminal sends them,
// usually a carriage return each.
//
// A terminal that has begun a paste ends it, so this waits for the end marker
// however long the paste is. An empty paste is nothing.
func readPaste(r *bufio.Reader) (Key, error) {
	var text []byte
	for !bytes.HasSuffix(text, []byte(pasteEnd)) {
		b, err := r.ReadByte()
		if err != nil {
			return Key{}, err
		}
		text = append(text, b)
	}
	text = text[:len(text)-len(pasteEnd)]
	if len(text) == 0 {
		return Key{}, ErrNoInput
	}
	return Key{Special: KeyPaste, Paste: string(text)}, nil
}

// readSequence reads the bytes after ESC through the sequence's final byte.
// SS3 (ESC O x) is always one byte after the O. CSI (ESC [ ...) is parameter
// bytes 0x30-0x3F, intermediate bytes 0x20-0x2F, then one final byte
// 0x40-0x7E (ECMA-48 5.4).
func readSequence(r *bufio.Reader) ([]byte, error) {
	intro, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	seq := []byte{intro}
	if intro == 'O' {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		return append(seq, b), nil
	}
	for len(seq) < maxSeqLen {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		seq = append(seq, b)
		if b >= 0x40 && b <= 0x7e {
			return seq, nil
		}
	}
	return seq, nil // no final byte in reach: dropped by the caller
}

// csiUKey decodes the kitty keyboard protocol's ESC [ code ; mods u, which
// Ghostty (and kitty) send for keys the legacy encoding can't tell apart --
// Ctrl-I from Tab, Ctrl-[ from Esc. params is the text between '[' and 'u'.
// Ctrl plus a letter or one of @[\]^_ becomes its control byte, the key it
// stands for everywhere else; a plain or shifted key becomes its character.
// Anything else (Alt, Super, the protocol's private-use function keys) has no
// meaning here and is dropped.
func csiUKey(params []byte) (Key, error) {
	fields := strings.Split(string(params), ";")
	field := func(i, def int) (int, bool) {
		if i >= len(fields) || fields[i] == "" {
			return def, true
		}
		v, err := strconv.Atoi(strings.SplitN(fields[i], ":", 2)[0])
		return v, err == nil
	}
	code, ok1 := field(0, -1)
	mods, ok2 := field(1, 1)
	if !ok1 || !ok2 || code < 0 || mods < 1 {
		return Key{}, ErrNoInput
	}

	const (
		shift = 1
		ctrl  = 4
		locks = 64 | 128 // caps lock, num lock: not modifiers for our purposes
	)
	m := (mods - 1) &^ locks

	switch {
	case code >= 0xE000 && code <= 0xF8FF: // functional keys live in private use
		return Key{}, ErrNoInput
	case m == 0:
		return Key{Regular: rune(code), Special: KeyNoSpl}, nil
	case m == shift:
		return Key{Regular: unicode.ToUpper(rune(code)), Special: KeyNoSpl}, nil
	case m&^shift == ctrl:
		switch {
		case code >= 'a' && code <= 'z':
			return Key{Regular: rune(code - 'a' + 1), Special: KeyNoSpl}, nil
		case code >= '@' && code <= '_' && code != '@': // A-Z [ \ ] ^ _
			return Key{Regular: rune(code - '@'), Special: KeyNoSpl}, nil
		}
	}
	return Key{}, ErrNoInput
}
