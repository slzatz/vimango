package vim

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// libvimCheckout is the fork's checkout -- $LIBVIM_DIR, or ../libvim beside
// this repository, as scripts/build-libvim.sh has it -- and the flags a C
// compiler needs to read its headers. The test is skipped without one, or
// without a C compiler.
func libvimCheckout(t *testing.T) (src string, cflags []string) {
	t.Helper()
	dir := os.Getenv("LIBVIM_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "libvim")
	}
	dir, _ = filepath.Abs(dir)
	src = filepath.Join(dir, "src")
	for _, need := range []string{"libvim.h", "ex_cmds.h", filepath.Join("auto", "config.h")} {
		if _, err := os.Stat(filepath.Join(src, need)); err != nil {
			t.Skipf("no built libvim checkout at %s (%s is missing); run scripts/build-libvim.sh, or set LIBVIM_DIR", dir, need)
		}
	}
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("no C compiler (cc)")
	}
	cflags = []string{"-I" + src, "-I" + filepath.Join(src, "proto"), "-DHAVE_CONFIG_H", "-w"}
	if runtime.GOOS == "darwin" {
		if _, err := os.Stat("/opt/homebrew/include"); err == nil {
			cflags = append(cflags, "-I/opt/homebrew/include")
		}
	}
	return src, cflags
}

// layoutProbe prints the size and layout of every type libvim_api.h defines
// and the value of every constant. Compiled once against the fork's headers
// and once against libvim_api.h, it must print the same thing.
const layoutProbe = `
#include <stdio.h>
#include <stddef.h>
#define SIZE(t) printf(#t ": %zu bytes, signed %d\n", sizeof(t), (int)((t)-1 < (t)0))
#define STRUCT(t) printf(#t ": %zu bytes\n", sizeof(t))
#define FIELD(t, f) printf(#t "." #f ": offset %zu, %zu bytes\n", offsetof(t, f), sizeof(((t *)0)->f))
#define VALUE(c) printf(#c " = %lld\n", (long long)(c))
int main(void)
{
  SIZE(char_u); SIZE(linenr_T); SIZE(colnr_T);
  STRUCT(pos_T); FIELD(pos_T, lnum); FIELD(pos_T, col); FIELD(pos_T, coladd);
  SIZE(msgPriority_T); VALUE(MSG_INFO); VALUE(MSG_WARNING); VALUE(MSG_ERROR);
  STRUCT(searchHighlight_T); FIELD(searchHighlight_T, start); FIELD(searchHighlight_T, end);
  SIZE(spellErrorType_T);
  VALUE(SPELL_ERROR_BAD); VALUE(SPELL_ERROR_CAPS); VALUE(SPELL_ERROR_RARE); VALUE(SPELL_ERROR_LOCAL);
  STRUCT(spellError_T);
  FIELD(spellError_T, lnum); FIELD(spellError_T, col); FIELD(spellError_T, len); FIELD(spellError_T, type);
  STRUCT(VoidCallback); STRUCT(MessageCallback);
  SIZE(screenLineMotion_T); VALUE(MOTION_H); VALUE(MOTION_L); VALUE(MOTION_M);
  STRUCT(CursorMoveScreenLineCallback); STRUCT(CursorMoveScreenPositionCallback);
  VALUE(FORWARD); VALUE(BACKWARD);
  VALUE(OK);
  VALUE(NORMAL); VALUE(VISUAL); VALUE(OP_PENDING); VALUE(CMDLINE); VALUE(INSERT); VALUE(NORMAL_BUSY);
  return 0;
}
`

// libvim_api.h is this package's own copy of the libvim declarations it uses,
// and nothing in an ordinary build compares it with the library: a prototype
// or a struct that has changed in the fork would compile, link and misbehave.
// This compares them, against the fork's checkout.
func TestHeaderMatchesLibvim(t *testing.T) {
	src, cflags := libvimCheckout(t)
	api, err := filepath.Abs("libvim_api.h")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cc := func(name, source string, args ...string) (string, error) {
		t.Helper()
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("cc", append(append(args, cflags...), file)...).CombinedOutput()
		return string(out), err
	}

	// Prototypes and globals: both headers in one file. The types are the
	// fork's (VIMANGO_CHECK_AGAINST_LIBVIM leaves ours out), so a
	// declaration of ours that differs from the fork's is a conflict.
	both := "#include \"libvim.h\"\n#define VIMANGO_CHECK_AGAINST_LIBVIM\n#include \"" + api + "\"\n"
	if out, err := cc("both.c", both, "-fsyntax-only"); err != nil {
		t.Errorf("libvim_api.h declares something differently from the fork's libvim.h:\n%s", out)
	}
	// ...and that check can fail: a wrong prototype is refused
	if _, err := cc("control.c", both+"int vimKey(int key);\n", "-fsyntax-only"); err == nil {
		t.Error("the control is wrong: a prototype that contradicts libvim.h compiled")
	}

	// Types and constants: the same probe against each header.
	layout := func(name, include string) string {
		t.Helper()
		bin := filepath.Join(dir, name)
		if out, err := cc(name+".c", "#include \""+include+"\"\n"+layoutProbe, "-o", bin); err != nil {
			t.Fatalf("compiling the layout probe against %s:\n%s", include, out)
		}
		out, err := exec.Command(bin).Output()
		if err != nil {
			t.Fatalf("running the layout probe for %s: %v", include, err)
		}
		return string(out)
	}
	theirs, ours := layout("theirs", "libvim.h"), layout("ours", api)
	if theirs != ours {
		t.Errorf("libvim_api.h's types or constants differ from the fork's.\nthe fork's:\n%s\nlibvim_api.h's:\n%s", theirs, ours)
	}
	if strings.Count(ours, "\n") < 30 {
		t.Errorf("the layout probe printed only:\n%s", ours)
	}

	// The command table is the fork's file.
	want, err := os.ReadFile(filepath.Join(src, "ex_cmds.h"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile("ex_cmds.h"); !bytes.Equal(got, want) {
		t.Errorf("vim/ex_cmds.h is not the fork's src/ex_cmds.h; copy it in (scripts/build-libvim.sh does)")
	}
}
