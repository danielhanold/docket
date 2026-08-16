<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0313 — Workspaces, GitHub PR adapter, and build evidence](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-16-0313-workspaces-github-pr-adapter-and-build-evidence.md)**
<!-- docket:backlink:end -->

# Workspaces, GitHub PR adapter, and build evidence

**Change:** 0313 · **Type:** feat · **Priority:** critical · **Date:** 2026-08-16 · **Status:**
Approved focused design

## Purpose and boundary

This change gives later agent workflows deterministic mechanics for one feature branch without
turning Go into a workflow engine. It prepares and inspects a long-lived feature worktree, publishes
the branch without overwriting divergent remote work, creates or updates one GitHub pull request
idempotently, and reads and writes the build-evidence block that certifies an exact tested commit.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) are fixed upstream constraints.
This spec resolves only change 0313's typed feature-workspace, feature-branch publication, GitHub
CLI, pull-request publication, and build-evidence interfaces. It does not reopen the program's
agent-first boundary, one-shot CLI protocol, compatibility contract, two repository modes, hard
cutover, external-command, effective-base, metadata-transaction, or deferred-capability decisions.

In particular, this change consumes the effective base already produced by
`domain.ResolveEffectiveBase`; it does not copy or reinterpret stack policy. It consumes the
repository identity and typed Git operations landed by 0308 and the exact-path/ownership discipline
landed by 0309; it does not widen the metadata transaction engine into a feature-workspace or
GitHub transaction.

## Landed foundation and independently deliverable result

Changes 0308 and 0309 are complete on `main`.

- `internal/gitcli` owns canonical repository discovery from any registered worktree, controlled
  direct Git execution, full object IDs, fully qualified refs, exact remote fetches, worktree
  inventory, changed-path inspection, reachability checks, exact-path commits, and lease pushes.
  It exposes named operations rather than an arbitrary command runner.
- `internal/domain` already owns stack traversal and returns a tagged `EffectiveBase`; ADR-0092's
  done-versus-stacked-merged rule is implemented and tested there.
- `internal/repository/transaction` owns short-lived detached metadata worktrees, expected-ref
  leases, semantic retries, request receipts, and recovery beneath
  `<common-dir>/docket/transactions/`. Its worktrees are not feature workspaces and stay untouched.
- `internal/document` already provides whole-population managed-marker validation and
  loss-preserving block replacement. Build-evidence handling can reuse that safety boundary rather
  than implementing a second permissive marker scanner.

Change 0313's independently reviewable deliverable is:

- narrow additions to `internal/gitcli` for named feature-branch worktree creation, clean removal,
  authoritative remote-branch probing, and non-forcing branch publication;
- `internal/workspace`, which persists ownership and preparation state separately from the
  checkout, prepares/inspects/cleans one `.worktrees/<slug>` feature workspace, and never resets or
  adopts an unproven branch or directory;
- `internal/githubcli`, a typed direct-`gh` adapter plus one idempotent ensure-PR operation using an
  explicit GitHub repository identity and exact head/base facts;
- `internal/evidence`, a strict codec, loss-preserving PR-body block writer, and exact-head verifier
  for the retained build-evidence schema; and
- hermetic real-Git and protocol-faithful fake-`gh` tests covering interruption and response-loss
  boundaries without creating a PR in the maintainer's repository.

There is no user-facing workflow command in this change. Change 0315 owns the application/CLI
operations that assemble authoritative change context, pass the resolved effective base into these
mechanics, run agent steps, attach artifacts, publish the PR, and transition metadata to
`implemented`. Freezing the mechanical request/result seams here keeps that later workflow change a
composition rather than a second implementation of Git and GitHub behavior.

## Chosen architecture

Keep the three kinds of state separate and compose them through typed values:

```text
authoritative snapshot + remote branch facts
                 |
                 v
       domain.ResolveEffectiveBase       (already landed; policy)
                 |
                 v
       workspace.Prepare / Inspect       (local owned workspace)
                 |
                 +-----------------------> workspace.PublishHead
                 |                              |
                 |                              v
                 |                        exact remote head
                 |                              |
                 v                              v
         feature agents edit              githubcli.EnsurePullRequest
         and commit here                   probe -> create/edit -> verify
                 |
                 v
       evidence.Record / Upsert / VerifyExact
```

