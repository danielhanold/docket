<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0444 — Reconcile uncertain publication records so cancellation and resume can finish](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-24-0444-reconcile-uncertain-publication-records-so-cancellation-and.md)**
<!-- docket:backlink:end -->
# Reconcile Uncertain Publication Records Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. (In this repo the build role is `docket-build`, which fulfills the subagent-driven contract.)

**Goal:** When a publication (PR create or feature-head push) returns an unobservable outcome and an identical retry later succeeds in the same run epoch, cancellation and successful closeout durably settle the original `uncertain` journal entry from local journal evidence alone, so cancellation can reach `cancelled` and resume can admit its one replacement.

**Architecture:** Extend the existing run-epoch journal entry (`AdmittedMutation`, `internal/app/rungate_epoch.go`) with an optional, typed publication descriptor persisted at admission time, before the external adapter runs. Add one pure matching helper over the journal (uncertain entry + later completed entry with an identical valid descriptor and the same operation) and one `epochCAS`-guarded settlement writer that flips only matched original entries `uncertain`→`completed`. Wire the settlement into the two authorized write paths — cancellation's teardown accounting (`reconcileEpochTeardown`) and the attributed keyed successful closeout (`completeSuccessfulRun`) — with no Git/GitHub calls anywhere in reconciliation. Read-only paths (`verifyTerminalEpochQuiescence`, `validateResumeQuiescence`, RunVerify, unattributed verdicts) gain no writes.

**Tech Stack:** Go; existing `internal/app` epoch store (flock + atomic-rename CAS), `crypto/sha256` digests, existing test fixtures (`newCancelFixture`, `runCancel`, `completeSuccessfulRun` seams, `mintFenceEpoch`, `fakeGitHub`, `fakeWorkspaceService`).

**Spec:** `docs/superpowers/specs/2026-09-23-reconcile-uncertain-publication-records-so-cancellation-and-design.md` (synchronized metadata tree: `.docket/docs/superpowers/specs/...` on branch `docket`; read it alongside this plan).

## Global Constraints

- Epoch schema stays **v1** with **additive optional fields only** — existing records must still decode (compatibility pattern of change 0441).
- Reconciliation is a **pure function of the durable epoch record**: no Git, no GitHub, no network calls, no remote re-observation, ever.
- An entry is settled **only** by a later (higher-index) completed entry in the **same epoch** with the **same operation** and a **field-for-field identical valid descriptor**. No match, a missing/malformed descriptor on either side, a differing field, or a later identical entry that is itself `admitted` or `uncertain` leaves the original pending.
- Never downgrade `completed` to `uncertain`; never touch unrelated entries, participants, or epoch state; a failed settlement write retains exclusion and reports a bounded finding.
- Never persist body bytes, raw remote URLs, credentials, or argv into journal records or findings — digests and bounded tokens only.
- Read-only paths gain **no writes**: `RunVerify`, unattributed verdicts, `verifyTerminalEpochQuiescence`, `validateResumeQuiescence`.
- Still-`admitted` entries, legacy entries without descriptors, malformed descriptors, and unknown operations remain pending (fail closed). Completed legacy entries remain accepted. No forced migration.
- Every new guard/predicate is mutation-tested (strip the guarded thing, watch the test redden — repo AGENTS.md rule).
- The BUILD gate runs the whole configured `build.test_command` from source through the Go suite runner (`internal/suiterunner`); read the budget report even on green.

## File Structure

- Create: `internal/app/rungate_publication.go` — descriptor type, digest helper, per-op validation, matching predicate, settlement writer.
- Create: `internal/app/rungate_publication_test.go` — unit + matrix tests for the above.
- Modify: `internal/app/rungate_epoch.go` — `AdmittedMutation` gains the optional `Publication` field.
- Modify: `internal/app/rungate_fence.go` — `admitWorkflowMutation` accepts and journals the descriptor; `MutationAdmissionHook` passes nil.
- Modify: `internal/app/pr_publish.go` — build the PR descriptor at the boundary.
- Modify: `internal/app/workspace_ops.go` — build the workspace descriptor at the boundary.
- Modify: `internal/app/rungate_cancel.go` — settlement wired into `reconcileEpochTeardown`.
- Modify: `internal/app/rungate_complete.go` — settlement wired into `completeSuccessfulRun`.
- Test: `internal/app/rungate_fence_test.go`, `internal/app/rungate_cancel_test.go`, `internal/app/rungate_complete_test.go`, `internal/app/rungate_before_test.go` — end-to-end acceptance.

---

### Task 1: Publication descriptor type, digests, and per-op validation

**Files:**
- Create: `internal/app/rungate_publication.go`
- Create: `internal/app/rungate_publication_test.go`

**Interfaces:**
- Produces: `type MutationPublication struct` (comparable — field-for-field equality is `==`), `func publicationDigest(label, value string) string`, `func validPublication(op string, p *MutationPublication) bool`. Later tasks rely on these exact names.

- [ ] **Step 1: Write the failing tests**

```go
package app

import "testing"

// TestPublicationDigestDomainSeparated: digests are sha256-hex, label-separated with
// an unambiguous NUL boundary so ("a","bc") can never collide with ("ab","c") or a
// different label over the same bytes.
func TestPublicationDigestDomainSeparated(t *testing.T) {
	d1 := publicationDigest("pr-title", "Add widget")
	if len(d1) != 64 {
		t.Fatalf("digest length = %d, want 64 hex chars", len(d1))
	}
	if d1 != publicationDigest("pr-title", "Add widget") {
		t.Fatal("digest must be deterministic")
	}
	if d1 == publicationDigest("pr-body", "Add widget") {
		t.Fatal("different labels over the same value must not collide")
	}
	if publicationDigest("l", "ab") == publicationDigest("la", "b") {
		t.Fatal("label/value boundary must be unambiguous")
	}
}

// TestValidPublicationPerOp: validation enforces operation/descriptor agreement —
// each op requires its own fields non-empty AND the other op's fields empty; an
// unknown op is never valid; nil is never valid.
func TestValidPublicationPerOp(t *testing.T) {
	pr := &MutationPublication{
		RepoHost: "github.com", RepoOwner: "danielhanold", RepoName: "docket",
		HeadRef: "fix/widget", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		BaseBranch:  "main",
		TitleDigest: publicationDigest("pr-title", "t"),
		BodyDigest:  publicationDigest("pr-body", "b"),
	}
	ws := &MutationPublication{
		RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/widget", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if !validPublication(OperationPRPublish, pr) {
		t.Fatal("complete PR descriptor must validate")
	}
	if !validPublication(OperationWorkspacePublish, ws) {
		t.Fatal("complete workspace descriptor must validate")
	}
	if validPublication(OperationPRPublish, nil) || validPublication(OperationWorkspacePublish, nil) {
		t.Fatal("nil descriptor is never valid")
	}
	if validPublication("metadata.transaction", pr) {
		t.Fatal("unknown operation is never valid")
	}
	// Each required PR field, blanked, invalidates.
	for _, blank := range []func(*MutationPublication){
		func(p *MutationPublication) { p.RepoHost = "" },
		func(p *MutationPublication) { p.RepoOwner = "" },
		func(p *MutationPublication) { p.RepoName = "" },
		func(p *MutationPublication) { p.HeadRef = "" },
		func(p *MutationPublication) { p.HeadCommit = "" },
		func(p *MutationPublication) { p.BaseBranch = "" },
		func(p *MutationPublication) { p.TitleDigest = "" },
		func(p *MutationPublication) { p.BodyDigest = "" },
	} {
		c := *pr
		blank(&c)
		if validPublication(OperationPRPublish, &c) {
			t.Fatalf("PR descriptor with a blanked required field must not validate: %+v", c)
		}
	}
	// Agreement: a PR descriptor carrying workspace-only fields (and vice versa) is
	// op/descriptor disagreement and invalid.
	crossPR := *pr
	crossPR.Remote = "origin"
	if validPublication(OperationPRPublish, &crossPR) {
		t.Fatal("PR descriptor carrying a workspace-only field must not validate")
	}
	crossWS := *ws
	crossWS.TitleDigest = "x"
	if validPublication(OperationWorkspacePublish, &crossWS) {
		t.Fatal("workspace descriptor carrying a PR-only field must not validate")
	}
	for _, blank := range []func(*MutationPublication){
		func(p *MutationPublication) { p.RepoDir = "" },
		func(p *MutationPublication) { p.Remote = "" },
		func(p *MutationPublication) { p.HeadRef = "" },
		func(p *MutationPublication) { p.HeadCommit = "" },
	} {
		c := *ws
		blank(&c)
		if validPublication(OperationWorkspacePublish, &c) {
			t.Fatalf("workspace descriptor with a blanked required field must not validate: %+v", c)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestPublicationDigest|TestValidPublication' -count=1`
