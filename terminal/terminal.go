package terminal

import (
	"bufio"
	"fmt"
	"os"
)

// Special keys
const (
	KeyNoSpl = iota
	KeyArrowLeft = iota + 999//ARROW_LEFT //KeyArrowLeft
	KeyArrowRight //ARROW_RIGHT //KeyArrowRight
	KeyArrowUp //ARROW_UP //KeyArrowUp
	KeyArrowDown //ARROW_DOWN //KeyArrowDown
	KeyDelete //DEL_KEY //KeyDelete
	KeyHome //HOME_KEY //KeyHome
	KeyEnd //END_KEY //KeyEnd
	KeyPageUp //PAGE_UP //KeyPageUp
	KeyPageDown //PAGE_DOWN //KeyPageDown
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyIns
)

var specialKeys = map[[4]byte]int{
  [4]byte{91, 65, 0, 0}    : KeyArrowUp,  //\x1b[A
  [4]byte{91, 66, 0, 0}    : KeyArrowDown, //\x1b[B
  [4]byte{91, 68, 0, 0}	   : KeyArrowLeft,
  [4]byte{91, 67, 0, 0}    : KeyArrowRight,
  [4]byte{91, 53, 126, 0}  : KeyPageUp, //126 = ~; 91 = [; 53 = 5 /x1b[5~
  [4]byte{91, 54, 126, 0}  : KeyPageDown, // \x1b[6~
  [4]byte{91, 72, 0, 0}    : KeyHome,
  [4]byte{91, 70, 0, 0}    : KeyEnd,
  [4]byte{91, 51, 126}     : KeyDelete,
  [4]byte{79, 80, 0, 0}    : KeyF1,
  [4]byte{79, 81, 0, 0}    : KeyF2,
  [4]byte{79, 82, 0, 0}    : KeyF3,
  [4]byte{79, 83, 0, 0}    : KeyF4,
  [4]byte{91, 49, 53, 126} : KeyF5,
  [4]byte{91, 49, 55, 126} : KeyF6,
  [4]byte{91, 49, 56, 126} : KeyF7,
  [4]byte{91, 49, 57, 126} : KeyF8,
  [4]byte{91, 50, 48, 126} : KeyF9,
  [4]byte{91, 50, 126, 0}  : KeyIns,
}
// ErrNoInput indicates that there is no input when reading from keyboard
// in raw mode. This happens when timeout is set to a low number
var ErrNoInput = fmt.Errorf("no input")

// Key represents the key entered by the user
type Key struct {
	Regular rune
	Special int
}

var bufr = bufio.NewReader(os.Stdin)

// ReadKey reads a key from Stdin processing it for VT100 sequences.
// Stdin should be put in raw mode with
// VT100 processing enabled prior to using RawReadKey. If terminal read is set to
// timeout mode and no key is pressed, then ErrNoInput will be returned
func ReadKey() (Key, error) {

  //slz: not clear to me when bufr.ReadRune(0 returns error
  // since it appears to return something even when invalid
	r, n, err := bufr.ReadRune()
	if err != nil {
		return Key{}, err
	}

	// this code handles situation where a timeout has been set
	// but no key was pressed
	if n == 0 && err == nil {
		return Key{}, ErrNoInput
	}

  //ascii escape is decimal 27
	if r != 27 {
		return Key{r, KeyNoSpl}, nil
	}

	// nothing has been buffered, probably plain escape
	if bufr.Buffered() == 0 {
		return Key{27, KeyNoSpl}, nil
	}

	stack := [4]byte{}
	for j := 0; j < 4; j++ {
		b, err := bufr.ReadByte() // could these just be bufr.Readbyte
		if err != nil {
			return Key{}, err
		}
		//stack = append(stack, r)
		stack[j] = b

		//if match, key := matchSplKeys(stack); match {
		if key, found := specialKeys[stack]; found {
			return Key{0, key}, nil
		}
	}
	// we couldn't make out the special key, let's just return escape
	// this is probably wrong but unless we have a custom bufio.Reader,
	// we can't do better
	return Key{27, KeyNoSpl}, nil
}
