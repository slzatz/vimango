package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
)

// Kitty unicode placeholder character U+10EEEE
// Cannot use \U escape in Go (requires exactly 8 hex digits), so we use UTF-8 bytes
// U+10EEEE in UTF-8: F4 8E BB BE
var kittyPlaceholderRune = string(rune(0x10EEEE))

const (
	kittyZeroDiacritic = "\u0305" // row/col = 0
)

// kittyImageTxOptions controls a kitty graphics transmission.
type kittyImageTxOptions struct {
	ImageID          uint32
	PlacementID      uint32
	Cols             int
	Rows             int
	Quiet            bool
	VirtualPlacement bool // set U=1 to create a virtual placement during transmit
	ZIndex           int
}

var kittyImageCounter uint32 = 100 // fallback counter if cache not available

func nextKittyImageID() uint32 {
	if globalImageCache != nil {
		if id := globalImageCache.NextKittyImageID(); id != 0 {
			return id
		}
	}
	return atomic.AddUint32(&kittyImageCounter, 1)
}

// kittyTransmitImage sends PNG (or other binary) data using kitty graphics protocol.
// It supports quiet mode (q=2) and optional inline virtual placement creation (U=1).
func kittyTransmitImage(out io.Writer, data []byte, opts kittyImageTxOptions) error {
	if app == nil || !app.kitty {
		return errors.New("kitty terminal not detected")
	}

	oscOpen, oscClose := KITTY_IMG_HDR, KITTY_IMG_FTR
	if IsTmuxScreen() {
		oscOpen, oscClose = TmuxOscOpenClose(oscOpen, oscClose)
	}

	header := []string{"a=T", "f=100"}
	if opts.Quiet {
		header = append(header, "q=2")
	}
	if opts.ImageID != 0 {
		header = append(header, fmt.Sprintf("i=%d", opts.ImageID))
	}
	if opts.PlacementID != 0 {
		header = append(header, fmt.Sprintf("p=%d", opts.PlacementID))
	}
	if opts.VirtualPlacement {
		header = append(header, "U=1")
	}
	if opts.Cols > 0 {
		header = append(header, fmt.Sprintf("c=%d", opts.Cols))
	}
	if opts.Rows > 0 {
		header = append(header, fmt.Sprintf("r=%d", opts.Rows))
	}
	if opts.ZIndex != 0 {
		header = append(header, fmt.Sprintf("z=%d", opts.ZIndex))
	}
	header = append(header, fmt.Sprintf("S=%d", len(data)))
	bsHdr := []byte(strings.Join(header, ",") + ",")

	oWC := NewWriteChunker(out, 4096)
	defer oWC.Flush()

	// send chunks
	oWC.CustomWriFunc = func(iWri io.Writer, bsDat []byte) (int, error) {
		parts := [][]byte{
			[]byte(oscOpen),
			bsHdr,
			[]byte("m=1;"),
			bsDat,
			[]byte(oscClose),
		}
		bsHdr = nil // header only on first chunk
		return iWri.Write(bytes.Join(parts, nil))
	}

	enc64 := base64.NewEncoder(base64.StdEncoding, &oWC)
	if _, err := enc64.Write(data); err != nil {
		return err
	}
	if err := enc64.Close(); err != nil {
		return err
	}

	// final chunk marker
	_, err := out.Write([]byte(oscOpen + "m=0;" + oscClose))
	return err
}

// kittyCreateVirtualPlacement issues an explicit virtual placement (U=1) with no data transfer.
func kittyCreateVirtualPlacement(out io.Writer, imgID, placementID uint32, cols, rows int, quiet bool) error {
	if app == nil || !app.kitty {
		return errors.New("kitty terminal not detected")
	}
	oscOpen, oscClose := KITTY_IMG_HDR, KITTY_IMG_FTR
	if IsTmuxScreen() {
		oscOpen, oscClose = TmuxOscOpenClose(oscOpen, oscClose)
	}

	args := []string{"a=p", "U=1"}
	if quiet {
		args = append(args, "q=2")
	}
	if imgID != 0 {
		args = append(args, fmt.Sprintf("i=%d", imgID))
	}
	if placementID != 0 {
		args = append(args, fmt.Sprintf("p=%d", placementID))
	}
	if cols > 0 {
		args = append(args, fmt.Sprintf("c=%d", cols))
	}
	if rows > 0 {
		args = append(args, fmt.Sprintf("r=%d", rows))
	}

	_, err := fmt.Fprintf(out, "%s%s%s", oscOpen, strings.Join(args, ","), oscClose)
	return err
}

