package govim

import (
	"testing"
)

func TestBasicSearch(t *testing.T) {
	// Create test engine with buffer
	engine := NewEngine()
	buf := &GoBuffer{
		id:     1,
		engine: engine,
		lines: []string{
			"First line with test pattern",
			"Second line",
			"Another test pattern here",
			"Final line with test",
		},
	}
	engine.currentBuffer = buf
	engine.buffers[buf.id] = buf
	
	// Start at the beginning
	engine.currentBuffer.cursorRow = 1
	engine.currentBuffer.cursorCol = 0
	
	// Test forward search (/)
	t.Run("Forward search", func(t *testing.T) {
		// Start the search
		engine.startSearch(1)
		
		// Check that we're in search mode
		if engine.GetMode() != ModeSearch || !engine.searching {
			t.Errorf("After /, expected to be in search mode, got mode=%d, searching=%v", 
				engine.GetMode(), engine.searching)
		}
		
		// Type the search pattern "test"
		engine.Input("t")
		engine.Input("e")
		engine.Input("s")
		engine.Input("t")
		
		// Check search buffer
		if engine.searchBuffer != "test" {
			t.Errorf("Expected search buffer to be 'test', got %q", engine.searchBuffer)
		}
		
		// Complete the search
		engine.Input("\r")
		
		// Check that we're back in normal mode
		if engine.GetMode() != ModeNormal {
			t.Errorf("After search, expected to be back in normal mode, got %d", engine.GetMode())
		}
		
		// Check that the cursor moved to the first match
		expectedRow, expectedCol := 1, 16
		if engine.currentBuffer.cursorRow != expectedRow || engine.currentBuffer.cursorCol != expectedCol {
			t.Errorf("After search for 'test', cursor at [%d,%d], want [%d,%d]",
				engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol, expectedRow, expectedCol)
		}
		
		// Check search results
		if len(engine.searchResults) != 3 {
			t.Errorf("Expected 3 search results for 'test', got %d", len(engine.searchResults))
		}
	})
	
	// Test next match (n)
	t.Run("Next match", func(t *testing.T) {
		// Navigate to next match
		engine.Input("n")
		
		// Check cursor position
		expectedRow, expectedCol := 3, 8
		if engine.currentBuffer.cursorRow != expectedRow || engine.currentBuffer.cursorCol != expectedCol {
			t.Errorf("After 'n', cursor at [%d,%d], want [%d,%d]",
				engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol, expectedRow, expectedCol)
		}
		
		// Go to next match (should wrap around)
		engine.Input("n")
		expectedRow, expectedCol = 4, 16
		if engine.currentBuffer.cursorRow != expectedRow || engine.currentBuffer.cursorCol != expectedCol {
			t.Errorf("After second 'n', cursor at [%d,%d], want [%d,%d]",
				engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol, expectedRow, expectedCol)
		}
		
		// Go to next match (should wrap to first match)
		engine.Input("n")
		expectedRow, expectedCol = 1, 16
		if engine.currentBuffer.cursorRow != expectedRow || engine.currentBuffer.cursorCol != expectedCol {
			t.Errorf("After third 'n' (wraparound), cursor at [%d,%d], want [%d,%d]",
				engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol, expectedRow, expectedCol)
		}
	})
	
	// Test previous match (N)
	t.Run("Previous match", func(t *testing.T) {
		// Navigate to previous match (should wrap to last match)
		engine.Input("N")
		
		// Check cursor position
		expectedRow, expectedCol := 4, 16
		if engine.currentBuffer.cursorRow != expectedRow || engine.currentBuffer.cursorCol != expectedCol {
			t.Errorf("After 'N', cursor at [%d,%d], want [%d,%d]",
				engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol, expectedRow, expectedCol)
		}
	})
}

