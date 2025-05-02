# Adapter Layer Implementation

## Overview

The adapter layer provides a clean interface to vim functionality that can be implemented either by the C or Go backends. This enables:

1. Easier transition from CGO to pure Go implementation
2. Runtime switching between implementations
3. Cleaner application code that doesn't depend on implementation details

## Architecture

The adapter layer consists of these key components:

1. **Interfaces**:
   - `VimBuffer`: Interface for buffer operations
   - `VimEngine`: Interface for the vim engine
   - `VimImplementation`: Interface to switch implementations

2. **Implementation Wrappers**:
   - `CGOImplementation`: Wraps the C implementation
   - `GoImplementation`: Wraps the Go implementation

3. **API Layer**:
   - Provides a unified API for the application
   - Hides implementation details
   - Simplifies switching implementations

## Current Status

- [x] Interface definitions completed
- [x] Basic adapter layer implementation
- [x] Command-line flag for selecting implementation
- [x] Core wrapper functions implemented
- [x] Initial application integration
- [x] Complete application integration
  - [x] Updated organizer_*.go files to use new API
  - [x] Updated editor_process_key.go to use new API
  - [x] Updated editor_cmd_line.go to use new API
  - [x] Updated editor_methods.go to use new API
  - [x] Updated editor_normal.go to use new API
  - [x] Updated organizer.go to use new API
  - [x] Updated dbfunc.go to use new API
  - [x] Verified app.go already uses new adapter API
- [ ] Testing with Go implementation

## Usage

### Configuring the Implementation

```go
// Configure at startup - default is C implementation
vim.Configure(vim.Config{UseGoImplementation: useGoVim})

// Toggle between implementations
vim.ToggleImplementation()

// Check which implementation is active
if vim.IsUsingGoImplementation() {
    fmt.Println("Using Go implementation")
}
```

### Using the API

The API is consistent regardless of the backend implementation:

```go
// Create a buffer
buffer := vim.NewBuffer(0)

// Set it as the current buffer
vim.SetCurrentBuffer(buffer)

// Send input
vim.SendInput("Hello")
vim.SendKey("<esc>")

// Execute commands
vim.ExecuteCommand("set iskeyword+=*")

// Get cursor position
pos := vim.GetCursorPosition()
```

## Migration Plan

1. Complete the adapter layer implementation
2. Update all application code to use the new API
3. Test thoroughly with the C implementation
4. Implement any missing Go functionality
5. Test with the Go implementation
6. Eventually remove CGO dependency

## API Function Mapping

| Old Function (C) | New Function (API) |
|------------------|-------------------|
| `vim.BufferNew(0)` | `vim.NewBuffer(0)` |
| `vim.BufferSetCurrent(buf)` | `vim.SetCurrentBuffer(buf)` |
| `vim.BufferGetLastChangedTick(buf)` | `buf.GetLastChangedTick()` |
| `vim.BufferSetLines(buf, start, end, lines, count)` | `buf.SetLines(start, end, lines)` |
| `vim.BufferLines(buf)` | `buf.Lines()` |
| `vim.CursorGetPosition()` | `vim.GetCursorPosition()` |
| `vim.CursorSetPosition(row, col)` | `vim.SetCursorPosition(row, col)` |
| `vim.Input(s)` | `vim.SendInput(s)` |
| `vim.Input2(s)` | `vim.SendMultiInput(s)` |
| `vim.Key(s)` | `vim.SendKey(s)` |
| `vim.Execute(cmd)` | `vim.ExecuteCommand(cmd)` |
| `vim.GetMode()` | `vim.GetCurrentMode()` |
| `vim.VisualGetRange()` | `vim.GetVisualRange()` |
| `vim.Eval(expr)` | `vim.EvaluateExpression(expr)` |
| `vim.SearchGetMatchingPair()` | `vim.GetMatchingPair()` |