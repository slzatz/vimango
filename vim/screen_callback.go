package vim

// The screen-motion callbacks live in their own file for the reason the spell
// callback does: a file with //export may only have declarations in its cgo
// preamble. The C types are spelled out (linenr_T is long, colnr_T int,
// screenLineMotion_T an int-sized enum).

import "C"

// libvim calls these synchronously from inside vimInput, on the goroutine
// that sent the key, so they need no locking. Each starts with the
// destination set to where the cursor is.

//export goCursorMoveScreenLine
func goCursorMoveScreenLine(motion C.int, count C.int, startLine C.long, destLine *C.long) {
	if screenLayout == nil {
		return
	}
	*destLine = C.long(screenLayout.ScreenLine(ScreenLineMotion(motion), int(count), int(startLine)))
}

//export goCursorMoveScreenPosition
func goCursorMoveScreenPosition(direction C.int, count C.int, lnum C.long, cursor C.int,
	curswant C.int, destLnum *C.long, destCol *C.int) {
	if screenLayout == nil {
		return
	}
	line, want := screenLayout.ScreenPosition(direction > 0, int(count), int(lnum), int(cursor), int(curswant))
	*destLnum = C.long(line)
	*destCol = C.int(want)
}
