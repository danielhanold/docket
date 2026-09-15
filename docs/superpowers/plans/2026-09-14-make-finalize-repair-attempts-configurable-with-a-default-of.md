<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0419 — Make finalize repair attempts configurable with a default of six](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0419-make-finalize-repair-attempts-configurable-with-a-default-of.md)**
<!-- docket:backlink:end -->
# Configurable Finalize Repair Attempts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the finalize integration-repair attempt cap a configurable `finalize.repair_max_attempts` (default 6, minimum 1) that the repair workflow actually obeys, and raise the built-in `finalize.resolver_max_attempts` default from 3 to 10.

**Architecture:** A new positive-int config leaf resolves through the normal layers (repo-local > repo-committed > global > built-in, ADR-0019 `scopeAny`), surfaces in `PreparedContext.Finalize` and the effective-config diagnostics, and rides into the `docket-integration-repair` dispatch payload authored by the finalize skill. The repair agent's hardcoded "at most two attempts" contract becomes "bounded to the dispatched repair-attempt budget"; exhaustion keeps the existing `stuck`/`halted` path. The resolver default change is a pure default bump — explicit overrides, `intLeaf(1)` validation, and receipt-snapshotted budgets are untouched.

**Tech Stack:** Go (internal/config, internal/app), maintained skill/agent markdown, generated embedded-tree assets (`go generate ./internal/assets`), harness golden wrappers (`go test -update`), Go-native suite runner.

**Spec:** none — change 0419 is trivial; the plan argues from the change file `docs/changes/active/0419-make-finalize-repair-attempts-configurable-with-a-default-of.md` (on the `docket` metadata branch; synchronized copy under `.docket/`).

## Global Constraints

- Repair default is **6**, resolver default becomes **10**; both are finite positive integers, minimum 1 (`intLeaf(1)`). No unlimited sentinel, no upper ceiling.
- Precedence for the new leaf: repository-local > repository-committed > global > built-in (`scope: scopeAny`, `disp: dispSupported`, `merge: mergeScalar`), exactly like `finalize.resolver_max_attempts`.
- The **initial** repair attempt counts toward the maximum (a budget of 1 means one fix attempt total); repair stops early on success; an exhausted budget maps to the existing `disposition: stuck` → `halted` path — no new disposition vocabulary (prohibition-needs-a-return-value: the exhaustion prohibition maps to `stuck`).
- Repair attempts stay separate from conflict-resolver dispatches: no change to resolver counting, durable reservation semantics (`finalize.resolver-reserve`), or receipt snapshotting. Existing owned rebase receipts retain their snapshotted budget by construction — do not touch `internal/app/finalize_reserve.go` / `finalize_rebase.go` logic.
- No Go-side enforcement of the repair cap exists or is added: the cap crosses to the agent through the dispatch payload the finalize skill authors, and the agent's contract enforces it. Assert the resolved non-default value at every propagation point (defaulted-param-hides-caller-wiring).
- Frozen surfaces stay byte-identical: `internal/install/legacydata/`, `internal/install/testdata/`, `docs/results/`, `docs/superpowers/plans/` and `docs/superpowers/specs/` history, and the accepted text of `docs/adrs/0010-*.md`. The "two attempts" wording in `internal/gatedrive/ownership.go`, `internal/gatedrive/integration_test.go`, and `internal/app/app_concurrency_race_integration_test.go` is unrelated (gate-run attempts, not repair) — leave it.
- Generated surfaces are regenerated, never hand-edited: `internal/assets/embedded/tree/**` via `go generate ./internal/assets`; `internal/harness/*/testdata/golden/**` via each harness package's `-update` flag.
- Cross-references anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054).
- Suite gate: the docket-build final gate runs the resolved `build.test_command` (`go run ./cmd/docket development test`); per-task steps run focused `go test ./<pkg>/ -count=1` (cached-runner-serves-a-mutated-tree).
- Before deleting or rewording any prose, grep the whole repo for the phrase being removed and sort hits into maintained vs frozen (restatement-accumulates-its-own-guards); collapse whitespace when grepping wrapped prose (phrase-grep-over-wrapped-prose).
- Ship the knob end-to-end: sample config, reference docs, and relaxed prose land in this change, written for a reader who never heard of change 0419 (config-knob-ship-end-to-end).

---

### Task 1: Raise the `finalize.resolver_max_attempts` built-in default to 10

