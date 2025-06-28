package govim

// Buffer interface mirrors the buf_T functionality from C
type Buffer interface {
	GetID() int
	GetLine(lnum int) string
	GetLineB(lnum int) []byte
	Lines() []string
	LinesB() [][]byte
	GetLineCount() int
	SetCurrent()
	IsModified() bool
	GetLastChangedTick() int
	SetLines(start, end int, lines []string) // Added to match VimBuffer interface
}

// Engine defines the full vim functionality interface
type Engine interface {
	Init(argc int)
	BufferOpen(filename string, lnum int, flags int) Buffer
	BufferNew(flags int) Buffer
	BufferGetCurrent() Buffer
	BufferSetCurrent(buf Buffer)

	CursorGetLine() int
	CursorGetPosition() [2]int
	CursorSetPosition(row, col int)

	Input(s string)
	Key(s string)
	Execute(cmd string)

	GetMode() int
	VisualGetRange() [2][2]int
	VisualGetType() int

	Eval(expr string) string
	SearchGetMatchingPair() [2]int
}
