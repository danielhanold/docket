<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0390 — maintenance sweep --scope full re-probes the remote per item, hanging the sweep](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-01-0390-maintenance-sweep-scope-full-re-probes-the-remote-per-item-h.md)**
<!-- docket:backlink:end -->
# Maintenance Sweep Batched Discovery, Shared Observations, and Network Deadlines — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `docket maintenance sweep --scope full` stop repeating repository setup and remote probes per item: batch active PR discovery, share one remote-heads inventory, assess non-actionable historical records from local snapshots, fetch metadata exactly once per dispatched operation attempt, and bound every sweep remote read to 30s and remote write to 60s.

**Architecture:** Three layers change. (1) The adapters (`internal/gitcli`, `internal/githubcli`) gain a read/write network-budget split, a strict complete remote-heads advertisement, and a GraphQL exact-number PR batch read. (2) The app layer (`internal/app`) gains a sweep session that captures the initial pin's setup and prepares one immutable metadata observation per dispatched operation attempt (a bound `StatusReader` that never re-fetches), a batched PR-facts selector, and a read-only snapshot assessment for historical `done`/`stacked-merged` records that resolves them without individual remote calls. (3) The CLI wires a sweep-only dependency builder whose Git/GitHub clients carry the 30s/60s policies while every standalone command keeps the 5-minute defaults.

**Tech Stack:** Go; `gh api graphql` for the PR batch; `git ls-remote --heads` for the shared heads inventory; existing gitcli/githubcli exec harness (`WithExecutable`) for fake-transport tests.

**Spec:** `docs/superpowers/specs/2026-08-31-maintenance-sweep-scope-full-re-probes-the-remote-per-item-h-design.md` (on the `docket` metadata branch; a synchronized copy is at `.docket/docs/superpowers/specs/…` in the primary tree). The spec is prescriptive — when a task summary and the spec disagree, the spec wins.

## Global Constraints

- Source revision the spec targets: `c95e5189febeecffa2487d3303069bce2b07b92f` (this branch's base).
- Sweep-only budgets: **remote reads 30 seconds, remote writes 60 seconds** — constructor policy on the sweep's clients only; package defaults (`defaultNetworkTimeout = 5 * time.Minute`, `defaultLocalTimeout = 30 * time.Second`) and every standalone command's policy are unchanged.
- A budget belongs to one top-level adapter operation *including* its internal failure-diagnosis or reconciliation probe (the `FetchBranch` shared-`netCtx` pattern in `internal/gitcli/refs.go` is the reference); never start a fresh clock for a diagnostic.
- A timed-out remote **write** is an uncertain result — never assume it did not happen; preserve the existing reconciliation/unknown outcomes.
- PR batches: at most **25 unique exact numbers** per GraphQL request, deterministic order, no per-record fallback on a failed batch, no head-based search, no list-all.
- Shared observations justify doing nothing, **never** a mutation. Every dispatched operation keeps its fresh metadata preparation and all existing merge/ownership/reachability/lease/exact-version proofs.
- One metadata fetch (`refs/heads/docket`) per dispatched operation attempt; zero repeated setup probes (default-branch discovery, default/integration fetches, `gatherRepoFacts` metadata `ls-remote`) and zero nested-reader re-fetches after the initial pin.
- Probe error ≠ clean absence, everywhere (learning `probe-error-is-not-clean-absence`): a failed shared inventory is *unknown*, a missing/foreign workspace manifest is never "clean", a null GraphQL alias is *unknown* never closed.
- Mutation-test every new guard (AGENTS.md "Guards and tests"); defeat Go's test cache with `-count=1` on every mutation probe (learning `cached-runner-serves-a-mutated-tree`).
- No wall-clock equality asserts calibrated on one machine; no test sleeps 30/60s (learning `tolerance-constant-calibrated-on-one-machine`).
- Tests must not call a live remote; use local origins and `WithExecutable` fakes.
- Build gate: `go run ./cmd/docket development test` (the resolved `finalize.test_command`); handle budget clause lines per `tests/README.md`.
- Comment cross-references anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054).

---

### Task 1: gitcli read/write network-budget split

**Files:**
- Modify: `internal/gitcli/client.go` (options, config, accessors)
- Modify: `internal/gitcli/exec.go` (`runRequest` budget selection)
- Modify: `internal/gitcli/push.go`, `internal/gitcli/refdelete.go` (mark write sites)
- Modify: `internal/gitcli/refs.go` (`FetchBranch` shared budget uses the read budget)
- Test: `internal/gitcli/client_test.go`, `internal/gitcli/exec_test.go`

**Interfaces:**
- Produces: `gitcli.WithNetworkReadTimeout(d time.Duration) Option`, `gitcli.WithNetworkWriteTimeout(d time.Duration) Option`, `(*gitcli.Client).NetworkReadTimeout() time.Duration`, `(*gitcli.Client).NetworkWriteTimeout() time.Duration`. `runRequest` gains `write bool` (meaningful only with `network: true`).

- [ ] **Step 1: Enumerate the network sites mechanically.** Per AGENTS.md ("Never hand-list the sites of a literal or an operation you are gating"), run `grep -rn "network: *true" internal/gitcli/ --include='*.go' | grep -v _test` and record the list in the test file header comment. Classify: `push.go` (`PushLease`, `PushCreateLease`) and `refdelete.go` (`DeleteRemoteRefLease`) are **writes**; every other site (fetch, ls-remote, discovery probes, `classifyFetchFailure`) is a **read**.

- [ ] **Step 2: Write the failing tests.**

```go
func TestNetworkReadWriteTimeoutOptions(t *testing.T) {
	c, err := NewClient(WithNetworkReadTimeout(30*time.Second), WithNetworkWriteTimeout(60*time.Second))
	if err != nil { t.Fatal(err) }
	if got := c.NetworkReadTimeout(); got != 30*time.Second { t.Fatalf("read timeout = %v", got) }
	if got := c.NetworkWriteTimeout(); got != 60*time.Second { t.Fatalf("write timeout = %v", got) }
}

func TestNetworkTimeoutDefaultsInheritBase(t *testing.T) {
	// Without the new options both budgets are the existing 5m network default,
	// so every standalone client is behaviorally unchanged.
	c, err := NewClient()
	if err != nil { t.Fatal(err) }
	if c.NetworkReadTimeout() != defaultNetworkTimeout || c.NetworkWriteTimeout() != defaultNetworkTimeout {
		t.Fatalf("defaults changed: read=%v write=%v", c.NetworkReadTimeout(), c.NetworkWriteTimeout())
	}
}

func TestNonPositiveReadWriteTimeoutRejected(t *testing.T) {
	if _, err := NewClient(WithNetworkReadTimeout(0)); err == nil { t.Fatal("zero read timeout accepted") }
	if _, err := NewClient(WithNetworkWriteTimeout(-time.Second)); err == nil { t.Fatal("negative write timeout accepted") }
}
```

