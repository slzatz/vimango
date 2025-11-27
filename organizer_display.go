package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	net_url "net/url"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
)

const (
	// PREVIEW_RIGHT_PADDING is the number of columns to reserve as padding
	// on the right side of the markdown preview pane (for text and images)
	PREVIEW_RIGHT_PADDING = 5
)

var (
	timeKeywordsRegex = regexp.MustCompile(`seconds|minutes|hours|days`)
	emptyImageMarker  = strings.Repeat(" ", IMAGE_MARKER_WIDTH)
	filledImageMarker = padImageMarker(IMAGE_MARKER_SYMBOL)

	// In-memory map of imageID -> dimensions for current render
	// Cleared at start of each renderMarkdown() call
	currentRenderImageDims = make(map[uint32]struct{ cols, rows int })
	currentRenderImageMux  sync.RWMutex
	// Track which image IDs have been transmitted in this process (kitty session)
	kittySessionImageMux   sync.RWMutex
	kittySessionImages     = make(map[uint32]kittySessionEntry) // imageID -> metadata
	seededKittyCache       bool
	trustKittyCache        bool
	kittyPurgeBeforeRender bool
	kittyClearedOnStart    bool
	kittyIDMap                    = make(map[string]uint32) // url -> small kitty ID
	kittyIDReverse                = make(map[uint32]string)
	kittyIDNext            uint32 = 1
	kittyImagesSent        uint64
	kittyBytesSent         uint64
)

type kittySessionEntry struct {
	fingerprint string
	confirmed   bool // true if this process sent the image data
}

func kittyQFlag() string {
	if os.Getenv("VIMANGO_KITTY_VERBOSE") != "" {
		return "1"
	}
	return "2"
}

func currentKittyWindowID() string {
	return os.Getenv("KITTY_WINDOW_ID")
}

const (
	// Transparent placeholder image ID (constant, reused for all images)
	KITTY_PLACEHOLDER_IMAGE_ID     = 1
	KITTY_PLACEHOLDER_PLACEMENT_ID = 1
)

type pendingRelativePlacement struct {
	imageID     uint32
	placementID uint32
	cols        int
	rows        int
}

// preparedImage holds the heavy work (load/decode/encode) for a markdown image.
type preparedImage struct {
	url               string
	data              []byte
	imgW              int
	imgH              int
	isGoogleDrive     bool
	cachedImageID     uint32
	cachedFingerprint string
	err               error
	source            string // "reuse-kitty" | "disk-cache" | "fresh-load"
}

var (
	// Track if transparent placeholder has been transmitted
	transparentPlaceholderTransmitted = false

	// Counter for relative placement IDs
	nextRelativePlacementID uint32 = 2 // Start at 2 (1 is reserved for transparent placeholder)

	// Pending relative placements to create after rendering
	pendingRelativePlacements []pendingRelativePlacement
	pendingPlacementsMux      sync.Mutex

	// Ordered list of image IDs for this render (to give deterministic lookups)
	currentRenderImageOrder []uint32
	currentRenderOrderIdx   int

	// Regex to extract markdown images
	markdownImageRegex = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

	// Diacritic table for encoding row/column positions in kitty placeholders
	kittyDiacritics = []rune{
		'\u0305', '\u030D', '\u030E', '\u0310', '\u0312', '\u033D', '\u033E', '\u033F',
		'\u0346', '\u034A', '\u034B', '\u034C', '\u0350', '\u0351', '\u0352', '\u0357',
		'\u035B', '\u0363', '\u0364', '\u0365', '\u0366', '\u0367', '\u0368', '\u0369',
		'\u036A', '\u036B', '\u036C', '\u036D', '\u036E', '\u036F', '\u0483', '\u0484',
	}
)

func (o *Organizer) sortColumnStart() int {
	return o.Screen.divider - TIME_COL_WIDTH + 2
}

func (o *Organizer) markerColumnStart() int {
	sortX := o.sortColumnStart()
	markerX := sortX - IMAGE_MARKER_WIDTH - IMAGE_MARKER_AGE_GAP
	if markerX <= LEFT_MARGIN+1 {
		return LEFT_MARGIN + 2
	}
	return markerX
}

func (o *Organizer) titleColumnWidth() int {
	markerX := o.markerColumnStart()
	width := markerX - LEFT_MARGIN - 1
	if width < 0 {
		return 0
	}
	return width
}

// should probably be named drawOrgRows
func (o *Organizer) refreshScreen() {
	var ab strings.Builder
	leftClearWidth := o.Screen.divider - LEFT_MARGIN - 1
	if leftClearWidth < 0 {
		leftClearWidth = 0
	}
	leftBlank := strings.Repeat(" ", leftClearWidth)

	ab.WriteString("\x1b[?25l") //hides the cursor

	//Below erase screen from middle to left - `1K` below is cursor to left erasing
	//Now erases time/sort column (+ 17 in line below)
	for j := TOP_MARGIN; j < o.Screen.textLines+1; j++ {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", j+TOP_MARGIN, LEFT_MARGIN+1)
		ab.WriteString(leftBlank)
	}
	//	}
	// put cursor at upper left after erasing
	ab.WriteString(fmt.Sprintf("\x1b[%d;%dH", TOP_MARGIN+1, LEFT_MARGIN+1))
	fmt.Print(ab.String())
	if o.taskview == BY_FIND {
		o.drawSearchRows()
		//o.drawActive() //////////////////////////
	} else {
		o.drawRows()
	}
}

func (o *Organizer) drawActiveRow(ab *strings.Builder) {
	// When drawing the current row there are only two things
	// we need to deal with: 1) horizontal scrolling and 2) visual mode highlighting

	titlecols := o.titleColumnWidth()
	y := o.fr - o.rowoff
	row := &o.rows[o.fr]

	if o.coloff > 0 {
		fmt.Fprintf(ab, "\x1b[%d;%dH\x1b[1K\x1b[%dG", y+TOP_MARGIN+1, titlecols+LEFT_MARGIN+1, LEFT_MARGIN+1)
		length := len(row.title) - o.coloff
		if length > titlecols {
			length = titlecols
		}
		if length < 0 {
			length = 0
		}
		beg := o.coloff
		if len(row.title[beg:]) > length {
			ab.WriteString(row.title[beg : beg+length])
		} else {
			ab.WriteString(row.title[beg:])
		}
	}

	if o.mode == VISUAL {
		var j, k int
		if o.highlight[1] > o.highlight[0] {
			j, k = 0, 1
		} else {
			k, j = 0, 1
		}

		fmt.Fprintf(ab, "\x1b[%d;%dH\x1b[1K\x1b[%dG", y+TOP_MARGIN+1, titlecols+LEFT_MARGIN+1, LEFT_MARGIN+1)
		ab.WriteString(row.title[o.coloff : o.highlight[j]-o.coloff])
		ab.WriteString(LIGHT_GRAY_BG)
		ab.WriteString(row.title[o.highlight[j] : o.highlight[k]-o.coloff])
		ab.WriteString(RESET)
		ab.WriteString(row.title[o.highlight[k]:])
	}
}

