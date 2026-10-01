<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0482 — Finish 0469's leftover "repair" (relink) and "Step 0" (startup check) wording](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-01-0482-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r.md)**
<!-- docket:backlink:end -->
# Finish 0469's leftover "repair" (relink) and "Step 0" (startup check) wording Implementation Plan

> **For agentic workers:** Execute with `docket-build`. It routes each task to the tier named on that task, runs the docket-build-task contract (focused check, edit, verify, self-review, one commit) and runs the single whole-suite gate at the end. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish ADR-0129 rows 80 and 84. Change "repair" to "relink" wherever it names the `change.relink` operation, and "Step-0" / "Step 0" to "startup check" wherever it names the shared `repository.prepare` startup check. Every other meaning stays.

**Architecture:** This change only edits words in Go string literals, Go comments, test wording, test fixtures, one test-closure name and one shell test header. No wire token, flag name, schema key, operation id or behavior changes. Each task first re-derives its sites from a fresh `git grep`, sorts each hit by the spec's sense rule, edits only the hits that name the target concept, then proves the result with a closing grep and a focused compile and test run.

**Tech Stack:** Go (cobra CLI, `internal/app`, `internal/cli`, `internal/repoguard`), bash shard headers under `tests/`.

**Spec:** `docs/superpowers/specs/2026-10-01-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r-design.md` (on the `docket` metadata branch; read it at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-01-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r-design.md`). Its *Sites to change* tables give the grooming-time inventory, and its *Kept* section lists every meaning that must not change.

## Global Constraints

- **Sense rule, "repair" → "relink":** change a site only where "repair" names the `change.relink` operation, its transaction, its workspace gate, or its result.
- **Sense rule, "Step-0" / "Step 0" → "startup check":** change a site only where it names the shared startup check (the `repository.prepare` operation, its contract, its context values, or its config export). Use "startup-check" (hyphenated) when it is a modifier, as in "startup-check operation" or "startup-check contract".
- **Re-derive, never hand-list (AGENTS.md *Guards and tests*).** The spec's tables are a grooming-time inventory. Every task starts from a fresh `git grep` and sorts each hit by sense. If a hit is not in the spec's inventory and not in its *Kept* list, apply the sense rule to it and say so in the commit body.
- **Use `git grep` or `command grep` for verification.** The interactive `grep` here is ugrep (learning `agent-shell-noop-reads-as-success`), and a grep that matches nothing looks the same as a clean result. Each closing grep in this plan states the exact hits it should return.
- **Kept, never edit:**
  - finalize's post-rebase repair, `docket-integration-repair`, and the `repair-needs-signoff` wire token (e.g. `internal/cli/revision_rename_test.go`'s `finalize block` row)
  - build's repair-and-rerun
  - the board / ADR-index "repair notice" (`writeRepairNotice`, `ADRIndexWithRepair`, and the "repair notice" / "repair entries" comments in `internal/app/derived_views.go`)
  - `reposetup.PlanRepairs`, `--repair-frontmatter`, `finalizeCleanupBacklinkRepair`
  - `status.go`'s "which repair can work" (generic "fix")
  - `change_integration_test.go`'s "not repairs"
  - the retirement guards in `internal/repoguard/retired_vocabulary_test.go` and `TestChangeRelinkRetiresRepairIdentity` in `internal/cli/change_test.go`
  - "identity" in its which-record sense
  - implement-next's own "Step 0" step label and every reference to it (`maintenance_preflight.go`, `internal/cli/maintenance.go`, `budgets_test.go`'s "Step 0 gained the named-invocation branch", every `prose_contracts_test.go` assert and comment)
  - the runbook's "Phase 2 step 0"
  - all frozen records: `testdata`, `docs/results`, `docs/superpowers`, `docs/changes`, `docs/adrs`
- **No test asserts any leftover string** (whole-repo grep at grooming, re-confirmed at planning: each user-visible string occurs only at its definition site). If a task's focused run reddens an assert that greps one of these strings, stop and report it. Do not restore the old wording to keep a grep green (learning `restatement-accumulates-its-own-guards`).
- **No ADR edit, no new guard, no skill or agent-wrapper edit.**
- **Commits:** stage explicit paths only, never `git add -A`.

## Review Focus

1. **Over-reach into a kept "repair" sense.** `internal/app/derived_views.go` has eight "repair" hits, and only the mutation-site list ("reclaim, repair, closeout") names relink. The other seven name the board/ADR-index repair notice. A blanket replace in that file would be wrong. Task 1's closing grep pins the expected remaining count.
2. **Over-reach into the `repair-needs-signoff` wire token** in `internal/cli/revision_rename_test.go`. It is a finalize block reason, not relink wording. Task 2's closing grep requires it to survive.
3. **Over-reach into implement-next's "Step 0" step label.** `maintenance_preflight.go`, `internal/cli/maintenance.go`, `budgets_test.go`'s "Step 0 gained the named-invocation branch" and `prose_contracts_test.go` must stay byte-identical. Task 3's closing grep lists exactly which hits must remain.
4. **The `skill_handoff_sites_test.go` `non_vacuity` fixtures** feed `classifyHandoffSite`. The edited fixture strings must still classify the same way. Task 3 runs that subtest by name.
5. **Partial user-visible help:** the `--id` flag help must take the sibling form ``change `id` to relink (required)``, keeping the backticked `` `id` `` placeholder that cobra renders as the flag's value name. Task 1 checks the rendered `change relink --help`.

---

### Task 1: Production "repair" → "relink" (user-visible strings and operation-naming comments)

**Tier:** standard. It touches user-visible messages, and `derived_views.go` needs the sense discrimination described in Review Focus 1.

**Files:**
- Modify: `internal/app/change_relink.go`: the stale-evidence refusal after the `ExpectRevision` check, the workspace-conflict message in the workspace-gate helper, and the `transaction.DispositionContended` result message
- Modify: `internal/cli/change.go`: the `--id` flag in `newRelinkSubcommand`
- Modify: `internal/app/derived_views.go`: only the mutation-site list in the declare-only-when-changed comment
- Modify: `internal/app/status.go`: only the named-callers comment ("workspace, merge, clear-block, repair, retarget")

**Interfaces:**
- Consumes: nothing.
- Produces: nothing. These are strings and comments only, and no identifier changes.

- [ ] **Step 1: Re-derive the sites**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r
git grep -n -i 'repair' -- internal/app/change_relink.go internal/cli/change.go internal/app/derived_views.go internal/app/status.go
```
Expected at planning time: 3 hits in `change_relink.go` (all change), 1 in `cli/change.go` (change), 8 in `derived_views.go` (only the "kill, lifecycle, mark-implemented, reclaim, repair, closeout" comment changes; the other 7 are the repair notice and `render.ADRIndexWithRepair`, which are kept), and 2 in `status.go` (the "clear-block, repair, retarget" comment changes; "which repair can work" is kept). Sort any new hit by the sense rule.

- [ ] **Step 2: Edit the user-visible strings**

In `internal/app/change_relink.go`:
- `"the change record moved since the approved revision; re-read authoritative context before repairing"` → `"the change record moved since the approved revision; re-read authoritative context before relinking"`
- `"an owned workspace targets %q, not the proposed branch %q; the repair would orphan it"` → `"an owned workspace targets %q, not the proposed branch %q; the relink would orphan it"`
- `"the change record moved during the repair transaction; re-read authoritative context"` → `"the change record moved during the relink transaction; re-read authoritative context"`

In `internal/cli/change.go`, `newRelinkSubcommand`:
```go
cmd.Flags().Int("id", 0, "change `id` to relink (required)")
```
(replacing ``"change `id` whose recorded identity to repair (required)"``)

- [ ] **Step 3: Edit the operation-naming comments**

- `internal/app/derived_views.go`: `kill, lifecycle, mark-implemented, reclaim, repair, closeout` → `kill, lifecycle, mark-implemented, reclaim, relink, closeout`. Change nothing else in this file.
- `internal/app/status.go`: `workspace, merge, clear-block, repair, retarget` → `workspace, merge, clear-block, relink, retarget`. Leave "which repair can work" alone.

Run `gofmt -l internal/app internal/cli`. Expected: no output. If a comment re-wrap is needed, keep the surrounding lines' wrap width.

- [ ] **Step 4: Closing grep, build, focused tests**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r
git grep -n -i 'repair' -- internal/app/change_relink.go internal/cli/change.go
git grep -n -i 'repair' -- internal/app/derived_views.go | wc -l
git grep -n -i 'repair' -- internal/app/status.go
go build ./... && go vet ./internal/app/ ./internal/cli/
go test ./internal/app/ -run 'Relink' -count=1
go test ./internal/cli/ -run 'Relink|Change' -count=1
go run ./cmd/docket change relink --help
```
Expected:
- the first grep prints nothing
- the `derived_views.go` count is `7`
- `status.go` shows only "which repair can work"
- build, vet and both test runs pass
- the `--help` output shows `--id id   change id to relink (required)`

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r
git add internal/app/change_relink.go internal/cli/change.go internal/app/derived_views.go internal/app/status.go
git commit -m "refactor(relink): say relink, not repair, in relink messages, flag help and comments (change 0482, ADR-0129 row 84)"
```

---

### Task 2: Test-only "repair" → "relink" wording

**Tier:** economy. These are mechanical wording edits in test comments, messages and panics, plus one closure rename, and the sense is fixed per file.

**Files:**
- Modify: `internal/app/change_relink_test.go` (21 sites: 13 `"<Method>: repair must not call this"` panics, plus `fake FinalizeGitHub for repair`, `ViewPullRequest the repair reads`, `panics: repair`, `a refused repair opened %d transactions`, `the repair refuses as`, `the repair op's Plan closure`, `the repair envelope`, `an AdoptPRHead repair`)
- Modify: `internal/app/change_relink_integration_test.go` (2 sites: `repaired record on origin does not carry the adopted branch`)
- Modify: `internal/app/change_integration_test.go` (12 sites in `TestIntegrationChangeRuntimeRelinkAdoptPRHeadPinsExactRevision`, `TestIntegrationChangeRuntimeRelinkAbsentWorkspaceNoConflict` and the relink workspace-conflict test after it)
- Modify: `internal/app/named_branch_facts_test.go` (3 sites: the section divider above `TestRelinkWorkspaceClearProbesOnlyOwnStack` and its two failure messages)
- Modify: `internal/app/derived_views_guard_test.go` (1 site: the CHANGE-PATH shape comment "reconcile, repair, halt, resume-halted")
- Modify: `internal/cli/revision_rename_test.go` (the `repair := func(value string)` closure and its two call sites)

**Interfaces:**
- Consumes: nothing from Task 1. Nothing here asserts on Task 1's strings.
- Produces: nothing.

- [ ] **Step 1: Re-derive the sites**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r
git grep -n -i 'repair' -- internal/app/change_relink_test.go internal/app/change_relink_integration_test.go internal/app/change_integration_test.go internal/app/named_branch_facts_test.go internal/app/derived_views_guard_test.go internal/cli/revision_rename_test.go
```
Expected at planning time:
- every hit in `change_relink_test.go`, `change_relink_integration_test.go`, `named_branch_facts_test.go` and `derived_views_guard_test.go` names relink, so all change
- `change_integration_test.go` has 12 relink hits in the three relink tests, plus one kept hit: "the historical corruption this change prevents, not repairs"
- `revision_rename_test.go` has the closure and its 2 call sites (change), plus the kept `"repair-needs-signoff"` wire token in the `finalize block` row

- [ ] **Step 2: Edit `change_relink_test.go` and `change_relink_integration_test.go`**

- Every `panic("<Method>: repair must not call this")` → `panic("<Method>: relink must not call this")`, for all 13 methods, keeping each method name.
- `// --- fake FinalizeGitHub for repair ---…` → `// --- fake FinalizeGitHub for relink ---…`. "relink" and "repair" are the same length, so the divider width does not change.
- `ViewPullRequest the repair reads` → `ViewPullRequest the relink reads`
- `Every non-Inspect method panics: repair` → `Every non-Inspect method panics: relink`
- `"a refused repair opened %d transactions, want 0"` → `"a refused relink opened %d transactions, want 0"`
- `the repair refuses as` → `the relink refuses as`
- `the repair op's Plan closure` → `the relink op's Plan closure`
- `the repair envelope` → `the relink envelope`
- `an AdoptPRHead repair` → `an AdoptPRHead relink`
- `change_relink_integration_test.go` (both sites): `"repaired record on origin does not carry the adopted branch:\n%s"` → `"relinked record on origin does not carry the adopted branch:\n%s"`

- [ ] **Step 3: Edit `change_integration_test.go`, `named_branch_facts_test.go`, `derived_views_guard_test.go`**

`change_integration_test.go`:
- `keying the repair op` → `keying the relink op`
- `t.Fatalf("repair = (%q, %q)", …)` → `t.Fatalf("relink = (%q, %q)", …)`
- `t.Fatalf("repair = (%q, %q) msg=%q findings=%v", …)` → `t.Fatalf("relink = (%q, %q) msg=%q findings=%v", …)`
- `change 0368's repair` → `change 0368's relink`
- `the repair proceeds` → `the relink proceeds`
- `the repair opens` → `the relink opens`
- `"record vanished after repair"` → `"record vanished after relink"`
- `"repaired origin record missing %q:\n%s"` → `"relinked origin record missing %q:\n%s"`
- `survived the repair` → `survived the relink`
- `blocks the repair` → `blocks the relink`
- `while the repair` → `while the relink`
- `lets the repair proceed` → `lets the relink proceed`
- Leave "not repairs" alone.

`named_branch_facts_test.go`:
- `// --- repair workspace ownership gate (fake reader + fake workspace) ------` → `// --- relink workspace ownership gate (fake reader + fake workspace) ------`
- `"repair workspace gate beside an unprobeable unrelated stack refused: %s: %s"` → `"relink workspace gate beside an unprobeable unrelated stack refused: %s: %s"`
- `"repair workspace gate with B's own parent unprobeable passed; want the fail-closed conflict"` → `"relink workspace gate with B's own parent unprobeable passed; want the fail-closed conflict"`

`derived_views_guard_test.go`: `reconcile, repair, halt, resume-halted` → `reconcile, relink, halt, resume-halted`

- [ ] **Step 4: Rename the closure in `revision_rename_test.go`**

```go
	// relink validates its own request shape before any read.
	relink := func(value string) string {
		out, _, _ := runCLI(t, "change", "relink", "--id", "1", "--expect-revision", value,
			"--adopt-pr-head", "--expect-pr", "1", "--expect-head", "b", "--repo-dir", dir, "--json")
		return out
	}
	if out := relink(""); !strings.Contains(out, "expect-revision must be") {
		t.Fatalf("relink control: an empty --expect-revision did not reach validation: %s", out)
	}
	if out := relink(rev); strings.Contains(out, "expect-revision must be") {
		t.Errorf("relink: --expect-revision %s never reached the request: %s", rev, out)
	}
```
Leave the `"repair-needs-signoff"` argument in the `finalize block` row unchanged.

- [ ] **Step 5: Closing grep, compile integration-tagged tests, focused tests**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r
git grep -n -i 'repair' -- internal/app/change_relink_test.go internal/app/change_relink_integration_test.go internal/app/change_integration_test.go internal/app/named_branch_facts_test.go internal/app/derived_views_guard_test.go internal/cli/revision_rename_test.go
gofmt -l internal/app internal/cli
go vet ./internal/app/ ./internal/cli/
go vet -tags integration ./internal/app/
go test ./internal/app/ -run 'Relink|DerivedViews' -count=1
go test ./internal/cli/ -run 'Revision' -count=1
```
Expected:
- the grep prints exactly two lines: `change_integration_test.go`'s "not repairs" comment and `revision_rename_test.go`'s `"repair-needs-signoff"` row
- `gofmt -l` prints nothing
- both vets pass (the integration-tagged vet compiles the edited `//go:build integration` files)
- both test runs pass

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r
git add internal/app/change_relink_test.go internal/app/change_relink_integration_test.go internal/app/change_integration_test.go internal/app/named_branch_facts_test.go internal/app/derived_views_guard_test.go internal/cli/revision_rename_test.go
git commit -m "test(relink): say relink, not repair, in relink test wording (change 0482, ADR-0129 row 84)"
```

---

### Task 3: "Step-0" / "Step 0" → "startup check" for the shared startup check

**Tier:** standard. It touches one user-visible help string, a guard's non-vacuity fixtures, and a sense split against implement-next's own "Step 0" label, which shares the same spelling.

**Files:**
- Modify: `internal/cli/repository.go`: the `prepare` subcommand's short help
- Modify: `internal/app/repository_prepare.go`: the file header comment, the dropped-transport list entries for `DOCKET_GI_*` and `DOCKET_DISPATCH_RETENTION_DAYS`, the `RunRepositoryPrepare` doc comment, and the `prepareGatherFailure` comment
- Modify: `internal/app/configrefusal_integration_test.go`: the `TestIntegrationRepoPrepareInvalidConfigDiagnostics` doc comment
- Modify: `internal/app/repoprepare_integration_test.go`: header comment
- Modify: `internal/repoguard/config_read_channel_test.go`: header comment
- Modify: `internal/repoguard/budgets_test.go`: only the change-0394 re-baseline note ("the new Step-0 capability bootstrap")
- Modify: `internal/repoguard/skill_handoff_sites_test.go`: the `non_vacuity` fixtures `unmarked`, `mention`, `braced`
- Modify: `tests/test_go_integration_app_repoprepare.sh`: header comment

**Interfaces:**
- Consumes: nothing.
- Produces: nothing.

- [ ] **Step 1: Re-derive and sort the sites**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r
git grep -n -E 'Step[- ]0' -- ':!docs/results' ':!docs/superpowers' ':!docs/changes' ':!docs/adrs' ':!**/testdata/**'
```
Sort every hit:
- **Change (startup check):** every `Step-0` hit in `repository_prepare.go`, `configrefusal_integration_test.go`, `repoprepare_integration_test.go`, `config_read_channel_test.go`, `budgets_test.go`'s "new Step-0 capability bootstrap", `skill_handoff_sites_test.go`'s three fixtures and `tests/test_go_integration_app_repoprepare.sh`, plus `repository.go`'s `(Step 0)`.
- **Keep (implement-next's step label or other numbering):** `maintenance_preflight.go` ("docket-implement-next's Step 0"), `internal/cli/maintenance.go` ("docket-implement-next's Step 0"), `budgets_test.go` ("Step 0 gained the named-invocation branch"), every `prose_contracts_test.go` hit, every skill or reference markdown hit naming implement-next's `### Step 0 — Sync & sweep`, and the runbook's "Phase 2 step 0".

A hit that fits neither list gets the sense rule: change it only if it names `repository.prepare`, its contract, its context values or its config export. Name it in the commit body.

- [ ] **Step 2: Edit the user-visible help and production comments**

`internal/cli/repository.go`:
```go
		"Prepare the repository for a workflow: pin topology and attach or fast-forward the .docket worktree (the startup check)",
```

`internal/app/repository_prepare.go`:
- `the sole shared Step-0` (file header; the sentence continues on the next line) → `the sole shared startup-check`
- `not a Step-0 context value` (the `DOCKET_GI_*` entry; "Step-0" may lead its comment line) → `not a startup-check context value`
- `not a Step-0 value` (the `DOCKET_DISPATCH_RETENTION_DAYS` entry) → `not a startup-check value`
- `RunRepositoryPrepare is the sole shared Step-0 operation` → `RunRepositoryPrepare is the sole shared startup-check operation`
- `Step-0 contract promises` (in `prepareGatherFailure`) → `startup-check contract promises`

Re-wrap a comment line only if it now exceeds the surrounding lines' width, and keep the `//` indentation of list continuation lines.

- [ ] **Step 3: Edit test comments, the shell header and the guard fixtures**

- `internal/app/configrefusal_integration_test.go`: `Step-0 contract promises` → `startup-check contract promises`
- `internal/app/repoprepare_integration_test.go`: ``the shared Step-0 `repository prepare` service`` → ``the shared startup-check `repository prepare` service``
- `internal/repoguard/config_read_channel_test.go`: `the config resolver / Step-0 export` → `the config resolver / startup-check export`
- `internal/repoguard/budgets_test.go`: `the new Step-0` (followed by "capability bootstrap") → `the new startup-check`. Leave "Step 0 gained the named-invocation branch" alone.
- `tests/test_go_integration_app_repoprepare.sh`: ``# the shared Step-0 `repository prepare` service scenarios`` → ``# the shared startup-check `repository prepare` service scenarios``
- `internal/repoguard/skill_handoff_sites_test.go`, `non_vacuity`:
```go
		const unmarked = "Run the **resolved plan skill** — `$SKILL_PLAN` from the startup-check config export."
		const mention = "Resolve nothing new: `$SKILL_PLAN`, `$SKILL_BUILD`, learnings enablement come from the startup-check export."
		const braced = "Run the **resolved plan skill** — `${SKILL_PLAN}` from the startup-check config export."
```

- [ ] **Step 4: Closing grep, compile, focused tests**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r
git grep -n -F 'Step-0' -- ':!docs/results' ':!docs/superpowers' ':!docs/changes' ':!docs/adrs' ':!**/testdata/**'
git grep -n -F 'Step 0' -- '*.go' '*.sh'
gofmt -l internal/app internal/cli internal/repoguard
go build ./... && go vet ./internal/app/ ./internal/cli/ ./internal/repoguard/
go vet -tags integration ./internal/app/
go test ./internal/repoguard/ -run 'SkillHandoff|ConfigReadChannel|Budget' -count=1 -v 2>&1 | tail -40
go test ./internal/app/ -run 'RepositoryPrepare|RepoPrepare' -count=1
go run ./cmd/docket repository --help
```
Expected:
- the `Step-0` grep prints nothing
- the `Step 0` Go/sh grep prints only `maintenance_preflight.go`, `internal/cli/maintenance.go`, `budgets_test.go`'s "Step 0 gained the named-invocation branch" and the `prose_contracts_test.go` lines, all naming implement-next's step
- `gofmt -l` prints nothing
- build and both vets pass
- the repoguard run passes, and its `-v` output shows the `non_vacuity` subtest of the skill-handoff test as `PASS`
- the app run passes
- the `repository --help` listing shows `prepare` with "… the .docket worktree (the startup check)"

If `go test -run` matches no test for a pattern (output `testing: warning: no tests to run`), find the real test names with `git grep -n '^func Test' internal/repoguard/skill_handoff_sites_test.go internal/repoguard/config_read_channel_test.go` and rerun with those names. A run that matched nothing has proved nothing.

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r
git add internal/cli/repository.go internal/app/repository_prepare.go internal/app/configrefusal_integration_test.go internal/app/repoprepare_integration_test.go internal/repoguard/config_read_channel_test.go internal/repoguard/budgets_test.go internal/repoguard/skill_handoff_sites_test.go tests/test_go_integration_app_repoprepare.sh
git commit -m "refactor(prepare): say startup check, not Step 0, for the shared repository.prepare check (change 0482, ADR-0129 row 80)"
```

---

## After the tasks

`docket-build` runs the whole-suite gate (`build.test_command` from `.docket.yml`) once after Task 3, so this plan has no gate task. Record the spec's *Testing* closing greps in the results file, using the outputs of the closing-grep steps in Tasks 1-3:
- `repair` across the changed files hits only the *Kept* sites
- `Step-0` hits nothing in maintained source
- `Step 0` hits only implement-next's step-label references and the runbook's own numbering
