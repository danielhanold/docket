<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0338 — Gate observe ships two serializations (shell state=name vs native protocol-v1 JSON) reconciled only by prose — converge on JSON, migrate the caller loop, retire the text contract](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-23-0338-gate-execution-terminal-sentinel-has-no-format-contract-poll.md)**
<!-- docket:backlink:end -->
# Gate Observe JSON Convergence Implementation Plan (change 0338)

> **For agentic workers:** REQUIRED SUB-SKILL: Use the docket-build skill to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One serialization for the gate's "done yet?" observation — the native gate's protocol-v1 JSON — with the caller loop migrated to it, `gate-run.sh --observe` hard-retired to a refusal, and the plain-text `state=<name>` observe contract gone.

**Architecture:** `scripts/gate-run.sh` keeps its launch/stop machinery byte-identical in behavior (change 0339 retires those) but its `--observe` verb becomes a non-zero refusal with a one-line stderr pointer to `docket gate observe`; the observe-only classifier functions it alone used are deleted. The canonical caller loop in `scripts/gate-run.md` § *The caller's loop* is rewritten to invoke `docket gate observe <run-dir> --json` and parse the document with jq (now a documented required dependency of the loop), preserving the 0286 loop's arm semantics: `running` retries, everything terminal breaks, anything unrecognizable — empty state, garbled document, jq failure, jq absent — fails closed to `unavailable` and breaks loudly. `skills/docket-build/SKILL.md` drops the prose that reconciled the two serializations and restates the posture in the JSON vocabulary and the native-gate invocation. `tests/test_gate_run.sh` is retargeted (refusal asserts, a fence-executing loop regression against both scripted and real native-gate JSON, mutation-tested fail-closed arms); `tests/test_gate_run_stop.sh` swaps its retired `--observe` oracle for direct record reads.

**Tech Stack:** Bash 4+ test harness (`tests/lib/gate_run_common.sh`), jq, the Go-v1 native gate (`docket gate launch/observe/stop`, `internal/app/gate.go`), `go build ./cmd/docket` for the real-JSON fixture, `go generate ./internal/assets/` for the embedded skill copy.

**Spec:** `docs/superpowers/specs/2026-08-22-gate-observe-json-convergence-design.md` (on the `docket` metadata branch; synchronized copy read at plan time). Change file: `docs/changes/active/0338-gate-execution-terminal-sentinel-has-no-format-contract-poll.md`.

## Global Constraints

- Suite command at the build gate: `scripts/run-tests.sh` (the `finalize.test_command` resolution). Run the whole suite, never only the enumerated tests. A trailing `OVER BUDGET:` line is a finding to act on.
- Never `producer | early-exiting-consumer` under `set -o pipefail` — capture into a variable first, then `grep <<<"$var"` (AGENTS.md § Shell). The new loop's `doc="$(…)"` capture-then-parse shape is this rule applied.
- A guard is code: mutation-test it — strip the thing it guards, watch it redden. Every new assert in this plan names its mutation key.
- Key guards on syntactic shape, never an enumerated list of spellings; derive call-site lists from a whole-repo grep, then sort prose vs executable.
- Cross-references anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054; `tests/test_comment_anchor_style.sh` enforces the filename+line form).
- `mv -f` on install/replace paths; `mktemp` always with a template (`"${TMPDIR:-/tmp}/<name>.XXXXXX"`, or beside the destination for atomic rename).
- Editing `skills/docket-build/SKILL.md` requires regenerating the embedded asset bundle (`go generate ./internal/assets/`), or `tests/test_asset_bundle_drift.sh` reddens.
- Go verification runs must defeat the result cache (`-count=1`) when used as mutation probes (learnings: cached-runner-serves-a-mutated-tree).
- Out of scope (do not touch): retiring `gate-run.sh`'s launch/liveness/stop machinery or `scripts/lib/docket-liveness.sh` (change 0339); any protocol-v1 schema change; change 0264 / ADR-0024; `scripts/runner-dispatch.*` (its `--observe` is a different verb on a different helper).

## Verified facts the plan rests on (read at plan time; re-verify cheaply at build)

These were read from the feature-tree source, not from the spec, and two of them correct the spec's sketch:

1. **`docket gate observe <run-dir>` prints a HUMAN text form (`state: <name>`) by default and protocol-v1 JSON only under the global `--json` flag** (`internal/cli/root.go`: `root.PersistentFlags().Bool("json", …)`; `internal/app/gate.go` text branch `"state: "+string(r.State)`). Every loop invocation must be `docket gate observe "$run_dir" --json`.
2. **The JSON `state` vocabulary is `running`, `passed`, `failed`, `signaled`, `stopped`, `vanished`** (`internal/process/process.go` `State` constants, carried verbatim by `GateState` — "mirrors process.State's spellings"). The native gate **never emits `died`**. The 0286 loop's `died` disposition therefore maps to the two JSON spellings `signaled` and `vanished`; a loop that leaves them to the fail-closed arm would swallow every real signal death as `unavailable`, violating the spec's "arm semantics unchanged" requirement. `cause` is a top-level omitempty string (`GateResult.Cause`).
3. **Observe's exit code is non-zero for real verdicts**: `mapObservation` maps `failed → gate-failed` and `signaled/stopped/vanished → interrupted`, and `app.ExitCode` maps both to exit 1 (only `applied`/`no-op` exit 0; `invalid-input` exits 2). So the loop's `|| true` on the capture is load-bearing on *more* states than in the 0286 text loop, and the test stub must mirror these exits.
4. **A failure document carries no `state` field at all** (`state` is omitempty; `mapGateFailure` builds an envelope without it), so `jq -r '.state // empty'` on it yields the empty string — the fail-closed arm's input.
5. **The native run dir carries `manifest.json` with a `pgid` field** (`internal/process/records.go`), which is how the real-JSON fixture mints `signaled` and `vanished` documents by signalling the real group.
6. **jq is already a working docket dependency** (`scripts/ensure-docket-env.sh`, `scripts/docket-status.sh`), so requiring it of the loop adds no new install burden; the suite machines have it.

## File Structure

- Modify: `scripts/gate-run.sh` — `--observe` verb arm becomes a refusal; observe-only functions deleted; header comment and `usage()` updated. Launch/stop behavior byte-identical.
- Modify: `scripts/gate-run.md` — usage/stdout-payload/exit-code rows for `--observe` replaced by the refusal; the six-state table and § *The caller's loop* rewritten to the native JSON contract; jq documented as a required dependency of the loop.
- Modify: `skills/docket-build/SKILL.md` — reconciling sentence deleted; posture restated in the JSON vocabulary and native-gate invocation.
- Regenerate: `internal/assets/embedded/tree/skills/docket-build/SKILL.md` (via `go generate ./internal/assets/`).
- Modify: `tests/test_gate_run.sh` — refusal asserts; observe-behavior sections whose premise died are removed; contract-page asserts retargeted; new fence-executing loop harness (scripted-JSON stub + real native gate) with mutation-tested fail-closed arms.
- Modify: `tests/test_gate_run_stop.sh` — `gate_run --observe` oracle replaced by direct record reads.
- Modify: `tests/test_gate_execution_posture.sh` — asserts pinned to the retired `state=` prose retargeted to the rewritten posture.
- Modify: `tests/runtime-budgets.tsv` — re-measured row for `tests/test_gate_run.sh` (the real-gate leg builds a binary and launches real runs).
- No change: `internal/app/gate.go`, `internal/cli/gate_test.go` (spec: no shape change expected), `scripts/docket.sh` (`gate-run` stays in `WRAPPED_OPS` for launch/stop), `scripts/lib/docket-liveness.sh`, `scripts/runner-dispatch.*`.

