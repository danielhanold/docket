<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0303 — Go migration program record and Bash-backlog disposition](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0303-go-migration-program-record-and-bash-backlog-disposition.md)**
<!-- docket:backlink:end -->

# Docket Go migration architecture

**Date:** 2026-08-12

**Status:** Approved in interactive architecture design

**Bash baseline:** `v0.9.2`

## Purpose

Docket has outgrown Bash as an implementation language. Its durable product is no longer a small
collection of shell conveniences: it is a concurrent, Git-backed lifecycle engine with structured
documents, generated views, process supervision, four agent-harness integrations, recovery rules,
and a test suite dominated by defenses against shell and platform behavior.

This design defines the target architecture and release boundary for replacing Docket's production
Bash implementation with Go. It is a program architecture, not an implementation plan for one
large pull request. After approval it is decomposed into independently buildable Docket changes.

## Settled constraints

- The final Bash release is already tagged as `v0.9.2`. Bash feature development stops there.
- Existing users may remain on that tag. The Go release has no public Bash-to-Go transition mode.
- Go v1 supports macOS and Linux on both amd64 and arm64.
- Homebrew packaging is deferred. Initial releases use GitHub Release artifacts and a small POSIX
  downloader.
- Released installations use embedded-and-extracted assets. Contributors retain a source-linked
  development installation.
- Claude, Codex, Cursor, and OpenCode are all first-class direct host harnesses in Go v1.
- Cross-harness runner delegation is not part of Go v1.
- Git and `gh` remain external executables. Docket invokes them directly without a shell.
- Markdown and Git remain the authoritative persistent store. Go introduces no database.
- The product remains agent-first. The Go binary owns deterministic mechanics, not model judgment.

## Compatibility contract

> Any repository in a valid, quiescent state under the final Bash release can be opened and
> operated on directly by the Go release without a repository migration. Go preserves persistent
> data meanings and safety invariants, but makes no compatibility promise for Bash paths,
> configuration helpers, helper-command output, machine protocols, or in-flight processes.

Existing documents are not normalized simply because Go reads them. During the private
self-hosting period, Go also writes only repository shapes understood by `v0.9.2`, providing a
development fallback. Reverse compatibility is not a permanent public promise after the Go
cutover.

## Capability boundary

### Required in Go v1

- Release installation and source-linked development installation.
- Claude, Codex, Cursor, and OpenCode as direct host harnesses.
- Shipped model/effort defaults plus global, per-machine overrides.
- Existing `docket`-mode and `main`-mode repositories.
- Fresh-repository bootstrap and the existing human-initiated `main`-mode to `docket`-mode
  migration.
- Existing committed, global, and repository-local configuration layers for retained behavior.
- Change manifests, lifecycle meanings, custom change types, dependencies, stacks, selection, and
  readiness.
- Interactive change capture and grooming.
- Read-only status, health checks, inline `BOARD.md`, and an explicitly mutating maintenance sweep.
- Claim, reconcile, feature worktrees, planning, build, review, results, local gates, PR publication,
  build evidence, and run verification.
- ADR creation, supersession, reversal, indexing, and relationships.
- Finalize, local rebase/retest, merge, archive, stack close-out, reclaim, recovery, and cleanup.
- Consumption of existing learning files and an explicit manual learning-record operation during
  the migration.

### Deferred from Go v1

- Automated learning harvest, learning-index maintenance, capacity checks, and promotion workflow.
- Autonomous grooming and its critic gate.
- Automatic capture of discovered follow-up work.
- Dummy mode.
- Terminal publishing of metadata onto the integration branch.
- CI-only and combined local/CI finalize gates.
- Results-only local-gate skipping.
- Cross-harness runner delegation.
- Per-repository model/effort routing.
- Skill rebinding and per-role `auto` substitution.
- Optional build checkpoint ledgers.