The workspace service owns local persistent state. `gitcli` owns Git processes. `githubcli` owns
`gh` processes and GitHub response decoding. `evidence` owns the trusted record shape but neither
runs a suite nor decides finalize policy. No package receives a generic subprocess runner or a
mutable metadata transaction worktree.

This is the recommended layered approach. Two alternatives are rejected:

1. **One `external-effects` operation that prepares, pushes, opens a PR, and edits metadata.** It
   would make a partially completed feature build look like one transaction even though local Git,
   the remote feature ref, GitHub, and the metadata ref cannot commit atomically. It would also pull
   claim-to-implemented orchestration from 0315 into this change.
2. **Reuse `repository/transaction.Engine` for feature workspaces or PR receipts.** That engine is
   deliberately a short-lived detached metadata writer whose correctness comes from a closed file
   plan and one exact target ref. A long-lived checked-out feature branch and a GitHub PR have
   different ownership, mutation, and recovery rules. Combining them would weaken both boundaries.

A package-private process interface was also considered for `gh` tests. It is unnecessary: like
`gitcli`, `githubcli.Client` can resolve an explicit executable, so tests exercise the production
argument, stdin, environment, timeout, and JSON-decoding path through a controlled fake executable.

## Package and dependency boundaries

The intended dependency direction is:

```text
internal/domain            internal/document
       ^                          ^
       |                          |
internal/workspace         internal/evidence
       |
       v
internal/gitcli            internal/githubcli
```

- `internal/gitcli` remains the only package that starts Git. Its new methods are named feature
  operations; it still exports no `Run`, arbitrary argv, caller-selected environment, or generic
  working-directory escape hatch.
- `internal/workspace` may import `domain` only for `EffectiveBase` and `ChangeID` value semantics.
  It does not load config, documents, a repository snapshot, or GitHub state, and it never starts a
  process except through `gitcli`.
- `internal/githubcli` starts only `gh`. It does not import `workspace`, `repository/transaction`,
  config, document, process, install, harness, or CLI packages.
- `internal/evidence` may reuse `document` for marker validation and exact source patching. It has
  no GitHub, filesystem, process, workflow, or configuration behavior.
- `internal/app` and `internal/cli` gain no workspace/PR workflow in this change. Their future 0315
  composition maps these packages' closed outcomes into the already-landed protocol-v1 result
  taxonomy.
- Existing production files in `internal/repository` and `internal/repository/transaction` are
  unchanged. A boundary test should keep the workspace/GitHub packages from leaking inward.

## Shared typed vocabulary

The exact Go file split may follow package conventions, but planning and implementation preserve
the following value semantics.

### Feature target

A caller supplies a change identity, the conventional feature ref, and a resolved effective base:

```go
type Target struct {
    ChangeID     domain.ChangeID
    Slug         string
    FeatureRef   gitcli.RefName       // refs/heads/feat/<slug>
    EffectiveBase domain.EffectiveBase
}
```

Construction rejects a non-positive change ID, invalid slug, a feature ref not exactly derived
from that slug, and any effective-base outcome other than `domain.BaseResolved`. The base's branch
is converted once to a fully qualified `refs/heads/...` value and validated by `gitcli`; an empty or
malformed branch is not treated as the integration branch.

The target deliberately contains no stack parent ID and no alternative base-selection rules. A
caller obtains `EffectiveBase` from the landed resolver against an authoritative snapshot and
remote branch facts. This service merely spends that decision. Tests wire real resolver outcomes
for unstacked, live-parent, done-parent, and recursively stacked-merged cases into the same target
constructor, proving that workspace and PR bases consume the resolver rather than shadowing it.

### Closed outcomes

Each effect returns a disposition plus facts, not a bare error or prose token:

- workspace prepare: `created`, `existing`, `resumed`, `contended`, `blocked`, or `failed`;
- workspace cleanup: `cleaned`, `already-clean`, `blocked`, or `failed`;
- branch publication: `published`, `already-published`, `contended`, `unknown`, or `failed`;
- PR publication: `created`, `adopted`, `updated`, `unchanged`, `contended`, `unknown`, or `failed`;
- evidence verification: `verified`, `missing`, `malformed`, or `stale`.

