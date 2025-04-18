package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/jung-kurt/gofpdf"
	"github.com/mandolyte/mdtopdf/v2"
	"github.com/slzatz/vimango/vim"
)

var cmd_lookup = map[string]func(*Organizer, int){
	"open":            (*Organizer).open,
	"o":               (*Organizer).open,
	"opencontext":     (*Organizer).openContext,
	"oc":              (*Organizer).openContext,
	"openfolder":      (*Organizer).openFolder,
	"of":              (*Organizer).openFolder,
	"openkeyword":     (*Organizer).openKeyword,
	"ok":              (*Organizer).openKeyword,
	"quit":            (*Organizer).quitApp,
	"q":               (*Organizer).quitApp,
	"q!":              (*Organizer).quitApp,
	"e":               (*Organizer).editNote,
	"vertical resize": (*Organizer).verticalResize,
	"vert res":        (*Organizer).verticalResize,
	"test":            (*Organizer).sync3,
	"sync":            (*Organizer).sync3,
	"bulktest":        (*Organizer).initialBulkLoad,
	"bulkload":        (*Organizer).initialBulkLoad,
	"reverseload":     (*Organizer).reverse,
	"reversetest":     (*Organizer).reverse,
	"new":             (*Organizer).newEntry,
	"n":               (*Organizer).newEntry,
	"refresh":         (*Organizer).refresh,
	"r":               (*Organizer).refresh,
	"find":            (*Organizer).find,
	"contexts":        (*Organizer).contexts,
	"context":         (*Organizer).contexts,
	"c":               (*Organizer).contexts,
	"folders":         (*Organizer).folders,
	"folder":          (*Organizer).folders,
	"f":               (*Organizer).folders,
	"keywords":        (*Organizer).keywords,
	"keyword":         (*Organizer).keywords,
	"k":               (*Organizer).keywords,
	"recent":          (*Organizer).recent,
	"log":             (*Organizer).log,
	"deletekeywords":  (*Organizer).deleteKeywords,
	"delkw":           (*Organizer).deleteKeywords,
	"delk":            (*Organizer).deleteKeywords,
	"showall":         (*Organizer).showAll,
	"show":            (*Organizer).showAll,
	"cc":              (*Organizer).updateContainer,
	"ff":              (*Organizer).updateContainer,
	"kk":              (*Organizer).updateContainer,
	"write":           (*Organizer).write,
	"w":               (*Organizer).write,
	"deletemarks":     (*Organizer).deleteMarks,
	"delmarks":        (*Organizer).deleteMarks,
	"delm":            (*Organizer).deleteMarks,
	"copy":            (*Organizer).copyEntry,
	"savelog":         (*Organizer).savelog,
	"save":            (*Organizer).save,
	"image":           (*Organizer).setImage,
	"images":          (*Organizer).setImage,
	"print":           (*Organizer).printDocument,
	"ha":              (*Organizer).printList,
	"ha2":             (*Organizer).printList2,
	"printlist":       (*Organizer).printList2,
	"pl":              (*Organizer).printList2,
	"sort":            (*Organizer).sortEntries,
}

func (o *Organizer) log(unused int) {
	o.Database.getSyncItems(MAX)
	o.fc, o.fr, o.rowoff = 0, 0, 0
	o.altRowoff = 0
	o.mode = SYNC_LOG //kluge INSERT, NORMAL, ...
	//o.view = SYNC_LOG_VIEW //TASK, FOLDER, KEYWORD ...
	o.view = -1 //TASK, FOLDER, KEYWORD ...

	// show first row's note
	o.Screen.eraseRightScreen()
	if len(o.rows) == 0 {
		o.ShowMessage(BL, "%sThere are no saved sync logs%s", BOLD, RESET)
		return
	}
	note := o.Database.readSyncLog(o.rows[o.fr].id)
	o.note = strings.Split(note, "\n")
	o.drawPreviewWithoutImages()
	o.clearMarkedEntries()
	o.ShowMessage(BL, "")
}

