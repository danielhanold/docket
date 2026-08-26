<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0334 — Make Docket dispatch minimal, non-recursive, and mechanically gated](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0334-claude-md-global-and-project-copies-out-of-sync-on-finalize-a.md)**
<!-- docket:backlink:end -->
# Minimal, Non-Recursive, Mechanically Gated Dispatch — Implementation Plan (change 0334)

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan
> task-by-task (routed multi-profile, no per-task review, single full-suite gate at the end).
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the always-loaded dispatch roster with a compact routing rule, inject an
exact-name self-recursion guard into every generated wrapper, and move the implement-next run
gate's attribution/retry mechanics behind a durable `gate-before`/`gate-verdict` facade.

**Architecture:** The facade is new Go `docket run gate-before` / `docket run gate-verdict`
subcommands over durable JSON records under `<git-common-dir>/docket/rungate/` (generalizing the
conventions of `scripts/lib/docket-dispatch-dir.sh`), delegating the run predicate to the existing
`internal/app.RunVerify`. The content changes (compact block, recursion guard, compact gate
payload) flow through the two lockstep generators — the Go adapters under `internal/harness/` and
the byte-identical shell mirror `sync-agents.sh` — from the authored payload sources under
`cursor-rules/` (re-embedded via `cmd/genassets`).

