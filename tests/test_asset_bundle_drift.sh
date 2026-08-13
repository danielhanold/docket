#!/usr/bin/env bash
# tests/test_asset_bundle_drift.sh — the embedded-asset drift gate (change 0311).
#
# internal/assets/embedded/ (manifest.json + tree/) is GENERATED from the repo's
# authored asset roots — skills/, agents/, cursor-rules/, .docket.example.yml —
# and committed, because the release binary embeds it. A committed generated
# tree drifts the moment somebody edits an authored file and does not
# regenerate, and the binary then ships assets that are not the ones in the
# repo. cmd/genassets -check is the comparator; THIS FILE is what makes the
# suite run it.
#
# WHY A SHELL FILE AND NOT JUST A GO TEST. internal/assets already has Go
# coverage of the generator, but a Go test's verdict is cacheable: `go test`
# replays a cached PASS for a package whose own inputs did not change, and an
# edit to skills/ does not change internal/assets' package inputs. The drift
# this gate exists to catch is therefore exactly the drift a cached Go test
# cannot see. The shell suite runs unconditionally, so the gate lands here.
#
# MUTATION EVIDENCE IS BUILT IN, not left to a reviewer's promise. Assert 2
# copies the authored roots plus the committed bundle into $TMPDIR, appends one
# byte to a bundled skill file, and requires -check to exit non-zero there.
# Assert 3 is its control: the SAME copy, unmutated, must still pass, so a
# broken or incomplete copy cannot masquerade as a caught drift. The copy is
# scoped to the paths the generator reads plus the tree it compares against —
# -repo points genassets at it, so no module, no vendored deps and no second Go
# build are involved.
#
# The assert helper is the tree's canonical one byte for byte: rule (a) of
# scripts/check-test-source-hygiene.sh is a byte-exact allowlist, and
# scripts/run-tests.sh accounts results on the `ok - ` / `NOT OK - ` markers
# it prints.
#
# Requires a Go toolchain on PATH (go.mod pins the version); fails loudly if
# absent rather than skipping — a skipped gate certifies nothing.
#
# CACHES. scripts/run-tests.sh gives every job a private HOME, so with
# GOMODCACHE/GOCACHE unset `go` finds neither a module cache nor a build cache
# and recompiles cold — network-dependent — on every suite run. This file pins
# both to `<git common dir>/docket-go-cache/{mod,build}` when the caller has not
# chosen its own, which is the same location and the same reasoning as
# tests/test_go_toolchain.sh (see the CACHES note in that file's header: outside
# every working tree, shared across worktrees, concurrent-safe, and `-modcacherw`
# required so an ordinary `rm -rf` can still remove it). Only the first run after
# a fresh clone needs the network.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'NOT OK - the drift gate cannot certify anything without a Go toolchain\n'
  exit 1
fi

# Keep whatever GOFLAGS the caller set; append rather than replace.
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"

