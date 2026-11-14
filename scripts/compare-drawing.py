#!/usr/bin/env python3
"""Compare what two vimango binaries write to the terminal for the same keys.

For a change that is meant to move code and not to change what is drawn (the
editor's layout, a package move): build the binary before and after, and run

    scripts/compare-drawing.py /path/to/vimango.before /path/to/vimango.after

from the vimango directory. Each binary is run on a pseudo-terminal, in
--editor mode and as the TUI, through the same keys, and everything it writes
is compared step by step. "SAME" means byte for byte, but for the age vim
gives an undo ("3 seconds ago").

Nothing here touches the real database or the server: the runs happen in a
temporary directory holding a backup copy of vimango.db (taken with sqlite3's
.backup, restored before every run so both binaries see the same rows), a
config.json with no PostgreSQL host, and links to vimango.vim, the glamour
styles and spell/. Google Drive is left out, so images are not fetched.

Needs python3 and sqlite3. Exit status 0 if the two are the same, 1 if not.
"""
import fcntl
import json
import os
import pty
import re
import select
import shutil
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time

ROWS, COLS = 40, 140

# Keys for the editor: an edit and :w, undo, a refused :q, :help and paging,
# a search, a selection, :sync (refused), a listing, a resize, line numbers,
# spelling suggestions, and out without saving.
EDITOR_KEYS = [
    b"jjw", b"iZZ\x1b", b":w\r", b"u", b":q\r", b":help\r", b" ", b"\x1b",
    b"/the\r", b"n", b":noh\r", b"vjj", b"\x1b", b":sync\r", b":marks\r", b"\x1b",
    (30, 100), b"G", b":number\r", b":number\r", b":spell\r", b"z=", b"\x1b", b":q!\r",
]
# The TUI: the same inside :e, then back in the organizer.
TUI_KEYS = [b":e\r"] + EDITOR_KEYS[:-1] + [(ROWS, COLS), b":x\r", b"j", b":e\r", b":q\r", b":q\r"]


def run(binary, args, steps, snapshot):
    """Run binary in the current directory and return what it wrote, per step."""
    for name in ("vimango.db", "vimango.db-wal", "vimango.db-shm", "registers.json"):
        if os.path.exists(name):
            os.remove(name)
    shutil.copy(snapshot, "vimango.db")

    pid, fd = pty.fork()
    if pid == 0:
        os.environ["TERM"] = "xterm-256color"  # no images: detection reads TERM
        os.execv(binary, [binary] + args)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", ROWS, COLS, 0, 0))

    def read(seconds):
        buf, end = b"", time.time() + seconds
        while time.time() < end:
            ready, _, _ = select.select([fd], [], [], 0.05)
            if ready:
                try:
                    data = os.read(fd, 65536)
                except OSError:
                    break
                if not data:
                    break
                buf += data
        return buf

    written = [read(2.0)]
    for step in steps:
        if isinstance(step, tuple):  # a resize
            fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", step[0], step[1], 0, 0))
            os.kill(pid, signal.SIGWINCH)
        else:
            try:
                os.write(fd, step)
            except OSError:
                pass  # it has exited
        written.append(read(0.7))
    try:
        os.kill(pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    os.waitpid(pid, 0)
    os.close(fd)
    return written


def canon(data):
    text = data.decode("utf8", "replace")
    return re.sub(r"\d+ (seconds?|minutes?) ago", "N ago", text)


def main():
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    before, after = (os.path.abspath(p) for p in sys.argv[1:3])
    src = os.getcwd()
    for needed in ("config.json", "vimango.db"):
        if not os.path.exists(needed):
            sys.exit(f"run this from the vimango directory: no {needed} here")

    work = tempfile.mkdtemp(prefix="vimango-compare-")
    snapshot = os.path.join(work, "snapshot.db")
    subprocess.run(["sqlite3", "vimango.db", f".backup {snapshot}"], check=True)
    config = json.load(open("config.json"))
    config["sqlite3"]["db"] = "vimango.db"
    config["postgres"]["host"] = ""  # no server from here
    json.dump(config, open(os.path.join(work, "config.json"), "w"))
    for name in ("vimango.vim", "spell", "default.json", config.get("glamour", {}).get("style", "")):
        if name and os.path.exists(name) and not os.path.lexists(os.path.join(work, name)):
            os.symlink(os.path.join(src, name), os.path.join(work, name))
    os.chdir(work)

    same = True
    try:
        for mode, args, steps in (("--editor", ["--editor"], EDITOR_KEYS), ("TUI", [], TUI_KEYS)):
            old = run(before, args, steps, snapshot)
            new = run(after, args, steps, snapshot)
            differing = 0
            for i, (a, b) in enumerate(zip(old, new)):
                ca, cb = canon(a), canon(b)
                if ca == cb:
                    continue
                differing += 1
                same = False
                at = next((k for k in range(min(len(ca), len(cb))) if ca[k] != cb[k]), min(len(ca), len(cb)))
                what = "start" if i == 0 else repr(steps[i - 1])
                print(f"  {mode}, step {i} ({what}): differs at byte {at}")
                print(f"    before {ca[max(0, at - 50):at + 70]!r}")
                print(f"    after  {cb[max(0, at - 50):at + 70]!r}")
            print(f"{mode}: {len(steps)} steps, {sum(map(len, old))} bytes before and "
                  f"{sum(map(len, new))} after, {differing} steps differ")
    finally:
        os.chdir(src)
        shutil.rmtree(work, ignore_errors=True)
    print("SAME" if same else "DIFFERENT")
    sys.exit(0 if same else 1)


if __name__ == "__main__":
    main()
