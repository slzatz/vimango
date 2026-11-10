package vim

// The message callback lives in its own file for the same reason the spell
// callback does: a file with //export may only have declarations in its cgo
// preamble, so it cannot include libvim.h. The C types are spelled out
// instead (char_u is unsigned char, msgPriority_T an int-sized enum).

import "C"

import "unsafe"

// libvim's msgPriority_T: MSG_INFO, MSG_WARNING, MSG_ERROR
const msgError = 2

var (
	capturing bool
	captured  []Message
)

// libvim calls this synchronously from inside vimInput/vimExecute, on the
// goroutine that made the call, so it needs no locking.
//
//export goMessageCallback
func goMessageCallback(title *C.uchar, msg *C.uchar, priority C.int) {
	if !capturing {
		return
	}
	m := Message{Error: priority == msgError}
	if title != nil {
		m.Title = C.GoString((*C.char)(unsafe.Pointer(title)))
	}
	if msg != nil {
		m.Text = C.GoString((*C.char)(unsafe.Pointer(msg)))
	}
	captured = append(captured, m)
}
