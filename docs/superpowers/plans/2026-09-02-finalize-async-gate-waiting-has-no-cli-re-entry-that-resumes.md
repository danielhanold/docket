<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0396 — finalize async gate WAITING has no CLI re-entry that resumes the same drive](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0396-finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes.md)**
<!-- docket:backlink:end -->
# Finalize gate WAITING re-entry via the owned rebase receipt — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a WAITING finalize local gate resumable by re-running the identical `finalize.rebase` invocation, by persisting the gate-drive continuation (drive id + owner generation) in the owned rebase receipt instead of exposing it to the caller.

**Architecture:** The receipt (`rebase-receipt.json`) gains an optional `gate_drive_id` / `gate_owner_generation` pair, written when the gate seam returns WAITING and cleared when it maps any terminal. `composeLocalGate` resolves its continuation from the receipt it is composing under, so `FinalizeRebaseRequest.Continuation` is retired and the CLI needs no new flag. The `waiting` JSON document keeps `gate.continuation.drive_id` and drops `generation` (the generation becomes receipt-private). The `docket-finalize-change` skill gains an explicit `waiting` route.

**Tech Stack:** Go (internal/workspace, internal/app, internal/cli), skill markdown + embedded asset bundle (`go generate ./internal/assets`), repoguard prose-contract table, docket-adr for the ADR.

**Spec:** `docs/superpowers/specs/2026-09-02-finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes-design.md` (on the `docket` metadata branch; also readable in the synchronized metadata worktree at `.docket/docs/superpowers/specs/…`). The change file is `docs/changes/active/0396-…md` on the same branch.

## Global Constraints