`unknown` is load-bearing. It means an irreversible or externally durable effect may have landed
but the adapter could not establish the postcondition. A later caller retries the same semantic
request; it never interprets `unknown` as safe permission to create again.

Package failures carry an operation/stage, a stable kind (`invalid-input`, `invalid-state`,
`external`, `cancelled`, `timed-out`, or `invalid-output`), and bounded safe detail. They never
include environment values, tokens, PR body bytes, remote URLs with credentials, or unbounded
stderr. Expected contention and already-applied states are ordinary outcomes, not string-matched
errors.

## Feature workspace design

### Location and durable ownership

The checked-out feature workspace stays at the retained, harness-visible path:

```text
<primary-worktree>/.worktrees/<slug>/
```

Repository discovery always supplies the canonical primary worktree, even when the caller starts
inside `.docket/`, another feature worktree, or a symlinked spelling. No path is derived from the
process CWD. The target path is built from the canonical primary worktree and a validated slug,
then canonicalized component by component before any identity comparison.

Ownership state lives outside the checkout and outside the metadata transaction tree:

```text
<common-dir>/docket/workspaces/
  registry.lock
  <sha256-of-feature-ref>/
    operation.lock
    manifest.json
```

The deterministic hashed directory gives a retry a stable lookup without placing a caller-derived
branch string in a filesystem path. Directories are `0700` and files `0600`, subject to a more
restrictive umask. `registry.lock` covers only first publication of a manifest; the per-workspace
lock serializes prepare, inspect-state refresh, branch publication, and cleanup for that one
workspace. No lock is held while an agent builds, and no PID or timestamp is treated as proof that
an agent is or is not alive.

The versioned JSON manifest records at least:

- schema version and stable workspace ID;
- canonical Git common-directory identity;
- change ID and slug;
- fully qualified feature and base refs;
- exact base commit used at initial preparation;
- canonical workspace path;
- phase (`allocating`, `ready`, or `cleaned`); and
- created/updated UTC timestamps for diagnostics.

Manifest writes use a same-directory temporary file, file sync, atomic rename, and directory sync.
The manifest is provenance, not an oracle: every operation also checks live Git worktree
registration, branch identity, and object reachability. Pathname, conventional branch name,
directory existence, PID, or age alone never certifies ownership.

A cleaned manifest is retained as a small tombstone. This makes response-loss cleanup retryable and
allows a later authorized resume to distinguish a Docket-created branch from a coincidental local
branch without retaining a checkout. Terminal branch deletion and eventual tombstone retirement
are 0316 policy, not this change.

### Prepare request and result

Preparation has the conceptual shape:

```go
type PrepareRequest struct {
    Repository gitcli.Repository
    Remote     gitcli.RemoteName
    Target     Target
}

type Workspace struct {
    ID          string
    Path        string
    FeatureRef  gitcli.RefName
    BaseRef     gitcli.RefName
    BaseCommit  gitcli.ObjectID
    HeadCommit  gitcli.ObjectID
    Dirty       bool
    Disposition PrepareDisposition
}
```

The service defensively copies inputs and exposes no mutable client, manifest, or environment.
`Prepare` is safe to call repeatedly and follows this order:

1. Validate the repository identity and target before creating a directory or branch.
2. Acquire the per-workspace operation lock, creating and atomically publishing an `allocating`
   manifest under the short registry lock when this is the first attempt.
3. Fetch the supplied resolved base branch from the selected remote and retain the exact commit.
   The service never uses a cached tracking ref after a failed freshness operation.
4. Inventory the exact target path, worktree registration, local feature ref, and manifest phase.
5. For a manifest-proven ready workspace, verify that Git registers that exact canonical path on
   the exact feature ref and that its HEAD is the ref's current commit. Return `existing` without
   checkout, reset, clean, stash, or byte mutation. Dirty and untracked state is reported, not
   repaired.
6. For a manifest-proven interrupted allocation, resume only the missing suffix. A branch already
   created by this manifest must still contain the recorded base commit; an attached worktree must
   match the manifest and Git registration. Preserve any commits or dirty bytes that appeared
   after worktree creation.
