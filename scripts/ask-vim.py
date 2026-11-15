#!/usr/bin/env python3
"""Ask stock vim what it does with keys that scroll or move the view.

    scripts/ask-vim.py ROWS KEYS [KEYS...]

types each KEYS into a vim of its own, one key at a time as a person does,
in a window the size of the test editor's pane -- 30 lines of 50 columns,
with 'wrap', 'linebreak' and 'scrolloff' 0, and no vimrc -- and prints the
first line in the window, the line the cursor is on and its column:

    $ scripts/ask-vim.py uneven '20Gzz' '<PageDown><PageUp>'
    {"uneven", "20Gzz", "15 20 1"},
    {"uneven", "<PageDown><PageUp>", "1 12 1"},

which is a line of vimAnswers in internal/editor/view_vim_test.go, the table
that holds the editor's view to vim's. Keys are written as vim writes them:
<C-d> <C-u> <C-e> <C-y> <C-f> <C-b> <PageDown> <PageUp> <CR> <Esc>.

ROWS is a set of rows by name, the same as vimRows makes in that test:

    numbered   100 rows of one screen line
    tall3      40 rows of three screen lines
    tall2      40 rows of two
    mixed      60 rows of one, but rows 22 and 24, of three
    uneven     60 rows of one, two, three and four screen lines, in turn

The answers in the table are vim 9.1.1752's. Needs python3 and a vim on the
path (or $VIM_BINARY).
"""
import fcntl
import os
import pty
import re
import select
import shutil
import struct
import sys
import tempfile
import termios
import time

LINES, COLS = 30, 50

KEYS = {
    "<C-d>": "\x04", "<C-u>": "\x15", "<C-e>": "\x05", "<C-y>": "\x19",
    "<C-f>": "\x06", "<C-b>": "\x02", "<CR>": "\r", "<Esc>": "\x1b",
    "<PageDown>": "\x1b[6~", "<PageUp>": "\x1b[5~",
}


def rows(name):
    numbered = ["row %d of the note" % (i + 1) for i in range(100)]
    if name == "numbered":
        return numbered
    if name == "tall3":
        return ["word " * 25] * 40
    if name == "tall2":
        return ["word " * 15] * 40
    if name == "mixed":
        mixed = numbered[:60]
        mixed[21] = mixed[23] = "word " * 25
        return mixed
    if name == "uneven":
        return ["word " * (10 * (i % 4 + 1) - 5) for i in range(60)]
    sys.exit(f"no rows called {name}: numbered, tall3, tall2, mixed or uneven")


def ask(vim, lines, keys):
    """Type keys into vim showing lines; returns "top line column"."""
    work = tempfile.mkdtemp(prefix="ask-vim-")
    note, answer = os.path.join(work, "note.txt"), os.path.join(work, "answer")
    with open(note, "w") as f:
        f.write("\n".join(lines) + "\n")
    args = [vim, "-u", "NONE", "-N", "-i", "NONE", "-n",
            "--cmd", "set t_RV= t_u7= t_RB= t_RF=",  # ask the terminal nothing: nobody answers
            "-c", "set scrolloff=0 laststatus=0 noruler noshowcmd wrap linebreak startofline"
                  " shortmess+=I notimeout nottimeout", note]
    pid, fd = pty.fork()
    if pid == 0:
        os.environ["TERM"] = "xterm"
        os.execvp(vim, args)
    # one more line than the window: vim's command line
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", LINES + 1, COLS, 0, 0))

    def drain(seconds):
        end = time.time() + seconds
        while time.time() < end:
            ready, _, _ = select.select([fd], [], [], 0.02)
            if ready:
                try:
                    os.read(fd, 65536)
                except OSError:
                    return

    try:
        drain(0.25)
        # A key at a time, so that vim brings its window up to date between
        # them, as it does for someone typing; given all at once (or to
        # :normal) it scrolls from a window it has not yet moved.
        for key in re.sub(r"<[^>]+>", lambda m: KEYS[m.group(0)], keys):
            os.write(fd, key.encode())
            drain(0.012)
        os.write(fd, b'\x1b:call writefile([line("w0")." ".line(".")." ".col(".")], "%s")\r:qa!\r'
                 % answer.encode())
        end = time.time() + 3
        while time.time() < end and not os.path.exists(answer):
            drain(0.02)
        drain(0.05)
        if not os.path.exists(answer):
            return "vim did not answer"
        with open(answer) as f:
            return f.read().strip()
    finally:
        try:
            os.kill(pid, 9)
            os.waitpid(pid, 0)
        except (ProcessLookupError, ChildProcessError):
            pass
        os.close(fd)
        shutil.rmtree(work, ignore_errors=True)


def main():
    if len(sys.argv) < 3 or sys.argv[1] in ("-h", "--help"):
        sys.exit(__doc__)
    vim = os.environ.get("VIM_BINARY", "vim")
    if not shutil.which(vim):
        sys.exit(f"no {vim} on the path")
    lines = rows(sys.argv[1])
    for keys in sys.argv[2:]:
        for name in re.findall(r"<[^>]+>", keys):
            if name not in KEYS:
                sys.exit(f"{name} is not a key this knows: {' '.join(KEYS)}")
        quoted = keys.replace("\\", "\\\\").replace('"', '\\"')
        print('{"%s", "%s", "%s"},' % (sys.argv[1], quoted, ask(vim, lines, keys)))


if __name__ == "__main__":
    main()
