<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0309 — Isolated metadata transaction engine](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0309-isolated-metadata-transaction-engine.md)**
<!-- docket:backlink:end -->

# Isolated Metadata Transaction Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `internal/repository/transaction` — a semantic-operation transaction engine that executes durable metadata mutations in Docket-owned private detached worktrees, validates complete before/after state, commits only declared paths, pushes under an exact expected-ref lease, retries semantically (at most four attempts), replays via request-ID receipts, and prunes only manifest-plus-live-lock-proven abandoned state — plus the narrow typed `internal/gitcli` additions it needs.

**Architecture:** The engine (outer adapter) sits above the pure read model: it fetches an exact target-ref revision, creates a locked private candidate under `<common-dir>/docket/transactions/`, loads/validates the complete base through a caller-supplied `StateLoader`, checks entity-version expectations, asks a `SemanticOperation` for a closed `MutationPlan` of final bytes, validates the complete candidate overlay, materializes exactly the declared paths through a rooted filesystem API, commits the explicit path set on detached `HEAD` with engine-owned trailers, and pushes with the literal lease `<TargetRef>:<base>`. Only a structurally proven lease loss retries — from a fresh fetch and a fresh plan, never a reused patch. `internal/gitcli` remains the only package that starts Git; its additions are named operations, no command runner.

**Tech Stack:** Go 1.26 stdlib only (`os` + `os.Root`, `crypto/rand`, `crypto/sha256`, `encoding/json`, `encoding/base64`, `syscall` flock as in `internal/install/lock.go`). Real temporary Git repositories in tests (pattern: `internal/gitcli/harness_test.go`); the test `StateLoader` is backed by the landed `document.Parse` + `repository.BuildSnapshot` — never a second production composer.

**Spec:** `docs/superpowers/specs/2026-08-15-isolated-metadata-transaction-engine-design.md` (on the `docket` metadata branch; reconciled 2026-08-15). The plan argues from the spec — executors read both.

## Global Constraints

- Module `github.com/danielhanold/docket`, Go 1.26; **no new module dependencies** — stdlib only (flock via `syscall.Flock`, the platform set already accepted by `internal/install/lock.go`).
- `gofmt -l` clean, `go vet ./...` clean, `go test ./...` green — enforced by `tests/test_go_toolchain.sh` through `scripts/run-tests.sh` (the resolved `finalize.test_command`); `go test -race ./...` green — enforced by `tests/test_go_race.sh`. Do NOT add a second shell producer for these packages: both files auto-discover new packages via `./...`.
- Existing production files in `internal/repository` stay free of `os`/process/Git imports — `internal/repository/boundary_test.go` guards this; the new code lives in the `internal/repository/transaction` subpackage. The subpackage may import `internal/repository`, `internal/domain`, `internal/document`, `internal/gitcli` — never `internal/cli`, `internal/app`, `internal/install`, `internal/harness`, `internal/config`-mutation paths, render, or GitHub packages.
- `internal/gitcli` keeps `run` package-private: no exported `Run`, argument-vector escape hatch, or caller-selected working directory. New operations take typed inputs and use argument arrays, the 0308 sanitized environment, and deadlines.
- No package reads the wall clock implicitly for a policy decision: the engine receives a `Clock`; tests pin it. Semantic operations never call `time.Now`.
- No production or test path invokes `git worktree prune`, resets a branch, deletes a ref, touches the primary checkout, the legacy `.docket/` worktree, a feature worktree, or a caller's index.
- Diagnostics are bounded and redact per 0308's adapter policy: never object bytes, unvalidated receipts, credentials, remote URLs, environment values, or unbounded subprocess output.
- Every mutation probe and manual re-verification uses `go test -count=1` (learnings: `cached-runner-serves-a-mutated-tree`).
- All Git path output is read NUL-delimited (`-z` / `--porcelain=v2 -z`), never display-form (learnings: `git-path-output-is-quoted`). Changed-path derivations that feed a safety predicate disable rename detection (learnings: `diff-derived-allowlist-needs-no-renames`).
- Path identity comparisons canonicalize every symlink hop (`filepath.Abs` + `filepath.EvalSymlinks`); macOS `/tmp` → `/private/tmp` makes this observable in every test (learnings: `canonicalise-every-symlink-hop`).
- Documented modes (dirs `0700`, files `0600`) are enforced with explicit `Chmod` after creation, and pinned by a test that sets `umask 077` itself (learnings: `promised-file-mode-needs-explicit-chmod`).
- Tests coordinate concurrent writers with channels/barriers, never sleeps; no live network — local bare remotes only.
- Retry re-derives everything from a fresh authoritative fetch (learnings: `cas-re-read-fresh-origin`); the idempotency probe keys on remote history — the state actually promised — never a local proxy (learnings: `idempotency-keying`).
- Commit-hook disabling is per command (`-c core.hooksPath=<owned empty dir>`); no shared/worktree Git config is ever mutated (learnings: `shared-git-config-mutation`).

## File Structure

```
internal/gitcli/
  worktree.go        # AddDetachedWorktree, RemoveWorktree, ListWorktrees (porcelain -z)
  status.go          # ChangedPaths: exact changed-path/status set for a worktree (v2 -z, no renames)
  commit.go          # CommitPaths: NUL-safe explicit-path stage+commit, hooks/signing disabled, trailers
  push.go            # PushLease (exact expected-old lease, --porcelain parse), IsAncestor
  trailers.go        # ScanCommitTrailers over reachable ancestry (git trailer grammar)
  worktree_test.go, status_test.go, commit_test.go, push_test.go, trailers_test.go

internal/repository/transaction/
  types.go           # Request, EntityExpectation, ExpectedVersion, OperationKey, IdempotencyKey,
                     #   MutationPlan, FileMutation, MutationKind + all input validation
  result.go          # Disposition, Result, typed Failure{Stage,Kind}, receipt schema validation
  tree.go            # Tree interface, baseTree (over gitcli.ObjectSource), overlayTree (base+plan)
  state.go           # LoadedState, StateLoader, AttemptState, SemanticOperation, OperationResult, Clock
  candidate.go       # transactions root, registry.lock, live.lock (flock), crypto/rand ID,
                     #   manifest schema + atomic write, phases, modes + explicit chmod
  materialize.go     # os.Root rooted writes: sibling-temp+rename, symlink/parent checks, readback
  commitverify.go    # declared-vs-actual changed-path set equality (both directions)
  engine.go          # Engine, Execute: attempt loop, gates, expectation checks, retry classification
  idempotency.go     # trailer block author/encode, ancestry scan, replay/misuse outcomes
  cleanup.go         # per-candidate cleanup; PruneAbandoned with full ownership proof
  types_test.go, result_test.go, tree_test.go, state_test.go, candidate_test.go,
  materialize_test.go, engine_test.go, idempotency_test.go, cleanup_test.go
  harness_test.go    # real-repo fixtures: bare origin + two independent clones, both topologies
  loader_test.go     # test StateLoader over document.Parse + repository.BuildSnapshot + small corpus
  concurrency_test.go# unrelated/same-entity/derived-overlap/four-losses writer matrix
  interrupt_test.go  # cancellation, lost response, materialization-failure injection
  recovery_test.go   # ownership & pruning matrix
  preserve_test.go   # byte-identical user checkout/index proof
```

