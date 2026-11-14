package main

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/slzatz/vimango/auth"
	"github.com/slzatz/vimango/internal/pane"
	"github.com/slzatz/vimango/internal/store"
	"github.com/slzatz/vimango/vim"
)

// exArgs is an organizer ex command line, parsed: what a command is handed
// instead of the line. ":set context work" is {name: "set context", arg:
// "work"}; ":rii!" is {name: "rii", bang: true}.
type exArgs struct {
	name string // the command as typed, alias or not, without a trailing "!"
	bang bool   // the name was followed by "!"
	arg  string // the rest of the line, trimmed; "" when there is none
}

// parseEx splits an ex command line into the command and its argument. Two
// commands have a space in their names ("set context", "add keyword"), so the
// first two words are tried as a name before the first one alone; everything
// after the name is the argument, spaces included (":find two words", ":open
// my folder").
func (o *Organizer) parseEx(line string) (exArgs, func(*Organizer, exArgs), bool) {
	words := strings.SplitN(strings.TrimSpace(line), " ", 3)
	for n := min(2, len(words)); n >= 1; n-- {
		ex := exArgs{
			name: strings.Join(words[:n], " "),
			arg:  strings.TrimSpace(strings.Join(words[n:], " ")),
		}
		if cmd, found := o.exCmds[ex.name]; found {
			return ex, cmd, true
		}
		if bare, bang := strings.CutSuffix(ex.name, "!"); bang {
			if cmd, found := o.exCmds[bare]; found {
				ex.name, ex.bang = bare, true
				return ex, cmd, true
			}
		}
	}
	return exArgs{name: strings.TrimSpace(line)}, nil, false
}

