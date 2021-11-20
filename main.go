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

	pos := vim.CursorGetPosition()
	fmt.Printf("Position: line = %d, col = %d\n", pos[0]-1, pos[1])

	vim.CursorSetPosition([2]int{2, 5})

	pos = vim.CursorGetPosition()
	fmt.Printf("Position: line = %d, col = %d\n", pos[0]-1, pos[1])

	vim.Execute("e!")

	vim.Key("<esc>")
	vim.Input("gg")
	z := vim.CursorGetLine()
	fmt.Printf("line number = %d\n", z)

	vim.Input("G")
	z = vim.CursorGetLine()
	fmt.Printf("line number = %d\n", z)
	vim.Input("gginorm\x1b")
	//vim.Key("<esc>")
	//vim.Input("\x1b")
	vim.Input("3lijon\x1b")
	z = vim.CursorGetLine()
	fmt.Printf("line number = %d\n", z)
	line := vim.BufferGetLine(buf0, 3)
	fmt.Printf("%s\n", line)

	vim.BufferSetLines(buf0, 0, 0, "Hello very young lovers wherever you are!", 1)
	line = vim.BufferGetLine(buf0, 1)
	fmt.Printf("%s\n", line)
	ss = vim.BufferLinesS(buf0)
	fmt.Printf("%v\n", ss)
	pos = vim.CursorGetPosition()
	fmt.Printf("Position: line = %d, col = %d\n", pos[0]-1, pos[1])

	fmt.Println("Done")
}
