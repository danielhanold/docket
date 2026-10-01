<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0480 — Report killed changes truthfully in finalize cleanup](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0480-check-finalize-cleanup-s-not-final-message-for-killed-change.md)**
<!-- docket:backlink:end -->
# Report Killed Changes Truthfully in Finalize Cleanup — Implementation Plan

> **For agentic workers:** Execution is via `docket-build` (tier workers running the
> `docket-build-task` contract), one task per worker, one commit per task, then the single
> full-suite gate. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `docket finalize cleanup --id <killed id>` reports `no-op` / `retained` /
`killed-retained` with a true message instead of `invalid-state` / `not-final`, and the
close-out kill-path docs (and their embedded copies) describe that outcome instead of promising
pruning.

**Architecture:** One new `case domain.StatusKilled:` in `FinalizeCleanup`'s status switch
(`internal/app/finalize_cleanup.go`), mirroring the existing `domain.StatusStackedMerged`
retention case but with its own stable reason constant. Two skill-reference prose sites are
corrected, guarded by new `proseContracts` rows, and the embedded asset bundle is regenerated
with the repo's own generator (`go generate ./internal/assets/` → `cmd/genassets`).

**Tech Stack:** Go (`internal/app`, `internal/repoguard`, `internal/assets`), Markdown skill
references, real-Git integration fixtures (`//go:build integration`).

**Spec:** `docs/superpowers/specs/2026-10-01-check-finalize-cleanup-s-not-final-message-for-killed-change-design.md`
(on the `docket` metadata branch).

## Global Constraints