func (a *App) setOrganizerExCmds(organizer *Organizer) map[string]func(*Organizer, exArgs) {
	registry := pane.NewCommandRegistry[func(*Organizer, exArgs)]()

	// Navigation commands
	// Note that the name in Register is the command that is recognized.
	registry.Register("open", (*Organizer).open, pane.CommandInfo{
		Aliases:     []string{"o"},
		Description: "Display notes with specified context, folder or keyword (will match in that order)",
		Usage:       "open <name>",
		Category:    "Navigation",
		Examples:    []string{":open work", ":o work"},
	})

	registry.Register("opencontext", (*Organizer).openContext, pane.CommandInfo{
		Aliases:     []string{"oc"},
		Description: "Display notes from specified context",
		Usage:       "opencontext <context_name>",
		Category:    "Navigation",
		Examples:    []string{":opencontext work", ":oc personal"},
	})

	registry.Register("openfolder", (*Organizer).openFolder, pane.CommandInfo{
		Aliases:     []string{"openfolder", "openf", "of"},
		Description: "Display notes from specified folder",
		Usage:       "openfolder <folder_name>",
		Category:    "Navigation",
		Examples:    []string{":openfolder projects", ":of archive"},
	})

	registry.Register("openkeyword", (*Organizer).openKeyword, pane.CommandInfo{
		Aliases:     []string{"openkeyword", "openk", "ok"},
		Description: "Display notes tagged with specified keyword",
		Usage:       "openkeyword <keyword>",
		Category:    "Navigation",
		Examples:    []string{":openkeyword urgent", ":ok meeting"},
	})

	// Data Management commands
	registry.Register("new", (*Organizer).newEntry, pane.CommandInfo{
		Aliases:     []string{"n"},
		Description: "Create new entry",
		Usage:       "new",
		Category:    "Entry Management",
		Examples:    []string{":new", ":n"},
	})

	registry.Register("write", (*Organizer).write, pane.CommandInfo{
		Aliases:     []string{"w"},
		Description: "Save all modified entries to database",
		Usage:       "write",
		Category:    "Entry Management",
		Examples:    []string{":write", ":w"},
	})

	registry.Register("sync", (*Organizer).synchronize, pane.CommandInfo{
		Aliases:     []string{"test", "sync?"},
		Description: "Synchronize with remote server (test or sync? for dry-run)",
		Usage:       "sync",
		Category:    "Data Management",
		Examples:    []string{":sync", ":test (dry-run)", ":sync? (dry-run)"},
	})

	registry.Register("refresh", (*Organizer).refresh, pane.CommandInfo{
		Aliases:     []string{"r"},
		Description: "Refresh current view",
		Usage:       "refresh",
		Category:    "Entry Management",
		Examples:    []string{":refresh", ":r"},
	})

	registry.Register("convertgdrive", (*Organizer).convertGoogleDriveURLs, pane.CommandInfo{
		Aliases:     []string{"cgd"},
		Description: "Convert Google Drive URLs to gdrive:ID format in current note",
		Usage:       "convertgdrive",
		Category:    "Images",
		Examples:    []string{":convertgdrive"},
	})

	// Search & Filter commands
	registry.Register("find", (*Organizer).find, pane.CommandInfo{
		Description: "Search entries using full-text search",
		Usage:       "find <search_terms>",
		Category:    "Search & Filter",
		Examples:    []string{":find meeting notes", ":find urgent todo"},
	})

	registry.Register("contexts", (*Organizer).containers, pane.CommandInfo{
		Aliases:     []string{"c"},
		Description: "show contexts",
		Usage:       "contexts",
		Category:    "Container Management",
		Examples:    []string{":contexts", "c"},
	})

	registry.Register("newc", (*Organizer).newContext, pane.CommandInfo{
		//Aliases:     []string{"c"},
		Description: "show contexts",
		Usage:       "contexts",
		Category:    "Container Management",
		Examples:    []string{":contexts", "c"},
	})

	registry.Register("newf", (*Organizer).newFolder, pane.CommandInfo{
		//Aliases:     []string{"c"},
		Description: "show contexts",
		Usage:       "contexts",
		Category:    "Container Management",
		Examples:    []string{":contexts", "c"},
	})

	registry.Register("folders", (*Organizer).containers, pane.CommandInfo{
		Aliases:     []string{"f"},
		Description: "show folders",
		Usage:       "folders",
		Category:    "Container Management",
		Examples:    []string{":folders", "f"},
	})

	registry.Register("keywords", (*Organizer).containers, pane.CommandInfo{
		Aliases:     []string{"k"},
		Description: "show keywords",
		Usage:       "keywords",
		Category:    "Container Management",
		Examples:    []string{":keywords", "k"},
	})

	registry.Register("set context", (*Organizer).setContext, pane.CommandInfo{
		Aliases:     []string{"set c", "setc", "sc"},
		Description: "Set context for entrie(s)",
		Usage:       "set context <context>",
		Category:    "Entry Management",
		Examples:    []string{":set context work", ":set c work"},
	})

	registry.Register("set folder", (*Organizer).setFolder, pane.CommandInfo{
		Aliases:     []string{"set f", "setf", "sf"},
		Description: "Set folder for entrie(s)",
		Usage:       "set folder <folder>",
		Category:    "Entry Management",
		Examples:    []string{":set folder todo", ":set f todo"},
	})

	registry.Register("add keyword", (*Organizer).addKeyword, pane.CommandInfo{
		Aliases:     []string{"add k"},
		Description: "Add keyword to entry",
		Usage:       "add k <keyword>",
		Category:    "Entry Management",
		Examples:    []string{":add keyword urgent", ":add k urgent"},
	})

	registry.Register("recent", (*Organizer).recent, pane.CommandInfo{
		Description: "Show recently modified entries",
		Usage:       "recent",
		Category:    "Search & Filter",
		Examples:    []string{":recent"},
	})

	// View Management commands
	registry.Register("sort", (*Organizer).sortEntries, pane.CommandInfo{
		Description: "Sort entries by column",
		Usage:       "sort <column>",
		Category:    "View Management",
		Examples:    []string{":sort modified", ":sort created", ":sort priority"},
	})

	registry.Register("showall", (*Organizer).showAll, pane.CommandInfo{
		//Aliases:     []string{"show"},
		Description: "Toggle showing completed/deleted entries",
		Usage:       "showall",
		Category:    "View Management",
		Examples:    []string{":showall", ":show"},
	})

	registry.Register("toggleimages", (*Organizer).toggleImages, pane.CommandInfo{
		Aliases:     []string{"ti"},
		Description: "Toggle inline image display on/off",
		Usage:       "toggleimages",
		Category:    "Images",
		Examples:    []string{":toggleimages", ":ti"},
	})

	registry.Register("showimageinfo", (*Organizer).showImageInfo, pane.CommandInfo{
		Aliases:     []string{"sii"},
		Description: "Toggle display of Google Drive folder/filename above images",
		Usage:       "showimageinfo",
		Category:    "Images",
		Examples:    []string{":showimageinfo", ":sii"},
	})

	registry.Register("refreshimageinfo", (*Organizer).refreshImageInfo, pane.CommandInfo{
		Aliases:     []string{"rii"},
		Description: "Refresh Google Drive image metadata (currently file path) for images in current note. ! also re-download images.",
		Usage:       "refreshimageinfo[!]",
		Category:    "Images",
		Examples:    []string{":refreshimageinfo", ":refreshimageinfo!", ":rii", ":rii!"},
	})

	registry.Register("imagescale", (*Organizer).scaleImages, pane.CommandInfo{
		Aliases:     []string{"is"},
		Description: "Scale inline images up (+), down (-), or to specific size (N columns)",
		Usage:       "imagescale [+|-|N]",
		Category:    "Images",
		Examples:    []string{":imagescale +", ":imagescale -", ":imagescale 30", ":imagescale 60"},
	})

	registry.Register("cachewidth", (*Organizer).cacheWidth, pane.CommandInfo{
		Aliases:     []string{"cw"},
		Description: "Set max pixel width for cached Google Drive images (default 800)",
		Usage:       "cachewidth [N]",
		Category:    "Images",
		Examples:    []string{":cachewidth", ":cachewidth 800", ":cachewidth 1200", ":cw 600"},
	})

	registry.Register("clearcache", (*Organizer).clearImageCache, pane.CommandInfo{
		Aliases:     []string{"clc"},
		Description: "Clear the disk image cache (forces re-download of Google Drive images)",
		Usage:       "clearcache",
		Category:    "Images",
		Examples:    []string{":clearcache", ":clc"},
	})

	registry.Register("imagereset", (*Organizer).imageReset, pane.CommandInfo{
		Aliases:     []string{"ir"},
		Description: "Clear terminal image cache and rerender current note",
		Usage:       "imagereset",
		Category:    "Images",
		Examples:    []string{":imagereset", ":ir"},
	})

	// Entry Management commands
	registry.Register("e", (*Organizer).editNote, pane.CommandInfo{
		Description: "Edit the current note, or the note with the given id",
		Usage:       "e [id]",
		Category:    "Entry Management",
		Examples:    []string{":e", ":e 123"},
	})

	registry.Register("copy", (*Organizer).copyEntry, pane.CommandInfo{
		Description: "Copy current entry",
		Usage:       "copy",
		Category:    "Entry Management",
		Examples:    []string{":copy"},
	})

	registry.Register("deletekeywords", (*Organizer).deleteKeywords, pane.CommandInfo{
		Aliases:     []string{"delkw", "delk"},
		Description: "Delete all keywords from current entry",
		Usage:       "deletekeywords",
		Category:    "Entry Management",
		Examples:    []string{":deletekeywords", ":delkw"},
	})

	registry.Register("deletemarks", (*Organizer).deleteMarks, pane.CommandInfo{
		Aliases:     []string{"delmarks", "delm"},
		Description: "Clear all marked entries",
		Usage:       "deletemarks",
		Category:    "Entry Management",
		Examples:    []string{":deletemarks", ":delm"},
	})

	// Container Management commands
	// System commands
	registry.Register("quit", (*Organizer).quitApp, pane.CommandInfo{
		Aliases:     []string{"q"},
		Description: "Exit application (q! forces quit without saving)",
		Usage:       "quit",
		Category:    "System",
		Examples:    []string{":quit", ":q", ":q!"},
	})

	// Help command
	registry.Register("help", (*Organizer).help, pane.CommandInfo{
		Aliases:     []string{"h"},
		Description: "Show help for commands",
		Usage:       "help [command|category]",
		Category:    "Help",
		Examples:    []string{":help", ":help open", ":help Navigation", ":h"},
	})

	// Store registry in organizer for help command access
	organizer.commandRegistry = registry

	return registry.GetFunctionMap()
}

