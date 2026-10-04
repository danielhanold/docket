<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0515 — Retire the finalize repair sign-off so a green repair merges](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0515-make-finalize-merge-honor-the-repair-sign-off-block-when-an.md)**
<!-- docket:backlink:end -->
# Retire the Finalize Repair Sign-off Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In this repository the plan is executed by `docket-build`.

**Goal:** A finalize repair that turns the rebased suite green publishes and merges like any other green change; `## Finalize blocked` becomes a visible note that never stops selection or merge, and the never-active binary code that pretended otherwise is deleted.

**Architecture:** Three binary edits remove dead "blocked note" machinery: the domain finalize selector loses its `blocked` map and `finalize-blocked` skip, `context.finalize` and the maintenance sweep stop passing it, and `finalize.merge` drops the marker term from `NotSuperseded`. The finalize skill, its `gate-failure.md` reference, the two gate agents, the convention skill, and the user docs are rewritten to state the new rule (green repair merges, recorded in the run report and `## Closeout notes`; approval is the repository's own policy). Prose sentinels pin the new rule and the absence of the retired token; the embedded bundle and harness goldens are regenerated from the edited sources.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), markdown skills/agents, `go generate ./internal/assets/` for the embedded bundle, harness golden `-update` flag, the Go-native suite runner (`go run ./cmd/docket development test`).

**Spec:** `docs/superpowers/specs/2026-10-04-make-finalize-merge-honor-the-repair-sign-off-block-when-an-design.md` (on the `docket` metadata branch; read-only copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-04-make-finalize-merge-honor-the-repair-sign-off-block-when-an-design.md`).

## Global Constraints

- Finalize adds no human stop of its own: a green repair merges on autonomous **and** attended runs. No `finalize.block`, no prompt, no `clear-block` for a repair.
- Add no new condition, skip reason, refusal, or halt anywhere (binary or prose). Only never-taken branches are deleted.
- A refused closeout-notes request is retried once without notes; a lost note never stops closeout.
- `finalize.block`, `finalize.clear-block`, `HasFinalizeBlocked`, the board's "finalize blocked — needs you" cell, and every other abort-and-report point are unchanged. Approval semantics (`require_pr_approval`, `ExplicitID`, `--admin`) are unchanged.
- The ADR (spec §4) and the `## Update` sections on ADR-0010 / ADR-0008 are recorded by the parent through `docket-adr` — no plan task creates or edits anything under `docs/adrs/`.
- Never edit frozen records: `docs/superpowers/plans/*` (other than this plan), `docs/superpowers/specs/*`, `docs/changes/**`, `docs/results/*`, `docs/adrs/*`, `testdata/**`, `internal/install/legacydata/**`, `internal/install/testdata/**`, `internal/repository/testdata/**`.
- Never hand-edit `internal/assets/embedded/tree/**` or `internal/harness/*/testdata/golden/**`: regenerate with `go generate ./internal/assets/` then `for p in claude codex cursor opencode; do go test ./internal/harness/$p/ -count=1 -update; done`.
- Docs state current behavior only — no change numbers or PR citations in `README.md`, `docs/guide/**`, `docs/concepts/**`, `docs/reference/**`, `docs/install/**`, `docs/comparison/**`, skills, or agents (ADR links are fine).
- Every `go test` run uses `-count=1` (the result cache can serve a mutated tree).
- Format Go with the toolchain gofmt: `"$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt" -w <files>`.
- Stage explicit paths only (never `git add -A` / `git add .`); one commit per task.
- Mutation probes restore from a backup copy (`cp f /tmp/...bak` then `cp -f` back), never `git checkout --` on uncommitted work; confirm each mutation landed with `grep -c` before reading the result.
- Shell: under `pipefail` never pipe into `grep -q`/`head`; capture into a variable first.

## Review Focus

1. **A legacy note written by an older binary** (`## Finalize blocked` naming `repair-needs-signoff`, in flight at upgrade) — the change must be selected as an ordinary actionable candidate and merge. Pinned by `TestContextFinalizeNotedChangeIsSelected` (Task 1) and the note-merge subtests (Task 2).
2. **An explicit `--id` on a noted change** — it must be actionable with **no** `override_note` (there is no `finalize-blocked` skip left to override). Pinned in Task 1's explicit-id subtest.
3. **A noted change whose PR already merged** — still `merged-recovery` (closeout strips the note). Pinned in Task 1's domain test.
4. **`finalize.merge` without an explicit id on a noted record** (the sweep/headless shape) — merges; the marker no longer reaches `NotSuperseded`. Pinned in Task 2's `note-never-blocks-auto-merge`.
5. **The repair's closeout note passes the notes validator** — a realistic `late_findings` entry (commit id, attempt count) must archive under `## Closeout notes`. Pinned in Task 3's e2e assertion.

---

### Task 1: Selection treats a `## Finalize blocked` note as visible only

**Files:**
- Modify: `internal/domain/finalize.go` (`skipFinalizeBlocked` const, `SelectFinalizeQueue`, `classifyFinalize`, their doc comments)
- Modify: `internal/domain/finalize_test.go` (every `SelectFinalizeQueue` call; `TestSelectFinalizeQueueSkipReasons`; `TestSelectFinalizeQueueExplicitOverride`; new `withFinalizeBlocked` + `TestSelectFinalizeQueueNoteNeverSkips`)
- Modify: `internal/app/finalize_context.go` (`ContextFinalize` call site, `overridableSkip`, delete `finalizeBlockedMap`, comments on `FinalizeCandidateReport` and `finalizeExplicitGuard`)
- Modify: `internal/app/maintenance.go` (the sweep's `SelectFinalizeQueue` call)
- Modify: `internal/app/finalize_context_test.go` (`TestContextFinalizeTypedReasons` closed skip set; new `TestContextFinalizeNotedChangeIsSelected`)

**Interfaces:**
- Produces: `func SelectFinalizeQueue(s Snapshot, facts map[ChangeID]PRFacts, allowlist []ChangeID) []FinalizeCandidate` (the `blocked` parameter is gone); `func classifyFinalize(s Snapshot, c Change, facts map[ChangeID]PRFacts) (band, skip string, f PRFacts)`; `overridableSkip(reason string) bool` returns true only for `"approval-required"`.

The signature change breaks both app callers, so the domain and app edits land in one task (the intermediate state must build).

- [ ] **Step 1: Write the failing domain test**

Add to `internal/domain/finalize_test.go`, next to the other option helpers (after `noPR`):

```go
// withFinalizeBlocked marks the record as carrying a `## Finalize blocked` note.
func withFinalizeBlocked() func(*ChangeSpec) {
	return func(sp *ChangeSpec) { sp.HasFinalizeBlocked = true }
}
```

and append this test:

```go
// TestSelectFinalizeQueueNoteNeverSkips pins that a `## Finalize blocked` note is
// visible only: a noted open PR bands like any other candidate (the next run
// retries it), and a noted merged PR is still merged-recovery. Mutation:
// reintroduce `if c.HasFinalizeBlocked() { return "", "finalize-blocked", f }`
// in classifyFinalize and this test fails.
func TestSelectFinalizeQueueNoteNeverSkips(t *testing.T) {
	changes := []Change{
		finChange(1, withFinalizeBlocked()), // open, approved, mergeable, noted
		finChange(2, withFinalizeBlocked()), // merged, noted
	}
	facts := map[ChangeID]PRFacts{
		1: {State: "open", Approved: true, Mergeable: "MERGEABLE", HeadBranch: finDefaultBranch},
		2: {State: "merged", HeadBranch: finDefaultBranch},
	}
	got := SelectFinalizeQueue(finSnapshot(changes...), facts, nil)
	if len(got) != 2 {
		t.Fatalf("candidates = %d, want 2: %+v", len(got), got)
	}
	byID := map[ChangeID]FinalizeCandidate{}
	for _, c := range got {
		byID[c.ID] = c
	}
	if c := byID[1]; c.SkipReason != "" || c.Band != "mergeable" {
		t.Errorf("noted open PR = band %q skip %q, want mergeable/actionable", c.Band, c.SkipReason)
	}
	if c := byID[2]; c.SkipReason != "" || c.Band != "merged-recovery" {
		t.Errorf("noted merged PR = band %q skip %q, want merged-recovery", c.Band, c.SkipReason)
	}
}
```

- [ ] **Step 2: Write the failing app test**

Append to `internal/app/finalize_context_test.go`:

```go
// TestContextFinalizeNotedChangeIsSelected pins that a `## Finalize blocked`
// note — including a legacy repair-needs-signoff note an older binary wrote — is
// visible only: auto-detect selects the change as an ordinary actionable
// candidate (the next run retries it), and an explicit id earns no override
// note, because there is no finalize-blocked skip left to override. Mutation:
// reintroduce a skip on c.HasFinalizeBlocked() in domain.classifyFinalize and
// both subtests fail.
func TestContextFinalizeNotedChangeIsSelected(t *testing.T) {
	pin := docketPin(t)
	for _, tc := range []struct {
		name string
		req  FinalizeContextRequest
	}{
		{"auto-detect", FinalizeContextRequest{}},
		{"explicit-id", FinalizeContextRequest{ID: 81}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			noted := finalizeBlob(81, "noted", "implemented", "high", prRefFor(81), "")
			noted.Data = append(noted.Data, []byte("\n## Finalize blocked\n\n- 2026-10-01 — reason `repair-needs-signoff`: an earlier finalize stopped for a sign-off.\n")...)
			prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{
				prRefFor(81): withHead(openFacts(81, "MERGEABLE", 1, 1), "feat/noted"),
			}}
			fake := &fakeReader{pin: pin, corpus: []StatusBlob{noted}}

			got := ContextFinalize(context.Background(), finalizeDeps(fake, prober, &recordingEngine{}), "", tc.req)
			if got.Result != ResultApplied || len(got.Candidates) != 1 {
				t.Fatalf("result=%q reason=%q candidates=%d", got.Result, got.Reason, len(got.Candidates))
			}
			c := got.Candidates[0]
			if c.SkipReason != "" || c.Band != "mergeable" {
				t.Errorf("noted change = band %q skip %q, want mergeable/actionable", c.Band, c.SkipReason)
			}
			if c.OverrideNote != "" {
				t.Errorf("noted change carries an override note %q; a note is not a skip", c.OverrideNote)
			}
		})
	}
}
```

- [ ] **Step 3: Run the domain test to verify it fails to compile against the old signature**

Run: `go test ./internal/domain/ -count=1 -run 'TestSelectFinalizeQueueNoteNeverSkips'`
Expected: FAIL — a build error in `finalize_test.go` (argument-count mismatch on `SelectFinalizeQueue`: the new test already uses the 3-argument form).

- [ ] **Step 4: Implement the domain change**

In `internal/domain/finalize.go`:

1. Delete the line `skipFinalizeBlocked    = "finalize-blocked"` from the closed skip-reason const block (re-run gofmt; the block re-aligns).
2. Change the signature and the call inside it:

```go
func SelectFinalizeQueue(s Snapshot, facts map[ChangeID]PRFacts, allowlist []ChangeID) []FinalizeCandidate {
```
```go
		band, skip, f := classifyFinalize(s, c, facts)
```
3. In the `SelectFinalizeQueue` doc comment, replace `a nil facts map, a nil blocked map, a nil\n// allowlist,` with `a nil facts map, a nil allowlist,` (keep the sentence otherwise), and add one sentence after the ordering paragraph:

```go
// A `## Finalize blocked` note on a record is visible only and never a skip:
// the next run retries the change.
```
4. Change `classifyFinalize`'s signature to `func classifyFinalize(s Snapshot, c Change, facts map[ChangeID]PRFacts) (band, skip string, f PRFacts)`, delete the block

```go
	if blocked[c.ID()] {
		return "", skipFinalizeBlocked, f
	}
```
and in its doc comment change `then status, PR state, draft, block, and dependency gates in\n// turn;` to `then status, PR state, draft, and dependency gates in\n// turn;`.

- [ ] **Step 5: Update the domain tests to the new signature and closed set**

In `internal/domain/finalize_test.go`:

- Every `SelectFinalizeQueue(x, facts, nil, nil)` becomes `SelectFinalizeQueue(x, facts, nil)`; `SelectFinalizeQueue(finSnapshot(changes...), facts, nil, []ChangeID{3, 1})` becomes `SelectFinalizeQueue(finSnapshot(changes...), facts, []ChangeID{3, 1})`; `SelectFinalizeQueue(finSnapshot(), nil, nil, nil)` becomes `SelectFinalizeQueue(finSnapshot(), nil, nil)`.
- `TestSelectFinalizeQueueSkipReasons`: delete the `finChange(14),               // finalize-blocked` row, the `14: open,` facts entry, the `blocked := map[ChangeID]bool{14: true}` line, and the `14: "finalize-blocked",` want entry; the call becomes `SelectFinalizeQueue(finSnapshot(changes...), facts, nil)`.
- `TestSelectFinalizeQueueExplicitOverride`: replace the whole function with

```go
func TestSelectFinalizeQueueExplicitOverride(t *testing.T) {
	// approval-required is the one skip reason the app layer overrides for an
	// explicit --id; here we only assert the token exists so that override has
	// something to key on.
	changes := []Change{finChange(1)}
	facts := map[ChangeID]PRFacts{
		1: {State: "open", Approved: false, Mergeable: "MERGEABLE"}, // approval-required
	}
	got := SelectFinalizeQueue(finSnapshot(changes...), facts, nil)
	if len(got) != 1 || got[0].SkipReason != "approval-required" {
		t.Fatalf("got %+v, want single id 1 approval-required", got)
	}
}
```

- [ ] **Step 6: Update the app callers**

In `internal/app/finalize_context.go`:

- `queue := domain.SelectFinalizeQueue(snap, facts, finalizeBlockedMap(), allowlistChangeIDs(selectIDs))` → `queue := domain.SelectFinalizeQueue(snap, facts, allowlistChangeIDs(selectIDs))`.
- Delete `finalizeBlockedMap` and its four-line doc comment entirely.
- Replace `overridableSkip` and its comment with:

```go
// overridableSkip reports whether a skip reason is one an explicit --id may
// override at the mutation layer: approval is the only human-overridable skip;
// every other skip reflects a state the merge path cannot be authorized past.
func overridableSkip(reason string) bool {
	return reason == "approval-required"
}
```
- In the `FinalizeCandidateReport` doc comment: `OverrideNote is set when a skip reason (approval-required or\n// finalize-blocked) is one an explicit --id may override` → `OverrideNote is set when the skip reason (approval-required) is\n// one an explicit --id may override`.
- In the `finalizeExplicitGuard` doc comment: `a skip-reasoned candidate (approval-required,\n// finalize-blocked, and the rest)` → `a skip-reasoned candidate (approval-required\n// and the rest)`.

In `internal/app/maintenance.go`: `queue := domain.SelectFinalizeQueue(inv.snap, facts, finalizeBlockedMap(), nil)` → `queue := domain.SelectFinalizeQueue(inv.snap, facts, nil)`.

In `internal/app/finalize_context_test.go` `TestContextFinalizeTypedReasons`: remove `"finalize-blocked": true, ` from the `skipTokens` map (the closed set no longer carries it).

Run: `"$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt" -w internal/domain/finalize.go internal/domain/finalize_test.go internal/app/finalize_context.go internal/app/maintenance.go internal/app/finalize_context_test.go`

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./internal/domain/ -count=1 -run 'TestSelectFinalizeQueue|TestMergeConditions'`
Expected: PASS.

Run: `go test ./internal/app/ -count=1 -run 'TestContextFinalize'`
Expected: PASS.

Run: `go vet ./internal/domain/ ./internal/app/` and `go build ./...`
Expected: clean (no reference to `finalizeBlockedMap` or `skipFinalizeBlocked` remains: `git grep -n -e finalizeBlockedMap -e skipFinalizeBlocked -- internal` prints nothing).

- [ ] **Step 8: Mutation-test both new tests**

Back up `internal/domain/finalize.go` (`cp internal/domain/finalize.go "${TMPDIR:-/tmp}/finalize.go.bak"`), then insert immediately before the `if !EvaluateDependencies(s, c).Satisfied {` line in `classifyFinalize`:

```go
	if c.HasFinalizeBlocked() {
		return "", "finalize-blocked", f
	}
```
Confirm it landed: `grep -c 'HasFinalizeBlocked' internal/domain/finalize.go` prints `1`.
Run: `go test ./internal/domain/ -count=1 -run TestSelectFinalizeQueueNoteNeverSkips` → Expected: FAIL (id 1 skip "finalize-blocked").
Run: `go test ./internal/app/ -count=1 -run TestContextFinalizeNotedChangeIsSelected` → Expected: FAIL in both subtests (this also proves the fixture's appended section decodes as `HasFinalizeBlocked`).
Restore: `cp -f "${TMPDIR:-/tmp}/finalize.go.bak" internal/domain/finalize.go`; confirm `grep -c 'HasFinalizeBlocked' internal/domain/finalize.go` prints `0`; re-run both tests → PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/domain/finalize.go internal/domain/finalize_test.go internal/app/finalize_context.go internal/app/maintenance.go internal/app/finalize_context_test.go
git commit -m "fix(finalize): a Finalize blocked note never removes a change from selection"
```

---

### Task 2: `finalize.merge` never reads the note

**Files:**
- Modify: `internal/app/finalize_merge.go` (`finalizeBlockedHeading` const moves out; `mergeConditionInputs.finalizeBlocked`; `mergeConditions`; `mergeContext.body`; `loadMergeContext`; the condition-recheck call; delete `changeHasFinalizeBlockedMarker` and `headingText`; `mergeConditionMessage("superseded")`; header and doc comments)
- Modify: `internal/app/finalize_block.go` (receives `finalizeBlockedHeading`; header comment paragraph about the reader; `finalizeBlockedSectionHeading` comment)
- Modify: `internal/cli/finalize.go` (the `ExplicitID: true` comment in the `finalize merge` subcommand)
- Test: `internal/app/finalize_merge_integration_test.go` (`TestIntegrationFinalizeMergeConditionAssembly`, `TestIntegrationFinalizeMergeConditionsRechecked`, `TestIntegrationFinalizeMergeExplicitIDOverrides`)

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces: `mergeConditionInputs` without `finalizeBlocked`; `mergeContext` without `body`; `NotSuperseded: in.revisionMatches`. `finalizeBlockedHeading` (value `"Finalize blocked"`) now lives in `internal/app/finalize_block.go`; `finalizeBlockedSectionHeading` is unchanged.

- [ ] **Step 1: Write the failing merge tests**

In `internal/app/finalize_merge_integration_test.go`, `TestIntegrationFinalizeMergeExplicitIDOverrides`: delete the two subtests `explicit-id-merges-past-finalize-blocked` and `auto-refuses-finalize-blocked` (from `t.Run("explicit-id-merges-past-finalize-blocked", ...` through the closing `})` of `auto-refuses-finalize-blocked`, including the `// Without an explicit id, ...` comment) and put this in their place:

```go
	// A `## Finalize blocked` note is visible only: it never stops a merge,
	// named or not. Mutation: reintroduce a refusal on
	// mc.change.HasFinalizeBlocked() before the condition recheck and both
	// subtests fail.
	for _, tc := range []struct {
		name     string
		explicit bool
	}{
		{"note-never-blocks-auto-merge", false},
		{"note-never-blocks-explicit-merge", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupMergeFixture(t, m)
			f.patchParent(t, "implemented", mergePRRef(), "## Finalize blocked\n\n- 2026-10-01 — reason `rebase-stuck`: an earlier attempt halted.")
			mergeCommit := f.mergeFeatureIntoBase(t)
			gh := f.baselineFake(t)
			gh.mergeOutcome = githubcli.MergeMerged
			gh.mergeFacts = mergedFactsFor(f.head, "main", mergeCommit)
			res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, tc.explicit, false))
			if res.Result != ResultApplied || res.Merge == nil {
				t.Fatalf("a Finalize blocked note stopped the merge: %q (reason %q)", res.Result, res.Reason)
			}
			if gh.mergeCalls != 1 {
				t.Fatalf("merge calls = %d, want 1", gh.mergeCalls)
			}
		})
	}