- New reason constant: `ReasonCleanupKilledRetained = "killed-retained"` — exact wire spelling.
- Killed case result: `ResultNoOp`, `Disposition: CleanupDispRetained`, `Reason: ReasonCleanupKilledRetained`.
- The killed case does NOT consult aborted-rebase scratch; it returns before the `default` branch.
- The `default` branch, `ReasonCleanupNotFinal`, and the `stacked-merged` case are unchanged
  (including the stacked-merged case's existing `Reason: ReasonCleanupNotFinal` — out of scope).
- Do not implement killed-change cleanup (that is change 0483). No other lifecycle message rewording.
- Embedded copies under `internal/assets/embedded/tree/skills/…` must be byte-identical to the
  edited sources; regenerate with `go generate ./internal/assets/` (never hand-copy), which also
  rewrites `internal/assets/embedded/manifest.json` (sizes + sha256). Verify with
  `go run ./cmd/genassets -check`.
- Skill budgets (`internal/repoguard/budgets_test.go`, words = `strings.Fields`):
  `docket-implement-next/references/edge-paths.md` ceiling is **118 lines / 1554 words** and it is
  currently **118 / 1550** — the edit there must add no line and at most 4 words.
  `docket-convention/references/close-out.md` ceiling 240 / 2150, currently 151 / 1467.
- Skill bodies ship into other repos (learning `distributed-body-has-no-local-repo`): no sentence
  that is only true in this repo; the house idiom of citing a docket change number as provenance
  is acceptable.
- Do not introduce new "terminal" lifecycle wording (ADR-0129 retired vocabulary); "final" is the term.
- Full suite: `go run ./cmd/docket development test` (= `build.test_command`).
- Mutation probes always defeat the Go test cache: `-count=1`. Restore mutations from a backup
  copy (`cp f f.bak; …; mv -f f.bak f`), never `git checkout --` on an uncommitted edit.

## Review Focus

1. **A killed change with a prepared workspace and published feature branch** (the reconcile-kill
   case) — cleanup must leave the worktree directory, local ref, and remote ref all in place.
   Pinned by Task 1's integration test asserting all three survive.
2. **Killed change carrying owned aborted-rebase scratch** — spec says killed never reaches the
   rebase path; the new case returns before `finalizeCleanupAbortedRebase`, so the scratch is
   left alone (consistent with "removes nothing"). No test added: a killed record cannot carry it
   (kills only from `proposed`/`in-progress`); reviewer should confirm the case sits before
   `default`.
3. **Human-readable output** — `CleanupOpResult.HumanText()` for a `no-op` prints
   `finalize.cleanup: change <id> retained`; the integration test asserts the `Message` names the
   killed condition (contains `"killed"`) so a copy-paste of the stacked-merged message is caught.
4. **Docs drift between source and embedded copy** — covered by `TestEmbeddedMatchesAuthored`
   and `go run ./cmd/genassets -check` in Task 2.
5. **Kill callers under abort-and-report** — the docs must state the `no-op` is success so callers
   continue; pinned by Task 2's `proseContracts` row phrase "a success, not a failure".

---

### Task 1: Killed-change retention case in `FinalizeCleanup`

**Tier hint:** standard (small code change, but the integration fixture needs a record moved
from `active/` to `archive/` on the metadata ref).

**Files:**
- Modify: `internal/app/finalize_cleanup.go` (reason const block; `CleanupDispRetained` doc comment; `FinalizeCleanup` switch; file header comment's stacked-merged sentence)
- Modify: `internal/app/finalize_cleanup_test.go` (`TestFinalLifecycleCodeSpellings` row)
- Modify: `internal/app/finalize_cleanup_integration_test.go` (new fixture + `TestIntegrationFinalizeCleanupKilledRetained`)

**Interfaces:**
- Consumes: existing `newCleanupResult`, `CleanupDispRetained`, `ResultNoOp`, `domain.StatusKilled`;
  test helpers `setupCloseoutFixture`, `planRepoModeDocket`, `groomPath`, `padID`,
  `lifecycleChange`, `runGit`, `(*gitRepo).writerAdvance`, `(*closeoutFixture).mergedCleanupFake`,
  `cleanupDeps`, `localBranchPresent`, `remoteBranchPresent`.
- Produces: exported constant `ReasonCleanupKilledRetained = "killed-retained"` (Task 2's docs cite this spelling).

Note on test placement: the spec says "a unit case in `finalize_cleanup_test.go` following the
stacked-merged retention test's pattern". That pattern (`TestIntegrationFinalizeCleanupStackedRetained`)
lives in `finalize_cleanup_integration_test.go` under `//go:build integration` and needs real Git,
and the suite's cleanup shard runs `^TestIntegrationFinalizeCleanup` (`tests/test_go_integration_app_cleanup.sh`).
So the behavioral case goes in the integration file with that prefix; the untagged
`finalize_cleanup_test.go` gets the wire-spelling pin. This is a deliberate placement deviation;
record it in the results file.

- [ ] **Step 1: Add the failing spelling pin (unit)**

In `internal/app/finalize_cleanup_test.go`, `TestFinalLifecycleCodeSpellings`, add a row after the
`ReasonCleanupNotFinal` row:

```go
		{"ReasonCleanupKilledRetained", string(ReasonCleanupKilledRetained), "killed-retained"},
```

- [ ] **Step 2: Add the failing integration case**

Append to `internal/app/finalize_cleanup_integration_test.go`, after
`TestIntegrationFinalizeCleanupStackedRetained`:

```go
// setupKilledCleanupFixture builds a closeout fixture (real prepared workspace
// and published feature branch) and then relocates its record to archive/ as a
// killed change — the state a reconcile-kill of a resumed in-progress change
// leaves behind. A killed record carries no branch or claim stamp (change.kill
// strips them), which lifecycleChange already models for status "killed".
func setupKilledCleanupFixture(t *testing.T) *closeoutFixture {
	t.Helper()
	f := setupCloseoutFixture(t, planRepoModeDocket())
	activePath := groomPath(f.id, f.slug)
	archivePath := "docs/changes/archive/2026-08-16-" + padID(f.id) + "-" + f.slug + ".md"
	// Sync the writer's metadata branch to origin before removing the active
	// record, so the removal commits on top of the published tip.
	runGit(t, f.repo.writer, "fetch", "-q", "origin", f.branch)
	runGit(t, f.repo.writer, "checkout", "-q", f.branch)
	runGit(t, f.repo.writer, "reset", "-q", "--hard", "origin/"+f.branch)
	runGit(t, f.repo.writer, "rm", "-q", activePath)
	f.repo.writerAdvance(t, f.branch, map[string]string{archivePath: lifecycleChange(f.id, f.slug, "killed")})
	f.revision = blobRevisionAt(t, f.repo.origin, f.branch, archivePath)
	return f
}

func TestIntegrationFinalizeCleanupKilledRetained(t *testing.T) {
	requireRealGit(t)
	f := setupKilledCleanupFixture(t)
	gh := f.mergedCleanupFake(f.head, strings.Repeat("d", 40))
	res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
	if res.Result != ResultNoOp || res.Disposition != CleanupDispRetained || res.Reason != ReasonCleanupKilledRetained {
		t.Fatalf("a killed change must be a retained no-op, got result %q disp %q reason %q (%s)",
			res.Result, res.Disposition, res.Reason, res.Message)
	}
	if !strings.Contains(res.Message, "killed") {
		t.Fatalf("the message must name the killed condition, got %q", res.Message)
	}
	if _, err := os.Stat(f.wp); err != nil {
		t.Fatalf("a killed change's workspace must be retained: %v", err)
	}
	if !f.localBranchPresent(t) || !f.remoteBranchPresent(t) {
		t.Fatalf("a killed change must retain its branches")
	}
}
```

Check the file's import block already has `context`, `os`, `strings` (it does for the existing
tests; add any that are missing).

- [ ] **Step 3: Run both to verify they fail**

Run: `go test -count=1 -run '^TestFinalLifecycleCodeSpellings$' ./internal/app/`
Expected: build failure — `undefined: ReasonCleanupKilledRetained`.

Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeCleanupKilledRetained$' ./internal/app/`
Expected: build failure, same undefined symbol. (If you temporarily stub the constant to see the
behavioral red, expect `got result "invalid-state" disp "pending" reason "not-final"`.)

- [ ] **Step 4: Implement**

In `internal/app/finalize_cleanup.go`:

1. In the reason `const` block, add directly after `ReasonCleanupNotFinal`:

```go
	ReasonCleanupKilledRetained   = "killed-retained"
```

(`gofmt` will realign the block.)

2. Replace the `CleanupDispRetained` doc comment with:

```go
	// CleanupDispRetained: the resource is deliberately retained (a stacked-merged
	// change kept until its root closes, a killed change whose workspace and
	// branches finalize cleanup does not remove, or a gate run kept for its
	// diagnostics).
```

3. In `FinalizeCleanup`'s switch, add this case immediately after the `domain.StatusStackedMerged`
case and before `default`:

```go
	case domain.StatusKilled:
		// Final, but never merged: finalize cleanup removes only a merged change's
		// resources, so a killed change's workspace and branches are retained. A
		// kill is reachable only from proposed/in-progress, never from the
		// implemented state where finalize rebases run, so no aborted-rebase
		// scratch is consulted.
		return newCleanupResult(OperationFinalizeCleanup, ResultNoOp, CleanupOpResult{
			ID: id, Disposition: CleanupDispRetained, Reason: ReasonCleanupKilledRetained,
			Message: "change is killed; finalize cleanup removes only a merged change's resources — its workspace and branches are retained",
		})
```

4. In the file header comment (the `// \`finalize cleanup\` runs an ordered suffix…` paragraph),
   insert one sentence immediately after the sentence ending "keeps the cleaned tombstone for
   replay and health attribution." and before "A stacked-merged change retains…":
   `A killed change is final but never merged, so its workspace and branches are retained (a no-op, killed-retained).`
   Re-wrap only that paragraph at the file's existing comment width; change no other words.

- [ ] **Step 5: Run to verify they pass**

Run: `gofmt -l internal/app/` → expect no output.
Run: `go test -count=1 -run '^TestFinalLifecycleCodeSpellings$' ./internal/app/` → PASS.
Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeCleanup' ./internal/app/` → PASS
(the whole cleanup shard, including the existing `OnlyAfterFinal` / `StackedRetained` cases).

- [ ] **Step 6: Mutation check (guards are code)**

```bash
f=internal/app/finalize_cleanup.go
cp "$f" "$f.bak"
# delete the three-statement killed case (the `case domain.StatusKilled:` line through its closing `})`)
```
Remove the case by editing the file (confirm with `grep -c 'case domain.StatusKilled' "$f"` → `0`).
Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeCleanupKilledRetained$' ./internal/app/`
Expected: FAIL with `got result "invalid-state" disp "pending" reason "not-final"`.
Restore: `mv -f "$f.bak" "$f"`, confirm `grep -c 'case domain.StatusKilled' "$f"` → `1`, re-run → PASS.

