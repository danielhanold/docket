#!/usr/bin/env bash
# tests/test_go_toolchain.sh — the whole-suite Go gate (change 0304).
#
# Runs the four canonical Go checks from the spec's build contract: gofmt
# cleanliness, go vet, go test, and the four-tuple CGO-off cross-build. This
# file is the REAL producer wiring those checks into scripts/run-tests.sh via
# the tests/test_*.sh discovery glob — not a documentation-only command.
#
# Guard shape: CHECKS_RUN counts each executed check and a final assert pins
# the count, so deleting a check from this file reddens the file itself
# rather than silently narrowing the gate. Deleting the FILE orphans its
# tests/runtime-budgets.tsv row, which reddens tests/test_runtime_budgets.sh.
#
# Requires a Go toolchain on PATH (go.mod pins the version); fails loudly if
# absent rather than skipping — a skipped gate certifies nothing.
#
# The assert helper is the tree's canonical one byte for byte: rule (a) of
# scripts/check-test-source-hygiene.sh is a byte-exact allowlist, and
# scripts/run-tests.sh accounts results on the `ok - ` / `NOT OK - ` markers
# it prints.
#
# CACHES. scripts/run-tests.sh gives every job a private HOME, so `go` finds no
# module cache and re-downloads this module's requirements into that throwaway
# home on every suite run. Two consequences are handled here rather than left
# to the runner:
#   - `-modcacherw` is REQUIRED, not a preference. Go writes module-cache files
#     read-only by default, and the runner's exit trap then cannot `rm -rf` its
#     own work directory: without this flag the suite leaks the whole job tree
#     and prints a page of `rm: Permission denied` on the way out.
#   - a CI image with a persistent cache exports GOMODCACHE/GOCACHE itself;
#     `go` reads them from the environment, so nothing here has to reach into
#     the invoking user's real home — a test that reads the developer's $HOME
#     is the shape the change-0227 parallel-safety audit forbids.
# The residual is a NETWORK dependency on the module proxy under the suite
# runner. Removing it means a persistent cache in the environment or vendoring,
# and this change deliberately does not vendor.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'NOT OK - the Go gate cannot certify anything without a Go toolchain\n'
  exit 1
fi

# Keep whatever GOFLAGS the caller set; append rather than replace.
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"

CHECKS_RUN=0
scratch="$(mktemp -d "${TMPDIR:-/tmp}/docket-go-gate.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT

# Check 1: gofmt reports no unformatted Go source.
CHECKS_RUN=$((CHECKS_RUN + 1))
unformatted="$(gofmt -l cmd internal 2>&1)"
assert "gofmt reports no unformatted files" '[ -z "$unformatted" ] || { printf "  unformatted: %s\n" "$unformatted" >&2; false; }'

# Check 2: go vet passes.
CHECKS_RUN=$((CHECKS_RUN + 1))
vet_out="$(go vet ./... 2>&1)"
vet_rc=$?
assert "go vet ./... passes" '[ "$vet_rc" -eq 0 ] || { printf "%s\n" "$vet_out" >&2; false; }'

# Check 3: go test passes on the host.
CHECKS_RUN=$((CHECKS_RUN + 1))
test_out="$(go test ./... 2>&1)"
test_rc=$?
assert "go test ./... passes" '[ "$test_rc" -eq 0 ] || { printf "%s\n" "$test_out" >&2; false; }'

# Check 4: CGO-off cross-build succeeds for each approved tuple.
CHECKS_RUN=$((CHECKS_RUN + 1))
build_failures=""
for tuple in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64; do
  goos="${tuple%/*}"
  goarch="${tuple#*/}"
  build_out="$(CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -o "$scratch/docket-$goos-$goarch" ./cmd/docket 2>&1)" \
    || build_failures="$build_failures $tuple:$build_out"
done
assert "CGO_ENABLED=0 go build succeeds for all four approved tuples" \
  '[ -z "$build_failures" ] || { printf "  failed:%s\n" "$build_failures" >&2; false; }'

# Self-count: the gate ran every check it claims to run. Deleting one from
# this file must redden the file, not silently narrow the whole-suite gate.
assert "all 4 Go checks executed" '[ "$CHECKS_RUN" -eq 4 ]'

exit $fail
