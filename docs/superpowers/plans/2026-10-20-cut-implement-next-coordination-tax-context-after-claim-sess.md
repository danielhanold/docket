<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0360 — Stop implement-next tripping over its own bookkeeping](../../changes/archive/2026-10-21-0360-cut-implement-next-coordination-tax-context-after-claim-sess.md)**
<!-- docket:backlink:end -->

# Stop implement-next tripping over its own bookkeeping — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build` (one build-tier worker per task under the `docket-build-task` contract). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make an implement-next run's own writes succeed first time — correct revision lookups, a working post-claim/resume context read, no claim refreshes, fewer re-syncs, correct evidence and ADR wiring — without weakening any conflict check.

**Architecture:** Three small Go changes (`context.implementation --id` serves an in-progress change; `docket schema <id>` positional; `repository.prepare` drops its dead `skills` field), two skill-prose tasks rewriting `docket-implement-next` (plus its references, the convention's lease wording, and `docket-adr`), then deletion of the `change.refresh-claim` operation once no skill names it. Every skill edit is pinned by prose-contract rows (absence rows for what was removed, presence rows bound to the new claims) and the embedded asset tree is regenerated.

**Tech Stack:** Go (cobra CLI, `internal/app` operations, `internal/domain` policy), Markdown skills embedded via `go generate ./internal/assets/`, repoguard prose/budget guards.

**Spec:** `docs/superpowers/specs/2026-10-20-cut-implement-next-coordination-tax-context-after-claim-sess-design.md` (on the `docket` branch; read it from the metadata worktree). Change record: `docs/changes/active/0360-cut-implement-next-coordination-tax-context-after-claim-sess.md`.

## Global Constraints

- No new receipt field. `committed_revision` stays a commit id; the run looks the record revision up with `git -C <metadata_worktree_path> rev-parse HEAD:<record path>`.
- No conflict check removed or weakened: every write keeps its exact-revision check and exact-lease push; claim eligibility, stacked-base resolution, and selection (`context.implementation` with no `--id` → build-ready `proposed` only) are unchanged.
- `context.implementation --id` serves **only** `in-progress` beyond today's set; `blocked`, `deferred`, `implemented`, `stacked-merged`, `done`, `killed` refuse exactly as today; a `proposed` change keeps today's readiness refusals.
- Keep `domain.RefreshClaim`: `change.reconcile` and `change.resume-halted` call it; `claimed_at` is still stamped by `change.claim`, `change.reconcile`, `change.resume-halted` and cleared on leaving `in-progress`. `reclaim.lease_ttl`, `change.reclaim`, and the sweep's reclaim are unchanged.
- **Do not break the running build.** This run's controller executes the *installed* skills against the *installed* `docket` binary, which still needs `change.refresh-claim`. Edit only the feature worktree's source tree. Never run `development.install`, `link-skills.sh`, `install.sh`, or any reinstall during this build, and never edit `~/.claude`, `~/.agents`, `~/.cursor`, or the primary checkout.
- Skill edits: after every edit under `skills/` run `go generate ./internal/assets/` and commit the regenerated `internal/assets/embedded/tree/**` in the same commit.
- Skills ship into other repos: no sentence keyed to this repo (learning `distributed-body-has-no-local-repo`). Docs describe only current behavior — no change/PR citations in `docs/` or `skills/` prose. Code comments may cite change 0360.
- Point-in-time records (`docs/changes/archive/**`, `docs/superpowers/**`, `docs/release/**`, Accepted ADRs such as ADR-0059) stay as written.
- Cross-references in maintained source anchor on symbol names or quoted clauses, never line numbers.
- Every new or edited guard is mutation-tested: confirm the mutation landed with a before/after `/usr/bin/grep -cF` count, run with `go test -count=1`, restore from a backup copy (`cp f f.bak` … `mv -f f.bak f`), never `git checkout --` (learnings `mutation-restore-needs-a-backup-copy`, `cached-runner-serves-a-mutated-tree`).
- Before deleting any skill sentence, `git grep -nF` the test tree for that sentence (learning `restatement-accumulates-its-own-guards`); repoint a dependent assert at the surviving owner, never re-add the deleted text.

## Review Focus

1. **A plain named run on an in-progress change it does not own** (no resume) must still end as today — the bundle reads `claim_eligible: false` and the skill surfaces the refusal and ends `drained`; it must never claim, reconcile, or build. Pinned by Task 4's SKILL.md presence row on the not-claimable wording and Task 1's `claim_eligible: false` assertion.
2. **An in-progress change with no recorded `branch:`** (malformed/hand-edited) — `workflow.feature_branch` falls back to the minted name rather than an empty string. Pinned by Task 1 subtest `no recorded branch falls back to mint`.
3. **`docket schema <id> --operation <other-id>`** — a disagreement refuses as invalid arguments (exit 2), never silently picks one; two positionals refuse too. Pinned by Task 2.
4. **A stale local metadata checkout** (dirty/ahead/diverged after #549's best-effort follow) yields a `contended` write — the skill's one re-sync + one retry must apply to every post-claim write except reconcile, which re-authors. Pinned by Task 4's presence row on the retry rule.
5. **Several ADRs in one run** — each `docket-adr` dispatch needs a fresh revision lookup, because the previous `adr.record` moved the record. Pinned by Task 5's presence row "one at a time, each with a fresh lookup".

---

## File Structure

| File | Task | Responsibility of the edit |
|---|---|---|
| `internal/app/implementation_context.go` | 1 | explicit-id in-progress bundle; feature branch from recorded `branch:` |
| `internal/app/implementation_context_test.go` | 1 | unit tests for the new rule |
| `internal/app/workflow_integration_test.go` | 1, 6 | real-git post-claim context read (1); delete refresh test (6) |
| `docs/reference/glossary.md` | 1, 6 | *Implementation context* entry (1); claim-lease snippet and `claim_dispositions` row (6) |
| `internal/cli/schema.go`, `internal/cli/schema_command_test.go`, `docs/reference/cli.md` | 2 | positional operation id |
| `internal/app/repository_prepare.go`, `internal/cli/repository_test.go` | 3 | drop `PrepareSkills` |
| `skills/docket-implement-next/SKILL.md` | 4, 5 | the prose rewrite |
| `skills/docket-implement-next/references/{results-artifact,fix-pass,edge-paths}.md` | 4, 5 | refresh removal; PR-body move |
| `skills/docket-convention/SKILL.md` | 4 | `claimed_at` manifest comment |
| `skills/docket-adr/SKILL.md` | 5 | producing-change pass-through |
| `internal/repoguard/prose_contracts_test.go` | 4, 5 | new sentinel rows |
| `internal/repoguard/budgets_test.go` | 4, 5 | ceilings for edited skill files |
| `internal/repoguard/private_writing_rule_test.go` | 5 | drop edge-paths from writers; floor 10 → 9 |
| `internal/assets/embedded/tree/**` | 4, 5 | regenerated |
| `internal/app/change_claim.go`, `change_claim_test.go`, `change_claim_integration_test.go`, `change_integration_test.go`, `claim_workflow_git_test.go`, `schema_registry.go`, `internal/cli/change.go`, `change_test.go`, `install.go`, `revision_rename_test.go`, `internal/domain/actions.go` (comment only), `internal/repoguard/retired_vocabulary_test.go` (comment only) | 6 | delete `change.refresh-claim` |

---

### Task 1: `context.implementation --id` serves the run's in-progress change

**Files:**
- Modify: `internal/app/implementation_context.go` (`selectContextChange`, `ContextImplementation`, the `ContextWorkflow.FeatureBranch` construction and its comment)
- Test: `internal/app/implementation_context_test.go`, `internal/app/workflow_integration_test.go`
- Modify: `docs/reference/glossary.md` (*Implementation context* entry)

**Interfaces:**
- Produces: `func selectContextChange(snap domain.Snapshot, facts domain.BranchFacts, req ImplementationContextRequest) (domain.Change, string, *ImplementationContextResult)` — the new middle return is the claim-refusal reason (`""` when claimable). `ImplementationContext.ClaimEligible`/`ClaimRefusal` are now populated from it. No JSON field added (the source bytes already carry `claimed_at`/`reconciled`; do not add bundle fields).

- [ ] **Step 1: Write the failing unit tests** in `implementation_context_test.go`.

`TestContextImplementationServesInProgressChange` — fixture: an active in-progress record with a spec, e.g.
`"---\nid: 11\nslug: alpha\ntitle: Alpha\nstatus: 'in-progress'\npriority: high\ntype: feat\ncreated: 2026-01-02\nspec: <specA>\nbranch: 'hotfix/alpha'\nclaimed_at: '2026-01-03T00:00:00Z'\nreconciled: true\n---\n\nBody.\n"` with revision `"v11"` and the spec artifact present. Request `{ID: 11}`. Assert:
```go
got.Result == ResultApplied && got.Context != nil
got.Context.ClaimEligible == false
got.Context.ClaimRefusal == "not-ready-" + string(domain.ReadyNotProposed)
got.Context.Readiness == string(domain.ReadyNotProposed)
got.Context.Change.Revision == "v11"
got.Context.Change.Summary.Status == "in-progress"
got.Context.Workflow.FeatureBranch == "hotfix/alpha" // recorded branch, NOT the mint feat/alpha
got.Context.Halt.RunHalted == false
```
Subtests: `run halted marker is readable` (same record plus `"\n## Run halted\n\n### 2026-08-14\n\nPaused.\n"` → `Halt.RunHalted == true`); `no recorded branch falls back to mint` (record without `branch:` → `FeatureBranch == "feat/alpha"`).

`TestContextImplementationRefusesOtherNonProposedStatuses` — table over `blocked` (with `blocked_by: 'x'`), `deferred`, `implemented`: each a *valid* active record with a spec; `{ID: n}` → `Result == ResultInvalidState`, `Reason == "not-ready-not-proposed"`, `Context == nil`. Add `done` and `killed` rows if the fakeReader fixture can express an archived record validly (`repository.LocationArchive` or the equivalent location constant); if it cannot, say so in a comment on the table (the unchanged `ClaimEligibility` path covers them).

Add to `TestContextImplementationTypedAbsence` a case `"in-progress is never auto-selected"`: corpus only the in-progress fixture, `req: {}` → `ResultNoOp`, `ReasonContextNoCandidate`.

Keep `TestContextImplementationFeatureBranchHonorsMintPrefix` passing unchanged (proposed changes still preview the mint).

- [ ] **Step 2: Run to verify they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestContextImplementation(ServesInProgressChange|RefusesOtherNonProposedStatuses|TypedAbsence|FeatureBranchHonorsMintPrefix)$' -v`
Expected: `ServesInProgressChange` FAILS (result `invalid-state`, reason `not-ready-not-proposed`); the others pass.

- [ ] **Step 3: Implement.** In `selectContextChange`, after the lookup and before the `ClaimEligibility` refusal: when `c.Status() == domain.StatusInProgress` and `ClaimEligibility` fails, return `(c, fail.Reason, nil)` instead of a refusal result. Every other failure path is unchanged. In `ContextImplementation`, set `ClaimEligible: refusal == ""`, `ClaimRefusal: refusal`; set `FeatureBranch` to `selected.Branch().Value` when `selected.Status() == domain.StatusInProgress` and `selected.Branch()` is present and non-empty (same `State == domain.FieldPresent && Value != ""` test the spec link uses), else `domain.MintBranch(...)`. Rewrite the comment "The explicit change must be claimable exactly as a claim requires" to state the new rule (an explicit id must be claimable as a claim requires, except that the run's own `in-progress` change is served read-only with `claim_eligible: false`), and the `FeatureBranch` comment ("The context is produced BEFORE the claim…") to cover both cases. Update the `selectContextChange` doc comment ("a claim-eligible change with a nil pointer on success").

- [ ] **Step 4: Run to verify pass**, same command. Expected: PASS.

- [ ] **Step 5: Mutation check.** Back up the file, delete the in-progress early return, confirm with `/usr/bin/grep -cF 'StatusInProgress' internal/app/implementation_context.go` that the count dropped, re-run Step 2's command → `ServesInProgressChange` must FAIL; restore with `mv -f` from the backup and re-run → PASS. Second mutation: force `FeatureBranch` to always mint → the recorded-branch assertion must FAIL; restore.

- [ ] **Step 6: Real-git integration test.** Add `TestIntegrationWorkflowRepoContextServesClaimedChange` to `workflow_integration_test.go` (prefix `TestIntegrationWorkflowRepo` so its existing integration shard runs it). Scaffold like `TestIntegrationWorkflowRepoRefreshClaimAppliesWhenBoardReRenderIsUnchanged` (`planRepoModes`, `buildReadyChange`, `fixtureSpecFile`, `planningDepsFor(t, cloneOrigin(...))`): claim via `ChangeClaim` at the pre-claim context's revision, then from a fresh clone call `ContextImplementation(..., ImplementationContextRequest{ID: id})` and assert `Result == ResultApplied`, `ClaimEligible == false`, `ClaimRefusal == "not-ready-not-proposed"`, `Change.Revision == blobRevisionAt(t, repo.origin, m.branch, recPath)` (the bundle revision is exactly the record blob a `git rev-parse HEAD:<record path>` would print), and `Workflow.FeatureBranch` equals the `branch:` value now on origin.
Run: `go test -tags integration -count=1 ./internal/app/ -run '^TestIntegrationWorkflowRepoContextServesClaimedChange$' -v` → PASS.

- [ ] **Step 7: Docs.** In `docs/reference/glossary.md` *Implementation context*, say the bundle is read before claiming and, by id, re-read for the run's own in-progress change (resume), where it reports `claim_eligible: false`.

- [ ] **Step 8: Commit**
```bash
git add internal/app/implementation_context.go internal/app/implementation_context_test.go internal/app/workflow_integration_test.go docs/reference/glossary.md
git commit -m "feat(context): serve the run's own in-progress change by id"
```

---

### Task 2: `docket schema <id>` accepts the operation id as a positional argument

**Files:**
- Modify: `internal/cli/schema.go` (`newSchemaCommand`)
- Test: `internal/cli/schema_command_test.go`
- Modify: `docs/reference/cli.md` (the `docket schema` bullet)

**Interfaces:**
- Produces: `Use: "schema [<id>]"`, `Args: cobra.MaximumNArgs(1)`. The catalog signature (built by `buildSignature` from the Use tail) becomes `[<id>] [--operation <id>]`.

- [ ] **Step 1: Write the failing test** `TestSchemaCommandPositionalOperation`:
```go
a, _, ca := runCLI(t, "schema", "change.create", "--json")
b, _, cb := runCLI(t, "schema", "--operation", "change.create", "--json")
// ca == 0 && cb == 0 && a == b
_, _, cboth := runCLI(t, "schema", "change.create", "--operation", "change.create", "--json") // agree → 0, same bytes as a
out, _, cmis := runCLI(t, "schema", "change.create", "--operation", "change.claim", "--json")
// cmis == 2; out contains `"result":"invalid-input"` and `"reason":"invalid-arguments"`
_, _, ctwo := runCLI(t, "schema", "change.create", "change.claim", "--json") // ctwo == 2
bad, _, cbad := runCLI(t, "schema", "no.such.operation", "--json") // cbad == 2, finding code unknown-operation (same as --operation)
```
Also assert the production catalog entry's signature: `entryByID(collectCapabilities(productionRootForTest(t)), "schema").Signature == "[<id>] [--operation <id>]"`.

- [ ] **Step 2: Run to verify it fails**
Run: `go test -count=1 ./internal/cli/ -run 'TestSchema' -v` → the new test FAILS (unknown positional under `cobra.NoArgs`).

- [ ] **Step 3: Implement** in `newSchemaCommand`: read `args[0]` when present; when `--operation` is also non-empty and differs, `return fmt.Errorf("schema: positional id %q and --operation %q disagree", ...)` (the root maps a returned error to `invalid-input` / `invalid-arguments`, exit 2); otherwise use whichever is set. Update `Short`/flag usage text if needed so help names both forms.

- [ ] **Step 4: Run to verify pass**, same command → PASS. Mutation: make the mismatch branch pick the positional silently → the mismatch assertion must FAIL; restore.

- [ ] **Step 5: Docs.** `docs/reference/cli.md`: `docket schema [<id>]` — one id positionally or via `--operation`.

- [ ] **Step 6: Commit**
```bash
git add internal/cli/schema.go internal/cli/schema_command_test.go docs/reference/cli.md
git commit -m "feat(schema): accept the operation id as a positional argument"
```

---

### Task 3: Drop the dead `skills` field from `repository.prepare`

**Files:**
- Modify: `internal/app/repository_prepare.go` (delete `PrepareSkills`, the `Skills` field on `PrepareContext`, the constructor's `Skills: PrepareSkills{}` and its comment, and the `resolved workflow skill bindings → PrepareContext.Skills` line in the consumer-inventory comment)
- Test: `internal/cli/repository_test.go` (`TestRepositoryPrepareHumanSummaryRedacted`), and a JSON assertion

**Interfaces:** none consumed or produced; the schema descriptor follows `RepositoryPrepareResult` through its single registry row automatically. Do not touch `internal/config/decode.go`'s `skills` refusal.

- [ ] **Step 1: Write the failing test.** Add `TestRepositoryPrepareJSONHasNoSkillsKey` to `internal/cli/repository_test.go`: stub `repositoryPrepareRunner` to return an applied result with a populated `PrepareContext{RepoRoot: "/repo"}`, run `runCLI(t, "repository", "prepare", "--json")`, decode `context` into `map[string]json.RawMessage`, assert key `"skills"` is absent **and** key `"repo_root"` is present (non-vacuity companion through the same decode). Also assert via `app.SchemaFor("repository.prepare", ...)` that no result field descriptor has key `skills` (and that `metadata_worktree_path` is present).

- [ ] **Step 2: Run** `go test -count=1 ./internal/cli/ -run 'TestRepositoryPrepare' -v` → new test FAILS (`skills` present).

- [ ] **Step 3: Implement** the deletions. In `TestRepositoryPrepareHumanSummaryRedacted`, drop the `Skills:` line and the skills-map assertion (its subject is gone); keep the origin-credential redaction assertion and update the doc comment.

- [ ] **Step 4: Run** `go test -count=1 ./internal/cli/ ./internal/app/ -run 'TestRepositoryPrepare|TestSchema' -v` → PASS; `go build ./...` clean.

- [ ] **Step 5: Commit**
```bash
git add internal/app/repository_prepare.go internal/cli/repository_test.go
git commit -m "refactor(prepare): drop the always-empty skills field"
```

---

### Task 4: implement-next revision lookups, re-syncs, and claim-lease prose

Edits `skills/docket-implement-next/SKILL.md`, `references/results-artifact.md`, `references/fix-pass.md`, `references/edge-paths.md` (resume prose only — the PR-body move is Task 5), and `skills/docket-convention/SKILL.md`. Spec sections 1, 2 (skill side), 3 (skill side), 4.

**Files:**
- Modify: the five skill files above
- Modify: `internal/repoguard/prose_contracts_test.go` (new `change_0360_revision_lookup` rows in `proseContracts`), `internal/repoguard/budgets_test.go`
- Regenerate: `internal/assets/embedded/tree/**`

**Interfaces:**
- Consumes: Task 1's behavior (`context.implementation --id` on an in-progress change returns the bundle with `claim_eligible: false`).
- Produces: the SKILL.md phrases Task 4's guard rows pin (below) — Task 5 must not reword them.

- [ ] **Step 1: Write the failing guard rows** in `proseContracts` (`internal/repoguard/prose_contracts_test.go`), sentinel `change_0360_revision_lookup`. The detector is a raw `strings.Contains`, so each phrase must sit on one physical line in the skill.
  - `skills/docket-implement-next/SKILL.md` present: ``"R=$(git -C <metadata_worktree_path> rev-parse HEAD:<record path>)"``, ``"is the commit id, never a `--revision` value"``, ``"re-sync once (the `repository.prepare` operation), look the revision up again, and retry the write once"``, `"Typed reads fetch on their own"`, ``"reads `claim_eligible: false`"``. Absent: `"change.refresh-claim"`, `"returns the new record revision"`, `"local tip == remote tip"`, `"re-sync the metadata worktree immediately before reading it"`, `"the claim refresh, plan attach, and every earlier checkpoint move the record"`, ``"the `claimed_at` re-stamp"``, ``"`claimed_at` refreshed"``, `"re-stamps `claimed_at: <UTC ISO-8601 now>` on every mutation"`.
  - `references/results-artifact.md` present: `"rev-parse HEAD:<record path>"`; absent: `"change.refresh-claim"`.
  - `references/fix-pass.md` present: `"the reason recorded"`; absent: ``"`claimed_at` refreshed"``.
  - `skills/docket-convention/SKILL.md` present: `"stamped by claim, reconcile, and resume-halted"`; absent: `"refreshed at phase boundaries"`.

- [ ] **Step 2: Run** `go test -count=1 ./internal/repoguard/ -run 'TestProseContracts$' -v` → FAILS on every new row.

- [ ] **Step 3: Before deleting each sentence**, `git grep -nF '<sentence fragment>' -- internal tests` and relocate any dependent assert (the 0517 row's `"the gate command from the build configuration"`, the 0448 rows, and the 0407 rows must keep matching — do not reword those phrases).

- [ ] **Step 4: Edit `skills/docket-implement-next/SKILL.md`:**
  - **Revision rule (spec 1).** Add one paragraph (natural home: Step 2, after the claim paragraph) stating: before each post-claim write to its own record — `change.reconcile`, `change.halt`, a reconcile-kill's `change.kill`, `workspace.prepare`, the revision handed to the plan-writer, the producing-change revision handed to `docket-adr`, each `change.attach-results`, `change.mark-implemented` — the run reads the revision from the local metadata checkout in the same shell call (or immediately before the dispatch carrying it): `R=$(git -C <metadata_worktree_path> rev-parse HEAD:<record path>)`, `<record path>` being the bundle's `change.path`, which does not move while the change is active. Every receipt's `committed_revision` is the commit id, never a `--revision` value. On `contended`: re-sync once (the `repository.prepare` operation), look the revision up again, and retry the write once — except `change.reconcile`, which keeps its re-read-and-re-author rule; a second `contended` follows the step's existing stop rule. The claim uses the pre-claim bundle's revision; `change.resume-halted` uses the context bundle's.
  - Claim paragraph: replace "It returns the new record revision, feature branch, lease classification" with wording naming the feature branch, lease classification, and disposition (no revision claim).
  - **Named path / resume (spec 2).** In Step 0's named-invocation paragraph and Step 1's bundle paragraph, state that for the run's own `in-progress` change (a named resume) `context.implementation --id` returns the bundle with `claim_eligible: false`, the recorded branch, and a readable `halt.run_halted`; a plain named run that reads `claim_eligible: false` surfaces the refusal and ends `drained` exactly as before — only a resume proceeds.
  - Step 2 resume sentence and Step 4/5 halt phrases: drop "refreshes the claim" only where it describes `change.refresh-claim`; keep it for `change.resume-halted` (still stamps). Replace "stays `in-progress` with `claimed_at` refreshed and the halt reason recorded" (Step 4 dispatch fallback, Step 5) with "stays `in-progress` with the halt reason recorded".
  - Step 3: the reconcile sentence "sets `reconciled: true`, refreshes the claim" may stay (reconcile still stamps).
  - **Re-syncs (spec 4).** Delete Step 4's SHA-compare opening (the `rev-parse HEAD` vs tracking-ref loop) — an `applied` receipt proves the exact-lease push landed; delete "(**re-sync the metadata worktree immediately before reading it**)"; delete both `change.refresh-claim` sentences in Step 4 and the Step 5 opener; replace the Step 4 *Continue* "re-read its revision (the `context.implementation` operation …) and refresh the claim again" with the local lookup; delete "after a re-sync" from the build-worker plan read, the Step 6.5 results read (mechanics item 2), and attach item 4 (which now uses the local lookup). Keep the re-sync after the plan-writer returns (*Verification (parent)*) and add the same one re-sync after `docket-adr` returns (Task 5 wires the ADR payload). Add one sentence where the re-syncs used to be: `repository.prepare` runs at startup, after a dispatched agent writes the change, once after a `contended` write, and on a contended claim — Typed reads fetch on their own, and plain-file reads of the metadata checkout follow every docket write, so no other re-sync is needed.
  - Postconditions: row 2 "**and landed** (local tip == remote tip)" → "**and landed** (an `applied` claim receipt)"; row 4 drop "Step 3's push SHA-confirmed **before** the branch is cut; then"; the paragraph under the table becomes: rows 3, 4, 6 and 7 carry the field-write rule's link-bearing condition (the regenerated `## Artifacts` block in the same commit) — no `claimed_at` re-stamp claim.
  - **Field-write rule.** Replace "the transaction re-stamps `claimed_at: <UTC ISO-8601 now>` on every mutation and, for any …" with: for any link-bearing field … regenerates the `## Artifacts` block and inline board in the same commit; `claimed_at` is stamped only by the claim, reconcile, and resume-halted transactions. Delete "the skill also refreshes the claim explicitly … where a stopped run is otherwise invisible."
- [ ] **Step 5: Edit the references and convention:**
  - `results-artifact.md`: delete the "**Refresh the claim** …" paragraph; in §6 step 1, pin the `change.complete` revision with the local lookup (`git -C <metadata_worktree_path> rev-parse HEAD:<record path>`) instead of "pinned from the `status` operation's `--json` output after a re-sync" (keep `path` from the bundle).
  - `fix-pass.md`: the three "with `claimed_at` refreshed and the reason recorded" phrases → "with the reason recorded".
  - `edge-paths.md` *Resume of an `in-progress` change*: the `change.resume-halted` description keeps "refreshes the claim" (true); in *The results seam*, drop "after a re-sync" from the load sentence only if the surrounding sentence still reads correctly.
  - `docket-convention/SKILL.md` manifest: `claimed_at: # UTC ISO-8601 claim lease (YYYY-MM-DDTHH:MM:SSZ); stamped by claim, reconcile, and resume-halted; cleared on leaving in-progress`.
- [ ] **Step 6: Regenerate and run**
```bash
go generate ./internal/assets/
go test -count=1 ./internal/repoguard/ ./internal/assets/ -v -run 'TestProseContracts|TestAlignmentContracts|TestSkillSizeBudgets|TestRetiredVocabularySeal|TestCapabilitySurface|TestPrivateWritingRule'
```
Expected: the new rows PASS. `TestSkillSizeBudgets` reports the edited files' new sizes. Set each edited file's row in `skillBudgets` to its new `wc -l`/`wc -w` (lowering where it shrank), appending `0360: <what changed> (old/old -> new/new); ` at the front of that row's comment. If a file grew, compress before raising; a raise needs its reason in the comment. Re-run → PASS.
- [ ] **Step 7: Mutation-test the rows.** For two absence rows (e.g. re-insert `change.refresh-claim` into SKILL.md; re-insert `refreshed at phase boundaries` into the convention) and two presence rows (delete the `rev-parse HEAD:<record path>` phrase; delete "Typed reads fetch on their own"): back up, mutate, confirm the count moved with `/usr/bin/grep -cF`, run `go test -count=1 ./internal/repoguard/ -run 'TestProseContracts$'` → FAIL naming the row; restore with `mv -f`; re-run → PASS. (Mutate the source `skills/` file; the guard reads the source tree.)
- [ ] **Step 8: Commit**
```bash
git add skills/docket-implement-next skills/docket-convention/SKILL.md internal/assets/embedded internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go
git commit -m "docs(implement-next): look revisions up locally, drop claim refreshes and redundant re-syncs"
```

---

### Task 5: implement-next Step 3/6/7 wording, ADR producing change, PR-body move

Spec sections 5, 6, 6a, 7, and the PR-body leftover of 8.

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Steps 3, 6, 7), `skills/docket-implement-next/references/edge-paths.md`, `skills/docket-adr/SKILL.md`
- Modify: `internal/repoguard/prose_contracts_test.go` (sentinel `change_0360_step_wording`), `internal/repoguard/budgets_test.go`, `internal/repoguard/private_writing_rule_test.go`
- Regenerate: `internal/assets/embedded/tree/**`

**Interfaces:**
- Consumes: Task 4's revision-lookup paragraph (refer to it; do not restate the command) and its re-sync sentence. Do not reword any phrase Task 4's rows pin.
- Produces: none for later tasks.

- [ ] **Step 1: Write the failing guard rows**, sentinel `change_0360_step_wording` (one physical line per phrase):
  - SKILL.md present: ``"the operation writes the `### YYYY-MM-DD` heading"``, ``"Unchanged against `<integration tip>`"``, ``"the receipt's `block` field"``, ``"passes the producing change as `{id, path, revision}`"``, `"one at a time, each with a fresh lookup"`, ``"`git diff --shortstat <effective base>...HEAD`"``, `"**Best-effort PR→issue reference.**"`, `"**PR-body back-link.**"`. Absent: ``"the `evidence.verify` operation"``, `"repeat the record → verify chain"`, ``"record that number in the change's `adrs:` relation through a `change.reconcile`"``, `"git diff --shortstat origin/<integration_branch>...HEAD"`, ``"Before assembling it, **read `references/edge-paths.md` now (blocking)**"``.
  - `references/edge-paths.md` absent: `"## PR-body assembly (Step 7)"`, `"Step 7's PR-body assembly"` (the intro wraps before "Step 7", so match only the line-local fragment; confirm with `/usr/bin/grep -cF` that it matches the unedited file once before trusting the row).
  - `skills/docket-adr/SKILL.md` present: ``"has already appended the number to that change's `adrs:`"``.
- [ ] **Step 2: Run** `go test -count=1 ./internal/repoguard/ -run 'TestProseContracts$' -v` → new rows FAIL.
- [ ] **Step 3: Edit SKILL.md.**
  - **Step 3 (spec 5):** when nothing changed (spec, stack base, related changes, and scope all hold), the request is `{id, revision, reconcile_log_entry}` with a one-line body such as "Unchanged against `<integration tip>`"; owned sections, spec sections, and relations are included only when reality requires a change. The `reconcile_log_entry` is the **section body only** — the operation writes the `### YYYY-MM-DD` heading (mirror the halt prose's "section body only" phrasing).
  - **Step 6 evidence (spec 6):** `evidence.record`'s receipt carries the evidence as the receipt's `block` field; write that text to a file — that file is the `--evidence` input for `pr.publish` and `change.mark-implemented`, which each re-verify it. Delete the `evidence.verify` sentence. "repeat the record → verify chain before publishing" → "repeat the record step before publishing". Keep *Validate the build evidence* and the phrase "the gate command from the build configuration" exactly.
  - **Step 6 ADRs (spec 6a):** each `docket-adr` dispatch payload passes the producing change as `{id, path, revision}` (revision from the Task 4 lookup; path from the bundle); `docket-adr` passes it through as the request's `change`, so `adr.record` appends the number to `adrs:` and re-renders `## Artifacts` in its own commit. Delete the "After re-syncing … record that number in the change's `adrs:` relation through a `change.reconcile` …" sentence. After the dispatch returns: re-sync once, then check from git that the returned number is in `adrs:`. A `contended` from `adr.record`: re-sync, look the revision up again, re-dispatch once, then Step 6's existing stop rule. Several ADRs are dispatched one at a time, each with a fresh lookup.
  - **Step 6 review size (spec 7):** the 1500-line modifier reads from `git diff --shortstat <effective base>...HEAD`, the effective base being the bundle's `effective_base` — `origin/<integration_branch>` for an ordinary change, the parent's branch for a stacked one (the base `workspace.prepare` cut from).
  - **Step 7 (spec 8):** move the four paragraphs of edge-paths' *PR-body assembly (Step 7)* (`**Best-effort PR→issue reference.**`, `**PR-body back-link.**`, `**Review outcome.**`, `**Plan/results links.**`) into Step 7's *PR-body assembly*, merging with the existing summary so nothing is said twice; keep every private-repository-writing-rule clause (the rule must still be named in SKILL.md — it already is); drop the "Before assembling it, read `references/edge-paths.md` now (blocking)" sentence. Byte-compare: each moved rule sentence appears in SKILL.md with its rationale (learning `skill-extraction-and-stub-pointer`).
- [ ] **Step 4: Edit edge-paths.md:** delete the `## PR-body assembly (Step 7)` section and the intro's "or Step 7's PR-body assembly" item (re-punctuate the list).
- [ ] **Step 5: Edit `skills/docket-adr/SKILL.md`:** the `change` bullet says `docket-implement-next` step 6 supplies it in its dispatch payload and you pass it through unchanged; *Return the number* says that when `change` was supplied the transaction has already appended the number to that change's `adrs:` — the caller only verifies it.
- [ ] **Step 6: Private-writing-rule guard.** Edge-paths no longer hands work to a writer of shipped text, so remove `"skills/docket-implement-next/references/edge-paths.md"` from `privateRuleWriters` in `internal/repoguard/private_writing_rule_test.go` and lower the floor `< 10` → `< 9` with a comment: the PR-body rules (and their private-rule clauses) moved into `docket-implement-next/SKILL.md`, which is still a writer row (change 0360). Confirm `TestPrivateWritingRuleReachesEveryWriter` still checks SKILL.md.
- [ ] **Step 7: Regenerate, run, budgets** — same commands as Task 4 Step 6, plus `TestPrivateWritingRule`. Update the `skillBudgets` rows for SKILL.md and edge-paths.md (`0360: …` comment prefix; edge-paths shrinks, so lower it). If SKILL.md's word count rises above its Task-4 ceiling, compress Step 7 first; raise only with a stated reason.
- [ ] **Step 8: Mutation-test** two absence rows (re-insert `the `evidence.verify` operation`; re-insert the edge-paths heading) and two presence rows (delete "one at a time, each with a fresh lookup"; delete the `<effective base>` diff phrase), as in Task 4 Step 7. Also mutate the private-rule guard: delete the private-rule phrase from SKILL.md → `TestPrivateWritingRuleReachesEveryWriter` FAILS; restore.
- [ ] **Step 9: Commit**
```bash
git add skills/docket-implement-next skills/docket-adr/SKILL.md internal/assets/embedded internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go internal/repoguard/private_writing_rule_test.go
git commit -m "docs(implement-next): name the evidence block, pass the producing change to docket-adr, inline PR-body assembly"
```

---

### Task 6: Delete the `change.refresh-claim` operation

Runs after Tasks 4–5, so no skill names the operation when it disappears.

**Files:**
- Modify: `internal/app/change_claim.go` — delete `OperationChangeRefreshClaim`, `ChangeRefreshClaim`, the `refresh` field and its branch in `changeClaimOp.Plan` (and the "claim refreshed" subject), and refresh wording in the file header, `ChangeClaimRequest`, `changeClaimOp`, and the board comment (`includeBoard` stays).
- Modify: `internal/app/schema_registry.go` (delete the `change.refresh-claim` row), `internal/cli/change.go` (delete `refreshClaim` and its `AddCommand` entry), `internal/cli/install.go` (delete `"change refresh-claim"` from `assetIndependent`).
- Modify tests: `internal/cli/change_test.go` (registration lists and the `--run-context` sub-test's refresh half; the op-key table row), `internal/cli/revision_rename_test.go` (both refresh rows), `internal/app/change_claim_test.go` (delete `baseRefreshOp`, `TestChangeRefreshClaimSkipsUnchangedBoard`, the empty refresh section markers; `TestClaimResultFromOutcomeFailedCarriesCause` uses `OperationChangeClaim`), `internal/app/change_integration_test.go` (delete `TestIntegrationChangeAuthoringRefreshClaimStampsOnly`), `internal/app/change_claim_integration_test.go` (delete `TestIntegrationRecordOpsChangeRefreshClaimUnrelatedInvalidRecordRefusals` and the refresh tail of the 0449 claim test), `internal/app/workflow_integration_test.go` (delete `TestIntegrationWorkflowRepoRefreshClaimAppliesWhenBoardReRenderIsUnchanged`), `internal/app/claim_workflow_git_test.go` (delete the 0335 marker and `planningDepsForClock` if nothing else uses it).
- Comments only: `internal/domain/actions.go` (`RefreshClaim` doc names its callers: `change.reconcile` and `change.resume-halted`; keep the function and its tests), `internal/app/change_halt.go` / `change_reconcile.go` / `derived_views.go` / `derived_views_test.go` / `runtracker_ownership_integration_test.go` comments that say "claim refresh" meaning the deleted operation, `internal/repoguard/retired_vocabulary_test.go` (the "Longest first" comment's example: replace `change.refresh-claim` with a surviving operation id that contains a shorter one, or state the rule without an example).
- Docs: `docs/reference/glossary.md` — delete the `refresh-claim` snippet line; the *Claim lease* prose says the lease is stamped at claim, reconcile, and resume; `claim_dispositions` row reported by `change claim` only.

**Interfaces:** Consumes nothing new. `domain.RefreshClaim` keeps its signature.

**Decision recorded here:** do **not** add a `retiredVocabulary` row. That table is the ADR-0129 rename ledger (every hit prints "ADR-0129 row …"); a deleted operation is not an ADR-0129 rename. Absence is pinned by Step 1's tests and Task 4's skill rows.

- [ ] **Step 1: Write the failing absence test** `TestChangeRefreshClaimRetired` in `internal/cli/change_test.go`:
```go
root := productionRootForTest(t)
if c, _, err := root.Find([]string{"change", "refresh-claim"}); err == nil && c.Name() == "refresh-claim" { t.Fatal("change refresh-claim is still registered") }
entries, _ := collectCapabilities(root)
if _, ok := entryByID(entries, "change.refresh-claim"); ok { t.Fatal("catalog still lists change.refresh-claim") }
if _, ok := entryByID(entries, "change.claim"); !ok { t.Fatal("non-vacuity: change.claim missing from catalog") }
if _, ok, _ := app.SchemaFor("change.refresh-claim", nil); ok { t.Fatal("schema registry still binds change.refresh-claim") }
```
(Use the real `entryByID`/`SchemaFor` signatures; pass the effects slice the existing tests pass.) Also `runCLI(t, "change", "refresh-claim", "--id", "1", "--revision", "x", "--json")` exits 2.
- [ ] **Step 2: Run** `go test -count=1 ./internal/cli/ -run 'TestChangeRefreshClaimRetired' -v` → FAIL.
- [ ] **Step 3: Delete** everything in *Files*. Then whole-repo `git grep -n -E 'refresh-claim|RefreshClaim|refresh_claim|claim refresh'` and sort hits: maintained Go/skills/docs must be gone or be `domain.RefreshClaim` (and its domain action label) or a comment describing reconcile/resume stamping; `docs/changes/archive`, `docs/superpowers`, `docs/release`, ADRs, and `testdata/**` tars stay.
- [ ] **Step 4: Run**
```bash
go build ./... && go vet ./...
go test -count=1 ./internal/cli/ ./internal/app/ ./internal/domain/ ./internal/repoguard/
go test -tags integration -count=1 ./internal/app/ -run '^TestIntegration(ChangeAuthoring|RecordOps|WorkflowRepo)'
```
Expected: PASS. Confirm `includeBoard`'s declare-only-when-changed behavior still has a test elsewhere (`git grep -n includeBoard -- '*_test.go'` or a board-unchanged case in another op's tests); if none remains, retarget the deleted `TestChangeRefreshClaimSkipsUnchangedBoard` at `includeBoard` directly rather than dropping the coverage (learning `test-premise-deleted-not-regated`).
- [ ] **Step 5: Mutation:** re-add the `refreshClaim` `AddCommand` entry (backup first) → `TestChangeRefreshClaimRetired` FAILS (and `TestAssetIndependentSetExact` fails on the missing map key); restore.
- [ ] **Step 6: Commit**
```bash
git add -u internal docs/reference/glossary.md
git commit -m "refactor(change): delete change.refresh-claim; claim, reconcile, and resume-halted stamp the lease"
```

---

### Final verification (part of the build gate, not a separate task)

- Whole suite through the configured build gate (`build.test_command` = `go run ./cmd/docket development test`), read the budget report even when green.
- `git grep -n -E 'refresh-claim|returns the new record revision|evidence\.verify' -- skills agents` → no instruction remains (the `evidence.verify` operation itself and its docs stay).
- The ADR decisions the spec lists under *Decisions worth an ADR* are recorded by the parent through Step 6's `docket-adr` path, not by a build task.
- Results-file note for the human: the new skill wording only takes effect after a reinstall and a fresh session (learning `generated-artifact-loaded-at-process-start`); the post-merge transcript tally (spec *Proof*) is the runtime check.
