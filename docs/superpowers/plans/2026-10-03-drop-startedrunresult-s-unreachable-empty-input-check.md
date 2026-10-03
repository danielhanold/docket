<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0495 — Drop startedRunResult's unreachable empty-input check](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-03-0495-drop-startedrunresult-s-unreachable-empty-input-check.md)**
<!-- docket:backlink:end -->
# Drop startedRunResult's Unreachable Empty-Input Check Implementation Plan

> **For agentic workers:** Execute with the `docket-build` skill (docket's build role): each task goes to a tier agent running the docket-build-task contract, followed by one full-suite gate at the end. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the dead `key == "" || runContext == ""` branch from `startedRunResult`, along with the two test assertions that keep it alive. Point the two-token guarantee's comments at the callers' pre-mint `scope-failed` refusal.

**Architecture:** This is a pure refactor in `internal/app`. Both production callers of `startedRunResult` (`RunStart` step 7 and `armResumeReplacement`) refuse an empty child capability as `run-untracked scope-failed` before any mint, and they take the key from a successful `MintRunTrackerRecord`. That leaves the helper's guard unreachable. The test edits come first so the suite stays green throughout. The guard and comment changes follow, after a mutation probe confirms that the real refusal is covered.

**Tech Stack:** Go (`internal/app`), and Go tests behind the `integration` build tag.

**Spec:** `docs/superpowers/specs/2026-10-03-drop-startedrunresult-s-unreachable-empty-input-check-design.md` (on the `docket` branch)

## Global Constraints

- No other change to `run.start` behavior, its refusal reasons, or the `run-started` report format.
- Do not touch the refusal ordering that 0491 settled. The `grant.ChildCapability == ""` refusals in `RunStart` and `armResumeReplacement` stay exactly as they are.
- `ReasonRunMintFailed` stays. The real mint and bind failures still return it.
- Leave the step-7 comment in `RunStart` (the one beginning "(7) Report the started run") as it is.
- Cross-references in comments anchor on symbol names, never line numbers (ADR-0054).
- Do not move the renamed test out of its integration-tagged file. Do not edit 0491's merged plan or results file.
- Add no new test. `TestIntegrationRunStartEmptyRunContextMintsNothing` already covers the pre-mint refusal.
- Every Go run that observes a change in outcome uses `-count=1`, so a cached result cannot pass for a real one (learning `cached-runner-serves-a-mutated-tree`).

## Review Focus

- **The real empty-run-context refusal on all three scope-preparing paths** (fresh start, no-run-record resume, cancelled-replacement resume). Expected: `run-untracked scope-failed`, nothing minted. Coverage is the existing `TestIntegrationRunStartEmptyRunContextMintsNothing`. Task 2 Step 1 mutation-probes it against both refusal sites before the guard goes.
- **The started result's full field set** (`Started`, `Result`, `Key`, `RunContext`, `Target`, `OwnerLifecycle`). Expected: unchanged. The renamed test is the only one that pins `Target`, so Task 1 keeps that assertion intact.
- **The two-token `run-started <key> <run-context>` line.** Expected: unchanged. `TestRunStartResultCarriesNoRunID` keeps pinning it.
- **A stale test name that claims a refusal it no longer exercises.** Expected: none left. Task 1 renames the test and rewrites its comment (learning `test-premise-deleted-not-regated`).
- **Mutation restore.** Expected: the probe never destroys uncommitted work. Task 2 restores from a `cp` backup, never `git checkout --` (learning `mutation-restore-needs-a-backup-copy`).

---

### Task 1: Drop the blank-input test assertions and rename the started-fields test

**Files:**
- Modify: `internal/app/runtracker_start_resume_integration_test.go` (the `TestIntegrationRunStartStartedResultRequiresKeyAndContext` function and its comment)
- Modify: `internal/app/runtracker_start_result_json_test.go` (the final assertion in `TestRunStartResultCarriesNoRunID`)

**Interfaces:**
- Consumes: `startedRunResult(key, runContext string) RunStartResult`, `runStartStoredTarget`, `ReasonOwnerLifecycleUnavailable`, `ResultApplied` (all existing, all unchanged).
- Produces: a test renamed to `TestIntegrationRunStartStartedResultFields`. Nothing else depends on it.

This task only removes assertions. With the guard still in place the tests stay green, so there is no red phase. The "failing test" for this change is the mutation probe in Task 2.

- [ ] **Step 1: Replace the integration test**

In `internal/app/runtracker_start_resume_integration_test.go`, replace the whole function and its comment, which currently read:

```go
// TestIntegrationRunStartStartedResultRequiresKeyAndContext (changes 0463, 0491): the
// started constructor refuses to start without a key or a run context. That
// guarantee is what makes the positional two-token line unambiguous.
func TestIntegrationRunStartStartedResultRequiresKeyAndContext(t *testing.T) {
	for _, tc := range [][2]string{{"", "ctx"}, {"k", ""}} {
		if got := startedRunResult(tc[0], tc[1]); got.Started || got.Reason != ReasonRunMintFailed || got.Key != "" || got.RunContext != "" {
			t.Fatalf("startedRunResult(%q, %q) must fail closed as run-untracked mint-failed, got %+v", tc[0], tc[1], got)
		}
	}
	got := startedRunResult("k", "ctx")
	if !got.Started || got.Result != ResultApplied || got.Key != "k" || got.RunContext != "ctx" ||
		got.Target != runStartStoredTarget || got.OwnerLifecycle != ReasonOwnerLifecycleUnavailable {
		t.Fatalf("started result fields wrong: %+v", got)
	}
}
```

with:

```go
// TestIntegrationRunStartStartedResultFields (changes 0463, 0491, 0495): a started
// result carries every field the report and its JSON form need: applied, started,
// the key, the run context, the stored target, and the owner-lifecycle caveat. The
// callers refuse an empty run context before minting (TestIntegrationRunStartEmptyRunContextMintsNothing),
// so the constructor itself has no empty-input branch to test.
func TestIntegrationRunStartStartedResultFields(t *testing.T) {
	got := startedRunResult("k", "ctx")
	if !got.Started || got.Result != ResultApplied || got.Key != "k" || got.RunContext != "ctx" ||
		got.Target != runStartStoredTarget || got.OwnerLifecycle != ReasonOwnerLifecycleUnavailable {
		t.Fatalf("started result fields wrong: %+v", got)
	}
}
```

- [ ] **Step 2: Drop the empty-key assertion from the JSON test**

In `internal/app/runtracker_start_result_json_test.go`, inside `TestRunStartResultCarriesNoRunID`, delete these three lines and nothing else:

```go
	if r := startedRunResult("", "ctx-token"); r.Started || r.Reason != ReasonRunMintFailed {
		t.Errorf("a start with no key must fail closed run-untracked mint-failed, got %+v", r)
	}
```

The test's name and comment ("the run.start result names only the key and the run context, and its started line is two tokens") still describe what it checks, so leave them alone.

- [ ] **Step 3: Confirm no reference to the old name survives in maintained source**

Run: `cd /Users/homer/dev/docket/.worktrees/drop-startedrunresult-s-unreachable-empty-input-check && grep -rn "StartedResultRequiresKeyAndContext" internal cmd tests || echo NONE`
Expected: `NONE`. Mentions under `docs/` are frozen records and stay as they are.

- [ ] **Step 4: Run both tests**

Run: `cd /Users/homer/dev/docket/.worktrees/drop-startedrunresult-s-unreachable-empty-input-check && go test -count=1 -run '^TestRunStartResultCarriesNoRunID$' ./internal/app/ && go test -tags integration -count=1 -run '^TestIntegrationRunStartStartedResultFields$' ./internal/app/`
Expected: both `ok`, and neither reports `(cached)`.

- [ ] **Step 5: Commit**

```bash
git -C /Users/homer/dev/docket/.worktrees/drop-startedrunresult-s-unreachable-empty-input-check add internal/app/runtracker_start_resume_integration_test.go internal/app/runtracker_start_result_json_test.go && git -C /Users/homer/dev/docket/.worktrees/drop-startedrunresult-s-unreachable-empty-input-check commit -m "test(0495): drop startedRunResult's blank-input asserts; rename the started-fields test"
```

### Task 2: Mutation-probe the pre-mint refusal, then delete the unreachable guard

**Files:**
- Modify: `internal/app/runtracker_start.go` (the `startedRunResult` function and its comment, plus the `RunStartResult.HumanText` comment)

**Interfaces:**
- Consumes: Task 1's test edits, already committed, so the guard's removal turns no test red.
- Produces: `func startedRunResult(key, runContext string) RunStartResult`. The signature is unchanged and the function is now a single `return newRunStartResult(...)`.

- [ ] **Step 1: Mutation-probe the covering test at both refusal sites**

`internal/app/runtracker_start.go` has two identical pre-mint refusals, `if serr != nil || grant.ChildCapability == "" {`: one in `armResumeReplacement` (the cancelled-replacement path) and one in `RunStart` step 5 (fresh start and no-run-record resume). Probe each one separately. Back up with `cp` and restore with `mv -f`, never `git checkout --`.

```bash
WT=/Users/homer/dev/docket/.worktrees/drop-startedrunresult-s-unreachable-empty-input-check
F=$WT/internal/app/runtracker_start.go
cp "$F" "$F.bak"
# Mutation A: weaken ONLY the RunStart step-5 site (the second occurrence).
awk 'BEGIN{n=0} /if serr != nil \|\| grant\.ChildCapability == "" \{/ {n++; if (n==2) sub(/ \|\| grant\.ChildCapability == ""/, "")} {print}' "$F.bak" > "$F"
grep -c 'grant.ChildCapability == ""' "$F"   # expect 1 (the mutation landed)
(cd "$WT" && go test -tags integration -count=1 -run '^TestIntegrationRunStartEmptyRunContextMintsNothing$' ./internal/app/)
mv -f "$F.bak" "$F"
```

Expected: the `grep -c` prints `1`, and the test FAILS on the fresh-start and/or no-run-record-resume subcases, either because a run-key directory was minted or because the report is not `scope-failed`.

```bash
cp "$F" "$F.bak"
# Mutation B: weaken ONLY the armResumeReplacement site (the first occurrence).
awk 'BEGIN{n=0} /if serr != nil \|\| grant\.ChildCapability == "" \{/ {n++; if (n==1) sub(/ \|\| grant\.ChildCapability == ""/, "")} {print}' "$F.bak" > "$F"
grep -c 'grant.ChildCapability == ""' "$F"   # expect 1
(cd "$WT" && go test -tags integration -count=1 -run '^TestIntegrationRunStartEmptyRunContextMintsNothing$' ./internal/app/)
mv -f "$F.bak" "$F"
git -C "$WT" diff --stat -- internal/app/runtracker_start.go   # expect empty (restored)
```

Expected: the test FAILS on the cancelled-replacement subcase, and the final `diff --stat` is empty. If either mutation leaves the test green, stop and report BLOCKED. The spec's claim that no coverage is lost would not hold.

- [ ] **Step 2: Delete the guard and rewrite `startedRunResult`'s comment**

In `internal/app/runtracker_start.go`, replace:

```go
// startedRunResult formats the started report for key. Its callers guarantee a
// non-empty key (a successful mint) and run context (an empty one is refused as
// scope-failed right after the scope is prepared, before any mint), so the positional
// `run-started <key> <run-context>` line is always two tokens. The empty-input check
// below is unreachable defense in depth, never the refusal point: by the time a
// caller formats, the records are already minted.
func startedRunResult(key, runContext string) RunStartResult {
	if key == "" || runContext == "" {
		return runUntracked(ReasonRunMintFailed)
	}
	return newRunStartResult(ResultApplied, RunStartResult{
```

with:

```go
// startedRunResult formats the started report for key. Its callers (RunStart and
// armResumeReplacement) pass a key from a successful mint and a run context they
// already checked non-empty before minting: an empty child capability is refused as
// run-untracked scope-failed right after the scope is prepared. So the positional
// `run-started <key> <run-context>` line is always two tokens.
func startedRunResult(key, runContext string) RunStartResult {
	return newRunStartResult(ResultApplied, RunStartResult{
```

The struct literal and closing lines below it stay as they are.

- [ ] **Step 3: Repoint `HumanText`'s comment**

In the same file, replace:

```go
// HumanText renders the one report line. A started run prints `run-started <key>
// <run-context>`. That is always two tokens, because every started result carries
// both (startedRunResult). A run-untracked
// report prints `run-untracked <reason-token>`; a usage error (a non-applied
```

with:

```go
// HumanText renders the one report line. A started run prints `run-started <key>
// <run-context>`. That is always two tokens, because the callers of startedRunResult
// refuse an empty run context as scope-failed before minting, and the key comes from
// a successful mint. A run-untracked
// report prints `run-untracked <reason-token>`; a usage error (a non-applied
```

The rest of that comment block stays as it is.

- [ ] **Step 4: Confirm the guard is gone and the refusals remain**

Run: `cd /Users/homer/dev/docket/.worktrees/drop-startedrunresult-s-unreachable-empty-input-check && grep -n 'defense in depth\|key == "" || runContext == ""' internal/app/runtracker_start.go || echo GONE; grep -c 'grant.ChildCapability == ""' internal/app/runtracker_start.go`
Expected: `GONE`, then `2`.

- [ ] **Step 5: Build, vet, and run the focused tests**

Run: `cd /Users/homer/dev/docket/.worktrees/drop-startedrunresult-s-unreachable-empty-input-check && go build ./... && go vet -tags integration ./internal/app/ && go test -count=1 -run '^TestRunStartResult' ./internal/app/ && go test -tags integration -count=1 -run '^TestIntegrationRunStart' ./internal/app/`
Expected: all `ok`, not `(cached)`.

- [ ] **Step 6: Commit**

```bash
git -C /Users/homer/dev/docket/.worktrees/drop-startedrunresult-s-unreachable-empty-input-check add internal/app/runtracker_start.go && git -C /Users/homer/dev/docket/.worktrees/drop-startedrunresult-s-unreachable-empty-input-check commit -m "refactor(0495): drop startedRunResult's unreachable empty-input check"
```

## Build gate

After both tasks, `docket-build` runs the whole suite through the configured `build.test_command` (`go run ./cmd/docket development test`, entered from source in the feature worktree). Read the budget report even if the run is green.
