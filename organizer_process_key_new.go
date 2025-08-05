//go:build newkey
// +build newkey

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/slzatz/vimango/vim"
)

// KeyProcessorHelpers encapsulates common key processing functionality
type KeyProcessorHelpers struct {
	organizer *Organizer
}

// VimKeyHandler handles vim key sending with termcode lookup
func (h *KeyProcessorHelpers) SendVimKey(c int) {
	if z, found := termcodes[c]; found {
		vim.SendKey(z)
	} else {
		vim.SendInput(string(c))
	}
}

// CursorManager handles cursor position updates and validation
type CursorManager struct {
	organizer *Organizer
}

func (cm *CursorManager) UpdateCursorPosition() {
	pos := vim.GetCursorPosition()
	cm.organizer.fc = pos[1]
}

func (cm *CursorManager) UpdateCursorPositionWithRowCheck() {
	pos := vim.GetCursorPosition()
	cm.organizer.fc = pos[1]
	// if move to a new row then draw task note preview or container info
	// and set cursor back to beginning of line
	if cm.organizer.fr != pos[0]-1 {
		cm.organizer.fr = pos[0] - 1
		cm.organizer.fc = 0
		vim.SetCursorPosition(cm.organizer.fr+1, 0)
		cm.organizer.altRowoff = 0
		if cm.organizer.view == TASK {
			cm.organizer.drawPreview()
		} else {
			cm.organizer.displayContainerInfo()
		}
	}
}

func (cm *CursorManager) PreventRowChangeInInsertMode() {
	pos := vim.GetCursorPosition()
	cm.organizer.fc = pos[1]
	// need to prevent row from changing in INSERT mode; for instance, when an up or down arrow is pressed
	if cm.organizer.fr != pos[0]-1 {
		vim.SetCursorPosition(cm.organizer.fr+1, cm.organizer.fc)
	}
}

// BufferSyncManager manages buffer change tracking and synchronization
type BufferSyncManager struct {
	organizer *Organizer
}

func (bsm *BufferSyncManager) SyncBufferChanges() {
	s := bsm.organizer.vbuf.Lines()[bsm.organizer.fr]
	bsm.organizer.rows[bsm.organizer.fr].title = s
	row := &bsm.organizer.rows[bsm.organizer.fr]
	tick := bsm.organizer.vbuf.GetLastChangedTick()
	if tick > bsm.organizer.bufferTick {
		row.dirty = true
		bsm.organizer.bufferTick = tick
	}
}

// StateResetter handles common cleanup operations
type StateResetter struct {
	organizer *Organizer
}

func (sr *StateResetter) ResetTabCompletion() {
	sr.organizer.tabCompletion.index = 0
	sr.organizer.tabCompletion.list = nil
}

func (sr *StateResetter) ResetCommand() {
	sr.organizer.command = ""
}

func (sr *StateResetter) HandleEscape() {
	sr.organizer.showMessage("")
	sr.organizer.command = ""
	vim.SendKey("<esc>")
	sr.organizer.last_mode = sr.organizer.mode // not sure this is necessary
	sr.organizer.mode = NORMAL

	// Get cursor position - now should be preserved correctly by the buffer
	pos := vim.GetCursorPosition()
	sr.organizer.fc = pos[1]
	sr.organizer.fr = pos[0] - 1

	sr.ResetTabCompletion()
	sr.organizer.Session.imagePreview = false
	if sr.organizer.view == TASK {
		sr.organizer.drawPreview()
	}
}

// ModeHandler interface for handling keys in different modes
type ModeHandler interface {
	HandleKey(c int) (handled bool, newMode Mode)
}

// InsertModeHandler handles INSERT mode key processing
type InsertModeHandler struct {
	organizer         *Organizer
	helpers           *KeyProcessorHelpers
	cursorManager     *CursorManager
	bufferSyncManager *BufferSyncManager
	stateResetter     *StateResetter
}

func NewInsertModeHandler(o *Organizer) *InsertModeHandler {
	return &InsertModeHandler{
		organizer:         o,
		helpers:           &KeyProcessorHelpers{organizer: o},
		cursorManager:     &CursorManager{organizer: o},
		bufferSyncManager: &BufferSyncManager{organizer: o},
		stateResetter:     &StateResetter{organizer: o},
	}
}