Deferred persistent data is preserved. A deferred behavior that is actively requested by config
fails before mutation with an actionable result; it never silently degrades. The learnings boundary
is the deliberate exception already approved: `learnings.enabled` continues to gate retained
consumption and explicit manual records, while Go reports that automatic harvest/index/promotion is
unavailable rather than treating the entire retained learning path as unsupported.

### Dropped or retired

- The GitHub Issues/Projects backlog mirror is dropped, not deferred. `issue:` fields remain inert
  historical data; `gh` remains in use for pull requests.
- `github_project` is historical/inert configuration. A requested `github` board surface is an
  unsupported mutation configuration.
- Bash, Python, and Perl runtime discovery are retired. Change 0285's required capability becomes
  the native Go process supervisor rather than a Python implementation.
- Bash portability, source-hygiene, profiling, backfill, and runtime-routing machinery do not carry
  into the Go product.

## Product boundary: agent first

Humans normally invoke Docket through skills and named agents. Skills own:

- Dialogue and explanation.
- Design and reconciliation judgment.
- Authored specs, plans, results, findings, and summaries.
- Review interpretation and decisions requiring human context.
- Harness-native dispatch of build, review, conflict-resolution, and repair agents.

Go owns:

- Configuration and repository discovery.
- Authoritative reads and context assembly.
- Domain validation, selection, and legal transitions.
- All durable metadata mutations and derived views.
- Git, GitHub PR mechanics, worktrees, and process supervision.
- Installation, assets, and harness-specific rendering.

The CLI remains directly useful for diagnostics, status, recovery, installation, and development,
but Go v1 does not build a second interactive workflow competing with the skills. The binary never
becomes a cross-harness agent dispatcher.

## Architecture shape

Go v1 ships as one module and one `docket` executable. Strong package boundaries do not imply
separate artifacts or public APIs.

```text
cmd/docket/
internal/
  app/          application operations and result taxonomy
  domain/       lifecycle, readiness, graphs, selection, policies
  config/       YAML layers, defaults, fences, capability classification
  document/     loss-preserving document reads and patches
  repository/   snapshots, validation, isolated transactions
  render/       board, artifact links, backlinks, ADR index
  gitcli/       typed external Git adapter
  githubcli/    typed external gh adapter
  process/      durable local-process supervision
  install/      embedded assets, ownership, atomic installation
  harness/
    claude/
    codex/
    cursor/
    opencode/
```

### Dependency direction

- Domain code imports no filesystem, Git, Markdown, CLI, process, or harness package.
- Application operations coordinate domain policy through interfaces implemented by adapters.
- Adapters depend inward; domain and application policy do not depend outward on an adapter.
- Harness implementations produce installation plans. The common installer performs mutations.
- Most packages remain under `internal`; Go v1 publishes no supported Go library API.

The stable external contracts are the repository format and versioned JSON protocol. Runtime Go
plugins are rejected: they add distribution, toolchain, dependency, initialization, security, and
test compatibility obligations while Docket has no third-party extension requirement. Separately
versioned Go libraries are likewise deferred until a real second consumer needs to embed Docket.
All four known harness implementations compile into the binary behind an internal interface.

## Skill-to-engine protocol

Skills invoke one-shot CLI processes. There is no daemon or socket service in Go v1.

- Small inputs use flags.
- Authored Markdown uses stdin or an explicit request file, never shell interpolation.
- `--json` writes exactly one versioned JSON document on stdout.
- Logs and unexpected diagnostics go to stderr.
- Human-facing commands default to readable text and offer `--json`.
- Handled outcomes carry a typed result kind; exit codes provide only coarse CLI categories.
- JSON consumers decide from the result kind, not from parsing prose or re-tokenizing a line.

Representative response:

```json
{
  "protocol_version": 1,
  "operation": "change.claim",
  "result": "contended",
  "change_id": 285,
  "observed_revision": "ca8251e8",
  "reason": "the target entity changed after it was read"
}
```

Representative result kinds are `applied`, `no-op`, `contended`, `invalid-input`, `invalid-state`,
`blocked`, `unsupported-config`, `gate-failed`, `external-failed`, `interrupted`, and
`internal-error`.