func (o *Organizer) open(pos int) {
	if pos == -1 {
		o.ShowMessage(BL, "You did not provide a context or folder!")
		//o.mode = NORMAL
		o.mode = o.last_mode
		return
	}

	var tid int
	var ok bool
	input := o.command_line[pos+1:]
	if tid, ok = o.Database.contextExists(input); ok {
		o.taskview = BY_CONTEXT
	}

	if !ok {
		if tid, ok = o.Database.folderExists(input); ok {
			o.taskview = BY_FOLDER
		}
	}

	if !ok {
		o.ShowMessage(BL, "%s is not a valid context or folder!", input)
		o.mode = o.last_mode
		return
	}

	if tid < 1 {
		o.ShowMessage(BL, "%q is an unsynced context or folder!", input)
		o.mode = o.last_mode
		return
	}

	o.filter = input
	o.ShowMessage(BL, "'%s' will be opened", o.filter)

	o.clearMarkedEntries()
	o.view = TASK
	o.mode = NORMAL
	o.fc, o.fr, o.rowoff = 0, 0, 0
	//o.rows = DB.filterEntries(o.taskview, o.filter, o.show_deleted, o.sort, o.sortPriority, MAX)
	o.FilterEntries(MAX)
	if len(o.rows) == 0 {
		o.insertRow(0, "", true, false, false, BASE_DATE)
		o.rows[0].dirty = false
		o.ShowMessage(BL, "No results were returned")
	}
	o.Session.imagePreview = false
	o.readRowsIntoBuffer()
	vim.CursorSetPosition(1, 0)
	o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
	o.altRowoff = 0
	o.drawPreview()
	return
}

func (o *Organizer) openContext(pos int) {
	if pos == -1 {
		o.ShowMessage(BL, "You did not provide a context!")
		o.mode = o.last_mode
		return
	}

	input := o.command_line[pos+1:]
	var tid int
	var ok bool
	if tid, ok = o.Database.contextExists(input); !ok {
		o.ShowMessage(BL, "%s is not a valid context!", input)
		o.mode = o.last_mode
		return
	}
	if tid < 1 {
		o.ShowMessage(BL, "%q is an unsynced context!", input)
		o.mode = o.last_mode
		return
	}

	o.filter = input

	o.ShowMessage(BL, "'%s' will be opened", o.filter)

	o.clearMarkedEntries()
	//o.folder = ""
	//o.keyword = ""
	o.taskview = BY_CONTEXT
	o.view = TASK
	o.mode = NORMAL
	o.fc, o.fr, o.rowoff = 0, 0, 0
	//o.rows = DB.filterEntries(o.taskview, o.filter, o.show_deleted, o.sort, o.sortPriority, MAX)
	o.FilterEntries(MAX)
	if len(o.rows) == 0 {
		o.insertRow(0, "", true, false, false, BASE_DATE)
		o.rows[0].dirty = false
		o.ShowMessage(BL, "No results were returned")
	}
	o.Session.imagePreview = false
	o.readRowsIntoBuffer()
	vim.CursorSetPosition(1, 0)
	o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
	o.altRowoff = 0
	o.drawPreview()
	return
}

func (o *Organizer) openFolder(pos int) {
	if pos == -1 {
		o.ShowMessage(BL, "You did not provide a folder!")
		o.mode = o.last_mode
		return
	}

	input := o.command_line[pos+1:]
	var ok bool
	var tid int
	if tid, ok = o.Database.folderExists(input); !ok {
		o.ShowMessage(BL, "%s is not a valid folder!", input)
		o.mode = o.last_mode
		return
	}
	if tid < 1 {
		o.ShowMessage(BL, "%q is an unsynced folder!", input)
		o.mode = o.last_mode
		return
	}

	o.filter = input

	o.ShowMessage(BL, "'%s' will be opened", o.filter)

	o.clearMarkedEntries()
	o.taskview = BY_FOLDER
	o.view = TASK
	o.mode = NORMAL
	o.fc, o.fr, o.rowoff = 0, 0, 0
	//o.rows = DB.filterEntries(o.taskview, o.filter, o.show_deleted, o.sort, o.sortPriority, MAX)
	o.FilterEntries(MAX)
	if len(o.rows) == 0 {
		o.insertRow(0, "", true, false, false, BASE_DATE)
		o.rows[0].dirty = false
		o.ShowMessage(BL, "No results were returned")
	}
	o.Session.imagePreview = false
	o.readRowsIntoBuffer()
	vim.CursorSetPosition(1, 0)
	o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
	o.altRowoff = 0
	o.drawPreview()
	return
}

