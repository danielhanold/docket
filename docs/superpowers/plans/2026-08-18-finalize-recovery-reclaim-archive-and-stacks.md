<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0316 — Finalize, recovery, reclaim, archive, and stacks](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-19-0316-finalize-recovery-reclaim-archive-and-stacks.md)**
<!-- docket:backlink:end -->
# Finalize, Recovery, Reclaim, Archive, and Stacks — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the terminal half of docket's Go migration: authoritative finalize context and selection, owned rebase/local-gate/publish/merge composition, atomic terminal metadata closeout for ordinary/stacked/root outcomes in both repository modes, persistent halt/finalize-blocked recovery, explicit and policy reclaim, merged-PR maintenance, terminal backlink repair, and ownership-safe cleanup — plus the revised skill assets that sequence it all.

**Architecture:** Claude stays the workflow controller; Go exposes small resumable checkpoint operations (`context finalize`, `finalize retarget-children|rebase|rebase-continue|rebase-abort|publish|block|clear-block|merge|closeout|cleanup`, `change halt|resume-halted|reclaim`, `maintenance sweep`, `gate cleanup`) whose promises are independently probeable. `internal/cli` parses and presents; `internal/app` pins authoritative context, checks cross-package preconditions, calls one named effect, maps closed outcomes; new narrow primitives go into `internal/gitcli` (owned rebase, ref deletion), `internal/githubcli` (retarget, comment, expected-head merge, merged reprobe), and `internal/workspace` (rebase receipt, rewrite publish). Every metadata write goes through the landed `repository/transaction.Engine` with exact entity versions. No central phase machine, no generic Git/`gh` runner, no new force-push escape hatch beyond the receipt-scoped lease.

**Tech Stack:** Go (Cobra CLI, protocol-v1 one-shot JSON), real-git integration tests with disposable repos + bare remotes in both `main` and `docket` metadata modes, protocol-faithful executable fake `gh`, bash suite runner `scripts/run-tests.sh`.

**Spec:** `docs/superpowers/specs/2026-08-18-finalize-recovery-reclaim-archive-and-stacks-design.md` — the authority; this plan argues from it. Read it before any task.

## Global Constraints

Copied from the spec; every task's requirements implicitly include these.

- Operation spellings are fixed (spec "Package and command boundaries") and shown in the Architecture line above. Scalar identities travel as flags; authored conflict/repair/halt/block/retarget-authorization content travels through a bounded request file or stdin, never command-line interpolation, and never into Git or `gh` argument strings.
- `internal/cli` contains **no** lifecycle, Git, GitHub, stack, reclaim, or cleanup policy. `internal/app` does not retain a workflow cursor or dispatch agents. `internal/domain` stays free of filesystems, documents, Git, GitHub, processes, and clocks except injected values.
- No generic command runner, checkout/reset/clean, arbitrary force push, arbitrary branch delete, or unversioned PR mutation is exported from `internal/gitcli`/`internal/githubcli`/`internal/workspace`. The rebase rewrite may force only through the exact old-value lease recorded by the owned attempt.
- Every metadata mutation: capability preflight, reload fresh origin, exact entity versions, atomic rendering of all affected derived views (artifact block, backlinks, inline board), whole-repository validation, expected-ref lease push. A semantic retry reapplies intent only while preconditions still hold; incompatible divergence is `contended`.
- Every external-effect probe has three outcomes — present, cleanly absent, and unknown — and `unknown` never shares a branch with `absent` when the other branch is destructive; on unknown, retain (learning `probe-error-is-not-clean-absence`). `unknown` never authorizes merge, overwrite, create, retarget, or delete.
- Every nothing-to-do/idempotency probe keys on the state the operation PROMISED (remote ref, GitHub snapshot, canonical archive record + transaction receipt), never a local proxy a half-completed run also leaves behind (learning `idempotency-keying`). The decision and the action read the same authoritative copy (learning `decide-and-act-on-the-same-copy`).
- A merge is never rolled back. Post-merge failures leave a verified recoverable closeout; cleanup and generated-link repair are independent retryable suffixes, never evidence the merge/archive succeeded.
- Child-agent (resolver/repair) returns are authored hints; Go verifies every mechanical claim against live Git index, rebase state, commits, gate record, remote refs, PR snapshot, and metadata before the next effect.
- Marker-bound sections/blocks validate whole-population order and balance before replacement (AGENTS.md); unknown frontmatter and unowned authored bytes stay byte-identical. Returned diagnostics redact report bodies, credentials, env values, credentialed URLs, unbounded stderr; byte-bounded truncation backs off to a rune boundary.
- Archive relocation compares surviving holders by content identity, not path (learning `relocation-reads-as-identity-reuse`); the move reuses the kill path's relocation machinery (one plan: `MutationCreate` at the archive path + delete of the active path — see `internal/app/change_kill.go`, "Archive move" comment), never a second permissive writer.
- Deferred capabilities stay deferred: no CI/`both` gate, no results-only strict-ancestor skip, no terminal publishing, no learning harvest, no auto-capture, no per-repo routing, no skill rebinding, no new lifecycle status. Mutating operations rerun capability preflight and return `unsupported-config` before any effect when a deferred capability is requested.
- Mutation-test every new guard (AGENTS.md "Guards and tests") and defeat Go's test cache: `go test -count=1` (learning `cached-runner-serves-a-mutated-tree`).
- The build gate runs the whole resolved suite via `scripts/run-tests.sh` (`finalize.test_command`); Go tests wire in through `tests/test_go_toolchain.sh`. Any **new** `tests/test_*.sh` file needs a `tests/runtime-budgets.tsv` row (learning `budget-headroom-is-spent-before-it-is-breached`). Treat every `OVER BUDGET:` line as a finding requiring action.
- Skill prose ships into consuming repos: no sentence true only in this repo (learning `distributed-body-has-no-local-repo`); every prohibition added to a closed-vocabulary contract names the return it maps to (learning `prohibition-needs-a-return-value`).
- Nothing allocated to 0305–0315, 0317–0318, 0322, or 0326: no reimplementation of landed foundations, no release packaging, no self-hosting cutover, no bootstrap/adoption, no config contraction. End-to-end tests build `./cmd/docket` to an explicit temporary path and invoke it by absolute path against disposable repos with hermetic isolated configuration; no test depends on a PATH `docket` or on Go verbs in `docket.sh`.

## File Structure

```
internal/domain/finalize.go(+_test)        finalize eligibility, ordering, skip reasons, merge conjunct policy
internal/domain/stackcloseout.go(+_test)   root-carry derivation: proven descendant closeout set from PR-destination chain
internal/gitcli/rebase.go(+_test)          owned rebase begin/state/stage/continue/abort, owned temp refs
internal/gitcli/refdelete.go(+_test)       checked local branch delete, remote ref delete under exact lease
internal/githubcli/retarget.go(+_test)     probe/act/verify base retarget of one exact versioned PR
internal/githubcli/comment.go(+_test)      idempotent marker-keyed PR comment ensure + probe
internal/githubcli/merge.go(+_test)        expected-head merge, authoritative merged reprobe (mergedAt, merge commit)
internal/workspace/rebasereceipt.go(+_test) ownership-scoped rebase receipt beside the manifest; owned refs bookkeeping
internal/workspace/rewrite.go(+_test)      narrow force-with-lease rewrite publish (consumes the receipt)
internal/app/finalize_context.go(+_test)   docket context finalize
internal/app/finalize_retarget.go(+_test)  finalize retarget-children
internal/app/finalize_rebase.go(+_test)    finalize rebase / rebase-continue / rebase-abort + local-gate composition
internal/app/finalize_publish.go(+_test)   finalize publish (rewrite push + PR evidence block update)
internal/app/finalize_block.go(+_test)     finalize block / clear-block
internal/app/finalize_merge.go(+_test)     finalize merge
internal/app/finalize_closeout.go(+_test)  finalize closeout (ordinary, stacked-merged, root carry) + backlink leg
internal/app/finalize_cleanup.go(+_test)   finalize cleanup + gate-run retention
internal/app/change_halt.go(+_test)        change halt / resume-halted (+ run verify run-halted verdict)
internal/app/change_reclaim.go(+_test)     change reclaim
internal/app/maintenance.go(+_test)        maintenance sweep
internal/cli/finalize.go(+_test)           finalize command tree
internal/cli/maintenance.go(+_test)        maintenance command tree; gate cleanup subcommand lands in cli/gate.go
skills/docket-finalize-change/…            revised sequencer skill
skills/docket-implement-next/…             halt/resume additions
internal/assets/embedded/…                 regenerated embedded copies
tests/test_go_finalize_e2e.sh              suite wiring for the new hermetic e2e file (if not already covered by test_go_toolchain.sh)
```

Repository-mode note (learning `metadata-branch-invisible-to-suite`): hermetic suites see only their disposable fixtures; anything about docket's own live metadata branch is verified at build time and recorded in the results file, never asserted by a test.

---

### Task 1: Domain finalize eligibility, ordering, and merge conjuncts

**Files:**
- Create: `internal/domain/finalize.go`, `internal/domain/finalize_test.go`

**Interfaces:**
- Consumes: `domain.Snapshot`, `domain.Change`, `domain.StackChildren`, `domain.ResolveEffectiveBase`, existing `domain` lifecycle vocabulary.
- Produces (later app tasks depend on these exact names):

