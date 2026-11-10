package vim

import "testing"

func TestExCommandTable(t *testing.T) {
	cmds := ExCommands()
	if len(cmds) < 500 {
		t.Fatalf("parsed %d ex commands from ex_cmds.h, expected over 500", len(cmds))
	}
	flags := map[string]ExFlags{}
	for _, c := range cmds {
		flags[c.Name] = c.Flags
	}
	for name, want := range map[string]ExFlags{
		"write":      ExRange | ExBang | ExXFile | ExExtra | ExTrlBar,
		"edit":       ExBang | ExXFile | ExExtra | ExTrlBar,
		"substitute": ExRange | ExExtra,
		"normal":     ExRange | ExBang | ExExtra,
		"nohlsearch": ExTrlBar,
		"!":          ExRange | ExBang | ExXFile | ExExtra,
	} {
		got, ok := flags[name]
		if !ok {
			t.Errorf("%q is not in the table", name)
		} else if got&(ExRange|ExBang|ExXFile|ExExtra|ExTrlBar) != want {
			t.Errorf("%q has flags %#x, want %#x", name, got, want)
		}
	}
}

// Abbreviations resolve as they do in vim.
func TestFindExCommand(t *testing.T) {
	for _, tc := range []struct{ in, name, rest string }{
		{"s/a/b/", "substitute", "/a/b/"},
		{"su/a/b/", "substitute", "/a/b/"},
		{"sg", "substitute", "g"}, // :s with the g flag, not a command "sg"
		{"sil echo 1", "silent", " echo 1"},
		{"sre", "srewind", ""},
		{"w", "write", ""},
		{"w!", "write", "!"},
		{"wri foo.txt", "write", " foo.txt"},
		{"w>>log", "write", ">>log"},
		{"wq", "wq", ""},
		{"wqa", "wqall", ""},
		{"x", "xit", ""},
		{"exi", "exit", ""},
		{"up", "update", ""},
		{"sav copy.md", "saveas", " copy.md"},
		{"e other", "edit", " other"},
		{"ene", "enew", ""},
		{"q", "quit", ""},
		{"q!", "quit", "!"},
		{"qa", "qall", ""},
		{"clo", "close", ""},
		{"b2", "buffer", "2"},
		{"bn", "bnext", ""},
		{"sp", "split", ""},
		{"vs", "vsplit", ""},
		{"tabnew", "tabnew", ""},
		{"!ls", "!", "ls"},
		{"r file", "read", " file"},
		{"so vimango.vim", "source", " vimango.vim"},
		{"cd /tmp", "cd", " /tmp"},
		{"d", "delete", ""},
		{"dl", "delete", ""},  // :delete with the l flag
		{"di", "display", ""}, // but :di is :display
		{"g/x/d", "global", "/x/d"},
		{"v/x/d", "vglobal", "/x/d"},
		{"norm dd", "normal", " dd"},
		{"noh", "nohlsearch", ""},
		{"se ts=4", "set", " ts=4"},
		{"ka", "k", "a"},
		{"m0", "move", "0"},
		{"t.", "t", "."},
		{"u", "undo", ""},
		{"red", "redo", ""},
		{"reg", "registers", ""},
		{"j", "join", ""},
		{">", ">", ""},
		{"&&", "&", "&"},
		{"@a", "@", "a"},
		{"h spell", "help", " spell"},
		{"sor u", "sort", " u"},
		{"py3 print(1)", "py3", " print(1)"},
	} {
		cmd, rest, ok := FindExCommand(tc.in)
		if !ok || cmd.Name != tc.name || rest != tc.rest {
			t.Errorf("FindExCommand(%q) = %q, rest %q, ok %v; want %q, rest %q", tc.in, cmd.Name, rest, ok, tc.name, tc.rest)
		}
	}
	for _, in := range []string{"", "Mycommand", "zzz", "42", "wfoo"} {
		if cmd, _, ok := FindExCommand(in); ok {
			t.Errorf("FindExCommand(%q) resolved to %q; want no built-in command", in, cmd.Name)
		}
	}
}

// What vim would have shown on its message line comes back from a capture.
func TestCaptureMessages(t *testing.T) {
	InitializeVim(0)
	buf := NewBuffer(0)
	SetCurrentBuffer(buf)
	buf.SetLines(0, -1, []string{"alpha"})

	run := func(cmd string) []Message {
		return CaptureMessages(func() { SendInput(":" + cmd + "\r") })
	}
	if m := run("echo 'hi'"); len(m) != 1 || m[0].Text != "hi" || m[0].Error {
		t.Errorf(":echo gave %+v", m)
	}
	if m := run("set tabstop?"); len(m) != 1 || m[0].Text != "  tabstop=8" {
		t.Errorf(":set tabstop? gave %+v", m)
	}
	if m := run("nosuchcommand"); len(m) != 1 || !m[0].Error || m[0].Text != "E492: Not an editor command: nosuchcommand" {
		t.Errorf("an unknown command gave %+v", m)
	}
	if m := run("s/alpha/beta/"); len(m) != 0 || buf.Line(1) != "beta" {
		t.Errorf("a quiet command gave %+v, line %q", m, buf.Line(1))
	}
	// outside a capture, messages are dropped as before
	SendInput(":echo 'dropped'\r")
	if m := run("echo 'kept'"); len(m) != 1 || m[0].Text != "kept" {
		t.Errorf("capture after an uncaptured message gave %+v", m)
	}
}
