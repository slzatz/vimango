package govim

import (
	"testing"
)

func TestBasicMotions(t *testing.T) {
	// Create test buffer with content
	engine := NewEngine()
	buf := &GoBuffer{
		id:     1,
		engine: engine,
		lines: []string{
			"Line one",
			"Line two is longer",
			"Line three",
			"Line four",
		},
	}
	engine.currentBuffer = buf
	engine.buffers[buf.id] = buf
	
	// Set initial cursor position
	engine.currentBuffer.cursorRow = 2
	engine.currentBuffer.cursorCol = 5
	
	// Test cases for basic motions
	tests := []struct {
		name     string
		input    string
		wantRow  int
		wantCol  int
	}{
		{"Move left", "h", 2, 4},
		{"Move right", "l", 2, 6},
		{"Move up", "k", 1, 5},
		{"Move down", "j", 3, 5},
		{"Move to start of line", "0", 2, 0},
		{"Move to end of line", "$", 2, 17},
		{"Move to first non-blank", "^", 2, 0}, // Line starts with "Line" so first non-blank is at 0
		{"Move word forward", "w", 2, 9},
		{"Move word end", "e", 2, 7}, // End of "two"
		{"Move word backward", "b", 2, 0}, // Back to "Line"
		{"Move to last line", "G", 4, 5},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset cursor for each test
			engine.currentBuffer.cursorRow = 2
			engine.currentBuffer.cursorCol = 5
			
			// Execute the motion
			engine.Input(tc.input)
				// Debug printout
				t.Logf("After input %s, buffer cursor at [%d,%d]", tc.input, engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol);
			
			// Check the result
			pos := engine.CursorGetPosition()
			if pos[0] != tc.wantRow || pos[1] != tc.wantCol {
				t.Errorf("After %s, got position [%d,%d], want [%d,%d]",
					tc.input, pos[0], pos[1], tc.wantRow, tc.wantCol)
			}
		})
	}
}

func TestCombinedMotions(t *testing.T) {
	// Create test buffer with content
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
	
	// Start at the beginning
	engine.currentBuffer.cursorRow = 1
	engine.currentBuffer.cursorCol = 0
	
	// Test a sequence of movements
	movements := []struct {
		input    string
		wantRow  int
		wantCol  int
	}{
		{"w", 1, 6},      // Move to "line"
		{"w", 1, 11},     // Move to "of"
		{"j", 2, 11},     // Move down
		{"$", 2, 25},     // Move to end of line
		{"b", 2, 22},     // Move back a word
		{"k", 1, 18},     // Move up (should adjust column)
		{"0", 1, 0},      // Move to start of line
	}
	
	for i, move := range movements {
		engine.Input(move.input)
		pos := engine.CursorGetPosition()
		
		if pos[0] != move.wantRow || pos[1] != move.wantCol {
			t.Errorf("Movement %d: After %s, got position [%d,%d], want [%d,%d]",
				i, move.input, pos[0], pos[1], move.wantRow, move.wantCol)
		}
	}
}

func TestModeTransitions(t *testing.T) {
	// Create test engine with buffer
	engine := NewEngine()
	buf := &GoBuffer{
		id:     1,
		engine: engine,
		lines:  []string{"Sample text"},
	}
	engine.currentBuffer = buf
	engine.buffers[buf.id] = buf
	
	// Start in normal mode
	if engine.GetMode() != ModeNormal {
		t.Errorf("Expected to start in normal mode, got %d", engine.GetMode())
	}
	
	// Switch to insert mode
	engine.Input("i")
	if engine.GetMode() != ModeInsert {
		t.Errorf("Expected to be in insert mode after 'i', got %d", engine.GetMode())
	}
	
	// Back to normal mode with ESC
	engine.Input("\x1b")
	if engine.GetMode() != ModeNormal {
		t.Errorf("Expected to be back in normal mode after ESC, got %d", engine.GetMode())
	}
	
	// Switch to visual mode
	engine.Input("v")
	if engine.GetMode() != ModeVisual {
		t.Errorf("Expected to be in visual mode after 'v', got %d", engine.GetMode())
	}
	
	// Check visual range start
	start := engine.visualStart
	if start[0] != engine.currentBuffer.cursorRow || start[1] != engine.currentBuffer.cursorCol {
		t.Errorf("Visual start position incorrect, got [%d,%d], expected [%d,%d]",
			start[0], start[1], engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol)
	}
	
	// Move right in visual mode
	engine.Input("l")
	end := engine.visualEnd
	if end[0] != engine.currentBuffer.cursorRow || end[1] != engine.currentBuffer.cursorCol {
		t.Errorf("Visual end position incorrect, got [%d,%d], expected [%d,%d]",
			end[0], end[1], engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol)
	}
	
	// Exit visual mode
	engine.Input("\x1b")
	if engine.GetMode() != ModeNormal {
		t.Errorf("Expected to be back in normal mode after ESC, got %d", engine.GetMode())
	}
}