A daemon could cache repository state, serialize operations, stream events, and supervise children,
but would add service discovery, permissions, lifecycle, version skew, cache invalidation, upgrade,
and multi-repository routing. Fresh remote state is required for mutations regardless, Go startup is
cheap, isolated transactions solve write contention, and long-running work has a narrower
supervisor. A daemon is reconsidered only in response to measured latency, live-UI, or supervision
needs.

## Read model and context bundles

There is no persistent shared `.docket/` worktree in the Go architecture. Skills read metadata
through Go queries rather than invoking `git show` or reading a shared checkout.

A context query:

1. Fetches the authoritative remote.
2. Reads Git objects without altering the user's checkout.
3. Parses a complete immutable repository snapshot.
4. Returns the repository revision, typed state, authored Markdown, relevant artifacts and
   relationships, and an entity version for each decision input.

The entity version is normally the Git blob identity of the relevant change file. A skill that
bases judgment on that content submits the version with its mutation. Go can then distinguish an
unrelated concurrent repository update from a change to the exact decision input.

Queries include status, context assembly, change/ADR inspection, deterministic selection, and run
verification. `docket status` is read-only. The status agent may explicitly invoke a mutating
maintenance sweep before reading status, but a human status command never writes unexpectedly.

## Metadata transaction model

Every durable metadata write passes through a coarse application operation. Skills never edit,
stage, commit, or push the metadata branch directly.

For each mutation Go:

1. Fetches the authoritative remote ref.
2. Creates a Docket-owned isolated transaction worktree at that exact commit.
3. Loads and validates the complete repository snapshot.
4. Checks operation preconditions and submitted entity versions.
5. Applies one legal domain operation.
6. Patches owned document fields and blocks.
7. Regenerates every affected derived view.
8. Validates the complete resulting snapshot.
9. Commits the explicit affected paths as one metadata commit.
10. Pushes with an exact expected-ref lease.
11. Removes the transaction worktree.

If an unrelated writer wins the push race, Go reloads current state and reapplies an operation whose
preconditions still hold. If the target entity changed incompatibly, the operation returns
`contended`; it never text-merges two authored decisions. A creation request carries a stable,
shape-validated request ID. The successful commit records that ID in a Docket-owned commit trailer;
a retry searches authoritative history and returns the original result rather than allocating a
second change or ADR. This avoids imposing a new global slug-uniqueness rule on Bash-era data.

One operation atomically includes its primary record and every affected v1-owned board, index,
artifact-link, and backlink output. A renderer or validation failure produces no commit, so a
v1-owned derived view cannot trail the state transition that required it. The deferred learnings
index is preserved but is not v1-owned; a manual learning write does not claim to refresh it.

The same transaction mechanism applies in both repository modes. Feature worktrees remain separate:
build agents edit code, plans, and results there, while Go owns creation, validation, and lifecycle.

## Domain model

The immutable repository snapshot contains changes, lifecycle state, dependencies, stacks, related
and discovered work, specs, plans, results, ADRs, existing learnings, and derived views. Domain
validation covers at least:

- Unique IDs, slugs, and paths.
- Status and directory placement.
- Legal transitions and required fields.
- Reference validity, dependency cycles, stack cycles, and effective bases.
- Readiness and deterministic selection.
- Claim leases, contention, reclaim, and halted states.
- ADR identity, relationships, and immutable accepted content.
- Archive dates and terminal meanings.

Derived files are output, never authority. Repository-wide validation runs before a mutating commit,
not only against the file being changed.

## Loss-preserving documents

Each existing document is represented as both typed semantics and original source bytes with the
locations of known fields and managed blocks.

- A real YAML decoder validates semantic frontmatter.
- A source-preserving layer patches only fields owned by the operation.
- Unknown fields, ordering, comments, quoting, blank lines, line endings, and authored Markdown
  remain byte-identical.