// kittyCreateRelativePlacement creates a placement tied to a parent placement (P/Q) with optional offsets.
func kittyCreateRelativePlacement(out io.Writer, imgID, placementID, parentImgID, parentPlacementID uint32, cols, rows int, dx, dy int) error {
	if app == nil || !app.kitty {
		return errors.New("kitty terminal not detected")
	}
	oscOpen, oscClose := KITTY_IMG_HDR, KITTY_IMG_FTR
	if IsTmuxScreen() {
		oscOpen, oscClose = TmuxOscOpenClose(oscOpen, oscClose)
	}

	args := []string{
		"a=p",
		"q=1", // show errors
		fmt.Sprintf("i=%d", imgID),
		fmt.Sprintf("p=%d", placementID),
		fmt.Sprintf("P=%d", parentImgID),
		fmt.Sprintf("Q=%d", parentPlacementID),
	}
	if cols > 0 {
		args = append(args, fmt.Sprintf("c=%d", cols))
	}
	if rows > 0 {
		args = append(args, fmt.Sprintf("r=%d", rows))
	}
	if dx != 0 {
		args = append(args, fmt.Sprintf("H=%d", dx))
	}
	if dy != 0 {
		args = append(args, fmt.Sprintf("V=%d", dy))
	}

	_, err := fmt.Fprintf(out, "%s%s%s", oscOpen, strings.Join(args, ","), oscClose)
	return err
}

// buildPlaceholderAnchor returns a 1x1 placeholder cell at row/col zero for the given ids.
// It intentionally restricts to a single cell so we don't rely on the full diacritic table yet.
func buildPlaceholderAnchor(imgID, placementID uint32) string {
	fg := fmt.Sprintf("\x1b[38;5;%dm", imgID&0xff)
	ul := ""
	if placementID != 0 {
		ul = fmt.Sprintf("\x1b[58;5;%dm", placementID&0xff)
	}
	reset := "\x1b[0m"
	return fg + ul + kittyPlaceholderRune + kittyZeroDiacritic + kittyZeroDiacritic + reset
}

// buildPlaceholderGrid emits a rows x cols block of placeholder cells with explicit row/col diacritics.
func buildPlaceholderGrid(imgID, placementID uint32, cols, rows int) string {
	var b strings.Builder
	fg := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", (imgID>>16)&0xff, (imgID>>8)&0xff, imgID&0xff)
	ul := fmt.Sprintf("\x1b[58;2;%d;%d;%dm", (placementID>>16)&0xff, (placementID>>8)&0xff, placementID&0xff)
	reset := "\x1b[0m"

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			rowDia := kittyRowColDiacritic(r)
			colDia := kittyRowColDiacritic(c)
			b.WriteString(fg)
			b.WriteString(ul)
			b.WriteString(kittyPlaceholderRune)
			b.WriteString(rowDia)
			b.WriteString(colDia)
		}
		// Add reset and newline - reset first to close color codes before newline
		if r < rows-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString(reset)
	if rows > 0 {
		b.WriteString("\n")
	}
	return b.String()
}

// kittyRowColDiacritic maps small non-negative integers to combining diacritics per kitty docs.
// Supports 0-9; values outside range clamp to 9.
func kittyRowColDiacritic(n int) string {
	if n < 0 {
		n = 0
	}
	if n >= len(diacritics) {
		n = len(diacritics) - 1
	}
	return string(diacritics[n])
}

