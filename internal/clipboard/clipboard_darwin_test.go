package clipboard

import (
	"fmt"
	"os"
	"testing"
)

// Text goes onto a pasteboard and comes back as it went. On a pasteboard of
// the test's own: the system's holds whatever the user copied last.
func TestPasteboardRoundTrip(t *testing.T) {
	b := named(fmt.Sprintf("vimango.test.%d", os.Getpid()))
	t.Cleanup(b.release)

	if text, ok := b.Text(); ok {
		t.Fatalf("a new pasteboard holds %q", text)
	}
	for _, text := range []string{
		"alpha",
		"two\nlines\n",
		"tab\there, café ☕, 日本語",
		"- [ ] a task\n\n    code\n",
	} {
		if !b.SetText(text) {
			t.Skipf("no pasteboard to write to (no login session?)")
		}
		if got, ok := b.Text(); !ok || got != text {
			t.Errorf("put %q, got %q (%v)", text, got, ok)
		}
	}

	// what a C string cannot carry is left out, not a reason to fail
	if !b.SetText("a\x00b\xffc") {
		t.Fatal("text with a NUL and a bad byte was refused")
	}
	if got, _ := b.Text(); got != "ab�c" {
		t.Errorf("got %q", got)
	}

	// nothing is text that is not there
	if !b.SetText("") {
		t.Fatal("the empty text was refused")
	}
	if text, ok := b.Text(); ok {
		t.Errorf("an empty pasteboard holds %q", text)
	}
}

func TestSystemIsThere(t *testing.T) {
	if System() == nil {
		t.Error("no system clipboard on macOS")
	}
}