func (o *Organizer) appendStandardRow(ab *strings.Builder, fr, y, titlecols int) {
	row := &o.rows[fr]
	// position cursor -note that you don't use lf/cr to position lines
	fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, LEFT_MARGIN+1)

	if row.star {
		ab.WriteString(BOLD)
	}
	if timeKeywordsRegex.MatchString(row.sort) {
		ab.WriteString(CYAN)
	} else {
		ab.WriteString(WHITE)
	}

	if row.archived && row.deleted {
		ab.WriteString(GREEN)
	} else if row.archived {
		ab.WriteString(YELLOW)
	} else if row.deleted {
		ab.WriteString(RED)
	}

	if row.dirty {
		ab.WriteString(BLACK + WHITE_BG)
	}
	if _, ok := o.marked_entries[row.id]; ok {
		ab.WriteString(BLACK + YELLOW_BG)
	}

	if len(row.title) > titlecols {
		ab.WriteString(row.title[:titlecols])
	} else {
		ab.WriteString(row.title)
		ab.WriteString(strings.Repeat(" ", titlecols-len(row.title)))
	}

	ab.WriteString(RESET)
	o.writeImageMarker(ab, y, row.hasImage)
	ab.WriteString(WHITE)
	sortX := o.Screen.divider - TIME_COL_WIDTH + 2
	width := o.Screen.divider - sortX
	if width > 0 {
		fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, sortX)
		ab.WriteString(strings.Repeat(" ", width))
		fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, sortX)
		if len(row.sort) > width {
			ab.WriteString(row.sort[:width])
		} else {
			ab.WriteString(row.sort)
		}
	}
	ab.WriteString(RESET)
}

func (o *Organizer) appendSearchRow(ab *strings.Builder, fr, y, titlecols int) {
	row := &o.rows[fr]

	fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, LEFT_MARGIN+1)

	if row.star {
		ab.WriteString(BOLD)
	}
	if timeKeywordsRegex.MatchString(row.sort) {
		ab.WriteString(CYAN)
	} else {
		ab.WriteString(WHITE)
	}

	if row.archived && row.deleted {
		ab.WriteString(GREEN)
	} else if row.archived {
		ab.WriteString(YELLOW)
	} else if row.deleted {
		ab.WriteString(RED)
	}

	if row.dirty {
		ab.WriteString(BLACK + WHITE_BG)
	}
	if _, ok := o.marked_entries[row.id]; ok {
		ab.WriteString(BLACK + YELLOW_BG)
	}

	if len(row.title) <= titlecols {
		ab.WriteString(row.ftsTitle)
	} else {
		pos := strings.Index(row.ftsTitle, "\x1b[49m")
		if pos > 0 && pos < titlecols+11 && len(row.ftsTitle) >= titlecols+15 {
			ab.WriteString(row.ftsTitle[:titlecols+15])
		} else {
			ab.WriteString(row.title[:titlecols])
		}
	}

	length := len(row.title)
	if length > titlecols {
		length = titlecols
	}
	if spaces := titlecols - length; spaces > 0 {
		ab.WriteString(strings.Repeat(" ", spaces))
	}
	ab.WriteString(RESET)
	o.writeImageMarker(ab, y, row.hasImage)

	sortX := o.Screen.divider - TIME_COL_WIDTH + 2
	width := o.Screen.divider - sortX
	if width > 0 {
		fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, sortX)
		ab.WriteString(strings.Repeat(" ", width))
		fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, sortX)
		if len(row.sort) > width {
			ab.WriteString(row.sort[:width])
		} else {
			ab.WriteString(row.sort)
		}
	}
	ab.WriteString(RESET)
}

func (o *Organizer) writeImageMarker(ab *strings.Builder, y int, hasImage bool) {
	markerX := o.markerColumnStart()
	if markerX <= LEFT_MARGIN {
		return
	}

	fmt.Fprintf(ab, "\x1b[%d;%dH", y+TOP_MARGIN+1, markerX)
	ab.WriteString(WHITE)
	if hasImage {
		ab.WriteString(filledImageMarker)
	} else {
		ab.WriteString(emptyImageMarker)
	}
	ab.WriteString(RESET)
	for i := 0; i < IMAGE_MARKER_AGE_GAP; i++ {
		fmt.Fprintf(ab, "\x1b[%d;%dH \x1b[37;1mx\x1b[0m", y+TOP_MARGIN+1, markerX+IMAGE_MARKER_WIDTH+i)
	}
}

func padImageMarker(symbol string) string {
	if IMAGE_MARKER_WIDTH <= 0 {
		return ""
	}
	runes := []rune(symbol)
	runeLen := len(runes)
	if runeLen == 0 {
		return emptyImageMarker
	}
	if runeLen >= IMAGE_MARKER_WIDTH {
		return string(runes[:IMAGE_MARKER_WIDTH])
	}
	return symbol + strings.Repeat(" ", IMAGE_MARKER_WIDTH-runeLen)
}

func (o *Organizer) drawRows() {
	if len(o.rows) == 0 {
		return
	}
	var ab strings.Builder
	titlecols := o.titleColumnWidth()

	for y := 0; y < o.Screen.textLines; y++ {
		fr := y + o.rowoff
		if fr > len(o.rows)-1 {
			break
		}
		o.appendStandardRow(&ab, fr, y, titlecols)
	}
	// instances that require a full redraw do not require separate drawing of active row
	//o.drawActiveRow(&ab)

	ab.WriteString(RESET)
	fmt.Print(ab.String())
}

// for drawing containers when making a selection
func (o *Organizer) drawAltRows() {

	if len(o.altRows) == 0 {
		return
	}

	var ab strings.Builder
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+1, o.Screen.divider+2)
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", o.Screen.divider+1)

	for y := 0; y < o.Screen.textLines; y++ {

		fr := y + o.altRowoff
		if fr > len(o.altRows)-1 {
			break
		}

		length := len(o.altRows[fr].title)
		if length > o.Screen.totaleditorcols {
			length = o.Screen.totaleditorcols
		}

		if o.altRows[fr].star {
			ab.WriteString("\x1b[1m") //bold
			ab.WriteString("\x1b[1;36m")
		}

		if fr == o.altFr {
			ab.WriteString("\x1b[48;5;236m") // 236 is a grey
		}

		ab.WriteString(o.altRows[fr].title[:length])
		ab.WriteString("\x1b[0m") // return background to normal
		ab.WriteString(lf_ret)
	}
	fmt.Print(ab.String())
}

func (o *Organizer) drawRenderedNote() {
	if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(debugLog, "drawRenderedNote: ENTER (note has %d lines)\n", len(o.note))
		debugLog.Close()
	}

	if len(o.note) == 0 {
		return
	}
	start := o.altRowoff
	var end int
	// check if there are more lines than can fit on the screen
	if len(o.note)-start > o.Screen.textLines-1 {
		end = o.Screen.textLines + start
	} else {
		//end = len(o.note) - 1
		end = len(o.note)
	}

	if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(debugLog, "drawRenderedNote: printing lines %d to %d\n", start, end)
		debugLog.Close()
	}

	fmt.Fprintf(os.Stdout, "\x1b[%d;%dH", TOP_MARGIN+1, o.Screen.divider+1)
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", o.Screen.divider+0)
	fmt.Print(strings.Join(o.note[start:end], lf_ret))
	fmt.Print(RESET) //sometimes there is an unclosed escape sequence

	// Note: With Unicode placeholders (U+10EEEE), we don't need separate placements
	// The placeholders are embedded directly in the text and reference the transmitted images
	// Calling createImagePlacementsAtPositions() would delete the images!

	if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(debugLog, "drawRenderedNote: COMPLETE\n")
		debugLog.Close()
	}
}

