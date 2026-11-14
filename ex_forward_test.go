package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

func TestClassifyEx(t *testing.T) {
	forward := []string{
		// editing
		"s/a/b/", "%s/a/b/g", "'<,'>s/a/b/", "s#a#b#", "sg", "&&", "~",
		"g/pat/d", "v/pat/d", "g/pat/normal dd", "g/pat/s/a/b/", "g!/pat/d", "g/pat/",
		"d", "1,5d", "dl", "y", "pu", "t.", "m0", "'<,'>m$", "j", ">", "<", "'<,'>>",
		"normal Aend", "'<,'>normal .", "%norm A;", "sort", "'<,'>sort u", "sort /pat/", "retab", "center", "le", "ri",
		"u", "undo", "red", "earlier 1m", "later 10s",
		// moving and marking
		"42", "$", "'a", "ka", "mark b", "delm a-z", "=",
		// state and information
		"noh", "set ts=4", "se ts?", "setl sw=2", "let x = 5", "echo x", "echo 'a|b'", "echo 1 || 0", "call setline(1, 'x')",
		"reg", "di", "marks", "ju", "changes", "undol", "his", "mes", "dig", "as", "ls", "pwd", "ve",
		"let", "let g:", "let x", "let x = 1 | let", "p", "1,3nu", "li", "map", "ab", "com",
		// mappings and commands
		"map Q gq", "nnoremap <F2> :noh<CR>", "iab teh the", "unmap Q", "command Foo echo 1", "au BufEnter * echo 1",
		"so vimango.vim", "ru! plugin/x.vim", "r notes.txt", "sleep 100m", "spellgood vimango", "spellr",
		// modifiers
		"silent s/a/b/", "sil! %s/a/b/", "keeppatterns %s/a/b/", "keepj normal G", "lockmarks d", "3verbose set ts?", "sandbox echo 1",
		// chained
		"noh | set ts=4", "d | d", "%s/a/b/ | noh", "let x = 1 | echo x",
		// not vim's built-ins: vim answers for itself
		"Mycommand arg", "zzz", `" a comment`,
	}
	for _, line := range forward {
		if d := classifyEx(line); d.action != exForward {
			t.Errorf("classifyEx(%q): action %d (%s), want forwarded", line, d.action, d.reason)
		}
	}

	actions := map[string]exAction{
		"w": exSave, "write": exSave, "wri": exSave, "w!": exSave, "wa": exSave, "silent w": exSave,
		"up": exSaveIfModified, "update": exSaveIfModified,
		"wq": exSaveQuit, "wq!": exSaveQuit, "x": exSaveQuit, "xit": exSaveQuit, "exi": exSaveQuit, "exit": exSaveQuit, "wqa": exSaveQuit, "xa": exSaveQuit,
		"q": exQuit, "quit": exQuit, "qa": exQuit, "clo": exQuit,
		"q!": exQuitForce, "quit!": exQuitForce, "qa!": exQuitForce,
		"h": exHelp, "help": exHelp, "hel write": exHelp,
	}
	for line, want := range actions {
		if d := classifyEx(line); d.action != want {
			t.Errorf("classifyEx(%q): action %d (%s), want %d", line, d.action, d.reason, want)
		}
	}
	if d := classifyEx("help write"); d.arg != "write" {
		t.Errorf(":help write: topic %q", d.arg)
	}

	refused := []string{
		// files
		"w notes.txt", "w! notes.txt", "w >> log", "wri x", "wq out.md", "x out.md", "up x", "sav copy.md", "1,5w part.txt", "'<,'>w sel.txt", "%w",
		"e other.md", "e!", "ene", "fin x", "vie x", "n", "N", "prev", "fir", "la", "arga x", "argdo d", "dr x", "f newname",
		"cd /tmp", "lcd ..", "chd /", "tcd /", "mks", "mkv", "mkvimrc", "mkexrc", "redir > out.txt", "redi @a", "wundo u", "rundo u", "wv", "rv", "ol",
		"pre", "rec", "helpt .", "pa foo", "luaf x", "pyf x", "rubyf x",
		// windows, buffers, tabs
		"sp", "split", "vs", "new", "vne", "on", "hid", "res 10", "winc w", "windo d", "b2", "bn", "bp", "bd", "bw", "bun", "ball", "bad x", "bufdo d",
		"sb 2", "sbn", "tabnew", "tabe x", "tabn", "tabc", "tabdo d", "pc", "ped x", "diffthis", "diffs x", "cope", "lop", "opt", "helpg x", "spelld",
		"vertical s/a/b/", "tab echo 1", "bot echo 1", "hide echo 1", "conf q", "bro w",
		// programs
		"!ls", "!rm -rf x", "'<,'>!sort", "sh", "mak", "gr foo", "vim /x/ *", "ter", "cs find", "st", "sus",
		// needs more than one line
		"a", "append", "i", "c", "o", "vi", "ex", "fu Foo()", "if 1", "endif", "wh 1", "for i in [1]", "try", "X", "P",
		// neither
		"sy on", "fo", "foldo", "startinsert", "star", "stopi", "debug echo 1", "prof start x", "lua print(1)", "py print(1)", "gui", "sign list", "colo x", "filt /x/ ls",
		// the c flag
		"s/a/b/c", "%s/a/b/gc", "s/a/b/cg | noh", "&&c",
		// chained and nested
		"noh | w notes.txt", "noh | w", "d | sp", "set ts=4 | e x", "%s/a/b/ | w x", "echo 1 | w x", "let x = 1 | q",
		"g/pat/w >> log", "g/pat/s/a/b/c", "v/pat/sp", "g/pat/!rm x", "silent w x", "sil! e x", "keepj bn",
		// ranges on save and close
		"1,5w", "'<,'>x", "%q",
	}
	for _, line := range refused {
		d := classifyEx(line)
		if d.action != exRefuse {
			t.Errorf("classifyEx(%q): action %d, want refused", line, d.action)
		} else if d.reason == "" {
			t.Errorf("classifyEx(%q): refused without a reason", line)
		}
	}
}

