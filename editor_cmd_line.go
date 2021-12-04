package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mandolyte/mdtopdf"
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
	"test":            (*Editor).sync,
	"sync":            (*Editor).sync,
	"save":            (*Editor).saveNoteToFile,
	"savefile":        (*Editor).saveNoteToFile,
	"syntax":          (*Editor).syntax,
	"number":          (*Editor).number,
	"num":             (*Editor).number,
	"ha":              (*Editor).printNote,
	//"modified":        (*Editor).modified, // debugging
	"quit":   (*Editor).quitActions,
	"q":      (*Editor).quitActions,
	"quit!":  (*Editor).quitActions,
	"q!":     (*Editor).quitActions,
	"x":      (*Editor).quitActions,
	"fmt":    (*Editor).goFormat,
	"rename": (*Editor).rename, //lsp command
	"pdf":    (*Editor).createPDF,
	"print":  (*Editor).printDocument,
	//"spell":  (*Editor).spell,
}

func (e *Editor) saveNoteToFile() {
	pos := strings.Index(e.command_line, " ")
	if pos == -1 {
		sess.showEdMessage("You need to provide a filename")
		return
	}
	filename := e.command_line[pos+1:]
	f, err := os.Create(filename)
	if err != nil {
		sess.showEdMessage("Error creating file %s: %v", filename, err)
		return
	}
	defer f.Close()

	_, err = f.Write(bytes.Join(e.bb, []byte("\n")))
	if err != nil {
		sess.showEdMessage("Error writing file %s: %v", filename, err)
		return
	}
	sess.showEdMessage("Note written to file %s", filename)
}

func (e *Editor) writeNote() {
	text := e.bufferToString()

	if taskFolder(e.id) == "code" {
		if lsp.name != "" {
			go sendDidChangeNotification(text)
			go e.drawDiagnostics()
		}
		go updateCodeFile(e.id, text)
	}

	updateNote(e.id, text)

	//explicitly writes note to set isModified to false
	vim.Execute("w")

	e.drawStatusBar() //need this since now refresh won't do it unless redraw =true
	sess.showEdMessage("isModified = %t", e.isModified())
}

func (e *Editor) readFile() {
	pos := strings.Index(e.command_line, " ")
	if pos == -1 {
		sess.showEdMessage("You need to provide a filename")
		return
	}

	filename := e.command_line[pos+1:]
	err := e.readFileIntoNote(filename)
	if err != nil {
		sess.showEdMessage("%v", err)
		return
	}
	sess.showEdMessage("Note generated from file: %s", filename)
}

// testing abs move but may revert to this one
func (e *Editor) verticalResize__() {
	pos := strings.LastIndex(e.command_line, " ")
	if pos == -1 {
		sess.showEdMessage("You need to provide a a number 0 - 100")
		return
	}
	pct, err := strconv.Atoi(e.command_line[pos+1:])
	if err != nil {
		sess.showEdMessage("You need to provide a number 0 - 100")
		return
	}
	moveDividerPct(pct)
}

func (e *Editor) verticalResize() {
	pos := strings.LastIndex(e.command_line, " ")
	opt := e.command_line[pos+1:]
	width, err := strconv.Atoi(opt)

	if opt[0] == '+' || opt[0] == '-' {
		width = sess.screenCols - sess.divider + width
	}

	if err != nil {
		sess.showEdMessage("The format is :vert[ical] res[ize] N")
		return
	}
	moveDividerAbs(width)
}

func (e *Editor) resize() {
	pos := strings.Index(e.command_line, " ")
	opt := e.command_line[pos+1:]
	if opt[0] == '+' || opt[0] == '-' {
		num, err := strconv.Atoi(opt[1:])
		if err != nil {
			sess.showEdMessage("The format is [+/-]N")
			return
		}
		for i := 0; i < num; i++ {
			e.changeSplit(int(opt[0]))
		}
	} else {
		num, err := strconv.Atoi(opt)
		if err != nil {
			sess.showEdMessage("The format is [+/-]N")
			return
		}

		if sess.textLines-num < 3 || num < 2 {
			return
		}

		e.screenlines = num
		op := e.output
		op.screenlines = sess.textLines - num - 1
		op.top_margin = num + 3

		sess.eraseRightScreen()
		sess.drawRightScreen()
	}
}

