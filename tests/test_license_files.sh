#!/usr/bin/env bash
# docket-suite: go
# tests/test_license_files.sh — the license-artifact guard (change 0404,
# superseding 0401's PolyForm guard).
#
# Drives internal/repoguard's TestLicenseFiles + TestLicenseReadmeSection:
# LICENSE (verbatim Apache License 2.0 — identifier, date line, and the
# appendix's distinctive clause — with the PolyForm-era strings asserted
# absent), NOTICE (the copyright line), CONTRIBUTING.md (the Signed-off-by
# DCO trailer), the asserted ABSENCE of LICENSE-ADDITIONAL-PERMISSIONS.md at
# the repo root, and the README License section (heading + links to LICENSE,
# NOTICE, and CONTRIBUTING.md). `go test -run` with
# a pattern that matches nothing exits 0, so this wrapper pins BOTH
# `--- PASS:` lines: deleting or renaming either Go test reddens THIS file
# even though `go test` would happily pass without it.
# CACHES. Same location and reasoning as tests/test_go_toolchain.sh (see the
# CACHES note in that file's header): <git common dir>/docket-go-cache/{mod,build}.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'NOT OK - the license guard cannot certify anything without a Go toolchain\n'
  exit 1
fi
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"
if [ -z "${GOMODCACHE:-}" ] || [ -z "${GOCACHE:-}" ]; then
  common_git_dir="$(git rev-parse --git-common-dir 2>/dev/null)"
  if [ -n "$common_git_dir" ]; then
    case "$common_git_dir" in /*) ;; *) common_git_dir="$REPO/$common_git_dir" ;; esac
    cache_root="$common_git_dir/docket-go-cache"
    if mkdir -p "$cache_root/mod" "$cache_root/build" 2>/dev/null; then
      export GOMODCACHE="${GOMODCACHE:-$cache_root/mod}"
      export GOCACHE="${GOCACHE:-$cache_root/build}"
    fi
  fi
fi

test_out="$(go test ./internal/repoguard/ -run '^TestLicense' -v 2>&1)"
test_rc=$?
assert "go test runs the license guards without failure" '[ "$test_rc" -eq 0 ] || { printf "%s\n" "$test_out" >&2; false; }'
# `go test -v` always prints `--- PASS: <name> (<dur>)`, so the trailing " ("
# is pinned: without it the fixed-string match is a substring test that a
# prefix-superstring rename (TestLicenseFiles -> TestLicenseFilesX) would still
# satisfy, leaving the "renaming reddens" contract above a vacuous claim.
assert "TestLicenseFiles exists and passed (a non-matching -run exits 0, so the PASS line is the proof)" \
  'grep -qF -- "--- PASS: TestLicenseFiles (" <<<"$test_out"'
assert "TestLicenseReadmeSection exists and passed" \
  'grep -qF -- "--- PASS: TestLicenseReadmeSection (" <<<"$test_out"'

if [ "$fail" -eq 0 ]; then printf 'ALL PASS\n'; exit 0; else exit 1; fi
