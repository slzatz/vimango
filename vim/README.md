# Vimango Vim Package

This package provides vim editing functionality for the Vimango application,
backed by **libvim via CGO** — the only implementation. (A pure-Go engine,
`govim`, and an adapter layer for switching between the two existed here
until 2026-07; they were removed once CGO-everywhere was settled and Windows
support was dropped.)

## Directory Structure

- `vim/` — API layer used by the application
  - `api.go` — package-level functions the app calls (`vim.SendKey`, …)
  - `api_cgo_compat.go` — older-style helpers operating on raw `cvim.Buffer`
  - `engine.go` — `InitializeVim` plus thin wrappers (`CGOEngineWrapper`,
    `CGOBufferWrapper`) that satisfy the interfaces in `interfaces/`
  - `interfaces/` — `VimEngine` / `VimBuffer` interfaces (application code
    holds buffers as `interfaces.VimBuffer`)
  - `cvim/` — the cgo bindings to libvim (`cvim.go`) and vendored headers
    (`src/`); the prebuilt `libvim.a` lives in the repo root

## Usage

```go
import "github.com/slzatz/vimango/vim"

vim.InitializeVim(0)

buffer := vim.NewBuffer(0)
vim.SetCurrentBuffer(buffer)

vim.SendInput("Hello, world!")
vim.SendKey("<esc>")

position := vim.GetCursorPosition()
mode := vim.GetCurrentMode()
```

## Adding a libvim binding

1. Confirm the symbol exists in the prebuilt library:
   `nm -gU libvim.a | grep vimWhatever` (declarations are in
   `cvim/src/libvim.h`).
2. Add the wrapper in `cvim/cvim.go` (C signature as a comment, Go func
   below — follow the existing style).
3. Expose it from `api.go`, either through `Engine` or as a direct `cvim`
   call (see the CommandLine* functions for the direct pattern).
4. Verify runtime behavior empirically — the vendored `src/` is
   headers-only; `cvim/cmdline_test.go` shows the test pattern.
