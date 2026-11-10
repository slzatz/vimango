package main

import (
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/slzatz/vimango/vim"
)

// What happens to an ex command typed in the note editor.
//
// vimango has a few commands of its own (:sync, :spell, :checkstale, ...) and
// its own meaning for saving and closing: a note lives in the database, so
// :w, :wq, :x and :q -- in any spelling vim accepts -- write the row or close
// the editor, never a file. Everything else is vim's, and is forwarded to
// libvim if it makes sense there.
//
// libvim is vim with no screen, one window and (for the editor) one buffer.
// Most of vim's 558 ex commands edit text or state and work as they always
// do. Some would do harm: :w name writes a file, :e and :bn leave the note's
// buffer for another (every later command then edits the wrong text), :cd
// moves the process's working directory out from under config.json and the
// database. Others need a window, a prompt or a shell that is not there. So
// the rule is a list of what is forwarded, exAllowed below; any other
// built-in command is refused with a reason. A name vim's table does not have
// (a user-defined :Command, a typo) is forwarded, and vim answers for itself.
//
// The check reads what was typed, including every command on a line joined
// with | and the command given to :g. It does not look inside vimscript:
// :execute, :normal and :call run whatever they are given. That is on
// purpose; the list guards against habit (:w notes.txt), not against intent.
//
// Starting a program and writing a file are the exceptions, and are not decided
// here at all. vim runs in restricted mode with file writes off
// (vim.InitializeVim), so ":r !cmd", a `cmd` in a file name, system() and
// ":!cmd" from a mapping or a script are refused by vim itself, with E145, and
// so are ":execute 'w name'", ":redir > name" and writefile(), wherever they
// come from. This list still refuses ":w name" by name, with a better message.

type exAction int

const (
	exForward        exAction = iota // send the line to vim
	exRefuse                         // not available here; reason says why
	exSave                           // :w
	exSaveIfModified                 // :update
	exSaveQuit                       // :wq, :x
	exQuit                           // :q
	exQuitForce                      // :q!
	exHelp                           // :help [arg]
)

type exDecision struct {
	action exAction
	reason string // exRefuse: shown to the user
	arg    string // exHelp: the topic
	// owned: the command is one of vim's spellings of save, close or help,
	// which are vimango's whatever was decided -- including a refusal, as for
	// ":w notes.txt". The registry of vimango's own commands is not consulted.
	owned bool
}

func refuse(reason string) exDecision { return exDecision{action: exRefuse, reason: reason} }

// vim's spellings of save and close, and what each means here.
var exVimangoActions = map[string]exAction{
	"write": exSave, "wall": exSave, "update": exSaveIfModified,
	"wq": exSaveQuit, "wqall": exSaveQuit, "xit": exSaveQuit, "xall": exSaveQuit, "exit": exSaveQuit,
	"quit": exQuit, "qall": exQuit, "quitall": exQuit, "close": exQuit,
	"help": exHelp,
}

// exAllowed is every built-in ex command that is forwarded to vim, by full
// name. Adding one is a decision that it is safe with one buffer, no window
// and no file: check what it does in libvim first (the vim package's tests
// show how).
var exAllowed = map[string]bool{}

func init() {
	for _, name := range strings.Fields(`
		substitute & ~ global vglobal delete yank put copy t move join > < normal sort retab center left right
		undo redo earlier later undojoin
		k mark delmarks @ * goto = print list number # z nohlsearch
		registers display marks jumps changes clearjumps undolist history messages digraphs ascii scriptnames version pwd
		ls buffers files
		set setlocal setglobal let unlet lockvar unlockvar echo echon echomsg echoerr echohl call execute
		abbreviate iabbrev cabbrev noreabbrev inoreabbrev cnoreabbrev unabbreviate iunabbrev cunabbrev abclear iabclear cabclear
		command delcommand comclear delfunction autocmd augroup doautocmd doautoall
		source runtime read sleep
		spellgood spellwrong spellundo spellrepall spellinfo`) {
		exAllowed[name] = true
	}
	for _, c := range vim.ExCommands() {
		if exMapCommand.MatchString(c.Name) {
			exAllowed[c.Name] = true
		}
	}
}

