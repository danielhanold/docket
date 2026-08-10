<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0282 — Launch-and-wait contract for long-running child processes — liveness-keyed, not marker-keyed](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-10-0282-launch-and-wait-contract-for-long-running-child-processes-li.md)**
<!-- docket:backlink:end -->

# Launch-and-wait contract for long-running child processes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `scripts/gate-run.sh` — a three-verb (`--launch` / `--observe` / `--stop`) helper that starts a long-running child detached and durable, and whose wait predicate is keyed on **process liveness plus a terminal record**, never on a success marker — so a dead child is detected on the next observation instead of at a wait-loop bound.

**Architecture:** One shared POSIX-ish Bash script with three verbs and a per-run directory holding the launch record, the identity token, the durable streams, and a separately-written terminal record. `--observe` classifies into six states with the terminal record always outranking a liveness probe, re-read on both sides of the probe. `--stop` owns termination, identity-checked before it signals, with the record outranking the stop at every step. The caller owns the polling loop and its budget; the helper never polls internally. Call-site posture (one bounded relaunch on `died`, gated on `--stop`'s report; `--stop` before abandoning a live child) is stated in `docket-build` § *Gate execution posture*, which `docket-finalize-change` already inherits by citation.

**Tech Stack:** Bash (repo floor: Bash 4+, `DOCKET_BASH_PATH` re-exec via `scripts/docket.sh`), the repo's own test harness (`tests/lib/`, `assert` helper), `tests/runtime-budgets.tsv`.

## Global Constraints

Copied verbatim from the spec and from the repo's promoted rules (AGENTS.md). Every task's requirements implicitly include this section.

- **stdout is the protocol; every diagnostic goes to stderr.** Each verb prints **exactly one machine-readable line** on stdout. Payload is per verb: `--launch` prints the absolute run-directory path on success and the single slash-free token `launch-failed` on failure; `--observe` prints `state=<state> [cause=<cause>]`; `--stop` prints one of `stopped` / `already-terminal` / `unavailable`. (spec assumptions 18, 25)
- **Callers key on the stdout report line, never the exit code.** The exit-code mapping is documented in `gate-run.md` for scripting completeness only. (assumption 11)
- **Six `--observe` states:** `running`, `passed`, `failed`, `died` (`cause=signal` | `cause=vanished`), `stopped`, `unavailable`. **Only `running` is retryable**; the other five are terminal. (assumption 20)
- **`died` is never `failed`.** A child killed by a signal never finished. `failed` (ran and went red) is the only state that may feed repair work. (assumption 3, 16)
- **Liveness is identity-checked, never bare `kill -0`** — except the one place assumption 9 scopes: `--stop` step 1's orphan probe, where the leader is known dead so no match is possible **and** an alive result can only move the outcome fail-closed. Bare `kill -0` is forbidden anywhere its false positive would read a dead run as `running` or let a signal reach a bystander. (assumption 9)
- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`, `head -n1`) under `set -o pipefail` — capture into a variable first, then `grep <<<"$var"`. (AGENTS.md)
- **`grep` for a pattern that leads with `--` must declare it**: `grep -E -e "<pat>"` or `grep -qF -- "<pat>"`. (AGENTS.md)
- **awk indent classes are `[^[:space:]]`, never `[^ ]`.** (AGENTS.md)
- **Always `mv -f`** on install/replace paths; **always template `mktemp`**: `"${TMPDIR:-/tmp}/<name>.XXXXXX"` — unless the temp file must sit beside its destination for a same-filesystem atomic rename, in which case template it there. Both apply here: the run-dir default is `"${TMPDIR:-/tmp}/gate-run.XXXXXX"`, and every record write is `mktemp` **beside** its destination then `mv -f`. (AGENTS.md, assumptions 12, 17)
- **A guard is code: mutation-test it** — strip the thing it guards, watch it redden — or it is decoration. Every assert named in this plan has a stated mutation. (AGENTS.md)
- **Test-only synchronization barriers are env-gated and inert by default** — unset means no-op at full speed, so a fixture hook can never become a production hang site. (spec § Mutation test)
- **Run the whole suite at the build gate** via `scripts/run-tests.sh` (the resolved `finalize.test_command`). A trailing `OVER BUDGET:` line is a finding to act on, not noise.
- **Cross-references anchor on a symbol name or a verbatim-quoted clause, never a line number.** (AGENTS.md, ADR-0054)

---

## Resolved at plan time — read before Task 1

Two spec obligations are discharged here so no task has to stall on them.

### 1. Site scope, derived by whole-repo grep (not enumerated)

Per spec § *Site rewiring* and the AGENTS.md never-hand-list rule, the executable-site scope was derived by a repository-root grep over launch/poll shapes (`nohup`, `setsid`, `kill -0`, `until … grep`, `tail -f`, `disown`, `$!`, background-run idioms) with only `.git`, `.docket`, `.worktrees` and `node_modules` excluded. Sorted prose vs executable:

| Site | Class | Disposition |
|---|---|---|
| `skills/docket-build/SKILL.md` § *Gate execution posture* | executable (agent-run normative prose) | **Rewired — Task 8.** The primary target. |
| `skills/docket-build/references/gate-execution.md` | executable (agent-run reference) | **Rewired — Task 8.** Mitigation names the helper; capability 5 gains a pointer only. |
| `skills/docket-build-task/SKILL.md` | citation only — already defers to `gate-execution.md` | **Untouched.** Inherits. |
| `skills/docket-finalize-change/SKILL.md` | citation only — *"obeys the **gate execution posture** `docket-build` owns"* | **Untouched.** Inherits by citation; spec says no restatement added. |
| `scripts/runner-dispatch.sh` (+ `.md`) | executable | **Conscious exclusion** (0277 churn). Its sentinel-only `--observe` gap is a named residual; stub **0284** was minted at reconcile. |
| `scripts/run-tests.sh` (`tpid=$!`) | executable | **Out of scope** — that is the suite's own internal parallelism. run-tests *is* the child, not a gate caller. |
| `skills/docket-convention/SKILL.md` | executable prose | **Untouched** — spec assumption 7: the gate posture's single home stays `docket-build`. |
| `cursor-rules/**`, `AGENTS.md` run gate, `agents/**` | executable prose | **Out of scope** — these govern *subagent dispatch* ("never background it and never poll"), a different lifecycle from a shell child process. |
| `skills/docket-build/references/delegation-execution.md` | executable reference | **Out of scope** — delegation lifecycle, owned by ADR-0080/change 0271. Named in the reconcile log so the grep output was read against a known candidate rather than blind. |
| everything under `docs/` | prose, point-in-time records | **Out of scope** — the convention forbids rewriting archived changes, specs, plans, and results. |

### 2. Session primitive — the probe ladder is resolved, and it is exhausted on macOS

Spec assumption 15 requires the plan to establish which primitive delivers a genuine new session. **Measured at plan time on darwin 25.6.0**, one variable per run:

- **Rung 1 — `setsid(1)`:** `command -v setsid` ⇒ **absent** on macOS. (Matches `runner-dispatch.md`'s recorded floor and `gate-execution-evidence.md`'s *"`setsid(1)` is not installed on macOS"*.) Where present (Linux), it is the preferred rung and passes trivially.
- **Rung 2 — `script(1)` (the macOS pty candidate):** present at `/usr/bin/script`, and it **fails the round-4 full capability set**, measured, on two independent criteria:
  - **Primitive-injected framing.** `/usr/bin/script -q /dev/null /bin/bash -c 'echo alive' </dev/null` produced the bytes `^ D \b \b a l i v e \r \n` — a `^D` typescript marker and pty CRLF translation. The durable log would not hold the child's bytes.
  - **No stream separation.** A pty merges the child's stdout and stderr onto one stream, which capability 2 and the stdout-is-the-protocol rule both forbid.
  - Additionally fragile: with stdin attached to a socket the same invocation failed outright with `script: tcgetattr/ioctl: Operation not supported on socket`.
- **Rung 3 — outcome:** the ladder is **exhausted on macOS without taking a new dependency**. Per assumption 15 this is pre-authorized, and the build **records and continues rather than stalling**:
  - `gate-run.md` records an **honestly narrowed per-platform capability note**: on a platform with no session primitive the contract delivers **own process group** (Bash job control, `set -m` — the technique ADR-0080 *measured*: *"the child gets its own process GROUP, not a new SESSION — it remains in the launcher's session, so session-scoped teardown was not tested and is not claimed"*) **plus the unchanged detachment handshake**. The contract must **not** claim "new session" anywhere it does not deliver one.
  - The narrowing is **escalated as a design finding** for a human to accept or supersede with a dependency. Name the supersede option in the escalation: `/usr/bin/perl` is present and `POSIX::setsid` works — and `gate-execution-evidence.md`'s own four harness verdicts were measured with exactly that shape (*"a `nohup`'d, fully-redirected, backgrounded helper that forks, calls `setsid(2)` in the child"*), so accepting a perl dependency would close the gap. Docket's recorded policy is that it takes no perl dependency; **only a human may change that**. This finding is carried to the results file and is an ADR candidate.

**Task 1 implements rung 1 where available and the narrowed rung 3 otherwise, gated by a runtime probe — never by a hard-coded platform name.**

---

## File Structure

| File | Responsibility |
|---|---|
| `scripts/gate-run.sh` (create) | The three verbs. Sole owner of run-dir layout, records, classification, and signalling. |
| `scripts/gate-run.md` (create) | The authoritative contract — Purpose / Usage / Behavior / Exit codes / Invariants, plus the per-platform capability note and the named residuals. |
| `tests/test_gate_run.sh` (create) | Launch, record, and observe asserts + the identity guard, each keyed to a reddening mutation. |
| `tests/test_gate_run_stop.sh` (create) | `--stop`'s asserts and the six deterministic interleaving race fixtures. Split from the above because one file carrying both would exceed its wall-clock budget and blur two review surfaces. |
| `tests/lib/gate_run_common.sh` (create) | Shared prologue: sandbox, `gate_run` invoker, barrier helpers, `assert` sourcing — the `tests/lib/runner_dispatch_detach_common.sh` pattern. |
| `tests/runtime-budgets.tsv` (modify) | One row per new test file. |
| `scripts/docket.sh` (modify) | Add `gate-run` to `WRAPPED_OPS` so the facade reaches it. |
| `skills/docket-build/SKILL.md` (modify) | § *Gate execution posture* names the helper, the liveness-keyed rule, the scoped one-relaunch rule, and the abandon rule. |
| `skills/docket-build/references/gate-execution.md` (modify) | Mitigation names the helper as its shipped implementation; capability 5 gains a pointer to `gate-run.md`'s six-state vocabulary. No harness verdict is rewritten or re-probed. |
| `tests/test_gate_execution_posture.sh` (modify) | Guard the new posture clauses by shape. |

## Run-directory layout (the contract every task shares)

```
<run-dir>/                        # minted by --launch beneath --root, umask 077
  launch                          # KEY=value: pid, pgid, identity, cmd, created
  identity                        # opaque identity token (process start time of the pgid leader)
  stdout.log                      # child stdout, durable, unmerged
  stderr.log                      # child stderr, durable, unmerged
  terminal                        # written ONLY by the wrapper, ONLY on child exit; atomic
                                  #   kind=exit code=<n>   |   kind=signal signal=<n>
  stop-intent                     # written by --stop step 4, immediately BEFORE the signal
  stopped                         # completed stop marker; written ONLY after termination verified
```

**Invariant, stated once and relied on by every verb:** the untrapped wrapper is the **only** writer of `terminal`, so a `terminal` file visible *after* a liveness probe was necessarily written by a child that completed. This is what makes the re-read in `--observe` step 3 and `--stop` steps 3/6 correct rather than merely defensive.

---

### Task 1: `--launch` — detachment, records, and the handshake

**Files:**
- Create: `scripts/gate-run.sh`
- Create: `tests/lib/gate_run_common.sh`
- Create: `tests/test_gate_run.sh`

**Interfaces:**
- Consumes: nothing.
- Produces: `gate-run.sh --launch [--root <dir>] -- <command…>` → prints an absolute run-dir path on stdout (exit 0) or the token `launch-failed` (non-zero). The run-dir layout above. Every later task reads `launch`, `identity`, `stdout.log`, `stderr.log` from it.

- [ ] **Step 1: Write the failing launch tests**

Create `tests/lib/gate_run_common.sh` with the shared prologue (mirror `tests/lib/runner_dispatch_detach_common.sh`'s shape: source the repo's `assert`, mint a sandbox under `"${TMPDIR:-/tmp}/gate-run-test.XXXXXX"`, and define `gate_run() { bash "$REPO/scripts/gate-run.sh" "$@"; }`).

Then in `tests/test_gate_run.sh`:

```bash
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/gate_run_common.sh"

# ---- launch returns a handle, and returns it FAST -------------------------------
start=$(date +%s)
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 5; exit 0')"; rc=$?
elapsed=$(( $(date +%s) - start ))
assert "launch exits 0" '[ "$rc" = "0" ]'
assert "launch prints an absolute path handle" '[ "${RD:0:1}" = "/" ]'
assert "the handle is a directory that exists" '[ -d "$RD" ]'
# THE POINT: launch must not block for the child's duration.
assert "launch returned well before the child finished" '[ "$elapsed" -lt 3 ]'

# ---- the run dir is private and fully recorded ----------------------------------
perms="$(stat -f '%Lp' "$RD" 2>/dev/null || stat -c '%a' "$RD")"
assert "run dir is 0700 (umask 077)" '[ "$perms" = "700" ]'
assert "launch record exists" '[ -f "$RD/launch" ]'
assert "identity token exists and is non-empty" '[ -s "$RD/identity" ]'
launch_rec="$(cat "$RD/launch")"
assert "launch record carries pid"  'grep -q "^pid="  <<<"$launch_rec"'
assert "launch record carries pgid" 'grep -q "^pgid=" <<<"$launch_rec"'
assert "launch record carries the command line" 'grep -q "^cmd=" <<<"$launch_rec"'
assert "streams are separate durable files" '[ -f "$RD/stdout.log" ] && [ -f "$RD/stderr.log" ]'
assert "no terminal record while the child runs" '[ ! -f "$RD/terminal" ]'

# ---- the child really is detached: its own process group ------------------------
pgid_rec="$(sed -n 's/^pgid=//p' "$RD/launch")"
child_pid="$(sed -n 's/^pid=//p' "$RD/launch")"
os_pgid="$(ps -o pgid= -p "$child_pid" | tr -d ' ')"
assert "recorded pgid matches the OS" '[ "$pgid_rec" = "$os_pgid" ]'
assert "the child is a group leader, not in the launcher's group" \
  '[ "$pgid_rec" = "$child_pid" ] && [ "$pgid_rec" != "$$" ]'

# ---- streams land unmerged and unframed -----------------------------------------
RD2="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'echo to-out; echo to-err >&2')"
for _ in $(seq 1 40); do [ -f "$RD2/terminal" ] && break; sleep 0.25; done
out="$(cat "$RD2/stdout.log")"; err="$(cat "$RD2/stderr.log")"
assert "stdout holds exactly the child's stdout" '[ "$out" = "to-out" ]'
assert "stderr holds exactly the child's stderr" '[ "$err" = "to-err" ]'
# The script(1) rung was rejected at plan time for exactly this: no injected framing, no CR.
assert "no primitive-injected framing in the durable log" '! grep -q $'"'"'\r'"'"' "$RD2/stdout.log"'

# ---- an existing run dir is REFUSED, never reused --------------------------------
mkdir -p "$SBX/collide/run-fixed"
out_line="$(gate_run --launch --root "$SBX/collide" --run-name run-fixed -- /bin/true 2>/dev/null)"
assert "an existing run dir reports the launch-failed token" '[ "$out_line" = "launch-failed" ]'
assert "the failure token carries no slash (shape-distinct from a handle)" \
  '[ "${out_line#*/}" = "$out_line" ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_gate_run.sh`
