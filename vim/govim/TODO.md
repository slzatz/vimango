# GoVim Implementation TODO List

This document tracks the planned work and priorities for the pure Go vim implementation.

## High Priority Tasks

1. **Adapter Layer Implementation**
   - [x] Design interface-based adapter layer
   - [x] Implement VimBuffer and VimEngine interfaces
   - [x] Create CGO and Go implementation wrappers
   - [x] Add configuration options for switching implementations
   - [x] Complete API layer (all vim functions)
   - [x] Update all application code to use new API
     - [x] Update organizer_*.go files
     - [x] Update editor_process_key.go
     - [x] Update editor_cmd_line.go
     - [x] Update editor_methods.go
     - [x] Update editor_normal.go
     - [x] Update organizer.go
     - [x] Update dbfunc.go
     - [x] Verify app.go already uses new adapter API
   - [x] Add per-buffer cursor position tracking

2. **Complete Basic Motion Set**
   - [x] Implement `e` (end of word)
   - [x] Implement `^` (first non-blank character)
   - [x] Implement `gg` (go to first line)
   - [x] Add support for motion counts (2j, 5w, etc.)
   - [x] Fix cursor positioning in normal and insert modes
   - [x] Improve arrow key handling
   - [x] Fix end-of-line cursor behavior

3. **Text Editing Commands**
   - [x] Implement delete (d + motion)
   - [x] Implement change (c + motion)
   - [x] Implement yank (y + motion)
   - [x] Implement put (p, P)
   - [x] Implement line deletion (dd)
   - [x] Implement line change (cc)
   - [x] Add support for x (delete character)
   - [x] Add support for o/O (open line below/above)
   - [x] Add support for I/A (insert at beginning/end of line)
   - [x] Implement J (join lines)
   - [x] Fix verb+motion state tracking (dw, cw, etc.)
   - [x] Implement special cases (cw behaving like ce)
   - [x] Implement r (replace character)
   - [x] Implement ~ (toggle case)
   - [x] **DELETE COMMANDS COUNT HANDLING (May 2025)**:
     - [x] Fix critical count handling issues for all d commands (3dw, 2d$, 5dd)
     - [x] Add count-aware delete functions (deleteWordWithCount, etc.)
     - [x] Fix dot command replay to properly handle counts and repetition
     - [x] Fix line merging bug where dw incorrectly merged lines
     - [x] Implement proper line boundary safety for word operations
     - [x] Add comprehensive test suite for all delete command scenarios
     - [x] Achieve feature parity with c commands for count, dot, and undo support

4. **Mode Transitions**
   - [x] Fix Escape key handling for all modes
   - [x] Improve mode transition between normal and insert modes
   - [x] Fix cursor positioning during mode transitions
   - [x] Add visual mode operations (d, y, c)
   - [x] Add visual mode indentation (< and >)
   - [x] Add visual mode case toggling (~)

5. **Search Functionality**
   - [x] Implement `/` and `?` search
   - [x] Implement `n` and `N` for next/prev match
   - [ ] Add search highlighting support

6. **Error Handling**
   - [x] Add robust error handling in buffer operations
   - [x] Improve file loading error handling
   - [x] Add recovery mechanisms for common operations

7. **Integration Testing**
   - [ ] Create comparison tests between C and Go implementations
   - [ ] Add benchmark tests to compare performance

## High Priority Features For Next Phase

1. **Registers System**
   - [ ] Implement named registers (a-z, A-Z for append)
   - [ ] Add numbered registers (0-9)
   - [ ] Add special registers (%, #, /, etc.)
   - [ ] Implement register viewing and manipulation

2. **Text Objects**
   - [ ] Implement word objects (iw, aw)
   - [ ] Add sentence objects (is, as)
   - [ ] Add paragraph objects (ip, ap)
   - [ ] Add bracket/quote objects (i", a", i), a), etc.)
   - [ ] Support operations on text objects (diw, ciw, yiw, etc.)

3. **Undo/Redo Functionality**
   - [x] Implement change tracking
   - [x] Create undo tree structure
   - [x] Add undo (u) command
   - [x] Add redo (Ctrl+R) command
   - [ ] Implement persistent undo