**Files:**
- Modify: `internal/config/schema.go` (the `finalize.resolver_max_attempts` registry row, `def: 3` → `def: 10`)
- Modify: `internal/config/defaults.go` (`ResolverMaxAttempts: builtinValue(3)` → `builtinValue(10)`)
- Modify: `internal/config/schema_test.go` (the registry-defaults table entry `"finalize.resolver_max_attempts": 3` and any default-value row asserting 3)
- Modify: `.docket.example.yml` (the `resolver_max_attempts` comment and value under `finalize:`)
- Test: `internal/config/schema_test.go`, `internal/config/defaults_test.go`, `internal/config/resolve_test.go`

**Interfaces:**
- Produces: built-in default 10 for `Finalize.ResolverMaxAttempts`; everything else about the leaf (JSON tag, validation, precedence) unchanged. Task 4 rewrites the "default 3" prose in skill references; Task 5 regenerates the embedded example twin.

- [ ] **Step 1: Write the failing test change**

In `internal/config/schema_test.go`, update the expected-defaults map entry (currently `"finalize.resolver_max_attempts":   3,`) to `10`, and any leaf-default test row that resolves the empty input to `3` for this path. `TestBuiltinEffectiveMatchesRegistryDefaults` in `internal/config/defaults_test.go` compares `defaults.go` against the registry `def` cells, so it needs no edit — it will red if the two files disagree mid-change. Leave every explicit-override row (`"5"` → 5, `"1"` → 1) and `TestPrecedenceResolverMaxAttempts` in `internal/config/resolve_test.go` untouched: they pin override preservation, which this change must not disturb.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -count=1`
Expected: FAIL — the updated default expectations (10) disagree with the registry/defaults still carrying 3.

- [ ] **Step 3: Implement the default bump**

In `internal/config/schema.go`, change the row

```go
{path: "finalize.resolver_max_attempts", kind: kindInt, def: 3,
	merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: intLeaf(1)},
```

to `def: 10`. In `internal/config/defaults.go`, change `ResolverMaxAttempts: builtinValue(3)` to `builtinValue(10)`.

- [ ] **Step 4: Update the authored example config**

In `.docket.example.yml` under `finalize:`, the entry currently reads "3 (the default) suits most rebases" and ends `resolver_max_attempts: 3`. Rewrite the default mentions to 10 and set the value line to `resolver_max_attempts: 10`, keeping the surrounding register (payoff first, no change numbers): e.g. "10 (the default) lets a long-lived branch work through many successive conflicts; lower it for a stricter ceiling." Keep the "takes effect on the next finalize you start, never on a rebase already in progress" sentence — it is the receipt-snapshot semantics, still true.

- [ ] **Step 5: Run the config package tests**

Run: `go test ./internal/config/ -count=1`
Expected: PASS (the example-correspondence guard checks key presence against the registry, not values, but keep it green). Note `internal/assets` drift tests may now red until Task 5 regenerates the embedded twin — that is expected; do not hand-edit `internal/assets/embedded/tree/.docket.example.yml`.

- [ ] **Step 6: Commit**

```bash
git add internal/config/ .docket.example.yml
git commit -m "feat(0419): raise finalize.resolver_max_attempts built-in default to 10"
```

---

### Task 2: Config leaf `finalize.repair_max_attempts` (default 6)

**Files:**
- Modify: `internal/config/schema.go` (new registry row beside `finalize.resolver_max_attempts`)
- Modify: `internal/config/config.go` (the `Finalize` struct)
- Modify: `internal/config/defaults.go` (builtin value)
- Modify: `internal/config/resolve.go` (effective-struct assignment)
- Modify: `.docket.example.yml` (documented entry under `finalize:` — required by `example_correspondence_test.go` Direction B)
- Test: `internal/config/schema_test.go`, `internal/config/decode_test.go`, `internal/config/defaults_test.go`, `internal/config/resolve_test.go`

**Interfaces:**
- Produces: `Finalize.RepairMaxAttempts Value[int]` with JSON tag `repair_max_attempts`; leaf path `finalize.repair_max_attempts`, `kind: kindInt`, `def: 6`, `validate: intLeaf(1)`, `merge: mergeScalar`, `scope: scopeAny`, `disp: dispSupported`. Task 3 maps it into `PrepareFinalize` and the diagnostics; Task 4 names it in dispatch prose.

- [ ] **Step 1: Write the failing tests**

Mirror the existing `finalize.resolver_max_attempts` coverage row-for-row, in each file's house style (read the neighboring rows first and copy their exact shape and diagnostic constants):

`internal/config/schema_test.go` — add `"finalize.repair_max_attempts"` to the known-leaf list (the block at the top listing `"finalize.resolver_max_attempts", "finalize.skip_results_only_delta",`), add `"finalize.repair_max_attempts": 6` to the registry-defaults map, and add validation rows beside the resolver ones:

```go
// finalize.repair_max_attempts: positive int, floor 1 (intLeaf(1)).
{"repair max attempts explicit", "finalize.repair_max_attempts", "9", 9, ""},
{"repair max attempts floor ok", "finalize.repair_max_attempts", "1", 1, ""},
{"repair max attempts zero", "finalize.repair_max_attempts", "0", nil, CodeInvalidValue},
{"repair max attempts negative", "finalize.repair_max_attempts", "-2", nil, CodeInvalidValue},
{"repair max attempts string", "finalize.repair_max_attempts", `"six"`, nil, CodeInvalidType},
{"repair max attempts bool", "finalize.repair_max_attempts", "true", nil, CodeInvalidType},
{"repair max attempts fraction", "finalize.repair_max_attempts", "2.5", nil, CodeInvalidType},
{"repair max attempts list", "finalize.repair_max_attempts", "[6]", nil, CodeInvalidType},
```

`internal/config/decode_test.go` — beside the resolver row:

```go
{row: "finalize.repair_max_attempts", path: "finalize.repair_max_attempts",
	block: "finalize:\n  repair_max_attempts: 9\n",
	flow:  "finalize: {repair_max_attempts: 9}\n", value: 9},
