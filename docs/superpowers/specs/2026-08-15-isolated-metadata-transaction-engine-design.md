<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0309 — Isolated metadata transaction engine](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-16-0309-isolated-metadata-transaction-engine.md)**
<!-- docket:backlink:end -->

# Isolated metadata transaction engine

**Change:** 0309 · **Type:** feat · **Priority:** critical · **Date:** 2026-08-15 · **Status:**
Approved design

## Purpose and boundary

This change gives Go one safe execution substrate for durable metadata mutations. One invocation
can base a semantic operation on an exact authoritative revision, work in a private Git index and
checkout, validate the complete before and after states, commit only the declared paths, and update
the target ref only while it still equals the revision the operation read.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) are fixed upstream constraints.
This spec resolves only change 0309's transaction interfaces, isolated-worktree lifecycle,
entity-version checks, commit/push discipline, semantic retry, replay receipts, and recovery
pruning. It does not reopen the two-mode repository model, agent-first boundary, compatibility
contract, hard cutover, or capability decisions in those documents, and it does not implement
behavior assigned to changes 0305–0308 or 0310–0318.

## Landed foundation and independently deliverable result

Changes 0307 and 0308 are complete.

- `internal/domain` supplies immutable entities, typed findings, lifecycle policy, and pure
  before/after decisions. `internal/repository` decodes already-supplied documents into a complete
  snapshot and validation report; `ValidationReport.HasErrors()` is the mutation gate.
- `internal/document` supplies loss-preserving field/block patches and canonical new-document
  bytes. It owns representation safety but no operation or Git behavior.
- `internal/gitcli` discovers one canonical repository from any registered worktree, fetches an
  exact branch revision, opens an immutable `ObjectSource`, and returns full Git object IDs for
  commits and blobs. It exposes no arbitrary command runner.

Change 0309 adds the outward write boundary around those pure/read-only foundations. Its
independently reviewable deliverable is:

- a transaction package under `internal/repository` that coordinates exact-revision attempts,
  typed state loading, semantic mutation planning, complete candidate validation, retry, and
  replay;
- narrow typed additions to `internal/gitcli` for detached transaction worktrees, explicit-path
  commits, exact-ref lease pushes, commit/reachability inspection, and exact trailer reads;
- Docket-owned private transaction directories with versioned ownership manifests and live locks;
- safe cleanup and abandoned-state pruning that never acts from pathname alone; and
- deterministic real-repository tests proving unrelated concurrent writes converge, same-entity
  writes contend, response-loss retries do not duplicate allocation, and the user's checkouts and
  indexes remain untouched.

There is no user-facing metadata command in this change. Change 0310 later supplies the production
source composer used to load a repository from config plus Git objects. Change 0312 supplies the
first real planning operations and renderers. The dependency graph intentionally makes 0312 depend
on both 0309 and 0310; this change freezes the transaction seam they compose through without
stealing either consumer's behavior.

## Chosen architecture

Use a semantic-operation engine with disposable exact-revision attempts:

```text
caller-owned semantic request
          |
          v
transaction.Engine
  fetch exact target ref
  create + lock private detached worktree
  load/validate complete base through StateLoader
  check caller's entity-version expectations
  ask SemanticOperation for a closed MutationPlan
  build/load/validate complete candidate overlay
  materialize + verify only the declared paths
  explicit-path commit + engine-owned trailers
  exact expected-ref lease push
  cleanup private attempt
          |
          v
typed result: applied | no-op | contended | refused | failed
```

The semantic operation is re-evaluated from fresh authoritative state on every retry. It never
hands the engine a patch computed in a previous invocation and never receives a mutable worktree
path or Git client. The engine owns concurrency, Git, filesystem containment, commit scope, and
retry; the operation owns domain intent, authored bytes, renderer content, and its result receipt.

Two alternatives are rejected:

- A byte-patch API prepared by the caller before transaction entry cannot safely retry after an
  unrelated writer advances the ref: its allocation, derived views, and validation facts are
  already stale. Reapplying those bytes is not semantic retry.
