<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0308 — Git adapter and authoritative object source](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0308-git-adapter-and-authoritative-object-source.md)**
<!-- docket:backlink:end -->

# Git adapter and authoritative object source

**Change:** 0308 · **Type:** feat · **Priority:** critical · **Date:** 2026-08-13 · **Status:**
Approved design

## Purpose and boundary

This change gives Go one typed boundary for discovering a Docket repository, refreshing remote Git
state, resolving exact revisions, and reading immutable tree/blob objects without altering the
user's checkout. It is the authoritative byte-and-identity source later repository operations use:
every decision input can be tied to one commit, and every returned file carries its Git blob ID as
an entity version.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) are fixed upstream constraints.
This spec resolves only change 0308's adapter interfaces, execution environment, ref/object read
semantics, typed failures, and real-Git fixture strategy. It does not reopen the program's hard
cutover, compatibility, two-mode repository, agent-first, or transaction decisions, and it does not
implement behavior assigned to changes 0305–0307 or 0309–0318.

## Landed foundation and independently deliverable result

Change 0304 is complete. The repository now has one Go module, `internal/app`'s protocol-v1 result
taxonomy, standard Go test/build gates, and the package-local versus frozen-repository fixture
convention. Change 0308 depends only on that foundation. Although changes 0305 and 0306 have also
landed by design time, this adapter does not import their configuration or document packages; it
remains independently understandable and reusable as the program map requires. Change 0307's
domain/read-model work has since landed (`internal/domain`, `internal/repository`; reconcile
2026-08-15) and is likewise not an input.

The independently reviewable deliverable is:

- `internal/gitcli`, containing typed repository discovery, controlled Git execution, remote
  default/ref inspection, targeted fetch, local ref resolution, and immutable object reads;
- an exact-revision `ObjectSource` boundary with NUL-safe tree listing plus efficient single and
  multi-blob reads returning exact bytes, Git modes/types, and opaque object IDs;
- stable typed failures with safe bounded diagnostics, cancellation, and local/network timeouts;
- real temporary-repository tests for both metadata topologies and every checkout-preservation
  guarantee; and
- no new CLI operation, worktree, commit, push, Docket parser, or domain behavior.

Change 0309 later consumes revisions and blob identities while adding isolated metadata
transactions. Change 0310 composes config, document parsing, the domain snapshot builder, and this
source into status/health queries. Change 0313 extends Git mechanics with feature workspaces and
GitHub pull-request behavior. Those consumers may depend on this change; their behavior does not
move into it.

## Chosen architecture

Use a layered adapter with an immutable, revision-bound read surface:

```text
invocation path
      |
      v
Discover --------------------------> Repository identity
                                         |
                     +-------------------+-------------------+
                     |                                       |
                     v                                       v
          RemoteDefaultBranch / ResolveRef             FetchBranch
                                                             |
                                                             v
                                                       exact Revision
                                                             |
                                                             v
                                                    OpenObjectSource
                                                             |
                                       +---------------------+------------------+
                                       |                                        |
                                       v                                        v
                              ListTree(prefixes)                     ReadBlobs(paths)
```

The changing boundary ends at `FetchBranch`: a fetch updates Git's local object/ref state and
returns one exact commit. `ObjectSource` is then permanently pinned to that commit. It never follows
a symbolic or remote-tracking ref after construction, never refreshes itself, and never returns
objects from a different revision. A caller that needs freshness fetches again and opens a new
source.

This is option 1 from the interactive design, refined with option 3's batch efficiency. It rejects
two alternatives:

- A broad mutable `Repository` interface would combine refs, reads, future transactions, pushes,
  and feature workspaces. It makes ref drift a caller concern, gives read-only consumers mutation
  capability, and requires 0308 to predict behavior owned by 0309 and 0313.
- A single high-level batch query would be compact for 0310 but either absorb Docket's configured
  path/mode policy or deprive 0309 of reusable revision and entity-version primitives.

Git command execution is an implementation detail behind these typed operations. No exported
`Run(args ...)` or generic command interface lets inward packages construct arbitrary Git
invocations.

## Typed vocabulary and interfaces

The exact file split may follow Go package conventions, but planning must preserve these
responsibilities and value semantics.

### Client and repository identity

`Client` owns the resolved Git executable, sanitized base environment, timeout policy, and process
factory/test seam. Production construction discovers `git` once; tests can provide an explicit
executable and shortened timeouts.

Repository discovery has the conceptual shape:

