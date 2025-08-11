package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/mandolyte/mdtopdf/v2"
	"github.com/slzatz/vimango/vim"
)

func (a *App) setEditorExCmds(editor *Editor) map[string]func(*Editor) {
	registry := NewCommandRegistry[func(*Editor)]()

	// File Operations commands
	registry.Register("write", (*Editor).writeNote, CommandInfo{
		Name:        "write",
		Aliases:     []string{"w"},
		Description: "Save current note to database",
		Usage:       "write",
		Category:    "File Operations",
		Examples:    []string{":write", ":w"},
	})

	registry.Register("writeall", (*Editor).writeAll, CommandInfo{
		Name:        "writeall",
		Aliases:     []string{"wa"},
		Description: "Save all open notes",
		Usage:       "writeall",
		Category:    "File Operations",
		Examples:    []string{":writeall", ":wa"},
	})

	registry.Register("read", (*Editor).readFile, CommandInfo{
		Name:        "read",
		Aliases:     []string{"readfile"},
		Description: "Read contents from file into current note",
		Usage:       "read <filename>",
		Category:    "File Operations",
		Examples:    []string{":read todo.txt", ":readfile /tmp/notes.md"},
	})

	registry.Register("save", (*Editor).saveNoteToFile, CommandInfo{
		Name:        "save",
		Aliases:     []string{"savefile"},
		Description: "Save current note to external file",
		Usage:       "save <filename>",
		Category:    "File Operations",
		Examples:    []string{":save backup.txt", ":savefile /tmp/note.md"},
	})

	// Editing commands
	registry.Register("syntax", (*Editor).syntax, CommandInfo{
		Name:        "syntax",
		Description: "Set syntax highlighting for current note",
		Usage:       "syntax <language>",
		Category:    "Editing",
		Examples:    []string{":syntax go", ":syntax python", ":syntax markdown"},
	})

	registry.Register("number", (*Editor).number, CommandInfo{
		Name:        "number",
		Aliases:     []string{"num"},
		Description: "Toggle line numbers on/off",
		Usage:       "number",
		Category:    "Editing",
		Examples:    []string{":number", ":num"},
	})

	registry.Register("fmt", (*Editor).goFormat, CommandInfo{
		Name:        "fmt",
		Description: "Format Go code using gofmt",
		Usage:       "fmt",
		Category:    "Editing",
		Examples:    []string{":fmt"},
	})

	registry.Register("run", (*Editor).run, CommandInfo{
		Name:        "run",
		Aliases:     []string{"r"},
		Description: "Execute current note as code",
		Usage:       "run",
		Category:    "Editing",
		Examples:    []string{":run", ":r"},
	})

	// Layout commands
	registry.Register("vertical resize", (*Editor).verticalResize, CommandInfo{
		Name:        "vertical resize",
		Aliases:     []string{"vert res"},
		Description: "Resize vertical divider",
		Usage:       "vertical resize <width>",
		Category:    "Layout",
		Examples:    []string{":vertical resize 80", ":vert res +10", ":vert res -5"},
	})

	registry.Register("resize", (*Editor).resize, CommandInfo{
		Name:        "resize",
		Aliases:     []string{"res"},
		Description: "Resize editor window",
		Usage:       "resize <height>",
		Category:    "Layout",
		Examples:    []string{":resize 20", ":res +5", ":res -3"},
	})

	// Output commands
	registry.Register("ha", (*Editor).printNote, CommandInfo{
		Name:        "ha",
		Description: "Print current note using vim hardcopy",
		Usage:       "ha",
		Category:    "Output",
		Examples:    []string{":ha"},
	})

	registry.Register("print", (*Editor).printDocument, CommandInfo{
		Name:        "print",
		Description: "Print current note as formatted document",
		Usage:       "print",
		Category:    "Output",
		Examples:    []string{":print"},
	})

	registry.Register("pdf", (*Editor).createPDF, CommandInfo{
		Name:        "pdf",
		Description: "Create PDF from current note",
		Usage:       "pdf",
		Category:    "Output",
		Examples:    []string{":pdf"},
	})

	// System commands
	registry.Register("quit", (*Editor).quitActions, CommandInfo{
		Name:        "quit",
		Aliases:     []string{"q", "quit!", "q!"},
		Description: "Close current editor (q!/quit! forces without saving)",
		Usage:       "quit",
		Category:    "System",
		Examples:    []string{":quit", ":q", ":q!", ":x"},
	})

	registry.Register("exit", (*Editor).exit, CommandInfo{
		Name:        "exit",
		Aliases:     []string{"x"},
		Description: "Save and exit the current file",
		Usage:       "x",
		Category:    "System",
		Examples:    []string{":exit", ":x"},
	})

	registry.Register("quitall", (*Editor).quitAll, CommandInfo{
		Name:        "quitall",
		Aliases:     []string{"qa"},
		Description: "Close all editors",
		Usage:       "quitall",
		Category:    "System",
		Examples:    []string{":quitall", ":qa"},
	})

	// Help command
	registry.Register("help", (*Editor).help, CommandInfo{
		Name:        "help",
		Aliases:     []string{"h"},
		Description: "Show help for editor commands",
		Usage:       "help [command|category]",
		Category:    "Help",
		Examples:    []string{":help", ":help write", ":help File Operations", ":h"},
	})

	// Store registry in editor for help command access
	editor.commandRegistry = registry

	return registry.GetFunctionMap()
}

