# GoVim Development Guide

This document provides guidance for developing the pure Go implementation of vim functionality.

## Architecture Overview

The GoVim implementation follows this architecture:

1. **Engine** (`GoEngine` in engine.go)
   - Core state management
   - Mode handling
   - Command dispatch

2. **Buffer** (`GoBuffer` in buffer.go)
   - File content management
   - Line operations
   - Modification tracking

3. **Command Handlers** (motion commands, etc.)
   - Each command is implemented as a function
   - Commands are registered in handler maps
   - Mode-specific command handling

4. **Interfaces** (interfaces.go)
   - Define APIs that match the C implementation
   - Allow for implementation switching

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