// help is ":help": every ex command, or what was asked for.
func (o *Organizer) help(ex exArgs) {
	o.mode = HELP
	o.drawNotice(o.helpFor(ex.arg))
	o.altRowoff = 0
	o.command_line = "" // needed because not going into normal mode right away where it is cleared on typing ":"
}

// helpFor is the help text for arg: nothing (every ex command), "normal", an
// ex command, a normal-mode key by its name ("Ctrl-D"), or a category of
// either kind, looked for in that order. Anything else gets the commands with
// names like it.
func (o *Organizer) helpFor(arg string) string {
	ex, normal := o.commandRegistry, o.normalCommandRegistry
	if arg == "" {
		return ex.FormatAllHelp()
	}
	if arg == "normal" {
		return o.formatNormalModeHelp()
	}
	if _, found := ex.GetCommandInfo(arg); found {
		return ex.FormatCommandHelp(arg)
	}
	if normal != nil {
		if key, found := normal.KeyFor(arg); found {
			return normal.FormatCommandHelp(key)
		}
	}
	if _, found := ex.GetAllCommands()[arg]; found {
		return ex.FormatCategoryHelp(arg)
	}
	if normal != nil {
		if _, found := normal.GetAllCommands()[arg]; found {
			return normal.FormatCategoryHelp(arg)
		}
	}

	suggestions := ex.SuggestCommand(arg)
	hint := "Use ':help' to see all available commands."
	if normal != nil {
		suggestions = append(suggestions, normal.SuggestCommand(arg)...)
		hint = "Use ':help' for ex commands or ':help normal' for normal mode commands."
	}
	if len(suggestions) > 0 {
		hint = "Did you mean: " + strings.Join(suggestions, ", ")
	}
	return fmt.Sprintf("Command or category '%s' not found.\n%s", arg, hint)
}

// newContext and newFolder are ":newc" and ":newf": pick a context or folder
// for the current entry from a numbered list.
func (o *Organizer) newContext(ex exArgs) { o.pickContainer(CONTEXT) }
func (o *Organizer) newFolder(ex exArgs)  { o.pickContainer(FOLDER) }

// pickContainer lists every context or folder, numbered, and waits in
// CONTAINER mode for the number of the one to move the current entry into.
func (o *Organizer) pickContainer(view View) {
	o.altView = view
	o.command = ""
	o.command_line = ""

	o.containerList = o.containerTitles(view.kind())
	var sb strings.Builder
	for i, title := range o.containerList {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, title)
	}
	o.mode = CONTAINER
	o.drawNotice(sb.String())
	o.altRowoff = 0
}