---

### Task 0: Enumerate the text contract's executable dependents (no commit)

**Files:** none modified — this is the derive-don't-hand-list gate for every later task.

**Interfaces:**
- Produces: the authoritative list of files each later task edits. The lists inside Tasks 1–6 were derived this way at plan time; this step re-derives them so drift since plan time is caught before any edit.

- [ ] **Step 1: Re-derive the dependent set with whole-repo greps**

Run from the feature worktree root:

```bash
grep -rn -- 'gate-run --observe' . --include='*' -l | grep -v '^\./\.git'
grep -rln 'state=passed\|state=running\|state=died\|state=unavailable' tests/ scripts/ skills/ internal/assets/
```

Expected executable dependents (sort any newcomer into prose vs executable before proceeding):
`scripts/gate-run.sh`, `scripts/gate-run.md` (fence — executable per learnings: agent-executed-markdown-is-code), `tests/test_gate_run.sh`, `tests/test_gate_run_stop.sh`, `tests/test_gate_execution_posture.sh`, `skills/docket-build/SKILL.md` (+ its embedded copy under `internal/assets/embedded/tree/`). `scripts/docket-status.sh` and `scripts/render-learnings-index.sh` hits are unrelated `state=` spellings (verify by eye and leave untouched). Hits under `docs/changes/`, `docs/superpowers/`, and `docs/adrs/` are point-in-time records — never edit them.

- [ ] **Step 2: Confirm the two spec corrections against source**

```bash
grep -n 'StateSignaled\|StateVanished\|StateRunning' internal/process/process.go
grep -n '"json", false' internal/cli/root.go
```

Expected: the six-state vocabulary with `signaled`/`vanished` (no `died`), and the persistent `--json` flag. If either read fails, stop and re-reconcile before building — every later task bakes these in.

---

### Task 1: Retire `--observe` in `gate-run.sh` and prune the premise-dead observe tests

**Files:**
- Modify: `scripts/gate-run.sh`
- Test: `tests/test_gate_run.sh`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: `gate-run.sh --observe <anything>` → exit 2, empty stdout, one stderr line containing the literal `docket gate observe`. All other verbs behaviorally unchanged. Tasks 2–5 rely on the refusal shape and on launch/stop still working.

- [ ] **Step 1: Write the failing refusal asserts**

In `tests/test_gate_run.sh`, replace the whole `--observe` behavior region — everything from the comment banner `# ---- --observe: SIX STATES, AND THE READ ORDER THAT MAKES THEM HONEST ----` through the end of the observe argument-error block (the `reap "$argrd_pgid"` line after `obs_arg_case "a valueless --reason" --reason`) — with:

```bash
# ---- --observe IS RETIRED (change 0338): one refusal shape, never a state line ---------------
# The observe operation's single serialization is the native gate's protocol-v1 JSON
# (`docket gate observe <run-dir> --json`); this helper refuses the verb so the plain-text
# `state=<name>` contract cannot be revived by a caller that still spells the old invocation.
# The read-order/identity/liveness properties the deleted sections here used to pin now live with
# the native gate (internal/process/observe_test.go, internal/cli/gate_test.go) — deleted as a
# coverage MOVE, not a loss (learnings: test-premise-deleted-not-regated).
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30')"
ref_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
ref_out="$(gate_run --observe "$RD" 2>"$SBX/refusal.err")"; ref_rc=$?
assert "--observe refuses with a non-zero exit" '[ "$ref_rc" != "0" ]'
# NEGATIVE FIRST (learnings: assert-detects-removal-not-replacement): the retired serialization
# must be ABSENT — an empty protocol channel, not any state line. Mutation: restore the old
# do_observe dispatch and this reddens on the state=running it prints.
assert "--observe prints NOTHING on stdout — the state= serialization is gone" \
  '[ -z "$ref_out" ]'
assert "the refusal is one stderr line pointing at the native gate" \
  '[ "$(grep -c . "$SBX/refusal.err")" = "1" ] && grep -qF -- "docket gate observe" "$SBX/refusal.err"'
# The refusal must not have cost the run anything: observe was read-only and the refusal is too.
assert "the refusal signalled nothing — the run is still live" 'kill -0 -"$ref_pgid" 2>/dev/null'
reap "$ref_pgid"
```