**Tech Stack:** Go (internal/app, internal/harness, cmd/docket), bash (sync-agents.sh, scripts/),
hermetic shell tests under tests/, Go golden tests under internal/harness/*/testdata.

**Spec:** `docs/superpowers/specs/2026-08-25-consolidate-dispatch-block-subagent-guard-design.md`
(on the `docket` metadata branch). Read it with the change file's "Implementation reality
(reconciled 2026-08-26)" section — the spec assumes shell generation; reality is Go generation
with a shell mirror.

## Global Constraints

- **Lockstep generators.** Every emitter/content change lands in BOTH the Go adapter(s) under
  `internal/harness/` AND `sync-agents.sh` in the same task, with both test surfaces (Go goldens +
  `tests/test_sync_agents_*.sh`) moving together. `./sync-agents.sh --check` must pass at the end
  of every content task.
- **Embedded tree is generated.** Never hand-edit `internal/assets/embedded/tree/**`. Edit the
  authored source (`cursor-rules/**`, `agents/**`) and regenerate with `go run ./cmd/genassets`;
  `tests/test_asset_bundle_drift.sh` enforces the tie.
- **No policy changes.** `verify-run`'s predicates (`scripts/verify-run.sh`,
  `internal/app/run_verify.go`) and the ADR-0075/0084/0088 safety policy are untouched. The facade
  relocates mechanics; it delegates the run predicate (learnings:
  duplicated-gate-copies-the-whole-predicate — never copy a predicate to a second site).
- **Guards are code.** Every new guard is mutation-tested: strip the guarded thing, watch the
  assert redden, restore. Go mutation probes run with `go test -count=1` (learnings:
  cached-runner-serves-a-mutated-tree). Prose greps collapse whitespace before matching
  (learnings: phrase-grep-over-wrapped-prose) and bind phrase to claim (learnings:
  prose-guard-binds-phrase-to-claim).
- **Deleting restated prose is never a one-file edit.** Before removing the roster or the
  hand-executed gate prose, grep the whole test suite for the prose being removed and relocate —
  not restore — every dependent assert (learnings: restatement-accumulates-its-own-guards).
- **Size direction.** The compact block must LOWER the always-loaded size bound: the new budget
  guard's ceiling must be strictly below the measured pre-change actual, with both numbers
  recorded (learnings: size-target-is-direction — direction made durable).
- **Suite at the gate.** The final gate runs the whole suite via `finalize.test_command`
  (`scripts/run-tests.sh`) plus `go test ./...`. Act on `SERIAL CONFIRMED OVER BUDGET:` lines.
- **Human merge-gate preconditions — NOT build tasks:** (1) removal of the hand-authored global
  `~/.claude/CLAUDE.md` Docket dispatch block; (2) the four-harness fresh-process external
  behavioral acceptance (Claude, Codex, Cursor, OpenCode). Repository tests cannot prove
  process-start artifacts behave (learnings: generated-artifact-loaded-at-process-start,
  external-truth-needs-a-human-checkpoint). The build must not touch `~/.claude/CLAUDE.md`.
- **Report lines, not exit codes.** Facade commands exit 0 whenever they produced a report line;
  non-zero is reserved for usage errors (learnings: exit-code-encodes-a-non-failure).

## Facade vocabulary (normative, from the spec — every task below uses these exact spellings)

Attributed mode (`gate-verdict <key>`):

```text
gate-done <key> no-attributable-claim
gate-done <key> run-complete <id>
gate-done <key> run-unclaimed <id>
gate-retry-once <key> run-incomplete <id> <unmet...>
gate-stop <key> run-incomplete <id> <unmet...>
gate-stop <key> run-halted <id>
gate-stop <key> run-waiting <id> <handoff-id> <phase>
gate-stop <key> ambiguous-claims <id...>
gate-stop <key> gate-unavailable <reason-token>
```

Unattributed mode (`gate-verdict --unattributed [<id>...]`):

```text
gate-observe run-complete <id>
gate-observe run-unclaimed <id>
gate-observe run-incomplete <id> <unmet...>
gate-observe run-halted <id>
gate-observe run-waiting <id> <handoff-id> <phase>
gate-observe no-current-run
gate-observe gate-unavailable <reason-token>
```

`gate-before implement-next` prints `gate-armed <key>` on success, `gate-unarmed <reason-token>`
on failure. A malformed/unknown underlying verdict maps to `gate-stop <key> gate-unavailable
unknown-verdict` — fail closed, never retry.

---

### Task 1: Durable gate-record store (`internal/app/rungate_store.go`)

Suggested profile: **premium** (durable state, concurrency, cleanup — named, correctable risk).

**Files:**
- Create: `internal/app/rungate_store.go`
- Test: `internal/app/rungate_store_test.go`

**Interfaces (Produces — later tasks rely on these exact names):**

```go
const (
    gateSchemaVersion = 1
    RetryUnused   = "unused"
    RetryConsumed = "consumed"
)

// GateRecord is the durable per-dispatch gate state. The key is a lookup
// token, never encoded state (spec: "Durable state").
type GateRecord struct {
    Schema        int    `json:"schema"`
    Repo          string `json:"repo"`           // canonical git-common-dir path
    Target        string `json:"target"`          // "docket-implement-next"
    CreatedAt     int64  `json:"created_at"`      // epoch seconds
    DispatchEpoch int64  `json:"dispatch_epoch"`  // captured AFTER the before-read
    BeforeIDs     []int  `json:"before_ids"`      // fresh-origin in-progress set
    AttributedID  int    `json:"attributed_id"`   // 0 = not yet attributed
    Retry         string `json:"retry"`           // RetryUnused | RetryConsumed
    Disposition   string `json:"disposition"`     // latest gate-* report line
    Terminal      bool   `json:"terminal"`
}

func gateRoot(repoDir string) (string, error)                       // <git-common-dir>/docket/rungate; resolves, never creates
func MintGateRecord(repoDir string, rec GateRecord) (string, error) // mints key, creates dir, writes record atomically
func LoadGateRecord(repoDir, key string) (GateRecord, error)
func SaveGateRecord(repoDir, key string, rec GateRecord) error      // temp-beside + rename (atomic)
func ConsumeGateRetry(repoDir, key string) (bool, error)            // true EXACTLY once per key
func PruneGateRecords(repoDir string)                                // terminal-only, retention window, best-effort
```

Design rules, copied from `scripts/lib/docket-dispatch-dir.sh` (the machinery being generalized —
read it first):

- Root under the git **common** dir so records survive `git worktree remove` and never enter a
  commit. Refuse an empty `git rev-parse --git-common-dir` answer.
- Key = `implement-next-<UTC yyyymmddThhmmssZ>-<pid>-<4 hex random>`. Validate on every load with
  `^[a-z0-9-]+$` and a length bound (reject before touching the filesystem — path-safety, like the
  dispatch dir's safe-key validation). A key that fails validation, a record whose `Repo` does not
  match the current repo's canonical common dir, an unsupported `Schema`, or unparseable JSON all
  return a typed error the caller maps to `gate-unavailable` (wrong-repo/malformed-key/
  corrupt-record reason tokens).
- Writes: `os.CreateTemp` in the record's own directory (same-filesystem), then `os.Rename` — the
  atomic-adjacent-replacement rule.
- `ConsumeGateRetry`: atomic via `os.OpenFile(dir/"retry-consumed", O_CREATE|O_EXCL, 0o644)`. The
  filesystem exclusive-create is the CAS: of two concurrent callers exactly one gets `true`. Also
  flip `Retry: RetryConsumed` in the JSON afterward (the marker is authority; the JSON field is
  the readable mirror — LoadGateRecord reports consumed when EITHER says consumed, so a crash
  between marker and JSON stays safe).
- `PruneGateRecords`: only records with `Terminal: true` AND a record file older than
  `DOCKET_DISPATCH_RETENTION_DAYS` (default 7, same env knob semantics as the dispatch dir) are
  removed. Nonterminal records are never age-pruned (spec: "live or nonterminal records must not
  be age-pruned merely because the originating process exited").

- [ ] **Step 1: Write the failing tests** in `internal/app/rungate_store_test.go` (fixture: a
  `t.TempDir()` git repo via `git init`; a worktree of it for the common-dir property). Cases:
  - mint → load round-trips every field; key matches the format regex.
  - record minted in repo A refuses to load from repo B (wrong-repo error).
  - malformed key (`"../escape"`, empty, uppercase, 300 chars) rejected before any filesystem read.
  - corrupt JSON and `Schema: 99` load as typed errors.
  - `SaveGateRecord` after `MintGateRecord` then reload from a **fresh** call (restart durability;
    no shared in-memory state).
  - load from a linked worktree of the same repo resolves the SAME record (common-dir root).
  - `ConsumeGateRetry` returns true once, then false; 16 goroutines racing → exactly one true.
  - prune removes a terminal record backdated past retention (`os.Chtimes`), keeps a terminal
    record inside the window, keeps a nonterminal record backdated past the window.
- [ ] **Step 2: Run and verify they fail** — `go test ./internal/app/ -run TestGate -count=1` →
  compile failure / FAIL.
- [ ] **Step 3: Implement `rungate_store.go`** per the interface above.
- [ ] **Step 4: Run to green** — same command, PASS.
- [ ] **Step 5: Mutation-check the CAS** — temporarily replace the `O_EXCL` open with a plain
  create; the concurrency test must redden. Restore (do NOT use `git checkout` to restore an
  uncommitted edit — learnings: mutation-restore-needs-a-backup-copy; re-apply the one-line edit
  by hand). Re-run to green.
- [ ] **Step 6: Commit** — `git add internal/app/rungate_store.go
  internal/app/rungate_store_test.go && git commit -m "feat(rungate): durable gate-record store
  under the git common dir"`.

---

### Task 2: `docket run gate-before` (arm the gate)

Suggested profile: **standard**.

**Files:**
- Create: `internal/app/rungate_before.go`
- Modify: `cmd/docket/main.go` (register the verb where `run verify` is registered — follow that
  exact registration pattern)
- Test: `internal/app/rungate_before_test.go`, extend `cmd/docket/gate_cli_test.go` only if the
  existing built-binary pattern requires it (a pure `internal/app` test is preferred)

**Interfaces:**
- Consumes: Task 1's store; the metadata re-sync and change-loading plumbing `internal/app` already
  uses — read `internal/app/change_claim.go` (it fetches fresh origin state before claiming) and
  `internal/app/run_verify.go`'s `PlanningDeps` loading, and reuse those helpers rather than
  writing a second fetch or a second change-file parser.
- Produces:

```go
// RunGateBeforeResult carries the report line fields; HumanText renders
// "gate-armed <key>" or "gate-unarmed <reason-token>".
type RunGateBeforeResult struct { Armed bool; Key string; Reason string; ... }
func RunGateBefore(ctx context.Context, deps PlanningDeps, repoDir string, target string) RunGateBeforeResult
```

Behavior (spec §gate-before, in order): (1) re-sync the metadata worktree to fresh origin; (2)
read the in-progress claim set (change files with `status: in-progress`, ids only); (3) capture
`DispatchEpoch = time.Now().Unix()` AFTER the before-read; (4) mint the durable record with
`Target: "docket-implement-next"`, `Retry: RetryUnused`; (5) print `gate-armed <key>`. Any failure
(sync failed, unreadable changes dir, mint failed) prints `gate-unarmed <reason-token>` — exit 0
either way; the report line is the contract. Only `implement-next` is an accepted target argument;
anything else is a usage error (non-zero).

- [ ] **Step 1: Write failing tests** — fixture: a metadata layout like
  `run_verify_test.go`'s (copy its fixture-building helpers' usage, not their code). Cases:
  `gate-armed` printed with a loadable key; record's `BeforeIDs` equals the fixture's in-progress
  ids; `DispatchEpoch >= CreatedAt` of the before-read; unreadable changes dir → `gate-unarmed`
  with a stable reason token; `docket run gate-before bogus-target` → usage error.
- [ ] **Step 2: Run, verify FAIL** — `go test ./internal/app/ -run TestRunGateBefore -count=1`.
- [ ] **Step 3: Implement** `rungate_before.go` + the CLI verb.
- [ ] **Step 4: Run to green**; also `go build ./...`.
- [ ] **Step 5: Commit** — `git commit -m "feat(rungate): docket run gate-before arms the
  implement-next gate"` (add only the files this task touched).

---

### Task 3: `docket run gate-verdict <key>` (attributed mode)

Suggested profile: **premium** (retry accounting; a wrong grant is the one unrecoverable move).

**Files:**
- Create: `internal/app/rungate_verdict.go`
- Modify: `cmd/docket/main.go` (register `run gate-verdict`)
- Test: `internal/app/rungate_verdict_test.go`

**Interfaces:**
- Consumes: Task 1 store (`LoadGateRecord`, `SaveGateRecord`, `ConsumeGateRetry`), Task 2's sync +
  in-progress reader, and **`RunVerify` itself** (`internal/app/run_verify.go`) for the run
  predicate — call it in-process; never re-derive `run-*` verdicts.
- Produces: `func RunGateVerdict(ctx, deps, repoDir, key string) RunGateVerdictResult` rendering
  exactly one line of the attributed vocabulary above.

Behavior (spec §gate-verdict): load record (any load error → `gate-stop <key> gate-unavailable
<reason>`); if `AttributedID != 0`, skip attribution and verify that stored id directly
(attribution never re-runs against a later claim set). Otherwise: re-sync; read in-progress ids
with their `claimed_at`; apply the three filters — (a) id not in `BeforeIDs`, (b) `claimed_at`
parses (RFC3339 → epoch; unparsable = excluded), (c) `claimed_at >= DispatchEpoch`. Zero
survivors → `gate-done <key> no-attributable-claim` (terminal). More than one → `gate-stop <key>
ambiguous-claims <ids…>` (terminal). Exactly one → store `AttributedID`, then delegate to
`RunVerify` and map:

| RunVerify verdict | Retry state | Report | Terminal |
|---|---|---|---|
| run-complete | any | `gate-done <key> run-complete <id>` | yes |
| run-unclaimed | any | `gate-done <key> run-unclaimed <id>` | yes |
| run-incomplete | unused, and `ConsumeGateRetry` returned true | `gate-retry-once <key> run-incomplete <id> <unmet...>` | no |
| run-incomplete | consumed (or lost the CAS) | `gate-stop <key> run-incomplete <id> <unmet...>` | yes |
| run-halted | any | `gate-stop <key> run-halted <id>` | yes |
| run-waiting | any | `gate-stop <key> run-waiting <id> <handoff-id> <phase>` | yes |
| anything else / malformed | any | `gate-stop <key> gate-unavailable unknown-verdict` | yes |

The retry permit is consumed BEFORE the report is emitted (spec: a lost retry is the safe
failure). `Disposition` and `Terminal` are persisted with `SaveGateRecord` before returning.
Handoff id and phase pass through verbatim from `RunVerify`'s waiting fields — never reformatted.

- [ ] **Step 1: Write failing tests.** Build on Task 2's fixture; drive `RunVerify` outcomes by
  building change fixtures the way `run_verify_test.go` does for each verdict. Cases (each is one
  spec acceptance row): zero candidates; one candidate per terminal verdict (complete, unclaimed,
  halted); waiting with exact handoff-id + phase preservation; first incomplete →
  `gate-retry-once` AND reloaded record shows `RetryConsumed`; second verdict call on the same key
  after that → `gate-stop … run-incomplete`; each three-filter rejection independently (id present
  in before-set; `claimed_at` missing; `claimed_at` malformed; `claimed_at < DispatchEpoch`);
  multiple candidates → `ambiguous-claims` listing every survivor; attributed-id short-circuit (a
  second call ignores a NEW claim added after attribution); malformed key / wrong repo / corrupt
  record → `gate-unavailable`; two concurrent verdict calls on one incomplete run → exactly one
  `gate-retry-once` (race the calls; assert by counting reports); two distinct keys in one repo
  stay isolated; process-restart simulation — run before/verdict through separate `RunGate*` calls
  sharing nothing but the repo dir (already the shape of these tests; assert it stays true by
  never passing a record value between them).
- [ ] **Step 2: Run, verify FAIL** — `go test ./internal/app/ -run TestRunGateVerdict -count=1`.
- [ ] **Step 3: Implement** `rungate_verdict.go` + CLI verb.
- [ ] **Step 4: Run to green.** The test proving retry consumption must assert the **post-pass
  durable state** (reload the record; marker present), not merely the emitted line.
- [ ] **Step 5: Mutation-check** — swap the consume-then-emit order to emit-then-consume; the
  concurrent-callers test must redden (or the post-state assert must). Restore by hand; re-run.
- [ ] **Step 6: Commit** — `git commit -m "feat(rungate): attributed gate-verdict with atomic
  one-retry accounting"`.

---

### Task 4: `docket run gate-verdict --unattributed [<id>...]` (observe-only mode)

Suggested profile: **standard**.

**Files:**
- Modify: `internal/app/rungate_verdict.go`, `cmd/docket/main.go`
- Test: extend `internal/app/rungate_verdict_test.go`

Behavior (spec §Unattributed mode): no key, no record, no writes. Re-sync; when hint ids are
supplied verify each (a hint is an id to verify, never attribution evidence); when none are
supplied verify every current in-progress id; no in-progress ids and no hints →
`gate-observe no-current-run`. One `gate-observe …` line per verified id, using `RunVerify`'s
verdict verbatim. Sync/read failure → `gate-observe gate-unavailable <reason-token>`. There is no
code path from `--unattributed` to `gate-retry-once` — the observe renderer must be structurally
unable to emit it (separate render function that only knows the `gate-observe` prefix).

- [ ] **Step 1: Write failing tests**: hints (mixed verdicts, one line each, input order); no
  hints → all in-progress ids; empty backlog → `no-current-run`; an incomplete run observed
  unattributed emits `gate-observe run-incomplete …` and writes NO record and consumes NOTHING
  (assert the rungate root is absent/unchanged); `--unattributed` combined with a key is a usage
  error.
- [ ] **Step 2: FAIL**, **Step 3: implement**, **Step 4: green** (`-count=1`).
- [ ] **Step 5: Commit** — `git commit -m "feat(rungate): observe-only unattributed gate-verdict"`.

---

### Task 5: Shell facade wrappers + hermetic cross-process matrix

Suggested profile: **standard**.

**Files:**
- Create: `scripts/gate-before.sh`, `scripts/gate-before.md`, `scripts/gate-verdict.sh`,
  `scripts/gate-verdict.md` (contract .md is required — `test_script_contracts_coverage.sh`)
- Modify: `scripts/docket.sh` (add `gate-before gate-verdict` to `WRAPPED_OPS` and the usage
  block)
- Test: `tests/test_gate_facade.sh` (new)

The wrappers are thin delegators following `scripts/verify-run.sh`'s existing seam —
`DOCKET_BIN="${DOCKET_BIN:-docket}"` — passing argv through to `"$DOCKET_BIN" run gate-before …` /
`"$DOCKET_BIN" run gate-verdict …` and forwarding stdout/exit code untouched. They exist so the
payload's `docket.sh gate-before implement-next` spelling works in consumer repos.

`tests/test_gate_facade.sh` is the cross-PROCESS half of the acceptance matrix (the in-process
half lives in Go): using the built binary (build once into the test's tmpdir, or reuse the suite's
existing built-binary helper if `tests/` has one — check how other tests obtain `DOCKET_BIN`),
against a hermetic fixture repo: (a) `docket.sh gate-before implement-next` prints
`gate-armed <key>`; (b) a separate `docket.sh gate-verdict <key>` invocation (fresh process =
restart durability) reports from the durable record; (c) retry consumed in process 1 is still
consumed in process 2; (d) `gate-verdict --unattributed` works through the wrapper; (e) unknown op
spelling still errors; (f) report lines carry exit 0. Follow the suite's per-file wall-clock
budget discipline (`tests/runtime-budgets.tsv` gets a row — see `tests/README.md` for where a new
test registers).

- [ ] **Step 1: Write `tests/test_gate_facade.sh` first**, run `bash tests/test_gate_facade.sh` →
  FAIL (op unknown).
- [ ] **Step 2: Implement** the two wrappers + `WRAPPED_OPS` + usage line + the two contract .md
  files (state: pure delegators; the Go binary owns behavior; mock seam `DOCKET_BIN`).
- [ ] **Step 3: Run to green**; also run `bash tests/test_script_contracts_coverage.sh` and
  `bash tests/test_runtime_budgets.sh`.
- [ ] **Step 4: Commit** — `git commit -m "feat(rungate): docket.sh gate-before/gate-verdict
  wrappers over the Go facade"`.

---

### Task 6: Compact run-gate payload (`cursor-rules/run-gate.md`) + test relocation

Suggested profile: **premium** (this rewrites the safety prose all four harnesses load; the guard
rewrite in `test_sync_agents_run_gate.sh` is large and must stay mutation-sensitive).

**Files:**
- Modify: `cursor-rules/run-gate.md` (full replacement below)
- Regenerate: `internal/assets/embedded/tree/cursor-rules/run-gate.md` via `go run ./cmd/genassets`
- Rewrite: `tests/test_sync_agents_run_gate.sh`
- Modify (as the pre-grep demands): any other test greping the removed prose

**Replacement payload — write exactly this** (the compact parent instruction, spec §"Compact
parent instruction" items 1–5 plus the two never-rules the vocabulary cannot carry alone):

```markdown
## Run gate — bracket a dispatched implement-next run with the gate facade

A dispatched run that stops early returns a report that reads as success, and a completion
notification is the CHILD's claim, not your report. The gate facade owns attribution, durable
state, and retry accounting — never hand-reimplement them. Docket's helper facade is not on
`PATH`: run each command below verbatim, expansion included.

1. Before dispatching `docket-implement-next`, run
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh gate-before implement-next` and keep
   the printed key in your own notes — a shell variable does not survive the next tool call. If it
   prints `gate-unarmed`, you may still dispatch, but the return is keyless (step 2's fallback)
   and can never authorize a re-dispatch.
2. After the run returns — or its detached completion notification arrives — run
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh gate-verdict <key>`. Without a key,
   run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh gate-verdict --unattributed`,
   adding any change id the notification names as a trailing hint argument.
3. Obey the facade's `gate-*` report line exactly — never its exit code, and never the child's
   prose.
4. Only `gate-retry-once` authorizes another dispatch: the same `docket-implement-next`, once, for
   the id and unmet conjuncts it names, keeping the same key. Every `gate-stop` and every
   `gate-observe` forbids re-dispatch — `run-halted` means a human is needed, and `run-waiting`
   names a continuation a fresh dispatch would NOT resume: report the handoff id and phase, then
   stop.
5. Never hand-reimplement attribution or infer permission from child prose, launch shape,
   timestamps, ids, or process exit codes.
```

Pre-step (learnings: restatement-accumulates-its-own-guards): `grep -rn 'DISPATCH_EPOCH\|with-claimed-at\|Detached dispatch\|before-set' tests/` and list every assert that greps the OLD
payload's prose; each one is either rewritten here (if it guards the payload) or repointed at the
artifact that owns the content (if it guards behavior now owned by the facade — the behavioral
invariant moved to Tasks 1–5's tests, so the repoint is a move, not a loss).

New guard content for `tests/test_sync_agents_run_gate.sh` (whitespace-collapsed phrase greps,
each bound to its claim):

- Positive: block contains `gate-before implement-next`; contains `gate-verdict`; contains
  `--unattributed`; the `gate-retry-once` sentence sits in the same numbered item as "once" and
  "same key" (bounded-gap phrase match); `run-halted` bound to "human"; `run-waiting` bound to
  "stop"; the `DOCKET_SCRIPTS_DIR` expansion spelling survives verbatim.
- Negative — detect the removed state, not the new wording (learnings:
  assert-detects-removal-not-replacement): the emitted block contains NO `DISPATCH_EPOCH`, NO
  `--with-claimed-at`, NO `ALL THREE filters` / three-filter procedure, NO `### Detached dispatch`
  heading. Scope these to the managed block, not the whole repo.
- Mutation check (recorded in the test header): restore one old detached-dispatch sentence into
  the payload → the negative assert reddens; delete the `gate-before` line → a positive assert
  reddens.

- [ ] **Step 1: Run the pre-grep** and write the relocation list into the task commit message.
- [ ] **Step 2: Rewrite the guards** in `test_sync_agents_run_gate.sh` against the NEW payload;
  run `bash tests/test_sync_agents_run_gate.sh` → FAIL (payload still old).
- [ ] **Step 3: Replace `cursor-rules/run-gate.md`** with the payload above; run
  `go run ./cmd/genassets`; run `bash tests/test_asset_bundle_drift.sh` → PASS.
- [ ] **Step 4: Re-sync generated surfaces** — `./sync-agents.sh` then `./sync-agents.sh --check`;
  run the rewritten test plus `bash tests/test_cursor_dispatch_rule.sh` (Cursor consumes the same
  payload — fix its greps if they touched removed prose) → all green; `go test ./internal/... -count=1`
  and refresh any harness goldens that embed the payload (`go test ./internal/harness/... -update`
  then re-run without `-update`, reviewing the golden diff by eye).
- [ ] **Step 5: Run the payload mutation checks** from the guard list above; restore; re-run green.
- [ ] **Step 6: Commit** — `git commit -m "feat(dispatch): compact facade-backed run-gate payload
  replaces the hand-executed procedure"` (include the regenerated embedded copy and goldens).

---

### Task 7: Compact dispatch block — drop the roster in BOTH generators + size bound

Suggested profile: **premium** (a lockstep two-generator edit with byte-identity and a new
regrowth bound).

**Files:**
- Modify: `internal/harness/dispatch.go` (`dispatchPreamble`, `DispatchInterior`),
  `internal/harness/dispatch_test.go`, adapter callers of `DispatchInterior`
  (`internal/harness/claude/claude.go`, `codex`, `opencode` — Cursor does not consume it)
- Modify: `sync-agents.sh` `assemble_agents_md_dispatch` (same edit, byte-identical output)
- Update: harness goldens (`internal/harness/*/testdata/golden`), `tests/test_sync_agents.sh`,
  `tests/test_sync_agents_codex_dispatch.sh`, `tests/test_sync_agents_claude_surface.sh` (must
  stay green), plus whatever the pre-grep finds
- Create: `tests/test_dispatch_block_budget.sh` (size bound)

**New interior — both generators emit exactly this** (heading, one paragraph, blank line, run-gate
payload; NO per-agent bullets, NO shipped-harness list — removing the list also removes the one
deliberate Go/shell variance, so the two generators become textually identical here; note that in
the emitter comments, per learnings: consolidation-flattens-caller-variance the divergence was
deliberate and is being retired, not overlooked):

```markdown
## Docket agents — dispatch, don't run inline