// From kitty rowcolumn-diacritics.txt (copied to avoid extra dependency)
var diacritics = []rune{
	'\u0305',
	'\u030D',
	'\u030E',
	'\u0310',
	'\u0312',
	'\u033D',
	'\u033E',
	'\u033F',
	'\u0346',
	'\u034A',
	'\u034B',
	'\u034C',
	'\u0350',
	'\u0351',
	'\u0352',
	'\u0357',
	'\u035B',
	'\u0363',
	'\u0364',
	'\u0365',
	'\u0366',
	'\u0367',
	'\u0368',
	'\u0369',
	'\u036A',
	'\u036B',
	'\u036C',
	'\u036D',
	'\u036E',
	'\u036F',
	'\u0483',
	'\u0484',
	'\u0485',
	'\u0486',
	'\u0487',
	'\u0592',
	'\u0593',
	'\u0594',
	'\u0595',
	'\u0597',
	'\u0598',
	'\u0599',
	'\u059C',
	'\u059D',
	'\u059E',
	'\u059F',
	'\u05A0',
	'\u05A1',
	'\u05A8',
	'\u05A9',
	'\u05AB',
	'\u05AC',
	'\u05AF',
	'\u05C4',
	'\u0610',
	'\u0611',
	'\u0612',
	'\u0613',
	'\u0614',
	'\u0615',
	'\u0616',
	'\u0617',
	'\u0657',
	'\u0658',
	'\u0659',
	'\u065A',
	'\u065B',
	'\u065D',
	'\u065E',
	'\u06D6',
	'\u06D7',
	'\u06D8',
	'\u06D9',
	'\u06DA',
	'\u06DB',
	'\u06DC',
	'\u06DF',
	'\u06E0',
	'\u06E1',
	'\u06E2',
	'\u06E4',
	'\u06E7',
	'\u06E8',
	'\u06EB',
	'\u06EC',
	'\u0730',
	'\u0732',
	'\u0733',
	'\u0735',
	'\u0736',
	'\u073A',
	'\u073D',
	'\u073F',
	'\u0740',
	'\u0741',
	'\u0743',
	'\u0745',
	'\u0747',
	'\u0749',
	'\u074A',
	'\u07EB',
	'\u07EC',
	'\u07ED',
	'\u07EE',
	'\u07EF',
	'\u07F0',
	'\u07F1',
	'\u07F3',
	'\u0816',
	'\u0817',
	'\u0818',
	'\u0819',
	'\u081B',
	'\u081C',
	'\u081D',
	'\u081E',
	'\u081F',
	'\u0820',
	'\u0821',
	'\u0822',
	'\u0823',
	'\u0825',
	'\u0826',
	'\u0827',
	'\u0829',
	'\u082A',
	'\u082B',
	'\u082C',
	'\u082D',
	'\u0951',
	'\u0953',
	'\u0954',
	'\u0F82',
	'\u0F83',
	'\u0F86',
	'\u0F87',
	'\u135D',
	'\u135E',
	'\u135F',
	'\u17DD',
	'\u193A',
	'\u1A17',
	'\u1A75',
	'\u1A76',
	'\u1A77',
	'\u1A78',
	'\u1A79',
	'\u1A7A',
	'\u1A7B',
	'\u1A7C',
	'\u1B6B',
	'\u1B6D',
	'\u1B6E',
	'\u1B6F',
	'\u1B70',
	'\u1B71',
	'\u1B72',
	'\u1B73',
	'\u1CD0',
	'\u1CD1',
	'\u1CD2',
	'\u1CDA',
	'\u1CDB',
	'\u1CE0',
	'\u1DC0',
	'\u1DC1',
	'\u1DC3',
	'\u1DC4',
	'\u1DC5',
	'\u1DC6',
	'\u1DC7',
	'\u1DC8',
	'\u1DC9',
	'\u1DCB',
	'\u1DCC',
	'\u1DD1',
	'\u1DD2',
	'\u1DD3',
	'\u1DD4',
	'\u1DD5',
	'\u1DD6',
	'\u1DD7',
	'\u1DD8',
	'\u1DD9',
	'\u1DDA',
	'\u1DDB',
	'\u1DDC',
	'\u1DDD',
	'\u1DDE',
	'\u1DDF',
	'\u1DE0',
	'\u1DE1',
	'\u1DE2',
	'\u1DE3',
	'\u1DE4',
	'\u1DE5',
	'\u1DE6',
	'\u1DFE',
	'\u20D0',
	'\u20D1',
	'\u20D4',
	'\u20D5',
	'\u20D6',
	'\u20D7',
	'\u20DB',
	'\u20DC',
	'\u20E1',
	'\u20E7',
	'\u20E9',
	'\u20F0',
	'\u2CEF',
	'\u2CF0',
	'\u2CF1',
	'\u2DE0',
	'\u2DE1',
	'\u2DE2',
	'\u2DE3',
	'\u2DE4',
	'\u2DE5',
	'\u2DE6',
	'\u2DE7',
	'\u2DE8',
	'\u2DE9',
	'\u2DEA',
	'\u2DEB',
	'\u2DEC',
	'\u2DED',
	'\u2DEE',
	'\u2DEF',
	'\u2DF0',
	'\u2DF1',
	'\u2DF2',
	'\u2DF3',
	'\u2DF4',
	'\u2DF5',
	'\u2DF6',
	'\u2DF7',
	'\u2DF8',
	'\u2DF9',
	'\u2DFA',
	'\u2DFB',
	'\u2DFC',
	'\u2DFD',
	'\u2DFE',
	'\u2DFF',
	'\uA66F',
	'\uA67C',
	'\uA67D',
	'\uA6F0',
	'\uA6F1',
	'\uA8E0',
	'\uA8E1',
	'\uA8E2',
	'\uA8E3',
	'\uA8E4',
	'\uA8E5',
	'\uA8E6',
	'\uA8E7',
	'\uA8E8',
	'\uA8E9',
	'\uA8EA',
	'\uA8EB',
	'\uA8EC',
	'\uA8ED',
	'\uA8EE',
	'\uA8EF',
	'\uA8F0',
	'\uA8F1',
	'\uAAB0',
	'\uAAB2',
	'\uAAB3',
	'\uAAB7',
	'\uAAB8',
	'\uAABE',
	'\uAABF',
	'\uAAC1',
	'\uFE20',
	'\uFE21',
	'\uFE22',
	'\uFE23',
	'\uFE24',
	'\uFE25',
	'\uFE26',
	'\U00010A0F',
	'\U00010A38',
	'\U0001D185',
	'\U0001D186',
	'\U0001D187',
	'\U0001D188',
	'\U0001D189',
	'\U0001D1AA',
	'\U0001D1AB',
	'\U0001D1AC',
	'\U0001D1AD',
	'\U0001D242',
	'\U0001D243',
	'\U0001D244',
}
