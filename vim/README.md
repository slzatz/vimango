# The vim package

vimango's binding to **libvim** — vim as a library — through cgo. It is the
only layer between the application and libvim's C API: the application calls
`vim.SendKey(…)`, and that function calls `vimKey`.

(Until 2026-11-10 there were three layers: this package forwarded to an
`Engine` interface, a wrapper type implemented it, and the wrapper called a
`cvim` package that held the cgo. The interface existed to switch between
libvim and a pure-Go engine, `govim`, which was removed in 2026-07.)

## Layout

- `vim.go` — buffers, input, cursor, mode, selection, command line; and
  `InitializeVim`, which puts libvim in restricted mode with file writes off
  (no program is started and no file written, by any route)
- `search.go` — where the matches of the last search are, and whether they
  should be showing
- `messages.go`, `message_callback.go` — what vim would write on its message
  line
- `excommands.go` — vim's command table, for resolving an ex command's name
  as vim does (`FindExCommand`)
- `spell.go`, `spell_callback.go` — spell checking (the callback has to live
  in its own file: a file with `//export` may only have declarations in its
  cgo preamble)
- `*_test.go` — tests that drive libvim directly
- `libvim_api.h` — the libvim declarations the binding uses, written out:
  the one header the package compiles against. A copy, so `header_test.go`
  checks it against the [slzatz/libvim](https://github.com/slzatz/libvim)
  fork's own headers.
- `ex_cmds.h` — vim's table of ex commands, the fork's file, embedded by
  `excommands.go`.

The code is in `libvim.a`, which `scripts/build-libvim.sh` builds from the
fork and puts in the project root. The linker flags name it by bare filename,
so run `go` from the project root.

## Usage

```go
import "github.com/slzatz/vimango/vim"

vim.InitializeVim(0)

buf := vim.NewBuffer(0)
vim.SetCurrentBuffer(buf)
buf.SetLines(0, -1, []string{"Hello, world"})

vim.SendMultiInput("A!")   // keys, one character at a time
vim.SendKey("<esc>")       // a key in <> notation

lines := buf.Lines()               // ["Hello, world!"]
pos := vim.GetCursorPosition()     // {line (1-based), byte column (0-based)}
mode := vim.GetCurrentMode()       // vim.ModeNormal
```

libvim is one process-wide vim and is not thread-safe: call it from one
goroutine (vimango's main loop).

## Adding a binding

1. Confirm the symbol is in the library: `nm -gU libvim.a | grep vimWhatever`.
2. Copy its declaration from the fork's `src/libvim.h` into `libvim_api.h`.
   If it brings a type or a constant with it, copy that too, and add it to
   the probe in `header_test.go`.
3. Add the function to `vim.go`, with the C signature in its comment like the
   others.
4. Run `go test ./vim`. `TestHeaderMatchesLibvim` checks `libvim_api.h`
   against the fork's headers (it needs the fork's checkout at `../libvim` or
   `$LIBVIM_DIR`); test the binding itself against the real library the way
   `cmdline_test.go` does. A declaration says what a function takes, not what
   it does.
