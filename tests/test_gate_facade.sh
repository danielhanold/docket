#!/usr/bin/env bash
# tests/test_gate_facade.sh — the cross-PROCESS half of the run-gate facade
# acceptance matrix (change 0334, Task 5).
#
# The in-process half lives in Go (internal/app/rungate_*_test.go): those tests
# drive RunGateBefore / RunGateVerdict / RunGateVerdictObserve directly, with
# fakes, in ONE process. This file proves the other half the Go tests structurally
# cannot: that the durable gate record survives across SEPARATE OS processes and
# that the consumer-facing `docket.sh gate-before implement-next` /
# `docket.sh gate-verdict <key>` spelling — the shell facade delegators over the Go
# binary — carries the report line and exit code through untouched. Every case here
# is a fresh `docket.sh …` process against a hermetic fixture repo; nothing is
# shared between an arm and its verdict but the on-disk record under the repo's git
# common dir.
#
# The wrappers under test are thin: scripts/gate-before.sh / scripts/gate-verdict.sh
# forward argv to `$DOCKET_BIN run gate-before …` / `… gate-verdict …` and exec, so
# stdout and the exit code pass through unmodified. DOCKET_BIN is their one mock
# seam; the delegator block below drives it with a stub that only echoes its argv,
# proving the passthrough without the Go binary. The end-to-end blocks then use the
# REAL built binary.
#
# Requires a Go toolchain and git on PATH (go.mod pins the Go version); fails loudly
# rather than skipping — a skipped gate certifies nothing.
#
# CACHES. scripts/run-tests.sh gives every job a private HOME, so with
# GOMODCACHE/GOCACHE unset `go` finds neither a module cache nor a build cache and
# recompiles cold on every suite run. This file pins both to
# `<git common dir>/docket-go-cache/{mod,build}` when the caller has not chosen its
# own — the same location and reasoning as tests/test_go_toolchain.sh and
# tests/test_go_finalize_e2e.sh (outside every working tree, shared across
# worktrees, `-modcacherw` so an ordinary `rm -rf` can still remove it). Only the
# first run after a fresh clone needs the network.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# Isolate git and the docket global-config layer from the developer's environment:
# a synthetic committer identity carried in the environment (never written to global
# config) and an empty XDG_CONFIG_HOME so a real ~/.config/docket cannot steer the
# binary's config resolution. The suite runner already isolates these; this keeps a
# standalone `bash tests/test_gate_facade.sh` hermetic too.
export GIT_AUTHOR_NAME=docket-test GIT_AUTHOR_EMAIL=t@t
export GIT_COMMITTER_NAME=docket-test GIT_COMMITTER_EMAIL=t@t

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
assert "git is on PATH" 'command -v git >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1 || ! command -v git >/dev/null 2>&1; then
  printf 'NOT OK - the gate facade matrix cannot certify anything without go and git\n'
  exit 1
fi

# A Bash 4+ interpreter for docket.sh's POSIX bootstrap to hand off to (macOS ships
# 3.2 as /bin/bash). Same probe as tests/test_docket_facade.sh.
DOCKET_BASH_PATH="$(command -v bash)"
for candidate in "$DOCKET_BASH_PATH" /opt/homebrew/bin/bash /usr/local/bin/bash; do
  [ -x "$candidate" ] || continue
  candidate_major="$("$candidate" -c 'echo "${BASH_VERSINFO[0]}"' 2>/dev/null)"
  if [ -n "$candidate_major" ] && [ "$candidate_major" -ge 4 ]; then DOCKET_BASH_PATH="$candidate"; break; fi
done
export DOCKET_BASH_PATH

TMP="$(mktemp -d "${TMPDIR:-/tmp}/gate-facade.XXXXXX")" || exit 1
trap 'rm -rf "$TMP"' EXIT

# Keep whatever GOFLAGS the caller set; append rather than replace.
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

# Build ./cmd/docket ONCE into the test tmpdir; every case reuses it.
BIN="$TMP/docket"
build_out="$(go build -o "$BIN" ./cmd/docket 2>&1)"
assert "the docket binary builds for the facade matrix" '[ -x "$BIN" ] || { printf "%s\n" "$build_out" >&2; false; }'
if [ ! -x "$BIN" ]; then
  printf 'NOT OK - cannot run the facade matrix without a built binary\n'
  exit 1
fi

# A minimal fake `gh` on PATH for the run-incomplete cases: RunVerify resolves the
# repository (`gh repo view`) and probes open PRs (`gh pr list`). Returning a fixed
# identity and zero PRs is enough to drive a claimed-but-unimplemented change to
# `run-incomplete` (pr-unverified + the earlier conjuncts), which is what exercises
# the retry permit. gate-before and the no-attributable-claim verdict never invoke
# gh, so having it on PATH throughout is harmless.
GHDIR="$TMP/fakegh"
mkdir -p "$GHDIR"
cat > "$GHDIR/gh" <<'GH'
#!/usr/bin/env bash
case "$1 $2" in
  "repo view") printf '{"nameWithOwner":"acme/widget","owner":{"login":"acme"},"name":"widget","url":"https://github.com/acme/widget"}\n' ;;
  "pr list")   printf '[]\n' ;;
  *)           printf '[]\n' ;;
