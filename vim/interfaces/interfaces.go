package interfaces

import "github.com/slzatz/vimango/vim/cvim"

// VimBuffer mirros the buf_T functionality from C
type VimBuffer interface {
	GetID() int
	GetLine(lnum int) string
	GetLineB(lnum int) []byte
	GetLineCount() int
	Lines() []string
	LinesB() [][]byte
	SetCurrent()
	IsModified() bool
	GetLastChangedTick() int
	SetLines(start, end int, lines []string)
}

// VimEngine represents the interface for the vim engine
type VimEngine interface {
	Init(argc int)
	BufferOpen(filename string, lnum int, flags int) VimBuffer
	BufferNew(flags int) VimBuffer
	BufferGetCurrent() VimBuffer
	BufferSetCurrent(buf VimBuffer)

	CursorGetLine() int
	CursorGetPosition() [2]int
	CursorSetPosition(row, col int)

	// Input handling
	Input(s string)
	Input2(s string)  // For multi-character input
	Key(s string)     // For special keys
	Execute(s string) // For ex commands

	// Mode operations
	GetMode() int
	GetCurrentMode() int // A special version for application compatibility
	GetSubMode() cvim.SubMode

	// Visual mode
	VisualGetRange() [2][2]int
	VisualGetType() int

	// Misc
	Eval(expr string) string
	SearchGetMatchingPair() [2]int
}

// VimImplementation allows switching between C and Go implementations
type VimImplementation interface {
	GetEngine() VimEngine
	GetName() string
}