```

Replace the test's doc comment with:

```go
// TestFinalizeMergeExplicitIDOverrides proves an explicit id never overrides
// wrong PR identity, an unsafe stack, the gate, or a superseding revision, and
// that a `## Finalize blocked` note never stops a merge, named or not.
```

In `TestIntegrationFinalizeMergeConditionsRechecked`, delete the `superseded-finalize-blocked` subtest (`t.Run("superseded-finalize-blocked", ...` through its `})`) and change the comment `// Metadata-shaped cases: a stale revision, a durable finalize-blocked marker,\n\t// and a not-implemented status.` to `// Metadata-shaped cases: a stale revision and a not-implemented status.`

- [ ] **Step 2: Run the new subtests to verify they fail**

Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeMergeExplicitIDOverrides$/note-never-blocks' ./internal/app/`
Expected: `note-never-blocks-auto-merge` FAILs (refused `superseded` — the live marker term `in.explicitID || !in.finalizeBlocked`); `note-never-blocks-explicit-merge` passes (explicit id satisfied the term). The auto subtest is the red one this task turns green.

- [ ] **Step 3: Implement the merge change**

In `internal/app/finalize_merge.go`:

1. Delete the `finalizeBlockedHeading` const and its four-line comment, and add to `internal/app/finalize_block.go` directly above `finalizeBlockedSectionHeading`:

```go
// finalizeBlockedHeading is the ATX heading text of the durable
// "## Finalize blocked" section finalize.block writes and finalize.clear-block
// removes. The section is a visible note only: no finalize selection or merge
// reads it.
const finalizeBlockedHeading = "Finalize blocked"
```
and replace the `finalizeBlockedSectionHeading` comment with:

```go
// finalizeBlockedSectionHeading is the full ATX H2 heading line the durable
// finalize-blocked section carries; its heading text is exactly
// finalizeBlockedHeading.
```
2. In `finalize_block.go`'s header comment, replace the paragraph beginning `// Task 10's \`finalize merge\` reads the "## Finalize blocked" marker through a` (four lines, ending `whose ATX text is exactly finalizeBlockedHeading, so the reader keeps working.`) with:

```go
// The section is a visible note: no finalize selection or merge reads it. The
// board and status read its presence through the record decoder.
```
3. Delete the `finalizeBlocked          bool` field from `mergeConditionInputs`.
4. Replace the `mergeConditions` doc comment and the `NotSuperseded` line:

```go
// mergeConditions assembles the domain merge conditions from the live inputs. The
// one human-overridable condition reads the explicit-id flag: an explicit id
// supplies approval. A superseding revision is NEVER overridable — NotSuperseded
// requires an exact revision match regardless of authorization.
```
```go
		NotSuperseded:       in.revisionMatches,
```
5. `mergeContext`: delete the `body     []byte` field and change its comment's `the resolved change and its exact record\n// revision and raw body (for the finalize-blocked marker), the resolved effective\n// base` to `the resolved change and its exact record\n// revision, the resolved effective base`.
6. `loadMergeContext`: its doc comment `resolves the change/revision/body/base/\n// target/repo` → `resolves the change/revision/base/\n// target/repo`; replace

