<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0348 — Enrich the exact-PR view with reviewDecision so open-PR snapshots populate Approved](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0348-enrich-open-pr-view-with-review-decision.md)**
<!-- docket:backlink:end -->
# Enrich Exact-PR View With reviewDecision Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `githubcli.ViewPullRequest` request GitHub's nullable `reviewDecision` and surface it as `PullRequest.Approved`, so `githubFinalizeProber` populates `domain.PRFacts.Approved` for open PRs and the existing approval gate finally sees real approval state.

**Architecture:** Only the exact-number view gains the field — a new view-specific `--json` field set (`prJSONFields + ",reviewDecision"`) used solely by `ViewPullRequest`. The shared decoder (`prViewJSON` → `toPullRequest`) grows one nullable field with a strict mapping (`APPROVED`→true; `REVIEW_REQUIRED`/`CHANGES_REQUESTED`/null/absent→false; unknown non-null→typed invalid-state failure). `Approved` stays out of `computeVersion` (the write-CAS token). `githubFinalizeProber.ProbePR` copies `pr.Approved` into `domain.PRFacts.Approved`. No domain predicate changes; stale "production can never populate approval" comments are removed.

**Tech Stack:** Go (stdlib only), existing `fakegh_test.go` scripted-arm fixture, `scripts/run-tests.sh` full-suite gate.

**Spec:** `docs/superpowers/specs/2026-08-26-enrich-open-pr-view-with-review-decision-design.md`

## Global Constraints

- Strict enum posture: unknown non-null `reviewDecision` is `KindInvalidState`, never folded to either boolean — matching `normalizeState`'s existing posture in `internal/githubcli/pr.go`.
- `Approved` must NOT enter `computeVersion` — the CAS token must be identical whether a snapshot came from the enriched view or a standard list/create/edit read.
- The standard `prJSONFields` constant is unchanged; only `ViewPullRequest` requests review state.
- No changes to exact-PR-number identity (ADR-0097), PR location, classify precedence, or approval-gate policy. Identity still outranks approval.
- Every mutation probe and re-verification runs with `-count=1` — Go's test cache serves stale passes otherwise (learning: cached-runner-serves-a-mutated-tree). A bare `go test` is never evidence.
- `gofmt -l` must be clean on every touched `.go` file before each commit.
- Final gate: run the whole suite via `scripts/run-tests.sh` (the resolved `finalize.test_command`), never only the focused Go packages.

---

### Task 1: Adapter — view-specific field set, strict reviewDecision decode, Approved on the snapshot

**Files:**
- Modify: `internal/githubcli/ensure.go` (the `prJSONFields` const block, ~line 48)
- Modify: `internal/githubcli/pr.go` (`PullRequest`, `prViewJSON`, `toPullRequest`, `computeVersion` doc comment)
- Modify: `internal/githubcli/probe.go` (`ViewPullRequest`'s `--json` argument, ~line 44)
- Test: `internal/githubcli/probe_test.go`

**Interfaces:**
- Consumes: existing `prJSONFields`, `prViewJSON`, `toPullRequest`, `newFailure`, `errEnum`, fake fixture (`newFakeClient`, `fakeScenario`, `fakeArm`, `ensPRJSON`, `ensRepoSpec`, `ensHead`, `ensHeadOid`, `ensBase`, `ensTitle`, `ensBody`, `probeRepo`).
- Produces: `PullRequest.Approved bool` (Task 2 reads it), package const `prViewJSONFields = prJSONFields + ",reviewDecision"`, unexported `normalizeReviewDecision(raw *string) (bool, error)`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/githubcli/probe_test.go`. Extend the file's import block to `context`, `encoding/json`, `fmt`, `strings`, `testing` (it currently imports only `context` and `testing`).

```go
// strPtr returns a pointer to s, for nullable fixture fields.
func strPtr(s string) *string { return &s }

// probePRJSONWithDecision renders one PR view object in gh's nested shape with
// an explicit reviewDecision: a string value, or JSON null when decision is nil.
// ensPRJSON deliberately stays decision-free — it feeds the standard-field
// list/create/edit tests, whose absent-field decode this change must preserve.
func probePRJSONWithDecision(number int, state string, decision *string) string {
	m := map[string]any{
		"number":      number,
		"url":         fmt.Sprintf("https://github.com/acme/widget/pull/%d", number),
		"state":       state,
		"isDraft":     false,
		"headRefName": ensHead,
		"headRefOid":  ensHeadOid,
		"baseRefName": ensBase,
		"title":       ensTitle,
		"body":        ensBody,
	}
	if decision == nil {
		m["reviewDecision"] = nil
	} else {
		m["reviewDecision"] = *decision
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// TestViewPullRequestRequestsReviewDecision pins the exact --json field set the
// exact-number view sends, as ONE LITERAL STRING. Matching against the
// prViewJSONFields constant instead would stay green if the constant silently
// lost the field (defaulted-param-hides-caller-wiring); the fake answers only a
// matching argv, so a view that requests anything else errors here.
func TestViewPullRequestRequestsReviewDecision(t *testing.T) {
	doc := probePRJSONWithDecision(7, "OPEN", strPtr("APPROVED"))
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		{ArgvPrefix: []string{"pr", "view", "7", "--repo", ensRepoSpec, "--json",
			"number,url,state,isDraft,headRefName,headRefOid,baseRefName,title,body,reviewDecision"}, Stdout: doc, Exit: 0},
	}})
	pr, err := c.ViewPullRequest(context.Background(), probeRepo(), 7)
	if err != nil {
		t.Fatalf("ViewPullRequest: %v", err)
	}
	if !pr.Approved {
		t.Errorf("Approved = false, want true for reviewDecision APPROVED")
	}
}

