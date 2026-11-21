package vim

// The callback for ":s" with the c flag lives in its own file for the reason
// the others do: a file with //export may only have declarations in its cgo
// preamble. The C types are spelled out (linenr_T is long, colnr_T int,
// char_u unsigned char).

import "C"

const keyEsc = 27

// libvim calls this synchronously from inside vimInput or vimExecute, on the
// goroutine that made the call, so it needs no locking. It does not return
// until the user has answered.
//
//export goSubstituteConfirm
func goSubstituteConfirm(lnum C.long, col C.int, endLnum C.long, endCol C.int, sub *C.uchar) C.int {
	if substituteConfirm == nil {
		return keyEsc
	}
	asking.Store(true)
	defer asking.Store(false)
	return C.int(substituteConfirm(SubstituteMatch{
		Line: int(lnum), Col: int(col),
		EndLine: int(endLnum), EndCol: int(endCol),
		Replacement: goString(sub),
	}))
}
