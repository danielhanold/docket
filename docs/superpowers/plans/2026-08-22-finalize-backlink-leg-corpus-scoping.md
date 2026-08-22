<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0337 — Finalize's integration-ref backlink leg refuses on unrelated pre-existing corpus errors** — `docs/changes/active/0337-finalize-leaves-a-permanent-terminal-backlink-pending-leg-un.md`
<!-- docket:backlink:end -->
# Finalize Integration-Ref Backlink Leg Corpus Scoping — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finalize's two integration-ref backlink legs stop refusing on unrelated pre-existing corpus errors on the integration branch (spec item A), and any remaining in-scope leg failure surfaces its typed cause instead of a bare coarse token (spec item D).

**Architecture:** Both legs — `runCloseoutBacklinkLeg` (`internal/app/finalize_closeout.go`) and `finalizeCleanupBacklinkRepair` (`internal/app/finalize_cleanup.go`) — currently run their transaction with `Loader: newPlanningLoader(cc.eff)`, which loads and validates the *entire* corpus on `refs/heads/<integration_branch>`; the engine refuses at `LoadBefore` when that corpus has any error, even though the mutation only patches `docket:backlink` blocks of plan/results artifacts, which are not corpus records. The fix swaps in a new scoped `StateLoader` that reads and parse-validates only the artifact paths actually patched (A), and a shared detail renderer that folds the transaction's typed cause — the `*transaction.Failure` on a failed disposition, the refusal findings on a refused one — into the `terminal-backlink-pending` finding message (D). No engine change, no new config knob, idempotency and retry-safety preserved.

**Tech Stack:** Go; existing packages `internal/repository/transaction`, `internal/document`, `internal/domain`, `internal/gitcli`; real-git integration test fixtures in `internal/app`.

**Spec:** `docs/superpowers/specs/2026-08-22-finalize-integration-backlink-corpus-scoping-design.md` (synchronized copy read from the metadata worktree; the spec travels with the change).

## Global Constraints

- **A + D only.** Spec item C (republishing the corrected ADR-0024 onto `main`) is explicitly OUT of this branch's scope: it is repo-local hygiene done directly to `main`. Do NOT touch anything under `docs/adrs/` and do NOT add any task for it.
- **No new config knob** — the fix is behavioral in the runtime (spec, "Cross-repo rollout").
- **No engine change.** `internal/repository/transaction/` is untouched: `Loader` is caller-supplied by design ("It is caller-supplied so the engine never embeds a second production composer" — `state.go`).
- **Idempotency preserved:** a replay stays a clean no-op; a landed leg's integration commit touches exactly the plan/results paths. The existing proofs (`TestCloseoutBacklinkLegDocketMode`, the `metadata-closeout-and-backlink-idempotent` subtest in `finalize_git_test.go`) must stay green unmodified.
- **Both legs change together.** The cleanup leg is the closeout leg's twin (learnings: `fix-reintroduces-its-own-defect-class` — "check the twin it did not touch").
- Go tests: always `-count=1` when re-running after an edit or during a mutation probe (learnings: `cached-runner-serves-a-mutated-tree`).
- Full suite at the build gate is `scripts/run-tests.sh` (that is what `finalize.test_command` resolves to in `.docket.yml`); act on any trailing `OVER BUDGET:` line.
- All paths below are repo-relative to the feature worktree.

## Verified code facts the tasks rely on

(Confirmed against the current feature tree at plan time — do not re-derive.)