When a requested Docket workflow has a registered same-name `docket-*` agent, dispatch that agent
instead of running the workflow inline: the agent carries that workflow's dispatch contract, its
skill preload, and whatever model and reasoning effort your config layers pin for it. Your
harness's native agent registry is authoritative for agent names, descriptions, and availability —
this block does not restate it. If no same-name agent is registered, do not invent one; follow the
workflow's own inline or unavailable-capability contract. Dispatch through the harness's native
named-agent dispatch, and pass the request through unchanged, including any change or ADR id.
```

`DispatchInterior`'s signature loses its roster input: `func DispatchInterior(runGate []byte)
string`. Update the three adapter call sites (they currently pass `sources`). The shell
`assemble_agents_md_dispatch` drops `$shipped_list` and the whole roster loop.

Guards:

- Rewrite `internal/harness/dispatch_test.go`: interior contains the routing rule's key phrases
  ("registered same-name", "authoritative for agent names, descriptions, and availability", "do
  not invent one"), contains the run-gate heading after the paragraph, and — negative — contains
  no line matching `^- \*\*docket-` (the roster's bullet SHAPE, not a spelling list) and does not
  contain "Delegate to the".
- Shell twins of those asserts in `tests/test_sync_agents.sh`'s block checks (whitespace-collapsed).
- `tests/test_dispatch_block_budget.sh`: assemble the block in a fixture repo via `./sync-agents.sh`,
  extract the managed block between its markers, and assert `wc -w` ≤ BUDGET. First **measure the
  OLD block's actual** on the pre-task tree (`git stash` nothing — measure before editing, record
  the number in the test header), then set BUDGET from the NEW measured actual rounded up to the
  next multiple of 50, and add a hard assert that BUDGET < the recorded old actual — the "lowers
  the bound" acceptance made durable. Include the non-vacuity self-check pattern
  `test_skill_size_budgets.sh` ends with (a deliberately over-budget pair is caught).

- [ ] **Step 1: Measure the old actual** (fixture repo, `./sync-agents.sh`, `wc -w` on the block)
  and record it.
- [ ] **Step 2: Pre-grep the roster prose** (`grep -rn 'Delegate to the' tests/` and
  `grep -rn 'docket-build-max\|docket-rebase-resolver' tests/` — sample agent names locate
  roster-dependent asserts); list relocations.
- [ ] **Step 3: Write the failing guards** — rewrite `dispatch_test.go` expectations, add the
  shell asserts and `test_dispatch_block_budget.sh`; run
  `go test ./internal/harness/ -count=1` and `bash tests/test_dispatch_block_budget.sh` → FAIL.
- [ ] **Step 4: Implement** both generators' edits; `go run ./cmd/genassets` is NOT needed (no
  authored payload changed) but goldens are: `go test ./internal/harness/... -update`, review the
  diff (bullets gone, paragraph swapped, nothing else), re-run without `-update`.
- [ ] **Step 5: Lockstep proof** — `./sync-agents.sh && ./sync-agents.sh --check` in the repo;
  run `bash tests/test_sync_agents.sh tests/test_sync_agents_codex_dispatch.sh
  tests/test_sync_agents_claude_surface.sh tests/test_sync_agents_run_gate.sh` → green.
- [ ] **Step 6: Mutation-check the negative guard** — re-add one roster bullet to the Go emitter →
  `dispatch_test.go` reddens; mirror the same probe on the shell side → shell assert reddens.
  Restore both by hand; re-run green (`-count=1`).
- [ ] **Step 7: Commit** — `git commit -m "feat(dispatch): compact registered-agent routing rule
  replaces the 17-agent roster in both generators"`.