func (h *InsertModeHandler) HandleKey(c int) (bool, Mode) {
	if c == '\r' {
		h.organizer.writeTitle() // now updates ftsTitle if taskview == BY_FIND
		vim.SendKey("<esc>")
		row := &h.organizer.rows[h.organizer.fr]
		row.dirty = false
		h.organizer.bufferTick = h.organizer.vbuf.GetLastChangedTick()
		h.stateResetter.ResetCommand()
		return true, NORMAL
	}

	h.helpers.SendVimKey(c)
	h.cursorManager.PreventRowChangeInInsertMode()
	h.bufferSyncManager.SyncBufferChanges()

	return true, INSERT
}

// NormalModeHandler handles NORMAL mode key processing
type NormalModeHandler struct {
	organizer         *Organizer
	helpers           *KeyProcessorHelpers
	cursorManager     *CursorManager
	bufferSyncManager *BufferSyncManager
	stateResetter     *StateResetter
}

func NewNormalModeHandler(o *Organizer) *NormalModeHandler {
	return &NormalModeHandler{
		organizer:         o,
		helpers:           &KeyProcessorHelpers{organizer: o},
		cursorManager:     &CursorManager{organizer: o},
		bufferSyncManager: &BufferSyncManager{organizer: o},
		stateResetter:     &StateResetter{organizer: o},
	}
}

func (h *NormalModeHandler) HandleKey(c int) (bool, Mode) {
	if c == ctrlKey('l') && h.organizer.last_mode == ADD_CHANGE_FILTER {
		h.organizer.Screen.eraseRightScreen()
		return true, ADD_CHANGE_FILTER
	}

	if c == '\r' {
		h.stateResetter.ResetCommand()
		row := &h.organizer.rows[h.organizer.fr]
		if row.dirty {
			h.organizer.writeTitle() // now updates ftsTitle if taskview == BY_FIND
			vim.SendKey("<esc>")
			row.dirty = false
			h.organizer.bufferTick = h.organizer.vbuf.GetLastChangedTick()
			return true, NORMAL
		}
	}

	if _, err := strconv.Atoi(string(c)); err != nil {
		h.organizer.command += string(c)
	}

	if cmd, found := h.organizer.normalCmds[h.organizer.command]; found {
		cmd(h.organizer)
		h.stateResetter.ResetCommand()
		vim.SendKey("<esc>")
		// Return the current mode after command execution (e.g., ":" switches to COMMAND_LINE)
		return true, h.organizer.mode
	}

	// in NORMAL mode don't want ' ' (leader), 'O', 'V', 'o' ctrl-V (22)
	// being passed to vim
	if _, ok := noopKeys[c]; ok {
		if c != int([]byte(leader)[0]) {
			h.organizer.showMessage("Ascii %d has no effect in Organizer NORMAL mode", c)
		}
		return true, NORMAL
	}

	// Send the keystroke to vim
	h.helpers.SendVimKey(c)
	//if z, found := termcodes[c]; found {
	//		h.organizer.ShowMessage(BR, "%s", z)
	//	}

	h.cursorManager.UpdateCursorPositionWithRowCheck()
	h.bufferSyncManager.SyncBufferChanges()

	mode := vim.GetCurrentMode()

	// OP_PENDING like 4da
	if mode == 4 {
		return true, NORMAL
	}

	h.stateResetter.ResetCommand()

	// the only way to get into EX_COMMAND or SEARCH
	if mode == 16 && h.organizer.mode != INSERT {
		h.organizer.showMessage("\\x1b[1m-- INSERT --\\x1b[0m")
	}

	newMode := modeMap[mode] //note that 8 => SEARCH (8 is also COMMAND)
	if newMode == VISUAL {
		pos := vim.GetVisualRange()
		h.organizer.highlight[1] = pos[1][1] + 1
		h.organizer.highlight[0] = pos[0][1]
	}

	s := h.organizer.vbuf.Lines()[h.organizer.fr]
	h.organizer.showMessage("%s", s)

	return true, newMode
}

