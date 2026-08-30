<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0377 — Migrate deferred Bash-facade workflow operations to native Go CLI verbs](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0377-migrate-deferred-bash-facade-workflow-operations-to-native-g.md)**
<!-- docket:backlink:end -->
# Migrate Deferred Bash-Facade Workflow Operations to Native Go CLI Verbs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Supply the last native workflow capabilities (structured `docket repository prepare`, derived-view health/repair, shared render-inclusion, typed stack/context coverage) and cut every maintained skill and generated product over to the typed Go CLI, ending at a mutation-tested zero-maintained-consumer seal — while leaving the frozen Bash facade present, green, and deletable by change 0370.

**Architecture:** Capability-oriented closure: existing typed commands absorb behavior they already own; only `repository.prepare` is genuinely new. Board/artifact-link/ADR-index rendering stays with the mutation transactions that own it, consolidated onto one shared app-layer inclusion path guarded by a mutation-shape invariant. Health (`repository check`) and authorized repair (`repository migrate`) cover deterministic derived-view drift. Skills consume protocol-v1 JSON; no forwarding shim, no one-for-one Bash verbs.

**Tech Stack:** Go (cobra CLI in `internal/cli`, operations in `internal/app`, classification in `internal/reposetup`, rendering in `internal/render`, snapshots in `internal/repository`), bash guard tests in `tests/`, canonical asset generator for `internal/assets/embedded`.

**Spec:** `docs/superpowers/specs/2026-08-30-migrate-deferred-bash-facade-workflow-operations-to-native-g-design.md` — lives on the `docket` metadata branch; read it from the `.docket` worktree.

## Global Constraints

- **Frozen facade untouched:** `scripts/docket.sh`, `scripts/lib/`, `scripts/run-tests.sh`, every `scripts/*.sh` and its `.md` contract, and the legacy bash tests stay byte-identical except for mechanically required test integration (acceptance 15). Change 0370 owns deletion; 0377 performs no 0370 work (acceptance 17).
- **No shell-assignment or `eval` interface** on any new command (acceptance 3). No `DOCKET_*` transport variables in the prepare result.
- **No compatibility verbs:** no `stack-base`/`stack-children`/`stack-closeout`/board-pass Go commands, no umbrella workflow facade, no forwarding shim.
- **Protocol-v1 envelopes** everywhere: stable operation key, closed disposition vocabulary, structured findings, typed context. Exit codes are presentation only (ADR-0074; learning `exit-code-encodes-a-non-failure`).
- **Fail closed:** probe error is never clean absence; a mutation commits its complete primary-plus-derived file set or writes nothing.
- **Test placement and runtime (acceptance 18):** long-running/real-repository scenarios go in the existing integration group (files named `*_integration_test.go` behind the integration build tag — copy the tag line from `internal/app/reposetup_integration_test.go` verbatim); no individual test may exceed 60 seconds — split or shard instead; the whole gate is `go run ./cmd/docket development test` from this checkout; inspect `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` lines; any `SERIAL CONFIRMED OVER BUDGET:` is an authoritative breach that must be resolved before the task commits.
- **Mutation discipline:** every new guard is mutation-tested (strip the guarded thing, watch it redden) with Go's cache defeated (`go test -count=1 …`; learning `cached-runner-serves-a-mutated-tree`). Guards key on syntactic shape, never enumerated spellings; site lists are derived by whole-repo grep, never hand-written (AGENTS.md).
- **Never hand-edit** `internal/assets/embedded/tree/**` — it is generator output; change canonical sources and regenerate.
- **GitHub board mirroring, main mode, and deferred product features stay absent/unsupported** (acceptance 16) — do not add repair or write paths for them.

---

## File Structure

| Area | Files |
|---|---|
| Prepare op | Create `internal/app/repository_prepare.go`, `internal/app/repository_prepare_test.go`, `internal/app/repoprepare_integration_test.go`; Modify `internal/cli/repository.go`, `internal/cli/repository_test.go` |
| Health/repair | Modify `internal/app/repository_check.go` (+test), `internal/app/repository_migrate.go` (+test), `internal/reposetup/*` finding vocabulary, `internal/app/repomigration_integration_test.go`, `internal/app/repocheck_integration_test.go` |
| Context gaps | Modify `internal/app/finalize_context.go` and/or `internal/app/maintenance.go` (+tests) only where the derived consumer inventory proves a gap |
| Shared inclusion | Create `internal/app/derived_views.go`, `internal/app/derived_views_test.go`, `internal/app/derived_views_guard_test.go`; Modify the 14 `render.Board(` call-site files in `internal/app` plus `internal/app/adr_ops.go` |
| Skills cutover | Modify `skills/*/SKILL.md` (all 12 skills as inventory demands), `skills/docket-convention/references/stacked-changes.md`, `skills/docket-convention/references/terminal-close-out.md`, maintained operator material (`README.md` where it instructs execution) |
| Products | Regenerate `internal/assets/embedded/tree/**` via the canonical generator |
| Seal | Create `tests/test_facade_consumer_seal.sh`; Modify `tests/test_go_consumer_migration_guard.sh` and any bash guard that greps rewired skill text (mechanical integration only) |

