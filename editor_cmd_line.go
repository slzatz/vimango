package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mandolyte/mdtopdf/v2"
	"github.com/slzatz/vimango/vim"
)

var e_lookup_C = map[string]func(*Editor){
	"write":           (*Editor).writeNote,
	"w":               (*Editor).writeNote,
	"wa":              (*Editor).writeAll,
	"qa":              (*Editor).quitAll,
	"read":            (*Editor).readFile,
	"readfile":        (*Editor).readFile,
	"vertical resize": (*Editor).verticalResize,
	"vert res":        (*Editor).verticalResize,
	"resize":          (*Editor).resize,
	"res":             (*Editor).resize,
	"compile":         (*Editor).compile,
	"c":               (*Editor).compile,
	"run":             (*Editor).run,
	"r":               (*Editor).run,
	"save":            (*Editor).saveNoteToFile,
	"savefile":        (*Editor).saveNoteToFile,
	"syntax":          (*Editor).syntax,
	"number":          (*Editor).number,
	"num":             (*Editor).number,
	"ha":              (*Editor).printNote,
	"quit":            (*Editor).quitActions,
	"q":               (*Editor).quitActions,
	"quit!":           (*Editor).quitActions,
	"q!":              (*Editor).quitActions,
	"x":               (*Editor).quitActions,
	"fmt":             (*Editor).goFormat,
	"pdf":             (*Editor).createPDF,
	"print":           (*Editor).printDocument,
	//"spell":  (*Editor).spell,
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
	e.bufferTick = vim.BufferGetLastChangedTick(e.vbuf)

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
		dir = "/home/slzatz/go_fragments/"
		//cmd = exec.Command("go", "build", "main.go")
		cmd = exec.Command("go", "build")
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
	pos := strings.Index(e.command_line, " ")
	if pos != -1 {
		args = strings.Split(e.command_line[pos+1:], " ")
	}

	var dir string
	var obj string
	var cmd *exec.Cmd
	//if getFolderTid(e.id) == 18 {
	if e.Database.taskContext(e.id) == "cpp" {
		obj = "./test_cpp"
		dir = "/home/slzatz/clangd_examples/"
	} else {
		//obj = "./main"
		obj = "./go_fragments"
		dir = "/home/slzatz/go_fragments/"
	}
	lang := Languages[e.Database.taskContext(e.id)]
	if lang == "cpp" {
		dir = "/home/slzatz/clangd_examples/"
		cmd = exec.Command("make")
	} else if lang == "go" {
		dir = "/home/slzatz/go_fragments/"
		cmd = exec.Command("go", "build", "main.go")
	} else if lang == "python" {
		obj = "./main.py"
		dir = "/home/slzatz/python_fragments/"
	} else {
		e.ShowMessage(BR, "I don't recognize %q", e.Database.taskContext(e.id))
		return
	}

	cmd = exec.Command(obj, args...)
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
	rows = append(rows, "%-----------------------")

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
	vim.Execute("ha")
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

func (e *Editor) quitActions() {
	cmd := e.command_line
	if cmd == "x" {
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

	vim.Execute("bw") // wipout the buffer

	index := -1
	for i, w := range app.Windows {
		if w == e {
			index = i
			break
		}
	}
	copy(app.Windows[index:], app.Windows[index+1:])
	app.Windows = app.Windows[:len(app.Windows)-1]

	if e.output != nil {
		index = -1
		for i, w := range app.Windows {
			if w == e.output {
				index = i
				break
			}
		}
		copy(app.Windows[index:], app.Windows[index+1:])
		app.Windows = app.Windows[:len(app.Windows)-1]
	}

	//if len(app.Windows) > 0 {
	if e.Session.numberOfEditors() > 0 {
		// easier to just go to first window which has to be an editor (at least right now)
		for _, w := range app.Windows {
			if ed, ok := w.(*Editor); ok { //need the type assertion
				e.Session.activeEditor = ed 
				break
			}
		}

		//p = app.Windows[0].(*Editor)
		vim.BufferSetCurrent(e.Session.activeEditor.vbuf)
		e.Screen.positionWindows()
		e.Screen.eraseRightScreen()
		e.Screen.drawRightScreen()

	} else { // we've quit the last remaining editor(s)
		// unless commented out earlier sess.p.quit <- causes panic
		//sess.p = nil
		e.Session.editorMode = false
		vim.BufferSetCurrent(org.vbuf) ///////////////////////////////////////////////////////////
		e.Screen.eraseRightScreen()

		if e.Screen.divider < 10 {
			e.Screen.edPct = 80
			app.moveDividerPct(80)
		}

		//org.readTitleIntoBuffer() // shouldn't be necessary
		org.drawPreview()
		app.returnCursor() //because main while loop if started in editor_mode -- need this 09302020
	}

}

func (e *Editor) writeAll() {
	for _, w := range app.Windows {
		if ed, ok := w.(*Editor); ok {
			vim.BufferSetCurrent(ed.vbuf)
			ed.writeNote()
		}
	}
	vim.BufferSetCurrent(e.vbuf)
	e.command_line = ""
	e.mode = NORMAL
}

func (e *Editor) quitAll() {

	for _, w := range app.Windows {
		if ed, ok := w.(*Editor); ok {
			if ed.isModified() {
				continue
			} else {
				index := -1
				for i, w := range app.Windows {
					if w == ed {
						index = i
						break
					}
				}
				copy(app.Windows[index:], app.Windows[index+1:])
				app.Windows = app.Windows[:len(app.Windows)-1]

				if ed.output != nil {
					index = -1
					for i, w := range app.Windows {
						if w == ed.output {
							index = i
							break
						}
					}
					copy(app.Windows[index:], app.Windows[index+1:])
					app.Windows = app.Windows[:len(app.Windows)-1]
				}
			}
		}
	}

	if e.Session.numberOfEditors() > 0 { // we could not quit some editors because they were in modified state
		for _, w := range app.Windows {
			if ed, ok := w.(*Editor); ok { //need this type assertion to have statement below
				e.Session.activeEditor = ed //p is the global representing the current editor
				break
			}
		}

		vim.BufferSetCurrent(e.Session.activeEditor.vbuf)
		e.Screen.positionWindows()
		e.Screen.eraseRightScreen()
		e.Screen.drawRightScreen()
		e.ShowMessage(BR, "Some editors had no write since the last change")

	} else { // we've been able to quit all editors because none were in modified state
		e.Session.editorMode = false
		vim.BufferSetCurrent(org.vbuf) ///////////////////////////////////////////////////////////
		e.Screen.eraseRightScreen()

		if e.Screen.divider < 10 {
			e.Screen.edPct = 80
			app.moveDividerPct(80)
		}

		//org.readTitleIntoBuffer() // shouldn't be necessary
		org.drawPreview()
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

	vim.BufferSetLines(e.vbuf, 0, -1, e.ss, len(e.ss))
	lines := vim.BufferGetLineCount(e.vbuf)
	e.ShowMessage(BL, "Number of lines in the formatted text = %d", lines)
	vim.CursorSetPosition(1, 0)
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
      Papersz: "",
      PdfFile: filename,
      TracerFile: "trace.log",
      Opts: nil,
      Theme: mdtopdf.LIGHT,
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