- Both backlink operations (`closeoutBacklinkOp.Plan`, `cleanupBacklinkOp.Plan`) read artifacts through `st.Tree` (`readTreeBlob`) only; **neither reads `st.State.Snapshot`**, so a scoped loader may leave `LoadedState.Snapshot` zero-valued.
- Both leg requests pass **no `Expected` expectations**, so `checkExpectations` runs over an empty set regardless of loader.
- The engine gates `before.Report.HasErrors()` and `after.Report.HasErrors()` (`engine.go`, "5."/"8.") and calls `req.Loader.ValidateEvolution(before, after)`; a refusal (`refusedOutcome`) carries findings in `res.Findings` with a **nil Go error**, while a failed disposition carries a typed `*transaction.Failure` as `execErr` (rendered by the existing `failureStatus` in `internal/app/planning.go`).
- `transaction.Tree` is `ListTree(ctx, prefixes []gitcli.RepoPath)` + `ReadBlobs(ctx, paths []gitcli.RepoPath) ([]gitcli.BlobResult, error)`; a `BlobResult` has `Path`, `Found`, and `Blob.Bytes` (see `planningLoader.Load`).
- `planningParseFinding` (`internal/app/planning.go`) is the model for normalizing a `document.Parse` failure into an error-severity `domain.Finding` with the typed `document.Error` kind as its code.
- Fixtures: `setupCloseoutFixture` (`internal/app/finalize_closeout_test.go`) seeds plan/results with backlink blocks on `main` in docket mode via `f.repo.writerAdvance(t, "main", files)`; `planRepoModeDocket()` (`internal/app/finalize_cleanup_test.go`) returns the docket mode; `(f *closeoutFixture).archiveClosed` drives merge + closeout to done; `artifactWithBacklink(activePath, heading, body)` renders an artifact whose backlink targets `activePath`; `originFile` / `originTip` / `originCommitPaths` read origin state.
- A file whose bytes begin with `---` and carry a YAML scalar with an unquoted colon-space (e.g. ``title: uses `context: fork` dispatch``) fails `document.Parse` — this is the exact real-world trigger (ADR-0024 on docket's `main`).

## File Structure

- Create: `internal/app/finalize_backlink_loader.go` — the scoped `StateLoader` (`newBacklinkArtifactLoader`) plus the shared leg-detail renderer (`backlinkLegDetail`). One file: both pieces exist only for the two backlink legs.
- Create: `internal/app/finalize_backlink_loader_test.go` — hermetic unit tests for the loader and the detail renderer (fake `transaction.Tree`).
- Modify: `internal/app/finalize_closeout.go` — `runCloseoutBacklinkLeg`: loader swap + detail-carrying message + comment rewrite.
- Modify: `internal/app/finalize_cleanup.go` — `finalizeCleanupBacklinkRepair`: loader swap + detail-carrying message + comment rewrite.
- Test (integration): `internal/app/finalize_closeout_test.go` — closeout-leg regression + diagnosability tests.
- Test (integration): `internal/app/finalize_cleanup_test.go` — cleanup-leg regression test.

---

### Task 1: Scoped backlink-artifact StateLoader

**Files:**
- Create: `internal/app/finalize_backlink_loader.go`
- Test: `internal/app/finalize_backlink_loader_test.go`

**Interfaces:**
- Consumes: `transaction.StateLoader`, `transaction.Tree`, `transaction.LoadedState`, `document.Parse`, `domain.NewValidationReport`, the existing `closeoutBacklinkTarget` struct (`artifactPaths []string`, `interior string` — `finalize_closeout.go`).
- Produces: `func newBacklinkArtifactLoader(targets []closeoutBacklinkTarget) transaction.StateLoader` — Tasks 2 and 3 pass this as `Loader:` in their `transaction.Request`.

- [ ] **Step 1: Write the failing unit test**

Create `internal/app/finalize_backlink_loader_test.go`:

```go
package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// fakeBacklinkTree serves scripted blobs; a nil entry answers "absent".
type fakeBacklinkTree struct {
	blobs map[string][]byte
}

func (f fakeBacklinkTree) ListTree(_ context.Context, _ []gitcli.RepoPath) ([]gitcli.TreeEntry, error) {
	panic("backlink loader must not list the tree — it reads exact paths only")
}

func (f fakeBacklinkTree) ReadBlobs(_ context.Context, paths []gitcli.RepoPath) ([]gitcli.BlobResult, error) {
	out := make([]gitcli.BlobResult, len(paths))
	for i, p := range paths {
		out[i].Path = p
		if b, ok := f.blobs[string(p)]; ok {
			out[i].Found = true
			out[i].Blob.Bytes = append([]byte(nil), b...)
		}
	}
	return out, nil
}

const backlinkTestArtifact = "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
	"> line\n" +
	"<!-- docket:backlink:end -->\n\n# Plan\n\nBody.\n"

// A well-formed frontmatter file that fails document.Parse: unquoted colon-space
// in a scalar — the exact real-world trigger (ADR-0024 on docket's main).
const backlinkTestMalformed = "---\ntitle: uses `context: fork` dispatch\n---\n\n# Doc\n"

func backlinkLoaderFor(paths ...string) transaction.StateLoader {
	return newBacklinkArtifactLoader([]closeoutBacklinkTarget{{artifactPaths: paths, interior: "x"}})
}

// TestBacklinkLoaderScopedToArtifacts: the loader validates ONLY the targeted
// artifact paths — it never lists (fake panics on ListTree), a parse-clean
// artifact yields no error, and an absent artifact is a clean skip.
func TestBacklinkLoaderScopedToArtifacts(t *testing.T) {
	tree := fakeBacklinkTree{blobs: map[string][]byte{
		"docs/superpowers/plans/p.md": []byte(backlinkTestArtifact),
	}}
	l := backlinkLoaderFor("docs/superpowers/plans/p.md", "docs/changes/results/r.md")
	st, err := l.Load(context.Background(), tree)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.Report.HasErrors() {
		t.Fatalf("clean artifacts reported errors: %+v", st.Report.Findings())
	}
	if _, ok := st.Sources["docs/superpowers/plans/p.md"]; !ok {
		t.Errorf("present artifact missing from Sources")
	}
	if _, ok := st.Sources["docs/changes/results/r.md"]; ok {
		t.Errorf("absent artifact conjured Sources bytes")
	}
	if evo := l.ValidateEvolution(st, st); len(evo) != 0 {
		t.Errorf("ValidateEvolution over artifacts returned findings: %+v", evo)
	}
}

// TestBacklinkLoaderRefusesOnTargetedParseFailure: a targeted artifact whose
// bytes fail document.Parse is an error-severity finding naming that path —
// the ONE in-scope condition the leg still gates on.
func TestBacklinkLoaderRefusesOnTargetedParseFailure(t *testing.T) {
	tree := fakeBacklinkTree{blobs: map[string][]byte{
		"docs/superpowers/plans/p.md": []byte(backlinkTestMalformed),
	}}
	st, err := backlinkLoaderFor("docs/superpowers/plans/p.md").Load(context.Background(), tree)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !st.Report.HasErrors() {
		t.Fatalf("malformed targeted artifact did not report an error")
	}
	fds := st.Report.Findings()
	found := false
	for _, fd := range fds {
		if fd.Entity.Path == "docs/superpowers/plans/p.md" && fd.Code != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("no finding names the malformed artifact path: %+v", fds)
	}
}

// TestBacklinkLegDetailRendersCauses: the shared renderer folds a refusal's
// findings (code + path) and a failure's typed stage/kind/detail into prose;
// a non-failed, non-refused result renders empty.
func TestBacklinkLegDetailRendersCauses(t *testing.T) {
	refused := transaction.Result{Disposition: transaction.DispositionRefused}
	refused.Findings = st0Findings() // helper below
	got := backlinkLegDetail(refused, nil)
	if !strings.Contains(got, "docs/adrs/0099-x.md") {
		t.Errorf("refusal detail does not name the offending path: %q", got)
	}

	failed := transaction.Result{Disposition: transaction.DispositionFailed}
	ferr := &transaction.Failure{Stage: transaction.StagePush, Kind: transaction.KindExternal, Detail: "push rejected"}
	got = backlinkLegDetail(failed, ferr)
	if !strings.Contains(got, string(transaction.StagePush)) || !strings.Contains(got, "push rejected") {
		t.Errorf("failure detail lost stage/detail: %q", got)
	}

	if got := backlinkLegDetail(transaction.Result{Disposition: transaction.DispositionContended}, nil); got != "" {
		t.Errorf("contended rendered a spurious detail: %q", got)
	}
}
```

Also add the tiny findings helper to the same test file (imports `"github.com/danielhanold/docket/internal/domain"`):

```go
func st0Findings() []domain.Finding {
	return []domain.Finding{{
		Code: "parse-failed", Severity: domain.SeverityError,
		Entity: domain.EntityRef{Path: "docs/adrs/0099-x.md"},
	}}
}
```

If a struct-field spelling in the fake (`Blob.Bytes`, `TreeEntry`) does not compile, mirror the exact field names `planningLoader.Load` in `internal/app/planning.go` uses — that function is the authoritative consumer of `BlobResult`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/app/ -run 'TestBacklinkLoader|TestBacklinkLegDetail' -count=1`
Expected: FAIL to compile — `newBacklinkArtifactLoader` and `backlinkLegDetail` undefined.

- [ ] **Step 3: Implement the loader and the detail renderer**

Create `internal/app/finalize_backlink_loader.go`:

```go
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// backlinkArtifactLoader is the scoped StateLoader for the two integration-ref
// backlink legs (runCloseoutBacklinkLeg, finalizeCleanupBacklinkRepair). The
// legs patch only the docket:backlink block of merged plan/results artifacts —
// paths that are not corpus records — so validating the integration branch's
// full corpus (newPlanningLoader) was never the right gate: the integration
// branch legitimately holds a partial corpus, and a pre-existing error in a
// record the mutation cannot touch must not refuse the patch. This loader
// reads exactly the targeted artifact paths and reports a parse failure of a
// TARGETED artifact as the one in-scope error; an absent artifact is a clean
// skip (the operation's own benign no-op case). Snapshot is left zero-valued:
// both backlink operations read through st.Tree only and never consult
// st.State.Snapshot, and the legs declare no entity expectations.
type backlinkArtifactLoader struct {
	paths []gitcli.RepoPath
}

// newBacklinkArtifactLoader builds the scoped loader over every artifact path
// the targets patch.
func newBacklinkArtifactLoader(targets []closeoutBacklinkTarget) transaction.StateLoader {
	var paths []gitcli.RepoPath
	for _, tg := range targets {
		for _, p := range tg.artifactPaths {
			paths = append(paths, gitcli.RepoPath(p))
		}
	}
	return backlinkArtifactLoader{paths: paths}
}

// Load reads the targeted artifacts through t. A tree/read failure is a Go
// error; a targeted artifact whose bytes fail document.Parse is an
// error-severity finding naming that path — the engine refuses on it at
// LoadBefore, and the leg's pending finding then carries the cause.
func (l backlinkArtifactLoader) Load(ctx context.Context, t transaction.Tree) (transaction.LoadedState, error) {
	blobs, err := t.ReadBlobs(ctx, l.paths)
	if err != nil {
		return transaction.LoadedState{}, fmt.Errorf("backlink loader: reading artifacts: %w", err)
	}
	documents := make(map[string]document.Document, len(blobs))
	sources := make(map[string][]byte, len(blobs))
	var findings []domain.Finding
	for _, b := range blobs {
		if !b.Found {
			continue // absent on the integration ref: the operation's benign skip
		}
		rel := string(b.Path)
		doc, perr := document.Parse(b.Blob.Bytes)
		if perr != nil {
			findings = append(findings, backlinkParseFinding(rel, perr))
			continue
		}
		documents[rel] = doc
		sources[rel] = append([]byte(nil), b.Blob.Bytes...)
	}
	return transaction.LoadedState{
		Report:    domain.NewValidationReport(findings),
		Documents: documents,
		Sources:   sources,
	}, nil
}

// ValidateEvolution: the corpus evolution rules govern records; the targeted
// plan/results artifacts are not records, so no before/after rule applies.
func (l backlinkArtifactLoader) ValidateEvolution(_, _ transaction.LoadedState) []domain.Finding {
	return nil
}

// backlinkParseFinding normalizes a document.Parse failure on a targeted
// artifact into an error-severity finding, mirroring planningParseFinding's
// code normalization; the entity kind is empty because the artifact is not a
// corpus record.
func backlinkParseFinding(rel string, err error) domain.Finding {
	code := "parse-failed"
	var de *document.Error
	if errors.As(err, &de) {
		code = string(de.Kind)
	}
	return domain.Finding{
		Code:     code,
		Severity: domain.SeverityError,
		Entity:   domain.EntityRef{Path: rel},
	}
}

// backlinkLegDetail renders the typed cause of a backlink leg that did not
// land, so the terminal-backlink-pending finding is self-diagnosing. A failed
// disposition renders the typed *transaction.Failure (stage/kind: detail); a
// refused disposition renders each refusal finding's code and path — after the
// scoped loader, the only refusals left are in-scope artifact-level ones. Any
// other disposition renders empty (the coarse token alone already says it).
func backlinkLegDetail(res transaction.Result, execErr error) string {
	if fs := failureStatus(res, execErr); fs != nil {
		out := fs.Kind
		if fs.Stage != "" {
			out = fs.Stage + "/" + fs.Kind
		}
		if fs.Detail != "" {
			out += ": " + fs.Detail
		}
		return out
	}
	if res.Disposition == transaction.DispositionRefused && len(res.Findings) > 0 {
		parts := make([]string, 0, len(res.Findings))
		for _, f := range res.Findings {
			p := f.Code
			if f.Entity.Path != "" {
				p += " at " + f.Entity.Path
			}
			parts = append(parts, p)
		}
		return "refused: " + strings.Join(parts, "; ")
	}
	return ""
}
```

If `domain.EntityRef`'s field spellings differ, mirror `planningParseFinding` in `internal/app/planning.go` exactly. If `FailureStatus` field names differ, mirror `failureStatus` in the same file.

- [ ] **Step 4: Run the unit tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestBacklinkLoader|TestBacklinkLegDetail' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_backlink_loader.go internal/app/finalize_backlink_loader_test.go
git commit -m "feat(0337): scoped StateLoader + typed-detail renderer for the backlink legs"
```

---

### Task 2: Closeout leg — swap the loader; regression test against a corrupted integration corpus

**Files:**
- Modify: `internal/app/finalize_closeout.go` (`runCloseoutBacklinkLeg`)
- Test: `internal/app/finalize_closeout_test.go`

**Interfaces:**
- Consumes: `newBacklinkArtifactLoader(targets []closeoutBacklinkTarget) transaction.StateLoader` (Task 1); fixtures `setupCloseoutFixture`, `planRepoModeDocket()`, `(f).mergeIntoBase`, `(f).baselineMergedFake`, `originTip`, `originFile`, `originCommitPaths`, `ReasonCloseoutBacklinkPending`.
- Produces: the closeout leg running under the scoped loader — the behavior Task 4's message change and Task 5's suite gate build on.

- [ ] **Step 1: Write the failing regression test**

Append to `internal/app/finalize_closeout_test.go` (a docket-mode-only test: in main mode the backlinks ride the metadata transaction and there is no integration leg):

```go
// --- TestCloseoutBacklinkLegIgnoresUnrelatedCorpusErrors ------------------

// TestCloseoutBacklinkLegIgnoresUnrelatedCorpusErrors is the 0337 regression:
// the integration branch carries a pre-existing corpus record the mutation
// never touches whose bytes fail document.Parse (an ADR with an unquoted
// colon-space title — the live ADR-0024 trigger). The backlink-only patch must
// LAND anyway: the leg's gate is scoped to the artifacts it patches, not the
// health of the integration branch's partial corpus.
func TestCloseoutBacklinkLegIgnoresUnrelatedCorpusErrors(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModeDocket())
	// Pre-existing, mutation-unrelated corpus error on the integration branch.
	f.repo.writerAdvance(t, "main", map[string]string{
		"docs/adrs/0099-malformed.md": "---\n" +
			"id: 99\n" +
			"title: uses `context: fork` dispatch\n" +
			"status: Accepted\n" +
			"date: 2026-08-22\n" +
			"---\n\n# 99. Malformed on purpose\n",
	})
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)
	mainBefore := originTip(t, f.repo.origin, "main")

	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("closeout = %q disp %q (reason %q)", res.Result, res.Disposition, res.Reason)
	}
	// The leg LANDED: no pending finding, and the integration ref advanced.
	for _, fd := range res.Findings {
		if fd.Code == ReasonCloseoutBacklinkPending {
			t.Fatalf("unrelated corpus error refused the backlink leg: %+v", fd)
		}
	}
	mainAfter := originTip(t, f.repo.origin, "main")
	if mainAfter == mainBefore {
		t.Fatalf("the integration ref did not advance (no backlink leg)")
	}
	// The leg's commit touched exactly the plan/results — the malformed record
	// and every other corpus byte are untouched.
	got := originCommitPaths(t, f.repo.origin, mainAfter)
	want := []string{f.planPath, f.resultsPath}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("integration-ref commit changed %v, want exactly %v", got, want)
	}
	// The retarget itself happened: the plan now backlinks the archive path.
	plan, ok := originFile(t, f.repo.origin, "main", f.planPath)
	if !ok {
		t.Fatalf("plan artifact vanished from main")
	}
	if !strings.Contains(plan, res.ArchivePath) {
		t.Errorf("plan backlink does not point at the archive path %q:\n%s", res.ArchivePath, plan)
	}
	if !strings.Contains(plan, "# Plan\n\nThe widget plan.") {
		t.Errorf("authored plan body disturbed:\n%s", plan)
	}
}
```

- [ ] **Step 2: Run it to verify it fails for the right reason**

Run: `go test ./internal/app/ -run TestCloseoutBacklinkLegIgnoresUnrelatedCorpusErrors -count=1`
Expected: FAIL at the pending-finding assert — the full-corpus loader refuses on the malformed ADR and the leg emits `terminal-backlink-pending`. This failure is also the proof that the fixture's malformed record genuinely trips the old gate (learnings: `assert-pins-outcome-not-mechanism` — a fixture that never triggered the old behavior would pass vacuously after any change). If it instead PASSES pre-fix, stop: the fixture record is not being classified as a corpus record (check the path against `corpusPrefixes`) — fix the fixture, do not proceed.

- [ ] **Step 3: Swap the closeout leg's loader**

In `internal/app/finalize_closeout.go`, `runCloseoutBacklinkLeg`, change one line of the `transaction.Request`:

```go
		Loader:     newBacklinkArtifactLoader(backlinkTargets),
```

(replacing `Loader:     newPlanningLoader(cc.eff),`). Nothing else in the function changes in this task.

- [ ] **Step 4: Run the regression and the existing leg proofs**

Run: `go test ./internal/app/ -run 'TestCloseoutBacklinkLeg|TestCloseoutOrdinary|TestCloseoutNeverEditsAuthoredBytes' -count=1`
Expected: PASS — the regression lands, and the pre-existing idempotency/exactness proofs are unaffected.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_closeout.go internal/app/finalize_closeout_test.go
git commit -m "fix(0337): closeout backlink leg gates only the artifacts it patches"
```

---

### Task 3: Cleanup leg — swap the loader; regression test for the self-healing sweep

**Files:**
- Modify: `internal/app/finalize_cleanup.go` (`finalizeCleanupBacklinkRepair`)
- Test: `internal/app/finalize_cleanup_test.go`

**Interfaces:**
- Consumes: `newBacklinkArtifactLoader` (Task 1); fixtures `setupCloseoutFixture`, `planRepoModeDocket()`, `(f).archiveClosed`, `(f).mergedCleanupFake`, `(f).cleanupDeps`, `artifactWithBacklink`, `groomPath`, `originFile`, `ReasonCleanupBacklinkPending`, `CleanupDispCleaned`.
- Produces: the cleanup repair leg running under the scoped loader — the mechanism by which a repo's backlog of stuck backlinks self-heals on the next sweep (spec, "Cross-repo rollout").

- [ ] **Step 1: Write the failing regression test**

Append to `internal/app/finalize_cleanup_test.go`:

```go
// --- TestCleanupBacklinkRepairIgnoresUnrelatedCorpusErrors ----------------

// TestCleanupBacklinkRepairIgnoresUnrelatedCorpusErrors is the 0337 sweep
// self-heal proof: a change closed out while the bug was live left stale
// active-path backlinks on the integration branch, which ALSO carries an
// unrelated malformed corpus record. The next cleanup must land the retarget
// anyway — the repair leg's gate is scoped to the artifacts it patches.
func TestCleanupBacklinkRepairIgnoresUnrelatedCorpusErrors(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModeDocket())
	head, mergeCommit := f.archiveClosed(t)

	// Recreate the stuck state on the integration branch: revert both artifacts
	// to stale active-path backlinks AND plant the unrelated malformed record.
	recPath := groomPath(f.id, f.slug)
	f.repo.writerAdvance(t, "main", map[string]string{
		f.planPath:    artifactWithBacklink(recPath, "Plan", "The widget plan."),
		f.resultsPath: artifactWithBacklink(recPath, "Results", "The widget results."),
		"docs/adrs/0099-malformed.md": "---\n" +
			"id: 99\n" +
			"title: uses `context: fork` dispatch\n" +
			"status: Accepted\n" +
			"date: 2026-08-22\n" +
			"---\n\n# 99. Malformed on purpose\n",
	})

	gh := f.mergedCleanupFake(head, mergeCommit)
	res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
	for _, fd := range res.Findings {
		if fd.Code == ReasonCleanupBacklinkPending {
			t.Fatalf("unrelated corpus error refused the cleanup repair leg: %+v", fd)
		}
	}
	if res.Disposition != CleanupDispCleaned {
		t.Fatalf("cleanup disposition = %q (reason %q msg %q), want %q",
			res.Disposition, res.Reason, res.Message, CleanupDispCleaned)
	}
	// The stale backlinks were re-pointed at the archive path.
	for _, p := range []string{f.planPath, f.resultsPath} {
		got, ok := originFile(t, f.repo.origin, "main", p)
		if !ok {
			t.Fatalf("artifact %q vanished from main", p)
		}
		if strings.Contains(got, recPath) {
			t.Errorf("artifact %q still backlinks the stale active path %q:\n%s", p, recPath, got)
		}
		if !strings.Contains(got, "docs/changes/archive/") {
			t.Errorf("artifact %q does not backlink an archive path:\n%s", p, got)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails for the right reason**

Run: `go test ./internal/app/ -run TestCleanupBacklinkRepairIgnoresUnrelatedCorpusErrors -count=1`
Expected: FAIL — the repair leg (still on the full-corpus loader) refuses on the malformed ADR: either the `ReasonCleanupBacklinkPending` finding assert or the `CleanupDispCleaned` assert reddens. As in Task 2, a pre-fix PASS means the fixture never tripped the old gate — stop and fix the fixture.

- [ ] **Step 3: Swap the cleanup leg's loader**

In `internal/app/finalize_cleanup.go`, `finalizeCleanupBacklinkRepair`, change one line of the `transaction.Request`:

```go
		Loader:     newBacklinkArtifactLoader(backlinkTargets),
```

(replacing `Loader:     newPlanningLoader(cc.eff),`). Nothing else in the function changes in this task.

- [ ] **Step 4: Run the regression and the cleanup suite file**

Run: `go test ./internal/app/ -run 'TestCleanupBacklinkRepairIgnoresUnrelatedCorpusErrors|TestFinalizeCleanup' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_cleanup.go internal/app/finalize_cleanup_test.go
git commit -m "fix(0337): cleanup backlink repair gates only the artifacts it patches"
```

---

### Task 4: Surface the typed failure in both legs' pending findings (spec D)

**Files:**
- Modify: `internal/app/finalize_closeout.go` (`runCloseoutBacklinkLeg`)
- Modify: `internal/app/finalize_cleanup.go` (`finalizeCleanupBacklinkRepair`)
- Test: `internal/app/finalize_closeout_test.go`

**Interfaces:**
- Consumes: `backlinkLegDetail(res transaction.Result, execErr error) string` (Task 1).
- Produces: `terminal-backlink-pending` findings whose `Message` names the cause (stage/kind/detail or refusal code + path) — what a future operator or agent diagnoses from without a source dive.

- [ ] **Step 1: Write the failing diagnosability test**

Append to `internal/app/finalize_closeout_test.go`. The forced still-in-scope failure: the plan artifact ITSELF is malformed on the integration ref, so the scoped loader refuses with a finding naming that exact path.

```go
// --- TestCloseoutBacklinkPendingFindingNamesTheCause ----------------------

// TestCloseoutBacklinkPendingFindingNamesTheCause is the 0337 diagnosability
// proof (spec D): when the leg still cannot land — here an IN-SCOPE failure,
// the targeted plan artifact's own bytes fail document.Parse — the
// terminal-backlink-pending finding carries the typed cause (the offending
// artifact path), never a bare coarse token. The change itself still closes
// out done+archived: the leg stays best-effort.
func TestCloseoutBacklinkPendingFindingNamesTheCause(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModeDocket())
	// Corrupt the targeted plan artifact on the integration branch: malformed
	// frontmatter fails document.Parse, an in-scope condition even after the
	// gate is scoped to the patched artifacts.
	f.repo.writerAdvance(t, "main", map[string]string{
		f.planPath: "---\ntitle: uses `context: fork` dispatch\n---\n\n" +
			artifactWithBacklink(groomPath(f.id, f.slug), "Plan", "The widget plan."),
	})
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)

	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("closeout = %q disp %q (reason %q)", res.Result, res.Disposition, res.Reason)
	}
	var pending *StatusFinding
	for i, fd := range res.Findings {
		if fd.Code == ReasonCloseoutBacklinkPending {
			pending = &res.Findings[i]
		}
	}
	if pending == nil {
		t.Fatalf("an in-scope malformed artifact did not leave the leg pending: %+v", res.Findings)
	}
	// The finding names the cause: the exact offending artifact path.
	if !strings.Contains(pending.Message, f.planPath) {
		t.Errorf("pending finding does not name the offending artifact:\n%s", pending.Message)
	}
	// And is not merely the old opaque form ending at the coarse token.
	if strings.HasSuffix(strings.TrimSpace(pending.Message), "the sweep will retry it") &&
		!strings.Contains(pending.Message, f.planPath) {
		t.Errorf("pending finding is still cause-free: %q", pending.Message)
	}
}
```

- [ ] **Step 2: Run it to verify it fails for the right reason**

Run: `go test ./internal/app/ -run TestCloseoutBacklinkPendingFindingNamesTheCause -count=1`
Expected: FAIL at the `strings.Contains(pending.Message, f.planPath)` assert — the finding exists (the scoped loader refuses on the in-scope parse failure, proving Task 2's gate still fires where it must) but the message carries only the coarse token. If the pending finding itself is absent, stop: the scoped loader is not refusing on a targeted-artifact parse failure — that is a Task 1/2 defect, not a message problem.

- [ ] **Step 3: Carry the detail in both legs' messages and rewrite the stale comments**

In `internal/app/finalize_closeout.go`, `runCloseoutBacklinkLeg`, replace the block from the comment above `result, _ := mapOutcome(...)` through the final `return &StatusFinding{...}` with:

```go
	// Best-effort secondary leg: the change stays truthfully done and the sweep
	// retries. The finding carries the transaction's typed cause — the failure's
	// stage/kind/detail, or a refusal's finding codes and paths — so a stuck leg
	// is self-diagnosing (change 0337); after the scoped loader, an in-scope
	// artifact-level problem is the only refusal left, and this names it.
	result, _ := mapOutcome(res, execErr, ResultInvalidState)
	if result == ResultApplied || result == ResultNoOp {
		return nil
	}
	msg := fmt.Sprintf("the change is done, but the integration-ref backlink leg did not land (%s)", result)
	if d := backlinkLegDetail(res, execErr); d != "" {
		msg += ": " + d
	}
	msg += "; the sweep will retry it"
	return &StatusFinding{
		Code:     ReasonCloseoutBacklinkPending,
		Severity: string(domain.SeverityWarning),
		Message:  msg,
	}
```

In `internal/app/finalize_cleanup.go`, `finalizeCleanupBacklinkRepair`, replace the corresponding block (comment, `result, _ := mapOutcome(...)`, early return, and the `cleanupWarning(...)` return) with:

```go
	// Best-effort secondary leg: the change stays truthfully done and the sweep
	// retries. The finding carries the transaction's typed cause — the failure's
	// stage/kind/detail, or a refusal's finding codes and paths — so a stuck leg
	// is self-diagnosing (change 0337); after the scoped loader, an in-scope
	// artifact-level problem is the only refusal left, and this names it.
	result, _ := mapOutcome(res, execErr, ResultInvalidState)
	if result == ResultApplied || result == ResultNoOp {
		return nil
	}
	msg := "the integration-ref backlink leg did not land (" + string(result) + ")"
	if d := backlinkLegDetail(res, execErr); d != "" {
		msg += ": " + d
	}
	msg += "; the sweep will retry it"
	f := cleanupWarning(ReasonCleanupBacklinkPending, msg)
	return &f
```

Both old comments claiming "it surfaces only the coarse result token … the coarse token in the warning is enough" are now false and must go — the replacement comment above is their successor. Do not leave either old comment behind.

- [ ] **Step 4: Run the diagnosability test and both legs' suites**

Run: `go test ./internal/app/ -run 'TestCloseoutBacklink|TestCleanupBacklinkRepair|TestFinalizeCleanup' -count=1`
Expected: PASS — including Tasks 2/3's regressions (a landed leg emits no finding, so the message change cannot affect them).

- [ ] **Step 5: Mutation-probe the new message path**

Temporarily change `backlinkLegDetail` in `internal/app/finalize_backlink_loader.go` to `return ""` on its refusal branch, run:
`go test ./internal/app/ -run TestCloseoutBacklinkPendingFindingNamesTheCause -count=1`
Expected: FAIL (the finding loses the path). Restore the branch **by reverting the edit you just made** (do NOT `git checkout -- <file>` — that restores to HEAD and would be fine here only because the file is committed; re-apply from your editor buffer to be safe), re-run, expect PASS. A probe that stays green is a defect to investigate before proceeding.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finalize_closeout.go internal/app/finalize_cleanup.go internal/app/finalize_closeout_test.go
git commit -m "fix(0337): backlink-pending findings carry the typed transaction cause"
```

---

### Task 5: Full-suite gate

**Files:**
- None modified — verification only (fix forward anything red, committing per fix).

**Interfaces:**
- Consumes: everything above.
- Produces: the build-evidence green run the review role reads.

- [ ] **Step 1: Run the Go package tests fresh**

Run: `go test ./... -count=1`
Expected: PASS across all packages.

- [ ] **Step 2: Run the repository suite**

Run: `scripts/run-tests.sh` (this is what `finalize.test_command` resolves to — the whole suite, never only the tests this plan names).
Expected: PASS. Treat any trailing `OVER BUDGET:` line as a finding to act on — it does not fail the run and nothing else will catch it.

- [ ] **Step 3: Confirm the untouched proofs stayed untouched**

Run: `git diff HEAD~4 --stat -- internal/repository/transaction/ docs/adrs/`
Expected: empty output — no engine change, no ADR touched (Global Constraints). If anything shows, revert it before proceeding.

- [ ] **Step 4: Commit (only if fixes were needed)**

Any fix made here gets its own commit with a message naming what was red and why.

---

## Self-Review (performed at plan time)

- **Spec coverage:** A → Tasks 1–3 (scoped loader, both legs). D → Tasks 1 + 4 (renderer, both legs' messages, diagnosability test). Testing section: regression → Task 2 Step 2 proves red-before/green-after; sweep self-heal → Task 3; diagnosability → Task 4; idempotency preserved → existing tests exercised in Tasks 2/4 and the full suite in Task 5. C → deliberately absent (Global Constraints). "No new config knob" → no config file touched anywhere.
- **Placeholder scan:** none — every step carries the exact code or command.
- **Type consistency:** `newBacklinkArtifactLoader([]closeoutBacklinkTarget) transaction.StateLoader` and `backlinkLegDetail(transaction.Result, error) string` are spelled identically in Tasks 1, 2, 3, and 4; both legs already hold `backlinkTargets []closeoutBacklinkTarget` at the swap site.
- **Known judgment call, recorded:** an absent artifact or missing block stays a benign skip (spec constraint), so the loader refuses only on a targeted artifact that is present but unparseable — block presence remains the operation's own skip logic, unchanged.
