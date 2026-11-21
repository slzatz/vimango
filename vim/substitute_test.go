package vim

import (
	"fmt"
	"slices"
	"testing"
)

// ":s" with the c flag asks whoever SetSubstituteConfirm names, once for
// each match, and does with each what the answer says: vim's own loop, with
// the application in place of the keyboard.
func TestSubstituteConfirmAsks(t *testing.T) {
	start := func(lines ...string) *Buffer {
		InitializeVim(0)
		b := NewBuffer(0)
		SetCurrentBuffer(b)
		b.SetLines(0, -1, lines)
		SendKey("<esc>")
		SendMultiInput("gg0")
		return b
	}
	// the answers to give, and what was asked: the match, the line it is on
	// as the buffer has it during the question, and where the cursor is
	var asked []string
	answerWith := func(b *Buffer, keys string) {
		asked = nil
		SetSubstituteConfirm(func(m SubstituteMatch) int {
			pos := GetCursorPosition()
			asked = append(asked, fmt.Sprintf("%d:%d-%d:%d %s %q cursor %d,%d",
				m.Line, m.Col, m.EndLine, m.EndCol, m.Replacement, b.Lines()[m.Line-1], pos[0], pos[1]))
			if len(asked) > len(keys) {
				return keyEsc
			}
			return int(keys[len(asked)-1])
		})
	}
	t.Cleanup(func() { SetSubstituteConfirm(nil) })
	check := func(t *testing.T, b *Buffer, want ...string) {
		t.Helper()
		if got := b.Lines(); !slices.Equal(got, want) {
			t.Errorf("the buffer is %q, want %q", got, want)
		}
	}

	t.Run("y, n and q", func(t *testing.T) {
		b := start("foo bar foo", "foo", "bar foo")
		answerWith(b, "ynyq")
		ExecuteCommand("%s/foo/X/gc")
		check(t, b, "X bar foo", "X", "bar foo")
		// at the second question the first change is in the line, and the
		// match is where it is in that line
		want := []string{
			`1:0-1:3 X "foo bar foo" cursor 1,0`,
			`1:6-1:9 X "X bar foo" cursor 1,6`,
			`2:0-2:3 X "foo" cursor 2,0`,
			`3:4-3:7 X "bar foo" cursor 3,4`,
		}
		if !slices.Equal(asked, want) {
			t.Errorf("asked:\n%q\nwant:\n%q", asked, want)
		}
	})

	t.Run("a is all the rest, l the last", func(t *testing.T) {
		b := start("foo foo", "foo", "foo foo")
		answerWith(b, "na")
		ExecuteCommand("%s/foo/X/gc")
		check(t, b, "foo X", "X", "X X")

		b = start("foo foo", "foo", "foo foo")
		answerWith(b, "nl")
		ExecuteCommand("%s/foo/X/gc")
		check(t, b, "foo X", "foo", "foo foo")
	})

	t.Run("a match across a line break", func(t *testing.T) {
		b := start("a b", "c d", "b")
		answerWith(b, "y")
		ExecuteCommand(`%s/b\nc/Z/c`)
		check(t, b, "a Z d", "b")
		if want := []string{`1:2-2:1 Z "a b" cursor 1,2`}; !slices.Equal(asked, want) {
			t.Errorf("asked %q, want %q", asked, want)
		}
	})

	t.Run("one undo for all of it", func(t *testing.T) {
		b := start("foo bar foo", "foo", "bar foo")
		answerWith(b, "yyyy")
		SendInput(":%s/foo/X/gc\r")
		check(t, b, "X bar X", "X", "bar X")
		SendInput("u")
		check(t, b, "foo bar foo", "foo", "bar foo")
	})

	t.Run("keys already typed answer first", func(t *testing.T) {
		b := start("foo", "foo", "foo")
		answerWith(b, "y")
		ExecuteCommand("nnoremap Q :%s/foo/X/gc<CR>yn")
		defer ExecuteCommand("mapclear")
		SendUserKey(func() { SendInput("Q") })
		check(t, b, "X", "foo", "X")
		if len(asked) != 1 {
			t.Errorf("asked %d times, want once: the mapping answered the first two", len(asked))
		}
	})

	t.Run("nobody to ask", func(t *testing.T) {
		b := start("foo", "foo")
		SetSubstituteConfirm(nil)
		msgs := CaptureMessages(func() { ExecuteCommand("%s/foo/X/gc") })
		check(t, b, "foo", "foo")
		for _, m := range msgs {
			if m.Error {
				t.Errorf("an error: %q", m.Text)
			}
		}
	})
}

