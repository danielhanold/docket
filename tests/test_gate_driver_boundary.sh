#!/usr/bin/env bash
# tests/test_gate_driver_boundary.sh — change 0342. Whole-repository architectural boundary guard
# for the native gate DRIVER (`docket gate drive start|advance|handoff|claim`, internal/gatedrive).
#
# WHAT THIS FILE IS FOR: change 0342 made the native gate driver the SOLE workflow surface for
# advancing a suite run. The raw verbs (`docket gate launch|observe|stop`) and the app-orchestration
# seam (`GateLaunch|GateObserve|GateStop`) survive as PRIMITIVES the driver composes — a workflow
# caller (a skill, an agent definition, an executable workflow script, an orchestration Go path)
# never composes them directly, never re-parses raw observation state, never recreates a sleep/poll
# loop, and a task-level `WAITING` return always names an explicit ownership handoff. This guard
# proves the whole tree obeys that boundary.
#
# HOW IT CLASSIFIES — by source SHAPE, never a per-file allowlist (CLAUDE.md: "derive the sites from
# a whole-repo grep, then sort them into prose vs executable"). Every match is sorted into one of:
#   executable-workflow      -> raw use here is a VIOLATION
#   primitive-impl / driver / cli / process  -> PERMITTED (the raw verbs legitimately live here)
#   primitive-level test / fixture / golden  -> PERMITTED
#   operator / point-in-time prose (docs/*)  -> PERMITTED (immutable + operator records)
# The category is derived from PATH shape (which layer the file is) plus CONTENT shape (a fenced
# runnable command vs an inline back-tick primitive MENTION; a call site `.GateLaunch(` vs a prose
# reference). The four sibling detectors below are each keyed on a syntactic shape a future
# regression re-grows into, never on an enumerated spelling of today's call sites.
#
# PROSE vs EXECUTABLE in markdown: a raw verb inside a fenced code block (```...```) is a runnable
# recipe and a VIOLATION; the same verb in an inline `code span` is a primitive being NAMED in prose
# and is PERMITTED — the retired caller loop, the finalize block-and-observe path, and the
# integration-repair agent-def all legitimately NAME the raw verbs in prose. Residual risk, recorded
# not hidden: an author who writes an imperative raw invocation in inline back-ticks (arguments and
# all) rather than a fence dodges the markdown detector; the fence is the mechanical shape signal the
# house convention already uses for every runnable recipe, and the Go/script/handoff detectors carry
# the rest of the teeth. The mutation proofs live in tests/test_gate_driver_boundary_mutation.sh.
#
# SCAN ROOT: default is the repo. `--scan-only <DIR>` runs ONLY the violation detectors against DIR
# (used by the mutation proofs to point the real classifier at a crafted fixture tree) and exits 1
# if any violation is found, 0 otherwise — no repo-structure floors, no TAP output.
#
# The assert helper is the tree's canonical one byte for byte (scripts/check-test-source-hygiene.sh
# rule (a) is a byte-exact allowlist); scripts/run-tests.sh accounts results on the ok/NOT OK markers.
set -uo pipefail

# --- shape detectors, each takes a scan ROOT and prints one line per violation ---------------------

# Enumerate the executable-workflow markdown/agent-def files under ROOT (skills, agent defs, and the
# embedded authored/generated copies of both). Path shape only — no per-file list.
_workflow_md() {
  local root="$1" d
  for d in "skills" "agents" \
           "internal/assets/embedded/tree/skills" "internal/assets/embedded/tree/agents"; do
    [ -d "$root/$d" ] || continue
    find "$root/$d" -type f \( -name '*.md' -o -name '*.toml' \) -print
  done
}

# Enumerate executable workflow shell scripts (scripts/*.sh). Tests live under tests/ and are a
# permitted primitive-level category, so they are NOT enumerated here.
_workflow_sh() {
  local root="$1"
  [ -d "$root/scripts" ] || return 0
  find "$root/scripts" -type f -name '*.sh' -print
}