// help displays help information for editor commands
func (e *Editor) help() {
	if e.commandRegistry == nil {
		e.ShowMessage(BR, "Help system not available")
		return
	}

	var helpText string
	pos := strings.Index(e.command_line, " ")

	if pos == -1 {
		// No arguments - show all ex commands
		helpText = e.commandRegistry.FormatAllHelp()
	} else {
		// Get the argument after "help "
		arg := e.command_line[pos+1:]

		// Check if it's request for normal mode help
		if arg == "normal" {
			if e.normalCommandRegistry != nil {
				helpText = e.formatNormalModeHelp()
			} else {
				helpText = "Normal mode help not available"
			}
		} else if _, exists := e.commandRegistry.GetCommandInfo(arg); exists {
			// Check if it's a specific ex command
			helpText = e.commandRegistry.FormatCommandHelp(arg)
		} else if e.normalCommandRegistry != nil {
			// Check if it's a normal mode command (by display name)
			if normalInfo, exists := e.findNormalCommandByDisplayName(arg); exists {
				helpText = e.normalCommandRegistry.FormatCommandHelp(normalInfo.Name)
			} else {
				// Check if it's a category (ex commands first, then normal)
				exCategories := e.commandRegistry.GetAllCommands()
				if _, exists := exCategories[arg]; exists {
					helpText = e.commandRegistry.FormatCategoryHelp(arg)
				} else if e.normalCommandRegistry != nil {
					normalCategories := e.normalCommandRegistry.GetAllCommands()
					if _, exists := normalCategories[arg]; exists {
						helpText = e.normalCommandRegistry.FormatCategoryHelp(arg)
					} else {
						// Not found - suggest similar commands from both registries
						exSuggestions := e.commandRegistry.SuggestCommand(arg)
						normalSuggestions := e.normalCommandRegistry.SuggestCommand(arg)
						allSuggestions := append(exSuggestions, normalSuggestions...)
						if len(allSuggestions) > 0 {
							helpText = fmt.Sprintf("Command or category '%s' not found.\nDid you mean: %s", arg, strings.Join(allSuggestions, ", "))
						} else {
							helpText = fmt.Sprintf("Command or category '%s' not found.\nUse ':help' for ex commands or ':help normal' for normal mode commands.", arg)
						}
					}
				} else {
					// Normal command registry not available
					suggestions := e.commandRegistry.SuggestCommand(arg)
					if len(suggestions) > 0 {
						helpText = fmt.Sprintf("Command or category '%s' not found.\nDid you mean: %s", arg, strings.Join(suggestions, ", "))
					} else {
						helpText = fmt.Sprintf("Command or category '%s' not found.\nUse ':help' to see all available commands.", arg)
					}
				}
			}
		} else {
			// Check if it's a category
			categories := e.commandRegistry.GetAllCommands()
			if _, exists := categories[arg]; exists {
				helpText = e.commandRegistry.FormatCategoryHelp(arg)
			} else {
				// Not found - suggest similar commands
				suggestions := e.commandRegistry.SuggestCommand(arg)
				if len(suggestions) > 0 {
					helpText = fmt.Sprintf("Command or category '%s' not found.\nDid you mean: %s", arg, strings.Join(suggestions, ", "))
				} else {
					helpText = fmt.Sprintf("Command or category '%s' not found.\nUse ':help' to see all available commands.", arg)
				}
			}
		}
	}

	// Create a temporary editor overlay for help display
	/*
		e.overlay = strings.Split(helpText, "\n")
		e.drawOverlay()
		e.redraw = true
		e.ShowMessage(BR, "Help displayed - press ESC to close")
	*/
	// Display help in the preview area
	app.Organizer.Screen.eraseRightScreen()
	app.Organizer.renderMarkdown(helpText)
	app.Organizer.altRowoff = 0
	app.Organizer.drawRenderedNote()
	e.mode = PREVIEW
	e.command_line = ""
}