- Workflow-aware methods such as `CreateChange`, `Groom`, `Claim`, or `Finalize` inside the engine
  would make 0309 own behavior explicitly assigned to 0312, 0315, and 0316. The transaction engine
  must be able to execute those operations later without defining them now.

A repository-wide advisory lock is also rejected. It cannot coordinate separate machines and
would turn a crashed process into a global write outage. The only locks in this design protect the
ownership and cleanup of one private local resource; Git's remote ref is the cross-machine
serialization point.

## Package and dependency boundaries

Preserve the pure package boundary that landed in 0307.

```text
internal/domain
      ^
      |
internal/repository          internal/document
      ^                             ^
      |                             |
internal/repository/transaction ----+
      |
      v
internal/gitcli
```

- Existing production files in `internal/repository` remain free of filesystem, process, and Git
  access. Its boundary guard continues to protect the read model.
- The new `internal/repository/transaction` subpackage is the outer adapter. It may use
  `internal/repository`, `internal/domain`, `internal/document`, and the typed `internal/gitcli`
  surface. It never imports CLI, render, GitHub, process, install, or harness packages.
- `internal/gitcli` remains the only package that starts Git. Its additions are named operations;
  no exported `Run`, argument-vector escape hatch, or caller-selected working directory is added.
- No package reads the wall clock implicitly for a policy decision. The engine receives a `Clock`
  or explicit timestamps for manifest/commit metadata; tests pin them. Git may record the actual
  commit timestamp supplied by the engine, but semantic operations do not call `time.Now`.

The subpackage boundary is deliberate rather than cosmetic: changing the existing flat
`internal/repository` package to import `os` and `gitcli` would erase the pure read-model invariant
0307 just established.

## Transaction-facing vocabulary

The exact Go file split may follow package conventions, but planning and implementation preserve
these value semantics.

### Engine request

One execution request has the conceptual shape:

```go
type Request struct {
    Repository   gitcli.Repository
    Remote       gitcli.RemoteName
    TargetRef    gitcli.RefName       // fully qualified refs/heads/...
    Expected     []EntityExpectation
    Idempotency  *IdempotencyKey
    Loader       StateLoader
    Operation    SemanticOperation
}
```

The repository, remote, and target ref are already resolved by the application layer. The engine
does not parse `.docket.yml`, choose main versus docket mode, discover configured document roots,
or run the configuration capability preflight. A later mutating application operation must receive
a successful 0305 mutation preflight before calling `Execute`; no transaction directory is created
before that call.

`TargetRef` must be a branch ref. Change 0309 never creates a missing metadata branch, guesses a
default, accepts a tracking ref as authority, or targets a tag. Main mode passes the configured
integration branch; docket mode passes the configured metadata branch. Both use the same engine.

The request and every nested slice/byte value are copied or treated immutably at construction. An
`Engine` and its `gitcli.Client` may be shared by concurrent goroutines; per-attempt state never
lives on either shared object.

### Tree and state loading

The transaction package defines a read-only tree interface with the subset needed by a repository
loader: exact revision identity, recursive literal-path listing, and ordered exact-path blob reads.
An adapter over the landed `gitcli.ObjectSource` implements the base tree. An in-memory overlay of
that base plus a `MutationPlan` implements the candidate tree.

```go
type StateLoader interface {
    Load(context.Context, Tree) (LoadedState, error)
    ValidateEvolution(before, after LoadedState) []domain.Finding
}
```

`LoadedState` contains the complete `domain.Snapshot`, its `ValidationReport`, and read-only access
to the parsed `document.Document` values and source identities an operation needs. Its collections
are immutable or defensively copied. A loader accounts for every authoritative path it was
configured to own; silently omitting an unreadable record is an error, matching 0307's accounting
contract.

The engine, not the operation, enforces both gates:

1. Before planning, `Load(base)` must succeed and the report must contain no error finding.
2. After planning, `Load(candidateOverlay)` plus `ValidateEvolution(before, after)` must succeed
   with no error finding.

