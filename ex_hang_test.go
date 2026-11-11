package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slzatz/vimango/vim"
)

// TestExProbeHelper is not a test: TestForwardedCommandsDoNotHang runs this
// binary again with VIMANGO_EX_PROBE set, and this runs that one command line
// in libvim and prints what happened. A command that waits for input never
// returns from libvim (and cannot be interrupted from Go), which is why each
// one gets a process of its own.
func TestExProbeHelper(t *testing.T) {
	line, ok := os.LookupEnv("VIMANGO_EX_PROBE")
	if !ok {
		t.Skip("helper for TestForwardedCommandsDoNotHang")
	}
	t.Chdir(t.TempDir())
	vim.InitializeVim(0)
	buf := vim.NewBuffer(0)
	vim.SetCurrentBuffer(buf)
	buf.SetLines(0, -1, []string{"alpha", "bravo", "charlie"})
	vim.ExecuteCommand("let g:probe = 1 | nmap Q x | iabbrev teh the | command Foo echo 1 | autocmd User Probe echo 1")
	msgs := vim.CaptureMessages(func() { vim.SendInput(":" + line + "\r") })
	entries, _ := os.ReadDir(".")
	fmt.Printf("PROBE-DONE msgs=%d mode=%d samebuf=%v files=%d\n", len(msgs), vim.GetCurrentMode(), vim.GetCurrentBuffer().GetID() == buf.GetID(), len(entries))
}

// exProbe runs one command line in a fresh process and reports whether libvim
// came back from it.
func exProbe(line string) (returned bool, report string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestExProbeHelper$", "-test.v")
	cmd.Env = append(os.Environ(), "VIMANGO_EX_PROBE="+line)
	out, _ := cmd.CombinedOutput()
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "PROBE-DONE") {
			return true, l
		}
	}
	if ctx.Err() != nil {
		return false, "did not return within 5s"
	}
	tail := strings.TrimSpace(string(out))
	if len(tail) > 300 {
		tail = tail[len(tail)-300:]
	}
	return false, "died: " + tail
}

// A forwarded command must come back. libvim has no screen, and a command
// that waits for one never returns -- inside a C call, so the editor would
// freeze with the note unsaved. Listing variables (bare :let) did: it padded
// a column until a counter that the screen code used to advance reached it
// (the fork fixed that in 6fc194385). This runs every forwarded command, bare
// and with arguments, each in its own process with a time limit.
func TestForwardedCommandsDoNotHang(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a process per command")
	}
	var lines []string
	for name := range exAllowed {
		lines = append(lines, name)
		if exMapCommand.MatchString(name) && !strings.Contains(name, "clear") && !strings.Contains(name, "unmap") {
			lines = append(lines, name+" Q x")
		}
	}
	lines = append(lines,
		"s/a/b/", "%s/a/b/g", "g/a/d", "g/a/p", "v/a/d", "normal x", "ka", "mark b", "delmarks!", "@a", "goto 1", "sleep 1m",
		"earlier 1f", "later 1f", "sort u", "copy 0", "move 0", "t 0", "put a", "z#", "history :", "history all", "retab 4",
		"set all", "set ts=4", "set ts?", "setlocal all", "setglobal all", "set termcap",
		"let g:x = 1", "let g:", "let b:", "let w:", "let v:version", "let g:probe", "unlet g:probe", "lockvar g:probe",
		"echo 1", "echo g:", "echo v:", "echomsg 'm'", "echoerr 'e'", "call setline(1, 'x')", "execute 'normal l'",
		"abbreviate teh the", "unabbreviate teh", "command Bar echo 2", "command Foo", "delcommand Foo",
		"autocmd User Probe", "autocmd User", "augroup vimangotest", "doautocmd User Probe",
		"source nothere.vim", "runtime nothere.vim", "read nothere.txt", "spellgood vimango", "registers a", "display a", "marks a",
		"42", "$", "noh | set ts=4", "Foo", "zzz",
	)
	slices.Sort(lines)
	lines = slices.Compact(lines)

	type result struct {
		line     string
		returned bool
		report   string
	}
	results := make([]result, len(lines))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, line := range lines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ok, report := exProbe(line)
			results[i] = result{line, ok, report}
		}()
	}
	wg.Wait()

	for _, r := range results {
		forwarded := classifyEx(r.line).action == exForward
		switch {
		case forwarded && !r.returned:
			t.Errorf(":%s is forwarded, and libvim %s", r.line, r.report)
		case forwarded && !strings.Contains(r.report, "samebuf=true"):
			t.Errorf(":%s is forwarded, and left the buffer: %s", r.line, r.report)
		case forwarded && !strings.Contains(r.report, "files=0"):
			t.Errorf(":%s is forwarded, and wrote a file: %s", r.line, r.report)
		case !forwarded && r.returned:
			t.Logf(":%s is refused but would have returned (%s)", r.line, r.report)
		}
	}
	t.Logf("%d command lines probed", len(lines))
}
