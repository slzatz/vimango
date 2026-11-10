package vim

/*
#include "libvim_api.h"

extern void goMessageCallback(unsigned char *title, unsigned char *msg, int priority);
*/
import "C"

// Message is something vim would have shown on its message line: the output
// of a command (":echo", ":set ts?", ":marks"), a report ("5 lines yanked",
// "4 fewer lines"), a warning ("search hit BOTTOM, continuing at TOP") or an
// error ("E486: Pattern not found"). libvim has no screen, so it hands them
// to a callback.
type Message struct {
	Title   string // usually empty; set for a few multi-part outputs
	Text    string
	Warning bool
	Error   bool
}

// CaptureMessages runs fn and returns the messages vim produced while it ran.
// Outside a capture vim's messages are dropped, as they always were: the
// application draws its own message line.
func CaptureMessages(fn func()) []Message {
	capturing = true
	captured = nil
	defer func() { capturing = false; captured = nil }()
	fn()
	return captured
}

// enableMessageCallback routes vim's messages to goMessageCallback.
// InitializeVim calls it.
//
// void vimSetMessageCallback(MessageCallback messageCallback);
func enableMessageCallback() {
	C.vimSetMessageCallback(C.MessageCallback(C.goMessageCallback))
}
