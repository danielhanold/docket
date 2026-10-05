<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0525 — Finalize stops on a private repo without the branch-rules API, and leaves half-removed workspaces](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-05-0525-finalize-stops-on-a-private-repo-without-the-branch-rules-ap.md)**
<!-- docket:backlink:end -->
# Finalize merges on a plan without branch rules, and finishes a removal Git started — Implementation Plan

> **For agentic workers:** This plan is executed by `docket-build` task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `finalize.merge` merges on a private repository whose GitHub plan has no branch rules (with a visible `branch-rules-unavailable` note), and `finalize cleanup` finishes a `git worktree remove` that Git started but could not complete (with a `workspace-remnant` note when the folder survives), while a blocked cleanup names what blocked it.

**Architecture:** Three narrow edits to existing finalize code. (1) `probeBranchMergeRules` (`internal/githubcli/mergemethod.go`) recognizes GitHub's exact plan-gate 403 body and reports "rules unavailable", so `MergePullRequest` picks from the repository settings and stamps `MergeResult.BranchRulesUnavailable`; `FinalizeMerge` turns that into one warning finding. (2) `cleanupReady` (`internal/workspace/cleanup.go`) re-lists worktrees after a `KindCommandFailed` removal; when the recorded path is no longer registered it finishes with `os.RemoveAll` and advances the manifest to cleaned, setting `CleanupResult.Remnant` if the delete fails. (3) `finalizeCleanupWorkspace` (`internal/app/finalize_cleanup.go`) carries the remnant as a non-blocking note and appends `BlockedBy` to the `workspace-blocked` message.

**Tech Stack:** Go (stdlib `encoding/json`, `os`, `path/filepath`, `strings`), real-Git integration tests behind the `integration` build tag, fake-`gh` subprocess fixtures, shell wrapper "git" executables for fault injection.

