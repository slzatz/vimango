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
// intended for strings with one character (could be multi-byte)
// no error if string has more than one char
func Input(s string) {
	C.vimInput(ucharP(s))

	/* this should also work
	z := unsafe.Pointer(C.CString(s))
	defer C.free(z)
	C.vimInput((*C.uchar)(z))
	*/
}

// intended for strings of more than one character
func Input2(s string) {
	/*
		// previous code - note sure it was ever correct
		for _, b := range []byte(s) {
			C.vimInput((*C.uchar)(&b))
		}
	*/
	// I think the below is correct but untested
	for _, x := range s {
		C.vimInput(ucharP(string(x)))
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
func BufferGetLineB(vbuf *C.buf_T, lineNum int) []byte {
	line := C.vimBufferGetLine(vbuf, C.long(lineNum))
	// need to cast to *C.char to use strlen
	// in vim.h: #define STRLEN(s) strlen((char *)(s))
	// this didn't work ln := C.int(C.STRLEN(unsafe.Pointer(line)))
	ln := C.int(C.strlen((*C.char)(unsafe.Pointer(line))))
	bb := C.GoBytes(unsafe.Pointer(line), ln)
	return bb
}

// returns [][]byte
func BufferLinesB(vbuf *C.buf_T) [][]byte {
	var bbb [][]byte
	lc := BufferGetLineCount(vbuf)
	for i := 1; i <= lc; i++ {
		bb := BufferGetLineB(vbuf, i)
		bbb = append(bbb, bb)
	}
	return bbb
}

// returns []string
func BufferLines(vbuf *C.buf_T) []string {
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
func BufferSetLinesB(vbuf *C.buf_T, start, end int, bb [][]byte, count int) {
	p := C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(uintptr(0))))
	defer C.free(unsafe.Pointer(p))
	view := (*[1<<30 - 1]*C.uchar)(unsafe.Pointer(p))[0:count:count]

	for i, b := range bb {
		b = append(b, 0) // make sure c-string will be zero-terminated
		view[i] = (*C.uchar)(C.CBytes(b))
		defer C.free(unsafe.Pointer(view[i]))
	}
	/* Either of the below work */
	//C.vimBufferSetLines(vbuf, C.long(start), C.long(end), &view[0], C.int(count))
	C.vimBufferSetLines(vbuf, C.long(start), C.long(end), (**C.uchar)(p), C.int(count))
}

//void vimBufferSetLines(buf_T *buf, linenr_T start, linenr_T end, char_u **lines, int count);
func BufferSetLines(vbuf *C.buf_T, start, end int, ss []string, count int) {
	p := C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(uintptr(0))))
	defer C.free(unsafe.Pointer(p))
	view := (*[1<<30 - 1]*C.uchar)(unsafe.Pointer(p))[0:count:count]

	for i, s := range ss {
		view[i] = (*C.uchar)(unsafe.Pointer(C.CString(s)))
		defer C.free(unsafe.Pointer(view[i]))
	}
	/* Either of the below work */
	//C.vimBufferSetLines(vbuf, C.long(start), C.long(end), &view[0], C.int(count))
	C.vimBufferSetLines(vbuf, C.long(start), C.long(end), (**C.uchar)(p), C.int(count))
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

/* There are some modal input experiences that aren't considered
full-fledged modes, but are nevertheless a modal input state.
Examples include insert-literal (C-V, C-G), search w/ confirmation, etc.
*/
//subMode_T vimGetSubMode(void);
// not in use
func GetSubmode() C.subMode_T {
	return C.vimGetSubMode()
}

type pendingOp_T struct {
	op_type int
	regname int
	count   int
}

//vimGetPendingOperator(pendingOp_T *pendingOp);
// unfortunately not very useful - does catch commands like 'd', 'c', 'Nd', 'Nc'
// not in use
func GetPendingOperator() (int, pendingOp_T) {
	var pendingOp C.pendingOp_T
	x := C.vimGetPendingOperator(&pendingOp)
	y := pendingOp_T{
		op_type: int(pendingOp.op_type),
		regname: int(pendingOp.regname),
		count:   int(pendingOp.count),
	}
	return int(x), y
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
