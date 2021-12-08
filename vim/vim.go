package vim

import (
	"unsafe"
)

/*
#include "src/libvim.h"
#cgo CFLAGS: -Iproto -DHAVE_CONFIG_H
#cgo LDFLAGS: libvim.a -lm -ltinfo -ldl -lacl
*/
import "C"

type Buffer *C.buf_T

func ucharP(s string) *C.uchar {
	var bb = []byte(s)
	bb = append(bb, 0)
	return (*C.uchar)(&bb[0])
}

//void vimInit(int argc, char **argv);
func Init(argc int) {
	var c *C.char
	C.vimInit(C.int(argc), &c)
}

//buf_T *vimBufferOpen(char_u *ffname_arg, linenr_T lnum, int flags);
func BufferOpen(filename string, lnum int, flags int) *C.buf_T {
	vbuf := C.vimBufferOpen(ucharP(filename), C.long(lnum), C.int(flags))
	return vbuf
}

//buf_T *vimBufferLoad(char_u *ffname_arg, linenr_T lnum, int flags)
//buf_T *vimBufferNew(int flags)
func BufferNew(flags int) *C.buf_T {
	vbuf := C.vimBufferNew(C.int(flags))
	return vbuf
}

//int vimBufferGetId(buf_T *buf);
func BufferGetId(vbuf *C.buf_T) int {
	id := C.vimBufferGetId(vbuf)
	return int(id)
}

//void vimBufferSetCurrent(buf_T *buf);
func BufferSetCurrent(vbuf *C.buf_T) {
	C.vimBufferSetCurrent(vbuf)
}

//buf_T *vimBufferGetCurrent(void);
func BufferGetCurrent() *C.buf_T {
	return C.curbuf
}

//void vimInput(char_u *input);
func Input(s string) {
	// can't use C.CString because wants uchar
	C.vimInput(ucharP(s))
}

func Input2(s string) {
	for _, b := range []byte(s) {
		C.vimInput((*C.uchar)(&b))
	}
}

//void vimKey(char_u *key);
func Key(s string) {
	C.vimKey(ucharP(s))
}

//char_u *vimBufferGetLine(buf_T *buf, linenr_T lnum);
//typedef long linenr_T;
//buf_T -> file_buffer is a complicated struct
func BufferGetLine(vbuf *C.buf_T, lineNum int) string {
	line := C.vimBufferGetLine(vbuf, C.long(lineNum))
	// not sure about casting C.uchar to C.char
	// and then using C.GoString but it seems to work
	// created BufferGetLine2 to explore alternative
	data := (*C.char)(unsafe.Pointer(line))
	s := C.GoString(data)
	return s
}

//char_u *vimBufferGetLine(buf_T *buf, linenr_T lnum);
func BufferGetLine2(vbuf *C.buf_T, lineNum int) []byte {
	line := C.vimBufferGetLine(vbuf, C.long(lineNum))
	// need to cast to *C.char to use strlen
	// in vim.h: #define STRLEN(s) strlen((char *)(s))
	// this didn't work ln := C.int(C.STRLEN(unsafe.Pointer(line)))
	ln := C.int(C.strlen((*C.char)(unsafe.Pointer(line))))
	bb := C.GoBytes(unsafe.Pointer(line), ln)
	return bb
}

// returns [][]byte
func BufferLines(vbuf *C.buf_T) [][]byte {
	var bbb [][]byte
	lc := BufferGetLineCount(vbuf)
	for i := 1; i <= lc; i++ {
		bb := BufferGetLine2(vbuf, i)
		bbb = append(bbb, bb)
	}
	return bbb
}

// returns []string
func BufferLinesS(vbuf *C.buf_T) []string {
	// should probably use string builder
	// line count starts from 1
	var ss []string
	lc := BufferGetLineCount(vbuf)
	for i := 1; i <= lc; i++ {
		s := BufferGetLine(vbuf, i)
		ss = append(ss, s)
	}
	return ss
}

//linenr_T vimCursorGetLine(void);
func CursorGetLine() int {
	lineNum := C.vimCursorGetLine()
	return int(lineNum)
}

//size_t vimBufferGetLineCount(buf_T *buf);
func BufferGetLineCount(vbuf *C.buf_T) int {
	lc := C.vimBufferGetLineCount(vbuf)
	return int(lc)
}

//void vimExecute(char_u *cmd);
func Execute(s string) {
	C.vimExecute(ucharP(s))
}

//void vimBufferSetLines(buf_T *buf, linenr_T start, linenr_T end, char_u **lines, int count);
// to avoid pointer arithmetic this repeatedly call C.vimBufferSetLines
// with one line - might be possible to address this
func BufferSetLines(vbuf *C.buf_T, bb [][]byte) {
	// size is the length of the longest line + 1 for null terminator
	size := 0
	for _, b := range bb {
		if len(b) > size {
			size = len(b)
		}
	}
	size += 1 //for trailing null for longest line(s)

	p1 := (*C.uchar)(C.malloc(C.sizeof_uchar * C.ulong(size)))
	p2 := (**C.uchar)(C.malloc(C.sizeof_uint))
	p2 = &p1

	view := (*[1 << 30]C.uchar)(unsafe.Pointer(p1))[0:size]
	for start, line := range bb {
		i := 0
		for _, x := range line {
			view[i] = C.uchar(x)
			i += 1
		}
		view[i] = 0
		C.vimBufferSetLines(vbuf, C.long(start), C.long(start-1), p2, C.int(1))
	}
	C.free(unsafe.Pointer(p1))
	//C.free(unsafe.Pointer(p2)) //panics
}