Warnings remain part of the loaded state and result diagnostics but do not block mutation. A
loader/programmer failure is distinct from a repository validation refusal. The production loader
and topology-specific source composition arrive in 0310; 0309's real-repository tests use a narrow
test loader backed by the landed `repository.BuildSnapshot`, never a second production composer.

### Semantic operation

The operation contract has the conceptual shape:

```go
type SemanticOperation interface {
    Key() OperationKey
    Plan(context.Context, AttemptState) (MutationPlan, OperationResult)
}
```

`AttemptState` carries the exact base revision, loaded immutable state, and read-only tree. The
operation returns either a typed refusal/no-op or a complete plan. It cannot read a filesystem,
invoke Git, mutate the supplied state, write a commit, push, or clean up a worktree.

`OperationKey` is a stable lowercase token matching `[a-z][a-z0-9.-]*`, for example
`change.groom`; it is not a user-authored label. Later application operations own mapping domain
failures to the protocol-v1 result taxonomy. This engine does not import `internal/app` merely to
reuse outward result strings.

### Mutation plan

```go
type MutationPlan struct {
    Files         []FileMutation
    CommitSubject string
    Receipt       []byte
}

type FileMutation struct {
    Path   gitcli.RepoPath
    Kind   MutationKind // create | replace | delete
    Bytes  []byte       // empty only when the intended file is empty or deleted by Kind
}
```

The plan contains final bytes produced against this attempt's state, not edit instructions for the
engine to interpret. Later operation packages use `internal/document` and their renderers to
produce those bytes. This keeps the transaction layer independent of frontmatter fields, managed
block names, board rows, ADR prose, and workflow statuses while still making the retry semantic:
the operation that generated the plan runs again on every new base.

The engine copies all plan bytes before validation. It rejects duplicate paths, empty/absolute or
non-canonical paths, `.`/`..` traversal, NUL, `.git` and descendants, directory targets, unsupported
object types, create-over-existing, replace/delete-of-absent, and any mutation whose current object
is not a regular Git blob. Existing file modes are preserved; new metadata documents are `100644`.
This change does not create executable metadata files, symlinks, gitlinks, directories as semantic
records, or submodule content.

`CommitSubject` is one non-empty UTF-8 line with control characters forbidden and a fixed 200-byte
maximum. The engine, not the operation, appends all Docket trailers. `Receipt` is canonical compact
JSON encoded from a closed operation-owned struct schema—no maps, insignificant whitespace, or
unknown fields—and is at most 4096 decoded bytes. It contains only the structural values required
to reproduce the operation result—IDs, safe repository paths, and object IDs. Authored Markdown,
secrets, environment values, remote URLs, and unbounded diagnostics are forbidden from receipts.

An empty file list is a no-op and creates no commit. A non-empty list whose materialized bytes
produce no Git delta is invalid-state rather than a successful empty commit: the operation's plan
did not describe reality.

## Entity-version expectations

The caller submits the exact decision inputs that came from an earlier context query:

```go
type EntityExpectation struct {
    Path    gitcli.RepoPath
    Version ExpectedVersion
}

type ExpectedVersion struct {
    Kind     VersionKind // blob | absent
    ObjectID gitcli.ObjectID
}
```

Blob expectations compare the complete Git object ID exactly; they never use an abbreviated SHA.
An absent expectation succeeds only when the exact path is absent from the fetched base tree. An
empty, malformed, duplicate, tree, gitlink, or symlink expectation is invalid input.

Expectations are checked on every attempt before `SemanticOperation.Plan` runs:

- A mismatch on the first attempt means the decision input is already stale and returns
  `contended` without creating a commit.
- If the lease is rejected because an unrelated path changed, the next fetch sees the same
  expected entities, the operation replans, and both writes can converge.
- If the target entity changed while the first attempt was being prepared, the next attempt's
  expectation check fails and returns `contended`; Docket never text-merges authored decisions.