`tests/runtime-budgets.tsv`: `tests/test_go_toolchain.sh` (20s) and `tests/test_go_race.sh` (60s — the table's **hard ceiling**) both absorb this package. Task 10 measures both standalone-serial and re-budgets/shards deliberately (learnings: `budget-headroom-is-spent-before-it-is-breached`; shard precedent: change 0324 and `tests/test_go_race.sh`'s own header).

---

### Task 1: Transaction vocabulary, input validation, typed results

**Files:**
- Create: `internal/repository/transaction/types.go`, `internal/repository/transaction/result.go`
- Test: `internal/repository/transaction/types_test.go`, `internal/repository/transaction/result_test.go`

**Interfaces:**
- Consumes: `gitcli.RepoPath`, `gitcli.ObjectID`, `gitcli.RefName`, `gitcli.RemoteName`, `gitcli.Repository`, `domain.Finding`.
- Produces (exact — every later task uses these):

```go
package transaction

type OperationKey string   // must match ^[a-z][a-z0-9.-]*$

type VersionKind string
const (
    VersionBlob   VersionKind = "blob"
    VersionAbsent VersionKind = "absent"
)

type ExpectedVersion struct {
    Kind     VersionKind
    ObjectID gitcli.ObjectID // required for blob (full hex, exact); must be empty for absent
}

type EntityExpectation struct {
    Path    gitcli.RepoPath
    Version ExpectedVersion
}

type RequestDigest string // "sha256:" + 64 lowercase hex

type IdempotencyKey struct {
    RequestID string        // 8–128 ASCII, ^[A-Za-z0-9][A-Za-z0-9._-]*$, case-sensitive
    Digest    RequestDigest
}

type MutationKind string
const (
    MutationCreate  MutationKind = "create"
    MutationReplace MutationKind = "replace"
    MutationDelete  MutationKind = "delete"
)

type FileMutation struct {
    Path  gitcli.RepoPath
    Kind  MutationKind
    Bytes []byte // must be nil/empty for delete; may be empty for an intentionally empty file
}

type MutationPlan struct {
    Files         []FileMutation
    CommitSubject string // one non-empty UTF-8 line, no control chars, <= 200 bytes
    Receipt       []byte // canonical compact JSON, <= 4096 bytes
}

// Validation (package-private, called by the engine before any Git/filesystem work):
func validateOperationKey(k OperationKey) error
func validateExpectations(exps []EntityExpectation) error   // shape + duplicates
func validateIdempotencyKey(k *IdempotencyKey) error        // nil is valid (no key)
func validatePlan(p MutationPlan) error                     // paths, kinds, subject, receipt
func validateReceipt(b []byte) error                        // compact canonical JSON object, closed values, size

type Disposition string
const (
    DispositionApplied        Disposition = "applied"
    DispositionAlreadyApplied Disposition = "already-applied"
    DispositionNoOp           Disposition = "no-op"
    DispositionContended      Disposition = "contended"
    DispositionRefused        Disposition = "refused"
    DispositionFailed         Disposition = "failed"
    DispositionInterrupted    Disposition = "interrupted"
)

type Result struct {
    Disposition     Disposition
    Operation       OperationKey
    RequestID       string           // empty when no key
    BaseCommit      gitcli.ObjectID  // last fetched base, when known
    RemoteCommit    gitcli.ObjectID  // last observed remote target, when known
    AppliedCommit   gitcli.ObjectID  // on applied / already-applied
    Attempts        int
    Receipt         []byte           // decoded validated receipt on applied/already-applied
    ContendedPaths  []gitcli.RepoPath // paths only, never bytes
    Findings        []domain.Finding  // refusal / validation diagnostics
    CleanupWarnings []string          // e.g. "cleanup-pending: <transaction-id>"
}

type Stage string   // "validate-request", "fetch", "idempotency-scan", "allocate", "worktree",
                    // "load-before", "expectations", "plan", "load-after", "materialize",
                    // "verify-delta", "commit", "push", "probe", "cleanup", "recovery"
type Kind string
const (
    KindInvalidInput  Kind = "invalid-input"  // bad request/plan/receipt/key
    KindInvalidState  Kind = "invalid-state"  // repo/history contradicts engine invariants
    KindValidation    Kind = "validation"     // before/after/evolution error findings
    KindExternal      Kind = "external"       // git/transport/auth/identity failures
    KindCancelled     Kind = "cancelled"
    KindUnknownResult Kind = "unknown-result" // push outcome not establishable
)

type Failure struct {
    Stage  Stage
    Kind   Kind
    Detail string // bounded, redacted
    Err    error  // wrapped cause, may be nil
}
func (f *Failure) Error() string
func (f *Failure) Unwrap() error
func AsFailure(err error) (*Failure, bool)
```

- [ ] **Step 1: Write failing tests.** Table-driven in `types_test.go`: operation keys (accept `change.groom`, `a`, `x9.y-z`; reject empty, `Change`, `9x`, `a b`, `a:b`, unicode); request-ID shape both bounds (7 and 129 bytes reject, 8 and 128 accept; reject space/colon/newline/unicode/control, leading `.`); digest shape (reject uppercase hex, wrong length, missing prefix); expectations (reject empty path, abbreviated SHA, blob-with-empty-ID, absent-with-ID, duplicate path); plan validation (reject: duplicate path, absolute path, `..` traversal, `.` segment, NUL byte, `.git` and `.git/x`, empty path, delete-with-bytes; subject: empty, embedded `\n`, control char, 201 bytes; accept 200-byte subject, empty `Bytes` on create); receipt validation (reject: non-JSON, JSON array, insignificant whitespace i.e. `!= json.Compact` of itself, 4097 bytes, nested unknown-shape maps as top value is fine to accept only when it is an object — assert an object with string/number/bool/array-of-scalar values passes and a map-typed re-marshal mismatch fails). `result_test.go`: `Failure` implements `error`, `Unwrap` round-trips through `errors.Is`, `AsFailure` matches wrapped and misses plain errors.
- [ ] **Step 2: Run `go test -count=1 ./internal/repository/transaction/` — expect FAIL (package does not compile / functions undefined).**
- [ ] **Step 3: Implement `types.go` + `result.go` exactly to the signatures above.** Receipt canonicality: unmarshal into `any`, re-marshal with `json.Marshal`, require byte equality with the input and top-level `map` decode refused by decoding into `json.RawMessage` object check — concretely: require first byte `{`, `json.Valid`, no `\n`/`\r`/insignificant whitespace via compact-equality, and `len<=4096`.
- [ ] **Step 4: Run `go test -count=1 ./internal/repository/transaction/` — expect PASS. Run `gofmt -l internal/` (empty) and `go vet ./...` (clean).**
- [ ] **Step 5: Commit** `feat(transaction): vocabulary types, input validation, typed results (0309 task 1)`.

