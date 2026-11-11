// Package vim is vimango's binding to libvim: vim as a library, with no
// screen of its own. The application sends it keys and ex commands and reads
// back the buffer, cursor and mode; drawing is the application's job.
//
// This is the only layer between the application and libvim's C API. Each
// function says which C function it calls; libvim_api.h declares them. libvim is one
// process-wide vim -- one current buffer, one mode, one cursor -- and is not
// thread-safe, so everything here must be called from one goroutine (the main
// loop).
//
// The static library is linked by bare filename, so libvim.a must be in the
// directory the go command is run from: the project root. It is built from
// the slzatz/libvim fork by scripts/build-libvim.sh. libvim_api.h is this
// package's own copy of the declarations it uses, checked against the fork's
// headers by TestHeaderMatchesLibvim.
package vim

/*
#include "libvim_api.h"
#cgo linux LDFLAGS: libvim.a -lm -ltinfo -ldl -lacl
#cgo darwin LDFLAGS: libvim.a -lm -lncurses /opt/homebrew/opt/gettext/lib/libintl.a -liconv -L/opt/homebrew/lib -framework CoreServices -framework AppKit
*/
import "C"

import (
	"unsafe"
)

// ucharP returns s as a NUL-terminated char_u* for the duration of one call.
// The pointer is into Go memory, which is fine for libvim functions that copy
// or consume their argument before returning; none of the ones used here keep it.
func ucharP(s string) *C.uchar {
	bb := append([]byte(s), 0)
	return (*C.uchar)(&bb[0])
}

func goString(s *C.uchar) string {
	return C.GoString((*C.char)(unsafe.Pointer(s)))
}

// InitializeVim starts libvim. Call it once, before anything else.
//
// vim is put in restricted mode, as "vim -Z" or rvim: it starts no other
// program. That is checked where vim would start one, so it holds however the
// request arrives -- ":r !cmd", ":!cmd", a `cmd` in a file name, system(), a
// mapping, a sourced script, :execute -- and each gets vim's own error (E145).
// vimango has no use for a shell inside the note editor, and a note's text or
// a habit from vim should not be able to run one. Restricted mode also turns
// off vimscript's delete(), rename() and mkdir().
//
// And vim writes no file. A note is a row in the database, saved by vimango;
// nothing here is a file for vim to write. With file writes off (the fork's
// vimSetFileWritesEnabled) every open-for-writing inside vim fails, so this
// too holds by any route: ":w name" and ":saveas" inside :execute or a
// mapping, ":redir > name", ":mksession", writefile(), a swap file after
// ":set swapfile". The one exception is vim's own: the word list in
// 'spellfile', which zg and zw write.
//
// void vimInit(int argc, char **argv);
func InitializeVim(argc int) {
	var c *C.char
	C.vimInit(C.int(argc), &c)
	C.restricted = 1
	C.vimSetFileWritesEnabled(0)
	enableSpellSuggestCallback()
	enableMessageCallback()
	enableScreenCallbacks()
	probeListsOutput()
}

// ---- Buffers

// Buffer is a vim buffer (libvim's buf_T).
type Buffer struct {
	buf *C.buf_T
}

// NewBuffer creates an empty buffer.
//
// buf_T *vimBufferNew(int flags);
func NewBuffer(flags int) *Buffer {
	return &Buffer{C.vimBufferNew(C.int(flags))}
}

// GetCurrentBuffer returns the buffer keys and commands currently act on.
func GetCurrentBuffer() *Buffer {
	return &Buffer{C.curbuf}
}

// SetCurrentBuffer makes buf the buffer keys and commands act on.
//
// void vimBufferSetCurrent(buf_T *buf);
func SetCurrentBuffer(buf *Buffer) {
	C.vimBufferSetCurrent(buf.buf)
}

// GetID returns vim's number for the buffer.
//
// int vimBufferGetId(buf_T *buf);
func (b *Buffer) GetID() int {
	return int(C.vimBufferGetId(b.buf))
}