Record the observed red line in your task report (the build role carries it into results).

- [ ] **Step 7: Real-binary check against a real killed change**

From the feature worktree, after Step 5 is green:
Run: `go run ./cmd/docket finalize cleanup --id 28 --json`
(change 0028 is an archived `killed` change in this repo; argv from the capability catalog:
`docket finalize cleanup --id <id> [--repo-dir <dir>]`.)
Expected JSON: `"result":"no-op"`, `"disposition":"retained"`, `"reason":"killed-retained"`.
This path returns before any destructive leg, so it touches nothing. Record the output for the
results file.

- [ ] **Step 8: Commit**

```bash
git add internal/app/finalize_cleanup.go internal/app/finalize_cleanup_test.go internal/app/finalize_cleanup_integration_test.go
git commit -m "fix(finalize): report a killed change as retained in finalize cleanup (change 0480)"
```

---

### Task 2: Correct the kill-path close-out docs, guard them, regenerate embedded copies

**Tier hint:** standard (prose edits under a tight word budget, plus a mutation-tested sentinel).

**Files:**
- Modify: `skills/docket-convention/references/close-out.md` (step 4, "Clean up the feature branch + worktree")
- Modify: `skills/docket-implement-next/references/edge-paths.md` (`## Reconcile-kill (Step 3, change OBSOLETE)`)
- Modify: `internal/repoguard/prose_contracts_test.go` (two `proseContracts` rows)
- Regenerate: `internal/assets/embedded/tree/skills/docket-convention/references/close-out.md`,
  `internal/assets/embedded/tree/skills/docket-implement-next/references/edge-paths.md`,
  `internal/assets/embedded/manifest.json` (via `go generate ./internal/assets/` only)

