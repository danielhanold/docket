<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0451 — Workspace publish refuses a feature head that moved after the app-level check](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0451-workspace-publish-refuses-a-feature-head-that-moved-after-th.md)**
<!-- docket:backlink:end -->
# Workspace Publish Expected-Head Refusal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use the docket-build skill to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `workspace.PublishHead` refuses under its own operation lock — pushing nothing — when the reinspected local head differs from the head the app-level check approved, closing the milliseconds-wide window in which a commit landing between `WorkspacePublish`'s Inspect and the service's reinspect gets pushed as if it were the checked head.

**Architecture:** Add an optional `ExpectedHead` field to `workspace.PublishRequest`; compare it against the reinspected head inside `PublishHead`, after `reinspectForPublish` and before any remote probe or push, while the per-workspace operation lock is held. A mismatch is the same refusal class the PR path (`internal/githubcli/ensure.go`, `EnsurePullRequest`) uses for its moved-head gate: a `failed` disposition carrying a `KindInvalidState` Failure. `WorkspacePublish` (internal/app/workspace_ops.go) passes the head it just checked (`req.Head`) as the expectation.

**Tech Stack:** Go; existing real-git test harness in `internal/workspace/publish_test.go`; fake-service unit tests in `internal/app` (untagged, plain `go test`).

**Spec:** Change 0451 is trivial — its change file's "What changes" section is the spec: `docs/changes/active/0451-workspace-publish-refuses-a-feature-head-that-moved-after-th.md` on the `docket` metadata branch.

## Global Constraints