// deleteAllKittyPlacements deletes all kitty image placements
func deleteAllKittyPlacements() {
	if !app.kitty {
		return
	}

	oscOpen, oscClose := "\x1b_G", "\x1b\\"
	if IsTmuxScreen() {
		oscOpen = "\x1bPtmux;\x1b\x1b_G"
		oscClose = "\x1b\x1b\\\\\x1b\\"
	}

	// Delete all placements: a=d (delete), d=A (all placements)
	cmd := oscOpen + "a=d,d=A,q=1" + oscClose
	fmt.Fprint(os.Stdout, cmd)
	os.Stdout.Sync()

	if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(debugLog, "DELETED: all kitty placements\n")
		debugLog.Close()
	}
}

// createKittyPlacement creates a placement at the current cursor position
func createKittyPlacement(imageID uint32, cols, rows int) error {
	if !app.kitty {
		return errors.New("kitty terminal not detected")
	}

	oscOpen, oscClose := "\x1b_G", "\x1b\\"
	if IsTmuxScreen() {
		oscOpen = "\x1bPtmux;\x1b\x1b_G"
		oscClose = "\x1b\x1b\\\\\x1b\\"
	}

	// Create placement at current cursor position: a=p (place), i=imageID, c=cols, r=rows
	args := []string{
		"a=p",
		"q=1", // show errors
		fmt.Sprintf("i=%d", imageID),
	}
	if cols > 0 {
		args = append(args, fmt.Sprintf("c=%d", cols))
	}
	if rows > 0 {
		args = append(args, fmt.Sprintf("r=%d", rows))
	}

	cmd := oscOpen + strings.Join(args, ",") + oscClose
	_, err := fmt.Fprint(os.Stdout, cmd)
	return err
}

// createImagePlacementsAtPositions creates kitty image placements at the correct screen positions
func (o *Organizer) createImagePlacementsAtPositions(startLine, endLine int) {
	if !app.kitty {
		return
	}

	// First, delete all existing placements
	deleteAllKittyPlacements()

	// Parse each visible line for image markers
	imageMarkerRegex := regexp.MustCompile(`\[KITTY_IMAGE:id=(\d+),cols=(\d+),rows=(\d+)\]`)

	for lineIdx := startLine; lineIdx < endLine && lineIdx < len(o.note); lineIdx++ {
		line := o.note[lineIdx]
		matches := imageMarkerRegex.FindAllStringSubmatch(line, -1)

		for _, match := range matches {
			if len(match) != 4 {
				continue
			}

			imageID, _ := strconv.ParseUint(match[1], 10, 32)
			cols, _ := strconv.Atoi(match[2])
			rows, _ := strconv.Atoi(match[3])

			// Calculate screen position
			screenRow := TOP_MARGIN + 1 + (lineIdx - startLine)
			screenCol := o.Screen.divider + 1

			// Move cursor to position
			fmt.Fprintf(os.Stdout, "\x1b[%d;%dH", screenRow, screenCol)

			// Create placement at current cursor position
			if err := createKittyPlacement(uint32(imageID), cols, rows); err != nil {
				if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
					fmt.Fprintf(debugLog, "ERROR creating placement: %v\n", err)
					debugLog.Close()
				}
			} else {
				if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
					fmt.Fprintf(debugLog, "PLACEMENT: created i=%d at row=%d,col=%d (%dx%d cells)\n",
						imageID, screenRow, screenCol, cols, rows)
					debugLog.Close()
				}
			}
		}
	}

	// Flush output
	os.Stdout.Sync()
}

// createPendingRelativePlacements creates all pending relative placements after Unicode placeholders are drawn
func createPendingRelativePlacements() {
	pendingPlacementsMux.Lock()
	defer pendingPlacementsMux.Unlock()

	if len(pendingRelativePlacements) == 0 {
		return
	}

	for _, p := range pendingRelativePlacements {
		if err := kittyCreateRelativePlacement(os.Stdout, p.imageID, p.placementID,
			KITTY_PLACEHOLDER_IMAGE_ID, KITTY_PLACEHOLDER_PLACEMENT_ID,
			p.cols, p.rows, 0, 0); err != nil {
			if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
				fmt.Fprintf(debugLog, "ERROR creating relative placement: %v\n", err)
				debugLog.Close()
			}
		} else {
			if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
				fmt.Fprintf(debugLog, "CREATED RELATIVE PLACEMENT: i=%d, p=%d, P=%d, Q=%d, c=%d, r=%d\n",
					p.imageID, p.placementID, KITTY_PLACEHOLDER_IMAGE_ID, KITTY_PLACEHOLDER_PLACEMENT_ID, p.cols, p.rows)
				debugLog.Close()
			}
		}
	}

	// Clear pending list
	pendingRelativePlacements = nil

	// Ensure all commands are sent to kitty immediately
	os.Stdout.Sync()

	if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(debugLog, "FLUSHED: all relative placement commands sent to kitty\n")
		debugLog.Close()
	}
}

func (o *Organizer) drawStatusBar() {

	var ab strings.Builder
	//position cursor and erase - and yes you do have to reposition cursor after erase
	fmt.Fprintf(&ab, "\x1b[%d;%dH\x1b[1K\x1b[%d;1H", o.Screen.textLines+TOP_MARGIN+1, o.Screen.divider, o.Screen.textLines+TOP_MARGIN+1)
	ab.WriteString("\x1b[7m") //switches to reversed colors

	var str string
	var id int
	var title string
	var keywords string
	if len(o.rows) > 0 {
		switch o.view {
		case TASK:
			id = o.getId()
			switch o.taskview {
			case BY_FIND:
				str = "search - " + o.Session.fts_search_terms
			case BY_FOLDER:
				str = fmt.Sprintf("%s[f] (%s[c])", o.filter, o.Database.taskContext(id))
			case BY_CONTEXT:
				str = fmt.Sprintf("%s[c] (%s[f])", o.filter, o.Database.taskFolder(id))
			case BY_RECENT:
				str = fmt.Sprintf("Recent: %s[c] %s[f]",
					o.Database.taskContext(id), o.Database.taskFolder(id))
			case BY_KEYWORD:
				str = o.filter + "[k]"
			}
		case CONTEXT:
			str = "Contexts"
		case FOLDER:
			str = "Folders"
		case KEYWORD:
			str = "Keywords"
		//case SYNC_LOG_VIEW:
		//	str = "Sync Log"
		default:
			str = "Other"
		}

		row := &o.rows[o.fr]

		if len(row.title) > 16 {
			title = row.title[:12] + "..."
		} else {
			title = row.title
		}

		id = row.id

		if o.view == TASK {
			keywords = o.Database.getTaskKeywords(row.id)
		}
	} else {
		title = "   No Results   "
		id = -1

	}

	// [49m - revert background to normal
	// 7m - reverses video
	// because video is reversted [42 sets text to green and 49 undoes it
	// also [0;35;7m -> because of 7m it reverses background and foreground
	// [0;7m is revert text to normal and reverse video
	status := fmt.Sprintf("\x1b[1m%s\x1b[0;7m %s \x1b[0;35;7m%s\x1b[0;7m %d %d/%d \x1b[1;42m%%s\x1b[0;7m sort: %s ",
		str, title, keywords, id, o.fr+1, len(o.rows), o.sort)

	// klugy way of finding length of string without the escape characters
	plain := fmt.Sprintf("%s %s %s %d %d/%d   sort: %s ",
		str, title, keywords, id, o.fr+1, len(o.rows), o.sort)
	length := len(plain)

	if length+len(fmt.Sprintf("%s", o.mode)) <= o.Screen.divider {
		/*
			s := fmt.Sprintf("%%-%ds", o.divider-length) // produces "%-25s"
			t := fmt.Sprintf(s, o.mode)
			fmt.Fprintf(&ab, status, t)
		*/
		fmt.Fprintf(&ab, status, fmt.Sprintf(fmt.Sprintf("%%-%ds", o.Screen.divider-length), o.mode))
	} else {
		status = fmt.Sprintf("\x1b[1m%s\x1b[0;7m %s \x1b[0;35;7m%s\x1b[0;7m %d %d/%d\x1b[49m",
			str, title, keywords, id, o.fr+1, len(o.rows))
		plain = fmt.Sprintf("%s %s %s %d %d/%d",
			str, title, keywords, id, o.fr+1, len(o.rows))
		length := len(plain)
		if length < o.Screen.divider {
			fmt.Fprintf(&ab, "%s%-*s", status, o.Screen.divider-length, " ")
		} else {
			status = fmt.Sprintf("\x1b[1m%s\x1b[0;7m %s %s %d %d/%d",
				str, title, keywords, id, o.fr+1, len(o.rows))
			ab.WriteString(status[:o.Screen.divider+10])
		}
	}
	ab.WriteString("\x1b[0m") //switches back to normal formatting
	fmt.Print(ab.String())
}

