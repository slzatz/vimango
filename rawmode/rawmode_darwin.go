//go:build darwin

package rawmode

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"golang.org/x/sys/unix"
)

func GetWindowSize() (*Winsize, error) {

	ws, err := unix.IoctlGetWinsize(unix.Stdout, unix.TIOCGWINSZ)
	if err != nil {
		return nil, fmt.Errorf("error fetching window size: %w", err)
	}
	if ws.Row == 0 || ws.Col == 0 {
		return nil, fmt.Errorf("Got a zero size column or row")
	}

	// Convert unix.Winsize to our platform-agnostic Winsize
	return &Winsize{
		Row:    ws.Row,
		Col:    ws.Col,
		Xpixel: ws.Xpixel,
		Ypixel: ws.Ypixel,
	}, nil

}

// Enable switches the console from cooked or canonical mode to raw mode.
// It returns the current terminal settings for use in restoring console
// serialized to a platform independent byte slice via gob
func Enable() ([]byte, error) {

	// On macOS, TIOCGETA is the equivalent of Linux's TCGETS
	termios, err := unix.IoctlGetTermios(unix.Stdin, unix.TIOCGETA)
	if err != nil {
		return nil, fmt.Errorf("error fetching existing console settings: %w", err)
	}

	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(termios); err != nil {
		return nil, fmt.Errorf("error serializing existing console settings: %w", err)
	}

	// turn off echo & canonical mode by using a bitwise clear operator &^
	termios.Lflag = termios.Lflag &^ (unix.ECHO | unix.ICANON | unix.ISIG | unix.IEXTEN)
	termios.Iflag = termios.Iflag &^ (unix.IXON | unix.ICRNL | unix.BRKINT | unix.INPCK | unix.ISTRIP)
	termios.Oflag = termios.Oflag &^ (unix.OPOST)
	termios.Cflag = termios.Cflag | unix.CS8

	// On macOS, TIOCSETAF is the equivalent of Linux's TCSETSF (set with flush)
	if err := unix.IoctlSetTermios(unix.Stdin, unix.TIOCSETAF, termios); err != nil {
		return buf.Bytes(), err
	}

	return buf.Bytes(), nil
}

// Restore restores the console to a previous raw setting
func Restore(original []byte) error {

	var termios unix.Termios

	if err := gob.NewDecoder(bytes.NewReader(original)).Decode(&termios); err != nil {
		return fmt.Errorf("error decoding terminal settings: %w", err)
	}

	if err := unix.IoctlSetTermios(unix.Stdin, unix.TIOCSETAF, &termios); err != nil {
		return fmt.Errorf("error restoring original console settings: %w", err)
	}
	return nil
}
