<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0347 — Honor recorded branch names instead of reconstructing feat/<slug>](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-26-0347-honor-recorded-branch-over-feat-prefix.md)**
<!-- docket:backlink:end -->
# Recorded Branch Identity Implementation Plan (change 0347)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the recorded `branch:` field the feature-head source of truth for every post-claim operation, mint `<type>/<slug>` (or `<branch_prefix>/<slug>`) at claim, and identify finalize's PR by its exact recorded number instead of head discovery — with fail-closed identity checkpoints instead of silent reconstruction.

**Architecture:** One new domain constructor (`MintBranch`) is the only place a branch name is ever built; it fires exactly at claim (plus reclaim's read-only orphan probe). `workspace.NewTarget` gains an explicit validated feature-branch parameter so a target not fed from the record becomes unrepresentable; every app-layer caller supplies the record's `branch:` through one shared `recordedBranch` helper that fails closed. Finalize's prober drops `FindOpenPullRequestsByHead` for an exported exact-number `ViewPullRequest`, compares the PR's reported head with the recorded branch, and surfaces structured identity verdicts that a new version-pinned `docket change repair-identity` operation (and the interactive finalize skill) resolve.

**Tech Stack:** Go (internal/domain, internal/app, internal/workspace, internal/githubcli, internal/repository, internal/render, internal/cli), `gh` CLI adapter with fakes, bash sentinel/guard tests under `tests/`.

**Spec:** `docs/superpowers/specs/2026-08-25-recorded-branch-identity-design.md` (metadata branch `docket`). The change file is `docs/changes/active/0347-honor-recorded-branch-over-feat-prefix.md`.

## Global Constraints

- **Mint once, record once, consume the record.** After this change, `domain.BranchForSlug` no longer exists; the only branch construction is `domain.MintBranch`, called at claim and in reclaim's read-only orphan detection. No post-claim path may rebuild a name from slug/type/prefix.
- **Unresolved identity authorizes no effects.** A post-claim operation seeing an absent/empty/malformed `branch:` returns invalid state and mutates nothing (spec: "A post-claim operation outside interactive finalize that sees an absent, empty, or invalid `branch:` returns invalid state and performs no mutation").
- **A probe has three outcomes.** Present, cleanly absent, unknown. "No failure is translated into `pr-closed` unless the exact numbered PR was cleanly observed closed and unmerged." A transport/decode failure stays `unknown` (learnings: probe-error-is-not-clean-absence).
- **No repository-wide prefix configuration, no new lifecycle state, no branch renames or workspace migration** (spec Out of scope).
- Go mutation probes and manual re-verification always run with `-count=1` (Go's test cache otherwise serves a green result against a mutated tree).
- The final gate runs the whole suite via whatever `finalize.test_command` resolves to (read it from config, never a second copy).
- Comment cross-references anchor on symbol names or verbatim-quoted clauses, never line numbers (`tests/test_comment_anchor_style.sh` enforces part of this).
- Every guard added here must be mutation-tested in the deletion direction as well as the addition direction (strip the guarded thing, watch it redden).

---

### Task 1: Domain mint constructor, `branch_prefix` field, and claim-time construction

**Files:**
- Modify: `internal/domain/types.go` (replace `BranchForSlug` body area with `ValidBranchComponent` + `MintBranch`; keep `BranchForSlug` in place until Task 4 deletes it — other files still call it and must keep compiling)
- Modify: `internal/domain/entities.go` (ChangeSpec gains `BranchPrefix OptionalString` + accessor)
- Modify: `internal/domain/actions.go` (`Claim` mints from type/prefix)
- Modify: `internal/repository/decode.go` (wire `branch_prefix`)
- Modify: `internal/render/record.go` (new-record frontmatter gains `branch_prefix`)
- Test: `internal/domain/types_test.go`, `internal/domain/actions_test.go`, plus the existing decode/render test files beside the modified sources

**Interfaces:**
- Produces: `domain.ValidBranchComponent(s string) bool`; `domain.MintBranch(typeToken string, prefix OptionalString, slug string) string`; `(Change).BranchPrefix() OptionalString`. Claim failure reason string `"invalid-branch-component"`.
- Consumes: existing `OptionalString{State, Value}`, `FieldPresent`, `newFailure`, `ValidSlugToken`, `ValidTypeToken`.

- [ ] **Step 1: Write failing domain tests**

In `internal/domain/types_test.go`:

```go
func TestValidBranchComponent(t *testing.T) {
	valid := []string{"feat", "fix", "chore", "hotfix", "feature", "a", "release-2"}
	invalid := []string{
		"", "feat/", "refs", "refs/heads", "a/b", "-lead", ".lead",
		"has space", "has\ttab", "a..b", "a@{b", "a~b", "a^b", "a:b",
		"a?b", "a*b", "a[b", "a\\b", "end.lock", "end.",
	}
	for _, s := range valid {
		if !ValidBranchComponent(s) { t.Errorf("ValidBranchComponent(%q) = false, want true", s) }
	}
	for _, s := range invalid {
		if ValidBranchComponent(s) { t.Errorf("ValidBranchComponent(%q) = true, want false", s) }
	}
}

func TestMintBranch(t *testing.T) {
	if got := MintBranch("fix", OptionalString{}, "my-slug"); got != "fix/my-slug" {
		t.Fatalf("type mint = %q, want fix/my-slug", got)
	}
	if got := MintBranch("fix", OptionalString{State: FieldPresent, Value: "hotfix"}, "my-slug"); got != "hotfix/my-slug" {
		t.Fatalf("prefix mint = %q, want hotfix/my-slug", got)
	}
	// A present-but-empty prefix falls back to the type.
	if got := MintBranch("chore", OptionalString{State: FieldPresent, Value: ""}, "s"); got != "chore/s" {
		t.Fatalf("empty prefix mint = %q, want chore/s", got)
	}
}
```

In `internal/domain/actions_test.go`, add claim cases following the file's existing fixture pattern for building a proposed `Change` (copy the construction the current `Claim` tests use):

- `TestClaimMintsTypePrefixedBranch`: a proposed change of type `fix` claims to `branch: fix/<slug>`; type `chore` claims to `chore/<slug>` (table over at least two types — spec: "Each built-in change type mints `<type>/<slug>`", and one non-built-in token such as `spike` for "a configured custom type behaves identically").
- `TestClaimHonorsBranchPrefixOverride`: change with `branch_prefix: hotfix` and type `fix` claims to `hotfix/<slug>`.
- `TestClaimRejectsInvalidBranchComponent`: change whose `branch_prefix` is `a/b` (and a second case whose *type* token is not a valid component, e.g. `bad type` injected directly into the spec fixture) fails with `FailInvalidInput` and reason `invalid-branch-component`, and no fields change.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/domain/ -run 'TestValidBranchComponent|TestMintBranch|TestClaim' -count=1`
Expected: FAIL — `ValidBranchComponent`/`MintBranch` undefined, claim mints `feat/<slug>`.

- [ ] **Step 3: Implement domain changes**

In `internal/domain/types.go`, next to `BranchForSlug` (do not delete it yet):

```go
// ValidBranchComponent reports whether s is usable as the single path
// component ahead of "/<slug>" in a minted branch name: non-empty, exactly one
// component (no slash, so never ref-qualified), and legal under the same
// shape rules workspace's validBranchRef applies to a full ref.
func ValidBranchComponent(s string) bool {
	if s == "" || strings.Contains(s, "/") {
		return false
	}
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, ".") {
		return false
	}
	if strings.HasSuffix(s, ".lock") || strings.HasSuffix(s, ".") {
		return false
	}
	if strings.Contains(s, "..") || strings.Contains(s, "@{") {
		return false
	}
	if strings.ContainsAny(s, " \t\r\n\v\f~^:?*[\\") || strings.IndexByte(s, 0) >= 0 {
		return false
	}
	return true
}

// MintBranch constructs the full feature-branch name a claim records:
// (branch_prefix when present and non-empty, otherwise the change type) +
// "/" + slug. This is the ONLY branch-name constructor; every post-claim
// operation consumes the recorded branch: field instead.
func MintBranch(typeToken string, prefix OptionalString, slug string) string {
	p := typeToken
	if prefix.State == FieldPresent && prefix.Value != "" {
		p = prefix.Value
	}
	return p + "/" + slug
}
```

(`types.go` may need `"strings"` added to its imports.)

In `internal/domain/entities.go`: add `BranchPrefix OptionalString` to the change spec struct beside `Branch`, and beside the `Branch()` accessor:

```go
// BranchPrefix returns the optional per-change mint-prefix override. It is
// durable human input consumed only at claim time; once branch: is populated
// it is informational and inert.
func (c Change) BranchPrefix() OptionalString { return c.spec.BranchPrefix }
```

In `internal/domain/actions.go` `Claim`, replace `b.setBranch(BranchForSlug(c.Slug()))` with component validation + mint:

```go
	component := c.Type()
	if p := c.BranchPrefix(); p.State == FieldPresent && p.Value != "" {
		component = p.Value
	}
	if !ValidBranchComponent(component) {
		return ActionResult{}, newFailure(c, FailInvalidInput, "invalid-branch-component", map[string]string{
			"component": component,
		})
	}
	b.setBranch(MintBranch(c.Type(), c.BranchPrefix(), c.Slug()))
```

Update `Claim`'s doc comment: it no longer records "the deterministic feat/<slug> branch" — it records "the minted `<type>/<slug>` branch, or `<branch_prefix>/<slug>` when the optional per-change override is present".

In `internal/repository/decode.go`: add `BranchPrefix scalar `yaml:"branch_prefix"`` to the wire struct beside `Branch`, and `spec.BranchPrefix = d.optionalString("branch_prefix", wire.BranchPrefix)` beside the existing branch line.

In `internal/render/record.go`: insert `{Name: "branch_prefix", Value: document.Null()},` immediately before the `branch` entry in the new-change frontmatter field list.

- [ ] **Step 4: Fix reddened neighbors, run package tests**

Run: `go test ./internal/domain/ ./internal/repository/ ./internal/render/ -count=1`
Existing claim-path tests asserting `feat/<slug>` and render golden fixtures asserting the exact frontmatter field list will redden — update those expectations to the new mint rule / field list (this is the intended behavior change, not test weakening). Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -u internal/domain internal/repository internal/render
git commit -m "feat(domain): mint <type>/<slug> branches with optional branch_prefix override"
```

---

### Task 2: Reclaim's conservative orphan detection and prefix survival

**Files:**
- Modify: `internal/domain/lease.go` (`blockingBranch` probes the fresh-mint candidate, not `feat/<slug>`)
- Modify: `internal/app/change_reclaim.go` (the candidate list around `candidates = append(candidates, domain.BranchForSlug(c.Slug()))` uses `domain.MintBranch`)
- Test: `internal/domain/lease_test.go`, `internal/app/change_reclaim_test.go`

**Interfaces:**
- Consumes: `domain.MintBranch`, `(Change).BranchPrefix()` from Task 1.
- Produces: no new API. Reclaim remains "a deliberate exception to the no-reconstruction consumer rule": read-only probing only, never a name selected for a Git mutation (spec, "Claim-time construction").

- [ ] **Step 1: Write failing tests**

In `internal/domain/lease_test.go` (follow the file's existing fixture pattern for `BranchFacts`):

- `TestBlockingBranchProbesMintCandidate`: an in-progress change of type `fix` with **no** recorded branch and a live remote branch `fix/<slug>` is blocked (BlockingBranch = `fix/<slug>`); with `branch_prefix: hotfix` and live `hotfix/<slug>` it is blocked on that name; a live `feat/<slug>` alone (type `fix`, no prefix) does **not** block.
- `TestBlockingBranchPrefersRecorded`: recorded `feature/other-name` live → BlockingBranch is the recorded name even when the mint candidate is also live (existing behavior, keep it pinned).
- `TestReclaimPreservesBranchPrefix`: reclaiming a branchless expired claim back to proposed leaves `BranchPrefix()` exactly as it was (spec: "Reclaim preserves `branch_prefix:` when returning a branchless expired claim to `proposed`"). Assert on the resulting change's field, following how the file asserts `Branch()` was cleared.

In `internal/app/change_reclaim_test.go`: extend the existing candidate-probe test so a change of type `fix` probes `fix/<slug>` (not `feat/<slug>`) as its fresh-mint candidate alongside the recorded name.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/domain/ -run 'TestBlockingBranch|TestReclaim' -count=1 && go test ./internal/app/ -run Reclaim -count=1`
Expected: FAIL — probes still use `feat/<slug>`.

- [ ] **Step 3: Implement**

`internal/domain/lease.go` `blockingBranch`: replace the `BranchForSlug` line with

```go
	if minted := MintBranch(c.Type(), c.BranchPrefix(), c.Slug()); facts.HasBranch(minted) {
		return minted
	}
```

and update `EvaluateReclaim`'s doc comment: "…neither the branch it recorded nor the branch a fresh claim would mint from type/branch_prefix/slug exists among the supplied facts."

`internal/app/change_reclaim.go`: replace `domain.BranchForSlug(c.Slug())` with `domain.MintBranch(c.Type(), c.BranchPrefix(), c.Slug())`.

`branch_prefix` survival needs no code: `Reclaim`'s builder only calls `setBranch("")`/`clearClaimedAt()`/`setReconciled(false)` — the test pins that this stays true.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/domain/ ./internal/app/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -u internal/domain internal/app
git commit -m "fix(reclaim): probe the fresh-mint candidate and preserve branch_prefix"
```

---

### Task 3: `workspace.NewTarget` takes the recorded branch; app callers supply it fail-closed

**Files:**
- Modify: `internal/workspace/target.go` (new `featureBranch` parameter + `FeatureBranch()` method; struct doc comment)
- Modify: `internal/workspace/prepare.go`, `internal/workspace/inspect.go`, `internal/workspace/publish.go`, `internal/workspace/cleanup.go` (re-derivations pass `t.FeatureBranch()`)
- Create: `internal/app/branch_identity.go` (shared `recordedBranch` helper)
- Modify: `internal/app/workspace_ops.go`, `internal/app/finalize_block.go`, `internal/app/finalize_merge.go`, `internal/app/finalize_cleanup.go` (both `NewTarget` sites) — each supplies the recorded branch and refuses when the helper errors
- Test: `internal/workspace/target_test.go`, `internal/app/branch_identity_test.go`, existing tests beside the modified app files

**Interfaces:**
- Produces:
  - `workspace.NewTarget(id domain.ChangeID, slug string, base domain.EffectiveBase, featureBranch string) (Target, error)` — rejects empty, `refs/`-qualified, or malformed `featureBranch` with the existing `invalidTarget` failure.
  - `(workspace.Target).FeatureBranch() string` — the short branch, `strings.TrimPrefix(string(t.FeatureRef), "refs/heads/")`.
  - `app.recordedBranch(c domain.Change) (string, error)` in `internal/app/branch_identity.go`:

```go
// recordedBranch returns c's recorded feature branch. It fails closed:
// an absent or empty branch: on a post-claim record is errBranchMissing,
// a value that cannot be a branch ref is errBranchMalformed. Callers map
// the error to their own invalid-state refusal and perform NO mutation.
var errBranchMissing = errors.New("branch-missing")
var errBranchMalformed = errors.New("branch-malformed")

func recordedBranch(c domain.Change) (string, error) {
	b := c.Branch()
	if b.State != domain.FieldPresent || b.Value == "" {
		return "", errBranchMissing
	}
	if strings.HasPrefix(b.Value, "refs/") || strings.HasPrefix(b.Value, "-") ||
		strings.ContainsAny(b.Value, " \t\r\n\v\f") || strings.Contains(b.Value, "@{") ||
		strings.Contains(b.Value, "..") || strings.IndexByte(b.Value, 0) >= 0 {
		return "", errBranchMalformed
	}
	return b.Value, nil
}
```

- Consumes: `c.Branch()` from the change record; Task 1's semantics (claim always records a full name).

- [ ] **Step 1: Write failing tests**

`internal/workspace/target_test.go` (extend the existing `NewTarget` table):

- A valid explicit branch distinct from any derivation — `NewTarget(1, "my-slug", resolvedBase, "feature/other-name")` — yields `FeatureRef == "refs/heads/feature/other-name"` and `FeatureBranch() == "feature/other-name"` (spec Verification: "Workspace targets accept and preserve a valid explicit feature branch distinct from `<type>/<slug>` and `feat/<slug>`").
- Rejections: empty branch, `refs/heads/feature/x` (already qualified), `-lead`, `a..b` — each returns the invalid-input failure and a zero Target.

`internal/app/branch_identity_test.go`: table over `recordedBranch` — present valid value returned; absent → `errBranchMissing`; present-empty → `errBranchMissing`; `refs/heads/x` and `a..b` → `errBranchMalformed`.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/workspace/ -run TestNewTarget -count=1`
Expected: FAIL — compile error (wrong arity) or missing method.

- [ ] **Step 3: Implement**

`internal/workspace/target.go`:
- Add the `featureBranch string` parameter; validate before use:

```go
	if featureBranch == "" {
		return Target{}, invalidTarget("empty feature branch")
	}
	if strings.HasPrefix(featureBranch, "refs/") {
		return Target{}, invalidTarget("feature branch is already ref-qualified")
	}
	featureRef := gitcli.RefName("refs/heads/" + featureBranch)
	if err := validBranchRef(featureRef); err != nil {
		return Target{}, invalidTarget("malformed feature branch")
	}
```

- Set `FeatureRef: featureRef` in the returned Target; delete the `domain.BranchForSlug` derivation.
- Rewrite the struct/constructor doc comments: FeatureRef is "always refs/heads/ + the caller-supplied recorded branch, validated here — never derived from the slug"; the field comment `// refs/heads/feat/<slug>, derived — never caller-supplied` becomes `// refs/heads/<recorded branch>, validated — supplied from the change record`.
- Add `FeatureBranch()`.

Internal re-derivations (`prepare.go`, `inspect.go`, `publish.go`, `cleanup.go`): change each `NewTarget(t.ChangeID, t.Slug, t.Base)` to `NewTarget(t.ChangeID, t.Slug, t.Base, t.FeatureBranch())`.

App callers — for each of `workspace_ops.go:` (the `NewTarget` site in the workspace-context builder), `finalize_block.go`, `finalize_merge.go`, and both sites in `finalize_cleanup.go`:

```go
	branch, berr := recordedBranch(c) // or wc.change / cc.change per site
	if berr != nil {
		return <that path's invalid-state refusal, reason berr.Error(), no mutation>
	}
	target, err := workspace.NewTarget(c.ID(), c.Slug(), base, branch)
```

Each site already has a typed refusal path for an invalid target — route the identity error through the same shape with the reason string from the error (`branch-missing` / `branch-malformed`). Where the surrounding operation would otherwise proceed to a Git/GitHub effect, the refusal must come first.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/workspace/ ./internal/app/ -count=1`
Fix reddened fixtures by supplying recorded branches in test change records (most app fixtures already record `feat/<slug>`-shaped branches from claim fixtures; where a fixture never set `branch:`, that path now correctly refuses — assert the refusal instead of weakening). At least one updated app test must use a deliberately non-derived name like `feature/renamed-head` end-to-end through its operation (spec Verification: "Every post-claim consumer operates on a recorded `feature/...` or `hotfix/...` name").
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/branch_identity.go internal/app/branch_identity_test.go
git add -u internal/workspace internal/app
git commit -m "feat(workspace): feature branch is an explicit validated NewTarget input from the record"
```

---

### Task 4: Retire slug reconstruction at the remaining consumers; delete `BranchForSlug`

**Files:**
- Modify: `internal/app/implementation_context.go`, `internal/app/finalize_context.go` (the report `Branch:` field site), `internal/app/finalize_retarget.go`, `internal/app/finalize_closeout.go`, `internal/app/finalize_merge.go` (child head site), `internal/app/finalize_cleanup.go` (the `featureBranch :=`, child-head, and `featureRef :=` sites)
- Modify: `internal/domain/types.go` (delete `BranchForSlug`)
- Test: existing `_test.go` beside each modified file

**Interfaces:**
- Consumes: `recordedBranch` (Task 3) for the candidate's own branch; for **stack relatives** (parent/children) read that relative's record: `parentBranch` from `parent.Branch()`, `childHead` from `child.Branch()` — each with the same fail-closed rule (spec: "Stack parent and child operations use each record's branch independently").
- Produces: after this task `git grep -n "BranchForSlug"` over `internal/` returns nothing; the function is gone.

- [ ] **Step 1: Derive the full population (do not hand-list)**

Run: `git grep -n 'BranchForSlug' -- 'internal/**/*.go'` and classify every hit: **minting** (`actions.go` — already migrated), **conservative detection** (`lease.go`, `change_reclaim.go` — already migrated), **post-claim consumption** (everything else — this task). The list must be empty of unclassified hits before Step 2. (AGENTS.md: never hand-list gated sites; derive from a whole-repo grep.)

- [ ] **Step 2: Write/adjust failing tests**

For each consumption site, extend the existing tests beside the file with a fixture whose recorded branch is a non-derived name (`feature/renamed-head`) and assert the operation acts on that name:

- `implementation_context`: the report's `FeatureBranch` equals the recorded name; a claimed record with no `branch:` yields the invalid-state refusal path, not a derived name.
- `finalize_context` report: `Branch` field equals the recorded value; empty when the record has none (identity classification for that comes in Task 7 — here just stop inventing a name).
- `finalize_retarget` / `finalize_merge` / `finalize_cleanup` / `finalize_closeout` stack sites: a child (or parent) whose record says `feature/child-head` is addressed by that name; a stack relative with a missing branch causes the operation to refuse before any Git/GitHub effect (assert via the fake that no mutation call was made).

- [ ] **Step 3: Run to verify failure, implement, verify pass**

Run each package's tests with `-count=1`; then replace every consumption call:
- own-record sites → `recordedBranch(...)` + refusal on error;
- relative-record sites → read `relative.Branch()` inline with the same present/non-empty check, refusing (or, for report-only paths like `finalizeDescendants`, carrying the empty string upward) without mutation.

Then delete `BranchForSlug` from `internal/domain/types.go` and build the module: `go build ./...` must succeed with zero remaining references (test files included — update `internal/domain/types_test.go`).

Run: `go test ./internal/... -count=1`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add -u internal
git commit -m "feat(app): consume recorded branch at every post-claim site; delete BranchForSlug"
```

---

### Task 5: Export an exact-number PR read in githubcli

**Files:**
- Modify: `internal/githubcli/probe.go` (new exported `ViewPullRequest`)
- Modify: `internal/githubcli/retarget.go` (private `viewPullRequest` delegates or is folded in)
- Modify: `internal/githubcli/merge.go` (`MergedFacts` gains `HeadBranch`; `probeMergeSnapshot`'s JSON field list gains `headRefName`)
- Test: `internal/githubcli/probe_test.go`, `internal/githubcli/merge_test.go`, `internal/githubcli/fakegh_test.go`

**Interfaces:**
- Produces:

```go
// ViewPullRequest reads ONE pull request by repository identity and exact
// positive number, returning the full normalized snapshot (state — open,
// closed, or merged — head branch, head object id, base branch, version).
// It reuses decodePullRequest: one JSON interpretation, never a second.
// A transport failure, non-zero exit, or decode hazard is a returned typed
// error — never a zero-value PR read as truth, and never "absent".
func (c *Client) ViewPullRequest(ctx context.Context, repo Repository, number int) (PullRequest, error)
```

  Rejects `number <= 0` as invalid input before any process runs. `MergedFacts.HeadBranch string` carries the merged PR's head branch name.
- Consumes: existing `run`, `decodePullRequest`, `prJSONFields`, `Failure` machinery.

- [ ] **Step 1: Write failing tests**

Extend `internal/githubcli/probe_test.go` using the package's fake-gh harness (see `fakegh_test.go` for how existing probes script responses):

- `TestViewPullRequestByNumber`: scripted `gh pr view 7 --repo ... --json ...` response with `state: "OPEN"`, `headRefName: "feature/renamed-head"` → returned PR has `Number == 7`, `State == StateOpen`, `HeadBranch == "feature/renamed-head"`.
- `TestViewPullRequestMergedState`: scripted `state: "MERGED"` decodes to `StateMerged` (no error).
- `TestViewPullRequestErrorIsNotAbsence`: scripted non-zero exit → error returned, zero PR; scripted undecodable JSON → error.
- `TestViewPullRequestRejectsNonPositive`: `ViewPullRequest(ctx, repo, 0)` errors without invoking gh.
- In `merge_test.go`: the merged-probe fixture's JSON gains `headRefName` and the test asserts `MergedFacts.HeadBranch` round-trips.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/githubcli/ -run 'TestViewPullRequest|Merged' -count=1`
Expected: FAIL — method undefined / field missing.

- [ ] **Step 3: Implement**

Move the body of `retarget.go`'s `viewPullRequest` into `probe.go` as `ViewPullRequest` with op `"probe"` (or keep a thin private wrapper in retarget.go calling the exported one so retarget's failure op strings stay stable — pick whichever keeps existing retarget tests green). Add the number guard. Add `HeadBranch` to `MergedFacts` and `headRefName` to the merge snapshot's requested/decoded JSON fields.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/githubcli/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -u internal/githubcli
git commit -m "feat(githubcli): exported exact-number ViewPullRequest; merged facts carry head branch"
```

---

### Task 6: Finalize probes the exact recorded PR, never head discovery

**Files:**
- Modify: `internal/app/finalize_context.go` (`FinalizePRProber` interface, `githubFinalizeProber.ProbePR`, `probeFinalizeFacts`; `FinalizeGitHub` interface drops `FindOpenPullRequestsByHead` if finalize no longer needs it — derive by grep, keep it if finalize_publish still uses it for pre-`pr:` adoption)
- Modify: `internal/domain/finalize.go` (`PRFacts` gains `HeadBranch string`)
- Test: `internal/app/finalize_context_test.go` (or the file where `ProbePR` is currently tested — find with `git grep -ln 'ProbePR' -- 'internal/app/*_test.go'`)

**Interfaces:**
- Produces: `ProbePR(ctx context.Context, repoDir, prRef string) (domain.PRFacts, error)` — the `headBranch` parameter is **removed**. `domain.PRFacts.HeadBranch` populated on open/closed (from `ViewPullRequest`) and merged (from `MergedFacts.HeadBranch`) outcomes.
- Consumes: `githubcli.(*Client).ViewPullRequest` (Task 5), existing `parsePRNumber`, `ProbeMerged`, `DiscoverRepository`.

- [ ] **Step 1: Write failing tests**

Using the existing prober fake pattern in the finalize context tests:

- `TestProbePRReadsExactNumber`: recorded `pr:` parses to 7; the fake's exact-view for 7 returns an open PR with head `feature/renamed-head`; result facts: `State == "open"`, `HeadBranch == "feature/renamed-head"`. Assert the fake's head-search method was **never called** (give the fake a call counter) — this is the assert that reddens when someone reintroduces `FindOpenPullRequestsByHead` on this path (deleting the exact-number lookup must redden; spec Verification).
- `TestProbePRClosedOnlyFromCleanExactRead`: exact view cleanly returns `state: closed` → facts `State == "closed"`.
- `TestProbePRUnknownOnViewError`: exact view errors → `probeFinalizeFacts` yields `State == "unknown"` with the parsed number token (existing behavior contract, now over the exact read). A view error must NOT produce `closed`.
- `TestProbePRMergedCarriesHeadBranch`: `ProbeMerged` reports merged with `HeadBranch` set → facts `State == "merged"`, `HeadBranch` populated.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run TestProbePR -count=1`
Expected: FAIL.

- [ ] **Step 3: Implement**

`githubFinalizeProber.ProbePR` becomes:

```go
func (p *githubFinalizeProber) ProbePR(ctx context.Context, repoDir, prRef string) (domain.PRFacts, error) {
	number, ok := parsePRNumber(prRef)
	if !ok {
		return domain.PRFacts{}, fmt.Errorf("pull-request reference %q carries no parseable number", prRef)
	}
	repo, err := p.gh.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return domain.PRFacts{}, err
	}
	outcome, mf, err := p.gh.ProbeMerged(ctx, repo, number)
	if err != nil {
		return domain.PRFacts{}, err
	}
	if outcome == githubcli.MergeMerged || outcome == githubcli.MergeAlreadyMerged {
		return domain.PRFacts{
			Number: strconv.Itoa(number), Version: mf.Version, State: "merged",
			HeadBranch: mf.HeadBranch, HeadOID: mf.HeadOID, BaseRef: mf.BaseRef,
			MergedAtUTC: mf.MergedAtUTC, MergeCommit: mf.MergeCommit,
		}, nil
	}
	pr, err := p.gh.ViewPullRequest(ctx, repo, number)
	if err != nil {
		return domain.PRFacts{}, err
	}
	return domain.PRFacts{
		Number: strconv.Itoa(number), Version: pr.Version, State: string(pr.State),
		Draft: pr.Draft, HeadBranch: pr.HeadBranch, HeadOID: pr.HeadCommit, BaseRef: pr.BaseBranch,
	}, nil
}
```

`probeFinalizeFacts` drops the `head := domain.BranchForSlug(...)` line (already gone after Task 4 — here remove the parameter plumbing). Update the `FinalizePRProber` interface and its fakes; add `ViewPullRequest` to the `FinalizeGitHub` interface. Update the `githubFinalizeProber` doc comment: the "open-PR probe by feature head" sentence becomes "the exact-number view for a non-merged PR"; delete the stale "Field coverage" clauses that no longer hold, keeping the ones that do (review decision/mergeability still absent for open PRs).

Stack-child probes: run `git grep -n 'ProbePR\|probeFinalizeFacts' -- internal/app` and route every finalize child/descendant probe through the same exact-number path with that child's own `pr:` (spec: "Stack child probes use each child's exact recorded PR and recorded branch under the same rule").

- [ ] **Step 4: Run to verify pass, then mutation-probe**

Run: `go test ./internal/app/ -count=1` → PASS.
Mutation probe (temporary, revert after): make `ProbePR` return `State: "closed"` on view error → `TestProbePRUnknownOnViewError` must redden under `-count=1`. Revert with `git checkout -- internal/app/finalize_context.go` **only if the mutation was uncommitted and yours** (never to restore over uncommitted intended work — stash first if unsure).

- [ ] **Step 5: Commit**

```bash
git add -u internal/app internal/domain
git commit -m "feat(finalize): identify the PR by exact recorded number; probe errors stay unknown"
```

---

### Task 7: Identity classification and failure vocabulary in the finalize report

**Files:**
- Modify: `internal/domain/finalize.go` (`classifyFinalize` compares recorded branch vs `PRFacts.HeadBranch`; new skip-reason constants)
- Modify: `internal/app/finalize_context.go` (`FinalizePRReport` gains `HeadBranch`; `FinalizeCandidateReport` evidence fields)
- Test: `internal/domain/finalize_test.go`, `internal/app/finalize_context_test.go`

**Interfaces:**
- Produces skip-reason string constants in `internal/domain/finalize.go` (exact tokens — the interactive skill and CLI key on them):
  - `skipBranchMissing = "branch-missing"` — recorded branch absent/empty; the exact PR's head is the only candidate, surfaced as evidence.
  - `skipBranchMismatch = "branch-pr-head-mismatch"` — recorded branch and exact PR head differ.
  - `skipBranchMalformed = "branch-malformed"` — recorded branch (shape-invalid) — reuse the `recordedBranch` shape rules.
  - existing `pr-unknown` stays exactly as is for transport/decode failures.
- Report fields: `FinalizePRReport.HeadBranch string \`json:"head_branch,omitempty"\``; `FinalizeCandidateReport` keeps `Branch` (now the recorded value verbatim, empty when absent — never invented).
- Rules: the mismatch/missing verdicts are computed only when facts.State is a cleanly observed `open` or `merged` (a `closed`/`unknown` PR classifies by the existing bands first — identity repair is meaningless against unknown evidence). Neither new skip reason is `--id`-overridable: **do not** set `OverrideNote` for them (spec: "`--id` selects the change but never overrides unresolved identity"; verification: "A mismatch emits both identities and cannot be bypassed by explicit id").

- [ ] **Step 1: Write failing tests**

`internal/domain/finalize_test.go` (follow the existing `classifyFinalize`/selector table pattern):

- open exact PR, head equals recorded `feature/renamed-head` → candidate bands normally (spec: "An open exact PR whose head matches recorded `feature/...` is finalizable without head search").
- open exact PR, head `feature/other` vs recorded `feature/renamed-head` → skip `branch-pr-head-mismatch`.
- recorded branch absent, open exact PR head present → skip `branch-missing`.
- recorded branch `refs/heads/x` → skip `branch-malformed`.
- facts `State == "unknown"` → stays `pr-unknown` regardless of branch state.
- facts cleanly `closed` → stays `pr-closed` (regression pin: the live-failure path — a head mismatch may no longer produce `pr-closed`).

`internal/app/finalize_context_test.go`: the candidate report carries `PR.HeadBranch` and both identities on a mismatch.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/domain/ -run Finalize -count=1`
Expected: FAIL.

- [ ] **Step 3: Implement**

In `classifyFinalize`, after the existing malformed checks and before banding, add the identity conjuncts in this order: malformed recorded branch → `branch-malformed`; then for facts.State `open`/`merged`: absent recorded branch → `branch-missing`; `facts.HeadBranch != recorded` → `branch-pr-head-mismatch`. Thread `PRFacts.HeadBranch` into `FinalizePRReport` in `finalize_context.go`'s fact-copying site (the `FinalizePRReport{...}` literal).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/domain/ ./internal/app/ -count=1` → PASS.

- [ ] **Step 5: Commit**

```bash
git add -u internal/domain internal/app
git commit -m "feat(finalize): structured identity verdicts replace silent pr-closed misclassification"
```

---

### Task 8: Version-pinned identity repair operation (`docket change repair-identity`)

**Files:**
- Create: `internal/app/change_repair.go`, `internal/app/change_repair_test.go`
- Modify: `internal/cli/change.go` (new subcommand)
- Test: `internal/cli/change_test.go`

**Interfaces:**
- Produces:

```go
// RepairIdentityRequest is the version-pinned identity repair finalize's
// checkpoint hands a human's decision to. Exactly one of AdoptPRHead /
// AdoptPR is set. Every Expect* field is the exact evidence the human
// approved; any drift (change version, PR head, PR number) loses the race
// and is refused as stale-evidence rather than applied.
type RepairIdentityRequest struct {
	ID            int
	ExpectVersion string // change-record version token from the finalize report
	// Trust the PR: adopt the exact PR's reported head as branch:.
	AdoptPRHead    bool
	ExpectPRNumber int    // the exact PR number the evidence showed
	ExpectHead     string // the head the human saw and approved
	// Trust the record: adopt this PR reference as pr: (only after an exact
	// read proves its head equals the recorded branch).
	AdoptPR      string
	ExpectBranch string // the recorded branch the human saw
}

func RepairIdentity(ctx context.Context, deps <the finalize dependency bundle used by the other change ops>, req RepairIdentityRequest) <the package's standard result envelope>
```

  Result reason tokens (closed set — every prohibition maps to a return, per learnings prohibition-needs-a-return-value): `repaired-branch`, `repaired-pr`, `stale-evidence`, `workspace-conflict`, `candidate-branch-absent`, `pr-unknown`, `invalid-request`. Follow the request/refusal envelope shape of `internal/app/change_claim.go` (nearest sibling op).
- Consumes: `githubcli.ViewPullRequest` (Task 5), the workspace inspect path (`internal/workspace/inspect.go` via the app seam `workspace_ops.go` uses) for the ownership check, the same remote branch-facts source `change_reclaim.go` gathers for the candidate-branch-exists proof, and the document layer's version-pinned patch machinery that existing metadata mutations use.

**Behavior (each clause below gets a test in Step 1):**
1. Re-read the change record; version differs from `ExpectVersion` → `stale-evidence`, no write.
2. **AdoptPRHead** (trust the PR / missing-branch recovery): exact-view `ExpectPRNumber`; view error → `pr-unknown`, no write; reported head ≠ `ExpectHead` → `stale-evidence`; the remote branch named by the head must exist (probe the same facts source reclaim uses) → else `candidate-branch-absent`; then the workspace gate (clause 4); then write `branch: <head>`.
3. **AdoptPR** (trust the record): parse `AdoptPR` with the ADR-0097 parser (`parsePRRef`) → unparseable is `invalid-request`; recorded branch must equal `ExpectBranch` else `stale-evidence`; exact-view the parsed number; error → `pr-unknown`; reported head ≠ recorded branch → `stale-evidence` (the supplied PR does not prove identity); write `pr: <ref>`.
4. **Workspace gate** (before any write, both directions): if a Docket workspace owned by this change exists and its target `FeatureBranch()` differs from the branch the record will carry after the repair → `workspace-conflict`, no write (spec: "If an owned workspace still targets the old name, finalize reports the conflict and stops before editing metadata"). No workspace, or workspace already targeting the proposed branch, passes. An inspect **error** is ambiguity → `workspace-conflict` path too, never a pass (unknown never authorizes; learnings probe-error-is-not-clean-absence).
5. The op writes exactly one field (`branch:` or `pr:`) plus the standard `updated:` stamp — nothing else. Re-probing after repair is the **workflow's** job (Task 9), not this op's.

- [ ] **Step 1: Write failing tests** — one per numbered clause above, against the fakes: e.g. `TestRepairAdoptPRHeadWritesBranch`, `TestRepairStaleVersionRefused`, `TestRepairStaleHeadRefused`, `TestRepairCandidateBranchAbsent`, `TestRepairWorkspaceConflictBlocks` (fixture: owned workspace targeting `feature/old-name`, repair proposes `feature/new-name` — assert the refusal AND, via a sentinel in the fixture, that the conflicting-workspace check actually executed; the spec's mutation test needs proof "the conflicting fixture was actually reached"), `TestRepairAdoptPRRequiresHeadEqualsRecorded`, `TestRepairViewErrorIsUnknownNotApplied`, `TestRepairWritesOnlyTheApprovedField` (diff the document before/after).

- [ ] **Step 2: Run to verify failure** — `go test ./internal/app/ -run TestRepair -count=1` → FAIL (undefined).

- [ ] **Step 3: Implement `internal/app/change_repair.go`** per the behavior clauses, then the CLI subcommand in `internal/cli/change.go`:

```
docket change repair-identity --id N --expect-version V \
  (--adopt-pr-head --expect-pr M --expect-head H | --adopt-pr REF --expect-branch B)
```

Flag validation (exactly one mode, all its evidence flags present) happens in the CLI layer → `invalid-request`. JSON/human output through the package's standard presenter, reason token verbatim.

- [ ] **Step 4: Run to verify pass** — `go test ./internal/app/ ./internal/cli/ -count=1` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/change_repair.go internal/app/change_repair_test.go
git add -u internal/cli
git commit -m "feat(change): version-pinned repair-identity op with workspace fail-closed gate"
```

---

### Task 9: Skill and convention surfaces — checkpoint, capture, and manifest docs

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (identity-repair checkpoint)
- Modify: `skills/docket-new-change/SKILL.md` (capture `branch_prefix` from natural language)
- Modify: `skills/docket-convention/SKILL.md` (manifest field, mint rule)
- Modify: `skills/docket-implement-next/SKILL.md` **only if** it states the `feat/<slug>` rule — derive by `git grep -n 'feat/' -- skills/ scripts/ docs/` and update every executable-or-normative statement of the old mint rule (prose examples of literal branch names in unrelated war-story text stay)
- Test: existing skill sentinel tests under `tests/` that redden (run the suite to find them), updated to the new prose with phrase-bound asserts

**Content requirements:**
- **docket-convention**: `branch_prefix:` documented beside `branch:` — optional, one unqualified branch-path component, captured at creation, consumed only at claim, inert after `branch:` is populated, surviving reclaim; the branch model section's mint rule becomes "`<type>/<slug>`, or `<branch_prefix>/<slug>` when the override is present; after claim the recorded `branch:` is the sole source of truth and is never reconstructed."
- **docket-new-change**: one instruction block — when the human's request names a branch prefix ("use the `hotfix/` prefix"), record the normalized scalar `branch_prefix: hotfix` (strip one presentation-only trailing slash; refuse — ask the human — on a slash-embedded or `refs/`-qualified value rather than repairing it). Claim never searches prose for naming clues.
- **docket-finalize-change**: a checkpoint section keyed on the Task 7 skip reasons:
  - `branch-pr-head-mismatch` → present the evidence (change id+version, recorded `branch:`, exact PR number/state, reported head) and exactly three choices: **Trust the PR** (`docket change repair-identity --id N --expect-version V --adopt-pr-head --expect-pr M --expect-head H`), **Trust the record** (`... --adopt-pr <ref> --expect-branch B` — the human supplies the correct PR reference), **Abort** (no writes).
  - `branch-missing` → offer ONLY the exact PR's reported head (the repair op itself proves the remote branch exists); confirm or abort. Never search for a likely branch or PR.
  - After a successful repair: **reload and re-probe from scratch** (`docket context finalize --id N` again) before any finalize effect; a `stale-evidence`/`workspace-conflict`/`pr-unknown`/`candidate-branch-absent` refusal is reported to the human verbatim and stops the flow.
  - Non-interactive callers (implement-next's sweep) halt with the structured evidence; they never repair autonomously.

- [ ] **Step 1: Make the edits above.**
- [ ] **Step 2: Run the shell suite to find reddened sentinels** — read the command from `finalize.test_command` in the effective docket config and run it. Update reddened prose guards to bind phrase to claim (not bare phrase-presence).
- [ ] **Step 3: Commit**

```bash
git add -u skills tests docs
git commit -m "docs(skills): identity-repair checkpoint, branch_prefix capture, mint-rule convention"
```

---

### Task 10: Static reconstruction guard

**Files:**
- Create: `tests/test_branch_reconstruction_guard.sh` (follow the header/refusal/remedy shape of an existing guard, e.g. `tests/test_comment_anchor_style.sh`; register it wherever `tests/README.md` says new tests are picked up)

**Guard content** (shape-keyed, computed floors — never hand-written counts; learnings: byte-pattern-guard-matches-a-spelling, marker-scoped-guard-needs-a-population-floor):

1. **Retired symbol stays retired:** `BranchForSlug` matches nowhere under `internal/` in any `.go` file (tests included). This is a total ban — the function was deleted in Task 4.
2. **No `feat/` reconstruction in executable Go:** over non-test `.go` files under `internal/`, strip comment-only lines (lines whose first non-space characters are `//`), then assert zero remaining occurrences of the byte pattern `"feat/` (the quote bounds the left side; this catches `"feat/" + x` and `fmt.Sprintf("feat/%s", ...)` alike). State the guard's known imprecision in its header as an assert-adjacent comment: it bans the quoted spelling only.
3. **Mint population floor:** compute the set of non-test `.go` files calling `MintBranch(` via grep; assert the set is exactly `internal/domain/actions.go`, `internal/domain/lease.go`, `internal/app/change_reclaim.go` — both directions (each expected file present = floor; no unexpected file = ceiling). A new legitimate mint site updates this list consciously.

- [ ] **Step 1: Write the guard, run it green** against the current tree.
- [ ] **Step 2: Mutation-test all three clauses** (temporary edits, then revert the mutation only — the guard file and your other work are committed or untouched):
  - re-add a `func BranchForSlug` stub in `internal/domain/types.go` → clause 1 reddens;
  - add `x := "feat/" + "y"; _ = x` in a non-test file → clause 2 reddens; add the same inside a `//` comment → stays green;
  - add a `domain.MintBranch("a", domain.OptionalString{}, "b")` call in `internal/app/finalize_merge.go` → clause 3 reddens; remove the `lease.go` call → clause 3 reddens (floor direction).
- [ ] **Step 3: Commit**

```bash
git add tests/test_branch_reconstruction_guard.sh
git add -u tests
git commit -m "test: shape-keyed guard against branch-name reconstruction"
```

---

### Task 11: Spec-mandated mutation tests and the full-suite gate

**Files:** none new — verification only, plus whatever small fixes fall out.

- [ ] **Step 1: Run the three mutation probes the spec names** (each: apply temporary mutation, run the named tests with `-count=1`, confirm red, revert the mutation):
  1. "replacing a recorded branch with a reconstructed name must redden" — in `internal/app/branch_identity.go`, make `recordedBranch` return `c.Type() + "/" + c.Slug()` unconditionally → Task 3/4's non-derived-name tests (`feature/renamed-head` fixtures) redden.
  2. "deleting the exact-PR-number lookup must redden" — in `githubFinalizeProber.ProbePR`, replace the `ViewPullRequest` call with a hardcoded `closed` result → Task 6's `TestProbePRReadsExactNumber` and Task 7's mismatch tests redden.
  3. "removing the workspace-conflict refusal must redden while proving the conflicting fixture was actually reached" — delete the workspace gate in `RepairIdentity` → `TestRepairWorkspaceConflictBlocks` reddens, and its reached-sentinel assert proves the fixture exercised the gate.

  Record each probe and its red output for the build-evidence record.
- [ ] **Step 2: Run the whole suite** via the command `finalize.test_command` resolves to (read it from config). Treat `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines as screening findings and any `SERIAL CONFIRMED OVER BUDGET:` line as a breach to act on — nothing else surfaces them.
- [ ] **Step 3: `go build ./... && go vet ./...`** clean.
- [ ] **Step 4: Commit any fallout fixes**

```bash
git add -u
git commit -m "test: spec mutation probes verified; suite green at the gate"
```

---

## Self-Review Notes

- **Spec coverage:** mint rule + `branch_prefix` (Task 1), reclaim exception + prefix survival (Task 2), explicit workspace target + consumer population (Tasks 3–4), exact PR read (Tasks 5–6), failure vocabulary + non-overridable identity skips (Task 7), repair transaction + workspace safety + missing-branch recovery (Task 8), interactive checkpoint + capture + convention docs + reload-and-re-probe (Task 9), static guard (Task 10), spec's named mutation tests + full-suite gate (Task 11). "End-to-end coverage includes ordinary finalize, stack retarget/closeout, cleanup, and workspace resume with deliberately non-derived branch names" is distributed as the `feature/renamed-head` fixtures required in Tasks 3, 4, 6, 7.
- **Population honesty:** Tasks 4 and 9 derive their site lists from whole-repo greps at execution time rather than trusting this plan's enumeration — the greps above were run while planning, but the executable rule is the grep, not the list.
- **Out of scope (do not build):** repo-wide prefix config, branch renames/workspace migration, slug-similarity search, integration-branch policy changes, new lifecycle states.