---

### Task 2: Tree interface, base/overlay trees, state and operation contracts

**Files:**
- Create: `internal/repository/transaction/tree.go`, `internal/repository/transaction/state.go`
- Test: `internal/repository/transaction/tree_test.go`, `internal/repository/transaction/state_test.go`

**Interfaces:**
- Consumes: Task 1 types; `gitcli.ObjectSource`, `gitcli.TreeEntry`, `gitcli.BlobResult`, `gitcli.Revision`; `domain.Snapshot`, `domain.ValidationReport`, `domain.Finding`; `document.Document`.
- Produces:

```go
// Tree is the read-only subset a repository loader needs.
type Tree interface {
    Revision() gitcli.Revision // overlay reports its base revision
    ListTree(ctx context.Context, prefixes []gitcli.RepoPath) ([]gitcli.TreeEntry, error)
    ReadBlobs(ctx context.Context, paths []gitcli.RepoPath) ([]gitcli.BlobResult, error)
}

func newBaseTree(src gitcli.ObjectSource) Tree
// newOverlayTree layers a validated plan over base: created/replaced paths
// resolve to plan bytes (mode 100644 for creates, base mode for replaces,
// ObjectID "" — loaders read bytes, not IDs, from an overlay); deleted paths
// vanish from both ListTree and ReadBlobs.
func newOverlayTree(base Tree, plan MutationPlan) (Tree, error)

type LoadedState struct {
    Snapshot  domain.Snapshot
    Report    domain.ValidationReport
    Documents map[string]document.Document // keyed by repo-relative path; defensively copied views
    Sources   map[string][]byte            // exact bytes per path (evolution input)
}

type StateLoader interface {
    Load(ctx context.Context, t Tree) (LoadedState, error)
    ValidateEvolution(before, after LoadedState) []domain.Finding
}

type AttemptState struct {
    Base  gitcli.Revision
    State LoadedState
    Tree  Tree
}

type OperationResult struct {
    Refused  bool
    Findings []domain.Finding // populated on refusal
}

type SemanticOperation interface {
    Key() OperationKey
    // Returns a closed plan of final bytes for THIS attempt's state, or a typed
    // refusal. error is a programmer/loader failure, never a domain outcome.
    Plan(ctx context.Context, st AttemptState) (MutationPlan, OperationResult, error)
}

type Clock interface{ Now() time.Time }
```

- [ ] **Step 1: Write failing tests** with an in-memory fake `gitcli.ObjectSource` (map path→(mode, bytes); implements `Revision/ListTree/ReadBlobs` — assert the fake satisfies the interface at compile time). `tree_test.go`: base tree passes through entries/blobs and revision; overlay: create adds an entry with mode `100644` and serves plan bytes; replace preserves the base mode (fixture includes a `100755` file) and serves plan bytes; delete removes the entry from `ListTree` and yields `Found: false` from `ReadBlobs`; untouched paths serve base bytes byte-identically; `newOverlayTree` rejects a plan whose replace/delete target is absent from base, whose create target exists in base, or whose target is not a regular blob (symlink `120000` / gitlink `160000` fixture entries) — these are the "exact before/after mode rules" and are checked here so every engine attempt inherits them. `state_test.go`: overlay input plan is copied — mutating the caller's `plan.Files[i].Bytes` after construction does not change what `ReadBlobs` returns.
- [ ] **Step 2: Run `go test -count=1 ./internal/repository/transaction/` — expect FAIL.**
- [ ] **Step 3: Implement `tree.go` + `state.go`.** Overlay copies all plan bytes at construction; `ListTree` merges base entries (minus deletes, plus creates) honoring the requested prefixes; deterministic path-sorted order matching `gitcli.ObjectSource` semantics.
- [ ] **Step 4: Run the package tests — PASS; `gofmt`/`go vet` clean.**
- [ ] **Step 5: Commit** `feat(transaction): tree overlay and state/operation contracts (0309 task 2)`.

---

### Task 3: gitcli — detached transaction worktrees and exact changed-path sets

**Files:**
- Create: `internal/gitcli/worktree.go`, `internal/gitcli/status.go`
- Test: `internal/gitcli/worktree_test.go`, `internal/gitcli/status_test.go` (reuse `harness_test.go` fixtures)

**Interfaces:**
- Consumes: `Client.run` (package-private), `Repository`, `ObjectID`, `RepoPath`, `Failure`.
- Produces:

```go
// Operations: "worktree-add", "worktree-remove", "worktree-list", "changed-paths"
// (add to the Operation doc list in failure.go).

// AddDetachedWorktree registers a detached worktree at path (absolute, already
// proven beneath the engine-owned root by the caller) checked out at commit.
// Never creates or resets a branch: `git -C <primary> worktree add --detach <path> <commit>`.
func (c *Client) AddDetachedWorktree(ctx context.Context, repo Repository, path string, commit ObjectID) error

// RemoveWorktree removes exactly the registered worktree at path
// (`git worktree remove --force <path>` — force because the transaction
// worktree intentionally carries staged/dirty state at cleanup time).
func (c *Client) RemoveWorktree(ctx context.Context, repo Repository, path string) error

type WorktreeInfo struct {
    Path     string   // as reported; callers canonicalize before comparing
    Head     ObjectID
    Detached bool
    Branch   RefName  // empty when detached
}
// ListWorktrees parses `git worktree list --porcelain -z` — never human display output.
func (c *Client) ListWorktrees(ctx context.Context, repo Repository) ([]WorktreeInfo, error)

type PathChange struct {
    Path   RepoPath
    Staged bool // change present in the index vs HEAD
}
// ChangedPaths returns the exact set of paths differing from HEAD in the
// worktree at dir — index and working tree, tracked and untracked — via
// `git -C <dir> status --porcelain=v2 -z --untracked-files=all --no-renames`.
// Rename detection off: a safety predicate must see both sides of any move.
func (c *Client) ChangedPaths(ctx context.Context, dir string) ([]PathChange, error)
```

- [ ] **Step 1: Write failing tests** against `newMainModeRepos`: add a detached worktree under `t.TempDir()` at a known commit → `ListWorktrees` contains it with `Detached: true` and the exact `Head`; the primary checkout's `HEAD`, branch, and index are unchanged (reuse the preservation snapshot idioms from `preserve_test.go`); no local branch was created (`branchExists` false for any new name). `RemoveWorktree` on a worktree with staged + untracked files succeeds and deregisters it; removing an unregistered path returns a typed `*Failure`, not a panic. `ChangedPaths`: create/modify/delete files (including the harness's hostile non-ASCII and quote-carrying paths) in a detached worktree, stage some — assert the exact set with `Staged` flags, byte-exact hostile paths (NUL-delimited parse, `core.quotePath=true` pinned in the fixture repo), and that a `git mv` reports **both** source and destination.
- [ ] **Step 2: Run `go test -count=1 ./internal/gitcli/` — expect FAIL.**
- [ ] **Step 3: Implement.** Reuse `run` with the sanitized environment; parse porcelain `-z` records only.
- [ ] **Step 4: Run `go test -count=1 ./internal/gitcli/` — PASS; `gofmt`/`go vet` clean.**
- [ ] **Step 5: Commit** `feat(gitcli): detached worktree lifecycle and exact changed-path sets (0309 task 3)`.