---

### Task 8: Exact-name recursion guard in every generated wrapper (both generators)

Suggested profile: **premium** (touches all four adapters + mirror; the guard's mutation matrix is
the spec's own acceptance).

**Files:**
- Modify: `internal/harness/harness.go` or new `internal/harness/guard.go` (shared emitter),
  `internal/harness/{claude,codex,cursor,opencode}/*.go` (`renderAgent` in each),
  their `*_test.go` + goldens
- Modify: `sync-agents.sh` (its wrapper-emission path — locate with
  `grep -n 'renderAgent\|emit.*agent\|agent body' sync-agents.sh` and read the surrounding
  function before editing)
- Create: `tests/test_sync_agents_recursion_guard.sh`

**Shared guard emitter — exact wording (spec §Decision 2):**

```go
// RecursionGuard is the self-recursion guard injected into every generated
// wrapper by every renderer. It prohibits exactly one edge — docket-X
// dispatching another docket-X for the assignment it already holds — and
// explicitly preserves required dispatches to different agents. It relies on
// the generated literal name, never on a name: field, a skill preload, or
// inference from surrounding prose.
func RecursionGuard(name string) string {
    return "You are already running as `" + name + "`. Carry out this wrapper's assigned charter " +
        "directly. Do not dispatch another `" + name + "` merely to perform the current " +
        "assignment. Dispatches to different agents explicitly required by the active charter " +
        "remain required."
}
```

Every renderer injects `RecursionGuard(s.Name)` as its own paragraph between the wrapper's
frontmatter/heading and the body it already emits (one consistent position per renderer; the
Cursor subagent renderer included). The shell mirror emits the byte-identical paragraph with the
agent name interpolated (guard the interpolation: names come from the source inventory, which the
mirror already trusts). The wording must NOT mention "your preloaded skill" (spec) — the phrase
"assigned charter" is what makes it correct for skill-less wrappers and shared-role-skill wrappers.

Guards (spec §"Wrapper recursion guard" — every bullet gets an assert):

- Go, per adapter: iterate the definitions the adapter actually renders (from `ParseInventory` /
  the goldens' input table — never a hand list) and assert each rendered body contains
  ``running as `<that wrapper's exact name>` `` AND the different-agent clause ("Dispatches to
  different agents explicitly required") AND not "your preloaded skill".
- `tests/test_sync_agents_recursion_guard.sh`: in a fixture repo run `./sync-agents.sh`, derive
  the expected wrapper set from the GENERATED output directory listing (both directions —
  learnings: correspondence-guard-runs-one-way: every generated wrapper carries the guard, and
  every guard names the file's own agent), and assert the three properties above per file, with a
  population floor (at least one wrapper found, and the count equals the source `agents/docket-*.md`
  count — computed, not hand-written; learnings: backstop-must-compute-not-reenumerate).
- Mutation matrix (spec-mandated; record each in the test header): (1) remove the injection call
  from ONE adapter → that adapter's Go test reddens; (2) replace `<name>` substitution with a
  fixed string → per-name assert reddens; (3) delete the different-agent sentence from
  `RecursionGuard` → clause assert reddens; (4) broaden the guard to prohibit ALL nested dispatch
  (delete "another `<name>`", write "any agent") → the exact-name assert reddens.

- [ ] **Step 1: Write the failing guards** — the per-adapter Go asserts and
  `test_sync_agents_recursion_guard.sh`; run them → FAIL (no guard exists yet; assert the
  pre-state is truly guard-free first: `grep -rn 'already running as' internal/harness sync-agents.sh`
  is empty).
- [ ] **Step 2: Implement** `RecursionGuard` + the four adapter injections + the mirror's
  injection.
- [ ] **Step 3: Goldens** — `go test ./internal/harness/... -update`, review the diff (exactly one
  new paragraph per wrapper, correct name each), re-run without `-update` → green (`-count=1`).
- [ ] **Step 4: Lockstep proof** — `./sync-agents.sh && ./sync-agents.sh --check`; run the new
  shell test plus `bash tests/test_sync_agents_codex.sh tests/test_sync_agents_cursor.sh
  tests/test_sync_agents_opencode.sh tests/test_sync_agents_claude_surface.sh` → green.
- [ ] **Step 5: Run the four-probe mutation matrix**; restore by hand after each probe; final
  green run.
- [ ] **Step 6: Commit** — `git commit -m "feat(dispatch): exact-name self-recursion guard in
  every generated wrapper"`.

---

### Task 9: Cursor distinction + whole-surface re-verification

Suggested profile: **economy** (assert-only sweep; no emitter changes expected).

**Files:**
- Modify (only as asserts demand): `tests/test_cursor_dispatch_rule.sh`,
  `tests/test_sync_agents_cursor.sh`, `cursor-rules/dispatch.head.md` comments if any now-false
  claim surfaces
- Test: the files above

Verify and pin the spec's Part 4: Cursor's always-applied rule remains its own routing surface —
per-agent fragments with their own headings, `dispatch.head.md` intact — while its assembled rule
carries the SAME compact gate payload (Task 6's text, already appended by
`assembleDispatchRule`). Add to `test_cursor_dispatch_rule.sh`: (a) the assembled Cursor rule
contains `gate-before implement-next` (shared facade trigger); (b) it contains NO
`DISPATCH_EPOCH` / detached-dispatch procedure (same negative pair as Task 6, scoped to the Cursor
artifact); (c) no assert anywhere claims Cursor consumes the `AGENTS.md`/`CLAUDE.md` managed block
— sweep with `grep -rn 'AGENTS.md' tests/test_cursor*` and fix any that do. Confirm the Cursor
fragments still cover exactly the agent inventory (the existing orphan check in
`assembleDispatchRule` stays authoritative — do not add a second hand list).

- [ ] **Step 1: Write the two new asserts**, run `bash tests/test_cursor_dispatch_rule.sh` —
  expected: already green if Task 6 landed correctly (this task's value is the pin, not a change);
  if red, the fix belongs in Task 6's payload, not here.
- [ ] **Step 2: Mutation-check** — point the Cursor assembler at a stub gate payload missing the
  trigger → assert (a) reddens; restore.
- [ ] **Step 3: Run the full sync-agents family** — `for t in tests/test_sync_agents*.sh
  tests/test_cursor*.sh; do bash "$t" || break; done` → all green.
- [ ] **Step 4: Commit** — `git commit -m "test(cursor): pin the shared gate trigger and the
  distinct routing surface"`.

---

### Task 10: Full-suite gate + human-precondition record

Suggested profile: **standard**.

**Files:**
- No new source. Fix-forward anything red.

- [ ] **Step 1: Whole suite** — run the command `finalize.test_command` resolves to
  (`scripts/run-tests.sh`; read the resolved value from config, never a second copy) to
  completion, plus `go test ./... -count=1` and `./sync-agents.sh --check`. Everything green; act
  on any `SERIAL CONFIRMED OVER BUDGET:` line (the new `test_gate_facade.sh` and rewritten
  run-gate test are the likely candidates — split or speed up, never skip).
- [ ] **Step 2: Verify the size acceptance** — `bash tests/test_dispatch_block_budget.sh` green
  and its header records old actual > new budget.
- [ ] **Step 3: Record the human merge-gate preconditions** for the PR body / results file —
  verbatim, so the reviewer sees them as gates, not follow-ups:
  1. Maintainer removes the hand-authored Docket dispatch block from `~/.claude/CLAUDE.md`
     (docket never touches it).
  2. Four-harness fresh-process behavioral acceptance (Claude, Codex, Cursor, OpenCode) per the
     spec's §"External harness acceptance" checklist — process-start artifacts cannot be certified
     from this session; stale processes must be terminated first. A failed native-discovery check
     on any harness is a release blocker for that harness's roster removal.
- [ ] **Step 4: Commit** any gate fixes — `git commit -m "test: green the full suite for change
  0334"`.

---

## Self-review (performed while writing)

- Spec coverage: Decision 1 → Task 7 (+ budget), Decision 2 → Task 8, Decision 3 → Tasks 1–6,
  Decision 4 → Tasks 6/7/9, Decision 5 → Task 10 Step 3 (human gate, not build work). Acceptance
  §compact-parent-surface → Tasks 6/7/9; §wrapper-recursion-guard → Task 8; §gate-facade matrix →
  Tasks 1–5 (every listed row appears in a Step 1 case list); §repository-verification → Task 10.
- Failure-posture rows all map: unreadable state/wrong repo/malformed → `gate-unavailable`
  (Tasks 1/3), unparsable `claimed_at` → filter exclusion (Task 3), concurrent permits → CAS
  (Tasks 1/3), keyless → observe-only with no retry path (Task 4), unknown vocabulary →
  `unknown-verdict` fail-closed (Task 3).
- Type consistency: `GateRecord`, `MintGateRecord`/`LoadGateRecord`/`SaveGateRecord`/
  `ConsumeGateRetry`/`PruneGateRecords`, `RunGateBefore`/`RunGateVerdict`, `RecursionGuard`, and
  the report vocabulary are spelled identically at every use above.
