package vim

// VimBuffer represents the interface for vim buffer operations
type VimBuffer interface {
	// Core buffer methods
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
	// Initialization
	Init(argc int)
	
	// Buffer operations
	BufferOpen(filename string, lnum int, flags int) VimBuffer
	BufferNew(flags int) VimBuffer
	BufferGetCurrent() VimBuffer
	BufferSetCurrent(buf VimBuffer)
	
	// Cursor operations
	CursorGetLine() int
	CursorGetPosition() [2]int
	CursorSetPosition(row, col int)
	
	// Input handling
	Input(s string)
	Input2(s string) // For multi-character input
	Key(s string)    // For special keys
	Execute(s string) // For ex commands
	
	// Mode operations
	GetMode() int
	
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