// TestViewPullRequestReviewDecisionMapping: the strict mapping — only APPROVED
// is true; REVIEW_REQUIRED, CHANGES_REQUESTED, and JSON null are false.
func TestViewPullRequestReviewDecisionMapping(t *testing.T) {
	cases := []struct {
		name     string
		decision *string
		want     bool
	}{
		{"approved", strPtr("APPROVED"), true},
		{"review-required", strPtr("REVIEW_REQUIRED"), false},
		{"changes-requested", strPtr("CHANGES_REQUESTED"), false},
		{"null", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := probePRJSONWithDecision(7, "OPEN", tc.decision)
			c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
				{ArgvPrefix: []string{"pr", "view", "7"}, Stdout: doc, Exit: 0},
			}})
			pr, err := c.ViewPullRequest(context.Background(), probeRepo(), 7)
			if err != nil {
				t.Fatalf("ViewPullRequest: %v", err)
			}
			if pr.Approved != tc.want {
				t.Errorf("Approved = %v, want %v", pr.Approved, tc.want)
			}
		})
	}
}

// TestViewPullRequestUnknownReviewDecisionFailsClosed: unknown non-null
// vocabulary is invalid external state — a typed invalid-state Failure and the
// zero PR, never a silently chosen boolean.
func TestViewPullRequestUnknownReviewDecisionFailsClosed(t *testing.T) {
	doc := probePRJSONWithDecision(7, "OPEN", strPtr("DISMISSED"))
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		{ArgvPrefix: []string{"pr", "view", "7"}, Stdout: doc, Exit: 0},
	}})
	pr, err := c.ViewPullRequest(context.Background(), probeRepo(), 7)
	if err == nil {
		t.Fatalf("unknown reviewDecision decoded cleanly; want typed invalid-state failure")
	}
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("error is not a typed *Failure: %v", err)
	}
	if f.Kind != KindInvalidState {
		t.Errorf("Kind = %v, want KindInvalidState", f.Kind)
	}
	if pr != (PullRequest{}) {
		t.Errorf("returned PR is not the zero value alongside the error")
	}
}

// TestVersionExcludesReviewDecision: the write-CAS token must not depend on
// review state — the same PR yields one token whether it arrived approved via
// the exact view or decision-free via a standard read. The Approved inequality
// assert keeps the fixture honest: if both documents decoded to the same
// Approved, equal versions would prove nothing.
func TestVersionExcludesReviewDecision(t *testing.T) {
	approved, err := decodePullRequest("probe", []byte(probePRJSONWithDecision(7, "OPEN", strPtr("APPROVED"))))
	if err != nil {
		t.Fatalf("decode approved: %v", err)
	}
	plain, err := decodePullRequest("probe", []byte(probePRJSONWithDecision(7, "OPEN", nil)))
	if err != nil {
		t.Fatalf("decode plain: %v", err)
	}
	if approved.Approved == plain.Approved {
		t.Fatalf("fixture vacuous: both documents decode to Approved=%v", approved.Approved)
	}
	if approved.Version != plain.Version {
		t.Errorf("Version differs on review state alone:\n approved %s\n plain    %s", approved.Version, plain.Version)
	}
}