Expectations protect human/model decision inputs. Mechanical whole-repository facts such as the
next available ID or a regenerated board are recomputed by the semantic operation from the fresh
snapshot on every attempt rather than represented by an impractical expectation on every record.
The exact-ref lease still proves the commit was built from one complete base.

## Attempt and worktree lifecycle

### Private root and layout

Transaction state lives below the canonical Git common directory returned by 0308:

```text
<common-dir>/docket/transactions/
  registry.lock
  <random-transaction-id>/
    manifest.json
    live.lock
    hooks/                 # empty, Docket-owned
    worktree/              # detached at the exact fetched commit
```

This layout is inside repository-private Git storage, so it does not appear as an untracked entry
in the primary checkout, the legacy `.docket/` worktree, or a feature worktree. Real Git 2.55 on
the development platform accepts a registered detached worktree at this location; the
cross-platform test matrix must prove the same supported behavior on Darwin and Linux.

Every directory is created with user-only permissions (`0700`), every manifest/lock with `0600`,
subject to a more restrictive umask. The transaction ID uses at least 128 bits from the operating
system cryptographic random source and a lowercase fixed-shape encoding. It is not derived from a
PID, request ID, branch, slug, timestamp, or user-controlled value.

`registry.lock` is held only while publishing a new candidate directory or inventorying candidates
for recovery. It prevents create-versus-prune races; it does not cover mutation work and therefore
does not serialize transactions. Each candidate's `live.lock` is acquired before its manifest is
published and held for that attempt's lifetime. Liveness is the held operating-system lock, never
the PID recorded for diagnostics.

The manifest is versioned JSON and records at least:

- schema version and transaction ID;
- canonical repository common-directory identity;
- target remote and fully qualified target ref;
- exact base commit;
- worktree path relative to the candidate root;
- lifecycle phase (`allocating`, `ready`, `committed`, or `pushed`); and
- created/updated UTC timestamps plus diagnostic PID.

Manifest writes use a same-directory temporary file, file sync, atomic rename, and directory sync.
The phase is recovery evidence, not authority about the remote: recovery and retries always inspect
Git before deciding whether a commit landed.

### Attempt order

For each of at most four total attempts, the engine performs this exact sequence:

1. Fetch `TargetRef` through `gitcli.FetchBranch` and retain its exact commit as `base`.
2. If an idempotency key is present, search the fetched commit's reachable ancestry for its exact
   receipt before allocating local state.
3. Under the short registry lock, allocate the candidate, acquire its live lock, and atomically
   publish the ownership manifest.
4. Add a detached Git worktree at `base`. No local branch is created or reset.
5. Open the landed immutable object source at `base`; load and validate the complete before state.
6. Check every entity expectation, then invoke the semantic operation.
7. Build the candidate overlay, load and validate its complete snapshot, and validate evolution.
8. Materialize the exact plan through a filesystem root anchored at the private worktree. Existing
   bytes outside planned files remain untouched.
9. Re-read every changed path and require exact byte equality with the plan. Ask Git for the actual
   changed-path set and require set equality with the declared paths in both directions.
10. Commit the explicit paths on detached `HEAD`, with user hooks and signing disabled for this
    Docket-owned non-interactive commit. Use the repository's configured Git identity; a missing
    identity is a typed external failure, not silently replaced with a hard-coded person.
11. Push the detached commit to `TargetRef` with the literal expected lease
    `<TargetRef>:<base>`. Never use an implicit tracking lease or `FETCH_HEAD`.
12. Record the observed outcome, remove the exact registered worktree, release the live lock, and
    remove the owned candidate directory.

Filesystem materialization uses `os.Root` or an equivalently containment-safe rooted API. It writes
replacements through sibling temporary files plus atomic rename and never follows an absolute or
escaping symlink. Every parent component must be a real directory rather than a symlink or special
file. A create may make missing parent directories only beneath the rooted worktree and only when
they are needed by a declared file path; directories themselves never enter the Git path set. The
private worktree is not shared, but containment remains required because its base tree is
repository-controlled input.

