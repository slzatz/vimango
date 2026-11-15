package vim

/*
#include "libvim_api.h"

extern void goCursorMoveScreenLine(int motion, int count, linenr_T startLine, linenr_T *destLine);
extern void goCursorMoveScreenPosition(int direction, int count, linenr_T lnum, colnr_T cursor,
                                       colnr_T curswant, linenr_T *destLnum, colnr_T *destCol);
extern void goCursorMoveScreenColumn(int motion, linenr_T lnum, colnr_T col, colnr_T *destCol);
extern void goScroll(int dir, long count);

static void virtualColumns(linenr_T lnum, colnr_T col, colnr_T *start, colnr_T *end)
{
  pos_T pos = {lnum, col, 0};
  *start = *end = 0;
  getvcol(curwin, &pos, start, NULL, end);
}

// The cursor to another line, as vim's own scrolling moves it: to the first
// non-blank after CTRL-D, CTRL-U, CTRL-F and CTRL-B (or the wanted column,
// with 'nostartofline'), to the wanted column after CTRL-E and CTRL-Y.
static void scrollCursorTo(linenr_T lnum, int startOfLine)
{
  pos_T pos = vimCursorGetPosition();
  update_curswant(); // vim puts off working it out until something asks
  pos.lnum = lnum;
  vimCursorSetPosition(pos);
  if (startOfLine)
    beginline(BL_SOL | BL_FIX);
  else
    coladvance(vimCursorGetColumnWant());
}
*/
import "C"

import "math"

// The motions that move by what is on screen -- gj and gk, H, M and L, and
// g0, g^, gm and g$ -- are not vim's in libvim. vim works them out from its
// window, and libvim has none that matches what the embedder draws (vimango
// wraps lines itself, at word boundaries), so libvim asks the embedder
// through three callbacks. The commands that scroll are not vim's either:
// libvim hands each to the embedder, through a fourth.

// ScreenLineMotion is which line of the window H, M or L wants.
type ScreenLineMotion int

const (
	ScreenTop    ScreenLineMotion = C.MOTION_H // H
	ScreenBottom ScreenLineMotion = C.MOTION_L // L
	ScreenMiddle ScreenLineMotion = C.MOTION_M // M
)

// ScreenColumnMotion is which character of a screen line g0, gm or g$ wants.
type ScreenColumnMotion int

const (
	ScreenLineStart  ScreenColumnMotion = C.MOTION_SCREEN_START  // g0, and g^ on its way
	ScreenLineMiddle ScreenColumnMotion = C.MOTION_SCREEN_MIDDLE // gm
	ScreenLineEnd    ScreenColumnMotion = C.MOTION_SCREEN_END    // g$
)

// Scroll is a way vim asks for the window to be scrolled. The first three put
// the line the cursor is on somewhere in the window; the rest move the
// window over the text, and it is the embedder that then moves the cursor,
// with ScrollCursorTo, if it has to.
type Scroll int

const (
	ScrollCursorTop    Scroll = C.SCROLL_CURSOR_TOP     // zt, z<CR>
	ScrollCursorMiddle Scroll = C.SCROLL_CURSOR_CENTERV // zz, z.
	ScrollCursorBottom Scroll = C.SCROLL_CURSOR_BOTTOM  // zb, z-
	ScrollLineUp       Scroll = C.SCROLL_LINE_UP        // CTRL-E: the text goes up count lines
	ScrollLineDown     Scroll = C.SCROLL_LINE_DOWN      // CTRL-Y
	ScrollHalfPageDown Scroll = C.SCROLL_HALFPAGE_DOWN  // CTRL-D: count lines on in the text, 0 for no count typed
	ScrollHalfPageUp   Scroll = C.SCROLL_HALFPAGE_UP    // CTRL-U
	ScrollPageDown     Scroll = C.SCROLL_PAGE_DOWN      // CTRL-F, <PageDown>: count pages
	ScrollPageUp       Scroll = C.SCROLL_PAGE_UP        // CTRL-B, <PageUp>
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
	// ScreenColumn is g0, gm and g$: the byte offset, in line, of a character
	// of the screen line that holds byte col -- its first, its last, or the
	// one half the width of the window along it (or its last, on a shorter
	// one). g^ asks for the first and goes on to the first non-blank itself,
	// and g$ with a count is ScreenPosition, count-1 lines down after $.
	ScreenColumn(motion ScreenColumnMotion, line, col int) int
	// Scroll is a command that scrolls, which vim leaves whole to the
	// layout: where the window goes, and where the cursor goes with it
	// (ScrollCursorTo). vim has already done what such a command does
	// first -- 5zt has gone to line 5, z<CR> to the first non-blank. The
	// commands that scroll sideways (zh, zl, zs, ze) are not passed on:
	// they are for a window that does not wrap.
	Scroll(s Scroll, count int)
}

var screenLayout ScreenLayout

// SetScreenLayout makes l the one vim asks. With nil, gj, gk, H, M and L do
// nothing, and so do the commands that scroll; g0, g^, gm and g$ are
// libvim's own again, worked out from its window of 80 columns: on a title,
// which is one line, they are 0, ^ and $.
//
// void vimSetCursorMoveScreenColumnCallback(CursorMoveScreenColumnCallback f);
// void vimSetScrollCallback(ScrollCallback f);
func SetScreenLayout(l ScreenLayout) {
	screenLayout = l
	if l == nil {
		C.vimSetCursorMoveScreenColumnCallback(nil)
		C.vimSetScrollCallback(nil)
		return
	}
	C.vimSetCursorMoveScreenColumnCallback(C.CursorMoveScreenColumnCallback(C.goCursorMoveScreenColumn))
	C.vimSetScrollCallback(C.ScrollCallback(C.goScroll))
}

// ScrollCursorTo moves the cursor to line (1-based, kept inside the buffer)
// the way a scrolling command of vim's does, and is for a layout's Scroll to
// call: to the first non-blank of the line, or with 'nostartofline' the
// column vim wants (CTRL-D, CTRL-U, CTRL-F, CTRL-B); or, startOfLine false,
// always to the column vim wants (CTRL-E and CTRL-Y, when the cursor's line
// has left the window).
func ScrollCursorTo(line int, startOfLine bool) {
	sol := C.int(0)
	if startOfLine {
		sol = 1
	}
	C.scrollCursorTo(C.linenr_T(line), sol)
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