// formatNormalModeHelp returns formatted help for all normal mode commands
func (e *Editor) formatNormalModeHelp() string {
	if e.normalCommandRegistry == nil {
		return "Normal mode help not available"
	}

	var help strings.Builder
	help.WriteString("Normal Mode Commands:\n\n")

	categories := e.normalCommandRegistry.GetAllCommands()

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
func (e *Editor) findNormalCommandByDisplayName(displayName string) (CommandInfo, bool) {
	if e.normalCommandRegistry == nil {
		return CommandInfo{}, false
	}

	allCommands := e.normalCommandRegistry.GetAllCommands()
	for _, commands := range allCommands {
		for _, cmd := range commands {
			if cmd.Name == displayName {
				return cmd, true
			}
		}
	}
	return CommandInfo{}, false
}

func (e *Editor) saveNoteToFile() {
	pos := strings.Index(e.command_line, " ")
	if pos == -1 {
		e.ShowMessage(BR, "You need to provide a filename")
		return
	}
	filename := e.command_line[pos+1:]
	f, err := os.Create(filename)
	if err != nil {
		e.ShowMessage(BR, "Error creating file %s: %v", filename, err)
		return
	}
	defer f.Close()

	_, err = f.WriteString(strings.Join(e.ss, "\n"))
	if err != nil {
		e.ShowMessage(BR, "Error writing file %s: %v", filename, err)
		return
	}
	e.ShowMessage(BR, "Note written to file %s", filename)
}

func (e *Editor) writeNote() {
	text := e.bufferToString()

	if e.Database.taskFolder(e.id) == "code" {
		go e.Database.updateCodeFile(e.id, text)
	}

	err := e.Database.updateNote(e.id, text)
	if err != nil {
		e.ShowMessage(BL, "Error in updating note (updateNote) for entry with id %d: %v", e.id, err)
		return
	}
	e.ShowMessage(BL, "Updated note and fts entry for entry %d", e.id) //////

	//explicitly writes note to set isModified to false
	//vim.Execute("w")
	e.bufferTick = e.vbuf.GetLastChangedTick()

	e.drawStatusBar() //need this since now refresh won't do it unless redraw =true
	e.ShowMessage(BR, "isModified = %t", e.isModified())
}

func (e *Editor) readFile() {
	pos := strings.Index(e.command_line, " ")
	if pos == -1 {
		e.ShowMessage(BR, "You need to provide a filename")
		return
	}

	filename := e.command_line[pos+1:]
	err := e.readFileIntoNote(filename)
	if err != nil {
		e.ShowMessage(BR, "%v", err)
		return
	}
	e.ShowMessage(BR, "Note generated from file: %s", filename)
}

// testing abs move but may revert to this one
func (e *Editor) verticalResize__() {
	pos := strings.LastIndex(e.command_line, " ")
	if pos == -1 {
		e.ShowMessage(BR, "You need to provide a a number 0 - 100")
		return
	}
	pct, err := strconv.Atoi(e.command_line[pos+1:])
	if err != nil {
		e.ShowMessage(BR, "You need to provide a number 0 - 100")
		return
	}
	app.moveDividerPct(pct)
}

func (e *Editor) verticalResize() {
	pos := strings.LastIndex(e.command_line, " ")
	opt := e.command_line[pos+1:]
	width, err := strconv.Atoi(opt)

	if opt[0] == '+' || opt[0] == '-' {
		width = e.Screen.screenCols - e.Screen.divider + width
	}

	if err != nil {
		e.ShowMessage(BR, "The format is :vert[ical] res[ize] N")
		return
	}
	app.moveDividerAbs(width)
}

func (e *Editor) resize() {
	pos := strings.Index(e.command_line, " ")
	opt := e.command_line[pos+1:]
	if opt[0] == '+' || opt[0] == '-' {
		num, err := strconv.Atoi(opt[1:])
		if err != nil {
			e.ShowMessage(BR, "The format is [+/-]N")
			return
		}
		for i := 0; i < num; i++ {
			e.changeSplit(int(opt[0]))
		}
	} else {
		num, err := strconv.Atoi(opt)
		if err != nil {
			e.ShowMessage(BR, "The format is [+/-]N")
			return
		}

		if e.Screen.textLines-num < 3 || num < 2 {
			return
		}

		e.screenlines = num
		op := e.output
		op.screenlines = e.Screen.textLines - num - 1
		op.top_margin = num + 3

		e.Screen.eraseRightScreen()
		e.Screen.drawRightScreen()
	}
}

func (e *Editor) compile() {

	var dir string
	var cmd *exec.Cmd
	lang := Languages[e.Database.taskContext(e.id)]
	if lang == "cpp" {
		dir = "/home/slzatz/clangd_examples/"
		cmd = exec.Command("make")
	} else if lang == "go" {
		dir = "/home/slzatz/vmgo_go_code/"
		//cmd = exec.Command("go", "build", "main.go")
		cmd = exec.Command("go", "run")
	} else if lang == "python" {
		e.ShowMessage(BR, "You don't have to compile python")
		return
	} else {
		e.ShowMessage(BR, "I don't recognize %q", e.Database.taskContext(e.id))
		return
	}
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		e.ShowMessage(BR, "Error in compile creating stdout pipe: %v", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		e.ShowMessage(BR, "Error in compile creating stderr pipe: %v", err)
		return
	}

	err = cmd.Start()
	if err != nil {
		e.ShowMessage(BR, "Error in compile starting command: %v", err)
		return
	}

	buffer_out := bufio.NewReader(stdout)
	buffer_err := bufio.NewReader(stderr)

	var rows []string
	rows = append(rows, "------------------------")

	for {
		bytes, _, err := buffer_out.ReadLine()
		if err == io.EOF {
			break
		}
		rows = append(rows, string(bytes))
	}

	for {
		bytes, _, err := buffer_err.ReadLine()
		if err == io.EOF {
			break
		}
		rows = append(rows, string(bytes))
	}
	if len(rows) == 1 {
		rows = append(rows, "The code compiled successfully")
	}

	rows = append(rows, "------------------------")

	op := e.output
	op.rowOffset = 0
	op.rows = rows
	op.drawText()
	// no need to call drawFrame or drawStatusBar
}

func (e *Editor) run() {

	var args []string
	//var combinedArgs []string
	var name string
	var dir string
	var cmd *exec.Cmd

	pos := strings.Index(e.command_line, " ")
	if pos != -1 {
		args = strings.Split(e.command_line[pos+1:], " ")
	}

	lang := Languages[e.Database.taskContext(e.id)]
	if lang == "go" {
		dir = "/home/slzatz/vmgo_go_code/"
		name = "go"
		args = append([]string{"run", "main.go"}, args...)
	} else if lang == "python" {
		dir = "/home/slzatz/vmgo_python_code/"
		name = "python"
		args = append([]string{"main.py"}, args...)
	} else {
		e.ShowMessage(BR, "I don't recognize %q", e.Database.taskContext(e.id))
		return
	}
	cmd = exec.Command(name, args...)
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		e.ShowMessage(BR, "Error in run creating stdout pipe: %v", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		e.ShowMessage(BR, "Error in run creating stderr pipe: %v", err)
		return
	}

	err = cmd.Start()
	if err != nil {
		e.ShowMessage(BR, "Error in run starting command: %v", err)
		return
	}

	buffer_out := bufio.NewReader(stdout)
	buffer_err := bufio.NewReader(stderr)

	var rows []string
	rows = append(rows, "------------------------")

	for {
		bytes, _, err := buffer_out.ReadLine()
		if err == io.EOF {
			break
		}
		rows = append(rows, string(bytes))
	}

	for {
		bytes, _, err := buffer_err.ReadLine()
		if err == io.EOF {
			break
		}
		rows = append(rows, string(bytes))
	}

	rows = append(rows, "------------------------")

	op := e.output
	op.rowOffset = 0
	op.rows = rows
	op.drawText()
	// no need to call drawFrame or drawStatusBar
}

func (e *Editor) syntax() {
	e.highlightSyntax = !e.highlightSyntax
	if e.highlightSyntax {
		e.left_margin_offset = LEFT_MARGIN_OFFSET
		//e.checkSpelling = false // can't syntax highlight(including markdown) and check spelling
	}
	e.drawText()
	// no need to call drawFrame or drawStatusBar
	e.ShowMessage(BR, "Syntax highlighting is %v", e.highlightSyntax)
}

func (e *Editor) printNote() {
	vim.ExecuteCommand("ha")
}

/*
// debugging
func (e *Editor) modified() {
	var result bool
	err := v.BufferOption(0, "modified", &result) //or e.vbuf
	if err != nil {
		e.ShowMessage(BL, "%s", err)
		return
	}
	e.ShowMessage(BL, "Modified = %t", result)
}
*/

func (e *Editor) exit() {
	e.quitActions()
}

func (e *Editor) quitActions() {
	cmd := e.command_line
	if cmd == "x" || cmd == "exit" {
		text := e.bufferToString()
		err := e.Database.updateNote(e.id, text)
		if err != nil {
			e.ShowMessage(BR, "Error in updateNote for entry with id %d: %v", e.id, err)
		}
		e.ShowMessage(BL, "Updated note and fts entry for entry %d", e.id) //////

	} else if cmd == "q!" || cmd == "quit!" {
		// do nothing = allow editor to be closed

	} else if e.isModified() {
		e.mode = NORMAL
		e.command = ""
		e.command_line = ""
		e.ShowMessage(BR, "No write since last change")
		return
	}

	vim.ExecuteCommand("bw") // wipout the buffer

	index := -1
	//for i, w := range app.Windows {
	for i, w := range e.Session.Windows {
		if w == e {
			index = i
			break
		}
	}
	copy(e.Session.Windows[index:], e.Session.Windows[index+1:])
	e.Session.Windows = e.Session.Windows[:len(e.Session.Windows)-1]

	if e.output != nil {
		index = -1
		for i, w := range e.Session.Windows {
			if w == e.output {
				index = i
				break
			}
		}
		copy(e.Session.Windows[index:], e.Session.Windows[index+1:])
		e.Session.Windows = e.Session.Windows[:len(e.Session.Windows)-1]
	}

	//if len(app.Windows) > 0 {
	if e.Session.numberOfEditors() > 0 {
		// easier to just go to first window which has to be an editor (at least right now)
		for _, w := range e.Session.Windows {
			if ed, ok := w.(*Editor); ok { //need the type assertion
				e.Session.activeEditor = ed
				break
			}
		}

		//p = app.Windows[0].(*Editor)
		vim.SetCurrentBuffer(e.Session.activeEditor.vbuf)
		e.Screen.positionWindows()
		e.Screen.eraseRightScreen()
		e.Screen.drawRightScreen()

	} else { // we've quit the last remaining editor(s)
		// unless commented out earlier sess.p.quit <- causes panic
		//sess.p = nil
		e.Session.editorMode = false
		vim.SetCurrentBuffer(app.Organizer.vbuf) ///////////////////////////////////////////////////////////
		e.Screen.eraseRightScreen()

		if e.Screen.divider < 10 {
			e.Screen.edPct = 80
			app.moveDividerPct(80)
		}

		//org.readTitleIntoBuffer() // shouldn't be necessary
		app.Organizer.drawPreview()
		app.returnCursor() //because main while loop if started in editor_mode -- need this 09302020
	}

}

func (e *Editor) writeAll() {
	for _, w := range e.Session.Windows {
		if ed, ok := w.(*Editor); ok {
			vim.SetCurrentBuffer(ed.vbuf)
			ed.writeNote()
		}
	}
	vim.SetCurrentBuffer(e.vbuf)
	e.command_line = ""
	e.mode = NORMAL
}

func (e *Editor) quitAll() {

	for _, w := range e.Session.Windows {
		if ed, ok := w.(*Editor); ok {
			if ed.isModified() {
				continue
			} else {
				index := -1
				for i, w := range e.Session.Windows {
					if w == ed {
						index = i
						break
					}
				}
				copy(e.Session.Windows[index:], e.Session.Windows[index+1:])
				e.Session.Windows = e.Session.Windows[:len(e.Session.Windows)-1]

				if ed.output != nil {
					index = -1
					for i, w := range e.Session.Windows {
						if w == ed.output {
							index = i
							break
						}
					}
					copy(e.Session.Windows[index:], e.Session.Windows[index+1:])
					e.Session.Windows = e.Session.Windows[:len(e.Session.Windows)-1]
				}
			}
		}
	}

	if e.Session.numberOfEditors() > 0 { // we could not quit some editors because they were in modified state
		for _, w := range e.Session.Windows {
			if ed, ok := w.(*Editor); ok { //need this type assertion to have statement below
				e.Session.activeEditor = ed //p is the global representing the current editor
				break
			}
		}

		vim.SetCurrentBuffer(e.Session.activeEditor.vbuf)
		e.Screen.positionWindows()
		e.Screen.eraseRightScreen()
		e.Screen.drawRightScreen()
		e.ShowMessage(BR, "Some editors had no write since the last change")

	} else { // we've been able to quit all editors because none were in modified state
		e.Session.editorMode = false
		vim.SetCurrentBuffer(app.Organizer.vbuf) ///////////////////////////////////////////////////////////
		e.Screen.eraseRightScreen()

		if e.Screen.divider < 10 {
			e.Screen.edPct = 80
			app.moveDividerPct(80)
		}

		//org.readTitleIntoBuffer() // shouldn't be necessary
		app.Organizer.drawPreview()
		app.returnCursor() //because main while loop if started in editor_mode -- need this 09302020
	}
}

/*
func (e *Editor) spell() {
	e.checkSpelling = !e.checkSpelling
	if e.checkSpelling {
		e.highlightSyntax = false // when you check spelling syntax highlighting off
		err := v.Command("set spell")
		if err != nil {
			sess.showEdMessage("Error in setting spelling %v", err)
		}
	} else {
		err := v.Command("set nospell")
		if err != nil {
			sess.showEdMessage("Error in setting no spelling %v", err)
		}
	}
	e.drawText()
	sess.showEdMessage("Spelling is %t", e.checkSpelling)
}
*/

func (e *Editor) number() {
	e.numberLines = !e.numberLines
	if e.numberLines {
		e.left_margin_offset = LEFT_MARGIN_OFFSET
	} else {
		e.left_margin_offset = 0
	}
	e.drawText()
	e.ShowMessage(BR, "Line numbering is %t", e.numberLines)
}

func (e *Editor) goFormat() {
	ss := []string{}
	//cmd := exec.Command("gofmt")
	cmd := exec.Command("goimports")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		e.ShowMessage(BR, "Problem in goimports stdout: %v", err)
		return
	}
	buf_out := bufio.NewReader(stdout)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		e.ShowMessage(BR, "Problem in goimports stdin: %v", err)
		return
	}
	err = cmd.Start()
	if err != nil {
		e.ShowMessage(BR, "Problem in cmd.Start (goimports) stdin: %v", err)
		return
	}

	for _, row := range e.ss {
		io.WriteString(stdin, row+"\n")
	}
	stdin.Close()

	for {
		s, err := buf_out.ReadString('\n')
		if err == io.EOF {
			break
		}
		ss = append(ss, s[:len(s)-1])
	}
	if len(ss) == 0 {
		e.ShowMessage(BL, "Return from goimports has length zero - likely code errors")
		return
	}

	e.ss = ss

	e.vbuf.SetLines(0, -1, e.ss)
	lines := e.vbuf.GetLineCount()
	e.ShowMessage(BL, "Number of lines in the formatted text = %d", lines)
	vim.SetCursorPosition(1, 0)
	e.fr = 0
	e.fc = 0
	e.scroll()
	e.drawText()
	app.returnCursor()
}

