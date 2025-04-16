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
	o.AppUI.eraseRightScreen()
	if len(o.rows) == 0 {
		sess.showOrgMessage("%sThere are no saved sync logs%s", BOLD, RESET)
		return
	}
	note := o.Database.readSyncLog(o.rows[o.fr].id)
	o.note = strings.Split(note, "\n")
	o.drawPreviewWithoutImages()
	o.clearMarkedEntries()
	o.AppUI.showOrgMessage("")
}

func (o *Organizer) open(pos int) {
	if pos == -1 {
		sess.showOrgMessage("You did not provide a context or folder!")
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
		sess.showOrgMessage("%s is not a valid context or folder!", input)
		o.mode = o.last_mode
		return
	}

	if tid < 1 {
		sess.showOrgMessage("%q is an unsynced context or folder!", input)
		o.mode = o.last_mode
		return
	}

	o.filter = input
	sess.showOrgMessage("'%s' will be opened", o.filter)

	o.clearMarkedEntries()
	o.view = TASK
	o.mode = NORMAL
	o.fc, o.fr, o.rowoff = 0, 0, 0
	//o.rows = DB.filterEntries(o.taskview, o.filter, o.show_deleted, o.sort, o.sortPriority, MAX)
	o.FilterEntries(MAX)
	if len(o.rows) == 0 {
		o.insertRow(0, "", true, false, false, BASE_DATE)
		o.rows[0].dirty = false
		sess.showOrgMessage("No results were returned")
	}
	sess.imagePreview = false
	o.readRowsIntoBuffer()
	vim.CursorSetPosition(1, 0)
	o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
	o.altRowoff = 0
	o.drawPreview()
	return
}

func (o *Organizer) openContext(pos int) {
	if pos == -1 {
		sess.showOrgMessage("You did not provide a context!")
		o.mode = o.last_mode
		return
	}

	input := o.command_line[pos+1:]
	var tid int
	var ok bool
	if tid, ok = o.Database.contextExists(input); !ok {
		sess.showOrgMessage("%s is not a valid context!", input)
		o.mode = o.last_mode
		return
	}
	if tid < 1 {
		sess.showOrgMessage("%q is an unsynced context!", input)
		o.mode = o.last_mode
		return
	}

	o.filter = input

	sess.showOrgMessage("'%s' will be opened", o.filter)

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
		sess.showOrgMessage("No results were returned")
	}
	sess.imagePreview = false
	o.readRowsIntoBuffer()
	vim.CursorSetPosition(1, 0)
	o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
	o.altRowoff = 0
	o.drawPreview()
	return
}

func (o *Organizer) openFolder(pos int) {
	if pos == -1 {
		sess.showOrgMessage("You did not provide a folder!")
		o.mode = o.last_mode
		return
	}

	input := o.command_line[pos+1:]
	var ok bool
	var tid int
	if tid, ok = o.Database.folderExists(input); !ok {
		sess.showOrgMessage("%s is not a valid folder!", input)
		o.mode = o.last_mode
		return
	}
	if tid < 1 {
		sess.showOrgMessage("%q is an unsynced folder!", input)
		o.mode = o.last_mode
		return
	}

	o.filter = input

	sess.showOrgMessage("'%s' will be opened", o.filter)

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
		sess.showOrgMessage("No results were returned")
	}
	sess.imagePreview = false
	o.readRowsIntoBuffer()
	vim.CursorSetPosition(1, 0)
	o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
	o.altRowoff = 0
	o.drawPreview()
	return
}

func (o *Organizer) openKeyword(pos int) {
	if pos == -1 {
		sess.showOrgMessage("You did not provide a keyword!")
		o.mode = o.last_mode
		return
	}
	input := o.command_line[pos+1:]
	var ok bool
	var tid int
	if tid, ok = o.Database.keywordExists(input); !ok {
		sess.showOrgMessage("%s is not a valid keyword!", input)
		o.mode = o.last_mode
		return
	}
	// this guard may not be necessary for keywords
	if tid < 1 {
		sess.showOrgMessage("%q is an unsynced keyword!", input)
		o.mode = o.last_mode
		return
	}

	o.filter = input

	sess.showOrgMessage("'%s' will be opened", o.filter)

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
		sess.showOrgMessage("No results were returned")
	}
	sess.imagePreview = false
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
  max_length := o.AppUI.PositionMessage(BL)
	if len(updated_rows) == 0 {
	  o.ShowMessage(max_length, "There were no rows to update")
	} else {
	  o.ShowMessage(max_length, "These ids were updated: %v", updated_rows)
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
		sess.showOrgMessage("No db write since last change")
	} else {
		app.Run = false
	}
}

