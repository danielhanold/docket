<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0358 — Treat empty-string reviewDecision as no-decision, not an invalid enum](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0358-treat-empty-review-decision-as-no-decision.md)**
<!-- docket:backlink:end -->
# Treat Empty-String reviewDecision as No-Decision Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `normalizeReviewDecision` treat a non-nil pointer to `""` the same as `nil` — "no required-review decision" (`Approved: false`, no error) — instead of an invalid enum, so finalize stops halting on `pr-unknown` for repos whose branch protection requires a PR but zero approvals.

**Architecture:** One-guard change in `internal/githubcli/pr.go` plus a doc-comment update, protected by one new regression test in `internal/githubcli/probe_test.go` that reuses the existing `probePRJSONWithDecision` fixture. The existing `TestViewPullRequestUnknownReviewDecisionFailsClosed` must stay green — genuinely unknown non-empty vocabulary still fails closed.

**Tech Stack:** Go; `go test`; whole-suite gate via `scripts/run-tests.sh`.

**Spec:** none (trivial change) — plan argues from the change file:
`docs/changes/active/0358-treat-empty-review-decision-as-no-decision.md` (on the `docket` metadata branch).

## Global Constraints

- Only the empty/null equivalence changes. A real `APPROVED` / `CHANGES_REQUESTED` / `REVIEW_REQUIRED` decision is handled exactly as before, and unknown non-empty enums still fail closed (change file, "Out of scope").
- `normalizeState` is deliberately NOT touched: PR `state` is a required field, so `nil` and unrecognized values there are errors by design. Do not "fix" the twin.
- Every verification `go test` run passes `-count=1` — Go's test cache can serve a pre-mutation verdict otherwise (learnings: cached-runner-serves-a-mutated-tree).
- The build gate runs the whole suite (`scripts/run-tests.sh`), never only the tests named here.

---

### Task 1: Empty-string reviewDecision is no-decision

**Files:**
- Modify: `internal/githubcli/pr.go` (function `normalizeReviewDecision` and its doc comment)
- Test: `internal/githubcli/probe_test.go` (new test, inserted after `TestViewPullRequestReviewDecisionMapping`)

**Interfaces:**
- Consumes: existing test helpers in `probe_test.go` — `probePRJSONWithDecision(number int, state string, decision *string) string`, `strPtr(s string) *string`, `newFakeClient`, `fakeScenario`, `fakeArm`, `probeRepo()`, `ensRepoSpec`.
- Produces: unchanged signature `normalizeReviewDecision(raw *string) (bool, error)`; new behavior: `raw == nil || *raw == ""` → `(false, nil)`.

- [ ] **Step 1: Write the failing regression test**

Insert into `internal/githubcli/probe_test.go`, between `TestViewPullRequestReviewDecisionMapping` (ends near the comment `// TestViewPullRequestUnknownReviewDecisionFailsClosed: unknown non-null`) and `TestViewPullRequestUnknownReviewDecisionFailsClosed`:

```go
// TestViewPullRequestEmptyReviewDecisionIsNoDecision: GitHub returns
// reviewDecision "" — an empty string, not JSON null — for a repository whose
// branch protection requires a PR but zero approvals. Empty string means the
// same thing as null: no required-review decision, so Approved is false and
// decode succeeds. Complements TestViewPullRequestUnknownReviewDecisionFailsClosed,
// which keeps genuinely unknown non-empty vocabulary failing closed.
func TestViewPullRequestEmptyReviewDecisionIsNoDecision(t *testing.T) {
	doc := probePRJSONWithDecision(7, "OPEN", strPtr(""))
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		{ArgvPrefix: []string{"pr", "view", "7"}, Stdout: doc, Exit: 0},
	}})
	pr, err := c.ViewPullRequest(context.Background(), probeRepo(), 7)
	if err != nil {
		t.Fatalf("empty-string reviewDecision errored; want no-decision success: %v", err)
	}
	if pr.Approved {
		t.Errorf("Approved = true, want false for empty-string reviewDecision")
	}
}
```

