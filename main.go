package main

import (
	"fmt"

	"github.com/slzatz/vimango/vim"
)

func main() {
	vim.Init(0)

	buf0 := vim.BufferOpen("testfile.txt", 1, 0)
	fmt.Printf("buffer pointer: %p -> %T\n", buf0, buf0)

	i := vim.BufferGetId(buf0)
	fmt.Printf("Current buffer id = %d\n", i)

	n := vim.BufferGetLineCount(buf0)
	fmt.Printf("Line Count = %d\n", n)

	ss := vim.BufferLinesS(buf0)
	fmt.Printf("%v\n", ss)

	m := vim.GetMode()
	fmt.Printf("NORMAL Mode = %d %X\n", m, m)
	vim.Input("i")
	m = vim.GetMode()
	fmt.Printf("INSERT Mode = %d\n", m)
	vim.Key("<esc>")
	vim.Input("v")
	m = vim.GetMode()
	fmt.Printf("Visual Mode = %d\n", m)
	t := vim.VisualGetType()
	fmt.Printf("Visual type = %d\n", t)
	vim.Key("<esc>")
	vim.Input("V")
	m = vim.GetMode()
	fmt.Printf("Visual Line = %d\n", m)
	t = vim.VisualGetType()
	fmt.Printf("Visual type = %d\n", t)
	vim.Key("<esc>")
	vim.Key("<c-v>")
	m = vim.GetMode()
	fmt.Printf("Visual Block = %d\n", m)
	t = vim.VisualGetType()
	fmt.Printf("Visual type = %d\n", t)
	vim.Key("<esc>")
	vim.Input("r")
	m = vim.GetMode()
	fmt.Printf("REPLACE = %d %X\n", m, m)
	vim.Key("<esc>")
	vim.Input("da")
	m = vim.GetMode()
	fmt.Printf("da = %d %X\n", m, m)
	vim.Key("<esc>")

	pos := vim.CursorGetPosition()
	fmt.Printf("Position: line = %d, col = %d\n", pos[0]-1, pos[1])

	vim.CursorSetPosition([2]int{2, 5})

	pos = vim.CursorGetPosition()
	fmt.Printf("Position: line = %d, col = %d\n", pos[0]-1, pos[1])

	vim.Execute("e!") // this throws away unsaved changes since the last save

	vim.Key("<esc>")
	vim.Input("gg")
	z := vim.CursorGetLine()
	fmt.Printf("line number = %d\n", z)

	vim.Input("G")
	z = vim.CursorGetLine()
	fmt.Printf("line number = %d\n", z)

	mod := vim.BufferGetModified(buf0)
	fmt.Printf("Buffer modified = %t\n", mod)
	vim.Input("gginorm [⌘]\x1b")
	mod = vim.BufferGetModified(buf0)
	fmt.Printf("Buffer modified = %t\n", mod)
	//vim.Key("<esc>")
	//vim.Input("\x1b")
	vim.Input("3lijon\x1b")
	z = vim.CursorGetLine()
	fmt.Printf("line number = %d\n", z)
	line := vim.BufferGetLine(buf0, 3)
	fmt.Printf("%s\n", line)

	vim.BufferSetLines(buf0, 0, 0, "Hello very young [\xe2\x8c\x98] [\u2318]lovers wherever you are!", 1)
	line = vim.BufferGetLine(buf0, 1)
	fmt.Printf("%s\n", line)
	bb := vim.BufferGetLine2(buf0, 1)
	fmt.Printf("% x\n", bb)
	ss = vim.BufferLinesS(buf0)
	fmt.Printf("%v\n", ss)
	pos = vim.CursorGetPosition()
	fmt.Printf("Position: line = %d, col = %d\n", pos[0]-1, pos[1])

	//visual
	vim.Input("0vlllllll")
	vpos := vim.VisualGetRange()
	fmt.Printf("Visual pos = %v\n", vpos)
	vim.Input("ll\x1b")
	vpos = vim.VisualGetRange()
	fmt.Printf("Visual pos = %v\n", vpos)
	fmt.Println("Done")
}
