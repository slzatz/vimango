package clipboard

/*
#cgo LDFLAGS: -framework AppKit
#include <stdlib.h>

char *vimangoPasteboardText(const char *name);
int vimangoPasteboardSetText(const char *name, const char *text);
void vimangoPasteboardRelease(const char *name);
*/
import "C"

import (
	"strings"
	"unsafe"
)

// System is the clipboard, or nil on a platform where vimango has none.
func System() *Board {
	return &Board{}
}

// Text is the text on the clipboard, and whether there is any: false when it
// is empty or holds something that is not text (an image, a file).
func (b *Board) Text() (string, bool) {
	name := b.cName()
	defer C.free(unsafe.Pointer(name))
	text := C.vimangoPasteboardText(name)
	if text == nil {
		return "", false
	}
	defer C.free(unsafe.Pointer(text))
	s := C.GoString(text)
	return s, s != ""
}

// SetText puts text on the clipboard in place of what is there, and reports
// whether it did. It can fail where there is no login session to hold a
// clipboard (a Mac reached over ssh).
func (b *Board) SetText(text string) bool {
	// the pasteboard takes a C string of UTF-8
	text = strings.ToValidUTF8(strings.ReplaceAll(text, "\x00", ""), "�")
	name, ctext := b.cName(), C.CString(text)
	defer C.free(unsafe.Pointer(name))
	defer C.free(unsafe.Pointer(ctext))
	return C.vimangoPasteboardSetText(name, ctext) != 0
}

func (b *Board) cName() *C.char {
	return C.CString(b.name)
}

// named is a pasteboard of the given name, for tests: the system's is the
// user's, and a test has no business writing to it. release gives it back.
func named(name string) *Board {
	return &Board{name: name}
}

func (b *Board) release() {
	name := b.cName()
	defer C.free(unsafe.Pointer(name))
	C.vimangoPasteboardRelease(name)
}
