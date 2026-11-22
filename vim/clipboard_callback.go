package vim

// The yank and clipboard callbacks live in their own file for the reason the
// others do: a file with //export may only have declarations in its cgo
// preamble. The C types are spelled out (char_u is unsigned char).

/*
#include <stdlib.h>
*/
import "C"

import (
	"strings"
	"unsafe"
)

// libvim calls this synchronously from inside vimInput or vimExecute, on the
// goroutine that made the call. The lines are the register's own memory, so
// they are copied here, before it returns.
//
//export goYankCallback
func goYankCallback(op, regname, blockType, numLines C.int, lines **C.uchar) {
	if yankCallback == nil {
		return
	}
	y := Yank{Op: byte(op), Register: byte(regname), Type: RegisterType(blockType)}
	if numLines > 0 && lines != nil {
		for _, line := range unsafe.Slice(lines, int(numLines)) {
			y.Lines = append(y.Lines, goString(line))
		}
	}
	yankCallback(y)
}

// libvim calls this at every read of a register, with the register's name. A
// true answer hands over an array of lines that vim keeps as the register's
// and frees with free(), so the array and each line come from malloc(). It
// keeps them only for "*, "+ and the unnamed register; an answer for any
// other is lost memory, and none is given.
//
//export goClipboardGetCallback
func goClipboardGetCallback(regname C.int, numLines *C.int, lines ***C.uchar, blockType *C.int) C.int {
	if clipboardRead == nil || (regname != '+' && regname != '*') {
		return 0
	}
	text, linewise, ok := clipboardRead(byte(regname))
	if !ok || len(text) == 0 {
		return 0
	}
	array := (**C.uchar)(C.malloc(C.size_t(len(text)) * C.size_t(unsafe.Sizeof((*C.uchar)(nil)))))
	slots := unsafe.Slice(array, len(text))
	for i, line := range text {
		// a NUL would end the line where it is
		slots[i] = (*C.uchar)(unsafe.Pointer(C.CString(strings.ReplaceAll(line, "\x00", ""))))
	}
	*lines = array
	*numLines = C.int(len(text))
	*blockType = C.int(Charwise)
	if linewise {
		*blockType = C.int(Linewise)
	}
	return 1
}