// :map and its relatives: nmap, inoremap, vunmap, cmapclear, ...
var exMapCommand = regexp.MustCompile(`^[nvxsoilct]?(nore|un)?map(clear)?$`)

// Modifiers that are harmless in front of a command, and ones that ask for a
// window, a tab or a dialog. Name and the shortest abbreviation vim accepts.
var exModifiers = []struct {
	name   string
	minLen int
	ok     bool
}{
	{"aboveleft", 3, false}, {"belowright", 3, false}, {"botright", 2, false}, {"browse", 3, false},
	{"confirm", 4, false}, {"filter", 4, false}, {"hide", 3, false},
	{"keepalt", 5, true}, {"keepjumps", 5, true}, {"keepmarks", 3, true}, {"keeppatterns", 5, true},
	{"leftabove", 5, false}, {"lockmarks", 3, true}, {"noautocmd", 3, true}, {"noswapfile", 3, true},
	{"rightbelow", 6, false}, {"sandbox", 3, true}, {"silent", 3, true}, {"tab", 3, false},
	{"topleft", 2, false}, {"unsilent", 3, true}, {"verbose", 4, true}, {"vertical", 4, false},
}

// Why a built-in command that is not forwarded is refused.
var exReasons = []struct {
	reason string
	names  *regexp.Regexp
}{
	{"vimango finds its database and settings relative to the working directory", regexp.MustCompile(`^[lt]?(cd|chdir)$`)},
	{"vimango shows one note in one window", regexp.MustCompile(
		`^(split|vsplit|new|vnew|sview|sfind|sall|sargument|s[Nn]ext|sprevious|srewind|sfirst|slast|sb.*|stag|stjump|stselect|only|hide|unhide|sunhide|resize|wincmd|windo|p(close|edit|pop|search)|pt.*|tab.*|buffer|b[Nn]ext|ball|badd|bdelete|bfirst|blast|bmodified|bprevious|brewind|bufdo|bunload|bwipeout|diff.*|[cl]open|[cl]window|[cl]close|options|help.*|spelldump)$`)},
	{"running other programs from the editor is not supported", regexp.MustCompile(
		`^(!|shell|l?make|l?grep(add)?|l?vimgrep(add)?|terminal|l?cscope|scscope|cstag|suspend|stop)$`)},
	{"it needs more input than one command line", regexp.MustCompile(
		`^(append|insert|change|open|visual|ex|function|if|elseif|else|endif|while|endwhile|for|endfor|try|catch|finally|endtry|endfunction|return|break|continue|finish|throw|debug|Print|X|promptfind|promptrepl)$`)},
}

func exRefusal(cmd vim.ExCommand) exDecision {
	why := "libvim has no use for it here"
	if cmd.Flags&vim.ExXFile != 0 {
		why = "it works on files, and a note lives in the database"
	}
	for _, r := range exReasons {
		if r.names.MatchString(cmd.Name) {
			why = r.reason
		}
	}
	return refuse(":" + cmd.Name + " is not available in vimango: " + why)
}

// skipExRange returns what follows the line range at the start of an ex
// command line: "%s/a/b/" -> "s/a/b/", "'<,'>sort" -> "sort", "42" -> "". A
// port of vim's skip_range (ex_docmd.c), so the two agree on where the
// command starts.
func skipExRange(s string) string {
	i := 0
loop:
	for i < len(s) && strings.IndexByte(" \t0123456789.$%'/?-+,;\\", s[i]) >= 0 {
		switch c := s[i]; c {
		case '\\': // \/ \? \& are the last search or substitute pattern
			if i+1 < len(s) && strings.IndexByte("?/&", s[i+1]) >= 0 {
				i++
			} else {
				break loop
			}
		case '\'': // a mark: skip its name
			i++
		case '/', '?': // a search pattern, up to its closing delimiter
			i++
			for i < len(s) && s[i] != c {
				if s[i] == '\\' && i+1 < len(s) {
					i++
				}
				i++
			}
		}
		if i < len(s) {
			i++
		}
	}
	for i < len(s) && strings.IndexByte(" \t:", s[i]) >= 0 {
		i++
	}
	return s[i:]
}

