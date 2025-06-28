//go:build cgo && !windows

package vim

import (
	"github.com/slzatz/vimango/vim/cvim"
	"github.com/slzatz/vimango/vim/interfaces"
)

// Backward compatibility functions for CGO implementation
// These functions allow old code to continue working with the new interface

// BufferNew creates a new buffer (old style)
func BufferNew(flags int) cvim.Buffer {
	b := Engine.BufferNew(flags)
	if wrapper, ok := b.(*CGOBufferWrapper); ok {
		return wrapper.buf
	}
	// Fallback for non-CGO implementation - this may cause issues
	return nil
}

// BufferSetCurrent sets the current buffer (old style)
func BufferSetCurrent(buf cvim.Buffer) {
	if buf == nil {
		return
	}
	wrapper := &CGOBufferWrapper{buf: buf}
	Engine.BufferSetCurrent(wrapper)
}

// BufferLines gets all lines from a buffer (old style)
func BufferLines(buf cvim.Buffer) []string {
	if buf == nil {
		return nil
	}
	wrapper := &CGOBufferWrapper{buf: buf}
	return wrapper.Lines()
}

// BufferGetLastChangedTick gets the last changed tick
func BufferGetLastChangedTick(buf cvim.Buffer) int {
	if buf == nil {
		return 0
	}
	wrapper := &CGOBufferWrapper{buf: buf}
	return wrapper.GetLastChangedTick()
}

// BufferSetLines sets lines in a buffer
func BufferSetLines(buf cvim.Buffer, start, end int, lines []string, count int) {
	if buf == nil {
		return
	}
	wrapper := &CGOBufferWrapper{buf: buf}
	wrapper.SetLines(start, end, lines)
}

// BufferToVimBuffer converts an old-style Buffer to a VimBuffer interface
func BufferToVimBuffer(buf cvim.Buffer) interfaces.VimBuffer {
	if buf == nil {
		return nil
	}
	return &CGOBufferWrapper{buf: buf}
}

// VimBufferToBuffer attempts to convert a VimBuffer to a Buffer
func VimBufferToBuffer(buf interfaces.VimBuffer) cvim.Buffer {
	if wrapper, ok := buf.(*CGOBufferWrapper); ok {
		return wrapper.buf
	}
	// For non-CGO buffers, we can't convert
	return nil
}