func (o *Organizer) openKeyword(pos int) {
	if pos == -1 {
		o.ShowMessage(BL, "You did not provide a keyword!")
		o.mode = o.last_mode
		return
	}
	input := o.command_line[pos+1:]
	var ok bool
	var tid int
	if tid, ok = o.Database.keywordExists(input); !ok {
		o.ShowMessage(BL, "%s is not a valid keyword!", input)
		o.mode = o.last_mode
		return
	}
	// this guard may not be necessary for keywords
	if tid < 1 {
		o.ShowMessage(BL, "%q is an unsynced keyword!", input)
		o.mode = o.last_mode
		return
	}

	o.filter = input

	o.ShowMessage(BL, "'%s' will be opened", o.filter)

	o.clearMarkedEntries()
	o.taskview = BY_KEYWORD
	o.view = TASK
	o.mode = NORMAL
	o.fc, o.fr, o.rowoff = 0, 0, 0
	//o.rows = DB.filterEntries(o.taskview, o.filter, o.show_deleted, o.sort, o.sortPriority, MAX)
	o.FilterEntries(MAX)
	if len(o.rows) == 0 {
		o.insertRow(0, "", true, false, false, BASE_DATE)
		o.rows[0].dirty = false
		o.ShowMessage(BL, "No results were returned")
	}
	o.Session.imagePreview = false
	o.readRowsIntoBuffer()
	vim.CursorSetPosition(1, 0)
	o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
	o.altRowoff = 0
	o.drawPreview()
	return
}

func (o *Organizer) write(pos int) {
  var updated_rows []int
	if o.view != TASK {
		//o.Database.updateRows()
    return
	}
	for i, r := range o.rows {
		if r.dirty {
      o.Database.updateTitle(&r)
      o.rows[i].dirty = false
      updated_rows = append(updated_rows, r.id)
		}
  }
	if len(updated_rows) == 0 {
	  o.ShowMessage(BL, "There were no rows to update")
	} else {
	  o.ShowMessage(BL, "These ids were updated: %v", updated_rows)
  }
	o.mode = o.last_mode
	o.command_line = ""
}

func (o *Organizer) quitApp(_ int) {
	if o.command_line == "q!" {
		app.Run = false
		return
	}
	unsaved_changes := false
	for _, r := range o.rows {
		if r.dirty {
			unsaved_changes = true
			break
		}
	}
	if unsaved_changes {
		o.mode = NORMAL
		o.ShowMessage(BL, "No db write since last change")
	} else {
		app.Run = false
	}
}

func (o *Organizer) editNote(id int) {

	if o.view != TASK {
		o.command = ""
		o.mode = o.last_mode
		o.ShowMessage(BL, "Only entries have notes to edit!")
		return
	}

	//pos is zero if no space and command modifier
	if id == -1 {
		id = o.getId()
	}
	if id == -1 {
		o.ShowMessage(BL, "You need to save item before you can create a note")
		o.command = ""
		o.mode = o.last_mode
		return
	}

	//sess.showOrgMessage("Edit note %d", id)
	o.Session.editorMode = true

	active := false
	for _, w := range app.Windows {
		if e, ok := w.(*Editor); ok {
			if e.id == id {
				active = true
				p = e
				break
			}
		}
	}

	if !active {
		p = app.NewEditor()
		app.Windows = append(app.Windows, p)
		p.id = id
    p.title = o.rows[o.fr].title
		p.top_margin = TOP_MARGIN + 1

		if o.Database.taskFolder(o.rows[o.fr].id) == "code" {
			p.output = &Output{}
			p.output.is_below = true
			p.output.id = id
			app.Windows = append(app.Windows, p.output)
		}
		o.Database.readNoteIntoBuffer(p, id)
		p.bufferTick = vim.BufferGetLastChangedTick(p.vbuf)

	}

	o.Screen.positionWindows()
	o.Screen.eraseRightScreen() //erases editor area + statusbar + msg
	//delete any images
	//fmt.Print("\x1b_Ga=d\x1b\\") //now in sess.eraseRightScreen
	o.Screen.drawRightScreen()
	p.mode = NORMAL

	o.command = ""
	o.mode = NORMAL
}

func (o *Organizer) verticalResize(pos int) {
	//pos := strings.LastIndex(o.command_line, " ")
	opt := o.command_line[pos+1:]
	width, err := strconv.Atoi(opt)

	if opt[0] == '+' || opt[0] == '-' {
		width = o.Screen.screenCols - o.Screen.divider - width
	}

	if err != nil {
		o.ShowMessage(BL, "The format is :vert[ical] res[ize] N")
		return
	}
	app.moveDividerAbs(width)
	o.mode = o.last_mode
}