Also delete these now-premise-dead regions of `tests/test_gate_run.sh` in the same edit (each drove `gate_run --observe`; their guarded properties belong to the native gate's own Go tests, or are re-pinned by later tasks):

- the observe-TOCTOU barrier race block (`# ---- THE OBSERVE TOCTOU RACE …` through its final `state=passed` assert),
- the barrier-inertness block (`# ---- THE BARRIER IS ENV-GATED AND INERT BY DEFAULT …` through `reap "$inert_pgid"`) — the barrier function itself stays in `gate-run.sh` (stop still uses it; `tests/test_gate_run_stop.sh` keeps its own inertness coverage),
- the identity-guard observe fixtures (`# ---- PROPERTY 2, THE IDENTITY GUARD …` and the two identity-source blocks after it, through `reap "$blank_pgid"`),
- the mid-flight-abort block (`# ---- AN observe_state THAT DIES MID-FLIGHT …` through `reap "$abort_pgid"`),
- the unusable-pgid observe loop (`# ---- A RECORD THAT CANNOT NAME A GROUP …` through `reap "$unusable_pgid"`).

Keep untouched: everything above the six-states banner (launch, records, detachment, wedges, failure path, terminal record) and everything from `lnch_arg_case` onward for launch; the contract-page assert region is retargeted in Task 4, not here — but the six-state-table asserts, the old caller-loop fence asserts, and the old `run_loop` harness (from `# ---- THE SIX STATES, DERIVED FROM THE TABLE …` and `# ---- THE CALLER'S LOOP …` through the fixture-8 assert `…"ERRExit|0"`) must be deleted **in this task** too, because Step 4's green run cannot survive them once Task 3 rewrites nothing yet — they still pass now, but they pin the exact prose Task 3 deletes; removing them here with the observe sections keeps each later task's suite state green. Also keep the "retryable rule stated twice" block and everything after it for now (Task 4 retargets what reddens).

- [ ] **Step 2: Run the test to verify the new asserts fail**

Run: `bash tests/test_gate_run.sh`
Expected: FAIL — `--observe` still prints `state=running` on stdout and exits 0, so "prints NOTHING on stdout" and "non-zero exit" go red.

- [ ] **Step 3: Implement the refusal in `scripts/gate-run.sh`**

Delete these observe-only functions (verified sole consumers at plan time: nothing outside the observe path calls them — `do_observe`, and via it `observe_state`, which alone calls `classify_record`, `log_tail_to_stderr`, and `group_alive_and_ours`):

- `classify_record`
- `log_tail_to_stderr`
- `observe_state`
- `do_observe`
- `group_alive_and_ours`

Keep `recorded_identity`, `recorded_pgid`, `signalable_pgid`, `identity_matches`, `record_field`, `barrier`, and every launch/stop function — `--stop` consumes them. Replace the verb dispatch's observe arm:

```bash
  --observe) die "the --observe verb is retired (change 0338); observe the native gate instead: docket gate observe <run-dir> --json"
             exit 2 ;;
```

Update the header comment's first line to `# scripts/gate-run.sh — detached launch / identity-checked stop for one long-running child process.` and add one line below it: `# The observe operation moved to the native gate (docket gate observe --json); --observe refuses with a pointer.` Update `usage()`:

```bash
usage() {
  printf '%s\n' \
    'usage: gate-run.sh --launch [--root <dir>] [--run-name <name>] -- <command…>' \
    '       gate-run.sh --stop <run-dir> [--reason <text>]' \
    '       (--observe is retired; use: docket gate observe <run-dir> --json)' \
    'Contract: scripts/gate-run.md' >&2
}
```

Do not touch `do_launch`, `do_wrap`, `do_stop`, `stop_run`, `terminal_record_token`, or the liveness lib sourcing.

- [ ] **Step 4: Run the file and the stop file**

Run: `bash tests/test_gate_run.sh` — Expected: PASS (refusal asserts green, launch/terminal/contract sections still green).
Run: `bash tests/test_gate_run_stop.sh` — Expected: FAIL (its oracle still calls `gate_run --observe`). That red is Task 2's starting state; do not fix it here.

- [ ] **Step 5: Mutation-test the refusal**

Temporarily re-point the `--observe` arm at a stub that prints `state=running` and exits 0; run `bash tests/test_gate_run.sh`; confirm the refusal asserts redden; revert the mutation by re-applying your own edit (keep a copy of the edited block first — `git checkout` would restore HEAD and destroy the task's work; learnings: mutation-restore-needs-a-backup-copy).

- [ ] **Step 6: Commit**

```bash
git add scripts/gate-run.sh tests/test_gate_run.sh
git commit -m "feat(0338): retire gate-run.sh --observe to a native-gate refusal"
```

(The suite as a whole is red at this commit boundary only in `tests/test_gate_run_stop.sh`, which the next task's commit repairs; the two tasks are split so a reviewer can reject the oracle migration without rejecting the refusal. If your build discipline requires green-at-every-commit, squash Tasks 1 and 2 into one commit at Task 2 Step 4 instead.)

---

### Task 2: Migrate `tests/test_gate_run_stop.sh` off the retired oracle

**Files:**
- Test: `tests/test_gate_run_stop.sh`

**Interfaces:**
- Consumes: Task 1's refusal (any `gate_run --observe` call now exits 2 with empty stdout).
- Produces: a green stop suite whose oracles are the run dir's own records. No later task touches this file.

- [ ] **Step 1: Replace every `gate_run --observe` oracle with a direct record read**

For each call site (find them all: capture `sites="$(grep -n -- 'gate_run --observe' tests/test_gate_run_stop.sh)"` — every one must be gone at the end), ask what the surrounding block *guards* (stop's write behavior, never observe's classification — the classification retired with the verb) and substitute:

| Old oracle | Replacement |
|---|---|
| `[ "$(gate_run --observe "$RD")" = "state=stopped" ]` after an ordinary live-child stop | `[ -f "$RD/stopped" ] && grep -q "^kind=signal" "$RD/terminal"` — the annotation plus our own signal's record, which is exactly what step 6 writes |
| `[ "$(gate_run --observe "$RD")" = "state=stopped" ]` after the KILL-escalation leg | `[ -f "$RD/stopped" ] && [ ! -f "$RD/terminal" ]` — the marker with no record, which is the leg's defining state |
| `[ "$(gate_run --observe "$RD")" = "state=passed" ]` (record outranks the stop) | `[ "$(cat "$RD/terminal" 2>/dev/null)" = "kind=exit code=0" ] && [ ! -f "$RD/stopped" ]` |
| `[ "$(gate_run --observe "$RD" 2>/dev/null)" = "state=died cause=vanished" ]` | `[ ! -f "$RD/terminal" ] && [ ! -f "$RD/stopped" ]` (the adjacent group-gone assert each such block already carries stays as the liveness half) |
| the `stop-intent`-beside-`kind=signal` reads-`stopped` blocks | `[ -f "$RD/stop-intent" ] && grep -q "^kind=signal" "$RD/terminal"` — pins the intent write the block exists to guard; the *classification* of intent-as-ours now belongs to the native gate |

Update each replaced assert's label to say what it now reads (e.g. "the stop annotated its own signal's record" instead of "observes as stopped"), and add one comment at the first replacement:

```bash
# The --observe oracle retired with change 0338 (the state=<name> serialization is gone), so every
# verdict here is read off the run dir's own records — the same files observe classified. What each
# assert GUARDS is unchanged: --stop's marker writes and their ordering.
```

- [ ] **Step 2: Run to green**

Run: `bash tests/test_gate_run_stop.sh`
Expected: PASS. If any block turns out to have guarded *only* observe's classification (nothing about stop's writes), delete it with a one-line comment naming where the coverage lives now (`internal/process/stop_test.go` / `observe_test.go`).

- [ ] **Step 3: Mutation-test one representative replacement**

In `scripts/gate-run.sh`'s `stop_run` step 6, temporarily skip the `atomic_write "$rd/stopped" …` annotation; run the stop file; confirm the ordinary-live-child-stop assert reddens; restore the line exactly (backup copy first, never `git checkout`).

- [ ] **Step 4: Commit**

```bash
git add tests/test_gate_run_stop.sh
git commit -m "test(0338): read stop verdicts off the run records, not the retired --observe"
```

---

### Task 3: Rewrite `scripts/gate-run.md` — the JSON caller's loop, the refusal, the jq dependency

**Files:**
- Modify: `scripts/gate-run.md`

**Interfaces:**
- Consumes: Task 1's refusal shape (documented here).
- Produces: the normative fence Task 4 extracts and executes verbatim, and these exact spellings later tasks rely on: heading `### The caller's loop` (unchanged); fence language `bash`; loop-resolved dispositions `passed` / `failed` / `died` / `stopped` / `unavailable` in variable `state`, with `cause` set beside `died`; the named diagnostic string `jq not found — the gate observe loop requires it`.

- [ ] **Step 1: Rewrite the page**

Apply all of the following edits (the fence below is normative — Task 4 executes these exact bytes):

**(a) Usage block and verb list.** Remove the `gate-run.sh --observe <run-dir>` usage line and the `--observe` bullet. In their place, after the `--launch` bullet, add:

```markdown
- `--observe` — **retired (change 0338).** The observe operation has exactly one serialization:
  the native gate's protocol-v1 JSON, read as `docket gate observe <run-dir> --json`. Invoking
  this verb refuses with a non-zero exit and a one-line stderr pointer to that command; nothing
  is printed on stdout. There is deliberately no passthrough shim — a second spelling of the same
  observation is the drift this retirement closes.
```

**(b) stdout-payload table.** Delete the `--observe` row (`state=<state>`, or `state=died cause=<signal\|vanished>`). The table keeps its `--launch` and `--stop` rows.

**(c) The six-state table.** Delete the `| State | Meaning | Retryable |` table and the paragraphs that read it (`**Only `running` is retryable.**` … "polling a decided run", and the malformed-`terminal` paragraph) from § Behavior's `### --observe` subsection; replace the whole `### --observe` subsection with:

```markdown
### `--observe` (retired)

The verb refuses: non-zero exit, empty stdout, one stderr line pointing at
`docket gate observe <run-dir> --json`. The observation itself — read order, identity-checked
liveness, the state vocabulary — is the native gate's contract now (`internal/app/gate.go`), and
this page no longer restates it. What a caller does with each observed state lives with the loop
below and in `skills/docket-build/SKILL.md` § *Gate execution posture*.
```

**(d) Exit-codes section.** Remove `--observe` from the two-value mapping's prose (`--stop` keeps it) and add one line: `` `--observe` exits `2` with nothing on stdout — a refusal, not a verdict. ``

**(e) § The caller's loop.** Replace the section's fence and its surrounding prose with:

````markdown
### The caller's loop

The helper never polls for you, and since change 0338 it does not observe for you either: the
loop drives the **native gate** directly and parses its protocol-v1 JSON with **jq — a required
dependency of this loop** (already a docket dependency elsewhere: `scripts/ensure-docket-env.sh`,
`scripts/docket-status.sh`). A missing jq is a loud terminal diagnostic, never a silent spin.
Copy this loop verbatim; `tests/test_gate_run.sh` extracts this fence and executes it against
scripted documents and against the real gate.

```bash
# `run_dir` is the run directory `docket gate launch` reported. GATE_OBSERVATION_BUDGET is the
# docket execution policy from the Step-0 config export, in minutes; 0 is legal and buys exactly
# one observation. The `:?` is load-bearing: bash arithmetic reads an unset name as 0, so a bare
# read would make a MISSING export look like a configured 0 and halt a healthy run one
# observation in.
deadline=$(( $(date +%s) + ${GATE_OBSERVATION_BUDGET:?from the Step-0 config export} * 60 ))
state="" cause=""
while :; do
  # The loop's one hard dependency, checked where it is used: without jq no document can be
  # read, so the only honest answer is a LOUD terminal unavailable — never a poll-again.
  if ! command -v jq >/dev/null 2>&1; then
    printf '%s\n' "jq not found — the gate observe loop requires it" >&2
    state=unavailable; break
  fi
  # Capture, THEN parse. The `|| true` is load-bearing: observe exits non-zero for real
  # verdicts too (failed, and every interrupted state), and the rule is that callers key on the
  # document, never the exit code — without it an errexit caller dies before any arm runs.
  doc="$(docket gate observe "$run_dir" --json)" || true
  st="$(jq -r '.state // empty' <<<"$doc" 2>/dev/null)" || st=""
  case "$st" in
    running) : ;;                                  # the only retryable state
    passed|failed|stopped) state="$st"; break ;;
    signaled|vanished)                             # the JSON spellings of a death: the child
      state=died                                   # never finished, so this is never `failed`
      cause="$(jq -r '.cause // empty' <<<"$doc" 2>/dev/null)" || cause=""
      break ;;
    *)                                             # empty state, garbled document, jq failure:
      printf '%s\n' "gate observe returned no recognizable state; failing closed as unavailable" >&2
      state=unavailable; break ;;                  # fail closed, NEVER a retry arm
  esac
  [ "$(date +%s)" -lt "$deadline" ] || break       # budget spent; `state` stays empty
  sleep 10
done
# An empty `state` means the budget ran out with the run still `running` — the fail-closed case,
# not a verdict about the child. The child was last seen live, so a caller abandoning here calls
# `docket gate stop "$run_dir"` before it reports (`skills/docket-build/SKILL.md`
# § *Gate execution posture*, *Abandoning a live child*).
```

**Never re-derive the state by hand from the document.** A grep or `cut` over the JSON re-creates
exactly the parser drift this loop's jq extraction retired; the document is parsed, or the arm is
the fail-closed one. The **unknown-document arm is terminal, never a retry**: a document outside
the vocabulary means the invocation or the environment is wrong, so the loop stops polling and
disposes it as `unavailable`. A retry there is precisely the shape that never terminates — it is
the 0337 incident.

The loop RESOLVES the native spellings into the caller's disposition vocabulary: `signaled` and
`vanished` both resolve to `died` (with `cause` carrying the document's own qualifier, possibly
empty), because a signalled or vanished child **never finished** — `died` is never `failed`, and
only `failed` may feed repair work. What to *do* with each resolved state is the caller's policy:
dispositions are stated in `skills/docket-build/SKILL.md` § *Gate execution posture*.
````

**(f) Purpose section.** Update the third property bullet from "**Only `running` is retryable.** The other five states are terminal." to "**Only `running` is retryable.** Every other observed state is terminal — the vocabulary and its reading now live with the native gate and the loop above." Update the second bullet's `--observe` mention ("`--observe` is the only verb that is a short call that returns") to name `docket gate observe` instead.

**(g) Tests section.** Reword to name the retargeted coverage: refusal + loop regression in `tests/test_gate_run.sh`, stop coverage in `tests/test_gate_run_stop.sh` reading the records directly.

Whole-page sweep afterward: `grep -n 'state=' scripts/gate-run.md` must return no observe-serialization hits (the run-directory layout block's record spellings `kind=exit code=<n>` etc. are not `state=` lines and stay).

- [ ] **Step 2: Sanity-run the fence by hand once**

From the worktree root, with the repo's installed `docket` on PATH:

```bash
rd="$(docket gate launch --root "${TMPDIR:-/tmp}/g338.XXXXXX-manual" --cwd "$PWD" -- /bin/sh -c 'exit 0' --json | jq -r '.run_dir')"
```

(Adjust to the launch verb's real flag spelling if it differs — read `internal/cli/gate.go`'s launch `Use:` line; the fence itself never launches, so only this manual probe cares.) Then paste the fence with `run_dir="$rd"` and `GATE_OBSERVATION_BUDGET=1` into a `bash -euo pipefail` invocation and confirm it resolves `state=passed` within one or two observations. This is a smoke check only — Task 4 is the real harness.

- [ ] **Step 3: Run the two gate test files**

Run: `bash tests/test_gate_run.sh && bash tests/test_gate_run_stop.sh`
Expected: PASS — Task 1 already removed the asserts that pinned the deleted prose. If anything reddens, it is a contract assert Task 1 should have removed; remove it now with the same premise-died justification.

- [ ] **Step 4: Commit**

```bash
git add scripts/gate-run.md
git commit -m "docs(0338): gate-run.md — native-JSON caller loop, jq dependency, --observe refusal"
```

---

### Task 4: Retarget `tests/test_gate_run.sh`'s contract asserts and execute the new fence against scripted JSON

**Files:**
- Test: `tests/test_gate_run.sh`

**Interfaces:**
- Consumes: Task 3's fence (extracted from `### The caller's loop`, language `bash`), its disposition vocabulary (`state` ∈ `passed|failed|died|stopped|unavailable|""`, `cause` beside `died`), and its two named diagnostics.
- Produces: `run_loop` harness (signature below) that Task 5's real-gate leg reuses: `run_loop <budget-minutes|UNSET> <mode> <line…>` → prints `<state>|<observation-count>` (and `<state>|<cause>|<count>` in cause mode — see Step 2).

- [ ] **Step 1: Retarget the contract-page asserts**

In the contract region of `tests/test_gate_run.sh` (from `# ---- THE CONTRACT, THE FACADE WIRING, AND THE BUDGET ROWS ----`):

Keep unchanged: the facade `WRAPPED_OPS` assert, the page-existence and non-vacuity floor, the section-heading loop (all eight headings survive the rewrite), the `csection`/`cfrom`/`flat` slicers, the Purpose/platform/residuals/invariants/terminal-record/layout/which-leg/exit-codes asserts, and the budget-row asserts.

Delete: the six-state-table extraction (`states_tbl=` block through the `retryable_rows` assert) and — if Task 1 left any of it — the old caller-loop asserts and `run_loop` harness. Replace the "and the section that defines the states states it there too" assert (its haystack `cfrom '### `--observe`'` now slices the retirement stub) with:

```bash
observe_blk="$(cfrom '### `--observe`')"
observe_flat="$(flat "$observe_blk")"
assert "the --observe section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$observe_blk")" -ge 4 ]'
# NEGATIVE FIRST: the page must no longer publish a state=<state> stdout payload for observe —
# the serialization this change retires. Scoped to the payload table so body prose about records
# cannot satisfy or violate it. Mutation: restore the old table row and this reddens.
payload_tbl="$(awk '/^\| Verb \| stdout payload \|/{f=1} f && /^\|/{print} f && !/^\|/{f=0}' <<<"$contract")"
assert "the stdout-payload table was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$payload_tbl")" -ge 3 ]'
assert "the payload table carries no observe state= row any more" \
  '! grep -qF -- "state=" <<<"$payload_tbl"'
assert "the retirement points at the native invocation, json flag included" \
  'grep -qF -- "docket gate observe" <<<"$observe_flat" && grep -qF -- "--json" <<<"$observe_flat"'
```

- [ ] **Step 2: Rebuild the fence-execution harness for JSON documents**

Where the old `LOOPBOX` harness stood, install (adapting the proven 0286 shape — simulated clock, `set -euo pipefail`, capped stub per learnings: mutation-target-needs-a-forced-exit):

```bash
# ---- THE CALLER'S LOOP IS A TAUGHT, EXECUTABLE SURFACE (0286 shape, 0338 serialization) --------
# The loop now parses the native gate's protocol-v1 JSON with jq. An agent runs the fence's bytes
# verbatim (learnings: agent-executed-markdown-is-code), so these asserts EXECUTE the fence.
usage_blk="$(csection Usage)"
assert "the contract carries a caller-loop subsection, inside Usage" \
  'grep -qxF -- "### The caller'"'"'s loop" <<<"$usage_blk"'
loop_sec="$(awk '
  /^### The caller'"'"'s loop$/ {f=1; next}
  !f {next}
  /^```/ {inf = 1 - inf; print; next}
  !inf && /^#+ / {f=0; next}
  {print}
' <<<"$contract")"
assert "the caller-loop section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$loop_sec")" -ge 20 ]'
loop_fence="$(awk '/^```bash$/ {inf=1; next}  inf && /^```$/ {inf=0; next}  inf {print}' <<<"$loop_sec")"
assert "the canonical loop fence was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$loop_fence")" -ge 15 ]'
loop_flat="$(flat "$loop_sec")"
# jq is a DOCUMENTED required dependency, bound to the loop rather than merely mentioned
# (learnings: prose-guard-binds-phrase-to-claim). Mutation: drop the dependency sentence.
assert "the section documents jq as a required dependency of the loop" \
  'grep -qiE "jq[^.]{0,80}required dependency" <<<"$loop_flat"'
assert "and it names the unknown-document arm as terminal, never a retry" \
  'grep -qiE "unknown[^.]{0,140}(never a retry|stop[s]? polling)" <<<"$loop_flat"'
assert "the section defers disposition policy to the build skill's posture" \
  'grep -qF -- "Gate execution posture" <<<"$loop_sec"'
assert "the section names the mandatory stop on the abandon-while-running leg" \
  'grep -qiE "abandon[^.]{0,100}gate stop[^.]{0,60}before it reports" <<<"$loop_flat"'

LOOPBOX="$SBX/loopbox"; mkdir -p "$LOOPBOX/bin"
cat >"$LOOPBOX/bin/docket" <<'STUB'
#!/usr/bin/env bash
# Stub native gate: answers the Nth line of $OBS_SCRIPT as a whole protocol document, repeating
# the last line forever. Mirrors the real exit mapping (internal/app/result.go ExitCode +
# gate.go mapObservation): running/passed -> 0, everything else -> 1. Past the cap it answers a
# vocabulary-outside document so a mutated, never-terminating fence resolves to a comparable
# `unavailable|201` instead of hanging the suite (learnings: mutation-target-needs-a-forced-exit).
n=$(( $(cat "$OBS_COUNT") + 1 )); printf '%s' "$n" >"$OBS_COUNT"
[ "$n" -le 200 ] || { printf '{"protocol_version":1,"operation":"gate.observe","result":"internal-error","state":"LOOPCAP"}\n'; exit 1; }
line="$(sed -n "${n}p" "$OBS_SCRIPT")"
[ -n "$line" ] || line="$(sed -n '$p' "$OBS_SCRIPT")"
printf '%s\n' "$line"
case "$line" in
  *'"state":"running"'*|*'"state":"passed"'*) exit 0 ;;
  *) exit 1 ;;