Expected: FAIL immediately — `scripts/gate-run.sh` does not exist.

- [ ] **Step 3: Implement `--launch`**

Create `scripts/gate-run.sh`. Header comment must state the contract pointer (`scripts/gate-run.md`) and the record-before-exec ordering rule by name, not by line number.

Structure:

```bash
#!/usr/bin/env bash
# scripts/gate-run.sh — detached launch / liveness-keyed observation / identity-checked stop
# for one long-running child process. Contract: scripts/gate-run.md
#
# ORDERING RULE (spec assumption 14, load-bearing): the user's command is NEVER exec'd before
# the pid/pgid/identity record is durably in the run dir. The wrapper records ITSELF after
# detaching, then forks the command. A wedge before the record can strand plumbing at worst,
# never an unaddressable command process.
set -euo pipefail

die() { printf '%s\n' "$*" >&2; }          # diagnostics: stderr, always
report() { printf '%s\n' "$1"; }           # the ONE stdout protocol line

LAUNCH_ESTABLISH_SECS="${GATE_RUN_ESTABLISH_SECS:-10}"

# --- identity ---------------------------------------------------------------------
# The runner-dispatch.md `child_lstart` shape: process start time as identity. A recycled
# pgid has a different start time, so it can never impersonate this run.
identity_of() { ps -o lstart= -p "$1" 2>/dev/null | tr -s ' ' | sed 's/^ *//;s/ *$//'; }

atomic_write() {  # atomic_write <dest> <content>   — temp BESIDE dest, then mv -f
  local dest="$1" content="$2" tmp
  tmp="$(mktemp "${dest}.XXXXXX")"
  printf '%s\n' "$content" >"$tmp"
  mv -f "$tmp" "$dest"
}

# --- session primitive ladder (resolved at plan time; probed at runtime, never by uname) ---
# Rung 1: setsid(1) where present -> a genuine new SESSION.
# Rung 2 (script(1)) was REJECTED at plan time: injected typescript framing + CRLF, and a pty
#         merges stdout/stderr. See scripts/gate-run.md § Per-platform capability note.
# Rung 3: own PROCESS GROUP via Bash job control (`set -m`) + the detachment handshake.
#         The contract is honestly narrowed on such platforms; it never claims "new session".
detach_mode() { command -v setsid >/dev/null 2>&1 && echo session || echo group; }
```

