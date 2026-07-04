#!/usr/bin/env bash
#
# build-libvim.sh — Build libvim.a for vimango's CGO build.
#
# libvim.a is a static library built from onivim/libvim. It is platform- and
# architecture-specific and is intentionally NOT checked into the repo
# (see .gitignore). Each machine must build its own copy.
#
# This script:
#   1. Detects the correct Homebrew prefix (Apple Silicon /opt/homebrew vs
#      Intel /usr/local) so it works on both.
#   2. Installs the ncurses/gettext build dependencies via Homebrew.
#   3. Clones onivim/libvim next to the vimango repo if it isn't already there.
#   4. Configures and builds libvim.a.
#   5. Copies libvim.a into the vimango project root (next to go.mod).
#
# Usage:
#   ./scripts/build-libvim.sh
#
# Override the libvim source location with LIBVIM_DIR if you keep it elsewhere:
#   LIBVIM_DIR=/path/to/libvim ./scripts/build-libvim.sh
#
set -euo pipefail

# --- Locate the vimango project root (the dir containing this script's parent).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VIMANGO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# --- Only supports macOS here; Linux uses the simpler recipe in INSTALL.md.
if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "This script targets macOS. On Linux, see INSTALL.md (libvim build is:"
  echo "  ./configure --disable-selinux CFLAGS=-fPIC && make libvim.a)."
  exit 1
fi

# --- Require Homebrew and detect its prefix (Apple Silicon vs Intel).
if ! command -v brew >/dev/null 2>&1; then
  echo "Error: Homebrew not found. Install it from https://brew.sh first."
  exit 1
fi
BREW_PREFIX="$(brew --prefix)"
echo "Using Homebrew prefix: ${BREW_PREFIX}"

# --- Install build dependencies.
echo "Installing build dependencies (ncurses, gettext)..."
brew install ncurses gettext

# --- Locate or clone the libvim source.
LIBVIM_DIR="${LIBVIM_DIR:-$(cd "${VIMANGO_ROOT}/.." && pwd)/libvim}"
if [[ ! -d "${LIBVIM_DIR}" ]]; then
  echo "Cloning onivim/libvim into ${LIBVIM_DIR}..."
  git clone https://github.com/onivim/libvim.git "${LIBVIM_DIR}"
else
  echo "Using existing libvim source at ${LIBVIM_DIR}"
fi

# --- Configure and build.
NCURSES_PREFIX="${BREW_PREFIX}/opt/ncurses"
cd "${LIBVIM_DIR}/src"

echo "Configuring libvim..."
./configure --disable-selinux --with-tlib=ncurses \
  CFLAGS="-I${NCURSES_PREFIX}/include -I${BREW_PREFIX}/include \
    -Wno-error=implicit-function-declaration -Wno-error=implicit-int \
    -Wno-error=int-conversion -Wno-error=incompatible-function-pointer-types \
    -Wno-error=unused-but-set-variable -Wno-error=deprecated-non-prototype \
    -Wno-error=implicit-int-float-conversion -Wno-error" \
  LDFLAGS="-L${NCURSES_PREFIX}/lib -L${BREW_PREFIX}/lib"

echo "Building libvim.a..."
make libvim.a

# --- Copy into the vimango project root.
cp libvim.a "${VIMANGO_ROOT}/libvim.a"
echo
echo "Done. libvim.a copied to ${VIMANGO_ROOT}/libvim.a"
echo "You can now build vimango with:"
echo "  cd ${VIMANGO_ROOT} && CGO_ENABLED=1 go build --tags=\"fts5,cgo\""