---

## Phase 1 — Land and test repository preparation (spec §Sequencing 1)

### Task 1: `repository.prepare` app operation — classification, refusal matrix, typed context

**Files:**
- Create: `internal/app/repository_prepare.go`
- Test: `internal/app/repository_prepare_test.go`

**Interfaces:**
- Consumes: `GatherSetupFacts(ctx, d SetupDeps, forMutation bool) (reposetup.Facts, setupContext, error)` and the classifier in `internal/reposetup` (pattern: `internal/app/repository_check.go` `RunRepositoryCheck`); `config.Effective` from `resolveSetupConfig`; `gitcli` worktree/fetch primitives (pattern: worktree attachment in `internal/app/repository_migrate.go` and `internal/reposeed`).
- Produces:
  - `const OperationRepositoryPrepare = "repository.prepare"`
  - `type PrepareOptions struct { RepoDir string }`
  - `type RepositoryPrepareResult struct { Disposition string; RepositoryState string; Context *PrepareContext; Findings []reposetup.Finding; Notices []string }` with closed disposition vocabulary `applied | no-op | refused | error`.
  - `type PrepareContext struct` — the closed typed context (spec §Structured result): `RepoRoot`, `OriginURL`, `DefaultBranch`, `IntegrationBranch`, `MetadataBranch` (each with pinned revision fields `…Revision`), `MetadataWorktreePath`, `ChangesDir`, `AdrsDir`, `ResultsDir`, `Finalize` (test command, merge method — mirror the supported `config.Effective` finalize fields), `Skills` (resolved workflow skill bindings), `ConfigDiagnostics []string`. **No generic map, no `DOCKET_*` names, no shell quoting.** Derive the exact field list from what current maintained consumers read out of `docket.sh preflight`/`env` — enumerate by `grep -rn 'DOCKET_[A-Z_]*' skills/ scripts/docket-status.sh scripts/lib/` and map each consumed variable to a typed field or record in a code comment why it is dropped (unused transport → dropped, per spec).
- Ordered flow (spec §Purpose and interface): discover repo/origin → pin default branch + load committed config + machine layers + integration branch → gather classifier facts, classify once → require fixed Go-v1 `docket`-branch topology → ensure `.docket` worktree exists/registered/hooks-disabled → synchronize clean-behind worktree to pinned remote metadata revision → return context. No planning record, no lifecycle decision, no remote push.

**Steps:**

- [ ] **Step 1: Write failing unit tests for the refusal/attach/fast-forward matrix.** Use the existing fake `setupProber` pattern from `internal/app/repository_facts_test.go` / `repository_check_test.go`. One test per disposition row, asserting BOTH the disposition and its mechanism (learning `assert-pins-outcome-not-mechanism`) — pin the finding code and the exact remedy string:

```go
func TestRepositoryPrepareFreshRefusesWithInitRemedy(t *testing.T) { /* state: fresh → Disposition "refused", remedy contains exactly "docket repository init" */ }
func TestRepositoryPrepareLegacyRefusesWithMigrateRemedy(t *testing.T) { /* legacy → refused, remedy "docket repository migrate" */ }
func TestRepositoryPrepareHealthyMissingWorktreeAttaches(t *testing.T) { /* healthy remote, no local .docket → applied, attach recorded */ }
func TestRepositoryPrepareCleanBehindFastForwards(t *testing.T) { /* clean strictly-behind → applied, ffwd to pinned remote revision */ }
func TestRepositoryPrepareCleanCurrentIsNoOp(t *testing.T) { /* already at pinned revision → no-op */ }
func TestRepositoryPrepareDirtyRefuses(t *testing.T)      // also: ahead, diverged, foreign, ambiguous registration, probe-unknown — each its OWN test + distinct finding code
func TestRepositoryPrepareInvalidConfigRefusesBeforeSync(t *testing.T) { /* config error → refused; assert NO sync attempt reached the prober */ }
func TestRepositoryPrepareProbeErrorIsNotAbsence(t *testing.T) { /* prober returns error → error disposition, never attach (learning probe-error-is-not-clean-absence) */ }
func TestRepositoryPrepareContextFieldsTyped(t *testing.T) { /* result JSON has no DOCKET_ key, no flat string map; assert resolved non-default values so wiring is visible (learning defaulted-param-hides-caller-wiring) */ }
```

