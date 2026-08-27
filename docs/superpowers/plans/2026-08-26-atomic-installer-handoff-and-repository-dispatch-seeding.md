<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0351 — Complete change 0334: stop writing global instruction files and actually deploy the recursion guard](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-27-0351-complete-0334-retire-global-instruction-writes-and-deploy-recursion-guard.md)**
<!-- docket:backlink:end -->
# Atomic Installer Handoff and Repository Dispatch Seeding — Implementation Plan (change 0351)

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan
> task-by-task (routed multi-profile, no per-task review, single full-suite gate at the end).
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop planning Docket dispatch instructions into user-global instruction files (retiring
provably-owned leftovers safely), make one `docket development install` invocation render and
install through the binary it just built, and seed parent-facing repository dispatch surfaces only
where a repository explicitly opts in via `agent_harnesses` — all as one preflighted, journaled,
all-or-nothing operation.

**Architecture:** Four seams move. (1) `internal/document` gains a `RemoveBlock` patch op and
`internal/install`'s transaction gains managed-block removal as a first-class journaled step, so
retiring a global `docket:dispatch` block preserves every byte outside it. (2) The four
`internal/harness/*` adapters stop planning their user-global dispatch target; a new retirement
planner in `internal/install` probes the four historical global destinations and removes only what
a prior installation record or the frozen legacy reproducer proves is Docket's. (3) A new
`internal/reposeed` package plans parent-facing repository surfaces (CLAUDE.md block or safe link,
shared AGENTS.md block, `.cursor/rules/docket-dispatch.mdc`) from an explicit repo-layer
`agent_harnesses` declaration, with per-working-tree ownership at `<git-dir>/docket/install.json`;
its targets join the machine transaction, and the transaction learns to publish both ownership
records under the journal. (4) `DevelopmentInstall` splits into a parent phase (validate, build a
candidate into a private temp dir, hand off via an explicit argv vector) and a candidate phase (a
hidden internal continuation that revalidates everything, plans with its own renderer, installs its
own bytes as the binary target, and applies the whole plan).

**Tech Stack:** Go (`internal/install`, `internal/harness/*`, `internal/reposeed` (new),
`internal/config`, `internal/gitcli`, `internal/document`, `internal/app`, `internal/cli`), Go
golden/adapter tests, hermetic shell tests under `tests/`, `scripts/run-tests.sh` as the suite
gate.

**Spec:** `docs/superpowers/specs/2026-08-26-atomic-installer-handoff-and-repository-dispatch-seeding-design.md`
(on the `docket` metadata branch). Read it with the change file's reconcile log and its
"Live repro & priority rationale" section — the recursion regression is live, and the change file
carries one open question this plan answers in Task 11 (docs).

## Global Constraints

- **All-or-nothing.** Any machine or repository conflict refuses the entire operation before the
  first destination mutation. There is no `--force`. Every refusal names the exact path, why
  ownership cannot be proven, and a manual remove-or-repair-and-rerun remedy.
- **Ownership proof is bytes, not markers.** A target may be rewritten or retired only when its
  current bytes (managed block: normalized interior) match the prior installation record or the
  frozen legacy reproducer; a recordless target may be adopted only when byte-identical to the
  desired render or the legacy reproducer. Marker presence or a Docket-looking filename proves
  nothing (spec "Alternatives considered").
- **Three probe outcomes.** Everywhere retirement or cleanup asks "is this still there / still
  ours?": present-and-proven, cleanly absent, and unknown are three answers. Unknown (a read
  error) refuses the run; it never shares a branch with "absent" (learnings:
  `probe-error-is-not-clean-absence`).
- **Repository authority is the explicit key, keyed on provenance.** Repository files are touched
  only when `agent_harnesses` was *supplied by* the repository or repository-local layer
  (`Value.Explicit && Provenance.Layer ∈ {LayerRepository, LayerRepositoryLocal}`). A global-layer
  declaration, an `agents:` table, or merely standing in a repository grants nothing (learnings:
  `guard-keyed-on-presence-not-provenance`, `opt-in-signal-not-file-presence`,
  `absent-target-certifies-permission`).
- **The parent never mutates.** In a development install the currently running binary validates
  and builds only. It must not acquire the lock, recover a journal, call any planner, render a
  target, or write any destination. The candidate owns all of that.
- **Stable machine reasons only.** Every new failure mode gets a `Reason*` constant in
  `internal/install` and a row in `internal/app.classifyInstall`. Nothing above the service keys
  on error text.
- **No wording changes.** `harness.DispatchInterior`, `harness.RecursionGuard`, and the run-gate
  payload are consumed verbatim — this change moves where they are written, never what they say.
- **Repository files stay unstaged.** The reconciler edits the selected working tree and never
  runs `git add` or `git commit` there.
- **Mutation-test every new guard** with `go test -count=1` (learnings:
  `cached-runner-serves-a-mutated-tree`; a bare `go test` verdict is not evidence).
- **Suite gate:** run the whole suite via `finalize.test_command` (`scripts/run-tests.sh`), never
  only the tests this plan names.
- **Fresh-process verification is a human checkpoint.** Harnesses load wrappers and parent
  instructions at process start; the session that installs cannot observe the result. Name it in
  the results file's human-verify section; report any harness not live-exercised as unverified
  (learnings: `generated-artifact-loaded-at-process-start`,
  `external-truth-needs-a-human-checkpoint`).

---

### Task 1: `document.RemoveBlock` patch operation

**Files:**
- Modify: `internal/document/patch.go`
- Test: `internal/document/patch_test.go` (extend the existing table-driven patch tests)