**Spec:** `docs/superpowers/specs/2026-10-05-finalize-stops-on-a-private-repo-without-the-branch-rules-ap-design.md` (on the `docket` branch; synchronized copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-05-finalize-stops-on-a-private-repo-without-the-branch-rules-ap-design.md`). Read it before starting.

## Global Constraints

- Plan-gate match: body decodes as `{"message": string, "status": string}`; `status` is exactly `"403"`; `message` starts with `Upgrade to GitHub ` and ends with ` or make this repository public to enable this feature.` Anything else stays a `KindExternal` failure → `MergeUnknown`.
- On the plan-gate answer the branch set is all three methods (`rebase`, `merge`, `squash` all true); repository probe, selection order, `--admin`, exact-head matching, the authoritative reprobe, and the no-retry-of-a-lower-method rule are unchanged.
- `MergeResult.BranchRulesUnavailable` is set on every outcome reached after the policy probes, including `method-unavailable`.
- Merge finding: code `branch-rules-unavailable`, severity warning, message exactly `GitHub does not offer branch rules for this repository on its plan; the merge method was chosen from the repository settings alone`. A note, never a stop: disposition, result, and method fields unchanged.
- Cleanup: after a `KindCommandFailed` removal, re-list; list error → `failed` error return; recorded path still registered → blocked, byte-untouched, reason `git refused the non-forcing removal`; no longer registered → `os.RemoveAll` on the manifest's recorded path, advance manifest ready→cleaned, return `cleaned`; if `os.RemoveAll` fails, still advance and return `cleaned` with `CleanupResult.Remnant` = that path.
- Remnant finding: code `workspace-remnant`, severity warning, message `Git removed the worktree but part of the folder could not be deleted; delete <path> by hand`. Disposition stays `cleaned`; the local and remote branch legs still run.
- `workspace-blocked` message appends `res.BlockedBy` (already capped at eight by `boundedReasons`), e.g. `the feature workspace is not a clean, ready checkout (.DS_Store); it is retained`.
- No special case for OS files (`.DS_Store`, `Thumbs.db`). Workspaces with uncommitted tracked changes still block. No durable "removing" manifest phase.
- `gate cleanup`'s "never a recursive pathname delete" comment (run logs) stays as it is.
- The ADR ("Cleanup finishes a worktree removal Git already committed to", related to ADR-0035) is recorded by the parent through `docket-adr`, **not** by any task here. Do not author an ADR file.
- Every new assert is mutation-checked: remove the behavior it guards, re-run with `-count=1` (Go's test cache serves stale passes otherwise — learning `cached-runner-serves-a-mutated-tree`), watch it fail, restore from a backup copy (never `git checkout --` over uncommitted work — learning `mutation-restore-needs-a-backup-copy`).
- Integration tests must use the shard prefixes so the suite runs them: `^TestIntegrationMerge` (githubcli), `^TestIntegrationFinalizeMerge` and `^TestIntegrationFinalizeCleanup` (internal/app), `^TestIntegrationWorkspaceLifecycle` (internal/workspace). Never run `./internal/app/` integration tests without `-run`.

## Review Focus

- A plan-gate body whose `status` is the **number** `403` (not the string) must stay unknown — Task 1 pins it.
- An **unrelated stale (prunable) worktree registration** in the repository when Git's removal half-fails must not stop the finish — the 0366 shape must still clean. Task 3 pins it (the registration check keys on the feature ref and on this exact path, never on "every registration resolves").
- A **plan-gated repository whose settings allow nothing** must still report `method-unavailable` and still carry `BranchRulesUnavailable` — Task 1 pins it.
- A **denied merge** on a plan-gated repository must still carry the `branch-rules-unavailable` note with the `denied` disposition unchanged — Task 2 pins it.
- A **remnant** must not turn the cleanup result `pending`: notes are kept out of the retryable-findings list that decides the disposition — Task 4 pins it (the cleaned assert reddens if the note is mixed into `findings`).

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `internal/githubcli/mergemethod.go` | plan-gate recognition in `probeBranchMergeRules` | 1 |
| `internal/githubcli/merge.go` | `MergeResult.BranchRulesUnavailable`; split act block into `issueMerge` | 1 |
| `internal/githubcli/mergemethod_integration_test.go`, `merge_integration_test.go` | probe + merge tests | 1 |
| `internal/app/finalize_merge.go` | `branch-rules-unavailable` finding | 2 |
| `internal/app/finalize_merge_test.go`, `finalize_merge_integration_test.go` | fake field + tests | 2 |
| `internal/workspace/cleanup.go` | finish a started removal; `Remnant`; comment updates | 3 |
| `internal/workspace/cleanup_integration_test.go` | four+one real-Git cleanup tests | 3 |
| `internal/app/finalize_cleanup.go` | remnant note, blocked message, file comment | 4 |
| `internal/app/finalize_cleanup_integration_test.go` | remnant + blocked-names-path tests | 4 |
| `skills/docket-finalize-change/SKILL.md`, `docs/guide/landing-changes.md`, `internal/repoguard/budgets_test.go`, `internal/assets/embedded/**` | prose, budget, embedded bundle | 5 |

Tasks 1→2 and 3→4 are sequential pairs (2 consumes 1's field, 4 consumes 3's field); the two pairs are independent of each other. Task 5 depends on nothing in code but describes behavior from 2 and 4.

---

### Task 1: Recognize GitHub's plan-gate answer in the branch-rules probe

**Files:**
- Modify: `internal/githubcli/mergemethod.go` (`probeBranchMergeRules`, new `isBranchRulesPlanGate`)
- Modify: `internal/githubcli/merge.go` (`MergeResult`, `MergePullRequest`, new `issueMerge`)
- Test: `internal/githubcli/mergemethod_integration_test.go`, `internal/githubcli/merge_integration_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `MergeResult.BranchRulesUnavailable bool` (exported field on `githubcli.MergeResult`); `func (c *Client) probeBranchMergeRules(ctx context.Context, repo Repository, baseBranch string) (methodSet, bool, *Failure)` — the `bool` is "rules unavailable on this plan"; `func isBranchRulesPlanGate(stdout []byte) bool`; `func (c *Client) issueMerge(ctx context.Context, repo Repository, number int, expectedHead ObjectRef, admin bool, method MergeMethod) (MergeResult, error)`.

- [ ] **Step 1: Write the failing probe tests**

In `internal/githubcli/mergemethod_integration_test.go`, first update the three existing callers of `probeBranchMergeRules` to the new three-value return (they will not compile otherwise; that is the expected red):

- `TestIntegrationMergeProbeBranchMergeRules`: `set, unavailable, f := c.probeBranchMergeRules(...)`, and add after the `f != nil` check:
  ```go
  	if unavailable {
  		t.Fatal("a readable rules array must not report rules-unavailable")
  	}
  ```
- `TestIntegrationMergeProbeBranchMergeRulesNoRestriction`: `set, unavailable, f := ...` and change the condition to `if f != nil || unavailable || set != (methodSet{rebase: true, merge: true, squash: true})`.
- `TestIntegrationMergeProbeBranchMergeRulesFailsClosed`: `if _, _, f := c.probeBranchMergeRules(...); f == nil {`.

Then append:

```go
// planGateBody is GitHub's literal answer to a branch-rules read on a private
// repository whose plan has no branch rules (reproduced 2026-10-05 against the
// 0366 fixture repository). `gh api` prints it on stdout and exits 1.
const planGateBody = `{"message":"Upgrade to GitHub Pro or make this repository public to enable this feature.","documentation_url":"https://docs.github.com/rest/repos/rules#get-rules-for-a-branch","status":"403"}`

// TestIntegrationMergeProbeBranchMergeRulesPlanGate: GitHub's plan-gate 403 means
// the branch has no rules — every method is permitted and the probe reports
// rules-unavailable with no failure. The Team wording matches too.
func TestIntegrationMergeProbeBranchMergeRulesPlanGate(t *testing.T) {
	bodies := map[string]string{
		"pro":  planGateBody,
		"team": `{"message":"Upgrade to GitHub Team or make this repository public to enable this feature.","status":"403"}`,
	}
	for name, body := range bodies {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{{
			ArgvPrefix: []string{"api"}, Stdout: body, Exit: 1,
			Stderr: "gh: Upgrade to GitHub Pro or make this repository public to enable this feature. (HTTP 403)",
		}}})
		set, unavailable, f := c.probeBranchMergeRules(context.Background(), Repository{Host: "github.com", Owner: "o", Name: "n"}, "main")
		if f != nil {
			t.Fatalf("%s: plan-gate answer must not be a failure, got %v", name, f)
		}
		if !unavailable {
			t.Fatalf("%s: plan-gate answer must report rules-unavailable", name)
		}
		if set != (methodSet{rebase: true, merge: true, squash: true}) {
			t.Fatalf("%s: plan-gate answer must permit every method, got %+v", name, set)
		}
	}
}

// TestIntegrationMergeProbeBranchMergeRulesPlanGateLookalikesFailClosed: only the
// exact plan-gate answer proceeds. Every lookalike stays a typed failure with an
// empty set and rules-unavailable false.
func TestIntegrationMergeProbeBranchMergeRulesPlanGateLookalikesFailClosed(t *testing.T) {
	cases := map[string]string{
		"other 403 message":       `{"message":"Resource not accessible by integration","status":"403"}`,
		"404 body":                `{"message":"Not Found","documentation_url":"https://docs.github.com/rest","status":"404"}`,
		"non-JSON stdout":         `Upgrade to GitHub Pro or make this repository public to enable this feature.`,
		"plan-gate text, 404":     `{"message":"Upgrade to GitHub Pro or make this repository public to enable this feature.","status":"404"}`,
		"plan-gate text, numeric": `{"message":"Upgrade to GitHub Pro or make this repository public to enable this feature.","status":403}`,
		"plan-gate text, no status": `{"message":"Upgrade to GitHub Pro or make this repository public to enable this feature."}`,
		"empty plan name":         `{"message":"Upgrade to GitHub  or make this repository public to enable this feature.","status":"403"}`,
		"empty stdout":            ``,
	}
	for name, body := range cases {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{{ArgvPrefix: []string{"api"}, Stdout: body, Exit: 1}}})
		set, unavailable, f := c.probeBranchMergeRules(context.Background(), Repository{Host: "github.com", Owner: "o", Name: "n"}, "main")
		if f == nil || unavailable || set != (methodSet{}) {
			t.Errorf("%s: must fail closed, got set=%+v unavailable=%v f=%v", name, set, unavailable, f)
		}
	}
}
```

Note on "empty plan name": the message `"Upgrade to GitHub  or make…"` (two spaces) is exactly prefix + suffix with nothing between them; the length rule in Step 4 rejects it.

- [ ] **Step 2: Write the failing merge-level tests**

In `internal/githubcli/merge_integration_test.go`, in `TestIntegrationMergeSelectsMergeWhenRebaseDisabled` add after the method check:

```go
	if res.BranchRulesUnavailable {
		t.Fatal("a readable rules array must not set BranchRulesUnavailable")
	}
```

Then append:

```go
// TestIntegrationMergePlanGateMergesWithRepoMethods: on a plan without branch
// rules, the merge proceeds with the repository settings' best method and the
// result carries BranchRulesUnavailable.
func TestIntegrationMergePlanGateMergesWithRepoMethods(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{
		Sequential: true,
		Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
			mrgRepoSettingsArm(repoAllTrue, 0),
			mrgBranchRulesArm(planGateBody, 1),
			mrgMergeArm(0),
			mrgViewArm(mergedPR(), 0),
		},
	})
	res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, ObjectRef(ensHeadOid), false)
	if err != nil {
		t.Fatalf("MergePullRequest: %v", err)
	}
	if res.Outcome != MergeMerged || res.Method != MethodRebase {
		t.Fatalf("outcome/method = %q/%q, want merged/rebase", res.Outcome, res.Method)
	}
	if !res.BranchRulesUnavailable {
		t.Fatal("a plan-gated merge must carry BranchRulesUnavailable")
	}
	assertMergeFlag(t, log, "--rebase")
}

// TestIntegrationMergePlanGateSquashOnly: the plan-gate answer contributes no
// restriction, so a squash-only repository still selects squash.
func TestIntegrationMergePlanGateSquashOnly(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{
		Sequential: true,
		Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
			mrgRepoSettingsArm(repoSquashOnly, 0),
			mrgBranchRulesArm(planGateBody, 1),
			mrgMergeArm(0),
			mrgViewArm(mergedPR(), 0),
		},
	})
	res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, ObjectRef(ensHeadOid), false)
	if err != nil {
		t.Fatalf("MergePullRequest: %v", err)
	}
	if res.Outcome != MergeMerged || res.Method != MethodSquash || !res.BranchRulesUnavailable {
		t.Fatalf("got outcome %q method %q unavailable %v, want merged/squash/true", res.Outcome, res.Method, res.BranchRulesUnavailable)
	}
	assertMergeFlag(t, log, "--squash")
}

// TestIntegrationMergePlanGateMethodUnavailableCarriesFlag: a plan-gated
// repository whose settings enable nothing is still method-unavailable, issues
// no merge, and still carries BranchRulesUnavailable.
func TestIntegrationMergePlanGateMethodUnavailableCarriesFlag(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{
		Sequential: true,
		Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
			mrgRepoSettingsArm(repoAllFalse, 0),
			mrgBranchRulesArm(planGateBody, 1),
		},
	})
	res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, ObjectRef(ensHeadOid), false)
	if err != nil {
		t.Fatalf("method-unavailable must be a value outcome, got %v", err)
	}
	if res.Outcome != MergeMethodUnavailable || !res.BranchRulesUnavailable {
		t.Fatalf("got outcome %q unavailable %v, want method-unavailable/true", res.Outcome, res.BranchRulesUnavailable)
	}
	if n := countArgv(log.records(t), "pr", "merge"); n != 0 {
		t.Fatalf("pr merge issued %d times, want 0", n)
	}
}

// TestIntegrationMergePlanGateLookalikeIssuesNoMerge: any other 403 on the
// branch-rules read is still unknown with a diagnostic error and no merge.
func TestIntegrationMergePlanGateLookalikeIssuesNoMerge(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{
		Sequential: true,
		Invocations: []fakeArm{
			mrgViewArm(openPR(ensHeadOid, "MERGEABLE"), 0),
			mrgRepoSettingsArm(repoAllTrue, 0),
			mrgBranchRulesArm(`{"message":"Resource not accessible by integration","status":"403"}`, 1),
		},
	})
	res, err := c.MergePullRequest(context.Background(), mrgRepo(), 7, ObjectRef(ensHeadOid), false)
	if err == nil || res.Outcome != MergeUnknown {
		t.Fatalf("got outcome %q err %v, want unknown with an error", res.Outcome, err)
	}
	if res.BranchRulesUnavailable {
		t.Fatal("a lookalike 403 must not set BranchRulesUnavailable")
	}
	if n := countArgv(log.records(t), "pr", "merge"); n != 0 {
		t.Fatalf("pr merge issued %d times, want 0", n)
	}
}
```

- [ ] **Step 3: Run to verify red**

Run: `go test -tags integration -count=1 -run '^TestIntegrationMerge' ./internal/githubcli/`
Expected: build failure (`assignment mismatch` on `probeBranchMergeRules`, `res.BranchRulesUnavailable undefined`).

- [ ] **Step 4: Implement plan-gate recognition**

In `internal/githubcli/mergemethod.go`, add `"strings"` to the imports, extend the file header comment's last sentence to read `…unobservable or malformed policy fails closed (three-outcome discipline; learnings: probe-error-is-not-clean-absence). The one exception is GitHub's plan-gate answer to the branch-rules read, which means the branch has no rules.`, and add before `probeBranchMergeRules`:

```go
// ghAPIErrorJSON is GitHub's REST error body, which `gh api` prints on stdout
// when a request fails. Pointer fields force presence; a numeric status or a
// missing field never decodes into a match.
type ghAPIErrorJSON struct {
	Message *string `json:"message"`
	Status  *string `json:"status"`
}

// GitHub's plan-gate wording for features a plan does not offer on private
// repositories ("Upgrade to GitHub Pro or make this repository public to
// enable this feature."). The plan name between them varies (Pro, Team).
const (
	planGatePrefix = "Upgrade to GitHub "
	planGateSuffix = " or make this repository public to enable this feature."
)

// isBranchRulesPlanGate reports whether a failed branch-rules read is GitHub's
// plan-gate answer: the body decodes, status is exactly "403", and the message
// is the plan-gate wording with a non-empty plan name. That answer means no
// branch rule exists on this plan, so none can restrict the merge. Anything
// else — another 403, a 404, a 5xx, a non-JSON body — returns false and stays
// unknown. A reworded message also returns false: a missed match fails safe.
func isBranchRulesPlanGate(stdout []byte) bool {
	var body ghAPIErrorJSON
	if err := json.Unmarshal(stdout, &body); err != nil || body.Message == nil || body.Status == nil {
		return false
	}
	if *body.Status != "403" {
		return false
	}
	msg := *body.Message
	return len(msg) > len(planGatePrefix)+len(planGateSuffix) &&
		strings.HasPrefix(msg, planGatePrefix) &&
		strings.HasSuffix(msg, planGateSuffix)
}
```

Note the length rule: `planGatePrefix` is 18 bytes and `planGateSuffix` is 55 bytes (73 together). The "empty plan name" message `Upgrade to GitHub  or make…` (two spaces) is exactly prefix + suffix, 73 bytes, so "strictly longer than prefix+suffix" rejects it while any real plan name (`Pro`, `Team`) passes. The rule is written against `len()` of the constants, so it needs no hand-counted number.

Change `probeBranchMergeRules`'s doc comment to add: `When GitHub answers with its plan-gate 403 (isBranchRulesPlanGate), the branch has no rules: it returns every method and rulesUnavailable=true.` Change its signature and returns:

```go
func (c *Client) probeBranchMergeRules(ctx context.Context, repo Repository, baseBranch string) (methodSet, bool, *Failure) {
	if baseBranch == "" {
		return methodSet{}, false, newFailure(mergeMethodOp, StageValidate, KindInvalidInput, "base branch is empty", nil)
	}
	path := "repos/" + repo.Owner + "/" + repo.Name + "/rules/branches/" + url.PathEscape(baseBranch)
	res, f := c.run(ctx, runRequest{
		op:      mergeMethodOp,
		args:    []string{"api", "--hostname", repo.Host, path},
		network: true,
	})
	if f != nil {
		return methodSet{}, false, f
	}
	if res.exitCode != 0 {
		if isBranchRulesPlanGate(res.stdout) {
			return methodSet{rebase: true, merge: true, squash: true}, true, nil
		}
		return methodSet{}, false, newFailure(mergeMethodOp, StageInvoke, KindExternal,
			"gh api branch rules failed: "+stderrExcerpt(res.stderr), nil)
	}
```

and every later `return methodSet{}, newFailure(...)` becomes `return methodSet{}, false, newFailure(...)`; the final `return permitted, nil` becomes `return permitted, false, nil`.

- [ ] **Step 5: Carry the flag through `MergePullRequest`**

In `internal/githubcli/merge.go`, extend the `MergeResult` doc comment with: `BranchRulesUnavailable is true when GitHub answered the branch-rules read with its plan-gate 403 (no branch rules exist on this repository's plan) and the method was chosen from the repository settings alone; it is set on every outcome reached after the policy probes, method-unavailable included.` Add the field:

```go
type MergeResult struct {
	Outcome                    MergeOutcome
	Method                     MergeMethod
	Facts                      MergedFacts
	RepoMethods, BranchMethods []MergeMethod
	BranchRulesUnavailable     bool
}
```

In `MergePullRequest`, replace from `branchSet, pf := c.probeBranchMergeRules(...)` to the end of the function with:

```go
	branchSet, rulesUnavailable, pf := c.probeBranchMergeRules(ctx, repo, snap.pr.BaseBranch)
	if pf != nil {
		return MergeResult{Outcome: MergeUnknown}, pf
	}
	method, ok := selectMergeMethod(repoSet.intersect(branchSet))
	if !ok {
		return MergeResult{
			Outcome:                MergeMethodUnavailable,
			RepoMethods:            repoSet.list(),
			BranchMethods:          branchSet.list(),
			BranchRulesUnavailable: rulesUnavailable,
		}, nil
	}
	res, err := c.issueMerge(ctx, repo, number, expectedHead, admin, method)
	res.BranchRulesUnavailable = rulesUnavailable
	return res, err
}

// issueMerge is the act half of MergePullRequest: the selected method at the
// exact expected head, never --delete-branch, resolved against a fresh
// authoritative reprobe. The allowed values are guarded — a method outside
// them renders no flag.
func (c *Client) issueMerge(ctx context.Context, repo Repository, number int, expectedHead ObjectRef, admin bool, method MergeMethod) (MergeResult, error) {
	flag := method.mergeFlag()
	if flag == "" {
		return MergeResult{Outcome: MergeUnknown}, newFailure(mergeOp, StageValidate, KindInvalidInput,
			"selected merge method outside the allowed values", nil)
	}
	args := []string{
		"pr", "merge", strconv.Itoa(number),
		"--repo", repo.Spec(),
		flag,
		"--match-head-commit", string(expectedHead),
	}
	if admin {
		args = append(args, "--admin")
	}
	res, mf := c.run(ctx, runRequest{op: mergeOp, args: args, network: true, write: true})
	if mf != nil {
		if mf.Stage == StageLaunch {
			// gh never started; nothing merged. Retain as unknown with no method.
			return MergeResult{Outcome: MergeUnknown}, mf
		}
		// A timeout/cancel may have landed the merge; it is NOT a denial. Verify.
		return c.verifyMerge(ctx, repo, number, expectedHead, false, method)
	}
	// A non-zero exit is a candidate denial; a zero exit is the expected success.
	// Both are resolved against a fresh authoritative reprobe.
	return c.verifyMerge(ctx, repo, number, expectedHead, res.exitCode != 0, method)
}
```

(The body of `issueMerge` is the existing act block moved verbatim; keep its existing comments.) Also update the file header comment's sentence `…an unobservable capability probe is unknown (retain).` to append ` GitHub's plan-gate answer to the branch-rules read means no branch rules exist; the method then comes from the repository settings alone and the result carries BranchRulesUnavailable.`

- [ ] **Step 6: Run to verify green**

Run: `go test -tags integration -count=1 -run '^TestIntegrationMerge' ./internal/githubcli/` and `go test -count=1 ./internal/githubcli/`
Expected: PASS.

- [ ] **Step 7: Mutation-check**

Back up `mergemethod.go` and `merge.go` (`cp f f.bak`), apply each mutation alone, run the Step 6 integration command, confirm red, restore with `mv -f f.bak f`:
1. `isBranchRulesPlanGate` → `return false` at top: PlanGate and the three merge-level PlanGate tests redden.
2. Delete the `*body.Status != "403"` check: "plan-gate text, 404" reddens.
3. Delete the `len(msg) >` clause: "empty plan name" reddens.
4. Delete `res.BranchRulesUnavailable = rulesUnavailable`: MergesWithRepoMethods/SquashOnly redden.
5. Remove `BranchRulesUnavailable: rulesUnavailable,` from the method-unavailable literal: MethodUnavailableCarriesFlag reddens.
6. Replace `rulesUnavailable` with `true` in the stamp: SelectsMergeWhenRebaseDisabled reddens.

- [ ] **Step 8: Commit**

```bash
git add internal/githubcli/mergemethod.go internal/githubcli/merge.go internal/githubcli/mergemethod_integration_test.go internal/githubcli/merge_integration_test.go
git commit -m "fix(githubcli): GitHub's plan-gate 403 on branch rules means no rules, not unknown"
```

---

### Task 2: Report `branch-rules-unavailable` on the finalize.merge result

**Files:**
- Modify: `internal/app/finalize_merge.go` (new constant, `FinalizeMerge` tail, new `mergeOutcomeResult`, new `branchRulesUnavailableFinding`)
- Modify: `internal/app/finalize_merge_test.go` (`fakeMergeGitHub` field)
- Test: `internal/app/finalize_merge_integration_test.go`

**Interfaces:**
- Consumes: `githubcli.MergeResult.BranchRulesUnavailable bool` (Task 1).
- Produces: `const FindingBranchRulesUnavailable = "branch-rules-unavailable"` in package `app`; `func branchRulesUnavailableFinding() StatusFinding`.

- [ ] **Step 1: Extend the fake**

In `internal/app/finalize_merge_test.go`, add to `fakeMergeGitHub` after `mergeBranchMethods`:

```go
	mergeRulesUnavailable bool
```

and in its `MergePullRequest` return literal add `BranchRulesUnavailable: f.mergeRulesUnavailable,`.

- [ ] **Step 2: Write the failing test**

Append to `internal/app/finalize_merge_integration_test.go`:

```go
// countFindingCode counts findings carrying exactly code.
func countFindingCode(fs []StatusFinding, code string) int {
	n := 0
	for _, f := range fs {
		if f.Code == code {
			n++
		}
	}
	return n
}

// TestIntegrationFinalizeMergeBranchRulesUnavailableNote proves a merge reached
// through GitHub's plan-gate answer carries exactly one branch-rules-unavailable
// warning — on a merged result and on a denial alike — and that the note never
// changes the result, disposition, or method. A normally-read rules array
// carries none.
func TestIntegrationFinalizeMergeBranchRulesUnavailableNote(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	assertNote := func(t *testing.T, res FinalizeMergeResult) {
		t.Helper()
		if n := countFindingCode(res.Findings, "branch-rules-unavailable"); n != 1 {
			t.Fatalf("branch-rules-unavailable findings = %d, want 1 (%+v)", n, res.Findings)
		}
		for _, f := range res.Findings {
			if f.Code != "branch-rules-unavailable" {
				continue
			}
			if f.Severity != "warning" {
				t.Fatalf("severity = %q, want warning", f.Severity)
			}
			if f.Message != "GitHub does not offer branch rules for this repository on its plan; the merge method was chosen from the repository settings alone" {
				t.Fatalf("message = %q", f.Message)
			}
		}
	}

	t.Run("merged", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		mergeCommit := f.mergeFeatureIntoBase(t)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeMethod = githubcli.MethodRebase
		gh.mergeFacts = mergedFactsFor(f.head, "main", mergeCommit)
		gh.mergeRulesUnavailable = true
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultApplied || res.Disposition != MergeDispMerged || res.Method != "rebase" || res.Merge == nil {
			t.Fatalf("got result %q disp %q method %q merge %v (reason %q)", res.Result, res.Disposition, res.Method, res.Merge, res.Reason)
		}
		assertNote(t, res)
	})

	t.Run("denied", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeDenied
		gh.mergeMethod = githubcli.MethodRebase
		gh.mergeRulesUnavailable = true
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Disposition != MergeDispDenied || res.Reason != "merge-denied" || res.Method != "rebase" {
			t.Fatalf("got disp %q reason %q method %q, want denied/merge-denied/rebase", res.Disposition, res.Reason, res.Method)
		}
		assertNote(t, res)
	})

	t.Run("rules-read-normally", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		mergeCommit := f.mergeFeatureIntoBase(t)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeMethod = githubcli.MethodRebase
		gh.mergeFacts = mergedFactsFor(f.head, "main", mergeCommit)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultApplied {
			t.Fatalf("result = %q (%s)", res.Result, res.Reason)
		}
		if n := countFindingCode(res.Findings, "branch-rules-unavailable"); n != 0 {
			t.Fatalf("a normal rules read carried %d branch-rules-unavailable findings, want 0", n)
		}
	})
}
```

If `countFindingCode` already exists in package `app` test files (`grep -rn "func countFindingCode" internal/app`), reuse it and drop this definition.

- [ ] **Step 3: Run to verify red**

Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeMergeBranchRulesUnavailableNote' ./internal/app/`
Expected: FAIL — `branch-rules-unavailable findings = 0, want 1` in `merged` and `denied`.

- [ ] **Step 4: Implement**

In `internal/app/finalize_merge.go`, after the `Reason…` const block add:

```go
// FindingBranchRulesUnavailable is the warning finding a finalize.merge result
// carries when GitHub answered the branch-rules read with its plan-gate 403:
// the repository's plan offers no branch rules, so the method came from the
// repository settings alone. A note, never a stop.
const FindingBranchRulesUnavailable = "branch-rules-unavailable"

// branchRulesUnavailableFinding is the one warning a plan-gated merge carries.
func branchRulesUnavailableFinding() StatusFinding {
	return StatusFinding{
		Code:     FindingBranchRulesUnavailable,
		Severity: string(domain.SeverityWarning),
		Message:  "GitHub does not offer branch rules for this repository on its plan; the merge method was chosen from the repository settings alone",
	}
}
```

(`domain` is already imported in this file; confirm with `grep -n '"github.com/danielhanold/docket/internal/domain"' internal/app/finalize_merge.go`.)

In `FinalizeMerge`, replace everything from `mres, merr := deps.GitHub.MergePullRequest(...)` to the end of the function with:

```go
	mres, merr := deps.GitHub.MergePullRequest(ctx, repo, canonicalN, githubcli.ObjectRef(req.Head), admin)
	res := mergeOutcomeResult(ctx, deps, mc, repo, canonicalN, req, id, mres, merr)
	if mres.BranchRulesUnavailable {
		res.Findings = append(res.Findings, branchRulesUnavailableFinding())
	}
	return res
}

// mergeOutcomeResult maps one MergePullRequest return onto the finalize.merge
// document. A transport/launch failure is unknown — the merge may or may not
// have landed; retain and reprobe on the next run, never fabricate a result.
func mergeOutcomeResult(ctx context.Context, deps FinalizeDeps, mc *mergeContext, repo githubcli.Repository, canonicalN int, req FinalizeMergeRequest, id int, mres githubcli.MergeResult, merr error) FinalizeMergeResult {
	if merr != nil {
		return newMergeResult(ResultExternalFailed, FinalizeMergeResult{
			ID: id, Disposition: MergeDispUnknown, Number: canonicalN, Reason: ReasonMergeProbeUnknown, Message: merr.Error(),
		})
	}
	switch mres.Outcome {
	// … the existing switch, moved verbatim (merged, already-merged,
	// method-unavailable, head-moved, not-mergeable, denied, unknown, default) …
	}
}
```

Move the existing `switch mres.Outcome { … }` body into `mergeOutcomeResult` byte-for-byte (it already references `ctx, deps, mc, repo, canonicalN, req, id, mres` — all now parameters). `newMergeResult` normalizes `Findings` to a non-nil slice, so the append is always on a real slice. Leave the existing "Every condition holds. Issue the expected-head merge…" comment above the `admin :=` line in place.

- [ ] **Step 5: Run to verify green**

Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeMerge' ./internal/app/` and `go test -count=1 ./internal/app/ -run 'Merge'`
Expected: PASS.

- [ ] **Step 6: Mutation-check**

Back up `finalize_merge.go`; (a) delete the `if mres.BranchRulesUnavailable { … }` block → `merged` and `denied` redden; (b) make the append unconditional → `rules-read-normally` reddens. Restore with `mv -f`.

- [ ] **Step 7: Commit**

```bash
git add internal/app/finalize_merge.go internal/app/finalize_merge_test.go internal/app/finalize_merge_integration_test.go
git commit -m "feat(finalize): note branch-rules-unavailable when the plan has no branch rules"
```

---

### Task 3: Finish a worktree removal Git already started

**Files:**
- Modify: `internal/workspace/cleanup.go` (`CleanupResult`, `cleanupReady` tail, new `finishStartedRemoval`, `advanceToCleaned`, `stillRegistered`, header comment)
- Test: `internal/workspace/cleanup_integration_test.go`

**Interfaces:**
- Consumes: existing `canonicalizePath` (`prepare.go`), `blockedCleanup`, `writeManifest`, `mapGitFailure`.
- Produces: `CleanupResult.Remnant string` — the recorded path when Cleanup finished a removal Git started but the folder could not be fully deleted (disposition is still `CleanupCleaned`); empty otherwise. Blocked reason string `git refused the non-forcing removal`.

**Background (verified 2026-10-05 in a scratch repo):** with a gitignored subdirectory `locked/` holding a file and chmod `0555`, `git status --porcelain` is clean, `git worktree remove -- <path>` passes its own clean check, prints `error: failed to delete '<path>': Permission denied`, exits 255, and **has already removed the registration**; the folder keeps `.git` and `locked/`. Ignored files do not trip Git's clean check, so no extra commit is needed to build the fixture.

- [ ] **Step 1: Write the test helpers and failing tests**

Append to `internal/workspace/cleanup_integration_test.go` (add `"strings"` to its imports if missing):

```go
// requireNonRoot skips a permission-driven removal-failure test as root, where
// a 0555 directory does not stop an unlink.
func requireNonRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission-based removal failure needs a non-root user")
	}
}

// lockIgnoredDir makes Git's delete of ws fail part-way, the way the 2026-10-05
// reproduction did: an ignored (so clean-check-invisible) directory "locked"
// holding one file, made read-only so its entry cannot be unlinked. The
// directory is made writable again at test end so the temp tree can be removed.
func lockIgnoredDir(t *testing.T, commonDir, ws string) string {
	t.Helper()
	info := filepath.Join(commonDir, "info")
	if err := os.MkdirAll(info, 0o755); err != nil {
		t.Fatal(err)
	}
	ex, err := os.OpenFile(filepath.Join(info, "exclude"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ex.WriteString("\nlocked/\n"); err != nil {
		t.Fatal(err)
	}
	if err := ex.Close(); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(ws, "locked")
	writeWorktreeFile(t, ws, "locked/keep", "ignored bytes\n")
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	return locked
}

// writeRemoveHookGit writes a wrapper "git" that runs pre before and post after
// a real `git worktree remove` (with $last bound to the removed path, and the
// real exit status preserved), and runs list before any `git worktree list`.
// Every other invocation execs the real git. The scripts are POSIX sh.
func writeRemoveHookGit(t *testing.T, pre, post, list string) string {
	t.Helper()
	dir := testsupport.TempDir(t)
	p := filepath.Join(dir, "git")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"worktree\" ] && [ \"$2\" = \"remove\" ]; then\n" +
		"  for a; do last=$a; done\n" +
		"  " + pre + "\n" +
		"  git \"$@\"; rc=$?\n" +
		"  " + post + "\n" +
		"  exit $rc\n" +
		"fi\n" +
		"if [ \"$1\" = \"worktree\" ] && [ \"$2\" = \"list\" ]; then\n" +
		"  " + list + "\n" +
		"fi\n" +
		"exec git \"$@\"\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// serviceWithGit builds a Service over exe while reusing an already-discovered
// repo (discovery must not run through a faulting wrapper).
func serviceWithGit(t *testing.T, exe string) *Service {
	t.Helper()
	c, err := gitcli.NewClient(gitcli.WithExecutable(exe))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	svc, err := NewService(c)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

// assertCleanedTombstone asserts the manifest advanced to the cleaned phase.
func assertCleanedTombstone(t *testing.T, repo gitcli.Repository, tgt Target) {
	t.Helper()
	m, present, err := loadManifest(metaDirOf(repo, tgt))
	if err != nil || !present || m.Phase != PhaseCleaned {
		t.Fatalf("manifest present=%v phase=%v err=%v; want cleaned tombstone", present, m.Phase, err)
	}
}

// TestIntegrationWorkspaceLifecycleCleanupFinishesStartedRemoval: Git passes its
// clean check, removes the registration, then fails part-way through the delete
// (the wrapper makes the locked directory deletable again right after). Docket
// finishes the delete: folder gone, manifest cleaned, no remnant, branch kept.
func TestIntegrationWorkspaceLifecycleCleanupFinishesStartedRemoval(t *testing.T) {
	requireNonRoot(t)
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	lockIgnoredDir(t, repo.CommonDir, ws)

	exe := writeRemoveHookGit(t, ":", "chmod 0755 \"$last/locked\"", ":")
	res := cleanupOK(t, serviceWithGit(t, exe), repo, tgt)

	if res.Disposition != CleanupCleaned || res.Remnant != "" {
		t.Fatalf("got disposition %q remnant %q; want cleaned with no remnant", res.Disposition, res.Remnant)
	}
	if _, err := os.Lstat(ws); !os.IsNotExist(err) {
		t.Fatalf("workspace dir Lstat err = %v; want removed", err)
	}
	if registeredLine(gitOut(t, r.Primary, "worktree", "list", "--porcelain"), ws) {
		t.Fatal("worktree still registered")
	}
	assertCleanedTombstone(t, repo, tgt)
	if !branchExists(r.Primary, "feat/"+prepSlug) {
		t.Fatal("feat branch deleted; cleanup never deletes a branch")
	}
}

// TestIntegrationWorkspaceLifecycleCleanupRemnantWhenFinishFails: Git started the
// delete and failed; Docket's finish fails on the same locked directory. The
// manifest is still cleaned and the result is cleaned with Remnant = the path.
func TestIntegrationWorkspaceLifecycleCleanupRemnantWhenFinishFails(t *testing.T) {
	requireNonRoot(t)
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	lockIgnoredDir(t, repo.CommonDir, ws)

	res := cleanupOK(t, svc, repo, tgt)

	if res.Disposition != CleanupCleaned {
		t.Fatalf("disposition = %q; want cleaned", res.Disposition)
	}
	if res.Remnant != ws {
		t.Fatalf("remnant = %q; want %q", res.Remnant, ws)
	}
	if _, err := os.Lstat(filepath.Join(ws, "locked", "keep")); err != nil {
		t.Fatalf("the undeletable leftover should remain: %v", err)
	}
	if registeredLine(gitOut(t, r.Primary, "worktree", "list", "--porcelain"), ws) {
		t.Fatal("worktree still registered")
	}
	assertCleanedTombstone(t, repo, tgt)
}

// TestIntegrationWorkspaceLifecycleCleanupRemnantIgnoresUnrelatedStaleRegistration:
// an unrelated prunable registration (directory deleted, still registered)
// does not stop the finish — the registration check keys on this path and the
// feature ref, never on every registration resolving.
func TestIntegrationWorkspaceLifecycleCleanupRemnantIgnoresUnrelatedStaleRegistration(t *testing.T) {
	requireNonRoot(t)
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	prunable := filepath.Join(repo.PrimaryWorktree, ".worktrees", "prunable")
	gitOut(t, r.Primary, "worktree", "add", "-b", "feat/prunable", prunable, "main")
	if err := os.RemoveAll(prunable); err != nil {
		t.Fatal(err)
	}
	lockIgnoredDir(t, repo.CommonDir, ws)

	exe := writeRemoveHookGit(t, ":", "chmod 0755 \"$last/locked\"", ":")
	res := cleanupOK(t, serviceWithGit(t, exe), repo, tgt)

	if res.Disposition != CleanupCleaned || res.Remnant != "" {
		t.Fatalf("got disposition %q remnant %q (blocked by %v); want cleaned", res.Disposition, res.Remnant, res.BlockedBy)
	}
	if _, err := os.Lstat(ws); !os.IsNotExist(err) {
		t.Fatalf("workspace dir Lstat err = %v; want removed", err)
	}
	if !registeredLine(gitOut(t, r.Primary, "worktree", "list", "--porcelain"), prunable) {
		t.Fatal("the unrelated prunable registration disappeared; cleanup must never prune")
	}
}

// TestIntegrationWorkspaceLifecycleCleanupGitRefusalStillBlocked: an untracked
// file appears after Docket's own clean check (the wrapper writes it just
// before the real removal). Git refuses with the path still registered: the
// result is blocked with the refusal reason, nothing is deleted, the manifest
// stays ready.
func TestIntegrationWorkspaceLifecycleCleanupGitRefusalStillBlocked(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	tracked := readFileBytes(t, filepath.Join(ws, "main.go"))

	exe := writeRemoveHookGit(t, "printf 'late\\n' > \"$last/late-untracked.txt\"", ":", ":")
	res := cleanupOK(t, serviceWithGit(t, exe), repo, tgt)

	if res.Disposition != CleanupBlocked {
		t.Fatalf("disposition = %q; want blocked", res.Disposition)
	}
	if !slicesContains(res.BlockedBy, "git refused the non-forcing removal") {
		t.Fatalf("BlockedBy = %v; want the refusal reason", res.BlockedBy)
	}
	if !containsWorktreePath(t, gitOut(t, r.Primary, "worktree", "list", "--porcelain"), ws) {
		t.Fatal("registration removed; a refusal must leave it")
	}
	if got := readFileBytes(t, filepath.Join(ws, "main.go")); got != tracked {
		t.Fatal("tracked file changed")
	}
	if _, err := os.Lstat(filepath.Join(ws, "late-untracked.txt")); err != nil {
		t.Fatalf("the late untracked file must survive: %v", err)
	}
	assertReadyManifestKept(t, r, repo, tgt)
}

// TestIntegrationWorkspaceLifecycleCleanupRelistErrorDeletesNothing: Git half-
// removes, then the re-list fails. A probe error is never absence: the result
// is a failed error, Docket deletes nothing (the wrapper made the folder fully
// deletable, so a wrongful RemoveAll would empty it), and the manifest stays
// ready.
func TestIntegrationWorkspaceLifecycleCleanupRelistErrorDeletesNothing(t *testing.T) {
	requireNonRoot(t)
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	lockIgnoredDir(t, repo.CommonDir, ws)

	marker := filepath.Join(testsupport.TempDir(t), "removed")
	exe := writeRemoveHookGit(t, ":",
		"chmod 0755 \"$last/locked\"; : > '"+marker+"'",
		"if [ -f '"+marker+"' ]; then echo 'fake git: list disabled' >&2; exit 1; fi")
	res, err := serviceWithGit(t, exe).Cleanup(context.Background(), CleanupRequest{Repository: repo, Target: tgt})

	if err == nil || res.Disposition != CleanupFailed {
		t.Fatalf("got disposition %q err %v; want failed with an error", res.Disposition, err)
	}
	if _, err := os.Lstat(filepath.Join(ws, "locked", "keep")); err != nil {
		t.Fatalf("Docket deleted the leftover after a failed re-list: %v", err)
	}
	if m, present, err := loadManifest(metaDirOf(repo, tgt)); err != nil || !present || m.Phase != PhaseReady {
		t.Fatalf("manifest present=%v phase=%v err=%v; want ready", present, m.Phase, err)
	}
}

// slicesContains reports whether any element of xs contains sub.
func slicesContains(xs []string, sub string) bool {
	for _, x := range xs {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}
```

Check before writing: `testsupport` is imported elsewhere in this package's integration tests (`prepare_integration_test.go` uses `testsupport.TempDir`); add `"github.com/danielhanold/docket/internal/testsupport"` to this file's imports if it is not already there. If any helper name above (`requireNonRoot`, `lockIgnoredDir`, `serviceWithGit`, `slicesContains`) already exists in the package (`grep -rn "func <name>" internal/workspace`), rename the new one rather than duplicating. Confirm `main.go` is a tracked file in the `mainModeRepo` fixture (`TestIntegrationWorkspaceLifecycleCleanupDirtyNeverRemoved` writes it as a dirty tracked edit); if not, use any tracked file the fixture commits.

- [ ] **Step 2: Run to verify red**

Run: `go test -tags integration -count=1 -run '^TestIntegrationWorkspaceLifecycleCleanup' ./internal/workspace/`
Expected: build failure on `res.Remnant` (field undefined).

- [ ] **Step 3: Implement**

In `internal/workspace/cleanup.go`:

Add `"os"` to the imports.

Replace the `CleanupResult` doc and struct with:

```go
// CleanupResult is the value outcome of a Cleanup. BlockedBy carries a bounded,
// redacted set of reasons/paths when the disposition is blocked (empty
// otherwise); it is diagnostic data, never an oracle. Remnant is set only on a
// cleaned disposition, to the recorded path, when Cleanup finished a removal Git
// had already started (its registration gone) but could not delete the whole
// folder; the checkout is no longer a Git checkout and a human deletes the rest.
type CleanupResult struct {
	Disposition CleanupDisposition
	Path        string
	BlockedBy   []string
	Remnant     string
}
```

Replace the tail of `cleanupReady` — from the `// Remove non-forcingly:` comment through the final `return CleanupResult{Disposition: CleanupCleaned, Path: m.Path}, nil` — with:

```go
	// Remove non-forcingly: Git rechecks cleanliness at the destructive boundary,
	// closing the check-then-remove race a forced removal would leave. A command
	// failure is either Git's refusal (nothing deleted, registration intact) or a
	// delete Git started and could not finish (registration already gone);
	// finishStartedRemoval tells them apart. Any other removal error (the process
	// could not run, was cancelled, or timed out) is a failure.
	if err := s.git.RemoveWorktreeClean(ctx, repo, m.Path); err != nil {
		if f, ok := gitcli.AsFailure(err); ok && f.Kind == gitcli.KindCommandFailed {
			return s.finishStartedRemoval(ctx, repo, dir, m, target)
		}
		return CleanupResult{Disposition: CleanupFailed, Path: m.Path}, mapGitFailure(cleanupOp, "remove", err)
	}
	return advanceToCleaned(dir, m, "")
}

// finishStartedRemoval classifies a `git worktree remove` that exited non-zero.
// Git runs its own clean check first, then deletes the tree and, even when that
// delete fails, deletes its registration ("there's no going back from here").
// So a fresh worktree list decides:
//   - the list cannot be read: a failed error — a probe error is never absence;
//   - the recorded path (or any registration on the feature ref) is still
//     registered: Git refused before deleting anything — blocked, byte-untouched;
//   - it is no longer registered: Git passed its clean check and committed to the
//     delete. Docket finishes it with os.RemoveAll on the recorded path — the one
//     case where Docket deletes a directory by path — and advances the manifest to
//     cleaned. If that delete fails too, the manifest still advances (the folder is
//     no longer a Git checkout) and Remnant names the leftover path.
//
// The leftover can only hold copies of tracked files at the verified head,
// gitignored files, and anything written during Git's delete — all of which Git
// was already deleting under the same clean proof.
func (s *Service) finishStartedRemoval(ctx context.Context, repo gitcli.Repository, dir string, m Manifest, target Target) (CleanupResult, error) {
	infos, err := s.git.ListWorktrees(ctx, repo)
	if err != nil {
		return CleanupResult{Disposition: CleanupFailed, Path: m.Path}, mapGitFailure(cleanupOp, "inventory", err)
	}
	if stillRegistered(infos, m.Path, target.FeatureRef) {
		return blockedCleanup(m.Path, "git refused the non-forcing removal"), nil
	}
	// Defensive: ownsManifest already proved this; never delete anything but
	// this repository's <primary>/.worktrees/<slug>.
	if m.Path != filepath.Join(repo.PrimaryWorktree, ".worktrees", target.Slug) {
		return blockedCleanup(m.Path, "recorded path is not this repository's workspace path"), nil
	}
	remnant := ""
	if err := os.RemoveAll(m.Path); err != nil {
		remnant = m.Path
	}
	return advanceToCleaned(dir, m, remnant)
}

// stillRegistered reports whether Git still holds a registration for the
// workspace after a failed removal: any attached registration on the feature
// ref, or a registration whose path is the recorded path lexically or
// canonically. An unrelated registration whose path no longer resolves is not
// this workspace — the proven registration was on the feature ref, and a
// refusal leaves it there — so it never blocks the finish.
func stillRegistered(infos []gitcli.WorktreeInfo, path string, featureRef gitcli.RefName) bool {
	for _, info := range infos {
		if !info.Detached && info.Branch == featureRef {
			return true
		}
		if abs, err := filepath.Abs(info.Path); err == nil && filepath.Clean(abs) == path {
			return true
		}
		if cp, err := canonicalizePath(info.Path); err == nil && cp == path {
			return true
		}
	}
	return false
}

// advanceToCleaned advances the manifest atomically from ready to the cleaned
// tombstone (the monotonic chain allows ready->cleaned only; refuse defensively
// if it does not hold) and returns cleaned, carrying remnant when set.
func advanceToCleaned(dir string, m Manifest, remnant string) (CleanupResult, error) {
	if !m.Phase.canAdvanceTo(PhaseCleaned) {
		return CleanupResult{Disposition: CleanupFailed, Path: m.Path}, &Failure{Op: cleanupOp, Stage: "manifest", Kind: KindInvalidState, Detail: "manifest phase may not advance to cleaned"}
	}
	m.Phase = PhaseCleaned
	m.UpdatedUTC = time.Now().UTC().Format(time.RFC3339)
	if err := writeManifest(dir, m); err != nil {
		return CleanupResult{Disposition: CleanupFailed, Path: m.Path}, &Failure{Op: cleanupOp, Stage: "manifest", Kind: KindExternal, Detail: "advancing manifest to cleaned", Err: err}
	}
	return CleanupResult{Disposition: CleanupCleaned, Path: m.Path, Remnant: remnant}, nil
}
```

Update the file header comment:
- Line 4–5: change `never a local or remote branch, never an administrative directory by pathname, never a transaction or sibling worktree,` to `never a local or remote branch, never a transaction or sibling worktree, never a directory by pathname except to finish a removal Git already committed to (step 4),`.
- Replace step 4's text with:
  ```
  //	4. remove via the NON-FORCING gitcli.RemoveWorktreeClean so Git itself rechecks
  //	   cleanliness at the destructive boundary. A preflight status check followed by
  //	   a forced removal would leave a race in which a worker writes between the two
  //	   calls and loses data; the non-forcing primitive closes it. When the removal
  //	   exits non-zero, a fresh worktree list decides: still registered is Git's
  //	   refusal (`blocked`, byte-untouched); no longer registered means Git passed its
  //	   own clean check and started deleting, so Cleanup finishes the delete of the
  //	   recorded path and reports any undeletable leftover as Remnant; an unreadable
  //	   list is `failed`;
  ```
- Step 5: change `after Git confirms removal,` to `after the removal completes (or is finished),`.

- [ ] **Step 4: Run to verify green**

Run: `go test -tags integration -count=1 -run '^TestIntegrationWorkspaceLifecycleCleanup' ./internal/workspace/` then `go test -count=1 ./internal/workspace/`
Expected: PASS (the existing Cleanup tests, including `DirtyNeverRemoved` and `NeverPrunes`, stay green).

- [ ] **Step 5: Mutation-check**

Back up `cleanup.go`; one at a time, run the Step 4 integration command, confirm red, restore with `mv -f cleanup.go.bak cleanup.go`:
1. In `cleanupReady`, revert the `KindCommandFailed` branch to `return blockedCleanup(m.Path, "git refused the non-forcing removal"), nil`: FinishesStartedRemoval, RemnantWhenFinishFails, IgnoresUnrelatedStaleRegistration redden.
2. Delete the `os.RemoveAll` call (keep `remnant := ""`): FinishesStartedRemoval reddens (folder still there).
3. Set `remnant = ""` unconditionally: RemnantWhenFinishFails reddens.
4. In `finishStartedRemoval`, treat a list error as absence (`if err != nil { infos = nil }`): RelistErrorDeletesNothing reddens.
5. Make `stillRegistered` return `false` always: GitRefusalStillBlocked reddens.
6. Replace `stillRegistered(...)` with `classifyRegistrationAbsence(infos, m.Path, target.FeatureRef) != regAbsent`: IgnoresUnrelatedStaleRegistration reddens (proves the stale-registration case is pinned).

- [ ] **Step 6: Commit**

```bash
git add internal/workspace/cleanup.go internal/workspace/cleanup_integration_test.go
git commit -m "fix(workspace): finish a worktree removal Git already started"
```

---

### Task 4: Carry the remnant note through finalize cleanup and name what blocked a workspace

**Files:**
- Modify: `internal/app/finalize_cleanup.go` (file comment, new constant, `finalizeCleanupDone`, `finalizeCleanupWorkspace`)
- Test: `internal/app/finalize_cleanup_integration_test.go`

**Interfaces:**
- Consumes: `workspace.CleanupResult.Remnant string`, `workspace.CleanupResult.BlockedBy []string` (Task 3).
- Produces: `const FindingWorkspaceRemnant = "workspace-remnant"`; `func finalizeCleanupWorkspace(ctx context.Context, deps FinalizeDeps, cc *closeoutContext) (clean bool, finding *StatusFinding, note *StatusFinding)`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/finalize_cleanup_integration_test.go`:

```go
// lockIgnoredWorkspaceDir makes Git's delete of the feature workspace fail part-
// way after it has removed the registration: an ignored (clean-check-invisible)
// directory holding one file, made read-only. Restored at test end.
func lockIgnoredWorkspaceDir(t *testing.T, commonDir, ws string) {
	t.Helper()
	info := filepath.Join(commonDir, "info")
	if err := os.MkdirAll(info, 0o755); err != nil {
		t.Fatal(err)
	}
	ex, err := os.OpenFile(filepath.Join(info, "exclude"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ex.WriteString("\nlocked/\n"); err != nil {
		t.Fatal(err)
	}
	if err := ex.Close(); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, ws, "locked/keep", "ignored bytes\n")
	locked := filepath.Join(ws, "locked")
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
}

// TestIntegrationFinalizeCleanupWorkspaceRemnant: Git removes the registration
// but part of the folder cannot be deleted. The workspace leg is done, the
// local and remote branch legs still run, and the result is cleaned with
// exactly one workspace-remnant warning naming the leftover path.
func TestIntegrationFinalizeCleanupWorkspaceRemnant(t *testing.T) {
	requireRealGit(t)
	if os.Geteuid() == 0 {
		t.Skip("permission-based removal failure needs a non-root user")
	}
	f := setupCloseoutFixture(t, planRepoModeDocket())
	head, mergeCommit := f.archiveClosed(t)
	gh := f.mergedCleanupFake(head, mergeCommit)
	ws, err := filepath.EvalSymlinks(f.wp)
	if err != nil {
		t.Fatal(err)
	}
	lockIgnoredWorkspaceDir(t, f.gitrepo.CommonDir, ws)

	res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)

	if res.Result != ResultApplied || res.Disposition != CleanupDispCleaned {
		t.Fatalf("cleanup = %q disp %q (%s) findings %+v; want applied/cleaned", res.Result, res.Disposition, res.Message, res.Findings)
	}
	if f.localBranchPresent(t) || f.remoteBranchPresent(t) {
		t.Fatal("the branch legs must still run after a remnant")
	}
	if len(res.Findings) != 1 || res.Findings[0].Code != "workspace-remnant" || res.Findings[0].Severity != "warning" {
		t.Fatalf("findings = %+v; want exactly one workspace-remnant warning", res.Findings)
	}
	want := "Git removed the worktree but part of the folder could not be deleted; delete " + ws + " by hand"
	if res.Findings[0].Message != want {
		t.Fatalf("message = %q; want %q", res.Findings[0].Message, want)
	}
}

// TestIntegrationFinalizeCleanupWorkspaceBlockedNamesPath: an untracked,
// non-ignored file blocks cleanup (nothing touched, branches retained) and the
// workspace-blocked finding names it.
func TestIntegrationFinalizeCleanupWorkspaceBlockedNamesPath(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModeDocket())
	head, mergeCommit := f.archiveClosed(t)
	gh := f.mergedCleanupFake(head, mergeCommit)
	writeRepoFile(t, f.wp, "stray-notes.txt", "not committed\n")

	res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)

	if res.Disposition != CleanupDispPending {
		t.Fatalf("disposition = %q; want pending", res.Disposition)
	}
	var msg string
	for _, fd := range res.Findings {
		if fd.Code == "workspace-blocked" {
			msg = fd.Message
		}
	}
	if !strings.Contains(msg, "(stray-notes.txt)") {
		t.Fatalf("workspace-blocked message = %q; want it to name stray-notes.txt", msg)
	}
	if !f.localBranchPresent(t) {
		t.Fatal("a blocked workspace must retain the local branch")
	}
	if _, err := os.Stat(filepath.Join(f.wp, "stray-notes.txt")); err != nil {
		t.Fatalf("the blocking file must survive: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify red**

Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeCleanupWorkspace' ./internal/app/`
Expected: FAIL — remnant test reports `pending` or a `workspace-blocked` finding is absent / message lacks `(stray-notes.txt)` (the Task 3 change alone yields `cleaned`, but the app layer does not yet emit the note; confirm the actual failure text before proceeding).

- [ ] **Step 3: Implement**

In `internal/app/finalize_cleanup.go`:

Add `"strings"` to the imports.

In the file comment, change `It never calls a global worktree prune, force-removes a checkout, recursively deletes by pathname, or touches the primary, metadata, transaction, sibling, or foreign worktree.` to `It never calls a global worktree prune, force-removes a checkout, or touches the primary, metadata, transaction, sibling, or foreign worktree, and never recursively deletes by pathname except where workspace.Cleanup finishes a removal Git already committed to (a leftover it cannot delete is reported as a workspace-remnant warning, never a stop).`

After the reasons const block add:

```go
// FindingWorkspaceRemnant is the warning a cleaned finalize-cleanup result
// carries when Git removed the worktree but part of its folder could not be
// deleted. A note for a human, never a retryable leg: the disposition stays
// cleaned.
const FindingWorkspaceRemnant = "workspace-remnant"
```

Replace `finalizeCleanupWorkspace` with:

```go
// finalizeCleanupWorkspace removes the feature checkout through workspace.Cleanup.
// It returns clean=true when the checkout is gone (cleaned or an already-clean
// tombstone) and clean=false with a retryable finding when the base is
// unresolved, the target is malformed, the inspection could not be answered, or
// the workspace is blocked — each retaining the workspace byte-untouched. A
// cleaned result whose folder could not be fully deleted also returns a
// non-retryable workspace-remnant note naming the leftover path.
func finalizeCleanupWorkspace(ctx context.Context, deps FinalizeDeps, cc *closeoutContext) (clean bool, finding *StatusFinding, note *StatusFinding) {
	base := domain.ResolveEffectiveBase(cc.snap, cc.change, domain.NewBranchFacts(nil))
	if base.Kind != domain.BaseResolved {
		f := cleanupWarning(ReasonCleanupUnresolvedBase, "the change's effective base did not resolve; the workspace is retained")
		return false, &f, nil
	}
	branch, berr := recordedBranch(cc.change)
	if berr != nil {
		f := cleanupWarning(berr.Error(), "the change's recorded feature branch is unusable; the workspace is retained")
		return false, &f, nil
	}
	target, err := workspace.NewTarget(cc.change.ID(), cc.change.Slug(), base, branch)
	if err != nil {
		f := cleanupWarning(ReasonCleanupUnresolvedBase, "the change's workspace target is malformed; the workspace is retained")
		return false, &f, nil
	}
	res, err := deps.Workspace.Cleanup(ctx, workspace.CleanupRequest{Repository: cc.repo, Target: target})
	if err != nil {
		f := cleanupWarning(ReasonCleanupWorkspaceProbe, "the feature workspace could not be inspected; it is retained")
		return false, &f, nil
	}
	switch res.Disposition {
	case workspace.CleanupCleaned, workspace.CleanupAlreadyClean:
		if res.Remnant != "" {
			n := cleanupWarning(FindingWorkspaceRemnant,
				"Git removed the worktree but part of the folder could not be deleted; delete "+res.Remnant+" by hand")
			return true, nil, &n
		}
		return true, nil, nil
	default:
		msg := "the feature workspace is not a clean, ready checkout; it is retained"
		if len(res.BlockedBy) > 0 {
			msg = "the feature workspace is not a clean, ready checkout (" + strings.Join(res.BlockedBy, ", ") + "); it is retained"
		}
		f := cleanupWarning(ReasonCleanupWorkspaceBlocked, msg)
		return false, &f, nil
	}
}
```

In `finalizeCleanupDone`, replace the Leg 2 block and everything after it to the end of the function with:

```go
	// Leg 2: remove the feature checkout through the landed manifest-fact-driven
	// Cleanup. A blocked workspace or an unanswerable inspection retains the
	// workspace and — because the branch may still be checked out — skips the ref
	// legs entirely. A remnant note is kept apart from the retryable findings: it
	// never turns a cleaned result pending.
	var notes []StatusFinding
	workspaceClean, wsFinding, wsNote := finalizeCleanupWorkspace(ctx, deps, cc)
	if wsFinding != nil {
		findings = append(findings, *wsFinding)
	}
	if wsNote != nil {
		notes = append(notes, *wsNote)
	}
	if !workspaceClean {
		return finalizeCleanupResult(id, CleanupDispPending, removed, findings,
			"the feature workspace could not be cleanly removed; the branches are retained")
	}

	// Leg 3: … (unchanged local-ref leg)
	// Leg 4: … (unchanged remote-ref leg)

	if childRetarget {
		r := newCleanupResult(OperationFinalizeCleanup, ResultApplied, CleanupOpResult{
			ID: id, Disposition: CleanupDispChildrenRetargetRequired, RemovedRefs: removed, Findings: findings,
			Reason:  ReasonCleanupChildProbe,
			Message: "an open child pull request still targets this branch; the remote branch is retained until the children are retargeted",
		})
		r.Findings = append(r.Findings, notes...)
		return r
	}
	if localDone && remoteDone && len(findings) == 0 {
		r := newCleanupResult(OperationFinalizeCleanup, ResultApplied, CleanupOpResult{
			ID: id, Disposition: CleanupDispCleaned, RemovedRefs: removed, Findings: findings,
			Message: "the final change was cleaned: workspace removed and feature refs deleted",
		})
		r.Findings = append(r.Findings, notes...)
		return r
	}
	r := finalizeCleanupResult(id, CleanupDispPending, removed, findings,
		"cleanup is partially complete; the retained legs are independently retryable")
	r.Findings = append(r.Findings, notes...)
	return r
}
```

Keep the Leg 3 and Leg 4 code exactly as it is today (only the Leg 2 block and the three returns change). Notes are appended **after** `finalizeCleanupResult` computes `Reason` from `findings[0]`, so a note never becomes the result's reason. `newCleanupResult` normalizes nil `Findings` to `[]StatusFinding{}`, so the appends are on real slices.

- [ ] **Step 4: Run to verify green**

Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeCleanup' ./internal/app/` and `go test -count=1 -run 'Cleanup' ./internal/app/`
Expected: PASS (existing `BranchDeletion`, `InjectedProbeErrors`, `Retryable`, etc. stay green).

- [ ] **Step 5: Mutation-check**

Back up `finalize_cleanup.go`; one at a time, run the Step 4 integration command, confirm red, restore with `mv -f`:
1. Append the remnant note to `findings` instead of `notes`: WorkspaceRemnant reddens (`pending`, not `cleaned`).
2. Drop the `if res.Remnant != ""` branch (return `true, nil, nil`): WorkspaceRemnant reddens (zero findings).
3. Remove the `len(res.BlockedBy) > 0` message branch: WorkspaceBlockedNamesPath reddens.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finalize_cleanup.go internal/app/finalize_cleanup_integration_test.go
git commit -m "feat(finalize): report a workspace remnant and name what blocked a workspace"
```

---

### Task 5: Document the plan-gate note and the remnant in the finalize skill and guide

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (steps 8, 9, 10 — append to existing lines, add no new lines)
- Modify: `docs/guide/landing-changes.md` (step 4)
- Modify: `internal/repoguard/budgets_test.go` (`docket-finalize-change/SKILL.md` row)
- Regenerate: `internal/assets/embedded/**` via `go generate ./internal/assets/`

**Interfaces:**
- Consumes: finding codes `branch-rules-unavailable` (Task 2) and `workspace-remnant` / `workspace-blocked` (Task 4).
- Produces: nothing code-facing.

Docs describe only current behavior: no change numbers, no PR citations, no "previously" in user-facing prose.

- [ ] **Step 1: Edit the skill (no new lines)**

In `skills/docket-finalize-change/SKILL.md`:

- Step 8 paragraph (begins ``The `finalize.merge` operation with``): after the sentence ending `…it is not \`merge-denied\` and is never retried with another method.` insert on the same line:
  ` On a private repository whose GitHub plan offers no branch rules, GitHub's plan-gate answer means the branch has no rules: the method comes from the repository settings alone and the result carries a \`branch-rules-unavailable\` warning finding; any other unreadable branch-rules response stays \`unknown\`.`
- Step 9, on the line that begins `repair commits, and the attempts used, alongside any notes from the invocation.`, after `…so the note is the repair's only surviving trace.` insert:
  ` A \`finalize.merge\` result carrying a \`branch-rules-unavailable\` finding likewise sends one \`late_findings\` entry recording that the merge method came from the repository settings alone.`
- Step 10 paragraph, after `…Cleanup failure never unwinds the merge.` insert:
  ` A \`cleaned\` result may carry a \`workspace-remnant\` warning naming a leftover folder to delete by hand; a \`workspace-blocked\` finding names what blocked the workspace.`

(The backslashes above only escape backticks for this plan; write plain backticks in the file.)

- [ ] **Step 2: Edit the guide**

In `docs/guide/landing-changes.md`, change step 4 from:

```
4. merges the pull request with the first merge method the repository permits: rebase, then merge
   commit, then squash;
```

to:

```
4. merges the pull request with the first merge method the repository permits: rebase, then merge
   commit, then squash. On a private repository whose GitHub plan has no branch rules, the method
   comes from the repository settings alone;
```

- [ ] **Step 3: Rebaseline the skill budget**

Run: `wc -l skills/docket-finalize-change/SKILL.md; wc -w skills/docket-finalize-change/SKILL.md`
Expected: lines still `239`; words ≈ 5820 + ~95.

In `internal/repoguard/budgets_test.go`, change the row `{"docket-finalize-change/SKILL.md", 239, 5820},` to the measured numbers `{"docket-finalize-change/SKILL.md", 239, <measured words>},` and prepend to its trailing comment: `// 0525: plan-gated merges carry branch-rules-unavailable into closeout notes; cleanup reports workspace-remnant and names what blocked a workspace (239/5820 -> 239/<measured words>); ` followed by the existing comment text (drop the existing leading `// `, keeping one).

- [ ] **Step 4: Regenerate the embedded bundle**

Run: `go generate ./internal/assets/`
Then: `git status --porcelain internal/assets/` — expect the embedded `skills/docket-finalize-change/SKILL.md` copy and the manifest to change.

- [ ] **Step 5: Verify**

Run: `go test -count=1 ./internal/repoguard/ ./internal/assets/`
Expected: PASS (`TestSkillSizeBudgets`, `TestEmbeddedMatchesAuthored`, the prose-contract sentinels, and anchor-style guards).

Mutation-check the budget row: temporarily set the word ceiling to the old `5820`, re-run `go test -count=1 -run TestSkillSizeBudgets ./internal/repoguard/`, confirm red, restore.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md docs/guide/landing-changes.md internal/repoguard/budgets_test.go internal/assets/
git commit -m "docs(finalize): plan-gated merges note branch-rules-unavailable; cleanup reports remnants"
```

---

## Final gate

`docket-build` runs the whole suite once at the end through the configured `build.test_command` (`go run ./cmd/docket development test`). Read its budget report even on green: a `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line for the `app_cleanup`, `app_merge`, `workspace_lifecycle`, or `githubcli_merge` shards is a finding to report (this change adds two app cleanup fixtures, three app merge sub-fixtures, and five workspace fixtures).

## Spec coverage map

| Spec section | Task |
|---|---|
| §1 Recognize the plan-gate answer; `MergeResult.BranchRulesUnavailable` on every post-probe outcome | 1 |
| §2 `branch-rules-unavailable` warning on finalize.merge | 2 |
| §2 skill sends one `late_findings` entry; guide step 4 sentence | 5 |
| §3 finish a started removal; list-error failed; still-registered blocked; `Remnant`; comment updates | 3 (workspace), 4 (`finalize_cleanup.go` file comment) |
| §3 ADR | parent, via `docket-adr` — not a plan task |
| §4 branches still run after a remnant; `workspace-remnant` warning; disposition cleaned | 4 |
| §5 `workspace-blocked` names `BlockedBy` | 4 |
| §6 tests (plan-gate probe incl. Team, lookalikes; finalize merge; workspace four cases; finalize cleanup two cases) | 1, 2, 3, 4 |