esac
STUB
chmod +x "$LOOPBOX/bin/docket"
printf '%s\n' "$loop_fence" >"$LOOPBOX/loop.body"

# Runs the byte-unmodified fence under `set -euo pipefail` with a SIMULATED clock (sleep advances
# a counter; date reports it), exactly the 0286 harness shape. $2 selects the PATH: `jq` keeps the
# real PATH (jq present) behind the stub dir; `nojq` restricts PATH to the stub dir alone, which
# is the simulated jq-absent machine. Output: `<state>|<cause>|<count>`.
run_loop(){ # $1 = budget minutes or UNSET; $2 = jq|nojq; $3… = scripted observe documents
  local budget="$1" pathmode="$2"; shift 2
  printf '%s\n' "$@" >"$LOOPBOX/script"
  printf '0' >"$LOOPBOX/count"
  {
    if [ "$budget" = UNSET ]; then printf '%s\n' 'set -eo pipefail'
    else                           printf '%s\n' 'set -euo pipefail'; fi
    if [ "$pathmode" = nojq ]; then printf 'PATH=%q\n' "$LOOPBOX/bin"
    else                            printf 'PATH=%q\n' "$LOOPBOX/bin:$PATH"; fi
    printf '%s\n' '__now=0' \
      'date(){ printf "%s\n" "$__now"; }' \
      'sleep(){ __now=$(( __now + ${1:-0} )); }' \
      'run_dir=/nonexistent-run-dir'
    [ "$budget" = UNSET ] || printf '%s\n' "GATE_OBSERVATION_BUDGET=$budget"
    cat "$LOOPBOX/loop.body"
    printf '%s\n' 'printf "%s|%s" "${state}" "${cause:-}"'
  } >"$LOOPBOX/harness.sh"
  local st
  st="$(OBS_SCRIPT="$LOOPBOX/script" OBS_COUNT="$LOOPBOX/count" \
        "$DOCKET_BASH_PATH" "$LOOPBOX/harness.sh" 2>"$LOOPBOX/harness.err")" || st="ERRExit|"
  printf '%s|%s' "$st" "$(cat "$LOOPBOX/count")"
}
J='{"protocol_version":1,"operation":"gate.observe","result":"applied","state":"running"}'
P='{"protocol_version":1,"operation":"gate.observe","result":"applied","state":"passed"}'
F='{"protocol_version":1,"operation":"gate.observe","result":"gate-failed","state":"failed","exit_code":1}'
S='{"protocol_version":1,"operation":"gate.observe","result":"interrupted","state":"stopped"}'
G='{"protocol_version":1,"operation":"gate.observe","result":"interrupted","state":"signaled","cause":"terminated"}'
V='{"protocol_version":1,"operation":"gate.observe","result":"interrupted","state":"vanished"}'
E='{"protocol_version":1,"operation":"gate.observe","result":"invalid-state","reason":"observe: no run at that path"}'
```

- [ ] **Step 3: Write the fixture asserts, each with a named mutation key**

```bash
# 1 — terminal states dispose in one observation, in the loop's own vocabulary.
assert "the loop disposes a passed document as passed, in one observation" \
  '[ "$(run_loop 5 jq "$P")" = "passed||1" ]'