The Git commit path set uses a NUL-safe pathspec input so spaces, tabs, and newlines cannot split a
path or exceed an argument-vector limit. Deletions, additions, and replacements are all staged and
committed through that one explicit set. The actual-delta equality check is the guard against an
operation bug, accidental generated file, hook, or filesystem side effect widening the commit.

## Commit and exact-ref update

`internal/gitcli` gains transaction-specific operations with the following responsibilities:

- add/remove one detached worktree at a caller-supplied path already proven beneath the owned root;
- report registered worktree identity without parsing human display output;
- return the exact changed-path/status set for a detached worktree;
- create a commit from exactly a supplied NUL-safe path list, with a bounded subject and
  engine-owned trailers;
- push an exact source commit to one fully qualified branch with the exact expected old commit;
- read exact Docket trailers from commits reachable from a supplied revision; and
- test exact commit reachability for ambiguous push recovery.

Each operation uses argument arrays, the controlled environment and deadlines landed in 0308, and
stable typed failures. No operation changes process current directory, repository/global config,
the primary checkout, the persistent `.docket/` checkout, another worktree, or a caller's index.

Commit hooks are disabled per command by pointing `core.hooksPath` at the candidate's empty owned
hooks directory; no shared/worktree Git config is mutated. Commit signing is explicitly disabled so
a global signing preference cannot prompt or make a machine-authored bookkeeping operation
interactive. Credential helpers remain available for the push under 0308's non-interactive policy.

The push result distinguishes three facts without matching human stderr:

1. **Applied:** the push returned success, or an authoritative postcondition probe shows the
   candidate commit reachable from the current remote target.
2. **Lease lost:** the remote target no longer equals `base` and does not contain the candidate
   commit. This is the only retryable condition.
3. **External/unknown failure:** the remote cannot be authoritatively inspected, still equals
   `base` after a failed push, or Git reports another typed failure. This is not contention and is
   never retried as if another writer won.

An explicit post-push probe runs when Git times out, is interrupted after launch, or returns a
non-success for which ref state can still be inspected within the caller's context. If truth cannot
be established, the result says the outcome is unknown and directs a request-ID retry where one is
available; it never claims failure merely because the client missed the response.

## Semantic retry and contention

There are four total attempts: the initial attempt plus at most three semantic retries. This is a
package-owned constant, not configuration and not a time-tuned backoff. Each retry performs a new
authoritative fetch, creates a new candidate, reloads the complete state, rechecks original entity
expectations, and invokes the semantic operation again. It never rebases, merges, resets a previous
worktree onto a new base, or reuses the previous plan.

No sleep is inserted between attempts. A remote ref advance is already the observed serialization
event; sleeping adds latency without proving the next base will remain still. The attempt cap and
caller context bound livelock. A fourth lease loss returns `contended` with the last observed
revision and attempt count.

Retry is forbidden for:

- entity-version mismatch;
- operation refusal or no-op;
- invalid request/plan/receipt;
- before/after validation or evolution failure;
- filesystem containment or actual-delta mismatch;
- commit identity, hook-disable, signing-disable, or commit failure;
- authentication, transport, remote-inspection, cancellation, or timeout failure; and
- cleanup/recovery failure.

This classification is structural. A command failure is never called a lost lease from an stderr
phrase, and a non-zero Git status is never sufficient by itself to prove contention.

## Request-ID idempotency and result receipts

The engine supports an optional idempotency key. Later application boundaries must require it for
every allocation/creation operation, including change and ADR creation. Mechanically idempotent
state transitions may omit it or opt in; 0309 does not enumerate those future operations.

```go
type IdempotencyKey struct {
    RequestID string
    Digest    RequestDigest
}
```

- `RequestID` is case-sensitive, 8–128 ASCII bytes, starts with an alphanumeric byte, and contains
  only alphanumerics, `.`, `_`, or `-`. Newlines, spaces, colons, Unicode, and control bytes are
  rejected so the value cannot alter commit structure.
- `Digest` is `sha256:<64 lowercase hex>` over the operation key and that operation's canonical
  immutable request encoding. The operation/application boundary owns canonical encoding; the
  engine validates and compares the resulting digest.