// formatNormalModeHelp returns formatted help for all normal mode commands
func (o *Organizer) formatNormalModeHelp() string {
	if o.normalCommandRegistry == nil {
		return "Normal mode help not available"
	}

	var help strings.Builder
	help.WriteString("# Normal Mode Commands:\n\n")

	categories := o.normalCommandRegistry.GetAllCommands()

	// Sort categories for consistent output
	var categoryNames []string
	for category := range categories {
		categoryNames = append(categoryNames, category)
	}
	sort.Strings(categoryNames)

	for _, category := range categoryNames {
		commands := categories[category]
		help.WriteString(fmt.Sprintf("### %s:\n", category))

		for _, cmd := range commands {
			help.WriteString(fmt.Sprintf("- `%-14s`%s\n", cmd.Name, cmd.Description))
		}
		help.WriteString("\n")
	}

	help.WriteString("Use ':help <key>' for detailed help on a specific normal mode command.\n")
	help.WriteString("Use ':help <category>' for commands in a specific category.\n")

	return help.String()
}

func (o *Organizer) openContainerSelection() {
	row := o.rows[o.fr]
	var ok bool
	switch o.view {
	case CONTEXT:
		o.taskview = BY_CONTEXT
		_, ok = o.containerUUID(store.Context, row.title)
	case FOLDER:
		o.taskview = BY_FOLDER
		_, ok = o.containerUUID(store.Folder, row.title)
	case KEYWORD:
		o.taskview = BY_KEYWORD
		// this guard to see if synced may not be necessary for keyword
		_, ok = o.containerUUID(store.Keyword, row.title)
	}

	if !ok {
		o.showMessage("You need to sync before you can use %q", row.title)
		return
	}
	o.filter = row.title
	o.generateNoteList()
}

func (o *Organizer) generateNoteList() {
	o.ShowMessage(BL, "'%s' will be opened", o.filter)
	o.clearMarkedEntries()
	o.view = TASK
	if o.setRows(o.entries()) {
		o.showMessage("No results were returned")
	}
	o.altRowoff = 0
	o.displayNote()
}

func (o *Organizer) open(ex exArgs) {
	if o.view != TASK {
		o.openContainerSelection()
		return
	}
	if ex.arg == "" {
		o.ShowMessage(BL, "You did not provide a context or folder!")
		//o.mode = o.last_mode
		// It might be necessary or prudent to also sendvim an <esc> here
		o.mode = NORMAL
		return
	}
	var ok bool
	input := ex.arg
	if _, ok = o.containerUUID(store.Context, input); ok {
		o.taskview = BY_CONTEXT
	}
	if !ok {
		if _, ok = o.containerUUID(store.Folder, input); ok {
			o.taskview = BY_FOLDER
		}
	}
	if !ok {
		o.ShowMessage(BL, "%s is not a valid context or folder!", input)
		if _, ok = o.containerUUID(store.Keyword, input); ok {
			o.taskview = BY_KEYWORD
		}
	}
	if !ok {
		o.ShowMessage(BL, "%s is not a valid context, folder or keyword!", input)
		o.mode = NORMAL
		return
	}
	o.filter = input
	o.generateNoteList()
	o.mode = NORMAL
	o.command = ""
}

func (o *Organizer) openContext(ex exArgs) {
	if ex.arg == "" {
		o.ShowMessage(BL, "You did not provide a context!")
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}
	input := ex.arg
	if _, ok := o.containerUUID(store.Context, input); !ok {
		o.ShowMessage(BL, "%s is either not a valid context or has not been synced!", input)
		o.mode = NORMAL
		return
	}
	o.taskview = BY_CONTEXT
	o.filter = input
	o.generateNoteList()
	o.mode = NORMAL
	o.command = ""
}

func (o *Organizer) openFolder(ex exArgs) {
	if ex.arg == "" {
		o.ShowMessage(BL, "You did not provide a folder!")
		o.mode = NORMAL
		return
	}
	input := ex.arg
	if _, ok := o.containerUUID(store.Folder, input); !ok {
		o.ShowMessage(BL, "%s is not a valid folder!", input)
		o.mode = NORMAL
		o.command = ""
		return
	}
	o.taskview = BY_FOLDER
	o.filter = input
	o.generateNoteList()
	o.mode = NORMAL
	o.command = ""
}

func (o *Organizer) openKeyword(ex exArgs) {
	if ex.arg == "" {
		o.ShowMessage(BL, "You did not provide a keyword!")
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}
	input := ex.arg
	if _, ok := o.containerUUID(store.Keyword, input); !ok {
		o.ShowMessage(BL, "%s is not a valid keyword!", input)
		//o.mode = o.last_mode
		o.mode = NORMAL
		o.command = ""
		return
	}
	o.taskview = BY_KEYWORD
	o.filter = input
	o.generateNoteList()
	o.mode = NORMAL
	o.command = ""
}