```go
type DiscoverOptions struct {
    InvocationPath string
}

type Repository struct {
    PrimaryWorktree string
    CommonDir       string
}

func (c *Client) Discover(ctx context.Context, opts DiscoverOptions) (Repository, error)
```

The real fields remain read-only values or accessors, not caller-mutable adapter state. A discovered
repository is identity plus typed operations; it does not carry a mutable "current revision" and
does not expose an index or checked-out branch as an authoritative read source.

`InvocationPath` may identify the primary checkout, a linked `.docket/` worktree, a feature
worktree, or a directory below any of them. Discovery resolves the main worktree for that Git
common directory, following ADR-0034 rather than treating `git rev-parse --show-toplevel` from a
linked worktree as the repository root. All identity paths are absolute, cleaned, and canonicalized
through every symlink hop before comparison. The operation rejects a missing path, a path outside a
Git repository, a bare repository, an unregistered worktree, and internally inconsistent Git
identity output.

Discovery is read-only. It may inspect worktree/common-directory metadata but does not create a
worktree, change `HEAD`, repair refs, write configuration, or touch the index.

### Refs, remote heads, and revisions

Use opaque, validated value types rather than unstructured strings:

```go
type ObjectID string
type RemoteName string
type RefName string

type Revision struct {
    Commit ObjectID
    Remote RemoteName
    Ref    RefName
}
```

`ObjectID` is normalized Git output and compared exactly. It is not truncated and does not assume
the 40-character SHA-1 shape; SHA-1 and SHA-256 repositories remain representable without changing
the API. `RefName` is fully qualified (`refs/heads/...`, `refs/remotes/...`) at adapter boundaries.
Remote names and refs are shape-validated before entering argument arrays; callers cannot smuggle
options or pathspec magic through them.

The repository exposes three read/refresh operations:

1. `RemoteDefaultBranch(ctx, remote)` queries the configured remote's authoritative `HEAD` symref
   and returns a fully qualified `refs/heads/...` name. It does not run `remote set-head`, mutate the
   cached `refs/remotes/<remote>/HEAD`, or fall back to a guessed branch when the remote cannot
   answer.
2. `FetchBranch(ctx, remote, refs/heads/<branch>)` performs one targeted, no-tags,
   no-submodule-recursion fetch into the corresponding remote-tracking ref, resolves the resulting
   commit, and returns a `Revision`. It does not prune other refs or fetch unrelated branches.
3. `ResolveRef(ctx, ref)` resolves an already-local fully qualified ref to an object ID with
   explicit found/not-found semantics. It never reads `FETCH_HEAD` as authority; that file is
   shared mutable process residue and cannot pin a caller's decision input.

Fetch necessarily writes Git's object database and the targeted remote-tracking ref. That is the
only mutation in this change. It does not write tracked files, the invocation worktree, its `HEAD`,
its index, its current branch, repository config, tags, unrelated refs, or submodules. Once fetch
returns, every later source lookup uses `Revision.Commit`, not the tracking ref that produced it.

An unavailable remote default, remote, source branch, or resolved commit is a typed failure. No
cached ref is silently accepted after a network operation meant to establish freshness fails.

### Immutable object source

Opening a source verifies that the supplied object exists locally and is a commit, then fixes it for
the source's lifetime:

```go
type ObjectSource interface {
    Revision() Revision
    ListTree(ctx context.Context, prefixes []RepoPath) ([]TreeEntry, error)
    ReadBlobs(ctx context.Context, paths []RepoPath) ([]BlobResult, error)
}

type TreeEntry struct {
    Path     RepoPath
    Mode     FileMode
    Type     ObjectType
    ObjectID ObjectID
}

type BlobResult struct {
    Path  RepoPath
    Found bool
    Blob  Blob
}

type Blob struct {
    Mode     FileMode
    ObjectID ObjectID
    Bytes    []byte
}
```

These names fix the responsibility, not a requirement to export every concrete representation.
The consumer-facing contract stays narrow even if the Go implementation returns a concrete source
that satisfies it.

`RepoPath` is an opaque repository-relative Git path stored in a Go string as bytes. The adapter
does not impose Docket's future UTF-8/document policy, but caller-supplied paths must be canonical:
no NUL, absolute form, empty component, `.` component, or `..` traversal. The empty prefix is
allowed only to list the root tree. Literal pathspec construction prevents leading punctuation or
Git pathspec magic from changing meaning.

