<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0421 — Make build and outer run gate attempt limits configurable](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-10-0421-make-build-and-outer-run-gate-attempt-limits-configurable.md)**
<!-- docket:backlink:end -->
# Make Build and Outer Run Gate Attempt Limits Configurable — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two independently configurable positive-integer attempt limits — `build.max_attempts` (default 4: initial full-suite run + up to three repair-and-rerun cycles) and `run.max_attempts` (default 2: initial implementation run + up to one eligible retry) — resolved through normal config precedence, snapshotted into durable gate state when the owning scope begins, and enforced with the existing attribution, continuation, and human-halt semantics intact.

**Architecture:** Mirror the change-0349 `finalize.resolver_max_attempts` precedent for the two config leaves (schema registry row, typed `Effective` leaf, built-in default, `assemble()` assignment, diagnostics row, example-config entry). The outer gate generalizes the binary `retry-consumed` O_EXCL marker in `internal/app/rungate_store.go` into a counted per-attempt marker budget snapshotted into the `GateRecord` at mint (schema v4, legacy fail-closed). The build gate gains net-new durable machinery: a per-(repo, change, phase) suite-attempt budget store in `internal/gatedrive`, reserved atomically inside `StartDrive` for build-owned drives before any launch, so re-observation/recovery/takeover/continuation never double-charge and no repair worker can bypass the cap. Skill and concept prose then replaces the fixed-limit language.

**Tech Stack:** Go (`internal/config`, `internal/app`, `internal/gatedrive`, `internal/repoguard`), maintained markdown skill contracts, `go generate ./internal/assets/` for embedded mirrors.

**Spec:** `docs/superpowers/specs/2026-09-10-make-build-and-outer-run-gate-attempt-limits-configurable-design.md` (on the `docket` metadata branch). Change file: `docs/changes/active/0421-make-build-and-outer-run-gate-attempt-limits-configurable.md` (id 421). Acceptance criteria are the spec's "Acceptance and validation" section; the coverage map at the end of this plan ties each criterion to its task.

## Global Constraints

