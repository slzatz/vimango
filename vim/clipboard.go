package vim

/*
#include "libvim_api.h"

extern void goYankCallback(int op, int regname, int blockType, int numLines, unsigned char **lines);
extern int goClipboardGetCallback(int regname, int *numLines, unsigned char ***lines, int *blockType);

// libvim hands a yank over as a struct; the Go side takes its fields.
static void yankShim(yankInfo_T *y)
{
  goYankCallback(y->op_char, y->regname, y->blockType, y->numLines, y->lines);
}

static void setYankCallback(int on) { vimSetYankCallback(on ? yankShim : NULL); }
static void setClipboardGetCallback(int on) { vimSetClipboardGetCallback(on ? goClipboardGetCallback : NULL); }
*/
import "C"

// vim's registers live in one process. What leaves it, and what comes in from
// outside, goes through two calls libvim makes: one when a yank or a delete
// has filled a register, one when a register is about to be read. With
// neither set (the default) "* and "+ are two more registers of this vim's
// own. The 'clipboard' option is not in libvim: "set clipboard=unnamed" does
// nothing.

// RegisterType is what a register holds: characters, whole lines, or a block.
type RegisterType int

const (
	Charwise  RegisterType = C.MCHAR
	Linewise  RegisterType = C.MLINE
	Blockwise RegisterType = C.MBLOCK
)

// Yank is text an operator has just put in a register.
type Yank struct {
	Op       byte         // the operator: 'y', 'd', 'c'
	Register byte         // the register named with it ('a', '+', '*'), 0 when none was
	Type     RegisterType // how the text was taken
	Lines    []string     // the text; for Linewise, whole lines
}

var (
	yankCallback  func(Yank)
	clipboardRead func(register byte) (lines []string, linewise bool, ok bool)
)

// SetYankCallback sets who is told of every yank and delete: y, d, c, x and
// the rest, typed or as ":yank" and ":delete", with or without a register.
// Not of a register written any other way (a recorded macro, ":let @a",
// setreg()). nil tells nobody.
//
// fn runs on the goroutine that sent the key, inside that call, and must not
// call into vim.
//
// void vimSetYankCallback(YankCallback callback);
func SetYankCallback(fn func(Yank)) {
	yankCallback = fn
	on := C.int(0)
	if fn != nil {
		on = 1
	}
	C.setYankCallback(on)
}

// SetClipboardCallback sets who supplies the text of the clipboard registers,
// "* and "+: read is asked each time one of them is read (a put, CTRL-R, an
// expression's @+) and answers with the text as lines, whether they are whole
// lines, and whether there is any. When there is none the register holds what
// was last yanked into it. nil makes the two registers the unnamed one again.
//
// **It is also what makes a delete into "* or "+ one**: with no clipboard to
// ask, libvim turns "+dd into a plain dd before it deletes (adjust_clip_reg),
// and the yank callback is told of no register. A yank into one ("+yy) is
// reported as that whether this is set or not.
//
// libvim asks at the read of every register, whichever it is, and would take
// an answer for the unnamed one too, which would make every "p" a paste from
// the clipboard. That is not offered here: read is asked for "* and "+ only.
//
// read runs on the goroutine that sent the key, inside that call, and must
// not call into vim.
//
// void vimSetClipboardGetCallback(ClipboardGetCallback callback);
func SetClipboardCallback(read func(register byte) (lines []string, linewise bool, ok bool)) {
	clipboardRead = read
	on := C.int(0)
	if read != nil {
		on = 1
	}
	C.setClipboardGetCallback(on)
}