func (o *Organizer) drawSearchRows() {
	if len(o.rows) == 0 {
		return
	}
	var ab strings.Builder
	titlecols := o.titleColumnWidth()

	for y := 0; y < o.Screen.textLines; y++ {
		fr := y + o.rowoff
		if fr > len(o.rows)-1 {
			break
		}
		o.appendSearchRow(&ab, fr, y, titlecols)
	}
	fmt.Print(ab.String())
}

// should be removed after more testing
func (o *Organizer) drawRowAt(fr int) {
	if fr < 0 || fr >= len(o.rows) {
		return
	}
	if fr < o.rowoff || fr >= o.rowoff+o.Screen.textLines {
		return
	}
	titlecols := o.titleColumnWidth()
	y := fr - o.rowoff
	var ab strings.Builder
	if o.taskview == BY_FIND {
		o.appendSearchRow(&ab, fr, y, titlecols)
	} else {
		o.appendStandardRow(&ab, fr, y, titlecols)
	}
	fmt.Print(ab.String())
}

func (o *Organizer) drawActive() {
	// When doing a partial redraw, we first draw the standard row, then
	// address horizontal scrolling and visual mode highlighting
	titlecols := o.titleColumnWidth()
	y := o.fr - o.rowoff
	var ab strings.Builder
	o.appendStandardRow(&ab, o.fr, y, titlecols)
	o.drawActiveRow(&ab)
	fmt.Print(ab.String())
}

func (o *Organizer) erasePreviousRowMarker(prevRow int) {
	y := prevRow - o.rowoff
	fmt.Fprintf(os.Stdout, "\x1b[%d;%dH ", y+TOP_MARGIN+1, 0)
}

// deleteAllKittyImages sends delete command for all cached kitty images
func deleteAllKittyImages() {
	if !app.kitty || !app.kittyPlace {
		return
	}

	oscOpen, oscClose := "\x1b_G", "\x1b\\"
	if IsTmuxScreen() {
		oscOpen = "\x1bPtmux;\x1b\x1b_G"
		oscClose = "\x1b\x1b\\\\\x1b\\"
	}

	// Delete all images: a=d (delete), d=A (all images)
	deleteCmd := oscOpen + "a=d,d=A" + oscClose
	os.Stdout.WriteString(deleteCmd)
}

// change function name to displayRenderedNote
func (o *Organizer) drawPreview() {
	if len(o.rows) == 0 {
		o.Screen.eraseRightScreen()
		return
	}

	// Don't delete images - let them persist in kitty for reuse
	// deleteAllKittyImages()

	id := o.rows[o.fr].id
	var note string
	if o.taskview != BY_FIND {
		note = o.Database.readNoteIntoString(id)
	} else {
		note = o.Database.highlightTerms2(id)
	}
	o.Screen.eraseRightScreen()

	var lang string
	if o.Database.taskFolder(id) == "code" {
		c := o.Database.taskContext(id)
		var ok bool
		if lang, ok = Languages[c]; !ok {
			lang = "markdown"
		}
	} else {
		lang = "markdown"
	}

	if lang == "markdown" {
		o.renderMarkdown(note)
	} else {
		o.renderCode(note, lang)
	}
	o.drawRenderedNote()
}

// extractImageURLs extracts all image URLs from markdown
func extractImageURLs(markdown string) []string {
	matches := markdownImageRegex.FindAllStringSubmatch(markdown, -1)
	urls := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 2 {
			urls = append(urls, match[2]) // match[2] is the URL
		}
	}
	return urls
}

// prepareKittyImage loads and encodes an image (possibly from cache) without transmitting it.
// It is safe to run concurrently.
func prepareKittyImage(url string) *preparedImage {
	initImageCache()

	res := &preparedImage{
		url:           url,
		isGoogleDrive: strings.Contains(url, "drive.google.com"),
		source:        "fresh-load",
	}

	var imgObj image.Image

	// Try cache metadata first (kitty reuse)
	if globalImageCache != nil {
		if entry, ok := globalImageCache.GetKittyMeta(url); ok {
			res.cachedImageID = entry.ImageID
			res.cachedFingerprint = entry.Fingerprint
			res.imgW = entry.Width
			res.imgH = entry.Height
		}
	}

	// Load cached base64 if available (Drive only)
	if res.isGoogleDrive && globalImageCache != nil {
		if cachedBase64, width, height, found := globalImageCache.GetCachedImageData(url); found {
			decoded, err := base64.StdEncoding.DecodeString(cachedBase64)
			if err == nil {
				res.data = decoded
				res.imgW = width
				res.imgH = height
				if res.cachedFingerprint == "" {
					res.cachedFingerprint = hashString(cachedBase64)
				}
				res.source = "disk-cache"
				return res
			}
		}
	}

	// Load fresh
	img, err := loadImageForGlamour(url)
	if err != nil {
		res.err = err
		return res
	}
	imgObj = img
	res.imgW = imgObj.Bounds().Dx()
	res.imgH = imgObj.Bounds().Dy()

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, imgObj); err != nil {
		res.err = err
		return res
	}
	res.data = buf.Bytes()

	// Cache Drive images
	if res.isGoogleDrive && globalImageCache != nil {
		base64Data := base64.StdEncoding.EncodeToString(res.data)
		res.cachedFingerprint = hashString(base64Data)
		_ = globalImageCache.StoreCachedImageData(url, base64Data, res.imgW, res.imgH)
	}

	if res.cachedFingerprint == "" {
		res.cachedFingerprint = hashBytes(res.data)
	}

	return res
}