func (e *Editor) compile() {

	var dir string
	var cmd *exec.Cmd
	lang := Languages[taskContext(e.id)]
	if lang == "cpp" {
		dir = "/home/slzatz/clangd_examples/"
		cmd = exec.Command("make")
	} else if lang == "go" {
		dir = "/home/slzatz/go_fragments/"
		cmd = exec.Command("go", "build", "main.go")
	} else if lang == "python" {
		sess.showEdMessage("You don't have to compile python")
		return
	} else {
		sess.showEdMessage("I don't recognize %q", taskContext(e.id))
		return
	}
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		sess.showEdMessage("Error in compile creating stdout pipe: %v", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		sess.showEdMessage("Error in compile creating stderr pipe: %v", err)
		return
	}

	err = cmd.Start()
	if err != nil {
		sess.showEdMessage("Error in compile starting command: %v", err)
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
	if taskContext(e.id) == "cpp" {
		obj = "./test_cpp"
		dir = "/home/slzatz/clangd_examples/"
	} else {
		obj = "./main"
		dir = "/home/slzatz/go_fragments/"
	}
	lang := Languages[taskContext(e.id)]
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
		sess.showEdMessage("I don't recognize %q", taskContext(e.id))
		return
	}

	cmd = exec.Command(obj, args...)
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		sess.showEdMessage("Error in run creating stdout pipe: %v", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		sess.showEdMessage("Error in run creating stderr pipe: %v", err)
		return
	}

	err = cmd.Start()
	if err != nil {
		sess.showEdMessage("Error in run starting command: %v", err)
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

func (e *Editor) sync() {
	var reportOnly bool
	if e.command_line == "test" {
		reportOnly = true
	}
	synchronize(reportOnly)
}

func (e *Editor) syntax() {
	e.highlightSyntax = !e.highlightSyntax
	if e.highlightSyntax {
		e.left_margin_offset = LEFT_MARGIN_OFFSET
		//e.checkSpelling = false // can't syntax highlight(including markdown) and check spelling
	}
	e.drawText()
	// no need to call drawFrame or drawStatusBar
	sess.showEdMessage("Syntax highlighting is %v", e.highlightSyntax)
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
		sess.showEdMessage("%s", err)
		return
	}
	sess.showEdMessage("Modified = %t", result)
}
*/

func (e *Editor) quitActions() {
	cmd := e.command_line
	if cmd == "x" {
		text := e.bufferToString()
		updateNote(e.id, text)

	} else if cmd == "q!" || cmd == "quit!" {
		// do nothing = allow editor to be closed

	} else if e.isModified() {
		e.mode = NORMAL
		e.command = ""
		e.command_line = ""
		sess.showEdMessage("No write since last change")
		return
	}

	vim.Execute("bw") // wipout the buffer

	index := -1
	for i, w := range windows {
		if w == e {
			index = i
			break
		}
	}
	copy(windows[index:], windows[index+1:])
	windows = windows[:len(windows)-1]

	if e.output != nil {
		index = -1
		for i, w := range windows {
			if w == e.output {
				index = i
				break
			}
		}
		copy(windows[index:], windows[index+1:])
		windows = windows[:len(windows)-1]
	}

	//if len(windows) > 0 {
	if sess.numberOfEditors() > 0 {
		// easier to just go to first window which has to be an editor (at least right now)
		for _, w := range windows {
			if ed, ok := w.(*Editor); ok { //need the type assertion
				p = ed //p is the global current editor
				break
			}
		}

		//p = windows[0].(*Editor)
		vim.BufferSetCurrent(p.vbuf)
		sess.positionWindows()
		sess.eraseRightScreen()
		sess.drawRightScreen()

	} else { // we've quit the last remaining editor(s)
		// unless commented out earlier sess.p.quit <- causes panic
		//sess.p = nil
		sess.editorMode = false
		vim.BufferSetCurrent(org.vbuf) ///////////////////////////////////////////////////////////
		sess.eraseRightScreen()

		if sess.divider < 10 {
			sess.cfg.ed_pct = 80
			moveDividerPct(80)
		}

		//org.readTitleIntoBuffer() // shouldn't be necessary
		org.drawPreview()
		sess.returnCursor() //because main while loop if started in editor_mode -- need this 09302020
	}

}

func (e *Editor) writeAll() {
	for _, w := range windows {
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

	for _, w := range windows {
		if ed, ok := w.(*Editor); ok {
			if ed.isModified() {
				continue
			} else {
				index := -1
				for i, w := range windows {
					if w == ed {
						index = i
						break
					}
				}
				copy(windows[index:], windows[index+1:])
				windows = windows[:len(windows)-1]

				if ed.output != nil {
					index = -1
					for i, w := range windows {
						if w == ed.output {
							index = i
							break
						}
					}
					copy(windows[index:], windows[index+1:])
					windows = windows[:len(windows)-1]
				}
			}
		}
	}

	if sess.numberOfEditors() > 0 { // we could not quit some editors because they were in modified state
		for _, w := range windows {
			if ed, ok := w.(*Editor); ok { //need this type assertion to have statement below
				p = ed //p is the global representing the current editor
				break
			}
		}

		vim.BufferSetCurrent(p.vbuf)
		sess.positionWindows()
		sess.eraseRightScreen()
		sess.drawRightScreen()
		sess.showEdMessage("Some editors had no write since the last change")

	} else { // we've been able to quit all editors because none were in modified state
		sess.editorMode = false
		vim.BufferSetCurrent(org.vbuf) ///////////////////////////////////////////////////////////
		sess.eraseRightScreen()

		if sess.divider < 10 {
			sess.cfg.ed_pct = 80
			moveDividerPct(80)
		}

		//org.readTitleIntoBuffer() // shouldn't be necessary
		org.drawPreview()
		sess.returnCursor() //because main while loop if started in editor_mode -- need this 09302020
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
	sess.showEdMessage("Line numbering is %t", e.numberLines)
}

func (e *Editor) goFormat() {
	bb := [][]byte{}
	//cmd := exec.Command("gofmt")
	cmd := exec.Command("goimports")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		sess.showEdMessage("Problem in gofmt stdout: %v", err)
		return
	}
	buf_out := bufio.NewReader(stdout)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		sess.showEdMessage("Problem in gofmt stdin: %v", err)
		return
	}
	err = cmd.Start()
	if err != nil {
		sess.showEdMessage("Problem in cmd.Start (gofmt) stdin: %v", err)
		return
	}

	for _, row := range e.bb {
		io.WriteString(stdin, string(row)+"\n")
	}
	stdin.Close()

	for {
		bytes, err := buf_out.ReadBytes('\n')

		if err == io.EOF {
			break
		}

		/*
			if len(bytes) == 0 {
				break
			}
		*/

		bb = append(bb, bytes[:len(bytes)-1])
	}
	e.bb = bb

	vim.BufferSetLines(e.vbuf, e.bb)
	pos := vim.CursorGetPosition()
	e.fr = pos[0] - 1
	e.fc = utf8.RuneCount(e.bb[e.fr][:pos[1]])
	e.scroll()
	e.drawText()
	sess.returnCursor()
	/*
		err = v.Command(fmt.Sprintf("w temp/buf%d", e.vbuf))
		if err != nil {
			sess.showEdMessage("Error in writing file in dbfunc: %v", err)
		}
	*/

}

func (e *Editor) rename() {
	pos := strings.Index(e.command_line, " ")
	if pos == -1 {
		sess.showEdMessage("You need to provide a filename")
		return
	}
	newName := e.command_line[pos+1:]
	sendRenameRequest(uint32(e.fr), uint32(e.fc), newName) //+1 seems to work better
}

func (e *Editor) createPDF() {
	pos := strings.Index(e.command_line, " ")
	if pos == -1 {
		sess.showEdMessage("You need to provide a filename")
		return
	}
	filename := e.command_line[pos+1:]
	content := bytes.Join(e.bb, []byte("\n"))
	pf := mdtopdf.NewPdfRenderer("", "", filename, "trace.log")
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

	err := pf.Process(content)
	if err != nil {
		sess.showEdMessage("pdf error:%v", err)
	}
}

func (e *Editor) printDocument() {
	if taskFolder(e.id) == "code" {
		c := taskContext(e.id)
		var ok bool
		var lang string
		if lang, ok = Languages[c]; !ok {
			sess.showEdMessage("I don't recognize the language")
			return
		}
		note := readNoteIntoString(e.id)
		var buf bytes.Buffer
		// github seems to work pretty well for printer output
		_ = Highlight(&buf, note, lang, "html", "github")

		f, err := os.Create("output.html")
		if err != nil {
			sess.showEdMessage("Error creating output.html: %v", err)
			return
		}
		defer f.Close()

		_, err = f.WriteString(buf.String())
		if err != nil {
			sess.showEdMessage("Error writing output.html: %s: %v", err)
			return
		}
		cmd := exec.Command("wkhtmltopdf", "--enable-local-file-access",
			"--no-background", "--minimum-font-size", "16", "output.html", "output.pdf")
		err = cmd.Run()
		if err != nil {
			sess.showEdMessage("Error creating pdf from code: %v", err)
		}
	} else {
		content := bytes.Join(e.bb, []byte("\n"))
		pf := mdtopdf.NewPdfRenderer("", "", "output.pdf", "trace.log")
		pf.TBody = mdtopdf.Styler{Font: "Arial", Style: "", Size: 12, Spacing: 2,
			TextColor: mdtopdf.Color{Red: 0, Green: 0, Blue: 0},
			FillColor: mdtopdf.Color{Red: 255, Green: 255, Blue: 255}}

		err := pf.Process(content)
		if err != nil {
			sess.showEdMessage("pdf error:%v", err)
		}
	}
	cmd := exec.Command("lpr", "output.pdf")
	err := cmd.Run()
	if err != nil {
		sess.showEdMessage("Error printing document: %v", err)
	}
}
