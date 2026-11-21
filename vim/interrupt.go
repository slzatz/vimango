package vim

/*
#include "libvim_api.h"
*/
import "C"

import "sync/atomic"

// A command can run for ever: ":while 1 | endwhile", a mapping that types
// itself again ("nmap Q jkQ"), a function that never returns. vim stops one
// on CTRL-C; libvim reads no keys while it runs, and the goroutine that would
// send the CTRL-C is the one inside the call. Interrupt is how another
// goroutine stops it.

// running counts the calls into libvim in progress that run commands or
// keys: what an interrupt can stop. Every function here that sends keys,
// runs an ex command or evaluates an expression adds itself while it runs.
var running atomic.Int32

// interrupted is set by Interrupt, and taken by CaptureMessages, which
// reports it as vim would have: vim itself says nothing when a ":while" is
// stopped.
var interrupted atomic.Bool

// enter and leave bracket a call that runs commands: defer leave(enter()).
func enter() struct{} { running.Add(1); return struct{}{} }
func leave(struct{})  { running.Add(-1) }

// Interrupt stops what vim is running, as CTRL-C does in vim, and reports
// whether anything was: with nothing running it does nothing. The call that
// was running returns; the keys still waiting in vim are dropped, Insert mode
// and a command line are ended, and the next call works as usual.
//
// **It is the one function in this package that may be called from any
// goroutine** -- it is meant to be, by whatever reads the keyboard while the
// main loop is inside libvim. libvim's vimInterrupt only sets a flag (vim's
// got_int), as a signal handler would.
//
// void vimInterrupt(void);
//
// While ":s" with the c flag is waiting for its answer (SetSubstituteConfirm)
// there is nothing to stop, and it reports false: the CTRL-C is a key, for
// whoever is being asked, and answers the question.
func Interrupt() bool {
	if running.Load() == 0 || asking.Load() {
		return false
	}
	// recorded first: the call can return as soon as vim has the flag, and
	// CaptureMessages looks for this on its way out
	interrupted.Store(true)
	C.vimInterrupt()
	return true
}

// interruptedMessage is what CaptureMessages adds for a call Interrupt
// stopped. vim's own word for it (e_interr), which some commands -- :g, :s,
// :sort -- say themselves.
var interruptedMessage = Message{Text: "Interrupted", Error: true}