- No force push, reset, merge, or rebase anywhere on the publish path; a mismatch pushes nothing (change body: "refuse … and push nothing").
- Match the PR path's disposition for a moved head: `EnsurePullRequest` refuses with `EnsureFailed` + `KindInvalidState` ("GitHub reports a head commit other than the expected published head"), so the workspace twin is `PublishFailed` + `&Failure{Kind: KindInvalidState}` — never `contended` (contended is reserved for observed remote interlopers).
- An empty `ExpectedHead` means "no expectation" and behaves exactly as today — the finalize path and every existing caller/test pass no expectation and must be untouched (change body, Out of scope: "Other publish or push paths").
- Learnings applied: `defaulted-param-hides-caller-wiring` (an optional field the callee defaults makes the caller's wiring invisible — Task 2 asserts the fake service RECEIVED the non-default expected head, so deleting the wiring reddens a test); `decide-and-act-on-the-same-copy` (the decision moves under the same lock as the action); `fix-reintroduces-its-own-defect-class` (the check compares the value the LOCK-HOLDING reinspect returned, not a second pre-lock read).
- Comments citing other code anchor on symbol names or quoted clauses, never line numbers (ADR-0054).
- The build gate runs the whole suite via the configured `build.test_command`; the per-task `go test` runs below are the fast inner loop only.

---

### Task 1: `ExpectedHead` on `workspace.PublishRequest` + locked comparison in `PublishHead`

**Files:**
- Modify: `internal/workspace/publish.go` (`PublishRequest` struct; `PublishHead`, immediately after the `reinspectForPublish` call)
- Test: `internal/workspace/publish_test.go`

**Interfaces:**
- Consumes: existing `Service.PublishHead`, `reinspectForPublish` (returns the reinspected head as `gitcli.ObjectID`), `Failure`, `KindInvalidState`, `PublishFailed`; test helpers `mainModeRepo`, `freshTarget`, `prepareOK`, `wsPathOf`, `commitInWorkspace`, `originFeatCommit`, `AsFailure`.
- Produces: `PublishRequest.ExpectedHead gitcli.ObjectID` — optional; when non-empty, `PublishHead` refuses (`PublishFailed`, `KindInvalidState`, stage `"verify"`) whenever the reinspected head differs, before any remote probe. Task 2 sets this field from the app layer.

- [ ] **Step 1: Write the failing test**

Append to `internal/workspace/publish_test.go`:

```go
// TestPublishExpectedHeadMoved proves PublishHead refuses under its own
// operation lock when the reinspected local head is not the caller's
// ExpectedHead — the moved-head window between the app-level check and the
// locked reinspect (change 0451). The refusal mirrors the PR path's
// moved-head gate in EnsurePullRequest ("GitHub reports a head commit other
// than the expected published head"): failed + invalid-state, and NOTHING is
// pushed — the origin feature ref stays absent.
func TestPublishExpectedHeadMoved(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)

	// The head the app-level check approved…
	checked := commitInWorkspace(t, ws, "feature.txt", "feature work\n")
	// …and a commit that lands after that check, before the locked publish.
	moved := commitInWorkspace(t, ws, "late.txt", "late work\n")
	if moved == checked {
		t.Fatalf("fixture: the second commit did not move the head")
	}

	res, err := svc.PublishHead(context.Background(), PublishRequest{
		Repository:   repo,
		Remote:       "origin",
		Target:       tgt,
		ExpectedHead: checked,
	})
	if err == nil {
		t.Fatalf("PublishHead accepted a moved head; result %+v", res)
	}
	f, ok := AsFailure(err)
	if !ok || f.Kind != KindInvalidState {
		t.Errorf("error = %v; want a Failure of kind %q", err, KindInvalidState)
	}
	if res.Disposition != PublishFailed {
		t.Errorf("Disposition = %q; want failed", res.Disposition)
	}
	if _, exists := originFeatCommit(t, r); exists {
		t.Errorf("origin feat ref exists after a refused publish; nothing must be pushed")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/workspace/ -run TestPublishExpectedHeadMoved -count=1`
Expected: FAIL to compile — `unknown field ExpectedHead in struct literal of type PublishRequest`.

- [ ] **Step 3: Implement the field and the locked comparison**

In `internal/workspace/publish.go`, extend the request struct (keeping its doc comment honest):

```go
// PublishRequest names the repository, the remote to publish to, and the fully
// validated target whose owned ready workspace HEAD is published. ExpectedHead
// is OPTIONAL: when non-empty, the reinspected local head must equal it or the
// publish is refused with nothing pushed — the caller's pre-lock head check is
// re-proven under the operation lock (change 0451). Empty means no expectation.
type PublishRequest struct {
	Repository   gitcli.Repository
	Remote       gitcli.RemoteName
	Target       Target
	ExpectedHead gitcli.ObjectID
}
```

Then in `PublishHead`, directly after the `reinspectForPublish` call returns `localHead` (before `ref := target.FeatureRef` and the remote probe):

```go
	// The caller's expected head is re-proven against the head the LOCKED
	// reinspect just returned — deciding on the same copy the push would act
	// on. A commit that landed after the app-level check moved the head, and
	// publishing it would push a commit nobody checked: refuse, push nothing.
	// Same refusal class as the PR path's moved-head gate in EnsurePullRequest
	// ("GitHub reports a head commit other than the expected published head").
	if req.ExpectedHead != "" && localHead != req.ExpectedHead {
		return PublishResult{Disposition: PublishFailed, Head: localHead},
			&Failure{Op: publishOp, Stage: "verify", Kind: KindInvalidState,
				Detail: "workspace head moved past the expected head; nothing pushed"}
	}
```

Also update the file-header flow comment: in the numbered spec-order list at the top of `publish.go`, extend item 2's sentence with `; when the request carries an expected head, the reinspected head must equal it (a moved head is refused with nothing pushed, change 0451)`.

- [ ] **Step 4: Run the test and the package to verify green**

Run: `go test ./internal/workspace/ -count=1`
Expected: PASS — the new test passes and every existing publish test (all with an empty `ExpectedHead`) is unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/workspace/publish.go internal/workspace/publish_test.go
git commit -m "feat(workspace): PublishHead refuses a head that moved past ExpectedHead (change 0451)"
```

---

### Task 2: `WorkspacePublish` passes its checked head as the expectation

**Files:**
- Modify: `internal/app/workspace_ops.go` (`WorkspacePublish`, the `workspace.PublishRequest` literal)
- Test: `internal/app/rungate_fence_test.go` (`TestWorkspacePublishJournalsPublicationIdentity`)

**Interfaces:**
- Consumes: Task 1's `PublishRequest.ExpectedHead gitcli.ObjectID`; the app file already imports `internal/gitcli`.
- Produces: the app-level publish carries `ExpectedHead: gitcli.ObjectID(req.Head)` — the same head its Inspect check and the journaled publication identity use — so the service re-proves it under the lock.

- [ ] **Step 1: Write the failing wiring assert**

In `internal/app/rungate_fence_test.go`, `TestWorkspacePublishJournalsPublicationIdentity`, directly after `call := svc.publishCalls[0]`, add:

```go
	// The adapter must be handed the SAME head the app-level check approved as
	// its locked expectation — the optional field defaults to empty, so only
	// this assert makes deleting the caller wiring redden (change 0451;
	// learnings: defaulted-param-hides-caller-wiring).
	if call.ExpectedHead != gitcli.ObjectID(head) {
		t.Fatalf("PublishHead ExpectedHead = %q, want the checked head %q", call.ExpectedHead, head)
	}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/app/ -run TestWorkspacePublishJournalsPublicationIdentity -count=1`
Expected: FAIL — `ExpectedHead = "", want the checked head "abcdef00…"` (the field compiles because Task 1 added it; the wiring does not exist yet).

- [ ] **Step 3: Wire the expectation**

In `WorkspacePublish` (internal/app/workspace_ops.go), extend the delegation literal:

```go
	res, err := wdeps.Service.PublishHead(ctx, workspace.PublishRequest{
		Repository:   wc.repo,
		Remote:       originRemote,
		Target:       target,
		ExpectedHead: gitcli.ObjectID(req.Head),
	})
```

And update the stale comment above the `if string(res.Head) != req.Head` post-push check: it currently reads "PublishHead reads its own local head under its lock, after the Inspect above, so a commit landing in between makes it act on a head other than the journaled req.Head." Reword its opening to reflect that the window is now closed while keeping the belt-and-suspenders verification, e.g.:

```go
	// PublishHead re-proves req.Head under its own lock (ExpectedHead, change
	// 0451), so a head that moved after the Inspect above is refused before any
	// push. The verified flag still independently requires the acted-on head to
	// be the journaled req.Head: a completion observed for a DIFFERENT commit
	// (or, with no head, for none we can name) is never verified evidence for
	// this entry's publication identity and can never settle an uncertain one.
```

Do not remove the `verified = false` guard itself — it stays as the journal's own invariant.

- [ ] **Step 4: Run the app package to verify green**

Run: `go test ./internal/app/ -count=1`
Expected: PASS (this package is heavy; if the machine is loaded, `-timeout 30m` is the known-safe serial confirm).

- [ ] **Step 5: Commit**

```bash
git add internal/app/workspace_ops.go internal/app/rungate_fence_test.go
git commit -m "feat(app): workspace publish hands its checked head to PublishHead as ExpectedHead (change 0451)"
```