**Interfaces:**
- Consumes: the wire spellings `no-op`, `retained`, `killed-retained` from Task 1.
- Produces: nothing code-facing.

- [ ] **Step 1: Re-derive the doc sites from a grep (do not trust this list)**

```bash
grep -rn -i 'kill' skills | grep -i 'clean\|prune' 
```
Expected relevant hits: `close-out.md` step 4 ("so a kill leg whose / branch never merged keeps its
feature ref") and `edge-paths.md` reconcile-kill ("The reference's cleanup step prunes any feature
worktree/branch already"). `docket-new-change/SKILL.md` ("the reference's cleanup step is a no-op
here" for a `proposed`-kill) is already true and stays unchanged. If the grep finds any other
claim that cleanup removes a killed change's resources, correct it the same way and add it to the
sentinel in Step 4.

- [ ] **Step 2: Add the failing sentinel rows**

In `internal/repoguard/prose_contracts_test.go`, append to the `proseContracts` slice (before its
closing `}`):

```go
	// change 0480 — finalize cleanup retains a killed change's resources and
	// reports no-op / retained / killed-retained (FinalizeCleanup's
	// domain.StatusKilled case). The absent phrases are the retired claims that
	// cleanup prunes or processes a killed change's worktree/branch
	// (assert-detects-removal). Each phrase sits on one physical line: this
	// detector is a raw strings.Contains, so a re-wrap that splits a phrase
	// reddens the row rather than passing silently.
	{sentinel: "change_0480_killed_cleanup_retained", file: "skills/docket-convention/references/close-out.md",
		present: []string{
			"reason `killed-retained`",
			"a success, not a failure",
		},
		absent: []string{
			"so a kill leg whose",
		}},
	{sentinel: "change_0480_killed_cleanup_retained", file: "skills/docket-implement-next/references/edge-paths.md",
		present: []string{
			"The cleanup step retains a killed change's worktree/branch (`killed-retained`, a `no-op`).",
		},
		absent: []string{
			"cleanup step prunes any feature worktree",
		}},
```

Run: `go test -count=1 -run '^TestProseContracts$' ./internal/repoguard/`
Expected: FAIL — missing required phrases and both retired phrases present.

- [ ] **Step 3: Edit `close-out.md` step 4**

Replace the paragraph that currently ends

```
   it — never the `.docket/` metadata worktree, the primary tree, or any out-of-tree path. Any
   resource whose ownership it cannot prove is **retained**, not force-removed (so a kill leg whose
   branch never merged keeps its feature ref rather than losing it). A failure aborts per the
   caller's posture.
```

with (keep the preceding lines of the paragraph — "Trust the typed outcome. Ownership is
proven…" through "…no open child PR still targeting" — unchanged):

```
   it — never the `.docket/` metadata worktree, the primary tree, or any out-of-tree path. Any
   resource whose ownership it cannot prove is **retained**, not force-removed. A failure aborts
   per the caller's posture.

   **Kill path:** cleanup removes only a merged `done` change's resources. On a `killed` change it
   removes nothing and returns `no-op` with disposition `retained` and reason `killed-retained` —
   a success, not a failure, so the kill caller continues. Any feature worktree or branch a
   reconcile-killed change already had stays in place; remove it by hand if it is no longer wanted
   (docket change 0483 tracks automatic killed-change cleanup). A `proposed`-kill has none.
```

The `done`-path text is otherwise unchanged. Confirm each `present` phrase is on one line:
`grep -cF -- 'reason \`killed-retained\`' skills/docket-convention/references/close-out.md` → `1`
and `grep -cF -- 'a success, not a failure' …` → `1`.

- [ ] **Step 4: Edit `edge-paths.md` reconcile-kill (line- and word-budget neutral)**

The current two lines

```
kill and is surfaced. The reference's cleanup step prunes any feature worktree/branch already
created. Terminal publication is deferred from Go v1 — the kill archives on `docket` via the `change.kill`
```

become exactly

```
kill and is surfaced. The cleanup step retains a killed change's worktree/branch (`killed-retained`, a `no-op`).
Terminal publication is deferred from Go v1 — the kill archives on `docket` via the `change.kill`
```

(10 words out, 11 in: +1 word, 0 lines.) Verify:
`wc -lw skills/docket-implement-next/references/edge-paths.md` → `118 1551`.
If you reword, stay ≤ 1554 words and 118 lines; exceeding requires a budgets row bump with a
`0480:` comment in the house format — avoid it.

- [ ] **Step 5: Run the sentinel to verify it passes, then mutation-test it**

Run: `go test -count=1 -run '^TestProseContracts$|^TestSkillSizeBudgets$' ./internal/repoguard/` → PASS.

Mutation (each from a backup, restore with `mv -f`):
1. In `edge-paths.md`, put back "The reference's cleanup step prunes any feature worktree/branch already" →
   `TestProseContracts` FAILS naming `cleanup step prunes any feature worktree` present.
2. In `close-out.md`, change `killed-retained` to `not-final` in the new paragraph →
   FAILS naming the missing `reason \`killed-retained\`` phrase.
Confirm each mutation landed with `grep -c` before running; restore and re-run → PASS.

- [ ] **Step 6: Regenerate the embedded copies**

```bash
go generate ./internal/assets/
go run ./cmd/genassets -check
cmp skills/docket-convention/references/close-out.md internal/assets/embedded/tree/skills/docket-convention/references/close-out.md
cmp skills/docket-implement-next/references/edge-paths.md internal/assets/embedded/tree/skills/docket-implement-next/references/edge-paths.md
go test -count=1 ./internal/assets/
```
Expected: `-check` exits 0, both `cmp` silent, `TestEmbeddedMatchesAuthored` PASS.
`git status --porcelain` must show only the two skill sources, their two embedded copies,
`internal/assets/embedded/manifest.json`, and `prose_contracts_test.go` — nothing else.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/references/close-out.md \
  skills/docket-implement-next/references/edge-paths.md \
  internal/assets/embedded/tree/skills/docket-convention/references/close-out.md \
  internal/assets/embedded/tree/skills/docket-implement-next/references/edge-paths.md \
  internal/assets/embedded/manifest.json \
  internal/repoguard/prose_contracts_test.go
git commit -m "docs(close-out): kill-path cleanup retains a killed change's resources (change 0480)"
```

---

### Build gate (docket-build, after both tasks)

Run the whole suite from source: `go run ./cmd/docket development test`. Read the budget report
even when green (`BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines). The cleanup shard
(`tests/test_go_integration_app_cleanup.sh`, prefix `^TestIntegrationFinalizeCleanup`) must
include and pass `TestIntegrationFinalizeCleanupKilledRetained`.

## Spec coverage

| Spec item | Task |
|---|---|
| `case domain.StatusKilled` → `no-op` / `retained` / `killed-retained`, true message | 1 |
| `CleanupDispRetained` doc comment lists killed | 1 |
| `default` / `not-final` / stacked-merged unchanged; no scratch consult | 1 (Global Constraints) |
| Behavioral test case (touches no workspace or ref) | 1 (integration file — placement deviation noted) |
| `TestFinalLifecycleCodeSpellings` pin | 1 |
| Mutation: remove the case → reddens to `invalid-state` / `not-final` | 1 Step 6 |
| `close-out.md` step 4 kill-path text; remove "kill leg" sentence | 2 |
| `edge-paths.md` reconcile-kill sentence | 2 |
| Grep-derived other sites | 2 Step 1 |
| Embedded copies byte-identical via repo mechanism | 2 Step 6 |
| `finalize cleanup --id <killed> --json` returns `no-op`/`retained`/`killed-retained` | 1 Step 7 |
| Full suite green | Build gate |
