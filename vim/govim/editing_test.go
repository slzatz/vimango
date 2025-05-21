package govim

import (
	"testing"
)

func TestEditingOperations(t *testing.T) {
	// Create test engine with buffer
	engine := NewEngine()
	buf := &GoBuffer{
		id:     1,
		engine: engine,
		lines: []string{
			"First line of text",
			"Second line with more text",
			"Third line is here",
			"Fourth and final line",
		},
	}
	engine.currentBuffer = buf
	engine.buffers[buf.id] = buf
	
	t.Run("Delete word", func(t *testing.T) {
		// Reset content
		buf.lines = []string{
			"First line of text",
			"Second line with more text",
			"Third line is here",
			"Fourth and final line",
		}
		
		// Position cursor at start of "line" in first line
		engine.currentBuffer.cursorRow = 1
		engine.currentBuffer.cursorCol = 6
		
		// Delete word (dw)
		engine.Input("d")
		engine.Input("w")
		
		// Check result
		expected := "First of text"
		if buf.lines[0] != expected {
			t.Errorf("After dw, got %q, want %q", buf.lines[0], expected)
		}
		
		// Check cursor position
		if engine.currentBuffer.cursorCol != 6 {
			t.Errorf("After dw, cursor column should be 6, got %d", engine.currentBuffer.cursorCol)
		}
	})
	
	t.Run("Delete line", func(t *testing.T) {
		// Reset content
		buf.lines = []string{
			"First line of text",
			"Second line with more text",
			"Third line is here",
			"Fourth and final line",
		}
		
		// Position cursor in the second line
		engine.currentBuffer.cursorRow = 2
		engine.currentBuffer.cursorCol = 5
		
		// Delete line (dd)
		engine.Input("d")
		engine.Input("d")
		
		// Check result - line should be removed
		if len(buf.lines) != 3 || buf.lines[1] != "Third line is here" {
			t.Errorf("After dd, expected deletion of second line, got %v", buf.lines)
		}
		
		// Check cursor position is at the start of the next line
		if engine.currentBuffer.cursorRow != 2 || engine.currentBuffer.cursorCol != 0 {
			t.Errorf("After dd, cursor should be at [2,0], got [%d,%d]", 
				engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol)
		}
	})
	
	t.Run("Yank and put", func(t *testing.T) {
		// Reset content
		buf.lines = []string{
			"First line of text",
			"Second line with more text",
			"Third line is here",
			"Fourth and final line",
		}
		
		// Position cursor at start of "line" in third line
		engine.currentBuffer.cursorRow = 3
		engine.currentBuffer.cursorCol = 6
		
		// Yank word (yw)
		engine.Input("y")
		engine.Input("w")
		
		// Move to end of "Fourth" in fourth line
		engine.currentBuffer.cursorRow = 4
		engine.currentBuffer.cursorCol = 6
		
		// Put after cursor (p)
		engine.Input("p")
		
		// Check result
		expected := "Fourth line and final line"
		if buf.lines[3] != expected {
			t.Errorf("After yw and p, got %q, want %q", buf.lines[3], expected)
		}
	})
	
	t.Run("Change word", func(t *testing.T) {
		// Reset content
		buf.lines = []string{
			"First line of text",
			"Second line with more text",
			"Third line is here",
			"Fourth and final line",
		}
		
		// Position cursor at the start of "line" in first line
		engine.currentBuffer.cursorRow = 1
		engine.currentBuffer.cursorCol = 6
		
		// Change word (cw) then add "word"
		engine.Input("c")
		engine.Input("w")
		
		// Should now be in insert mode
		if engine.GetMode() != ModeInsert {
			t.Errorf("After cw, expected to be in insert mode, got %d", engine.GetMode())
		}
		
		// Simulate typing "word"
		engine.Input("w")
		engine.Input("o")
		engine.Input("r")
		engine.Input("d")
		
		// Return to normal mode
		engine.Input("\x1b")
		
		// Check result
		expected := "First wordof text"
		if buf.lines[0] != expected {
			t.Errorf("After cw and typing 'word', got %q, want %q", buf.lines[0], expected)
		}
	})
	
	t.Run("Motion counts", func(t *testing.T) {
		// Reset content
		buf.lines = []string{
			"First line of text",
			"Second line with more text",
			"Third line is here",
			"Fourth and final line",
		}
		
		// Position cursor at start
		engine.currentBuffer.cursorRow = 1
		engine.currentBuffer.cursorCol = 0
		
		// Move down 3 lines with count (3j)
		engine.Input("3")
		engine.Input("j")
		
		// Check position
		if engine.currentBuffer.cursorRow != 4 {
			t.Errorf("After 3j, expected to be at row 4, got %d", engine.currentBuffer.cursorRow)
		}
		
		// Move right 5 characters with count (5l)
		engine.Input("5")
		engine.Input("l")
		
		// Check position
		if engine.currentBuffer.cursorCol != 5 {
			t.Errorf("After 5l, expected to be at column 5, got %d", engine.currentBuffer.cursorCol)
		}
	})
}