func (o *Organizer) editNote(id int) {

	if o.view != TASK {
		o.command = ""
		o.mode = o.last_mode
		sess.showOrgMessage("Only entries have notes to edit!")
		return
	}

	//pos is zero if no space and command modifier
	if id == -1 {
		id = o.getId()
	}
	if id == -1 {
		sess.showOrgMessage("You need to save item before you can create a note")
		o.command = ""
		o.mode = o.last_mode
		return
	}

	//sess.showOrgMessage("Edit note %d", id)
	sess.editorMode = true

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

	sess.positionWindows()
	sess.eraseRightScreen() //erases editor area + statusbar + msg
	//delete any images
	//fmt.Print("\x1b_Ga=d\x1b\\") //now in sess.eraseRightScreen
	sess.drawRightScreen()
	p.mode = NORMAL

	o.command = ""
	o.mode = NORMAL
}

func (o *Organizer) verticalResize(pos int) {
	//pos := strings.LastIndex(o.command_line, " ")
	opt := o.command_line[pos+1:]
	width, err := strconv.Atoi(opt)

	if opt[0] == '+' || opt[0] == '-' {
		width = sess.screenCols - sess.divider - width
	}

	if err != nil {
		sess.showEdMessage("The format is :vert[ical] res[ize] N")
		return
	}
	moveDividerAbs(width)
	//sess.cfg.ed_pct = 100 * width / sess.screenCols // in moveDividerAbs
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
	sess.showOrgMessage("\x1b[1m-- INSERT --\x1b[0m")
	sess.eraseRightScreen() //erases the note area
	o.mode = INSERT
	vim.CursorSetPosition(1, 0)
	vim.Input("i")
}

func (o *Organizer) refresh(unused int) {
	if o.view == TASK {
		if o.taskview == BY_FIND {
			o.mode = FIND
			o.fc, o.fr, o.rowoff = 0, 0, 0
			o.rows = o.Database.searchEntries(sess.fts_search_terms, o.show_deleted, false)
			if len(o.rows) == 0 {
				o.insertRow(0, "", true, false, false, BASE_DATE)
				o.rows[0].dirty = false
				sess.showOrgMessage("No results were returned")
			}
			/*
				if unused != -1 { //complete kluge has to do with refreshing when syncing
					o.drawPreview()
				}
			*/
			sess.imagePreview = false
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
				sess.showOrgMessage("No results were returned")
			}
			sess.imagePreview = false
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
			sess.showOrgMessage("No results were returned")
		}
		o.readRowsIntoBuffer() ////////////////////////////////////////////
		vim.CursorSetPosition(1, 0)
		o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
		if unused != -1 {
			o.displayContainerInfo()
		}
		o.AppUI.showMessage(BL, "view refreshed")
	}
	o.clearMarkedEntries()
}