`ListTree` recursively lists leaf entries below one or more literal prefixes: ordinary/executable
files and symlinks are Git blobs, while gitlinks are commit entries. Intermediate tree objects are
implicit and are not returned; an absent prefix returns no entries. Results sort by raw path bytes
and de-duplicate entries selected through overlapping prefixes. Each leaf's mode, Git object type,
and object ID are returned without following symlinks or entering submodules.

`ReadBlobs` handles one or many exact paths in one batch and returns results in request order. It
rejects duplicate request paths so one result has one unambiguous input identity. A missing path is
`Found: false`, not a Git-process failure: an absent `.docket.yml`, optional artifact, or optional
directory is legitimate input for later configuration/repository policy. A path that exists as a
tree or gitlink rather than a blob is an `unexpected-object` failure. A symlink is a blob; its bytes
are the stored link target and are never resolved through the filesystem.

Every returned byte slice is owned by the result. A caller cannot mutate the source or another
result through shared buffers. No source method consults the working tree or index, and no global
cache makes one repository's objects visible through another repository identity.

### Batch plumbing and path safety

Tree enumeration uses Git's NUL-delimited plumbing form and parses it directly from the child
process stream. It never requests display-form paths, never relies on `core.quotePath`, and never
round-trips NUL-delimited output through a text value that could lose delimiters. Tests cover spaces,
non-ASCII bytes, tabs, and embedded newlines in Git paths.

Blob reads first resolve paths to typed tree entries at the pinned commit and then send object IDs
to one `git cat-file --batch`-style process. The parser consumes each declared size exactly, keeps
stdout as binary data, and verifies response order, object ID, type, and framing. It does not build
ambiguous `<revision>:<path>` tokens or launch one process per blob. Empty input returns an empty
result without starting Git.

Malformed, truncated, reordered, or type-inconsistent plumbing output fails the entire call. The
adapter never returns a partial snapshot-shaped result that a later consumer could mistake for
complete authoritative state.

## Controlled command execution

All commands use an explicit executable, argument vector, working directory, environment, stdin,
stdout, and stderr. Docket never constructs a shell command string and never invokes a shell to run
Git.

### Executable and directory policy

- Production construction resolves `git` through the process search path once and records an
  absolute executable path. A missing or unusable executable fails before repository discovery.
- After discovery, repository commands run with `git -C` anchored at the canonical primary
  worktree. Git still reaches the shared common object database from there; callers cannot override
  the working directory per command or redirect an operation back to the invocation worktree.
- The adapter passes `--` or literal-pathspec forms wherever a Git command accepts caller-derived
  values. Values are shape-validated even when argument-array execution already removes shell
  injection.
- No operation changes the Go process's current directory or process-global environment.

### Environment policy

Start from the inherited environment so normal Git configuration, `HOME`/XDG config, certificate
roots, proxies, SSH agents, credential helpers, and transport settings keep working. Before every
Git process, remove environment controls whose semantic purpose is to redirect repository
discovery, the Git directory/common directory, worktree, index, object database, namespace, config
injection, or tracing. This includes the `GIT_DIR`, `GIT_COMMON_DIR`, `GIT_WORK_TREE`,
`GIT_INDEX_FILE`, `GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`, `GIT_NAMESPACE`, and
`GIT_CONFIG_*` families by semantic class, not only the examples named here.

The adapter then adds stable locale settings and disables terminal or GUI credential prompting.
Non-interactive credential helpers may still satisfy authentication; a credential path that needs
human interaction fails promptly and safely. Tests pin both sides: a planted repository/config
redirection variable cannot affect the target, while a benign authentication/transport sentinel
survives for a helper to observe.

Environment values are never included wholesale in diagnostics. Trace variables are removed so a
caller's tracing preference cannot mix arbitrary process detail into bounded adapter diagnostics.

### Cancellation, timeouts, and output

Every operation accepts `context.Context`. A shorter caller deadline wins. Without one, production
defaults bound local plumbing at 30 seconds per command and network discovery/fetch at five minutes;
tests can override both with positive durations. Product callers cannot disable bounds with a zero
or negative duration.

On cancellation or timeout the entire Git process is terminated, pipes are drained/closed, and the
operation waits for process cleanup before returning. Change 0308 owns ordinary one-shot child
cleanup only; durable process groups, observation, signals, and recovery belong to change 0314.

Stdout and stderr are captured separately. Machine-readable stdout is parsed only by the operation
that requested it. Stderr is retained only as a bounded, explicitly truncated diagnostic excerpt.
Blob/tree bytes, environment values, credentials, configured remote URLs, and arbitrary config
contents never enter an error message.

