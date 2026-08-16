<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0310 — Read-only status and health vertical slice](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-16-0310-read-only-status-and-health-vertical-slice.md)**
<!-- docket:backlink:end -->

# Read-only status and health vertical slice

**Change:** 0310
**Date:** 2026-08-15
**Status:** Approved design

## Purpose

Change 0310 delivers the first retained, user-visible read workflow in the Go executable: one
`docket status` command that opens an existing repository, constructs the authoritative docket
snapshot, and reports backlog state and health without changing docket metadata.

The approved Go migration program map and architecture design remain the governing constraints.
This document narrows those decisions to an independently deliverable vertical slice. It consumes
the foundations landed by changes 0305 through 0308, but does not redesign or reimplement them. It
does not take behavior from change 0309 or changes 0311 through 0318.

## Delivered boundary

The slice owns the application composition and presentation needed to turn existing read
capabilities into a complete status operation:

1. Discover the canonical repository and its authoritative Git revisions.
2. Resolve configuration for that repository and metadata mode.
3. Read and parse docket records from Git objects rather than checkout files.
4. Build the landed immutable domain snapshot and its validation report.
5. Evaluate readiness, dependencies, effective stack bases, and the ordered ready queue.
6. Validate the existence of linked spec, plan, and results artifacts on their authoritative
   branches.
7. Return one protocol-v1 operation result that both JSON and human presenters consume.

The command is observational. It may perform targeted fetches needed to obtain authoritative Git
objects and remote-tracking refs. It does not modify checkout bytes, the index, `HEAD`, checked-out
branches, docket records, generated boards, commits, pull requests, or remote metadata branches.

## Landed foundations

The implementation is a client of these already-landed boundaries:

- `internal/gitcli` discovers the primary worktree, resolves the remote default branch, fetches
  exact branches, opens revision-pinned object sources, and reads trees and blobs without changing
  a checkout.
- `internal/config` decodes layered sources, resolves the effective configuration, emits
  diagnostics, and classifies capabilities.
- `internal/document` parses loss-preserving Markdown documents and returns typed structural
  errors.
- `internal/repository` decodes records, builds an immutable snapshot, and returns deterministic
  validation findings.
- `internal/domain` owns readiness, dependency and stack semantics, effective-base resolution, and
  ready-queue selection.
- `internal/app` owns protocol-v1 envelopes, result kinds, and exit semantics.
- `internal/cli` owns Cobra wiring, the global `--json` transport, and presentation.

Change 0310 may add narrow application-facing read interfaces around those capabilities where
composition requires dependency injection. It must not broaden their policy or duplicate their
parsers, validators, graph walks, or Git process mechanics.

## Architecture

The command has one application operation and one result model:

```text
docket status
    -> status application operation
        -> Git discovery and revision-pinned object sources
        -> configuration resolution
        -> document parsing and repository snapshot construction
        -> domain readiness, stack, and selection queries
        -> artifact-reference checks
    -> StatusResult
        -> protocol-v1 JSON presenter
        -> human status presenter
```

The application operation owns orchestration, cancellation propagation, and translation from
landed domain values into a presentation DTO. The CLI owns argument validation and rendering only.
Both output modes receive the same `StatusResult`; the human path must not rerun repository reads or
derive different status semantics.

The command is registered as asset-independent because it reads repository state and does not
require an installed generated-asset tree.

## Authoritative read algorithm

### 1. Discover and pin repository context

Repository discovery starts from the invocation path and anchors on the primary worktree. The
operation resolves `origin`'s default branch, fetches the exact branches required for the active
metadata mode, and opens an object source pinned to each resolved revision.

The resolved context records:

- metadata mode;
- remote default branch and revision;
- configured integration branch and revision; and
- configured metadata branch and revision when docket mode is active.

Revision values are captured once per operation. Later reads use those pinned object sources, so a
concurrent remote update cannot mix versions within one result.

### 2. Resolve configuration

The repository `.docket.yml` source is read from the pinned remote-default object source, not the
working tree. It is combined with built-in defaults and the permitted global and repository-local
machine layers using the landed configuration resolver. Repository-local machine configuration is
located relative to the canonical primary worktree.