func (o *Organizer) find(pos int) {

	if pos == -1 {
		sess.showOrgMessage("You did not enter something to find!")
		o.mode = o.last_mode
		return
	}

	searchTerms := strings.ToLower(o.command_line[pos+1:])
	sess.fts_search_terms = searchTerms
	if len(searchTerms) < 3 {
		sess.showOrgMessage("You need to provide at least 3 characters to search on")
		return
	}

	o.filter = ""
	o.taskview = BY_FIND
	o.view = TASK
	o.mode = FIND
	o.fc, o.fr, o.rowoff = 0, 0, 0

	sess.showOrgMessage("Search for '%s'", searchTerms)
	o.rows = o.Database.searchEntries(searchTerms, o.show_deleted, false)
	if len(o.rows) == 0 {
		o.insertRow(0, "", true, false, false, BASE_DATE)
		o.rows[0].dirty = false
	}
	sess.imagePreview = false
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
		o.AppUI.showOrgMessage("Synchronization error: %v", err)
		return
	}
	o.command_line = ""
	o.AppUI.eraseRightScreen()
	note := generateWWString(log, o.AppUI.totaleditorcols)
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
	o.AppUI.eraseRightScreen()
	note := generateWWString(log, o.AppUI.totaleditorcols)
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
	o.AppUI.eraseRightScreen()
	note := generateWWString(log, o.AppUI.totaleditorcols)
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
		sess.eraseRightScreen()
		o.view = CONTEXT
		o.Database.getContainers()
		if len(o.rows) == 0 {
			o.insertRow(0, "", true, false, false, BASE_DATE)
			o.rows[0].dirty = false
			sess.showOrgMessage("No results were returned")
		}
		o.readRowsIntoBuffer()
		vim.CursorSetPosition(1, 0)
		o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
		o.displayContainerInfo()
		sess.showOrgMessage("Retrieved contexts")
		return
	}

	input := o.command_line[pos+1:]
	var tid int
	var ok bool
	if tid, ok = o.Database.contextExists(input); !ok {
		sess.showOrgMessage("%s is not a valid context!", input)
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
		sess.eraseRightScreen()
		o.view = FOLDER
		o.Database.getContainers()

		if len(o.rows) == 0 {
			o.insertRow(0, "", true, false, false, BASE_DATE)
			o.rows[0].dirty = false
			sess.showOrgMessage("No results were returned")
		}
		o.readRowsIntoBuffer()
		vim.CursorSetPosition(1, 0)
		o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
		o.displayContainerInfo()
		sess.showOrgMessage("Retrieved folders")
		return
	}

	input := o.command_line[pos+1:]
	var ok bool
	var tid int
	if tid, ok = o.Database.folderExists(input); !ok {
		sess.showOrgMessage("%s is not a valid folder!", input)
		return
	}

	if tid < 1 {
		sess.showOrgMessage("Folder is unsynced")
		return
	}

	if len(o.marked_entries) > 0 {
		for entry_id, _ := range o.marked_entries {
			o.Database.updateTaskFolderByTid(tid, entry_id)
		}
		sess.showOrgMessage("Marked entries moved into folder %s", input)
		return
	}
o.Database.updateTaskFolderByTid(tid, o.rows[o.fr].id)
	sess.showOrgMessage("Moved current entry (since none were marked) into folder %s", input)
}

func (o *Organizer) keywords(pos int) {

	o.mode = NORMAL

	if pos == -1 {
		sess.eraseRightScreen()
		o.view = KEYWORD
		o.Database.getContainers()

		if len(o.rows) == 0 {
			o.insertRow(0, "", true, false, false, BASE_DATE)
			o.rows[0].dirty = false
			sess.showOrgMessage("No results were returned")
		}
		o.readRowsIntoBuffer()
		vim.CursorSetPosition(1, 0)
		o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
		o.displayContainerInfo()
		sess.showOrgMessage("Retrieved keywords")
		return
	}

	// not necessary if handled in sync (but not currently handled there)
	if len(o.marked_entries) == 0 && o.Database.entryTidFromId(o.rows[o.fr].id) < 1 {
		sess.showOrgMessage("The entry has not been synced yet!")
		o.mode = o.last_mode
		return
	}

	input := o.command_line[pos+1:]
	var ok bool
	var tid int
	if tid, ok = o.Database.keywordExists(input); !ok {
		sess.showOrgMessage("%s is not a valid keyword!", input)
		o.mode = o.last_mode
		return
	}

	if tid < 1 {
		sess.showOrgMessage("%q is an unsynced keyword!", input)
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
			sess.showOrgMessage("Added keyword %s to marked entries except for previously unsynced entries: %s", input, strings.Join(unsynced, ", "))
		} else {
			sess.showOrgMessage("Added keyword %s to marked entries", input)
		}
		return
	}

	// get here if no marked entries
	o.Database.addTaskKeywordByTid(tid, o.rows[o.fr].id, true)
	sess.showOrgMessage("Added keyword %s to current entry (since none were marked)", input)
}

