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

// ListsOutput reports whether this libvim.a sends what a command lists --
// ":registers", ":map", ":print", ":history" -- as a message. The fork does
// from 6fc194385. A library from before it links just the same and says
// nothing for those commands, and it never comes back from a listing of
// variables (":let", ":let g:"), so the application asks before forwarding
// one.
func ListsOutput() bool {
	return listsOutput
}

var listsOutput bool

// probeListsOutput finds out by running a command that only lists. ":echon"
// writes its text a piece at a time and has no message of its own.
func probeListsOutput() {
	msgs := CaptureMessages(func() { ExecuteCommand("echon 'x'") })
	listsOutput = len(msgs) == 1 && msgs[0].Text == "x"
}

// enableMessageCallback routes vim's messages to goMessageCallback.
// InitializeVim calls it.
//
// void vimSetMessageCallback(MessageCallback messageCallback);
func enableMessageCallback() {
	C.vimSetMessageCallback(C.MessageCallback(C.goMessageCallback))
}