// VisualModeHandler handles VISUAL mode key processing
type VisualModeHandler struct {
	organizer         *Organizer
	helpers           *KeyProcessorHelpers
	cursorManager     *CursorManager
	bufferSyncManager *BufferSyncManager
	stateResetter     *StateResetter
}

func NewVisualModeHandler(o *Organizer) *VisualModeHandler {
	return &VisualModeHandler{
		organizer:         o,
		helpers:           &KeyProcessorHelpers{organizer: o},
		cursorManager:     &CursorManager{organizer: o},
		bufferSyncManager: &BufferSyncManager{organizer: o},
		stateResetter:     &StateResetter{organizer: o},
	}
}

func (h *VisualModeHandler) HandleKey(c int) (bool, Mode) {
	if c == 'j' || c == 'k' || c == 'J' || c == 'V' || c == ctrlKey('v') || c == 'g' || c == 'G' {
		h.organizer.showMessage("Ascii %d has no effect in Organizer VISUAL mode", c)
		return true, VISUAL
	}

	h.helpers.SendVimKey(c)

	h.cursorManager.PreventRowChangeInInsertMode()
	h.bufferSyncManager.SyncBufferChanges()

	mode := vim.GetCurrentMode() // I think just a few possibilities - stay in VISUAL or something like 'x' switches to NORMAL and : to command
	newMode := modeMap[mode]     //note that 8 => SEARCH (8 is also COMMAND)
	h.stateResetter.ResetCommand()

	visPos := vim.GetVisualRange()
	h.organizer.highlight[1] = visPos[1][1] + 1
	h.organizer.highlight[0] = visPos[0][1]

	s := h.organizer.vbuf.Lines()[h.organizer.fr]
	h.organizer.showMessage("visual %s; %d %d", s, h.organizer.highlight[0], h.organizer.highlight[1])

	return true, newMode
}

// CommandLineModeHandler handles COMMAND_LINE mode key processing
type CommandLineModeHandler struct {
	organizer     *Organizer
	stateResetter *StateResetter
}

func NewCommandLineModeHandler(o *Organizer) *CommandLineModeHandler {
	return &CommandLineModeHandler{
		organizer:     o,
		stateResetter: &StateResetter{organizer: o},
	}
}

func (h *CommandLineModeHandler) HandleKey(c int) (bool, Mode) {
	switch c {
	case '\r':
		var cmd func(*Organizer, int)
		var found bool
		var s string
		pos := strings.LastIndex(h.organizer.command_line, " ")
		if pos == -1 {
			s = h.organizer.command_line
			if cmd, found = h.organizer.exCmds[s]; found {
				cmd(h.organizer, pos)
			}
		} else {
			s = h.organizer.command_line[:pos]
			if cmd, found = h.organizer.exCmds[s]; found {
				cmd(h.organizer, pos)
			}
		}
		h.stateResetter.ResetTabCompletion()

		if !found {
			// Try to provide helpful suggestions using command registry
			if h.organizer.commandRegistry != nil {
				suggestions := h.organizer.commandRegistry.SuggestCommand(s)
				if len(suggestions) > 0 {
					h.organizer.showMessage("%sCommand '%s' not found. Did you mean: %s?%s", RED_BG, s, strings.Join(suggestions, ", "), RESET)
				} else {
					h.organizer.showMessage("%sCommand '%s' not found. Use ':help' to see available commands.%s", RED_BG, s, RESET)
				}
			} else {
				// Fallback if registry not available
				h.organizer.showMessage("%sNot a recognized command: %s%s", RED_BG, s, RESET)
			}
			return true, h.organizer.last_mode
		}
		// Command was found and executed - return current mode (command may have changed it)
		return true, h.organizer.mode

	case '\t':
		pos := strings.Index(h.organizer.command_line, " ")
		if pos == -1 {
			return true, COMMAND_LINE
		}
		if h.organizer.tabCompletion.list != nil {
			h.organizer.tabCompletion.index++
			if h.organizer.tabCompletion.index > len(h.organizer.tabCompletion.list)-1 {
				h.organizer.tabCompletion.index = 0
			}
		} else {
			h.organizer.ShowMessage(BR, "tab")
			h.organizer.tabCompletion.index = 0
			cmd := h.organizer.command_line[:pos]
			option := h.organizer.command_line[pos+1:]

			if !(cmd == "o" || cmd == "cd") {
				return true, COMMAND_LINE
			}
			for _, k := range h.organizer.filterList {
				if strings.HasPrefix(k.Text, option) {
					h.organizer.tabCompletion.list = append(h.organizer.tabCompletion.list, FilterNames{Text: k.Text, Char: k.Char})
				}
			}

			if len(h.organizer.tabCompletion.list) == 0 {
				return true, COMMAND_LINE
			}
		}

		h.organizer.command_line = h.organizer.command_line[:pos+1] + h.organizer.tabCompletion.list[h.organizer.tabCompletion.index].Text
		h.organizer.showMessage(":%s (%c)", h.organizer.command_line, h.organizer.tabCompletion.list[h.organizer.tabCompletion.index].Char)
		return true, COMMAND_LINE

	case DEL_KEY, BACKSPACE:
		length := len(h.organizer.command_line)
		if length > 0 {
			h.organizer.command_line = h.organizer.command_line[:length-1]
		}

	default:
		h.organizer.command_line += string(c)
	}

	h.stateResetter.ResetTabCompletion()
	h.organizer.showMessage(":%s", h.organizer.command_line)

	return true, COMMAND_LINE
}