Configuration must resolve far enough to identify metadata mode, paths, and authoritative
branches. A malformed or unresolved configuration that prevents that topology is an operation
failure. Deferred mutation capabilities are still reportable diagnostics and do not block this
read-only command.

### 3. Read the metadata corpus

In main mode, docket records come from the pinned integration/default branch source selected by
the resolved configuration. In docket mode, records come from the separately pinned metadata
branch source. The operation reads configured active and archived change directories, ADRs, and
learnings when enabled.

Tree listing determines the corpus. Blob reads return exact bytes and blob identifiers. Blob
identifiers are retained as entity versions in the result rather than replaced with timestamps or
working-tree state.

Each document is parsed through `internal/document` and decoded through `internal/repository`.
One malformed record does not prevent inspection of the remaining repository: its typed parse or
decode problem becomes a health finding, while all usable records continue into snapshot
construction.

### 4. Build and query the snapshot

The operation builds one complete repository snapshot and preserves the builder's deterministic
validation report. It then uses landed domain queries to compute, for each active change:

- typed readiness and its reason;
- unmet dependency identifiers;
- declared stack parent and resolved effective base;
- record location and version; and
- inclusion in the ordered ready queue.

Where effective-base resolution needs branch facts, the operation fetches and resolves only the
distinct feature branches referenced by live stack relationships. This is a narrow status read;
it does not enumerate workspaces, inspect pull requests, manage branches, or create a generalized
feature-branch workflow.

The ready queue comes directly from the landed deterministic selector. The application layer must
not recreate priority ordering, dependency satisfaction, or stack-terminal rules.

### 5. Check linked artifacts

The operation validates artifact links against their authoritative sources:

- a change's spec is checked on the metadata source;
- plan and results links are checked on the integration source; and
- empty links remain distinct from links whose target is missing.

These are structural health checks only. The operation does not parse plans into executable work,
write backlinks, repair links, or render managed blocks.

## Command surface

Change 0310 adds:

```text
docket status [--repo-dir PATH] [--type TYPE] [--priority PRIORITY] [--json]
```

`--repo-dir` defaults to the invocation directory. `--type` and `--priority` are repeatable
backlog-projection filters and accept only configured closed values. The existing global `--json`
contract applies.

Filters affect the displayed active-change projection and the ready queue derived from that
projection. They do not reduce repository loading or health validation: health always describes
the complete authoritative corpus. This keeps a filtered report from hiding unrelated corruption.

No maintenance option is added to `docket status`. An agent that wants a refreshed board or a
maintenance sweep must invoke that mutating operation explicitly before status, as settled by the
program architecture.

## Result contract

`StatusResult` is an application DTO wrapped in the existing protocol-v1 envelope. Its JSON form
contains these semantic sections:

- `context`: mode, branch names, and exact authoritative revisions;
- `summary`: deterministic counts for the complete corpus and displayed active projection;
- `changes`: displayed active changes with identity, metadata, location, record version,
  readiness, unmet dependencies, and stack resolution;
- `ready`: the ordered identifiers selected from the displayed projection;
- `records`: entity identity, authoritative location, and blob version for every loaded record;
  and
- `findings`: normalized configuration, document, repository, artifact, and status-read findings.

Closed result values use named strings already owned by their domain types. Collections encode as
empty arrays rather than `null`. Ordering is stable: changes and records use repository-defined
identity order, ready identifiers preserve selector order, and findings preserve the landed report
ordering followed by deterministic status-read checks.

Each finding has stable machine fields for code, severity, entity kind and identity, field or path
when applicable, and related identities. Human detail and remedies are explanatory strings, not a
secondary parseable protocol. Any printed remedy must be valid for the exact reported state.

Host-specific absolute paths are omitted from protocol context. Human output may identify the
repository by a safe display path, but consumers use branch names, revisions, record locations,
and entity versions as stable context.

## Human report

The human presenter provides the same semantics in a compact, deterministic report:

1. repository mode and short authoritative revisions;
2. complete and displayed counts;
3. the ordered ready queue;
4. one row per displayed active change with readiness, unmet dependencies, and effective base;
5. health totals followed by ordered error and warning findings.

An empty ready queue, no matching filtered changes, and a healthy repository are explicit states,
not missing sections. The report is terminal output only. It is not a board surface and is not
required to preserve Bash line layout or incidental wording.

