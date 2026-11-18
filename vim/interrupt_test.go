package vim

import (
	"strings"
	"testing"
	"time"
)

// Interrupt, called from another goroutine as the key reader calls it, stops
// a command that would never end, and the next call works. Without it each
// of these would never return; the test's own deadline is the one for the
// whole package.
func TestInterruptStopsWhatNeverEnds(t *testing.T) {
	start := func() *Buffer {
		InitializeVim(0)
		b := NewBuffer(0)
		SetCurrentBuffer(b)
		b.SetLines(0, -1, []string{"alpha one", "bravo two", "charlie three"})
		SendKey("<esc>")
		SendMultiInput("gg0")
		return b
	}
	interruptSoon := func() <-chan bool {
		done := make(chan bool, 1)
		go func() {
			time.Sleep(100 * time.Millisecond)
			done <- Interrupt()
		}()
		return done
	}
	worksAfterwards := func(t *testing.T, b *Buffer) {
		t.Helper()
		if GetCurrentMode() != ModeNormal && GetCurrentMode() != ModeNormalBusy {
			t.Errorf("mode %v afterwards, want NORMAL", GetCurrentMode())
		}
		SendMultiInput("gg0x")
		if got := b.Lines()[0]; got != "lpha one" {
			t.Errorf("x afterwards left %q", got)
		}
	}

	t.Run("a while that never ends", func(t *testing.T) {
		b := start()
		done := interruptSoon()
		msgs := CaptureMessages(func() { ExecuteCommand("while 1 | endwhile") })
		if !<-done {
			t.Error("Interrupt said nothing was running")
		}
		if len(msgs) == 0 || msgs[len(msgs)-1] != interruptedMessage {
			t.Errorf("messages %v, want Interrupted last", msgs)
		}
		worksAfterwards(t, b)
	})

	t.Run("a mapping that types itself again", func(t *testing.T) {
		b := start()
		ExecuteCommand("nmap Q jkQ")
		defer ExecuteCommand("mapclear")
		done := interruptSoon()
		var msgs []Message
		SendUserKey(func() { msgs = CaptureMessages(func() { SendInput("Q") }) })
		<-done
		for _, m := range msgs {
			if strings.Contains(m.Text, ":qa") {
				t.Errorf("the interrupt was taken as a typed CTRL-C: %q", m.Text)
			}
		}
		worksAfterwards(t, b)
	})

	t.Run("in Insert mode", func(t *testing.T) {
		b := start()
		ExecuteCommand("imap Q aQ")
		defer ExecuteCommand("mapclear!")
		done := interruptSoon()
		SendUserKey(func() { SendMultiInput("iQ") })
		<-done
		if GetCurrentMode() == ModeInsert {
			t.Error("still in INSERT after the interrupt")
		}
		before := b.Lines()[0]
		if !strings.HasPrefix(before, "aaa") {
			t.Errorf("line 1 is %q: what was inserted should stay", before)
		}
		SendMultiInput("gg0x")
		if got := b.Lines()[0]; got != before[1:] {
			t.Errorf("x afterwards: %d bytes left of %d", len(got), len(before))
		}
	})

	t.Run("with nothing running", func(t *testing.T) {
		b := start()
		if Interrupt() {
			t.Error("Interrupt with nothing running said it stopped something")
		}
		msgs := CaptureMessages(func() { SendInput("x") })
		if len(msgs) != 0 {
			t.Errorf("messages %v for x", msgs)
		}
		if got := b.Lines()[0]; got != "lpha one" {
			t.Errorf("x left %q", got)
		}
	})
}
