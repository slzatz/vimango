package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/mandolyte/mdtopdf/v2"
	"github.com/slzatz/vimango/vim"
)

func (a *App) setOrganizerExCmds(organizer *Organizer) map[string]func(*Organizer, int) {
	registry := NewCommandRegistry[func(*Organizer, int)]()

	// Navigation commands
	registry.Register("open", (*Organizer).open, CommandInfo{
		Name:        "open",
		Aliases:     []string{"o", "cd"},
		Description: "Open context, folder, or keyword by name",
		Usage:       "open <name>",
		Category:    "Navigation",
		Examples:    []string{":open work", ":o personal", ":cd project"},
	})

	registry.Register("opencontext", (*Organizer).openContext, CommandInfo{
		Name:        "opencontext",
		Aliases:     []string{"oc"},
		Description: "Open specific context",
		Usage:       "opencontext <context_name>",
		Category:    "Navigation",
		Examples:    []string{":opencontext work", ":oc personal"},
	})

	registry.Register("openfolder", (*Organizer).openFolder, CommandInfo{
		Name:        "openfolder",
		Aliases:     []string{"of"},
		Description: "Open specific folder",
		Usage:       "openfolder <folder_name>",
		Category:    "Navigation",
		Examples:    []string{":openfolder projects", ":of archive"},
	})

	registry.Register("openkeyword", (*Organizer).openKeyword, CommandInfo{
		Name:        "openkeyword",
		Aliases:     []string{"ok"},
		Description: "Open entries with specific keyword",
		Usage:       "openkeyword <keyword>",
		Category:    "Navigation",
		Examples:    []string{":openkeyword urgent", ":ok meeting"},
	})

	// Data Management commands
	registry.Register("new", (*Organizer).newEntry, CommandInfo{
		Name:        "new",
		Aliases:     []string{"n"},
		Description: "Create new entry",
		Usage:       "new",
		Category:    "Data Management",
		Examples:    []string{":new", ":n"},
	})

	registry.Register("write", (*Organizer).write, CommandInfo{
		Name:        "write",
		Aliases:     []string{"w"},
		Description: "Save all modified entries to database",
		Usage:       "write",
		Category:    "Data Management",
		Examples:    []string{":write", ":w"},
	})

	registry.Register("sync", (*Organizer).synchronize, CommandInfo{
		Name:        "sync",
		Aliases:     []string{"test"},
		Description: "Synchronize with remote server (test for dry-run)",
		Usage:       "sync",
		Category:    "Data Management",
		Examples:    []string{":sync", ":test (dry-run)"},
	})

	/*
		registry.Register("bulkload", (*Organizer).initialBulkLoad, CommandInfo{
			Name:        "bulkload",
			Aliases:     []string{"bulktest"},
			Description: "Perform initial bulk data load",
			Usage:       "bulkload",
			Category:    "Data Management",
			Examples:    []string{":bulkload", ":bulktest (dry-run)"},
		})

		registry.Register("reverseload", (*Organizer).reverse, CommandInfo{
			Name:        "reverseload",
			Aliases:     []string{"reversetest"},
			Description: "Perform reverse bulk data load",
			Usage:       "reverseload",
			Category:    "Data Management",
			Examples:    []string{":reverseload", ":reversetest (dry-run)"},
		})
	*/
	registry.Register("refresh", (*Organizer).refresh, CommandInfo{
		Name:        "refresh",
		Aliases:     []string{"r"},
		Description: "Refresh current view",
		Usage:       "refresh",
		Category:    "Data Management",
		Examples:    []string{":refresh", ":r"},
	})

	registry.Register("research", (*Organizer).startResearch, CommandInfo{
		Name:        "research",
		Aliases:     []string{},
		Description: "Start deep research using current note as prompt",
		Usage:       "research",
		Category:    "Data Management",
		Examples:    []string{":research"},
	})

	registry.Register("researchdebug", (*Organizer).startResearchDebug, CommandInfo{
		Name:        "researchdebug",
		Aliases:     []string{"rd"},
		Description: "Start deep research with full debug information",
		Usage:       "researchdebug",
		Category:    "Data Management",
		Examples:    []string{":researchdebug", ":rd"},
	})

	registry.Register("researchtest", (*Organizer).startResearchNotificationTest, CommandInfo{
		Name:        "researchtest",
		Aliases:     []string{"rtest"},
		Description: "Generate sample research status notifications for testing",
		Usage:       "researchtest",
		Category:    "Data Management",
		Examples:    []string{":researchtest", ":rtest"},
	})

	// Search & Filter commands
	registry.Register("find", (*Organizer).find, CommandInfo{
		Name:        "find",
		Description: "Search entries using full-text search",
		Usage:       "find <search_terms>",
		Category:    "Search & Filter",
		Examples:    []string{":find meeting notes", ":find urgent todo"},
	})

	registry.Register("list", (*Organizer).list, CommandInfo{
		Name:        "list",
		Aliases:     []string{"l"},
		Description: "list contexts, folders, or keywords",
		Usage:       "list <context|folder|keyword>",
		Category:    "Container Management",
		Examples:    []string{":list contexts", ":l f"},
	})

	registry.Register("set context", (*Organizer).setContext, CommandInfo{
		Name:        "set context",
		Aliases:     []string{"set c", "mv"},
		Description: "Set context for entrie(s)",
		Usage:       "set context <context_name>",
		Category:    "Entry Management",
		Examples:    []string{":set context work", ":set c work"},
	})

	registry.Register("folders", (*Organizer).setFolder, CommandInfo{
		Name:        "set folder",
		Aliases:     []string{"set f", "mvf"},
		Description: "Set folder for entrie(s)",
		Usage:       "set folder <folder_name>",
		Category:    "Entry Management",
		Examples:    []string{":set folder todo", ":set f todo"},
	})

	registry.Register("keywords", (*Organizer).keywords, CommandInfo{
		Name:        "keywords",
		Aliases:     []string{"keyword", "k"},
		Description: "Show all keywords or add keyword to entries",
		Usage:       "keywords [keyword]",
		Category:    "Search & Filter",
		Examples:    []string{":keywords", ":k urgent", ":keyword meeting"},
	})

	registry.Register("recent", (*Organizer).recent, CommandInfo{
		Name:        "recent",
		Description: "Show recently modified entries",
		Usage:       "recent",
		Category:    "Search & Filter",
		Examples:    []string{":recent"},
	})

	/*
		registry.Register("log", (*Organizer).log, CommandInfo{
			Name:        "log",
			Description: "Show synchronization log entries",
			Usage:       "log",
			Category:    "Search & Filter",
			Examples:    []string{":log"},
		})
	*/
	// View Control commands
	registry.Register("sort", (*Organizer).sortEntries, CommandInfo{
		Name:        "sort",
		Description: "Sort entries by column",
		Usage:       "sort <column>",
		Category:    "View Control",
		Examples:    []string{":sort modified", ":sort created", ":sort priority"},
	})

	registry.Register("showall", (*Organizer).showAll, CommandInfo{
		Name: "showall",
		//Aliases:     []string{"show"},
		Description: "Toggle showing completed/deleted entries",
		Usage:       "showall",
		Category:    "View Control",
		Examples:    []string{":showall", ":show"},
	})

	/*
		registry.Register("image", (*Organizer).setImage, CommandInfo{
			Name:        "image",
			Aliases:     []string{"images"},
			Description: "Toggle image preview on/off",
			Usage:       "image <on|off>",
			Category:    "View Control",
			Examples:    []string{":image on", ":images off"},
		})
	*/

	registry.Register("webview", (*Organizer).showWebView, CommandInfo{
		Name:        "webview",
		Aliases:     []string{"wv"},
		Description: "Show current note in web browser",
		Usage:       "webview",
		Category:    "View Control",
		Examples:    []string{":webview", ":wv"},
	})

	registry.Register("closewebview", (*Organizer).closeWebView, CommandInfo{
		Name:        "closewebview",
		Aliases:     []string{"cwv"},
		Description: "Close webkit webview window",
		Usage:       "closewebview",
		Category:    "View Control",
		Examples:    []string{":closewebview", ":cwv"},
	})

	registry.Register("toggleimages", (*Organizer).toggleImages, CommandInfo{
		Name:        "toggleimages",
		Aliases:     []string{"ti"},
		Description: "Toggle inline image display on/off",
		Usage:       "toggleimages",
		Category:    "View Control",
		Examples:    []string{":toggleimages", ":ti"},
	})

	registry.Register("imagescale", (*Organizer).scaleImages, CommandInfo{
		Name:        "imagescale",
		Aliases:     []string{"is"},
		Description: "Scale inline images up (+), down (-), or to specific size (N columns)",
		Usage:       "imagescale [+|-|N]",
		Category:    "View Control",
		Examples:    []string{":imagescale +", ":imagescale -", ":imagescale 30", ":imagescale 60"},
	})

	registry.Register("kittyreset", (*Organizer).kittyReset, CommandInfo{
		Name:        "kittyreset",
		Aliases:     []string{"kitty-reset"},
		Description: "Clear kitty image cache and rerender current note",
		Usage:       "kittyreset",
		Category:    "View Control",
		Examples:    []string{":kittyreset"},
	})

	registry.Register("vertical resize", (*Organizer).verticalResize, CommandInfo{
		Name:        "vertical resize",
		Aliases:     []string{"vert res"},
		Description: "Resize vertical divider",
		Usage:       "vertical resize <width>",
		Category:    "View Control",
		Examples:    []string{":vertical resize 80", ":vert res +10", ":vert res -5"},
	})

	// Entry Management commands
	registry.Register("e", (*Organizer).editNote, CommandInfo{
		Name:        "e",
		Description: "Edit note for current or specified entry",
		Usage:       "e [entry_id]",
		Category:    "Entry Management",
		Examples:    []string{":e", ":e 123"},
	})

	registry.Register("copy", (*Organizer).copyEntry, CommandInfo{
		Name:        "copy",
		Description: "Copy current entry",
		Usage:       "copy",
		Category:    "Entry Management",
		Examples:    []string{":copy"},
	})

	registry.Register("deletekeywords", (*Organizer).deleteKeywords, CommandInfo{
		Name:        "deletekeywords",
		Aliases:     []string{"delkw", "delk"},
		Description: "Delete all keywords from current entry",
		Usage:       "deletekeywords",
		Category:    "Entry Management",
		Examples:    []string{":deletekeywords", ":delkw"},
	})

	registry.Register("deletemarks", (*Organizer).deleteMarks, CommandInfo{
		Name:        "deletemarks",
		Aliases:     []string{"delmarks", "delm"},
		Description: "Clear all marked entries",
		Usage:       "deletemarks",
		Category:    "Entry Management",
		Examples:    []string{":deletemarks", ":delm"},
	})

	// Container Management commands
	/*
			registry.Register("cc", (*Organizer).updateContainer, CommandInfo{
				Name:        "cc",
				Description: "Update context for marked or current entry",
				Usage:       "cc",
				Category:    "Container Management",
				Examples:    []string{":cc"},
			})

		registry.Register("ff", (*Organizer).updateContainer, CommandInfo{
			Name:        "ff",
			Description: "Update folder for marked or current entry",
			Usage:       "ff",
			Category:    "Container Management",
			Examples:    []string{":ff"},
		})

		registry.Register("kk", (*Organizer).updateContainer, CommandInfo{
			Name:        "kk",
			Description: "Update keyword for marked or current entry",
			Usage:       "kk",
			Category:    "Container Management",
			Examples:    []string{":kk"},
		})
	*/
	// Output & Export commands
	registry.Register("print", (*Organizer).printDocument, CommandInfo{
		Name:        "print",
		Description: "Print current note as PDF",
		Usage:       "print",
		Category:    "Output & Export",
		Examples:    []string{":print"},
	})

	registry.Register("ha", (*Organizer).printList, CommandInfo{
		Name:        "ha",
		Description: "Print current list using vim hardcopy",
		Usage:       "ha",
		Category:    "Output & Export",
		Examples:    []string{":ha"},
	})

	registry.Register("printlist", (*Organizer).printList2, CommandInfo{
		Name:        "printlist",
		Aliases:     []string{"ha2", "pl"},
		Description: "Print formatted list as PDF",
		Usage:       "printlist",
		Category:    "Output & Export",
		Examples:    []string{":printlist", ":pl"},
	})

	registry.Register("save", (*Organizer).save, CommandInfo{
		Name:        "save",
		Description: "Save current note to file",
		Usage:       "save <filename>",
		Category:    "Output & Export",
		Examples:    []string{":save note.txt", ":save /tmp/backup.md"},
	})

	/*
		registry.Register("savelog", (*Organizer).savelog, CommandInfo{
			Name:        "savelog",
			Description: "Save current sync log to database",
			Usage:       "savelog",
			Category:    "Output & Export",
			Examples:    []string{":savelog"},
		})
	*/
	// System commands
	registry.Register("quit", (*Organizer).quitApp, CommandInfo{
		Name:        "quit",
		Aliases:     []string{"q", "q!"},
		Description: "Exit application (q! forces quit without saving)",
		Usage:       "quit",
		Category:    "System",
		Examples:    []string{":quit", ":q", ":q!"},
	})

	registry.Register("which", (*Organizer).whichVim, CommandInfo{
		Name:        "which",
		Description: "Show which vim implementation is active",
		Usage:       "which",
		Category:    "System",
		Examples:    []string{":which"},
	})

	// Help command
	registry.Register("help", (*Organizer).help, CommandInfo{
		Name:        "help",
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

// help displays help information for organizer commands
func (o *Organizer) help(pos int) {
	/*
		if o.commandRegistry == nil {
			o.ShowMessage(BL, "Help system not available")
			o.mode = o.last_mode
			return
		}
	*/

	var helpText string

	if pos == -1 {
		// No arguments - show all ex commands
		helpText = o.commandRegistry.FormatAllHelp()
	} else {
		// Get the argument after "help "
		arg := o.command_line[pos+1:]

		// Check if it's request for normal mode help
		if arg == "normal" {
			if o.normalCommandRegistry != nil {
				helpText = o.formatNormalModeHelp()
			} else {
				helpText = "Normal mode help not available"
			}
		} else if _, exists := o.commandRegistry.GetCommandInfo(arg); exists {
			// Check if it's a specific ex command
			helpText = o.commandRegistry.FormatCommandHelp(arg)
		} else if o.normalCommandRegistry != nil {
			// Check if it's a normal mode command (by display name)
			if normalInfo, exists := o.findNormalCommandByDisplayName(arg); exists {
				helpText = o.normalCommandRegistry.FormatCommandHelp(normalInfo.Name)
			} else {
				// Check if it's a category (ex commands first, then normal)
				exCategories := o.commandRegistry.GetAllCommands()
				if _, exists := exCategories[arg]; exists {
					helpText = o.commandRegistry.FormatCategoryHelp(arg)
				} else if o.normalCommandRegistry != nil {
					normalCategories := o.normalCommandRegistry.GetAllCommands()
					if _, exists := normalCategories[arg]; exists {
						helpText = o.normalCommandRegistry.FormatCategoryHelp(arg)
					} else {
						// Not found - suggest similar commands from both registries
						exSuggestions := o.commandRegistry.SuggestCommand(arg)
						normalSuggestions := o.normalCommandRegistry.SuggestCommand(arg)
						allSuggestions := append(exSuggestions, normalSuggestions...)
						if len(allSuggestions) > 0 {
							helpText = fmt.Sprintf("Command or category '%s' not found.\nDid you mean: %s", arg, strings.Join(allSuggestions, ", "))
						} else {
							helpText = fmt.Sprintf("Command or category '%s' not found.\nUse ':help' for ex commands or ':help normal' for normal mode commands.", arg)
						}
					}
				} else {
					// Normal command registry not available
					suggestions := o.commandRegistry.SuggestCommand(arg)
					if len(suggestions) > 0 {
						helpText = fmt.Sprintf("Command or category '%s' not found.\nDid you mean: %s", arg, strings.Join(suggestions, ", "))
					} else {
						helpText = fmt.Sprintf("Command or category '%s' not found.\nUse ':help' to see all available commands.", arg)
					}
				}
			}
		} else {
			// Check if it's a category
			categories := o.commandRegistry.GetAllCommands()
			if _, exists := categories[arg]; exists {
				helpText = o.commandRegistry.FormatCategoryHelp(arg)
			} else {
				// Not found - suggest similar commands
				suggestions := o.commandRegistry.SuggestCommand(arg)
				if len(suggestions) > 0 {
					helpText = fmt.Sprintf("Command or category '%s' not found.\nDid you mean: %s", arg, strings.Join(suggestions, ", "))
				} else {
					helpText = fmt.Sprintf("Command or category '%s' not found.\nUse ':help' to see all available commands.", arg)
				}
			}
		}
	}

	o.drawNotice(helpText)
	o.altRowoff = 0
	o.mode = NAVIGATE_NOTICE
	o.command_line = ""
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
		help.WriteString(fmt.Sprintf("## %s:\n", category))

		for _, cmd := range commands {
			help.WriteString(fmt.Sprintf("`  %-15s` - %s\n", cmd.Name, cmd.Description))
		}
		help.WriteString("\n")
	}

	help.WriteString("Use ':help <key>' for detailed help on a specific normal mode command.\n")
	help.WriteString("Use ':help <category>' for commands in a specific category.\n")

	return help.String()
}

// findNormalCommandByDisplayName finds a normal command by its display name
func (o *Organizer) findNormalCommandByDisplayName(displayName string) (CommandInfo, bool) {
	if o.normalCommandRegistry == nil {
		return CommandInfo{}, false
	}

	allCommands := o.normalCommandRegistry.GetAllCommands()
	for _, commands := range allCommands {
		for _, cmd := range commands {
			if cmd.Name == displayName {
				return cmd, true
			}
		}
	}
	return CommandInfo{}, false
}

/* would bring back previous syncs, which I never use
func (o *Organizer) log(_ int) {

	//db.MainDB.Query(fmt.Sprintf("SELECT id, title, %s FROM sync_log ORDER BY %s DESC LIMIT %d", org.sort, org.sort, max))
	o.rows = o.Database.getSyncItems(o.sort, MAX) //getSyncItems should have an err returned too
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
	o.drawRenderedNote()
	o.clearMarkedEntries()
	o.ShowMessage(BL, "")
}
*/

func (o *Organizer) openContainerSelection() {
	row := o.rows[o.fr]
	var ok bool
	switch o.view {
	case CONTEXT:
		o.taskview = BY_CONTEXT
		_, ok = o.Database.contextExists(row.title)
	case FOLDER:
		o.taskview = BY_FOLDER
		_, ok = o.Database.folderExists(row.title)
	case KEYWORD:
		o.taskview = BY_KEYWORD
		// this guard to see if synced may not be necessary for keyword
		_, ok = o.Database.keywordExists(row.title)
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
	o.mode = NORMAL
	o.fc, o.fr, o.rowoff = 0, 0, 0
	o.FilterEntries(MAX)
	if len(o.rows) == 0 {
		o.insertRow(0, "", true, false, false, BASE_DATE)
		o.rows[0].dirty = false
		o.showMessage("No results were returned")
	}
	o.Session.imagePreview = false
	o.readRowsIntoBuffer()
	vim.SetCursorPosition(1, 0)
	o.bufferTick = o.vbuf.GetLastChangedTick()
	o.altRowoff = 0
	o.drawPreview()
}

func (o *Organizer) open(pos int) {
	if o.view != TASK {
		o.openContainerSelection()
		return
	}
	if pos == -1 {
		o.ShowMessage(BL, "You did not provide a context or folder!")
		//o.mode = o.last_mode
		// It might be necessary or prudent to also sendvim an <esc> here
		o.mode = NORMAL
		return
	}
	var ok bool
	input := o.command_line[pos+1:]
	if _, ok = o.Database.contextExists(input); ok {
		o.taskview = BY_CONTEXT
	}
	if !ok {
		if _, ok = o.Database.folderExists(input); ok {
			o.taskview = BY_FOLDER
		}
	}
	if !ok {
		o.ShowMessage(BL, "%s is not a valid context or folder!", input)
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}
	o.filter = input
	o.generateNoteList()
}

func (o *Organizer) openContext(pos int) {
	if pos == -1 {
		o.ShowMessage(BL, "You did not provide a context!")
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}
	input := o.command_line[pos+1:]
	if _, ok := o.Database.contextExists(input); !ok {
		o.ShowMessage(BL, "%s is either not a valid context or has not been synced!", input)
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}
	o.taskview = BY_CONTEXT
	o.filter = input
	o.generateNoteList()
}

func (o *Organizer) openFolder(pos int) {
	if pos == -1 {
		o.ShowMessage(BL, "You did not provide a folder!")
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}
	input := o.command_line[pos+1:]
	if _, ok := o.Database.folderExists(input); !ok {
		o.ShowMessage(BL, "%s is not a valid folder!", input)
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}
	o.taskview = BY_FOLDER
	o.filter = input
	o.generateNoteList()
}

func (o *Organizer) openKeyword(pos int) {
	if pos == -1 {
		o.ShowMessage(BL, "You did not provide a keyword!")
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}
	input := o.command_line[pos+1:]
	if _, ok := o.Database.keywordExists(input); !ok {
		o.ShowMessage(BL, "%s is not a valid keyword!", input)
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}
	o.taskview = BY_KEYWORD
	o.filter = input
	o.generateNoteList()
}

func (o *Organizer) write(pos int) {
	var updated_rows []int
	if o.view != TASK {
		return
	}
	for i, r := range o.rows {
		if r.dirty {
			if r.id != -1 {
				err := o.Database.updateTitle(&r)
				if err != nil {
					o.ShowMessage(BL, "Error updating title: id %d: %v", r.id, err)
					continue
				}
			} else {
				var context_tid, folder_tid int
				switch o.taskview {
				case BY_CONTEXT:
					context_tid, _ = o.Database.contextExists(o.filter)
					folder_tid = 1
				case BY_FOLDER:
					folder_tid, _ = o.Database.folderExists(o.filter)
					context_tid = 1
				default:
					context_tid = 1
					folder_tid = 1
				}
				err := o.Database.insertTitle(&r, context_tid, folder_tid)
				if err != nil {
					o.ShowMessage(BL, "Error inserting title id %d: %v", r.id, err)
					continue
				}
			}
			o.rows[i].dirty = false
			updated_rows = append(updated_rows, r.id)
		}
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
	var ae *Editor
	if o.view != TASK {
		o.command = ""
		//o.mode = o.last_mode
		o.mode = NORMAL
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
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}

	o.Session.editorMode = true

	active := false
	for _, w := range o.Session.Windows {
		if e, ok := w.(*Editor); ok {
			if e.id == id {
				active = true
				ae = e
				break
			}
		}
	}

	if !active {
		ae = app.NewEditor()
		o.Session.Windows = append(o.Session.Windows, ae)
		ae.id = id
		ae.title = o.rows[o.fr].title
		ae.top_margin = TOP_MARGIN + 1

		if o.Database.taskFolder(o.rows[o.fr].id) == "code" {
			ae.output = &Output{}
			ae.output.is_below = true
			ae.output.id = id
			o.Session.Windows = append(o.Session.Windows, ae.output)
		}
		note := o.Database.readNoteIntoString(id)
		ae.ss = strings.Split(note, "\n")
		// Make sure we have at least one line, even if the note was empty
		if len(ae.ss) == 0 {
			ae.ss = []string{""}
		}
		ae.vbuf = vim.NewBuffer(0)
		vim.SetCurrentBuffer(ae.vbuf)
		ae.vbuf.SetLines(0, -1, ae.ss)
		//////// need to look at whether we need both buffer and save tick 10/01/2025
		ae.bufferTick = ae.vbuf.GetLastChangedTick()
		ae.saveTick = ae.vbuf.GetLastChangedTick()
		o.Session.activeEditor = ae
	}

	o.Screen.positionWindows()
	o.Screen.eraseRightScreen() //erases editor area + statusbar + msg
	o.Screen.drawRightScreen()
	ae.mode = NORMAL
	o.Session.activeEditor = ae
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
	//o.mode = o.last_mode
	o.mode = NORMAL
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

func (o *Organizer) newEntry(_ int) {
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

// flag is -1 if called as an ex command and 0 if called by another Organizer method
func (o *Organizer) refresh(flag int) {
	var err error
	if o.view == TASK {
		if o.taskview == BY_FIND {
			o.mode = NORMAL ///////////////////////////
			//o.mode = FIND ////////////////////////////////////
			o.fc, o.fr, o.rowoff = 0, 0, 0
			o.rows, err = o.Database.searchEntries(o.Session.fts_search_terms, o.sort, o.show_deleted, false)
			if err != nil {
				o.ShowMessage(BL, "Error searching for %s: %v", o.Session.fts_search_terms, err)
				return
			}
			if len(o.rows) == 0 {
				o.insertRow(0, "", true, false, false, BASE_DATE)
				o.rows[0].dirty = false
				o.ShowMessage(BL, "No results were returned")
			}
			o.Session.imagePreview = false
			o.readRowsIntoBuffer()
			vim.SetCursorPosition(1, 0)
			o.bufferTick = o.vbuf.GetLastChangedTick()
			o.drawPreview()
		} else {
			//o.mode = o.last_mode
			o.mode = NORMAL
			o.fc, o.fr, o.rowoff = 0, 0, 0
			o.FilterEntries(MAX)
			if len(o.rows) == 0 {
				o.insertRow(0, "", true, false, false, BASE_DATE)
				o.rows[0].dirty = false
				o.ShowMessage(BL, "No results were returned")
			}
			o.Session.imagePreview = false
			o.readRowsIntoBuffer()
			vim.SetCursorPosition(1, 0)
			o.bufferTick = o.vbuf.GetLastChangedTick()
			o.ShowMessage(BL, "View refreshed")
			o.drawPreview()
		}
	} else {
		//o.mode = o.last_mode
		o.mode = NORMAL
		o.sort = "modified" //It's actually sorted by alpha but displays the modified field
		o.rows = o.Database.getContainers(o.view)
		if len(o.rows) == 0 {
			//o.mode = NO_ROWS  //I don't think NO_ROWS exists any more
			o.insertRow(0, "", true, false, false, BASE_DATE)
			o.rows[0].dirty = false
			o.ShowMessage(BL, "No results were returned")
		}
		o.fc, o.fr, o.rowoff = 0, 0, 0
		o.filter = ""
		o.readRowsIntoBuffer()
		vim.SetCursorPosition(1, 0)
		o.bufferTick = o.vbuf.GetLastChangedTick()
		if flag != -1 {
			o.displayContainerInfo()
		}
		o.ShowMessage(BL, "view refreshed")
	}
	o.clearMarkedEntries()
}

func (o *Organizer) find(pos int) {
	var err error
	if pos == -1 {
		o.ShowMessage(BL, "You did not enter something to find!")
		//o.mode = o.last_mode
		o.mode = NORMAL
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
	o.mode = NORMAL
	o.fc, o.fr, o.rowoff = 0, 0, 0

	o.ShowMessage(BL, "Search for '%s'", searchTerms)
	o.rows, err = o.Database.searchEntries(searchTerms, o.sort, o.show_deleted, false)
	if err != nil {
		o.ShowMessage(BL, "Error searching for %s: %v", o.Session.fts_search_terms, err)
		return
	}
	if len(o.rows) == 0 {
		o.insertRow(0, "", true, false, false, BASE_DATE)
		o.rows[0].dirty = false
	}
	o.Session.imagePreview = false
	o.readRowsIntoBuffer()
	vim.SetCursorPosition(1, 0)
	o.bufferTick = o.vbuf.GetLastChangedTick()
	o.drawPreview()
}

func (o *Organizer) synchronize(_ int) {
	var log string
	var err error
	if o.command_line == "test" {
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
	o.command_line = ""
	o.drawNotice(log)
	o.altRowoff = 0
	o.mode = NAVIGATE_NOTICE
}

/*
func (o *Organizer) initialBulkLoad(_ int) {
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
	o.drawNotice(note)
	o.altRowoff = 0
	o.mode = NAVIGATE_NOTICE
}

func (o *Organizer) reverse(_ int) {
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
	o.drawNotice(note)
	o.altRowoff = 0
	o.mode = NAVIGATE_NOTICE
}
*/

func (o *Organizer) list(pos int) {
	o.mode = NORMAL

	if pos == -1 {
		o.view = CONTEXT
	} else {
		input := o.command_line[pos+1:]
		if input[0] == 'c' {
			o.view = CONTEXT
		} else if input[0] == 'f' {
			o.view = FOLDER
		} else if input[0] == 'k' {
			o.view = KEYWORD
		} else {
			o.ShowMessage(BL, "You need to provide a valid view: c for context, f for folder, k for keyword")
			//o.mode = o.last_mode
			o.mode = NORMAL
			return
		}
		o.command_line = ""
	}

	o.Screen.eraseRightScreen()
	o.sort = "modified" //It's actually sorted by alpha but displays the modified field
	o.rows = o.Database.getContainers(o.view)
	if len(o.rows) == 0 {
		o.insertRow(0, "", true, false, false, BASE_DATE)
		o.rows[0].dirty = false
		o.ShowMessage(BL, "No results were returned")
	}
	o.fc, o.fr, o.rowoff = 0, 0, 0
	o.filter = ""
	o.readRowsIntoBuffer()
	vim.SetCursorPosition(1, 0)
	o.bufferTick = o.vbuf.GetLastChangedTick()
	o.displayContainerInfo()
	o.ShowMessage(BL, "Retrieved contexts")
}
func (o *Organizer) setContext(pos int) {
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
		o.ShowMessage(BL, "Context is unsynced")
		return
	}

	if len(o.marked_entries) > 0 {
		for id := range o.marked_entries {
			err := o.Database.updateTaskContextByTid(tid, id)
			if err != nil {
				o.ShowMessage(BL, "Error updating context (updateTaskContextByTid) for entry %d to tid %d: %v", id, tid, err)
				return
			}
		}
		o.ShowMessage(BL, "Marked entries moved into context %s", input)
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

func (o *Organizer) setFolder(pos int) {
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
		for id, _ := range o.marked_entries {
			err := o.Database.updateTaskFolderByTid(tid, id)
			if err != nil {
				o.ShowMessage(BL, "Error updating folder (updateTaskFolderByTid) for entry %d to tid %d: %v", id, tid, err)
				return
			}
		}
		o.ShowMessage(BL, "Marked entries moved into folder %s", input)
		return
	}
	//o.Database.updateTaskFolderByTid(tid, o.rows[o.fr].id)
	id := o.rows[o.fr].id
	err := o.Database.updateTaskFolderByTid(tid, id)
	if err != nil {
		o.ShowMessage(BL, "Error updating folder (updateTaskFolderByTid) for entry %d to tid %d: %v", id, tid, err)
		return
	}
	o.ShowMessage(BL, "Moved current entry (since none were marked) into folder %s", input)
	///o.drawStatusBar() /////// won't do anything given the current code
}

func (o *Organizer) keywords(pos int) {
	o.mode = NORMAL

	if pos == -1 {
		o.Screen.eraseRightScreen()
		o.view = KEYWORD
		o.sort = "modified" //It's actually sorted by alpha but displays the modified field
		o.rows = o.Database.getContainers(o.view)

		if len(o.rows) == 0 {
			o.insertRow(0, "", true, false, false, BASE_DATE)
			o.rows[0].dirty = false
			o.ShowMessage(BL, "No results were returned")
		}
		o.fc, o.fr, o.rowoff = 0, 0, 0
		o.filter = ""
		o.readRowsIntoBuffer()
		vim.SetCursorPosition(1, 0)
		o.bufferTick = o.vbuf.GetLastChangedTick()
		o.displayContainerInfo()
		o.ShowMessage(BL, "Retrieved keywords")
		return
	}

	// not necessary if handled in sync (but not currently handled there)
	if len(o.marked_entries) == 0 && o.Database.entryTidFromId(o.rows[o.fr].id) < 1 {
		o.ShowMessage(BL, "The entry has not been synced yet!")
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}

	input := o.command_line[pos+1:]
	var ok bool
	var tid int
	if tid, ok = o.Database.keywordExists(input); !ok {
		o.ShowMessage(BL, "%s is not a valid keyword!", input)
		//o.mode = o.last_mode
		o.mode = NORMAL
		return
	}

	if tid < 1 {
		o.ShowMessage(BL, "%q is an unsynced keyword!", input)
		//o.mode = o.last_mode
		o.mode = NORMAL
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

func (o *Organizer) recent(_ int) {
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
	vim.SetCursorPosition(1, 0)
	o.bufferTick = o.vbuf.GetLastChangedTick()
	o.drawPreview()
}

func (o *Organizer) deleteKeywords(_ int) {
	id := o.getId()
	res := o.Database.deleteKeywords(id)
	//o.mode = o.last_mode
	o.mode = NORMAL
	if res != -1 {
		o.ShowMessage(BL, "%d keyword(s) deleted from entry %d", res, id)
	}
}

func (o *Organizer) showAll(_ int) {

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

/*
func (o *Organizer) updateContainer(_ int) {
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
	o.altRows = o.Database.getAltContainers(o.altView) //O.mode = NORMAL is in get_containers
	if len(o.altRows) != 0 {
		o.altFr = 0
		o.mode = ADD_CHANGE_FILTER
		o.ShowMessage(BL, "Select context to add to marked or current entry")
	}
}
*/

func (o *Organizer) deleteMarks(_ int) {
	o.clearMarkedEntries()
	o.mode = NORMAL
	o.command_line = ""
	o.ShowMessage(BL, "Marks cleared")
}

func (o *Organizer) copyEntry(_ int) {
	//copyEntry()
	o.mode = NORMAL
	o.command_line = ""
	o.refresh(0)
	o.ShowMessage(BL, "Entry copied")
}

/*
func (o *Organizer) savelog(_ int) {
	if o.last_mode == NAVIGATE_NOTICE {
		title := fmt.Sprintf("%v", time.Now().Format("Mon Jan 2 15:04:05"))
		o.Database.insertSyncEntry(title, strings.Join(o.note, "\n"))
		o.ShowMessage(BL, "Sync log save to database")
		o.command_line = ""
		o.mode = NAVIGATE_NOTICE
	} else {
		o.ShowMessage(BL, "There is no sync log to save")
		o.command_line = ""
		o.mode = o.last_mode
	}
}
*/

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

/*
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
*/

func (o *Organizer) printDocument(_ int) {
	id := o.rows[o.fr].id
	note := o.Database.readNoteIntoString(id)
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
			Papersz:     "",
			PdfFile:     "output.pdf",
			TracerFile:  "trace.log",
			Opts:        nil,
			Theme:       mdtopdf.LIGHT,
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
	//o.mode = o.last_mode
	o.mode = NORMAL
	o.command_line = ""
}

func (o *Organizer) showWebView(_ int) {

	if len(o.note) == 0 {
		return
	}

	// Get current note content
	id := o.rows[o.fr].id
	note := o.Database.readNoteIntoString(id)
	//note := strings.Join(e.vbuf.Lines(), "\n")

	// Get note title from the editor
	title := o.rows[o.fr].title
	if title == "" {
		title = "Untitled Note"
	}

	// Convert to HTML
	htmlContent, err := RenderNoteAsHTML(title, note)
	if err != nil {
		o.ShowMessage(BL, "Error rendering HTML: %v", err)
		return
	}

	// Check if webview is available
	if !IsWebviewAvailable() {
		o.ShowMessage(BL, ShowWebviewNotAvailableMessage())
		// Fall back to opening in browser
		err = OpenNoteInWebview(title, htmlContent)
		if err != nil {
			o.ShowMessage(BL, "Error opening note: %v", err)
		}
		return
	}

	// Open in webview in a goroutine since it blocks
	// This will either create a new webview or update the existing one
	go func() {
		err := OpenNoteInWebview(title, htmlContent)
		if err != nil {
			// Note: Can't directly show message from goroutine
			// Could implement a channel-based message system if needed
		}
	}()

	if IsWebviewRunning() {
		o.ShowMessage(BL, "Updated webview content")
	} else {
		o.ShowMessage(BL, "Launched webview")
	}
	o.mode = NORMAL
}

func (o *Organizer) closeWebView(_ int) {
	if !IsWebviewRunning() {
		o.ShowMessage(BL, "No webview window is currently open")
		o.mode = NORMAL
		return
	}

	err := CloseWebview()
	if err != nil {
		o.ShowMessage(BL, "Error closing webview: %v", err)
	} else {
		o.ShowMessage(BL, "Closed webview window")
	}
	o.mode = NORMAL
	o.command_line = ""
}

func (o *Organizer) whichVim(_ int) {
	var msg string
	if vim.ActiveImplementation == vim.ImplGo {
		msg = "Go Vim"
	} else {
		msg = "CGO Vim"
	}
	o.ShowMessage(BL, "vim version: %s", msg)
	o.mode = NORMAL
	o.command_line = ""
}

func (o *Organizer) printList(_ int) {
	var ss []string
	for i, row := range o.rows {
		ss = append(ss, fmt.Sprintf("%2d. %s", i+1, row.title))
	}
	tempBuf := vim.NewBuffer(0)
	tempBuf.SetLines(0, -1, ss)
	vim.SetCurrentBuffer(tempBuf)
	vim.ExecuteCommand("ha")

	if o.Session.activeEditor != nil {
		vim.SetCurrentBuffer(o.Session.activeEditor.vbuf)
	}
	//o.mode = o.last_mode
	o.mode = NORMAL
	o.command_line = ""
}

func (o *Organizer) printList2(_ int) {
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
	//o.mode = o.last_mode
	o.mode = NORMAL
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
		o.ShowMessage(BL, "The sort columns are modified, added, and priority")
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
		vim.SetCursorPosition(1, 0)
		o.bufferTick = o.vbuf.GetLastChangedTick()
		o.drawPreview()
	*/
}

// startResearch initiates deep research using the current entry's note as the research prompt
func (o *Organizer) startResearch(_ int) {
	// Check if research manager is available
	if app.ResearchManager == nil {
		o.ShowMessage(BL, "Research feature not available - Claude API key not configured")
		return
	}

	// Get the current entry
	if o.fr >= len(o.rows) {
		o.ShowMessage(BL, "No entry selected for research")
		return
	}

	currentRow := o.rows[o.fr]
	if currentRow.id == -1 {
		o.ShowMessage(BL, "Cannot research unsaved entries")
		return
	}

	// Read the current entry's note content to use as research prompt
	prompt := o.Database.readNoteIntoString(currentRow.id)
	if len(strings.TrimSpace(prompt)) == 0 {
		o.ShowMessage(BL, "Entry has no content to use as research prompt")
		o.mode = NORMAL
		o.command_line = ""
		return
	}

	// Generate a research title from the entry title
	researchTitle := fmt.Sprintf("Research for: %s", currentRow.title)
	if len(researchTitle) > 100 {
		researchTitle = researchTitle[:97] + "..."
	}

	// Start the research (normal mode - no debug info)
	taskID, err := app.ResearchManager.StartResearch(researchTitle, prompt, currentRow.id, false)
	if err != nil {
		o.mode = NORMAL
		o.command_line = ""
		o.ShowMessage(BL, "Failed to start research: %v", err)
		return
	}

	o.mode = NORMAL
	o.command_line = ""
	o.ShowMessage(BL, "Research started: %s (Task ID: %s)", researchTitle, taskID)
}

// startResearchDebug initiates deep research with full debug information
func (o *Organizer) startResearchDebug(_ int) {
	// Check if research manager is available
	if app.ResearchManager == nil {
		o.ShowMessage(BL, "Research feature not available - Claude API key not configured")
		return
	}

	// Get the current entry
	if o.fr >= len(o.rows) {
		o.ShowMessage(BL, "No entry selected for research")
		return
	}

	currentRow := o.rows[o.fr]
	if currentRow.id == -1 {
		o.ShowMessage(BL, "Cannot research unsaved entries")
		return
	}

	// Read the current entry's note content to use as research prompt
	prompt := o.Database.readNoteIntoString(currentRow.id)
	if len(strings.TrimSpace(prompt)) == 0 {
		o.ShowMessage(BL, "Entry has no content to use as research prompt")
		return
	}

	// Generate a research title from the entry title
	researchTitle := fmt.Sprintf("Research for: %s", currentRow.title)
	if len(researchTitle) > 100 {
		researchTitle = researchTitle[:97] + "..."
	}

	// Start the research (debug mode - with full debug info)
	taskID, err := app.ResearchManager.StartResearch(researchTitle, prompt, currentRow.id, true)
	if err != nil {
		o.ShowMessage(BL, "Failed to start debug research: %v", err)
		return
	}

	o.ShowMessage(BL, "Debug research started: %s (Task ID: %s)", researchTitle, taskID)
}

func (o *Organizer) startResearchNotificationTest(_ int) {
	o.mode = NORMAL
	o.command_line = ""
	o.ShowMessage(BL, "Research notification test started; sample updates will appear")

	go func() {
		steps := []struct {
			delay   time.Duration
			message string
		}{
			{2 * time.Second, "[TEST] Research: queued background task"},
			{2 * time.Second, "[TEST] Research: contacting knowledge sources"},
			{2 * time.Second, "[TEST] Research: synthesizing draft findings"},
			{2 * time.Second, "[TEST] Research: completed"},
		}
		for _, step := range steps {
			time.Sleep(step.delay)
			app.addNotification(step.message)
		}
	}()
}

// toggleImages toggles inline image display on/off
func (o *Organizer) toggleImages(_ int) {
	o.mode = NORMAL
	o.command_line = ""

	app.showImages = !app.showImages

	status := "OFF"
	if app.showImages {
		status = "ON"
	}

	o.ShowMessage(BL, fmt.Sprintf("Images: %s", status))
	o.drawPreview()
}

// scaleImages changes the image scale (width in columns)
func (o *Organizer) scaleImages(pos int) {
	// Parse argument from command line
	var argStr string
	if pos+1 < len(o.command_line) {
		argStr = strings.TrimSpace(o.command_line[pos+1:])
	}

	o.mode = NORMAL
	o.command_line = ""

	if argStr == "" {
		o.ShowMessage(BL, fmt.Sprintf("Current image scale: %d columns", app.imageScale))
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
			o.ShowMessage(BL, fmt.Sprintf("Invalid scale value: %s (use +, -, or number)", argStr))
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

	// Delete all existing kitty images (they're at the old scale)
	// Next render will transmit at the new scale
	deleteAllKittyImages()

	o.ShowMessage(BL, fmt.Sprintf("Image scale: %d columns", app.imageScale))
	o.drawPreview()
}

// kittyReset clears kitty images and local caches, then rerenders current note.
func (o *Organizer) kittyReset(pos int) {
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
	o.ShowMessage(BL, "Kitty images cleared; since last reset: %d images, %.2f MB sent", sent, mb)
	o.drawPreview()
}