- Unknown body sections and historical fields are preserved.
- Managed blocks are rewritten only after marker order and balance are validated.
- Existing records are never normalized as a side effect of being read.
- Newly created documents use one canonical Go-owned format.
- Go does not silently repair malformed records.

Merged plans/results, archived history, and accepted ADR content remain frozen according to the
existing convention. The only later writes are already-owned generated blocks or status changes
whose existing contracts explicitly permit them.

No mandatory repository schema version is inserted for v1. Protocol versioning and repository
format evolution remain independent.

## Operation surface

Commands align with domain intent rather than existing shell-helper filenames. Final spelling is an
implementation-plan concern, but the operation families are:

```text
Read-only
  status
  context
  change inspect/select
  adr inspect
  run verify

Metadata transactions
  change create
  change groom
  change claim
  change reconcile
  change block/defer/kill
  change attach-plan
  change attach-results
  change mark-implemented
  change reclaim
  learning record/update
  adr record/supersede/reverse
  finalize closeout
  maintenance sweep

Feature-branch and external effects
  workspace prepare/inspect/cleanup
  gate launch/observe/stop
  pr publish
  finalize prepare/merge

Machine installation
  install
  install check
  development install

Repository setup
  repository init
  repository migrate
  repository check
```

These are not generic frontmatter setters. For example, `change groom` attaches the authored spec,
checks the submitted entity version, transitions the change to build-ready, renders backlinks and
artifacts, validates the repository, updates the board, commits, and pushes as one operation.

There is no hidden central workflow session. Durable repository and external state are sufficient
to resume: claims and leases, feature branches and worktrees, plans and results, PR and merge state,
build evidence, halted/finalize-blocked markers, and terminal archive records. A later invocation
inspects that state and continues, reports contention, or requires a human.

`learning record/update` is deliberately manual: a human or agent identifies and authors the
lesson, and Go performs the loss-preserving metadata transaction. It does not harvest signals,
render the deferred index, enforce capacity, or promote a rule. Promotion into `AGENTS.md` remains
human-gated and outside the automated subsystem.

## External commands and resumable effects

Git and `gh` are invoked with explicit argument arrays under controlled working directories and
environments. Their adapters own executable discovery, output capture, typed failure, cancellation,
timeouts, and safe diagnostics. No domain code constructs or parses shell command strings.

GitHub and cross-branch effects cannot be atomic with a metadata commit. They follow an idempotent
sequence:

1. Read and validate current authoritative state.
2. Probe whether the external effect already happened.
3. Perform it only when absent.
4. Verify it from the authoritative external source.
5. Record the verified result in one metadata transaction.
6. Make cleanup independently retryable.

PR publication searches by feature branch before creation. Finalize verifies merge SHA and GitHub's
merge timestamp rather than trusting a local clock. A crash after merge but before archive is
recovered by rerunning finalize or the explicit maintenance sweep. Cleanup happens after durable
close-out and can be retried. Docket never attempts automatic compensating rollback of a merge or
other irreversible external effect.

Cancellation before a metadata push leaves no durable mutation. A response lost after a successful
push converges on retry through semantic idempotency and authoritative probes.

## Native process supervisor

The `docket` executable has an internal supervisor mode for long-running local gates. It is a
narrow, per-run subprocess rather than a machine daemon.

- The supervisor establishes a new Unix session/process group before executing the command.
- It redirects stdout and stderr to separate durable files and closes stdin.
- It waits on the child and records exact exit status versus signal termination.
- Terminal state is written atomically in a private, mode-0700 run directory.
- Launch, observe, and stop are separate invocations.
- An ownership token plus a live lock prevents signalling a reused PID.
- Run diagnostics outlive the process until an owned-state cleanup removes them.
- Darwin and Linux behavior sits behind platform-specific implementations and a shared contract.

This directly absorbs the capability sought by change 0285. No Python or Perl runtime is
discovered, and no `128+signal` heuristic is used.

Temporary worktrees and run directories carry Docket ownership manifests and live locks. Recovery
may prune abandoned owned state, but never a worktree or directory identified only by pathname.