Plus an exec-level selection test (in `exec_test.go`, following that file's existing fake-executable pattern): a `runRequest{network: true, write: true}` against a sleeping fake git must fail at the write budget when the write budget is short and the read budget long, and vice versa — inject e.g. `WithNetworkReadTimeout(50*time.Millisecond)`, `WithNetworkWriteTimeout(5*time.Second)`, assert the read request times out and the write request survives a 200ms sleep. No wall-clock equality; assert only which one timed out.

- [ ] **Step 3: Run tests to verify they fail** (`go test ./internal/gitcli/ -run 'TestNetwork.*Timeout' -count=1`). Expected: compile failure (options undefined).

- [ ] **Step 4: Implement.** In `client.go`: add `networkReadTimeout, networkWriteTimeout time.Duration` to `clientConfig` and `Client` (zero in config means "inherit `networkTimeout`"; resolve at `NewClient` after validating `> 0` when explicitly set, mirroring the existing non-positive checks). Add the two options and the two accessors. In `exec.go` timeout selection (`if req.network { timeout = c.networkTimeout }`), select `c.networkWriteTimeout` when `req.network && req.write`, else `c.networkReadTimeout` when `req.network`. Mark `write: true` on the three write sites from Step 1. In `refs.go` `FetchBranch`, change the shared `netCtx` construction to use `c.networkReadTimeout` (quote its existing comment "two network processes serving one operation" — the pair still shares one budget, now the read budget).

- [ ] **Step 5: Run the package tests** (`go test ./internal/gitcli/ -count=1`), verify green, then mutation-test: flip one write site to `write: false` and confirm the exec selection test reddens (`-count=1`); restore.

- [ ] **Step 6: Commit** (`git add internal/gitcli && git commit -m "feat(gitcli): split network budget into read/write timeouts"`).

---

### Task 2: githubcli read/write network-budget split

**Files:**
- Modify: `internal/githubcli/client.go` (options, config, accessors, `runRequest.write`, selection)
- Modify: write sites in `internal/githubcli/merge.go`, `internal/githubcli/ensure.go`, `internal/githubcli/retarget.go`, `internal/githubcli/comment.go`
- Test: `internal/githubcli/client_test.go`

**Interfaces:**
- Produces: `githubcli.WithNetworkReadTimeout(d) Option`, `githubcli.WithNetworkWriteTimeout(d) Option`, `(*githubcli.Client).NetworkReadTimeout()`, `(*githubcli.Client).NetworkWriteTimeout()`.

- [ ] **Step 1: Enumerate sites** with `grep -rn "network: *true" internal/githubcli/ --include='*.go' | grep -v _test`. Writes are the gh invocations that mutate GitHub state: the merge invocation in `MergePullRequest`, the create/edit mutations in `EnsurePullRequest`/`mutateAndVerify`, the retarget edit in `RetargetPullRequest`, and the comment post in `EnsureComment`. Their **verification/reprobe reads** (`verifyMerge`, `verifyPostMutation`, `viewPullRequest`, `FindComment`) are reads. Record the derived classification in the test header comment.

- [ ] **Step 2: Write failing tests** mirroring Task 1 (options resolve, defaults inherit `defaultNetworkTimeout`, non-positive rejected, and a fake-`gh` selection test using `WithExecutable` + a sleeping script, asserting a short read budget times out a read op while a long write budget lets a write op complete).

- [ ] **Step 3: Run to verify failure**, **Step 4: implement** exactly as Task 1 (the selection lives in `(*Client).run`, "selects the network vs local default timeout"), **Step 5: package tests + one mutation probe** (unmark one write site → selection test reddens; restore).

- [ ] **Step 6: Commit** (`git commit -m "feat(githubcli): split network budget into read/write timeouts"`).

---

### Task 3: gitcli.ListRemoteHeads — one complete remote-heads advertisement

**Files:**
- Create: `internal/gitcli/remoteheads.go`
- Test: `internal/gitcli/remoteheads_integration_test.go` (local-origin fixture, same pattern as `remoteref_integration_test.go`)

**Interfaces:**
- Produces: `func (c *Client) ListRemoteHeads(ctx context.Context, repo Repository, remote RemoteName) (map[RefName]ObjectID, error)` — network **read**; success with an empty advertisement returns an empty non-nil map.

- [ ] **Step 1: Write failing tests** against a local origin fixture:

```go
func TestListRemoteHeadsCompleteAdvertisement(t *testing.T) {
	// fixture: origin with branches main, docket, feature/x → map has exactly
	// refs/heads/main, refs/heads/docket, refs/heads/feature/x with valid OIDs.
}
func TestListRemoteHeadsEmptyOriginIsEmptyMapNotError(t *testing.T) { /* bare origin, no refs */ }
func TestListRemoteHeadsMalformedLineIsFailureNotPartial(t *testing.T) {
	// WithExecutable fake git printing a bad OID line → typed *Failure
	// (KindInvalidOutput), never a partial map.
}
func TestListRemoteHeadsDuplicateRefIsFailure(t *testing.T) { /* fake prints one ref twice */ }
func TestListRemoteHeadsTransportFailureIsError(t *testing.T) { /* fake exits 128 → KindCommandFailed */ }
```

- [ ] **Step 2: Run to verify failure.**

- [ ] **Step 3: Implement.** `git ls-remote --heads <remote>` via `c.run(... network: true)` (read budget). Parse each line as `<oid>\t<ref>`: validate the OID with the package's object-id validation, require the `refs/heads/` prefix and a valid ref name, refuse duplicates. Any malformed or duplicate line, or a non-zero exit, is a typed `*Failure` — the caller must never infer absence from a failed or partial read. Doc comment must state, verbatim from the spec's intent: a failed shared inventory is *unknown, not permission to fan out into individual inventory probes*.

- [ ] **Step 4: Green + mutation-test** (make the parser skip a malformed line instead of failing → duplicate/malformed tests must redden; restore). **Step 5: Commit** (`git commit -m "feat(gitcli): ListRemoteHeads strict complete advertisement"`).

---

### Task 4: githubcli.ViewPullRequestsBatch — aliased GraphQL exact-number batch

**Files:**
- Create: `internal/githubcli/prbatch.go`
- Test: `internal/githubcli/prbatch_test.go` (decode/construction units), `internal/githubcli/prbatch_integration_test.go` (fake `gh` transport)

**Interfaces:**
- Produces:

```go
// BatchPRResult is one exact-number slot of a batch read. Found=false is
// UNKNOWN (missing/null alias, wrong number, malformed required field) — never
// a closed/absent verdict.
type BatchPRResult struct {
	Found       bool
	PR          PullRequest // normalized exactly as ViewPullRequest normalizes
	MergedAtUTC string      // merged PRs only
	MergeCommit string      // merged PRs only
}
func (c *Client) ViewPullRequestsBatch(ctx context.Context, repo Repository, numbers []int) (map[int]BatchPRResult, error)
```

- [ ] **Step 1: Write failing unit tests** for query construction and decode:

```go
func TestBatchQueryAliasesOnePerNumber(t *testing.T)      // 3 numbers → pr0/pr1/pr2 aliases, deterministic order
func TestBatchRejectsEmptyOversizedDuplicateInvalid(t *testing.T) // 0, 26, dup, non-positive → KindInvalidInput
func TestBatchDecodeMergedOpenClosedDraftApproved(t *testing.T)   // per-alias normalization parity
func TestBatchNullAliasIsUnknownNeverClosed(t *testing.T)
func TestBatchWrongNumberInAliasIsUnknown(t *testing.T)
func TestBatchMalformedFieldIsUnknownForThatPROnly(t *testing.T)
func TestBatchGraphQLErrorsFailWholeBatchEvenHTTP200(t *testing.T)
func TestBatchVersionMatchesSingleViewFixture(t *testing.T)       // computeVersion parity with ViewPullRequest
```

- [ ] **Step 2: Run to verify failure.**

- [ ] **Step 3: Implement.** Validate: 1–25 numbers, all positive, all unique (`KindInvalidInput` otherwise; the 25 cap is *this adapter's conservative batch size, not a GitHub limit* — say so in the doc comment). Build one query with aliased exact-number fields:

```graphql
query($owner:String!,$name:String!){ repository(owner:$owner,name:$name){
  pr0: pullRequest(number: 101){ number url state isDraft reviewDecision
    headRefName headRefOid baseRefName title body mergedAt mergeCommit { oid } }
  pr1: pullRequest(number: 205){ ... } } }
```

Run `gh api graphql -f query=… -F owner=… -F name=…` through `c.run` (`network: true`, read budget). Decode the envelope strictly: a transport failure, non-zero exit, malformed envelope, **or a non-empty top-level `errors` array (even with data present)** fails the whole batch as a typed `*Failure` — never shrink to per-record retries. Per alias: `null` → `Found: false`; a decoded PR whose `number` ≠ the requested number → `Found: false`; reuse `normalizeState`, `normalizeReviewDecision`, `validateFullObjectID`, and `computeVersion` so `PR.Version` is byte-identical to a `ViewPullRequest` of the same snapshot — do **not** write a second normalization (a malformed required field makes that one PR unknown, not the batch). For `state == MERGED`, populate `MergedAtUTC` from `mergedAt` and `MergeCommit` from `mergeCommit.oid`. No pagination, no nested connections.

- [ ] **Step 4: Integration test with a fake `gh`** (`WithExecutable`): script records argv + stdin to a file and prints a canned response. Assert **one process invocation** for a 25-number batch (request-count truth at the transport boundary, not a batch-method fake), and cover: a failed batch between two successful batches leaves the successes usable (drive via the app helper in Task 5 or two direct calls), deleted-head-ref merged PR, partial response with GraphQL errors.

- [ ] **Step 5: Green + mutation-test** (make a null alias decode as `closed` → `TestBatchNullAliasIsUnknownNeverClosed` reddens; make `errors` tolerated → whole-batch test reddens; restore). **Step 6: Commit** (`git commit -m "feat(githubcli): ViewPullRequestsBatch aliased exact-number GraphQL read"`).

---

### Task 5: app — batched PR selection and one shared GitHub identity

**Files:**
- Create: `internal/app/sweep_prfacts.go`
- Modify: `internal/app/finalize_context.go` (add `FinalizeDeps.PRBatch` field), `internal/app/maintenance.go` (replace the per-change `probeFinalizeFacts` loop)
- Modify: `internal/cli/finalize.go` (`newFinalizeDeps` wires the production batch reader)
- Test: `internal/app/sweep_prfacts_test.go`, extend `internal/app/maintenance_test.go`

**Interfaces:**
- Produces:

```go
// SweepPRBatchReader reads exact-number PR facts in deterministic batches for
// the maintenance sweep. A failed batch reports its numbers in Failures and
// omits them from Facts — unknown, never absent/closed.
type SweepPRBatchReader interface {
	ProbePRSet(ctx context.Context, repoDir string, numbers []int) SweepPRSetResult
}
type SweepPRSetResult struct {
	Facts    map[int]domain.PRFacts
	Failures []SweepPRBatchFailure // one per failed batch (incl. identity resolution: one failure covering all numbers)
}
type SweepPRBatchFailure struct {
	Numbers []int
	Message string
}
func NewSweepPRBatchReader(gh *githubcli.Client) SweepPRBatchReader
```

- Consumes: `githubcli.ViewPullRequestsBatch` (Task 4), `parsePRNumber`, `finalizeHasPRRef`, the `domain.PRFacts` field population of `githubFinalizeProber.ProbePR` (merged and non-merged shapes).

- [ ] **Step 1: Write failing tests.**

```go
func TestSweepPRSetBatchesOf25(t *testing.T)          // 0→0 calls, 1→1, 25→1, 26→2, 51→3 (fake reader counting)
func TestSweepPRSetDedupesAndSorts(t *testing.T)       // duplicate refs share one response slot
func TestSweepPRSetFailedBatchIsUnknownPlusFinding(t *testing.T) // middle batch fails; others usable; no per-PR fallback
func TestSweepPRSetIdentityResolvedOnce(t *testing.T)  // counting fake gh: exactly one DiscoverRepository; a failed
                                                       // resolution fails once for the invocation, no per-consumer retry
func TestSweepPRSetFactsParity(t *testing.T)           // merged/open/closed/draft/approved PRFacts match the existing
                                                       // ProbePR shapes incl. conservative empty Mergeable/diff for open
```

And in `maintenance_test.go`: `TestSweepSelectionUsesBatchedFactsNotPerChangeProbe` — orchestration-level, injecting the seam below, asserting `probeFinalizeFacts`/`ProbePR` is never called per change.

- [ ] **Step 2: Run to verify failure.**

- [ ] **Step 3: Implement `sweep_prfacts.go`.** Production reader: resolve `gh.DiscoverRepository(ctx, repoDir)` **once** (memoize failure too — one failure entry covering all numbers, no retry per consumer); dedupe + sort numbers ascending; chunk into ≤25; one `ViewPullRequestsBatch` per chunk. Map each `BatchPRResult` to `domain.PRFacts`: merged → `{Number, Version, State: "merged", HeadBranch, HeadOID, BaseRef, MergedAtUTC, MergeCommit}`; open/closed → `{Number, Version, State, Draft, Approved, HeadBranch, HeadOID, BaseRef}` (Mergeable/ChangedFiles/DiffLines stay zero — quote `githubFinalizeProber`'s "conservative reading" comment). `Found: false` → omitted (unknown). Add an app-level selection helper:

```go
// sweepSelectPRFacts derives the facts map for domain.SelectFinalizeQueue from
// batched exact-number reads. Changes whose numbers fall in a failed batch (or
// carry an unparseable ref) get zero-value PRFacts — unknown, exactly what the
// old per-change probe error produced — plus one StatusFinding per failed batch.
func sweepSelectPRFacts(ctx context.Context, batch SweepPRBatchReader, repoDir string, snap domain.Snapshot) (map[domain.ChangeID]domain.PRFacts, []StatusFinding)
```

Population predicate is copied whole from the existing loop (`c.Status().Terminal() || !finalizeHasPRRef(c)` — learning `duplicated-gate-copies-the-whole-predicate`: keep it one shared predicate function if extraction is cheap).

- [ ] **Step 4: Rewire `maintenanceSweep`.** Add a seam field `probeFacts func(ctx context.Context, snap domain.Snapshot) (map[domain.ChangeID]domain.PRFacts, []StatusFinding)` to `sweepOps`; production (`MaintenanceSweep`) wires it to `sweepSelectPRFacts` over `deps.PRBatch`; delete the per-change `probeFinalizeFacts` loop from `maintenanceSweep`; append the returned findings to `MaintenanceResult.Findings` (discovery diagnostics are emitted even when no operation is selected — *silent omission is not success*). `newFinalizeDeps` wires `PRBatch: app.NewSweepPRBatchReader(ghClient)`. Standalone `ProbePR` and every other `FinalizeDeps` consumer are untouched.

- [ ] **Step 5: Green (`go test ./internal/app/ ./internal/githubcli/ -count=1`) + mutation-test** (reintroduce a per-PR `ViewPullRequest` loop inside the production reader → the transport-count integration assert from Task 4/Task 10 reddens; make a failed batch fall back to singles → `TestSweepPRSetFailedBatchIsUnknownPlusFinding` reddens; restore). **Step 6: Commit** (`git commit -m "feat(app): batched exact-number PR selection for maintenance sweep"`).

---

### Task 6: app — sweep session, one observation per attempt, bound reader

**Files:**
- Create: `internal/app/sweep_session.go`
- Test: `internal/app/sweep_session_test.go`

**Interfaces:**
- Produces:

```go
// sweepObservation is one immutable metadata observation: the captured setup
// combined with exactly one fresh metadata fetch, plus the corpus/snapshot/blob
// versions read at that revision. It serves ONE dispatched operation attempt
// and is discarded after the operation returns.
type sweepObservation struct {
	pin StatusPin       // captured setup + this attempt's fresh MetadataRevision
	inv sweepInventory  // snapshot + versionByPath at that revision
	blobs []StatusBlob  // the exact corpus bytes, for the bound reader
}

// sweepPreparer prepares one observation per operation attempt.
type sweepPreparer interface {
	Prepare(ctx context.Context) (*sweepObservation, error)
}

// newSweepSession derives the production preparer from the sweep's initial pin.
// The session is bound to the discovered repository and the invocation's
// captured configuration; it never reruns default-branch discovery, setup
// fetches, or the topology probe.
func newSweepSession(client *gitcli.Client, repo gitcli.Repository, base StatusPin) *sweepSession

// boundStatusReader serves a prepared observation: PinContext returns the
// supplied pin WITHOUT fetching; ReadCorpus returns the same immutable corpus
// WITHOUT rereading the remote. Artifact and branch reads delegate to the live
// reader (operation-specific proofs stay live).
func newBoundStatusReader(obs *sweepObservation, live StatusReader) StatusReader
```

- Consumes: `StatusPin`, `sweepInventory`, `sweepBuildSnapshot`, `gitcli.Client.FetchBranch`, `gitStatusReader` (same package).

- [ ] **Step 1: Write failing tests** (fixture: local origin repo with a docket metadata branch — reuse the fixture builders in `internal/app/finalize_e2e_test.go` / `claim_workflow_git_test.go`):

```go
func TestPrepareIsOneMetadataFetchZeroSetupProbes(t *testing.T) {
	// counting fake git executable: Prepare() runs exactly one `fetch … refs/heads/docket`
	// and zero `ls-remote`, zero default/integration fetches.
}
func TestPrepareObservesFreshMetadataTip(t *testing.T) {
	// advance origin's docket branch between two Prepare() calls from an
	// independent writer; second observation carries the new revision and blob
	// versions — a supplied tip from a previous attempt never stands in.
}
func TestPrepareFailedFetchIsErrorNeverStaleFallback(t *testing.T) {
	// delete the remote docket branch → Prepare returns the classified error;
	// no prior revision is reused, no absence is inferred.
}
func TestBoundReaderNeverFetches(t *testing.T) {
	// counting fake git: PinContext + ReadCorpus on the bound reader run zero
	// network git processes and return the observation's pin/corpus verbatim.
}
func TestSessionRefusesDifferentRepository(t *testing.T) {
	// the session is bound to its original repository; a repoDir naming another
	// repo must error, never silently reuse captured facts.
}
```

- [ ] **Step 2: Run to verify failure.**

- [ ] **Step 3: Implement.** `sweepSession.Prepare`: one `fetchPinnedRevision(ctx, client, repo, refs/heads/docket)`; build `pin` by copying the captured base `StatusPin` (default/integration branches+revisions, `Config`, `ConfigDiags`, `RepoWebURL` — *the initial pin's configuration is used consistently throughout the invocation*) with only `MetadataRevision` replaced; read the corpus **once** at that revision through a package-local read (reuse `sweepBuildSnapshot` against a reader bound to the new pin — implement corpus reading via `gitStatusReader` mechanics with the session's client+repo, not a second `PinContext`). `boundStatusReader`: `PinContext` returns `obs.pin` (and must not touch the repoDir argument beyond the same-repository guard), `ReadCorpus` returns `obs.blobs`, `BranchFacts`/`ArtifactExists`/`ReadArtifact` delegate to `live`. Preserve `FetchBranch`'s bounded failure-classification behavior (its diagnostic probe rides inside the same call); add **no** extra pre-fetch probe, no cache, no TTL.

- [ ] **Step 4: Green + mutation-test** (make `Prepare` also refetch the integration branch → the counting test reddens; make the bound `ReadCorpus` call the live reader → `TestBoundReaderNeverFetches` reddens; restore). **Step 5: Commit** (`git commit -m "feat(app): sweep session prepares one metadata observation per attempt"`).

---

### Task 7: app — thread the observation through every dispatched operation

**Files:**
- Modify: `internal/app/maintenance.go` (`sweepOps` signatures, `MaintenanceSweep` closures, `sweepRunCloseout`/`sweepRunCleanup`/`sweepRunReclaim`, retire `sweepReloadPresent`/`sweepReloadVersion` re-pinning)
- Test: `internal/app/maintenance_test.go` (update fakes; new asserts)

**Interfaces:**
- Consumes: `sweepPreparer`, `sweepObservation`, `newBoundStatusReader` (Task 6).
- Produces (new `sweepOps` shape — every unit-test fake updates to it):

```go
type sweepOps struct {
	prepare    func(ctx context.Context) (*sweepObservation, error)
	probeFacts func(ctx context.Context, snap domain.Snapshot) (map[domain.ChangeID]domain.PRFacts, []StatusFinding)
	closeout   func(ctx context.Context, id int, obs *sweepObservation) CloseoutResult
	cleanup    func(ctx context.Context, id int, obs *sweepObservation) CleanupOpResult
	reclaim    func(ctx context.Context, id int, version string, obs *sweepObservation) ChangeReclaimResult
}
```

- [ ] **Step 1: Write failing orchestration tests** (fakes record which observation each op received):

```go
func TestEachAttemptPreparesOnceAndSharesObservation(t *testing.T) {
	// one closeout item: exactly one prepare() for the closeout attempt; the
	// presence check and the dispatched op observe the SAME *sweepObservation.
}
func TestCleanupSuffixPreparesAgain(t *testing.T) {
	// successful closeout + suffix: two prepare() calls, distinct observations —
	// closeout may have changed paths/statuses; never share across operations.
}
func TestDisabledReclaimPreparesNothing(t *testing.T)     // reclaim.auto=false: zero prepare()
func TestPrepareFailureIsReloadFailedSkipNoDispatch(t *testing.T) // existing ReasonSweepReloadFailed entry, op not called
func TestVanishedOnObservationSkipsAfterThatPrepare(t *testing.T) // fetch succeeded, record absent → item-vanished; no retry fetch
func TestReclaimVersionFromObservation(t *testing.T)      // version passed to reclaim == obs.inv.versionByPath[path]
}
```

- [ ] **Step 2: Run to verify failure** (compile errors in fakes are the expected first failure; update fakes as part of this task).

- [ ] **Step 3: Implement.** In `maintenanceSweep`: replace each `sweepReloadVersion`/`sweepReloadPresent` call with `obs, err := ops.prepare(ctx)`; on error → existing `ReasonSweepReloadFailed` skip entry; then presence/ambiguity and blob version come from `obs.inv` (keep the *"absent or ambiguous on reload"* vanished skip verbatim — a successful fetch with a missing record is vanished, never a fetch failure). Pass `obs` to the op. In `MaintenanceSweep` (production wiring): build the session — `deps.Planning.Reader` must be the production `*gitStatusReader`; after the initial `PinContext` in `maintenanceSweep`… the pin happens inside `maintenanceSweep`, so restructure: `maintenanceSweep` performs the initial pin, then calls a new `ops.bind(pin)`-style hook, OR (simpler, do this) `MaintenanceSweep` builds closures that lazily construct the session on first use from `deps.Planning.Client`, the reader's discovered repository, and the initial pin threaded via the `prepare` closure — concretely: `maintenanceSweep` gains `ops.prepare = mkPrepare(pin)` by making `sweepOps.prepare` a factory field `prepareWith func(pin StatusPin) sweepPreparer` that `maintenanceSweep` invokes once after the initial pin succeeds. Keep the injected `sweepOps` test seam working with a trivial fake preparer. Each production closure builds a **bound deps copy** for its attempt:

```go
depsFor := func(obs *sweepObservation) FinalizeDeps {
	d := deps
	p := d.Planning
	p.Reader = newBoundStatusReader(obs, deps.Planning.Reader)
	d.Planning = p
	return d
}
// closeout: FinalizeCloseout(ctx, depsFor(obs), repoDir, id, CloseoutNotes{})
// cleanup:  FinalizeCleanup(ctx, depsFor(obs), repoDir, id)
// reclaim:  ChangeReclaim(ctx, depsFor(obs).Planning, wdeps, repoDir, …)
```

This also binds reclaim's nested `WorkspaceInspect` (it reads through `deps.Planning.Reader`) — the nested reader performs **zero additional metadata fetches**. Do not touch the operations' own GitHub/merge/ownership/branch-absence/workspace/transaction proofs or their direct integration-branch fetches (they must observe live remote state); the transaction engine's base fetch and push reconciliation are *additional intentional calls*, not reader reloads. Update `MaintenanceEntry`'s doc comment and the file-header comment: the fresh-authority contract is now *one metadata fetch for the whole operation attempt and zero repeated setup probes or nested-reader fetches* — quote that clause.

- [ ] **Step 4: Green** (`go test ./internal/app/ -run TestSweep -count=1`, then the whole package). **Step 5: Mutation-test** (bypass the session in `sweepRunCloseout` by calling the live reader's `PinContext` → `TestEachAttemptPreparesOnceAndSharesObservation` reddens; share one observation across closeout+suffix → `TestCleanupSuffixPreparesAgain` reddens; restore, `-count=1` throughout). **Step 6: Commit** (`git commit -m "feat(app): one shared metadata observation per sweep operation attempt"`).

---

### Task 8: app — snapshot assessment of historical records (full scope)

**Files:**
- Create: `internal/app/maintenance_assess.go`
- Modify: `internal/app/maintenance.go` (full-scope worklist: assess first, enqueue only actionable; new reason constants; `MaintenanceEntry` contract comment)
- Modify: `internal/app/finalize_cleanup.go` / `internal/app/finalize_closeout.go` — extract (do not duplicate) the per-target "would the terminal backlink block's bytes change" computation used by `closeoutBacklinkTargets` and `cleanupBacklinkOp.Plan` into a helper both the cleanup transaction and the assessment call, e.g. `func backlinkLegHasWork(doc []byte, renderedInterior string) (bool, error)` returning an error for malformed/unbalanced markers (learning `restatement-accumulates-its-own-guards`: move, don't copy).
- Test: `internal/app/maintenance_assess_test.go`

**Interfaces:**
- Produces:

```go
// New stable skip/assessment reasons (closed vocabulary additions):
const (
	ReasonSweepSnapshotNoWork   = "snapshot-no-work"
	ReasonSweepSnapshotRetained = "snapshot-retained"
	ReasonSweepSnapshotBlocked  = "snapshot-blocked"
	ReasonSweepSnapshotUnknown  = "snapshot-unknown"
	ReasonSweepSnapshotInvalid  = "snapshot-invalid"
)

// sweepSharedFacts are the invocation-shared read-only observations. Each leg
// records failure separately; a failed input yields UNKNOWN assessments for the
// legs that depend on it, never a fan-out into individual probes.
type sweepSharedFacts struct {
	remoteHeads    map[gitcli.RefName]gitcli.ObjectID
	remoteHeadsErr error // set ⇒ remote-ref absence is unprovable this invocation
	worktrees      []gitcli.WorktreeInfo
	worktreesErr   error
}

// sweepAssessHistorical resolves every full-scope done/stacked-merged candidate
// from shared snapshots and local inspection. It returns the non-actionable
// entries (Disposition skipped/blocked/unknown, EMPTY Operation — a
// pre-dispatch observation, never a fabricated cleanup result) and the ids that
// warrant one normal fresh cleanup attempt.
func sweepAssessHistorical(ctx context.Context, deps FinalizeDeps, wdeps WorkspaceDeps,
	inv sweepInventory, pin StatusPin, shared sweepSharedFacts,
	candidates []sweepWorkItem) (entries []MaintenanceEntry, actionable []sweepWorkItem)
```

- Consumes: `gitcli.ListRemoteHeads` (Task 3, gathered lazily in `maintenanceSweep` only when `done` candidates exist), `workspace.Service.Inspect` (local-only — "Inspect takes no remote: it never fetches"), the extracted backlink helper, the pinned integration revision (`pin.IntegrationRevision`) via `deps.Planning.Reader.ReadArtifact`.

- [ ] **Step 1: Write failing tests.** One test per assessment rule; fixtures build records + local state, a scripted `sweepSharedFacts`, and assert the exact entry `{Disposition, Reason, Operation: ""}` **and** that no cleanup op and no per-item remote call was dispatched:

```go
func TestAssessStackedMergedIsSnapshotRetainedNoDispatch(t *testing.T)      // skipped/snapshot-retained
func TestAssessCleanTombstoneAbsentRefsCorrectBacklinksIsNoWork(t *testing.T) // skipped/snapshot-no-work; asserts absence of ALL mutations
func TestAssessMissingManifestNoBacklinkWorkIsBlockedNotClean(t *testing.T) // blocked/snapshot-blocked; a missing/foreign manifest never certifies clean
func TestAssessMissingManifestWithStaleBacklinkIsActionable(t *testing.T)   // independent backlink leg → actionable despite workspace blocker
func TestAssessStaleBacklinkLegIsActionable(t *testing.T)                   // each leg in turn:
func TestAssessReadyWorkspaceLegIsActionable(t *testing.T)
func TestAssessLeftoverLocalRefLegIsActionable(t *testing.T)
func TestAssessLeftoverRemoteRefLegIsActionable(t *testing.T)               // via shared heads map
func TestAssessMalformedMarkersAreUnknownNeverNoWork(t *testing.T)          // unknown/snapshot-unknown
func TestAssessInvalidRecordDataIsSnapshotInvalid(t *testing.T)             // invalid branch/duplicate identity → skipped/snapshot-invalid
func TestAssessFailedRemoteHeadsBlocksNoWorkButNotLocalLegs(t *testing.T)   // remoteHeadsErr: absence-dependent verdict → unknown; locally
                                                                            // established work still dispatches
func TestAssessEmptyAdvertisementMeansNoHeads(t *testing.T)                 // success+empty ⇒ absence IS proven
func TestAssessUnknownLegNamedInMessageEvenWhenBlocked(t *testing.T)
```

- [ ] **Step 2: Run to verify failure.**

- [ ] **Step 3: Implement the assessment.** For each candidate, in order:
  1. Validate record identity; for `done`, validate recorded branch, target/base, canonical PR reference — invalid/ambiguous → `skipped`/`snapshot-invalid` (distinguishable from clean absence). Do **not** add these prerequisites to the `stacked-merged` path.
  2. `stacked-merged` → `skipped`/`snapshot-retained` (the existing unconditional retention; never dispatch merely to rediscover it).
  3. For `done`, assess all **independent legs**: (a) backlink leg — for each of the record's plan/results targets, read the artifact bytes at `pin.IntegrationRevision` (`ReadArtifact` with source `integration`) and ask the extracted helper whether the rendered interior differs (already-correct blocks and missing artifacts are no-effect only where the existing cleanup planner treats them that way; malformed markers/unreadable blobs/render failures → unresolved); (b) workspace leg — `workspace.Service.Inspect` with the snapshot-resolved target: `StateCleaned` (owned tombstone, no registration) certifies already-clean; `StateReady` owned = possible work; missing/foreign manifest does **not** certify clean; probe error → unresolved; (c) local ref leg from `shared.worktrees` + `ResolveRef` (local, no fetch); (d) remote ref leg from `shared.remoteHeads` keyed by exact full ref.
  4. Any leg with possible work → keep the item in `actionable` (it then flows through Task 7's normal prepare+`FinalizeCleanup` with all live proofs; a workspace blocker must not hide a needed independent backlink repair).
  5. Otherwise: all legs provably no-effect → `skipped`/`snapshot-no-work`; a local blocker preventing all remaining effects → `blocked`/`snapshot-blocked` naming the blocker; unresolved inspection → `unknown`/`snapshot-unknown` with the diagnostic. Unknown independent legs stay named in the message even when another leg is blocked. None of these outcomes triggers an individual metadata, PR, or remote-ref read.

  In `maintenanceSweep`, full scope: build `sweepSharedFacts` **only when there are consumers** (`done` candidates needing remote-ref assessment → one `ListRemoteHeads`; none → zero calls), run the assessment, emit its entries, and enqueue only `actionable`. Implementation scope keeps its existing deferred **count** — deferrals stay unprobed counts, never these per-item assessments (its scaling test `TestSweepImplementationScopeDoesNotGrowWithHistory` must stay green unmodified). Update the `MaintenanceEntry` doc comment to distinguish pre-dispatch observations (empty `Operation`) from dispatched-operation results; no applied count is manufactured. No age cutoff, no cache, no retry queue, no new status.

- [ ] **Step 4: Green + mutation-test** (treat `remoteHeadsErr` as an empty map → `TestAssessFailedRemoteHeadsBlocksNoWorkButNotLocalLegs` and `TestAssessEmptyAdvertisementMeansNoHeads` disagree and one reddens; skip the backlink leg when the workspace is blocked → `TestAssessMissingManifestWithStaleBacklinkIsActionable` reddens; restore). **Step 5: Commit** (`git commit -m "feat(app): snapshot assessment resolves non-actionable historical sweep records locally"`).

---

### Task 9: CLI — sweep-only 30s/60s dependency builder

**Files:**
- Modify: `internal/cli/maintenance.go` (new `newSweepFinalizeDeps`; the sweep subcommand uses it instead of `newFinalizeDeps`)
- Test: `internal/cli/maintenance_test.go` (or the package's existing wiring-test home)

**Interfaces:**
- Consumes: Tasks 1–2 options/accessors; `newPlanningDeps`'s construction shape (`internal/cli/change.go`).
- Produces:

```go
const (
	sweepNetworkReadTimeout  = 30 * time.Second
	sweepNetworkWriteTimeout = 60 * time.Second
)
func newSweepFinalizeDeps() (app.FinalizeDeps, error)
```

- [ ] **Step 1: Write failing tests** asserting resolved **non-default** policy through the real builder (learning `defaulted-param-hides-caller-wiring` — assert the resolved value, not the argument):

```go
func TestSweepDepsCarrySweepNetworkPolicies(t *testing.T) {
	deps, err := newSweepFinalizeDeps()
	// deps.Planning.Client.NetworkReadTimeout() == 30s, NetworkWriteTimeout() == 60s;
	// the GitHub client likewise (reach it via the concrete type the builder returns —
	// keep a typed handle in the test, or have the builder also return the clients
	// under test through a package-private struct).
}
func TestSweepDepsShareOneClientAcrossNestedSeams(t *testing.T) {
	// Engine, Reader, workspace Service, CleanupGit, Gate all wrap the SAME
	// *gitcli.Client instance (pointer equality where reachable), so no nested
	// dependency escapes the policy. PRProber and PRBatch wrap the same
	// *githubcli.Client.
}
func TestStandaloneDepsKeepDefaultPolicies(t *testing.T) {
	// newFinalizeDeps clients report the 5m default on both budgets.
}
```

- [ ] **Step 2: Run to verify failure.**

- [ ] **Step 3: Implement.** `newSweepFinalizeDeps` mirrors `newFinalizeDeps`/`newPlanningDeps` but constructs `gitcli.NewClient(gitcli.WithNetworkReadTimeout(sweepNetworkReadTimeout), gitcli.WithNetworkWriteTimeout(sweepNetworkWriteTimeout))` and `githubcli.NewClient(githubcli.WithNetworkReadTimeout(…), githubcli.WithNetworkWriteTimeout(…))`, then builds engine/reader/workspace/prober/batch/gate/CleanupGit over those exact clients (single client instance per adapter — leaving any reachable network path at the five-minute default fails the requirement). Swap the call in `newMaintenanceSweepSubcommand`. Doc comment records the measurement rationale next to the constants (learning `tolerance-constant-calibrated-on-one-machine`): healthy remote ops measured ~0.5s; 30s/60s are generous ceilings, sweep-only.

- [ ] **Step 4: Green + mutation-test** (build `CleanupGit` from a second default `gitcli.NewClient()` → `TestSweepDepsShareOneClientAcrossNestedSeams` reddens; restore). **Step 5: Commit** (`git commit -m "feat(cli): sweep-only 30s read / 60s write network deadlines"`).

---

### Task 10: Integration — traffic accounting, blocked-probe regression, scaling, movement

**Files:**
- Create: `internal/app/maintenance_traffic_integration_test.go`
- Modify: `internal/app/maintenance_test.go` / `finalize_e2e_test.go` fixtures only as needed (reuse their local-origin builders)
- Reference: `tests/README.md` for placement/budget rules — a new heavy file gets its own wall-clock budget row; keep fixtures shared per file.

**Interfaces:**
- Consumes: everything above via `app.MaintenanceSweep` with production wiring over counting fake `git`/`gh` executables (`gitcli.WithExecutable` wrapping the real git for allowed ops while logging argv; `githubcli.WithExecutable` a scripted fake). Classify each logged invocation by **purpose** (fetch of `refs/heads/docket` = metadata preparation; `ls-remote --symref … HEAD` = setup probe; `ls-remote --heads` = shared inventory; `api graphql` = PR batch; integration fetch inside cleanup = required proof), never by banning an argv spelling globally.

- [ ] **Step 1: Traffic-accounting test.** Drive multiple items, both scopes, equal and distinct default/integration branches, enabled and disabled reclaim, and a successful closeout+cleanup suffix. Assert from the recorded log: full setup ran once per invocation; GitHub identity resolved at most once; zero PR requests for zero PRs, one per 25 unique numbers; at most one `ls-remote --heads` (not one per historical branch); no historical merge checks for no-work/blocked records; **exactly one metadata fetch per successful preparation**, shared by helper+operation+nested readers (exercise reclaim so `WorkspaceInspect` runs — top-level fakes alone miss it); disabled reclaim prepares nothing; required operation-specific calls still present with their gates. Count actual processes, not interface calls.

- [ ] **Step 2: Phase-aware blocked-probe regression.** Replace/extend the old all-network-blocked completion coverage: the fake git allows initial setup, shared discovery, one preparation fetch per dispatched operation, and required operation traffic; after setup it **fails fast** (not hangs) any redundant setup probe or duplicate reader fetch, under a short independent watchdog (learning `mutation-target-needs-a-forced-exit` — the mutation probe must redden, not hang). Assert terminal return, the expected successful entries, and zero forbidden-probe attempts; *merely returning after skipping the whole worklist is not a pass* — pin the applied entries (learning `assert-pins-outcome-not-mechanism`). Separately: block an **allowed** metadata fetch and assert the bounded `reload-failed` skip; a sweep with blocked required access does not complete its mutations.

- [ ] **Step 3: Scaling fixture.** Real sweep wiring with 1, 25, and 250 historical records at fixed active/pending work — clean tombstones with absent refs and correct backlinks, stacked-merged records, legacy missing/foreign manifests without backlink work, locally blocked workspaces. Assert equal remote-call counts across populations, one truthful entry per full-scope candidate, and that implementation scope inspects no deferred resources, fetches no deferred remote heads, and keeps its unprobed deferred count.

- [ ] **Step 4: Metadata and source movement.** From an independent writer, advance origin's docket branch mid-sweep (between prepare points, via a fake-git hook or two-phase fixture): the next preparation observes changed blob versions/vanished records; helper+operation+nested readers agree on one observation even when a concurrent edit lands after it (the transaction's fresh-base/exact-version checks catch the conflict); the cleanup suffix's new observation sees the record its closeout archived; reusing a previous attempt's revision must redden. Advance default/integration/config after setup: this invocation keeps captured setup; direct proof fetches still see live state; metadata-fetch failure/deletion proves no stale fallback. For non-dispatched assessments, introduce work after inventory: this invocation makes no mutation and no fresh-verification claim; a shared clean snapshot never suppresses a mandatory cleanup suffix (assert the suffix still ran for a closeout whose record looked clean at inventory).

- [ ] **Step 5: Mutation sweep for this task's guards** (per Global Constraints, all with `-count=1`): restore a redundant pin in an operation closure; leave a nested reader unbound; reintroduce per-history refreshes; loop individual PR views inside the batch adapter; treat an unknown advertisement as empty; bypass an independent backlink leg. Each must redden a named test above. Also cover a second invocation and a second repository so observation reuse cannot escape its declared lifetime.

- [ ] **Step 6: Commit** (`git commit -m "test(app): sweep traffic accounting, blocked-probe, scaling, and movement regressions"`).

---

### Task 11: Timeout behavior and safety tests

**Files:**
- Create: `internal/app/maintenance_timeout_integration_test.go` (plus small additions to `internal/gitcli`/`internal/githubcli` integration tests where a hazard is adapter-local)

**Interfaces:**
- Consumes: Tasks 1–2 options (short injected durations), `WithExecutable` sleeping fakes.

- [ ] **Step 1: Policy propagation from source.** Derive the reachable network sites from the Task 1/2 greps and assert each receives the correct policy through the sweep command's real dependency builder (this is Task 9's pointer-equality test plus per-adapter selection tests — extend, don't duplicate; learning `correspondence-guard-runs-one-way`: also assert no `network: true` site is left unclassified by grepping for sites missing an explicit read/write disposition in the classification comment).

- [ ] **Step 2: Bounded-return tests with short durations.** Using milliseconds-scale injected budgets and sleeping fakes, block in turn: the initial pin's fetch (→ the sweep's typed external-failure refusal retains the adapter's timeout kind/message), a later metadata preparation (→ `reload-failed` skip, no dispatch), a PR batch read (→ unknown facts + finding), a transaction fetch, and a push/delete (→ existing unknown/recoverable outcome; no forbidden follow-on mutation; a potentially-applied timed-out write stays unknown unless reconciliation proves otherwise). Assert bounded return with scheduling/reaping allowance — a generous ceiling assert (e.g. returned within 5s for a 50ms budget), never equality.

- [ ] **Step 3: Budget-sharing tests.** A fetch whose failure-classification probe follows it shares one read budget (extend the existing `FetchBranch` shared-`netCtx` coverage in `internal/gitcli` with a read-budget variant); a push plus its reconciliation shares one write budget; a successful read's cancelled deadline context must not leak into the following mutation (drive a read-then-write sequence and assert the write is not prematurely cancelled).

- [ ] **Step 4: Mutation tests** (`-count=1`): drop policy propagation at one nested dependency (Task 9 test reddens); restore a duplicate reader fetch (Task 10 reddens); reset the clock before a diagnostic probe (budget-sharing test reddens); collapse read and write to one 30s policy (**a blanket thirty-second policy must fail** — assert `NetworkWriteTimeout() == 60s` distinctly, and have one behavioral test whose fake write survives longer than the read budget).

- [ ] **Step 5: Commit** (`git commit -m "test: sweep network deadline propagation, bounding, and budget sharing"`).

---

### Task 12: Performance evidence and build gate

**Files:**
- Create: `internal/app/maintenance_perf_evidence_test.go` **only if** a repeatable harness fits the suite budget; otherwise record the measurement procedure + numbers in the change's results evidence (the build's evidence record), not a suite test. This is a measured-oracle change (learning `optimization-needs-a-measured-oracle`): correctness asserts cannot prove it; wall clock and call counts are the acceptance.

- [ ] **Step 1: Measure before/after.** On one isolated representative multi-item workload (local origin; equal synthetic network latency injected via a fake git/gh that sleeps a fixed per-invocation delay — latency×call-count is then deterministic): record medians of at least 5 runs for the pre-change baseline (`git stash` or the merge-base build) and the candidate, plus categorized call counts (metadata fetches, setup probes, GitHub requests, operation proofs — from the Task 10 logs). Equivalent final resource/safety outcomes must hold; record the intentional reporting difference (old dispatched no-effect cleanup entries vs new snapshot assessments — disposition tokens are not required to match).

- [ ] **Step 2: Verify the call-budget table.** From the categorized counts: batched discovery (`ceil(P/25)` PR requests, ≤1 heads advertisement, ≤1 identity resolution); 0 individual calls for non-actionable historical assessments; 1 metadata preparation fetch per dispatched operation (2 for closeout+suffix); 0 nested-reader fetches. The baseline shows ≥2 full pins per dispatched operation (8–10 context-read calls each, plus nested pins). The candidate must reduce measured time on this controlled workload; do not infer end-to-end improvement solely from call counts, and do not promise archive-independent total runtime (payload/parsing still grow). Measure the 1/25/250 scaling fixture's request counts separately at fixed actual work.

- [ ] **Step 3: Record the evidence** in the build's results/evidence record: both medians, both call-count tables, the fixture description, hardware note, and the limitation that no live-sweep performance was measured (a live sweep mutates and is not a benchmark fixture).

- [ ] **Step 4: Run the full build gate.** `go run ./cmd/docket development test` from the feature worktree. Handle `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines as screening findings and any `SERIAL CONFIRMED OVER BUDGET:` line as an authoritative breach per `tests/README.md` (serial-confirm before acting). New integration files added by Tasks 10–11 get budget rows per that README.

- [ ] **Step 5: Commit any evidence/receipt files** the build contract requires (`git commit -m "test: performance evidence for sweep discovery batching"`).

---

## Self-Review (performed)

- **Spec coverage:** batched PR discovery (Tasks 4–5), shared GitHub identity (5, 10), shared remote heads (3, 8, 10), local historical assessment with truthful no-work/retained/blocked/unknown/invalid entries and independent-leg dispatch (8), one observation per attempt incl. nested readers and cleanup-suffix re-preparation (6–7), setup-probe elimination (6–7, 10), 30s/60s sweep-only deadlines with read/write distinction, budget sharing, and no timeout multiplication (1–2, 9, 11), failure/reporting behavior (5, 7, 8, 11), verification matrix incl. mutation tests, scaling, movement, blocked-probe phase fixture (10–11), performance evidence + build gate (12). Out-of-scope items (scope membership, proofs, global defaults, total deadline, mutation batching, health pass) are touched by no task.
- **Type consistency:** `sweepObservation`/`sweepPreparer`/`newBoundStatusReader` defined in Task 6 and consumed with those names in 7, 10; `SweepPRBatchReader.ProbePRSet` defined in 5, wired in 9; `BatchPRResult`/`ViewPullRequestsBatch` defined in 4, consumed in 5; reason constants defined in 8 and asserted in 8/10.
- **Known judgment points for the builder (not placeholders, decisions delegated with bounds):** the exact factory shape for handing the initial pin into `sweepOps.prepare` (Task 7 names the constraint — test seam preserved, session built once after the initial pin succeeds); the exact extraction seam for the backlink byte-comparison helper (Task 8 requires *move, don't copy*, malformed markers → error). Both are internal shapes the spec explicitly leaves as implementation details ("Concrete helper/type names are implementation details").