7. On a fresh allocation, require the local feature ref, remote feature ref, target path, and Git
   registration all to be absent. Add one branch-attached worktree at the exact fetched base, using
   the conventional feature ref. Never create with `-B`, reset an existing branch, or force-remove
   a path to make room.
8. Reinspect the resulting registration, ref, HEAD, and ancestry, then atomically advance the
   manifest to `ready` and return the verified facts.

An existing path or worktree registration with no matching manifest is `blocked` and left
byte-untouched. A local or remote feature branch present before first manifest publication is also
`blocked`; this change does not adopt pre-Go in-flight work or guess ownership from `feat/<slug>`.
The migration compatibility contract covers valid quiescent `v0.9.2` repositories, not an active
Bash agent's half-built branch.

Git hooks and ordinary feature-branch commit behavior remain enabled. The empty-hooks and
no-signing policy from metadata transactions does not apply to agent-authored feature commits.

### Inspect

`Inspect` is read-only. It returns the manifest phase, canonical path, Git registration, symbolic
branch, branch and workspace HEADs, recorded base, ancestry result, and exact dirty/staged/untracked
path summary. A malformed or foreign manifest is data in a typed invalid-state result; inspection
does not delete, repair, reset, fetch, or normalize anything.

Inspection distinguishes at least:

- ready and internally consistent;
- cleaned with no registered worktree;
- allocating and safely resumable;
- dirty but owned;
- missing or moved branch;
- path/registration/manifest mismatch; and
- foreign or malformed state.

Later agent dispatchers use the returned canonical path. They never reconstruct
`.worktrees/<slug>` independently, and they can compare it with Git's registered worktree set before
handing it to a feature-scoped worker under ADR-0083.

### Clean workspace removal

`Cleanup` removes only the checkout, not either local or remote branch. Before any removal it
requires the complete manifest and registration proof above, confirms the workspace is on the
recorded feature ref, and asks Git for the exact tracked/untracked delta.

A dirty, untracked, conflicted, detached, moved-HEAD, or mismatched workspace is `blocked`. Clean
removal uses a new non-forcing `gitcli` operation so Git itself rechecks cleanliness at the
destructive boundary; a preflight check followed by the existing force-removal method would leave a
race in which a worker writes between the two calls and loses data. After Git confirms removal, the
manifest advances atomically to `cleaned`. A retry that sees the cleaned manifest plus no exact
registration returns `already-clean`.

Cleanup never invokes global `git worktree prune`, deletes a branch, removes Git administrative
directories by pathname, enters the legacy `.docket/` worktree, touches a transaction worktree, or
widens from one manifest to an inventory sweep. Finalize's decision to clean after merge, recover a
dirty abandoned build, delete local/remote branches, or sweep multiple workspaces belongs to 0316.

## Feature-branch publication

Opening a PR requires GitHub's head ref to contain the exact commit the caller intends to review.
This is a separate idempotent workspace operation, not an incidental `git push` hidden inside
`gh pr create`.

`PublishHead` accepts an owned ready workspace and:

1. reinspects ownership, exact feature-branch attachment, HEAD, and dirty state;
2. refuses a dirty or inconsistent workspace;
3. probes the authoritative remote feature ref structurally;
4. returns `already-published` when it already equals the local feature HEAD;
5. creates the missing remote ref under an explicit absent-ref lease;
6. fast-forwards an existing remote ref only when its exact commit is an ancestor of local HEAD,
   under an explicit expected-old lease; and
7. refuses divergence as `contended` — no force push, reset, merge, or rebase.

After every push result that is not structurally conclusive, the operation probes the remote ref
again within the remaining caller budget. Exact equality with the intended head is `published`;
an observed different commit is `contended`; an unobservable remote is `unknown`. A retry starts
with the same authoritative probe and therefore adopts a push whose response was lost instead of
pushing or creating twice.

The promise is **the exact commit reached the exact remote feature ref**. A clean local tree, a
local branch, an upstream configuration, or a successful process exit is not the idempotency key.

`internal/gitcli` gains only the named primitives needed for this flow:

- add one branch-attached worktree at an exact start commit;
- attach one manifest-proven existing local branch to a worktree;
- remove one worktree without `--force`;
- probe one fully qualified remote branch and return found/absent plus full object ID;
- push one exact commit to one fully qualified branch with an absent or exact expected-old lease,
  never forcing a non-fast-forward update; and
- report structural push outcomes and exact postcondition probes without matching human stderr.

The existing detached transaction operations keep their semantics. No generic branch create,
checkout, reset, clean, rebase, merge, delete, or force-push escape hatch is exported.

## GitHub CLI adapter and PR publication

### Client and repository identity

`githubcli.Client` mirrors `gitcli.Client`'s execution posture:

- resolve the `gh` executable once, with an explicit executable option for tests;
- invoke it directly with argument arrays and controlled working directories, never a shell;
- apply bounded local/network deadlines and reap a cancelled process before returning;
- keep normal GitHub authentication channels available but never include token values in results;
- capture stdout and stderr separately, bound and redact diagnostics, and never echo PR body stdin;
  and
- expose no arbitrary command runner.

One discovery operation resolves the current checkout's GitHub `host/owner/repository` identity
through `gh repo view` from the canonical primary worktree. This is the only call allowed to infer a
repository from Git context. It returns a validated typed identity; every PR list/view/create/edit
call thereafter passes that identity explicitly with `--repo`, so a caller's CWD, `GH_REPO`, or a
different checked-out worktree cannot retarget the effect.

Repository, branch, title, and body values are never interpolated into a shell command. Authored
Markdown reaches `gh` through stdin (`--body-file -`) rather than an argv value or temporary file.

### Pull-request value and version

The adapter returns a typed snapshot:

```go
type PullRequest struct {
    Number      int
    URL         string
    State       State      // open, closed, merged
    Draft       bool
    HeadBranch  string
    HeadCommit  string      // full GitHub-reported object ID
    BaseBranch  string
    Title       string
    Body        string
    Version     string      // sha256 over the exact mutable snapshot above
}
```

The opaque version is a local optimistic-concurrency token. It contains no body bytes and is
recomputed from the latest authoritative response. GitHub does not offer this flow a server-side
compare-and-swap edit, so the token cannot eliminate a human edit racing in the interval between
probe and update; it does ensure Docket never deliberately overwrites a body/base/title different
from the exact snapshot its caller approved, and the post-edit verify detects a race. That residual
is reported honestly rather than disguised as atomicity.

JSON is decoded from `gh`'s documented fields, not from display text, jq fragments, stderr, or a
flattened fake-only shape. Missing required fields, duplicate/ambiguous candidates, malformed full
object IDs, or an unexpected enum are `invalid-output`/`invalid-state`, never zero-value data.

### Ensure request

The idempotent publication request has this conceptual shape:

```go
type EnsurePullRequestRequest struct {
    Repository      Repository
    HeadBranch      string
    ExpectedHead    string
    BaseBranch      string
    Title           string
    Body            string
    ExpectedVersion string // empty for create-or-adopt; required to update a mismatch
}
```

The expected head comes from a successful `workspace.PublishHead` result. The adapter refuses to
create or update when GitHub reports a different head commit. Base branch is the same resolved
effective-base branch supplied to workspace preparation; the adapter never guesses `main` or walks
the stack itself.

Go v1 opens ready-for-review PRs. Draft conversion, review submission, checks polling, comments,
merge, and close/reopen are not part of this request. An existing draft is returned as invalid
state for the later workflow/human to resolve rather than silently changing its review state.

### Probe, act, verify

`EnsurePullRequest` follows one fixed recovery sequence:

1. Query all PRs for the explicit repository and exact head branch, including terminal states.
2. More than one open PR is ambiguous and blocks. A closed or merged same-head PR with no open PR
   also blocks; Docket does not create a duplicate history after a terminal PR.
3. If one open PR already matches head commit, base, title, body, and ready state, return `adopted`
   or `unchanged`. This is the lost-create-response recovery path.
4. If one open PR differs, require its current opaque version to equal `ExpectedVersion` before
   editing. An empty or mismatched expected version is `contended`, leaving the PR untouched.