## Configuration

Go uses a real YAML decoder. For retained behavior it preserves the existing precedence model:

```text
built-in < global machine config < committed repository config < repository-local machine config
```

Coordination-sensitive keys remain committed-only. Agent model/effort overrides are deliberately
narrower in v1: shipped defaults plus global, per-machine overrides only. Per-repository routing and
skill rebinding are unsupported.

Legacy keys are classified rather than handled ad hoc:

- **Supported:** validate and apply.
- **Obsolete:** warn and ignore, such as `runtime.bash`.
- **Historical/inert:** preserve without behavior, such as `github_project`.
- **Deferred but requested:** report `unsupported-config` before mutation.

Read-only diagnostics may inspect and report a repository carrying unsupported settings. A mutation
never begins its transaction while active unsupported behavior is requested.

`learnings.enabled` is classified as supported for reading and explicit manual record/update. When
true, diagnostics also report that the automated harvest, index, capacity, and promotion machinery
is deferred; this warning does not block the retained manual path.

## Installation and embedded assets

GitHub Releases publish binaries for:

```text
darwin-arm64
darwin-amd64
linux-arm64
linux-amd64
```

A small POSIX downloader detects the platform, downloads the requested release and checksum
manifest, verifies it, installs under a user-controlled executable directory, and runs
`docket install`. Homebrew is deferred.

The binary embeds skills and references, agent templates, harness defaults, dispatch instructions,
configuration schemas, document templates, and an asset/protocol manifest. `docket install`
extracts a versioned asset tree atomically and installs links or generated files into detected or
explicitly selected harnesses. An ownership manifest prevents overwriting or pruning unrelated user
files.

The common installer owns atomic writes, drift detection, rollback, and ownership. A harness
implementation only detects its native layout and returns an installation plan. Machine-level
wrappers use shipped defaults plus global model/effort overrides. Existing Docket-owned project
wrappers that would shadow the Go installation are replaced or removed only after their ownership
is proven; unrelated project files are untouched.

The source-linked development installation uses the same render and validation pipeline but:

- Builds the development binary from the checkout.
- Links discoverable skill directories to the source tree for immediate skill edits.
- Generates wrappers from source templates and current global configuration.
- Records source revision and asset-protocol version.
- Refuses startup when binary and source assets are incompatible.
- Is rerun after changes to templates, defaults, or wrapper generation.

Released users therefore get self-contained, version-matched assets while contributors retain an
edit-once workflow.

## Testing strategy

The Go suite preserves behavioral confidence without mechanically porting the Bash suite.

### Domain tests

Table-driven unit tests cover lifecycle, readiness, selection, dependencies, stacks, claims,
reclaim, and finalize eligibility without filesystem or subprocess dependencies.

### Document compatibility tests

Frozen `v0.9.2` repository fixtures cover loss-preserving patches, frontmatter shapes, managed
markers, unknown fields, comments, whitespace, line endings, and new-document golden files. Go
fuzzing targets frontmatter and marker boundaries.

### Transaction tests

Real temporary repositories and local bare remotes cover both repository modes, unrelated
concurrent mutations, same-entity contention, lost pushes, semantic retry, interruption, and owned
cleanup.

### Adapter and process tests

Real Git exercises worktree, ref, rebase, lease, and push behavior. Most `gh` behavior uses
controlled fakes with a smaller live acceptance suite. Real helper processes exercise normal exit,
signals, cancellation, group teardown, response loss, and orphan recovery on macOS and Linux.

### Harness and end-to-end tests

Golden installations cover all four native harness formats, ownership, drift, upgrades, and source
links. Pre-release acceptance invokes Docket directly through Claude, Codex, Cursor, and OpenCode.
End-to-end scenarios cover capture through groom, claim through PR, finalize through archive,
ADRs, stacks, and crash/retry boundaries.

### Compatibility comparison