Expected: FAIL (undefined: `publicationDigest`, `MutationPublication`, `validPublication`).

- [ ] **Step 3: Write the implementation**

```go
// Publication-identity reconciliation for the run-epoch mutation journal
// (change 0444). A publication whose remote outcome could not be observed leaves
// an `uncertain` admitted-mutation entry; when a LATER admission in the SAME
// epoch with an IDENTICAL publication identity completed, the completed retry's
// adapter already verified the exact postcondition (githubcli EnsurePullRequest's
// post-mutation verification; workspace PublishHead's reprobeAfterPush), so the
// original obligation is settled by a LOCAL journal comparison — no Git or GitHub
// call is ever made here. Everything in this file is a pure function of the
// durable epoch record, except settleUncertainPublications, which persists the
// settlement through the ordinary epochCAS.
package app

import (
	"crypto/sha256"
	"encoding/hex"
)

// MutationPublication is the immutable publication identity captured at a
// publication boundary, persisted with the `admitted` journal entry BEFORE the
// external adapter runs. It carries identity only — digests and resolved
// locators, never body bytes, remote URLs, credentials, or argv. The struct is
// comparable: field-for-field identity is Go ==.
type MutationPublication struct {
	// PR publication (pr.publish): the already-resolved GitHub identity, exact
	// head branch and full requested commit, effective base branch, and
	// deterministic digests of the requested title and fully assembled body.
	RepoHost    string `json:"repo_host,omitempty"`
	RepoOwner   string `json:"repo_owner,omitempty"`
	RepoName    string `json:"repo_name,omitempty"`
	BaseBranch  string `json:"base_branch,omitempty"`
	TitleDigest string `json:"title_digest,omitempty"`
	BodyDigest  string `json:"body_digest,omitempty"`
	// Workspace publication (workspace.publish): canonical repository identity
	// (the discovered common dir, already symlink-canonical) and remote name.
	RepoDir string `json:"repo_dir,omitempty"`
	Remote  string `json:"remote,omitempty"`
	// Shared: the exact ref/branch published and the full intended commit —
	// the head actually handed to the publication operation.
	HeadRef    string `json:"head_ref,omitempty"`
	HeadCommit string `json:"head_commit,omitempty"`
}

// publicationDigest is the deterministic digest convention for authored inputs
// in a publication descriptor: sha256 over label + NUL + exact value bytes,
// hex-encoded. The NUL keeps the label/value boundary unambiguous (labels
// contain no NUL), so distinct (label, value) pairs never collide by
// concatenation.
func publicationDigest(label, value string) string {
	sum := sha256.Sum256(append(append([]byte(label), 0), []byte(value)...))
	return hex.EncodeToString(sum[:])
}

// validPublication reports whether p is a COMPLETE descriptor for op — every
// field the operation requires is present AND every field it does not own is
// empty (operation/descriptor agreement). A nil descriptor, a partial one, or
// an unknown operation is invalid: reconciliation must never trust it
// (acceptance: legacy/malformed/unknown entries remain pending).
func validPublication(op string, p *MutationPublication) bool {
	if p == nil {
		return false
	}
	switch op {
	case OperationPRPublish:
		return p.RepoHost != "" && p.RepoOwner != "" && p.RepoName != "" &&
			p.BaseBranch != "" && p.TitleDigest != "" && p.BodyDigest != "" &&
			p.HeadRef != "" && p.HeadCommit != "" &&
			p.RepoDir == "" && p.Remote == ""
	case OperationWorkspacePublish:
		return p.RepoDir != "" && p.Remote != "" &&
			p.HeadRef != "" && p.HeadCommit != "" &&
			p.RepoHost == "" && p.RepoOwner == "" && p.RepoName == "" &&
			p.BaseBranch == "" && p.TitleDigest == "" && p.BodyDigest == ""
	default:
		return false
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestPublicationDigest|TestValidPublication' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/rungate_publication.go internal/app/rungate_publication_test.go
git commit -m "feat(app): typed publication identity descriptor with per-op validation (change 0444)"
```

---

### Task 2: Journal the descriptor at admission — schema field + `admitWorkflowMutation` threading

**Files:**
- Modify: `internal/app/rungate_epoch.go` (the `AdmittedMutation` struct)
- Modify: `internal/app/rungate_fence.go` (`admitWorkflowMutation`, `MutationAdmissionHook`)
- Modify: `internal/app/pr_publish.go`, `internal/app/workspace_ops.go` (call sites — pass `nil` for now; Tasks 3–4 supply real descriptors)
- Modify: every test call site of `admitWorkflowMutation` (in `internal/app/rungate_fence_test.go` — pass `nil`)
- Test: `internal/app/rungate_publication_test.go`

**Interfaces:**
- Consumes: `MutationPublication`, `validPublication` (Task 1).
- Produces: `AdmittedMutation.Publication *MutationPublication` (JSON `publication,omitempty`); new signature `admitWorkflowMutation(repoDir, op string, pub *MutationPublication) (mutationJournalDone, error)`. Later tasks rely on both.

- [ ] **Step 1: Write the failing test**

```go
// TestAdmissionJournalsPublicationDescriptorAndLegacyDecodes: an admission carrying a
// descriptor persists it verbatim in the journal entry (schema v1, additive field);
// an existing entry WITHOUT the field still decodes (legacy compatibility).
func TestAdmissionJournalsPublicationDescriptorAndLegacyDecodes(t *testing.T) {
	fx := newCancelFixture(t, false) // active epoch bound to the fixture worktree
	pub := &MutationPublication{
		RepoHost: "github.com", RepoOwner: "o", RepoName: "r",
		HeadRef: "fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		BaseBranch:  "main",
		TitleDigest: publicationDigest("pr-title", "t"),
		BodyDigest:  publicationDigest("pr-body", "b"),
	}
	done, err := admitWorkflowMutation(fx.worktree, OperationPRPublish, pub)
	if err != nil {
		t.Fatalf("admitWorkflowMutation: %v", err)
	}
	done(mutationStatusUncertain)

	ep, _, lerr := LoadEpochRecord(fx.repo, fx.key)
	if lerr != nil {
		t.Fatalf("LoadEpochRecord: %v", lerr)
	}
	if len(ep.AdmittedMutations) != 1 {
		t.Fatalf("journal length = %d, want 1", len(ep.AdmittedMutations))
	}
	got := ep.AdmittedMutations[0]
	if got.Status != mutationStatusUncertain || got.OpKey != OperationPRPublish {
		t.Fatalf("entry = %+v, want uncertain pr.publish", got)
	}
	if got.Publication == nil || *got.Publication != *pub {
		t.Fatalf("persisted descriptor = %+v, want %+v", got.Publication, pub)
	}

	// Legacy shape: an entry with no publication field decodes and stays usable.
	if cerr := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.AdmittedMutations = append(r.AdmittedMutations,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted})
		return nil
	}); cerr != nil {
		t.Fatalf("epochCAS append legacy: %v", cerr)
	}
	ep2, _, lerr2 := LoadEpochRecord(fx.repo, fx.key)
	if lerr2 != nil {
		t.Fatalf("LoadEpochRecord after legacy append: %v", lerr2)
	}
	if ep2.AdmittedMutations[1].Publication != nil {
		t.Fatal("legacy entry must decode with a nil descriptor")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestAdmissionJournalsPublicationDescriptor -count=1`