// Every built-in command is accounted for, and the list of what is forwarded
// holds no command that takes file names other than the three that only read.
func TestExPolicyCoversVimsTable(t *testing.T) {
	known := map[string]bool{}
	for _, c := range vim.ExCommands() {
		known[c.Name] = true
		_, mine := exVimangoActions[c.Name]
		if exAllowed[c.Name] && mine {
			t.Errorf(":%s is both forwarded and vimango's own", c.Name)
		}
		if exAllowed[c.Name] && c.Flags&vim.ExXFile != 0 && c.Name != "source" && c.Name != "runtime" && c.Name != "read" {
			t.Errorf(":%s takes file names and is on the forwarded list", c.Name)
		}
		if !exAllowed[c.Name] && !mine {
			if d := exRefusal(c); d.action != exRefuse || !strings.Contains(d.reason, ":"+c.Name+" is not available") {
				t.Errorf(":%s has no refusal: %+v", c.Name, d)
			}
		}
	}
	for name := range exAllowed {
		if !known[name] {
			t.Errorf("forwarded list names %q, which is not a vim command", name)
		}
	}
	for name := range exVimangoActions {
		if !known[name] {
			t.Errorf("vimango's own list names %q, which is not a vim command", name)
		}
	}
	for _, m := range exModifiers {
		if !known[m.name] {
			t.Errorf("modifier %q is not in vim's table", m.name)
		}
	}
}

// message returns what the editor showed for an ex command, without escapes.
func exMessage(t *testing.T, e *Editor, line string) string {
	t.Helper()
	out := captureStdout(t, func() { e.exCommand(line) })
	var b strings.Builder
	for i := 0; i < len(out); i++ {
		if out[i] == 0x1b {
			for i < len(out) && !(out[i] >= 'A' && out[i] <= 'Z' || out[i] >= 'a' && out[i] <= 'z') {
				i++
			}
			continue
		}
		b.WriteByte(out[i])
	}
	return b.String()
}

// vim's commands work on the note as they do in vim, and the editor's copy of
// the text and cursor follows.
func TestForwardedCommandsEditTheNote(t *testing.T) {
	e := newTestEditor(t, "delta", "alpha", "charlie", "bravo")
	withOrganizer(t, e)

	e.sendKeys(":sort")
	if redraw := e.editorProcessKey('\r'); !redraw {
		t.Error(":sort returned redraw=false; the change would not be drawn")
	}
	if got := strings.Join(e.ss, " "); got != "alpha bravo charlie delta" {
		t.Errorf("after :sort e.ss = %q", got)
	}

	e.exCommand("3")
	if e.fr != 2 {
		t.Errorf(":3 left the cursor on row %d, want 2", e.fr)
	}
	e.exCommand("d")
	e.exCommand("%s/a/A/g")
	e.exCommand("1m$")
	e.exCommand("$normal Aend")
	if got := strings.Join(e.ss, " "); got != "brAvo deltA AlphAend" {
		t.Errorf("after :d, :%%s, :m, :normal e.ss = %q", got)
	}
	e.exCommand("u")
	if got := e.line(2); got != "AlphA" {
		t.Errorf("after :u line 3 is %q", got)
	}
	e.exCommand("t.") // the cursor is on the last line: a copy below it
	if len(e.ss) != 4 {
		t.Errorf("after :t. there are %d lines, want 4", len(e.ss))
	}

	// a visual-mode range, as vim writes it when ':' is pressed
	vim.SetCursorPosition(1, 0)
	e.sendKeys("Vj:")
	e.sendKeys("s/A/x/g")
	e.editorProcessKey('\r')
	if e.line(0) != "brxvo" || e.line(1) != "deltx" || e.line(2) != "AlphA" {
		t.Errorf("after :'<,'>s the buffer is %q", e.vbuf.Lines())
	}
	if e.mode != NORMAL {
		t.Errorf("mode after a forwarded command is %v", e.mode)
	}
}

