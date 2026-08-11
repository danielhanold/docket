<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0284 — runner-dispatch --observe is sentinel-only: adopt 0282's identity-checked liveness probe](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0284-runner-dispatch-observe-is-sentinel-only-adopt-0282-s-identi.md)**
<!-- docket:backlink:end -->

# `runner-dispatch --observe`: an identity-checked liveness probe — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `runner-dispatch.sh --observe` an identity-checked process-liveness probe, so a delegated child that died without writing its `done` sentinel is detected on the next observation instead of at the far end of a 60-minute budget.

**Architecture:** Extract the liveness predicate `gate-run.sh` already owns into a new `scripts/lib/docket-liveness.sh` that takes **values, not run dirs** (the two consumers' on-disk record layouts are incompatible and both are load-bearing). Refactor both existing sites onto it, then insert a new step into `--observe`'s read order between the terminal-state reads and the clock arithmetic. A child found dead is disposed by **git**, not assumed resultless, and the verdict is recorded by reusing the existing `killed` marker with a new `cause=child-vanished`.

**Tech Stack:** Bash 4+ (`DOCKET_BASH_PATH`), POSIX `ps`/`kill`, the repo's own `tests/test_*.sh` assert harness, `tests/runtime-budgets.tsv`.

## Global Constraints

Copied verbatim from the spec, the change's reconcile log, and `CLAUDE.md`. **Every task's requirements implicitly include this section.**

1. **`tests/test_gate_run.sh` and `tests/test_gate_run_stop.sh` must pass UNCHANGED.** The §1 refactor is behaviour-preserving for gate-run; an edit to either file is the tell that it was not. Do not edit them. Do not "fix" them.
2. **Two source-shape asserts in `tests/test_gate_run.sh` pin symbol spellings** (reconcile finding 1). `tests/test_gate_run.sh:191` greps for `identity_matches "$RUN_DIR"` and `:200` greps for `SPAWN_IDENT="$(identity_of "$SPAWN_PID")"`. Therefore **`identity_of` and `identity_matches` SURVIVE in `gate-run.sh` as thin delegations onto the lib** — the conjunct ladder moves out, the spellings stay. The spec's §1 sentence "identity_of and group_alive_and_ours are deleted" is superseded for `identity_of` only. `group_alive_and_ours` **is** collapsed into the lib call (no test pins its spelling).
3. **`identity_matches` must NOT collapse into `docket_group_alive_and_ours`.** That would add a `kill -0` conjunct to gate-run's pre-signal re-check at `gate-run.sh`'s `stop_run`, which is a re-specification; the spec forbids re-specifying gate-run.
4. **The dead path inherits change 0208's two non-verdict legs** (reconcile finding 2): when `ANCHOR_FALLBACK=1` the verdict is `task-unverifiable worktree-removed`, and when the launch record carries no `branch` it is `task-unverifiable launch-branch-missing`. Never fall back to the observation-time branch or to the main worktree.
5. **No new exit code.** `0` / `1` / `4` already span the outcomes. The generated shim's loop keys on `4`.
6. **No new terminal file.** The dead path reuses the `killed` marker: `cause=child-vanished`, `reason=group-already-gone`.
7. **Never `producer | early-exiting-consumer` under `set -o pipefail`** — capture into a variable, then `grep <<<"$var"`.
8. **`grep` for a pattern that leads with `--` must declare it**: `grep -E -e "<pat>"` or `grep -qF -- "<pat>"`.
9. **`mv -f`, never bare `mv`**, on any install/replace path.
10. **A guard is code: mutation-test it.** Deletion and inversion are DIFFERENT probes; a comparison operator needs both. **Confirm the probe actually CHANGED BYTES** (`grep -c` before/after, and the counter literal must be capable of changing) before trusting a green.
11. **Key a guard on syntactic SHAPE, never an enumerated list of spellings.**
12. **Match comment-stripped lines** in any contract guard whose literal is also discussed in that document's own prose (`runner-dispatch.md` will now discuss `cause=child-vanished`).
13. **Run the whole suite at the build gate** via `scripts/run-tests.sh`. A trailing `OVER BUDGET:` line is a finding to act on.
14. **NEVER run `scripts/run-tests.sh --timings <test path>` against a real test file** — it truncates the named file to zero bytes (issue #0290, unfixed). Measure with `time bash tests/<file>.sh` instead.
15. `tests/test_sync_agents_runners.sh` at ~190s against a 60s ceiling is PRE-EXISTING (#0280) — leave it alone.
16. **`tests/test_runtime_budgets.sh` pins `EXPECTED_TOTAL`** (the sum of every ceiling). A new row or a raised ceiling reddens it; re-seed and say which case it was.

## File Structure

| File | Responsibility |
|---|---|
| `scripts/lib/docket-liveness.sh` | **New.** The single definition of "is this recorded process group still alive and still ours?", as two value-taking functions plus a caller-printable reason variable. No knowledge of any record layout. |
| `scripts/gate-run.sh` | Sources the lib. `identity_of`/`identity_matches` become layout-adapting delegations; `group_alive_and_ours` becomes one lib call. Behaviour unchanged. |
| `scripts/runner-dispatch.sh` | Sources the lib. `ps_lstart` deleted; `terminate_dispatch`'s conjunct ladder replaced by one lib call + `DOCKET_LIVENESS_WHY`. Gains the test-only `barrier` hook, the new `--observe` liveness leg, the dead-path git dispositions, and the `cause=child-vanished` reader arm. |
| `scripts/runner-dispatch.md` | The *Liveness vs correctness* section rewritten; the give-up path's residual paragraph amended. |
| `tests/test_docket_liveness.sh` | **New.** Pins the lib's interface and every fail-closed leg, against real self-spawned groups. |
| `tests/test_runner_dispatch_observe.sh` | New arms for the seven cases in the spec's § Testing. |
| `tests/runtime-budgets.tsv` | One new row; re-measured ceiling for the observe shard. |
| `tests/test_runtime_budgets.sh` | `EXPECTED_TOTAL` re-seeded. |

---

### Task 1: The liveness lib and its test

**Files:**
- Create: `scripts/lib/docket-liveness.sh`
- Create: `tests/test_docket_liveness.sh`
- Modify: `tests/runtime-budgets.tsv` (add one row)
- Modify: `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`, line 28)

**Interfaces:**
- Consumes: nothing (this is the base of the change).
- Produces, relied on by Tasks 2–5:
  - `docket_identity_of <pid>` → prints the normalized `ps -o lstart=` token on stdout; empty string when the pid is gone or `ps` is unreadable. **Always returns 0** so a caller under `set -e` can assign from it.
  - `docket_group_alive_and_ours <pgid> <expected-identity>` → returns `0` when the group exists AND the process leading it started at `<expected-identity>`; non-zero otherwise. Sets the global `DOCKET_LIVENESS_WHY` to a caller-printable reason on **every** non-zero return, and to the empty string on `0`.

- [ ] **Step 1: Write the failing test**

Create `tests/test_docket_liveness.sh`. Note the fixture shape: a real self-spawned background group, because the predicate is about real processes. `set -m` makes a background job its own group leader, which is the same primitive the two production callers rely on.

```bash
#!/usr/bin/env bash
# tests/test_docket_liveness.sh — scripts/lib/docket-liveness.sh, the ONE identity-checked liveness
# predicate shared by gate-run.sh and runner-dispatch.sh (change 0284).
# Run: bash tests/test_docket_liveness.sh
#
# The lib takes VALUES, never run dirs: its two consumers store their records in incompatible
# layouts and each keeps its own reader. So every case here passes a pgid and a token directly,
# which is also what makes the file hermetic — no run dir, no launcher, no git.
set -uo pipefail
unset XDG_CONFIG_HOME
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

. "$ROOT/scripts/lib/docket-liveness.sh"

# --- a real self-spawned group -----------------------------------------------------
# `set -m` in a subshell makes the background job a PROCESS-GROUP LEADER, so its pid IS its pgid —
# the same construction runner-dispatch.sh's --launch uses and gate-run.sh's rung 3 falls back to.
# The marker is passed as the executed program's OWN argument rather than embedded in a `-c`
# comment: a single simple command under `sh -c` is EXEC'd, so a comment marker vanishes from the
# argv the kernel shows (learnings: exec-optimization-erases-the-process-marker).
PGIDS=()
cleanup(){ local p; for p in "${PGIDS[@]:-}"; do case "$p" in ''|*[!0-9]*) continue ;; esac
  kill -KILL -"$p" 2>/dev/null; done; }
trap cleanup EXIT

spawn_group(){  # sets SPAWN_PID (== its own pgid); the child sleeps until killed
  set -m
  sleep 300 &
  SPAWN_PID=$!
  set +m
  PGIDS+=("$SPAWN_PID")
  # Fixture sanity is asserted by the caller, not assumed here.
}

# ---- docket_identity_of ----------------------------------------------------------
spawn_group
assert "fixture sanity: the spawned job leads its own process group" \
  '[ "$(ps -o pgid= -p "$SPAWN_PID" 2>/dev/null | tr -d " ")" = "$SPAWN_PID" ]'

tok="$(docket_identity_of "$SPAWN_PID")"
assert "docket_identity_of returns a non-empty token for a live pid" '[ -n "$tok" ]'
assert "docket_identity_of is stable across calls" '[ "$(docket_identity_of "$SPAWN_PID")" = "$tok" ]'
assert "the token is whitespace-normalized (no runs, no edges)" \
  '[ "$tok" = "$(tr -s "[:space:]" " " <<<"$tok" | sed -e "s/^ //" -e "s/ $//")" ]'
# Always-0 is load-bearing: gate-run.sh runs under `set -e` and ASSIGNS from this.
docket_identity_of 999999 >/dev/null; rc=$?
assert "docket_identity_of returns 0 even for a gone pid (set -e callers assign from it)" '[ "$rc" = "0" ]'
assert "docket_identity_of is empty for a gone pid" '[ -z "$(docket_identity_of 999999)" ]'

# ---- docket_group_alive_and_ours: the happy leg ----------------------------------
DOCKET_LIVENESS_WHY="stale-sentinel"
docket_group_alive_and_ours "$SPAWN_PID" "$tok"; rc=$?
assert "a live group with a MATCHING token is alive" '[ "$rc" = "0" ]'
assert "DOCKET_LIVENESS_WHY is cleared on the alive leg" '[ -z "$DOCKET_LIVENESS_WHY" ]'

# ---- the pid-reuse case: live group, MISMATCHED token ----------------------------
docket_group_alive_and_ours "$SPAWN_PID" "Thu Jan  1 00:00:00 1970"; rc=$?
assert "a live group with a MISMATCHED token is DEAD (the pid-reuse case)" '[ "$rc" != "0" ]'
assert "and the reason names the mismatch, quoting both tokens" \
  '[ -n "$DOCKET_LIVENESS_WHY" ] && grep -qF "1970" <<<"$DOCKET_LIVENESS_WHY" && grep -qF "$tok" <<<"$DOCKET_LIVENESS_WHY"'

# ---- an empty expected token fails CLOSED ----------------------------------------
docket_group_alive_and_ours "$SPAWN_PID" ""; rc=$?
assert "an EMPTY expected token is dead — nothing to compare is not agreement" '[ "$rc" != "0" ]'
assert "and it says so" '[ -n "$DOCKET_LIVENESS_WHY" ]'

# ---- the same group after it exits ------------------------------------------------
kill -KILL -"$SPAWN_PID" 2>/dev/null
waited=0
while kill -0 -"$SPAWN_PID" 2>/dev/null && [ "$waited" -lt 100 ]; do sleep 0.1; waited=$(( waited + 1 )); done
assert "fixture sanity: the group is really gone" '! kill -0 -"$SPAWN_PID" 2>/dev/null'
docket_group_alive_and_ours "$SPAWN_PID" "$tok"; rc=$?
assert "a group that has exited is dead" '[ "$rc" != "0" ]'
assert "and the reason is non-empty" '[ -n "$DOCKET_LIVENESS_WHY" ]'

# ---- the syntactic floor: NOTHING is probed, so no kill is issued ----------------
# `kill -0 -0` means THIS caller's own group and `-1` means every process the user can signal, so
# neither may ever be treated as a recorded group. The assert is that no `kill` reaches the OS at
# all: a `kill` shim on PATH-free ground is not available inside a sourced function, so the probe
# is a FUNCTION OVERRIDE of the builtin's wrapper — shadowing `kill` in this shell records every
# call the lib makes.
KILLLOG="$(mktemp "${TMPDIR:-/tmp}/liveness-kill.XXXXXX")"
kill(){ printf '%s\n' "$*" >> "$KILLLOG"; builtin kill "$@"; }
for bad in "" "abc" "0" "1" "12x"; do
  : > "$KILLLOG"
  docket_group_alive_and_ours "$bad" "$tok"; rc=$?
  assert "a pgid of '${bad:-<empty>}' is dead" '[ "$rc" != "0" ]'
  assert "a pgid of '${bad:-<empty>}' explains itself" '[ -n "$DOCKET_LIVENESS_WHY" ]'
  assert "a pgid of '${bad:-<empty>}' probes NOTHING — no kill is issued" '[ ! -s "$KILLLOG" ]'
done
unset -f kill
rm -f "$KILLLOG"

exit "$fail"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_docket_liveness.sh`
Expected: FAIL — `scripts/lib/docket-liveness.sh: No such file or directory`.

- [ ] **Step 3: Write the lib**

Create `scripts/lib/docket-liveness.sh`. **The conjunct order is normative** — the syntactic pgid floor comes first (so a bad pgid probes nothing), then `kill -0`, then identity. That order is what makes gate-run's refactor behaviour-preserving: its `group_alive_and_ours` already ran `recorded_pgid` → `kill -0` → `identity_matches`.

```bash
#!/usr/bin/env bash
# scripts/lib/docket-liveness.sh — process-group liveness, IDENTITY-CHECKED (change 0284).
# Sourced by scripts/gate-run.sh and scripts/runner-dispatch.sh; never executed directly.
#
# ONE PREDICATE, TWO CONSUMERS. Before this lib each script carried its own copy: gate-run.sh's
# `group_alive_and_ours` and runner-dispatch.sh's inline ladder inside `terminate_dispatch`. Two
# copies of a predicate that must AGREE is the drift the `duplicated-gate-copies-the-whole-predicate`
# learning describes, and the copies had already diverged on one leg (an empty recorded token: the
# gate failed closed, the dispatch SKIPPED the conjunct). This file is the single definition.
#
# IT TAKES VALUES, NEVER RUN DIRS. The two consumers store their records in incompatible layouts —
# gate-run.sh keys `pid`/`pgid`/`identity` across `$rd/launch` plus a separate `$rd/identity` file,
# runner-dispatch.sh keys `pgid`/`child_pid`/`child_lstart` in `$DDIR/launch`. Each keeps its own
# reader and passes the extracted values in. A layout-aware lib would have to know both, which is
# exactly the coupling change 0282's assumption 1 rejected.
#
# WHY A PGID IS NOT ENOUGH: a pgid is a REUSABLE NAME. An hour after a child died the OS may have
# handed that id to an unrelated tree, and `kill -0 -<pgid>` would answer for the stranger. So the
# conjunction is: the group exists AND the process leading it started at the instant the caller
# recorded. Identity is the process START TIME, compared as an EXACT STRING and never parsed into a
# date — the `ps -o lstart=` rendering is platform- and locale-dependent, and both sides of every
# comparison come from the same `ps` on the same machine, so parsing it would add a failure mode
# and buy nothing.
#
# EVERY LEG FAILS CLOSED — an absent pgid, a non-numeric one, `0` or `1`, an empty recorded token,
# an empty live token. The asymmetry is the whole justification: a false `dead` costs one wasted
# observation, while a false `alive` costs the caller its ENTIRE budget on a run that is not there.

# The normalized start-time token for a pid; empty when the pid is gone or `ps` cannot be read.
# ALWAYS RETURNS 0: an absent pid is an empty token, not an error, so a caller under `set -e` can
# assign from it (gate-run.sh does, at `SPAWN_IDENT=`).
docket_identity_of(){  # $1 = pid -> normalized `ps -o lstart=` token, or empty
  local s
  s="$(ps -o lstart= -p "${1:-0}" 2>/dev/null || true)"
  s="$(tr -s '[:space:]' ' ' <<<"$s")"
  s="${s# }"
  printf '%s' "${s% }"
}

# The caller-printable reason for the most recent non-zero `docket_group_alive_and_ours`.
#
# THIS VARIABLE IS WHAT MAKES THIS ONE PREDICATE RATHER THAN TWO. runner-dispatch.sh's
# `terminate_dispatch` needs the REASON its identity check failed, for its "NOT signalling process
# group …" diagnostic; the new --observe leg needs only the boolean. A lib returning only a boolean
# would have forced `terminate_dispatch` to keep a private reason-producing copy — a second
# predicate wearing the first one's answer, which is the drift this file exists to prevent.
DOCKET_LIVENESS_WHY=""

# 0 when the group exists AND is still the one the caller recorded; non-zero otherwise.
# Sets DOCKET_LIVENESS_WHY on every non-zero return, and clears it on 0.
docket_group_alive_and_ours(){  # $1 = pgid, $2 = expected identity token
  local pgid="${1:-}" want="${2:-}" have
  DOCKET_LIVENESS_WHY=""

  # 1. THE SYNTACTIC FLOOR, and it comes FIRST so that a record naming nothing probes nothing.
  #    `kill … -0` means THIS caller's own process group and `kill … -1` means every process the
  #    user can signal — as a probe each answers for a bystander, and at the signalling call sites
  #    downstream each would take the caller (or the machine) down with it. Neither may ever stand
  #    in for a recorded run's group, so both are refused here rather than at each call site.
  case "$pgid" in
    ''|*[!0-9]*)
      DOCKET_LIVENESS_WHY="the record names no usable process group (got '${pgid}')"
      return 1 ;;
  esac
  if [ "$pgid" -le 1 ]; then
    DOCKET_LIVENESS_WHY="process group '$pgid' is not a recorded run's group ('0' means this process's own group and '1' means every process this user can signal)"
    return 1
  fi

  # 2. LIVENESS. Cheap, and answers for whoever holds the name NOW — which is why it is never the
  #    last word.
  kill -0 -"$pgid" 2>/dev/null || {
    DOCKET_LIVENESS_WHY="process group $pgid is gone"
    return 1
  }

  # 3. IDENTITY. Fails CLOSED on either token being empty: nothing to compare is not agreement.
  [ -n "$want" ] || {
    DOCKET_LIVENESS_WHY="the record carries no identity token for group $pgid, so the live group cannot be proven to be its own"
    return 1
  }
  have="$(docket_identity_of "$pgid")"
  [ -n "$have" ] || {
    DOCKET_LIVENESS_WHY="group $pgid has no readable start time, so it cannot be proven to be the recorded one"
    return 1
  }
  [ "$have" = "$want" ] || {
    DOCKET_LIVENESS_WHY="group $pgid started at '$have', not at the recorded '$want' — the id was recycled"
    return 1
  }
  return 0
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `bash tests/test_docket_liveness.sh`
Expected: PASS — every line `ok - …`, exit 0.

- [ ] **Step 5: Mutation-test the identity conjunct — deletion AND inversion**

Both probes are required (learnings: a comparison operator needs both). **Confirm each edit changed bytes before believing its result.**

```bash
cd "$(git rev-parse --show-toplevel)"
L=scripts/lib/docket-liveness.sh
cp "$L" /tmp/liveness.orig

# --- probe A: DELETE the identity comparison (the conjunct is dropped entirely) ---
before=$(grep -c 'not at the recorded' "$L")
perl -0pi -e 's/  \[ "\$have" = "\$want" \] \|\| \{\n.*?\n    return 1\n  \}\n//s' "$L"
after=$(grep -c 'not at the recorded' "$L")
echo "landing check: $before -> $after"   # MUST be 1 -> 0; anything else means the probe did not apply
[ "$before" = 1 ] && [ "$after" = 0 ] || { echo "MUTATION DID NOT LAND"; cp /tmp/liveness.orig "$L"; exit 1; }
bash tests/test_docket_liveness.sh; echo "probe A rc=$?"   # MUST be non-zero (the mismatched-token case reddens)
cp /tmp/liveness.orig "$L"

# --- probe B: INVERT the comparison (`=` becomes `!=`) ---
before=$(grep -c '\[ "\$have" = "\$want" \]' "$L")
perl -pi -e 's/\[ "\$have" = "\$want" \]/[ "\$have" != "\$want" ]/' "$L"
after=$(grep -c '\[ "\$have" != "\$want" \]' "$L")
echo "landing check: $before -> $after"   # MUST be 1 -> 1 with the ORIGINAL spelling now absent
[ "$before" = 1 ] && [ "$after" = 1 ] && [ "$(grep -c '\[ "\$have" = "\$want" \]' "$L")" = 0 ] \
  || { echo "MUTATION DID NOT LAND"; cp /tmp/liveness.orig "$L"; exit 1; }
bash tests/test_docket_liveness.sh; echo "probe B rc=$?"   # MUST be non-zero (the MATCHING-token case reddens)
cp /tmp/liveness.orig "$L"

# --- probe C: DELETE the syntactic floor's `-le 1` refusal ---
before=$(grep -c 'is not a recorded run.s group' "$L")
perl -0pi -e 's/  if \[ "\$pgid" -le 1 \]; then\n.*?\n    return 1\n  fi\n//s' "$L"
after=$(grep -c 'is not a recorded run.s group' "$L")
echo "landing check: $before -> $after"   # MUST be 1 -> 0
[ "$before" = 1 ] && [ "$after" = 0 ] || { echo "MUTATION DID NOT LAND"; cp /tmp/liveness.orig "$L"; exit 1; }
bash tests/test_docket_liveness.sh; echo "probe C rc=$?"   # MUST be non-zero (the '0'/'1' no-kill cases redden)
cp /tmp/liveness.orig "$L"
git diff --stat -- "$L"    # MUST be empty: the lib is byte-restored
```

**If any probe's landing check does not print the stated transition, the probe is wrong, not the guard** — fix the probe (these are hard-wrapped shell blocks; a single-line pattern can silently no-op) and re-run. **If a probe lands and the test still passes, that is a finding about the test** — the assert is not reaching the conjunct; fix the test before continuing.

- [ ] **Step 6: Budget the new file**

Measure it (never with `run-tests.sh --timings` — issue #0290 truncates the target file):

```bash
cd "$(git rev-parse --show-toplevel)"
for i in 1 2 3; do /usr/bin/time -p bash tests/test_docket_liveness.sh >/dev/null 2>/tmp/t.$i; grep '^real' /tmp/t.$i; done
```

Take the **slowest** of the three, round up to the next multiple of 5, add 5 (minimum 10) — the table's seeding rule, stated in its own header. Insert the row in the file's existing **alphabetical** position:

```
tests/test_docket_liveness.sh	<N>	parallel
```

Then re-seed `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh:28`, adding `<N>` to `1680`, and amend the trailing comment to say which of the two legitimate cases this was:

```bash
EXPECTED_TOTAL=<1680+N> # the sum of every ceiling; +<N> for tests/test_docket_liveness.sh, a NEW test file bringing its own row (change 0284).
```

- [ ] **Step 7: Verify the budget guard is green**

Run: `bash tests/test_runtime_budgets.sh`
Expected: PASS, including `the table's budgeted total is <1680+N> seconds`.

- [ ] **Step 8: Commit**

```bash
git add scripts/lib/docket-liveness.sh tests/test_docket_liveness.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "feat(0284): scripts/lib/docket-liveness.sh — one identity-checked liveness predicate"
```

---

### Task 2: Refactor `gate-run.sh` onto the lib

**Files:**
- Modify: `scripts/gate-run.sh` (the `identity_of` definition ~line 40; the identity/liveness block ~lines 473–498; add one source line near `SELF=` at line 22)
- Test: `tests/test_gate_run.sh`, `tests/test_gate_run_stop.sh` — **both unchanged; they ARE the gate for this task.**

**Interfaces:**
- Consumes: `docket_identity_of`, `docket_group_alive_and_ours`, `DOCKET_LIVENESS_WHY` from Task 1.
- Produces: nothing new. `identity_of`, `identity_matches`, `group_alive_and_ours` keep their exact existing signatures and semantics for the rest of `gate-run.sh`.

- [ ] **Step 1: Record the pre-refactor baseline**

This task's only success criterion is "these two files still pass, unedited". Prove they pass **before** you touch anything, so a pre-existing failure is never mistaken for your regression.

```bash
cd "$(git rev-parse --show-toplevel)"
bash tests/test_gate_run.sh > /tmp/gr.before 2>&1; echo "before rc=$?"
bash tests/test_gate_run_stop.sh > /tmp/grs.before 2>&1; echo "before rc=$?"
```
Expected: both `rc=0`. If either is already red, STOP and report — do not build on a red base.

- [ ] **Step 2: Source the lib**

`gate-run.sh` sources nothing today, but it already resolves its own absolute path at line 22 (`SELF=`), which the detached wrapper re-execs. Add the source immediately after that line so the wrapper subprocess gets it too:

```bash
SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")"
# The liveness predicate is shared with runner-dispatch.sh (change 0284) — one definition, so the
# observe side and the signal side can never drift into two different notions of "ours". Sourced
# through $SELF rather than $0: --__wrap re-execs this file detached, and the wrapper needs it too.
# shellcheck source=lib/docket-liveness.sh
. "$(dirname "$SELF")/lib/docket-liveness.sh"
```

- [ ] **Step 3: Replace the three functions**

Replace `identity_of` (at ~line 40, keeping its surrounding header comment's still-true half) with a delegation. **Do not delete the name** — `tests/test_gate_run.sh:200` pins `SPAWN_IDENT="$(identity_of "$SPAWN_PID")"` and that file may not be edited (Global Constraint 2).

```bash
# The identity token, from scripts/lib/docket-liveness.sh (change 0284). The NAME survives the
# extraction deliberately: this file's call sites are pinned by source-shape asserts in
# tests/test_gate_run.sh, and that file is this refactor's own behaviour-preserving gate — editing
# it to chase a rename would remove the evidence the refactor was safe. What moved out is the
# PREDICATE; what stayed is the spelling.
identity_of() {  # $1 = pid -> normalized start-time token, empty when the pid is gone
  docket_identity_of "${1:-}"
}
```

Replace `identity_matches` (~line 476) with the layout adapter — it reads *gate-run's* record layout and then compares, and it deliberately does **not** call `docket_group_alive_and_ours` (Global Constraint 3: that would add a `kill -0` conjunct to `stop_run`'s pre-signal re-check, a re-specification):

```bash
# IDENTITY, ON ITS OWN — the one rule, shared by every caller that needs it, so the observe side and
# the signal side can never drift into two different notions of "ours". Fails CLOSED on either token
# being empty: nothing to compare is not agreement.
#
# NOT `docket_group_alive_and_ours`, deliberately: this is asked at points where liveness has
# ALREADY been established separately (`stop_run` probes `kill -0` itself, then re-asks identity
# alone immediately before signalling). Folding a liveness conjunct in here would change what those
# call sites test, and this change refactors gate-run.sh — it never re-specifies it.
identity_matches() {  # $1 = run dir, $2 = pgid
  local want have
  want="$(recorded_identity "$1")"
  [ -n "$want" ] || return 1
  have="$(docket_identity_of "$2")"
  [ -n "$have" ] || return 1
  [ "$have" = "$want" ]
}
```

Replace `group_alive_and_ours` (~line 492) with the lib call, its two layout readers supplying the arguments. This is the collapse the spec's §1 specifies, and no test pins its internals:

```bash
# LIVENESS, IDENTITY-CHECKED — never a bare `kill -0` (spec assumption 9). The predicate itself now
# lives in scripts/lib/docket-liveness.sh, shared with runner-dispatch.sh; this file keeps only its
# own record readers, which is the split that let one predicate serve two incompatible layouts.
#
# The lib's `0`/`1` refusal is a NO-OP here — `recorded_pgid` already refuses anything not `> 1`
# before this call. It is carried in the lib for runner-dispatch.sh, whose pgid is a raw record read
# with no such filter. Stating it so a reader does not mistake the lib call for a behaviour change.
group_alive_and_ours() {  # $1 = run dir
  docket_group_alive_and_ours "$(recorded_pgid "$1")" "$(recorded_identity "$1")"
}
```

- [ ] **Step 4: Verify both gate-run suites still pass, and that neither file was edited**

```bash
cd "$(git rev-parse --show-toplevel)"
bash tests/test_gate_run.sh; echo "rc=$?"
bash tests/test_gate_run_stop.sh; echo "rc=$?"
git status --porcelain -- tests/test_gate_run.sh tests/test_gate_run_stop.sh
```
Expected: both `rc=0`, and the `git status` output is **empty**. A non-empty status here is a task failure regardless of the test results (Global Constraint 1).

- [ ] **Step 5: Prove the refactor is load-bearing, not cosmetic**

A delegation that is never reached would also pass. Confirm the lib is actually the code path:

```bash
cd "$(git rev-parse --show-toplevel)"
cp scripts/lib/docket-liveness.sh /tmp/liveness.orig
before=$(grep -c 'not at the recorded' scripts/lib/docket-liveness.sh)
perl -pi -e 's/\[ "\$have" = "\$want" \]/[ "\$have" != "\$want" ]/' scripts/lib/docket-liveness.sh
after=$(grep -c '\[ "\$have" != "\$want" \]' scripts/lib/docket-liveness.sh)
echo "landing check: identity-compare inverted, now-inverted count=$after (expect 1)"
[ "$after" = 1 ] || { echo "MUTATION DID NOT LAND"; cp /tmp/liveness.orig scripts/lib/docket-liveness.sh; exit 1; }
bash tests/test_gate_run.sh >/dev/null 2>&1; echo "gate_run under an inverted lib: rc=$? (MUST be non-zero)"
cp /tmp/liveness.orig scripts/lib/docket-liveness.sh
git diff --stat -- scripts/lib/docket-liveness.sh   # MUST be empty
```

If `tests/test_gate_run.sh` stays green under the inverted lib, `gate-run.sh` is not reaching the lib — investigate before continuing; do not record it as a residual (learnings: `residual-is-for-undetectable-not-unprobed`).

- [ ] **Step 6: Commit**

```bash
git add scripts/gate-run.sh
git commit -m "refactor(0284): gate-run.sh onto scripts/lib/docket-liveness.sh — behaviour-preserving"
```

---

### Task 3: Refactor `runner-dispatch.sh` onto the lib, and add the test-only barrier hook

**Files:**
- Modify: `scripts/runner-dispatch.sh` — delete `ps_lstart` (~lines 76–82) and its two call sites (`CHILD_LSTART=` ~line 535, `now_lstart=` ~line 885); replace `terminate_dispatch`'s conjunct ladder (~lines 875–896); add the source line near line 59; add `barrier`.
- Test: `tests/test_runner_dispatch_observe.sh`, `tests/test_runner_dispatch_detach.sh`, `tests/test_runner_dispatch_build_gate.sh`, `tests/test_runner_dispatch.sh` — all existing; they are this task's gate.

**Interfaces:**
- Consumes: `docket_identity_of`, `docket_group_alive_and_ours`, `DOCKET_LIVENESS_WHY` from Task 1.
- Produces, relied on by Tasks 4–5:
  - `barrier <point-name>` — a test-only two-way rendezvous inside `runner-dispatch.sh`. Inert unless `RUNNER_DISPATCH_TEST_BARRIER` equals `<point-name>`; when armed it creates `$RUNNER_DISPATCH_TEST_BARRIER_FILE.reached`, then waits (bounded, 30s) for `$RUNNER_DISPATCH_TEST_BARRIER_FILE.release`. Always returns 0.

- [ ] **Step 1: Baseline the four existing shards**

```bash
cd "$(git rev-parse --show-toplevel)"
for f in tests/test_runner_dispatch.sh tests/test_runner_dispatch_detach.sh \
         tests/test_runner_dispatch_observe.sh tests/test_runner_dispatch_build_gate.sh; do
  bash "$f" >/dev/null 2>&1; echo "$f rc=$?"
done
```
Expected: all `rc=0`. A red baseline STOPs the task.

- [ ] **Step 2: Source the lib and delete `ps_lstart`**

Add after the existing `docket-agent-scope.sh` source (line 65):

```bash
# The liveness predicate, shared with gate-run.sh (change 0284). Before this lib, the identity
# conjuncts below existed here as an inline ladder and in gate-run.sh as its own copy, and the two
# had already diverged: on an EMPTY recorded token this file SKIPPED the conjunct while gate-run.sh
# failed closed. The lib is fail-closed, and this file inherits that — see the observe verb's
# header for why the change is behaviour-preserving on every reachable input.
# shellcheck source=lib/docket-liveness.sh
. "$SELF_DIR/lib/docket-liveness.sh"
```

Delete the whole `ps_lstart` function (lines 76–82) **and the header comment paragraph above it that describes the predicate** (lines ~70–75) — that reasoning now lives in the lib and duplicating it is what this task removes. Replace both call sites:

- `CHILD_LSTART="$(ps_lstart "$CHILD_PID")"` → `CHILD_LSTART="$(docket_identity_of "$CHILD_PID")"`
- the `now_lstart="$(ps_lstart "$lchild")"` line disappears entirely with the ladder in Step 3.

- [ ] **Step 3: Replace `terminate_dispatch`'s identity ladder with one lib call**

Replace the block from `local kill_reason="$cause" signal_group=0 …` through the closing `fi` of the `if [ -z "$LPGID" ]` / `else` construct (~lines 875–896) with:

```bash
    # Every one of these is initialized: `local x` leaves x UNSET, and this script runs under
    # `set -u`, so a later read on a path that skipped the assignment would abort the observation.
    local kill_reason="$cause" signal_group=0 identity_why="" lchild="" now_pgid=""
    lchild="$(launch_field "$DDIR" child_pid)"
    if [ -z "$LPGID" ]; then
      identity_why="the launch record names no process group"
    else
      case "$lchild" in
        ''|*[!0-9]*) identity_why="the launch record names no usable child pid" ;;
        *)
          # CONJUNCT 1, and it stays HERE rather than moving into the lib: it asks whether the
          # recorded CHILD still leads the recorded GROUP, which is a question about THIS file's
          # record layout (`child_pid` + `pgid`), not about liveness. The lib takes values; the
          # layout stays with its owner.
          now_pgid="$(ps -o pgid= -p "$lchild" 2>/dev/null | tr -d ' ')"
          if [ -z "$now_pgid" ]; then
            identity_why="the launched child (pid $lchild) is gone, so the group is no longer provably its own"
          elif [ "$now_pgid" != "$LPGID" ]; then
            identity_why="pid $lchild now leads group $now_pgid, not the recorded $LPGID"
          # CONJUNCT 2, now the shared predicate. It carries the reason in DOCKET_LIVENESS_WHY,
          # which is what let this call site drop its private copy of the wording rather than keep
          # a second predicate wearing the first one's answer.
          elif ! docket_group_alive_and_ours "$LPGID" "$(launch_field "$DDIR" child_lstart)"; then
            identity_why="$DOCKET_LIVENESS_WHY"
          else
            signal_group=1
          fi ;;
      esac
    fi
```

**Note the one intentional behaviour change**, recorded in the spec's assumption 2: this file previously *skipped* the token conjunct when `child_lstart` was empty, degrading to the pgid check alone; the lib fails closed. It is behaviour-preserving on every **reachable** input — `--launch` records an empty `child_lstart` only when `ps` saw no process, i.e. the child had already finished, in which case the wrapper writes `done` and the sentinel read disposes before this path is reached. Amend the surrounding contract comment's sentence "The token conjunct is skipped only when the launch recorded none." to say the opposite, and say why (see Task 6 for the `.md` half).

- [ ] **Step 4: Add the barrier hook**

Add beside `launch_field`, before the `--observe` verb block. **Deliberately the same shape as `gate-run.sh`'s `barrier`, not a new invention** — env-gated on a point NAME, inert by default, bounded even when armed, two-way rendezvous.

```bash
# Test-only synchronization point, the same shape as gate-run.sh's `barrier` (change 0284). A
# TWO-WAY RENDEZVOUS: it announces its arrival (`<file>.reached`) so a fixture knows the process is
# held at exactly this point and nowhere else, then waits to be let go (`<file>.release`). That
# handshake is what makes an INTERLEAVING deterministic instead of a sleep-tuned guess — and the
# observe verb's step-1/step-3 window cannot be entered any other way.
#
# ENV-GATED AND INERT BY DEFAULT, and it is the POINT variable that arms it: with
# RUNNER_DISPATCH_TEST_BARRIER unset this is a no-op at full speed no matter what else is in the
# environment, so the hook can never itself become a hang site in production. The match is on the
# point NAME, so arming one rendezvous cannot silently hold every other call site as well.
#
# BOUNDED even when armed: a fixture that forgets to release must fail its own bounded wait and
# leave a red assert behind, never hang the suite.
barrier(){  # $1 = the point this call site names
  [ "${RUNNER_DISPATCH_TEST_BARRIER:-}" = "$1" ] || return 0
  local f="${RUNNER_DISPATCH_TEST_BARRIER_FILE:?barrier point '$1' armed without RUNNER_DISPATCH_TEST_BARRIER_FILE}"
  : >"$f.reached"
  local waited=0
  while [ ! -e "$f.release" ] && [ "$waited" -lt 300 ]; do
    sleep 0.1; waited=$(( waited + 1 ))
  done
  return 0
}
```

- [ ] **Step 5: Verify all four shards still pass**

```bash
cd "$(git rev-parse --show-toplevel)"
for f in tests/test_runner_dispatch.sh tests/test_runner_dispatch_detach.sh \
         tests/test_runner_dispatch_observe.sh tests/test_runner_dispatch_build_gate.sh; do
  bash "$f" >/dev/null 2>&1; echo "$f rc=$?"
done
```
Expected: all `rc=0`.

- [ ] **Step 6: Prove the barrier is inert by default**

An always-on hang site in production is the one way this hook can do damage, so assert the default explicitly rather than inferring it from the green suite:

```bash
cd "$(git rev-parse --show-toplevel)"
# Unset: must not touch the filesystem and must not block.
env -u RUNNER_DISPATCH_TEST_BARRIER -u RUNNER_DISPATCH_TEST_BARRIER_FILE \
  bash -c '. scripts/runner-dispatch.sh 2>/dev/null; true' >/dev/null 2>&1
# Armed on a DIFFERENT point name: still inert (the match is on the name).
echo "inert-by-default is asserted for real in Task 4's test file; this is the smoke check"
```
The real assert lands in Task 4 (`arming a different point name does not hold this one`). This step is the smoke check only.

- [ ] **Step 7: Commit**

```bash
git add scripts/runner-dispatch.sh
git commit -m "refactor(0284): runner-dispatch.sh onto docket-liveness.sh; add the test-only barrier hook"
```

---

### Task 4: The `--observe` liveness leg, the step-4 re-read, and the `child-vanished` marker

**Files:**
- Modify: `scripts/runner-dispatch.sh` — insert the new leg between the `killed`-marker read (~line 984) and the clock reads (~line 989); add the `child-vanished` arm to the step-2 `killed` reader (~line 973).
- Modify: `tests/test_runner_dispatch_observe.sh` — append the arms for spec § Testing cases 1, 2, 3, 6, 7.

**Interfaces:**
- Consumes: `docket_group_alive_and_ours` / `DOCKET_LIVENESS_WHY` (Task 1), `barrier` (Task 3), and the existing `report_done_disposition`, `relay_child_stdout`, `launch_field`, `killed_field`.
- Produces, relied on by Task 5:
  - `dispose_vanished_child()` — called with no arguments on the dead leg. Relays the child's stdout, records the terminal marker (`cause=child-vanished`, `reason=group-already-gone`), and exits. **Never returns.** Task 5 fills in its git-verdict body; this task ships it with the `unavailable`-only body.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_runner_dispatch_observe.sh`. These are cases 1, 2, 3, 6 and 7 of the spec's § Testing; case 4 and 5 (the git dispositions) land in Task 5.

```bash
# ---- 0284: a child that DIED without a sentinel is detected on THIS observation ----
# THE HEADLINE. Before change 0284 the predicate was "no sentinel ⇒ still running", so this exact
# fixture returned 4 on every pass until the 60-minute budget expired. The assert is therefore a
# pair: not-4, AND in seconds rather than in budget-minutes.
make_fixture
FAKE_SLEEP=300 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
lpgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
assert "0284: fixture sanity — the launch recorded a usable pgid" '[ -n "$lpgid" ] && [ "$lpgid" -gt 1 ]'
assert "0284: fixture sanity — the child is alive before we kill it" 'kill -0 -"$lpgid" 2>/dev/null'
# Kill the GROUP without letting the wrapper write `done`: SIGKILL is untrappable, so the untrapped
# wrapper subshell dies with it and no sentinel can ever appear. That is precisely the state the
# sentinel-only predicate could not see.
kill -KILL -"$lpgid" 2>/dev/null
waited=0
while kill -0 -"$lpgid" 2>/dev/null && [ "$waited" -lt 100 ]; do sleep 0.1; waited=$(( waited + 1 )); done
assert "0284: fixture sanity — the group is gone" '! kill -0 -"$lpgid" 2>/dev/null'
assert "0284: fixture sanity — and no sentinel was ever written" '[ ! -f "$DDIR/done" ]'
vstart=$(date +%s)
BUDGET=60 out="$(observe "$KEY" 2>&1)"; rc=$?
velapsed=$(( $(date +%s) - vstart ))
assert "0284: a vanished child does NOT observe as still running" '[ "$rc" != "4" ]'
assert "0284: it is terminal — result unavailable (1)" '[ "$rc" = "1" ]'
assert "0284: and it was detected in seconds, not at the far end of the 60m budget" '[ "$velapsed" -lt 30 ]'
assert "0284: the diagnostic says the child died without a sentinel" \
  'grep -qi "without .*sentinel\|vanished" <<<"$out"'
assert "0284: and it names the dispatch dir so the orphans it did not reap can be found" \
  'grep -qF "$DDIR" <<<"$out"'
# NO FABRICATED EXIT CODE (spec § Testing case 5, shape-keyed rather than wording-enumerated): the
# child said nothing at all, so an "exited <n>" phrase would assert a code that was never read.
assert "0284: the dead path never claims an exit code it did not read" \
  '! grep -qE "exited [0-9]+" <<<"$out"'

# ---- 0284: the terminal marker is the EXISTING one, with the new cause -------------
assert "0284: a killed marker was recorded (no second terminal file was minted)" '[ -f "$DDIR/killed" ]'
assert "0284: its cause is child-vanished" 'grep -qx "cause=child-vanished" "$DDIR/killed"'
assert "0284: its reason says nothing was signalled" 'grep -qx "reason=group-already-gone" "$DDIR/killed"'

# ---- 0284: idempotence — the second observation short-circuits at step 2 -----------
out2="$(observe "$KEY" 2>&1)"; rc2=$?
assert "0284: re-observing a vanished dispatch is identical in code" '[ "$rc2" = "$rc" ]'
assert "0284: re-observing a vanished dispatch is identical in output" '[ "$out2" = "$out" ]'

# ---- 0284: NOTHING IS SIGNALLED on the dead path ----------------------------------
# The orphan residual is a PROMISE, so it needs a discriminating fixture: a surviving process that
# would die if the facade signalled the recorded group. The canary is spawned into the DEAD leader's
# group id — it cannot be, since that group is gone — so instead the fixture is the inverse: a live
# process whose group id the launch record is REWRITTEN to name. Rewriting the record is legitimate
# here: it is exactly the "a pgid is a reusable name" state the identity conjunct exists to catch.
make_fixture
FAKE_SLEEP=300 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
lpgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
kill -KILL -"$lpgid" 2>/dev/null
waited=0
while kill -0 -"$lpgid" 2>/dev/null && [ "$waited" -lt 100 ]; do sleep 0.1; waited=$(( waited + 1 )); done
# The canary leads its OWN group. The marker is the executed program's own argument, never a `-c`
# comment: a single simple command under `sh -c` is EXEC'd and the comment vanishes from argv
# (learnings: exec-optimization-erases-the-process-marker).
( set -m; sleep 60 & echo $! > "$SBX/canary.pgid" ) 
canary="$(cat "$SBX/canary.pgid")"
assert "0284: fixture sanity — the canary leads its own group" \
  '[ "$(ps -o pgid= -p "$canary" 2>/dev/null | tr -d " ")" = "$canary" ]'
# Point the DEAD dispatch's record at the LIVE canary group — the pid-reuse shape exactly.
sed -i.bak "s/^pgid=.*/pgid=$canary/" "$DDIR/launch" && rm -f "$DDIR/launch.bak"
assert "0284: fixture sanity — the record now names the canary's group" \
  '[ "$(sed -n "s/^pgid=//p" "$DDIR/launch")" = "$canary" ]'
BUDGET=60 out="$(observe "$KEY" 2>&1)"; rc=$?
assert "0284: a record naming a group that is not ours is still terminal (1)" '[ "$rc" = "1" ]'
assert "0284: NOTHING was signalled — the canary is still running" 'kill -0 -"$canary" 2>/dev/null'
reap "$canary"

# ---- 0284: the SENTINEL OUTRANKS liveness -----------------------------------------
# Pins step 1's precedence against the new step 3: a dead child WITH a `done` sentinel takes the
# sentinel disposition, unchanged. Without this the new leg could silently capture completed runs.
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
waited=0
while [ ! -f "$DDIR/done" ] && [ "$waited" -lt 100 ]; do sleep 0.1; waited=$(( waited + 1 )); done
assert "0284: fixture sanity — the child finished and wrote its sentinel" '[ -f "$DDIR/done" ]'
lpgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
assert "0284: fixture sanity — and its group is gone (so liveness alone would say vanished)" \
  '! kill -0 -"$lpgid" 2>/dev/null'
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "0284: a dead child WITH a sentinel takes the sentinel disposition (0)" '[ "$rc" = "0" ]'
assert "0284: and it is worded as a completion, not as a vanishing" \
  'grep -qi "complete" <<<"$out" && ! grep -qi "vanished" <<<"$out"'
assert "0284: no killed marker is written over a completed run" '[ ! -f "$DDIR/killed" ]'

# ---- 0284: the step-4 RE-READ closes the probe's own TOCTOU window ----------------
# Steps 1-3 span a `ps` call and a `kill -0`; the child has every chance to finish inside that
# window, and without the re-read a run that PASSED is disposed as dead. The barrier holds the
# observer at exactly the pre-probe point so the interleaving is deterministic rather than
# sleep-tuned. Inert unless armed, so no other arm in this file is affected.
make_fixture
FAKE_SLEEP=300 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
lpgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
kill -KILL -"$lpgid" 2>/dev/null
waited=0
while kill -0 -"$lpgid" 2>/dev/null && [ "$waited" -lt 100 ]; do sleep 0.1; waited=$(( waited + 1 )); done
BFILE="$SBX/barrier"
( cd "$SBX" && RUNNERS_DIR="$RDIR" DELEGATION_OBSERVATION_BUDGET=60 \
    RUNNER_DISPATCH_TEST_BARRIER=pre-liveness-probe \
    RUNNER_DISPATCH_TEST_BARRIER_FILE="$BFILE" \
    bash "$FACADE" --observe "$KEY" --runner fake --agent status ) > "$SBX/bout" 2> "$SBX/berr" &
bpid=$!
waited=0
while [ ! -e "$BFILE.reached" ] && [ "$waited" -lt 200 ]; do sleep 0.1; waited=$(( waited + 1 )); done
assert "0284: fixture sanity — the observer is held at the pre-probe barrier" '[ -e "$BFILE.reached" ]'
# The child "finishes" INSIDE the window: write the sentinel the wrapper would have written.
printf 'exit_code=0\n' > "$DDIR/done.partial" && mv -f "$DDIR/done.partial" "$DDIR/done"
: > "$BFILE.release"
wait "$bpid"; rc=$?
berr="$(cat "$SBX/berr")"
assert "0284: a sentinel that lands inside the probe window is honoured, not disposed as dead" '[ "$rc" = "0" ]'
assert "0284: and it reports the COMPLETED disposition, never child-vanished" \
  'grep -qi "complete" <<<"$berr" && ! grep -qi "vanished" <<<"$berr"'
assert "0284: no killed marker was written over the sentinel that arrived" '[ ! -f "$DDIR/killed" ]'

# ---- 0284: the barrier is inert unless armed on ITS OWN point name ----------------
# An always-on hang site is the one way a test hook can damage production, and arming point A must
# never hold point B. Both are asserted by a timed observation that must NOT stall.
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
istart=$(date +%s)
( cd "$SBX" && RUNNERS_DIR="$RDIR" DELEGATION_OBSERVATION_BUDGET=60 \
    RUNNER_DISPATCH_TEST_BARRIER=some-other-point \
    RUNNER_DISPATCH_TEST_BARRIER_FILE="$SBX/never" \
    bash "$FACADE" --observe "$KEY" --runner fake --agent status ) >/dev/null 2>&1
assert "0284: arming a DIFFERENT barrier point does not hold this one" \
  '[ $(( $(date +%s) - istart )) -lt 10 ]'
assert "0284: and an unarmed barrier creates no rendezvous files" '[ ! -e "$SBX/never.reached" ]'
```

- [ ] **Step 2: Run them to verify they fail**

Run: `bash tests/test_runner_dispatch_observe.sh`
Expected: FAIL — the headline arm reports `rc=4` (still running) and `velapsed` is small but the not-4 assert reddens; the `cause=child-vanished` and barrier arms redden too.

- [ ] **Step 3: Add the `child-vanished` arm to the step-2 `killed` reader**

In the `if [ -f "$DDIR/killed" ]; then` block, extend the `KCAUSE` case (~line 973):

```bash
    case "$KCAUSE" in
      child-vanished)       KWHAT="the delegated child died without writing a sentinel" ;;
      budget-unenforceable) KWHAT="the observation budget could not be enforced (${KDETAIL:-reason unrecorded})" ;;
      *)                    KWHAT="the budget was exhausted" ;;
    esac
```

- [ ] **Step 4: Insert the liveness leg and the disposer**

Insert immediately **after** the `killed`-marker block's closing `fi` (~line 984) and **before** the `NOW="$(date -u +%s …)"` clock read (~line 989):

```bash
  # 3. LIVENESS — new in change 0284, and it sits HERE for a reason: AFTER both terminal-state reads
  #    and BEFORE the clock arithmetic.
  #
  #    AFTER the terminal reads, because a terminal record outranks any probe of the group the child
  #    used to lead. A `done` sitting beside a dead group means the child COMPLETED and then exited,
  #    which is the ordinary happy path, not a vanishing.
  #
  #    BEFORE the clock arithmetic, because DEADNESS IS KNOWABLE WITHOUT A READABLE CLOCK. Placed
  #    after, an unreadable clock or an unparseable `started_at` — the `note_unenforceable` family —
  #    would keep a dead child spinning for three more observations and then terminate it on the
  #    WRONG cause. The `unenforceable` counter is untouched by this leg: it is reset or incremented
  #    only on the still-running path, which this step now guards.
  #
  #    THE DEFECT THIS REMOVES: the predicate here used to be "no sentinel ⇒ still running", so a
  #    child killed externally, crashed, or whose host was suspended read `still running` (exit 4) on
  #    every observation for the WHOLE budget — 60 minutes by default. The identity conjuncts already
  #    existed in this file, inside `terminate_dispatch`, but were consulted only when the facade was
  #    about to SIGNAL; this consults them one lifecycle phase earlier, as a VERDICT INPUT.
  barrier pre-liveness-probe
  if ! docket_group_alive_and_ours "$LPGID" "$(launch_field "$DDIR" child_lstart)"; then
    VANISHED_WHY="$DOCKET_LIVENESS_WHY"
    # 4. DEAD ⇒ RE-READ THE SENTINEL. LOAD-BEARING, NOT DEFENSIVE. Steps 1-3 span a `ps` call and a
    #    `kill -0`, and the child has every chance to finish inside that window; without this re-read
    #    a run that PASSED is disposed as dead. The soundness argument is the one already written at
    #    this file's pair of give-up re-reads: the untrapped wrapper subshell is the ONLY writer of
    #    `done`, so a sentinel visible HERE was necessarily written by a child that completed.
    #    `report_done_disposition` never returns.
    [ -f "$DDIR/done" ] && report_done_disposition
    dispose_vanished_child
  fi
```

Define `dispose_vanished_child` beside `terminate_dispatch`, before the numbered read order. **This task ships the `unavailable`-only body**; Task 5 replaces the marked block with the git verdicts.

```bash
  # THE DEAD-CHILD DISPOSITION (change 0284). Reached only from the liveness leg below, and only
  # after its step-4 sentinel re-read came back empty. NEVER RETURNS.
  #
  # THE VERDICT IS RECORDED IN THE EXISTING `killed` MARKER, not in a new terminal file. `--observe`'s
  # idempotence guarantee obliges a terminal record, and the `cause`/`reason` split already carries
  # exactly the two axes this leg needs: `cause=child-vanished` says WHY the facade gave up, and
  # `reason=group-already-gone` says nothing was signalled. A second terminal file would need its own
  # precedence rule against `done` and `killed`, and would be a third state for every reader to order.
  #
  # NOTHING IS SIGNALLED, and that is not an omission: the liveness probe just established that the
  # group is not provably ours, which is the precondition `terminate_dispatch` already refuses to
  # signal under. THE RESIDUAL THIS ACCEPTS, deliberately and identically to the give-up path: a
  # supervisor that died while processes it spawned keep running is reported dead and those orphans
  # are NOT reaped. Killing them would mean signalling a group that cannot be proven still ours, and
  # an unrelated process group dying is the worse failure and the unrecoverable one, while an orphan
  # is visible and reapable. So the diagnostic NAMES THE DISPATCH DIR, which is how a human finds them.
  dispose_vanished_child(){
    printf 'killed_at=%s\nreason=%s\ncause=%s\ndetail=%s\nbudget_minutes=%s\n' \
      "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "group-already-gone" "child-vanished" "${VANISHED_WHY:-}" \
      "$DELEGATION_OBSERVATION_BUDGET" > "$DDIR/killed.partial"
    # `mv -f`, not `mv`: BSD `mv` onto an unwritable destination with a tty PROMPTS, self-answers `n`
    # at EOF, and exits 0 — the marker would be silently lost and this leg would re-fire every pass.
    mv -f "$DDIR/killed.partial" "$DDIR/killed" || die "could not record the kill marker in $DDIR"
    # ----- TASK 5 REPLACES FROM HERE ---------------------------------------------------
    printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (the delegated child died without writing a sentinel: %s); nothing was signalled, so any processes it spawned are still running — inspect %s\n' \
      "$OBSERVE_KEY" "${VANISHED_WHY:-reason unrecorded}" "$DDIR" >&2
    relay_child_stdout
    exit 1
    # ----- TASK 5 REPLACES TO HERE -----------------------------------------------------
  }
```

Also initialize `VANISHED_WHY=""` beside `LPGID`/`LSTART` (~line 614), because this script runs under `set -u`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_runner_dispatch_observe.sh`
Expected: PASS. Then re-run the three sibling shards to confirm no collateral damage:
```bash
for f in tests/test_runner_dispatch.sh tests/test_runner_dispatch_detach.sh tests/test_runner_dispatch_build_gate.sh; do bash "$f" >/dev/null 2>&1; echo "$f rc=$?"; done
```
Expected: all `rc=0`.

- [ ] **Step 6: Mutation-test the two new guards**

```bash
cd "$(git rev-parse --show-toplevel)"
R=scripts/runner-dispatch.sh
cp "$R" /tmp/rd.orig

# --- probe A: REMOVE the step-3 liveness probe (the headline must redden into exit 4) ---
before=$(grep -c 'barrier pre-liveness-probe' "$R")
perl -0pi -e 's/  barrier pre-liveness-probe\n  if ! docket_group_alive_and_ours.*?\n  fi\n//s' "$R"
after=$(grep -c 'barrier pre-liveness-probe' "$R")
echo "landing check: $before -> $after"    # MUST be 1 -> 0
[ "$before" = 1 ] && [ "$after" = 0 ] || { echo "MUTATION DID NOT LAND"; cp /tmp/rd.orig "$R"; exit 1; }
bash tests/test_runner_dispatch_observe.sh >/dev/null 2>&1; echo "probe A rc=$? (MUST be non-zero)"
cp /tmp/rd.orig "$R"

# --- probe B: DELETE the step-4 sentinel re-read (the TOCTOU arm must redden) ---
before=$(grep -c '\[ -f "\$DDIR/done" \] && report_done_disposition' "$R")
echo "occurrences of the re-read idiom: $before (expect 3 — two in terminate_dispatch, one new)"
# Delete ONLY the new one: it is the line immediately preceding `dispose_vanished_child`.
perl -0pi -e 's/    \[ -f "\$DDIR\/done" \] && report_done_disposition\n    dispose_vanished_child\n/    dispose_vanished_child\n/s' "$R"
after=$(grep -c '\[ -f "\$DDIR/done" \] && report_done_disposition' "$R")
echo "landing check: $before -> $after"    # MUST be 3 -> 2
[ "$after" = $(( before - 1 )) ] || { echo "MUTATION DID NOT LAND"; cp /tmp/rd.orig "$R"; exit 1; }
bash tests/test_runner_dispatch_observe.sh >/dev/null 2>&1; echo "probe B rc=$? (MUST be non-zero)"
cp /tmp/rd.orig "$R"

# --- probe C: MOVE the probe AFTER the clock reads (ordering is a guard too) ---
# Placed after `note_unenforceable`'s clock guards, a dispatch with an unreadable started_at spins
# three more passes before terminating on the wrong cause. Simulate by blanking started_at in the
# headline fixture rather than by moving code — see Task 4 Step 1's arm if you extend this.
echo "probe C is covered by the ordering assert in Task 5 Step 6; no code move needed here"

git diff --stat -- "$R"   # MUST be empty: the script is byte-restored
```

**If a landing check does not print its stated transition, the probe is wrong** — these are hard-wrapped blocks and a single-line pattern silently no-ops. Fix the probe and re-run before drawing any conclusion.

- [ ] **Step 7: Commit**

```bash
git add scripts/runner-dispatch.sh tests/test_runner_dispatch_observe.sh
git commit -m "feat(0284): --observe probes process liveness; a vanished child is terminal on the next pass"
```

---

### Task 5: Git decides the dead child's disposition

**Files:**
- Modify: `scripts/runner-dispatch.sh` — replace the marked block inside `dispose_vanished_child`.
- Modify: `tests/test_runner_dispatch_observe.sh` — append the arms for spec § Testing case 4.

**Interfaces:**
- Consumes: `dispose_vanished_child` (Task 4), and the existing `observe_implement_next`, `launch_field`, `relay_child_stdout`, `ANCHOR_FALLBACK`, `VERIFY_RUN`, `DOCKET_BASH_PATH`.
- Produces: the final dead-path exit-code contract — `0` when git says the work landed, `1` otherwise.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_runner_dispatch_observe.sh`. The `verify-run` results are driven by a **stub** on the facade's own resolution path; check how `tests/test_runner_dispatch_build_gate.sh` already stubs `verify-run` and reuse that mechanism verbatim rather than inventing a second one.

```bash
# ---- 0284 case 4: GIT DECIDES the dead child's disposition ------------------------
# A dead child is NOT automatically "no result": a delegated run can commit its work, push its
# branch and open its PR and THEN be killed before the wrapper's `mv -f` lands. Reporting
# `unavailable` over evidence sitting in git sends a human hunting for work that is already
# committed — change 0258's failure, inverted.
#
# The helper below is the shared shape for the four arms: launch, kill the group without a sentinel,
# stub verify-run's answer, observe once.
vanish_with_verdict(){  # $1 = agent, $2 = the verify-run stdout to stub, $3 = extra launch args
  make_fixture
  FAKE_SLEEP=300 FAKE_TAIL=0 FAKE_RC=0
  KEY="$(launch "$1" ${3:-})"
  DDIR="$(ddir_for "$KEY")"
  local lp; lp="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
  kill -KILL -"$lp" 2>/dev/null
  local w=0
  while kill -0 -"$lp" 2>/dev/null && [ "$w" -lt 100 ]; do sleep 0.1; w=$(( w + 1 )); done
  # Stub verify-run the same way tests/test_runner_dispatch_build_gate.sh does.
  stub_verify_run "$2"
}

# --- implement-next + run-complete => 0 -------------------------------------------
vanish_with_verdict implement-next "run-complete 284"
BUDGET=60 out="$(observe "$KEY" implement-next 2>&1)"; rc=$?
assert "0284: a vanished implement-next whose work LANDED exits 0" '[ "$rc" = "0" ]'
assert "0284: and the wording states the death FIRST, then the git verdict" \
  'grep -qi "died without" <<<"$out" && grep -qF "run-complete" <<<"$out"'
assert "0284: it never claims an exit code it did not read" '! grep -qE "exited [0-9]+" <<<"$out"'

# --- implement-next + run-halted => 1, halted wording preserved --------------------
vanish_with_verdict implement-next "run-halted 284"
BUDGET=60 out="$(observe "$KEY" implement-next 2>&1)"; rc=$?
assert "0284: a vanished implement-next that HALTED exits 1" '[ "$rc" = "1" ]'
assert "0284: and the halted wording is preserved" 'grep -qi "halted" <<<"$out"'

# --- build-* + task-committed => 0 -------------------------------------------------
vanish_with_verdict build-standard "task-committed"
BUDGET=60 out="$(observe "$KEY" build-standard 2>&1)"; rc=$?
assert "0284: a vanished build task whose commit LANDED exits 0" '[ "$rc" = "0" ]'
assert "0284: and echoes the git verdict" 'grep -qF "task-committed" <<<"$out"'

# --- build-* + no evidence => 1 unavailable ---------------------------------------
vanish_with_verdict build-standard "task-incomplete"
BUDGET=60 out="$(observe "$KEY" build-standard 2>&1)"; rc=$?
assert "0284: a vanished build task with NO git evidence exits 1" '[ "$rc" = "1" ]'
assert "0284: and says the work is unavailable" 'grep -qi "unavailable\|no result" <<<"$out"'

# --- any other agent => 1, no git verdict claimed ---------------------------------
vanish_with_verdict status ""
BUDGET=60 out="$(observe "$KEY" status 2>&1)"; rc=$?
assert "0284: a vanished status dispatch exits 1 with no git verdict claimed" \
  '[ "$rc" = "1" ] && ! grep -qF "task-committed" <<<"$out"'

# --- 0208 IS NOT REGRESSED: a removed worktree is an honest non-verdict ------------
# --observe on a removed worktree deliberately reassigns ANCHOR to the repo root (ANCHOR_FALLBACK=1).
# Verifying the build there would answer a question nobody asked, so the sentinel path reports
# `task-unverifiable worktree-removed` — and the dead path must do the same rather than manufacture
# a verdict against the main tree.
vanish_with_verdict build-standard "task-committed"
# Remove the anchor worktree the same way the existing 0271/0208 arms in this file do.
remove_anchor_worktree
BUDGET=60 out="$(observe "$KEY" build-standard 2>&1)"; rc=$?
assert "0284: a vanished build task whose WORKTREE is gone is not verified against the main tree" \
  '[ "$rc" = "1" ]'
assert "0284: and it says so honestly" 'grep -qF "worktree-removed" <<<"$out"'

# --- 0208 IS NOT REGRESSED: a launch record with no branch is a non-verdict --------
vanish_with_verdict build-standard "task-committed"
sed -i.bak 's/^branch=.*/branch=/' "$DDIR/launch" && rm -f "$DDIR/launch.bak"
assert "0284: fixture sanity — the launch record now names no branch" \
  '[ -z "$(sed -n "s/^branch=//p" "$DDIR/launch")" ]'
BUDGET=60 out="$(observe "$KEY" build-standard 2>&1)"; rc=$?
assert "0284: a branchless launch record is not verified against the observation-time branch" \
  '[ "$rc" = "1" ]'
assert "0284: and it says so honestly" 'grep -qF "launch-branch-missing" <<<"$out"'
```

**Before running these, read `tests/test_runner_dispatch_build_gate.sh` and adapt `stub_verify_run` and `remove_anchor_worktree` to whatever that file actually calls them** — those two helpers are named here for the shape, and the real spellings live in that file and in `tests/lib/runner_dispatch_detach_common.sh`. If neither helper exists, write them into `tests/lib/runner_dispatch_detach_common.sh` so both shards share one copy.

- [ ] **Step 2: Run them to verify they fail**

Run: `bash tests/test_runner_dispatch_observe.sh`
Expected: FAIL — every arm that expects `rc=0` gets `1`, because Task 4's body routes the whole dead path to `unavailable`.

- [ ] **Step 3: Replace the marked block**

```bash
    # GIT DECIDES. The disagreement rule already governing the sentinel path — liveness from the
    # process, correctness from git — extends to this leg, and the dispositions are the ones
    # `report_done_disposition` already routes to, reached with NO EXIT CODE TO CONSULT.
    #
    # RE-WORDING IS REQUIRED, NOT OPTIONAL. Both dispositions on the sentinel path narrate a child
    # that "exited 0 but git disagrees". On THIS leg the child said nothing at all, so every message
    # states THE DEATH FIRST and the git verdict second. Asserting an exit code that was never read
    # is the fabricated-verdict failure `classify_record` refuses over a malformed record.
    local died="the delegated child died without writing a sentinel (${VANISHED_WHY:-reason unrecorded})"
    case "$AGENT" in
      implement-next)
        # `observe_implement_next` EXITS on a positive finding and RETURNS otherwise. Its own
        # wording already leads with the verdict rather than with an exit code, so it is reused
        # verbatim — a second copy worded for this leg is the drift this change exists to remove.
        printf 'runner-dispatch: observe %s — %s; reading git for a verdict\n' "$OBSERVE_KEY" "$died" >&2
        observe_implement_next ;;
      build-*)
        local lsince lbranch gitv=""
        lsince="$(launch_field "$DDIR" since_sha)"
        lbranch="$(launch_field "$DDIR" branch)"
        # THE SAME TWO NON-VERDICTS THE SENTINEL PATH TAKES (change 0208, ADR-0083). Neither is a
        # failure of this leg: `--observe` on a removed worktree deliberately reassigns ANCHOR to the
        # repo root, and verifying the build THERE answers a question nobody asked; falling back to
        # the observation-time branch reinstates the vacuity 0271 removed. Both are NO POSITIVE
        # EVIDENCE and are surfaced as such.
        if [ "$ANCHOR_FALLBACK" = 1 ]; then
          gitv="task-unverifiable worktree-removed"
        elif [ -z "$lbranch" ]; then
          gitv="task-unverifiable launch-branch-missing"
        else
          gitv="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" --build --worktree "$ANCHOR" \
                    --branch "$lbranch" --since "${lsince:-}" 2>/dev/null)"
        fi
        case "$gitv" in
          task-committed*)
            printf 'runner-dispatch: observe %s — COMPLETE: %s, but git says %s, so the work landed\n' \
              "$OBSERVE_KEY" "$died" "$gitv" >&2
            relay_child_stdout
            exit 0 ;;
        esac
        printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE: %s, and git has no evidence the work landed (%s); nothing was signalled, so any processes it spawned are still running — inspect %s\n' \
          "$OBSERVE_KEY" "$died" "${gitv:-no-verdict}" "$DDIR" >&2
        relay_child_stdout
        exit 1 ;;
    esac
    # Every other agent: no git verdict is read and none is claimed.
    printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE: %s; nothing was signalled, so any processes it spawned are still running — inspect %s\n' \
      "$OBSERVE_KEY" "$died" "$DDIR" >&2
    relay_child_stdout
    exit 1
```

Note `observe_implement_next` **returns** when it establishes nothing, so control falls through to the final `unavailable` printf — which is the correct disposition for an unarmed or unattributable implement-next dispatch, and matches the sentinel path's structure exactly.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_runner_dispatch_observe.sh`
Expected: PASS. Re-run the sibling shards:
```bash
for f in tests/test_runner_dispatch.sh tests/test_runner_dispatch_detach.sh tests/test_runner_dispatch_build_gate.sh; do bash "$f" >/dev/null 2>&1; echo "$f rc=$?"; done
```
Expected: all `rc=0`.

- [ ] **Step 5: Mutation-test the git disposition**

```bash
cd "$(git rev-parse --show-toplevel)"
R=scripts/runner-dispatch.sh
cp "$R" /tmp/rd.orig

# --- probe A: route the dead path STRAIGHT to unavailable (drop the git read) ---
before=$(grep -c 'reading git for a verdict' "$R")
perl -0pi -e 's/    local died=.*?\n    case "\$AGENT" in\n.*?\n    esac\n/    local died="the delegated child died without writing a sentinel";\n/s' "$R"
after=$(grep -c 'reading git for a verdict' "$R")
echo "landing check: $before -> $after"   # MUST be 1 -> 0
[ "$before" = 1 ] && [ "$after" = 0 ] || { echo "MUTATION DID NOT LAND"; cp /tmp/rd.orig "$R"; exit 1; }
bash tests/test_runner_dispatch_observe.sh >/dev/null 2>&1; echo "probe A rc=$? (MUST be non-zero — the three landed-work arms redden)"
cp /tmp/rd.orig "$R"

# --- probe B: DELETE the ANCHOR_FALLBACK leg (0208 regression guard) ---
before=$(grep -c 'task-unverifiable worktree-removed' "$R")
echo "occurrences: $before (expect 2 — the sentinel path's and this leg's)"
perl -0pi -e 's/        if \[ "\$ANCHOR_FALLBACK" = 1 \]; then\n          gitv="task-unverifiable worktree-removed"\n        elif/        if/s' "$R"
after=$(grep -c 'task-unverifiable worktree-removed' "$R")
echo "landing check: $before -> $after"   # MUST be 2 -> 1
[ "$after" = $(( before - 1 )) ] || { echo "MUTATION DID NOT LAND"; cp /tmp/rd.orig "$R"; exit 1; }
bash tests/test_runner_dispatch_observe.sh >/dev/null 2>&1; echo "probe B rc=$? (MUST be non-zero)"
cp /tmp/rd.orig "$R"

# --- probe C: INVERT the branch-empty test (deletion and inversion are different probes) ---
before=$(grep -c 'elif \[ -z "\$lbranch" \]; then' "$R")
perl -pi -e 's/elif \[ -z "\$lbranch" \]; then/elif [ -n "\$lbranch" ]; then/' "$R"
after=$(grep -c 'elif \[ -n "\$lbranch" \]; then' "$R")
echo "landing check: $before -> $after"   # MUST be 1 -> 1, with the original spelling gone
[ "$before" = 1 ] && [ "$after" = 1 ] && [ "$(grep -c 'elif \[ -z "\$lbranch" \]; then' "$R")" = 0 ] \
  || { echo "MUTATION DID NOT LAND"; cp /tmp/rd.orig "$R"; exit 1; }
bash tests/test_runner_dispatch_observe.sh >/dev/null 2>&1; echo "probe C rc=$? (MUST be non-zero)"
cp /tmp/rd.orig "$R"

git diff --stat -- "$R"   # MUST be empty
```

- [ ] **Step 6: Assert the probe's ORDERING against the clock reads**

The spec's "why the probe precedes the clock reads" is a claim about behaviour, so it gets an assert rather than a comment:

```bash
# ---- 0284: deadness is knowable without a readable clock -------------------------
# Placed AFTER the clock reads, a dispatch with an unreadable `started_at` would take the
# `note_unenforceable` path for three more observations and then terminate on the WRONG cause. So a
# vanished child with a blanked start time must still be disposed as child-vanished, first pass.
```
Append that arm to `tests/test_runner_dispatch_observe.sh`: reuse `vanish_with_verdict status ""`, then `sed -i.bak 's/^started_at=.*/started_at=/' "$DDIR/launch"`, observe once, and assert `rc=1`, `grep -qx "cause=child-vanished" "$DDIR/killed"`, and `[ ! -f "$DDIR/unenforceable" ]` — the counter must never have been touched.

- [ ] **Step 7: Commit**

```bash
git add scripts/runner-dispatch.sh tests/test_runner_dispatch_observe.sh
git commit -m "feat(0284): git decides a vanished child's disposition, honouring 0208's non-verdicts"
```

---

### Task 6: Contract prose, and the budget the new arms cost

**Files:**
- Modify: `scripts/runner-dispatch.md` — the *Liveness vs correctness* section (~lines 489–493) and the give-up path's residual paragraph (~lines 478–487).
- Modify: `tests/runtime-budgets.tsv` — re-measured ceiling for `tests/test_runner_dispatch_observe.sh`.
- Modify: `tests/test_runtime_budgets.sh` — `EXPECTED_TOTAL` if the ceiling moved.

**Interfaces:**
- Consumes: everything from Tasks 1–5.
- Produces: the final contract; nothing downstream.

- [ ] **Step 1: Rewrite the *Liveness vs correctness* section**

The **repealed sentence** is: *"The sentinel is the only source of liveness — the facade never infers 'still running' from git state, and never infers 'finished' from anything but the wrapper's own sentinel."* Replace the paragraph with:

```markdown
**Liveness vs correctness (change 0284).** Three sources, in a fixed order: **the terminal record
first, process liveness second, git last.** A `done` sentinel or a `killed` marker outranks any
probe of the group the child used to lead — the wrapper is the only writer of the sentinel, so a
record that exists describes a child that reached the end. Only when neither exists does the facade
probe **liveness**, through the identity-checked predicate in `scripts/lib/docket-liveness.sh`
shared with `gate-run.sh`: the recorded group must still exist *and* the process leading it must
still have started at the instant `--launch` recorded. Fail-closed on every leg, because a false
*dead* costs one wasted observation while a false *alive* costs the caller its entire budget.

The half of the old rule that still holds, unchanged: **correctness never comes from liveness, and
liveness never comes from git.** A child that is running says nothing about whether its work is
sound, and git state says nothing about whether a process is alive. What changed is only that the
*sentinel* is no longer the sole liveness source — before change 0284 the predicate was "no sentinel
⇒ still running", so a child killed externally, crashed, or whose host was suspended read *still
running* for the whole 60-minute budget.

**`cause=child-vanished`.** A child found dead with no sentinel is terminal on that observation. The
verdict is recorded in the **existing** `killed` marker with `cause=child-vanished` and
`reason=group-already-gone` — no second terminal file, because the `cause`/`reason` split already
carries the two axes (*why the facade gave up*, and *whether anything was signalled*). Nothing is
signalled: the probe has just established the group is not provably ours, which is the precondition
the give-up path already refuses to signal under.

**A dead child is not automatically "no result".** Git decides, on the same disagreement rule as the
sentinel path: a delegated run can commit its work, push its branch and open its PR and *then* be
killed before the wrapper's `mv -f` lands, and reporting *unavailable* over evidence sitting in git
sends a human hunting for work that is already committed. An `implement-next` dispatch takes the run
gate's verdict; a `build-*` dispatch reads `verify-run --build`, and inherits both of that path's
honest non-verdicts — `task-unverifiable worktree-removed` when the anchor worktree is gone, and
`task-unverifiable launch-branch-missing` when the launch record names no branch. Every other agent
is *unavailable* with no git verdict read and none claimed. **No message on this path asserts an
exit code**: the child said nothing at all, and a code that was never read is a fabricated verdict.

**The orphan residual now shapes a verdict, not only a kill decision.** A supervisor that died while
processes it spawned keep running is reported dead and those orphans are **not** reaped — the same
accepted residual the give-up path documents below, for the same reason. The diagnostic names the
dispatch dir so a human can find them.
```

- [ ] **Step 2: Amend the give-up path's residual paragraph**

In the paragraph ending *"…while an orphan is visible and reapable"*, append one sentence:

```markdown
Since change 0284 this same residual also shapes a **verdict** rather than only a kill decision: the
liveness leg reaches it one lifecycle phase earlier, on an observation that has not yet spent its
budget.
```

Also correct the sentence *"The token conjunct is skipped only when the launch recorded none."* — it is now **fail-closed**:

```markdown
A launch record carrying **no** token fails the conjunct closed (change 0284, adopting
`gate-run.sh`'s posture through the shared predicate). That is behaviour-preserving on every
reachable input: `--launch` records an empty `child_lstart` only when `ps` saw no process — i.e. the
child had already finished — in which case the wrapper writes `done` and the sentinel read disposes
before either leg is reached.
```

- [ ] **Step 3: Verify the doc guards**

```bash
cd "$(git rev-parse --show-toplevel)"
bash tests/test_script_contracts.sh 2>/dev/null || echo "(no such file — check tests/README.md for the contract-guard file's real name and run it)"
grep -rn "only source of liveness" scripts/ || echo "ok - the repealed sentence is gone from scripts/"
```
The repealed sentence must survive **only** in point-in-time records (`docs/changes/archive/`, `docs/superpowers/specs/`), which are never rewritten.

- [ ] **Step 4: Re-measure the observe shard and re-budget it**

Task 4 and Task 5 added a dozen arms, several of which spawn real processes and wait on them. The row is at **25s** today. Measure (never `run-tests.sh --timings` — #0290):

```bash
cd "$(git rev-parse --show-toplevel)"
for i in 1 2 3; do /usr/bin/time -p bash tests/test_runner_dispatch_observe.sh >/dev/null 2>/tmp/o.$i; grep '^real' /tmp/o.$i; done
```

Take the slowest, round up to the next multiple of 5, add 5. If that exceeds **25**, raise the row and add the delta to `EXPECTED_TOTAL`, amending its comment to name this case. **If it exceeds 60, do not raise it** — the table forbids any row above 60; shard the file instead, following the seam `tests/lib/runner_dispatch_detach_common.sh`'s header already documents.

**Report the remaining margin as a number** in the results file, never as "did not trip the budget check" (learnings: `budget-headroom-is-spent-before-it-is-breached`). Change 0252 is queued against `scripts/runner-dispatch.sh`, so the next change to touch this family inherits whatever margin you leave.

- [ ] **Step 5: Run the whole suite**

```bash
cd "$(git rev-parse --show-toplevel)"
bash scripts/run-tests.sh
```
Expected: green. A trailing `OVER BUDGET:` line naming any file **this change touched** is a finding to act on — go back to Step 4. `tests/test_sync_agents_runners.sh` over budget is pre-existing (#0280) and is **not** yours.

- [ ] **Step 6: Commit**

```bash
git add scripts/runner-dispatch.md tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "docs(0284): runner-dispatch.md — three liveness sources, cause=child-vanished, the orphan residual"
```

---

## Self-review

**Spec coverage.** §1 lib → Task 1; both refactors → Tasks 2 and 3; §2 the new leg and its ordering → Task 4 (ordering asserted in Task 5 Step 6); §3 the git dispositions and the re-wording → Task 5; §4 the reused marker and the reader arm → Task 4 Steps 3–4; §5 contract prose → Task 6. § Testing cases 1, 2, 3, 6, 7 → Task 4 Step 1; cases 4 and 5 → Task 5 Step 1 and Task 4 Step 1's shape assert. Assumption 2's fail-closed change → Task 3 Step 3 plus Task 6 Step 2. The unchanged-gate-run-tests rule → Task 2 Steps 1, 4 and 5.

**Reconcile findings.** Finding 1 (symbol survival) → Global Constraints 2–3 and Task 2 Step 3. Finding 2 (`ANCHOR_FALLBACK`) → Global Constraint 4, Task 5 Step 3, and two dedicated arms plus mutation probe B. Finding 3 (the barrier hook) → Task 3 Step 4. Finding 4 (lib count) → cosmetic, folded into Task 1's header wording.

**Known soft spots the implementer must resolve, not paper over.** Task 5 Step 1 names `stub_verify_run` and `remove_anchor_worktree` for their *shape*; their real spellings live in `tests/test_runner_dispatch_build_gate.sh` and `tests/lib/runner_dispatch_detach_common.sh` and must be read before the arm is written. Every mutation probe in this plan carries a landing check — **run the probe, confirm the byte transition it states, and treat a mismatch as a defect in the probe**, since plan-supplied probe code in this repo has shipped defective repeatedly.