// Line returns line n of the buffer, 1-based.
//
// char_u *vimBufferGetLine(buf_T *buf, linenr_T lnum);
func (b *Buffer) Line(n int) string {
	return goString(C.vimBufferGetLine(b.buf, C.long(n)))
}

// Lines returns every line of the buffer.
//
// size_t vimBufferGetLineCount(buf_T *buf);
func (b *Buffer) Lines() []string {
	n := int(C.vimBufferGetLineCount(b.buf))
	lines := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		lines = append(lines, b.Line(i))
	}
	return lines
}

// SetLines replaces the lines after line start up to and including line end
// with lines; end -1 means through the last line, so SetLines(0, -1, lines)
// replaces the whole buffer and SetLines(0, 0, lines) inserts at the top.
//
// void vimBufferSetLines(buf_T *buf, linenr_T start, linenr_T end, char_u **lines, int count);
func (b *Buffer) SetLines(start, end int, lines []string) {
	count := len(lines)
	p := C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(uintptr(0))))
	defer C.free(p)
	view := unsafe.Slice((**C.uchar)(p), count)
	for i, s := range lines {
		view[i] = (*C.uchar)(unsafe.Pointer(C.CString(s)))
		defer C.free(unsafe.Pointer(view[i]))
	}
	C.vimBufferSetLines(b.buf, C.long(start), C.long(end), (**C.uchar)(p), C.int(count))
}

// GetLastChangedTick returns vim's b:changedtick: it goes up every time the
// buffer's text changes, which is how the application knows to redraw and
// whether there are unsaved changes.
//
// long vimBufferGetLastChangedTick(buf_T *buf);
func (b *Buffer) GetLastChangedTick() int {
	return int(C.vimBufferGetLastChangedTick(b.buf))
}

// ---- Input

// SendInput sends one typed character (it may be multi-byte), as if from the
// keyboard: "x", "é", ":".
//
// void vimInput(char_u *input);
func SendInput(s string) {
	C.vimInput(ucharP(s))
}

// SendMultiInput sends a string one character at a time.
func SendMultiInput(s string) {
	for _, r := range s {
		C.vimInput(ucharP(string(r)))
	}
}

// SendKey sends a key in vim's <> notation: "<esc>", "<c-u>", "<up>".
//
// void vimKey(char_u *key);
func SendKey(s string) {
	C.vimKey(ucharP(s))
}

// ExecuteCommand runs an ex command, without the leading ':'.
//
// void vimExecute(char_u *cmd);
func ExecuteCommand(s string) {
	C.vimExecute(ucharP(s))
}

// SendUserKey forwards a key the user typed with vim's mapping expansion on,
// so definitions from vimango.vim apply to it. Every other Send* call runs
// with expansion off: vimango's own key sequences (the ":" forwards, the
// cmdline's <c-e><c-u>) must mean what vim defines them to mean, whatever the
// user has mapped.
//
// libvim's vimInit leaves :map definitions stored but never expanded; the
// fork's vimSetMappingsEnabled turns expansion on and off.
//
// void vimSetMappingsEnabled(int enabled);
func SendUserKey(send func()) {
	C.vimSetMappingsEnabled(1)
	defer C.vimSetMappingsEnabled(0)
	send()
}

// InputPending reports how many bytes vim has been sent and has not acted on.
// It is non-zero when the keys so far begin a longer mapping: with
// "inoremap jk <Esc>", after "j". The next key decides -- "k" completes the
// mapping, anything else is taken after the "j" -- and so does InputTimeout.
//
// int vimInputPending(void);
func InputPending() int {
	return int(C.vimInputPending())
}

// InputTimeout is what 'timeoutlen' running out is in vim: it stops waiting
// for the rest of a half-typed mapping and takes the keys that are there, as
// they are or as the longest mapping they do complete. libvim has no clock,
// so the caller keeps the time. It belongs inside SendUserKey, like the keys
// it is about. With nothing waiting it does nothing.
//
// void vimInputTimeout(void);
func InputTimeout() {
	C.vimInputTimeout()
}