## Failure model

Adapter failures are structured independently of `internal/app`'s protocol results:

```go
type Failure struct {
    Operation Operation
    Kind      FailureKind
    ExitCode  int
    Detail    string
    Err       error
}
```

The stable kinds are:

| Kind | Meaning |
|---|---|
| `invalid-request` | Caller supplied a malformed path, remote name, ref, revision, or options value. |
| `executable-unavailable` | Git could not be discovered or started. |
| `invalid-repository` | Invocation path is not a usable non-bare worktree repository or its identity is inconsistent. |
| `remote-unavailable` | The named configured remote is absent or authoritative remote inspection cannot complete. |
| `ref-unavailable` | A requested remote or local ref is authoritatively absent. |
| `command-failed` | Git reported an external failure not safely classifiable by structured behavior, including authentication/network failures. |
| `unexpected-object` | A requested commit/blob has the wrong Git object type. |
| `invalid-output` | Git plumbing output is malformed, truncated, or internally inconsistent. |
| `cancelled` | The caller cancelled the operation. |
| `timed-out` | The adapter's or caller's deadline expired. |

Classification uses argument validation, documented command status, and explicit probes. It never
matches human stderr phrases, which vary by Git version, locale, transport, and credential helper.
When reliable refinement is impossible the kind is `command-failed`, not a guessed authentication
or network subtype.

Path absence from a successfully read tree is represented by `BlobResult.Found == false` and is not
in this failure vocabulary. Conversely, inability to resolve the commit/tree needed to answer that
question is a failure rather than a confident "not found."

`Detail` is safe explanatory prose, not a decision input. It may name the typed operation, canonical
repository path, remote *name*, validated ref, exit status, and bounded stderr excerpt. It never
prints the full inherited environment, a remote URL, credential material, stdin, object bytes, or
unbounded tool output. Later application operations own mapping to protocol results such as
`invalid-input`, `external-failed`, `interrupted`, or `internal-error`.

## Metadata topology support without mode policy

The adapter does not define a `MetadataMode`, parse `.docket.yml`, choose branches, or know the
configured changes/ADR/results directories. Both supported topologies emerge from the same
primitives:

- **Main mode:** a caller discovers the remote default branch, fetches that branch, opens one source,
  and may find both `.docket.yml` and live planning records at that revision.
- **Docket mode:** a caller first fetches/reads the remote default branch to obtain configuration,
  then explicitly fetches `refs/heads/docket` and opens a second source for live planning records.

The second sequence is composition owned by change 0310. Change 0308 proves the primitives against
that topology but does not import config, apply defaults, enforce the bootstrap matrix, assemble a
domain snapshot, or create the Bash-era persistent `.docket/` worktree. ADR-0001 remains the stored
topology/compatibility context; the approved Go architecture's authoritative object reads replace
shared-worktree reads for Go operations.

## Testing strategy

### Real repository harness

Primary behavior tests create a temporary non-bare invocation repository, a local bare `origin`,
and a separate writer clone. Setup uses the discovered real Git executable with explicit test
identity configuration; product calls use the adapter itself. No live network, user repository,
global Git configuration, Bash runtime, or frozen `v0.9.2` fixture is required.

Two builders create the exact topology under test:

1. Main mode: default/integration/metadata content on one remote branch.
2. Docket mode: `.docket.yml` on the default branch and planning content on a separate orphan
   `docket` branch, with primary, `.docket/`, and feature linked worktrees registered where the
   scenario needs them.

The tests supply branch names and paths directly. They do not parse config or documents and do not
assert domain meanings.

### Required proof matrix

**Repository identity**

- Invocation from the primary worktree, a nested directory, the linked `.docket/` worktree, a
  feature worktree, and symlinked spellings resolves to one canonical primary worktree and common
  directory.
- Missing paths, non-repositories, bare repositories, and inconsistent fake discovery output return
  the intended typed failure.

**Checkout preservation**

- Before fetch/read, record the invocation worktree's symbolic/detached `HEAD`, commit, current
  branch, index tree, tracked modifications, untracked entries, and representative bytes.
- Advance the bare remote from the writer clone, then fetch and read through the adapter.
- Assert that fetched remote objects and the targeted tracking ref are updated while every recorded
  checkout property is byte-identical. Include a dirty tracked file and an untracked file so a
  hidden reset/checkout cannot pass on a clean fixture.