// TestStandardFieldSetExcludesReviewDecision: only the exact-number view widens.
// The standard list/create/edit set must not gain review state, and the view
// set must be exactly the standard set plus reviewDecision.
func TestStandardFieldSetExcludesReviewDecision(t *testing.T) {
	if strings.Contains(prJSONFields, "reviewDecision") {
		t.Fatalf("prJSONFields gained reviewDecision; only ViewPullRequest requests review state")
	}
	if prViewJSONFields != prJSONFields+",reviewDecision" {
		t.Fatalf("prViewJSONFields = %q, want prJSONFields+%q", prViewJSONFields, ",reviewDecision")
	}
}
```

Also extend the existing `TestViewPullRequestByNumber` (absent-field posture — its `ensPRJSON` document has no `reviewDecision` key): after the `HeadBranch` assert, add:

```go
	if pr.Approved {
		t.Errorf("Approved = true for a view response with no reviewDecision field; absent must read false")
	}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd /Users/homer/dev/docket/.worktrees/enrich-open-pr-view-with-review-decision && go test -count=1 ./internal/githubcli/ -run 'TestViewPullRequest|TestVersionExcludesReviewDecision|TestStandardFieldSetExcludesReviewDecision' -v`
Expected: compile FAILURE — `pr.Approved`, `prViewJSONFields` undefined. That compile error is the red; do not weaken the tests to dodge it.

- [ ] **Step 3: Implement the adapter enrichment**

In `internal/githubcli/ensure.go`, directly below the `prJSONFields` const (keep that const byte-identical), add:

```go
// prViewJSONFields is the exact-number view's field set: the standard
// prJSONFields plus GitHub's nullable reviewDecision. Only ViewPullRequest
// requests review state — the list/create/edit paths keep the standard set, so
// their snapshots and write-CAS versions are untouched by review activity.
const prViewJSONFields = prJSONFields + ",reviewDecision"
```

In `internal/githubcli/pr.go`:

1. Add to the `PullRequest` struct, after `Body string`:

```go
	Approved   bool   // reviewDecision == APPROVED on an enriched exact view; always false from standard-field reads
```

2. Add to `prViewJSON`, after the `Body` field:

```go
	// ReviewDecision is requested only by the exact-number view
	// (prViewJSONFields). Absent (standard field set) and JSON null (no
	// decision yet) are both nil and both map to unapproved.
	ReviewDecision *string `json:"reviewDecision"`
```

3. Add the mapping function next to `normalizeState`:

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

4. In `toPullRequest`, after the `normalizeState` block and before the `pr := PullRequest{...}` literal:

```go
	approved, err := normalizeReviewDecision(raw.ReviewDecision)
	if err != nil {
		return PullRequest{}, newFailure(op, StageDecode, KindInvalidState, err.Error(), err)
	}
```

and add `Approved:   approved,` to the `PullRequest` literal (after `Body`).

5. `computeVersion` body stays untouched. Extend its doc comment — after the sentence ending "URL is server-assigned and excluded.", add:

```go
// Approved is deliberately excluded too: review state is view-only, read-only
// gate evidence — the standard list/create/edit snapshots never request it, and
// including it would give the same PR incompatible tokens depending on which
// read shape produced the snapshot. Finalize reloads review state directly
// before effects rather than authorizing a review mutation through this token.
```

In `internal/githubcli/probe.go`, in `ViewPullRequest`'s `runRequest` args, change `"--json", prJSONFields,` to `"--json", prViewJSONFields,`. Extend the function's doc comment: after "It reuses decodePullRequest: one JSON interpretation, never a second.", add "It alone requests reviewDecision (prViewJSONFields), so the snapshot's Approved reflects GitHub's review decision."

- [ ] **Step 4: Run the package tests and verify green**

Run: `cd /Users/homer/dev/docket/.worktrees/enrich-open-pr-view-with-review-decision && go test -count=1 ./internal/githubcli/ && gofmt -l internal/githubcli/`
Expected: `ok`, and `gofmt -l` prints nothing.

- [ ] **Step 5: Mutation-probe the new guards**

Each probe: apply the mutation, run `go test -count=1 ./internal/githubcli/`, confirm the named test goes RED, then restore the edit exactly (revert your own edit by hand — never `git checkout -- <file>`, which restores to HEAD and destroys the uncommitted implementation; learning: mutation-restore-needs-a-backup-copy). Treat any `(cached)` output as no evidence.

1. In `probe.go`, change `prViewJSONFields` back to `prJSONFields` → `TestViewPullRequestRequestsReviewDecision` must fail (fake arm unmatched → error).
2. In `normalizeReviewDecision`, change the `default` branch to `return false, nil` → `TestViewPullRequestUnknownReviewDecisionFailsClosed` must fail.
3. In `normalizeReviewDecision`, change `case "APPROVED": return true, nil` to `return false, nil` → the mapping test's `approved` case and `TestVersionExcludesReviewDecision`'s vacuity assert must fail.

All three probes must redden; restore and re-run to green after each.

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/enrich-open-pr-view-with-review-decision
git add internal/githubcli/ensure.go internal/githubcli/pr.go internal/githubcli/probe.go internal/githubcli/probe_test.go
git commit -m "feat(githubcli): enrich exact-PR view with reviewDecision, expose Approved"
```