`do_launch` in order:

1. Parse `--root` (default `"${TMPDIR:-/tmp}/gate-run.XXXXXX"` **as an mktemp template**), optional `--run-name` (test-only determinism hook for the collision assert), and the `--` command.
2. `umask 077`. Mint the run dir beneath the root — the helper always mints, so callers cannot collide. **An existing run dir is refused, never reused**: `report launch-failed`, diagnostic to stderr, exit non-zero.
3. If the root is unwritable: same `launch-failed` token (assumption 25 — one shape, never a taxonomy).
4. Spawn the **wrapper** detached: under rung 1, `setsid`; under rung 3, `set -m` + background so the job becomes a process-group leader. Redirect the wrapper's own stdin from `/dev/null` and both its streams into the run dir.
5. **The wrapper, in the child:** determine its own `pid`/`pgid`, compute `identity_of "$pgid"`, `atomic_write` `identity`, `atomic_write` `launch` (pid, pgid, identity, cmd, created) — **and only then** exec/fork the user's command with `stdout.log` / `stderr.log` attached and stdin from `/dev/null`. On the command's exit, classify and `atomic_write` `terminal` (Task 2), untrapped.
6. **The handshake:** the launcher waits, bounded by `LAUNCH_ESTABLISH_SECS`, for `launch` **and** `identity` to both exist and be non-empty. Only then does it `report "$RUN_DIR"`.
7. **The failure path is a bounded stop.** On timeout: if `launch` exists, `TERM` the recorded pgid → bounded grace → `KILL`; else best-effort kill the direct spawn child. Verify what was killed. Report anything unverified **loudly on stderr with the run-dir path for manual disposal**, then `report launch-failed`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_gate_run.sh`
Expected: PASS, all asserts.

- [ ] **Step 5: Mutation-check the two wedge points**

These are asserts, not manual checks — add them to `tests/test_gate_run.sh` using the env-gated barrier (`GATE_RUN_TEST_WEDGE=<point>`, inert when unset):

```bash
# Wedge AFTER the record lands: the recorded group must die, nothing leaks.
RD3="$(GATE_RUN_TEST_WEDGE=post-record GATE_RUN_ESTABLISH_SECS=2 \
        gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30' 2>/dev/null)"
assert "a wedged launch reports the failure token" '[ "$RD3" = "launch-failed" ]'
wedged_pgid="$(sed -n 's/^pgid=//p' "$SBX/runs"/*/launch 2>/dev/null | tail -1)"
assert "the recorded group was killed by the failure path" \
  '[ -z "$wedged_pgid" ] || ! kill -0 -"$wedged_pgid" 2>/dev/null'

# Wedge BEFORE the record: no command process may EVER have existed.
before="$(pgrep -f 'gate-run-canary' | wc -l | tr -d ' ')"
GATE_RUN_TEST_WEDGE=pre-record GATE_RUN_ESTABLISH_SECS=2 \
  gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30 # gate-run-canary' >/dev/null 2>&1 || true
after="$(pgrep -f 'gate-run-canary' | wc -l | tr -d ' ')"
assert "no command process exists when the wedge precedes the record" '[ "$after" = "$before" ]'
```

**Mutations that must redden:** delete the failure-path self-stop (post-record assert reddens); reorder exec before the record write (pre-record assert reddens).

- [ ] **Step 6: Commit**

```bash
git add scripts/gate-run.sh tests/lib/gate_run_common.sh tests/test_gate_run.sh
git commit -m "feat(0282): gate-run --launch — detached, record-before-exec, handshake-gated"
```

---

### Task 2: The terminal record — termination *kind*, not a bare integer

**Files:**
- Modify: `scripts/gate-run.sh` (the wrapper's exit path)
- Modify: `tests/test_gate_run.sh`

**Interfaces:**
- Consumes: Task 1's wrapper and run dir.
- Produces: `<run-dir>/terminal` containing exactly one of `kind=exit code=<n>` or `kind=signal signal=<n>`, written atomically, only by the wrapper, only on child exit. Task 3 classifies from it.

- [ ] **Step 1: Write the failing tests**

```bash
# ---- kind=exit for an ordinary exit ----------------------------------------------
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'exit 0')"
await_terminal "$RD"
rec="$(cat "$RD/terminal")"
assert "a clean exit records kind=exit code=0" '[ "$rec" = "kind=exit code=0" ]'

RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'exit 1')"
await_terminal "$RD"
rec="$(cat "$RD/terminal")"
assert "a red exit records kind=exit code=1" '[ "$rec" = "kind=exit code=1" ]'