```go
	revision, body := "", []byte(nil)
	for _, b := range blobs {
		if b.Path == c.Path() {
			revision = b.Revision
			body = b.Data
			break
		}
	}
```
with
```go
	revision := ""
	for _, b := range blobs {
		if b.Path == c.Path() {
			revision = b.Revision
			break
		}
	}
```
and the return literal's `snap: snap, change: c, revision: revision, body: body,` → `snap: snap, change: c, revision: revision,`.
7. In the condition-recheck call, delete the line `finalizeBlocked:          changeHasFinalizeBlockedMarker(mc.body),`.
8. Delete `changeHasFinalizeBlockedMarker` and `headingText` with their doc comments (nothing else uses them: `git grep -n -e headingText -e changeHasFinalizeBlockedMarker -- internal` must print nothing afterwards).
9. `mergeConditionMessage`: `case "superseded":` returns `"the request was superseded by a newer record revision; re-read context finalize"`.
10. Header comment point 1: replace its last three lines

```go
//      issues NO merge call. An explicit id (attended, human-named) satisfies
//      the approval and finalize-blocked skips but never a wrong PR identity, an
//      unsafe stack, the repair sign-off (gate), or a superseding revision.
```
with these four lines:

```go
//      issues NO merge call. An explicit id (attended, human-named) satisfies
//      the approval skip but never a wrong PR identity, an unsafe stack, the
//      gate, or a superseding revision. A `## Finalize blocked` note is never a
//      merge condition.
```
11. `FinalizeMergeRequest` doc: `which\n// supplies the approval and finalize-blocked authorization.` → `which\n// supplies the approval authorization.`

In `internal/cli/finalize.go`, the merge subcommand comment `// authorization the approval and finalize-blocked overrides read.` → `// authorization the approval override reads.`

Run the toolchain gofmt on `internal/app/finalize_merge.go internal/app/finalize_block.go internal/cli/finalize.go internal/app/finalize_merge_integration_test.go`.

- [ ] **Step 4: Update the pure condition-assembly test**

In `TestIntegrationFinalizeMergeConditionAssembly`:
- In `good`, change `revisionMatches:          true, finalizeBlocked: false,` to `revisionMatches:          true,` (gofmt re-aligns).
- Delete the case `{"superseded-blocked", func(in *mergeConditionInputs) { in.explicitID = false; in.finalizeBlocked = true }, "superseded"},`.
- Delete the `explicit-id-overrides-blocked` subtest.
- Change the comment `// The overridable conditions: an explicit id satisfies approval and a\n\t// finalize-blocked marker, but never a superseding revision.` to `// The overridable condition: an explicit id satisfies approval, but never a\n\t// superseding revision.`

- [ ] **Step 5: Run the merge tests to verify they pass**

Run: `go build ./... && go vet ./internal/app/ ./internal/cli/`
Expected: clean.

Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeMerge' ./internal/app/`
Expected: PASS (all subtests, both note subtests included).

Run: `go test ./internal/app/ ./internal/cli/ -count=1 -run 'TestFinalizeMerge|TestFinalizeBlock|TestFinalizeClearBlock|TestContextFinalize'`
Expected: PASS (or "no tests to run" for an unmatched pattern — not a failure).

- [ ] **Step 6: Mutation-test the note subtests**

Back up `internal/app/finalize_merge.go`, then insert immediately before the line `evHead, _, evGreen := prBodyEvidence(pr)`:

```go
	if mc.change.HasFinalizeBlocked() {
		return mergeRefusal(ResultContended, MergeDispContended, "superseded", "mutation probe", id)
	}
```
Confirm: `grep -c 'mutation probe' internal/app/finalize_merge.go` prints `1`.
Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeMergeExplicitIDOverrides$/note-never-blocks' ./internal/app/` → Expected: both subtests FAIL.
Restore from the backup with `cp -f`; confirm the count is `0`; re-run → PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/finalize_merge.go internal/app/finalize_block.go internal/cli/finalize.go internal/app/finalize_merge_integration_test.go
git commit -m "fix(finalize): merge never reads a Finalize blocked note"
```

---

### Task 3: End-to-end — a green repair merges with no block

**Files:**
- Test: `internal/app/finalize_e2e_test.go` (`TestE2EConflictAndRepair`)
- Test: `internal/cli/revision_rename_test.go` (`TestRevisionFlagReachesRequest`'s `finalize block` row)

**Interfaces:**
- Consumes: Task 2's merge (no marker term). Uses existing helpers `s.dk`, `s.writeInput`, `s.ver`, `originFile`, `GateLaunch`, `pollGatePassed`, `EvidenceRecord`, `runGit`, `writeRepoFile`.

- [ ] **Step 1: Rewrite the e2e test's repair leg**

Replace the doc comment of `TestE2EConflictAndRepair` with:

```go
// TestE2EConflictAndRepair drives the full conflict/repair closing path through
// CLI argv: a base-conflicting rebase stops CONFLICTED; a verified resolver
// report continues it; the local suite is RED at the rebased head (repair work);
// the repair makes the suite green, and the run re-gates, records evidence,
// publishes, merges, and closes out with the repair named in the archived
// record's closeout notes — no finalize block and no clear-block on the path.
```

Delete step (3) entirely — from the comment `// (3) The operator records the authored repair behind a durable finalize-` through the closing `}` of the `if rec, _ := originFile(...); !strings.Contains(rec, "repair-needs-signoff") {` block (this removes `headAfterContinue`, `blockPath`, and `blk`).