Every successful transaction commit carries exactly one engine-authored block:

```text
Docket-Transaction-ID: <random transaction id>
Docket-Operation: <operation key>
Docket-Request-ID: <request id>                 # omitted together when no key
Docket-Request-Digest: sha256:<hex>              # omitted together when no key
Docket-Result: <unpadded base64url canonical JSON receipt>
```

`Docket-Transaction-ID`, `Docket-Operation`, and `Docket-Result` are always present. The two request
trailers are both present or both absent. Trailer parsing uses Git's trailer grammar or exact
commit-message parsing and then validates multiplicity, shape, decoded size, JSON canonicality, and
receipt schema. It never uses substring grep over commit prose.

Before local allocation, a keyed request scans the full ancestry reachable from the freshly
fetched target revision; no arbitrary history-depth window may expire idempotency. Outcomes are:

- no exact request ID: proceed;
- exactly one ID with matching operation and digest: decode and return its original receipt as an
  already-applied result, including the authoritative commit ID;
- the ID exists with a different operation or digest: `invalid-input` (`request-id-reused`);
- duplicate receipts, malformed engine trailers, or contradictory matches: `invalid-state`; never
  choose a winner by commit order.

The receipt is the original small structural result, not a fresh reconstruction from today's
repository. This matters for allocated IDs and paths: after a response loss, the caller receives
the ID the successful commit actually created rather than allocating a second one or inferring from
a slug that was never globally unique in Bash-era data.

The random transaction ID separately lets the same invocation probe an ambiguous push. It is not a
cross-invocation idempotency promise: a process that disappears entirely can be resumed safely for
an allocation only through its caller-stable request ID.

## Outcomes and failure posture

The transaction package returns a typed internal result containing at least:

- disposition (`applied`, `already-applied`, `no-op`, `contended`, `refused`, `failed`, or
  `interrupted`);
- operation key and optional request ID;
- base, observed remote, and applied commit IDs when known;
- attempt count;
- decoded operation receipt on applied/already-applied;
- contended expectation paths without their bytes;
- structured validation findings on refusal; and
- cleanup status/warnings.

No caller decides from `error.Error()` or Git stderr. Programmer/call-shape errors use Go errors;
expected repository, domain, contention, external, and interruption outcomes are typed values or
typed package failures. A later application layer maps them to the landed protocol results.

Cleanup does not rewrite history:

- Before a successful push, any failure leaves no durable metadata change. Cleanup is attempted
  after the diagnostic facts have been captured.
- After a successful or authoritatively verified push, the result remains applied even if local
  cleanup fails. It carries a `cleanup-pending` warning and the owned transaction identity; it is
  never relabelled failed, because the promised remote state already exists.
- A lost/unknown response with a stable request ID tells the caller to retry that same request. The
  next invocation searches remote history before allocating anything.

Commit, push, manifest, and error diagnostics are bounded and redact under 0308's adapter policy.
No object bytes, request receipt before validation, credentials, remote URL, environment, or
arbitrary subprocess output is logged.

## Cleanup and recovery pruning

Normal cleanup targets only the current manifest-proven candidate. It captures all diagnostics
needed by the result before removing files, asks Git to remove that exact registered worktree, and
then removes the candidate through a root anchored at the transactions directory. Failure at one
stage is reported; cleanup does not broaden to a global prune.

The package also exposes a local `PruneAbandoned` operation for later explicit maintenance. It is a
mechanical local-resource operation, not change 0316's repository lifecycle sweep. Under the short
registry lock it inventories the transactions root, then evaluates each candidate before deleting
any candidate:

1. Directory name and permissions have the owned shape.
2. `manifest.json` is a supported schema and every identity/path field is canonical.
3. Manifest repository identity equals the current canonical common directory.
4. The candidate and worktree resolve beneath the owned transactions root.
5. Git registration is absent or names exactly this worktree and common directory.
6. The candidate `live.lock` can be acquired non-blocking.