# ---- kind=signal for death by signal — THE assert that keeps a never-ran suite
# ---- from minting integration-repair work (assumption 16).
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30')"
pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
kill -TERM -"$pgid" 2>/dev/null || true
await_terminal "$RD"
rec="$(cat "$RD/terminal")"
assert "a TERMed child records kind=signal, never kind=exit" 'grep -q "^kind=signal" <<<"$rec"'
assert "the signal number is recorded" 'grep -q "signal=15" <<<"$rec"'
```

Add `await_terminal()` to `tests/lib/gate_run_common.sh` — a bounded loop on file existence, never an unbounded wait.

- [ ] **Step 2: Run to verify failure**

Run: `bash tests/test_gate_run.sh`
Expected: the three `kind=` asserts FAIL (no `terminal` classification yet, or a bare integer).

- [ ] **Step 3: Implement the classification**

In the wrapper's exit path:

```bash
# The POSIX-shell floor and which way it is biased (spec assumption 16, NAMED RESIDUAL):
# a shell sees only $?, which conflates a genuine `exit 143` with death by signal 15. A code
# in 129..192 is therefore recorded kind=signal. The two errors are NOT symmetric: reading a
# signal death as `failed` mints integration-repair work for tests that never ran, which
# assumption 3 forbids; reading a genuine `exit 143` as `died` costs one relaunch that
# reproduces the same code and then halts. Recorded in scripts/gate-run.md § Named residuals.
rc=$?
if [ "$rc" -ge 129 ] && [ "$rc" -le 192 ]; then
  atomic_write "$RUN_DIR/terminal" "kind=signal signal=$(( rc - 128 ))"
else
  atomic_write "$RUN_DIR/terminal" "kind=exit code=$rc"
fi
```

The wrapper is **untrapped** — it must not install a `TERM` handler, or the record would stop being evidence that the child itself completed.

- [ ] **Step 4: Run to verify pass**

Run: `bash tests/test_gate_run.sh`
Expected: PASS.

**Mutation that must redden:** record the bare `$?` integer and classify by zero/nonzero — the `kind=signal` asserts fail.

- [ ] **Step 5: Commit**

```bash
git add scripts/gate-run.sh tests/lib/gate_run_common.sh tests/test_gate_run.sh
git commit -m "feat(0282): terminal record encodes termination kind, not a bare integer"
```

---

### Task 3: `--observe` — six states, record-outranks-liveness read order

**Files:**
- Modify: `scripts/gate-run.sh`
- Modify: `tests/test_gate_run.sh`

**Interfaces:**
- Consumes: the run dir, `terminal`, `identity`, `launch`, and (read-only) `stopped` / `stop-intent` from Task 5.
- Produces: `gate-run.sh --observe <run-dir>` → one stdout line `state=<state>` or `state=died cause=<signal|vanished>`. Every caller's loop keys on this line.

- [ ] **Step 1: Write the failing tests**

```bash
# ---- running / passed / failed ----------------------------------------------------
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 20')"
assert "a live child observes as running" '[ "$(gate_run --observe "$RD")" = "state=running" ]'
kill -KILL -"$(sed -n 's/^pgid=//p' "$RD/launch")" 2>/dev/null || true

RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'exit 0')"; await_terminal "$RD"
assert "a clean exit observes as passed" '[ "$(gate_run --observe "$RD")" = "state=passed" ]'

RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'exit 1')"; await_terminal "$RD"
assert "a red exit observes as failed" '[ "$(gate_run --observe "$RD")" = "state=failed" ]'

# ---- died cause=signal: the 0276 headline shape ------------------------------------
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30')"
kill -TERM -"$(sed -n 's/^pgid=//p' "$RD/launch")" 2>/dev/null || true
await_terminal "$RD"
assert "a signal death observes as died cause=signal, never failed" \
  '[ "$(gate_run --observe "$RD")" = "state=died cause=signal" ]'

# ---- died cause=vanished: group gone, no record ever written -----------------------
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30')"
kill -KILL -"$(sed -n 's/^pgid=//p' "$RD/launch")" 2>/dev/null || true   # KILL: no record
sleep 1
obs="$(gate_run --observe "$RD" 2>"$SBX/obs.err")"
assert "a vanished group observes as died cause=vanished" '[ "$obs" = "state=died cause=vanished" ]'
# THE PROMPTNESS PROPERTY the whole change exists for: detected on the NEXT observation.
assert "the log tail goes to STDERR, never the protocol channel" '[ -s "$SBX/obs.err" ]'
assert "stdout carried exactly one line" '[ "$(wc -l <<<"$obs")" = "1" ]'

# ---- unavailable: malformed record, and an unreadable run dir -----------------------
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'exit 0')"; await_terminal "$RD"
printf 'garbage\n' >"$RD/terminal"
assert "a malformed record observes as unavailable" \
  '[ "$(gate_run --observe "$RD" 2>/dev/null)" = "state=unavailable" ]'
assert "a nonexistent run dir observes as unavailable" \
  '[ "$(gate_run --observe "$SBX/no-such-run" 2>/dev/null)" = "state=unavailable" ]'

# ---- THE IDENTITY GUARD: a recycled pgid must never read alive ----------------------
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30')"
pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
kill -KILL -"$pgid" 2>/dev/null || true; sleep 1
# Substitute a live FOREIGN group under the recorded pgid by rewriting the record to point at
# a process that is alive but whose identity token does not match.
foreign_pgid="$(start_foreign_group)"          # helper in gate_run_common.sh
sed -i.bak "s/^pgid=.*/pgid=$foreign_pgid/" "$RD/launch"
assert "an identity mismatch reads died, never running" \
  '[ "$(gate_run --observe "$RD" 2>/dev/null)" = "state=died cause=vanished" ]'
