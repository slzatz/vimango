package main

import (
	"fmt"

	"github.com/slzatz/vimango/vim"
)

func main() {
	vim.VimInit(0)

	buf0 := vim.VimBufferOpen("testfile.txt", 1, 0)
	fmt.Printf("buffer pointer: %p -> %T\n", buf0, buf0)

	i := vim.VimBufferGetId(buf0)
	fmt.Printf("Current buffer id = %d\n", i)

	n := vim.VimBufferGetLineCount(buf0)
	fmt.Printf("Line Count = %d\n", n)

	ss := vim.VimBufferLinesS(buf0)
	fmt.Printf("%v\n", ss)

	pos := vim.VimCursorGetPosition()
	fmt.Printf("Position: line = %d, col = %d\n", pos[0]-1, pos[1])

	vim.VimExecute("e!")

	/*
		var x = []byte("g")
		x = append(x, 0)
		C.vimInput((*C.uchar)(&x[0]))
	*/

	vim.VimKey("<esc>")
	vim.VimInput("g")
	vim.VimInput("g")
	z := vim.VimCursorGetLine()
	fmt.Printf("line number = %d\n", z)

	vim.VimInput("G")
	z = vim.VimCursorGetLine()
	fmt.Printf("line number = %d\n", z)
	line := vim.VimBufferGetLine(buf0, 3)
	fmt.Printf("%s\n", line)

	vim.VimBufferSetLines(buf0, 0, 0, "Hello very young lovers wherever you are!", 1)
	line = vim.VimBufferGetLine(buf0, 1)
	fmt.Printf("%s\n", line)
	ss = vim.VimBufferLinesS(buf0)
	fmt.Printf("%v\n", ss)
	pos = vim.VimCursorGetPosition()
	fmt.Printf("Position: line = %d, col = %d\n", pos[0]-1, pos[1])

	fmt.Println("Done")
}

/*
//buf_T *vimBufferOpen(char_u *ffname_arg, linenr_T lnum, int flags);
func vimBufferOpen(filename string, lnum int, flags int) *C.buf_T {
	vbuf := C.vimBufferOpen(ucharP(filename), C.long(lnum), C.int(flags))
	return vbuf
}

//int vimBufferGetId(buf_T *buf);
func vimBufferGetId(vbuf *C.buf_T) int {
	id := C.vimBufferGetId(vbuf)
	return int(id)
}

func vimInput(s string) {
	C.vimInput(ucharP(s))
}

func vimKey(s string) {
	C.vimKey(ucharP(s))
}

//char_u *line = vimBufferGetLine(curbuf, vimCursorGetLine());
//char_u *vimBufferGetLine(buf_T *buf, linenr_T lnum);
//typedef long linenr_T;
//buf_T -> file_buffer is a complicated struct
func vimBufferGetLine(vbuf *C.buf_T, lineNum int) string {
	line := C.vimBufferGetLine(vbuf, C.long(lineNum))
	data := (*C.char)(unsafe.Pointer(line))
	s := C.GoString(data)
	return s
}

func vimBufferLines(vbuf *C.buf_T) [][]byte {
	var bb [][]byte
	lc := vimBufferGetLineCount(vbuf)
	for i := 1; i <= lc; i++ {
		s := vimBufferGetLine(vbuf, i)
		bb = append(bb, []byte(s))
	}
	return bb
}

func vimBufferLinesS(vbuf *C.buf_T) []string {
	// should probably use string builder
	// line count starts from 1
	var ss []string
	lc := vimBufferGetLineCount(vbuf)
	for i := 1; i <= lc; i++ {
		s := vimBufferGetLine(vbuf, i)
		ss = append(ss, s)
	}
	return ss
}

func vimCursorGetLine() int {
	lineNum := C.vimCursorGetLine()
	return int(lineNum)
}

func ucharP(s string) *C.uchar {
	var x = []byte(s)
	x = append(x, 0)
	return (*C.uchar)(&x[0])
}

func vimBufferGetLineCount(vbuf *C.buf_T) int {
	lc := C.vimBufferGetLineCount(vbuf)
	return int(lc)
}

func vimExecute(s string) {
	C.vimExecute(ucharP(s))
}

//void vimBufferSetLines(buf_T *buf, linenr_T start, linenr_T end, char_u **lines, int count);
func vimBufferSetLines(vbuf *C.buf_T, start int, end int, s string, count int) {
	//p1 := (*C.uchar)(C.malloc(C.sizeof_uint * C.ulong(len(s)+1)))
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
func vimCursorGetPosition() [2]int {
	p := C.vimCursorGetPosition()
	var pos [2]int
	pos[0] = int(p.lnum)
	pos[1] = int(p.col)
	return pos
}
*/