func TestBackwardSearch(t *testing.T) {
	// Create test engine with buffer
	engine := NewEngine()
	buf := &GoBuffer{
		id:     1,
		engine: engine,
		lines: []string{
			"Line with example text",
			"Another example line",
			"The third example is here",
			"Final line with example",
		},
	}
	engine.currentBuffer = buf
	engine.buffers[buf.id] = buf
	
	// Start at the end
	engine.currentBuffer.cursorRow = 4
	engine.currentBuffer.cursorCol = 0
	
	// Test backward search (?)
	t.Run("Backward search", func(t *testing.T) {
		// Start the search
		engine.startSearch(-1)
		
		// Check that we're in search mode
		if engine.GetMode() != ModeSearch || !engine.searching {
			t.Errorf("After ?, expected to be in search mode, got mode=%d, searching=%v", 
				engine.GetMode(), engine.searching)
		}
		
		// Type the search pattern "example"
		engine.Input("e")
		engine.Input("x")
		engine.Input("a")
		engine.Input("m")
		engine.Input("p")
		engine.Input("l")
		engine.Input("e")
		
		// Check search buffer
		if engine.searchBuffer != "example" {
			t.Errorf("Expected search buffer to be 'example', got %q", engine.searchBuffer)
		}
		
		// Complete the search
		engine.Input("\r")
		
		// Check that we're back in normal mode
		if engine.GetMode() != ModeNormal {
			t.Errorf("After search, expected to be back in normal mode, got %d", engine.GetMode())
		}
		
		// Verify we found at least one match
		if len(engine.searchResults) == 0 {
			t.Errorf("Expected to find matches for 'example', but found none")
		} else {
			// The current position should be at a match
			matchFound := false
			for _, match := range engine.searchResults {
				if match[0] == engine.currentBuffer.cursorRow && match[1] == engine.currentBuffer.cursorCol {
					matchFound = true
					break
				}
			}
			
			if !matchFound {
				t.Errorf("After search, cursor [%d,%d] is not at any match position",
					engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol)
			}
		}
		
		// Store current position for next tests
		currentRow := engine.currentBuffer.cursorRow
		currentCol := engine.currentBuffer.cursorCol
		
		// Check search results count
		if len(engine.searchResults) != 4 {
			t.Errorf("Expected 4 search results for 'example', got %d", len(engine.searchResults))
		}
		
		// Reset cursor for next tests
		engine.currentBuffer.cursorRow = currentRow
		engine.currentBuffer.cursorCol = currentCol
	})
	
	// Test next match (n)
	t.Run("Next match in search", func(t *testing.T) {
		// Store current position
		oldRow := engine.currentBuffer.cursorRow
		oldCol := engine.currentBuffer.cursorCol
		
		// Navigate to next match
		engine.Input("n")
		
		// We should move to a different match position
		if engine.currentBuffer.cursorRow == oldRow && engine.currentBuffer.cursorCol == oldCol {
			t.Errorf("After 'n', cursor didn't move, still at [%d,%d]",
				engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol)
		}
		
		// The new position should be a valid match
		matchFound := false
		for _, match := range engine.searchResults {
			if match[0] == engine.currentBuffer.cursorRow && match[1] == engine.currentBuffer.cursorCol {
				matchFound = true
				break
			}
		}
		
		if !matchFound {
			t.Errorf("After 'n', cursor [%d,%d] is not at any match position",
				engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol)
		}
	})
	
	// Test previous match (N)
	t.Run("Previous match in search", func(t *testing.T) {
		// Store current position
		oldRow := engine.currentBuffer.cursorRow
		oldCol := engine.currentBuffer.cursorCol
		
		// Navigate to previous match
		engine.Input("N")
		
		// We should move to a different match position
		if engine.currentBuffer.cursorRow == oldRow && engine.currentBuffer.cursorCol == oldCol {
			t.Errorf("After 'N', cursor didn't move, still at [%d,%d]",
				engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol)
		}
		
		// The new position should be a valid match
		matchFound := false
		for _, match := range engine.searchResults {
			if match[0] == engine.currentBuffer.cursorRow && match[1] == engine.currentBuffer.cursorCol {
				matchFound = true
				break
			}
		}
		
		if !matchFound {
			t.Errorf("After 'N', cursor [%d,%d] is not at any match position",
				engine.currentBuffer.cursorRow, engine.currentBuffer.cursorCol)
		}
	})
}