---

### Task 4: gitcli — explicit-path commit, lease push, reachability, trailer scan

**Files:**
- Create: `internal/gitcli/commit.go`, `internal/gitcli/push.go`, `internal/gitcli/trailers.go`
- Test: `internal/gitcli/commit_test.go`, `internal/gitcli/push_test.go`, `internal/gitcli/trailers_test.go`

**Interfaces:**
- Consumes: Task 3 operations; `FetchBranch`, `ResolveRef`.
- Produces:

```go
type Trailer struct{ Key, Value string } // Key: token grammar; Value: one line, no control chars

type CommitRequest struct {
    Dir       string    // the detached transaction worktree
    Paths     []RepoPath // explicit set: additions, replacements, AND deletions
    Subject   string
    Trailers  []Trailer // engine-owned; appended as the message's final trailer block
    HooksPath string    // owned empty dir; passed as -c core.hooksPath=<HooksPath>
    When      time.Time // engine clock; sets GIT_AUTHOR_DATE / GIT_COMMITTER_DATE
}
// CommitPaths stages exactly Paths via
// `git add --pathspec-from-file=- --pathspec-file-nul` (git add stages removals
// for named deleted paths), then commits with `-c core.hooksPath=… -c commit.gpgsign=false
// commit --no-verify -F <msgfile>` and returns the new commit ID. Uses the
// repository's configured identity; a missing user.name/user.email surfaces as a
// typed *Failure (KindExternal-shaped kind from 0308's taxonomy), never a
// hard-coded person.
func (c *Client) CommitPaths(ctx context.Context, repo Repository, req CommitRequest) (ObjectID, error)

type PushDisposition string
const (
    PushApplied  PushDisposition = "applied"
    PushLeaseLost PushDisposition = "lease-lost"
    PushFailed   PushDisposition = "failed"  // external/unknown; never retried as contention
)
type PushOutcome struct {
    Disposition PushDisposition
    Remote      ObjectID // observed remote target when establishable, else ""
}
// PushLease pushes commit to ref with the literal expected old value:
// `git push --porcelain --force-with-lease=<ref>:<expected> <remote> <commit>:<ref>`.
// Classification is structural: parse the --porcelain per-ref result line
// (ok / rejected with reason token), never human stderr. Lease-lost requires the
// porcelain rejection AND a follow-up fetch showing remote != expected and
// !IsAncestor(commit, remote); anything else is PushFailed.
func (c *Client) PushLease(ctx context.Context, repo Repository, remote RemoteName, ref RefName, commit, expected ObjectID) (PushOutcome, error)

// IsAncestor: `git merge-base --is-ancestor <a> <b>` — exit 0 → true, exit 1 → false,
// anything else a typed failure.
func (c *Client) IsAncestor(ctx context.Context, repo Repository, ancestor, descendant ObjectID) (bool, error)

type CommitTrailers struct {
    Commit   ObjectID
    Trailers []Trailer
}
// ScanCommitTrailers walks every commit reachable from `from` and returns, for
// commits carrying at least one of keys, their full trailer sets — parsed with
// Git's own trailer interpretation:
// `git log -z --format=%H%x01%(trailers:only,unfold) <from>`.
// No history-depth window; full ancestry.
func (c *Client) ScanCommitTrailers(ctx context.Context, repo Repository, from ObjectID, keys []string) ([]CommitTrailers, error)
```

- [ ] **Step 1: Write failing tests.** `commit_test.go`: in a detached worktree — commit a set containing a create, a replace, a delete, and a hostile-path file; `git diff-tree --no-renames --no-commit-id --name-only -z -r <commit>` equals exactly the declared set; a dirty *undeclared* file in the same worktree is NOT in the commit; trailers appear as a well-formed final block (`git interpret-trailers --parse` output matches); a `pre-commit` hook installed in the fixture repo's real hooks dir that would `exit 1` does not fire (hooksPath override proof); `commit.gpgsign=true` set in the fixture repo config does not break the commit (signing disabled per command); with `user.name` unset in an env-isolated fixture, `CommitPaths` returns a typed failure. Author/committer dates equal `req.When`. `push_test.go`: push to the bare origin with correct expected → `PushApplied` and origin ref equals the commit; advance origin via the writer clone first → `PushLeaseLost` with `Remote` set to the winner; expected equal to actual-but-transport-broken (point remote at a nonexistent path in a throwaway config? — instead: pass an `expected` that matches while the remote repo directory has been made unreadable via chmod 000, restored in cleanup) → `PushFailed`, never lease-lost. `IsAncestor` truth table. `trailers_test.go`: three commits — one with two Docket trailers, one with none, one with a trailer-looking line in the body but not in the trailer block — scan returns exactly the right commits/pairs; hostile subject content (`Docket-Result: x` as the subject's text) does not match (grammar, not substring grep).
- [ ] **Step 2: Run `go test -count=1 ./internal/gitcli/` — expect FAIL.**
- [ ] **Step 3: Implement `commit.go`, `push.go`, `trailers.go`.** Message file written to a temp file inside the worktree's candidate dir is not available at this layer — build the message via `-F -` on stdin. Validate `Trailer` key/value shape before composing.
- [ ] **Step 4: Run `go test -count=1 ./internal/gitcli/` — PASS; `gofmt`/`go vet` clean.**
- [ ] **Step 5: Commit** `feat(gitcli): explicit-path commit, exact-lease push, reachability, trailer scan (0309 task 4)`.

---

### Task 5: Candidate lifecycle — private root, locks, manifest, transaction ID

**Files:**
- Create: `internal/repository/transaction/candidate.go`
- Test: `internal/repository/transaction/candidate_test.go`

**Interfaces:**
- Consumes: Task 1 types; `gitcli.Repository`; flock pattern from `internal/install/lock.go` (reference only — reimplement here; transaction must not import `internal/install`).
- Produces:

```go
const manifestSchemaVersion = 1

type phase string
const (
    phaseAllocating phase = "allocating"
    phaseReady      phase = "ready"
    phaseCommitted  phase = "committed"
    phasePushed     phase = "pushed"
)

