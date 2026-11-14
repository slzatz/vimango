package editor

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slzatz/vimango/internal/localdb"
	"github.com/slzatz/vimango/internal/store"
)

// The editor finds vimango.vim, spell/ and the chroma style relative to the
// working directory, which for vimango is its own directory. go test runs a
// package's tests in the package's directory, so they start by going there.
func TestMain(m *testing.M) {
	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// captureStdout returns what fn writes to the terminal.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	real := os.Stdout
	os.Stdout = w
	out := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		out <- string(b)
	}()
	fn()
	os.Stdout = real
	w.Close()
	return <-out
}

// newTestStore is a throwaway vimango.db as --init makes one: the schema, the
// default containers, and the migrations (WAL, the search index and its
// triggers).
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := localdb.Open(filepath.Join(t.TempDir(), "vimango.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := localdb.Create(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

// stripANSI is s without its escape sequences. (The application has the same
// function, for wrapping the preview; the editor only needs it to read what a
// test drew.)
func stripANSI(s string) string {
	var result strings.Builder
	// States: 0=Normal, 1=Saw ESC, 2=Inside CSI, 3=Inside OSC, 4=Inside APC
	state := 0
	var prevRune rune

	for _, r := range s {
		switch state {
		case 0: // Normal text
			if r == '\x1b' {
				state = 1
			} else {
				result.WriteRune(r)
			}
		case 1: // Saw ESC
			if r == '[' {
				state = 2 // CSI sequence
			} else if r == ']' {
				state = 3 // OSC sequence
			} else if r == '_' {
				state = 4 // APC sequence
			} else {
				state = 0 // Unknown escape, reset
			}
		case 2: // Inside CSI sequence - ends with letter @ through ~
			if r >= '@' && r <= '~' {
				state = 0
			}
		case 3: // Inside OSC sequence - ends with BEL (\x07) or ST (\x1b\)
			if r == '\x07' {
				state = 0
			} else if prevRune == '\x1b' && r == '\\' {
				state = 0
			}
		case 4: // Inside APC sequence - ends with ST (\x1b\)
			if prevRune == '\x1b' && r == '\\' {
				state = 0
			}
		}
		prevRune = r
	}
	return result.String()
}

// ctrlKey is the byte Ctrl and a letter send.
func ctrlKey(b byte) int { return int(b & 0x1f) }
