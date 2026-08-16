<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0313 — Workspaces, GitHub PR adapter, and build evidence](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0313-workspaces-github-pr-adapter-and-build-evidence.md)**
<!-- docket:backlink:end -->

# Workspaces, GitHub PR Adapter, and Build Evidence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the typed, ownership-safe, idempotent mechanics for one feature branch: narrow `internal/gitcli` additions (remote-branch probe, absent-lease create push, branch-attached worktree add/attach, non-forcing worktree removal), `internal/workspace` (manifest-owned prepare/inspect/publish/cleanup of one `.worktrees/<slug>` checkout), `internal/githubcli` (typed direct-`gh` adapter with one idempotent ensure-PR operation), and `internal/evidence` (strict build-evidence codec, loss-preserving PR-body upsert, exact-head verifier).

**Architecture:** Three kinds of state stay separate and compose through typed values: `workspace` owns local persistent workspace state (manifest under `<common-dir>/docket/workspaces/`, checkout at `<primary>/.worktrees/<slug>`), `gitcli` owns Git processes, `githubcli` owns `gh` processes and GitHub response decoding, `evidence` owns the trusted record shape. Every external write follows probe → act → verify, so a retry after response loss adopts the effect instead of duplicating it. `unknown` is a load-bearing outcome: an effect may have landed but the postcondition could not be established — never permission to create again.

**Tech Stack:** Go 1.26 stdlib only (`crypto/sha256`, `encoding/json`, `encoding/hex`, `syscall` flock as in `internal/install/lock.go` and `internal/repository/transaction`). Real temporary Git repositories with local bare remotes in tests (pattern: `internal/gitcli/harness_test.go`); all GitHub behavior runs through a protocol-faithful executable fake driven via `WithExecutable` — no live network anywhere in the suite.

**Spec:** `docs/superpowers/specs/2026-08-16-workspaces-github-pr-adapter-and-build-evidence-design.md` (on the `docket` metadata branch; reconciled 2026-08-16). The plan argues from the spec — executors read both.

## Global Constraints