assert "the loop disposes a failed document as failed (despite its exit-1 transport)" \
  '[ "$(run_loop 5 jq "$F")" = "failed||1" ]'   # mutation key: drop the fence's `|| true` -> ERRExit
assert "the loop disposes a stopped document as stopped" \
  '[ "$(run_loop 5 jq "$S")" = "stopped||1" ]'
# 2 — running is the ONLY retryable state, and the retry actually happens.
assert "the loop retries running and takes the next document's verdict" \
  '[ "$(run_loop 5 jq "$J" "$J" "$F")" = "failed||3" ]'
# 3 — THE VOCABULARY CORRECTION THE SPEC'S SKETCH MISSED: the native gate spells a death
# signaled/vanished, never died. Both must resolve to the died disposition, cause carried —
# an arm that leaves them to `*)` reads every real signal death as unavailable.
assert "a signaled document resolves to the died disposition, cause extracted" \
  '[ "$(run_loop 5 jq "$G")" = "died|terminated|1" ]'
assert "a vanished document resolves to died too, with an empty cause" \
  '[ "$(run_loop 5 jq "$V")" = "died||1" ]'
# 4 — THE DEFECT THIS CHANGE EXISTS FOR, in each garbled shape: disposed in ONE observation,
# never polled. Mutation key: rewrite `*)` into a retry arm -> both halves invert (and the
# LOOPCAP stub bounds the mutated run at unavailable||201 instead of a hang).
assert "the loop fails closed on a stateless failure document, in exactly one observation" \
  '[ "$(run_loop 5 jq "$E")" = "unavailable||1" ]'