esac
exit 0
GH
chmod +x "$GHDIR/gh"

# future_stamp prints an RFC3339 UTC timestamp comfortably after "now" (GNU and BSD
# date), so a claim minted after gate-before always satisfies the attribution
# filter's `claimed_at >= DispatchEpoch`, free of any sub-second race.
future_stamp(){ date -u -v+1H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '+1 hour' +%Y-%m-%dT%H:%M:%SZ; }

# newfix builds a hermetic main-mode docket repo under $1 and prints the path of the
# invocation clone the facade operates on. main mode keeps the fixture minimal:
# every change record lives on the default branch, whose .docket.yml declares
# metadata_branch: main. Mirrors internal/app's newMainModeRepo.
newfix(){
  local root="$1" origin="$1/origin.git" writer="$1/writer" inv="$1/invocation"
  mkdir -p "$root"
  git init --bare -q -b main "$origin"
  git init -q -b main "$writer"
  git -C "$writer" config user.email t@t; git -C "$writer" config user.name docket-test
  printf 'metadata_branch: main\nfinalize:\n  test_command: '\''exit 0'\''\n' > "$writer/.docket.yml"
  printf 'readme\n' > "$writer/README.md"
  mkdir -p "$writer/docs/changes/active"
  git -C "$writer" add -A
  git -C "$writer" commit -q -m "main content"
  git -C "$writer" remote add origin "$origin"
  git -C "$writer" push -q -u origin main
  git clone -q "$origin" "$inv"
  git -C "$inv" config user.email t@t; git -C "$inv" config user.name docket-test
  printf '%s\n' "$inv"
}

# push_in_progress writes an in-progress change record ($2=id, $3=slug, $4=claimed_at)
# to the writer clone of fixture $1 and pushes it to origin main. No pr, no plan, no
# feature branch — so RunVerify reports run-incomplete on it.
push_in_progress(){
  local writer="$1/writer" id="$2" slug="$3" claimed="$4" padded
  printf -v padded '%04d' "$id"
  cat > "$writer/docs/changes/active/$padded-$slug.md" <<EOF
---
id: $id
slug: $slug
title: Change $slug
status: in-progress
priority: high
type: feat
created: 2026-01-02
claimed_at: $claimed
branch: feat/$slug
---

Body of $slug.
EOF
  git -C "$writer" add -A
  git -C "$writer" commit -q -m "claim $id"
  git -C "$writer" push -q origin main
}

# dk runs the real shell facade against the built binary, gh on PATH, an isolated
# XDG config home per fixture. Every case is its own process — the point of the file.
dk(){
  local inv="$1"; shift
  DOCKET_BIN="$BIN" XDG_CONFIG_HOME="$inv/../xdg" PATH="$GHDIR:$PATH" \
    bash "$REPO/scripts/docket.sh" "$@" --repo-dir "$inv"
}

# ============================================================================
# (0) Delegator seam — argv passthrough + exit-code passthrough via the DOCKET_BIN
# mock, no Go binary. Proves the wrappers are pure forwarders (the verify-run.sh
# seam pattern).
# ============================================================================
STUB="$TMP/stub-docket"
cat > "$STUB" <<'STUBSH'
#!/usr/bin/env bash
printf 'ARGV:%s\n' "$*"
exit 7
STUBSH
chmod +x "$STUB"

seam_before="$(DOCKET_BIN="$STUB" bash "$REPO/scripts/gate-before.sh" implement-next --repo-dir /x 2>&1)"; seam_before_rc=$?
assert "gate-before.sh forwards argv to '\$DOCKET_BIN run gate-before …'" \
  '[ "$seam_before" = "ARGV:run gate-before implement-next --repo-dir /x" ]'
assert "gate-before.sh passes the binary exit code through untouched" '[ "$seam_before_rc" -eq 7 ]'

seam_verdict="$(DOCKET_BIN="$STUB" bash "$REPO/scripts/gate-verdict.sh" --unattributed 42 2>&1)"; seam_verdict_rc=$?
assert "gate-verdict.sh forwards argv to '\$DOCKET_BIN run gate-verdict …'" \
  '[ "$seam_verdict" = "ARGV:run gate-verdict --unattributed 42" ]'
assert "gate-verdict.sh passes the binary exit code through untouched" '[ "$seam_verdict_rc" -eq 7 ]'

# ============================================================================
# (a)+(f) `docket.sh gate-before implement-next` arms the gate: `gate-armed <key>`,
# exit 0 (a report line is never a process failure).
# ============================================================================
FIXA="$TMP/fixa"; INVA="$(newfix "$FIXA")"
armed_out="$(dk "$INVA" gate-before implement-next 2>&1)"; armed_rc=$?
assert "gate-before implement-next prints a gate-armed report line" \
  '[ "${armed_out%% *}" = "gate-armed" ]'
