package rawmode

// Winsize represents terminal window dimensions in a platform-agnostic way
type Winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}