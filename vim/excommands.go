package vim

import (
	_ "embed"
	"regexp"
	"strings"
	"sync"
)

// vim's table of ex commands: the fork's src/ex_cmds.h, the header libvim is
// built with. scripts/build-libvim.sh copies it here with libvim.a, and
// TestHeaderMatchesLibvim fails if the two differ, so the table here is the
// library's.
//
//go:embed ex_cmds.h
var exCmdsHeader string

// ExFlags are a command's flags from ex_cmds.h; only the ones the application
// needs are named.
type ExFlags uint32

const (
	ExRange  ExFlags = 0x001 // takes a line range
	ExBang   ExFlags = 0x002 // takes a ! after the name
	ExExtra  ExFlags = 0x004 // takes arguments
	ExXFile  ExFlags = 0x008 // its arguments are file names
	ExTrlBar ExFlags = 0x100 // a | ends it and starts another command
)

var exFlagBits = map[string]ExFlags{
	"RANGE": ExRange, "BANG": ExBang, "EXTRA": ExExtra, "XFILE": ExXFile, "TRLBAR": ExTrlBar,
	"FILES": ExXFile | ExExtra, "WORD1": ExExtra, "FILE1": ExXFile | ExExtra,
}

// ExCommand is one built-in ex command.
type ExCommand struct {
	Name  string // the full name: "substitute", "write", "!"
	Flags ExFlags
}

var (
	exCommandsOnce sync.Once
	exCommands     []ExCommand
)

var exEntry = regexp.MustCompile(`EX\(\s*CMD_\w+,\s*"([^"]+)",\s*\w+,\s*([^,]+?),\s*\w+\s*\)`)

// ExCommands returns vim's built-in ex commands in table order, which is the
// order that decides what an abbreviation means: ":s" is the first command
// whose name starts with "s".
func ExCommands() []ExCommand {
	exCommandsOnce.Do(func() {
		for _, m := range exEntry.FindAllStringSubmatch(exCmdsHeader, -1) {
			var flags ExFlags
			for _, f := range strings.Split(m[2], "|") {
				flags |= exFlagBits[strings.TrimSpace(f)]
			}
			exCommands = append(exCommands, ExCommand{Name: m[1], Flags: flags})
		}
	})
	return exCommands
}

func isAlpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isAlnum(c byte) bool { return isAlpha(c) || c >= '0' && c <= '9' }

// FindExCommand reads the command name at the start of s -- what is left of a
// command line once its range and modifiers are gone -- and resolves it the
// way vim does (find_command in ex_docmd.c), abbreviations and all: "s", "su"
// and "substitute" are substitute, "w" and "wri" are write. rest is what
// follows the name.
//
// ok is false when no built-in command matches: s is empty, a user-defined
// command (those start with a capital), or not a command at all.
func FindExCommand(s string) (cmd ExCommand, rest string, ok bool) {
	if s == "" {
		return ExCommand{}, s, false
	}
	at := func(i int) byte {
		if i < len(s) {
			return s[i]
		}
		return 0
	}
	byName := func(name string) ExCommand {
		for _, c := range ExCommands() {
			if c.Name == name {
				return c
			}
		}
		return ExCommand{Name: name}
	}

	// vim's exceptions to "the name is the letters":
	// - k can be followed directly by its mark: ":ka"
	// - s can be followed directly by a flag: ":sg", ":si", ":sr", ":sc", but
	//   :sre[wind], :scr[iptnames], :scs[cope], :sim[alt], :sig[ns] and
	//   :sil[ent] are commands of their own
	if s[0] == 'k' {
		return byName("k"), s[1:], true
	}
	if s[0] == 's' && ((at(1) == 'c' && (at(2) == 0 || (at(2) != 's' && at(2) != 'r' && (at(3) == 0 || (at(3) != 'i' && at(4) != 'p'))))) ||
		at(1) == 'g' || (at(1) == 'i' && at(2) != 'm' && at(2) != 'l' && at(2) != 'g') || at(1) == 'I' || (at(1) == 'r' && at(2) != 'e')) {
		return byName("substitute"), s[1:], true
	}

	n := 0
	for n < len(s) && isAlpha(s[n]) {
		n++
	}
	if at(0) == 'p' && at(1) == 'y' { // :py3, :python3, :py3file
		for n < len(s) && isAlnum(s[n]) {
			n++
		}
	}
	if n == 0 && strings.IndexByte("@*!=><&~#", s[0]) >= 0 {
		n = 1
	}
	if n == 0 {
		return ExCommand{}, s, false
	}
	nameLen := n
	// ":dl" and ":dp" (and ":dell", ":delp") are :delete with the l or p flag
	if s[0] == 'd' && (s[n-1] == 'l' || s[n-1] == 'p') && strings.HasPrefix("delete", s[:n-1]) && !strings.HasPrefix("delete", s[:n]) {
		nameLen--
	}

	typed := s[:nameLen]
	for _, c := range ExCommands() {
		if strings.HasPrefix(c.Name, typed) {
			return c, s[n:], true
		}
	}
	return ExCommand{}, s, false
}