```

`internal/config/defaults_test.go` — add `"finalize.repair_max_attempts": eff.Finalize.RepairMaxAttempts.Value,` to the `TestBuiltinEffectiveMatchesRegistryDefaults` leaves map.

`internal/config/resolve_test.go` — extend the leaf-accessor switch with a `finalize.repair_max_attempts` case returning `eff.Finalize.RepairMaxAttempts` fields, and add `TestPrecedenceRepairMaxAttempts` cloned from `TestPrecedenceResolverMaxAttempts` (four-layer precedence: repo-local > repo-committed > global > built-in 6), with a fixture helper emitting `finalize:\n  repair_max_attempts: %d\n`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -count=1`
Expected: FAIL (unknown leaf `finalize.repair_max_attempts` / no field `RepairMaxAttempts`).

- [ ] **Step 3: Implement leaf, struct field, default, assignment**

`internal/config/schema.go`, directly after the resolver row:

```go
{path: "finalize.repair_max_attempts", kind: kindInt, def: 6,
	merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: intLeaf(1)},
```

`internal/config/config.go`, in `type Finalize struct` after `ResolverMaxAttempts`:

```go
// RepairMaxAttempts caps integration-repair fix attempts per red rebased
// gate (change 0419). Positive; the initial attempt counts. Enforced by the
// dispatched repair agent's contract, not by a durable Go reservation —
// unlike ResolverMaxAttempts it is never snapshotted into a receipt.
RepairMaxAttempts Value[int] `json:"repair_max_attempts"`
```

`internal/config/defaults.go`, in the `Finalize{...}` literal: `RepairMaxAttempts: builtinValue(6),`

`internal/config/resolve.go`, beside the resolver assignment:

```go
set(assign(&eff.Finalize.RepairMaxAttempts, r.declared, "finalize.repair_max_attempts"))
```

- [ ] **Step 4: Document the knob in the example config**

In `.docket.example.yml`, after the `resolver_max_attempts` entry (keeping its house format — prose comment, `# scope:` line, then the value):

```yaml
  # repair_max_attempts — how many fix attempts the integration-repair agent gets when the test
  # suite is red after finalize's rebase, the initial attempt included, before finalize stops and
  # hands the failure to you. 6 (the default) gives a stubborn semantic breakage room to converge;
  # set 1 for a single-shot repair. Must be a whole number of 1 or more. Separate from
  # resolver_max_attempts, which budgets rebase conflict resolution, not post-rebase repair.
  # scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)
  repair_max_attempts: 6
```

- [ ] **Step 5: Run the config package tests**

Run: `go test ./internal/config/ -count=1`
Expected: PASS, including `example_correspondence_test.go` in both directions.

- [ ] **Step 6: Commit**

```bash
git add internal/config/ .docket.example.yml
git commit -m "feat(0419): add finalize.repair_max_attempts config leaf (default 6, min 1)"
```

---