func (e *Editor) createPDF() {
	pos := strings.Index(e.command_line, " ")
	if pos == -1 {
		e.ShowMessage(BL, "You need to provide a filename")
		return
	}
	filename := e.command_line[pos+1:]

	params := mdtopdf.PdfRendererParams{
		Orientation: "",
		Papersz:     "",
		PdfFile:     filename,
		TracerFile:  "trace.log",
		Opts:        nil,
		Theme:       mdtopdf.LIGHT,
	}

	//pf := mdtopdf.NewPdfRenderer("", "", filename, "trace.log", nil, mdtopdf.LIGHT)
	pf := mdtopdf.NewPdfRenderer(params)
	/*
		pf.Pdf.SetSubject("How to convert markdown to PDF", true)
		pf.Pdf.SetTitle("Example PDF converted from Markdown", true)
		pf.THeader = mdtopdf.Styler{Font: "Times", Style: "IUB", Size: 20, Spacing: 2,
			TextColor: mdtopdf.Color{Red: 0, Green: 0, Blue: 0},
			FillColor: mdtopdf.Color{Red: 179, Green: 179, Blue: 255}}
	*/
	pf.TBody = mdtopdf.Styler{Font: "Arial", Style: "", Size: 12, Spacing: 2,
		TextColor: mdtopdf.Color{Red: 0, Green: 0, Blue: 0},
		FillColor: mdtopdf.Color{Red: 255, Green: 255, Blue: 255}}

	content := strings.Join(e.ss, "\n")
	err := pf.Process([]byte(content))
	if err != nil {
		e.ShowMessage(BL, "pdf error:%v", err)
	}
}