type manifest struct {
    Schema        int              `json:"schema"`
    TransactionID string           `json:"transaction_id"`
    CommonDir     string           `json:"common_dir"`      // canonical
    Remote        gitcli.RemoteName `json:"remote"`
    TargetRef     gitcli.RefName   `json:"target_ref"`
    BaseCommit    gitcli.ObjectID  `json:"base_commit"`
    WorktreeRel   string           `json:"worktree_rel"`    // "worktree"
    Phase         phase            `json:"phase"`
    CreatedUTC    string           `json:"created_utc"`     // RFC3339, from Clock
    UpdatedUTC    string           `json:"updated_utc"`
    PID           int              `json:"pid"`             // diagnostic only — never liveness
}

func transactionsRoot(repo gitcli.Repository) string // <CommonDir>/docket/transactions

func newTransactionID() (string, error) // 32 lowercase hex chars from crypto/rand (128 bits)

type candidate struct {
    id       string
    root     string // <transactionsRoot>/<id>
    worktree string // <root>/worktree
    hooks    string // <root>/hooks
    live     *fileLock
}

// allocateCandidate: under withRegistryLock — mkdir root (0700 + explicit Chmod),
// mkdir hooks (empty, 0700), acquire live.lock (0600) BEFORE writing manifest,
// then atomically publish manifest.json (0600): same-directory temp file,
// f.Sync(), os.Rename, directory sync.
func allocateCandidate(clk Clock, repo gitcli.Repository, remote gitcli.RemoteName,
    ref gitcli.RefName, base gitcli.ObjectID) (*candidate, error)

func (c *candidate) setPhase(clk Clock, p phase) error // atomic manifest rewrite

// withRegistryLock: flock on <transactionsRoot>/registry.lock (0600), blocking,
// held only across allocate/inventory — never across mutation work.
func withRegistryLock(root string, fn func() error) error

type fileLock struct{ f *os.File }
func acquireLock(path string, block bool) (*fileLock, error) // flock LOCK_EX (|LOCK_NB when !block)
func (l *fileLock) release() error
```

- [ ] **Step 1: Write failing tests.** Allocation creates `<common>/docket/transactions/<id>/` with `manifest.json`, `live.lock`, empty `hooks/`; ID matches `^[0-9a-f]{32}$` and two allocations differ; **under `syscall.Umask(0o077)` (set and restored inside the test) AND under `0o022`** dirs are exactly `0700`, files `0600` (explicit-chmod proof); manifest JSON round-trips with all fields, `Phase` transitions rewrite atomically (no partial file observable: write a large manifest and read concurrently in a goroutine loop — every read parses); a second `acquireLock(live, false)` on a held lock fails with a would-block error; `withRegistryLock` excludes a concurrent locker (goroutine + channel barrier, no sleeps); registry lock is NOT held while the caller's `fn` has returned. `transactionsRoot` sits under the harness repo's real common dir and is invisible to `git status` in the primary checkout (run `ChangedPaths` — empty).
- [ ] **Step 2: Run `go test -count=1 ./internal/repository/transaction/` — expect FAIL.**
- [ ] **Step 3: Implement `candidate.go`** (flock exactly as `internal/install/lock.go` motivates: `LOCK_EX|LOCK_NB` on an `O_CREATE` file; liveness is the held lock, never PID).
- [ ] **Step 4: Run package tests — PASS; also `go test -race -count=1 ./internal/repository/transaction/` for the lock tests; `gofmt`/`go vet` clean.**
- [ ] **Step 5: Commit** `feat(transaction): candidate lifecycle — private root, flock ownership, atomic manifest (0309 task 5)`.

---

### Task 6: Rooted materialization and actual-delta verification

**Files:**
- Create: `internal/repository/transaction/materialize.go`, `internal/repository/transaction/commitverify.go`
- Test: `internal/repository/transaction/materialize_test.go`

**Interfaces:**
- Consumes: Task 1 `MutationPlan`/`FileMutation`; Task 3 `gitcli.PathChange`/`ChangedPaths`.
- Produces:

```go
// materializePlan applies a validated plan inside the worktree through an
// os.Root anchored there. For create/replace: write to a sibling temp file in
// the target's directory, sync, rename into place. For delete: remove the file.
// Creates may make missing parent directories only beneath the root and only for
// declared file paths. Refuses (typed *Failure, stage "materialize"): any parent
// component that is a symlink or non-directory; any target that is a symlink;
// escaping/absolute paths (defense in depth — validatePlan already rejected them).
// Existing bytes outside planned paths are never touched.
func materializePlan(worktree string, plan MutationPlan) error

// verifyMaterialized re-reads every created/replaced path and requires exact
// byte equality with the plan; requires deleted paths absent.
func verifyMaterialized(worktree string, plan MutationPlan) error

// verifyActualDelta asks Git for the worktree's actual changed-path set and
// requires SET EQUALITY with the plan's declared paths in BOTH directions —
// an undeclared changed path and a declared-but-unchanged path are each
// *Failure{Stage: "verify-delta", Kind: KindInvalidState}. A non-empty plan
// producing an empty Git delta is the spec's "plan did not describe reality".
func verifyActualDelta(ctx context.Context, client *gitcli.Client, repo gitcli.Repository,
    worktree string, plan MutationPlan) error
```

- [ ] **Step 1: Write failing tests** in a real detached worktree from the harness: create/replace/delete land byte-exact (`verifyMaterialized` passes; unrelated files byte-identical before/after — hash the whole tree minus declared paths); create under a new nested dir works; **containment matrix**: replace-target-is-symlink refused; parent-component-is-symlink (link a dir to outside `t.TempDir()`) refused with nothing written outside the root (assert the outside dir's mtime/contents unchanged); executable base file keeps mode `100755` through replace (sibling-temp rename preserves nothing by itself — copy the original mode onto the temp file before rename, and assert it). `verifyActualDelta`: exact match passes; an extra stray file in the worktree fails (undeclared direction); a declared path whose bytes equal base fails (declared-but-unchanged direction); hostile paths pass byte-exact.
- [ ] **Step 2: Run `go test -count=1 ./internal/repository/transaction/` — expect FAIL.**
- [ ] **Step 3: Implement** using `os.OpenRoot` (Go 1.24+ `os.Root`: `Stat`/`Lstat`/`Mkdir`/`Create`/`Rename`/`Remove` all refuse escapes and symlink traversal by construction) — verify parents with `Lstat` per component.
- [ ] **Step 4: Run package tests — PASS; `gofmt`/`go vet` clean.**
- [ ] **Step 5: Commit** `feat(transaction): rooted materialization and two-way actual-delta verification (0309 task 6)`.

---

### Task 7: Engine — attempt loop, gates, expectations, retry classification, outcomes

**Files:**
- Create: `internal/repository/transaction/engine.go`, `internal/repository/transaction/cleanup.go` (per-candidate cleanup half only)
- Test: `internal/repository/transaction/engine_test.go`, `internal/repository/transaction/loader_test.go`, `internal/repository/transaction/harness_test.go`

**Interfaces:**
- Consumes: everything from Tasks 1–6; `gitcli.FetchBranch`, `gitcli.OpenObjectSource`, `repository.BuildSnapshot` (tests), `document.Parse` (tests).
- Produces:

```go
type Request struct {
    Repository  gitcli.Repository
    Remote      gitcli.RemoteName
    TargetRef   gitcli.RefName // must be fully qualified refs/heads/...
    Expected    []EntityExpectation
    Idempotency *IdempotencyKey
    Loader      StateLoader
    Operation   SemanticOperation
}

