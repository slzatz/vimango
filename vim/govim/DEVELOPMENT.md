# GoVim Development Guide

This document provides guidance for developing the pure Go implementation of vim functionality.

## Architecture Overview

The GoVim implementation follows this architecture:

1. **Engine** (`GoEngine` in engine.go)
   - Core state management
   - Mode handling
   - Command dispatch
   - Per-buffer cursor position tracking

2. **Buffer** (`GoBuffer` in buffer.go)
   - File content management
   - Line operations
   - Modification tracking
   - Cursor position storage
   - Undo/redo stacks

3. **Input Processing** (input.go)
   - Command handlers implementation
   - Key processing logic
   - Mode transitions
   - Visual mode operations
   - Text manipulation functions

4. **Interfaces** (interfaces.go)
   - Define APIs that match the C implementation
   - Allow for implementation switching

## Input Processing Design

When implementing new commands or operations:

1. **Add State Tracking if Needed**:
   - For commands that require multiple keystrokes (like 'r' followed by a character),
     add appropriate state flags in the `GoEngine` struct
   - Initialize and reset these flags properly in the constructor and ESC handler

2. **Command Implementation**:
   - Implement the command logic in the `Input` method in input.go
   - Add proper count support when applicable
   - Always save the buffer state for undo operations before modifying content
   - Reset state flags and counts after command execution

3. **Visual Mode Support**:
   - When adding a new command, consider implementing both normal mode and visual mode variants
   - For operations that can work on selections (like case changing), add support to `visualOperation`
   - Implement a dedicated function for the visual mode operation (like `changeCaseVisualSelection`)

## Visual Mode Design

When implementing or modifying visual mode functionality, follow these guidelines:

1. **Use Helper Functions**:
   - `enterVisualMode(visualType int)` - Call this to initialize visual mode with a specific type
   - `updateVisualSelection()` - Call this after any cursor movement in visual mode
   - `exitVisualMode()` - Call this when exiting visual mode
   - `visualOperation(op string)` - Use this for operations on visual selections

2. **Mode Handling**:
   - Visual mode should always properly return to the appropriate mode after operations
   - Yank operations should return to normal mode
   - Delete operations should return to normal mode
   - Change operations should enter insert mode
   - Case toggle operations should return to normal mode

3. **Visual Selection State**:
   - `visualStart` holds the start position of the selection
   - `visualEnd` holds the end position (current cursor)
   - `visualType` determines the type of visual mode (0 = char, 1 = line, 2 = block)
   - Always use `getNormalizedVisualSelection()` to get a properly ordered selection

4. **Operation Process Flow**:
   - Enter visual mode with `enterVisualMode()`
   - Update selection with cursor movement
   - Perform operations with `visualOperation()`
   - If needed, exit explicitly with `exitVisualMode()`

## Per-Buffer Cursor Position

The implementation now tracks cursor positions per buffer:

1. **Storage**:
   - Each `GoBuffer` instance stores its own cursor position (cursorRow, cursorCol)
   - This enables independent cursor position tracking for each buffer

2. **Handling on Buffer Switch**:
   - When switching buffers, save the current cursor position in the old buffer
   - Restore the saved cursor position from the new buffer
   - This preserves cursor positions when switching between contexts (e.g., note editing and organizer)

3. **Position Validation**:
   - Always validate cursor positions after restoration to ensure they're within buffer bounds
   - Use `validateCursorPosition()` to ensure consistent cursor behavior

## Command Implementation Patterns

When implementing commands, follow these patterns:

1. **Replace Character (r)**:
   - Set state flag (awaitingReplace) after 'r' command
   - Process the next keystroke as replacement character
   - Update buffer content at cursor position
   - Reset state flag after operation

2. **Toggle Case (~)**:
   - Support count prefix for multiple character changes
   - Implement character-by-character case toggling
   - Support both normal and visual mode operations
   - Adjust cursor position after operation

3. **Verb+Motion Commands**:
   - Use awaitingMotion flag for multi-key commands
   - Check for double-letter commands (like 'dd', 'yy')
   - Handle special cases (like 'cw' behaving like 'ce')
   - Maintain state between keypresses

## Development Guidelines

When implementing new features:

1. **Study the C Implementation**
   - Look at how the feature works in libvim
   - Understand edge cases and behaviors

2. **Test-Driven Development**
   - Write tests first based on expected vim behavior
   - Implement the feature to pass the tests
   - Add comparison tests with C implementation where possible

3. **Code Organization**
   - Group related functionality in focused files
   - Use clear naming conventions
   - Add comments for complex logic

4. **Performance Considerations**
   - Avoid unnecessary allocations
   - Use efficient data structures
   - Consider using byte slices for performance-critical operations

## Implementation Strategy

When implementing vim features:

1. **Adapter Layer Integration**
   - Update application to use adapter layer API
   - Ensure both C and Go implementations work with the adapter
   - Keep implementation details behind the interface

2. **Focus on Core Functionality First**
   - Basic motions and text manipulation
   - Essential ex commands
   - Simple mode transitions

3. **Add Advanced Features Later**
   - Macros, registers, marks
   - Complex text objects
   - Advanced editing operations

4. **Keep Feature Parity with C Version**
   - Ensure behaviors match exactly
   - Test edge cases thoroughly
   - Maintain API compatibility via the adapter layer

## Testing

All features should have comprehensive tests:

1. **Unit Tests**
   - Test individual commands
   - Cover edge cases
   - Verify mode transitions

2. **Integration Tests**
   - Test sequences of commands
   - Verify buffer state after operations
   - Compare with expected vim behavior

3. **Comparison Tests**
   - Create tests that run same operations on both C and Go implementations
   - Verify results match

## Debugging Tips

When debugging the implementation:

1. **Trace Mode States**
   - Log mode transitions
   - Verify cursor positions after commands
   - Check buffer modifications

2. **Command Sequence Testing**
   - Test commands in sequence to see if state is maintained properly
   - Verify buffer content matches expectations

3. **Dump State**
   - Create debug functions to dump internal state
   - Compare state with expectations
   - Identify where behaviors diverge

4. **Buffer Debugging**
   - Verify buffer content after operations
   - Check cursor position tracking between buffer switches
   - Monitor undo/redo stacks for completeness