// What vim says comes back: output, and its own error for a command it does
// not have.
func TestForwardedCommandsShowVimsReply(t *testing.T) {
	e := newTestEditor(t, "alpha")
	withOrganizer(t, e)

	if msg := exMessage(t, e, "echo 6*7"); !strings.Contains(msg, "42") {
		t.Errorf(":echo showed %q", msg)
	}
	if msg := exMessage(t, e, "set tabstop?"); !strings.Contains(msg, "tabstop=8") {
		t.Errorf(":set tabstop? showed %q", msg)
	}
	if msg := exMessage(t, e, "s/nothere/x/"); !strings.Contains(msg, "E486") {
		t.Errorf("a failed :s showed %q", msg)
	}
	if msg := exMessage(t, e, "nosuchcommand"); !strings.Contains(msg, "E492") {
		t.Errorf("an unknown command showed %q", msg)
	}
	// several lines go to the overlay :help uses
	e.sendKeys("mamb")
	if msg := exMessage(t, e, "marks"); e.mode != HELP || !strings.Contains(msg, "mark") {
		t.Errorf(":marks: mode %v, showed %q", e.mode, msg)
	}
	e.editorProcessKey('\x1b')
	if e.mode == HELP {
		t.Error("Esc did not close the overlay")
	}
}

// Refused commands change nothing: no file, no other buffer, the same working
// directory, the note's text as it was.
func TestRefusedCommandsDoNothing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	e := newTestEditor(t, "alpha", "bravo")
	a := editorOnly(t, e)

	for _, line := range []string{
		"w notes.txt", "w! notes.txt", "wq notes.txt", "sav notes.txt", "'<,'>w part.txt", "redir > out.txt",
		"e other.txt", "ene", "bn", "b1", "sp", "new", "tabnew", "cd /", "lcd /", "!touch made-by-shell", "sh",
		"noh | w notes.txt", "g/alpha/w >> log.txt", "s/a/b/c", "append",
	} {
		msg := exMessage(t, e, line)
		if !strings.Contains(msg, "vimango") {
			t.Errorf(":%s was refused with %q", line, msg)
		}
		if id := vim.GetCurrentBuffer().GetID(); id != e.vbuf.GetID() {
			t.Fatalf(":%s left vim on buffer %d, not the note's", line, id)
		}
	}

	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("refused commands created files: %v", entries)
	}
	if wd, _ := os.Getwd(); !sameDir(wd, dir) {
		t.Errorf("working directory is now %s", wd)
	}
	if got := strings.Join(e.vbuf.Lines(), "|"); got != "alpha|bravo" {
		t.Errorf("the note changed: %q", got)
	}
	if !a.Run || a.Session.editor != e {
		t.Error("a refused command closed the editor")
	}
}