A held lock is live regardless of timestamp or PID and is skipped. An unsupported/malformed
manifest, mismatched identity, foreign directory, unexpected symlink, ambiguous registration, or
unresolvable path is reported and left byte-untouched. Absence of a manifest never certifies
ownership. No age threshold overrides a held lock, and PID liveness is never consulted.

For a valid abandoned candidate, recovery inspects the target ref and candidate commit before
deletion so its report can distinguish unpushed residue from an already-reachable pushed commit.
It may remove the exact registered worktree and owned files; it never resets a branch, deletes a
commit/ref, removes a feature worktree, enters the legacy `.docket/` worktree, or invokes global
`git worktree prune`. If Git cannot safely remove stale administrative state by exact identity, the
candidate remains with a diagnostic rather than deleting internal Git directories by guessed
pathname.

Recovery continues across independent valid candidates but returns a complete deterministic report
of pruned, live, foreign, malformed, and cleanup-failed entries. Within one candidate it validates
the complete ownership proof before the first destructive step, so a late malformed field cannot
produce a half-authorized deletion.

## Testing strategy

### Interface and pure tests

Package tests use immutable fakes for the loader and semantic operation to cover:

- request, operation-key, entity-expectation, plan, commit-subject, receipt, and trailer validation;
- before-report, after-report, and evolution-error gates;
- create/replace/delete semantics and exact before/after mode rules;
- duplicate and hostile paths, `.git` paths, traversal, symlinks, gitlinks, and directory targets;
- four-attempt accounting and the rule that only a structurally proven lease loss retries;
- typed outcome mappings and the post-push cleanup-warning posture; and
- defensive copying/concurrent engine use under the race detector.

The test `StateLoader` uses the landed `document.Parse` and `repository.BuildSnapshot` over a small
complete Docket corpus. It proves the engine invokes complete validation at both gates without
shipping the production config/Git source composer owned by 0310.

### Real Git topology harness

Real-repository tests create local bare remotes and independent non-bare clones for both supported
topologies:

1. main mode, with metadata and code on the default/integration branch;
2. docket mode, with `.docket.yml` on the default branch and an orphan metadata branch.

Using independent clones is required for concurrency proof: two writers that share one Git common
directory do not model separate machines. Before every scenario, record each invocation checkout's
branch/detached state, `HEAD`, index entries, staged/unstaged bytes, and untracked files. After the
transaction, require all properties byte-identical in both clones; only remote objects/refs and
private Docket transaction state may move.

### Required concurrency and interruption matrix

Tests coordinate writers with explicit channels/barriers, never sleeps:

- **Unrelated writers:** both plan from revision A; writer 1 changes record X and pushes; writer 2's
  lease loses, it fetches, verifies its expected record Y is unchanged, replans from B, and pushes
  a commit containing both outcomes. Its first plan bytes must not appear in the second commit.
- **Same entity:** both expect blob X1; writer 1 pushes X2; writer 2's retry sees X2 rather than X1
  and returns `contended` without a commit.
- **Derived overlap:** two otherwise unrelated operations both regenerate one derived file. The
  second operation replans the derived bytes from fresh state so the view contains both primary
  changes; it never resolves the file with a text merge.
- **Four lease losses:** a controlled writer advances the ref before each push. Exactly four
  attempts occur, every earlier candidate is cleaned, and the outcome is contended.
- **Lost response:** the remote accepts an allocating commit while the client observes no success.
  Repeating the same request ID returns the original allocated result and leaves exactly one
  matching receipt/record in history.
- **Request-ID misuse:** same ID with a different operation or digest is refused; duplicate or
  malformed historical receipts are invalid-state.
- **Cancellation:** cancel before commit, during local Git, and during push. Prove no pre-push
  cancellation changes the remote; for an ambiguous push, prove the postcondition/next-request
  path never duplicates work.
- **Materialization failure:** fail each write/rename/readback/delta/commit step and prove no push
  occurs and unrelated bytes remain exact.

### Ownership and recovery matrix

