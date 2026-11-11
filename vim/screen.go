package vim

/*
#include "libvim_api.h"

extern void goCursorMoveScreenLine(int motion, int count, linenr_T startLine, linenr_T *destLine);
extern void goCursorMoveScreenPosition(int direction, int count, linenr_T lnum, colnr_T cursor,
                                       colnr_T curswant, linenr_T *destLnum, colnr_T *destCol);

static void virtualColumns(linenr_T lnum, colnr_T col, colnr_T *start, colnr_T *end)
{
  pos_T pos = {lnum, col, 0};
  *start = *end = 0;
  getvcol(curwin, &pos, start, NULL, end);
}
*/
import "C"

import "math"

// The motions that move by what is on screen -- gj and gk, H, M and L -- are
// not vim's in libvim. vim works them out from its window, and libvim has
// none that matches what the embedder draws (vimango wraps lines itself, at
// word boundaries), so libvim asks the embedder through two callbacks. With
// no ScreenLayout set the cursor stays where it is.

// ScreenLineMotion is which line of the window H, M or L wants.
type ScreenLineMotion int

const (
	ScreenTop    ScreenLineMotion = C.MOTION_H // H
	ScreenBottom ScreenLineMotion = C.MOTION_L // L
	ScreenMiddle ScreenLineMotion = C.MOTION_M // M
)

// MaxCol is the wanted column after $: the end of whatever line the cursor
// moves to.
const MaxCol = math.MaxInt32

// ScreenLayout answers vim's questions about how the current buffer is laid
// out on screen. Lines are 1-based, columns are byte offsets, and the wanted
// column (vim's curswant) is a virtual column: see VirtualColumns.
type ScreenLayout interface {
	// ScreenPosition is gj (down) and gk: count screen lines from line, col,
	// keeping to the wanted column. It returns the line to move to and the
	// virtual column to want there.
	ScreenPosition(down bool, count, line, col, curswant int) (destLine, destCurswant int)
	// ScreenLine is H, M and L: the line count lines from the top or the
	// bottom of the window, or the one in its middle.
	ScreenLine(motion ScreenLineMotion, count, line int) int
}

var screenLayout ScreenLayout

// SetScreenLayout makes l the one vim asks; nil makes the motions do nothing.
func SetScreenLayout(l ScreenLayout) {
	screenLayout = l
}

// void vimSetCursorMoveScreenLineCallback(CursorMoveScreenLineCallback f);
// void vimSetCursorMoveScreenPositionCallback(CursorMoveScreenPositionCallback f);
func enableScreenCallbacks() {
	C.vimSetCursorMoveScreenLineCallback(C.CursorMoveScreenLineCallback(C.goCursorMoveScreenLine))
	C.vimSetCursorMoveScreenPositionCallback(C.CursorMoveScreenPositionCallback(C.goCursorMoveScreenPosition))
}

// VirtualColumns returns the first and last of vim's virtual columns
// (0-based) that the character at byte offset col of line takes, in the
// current buffer: by vim's count, with its 'tabstop' and character widths. A
// wanted column is one of these; wanting a character's start lands on it.
// col is clamped to the line (its end is the column after the last
// character).
//
// void getvcol(win_T *wp, pos_T *pos, colnr_T *start, colnr_T *cursor, colnr_T *end);
func VirtualColumns(line, col int) (start, end int) {
	n := int(C.vimBufferGetLineCount(C.curbuf))
	if line < 1 || line > n {
		return 0, 0
	}
	col = max(0, min(col, len(goString(C.vimBufferGetLine(C.curbuf, C.linenr_T(line))))))
	var s, e C.colnr_T
	C.virtualColumns(C.linenr_T(line), C.colnr_T(col), &s, &e)
	return int(s), int(e)
}

// LineCount returns the number of lines in the buffer.
//
// size_t vimBufferGetLineCount(buf_T *buf);
func (b *Buffer) LineCount() int {
	return int(C.vimBufferGetLineCount(b.buf))
}