**Interfaces:**
- Produces: `func (p *PatchSet) RemoveBlock(name string)` — queues removal of the named managed
  block (both markers and the interior), preserving every byte outside the block, including the
  blank separation the block's own insertion added is NOT chased: exactly the marker-to-marker
  line range is removed. Resolution fails (via the existing `Document.Apply` error path) when the
  named block is absent, mirroring `resolveReplaceBlock`'s absent-block error.

- [ ] **Step 1: Write failing tests.** In `internal/document/patch_test.go` add cases:
  - `remove block leaves surrounding prose byte-identical`: parse a document with prose, a
    `dispatch` block, more prose; `p.RemoveBlock("dispatch")`; assert the applied output equals
    the original with exactly the block's marker-to-marker lines deleted.
  - `remove absent block errors`: `RemoveBlock("nope")` on a document without it; `Apply` returns
    an error naming the block.
  - `remove one of two blocks keeps the other`: two named blocks; remove one; the other block's
    bytes are untouched.
  - Malformed-marker documents already fail at `document.Parse` (dangling/out-of-order/nested);
    add one assert that such a document never reaches `RemoveBlock` (Parse errors first) so the
    removal path cannot consume to EOF.
- [ ] **Step 2: Run** `go test -count=1 ./internal/document/` — expect the new cases to fail
  (`RemoveBlock` undefined).
- [ ] **Step 3: Implement** `RemoveBlock` in `patch.go`: a new edit kind resolved analogously to
  `resolveReplaceBlock` (look up the block via `d.Block(name)`, error when absent), whose
  rendered replacement for the block's line range is the empty string.
- [ ] **Step 4: Run** `go test -count=1 ./internal/document/` — expect PASS.
- [ ] **Step 5: Mutation-probe:** temporarily make the removal keep the closing marker; the
  byte-identical assert must redden. Restore.
- [ ] **Step 6: Commit** `feat(document): RemoveBlock patch op for managed-block retirement`

---

### Task 2: Managed-block removal as a first-class transaction step

**Files:**
- Modify: `internal/install/txn.go` (`removalTarget`, `applyStep`, `restoreStep` if needed)
- Test: `internal/install/txn_test.go`

**Interfaces:**
- Consumes: Task 1's `PatchSet.RemoveBlock`.
- Produces: `BeginTxnWithRemovals` now accepts removal records with
  `Kind == KindManagedBlock` (`BlockName` required). Applying such a step rewrites the file with
  only that block removed, through the existing staging+rename path so the journal pre-image
  covers rollback. `KindFile`/`KindSymlink` removal behavior is unchanged.

- [ ] **Step 1: Write failing tests** in `txn_test.go`:
  - `managed-block removal strips only the block`: seed a file with user prose around a
    `dispatch` block; run a transaction whose removals list carries a
    `TargetRecord{Kind: KindManagedBlock, BlockName: "dispatch", Path: ...}`; assert the file
    afterward equals prose-only bytes and the file still exists.
  - `managed-block removal rolls back`: inject an FS failure (the existing failing-`FSOps` test
    seam) after the removal step; assert byte-for-byte restoration of the original file.
  - `removal of a malformed-marker file refuses`: seed dangling markers; the transaction must
    refuse before mutating (the `document.Parse` error surfaces from planning/apply) and the file
    is untouched.
- [ ] **Step 2: Run** `go test -count=1 ./internal/install/ -run TestTxn` — expect FAIL: current
  `removalTarget` returns "which a transaction never deletes" for `KindManagedBlock`.