assert "the loop fails closed on a non-JSON line" \
  '[ "$(run_loop 5 jq "hello world")" = "unavailable||1" ]'
assert "the loop fails closed on an empty line" \
  '[ "$(run_loop 5 jq "")" = "unavailable||1" ]'
assert "the fail-closed diagnostic is loud, on stderr" \
  'run_loop 5 jq "$E" >/dev/null; grep -qiF -- "failing closed" "$LOOPBOX/harness.err"'
# 5 — jq ABSENT is a NAMED terminal diagnostic before any observation, never a silent spin.
# Mutation key: delete the fence's `command -v jq` check -> the count reads 1 (the doc was
# fetched) and the named diagnostic vanishes; both asserts redden.
assert "a jq-less PATH resolves unavailable with zero observations" \
  '[ "$(run_loop 5 nojq "$P")" = "unavailable||0" ]'
assert "and the diagnostic names jq by the contract's own words" \
  'run_loop 5 nojq "$P" >/dev/null; grep -qF -- "jq not found — the gate observe loop requires it" "$LOOPBOX/harness.err"'
# 6 — budget semantics, unchanged from 0286. Mutation keys: (a) drop the deadline check -> the
# running fixture resolves unavailable||201 off the stub cap; (b) drop the `:?` -> UNSET reads "||1".
assert "a zero budget buys one observation and reports no verdict" \
  '[ "$(run_loop 0 jq "$J")" = "||1" ]'
assert "a running run that never settles stops at the budget with no verdict" \
  '[ "$(run_loop 5 jq "$J")" = "||31" ]'
assert "an unset budget aborts the loop instead of passing for a configured zero" \
  '[ "$(run_loop UNSET jq "$J")" = "ERRExit||0" ]'
```

(If the observation counts differ by one from the fence's final shape, fix the *expected values* from a hand-trace of the fence, never by loosening an assert to a range.)

- [ ] **Step 4: Run the file; then run the four mutations**

Run: `bash tests/test_gate_run.sh` — Expected: PASS.

Then, one at a time against `scripts/gate-run.md`'s fence (backup copy first; restore by re-applying the saved bytes, never `git checkout`):
1. Delete the `command -v jq` block → asserts 5 redden (`unavailable||1`-shaped count and missing named diagnostic).
2. Rewrite `*)` to `: ;;` (retry) → asserts 4 redden with `unavailable||201`, quickly, not a hang.
3. Delete `|| true` on the capture → the failed-document assert reads `ERRExit`.
4. Delete `signaled|vanished)`'s arm → assert 3 reddens (`unavailable||1`).
5. Delete the `:?` from the budget read → the UNSET assert reads `||1`.

Each must redden; a mutation that stays green is a defect to fix before proceeding (guards-are-code).

- [ ] **Step 5: Commit**

```bash
git add tests/test_gate_run.sh
git commit -m "test(0338): execute the JSON caller loop fence with mutation-keyed fail-closed arms"
```

---

### Task 5: Drive the fence against the real native gate through each terminal state

**Files:**
- Test: `tests/test_gate_run.sh` (extend the harness region)
- Modify: `tests/runtime-budgets.tsv` (the `tests/test_gate_run.sh` row)

**Interfaces:**
- Consumes: Task 4's `run_loop` (mode `jq`; with the real binary first on PATH the stub is bypassed — see Step 1's `REALBIN` PATH note), the fence, and `manifest.json`'s `pgid` field (Verified fact 5).
- Produces: nothing later tasks consume.

- [ ] **Step 1: Build the binary once and add a real-gate runner**

Append to the harness region (before the budget-row asserts):

```bash
# ---- AND THE FENCE RUNS AGAINST THE REAL GATE'S OWN DOCUMENTS -------------------------------
# The scripted stub above proves the ARMS; this leg proves the documents are real — the fence's
# jq extraction against bytes internal/app/gate.go actually emits, through every terminal state.
# Built once, like tests/test_asset_bundle_drift.sh builds its comparator.
REALBIN="$SBX/realbin"; mkdir -p "$REALBIN"
build_out="$( (cd "$REPO" && go build -o "$REALBIN/docket" ./cmd/docket) 2>&1 )"
assert "the native gate binary builds" '[ -x "$REALBIN/docket" ] || { printf "%s\n" "$build_out" >&2; false; }'