// While the question is asked a CTRL-C is a key, for whoever is asked: there
// is nothing running for Interrupt to stop, and it says so. Returned as the
// answer, it stops the command with what was accepted kept.
func TestInterruptIsAKeyWhileSubstituteAsks(t *testing.T) {
	InitializeVim(0)
	b := NewBuffer(0)
	SetCurrentBuffer(b)
	b.SetLines(0, -1, []string{"foo", "foo", "foo"})
	SendKey("<esc>")
	SendMultiInput("gg0")

	var took []bool
	SetSubstituteConfirm(func(SubstituteMatch) int {
		took = append(took, Interrupt())
		if len(took) == 1 {
			return 'y'
		}
		return 3 // CTRL-C
	})
	t.Cleanup(func() { SetSubstituteConfirm(nil) })

	msgs := CaptureMessages(func() { SendInput(":%s/foo/X/c\r") })
	if got, want := b.Lines(), []string{"X", "foo", "foo"}; !slices.Equal(got, want) {
		t.Errorf("the buffer is %q, want %q", got, want)
	}
	if !slices.Equal(took, []bool{false, false}) {
		t.Errorf("Interrupt during the questions said %v, want false each time", took)
	}
	for _, m := range msgs {
		if m == interruptedMessage {
			t.Error("the command was reported interrupted; CTRL-C was an answer")
		}
	}
	if Interrupt() {
		t.Error("Interrupt with nothing running said it stopped something")
	}
	SendMultiInput("gg0x")
	if got := b.Lines()[0]; got != "" {
		t.Errorf("x afterwards left %q", got)
	}
}

// Whoever is asked may read from vim and must send it nothing: vim is in the
// middle of a command. The package holds it to that. A key, a command, an
// expression, lines set, another buffer made current, the cursor moved --
// each made while the question is asked does nothing, and TakeRefused says
// which; what was read is right, and afterwards everything works again.
func TestNothingIsSentToVimWhileSubstituteAsks(t *testing.T) {
	InitializeVim(0)
	other := NewBuffer(0)
	b := NewBuffer(0)
	SetCurrentBuffer(b)
	b.SetLines(0, -1, []string{"foo one", "foo two"})
	SendKey("<esc>")
	SendMultiInput("gg0")
	TakeRefused()

	var read []string
	var evaluated string
	var captured []Message
	SetSubstituteConfirm(func(m SubstituteMatch) int {
		// reading
		read = append(read, fmt.Sprintf("%q cursor %v matches %d", b.Lines(), GetCursorPosition(), len(b.SearchMatches(1, 2))))
		// and everything that is not
		SendInput("x")
		SendMultiInput("dd")
		SendKey("<esc>")
		SendUserKey(func() { SendInput("x") })
		InputTimeout()
		ExecuteCommand("normal dd")
		evaluated = EvaluateExpression("execute('normal dd') . 'ran'")
		captured = CaptureMessages(func() { ExecuteCommand("echo 'said'") })
		b.SetLines(0, -1, []string{"gone"})
		SetCurrentBuffer(other)
		SetCursorPosition(2, 3)
		ScrollCursorTo(2, true)
		return 'n'
	})
	t.Cleanup(func() { SetSubstituteConfirm(nil); TakeRefused() })

	msgs := CaptureMessages(func() { SendInput(":%s/foo/X/c\r") })

	if got, want := b.Lines(), []string{"foo one", "foo two"}; !slices.Equal(got, want) {
		t.Errorf("the buffer is %q, want %q: something sent during the question got through", got, want)
	}
	if GetCurrentBuffer().GetID() != b.GetID() {
		t.Error("vim was taken to another buffer during the question")
	}
	wantRead := []string{
		`["foo one" "foo two"] cursor [1 0] matches 2`,
		`["foo one" "foo two"] cursor [2 0] matches 2`,
	}
	if !slices.Equal(read, wantRead) {
		t.Errorf("read during the questions:\n%q\nwant:\n%q", read, wantRead)
	}
	if evaluated != "" || captured != nil {
		t.Errorf("an expression gave %q and a capture %v during the question, want nothing from either", evaluated, captured)
	}
	for _, m := range msgs {
		if m.Text == "said" {
			t.Error("a command run during the question spoke into the command's own messages")
		}
	}
	want := []string{"SendInput", "SendMultiInput", "SendKey", "SendUserKey", "InputTimeout", "ExecuteCommand",
		"EvaluateExpression", "CaptureMessages", "SetLines", "SetCurrentBuffer", "SetCursorPosition", "ScrollCursorTo"}
	if got := TakeRefused(); !slices.Equal(got, want) {
		t.Errorf("refused %q, want %q", got, want)
	}
	if got := TakeRefused(); got != nil {
		t.Errorf("refused calls were not forgotten once taken: %q", got)
	}

	// the question over, the same calls are vim's again
	SendMultiInput("gg0x")
	ExecuteCommand("2d")
	if got, want := b.Lines(), []string{"oo one"}; !slices.Equal(got, want) {
		t.Errorf("afterwards the buffer is %q, want %q", got, want)
	}
	if got := TakeRefused(); got != nil {
		t.Errorf("calls after the question were refused: %q", got)
	}
}
