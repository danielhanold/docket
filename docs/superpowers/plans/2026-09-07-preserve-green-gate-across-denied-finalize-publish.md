<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0408 — Finalize publish is denied by the auto-mode classifier whenever the gate rebases](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-08-0408-finalize-publish-is-denied-by-the-auto-mode-classifier-whene.md)**
<!-- docket:backlink:end -->
# Preserve a Still-Valid Green Gate Across a Denied Finalize Publish — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist a completed-gate publish checkpoint in the owned rebase receipt when finalize's local gate passes for a rebased head, and reuse it on resume to skip the suite and go straight to publish when every recorded identity still matches — so a denied publish never discards a passed green gate.

**Architecture:** Six new all-scalar-string checkpoint fields on `workspace.RebaseReceipt` (tested head, effective base head, byte-exact resolved `finalize.test_command`, gate policy, PR number, green evidence block), validated all-or-none and mutually exclusive with the live gate-continuation pair. `composeLocalGate` records the checkpoint in the same atomic receipt write that clears the continuation pair at a PASSED terminal for a real rebase; `recoverFromReceipt` consults it on resume through a pure `checkpointDecision` and either returns a `skipped` gate report carrying the recorded evidence (no suite invocation) or clears the stale checkpoint and re-runs the gate exactly as today. `PublishRewrite` and `FinalizePublish` are untouched — the receipt stays `==`-comparable, so the existing decide-and-act-on-the-same-copy equality gates hold with the new fields for free.

**Tech Stack:** Go; real-Git integration test fixtures already in `internal/app` (`//go:build integration`) and `internal/workspace`; `internal/evidence` for record verification.

**Spec:** `docs/superpowers/specs/2026-09-07-finalize-publish-is-denied-by-the-auto-mode-classifier-whene-design.md` (synchronized copy under `.docket/`; the backlinked change is `docs/changes/active/0408-finalize-publish-is-denied-by-the-auto-mode-classifier-whene.md` on the `docket` branch).

## Global Constraints