- [ ] **Step 2: Run tests, verify they fail** — `go test ./internal/app/ -run TestRepositoryPrepare -count=1` fails with undefined `RunRepositoryPrepare`.
- [ ] **Step 3: Implement `RunRepositoryPrepare(ctx context.Context, d SetupDeps, o PrepareOptions) RepositoryPrepareResult`** following the ordered flow. Never initialize or migrate implicitly; attachment and fast-forward are the only mutations, both idempotent and keyed on remote state (learning `idempotency-keying`). Lost-response retry converges by re-reading topology + revision.
- [ ] **Step 4: Run tests to green** — same command, PASS.
- [ ] **Step 5: Self-review against spec §State and refusal behavior** (every bullet has a test) and commit: `git add internal/app/repository_prepare.go internal/app/repository_prepare_test.go && git commit -m "feat(0377): repository.prepare app operation with typed context and fail-closed sync"`.

### Task 2: CLI `docket repository prepare --repo-dir <dir> --json`

**Files:**
- Modify: `internal/cli/repository.go`
- Test: `internal/cli/repository_test.go`

**Interfaces:**
- Consumes: `RunRepositoryPrepare` from Task 1; the existing repository-subcommand registration and presenter/JSON-mode plumbing in `internal/cli/repository.go` (`Use: name` helper) and `internal/cli/jsonmode.go`.
- Produces: `docket repository prepare` subcommand, protocol-v1 JSON envelope on `--json`, short **redacted** human summary otherwise (no full config dump); exit code mapped by the same presentation rule as `repository check` (`CheckExitCode` pattern).

**Steps:**

- [ ] **Step 1: Write failing CLI tests** in `internal/cli/repository_test.go` (follow the existing `repository check` CLI test pattern): command registered under `repository`; `--json` emits envelope with `"operation":"repository.prepare"`; human output contains a one-line summary and NOT the origin URL credentials or full skills map; unknown flag refused.
- [ ] **Step 2: Run to fail** — `go test ./internal/cli/ -run Prepare -count=1`.
- [ ] **Step 3: Implement the subcommand** wiring `--repo-dir` (default: discovered cwd repo via `repodir.go` helper) through `SetupDeps` exactly as `repository check` does.
- [ ] **Step 4: Run to pass**, then run the package suites: `go test ./internal/cli/ ./internal/app/ -count=1`.
- [ ] **Step 5: Commit** — `git add internal/cli/repository.go internal/cli/repository_test.go && git commit -m "feat(0377): docket repository prepare CLI verb"`.

### Task 3: Prepare integration tests against real local repositories

**Files:**
- Create: `internal/app/repoprepare_integration_test.go` (integration build tag — copy the tag line verbatim from `internal/app/reposetup_integration_test.go`)
- Modify: `tests/` gains the matching runner entry ONLY if the suite requires one per group — check how `test_go_integration_app_repocheck.sh` is wired and mirror it as `tests/test_go_integration_app_repoprepare.sh`; read `tests/README.md` first.

**Interfaces:**
- Consumes: the local-remote fixture helpers used by `reposetup_integration_test.go` / `repomigration_integration_test.go` (bare origin + clone).
- Produces: end-to-end proof for: worktree attachment to a healthy remote; clean-behind fast-forward to the pinned revision; dirty refusal (worktree file modified → refused, file byte-identical after); ahead refusal; diverged refusal; re-run idempotence (second prepare → `no-op`).

**Steps:**

- [ ] **Step 1: Write the failing integration tests.** Each scenario builds a bare origin with a seeded `docket` branch, a primary clone, then asserts disposition + on-disk state. The refusal tests assert the worktree bytes are untouched afterward (positive evidence, not just exit disposition).
- [ ] **Step 2: Run to fail/finish** — use the group command the runner uses (see the wiring found in Step 0 of this task); confirm each test completes well under 60s; split any that does not.
- [ ] **Step 3: Implement fixes** surfaced by real-git behavior (worktree registration probing, hooks disabling via the same mechanism `scripts/disable-worktree-hooks.sh` documents — read it as the behavioral oracle, implement natively).
- [ ] **Step 4: Run the whole suite** — `go run ./cmd/docket development test`; inspect budget lines.
- [ ] **Step 5: Commit** — `git add internal/app/repoprepare_integration_test.go tests/ && git commit -m "test(0377): repository prepare integration coverage (attach, ffwd, refusal, idempotence)"`.

---

## Phase 2 — Fill only the proven health/context gaps (spec §Sequencing 2)

### Task 4: Derive and record the maintained-consumer inventory

**Files:**
- Create: none committed as a standalone doc — the inventory lands as the header comment of the seal test in Task 13 and drives Tasks 5–11. Produce it now and paste it into the task-11/13 work notes (evidence record), because Tasks 5, 6, 9–11 each consume it.

**Steps:**

- [ ] **Step 1: Derive the complete executable inventory by shape, never by hand-list** (AGENTS.md; spec §Legacy capability disposition "derives the complete executable inventory"):