// exModifier reports whether s starts with the modifier name (at least minLen
// letters of it, then something that is not a letter), and what follows.
func exModifier(s, name string, minLen int) (rest string, ok bool) {
	i := 0
	for i < len(s) && i < len(name) && s[i] == name[i] {
		i++
	}
	if i < minLen || (i < len(s) && (s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z')) {
		return s, false
	}
	return strings.TrimLeft(s[i:], " \t"), true
}

// exNextCommand finds the | that ends the command whose arguments are args
// and returns what follows it. quotes says the arguments are an expression,
// where | inside a string, and ||, do not end the command.
func exNextCommand(args string, quotes bool) (next string, found bool) {
	var quote byte
	for i := 0; i < len(args); i++ {
		c := args[i]
		switch {
		case c == '\\' || c == 0x16: // backslash or Ctrl-V: the next character is literal
			i++
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case quotes && (c == '"' || c == '\''):
			quote = c
		case c == '|':
			if quotes && i+1 < len(args) && args[i+1] == '|' {
				i++
				continue
			}
			return args[i+1:], true
		}
	}
	return "", false
}

// exAfterPattern skips a /pattern/ whose opening delimiter is s[0] and
// returns what follows the closing one ("" if it is not closed).
func exAfterPattern(s string) string {
	delim := s[0]
	for i := 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++
		} else if s[i] == delim {
			return s[i+1:]
		}
	}
	return ""
}

func isExDelimiter(c byte) bool {
	return !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == ' ' || c == '\t' || c == '"' || c == '|' || c == '\\')
}

// classifyEx decides what to do with an ex command line (without its ':').
func classifyEx(line string) exDecision {
	return classifyExCommand(line, true)
}