// transmitPreparedKittyImage transmits (or reuses) a prepared image and returns imageID, cols, rows.
// Must be called in render order to keep Glamour's lookup ordering intact.
func transmitPreparedKittyImage(prep *preparedImage, maxCols int) (uint32, int, int) {
	if prep == nil || prep.err != nil || prep.imgW == 0 || prep.imgH == 0 {
		return 0, 0, 0
	}

	targetCols := maxCols - 2
	if targetCols <= 0 || targetCols > app.imageScale {
		targetCols = app.imageScale
	}
	rows := int(float64(prep.imgH) / float64(prep.imgW) * float64(targetCols) * 0.42)
	if rows < 1 {
		rows = 1
	}

	isTmux := IsTmuxScreen()

	// Reuse if fingerprint matches cache
	if prep.cachedImageID != 0 && prep.cachedFingerprint != "" {
		kittySessionImageMux.RLock()
		entry, ok := kittySessionImages[prep.cachedImageID]
		kittySessionImageMux.RUnlock()

		if ok && (entry.fingerprint == prep.cachedFingerprint || entry.fingerprint == "") && (entry.confirmed || trustKittyCache) {
			if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
				fmt.Fprintf(debugLog, "transmitKittyImage[reuse-kitty]: %s -> ID=%d, cols=%d, rows=%d\n",
					prep.url, prep.cachedImageID, targetCols, rows)
				debugLog.Close()
			}
			_ = kittyUpdateVirtualPlacement(prep.cachedImageID, targetCols, rows, isTmux)
			if prep.isGoogleDrive && globalImageCache != nil {
				_ = globalImageCache.UpdateKittyMeta(prep.url, prep.cachedImageID, targetCols, rows, prep.cachedFingerprint)
			}
			return prep.cachedImageID, targetCols, rows
		}
	}

	// Need to transmit
	imageID := nextSmallKittyID(prep.url)
	if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(debugLog, "transmitKittyImage[%s]: %s -> ID=%d, cols=%d, rows=%d (scale=%d)\n",
			prep.source, prep.url, imageID, targetCols, rows, app.imageScale)
		debugLog.Close()
	}

	if err := kittyTransmitActualImage(prep.data, imageID, targetCols, rows, isTmux); err != nil {
		return 0, 0, 0
	}

	kittySessionImageMux.Lock()
	kittySessionImages[imageID] = kittySessionEntry{
		fingerprint: prep.cachedFingerprint,
		confirmed:   true,
	}
	kittySessionImageMux.Unlock()

	if prep.isGoogleDrive && globalImageCache != nil {
		_ = globalImageCache.UpdateKittyMeta(prep.url, imageID, targetCols, rows, prep.cachedFingerprint)
	}

	return imageID, targetCols, rows
}

// replaceImagesWithPlaceholders replaces glamour's image output with kitty placeholders

// kittyTransmitImageToStdout transmits image data directly to stdout using kitty protocol
func kittyTransmitImageToStdout(data []byte, imageID uint32, cols, rows int, isTmux bool) error {
	oscOpen, oscClose := "\x1b_G", "\x1b\\"
	if isTmux {
		oscOpen = "\x1bPtmux;\x1b\x1b_G"
		oscClose = "\x1b\x1b\\\\\x1b\\"
	}

	// Single-step: transmit with U=1 for Unicode placeholders (like Yazi)
	header := []string{"a=T", "f=100", fmt.Sprintf("q=%s", kittyQFlag()), "U=1"} // a=T (transmit), U=1 (Unicode placeholders)
	if imageID != 0 {
		header = append(header, fmt.Sprintf("i=%d", imageID))
	}
	header = append(header, fmt.Sprintf("S=%d", len(data)))
	// Include size for proper virtual placement
	if cols > 0 {
		header = append(header, fmt.Sprintf("c=%d", cols))
	}
	if rows > 0 {
		header = append(header, fmt.Sprintf("r=%d", rows))
	}
	bsHdr := []byte(strings.Join(header, ",") + ",")

	// Chunk and transmit
	chunkSize := 4096
	encoded := base64.StdEncoding.EncodeToString(data)

	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]

		parts := [][]byte{[]byte(oscOpen)}
		if bsHdr != nil {
			parts = append(parts, bsHdr)
			bsHdr = nil
		}
		parts = append(parts, []byte("m=1;"), []byte(chunk), []byte(oscClose))

		if _, err := os.Stdout.Write(bytes.Join(parts, nil)); err != nil {
			return err
		}
	}

	// Final chunk marker
	_, err := os.Stdout.WriteString(oscOpen + "m=0;" + oscClose)
	return err
}

// createTransparentPNG creates a 1x1 transparent PNG
func createTransparentPNG() []byte {
	// 1x1 transparent PNG (67 bytes)
	// This is the minimal valid PNG with full transparency
	transparentPNG := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1 size
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, // RGBA mode
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, // Compressed data
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, // IEND chunk
		0x42, 0x60, 0x82,
	}
	return transparentPNG
}

// transmitTransparentPlaceholder transmits the 1x1 transparent placeholder image once
func transmitTransparentPlaceholder() error {
	if transparentPlaceholderTransmitted {
		return nil
	}

	pngData := createTransparentPNG()
	isTmux := IsTmuxScreen()

	oscOpen, oscClose := "\x1b_G", "\x1b\\"
	if isTmux {
		oscOpen = "\x1bPtmux;\x1b\x1b_G"
		oscClose = "\x1b\x1b\\\\\x1b\\"
	}

	// STEP 1: Transmit transparent image data (a=T to transmit and display)
	header1 := []string{
		"a=T",
		"f=100",
		"q=1", // q=1 to see errors
		fmt.Sprintf("i=%d", KITTY_PLACEHOLDER_IMAGE_ID), // image ID = 1
		fmt.Sprintf("S=%d", len(pngData)),               // data size
	}

	bsHdr := []byte(strings.Join(header1, ",") + ",")

	// Transmit in one chunk (it's tiny)
	encoded := base64.StdEncoding.EncodeToString(pngData)
	parts := [][]byte{
		[]byte(oscOpen),
		bsHdr,
		[]byte("m=0;"), // single chunk
		[]byte(encoded),
		[]byte(oscClose),
	}

	if _, err := os.Stdout.Write(bytes.Join(parts, nil)); err != nil {
		return err
	}

	// STEP 2: Create virtual placement (a=p with U=1) - SEPARATE command!
	header2 := []string{
		"a=p", // create placement
		"U=1", // virtual placement
		fmt.Sprintf("i=%d", KITTY_PLACEHOLDER_IMAGE_ID),     // image ID = 1
		fmt.Sprintf("p=%d", KITTY_PLACEHOLDER_PLACEMENT_ID), // placement ID = 1
		"c=1", // 1 column
		"r=1", // 1 row
		"q=1", // q=1 to see errors
	}

	cmd2 := oscOpen + strings.Join(header2, ",") + oscClose
	if _, err := os.Stdout.WriteString(cmd2); err != nil {
		return err
	}

	transparentPlaceholderTransmitted = true

	// DEBUG: Log successful transmission
	if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(debugLog, "TRANSPARENT PLACEHOLDER: transmitted i=%d with a=T\n", KITTY_PLACEHOLDER_IMAGE_ID)
		fmt.Fprintf(debugLog, "VIRTUAL PLACEMENT: created p=%d with U=1, c=1, r=1\n", KITTY_PLACEHOLDER_PLACEMENT_ID)
		debugLog.Close()
	}

	return nil
}