- [ ] **Step 3: Implement.** In `removalTarget`, accept `KindManagedBlock` (require
  `rec.BlockName != ""`, else `ErrInvalidTarget`). In `applyStep`'s remove branch, branch on
  `step.Kind`: for `KindManagedBlock`, read the file, `document.Parse`, `RemoveBlock`, and write
  the result via `writeThroughStaging` (preserving the file's pre-image mode); a file that is
  absent at apply time fails pre-image verification exactly as updates do today.
- [ ] **Step 4: Run** `go test -count=1 ./internal/install/` — expect PASS.
- [ ] **Step 5: Commit** `feat(install): managed-block removal as a journaled transaction step`

---

### Task 3: Adapters stop planning global dispatch; retirement planner removes proven leftovers

**Files:**
- Modify: `internal/harness/claude/claude.go`, `internal/harness/codex/codex.go`,
  `internal/harness/opencode/opencode.go`, `internal/harness/cursor/cursor.go` — delete the
  dispatch target from each `Plan`; export the historical destination instead.
- Create: `internal/install/retire.go`
- Modify: `internal/install/service.go` (`applyPlan`, `Check`, `desiredState`)
- Test: `internal/install/retire_test.go`, each adapter's `*_test.go` and `testdata` goldens,
  `internal/install/service_test.go`

**Interfaces:**
- Produces (per adapter, e.g. claude):
  `func GlobalDispatchTarget(r install.UserRoots) install.Target` returning the target the adapter
  *used to* plan (claude/codex/opencode: `KindManagedBlock`, `BlockName: "dispatch"`, path under
  the harness root; cursor: `KindFile` at `~/.cursor/rules/docket-dispatch.mdc`), `Content` left
  nil — retirement needs the location and identity fields only.
- Produces: `func PlanGlobalRetirements(roots UserRoots, historical []Target, prior *State, legacy LegacyReproducer) (removals []TargetRecord, conflicts []Inspection, err error)`
  in `retire.go`. For each historical destination:
  - stat/read errors → `err` (refuse the run; never the absent branch);
  - cleanly absent, or a prior record shows it was already retired → nothing;
  - present managed block whose normalized interior digest matches the prior record's `SHA256`
    for that path, or matches the legacy reproducer (`provenByLegacyInterior`) → a removal
    record;
  - present cursor rule whose whole-file bytes match the prior record or legacy reproducer
    (`provenByLegacy`) → a removal record;
  - present but edited interior, malformed markers, foreign kind (dir/symlink where a file is
    expected), or an unresolvable symlink → a conflict `Inspection` carrying the existing
    remedy-composing `ConflictDetail` machinery.
- Modifies `applyPlan`: call `PlanGlobalRetirements`, append its conflicts to the plan's
  conflicts (so one refusal collects all), append its removals to the transaction's removals,
  and DELETE the "managed block retired from the plan; the file is the user's and is left in
  place" retain branch — a `KindManagedBlock` prune now becomes a removal gated by the same
  proof, and an unprovable one is a refusal, not a silent keep. `desiredState` drops
  role-`dispatch` records that were removed, so no stale retained record remains.
- Modifies `Check`: report a still-present proven global dispatch artifact as drift
  (`OpDrift`, detail "global dispatch surface awaiting retirement; run docket install") and an
  unproven one as the conflict it is; drop the `continue // retained by design` skip.

- [ ] **Step 1: Write failing adapter tests.** In each adapter test file, flip the existing
  dispatch-target asserts: `Plan` output must contain NO target with `Role == "dispatch"`, and
  `GlobalDispatchTarget` must return the exact historical path/kind/blockname. Regenerate/adjust
  goldens under each adapter's `testdata` to drop the dispatch surface. Keep
  `TestCrossHarness*` (`internal/harness/cross_harness_test.go`) consistent — the planned target
  inventory must contain no global parent surface (this assert IS the spec's harness-contract
  gate; anchor it on `Plan` output, not on a golden listing).
- [ ] **Step 2: Write failing retirement tests** in `retire_test.go`, covering the spec's global
  retirement matrix per managed-block harness: unchanged prior-record block (removed), exact
  frozen legacy block with surrounding user prose (removed, prose byte-identical), edited
  interior (conflict; file untouched), dangling/out-of-order markers (conflict), foreign file
  kind (conflict), escaping symlink (conflict), probe error via unreadable parent (run refuses,
  file untouched). For cursor: exact prior and legacy whole-file bytes (removed), edited bytes /
  foreign kind / symlink (conflict). And one `applyPlan`-level test: a single conflicted global
  target blocks binary, wrapper, skill, and state changes alike.
- [ ] **Step 3: Run** `go test -count=1 ./internal/harness/... ./internal/install/` — expect FAIL.
- [ ] **Step 4: Implement** the adapter deletions + `GlobalDispatchTarget` exports and
  `retire.go` as specified, wiring into `applyPlan` (both Install and DevelopmentInstall flow
  through it) and `Check`.
- [ ] **Step 5: Run** `go test -count=1 ./internal/harness/... ./internal/install/ ./internal/app/`
  — expect PASS (fix `internal/app` fallout: `legacyNotAdoptedNote` and result rendering are
  unchanged, but service tests asserting the old retain note must now assert removal).
- [ ] **Step 6: Mutation-probe:** disable the legacy-reproducer proof branch — the
  exact-legacy-block test must redden; disable the conflict branch for edited interiors — the
  edited-interior test must observe a partial mutation attempt and redden.
- [ ] **Step 7: Commit** `feat(install): retire global dispatch surfaces with ownership proof; adapters stop planning them`

---

### Task 4: `agent_harnesses` becomes a typed, provenance-carrying repository input

**Files:**
- Modify: `internal/config/schema.go` (the `agent_harnesses` row), `internal/config/config.go`
  (`Effective`), `internal/config/resolve.go`
- Test: `internal/config/resolve_test.go`, `internal/config/schema_test.go`

**Interfaces:**
- Produces: `Effective.AgentHarnesses Value[[]string]` — resolved with the existing
  `mergeListReplace` precedence (repository-local replaces repository), validated as: allowed
  tokens exactly `claude|codex|cursor|opencode`, duplicate-free, each violation a
  `CodeInvalidValue` diagnostic that invalidates the snapshot. The leaf's classification moves
  from `dispInert` to supported.
- Contract for consumers (Task 8 keys on this): write authority exists iff
  `AgentHarnesses.Explicit && (AgentHarnesses.Provenance.Layer == LayerRepository || AgentHarnesses.Provenance.Layer == LayerRepositoryLocal)`.
  A global-layer declaration may still resolve (the key is `scope: scopeAny` historically and
  global files in the wild carry it) but its provenance is `LayerGlobal`, which the installer
  never honors. `Explicit == true` with an empty list is the deliberate retire-everything state;
  `Explicit == false` is the touch-nothing state.

- [ ] **Step 1: Write failing tests** in `resolve_test.go`:
  - repo layer `agent_harnesses: [claude, codex]` → `Explicit: true`, value `[claude codex]`,
    `Provenance.Layer == LayerRepository`.
  - repo-local `agent_harnesses: []` over repo non-empty → `Explicit: true`, empty value,
    `Provenance.Layer == LayerRepositoryLocal` (replace, not append).
  - global-only declaration → `Provenance.Layer == LayerGlobal` (the consumer contract's
    never-honored case; assert the provenance, since that is the guard's key).
  - key absent everywhere → `Explicit: false`.
  - `[claude, claude]` and `[emacs]` → `CodeInvalidValue` diagnostics, snapshot invalid.
- [ ] **Step 2: Run** `go test -count=1 ./internal/config/` — expect FAIL.
- [ ] **Step 3: Implement:** add the `Effective` field, wire the schema row's `validate:
  listLeaf(listOpts{dupFree: true, allowed: ...})` (reuse the existing allowed-token option the
  dummy-surface leaf uses), and land the resolved value + provenance in `Effective` the way
  `BoardSurfaces` does. Update `schema_test.go`'s leaf-inventory expectations.
- [ ] **Step 4: Run** `go test -count=1 ./internal/config/` — expect PASS.
- [ ] **Step 5: Commit** `feat(config): agent_harnesses as a typed repo-surface list with provenance`

---### Task 5: Worktree discovery for installation commands

**Files:**
- Create: `internal/gitcli/worktreeid.go`
- Test: `internal/gitcli/worktreeid_test.go`

**Interfaces:**
- Produces: `func (c *Client) DiscoverWorktree(ctx context.Context, opts DiscoverOptions) (WorktreeIdentity, error)` with
  `type WorktreeIdentity struct { Root string; GitDir string }` — the canonical
  (Abs + every-symlink-hop, matching `Discover`'s discipline; learnings:
  `canonicalise-every-symlink-hop`) toplevel of the working tree CONTAINING the invocation path,
  plus that worktree's own absolute git dir (`git rev-parse --show-toplevel --absolute-git-dir`
  in one process). Unlike `Discover`, a linked worktree resolves to ITSELF, because per-worktree
  ownership isolation is the point. Bare repositories and non-repositories fail with the
  existing `KindInvalidRepository` failure kind.

- [ ] **Step 1: Write failing tests** in `worktreeid_test.go` using the package's existing
  fixture helpers: (a) from the repo root and from a nested directory → same identity; (b) from
  a linked worktree → that worktree's root and its `.git/worktrees/<name>` git dir, NOT the
  primary's; (c) a non-repo temp dir → `KindInvalidRepository`; (d) a path reached through a
  symlink resolves to the canonical root (build the fixture under `t.TempDir()`, which on macOS
  exercises `/tmp → /private/tmp`).
- [ ] **Step 2: Run** `go test -count=1 ./internal/gitcli/ -run TestDiscoverWorktree` — FAIL.
- [ ] **Step 3: Implement** using the package's `run`/failure plumbing, canonicalizing both
  outputs with `filepath.EvalSymlinks`.
- [ ] **Step 4: Run** `go test -count=1 ./internal/gitcli/` — PASS.
- [ ] **Step 5: Commit** `feat(gitcli): DiscoverWorktree resolves the containing worktree and its git dir`

---

### Task 6: `internal/reposeed` — repository surface planner

**Files:**
- Create: `internal/reposeed/plan.go`, `internal/reposeed/plan_test.go`

**Interfaces:**
- Consumes: `harness.DispatchInterior`/`harness.RunGate` (the interior bytes), the cursor
  adapter's dispatch rule content (export the renderer the global rule used as
  `cursor.DispatchRuleContent(runGate []byte) []byte` if `Plan`'s deletion in Task 3 orphaned
  it), `install.Target`.
- Produces:
  ```go
  type PlanInput struct {
      WorktreeRoot string   // canonical absolute
      Harnesses    []string // the repo's explicit opt-ins, already validated tokens
      RunGate      []byte
  }
  // Plan renders the parent-facing repository targets for the opted-in
  // harnesses. It is pure and emits NO agent definitions and NO skills.
  func Plan(in PlanInput) ([]install.Target, map[string][]string, error)
  ```
  The second return maps cleaned target path → harness owners (shared surfaces carry several).
  Surface table (paths joined under `WorktreeRoot`):
  - `claude` → managed block `dispatch` in `CLAUDE.md`, EXCEPT when a shared `AGENTS.md` surface
    is in this plan (codex or opencode also opted in) AND `CLAUDE.md` is absent or is already a
    relative symlink to `AGENTS.md`: then plan `CLAUDE.md` as `KindSymlink` with `LinkTarget`
    `<root>/AGENTS.md`. An existing regular `CLAUDE.md` always keeps its content and gets its
    own managed block. (Plan is pure: it receives the CLAUDE.md pre-state as an input field
    `ClaudeMDState` — `absent | regularFile | linkToAgents | other` — computed by the caller;
    `other` is planned as a managed block so inspection reports the conflict with a remedy.)
  - `codex`, `opencode` → ONE shared managed block `dispatch` in `AGENTS.md` (one target, both
    owners).
  - `cursor` → `KindFile` at `.cursor/rules/docket-dispatch.mdc`.
  All content is `DispatchInterior(runGate)` for blocks and the cursor rule renderer for the
  rule. Containment: every planned path must be inside `WorktreeRoot` after `filepath.Clean`;
  emit an error otherwise (symlink escape is caught at inspection time by the existing
  canonical-path machinery, and Task 8 adds the pre-transaction refusal test).

- [ ] **Step 1: Write failing tests** covering: each harness alone; codex+opencode share one
  AGENTS.md target with both owners; claude+codex with `ClaudeMDState: absent` plans the
  symlink; claude+codex with `regularFile` plans a managed block and leaves AGENTS.md shared by
  codex only; claude alone never plans AGENTS.md; empty `Harnesses` plans nothing; unknown
  token errors (defense in depth — config already validated); no target's `Role` is ever
  `skill` or `agent`; byte-identical `DispatchInterior` across the claude block, the shared
  block, and the goldens from Task 3's adapters (the wording constraint).
- [ ] **Step 2: Run** `go test -count=1 ./internal/reposeed/` — FAIL (package new).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** — PASS.
- [ ] **Step 5: Commit** `feat(reposeed): parent-facing repository surface planner`

---

### Task 7: Per-worktree repository ownership record

**Files:**
- Create: `internal/reposeed/record.go`, `internal/reposeed/record_test.go`

**Interfaces:**
- Produces:
  ```go
  const RecordFormatVersion = 1
  type SurfaceRecord struct {
      Path       string             `json:"path"` // worktree-relative, slash-separated
      Kind       install.TargetKind `json:"kind"`
      BlockName  string             `json:"block_name,omitempty"`
      LinkTarget string             `json:"link_target,omitempty"` // relative, symlink kind
      SHA256     string             `json:"sha256,omitempty"`      // file: whole file; block: normalized interior
      Harnesses  []string           `json:"harnesses"`             // sorted owners
  }
  type Record struct {
      FormatVersion int             `json:"format_version"`
      Surfaces      []SurfaceRecord `json:"surfaces"` // sorted by Path
  }
  func RecordPath(gitDir string) string                  // <gitDir>/docket/install.json
  func LoadRecord(path string) (*Record, error)          // absent → (nil, nil); corrupt → error
  func (r *Record) ToState(worktreeRoot string) *install.State // absolute-path TargetRecords so
                                                        // install.InspectTarget can consume it
  func DesiredRecord(targets []install.Target, owners map[string][]string, worktreeRoot string) (*Record, error)
  func EncodeRecord(r *Record) ([]byte, error)           // canonical bytes for journaled publish
  ```
  Semantics: paths are stored worktree-relative so a moved worktree keeps its history; `ToState`
  joins them under the current root. Ownership rules are the machine installer's, supplied by
  reusing `install.InspectTarget` against `ToState`'s synthetic state (plus the legacy
  reproducer for a repo-committed legacy block, passed through unchanged). Scoped runs (Task 8)
  update the named harnesses' surfaces and carry every other surface record forward; a shared
  surface losing one owner but keeping another stays planned and keeps the remaining owner.

- [ ] **Step 1: Write failing tests:** round-trip encode/load; absent file → `(nil, nil)`;
  corrupt JSON → error (never "not installed" adoption; mirror `install.LoadState`'s comment);
  `ToState` produces absolute cleaned paths; `DesiredRecord` sorts surfaces and owners; a
  worktree-relative path escaping the root (`../x`) is refused by `DesiredRecord`.
- [ ] **Step 2: Run** `go test -count=1 ./internal/reposeed/` — FAIL.
- [ ] **Step 3–4: Implement; run to PASS.**
- [ ] **Step 5: Commit** `feat(reposeed): per-worktree ownership record under the git dir`

---

### Task 8: One transaction across machine, retirement, repository, and both state documents

**Files:**
- Modify: `internal/install/txn.go` (`Txn.Commit` → multi-document publish), `internal/install/service.go`
  (`applyPlan` signature grows a repository phase), `internal/install/state.go` (no shape change;
  `WriteStateAtomic` reused)
- Create: `internal/install/repophase.go` (the glue type so `internal/install` stays free of
  `reposeed`/`gitcli` imports)
- Test: `internal/install/txn_test.go`, `internal/install/service_test.go`

**Interfaces:**
- Produces:
  ```go
  // StateDoc is one ownership document the transaction publishes at commit.
  type StateDoc struct{ Path string; Bytes []byte }
  func (t *Txn) CommitDocs(docs []StateDoc) error
  ```
  `BeginTxnWithRemovals` (or a new `BeginTxnFull`) journals each doc's pre-image (absent or
  prior bytes) alongside the target steps; `Rollback`/`Recover` restore them. The old
  `Commit(statePath, state)` becomes a thin wrapper encoding via `encodeState` and calling
  `CommitDocs`.
  ```go
  // RepoPhase is the repository half of one installation plan, assembled by
  // internal/app (which may import reposeed/gitcli) and consumed here.
  type RepoPhase struct {
      Authorized  bool          // explicit repo-layer opt-in existed
      Targets     []Target      // absolute; empty with Authorized=true means retire-only
      Owners      map[string][]string
      PriorState  *State        // reposeed.Record.ToState(...); nil when no record
      Removals    []TargetRecord // retire-everything / dropped-surface removals, proof-gated
      RecordPath  string        // <git-dir>/docket/install.json
      RecordBytes []byte        // desired record; nil when Authorized=false
  }
  ```
  `applyPlan(o Options, p plannedInstallation, repo *RepoPhase, out Outcome) Outcome`:
  inspections run over machine targets against the machine state AND repo targets against
  `repo.PriorState`; ALL conflicts (machine, retirement, repository) collect into one refusal;
  the single transaction carries machine steps + retirement removals + repo steps + repo
  removals, and `CommitDocs` publishes the machine state and (when authorized) the repo record
  together. `repo == nil` or `Authorized == false` must leave any prior repo record untouched
  and add the outcome action `Action{Op: OpKeep, Path: "<worktree or (none)>", Detail:
  "repository reconciliation not authorized: no explicit agent_harnesses declaration"}` so the
  no-op is named, not implied (spec "Diagnostics and observable behavior").

- [ ] **Step 1: Write failing tests:**
  - `repo conflict blocks machine work`: one unowned repo target → binary/wrapper/skill/global
    destinations all byte-identical afterward. Then MUTATION-TEST the all-or-nothing guard as
    the spec demands: disable the repository-conflict preflight (skip repo inspections) in a
    temporary mutation and assert this test observes a partial machine change — it must redden;
    restore.
  - `commit publishes both documents; rollback restores both`: inject a rename failure on the
    second doc; both the machine state and the repo record read back as their pre-images.
  - `interrupt at every durable journal point recovers`: extend the existing txn interruption
    test loop to include the two doc steps; the next `recoverPending` restores both sides to
    the same side of the operation.
  - `unauthorized run touches no repo file and keeps the prior record`, with the named no-op
    action asserted.
  - `explicit empty list retires unchanged owned surfaces and publishes an empty record;
    an edited owned surface blocks everything`.
  - `two records, two worktrees`: two temp worktree roots with separate record paths; a run
    against one leaves the other's surfaces and record byte-identical (the isolation matrix).
- [ ] **Step 2: Run** `go test -count=1 ./internal/install/` — FAIL.
- [ ] **Step 3: Implement** as specified. Keep `Check` repo-free in this task (machine `check`
  semantics unchanged; the spec's check surface for repositories is the named no-op).
- [ ] **Step 4: Run** `go test -count=1 ./internal/install/ ./internal/app/` — PASS.
- [ ] **Step 5: Commit** `feat(install): single journaled transaction across machine, retirement, repository, and both ownership records`

---

### Task 9: Fresh-binary development-install handoff

**Files:**
- Modify: `internal/install/devmode.go`, `internal/install/service.go` (reason constants)
- Modify: `internal/cli/root.go`, `internal/cli/install.go` (hidden continuation command +
  `assetIndependent` entries)
- Test: `internal/install/devmode_test.go`, `internal/cli/root_test.go` (or the
  `TestAssetIndependentSetExact` home), `cmd/docket` end-to-end test file

**Interfaces:**
- Produces in `internal/install`:
  ```go
  const ReasonHandoffFailed = "handoff-failed"
  // HandoffRunner executes the candidate binary with an explicit argv vector,
  // relaying stdio, and returns its exit code. Never a shell string.
  type HandoffRunner func(binary string, argv []string, env []string) (exitCode int, err error)
  type DevOptions struct {
      ...existing fields...
      Handoff HandoffRunner // required, like GoRunner
      // Continuation marks this process AS the candidate: plan and apply, build
      // nothing, hand off to nothing.
      Continuation bool
  }
  ```
  `DevelopmentInstall` splits:
  - **Parent path** (`Continuation == false`): `requireOptions`, config preflight,
    `validateSourceRoot`, committed-manifest protocol check, drift check, `buildBinary` into the
    private staging dir (keep the staged file; move `defer os.RemoveAll` after the handoff
    returns), then invoke `o.Handoff(stagedBinary, argv, os.Environ())` where argv is exactly
    `["development", "install", "--internal-continuation", "--source", source, "--bin-dir", binDir]`
    plus one `--harness` per explicit selection and `--repo-dir` when the public command got
    one. The child's outcome IS the result: exit 0 → the parent parses nothing and returns the
    child's already-printed document by returning an `Outcome` marked as relayed (see CLI note
    below); non-zero → `fail(out, ReasonHandoffFailed, ...)` carrying the exit code. The parent
    acquires NO lock, calls NO planner, and never touches `applyPlan` — delete those calls from
    the parent path.
  - **Candidate path** (`Continuation == true`): repeat ALL mutable-world validation
    (`requireOptions`, preflight, `validateSourceRoot`, manifest+protocol, `assets.Generate` +
    `DiffTree` + digest — a source changed between build and handoff fails here), lock,
    `recoverPending`, plan (selection per Task 10), and the binary target's `Content` is the
    candidate's OWN bytes: `os.Executable()` → `filepath.EvalSymlinks` → `os.ReadFile`. Then
    `applyPlan` with the repo phase. The candidate never builds and never hands off — the
    recursion stop is structural, not a flag check inside a shared path.
  - Release `Install` is untouched: it already IS the candidate version (spec: no build handoff).
- Produces in `internal/cli`: the continuation is the same `development install` command with a
  hidden flag `--internal-continuation` (`Hidden: true`, help text "internal; not a supported
  installation mode"), so `assetIndependent` needs no new key. The CLI relay rule: when the
  parent's handoff succeeded, the child has already printed the result document; the parent
  prints nothing and exits with the child's code (implement by having the parent command return
  a sentinel the presenter skips — mirror how the presenter handles an already-rendered result,
  and pin it with a test that the document appears exactly once).

- [ ] **Step 1: Write failing seam tests** in `devmode_test.go`:
  - `parent never plans or mutates`: `DevOptions` whose `Planners` all have `Plan` funcs that
    `t.Fatal("parent invoked a planner")`, an `FSOps` that fails every write, and a stub
    `Handoff` recording its argv; a parent run must succeed (relay) without tripping either,
    and the recorded argv must be the exact vector above. This is the spec's
    remove-the-handoff-and-it-reddens test: temporarily re-adding a parent-side `applyPlan`
    call must fail it.
  - `parent build failure is a no-op`: failing `GoRunner`; no lock file, no journal, no state.
  - `handoff exit code propagates`: stub returns 3 → `ReasonHandoffFailed`, `Err` names 3.
  - `candidate installs its own bytes`: run the candidate path with `Continuation: true` in a
    temp home against a fixture checkout (reuse the package's existing devmode fixtures);
    assert the installed `bin/docket` bytes equal the test binary's own `os.Executable`
    contents and the state records mode `0o755` (learnings:
    `promised-file-mode-needs-explicit-chmod` — the chmod already exists; assert through it).
  - `candidate refuses a drifted source`: mutate the fixture checkout between "build" and the
    candidate run → `ReasonSourceAssetsDrifted`, nothing written.
- [ ] **Step 2: Run** `go test -count=1 ./internal/install/ -run TestDev` — FAIL.
- [ ] **Step 3: Implement** the split; add `DefaultHandoffRunner` (exec.Command, `Stdout`/
  `Stderr` passthrough, `ExitError` → code) beside `DefaultGoRunner`.
- [ ] **Step 4: Write the fresh-render regression** at the `cmd/docket` level (the spec's
  acceptance test): using the existing TestMain-built binary as "the source tree's renderer",
  fabricate an "old installed binary" as a small stub executable (write a Go file to a temp
  dir, `go build` it in the test — the stub prints a canned document omitting a witness line
  and exits 0) at the bin destination; run `docket development install --source <fixture>` with
  `HOME`/XDG pinned to a temp root; assert the resulting installed binary and wrappers carry
  the new witness (the recursion guard line from `harness.RecursionGuard` serves as the
  witness — it exists in source goldens and is exactly what the live defect dropped). Keep the
  test budget-conscious: one build, one run.
- [ ] **Step 5: Run** `go test -count=1 ./cmd/... ./internal/...` — PASS.
- [ ] **Step 6: Commit** `feat(install): fresh-binary candidate handoff owns all development-install mutation`

---

### Task 10: CLI and app wiring — `--repo-dir`, repository selection, scope reporting

**Files:**
- Modify: `internal/cli/install.go` (`installOptions` grows the repo resolution),
  `internal/cli/root.go` (flags), `internal/app/install.go` (assemble `RepoPhase`, result
  fields), `internal/app` classifier
- Create: `internal/app/repophase.go` (+ `repophase_test.go`)
- Test: `internal/cli/root_test.go`, `internal/app/install_test.go`

**Interfaces:**
- Flags: `--repo-dir <path>` on `docket install` AND `docket development install` ("repository
  whose parent-facing dispatch surfaces are reconciled; default: the Git worktree containing
  the current directory; outside Git, machine-only").
- Produces `internal/app.ResolveRepoPhase`:
  ```go
  func ResolveRepoPhase(ctx context.Context, git *gitcli.Client, repoDir string, runGate []byte, legacy install.LegacyReproducer) (*install.RepoPhase, string, error)
  ```
  Behavior: explicit `repoDir` that is absent/not a worktree → error mapped to
  `install.ReasonInvalidOptions`-class refusal with a distinct new reason
  `ReasonInvalidRepoDir = "invalid-repo-dir"` (add to `internal/install` constants and the
  classifier's invalid-input row); empty `repoDir` + cwd outside Git → `(nil, "", nil)`
  (machine-only; the outcome's not-authorized action still prints); otherwise
  `DiscoverWorktree`, `config.LoadFilesystemSources({RepoDir: root})`, `config.Resolve`, read
  `Effective.AgentHarnesses`, apply the provenance guard from Task 4, and when authorized build
  the phase: compute `ClaudeMDState` (Lstat CLAUDE.md; readlink; classify), `reposeed.Plan`,
  `reposeed.LoadRecord`, `DesiredRecord`, proof-gated removals for dropped/retired surfaces
  (shared AGENTS.md is retired only when NO remaining owner requires it). The second return is
  the worktree root for reporting.
- Selection union: in `installOptions`/command bodies, when no `--harness` was given, the
  machine harness set becomes detection ∪ repo opt-ins (pass the opt-ins into
  `selectPlanners` via `Options.Harnesses` union logic in `internal/app` — implement as: run
  detection as today, then add planners for opted-in names not already selected; an unknown
  token never reaches here, config validated it). With `--harness`, machine set = the flags,
  and the repo phase narrows to `flags ∩ opt-ins` while `RepoPhase.PriorState`
  carries unrelated records forward (Task 7 semantics).
- Reporting: `InstallResult` gains `RepoHarnesses []string` and `RepoDir string` (JSON
  `repo_harnesses`, `repo_dir`; `HumanText` prints `repository: <dir>` and
  `repository harnesses: ...` when set) so a scoped run is visible rather than inferred.
- `development install`'s continuation passes `--repo-dir` through verbatim (Task 9's argv), so
  parent and candidate resolve the same repository; the candidate re-runs `ResolveRepoPhase`
  itself (revalidation).

- [ ] **Step 1: Write failing tests:** `repophase_test.go` covering the repository matrix rows
  that live at this layer — discovery from root and nested dir; explicit `--repo-dir`; invalid
  explicit → `ReasonInvalidRepoDir`; outside Git → nil phase; absent key → nil-authorized
  phase; global-layer declaration → NOT authorized (provenance guard — mutation-test by
  flipping the guard to `Explicit` alone and watching this redden); `agents:` table alone →
  not authorized; scoped `--harness codex` against opt-ins `[claude codex]` → repo targets
  only for codex, claude's record carried; `HumanText`/JSON expose the new fields.
- [ ] **Step 2: Run** `go test -count=1 ./internal/app/ ./internal/cli/` — FAIL.
- [ ] **Step 3: Implement**; remember `installOptions`' existing comment (repo config must not
  steer HOME writes) still holds: the repo snapshot feeds ONLY the repo phase and selection
  union, never `Planners`' agent table.
- [ ] **Step 4: Run** `go test -count=1 ./internal/app/ ./internal/cli/ ./internal/install/` — PASS.
- [ ] **Step 5: Commit** `feat(cli,app): --repo-dir, explicit repository opt-in, and scope-visible install reporting`

---

### Task 11: Documentation

**Files:**
- Modify: `README.md` (installation + harness setup sections),
  `internal/assets/embedded/tree/.docket.example.yml` → edit the AUTHORED source instead
  (`.docket.example.yml` at the repo root if that is the authored copy — verify with
  `grep -rn "agent_harnesses" --include=.docket.example.yml .` and regenerate via
  `go run ./cmd/genassets`), `docs/codex/`, `docs/cursor/`, `docs/opencode/` setup docs, and
  the docket-convention skill's harness/agent-layer reference
  (`skills/docket-convention/references/agent-layer.md` authored source) where it describes
  dispatch installation.
- Test: `tests/` — extend the existing docs sentinel test file that covers README installation
  claims if one exists (`grep -rln "sync-agents" tests/` to find it), else add
  `tests/test_install_docs_claims.sh`.

- [ ] **Step 1: Write the doc changes:** describe Go-owned automatic repository surface
  reconciliation; the explicit `agent_harnesses` opt-in with its three states (absent /
  non-empty / explicit-empty); `--repo-dir`; scoped `--harness`; global dispatch retirement
  safety (proof-gated, no `--force`, remedy-and-rerun); and the fresh-process requirement after
  any changed wrapper or parent surface ("clearing a conversation is not enough"). Remove any
  sentence claiming the Bash `sync-agents.sh`/`install.sh` synchronization path still runs.
  **Answer the change file's open question here:** add a short "stale project-level wrappers"
  note to the Claude setup docs — Docket installs agent wrappers machine-globally only; a
  repository-local `.claude/agents/docket-*.md` copy (as the pre-0334 era left behind) shadows
  the guarded global wrapper and re-enables recursive self-dispatch; the remedy is to delete
  those project-level copies, which Docket will not touch because it never owned them.
- [ ] **Step 2: Guard the claims** with sentinel asserts in the shell test: the README names
  `agent_harnesses` within bounded distance of "opt-in" (bind phrase to claim; learnings:
  `prose-guard-binds-phrase-to-claim`, collapse whitespace before matching per
  `phrase-grep-over-wrapped-prose`), and a negated assert (with `grep -qF --`-style safe
  invocation per repo shell rules) that the installation section no longer instructs running
  `sync-agents.sh`.
- [ ] **Step 3: Run** the touched shell tests directly, then `go run ./cmd/genassets` and
  `go test -count=1 ./internal/assets/` if authored skill/reference sources moved.
- [ ] **Step 4: Commit** `docs: repository dispatch opt-in, retirement safety, fresh-process requirement`

---

### Task 12: Whole-suite gate and human-verify record

- [ ] **Step 1:** Run the full suite: `scripts/run-tests.sh` (the `finalize.test_command`
  resolution — read it there if it moved). Treat `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` lines
  as screening findings and any `SERIAL CONFIRMED OVER BUDGET:` as an authoritative breach to
  act on. Also run `go test -race -count=1 ./...` if the suite does not already.
- [ ] **Step 2:** Fix anything red (each fix is its own minimal commit).
- [ ] **Step 3:** Record in the build evidence / results notes the spec's harness-contract
  residue as HUMAN VERIFICATION items, not claims: after one real
  `docket development install --source /Users/homer/dev/docket`, a FRESH process of each of
  Claude Code, Codex, Cursor, and OpenCode must observe the installed recursion guard in its
  wrappers and resolve the named Docket agents through the compact repository surface; a
  process started before the install is invalid evidence; any harness whose live vendor
  behavior is not exercised is reported *unverified*, never inferred from another harness.
- [ ] **Step 4: Commit** any evidence/doc residue: `test: full-suite gate for change 0351`

---

## Self-Review (performed while writing)

- **Spec coverage:** command boundary + `--repo-dir` (Tasks 5, 10); repository harness
  configuration and three states (Tasks 4, 8, 10); parent-facing surfaces incl. Claude
  share/regular-file paths and Codex/OpenCode co-ownership (Task 6); ownership isolation per
  worktree (Tasks 7, 8); fresh-binary handoff incl. recursion stop, revalidation, self-bytes
  binary target, release-install unaffected (Task 9); unified preflight/transaction, journaled
  dual state publication, recovery (Task 8); retirement of global dispatch artifacts incl. the
  no-recreate guarantee (Task 3 — adapters no longer plan the target, so recreation is
  structurally impossible; the cross-harness inventory assert pins it); diagnostics/observable
  behavior (Tasks 8–10: `ReasonHandoffFailed`, `ReasonInvalidRepoDir`, not-authorized action,
  scope-visible reporting, no parent success before candidate completion); acceptance matrices
  (fresh-render Task 9 Step 4; retirement matrix Task 3 Step 2; repository matrix Tasks 8/10;
  atomicity/recovery + mutation-tested all-or-nothing Task 8; harness contract Task 3 Step 1 +
  Task 12 human items; documentation + full suite Tasks 11–12).
- **Known intentional narrowing:** repository `check` semantics stay at "named no-op /
  machine-only" — the spec assigns repository *reconciliation* to install operations and a
  future `repository check` change reuses the reconciler (spec non-goals).
- **Type consistency:** `RepoPhase` is defined in Task 8 and consumed in Tasks 9–10;
  `WorktreeIdentity` in Task 5 feeds Task 10; `RemoveBlock` (Task 1) feeds Task 2;
  `GlobalDispatchTarget` (Task 3) is per-adapter; `Value[[]string]` provenance guard wording is
  identical in Tasks 4 and 10.