```

- [ ] **Step 2: Run to verify failure**

Run: `bash tests/test_gate_run.sh`
Expected: every `--observe` assert FAILS — the verb does not exist.

- [ ] **Step 3: Implement `--observe`**

The read order is the contract. Implement it literally:

```bash
do_observe() {
  local rd="$1"
  [ -d "$rd" ] && [ -r "$rd/launch" ] || { die "rundir-unreadable: $rd"; report "state=unavailable"; return 1; }

  # 1. Terminal record FIRST — it always wins.
  local rec; rec="$(classify_record "$rd")" && { report "$rec"; return 0; }

  # 2. Liveness — identity-checked (assumption 9), never a bare kill -0.
  if group_alive_and_ours "$rd"; then report "state=running"; return 0; fi

  # 3. Dead or identity-mismatched ⇒ RE-READ. THE WHOLE POINT: atomicity prevents partial
  #    reads, not stale ones. Without this, "no record → child finishes → dead probe" turns a
  #    run that PASSED into a died, and triggers a relaunch on it.
  rec="$(classify_record "$rd")" && { report "$rec"; return 0; }

  if [ -f "$rd/stopped" ]; then report "state=stopped"; return 0; fi
  die "$(tail -n 20 "$rd/stderr.log" 2>/dev/null)"      # diagnostic: stderr only
  report "state=died cause=vanished"
}
```

`classify_record` returns non-zero when `terminal` is absent; when present it maps
`kind=exit code=0` → `state=passed`; `kind=exit` nonzero → `state=failed`;
`kind=signal` → `state=stopped` **if** `stopped` **or** `stop-intent` exists, else `state=died cause=signal`;
anything unparseable → `state=unavailable` (a malformed record means the *supervisor* did not finish cleanly, and a verdict read out of garbage is fabricated).

`group_alive_and_ours` is the conjunction: `kill -0 -"$pgid"` **and** `identity_of "$pgid"` equals the recorded token.

Add `start_foreign_group()` to `tests/lib/gate_run_common.sh`.

- [ ] **Step 4: Run to verify pass**

Run: `bash tests/test_gate_run.sh`
Expected: PASS.

**Mutations that must redden:** drop the identity cross-check (the recycled-pgid assert reads `running`); classify `kind=signal` as `failed` (the died-cause-signal assert); make `unavailable` retryable (see Task 7's contract assert).

- [ ] **Step 5: Commit**

```bash
git add scripts/gate-run.sh tests/lib/gate_run_common.sh tests/test_gate_run.sh
git commit -m "feat(0282): gate-run --observe — six states, identity-checked liveness, record outranks"
```

---

### Task 4: The observe TOCTOU race — a deterministic interleaving fixture

**Files:**
- Modify: `scripts/gate-run.sh` (env-gated barrier)
- Modify: `tests/test_gate_run.sh`
- Modify: `tests/lib/gate_run_common.sh`

**Interfaces:**
- Consumes: Task 3's `do_observe`.
- Produces: `GATE_RUN_TEST_BARRIER=<point>` + `GATE_RUN_TEST_BARRIER_FILE=<path>` — a wait-for-file barrier, **inert when unset**. Task 6 reuses both.

- [ ] **Step 1: Write the failing race test**

```bash
# OBSERVE TOCTOU (assumption 19). Hold the observer BETWEEN its first record read and its
# liveness probe; let the wrapper write kind=exit code=0 and exit; release.
# The observation must report PASSED, never died. This is the mutation that otherwise ships
# a died on a run that passed — and with it a spurious relaunch.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'while [ ! -f '"$SBX"'/go ]; do sleep 0.1; done; exit 0')"
BAR="$SBX/observe-barrier"
( GATE_RUN_TEST_BARRIER=post-first-record GATE_RUN_TEST_BARRIER_FILE="$BAR" \
    gate_run --observe "$RD" >"$SBX/obs.out" 2>/dev/null ) &
obs_job=$!
wait_for_file "$BAR.reached"            # observer is now held, after its first record read
touch "$SBX/go"; await_terminal "$RD"   # child completes and the record lands
touch "$BAR.release"                    # release the observer
wait "$obs_job"
assert "an observer held across completion reports passed, never died" \
  '[ "$(cat "$SBX/obs.out")" = "state=passed" ]'
```

Add `wait_for_file()` (bounded) to `tests/lib/gate_run_common.sh`.

- [ ] **Step 2: Run to verify failure**

Run: `bash tests/test_gate_run.sh`
Expected: FAIL — no barrier exists, so `wait_for_file` times out.

- [ ] **Step 3: Implement the barrier and wire it**

```bash
# Test-only synchronization point. ENV-GATED AND INERT BY DEFAULT: unset means a no-op at full
# speed, so this hook can never itself become a hang site in production.
barrier() {
  [ "${GATE_RUN_TEST_BARRIER:-}" = "$1" ] || return 0
  local f="${GATE_RUN_TEST_BARRIER_FILE:?barrier point set without a barrier file}"
  : >"$f.reached"
  local waited=0
  while [ ! -f "$f.release" ]; do sleep 0.1; waited=$((waited+1)); [ "$waited" -gt 300 ] && break; done
}
```

Call `barrier post-first-record` in `do_observe` between step 1 and step 2.

- [ ] **Step 4: Run to verify pass**

Run: `bash tests/test_gate_run.sh`
Expected: PASS.

- [ ] **Step 5: Prove the barrier is inert by default**

```bash
start=$(date +%s%N)
gate_run --observe "$RD" >/dev/null 2>&1
assert "the barrier is inert when unset (no measurable stall)" \
  '[ $(( ($(date +%s%N) - start) / 1000000 )) -lt 2000 ]'
```

**Mutation that must redden:** delete `do_observe`'s step-3 re-read — the held-observer assert reports `died`.

- [ ] **Step 6: Commit**

```bash
git add scripts/gate-run.sh tests/lib/gate_run_common.sh tests/test_gate_run.sh
git commit -m "test(0282): observe TOCTOU pinned by a deterministic interleaving fixture"
```

---

### Task 5: `--stop` — the seven steps, three report tokens

**Files:**
- Modify: `scripts/gate-run.sh`
- Create: `tests/test_gate_run_stop.sh`

**Interfaces:**
- Consumes: the run dir, `identity`, `launch`, `terminal`.
- Produces: `gate-run.sh --stop <run-dir> [--reason <text>]` → one stdout token: `stopped` | `already-terminal` | `unavailable`. Writes `stop-intent` (pre-signal) and `stopped` (post-verification). **Never writes `terminal`.**

- [ ] **Step 1: Write the failing tests**

Create `tests/test_gate_run_stop.sh` (same prologue source):

```bash
# ---- stopping a live child leaves the recorded group verified gone -------------------
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
assert "stopping a live child reports stopped" '[ "$(gate_run --stop "$RD")" = "stopped" ]'
assert "the recorded group is verified gone" '! kill -0 -"$pgid" 2>/dev/null'
assert "a completed stop marker exists" '[ -f "$RD/stopped" ]'
assert "--stop writes NO terminal record" '[ ! -f "$RD/terminal" ]'
assert "a stopped run observes as stopped, never passed" \
  '[ "$(gate_run --observe "$RD")" = "state=stopped" ]'

# ---- KILL escalation: a child that ignores TERM is still gone after the grace ---------
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'trap "" TERM; sleep 60')"
pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
assert "a TERM-ignoring child is still stopped" '[ "$(gate_run --stop "$RD")" = "stopped" ]'
assert "KILL escalation removed the group" '! kill -0 -"$pgid" 2>/dev/null'

# ---- idempotence -----------------------------------------------------------------------
assert "a second --stop reports already-terminal" '[ "$(gate_run --stop "$RD")" = "already-terminal" ]'
gate_run --stop "$RD" >/dev/null 2>&1
assert "a second --stop is not an error" '[ "$?" = "0" ]'

RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'exit 0')"; await_terminal "$RD"
assert "--stop on an already-passed run reports already-terminal" \
  '[ "$(gate_run --stop "$RD")" = "already-terminal" ]'
assert "and it observes as passed still — the stop did not reclassify it" \
  '[ "$(gate_run --observe "$RD")" = "state=passed" ]'

# ---- IDENTITY GUARD ON THE SIGNAL PATH: a recycled pgid must be signalled NOT AT ALL ----
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
kill -KILL -"$(sed -n 's/^pgid=//p' "$RD/launch")" 2>/dev/null || true; sleep 1
foreign_pgid="$(start_foreign_group)"
sed -i.bak "s/^pgid=.*/pgid=$foreign_pgid/" "$RD/launch"
assert "a recycled group reports unavailable" '[ "$(gate_run --stop "$RD" 2>/dev/null)" = "unavailable" ]'
assert "and the bystander group was NOT signalled" 'kill -0 -"$foreign_pgid" 2>/dev/null'
kill -KILL -"$foreign_pgid" 2>/dev/null || true

# ---- VANISHED-GROUP GATE (assumption 21): no record, group absent ⇒ no marker ----------
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
kill -KILL -"$(sed -n 's/^pgid=//p' "$RD/launch")" 2>/dev/null || true; sleep 1
assert "a vanished group reports already-terminal" '[ "$(gate_run --stop "$RD" 2>/dev/null)" = "already-terminal" ]'
assert "and writes NO stop marker — the vanished death must stay relaunchable" '[ ! -f "$RD/stopped" ]'
assert "so it still observes as died cause=vanished" \
  '[ "$(gate_run --observe "$RD" 2>/dev/null)" = "state=died cause=vanished" ]'

