package main

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/disintegration/imaging"
)

var (
	ESC_ERASE_DISPLAY = "\x1b[2J\x1b[0;0H"
	E_NON_TTY         = errors.New("NON TTY")
	E_TIMED_OUT       = errors.New("TERM RESPONSE TIMED OUT")
)

func ExtractFileID(url string) (string, error) {
	// A regular expression to find the file ID.
	// It looks for a string of letters, numbers, hyphens, and underscores
	// that is between "/d/" and the next "/".
	re := regexp.MustCompile(`/d/([a-zA-Z0-9_-]+)`)

	// Find the submatches in the URL string.
	matches := re.FindStringSubmatch(url)

	// The result of FindStringSubmatch is a slice where:
	// - matches[0] is the full text that matched the expression (e.g., "/d/1Fj-...")
	// - matches[1] is the text captured by the first group `(...)` (the file ID)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", errors.New("google drive file ID not found in URL")
}
func loadGoogleImage(path string, maxWidth, maxHeight int) (img image.Image, imgFmt string, err error) {
	fileID, err := ExtractFileID(path)
	resp, err := app.Session.googleDrive.Files.Get(fileID).Download()
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = errors.New("Received non-OK response code")
		//return fmt.Errorf("received non-OK response status: %s", resp.Status)
		return
	}

	img, imgFmt, err = image.Decode(resp.Body)
	if img.Bounds().Max.X > maxWidth || img.Bounds().Max.Y > maxHeight {
		img = imaging.Fit(img, maxWidth, maxHeight, imaging.Lanczos)
	}
	//if img.Bounds().Max.Y > app.Session.imgSizeY {
	//	img = imaging.Resize(img, 0, app.Session.imgSizeY, imaging.Lanczos)
	//}
	return
}

func loadImage(path string, maxWidth, maxHeight int) (img image.Image, imgFmt string, err error) {
	//fmt.Printf("loadImage: path=%s, maxWidth=%d, maxHeight=%d\n", path, maxWidth, maxHeight)
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	img, imgFmt, err = image.Decode(f)
	if img.Bounds().Max.X > maxWidth || img.Bounds().Max.Y > maxHeight {
		//img = imaging.Resize(img, maxWidth, 0, imaging.Lanczos)
		img = imaging.Fit(img, maxWidth, maxHeight, imaging.Lanczos)
	}
	//sess.showOrgMessage("bounds = %v", img.Bounds())
	return
}

func loadWebImage(URL string) (img image.Image, imgFmt string, err error) {
	//Get the response bytes from the url
	response, err := http.Get(URL)
	if err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		err = errors.New("Received non 200 response code")
		return
	}
	img, imgFmt, err = image.Decode(response.Body)
	if img.Bounds().Max.Y > app.Session.imgSizeY {
		img = imaging.Resize(img, 0, app.Session.imgSizeY, imaging.Lanczos)
	}
	return
}

func displayImage(img image.Image) {

	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error encoding image: %v", err)
		return
	}

	err = KittyCopyPNGInline(os.Stdout, buf, int64(buf.Len()))
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error in KittyCopyPNG...: %v", err)
	}
}

// transforms given open/close terminal escapes to pass through tmux to parent terminal
func TmuxOscOpenClose(opn, cls string) (string, string) {

	opn = "\x1bPtmux;" + strings.ReplaceAll(opn, "\x1b", "\x1b\x1b")
	cls = strings.ReplaceAll(cls, "\x1b", "\x1b\x1b") + "\x1b\\"
	return opn, cls
}

func IsTmuxScreen() bool {
	TERM := strings.ToLower(strings.TrimSpace(os.Getenv("TERM")))
	return strings.HasPrefix(TERM, "screen")
}

