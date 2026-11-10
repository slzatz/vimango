# The vim package

vimango's binding to **libvim** — vim as a library — through cgo. It is the
only layer between the application and libvim's C API: the application calls
`vim.SendKey(…)`, and that function calls `vimKey`.

(Until 2026-11-10 there were three layers: this package forwarded to an
`Engine` interface, a wrapper type implemented it, and the wrapper called a
`cvim` package that held the cgo. The interface existed to switch between
libvim and a pure-Go engine, `govim`, which was removed in 2026-07.)

## Layout

- `vim.go` — buffers, input, cursor, mode, selection, command line
- `spell.go`, `spell_callback.go` — spell checking (the callback has to live
  in its own file: a file with `//export` may only have declarations in its
  cgo preamble)
- `*_test.go` — tests that drive libvim directly
- `src/`, `proto/`, `auto/` — libvim's headers, copied from the
  [slzatz/libvim](https://github.com/slzatz/libvim) fork. Headers only; the
  code is in `libvim.a`, which `scripts/build-libvim.sh` builds and puts in
  the project root. The linker flags name it by bare filename, so run `go`
  from the project root.

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
mode := vim.GetCurrentMode()       // 1 = NORMAL
```

libvim is one process-wide vim and is not thread-safe: call it from one
goroutine (vimango's main loop).

## Adding a binding

1. Confirm the symbol is in the library: `nm -gU libvim.a | grep vimWhatever`.
   Its declaration is in `src/libvim.h`; if the fork added it, copy the
   updated header in.
2. Add the function to `vim.go`, with the C signature in its comment like the
   others.
3. Test it against the real library — `cmdline_test.go` shows the pattern.
   The headers say what a function takes, not what it does.