- Run the proof from primary, metadata, and feature linked worktrees in both topologies.

**Revision consistency**

- Fetch commit A and open source A; advance the remote to B; prove source A continues to return A's
  tree and blob bytes.
- Fetch again and open source B; prove it returns B.
- Exercise an unrelated remote branch update and prove it neither changes source identity nor
  contaminates returned data.

**Objects, paths, and batching**

- Compare commit/blob IDs, modes, types, and bytes with independent real-Git plumbing.
- Read multiple blobs in one adapter call and prove the production path uses one batch process, not
  one process per blob.
- Cover empty files, missing paths, overlapping prefixes, symlinks, gitlinks, spaces, non-ASCII
  path bytes, tabs, embedded newlines, and a `core.quotePath=true` fixture.
- Prove symlink bytes are returned without following the filesystem target; a gitlink requested as a
  blob returns `unexpected-object`.
- Inject truncated, reordered, wrong-type, wrong-size, and extra plumbing frames; each must fail
  without returning partial results.

**Environment and failures**

- Use the Go test binary's helper-process mode (or a tiny Go-built helper), never a shell script, to
  record argv/environment, emit controlled stdout/stderr, block until cancellation, and return
  selected statuses.
- Prove each redirection/config/tracing environment class is removed and at least one benign
  auth/transport sentinel remains.
- Prove executable missing, remote/ref missing, generic command failure, cancellation, timeout, and
  malformed output produce their exact stable kinds.
- Plant a secret in environment, remote URL, stderr overflow, and blob input; verify no returned
  diagnostic exposes it and truncation is explicit.

### Suite integration and mutation discipline

All new tests are ordinary Go tests under `internal/gitcli`; change 0304's existing auto-discovered
Go suite producer already runs them through the authoritative `scripts/run-tests.sh` gate. Do not
add a second shell producer or duplicate `finalize.test_command` merely for this package.

Mutation checks at build time must at least:

- replace the pinned commit in one source call with its moving remote-tracking ref and observe the
  A/B consistency test fail;
- remove one repository-redirection environment scrub and observe the environment test target the
  planted wrong repository;
- replace the NUL-safe path parser with display-form parsing and observe a hostile path fixture
  fail; and
- introduce a checkout/reset or index read into the fetch/read path and observe the dirty-worktree
  preservation proof fail.

Run the complete resolved suite at the build gate and act on any `OVER BUDGET:` report. Tests may
use temporary repositories and local file remotes on Darwin and Linux; no test depends on timing a
live network or executing a foreign cross-compiled binary.

## Out of scope

- Configuration loading, precedence, default/integration/metadata branch selection, capability
  diagnostics, or mutation preflight (0305).
- YAML/Markdown parsing, source-preserving documents, managed blocks, or patching (0306).
- Domain entities, repository snapshots, validation, lifecycle, graphs, selection, or effective-base
  policy (0307).
- Docket-owned transaction worktrees, ownership locks, commits, explicit-path staging, expected-ref
  leases, pushes, semantic retries, request idempotency, cleanup, or recovery pruning (0309).
- Status/context assembly, health findings, human text, or protocol JSON operations (0310).
- Installer/assets/harness behavior (0311), planning operations/renderers (0312), feature
  workspaces/branches, `gh`, PRs, and build evidence (0313), process supervision (0314), workflow
  orchestration (0315–0316), release acceptance (0317), and self-hosting/cutover (0318).
- A persistent shared Go metadata checkout, repository migration, submodule traversal, partial/shallow
  clone repair, Git LFS materialization, signing, hooks, arbitrary Git command execution, a public Go
  library, or a user-facing CLI operation.

The adapter may observe and read valid shallow repositories when the requested commit/object is
available after its targeted fetch, but expanding or repairing arbitrary partial-clone filters is
not part of this change.

## Acceptance boundary

Change 0308 is complete when the resolved whole suite proves that `internal/gitcli` can discover the
same primary repository from every registered worktree, authoritatively fetch and pin an explicit
remote branch, and list/read exact revisioned bytes plus blob identities in both metadata
topologies—including hostile Git paths—while preserving the invocation checkout's branch, `HEAD`,
index, dirty files, untracked files, and bytes. Git execution must be direct, bounded,
non-interactive, environment-controlled, batch-efficient, and typed on failure.

No configuration/document/domain interpretation, metadata transaction, status output, worktree or
PR behavior, workflow, or implementation owned by changes 0305–0307 or 0309–0318 is required for
that proof.