/*
func (o *Organizer) verticalResize__(pos int) {
	if pos == -1 {
		sess.showOrgMessage("You need to provide a number 0 - 100")
		return
	}
	pct, err := strconv.Atoi(o.command_line[pos+1:])
	if err != nil {
		sess.showOrgMessage("You need to provide a number 0 - 100")
		//o.mode = NORMAL
		o.mode = o.last_mode
		return
	}
	moveDividerPct(pct)
	//o.mode = NORMAL
	o.mode = o.last_mode
}
*/

func (o *Organizer) newEntry(unused int) {
	row := Row{
		id: -1,
		//title:    " ",
		star:  false,
		dirty: true,
		sort:  time.Now().Format("3:04:05 pm"), //correct whether added, created, modified are the sort
	}

	vim.BufferSetLines(o.vbuf, 0, 0, []string{""}, 1)
	o.rows = append(o.rows, Row{})
	copy(o.rows[1:], o.rows[0:])
	o.rows[0] = row

	o.fc, o.fr, o.rowoff = 0, 0, 0
	o.command = ""
	//o.repeat = 0
	o.ShowMessage(BL, "\x1b[1m-- INSERT --\x1b[0m")
	o.Screen.eraseRightScreen() //erases the note area
	o.mode = INSERT
	vim.CursorSetPosition(1, 0)
	vim.Input("i")
}

func (o *Organizer) refresh(unused int) {
	if o.view == TASK {
		if o.taskview == BY_FIND {
			o.mode = FIND
			o.fc, o.fr, o.rowoff = 0, 0, 0
			o.rows = o.Database.searchEntries(o.Session.fts_search_terms, o.show_deleted, false)
			if len(o.rows) == 0 {
				o.insertRow(0, "", true, false, false, BASE_DATE)
				o.rows[0].dirty = false
				o.ShowMessage(BL, "No results were returned")
			}
			/*
				if unused != -1 { //complete kluge has to do with refreshing when syncing
					o.drawPreview()
				}
			*/
			o.Session.imagePreview = false
			//o.readTitleIntoBuffer() /////////////////////////////////////////////
			o.readRowsIntoBuffer() ////////////////////////////////////////////
			vim.CursorSetPosition(1, 0)
			o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
			o.drawPreview()
		} else {
			o.mode = o.last_mode
			o.fc, o.fr, o.rowoff = 0, 0, 0
			//o.rows = DB.filterEntries(o.taskview, o.filter, o.show_deleted, o.sort, o.sortPriority, MAX)
	    o.FilterEntries(MAX)
			if len(o.rows) == 0 {
				o.insertRow(0, "", true, false, false, BASE_DATE)
				o.rows[0].dirty = false
				o.ShowMessage(BL, "No results were returned")
			}
			o.Session.imagePreview = false
			//o.readTitleIntoBuffer() /////////////////////////////////////////////
			o.readRowsIntoBuffer() ////////////////////////////////////////////
			vim.CursorSetPosition(1, 0)
			o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
			o.drawPreview()
		}
		//sess.showOrgMessage("Entries will be refreshed")
	} else {
		o.mode = o.last_mode
		o.Database.getContainers()
		if len(o.rows) == 0 {
			o.insertRow(0, "", true, false, false, BASE_DATE)
			o.rows[0].dirty = false
			o.ShowMessage(BL, "No results were returned")
		}
		o.readRowsIntoBuffer() ////////////////////////////////////////////
		vim.CursorSetPosition(1, 0)
		o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
		if unused != -1 {
			o.displayContainerInfo()
		}
		o.ShowMessage(BL, "view refreshed")
	}
	o.clearMarkedEntries()
}

func (o *Organizer) find(pos int) {

	if pos == -1 {
		o.ShowMessage(BL, "You did not enter something to find!")
		o.mode = o.last_mode
		return
	}

	searchTerms := strings.ToLower(o.command_line[pos+1:])
	o.Session.fts_search_terms = searchTerms
	if len(searchTerms) < 3 {
		o.ShowMessage(BL, "You need to provide at least 3 characters to search on")
		return
	}

	o.filter = ""
	o.taskview = BY_FIND
	o.view = TASK
	o.mode = FIND
	o.fc, o.fr, o.rowoff = 0, 0, 0

	o.ShowMessage(BL, "Search for '%s'", searchTerms)
	o.rows = o.Database.searchEntries(searchTerms, o.show_deleted, false)
	if len(o.rows) == 0 {
		o.insertRow(0, "", true, false, false, BASE_DATE)
		o.rows[0].dirty = false
	}
	o.Session.imagePreview = false
	o.readRowsIntoBuffer() ////////////////////////////////////////////
	vim.CursorSetPosition(1, 0)
	o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
	o.drawPreview()
}