const maxAttempts = 4 // initial + at most three semantic retries; package constant, not config

type Engine struct{ /* client *gitcli.Client; clock Clock — shared, no per-attempt state */ }
func NewEngine(client *gitcli.Client, clock Clock) (*Engine, error)

// Execute runs the spec's exact attempt sequence. Request slices/bytes are
// copied at entry; Engine and Client are safe for concurrent goroutines.
// Go error return = programmer/call-shape failure only; every expected outcome
// (contended/refused/no-op/failed/interrupted) is a Result.
func (e *Engine) Execute(ctx context.Context, req Request) (Result, error)

// cleanupCandidate (cleanup.go): capture diagnostics first, RemoveWorktree on
// the exact registered path, release live.lock, remove the candidate dir via a
// root anchored at transactionsRoot. Failure after a successful push produces
// Result{Disposition: applied, CleanupWarnings: ["cleanup-pending: <id>"]} —
// never a relabel to failed.
func (e *Engine) cleanupCandidate(ctx context.Context, repo gitcli.Repository, c *candidate) []string
```

Per-attempt order (implement exactly; the spec's list is authoritative): fetch → (idempotency scan — Task 8 stub returns not-found for nil key; keyed requests error `KindInvalidInput` until Task 8 lands, and `engine_test.go` covers keys only in Task 8) → allocate candidate → add detached worktree at base → open object source at base, `Loader.Load(baseTree)`, refuse on `Report.HasErrors()` → check every expectation against the base tree (`ListTree` exact-path lookups; blob compares full ObjectID; absent requires no entry) → `Operation.Plan` → `validatePlan` → overlay load + `Report.HasErrors()` + `ValidateEvolution` → empty `Files` ⇒ no-op (no commit) → `materializePlan` + `verifyMaterialized` + `verifyActualDelta` → `CommitPaths` (subject + engine trailer block; hooks dir = candidate's; `When` from clock) → `setPhase(committed)` → `PushLease(ref, commit, base)` → applied: `setPhase(pushed)`, cleanup, return; lease-lost: cleanup, next attempt; failed: post-push probe (fetch + `IsAncestor(commit, remote)`; reachable ⇒ applied) else `failed`/`interrupted`/unknown per spec.

Retry classification: ONLY `PushLeaseLost` (already structurally proven inside `PushLease`) retries. First-attempt expectation mismatch ⇒ `contended` immediately; mismatch on a later attempt ⇒ `contended`. Everything else — validation, containment, commit, auth/transport, cancellation, cleanup — terminal with its typed kind. Fourth lease loss ⇒ `contended` with `Attempts: 4` and last observed remote.

- [ ] **Step 1: Write the test loader + harness.** `loader_test.go`: `testLoader` implementing `StateLoader` over a small complete Docket corpus (config + 2 changes + 1 ADR under `docs/changes/…`, mirroring `internal/repository/testdata` shapes): `Load` lists `docs/` via the tree, `document.Parse`es each record, `repository.BuildSnapshot` → `LoadedState`; unreadable/omitted accounted path ⇒ error. `ValidateEvolution` delegates to `repository.ValidateEvolution` with sources from both states. `harness_test.go`: port `newMainModeRepos`/`newDocketModeRepos` shapes from `internal/gitcli/harness_test.go` — bare origin + writer clone + invocation clone, corpus committed on the target branch, `core.quotePath=true`; docket topology: orphan `docket` branch holding the corpus, `.docket.yml` on main.
- [ ] **Step 2: Write failing engine tests** (fakes for loader/operation where Git is not the subject; real harness where it is): happy path — applied, commit on origin contains exactly the planned paths (verify with `diff-tree --no-renames`), trailer block present, worktree/candidate gone after cleanup, `Result` fields all populated; no-op — empty plan ⇒ no commit on origin, `DispositionNoOp`; refusal — `OperationResult{Refused: true}` ⇒ `DispositionRefused` with findings, no commit; before-gate — corrupt a corpus record so `HasErrors()` ⇒ refused/failed (`KindValidation`) before `Plan` is invoked (operation records it was never called); after-gate — operation plans an invalid record ⇒ `KindValidation`, nothing pushed; evolution-gate — operation rewrites a frozen ADR body ⇒ blocked; expectation matrix — matching blob passes, stale blob ⇒ contended first-attempt without any candidate commit, absent passes/fails correctly; non-branch `TargetRef` (tag ref / short name) ⇒ `KindInvalidInput`; target branch missing on origin ⇒ typed failure, never branch creation; concurrent `Execute` calls on one Engine under `-race` (two goroutines, separate refs in separate harness repos, channel-coordinated).
- [ ] **Step 3: Run `go test -count=1 ./internal/repository/transaction/` — expect FAIL.**
- [ ] **Step 4: Implement `engine.go` + the cleanup half of `cleanup.go`.**
- [ ] **Step 5: Run `go test -count=1 ./internal/repository/transaction/` then `go test -race -count=1 ./internal/repository/transaction/` — PASS; `gofmt`/`go vet` clean.**
- [ ] **Step 6: Commit** `feat(transaction): engine attempt loop, validation gates, lease push, typed outcomes (0309 task 7)`.

---

### Task 8: Request-ID idempotency — trailer block, ancestry scan, replay

**Files:**
- Create: `internal/repository/transaction/idempotency.go`
- Modify: `internal/repository/transaction/engine.go` (replace the Task 7 keyed-request stub with the real scan; author the two request trailers)
- Test: `internal/repository/transaction/idempotency_test.go`

**Interfaces:**
- Consumes: Task 4 `ScanCommitTrailers`; Task 1 validation.
- Produces:

```go
const (
    trailerTransactionID = "Docket-Transaction-ID"
    trailerOperation     = "Docket-Operation"
    trailerRequestID     = "Docket-Request-ID"
    trailerRequestDigest = "Docket-Request-Digest"
    trailerResult        = "Docket-Result" // unpadded base64url of the canonical JSON receipt
)

// engineTrailers builds the exactly-one engine-authored block: the three
// always-present trailers, plus both request trailers when key != nil.
func engineTrailers(txnID string, op OperationKey, key *IdempotencyKey, receipt []byte) []gitcli.Trailer

type replayOutcome struct {
    kind    replayKind // replayNone | replayFound | replayIDReused | replayInvalidState
    commit  gitcli.ObjectID
    op      OperationKey
    receipt []byte // decoded, validated
}
// scanForRequest: full ancestry from the fetched base — decode base64url,
// validateReceipt, validate multiplicity/shape. Exactly one match with same
// op+digest → replayFound; same ID different op/digest → replayIDReused
// (invalid-input "request-id-reused"); duplicates/malformed/contradictory →
// replayInvalidState. Never picks a winner by commit order.
func (e *Engine) scanForRequest(ctx context.Context, repo gitcli.Repository,
    from gitcli.ObjectID, key *IdempotencyKey) (replayOutcome, error)
