package interfaces

import "github.com/slzatz/vimango/vim/cvim"

// VimBuffer mirros the buf_T functionality from C
type VimBuffer interface {
	GetID() int
	Lines() []string
	GetLastChangedTick() int
	SetLines(start, end int, lines []string)
	// SpellErrors returns the misspellings in lines start..end (1-based,
	// inclusive); empty unless the buffer is current and 'spell' is set
	SpellErrors(start, end int) []cvim.SpellError
}

// VimEngine represents the interface for the vim engine
type VimEngine interface {
	Init(argc int)
	BufferNew(flags int) VimBuffer
	BufferGetCurrent() VimBuffer
	BufferSetCurrent(buf VimBuffer)

	CursorGetPosition() [2]int
	CursorSetPosition(row, col int)

	// Input handling
	Input(s string)
	Input2(s string)                 // For multi-character input
	Key(s string)                    // For special keys
	Execute(s string)                // For ex commands
	SetMappingsEnabled(enabled bool) // expand :map definitions for Input/Key

	// Mode operations
	GetMode() int
	GetSubMode() cvim.SubMode

	// Visual mode
	VisualGetRange() [2][2]int
	VisualGetType() int

	// Misc
	Eval(expr string) string
	SearchGetMatchingPair() [2]int

	// Spell checking
	SpellGetSuggestions() (word string, suggestions []string, ok bool)
	TakeSpellSuggestRequest() bool // z= typed since the last call
}