func (o *Organizer) sync3(unused int) {
	var log string
	var err error
	if o.command_line == "test" {
		// true => reportOnly
		log = app.Synchronize(true) //Synchronize should return an error:wa
    err = nil //FIXME
	} else {
		log = app.Synchronize(false)
    err = nil //FIXME
	}
	
	if err != nil {
		o.ShowMessage(BL, "Synchronization error: %v", err)
		return
	}
	o.command_line = ""
	o.Screen.eraseRightScreen()
	note := generateWWString(log, o.Screen.totaleditorcols)
	// below draw log as markeup
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("darkslz.json"),
		glamour.WithWordWrap(0),
	)
	note, _ = r.Render(note)
	note = strings.TrimSpace(note)
	note = strings.ReplaceAll(note, "^^^", "\n") ///////////////04072022
	//headings seem to place \x1b[0m after the return
	note = strings.ReplaceAll(note, "\n\x1b[0m", "\x1b[0m\n")
	note = strings.ReplaceAll(note, "\n\n\n", "\n\n")
	o.note = strings.Split(note, "\n")
	o.altRowoff = 0
	o.drawPreviewWithoutImages()
	o.mode = PREVIEW_SYNC_LOG
}

func (o *Organizer) initialBulkLoad(unused int) {
	var log string
	if o.command_line == "bulktest" {
		// true => reportOnly
		log = bulkLoad(true)
	} else {
		log = bulkLoad(false)
	}
	o.command_line = ""
	o.Screen.eraseRightScreen()
	note := generateWWString(log, o.Screen.totaleditorcols)
	// below draw log as markeup
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("darkslz.json"),
		glamour.WithWordWrap(0),
	)
	note, _ = r.Render(note)
	if note[0] == '\n' {
		note = note[1:]
	}
	o.note = strings.Split(note, "\n")
	o.altRowoff = 0
	o.drawPreviewWithoutImages()
	o.mode = PREVIEW_SYNC_LOG
}

func (o *Organizer) reverse(unused int) {
	var log string
	if o.command_line == "reversetest" {
		// true => reportOnly
		log = reverseBulkLoad(true)
	} else {
		log = reverseBulkLoad(false)
	}
	o.command_line = ""
	o.Screen.eraseRightScreen()
	note := generateWWString(log, o.Screen.totaleditorcols)
	// below draw log as markeup
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("darkslz.json"),
		glamour.WithWordWrap(0),
	)
	note, _ = r.Render(note)
	if note[0] == '\n' {
		note = note[1:]
	}
	o.note = strings.Split(note, "\n")
	o.altRowoff = 0
	o.drawPreviewWithoutImages()
	o.mode = PREVIEW_SYNC_LOG
}

func (o *Organizer) contexts(pos int) {
	o.mode = NORMAL

	if pos == -1 {
		o.Screen.eraseRightScreen()
		o.view = CONTEXT
		o.Database.getContainers()
		if len(o.rows) == 0 {
			o.insertRow(0, "", true, false, false, BASE_DATE)
			o.rows[0].dirty = false
			o.ShowMessage(BL, "No results were returned")
		}
		o.readRowsIntoBuffer()
		vim.CursorSetPosition(1, 0)
		o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
		o.displayContainerInfo()
		o.ShowMessage(BL, "Retrieved contexts")
		return
	}

	input := o.command_line[pos+1:]
	var tid int
	var ok bool
	if tid, ok = o.Database.contextExists(input); !ok {
		o.ShowMessage(BL, "%s is not a valid context!", input)
		return
	}
	/*
		for context, folder, and I think keyword - you need to sync a new context etc first
		before you can add a task to it or you'll get a FOREIGN KEY constraint error because
		the task will have a context_tid of [0, -1 ...] and the context tid will be changed
		from that number to the server id and now there is not context tid that matches the task's context_tid
	*/
	if tid < 1 {
		o.showMessage("Context is unsynced")
		return
	}

	if len(o.marked_entries) > 0 {
		for id := range o.marked_entries {
			err := o.Database.updateTaskContextByTid(tid, id) 
      if err != nil {
		    o.showMessage("Error updating context (updateTaskContextByTid) for entry %d to tid %d: %v", id, tid, err)
        return
      }
		}
		o.showMessage("Marked entries moved into context %s", input)
		return
	}
  id := o.rows[o.fr].id
	err := o.Database.updateTaskContextByTid(tid, id)
    if err != nil {
	   o.showMessage("Error updating context (updateTaskContextByTid) for entry %d to tid %d: %v", id, tid, err)
      return
    }
	o.showMessage("Moved current entry (since none were marked) into context %s", input)
}