5. If no PR exists, create it with explicit head/base/repository/title and body on stdin.
6. After create or edit, query by PR number and by head branch and require both views to name one
   open, ready PR whose head commit, base, title, and body equal the request.
7. If `gh` times out, is cancelled after launch, or exits unsuccessfully after a mutation may have
   reached GitHub, requery. Exact postcondition means `created`/`updated`; an observed different
   state means `contended`; inability to establish truth means `unknown`.

There is no automatic compensating close, reopen, body rollback, base rollback, or second create.
On retry the initial probe is sufficient to adopt the already-created or already-updated PR.

PR URLs and numbers are returned as verified data for 0315's later metadata transaction. This
change does not write `pr:`, `status: implemented`, `claimed_at:`, `results:`, the board, or any
other planning record.

## Build-evidence mechanics

### Retained record

`internal/evidence` preserves ADR-0066's four-field, marker-bounded schema:

```text
<!-- docket:build-evidence:start -->
command:  <exact full-suite command>
result:   green
head_sha: <full 40- or 64-hex object ID>
ran_at:   <UTC RFC3339 seconds>
<!-- docket:build-evidence:end -->
```

The Go value is immutable:

```go
type Record struct {
    Command string
    Result  Result
    Head    string
    RanAt   time.Time
}
```

Validation requires valid UTF-8, a non-empty single-line command with no control bytes,
`result == green`, a normalized full lowercase 40- or 64-hex object ID, and a UTC second-precision
timestamp. A red, interrupted, running, or unavailable gate creates no record. The evidence package
does not infer green from output text, run a command, read a gate artifact, or accept `128+signal`
heuristics; 0314/0315 supply the already-decided terminal gate outcome.

Parsing requires exactly one start/end pair outside Markdown code fences, no nesting, duplicate,
dangling, malformed, or out-of-order Docket marker anywhere in the body, exactly one occurrence of
each known key, no unknown keys, and no nonblank extra lines. Keys are split only at their first
colon so an exact command may contain later colons. CRLF and LF inputs are accepted; canonical new
blocks use LF.

### Loss-preserving body update

The package provides three separate operations:

- `Render(Record)` returns one canonical complete block;
- `Extract(body)` returns the strict record from an existing PR body; and
- `Upsert(body, Record)` replaces only the validated block interior when present, or appends one
  canonical block when absent, preserving every pre-existing body byte outside the insertion.

`Extract` and `Upsert` reuse `document.Parse`'s whole-population, fence-aware marker validation.
Replacement uses the landed patch API and reparses the candidate before returning it. The absent
case validates the original population, appends with a deterministic blank-line boundary, reparses
the candidate, and returns no bytes if either parse fails. A backlink, review narrative, findings
table, human edit, or other managed block is never normalized or reconstructed by this package.

The updater does not own the complete PR body. Change 0315 may assemble authored prose and use this
operation before submitting an explicit expected PR version; 0316 may read the same block while
deciding its finalize gate. A malformed marker population blocks both rather than consuming to EOF
or overwriting human content.

### Verification boundary

`VerifyExact(record, head)` is deliberately narrow:

1. the record parsed successfully;
2. its result is `green`; and
3. its full `head_sha` equals the exact supplied branch HEAD.

The supplied head is obtained authoritatively through Git, never from the current directory or a
PR title/body claim. The result distinguishes missing, malformed, and stale so later application
code can map them without parsing prose.

The results-only strict-ancestor exemption from ADR-0066's 2026-08-07 update is **not** implemented
here. It is a finalize decision requiring configured repository policy plus a fresh, no-renames Git
delta at merge time, and remains 0316's scope. This package preserves full object IDs and record
bytes so that later consumer can implement the policy without weakening exact verification here.

## Failure, cancellation, and recovery posture

The common rule is probe the promised postcondition, not a local proxy:

| Effect | Promise | Authoritative retry probe |
|---|---|---|
| Workspace prepare | exact registered path is attached to exact feature ref and contains recorded base | manifest + `git worktree list --porcelain -z` + refs/reachability |
| Workspace cleanup | exact owned registration is absent and manifest is `cleaned` | manifest + exact worktree registration |
| Branch publication | remote feature ref equals intended full head | remote-ref query |
| PR creation/update | one open PR has exact repo/head OID/base/title/body | PR list + PR view JSON |
| Evidence write | one balanced block reparses to the requested record | parse returned body bytes |

