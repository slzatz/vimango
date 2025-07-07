//go:build windows

package main

const (
	BLACK   string = "\x1b[38;2;60;56;54m"    // rgb(60, 56, 54) "\x1b[30m"
	RED     string = "\x1b[38;2;204;36;29m"   //rgb(204, 36, 29) "\x1b[31m"
	GREEN   string = "\x1b[38;2;152;151;26m"  //rgb(152, 151, 26) // "\x1b[32m"
	YELLOW  string = "\x1b[38;2;215;153;33m"  //rgb(215, 153, 33) 33m
	BLUE    string = "\x1b[38;2;69;133;136m"  //rgb(69, 133, 136)
	MAGENTA string = "\x1b[38;2;177;98;134m"  //rgb(177, 98, 134)
	CYAN    string = "\x1b[38;2;104;157;106m" //rgb(104, 157, 106)
	WHITE   string = "\x1b[38;2;168;153;132m" //rgb(168, 153, 132)

	RED_BOLD     string = "\x1b[1;38;2;204;36;29m"   //"\x1b[1;31m"
	GREEN_BOLD   string = "\x1b[1;38;2;152;151;26m"  //"\x1b[1;32m"
	YELLOW_BOLD  string = "\x1b[1;38;2;215;153;33m"  //"\x1b[1;33m"
	BLUE_BOLD    string = "\x1b[1;38;2;69;133;136m"  //"\x1b[1;34m"
	MAGENTA_BOLD string = "\x1b[1;38;2;177;98;134m"  //"\x1b[1;35m" color5: #b16286
	CYAN_BOLD    string = "\x1b[1;38;2;104;157;106m" //"\x1b[1;36m" color6: #689d6a
	WHITE_BOLD   string = "\x1b[1;38;2;168;153;132m" //"\x1b[1;37m"

	RED_BG     string = "\x1b[48;2;204;36;29m"    //"\x1b[41m"
	GREEN_BG   string = "\x1b[48;2;152;151;26m"   //"\x1b[42m"
	YELLOW_BG  string = "\x1b[48;2;215;153;33m"   //"\x1b[43m"
	BLUE_BG    string = "\x1b[48;2;69;133;136m"   //"\x1b[44m"
	MAGENTA_BG string = "\x1b[48;2;177;98;134m"   //"\x1b[45m"
	CYAN_BG    string = "\x1b[48;2;104;157;106m"  //"\x1b[46m"
	WHITE_BG   string = "\x1b[48;2;168;153;132m " //"\x1b[47m"
	DEFAULT_BG string = "\x1b[48;2;60;56;54m"     //"\x1b[49m"

	// 8bit 256 color 48;5 => background
	LIGHT_GRAY_BG string = "\x1b[48;2;200;200;200m" // rgb(200, 200, 200)there should probably be rgb too "\x1b[48;5;242m"
	DARK_GRAY_BG  string = "\x1b[48;2;50;50;50m"    //"\x1b[48;5;236m"
)
