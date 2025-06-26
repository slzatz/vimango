# Vim Implementation Refactoring for Cross-Platform Compatibility

## Overview

This document details the refactoring efforts undertaken to enable the `vimango` application to compile and run on Windows, specifically addressing the dependency on CGO (C-Go interoperability) for the `libvim` (C library) implementation of Vim functionality. The primary goal was to achieve Windows compatibility while preserving the existing flexibility on Linux to choose between the CGO-based `libvim` and a pure Go implementation of Vim.

## The Problem

The `vimango` application previously relied on a CGO-based wrapper (`vim/cvim`) for its Vim functionality. While this worked well on Linux, CGO introduces significant challenges for cross-compilation, particularly to Windows, due to the need for a C toolchain and specific library linking.

The core issue was that the `vim` package, specifically `vim/api.go`, had direct import dependencies on `github.com/slzatz/vimango/vim/cvim` and directly referenced `cvim.Buffer` types. This meant that simply excluding the `vim/cvim` package during a Windows build would lead to compilation errors throughout the `vim` package.

## Solution Implemented

The solution involved a combination of Go build tags and strategic file splitting to conditionally compile code based on the target operating system, effectively isolating the CGO dependency.

### 1. Isolating CGO Code with Build Tags

*   **`vim/cvim/cvim.go` Modification:** The `cvim.go` file, which contains the CGO calls to `libvim`, was tagged with `//go:build !windows`. This build constraint instructs the Go compiler to *only* include this file when building for operating systems that are *not* Windows (e.g., Linux, macOS). This prevents any CGO-related compilation errors when targeting Windows.

### 2. Providing a Windows Stub for `cvim`

*   **`vim/cvim/cvim_windows.go` Creation:** Since `vim/api.go` (and potentially other files) directly imported the `cvim` package and referenced types like `cvim.Buffer`, simply excluding `cvim.go` would cause "package not found" or "undefined type" errors on Windows. To resolve this, a new file, `vim/cvim/cvim_windows.go`, was created with the build tag `//go:build windows`. This file provides a minimal, dummy definition for `cvim.Buffer` (as `uintptr`) and other necessary types/functions. This satisfies the Go compiler's import requirements on Windows, allowing the `vim` package to compile, even though the CGO functionality itself is not present or usable.

### 3. Splitting the Adapter Logic

The original `vim/adapter.go` file contained logic for both CGO and pure Go implementations. To manage platform-specific initialization and ensure proper linking, this file was split:

*   **`vim/adapter_common.go`:** This file now contains all the platform-independent interfaces (`VimImplementation`, `VimEngine`, `VimBuffer`) and the pure Go implementation (`GoImplementation`, `GoEngineWrapper`, `GoBufferWrapper`). This code is included in all builds.
*   **`vim/adapter_cgo_linux.go`:** This file contains the `CGOImplementation`, `CGOEngineWrapper`, and `CGOBufferWrapper` definitions. It is tagged with `//go:build !windows`, ensuring it's only included in non-Windows builds. It also includes an `init()` function to set the default `activeImpl` to `CGOImplementation` on Linux.
*   **`vim/adapter_cgo_windows.go`:** This file provides dummy implementations for `CGOImplementation`, `CGOEngineWrapper`, and `CGOBufferWrapper` for Windows builds. It is tagged with `//go:build windows`. Its `init()` function ensures that `activeImpl` is *always* set to `GoImplementation` on Windows, effectively forcing the pure Go version.

### 4. Refactoring Initialization and Switching Logic

The central `InitializeVim` and `SwitchToCImplementation` functions, previously in `vim/api.go`, were moved to platform-specific files to control which implementation is activated at compile time:

*   **`vim/init_linux.go`:** Tagged with `//go:build !windows`, this file contains the `InitializeVim` and `SwitchToCImplementation` functions. On Linux, `InitializeVim` checks the `--go-vim` command-line flag to dynamically choose between the CGO and pure Go implementations. `SwitchToCImplementation` correctly sets the active implementation to CGO.
*   **`vim/init_windows.go`:** Tagged with `//go:build windows`, this file contains its own `InitializeVim` and `SwitchToCImplementation` functions. On Windows, `InitializeVim` *always* forces the pure Go implementation, regardless of the `--go-vim` flag. The `SwitchToCImplementation` function in this file is a `panic`ing stub, ensuring that any attempt to switch to the CGO implementation on Windows (which is impossible) results in a clear error.
*   **`vim/api.go` Updates:** The original `InitializeVim` and `SwitchToCImplementation` functions were removed from `vim/api.go`. The `ToggleImplementation` function was updated to call a new `SwitchImplementation` function (which is now handled by the platform-specific `init_*.go` files). The `Engine` and `activeImpl` variables were also correctly declared in `api.go` to be accessible across the package.

## Outcome

This refactoring successfully achieves the following:

*   **Windows Compatibility:** The `vimango` application can now be compiled for Windows without requiring a C toolchain, as all CGO-dependent code is excluded or stubbed out.
*   **Linux Flexibility Preserved:** On Linux, the application retains the ability to default to the CGO-based `libvim` implementation and can still be switched to the pure Go implementation using the `--go-vim` command-line argument.
*   **Clean Separation:** The use of build tags provides a clean and idiomatic way to separate platform-specific code, making the codebase more maintainable.

## Future Considerations

While this refactoring addresses the immediate cross-platform compilation issue, a long-term improvement would involve further refactoring of the application to completely remove direct dependencies on `cvim.Buffer` types in `vim/api.go` and other parts of the codebase. This would allow for a cleaner separation and potentially remove the need for the `cvim_windows.go` stub file.