assert "the armed report line exits 0" '[ "$armed_rc" -eq 0 ]'
KEYA="$(awk '{print $2}' <<<"$armed_out")"
assert "the armed report line carries a non-empty key" '[ -n "$KEYA" ]'

# ============================================================================
# (b) A SEPARATE process reads the durable record: `gate-verdict <key>` with no
# attributable claim reports `gate-done <key> no-attributable-claim`, exit 0. The
# arm and this verdict share nothing but the on-disk record — restart durability.
# ============================================================================
verdictb_out="$(dk "$INVA" gate-verdict "$KEYA" 2>&1)"; verdictb_rc=$?
assert "a fresh-process gate-verdict resolves the durable record" \
  '[ "$verdictb_out" = "gate-done $KEYA no-attributable-claim" ]'
assert "the durable-record verdict exits 0" '[ "$verdictb_rc" -eq 0 ]'

# ============================================================================
# (c) The one retry permit is consumed durably ACROSS processes. Arm, then (in a
# later process) introduce one attributable in-progress claim that verifies to
# run-incomplete: the first verdict spends the permit (`gate-retry-once`), and a
# SECOND, separate-process verdict on the same key sees the permit already gone
# (`gate-stop … run-incomplete`). Exactly one gate-retry-once is ever granted.
# ============================================================================
FIXC="$TMP/fixc"; INVC="$(newfix "$FIXC")"
armedc_out="$(dk "$INVC" gate-before implement-next 2>&1)"
KEYC="$(awk '{print $2}' <<<"$armedc_out")"
assert "the retry fixture armed with a key" '[ "${armedc_out%% *}" = "gate-armed" ] && [ -n "$KEYC" ]'
# The claim appears AFTER the arm, with a claimed_at after the dispatch epoch.
push_in_progress "$FIXC" 42 alpha "$(future_stamp)"

verdictc1_out="$(dk "$INVC" gate-verdict "$KEYC" 2>&1)"; verdictc1_rc=$?
assert "the first verdict on an incomplete run grants the single retry" \
  '[ "${verdictc1_out%% *}" = "gate-retry-once" ]'
assert "the granted-retry line names run-incomplete for the attributed id" \
  'grep -qF "run-incomplete 42" <<<"$verdictc1_out"'
assert "the gate-retry-once line exits 0" '[ "$verdictc1_rc" -eq 0 ]'

verdictc2_out="$(dk "$INVC" gate-verdict "$KEYC" 2>&1)"; verdictc2_rc=$?
assert "a later process sees the retry permit already consumed" \
  '[ "${verdictc2_out%% *}" = "gate-stop" ]'
assert "the second verdict still reports run-incomplete for the same id" \
  'grep -qF "run-incomplete 42" <<<"$verdictc2_out"'
assert "the second, permit-spent verdict is not another gate-retry-once" \
  '! grep -qF "gate-retry-once" <<<"$verdictc2_out"'
assert "the gate-stop line exits 0" '[ "$verdictc2_rc" -eq 0 ]'

# ============================================================================
# (d) `gate-verdict --unattributed` works through the wrapper: observe-only, no
# key, no record. An empty backlog reports `gate-observe no-current-run`; a hint id
# naming a proposed change reports `gate-observe run-unclaimed <id>`.
# ============================================================================
observe_out="$(dk "$INVA" gate-verdict --unattributed 2>&1)"; observe_rc=$?
assert "unattributed gate-verdict on an empty backlog reports no-current-run" \
  '[ "$observe_out" = "gate-observe no-current-run" ]'
assert "the observe report line exits 0" '[ "$observe_rc" -eq 0 ]'

FIXD="$TMP/fixd"; INVD="$(newfix "$FIXD")"
cat > "$FIXD/writer/docs/changes/active/0009-beta.md" <<'EOF'
---
id: 9
slug: beta
title: Change beta
status: proposed
priority: high
type: feat
created: 2026-01-02
---

Body of beta.
EOF
git -C "$FIXD/writer" add -A
git -C "$FIXD/writer" commit -q -m "propose 9"
git -C "$FIXD/writer" push -q origin main
observe_hint_out="$(dk "$INVD" gate-verdict --unattributed 9 2>&1)"
assert "an unattributed hint verifies the named id verbatim (run-unclaimed)" \
  '[ "$observe_hint_out" = "gate-observe run-unclaimed 9" ]'

# ============================================================================
# (e) An unknown op spelling still errors through the facade: the WRAPPED_OPS
# allowlist is exact, so a near-miss is rejected (exit 2), never silently run.
# ============================================================================
DOCKET_BIN="$BIN" DOCKET_BASH_PATH="$DOCKET_BASH_PATH" bash "$REPO/scripts/docket.sh" gate-verdictx >/dev/null 2>&1
assert "an unknown op near-miss (gate-verdictx) is rejected with exit 2" '[ $? -eq 2 ]'
DOCKET_BIN="$BIN" DOCKET_BASH_PATH="$DOCKET_BASH_PATH" bash "$REPO/scripts/docket.sh" gate-befor >/dev/null 2>&1
assert "an unknown op near-miss (gate-befor) is rejected with exit 2" '[ $? -eq 2 ]'

exit "$fail"