An operation validates every ownership/input fact before the first mutation. It captures any
diagnostic needed by the result before cleanup. Cancellation before an external launch returns
interrupted with no claimed effect; cancellation or timeout after launch triggers the same bounded
postcondition probe where budget remains, otherwise `unknown`. No retry uses a previous mutable
worktree snapshot or a cached GitHub response as authority.

Only prepare resumes a manifest-proven partial local allocation. No operation auto-adopts a foreign
directory, resets a colliding branch, force-pushes divergence, creates a second PR, or deletes data
to make a retry look clean.

## Testing strategy

### Pure contract tests

Table-driven tests cover:

- target validation and exact `feat/<slug>` derivation;
- consumption of every tagged `domain.EffectiveBase` outcome without a fallback;
- manifest schema, canonical identities, phase transitions, and atomic write recovery;
- closed disposition/failure mappings and nil/non-nil collection hygiene;
- PR repository/branch/value/version validation;
- evidence render/parse round trips, 40/64-hex heads, timestamps, duplicate/missing/unknown keys,
  CRLF, colons in commands, code-fenced marker examples, and every marker imbalance; and
- loss-preserving evidence replacement/insertion with byte equality outside the owned block.

Each guard gets a negative test that removes or violates the premise and proves the operation
fails before mutation. Marker, ownership, branch-ancestry, and postcondition gates are
mutation-tested directly; a green happy-path suite alone is not evidence they bite.

### Real-Git workspace matrix

Use temporary real repositories and local bare remotes. Cover both metadata topologies even though
feature workspace mechanics are topology-neutral: main mode with one planning/code branch, and
docket mode with an orphan metadata branch plus primary, `.docket/`, transaction, and feature
worktrees registered concurrently.

The matrix proves:

- invocation from the primary checkout, `.docket/`, another feature worktree, a nested directory,
  and symlinked spellings resolves one canonical workspace location;
- an unstacked change starts at the fetched integration branch;
- a live-parent stack starts at the parent's remote branch;
- a done parent starts at integration, while a branchless stacked-merged parent resolves through
  its own base, using actual `domain.ResolveEffectiveBase` outputs;
- first prepare creates one local branch and one attached worktree at the exact fetched commit;
- repeated prepare preserves commits, index, dirty bytes, untracked files, branch, and HEAD;
- interruption after manifest publication, branch creation, and worktree registration converges
  on retry without a second branch/worktree;
- pre-existing path, local branch, remote branch, foreign manifest, wrong registration, symlinked
  parent, and malformed manifest all block without byte changes;
- concurrent prepare/cleanup of the same workspace serializes while different workspace IDs do not;
- clean cleanup removes only the exact registration and leaves the branch; dirty/conflicted or
  moved state blocks, including a write raced after the preliminary status check;
- no path calls global prune, force removal, checkout/reset/clean, branch deletion, or touches the
  primary, `.docket/`, transaction, or sibling feature index; and
- branch publication creates an absent remote ref, fast-forwards an ancestor, adopts a
  response-lost success, rejects divergence, and reports unknown when the remote cannot be probed.

The tests record branch/HEAD/index/dirty/untracked bytes in every uninvolved worktree before the
operation and compare them afterward. They also inspect fake/real Git call logs so a test that
silently takes a permissive fallback cannot pass as coverage of the hard branch.

### Protocol-faithful `gh` tests

All default-suite GitHub behavior runs through an executable fake that receives the production
argv, cwd, environment, stdin, and cancellation path. The fake emits the exact nested JSON fields
real `gh` commands document and records invocation side effects separately from stdout, so a test
can prove whether a create/edit call happened.

Cover at least:

- no PR -> one create -> exact verified result;
- create succeeds but its response is lost -> requery adopts one matching PR and never creates a
  second;