- **Counting semantics (verbatim from spec):** both limits count total attempts *including the initial attempt*. A value of 1 disables retries. Zero, negative, fractional, string, boolean, and collection values are invalid (`intLeaf(1)` handles all of these). Stop early on success.
- **Defaults:** `build.max_attempts: 4`, `run.max_attempts: 2`. Default behavior must be byte-for-byte today's behavior (one outer retry; initial suite run + repair cycles bounded at 3).
- **Snapshot rule:** each budget is snapshotted when its owning gate scope begins. Config edits affect newly started scopes only — never increase, reset, or rewrite an already-owned budget.
- **Out of scope — do not touch:** finalize repair/resolver limits (`finalize.resolver_max_attempts`, change 0419's territory), the post-review fix-loop bound in `skills/docket-implement-next/references/fix-loop.md` (`REVIEW_MAX_FIX_TASKS` / `review.max_fix_tasks`), attribution/permission/human-halt rules, budget resets on continuation, unlimited retries, progress heuristics.
- **Non-retryable stays non-retryable:** infrastructure errors, unavailable results, invalid configuration, unsafe worker outcomes, and exhausted observation budgets keep their existing halt/fail-closed handling. Exhaustion is a bound, not a licence to continue past another halt condition. `run-halted` keeps absolute precedence: a build that records `run-halted` cannot acquire more build repairs via an outer retry.
- **Report-line compatibility:** every existing `gate-*` report line keeps its exact shape. Usage/limit surfaces are additive (JSON fields, HumanText lines) — never a changed token in a parsed line.
- **Never hand-edit** `internal/assets/embedded/tree/**`. After editing an authored surface that has an embedded twin, run `go generate ./internal/assets/` (drift guard: `TestEmbeddedMatchesAuthored`). Ignore any `.worktrees/` copies when deriving sites.
- **Guard discipline (repo AGENTS.md):** derive edit/guard sites from a whole-repo grep, never a hand-list; key guards on syntactic shape; mutation-test every new guard with `go test -count=1` (Go caches results otherwise) and restore mutations from a backup **copy** (`cp file file.bak`), never `git checkout --` over uncommitted work.
- **Budget pins:** `internal/repoguard/budgets_test.go` pins skill files at exact line/word counts — `docket-build/SKILL.md` (410/4102) and `docket-implement-next/SKILL.md` (210/7530) are at or near their ceilings. Prose edits require a documented one-time re-baseline pinned at the exact new counts (house precedent: the 0416/0410 header notes in that file).
- **Stage by explicit path only** — never `git add -A` (shared worktree discipline). Commit after each task.
- **Whole-suite gate** (run by the build controller after the last task, per its own contract, never a second copy of the command): the resolved `build.test_command`, which is `go run ./cmd/docket development test`, entered from source. Read the budget report even on green.
- **Decisions to record via ADR (for the parent's Step 6, not for any build worker to write):** (a) the outer run gate's single-retry marker generalizes to a counted, config-snapshotted budget with GateRecord schema v4 and fail-closed legacy handling; (b) the build full-suite repair bound moves from prose to a durable scope-owned reservation in `internal/gatedrive`. Note both in the coordinator's results context so `docket-implement-next` dispatches `docket-adr`.

## File structure (locked decisions)

| Concern | File | Decision |
|---|---|---|
| Config leaves | `internal/config/schema.go`, `config.go`, `defaults.go`, `resolve.go` | `build.max_attempts` joins the existing `build` block; `run.max_attempts` introduces a new one-leaf `run` block (`Run` struct on `Effective`) |
| Diagnostics | `internal/app/config.go` | two new `leafLine` rows in `effectiveLines` |
| Typed context | `internal/app/repository_prepare.go` | `PrepareBuild.MaxAttempts int` mirrors `cfg.Build.MaxAttempts.Value`. `run.max_attempts` deliberately does **not** enter PrepareContext — the outer gate resolves it from authoritative config at the facade (spec: "the outer gate resolves its own run value through authoritative configuration") |
| Outer gate store | `internal/app/rungate_store.go` | `GateRecord` schema v4: adds `AttemptLimit`; per-attempt O_EXCL markers `retry-consumed-<n>` replace the single `retry-consumed`; marker files stay the authority; v3 records fail closed (existing schema-mismatch discipline) |
| Outer gate policy | `internal/app/rungate_verdict.go`, `rungate_before.go` (or wherever `MintGateRecord` is called — locate by grep) | mint snapshots `run.max_attempts`; the `VerdictRunIncomplete` case grants at most `AttemptLimit-1` retries via the counted CAS |
| Build budget store | `internal/gatedrive/suitebudget.go` (new) + `suitebudget_test.go` | durable per-(repo-identity, change-id, phase) suite-attempt budget record under `<git-common-dir>/docket/gate-suite-budgets/v1/`, flock + generation-CAS discipline copied from `scope.go` |
| Build budget wiring | `internal/app/gate_drive.go`, `internal/gatedrive/driver.go` | build-owned `StartDrive` with a change-id identity reserves one attempt before creating the drive; exhaustion is a typed refusal naming `build.max_attempts` used/limit |
| Prose | `skills/docket-build/SKILL.md`, `skills/docket-build/references/gate-caller-loop.md`, `skills/docket-implement-next/SKILL.md`, `docs/concepts/run-gate.md`, `docs/concepts/build-profiles-and-gate.md`, `.docket.example.yml` | fixed-limit language replaced only where it describes these two budgets |

---

### Task 1: Config leaves end-to-end (`build.max_attempts`, `run.max_attempts`)

**Build profile:** standard

Reason: a pattern-following enumeration with a strong in-repo precedent (change 0349) and existing correspondence guards; the risk is missing an enumerated site, and the guards catch that.

**Files:**
- Modify: `internal/config/schema.go` (registry rows — the `build` block sits near the `finalize.resolver_max_attempts` row at ~line 222 and the `build.gate`/`build.test_command` rows)
- Modify: `internal/config/config.go` (`Build` struct ~line 156; new `Run` struct; `Effective` field)
- Modify: `internal/config/defaults.go` (`builtinEffective()` ~lines 25–64)
- Modify: `internal/config/resolve.go` (`assemble()` ~lines 273–326)
- Modify: `internal/app/config.go` (`effectiveLines()` ~lines 234–269)
- Modify: `.docket.example.yml` (documented entries; the resolver never reads this file — it is the canonical config reference, guarded by `internal/config/example_correspondence_test.go` in BOTH directions, so a schema row without an example entry reddens Direction B)
- Modify (regenerate): `internal/assets/embedded/tree/.docket.example.yml` via `go generate ./internal/assets/`
- Test: `internal/config/schema_test.go`, `decode_test.go`, `defaults_test.go`, `resolve_test.go`; `internal/app/config_test.go`

**Interfaces (Produces):**
- `config.Effective.Build.MaxAttempts config.Value[int]` (JSON `max_attempts`), built-in 4
- `config.Effective.Run.MaxAttempts config.Value[int]` (JSON `max_attempts`) on new `config.Run` struct (JSON `run`), built-in 2
- Diagnostics rows `build.max_attempts = 4  [built-in]` and `run.max_attempts = 2  [built-in]`

- [ ] **Step 1: Write the failing tests first.** Extend the existing enumerated tables (each is an intentional enumerated-floor — every one must gain rows or the change is invisible to it):
  - `schema_test.go`: add `"build.max_attempts"` and `"run.max_attempts"` to the known-paths list (~line 21), the defaults table (~line 116: `"build.max_attempts": 4`, `"run.max_attempts": 2`), and the validation cases (~line 287) mirroring the `finalize.resolver_max_attempts` rows exactly — explicit valid value, value 1 valid, `0` rejected `must be >= 1`, negative rejected, non-integer (`"four"`, `3.5`, `true`) rejected via `intLeaf`:

    ```go
    // build.max_attempts / run.max_attempts: positive int, floor 1 (intLeaf(1)).
    {"build max attempts explicit", "build.max_attempts", "6", 6, ""},
    {"build max attempts one", "build.max_attempts", "1", 1, ""},
    {"build max attempts zero", "build.max_attempts", "0", 0, "must be >= 1"},
    {"run max attempts explicit", "run.max_attempts", "5", 5, ""},
    {"run max attempts zero", "run.max_attempts", "0", 0, "must be >= 1"},
    ```

  - `decode_test.go` (~line 71): block and flow rows for both keys (`"build:\n  max_attempts: 5\n"`, `"run:\n  max_attempts: 3\n"`, flow twins).
  - `defaults_test.go` (~line 150): `"build.max_attempts": eff.Build.MaxAttempts.Value` and `"run.max_attempts": eff.Run.MaxAttempts.Value`.
  - `resolve_test.go`: extend the leaf switch (~line 77) with both paths and the YAML snippet helper (~line 103 pattern), then add `TestPrecedenceBuildMaxAttempts` and `TestPrecedenceRunMaxAttempts` cloned from `TestPrecedenceResolverMaxAttempts` (~line 350): built-in default with `Explicit == false`, repository layer override, repository-local override winning over repository, global layer, each asserting Value + Provenance.Layer + Explicit.
  - `internal/app/config_test.go`: clone `TestConfigDiagnosticsResolverMaxAttemptsSurface` (~line 718) as `TestConfigDiagnosticsAttemptLimitsSurface`: default run asserts JSON values 4/2 and HumanText contains `build.max_attempts = 4  [built-in]` and `run.max_attempts = 2  [built-in]`; a repo-layer write (`"build:\n  max_attempts: 6\nrun:\n  max_attempts: 3\n"`) asserts 6/3 with repository provenance.
- [ ] **Step 2: Run the new tests, confirm they fail** (unknown key / missing field compile errors count): `go test -count=1 ./internal/config/ ./internal/app/ -run 'Precedence|Schema|Defaults|Decode|ConfigDiagnostics'` — expect FAIL/compile error.
- [ ] **Step 3: Implement.**
  - `schema.go`, in the `// 15: build.` group:

    ```go
    {path: "build.max_attempts", kind: kindInt, def: 4,
        merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: intLeaf(1)},
    ```

    and a new numbered group beside it (renumber the group comments consistently):

    ```go
    // run. The outer implement-next run gate's own attempt policy (change 0421).
    {path: "run.max_attempts", kind: kindInt, def: 2,
        merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: intLeaf(1)},
    ```

  - `config.go`: add to `Build`:

    ```go
    // MaxAttempts caps logical build full-suite attempts per owned build phase
    // (change 0421), counting the initial run. Positive; snapshotted into the
    // durable suite-attempt budget when the phase's first build-owned drive starts.
    MaxAttempts Value[int] `json:"max_attempts"`
    ```

    and a new struct + `Effective` field (place `Run Run \`json:"run"\`` after `Build`):

    ```go
    // Run is the outer implement-next run gate's own attempt policy (change 0421).
    type Run struct {
        // MaxAttempts caps total attributed implementation attempts per gate
        // arming, counting the original dispatch. Positive; snapshotted into the
        // GateRecord at mint.
        MaxAttempts Value[int] `json:"max_attempts"`
    }
    ```

  - `defaults.go`: `MaxAttempts: builtinValue(4)` inside the `Build{...}` literal; `Run: Run{MaxAttempts: builtinValue(2)}`.
  - `resolve.go` `assemble()`: `set(assign(&eff.Build.MaxAttempts, r.declared, "build.max_attempts"))` beside the other build assigns; `set(assign(&eff.Run.MaxAttempts, r.declared, "run.max_attempts"))`.
  - `internal/app/config.go` `effectiveLines()`: after the `build.test_command` row add `leafLine("build.max_attempts", strconv.Itoa(eff.Build.MaxAttempts.Value), eff.Build.MaxAttempts.Provenance)`; add a `run.max_attempts` row (position it to match `Effective` declaration order — the function walks the struct in declaration order).
  - `.docket.example.yml`: inside the existing `build:` block add a commented-default entry; add a new `run:` block. Write the comments for a user deciding whether to set the key, leading with what it does for them, never with the change number or mechanism (learnings: `config-knob-ship-end-to-end`). Model on the `resolver_max_attempts` entry at ~line 118:

    ```yaml
    # max_attempts — how many full test-suite runs the build phase may spend before it stops and
    # asks for help: the first run plus a repair-and-rerun for each red result. 4 (the default)
    # allows three repairs; 1 means a red suite halts immediately with no repair attempt.
    max_attempts: 4
    ```

    ```yaml
    run:
      # max_attempts — how many end-to-end implementation attempts an autonomous run may make on
      # one change: the original attempt plus eligible retries after an incomplete run. 2 (the
      # default) allows a single retry; 1 disables retries. Explicit halts always need a human
      # regardless of this limit.
      max_attempts: 2
    ```

- [ ] **Step 4: Regenerate the embedded twin:** `go generate ./internal/assets/`.
- [ ] **Step 5: Run the package tests:** `go test -count=1 ./internal/config/ ./internal/app/ ./internal/assets/` — expect PASS (the correspondence guard proves both directions; the embed guard proves the twin).
- [ ] **Step 6: Commit** (explicit paths: the five Go files, the two test files touched in `internal/config`, `internal/app/config_test.go`, `.docket.example.yml`, and the regenerated embedded files): `git commit -m "feat(0421): add build.max_attempts and run.max_attempts config leaves"`.

---

### Task 2: Typed context — `PrepareBuild.MaxAttempts`

**Build profile:** economy

Reason: a two-field mirror with an exact in-file precedent one struct above (`PrepareFinalize.ResolverMaxAttempts`) and an existing parity-test pattern.

**Files:**
- Modify: `internal/app/repository_prepare.go` (`PrepareBuild` struct ~lines 109–112; assembly site ~lines 455–467)
- Test: `internal/app/repository_prepare_test.go`

**Interfaces (Produces):** `PrepareBuild.MaxAttempts int` (JSON `max_attempts`) — the value `context.implementation` hands the build role; the build skill reads it as `build_max_attempts` alongside `build_gate`/`build_test_command`.

- [ ] **Step 1: Write the failing test.** Find the existing `PrepareFinalize`/`PrepareBuild` parity or assembly test in `repository_prepare_test.go` (grep `ResolverMaxAttempts`); clone its pattern to assert that a repo configured with `build:\n  max_attempts: 6\n` yields `PrepareBuild.MaxAttempts == 6` and that the default yields 4. Assert the **resolved non-default** value, not just the default — a defaulted mirror hides broken wiring (learnings: `defaulted-param-hides-caller-wiring`).
- [ ] **Step 2: Run it, confirm compile failure/FAIL:** `go test -count=1 ./internal/app/ -run RepositoryPrepare`.
- [ ] **Step 3: Implement:**

  ```go
  // MaxAttempts is the resolved build.max_attempts cap (change 0421): how many
  // logical full-suite attempts the build phase may reserve, counting the
  // initial run. Mirrored from config so the build skill reads it here rather
  // than counting runs itself.
  MaxAttempts int `json:"max_attempts"`
  ```

  and in the assembly literal: `MaxAttempts: cfg.Build.MaxAttempts.Value,`.
- [ ] **Step 4: Run and pass:** `go test -count=1 ./internal/app/ -run RepositoryPrepare`.
- [ ] **Step 5: Commit:** `git commit -m "feat(0421): carry build.max_attempts through PrepareBuild typed context"`.

---

### Task 3: Outer gate durable counted budget — store primitives (schema v4)

**Build profile:** premium

Reason: durable-state schema bump with legacy handling and a concurrency CAS — consequential, correctable, with named hazards (double-grant, legacy reinterpretation).

**Files:**
- Modify: `internal/app/rungate_store.go`
- Test: `internal/app/rungate_store_test.go`

**Interfaces (Produces):**
- `GateRecord.AttemptLimit int` (JSON `attempt_limit`) — snapshotted `run.max_attempts`, set at mint, immutable thereafter
- `GateRetryUsage(repoDir, key string) (used int, err error)` — counts consumed retry markers (authority)
- `ConsumeGateRetry(repoDir, key string, attempt, limit int) (granted bool, err error)` — **changed signature**: per-attempt CAS; grants iff the marker for `attempt` did not exist and `attempt < limit`
- `gateSchemaVersion = 4`

Design (locked):
- The single `retry-consumed` marker generalizes to per-attempt markers `retry-consumed-<n>` (n = the attempt number whose completed-incomplete observation is being retried; the original dispatch is attempt 1, so a default limit 2 spends exactly `retry-consumed-1`). Marker files remain the authority; `GateRecord.Retry` stays a best-effort readable mirror with its existing `unused|consumed` vocabulary (consumed == at least one marker exists), so no JSON consumer breaks.
- Attempts used = `1 + count(markers)`; a **bare legacy `retry-consumed` marker counts as one consumed marker** — an older consumed marker must never read as unused configurable budget.
- Legacy records: bump `gateSchemaVersion` to 4 and extend the existing version-mismatch comment block (~lines 55–62). A v3 (or older) record fails closed on load with the existing schema-mismatch diagnostic — the actionable recovery it names (`gate-before --resume` re-arm for a still-valid in-progress change) is the supported path, exactly the 0407 precedent. This satisfies the spec's "otherwise refuse safely with an actionable diagnostic" arm; the "preserve when readable" arm is satisfied for markers (above), and a silent v3→v4 migration is deliberately rejected — record why in the version comment.
- `AttemptLimit` is stamped by `MintGateRecord` (Task 4 threads the value); `SaveGateRecord` must refuse to persist a record whose `AttemptLimit < 1` when `Schema == 4` (a corrupt/unstamped record fails closed like the partial-triple checks).

- [ ] **Step 1: Write the failing tests** in `rungate_store_test.go` (extend existing store tests; keep their fixture helpers):
  - `TestConsumeGateRetryPerAttemptCAS`: with limit 3 — `ConsumeGateRetry(dir, key, 1, 3)` grants; a second call for attempt 1 returns `false` (marker exists); `ConsumeGateRetry(dir, key, 2, 3)` grants; `ConsumeGateRetry(dir, key, 3, 3)` returns `false` without creating a marker (`attempt < limit` violated — assert no `retry-consumed-3` file). `GateRetryUsage` reports 2.
  - `TestConsumeGateRetryLimitOne`: limit 1 never grants and creates no marker.
  - `TestGateRetryUsageCountsLegacyMarker`: hand-create a bare `retry-consumed` file in the key dir; `GateRetryUsage` reports 1 and `ConsumeGateRetry(..., 1, 2)` refuses (attempt 1 already consumed — implement by treating the legacy marker as the attempt-1 marker).
  - `TestLoadGateRecordRefusesV3`: write a v3-shaped record.json; `LoadGateRecord` fails with the schema-mismatch diagnostic (assert the typed error, not prose).
  - `TestSaveGateRecordRefusesUnstampedLimit`: a v4 record with `AttemptLimit: 0` refuses to persist (corrupt-record error).
  - Concurrency: extend the existing concurrent-consume test (grep `Concurrent` in the file) so N goroutines racing `ConsumeGateRetry(dir, key, 1, 2)` grant exactly once.
- [ ] **Step 2: Run, confirm FAIL/compile error:** `go test -count=1 ./internal/app/ -run 'GateRetry|GateRecord|RunGateStore'`.
- [ ] **Step 3: Implement** in `rungate_store.go`:
  - `const gateSchemaVersion = 4` with an extended history comment (v4: counted retry budget, `AttemptLimit` snapshot, per-attempt markers; v3 fails closed — never reinterpret an older consumed marker as unused budget).
  - `AttemptLimit int \`json:"attempt_limit"\`` on `GateRecord` with a comment naming `run.max_attempts` and the snapshot rule.
  - Marker helpers:

    ```go
    // gateRetryMarkerFor names the per-attempt CAS marker: creating it grants the
    // single retry that moves attempt n to n+1. The bare legacy name (schema v3's
    // single-permit marker) is read as the attempt-1 marker so an already-consumed
    // legacy permit can never be re-granted.
    func gateRetryMarkerFor(attempt int) string { return fmt.Sprintf("%s-%d", gateRetryMarkerName, attempt) }
    ```

  - `ConsumeGateRetry(repoDir, key string, attempt, limit int) (bool, error)`: validate key; refuse (`false, nil`) when `attempt >= limit` or `attempt < 1` **before** any filesystem write; for attempt 1, first `os.Stat` the legacy `retry-consumed` name and treat existence as already-spent; then the existing O_CREATE|O_EXCL create on `gateRetryMarkerFor(attempt)` decides the race exactly as today (lost CAS → `false, nil`). Keep the best-effort `Retry = RetryConsumed` mirror flip.
  - `GateRetryUsage`: `os.ReadDir` the key dir, count entries matching the marker shape (legacy name + `retry-consumed-<n>`), return the count. Not load-bearing for grants (the CAS is), but the diagnostics surface.
  - `SaveGateRecord`: add the `Schema == gateSchemaVersion && rec.AttemptLimit < 1` corrupt-record refusal beside the partial-triple/pair checks. Fix every existing call site that constructs records (grep `MintGateRecord|GateRecord{`) to stamp a valid limit — test fixtures included (a temporary default of 2 in fixtures preserves their old semantics).
- [ ] **Step 4: Run and pass:** `go test -count=1 ./internal/app/ -run 'GateRetry|GateRecord|RunGateStore'`.
- [ ] **Step 5: Mutation-probe the two hazards** (backup with `cp` first, restore from the copy): (a) invert the `attempt >= limit` refusal to `>` — `TestConsumeGateRetryPerAttemptCAS`'s no-third-grant assert and `TestConsumeGateRetryLimitOne` must redden; (b) drop the legacy-marker stat — `TestGateRetryUsageCountsLegacyMarker` must redden. Run with `-count=1`. Record both probes in the commit message body.
- [ ] **Step 6: Commit:** `git commit -m "feat(0421): counted per-attempt outer-gate retry budget (GateRecord schema v4)"`.

---

### Task 4: Outer gate policy — mint snapshot + verdict wiring + diagnostics

**Build profile:** premium

Reason: this is the semantic heart of the outer half — grant atomicity, continuation/waiting non-consumption, halt precedence — with named ordering mutations already pinned by existing tests that must keep protecting the reordered code.

**Files:**
- Modify: `internal/app/rungate_verdict.go` (`VerdictRunIncomplete` case ~lines 239–276; `RunGateVerdictResult` struct — locate by grep)
- Modify: the `MintGateRecord` caller in the gate-before path (locate: `grep -rn "MintGateRecord" internal/`) so the mint snapshots `run.max_attempts` from the same authoritative config load the facade command already performs (if the gate-before command does not currently resolve config, load it through the existing app config loader used by `OperationConfig` — never a second resolver)
- Test: `internal/app/rungate_verdict_test.go`, plus the integration surfaces `internal/app/change_integration_test.go` and `internal/app/app_concurrency_race_integration_test.go` (extend the existing single-grant CAS coverage — grep `TestRunGateVerdictConcurrentRetryGrantsOnce` and the race test that exercises it under `-race`)

**Interfaces (Consumes):** Task 3's `ConsumeGateRetry(repoDir, key, attempt, limit)`, `GateRetryUsage`, `GateRecord.AttemptLimit`. Task 1's `Effective.Run.MaxAttempts`.
**Interfaces (Produces):** `RunGateVerdictResult` gains `AttemptsUsed int \`json:"attempts_used,omitempty"\`` and `AttemptLimit int \`json:"attempt_limit,omitempty"\`` (additive; HumanText may append a `attempts: <used>/<limit>` line, but the `gate-retry-once`/`gate-stop` report tokens are untouched).

Design (locked):
- Current attempt number derives from the marker authority: `attempt = 1 + used` where `used = GateRetryUsage(...)`. The grant call becomes `ConsumeGateRetry(repoDir, key, attempt, rec.AttemptLimit)`; `granted == true` → `gate-retry-once` (unchanged report line, still a grant for exactly one next dispatch); `granted == false` → the existing terminal `gate-stop` line. This is atomic and tied to the attempt transition: two concurrent observers of the same completed attempt race on the same marker, one grants; a repeat observation after the grant sees the marker and stops — today's semantics, generalized.
- Order stays: outer-takeover continuation check **before** any consumption (the existing `[ORDERING MUTATION]` comment and `TestVerdictIncompleteWithTrackedDriveContinuesWithoutRetry` must survive verbatim); consume **before** choosing the report (a lost retry stays the safe failure). `run-waiting` (`gateContinueFromWaiting`), tracked-drive recovery, and routine observation paths are untouched — they must consume nothing (they never reach the CAS).
- `run-halted` remains terminal ahead of any counting; a `LoadGateRecord` failure (including a legacy v3 record) keeps its existing fail-closed disposition — uncertain state cannot fabricate permission.
- If `AttemptLimit` reads 0 from a record that somehow bypassed the save guard, treat as the safe minimum 1 (no grant) — never as unlimited.

- [ ] **Step 1: Write the failing tests:**
  - `TestVerdictIncompleteRespectsAttemptLimit`: table-driven over limits {1, 2, 4}: with limit 1 the first eligible incomplete → `gate-stop` and no marker; with limit 2 → one `gate-retry-once` then `gate-stop`; with limit 4 → exactly three `gate-retry-once` grants across successive eligible incompletes, each on a distinct attempt transition, then `gate-stop`. Assert `AttemptsUsed`/`AttemptLimit` in the result JSON at each step.
  - `TestVerdictIncompleteRepeatObservationDoesNotDoubleGrant`: after a `gate-retry-once` for attempt 1, a second verdict call *without* a new attempt completing → `gate-stop` (marker already present), and `GateRetryUsage` still 1.
  - Extend `TestRunGateVerdictConcurrentRetryGrantsOnce` (and its `-race` twin in `app_concurrency_race_integration_test.go`) to run at limit 3: N concurrent verdicts on the same completed attempt grant exactly one retry total — a counted budget must not let concurrency spend several future attempts.
  - `TestVerdictHaltPrecedenceOverBudget`: a `run-halted` verdict with a fresh (unspent) limit-4 record → `gate-stop`/halted, zero markers.
  - `TestMintSnapshotsRunMaxAttempts`: arming against a repo configured `run:\n  max_attempts: 3\n` produces a record with `AttemptLimit == 3`; default repo → 2; a config edit after mint does not change the loaded record's limit (snapshot rule).
  - `TestVerdictContinuationConsumesNoAttempt`: assert the existing continuation path (scope-bound incomplete → `gate-continue`) leaves `GateRetryUsage` at 0 — the assert that reddens if someone moves the CAS above the takeover check.
- [ ] **Step 2: Run, confirm FAIL:** `go test -count=1 ./internal/app/ -run 'Verdict|Mint'`.
- [ ] **Step 3: Implement** per the locked design. Keep both existing `[MUTATION]`/`[ORDERING MUTATION]` comments accurate — update their prose to the counted form, and verify the tests they cite still exist and still cover the reordered code.
- [ ] **Step 4: Run the package (including race integration):** `go test -count=1 -race ./internal/app/ -run 'Verdict|Gate|Mint'` — expect PASS.
- [ ] **Step 5: Mutation-probe:** (a) swap the consume-then-report order to report-then-consume — the concurrency test must redden; (b) replace the derived `attempt` with a constant 1 — `TestVerdictIncompleteRespectsAttemptLimit`'s limit-4 leg must redden (it would grant unbounded retries on marker-1 collisions… verify it actually reddens; if the assert stays green, the test is decoration — fix the test, not the probe). Restore from `cp` backups; run with `-count=1`.
- [ ] **Step 6: Commit:** `git commit -m "feat(0421): outer run gate grants retries from the snapshotted run.max_attempts budget"`.

---

### Task 5: Build suite-attempt budget store (`internal/gatedrive`)

**Build profile:** premium

Reason: net-new durable machinery, but with the store discipline fully prescribed by two in-package precedents (`store.go`, `scope.go`) and the semantics locked below; the hazard is concurrency/idempotency, which the prescribed CAS pattern and tests target.

**Files:**
- Create: `internal/gatedrive/suitebudget.go`
- Test: `internal/gatedrive/suitebudget_test.go`

**Interfaces (Produces):**

```go
// SuiteBudgetKey identifies one owning build phase's full-suite attempt budget.
type SuiteBudgetKey struct {
    RepoIdentity string
    ChangeID     string
    Phase        string
}

// ReserveSuiteAttempt durably reserves one logical full-suite attempt for key,
// creating the budget record with the snapshotted limit on first reservation.
// It returns the attempt number reserved (1-based) and the snapshot actually in
// force. A spent budget returns ErrSuiteBudgetExhausted (typed StoreError cause)
// with no state change; there are no refunds, so a lost launch can never
// overrun the configured bound.
func (s *Store) ReserveSuiteAttempt(key SuiteBudgetKey, limit int) (attempt, snappedLimit int, err error)

// SuiteBudgetUsage reports (used, limit) without reserving; (0, 0, nil) when no
// record exists yet.
func (s *Store) SuiteBudgetUsage(key SuiteBudgetKey) (used, limit int, err error)
```

Design (locked):
- Storage: `<git-common-dir>/docket/gate-suite-budgets/v1/<id>/record.json`, where `<id>` is the lowercase hex sha256 of `RepoIdentity + "\x00" + ChangeID + "\x00" + Phase` (path-safe by construction, mirrors `capHash`). Record `{schema_version: 1, repo_identity, change_id, phase, limit, used}` inside the same `storedScope`-style generation envelope; 0700 dir, 0600 file, `writeAtomicJSON`, per-record flock + generation CAS copied from `scopeCAS` (~`scope.go:307`). Unknown schema versions fail closed with a typed `StoreError` — the package's uniform rule.
- Snapshot: the `limit` parameter is consulted **only** when the record is being created (first reservation of the phase — "when its owning gate scope begins"); every later reservation enforces the stored `limit` and ignores the parameter. This is how config edits mid-phase cannot rewrite an owned budget, and how recovery/continuation preserve consumed budget (the record simply persists).
- Capacity guard before increment, exactly like `finalize_reserve.go`'s admission cap (~line 236): `used == limit` → exhausted, state untouched.
- `limit < 1` at creation is an invalid-request typed error (config validation makes it unreachable; fail closed anyway).

- [ ] **Step 1: Write the failing tests** (`suitebudget_test.go`, using the package's existing temp-repo store fixtures — grep how `scope_test.go` builds a `Store`):
  - `TestReserveSuiteAttemptCountsAndExhausts`: limit 4 → reservations return attempts 1,2,3,4; the fifth returns the exhausted error with `SuiteBudgetUsage` = (4, 4); no record field changed by the refused call.
  - `TestReserveSuiteAttemptSnapshotsLimitOnCreate`: first reserve with limit 4, second reserve passing limit 99 → still capped at 4 (`snappedLimit == 4`); a fresh key with limit 1 → attempt 1 granted, second reserve exhausted.
  - `TestReserveSuiteAttemptLimitOne`: limit 1 grants exactly the initial attempt.
  - `TestReserveSuiteAttemptConcurrent`: N goroutines racing on one key with limit 4 → exactly 4 distinct attempt numbers granted, the rest exhausted (run under `-race`).
  - `TestSuiteBudgetPersistsAcrossStoreReopen`: reserve twice, construct a new `Store` over the same repo dir, `SuiteBudgetUsage` = (2, 4) — the interruption/continuation-preservation assert.
  - `TestReserveSuiteAttemptInvalidLimit`: limit 0 → typed invalid-request error, no record created.
- [ ] **Step 2: Run, confirm FAIL:** `go test -count=1 ./internal/gatedrive/ -run SuiteBudget`.
- [ ] **Step 3: Implement `suitebudget.go`** per the locked design, reusing `writeAtomicJSON`, the flock helper, and the generation-CAS retry loop (`ownerCASMaxAttempts`) rather than re-deriving them.
- [ ] **Step 4: Run and pass (with race):** `go test -count=1 -race ./internal/gatedrive/ -run SuiteBudget`.
- [ ] **Step 5: Commit:** `git commit -m "feat(0421): durable per-phase suite-attempt budget store in gatedrive"`.

---

### Task 6: Build budget enforcement at the drive-start seam + facade diagnostics

**Build profile:** premium

Reason: the enforcement-point choice is the change's central build-side claim (no worker can bypass the cap); wiring it wrong is consequential and the bypass hazard is named.

**Files:**
- Modify: `internal/app/gate_drive.go` (`StartDrive` request path, ~lines 127–291; result/reason surface)
- Modify: `internal/gatedrive/driver.go` only if the reservation must sit inside the engine's `StartDrive` (decide at build: the reservation MUST be ordered before the drive record is created and before any launch — put it at the last common point through which every build-owned start flows; verify by reading how `internal/cli`'s gate commands and `finalize_rebase.go` reach `StartDrive`, and confirm finalize's drives use a different owner so they are not charged)
- Test: `internal/app/gate_drive_test.go`, `internal/gatedrive/driver_test.go` (only if the engine changed)

**Interfaces (Consumes):** Task 5's `ReserveSuiteAttempt`/`SuiteBudgetUsage`; Task 1's `Effective.Build.MaxAttempts`.
**Interfaces (Produces):** a build-owned `gate.drive.start` that is refused on exhaustion with a stable reason naming `build.max_attempts` and used/limit, e.g. `Reason: "suite-attempts-exhausted"` plus HumanText `the build full-suite attempt budget is spent (4/4 used); raising build.max_attempts takes effect on the next build phase, not this one — halt per the build skill's halting conditions`.

Design (locked):
- **Reservation trigger:** a `StartDrive` whose resolved owner is the build role **and** whose identity bundle carries a non-empty `ChangeID` (change 0416 guarantees scoped starts carry the full bundle: change/task/phase identity). Key: `{RepoIdentity, ChangeID, Phase: "build"}` — the phase literal, not `req.Phase`, so a task-owned focused-test start (`--owner task`) is never charged and a repair worker's build-owned rerun in the same phase always is. Limit: `cfg.Build.MaxAttempts.Value` from the same authoritative config resolution `StartDrive` already performs to resolve the build-owned command (never a second resolver).
- **Ordering:** reserve **before** the drive record is created or any process launches. A reservation that succeeds but whose drive creation then fails is a spent attempt — no refunds (the admission-cap rule; a lost dispatch can never overrun the bound).
- **What is never charged:** `gate.drive.advance` (observation), driver relaunch recovery (`driver.go` `Attempt++` ~line 445 — a one-relaunch recovery of the *same* logical run), `Takeover` (ownership transfer), handoff claim / `run_waiting` continuation, `PrepareScope`. None of these paths call `StartDrive`, so the invariant holds structurally — assert it anyway (Step 1) so a refactor cannot silently move the charge.
- **Boundary kept:** a build-owned start with **no** ChangeID (scopeless ad-hoc drive, pre-0359 behavior) is unbudgeted — existing behavior preserved, documented in the function comment and pinned by a test.
- **Skipped gate:** `build_gate: off` launches no suite and therefore reserves nothing (it never reaches `StartDrive`); no code needed, but the Task 7 prose states it.

- [ ] **Step 1: Write the failing tests** in `gate_drive_test.go` (use its existing fake-engine/repo fixtures):
  - `TestBuildOwnedStartReservesSuiteAttempt`: four build-owned starts for one change succeed carrying attempt numbers 1–4; the fifth is refused with the exhausted reason and **no drive is created** (assert engine saw no fifth `NewDrive`).
  - `TestBuildStartLimitOne`: repo configured `build:\n  max_attempts: 1\n` → the first start succeeds, the second is refused.
  - `TestTaskOwnedStartNotBudgeted`: task-owned starts for the same change never touch the budget (`SuiteBudgetUsage` stays 0).
  - `TestScopelessBuildStartNotBudgeted`: build-owned start with empty ChangeID succeeds regardless of a spent budget for some change.
  - `TestAdvanceRecoverTakeoverDoNotCharge`: after one budgeted start, exercise advance/recovery/takeover fakes and assert usage still 1.
  - `TestExhaustionDiagnosticNamesKnob`: the refusal HumanText contains `build.max_attempts` and `4/4`.
- [ ] **Step 2: Run, confirm FAIL:** `go test -count=1 ./internal/app/ -run 'BuildOwnedStart|BuildStart|Budget'`.
- [ ] **Step 3: Implement** per the locked design.
- [ ] **Step 4: Run and pass (race included):** `go test -count=1 -race ./internal/app/ ./internal/gatedrive/`.
- [ ] **Step 5: Mutation-probe the bypass hazard:** temporarily key the reservation on `req.Phase` instead of the `"build"` literal with an empty phase — `TestBuildOwnedStartReservesSuiteAttempt` or `TestTaskOwnedStartNotBudgeted` must redden; and delete the reservation call entirely — the exhaustion tests must redden (proves the guard is code, not decoration). Restore from `cp` backups; `-count=1`.
- [ ] **Step 6: Commit:** `git commit -m "feat(0421): build-owned drive starts reserve the phase suite-attempt budget"`.

---

### Task 7: Prose, docs, and generated surfaces — replace the fixed-limit language

**Build profile:** premium

Reason: maintained contract prose is executable surface here (agents run it); the hazards are the enumerated-floor (a missed site keeps instructing the old fixed limit) and reddening prose sentinels that pin the copies — both need a whole-repo derivation pass, plus exact-count budget re-baselines.

**Files:**
- Modify: `skills/docket-build/SKILL.md` (halting-conditions bullet "The suite is still red after the max repair — there is no second repair round" ~line 219; the Red paragraph "no repeated repair/review loop; failure after the max repair path halts" ~lines 279–290)
- Modify: `skills/docket-build/references/gate-caller-loop.md` (any fixed-repair-bound language — derive)
- Modify: `skills/docket-implement-next/SKILL.md` (the "Verify the run" paragraph's continuation-vs-`gate-retry-once` prose ~line 142)
- Modify: `docs/concepts/run-gate.md` (~lines 46–47, the `gate-retry-once` legend) and `docs/concepts/build-profiles-and-gate.md` (repair narrative)
- Modify: `internal/repoguard/budgets_test.go` (re-baselines) and any prose-sentinel tests the suite reddens
- Regenerate: `go generate ./internal/assets/` (embedded twins of every edited skill file)

**Interfaces (Consumes):** Task 2's `build_max_attempts` in the implementation context; Task 6's exhaustion refusal; Task 4's unchanged `gate-retry-once` report line.

- [ ] **Step 1: Derive the site list (never hand-enumerate).** From the worktree root:

  ```bash
  grep -rniE "one retry|single retry|retry-once|exactly one|second repair|max repair|no repeated repair|three repair" \
    skills/ docs/ AGENTS.md CLAUDE.md README.md --include="*.md" | grep -v "\.worktrees/"
  ```

  Sort hits into: (a) **executable/maintained** — edit only where the sentence describes one of THESE TWO budgets as fixed; (b) **still-true** — `gate-retry-once` being "a grant for exactly one next dispatch" is unchanged, per-grant language stays; (c) **point-in-time records** (archived changes, specs, results, Accepted ADRs, `fix-loop.md`'s REVIEW bound) — never touched. Record the sorted list in the task's commit message body.
- [ ] **Step 2: Edit `skills/docket-build/SKILL.md`.** Replace the single-repair-cycle contract with the budgeted loop, reading the limit from the implementation context (`build_max_attempts`, default 4), e.g.:
  - The Red paragraph: each red full-suite result, while the budget admits another attempt, becomes exactly one synthetic integration-repair task on the ladder `premium -> max`; the repair worker's post-fix full-suite rerun **is** the next budgeted attempt (started build-owned through the same driver, so the facade charges it — no bypass); a refused start (`suite-attempts-exhausted`) or a red final permitted run halts per *Halting conditions* with the exhaustion reason naming `build.max_attempts` and used/limit. `build_max_attempts: 1` means a red initial run halts with no repair cycle. Green at any point ends the phase immediately; review is never invoked while red.
  - The halting bullet: "**The suite-attempt budget is exhausted and the suite is still red** — the last permitted full-suite run failed, or `gate.drive.start` refuses with `suite-attempts-exhausted`; there is no repair round beyond the budget."
  - State explicitly: `build_gate: off` runs no suite and spends no attempt; infrastructure/unavailable-result/config-gap/observation-budget halts are unchanged and are **not** red results to repair.
- [ ] **Step 3: Edit the remaining maintained surfaces** per the Step-1 sort: `gate-caller-loop.md` (align its loop bound language), `docket-implement-next/SKILL.md` (where it implies the facade can grant only one retry ever, generalize to: each `gate-retry-once` authorizes exactly one next dispatch, and the facade may issue another on a later eligible attempt when the configured `run.max_attempts` budget allows; continuations still consume none), `docs/concepts/run-gate.md` (annotate the `gate-retry-once` legend line: "exactly one more launch, same key — granted at most `run.max_attempts - 1` times"), `docs/concepts/build-profiles-and-gate.md` (repair narrative gains the budget). Do **not** edit the root `AGENTS.md`/`CLAUDE.md` run-gate block unless Step 1 shows a sentence that is now false — its per-grant wording ("Only `gate-retry-once` authorizes another dispatch … once") remains true per grant; promotion of new prose there is a human decision.
- [ ] **Step 4: Regenerate embedded assets:** `go generate ./internal/assets/`.
- [ ] **Step 5: Run the full Go suite once** (`go test -count=1 ./...`) and fix what the prose edits reddened: budget pins in `internal/repoguard/budgets_test.go` (re-baseline at the exact new counts with a `// 0421:` note, house style), and any prose sentinels that grep the replaced sentences (learnings: `restatement-accumulates-its-own-guards` — a sentinel pinning the OLD copy must be rewritten to pin the NEW claim meaningfully, bound phrase-to-claim, not deleted).
- [ ] **Step 6: Commit** (skill files, docs, budgets_test re-baseline, regenerated embedded tree): `git commit -m "docs(0421): budgeted repair and retry language across maintained gate surfaces"`.

---

### Task 8: Integration coverage + final sweep

**Build profile:** standard

Reason: cross-package integration assertions over machinery Tasks 3–6 built, plus a mechanical closing audit; everything is prescribed.

**Files:**
- Test: `internal/app/change_integration_test.go`, `internal/app/app_concurrency_race_integration_test.go` (extend), `internal/gatedrive/driver_test.go` / `integration_test.go` (only if Task 6 touched the engine)

- [ ] **Step 1: Write the integration tests** (extend the existing end-to-end gate fixtures — grep how `change_integration_test.go` drives gate-before → verdict):
  - End-to-end outer budget: arm, attribute, drive an incomplete → `gate-retry-once`; second incomplete → `gate-stop` (default 2); repeat at configured 3 → two retries; at 1 → none. Assert the report lines parse exactly as before (same leading tokens).
  - Continuation non-consumption end-to-end: a `run-waiting`/`gate-continue` path across a simulated interruption preserves usage (reopen stores, re-verdict, assert `GateRetryUsage` unchanged).
  - Build budget across interruption: reserve 2 of 4, reconstruct the app service over the same repo (fresh `Store`), start again → attempt 3 (preserved, not reset).
- [ ] **Step 2: Run the affected packages with race:** `go test -count=1 -race ./internal/app/ ./internal/gatedrive/ ./internal/config/` — expect PASS.
- [ ] **Step 3: Closing audit (record results in the commit body):**
  - Re-run the Task 7 Step-1 grep — zero remaining maintained-surface hits describing these two budgets as fixed.
  - `grep -rn "retry-consumed\b" internal/ --include="*.go"` — every remaining bare-name site is the legacy-compat read path or a test fixture, none a grant path.
  - Confirm no edit touched `fix-loop.md`, `finalize.resolver_max_attempts` behavior, or any file under `docs/changes/archive/`, `docs/adrs/` (Accepted), `docs/results/`.
  - Confirm the ADR note (Global Constraints, "Decisions to record") is surfaced in the coordinator's results context for the parent's Step 6.
- [ ] **Step 4: Commit:** `git commit -m "test(0421): integration coverage for counted gate budgets + closing audit"`.

After this task the build controller applies its own full-suite gate per its contract (`build_gate: local`, resolved `build.test_command` = `go run ./cmd/docket development test`), reads the budget report even on green, and proceeds — none of that is a plan task.

---

## Acceptance-criteria coverage map (spec §"Acceptance and validation")

| Criterion | Task(s) |
|---|---|
| Defaults 4/2, precedence + provenance, value 1 valid, reject invalid types / non-positives, values above default | 1 |
| Build limit 4: three reds + fourth green succeeds; four reds halt, fifth never launched; early green stops; limit 1 no repair cycle | 5, 6 (Go enforcement), 7 (contract prose) |
| Run limit 2: one retry then stop; limit 1 none; larger caps grant exactly that many distinct eligible transitions | 3, 4, 8 |
| Repeat/concurrent observations cannot duplicate grants or exceed either bound | 3 (store CAS), 4 (verdict, -race), 5–6 (budget CAS), 8 |
| Resume/ownership-transfer preserve usage; continuations never charged twice; config edits don't rewrite an owned snapshot | 4 (mint snapshot), 5 (create-time snapshot, reopen persistence), 6 (no-charge paths), 8 |
| Explicit halts / invalid or unattributed ownership / infrastructure failures / unavailable results stay non-retryable; build-off + exact-HEAD evidence unchanged | 4 (halt precedence), 6 (off/scopeless boundaries), 7 (prose) |
| Legacy durable-state handling; typed context propagation; diagnostics; generated caller/build instruction consistency; mutation-tested guards | 3 (v3 + legacy marker), 2 (context), 1+4+6 (diagnostics), 7 (generated surfaces), 3–6 mutation probes |
| Whole-suite gate via the Go runner + budget report read | build controller's own gate after Task 8 |
