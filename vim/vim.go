package vim

import (
	"fmt"
	"unsafe"
)

/*
#include "src/libvim.h"
#cgo CFLAGS: -Iproto -DHAVE_CONFIG_H
#cgo LDFLAGS: libvim.a -lm -ltinfo -ldl -lacl
*/
import "C"

func ucharP(s string) *C.uchar {
	var x = []byte(s)
	x = append(x, 0)
	return (*C.uchar)(&x[0])
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

//int vimBufferGetId(buf_T *buf);
func BufferGetId(vbuf *C.buf_T) int {
	id := C.vimBufferGetId(vbuf)
	return int(id)
}

//buf_T *vimBufferGetCurrent(void);
func BufferGetCurrent() *C.buf_T {
	return C.curbuf
}

//void vimInput(char_u *input);
func Input(s string) {
	C.vimInput(ucharP(s))
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
	data := (*C.char)(unsafe.Pointer(line))
	s := C.GoString(data)
	return s
}

func BufferLines(vbuf *C.buf_T) [][]byte {
	var bb [][]byte
	lc := BufferGetLineCount(vbuf)
	for i := 1; i <= lc; i++ {
		s := BufferGetLine(vbuf, i)
		bb = append(bb, []byte(s))
	}
	return bb
}

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
func BufferSetLines(vbuf *C.buf_T, start int, end int, s string, count int) {
	p1 := (*C.uchar)(C.malloc(C.sizeof_uchar * C.ulong(len(s)+1)))
	p2 := (**C.uchar)(C.malloc(C.sizeof_uint))
	p2 = &p1

	view := (*[1 << 30]C.uchar)(unsafe.Pointer(p1))[0 : len(s)+1]
	for i, x := range s {
		view[i] = C.uchar(x)
		view[len(s)] = 0 //may not be necessary
	}
	// need to move some bytes into it - how
	fmt.Printf("%p\n", p2)
	fmt.Printf("%v\n", &p1)
	C.vimBufferSetLines(vbuf, C.long(start), C.long(end), p2, C.int(count))
	//C.free(unsafe.Pointer(p2)) //panics
	C.free(unsafe.Pointer(p1))
}

//pos_T vimCursorGetPosition(void);
func CursorGetPosition() [2]int {
	p := C.vimCursorGetPosition()
	var pos [2]int
	pos[0] = int(p.lnum)
	pos[1] = int(p.col)
	return pos
}