# Pin the caches out of the job's throwaway HOME — see CACHES in this header.
if [ -z "${GOMODCACHE:-}" ] || [ -z "${GOCACHE:-}" ]; then
  common_git_dir="$(git rev-parse --git-common-dir 2>/dev/null)"
  if [ -n "$common_git_dir" ]; then
    # `--git-common-dir` answers relative to the working tree in a plain clone
    # and absolute from a linked worktree; normalize before building on it.
    case "$common_git_dir" in /*) ;; *) common_git_dir="$REPO/$common_git_dir" ;; esac
    cache_root="$common_git_dir/docket-go-cache"
    if mkdir -p "$cache_root/mod" "$cache_root/build" 2>/dev/null; then
      export GOMODCACHE="${GOMODCACHE:-$cache_root/mod}"
      export GOCACHE="${GOCACHE:-$cache_root/build}"
    fi
  fi
fi

scratch="$(mktemp -d "${TMPDIR:-/tmp}/docket-asset-drift.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT

# Build the comparator ONCE and drive it three times with -repo. `go run` would
# recompile per invocation, and the flag is the generator's own supported way to
# read a repo that is not the working directory.
tool="$scratch/genassets"
build_out="$(go build -o "$tool" ./cmd/genassets 2>&1)"
build_rc=$?
assert "cmd/genassets builds" '[ "$build_rc" -eq 0 ] || { printf "%s\n" "$build_out" >&2; false; }'
if [ "$build_rc" -ne 0 ]; then
  printf 'NOT OK - the drift gate cannot certify anything without the comparator\n'
  exit 1
fi

# --- Assert 1: the committed bundle matches the authored roots, right here ----
# No -repo: this exercises the default invocation a developer types, from the
# repo root, against the real working tree.
live_out="$("$tool" -check 2>&1)"
live_rc=$?
assert "internal/assets/embedded is in sync with the authored asset roots" \
  '[ "$live_rc" -eq 0 ] || { printf "%s\n" "$live_out" >&2; printf "  Regenerate with: go generate ./internal/assets/  (or: go run ./cmd/genassets)\n" >&2; false; }'

# --- The copied repo the next two asserts share ------------------------------
# Scoped to what the comparator reads: the four authored roots named by
# assets.DefaultAllowedRoots, plus the generated tree it compares against.
# ROOTS IS DERIVED, not hand-listed: the generator's own root table is the
# source of truth, so a root added there without a copy line here would leave
# this probe silently comparing a repo that is missing it.
copy="$scratch/repo"
mkdir -p "$copy/internal/assets"
roots="$(awk '/^func DefaultAllowedRoots\(/{inside=1} inside && match($0, /Root: "[^"]+"/){print substr($0, RSTART+7, RLENGTH-8)} inside && /^}$/{exit}' \
  "$REPO/internal/assets/generate.go")"
copy_rc=0
while IFS= read -r root; do
  [ -n "$root" ] || continue
  cp -R "$REPO/$root" "$copy/$root" || copy_rc=1
done <<<"$roots"
cp -R "$REPO/internal/assets/embedded" "$copy/internal/assets/embedded" || copy_rc=1
assert "the probe repo copies every root assets.DefaultAllowedRoots declares" \
  '[ "$copy_rc" -eq 0 ] && [ -n "$roots" ] && [ "$(grep -c . <<<"$roots")" -ge 4 ]'

# --- Assert 3 (control, run first): the untouched copy still passes -----------
# Without this, assert 2 proves nothing — an incomplete copy exits 1 too.
control_out="$("$tool" -check -repo "$copy" 2>&1)"
control_rc=$?
assert "the untouched probe copy passes -check (the mutation probe's control)" \
  '[ "$control_rc" -eq 0 ] || { printf "%s\n" "$control_out" >&2; false; }'

# --- Assert 2: mutation probe — one appended byte must redden -check ----------
# The victim is DERIVED from the bundle's own manifest rather than named: a
# hardcoded path rots the day that skill is renamed, and the guard would then
# mutate nothing and still report green.
victim_rel="$(awk -F'"' '/"path":[[:space:]]*"skills\//{print $4}' "$copy/internal/assets/embedded/manifest.json" | LC_ALL=C sort)"
victim_rel="${victim_rel%%$'\n'*}"
assert "a bundled skills/ file was resolved to mutate" '[ -n "$victim_rel" ] && [ -f "$copy/$victim_rel" ]'
printf 'x' >> "$copy/$victim_rel"

mutated_out="$("$tool" -check -repo "$copy" 2>&1)"
mutated_rc=$?
assert "one appended byte in a bundled asset makes -check exit non-zero" \
  '[ "$mutated_rc" -ne 0 ] || { printf "  -check passed after mutating %s — the drift gate is decoration\n" "$victim_rel" >&2; false; }'
assert "the drift report names the mutated path" \
  'grep -qF -- "$victim_rel" <<<"$mutated_out" || { printf "%s\n" "$mutated_out" >&2; false; }'

exit $fail