// classifyExCommand classifies one command and, through it, the ones chained
// after it. first is true for the command the user typed at the start of the
// line: only that one may be vimango's own save, close or help.
func classifyExCommand(line string, first bool) exDecision {
	s := strings.TrimLeft(line, " \t:")
	afterRange := skipExRange(s)
	hasRange := len(afterRange) != len(s)

	// modifiers, each possibly followed by another
	for {
		matched := false
		for _, m := range exModifiers {
			from := s
			if m.name == "tab" || m.name == "verbose" { // these take a count in front: :3verbose
				from = afterRange
			}
			rest, ok := exModifier(from, m.name, m.minLen)
			if !ok {
				continue
			}
			if !m.ok {
				return refuse(":" + m.name + " is not available in vimango: vimango shows one note in one window")
			}
			if strings.HasPrefix(rest, "!") { // :silent!
				rest = strings.TrimLeft(rest[1:], " \t")
			}
			s, matched = strings.TrimLeft(rest, " \t:"), true
			afterRange = skipExRange(s)
			hasRange = len(afterRange) != len(s)
			break
		}
		if !matched {
			break
		}
	}
	s = afterRange

	if s == "" || s[0] == '"' {
		return exDecision{action: exForward} // a bare range (":42" goes to line 42), or a comment
	}
	if s[0] == '|' {
		return classifyExCommand(s[1:], false)
	}

	cmd, rest, ok := vim.FindExCommand(s)
	if !ok {
		// not a built-in: a user-defined command, or nothing vim knows. Vim says which.
		return exDecision{action: exForward}
	}

	if action, mine := exVimangoActions[cmd.Name]; mine {
		if !first {
			return refuse(":" + cmd.Name + " cannot follow another command in vimango; give it on its own")
		}
		owned := func(d exDecision) exDecision { d.owned = true; return d }
		bang := strings.HasPrefix(rest, "!")
		arg := strings.TrimSpace(strings.TrimPrefix(rest, "!"))
		if action == exHelp {
			return owned(exDecision{action: exHelp, arg: arg})
		}
		switch {
		case hasRange:
			return owned(refuse(":" + cmd.Name + " takes no range in vimango: it saves or closes the whole note"))
		case arg != "" && (action == exSave || action == exSaveIfModified || action == exSaveQuit):
			return owned(refuse(":" + cmd.Name + " takes no file name in vimango: it saves the note to the database"))
		case arg != "":
			return owned(refuse(":" + cmd.Name + " takes no argument in vimango"))
		case action == exQuit && bang:
			return owned(exDecision{action: exQuitForce})
		}
		return owned(exDecision{action: action})
	}

	if !exAllowed[cmd.Name] {
		return exRefusal(cmd)
	}
	// :let with no assignment lists variables, and libvim never comes back
	// from the listing: it waits, inside the C call, for a key to page with.
	// (Found by TestForwardedCommandsDoNotHang, which runs every forwarded
	// command; this is the only one that does it.)
	if cmd.Name == "let" && !strings.Contains(rest, "=") {
		return refuse(":let without an assignment lists variables, which libvim cannot show; use :echo for a value")
	}

	// The command is forwarded. Does another one follow it?
	var next string
	var chained bool
	switch name := cmd.Name; {
	case name == "normal" || name == "autocmd" || name == "doautocmd" || name == "doautoall" || name == "command" ||
		exMapCommand.MatchString(name) || strings.Contains(name, "abbrev"):
		// the rest of the line is theirs, bars included

	case name == "global" || name == "vglobal":
		// :g/pattern/command -- the command runs on every matching line
		pat := strings.TrimPrefix(rest, "!")
		if pat == "" || !isExDelimiter(pat[0]) {
			break
		}
		if sub := strings.TrimSpace(exAfterPattern(pat)); sub != "" {
			if d := classifyExCommand(sub, false); d.action != exForward {
				return d
			}
		}

	case name == "substitute" || name == "&" || name == "~":
		// [/pattern/replacement/] [flags] [count], then possibly | another command
		tail := rest
		if name == "substitute" && tail != "" && isExDelimiter(tail[0]) {
			delim := tail[:1]
			tail = exAfterPattern(tail) // past the pattern
			if tail != "" {
				tail = exAfterPattern(delim + tail) // past the replacement
			}
		}
		flags := tail
		if i := strings.IndexAny(tail, " \t|"); i >= 0 {
			flags = tail[:i]
		}
		if strings.Contains(flags, "c") {
			return refuse(":s with the c flag asks before each change, and vimango has no prompt for that")
		}
		next, chained = exNextCommand(tail, false)

	case cmd.Flags&vim.ExTrlBar != 0:
		next, chained = exNextCommand(rest, false)

	default: // :echo, :let, :call, :execute: an expression, then possibly | another command
		next, chained = exNextCommand(rest, true)
	}
	if chained {
		if d := classifyExCommand(next, false); d.action != exForward {
			return d
		}
	}
	return exDecision{action: exForward}
}

// forwardEx sends an ex command line to vim, shows what vim says back, and
// brings the editor's copy of the buffer and cursor up to date. It reports
// whether the text changed and so needs drawing.
//
// Three checks follow every forwarded command, whatever the list above let
// through, because each of these would quietly break the editor: vim left on
// another buffer, vim left in some mode other than normal, and the process's
// working directory moved (vimango finds its database and config relative to
// it).
func (e *Editor) forwardEx(line string) (redraw bool) {
	cwd, cwdErr := os.Getwd()
	msgs := vim.CaptureMessages(func() { vim.SendInput(":" + line + "\r") })

	var problems []string
	if m := vim.GetCurrentMode(); m != vim.ModeNormal && m != vim.ModeNormalBusy { // not NORMAL: insert, visual, an unfinished command line
		vim.SendKey("<esc>")
	}
	if vim.GetCurrentBuffer().GetID() != e.vbuf.GetID() {
		vim.SetCurrentBuffer(e.vbuf)
		problems = append(problems, "that command left the note's buffer; vimango went back to it")
	}
	if now, err := os.Getwd(); cwdErr == nil && err == nil && now != cwd {
		os.Chdir(cwd)
		problems = append(problems, "that command changed the working directory; vimango changed it back")
	}

	e.resyncFromVim()
	if tick := e.vbuf.GetLastChangedTick(); tick > e.bufferTick {
		e.bufferTick = tick
		redraw = true
	}

	e.showVimMessages(msgs, problems)
	return redraw
}