```go
type PRFacts struct {
    Number, Version        string
    State                  string // "open" | "closed" | "merged" | "unknown"
    Draft, Approved        bool
    Mergeable              string // "MERGEABLE" | "CONFLICTING" | "UNKNOWN"
    HeadOID, BaseRef       string
    ChangedFiles, DiffLines int
    MergedAtUTC            string // RFC3339 or ""
    MergeCommit            string
}
type FinalizeCandidate struct {
    ID         ChangeID
    Band       string // "merged-recovery" | "mergeable" | "conflicting" | "unknown"
    SkipReason string // "" when actionable; else closed reason token
}
// Ordering: merged-recovery first (closeout work), then dependency-eligible open PRs:
// MERGEABLE before CONFLICTING/UNKNOWN, then smaller ChangedFiles, smaller DiffLines,
// then priority, created date, id. Deterministic and nil-safe.
func SelectFinalizeQueue(s Snapshot, facts map[ChangeID]PRFacts, blocked map[ChangeID]bool, allowlist []ChangeID) []FinalizeCandidate
// Closed skip reasons: "not-implemented", "draft", "pr-closed", "approval-required",
// "finalize-blocked", "dependency-unmerged", "malformed", "pr-unknown".
type MergeConjuncts struct {
    Implemented, PRIdentityMatch, HeadsAgree, OpenNonDraft,
    BaseIsEffectiveBase, GateSatisfied, ApprovalSatisfied,
    NoOpenChildren, NotSuperseded bool
}
func (m MergeConjuncts) AllHold() bool
func (m MergeConjuncts) FirstFailure() string // closed token per field, "" when all hold
```

- [ ] **Step 1: Write failing tests** in `internal/domain/finalize_test.go`:
  - `TestSelectFinalizeQueueOrdering` — table of candidates covering: already-merged PR sorts first as `merged-recovery`; MERGEABLE before CONFLICTING; CONFLICTING and UNKNOWN in the same band after MERGEABLE; changed-file then line tiebreaks; priority/date/id tail; identical inputs yield identical output across two calls (determinism).
  - `TestSelectFinalizeQueueSkipReasons` — one row per closed skip reason above; assert exact token and that a skipped candidate still appears (surfaced, not omitted).
  - `TestSelectFinalizeQueueExplicitOverride` — approval-required and finalize-blocked are skip reasons in auto mode, but the test asserts the tokens exist so the app layer can override them for an explicit `--id` (the override itself is app policy, Task 10).
  - `TestSelectFinalizeQueueAllowlist` — allowlist bounds membership without reordering survivors.
  - `TestSelectFinalizeQueueDependencyOrder` — a change whose `depends_on` names an unmerged change gets `dependency-unmerged`.
  - `TestSelectFinalizeQueueNilSafe` — nil facts map, nil allowlist, empty snapshot: empty slice, no panic.
  - `TestMergeConjunctsFirstFailure` — each false field maps to its distinct closed token; all-true → `AllHold()` and `""`.
- [ ] **Step 2: Run red:** `go test ./internal/domain/ -run 'TestSelectFinalizeQueue|TestMergeConjuncts' -count=1` — expect compile failure.
- [ ] **Step 3: Implement** `internal/domain/finalize.go`. Pure functions over injected facts; follow the style of `internal/domain/selection.go` (grep `SelectQueue` for the sort/stability idiom).
- [ ] **Step 4: Green:** same command. Then `go test ./internal/domain/ -count=1`.
- [ ] **Step 5: Commit:** `git add internal/domain/finalize.go internal/domain/finalize_test.go && git commit -m "feat(0316): domain finalize eligibility, ordering, and merge conjuncts"`

### Task 2: Domain stack-root closeout derivation

**Files:**
- Create: `internal/domain/stackcloseout.go`, `internal/domain/stackcloseout_test.go`

**Interfaces:**
- Consumes: `domain.Snapshot`, `StackDescendantsParentFirst`, `MarkDone`, `MarkStackedMerged`, `PRFacts` (Task 1).
- Produces:

```go
type CarriedDescendant struct {
    ID     ChangeID
    Proof  string // "" when proven; else closed refusal token
}
// Verifies, per descendant, the chain of merged PR destinations that carried its
// stacked-merged code into the root. Closed refusal tokens: "not-stacked-merged",
// "destination-mismatch", "pr-unknown", "chain-broken", "cycle", "killed-ancestor".
func DeriveRootCloseoutSet(s Snapshot, root ChangeID, facts map[ChangeID]PRFacts) ([]CarriedDescendant, *PolicyFailure)
// All-or-nothing: any unproven descendant fails the whole set (the caller keeps the
// root recoverable rather than writing false descendant done records).
func RootCloseoutProven(set []CarriedDescendant) bool
```

- [ ] **Step 1: Write failing tests** `internal/domain/stackcloseout_test.go`:
  - `TestDeriveRootCloseoutSetHappyChain` — 3-level stack, every descendant `stacked-merged` with a merged PR whose base is its parent's branch: all proven, parent-first order.
  - `TestDeriveRootCloseoutSetRefusals` — table: descendant still `in-progress` → `not-stacked-merged`; merged into wrong branch → `destination-mismatch`; missing/unknown PR facts → `pr-unknown`; a gap in the chain → `chain-broken`; cyclic `stacked_on` → `cycle` (no infinite loop); killed ancestor mid-chain → `killed-ancestor`.
  - `TestRootCloseoutProvenAllOrNothing` — one unproven entry → false.
  - `TestDeriveRootCloseoutSetIgnoresRenderedState` — snapshot where a rendered-table-style relation would disagree with `stacked_on` graph: the graph wins (no descendant promoted from a branch name or table).
- [ ] **Step 2: Run red:** `go test ./internal/domain/ -run 'RootCloseout' -count=1`.
- [ ] **Step 3: Implement.** Reuse the cycle guard idiom from `internal/domain/stack.go` (`StackDescendantsParentFirst`).
- [ ] **Step 4: Green**, then `go test ./internal/domain/ -count=1`.
- [ ] **Step 5: Commit:** `git commit -m "feat(0316): domain stack-root closeout derivation"` (add both files).

### Task 3: gitcli owned rebase and checked ref deletion

**Files:**
- Create: `internal/gitcli/rebase.go`, `internal/gitcli/rebase_test.go`, `internal/gitcli/refdelete.go`, `internal/gitcli/refdelete_test.go`

**Interfaces:**
- Consumes: existing `gitcli.Client`, `Repository`, `RefName`, `ObjectID`, `PushLease`, worktree helpers.
- Produces:

```go
type RebaseDisposition string // "unchanged" | "rebased" | "conflicted" | "in-progress-foreign" | "failed"
type RebaseStatus struct {
    Disposition   RebaseDisposition
    HeadOID       ObjectID
    UnmergedPaths []string // repo-relative, NUL-read (git paths are C-quoted by default)
}
// Begin rebases worktreeDir's checked-out branch onto exact baseOID; requires clean tree,
// exact expected current head. Creates owned refs refs/docket/finalize/<id>/orig and /base first.
func (c *Client) BeginRebase(ctx context.Context, worktreeDir string, expectedHead, baseOID ObjectID, ownedRefPrefix string) (RebaseStatus, error)
func (c *Client) RebaseState(ctx context.Context, worktreeDir string) (RebaseStatus, error) // read-only; detects foreign/absent/in-progress
// Stages ONLY the given repo-relative paths (validated against live unmerged entries by the caller),
// then continues non-interactively (GIT_EDITOR=true).
func (c *Client) StageAndContinueRebase(ctx context.Context, worktreeDir string, paths []string) (RebaseStatus, error)
// Aborts and verifies HEAD returned to origHead; error if verification fails.
func (c *Client) AbortRebase(ctx context.Context, worktreeDir string, origHead ObjectID) error
func (c *Client) SetOwnedRef(ctx context.Context, repo Repository, ref RefName, oid ObjectID) error   // refuses refs outside refs/docket/
func (c *Client) DeleteOwnedRef(ctx context.Context, repo Repository, ref RefName) error              // same fence
// Deletes a local branch only when tip equals expectedTip AND the branch is not checked out anywhere.
func (c *Client) DeleteLocalBranchChecked(ctx context.Context, repo Repository, branch RefName, expectedTip ObjectID) error
// Remote delete under exact old-value lease (push :ref with --force-with-lease=<ref>:<expectedTip>).
func (c *Client) DeleteRemoteRefLease(ctx context.Context, repo Repository, remote RemoteName, ref RefName, expectedTip ObjectID) (PushOutcome, error)
```

- [ ] **Step 1: Write failing real-git tests** (follow `internal/gitcli/worktree_test.go` fixture style: `t.TempDir()` repos, bare remotes):
  - `TestBeginRebaseNoop` — feature already on base → `unchanged`, owned refs created, head unchanged.
  - `TestBeginRebaseRewrites` — divergent base → `rebased`, new head not equal orig, orig ref preserves the pre-rebase commit.
  - `TestBeginRebaseConflict` — conflicting edit → `conflicted`, `UnmergedPaths` exact (include a path with a space to exercise NUL reading).
  - `TestBeginRebaseRefusals` — dirty tree, wrong expectedHead, existing in-progress rebase → error / `in-progress-foreign`; tree untouched (assert head and status unchanged after).
  - `TestStageAndContinueRebaseMultiConflict` — two sequential conflicts; first continue returns the second conflict; second completes.
  - `TestAbortRebaseRestoresOrig` — abort mid-conflict; HEAD equals orig; error if orig mismatched.
  - `TestOwnedRefFence` — `SetOwnedRef`/`DeleteOwnedRef` refuse `refs/heads/…`.
  - `TestDeleteLocalBranchChecked` — deletes on exact tip; refuses on moved tip; refuses while checked out in a worktree.
  - `TestDeleteRemoteRefLease` — deletes on exact tip; concurrent move → rejected outcome, ref retained; already-absent probe distinct from probe error (three outcomes).