# (A) A raw `docket gate launch|observe|stop` verb inside a FENCED code block of a workflow markdown
# file, OR on a non-comment command line of a workflow shell script. Shape: fence state via awk; a
# `#`-led line in a script is a comment, not a command.
_scan_raw_fenced() {
  local root="$1" f block_hits sh_hits
  local files; files="$(_workflow_md "$root")"
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    block_hits="$(awk '
      /^[[:space:]]*```/ { infence = !infence; next }
      infence && /docket[[:space:]]+gate[[:space:]]+(launch|observe|stop)([[:space:]]|$)/ {
        printf "A\t%s:%d: %s\n", FILENAME, NR, $0
      }' "$f")"
    [ -n "$block_hits" ] && printf '%s\n' "$block_hits"
  done <<<"$files"
  local scripts; scripts="$(_workflow_sh "$root")"
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    sh_hits="$(awk '
      /^[[:space:]]*#/ { next }
      /docket[[:space:]]+gate[[:space:]]+(launch|observe|stop)([[:space:]]|$)/ {
        printf "A\t%s:%d: %s\n", FILENAME, NR, $0
      }' "$f")"
    [ -n "$sh_hits" ] && printf '%s\n' "$sh_hits"
  done <<<"$scripts"
}

# (B) A direct app-orchestration CALL to GateLaunch/GateObserve/GateStop (shape: `.GateLaunch(`)
# OUTSIDE the layers where the raw seam legitimately lives. Permitted layers, by path shape:
#   internal/cli/**        — the raw `docket gate` CLI adapter (the raw verbs' own impl)
#   internal/gatedrive/**  — the driver, which composes the primitive
#   internal/process/**    — the process primitive itself
#   internal/app/gate*.go  — the gate seam DEFINITIONS and supervisor (the primitive package)
# A call anywhere else (a finalize/implement-next/build orchestration path) is a VIOLATION.
_scan_direct_go_call() {
  local root="$1" f hits rel
  local files
  files="$( { [ -d "$root/internal" ] && find "$root/internal" -type f -name '*.go' ! -name '*_test.go' -print; \
              [ -d "$root/cmd" ] && find "$root/cmd" -type f -name '*.go' ! -name '*_test.go' -print; } )"
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    rel="${f#"$root"/}"
    case "$rel" in
      internal/cli/*|internal/gatedrive/*|internal/process/*) continue ;;
    esac
    # the gate seam definitions/supervisor: internal/app/gate*.go (basename shape, the primitive family)
    case "$rel" in
      internal/app/gate*.go) continue ;;
    esac
    hits="$(grep -nE '\.(GateLaunch|GateObserve|GateStop)\(' "$f" 2>/dev/null || true)"
    [ -n "$hits" ] && while IFS= read -r line; do
      printf 'B\t%s:%s\n' "$rel" "$line"
    done <<<"$hits"
  done <<<"$files"
}

# (C) A workflow copy that RE-PARSES raw observation state or RECREATES a sleep/poll loop: a fenced
# block of a workflow markdown file that mentions `docket gate observe` together with a poll idiom
# (jq extraction, a `sleep <n>` delay, or a `while` loop). Shape, not spelling.
_scan_poll_loop() {
  local root="$1" f block
  local files; files="$(_workflow_md "$root")"
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    block="$(awk '/^[[:space:]]*```/{inf=!inf; next} inf{print}' "$f")"
    if grep -qE 'docket[[:space:]]+gate[[:space:]]+observe' <<<"$block" \
       && grep -qiE 'jq|sleep[[:space:]]+[0-9]|while' <<<"$block"; then
      printf 'C\t%s: fenced observe poll/parse loop\n' "${f#"$root"/}"
    fi
  done <<<"$files"
}

# (D) A task-level WAITING return contract that omits the explicit handoff identity. Shape: a
# workflow contract that declares `WAITING` as an AGENT-RETURN outcome token — the bare uppercase
# token co-located with its return siblings COMPLETE/BLOCKED/NEEDS_ESCALATION (NOT the driver
# dispositions PASSED/FAILED/HALTED, and NOT the hyphenated `run-waiting` verdict) — MUST also name
# the single-use handoff token AND state that a bare wait with no handoff is not a valid return.
# Missing EITHER clause is a VIOLATION.
_scan_waiting_handoff() {
  local root="$1" f flat sib='COMPLETE|BLOCKED|NEEDS_ESCALATION'
  local files; files="$(_workflow_md "$root")"
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    # strip back-ticks so a back-tick-wrapped vocabulary still reads as a token set, then flatten wraps
    flat="$(tr -d '\140' < "$f" | tr -s '[:space:]' ' ')"
    if grep -qE "(^|[^-A-Za-z])WAITING([^-A-Za-z]).{0,45}($sib)" <<<"$flat" \
       || grep -qE "($sib)([^-A-Za-z]).{0,45}([^-A-Za-z])WAITING([^-A-Za-z])" <<<"$flat"; then
      local names_token=0 forbids_bare=0
      grep -qiE 'handoff token' <<<"$flat" && names_token=1
      grep -qiE 'no handoff token|without[^.]{0,25}handoff|bare[^.]{0,45}wait' <<<"$flat" && forbids_bare=1
      if [ "$names_token" -eq 0 ] || [ "$forbids_bare" -eq 0 ]; then
        printf 'D\t%s: WAITING outcome without explicit handoff identity (token=%d bare-invalid=%d)\n' \
          "${f#"$root"/}" "$names_token" "$forbids_bare"
      fi
    fi
  done <<<"$files"
}

scan_violations() {
  local root="$1"
  _scan_raw_fenced "$root"
  _scan_direct_go_call "$root"
  _scan_poll_loop "$root"
  _scan_waiting_handoff "$root"
}

# --- --scan-only mode: run detectors against an arbitrary root (mutation-proof harness) ------------
if [ "${1:-}" = "--scan-only" ]; then
  root="${2:?--scan-only needs a directory}"
  root="$(cd "$root" && pwd -P)"
  violations="$(scan_violations "$root")"
  if [ -n "$violations" ]; then
    printf '%s\n' "$violations"
    exit 1
  fi
  exit 0
fi

# --- default mode: TAP-style asserts against the repo ---------------------------------------------
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# Population floors FIRST: an empty enumeration passes every "no violations" negative by default, so
# the guard must redden here if it fails to SEE the source it exists to police.
wf_md_count="$(_workflow_md "$REPO" | grep -c .)"
go_count="$( { find "$REPO/internal" "$REPO/cmd" -type f -name '*.go' ! -name '*_test.go' -print 2>/dev/null; } | grep -c .)"
assert "the guard enumerates the workflow contract corpus (>= 20 files)" '[ "$wf_md_count" -ge 20 ]'
assert "the guard enumerates the Go orchestration corpus (>= 20 files)" '[ "$go_count" -ge 20 ]'

# Sanity floor: the driver surface it is protecting actually exists in the tree.
assert "the native gate driver package is present" '[ -d "$REPO/internal/gatedrive" ]'
assert "at least one contract declares the WAITING outcome (detector D has live input)" \
  'grep -rqE "(^|[^-A-Za-z])WAITING([^-A-Za-z])" "$REPO/skills/docket-build-task/SKILL.md"'

# The boundary itself: the whole tree is clean under every shape detector.
violations="$(scan_violations "$REPO")"
assert "no executable workflow composes the raw gate verbs in a fenced recipe (A)" \
  '[ -z "$(grep -E "^A"$'"'"'\t'"'"' <<<"$violations")" ]'
assert "no orchestration Go path calls GateLaunch/Observe/Stop outside cli/driver/process (B)" \
  '[ -z "$(grep -E "^B"$'"'"'\t'"'"' <<<"$violations")" ]'
assert "no workflow copy re-parses raw observe or recreates a sleep/poll loop (C)" \
  '[ -z "$(grep -E "^C"$'"'"'\t'"'"' <<<"$violations")" ]'
assert "every task-level WAITING contract names an explicit handoff identity (D)" \
  '[ -z "$(grep -E "^D"$'"'"'\t'"'"' <<<"$violations")" ]'
assert "the tree is fully clean under the gate-driver boundary" '[ -z "$violations" ]'

if [ "$fail" -ne 0 ] && [ -n "$violations" ]; then
  printf 'boundary violations:\n%s\n' "$violations" >&2
fi
exit "$fail"