```bash
grep -rnE '[^[:alnum:]_-]docket\.sh[^[:alnum:]_-]|DOCKET_SCRIPTS_DIR|DOCKET_BASH_PATH' \
  --exclude-dir=.git --exclude-dir=.worktrees --exclude-dir=.docket . | \
  grep -vE '^\./(docs/|internal/repository/testdata/corpus/)'
```

- [ ] **Step 2: Classify every hit** into: (a) maintained executable (skills, agent instructions, generated products, operator docs an agent runs verbatim — learning `agent-executed-markdown-is-code`); (b) frozen Bash implementation + its parity/deletion tests (`scripts/`, `tests/test_docket_*` etc.); (c) immutable history (`docs/`); (d) frozen release artifacts. For each class-(a) site, map the operation to its native disposition using the spec's Legacy-capability table.
- [ ] **Step 3: For each `stack-children`/`stack-closeout`/`adr-checks` class-(a) consumer, record which typed field or finding it needs** — this is the gap list Task 6 implements. If a spec-anticipated gap has no consumer, it is NOT built (YAGNI; spec: "extending only narrow gaps").
- [ ] **Step 4: Commit nothing yet** — carry the inventory forward in the build evidence record.

### Task 5: Derived-view drift findings in `repository check` + mechanical repair in `repository migrate`

**Files:**
- Modify: `internal/app/repository_check.go`, `internal/app/repository_check_test.go`
- Modify: `internal/app/repository_migrate.go`, `internal/app/repository_migrate_test.go`
- Modify: `internal/reposetup` finding/repair vocabulary files (locate with `grep -rn 'RepairFinding\|Finding{' internal/reposetup | grep -v _test`)
- Modify: `internal/app/repocheck_integration_test.go`, `internal/app/repomigration_integration_test.go`

**Interfaces:**
- Consumes: `gatherFrontmatterFindings`/`corpusFindings` (existing corpus read + `repository.BuildSnapshot`), `render.Board(render.BoardInput{Snapshot})`, `render.ADRIndex(snap)`, the artifact-links renderer in `internal/render/artifacts.go`, and migrate's existing preview/authorization model (`MigrateOptions`, pinned-revision re-proof).
- Produces: new stable finding codes (extend the closed vocabulary; keep naming style of existing codes): `board-stale`, `board-malformed`, `artifact-links-stale`, `artifact-links-missing`, `artifact-links-malformed`, `adr-index-stale`, `adr-index-malformed`, plus ADR-ledger consistency findings (identity/status/relationship/filename) **only where `repository.BuildSnapshot` does not already emit them** — check first with `grep -rn 'Finding' internal/repository/validate.go`. Each deterministic byte-difference finding carries `Repairable: true`; malformed markers, authored-prose divergence, illegal ADR evolution, missing referenced artifacts stay `Repairable: false` (manual review).

**Steps:**