- [ ] **Step 2: Red:** `go test ./internal/gitcli/ -run 'Rebase|OwnedRef|Delete' -count=1`.
- [ ] **Step 3: Implement.** Use `c.run`-style exec plumbing from `internal/gitcli/exec.go`; no new generic runner. Detect in-progress rebase via `rev-parse --git-path rebase-merge` / `rebase-apply` existence.
- [ ] **Step 4: Green**, then `go test ./internal/gitcli/ -count=1`.
- [ ] **Step 5: Commit:** `git commit -m "feat(0316): gitcli owned rebase and checked ref deletion primitives"` (add the four files).

### Task 4: githubcli retarget, idempotent comment, expected-head merge

**Files:**
- Create: `internal/githubcli/retarget.go`, `internal/githubcli/retarget_test.go`, `internal/githubcli/comment.go`, `internal/githubcli/comment_test.go`, `internal/githubcli/merge.go`, `internal/githubcli/merge_test.go`
- Modify: `internal/githubcli/fakegh_test.go` (extend the executable fake with `pr edit --base`, `pr comment`, comment listing, `pr merge --match-head-commit`, mergedAt/mergeCommit fields, lazy `UNKNOWN` mergeability, response-loss and permission-denied modes)

**Interfaces:**
- Consumes: existing `Client.run`, `PullRequest`, `computeVersion`, probe helpers.
- Produces:

```go
// Probe/act/verify retarget of one PR onto newBase; refuses when the live PR version differs.
type RetargetOutcome string // "retargeted" | "already" | "contended" | "unknown"
func (c *Client) RetargetPullRequest(ctx context.Context, repo Repository, number int, expectedVersion, newBase string) (RetargetOutcome, PullRequest, error)
// Ensures exactly one comment whose body starts with marker (a Docket-owned attempt marker line).
type CommentOutcome string // "created" | "already" | "unknown"
func (c *Client) EnsureComment(ctx context.Context, repo Repository, number int, marker, body string) (CommentOutcome, string /*url*/, error)
func (c *Client) FindComment(ctx context.Context, repo Repository, number int, marker string) (found bool, url string, err error)
// Merge with merge-commit semantics at an exact expected head; never requests branch deletion.
type MergeOutcome string // "merged" | "already-merged" | "head-moved" | "not-mergeable" | "denied" | "unknown"
type MergedFacts struct{ HeadOID, BaseRef, MergedAtUTC, MergeCommit string; Version string }
func (c *Client) MergePullRequest(ctx context.Context, repo Repository, number int, expectedHead ObjectRef, admin bool) (MergeOutcome, MergedFacts, error)
// Authoritative reprobe usable after success, timeout, cancellation, or lost response.
func (c *Client) ProbeMerged(ctx context.Context, repo Repository, number int) (MergeOutcome, MergedFacts, error)
```

