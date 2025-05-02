package govim

import (
	// No imports needed currently
)

// This file provides compatibility wrappers to match the C API

// --------------------------------
// Buffer Methods
// --------------------------------

// BufferOpen opens a file and returns the buffer
func BufferOpen(filename string, lnum int, flags int) Buffer {
	return defaultEngine.BufferOpen(filename, lnum, flags)
}

// BufferNew creates a new empty buffer
func BufferNew(flags int) Buffer {
	return defaultEngine.BufferNew(flags)
}

// BufferGetId gets the buffer ID
func BufferGetId(buf Buffer) int {
	return buf.GetID()
}

// BufferSetCurrent sets the current buffer
func BufferSetCurrent(buf Buffer) {
	buf.SetCurrent()
}

// BufferGetCurrent gets the current buffer
func BufferGetCurrent() Buffer {
	return defaultEngine.BufferGetCurrent()
}

// BufferGetLine gets a line from the buffer
func BufferGetLine(buf Buffer, lineNum int) string {
	return buf.GetLine(lineNum)
}

// BufferGetLineB gets a line as bytes from the buffer
func BufferGetLineB(buf Buffer, lineNum int) []byte {
	return buf.GetLineB(lineNum)
}

// BufferLines gets all lines from the buffer
func BufferLines(buf Buffer) []string {
	return buf.Lines()
}

// BufferLinesB gets all lines as bytes from the buffer
func BufferLinesB(buf Buffer) [][]byte {
	return buf.LinesB()
}

// BufferGetLineCount gets the number of lines in the buffer
func BufferGetLineCount(buf Buffer) int {
	return buf.GetLineCount()
}

// BufferGetModified checks if the buffer has been modified
func BufferGetModified(buf Buffer) bool {
	return buf.IsModified()
}

// BufferGetLastChangedTick gets the last changed tick
func BufferGetLastChangedTick(buf Buffer) int {
	return buf.GetLastChangedTick()
}

// --------------------------------
// Cursor Methods
// --------------------------------

// CursorGetLine gets the current cursor line
func CursorGetLine() int {
	return defaultEngine.CursorGetLine()
}

// CursorGetPosition gets the cursor position
func CursorGetPosition() [2]int {
	return defaultEngine.CursorGetPosition()
}

// CursorSetPosition sets the cursor position
func CursorSetPosition(row, col int) {
	defaultEngine.CursorSetPosition(row, col)
}

// CursorSetPosition_old for compatibility
func CursorSetPosition_old(pos [2]int) {
	defaultEngine.CursorSetPosition(pos[0], pos[1])
}

// --------------------------------
// Input Methods
// --------------------------------

// Input sends input to vim
func Input(s string) {
	defaultEngine.Input(s)
}

// Input2 sends multiple character input
func Input2(s string) {
	for _, x := range s {
		defaultEngine.Input(string(x))
	}
}

// Key sends a key with terminal codes replaced
func Key(s string) {
	defaultEngine.Key(s)
}

// Execute runs a vim command
func Execute(s string) {
	defaultEngine.Execute(s)
}

// --------------------------------
// Mode and Visual Methods
// --------------------------------

// GetMode gets the current mode
func GetMode() int {
	return defaultEngine.GetMode()
}

// VisualGetRange gets the visual selection range
func VisualGetRange() [2][2]int {
	return defaultEngine.VisualGetRange()
}

// VisualGetType gets the visual mode type
func VisualGetType() int {
	return defaultEngine.VisualGetType()
}

// --------------------------------
// Search Methods
// --------------------------------

// GetSearchState returns the current search state
func GetSearchState() (bool, string, int, int) {
	return defaultEngine.GetSearchState()
}

// GetSearchResults returns all matches for the current search
func GetSearchResults() [][2]int {
	return defaultEngine.GetSearchResults()
}

// --------------------------------
// Misc Methods
// --------------------------------

// Eval evaluates a vim expression
func Eval(s string) string {
	return defaultEngine.Eval(s)
}

// SearchGetMatchingPair finds matching brackets
func SearchGetMatchingPair() [2]int {
	return defaultEngine.SearchGetMatchingPair()
}

// Note: These functions were duplicates and have been removed to avoid redeclaration errors

// --------------------------------
// Engine Instance
// --------------------------------

// defaultEngine is the singleton engine
var defaultEngine = NewEngine()

// Init initializes the vim engine
func Init(argc int) {
	defaultEngine.Init(argc)
}