# ---- ORPHAN PROBE on the record-present path (assumption 24) ---------------------------
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
kill -TERM -"$(sed -n 's/^pgid=//p' "$RD/launch")" 2>/dev/null || true
await_terminal "$RD"                                   # a kind=signal record now exists
orphan_pgid="$(start_foreign_group)"                   # plant a live process in the recorded pgid
sed -i.bak "s/^pgid=.*/pgid=$orphan_pgid/" "$RD/launch"
assert "live orphans under a present record report unavailable" \
  '[ "$(gate_run --stop "$RD" 2>/dev/null)" = "unavailable" ]'
assert "and nothing was signalled — ownership is unprovable, detection is not" \
  'kill -0 -"$orphan_pgid" 2>/dev/null'
kill -KILL -"$orphan_pgid" 2>/dev/null || true
```

- [ ] **Step 2: Run to verify failure**

Run: `bash tests/test_gate_run_stop.sh`
Expected: FAIL — `--stop` does not exist.

- [ ] **Step 3: Implement `--stop`, literally seven steps**

```bash
do_stop() {
  local rd="$1" reason="${2:-}"
  [ -d "$rd" ] && [ -r "$rd/launch" ] || { die "rundir-unreadable: $rd"; report "unavailable"; return 1; }
  local pgid; pgid="$(sed -n 's/^pgid=//p' "$rd/launch")"

  # 1. Record present ⇒ PROBE the recorded group before reporting (assumption 24).
  #    The leader is dead, so ownership of any survivor is unprovable — but detection is
  #    possible even where safe signalling is not, and relaunching over live orphans is the
  #    0231 double-run state. This is the ONE admissible bare probe (assumption 9): no match
  #    is possible and an alive result can only move the outcome fail-closed.
  if [ -f "$rd/terminal" ]; then
    if kill -0 -"$pgid" 2>/dev/null; then die "orphans-detected"; report "unavailable"; return 1; fi
    report "already-terminal"; return 0
  fi

  # 2. Validate identity. An ABSENT group is not an identity failure — unavailable requires a
  #    group that EXISTS — so it falls through to step 4's absent branch.
  if kill -0 -"$pgid" 2>/dev/null && ! identity_matches "$rd" "$pgid"; then
    die "ownership-unprovable"; report "unavailable"; return 1
  fi

  # 3. Re-read the record IMMEDIATELY before signalling — nothing but the intent write and the
  #    kill separate the test from the act.
  [ -f "$rd/terminal" ] && { report "already-terminal"; return 0; }
  barrier pre-term

  # 4. Probe the recorded group with the assumption 9 conjuncts, not a bare probe.
  if ! kill -0 -"$pgid" 2>/dev/null; then
    [ -f "$rd/terminal" ] && { report "already-terminal"; return 0; }
    report "already-terminal"; return 0            # NOTHING is written: stays relaunchable
  fi
  identity_matches "$rd" "$pgid" || { die "ownership-unprovable"; report "unavailable"; return 1; }
  atomic_write "$rd/stop-intent" "reason=${reason}"     # claims only that a signal is IMMINENT
  kill -TERM -"$pgid" 2>/dev/null || true
  local waited=0
  while kill -0 -"$pgid" 2>/dev/null && [ "$waited" -lt 20 ]; do sleep 0.5; waited=$((waited+1)); done
  kill -0 -"$pgid" 2>/dev/null && kill -KILL -"$pgid" 2>/dev/null || true

  # 5. Verify the group is gone.
  local v=0; while kill -0 -"$pgid" 2>/dev/null && [ "$v" -lt 20 ]; do sleep 0.25; v=$((v+1)); done
  kill -0 -"$pgid" 2>/dev/null && { die "termination-unverified"; report "unavailable"; return 1; }

  # 6. Re-read AFTER the kill and BEFORE any marker is written.
  barrier post-kill-pre-annotate
  if [ -f "$rd/terminal" ]; then
    # A kind=signal here is almost certainly the step-4 TERM. Annotate, or a deliberately
    # cancelled run reads died forever and an idempotent site relaunches a cancellation.
    grep -q "^kind=signal" "$rd/terminal" && atomic_write "$rd/stopped" "reason=${reason}"
    report "already-terminal"; return 0
  fi

  # 7. Only now — the group HAVING ACTUALLY BEEN SIGNALLED and verified gone.
  atomic_write "$rd/stopped" "stopped_at=$(date -u +%Y-%m-%dT%H:%M:%SZ) reason=${reason}"
  report "stopped"
}
```

- [ ] **Step 4: Run to verify pass**

Run: `bash tests/test_gate_run_stop.sh`
Expected: PASS.

**Mutations that must redden:** drop the `KILL` escalation (TERM-ignoring assert); lose idempotence; write a `terminal` record from `--stop`; remove the pre-signal identity check (bystander is killed); write the marker unconditionally at step 7 (vanished-group assert); drop the step-1 orphan probe.

- [ ] **Step 5: Commit**

```bash
git add scripts/gate-run.sh tests/test_gate_run_stop.sh
git commit -m "feat(0282): gate-run --stop — identity-checked, record outranks the stop at every step"
```

---

### Task 6: The five remaining `--stop` race fixtures

**Files:**
- Modify: `scripts/gate-run.sh` (barrier call sites only)
- Modify: `tests/test_gate_run_stop.sh`

**Interfaces:**
- Consumes: Task 4's barrier, Task 5's `do_stop`.
- Produces: no new interface — five asserts, each keyed to a mutation.

- [ ] **Step 1: Write the five failing race tests**

Each uses the barrier, never a sleep.

```bash
# (1) STOP-VERSUS-COMPLETION (assumption 21). Hold --stop after its identity check and before
#     its TERM; let the child complete; release. It must report already-terminal, signal
#     nothing, and write no marker.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'while [ ! -f '"$SBX"'/go1 ]; do sleep 0.1; done; exit 0')"
BAR="$SBX/b1"
( GATE_RUN_TEST_BARRIER=pre-term GATE_RUN_TEST_BARRIER_FILE="$BAR" gate_run --stop "$RD" >"$SBX/s1.out" 2>/dev/null ) &
j=$!; wait_for_file "$BAR.reached"; touch "$SBX/go1"; await_terminal "$RD"; touch "$BAR.release"; wait "$j"
assert "a stop held across completion reports already-terminal" '[ "$(cat "$SBX/s1.out")" = "already-terminal" ]'
assert "and it observes as passed — the completed run kept its verdict" \
  '[ "$(gate_run --observe "$RD")" = "state=passed" ]'

# (2) MARKER-BEFORE-VERIFICATION (assumption 21). Kill --stop between its signal and its marker
#     write; assert NO stopped marker, and a subsequent --stop still attempts termination
#     rather than no-opping on a marker claiming a kill that never happened.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'trap "" TERM; sleep 60')"
BAR="$SBX/b2"
( GATE_RUN_TEST_BARRIER=post-kill-pre-annotate GATE_RUN_TEST_BARRIER_FILE="$BAR" \
    gate_run --stop "$RD" >/dev/null 2>&1 ) & j=$!
wait_for_file "$BAR.reached"; kill -KILL "$j" 2>/dev/null; wait "$j" 2>/dev/null || true
assert "a half-dead stop leaves NO marker" '[ ! -f "$RD/stopped" ]'