func (e *Editor) printDocument() {
	if e.Database.taskFolder(e.id) == "code" {
		c := e.Database.taskContext(e.id)
		var ok bool
		var lang string
		if lang, ok = Languages[c]; !ok {
			e.ShowMessage(BR, "I don't recognize the language")
			return
		}
		note := e.Database.readNoteIntoString(e.id)
		var buf bytes.Buffer
		// github seems to work pretty well for printer output
		_ = Highlight(&buf, note, lang, "html", "github")

		f, err := os.Create("output.html")
		if err != nil {
			e.ShowMessage(BR, "Error creating output.html: %v", err)
			return
		}
		defer f.Close()

		_, err = f.WriteString(buf.String())
		if err != nil {
			e.ShowMessage(BR, "Error writing output.html: %s: %v", err)
			return
		}
		cmd := exec.Command("wkhtmltopdf", "--enable-local-file-access",
			"--no-background", "--minimum-font-size", "16", "output.html", "output.pdf")
		err = cmd.Run()
		if err != nil {
			e.ShowMessage(BR, "Error creating pdf from code: %v", err)
		}
	} else {
		content := strings.Join(e.ss, "\n")

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

		err := pf.Process([]byte(content))
		if err != nil {
			e.ShowMessage(BR, "pdf error:%v", err)
		}
	}
	cmd := exec.Command("lpr", "output.pdf")
	err := cmd.Run()
	if err != nil {
		e.ShowMessage(BR, "Error printing document: %v", err)
	}
}