## Health and failure semantics

Health is repository data, not command failure. Parseable repository defects, validation errors,
missing linked artifacts, unresolved dependencies, and stack problems remain in `findings` while
the operation returns `result: applied` and exits successfully. This lets callers inspect and
repair an unhealthy repository using the same stable result shape.

The operation fails only when it cannot construct trustworthy read context:

- invalid arguments, invocation path, or topology-blocking configuration produce
  `invalid-input`;
- unavailable authoritative Git refs, remote failures, or Git executable failures produce
  `external-failed`;
- cancellation or timeout produces `interrupted`; and
- an impossible internal contract violation produces `internal-error`.

Failures still emit exactly one protocol-v1 result. They do not emit a partial `StatusResult` that
could be mistaken for a complete repository report.

## Compatibility and test strategy

Implementation follows test-driven development and covers the slice at four levels.

### Application tests

Injected readers exercise both metadata modes, pinned-revision consistency, filter semantics,
artifact-source selection, partial document damage, stable ordering, cancellation, and the result
mapping for each failure class. Tests prove that filters cannot suppress health findings.

### Git integration tests

Temporary real repositories and bare remotes cover discovery from nested paths, dirty worktrees,
main and docket metadata modes, distinct integration and metadata revisions, branch facts for
stacks, missing refs, and concurrent remote movement. Before/after assertions prove that status
does not change checkout files, index state, `HEAD`, checked-out branches, or docket documents.
Targeted fetch updates to Git's object database and remote-tracking refs are the only permitted
local mutation.

### Frozen semantic corpus

The new status fixtures are historical snapshots derived from the refreshed `v0.9.3` tag at commit
`dd742abd5e9fcdf8ffe78eb6f36a293410873bbf`. Their provenance record names the tag, peeled commit,
capture procedure, selected source paths, and expected semantic outcomes. The corpus is versioned
as `v0.9.3`; it must not silently reuse or relabel the earlier `v0.9.2` fixtures.

The corpus compares semantics rather than Bash presentation bytes: status counts, selection order,
dependency satisfaction, effective stack bases, health classifications, and authoritative artifact
locations. The plan-writer feature present in `v0.9.3` is fixture input only; it does not add
plan-writing behavior to change 0310. Existing historical or rollback references to `v0.9.2` remain
unchanged outside this new corpus.

### Presenter and protocol tests

Golden tests pin representative healthy, unhealthy, filtered, and empty human reports and JSON
documents. Protocol tests enforce one JSON document, protocol version, result kind, empty-array
encoding, deterministic ordering, and exit mapping. Mutation probes remove the read-only guard,
artifact-source distinction, and full-corpus health behavior in turn and must make their protecting
tests fail.

The build gate runs the repository's complete configured test suite, not only change 0310's focused
tests.

## Explicit exclusions

Change 0310 does not own any of the following:

- configuration schema or capability-policy changes from 0305;
- document patching, serialization, or managed-block rendering from 0306;
- new domain lifecycle, graph, readiness, or selection policy from 0307;
- a rewritten Git adapter or general Git porcelain from 0308;
- transaction worktrees, leases, commits, pushes, compare-and-swap retries, or recovery from 0309;
- installer, embedded-asset, or harness behavior from 0311;
- mutation planning and renderers from 0312;
- feature workspaces, pull requests, or evidence capture from 0313;
- supervisor or delegated-process behavior from 0314;
- claim, finalize, reclaim, maintenance-sweep, or lifecycle mutation behavior from 0315 and 0316;
- release automation from 0317 or Bash cutover/removal from 0318; or
- change 0261's unmerged board surface and new health check. Existing landed marker data may be
  reported only through the domain semantics already available to this slice.

It also does not edit the approved program map or architecture design, create a new program-level
decision, or require a new ADR.

## Acceptance boundary

Change 0310 is complete when `docket status` can read either supported metadata mode from pinned
authoritative Git objects, return deterministic protocol-v1 JSON and human output for status,
readiness, stack context, selection, artifact integrity, and health, and pass the `v0.9.3` semantic
corpus without mutating docket metadata or a user's checkout.

At that point the Go executable has a trustworthy read-only vertical slice. All write paths and
later workflow behavior remain owned by their existing changes.