/*
Handles request/response terminal control sequences like <ESC>[0c

STDIN & STDOUT are parameterized for special cases.
os.Stdin & os.Stdout are usually sufficient.

`sRq` should be the request control sequence to the terminal.

NOTE: only captures up to 1KB of response

NOTE: when println debugging the response, probably want to go-escape
it, like:

	fmt.Printf("%#v\n", sRsp)

since most responses begin with <ESC>, which the terminal treats as
another control sequence rather than text to output.

func TermRequestResponse(fileIN, fileOUT *os.File, sRq string) (sRsp []byte, E error) {

	// 	defer func() {
	// 		if E != nil {
	// 			if _, file, line, ok := runtime.Caller(1); ok {
	// 				E = fmt.Errorf("%s:%d - %s", file, line, E.Error())
	// 			}
	// 		}
	// 	}()

	fdIN := int(fileIN.Fd())

	// NOTE: raw mode tip came from https://play.golang.org/p/kcMLTiDRZY
	if !term.IsTerminal(fdIN) {
		return nil, E_NON_TTY
	}

	// STDIN "RAW MODE" TO CAPTURE TERMINAL RESPONSE
	// NOTE: without this, response bypasses stdin,
	//       and is written directly to the console
	var oldState *term.State
	if oldState, E = term.MakeRaw(fdIN); E != nil {
		return
	}
	defer func() {
		// CAPTURE RESTORE ERROR (IF ANY) IF THERE HASN'T ALREADY BEEN AN ERROR
		if e2 := term.Restore(fdIN, oldState); E == nil {
			E = e2
		}
	}()

	// SEND REQUEST
	if _, E = fileOUT.Write([]byte(sRq)); E != nil {
		return
	}

	TMP := make([]byte, 1024)

	// WAIT 1/16 SECOND FOR TERM RESPONSE.  IF TIMER EXPIRES,
	// TRIGGER BYTES TO STDIN SO .Read() CAN FINISH
	tmr := time.NewTimer(time.Second >> 4)
	cDone := make(chan bool)
	WG := sync.WaitGroup{}
	WG.Add(1)
	go func() {
		select {
		case <-tmr.C:
			// "Report Cursor Position (CPR) [row; column]
			// JUST TO GET SOME BYTES TO STDIN
			// NOTE: seems to work for everything except mlterm
			fileOUT.Write([]byte("\x1b\x1b[" + "6n"))
			break
		case <-cDone:
			break
		}
		WG.Done()
	}()

	// CAPTURE RESPONSE
	nBytes, E := fileIN.Read(TMP)

	// ENSURE GOROUTINE TERMINATION
	if tmr.Stop() {
		cDone <- true
	} else {
		// fmt.Fprintf(os.Stderr, "%#v\n", string(TMP[1:nBytes]))
		E = E_TIMED_OUT
	}
	WG.Wait()

	if (nBytes > 0) && (E != E_TIMED_OUT) {
		return TMP[:nBytes], nil
	}

	return nil, E
}

/*
NOTE: the calling program MUST be connected to an actual terminal for this to work

Requests terminal attributes per:
https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h4-Functions-using-CSI-_-ordered-by-the-final-character-lparen-s-rparen:CSI-Ps-c.1CA3

	CSI Ps c  Send Device Attributes (Primary DA).
		Ps = 0  or omitted ⇒  request attributes from terminal.  The
	response depends on the decTerminalID resource setting.
		⇒  CSI ? 1 ; 2 c     ("VT100 with Advanced Video Option")
		⇒  CSI ? 1 ; 0 c     ("VT101 with No Options")
		⇒  CSI ? 4 ; 6 c     ("VT132 with Advanced Video and Graphics")
		⇒  CSI ? 6 c         ("VT102")
		⇒  CSI ? 7 c         ("VT131")
		⇒  CSI ? 1 2 ; Ps c  ("VT125")
		⇒  CSI ? 6 2 ; Ps c  ("VT220")
		⇒  CSI ? 6 3 ; Ps c  ("VT320")
		⇒  CSI ? 6 4 ; Ps c  ("VT420")

	The VT100-style response parameters do not mean anything by
	themselves.  VT220 (and higher) parameters do, telling the
	host what features the terminal supports:
		Ps = 1    ⇒  132-columns.
		Ps = 2    ⇒  Printer.
		Ps = 3    ⇒  ReGIS graphics.
		Ps = 4    ⇒  Sixel graphics.
		Ps = 6    ⇒  Selective erase.
		Ps = 8    ⇒  User-defined keys.
		Ps = 9    ⇒  National Replacement Character sets.
		Ps = 1 5  ⇒  Technical characters.
		Ps = 1 6  ⇒  Locator port.
		Ps = 1 7  ⇒  Terminal state interrogation.
		Ps = 1 8  ⇒  User windows.
		Ps = 2 1  ⇒  Horizontal scrolling.
		Ps = 2 2  ⇒  ANSI color, e.g., VT525.
		Ps = 2 8  ⇒  Rectangular editing.
		Ps = 2 9  ⇒  ANSI text locator (i.e., DEC Locator mode).
*/
/*
func RequestTermAttributes() (sAttrs []int, E error) {

	text, E := TermRequestResponse(os.Stdin, os.Stdout, "\x1b[0c")
	if E != nil {
		return
	}

	// EXTRACT CODES
	t2 := rxNumber.FindAll(text, -1)
	sAttrs = make([]int, len(t2))
	for ix, sN := range t2 {
		iN, _ := strconv.Atoi(string(sN))
		sAttrs[ix] = iN
	}

	return
}

var rxNumber = regexp.MustCompile(`\d+`)

*/
func lcaseEnv(k string) string {
	return strings.ToLower(strings.TrimSpace(os.Getenv(k)))
}

func GetEnvIdentifiers() map[string]string {

	KEYS := []string{"TERM", "TERM_PROGRAM", "LC_TERMINAL"}
	V := make(map[string]string)
	for _, K := range KEYS {
		V[K] = lcaseEnv(K)
	}

	return V
}

// NOTE: uses $TERM, which is overwritten by tmux
func IsTermKitty() bool {

	V := GetEnvIdentifiers()
	return V["TERM"] == "xterm-kitty"
}

/*
// displayPNGFromFile - not in use
func readPNGIntoBuffer(path string) (err error) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)

	return KittyCopyPNGInline(os.Stdout, reader, int64(reader.Size()))
}
*/