func (o *Organizer) write(ex exArgs) {
	var updated_rows []int
	if o.view != TASK {
		return
	}
	for i := range o.rows {
		// the row itself, not a copy: a new note's id has to land in the list
		row := &o.rows[i]
		if !row.dirty {
			continue
		}
		if row.id != -1 {
			if err := o.Store.SetTitle(row.id, row.title); err != nil {
				o.ShowMessage(BL, "Error updating title: id %d: %v", row.id, err)
				continue
			}
		} else {
			contextUUID, folderUUID := o.newEntryContainers()
			if err := o.addEntry(row, contextUUID, folderUUID); err != nil {
				o.ShowMessage(BL, "Error inserting title %q: %v", row.title, err)
				continue
			}
		}
		row.dirty = false
		updated_rows = append(updated_rows, row.id)
	}
	if len(updated_rows) == 0 {
		o.ShowMessage(BL, "There were no rows to update")
	} else {
		o.ShowMessage(BL, "These ids were updated: %v", updated_rows)
	}
	//o.mode = o.last_mode
	o.mode = NORMAL
	o.command_line = ""
}

func (o *Organizer) quitApp(ex exArgs) {
	if ex.bang {
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

// editNote is ":e": the current note, or ":e 123" for the note with that id.
func (o *Organizer) editNote(ex exArgs) {
	id := -1
	if ex.arg != "" {
		n, err := strconv.Atoi(ex.arg)
		if exists, _ := o.Store.Exists(n); err != nil || !exists {
			o.mode = NORMAL
			o.ShowMessage(BL, "There is no note with id %q", ex.arg)
			return
		}
		id = n
	}
	o.openInEditor(id)
}

// openInEditor opens note id in the editor; -1 is the current row's note.
func (o *Organizer) openInEditor(id int) {
	if o.view != TASK {
		o.command = ""
		//o.mode = o.last_mode
		o.mode = NORMAL
		o.ShowMessage(BL, "Only entries have notes to edit!")
		return
	}

	if id == -1 {
		id = o.getId()
	}
	if id == -1 {
		o.ShowMessage(BL, "You need to save a note's title before you can edit it!")
		o.command = ""
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}

	// One note at a time: the organizer is only reachable once the previous
	// editor has been closed (:q/:x), so there is never one to reuse.
	//
	// The row's title, not the database's: it may carry edits not yet saved.
	title := ""
	if o.fr < len(o.rows) && o.rows[o.fr].id == id {
		title = o.rows[o.fr].title
	}
	if err := app.openEditor(id, title); err != nil {
		o.ShowMessage(BL, "Error retrieving note for id %d: %v", id, err)
	}
	o.command = ""
	o.mode = NORMAL
}

func (o *Organizer) newEntry(ex exArgs) {
	row := Row{
		id:    -1,
		star:  false,
		dirty: true,
		sort:  time.Now().Format("3:04:05 pm"), //correct whether added, created, modified are the sort
	}

	o.vbuf.SetLines(0, 0, []string{""})
	o.rows = append(o.rows, Row{})
	copy(o.rows[1:], o.rows[0:])
	o.rows[0] = row

	o.fc, o.fr, o.rowoff = 0, 0, 0
	o.command = ""
	o.ShowMessage(BL, "\x1b[1m-- INSERT --\x1b[0m")
	o.Screen.eraseRightScreen()
	o.mode = INSERT
	vim.SetCursorPosition(1, 0)
	vim.SendInput("i")
}

// refresh is ":refresh". In a container view it leaves the right pane as it is.
func (o *Organizer) refresh(ex exArgs) {
	o.refreshView(false)
}

// refreshView reloads the rows of the current view from the database. With
// containerInfo, a container view also redraws the first container's details.
func (o *Organizer) refreshView(containerInfo bool) {
	if o.view == TASK {
		if o.taskview == BY_FIND {
			o.mode = NORMAL
			// keep the current rows if the search fails (see find)
			rows, err := o.search(o.Session.fts_search_terms)
			if err != nil {
				o.ShowMessage(BL, "Error searching for %s: %v", o.Session.fts_search_terms, err)
				return
			}
			if o.setRows(rows) {
				o.ShowMessage(BL, "No results were returned")
			}
			o.displayNote()
		} else {
			o.mode = NORMAL
			if o.setRows(o.entries()) {
				o.ShowMessage(BL, "No results were returned")
			}
			o.ShowMessage(BL, "View refreshed")
			o.displayNote()
		}
	} else {
		o.mode = NORMAL
		o.sort = "modified" //It's actually sorted by alpha but displays the modified field
		if o.setRows(o.containerRows()) {
			o.ShowMessage(BL, "No results were returned")
		}
		o.filter = ""
		if containerInfo {
			o.displayContainerInfo()
		}
		o.ShowMessage(BL, "view refreshed")
	}
	o.clearMarkedEntries()
}

func (o *Organizer) find(ex exArgs) {
	if ex.arg == "" {
		o.ShowMessage(BL, "You did not enter something to find!")
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}

	searchTerms := strings.ToLower(ex.arg)
	o.mode = NORMAL
	if len(searchTerms) < 3 {
		o.ShowMessage(BL, "You need to provide at least 3 characters to search on")
		return
	}

	// Search first and change the view only once there are results to show:
	// a search that fails (a query FTS5 cannot parse, say) must leave the
	// list as it was. It used to empty o.rows, and the next key then indexed
	// into nothing and took the app down.
	rows, err := o.search(searchTerms)
	if err != nil {
		o.ShowMessage(BL, "Error searching for %s: %v", searchTerms, err)
		return
	}

	o.Session.fts_search_terms = searchTerms
	o.filter = ""
	o.taskview = BY_FIND
	o.view = TASK
	o.ShowMessage(BL, "Search for '%s'", searchTerms)
	o.setRows(rows)
	o.displayNote()
}

func (o *Organizer) synchronize(ex exArgs) {
	var log string
	var err error
	if ex.name == "test" || ex.name == "sync?" {
		// true => reportOnly
		log = app.Synchronize(true) //Synchronize should return an error
		err = nil                   //FIXME
	} else {
		log = app.Synchronize(false)
		err = nil //FIXME
	}

	if err != nil {
		o.ShowMessage(BL, "Synchronization error: %v", err)
		return
	}
	o.command_line = "" // needed because not going to normal mode right away where it is cleared on typing ":"
	o.drawNotice(log)
	o.altRowoff = 0
	o.mode = NAVIGATE_NOTICE
}

func (o *Organizer) containers(ex exArgs) {
	cmd := ex.name
	var containerType string
	if cmd == "contexts" || cmd == "c" {
		o.view = CONTEXT
		containerType = "contexts"
	} else if cmd == "folders" || cmd == "f" {
		o.view = FOLDER
		containerType = "folders"
	} else { //if cmd == "keywords" || cmd == "k" {
		o.view = KEYWORD
		containerType = "keywords"
	}
	o.mode = NORMAL

	o.command_line = ""

	o.Screen.eraseRightScreen()
	o.sort = "modified" //It's actually sorted by alpha but displays the modified field
	if o.setRows(o.containerRows()) {
		o.ShowMessage(BL, "No results were returned")
	}
	o.filter = ""
	o.displayContainerInfo()
	o.ShowMessage(BL, "Retrieved %s", containerType)
}

// setContext is ":set context <name>"; with no name, the default context.
func (o *Organizer) setContext(ex exArgs) {
	o.mode = NORMAL
	o.moveToContext(cmp.Or(ex.arg, "none"))
}

// setFolder is ":set folder <name>"; with no name, the default folder.
func (o *Organizer) setFolder(ex exArgs) {
	o.mode = NORMAL
	o.moveToFolder(cmp.Or(ex.arg, "none"))
}

func (o *Organizer) addKeyword(ex exArgs) {
	o.mode = NORMAL
	input := ex.arg
	if input == "" {
		o.ShowMessage(BL, "You did not provide a keyword!")
		return
	}
	var ok bool
	var keywordUUID string
	if keywordUUID, ok = o.containerUUID(store.Keyword, input); !ok {
		o.ShowMessage(BL, "%s is not a valid keyword!", input)
		o.mode = NORMAL
		return
	}

	if len(o.marked_entries) > 0 {
		failed := 0
		var lastErr error
		for entry_id := range o.marked_entries {
			if err := o.Store.AddKeyword(entry_id, keywordUUID); err != nil {
				failed++
				lastErr = err
			}
		}
		if failed > 0 {
			o.ShowMessage(BL, "Keyword %s not added to %d of %d marked entries: %v", input, failed, len(o.marked_entries), lastErr)
			return
		}
		o.ShowMessage(BL, "Added keyword %s to marked entries", input)
		return
	}

	// get here if no marked entries
	if err := o.Store.AddKeyword(o.rows[o.fr].id, keywordUUID); err != nil {
		o.ShowMessage(BL, "Keyword %s not added: %v", input, err)
		return
	}
	o.ShowMessage(BL, "Added keyword %s to current entry", input)
}

func (o *Organizer) recent(ex exArgs) {
	o.ShowMessage(BL, "Will retrieve recent items")
	o.clearMarkedEntries()
	o.filter = ""
	o.taskview = BY_RECENT
	o.view = TASK
	o.mode = NORMAL
	if o.setRows(o.entries()) {
		o.ShowMessage(BL, "No results were returned")
	}
	o.displayNote()
}

func (o *Organizer) deleteKeywords(ex exArgs) {
	id := o.getId()
	o.mode = NORMAL
	n, err := o.Store.RemoveKeywords(id)
	if err != nil {
		o.ShowMessage(BL, "Keywords not deleted from entry %d: %v", id, err)
		return
	}
	o.ShowMessage(BL, "%d keyword(s) deleted from entry %d", n, id)
}

func (o *Organizer) showAll(ex exArgs) {

	if o.view != TASK {
		return
	}
	o.show_deleted = !o.show_deleted
	o.refreshView(true)
	if o.show_deleted {
		o.ShowMessage(BL, "Showing completed/deleted")
	} else {
		o.ShowMessage(BL, "Hiding completed/deleted")
	}
}

func (o *Organizer) deleteMarks(ex exArgs) {
	o.clearMarkedEntries()
	o.mode = NORMAL
	o.command_line = ""
	o.ShowMessage(BL, "Marks cleared")
}

func (o *Organizer) copyEntry(ex exArgs) {
	// Validate we're in task view with entries
	if o.view != TASK || len(o.rows) == 0 {
		o.ShowMessage(BL, "No entry to copy")
		o.mode = NORMAL
		o.command_line = ""
		return
	}

	currentRow := o.rows[o.fr]
	if currentRow.id == -1 {
		o.ShowMessage(BL, "Cannot copy unsaved entry")
		o.mode = NORMAL
		o.command_line = ""
		return
	}

	// Copy the entry in the database
	newID, err := o.Store.Copy(currentRow.id)
	if err != nil {
		o.ShowMessage(BL, "Error copying entry: %v", err)
		o.mode = NORMAL
		o.command_line = ""
		return
	}

	o.mode = NORMAL
	o.command_line = ""
	o.refreshView(true) // Refresh to show the new entry
	o.ShowMessage(BL, "Entry copied (new id: %d)", newID)
}

func (o *Organizer) sortEntries(ex exArgs) {
	if ex.arg == "" {
		o.ShowMessage(BL, "You need to provide a column to sort by")
		return
	}
	sort := ex.arg
	if _, OK := sortColumns[sort]; OK {
		if sort == "priority" {
			o.sortPriority = !o.sortPriority
		} else {
			o.sort = sort
		}
	} else {
		o.ShowMessage(BL, "The sort columns are modified, added, and priority")
		return
	}
	o.refreshView(true)
}

// toggleImages toggles inline image display on/off
func (o *Organizer) toggleImages(ex exArgs) {
	o.mode = NORMAL
	o.command_line = ""

	app.showImages = !app.showImages

	status := "OFF"
	if app.showImages {
		status = "ON"
	}

	o.ShowMessage(BL, "Images: %s", status)
	o.displayNote()
}

// showImageInfo toggles display of Google Drive folder/filename above images
func (o *Organizer) showImageInfo(ex exArgs) {
	o.mode = NORMAL
	o.command_line = ""

	app.showImageInfo = !app.showImageInfo

	status := "OFF"
	if app.showImageInfo {
		status = "ON"
	}

	o.ShowMessage(BL, "Image info: %s", status)
	o.displayNote()
}

// refreshImageInfo refreshes Google Drive metadata for images in the current note
// With ! suffix, also re-downloads the image bytes
func (o *Organizer) refreshImageInfo(ex exArgs) {
	// Check for ! suffix (force re-download)
	redownload := ex.bang

	o.mode = NORMAL
	o.command_line = ""

	// Get the current entry's note
	if len(o.rows) == 0 {
		o.ShowMessage(BL, "No entry selected")
		return
	}
	entry := o.rows[o.fr]
	note := o.noteText(entry.id)

	// Extract image URLs from the note
	urls := extractImageURLs(note)
	if len(urls) == 0 {
		o.ShowMessage(BL, "No images in current note")
		return
	}

	// Count Google Drive images
	gdriveCount := 0
	for _, url := range urls {
		if strings.HasPrefix(url, "gdrive:") || strings.Contains(url, "drive.google.com") {
			gdriveCount++
		}
	}

	if gdriveCount == 0 {
		o.ShowMessage(BL, "No Google Drive images in current note")
		return
	}

	o.ShowMessage(BL, "Refreshing metadata for %d image(s)...", gdriveCount)

	// Process each Google Drive image
	refreshed := 0
	for _, url := range urls {
		if !strings.HasPrefix(url, "gdrive:") && !strings.Contains(url, "drive.google.com") {
			continue
		}

		fileID, err := ExtractFileID(url)
		if err != nil {
			continue
		}

		// Fetch fresh metadata from Google Drive
		if app.Session.googleDrive != nil {
			fileName, folderName, err := auth.GetGDriveFileInfo(app.Session.googleDrive, fileID)
			if err == nil && globalImageCache != nil {
				_ = globalImageCache.UpdateGDriveMeta(url, fileName, folderName)
				refreshed++
			}
		}

		// If ! suffix, also invalidate the cached image to force re-download
		if redownload && globalImageCache != nil {
			// Remove from cache - it will be re-downloaded on next render
			globalImageCache.InvalidateCacheEntry(url)
		}
	}

	action := "metadata"
	if redownload {
		action = "metadata + images"
	}
	o.ShowMessage(BL, "Refreshed %s for %d image(s)", action, refreshed)
	o.displayNote()
}

// scaleImages changes the image scale (width in columns)
func (o *Organizer) scaleImages(ex exArgs) {
	// Parse argument from command line
	var argStr string
	if ex.arg != "" {
		argStr = strings.TrimSpace(ex.arg)
	}

	o.mode = NORMAL
	o.command_line = ""

	if argStr == "" {
		o.ShowMessage(BL, "Current image scale: %d columns", app.imageScale)
		return
	}

	var newScale int
	var err error

	switch argStr {
	case "+":
		newScale = app.imageScale + 5
	case "-":
		newScale = app.imageScale - 5
	default:
		newScale, err = strconv.Atoi(argStr)
		if err != nil {
			o.ShowMessage(BL, "Invalid scale value: %s (use +, -, or number)", argStr)
			return
		}
	}

	// Validate range
	if newScale < 10 {
		o.ShowMessage(BL, "Image scale too small (minimum: 10 columns)")
		return
	}
	if newScale > 100 {
		o.ShowMessage(BL, "Image scale too large (maximum: 100 columns)")
		return
	}

	// Update scale
	app.imageScale = newScale

	// Save preferences
	if err := app.SavePreferences(); err != nil {
		// Silently ignore save errors (preferences not critical)
	}

	// Delete all existing kitty images (they're at the old scale)
	// Next render will transmit at the new scale
	deleteAllKittyImages()

	o.ShowMessage(BL, "Image scale: %d columns", app.imageScale)
	o.displayNote()
}

// cacheWidth sets or displays the max pixel width for cached Google Drive images
func (o *Organizer) cacheWidth(ex exArgs) {
	var argStr string
	if ex.arg != "" {
		argStr = strings.TrimSpace(ex.arg)
	}

	o.mode = NORMAL
	o.command_line = ""

	if argStr == "" {
		o.ShowMessage(BL, "Cache max width: %d pixels", app.imageCacheMaxWidth)
		return
	}

	newWidth, err := strconv.Atoi(argStr)
	if err != nil {
		o.ShowMessage(BL, "Invalid width value: %s (use a number)", argStr)
		return
	}

	// Validate range (same as in LoadPreferences)
	if newWidth < 400 {
		o.ShowMessage(BL, "Cache width too small (minimum: 400 pixels)")
		return
	}
	if newWidth > 2000 {
		o.ShowMessage(BL, "Cache width too large (maximum: 2000 pixels)")
		return
	}

	// Update width
	app.imageCacheMaxWidth = newWidth

	// Save preferences
	if err := app.SavePreferences(); err != nil {
		// Silently ignore save errors (preferences not critical)
	}

	o.ShowMessage(BL, "Cache max width: %d pixels (new images will use this size)", app.imageCacheMaxWidth)
}

// clearImageCache removes all files from the disk image cache
func (o *Organizer) clearImageCache(ex exArgs) {
	o.mode = NORMAL
	o.command_line = ""

	if globalImageCache == nil {
		o.ShowMessage(BL, "Image cache not initialized")
		return
	}

	// Get stats before clearing
	count, size := globalImageCache.GetCacheStats()

	// Clear the cache directory
	cacheDir := "./image_cache"
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		o.ShowMessage(BL, "Error reading cache directory: %v", err)
		return
	}

	var removed int
	for _, entry := range entries {
		if entry.Name() == "cache_index.json" || entry.Name() == ".write_test" {
			continue // Keep the index file structure
		}
		path := filepath.Join(cacheDir, entry.Name())
		if err := os.Remove(path); err == nil {
			removed++
		}
	}

	// Reset the cache index
	globalImageCache.mutex.Lock()
	globalImageCache.index.Entries = make(map[string]CacheEntry)
	globalImageCache.saveIndex()
	globalImageCache.mutex.Unlock()

	o.ShowMessage(BL, "Cleared image cache: %d files removed (%.1f MB freed)", count, float64(size)/(1024*1024))
}

// imageReset clears terminal graphics cache and local caches, then rerenders current note.
// Works with any terminal supporting the kitty graphics protocol (kitty, ghostty, etc.)
func (o *Organizer) imageReset(ex exArgs) {
	deleteAllKittyImages()
	kittySessionImageMux.Lock()
	kittySessionImages = make(map[uint32]kittySessionEntry)
	kittySessionImageMux.Unlock()
	kittyIDMap = make(map[string]uint32)
	kittyIDReverse = make(map[uint32]string)
	kittyIDNext = 1
	sent := kittyImagesSent
	bytes := kittyBytesSent
	kittyImagesSent = 0
	kittyBytesSent = 0
	mb := float64(bytes) / (1024 * 1024)
	o.ShowMessage(BL, "Terminal images cleared; since last reset: %d images, %.2f MB sent", sent, mb)
	o.displayNote()
}

// convertGoogleDriveURLs converts Google Drive URLs to gdrive:ID format in the current note
func (o *Organizer) convertGoogleDriveURLs(ex exArgs) {
	if o.view != TASK || len(o.rows) == 0 {
		o.ShowMessage(BL, "No entry selected")
		return
	}

	currentRow := o.rows[o.fr]
	if currentRow.id == -1 {
		o.ShowMessage(BL, "Cannot convert unsaved entries")
		return
	}

	// Read the current entry's note content
	currentNote := o.noteText(currentRow.id)

	// Convert URLs
	convertedNote, count := ConvertGoogleDriveURLsToShort(currentNote)

	if count == 0 {
		o.ShowMessage(BL, "No Google Drive URLs found to convert")
		return
	}

	// Update the note in database
	err := o.Store.SetNote(currentRow.id, convertedNote)
	if err != nil {
		o.ShowMessage(BL, "Error updating note: %v", err)
		return
	}

	// Update hasImage flag if needed
	hasImage := containsGoogleDriveImage(convertedNote)
	if hasImage != currentRow.hasImage {
		o.rows[o.fr].hasImage = hasImage
	}

	// Refresh display to show updated note
	o.displayNote()
	o.ShowMessage(BL, "Converted %d Google Drive URL(s) to gdrive: format", count)
}