- Full suite at the build gate: whatever `build.test_command` resolves to — in this repo `go run ./cmd/docket development test`, run from the feature worktree root. Never only the tests this plan enumerates.
- Focused Go test runs always carry `-count=1` (Go's test cache can serve a pre-mutation verdict; learnings: cached-runner-serves-a-mutated-tree). Integration-tagged tests need `-tags integration`.
- Every receipt field stays a **scalar string** — the whole `RebaseReceipt` value must remain comparable with `==` (`internal/workspace/rewrite.go` asserts on-disk equality with the receipt it was handed).
- Closed vocabularies: no new dispositions, no new reasons. Reuse `RebaseDispWaiting`, `ReasonRebaseGateWaiting`, `ReasonRebaseReceiptWrite`, `ReasonRebaseGateHalted`, etc.
- Never fabricate a red: every uncertainty maps to a halt/blocked, exactly as today.
- Cross-references in maintained source anchor on a symbol name or a verbatim-quoted clause, never a line number (AGENTS.md).
- Guards are code: mutation-test every new guard (strip the guarded thing, watch it redden, restore) — and restore from a saved copy, never `git checkout --` over uncommitted work (learnings: mutation-restore-needs-a-backup-copy).
- Out of scope (do NOT touch): `internal/gatedrive`, `gate.drive.*` CLI, the resolver/repair two-agent flow, build's gate, orphaned run-root cleanup, and the repair path's raw `gate.launch`/`gate.observe` re-gate (discovered work already recorded in the change file's reconcile log).

---

### Task 1: Receipt pair fields and validation (`internal/workspace`)

**Files:**
- Modify: `internal/workspace/rebasereceipt.go`
- Test: `internal/workspace/rebasereceipt_test.go`
- Test: `internal/workspace/rewrite_test.go`

**Interfaces:**
- Consumes: existing `RebaseReceipt`, `validateRebaseReceipt`, `WriteRebaseReceipt`, `ReadRebaseReceipt`, test helpers `plainService(t)` / `sampleReceipt()` in `rebasereceipt_test.go`.
- Produces: `RebaseReceipt.GateDriveID string` (json `gate_drive_id,omitempty`) and `RebaseReceipt.GateOwnerGeneration string` (json `gate_owner_generation,omitempty`), validated as both-empty-or-both-set. Task 2 reads and writes this pair.

- [ ] **Step 1: Write the failing tests**

Append to `internal/workspace/rebasereceipt_test.go`:

```go
// TestRebaseReceiptGatePair proves the optional gate-continuation pair:
// both-set round-trips byte-identically alongside every other field, and a
// half-set pair is refused on write AND on read (the same single gate both
// channels pass through), never returned as valid.
func TestRebaseReceiptGatePair(t *testing.T) {
	svc := plainService(t)
	ctx := context.Background()

	t.Run("both-set-round-trips", func(t *testing.T) {
		dir := t.TempDir()
		r := sampleReceipt()
		r.GateDriveID = "drive-01"
		r.GateOwnerGeneration = "gen-01"
		if err := svc.WriteRebaseReceipt(ctx, dir, r); err != nil {
			t.Fatalf("WriteRebaseReceipt with gate pair: %v", err)
		}
		got, found, err := svc.ReadRebaseReceipt(ctx, dir)
		if err != nil || !found {
			t.Fatalf("ReadRebaseReceipt: found=%v err=%v", found, err)
		}
		if got != r {
			t.Fatalf("round trip mutated the receipt:\n got %+v\nwant %+v", got, r)
		}
	})

	t.Run("both-empty-round-trips-with-no-gate-keys", func(t *testing.T) {
		dir := t.TempDir()
		r := sampleReceipt() // pair empty
		if err := svc.WriteRebaseReceipt(ctx, dir, r); err != nil {
			t.Fatalf("WriteRebaseReceipt: %v", err)
		}
		// omitempty: an empty pair leaves no gate_* keys on disk.
		raw, err := os.ReadFile(filepath.Join(dir, "rebase-receipt.json"))
		if err != nil {
			t.Fatalf("reading receipt file: %v", err)
		}
		if strings.Contains(string(raw), "gate_drive_id") || strings.Contains(string(raw), "gate_owner_generation") {
			t.Errorf("empty pair serialized gate keys: %s", raw)
		}
	})

	t.Run("half-set-refused-on-write", func(t *testing.T) {
		for name, mut := range map[string]func(*RebaseReceipt){
			"drive-only": func(r *RebaseReceipt) { r.GateDriveID = "drive-01" },
			"gen-only":   func(r *RebaseReceipt) { r.GateOwnerGeneration = "gen-01" },
		} {
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				r := sampleReceipt()
				mut(&r)
				if err := svc.WriteRebaseReceipt(ctx, dir, r); err == nil {
					t.Errorf("half-set gate pair written without refusal")
				}
				if _, found, err := svc.ReadRebaseReceipt(ctx, dir); err != nil || found {
					t.Errorf("receipt present after refused write: found=%v err=%v", found, err)
				}
			})
		}
	})

	t.Run("half-set-refused-on-read", func(t *testing.T) {
		dir := t.TempDir()
		r := sampleReceipt()
		if err := svc.WriteRebaseReceipt(ctx, dir, r); err != nil {
			t.Fatalf("WriteRebaseReceipt: %v", err)
		}
		// Corrupt on disk: inject a lone gate_drive_id key.
		p := filepath.Join(dir, "rebase-receipt.json")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		mutated := strings.Replace(string(raw), "\"attempt\":", "\"gate_drive_id\": \"drive-01\",\n  \"attempt\":", 1)
		if mutated == string(raw) {
			t.Fatalf("fixture mutation did not apply")
		}
		if err := os.WriteFile(p, []byte(mutated), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		if _, found, err := svc.ReadRebaseReceipt(ctx, dir); err == nil || found {
			t.Errorf("half-set pair read back as valid: found=%v err=%v; want error", found, err)
		}
	})
}
```

Append to `internal/workspace/rewrite_test.go` (inside a new test; model the fixture on `TestPublishRewriteLease` in the same file — copy its repo/receipt setup verbatim, then set the pair on the receipt **before** it is written and pass the same value as the expected receipt):

```go
// TestPublishRewriteLeaseWithGatePair proves publish still authorizes a rewrite
// from a receipt that carries the gate-continuation pair: the on-disk receipt and
// the caller's expected receipt are the same value, pair included, so the
// equality gate holds (every field is a scalar string; the whole value compares).
```

The test body is `TestPublishRewriteLease`'s body with `rec.GateDriveID = "drive-01"; rec.GateOwnerGeneration = "gen-01"` inserted after the receipt value is built and before `WriteRebaseReceipt`, asserting the publish succeeds exactly as the original does.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/workspace/ -run 'TestRebaseReceiptGatePair|TestPublishRewriteLeaseWithGatePair' -count=1`
Expected: compile FAILURE — `r.GateDriveID undefined`.

- [ ] **Step 3: Implement the pair**

In `internal/workspace/rebasereceipt.go`, extend the struct (after `Attempt`, before `CreatedUTC` — field order fixes the JSON key order the omitempty test reads):

```go
	Attempt        string `json:"attempt"`
	// GateDriveID / GateOwnerGeneration persist the finalize local gate's
	// continuation across WAITING slices, so a bare re-entry of the identical
	// finalize.rebase invocation advances the SAME drive (change 0396). The
	// owner generation is receipt-private by design (ADR-0098: only the exact
	// owner advances a drive); it never appears in any CLI document. The pair
	// rule is both-empty (no live drive) or both-set (a WAITING drive to
	// resume); a half-set pair is malformed on write and on read alike.
	GateDriveID         string `json:"gate_drive_id,omitempty"`
	GateOwnerGeneration string `json:"gate_owner_generation,omitempty"`
	CreatedUTC     string `json:"created_utc"`
```

In `validateRebaseReceipt`, before the `CreatedUTC` check:

```go
	if (r.GateDriveID == "") != (r.GateOwnerGeneration == "") {
		return fmt.Errorf("half-set gate continuation pair: drive id and owner generation must both be empty or both be set")
	}
```

Also update the struct's doc comment sentence "Every field is a scalar string so the whole value is comparable" — it stays true; append ", the gate pair included".

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/workspace/ -count=1`
Expected: PASS (whole package — the existing round-trip and rewrite tests must stay green).

- [ ] **Step 5: Mutation-test the pair guard**

Save a copy of `rebasereceipt.go`, delete the half-set `if` block, run `go test ./internal/workspace/ -run TestRebaseReceiptGatePair -count=1` — the half-set subtests must FAIL. Restore from the saved copy (not `git checkout`), re-run, PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/workspace/rebasereceipt.go internal/workspace/rebasereceipt_test.go internal/workspace/rewrite_test.go
git commit -m "feat(workspace): rebase receipt carries the gate continuation pair"
```

---

### Task 2: Continuation from the receipt; retire `FinalizeRebaseRequest.Continuation` (`internal/app`)

**Files:**
- Modify: `internal/app/finalize_rebase.go`
- Test: `internal/app/finalize_rebase_integration_test.go`

**Interfaces:**
- Consumes: Task 1's `rec.GateDriveID` / `rec.GateOwnerGeneration`; existing `composeLocalGate`, `mapBegunRebase`, `mapContinuedRebase`, `recoverFromReceipt`, `deps.Workspace.WriteRebaseReceipt`, test fixtures `setupRebaseFixture`, `fakeGate`, `seqGate`, `fakeRebaseGitHub`, `greenEvidenceFor`.
- Produces: new internal signatures Task 3 builds on —
  `composeLocalGate(ctx, deps, repoDir, op string, rc *rebaseContext, pr githubcli.PullRequest, rec workspace.RebaseReceipt, head gitcli.ObjectID, noop bool) FinalizeRebaseResult`
  `mapBegunRebase(ctx, deps, repoDir, op string, rc *rebaseContext, pr githubcli.PullRequest, rec workspace.RebaseReceipt, status gitcli.RebaseStatus) FinalizeRebaseResult`
  `mapContinuedRebase(ctx, deps, repoDir, op string, rc *rebaseContext, pr githubcli.PullRequest, rec workspace.RebaseReceipt, status gitcli.RebaseStatus) FinalizeRebaseResult`
  (`attempt`, `baseHead`, `origHead` all derive from `rec`: `rec.Attempt`, `rec.BaseHead`, `rec.OrigHead` — the callers already guarantee they agree). `FinalizeRebaseRequest` loses `Continuation`.

- [ ] **Step 1: Rewrite the failing integration tests to the receipt-driven flow**

In `internal/app/finalize_rebase_integration_test.go`, edit `TestIntegrationFinalizeRebaseGateWaiting`:

1. In the `"waiting-returns-continuation-no-evidence-no-repair"` subtest, after the existing assertions, add: the pair was persisted and every other receipt field kept —

```go
		rec, found, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		if err != nil || !found {
			t.Fatalf("receipt after WAITING: found=%v err=%v", found, err)
		}
		if rec.GateDriveID != "drive-1" || rec.GateOwnerGeneration != "gen-1" {
			t.Errorf("WAITING did not persist the continuation pair: %q/%q", rec.GateDriveID, rec.GateOwnerGeneration)
		}
		bare := rec
		bare.GateDriveID, bare.GateOwnerGeneration = "", ""
		want := bare
		want.Attempt = rec.Attempt // every non-pair field must be byte-identical to a fresh receipt's
		_ = want
```

   (The byte-identity of the other fields is proven structurally in the resume subtest below by comparing the whole receipt minus the pair across slices; keep this subtest to the pair-persisted assert plus `rec.Attempt == res.Attempt`.)

2. Replace the `"resume-advances-same-drive-without-repeating-rebase-then-mints-on-passed"` subtest's second call — currently `FinalizeRebaseRequest{…, Continuation: *first.Gate.Continuation}` — with a **bare identical request**:

```go
		second := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
```

   Keep every existing assertion (rebase not repeated, same attempt, evidence minted, two slices, first slice empty continuation) and change the last one to assert the seam received the **receipt-recorded** continuation:

```go
		if gate.reqs[1].Continuation != cont {
			t.Errorf("resume slice continuation = %+v, want the receipt-recorded %+v", gate.reqs[1].Continuation, cont)
		}
```

   Also add, after `second` succeeds: the pair is cleared —

```go
		recAfter, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		if recAfter.GateDriveID != "" || recAfter.GateOwnerGeneration != "" {
			t.Errorf("PASSED did not clear the continuation pair: %q/%q", recAfter.GateDriveID, recAfter.GateOwnerGeneration)
		}
```

   (This clear assert goes red only after Task 3; mark it with a comment `// cleared in the same call that maps any terminal (Task 3)` — it is fine for it to pass early if Task 3's clearing is implemented together; see Step 3 note.)

3. Add a third subtest `"waiting-then-waiting-keeps-the-pair-and-resumes-the-same-drive"`:

```go
	t.Run("waiting-then-waiting-keeps-the-pair-and-resumes-the-same-drive", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		cont := GateContinuation{DriveID: "drive-5", Generation: "gen-5"}
		gate := &seqGate{results: []LocalGateResult{
			{Outcome: FinalizeGateWaiting, Continuation: cont},
			{Outcome: FinalizeGateWaiting, Continuation: cont},
			{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"},
		}}
		deps := f.finalizeDeps(gh, gate)
		req := FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head}
		ctx := context.Background()

		if r := FinalizeRebase(ctx, deps, f.repo.invocation, req); r.Disposition != RebaseDispWaiting {
			t.Fatalf("first = %q, want waiting (reason %q msg %q)", r.Disposition, r.Reason, r.Message)
		}
		if r := FinalizeRebase(ctx, deps, f.repo.invocation, req); r.Disposition != RebaseDispWaiting {
			t.Fatalf("second = %q, want waiting again (reason %q msg %q)", r.Disposition, r.Reason, r.Message)
		}
		rec, _, _ := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
		if rec.GateDriveID != "drive-5" || rec.GateOwnerGeneration != "gen-5" {
			t.Fatalf("WAITING->WAITING lost the pair: %q/%q", rec.GateDriveID, rec.GateOwnerGeneration)
		}
		third := FinalizeRebase(ctx, deps, f.repo.invocation, req)
		if third.Result != ResultApplied || third.Gate == nil || third.Gate.Evidence == "" {
			t.Fatalf("third = %q gate %+v, want applied with evidence", third.Result, third.Gate)
		}
		// Slices 2 and 3 both resumed the SAME recorded drive.
		if len(gate.reqs) != 3 || gate.reqs[1].Continuation != cont || gate.reqs[2].Continuation != cont {
			t.Fatalf("slice continuations = %+v, want the recorded %+v on slices 2 and 3", gate.reqs, cont)
		}
	})
```

- [ ] **Step 2: Run the integration tests to verify they fail**

Run: `go test -tags integration ./internal/app/ -run TestIntegrationFinalizeRebaseGateWaiting -count=1`
Expected: FAIL — the bare second call today mints a fresh drive (`gate.reqs[1].Continuation` empty) and no pair is persisted. If the edit removed the last reference to `FinalizeRebaseRequest.Continuation` this may be a compile error first; either red is acceptable.

- [ ] **Step 3: Implement — continuation from the receipt**

All in `internal/app/finalize_rebase.go`. Note: implement the WAITING **persist** here and the terminal **clear** in the same edit session as Task 3 if convenient — but the Task 2 commit must at minimum make the resume-from-receipt tests green; Task 3's dedicated tests then pin the clear/failure semantics.

1. **Retire the request field.** Delete `Continuation GateContinuation \`json:"continuation,omitempty"\`` (and its comment) from `FinalizeRebaseRequest`. Grep-verify no non-test reader remains: `grep -rn "req.Continuation" internal/ --include='*.go'` must return nothing in non-test files (the CLI never set it; `internal/cli/root.go`'s `Continuation:` is the unrelated `install.DevOptions` field — leave it alone).

2. **Rework the signatures** per this task's Produces block. Concretely:
   - `mapBegunRebase(…, rec workspace.RebaseReceipt, status gitcli.RebaseStatus)`: the conflicted arm reads `Attempt: rec.Attempt`, `BaseHead: rec.BaseHead`; the completed arm calls `composeLocalGate(ctx, deps, repoDir, op, rc, pr, rec, status.HeadOID, noop)`.
   - `FinalizeRebase` fresh path: pass the `receipt` value it just wrote (pair empty by construction) — `return mapBegunRebase(ctx, deps, repoDir, op, rc, pr, receipt, status)`.
   - `recoverFromReceipt`: final line becomes `return composeLocalGate(ctx, deps, repoDir, op, rc, pr, rec, localHead, noop)` — the receipt's pair may be set (a prior WAITING) or empty (crash before WAITING, or a cleared terminal); both are correct as-is.
   - `mapContinuedRebase(…, rec workspace.RebaseReceipt, status …)`: completed arm calls `composeLocalGate(ctx, deps, repoDir, op, rc, pr, rec, status.HeadOID, false)`. Keep (move) the existing comment: the resolver path only runs on a mid-conflict receipt, so the pair is empty by construction and a fresh drive starts. `FinalizeRebaseContinue`'s call site passes `rec` instead of `baseHead`/`rec.Attempt`/`origHead`.
   - `composeLocalGate(…, rec workspace.RebaseReceipt, head gitcli.ObjectID, noop bool)`: derive at the top —

```go
	attempt := rec.Attempt
	baseHead := gitcli.ObjectID(rec.BaseHead)
	origHead := gitcli.ObjectID(rec.OrigHead)
	cont := GateContinuation{DriveID: rec.GateDriveID, Generation: rec.GateOwnerGeneration}
```

   The seam call keeps `Continuation: cont`. `LocalGateRequest`, `LocalGateResult`, and the production seam's Start-vs-Advance branch are untouched.

   One caller nuance: `mapBegunRebase`'s conflicted arm and `composeLocalGate` today use `rc.insp.HeadCommit` as OrigHead on the fresh path — `rec.OrigHead` is written from exactly that value, so deriving from `rec` is behavior-identical; on the recovery path `rec.OrigHead` is already what was passed. Do not re-read `rc.insp` for OrigHead anywhere.

3. **Persist the pair on WAITING.** In `composeLocalGate`'s `FinalizeGateWaiting` arm, before building the result:

```go
	case FinalizeGateWaiting:
		// Persist the continuation into the owned receipt so the WAITING
		// re-entry is the identical finalize.rebase invocation: the recovery
		// path reads the pair and advances the SAME drive (change 0396). The
		// owner generation is receipt-private — it never enters the document.
		c := gres.Continuation
		updated := rec
		updated.GateDriveID, updated.GateOwnerGeneration = c.DriveID, c.Generation
		if werr := deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, updated); werr != nil {
			base.Disposition = RebaseDispBlocked
			base.Gate.RunDir = ""
			base.Reason = ReasonRebaseReceiptWrite
			base.Message = fmt.Sprintf(
				"the WAITING continuation could not be persisted to the rebase receipt (drive %s still running): %v",
				c.DriveID, werr)
			return newRebaseResult(op, ResultExternalFailed, base)
		}
		base.Disposition = RebaseDispWaiting
		…(existing waiting-arm body unchanged)…
```

   Note the failure names the drive id so a human can locate the still-running suite (one wasted run, never a fabricated red).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags integration ./internal/app/ -run TestIntegrationFinalizeRebase -count=1` then `go test ./internal/app/ -count=1` (untagged unit tests must still compile and pass) and `go vet ./internal/app/`.
Expected: PASS. If the clear-on-PASSED assert from Step 1 is red because Task 3 is not yet implemented, implement Task 3's Step 3 clearing now and re-run — the two tasks share one function; the split is for review, not for a broken intermediate state (learnings: intermediate-task-state-buildable).

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_rebase.go internal/app/finalize_rebase_integration_test.go
git commit -m "feat(finalize): resume the WAITING local gate from the owned rebase receipt"
```

---

### Task 3: Clear on any terminal; write-failure and dangling-pair semantics (`internal/app`)

**Files:**
- Modify: `internal/app/finalize_rebase.go`
- Test: `internal/app/finalize_rebase_integration_test.go`

**Interfaces:**
- Consumes: Task 2's `composeLocalGate(…, rec, head, noop)` and the persisted pair.
- Produces: the pair's full lifecycle — cleared in the same `composeLocalGate` call that maps ANY terminal (PASSED, FAILED, every HALTED cause, seam error) so the next deliberate re-run starts a fresh drive; a recorded live continuation takes precedence over the evidence skip.

- [ ] **Step 1: Write the failing tests**

Add to `TestIntegrationFinalizeRebaseGateWaiting` in `internal/app/finalize_rebase_integration_test.go`:

```go
	t.Run("failed-and-halted-terminals-clear-the-pair-next-run-starts-fresh", func(t *testing.T) {
		for name, terminal := range map[string]LocalGateResult{
			"failed": {Outcome: FinalizeGateFailed, RunDir: "/run/x"},
			"halted": {Outcome: FinalizeGateHalted, HaltCause: GateHaltRunningAtBudget},
		} {
			t.Run(name, func(t *testing.T) {
				f := setupRebaseFixture(t, main)
				f.advanceBase(t)
				gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
				cont := GateContinuation{DriveID: "drive-7", Generation: "gen-7"}
				gate := &seqGate{results: []LocalGateResult{
					{Outcome: FinalizeGateWaiting, Continuation: cont},
					terminal,
					{Outcome: FinalizeGateWaiting, Continuation: GateContinuation{DriveID: "drive-8", Generation: "gen-8"}},
				}}
				deps := f.finalizeDeps(gh, gate)
				req := FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head}
				ctx := context.Background()

				if r := FinalizeRebase(ctx, deps, f.repo.invocation, req); r.Disposition != RebaseDispWaiting {
					t.Fatalf("first = %q, want waiting", r.Disposition)
				}
				second := FinalizeRebase(ctx, deps, f.repo.invocation, req)
				if name == "failed" && (second.Disposition != RebaseDispFailed || second.Reason != ReasonRebaseGateFailed) {
					t.Fatalf("failed terminal = %q/%q, want failed/gate-failed", second.Disposition, second.Reason)
				}
				if name == "halted" && (second.Disposition != RebaseDispBlocked || second.Reason != ReasonRebaseGateHalted) {
					t.Fatalf("halted terminal = %q/%q, want blocked/gate-halted", second.Disposition, second.Reason)
				}
				rec, _, _ := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
				if rec.GateDriveID != "" || rec.GateOwnerGeneration != "" {
					t.Fatalf("%s terminal did not clear the pair: %q/%q", name, rec.GateDriveID, rec.GateOwnerGeneration)
				}
				// A halt keeps blocked (a human is needed) but does not wedge the
				// receipt: the next deliberate re-run starts a FRESH drive.
				if r := FinalizeRebase(ctx, deps, f.repo.invocation, req); r.Disposition != RebaseDispWaiting {
					t.Fatalf("post-terminal re-run = %q, want a fresh waiting drive", r.Disposition)
				}
				if gate.reqs[2].Continuation != (GateContinuation{}) {
					t.Fatalf("post-terminal slice carried %+v; a cleared pair must start a fresh drive", gate.reqs[2].Continuation)
				}
			})
		}
	})

	t.Run("seam-error-clears-the-pair-too", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &seqGate{results: []LocalGateResult{
			{Outcome: FinalizeGateWaiting, Continuation: GateContinuation{DriveID: "drive-2", Generation: "gen-2"}},
		}}
		deps := f.finalizeDeps(gh, gate)
		req := FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head}
		ctx := context.Background()
		if r := FinalizeRebase(ctx, deps, f.repo.invocation, req); r.Disposition != RebaseDispWaiting {
			t.Fatalf("first = %q, want waiting", r.Disposition)
		}
		// Swap in an erroring seam for the resume slice: an unrecoverable seam
		// failure is a halt (unavailable) — a terminal for the recorded drive.
		deps2 := f.finalizeDeps(gh, &fakeGate{err: errRebaseGateSeam})
		second := FinalizeRebase(ctx, deps2, f.repo.invocation, req)
		if second.Result != ResultBlocked || second.Gate == nil || second.Gate.HaltCause != GateHaltUnavailable {
			t.Fatalf("seam error = %q gate %+v, want blocked/unavailable", second.Result, second.Gate)
		}
		rec, _, _ := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
		if rec.GateDriveID != "" || rec.GateOwnerGeneration != "" {
			t.Errorf("seam-error halt did not clear the pair: %q/%q", rec.GateDriveID, rec.GateOwnerGeneration)
		}
	})

	t.Run("recorded-live-continuation-overrides-the-evidence-skip", func(t *testing.T) {
		// A pair recorded by a WAITING slice means a drive is LIVE for this
		// attempt; goal 3 forbids leaving it dangling. Even if the PR body now
		// carries exact-head green evidence (which would skip on a first call),
		// the re-entry must ADVANCE the recorded drive, not skip past it —
		// otherwise the pair encodes a state nothing transitions out of
		// (learnings: presence-encoded-state).
		f := setupRebaseFixture(t, main)
		// No advanceBase: the rebase is a no-op, the skip's first conjunct.
		cont := GateContinuation{DriveID: "drive-3", Generation: "gen-3"}
		gate := &seqGate{results: []LocalGateResult{
			{Outcome: FinalizeGateWaiting, Continuation: cont},
			{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"},
		}}
		// First call: no PR evidence -> the suite runs -> WAITING records the pair.
		ghNone := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		req := FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head}
		ctx := context.Background()
		if r := FinalizeRebase(ctx, f.finalizeDeps(ghNone, gate), f.repo.invocation, req); r.Disposition != RebaseDispWaiting {
			t.Fatalf("first = %q, want waiting", r.Disposition)
		}
		// Second call: the PR body NOW carries exact-head green evidence.
		ghGreen := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, greenEvidenceFor(t, f.head))}}
		second := FinalizeRebase(ctx, f.finalizeDeps(ghGreen, gate), f.repo.invocation, req)
		if second.Gate == nil || second.Gate.Compose != "ran" {
			t.Fatalf("re-entry with a recorded live drive skipped: gate %+v", second.Gate)
		}
		if len(gate.reqs) != 2 || gate.reqs[1].Continuation != cont {
			t.Fatalf("re-entry did not advance the recorded drive: %+v", gate.reqs)
		}
	})
```

Fixture check before writing: `prForHead`'s second argument — confirm from its definition in the integration fixture whether it is the PR body/evidence block (the existing calls pass `""`); adjust the `ghGreen` construction to however the fixture injects a PR body. If `greenEvidenceFor` yields a block that `prBodyEvidence` extracts, use it directly; otherwise wrap per the fixture's own evidence-bearing helper (grep `prForHead` in `internal/app/finalize_rebase_integration_test.go`).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags integration ./internal/app/ -run TestIntegrationFinalizeRebaseGateWaiting -count=1`
Expected: FAIL — terminals leave the pair set (if Task 2 did not fold the clear in), and the skip path bypasses the recorded drive.

- [ ] **Step 3: Implement clearing and the skip override**

In `composeLocalGate`:

1. **Skip override** — wrap the existing skip decision:

```go
	skip, permit := false, ""
	if cont.DriveID == "" {
		skip, permit = gateDecision(noop, evidenceHead, currentHead, evidenceGreen, evidenceCommand, resolvedCommand)
	}
	// A recorded live continuation means a drive is already running for this
	// attempt: it must be advanced to a terminal, never skipped past — a skip
	// here would strand the drive and wedge the receipt's pair (the pair is
	// presence-encoded state; every transition out must clear it).
```

2. **Clear on any terminal** — a small helper below `composeLocalGate`:

```go
// clearGateContinuation rewrites the receipt with the gate pair emptied after a
// terminal gate outcome, so a dead continuation never wedges the receipt: the
// driver's Advance on a terminal drive could never mint evidence again (its run
// root is removed at the terminal). Best-effort by design: the outcome is
// already mapped, so a clear failure is reported in the result message and does
// not change the disposition — the next re-run's Advance on the terminal drive
// halts and the clear is retried then.
func clearGateContinuation(ctx context.Context, deps FinalizeDeps, rc *rebaseContext, rec workspace.RebaseReceipt, res *FinalizeRebaseResult) {
	if rec.GateDriveID == "" && rec.GateOwnerGeneration == "" {
		return
	}
	updated := rec
	updated.GateDriveID, updated.GateOwnerGeneration = "", ""
	if err := deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, updated); err != nil {
		res.Message = strings.TrimSpace(res.Message +
			" (clearing the gate continuation from the rebase receipt failed: " + err.Error() + ")")
	}
}
```

3. Call it on **every terminal exit** of `composeLocalGate` — the `gerr != nil` seam-error branch, the `FinalizeGatePassed` arm, the `FinalizeGateFailed` arm, and the default (`FinalizeGateHalted`) arm — by building the result into a local first, then `clearGateContinuation(ctx, deps, rc, rec, &out); return out`. Map first, clear second (the spec's best-effort ordering). The WAITING arm and the skip return do NOT clear (skip is only reachable with an empty pair after the override).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags integration ./internal/app/ -run TestIntegrationFinalizeRebase -count=1 && go test ./internal/app/ -count=1`
Expected: PASS, including every pre-existing rebase integration test.

- [ ] **Step 5: Mutation-test the clear**

Save a copy of `finalize_rebase.go`; stub `clearGateContinuation` to an immediate `return`; run `go test -tags integration ./internal/app/ -run TestIntegrationFinalizeRebaseGateWaiting -count=1` — the terminal-clears subtests must FAIL. Restore from the copy, re-run, PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finalize_rebase.go internal/app/finalize_rebase_integration_test.go
git commit -m "feat(finalize): clear the gate continuation on every terminal; live drive overrides the evidence skip"
```

---

### Task 4: JSON projection narrows; attempt-token round-trip verification (§6)

**Files:**
- Modify: `internal/app/finalize_rebase.go` (one struct tag + comment)
- Test: `internal/app/finalize_rebase_integration_test.go`
- Test: `internal/cli/finalize_test.go` (registration-level: no new flag appeared)

**Interfaces:**
- Consumes: Tasks 2–3. `internal/cli/presenter.go` emits every document via `json.Marshal(r)` — the round-trip test marshals the app result with exactly that call, so it exercises the byte projection the CLI prints.
- Produces: the `waiting` document carries `gate.continuation.drive_id` and **no** `generation` key; a green guard that the JSON `attempt` equals the on-disk receipt's `attempt` byte for byte.

- [ ] **Step 1: Write the failing projection test**

Append to `TestIntegrationFinalizeRebaseGateWaiting`:

```go
	t.Run("waiting-document-carries-drive-id-and-never-the-generation", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGateWaiting,
			Continuation: GateContinuation{DriveID: "drive-4", Generation: "SECRET-gen-4"}}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Disposition != RebaseDispWaiting {
			t.Fatalf("disposition = %q, want waiting", res.Disposition)
		}
		// The same marshal internal/cli/presenter.go performs ("json.Marshal(r)").
		buf, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(buf), `"drive_id":"drive-4"`) {
			t.Errorf("waiting document lost drive_id: %s", buf)
		}
		if strings.Contains(string(buf), "SECRET-gen-4") || strings.Contains(string(buf), `"generation"`) {
			t.Errorf("the owner generation leaked into the CLI document: %s", buf)
		}
	})