Expected: FAIL to compile (`admitWorkflowMutation` takes 2 args; `AdmittedMutation` has no `Publication` field).

- [ ] **Step 3: Implement**

In `internal/app/rungate_epoch.go`, extend `AdmittedMutation` (keep the existing comment, add one sentence):

```go
// AdmittedMutation is one journaled workflow-mutation admission at a shared
// mutation boundary (transaction engine, PR publish, workspace publish). Status is
// admitted|completed|uncertain; a cancellation stays pending until every admitted
// entry is completed or uncertain-reconciled (Task 11 wires the journal writes,
// Task 10 reads them). OpKey is the bounded operation key, never argv/env/content.
// Publication is the optional immutable publication identity captured at admission
// (change 0444) — additive schema-v1 field; nil on legacy and non-publication
// entries. Reconciliation trusts it only when validPublication accepts it.
type AdmittedMutation struct {
	OpKey       string               `json:"op_key"`
	Status      string               `json:"status"` // admitted|completed|uncertain
	Publication *MutationPublication `json:"publication,omitempty"`
}
```

In `internal/app/rungate_fence.go`, change the signature and the append (doc comment: add "pub is the optional publication identity journaled with the admission (change 0444); nil for non-publication boundaries"):

```go
func admitWorkflowMutation(repoDir, op string, pub *MutationPublication) (mutationJournalDone, error) {
```

and inside the `EpochActive` arm:

```go
		case EpochActive:
			idx = len(rec.AdmittedMutations)
			rec.AdmittedMutations = append(rec.AdmittedMutations, AdmittedMutation{
				OpKey:       op,
				Status:      mutationStatusAdmitted,
				Publication: pub,
			})
			return nil
```

Update `MutationAdmissionHook` (engine mutations carry no publication identity):

```go
		done, err := admitWorkflowMutation(repoDir, string(op), nil)
```

Update the two production boundary call sites to pass `nil` for now (`pr_publish.go`: `admitWorkflowMutation(repoDir, OperationPRPublish, nil)`; `workspace_ops.go`: `admitWorkflowMutation(repoDir, OperationWorkspacePublish, nil)`), and every `admitWorkflowMutation(` call in `internal/app/rungate_fence_test.go` to add a trailing `, nil`. Find all sites with `grep -rn 'admitWorkflowMutation(' internal/` — never from memory.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/app/ -run 'TestAdmission|TestFence|TestInFlightMutation' -count=1`
Expected: PASS (new test green; existing fence tests still green with `nil`).

- [ ] **Step 5: Commit**

```bash
git add internal/app/rungate_epoch.go internal/app/rungate_fence.go internal/app/pr_publish.go internal/app/workspace_ops.go internal/app/rungate_fence_test.go internal/app/rungate_publication_test.go
git commit -m "feat(app): admitted-mutation journal carries an optional publication descriptor (change 0444)"
```

---

### Task 3: Capture the PR publication descriptor at the `PRPublish` boundary

**Files:**
- Modify: `internal/app/pr_publish.go` (step 8a, the `admitWorkflowMutation` call)
- Test: `internal/app/rungate_fence_test.go`

**Interfaces:**
- Consumes: `MutationPublication`, `publicationDigest`, new `admitWorkflowMutation` signature.
- Produces: production PR admissions journal a descriptor built from `repo` (the discovered `githubcli.Repository`), `headBranch`, `req.Head`, `baseBranch`, and digests of `req.Title` and the fully assembled `body`.

- [ ] **Step 1: Write the failing test**

Follow the existing idiom of `TestFenceBlocksPRPublishAfterCancel` (`internal/app/rungate_fence_test.go` — reuse `newWorkingRepo`, `mintFenceEpoch` with `EpochActive`, `prReader`, `fakeGitHub`, `readyService`, `prHead`, `prEvidenceBytes`, and its helper for loading the minted epoch by key — `mintFenceEpoch` returns/records the key; read how the neighboring active-epoch tests load the record and do the same):

```go
// TestPRPublishJournalsPublicationIdentity: a fenced (active-epoch) PR publish
// journals a VALID descriptor carrying the resolved repo identity, exact head
// branch + full commit, base branch, and title/body digests — and the journal
// bytes never contain the raw title or body (digests only).
func TestPRPublishJournalsPublicationIdentity(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	key := mintFenceEpoch(t, repoDir, repoDir, EpochActive) // adapt to the helper's real shape

	gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("verified")}}
	deps := workspaceDepsFor(t, prReader(t))
	const secretTitle = "Add widget SEKRET-TITLE-BYTES"
	const secretBody = "Authored prose SEKRET-BODY-BYTES.\n"

	res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: gh},
		repoDir, PRPublishRequest{ID: 7, Head: prHead, Title: secretTitle, Body: secretBody, EvidenceRecord: prEvidenceBytes(t, prHead)})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q (reason %q), want applied", res.Result, res.Reason)
	}

	ep, _, err := LoadEpochRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if len(ep.AdmittedMutations) != 1 {
		t.Fatalf("journal length = %d, want 1", len(ep.AdmittedMutations))
	}
	m := ep.AdmittedMutations[0]
	if m.Status != mutationStatusCompleted {
		t.Fatalf("status = %q, want completed (observed EnsureCreated)", m.Status)
	}
	if !validPublication(OperationPRPublish, m.Publication) {
		t.Fatalf("journaled descriptor must be valid, got %+v", m.Publication)
	}
	if m.Publication.HeadCommit != prHead {
		t.Fatalf("HeadCommit = %q, want the requested head %q", m.Publication.HeadCommit, prHead)
	}
	if m.Publication.TitleDigest != publicationDigest("pr-title", secretTitle) {
		t.Fatal("TitleDigest must be the deterministic digest of the requested title")
	}
	if m.Publication.BodyDigest == "" || m.Publication.BodyDigest == publicationDigest("pr-body", secretBody) {
		// The digest covers the fully ASSEMBLED body (backlink + evidence woven in),
		// which differs from the raw authored prose.
		t.Fatal("BodyDigest must digest the assembled body, not the raw authored prose")
	}

	// Leak check on the durable record bytes: digests only, never content.
	raw, rerr := os.ReadFile(filepath.Join(gateKeyDirForTest(t, repoDir, key), "epoch.json")) // reuse/introduce the test helper the fence tests use to reach the key dir
	if rerr != nil {
		t.Fatalf("read epoch record: %v", rerr)
	}
	for _, secret := range []string{"SEKRET-TITLE-BYTES", "SEKRET-BODY-BYTES"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("journal bytes leak %q", secret)
		}
	}
}
```

(If no `gateKeyDirForTest` helper exists, compute the path the way `removeAdmissionRecord` in `rungate_cancel_test.go` documents the storage layout, via `gateGitCommonDir` + `docket/rungate/<key>/epoch.json`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestPRPublishJournalsPublicationIdentity -count=1`
Expected: FAIL — descriptor is nil (`validPublication` false).

- [ ] **Step 3: Implement the capture**

In `PRPublish` (step 8a), the descriptor is built from values already resolved and verified above it — never re-derived later from mutable state — and journaled before `EnsurePullRequest`:

```go
	// (8a) ... existing comment ... The admission journals the immutable publication
	// identity (change 0444): resolved repo, exact head branch + full requested
	// commit, effective base, and digests of the requested title and the fully
	// assembled body — so a later identical retry can settle an uncertain entry
	// from local journal evidence alone.
	pub := &MutationPublication{
		RepoHost:    repo.Host,
		RepoOwner:   repo.Owner,
		RepoName:    repo.Name,
		HeadRef:     headBranch,
		HeadCommit:  req.Head,
		BaseBranch:  baseBranch,
		TitleDigest: publicationDigest("pr-title", req.Title),
		BodyDigest:  publicationDigest("pr-body", string(body)),
	}
	done, ferr := admitWorkflowMutation(repoDir, OperationPRPublish, pub)
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run 'TestPRPublish|TestFence' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/pr_publish.go internal/app/rungate_fence_test.go
git commit -m "feat(app): PR publish journals its publication identity at admission (change 0444)"
```

---

### Task 4: Capture the workspace publication descriptor at the `WorkspacePublish` boundary

**Files:**
- Modify: `internal/app/workspace_ops.go` (the fence call in `WorkspacePublish`)
- Test: `internal/app/rungate_fence_test.go`

**Interfaces:**
- Consumes: `MutationPublication`, new `admitWorkflowMutation` signature.
- Produces: production workspace admissions journal a descriptor from `wc.repo.CommonDir` (already symlink-canonical — `gitcli.Repository` doc), `originRemote`, `target.FeatureRef`, and `req.Head` (the head the reinspection just proved is the one being handed to `PublishHead`).

- [ ] **Step 1: Write the failing test**

Mirror `TestFenceBlocksWorkspacePublishAfterCancel`'s fixture (same file), but with `EpochActive` and a successful publish; assert the journaled entry:

```go
// TestWorkspacePublishJournalsPublicationIdentity: an active-epoch workspace publish
// journals a VALID workspace descriptor: canonical repo identity, remote name, exact
// feature ref, and the full intended commit.
func TestWorkspacePublishJournalsPublicationIdentity(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	key := mintFenceEpoch(t, repoDir, repoDir, EpochActive)

	const head = "abcdef0000000000000000000000000000000000"
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")}}
	svc := &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)},
		publishRes: workspace.PublishResult{Disposition: workspace.PublishPublished, Head: gitcli.ObjectID(head)},
	}
	res := WorkspacePublish(context.Background(), depsForReader(t, reader), WorkspaceDeps{Service: svc},
		repoDir, WorkspacePublishRequest{ID: 7, Head: head})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q (reason %q), want applied", res.Result, res.Reason)
	}

	ep, _, err := LoadEpochRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if len(ep.AdmittedMutations) != 1 {
		t.Fatalf("journal length = %d, want 1", len(ep.AdmittedMutations))
	}
	m := ep.AdmittedMutations[0]
	if !validPublication(OperationWorkspacePublish, m.Publication) {
		t.Fatalf("journaled descriptor must be valid, got %+v", m.Publication)
	}
	if m.Publication.HeadCommit != head || m.Publication.Remote != "origin" {
		t.Fatalf("descriptor = %+v, want head %q on origin", m.Publication, head)
	}
	if m.Publication.HeadRef == "" || m.Publication.RepoDir == "" {
		t.Fatalf("descriptor must carry the exact feature ref and canonical repo dir: %+v", m.Publication)
	}
}
```