// AddChangeFilterModeHandler handles ADD_CHANGE_FILTER mode
type AddChangeFilterModeHandler struct {
	organizer *Organizer
}

func NewAddChangeFilterModeHandler(o *Organizer) *AddChangeFilterModeHandler {
	return &AddChangeFilterModeHandler{organizer: o}
}

func (h *AddChangeFilterModeHandler) HandleKey(c int) (bool, Mode) {
	switch c {
	case ARROW_UP, ARROW_DOWN, 'j', 'k':
		h.organizer.moveAltCursor(c)
		return true, ADD_CHANGE_FILTER

	case '\r':
		altRow := &h.organizer.altRows[h.organizer.altFr] //currently highlighted container row
		var tid int
		row := &h.organizer.rows[h.organizer.fr] //currently highlighted entry row
		switch h.organizer.altView {
		case KEYWORD:
			tid, _ = h.organizer.Database.keywordExists(altRow.title)
		case FOLDER:
			tid, _ = h.organizer.Database.folderExists(altRow.title)
		case CONTEXT:
			tid, _ = h.organizer.Database.contextExists(altRow.title)
		}
		if tid < 1 {
			h.organizer.showMessage("%q has not been synched yet - must do that before adding tasks", altRow.title)
			return true, ADD_CHANGE_FILTER
		}
		if len(h.organizer.marked_entries) == 0 {
			switch h.organizer.altView {
			case KEYWORD:
				h.organizer.Database.addTaskKeywordByTid(tid, row.id, true)
				h.organizer.showMessage("Added keyword %s to current entry", altRow.title)
			case FOLDER:
				h.organizer.Database.updateTaskFolderByTid(tid, row.id)
				h.organizer.showMessage("Current entry folder changed to %s", altRow.title)
			case CONTEXT:
				err := h.organizer.Database.updateTaskContextByTid(tid, row.id)
				if err != nil {
					h.organizer.showMessage("Error updating context (updateTaskContextByTid) for entry %d to tid %d: %v", row.id, tid, err)
					return true, ADD_CHANGE_FILTER
				}
				h.organizer.showMessage("Current entry had context changed to %s", altRow.title)
			}
		} else {
			for id := range h.organizer.marked_entries {
				switch h.organizer.altView {
				case KEYWORD:
					h.organizer.Database.addTaskKeywordByTid(tid, id, true)
				case FOLDER:
					h.organizer.Database.updateTaskFolderByTid(tid, id)
				case CONTEXT:
					err := h.organizer.Database.updateTaskContextByTid(tid, id)
					if err != nil {
						h.organizer.showMessage("Error updating context (updateTaskContextByTid) for entry %d to tid %d: %v", id, tid, err)
						return true, ADD_CHANGE_FILTER
					}
				}
				h.organizer.showMessage("Marked entries' %d changed/added to %s", h.organizer.altView, altRow.title)
			}
		}
		return true, ADD_CHANGE_FILTER
	}
	return true, ADD_CHANGE_FILTER
}

