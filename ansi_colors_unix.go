//go:build !windows

package main

const (
	BLACK   string = "\x1b[30m"
	RED     string = "\x1b[31m"
	GREEN   string = "\x1b[32m"
	YELLOW  string = "\x1b[33m"
	BLUE    string = "\x1b[34m"
	MAGENTA string = "\x1b[35m"
	CYAN    string = "\x1b[36m"
	WHITE   string = "\x1b[37m"

	RED_BOLD     string = "\x1b[1;31m"
	GREEN_BOLD   string = "\x1b[1;32m"
	YELLOW_BOLD  string = "\x1b[1;33m"
	BLUE_BOLD    string = "\x1b[1;34m"
	MAGENTA_BOLD string = "\x1b[1;35m"
	CYAN_BOLD    string = "\x1b[1;36m"
	WHITE_BOLD   string = "\x1b[1;37m"

	RED_BG     string = "\x1b[41m"
	GREEN_BG   string = "\x1b[42m"
	YELLOW_BG  string = "\x1b[43m"
	BLUE_BG    string = "\x1b[44m"
	MAGENTA_BG string = "\x1b[45m"
	CYAN_BG    string = "\x1b[46m"
	WHITE_BG   string = "\x1b[47m"
	DEFAULT_BG string = "\x1b[49m"

	// 8bit 256 color 48;5 => background
	LIGHT_GRAY_BG string = "\x1b[48;5;242m"
	DARK_GRAY_BG  string = "\x1b[48;5;236m"
)
