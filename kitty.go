package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const (
	KITTY_IMG_HDR = "\x1b_G"
	KITTY_IMG_FTR = "\x1b\\"
)

// Encode raw PNG data into Kitty terminal format
func KittyCopyPNGInline(out io.Writer, in io.Reader, nLen int64) (E error) {

	if app.kitty == false {
		return errors.New("This is not a kitty terminal")
	}

	OSC_OPEN, OSC_CLOSE := KITTY_IMG_HDR, KITTY_IMG_FTR

	// LAST CHUNK SIGNAL `m=0` TO KITTY
	defer func() {

		if E == nil {
			out.Write([]byte(OSC_OPEN))
			out.Write([]byte("m=0;"))
			_, E = out.Write([]byte(OSC_CLOSE))
		}
	}()

	// PIPELINE: PNG -> B64 -> CHUNKER -> out io.Writer
	// SEND IN 4K CHUNKS
	oWC := NewWriteChunker(out, 4096)
	defer oWC.Flush()
	bsHdr := []byte(fmt.Sprintf("a=T,f=100,z=-1,S=%d,", nLen))
	//bsHdr := []byte(fmt.Sprintf("a=T,f=100,z=-1,w=300,h=200,x=300,y=200,S=%d,", nLen))
	oWC.CustomWriFunc = func(iWri io.Writer, bsDat []byte) (int, error) {

		parts := [][]byte{
			[]byte(OSC_OPEN),
			bsHdr,
			[]byte("m=1;"),
			bsDat,
			[]byte(OSC_CLOSE),
		}

		bsHdr = nil

		return iWri.Write(bytes.Join(parts, nil))
	}

	enc64 := base64.NewEncoder(base64.StdEncoding, &oWC)
	defer enc64.Close()

	_, E = io.Copy(enc64, in)
	return
}