# (3) DELIBERATE STOP RECORDED AS SIGNAL DEATH (assumption 23). Hold --stop between its TERM
#     and its step-6 re-read; let the wrapper reap the signal death and write kind=signal;
#     release. --stop must report already-terminal AND write the stop marker, and a subsequent
#     --observe must report stopped, never died.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
BAR="$SBX/b3"
( GATE_RUN_TEST_BARRIER=post-kill-pre-annotate GATE_RUN_TEST_BARRIER_FILE="$BAR" \
    gate_run --stop "$RD" >"$SBX/s3.out" 2>/dev/null ) & j=$!
wait_for_file "$BAR.reached"; await_terminal "$RD"; touch "$BAR.release"; wait "$j"
assert "the annotation path reports already-terminal" '[ "$(cat "$SBX/s3.out")" = "already-terminal" ]'
assert "a deliberately cancelled run observes as stopped, never died" \
  '[ "$(gate_run --observe "$RD")" = "state=stopped" ]'

# (4) INTENT SURVIVES A CRASHED ANNOTATION (assumption 23). Kill --stop between its step-6
#     record-found read and the annotation write; the stop-intent already exists, so a
#     subsequent --observe must still report stopped.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
BAR="$SBX/b4"
( GATE_RUN_TEST_BARRIER=post-kill-pre-annotate GATE_RUN_TEST_BARRIER_FILE="$BAR" \
    gate_run --stop "$RD" >/dev/null 2>&1 ) & j=$!
wait_for_file "$BAR.reached"; await_terminal "$RD"; kill -KILL "$j" 2>/dev/null; wait "$j" 2>/dev/null || true
assert "the stop-intent record exists" '[ -f "$RD/stop-intent" ]'
assert "intent alone reclassifies the signal death as deliberate" \
  '[ "$(gate_run --observe "$RD")" = "state=stopped" ]'

# (5) RECYCLED GROUP INSIDE THE STOP WINDOW (assumptions 9, 24). Hold --stop between its step-2
#     identity validation and its step-4 probe; substitute a live foreign group under the
#     recorded pgid; release. --stop must report unavailable and signal NOTHING AT ALL.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
orig_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
BAR="$SBX/b5"
( GATE_RUN_TEST_BARRIER=pre-term GATE_RUN_TEST_BARRIER_FILE="$BAR" gate_run --stop "$RD" >"$SBX/s5.out" 2>/dev/null ) & j=$!
wait_for_file "$BAR.reached"
kill -KILL -"$orig_pgid" 2>/dev/null || true; sleep 1
foreign_pgid="$(start_foreign_group)"; sed -i.bak "s/^pgid=.*/pgid=$foreign_pgid/" "$RD/launch"
touch "$BAR.release"; wait "$j"
assert "a group recycled inside the stop window reports unavailable" '[ "$(cat "$SBX/s5.out")" = "unavailable" ]'
assert "and the bystander was never signalled" 'kill -0 -"$foreign_pgid" 2>/dev/null'
kill -KILL -"$foreign_pgid" 2>/dev/null || true
```

- [ ] **Step 2: Run to verify failure**

Run: `bash tests/test_gate_run_stop.sh`
Expected: fixtures 1–5 FAIL where a barrier point is missing or a step is absent.

- [ ] **Step 3: Add the missing barrier call sites and step-4 identity re-check**

`do_stop` already calls `barrier pre-term` (step 3→4 boundary) and `barrier post-kill-pre-annotate` (step 5→6 boundary). Confirm step 4 runs the **assumption 9 conjuncts** — `kill -0` **and** `identity_matches` — so an alive-but-mismatched group reports `unavailable` and signals nothing. This is the one probe whose alive result gates a `TERM`, so it must re-check identity on the same side of the fence as the kill.

- [ ] **Step 4: Run to verify pass**

Run: `bash tests/test_gate_run_stop.sh`
Expected: PASS.

**Mutations that must redden, one per fixture:** delete the pre-signal re-read (1); write the marker before verification (2); delete the step-6 annotation write (3); delete the pre-signal intent write (4); delete the step-4 identity re-check (5).

- [ ] **Step 5: Commit**

```bash
git add scripts/gate-run.sh tests/test_gate_run_stop.sh
git commit -m "test(0282): five deterministic interleaving fixtures pin --stop's races"
```

---

### Task 7: Contract, facade wiring, and budgets

**Files:**
- Create: `scripts/gate-run.md`
- Modify: `scripts/docket.sh` (the `WRAPPED_OPS` list)
- Modify: `tests/runtime-budgets.tsv`
- Modify: `tests/test_gate_run.sh` (contract-fidelity asserts)

**Interfaces:**
- Consumes: everything above.
- Produces: `"${DOCKET_SCRIPTS_DIR}"/docket.sh gate-run …` as the caller-facing invocation, and the authoritative contract every call site cites.

- [ ] **Step 1: Write the failing wiring and contract asserts**

```bash
assert "the facade wraps gate-run" 'grep -qF -- "gate-run" "$REPO/scripts/docket.sh"'
assert "gate-run has a co-located contract" '[ -f "$REPO/scripts/gate-run.md" ]'
contract="$(cat "$REPO/scripts/gate-run.md")"
for s in Purpose Usage Behavior "Exit codes" Invariants; do
  assert "the contract has a $s section" 'grep -q "^## .*'"$s"'" <<<"$contract"'
done
# The vocabulary lives HERE (assumption 10) — the skill posture only points at it.
for st in running passed failed died stopped unavailable; do
  assert "the contract defines the $st state" 'grep -qF -- "'"$st"'" <<<"$contract"'
done
assert "the contract states that only running is retryable" \
  'grep -qiE "only .running. is retryable" <<<"$contract"'
assert "the contract records the per-platform capability note" \
  'grep -qi "per-platform capability" <<<"$contract"'
assert "the contract never claims a new session unconditionally" \
  '! grep -qiE "always (creates|delivers) a new session" <<<"$contract"'
assert "the contract records the 129..192 named residual" 'grep -qF -- "129" <<<"$contract"'
# Budgets: a new test file with no budget row is how the suite silently grows.
budgets="$(cat "$REPO/tests/runtime-budgets.tsv")"
assert "test_gate_run.sh has a budget row" 'grep -qF -- "tests/test_gate_run.sh" <<<"$budgets"'
assert "test_gate_run_stop.sh has a budget row" 'grep -qF -- "tests/test_gate_run_stop.sh" <<<"$budgets"'
```

- [ ] **Step 2: Run to verify failure**

Run: `bash tests/test_gate_run.sh`
Expected: FAIL on every assert above.

- [ ] **Step 3: Write the contract, wire the facade, add the budget rows**

`scripts/gate-run.md` — follow the house shape (`scripts/runner-dispatch.md` is the nearest model). Required content:

- **Purpose / Usage / Behavior / Exit codes / Invariants.**
- The **six-state table** verbatim from the spec, with the retryability column and the sentence *"Only `running` is retryable."*
- The **stdout-is-the-protocol** rule with the per-verb payloads, and the three `--stop` report tokens with their conditions.
- `--stop`'s **seven steps** in order, and the record-outranks-the-stop invariant.
- **§ Per-platform capability note** — the honestly narrowed contract from *Resolved at plan time* above. Where the ladder is exhausted the page says **own process group plus the detachment handshake**, and says so per platform; it must not claim "new session" anywhere it does not deliver one. Cite ADR-0080's measured clause and `gate-execution-evidence.md`'s `setsid(1)`-absent finding by **verbatim quote**, never by line number.
- **§ Named residuals** — (a) the `129..192` shell-floor heuristic and why the bias is chosen; (b) a child that escaped the recorded group (double-fork, own session) survives `--stop`; (c) an external signal landing after the stop-intent is recorded as deliberate; (d) the macOS session-primitive narrowing, with the perl supersede option named as a **human decision**.
- **§ Exit codes** — documented "for scripting completeness only; callers key on the stdout report line."

In `scripts/docket.sh`, add `gate-run` to `WRAPPED_OPS`.

In `tests/runtime-budgets.tsv`, add two rows (tab-separated, `parallel`). Seed generously — these files launch real processes and wait on real barriers; start at `40` for `tests/test_gate_run.sh` and `60` for `tests/test_gate_run_stop.sh`, then tighten to the measured value after Task 9's suite run rather than guessing downward.

- [ ] **Step 4: Run to verify pass**

Run: `bash tests/test_gate_run.sh`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/gate-run.md scripts/docket.sh tests/runtime-budgets.tsv tests/test_gate_run.sh
git commit -m "docs(0282): gate-run contract, facade wiring, and runtime budget rows"
```

