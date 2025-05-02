package govim

import (
	"io/ioutil"
	"strings"
)

// GoBuffer is the Go implementation of a vim buffer
type GoBuffer struct {
	id        int
	lines     []string
	name      string
	modified  bool
	engine    *GoEngine
	lastTick  int
}

// GetID returns the buffer ID
func (b *GoBuffer) GetID() int {
	return b.id
}

// GetLine returns the line at the given line number (1-based indexing)
func (b *GoBuffer) GetLine(lnum int) string {
	if lnum < 1 || lnum > len(b.lines) {
		return ""
	}
	return b.lines[lnum-1]
}

// GetLineB returns the line as a byte slice
func (b *GoBuffer) GetLineB(lnum int) []byte {
	return []byte(b.GetLine(lnum))
}

// Lines returns all lines in the buffer
func (b *GoBuffer) Lines() []string {
	result := make([]string, len(b.lines))
	copy(result, b.lines)
	return result
}

// LinesB returns all lines as byte slices
func (b *GoBuffer) LinesB() [][]byte {
	result := make([][]byte, len(b.lines))
	for i, line := range b.lines {
		result[i] = []byte(line)
	}
	return result
}

// GetLineCount returns the number of lines in the buffer
func (b *GoBuffer) GetLineCount() int {
	return len(b.lines)
}

// SetCurrent makes this buffer the current buffer
func (b *GoBuffer) SetCurrent() {
	b.engine.currentBuffer = b
}

// IsModified returns whether the buffer has been modified
func (b *GoBuffer) IsModified() bool {
	return b.modified
}

// GetLastChangedTick returns the last changed tick
func (b *GoBuffer) GetLastChangedTick() int {
	return b.lastTick
}

// SetLines sets all lines in the buffer
func (b *GoBuffer) SetLines(start, end int, lines []string) {
	// Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			// Log the panic
			println("PANIC in GoBuffer.SetLines:", r)
			println("Buffer:", b.id, "Start:", start, "End:", end, "Lines len:", len(lines))
		}
	}()
	
	// Validate inputs to prevent panics
	if b.lines == nil {
		b.lines = []string{""}
	}
	
	if lines == nil {
		lines = []string{}
	}
	
	// Bounds check for start and end
	if start < 0 {
		start = 0
	}
	
	// If end is -1, it means replace all lines from start
	if end == -1 {
		end = len(b.lines)
	}
	
	// Ensure start <= end <= len(b.lines)
	if start > len(b.lines) {
		start = len(b.lines)
	}
	
	if end > len(b.lines) {
		end = len(b.lines)
	}
	
	if start > end {
		start = end
	}
	
	// Calculate number of lines to replace
	count := end - start
	
	// Create new lines array with replaced content
	newLines := make([]string, 0, len(b.lines) - count + len(lines))
	
	// Add lines before the replacement
	newLines = append(newLines, b.lines[:start]...)
	
	// Add the new lines
	newLines = append(newLines, lines...)
	
	// Add lines after the replacement
	if end < len(b.lines) {
		newLines = append(newLines, b.lines[end:]...)
	}
	
	// Update the buffer's lines
	b.lines = newLines
	b.markModified()
}

// Mark the buffer as modified and increment the change tick
func (b *GoBuffer) markModified() {
	b.modified = true
	b.lastTick++
}

// Load a file into the buffer
func (b *GoBuffer) loadFile(filename string) error {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	
	b.name = filename
	b.lines = strings.Split(string(content), "\n")
	b.modified = false
	return nil
}