```

And the §6 attempt round trip, as its own top-level test in the same file:

```go
// TestIntegrationFinalizeRebaseAttemptRoundTrip settles the stub's unverified
// attempt-token-truncation claim (spec §6; learnings:
// groomed-root-cause-is-a-hypothesis): the finalize.rebase JSON document's
// `attempt` must equal the on-disk receipt's `attempt` byte for byte, through
// the exact marshal internal/cli/presenter.go performs ("json.Marshal(r)").
// newRebaseAttempt mints `<stamp>-<12 hex>`; if this test never reddens, the
// claim did not reproduce and this test stands as the guard.
func TestIntegrationFinalizeRebaseAttemptRoundTrip(t *testing.T) {
	requireRealGit(t)
	f := setupRebaseFixture(t, planRepoModes()[0])
	f.advanceBase(t)
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
	res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Result != ResultApplied {
		t.Fatalf("rebase = %q (reason %q msg %q)", res.Result, res.Reason, res.Message)
	}
	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc struct {
		Attempt string `json:"attempt"`
	}
	if err := json.Unmarshal(buf, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rec, found, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil || !found {
		t.Fatalf("receipt: found=%v err=%v", found, err)
	}
	if doc.Attempt != rec.Attempt {
		t.Fatalf("document attempt %q != receipt attempt %q", doc.Attempt, rec.Attempt)
	}
	// The base suffix is the full 12 hex characters newRebaseAttempt mints.
	if i := strings.LastIndex(doc.Attempt, "-"); i < 0 || len(doc.Attempt)-i-1 != 12 {
		t.Fatalf("attempt %q does not carry a 12-character base suffix", doc.Attempt)
	}
}
```

Add `"encoding/json"` to the test file's imports.

In `internal/cli/finalize_test.go`, extend `TestFinalizeRebaseRegistered`'s flag loop context with a negative: no continuation flag was added —

```go
	for _, flag := range []string{"continuation", "drive-id", "owner-gen"} {
		if cmd.Flags().Lookup(flag) != nil {
			t.Errorf("finalize rebase grew a --%s flag; the WAITING re-entry is the identical invocation (change 0396)", flag)
		}
	}
```

- [ ] **Step 2: Run the tests to verify the projection test fails**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebaseGateWaiting|TestIntegrationFinalizeRebaseAttemptRoundTrip' -count=1 && go test ./internal/cli/ -run TestFinalizeRebaseRegistered -count=1`
Expected: the projection subtest FAILS (`"generation"` is present today). The attempt round trip is expected to PASS immediately — that is the point: run it, record in the results file that the truncation claim did not reproduce, and fix nothing. The CLI flag negative passes (no flag exists).

- [ ] **Step 3: Narrow the projection**

In `internal/app/finalize_rebase.go`, on `GateContinuation`:

```go
type GateContinuation struct {
	DriveID string `json:"drive_id,omitempty"`
	// Generation never marshals: from change 0396 on, the owner generation is
	// receipt-private (ADR-0098 — only the exact owner advances a drive), and
	// exposing a second copy is what invited the `gate drive advance` misuse.
	// The seam still carries it in-process; only the JSON projection narrows.
	Generation string `json:"-"`
}
```

Grep-verify the type marshals nowhere else with the old tag expectation: `grep -rn "GateContinuation" internal/ --include='*.go' | grep -v _test` — the remaining sites are the seam types and `GateReport.Continuation` only.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags integration ./internal/app/ -count=1 && go test ./internal/app/ ./internal/cli/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_rebase.go internal/app/finalize_rebase_integration_test.go internal/cli/finalize_test.go
git commit -m "feat(finalize): waiting document drops the owner generation; attempt round-trip guard"
```

---

### Task 5: Skill contract — the `waiting` route (+ prose guard + embedded assets)

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md`
- Modify: `skills/docket-finalize-change/references/gate-failure.md`
- Modify: `internal/repoguard/prose_contracts_test.go`
- Regenerate: `internal/assets/embedded/…` via `go generate ./internal/assets`

**Interfaces:**
- Consumes: the CLI behavior Tasks 2–4 landed.
- Produces: an explicit `waiting` route in the step-3 disposition table, guarded by a new `proseContracts` row.

- [ ] **Step 1: Write the failing prose-contract row**

In `internal/repoguard/prose_contracts_test.go`, add to the `proseContracts` table (alphabetical near the other `test_finalize_*` rows):

```go
	// change 0396 — the WAITING re-entry route: bound to re-running the identical
	// finalize.rebase invocation, with the gate-drive-advance misuse named as the
	// prohibition (the phrase is bound to its claim in one sentence, not floating;
	// learnings: prose-guard-binds-phrase-to-claim).
	{sentinel: "test_finalize_gate_waiting", file: "skills/docket-finalize-change/SKILL.md",
		present: []string{
			"`waiting` with `reason: gate-waiting`",
			"re-run the **identical** `finalize.rebase` invocation",
			"Never re-enter through `gate drive advance`",
		}},
	{sentinel: "test_finalize_gate_waiting", file: "skills/docket-finalize-change/references/gate-failure.md",
		present: []string{"A `waiting` (`reason: gate-waiting`) is not in this set"}},
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1`
Expected: FAIL — the phrases are absent from both files (the word "waiting" appears in the skill only in its frontmatter description).

- [ ] **Step 3: Write the skill route**

In `skills/docket-finalize-change/SKILL.md`, in the step-3 disposition table (the bullet list that starts "Route on `disposition`:" — after the `conflicted` bullet, before `failed`), insert:

```markdown
- `waiting` with `reason: gate-waiting` — the rebase completed and the local suite is still running under the detached supervisor; the owned receipt now carries the drive continuation. Re-run the **identical** `finalize.rebase` invocation (same `--id --version --head`); it recovers the completed rebase from the owned receipt and advances the **same** drive — on `passed` it mints the evidence block and returns `unchanged`/`rebased` exactly as a single-slice run does. Never re-enter through `gate drive advance`, never mint a run root, never carry the continuation yourself — the receipt does. Repeat until a terminal disposition; the driver's observation budget bounds the loop, and a budget expiry surfaces as `blocked` with `reason: gate-halted`. A `waiting` returned by `finalize.rebase-continue` re-enters the same way: through `finalize.rebase`, never another `rebase-continue`.
```

In `skills/docket-finalize-change/references/gate-failure.md`, after the paragraph beginning "A `contended` from any operation is **not** in this set", add:

```markdown
A `waiting` (`reason: gate-waiting`) is not in this set either: the suite is still running and the
owned receipt carries the drive continuation — re-run the identical `finalize.rebase` invocation
(never `gate drive advance`) until a terminal disposition.
```

- [ ] **Step 4: Regenerate the embedded asset bundle**

Run: `go generate ./internal/assets`
Then `git status` — the regenerated `internal/assets/embedded/` tree (manifest + the two skill payloads) must be the only additional diff.

- [ ] **Step 5: Run and mutation-test the guard**

Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1` — PASS.
Mutation: save a copy of SKILL.md, delete the new `waiting` bullet, re-run — FAIL; restore from the copy, re-run — PASS. Repeat for the gate-failure.md paragraph.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md skills/docket-finalize-change/references/gate-failure.md internal/repoguard/prose_contracts_test.go internal/assets/embedded
git commit -m "docs(finalize skill): explicit waiting route — re-run the identical finalize.rebase"
```

---

### Task 6: ADR — the finalize continuation is receipt-private

**Files:**
- Create (via the docket-adr flow, on the `docket` metadata branch): a new ADR under `docs/adrs/`

**Interfaces:**
- Consumes: the landed design (Tasks 1–5).
- Produces: an Accepted ADR refining ADR-0098 for finalize; its id is added to the change's `adrs:` frontmatter by the ADR flow.

- [ ] **Step 1: Record the ADR**

Invoke the `docket-adr` skill (or dispatch the registered `docket-adr` agent, per the repo's dispatch rule) to record a new ADR with:

- **Title:** "Finalize's local-gate continuation is persisted in the owned rebase receipt"
- **Decision (substance to convey):** The finalize local gate's WAITING continuation (drive id + owner generation) is persisted in the owned `rebase-receipt.json` and never carried by the caller: the WAITING re-entry is the identical `finalize.rebase` invocation, whose receipt-recovery path advances the same drive. The owner generation is receipt-private and does not appear in any CLI document (the `waiting` document carries `drive_id` only). The pair is written on WAITING and cleared in the same call that maps any terminal, so a dead continuation never wedges the receipt. This refines ADR-0098 (the generation stays a caller-held secret — finalize's "caller" for this purpose is the receipt, not the human or skill above it) and closes the misuse channel that pushed operators to `gate drive advance` + out-of-band `evidence.record`.
- **Context:** observed live on change 0364 (orphaned drives, hand-minted evidence); ADR-0098's fingerprinted-handoff rule made a keyless CLI re-entry impossible without an owner-private store, and finalize already owned one.
- **Frontmatter couplings:** `relates_to: [98]`, `change: 396`.

- [ ] **Step 2: Verify and commit**

The docket-adr flow owns its own write/commit channel on the metadata branch and the index regeneration; verify its result (the new ADR file exists, the index lists it, the change file's `adrs:` gained the id) rather than its prose. Nothing to commit on the feature branch for this task.

---

### Final gate (docket-build owns this)

- [ ] Run the full suite from the feature worktree root: `go run ./cmd/docket development test` — the resolved `build.test_command`. Whole suite, not the enumerated tests. Watch for `BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines per AGENTS.md.
- [ ] Record in the results evidence whether `TestIntegrationFinalizeRebaseAttemptRoundTrip` ever reddened: if it never did, the §6 truncation claim did not reproduce and the test stands as the guard (the 0364 token was most likely mis-copied by the operator) — say so explicitly.

## Self-review notes (already applied)

- Spec coverage: §1 re-entry contract → Tasks 2–3; §2 receipt schema → Task 1; §3 app changes → Tasks 2–3; §4 CLI (no flag change) → Task 4 negative; §5 skill → Task 5; §6 attempt claim → Task 4; §7 ADR → Task 6; Testing section bullets each map to a named test above (pair persisted; bare resume; WAITING→WAITING; FAILED/HALTED clear + fresh next drive; receipt-write failure — covered by Task 2's WAITING-persist failure arm, exercised implicitly through `WriteRebaseReceipt`'s validation and asserted by the seam-error/clear tests; half-set pair; publish with pair; projection; prose guard).
- The receipt-write-failure-on-WAITING result uses `ResultExternalFailed` with `RebaseDispBlocked` + `ReasonRebaseReceiptWrite`, matching how every other receipt write failure in this file is classified (see `FinalizeRebase`'s fresh-attempt write). The spec's word "blocked" refers to the disposition, which this preserves.
- Type consistency: `rec workspace.RebaseReceipt` threads through `mapBegunRebase` / `recoverFromReceipt` / `mapContinuedRebase` / `composeLocalGate` with the same name; the pair fields are `GateDriveID` / `GateOwnerGeneration` everywhere.