func (o *Organizer) folders(pos int) {
	o.mode = NORMAL

	if pos == -1 {
		o.Screen.eraseRightScreen()
		o.view = FOLDER
		o.Database.getContainers()

		if len(o.rows) == 0 {
			o.insertRow(0, "", true, false, false, BASE_DATE)
			o.rows[0].dirty = false
			o.ShowMessage(BL, "No results were returned")
		}
		o.readRowsIntoBuffer()
		vim.CursorSetPosition(1, 0)
		o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
		o.displayContainerInfo()
		o.ShowMessage(BL, "Retrieved folders")
		return
	}

	input := o.command_line[pos+1:]
	var ok bool
	var tid int
	if tid, ok = o.Database.folderExists(input); !ok {
		o.ShowMessage(BL, "%s is not a valid folder!", input)
		return
	}

	if tid < 1 {
		o.ShowMessage(BL, "Folder is unsynced")
		return
	}

	if len(o.marked_entries) > 0 {
		for entry_id, _ := range o.marked_entries {
			o.Database.updateTaskFolderByTid(tid, entry_id)
		}
		o.ShowMessage(BL, "Marked entries moved into folder %s", input)
		return
	}
o.Database.updateTaskFolderByTid(tid, o.rows[o.fr].id)
	o.ShowMessage(BL, "Moved current entry (since none were marked) into folder %s", input)
}

func (o *Organizer) keywords(pos int) {

	o.mode = NORMAL

	if pos == -1 {
		o.Screen.eraseRightScreen()
		o.view = KEYWORD
		o.Database.getContainers()

		if len(o.rows) == 0 {
			o.insertRow(0, "", true, false, false, BASE_DATE)
			o.rows[0].dirty = false
			o.ShowMessage(BL, "No results were returned")
		}
		o.readRowsIntoBuffer()
		vim.CursorSetPosition(1, 0)
		o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
		o.displayContainerInfo()
		o.ShowMessage(BL, "Retrieved keywords")
		return
	}

	// not necessary if handled in sync (but not currently handled there)
	if len(o.marked_entries) == 0 && o.Database.entryTidFromId(o.rows[o.fr].id) < 1 {
		o.ShowMessage(BL, "The entry has not been synced yet!")
		o.mode = o.last_mode
		return
	}

	input := o.command_line[pos+1:]
	var ok bool
	var tid int
	if tid, ok = o.Database.keywordExists(input); !ok {
		o.ShowMessage(BL, "%s is not a valid keyword!", input)
		o.mode = o.last_mode
		return
	}

	if tid < 1 {
		o.ShowMessage(BL, "%q is an unsynced keyword!", input)
		o.mode = o.last_mode
		return
	}
	var unsynced []string
	if len(o.marked_entries) > 0 {
		for entry_id, _ := range o.marked_entries {
			// not necessary if handled in sync (but not currently handled there)
			if o.Database.entryTidFromId(entry_id) < 1 {
				unsynced = append(unsynced, strconv.Itoa(entry_id))
				continue
			}
			o.Database.addTaskKeywordByTid(tid, entry_id, true) //true = update fts_dn
		}
		if len(unsynced) > 0 {
			o.ShowMessage(BL, "Added keyword %s to marked entries except for previously unsynced entries: %s", input, strings.Join(unsynced, ", "))
		} else {
			o.ShowMessage(BL, "Added keyword %s to marked entries", input)
		}
		return
	}

	// get here if no marked entries
	o.Database.addTaskKeywordByTid(tid, o.rows[o.fr].id, true)
	o.ShowMessage(BL, "Added keyword %s to current entry (since none were marked)", input)
}

func (o *Organizer) recent(unused int) {
	o.ShowMessage(BL, "Will retrieve recent items")
	o.clearMarkedEntries()
	o.filter = ""
	o.taskview = BY_RECENT
	o.view = TASK
	o.mode = NORMAL
	o.fc, o.fr, o.rowoff = 0, 0, 0
	//o.rows = DB.filterEntries(o.taskview, o.filter, o.show_deleted, o.sort, o.sortPriority, MAX)
  o.FilterEntries(MAX)
	if len(o.rows) == 0 {
		o.insertRow(0, "", true, false, false, BASE_DATE)
		o.rows[0].dirty = false
		o.ShowMessage(BL, "No results were returned")
	}
	o.Session.imagePreview = false
	o.readRowsIntoBuffer()
	vim.CursorSetPosition(1, 0)
	o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
	o.drawPreview()
}