# Launch through the REAL binary; wait until the run is terminal (cheap raw polls, budget-bounded
# by the loop below at 60 real iterations of 0.5s); then run the byte-unmodified fence ONCE with
# a fresh simulated clock, so each terminal state is read by the fence in exactly one observation.
real_loop(){ # $1 = run dir -> prints `<state>|<cause>`
  local rd="$1"
  {
    printf '%s\n' 'set -euo pipefail' \
      "PATH=$REALBIN:\$PATH" \
      '__now=0' 'date(){ printf "%s\n" "$__now"; }' 'sleep(){ __now=$(( __now + ${1:-0} )); }' \
      "run_dir=$rd" 'GATE_OBSERVATION_BUDGET=5'
    cat "$LOOPBOX/loop.body"
    printf '%s\n' 'printf "%s|%s" "${state}" "${cause:-}"'
  } >"$LOOPBOX/real-harness.sh"
  "$DOCKET_BASH_PATH" "$LOOPBOX/real-harness.sh" 2>/dev/null
}
real_launch(){ # $@ = child command -> prints the run dir
  PATH="$REALBIN:$PATH" docket gate launch --root "$SBX/native-runs" --cwd "$REPO" -- "$@" --json \
    | jq -r '.run_dir'
}
await_native_terminal(){ # $1 = run dir -> waits (real time) until observe stops saying running
  local i=0 st
  while [ "$i" -lt 60 ]; do
    st="$(PATH="$REALBIN:$PATH" docket gate observe "$1" --json 2>/dev/null | jq -r '.state // empty')" || st=""
    case "$st" in running|"") /bin/sleep 0.5; i=$(( i + 1 )) ;; *) return 0 ;; esac
  done
  return 1
}
```

(Two spellings above must be verified against the CLI before they are trusted, exactly once: the launch verb's flag names — read `internal/cli/gate.go`'s launch command `Use:`/flag definitions — and whether `--json` may trail the `--`-separated child argv; if it must precede, move it before `--`. Fix the helper, not the fence.)

- [ ] **Step 2: The five terminal states, for real**

```bash
# passed / failed — the child's own verdicts.
RDP="$(real_launch /bin/sh -c 'exit 0')"
assert "real launch handed back a run dir" '[ -d "$RDP" ]'
assert "the real run reached a terminal state" 'await_native_terminal "$RDP"'
assert "the fence reads the real passed document as passed" '[ "$(real_loop "$RDP")" = "passed|" ]'
RDF="$(real_launch /bin/sh -c 'exit 3')"
assert "the failed run reached a terminal state" 'await_native_terminal "$RDF"'
assert "the fence reads the real failed document as failed" '[ "$(real_loop "$RDF")" = "failed|" ]'
# stopped — the gate's own stop verb.
RDS="$(real_launch /bin/sh -c 'sleep 30')"
PATH="$REALBIN:$PATH" docket gate stop "$RDS" --json >/dev/null 2>&1 || true
assert "the stopped run reached a terminal state" 'await_native_terminal "$RDS"'
assert "the fence reads the real stopped document as stopped" '[ "$(real_loop "$RDS")" = "stopped|" ]'
# signaled — an external TERM of the real group, read from the run's own manifest.
RDG="$(real_launch /bin/sh -c 'sleep 30')"
sig_native_pgid="$(jq -r '.pgid' "$RDG/manifest.json")"
kill -TERM -"$sig_native_pgid" 2>/dev/null || true
assert "the signaled run reached a terminal state" 'await_native_terminal "$RDG"'
sig_read="$(real_loop "$RDG")"
assert "the fence resolves the real signaled document to died (got '$sig_read')" \
  '[ "${sig_read%%|*}" = "died" ]'
# vanished — KILL the whole group so no record can ever be written.
RDV="$(real_launch /bin/sh -c 'sleep 30')"
van_native_pgid="$(jq -r '.pgid' "$RDV/manifest.json")"
kill -KILL -"$van_native_pgid" 2>/dev/null || true
assert "the vanished run reached a terminal state" 'await_native_terminal "$RDV"'
van_read="$(real_loop "$RDV")"
assert "the fence resolves the real vanished document to died (got '$van_read')" \
  '[ "${van_read%%|*}" = "died" ]'