No new imports are needed — `context` is already imported by the file.

- [ ] **Step 2: Run the new test to verify it fails**

Run: `go test -count=1 ./internal/githubcli/ -run TestViewPullRequestEmptyReviewDecisionIsNoDecision -v`
Expected: FAIL — `ViewPullRequest` errors with the typed failure wrapping `unrecognized pull-request reviewDecision enum` (the `t.Fatalf("empty-string reviewDecision errored…")` branch fires). This red is the mutation evidence that the new test detects the pre-fix behavior.

- [ ] **Step 3: Implement the guard and doc-comment change**

In `internal/githubcli/pr.go`, replace the current function (doc comment included):

```go
// normalizeReviewDecision maps GitHub's nullable reviewDecision enum to the
// Approved boolean. Only APPROVED is an affirmative decision; REVIEW_REQUIRED,
// CHANGES_REQUESTED, and null/absent are false — null never becomes true merely
// because a repository has no required-review rule. Unknown non-null vocabulary
// is invalid external state and is rejected, never folded into either outcome.
func normalizeReviewDecision(raw *string) (bool, error) {
	if raw == nil {
		return false, nil
	}
	switch *raw {
	case "APPROVED":
		return true, nil
	case "REVIEW_REQUIRED", "CHANGES_REQUESTED":
		return false, nil
	default:
		return false, errEnum("unrecognized pull-request reviewDecision enum")
	}
}
```

with:

```go
// normalizeReviewDecision maps GitHub's nullable reviewDecision enum to the
// Approved boolean. Only APPROVED is an affirmative decision; REVIEW_REQUIRED,
// CHANGES_REQUESTED, and null/absent are false — null never becomes true merely
// because a repository has no required-review rule. GitHub also reports "no
// decision" as an empty STRING (not JSON null) when branch protection requires
// a PR but zero approvals, so empty string is equally no-decision, never an
// invalid enum. Unknown non-empty vocabulary is invalid external state and is
// rejected, never folded into either outcome.
func normalizeReviewDecision(raw *string) (bool, error) {
	if raw == nil || *raw == "" {
		return false, nil
	}
	switch *raw {
	case "APPROVED":
		return true, nil
	case "REVIEW_REQUIRED", "CHANGES_REQUESTED":
		return false, nil
	default:
		return false, errEnum("unrecognized pull-request reviewDecision enum")
	}
}
```

The doc comment's last sentence deliberately changes "Unknown non-null" to "Unknown non-empty" — with the guard in place, the `default` arm is reachable only for non-empty strings, and the comment must say what the code does.

- [ ] **Step 4: Run the package tests to verify the fix and the fail-closed twin**

Run: `go test -count=1 ./internal/githubcli/ -v -run 'TestViewPullRequest|TestVersionExcludesReviewDecision|TestStandardFieldSetExcludesReviewDecision'`
Expected: PASS — including `TestViewPullRequestEmptyReviewDecisionIsNoDecision` (the fix), `TestViewPullRequestUnknownReviewDecisionFailsClosed` (DISMISSED still errors with `KindInvalidState`), and `TestViewPullRequestReviewDecisionMapping` (APPROVED/REVIEW_REQUIRED/CHANGES_REQUESTED/null unchanged).

Then run the full Go package set: `go test -count=1 ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/githubcli/pr.go internal/githubcli/probe_test.go
git commit -m "fix(githubcli): treat empty-string reviewDecision as no-decision"
```

---

## Build gate

After the task completes, docket-build's suite gate runs the whole suite via `scripts/run-tests.sh` (the command `finalize.test_command` resolves to) — never only the tests enumerated above.

## Post-merge (outside this plan's scope, recorded for the human loop)

- Rebuild the installed binary after merge: `docket development install --source /Users/homer/dev/docket`.
- Re-run `docket-finalize-change 356` once the rebuilt binary is installed.