func BufferSetLine(vbuf *C.buf_T, line int, b []byte) {
	// size is the length of the longest line + 1 for null terminator
	size := len(b) + 1

	p1 := (*C.uchar)(C.malloc(C.sizeof_uchar * C.ulong(size)))
	p2 := (**C.uchar)(C.malloc(C.sizeof_uint))
	p2 = &p1

	view := (*[1 << 30]C.uchar)(unsafe.Pointer(p1))[0:size]
	i := 0
	for i = 0; i < size-1; i++ {
		view[i] = C.uchar(b[i])
	}
	view[size-1] = 0
	C.vimBufferSetLines(vbuf, C.long(line), C.long(line), p2, C.int(1))
	C.free(unsafe.Pointer(p1))
	//C.free(unsafe.Pointer(p2)) //panics
}

func BufferSetLine__(vbuf *C.buf_T, b []byte) {
	// size is the length of the longest line + 1 for null terminator
	size := len(b) + 1

	p1 := (*C.uchar)(C.malloc(C.sizeof_uchar * C.ulong(size)))
	p2 := (**C.uchar)(C.malloc(C.sizeof_uint))
	p2 = &p1

	view := (*[1 << 30]C.uchar)(unsafe.Pointer(p1))[0:size]
	i := 0
	for i = 0; i < size-1; i++ {
		view[i] = C.uchar(b[i])
	}
	view[size-1] = 0
	C.vimBufferSetLines(vbuf, C.long(0), C.long(-1), p2, C.int(1))
	C.free(unsafe.Pointer(p1))
	//C.free(unsafe.Pointer(p2)) //panics
}

//v.SetBufferText(e.vbuf, line, startChar, line, endChar, [][]byte{[]byte(edit.NewText)})
//pos_T vimCursorGetPosition(void);
func CursorGetPosition() [2]int {
	p := C.vimCursorGetPosition()
	var pos [2]int
	pos[0] = int(p.lnum)
	pos[1] = int(p.col)
	return pos
}

//void vimCursorSetPosition(pos_T pos);
func CursorSetPosition_old(pos [2]int) {
	var p C.pos_T
	p.lnum = C.long(pos[0])
	p.col = C.int(pos[1])
	C.vimCursorSetPosition(p)
}

//void vimCursorSetPosition(pos_T pos);
func CursorSetPosition(r, c int) {
	var p C.pos_T
	p.lnum = C.long(r)
	p.col = C.int(c)
	C.vimCursorSetPosition(p)
}

/*
int vimGetMode(void);
modes are in vim.h there are DEFINES for NORMAL, VISUAL, INSERT, OP_PENDING (blocking?)
etc.
NORMAL 1
VISUAL 2 (v -> 118; V -> 86; ctrl-v -> 22)
OP_PENDING 4; examples "2d" "da" etc
CMDLINE 8
INSERT 16 0x10
REPLACE -> NORMAL_BUSY 257 0x101
SEARCH 8
Note there are two modes which sometimes overlap and sometimes don't: the mode vim is in and the mode that listmango is in
*/
func GetMode() int {
	m := C.vimGetMode()
	return int(m)
}

//int vimBufferGetModified(buf_T *buf);
func BufferGetModified(vbuf *C.buf_T) bool {
	b := C.vimBufferGetModified(vbuf)
	if b == 0 {
		return false
	}
	return true
}

//void vimVisualGetRange(pos_T *startPos, pos_T *endPos);
func VisualGetRange() [2][2]int {
	var startPos, endPos C.pos_T
	C.vimVisualGetRange(&startPos, &endPos)
	var pos [2][2]int
	pos[0][0] = int(startPos.lnum)
	pos[0][1] = int(startPos.col)
	pos[1][0] = int(endPos.lnum)
	pos[1][1] = int(endPos.col)
	return pos
}

/*
vimVisualGetType() == Ctrl_V);
int vimVisualGetType(void) { return VIsual_mode; }
Visual = 118 'v'
Visual Line = 86 'V'
Visual block = 22  ctrl-v
*/
func VisualGetType() int {
	t := C.vimVisualGetType()
	return int(t)
}

func Eval(s string) string {
	r := C.vimEval(ucharP(s))
	data := (*C.char)(unsafe.Pointer(r))
	return C.GoString(data)
}

func SearchGetMatchingPair() [2]int {
	var pos [2]int
	p := C.vimSearchGetMatchingPair(0)
	if p == nil {
		return pos
	}
	pos[0] = int(p.lnum)
	pos[1] = int(p.col)
	return pos
}

func BufferGetLastChangedTick(vbuf *C.buf_T) int {
	tick := C.vimBufferGetLastChangedTick(vbuf)
	return int(tick)
}

/* topline doesn't seem to work so no need for the below
func WindowGetTopLine() int {
	t := C.vimWindowGetTopLine()
	return int(t)
}

func WindowGetWidth() int {
	w := C.vimWindowGetWidth()
	return int(w)
}

func WindowGetHeight() int {
	w := C.vimWindowGetHeight()
	return int(w)
}

func WindowSetWidth(w int) {
	C.vimWindowSetWidth(C.int(w))
}

func WindowSetHeight(h int) {
	C.vimWindowSetHeight(C.int(h))
}
*/
