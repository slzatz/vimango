package vim

/*
#include "libvim_api.h"

extern int goSubstituteConfirm(linenr_T lnum, colnr_T col, linenr_T endLnum, colnr_T endCol,
                               unsigned char *sub);
*/
import "C"

import "sync/atomic"

// ":s/pat/sub/c" asks before each change. vim shows the match, writes
// "replace with sub (y/n/a/q/l/^E/^Y)?" and waits for a key. libvim has no
// screen to show the match on and no keyboard, so it asks the application:
// once for each match, in the middle of the call that ran the command.

// SubstituteMatch is the match ":s" with the c flag is asking about: from
// byte Col of line Line (1-based) up to, not including, byte EndCol of line
// EndLine, and the replacement as it was typed ("[&]\r", not what it comes
// to).
type SubstituteMatch struct {
	Line, Col       int
	EndLine, EndCol int
	Replacement     string
}

// substituteConfirm is who answers (SetSubstituteConfirm), or nil.
var substituteConfirm func(SubstituteMatch) int

// asking is set while substituteConfirm is waiting for the user. A call
// into libvim is in progress then, and it is not one an interrupt can stop:
// it is waiting for a key, and CTRL-C is one (Interrupt).
var asking atomic.Bool

// SetSubstituteConfirm sets who answers when ":s" with the c flag asks about
// a match. ask returns the key the user typed: 'y' to change the match, 'n'
// to leave it, 'a' for it and all the rest, 'l' for it and no more, and 'q',
// <esc> (27) or CTRL-C (3) to stop there. For any other key vim asks again.
//
// **ask is the one thing libvim calls that waits for the user.** It runs on
// the goroutine that sent the key or ran the command, inside that call,
// which returns when the last match is answered; ask reads the key itself.
// Meanwhile it may read what it likes -- the buffer and the cursor are what
// vim would be showing, with the changes accepted so far in place and the
// cursor on the match -- and must not send keys, run a command or change
// the buffer.
//
// With nil, the question is answered with <esc>: nothing more is changed.
// Keys already typed are the answer before anyone is asked, as in vim: the
// rest of a mapping, or of a ":normal".
//
// void vimSetSubstituteConfirmCallback(SubstituteConfirmCallback callback);
func SetSubstituteConfirm(ask func(SubstituteMatch) int) {
	substituteConfirm = ask
}

// enableSubstituteConfirm has libvim ask goSubstituteConfirm. InitializeVim
// calls it.
func enableSubstituteConfirm() {
	C.vimSetSubstituteConfirmCallback(C.SubstituteConfirmCallback(C.goSubstituteConfirm))
}