// kittyTransmitActualImage transmits actual image data and creates a placement
func kittyTransmitActualImage(data []byte, imageID uint32, cols, rows int, isTmux bool) error {
	oscOpen, oscClose := "\x1b_G", "\x1b\\"
	if isTmux {
		oscOpen = "\x1bPtmux;\x1b\x1b_G"
		oscClose = "\x1b\x1b\\\\\x1b\\"
	}

	// Transmit image data and create virtual placement (a=T,U=1)
	header := []string{
		"a=T",                             // Transmit data AND create placement
		"f=100",                           // PNG format
		fmt.Sprintf("q=%s", kittyQFlag()), // Quiet or verbose based on env
		"U=1",                             // Create Unicode placeholder virtual placement
		fmt.Sprintf("i=%d", imageID),
		fmt.Sprintf("p=%d", imageID), // Placement ID (must match placeholder grids)
		fmt.Sprintf("c=%d", cols),
		fmt.Sprintf("r=%d", rows),
		fmt.Sprintf("S=%d", len(data)),
	}
	bsHdr := []byte(strings.Join(header, ",") + ",")

	// Chunk and transmit
	chunkSize := 4096
	encoded := base64.StdEncoding.EncodeToString(data)

	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]

		parts := [][]byte{[]byte(oscOpen)}
		if bsHdr != nil {
			parts = append(parts, bsHdr)
			bsHdr = nil
		}
		parts = append(parts, []byte("m=1;"), []byte(chunk), []byte(oscClose))

		if _, err := os.Stdout.Write(bytes.Join(parts, nil)); err != nil {
			return err
		}
	}

	// Final chunk marker
	_, err := os.Stdout.WriteString(oscOpen + "m=0;" + oscClose)
	if err == nil {
		kittySessionImageMux.Lock()
		kittySessionImages[imageID] = kittySessionEntry{
			fingerprint: "",
			confirmed:   true,
		}
		kittySessionImageMux.Unlock()
		kittyImagesSent++
		kittyBytesSent += uint64(len(data))
	}
	return err
}

// kittyUpdateVirtualPlacement refreshes (or creates) a virtual placement without sending image data.
// This lets us reuse cached images while resizing to new cols/rows when imageScale changes.
func kittyUpdateVirtualPlacement(imageID uint32, cols, rows int, isTmux bool) error {
	oscOpen, oscClose := "\x1b_G", "\x1b\\"
	if isTmux {
		oscOpen = "\x1bPtmux;\x1b\x1b_G"
		oscClose = "\x1b\x1b\\\\\x1b\\"
	}

	args := []string{
		"a=p",
		"U=1",
		fmt.Sprintf("i=%d", imageID),
		fmt.Sprintf("p=%d", imageID),
		fmt.Sprintf("q=%s", kittyQFlag()),
	}
	if cols > 0 {
		args = append(args, fmt.Sprintf("c=%d", cols))
	}
	if rows > 0 {
		args = append(args, fmt.Sprintf("r=%d", rows))
	}

	_, err := fmt.Fprintf(os.Stdout, "%s%s%s", oscOpen, strings.Join(args, ","), oscClose)
	return err
}

var renderingMarkdown bool // Guard against re-entrant calls

// seedKittySessionFromCache populates kittySessionImages from disk cache so we can
// reuse images already stored in the running kitty terminal (same tab).
// Set VIMANGO_DISABLE_KITTY_CACHE_SEEDING to skip this behavior.
func seedKittySessionFromCache() {
	if seededKittyCache {
		return
	}
	if os.Getenv("VIMANGO_TRUST_KITTY_CACHE") != "" {
		trustKittyCache = true
	}
	if os.Getenv("VIMANGO_DISABLE_KITTY_CACHE_SEEDING") != "" {
		seededKittyCache = true
		return
	}
	if globalImageCache == nil {
		initImageCache()
	}
	if globalImageCache == nil {
		seededKittyCache = true
		return
	}

	// Only seed if the cache was written by the same kitty window
	globalImageCache.mutex.RLock()
	cachedWindow := globalImageCache.index.KittyWindow
	globalImageCache.mutex.RUnlock()
	if cachedWindow == "" || cachedWindow != currentKittyWindowID() {
		seededKittyCache = true
		return
	}

	globalImageCache.mutex.RLock()
	for _, entry := range globalImageCache.index.Entries {
		if entry.ImageID == 0 || entry.Fingerprint == "" {
			continue
		}
		kittySessionImageMux.Lock()
		kittySessionImages[entry.ImageID] = kittySessionEntry{
			fingerprint: entry.Fingerprint,
			confirmed:   false, // assumed present from previous run in same kitty
		}
		kittySessionImageMux.Unlock()
	}
	globalImageCache.mutex.RUnlock()
	seededKittyCache = true
}

// kittyImageCacheLookup returns image info for glamour during rendering
// This is called by glamour to create markers: [KITTY_IMAGE:id=X,cols=Y,rows=Z]
// We don't actually look up by URL - glamour passes the URL but we need to return
// dimensions for ALL images that were just transmitted. Since glamour processes
// images in order, we'll track the "next" imageID to return.
var nextImageLookupID uint32 = 1

func kittyImageCacheLookup(url string) (uint32, int, int, bool) {
	currentRenderImageMux.Lock()
	defer currentRenderImageMux.Unlock()

	if currentRenderOrderIdx < len(currentRenderImageOrder) {
		id := currentRenderImageOrder[currentRenderOrderIdx]
		currentRenderOrderIdx++
		if dims, exists := currentRenderImageDims[id]; exists {
			if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
				fmt.Fprintf(debugLog, "kittyImageCacheLookup(%s): returning ID=%d, cols=%d, rows=%d (ord=%d/%d)\n",
					url, id, dims.cols, dims.rows, currentRenderOrderIdx, len(currentRenderImageOrder))
				debugLog.Close()
			}
			return id, dims.cols, dims.rows, true
		}
	}

	// Not found - shouldn't happen if pre-transmission worked
	if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(debugLog, "kittyImageCacheLookup(%s): NOT FOUND (ordIdx=%d len=%d)\n", url, currentRenderOrderIdx, len(currentRenderImageOrder))
		debugLog.Close()
	}
	return 0, 0, 0, false
}

