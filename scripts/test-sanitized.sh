#!/usr/bin/env bash
#
# test-sanitized.sh — run vimango's tests, and libvim's own, against a libvim
# built with AddressSanitizer (or UndefinedBehaviorSanitizer).
#
# The ordinary libvim.a is not instrumented, so a read past the end of a line
# inside vim shows only if it happens to crash. This builds an instrumented
# library and runs against it:
#   1. vimango's Go suite (go test ./...),
#   2. the fork's API tests (src/apitest/*.c),
#   3. the fork's case files (src/apitest/compare/cases: about 2,600 key
#      sequences, ex commands and vimscript functions, each in a process of
#      its own),
#   4. api_edges.exe, which calls the C API with arguments it does not expect.
#      Its reports are printed and not counted: they are misuse, and listed in
#      the fork's TODO.md.
#
# Nothing here is touched: the fork's checkout and this tree are copied to a
# work directory (kept between runs, so the second run compiles only what
# changed), and the copies are what is built. Uncommitted changes on either
# side are included. The fork must have been built once (auto/config.mk), which
# scripts/build-libvim.sh does.
#
# Usage:
#   ./scripts/test-sanitized.sh            # AddressSanitizer, all four
#   ./scripts/test-sanitized.sh --ubsan    # UndefinedBehaviorSanitizer instead
#   ./scripts/test-sanitized.sh --go-only  # vimango's suite alone
#   ./scripts/test-sanitized.sh --clean    # throw the work directory away first
#
# Exit status 0 when every test passes and no sanitizer report was written for
# 1 to 3. The reports are in the work directory, whose path is printed.
#
set -euo pipefail

SAN=address
GO_ONLY=0
CLEAN=0
for arg in "$@"; do
  case "$arg" in
    --ubsan) SAN=undefined ;;
    --go-only) GO_ONLY=1 ;;
    --clean) CLEAN=1 ;;
    -h | --help) sed -n '2,32p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "error: unknown option: $arg (see --help)" >&2; exit 1 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VIMANGO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
LIBVIM_DIR="${LIBVIM_DIR:-$(cd "${VIMANGO_ROOT}/.." && pwd)/libvim}"

if [[ ! -f "${LIBVIM_DIR}/src/auto/config.mk" ]]; then
  echo "error: no configured libvim checkout at ${LIBVIM_DIR}; run scripts/build-libvim.sh first." >&2
  exit 1
fi

TMP="${TMPDIR:-/tmp}"
WORK="${TMP%/}/vimango-sanitized/${SAN}"
[[ "${CLEAN}" -eq 1 ]] && rm -rf "${WORK}"
REPORTS="${WORK}/reports"
rm -rf "${REPORTS}" "${WORK}/vimango"
mkdir -p "${WORK}/libvim" "${WORK}/vimango" "${REPORTS}" "${WORK}/stage"

# -Wno-deprecated-declarations: AddressSanitizer turns _FORTIFY_SOURCE off, and
# the macOS SDK then calls sprintf() deprecated, under the fork's -Werror.
SANITIZER_CFLAGS="-g -O1 -fsanitize=${SAN} -fno-omit-frame-pointer -Wno-deprecated-declarations"

# One report file per process, named for the step that was running.
# detect_leaks: vim frees little on exit, and every process would report it
# (Linux; macOS has no leak detection). A report ends the process under
# AddressSanitizer, and not under UndefinedBehaviorSanitizer.
reports_to() {
  export ASAN_OPTIONS="abort_on_error=1:detect_leaks=0:log_path=${REPORTS}/$1"
  export UBSAN_OPTIONS="print_stacktrace=1:log_path=${REPORTS}/$1"
}

case "$(uname -s)" in
  Darwin) JOBS="$(sysctl -n hw.ncpu)"; EXTRA_LIBS="-L$(brew --prefix gettext)/lib" ;;
  Linux) JOBS="$(nproc)"; EXTRA_LIBS="" ;;
  *) echo "error: unsupported platform $(uname -s)" >&2; exit 1 ;;
esac

failed=0

# --- The library. The copy keeps its own objects, so the checkout's stay as
# they are and a second run compiles only the files that changed.
echo "==> Building libvim with -fsanitize=${SAN} ($(git -C "${LIBVIM_DIR}" log -1 --format='%h %s'))"
rsync -a --delete --exclude .git --exclude '/src/objects/*.o' --exclude '/src/libvim.a' \
  --exclude '*.exe' --exclude '*.dSYM' "${LIBVIM_DIR}/" "${WORK}/libvim/"
if ! make -C "${WORK}/libvim/src" libvim.a "-j${JOBS}" SANITIZER_CFLAGS="${SANITIZER_CFLAGS}" \
     > "${WORK}/build.log" 2>&1; then
  tail -30 "${WORK}/build.log" >&2
  echo "error: the instrumented library did not build (${WORK}/build.log)" >&2
  exit 1
fi