```

If the signaled/vanished fixtures prove racy against the real supervisor (e.g. the stop-vs-signal window), key the assert on the resolved disposition only — as written above — never on the cause string; and if `manifest.json`'s pgid field name differs, read it from `internal/process/records.go` and fix the extraction, not the assert.

- [ ] **Step 3: Run the file; re-measure the budget row**

Run: `bash tests/test_gate_run.sh` — Expected: PASS.

Then re-measure per `tests/runtime-budgets.tsv`'s own documented procedure (worst of 3 wall-clock runs → next multiple of 5, plus 5). The row currently reads `tests/test_gate_run.sh	20	parallel`; the Go build plus five real launches will move it. Update the number from the measurement — never guess — and record the three readings in a comment beside the row, matching the file's house style (learnings: budget-headroom-is-spent-before-it-is-breached; tolerance-constant-calibrated-on-one-machine — record the measurement, not just the number).

- [ ] **Step 4: Commit**

```bash
git add tests/test_gate_run.sh tests/runtime-budgets.tsv
git commit -m "test(0338): drive the caller loop against real native-gate JSON through every terminal state"
```

---

### Task 6: `skills/docket-build/SKILL.md` — delete the reconciling prose, restate the posture in JSON

**Files:**
- Modify: `skills/docket-build/SKILL.md`
- Regenerate: `internal/assets/embedded/tree/skills/docket-build/SKILL.md` (never hand-edit; `go generate ./internal/assets/`)
- Test: `tests/test_gate_execution_posture.sh`

**Interfaces:**
- Consumes: Task 3's loop vocabulary (`passed`/`failed`/`died`/`stopped`/`unavailable` as loop-resolved dispositions) and section pointer (`gate-run.md` § *The caller's loop*).
- Produces: nothing later tasks consume.

- [ ] **Step 1: Rewrite the posture's serialization prose**

In § *Gate execution posture* ("The shipped implementation of clauses 1–3" paragraph and the died-state block):

**(a)** Replace the sentence beginning "The landed Go-v1 gate is the native `docket gate launch` / `observe <run-dir>` / `stop <run-dir>` supervisor (its `observe` emits protocol-v1 JSON); the `gate-run` facade named here remains the observation-loop discipline a worker drives, whose `gate-run.md` `state=<name>` contract — not the native JSON — the caller's loop below keys on." with:

```markdown
Observation is the native gate's: each short-lived look is
`docket gate observe <run-dir> --json`, one protocol-v1 JSON document per call, parsed with jq —
the observe serialization since change 0338, and the only one. The plain-text `state=<name>`
observe contract is retired; `gate-run --observe` refuses with a pointer.
```

**(b)** Replace the sentence "**Reuse the canonical loop** in `gate-run.md` § *The caller's loop* verbatim rather than authoring one, and key each `case` arm on the full printed `state=<name>` line: a loop that re-tokenizes that line and matches bare state names matches nothing, so it never terminates on a state — it polls a finished gate until the budget is spent." with:

```markdown
**Reuse the canonical loop** in `gate-run.md` § *The caller's loop* verbatim rather than
authoring one: it captures the document, extracts `.state` with jq, resolves the native
spellings (`signaled`/`vanished` resolve to `died`), and fails closed — a hand-rolled reading
of the document is exactly the parser drift that spun the 0337 gate until a human resumed it.
```

**(c)** In the died-state block, rewrite every `state=<name>` spelling to the loop's resolved-state vocabulary: "`state=passed` or `state=failed` keep that verdict … `state=died` takes the one relaunch, `state=stopped` and `state=unavailable` never relaunch" becomes "an observed `passed` or `failed` keeps that verdict (the run finished after all), a `died` resolution (`signaled` or `vanished` in the document) takes the one relaunch, `stopped` and `unavailable` never relaunch". Rewrite the stop-token framing sentence ("every state named inside a bullet is written `state=<name>` for the value the verb `--observe` returns") to: "every state named inside a bullet is the loop's resolved reading of a fresh `docket gate observe <run-dir> --json` document". The stop verb the bullets describe becomes `docket gate stop <run-dir>` where the prose names an invocation; keep the disposition logic of each bullet byte-for-byte in meaning.

**(d)** Sweep: `grep -n 'state=' skills/docket-build/SKILL.md` must return zero hits afterward, and `grep -n 'not the native JSON' skills/docket-build/SKILL.md` must return zero hits (the reconciling clause is the thing this change kills — assert-detects-removal-not-replacement says check the absence, not the new wording).

- [ ] **Step 2: Regenerate the embedded copy**

Run: `go generate ./internal/assets/`
Then: `bash tests/test_asset_bundle_drift.sh` — Expected: PASS.

- [ ] **Step 3: Retarget `tests/test_gate_execution_posture.sh`**

Run: `bash tests/test_gate_execution_posture.sh`. For every reddened assert, relocate — never restore the deleted prose (learnings: restatement-accumulates-its-own-guards). Known dependents from plan-time reading (re-derive from the red run, these anchors may have drifted):

- the assert "helper: the keying rule is bound to the full printed state= form" (`grep -qiE "(key|match)[^.]{0,120}state=[^.]{0,60}(form|printed|line)"`) — replace with a binding to the new rule plus the negative that the old form is gone:

```bash
assert "helper: the loop is bound to the JSON document and its jq extraction" \
  'grep -qiE "docket gate observe[^.]{0,120}--json" <<<"$helper_flat" && grep -qiE "jq" <<<"$helper_flat"'
assert "helper: the retired state= keying no longer appears in the posture" \
  '! grep -qF -- "state=" <<<"$helper_flat"'
```

- the asserts pinning `docket.sh gate-run` as the shipped implementation (`grep -qE "docket\.sh gate-run"` in the helper and mitigation paragraphs) — launch and stop remain facade-reached until 0339, so these should stay green; if the rewrite moved the invocation out of the sliced paragraph, adjust the slice anchor, not the claim.
- the died-flow assert ("the already-terminal leg re-observes and keys on what returns") — its property survives; repoint its grep at the reworded bullet if the exact phrase moved.

Every replacement gets a mutation pass: temporarily restore clause (a)'s old sentence into SKILL.md, confirm the negative assert reddens, revert (backup copy, not `git checkout`).

- [ ] **Step 4: Run the neighboring guards**

Run: `bash tests/test_gate_execution_posture.sh && bash tests/test_skill_size_budgets.sh && bash tests/test_asset_bundle_drift.sh`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-build/SKILL.md internal/assets/embedded/tree/skills/docket-build/SKILL.md tests/test_gate_execution_posture.sh
git commit -m "feat(0338): docket-build posture keys on the native JSON observe; reconciling prose retired"
```

---

### Task 7: Whole-suite gate

**Files:** none new.

- [ ] **Step 1: Run the whole suite**

Run: `scripts/run-tests.sh` (background it to a stable log with a blocking monitor if it approaches the foreground ceiling; key on the exit code, per repo memory the suite runs near 600s).
Expected: PASS, and **no `OVER BUDGET:` trailing line** — if `tests/test_gate_run.sh` reads over budget, Task 5 Step 3's measurement was optimistic; re-measure and fix the row, do not shrug it off.

- [ ] **Step 2: Run the Go tests uncached as a final cross-check**

Run: `go test ./internal/... -count=1`
Expected: PASS with no diffs to `internal/app/gate.go` / `internal/cli/gate_test.go` in `git status` — the spec's "no shape change expected" holds; if you found yourself editing either, stop and revisit, that is a scope breach.

- [ ] **Step 3: Commit anything outstanding and confirm the branch state**

```bash
git status --short   # expect: clean
git log --oneline f94844d78be078c7d7c67afafe63026f0fe8ef7c..HEAD
```

Expected history: the plan commit plus the five task commits above, all on `feat/gate-execution-terminal-sentinel-has-no-format-contract-poll`.

---

## Self-Review (performed at plan time)

- **Spec coverage:** gate-run.sh refusal → Task 1; gate-run.md exit-table/output-contract removal, loop rewrite, jq dependency, refusal doc → Task 3; SKILL.md sentence deletion + vocabulary update → Task 6; test retargeting (a) refusal, (b) real-JSON loop regression, (c) mutation-tested fail-closed arm incl. jq-absent PATH → Tasks 1, 4, 5; Go files untouched → Task 7 Step 2 guard. Forced scope additions beyond the spec's file list, each entailed by the spec's own decisions: `tests/test_gate_run_stop.sh` (its oracle was the retired verb — Task 2), `tests/test_gate_execution_posture.sh` (it greps the SKILL prose being rewritten — Task 6), the embedded asset copy (drift-guarded generated artifact — Task 6), `tests/runtime-budgets.tsv` (the real-gate leg's cost — Task 5).
- **Spec corrections baked in:** the `--json` flag (reconcile log) and the `signaled`/`vanished`-not-`died` vocabulary (verified against `internal/process/process.go`; the fence resolves them to the `died` disposition, keeping the spec's "arm semantics unchanged" requirement true in meaning where its sketch was wrong in spelling).
- **Type/name consistency:** `run_loop <budget> <jq|nojq> <docs…>` → `state|cause|count` used identically in Tasks 4–5; disposition vocabulary `passed/failed/died/stopped/unavailable` consistent across fence, SKILL rewrite, and asserts; the named jq diagnostic string is byte-identical in Task 3's fence and Task 4's assert.
- **Placeholder scan:** none — every step carries its bytes or names the exact source symbol to read (`internal/cli/gate.go` launch flags, `internal/process/records.go` manifest fields) where a value must be read rather than assumed.