- one exact open PR -> unchanged/adopted with no mutation;
- one mismatched open PR -> no expected version blocks; matching version edits once and verifies;
- a concurrent version change before edit returns contended and preserves the current body;
- an edit response lost after success is recovered by exact requery;
- wrong GitHub head OID, draft PR, terminal same-head PR, multiple open PRs, malformed JSON, missing
  fields, unexpected enums, auth failure, timeout, and cancellation;
- explicit `--repo` on every post-discovery invocation, including calls issued from a feature
  worktree whose CWD would infer differently;
- body bytes travel only on stdin and never appear in argv or diagnostics; and
- sensitive-looking token/URL/stderr fixtures are bounded and redacted in returned failures.

The fake executable itself has pass-through and invocation-witness tests so deleting its dispatch
arm reddens the target assertions. A flattened JSON mock or catch-all exit zero is forbidden.

### Live GitHub boundary

Change 0313 does not create, edit, or close a real PR as part of the ordinary suite or grooming.
The approved architecture assigns a smaller live acceptance layer to the release program, and
0317 owns direct four-harness/release acceptance. This change leaves a deterministic test seam and
documents the exact opt-in smoke request 0317 will run against a disposable repository; it does not
claim fake JSON is proof of current vendor behavior.

### Whole-suite gate

Implementation uses TDD and runs the repository's whole resolved suite, not only Go package tests.
The build records any wall-clock `OVER BUDGET:` finding and adds measured per-file budget rows for
new integration scripts/tests rather than hiding adapter latency in one oversized test.

## Out of scope

The boundary is intentionally strict.

- Configuration loading/capability classification (0305), loss-preserving general documents
  (0306), domain/snapshot/graph/effective-base policy (0307), Git discovery/object reads already
  delivered by 0308, metadata transaction semantics delivered by 0309, status/health/context
  composition (0310), installation/harness rendering (0311), and planning mutations/renderers/ADRs
  (0312) are consumed where necessary but not reimplemented or widened.
- No CLI workflow operation, candidate selection, claim, lease refresh, reconcile, plan/results
  attachment, agent dispatch, build/review routing, run verification, PR association, metadata
  field write, board refresh, or transition to `implemented` (0315).
- No process session/group, durable gate run, exact exit-versus-signal record, observe/stop, or local
  gate policy (0314).
- No rebase, force-push, PR checks/comment/review/merge/close, stack-child retargeting, merge-time
  evidence exemption, archive, reclaim, halted/finalize-blocked state, branch deletion, terminal
  cleanup policy, multi-workspace sweep, or interrupted close-out recovery (0316).
- No release artifacts, downloader, live four-harness acceptance (0317), self-hosting, config
  contraction, Bash deletion, or public cutover (0318).
- No GitHub Issues/Projects mirror, draft workflow, CI gate, daemon, database, global lock, arbitrary
  subprocess API, shell command string, Git LFS/submodule materialization, repository migration,
  branch-name guess, generic checkout/reset/clean, or automatic adoption of pre-existing in-flight
  Bash work.

The fixed upstream program decisions remain fixed. This change adds no new repository format and
does not normalize any `v0.9.2` persistent document.

## Acceptance boundary

Change 0313 is complete when the resolved whole suite proves all of the following as one
independently reviewable slice:

1. A caller can hand the service a landed `domain.EffectiveBase`, prepare or resume exactly one
   manifest-owned `.worktrees/<slug>` checkout at that remote base, inspect it without mutation,
   and remove it only when clean and exactly proven owned.
2. Publishing a ready workspace makes the exact committed HEAD reachable at the exact remote
   feature ref without force; replay after response loss adopts the remote postcondition and remote
   divergence is never overwritten.
3. Against a protocol-faithful fake `gh`, one exact ready PR is created, adopted, or explicitly
   versioned-updated by repository/head/base identity; every ambiguous response is followed by an
   authoritative probe and no retry creates a duplicate.
4. Build evidence round-trips through a marker-balanced PR body, preserves every unowned byte, and
   verifies only a green record at the exact full branch HEAD.
5. Primary, metadata, transaction, sibling feature, foreign, dirty, and human-authored state remain
   byte-identical unless the operation explicitly owns that exact workspace/ref/PR field/block.

No behavior owned by changes 0305–0312 or 0314–0318 is required for that proof, and no plan or code
implementation is part of this grooming change.