- [ ] **Step 1: Write failing check tests** (unit, corpus-record fixtures per `repository_check_test.go` pattern): stale board bytes → `board-stale` repairable; board matching canonical render → no finding; dangling/out-of-order artifact-link markers → `artifact-links-malformed` NOT repairable (AGENTS.md marker order/balance rule); missing block where the record has artifacts → `artifact-links-missing` repairable; ADR index drift → `adr-index-stale`; a corpus read ERROR yields an error-severity outcome, never "no findings, all clean" (extend the existing `gatherFrontmatterFindings` read-error behavior test — a check must not fabricate absence).
- [ ] **Step 2: Run to fail** — `go test ./internal/app/ -run 'RepositoryCheck' -count=1`.
- [ ] **Step 3: Implement check-side comparison**: render candidate bytes from the pinned corpus snapshot through the canonical renderers and byte-compare against the corpus files. Reuse one comparison helper for all three views.
- [ ] **Step 4: Write failing migrate tests**: preview on a healthy-topology repo with stale board reports the exact pinned source revision + file set; authorized retry re-proves the revision (stale revision → refused `revision-moved`-style finding, follow migrate's existing code) and rewrites ONLY canonical derived bytes; marker order/balance validated before any block replacement (malformed → refuse, file untouched); authored content never modified (fixture: record with drifted authored prose AND stale board → board repaired, prose untouched).
- [ ] **Step 5: Implement migrate-side repair**, extending the existing preview/authorize path so repair works even when topology is already healthy (today migrate targets legacy conversion — add the healthy-with-repairable-findings entry condition).
- [ ] **Step 6: Run app tests + integration groups to green**; whole suite `go run ./cmd/docket development test`.
- [ ] **Step 7: Commit** — `git commit -m "feat(0377): derived-view drift findings in repository check and authorized mechanical repair in migrate"` (add exactly the files touched; never `add -A`).

### Task 6: Typed context extensions for proven stack/ADR-check gaps

**Files:**
- Modify (only as Task 4's gap list demands): `internal/app/finalize_context.go` (+`finalize_context_test.go`), `internal/app/maintenance.go` (+test), possibly `internal/app/implementation_context.go` (+test)
- Modify: `internal/cli/context.go`/`internal/cli/finalize.go` tests only if the JSON surface gains fields

**Interfaces:**
- Consumes: Task 4's gap list; the existing effective-base result in `implementation_context.go` (spec: `stack-base` is already covered — verify, do not rebuild); descendant computation already inside finalize/maintenance (locate: `grep -rn 'descendant\|children' internal/app/finalize_*.go internal/app/maintenance.go | grep -v _test`).
- Produces: for each retained `stack-children` consumer lacking a typed field, the field added to the existing finalize or maintenance context result (e.g. `Descendants []DescendantInfo` with change id, branch, status) — extension of an existing struct, never a new command. `adr-checks` consumers point at `repository check` findings (Task 5) — no ADR checker command.

**Steps:**

- [ ] **Step 1: For each gap on the list, write the failing test first** against the owning context operation, asserting the typed field's content on a fixture with a real stack (parent + two children, one killed — killed/missing parents keep their fail-closed outcomes, no integration-branch fallback; assert the refusal finding, not a fallback branch).
- [ ] **Step 2: Run to fail, implement, run to green** — `go test ./internal/app/ -count=1`.
- [ ] **Step 3: If the gap list is empty for a capability, record that in the evidence record and build nothing** (this is an acceptable, spec-sanctioned outcome).
- [ ] **Step 4: Commit** — `git commit -m "feat(0377): typed context fields for retained stack consumers (inventory-proven gaps only)"`.

---

## Phase 3 — Shared derived-view inclusion + mutation guard (spec §Sequencing 3)

### Task 7: One shared app-layer inclusion path for board / artifact-links / ADR-index

**Files:**
- Create: `internal/app/derived_views.go`
- Test: `internal/app/derived_views_test.go`
- Modify: every current direct render call site in `internal/app` — derive the list, do not trust this plan's snapshot: `grep -rn 'render\.Board(\|render\.ADRIndex(' internal/app | grep -v _test`. At plan time: 14 `render.Board(` sites (`change_create.go`, `change_claim.go`, `change_groom.go`, `change_attach.go`, `change_reconcile.go`, `change_repair.go`, `change_reclaim.go`, `change_kill.go`, `change_lifecycle.go`, `change_implemented.go`, `finalize_block.go`, `finalize_closeout.go` ×2, `status_git.go` if mutating) and 2 `render.ADRIndex(` sites in `adr_ops.go`.

**Interfaces:**
- Consumes: `render.Board`, `render.ADRIndex`, artifact-links renderer, `transaction.Tree`, `boardMutationKind` (hoist it out of `change_create.go` into the new file).
- Produces:

```go
// includeBoard renders the candidate snapshot through the canonical board
// renderer and appends BOARD.md to the transaction file set. Every
// board-authoritative mutation MUST route here (guarded by
// derived_views_guard_test.go).
func includeBoard(ctx context.Context, tree transaction.Tree, boardPath string, candidate domain.Snapshot, files *[]transaction.FileChange) error
func includeADRIndex(candidate domain.Snapshot, indexPath string, files *[]transaction.FileChange) error
func includeArtifactLinks(/* record + candidate snapshot */) error // signature per the current per-op artifact-link rendering — diff the call sites BEFORE templating (learning consolidation-flattens-caller-variance)
```

**Steps:**

- [ ] **Step 1: Diff the existing call sites against each other first** (learning `consolidation-flattens-caller-variance`): confirm every site renders from the candidate snapshot with identical input shape; any variance (e.g. `finalize_block.go` uses `snap`) is either intentional (preserve, document in the helper's comment) or a latent bug (fix in its own commit).
- [ ] **Step 2: Write failing unit tests** for the helpers: rendered bytes byte-equal to a direct `render.Board` call; mutation kind create-vs-replace preserved; renderer error propagates and leaves `files` unmodified (no partial append).
- [ ] **Step 3: Implement helpers, then convert call sites one file at a time**, running `go test ./internal/app/ -count=1` after each file. Intermediate states stay buildable (learning `intermediate-task-state-buildable`).
- [ ] **Step 4: Assert no private copies remain**: `grep -rn 'render\.Board(\|render\.ADRIndex(' internal/app | grep -v _test | grep -v derived_views.go` must list zero lines.
- [ ] **Step 5: Whole suite green** — `go run ./cmd/docket development test`.
- [ ] **Step 6: Commit** — `git commit -m "refactor(0377): shared derived-view inclusion path for board, artifact links, ADR index"`.

### Task 8: Mutation-shape structural guard over board-authoritative writes

**Files:**
- Create: `internal/app/derived_views_guard_test.go`

**Interfaces:**
- Consumes: Go AST/package inspection over `internal/app` (use `go/parser` + `go/ast` in the test, or `packages.Load`), the transaction file-set construction sites.
- Produces: a compile-time-ish invariant test: **any function in `internal/app` that adds a change-record path (a path under the configured changes dirs) to a transaction file set must, in the same function or its callees, reach `includeBoard`** — keyed on the mutation's shape (writes a change record) and common ownership, never a command-name list (spec §Derived-view ownership).

**Steps:**

- [ ] **Step 1: Write the guard test.** Implementation approach: parse `internal/app` packages; find call sites of the transaction-append idiom for change-record paths (identify the concrete idiom while doing Task 7 and pin it here — e.g. construction of `repository.InputDocument{Kind: repository.KindChange, …}` feeding `BuildSnapshot` followed by a commit of the file set); for each enclosing top-level operation, require a reachable `includeBoard` call. Fail with the function name on violation. Also enforce a **population floor**: assert the scanner found ≥ the count of operations derived at runtime from the census itself, and cross-check both directions (learning `correspondence-guard-runs-one-way`, `marker-scoped-guard-needs-a-population-floor`): every `includeBoard` caller is a change-record mutator, and every change-record mutator calls `includeBoard`.
- [ ] **Step 2: Mutation-test it, both halves** (learning `backstop-must-compute-not-reenumerate`): (a) in a scratch copy of one op (e.g. `change_kill.go`), delete the `includeBoard` call → `go test ./internal/app/ -run DerivedViewsGuard -count=1` REDDENS naming the function; (b) delete the guard's population source (point it at an empty package) → the floor reddens, proving non-vacuity. Restore from a backup copy, not `git checkout` (learning `mutation-restore-needs-a-backup-copy`) — the working tree holds Task 7/8 uncommitted work only if you skipped Task 7's commit; verify tree state first.
- [ ] **Step 3: Record both mutation readings in the evidence record.** Run whole suite.
- [ ] **Step 4: Commit** — `git commit -m "test(0377): mutation-shape guard: change-record mutations must include canonical board output"`.

---

## Phase 4 — Rewire canonical skills and references (spec §Sequencing 4)

Command implementations (Phases 1–3) are all landed before any call site moves; the frozen facade stays functional throughout, so the migration host can build its own replacement.

### Task 9: Rewire status, ADR, and convention derived-view prose

**Files:**
- Modify: `skills/docket-status/SKILL.md`, `skills/docket-adr/SKILL.md`, `skills/docket-convention/SKILL.md`
- Modify: the bash guard tests that grep these skills' old text — derive: `grep -rln 'docket-status\|docket-adr\|docket-convention' tests/*.sh`, then for each, run it and fix ONLY assertions that pin the rewired prose (learning `restatement-accumulates-its-own-guards`: the copies are load-bearing; acceptance 15 permits mechanically required test integration only).

**Cutover mapping (from spec §Legacy capability disposition + §Workflow data flow):**
- Step-0 preamble: `docket.sh preflight` / `DOCKET_SCRIPTS_DIR` resolution → `docket repository prepare --repo-dir <dir> --json`, validate the envelope, carry typed context values literally.
- Plain status: `docket.sh docket-status` → `docket maintenance sweep --json` then read-only `docket status --json`.
- `--board-only`: removed from maintained workflows entirely — no replacement command; board renders atomically with mutations. Exceptional drift → `docket repository check` / authorized `docket repository migrate`.
- ADR skill: typed record/supersede/reverse operations own index rendering; the repair-only `docket.sh render-adr-index` escape hatch becomes `docket repository migrate` repair; `adr-checks` → `docket repository check` findings.

**Steps:**

- [ ] **Step 1: Rewire each SKILL.md** per the mapping. No skill parses human output, reconstructs a branch from prose, invokes a renderer, edits a managed block, or follows a typed mutation with a second board commit (spec §Workflow data flow — treat as a review checklist per file).
- [ ] **Step 2: Run the bash test files identified above** plus `tests/test_go_consumer_migration_guard.sh`; fix reddened guards by re-pointing their positive floors at the new Go invocations (each migrated file must still carry a positive floor — its new Go verb — so deletion reddens).
- [ ] **Step 3: Whole suite** — `go run ./cmd/docket development test`.
- [ ] **Step 4: Commit** — `git commit -m "feat(0377): cut status/ADR/convention skills to native prepare, sweep+status, and typed ADR ops"`.

### Task 10: Rewire implement-next, finalize, and stack references

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-convention/references/stacked-changes.md`, `skills/docket-convention/references/terminal-close-out.md`
- Modify: reddened bash guards (same derivation as Task 9)

**Cutover mapping:**
- `stack-base` → `docket context implementation` effective-base result (already typed — cite the exact JSON field name from `implementation_context.go`).
- `stack-children` → the finalize/maintenance context field from Task 6 (use the exact field name Task 6 produced).
- `stack-closeout` → typed finalize operations + `docket maintenance sweep`.
- terminal-close-out's killed-outcome `docket.sh archive-change` leg → the typed operation that owns killed-change archival (verify which finalize/maintenance op covers it via Task 4's inventory; if none does, that is a Task 6 gap that must already have been filled — check before rewiring).
- Step-0 preamble → `docket repository prepare --json` as in Task 9.

**Steps:**

- [ ] **Step 1: Rewire the four files** per the mapping, preserving each skill's workflow structure (only the command surface changes).
- [ ] **Step 2: Update `tests/test_go_consumer_migration_guard.sh`'s stage-local slices** — its header documents two frozen legacy callers inside migrated files (terminal-close-out killed leg, docket-adr repair leg); Tasks 9–10 remove both, so the slice exemptions and their named terminators must be retired in the same commit, keeping the guard's positive floors.
- [ ] **Step 3: Whole suite green.**
- [ ] **Step 4: Commit** — `git commit -m "feat(0377): cut implement-next/finalize skills and stack references to typed context and finalize ops"`.

### Task 11: Rewire remaining skills, agent instructions, and operator material; remove env-var dependence

**Files:**
- Modify: `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-auto-groom/SKILL.md`, `skills/docket-brainstorm/SKILL.md` + `skills/docket-build*/SKILL.md` + `skills/docket-review/SKILL.md` (only if the Task 4 inventory shows facade hits), registered-agent parent instructions (derive: `grep -rln 'docket\.sh\|DOCKET_SCRIPTS_DIR\|DOCKET_BASH_PATH' agents/ .claude/ README.md 2>/dev/null` — class-(a) hits only), maintained operator docs.
- Do NOT touch: `docs/**`, `scripts/**`, frozen release artifacts, archived material (immutable history keeps point-in-time truth).

**Steps:**

- [ ] **Step 1: Sweep every remaining class-(a) inventory site** from Task 4 to its native disposition; new-change/groom flows submit authored Markdown through `docket change create` / `docket change groom` (atomic record+spec+links+board writes — no follow-up board step).
- [ ] **Step 2: Re-run the Task 4 inventory grep** and confirm every remaining hit is class (b), (c), or (d). Zero maintained executable hits remain.
- [ ] **Step 3: Fix reddened guards** (same discipline as Task 9 Step 2). Whole suite green.
- [ ] **Step 4: Commit** — `git commit -m "feat(0377): complete maintained-consumer cutover; drop DOCKET_SCRIPTS_DIR/DOCKET_BASH_PATH dependence"`.

---

## Phase 5 — Regenerate products and prove deterministic parity (spec §Sequencing 5)

### Task 12: Regenerate embedded assets and harness products; byte-clean double generation

**Files:**
- Regenerate: `internal/assets/embedded/tree/**` (never hand-edit)
- Modify: nothing else, unless the generator itself needs a source-path addition for a file created by Tasks 9–11 (then modify the generator's manifest, not its output)

**Steps:**

- [ ] **Step 1: Find the canonical generator** — `grep -rn 'go:generate\|embedded/tree' internal/assets/*.go Makefile 2>/dev/null | grep -v tree/` and read `tests/test_asset_bundle_drift.sh` to learn the sanctioned regeneration command. Run it.
- [ ] **Step 2: Run it a second time; assert byte-clean**: `git status --porcelain internal/assets/` after generation #2 must equal the state after #1 (empty diff between runs). A dirty second run is a generator-determinism defect to fix, not to commit around.
- [ ] **Step 3: Verify generated products carry the new command surface**: `grep -rln 'docket repository prepare' internal/assets/embedded/tree/skills/ | wc -l` ≥ the count of rewired operating skills, and the Task 4 inventory grep over `internal/assets/embedded/` returns zero class-(a) hits.
- [ ] **Step 4: Whole suite green** (drift guards like `test_asset_bundle_drift.sh` now prove source/product parity). Note: the session's installed `docket` binary and harness still hold pre-change products — do not "verify" by invoking the installed binary's skills (learning `generated-artifact-loaded-at-process-start`); parity is proven by the drift guard, not live behavior.
- [ ] **Step 5: Commit** — `git commit -m "chore(0377): regenerate embedded assets and harness products from rewired canonical sources"`.

---

## Phase 6 — Consumer seal + whole suite (spec §Sequencing 6)

### Task 13: Whole-repository shape-derived consumer seal

**Files:**
- Create: `tests/test_facade_consumer_seal.sh`
- Modify: `tests/runtime-budgets.tsv` if the runner requires a budget row (check `tests/README.md`)

**Design (spec §Consumer seal — every clause is a requirement):**
- **Detection shapes**, each a bounded byte pattern (learning `byte-pattern-guard-matches-a-spelling` — bound both sides with `[^[:alnum:]_-]` classes): direct execution (`docket.sh <op>`), variable-composed execution (`$…/docket.sh`, `"${DOCKET_SCRIPTS_DIR…}"`), indirect delegation (`bash …docket.sh`), sourced runtime dependency (`source`/`.` of `scripts/lib` or facade files), generator-emitted calls (scan `internal/assets/embedded/tree/**` with the same shapes).
- **Allowed categories are structural, never a filename/count allowlist** (ADR-0050): (1) the frozen Bash implementation and its parity/deletion tests — the `scripts/` directory and the legacy `tests/test_*.sh` files that exercise it, identified by directory + by the file's own shebang-and-fixture shape, not by name; (2) immutable history — `docs/` (the same exclusion line `tests/test_grep_portability.sh` draws) and `internal/repository/testdata/corpus/` (learning `frozen-fixture-corpus-trips-repo-wide-scans` — exclusion bounded to that exact path); (3) frozen release artifacts (locate the release-asset dir via `ls internal/release` and bound the exclusion to it).
- **An unknown executable site fails the seal** — classification is exhaustive: every hit must fall in an allowed category or the test fails printing the site.
- **Non-vacuity floor derived from the maintained product shape**: computed, not written (AGENTS.md `enumerated-floor`) — e.g. assert the scanner visited ≥ the live count of `skills/*/SKILL.md` + embedded-tree skill files, computing that count from the filesystem at run time; assert the frozen-facade population still exists (`scripts/docket.sh` present) so "seal green because 0370 already deleted everything" reads as the distinct condition it is.

**Steps:**

- [ ] **Step 1: Write the seal test** with the header comment carrying: the Task 4 inventory summary, each detection shape with its boundary rationale, each structural exclusion with its bound, and the named residual (prose paraphrases without the byte token survive; whole-branch review owns that — same residual the 0369 guard names).
- [ ] **Step 2: Mutation probes — one per forbidden shape** (spec: "Mutation probes introduce each forbidden shape in maintained material and prove the guard reddens"): for each of the five detection shapes, plant a violating line in a scratch copy of a maintained skill file (work on `"${TMPDIR:-/tmp}/seal-probe.XXXXXX"` copies of the tree, or back up + restore the real file around each probe), run the seal, assert RED with the site named. Then two vacuity probes: (a) point the scanner at an empty tree → the population floor reddens; (b) bypass canonical generation by planting a violation only in `internal/assets/embedded/tree/` → reddens (proves generated products are in-population).
- [ ] **Step 3: Run the seal against the real tree — GREEN.** Record all probe readings in the evidence record.
- [ ] **Step 4: Confirm the seal + legacy suites coexist**: the frozen facade's own tests (`test_docket_facade.sh`, `test_docket_preflight.sh`, etc.) still pass untouched — `go run ./cmd/docket development test`.
- [ ] **Step 5: Commit** — `git commit -m "test(0377): whole-repo shape-derived facade consumer seal with mutation-proved non-vacuity"`.

### Task 14: Final gate — whole suite, budgets, acceptance sweep

**Steps:**

- [ ] **Step 1: Run the whole gate** — `go run ./cmd/docket development test` from this checkout. All files green.
- [ ] **Step 2: Budget review** (acceptance 18): inspect every `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line; for any `SERIAL CONFIRMED OVER BUDGET:` line, treat as authoritative and resolve (split/shard the offending file; integration-scale scenarios belong in the integration group, and this change added several — new tests near their ceiling are already spent, per learning `budget-headroom-is-spent-before-it-is-breached`).
- [ ] **Step 3: Acceptance sweep** — walk the spec's 18 acceptance criteria; for each, name the commit/test that satisfies it in the evidence record. Specifically re-verify 15 (frozen surface byte-identical: `git diff <pre-dispatch-HEAD>..HEAD --stat -- scripts/ | cat` shows nothing, and the legacy test diff contains only the mechanical integrations Tasks 9–11 justified) and 17 (no 0370 branch/plan/PR artifacts: `git log --all --oneline --grep 0370` shows no new work).
- [ ] **Step 4: Commit any evidence-record updates** — `git commit -m "test(0377): final suite gate and acceptance sweep"` (only if files changed).

---

## Self-Review Notes (performed at plan time)

- **Spec coverage:** §prepare → Tasks 1–3; §Workflow data flow + §Legacy disposition table → Tasks 9–11; §Derived-view ownership → Tasks 7–8; §Health and repair → Task 5; §Stack behavior → Tasks 6, 10; §Skill/product migration → Tasks 9–12; §Consumer seal → Task 13; §Verification → distributed + Task 14; §Sequencing 1–6 → Phases 1–6 in order. Acceptance 1–18 each map to a named task (sweep re-run in Task 14 Step 3).
- **Ordering:** commands land before call sites (Phases 1–3 before 4), intermediate commits buildable, facade functional throughout — matches §Sequencing and recovery.
- **Deliberately derived-not-enumerated:** call-site lists, consumer inventory, guard populations, and non-vacuity floors are specified as derivation commands rather than frozen lists — that is the repo's standing rule (AGENTS.md), not a placeholder.