```

- [ ] **Step 1: Write failing tests.** Real harness: applied keyed commit carries exactly the five trailers (assert via `git interpret-trailers --parse` on the origin commit; unkeyed commit carries exactly three — never one of the request pair alone); re-`Execute` with the same key after success ⇒ `DispositionAlreadyApplied`, the ORIGINAL receipt bytes (plant a receipt containing an allocated ID; assert the replay returns that ID even after the corpus moved on), `AppliedCommit` = the original commit, and **no new commit** on origin; same ID + different digest ⇒ `KindInvalidInput` detail `request-id-reused`; two hand-crafted history commits with the same request ID (write them via the writer clone with `git commit -m` composed trailers) ⇒ `KindInvalidState`; malformed `Docket-Result` (bad base64 / non-canonical JSON / >4096 decoded) ⇒ `KindInvalidState`; a commit whose body *prose* contains `Docket-Request-ID: x` outside the trailer block does not match; deep history (the key’s commit buried under 30 later commits) is still found — no depth window.
- [ ] **Step 2: Run `go test -count=1 ./internal/repository/transaction/` — expect FAIL.**
- [ ] **Step 3: Implement `idempotency.go`; wire the pre-allocation scan into `Execute`** (scan runs after fetch, before any local allocation — spec order step 2).
- [ ] **Step 4: Run package tests — PASS; `gofmt`/`go vet` clean.**
- [ ] **Step 5: Commit** `feat(transaction): request-ID idempotency — engine trailer block, ancestry scan, replay (0309 task 8)`.

---

### Task 9: PruneAbandoned — ownership-proven recovery

**Files:**
- Modify: `internal/repository/transaction/cleanup.go` (add the recovery half)
- Test: `internal/repository/transaction/recovery_test.go` (and `cleanup_test.go` for the report type)

**Interfaces:**
- Consumes: Tasks 3–5 (ListWorktrees/RemoveWorktree, candidate/manifest/locks); Task 4 `IsAncestor`, `gitcli.ResolveRef`.
- Produces:

```go
type PruneEntry struct {
    ID      string // directory basename as found (may be malformed)
    Verdict string // "pruned" | "live" | "foreign" | "malformed" | "cleanup-failed"
    Pushed  bool   // for pruned: candidate commit already reachable from target
    Detail  string // bounded diagnostic
}
type PruneReport struct{ Entries []PruneEntry } // deterministic order (by ID)