// SyncLogModeHandler handles SYNC_LOG mode
type SyncLogModeHandler struct {
	organizer *Organizer
}

func NewSyncLogModeHandler(o *Organizer) *SyncLogModeHandler {
	return &SyncLogModeHandler{organizer: o}
}

func (h *SyncLogModeHandler) HandleKey(c int) (bool, Mode) {
	switch c {
	case ARROW_UP, 'k':
		if len(h.organizer.rows) == 0 || h.organizer.fr == 0 {
			return true, SYNC_LOG
		}
		h.organizer.fr--
		h.organizer.Screen.eraseRightScreen()
		h.organizer.altRowoff = 0
		note := h.organizer.Database.readSyncLog(h.organizer.rows[h.organizer.fr].id)
		h.organizer.note = strings.Split(note, "\n")
		h.organizer.drawRenderedNote()

	case ARROW_DOWN, 'j':
		if len(h.organizer.rows) == 0 || h.organizer.fr == len(h.organizer.rows)-1 {
			return true, SYNC_LOG
		}
		h.organizer.fr++
		h.organizer.Screen.eraseRightScreen()
		h.organizer.altRowoff = 0
		note := h.organizer.Database.readSyncLog(h.organizer.rows[h.organizer.fr].id)
		h.organizer.note = strings.Split(note, "\n")
		h.organizer.drawRenderedNote()

	case ':':
		h.organizer.showMessage(":")
		h.organizer.command_line = ""
		h.organizer.last_mode = h.organizer.mode
		return true, COMMAND_LINE

	case ctrlKey('j'):
		if len(h.organizer.rows) == 0 {
			return true, SYNC_LOG
		}
		if len(h.organizer.note) > h.organizer.altRowoff+h.organizer.Screen.textLines {
			if len(h.organizer.note) < h.organizer.altRowoff+2*h.organizer.Screen.textLines {
				h.organizer.altRowoff = len(h.organizer.note) - h.organizer.Screen.textLines
			} else {
				h.organizer.altRowoff += h.organizer.Screen.textLines
			}
		}
		h.organizer.Screen.eraseRightScreen()
		h.organizer.drawRenderedNote()

	case ctrlKey('k'):
		if len(h.organizer.rows) == 0 {
			return true, SYNC_LOG
		}
		if h.organizer.altRowoff > h.organizer.Screen.textLines {
			h.organizer.altRowoff -= h.organizer.Screen.textLines
		} else {
			h.organizer.altRowoff = 0
		}
		h.organizer.Screen.eraseRightScreen()
		h.organizer.drawRenderedNote()

	case ctrlKey('d'):
		if len(h.organizer.rows) == 0 {
			return true, SYNC_LOG
		}
		if len(h.organizer.marked_entries) == 0 {
			h.organizer.Database.deleteSyncItem(h.organizer.rows[h.organizer.fr].id)
		} else {
			for id := range h.organizer.marked_entries {
				h.organizer.Database.deleteSyncItem(id)
			}
		}
		h.organizer.log(0)

	case 'm':
		if len(h.organizer.rows) == 0 {
			return true, SYNC_LOG
		}
		h.organizer.mark()
	}
	return true, SYNC_LOG
}

// PreviewSyncLogModeHandler handles PREVIEW_SYNC_LOG mode
type PreviewSyncLogModeHandler struct {
	organizer *Organizer
}

func NewPreviewSyncLogModeHandler(o *Organizer) *PreviewSyncLogModeHandler {
	return &PreviewSyncLogModeHandler{organizer: o}
}