For retained behavior, `v0.9.2` and Go run against independent copies of the same fixtures. The
comparison is semantic: legal transitions, selection, persistent meanings, dependency/stack
results, terminal outcomes, and safety posture. It is not blanket byte parity for generated output
or helper protocols. Existing bytes outside an intentional patch do require exact preservation.

Every guard has a negative test that violates its premise and proves the operation fails before
mutation. Bash-only runtime, quoting, grep/awk, pipeline, BSD-tool, source-shape, and shell-runner
tests are deleted when their production surface disappears.

## Migration and cutover

The implementation is incremental; the public release is a hard replacement.

1. **Freeze the baseline** — `v0.9.2` remains the final Bash tag; freeze compatibility repositories
   and retained outcomes. Use it to manage the migration backlog initially.
2. **Build foundations** — module/binary, config, documents, domain, Git, transactions, JSON,
   installation, repository setup/migration, and harness asset machinery.
3. **Read-only vertical slice** — open, validate, status, health, selection, and stack results on
   existing repositories.
4. **Planning-state workflows** — create, groom, lifecycle transitions, ADRs, inline board, and
   concurrent metadata writes.
5. **Implementation workflows** — claim, reconcile, worktrees, plans/results, local supervisor,
   PRs, evidence, and implemented transition.
6. **Finalize and recovery** — rebase/retest/merge, archive, stacks, reclaim, sweep, interruption,
   and cleanup in both repository modes.
7. **Four-harness dogfood** — native install and direct execution through Claude, Codex, Cursor,
   and OpenCode; Cursor is an explicit migration environment, not a documentation-only claim.
8. **Contract Docket's active config** — before self-hosting, explicitly disable the deferred
   features Docket currently enables: `auto_capture`, terminal publishing, build checkpoints, and
   results-only gate skipping. This is a reviewed repository-policy change, never an installer-side
   rewrite or silent fallback.
9. **Self-host and cut over** — Go manages Docket's own repository; manual learning capture bridges
   the deferred automation; production Bash and Bash-only tests leave active development; publish
   all four release targets.

Go is first exercised against disposable clones and fixtures. It does not mutate Docket's live
metadata branch until retained workflows and recovery gates pass.

### Cutover gates

- Existing valid repositories open without migration.
- The complete retained lifecycle works end to end.
- Concurrent writers share no mutable index or dirty metadata worktree.
- Process supervision distinguishes exit from signal on Darwin and Linux.
- All four harnesses pass installation and direct-invocation acceptance.
- All four release targets build and pass their applicable smoke tests.
- The installed product requires no Bash, Python, or Perl runtime.
- Git and `gh` failures are actionable and non-destructive.
- Compatibility and failure-injection suites are green.
- Deferred settings fail before mutation instead of silently degrading.
- Docket's own committed configuration lies inside the supported Go v1 capability envelope.

The rollback artifact is tag `v0.9.2`; the Go binary carries no Bash fallback.

## Program tracking and decomposition

This specification is the stable what-and-why record, not a live checklist. After human review it
is attached to a small documentation-only Docket change that records the architecture and program
decomposition. Implementation work is then minted as independently buildable, reviewable Docket
changes, linked back through `discovered_from`/`related` and ordered only by real `depends_on`
edges. Their ordinary change records and `BOARD.md` are the live progress source; no second status
checklist is maintained.

The first decomposition follows the migration phases above. Each phase may require more than one
change; no change spans multiple architectural components merely to preserve the phase numbering.

## Out of scope

- Implementing any migration phase in this architecture record.
- A daemon, local RPC service, or workflow server.
- Runtime Go plugins or a public Go library API.
- A public Bash/Go hybrid or compatibility shim.
- Homebrew packaging.
- Windows support.
- Reintroducing any capability listed as deferred or dropped.
- Redesigning the Docket document convention beyond what the loss-preserving Go engine requires.

## Success criterion

The migration succeeds when Docket can manage its own complete retained lifecycle through the Go
binary, directly from all four supported harnesses, while an existing `v0.9.2` repository requires
no format migration and no production Bash/Python/Perl runtime remains.