// PruneAbandoned inventories <transactionsRoot> under the registry lock, then
// evaluates each candidate against the spec's six ownership checks — directory
// shape+permissions, supported canonical manifest, repository identity equals
// the CURRENT canonical common dir, candidate+worktree resolve beneath the
// owned root (every symlink hop canonicalized), Git registration absent or
// exactly this worktree, live.lock acquired non-blocking — and deletes ONLY
// candidates passing ALL SIX, validating the complete proof before the first
// destructive step. A held lock is live regardless of timestamp/PID. Everything
// else is reported and left byte-untouched. Never resets a branch, deletes a
// ref, or invokes global worktree prune.
func (e *Engine) PruneAbandoned(ctx context.Context, repo gitcli.Repository) (PruneReport, error)
```

- [ ] **Step 1: Write failing tests** — the spec's ownership and recovery matrix, one assert group each: (a) two concurrently active candidates in one clone (both locks held) both report `live`, both survive; (b) a normally abandoned candidate (manifest valid, lock released, worktree registered) is pruned — worktree deregistered, directory gone — and `Pushed` is correct in both variants (candidate commit pushed vs never pushed: build both, verify via `ResolveRef` + `IsAncestor` before deletion so the report can distinguish); (c) held lock + ancient `CreatedUTC` + PID of a dead process ⇒ `live`, untouched — no age/PID override; (d) byte-untouched survivals, each with verdict: missing manifest, truncated JSON, `Schema: 99`, manifest whose `CommonDir` names a different repo, `WorktreeRel` escaping (`../../x`), candidate root replaced by a symlink to a foreign dir, a foreign-named directory (`not-32-hex`), ambiguous registration (Git registration pointing elsewhere) — for each, hash the candidate tree before/after and require identity; (e) concurrent create-vs-prune: a goroutine holding the registry lock mid-allocation blocks `PruneAbandoned` from observing the half-published dir (channel-ordered proof); (f) forced worktree-removal failure (chmod the worktree dir 000, restore in cleanup) ⇒ `cleanup-failed`, candidate retained with diagnostics; (g) after every scenario, the harness's user checkouts are byte-identical and no global prune ran (a second, unrelated *stale-looking but valid* worktree registration planted in the repo still exists afterward).
- [ ] **Step 2: Run `go test -count=1 ./internal/repository/transaction/` — expect FAIL.**
- [ ] **Step 3: Implement the recovery half of `cleanup.go`.**
- [ ] **Step 4: Run package tests + `-race` — PASS; `gofmt`/`go vet` clean.**
- [ ] **Step 5: Commit** `feat(transaction): PruneAbandoned with six-point ownership proof (0309 task 9)`.

---

### Task 10: Concurrency/interruption matrix, preservation proof, mutation evidence, budgets

**Files:**
- Create: `internal/repository/transaction/concurrency_test.go`, `internal/repository/transaction/interrupt_test.go`, `internal/repository/transaction/preserve_test.go`
- Modify: `tests/runtime-budgets.tsv` (only if measurement demands — see Step 5)

**Interfaces:** Consumes the full engine; produces no new API. Writers are two `Engine`s over two independent clones of one bare origin (never one shared common dir), coordinated by channels/hooks in fake-free real-Git scenarios. To make writer 2 lose deterministically, wrap its `SemanticOperation.Plan` to block on a channel until writer 1's `Execute` returns.

- [ ] **Step 1: Write the concurrency matrix (both topologies — run each scenario against main-mode and docket-mode harnesses):**
  - *Unrelated writers:* both plan from revision A; writer 1 mutates record X and applies; writer 2 (expecting record Y) blocks pre-push, loses the lease, refetches B, replans, applies — final origin tree contains both outcomes; capture writer 2's first-plan bytes and assert they do NOT appear in its second commit (`diff-tree` blob compare); `Attempts == 2`.
  - *Same entity:* both expect blob X1; writer 1 pushes X2; writer 2 ⇒ `DispositionContended`, `ContendedPaths == [X]`, zero commits by writer 2 on origin.
  - *Derived overlap:* both operations regenerate one derived file (e.g. a board-like index listing all records); the loser's replan renders it from fresh state — final derived bytes reflect both primary changes; assert no text-merge artifact (the exact expected rendering, byte-for-byte).
  - *Four lease losses:* a controlled writer advances origin before each push (hook: wrap `Plan` to signal, advance via writer clone, release); exactly 4 attempts, `DispositionContended`, and the transactions root is EMPTY afterward (every candidate cleaned).
- [ ] **Step 2: Write the interruption matrix (`interrupt_test.go`):**
  - *Lost response:* apply a keyed allocating operation, then simulate response loss by re-running the same request against a fresh `Engine` — `already-applied`, original receipt, and exactly one matching receipt commit in origin history (`ScanCommitTrailers` count == 1).
  - *Cancellation:* cancel the context (a) inside `Plan` (operation blocks on ctx), (b) during local Git (deadline via `gitcli.WithLocalTimeout` on a wrapped slow fixture is not injectable — instead cancel between commit and push using an operation-observable barrier in a test-only seam: split `Execute`'s attempt via the existing phase writes and cancel a context wired to expire after `phaseCommitted` is observed by polling the manifest from the test goroutine), (c) before the first fetch (pre-cancelled ctx) — in every pre-push case origin's ref is byte-identical before/after and disposition is `interrupted`/`cancelled`-kinded, never applied.
  - *Materialization failure:* inject failures by constructing plans that trip each verifier — symlinked parent (materialize step), post-materialize corruption is not injectable without a seam, so cover readback/delta by planting a stray file via the operation? No — the operation cannot touch the filesystem: instead call `materializePlan`/`verifyMaterialized`/`verifyActualDelta` directly against a prepared worktree for the step-level failures (Task 6 already does), and at engine level use a plan whose declared file equals base bytes (delta mismatch) plus a hostile-parent plan (containment): assert no push occurred and unrelated bytes exact.
- [ ] **Step 3: Write the preservation proof (`preserve_test.go`):** before every scenario above runs (factor a helper `snapshotCheckouts(t, repos)` / `assertCheckoutsUnchanged(t, snap)` capturing branch/detached state, `HEAD`, full index (`ls-files --stage -z`), staged/unstaged diff bytes, untracked list, and working-tree file hashes for BOTH clones' checkouts): after each transaction only origin refs/objects and `<common>/docket/transactions` state may differ. Wire these two helpers into Tasks' scenarios via shared harness helpers, and add a dedicated dirty-checkout case: the invocation clone has staged AND unstaged AND untracked changes; a transaction applies; all three survive byte-identically.
- [ ] **Step 4: Mutation evidence.** For each safety predicate, apply the mutation, prove it LANDED (re-read the mutated line), run the named focused test with `go test -count=1 -run <Name>`, require RED, then restore from the committed tree (`git diff` must be empty after restore — the working tree holds no uncommitted work at this point, making `git checkout -- <file>` safe; learnings: `mutation-restore-needs-a-backup-copy`). Minimum mutations (spec's list): exact ObjectID equality in the expectation check → same-entity test; drop the undeclared direction of `verifyActualDelta` → stray-file test; drop the declared-but-unchanged direction → delta-mismatch test; replace the explicit pathspec stage with `git add -A` → undeclared-dirty-file commit test; replace `--force-with-lease=<ref>:<expected>` with bare `--force-with-lease` → lease-lost test (must fail to observe the loss); reuse the first plan on retry (cache it) → unrelated-writers first-plan-bytes test; skip the ancestry scan for keyed requests → lost-response test; skip digest comparison → request-id-reused test; disable each of before/after/evolution gates → their gate tests; drop root containment `Lstat` walk → symlinked-parent test; drop manifest `CommonDir` identity check → wrong-repo prune survival test; drop registration-identity check → ambiguous-registration test; skip the live-lock non-blocking acquire → held-lock prune test; invert post-push `IsAncestor` probe classification → cleanup-warning/applied posture test. Record each mutation → red test pairing in the build-evidence notes for the results file.
- [ ] **Step 5: Budgets and full suite.** Measure standalone serial: `scripts/run-tests.sh -j 1 --timings /tmp/t1 tests/test_go_toolchain.sh` and `scripts/run-tests.sh -j 1 --timings /tmp/t2 tests/test_go_race.sh`. `test_go_toolchain.sh` (20s row): if the worst standalone-serial reading exceeds the row, re-budget by the table's own rule (next multiple of 5 plus 5s margin on the worst standalone serial reading — change 0137's rule). `test_go_race.sh` sits AT the 60s **hard ceiling** with `internal/gitcli` already ~49s instrumented; this package's real-git fixtures will add real cost. If its measurement breaches, do NOT raise the row (the ceiling is a relief counter): shard per the file's own header and change 0324's precedent — a sibling `tests/test_go_race_transaction.sh` running `go test -race ./internal/repository/transaction/` with the main race file excluding that package via `go list ./... | grep -v`-derived package list PLUS a completeness guard asserting the two files' package sets partition `go list ./...` exactly (derive, never hand-enumerate — CLAUDE.md's enumerated-floor rule), and re-seed `EXPECTED_TOTAL` in `tests/runtime-budgets.tsv`'s guard if the table gains a row. Either way, record the measured margins as NUMBERS for the results file (learnings: `budget-headroom-is-spent-before-it-is-breached`).
- [ ] **Step 6: Run the full gate** exactly as `finalize.test_command` resolves it (read it from config — it is `scripts/run-tests.sh`): expect green; treat any trailing `OVER BUDGET:` line as a finding to resolve now (Step 5), not noise.
- [ ] **Step 7: Commit** `test(transaction): concurrency/interruption matrix, preservation proof, mutation evidence, budgets (0309 task 10)`.

---

## Self-Review Notes (spec coverage)

- Engine request / vocabulary / plan / receipt rules → Tasks 1–2. Entity expectations → Tasks 1, 7. Tree + StateLoader + both validation gates → Tasks 2, 7. Private root/manifest/locks/ID → Task 5. Attempt order → Task 7. gitcli additions (worktree, changed paths, commit, lease push, reachability, trailers) → Tasks 3–4. Materialization + NUL-safe pathspec + actual-delta guard → Tasks 4, 6. Push tri-state + post-push probe + cleanup-pending posture → Tasks 4, 7. Semantic retry bound + forbidden-retry classification → Task 7. Idempotency + trailer block + ancestry scan → Task 8. Cleanup + PruneAbandoned six checks → Tasks 7, 9. Testing strategy: interface/pure tests → Tasks 1–2, 5–6; real topology harness + independent clones → Task 7; concurrency/interruption matrix → Task 10; ownership/recovery matrix → Task 9; guards/mutation evidence + budgets → Task 10.
- Explicitly NOT here (spec exclusions): no production loader/composer (0310), no workflow operations (0312/0315/0316), no CLI, no config parsing, no branch creation, no GitHub effects, no process supervision.
- Known intentional deviations from the spec's conceptual sketches: `SemanticOperation.Plan` returns `(MutationPlan, OperationResult, error)` — the third return separates programmer failure from domain outcome, which the spec's failure-posture section requires; `StateLoader` is otherwise verbatim. The spec permits file-split latitude ("The exact Go file split may follow package conventions").