Renumber and reword the following comment from `// (4) Retry: the human signs off. The repair lands ...` to:

```go
	// (3) The repair lands `.repaired` (making the suite green) on the
	// already-rebased head, then the local gate is re-run and its exact-head green
	// evidence recorded through the landed gate/evidence seams (the skill's
	// "gate -> evidence record" step).
```

Replace everything from the `// (5) Publish the repaired head under the owned receipt, clear the block (the` comment to the end of the function with:

```go
	// (4) Publish the repaired head under the owned receipt and merge — a green
	// repair merges like any other green change; there is no block to clear.
	evPath := s.writeInput(t, "repaired-ev.txt", evd.Block)
	pub := s.dk(t, "", "finalize", "publish", "--id", strconv.Itoa(s.id), "--attempt", attempt, "--head", repairHead, "--evidence", evPath)
	if pub.result() != "applied" && pub.result() != "no-op" {
		t.Fatalf("repair publish = %q\n%s", pub.result(), pub.stdout)
	}
	mg := s.dk(t, "", "finalize", "merge", "--id", strconv.Itoa(s.id), "--revision", s.ver(t), "--head", repairHead)
	if mg.result() != "applied" {
		t.Fatalf("post-repair merge = %q\n%s", mg.result(), mg.stdout)
	}

	// (5) Close out with the repair named in the closeout notes, so the archived
	// record keeps what broke and the repair commit.
	finding := "integration repair: the rebased suite was red (missing .repaired); repair commit " + repairHead + "; 1 attempt"
	notesPath := s.writeInput(t, "closeout-notes.json", `{"late_findings":["`+finding+`"]}`)
	co := s.dk(t, "", "finalize", "closeout", "--id", strconv.Itoa(s.id), "--input", notesPath)
	if co.str("disposition") != "done-archived" {
		t.Fatalf("post-repair closeout = %q\n%s", co.str("disposition"), co.stdout)
	}
	archived, ok := originFile(t, s.repo.origin, s.mode.branch, co.str("archive_path"))
	if !ok {
		t.Fatalf("archived record %q not on %s\n%s", co.str("archive_path"), s.mode.branch, co.stdout)
	}
	if !strings.Contains(archived, "## Closeout notes") || !strings.Contains(archived, repairHead) {
		t.Errorf("archived record does not name the repair in its closeout notes:\n%s", archived)
	}
```

If `go vet` reports an unused variable or import afterwards (e.g. nothing else in the function used a removed value), remove only that leftover.

- [ ] **Step 2: Replace the retired reason token in the CLI revision test**

In `internal/cli/revision_rename_test.go`, in the `finalize block` row, change `"--reason", "repair-needs-signoff"` to `"--reason", "rebase-stuck"`.

- [ ] **Step 3: Run the tests**

Run: `go vet -tags e2e ./internal/app/`
Expected: clean.

Run: `go test -tags e2e -count=1 -timeout 20m -run '^TestE2EConflictAndRepair$' ./internal/app/`
Expected: PASS. (Before Task 2 this path would also have merged, because the CLI always sends `ExplicitID: true`; the test pins the end-to-end shape — no block, no clear-block, merge, closeout notes — not a red-to-green transition.)

Run: `go test ./internal/cli/ -count=1 -run '^TestRevisionFlagReachesRequest$'`
Expected: PASS.

Run: `bash tests/test_go_finalize_e2e.sh`
Expected: every line `ok - …`, exit 0.

- [ ] **Step 4: Prove the closeout-notes assert is live**

Back up `internal/app/finalize_e2e_test.go`; change the closeout call to omit `"--input", notesPath` (confirm with `grep -c '"--input", notesPath' internal/app/finalize_e2e_test.go` → `0`). Run the e2e test → Expected: FAIL with "does not name the repair". Restore with `cp -f`, re-run → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_e2e_test.go internal/cli/revision_rename_test.go
git commit -m "test(finalize): a green repair publishes, merges, and closes out with no block"
```

---

### Task 4: Skills and agents state the new rule; regenerate bundle and goldens

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md`
- Modify: `skills/docket-finalize-change/references/gate-failure.md`
- Modify: `skills/docket-convention/SKILL.md` (the `## Finalize blocked` bullet in the marker-section list)
- Modify: `agents/docket-integration-repair.md`
- Modify: `agents/docket-rebase-resolver.md`
- Modify: `internal/repoguard/prose_contracts_test.go` (replace the two `align_0502_signoff` rows)
- Modify: `internal/repoguard/budgets_test.go` (re-baseline the finalize SKILL.md and gate-failure.md ceilings; the convention SKILL.md ceiling only if it changes)
- Regenerate: `internal/assets/embedded/tree/**`, `internal/harness/{claude,codex,cursor,opencode}/testdata/golden/docket-integration-repair.*` and `docket-rebase-resolver.*`

**Interfaces:**
- Produces: the sentinel phrase `A repair that turns the rebased suite green publishes and merges like any other green change` in both finalize files; the agent phrase `when the suite is green, publishes and merges it`.

- [ ] **Step 1: Write the failing sentinel rows**

In `internal/repoguard/prose_contracts_test.go`, replace the two `align_0502_signoff` rows (the comment `// 0502 bug 4: a relayed sign-off is not authority; ...` and both `{sentinel: "align_0502_signoff", ...}` entries) with:

```go
	// 0515: finalize adds no human gate of its own — a repair that turns the
	// rebased suite green publishes and merges, named in the run report and the
	// closeout notes; the retired sign-off token, its block/clear-block ritual,
	// and the never-wired finalize-blocked skip are gone from the agent surfaces.
	{sentinel: "align_0515_green_repair_merges", file: "skills/docket-finalize-change/SKILL.md",
		present: []string{"A repair that turns the rebased suite green publishes and merges like any other green change",
			"one `late_findings` entry naming what broke"},
		absent: []string{"repair-needs-signoff", "First record the sign-off requirement durably",
			"`finalize-blocked`"}},
	{sentinel: "align_0515_green_repair_merges", file: "skills/docket-finalize-change/references/gate-failure.md",
		present: []string{"A repair that turns the rebased suite green publishes and merges like any other green change",
			"Dismiss stale pull request approvals when new commits are pushed"},
		absent: []string{"repair-needs-signoff", "It first records the sign-off requirement durably",
			"Auto-detect selection skips"}},
	{sentinel: "align_0515_green_repair_merges", file: "agents/docket-integration-repair.md",
		present: []string{"when the suite is green, publishes and merges it"},
		absent:  []string{"repair-needs-signoff", "must never merge unseen"}},
	{sentinel: "align_0515_green_repair_merges", file: "agents/docket-rebase-resolver.md",
		absent: []string{"repair sign-off"}},
	{sentinel: "align_0515_green_repair_merges", file: "skills/docket-convention/SKILL.md",
		present: []string{"never stops finalize selection or merge"},
		absent:  []string{"makes later **auto-detect** finalize runs skip the change"}},
```

Run: `go test ./internal/repoguard/ -count=1 -run '^TestAlignmentContracts$'`
Expected: FAIL naming the missing present phrases and the present retired phrases.

- [ ] **Step 2: Rewrite `skills/docket-finalize-change/SKILL.md`**

Make exactly these edits:

1. *Overview*, the "Closeout notes ride the invocation" paragraph: `it records only the
context supplied at invocation.` → `it records only the
context supplied at invocation and the facts of a repair this run authored (step 6).`
2. *Selection*, Explicit id bullet: `it overrides the \`approval-required\` and \`finalize-blocked\` skip reasons (the "I looked at it, merge/retry" signal).` → `it overrides the \`approval-required\` skip reason (the "I looked at it, merge" signal).`
3. *Terminal disposition*, the blocked-but-non-empty paragraph: `A candidate in scope but skipped for any human-requiring reason — a real skip-reason token, or a \`## Finalize blocked\` marker on the auto-detect path (a named id overrides that skip) — counts toward the non-empty set and yields \`halted\`.` → `A candidate in scope but skipped for a human-requiring skip-reason token counts toward the non-empty set and yields \`halted\`.`
4. Step 1: `A candidate whose only skip reason is \`approval-required\` or \`finalize-blocked\` on an explicitly named or allowlisted run` → `A candidate whose only skip reason is \`approval-required\` on an explicitly named or allowlisted run`.
5. Replace the whole `### 6. Sign-off on an authored repair` section (heading through the line `A pass with **no** authored repair (an exact-head-evidence skip, or a clean first-try rebase) skips this step.`) with:

```markdown
### 6. A green repair merges

A repair that turns the rebased suite green publishes and merges like any other green change, on autonomous and attended runs alike: continue to step 7 and step 8 with the repaired head and its recorded evidence. Finalize adds no stop of its own here — no `finalize.block`, no prompt, no `finalize.clear-block`. Approval stays the repository's policy: when branch protection requires approvals and dismisses stale approvals on new commits, publishing the repair removes the PR's approval and the merge waits for a fresh one (`references/gate-failure.md`). The repair stays visible: the run's final report names what broke, the claimed repair commits, and the attempts used, and step 9 records the same facts in the archived record's `## Closeout notes`.

A pass with **no** authored repair (an exact-head-evidence skip, or a clean first-try rebase) has nothing to record here.
```
6. Step 7: delete the final sentence `On the attended repair path, after the human's go-ahead (step 6), follow publish with the \`finalize.clear-block\` operation with \`--id <id> --revision <revision> --head <repaired head> --pr-number <n>\`, which removes the marker only after reprobing the exact current head, valid gate evidence, the published remote ref, and the matching open PR.`
7. Step 8, last sentence of the `--admin` paragraph: `A named id overrides the \`approval-required\` and \`finalize-blocked\` skips (step 1); it never overrides malformed state, a wrong PR identity, an unsafe stack, or the repair sign-off.` → `A named id overrides the \`approval-required\` skip (step 1); it never overrides malformed state, a wrong PR identity, or an unsafe stack.`
8. Step 9: after the sentence ending `With no
notes, call the unchanged no-input form and archive immediately — no post-merge pause or second user step.` insert:

```markdown
A run that authored a repair (step 6) always sends notes: one `late_findings` entry naming what broke, the claimed
repair commits, and the attempts used, alongside any notes from the invocation. If closeout refuses that notes request
for any reason, re-run it once without `--input` and route on that result — a lost note never stops the closeout.
```
9. Rename `## Sign-off, abort, and the blocked marker` to `## Abort and the blocked marker`, and in its paragraph change `The full abort-and-report set, the two-agent split, the sign-off rule, and the \`## Finalize blocked\` marker's write shape and lifecycle` → `The full abort-and-report set, the two-agent split, the green-repair rule, and the \`## Finalize blocked\` marker's write shape and lifecycle`; after `the \`finalize.clear-block\` operation removes it after a successful reprobe.` append ` The marker is a visible note: it never stops selection or merge, so the next run retries the change.`

Verify: `grep -n -e 'finalize-blocked' -e 'repair-needs-signoff' -e 'sign-off' skills/docket-finalize-change/SKILL.md` prints nothing (capture into a variable first, then test it is empty).

- [ ] **Step 3: Rewrite `skills/docket-finalize-change/references/gate-failure.md`**

1. *The two agents* intro: delete the sentence `An authored repair from
\`docket-integration-repair\` fires the sign-off rule below; pure conflict resolution does not.` (the preceding sentence `…other harnesses through their native worktree mechanism.` then ends the paragraph).
2. Replace the whole `## Sign-off on auto-authored repairs` section (heading through `**prompts** for go-ahead before the \`finalize.merge\` operation.`) with:

```markdown
## A green repair merges

A repair that turns the rebased suite green publishes and merges like any other green change, on
autonomous and attended runs alike — finalize adds no human stop of its own, so a repair gets no
`finalize.block`, no prompt, and no `finalize.clear-block`. The repair stays visible: the run's final
report names what broke, the claimed repair commits, and the attempts used, and closeout records the
same facts as a `late_findings` entry under `## Closeout notes`. A refused notes request is retried
once without notes; a lost note never stops closeout.