// vim starts no program, by whatever route the request reaches it: typed, from
// vimscript, from a mapping or from a sourced file. The check on what was
// typed (classifyEx) cannot see most of these; vim's restricted mode stops
// them where the shell would start.
func TestVimStartsNoProgram(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("in.txt", []byte("from a file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	script := "!touch by-script-bang\ncall system('touch by-script-system')\nr !touch by-script-read\n"
	if err := os.WriteFile("script.vim", []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	e := newTestEditor(t, "alpha", "bravo")
	editorOnly(t, e)

	// typed on the command line: vim says why not
	for _, line := range []string{
		"r !touch by-read", "r!touch by-read-bang", "0r !touch by-read-range", "read ++enc=utf-8 !touch by-read-opt",
		"silent r !touch by-read-silent", "g/alpha/r !touch by-read-global",
		"r `touch by-backtick`", "source `touch by-source-backtick`", "runtime `touch by-runtime-backtick`",
		"echo system('touch by-system')", "call system('touch by-call')", "let x = systemlist('touch by-systemlist')",
		"execute 'r !touch by-execute'", "execute '!touch by-execute-bang'",
		"call delete('in.txt')", "call rename('in.txt', 'moved.txt')", "call mkdir('made')",
		// ran the patch program until libvim checked where the shell starts
		// (src/apitest/file_writes.c there); here it stops sooner, at the
		// temporary file it would write
		"execute 'diffpatch in.txt'",
	} {
		if msg := exMessage(t, e, line); !strings.Contains(msg, "not allowed") && !strings.Contains(msg, "vimango") {
			t.Errorf(":%s was answered with %q, want a refusal", line, msg)
		}
	}
	// and by routes that never pass the command line
	e.exCommand(`normal :r !touch by-normal` + "\r")
	e.exCommand("source script.vim")
	e.exCommand("nnoremap Q :r !touch by-mapping<CR>")
	e.sendKeys("Q")
	e.exCommand("nnoremap Q :!touch by-mapping-bang<CR>")
	e.sendKeys("Q")

	entries, _ := os.ReadDir(dir)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if got := strings.Join(names, " "); got != "in.txt script.vim" {
		t.Errorf("the directory now holds %q; a program ran, or a file was moved or made", got)
	}
	if got := strings.Join(e.vbuf.Lines(), "|"); got != "alpha|bravo" {
		t.Errorf("the note changed: %q", got)
	}
	if m := vim.GetCurrentMode(); m != vim.ModeNormal && m != vim.ModeNormalBusy { // NORMAL, or NORMAL just after a key
		t.Errorf("vim was left in mode %d", m)
	}

	// reading a file is still vim's :read
	e.exCommand("$r in.txt")
	if got := strings.Join(e.vbuf.Lines(), "|"); got != "alpha|bravo|from a file" {
		t.Errorf("after :$r in.txt the note is %q", got)
	}
}

func sameDir(a, b string) bool {
	ra, _ := filepath.EvalSymlinks(a)
	rb, _ := filepath.EvalSymlinks(b)
	return ra == rb
}

// The checks after a forwarded command, for anything the list lets through
// that it should not have: forwardEx is called directly here, past the
// classifier, with commands known to do the damage.
func TestForwardExRepairsWhatACommandBroke(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	e := newTestEditor(t, "alpha")
	withOrganizer(t, e)

	out := captureStdout(t, func() { e.forwardEx("enew") })
	if vim.GetCurrentBuffer().GetID() != e.vbuf.GetID() {
		t.Fatal("after a command that switched buffers, vim is not back on the note's")
	}
	if !strings.Contains(out, "went back") {
		t.Errorf("no word of the buffer switch: %q", out)
	}
	e.exCommand("normal Aok")
	if e.line(0) != "alphaok" {
		t.Errorf("the next edit went to %q", e.vbuf.Lines())
	}

	captureStdout(t, func() { e.forwardEx("cd /") })
	if wd, _ := os.Getwd(); !sameDir(wd, dir) {
		t.Errorf("after a command that changed directory, the process is in %s", wd)
	}
}

// Saving and closing, in vim's other spellings, are vimango's: the database,
// not a file.
func TestVimSpellingsOfSaveAndClose(t *testing.T) {
	dir := t.TempDir()
	t.Run("wq saves to the database and closes", func(t *testing.T) {
		e, a := savedNoteEditor(t, "alpha")
		t.Chdir(dir)
		e.sendKeys("x")
		e.exCommand("wq")
		if note, _ := noteInDB(t, e); note != "lpha" || a.Run {
			t.Errorf("after :wq the database holds %q and Run is %v", note, a.Run)
		}
	})
	t.Run("wri saves", func(t *testing.T) {
		e, a := savedNoteEditor(t, "alpha")
		e.sendKeys("x")
		e.exCommand("wri")
		if note, _ := noteInDB(t, e); note != "lpha" || !a.Run || e.isModified() {
			t.Errorf("after :wri the database holds %q, Run %v, modified %v", note, a.Run, e.isModified())
		}
	})
	t.Run("update saves only a modified note", func(t *testing.T) {
		e, _ := savedNoteEditor(t, "alpha")
		e.exCommand("up")
		if _, modified := noteInDB(t, e); modified != oldStamp {
			t.Error(":update wrote a clean note")
		}
		e.sendKeys("x")
		e.exCommand("up")
		if note, _ := noteInDB(t, e); note != "lpha" {
			t.Errorf(":update on a modified note left %q", note)
		}
	})
	t.Run("a failed wq stays open", func(t *testing.T) {
		e, a := savedNoteEditor(t, "alpha")
		e.sendKeys("x")
		e.Store.DB().Close()
		e.exCommand("wq")
		if !a.Run || !e.isModified() {
			t.Error(":wq closed the editor although the note was not saved")
		}
	})
	t.Run("clo and qa close; q! discards", func(t *testing.T) {
		for _, line := range []string{"clo", "qa"} {
			e, a := savedNoteEditor(t, "alpha")
			e.exCommand(line)
			if a.Run {
				t.Errorf(":%s did not close a clean note", line)
			}
		}
		e, a := savedNoteEditor(t, "alpha")
		e.sendKeys("x")
		e.exCommand("qa")
		if !a.Run {
			t.Error(":qa closed a modified note")
		}
		e.exCommand("qa!")
		if a.Run {
			t.Error(":qa! did not close")
		}
	})
	t.Run("hel shows vimango's help", func(t *testing.T) {
		e, _ := savedNoteEditor(t, "alpha")
		if msg := exMessage(t, e, "hel write"); e.mode != HELP || !strings.Contains(msg, "write") {
			t.Errorf(":hel write: mode %v, showed %q", e.mode, msg)
		}
	})
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("save and close commands created files: %v", entries)
	}
}

// Every command on the forwarded list is run once in libvim, with arguments
// where it needs them, to check the claim the list makes: afterwards vim is
// still on the note's buffer and in normal mode, the working directory has not
// moved, and nothing was written to disk. (A command that waited for input
// would hang this test.)
func TestForwardedListIsSafeInLibvim(t *testing.T) {
	repo, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	t.Chdir(dir)
	for name, text := range map[string]string{"in.txt": "from a file\n", "src.vim": "let g:sourced = 1\n"} {
		if err := os.WriteFile(name, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	e := newTestEditor(t, "alpha", "bravo", "charlie", "delta")
	withOrganizer(t, e)
	// the overlay renders with a glamour style, found by path
	screenOf(e).glamourStyle = filepath.Join(repo, "default.json")
	vim.ExecuteCommand("let g:locked = 1")
	vim.ExecuteCommand("let @a = 'x'")

	samples := map[string]string{
		"substitute": "s/a/b/", "global": "g/zz/d", "vglobal": "v/./d", "normal": "normal x", "k": "ka", "mark": "mark b",
		"delmarks": "delmarks a", "@": "@a", "*": "*a", "goto": "goto 1", "sleep": "sleep 1m", "earlier": "earlier 1f", "later": "later 1f",
		"set": "set ts=4", "setlocal": "setlocal sw=2", "setglobal": "setglobal sw=2", "let": "let g:x = 1", "unlet": "unlet g:x",
		"lockvar": "lockvar g:locked", "unlockvar": "unlockvar g:locked", "echo": "echo 1", "echon": "echon 1", "echomsg": "echomsg 'm'",
		"echoerr": "echoerr 'e'", "echohl": "echohl None", "call": "call setline(1, 'alpha')", "execute": "execute 'normal l'",
		"abbreviate": "abbreviate teh the", "iabbrev": "iabbrev teh the", "cabbrev": "cabbrev teh the", "noreabbrev": "noreabbrev adn and",
		"inoreabbrev": "inoreabbrev adn and", "cnoreabbrev": "cnoreabbrev adn and", "unabbreviate": "unabbreviate teh",
		"iunabbrev": "iunabbrev adn", "cunabbrev": "cunabbrev adn",
		"command": "command Foo echo 1", "delcommand": "delcommand Foo", "delfunction": "delfunction Nothere",
		"autocmd": "autocmd User Vimango echo 1", "augroup": "augroup vimangotest", "doautocmd": "doautocmd User Vimango", "doautoall": "doautoall User Vimango",
		"source": "source src.vim", "runtime": "runtime nothere.vim", "read": "read in.txt",
		"spellgood": "spellgood vimango", "spellwrong": "spellwrong vimango", "spellundo": "spellundo vimango", "sort": "sort", "copy": "copy 0",
		"move": "move 0", "t": "t 0", "put": "put a", "z": "z#", "history": "history :", "digraphs": "digraphs", "retab": "retab 4",
	}
	var names []string
	for name := range exAllowed {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		line, ok := samples[name]
		switch {
		case ok:
		case exMapCommand.MatchString(name) && strings.Contains(name, "unmap"):
			line = name + " Q"
		case exMapCommand.MatchString(name) && !strings.Contains(name, "clear"):
			line = name + " Q x"
		default:
			line = name
		}
		if d := classifyEx(line); d.action != exForward {
			t.Errorf("sample %q for :%s is not forwarded: %s", line, name, d.reason)
			continue
		}

		out := captureStdout(t, func() { e.exCommand(line) })
		if e.mode == HELP { // multi-line output went to the overlay
			e.editorProcessKey('\x1b')
		}
		if strings.Contains(out, "went back") || strings.Contains(out, "changed it back") {
			t.Errorf(":%s (%q) needed repair afterwards: %q", name, line, out)
		}
		if id := vim.GetCurrentBuffer().GetID(); id != e.vbuf.GetID() {
			t.Fatalf(":%s (%q) left vim on buffer %d", name, line, id)
		}
		if m := vim.GetCurrentMode(); m != vim.ModeNormal && m != vim.ModeNormalBusy {
			t.Errorf(":%s (%q) left vim in mode %d", name, line, m)
			vim.SendKey("<esc>")
		}
		if wd, _ := os.Getwd(); !sameDir(wd, dir) {
			t.Fatalf(":%s (%q) moved the working directory to %s", name, line, wd)
		}
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if entry.Name() != "in.txt" && entry.Name() != "src.vim" {
				t.Errorf(":%s (%q) created %s", name, line, entry.Name())
				os.Remove(entry.Name())
			}
		}
	}
	t.Logf("%d forwarded commands checked", len(names))
}

// Pressing ':' with a selection, or after a count, opens the command line with
// the range vim puts there, and it is on screen before anything is typed.
func TestCommandLineOpensWithVimsRange(t *testing.T) {
	e := newTestEditor(t, "alpha", "bravo", "charlie", "delta")
	withOrganizer(t, e)

	e.sendKeys("Vj")
	out := captureStdout(t, func() { e.editorProcessKey(':') })
	if e.mode != EX_COMMAND || e.command_line != "'<,'>" || e.cmdLineCursor != len("'<,'>") {
		t.Errorf("':' in VISUAL: mode %v, line %q, cursor %d; want EX_COMMAND, \"'<,'>\", 5", e.mode, e.command_line, e.cmdLineCursor)
	}
	if !strings.Contains(out, ":'<,'>") {
		t.Errorf("the range was not drawn when the line opened: %q", out)
	}
	e.sendKeys("d")
	e.editorProcessKey('\r')
	if got := strings.Join(e.vbuf.Lines(), " "); got != "charlie delta" {
		t.Errorf("after :'<,'>d the note is %q", got)
	}

	e.sendKeys("2")
	captureStdout(t, func() { e.editorProcessKey(':') })
	if e.command_line != ".,.+1" {
		t.Errorf("':' after a count of 2 opened with %q, want \".,.+1\"", e.command_line)
	}
	e.editorProcessKey('\x1b')

	captureStdout(t, func() { e.editorProcessKey(':') })
	if e.command_line != "" {
		t.Errorf("a plain ':' opened with %q", e.command_line)
	}
}

// keyMessage returns what the editor showed while handling keys, without escapes.
func keyMessage(t *testing.T, e *Editor, keys string) string {
	t.Helper()
	out := captureStdout(t, func() {
		for _, r := range keys {
			e.editorProcessKey(int(r))
		}
	})
	var b strings.Builder
	for i := 0; i < len(out); i++ {
		if out[i] == 0x1b {
			for i < len(out) && !(out[i] >= 'A' && out[i] <= 'Z' || out[i] >= 'a' && out[i] <= 'z') {
				i++
			}
			continue
		}
		b.WriteByte(out[i])
	}
	return b.String()
}

// What vim says about a normal-mode key reaches the message line: a search
// that finds nothing, a mark that is not set, an empty register.
func TestVimMessagesForKeysAreShown(t *testing.T) {
	e := newTestEditor(t, "alpha bravo", "charlie")
	withOrganizer(t, e)

	msg := keyMessage(t, e, "/nowhere\r")
	if !strings.Contains(msg, "E486: Pattern not found: nowhere") {
		t.Errorf("a failed search showed %q", msg)
	}
	if strings.Count(msg, "E486") != 1 {
		t.Errorf("the failed search was reported %d times: %q", strings.Count(msg, "E486"), msg)
	}
	// vim warns that it is going round the end before it knows the search
	// will fail; the error is what stays on vim's screen, and here
	if strings.Contains(msg, "search hit") {
		t.Errorf("a failed search also showed the wrap warning: %q", msg)
	}
	if msg := keyMessage(t, e, "'z"); !strings.Contains(msg, "E20: Mark not set") {
		t.Errorf("a jump to an unset mark showed %q", msg)
	}
	if msg := keyMessage(t, e, `"zp`); !strings.Contains(msg, "E353: Nothing in register z") {
		t.Errorf("a put from an empty register showed %q", msg)
	}
	if msg := keyMessage(t, e, "\x07"); !strings.Contains(msg, "line 1 of 2") || strings.Contains(msg, "No Name") { // Ctrl-G
		t.Errorf("Ctrl-G showed %q", msg)
	}

	// vim has nothing to say about most keys, and then the message line is
	// left alone: nothing is written at all
	for _, keys := range []string{"l", "w", "j", "k", "x", "yy", "p", "J", "r!"} {
		if msg := keyMessage(t, e, keys); msg != "" {
			t.Errorf("%q wrote %q to the screen", keys, msg)
		}
	}
	// in INSERT the mode message stays until Esc
	if msg := keyMessage(t, e, "i"); !strings.Contains(msg, "-- INSERT --") {
		t.Errorf("entering INSERT showed %q", msg)
	}
	if msg := keyMessage(t, e, "abc"); msg != "" {
		t.Errorf("typing in INSERT wrote %q to the screen", msg)
	}
	e.editorProcessKey('\x1b')
}

// vim's reports and warnings for keys: how many lines a yank or delete took,
// what undo did, a search going round the end of the note.
func TestVimReportsForKeysAreShown(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "row"
	}
	lines[4], lines[14], lines[24] = "row 5 foo", "row 15 foo", "row 25 foo"
	e := newTestEditor(t, lines...)
	withOrganizer(t, e)

	for _, tc := range []struct{ keys, want string }{
		{"gg5yy", "5 lines yanked"},
		{"p", "5 more lines"},
		{"u", "5 fewer lines"},
		{"4dd", "4 fewer lines"},
		{"u", "4 more lines"},
		{"3>>", "3 lines >ed 1 time"},
		{"u", "before #"},
		{"g\x07", "Line 1 of 30"}, // g Ctrl-G
	} {
		if msg := keyMessage(t, e, tc.keys); !strings.Contains(msg, tc.want) {
			t.Errorf("%q showed %q, want %q", tc.keys, msg, tc.want)
		}
	}

	// a search that goes round the end says so, once, on n and N
	keyMessage(t, e, "gg/foo\r")
	if msg := keyMessage(t, e, "n"); strings.Contains(msg, "search hit") {
		t.Errorf("n without a wrap showed %q", msg)
	}
	keyMessage(t, e, "n")
	if msg := keyMessage(t, e, "n"); strings.Count(msg, "search hit BOTTOM, continuing at TOP") != 1 {
		t.Errorf("n round the end showed %q", msg)
	}
	if msg := keyMessage(t, e, "N"); !strings.Contains(msg, "search hit TOP, continuing at BOTTOM") {
		t.Errorf("N round the top showed %q", msg)
	}

	// and on the Enter of a typed search. libvim used to get this one wrong:
	// both directions for a search that did not wrap, nothing for one that did
	if msg := keyMessage(t, e, "20G/foo\r"); strings.Contains(msg, "search hit") {
		t.Errorf("a typed search that did not wrap showed %q", msg)
	}
	if e.fr != 24 {
		t.Errorf("the search landed on row %d, want 24", e.fr)
	}
	if msg := keyMessage(t, e, "/foo\r"); strings.Count(msg, "search hit BOTTOM, continuing at TOP") != 1 || strings.Contains(msg, "hit TOP") {
		t.Errorf("a typed search round the end showed %q", msg)
	}
	if e.fr != 4 {
		t.Errorf("the search landed on row %d, want 4", e.fr)
	}
	if msg := keyMessage(t, e, "?foo\r"); strings.Count(msg, "search hit TOP, continuing at BOTTOM") != 1 || strings.Contains(msg, "hit BOTTOM") {
		t.Errorf("a typed backward search round the top showed %q", msg)
	}
	if e.fr != 24 {
		t.Errorf("the backward search landed on row %d, want 24", e.fr)
	}
}

// A typed search does what it does in vim. libvim ran two searches for it,
// from wherever the incremental search had left the cursor, so an offset was
// reported as "Pattern not found", an empty pattern did not repeat, and a
// search that failed could move the cursor.
func TestTypedSearch(t *testing.T) {
	e := newTestEditor(t, "one foo", "two", "three foo", "four")
	withOrganizer(t, e)
	t.Cleanup(func() { vim.ExecuteCommand("set wrapscan") })
	at := func(what string, row, col int) {
		t.Helper()
		if e.fr != row || e.fc != col {
			t.Errorf("%s: cursor at row %d, column %d; want %d, %d", what, e.fr, e.fc, row, col)
		}
	}

	if msg := keyMessage(t, e, "/foo/e\r"); strings.Contains(msg, "E486") {
		t.Errorf("/foo/e showed %q", msg)
	}
	at("/foo/e", 0, 6)
	if msg := keyMessage(t, e, "gg/foo/+1\r"); strings.Contains(msg, "E486") {
		t.Errorf("/foo/+1 showed %q", msg)
	}
	at("/foo/+1", 1, 0)

	// an empty pattern is the last one again
	keyMessage(t, e, "gg/foo\r")
	at("/foo", 0, 4)
	keyMessage(t, e, "/\r")
	at("/ after /foo", 2, 6)

	// back to where the search started
	keyMessage(t, e, "``")
	at("``", 0, 4)

	// a search that fails leaves the cursor where it was, though the
	// incremental search had shown the match on the way
	vim.ExecuteCommand("set nowrapscan")
	if msg := keyMessage(t, e, "G/one\r"); !strings.Contains(msg, "E385") {
		t.Errorf("a search that may not wrap showed %q", msg)
	}
	at("a failed search", 3, 0)
	vim.ExecuteCommand("set wrapscan")

	// and so does <esc>
	keyMessage(t, e, "/foo\x1b")
	at("an abandoned search", 3, 0)
}

// A forwarded command's report goes on the message line; only a listing
// takes the overlay.
func TestForwardedCommandReports(t *testing.T) {
	e := newTestEditor(t, "alpha", "alpha", "alpha", "bravo", "bravo", "bravo")
	withOrganizer(t, e)

	if msg := exMessage(t, e, "%s/alpha/x/"); !strings.Contains(msg, "3 substitutions on 3 lines") || e.mode == HELP {
		t.Errorf(":%%s showed %q (mode %v)", msg, e.mode)
	}
	if msg := exMessage(t, e, "1,3co$ | 4,6co$"); strings.Count(msg, "3 more lines") != 2 || e.mode == HELP {
		t.Errorf("two reports showed %q (mode %v); want both on the message line", msg, e.mode)
	}
	if msg := exMessage(t, e, "u"); !strings.Contains(msg, "6 fewer lines") {
		t.Errorf(":u showed %q", msg)
	}
	// a report does not silence the next one (libvim used to "keep" the :s count)
	if msg := exMessage(t, e, "4,6d"); !strings.Contains(msg, "3 fewer lines") {
		t.Errorf(":4,6d after a :s showed %q", msg)
	}
	if msg := keyMessage(t, e, "u"); !strings.Contains(msg, "3 more lines") {
		t.Errorf("u showed %q", msg)
	}
	if msg := exMessage(t, e, "silent %s/x/y/"); strings.Contains(msg, "substitutions") {
		t.Errorf(":silent still showed %q", msg)
	}
	e.sendKeys("ma")
	if exMessage(t, e, "marks"); e.mode != HELP {
		t.Errorf(":marks did not open the overlay (mode %v)", e.mode)
	}
	e.editorProcessKey('\x1b')
}

// What a command lists is shown: on the message line when it is one line, in
// the overlay when it is more. libvim used to drop it -- :registers, :history,
// :messages, :version, :map, :print and :let said nothing, and :let never
// came back.
func TestListingsAreShown(t *testing.T) {
	e := newTestEditor(t, "alpha", "bravo", "charlie")
	withOrganizer(t, e)
	vim.ExecuteCommand("mapclear | mapclear!")
	vim.ExecuteCommand("nnoremap ,d dd")
	vim.ExecuteCommand("messages clear")
	vim.ExecuteCommand("let g:listed = 7")
	t.Cleanup(func() { vim.ExecuteCommand("mapclear | mapclear!") })
	e.sendKeys("yy")

	closeOverlay := func() {
		if e.mode == HELP {
			captureStdout(t, func() { e.editorProcessKey('\x1b') })
		}
	}
	overlay := []struct{ line, want string }{
		{"registers", `--- Registers ---`},
		{"registers", `"0   alpha^J`},
		{"1,3print", "alpha\r\n"}, // each line on its own
		{"2,3#", "  3 charlie"},
		{"history", "cmd history"},
		{"history -3,", "2,3#"}, // what was typed above
		{"version", "VIM - Vi IMproved"},
		{"let", "listed                #7"},
	}
	for _, c := range overlay {
		msg := exMessage(t, e, c.line)
		if e.mode != HELP || !strings.Contains(msg, c.want) {
			t.Errorf(":%s: mode %v, want the overlay with %q; showed %q", c.line, e.mode, c.want, msg)
		}
		closeOverlay()
	}

	messageLine := []struct{ line, want string }{
		{"map", "n  ,d          * dd"},
		{"1print", "alpha"},
		{"let g:listed", "g:listed              #7"},
		{"echon 'a' | echon 'b'", "ab"},
	}
	for _, c := range messageLine {
		msg := exMessage(t, e, c.line)
		if e.mode == HELP || !strings.Contains(msg, c.want) {
			t.Errorf(":%s: mode %v, want %q on the message line; showed %q", c.line, e.mode, c.want, msg)
		}
		closeOverlay()
	}

	// :messages is what vim has said, and :echomsg is how a script says it
	exMessage(t, e, "messages clear")
	exMessage(t, e, "echomsg 'first thing'")
	exMessage(t, e, "echomsg 'second thing'")
	if msg := exMessage(t, e, "messages"); !strings.Contains(msg, "first thing") || !strings.Contains(msg, "second thing") {
		t.Errorf(":messages showed %q", msg)
	}
	closeOverlay()

	// said once: these send a message of their own
	if msg := exMessage(t, e, "echo 'only once'"); !strings.HasSuffix(msg, "once'only once") {
		t.Errorf(":echo showed %q", msg)
	}
	// and nothing at all under :silent
	if msg := exMessage(t, e, "silent registers"); strings.Contains(msg, "Registers") || e.mode == HELP {
		t.Errorf(":silent registers showed %q (mode %v)", msg, e.mode)
	}
}

// A libvim.a from before the fork listed anything links without complaint,
// and never comes back from a listing of variables. vimango asks the library
// it has, and refuses the listing when it is that old.
func TestOldLibraryDoesNotListVariables(t *testing.T) {
	vim.InitializeVim(0)
	if !vim.ListsOutput() {
		t.Fatal("this libvim.a does not send listings: rebuild it (./scripts/build-libvim.sh)")
	}
	for _, line := range []string{"let", "let g:", "let g:x", "noh | let"} {
		if d := classifyEx(line); d.action != exForward {
			t.Errorf(":%s is refused: %s", line, d.reason)
		}
	}

	libvimListsOutput = func() bool { return false }
	t.Cleanup(func() { libvimListsOutput = vim.ListsOutput })
	for _, line := range []string{"let", "let g:", "let g:x", "noh | let"} {
		if d := classifyEx(line); d.action == exForward {
			t.Errorf("with an old library :%s is forwarded, and would hang", line)
		}
	}
	if d := classifyEx("let g:x = 1"); d.action != exForward {
		t.Errorf("with an old library an assignment is refused: %s", d.reason)
	}
}

// vim writes no file, by whatever route: the check on what was typed
// (classifyEx) refuses ":w name" and its relatives by name, but does not look
// inside vimscript, where ":execute 'w name'" and writefile() used to write.
// libvim now refuses where a file would be opened for writing.
func TestVimWritesNoFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("kept.txt", []byte("original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	script := "w by-script\ncall writefile(['x'], 'by-script-writefile')\nredir > by-script-redir\nsilent echo 1\nredir END\n"
	if err := os.WriteFile("script.vim", []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	e := newTestEditor(t, "alpha", "bravo")
	editorOnly(t, e)

	// typed: refused by vimango's list or by vim, and either says so
	for _, line := range []string{
		"w by-w", "1,2w by-range", "saveas by-saveas", "update by-update", "wq by-wq", "x by-x",
		"call writefile(['x'], 'by-writefile')", "call writefile(['x'], 'kept.txt')", "call writefile(['x'], 'kept.txt', 'a')",
		"execute 'w by-execute'", "execute 'w! kept.txt'", "execute 'saveas by-execute-saveas'", "execute '1,2w by-execute-range'",
		"execute 'redir > by-redir'", "execute 'mksession by-mksession'", "execute 'mkexrc by-mkexrc'", "execute 'mkview by-mkview'",
		"execute 'mkspell by-mkspell kept.txt'",
	} {
		if msg := exMessage(t, e, line); !strings.Contains(msg, "not allowed") && !strings.Contains(msg, "vimango") {
			t.Errorf(":%s was answered with %q, want a refusal", line, msg)
		}
		e.exCommand("redir END")
	}
	// and by routes that never pass the command line
	e.exCommand(`normal :w by-normal` + "\r")
	e.exCommand("source script.vim")
	e.exCommand("nnoremap Q :w by-mapping<CR>")
	e.sendKeys("Q")
	e.exCommand("nnoremap Q :call writefile(['x'], 'by-mapping-writefile')<CR>")
	e.sendKeys("Q")
	e.exCommand("set swapfile directory=.")
	e.sendKeys("ix\x1b")
	e.exCommand("set verbosefile=by-verbosefile verbose=1")
	e.exCommand("set verbose=0 verbosefile=")

	entries, _ := os.ReadDir(dir)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if got := strings.Join(names, " "); got != "kept.txt script.vim" {
		t.Errorf("the directory now holds %q; vim wrote a file", got)
	}
	if kept, _ := os.ReadFile("kept.txt"); string(kept) != "original\n" {
		t.Errorf("vim changed a file that was there: %q", kept)
	}
	if got := vim.GetCurrentBuffer(); got.GetID() != e.vbuf.GetID() {
		t.Error("vim is no longer on the note's buffer")
	}
	// reading is untouched
	if msg := exMessage(t, e, "echo readfile('kept.txt')[0]"); !strings.Contains(msg, "original") {
		t.Errorf("readfile() with writes off answered %q", msg)
	}
}