// showVimMessages puts what vim said about a forwarded command where the user
// can see it: an error, or a report or two, on the message line; a listing
// (:marks, :ls) in the same overlay :help uses.
func (e *Editor) showVimMessages(msgs []vim.Message, problems []string) {
	var lines, errors []string
	listing := false
	for _, m := range msgs {
		text := strings.Trim(m.Title+m.Text, "\n")
		// a title, or several lines in one message, is a table: keep its shape
		listing = listing || (!m.Error && (m.Title != "" || strings.Contains(text, "\n")))
		for _, l := range strings.Split(text, "\n") {
			if l = strings.TrimRight(l, " \t\r"); strings.TrimSpace(l) == "" {
				continue
			} else if m.Error {
				errors = append(errors, strings.TrimSpace(l))
			} else {
				lines = append(lines, l)
			}
		}
	}
	errors = append(errors, problems...)

	// what fits on the message line: one report, or a few short ones
	oneLine := ""
	if !listing {
		parts := make([]string, len(lines))
		for i, l := range lines {
			parts[i] = strings.TrimSpace(l)
		}
		oneLine = strings.Join(parts, "; ")
	}
	width := e.Screen.screenCols - e.Screen.divider

	switch {
	case len(errors) > 0:
		e.ShowMessage(BR, "\x1b[41m%s\x1b[0m", strings.Join(errors, "; "))
	case len(lines) == 0:
		e.ShowMessage(BR, "")
	case !listing && len(oneLine) <= width:
		e.ShowMessage(BR, "%s", oneLine)
	default:
		// as a code block, so markdown rendering leaves vim's columns alone
		e.mode = HELP
		app.Organizer.drawNotice("```\n" + strings.Join(lines, "\n") + "\n```")
		app.Organizer.altRowoff = 0
	}
}

// showKeyMessages shows what vim said about a key typed in the editor, on the
// message line: an error ("E486: Pattern not found", "E20: Mark not set"), a
// report ("5 lines yanked", "4 fewer lines", what undo did, g Ctrl-G's word
// count) or a warning ("search hit BOTTOM, continuing at TOP"). Nothing is
// shown, and the line is left as it is, when vim said nothing -- which is
// most keys.
//
// Until 2026-11-10 none of this reached the user, and it was not a decision.
// libvim has no screen and hands its messages to a callback: vimango had
// never registered one, and libvim itself dropped everything but errors
// until the fork's 2da770b06.
//
// endsSearch says the key was the Enter that completes a typed /pattern.
// libvim's wrap warning is not to be trusted there -- it searches again from
// where the incremental search left the cursor, reporting a wrap that did not
// happen and missing one that did (the fork's PATCHES.md) -- so warnings are
// dropped for that one key. n, N, * and # report wraps correctly.
func (e *Editor) showKeyMessages(msgs []vim.Message, endsSearch bool) {
	var texts []string
	isError := false
	for _, m := range msgs {
		if m.Warning && endsSearch {
			continue
		}
		for _, l := range strings.Split(m.Title+m.Text, "\n") {
			// Ctrl-G leads with vim's idea of the file -- "[No Name]"
			// [Modified] -- which means nothing here: the note has no file,
			// and vim thinks every buffer it never wrote is modified.
			l = strings.TrimPrefix(strings.TrimSpace(l), `"[No Name]" `)
			l = strings.TrimPrefix(l, "[Modified] ")
			if l == "" || slices.Contains(texts, l) { // vim says some things twice
				continue
			}
			texts = append(texts, l)
			isError = isError || m.Error
		}
	}
	if len(texts) == 0 {
		return
	}
	if isError {
		e.ShowMessage(BR, "\x1b[41m%s\x1b[0m", strings.Join(texts, "; "))
	} else {
		e.ShowMessage(BR, "%s", strings.Join(texts, "; "))
	}
}