---

### Task 8: Site rewiring — the gate execution posture

**Files:**
- Modify: `skills/docket-build/SKILL.md` (§ *Gate execution posture*)
- Modify: `skills/docket-build/references/gate-execution.md`
- Modify: `tests/test_gate_execution_posture.sh`

**Interfaces:**
- Consumes: the shipped helper and its contract.
- Produces: the normative posture every build worker and finalize's gate reads.

- [ ] **Step 1: Write the failing posture guard**

In `tests/test_gate_execution_posture.sh`, guard by **shape**, never by an enumerated list of spellings:

```bash
posture="$(cat "$REPO/skills/docket-build/SKILL.md")"
assert "the posture names the shipped helper" 'grep -qF -- "gate-run" <<<"$posture"'
assert "the posture states the liveness-keyed rule" \
  'grep -qiE "liveness|never on a (success )?marker" <<<"$posture"'
assert "the posture states the scoped one-relaunch rule" \
  'grep -qiE "idempotent" <<<"$posture"'
assert "the posture states the abandon-a-live-child rule" \
  'grep -qiE "stop .* before it reports|--stop" <<<"$posture"'
ref="$(cat "$REPO/skills/docket-build/references/gate-execution.md")"
assert "capability 5 points at the contract's vocabulary" 'grep -qF -- "gate-run.md" <<<"$ref"'
# Assumption 10: no harness verdict is rewritten or re-probed by this change.
for h in claude cursor codex opencode; do
  assert "the $h verdict is untouched" 'grep -qE "^### '"$h"'$" <<<"$ref"'
done
```

- [ ] **Step 2: Run to verify failure**

Run: `bash tests/test_gate_execution_posture.sh`
Expected: FAIL on the helper-name and rule asserts.

- [ ] **Step 3: Rewire the two sites**

In `skills/docket-build/SKILL.md` § *Gate execution posture*, keep clauses 1–6 as they are (they are stated by capability and remain correct) and add, in the same voice:

> The shipped implementation of clauses 1–3 is `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh gate-run` — `--launch` to start the suite detached and durable, `--observe` for each short-lived look, `--stop` to terminate one. **Key the wait on the observation's reported state, never on a success marker appearing in the log.** The two differ exactly when the child dies, which is the one moment the wait exists for: a marker-keyed loop runs to its full budget and then reports nothing diagnostic. The six states and their retryability are `gate-run.md`'s contract; **only `running` is retryable.**
>
> On `died` — the child never finished, which is **not** a red suite and never mints repair work — the posture for an **idempotent** child (the suite gate) is: `--stop`, then **one** bounded relaunch **gated on `--stop`'s report** — `stopped` relaunches; `already-terminal` re-observes first and keys on the state that returns (`passed`/`failed` keep the verdict, `died` takes the one relaunch, `stopped` and `unavailable` never relaunch); `unavailable` aborts and reports without relaunching, because there the surviving group could not be proven ours and relaunching would race a live suite. A second `died` is abort-and-report. A **non-idempotent** child keeps its site's existing failure posture.
>
> A caller that stops observing while the state is still `running` — budget exhaustion, halt, or abort — calls `--stop` **before it reports**, so no suite outlives the run the human is about to inspect. Every leg halts; `unavailable` halts **loudly**, because that is the one leg where the human inherits a live process.

In `skills/docket-build/references/gate-execution.md`: the mitigation paragraph names the helper as its shipped implementation, and **capability 5 gains a pointer only** — a sentence saying the state vocabulary is mechanized harness-independently by `scripts/gate-run.md`, which is why the harness-capability list is not its owner. **Rewrite no verdict and re-probe no harness** (assumption 10). Add nothing to `docket-finalize-change` or `docket-build-task` — both already inherit by citation.

- [ ] **Step 4: Run to verify pass**

Run: `bash tests/test_gate_execution_posture.sh`
Expected: PASS.

**Mutation that must redden:** delete the liveness-keyed sentence from the posture — the shape assert fails.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-build/SKILL.md skills/docket-build/references/gate-execution.md tests/test_gate_execution_posture.sh
git commit -m "feat(0282): gate posture names the helper and the liveness-keyed wait rule"
```

---

### Task 9: Full-suite gate and budget tightening

**Files:**
- Modify: `tests/runtime-budgets.tsv` (tighten to measured)

- [ ] **Step 1: Run the whole suite**

Run: `scripts/run-tests.sh`
Expected: green. Run the **whole** suite, never only the files this plan enumerated.

- [ ] **Step 2: Act on any `OVER BUDGET:` line**

A trailing `OVER BUDGET:` block does **not** fail the run, so nothing else will catch it. If either new file is listed, either shard it or raise its row with the measured number and a one-line reason.

- [ ] **Step 3: Tighten the two seeded rows to the measured wall clock**

Replace the generous seeds from Task 7 with the observed durations plus headroom.

- [ ] **Step 4: Re-run and commit**

```bash
scripts/run-tests.sh
git add tests/runtime-budgets.tsv
git commit -m "test(0282): tighten gate-run budget rows to measured wall clock"
```

---

## Self-Review

**1. Spec coverage.** Every spec section maps to a task: `--launch` + handshake + run dir + `launch-failed` (Task 1, assumptions 12/14/17/25); terminal record kind (Task 2, assumption 16); `--observe` six states + read order + identity guard (Task 3, assumptions 9/19/20); the observe TOCTOU fixture (Task 4, assumption 19); `--stop`'s seven steps + three reports + orphan probe + intent record (Task 5, assumptions 13/21/23/24); the five remaining race fixtures (Task 6); the contract with the vocabulary, the per-platform note and the named residuals + facade + budgets (Task 7, assumptions 10/11); site rewiring and the `died`/abandon call-site postures (Task 8, assumptions 4/5/6/7); the suite gate (Task 9). The two plan-time obligations the spec put on *the plan itself* — the derived site scope and the session-primitive ladder (assumption 15) — are discharged in *Resolved at plan time* above rather than deferred into a task.

**2. Placeholder scan.** No `TBD`, no "add appropriate error handling", no "similar to Task N". Every code step carries real code; every test step carries real asserts.

**3. Type consistency.** Names used across tasks are fixed and consistent: run-dir files `launch` / `identity` / `stdout.log` / `stderr.log` / `terminal` / `stop-intent` / `stopped`; functions `identity_of` / `identity_matches` / `atomic_write` / `classify_record` / `group_alive_and_ours` / `barrier` / `do_launch` / `do_observe` / `do_stop`; test helpers `gate_run` / `await_terminal` / `wait_for_file` / `start_foreign_group`; barrier points `pre-record` / `post-record` (launch wedges) and `post-first-record` / `pre-term` / `post-kill-pre-annotate` (observe and stop).

**Escalation carried out of this plan and into the results file:** the macOS session-primitive narrowing (assumption 15) is a design finding a human must accept or supersede with a dependency. It is an ADR candidate, not something any task may decide.