- Two active transactions in one clone hold distinct worktrees and indexes and can plan
  concurrently.
- A live candidate's held lock defeats pruning even with an ancient timestamp or dead-looking PID.
- A normally abandoned candidate is removed after exact manifest, root, registration, and lock
  checks.
- Missing, truncated, reordered-schema, wrong-repository, path-escaping, symlinked, foreign, and
  ambiguous-registration candidates survive byte-identically and appear in the report.
- Recovery started during candidate creation cannot observe/delete the half-published directory;
  the registry lock orders the two operations.
- Forced worktree-removal failure retains enough state and diagnostics for a later retry.
- No test or production path invokes global worktree prune or touches a feature/legacy metadata
  worktree.

### Guards and mutation evidence

Each safety predicate is mutation-tested. At minimum, remove or weaken:

- the exact entity-version comparison;
- one direction of actual-path-set equality;
- explicit pathspec use at add or commit;
- the literal expected old ref in the push lease;
- fresh re-planning on retry;
- the request-history lookup or digest comparison;
- before, after, or evolution validation;
- root containment, manifest identity, registration identity, or live-lock check; and
- post-push applied-versus-cleanup-failed classification.

Each mutation must redden a focused test after proving the mutation landed. Go reruns use
`-count=1`; concurrency-sensitive package tests also pass under the existing repo-wide race gate.
The build gate runs the complete resolved suite through `scripts/run-tests.sh` and acts on every
`OVER BUDGET:` report. Existing Go producers already auto-discover the new package, so this change
adds no second shell test runner.

## Explicit exclusions

Change 0309 does not implement:

- configuration loading, topology selection, capability diagnostics, or mutation preflight (0305);
- new document patch/render behavior (0306);
- new lifecycle/domain rules or snapshot semantics (0307);
- general Git discovery/object behavior already owned by 0308, or an arbitrary Git runner;
- read-only repository source composition, status context, selection/health presentation, human
  text, protocol JSON, or mutating maintenance commands (0310);
- installation, assets, or harness rendering (0311);
- change create/groom/block/defer/kill, manual learning, ADR operations, inline board, artifact
  links, backlinks, or ADR-index rendering (0312);
- feature workspaces/branches, effective-base creation, GitHub/`gh`, pull requests, external-effect
  probes, or build evidence (0313);
- process sessions/groups, gate launch/observe/stop, or durable run directories (0314);
- claim/reconcile/plan/results/implemented workflow orchestration or agent dispatch (0315);
- finalize/rebase/merge/archive/reclaim/stack/sweep policy or terminal cleanup (0316);
- release packaging/acceptance (0317) or self-hosting/cutover (0318).

It also excludes a persistent shared Go metadata checkout, cross-repository/global locks, a daemon,
database, generic filesystem transaction API, user-facing transaction CLI, automatic repair of
malformed records, automatic merge of authored Markdown, signing, submodule/LFS materialization,
repository migration, and deletion of any worktree or directory identified only by path.

ADR-0001 and ADR-0034 remain the stored topology and repository-identity context. ADR-0089's
shared-worktree retry posture remains the Bash product's historical choice; the later approved Go
architecture deliberately replaces that mutable shared index with the isolated mechanism in this
spec. This change does not rewrite or reopen either record.

## Acceptance boundary

Change 0309 is complete when the resolved whole suite proves that two independent Go writers can
mutate the same remote metadata branch without sharing an index or working directory: unrelated
semantic changes converge through fresh-state re-planning, same-entity changes return typed
contention, exact-path commits cannot widen, request-ID replay after a lost response returns the
original result exactly once, and abandoned local state is pruned only after manifest plus live-lock
ownership proof.

The proof covers both metadata topologies, hostile repository paths, push rejection and ambiguous
response boundaries, cancellation, cleanup failure, concurrent recovery, and byte-identical
preservation of every user checkout and index. No planning workflow, renderer content, feature
workspace, GitHub effect, process supervisor, implementation/finalize behavior, release, or cutover
work owned by changes 0310–0318 is required for that proof.