func (h *PreviewSyncLogModeHandler) HandleKey(c int) (bool, Mode) {
	switch c {
	case ':':
		h.organizer.exCmd()
		return true, PREVIEW_SYNC_LOG

	case ctrlKey('j'):
		h.organizer.altRowoff++
		h.organizer.Screen.eraseRightScreen()
		h.organizer.drawRenderedNote()

	case ctrlKey('k'):
		if h.organizer.altRowoff > 0 {
			h.organizer.altRowoff--
		}
		h.organizer.Screen.eraseRightScreen()
		h.organizer.drawRenderedNote()

	case PAGE_DOWN:
		if len(h.organizer.note) > h.organizer.altRowoff+h.organizer.Screen.textLines {
			if len(h.organizer.note) < h.organizer.altRowoff+2*h.organizer.Screen.textLines {
				h.organizer.altRowoff = len(h.organizer.note) - h.organizer.Screen.textLines
			} else {
				h.organizer.altRowoff += h.organizer.Screen.textLines
			}
		}
		h.organizer.Screen.eraseRightScreen()
		h.organizer.drawRenderedNote()

	case PAGE_UP:
		if h.organizer.altRowoff > h.organizer.Screen.textLines {
			h.organizer.altRowoff -= h.organizer.Screen.textLines
		} else {
			h.organizer.altRowoff = 0
		}
		h.organizer.Screen.eraseRightScreen()
		h.organizer.drawRenderedNote()
	}
	return true, PREVIEW_SYNC_LOG
}

// LinksModeHandler handles LINKS mode
type LinksModeHandler struct {
	organizer *Organizer
}

func NewLinksModeHandler(o *Organizer) *LinksModeHandler {
	return &LinksModeHandler{organizer: o}
}

func (h *LinksModeHandler) HandleKey(c int) (bool, Mode) {
	if c < 49 || c > 57 {
		h.organizer.showMessage("That's not a number between 1 and 9")
		return true, NORMAL
	}
	linkNum := c - 48
	var found string
	pre := fmt.Sprintf("[%d]", linkNum)
	for _, line := range h.organizer.note {
		idx := strings.Index(line, pre)
		if idx != -1 {
			found = line
			break
		}
	}
	if found == "" {
		h.organizer.showMessage("There is no [%d]", linkNum)
		return true, NORMAL
	}
	beg := strings.Index(found, "http")
	end := strings.Index(found, "\\x1b\\\\")
	url := found[beg:end]
	h.organizer.showMessage("Opening %q", url)
	cmd := exec.Command("qutebrowser", url)
	err := cmd.Start()
	if err != nil {
		h.organizer.showMessage("Problem opening url: %v", err)
	}
	return true, NORMAL
}

// Main dispatcher function
func (o *Organizer) organizerProcessKey(c int) {
	// Handle global escape key
	if c == '\x1b' {
		stateResetter := &StateResetter{organizer: o}
		stateResetter.HandleEscape()
		return
	}

	// Get the appropriate mode handler
	handler := o.getModeHandler(o.mode)
	if handler != nil {
		handled, newMode := handler.HandleKey(c)
		if handled {
			o.mode = newMode
		}
	}
}

// handlerCache stores cached handlers per organizer instance
var handlerCacheMap = make(map[*Organizer]map[Mode]ModeHandler)

// getModeHandler returns the appropriate handler for the current mode (cached)
func (o *Organizer) getModeHandler(mode Mode) ModeHandler {
	// Initialize cache for this organizer if needed
	if handlerCacheMap[o] == nil {
		handlerCacheMap[o] = make(map[Mode]ModeHandler)
	}

	cache := handlerCacheMap[o]

	// Return cached handler if exists
	if handler, exists := cache[mode]; exists {
		return handler
	}

	// Create new handler and cache it
	var handler ModeHandler
	switch mode {
	case INSERT:
		handler = NewInsertModeHandler(o)
	case NORMAL:
		handler = NewNormalModeHandler(o)
	case VISUAL:
		handler = NewVisualModeHandler(o)
	case COMMAND_LINE:
		handler = NewCommandLineModeHandler(o)
	case ADD_CHANGE_FILTER:
		handler = NewAddChangeFilterModeHandler(o)
	case SYNC_LOG:
		handler = NewSyncLogModeHandler(o)
	case PREVIEW_SYNC_LOG:
		handler = NewPreviewSyncLogModeHandler(o)
	case LINKS:
		handler = NewLinksModeHandler(o)
	default:
		return nil
	}

	cache[mode] = handler
	return handler
}
