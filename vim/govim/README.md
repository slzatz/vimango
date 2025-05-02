# GoVim Implementation

This package provides a pure Go implementation of the Vim editor functionality, designed to be a drop-in replacement for the CGO-based libvim integration.

## Current Status

This is a work-in-progress implementation that aims to gradually replace the CGO-based libvim implementation with pure Go code.

## Implementation Strategy

The implementation follows these principles:

1. **API Compatibility**: Match the existing vim.go API exactly to allow seamless switching
2. **Incremental Development**: Implement features one by one, with tests for each
3. **Performance Focus**: Optimize for speed and memory usage
4. **No External Dependencies**: Use only the Go standard library

## Roadmap

- [x] Basic buffer implementation
- [x] Cursor positioning
- [ ] Input command processing
- [ ] Movement commands (h,j,k,l, etc.)
- [ ] Text editing (d, c, y, p, etc.)
- [ ] Visual mode
- [ ] Search and replace
- [ ] Ex commands

## Usage

The adapter.go file in the parent package provides a switching mechanism to toggle between the CGO and Go implementations:

```go
// Use CGO implementation (default)
vim.DisableGoImplementation()

// Use pure Go implementation
vim.EnableGoImplementation()

// Open a buffer with the active implementation
buffer := vim.BufferOpenAdapter("filename.txt", 1, 0)
```

## Testing

Each feature should include tests that verify it works exactly like the CGO implementation.

## Contributing

To contribute to this implementation:

1. Choose a Vim feature to implement
2. Study the C implementation in the libvim source
3. Create a Go implementation with the same behavior
4. Add tests to verify the behavior matches
5. Submit a PR with your implementation