4. **Advanced Motions**
   - [ ] Implement character find (f, F, t, T)
   - [ ] Add sentence/paragraph motions (( and )), { and })
   - [x] Implement % for matching brackets
   - [ ] Add ge, gE (backward end of word)

5. **Complete Visual Mode**
   - [x] Add line-visual mode (Shift+V)
   - [x] Add block-visual mode (Ctrl+V)
   - [x] Implement block operations
   - [x] Refactor visual mode code for better maintainability 
   - [ ] Add support for incremental visual selection update
   - [ ] Improve visual mode feedback for terminal UI

## Medium Priority Tasks

1. **Ex Commands**
   - [ ] Implement core set of Ex commands
   - [ ] Add support for Ex command history
   - [ ] Implement command-line window (q:)

2. **Marks and Jumps**
   - [ ] Implement marks (m{a-zA-Z})
   - [ ] Add mark navigation (', `)
   - [ ] Create jump list
   - [ ] Implement Ctrl+O, Ctrl+I for jump navigation
   - [ ] Add change list and g;, g, navigation

3. **Auto-indentation**
   - [ ] Add basic auto-indent
   - [ ] Implement smart indent
   - [ ] Support indent operations (==, =motion)

4. **Macros**
   - [ ] Implement macro recording (q{register})
   - [ ] Add macro playback (@{register})
   - [ ] Support recursive macros

5. **Additional Normal Mode Commands**
   - [ ] Implement J (join lines)
   - [ ] Implement z commands (scrolling and folding)
   - [ ] Implement gU/gu (uppercase/lowercase)
   - [ ] Implement < and > (indentation)

## Lower Priority Tasks

1. **Window Management**
   - [ ] Add split windows (:split, :vsplit)
   - [ ] Implement window navigation (Ctrl+W commands)

2. **Folding**
   - [ ] Implement manual folding (zf, zo, zc)
   - [ ] Add indent-based folding

3. **Advanced Editing**
   - [ ] Add completion (Ctrl+N, Ctrl+P)
   - [ ] Implement abbreviations
   - [ ] Add increment/decrement (Ctrl+A, Ctrl+X)

4. **Performance Optimization**
   - [ ] Optimize buffer operations
   - [ ] Add benchmarks
   - [ ] Profile and improve hotspots

5. **Syntax Highlighting Integration**
   - [ ] Implement basic syntax highlighting
   - [ ] Add support for language-specific highlighting

## Known Issues

- ~~Cursor positioning at end of lines needs verification~~ (Fixed May 2025)
- ~~Visual mode selection tracking could be improved~~ (Improved May 2025)
- ~~Buffer content persistence between context switches~~ (Fixed May 2025)
- ~~Verb+motion commands not working properly~~ (Fixed May 2025)
- ~~Visual mode code organization and maintainability~~ (Refactored May 2025)
- ~~Buffer cursor position not preserved between switches~~ (Fixed May 2025)
- Tests needed for edge cases (empty buffers, special characters)
- Unexpected behavior with long lines (no text wrapping)
- Missing most text-object based operations
- Limited register functionality (only unnamed register)
- ~~No undo/redo support~~ (Implemented May 2025)

## Implementation Status Summary

The current Go vim implementation successfully handles:
- Basic cursor movement and navigation
- Basic and compound editing operations
- Verb+motion commands (dw, cw, y$, etc.)
- Mode switching with correct cursor positioning
- Visual mode with operations (y, d, c, ~)
- Fundamental search functionality
- Text yanking and pasting
- Cursor position tracking per buffer
- Replace character with 'r'
- Toggle case with '~' (both normal and visual modes)
- Undo/redo functionality

However, it still lacks several key vim features that will be the focus of the next development phase,
as detailed in the "High Priority Features For Next Phase" section above.

## Next Commands to Implement

The following commands would be good candidates for the next implementation phase:
1. Text object operations (diw, ciw, yiw, etc.)
2. Character find commands (f, F, t, T)
3. Named registers
4. Additional normal mode utility commands:
   - gU/gu (uppercase/lowercase line)
   - < and > (indentation)
   - Ctrl+A/Ctrl+X (increment/decrement numbers)

## Migration Plan

1. Implement core functionality in Go
2. Add tests to verify behavior matches C implementation
3. Create adapter for seamless switching
4. Switch components one by one to Go implementation 
5. Remove CGO dependency when all components are working