- Module `github.com/danielhanold/docket`, Go 1.26; **no new module dependencies** — stdlib only.
- `gofmt -l` clean, `go vet ./...` clean, `go test ./...` green — enforced by `tests/test_go_toolchain.sh`; `go test -race` green — enforced by `tests/test_go_race.sh` + `tests/test_go_race_transaction.sh`, which auto-discover new packages via `go list ./...`. Do NOT add a second shell producer for these packages.
- Dependency direction (spec §"Package and dependency boundaries"): `workspace` imports `domain` (only `EffectiveBase`/`ChangeID`/slug value semantics) and `gitcli`; `evidence` imports `document` (and `gitcli` only if needed for `ObjectID` validation — prefer a local hex check to keep evidence process-free); `githubcli` imports NONE of `workspace`, `repository`, `repository/transaction`, `config`, `document`, `process`, `install`, `harness`, `cli`, `app`. `internal/app` and `internal/cli` gain no workspace/PR workflow. Existing production files in `internal/repository`, `internal/repository/transaction`, and all other landed packages are byte-unchanged.
- `internal/gitcli` keeps `run` package-private: no exported `Run`, argv escape hatch, caller-selected environment, or generic working-directory escape. New methods are named feature operations using argument arrays, the 0308 sanitized environment, and deadlines. No generic branch create, checkout, reset, clean, rebase, merge, delete, or force-push is exported.
- `githubcli` starts only `gh`, never a shell; repository/branch/title/body values never appear in a shell command; authored Markdown reaches `gh` via stdin (`--body-file -`), never argv or a temp file.
- No production or test path invokes `git worktree prune`, force-removes to make room, resets/deletes a branch, force-pushes, checks out in a shared worktree, or touches the primary checkout, `.docket/`, a transaction worktree, or a sibling feature worktree's index.
- Failures carry operation/stage + a stable kind (`invalid-input`, `invalid-state`, `external`, `cancelled`, `timed-out`, `invalid-output`) and bounded redacted detail — never tokens, env values, PR body bytes, credentialed URLs, or unbounded stderr.
- Every mutation probe and manual re-verification uses `go test -count=1` (learnings: `cached-runner-serves-a-mutated-tree`).
- Git path output is read NUL-delimited (`--porcelain -z` / `--porcelain=v2 -z`), never display form (learnings: `git-path-output-is-quoted`).
- Path identity comparisons canonicalize every symlink hop (`filepath.Abs` + `filepath.EvalSymlinks`, component-checked); macOS `/tmp` → `/private/tmp` makes this observable in every test (learnings: `canonicalise-every-symlink-hop`).
- Documented modes (dirs `0700`, files `0600`) are enforced with explicit `Chmod` after creation and pinned by a test that sets `umask 077` itself (learnings: `promised-file-mode-needs-explicit-chmod`).
- Any probe with a destructive or permissive consumer distinguishes present / cleanly-absent / unknown — an errored probe NEVER shares a branch with clean absence (learnings: `probe-error-is-not-clean-absence`; the 0309 review's important finding). Every such site gets an injected-failure test proving the resource survives.
- Idempotency probes key on the state PROMISED (the exact remote ref / the exact open PR on GitHub), never a local proxy (learnings: `idempotency-keying`, `decide-and-act-on-the-same-copy`). Retries re-derive from a fresh authoritative probe, never a cached response (learnings: `cas-re-read-fresh-origin`).
- Tests coordinate concurrency with channels/flock barriers, never sleeps; no live network — local bare remotes and the fake `gh` only. No test routes through a degrade path silently (learnings: `green-suite-untested-branch`): fake/real call logs are asserted so the hard branch is proven exercised.
- Whole-suite gate: `scripts/run-tests.sh` (the resolved `finalize.test_command`); treat any `OVER BUDGET:` line as a finding to act on; budget decisions follow the table's own rounding rule on worst standalone-serial readings (learnings: `budget-headroom-is-spent-before-it-is-breached`).

## File Structure

```
internal/gitcli/
  remoteref.go        # ProbeRemoteBranch: found/absent/unknown + full ObjectID (ls-remote)
  push.go             # + PushCreateLease: absent-lease create (--force-with-lease=<ref>:)
  worktree.go         # + AddBranchWorktree, AttachBranchWorktree, RemoveWorktreeClean
  remoteref_test.go, push_test.go (extend), worktree_test.go (extend)

internal/workspace/
  target.go           # Target + NewTarget: validation, feat/<slug> derivation, base spend
  result.go           # dispositions (prepare/cleanup/publish), Failure{Op,Stage,Kind}
  manifest.go         # versioned JSON manifest, hashed dir, atomic write, phases, tombstone
  locks.go            # registry.lock + per-workspace operation.lock (flock, non-blocking probe)
  service.go          # Service, NewService, options
  prepare.go          # Prepare: validate → lock → fetch base → inventory → create/resume/adopt
  inspect.go          # Inspect: read-only state classification
  cleanup.go          # Cleanup: proof-gated non-forcing removal, tombstone advance
  publish.go          # PublishHead: probe → create/ff push → verify postcondition
  target_test.go, result_test.go, manifest_test.go, locks_test.go,
  prepare_test.go, inspect_test.go, cleanup_test.go, publish_test.go,
  harness_test.go     # real-repo fixtures: bare origin + primary clone, both topologies
  boundary_test.go    # import boundary + no-forbidden-git-verbs guard

internal/githubcli/
  client.go           # Client, NewClient, options (WithExecutable/timeouts), run (private)
  repo.go             # Repository identity + DiscoverRepository (gh repo view)
  pr.go               # PullRequest value, Version (sha256), JSON decode from documented fields
  ensure.go           # EnsurePullRequest: probe → create/edit → verify
  failure.go          # Failure{Op,Stage,Kind}, redaction
  client_test.go, repo_test.go, pr_test.go, ensure_test.go,
  fakegh_test.go      # protocol-faithful fake gh (TestHelperProcess pattern) + witness tests
  boundary_test.go

internal/evidence/
  record.go           # Record + NewRecord validation
  codec.go            # Render / Extract over document.Parse ("build-evidence" block)
  upsert.go           # Upsert: loss-preserving replace-or-append via document patch API
  verify.go           # Verify: verified/missing/malformed/stale
  record_test.go, codec_test.go, upsert_test.go, verify_test.go, boundary_test.go
```

Budgets: the three Go rows (`test_go_toolchain.sh` 45s, `test_go_race.sh` 60s hard ceiling, `test_go_race_transaction.sh` 45s) absorb four new packages; the workspace real-git matrix is the expensive one under `-race`. Task 12 measures standalone serial and shards a `tests/test_go_race_workspace.sh` (0309/0324 precedent, completeness guard included) if — and only if — measurement demands.

Build-profile mapping (docket-build): Task 3 → economy; Tasks 1, 2, 4, 8, 9, 11 → standard; Tasks 5, 6, 7, 10, 12 → premium (destructive boundaries, idempotent external writes, and the final gate). No task needs max: every decision here is walk-backable behind an unmerged branch.

---

### Task 1: gitcli — remote-branch probe and absent-lease create push

**Files:**
- Create: `internal/gitcli/remoteref.go`, `internal/gitcli/remoteref_test.go`
- Modify: `internal/gitcli/push.go` (add `PushCreateLease` below `PushLease`)
- Test: extend `internal/gitcli/push_test.go`

**Interfaces:**
- Consumes: `Client.run`, `validateRemoteName`/`validateRefName`/`validateObjectID`, `Repository`, `PushOutcome`/`PushDisposition`, `FetchBranch`, `IsAncestor` — all landed.
- Produces (later tasks use these exactly):

```go
// RemoteRefState is a three-outcome probe result. An errored probe is an error
// return, NEVER RemoteRefAbsent (learnings: probe-error-is-not-clean-absence).
type RemoteRefState string
const (
    RemoteRefFound  RemoteRefState = "found"
    RemoteRefAbsent RemoteRefState = "absent"
)
type RemoteRef struct {
    State  RemoteRefState
    Commit ObjectID // full id when found; empty when absent
}

// ProbeRemoteBranch asks the remote authoritatively via
// `git ls-remote --exact <remote> <ref>` (network op, fetch deadline).
// Exactly one matching line => found + full ObjectID; zero lines with exit 0
// => absent; any non-zero exit, malformed line, abbreviated id, or multiple
// lines => *Failure (external / invalid-output), no RemoteRef.
func (c *Client) ProbeRemoteBranch(ctx context.Context, repo Repository, remote RemoteName, ref RefName) (RemoteRef, error)

// PushCreateLease pushes commit to a ref the caller asserts is ABSENT, via
// `git push --porcelain --force-with-lease=<ref>: <remote> <commit>:<ref>`
// (empty expected value = "expect ref to not exist"). Classification mirrors
// PushLease structurally: ok line => applied; '!' rejection => follow-up
// ProbeRemoteBranch — found at a commit != pushed and not containing it =>
// lease-lost (someone created it first); found already == pushed commit =>
// applied (our own lost response); unknown/unprobeable => failed.
func (c *Client) PushCreateLease(ctx context.Context, repo Repository, remote RemoteName, ref RefName, commit ObjectID) (PushOutcome, error)
```

- [ ] **Step 1: Write failing tests.** In `remoteref_test.go`, against the real-git harness (bare origin + clone, pattern from `harness_test.go`): (a) probe an existing branch → `RemoteRefFound` + exact full id equal to the origin's ref; (b) probe a never-created branch → `RemoteRefAbsent`, empty commit; (c) probe with an unconfigured remote name → error, not absent; (d) invalid ref/remote inputs → invalid-request `*Failure`; (e) two refs where one is a prefix of the other (`refs/heads/feat/x` vs `refs/heads/feat/x2`) — probing `feat/x` returns only `feat/x`'s id (proves `--exact`/full-ref matching, not prefix). In `push_test.go`: (f) `PushCreateLease` onto an absent ref → `PushApplied` and origin ref equals the pushed commit; (g) create raced — origin already holds a divergent commit for that ref → `PushLeaseLost` with `Remote` set to the winner; (h) origin already holds EXACTLY the pushed commit (simulated lost response: push once out-of-band, then call) → `PushApplied` (adopt, not duplicate); (i) transport failure (remote URL pointing at a nonexistent path) → `PushFailed`, never lease-lost.
- [ ] **Step 2: Run `go test -count=1 -run 'ProbeRemote|PushCreate' ./internal/gitcli/` — expect FAIL (undefined symbols).**
- [ ] **Step 3: Implement** `remoteref.go` and `PushCreateLease` exactly to the signatures above. `ls-remote` output parse: split lines, each `<40-or-64-hex>\t<refname>`, require the refname byte-equal to the requested fully qualified ref; validate the id with `validateObjectID`.
- [ ] **Step 4: Run `go test -count=1 ./internal/gitcli/` — PASS; `gofmt -l internal/gitcli` empty; `go vet ./internal/gitcli/` clean.**
- [ ] **Step 5: Commit** `feat(gitcli): remote-branch probe and absent-lease create push (0313 task 1)`.

---

### Task 2: gitcli — branch-attached worktree add/attach and non-forcing removal

**Files:**
- Modify: `internal/gitcli/worktree.go`
- Test: extend `internal/gitcli/worktree_test.go`

**Interfaces:**
- Consumes: `Client.run`, validation helpers, `ListWorktrees`, `ResolveRef`.
- Produces:

```go
// AddBranchWorktree creates a NEW local branch at exactly startCommit and
// attaches a new worktree to it, via
// `git worktree add -b <shortBranch> -- <path> <startCommit>`.
// branch must be a fully qualified refs/heads/... name (shortBranch derived by
// stripping the prefix); an already-existing local branch is git's own error
// surfaced as command-failed — this method NEVER passes -B and never resets.
func (c *Client) AddBranchWorktree(ctx context.Context, repo Repository, path string, branch RefName, startCommit ObjectID) error

// AttachBranchWorktree attaches an EXISTING local branch to a new worktree via
// `git worktree add -- <path> <shortBranch>`. Used only by the manifest-proven
// resume path; a missing branch is command-failed, never a create.
func (c *Client) AttachBranchWorktree(ctx context.Context, repo Repository, path string, branch RefName) error

// RemoveWorktreeClean deregisters the worktree at path WITHOUT --force:
// `git worktree remove -- <path>`. Git itself rechecks cleanliness at the
// destructive boundary, closing the check-then-remove race a preflight +
// RemoveWorktree (forced) would leave. A dirty/blocked removal is git's
// non-zero exit surfaced as command-failed with bounded stderr.
func (c *Client) RemoveWorktreeClean(ctx context.Context, repo Repository, path string) error
```

- [ ] **Step 1: Write failing tests** in `worktree_test.go` against the real-git harness: (a) `AddBranchWorktree` with a fresh branch → worktree registered at path (via `ListWorktrees`), symbolic HEAD is the branch, branch tip == startCommit; (b) same call again (branch now exists) → command-failed error, no second worktree, existing branch tip unchanged — proves no `-B`/reset semantics; (c) branch not `refs/heads/`-qualified or relative path → invalid-request; (d) `AttachBranchWorktree` onto an existing branch → registered, attached, HEAD == branch tip; (e) `AttachBranchWorktree` onto a missing branch → command-failed, nothing created; (f) `RemoveWorktreeClean` on a clean attached worktree → deregistered, directory gone, branch STILL EXISTS with tip unchanged; (g) `RemoveWorktreeClean` on a worktree with a dirty tracked file → command-failed, file bytes intact, registration intact; (h) with an untracked file → same refusal (git refuses untracked too), bytes intact; (i) mutation-witness: assert via the harness's recorded git argv log (or by grepping the built source in the boundary test — see Task 12) that no `--force` and no `-B` reach these three operations.
- [ ] **Step 2: Run `go test -count=1 -run 'BranchWorktree|RemoveWorktreeClean' ./internal/gitcli/` — expect FAIL.**
- [ ] **Step 3: Implement** the three methods. `-b` value: `strings.TrimPrefix(string(branch), "refs/heads/")`, refuse when unchanged (not qualified).
- [ ] **Step 4: Run `go test -count=1 ./internal/gitcli/` — PASS; gofmt/vet clean.**
- [ ] **Step 5: Commit** `feat(gitcli): branch-attached worktree add/attach and non-forcing removal (0313 task 2)`.

---

### Task 3: workspace — target vocabulary, closed outcomes, typed failures

**Files:**
- Create: `internal/workspace/target.go`, `internal/workspace/result.go`
- Test: `internal/workspace/target_test.go`, `internal/workspace/result_test.go`

**Interfaces:**
- Consumes: `domain.ChangeID`, `domain.ValidSlugToken`, `domain.BranchForSlug`, `domain.EffectiveBase` + `domain.BaseResolved` (see `internal/domain/stack.go`), `gitcli.RefName` validation.
- Produces (every later workspace task uses these):

```go
package workspace

type Target struct {
    ChangeID   domain.ChangeID
    Slug       string
    FeatureRef gitcli.RefName        // refs/heads/feat/<slug>, derived — never caller-supplied
    Base       domain.EffectiveBase  // must be Kind == domain.BaseResolved
    BaseRef    gitcli.RefName        // refs/heads/<base branch>, validated once here
}

// NewTarget validates and derives. Rejected: non-positive id; invalid slug
// (domain.ValidSlugToken); base kind != BaseResolved; empty/malformed base
// branch (converted once to refs/heads/... and checked by gitcli ref rules —
// an empty branch is NEVER treated as the integration branch). FeatureRef is
// always refs/heads/ + domain.BranchForSlug(slug); no caller override exists,
// so "a feature ref not exactly derived from the slug" is unrepresentable.
func NewTarget(id domain.ChangeID, slug string, base domain.EffectiveBase) (Target, error)

type PrepareDisposition string
const (
    PrepareCreated   PrepareDisposition = "created"
    PrepareExisting  PrepareDisposition = "existing"
    PrepareResumed   PrepareDisposition = "resumed"
    PrepareContended PrepareDisposition = "contended"
    PrepareBlocked   PrepareDisposition = "blocked"
    PrepareFailed    PrepareDisposition = "failed"
)
type CleanupDisposition string
const (
    CleanupCleaned      CleanupDisposition = "cleaned"
    CleanupAlreadyClean CleanupDisposition = "already-clean"
    CleanupBlocked      CleanupDisposition = "blocked"
    CleanupFailed       CleanupDisposition = "failed"
)
type PublishDisposition string
const (
    PublishPublished        PublishDisposition = "published"
    PublishAlreadyPublished PublishDisposition = "already-published"
    PublishContended        PublishDisposition = "contended"
    PublishUnknown          PublishDisposition = "unknown"
    PublishFailed           PublishDisposition = "failed"
)

type Stage string // "validate", "lock", "fetch", "inventory", "allocate",
                  // "worktree", "verify", "manifest", "probe", "push", "remove"
type Kind string
const (
    KindInvalidInput  Kind = "invalid-input"
    KindInvalidState  Kind = "invalid-state"
    KindExternal      Kind = "external"
    KindCancelled     Kind = "cancelled"
    KindTimedOut      Kind = "timed-out"
    KindInvalidOutput Kind = "invalid-output"
)
type Failure struct {
    Op     string // "prepare" | "inspect" | "cleanup" | "publish-head"
    Stage  Stage
    Kind   Kind
    Detail string // bounded, redacted
    Err    error
}
func (f *Failure) Error() string
func (f *Failure) Unwrap() error
func AsFailure(err error) (*Failure, bool)
```

- [ ] **Step 1: Write failing tests.** `target_test.go`, table-driven: accept a valid `(7, "fix-the-thing", BaseResolved{Branch:"main"})` → FeatureRef `refs/heads/feat/fix-the-thing`, BaseRef `refs/heads/main`; reject id 0 and -1; reject slugs `""`, `Fix`, `a_b`, `a b`, `feat/x`, unicode (derive cases from `domain.ValidSlugToken`'s rules, but assert through `NewTarget`); reject every non-`BaseResolved` `EffectiveBaseKind` — iterate the REAL tagged constants from `internal/domain/stack.go` (read them; do not restate a hand list — CLAUDE.md's derive-don't-enumerate rule) and assert each non-resolved kind fails; reject `BaseResolved` with empty branch and with branch `"refs/heads/main"` already qualified twice or containing `..`/space (gitcli ref rules). Resolver-consumption proof (spec §"Feature target"): build four REAL `domain.ResolveEffectiveBase` outcomes — unstacked, live-parent stack, done-parent, recursively stacked-merged (fixture snapshots per `internal/domain/stack_test.go` patterns) — and feed each into `NewTarget`, asserting the constructor spends exactly the resolver's branch. `result_test.go`: `Failure` implements `error`; `Unwrap`/`errors.Is` round-trip; `AsFailure` matches wrapped, misses plain.
- [ ] **Step 2: Run `go test -count=1 ./internal/workspace/` — expect FAIL (package absent).**
- [ ] **Step 3: Implement `target.go` + `result.go`** to the signatures above.
- [ ] **Step 4: Run `go test -count=1 ./internal/workspace/` — PASS; gofmt/vet clean.**
- [ ] **Step 5: Commit** `feat(workspace): target vocabulary, closed outcomes, typed failures (0313 task 3)`.

---

### Task 4: workspace — manifest store, hashed registry, locks, atomic writes

**Files:**
- Create: `internal/workspace/manifest.go`, `internal/workspace/locks.go`
- Test: `internal/workspace/manifest_test.go`, `internal/workspace/locks_test.go`

**Interfaces:**
- Consumes: Task 3 types; flock pattern from `internal/install/lock.go` / `internal/repository/transaction/candidate.go` (read both before writing — reuse the shape, do not import the packages).
- Produces:

```go
type Phase string
const (
    PhaseAllocating Phase = "allocating"
    PhaseReady      Phase = "ready"
    PhaseCleaned    Phase = "cleaned"  // tombstone: retained after cleanup
)

type Manifest struct {
    Schema     int              // 1
    ID         string           // stable workspace id: hex sha256 of feature ref (dir name)
    CommonDir  string           // canonical git common dir (ownership identity)
    ChangeID   domain.ChangeID
    Slug       string
    FeatureRef gitcli.RefName
    BaseRef    gitcli.RefName
    BaseCommit gitcli.ObjectID  // exact fetched base at first preparation
    Path       string           // canonical workspace path
    Phase      Phase
    CreatedUTC string           // RFC3339 seconds, diagnostics only — never an oracle
    UpdatedUTC string
}

// Paths: root = <commonDir>/docket/workspaces; dir = root/<hex sha256(featureRef)>;
// manifest = dir/manifest.json; per-workspace lock = dir/operation.lock;
// registry lock = root/registry.lock. Dirs 0700, files 0600, explicit Chmod.
func workspacesRoot(commonDir string) string
func workspaceDir(commonDir string, ref gitcli.RefName) string

// loadManifest: three-outcome — (m, true, nil) present+valid; (zero, false, nil)
// cleanly absent (os.IsNotExist on the exact path); (zero, false, err) anything
// else: unreadable, truncated JSON, unknown schema, field/identity violations.
// Unknown NEVER reads as absent.
func loadManifest(dir string) (Manifest, bool, error)

// writeManifest: same-directory temp file, write, Sync, Chmod 0600, atomic
// rename, directory sync. Validates before writing; a reload of the written
// bytes must round-trip equal.
func writeManifest(dir string, m Manifest) error

// locks (flock, *os.File based):
func acquireRegistryLock(root string) (release func(), err error)          // blocking, short critical section
func acquireOperationLock(dir string) (release func(), err error)          // blocking: serializes prepare/inspect-refresh/publish/cleanup
func tryOperationLock(dir string) (release func(), held bool, err error)   // non-blocking probe for tests/diagnostics
```

- [ ] **Step 1: Write failing tests.** `manifest_test.go`: (a) round-trip write→load equality for a full manifest; (b) hashed dir name is exactly `hex(sha256("refs/heads/feat/x"))` and differs across refs — no caller-derived branch string in a path component; (c) three-outcome load: absent dir/file → `(false, nil)`; truncated JSON, `Schema: 99`, empty `CommonDir`, phase `"weird"`, invalid `BaseCommit` → error (each case, and NEVER `(false, nil)`); a directory unreadable via chmod 000 (restore in cleanup) → error, not absent; (d) atomicity: after a successful write no `*.tmp` sibling survives; (e) modes: run under `umask 077` AND assert dir `0700`/file `0600` explicitly; add a second write under umask 022 asserting the same (explicit Chmod, not umask luck); (f) phase transition helper rules: `allocating→ready`, `ready→cleaned` allowed; `cleaned→ready`, `ready→allocating` refused. `locks_test.go`: (g) two goroutines contending `acquireOperationLock` on one dir serialize (channel-ordered proof: second acquire observes first release); (h) different workspace dirs do not serialize; (i) `tryOperationLock` reports held while the blocking lock is out, free after release.
- [ ] **Step 2: Run `go test -count=1 ./internal/workspace/` — expect FAIL.**
- [ ] **Step 3: Implement `manifest.go` + `locks.go`.**
- [ ] **Step 4: Run `go test -count=1 ./internal/workspace/` and `go test -race -count=1 -run Lock ./internal/workspace/` — PASS; gofmt/vet clean.**
- [ ] **Step 5: Commit** `feat(workspace): manifest store, hashed registry, locks, atomic writes (0313 task 4)`.

---

### Task 5: workspace — Service and Prepare (fresh allocation), real-Git harness

**Files:**
- Create: `internal/workspace/service.go`, `internal/workspace/prepare.go`, `internal/workspace/harness_test.go`, `internal/workspace/prepare_test.go`

**Interfaces:**
- Consumes: Tasks 1–4; `gitcli.Client` (`Discover`, `FetchBranch`, `ResolveRef`, `ListWorktrees`, `AddBranchWorktree`, `ProbeRemoteBranch`, `IsAncestor`, `ChangedPaths`).
- Produces:

```go
type Service struct { /* git *gitcli.Client, private */ }
func NewService(git *gitcli.Client) (*Service, error)  // nil client is invalid-input

type PrepareRequest struct {
    Repository gitcli.Repository
    Remote     gitcli.RemoteName
    Target     Target
}
type Workspace struct {
    ID          string
    Path        string               // canonical
    FeatureRef  gitcli.RefName
    BaseRef     gitcli.RefName
    BaseCommit  gitcli.ObjectID
    HeadCommit  gitcli.ObjectID
    Dirty       bool                 // reported, never repaired
    Disposition PrepareDisposition
}
// Prepare is safe to call repeatedly; follows spec §"Prepare request and result"
// order 1–8 exactly. Fresh-allocation path (this task): local feature ref,
// remote feature ref, target path, and Git registration must ALL be absent
// before creating; creates one branch-attached worktree at the exact fetched
// base via AddBranchWorktree; reinspects registration/ref/HEAD/ancestry; then
// advances the manifest to ready. Never -B, never reset, never force-remove.
func (s *Service) Prepare(ctx context.Context, req PrepareRequest) (Workspace, error)
```

Harness (`harness_test.go`): builds a bare origin + a primary clone under `t.TempDir()`, with helpers `mainModeRepo(t)` and `docketModeRepo(t)` (orphan `docket` branch + registered `.docket/` worktree + one detached transaction-style worktree + one sibling feature worktree, mirroring `internal/repository/transaction`'s harness shapes), plus `snapshotTree(t, dir) map[string]string` (path→content hash, includes branch/HEAD/index via `git status --porcelain=v2 -z` and `ls-files --stage`) and `assertUnchanged(t, before, dir)` for uninvolved-worktree preservation proofs. The workspace path is `<primary>/.worktrees/<slug>` derived from `Repository.PrimaryWorktree` — never CWD.

- [ ] **Step 1: Write failing tests** (`prepare_test.go`, run each core scenario against BOTH topologies): (a) fresh prepare of an unstacked target (base = fetched `refs/heads/main`) → `PrepareCreated`; assert: worktree registered at canonical `<primary>/.worktrees/<slug>`, symbolic HEAD is `refs/heads/feat/<slug>`, branch tip == the origin main commit AT FETCH TIME (advance origin main after fetch inside the test seam is not injectable — instead pre-advance a local stale tracking ref before calling and assert the prepared base equals origin's CURRENT commit, proving a real fetch, never a cached tracking ref), manifest `ready` with exact `BaseCommit`; (b) live-parent stack target (base = parent's remote branch, built via a real `domain.ResolveEffectiveBase` outcome) starts at the parent branch commit; (c) done-parent target starts at integration; recursively stacked-merged parent resolves through its own base (again: REAL resolver outputs wired through `NewTarget` — the constructor tests in Task 3 prove rejection; these prove consumption); (d) returned `Workspace` facts are reinspected values: corrupt nothing, just assert HeadCommit == branch tip read back via `ResolveRef`; (e) invalid request (mismatched repo identity — a `Repository` whose CommonDir is a different temp repo) → invalid-input before any directory or branch exists (assert `.worktrees` absent, no branch); (f) fetch failure (origin moved/deleted base branch) → external failure, nothing created; (g) preservation: `assertUnchanged` on the primary checkout, `.docket/` worktree, transaction worktree, and the sibling feature worktree for every scenario above.
- [ ] **Step 2: Run `go test -count=1 ./internal/workspace/` — expect FAIL.**
- [ ] **Step 3: Implement `service.go` + `prepare.go`** (fresh path; the resume/existing/blocked inventory arms return typed invalid-state "not yet implemented" failures ONLY if unreachable in this task's tests — otherwise implement the absence checks now: local ref probe via `ResolveRef` (absent = clean), remote via `ProbeRemoteBranch`, path via `os.Lstat`, registration via `ListWorktrees`; each probe three-outcome).
- [ ] **Step 4: Run `go test -count=1 ./internal/workspace/` — PASS; gofmt/vet clean.**
- [ ] **Step 5: Commit** `feat(workspace): Service and fresh-allocation Prepare with real-git harness (0313 task 5)`.

---

### Task 6: workspace — Prepare existing/resume/blocked matrix and Inspect

**Files:**
- Create: `internal/workspace/inspect.go`, `internal/workspace/inspect_test.go`
- Modify: `internal/workspace/prepare.go`; Test: extend `internal/workspace/prepare_test.go`

**Interfaces:**
- Consumes: Tasks 3–5; `gitcli.AttachBranchWorktree`, `gitcli.ChangedPaths`.
- Produces:

```go
type InspectRequest struct {
    Repository gitcli.Repository
    Target     Target
}
type StateKind string
const (
    StateReady      StateKind = "ready"        // internally consistent
    StateCleaned    StateKind = "cleaned"      // tombstone, no registration
    StateResumable  StateKind = "allocating"   // safely resumable partial
    StateDirty      StateKind = "dirty-owned"  // owned but dirty
    StateBranchGone StateKind = "branch-missing"
    StateMismatch   StateKind = "mismatch"     // path/registration/manifest disagree
    StateForeign    StateKind = "foreign"      // no/foreign/malformed manifest
)
type Inspection struct {
    Kind        StateKind
    Phase       Phase
    Path        string
    Registered  bool
    Branch      gitcli.RefName
    BranchHead  gitcli.ObjectID
    HeadCommit  gitcli.ObjectID
    BaseCommit  gitcli.ObjectID
    BaseReached bool     // recorded base reachable from head
    DirtyPaths  []string // exact tracked-dirty + staged + untracked path summary
}
// Inspect is read-only: never deletes, repairs, resets, fetches, or normalizes.
// A malformed or foreign manifest is DATA in Kind/StateForeign, not an error
// that hides state — but an unreadable filesystem/Git probe is still an error.
func (s *Service) Inspect(ctx context.Context, req InspectRequest) (Inspection, error)
```

- [ ] **Step 1: Write failing tests.** Extend `prepare_test.go` (both topologies for the starred ones): (a)* repeated Prepare on a ready workspace → `PrepareExisting`; commits added, staged bytes, dirty tracked bytes, and untracked files created between the calls all survive byte-identically; `Dirty` reported true, nothing repaired; (b)* interrupted-allocation resume: simulate interruption at each boundary by constructing the on-disk state a crash leaves — (i) manifest `allocating`, no branch/worktree → resume creates both, `PrepareResumed`; (ii) manifest + branch at recorded base, no worktree → resume attaches via `AttachBranchWorktree` (witness: the branch tip is NOT moved even when origin advanced meanwhile); (iii) manifest + branch + registered worktree, phase still `allocating` → resume verifies and advances to ready only; commits/dirty bytes made post-creation survive; (c) branch created by this manifest but NOT containing the recorded base commit (simulate by rewriting the local branch out-of-band) → `PrepareBlocked`, untouched; (d)* blocked matrix, each left byte-untouched (snapshot/compare the colliding artifact): pre-existing target directory with no manifest; foreign Git registration at the path; pre-existing LOCAL `feat/<slug>` branch with no manifest; pre-existing REMOTE `feat/<slug>` branch with no manifest (no adoption of pre-Go work); malformed manifest JSON; manifest whose `CommonDir` names a different repo; (e) concurrent Prepare of the SAME target from two goroutines serializes (operation lock) and yields one `created` + one `existing`/`resumed`, exactly one branch and one registration; different targets proceed concurrently; (f) probe-failure injection: make `ListWorktrees` fail (chmod the common dir's `worktrees` listing path unreadable, or point the Client at a broken `git` via `WithExecutable` for one call — a harness sub-Client) → Prepare returns external failure and creates NOTHING (unknown never reads as absent). `inspect_test.go`: one case per `StateKind` — build each state with harness primitives, assert the classification, `DirtyPaths` exactness (staged + unstaged + untracked, sorted), and read-only-ness (full tree snapshot before/after Inspect identical, including mtimes-insensitive content compare); malformed manifest → `StateForeign` with the parse detail in the Inspection, not an error; unreadable manifest dir → error.
- [ ] **Step 2: Run `go test -count=1 ./internal/workspace/` — expect FAIL.**
- [ ] **Step 3: Implement** the inventory/existing/resume/blocked arms in `prepare.go` and all of `inspect.go`.
- [ ] **Step 4: Run `go test -count=1 ./internal/workspace/` and `-race -count=1 -run Concurrent` — PASS; gofmt/vet clean.**
- [ ] **Step 5: Commit** `feat(workspace): prepare existing/resume/blocked matrix and read-only Inspect (0313 task 6)`.

---

### Task 7: workspace — proof-gated Cleanup

**Files:**
- Create: `internal/workspace/cleanup.go`, `internal/workspace/cleanup_test.go`

**Interfaces:**
- Consumes: Tasks 2–6 (`RemoveWorktreeClean`, manifest, locks, inspection internals).
- Produces:

```go
type CleanupRequest struct {
    Repository gitcli.Repository
    Target     Target
}
type CleanupResult struct {
    Disposition CleanupDisposition
    Path        string
    BlockedBy   []string // bounded reasons/paths on blocked
}
// Cleanup removes ONLY the checkout — never a local or remote branch. Requires
// complete manifest + live-Git registration proof, exact feature-ref attachment,
// and an exact clean tracked/untracked delta; then removes via the NON-FORCING
// RemoveWorktreeClean (git rechecks at the destructive boundary), advances the
// manifest to cleaned (tombstone), and returns cleaned. cleaned-manifest + no
// exact registration => already-clean. Everything unproven or dirty => blocked,
// byte-untouched. Never prune, never branch delete, never a sweep.
func (s *Service) Cleanup(ctx context.Context, req CleanupRequest) (CleanupResult, error)
```

- [ ] **Step 1: Write failing tests** (both topologies for a, b): (a) ready + clean workspace → `CleanupCleaned`: registration gone, directory gone, manifest phase `cleaned` retained (tombstone file exists and loads), LOCAL BRANCH STILL EXISTS at its tip; (b) retry after (a) → `CleanupAlreadyClean`, idempotent; (c) blocked matrix, each byte-untouched with the exact artifact hashed before/after: dirty tracked file; staged file; untracked file; unresolved conflict (build via a real failed merge in the workspace); detached HEAD (checkout a commit in the workspace out-of-band); moved HEAD (branch reset out-of-band so HEAD != recorded expectations); registration/path mismatch (deregister + re-register elsewhere); missing manifest; foreign-CommonDir manifest; (d) the check-then-remove race: acquire the flow up to the preliminary status check, then write a file into the workspace from the test before the removal call executes — with the non-forcing primitive git itself refuses; prove by a seam-free construction: call `Cleanup` on a workspace where a file is created AFTER `ChangedPaths` would run but the file exists before `RemoveWorktreeClean` — since the engine is not instrumentable, cover this at the PRIMITIVE level instead (Task 2 test g/h already proves git refuses a dirty removal) AND at the engine level assert cleanup's removal path uses `RemoveWorktreeClean` by mutation: temporarily reroute to forced `RemoveWorktree` and watch test (c-untracked)'s sibling — a workspace made dirty between inspect and remove — lose data; the committed test encodes the observable contract: a dirty workspace is NEVER removed and its bytes survive; (e) probe-failure injection: `ListWorktrees` error during cleanup → `failed` (not `already-clean`), workspace fully intact — the 0309 review's exact defect class (learnings: `probe-error-is-not-clean-absence`); (f) no global prune: plant a second, prunable-looking but valid registration elsewhere in the repo; after every cleanup scenario it still exists; (g) preservation on primary/`.docket`/transaction/sibling worktrees throughout.
- [ ] **Step 2: Run `go test -count=1 ./internal/workspace/` — expect FAIL.**
- [ ] **Step 3: Implement `cleanup.go`.**
- [ ] **Step 4: Run `go test -count=1 ./internal/workspace/` — PASS; gofmt/vet clean.**
- [ ] **Step 5: Commit** `feat(workspace): proof-gated non-forcing Cleanup with tombstone (0313 task 7)`.

---

### Task 8: workspace — PublishHead

**Files:**
- Create: `internal/workspace/publish.go`, `internal/workspace/publish_test.go`

**Interfaces:**
- Consumes: Tasks 1, 5–6 (`ProbeRemoteBranch`, `PushLease`, `PushCreateLease`, `IsAncestor`, inspection internals).
- Produces:

```go
type PublishRequest struct {
    Repository gitcli.Repository
    Remote     gitcli.RemoteName
    Target     Target
}
type PublishResult struct {
    Disposition PublishDisposition
    Head        gitcli.ObjectID // intended local head
    Remote      gitcli.ObjectID // observed remote head when established
}
// PublishHead: reinspect ownership/attachment/HEAD/dirty → refuse dirty or
// inconsistent (failed/invalid-state) → ProbeRemoteBranch → equal already =>
// already-published → absent => PushCreateLease → found+ancestor-of-local =>
// PushLease(expected=observed) → found+divergent => contended, NO force/reset/
// merge/rebase. After any non-conclusive push result, re-probe within ctx
// budget: exact equality => published; different commit => contended;
// unprobeable => unknown. The promise/idempotency key: THE EXACT COMMIT
// REACHED THE EXACT REMOTE FEATURE REF — never a clean tree, local branch,
// upstream config, or exit status (learnings: idempotency-keying).
func (s *Service) PublishHead(ctx context.Context, req PublishRequest) (PublishResult, error)
```

- [ ] **Step 1: Write failing tests** against real bare origins: (a) absent remote ref → `published`, origin ref == local HEAD; (b) repeat → `already-published`, and the origin's reflog/ref shows NO second update (capture origin ref mtime/value before); (c) remote at an ancestor of local (publish, commit locally, publish again) → fast-forward `published` under an expected-old lease — witness the lease by racing: after the first probe result is baked, advance origin out-of-band to a divergent commit, and assert the outcome is `contended`/`failed`, never an overwrite (origin still holds the interloper); construct this via the primitive-level race test in gitcli (Task 1 g) plus an engine-level divergence case: (d) remote holds a commit NOT an ancestor and not equal → `contended`, origin untouched — no force; (e) lost-response adoption: push local HEAD to origin out-of-band (simulating our own push whose response was lost), then PublishHead → `already-published` — keyed on the REMOTE state, not any local marker; (f) dirty workspace → refused (failed, invalid-state), origin untouched; (g) workspace on wrong branch/detached → refused; (h) unprobeable remote (break the remote URL after preparation) → `unknown`, and the result carries no fabricated Remote id; (i) local-proxy mutation guard: the "nothing to do" branch must key on remote equality — assert (e) again after ALSO making the local tree clean and upstream configured, proving those proxies are not consulted (they are absent from the signature — the test documents the promise).
- [ ] **Step 2: Run `go test -count=1 ./internal/workspace/` — expect FAIL.**
- [ ] **Step 3: Implement `publish.go`.**
- [ ] **Step 4: Run `go test -count=1 ./internal/workspace/` — PASS; gofmt/vet clean.**
- [ ] **Step 5: Commit** `feat(workspace): idempotent PublishHead with lease pushes and postcondition probes (0313 task 8)`.

---

### Task 9: githubcli — client, repository identity, PR value, fake gh harness

**Files:**
- Create: `internal/githubcli/client.go`, `internal/githubcli/failure.go`, `internal/githubcli/repo.go`, `internal/githubcli/pr.go`
- Test: `internal/githubcli/client_test.go`, `internal/githubcli/repo_test.go`, `internal/githubcli/pr_test.go`, `internal/githubcli/fakegh_test.go`

**Interfaces:**
- Consumes: nothing from other new packages (boundary: `githubcli` imports no docket package except stdlib; it may NOT import gitcli — head commits arrive as plain validated strings).
- Produces:

```go
package githubcli

type Client struct{ /* private: exe, localTimeout, networkTimeout, baseEnv */ }
type Option func(*clientConfig)
func WithExecutable(path string) Option
func WithLocalTimeout(d time.Duration) Option
func WithNetworkTimeout(d time.Duration) Option
func WithBaseEnvironment(env []string) Option // tests pin env; GH_REPO etc. stripped
func NewClient(opts ...Option) (*Client, error) // resolves gh once (LookPath) unless injected

type Repository struct{ Host, Owner, Name string } // all validated non-empty, no '/', no whitespace
// DiscoverRepository: `gh repo view --json nameWithOwner,owner,name` (plus host
// from gh's context) run FROM dir (the canonical primary worktree) — the ONLY
// call allowed to infer a repository from Git context. Every later call passes
// --repo host/owner/name explicitly.
func (c *Client) DiscoverRepository(ctx context.Context, dir string) (Repository, error)
func (r Repository) Spec() string // "host/owner/name" for --repo

type State string
const (StateOpen State = "open"; StateClosed State = "closed"; StateMerged State = "merged")
type PullRequest struct {
    Number     int
    URL        string
    State      State
    Draft      bool
    HeadBranch string
    HeadCommit string // full GitHub-reported object id, validated 40/64 lowercase hex
    BaseBranch string
    Title      string
    Body       string
    Version    string // "sha256:" + 64 hex over the exact mutable snapshot (see computeVersion)
}
// computeVersion: sha256 over a length-prefixed canonical concatenation of
// (strconv.Itoa(Number), string(State), draft "t"/"f", HeadBranch, HeadCommit,
// BaseBranch, Title, Body) — length prefixes prevent field-boundary collisions.
// Recomputed only from the latest authoritative response; contains no body bytes.

// Failure mirrors workspace.Failure: Op/Stage/Kind six-kind set
// (invalid-input, invalid-state, external, cancelled, timed-out, invalid-output),
// bounded redacted detail. Token-looking values (ghp_/gho_/github_pat_ prefixes,
// Authorization headers) and credentialed URLs in stderr are redacted; stderr
// excerpts bounded (reuse the 0308 excerpt length policy: first 512 bytes,
// lone line count noted).
```

Fake `gh` (`fakegh_test.go`): the `TestHelperProcess` pattern — `TestMain`/helper test re-executes the test binary (`os.Executable()`) with `GO_WANT_FAKE_GH=1`; the fake reads a scenario file (JSON path in `FAKE_GH_SCENARIO`) mapping expected invocations (matched on argv prefix like `pr list`, `pr view`, `pr create`, `pr edit`, `repo view`) to scripted responses `{stdout, stderr, exit, delayMs}`, and appends every invocation — full argv, cwd, complete stdin bytes, selected env keys — to a NUL-delimited witness log (`FAKE_GH_LOG`). Responses carry the exact NESTED JSON field shapes real `gh --json` documents (`number`, `url`, `state`, `isDraft`, `headRefName`, `headRefOid`, `baseRefName`, `title`, `body`) — never a flattened fake-only shape. Helpers `newFakeClient(t, scenario) (*Client, *witnessLog)`.

- [ ] **Step 1: Write failing tests.** `fakegh_test.go` first — the fake's OWN witness tests: (a) pass-through: a scripted `pr view` scenario invoked directly through `Client` yields the scripted stdout decoded; (b) invocation-witness: the log records exact argv, cwd, and stdin bytes; (c) deleting a scenario's dispatch arm (an unmatched invocation) makes the fake exit 64 with a diagnostic — an unexpected call can never silently succeed (a catch-all exit 0 is forbidden); (d) delay + ctx timeout: a `delayMs` beyond `WithNetworkTimeout` produces `timed-out` and the child process is reaped (assert the helper exits, e.g. log file closed/flushed marker absent). `client_test.go`: (e) `NewClient` with no executable and empty PATH → error; injected fake → ok; (f) env hygiene: `GH_REPO`/`GH_HOST` set in the parent are NOT visible to the fake (witness env capture) — discovery aside, retargeting env is stripped by the sanitized base env. `repo_test.go`: (g) discovery decodes host/owner/name from documented fields; missing field / malformed JSON / empty owner → invalid-output, never zero-value identity; (h) discovery runs in the requested dir (witness cwd). `pr_test.go`: (i) decode a full PR from nested documented fields; reject: missing `headRefOid`, abbreviated oid, unknown `state` enum, duplicate PRs where one was requested (list-of-2 for a single-view decode) → invalid-output/invalid-state; (j) `computeVersion` changes when any single field changes and is stable across map ordering; two PRs differing only by (`Title`="ab",`Body`="c") vs (`Title`="a",`Body`="bc") get DIFFERENT versions (length-prefix proof).
- [ ] **Step 2: Run `go test -count=1 ./internal/githubcli/` — expect FAIL.**
- [ ] **Step 3: Implement** `client.go` (private `run` with argv arrays, sanitized env, deadlines, separate stdout/stderr capture, stdin plumb, process reap on cancel — mirror `gitcli/exec.go`'s posture), `failure.go`, `repo.go`, `pr.go`, and the fake helper.
- [ ] **Step 4: Run `go test -count=1 ./internal/githubcli/` — PASS; gofmt/vet clean.**
- [ ] **Step 5: Commit** `feat(githubcli): typed gh client, repository identity, PR value, protocol-faithful fake (0313 task 9)`.

---

### Task 10: githubcli — EnsurePullRequest (probe → act → verify)

**Files:**
- Create: `internal/githubcli/ensure.go`, `internal/githubcli/ensure_test.go`

**Interfaces:**
- Consumes: Task 9 (Client, Repository, PullRequest, fake harness).
- Produces:

```go
type EnsureDisposition string
const (
    EnsureCreated   EnsureDisposition = "created"
    EnsureAdopted   EnsureDisposition = "adopted"
    EnsureUpdated   EnsureDisposition = "updated"
    EnsureUnchanged EnsureDisposition = "unchanged"
    EnsureContended EnsureDisposition = "contended"
    EnsureUnknown   EnsureDisposition = "unknown"
    EnsureFailed    EnsureDisposition = "failed"
)
type EnsurePullRequestRequest struct {
    Repository      Repository
    HeadBranch      string
    ExpectedHead    string // full oid from a successful workspace.PublishHead
    BaseBranch      string // the resolved effective-base branch, never guessed
    Title           string
    Body            string
    ExpectedVersion string // empty => create-or-adopt only; required to update
}
type EnsureResult struct {
    Disposition EnsureDisposition
    PR          PullRequest // verified snapshot on created/adopted/updated/unchanged
}
// Fixed sequence (spec §"Probe, act, verify" 1–7): list ALL states by head
// branch with --repo → ambiguity/terminal blocks (invalid-state) → exact open
// match => adopted/unchanged → differing open PR: version CAS gate => edit or
// contended → no PR => create (body on stdin) → post-mutation verify by number
// AND by head branch, both must agree on one open ready PR equal to the
// request → lost/ambiguous responses => requery; exact => created/updated;
// different => contended; unestablishable => unknown. Wrong GitHub head oid
// (!= ExpectedHead) refuses create AND update. Draft existing PR =>
// invalid-state. Never close/reopen/rollback/second create.
func (c *Client) EnsurePullRequest(ctx context.Context, req EnsurePullRequestRequest) (EnsureResult, error)
```

- [ ] **Step 1: Write failing tests** — all through the fake, each asserting BOTH the disposition/result AND the witness log (which calls happened, which did not — learnings: `green-suite-untested-branch`): (a) no PR → one `pr create` (witness: exactly one) with `--repo host/owner/name`, `--head`, `--base`, `--title`, `--body-file -`; body bytes appear in stdin capture and in NO argv element; post-create verify queries run; result `created` with the verified snapshot; (b) create response lost — model both faces: (b1) same-call recovery: create exits nonzero after the mutation "landed" (the follow-up queries return the matching PR) → `created` (postcondition established by requery); (b2) retry-call recovery: a fresh Ensure call against a scenario already containing the exact PR → `adopted`, witness shows NO create — the spec's lost-create-response path; (c) one exact open PR (head oid, base, title, body, ready state all equal to the request) → no mutation calls; the adopted-vs-unchanged split follows the spec's pairing: `adopted` when `ExpectedVersion` is empty (the lost-create recovery face, same as b2), `unchanged` when a supplied `ExpectedVersion` matches the exact snapshot — assert both tokens reachable; (d) one differing open PR + empty `ExpectedVersion` → `contended`, witness: no edit; + mismatched version → `contended`, no edit, body preserved (scenario unchanged); + matching version → exactly one `pr edit` then verify → `updated`; (e) concurrent change: scenario returns version-X on probe, but the post-edit verify returns a body a human raced in → `contended` reported honestly (no rollback call in witness); (f) edit response lost (edit exits 1, requery shows the edit landed) → `updated`; (g) blocks: two open PRs same head → invalid-state; closed/merged same-head PR with no open PR → invalid-state (no duplicate history); existing draft → invalid-state; GitHub head oid != `ExpectedHead` → refused before any mutation (witness: no create/edit); (h) decode hazards: malformed JSON, missing field, unexpected state enum → invalid-output, no mutation; auth failure (gh exit 4 + stderr) → external with REDACTED bounded stderr (plant a `ghp_...` token and a credentialed URL in the scenario stderr; assert absent from `Failure.Detail`); timeout/cancellation mid-create → requery within budget; if the requery scenario is also dead → `unknown`; (i) every post-discovery invocation in EVERY scenario above carries explicit `--repo` (iterate the whole witness log; single loop, derived not enumerated), including one scenario whose cwd is a different directory that would infer differently.
- [ ] **Step 2: Run `go test -count=1 ./internal/githubcli/` — expect FAIL.**
- [ ] **Step 3: Implement `ensure.go`.**
- [ ] **Step 4: Run `go test -count=1 ./internal/githubcli/` — PASS; gofmt/vet clean.**
- [ ] **Step 5: Commit** `feat(githubcli): idempotent EnsurePullRequest with probe-act-verify recovery (0313 task 10)`.

---

### Task 11: evidence — strict codec, loss-preserving upsert, exact-head verify

**Files:**
- Create: `internal/evidence/record.go`, `internal/evidence/codec.go`, `internal/evidence/upsert.go`, `internal/evidence/verify.go`
- Test: `internal/evidence/record_test.go`, `internal/evidence/codec_test.go`, `internal/evidence/upsert_test.go`, `internal/evidence/verify_test.go`

**Interfaces:**
- Consumes: `document.Parse` (whole-population fence-aware marker validation), the document patch API (`PatchSet.ReplaceBlock`/`InsertBlock`, `Document.Apply`) — block name `build-evidence`. No gitcli import: a local `validHead(s string) bool` (40 or 64 lowercase hex) keeps evidence process-free.
- Produces:

```go
package evidence

type Result string
const ResultGreen Result = "green" // the only valid value; red/interrupted create no record

type Record struct {
    Command string    // non-empty single line, valid UTF-8, no control bytes
    Result  Result
    Head    string    // normalized full lowercase 40- or 64-hex object id
    RanAt   time.Time // UTC, second precision
}
func NewRecord(command, head string, ranAt time.Time) (Record, error) // validates + normalizes (lowercases head, truncates to seconds, converts to UTC)

// Render returns the one canonical complete block (LF line endings):
// <!-- docket:build-evidence:start -->\ncommand:  <c>\nresult:   green\n
// head_sha: <h>\nran_at:   <RFC3339 seconds UTC>\n<!-- docket:build-evidence:end -->
func Render(r Record) string

// Extract: strict parse of an existing PR body. Requires: document.Parse
// succeeds over the WHOLE body (every docket marker balanced, fence-aware);
// exactly one build-evidence block; interior has exactly one line per known key
// (command, result, head_sha, ran_at), no unknown keys, no nonblank extras;
// keys split at the FIRST colon only (commands may contain colons); CRLF and
// LF accepted. Failures are typed: ErrMissing (no block) vs malformed (detail).
func Extract(body []byte) (Record, error)
var ErrMissing = errors.New("evidence: no build-evidence block")

// Upsert: replace only the validated block interior when present (document
// patch ReplaceBlock), or append one canonical block with a deterministic
// blank-line boundary when absent (InsertBlock at end). Validates the ORIGINAL
// whole population first, reparses the candidate (Extract must return exactly
// r), returns no bytes on any failure. Every byte outside the owned block is
// preserved exactly.
func Upsert(body []byte, r Record) ([]byte, error)

type Verdict string
const (
    VerdictVerified  Verdict = "verified"
    VerdictMissing   Verdict = "missing"
    VerdictMalformed Verdict = "malformed"
    VerdictStale     Verdict = "stale"
)
// Verify: Extract; missing => missing; parse failure => malformed; parsed but
// record.Head != the supplied exact full branch HEAD (caller obtains it
// authoritatively via Git, never a body claim) => stale; equal (case-normalized,
// full-length equality — NEVER a prefix test; learnings:
// identity-match-relaxed-to-prefix-is-vacuous) + green => verified.
func Verify(body []byte, head string) Verdict
```

- [ ] **Step 1: Write failing tests.** `record_test.go`: accept a 40-hex and a 64-hex head (normalized lowercase), a command containing `: ` and unicode; reject empty command, `\n`/`\t`/`\x00` in command, 39/41/63/65-hex, uppercase-only-after-normalize is fine (assert normalization), non-UTC input normalized to UTC, sub-second truncated. `codec_test.go`: Render→Extract round-trip byte-exact for both hex widths; Extract matrix — CRLF body accepted; duplicate key, missing key, unknown key `extra: x`, nonblank stray line, `result: red`, two blocks, dangling start, dangling end, out-of-order, NESTED other docket markers malformed anywhere in the body (document.Parse's whole-population rule), a ```-fenced example containing the marker pair is IGNORED (fence-aware: block outside fences parses, fenced copy doesn't confuse it), body with no block → `ErrMissing` (and `errors.Is` works). `upsert_test.go`: (a) replace: a body with backlink block + prose + stale evidence block + findings table → only the evidence interior changes; every other byte identical (compare full prefix/suffix slices); (b) append: body with backlink + prose, no evidence → one canonical block appended with exactly one blank-line boundary; prefix byte-identical; (c) idempotence: Upsert twice with the same record → second output == first; (d) malformed population (a dangling foreign marker) → error, nil bytes, input untouched; (e) reparse-gate mutation: a record whose command would render ambiguously cannot exist (validation), so instead prove the reparse gate by the contract: Extract(Upsert(body,r)) == r for a property-style table including colon-y commands and CRLF originals. `verify_test.go`: verified on exact equal head; stale on different head AND on a head differing only in case pre-normalization... (normalize first — assert verified for uppercase input of the same id); stale on 64-vs-40 of same prefix; missing; malformed; a `result` line that says green but head matches a PREFIX only → stale (full-length equality).
- [ ] **Step 2: Run `go test -count=1 ./internal/evidence/` — expect FAIL.**
- [ ] **Step 3: Implement** the four files.
- [ ] **Step 4: Run `go test -count=1 ./internal/evidence/` — PASS; gofmt/vet clean.**
- [ ] **Step 5: Commit** `feat(evidence): strict build-evidence codec, loss-preserving upsert, exact-head verify (0313 task 11)`.

---

### Task 12: Boundary guards, CWD/symlink matrix, mutation evidence, budgets, whole-suite gate

**Files:**
- Create: `internal/workspace/boundary_test.go`, `internal/githubcli/boundary_test.go`, `internal/evidence/boundary_test.go`
- Modify: `internal/workspace/prepare_test.go` (CWD/symlink matrix), `tests/runtime-budgets.tsv` (only if measurement demands), possibly create `tests/test_go_race_workspace.sh` (only if measurement demands)

**Interfaces:** Consumes everything; produces no new API.

- [ ] **Step 1: Boundary tests** (pattern: `internal/repository/transaction`'s 0309 import-boundary guard — read it first, mirror its mechanism): parse each package's non-test imports via `go/parser` (or `go list -f`): `workspace` may import only stdlib + `internal/domain` + `internal/gitcli`; `githubcli` only stdlib; `evidence` only stdlib + `internal/document`; and the REVERSE direction: no landed production package (`app`, `cli`, `repository`, `repository/transaction`, `config`, `install`, `harness`, `document`, `domain`, `gitcli`) imports any of the three new packages (iterate `go list ./internal/...` — derived, never hand-enumerated). Also a shape guard on gitcli's new methods: grep the package source asserting no `worktree add` argv contains `-B` and no `worktree remove` argv built by `RemoveWorktreeClean`/`AddBranchWorktree`/`AttachBranchWorktree` contains `--force` (anchor on the argv literals in the named functions; the existing forced `RemoveWorktree` keeps its documented `--force`). Verify each boundary test reddens: temporarily add a forbidden import / a `--force` literal, run `go test -count=1 -run Boundary`, observe RED, revert.
- [ ] **Step 2: CWD/symlink invocation matrix** (spec §"Real-Git workspace matrix" first bullet): in `prepare_test.go`, run Prepare with `gitcli.Discover` seeded from (a) the primary checkout, (b) inside `.docket/`, (c) inside another feature worktree, (d) a nested subdirectory, (e) a symlinked spelling of the primary path (`t.TempDir()`-based symlink; macOS `/tmp`→`/private/tmp` gives a second free case) — all five resolve ONE canonical workspace location and the same manifest identity (same hashed dir, `PrepareExisting` after the first).
- [ ] **Step 3: Mutation evidence.** For each safety predicate, apply the mutation, PROVE it landed (re-read the mutated line), run the named focused test with `go test -count=1 -run <Name>`, require RED, restore from the committed tree only when `git status` shows no other uncommitted work (learnings: `mutation-restore-needs-a-backup-copy`). Minimum set: drop the remote-feature-ref absence check in fresh Prepare → pre-existing-remote-branch blocked test; drop manifest `CommonDir` identity check → foreign-manifest test; swap `RemoveWorktreeClean` for forced `RemoveWorktree` in cleanup → dirty-cleanup byte-survival test; collapse a `ListWorktrees` error to "absent" in cleanup → probe-injection test (Task 7e); drop the post-push re-probe in PublishHead → lost-response adoption test; relax the ensure head-oid equality to prefix → wrong-head refusal test; drop `--repo` from one post-discovery call → witness-log iteration test; skip Upsert's original-population validation → dangling-marker test; relax Verify equality to `strings.HasPrefix` → prefix-stale test; delete the fake gh's unmatched-invocation exit-64 arm → fake witness test (c). Record each mutation → red pairing for the results file.
- [ ] **Step 4: Budgets.** Measure standalone serial on this machine: `scripts/run-tests.sh -j 1 --timings "${TMPDIR:-/tmp}/t1.XXXXXX-resolved" tests/test_go_toolchain.sh`, same for `tests/test_go_race.sh` and `tests/test_go_race_transaction.sh` (use real mktemp-templated paths). `test_go_toolchain.sh` (45s row): re-budget by the table's rule (next multiple of 5 + 5s margin on the worst standalone-serial reading) if exceeded. `test_go_race.sh` (60s HARD ceiling — a relief counter, never raised): the workspace real-git matrix instrumented under `-race` is the likely breacher; if it breaches, shard `tests/test_go_race_workspace.sh` running `go test -race ./internal/workspace/` and derive the main race file's exclusion from `go list` exactly as `test_go_race_transaction.sh` did, EXTENDING the existing partition-completeness guard to three shards (the guard proves the shards partition `go list ./...` exactly — derived, never enumerated), add the new row measured by the same rule, and re-seed the table's `EXPECTED_TOTAL` guard. Record all margins as NUMBERS for the results file.
- [ ] **Step 5: Whole-suite gate.** Run the resolved `finalize.test_command` (read it from config — it resolves to `scripts/run-tests.sh`), backgrounded to a stable log with a blocking monitor, keyed on exit code. Expect green; treat any `OVER BUDGET:` line as a Step 4 finding to resolve now.
- [ ] **Step 6: Commit** `test(0313): boundary guards, invocation matrix, mutation evidence, budgets (task 12)`.

---

## Self-Review Notes (spec coverage)

- Acceptance boundary 1 (prepare/resume/inspect/clean one manifest-owned checkout at the resolved base) → Tasks 3–7, 12. Boundary 2 (exact-head publication, no force, response-loss adoption, divergence never overwritten) → Tasks 1, 8. Boundary 3 (one exact PR created/adopted/versioned-updated against the fake, probe after every ambiguity, no duplicate on retry) → Tasks 9–10. Boundary 4 (evidence round-trip, byte preservation, green-at-exact-head verify) → Task 11. Boundary 5 (uninvolved state byte-identical) → preservation asserts woven through Tasks 5–8 plus Task 12's matrix.
- Spec sections: shared typed vocabulary → Tasks 3, 9, 11; location/durable ownership + manifest/locks → Task 4; prepare order 1–8 → Tasks 5–6; inspect kinds → Task 6; clean removal → Task 7 (non-forcing primitive: Task 2); feature-branch publication + gitcli primitive list → Tasks 1, 2, 8; gh client posture + repository identity + PR value/version → Task 9; ensure sequence 1–7 → Task 10; evidence record/update/verify → Task 11; failure/cancellation/recovery table → distributed per-effect tests (Tasks 5–8, 10) with probe-injection cases; testing strategy (pure/real-git/fake-gh/whole-suite) → every task's Step 1 plus Task 12; live GitHub boundary → deliberately NOT tested live (0317's release gate) — the fake is the deterministic seam, and `ensure.go`'s doc comment must state the exact opt-in smoke request 0317 will run against a disposable repository (add that comment in Task 10 Step 3).
- Known intentional shapes: `Target` carries a derived `BaseRef` field beyond the spec's sketch (validated once at construction — the spec requires exactly this conversion, and later tasks consume it); `NewTarget` derives `FeatureRef` rather than accepting one, making the spec's "feature ref not exactly derived from slug" rejection unrepresentable instead of checked; `Verify` takes the body (calling `Extract` itself) rather than a pre-parsed record, so missing/malformed/stale map from one entry point — `VerifyExact`'s three spec clauses are all inside it. The spec permits file-split latitude ("The exact Go file split may follow package conventions").
- Explicitly NOT here (spec exclusions): no CLI/app workflow (0315), no process/gate supervision (0314), no rebase/merge/finalize/branch deletion/multi-workspace sweep (0316), no release/live acceptance (0317), no draft conversion/review/checks/comments, no metadata writes (`pr:`, `status:`, board), no strict-ancestor evidence exemption (0316).