- The receipt stays a flat struct of comparable scalar fields — `RebaseReceipt` must remain `==`-comparable (its doc comment promises it; `PublishRewrite`'s `onDisk != req.Receipt` gate and the write round-trip guard `reloaded != r` depend on it). No nested structs, no slices, no maps.
- `PublishRewrite` (`internal/workspace/rewrite.go`) and `FinalizePublish` (`internal/app/finalize_publish.go`) change in NO way: the exact ref-and-old-OID lease, the noop/contended/unknown response-loss semantics, and the authored-PR-body preservation via `evidence.Upsert` are preserved byte-for-byte. Task 5 proves compatibility; it does not modify them.
- The receipt's identity and moved-base refusals in `recoverFromReceipt` remain in force AHEAD of any checkpoint reuse — a moved base still blocks with `ReasonRebaseMovedBase` exactly as today, checkpoint or not.
- Reuse applies only to still-valid evidence: any mismatch (head, base, resolved command, gate policy, PR identity, evidence no longer verifying) invalidates the checkpoint and the gate re-runs. Skipped (non-green) evidence never waives the gate.
- No new Go primitive distinguishing a host denial from a Go result — a host denial means the binary never ran; that distinction stays in the finalize skill/harness. This plan touches only receipt persistence and the rebase-resume policy.
- Heads are compared as full-length lowercase hex object ids (the existing `gateDecision` convention: the caller normalizes with `strings.ToLower`).
- Every guard is mutation-tested: strip what it guards, watch the assert redden, restore. Always defeat Go's test cache with `-count=1` when re-running after a mutation.
- Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers.
- The final gate runs the WHOLE suite via the command `build.test_command` resolves to (in this repo `go run ./cmd/docket development test`, from the checkout under review), never just the tests this plan enumerates.
- Commit messages follow the repo idiom: `feat(0408): …` / `test(0408): …` / `docs(0408): …`.

## File Structure

- Modify: `internal/workspace/rebasereceipt.go` — six checkpoint fields + validation (Task 1).
- Modify: `internal/workspace/rebasereceipt_test.go` — checkpoint validation/round-trip tests (Task 1).
- Modify: `internal/app/finalize_rebase.go` — pure `publishCheckpoint`/`checkpointDecision` helpers, `resolvedFinalizeGateConfig`, PASSED-terminal recording, resume reuse/invalidation (Tasks 2–4).
- Modify: `internal/app/finalize_rebase_test.go` — pure-function tables + the `headEvidenceGate` fake (Tasks 2, 3).
- Modify: `internal/app/finalize_rebase_integration_test.go` — record/reuse/invalidation behavioral tests (Tasks 3, 4).
- Modify: `internal/workspace/rewrite_test.go` — checkpoint-bearing receipt through `PublishRewrite` (Task 5).
- Modify: `internal/app/finalize_publish_test.go` — pin that the publish fixture now carries a checkpoint; end-to-end denied-publish resume (Task 5).
- Create (via the docket-adr skill, metadata branch): `docs/adrs/0112-…` — the completed-evidence publish checkpoint decision (Task 6).

Run integration-tagged app tests with `-tags integration`; untagged tests run plain. All test commands in this plan are run from the feature worktree root.

---

### Task 1: Receipt checkpoint fields and validation

**Files:**
- Modify: `internal/workspace/rebasereceipt.go`
- Test: `internal/workspace/rebasereceipt_test.go`

**Interfaces:**
- Consumes: existing `RebaseReceipt`, `validateRebaseReceipt`, `WriteRebaseReceipt`/`ReadRebaseReceipt`.
- Produces: six new string fields on `workspace.RebaseReceipt` — `PublishCheckpointHead`, `PublishCheckpointBaseHead`, `PublishCheckpointCommand`, `PublishCheckpointGate`, `PublishCheckpointPRNumber`, `PublishCheckpointEvidence` — all `omitempty`, validated all-or-none, mutually exclusive with a set `GateDriveID`. Later tasks read and write these exact names.

- [ ] **Step 1: Write the failing tests**

Append to `internal/workspace/rebasereceipt_test.go` (same file, same imports — `strings`, `os`, `filepath`, `testsupport` are already imported):

```go
// checkpointReceipt is sampleReceipt carrying a fully-set publish checkpoint:
// the completed-gate evidence for a rebased head, recorded so a denied publish
// can resume without re-running the suite (change 0408).
func checkpointReceipt() RebaseReceipt {
	r := sampleReceipt()
	r.PublishCheckpointHead = strings.Repeat("d", 40)
	r.PublishCheckpointBaseHead = strings.Repeat("c", 40)
	r.PublishCheckpointCommand = "go test ./..."
	r.PublishCheckpointGate = "local"
	r.PublishCheckpointPRNumber = "7"
	r.PublishCheckpointEvidence = "<!-- evidence -->\nresult: green\n"
	return r
}

// TestRebaseReceiptPublishCheckpoint proves the optional publish checkpoint:
// fully-set round-trips byte-identically, fully-empty serializes no
// publish_checkpoint_* keys, a partially-set checkpoint is refused on write,
// a malformed member (bad head, non-positive PR number) is refused, and a
// checkpoint coexisting with a live gate-continuation pair is refused — a
// completed terminal and a live drive are mutually exclusive states.
func TestRebaseReceiptPublishCheckpoint(t *testing.T) {
	svc := plainService(t)
	ctx := context.Background()

	t.Run("fully-set-round-trips", func(t *testing.T) {
		dir := testsupport.TempDir(t)
		r := checkpointReceipt()
		if err := svc.WriteRebaseReceipt(ctx, dir, r); err != nil {
			t.Fatalf("WriteRebaseReceipt with checkpoint: %v", err)
		}
		got, found, err := svc.ReadRebaseReceipt(ctx, dir)
		if err != nil || !found {
			t.Fatalf("ReadRebaseReceipt: found=%v err=%v", found, err)
		}
		if got != r {
			t.Fatalf("round trip mutated the receipt:\n got %+v\nwant %+v", got, r)
		}
	})

	t.Run("empty-checkpoint-serializes-no-keys", func(t *testing.T) {
		dir := testsupport.TempDir(t)
		if err := svc.WriteRebaseReceipt(ctx, dir, sampleReceipt()); err != nil {
			t.Fatalf("WriteRebaseReceipt: %v", err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "rebase-receipt.json"))
		if err != nil {
			t.Fatalf("reading receipt file: %v", err)
		}
		if strings.Contains(string(raw), "publish_checkpoint") {
			t.Errorf("empty checkpoint serialized publish_checkpoint keys: %s", raw)
		}
	})

	t.Run("partial-and-malformed-refused-on-write", func(t *testing.T) {
		for name, mut := range map[string]func(*RebaseReceipt){
			"head-only":       func(r *RebaseReceipt) { *r = sampleReceipt(); r.PublishCheckpointHead = strings.Repeat("d", 40) },
			"missing-evidence": func(r *RebaseReceipt) { *r = checkpointReceipt(); r.PublishCheckpointEvidence = "" },
			"bad-head":        func(r *RebaseReceipt) { *r = checkpointReceipt(); r.PublishCheckpointHead = "not-a-sha" },
			"bad-base-head":   func(r *RebaseReceipt) { *r = checkpointReceipt(); r.PublishCheckpointBaseHead = "nope" },
			"zero-pr-number":  func(r *RebaseReceipt) { *r = checkpointReceipt(); r.PublishCheckpointPRNumber = "0" },
			"non-numeric-pr":  func(r *RebaseReceipt) { *r = checkpointReceipt(); r.PublishCheckpointPRNumber = "seven" },
			"live-drive-coexists": func(r *RebaseReceipt) {
				*r = checkpointReceipt()
				r.GateDriveID, r.GateOwnerGeneration = "drive-01", "gen-01"
			},
		} {
			t.Run(name, func(t *testing.T) {
				dir := testsupport.TempDir(t)
				var r RebaseReceipt
				mut(&r)
				if err := svc.WriteRebaseReceipt(ctx, dir, r); err == nil {
					t.Errorf("invalid checkpoint written without refusal")
				}
				if _, found, err := svc.ReadRebaseReceipt(ctx, dir); err != nil || found {
					t.Errorf("receipt present after refused write: found=%v err=%v", found, err)
				}
			})
		}
	})
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/workspace/ -run TestRebaseReceiptPublishCheckpoint -count=1`
Expected: compile FAIL — `r.PublishCheckpointHead undefined`.

- [ ] **Step 3: Implement the fields and validation**

In `internal/workspace/rebasereceipt.go`:

Add `"strconv"` to the imports.

Add to the `RebaseReceipt` struct, after `GateOwnerGeneration` and before `CreatedUTC`:

```go
	// PublishCheckpoint* persist the COMPLETED-gate publish checkpoint (change
	// 0408): when finalize's local gate reaches PASSED for a real (rewritten)
	// rebase, the tested head, the effective base head, the byte-exact resolved
	// finalize.test_command, the gate policy, the open PR number, and the green
	// evidence block are recorded so a resume after a denied publish can reuse
	// the still-valid evidence instead of re-running the suite. The field rule
	// is all-empty (no checkpoint) or all-set (a completed gate to reuse); a
	// partially-set checkpoint is malformed on write and on read alike, and a
	// checkpoint never coexists with a live gate-continuation pair — a terminal
	// and a live drive are mutually exclusive.
	PublishCheckpointHead     string `json:"publish_checkpoint_head,omitempty"`
	PublishCheckpointBaseHead string `json:"publish_checkpoint_base_head,omitempty"`
	PublishCheckpointCommand  string `json:"publish_checkpoint_command,omitempty"`
	PublishCheckpointGate     string `json:"publish_checkpoint_gate,omitempty"`
	PublishCheckpointPRNumber string `json:"publish_checkpoint_pr_number,omitempty"`
	PublishCheckpointEvidence string `json:"publish_checkpoint_evidence,omitempty"`
```

Add to `validateRebaseReceipt`, after the half-set gate-pair check and before the `CreatedUTC` parse:

```go
	cpFields := []string{
		r.PublishCheckpointHead, r.PublishCheckpointBaseHead, r.PublishCheckpointCommand,
		r.PublishCheckpointGate, r.PublishCheckpointPRNumber, r.PublishCheckpointEvidence,
	}
	set := 0
	for _, f := range cpFields {
		if f != "" {
			set++
		}
	}
	if set != 0 && set != len(cpFields) {
		return fmt.Errorf("partially-set publish checkpoint: all checkpoint fields must be empty or all set")
	}
	if set != 0 {
		if !validObjectID(gitcli.ObjectID(r.PublishCheckpointHead)) {
			return fmt.Errorf("invalid publish checkpoint head")
		}
		if !validObjectID(gitcli.ObjectID(r.PublishCheckpointBaseHead)) {
			return fmt.Errorf("invalid publish checkpoint base head")
		}
		if n, err := strconv.Atoi(r.PublishCheckpointPRNumber); err != nil || n <= 0 {
			return fmt.Errorf("invalid publish checkpoint pr number")
		}
		if r.GateDriveID != "" {
			return fmt.Errorf("publish checkpoint cannot coexist with a live gate continuation")
		}
	}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/workspace/ -run 'TestRebaseReceipt' -count=1`
Expected: PASS (the new test AND the existing round-trip/gate-pair tests — the new validation must not redden them).

- [ ] **Step 5: Mutation-test the validation**

Temporarily delete the `if r.GateDriveID != ""` clause; run `go test ./internal/workspace/ -run TestRebaseReceiptPublishCheckpoint -count=1`; expect FAIL on `live-drive-coexists`. Restore. Temporarily change `set != len(cpFields)` to `set > len(cpFields)`; rerun; expect FAIL on `head-only`. Restore, rerun, expect PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/workspace/rebasereceipt.go internal/workspace/rebasereceipt_test.go
git commit -m "feat(0408): rebase receipt carries a completed-gate publish checkpoint"
```

---

### Task 2: Pure checkpoint policy — `publishCheckpoint`, `checkpointDecision`, `resolvedFinalizeGateConfig`

**Files:**
- Modify: `internal/app/finalize_rebase.go`
- Test: `internal/app/finalize_rebase_test.go`

**Interfaces:**
- Consumes: Task 1's `PublishCheckpoint*` receipt fields; existing `resolvedFinalizeCommand`.
- Produces:
  - `type publishCheckpoint struct { Head, BaseHead, Command, Gate, PRNumber, Evidence string }`
  - `func publishCheckpointOf(rec workspace.RebaseReceipt) (publishCheckpoint, bool)` — present only when all six fields are non-empty.
  - `func checkpointDecision(cp publishCheckpoint, currentHead, liveBaseHead, resolvedCommand, resolvedGate string, prNumber int, evidenceVerified bool) bool` — pure reuse policy.
  - `func resolvedFinalizeGateConfig(ctx context.Context, deps FinalizeDeps, repoDir string) (command, gate string)` — replaces `resolvedFinalizeCommand` (which is deleted; its one call site in `composeLocalGate` updated).

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/finalize_rebase_test.go`:

```go
// TestCheckpointDecision pins the pure publish-checkpoint reuse policy: reuse
// requires verified evidence AND full equality on tested head, base head,
// resolved command, gate policy, and PR number — every conjunct non-vacuous
// (an empty resolved command or gate never matches; "never rerun a green gate"
// governs valid evidence, never stale evidence).
func TestCheckpointDecision(t *testing.T) {
	head := strings.Repeat("ab", 20)
	base := strings.Repeat("cd", 20)
	good := publishCheckpoint{
		Head: head, BaseHead: base, Command: "go test ./...",
		Gate: "local", PRNumber: "7", Evidence: "block",
	}
	cases := []struct {
		name        string
		cp          publishCheckpoint
		currentHead string
		liveBase    string
		resolvedCmd string
		resolvedGt  string
		prNumber    int
		verified    bool
		want        bool
	}{
		{"all-match-reuses", good, head, base, "go test ./...", "local", 7, true, true},
		{"unverified-evidence-runs", good, head, base, "go test ./...", "local", 7, false, false},
		{"moved-head-runs", good, strings.Repeat("ef", 20), base, "go test ./...", "local", 7, true, false},
		{"moved-base-runs", good, head, strings.Repeat("ef", 20), "go test ./...", "local", 7, true, false},
		{"changed-command-runs", good, head, base, "make check", "local", 7, true, false},
		{"empty-resolved-command-runs", publishCheckpoint{Head: head, BaseHead: base, Command: "", Gate: "local", PRNumber: "7", Evidence: "block"}, head, base, "", "local", 7, true, false},
		{"changed-gate-policy-runs", good, head, base, "go test ./...", "off", 7, true, false},
		{"different-pr-runs", good, head, base, "go test ./...", "local", 8, true, false},
		{"zero-pr-runs", good, head, base, "go test ./...", "local", 0, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkpointDecision(tc.cp, tc.currentHead, tc.liveBase, tc.resolvedCmd, tc.resolvedGt, tc.prNumber, tc.verified)
			if got != tc.want {
				t.Fatalf("checkpointDecision = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPublishCheckpointOf proves the presence probe: a fully-set checkpoint is
// extracted; any empty member reads as absent (never a partial checkpoint).
func TestPublishCheckpointOf(t *testing.T) {
	full := workspace.RebaseReceipt{
		PublishCheckpointHead:     strings.Repeat("ab", 20),
		PublishCheckpointBaseHead: strings.Repeat("cd", 20),
		PublishCheckpointCommand:  "go test ./...",
		PublishCheckpointGate:     "local",
		PublishCheckpointPRNumber: "7",
		PublishCheckpointEvidence: "block",
	}
	if cp, ok := publishCheckpointOf(full); !ok || cp.Head != full.PublishCheckpointHead || cp.Evidence != "block" {
		t.Fatalf("publishCheckpointOf(full) = (%+v, %v), want present with the receipt's fields", cp, ok)
	}
	missing := full
	missing.PublishCheckpointEvidence = ""
	if _, ok := publishCheckpointOf(missing); ok {
		t.Fatalf("publishCheckpointOf with an empty member reported present; want absent")
	}
	if _, ok := publishCheckpointOf(workspace.RebaseReceipt{}); ok {
		t.Fatalf("publishCheckpointOf(zero) reported present; want absent")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestCheckpointDecision|TestPublishCheckpointOf' -count=1`
Expected: compile FAIL — `undefined: checkpointDecision` / `publishCheckpoint`.

- [ ] **Step 3: Implement the pure helpers**

In `internal/app/finalize_rebase.go`, add below `gateDecision`:

```go
// publishCheckpoint is the in-process view of a receipt's completed-gate publish
// checkpoint (change 0408). It exists only when every member is non-empty — the
// receipt validation enforces the same all-or-none rule at the persistence
// boundary, so a partial value can never reach checkpointDecision.
type publishCheckpoint struct {
	Head, BaseHead, Command, Gate, PRNumber, Evidence string
}

// publishCheckpointOf extracts the receipt's publish checkpoint, reporting
// presence only when all six fields are set.
func publishCheckpointOf(rec workspace.RebaseReceipt) (publishCheckpoint, bool) {
	cp := publishCheckpoint{
		Head:     rec.PublishCheckpointHead,
		BaseHead: rec.PublishCheckpointBaseHead,
		Command:  rec.PublishCheckpointCommand,
		Gate:     rec.PublishCheckpointGate,
		PRNumber: rec.PublishCheckpointPRNumber,
		Evidence: rec.PublishCheckpointEvidence,
	}
	if cp.Head == "" || cp.BaseHead == "" || cp.Command == "" || cp.Gate == "" || cp.PRNumber == "" || cp.Evidence == "" {
		return publishCheckpoint{}, false
	}
	return cp, true
}

// checkpointDecision decides whether a completed-gate publish checkpoint may be
// reused on a finalize resume, skipping the suite. Reuse requires re-verified
// green evidence AND full equality on every recorded identity: the tested head
// is the current local head, the recorded base head is the live effective base
// head, the recorded command is byte-equal to the currently resolved
// finalize.test_command, the gate policy is unchanged, and the open PR is the
// recorded one. Empty-vs-empty never matches (a vacuous conjunct must not
// skip); any mismatch means the gate re-runs — "never rerun a green gate"
// governs valid evidence, never stale evidence. Head comparisons are
// full-length lowercase equality; the caller normalizes.
func checkpointDecision(cp publishCheckpoint, currentHead, liveBaseHead, resolvedCommand, resolvedGate string, prNumber int, evidenceVerified bool) bool {
	return evidenceVerified &&
		currentHead != "" && cp.Head == currentHead &&
		liveBaseHead != "" && cp.BaseHead == liveBaseHead &&
		resolvedCommand != "" && cp.Command == resolvedCommand &&
		resolvedGate != "" && cp.Gate == resolvedGate &&
		prNumber > 0 && cp.PRNumber == strconv.Itoa(prNumber)
}
```

Replace `resolvedFinalizeCommand` (delete it) with:

```go
// resolvedFinalizeGateConfig re-reads the authoritative finalize gate
// configuration the same way processFinalizeGate.buildDriveService pins it
// (deps.Planning.Reader PinContext → pin.Config.Effective.Finalize), returning
// the resolved finalize.test_command and finalize.gate policy. Command
// selection is an explicit domain boundary; no caller substitutes a command
// around authoritative configuration. A resolution failure yields ("", "") —
// gateDecision and checkpointDecision then never skip (their non-empty
// conjuncts fail), so the suite runs, fail-closed.
func resolvedFinalizeGateConfig(ctx context.Context, deps FinalizeDeps, repoDir string) (command, gate string) {
	pin, err := deps.Planning.Reader.PinContext(ctx, repoDir)
	if err != nil {
		return "", ""
	}
	return pin.Config.Effective.Finalize.TestCommand.Value, pin.Config.Effective.Finalize.Gate.Value
}
```

In `composeLocalGate`, replace

```go
	resolvedCommand := resolvedFinalizeCommand(ctx, deps, repoDir)
```

with

```go
	resolvedCommand, resolvedGatePolicy := resolvedFinalizeGateConfig(ctx, deps, repoDir)
	_ = resolvedGatePolicy // consumed by the PASSED-terminal checkpoint recording (next task)
```

(The blank assignment is temporary scaffolding removed in Task 3 when the recording lands.)

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestCheckpointDecision|TestPublishCheckpointOf|TestGateDecision' -count=1`
Expected: PASS (including the pre-existing `TestGateDecision*` tables).

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_rebase.go internal/app/finalize_rebase_test.go
git commit -m "feat(0408): pure publish-checkpoint reuse policy and resolved gate config"
```

---

### Task 3: Record the checkpoint at the PASSED terminal for a real rebase

**Files:**
- Modify: `internal/app/finalize_rebase.go` (`composeLocalGate`, `clearGateContinuation`, new `recordGatePassedReceipt`)
- Test: `internal/app/finalize_rebase_test.go` (the `headEvidenceGate` fake), `internal/app/finalize_rebase_integration_test.go`

**Interfaces:**
- Consumes: Task 1's receipt fields; Task 2's `resolvedFinalizeGateConfig`.
- Produces: `recordGatePassedReceipt(ctx context.Context, deps FinalizeDeps, rc *rebaseContext, rec workspace.RebaseReceipt, noop bool, pr githubcli.PullRequest, currentHead, resolvedCommand, gatePolicy, evidenceBlock string, res *FinalizeRebaseResult)` — the single PASSED-terminal receipt write (clears the pair, records or clears the checkpoint). Widened `clearGateContinuation` that also clears checkpoint fields on non-PASSED terminals. Test fake `headEvidenceGate` (mints evidence for the exact requested head) used again in Tasks 4–5.

- [ ] **Step 1: Write the failing tests**

Append the fake to `internal/app/finalize_rebase_test.go` (beside `fakeGate`; `time` and `evidence` are already imported):

```go
// headEvidenceGate is a passing FinalizeGate that mints green evidence
// certifying the EXACT head each request names — the way the production seam
// does (EvidenceRecord is handed req.Head) — so a recorded publish checkpoint
// verifies against the rebased head. It counts calls like fakeGate so skip
// paths can assert the suite never ran.
type headEvidenceGate struct {
	t     *testing.T
	calls int
}

func (g *headEvidenceGate) RunLocalGate(_ context.Context, req LocalGateRequest) (LocalGateResult, error) {
	g.calls++
	rec, err := evidence.NewRecord("go test ./...", strings.ToLower(req.Head), time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	if err != nil {
		g.t.Fatalf("evidence.NewRecord: %v", err)
	}
	return LocalGateResult{Outcome: FinalizeGatePassed, Evidence: evidence.Render(rec), RunDir: "/run/x"}, nil
}
```

Append to `internal/app/finalize_rebase_integration_test.go` (inside the `//go:build integration` file; `evidence` may need adding to its imports):

```go
// TestIntegrationFinalizeRebasePassedRecordsPublishCheckpoint proves a PASSED
// local gate for a REAL rewrite records the completed-gate publish checkpoint
// in the owned receipt — tested head, base head, resolved command, gate
// policy, PR number, and evidence that verifies green for the rebased head —
// with the gate-continuation pair clear; and that a PASSED gate for a NO-OP
// rebase records no checkpoint.
func TestIntegrationFinalizeRebasePassedRecordsPublishCheckpoint(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]

	t.Run("real-rewrite-records", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &headEvidenceGate{t: t}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Result != ResultApplied || res.Disposition != RebaseDispRebased {
			t.Fatalf("rebase = %q disp %q (reason %q msg %q), want applied/rebased", res.Result, res.Disposition, res.Reason, res.Message)
		}
		rewritten := f.localHead()
		rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		if err != nil || !present {
			t.Fatalf("receipt after PASSED gate: present=%v err=%v", present, err)
		}
		if rec.PublishCheckpointHead != rewritten {
			t.Errorf("checkpoint head = %q, want the rewritten head %q", rec.PublishCheckpointHead, rewritten)
		}
		if rec.PublishCheckpointBaseHead != rec.BaseHead {
			t.Errorf("checkpoint base head = %q, want the receipt base head %q", rec.PublishCheckpointBaseHead, rec.BaseHead)
		}
		if rec.PublishCheckpointCommand != "go test ./..." {
			t.Errorf("checkpoint command = %q, want the resolved finalize.test_command", rec.PublishCheckpointCommand)
		}
		if rec.PublishCheckpointGate != "local" {
			t.Errorf("checkpoint gate policy = %q, want %q", rec.PublishCheckpointGate, "local")
		}
		if rec.PublishCheckpointPRNumber != "1" {
			t.Errorf("checkpoint pr number = %q, want %q", rec.PublishCheckpointPRNumber, "1")
		}
		if v := evidence.Verify([]byte(rec.PublishCheckpointEvidence), rewritten); v != evidence.VerdictVerified {
			t.Errorf("checkpoint evidence verdict for the rewritten head = %q, want verified", v)
		}
		if rec.GateDriveID != "" || rec.GateOwnerGeneration != "" {
			t.Errorf("gate pair not cleared at the PASSED terminal: (%q, %q)", rec.GateDriveID, rec.GateOwnerGeneration)
		}
	})

	t.Run("noop-rebase-records-nothing", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		// No advanceBase: the feature already sits on the base; the gate still runs
		// (no PR evidence waives it) but the pass is for a no-op rebase.
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &headEvidenceGate{t: t}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Disposition != RebaseDispUnchanged {
			t.Fatalf("disp = %q (reason %q), want unchanged", res.Disposition, res.Reason)
		}
		rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		if err != nil || !present {
			t.Fatalf("receipt: present=%v err=%v", present, err)
		}
		if rec.PublishCheckpointHead != "" || rec.PublishCheckpointEvidence != "" {
			t.Errorf("a no-op rebase recorded a publish checkpoint: %+v", rec)
		}
	})
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -tags integration -run TestIntegrationFinalizeRebasePassedRecordsPublishCheckpoint -count=1`
Expected: FAIL — `checkpoint head = "", want the rewritten head …` (the fake compiles; recording does not exist yet).

- [ ] **Step 3: Implement the recording**

In `internal/app/finalize_rebase.go`:

In `composeLocalGate`, drop the Task 2 scaffolding line (`_ = resolvedGatePolicy`) and change the PASSED case from

```go
	case FinalizeGatePassed:
		base.Gate.Evidence = gres.Evidence
		out := newRebaseResult(op, ResultApplied, base)
		clearGateContinuation(ctx, deps, rc, rec, &out)
		return out
```

to

```go
	case FinalizeGatePassed:
		base.Gate.Evidence = gres.Evidence
		out := newRebaseResult(op, ResultApplied, base)
		recordGatePassedReceipt(ctx, deps, rc, rec, noop, pr, currentHead, resolvedCommand, resolvedGatePolicy, gres.Evidence, &out)
		return out
```

Add beside `clearGateContinuation`:

```go
// recordGatePassedReceipt persists the PASSED terminal into the owned receipt
// in ONE atomic write: the live gate-continuation pair is cleared (the drive is
// terminal), and — for a real rewrite whose PR identity is known and whose gate
// configuration resolved — the completed-gate publish checkpoint is recorded so
// a denied publish can resume without re-running the suite (change 0408). A
// no-op rebase records no checkpoint (its skip waiver is the PR-body evidence
// gateDecision already honors). Best-effort like clearGateContinuation: the
// gate outcome is already mapped, so a write failure is appended to the result
// message and never changes the disposition — the resume simply re-runs the
// gate, fail-closed.
func recordGatePassedReceipt(ctx context.Context, deps FinalizeDeps, rc *rebaseContext, rec workspace.RebaseReceipt, noop bool, pr githubcli.PullRequest, currentHead, resolvedCommand, gatePolicy, evidenceBlock string, res *FinalizeRebaseResult) {
	updated := rec
	updated.GateDriveID, updated.GateOwnerGeneration = "", ""
	updated.PublishCheckpointHead, updated.PublishCheckpointBaseHead = "", ""
	updated.PublishCheckpointCommand, updated.PublishCheckpointGate = "", ""
	updated.PublishCheckpointPRNumber, updated.PublishCheckpointEvidence = "", ""
	if !noop && pr.Number > 0 && resolvedCommand != "" && gatePolicy != "" && evidenceBlock != "" {
		updated.PublishCheckpointHead = currentHead
		updated.PublishCheckpointBaseHead = rec.BaseHead
		updated.PublishCheckpointCommand = resolvedCommand
		updated.PublishCheckpointGate = gatePolicy
		updated.PublishCheckpointPRNumber = strconv.Itoa(pr.Number)
		updated.PublishCheckpointEvidence = evidenceBlock
	}
	if updated == rec {
		return
	}
	if err := deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, updated); err != nil {
		res.Message = strings.TrimSpace(res.Message +
			" (persisting the gate terminal to the rebase receipt failed: " + err.Error() + ")")
	}
}
```

Widen `clearGateContinuation` (used by the FAILED/HALTED/seam-error terminals) so every transition out of the checkpoint state removes it — presence-encoded state discipline. Replace its early-return and clearing body:

```go
func clearGateContinuation(ctx context.Context, deps FinalizeDeps, rc *rebaseContext, rec workspace.RebaseReceipt, res *FinalizeRebaseResult) {
	updated := rec
	updated.GateDriveID, updated.GateOwnerGeneration = "", ""
	updated.PublishCheckpointHead, updated.PublishCheckpointBaseHead = "", ""
	updated.PublishCheckpointCommand, updated.PublishCheckpointGate = "", ""
	updated.PublishCheckpointPRNumber, updated.PublishCheckpointEvidence = "", ""
	if updated == rec {
		return
	}
	if err := deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, updated); err != nil {
		res.Message = strings.TrimSpace(res.Message +
			" (clearing the gate continuation from the rebase receipt failed: " + err.Error() + ")")
	}
}
```

Update `clearGateContinuation`'s doc comment first line to say it clears "the gate pair AND any publish checkpoint" at a non-passed terminal (keep the existing rationale sentences about the driver's Advance and best-effort semantics).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ -tags integration -run 'TestIntegrationFinalizeRebase' -count=1`
Expected: PASS — the new test and every pre-existing `TestIntegrationFinalizeRebase*` test (the happy-path, waiting, response-loss-recovery, foreign-state, and precondition tests must stay green; the response-loss test's replay now hits the checkpoint with stale fixture evidence, which invalidates in Task 4 — until Task 4 lands, the replay path has no reuse branch, so behavior is unchanged).

Also run: `go test ./internal/app/ ./internal/workspace/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_rebase.go internal/app/finalize_rebase_test.go internal/app/finalize_rebase_integration_test.go
git commit -m "feat(0408): PASSED gate terminal records the publish checkpoint in the receipt"
```

---

### Task 4: Reuse or invalidate the checkpoint on resume

**Files:**
- Modify: `internal/app/finalize_rebase.go` (`recoverFromReceipt`)
- Test: `internal/app/finalize_rebase_integration_test.go`

**Interfaces:**
- Consumes: Task 2's `publishCheckpointOf`/`checkpointDecision`/`resolvedFinalizeGateConfig`; Task 3's recorded checkpoints; `evidence.Verify` / `evidence.VerdictVerified`; the existing `gateComposeSkipped`, `GateReport`, `ReasonRebaseReceiptWrite`.
- Produces: the resume behavior later tasks and the finalize skill rely on — a valid checkpoint returns `applied`/`rebased` with `Gate{Compose: "skipped", Permit: <head>, Evidence: <recorded block>}` and no suite invocation; a stale checkpoint is cleared (in memory and on disk) and the gate re-runs.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/finalize_rebase_integration_test.go`:

```go
// setupPassedRebaseCheckpoint drives a real rewrite to a PASSED gate whose
// publish checkpoint is recorded (the state a denied publish leaves behind),
// returning the fixture, the gate fake (for call counting), the deps, and the
// rewritten head.
func setupPassedRebaseCheckpoint(t *testing.T) (*rebaseFixture, *headEvidenceGate, FinalizeDeps, string) {
	t.Helper()
	f := setupRebaseFixture(t, planRepoModes()[0])
	f.advanceBase(t)
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &headEvidenceGate{t: t}
	deps := f.finalizeDeps(gh, gate)
	res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Disposition != RebaseDispRebased || gate.calls != 1 {
		t.Fatalf("setup rebase = disp %q gate calls %d (reason %q), want rebased with one gate run", res.Disposition, gate.calls, res.Reason)
	}
	rewritten := f.localHead()
	rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil || !present || rec.PublishCheckpointHead != rewritten {
		t.Fatalf("setup did not record a checkpoint for the rewritten head: present=%v err=%v cp=%q", present, err, rec.PublishCheckpointHead)
	}
	return f, gate, deps, rewritten
}

// tamperCheckpoint rewrites the on-disk receipt with one checkpoint field
// mutated — a still-VALID receipt whose recorded identity no longer matches
// current reality — so a resume must invalidate it and re-run the gate.
func tamperCheckpoint(t *testing.T, f *rebaseFixture, mut func(*workspace.RebaseReceipt)) {
	t.Helper()
	rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil || !present {
		t.Fatalf("reading receipt to tamper: present=%v err=%v", present, err)
	}
	mut(&rec)
	if err := f.svc.WriteRebaseReceipt(context.Background(), f.metaDir, rec); err != nil {
		t.Fatalf("writing tampered receipt: %v", err)
	}
}

// TestIntegrationFinalizeRebaseCheckpointReuse proves the marquee behavior: a
// resume after a denied publish (completed rewrite, checkpoint recorded, remote
// and PR untouched) reuses the recorded evidence and publishes-readies WITHOUT
// invoking the suite — the gate report is skipped, carries the recorded
// evidence verifying the rewritten head, and the gate seam is never called a
// second time.
func TestIntegrationFinalizeRebaseCheckpointReuse(t *testing.T) {
	requireRealGit(t)
	f, gate, deps, rewritten := setupPassedRebaseCheckpoint(t)

	res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})

	if res.Result != ResultApplied || res.Disposition != RebaseDispRebased {
		t.Fatalf("resume = %q disp %q (reason %q msg %q), want applied/rebased", res.Result, res.Disposition, res.Reason, res.Message)
	}
	if gate.calls != 1 {
		t.Fatalf("gate calls = %d, want 1 — a valid checkpoint must never re-run the suite", gate.calls)
	}
	if res.Gate == nil || res.Gate.Compose != gateComposeSkipped {
		t.Fatalf("gate report = %+v, want compose skipped", res.Gate)
	}
	if res.Gate.Permit != rewritten {
		t.Errorf("skip permit = %q, want the rewritten head %q", res.Gate.Permit, rewritten)
	}
	if v := evidence.Verify([]byte(res.Gate.Evidence), rewritten); v != evidence.VerdictVerified {
		t.Errorf("reused evidence verdict = %q, want verified for the rewritten head", v)
	}
	if res.Head != rewritten || res.Attempt == "" {
		t.Errorf("resume head/attempt = %q/%q, want the rewritten head and the owned attempt", res.Head, res.Attempt)
	}
}

// TestIntegrationFinalizeRebaseCheckpointInvalidation proves every recorded
// identity is load-bearing: a moved local head, a changed recorded command, a
// changed gate policy, a different PR, and evidence for the wrong head each
// invalidate the checkpoint — the gate re-runs and the receipt's checkpoint is
// rewritten by the new terminal, never reused stale. A moved BASE keeps its
// existing refusal ahead of any reuse.
func TestIntegrationFinalizeRebaseCheckpointInvalidation(t *testing.T) {
	requireRealGit(t)

	t.Run("moved-local-head-reruns", func(t *testing.T) {
		f, gate, deps, _ := setupPassedRebaseCheckpoint(t)
		// The head moves past the checkpoint (still descending the base).
		writeRepoFile(t, f.wp, "more.txt", "more work\n")
		runGit(t, f.wp, "add", "-A")
		runGit(t, f.wp, "commit", "-q", "-m", "extra work after the pass")
		moved := f.localHead()
		res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Disposition != RebaseDispRebased || gate.calls != 2 {
			t.Fatalf("resume = disp %q gate calls %d, want rebased with the gate re-run", res.Disposition, gate.calls)
		}
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		if rec.PublishCheckpointHead != moved {
			t.Errorf("checkpoint after re-run = %q, want re-recorded for the moved head %q", rec.PublishCheckpointHead, moved)
		}
	})

	tamperCases := map[string]func(*workspace.RebaseReceipt){
		"changed-command":  func(r *workspace.RebaseReceipt) { r.PublishCheckpointCommand = "make other-suite" },
		"changed-gate":     func(r *workspace.RebaseReceipt) { r.PublishCheckpointGate = "off" },
		"different-pr":     func(r *workspace.RebaseReceipt) { r.PublishCheckpointPRNumber = "99" },
		"wrong-head-evidence": func(r *workspace.RebaseReceipt) {
			r.PublishCheckpointEvidence = strings.ReplaceAll(
				r.PublishCheckpointEvidence, r.PublishCheckpointHead, r.OrigHead)
		},
	}
	for name, mut := range tamperCases {
		mut := mut
		t.Run(name+"-reruns", func(t *testing.T) {
			f, gate, deps, rewritten := setupPassedRebaseCheckpoint(t)
			tamperCheckpoint(t, f, mut)
			res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
				FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
			if res.Disposition != RebaseDispRebased || gate.calls != 2 {
				t.Fatalf("resume = disp %q gate calls %d (reason %q), want rebased with the gate re-run", res.Disposition, gate.calls, res.Reason)
			}
			rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
			if rec.PublishCheckpointHead != rewritten || rec.PublishCheckpointCommand != "go test ./..." {
				t.Errorf("stale checkpoint not replaced by the re-run terminal: %+v", rec)
			}
		})
	}

	t.Run("moved-base-still-blocks-ahead-of-reuse", func(t *testing.T) {
		f, gate, deps, _ := setupPassedRebaseCheckpoint(t)
		f.advanceBase(t)
		res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebaseMovedBase)
		if gate.calls != 1 {
			t.Errorf("gate calls = %d, want 1 — a moved base neither reuses nor re-runs", gate.calls)
		}
	})
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -tags integration -run 'TestIntegrationFinalizeRebaseCheckpoint' -count=1`
Expected: `TestIntegrationFinalizeRebaseCheckpointReuse` FAILs with `gate calls = 2, want 1` (no reuse branch yet); the invalidation subtests may pass trivially (the gate always re-runs today) except assertions on checkpoint rewriting, which Task 3 already satisfies — the reuse test is the red driver.

- [ ] **Step 3: Implement reuse and invalidation in `recoverFromReceipt`**

In `internal/app/finalize_rebase.go`, in `recoverFromReceipt`, replace

```go
	noop := string(localHead) == rec.OrigHead
	// The receipt's pair may be set (a prior WAITING to resume) or empty (a crash
	// before WAITING, or a cleared terminal); composeLocalGate derives the
	// continuation from rec, so both are correct as-is.
	return composeLocalGate(ctx, deps, repoDir, op, rc, pr, rec, localHead, noop)
```

with

```go
	noop := string(localHead) == rec.OrigHead
	// A completed-gate publish checkpoint (change 0408): when the recorded
	// identities all still match current reality — tested head, base head,
	// resolved command, gate policy, PR — and the recorded evidence re-verifies
	// green for this exact head, the suite is NOT re-run; the recorded evidence
	// is returned so the caller proceeds straight to publish. Any mismatch
	// invalidates: the checkpoint is cleared (on disk AND in the copy handed to
	// composeLocalGate, so no later receipt write resurrects stale evidence)
	// and the gate re-runs exactly as today. A live continuation pair never
	// coexists with a checkpoint (receipt validation), so reuse never strands a
	// running drive.
	if !noop && rec.GateDriveID == "" {
		if cp, ok := publishCheckpointOf(rec); ok {
			currentHead := strings.ToLower(string(localHead))
			resolvedCommand, resolvedGatePolicy := resolvedFinalizeGateConfig(ctx, deps, repoDir)
			verified := evidence.Verify([]byte(cp.Evidence), currentHead) == evidence.VerdictVerified
			if checkpointDecision(cp, currentHead, string(baseHead), resolvedCommand, resolvedGatePolicy, pr.Number, verified) {
				return newRebaseResult(op, ResultApplied, FinalizeRebaseResult{
					ID: id, Disposition: RebaseDispRebased, Head: string(localHead), OrigHead: rec.OrigHead,
					Base: rc.base.Branch, BaseHead: rec.BaseHead, Attempt: rec.Attempt,
					Gate: &GateReport{Compose: gateComposeSkipped, Permit: currentHead, Evidence: cp.Evidence},
				})
			}
			// Stale checkpoint: reuse applies only to still-valid evidence. Clear it
			// before the gate re-runs; a clear that cannot be persisted is a blocked
			// receipt write, not a silent proceed over stale durable state.
			rec.PublishCheckpointHead, rec.PublishCheckpointBaseHead = "", ""
			rec.PublishCheckpointCommand, rec.PublishCheckpointGate = "", ""
			rec.PublishCheckpointPRNumber, rec.PublishCheckpointEvidence = "", ""
			if err := deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, rec); err != nil {
				return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseReceiptWrite, err.Error(), id)
			}
		}
	}
	// The receipt's pair may be set (a prior WAITING to resume) or empty (a crash
	// before WAITING, or a cleared terminal); composeLocalGate derives the
	// continuation from rec, so both are correct as-is.
	return composeLocalGate(ctx, deps, repoDir, op, rc, pr, rec, localHead, noop)
```

Note `evidence` is already imported in `finalize_rebase.go`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ -tags integration -run 'TestIntegrationFinalizeRebase' -count=1`
Expected: PASS — the checkpoint tests AND every pre-existing rebase integration test. Watch `TestIntegrationFinalizeRebaseResponseLossRecovery` specifically: its fixture gate mints evidence for the ORIG head, so the replay's checkpoint fails re-verification, invalidates, and re-runs — the test's replay assertions (same attempt, same rewritten head, applied/rebased) must stay green.

- [ ] **Step 5: Mutation-test the no-suite guard**

Temporarily disable the reuse branch by changing `if checkpointDecision(` to `if false && checkpointDecision(`; run `go test ./internal/app/ -tags integration -run TestIntegrationFinalizeRebaseCheckpointReuse -count=1`; expect FAIL on `gate calls = 2, want 1`. Restore. Then mutate one conjunct wiring: in the reuse call, replace `pr.Number` with `7`; run `go test ./internal/app/ -tags integration -run 'TestIntegrationFinalizeRebaseCheckpointInvalidation/different-pr-reruns' -count=1`; expect FAIL (checkpoint for PR 99 vs live PR 1 — wait, the tamper sets the RECORDED number to 99 while the live PR is 1, so hardcoding the live side to 7 makes the mismatch hold both ways; instead mutate by replacing `verified` with `true` and run the `wrong-head-evidence-reruns` subtest, expecting FAIL on `gate calls`). Restore all mutations, rerun the full checkpoint set with `-count=1`, expect PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finalize_rebase.go internal/app/finalize_rebase_integration_test.go
git commit -m "feat(0408): finalize resume reuses a still-valid publish checkpoint, never a stale one"
```

---

### Task 5: Publish-path compatibility — checkpoint-bearing receipts through PublishRewrite and FinalizePublish

**Files:**
- Test: `internal/workspace/rewrite_test.go`
- Test: `internal/app/finalize_publish_test.go`

No production files change in this task. It proves the untouched publish path accepts the widened receipt: `PublishRewrite`'s `onDisk != req.Receipt` equality gate and its lease/noop/contended/response-loss behavior, and `FinalizePublish`'s PR-body preservation, all hold with checkpoint fields present.

**Interfaces:**
- Consumes: Task 1's fields; `receiptFor` and the workspace test helpers in `rewrite_test.go`; `setupPublishFixture`, `recFor`, `authoredPRBody`, `openPRForPublish`, `fakePublishGitHub` in `finalize_publish_test.go`.
- Produces: nothing new — regression pins only.

- [ ] **Step 1: Write the workspace twin test**

Append to `internal/workspace/rewrite_test.go` (model: `TestPublishRewriteLeaseWithGatePair`):

```go
// TestPublishRewriteLeaseWithPublishCheckpoint proves publish still authorizes
// a rewrite from a receipt carrying the completed-gate publish checkpoint
// (change 0408): the on-disk receipt and the caller's expected receipt are the
// same value, checkpoint included, so the equality gate holds — and the exact
// lease/reprobe behavior is unchanged.
func TestPublishRewriteLeaseWithPublishCheckpoint(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	base := gitcli.ObjectID(gitOut(t, ws, "rev-parse", "HEAD"))
	head1 := commitInWorkspace(t, ws, "feature.txt", "feature work\n")

	if res, err := publishHead(t, svc, repo, tgt); err != nil || res.Disposition != PublishPublished {
		t.Fatalf("seed publish = %q err=%v; want published", res.Disposition, err)
	}
	newHead := rewriteWorkspaceHead(t, ws)
	if newHead == head1 {
		t.Fatalf("fixture: rewrite did not change the head")
	}

	dir := metaDirOf(repo, tgt)
	rec := receiptFor(repo, tgt, head1, base, "attempt-01")
	rec.PublishCheckpointHead = string(newHead)
	rec.PublishCheckpointBaseHead = string(base)
	rec.PublishCheckpointCommand = "go test ./..."
	rec.PublishCheckpointGate = "local"
	rec.PublishCheckpointPRNumber = "7"
	rec.PublishCheckpointEvidence = "result: green\n"
	if err := svc.WriteRebaseReceipt(context.Background(), dir, rec); err != nil {
		t.Fatalf("WriteRebaseReceipt: %v", err)
	}

	outcome, err := svc.PublishRewrite(context.Background(), RewriteRequest{Dir: dir, Receipt: rec, NewHead: string(newHead)})
	if err != nil {
		t.Fatalf("PublishRewrite: %v", err)
	}
	if outcome != RewritePublished {
		t.Errorf("outcome = %q; want published", outcome)
	}
	if got, ok := originFeatCommit(t, r); !ok || got != newHead {
		t.Errorf("origin feat ref = %q (ok=%v); want rewritten head %q", got, ok, newHead)
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/workspace/ -run TestPublishRewriteLeaseWithPublishCheckpoint -count=1`
Expected: PASS immediately (the equality gate compares whole values). If it fails, that is a Task 1 defect — stop and fix there.

- [ ] **Step 3: Pin the app-side publish fixture and the end-to-end resume**

Append to `internal/app/finalize_publish_test.go` (`evidence`, `strings`, `context`, `githubcli` already imported):

```go
// TestFinalizePublishAfterCheckpointResume proves the reuse path end to end:
// a completed rewrite whose PASSED gate recorded a publish checkpoint, a
// denied publish (nothing pushed, PR untouched), a finalize resume that reuses
// the checkpoint WITHOUT re-running the suite, and a FinalizePublish driven by
// the reused evidence that lands the rewritten head under the exact lease and
// converges the PR body while preserving every authored byte outside the
// managed evidence block.
func TestFinalizePublishAfterCheckpointResume(t *testing.T) {
	requireRealGit(t)
	f := setupRebaseFixture(t, planRepoModes()[0])
	f.advanceBase(t)
	rebaseGH := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &headEvidenceGate{t: t}
	deps := f.finalizeDeps(rebaseGH, gate)

	// The rebase completes, the gate passes, the checkpoint is recorded — and
	// then the publish is denied: no push happens, the remote and PR still hold
	// the original head.
	first := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if first.Disposition != RebaseDispRebased || gate.calls != 1 {
		t.Fatalf("setup rebase = disp %q calls %d, want rebased/1", first.Disposition, gate.calls)
	}
	rewritten := f.localHead()

	// The resume reuses the checkpoint: skipped compose, evidence returned, no
	// second suite run.
	resume := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if resume.Gate == nil || resume.Gate.Compose != gateComposeSkipped || gate.calls != 1 {
		t.Fatalf("resume gate = %+v calls %d, want skipped with no re-run", resume.Gate, gate.calls)
	}

	// The reused evidence drives finalize publish, exactly as the skill would.
	authored := "## Summary\n\nAuthored intro prose.\n\nAuthored outro prose.\n"
	pubGH := &fakePublishGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{{
		Number: 1, URL: "https://example.test/pr/1", State: githubcli.StateOpen,
		HeadBranch: "feat/" + f.slug, HeadCommit: rewritten, BaseBranch: "main",
		Title: "Add the widget", Body: authored, Version: "sha256:" + strings.Repeat("d", 64),
	}}}
	pres := FinalizePublish(context.Background(), FinalizeDeps{Planning: f.deps, GitHub: pubGH, Workspace: f.svc},
		f.repo.invocation, FinalizePublishRequest{
			ID: f.id, Attempt: resume.Attempt, Head: rewritten,
			EvidenceRecord: []byte(resume.Gate.Evidence),
		})
	if pres.Result != ResultApplied || pres.Disposition != PublishDispPublished {
		t.Fatalf("publish = %q disp %q (reason %q msg %q), want applied/published", pres.Result, pres.Disposition, pres.Reason, pres.Message)
	}
	if tip := runGit(t, f.repo.origin, "rev-parse", "refs/heads/feat/"+f.slug); tip != rewritten {
		t.Errorf("origin feature tip = %q, want the rewritten head %q", tip, rewritten)
	}
	// The authored bytes survived; the managed block certifies the rewritten head.
	body := pubGH.lastEnsuredBody()
	if !strings.Contains(body, "Authored intro prose.") || !strings.Contains(body, "Authored outro prose.") {
		t.Errorf("authored PR-body bytes were not preserved:\n%s", body)
	}
	got, err := evidence.Extract([]byte(body))
	if err != nil || got.Head != rewritten {
		t.Errorf("converged evidence head = %q err=%v, want %q", got.Head, err, rewritten)
	}
}
```

Then wire the one helper this test needs onto the existing `fakePublishGitHub` (read the fake's struct first; it records the `EnsurePullRequest` request — if it already stores the last ensured body under another name, use that name in the test instead of adding a method; only if nothing stores it, add):

```go
// lastEnsuredBody returns the body of the most recent EnsurePullRequest call.
func (f *fakePublishGitHub) lastEnsuredBody() string {
	return f.ensured.Body
}
```

adjusting `ensured` to the fake's actual field that captures the request (inspect `fakePublishGitHub.EnsurePullRequest` around the `internal/app/finalize_publish_test.go` fake definition; it sits at the top of the file). If the fake does not capture the request, add a `lastEnsure githubcli.EnsurePullRequestRequest` field assigned at the top of `EnsurePullRequest` and return `f.lastEnsure.Body`.

Also append a one-line pin inside `setupPublishFixture` (after `rewritten` is derived) so every existing publish test now demonstrably runs over a checkpoint-bearing receipt:

```go
	if rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir); err != nil || !present || (rec.PublishCheckpointHead == "" && rec.PublishCheckpointEvidence == "") {
		// The fixture's fakeGate mints evidence for the ORIG head, so the recorded
		// checkpoint will not re-verify on resume — but it IS recorded, which is
		// exactly what the downstream publish tests must tolerate.
		t.Fatalf("publish fixture receipt carries no checkpoint (present=%v err=%v); downstream tests would no longer cover checkpoint-bearing receipts", present, err)
	}
```

Note: the fixture's `fakeGate` returns `greenEvidenceFor(t, f.head)` — prose-wrapped evidence for the orig head. If `recordGatePassedReceipt` recorded it, the checkpoint head is the REWRITTEN head with orig-head evidence: still a valid receipt (validation checks shape, not cross-field evidence), recorded because the evidence block is non-empty. If instead you find the fixture records nothing (e.g. the evidence guard), relax the pin to assert whichever state is true and say why — the pin's job is that the assertion matches reality and reddens if the fixture drifts.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/app/ -run 'TestFinalizePublish' -count=1` and `go test ./internal/app/ -tags integration -count=1 ./internal/workspace/ -count=1`
Expected: PASS — all pre-existing publish tests (order, crash replay, unknown-stops, foreign-attempt, shape refusals, skipped-evidence) still green over checkpoint-bearing receipts, plus the new end-to-end test.

- [ ] **Step 5: Commit**

```bash
git add internal/workspace/rewrite_test.go internal/app/finalize_publish_test.go
git commit -m "test(0408): checkpoint-bearing receipts flow through the untouched publish path"
```

---

### Task 6: Record the new ADR

**Files:**
- Create (metadata branch, via the docket-adr skill): `docs/adrs/0112-completed-gate-publish-checkpoint-in-the-owned-rebase-receipt.md` (0112 expected — use the NEXT AVAILABLE number at execution time; ADRs currently run to 0111. Never overwrite ADR-0105.)

- [ ] **Step 1: Dispatch the registered `docket-adr` agent** (per AGENTS.md "Docket agents — dispatch, don't run inline") to record a NEW ADR for change 0408 with this decision content:

- **Title:** A completed-gate publish checkpoint is persisted in the owned rebase receipt
- **Status:** Accepted
- **Context:** A finalize that performs a real rebase runs the local gate on the rewritten head and records green evidence, then publishes with a force-with-lease push. When the publish is denied (most sharply by a host permission classifier, so the binary never runs), nothing publish-specific persisted: on resume, `recoverFromReceipt` sees the local head is no longer the receipt's `OrigHead`, so `composeLocalGate` re-runs the full suite for a head it already certified. ADR-0105 persists the gate's LIVE continuation (drive id + owner generation) so a WAITING drive resumes; it does not persist COMPLETED evidence, and its own consequence note named this residue.
- **Decision:** When the local gate reaches PASSED for a real rebase, the owned rebase receipt additionally records a completed-gate publish checkpoint: the tested head, the effective base head, the byte-exact resolved `finalize.test_command`, the gate policy, the open PR number, and the green evidence block — written in the same atomic receipt write that clears the continuation pair, all-or-none, mutually exclusive with a live pair, keeping the receipt `==`-comparable. On a resume with no rebase in progress and the head descending the base, a checkpoint whose every recorded identity still matches current reality (and whose evidence re-verifies green for the exact current head) skips the suite and returns the recorded evidence for publication; any mismatch clears the checkpoint and the gate re-runs. The identity and moved-base refusals stay ahead of any reuse. Host-denial detection stays a skill/harness concern — no Go primitive distinguishes a denial from a Go result, because a denied binary never ran.
- **Consequences:** A denied publish costs a resume, not a suite re-run; stale evidence can never publish (every conjunct is equality-checked and the evidence is re-verified); the receipt gains six optional scalar fields every existing equality gate (PublishRewrite's decide-and-act copy check, the write round-trip guard) covers automatically; a checkpoint that cannot be persisted degrades to today's behavior (the gate re-runs), fail-closed.
- **Relations:** extends ADR-0105 (live continuation → completed evidence); refines ADR-0098 (the receipt remains the owner-held authorization surface finalize's caller presents). Related change: 0408.

- [ ] **Step 2: Verify** the new ADR file exists on the metadata branch with the next free number, ADR-0105 is untouched, and the ADR index regenerated cleanly (the docket-adr skill owns index/regeneration mechanics — verify its report, then confirm with `git -C .docket log --oneline -3` / reading the new file).

(No feature-branch commit: the ADR lands on the metadata branch through the skill's own write path.)

---

### Task 7: Full-suite gate

- [ ] **Step 1: Run the whole suite from resolved configuration**

Resolve `build.test_command` from config (in this repo it resolves to `go run ./cmd/docket development test`) and run it from the feature worktree root. Never substitute a hand-rolled `go test` subset for this gate.

Expected: SUITE PASS. Treat any `SERIAL CONFIRMED OVER BUDGET:` line as an authoritative breach to act on; `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` lines are screening findings to note in the results, not failures.

- [ ] **Step 2: Fix anything red, re-run to green, then commit any fixes**

```bash
git add -u
git commit -m "fix(0408): suite-gate fixes"
```

(Skip the commit if the suite passed with no changes.)

---

## Self-Review (performed while writing)

- **Spec coverage:** Record (Task 3), Reuse (Task 4), Invalidate (Task 4 incl. moved-base-stays-blocked), receipt discipline mirroring ADR-0105 write-on-terminal/clear-on-transition (Tasks 1, 3, 4 — closeout clearing is the existing whole-receipt removal, unchanged), PublishRewrite/FinalizePublish preservation (Global Constraints + Task 5), suite-not-invoked assert with mutation test (Task 4), contention/response-loss and PR-body preservation (Task 5 — existing tests now run over checkpoint-bearing receipts, plus the end-to-end test), full suites from resolved config (Task 7), new ADR relating to 0105/0098 (Task 6), host-denial-stays-in-skill (out of code scope; restated in the ADR).
- **Type consistency:** `publishCheckpoint`/`publishCheckpointOf`/`checkpointDecision`/`recordGatePassedReceipt`/`resolvedFinalizeGateConfig` and the six `PublishCheckpoint*` field names are spelled identically across Tasks 1–5.
- **Known judgment call for the executor:** Task 5's fixture pin depends on whether `recordGatePassedReceipt`'s guard records the fixture's orig-head evidence (it does: the guard checks non-emptiness, not cross-verification). If reality differs when you get there, make the pin assert the true state — its purpose is drift detection, not a specific value.