---

### Task 2: Finalize prober propagation and stale-comment cleanup

**Files:**
- Modify: `internal/app/finalize_context.go` (`githubFinalizeProber` doc comment "Field coverage" paragraph; `ProbePR`'s open/closed return literal)
- Modify: `internal/domain/finalize.go` (`classifyFinalize` doc comment)
- Modify: `internal/domain/finalize_test.go` (`TestSelectFinalizeQueueIdentityBeforeApproval` doc comment)
- Test: `internal/app/finalize_context_test.go`

**Interfaces:**
- Consumes: Task 1's `githubcli.PullRequest.Approved bool`; existing `fakeProberGitHub`, `proberRepo()`, `prRefFor(int)`, `NewGitHubFinalizeProber`.
- Produces: `domain.PRFacts.Approved` populated from the exact view for open/closed PRs. No signature changes anywhere.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/finalize_context_test.go`, next to `TestProbePRReadsExactNumber`:

```go
// TestProbePRPropagatesApproval: the exact view's Approved observation flows
// into PRFacts for a non-merged PR — true and false both propagate. The true
// leg is the mutation seam: deleting the Approved copy in ProbePR must redden
// it (a zero-value PRFacts already reads false, so only true discriminates).
func TestProbePRPropagatesApproval(t *testing.T) {
	for _, approved := range []bool{true, false} {
		gh := &fakeProberGitHub{
			repo: proberRepo(),
			views: map[int]githubcli.PullRequest{
				7: {Number: 7, State: githubcli.StateOpen, Approved: approved, HeadBranch: "feature/head", HeadCommit: "h7", BaseBranch: "main", Version: "v7"},
			},
		}
		facts, err := NewGitHubFinalizeProber(gh).ProbePR(context.Background(), "", prRefFor(7))
		if err != nil {
			t.Fatalf("ProbePR(approved=%v): %v", approved, err)
		}
		if facts.Approved != approved {
			t.Errorf("PRFacts.Approved = %v, want %v", facts.Approved, approved)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/enrich-open-pr-view-with-review-decision && go test -count=1 ./internal/app/ -run TestProbePRPropagatesApproval -v`
Expected: FAIL on the `approved=true` leg — `PRFacts.Approved = false, want true`.

- [ ] **Step 3: Implement propagation and clean the stale comments**

In `internal/app/finalize_context.go`:

1. In `ProbePR`'s final (non-merged) return literal, add `Approved:   pr.Approved,` alongside `Draft: pr.Draft`.
2. Replace the "Field coverage" paragraph of the `githubFinalizeProber` doc comment:

Old:
```go
// Field coverage: the current githubcli probes do not carry a PR's review
// decision, mergeability, or diff size for an OPEN pull request, so those fields
// (Approved, Mergeable, ChangedFiles, DiffLines) stay zero for an open PR — a
// conservative reading (an open PR bands as UNKNOWN mergeability and unapproved)
// that never over-permits. The merged path is fully faithful.
```

New:
```go
// Field coverage: the exact-number view carries reviewDecision, so Approved
// reflects GitHub's actual review decision for a non-merged PR. The probes
// still do not carry mergeability or diff size for an OPEN pull request, so
// those fields (Mergeable, ChangedFiles, DiffLines) stay zero — a conservative
// reading (an open PR bands as UNKNOWN mergeability) that never over-permits.
// The merged path is fully faithful.
```

In `internal/domain/finalize.go`, in the `classifyFinalize` doc comment, replace:

```go
// head mismatch outranks approval — identity is more fundamental, and githubcli's
// open-PR view omits reviewDecision so Approved is never true in production;
// anything surviving is an actionable open PR banded by mergeability. Identity is
```

with:

```go
// head mismatch outranks approval — identity is more fundamental than the
// approval observation, whatever the observed decision;
// anything surviving is an actionable open PR banded by mergeability. Identity is
```

In `internal/domain/finalize_test.go`, replace `TestSelectFinalizeQueueIdentityBeforeApproval`'s doc comment. The test body is untouched — its premise (identity ordering) survives this change; only the false claim about production goes (learning: test-premise-deleted-not-regated — the block guards the classify ordering, not the retired field gap).

Old:
```go
// TestSelectFinalizeQueueIdentityBeforeApproval pins the production shape: an
// open PR whose exact head disagrees with the recorded branch surfaces
// branch-pr-head-mismatch even when Approved is false — the shape githubcli's
// open-PR view always yields, since prJSONFields omits reviewDecision so
// PRFacts.Approved is never populated true in production. Identity is more
// fundamental than approval and must be reconciled before the approval gate;
// before the classify reorder this masked the mismatch as approval-required and
// never routed to the repair checkpoint. The head is observed (non-empty), so
// the facts are cleanly observed and identity applies.
```

New:
```go
// TestSelectFinalizeQueueIdentityBeforeApproval pins the classify ordering: an
// open PR whose exact head disagrees with the recorded branch surfaces
// branch-pr-head-mismatch even when Approved is false. The Approved: false
// input is an intentional unapproved fixture — production now observes real
// approval via the exact view's reviewDecision — and identity must still win
// for an unapproved PR. Identity is more fundamental than approval and must be
// reconciled before the approval gate; before the classify reorder this masked
// the mismatch as approval-required and never routed to the repair checkpoint.
// The head is observed (non-empty), so the facts are cleanly observed and
// identity applies.
```

Then sweep for any remaining stale claim: `grep -rn "never true in production\|never populated true\|omits reviewDecision" internal/ --include="*.go"` must return nothing.

- [ ] **Step 4: Run the tests and verify green**

Run: `cd /Users/homer/dev/docket/.worktrees/enrich-open-pr-view-with-review-decision && go test -count=1 ./internal/app/ ./internal/domain/ && gofmt -l internal/app internal/domain`
Expected: both `ok`, `gofmt -l` prints nothing.

Confirm (read, don't rewrite) that the spec's remaining domain obligations are already pinned by existing tests: an approved open PR passing the approval conjunct (`internal/domain/finalize_test.go` — the `Approved: true, Mergeable: "MERGEABLE"` fixtures banding actionable, e.g. the fixtures near the top of the file) and `Approved: false` still yielding `approval-required` (the fixture commented `// approval-required`). If either is genuinely absent, add it as a table case beside its neighbors; do not duplicate coverage that exists.

- [ ] **Step 5: Mutation-probe the population seam**

Delete `Approved:   pr.Approved,` from `ProbePR`, run `go test -count=1 ./internal/app/`, confirm `TestProbePRPropagatesApproval` goes RED, restore the line by hand, re-run to green. `(cached)` output is no evidence.

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/enrich-open-pr-view-with-review-decision
git add internal/app/finalize_context.go internal/app/finalize_context_test.go internal/domain/finalize.go internal/domain/finalize_test.go
git commit -m "feat(finalize): propagate exact-view Approved into PRFacts; retire stale never-approved comments"
```

---

### Task 3: Full-suite gate

**Files:**
- No source changes. Runs the configured build gate.

**Interfaces:**
- Consumes: Tasks 1–2 committed on the branch.
- Produces: a green whole-suite run — the evidence record for review.

- [ ] **Step 1: Run the whole suite through the build gate**

Run `scripts/run-tests.sh` from the feature worktree root (this is `finalize.test_command`; docket-build's gate driver applies — a forked worker drives it via inline blocking slices, never backgrounds-and-yields).

Expected: PASS. Read the output for `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` (screening findings to note in the results evidence) and `SERIAL CONFIRMED OVER BUDGET:` (an authoritative breach to act on) — nothing else surfaces these.

- [ ] **Step 2: Fix anything red, then re-run to a clean pass**

A red here is a defect in Tasks 1–2 or a real interaction — use superpowers:systematic-debugging; never weaken an existing test to green. Amend or add a commit on the same branch as appropriate, and re-run the gate to a full clean pass.

---

## Self-Review (performed at plan time)

- **Spec coverage:** field-set widening + strict mapping + typed unknown failure (Task 1); absent-field/standard-caller preservation (Task 1 Step 1 `TestViewPullRequestByNumber` extension + `TestStandardFieldSetExcludesReviewDecision`); Version exclusion + doc update (Task 1); prober propagation + PRFacts (Task 2); stale-comment removal + regression-test reframing (Task 2); mutation of the population seam (Task 2 Step 5); full-suite gate (Task 3). Merged reprobe path untouched, per spec.
- **Types:** `normalizeReviewDecision(raw *string) (bool, error)`; `PullRequest.Approved bool`; `prViewJSONFields` const — spelled identically in every task.
- **No placeholders remain.**