// replaceKittyImageMarkers converts glamour's text markers into actual Unicode placeholder grids
// Markers have format: [KITTY_IMAGE:id=42,cols=30,rows=15]
func replaceKittyImageMarkers(text string) string {
	markerRegex := regexp.MustCompile(`\[KITTY_IMAGE:id=(\d+),cols=(\d+),rows=(\d+)\]`)

	return markerRegex.ReplaceAllStringFunc(text, func(match string) string {
		submatches := markerRegex.FindStringSubmatch(match)
		if len(submatches) != 4 {
			// Parse error - return original marker
			return match
		}

		// Parse image ID, cols, rows from marker
		imageID, err1 := strconv.ParseUint(submatches[1], 10, 32)
		cols, err2 := strconv.Atoi(submatches[2])
		rows, err3 := strconv.Atoi(submatches[3])

		if err1 != nil || err2 != nil || err3 != nil {
			// Parse error - return original marker
			return match
		}

		// For virtual placements (a=T,U=1), placementID = imageID
		placementID := uint32(imageID)

		if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
			fmt.Fprintf(debugLog, "REPLACE: Converting marker to grid: id=%d, cols=%d, rows=%d\n", imageID, cols, rows)
			debugLog.Close()
		}

		// Generate the Unicode placeholder grid
		grid := buildPlaceholderGrid(uint32(imageID), placementID, cols, rows)

		if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
			fmt.Fprintf(debugLog, "PLACEHOLDER GRID: Building grid for id=%d, cols=%d, rows=%d\n", imageID, cols, rows)
			// Count actual newlines in the grid
			newlineCount := strings.Count(grid, "\n")
			fmt.Fprintf(debugLog, "GRID STATS: grid length=%d, newline count=%d, expected rows=%d\n", len(grid), newlineCount, rows)
			debugLog.Close()
		}
		return grid
	})
}

func (o *Organizer) renderMarkdown(s string) {
	// Prevent infinite loop from re-entrant calls
	if renderingMarkdown {
		if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
			fmt.Fprintf(debugLog, "WARNING: Prevented re-entrant renderMarkdown call!\n")
			debugLog.Close()
		}
		return
	}
	renderingMarkdown = true
	defer func() { renderingMarkdown = false }()

	// Pre-transmit kitty images if kitty is available and images are enabled
	if app.kitty && app.kittyPlace && app.showImages {
		// Seed session map from disk cache (assumed present in current kitty) unless disabled
		seedKittySessionFromCache()

		// Clear the dimension map for this render
		currentRenderImageMux.Lock()
		currentRenderImageDims = make(map[uint32]struct{ cols, rows int })
		nextImageLookupID = 1 // Reset lookup counter
		currentRenderImageOrder = currentRenderImageOrder[:0]
		currentRenderOrderIdx = 0
		currentRenderImageMux.Unlock()

		imageURLs := extractImageURLs(s)
		if len(imageURLs) > 0 {
			if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
				fmt.Fprintf(debugLog, "renderMarkdown: pre-transmitting %d images\n", len(imageURLs))
				debugLog.Close()
			}

			// Parallel prepare (fetch/decode/encode) with bounded workers, then ordered transmit.
			preparedMap := make(map[string]*preparedImage)
			type job struct{ url string }
			type result struct {
				prep *preparedImage
			}

			jobCh := make(chan job)
			resCh := make(chan result, len(imageURLs))
			var wg sync.WaitGroup

			workers := runtime.NumCPU()
			if workers > 6 {
				workers = 6
			}
			if workers < 2 {
				workers = 2
			}

			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := range jobCh {
						resCh <- result{prep: prepareKittyImage(j.url)}
					}
				}()
			}

			// Deduplicate URLs but preserve mapping to original order
			seen := make(map[string]bool)
			for _, url := range imageURLs {
				if !seen[url] {
					seen[url] = true
					jobCh <- job{url: url}
				}
			}
			close(jobCh)

			go func() {
				wg.Wait()
				close(resCh)
			}()

			for r := range resCh {
				if r.prep != nil {
					preparedMap[r.prep.url] = r.prep
				}
			}

			// Ordered transmit to keep kitty IDs aligned with markdown order
			for _, url := range imageURLs {
				prep := preparedMap[url]
				imageID, cols, rows := transmitPreparedKittyImage(prep, o.Screen.totaleditorcols-PREVIEW_RIGHT_PADDING)
				if imageID != 0 {
					currentRenderImageMux.Lock()
					currentRenderImageDims[imageID] = struct{ cols, rows int }{cols, rows}
					currentRenderImageOrder = append(currentRenderImageOrder, imageID)
					currentRenderImageMux.Unlock()
				}
			}
		}
	}

	// Configure renderer options WITH kitty support
	options := []glamour.TermRendererOption{
		glamour.WithStylePath(getGlamourStylePath()),
		glamour.WithWordWrap(0),
	}

	// Enable kitty image rendering if available and enabled
	if app.kitty && app.kittyPlace && app.showImages {
		options = append(options, glamour.WithKittyImages(true, kittyImageCacheLookup))
	}

	r, _ := glamour.NewTermRenderer(options...)

	if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(debugLog, "renderMarkdown: calling r.Render()\n")
		debugLog.Close()
	}

	note, _ := r.Render(s)

	if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(debugLog, "renderMarkdown: r.Render() returned, note length=%d\n", len(note))
		// Show where markers appear in rendered text
		markerIdx := strings.Index(note, "[KITTY_IMAGE:")
		previewLen := 200
		if len(note) < previewLen {
			previewLen = len(note)
		}
		if markerIdx >= 0 {
			fmt.Fprintf(debugLog, "MARKER POSITION: Found at index %d (first 200 chars: %q)\n", markerIdx, note[:previewLen])
			// Show context around second marker if it exists
			secondMarkerIdx := strings.Index(note[markerIdx+1:], "[KITTY_IMAGE:")
			if secondMarkerIdx >= 0 {
				secondMarkerIdx += markerIdx + 1
				// Show 100 chars before and after second marker
				contextStart := secondMarkerIdx - 100
				if contextStart < 0 {
					contextStart = 0
				}
				contextEnd := secondMarkerIdx + 100
				if contextEnd > len(note) {
					contextEnd = len(note)
				}
				fmt.Fprintf(debugLog, "SECOND MARKER CONTEXT (index %d): %q\n", secondMarkerIdx, note[contextStart:contextEnd])
			}
		} else {
			fmt.Fprintf(debugLog, "MARKER POSITION: NOT FOUND in rendered output (first 200 chars: %q)\n", note[:previewLen])
		}
		debugLog.Close()
	}

	// Replace glamour's text markers with actual Unicode placeholder grids
	if app.kitty && app.kittyPlace && app.showImages {
		note = replaceKittyImageMarkers(note)

		if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
			fmt.Fprintf(debugLog, "renderMarkdown: after replaceKittyImageMarkers, note length=%d\n", len(note))
			// Show first part of text to see if placeholder is at top
			previewLen := 200
			if len(note) < previewLen {
				previewLen = len(note)
			}
			fmt.Fprintf(debugLog, "AFTER REPLACEMENT: First 200 chars: %q\n", note[:previewLen])
			debugLog.Close()
		}
	}

	// glamour seems to add a '\n' at the start
	note = strings.TrimSpace(note)

	if o.taskview == BY_FIND {
		// could use strings.Count to make sure they are balanced
		note = strings.ReplaceAll(note, "qx", "\x1b[48;5;31m") //^^
		note = strings.ReplaceAll(note, "qy", "\x1b[0m")       // %%
	}

	if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(debugLog, "renderMarkdown: calling WordWrap()\n")
		debugLog.Close()
	}

	note = WordWrap(note, o.Screen.totaleditorcols-PREVIEW_RIGHT_PADDING)

	if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(debugLog, "renderMarkdown: WordWrap() returned\n")
		debugLog.Close()
	}

	o.note = strings.Split(note, "\n")

	if debugLog, err := os.OpenFile("kitty_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(debugLog, "renderMarkdown: FINISHED (note has %d lines)\n", len(o.note))
		debugLog.Close()
	}
}