Approval is the repository's policy, not a docket gate. When branch protection requires approvals and
has GitHub's "Dismiss stale pull request approvals when new commits are pushed" turned on, publishing
the repair dismisses the PR's approval: GitHub refuses the merge (`halted`), and with
`finalize.require_pr_approval: true` auto-detect skips the PR as `approval-required` until a human
approves it again. That setting is off by default; with it off the earlier approval stands and the
repair merges. A team that wants repairs re-reviewed turns it on.
```
3. *abort-and-report points*: delete the bullet `- an **authored repair under autonomous finalize** — the sign-off rule above (\`repair-needs-signoff\`);`.
4. *The \`## Finalize blocked\` marker — write shape and lifecycle*: replace the bullet beginning `- **Auto-detect selection skips** any unmerged change` (three lines) with:

```markdown
- The section is a **visible note, never a stop**: selection and merge ignore it, so the next run
  retries the change and a transient failure (a flaky test, a busy worktree, a moved base) heals on
  its own. An **already-merged PR is a merged-recovery candidate** as always.
```
replace the bullet beginning `- A **\`CONFLICTING\` PR is not marked at selection time**` with:

```markdown
- A **`CONFLICTING` PR is not marked at selection time** — the resolver usually resolves it.
  Marking happens only at an abort-and-report point.
```
and replace the last bullet (beginning `- **The \`finalize.clear-block\` operation removes the section**`) with:

```markdown
- **The `finalize.clear-block` operation removes the section** on an unmerged change by hand: it
  reprobes the exact current head, valid gate evidence, the published remote ref, and the matching
  open PR before removal — each missing condition refuses and the marker stays. Closeout strips a
  stale section when it records a merged change `done` or `stacked-merged`, so a merged record
  carries no marker.
```
Keep every `## ` heading named in `internal/repoguard/prose_contracts_test.go`'s section-scoped rows (`## abort-and-report points (the full set)`, `## The reconciliation-write exception (recover, not abort)`, `## The finalize gate shares the worktree's one lock`) byte-identical.

Verify: `grep -n -e 'repair-needs-signoff' -e 'sign-off' -e 'Auto-detect selection skips' skills/docket-finalize-change/references/gate-failure.md` prints nothing.

- [ ] **Step 4: Edit the convention skill and the two agents**

`skills/docket-convention/SKILL.md`, the `## Finalize blocked` bullet in the marker-section list: replace `presence drives the board's \`finalize blocked — needs you\` cell and makes later **auto-detect** finalize runs skip the change. A human retries a marked change by **naming its id**, which overrides the skip. The clearing rule` with `presence drives the board's \`finalize blocked — needs you\` cell. It is a visible note that never stops finalize selection or merge, so the next finalize run retries the change. The clearing rule` (the rest of the bullet is unchanged).

`agents/docket-integration-repair.md`:
- frontmatter `description:` → `Makes the test suite pass after finalize's rebase lands — root-causes the red tests, writes a minimal fix within the dispatched repair-attempt budget, never weakens tests, and returns a structured repair report the sequencer re-gates before merging.`
- Replace `Because your output is code the human's PR review never saw, a successful repair must never merge unseen. Return your work as a structured repair report` with `Return your work as a structured repair report`.
- Replace the paragraph `The sequencer gates the merge on that report — interactive sign-off after a prompt, or an autonomous run recording a durable \`repair-needs-signoff\` finalize-blocked marker and stopping (\`halted\`).` with `The sequencer re-gates your repaired head and, when the suite is green, publishes and merges it; what broke and your claimed commits go into its run report and the archived record's closeout notes.`

`agents/docket-rebase-resolver.md`: delete the final sentence `Pure conflict resolution completes the merge the human already intended and does not trigger the repair sign-off.`

- [ ] **Step 5: Regenerate the embedded bundle and the harness goldens**

Run: `go generate ./internal/assets/`
Run: `for p in claude codex cursor opencode; do go test ./internal/harness/$p/ -count=1 -update; done`
Then: `git status --porcelain` must list only the five edited sources, `internal/repoguard/prose_contracts_test.go`, `internal/assets/embedded/tree/...` copies of those five sources, and the eight goldens `internal/harness/*/testdata/golden/docket-integration-repair.*` / `docket-rebase-resolver.*`. Anything else is unexpected — stop and investigate.

- [ ] **Step 6: Re-baseline the line/word ceilings**

Measure: `wc -l -w skills/docket-finalize-change/SKILL.md skills/docket-finalize-change/references/gate-failure.md skills/docket-convention/SKILL.md`. In `internal/repoguard/budgets_test.go`, set the `docket-finalize-change/SKILL.md` and `docket-finalize-change/references/gate-failure.md` rows to the exact new line/word counts and prepend to each row's trailing comment `0515: the repair sign-off is retired; a green repair merges (<old L>/<old W> -> <new L>/<new W>); ` (old values: `238/5808` and `160/2086`). Change the `docket-convention/SKILL.md` row only if its count now exceeds `357/7007` (then raise it to the exact count with the same `0515:` annotation) or if it dropped (lower it to the exact count, same annotation).

- [ ] **Step 7: Run the guards and harness tests**

Run: `go test ./internal/repoguard/ ./internal/assets/ ./internal/harness/... -count=1`
Expected: PASS (including `TestAlignmentContracts`, `TestProseContracts`, `TestRebaseRecoveryDocContracts`, the budget test, the embedded-tree correspondence test, and every golden test).

- [ ] **Step 8: Mutation-test the new sentinel**

Back up `skills/docket-finalize-change/SKILL.md` (`cp skills/docket-finalize-change/SKILL.md "${TMPDIR:-/tmp}/finalize-skill.bak"`). Task 4 is not committed yet, so HEAD still holds the original: write it to a scratch file (`git show HEAD:skills/docket-finalize-change/SKILL.md > "${TMPDIR:-/tmp}/finalize-skill.orig"`) and copy its `### 6. Sign-off on an authored repair` section by hand over the new `### 6. A green repair merges` section. Confirm with `grep -c 'repair-needs-signoff' skills/docket-finalize-change/SKILL.md` ≥ `1`. Run `go test ./internal/repoguard/ -count=1 -run '^TestAlignmentContracts$'` → Expected: FAIL for `align_0515_green_repair_merges` (absent phrase present, present phrase missing). Restore with `cp -f` from the backup, confirm the count is `0`, and re-run → PASS. (Never run `go generate` while the mutation is in place.)

- [ ] **Step 9: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md skills/docket-finalize-change/references/gate-failure.md skills/docket-convention/SKILL.md agents/docket-integration-repair.md agents/docket-rebase-resolver.md internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go internal/assets/embedded/tree internal/harness/claude/testdata/golden internal/harness/codex/testdata/golden internal/harness/cursor/testdata/golden internal/harness/opencode/testdata/golden
git commit -m "docs(skills): a green finalize repair merges; the sign-off is retired"
```

---

### Task 5: User docs describe what finalize does

**Files:**
- Modify: `README.md`
- Modify: `docs/concepts/finalize-sequencer.md`
- Modify: `docs/guide/landing-changes.md`
- Modify: `docs/guide/proving-the-build.md`
- Modify: `docs/reference/glossary.md`
- Modify: `docs/install/models-and-effort.md`
- Modify: `docs/comparison/ai-native-sdlc-playbook.md`

**Interfaces:**
- Consumes: the rule as stated in Task 4 (no new terms).

- [ ] **Step 1: README and concepts**

`README.md`, *Where you decide*: replace
```
- **Finalize's confirmations.** Close-out merges only with your authorization, and unattended
  repair at the finalize gate blocks for your sign-off.
```
with
```
- **Finalize's confirmations.** Close-out merges only with your authorization.
```

`docs/concepts/finalize-sequencer.md`: replace the bullet beginning `- A repair that an agent authored during an unattended finalize is not merged on` (four lines, ending `\`docket finalize clear-block\` and runs finalize again.`) with
```
- A repair that turns the suite green merges like any other green change; the
  run report and the archived record's closeout notes name what broke and the
  repair commits. Finalize adds no human stop of its own: when the repository
  requires approvals and dismisses stale approvals on new commits, pushing the
  repair removes the approval and the merge waits for a fresh one.
```
and in *The invariants* replace `if it cannot re-green the suite, the sequence stops
  for a human. An autonomously authored repair always waits for a human's
  sign-off before it merges.` with `if it cannot re-green the suite, the sequence stops
  for a human. A repair that re-greens the suite merges; reviewing repairs is
  the repository's approval policy, not a finalize step.`

