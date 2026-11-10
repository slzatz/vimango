package vim

/*
#include "src/libvim.h"

extern void goMessageCallback(unsigned char *title, unsigned char *msg, int priority);
*/
import "C"

// Message is something vim would have shown on its message line: the output
// of a command (":echo", ":set ts?", ":registers") or an error ("E492: Not an
// editor command"). libvim has no screen, so it hands them to a callback.
type Message struct {
	Title string // usually empty; set for a few multi-part outputs
	Text  string
	Error bool
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