- [ ] **Step 1: Write failing protocol tests** against the extended fake (protocol-faithful: production argv/stdin/env, current nested GitHub JSON):
  - `TestRetargetProbeActVerify` — retarget happens, verified snapshot has new base; `already` on a second call (idempotent by promised state); version drift → `contended`, no edit issued (assert via the fake's request log); probe error → `unknown`.
  - `TestEnsureCommentIdempotent` — first call creates; second finds by marker and returns `already` with the same URL; comment-list probe failure → `unknown` and no create issued.
  - `TestMergeExpectedHead` — merge with `--match-head-commit`; moved head → `head-moved`, no merge; already merged exact PR → `already-merged` with facts; permission denial → `denied`; response loss then `ProbeMerged` recovers `already-merged` with mergedAt + merge commit; open PR reprobe → not merged; malformed fake output → `unknown`.
  - `TestMergeNeverDeletesBranch` — assert the fake never receives `--delete-branch`.
  - `TestFakeLazyMergeability` — `UNKNOWN` mergeability round-trips as `UNKNOWN`, not clean.
- [ ] **Step 2: Red:** `go test ./internal/githubcli/ -run 'Retarget|Comment|Merge' -count=1`.
- [ ] **Step 3: Implement** the three files following `ensure.go`'s probe/act/verify shape. Comment body crosses via stdin (`--body-file -`), never argv.
- [ ] **Step 4: Green**, then `go test ./internal/githubcli/ -count=1`.
- [ ] **Step 5: Commit:** `git commit -m "feat(0316): githubcli retarget, idempotent comment, expected-head merge"` (add/modify the seven files).

### Task 5: workspace rebase receipt and narrow rewrite publish

**Files:**
- Create: `internal/workspace/rebasereceipt.go`, `internal/workspace/rebasereceipt_test.go`, `internal/workspace/rewrite.go`, `internal/workspace/rewrite_test.go`

**Interfaces:**
- Consumes: `workspace.Manifest`, `Service`, `gitcli.PushLease`, Task 3 owned refs.
- Produces:

```go
// Effect receipt, not a phase machine: proves which rewrite may be continued/aborted/published.
type RebaseReceipt struct {
    RepoIdentity, ChangeID           string
    OrigHead, OrigRemoteHead         string
    BaseRef, BaseHead                string
    Attempt                          string // opaque attempt token
    CreatedUTC                       string
}
func (s *Service) WriteRebaseReceipt(ctx context.Context, dir string, r RebaseReceipt) error   // beside the manifest, atomic rename
func (s *Service) ReadRebaseReceipt(ctx context.Context, dir string) (RebaseReceipt, bool, error) // three outcomes: found / cleanly absent / error
func (s *Service) ClearRebaseReceipt(ctx context.Context, dir string) error
// Narrow rewrite publication. Refuses without a matching receipt; pushes exactly newHead
// under --force-with-lease against the receipt's OrigRemoteHead; reprobes to equality.
type RewriteOutcome string // "published" | "noop" | "contended" | "unknown"
func (s *Service) PublishRewrite(ctx context.Context, req RewriteRequest) (RewriteOutcome, error)
type RewriteRequest struct{ Dir string; Receipt RebaseReceipt; NewHead string }
```

- [ ] **Step 1: Write failing tests** (real-git fixtures per `internal/workspace/publish_test.go`):
  - `TestRebaseReceiptRoundTrip` — write/read/clear; read after clear is cleanly absent; corrupt file is error, not absent.
  - `TestPublishRewriteLease` — remote at OrigRemoteHead → push lands exactly NewHead; reprobe proves equality.
  - `TestPublishRewriteNoop` — remote already at NewHead → `noop`, no push issued.
  - `TestPublishRewriteContention` — remote moved past OrigRemoteHead → `contended`, remote untouched.
  - `TestPublishRewriteRefusesWithoutReceipt` — missing/mismatched receipt (wrong repo identity, wrong change) → error before any push.
  - `TestPublishRewriteUnknownRetains` — injected probe failure → `unknown`, no second mutation.
  - `TestGeneralPublishStillRefusesRewrite` — the landed `PublishHead` still refuses non-fast-forward (the general service is not weakened).
- [ ] **Step 2: Red:** `go test ./internal/workspace/ -run 'RebaseReceipt|Rewrite' -count=1`.
- [ ] **Step 3: Implement.** Receipt file `rebase-receipt.json` in the manifest directory; temp-file-beside-destination + rename for atomicity.
- [ ] **Step 4: Green**, then `go test ./internal/workspace/ -count=1`.
- [ ] **Step 5: Commit:** `git commit -m "feat(0316): workspace rebase receipt and lease-scoped rewrite publish"`

### Task 6: `docket context finalize`

**Files:**
- Create: `internal/app/finalize_context.go`, `internal/app/finalize_context_test.go`, `internal/cli/finalize.go`, `internal/cli/finalize_test.go`
- Modify: `internal/cli/root.go` (register), `internal/cli/context.go` (add `finalize` subcommand delegating to app)

**Interfaces:**
- Consumes: the status pin/corpus reader (`internal/app/status.go` snapshot assembly), `domain.SelectFinalizeQueue` (Task 1), `githubcli` probes, `workspace.Inspect`, gate policy from `config.Effective`.
- Produces:

```go
type FinalizeContextRequest struct{ ID int; Allowlist []int }
type FinalizeContextResult struct {
    Result // protocol-v1 embed, HumanText()
    Candidates []FinalizeCandidateReport
}
type FinalizeCandidateReport struct {
    // spec "Authoritative finalize context": change path/bytes/version/status/branch/PR/plan/
    // results/markers; effective base + exact remote heads; workspace ownership/cleanliness;
    // existing rebase receipt; full PR facts incl. body-evidence verdict and comments;
    // dependency + stack relations with each descendant's lifecycle and PR destination;
    // open child PR set; resolved gate/suite/approval/reclaim policy, repo mode, refs,
    // capability notices; typed candidate/skip reason. Nil collections normalized.
}
func ContextFinalize(ctx context.Context, deps FinalizeDeps, repoDir string, req FinalizeContextRequest) FinalizeContextResult
type FinalizeDeps struct { /* Planning-style deps + GitHub client + workspace service + gitcli; Clock injected */ }
```

- [ ] **Step 1: Write failing app tests** against fake reader + fake GitHub/git seams (pattern: `internal/app/implementation_context_test.go`):
  - `TestContextFinalizeSelection` — no id: ordering matches `SelectFinalizeQueue`; merged-recovery candidate surfaces first; one metadata pin (count `PinContext` calls == 1).
  - `TestContextFinalizeExplicitID` — inspects exactly that record even when skip-reasoned (approval-required, finalize-blocked) — reported as candidate with override note; malformed state stays refused.
  - `TestContextFinalizeProbeErrorIsUnknown` — GitHub probe error → PR facts `unknown`, candidate band `unknown`; never clean absence.
  - `TestContextFinalizeStackFacts` — descendant lifecycles and open child PR set come from the graph, not rendered tables.
  - `TestContextFinalizeTypedReasons` — every skipped candidate carries a closed reason token; nothing omitted or guessed.
  - `TestContextFinalizeReadOnly` — no transaction, no Git mutation (fake seams record zero writes).
- [ ] **Step 2: Red**, **Step 3: Implement**, **Step 4: Green** (`go test ./internal/app/ -run ContextFinalize -count=1`, then package suite).
- [ ] **Step 5: CLI.** Failing `internal/cli/finalize_test.go` + `context_test.go` additions: `docket context finalize --json` emits exactly one protocol-v1 document; `--id`/`--allowlist` route. Implement `newFinalizeCommand` mirroring `newChangeCommand`; register. Green.
- [ ] **Step 6:** `go test ./internal/app/ ./internal/cli/ -count=1`; `gofmt -l internal/ | (! grep .)`; `go vet ./...`.
- [ ] **Step 7: Commit:** `git commit -m "feat(0316): authoritative finalize context and selection"`

### Task 7: `finalize retarget-children`

**Files:**
- Create: `internal/app/finalize_retarget.go`, `internal/app/finalize_retarget_test.go`
- Modify: `internal/cli/finalize.go`, `internal/cli/finalize_test.go`

**Interfaces:**
- Consumes: `domain.StackChildren`, `githubcli.RetargetPullRequest` (Task 4), request-file decode (follow the bounded request-file reader used by `change reconcile` in `internal/app/change_reconcile.go`).
- Produces:

```go
type RetargetChildrenRequest struct {
    ID int; Version string
    // request file: the exact human-authorized set from context finalize
    Children []AuthorizedChild // {ID int; PRNumber int; PRVersion string}
}
func FinalizeRetargetChildren(ctx context.Context, deps FinalizeDeps, repoDir string, req RetargetChildrenRequest) RetargetChildrenResult
```

- [ ] **Step 1: Failing app tests:**
  - `TestRetargetChildrenHappy` — each authorized open child probe/act/verified onto the parent's effective base; retry adopts an already-retargeted exact PR as no-op.
  - `TestRetargetChildrenNewChildBlocks` — a child open in the live graph but absent from the authorized set → `contended`, zero edits issued.
  - `TestRetargetChildrenVersionDrift` — changed PR version → `contended`; ambiguous PR (two open PRs for one child head) → `contended`; probe error → `unknown`; in every case no parent-merge-enabling success.
  - `TestRetargetChildrenLeavesStackedOn` — no metadata mutation at all (no transaction executed); `stacked_on:` untouched by design.
  - `TestRetargetChildrenSkipsTerminalChildren` — `stacked-merged`/`done` children don't block and aren't edited.
- [ ] **Step 2: Red. Step 3: Implement. Step 4: Green** (`-run RetargetChildren -count=1`).
- [ ] **Step 5: CLI** subcommand `finalize retarget-children --id --version --input`; failing test asserting flag routing + single protocol document; green.
- [ ] **Step 6: Commit:** `git commit -m "feat(0316): finalize retarget-children with exact authorized set"`

### Task 8: `finalize rebase` / `rebase-continue` / `rebase-abort` + local gate composition

**Files:**
- Create: `internal/app/finalize_rebase.go`, `internal/app/finalize_rebase_test.go`
- Modify: `internal/cli/finalize.go`, `internal/cli/finalize_test.go`

**Interfaces:**
- Consumes: Task 3 rebase primitives, Task 5 receipt, `domain.ResolveEffectiveBase`, `workspace.Inspect`, landed gate launch/observe (`internal/app/gate.go`) and `evidence record` (`internal/app/evidence_ops.go`), resolver report decode.
- Produces:

```go
type FinalizeRebaseRequest struct{ ID int; Version, Head string }
// dispositions: "unchanged" | "rebased" | "conflicted" | "contended" | "blocked" | "failed"
func FinalizeRebase(ctx context.Context, deps FinalizeDeps, repoDir string, req FinalizeRebaseRequest) FinalizeRebaseResult
type ResolverReport struct { // versioned JSON envelope, bounded UTF-8, from file/stdin
    ChangeID int; Attempt string; Disposition string // "resolved" | "stuck"
    Summary string; TouchedPaths, ConflictedPaths []string
    ObservedHead, ObservedBase string; RecommendedAction string
}
func FinalizeRebaseContinue(ctx context.Context, deps FinalizeDeps, repoDir string, id int, attempt string, report ResolverReport) FinalizeRebaseResult
func FinalizeRebaseAbort(ctx context.Context, deps FinalizeDeps, repoDir string, id int, attempt string, report ResolverReport) FinalizeRebaseResult
// Local-gate decision after a completed rebase (pure, tested directly):
// skip only when the rebase was a no-op AND PR body evidence parses green for the EXACT current
// head (permit named in result); anything else runs the full resolved suite. No strict-ancestor skip.
func gateDecision(noop bool, evidenceHead, currentHead string, evidenceGreen bool) (skip bool, permit string)
```

- [ ] **Step 1: Failing app tests** (real-git fixture worktree + fake gate seam):
  - `TestFinalizeRebasePreconditions` — table: not `implemented`, version drift, unresolved base, dirty workspace, remote head ≠ expected, PR base mismatch → `contended`/`blocked` with closed reasons, receipt NOT written, Git untouched.
  - `TestFinalizeRebaseHappyAndReceipt` — receipt written before Git mutation with exact orig/base identities; owned refs exist; disposition `rebased`/`unchanged` correct.
  - `TestFinalizeRebaseResponseLossRecovery` — completed rewrite + lost response: second call inspects receipt/refs/head/ancestry + no in-progress rebase and returns the same `rebased` outcome; never rebases a different head.
  - `TestFinalizeRebaseForeignStateBlocked` — pre-existing foreign rebase or moved base → `blocked`, retained, never reset or adopted.
  - `TestFinalizeRebaseContinueValidatesReport` — paths outside live unmerged set → refusal; valid report stages exactly those paths and continues; next conflict or verified completion returned; wrong attempt token → refusal.
  - `TestFinalizeRebaseAbortVerifiesRestore` — abort proves owned attempt, restores orig head, verified; ambiguous report routed here completes with `blocked` marker recommendation.
  - `TestGateDecision` — table: no-op + exact-head green evidence → skip with named permit; moved head, real rebase, stale evidence, strict-ancestor-only evidence → run suite (spec: the deferred exemption is NOT implemented).
  - `TestFinalizeRebaseGateOutcomes` — passed → evidence via landed `evidence record` only; failed → repair-work disposition; running-at-budget/signaled/stopped/vanished/malformed/unavailable → halt disposition, never a fabricated red.
- [ ] **Step 2: Red. Step 3: Implement. Step 4: Green** (`-run FinalizeRebase -count=1`).
- [ ] **Step 5: CLI** subcommands with `--input` report files; green.
- [ ] **Step 6: Commit:** `git commit -m "feat(0316): finalize rebase, resolver continue/abort, and local gate composition"`

### Task 9: `finalize publish` — rewrite push + PR evidence update

**Files:**
- Create: `internal/app/finalize_publish.go`, `internal/app/finalize_publish_test.go`
- Modify: `internal/cli/finalize.go`, `internal/cli/finalize_test.go`

**Interfaces:**
- Consumes: Task 5 `PublishRewrite`, landed PR evidence-block replacement (grep `internal/app/pr_publish.go` for the managed build-evidence block writer), `githubcli` PR probe by number/head/base/version.
- Produces:

```go
type FinalizePublishRequest struct{ ID int; Attempt, Head string; EvidencePath string }
func FinalizePublish(ctx context.Context, deps FinalizeDeps, repoDir string, req FinalizePublishRequest) FinalizePublishResult
```

- [ ] **Step 1: Failing tests:**
  - `TestFinalizePublishOrder` — probes remote first; no-op when already at head; otherwise lease push per Task 5; then PR reprobe by number+expected version; evidence block loss-preservingly replaced (authored prose, title, other body bytes byte-identical — assert on full body); probe/act/verify; never creates a second PR.
  - `TestFinalizePublishCrashReplay` — crash after push, before PR update: replay adopts the rewrite (remote-head probe) and resumes only the PR update. Crash after both: full no-op.
  - `TestFinalizePublishUnknownStops` — reprobe unknown → `unknown`, no second mutation, no merge-enabling success.
  - `TestFinalizePublishRefusesForeignAttempt` — attempt token not matching the receipt → refusal before any push.
- [ ] **Step 2: Red. Step 3: Implement. Step 4: Green. Step 5: CLI + green.**
- [ ] **Step 6: Commit:** `git commit -m "feat(0316): finalize publish — receipt-scoped rewrite push and PR evidence update"`

### Task 10: `finalize merge` with authoritative verification

**Files:**
- Create: `internal/app/finalize_merge.go`, `internal/app/finalize_merge_test.go`
- Modify: `internal/cli/finalize.go`, `internal/cli/finalize_test.go`

**Interfaces:**
- Consumes: `domain.MergeConjuncts` (Task 1), Task 4 `MergePullRequest`/`ProbeMerged`, `gitcli` fetch + `IsAncestor` for merge-commit reachability.
- Produces:

```go
type FinalizeMergeRequest struct{ ID int; Version, Head string; Admin bool; ExplicitID bool /* human authorization */ }
type VerifiedMerge struct{ PRNumber int; PRVersion, HeadOID, BaseRef, MergedAtUTC, MergeCommit string }
func FinalizeMerge(ctx context.Context, deps FinalizeDeps, repoDir string, req FinalizeMergeRequest) FinalizeMergeResult // carries VerifiedMerge on success
```

- [ ] **Step 1: Failing tests:**
  - `TestFinalizeMergeConjunctsRechecked` — table: each `MergeConjuncts` field falsified immediately before the effect (fresh reload) → refusal with that field's token, no merge call issued. Includes: unretargeted open child, newer finalize-blocked marker, superseding metadata version.
  - `TestFinalizeMergeExplicitIDOverrides` — explicit id satisfies approval and finalize-blocked skip; does NOT override malformed state, wrong PR identity, unsafe stack, or repair sign-off.
  - `TestFinalizeMergeAdminGate` — `--admin` honored only with `ExplicitID` (attended, named); never inferred from approval absence or a permission error (denied stays denied).
  - `TestFinalizeMergeVerification` — after merge (and after simulated timeout/lost response) reprobe returns exact facts; Git fetch proves merge commit reachable from destination tip; open PR → not merged; different head/base → contended/invalid; unobservable → unknown; none of those permits closeout.
  - `TestFinalizeMergeAlreadyMergedNoop` — already-merged exact PR → verified no-op result regardless of who merged; never a second merge (fake request log).
- [ ] **Step 2: Red. Step 3: Implement. Step 4: Green. Step 5: CLI + green.**
- [ ] **Step 6: Commit:** `git commit -m "feat(0316): finalize merge with expected head and authoritative verification"`

### Task 11: `finalize block` / `clear-block` and `change halt` / `resume-halted`

**Files:**
- Create: `internal/app/finalize_block.go`, `internal/app/finalize_block_test.go`, `internal/app/change_halt.go`, `internal/app/change_halt_test.go`
- Modify: `internal/cli/finalize.go`, `internal/cli/change.go` (+ tests), `internal/app/run_verify.go` + `run_verify_test.go` (`run-halted` verdict), `internal/app/implementation_context.go` + test (report durable halt checkpoints)

**Interfaces:**
- Consumes: Task 4 `EnsureComment`/`FindComment`, `transaction.Engine`, `domain.EvaluateLease`, section upsert machinery (grep `internal/document` for the managed-section upsert used by the reconcile log).
- Produces:

```go
// finalize block: comment first (idempotent by owned attempt marker), then one exact-version
// transaction upserting ONE "## Finalize blocked" section (date, reason, attempt, verified facts,
// comment URL, remedy). Re-mark replaces the interior / appends an attempt; never a second heading.
func FinalizeBlock(ctx context.Context, deps FinalizeDeps, repoDir string, req BlockRequest) BlockResult
func FinalizeClearBlock(ctx context.Context, deps FinalizeDeps, repoDir string, req ClearBlockRequest) BlockResult
// change halt: one bounded authored report into "## Run halted" on an in-progress change;
// branch, lease, workspace, evidence untouched.
func ChangeHalt(ctx context.Context, deps PlanningDeps, repoDir string, req HaltRequest) HaltResult
// resume: requires exact marked record + explicit AcknowledgeQuiescent; reprobes branch/workspace/
// live gate; refreshes claim; removes marker; preserves checkpoints; never resets/adopts a
// workspace whose writer may be live.
func ChangeResumeHalted(ctx context.Context, deps PlanningDeps, repoDir string, req ResumeRequest) HaltResult
```

- [ ] **Step 1: Failing tests:**
  - `TestFinalizeBlockCommentThenMarker` — order enforced; crash between them replayed: comment found by marker, reused, marker transaction finished; comment probe failure → unknown, NO marker written claiming the comment exists.
  - `TestFinalizeBlockSingleSection` — re-mark never duplicates the heading; marker order/balance validated before rewrite; board rerendered atomically in the same transaction.
  - `TestFinalizeClearBlockReprobes` — requires exact current head + valid gate evidence (unless gate off) + published remote ref + matching open PR before removal; each missing conjunct refuses.
  - `TestChangeHaltPreservesCheckpoints` — `in-progress` record gains `## Run halted` with bounded report; branch/claim/workspace fields byte-identical; non-in-progress refused.
  - `TestChangeResumeHalted` — refuses without `--acknowledge-quiescent`; refuses on version drift; reprobes and refuses on a live gate lock; success refreshes claim, removes exactly the marker section, preserves every other byte.
  - `TestRunVerifyHaltedVerdict` — `run verify` on a halted change returns closed `run-halted`.
  - `TestContextImplementationReportsHalt` — `context implementation --id` surfaces the durable halt checkpoints.
- [ ] **Step 2: Red. Step 3: Implement. Step 4: Green** (`go test ./internal/app/ -run 'FinalizeBlock|FinalizeClearBlock|ChangeHalt|ChangeResume|RunVerify|ContextImplementation' -count=1`).
- [ ] **Step 5: CLI** — `finalize block|clear-block`, `change halt|resume-halted --acknowledge-quiescent`; green.
- [ ] **Step 6: Commit:** `git commit -m "feat(0316): durable finalize-blocked and run-halted state with verified recovery"`

### Task 12: `finalize closeout` — ordinary archive, stacked-merged, root carry, backlink leg

**Files:**
- Create: `internal/app/finalize_closeout.go`, `internal/app/finalize_closeout_test.go`
- Modify: `internal/cli/finalize.go`, `internal/cli/finalize_test.go`

**Interfaces:**
- Consumes: Task 10 `VerifiedMerge`, `domain.MarkDone`/`MarkStackedMerged`, Task 2 `DeriveRootCloseoutSet`, the kill path's relocation plan (`internal/app/change_kill.go` "Archive move" idiom: `MutationCreate` archive + delete active in ONE transaction plan), board/backlink renderers, `transaction.Engine`.
- Produces:

```go
// No caller-supplied done boolean or archive date: reloads metadata, reprobes PR + destination,
// derives UTC archive date from verified GitHub mergedAt.
func FinalizeCloseout(ctx context.Context, deps FinalizeDeps, repoDir string, id int) CloseoutResult
// dispositions: "done-archived" | "stacked-merged" | "root-archived" (with carried ids) |
// "already" | "children-retarget-required" | "contended" | "blocked" | "unknown"
```

- [ ] **Step 1: Failing tests (real-git, both metadata modes where the transaction shape differs):**
  - `TestCloseoutOrdinary` — verified integration-destination merge: one transaction applies `MarkDone` (only after merge-commit reachability proof), stamps `updated:` from merge UTC date, clears claim, preserves historical branch/PR fields, relocates to the dated archive path, rerenders artifact block + spec backlink + board, validates, explicit-path commit, lease push. Failure injected at renderer/validate/push → no remotely partial outcome.
  - `TestCloseoutStackedMerged` — PR destination is live parent's branch: `MarkStackedMerged` in place, stale gate marker cleared, board rerendered; NOT archived, branch/workspace retained. Stacked PR retargeted+merged into integration → ordinary done path.
  - `TestCloseoutRootCarry` — root merged to integration: `DeriveRootCloseoutSet` proven → ONE transaction archives root + every carried descendant using the ROOT's merge date for all archive filenames, one board render over the final population. One unproven descendant → root stays recoverable, zero descendant writes.
  - `TestCloseoutIdempotent` — replay after response-lost success: canonical archive record + transaction receipt exist → no-op (keyed on the promised state, not a clean tree).
  - `TestCloseoutRefusals` — open PR, unknown probe, destination mismatch, source status illegal for the transition → closed refusal, no mutation.
  - `TestCloseoutBacklinkLegDocketMode` — docket mode: metadata transaction lands first; follow-up isolated integration-ref operation patches ONLY existing `docket:backlink` blocks in verified plan/results paths, explicit-path commit with stable closeout request trailer, exact expected-ref lease push. Failed/contended leg: change stays truthfully `done`, typed `terminal-backlink-pending` finding emitted; retry recovers from remote block bytes + trailer. Main mode: single atomic transaction covers all blocks.
  - `TestCloseoutNeverEditsAuthoredBytes` — plan/results authored content outside the backlink block byte-identical.
- [ ] **Step 2: Red. Step 3: Implement. Step 4: Green** (`-run Closeout -count=1`). **Step 5: CLI + green.**
- [ ] **Step 6: Commit:** `git commit -m "feat(0316): atomic terminal closeout for ordinary, stacked, and root outcomes"`

### Task 13: `change reclaim`

**Files:**
- Create: `internal/app/change_reclaim.go`, `internal/app/change_reclaim_test.go`
- Modify: `internal/cli/change.go`, `internal/cli/change_test.go`

**Interfaces:**
- Consumes: `domain.EvaluateLease`/`EvaluateReclaim`/`Reclaim` (landed), `config.Reclaim` TTL, injected `Clock`, gitcli branch probes, `workspace.Inspect`, gate ownership probes.
- Produces:

```go
type ChangeReclaimRequest struct{ ID int; Version string }
func ChangeReclaim(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeReclaimRequest) ChangeReclaimResult
```

- [ ] **Step 1: Failing tests:**
  - `TestReclaimRequiresStrictExpiry` — table on lease states: missing, empty, malformed, future, exactly-at-boundary, non-in-progress → refusal with stable reason; strictly expired proceeds.
  - `TestReclaimRequiresProvenAbsence` — recorded AND conventional branch must be cleanly absent locally and remotely; live workspace (allocating/ready/dirty/ambiguous), owned gate run, or any ownership record → skip; ANY probe error → skip (`unknown` never shares the absent branch), zero cleanup/delete/reset/marker-removal side effects on every refusal.
  - `TestReclaimTransaction` — success: landed `Reclaim` action applied, one dated `## Reclaim log` entry (previous claim + proof summary), status `proposed`, branch/claim cleared, `reconciled: false`, links/board rerendered, validate/commit/push; exact-version contention → `contended`.
  - `TestReclaimIndependentOfAutoPolicy` — explicit reclaim works with `reclaim.auto: false`.
- [ ] **Step 2: Red. Step 3: Implement. Step 4: Green. Step 5: CLI (`change reclaim --id --version`) + green.**
- [ ] **Step 6: Commit:** `git commit -m "feat(0316): proof-gated explicit change reclaim"`

### Task 14: `finalize cleanup` and `gate cleanup` retention

**Files:**
- Create: `internal/app/finalize_cleanup.go`, `internal/app/finalize_cleanup_test.go`
- Modify: `internal/cli/finalize.go`, `internal/cli/gate.go` (add `gate cleanup <absolute-run-dir>`), + their tests, `internal/app/gate.go` if the run-record reader needs an ownership/terminal probe export

**Interfaces:**
- Consumes: Task 3 `DeleteLocalBranchChecked`/`DeleteRemoteRefLease`, landed `workspace.Cleanup` (manifest-fact driven), Task 12 backlink repair, `internal/process` run records/locks, `internal/evidence`.
- Produces:

```go
func FinalizeCleanup(ctx context.Context, deps FinalizeDeps, repoDir string, id int) CleanupOpResult
// ordered suffix: reload terminal state + verified destination; repair backlinks first if pending;
// landed workspace removal from manifest facts (never a recomputed base); local ref delete only when
// exact recorded tip is worktree-detached AND contained in the verified merge chain; remote ref
// delete only under exact lease after a fresh probe shows no open child PR targets it; tombstone kept.
func GateCleanup(ctx context.Context, deps FinalizeDeps, runDir string) CleanupOpResult
// removes one exact private run dir only after: ownership valid, terminal record, no live lock/group,
// and durable exact-head evidence OR persisted halt/finalize report. Retains failed/vanished/
// ambiguous/unreported runs.
```

- [ ] **Step 1: Failing tests:**
  - `TestFinalizeCleanupOnlyAfterTerminal` — non-terminal change → refusal; aborted-owned-rebase restore is the one pre-terminal exception.
  - `TestFinalizeCleanupBranchDeletion` — local delete requires exact tip + detached + merge-chain containment (falsify each → retained); remote delete requires lease + fresh no-open-child probe (open child → retained + `children-retarget-required` on the out-of-band-merge case); stacked-merged changes retain workspace and branches until root closes.
  - `TestFinalizeCleanupInjectedProbeErrors` — one injected-error test per probe (list/ancestry/ref/manifest/lock) proving the resource is RETAINED and the result is pending, never destroyed (learning `probe-error-is-not-clean-absence`).
  - `TestFinalizeCleanupRetryable` — each leg independently retryable; a completed leg replays as already-clean only on clean absence + tombstone/receipt; foreign absence not adopted.
  - `TestGateCleanupRetention` — table: live lock, failed-without-report, ambiguous ownership, foreign dir, probe error → retained; owned + terminal + evidence-or-report → removed with cleanup receipt; second call no-op via receipt.
  - `TestCleanupNeverTouchesForeignTrees` — no global prune, no force-remove, no pathname-recursive delete; primary/metadata/transaction/sibling worktrees untouched (assert via recorded git ops).
- [ ] **Step 2: Red. Step 3: Implement. Step 4: Green. Step 5: CLI + green.**
- [ ] **Step 6: Commit:** `git commit -m "feat(0316): ownership-safe finalize cleanup and gate-run retention"`

### Task 15: `maintenance sweep`

**Files:**
- Create: `internal/app/maintenance.go`, `internal/app/maintenance_test.go`, `internal/cli/maintenance.go`, `internal/cli/maintenance_test.go`
- Modify: `internal/cli/root.go`

**Interfaces:**
- Consumes: Tasks 12–14 operations, `domain.SelectFinalizeQueue` merged-recovery band, `config.Reclaim.Auto`.
- Produces:

```go
func MaintenanceSweep(ctx context.Context, deps FinalizeDeps, repoDir string) MaintenanceResult
// pins an initial inventory; deterministic order; reloads fresh authority before EVERY mutation;
// per-item entries: applied | noop | contended | blocked | unknown | failed (sorted, structured).
```

- [ ] **Step 1: Failing tests:**
  - `TestSweepFindsMergedImplemented` — active `implemented` with merged PR → closeout invoked; stacked children closed before ancestors (order asserted); root then carries descendants.
  - `TestSweepRetriesSuffixes` — pending backlink repair and pending cleanup retried for archived/done records and completed stacks.
  - `TestSweepReclaimGatedOnAuto` — `reclaim.auto: true` → eligible reclaim attempted; `false` → skipped with reason.
  - `TestSweepNeverEscalates` — never merges an open PR, never overrides approval, never retargets without authorization, never edits authored results; finalize marker does not suppress recovery of an out-of-band-merged PR.
  - `TestSweepItemIsolation` — one item's failure doesn't stop independent items; within an item, a destructive suffix never runs after an unknown prerequisite.
  - `TestSweepStructuredReport` — every item reported with closed disposition; no collapsed boolean.
  - `TestSweepReloadsBeforeMutation` — fake engine counts reloads: one fresh reload per mutation.
- [ ] **Step 2: Red. Step 3: Implement. Step 4: Green. Step 5: CLI (`docket maintenance sweep`; `docket status` stays read-only) + green.**
- [ ] **Step 6: Commit:** `git commit -m "feat(0316): merged-PR maintenance sweep with per-item authority reloads"`

### Task 16: Real-git failure-injection matrix (both repository modes)

**Files:**
- Create: `internal/app/finalize_git_test.go` (integration-tagged like the existing `*_git_test.go` files)

**Interfaces:** consumes Tasks 6–15 through their app entry points against disposable repos + bare remotes.

- [ ] **Step 1: Write the matrix test file** (these tests are the deliverable; they must be red-then-green against injected fault seams — add a fault-injection hook to `FinalizeDeps` if Tasks 6–15 did not already, following the interrupt-hook pattern in `internal/repository/transaction/interrupt_test.go`):
  - `TestFinalizeInterruptionMatrix` — for BOTH `main` and `docket` modes, inject interruption before and after each boundary from the spec's recovery table — rebase start/conflict, rebase completion, force-with-lease push, PR evidence update, child retarget, PR merge, metadata closeout push, integration backlink push, worktree removal, local/remote ref delete, gate-run delete — then rerun the same operation and assert convergence: no duplicate merge/comment/PR, no false `done`, no unsafe overwrite/delete, no stranded stack, no hidden phase state consulted (grep the receipt: only receipt/refs/manifest/live state read).
  - `TestFinalizeConcurrentMovement` — concurrent base move, remote feature-head move, and same-entity metadata contention each produce `contended`, never a text-merge or overwrite.
  - `TestFinalizeBytePreservation` — after every closeout variant, all bytes outside owned patches are identical (full-file compare against pre-images).
  - `TestFinalizeNoForeignWrites` — no writes to primary/metadata/transaction/sibling worktree indexes during any operation (watch mtimes/HEADs).
- [ ] **Step 2–4: Red where seams are missing, implement seams, green:** `go test ./internal/app/ -run 'FinalizeInterruption|FinalizeConcurrent|FinalizeByte|FinalizeNoForeign' -count=1` (expect minutes; keep fixtures minimal).
- [ ] **Step 5: Commit:** `git commit -m "test(0316): failure-injection and concurrency matrix for the terminal path"`

### Task 17: Hermetic end-to-end matrix, negative capability fixture, mutation pass

**Files:**
- Create: `internal/app/finalize_e2e_test.go` (builds `./cmd/docket` to `t.TempDir()`, absolute-path invocation, isolated global + repo-local config via env overrides — copy the harness from `internal/app/workflow_e2e_test.go`)
- Modify: `tests/runtime-budgets.tsv` only if a new `tests/test_*.sh` wrapper is added (prefer wiring through the existing `tests/test_go_toolchain.sh`; check first — learning `check-plumbing-auto-discovery`)

- [ ] **Step 1: Write the e2e tests** (spec "End-to-end and mutation tests", every bullet):
  - `TestE2EOrdinaryFinalize` — from a 0315-style verified `implemented` state: local-gate finalize → rebase → gate → publish → merge → closeout → cleanup, in BOTH repository modes, driven purely through CLI argv.
  - `TestE2EConflictAndRepair` — rebase conflict → resolver-report continue → red suite → repair-report + sign-off halt (`repair-needs-signoff` recorded) → retry → merge → closeout.
  - `TestE2EResponseLossConvergence` — kill the binary (or fault-inject) after each external effect; rerun; assert convergence without duplicates or false state.
  - `TestE2EStack` — child merge → `stacked-merged`; root merge carries all descendants to `done`; open unauthorized children → halt + branch retention (never a closed child PR).
  - `TestE2EOutOfBandMergeRecovered` — human-merged PR recovered by `maintenance sweep`.
  - `TestE2EHaltResumeAndReclaim` — persistent implementation halt + resume; expired no-work reclaim.
  - `TestE2EUnsupportedConfigFence` — negative fixture loading repo-local `agents.*`, `auto_capture.enabled`, `build.checkpoint`, `finalize.skip_results_only_delta`, and `terminal_publish`: EVERY mutating 0316 operation returns `unsupported-config` naming the blockers, with zero metadata/Git/GitHub/workspace/gate/cleanup effect; global model/effort pins remain supported. Fixture lives in the test's own tempdir YAML — do NOT copy it into any frozen fixture tree (learning `config-edit-trips-its-own-frozen-drift-guard`).
  - `TestE2ENoPathDocketDependency` — run with a PATH scrubbed of `docket`; everything still passes (absolute-path invocation only).
- [ ] **Step 2–4: Red, fix, green:** `go test ./internal/app/ -run 'TestE2E' -count=1`.
- [ ] **Step 5: Mutation pass on every new guard** in Tasks 1–17: for each safety refusal (merge conjunct, cleanup probe, reclaim proof, receipt fence, marker balance, backlink-only patch), strip the guarded premise, run `go test -count=1` on its test, watch it redden, restore (keep a copy of the edit — `git checkout --` restores to HEAD and would destroy uncommitted work; learning `mutation-restore-needs-a-backup-copy`). Record any assert that refuses to redden as a finding, not a residual (learning `residual-is-for-undetectable-not-unprobed`).
- [ ] **Step 6: Commit:** `git commit -m "test(0316): hermetic end-to-end matrix, capability fence, mutation pass"`

### Task 18: Skill and embedded-asset revisions

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (+ its `references/`), `skills/docket-implement-next/SKILL.md` (+ references), the resolver/repair agent templates under `internal/assets/embedded/tree/agents/` sources (`docket-rebase-resolver`, `docket-integration-repair`), `docket-status` skill (maintenance invocation), command manifests/focused references the assets tree carries
- Regenerate: `go generate ./internal/assets/` in the same task (learning `generated-artifact-loaded-at-process-start` — note in the results file that a host restart is needed before the new assets are live)

- [ ] **Step 1: Revise `docket-finalize-change`** into the Claude-owned sequencer: `context finalize` → (attended-only child authorization + `finalize retarget-children`) → `finalize rebase`/resolver loop (resolver gets at most two attempts, enforced by the skill) → gate → `evidence record` → `finalize publish` → sign-off rules (authored repair ⇒ autonomous run records `repair-needs-signoff` and stops; attended run asks) → `finalize merge` → `finalize closeout` → `finalize cleanup`. One merge per invocation; `advanced`/`contended`/`drained`/`halted` driver contract retained; every prohibition names its return token; dispatch unavailability halts (existing carve-out).
- [ ] **Step 2: Revise resolver/repair agents:** resolver edits conflicts in the returned workspace only and returns the versioned `ResolverReport` JSON (Task 8 envelope — spell out the exact fields in the agent body); never runs rebase mechanics or tests. Repair agent authors the bounded fix, returns the report with claimed commits and `repaired`/`stuck`; never merges or transitions metadata.
- [ ] **Step 3: Revise `docket-implement-next` + `docket-status`:** halt/resume path (`change halt`, `change resume-halted --acknowledge-quiescent`, `run verify` → `run-halted`); status skill may run `docket maintenance sweep` before its read when asked; human `docket status` stays read-only. No sentence keyed to this repo.
- [ ] **Step 4: Regenerate embedded assets:** `go generate ./internal/assets/ && go test ./internal/assets/ -count=1` (drift guard must pass).
- [ ] **Step 5: Run asset-related bash guards:** capture `bash tests/test_asset_bundle_drift.sh` output into a variable and inspect (no early-exiting pipe consumers).
- [ ] **Step 6: Commit:** `git add skills/ internal/assets/ && git commit -m "feat(0316): finalize/resolver/repair/status/implement-next Go-v1 assets + embedded regeneration"`

### Task 19: Whole-suite gate and budget check

- [ ] **Step 1:** `gofmt -l internal/ cmd/ | (! grep .)`; `go vet ./...`; `go test ./... -count=1`.
- [ ] **Step 2:** Run the full resolved suite in the background to a log: `scripts/run-tests.sh > /tmp/docket-0316-suite.log 2>&1` (backgrounded per repo norms), then inspect the complete log. The gate reads any non-zero exit as red.
- [ ] **Step 3:** Treat every `OVER BUDGET:` line as a finding: either fix the cost or adjust `tests/runtime-budgets.tsv` with justification recorded for the results file. New shell test files (if any) must already have budget rows.
- [ ] **Step 4:** Fix any red; re-run the affected files, then the whole suite once more if anything changed.
- [ ] **Step 5: Commit** any fixes: `git commit -m "test(0316): suite gate fixes"` (explicit paths only).

### Task 20: Migrate the tests that describe the skills Task 18 rewrote

**Why this task exists.** Task 18 rewrote `docket-finalize-change` from a Bash procedure into a
Go-verb sequencer but carried no step for the tests that assert what that skill says. The Step-5
gate came back red with `files=123 passed=95 failed=28`, and the run halted because — in its own
words — "the plan/spec carried no test-migration mapping, and guessing that boundary would either
weaken a test or ship a broken skill." That mapping is this task. Do not re-derive it from the
diff: the diff cannot tell a deliberate deletion from a reworded one, and deriving it from the diff
is what produced the wrong first answers (restore the learning harvest; the Go failures are stale
goldens; `SKILL_FINISH` is a regression — all three are wrong).

**The authority is not your judgement.** Every assertion you retire must cite one of exactly three
sources. If an assertion matches none of them, it is a genuine loss: STOP and surface it rather
than retiring it.

1. **This change's own *Out of scope* section** names the deferred capabilities verbatim:
   "deferred CI/combined gates, results-only skips, terminal publishing, automatic learning
   harvest, capture/groom automation, cross-harness routing, skill rebinding, or Bash fallback
   behavior." An assertion testing any of these is obsolete-by-deferral.
2. **A Go symbol that now owns the behavior.** Each is verified to exist on this branch:

   | Behavior the skill used to restate | Go owner |
   |---|---|
   | Selection eligibility, ordering, skip reasons | `internal/app/finalize_context.go` — `FinalizeCandidateReport.Band`/`.SkipReason`/`.OverrideNote`, `FinalizePolicy` |
   | Explicit id overriding `require_pr_approval` | `internal/app/finalize_merge.go` — `ApprovalSatisfied: in.explicitID \|\| !in.requireApproval` |
   | The conditional local-suite skip | `internal/app/finalize_rebase.go` — `gateDecision` |
   | Board refresh after a transition | 12 sites under `internal/app/`, `finalize_closeout.go` among them — every mutating transaction re-renders `BOARD.md` in its own commit |
   | Commit scoping / staging discipline | Go transactions; the skill no longer commits |
   | `skills.finish` (`SKILL_FINISH`) | `internal/config/schema.go` — `dispDeferredActive`: any explicit value BLOCKS mutation |
   | `dummy_mode.enabled` | `internal/config/schema.go` — `dispDeferred`: enabling it blocks mutation, so the skill never runs with it on |
   | Bash suite routing (`runtime.bash`) | classified `obsolete-setting` — "docket no longer ships" it |

3. **A positive statement in the rewritten skill.** Where a deferral could be mistaken for an
   oversight, the skill says so outright — e.g. "There is no strict-ancestor or results-only skip."
   Assert that sentence rather than deleting the guard.

**Three categories, not two.** The halt report framed this as "content was stripped." Re-triage of
all 142 failing assertions found three kinds, and the third is the trap:

- **(a) Rightly deleted** — a Go verb absorbed it, or *Out of scope* defers it. Retire with a cite.
- **(b) Wrongly deleted** — genuine collateral damage. Restore. Only three were found, all already
  fixed in commit `8c74c1c8`: the `docket-build` *Gate execution posture* citation (narrowed to
  point 4, the yield-vs-block rule, which `docket gate` cannot own because it is a property of the
  CALLER's dispatch posture); the dummy-mode paragraph (deflated to state its deferred status); and
  the "BOARD.md is never published" invariant.
- **(c) Not deleted at all** — the behavior is preserved and the assertion is brittle. The canonical
  example: `grep -Eqi "already-merged.{0,40}changes in one run does not violate"` fails because the
  rewrite inserted a 51-character parenthetical, overrunning a `{0,40}` window. Nothing changed but
  the spacing. **Rewrite these to key on shape, never on character distance** — AGENTS.md: "Key a
  guard on syntactic shape, never an enumerated list of spellings." Do not "fix" category (c) by
  editing the skill back; the skill is correct.

**How to retire, precisely.** Deleting a guard is how a regression hides, so a retired assertion
becomes an INVERTED guard that proves the boundary stayed retired, preceded by a non-vacuity anchor
(an absent or empty file satisfies every bare `! grep` while proving nothing). The pattern, as
landed in `tests/test_configured_bash_finalize.sh`:

```sh
assert "finalize SKILL.md exists and is non-empty" '[ -s "$FIN" ]'
assert "finalize SKILL.md is the Go sequencer (non-vacuity anchor)" 'grep -qF "docket finalize" "$FIN"'
assert "finalize publishes no configured-bash start marker" \
  '! grep -qF -- "<!-- configured-bash-finalize:start -->" "$FIN"'
```

Each retired block carries a comment naming (i) what it used to guard, (ii) which of the three
authorities retires it, and (iii) when the file may be deleted outright. Bash removal is change
0318's, not this change's, so Bash-era files are inverted here and deleted there.

**Mutation-test every guard you touch.** Strip the thing it guards and watch it redden; a guard
that stays green is a defect. `tests/test_configured_bash_finalize.sh` was verified this way —
re-adding the marker reddens three assertions.

**Beware the vacuous loop.** The repo's shell is zsh in some contexts; `for t in $TESTS` does NOT
word-split there, so a re-baseline loop silently runs once on a non-existent filename and reports
zero failures. Run batch loops under `/opt/homebrew/bin/bash -c`, and sanity-check any "0 remaining"
against a direct single-file run before believing it.

**Inventory — 118 failing assertions across 20 files**, each with its category and the authority
that settles it. Counts were measured on this branch at commit `52226dba`; re-measure before you
start, because Task 18 fixes move them.

| File | # | Cat | Authority |
|---|---|---|---|
| `test_finalize_disposition.sh` | 33 | a, c | `finalize_context.go` owns selection/ordering/skips; some are (c) — brittle `.{0,N}` windows over preserved text |
| `test_finalize_gate.sh` | 27 | a | `finalize_context.go` (`Policy`, `Band`, `SkipReason`), `finalize_merge.go` (`ApprovalSatisfied`), `finalize publish` receipt lease; terminal-publish rows are *Out of scope* |
| `test_dispatch_capability.sh` | 9 | **c** | Content is INTACT at `SKILL.md` "## Dispatch unavailability — the carve-out" — names both agents, cites the convention's *Dispatch-capability resolution*, forbids inferring from a tool name. Nine per-step mentions became one section; rewrite the locators, change nothing in the skill |
| `test_closeout.sh` | 7 | a | *Out of scope*: terminal publishing; plus `docket.sh` facade calls |
| `test_gate_execution_posture.sh` | 6 | c | Citation restored in `8c74c1c8`; the remainder are structural locators keyed to the old per-step layout |
| `test_docket_metadata_branch.sh` | 5 | a | *Out of scope*: terminal publishing (`origin/docket` copy, main-mode skip, Accepted gate) |
| `test_stack_closeout.sh` | 4 | a | `docket.sh stack-closeout` → `docket finalize closeout` (root carry) |
| `test_learnings_ledger.sh` | 4 | a | *Out of scope*: automatic learning harvest. **Do NOT restore the harvest step** — `docket learning` is manual `record`/`update` only, by design |
| `test_dummy_mode.sh` | 4 | a | `dummy_mode.enabled` is `dispDeferred` — enabling it blocks all mutation, so surface bindings cannot be exercised |
| `test_docket_example_yml.sh` | 3 | a | Step-0 export / `FINALIZE_*` env channel → `context finalize`'s `Policy` block |
| `test_shared_worktree_commit_scope.sh` | 3 | a | Go transactions own committing; the skill stages nothing |
| `test_readme_finalize_docs.sh` | 3 | a | README prose describing the Bash flow |
| `test_config_read_channel.sh` | 2 | a | Step-0 export channel |
| `test_skill_handoff_precedence.sh` | 2 | a | `skills.finish` is `dispDeferredActive`; the "human is present" exception is an exception TO a capability the binary refuses, so zero occurrences is correct |
| `test_board_refresh_on_transition.sh` | 1 | a | Board absorbed into every mutating transaction; the never-published invariant was restored in `8c74c1c8` |
| `test_docket_stack.sh` | 1 | a | `docket.sh stack-children` → context bundle `Descendants`/`OpenChildPRs` |
| `test_change_links_coverage.sh` | 1 | a | `docket.sh render-change-links` → `finalize closeout` backlink leg |
| `test_skill_facade_wiring.sh` | 1 | a | Bash facade wiring |
| `test_sync_agents_run_gate.sh` | 1 | — | Mechanical: regenerate the committed AGENTS.md block per the recipe in the test |
| `test_results_artifact.sh` | 1 | **STOP** | Post-merge results appending is a GENUINE loss, NOT deferred — it is absent from *Out of scope*. Tracked as **change 0330**. Leave this assertion failing or skip it with a pointer to 0330; do NOT retire it as obsolete and do NOT invent a home for it here |

**Worked examples already landed** (follow their shape): `tests/test_configured_bash_finalize.sh`
(whole-file inversion), `tests/test_docket_review.sh` (block retirement, 15 assertions), and
`tests/test_docket_config.sh` (single assertion inverted with a non-vacuity anchor).

- [ ] **Step 1: Re-measure.** Run each file in the inventory under `/opt/homebrew/bin/bash` and
      record the current failing count per file. Do not trust the table's numbers without this.
- [ ] **Step 2: Category (c) first** — `test_dispatch_capability.sh` and
      `test_gate_execution_posture.sh`. Rewrite locators to match the consolidated sections. **No
      skill edits**: if you find yourself editing `SKILL.md` to satisfy one of these, you have
      mis-categorised it.
- [ ] **Step 3: Category (a), largest first** — `test_finalize_disposition.sh` then
      `test_finalize_gate.sh`, then the remainder. Invert each retired block with a non-vacuity
      anchor and a comment citing its authority. Where a section fails wholesale for one reason,
      retire the section with one header rather than scattering a dozen comments.
- [ ] **Step 4: `test_sync_agents_run_gate.sh`** — regenerate the AGENTS.md block per the recipe
      the test itself carries; do not hand-edit the block.
- [ ] **Step 5: Mutation pass.** For every guard touched, strip what it guards, confirm it reddens,
      restore. Record the mutations in the results file.
- [ ] **Step 6: Whole suite.** `scripts/run-tests.sh` backgrounded to a log; inspect the complete
      log. Treat `OVER BUDGET:` lines as findings per Task 19 Step 3 — the branch already carries a
      known breach on ~10 files under parallel contention (`test_go_toolchain` 363s vs a 55s
      budget), which is a finding to act on, not a pass/fail cause.
- [ ] **Step 7: Commit** with explicit paths: `git add tests/ && git commit -m "test(0316): migrate
      the finalize-skill tests to the Go sequencer contract"`.

**Halt rather than guess.** If an assertion matches none of the three authorities, it is category
(b) — a real loss. Surface it; do not retire it, and do not restore a deferred capability to make a
test green. Weakening a test to reach green is the one unrecoverable move here.

## Self-Review

Checked against the spec section by section:

- **Purpose/boundary, launch precondition:** external to the plan (host bootstrap verified at claim; see the change file's reconcile log). Tests never require PATH `docket` (Task 17 `TestE2ENoPathDocketDependency`).
- **Chosen architecture / command boundaries:** every listed operation has a task (context finalize 6; retarget 7; rebase/continue/abort 8; publish 9; block/clear-block 11; merge 10; closeout 12; cleanup 14; halt/resume/reclaim 11/13; sweep 15; gate cleanup 14). No `finalize advance`, no phase machine (Task 8 receipt is an effect receipt; Task 16 asserts no hidden state is consulted).
- **Context/selection:** Task 1 ordering + Task 6. **Open-child gate:** Tasks 7 and 10. **Rebase/local gate:** Tasks 3, 5, 8. **Reports:** Task 8 envelope + Task 18 agents. **Publication:** Tasks 5, 9. **Halt/blocked:** Task 11. **Merge:** Task 10. **Terminal transaction/archive:** Task 12. **Stack closeout:** Tasks 2, 12. **Backlinks both modes:** Task 12. **Reclaim:** Task 13. **Sweep:** Task 15. **Cleanup/retention:** Task 14. **Recovery matrix:** Task 16 mirrors the spec table row-for-row. **Testing strategy:** pure (1,2, unit parts of 6–15), real-git (3,5,16), protocol-faithful fake (4), gate/repair/retention (8,14), e2e + mutation (17). **Skills:** Task 18. **Exclusions:** Global Constraints.
- **Type consistency:** `PRFacts`/`FinalizeCandidate`/`MergeConjuncts` (Task 1) consumed in 6/10/15; `RebaseReceipt` (5) in 8/9; `ResolverReport` (8) in 18; `VerifiedMerge` (10) in 12; `FinalizeDeps` introduced in 6 and reused 7–17.
- **Test migration (added post-hoc):** the original plan had NO task for migrating the tests that
  describe the skills Task 18 rewrites. That omission is what turned the Step-5 gate red and
  halted the run; Task 20 closes it and carries the mapping the halt report said was missing.
- **Placeholders:** none — every step names its tests, commands, and closed tokens; implementation steps point at the concrete landed pattern to follow (a deliberate choice for workers in this codebase, matching the accepted 0315 plan style).