- [ ] **Step 2: Landing-changes guide**

`docs/guide/landing-changes.md`:
1. In the `/loop docket-finalize-change <id>,<id>,<id>` bullet: `it merges pull requests \`finalize.require_pr_approval\` would otherwise
  hold, and retries a change already marked \`## Finalize blocked\`.` → `it merges pull requests \`finalize.require_pr_approval\` would otherwise
  hold.`
2. Replace the first paragraph of *When finalize is blocked* (from `A change whose finalize gate fails is marked` through `exactly what re-runs a change already sitting under \`## Finalize blocked\`.`) with:
```
A change whose finalize run stops for a human is marked with a `## Finalize blocked` section (dated in
its body) and shows on the board as **finalize blocked — needs you**. The section is a note, not a
lock: later runs still select the change and retry it, so a transient failure (a flaky test, a busy
worktree, a moved base) heals on its own, and closeout removes the section once the change merges.
To retry one change specifically, name its id: `/loop docket-finalize-change <id>`.
```
3. Delete the paragraph beginning `One block always waits for you, even when you name the id.` (five lines, ending `\`docket finalize clear-block\` and run finalize again.`).
4. At the end of the *Repos that require approvals (human sign-off preserved).* paragraph (after `…forces past an unsatisfiable required review.`), add:
```

When branch protection also turns on GitHub's **Dismiss stale pull request approvals when new commits
are pushed**, a repair finalize pushes to turn the rebased suite green dismisses that approval, and the
merge waits for a fresh one — that is how a team gets repairs re-reviewed. The setting is off by
default; with it off, the earlier approval stands and the repair merges like any other green change.
```

- [ ] **Step 3: Proving-the-build, install, comparison**

`docs/guide/proving-the-build.md`: `then hands a structured report back to the close-out sequence, which gates the merge behind sign-off
on that repair.` → `then hands a structured report back to the close-out sequence, which re-runs the suite and merges once it is
green, naming the repair in its report and the archived record's closeout notes.`

`docs/install/models-and-effort.md`: `(which keeps real prompts — the multi-candidate batch confirmation and
repair sign-off — so a headless drive is authorized by` → `(which keeps a real prompt — the multi-candidate batch confirmation — so a
headless drive is authorized by`.

`docs/comparison/ai-native-sdlc-playbook.md`: in the "Conflict and repair agents at the gate" row, `; unattended repair blocks for sign-off.` → `; a repair that greens the suite merges, named in the closeout notes.`; in the "Human judgement points" row, `finalize confirmations and repair sign-off,` → `finalize confirmations,`.

- [ ] **Step 4: Glossary**

`docs/reference/glossary.md`:
1. *Finalize blocked / reason token / clear-block*: replace `When a gate failure needs a human, finalize writes \`## Finalize blocked\` with a typed **reason
token** (e.g. a mismatched PR head). Auto-detect runs skip a blocked change; naming its id
overrides that. A human clears the block explicitly.` with `When a gate failure needs a human, finalize writes \`## Finalize blocked\` with a typed **reason
token** (e.g. a mismatched PR head). The section is a visible note: selection and merge ignore it,
so the next run retries the change, and closeout removes it after the merge. \`clear-block\` removes
it by hand.`
2. *Finalize selection*: `a named id or allowlist member overrides the \`approval-required\` and
\`finalize-blocked\` skips.` → `a named id or allowlist member overrides the \`approval-required\`
skip.`
3. The `docket-finalize-change` entry: `Naming ids authorizes a headless drive and overrides the \`approval-required\` and
\`finalize-blocked\` skips.` → `Naming ids authorizes a headless drive and overrides the \`approval-required\`
skip.`
4. Delete the whole `### Repair sign-off (\`repair-needs-signoff\`)` entry (heading through its closing code fence and the blank line after it) and its index line `- [Repair sign-off (repair-needs-signoff)](#repair-sign-off-repair-needs-signoff)`.

- [ ] **Step 5: Sweep for leftovers and run the doc guards**

Run (capture, then inspect):
```bash
out="$(git grep -n -I -i -e 'repair-needs-signoff' -e 'repair sign-off' -e 'blocks for your sign-off' -e 'finalize-blocked. skip' -e 'finalize-blocked` skip' -- README.md docs/guide docs/concepts docs/reference docs/install docs/comparison skills agents cursor-rules scripts || true)"; printf '%s\n' "$out"
```
Expected: empty. (Matches under `docs/adrs/`, `docs/changes/`, `docs/results/`, `docs/superpowers/`, `testdata/`, and `internal/install/legacydata/` are frozen records and stay.)

Run: `go test ./internal/repoguard/ -count=1`
Expected: PASS (docs alignment, glossary/link guards, absence seals).

- [ ] **Step 6: Commit**

```bash
git add README.md docs/concepts/finalize-sequencer.md docs/guide/landing-changes.md docs/guide/proving-the-build.md docs/reference/glossary.md docs/install/models-and-effort.md docs/comparison/ai-native-sdlc-playbook.md
git commit -m "docs: a green finalize repair merges; Finalize blocked is a visible note"
```

---

### Task 6: Whole-suite gate

**Files:** none (verification only).

- [ ] **Step 1: Confirm no maintained Go source still names the retired machinery**

```bash
out="$(git grep -n -e finalizeBlockedMap -e skipFinalizeBlocked -e changeHasFinalizeBlockedMarker -e 'in.finalizeBlocked' -e 'repair-needs-signoff' -- '*.go' ':!internal/assets/embedded' || true)"; printf '%s\n' "$out"
```
Expected: only the legacy-note fixture line in `internal/app/finalize_context_test.go` (`TestContextFinalizeNotedChangeIsSelected`), which is intentional.

- [ ] **Step 2: Run the whole suite from source**

Run: `go run ./cmd/docket development test`
Expected: exit 0. Read the budget report even on green: any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding to note; a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach to act on (most likely candidates: `tests/test_go_finalize_e2e.sh`, `tests/test_go_integration_app_merge.sh`).

- [ ] **Step 3: If anything is red, fix it in the task that owns the file and re-run the suite; no separate commit for a green run.**

---

## Notes for the executor

- Rollout: this edits an agent file and the finalize skill. After the install, running sessions must be restarted before the next finalize so no run mixes the old and new contracts (report this in the results file; no code change).
- `docs/adrs/**` is out of scope for every task: the new ADR and the dated `## Update` on ADR-0010 and ADR-0008 are recorded by the parent through `docket-adr`.