### Task 3: Expose the resolved cap in prepared context and config diagnostics

**Files:**
- Modify: `internal/app/repository_prepare.go` (the `PrepareFinalize` struct and the `Finalize: PrepareFinalize{...}` mapping)
- Modify: `internal/app/config.go` (the effective-config `leafLine` list)
- Test: `internal/app/repository_prepare_test.go`, `internal/app/config_test.go`

**Interfaces:**
- Consumes: `Finalize.RepairMaxAttempts Value[int]` from Task 2.
- Produces: `PrepareFinalize.RepairMaxAttempts int` with JSON tag `repair_max_attempts` (the typed value the finalize skill's Step-0 `repository.prepare` context carries into the repair dispatch payload), and a `finalize.repair_max_attempts` row in effective-config diagnostics with value and provenance.

- [ ] **Step 1: Write the failing tests**

In `internal/app/repository_prepare_test.go`, find the existing assertion on `Finalize.ResolverMaxAttempts` and add a sibling asserting `Finalize.RepairMaxAttempts`. Assert a **non-default resolved value** (fixture config setting `finalize:\n  repair_max_attempts: 9\n`, expect 9) so deleting the mapping line cannot stay green (defaulted-param-hides-caller-wiring); also assert the built-in 6 on the unconfigured fixture beside however `ResolverMaxAttempts` is pinned there. In `internal/app/config_test.go`, extend the effective-config output expectation with the `finalize.repair_max_attempts` row exactly as the `finalize.resolver_max_attempts` row is asserted (value `6`, provenance built-in, plus any explicit-layer case the file already exercises); update any expected-output literal that pins the resolver row's default `3` to `10`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestRepositoryPrepare|TestConfig' -count=1`
Expected: FAIL (no field `RepairMaxAttempts` on `PrepareFinalize`; missing diagnostics row).

- [ ] **Step 3: Implement**

`internal/app/repository_prepare.go`, in `type PrepareFinalize struct` after `ResolverMaxAttempts`:

```go
// RepairMaxAttempts is the resolved finalize.repair_max_attempts cap the
// finalize skill hands to the integration-repair dispatch (change 0419).
RepairMaxAttempts int `json:"repair_max_attempts"`
```

and in the `Finalize: PrepareFinalize{...}` literal: `RepairMaxAttempts: cfg.Finalize.RepairMaxAttempts.Value,`

`internal/app/config.go`, directly after the resolver `leafLine`:

```go
leafLine("finalize.repair_max_attempts", strconv.Itoa(eff.Finalize.RepairMaxAttempts.Value), eff.Finalize.RepairMaxAttempts.Provenance),
```

- [ ] **Step 4: Run the app package tests**

Run: `go test ./internal/app/ -count=1`
Expected: PASS. If other `internal/app` fixtures pin the old resolver default 3 in expected JSON/context output (grep `internal/app` tests for `resolver_max_attempts` and `"3"` in finalize-context/prepare expectations), update those expectations to 10 in this step — never by weakening the assert.

- [ ] **Step 5: Commit**

```bash
git add internal/app/
git commit -m "feat(0419): expose finalize.repair_max_attempts in prepared context and config diagnostics"
```

---

### Task 4: Rewire the repair-attempt contract in maintained agent and skill prose

**Files:**
- Modify: `agents/docket-integration-repair.md` (description, charter, autonomy — the three "two attempts" sites)
- Modify: `cursor-rules/dispatch/docket-integration-repair.md` (payload requirements + "at most two")
- Modify: `skills/docket-finalize-change/SKILL.md` (step 5 dispatch block, and the `resolver-budget-exhausted` "default 3" line)
- Modify: `skills/docket-finalize-change/references/gate-failure.md` ("default 3", "at most two attempts", "≤2 attempts")
- Modify: `docs/guide/proving-the-build.md` ("bounded to at most two attempts")
- Modify: `docs/reference/skills-and-agents.md` ("in at most two attempts")

**Interfaces:**
- Consumes: `PrepareFinalize.RepairMaxAttempts` (Task 3) as the value the finalize skill's Step-0 prepared context carries.
- Produces: a repair contract phrased against "the dispatched repair-attempt budget (`finalize.repair_max_attempts`, default 6)". Task 5's regeneration propagates these files into the embedded tree and harness goldens.

- [ ] **Step 1: Inventory the phrases before rewording**

Run (capture, then grep the variable — pipefail discipline):

```bash
out=$(grep -rn "at most two\|two attempts\|two repair\|≤2 attempts\|default 3" agents/ cursor-rules/ skills/ docs/guide/ docs/reference/ README.md 2>/dev/null)
printf '%s\n' "$out"
```

Every hit must be either rewritten in this task or justified frozen. Do not touch `docs/adrs/`, `docs/results/`, `docs/superpowers/`, `internal/install/legacydata/`, `internal/install/testdata/`.

- [ ] **Step 2: Rewrite `agents/docket-integration-repair.md`**

Three edits, preserving each sentence's surrounding contract:

- Frontmatter `description:`: "…writes a minimal fix in at most two attempts…" → "…writes a minimal fix within the dispatched repair-attempt budget…".
- Charter (line beginning "Charter: own every red-test outcome…"): replace "You are bounded to at most two repair attempts." with: "You are bounded to the repair-attempt budget your dispatch payload names (`repair_max_attempts`, resolved from `finalize.repair_max_attempts`; treat an unstated budget as 6, the built-in default). The initial attempt counts as attempt 1; stop as soon as the suite is green."
- Autonomy paragraph: replace "If you cannot reach green within two attempts, return `disposition: stuck`…" with "If you cannot reach green within the dispatched budget, return `disposition: stuck`…" (rest of the sentence unchanged — the stuck report stays `halted`).

- [ ] **Step 3: Rewrite the finalize skill surfaces**

`skills/docket-finalize-change/SKILL.md`:
- In the `docket:feature-dispatch` block (step 5), extend the payload list so the dispatch names the budget — after the "Feature worktree:" payload line add: `Repair-attempt budget: <finalize.repair_max_attempts from the Step-0 prepared context> (initial attempt included)`. In the same block, change "bounded two-attempt feature-branch fix" to "feature-branch fix bounded to the dispatched repair-attempt budget", and "a repair that cannot reach green in two attempts" to "a repair that cannot reach green within the configured `finalize.repair_max_attempts` budget". Validate marker order and balance before editing inside the managed block; edit only between `docket:feature-dispatch:start` and `:end`.
- In the rebase-outcomes list, change "`finalize.resolver_max_attempts` budget (default 3)" to "(default 10)".

`skills/docket-finalize-change/references/gate-failure.md`:
- "`finalize.resolver_max_attempts` budget (default 3)" → "(default 10)".
- "authors a **bounded** minimal fix in at most two attempts" → "authors a **bounded** minimal fix within the configured `finalize.repair_max_attempts` budget (default 6, the initial attempt included)".
- "a repair that cannot reach green in two attempts, is `halted`" → "a repair that cannot reach green within that budget, is `halted`".
- In the halt list, "a **red rebased suite the repair cannot green** in ≤2 attempts (`stuck`)" → "a **red rebased suite the repair cannot green** within the configured repair budget (`stuck`)".

`cursor-rules/dispatch/docket-integration-repair.md`: extend "The prompt must include the red-test output, the base it was rebased onto, and the feature worktree…" to also require "…and the repair-attempt budget (`finalize.repair_max_attempts` from the prepared context)"; change "writes a minimal fix in at most two attempts" to "writes a minimal fix within the dispatched repair-attempt budget".

- [ ] **Step 4: Rewrite the two docs pages**

`docs/guide/proving-the-build.md`: "bounded to at most two attempts" → "bounded to the configured `finalize.repair_max_attempts` budget (default 6)". `docs/reference/skills-and-agents.md`: "re-green the suite after finalize's rebase in at most two attempts" → "re-green the suite after finalize's rebase within the configured repair-attempt budget (default 6)".

- [ ] **Step 5: Verify no maintained stragglers, then commit**

Re-run the Step-1 inventory; assert the only remaining hits are the frozen surfaces named in Global Constraints (plus the unrelated gatedrive/concurrency files). Expect the `internal/assets` and `internal/harness` drift/golden tests to be red until Task 5.

```bash
git add agents/ cursor-rules/ skills/ docs/guide/ docs/reference/
git commit -m "docs(0419): repair contract bounded by the configured repair-attempt budget"
```

---

### Task 5: Regenerate embedded tree and harness goldens

**Files:**
- Regenerate: `internal/assets/embedded/tree/**` (agents, cursor-rules, skills, `.docket.example.yml` twins)
- Regenerate: `internal/harness/{claude,cursor,opencode}/testdata/golden/docket-integration-repair.md` (and any other goldens the renderer derives from edited sources; the codex package too if its tests demand it)

**Interfaces:**
- Consumes: Tasks 1, 2, 4's authored edits.
- Produces: generated twins byte-consistent with authored sources, proven by the packages' own drift guards.

- [ ] **Step 1: Regenerate the embedded tree**

Run: `go generate ./internal/assets`
Then: `go test ./internal/assets/ -count=1`
Expected: PASS. Inspect `git diff --stat internal/assets/embedded/tree/` — only files whose authored sources this change edited may differ (the two example-yml twins, the finalize skill pair, the repair agent, the cursor dispatch rule). Any other drift is a stop-and-investigate, not a commit.

- [ ] **Step 2: Regenerate harness goldens**

Run each harness package with its update flag, then verify clean:

```bash
go test ./internal/harness/claude/ -count=1 -update
go test ./internal/harness/cursor/ -count=1 -update
go test ./internal/harness/opencode/ -count=1 -update
go test ./internal/harness/... -count=1
```

Expected: final run PASS. Same diff discipline: only repair-agent-derived goldens change.

- [ ] **Step 3: Commit**

```bash
git add internal/assets/embedded/tree/ internal/harness/
git commit -m "chore(0419): regenerate embedded tree and harness goldens for the configurable repair budget"
```

---

### Task 6: Record the ADR replacing ADR-0010's fixed repair cap

**Files:**
- Create: `docs/adrs/<next-number>-<slug>.md` via the docket-adr flow (dispatch the registered `docket-adr` agent; it owns numbering, the index, and the metadata-branch write)

**Interfaces:**
- Consumes: the shipped design (Tasks 1–5).
- Produces: an additive Accepted ADR; change 0419's `adrs:` frontmatter gains the new id through the tracked metadata flow the build role owns (adr-update-delivery: the ADR ships with this change, never standalone).

- [ ] **Step 1: Dispatch docket-adr**

Record an additive decision titled approximately "Finalize repair attempts are configuration-bounded" covering: (a) `finalize.repair_max_attempts` (default 6, min 1, ADR-0019 `scopeAny` precedence) replaces the fixed ≤2 repair bound ADR-0010 specified — ADR-0010's accepted text is historical and is not rewritten; (b) the cap is enforced by the dispatched repair agent's contract via the dispatch payload, not by a durable Go reservation, deliberately unlike the resolver budget (change 0349) because repair attempts happen inside one dispatch rather than across many; (c) the initial attempt counts and exhaustion maps to the existing `stuck`/`halted` path; (d) the resolver built-in default rises 3 → 10 with overrides and receipt-snapshotted budgets preserved. Relate it to ADR-0010 and ADR-0019 (and the change-0349 resolver-reservation ADR the index names); supersede nothing.

- [ ] **Step 2: Verify and commit linkage**

Confirm the docket-adr run reports the new ADR id and index update, and that change 0419's `adrs:` list carries the new id via the tracked flow. Any feature-branch artifact the flow leaves is committed as `docs(0419): record configurable-repair-budget ADR`; metadata-branch writes belong to the docket-adr flow, not hand git.

---

### Task 7: Final verification sweep

**Files:** none new — whole-tree checks.

- [ ] **Step 1: Mutation-check the propagation asserts**

Temporarily comment out the `RepairMaxAttempts` assignment in `internal/app/repository_prepare.go`'s `PrepareFinalize` literal; run `go test ./internal/app/ -run TestRepositoryPrepare -count=1`; expected FAIL (proves Task 3's non-default assert bites). Restore the line exactly (undo the edit by hand — never `git checkout --`, which restores to HEAD and can eat uncommitted work), re-run, expected PASS.

- [ ] **Step 2: Whole-repo phrase audit**

```bash
out=$(grep -rn "at most two\|cannot reach green within two\|≤2 attempts" --include='*.md' --include='*.go' . | grep -v '^\./\.git' | grep -v 'docs/adrs/\|docs/results/\|docs/superpowers/\|internal/install/legacydata/\|internal/install/testdata/\|docs/changes/')
printf '%s\n' "$out"
```

Expected: only the unrelated `internal/gatedrive` / `internal/app/app_concurrency_race_integration_test.go` hits (gate-run attempt prose) remain. Anything else is an unshipped surface — fix it before the gate.

- [ ] **Step 3: Hand to the docket-build final gate**

The docket-build final suite gate runs the resolved `build.test_command` (`go run ./cmd/docket development test`) from source and owns pass/fail; read the budget report even on green. No separate commit unless the sweep fixed something (then `fix(0419): <what>`).