(Adapt the deps/reader helper names to what `TestFenceBlocksWorkspacePublishAfterCancel` actually uses in this file — read that test first and reuse its exact helpers.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestWorkspacePublishJournalsPublicationIdentity -count=1`
Expected: FAIL — nil descriptor.

- [ ] **Step 3: Implement the capture**

In `WorkspacePublish`, replace the fence call:

```go
	// Run-epoch mutation fence ... existing comment ... The admission journals the
	// immutable publication identity (change 0444): canonical repository identity,
	// remote name, exact feature ref, and the full intended commit — the same head
	// the reinspection above just proved current.
	pub := &MutationPublication{
		RepoDir:    wc.repo.CommonDir,
		Remote:     string(originRemote),
		HeadRef:    string(target.FeatureRef),
		HeadCommit: req.Head,
	}
	done, ferr := admitWorkflowMutation(repoDir, OperationWorkspacePublish, pub)
```

(If `originRemote` is already a plain string constant, drop the conversion; check its declaration in this file.)

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run 'TestWorkspacePublish|TestFence' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/workspace_ops.go internal/app/rungate_fence_test.go
git commit -m "feat(app): workspace publish journals its publication identity at admission (change 0444)"
```

---

### Task 5: Pure matching predicate — uncertain entry settled by a later completed identical retry

**Files:**
- Modify: `internal/app/rungate_publication.go`
- Test: `internal/app/rungate_publication_test.go`

**Interfaces:**
- Consumes: `EpochRecord`, `AdmittedMutation`, `validPublication`, `mutationStatusUncertain`/`mutationStatusCompleted`.
- Produces: `func publicationRetryMatch(rec EpochRecord, i int) bool` (does entry `i` have a settling match?) and `func settleablePublicationIndexes(rec EpochRecord) []int` (all matched indexes, ascending). Pure — no IO, no clock.

- [ ] **Step 1: Write the failing matrix test** (acceptance items 4 and 5)

```go
// TestPublicationRetryMatchMatrix: an uncertain entry is settled ONLY by a later
// (higher-index) completed entry with the same operation and a field-for-field
// identical VALID descriptor. Everything else leaves it pending.
func TestPublicationRetryMatchMatrix(t *testing.T) {
	base := MutationPublication{
		RepoHost: "github.com", RepoOwner: "o", RepoName: "r",
		HeadRef: "fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		BaseBranch:  "main",
		TitleDigest: publicationDigest("pr-title", "t"),
		BodyDigest:  publicationDigest("pr-body", "b"),
	}
	alter := func(f func(*MutationPublication)) *MutationPublication {
		c := base
		f(&c)
		return &c
	}
	rec := func(entries ...AdmittedMutation) EpochRecord {
		return EpochRecord{AdmittedMutations: entries}
	}
	uncertain := AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusUncertain, Publication: &base}

	cases := []struct {
		name string
		rec  EpochRecord
		want bool
	}{
		{"identical completed retry settles", rec(uncertain,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Publication: &base}), true},
		{"no later entry", rec(uncertain), false},
		{"later identical but admitted", rec(uncertain,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, Publication: &base}), false},
		{"later identical but uncertain", rec(uncertain,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusUncertain, Publication: &base}), false},
		{"identical completed at LOWER index never settles", rec(
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Publication: &base},
			uncertain), false},
		{"different operation", rec(uncertain,
			AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Publication: &base}), false},
		{"missing descriptor on the retry", rec(uncertain,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted}), false},
		{"missing descriptor on the original (legacy)", rec(
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusUncertain},
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Publication: &base}), false},
		{"malformed descriptor on the retry", rec(uncertain,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted,
				Publication: alter(func(p *MutationPublication) { p.BaseBranch = "" })}), false},
		{"different owner", rec(uncertain, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted,
			Publication: alter(func(p *MutationPublication) { p.RepoOwner = "other" })}), false},
		{"different head commit", rec(uncertain, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted,
			Publication: alter(func(p *MutationPublication) { p.HeadCommit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" })}), false},
		{"different head branch", rec(uncertain, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted,
			Publication: alter(func(p *MutationPublication) { p.HeadRef = "fix/other" })}), false},
		{"different base", rec(uncertain, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted,
			Publication: alter(func(p *MutationPublication) { p.BaseBranch = "develop" })}), false},
		{"different title digest", rec(uncertain, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted,
			Publication: alter(func(p *MutationPublication) { p.TitleDigest = publicationDigest("pr-title", "T2") })}), false},
		{"different body digest", rec(uncertain, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted,
			Publication: alter(func(p *MutationPublication) { p.BodyDigest = publicationDigest("pr-body", "B2") })}), false},
		{"eligible only when uncertain: completed original is not a match target", rec(
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Publication: &base},
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Publication: &base}), false},
		{"still-admitted original is not eligible", rec(
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, Publication: &base},
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Publication: &base}), false},
	}
	for _, tc := range cases {
		if got := publicationRetryMatch(tc.rec, 0); got != tc.want {
			t.Errorf("%s: match = %v, want %v", tc.name, got, tc.want)
		}
	}

	// Workspace analog: identical settles, a differing remote/ref/repo-dir does not.
	wsBase := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	wsUncertain := AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &wsBase}
	if !publicationRetryMatch(rec(wsUncertain,
		AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Publication: &wsBase}), 0) {
		t.Error("identical completed workspace retry must settle")
	}
	for name, f := range map[string]func(*MutationPublication){
		"remote":   func(p *MutationPublication) { p.Remote = "backup" },
		"ref":      func(p *MutationPublication) { p.HeadRef = "refs/heads/other" },
		"repo dir": func(p *MutationPublication) { p.RepoDir = "/elsewhere/.git" },
		"head":     func(p *MutationPublication) { p.HeadCommit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" },
	} {
		c := wsBase
		f(&c)
		if publicationRetryMatch(rec(wsUncertain,
			AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Publication: &c}), 0) {
			t.Errorf("workspace retry differing in %s must not settle", name)
		}
	}
}

// TestSettleablePublicationIndexes: collects every settleable index, ascending, and
// nothing else.
func TestSettleablePublicationIndexes(t *testing.T) {
	base := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	r := EpochRecord{AdmittedMutations: []AdmittedMutation{
		{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &base}, // 0: settleable
		{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain},                     // 1: legacy, pending
		{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Publication: &base}, // 2: the retry
	}}
	got := settleablePublicationIndexes(r)
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("settleable = %v, want [0]", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestPublicationRetryMatch|TestSettleablePublication' -count=1`
Expected: FAIL (undefined functions).

- [ ] **Step 3: Implement**

Append to `internal/app/rungate_publication.go`:

```go
// publicationRetryMatch reports whether journal entry i is an uncertain
// publication settled by a later completed identical retry — the ONLY settling
// evidence this change accepts (spec "Match against a completed identical
// retry"). Eligibility: status uncertain AND a valid descriptor for its own
// operation. Settlement: some HIGHER-index entry in the same record with the
// same OpKey, a valid descriptor equal field-for-field, and status completed.
// A still-admitted, uncertain, lower-index, cross-operation, descriptor-less,
// or malformed candidate never settles. Pure over the record: no IO, no Git,
// no GitHub.
func publicationRetryMatch(rec EpochRecord, i int) bool {
	if i < 0 || i >= len(rec.AdmittedMutations) {
		return false
	}
	m := rec.AdmittedMutations[i]
	if m.Status != mutationStatusUncertain || !validPublication(m.OpKey, m.Publication) {
		return false
	}
	for j := i + 1; j < len(rec.AdmittedMutations); j++ {
		c := rec.AdmittedMutations[j]
		if c.Status != mutationStatusCompleted || c.OpKey != m.OpKey {
			continue
		}
		if !validPublication(c.OpKey, c.Publication) {
			continue
		}
		if *c.Publication == *m.Publication {
			return true
		}
	}
	return false
}

// settleablePublicationIndexes returns, ascending, every journal index
// publicationRetryMatch settles. The settlement writer re-derives this under
// the epoch lock; accounting callers never act on a stale copy.
func settleablePublicationIndexes(rec EpochRecord) []int {
	var idxs []int
	for i := range rec.AdmittedMutations {
		if publicationRetryMatch(rec, i) {
			idxs = append(idxs, i)
		}
	}
	return idxs
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestPublicationRetryMatch|TestSettleablePublication' -count=1`
Expected: PASS.

- [ ] **Step 5: Mutation-test the predicate** (repo rule: a guard is code)

Temporarily weaken `*c.Publication == *m.Publication` to `c.Publication.HeadCommit == m.Publication.HeadCommit`, run Step 4's command with `-count=1` — the matrix cases for owner/base/title/body/remote/ref/repo-dir MUST fail. Also temporarily drop the `c.Status != mutationStatusCompleted` guard — the admitted/uncertain cases MUST fail. Restore the real code from your editor buffer (never `git checkout --` over uncommitted work), and re-run to green.

- [ ] **Step 6: Commit**

```bash
git add internal/app/rungate_publication.go internal/app/rungate_publication_test.go
git commit -m "feat(app): pure identical-completed-retry matcher over the mutation journal (change 0444)"
```

---

### Task 6: Durable settlement writer

**Files:**
- Modify: `internal/app/rungate_publication.go`
- Test: `internal/app/rungate_publication_test.go`

**Interfaces:**
- Consumes: `epochCAS`, `errEpochFenceNoWrite`, `settleablePublicationIndexes`, `publicationRetryMatch`.
- Produces: `func settleUncertainPublications(repoDir, gateKey string) (settled []string, findings []string)` — `settled` holds `"mutation-settled:<op>"` informational tokens; `findings` holds at most `"mutation-settle-failed"` on a persistence failure. Tasks 7–8 call exactly this.

- [ ] **Step 1: Write the failing tests**

```go
// TestSettleUncertainPublicationsDurable: settlement re-derives matches under the
// epoch lock, flips ONLY matched originals uncertain→completed, is idempotent, and
// never touches unmatched entries, participants, or epoch state.
func TestSettleUncertainPublicationsDurable(t *testing.T) {
	fx := newCancelFixture(t, false)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.State = EpochCancelling // settlement is observation of fact; it works on a fenced epoch
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationPRPublish, Status: mutationStatusUncertain}, // legacy: stays pending
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}

	settled, findings := settleUncertainPublications(fx.repo, fx.key)
	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	if len(settled) != 1 || settled[0] != "mutation-settled:"+OperationWorkspacePublish {
		t.Fatalf("settled = %v, want [mutation-settled:workspace.publish]", settled)
	}

	ep, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ep.AdmittedMutations[0].Status != mutationStatusCompleted {
		t.Fatal("matched original must be durably completed — assert the RECORD changed, not a result string")
	}
	if ep.AdmittedMutations[0].Publication == nil || *ep.AdmittedMutations[0].Publication != desc {
		t.Fatal("settlement must preserve the entry's identity")
	}
	if ep.AdmittedMutations[1].Status != mutationStatusUncertain {
		t.Fatal("legacy descriptor-less entry must remain pending")
	}
	if ep.State != EpochCancelling {
		t.Fatalf("epoch state = %q; settlement must never transition the epoch", ep.State)
	}

	// Idempotent replay: nothing left to settle, no findings, record unchanged.
	settled2, findings2 := settleUncertainPublications(fx.repo, fx.key)
	if len(settled2) != 0 || len(findings2) != 0 {
		t.Fatalf("replay settled=%v findings=%v, want none", settled2, findings2)
	}
}

// TestSettleUncertainPublicationsFailureIsBoundedFinding: an unreadable epoch is a
// bounded finding, never a panic and never a fabricated settlement.
func TestSettleUncertainPublicationsFailureIsBoundedFinding(t *testing.T) {
	fx := newCancelFixture(t, false)
	// Corrupt the record so the CAS read fails closed.
	dir := filepath.Join(fx.common, "docket", "rungate", fx.key)
	if err := os.WriteFile(filepath.Join(dir, "epoch.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("corrupt record: %v", err)
	}
	settled, findings := settleUncertainPublications(fx.repo, fx.key)
	if len(settled) != 0 {
		t.Fatalf("settled = %v, want none on failure", settled)
	}
	if len(findings) != 1 || findings[0] != "mutation-settle-failed" {
		t.Fatalf("findings = %v, want [mutation-settle-failed]", findings)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run TestSettleUncertainPublications -count=1`
Expected: FAIL (undefined `settleUncertainPublications`).

- [ ] **Step 3: Implement**

```go
// settleUncertainPublications durably settles every uncertain publication entry
// proven by a later completed identical retry (change 0444). The whole
// re-read + match + write runs under one epochCAS, so the matches are
// re-derived from the FRESH record under the lock — an appended unrelated
// entry can never be cleared by an older snapshot, a raced completion
// callback or concurrent cancel serializes, and completed is never
// downgraded (the only transition is uncertain→completed on a matched
// original; identity, siblings, participants, and epoch state are untouched).
// It is invoked ONLY from authorized write paths (cancellation teardown and
// the attributed keyed successful closeout); read-only verification paths
// never call it. A persistence/read failure is a bounded finding
// ("mutation-settle-failed") — the entry stays uncertain, exclusion is
// retained, and repeating the same cancel/keyed verdict retries the write.
// A no-match pass writes nothing (errEpochFenceNoWrite).
func settleUncertainPublications(repoDir, gateKey string) (settled, findings []string) {
	err := epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		idxs := settleablePublicationIndexes(*rec)
		if len(idxs) == 0 {
			return errEpochFenceNoWrite
		}
		for _, i := range idxs {
			rec.AdmittedMutations[i].Status = mutationStatusCompleted
			settled = append(settled, "mutation-settled:"+rec.AdmittedMutations[i].OpKey)
		}
		return nil
	})
	if err != nil && !errors.Is(err, errEpochFenceNoWrite) {
		return nil, []string{"mutation-settle-failed"}
	}
	if err != nil {
		return nil, nil // nothing to settle: no write, no finding
	}
	return settled, nil
}
```

(Add `"errors"` to the file's imports. Note: on the failure return, discard any `settled` tokens accumulated inside the aborted CAS closure — the write never landed, so return `nil` for settled, exactly as written above.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run TestSettleUncertainPublications -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/rungate_publication.go internal/app/rungate_publication_test.go
git commit -m "feat(app): durable CAS settlement of retry-proven uncertain publications (change 0444)"
```

---

### Task 7: Wire settlement into cancellation, prove the stuck scenario ends (acceptance 1 and 2)

**Files:**
- Modify: `internal/app/rungate_cancel.go` (`reconcileEpochTeardown`)
- Test: `internal/app/rungate_cancel_test.go`

**Interfaces:**
- Consumes: `settleUncertainPublications` (Task 6), existing fixtures `newCancelFixture` / `runCancel` / `cancelSeams` / `fakeCancelStopper` / `okLaunchReconciler` / `hasFinding` / `loadEpochState` (all in `rungate_cancel_test.go`).
- Produces: cancellation settles before its journal check; `verifyTerminalEpochQuiescence` and `validateResumeQuiescence` then pass on the settled record with no code change of their own.

- [ ] **Step 1: Write the failing tests**

```go
// TestCancelSettlesUncertainPublicationWithIdenticalRetry (acceptance 1): an
// uncertain PR publication plus a later completed identical retry — with every
// process teardown proven — lets cancellation durably complete the original entry
// and report cancelled; the terminal epoch is then quiescent for resume and
// SupersedeCancelledEpoch admits exactly one replacement.
func TestCancelSettlesUncertainPublicationWithIdenticalRetry(t *testing.T) {
	fx := newCancelFixture(t, true)
	desc := MutationPublication{
		RepoHost: "github.com", RepoOwner: "o", RepoName: "r",
		HeadRef: "fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		BaseBranch:  "main",
		TitleDigest: publicationDigest("pr-title", "t"),
		BodyDigest:  publicationDigest("pr-body", "b"),
	}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationPRPublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")

	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q (findings %v), want cancelled", res.Disposition, res.Findings)
	}
	if hasFinding(res.Findings, "mutation-pending") {
		t.Fatalf("findings = %v, must not report the settled mutation pending", res.Findings)
	}
	ep, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ep.AdmittedMutations[0].Status != mutationStatusCompleted {
		t.Fatal("the ORIGINAL record must be durably completed, not merely the result string")
	}
	// Resume path: the terminal epoch is quiescent and admits its one replacement.
	if ok, detail := validateResumeQuiescence(cancelSeams{store: fx.store, launches: okLaunchReconciler()}, ep); !ok {
		t.Fatalf("resume quiescence = %q, want quiescent after settlement", detail)
	}
	if err := SupersedeCancelledEpoch(fx.repo, fx.key, "replacement-key"); err != nil {
		t.Fatalf("SupersedeCancelledEpoch: %v (resume must admit exactly one replacement)", err)
	}
}

// TestCancelStaysPendingWithoutCompletedIdenticalRetry (acceptance 2): a workspace
// publication settles analogously, and an uncertain entry with NO completed
// identical retry keeps cancellation-pending — then a subsequent identical
// completed retry lets the SAME pending cancellation finish (acceptance 4 tail).
func TestCancelStaysPendingWithoutCompletedIdenticalRetry(t *testing.T) {
	fx := newCancelFixture(t, true)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}

	first := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if first.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (no completed identical retry)", first.Disposition)
	}
	if !hasFinding(first.Findings, "mutation-pending:"+OperationWorkspacePublish) {
		t.Fatalf("findings = %v, want mutation-pending:workspace.publish", first.Findings)
	}

	// A subsequent identical successful retry lands in the journal; the SAME repeat
	// cancel now converges.
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.AdmittedMutations = append(r.AdmittedMutations,
			AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Publication: &desc})
		return nil
	}); err != nil {
		t.Fatalf("append retry: %v", err)
	}
	second := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if second.Disposition != CancelDispositionCancelled {
		t.Fatalf("repeat disposition = %q (findings %v), want cancelled", second.Disposition, second.Findings)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestCancelSettlesUncertain|TestCancelStaysPendingWithout' -count=1`
Expected: first test FAILS with disposition `cancellation-pending` (nothing settles yet); second test's tail FAILS the same way.

- [ ] **Step 3: Wire settlement into `reconcileEpochTeardown`**

Insert between step (5c) and step (6) — before the re-enumeration reload, so steps (6)–(7) read the settled journal:

```go
	// (5d) Settle uncertain publications proven by a later completed identical
	// retry (change 0444) — a durable, journal-derived repair with NO Git or
	// GitHub call. Runs before the re-enumeration reload so steps (6)-(7)
	// evaluate the settled record. Shared by both fencing authorities
	// (run.cancel and the death guardian): settlement is observation of durable
	// journal fact, like the completion callback. A failed settlement write is
	// a bounded finding; the entry stays uncertain and step (7) keeps the
	// cancellation pending, so exclusion is retained until a repeat converges.
	settledTokens, sfindings := settleUncertainPublications(repoDir, gateKey)
	findings = append(findings, settledTokens...)
	findings = append(findings, sfindings...)
```

No other change: step (7) already reads `reEp.AdmittedMutations` from the post-settlement reload, `verifyTerminalEpochQuiescence` / `validateResumeQuiescence` / `repairTerminalEpoch` stay read-only and see the durably settled record.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestCancel|TestRepair|TestResume' -count=1`
Expected: PASS — including all pre-existing cancel tests (`TestCancelPendingOnUncompletedMutation` still passes: its seeded entry is `admitted` with no descriptor, which never settles).

- [ ] **Step 5: Mutation-test the wiring**

Temporarily delete the (5d) block; the two new tests MUST fail (pending instead of cancelled). Restore from the editor buffer and re-run to green.

- [ ] **Step 6: Commit**

```bash
git add internal/app/rungate_cancel.go internal/app/rungate_cancel_test.go
git commit -m "fix(app): cancellation settles retry-proven uncertain publications so it can finish (change 0444)"
```

---

### Task 8: Wire settlement into the attributed successful closeout; read-only paths gain no writes (acceptance 3)

**Files:**
- Modify: `internal/app/rungate_complete.go` (`completeSuccessfulRun`)
- Test: `internal/app/rungate_complete_test.go`

**Interfaces:**
- Consumes: `settleUncertainPublications`, the existing completion fixture in `rungate_complete_test.go` (its `seams()` helper, `fakeLaunchObserver`, `completeSuccessfulRun`).
- Produces: the attributed keyed closeout settles from journal evidence and persists only journal-derived state; it still stops no task, launches no mutation, settles no never-launched reservation.

- [ ] **Step 1: Write the failing tests**

Use the completion fixture already defined at the top of `rungate_complete_test.go` (read `TestCompleteSuccessfulRunHappyPath` first and reuse its fixture constructor exactly):

```go
// TestCompleteSuccessfulRunSettlesUncertainPublication (acceptance 3): the REAL
// attributed closeout path settles an uncertain publication proven by an identical
// completed retry, then completes the epoch — with no stop sent and no mutation
// launched (the seams carry no stopper; assert the observer saw no stop calls the
// same way TestCompleteSuccessfulRunSendsNoStops does).
func TestCompleteSuccessfulRunSettlesUncertainPublication(t *testing.T) {
	fx := newCompleteFixture(t) // the file's existing fixture helper — adapt to its real name
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if !ok {
		t.Fatalf("closeout blocked: reason=%q findings=%v; a settled journal must complete", reason, findings)
	}
	ep, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ep.State != EpochCompleted {
		t.Fatalf("state = %q, want completed", ep.State)
	}
	if ep.AdmittedMutations[0].Status != mutationStatusCompleted {
		t.Fatal("the original entry must be durably settled by the closeout")
	}
}

// TestCompleteSuccessfulRunStillBlocksWithoutRetry: an uncertain publication with no
// completed identical retry keeps the closeout blocked (completion-unaccounted) —
// change 0441's fail-closed accounting is not weakened.
func TestCompleteSuccessfulRunStillBlocksWithoutRetry(t *testing.T) {
	fx := newCompleteFixture(t)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if ok || reason != "completion-unaccounted" {
		t.Fatalf("ok=%v reason=%q findings=%v, want blocked completion-unaccounted", ok, reason, findings)
	}
}

// TestReadOnlyPathsNeverSettle: the read-only verification predicates report the
// pending truth of a settleable pair but write NOTHING — the durable record is
// byte-identical after they run (RunVerify and unattributed verdicts consume these
// same predicates).
func TestReadOnlyPathsNeverSettle(t *testing.T) {
	fx := newCancelFixture(t, false)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.State = EpochCancelled
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	recPath := filepath.Join(fx.common, "docket", "rungate", fx.key, "epoch.json")
	before, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	ep, _, lerr := LoadEpochRecord(fx.repo, fx.key)
	if lerr != nil {
		t.Fatalf("LoadEpochRecord: %v", lerr)
	}
	seams := cancelSeams{launches: okLaunchReconciler()}
	verifyTerminalEpochQuiescence(seams, ep)
	validateResumeQuiescence(seams, ep)
	after, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatalf("re-read record: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("read-only verification paths must not write the epoch record")
	}
}
```

Note the read-only predicates deliberately still see the UNSETTLED entry as pending: settlement is a write, and only cancellation/attributed-closeout may write. That is the spec's rule, not a gap.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestCompleteSuccessfulRunSettles|TestCompleteSuccessfulRunStillBlocks|TestReadOnlyPathsNeverSettle' -count=1`
Expected: the settlement test FAILS (`completion-unaccounted`); the other two PASS already (they pin behavior that must not change — keep them as regression guards).

- [ ] **Step 3: Wire settlement into `completeSuccessfulRun`**

Insert immediately after the step (1) fence block (after the `observed == EpochCompleted` early return), before the step (2) reload — so both the step (3) accounting read and the step (4) re-enumeration read see the settled journal:

```go
	// (1b) Settle uncertain publications proven by a later completed identical
	// retry (change 0444) — journal-derived evidence only, persisted through the
	// ordinary epoch CAS. The attributed keyed closeout is a WRITE path (unlike
	// RunVerify and unattributed verdicts, which stay read-only), but it still
	// stops no task, launches no mutation, and settles no never-launched
	// reservation: the only write is uncertain→completed on matched journal
	// entries. A failed settlement is a bounded finding; the entry stays
	// uncertain and the accounting below blocks fail-closed as before.
	settledTokens, sfindings := settleUncertainPublications(repoDir, gateKey)
	findings = append(append(findings, settledTokens...), sfindings...)
```

(`findings` is the named return; appending here surfaces the tokens on every exit path below.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestCompleteSuccessfulRun|TestReadOnly' -count=1`
Expected: PASS, including every pre-existing `TestCompleteSuccessfulRun*` test (their fixtures journal no settleable pair, so settlement is a no-op no-write for them).

- [ ] **Step 5: Commit**

```bash
git add internal/app/rungate_complete.go internal/app/rungate_complete_test.go
git commit -m "fix(app): attributed successful closeout settles retry-proven uncertain publications (change 0444)"
```

---

### Task 9: Concurrency and interruption hardening (acceptance 6)

**Files:**
- Test: `internal/app/rungate_publication_test.go`

**Interfaces:**
- Consumes: everything above; no production change expected (the CAS design already provides these properties — these tests prove it and pin it).

- [ ] **Step 1: Write the tests**

```go
// TestSettlementNeverDowngradesUnderRacingCallback: a completion callback racing the
// settlement (both under the epoch CAS) can never regress completed→uncertain, and
// an unrelated entry appended between match and write is never cleared — the
// settlement re-derives its matches under the lock (cas-re-read-fresh-origin).
func TestSettlementNeverDowngradesUnderRacingCallback(t *testing.T) {
	fx := newCancelFixture(t, false)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	other := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/other", HeadCommit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Simulate the race: an unrelated admission lands right before settlement runs.
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.AdmittedMutations = append(r.AdmittedMutations,
			AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, Publication: &other})
		return nil
	}); err != nil {
		t.Fatalf("append racer: %v", err)
	}
	settled, findings := settleUncertainPublications(fx.repo, fx.key)
	if len(findings) != 0 || len(settled) != 1 {
		t.Fatalf("settled=%v findings=%v, want one settlement and no finding", settled, findings)
	}
	ep, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ep.AdmittedMutations[2].Status != mutationStatusAdmitted {
		t.Fatal("the racing unrelated admission must be untouched")
	}
	if ep.AdmittedMutations[1].Status != mutationStatusCompleted {
		t.Fatal("a completed entry must never be downgraded")
	}

	// Concurrent settlement + callbacks under real parallelism: run both repeatedly;
	// the flock-serialized CAS must keep every invariant.
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			settleUncertainPublications(fx.repo, fx.key)
		}()
	}
	wg.Wait()
	final, _, ferr := LoadEpochRecord(fx.repo, fx.key)
	if ferr != nil {
		t.Fatalf("final load: %v", ferr)
	}
	for i, m := range final.AdmittedMutations[:2] {
		if m.Status != mutationStatusCompleted {
			t.Fatalf("entry %d = %q after concurrent settlements, want completed", i, m.Status)
		}
	}
}

// TestSettlementInterruptionConverges: a settlement whose write failed (simulated by
// the corrupt-record bounded-finding path in Task 6) leaves the entry uncertain;
// once the record is writable again, repeating the SAME operation converges — an
// interruption after a successful write is a harmless idempotent replay (already
// proven by TestSettleUncertainPublicationsDurable's replay assert).
func TestSettlementInterruptionConverges(t *testing.T) {
	fx := newCancelFixture(t, true)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// First cancel with the epoch record made unreadable mid-flight is not modeled
	// here (the store is a single file); instead prove the FINDING path retains
	// exclusion end-to-end: chmod the key dir read-only so the CAS temp-file write
	// fails, run cancel, expect pending with mutation-settle-failed + mutation-pending.
	dir := filepath.Join(fx.common, "docket", "rungate", fx.key)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}
	res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition == CancelDispositionCancelled {
		t.Fatalf("disposition = cancelled with an unpersistable settlement; findings=%v", res.Findings)
	}
	if !hasFinding(res.Findings, "mutation-settle-failed") {
		t.Fatalf("findings = %v, want mutation-settle-failed", res.Findings)
	}
	// Writable again: the SAME repeat cancel converges.
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod back: %v", err)
	}
	res2 := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if res2.Disposition != CancelDispositionCancelled {
		t.Fatalf("repeat disposition = %q (findings %v), want cancelled", res2.Disposition, res2.Findings)
	}
}
```

(If the chmod trick fails on this store because `runCancel`'s own earlier CAS writes also need the dir — read the first cancel's actual refusal and adjust: the assertion that matters is *pending-or-refused, never cancelled, with the entry still uncertain*, then convergence once writable. `t.Cleanup` the chmod restore so a failing test never leaves an unwritable temp dir.)

- [ ] **Step 2: Run tests**

Run: `go test ./internal/app/ -run 'TestSettlementNeverDowngrades|TestSettlementInterruption' -count=1`
Expected: PASS (these pin existing CAS properties; investigate any failure as a real defect, not a test to loosen).

- [ ] **Step 3: Commit**

```bash
git add internal/app/rungate_publication_test.go
git commit -m "test(app): settlement race, downgrade, and interruption invariants (change 0444)"
```

---

### Task 10: Production wiring proof, no-adapter-call capture, leak audit, and the suite gate (acceptance 7 and 8)

**Files:**
- Test: `internal/app/rungate_fence_test.go` (wiring + capture + leak)
- Verify: `internal/repoguard` suite still green (its admission-correspondence guard anchors on `admitWorkflowMutation` by name — unchanged).

**Interfaces:**
- Consumes: everything above; the `fakeGitHub` / `fakeWorkspaceService` fixtures with call capture.

- [ ] **Step 1: Write the end-to-end wiring test**

```go
// TestProductionUncertainThenIdenticalRetryThenCancel (acceptance 7): descriptors
// journaled by the REAL PRPublish boundary — an uncertain first attempt (transport
// failure), then an identical successful retry — are matched by the REAL cancel
// path, which issues NO GitHub call: the capture adapter's ensure count does not
// move during cancellation.
func TestProductionUncertainThenIdenticalRetryThenCancel(t *testing.T) {
	fx := newCancelFixture(t, true)
	// Point the fixture epoch's worktree at a real working repo so PRPublish's
	// canonical-worktree fence resolves to this epoch — mirror how the fence tests
	// bind mintFenceEpoch's worktree, but on the cancel fixture's key (read
	// TestFenceBlocksPRPublishAfterCancel's repo setup and reuse it; bind that
	// repo path into fx's epoch via epochCAS on r.Worktree).
	repoDir := newWorkingRepo(t, nil).invocation
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.Worktree = repoDir
		return nil
	}); err != nil {
		t.Fatalf("bind worktree: %v", err)
	}

	deps := workspaceDepsFor(t, prReader(t))
	req := PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: "Authored prose.\n", EvidenceRecord: prEvidenceBytes(t, prHead)}

	// Attempt 1: transport failure -> uncertain.
	ghFail := &fakeGitHub{repo: prRepo(), ensureErr: errTransport} // reuse/introduce the transport-failure fake the existing uncertain-path fence tests use
	if res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: ghFail}, repoDir, req); res.Result != ResultExternalFailed {
		t.Fatalf("first attempt result = %q, want external-failed", res.Result)
	}
	// Attempt 2: identical request succeeds -> completed with an identical descriptor.
	ghOK := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("verified")}}
	if res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: ghOK}, repoDir, req); res.Result != ResultApplied {
		t.Fatalf("retry result = %q, want applied", res.Result)
	}
	ep, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if len(ep.AdmittedMutations) != 2 ||
		ep.AdmittedMutations[0].Status != mutationStatusUncertain ||
		ep.AdmittedMutations[1].Status != mutationStatusCompleted {
		t.Fatalf("journal = %+v, want [uncertain, completed]", ep.AdmittedMutations)
	}

	// Cancellation matches through the PRODUCTION-journaled descriptors and issues
	// no GitHub call: cancel takes no GitHub deps at all, and the adapters' capture
	// counters must not move while it runs.
	ensureCallsBefore := len(ghOK.ensureCalls) + len(ghFail.ensureCalls)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q (findings %v), want cancelled via the production-journaled match", res.Disposition, res.Findings)
	}
	if len(ghOK.ensureCalls)+len(ghFail.ensureCalls) != ensureCallsBefore {
		t.Fatal("reconciliation must issue no GitHub call")
	}
}
```

(Adapt the transport-failure construction to the `fakeGitHub` fixture's real error field — read the existing test that drives `mutationJournalStatus` to `uncertain` through `mapGitHubFailure` and reuse its exact idiom. If the two attempts must share one epoch, both `PRPublish` calls run against the same `repoDir` bound above; if the workspace-head fixture (`readyService(prHead)`) needs per-call state, construct it per call as the neighbors do.)

- [ ] **Step 2: Run it, then run the leak audit already written in Task 3**

Run: `go test ./internal/app/ -run 'TestProductionUncertain|TestPRPublishJournals' -count=1`
Expected: PASS.

- [ ] **Step 3: Repo-wide derivation check** (AGENTS.md: derive sites from grep, never memory)

Run: `grep -rn 'admitWorkflowMutation(' internal/ --include='*.go'`
Confirm: exactly the three production sites (engine hook with `nil`, PR with descriptor, workspace with descriptor) plus tests. Run `go test ./internal/repoguard/ -count=1` — the admission-correspondence guard must still pass.

- [ ] **Step 4: Full suite gate**

Resolve the command from config — read `build.test_command` from `.docket.yml` (never a second copy) — and run it from source in the feature worktree. Expected: SUITE green. Read the budget report; act on any `SERIAL CONFIRMED OVER BUDGET:` line and surface `BUDGET WATCH:` lines in the results notes.

- [ ] **Step 5: Commit**

```bash
git add internal/app/rungate_fence_test.go
git commit -m "test(app): production wiring, no-adapter-call, and leak proofs for publication reconciliation (change 0444)"
```

---

## Self-Review Notes (performed while writing)

- **Spec coverage:** descriptor capture (Tasks 1–4), matching (Task 5), durable settlement + failure semantics (Task 6), cancellation wiring + resume (Task 7), successful-closeout wiring + read-only exclusion (Task 8), races/interruption (Task 9), production wiring/no-remote-calls/leaks/legacy + suite gate (Tasks 5, 9, 10). Explicit limits need no code: no new CLI/config/daemon/store is introduced anywhere; live re-observation is structurally absent (reconciliation code imports no Git/GitHub adapter — Task 10 also proves it dynamically).
- **`PullRequest.Version` is deliberately unused** as identity (spec: `computeVersion` includes the server-assigned number); the descriptor's own fields are the identity.
- **Fixture-name caveat:** test helpers referenced from existing files (`newCompleteFixture`, `mintFenceEpoch`'s exact return shape, the transport-failure fake) must be adapted to the names actually present — each task says to read the neighboring test first. The behavior asserted is fixed; the helper spelling is not.
- **Type consistency check:** `MutationPublication` (comparable struct, `==`), `publicationDigest(label, value string) string`, `validPublication(op string, p *MutationPublication) bool`, `publicationRetryMatch(rec EpochRecord, i int) bool`, `settleablePublicationIndexes(rec EpochRecord) []int`, `settleUncertainPublications(repoDir, gateKey string) (settled, findings []string)`, `admitWorkflowMutation(repoDir, op string, pub *MutationPublication) (mutationJournalDone, error)` — used with these exact shapes in every task.