func (o *Organizer) deleteKeywords(unused int) {
	id := o.getId()
	res := o.Database.deleteKeywords(id)
	o.mode = o.last_mode
	if res != -1 {
		o.ShowMessage(BL, "%d keyword(s) deleted from entry %d", res, id)
	}
}

func (o *Organizer) showAll(unused int) {

	if o.view != TASK {
		return
	}
	o.show_deleted = !o.show_deleted
	o.show_completed = !o.show_completed
	o.refresh(0)
	if o.show_deleted {
		o.ShowMessage(BL, "Showing completed/deleted")
	} else {
		o.ShowMessage(BL, "Hiding completed/deleted")
	}
}

func (o *Organizer) updateContainer(unused int) {
	//o.current_task_id = o.rows[o.fr].id
	o.Screen.eraseRightScreen()
	switch o.command_line {
	case "cc":
		o.altView = CONTEXT
	case "ff":
		o.altView = FOLDER
	case "kk":
		o.altView = KEYWORD
	}
	getAltContainers() //O.mode = NORMAL is in get_containers
	if len(o.altRows) != 0 {
		o.mode = ADD_CHANGE_FILTER
		o.ShowMessage(BL, "Select context to add to marked or current entry")
	}
}

func (o *Organizer) deleteMarks(unused int) {
	o.clearMarkedEntries()
	o.mode = NORMAL
	o.command_line = ""
	o.ShowMessage(BL, "Marks cleared")
}

func (o *Organizer) copyEntry(unused int) {
	//copyEntry()
	o.mode = NORMAL
	o.command_line = ""
	o.refresh(0)
	o.ShowMessage(BL, "Entry copied")
}

func (o *Organizer) savelog(unused int) {
	if o.last_mode == PREVIEW_SYNC_LOG {
		title := fmt.Sprintf("%v", time.Now().Format("Mon Jan 2 15:04:05"))
		o.Database.insertSyncEntry(title, strings.Join(o.note, "\n"))
		o.ShowMessage(BL, "Sync log save to database")
		o.command_line = ""
		o.mode = PREVIEW_SYNC_LOG
	} else {
		o.ShowMessage(BL, "There is no sync log to save")
		o.command_line = ""
		o.mode = o.last_mode
	}
}

func (o *Organizer) save(pos int) {
	if pos == -1 {
		o.ShowMessage(BL, "You need to provide a filename")
		return
	}
	filename := o.command_line[pos+1:]
	f, err := os.Create(filename)
	if err != nil {
		o.ShowMessage(BL, "Error creating file %s: %v", filename, err)
		return
	}
	defer f.Close()

	_, err = f.WriteString(strings.Join(o.note, "\n"))
	if err != nil {
		o.ShowMessage(BL, "Error writing file %s: %v", filename, err)
		return
	}
	o.ShowMessage(BL, "Note written to file %s", filename)
}

func (o *Organizer) setImage(pos int) {
	if pos == -1 {
		o.ShowMessage(BL, "You need to provide an option ('on' or 'off')")
		return
	}
	opt := o.command_line[pos+1:]
	if opt == "on" {
		o.Session.imagePreview = true
	} else if opt == "off" {
		o.Session.imagePreview = false
	} else {
		o.ShowMessage(BL, "Your choice of options is 'on' or 'off'")
	}
	o.mode = o.last_mode
	o.drawPreview()
	o.command_line = ""
}