func (o *Organizer) recent(unused int) {
	sess.showOrgMessage("Will retrieve recent items")
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
		sess.showOrgMessage("No results were returned")
	}
	sess.imagePreview = false
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
		sess.showOrgMessage("%d keyword(s) deleted from entry %d", res, id)
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
		sess.showOrgMessage("Showing completed/deleted")
	} else {
		sess.showOrgMessage("Hiding completed/deleted")
	}
}

func (o *Organizer) updateContainer(unused int) {
	//o.current_task_id = o.rows[o.fr].id
	sess.eraseRightScreen()
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
		sess.showOrgMessage("Select context to add to marked or current entry")
	}
}

func (o *Organizer) deleteMarks(unused int) {
	o.clearMarkedEntries()
	o.mode = NORMAL
	o.command_line = ""
	sess.showOrgMessage("Marks cleared")
}

func (o *Organizer) copyEntry(unused int) {
	//copyEntry()
	o.mode = NORMAL
	o.command_line = ""
	o.refresh(0)
	sess.showOrgMessage("Entry copied")
}

func (o *Organizer) savelog(unused int) {
	if o.last_mode == PREVIEW_SYNC_LOG {
		title := fmt.Sprintf("%v", time.Now().Format("Mon Jan 2 15:04:05"))
		o.Database.insertSyncEntry(title, strings.Join(o.note, "\n"))
		sess.showOrgMessage("Sync log save to database")
		o.command_line = ""
		o.mode = PREVIEW_SYNC_LOG
	} else {
		sess.showOrgMessage("There is no sync log to save")
		o.command_line = ""
		o.mode = o.last_mode
	}
}

func (o *Organizer) save(pos int) {
	if pos == -1 {
		sess.showOrgMessage("You need to provide a filename")
		return
	}
	filename := o.command_line[pos+1:]
	f, err := os.Create(filename)
	if err != nil {
		sess.showOrgMessage("Error creating file %s: %v", filename, err)
		return
	}
	defer f.Close()

	_, err = f.WriteString(strings.Join(o.note, "\n"))
	if err != nil {
		sess.showOrgMessage("Error writing file %s: %v", filename, err)
		return
	}
	sess.showOrgMessage("Note written to file %s", filename)
}

func (o *Organizer) setImage(pos int) {
	if pos == -1 {
		sess.showOrgMessage("You need to provide an option ('on' or 'off')")
		return
	}
	opt := o.command_line[pos+1:]
	if opt == "on" {
		sess.imagePreview = true
	} else if opt == "off" {
		sess.imagePreview = false
	} else {
		sess.showOrgMessage("Your choice of options is 'on' or 'off'")
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
			sess.showOrgMessage("I don't recognize the language")
			return
		}
		//note := readNoteIntoString(id)
		var buf bytes.Buffer
		// github seems to work pretty well for printer output
		_ = Highlight(&buf, note, lang, "html", "github")

		f, err := os.Create("output.html")
		if err != nil {
			sess.showOrgMessage("Error creating output.html: %v", err)
			return
		}
		defer f.Close()

		_, err = f.WriteString(buf.String())
		if err != nil {
			sess.showOrgMessage("Error writing output.html: %s: %v", err)
			return
		}
		cmd := exec.Command("wkhtmltopdf", "--enable-local-file-access",
			"--no-background", "--minimum-font-size", "16", "output.html", "output.pdf")
		err = cmd.Run()
		if err != nil {
			sess.showOrgMessage("Error creating pdf from code: %v", err)
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
			sess.showEdMessage("pdf error:%v", err)
		}
	}
	cmd := exec.Command("lpr", "output.pdf")
	err := cmd.Run()
	if err != nil {
		sess.showEdMessage("Error printing document: %v", err)
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
		sess.showEdMessage("Error printing document: %v", err)
	}
	o.mode = o.last_mode
	o.command_line = ""
}

func (o *Organizer) sortEntries(pos int) {
	if pos == -1 {
		sess.showOrgMessage("You need to provide a column to sort by")
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
		sess.showOrgMessage("The sort columns are modified, added, created and priority")
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