# --- 1. vimango's suite, in a copy of this tree with that library in it: the
# linker takes libvim.a from the directory go is run in (vim/vim.go). What git
# tracks or would track is copied, and the spell dictionary; not config.json,
# the database or the word list.
echo "==> vimango's suite"
(cd "${VIMANGO_ROOT}" && git ls-files -co --exclude-standard -z |
  while IFS= read -r -d '' f; do
    # a tracked file that was deleted and not committed is still listed
    if [[ -e "$f" ]]; then printf '%s\0' "$f"; fi
  done | rsync -a --from0 --files-from=- . "${WORK}/vimango/")
cp "${WORK}/libvim/src/libvim.a" "${WORK}/vimango/libvim.a"
if [[ -f "${VIMANGO_ROOT}/spell/en.utf-8.spl" ]]; then
  mkdir -p "${WORK}/vimango/spell"
  cp "${VIMANGO_ROOT}"/spell/en.utf-8.spl "${VIMANGO_ROOT}"/spell/en.utf-8.sug "${WORK}/vimango/spell/"
fi
reports_to go
# go's own -asan is not available on darwin/arm64; linking the runtime in is
# enough, since libvim is the code to check and it is already instrumented.
if ! (cd "${WORK}/vimango" && CGO_ENABLED=1 CGO_LDFLAGS="${CGO_LDFLAGS:-} -fsanitize=${SAN}" \
      go test --tags=fts5 -count=1 ./... > "${WORK}/go.log" 2>&1); then
  failed=1
fi
grep -E '^(ok|FAIL|---|panic)' "${WORK}/go.log" || true

if [[ "${GO_ONLY}" -eq 0 ]]; then
  cd "${WORK}/libvim/src"

  # --- 2. The fork's API tests, as its build scripts run them.
  echo "==> libvim's API tests"
  n=0
  bad=0
  for t in apitest/*.c; do
    name="$(basename "$t" .c)"
    n=$((n + 1))
    if ! make "apitest/${name}.test.exe" SANITIZER_CFLAGS="${SANITIZER_CFLAGS}" \
         EXTRA_LIBS="${EXTRA_LIBS}" DEST_BIN="${WORK}/stage" > "${WORK}/stage/${name}.build.log" 2>&1; then
      echo "  DID NOT BUILD  ${name} (${WORK}/stage/${name}.build.log)"
      bad=$((bad + 1))
      continue
    fi
    reports_to "api.${name}"
    if ! (cd apitest && "./${name}.test.exe" > "${WORK}/stage/${name}.run.log" 2>&1); then
      echo "  FAIL  ${name} (${WORK}/stage/${name}.run.log)"
      bad=$((bad + 1))
    fi
  done
  echo "  ${n} programs, ${bad} failed"
  [[ "${bad}" -eq 0 ]] || failed=1

  # --- 3 and 4. The case files and the API's edges (apitest/compare/README.md).
  echo "==> libvim's case files"
  if ! make -f Makefile -f apitest/compare/rules.mk compare-tools \
       SANITIZER_CFLAGS="${SANITIZER_CFLAGS}" EXTRA_LIBS="${EXTRA_LIBS}" > "${WORK}/compare-build.log" 2>&1; then
    tail -20 "${WORK}/compare-build.log" >&2
    echo "error: probe.exe did not build (${WORK}/compare-build.log)" >&2
    exit 1
  fi
  cd apitest/compare
  for f in cases/*.txt; do
    reports_to "cases.$(basename "$f" .txt)"
    # Its status is not used: two mappings in ex.txt are meant never to end,
    # and "did not come back" is what they should do.
    printf '  %-26s %s\n' "$(basename "$f" .txt)" "$(./sweep.sh "$f" 2>&1 | tail -1)"
  done

  reports_to edges
  ./api_edges.exe > "${WORK}/api_edges.log" 2>&1 || true
fi

# --- What the sanitizer said: each distinct report, with how often.
summaries() {
  cat "$@" 2>/dev/null | grep -aE '^SUMMARY: AddressSanitizer: |runtime error: ' |
    sed -E 's/^SUMMARY: [A-Za-z]+: //; s#^.*/src/##' | sort | uniq -c | sort -rn
}
echo
counted="$(find "${REPORTS}" -type f ! -name 'edges.*' | wc -l | tr -d ' ')"
if [[ "${counted}" -gt 0 ]]; then
  echo "Sanitizer reports (${counted} of the files in ${REPORTS}):"
  find "${REPORTS}" -type f ! -name 'edges.*' -print0 | xargs -0 cat | summaries -
  failed=1
else
  echo "No sanitizer report from vimango's suite, the API tests or the case files."
fi
if compgen -G "${REPORTS}/edges.*" > /dev/null; then
  echo "From api_edges.exe (misuse of the API; not counted, see the fork's TODO.md):"
  summaries "${REPORTS}"/edges.*
fi
echo "Work directory: ${WORK}"
if [[ "${failed}" -ne 0 ]]; then
  echo "FAILED: see ${REPORTS} and ${WORK}/go.log" >&2
  exit 1
fi