func (o *Organizer) printDocument(unused int) {
	id := o.rows[o.fr].id
	note := DB.readNoteIntoString(id)
	if o.Database.taskFolder(id) == "code" {
		c := o.Database.taskContext(id)
		var ok bool
		var lang string
		if lang, ok = Languages[c]; !ok {
			o.ShowMessage(BL, "I don't recognize the language")
			return
		}
		//note := readNoteIntoString(id)
		var buf bytes.Buffer
		// github seems to work pretty well for printer output
		_ = Highlight(&buf, note, lang, "html", "github")

		f, err := os.Create("output.html")
		if err != nil {
			o.ShowMessage(BL, "Error creating output.html: %v", err)
			return
		}
		defer f.Close()

		_, err = f.WriteString(buf.String())
		if err != nil {
			o.ShowMessage(BL, "Error writing output.html: %s: %v", err)
			return
		}
		cmd := exec.Command("wkhtmltopdf", "--enable-local-file-access",
			"--no-background", "--minimum-font-size", "16", "output.html", "output.pdf")
		err = cmd.Run()
		if err != nil {
			o.ShowMessage(BL, "Error creating pdf from code: %v", err)
		}
	} else {
  
  params := mdtopdf.PdfRendererParams{
      Orientation: "",
      Papersz: "",
      PdfFile: "output.pdf",
      TracerFile: "trace.log",
      Opts: nil,
      Theme: mdtopdf.LIGHT,
  }

	pf := mdtopdf.NewPdfRenderer(params)

		//pf := mdtopdf.NewPdfRenderer("", "", "output.pdf", "trace.log", nil, mdtopdf.LIGHT)
		pf.TBody = mdtopdf.Styler{Font: "Arial", Style: "", Size: 12, Spacing: 2,
			TextColor: mdtopdf.Color{Red: 0, Green: 0, Blue: 0},
			FillColor: mdtopdf.Color{Red: 255, Green: 255, Blue: 255}}

		err := pf.Process([]byte(note))
		if err != nil {
			o.ShowMessage(BL, "pdf error:%v", err)
		}
	}
	cmd := exec.Command("lpr", "output.pdf")
	err := cmd.Run()
	if err != nil {
		o.ShowMessage(BL, "Error printing document: %v", err)
	}
	o.mode = o.last_mode
	o.command_line = ""
}

func (o *Organizer) printList(unused int) {
	var ss []string
	for i, row := range o.rows {
		ss = append(ss, fmt.Sprintf("%2d. %s", i+1, row.title))
	}
	tempBuf := vim.BufferNew(0)
	vim.BufferSetLines(tempBuf, 0, -1, ss, len(ss))
	vim.BufferSetCurrent(tempBuf)
	vim.Execute("ha")

	if p != nil {
		vim.BufferSetCurrent(p.vbuf)
	}
	o.mode = o.last_mode
	o.command_line = ""
}

func (o *Organizer) printList2(unused int) {
	pdf := gofpdf.New("P", "mm", "Letter", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 12)
	curDate := time.Now().Format("January 02, 2006")
	title := fmt.Sprintf("To Do List %s", curDate)
	pdf.CellFormat(190, 1, title, "0", 0, "CM", false, 0, "") //190,7
	var n int
	pageCount := 1
	for i, row := range o.rows {
		line := fmt.Sprintf("%2d. %s", i+1, row.title)
		if row.star {
			pdf.SetFont("Arial", "B", 10)
		} else {
			pdf.SetFont("Arial", "", 10)
		}
		//if i%25 == 0 {
		if pdf.PageCount() != pageCount {
			pageCount += 1
			n = 0
		}
		pdf.SetXY(5, float64(20+n*5))
		pdf.CellFormat(1, 1, line, "0", 0, "L", false, 0, "") // cell format doesn't matter if no border? 7
		n++
	}

	pdf.OutputFileAndClose("output.pdf")
	cmd := exec.Command("lpr", "output.pdf")
	err := cmd.Run()
	if err != nil {
		o.ShowMessage(BL, "Error printing document: %v", err)
	}
	o.mode = o.last_mode
	o.command_line = ""
}

func (o *Organizer) sortEntries(pos int) {
	if pos == -1 {
		o.ShowMessage(BL, "You need to provide a column to sort by")
		return
	}
	sort := o.command_line[pos+1:]
	if _, OK := sortColumns[sort]; OK {
		if sort == "priority" {
			o.sortPriority = !o.sortPriority
		} else {
			o.sort = sort
		}
	} else {
		o.ShowMessage(BL, "The sort columns are modified, added, created and priority")
		return
	}
	o.refresh(0)
	/*
		o.rows = filterEntries(o.taskview, o.filter, o.show_deleted, o.sort, MAX)
		if len(o.rows) == 0 {
			o.insertRow(0, "", true, false, false, BASE_DATE)
			o.rows[0].dirty = false
			sess.showOrgMessage("No results were returned")
		}
		sess.imagePreview = false
		o.readRowsIntoBuffer()
		vim.CursorSetPosition(1, 0)
		o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
		o.drawPreview()
	*/
}