// loadImageForGlamour loads images for glamour's kitty renderer
// It handles local files, Google Drive, and web URLs
func loadImageForGlamour(url string) (image.Image, error) {
	parsed, _ := net_url.Parse(url)
	maxW := int(app.Screen.ws.Xpixel)
	maxH := int(app.Screen.ws.Ypixel)
	if maxW == 0 {
		maxW = app.Screen.totaleditorcols * 8
	}
	if maxH == 0 {
		maxH = app.Session.imgSizeY
	}

	isHTTP := parsed.Scheme == "http" || parsed.Scheme == "https"
	isFile := parsed.Scheme == "file"

	// Try Google Drive first if we can extract a file ID
	if id, err := ExtractFileID(url); err == nil && id != "" {
		img, _, gErr := loadGoogleImage(url, maxW, maxH)
		if gErr == nil {
			return img, nil
		}
		// Fall through to other methods on error
	}

	// Try HTTP/HTTPS URLs
	if isHTTP {
		img, _, err := loadWebImage(url)
		return img, err
	}

	// Try file:// URLs
	if isFile {
		path := parsed.Path
		if path == "" {
			path = url
		}
		img, _, err := loadImage(path, maxW, maxH)
		return img, err
	}

	// Try as local filesystem path (supports mounted Drive paths)
	img, _, err := loadImage(url, maxW, maxH)
	return img, err
}

func (o *Organizer) renderCode(s string, lang string) {
	var buf bytes.Buffer
	_ = Highlight(&buf, s, lang, "terminal16m", o.Session.style[o.Session.styleIndex])
	note := buf.String()

	if o.taskview == BY_FIND {
		// could use strings.Count to make sure they are balanced
		note = strings.ReplaceAll(note, "qx", "\x1b[48;5;31m") //^^
		note = strings.ReplaceAll(note, "qy", "\x1b[0m")       // %%
	}
	note = WordWrap(note, o.Screen.totaleditorcols)
	o.note = strings.Split(note, "\n")
}

func (o *Organizer) drawNotice(s string) {
	o.renderNotice(s)
	if len(o.notice) == 0 {
		return
	}
	o.drawNoticeLayer()
	o.drawNoticeText()
}

func (o *Organizer) renderNotice(s string) {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath(getGlamourStylePath()),
		glamour.WithWordWrap(0),
	)
	note, _ := r.Render(s)
	// glamour seems to add a '\n' at the start
	note = strings.TrimSpace(note)

	note = WordWrap(note, o.Screen.totaleditorcols-PREVIEW_RIGHT_PADDING)
	o.notice = strings.Split(note, "\n")
}

func (o *Organizer) drawNoticeLayer() {
	var ab strings.Builder
	width := o.Screen.totaleditorcols - 10
	length := len(o.notice)
	if length > o.Screen.textLines-10 {
		length = o.Screen.textLines - 10
	}

	// \x1b[NC moves cursor forward by N columns
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", o.Screen.divider+6)

	//hide the cursor
	ab.WriteString("\x1b[?25l")
	// move the cursor
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, o.Screen.divider+7)

	//erase set number of chars on each line
	erase_chars := fmt.Sprintf("\x1b[%dX", o.Screen.totaleditorcols-10)
	for i := 0; i < length; i++ {
		ab.WriteString(erase_chars)
		ab.WriteString(lf_ret)
	}

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+6, o.Screen.divider+7)

	// \x1b[ 2*x is DECSACE to operate in rectable mode
	// \x1b[%d;%d;%d;%d;48;5;235$r is DECCARA to apply specified attributes (background color 235) to rectangle area
	// \x1b[ *x is DECSACE to exit rectangle mode
	fmt.Fprintf(&ab, "\x1b[2*x\x1b[%d;%d;%d;%d;48;5;235$r\x1b[*x",
		TOP_MARGIN+6, o.Screen.divider+7, TOP_MARGIN+4+length, o.Screen.divider+7+width)
	ab.WriteString("\x1b[48;5;235m") //draws the box lines with same background as above rectangle
	fmt.Print(ab.String())
	o.drawNoticeBox()
}

func (o *Organizer) drawNoticeBox() {
	width := o.Screen.totaleditorcols - 10
	length := len(o.notice) + 1
	if length > o.Screen.textLines-9 {
		length = o.Screen.textLines - 9
	}
	var ab strings.Builder
	move_cursor := fmt.Sprintf("\x1b[%dC", width)

	ab.WriteString("\x1b(0") // Enter line drawing mode
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5, o.Screen.divider+6)
	ab.WriteString("\x1b[37;1ml") //upper left corner

	for i := 1; i < length; i++ {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5+i, o.Screen.divider+6)
		// x=0x78 vertical line (q=0x71 is horizontal) 37=white; 1m=bold (only need 1 m)
		ab.WriteString("\x1b[37;1mx")
		ab.WriteString(move_cursor)
		ab.WriteString("\x1b[37;1mx")
	}

	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+4+length, o.Screen.divider+6)
	ab.WriteString("\x1b[1B")
	ab.WriteString("\x1b[37;1mm") //lower left corner

	move_cursor = fmt.Sprintf("\x1b[1D\x1b[%dB", length)

	for i := 1; i < width+1; i++ {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5, o.Screen.divider+6+i)
		ab.WriteString("\x1b[37;1mq")
		ab.WriteString(move_cursor)
		ab.WriteString("\x1b[37;1mq")
	}

	ab.WriteString("\x1b[37;1mj") //lower right corner
	fmt.Fprintf(&ab, "\x1b[%d;%dH", TOP_MARGIN+5, o.Screen.divider+7+width)
	ab.WriteString("\x1b[37;1mk") //upper right corner

	//exit line drawing mode
	ab.WriteString("\x1b(B")
	ab.WriteString("\x1b[0m")
	ab.WriteString("\x1b[?25h")
	fmt.Print(ab.String())
}

func (o *Organizer) drawNoticeText() {
	length := len(o.notice)
	if length > o.Screen.textLines-10 {
		length = o.Screen.textLines - 10
	}
	start := o.altRowoff
	var end int
	// check if there are more lines than can fit on the screen
	if len(o.notice)-start > length {
		end = length + start - 2
	} else {
		//end = len(o.note) - 1
		end = len(o.notice)
	}
	fmt.Fprintf(os.Stdout, "\x1b[%d;%dH", TOP_MARGIN+6, o.Screen.divider+8)
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", o.Screen.divider+7)
	fmt.Print(strings.Join(o.notice[start:end], lf_ret))
	fmt.Print(RESET) //sometimes there is an unclosed escape sequence
}
func nextSmallKittyID(url string) uint32 {
	if id, ok := kittyIDMap[url]; ok {
		return id
	}
	id := kittyIDNext
	kittyIDNext++
	// Avoid zero
	if kittyIDNext == 0 {
		kittyIDNext = 1
	}
	kittyIDMap[url] = id
	kittyIDReverse[id] = url
	return id
}