// EvaluateExpression evaluates a vimscript expression and returns its value
// as a string: EvaluateExpression("getreg('a')").
//
// char_u *vimEval(char_u *str);
func EvaluateExpression(expr string) string {
	r := C.vimEval(ucharP(expr))
	if r == nil {
		return ""
	}
	defer C.vim_free(unsafe.Pointer(r)) // vimEval returns an allocated copy
	return goString(r)
}

// ---- Cursor, mode, selection

// GetCursorPosition returns the cursor as {line, column}: the line 1-based,
// the column a 0-based byte offset into it.
//
// pos_T vimCursorGetPosition(void);
func GetCursorPosition() [2]int {
	p := C.vimCursorGetPosition()
	return [2]int{int(p.lnum), int(p.col)}
}

// SetCursorPosition moves the cursor; row is 1-based, col a 0-based byte offset.
//
// void vimCursorSetPosition(pos_T pos);
func SetCursorPosition(row, col int) {
	var p C.pos_T
	p.lnum = C.long(row)
	p.col = C.int(col)
	C.vimCursorSetPosition(p)
}

// Mode is the state vim is in, as GetCurrentMode reports it.
type Mode int

// vim's modes, with the values vim.h gives them. These are the ones a key can
// leave vim in here; vim.h has more (Replace is ModeInsert with a flag).
const (
	ModeNormal     Mode = C.NORMAL      // 1
	ModeVisual     Mode = C.VISUAL      // 2; GetVisualType says which kind
	ModeOpPending  Mode = C.OP_PENDING  // 4: an operator waiting for its motion, as after "d" or "2d"
	ModeCmdline    Mode = C.CMDLINE     // 8: typing after ':', '/' or '?'
	ModeInsert     Mode = C.INSERT      // 16
	ModeNormalBusy Mode = C.NORMAL_BUSY // 257: NORMAL with a command in progress, as after "r"
)

// GetCurrentMode returns vim's mode.
//
// int vimGetMode(void);
func GetCurrentMode() Mode {
	return Mode(C.vimGetMode())
}

// GetVisualRange returns the two ends of the visual selection, each as
// {line, column} like GetCursorPosition: where it started, and the cursor.
//
// void vimVisualGetRange(pos_T *startPos, pos_T *endPos);
func GetVisualRange() [2][2]int {
	var startPos, endPos C.pos_T
	C.vimVisualGetRange(&startPos, &endPos)
	return [2][2]int{
		{int(startPos.lnum), int(startPos.col)},
		{int(endPos.lnum), int(endPos.col)},
	}
}

// GetVisualType returns the key that started the selection: 'v' (118)
// characterwise, 'V' (86) linewise, Ctrl-V (22) blockwise.
//
// int vimVisualGetType(void);
func GetVisualType() int {
	return int(C.vimVisualGetType())
}

// GetMatchingPair returns the position of the bracket matching the one under
// the cursor, as {line, column}, or {0, 0} if there is none.
//
// pos_T *vimSearchGetMatchingPair(int initc);
func GetMatchingPair() [2]int {
	p := C.vimSearchGetMatchingPair(0)
	if p == nil {
		return [2]int{}
	}
	return [2]int{int(p.lnum), int(p.col)}
}

// ---- Command line
//
// While the user types after ':' vim is in cmdline mode and its cmdline buffer
// is the line editor; these read it back so the application can display it.

// CommandLineGetType returns ':', '/' or '?' while vim is in cmdline mode, 0 otherwise.
//
// char_u vimCommandLineGetType(void);
func CommandLineGetType() byte {
	return byte(C.vimCommandLineGetType())
}

// CommandLineGetText returns what has been typed on the command line, without
// the leading ':' or '/'; "" outside cmdline mode.
//
// char_u *vimCommandLineGetText(void);
func CommandLineGetText() string {
	t := C.vimCommandLineGetText()
	if t == nil {
		return ""
	}
	return goString(t)
}

// CommandLineGetPosition returns the cursor's byte offset within the command line.
//
// int vimCommandLineGetPosition(void);
func CommandLineGetPosition() int {
	return int(C.vimCommandLineGetPosition())
}
