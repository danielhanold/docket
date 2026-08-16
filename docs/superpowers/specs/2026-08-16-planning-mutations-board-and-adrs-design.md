<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0312 — Planning mutations, inline board, and ADRs](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0312-planning-mutations-board-and-adrs.md)**
<!-- docket:backlink:end -->

# Planning mutations, inline board, and ADRs

**Change:** 0312

**Date:** 2026-08-16

**Status:** Approved

## Purpose

Change 0312 is the first complete planning-state write slice in the Go migration. It adds coarse
application operations for retained change, manual-learning, and ADR mutations, plus the derived
views those operations own. Each operation must leave the metadata branch valid in one atomic
commit, even when another writer updates the branch concurrently.

The approved program map and architecture design are upstream constraints. This design applies
them to change 0312; it does not revisit their decisions. In particular, Git and Markdown remain
authoritative, mutations run in isolated transaction worktrees, authored Markdown crosses the CLI
boundary through stdin or an explicit request file, and derived views are never authority.

## Independently deliverable result

After this change, the Go binary can perform these metadata operations:

- `change create`
- `change groom`
- `change block`
- `change defer`
- `change kill`
- `learning record`
- `learning update`
- `adr record`
- `adr supersede`
- `adr reverse`

Those operations are usable independently of the later implementation and finalization slices. A
new change can be captured and groomed into a build-ready record, manual learnings can be retained,
and ADRs can be created or replaced while the inline board, change artifact links, artifact
backlinks, and ADR index remain synchronized with their sources.

Change 0312 consumes the foundations delivered by changes 0305 through 0311. Most importantly, it
uses the transaction engine from 0309 and the authoritative, versioned read context from 0310. It
does not reimplement or broaden those foundations.

## Boundary with neighboring changes

This change owns planning records and their v1-owned derived views. It does not own:

- configuration, document parsing, domain rules, Git plumbing, the transaction engine, status, or
  installation behavior already assigned to changes 0305 through 0311;
- autonomous grooming, automatic learning capture or harvest, learning-index maintenance,
  learning-capacity enforcement, or promotion into `AGENTS.md`;
- feature worktrees, GitHub pull requests, build evidence, or child-process supervision;
- claim, reconcile, plan attachment, results attachment, or mark-implemented transitions;
- finalize, merge, reclaim, recovery, stack propagation, terminal publishing, or workspace cleanup;
- release packaging, compatibility cutover, self-hosting, or deletion of the Bash implementation;
- a GitHub Issues or Projects backlog mirror.

The later changes 0313 through 0318 retain all of those downstream responsibilities. Change 0312
may expose pure renderer and application seams they reuse, but it must not implement their
workflows.

`change kill` is intentionally narrow. It performs the metadata transition that 0312 owns and
moves the record to its required dated archive path in the same commit. It does not delete a
feature worktree, branch, pull request, or other external state; the later orchestration and
finalize slices own those effects.

## Architecture

The write path is:

```text
skill or human
  -> one-shot CLI with a typed operation request
  -> internal/app planning operation
       -> preflight and authoritative context from the 0305/0310 seams
       -> internal/repository transaction engine from 0309
            -> reload fresh snapshot for each attempt
            -> apply domain transition
            -> patch owned source fields and sections
            -> render every affected v1-owned view
            -> validate the complete candidate repository
            -> commit and exact-lease push
  -> one versioned result document
```

There is no generic frontmatter setter and no hidden workflow session. Each application operation
has a typed request, explicit preconditions, a closed set of files it may plan, and a typed result.
The transaction planner is semantic: on a retry it rebuilds the plan from the fresh authoritative
snapshot rather than replaying a stale byte patch.

### Package responsibilities

`internal/app` owns the coarse operations, request validation, clock use, preflight, context
assembly, transaction invocation, and mapping of transaction outcomes to the public result
taxonomy.

`internal/render` owns pure rendering and source-preserving authored-section edits for the record
types in this slice. It receives typed values and source bytes and returns bytes or a refusal. It
does not read the filesystem, invoke Git, inspect the clock, parse CLI flags, or commit anything.

`internal/cli` owns command parsing and human/JSON presentation only. Small scalar inputs may be
flags. Authored Markdown is read from stdin or an explicit request file and is never interpolated
into a shell command. Final command and flag spelling remains an implementation-plan concern, as
settled by the program architecture.

The existing `internal/domain`, `internal/document`, `internal/repository`, `internal/config`, and
0309/0310 application seams remain the authorities for lifecycle rules, loss-preserving field and
managed-block patches, repository validation, capability fences, transactions, and versioned read
context.

## Request contract

Every update request identifies each decision input by its semantic identity, current canonical
path, and exact full Git blob identity returned by the 0310 read context. A stale or moved target is
reported as contention; authored decisions are never text-merged.

Every creation request carries a stable, shape-validated request ID. Its canonical request digest
includes all semantic inputs, including authored section bytes and relationships. The 0309
idempotency mechanism records and replays the original result so a lost response cannot allocate a
second change, learning, or ADR.

Requests use typed fields rather than arbitrary field names. Relationship collections are supplied
as complete desired values, not incremental text edits. Authored body input is a collection of
named section values with one of three intents:

```text
preserve  leave the existing section byte-identical
replace   replace or insert this operation-owned section with the submitted Markdown
remove    remove this operation-owned section
```

The application validates the complete request before it starts a transaction: required scalars,
request ID, authored-section shape, duplicate relationships, target references, submitted entity
versions, and operation-specific invariants. A validation failure cannot leave a partial record or
derived view.

### Authored-section safety

Existing documents are not regenerated. The renderer locates exact top-level Markdown headings
outside fenced code blocks, requires each operation-owned heading to be unique, validates all
requested edits first, and then splices edits from the end of the file toward the beginning. It
parses and validates the candidate again before returning it to the transaction.

For change records, the owned authored headings in this slice are:

- `## Why`
- `## What changes`
- `## Out of scope`
- `## Open questions`
- `## Why deferred`
- `## Why killed`
- `## Auto-groom blocked`

For learning records, the owned authored headings are `## Apply` and `## War story`. ADR content is
newly authored at creation and frozen after acceptance; supersede and reverse edit only the old
ADR's owned status field.

Unknown headings, unknown fields, comments, field order, quoting, blank lines, line endings, and
all unowned body bytes remain byte-identical. Managed blocks are still changed through the
loss-preserving document layer, including its marker-order and balance checks. Newly created
records use canonical Go-owned templates and serializers.

## Change operations

### `change create`

The request contains a stable request ID, title, type, priority, authored `Why`, `What changes`, and
`Out of scope` sections, and complete initial dependency, stack, relationship, discovery, and ADR
references.

On every transaction attempt the operation scans the fresh active and archived corpus, allocates
`max(id) + 1`, derives and validates the canonical slug, and plans one canonical active change
record. It never fills an ID gap. The new record has the normal proposed-state defaults, an empty
managed artifact block, no claim, and the current application date. Existing Bash-era records are
not normalized merely because the corpus was scanned.

All references and cycles are validated against the candidate snapshot. If the configured board
surface is `inline`, the same plan includes the candidate board bytes. If `board_surfaces` is empty,
the operation does not create, remove, or rewrite a board. A requested `github` surface is rejected
by preflight before the transaction begins.

### `change groom`

The request contains the exact proposed change version and chooses exactly one outcome: authored
spec or trivial. It also carries complete desired relationship values and structured edits for the
proposal sections. Grooming never claims the change.

For an authored spec, the operation:

1. Requires the change to need design and to be legal to groom.
2. Validates the submitted spec Markdown before mutation.
3. Creates `docs/superpowers/specs/<UTC-date>-<change-slug>-design.md`.
4. Inserts the generated change backlink into that spec.
5. Sets the change's `spec` field to that path and keeps `trivial: false`.
6. Applies only the submitted proposal-section and relationship edits.
7. Removes a resolved `Open questions` section and any stale `Auto-groom blocked` section when the
   request asks to remove them.
8. Updates the change date, artifact block, and inline board.

The spec and every metadata output above are one transaction plan. There is no state in which the
change points to a missing spec or the spec points at an old change location.

For a trivial verdict, the operation creates no spec, requires a concise authored rationale in the
proposal body, sets `trivial: true`, leaves `spec` empty, applies the same safe relationship and
section rules, and regenerates the affected change artifacts and board.

### `change block`

The request contains the exact change version and a non-empty block reason. The domain operation
must accept the current lifecycle state. The renderer changes only the owned lifecycle fields,
updates the date and managed artifact block, and includes the inline board when enabled. It does
not inspect or stop a child process; process-aware blocking belongs to later workflows.

### `change defer`

The request contains the exact change version and an authored `Why deferred` section. The domain
operation must accept the transition from the current state. The renderer changes only owned
lifecycle fields, safely inserts or replaces that section, updates the date and artifact block,
and includes the inline board when enabled.

### `change kill`

The request contains the exact change version and an authored `Why killed` section. The domain
operation must accept the transition. Using the application clock's UTC date, the planner:

- applies the killed-state fields and clears claim metadata through the domain operation;
- safely inserts or replaces `Why killed`;
- creates the canonical dated archive path and deletes the former active path;
- rerenders the archived record's artifact block;
- updates the backlink in a metadata-resident linked spec, when present, to the archive location;
- includes the inline board when enabled.

All of those metadata changes are atomic. The operation makes no assertion that external feature
state has been cleaned up. Later orchestration must sequence or perform those external effects.

## Manual learning operations

Both learning operations require `learnings.enabled: true`. They are the deliberate supported
exception to the otherwise deferred automatic learning subsystem.

### `learning record`

The request contains a stable request ID, a valid slug, hook, topic list, related change IDs, and
authored `Apply` and `War story` sections. The operation validates the complete corpus and creates
one canonical retained finding with `promotion_state: retained`, an empty promotion target, and
current created/updated dates. It refuses a duplicate canonical slug rather than overwriting an
existing finding.

### `learning update`

The request contains the exact learning path and blob identity plus complete desired hook, topics,
change references, and structured edits for `Apply` and `War story`. It preserves identity,
creation date, promotion state, promotion target, unknown fields, and unknown body sections. It
updates the date only when semantic content changes.

Neither operation regenerates the learnings README/index, enforces a retention cap, harvests a
result, decides promotion, or edits `AGENTS.md`. Existing learnings-index bytes are outside the
file plan and remain untouched.

## ADR operations

ADR creation requests carry a stable request ID, title, status-independent authored decision
sections, complete relationship lists, and an optional exact version of the producing change. On
each attempt the planner allocates `max(ADR id) + 1` from fresh state and never fills a gap.

### `adr record`

The operation creates one canonical Accepted ADR with the current date and supplied `Context`,
`Decision`, `Consequences`, and `Alternatives considered` sections. When a producing change is
supplied, the ADR ID is atomically appended to that change's typed `adrs` collection, the change
date is updated, and its artifact block is rerendered.

### `adr supersede`

The request identifies an exact Accepted ADR and supplies the complete authored successor ADR. The
domain transition creates the new Accepted ADR with a `supersedes` edge and changes only the old
ADR's status to Superseded. The old accepted body remains frozen. An optional producing change is
updated as for `adr record`.

### `adr reverse`

The request has the same shape as supersede but records a reversal edge and changes only the old
ADR's status to Reversed. The old accepted body remains frozen. An optional producing change is
updated as for `adr record`.

Every ADR operation renders the complete ADR index from the candidate ADR snapshot in the same
transaction. A renderer or validation refusal means neither source ADR nor index is committed.
This slice does not add a generic ADR update operation; immutable accepted content and relationship
rules remain domain authority.

## Renderer contracts

The renderers in `internal/render` are deterministic pure functions. Equal typed input, source
bytes, link context, and configuration produce byte-identical output.

### Canonical records

New change, learning, and ADR records follow the current templates, field types, field order,
quoting guarantees, section order, marker spelling, and trailing-newline convention. Free-text
frontmatter scalars are quoted by construction. Existing records use loss-preserving patches and
are never passed through the canonical new-record serializer.

### Inline board

The board renderer consumes the complete candidate change snapshot and the same readiness,
dependency, stack, and selection policies used by status. It reproduces the established inline
surface ordering and wording. It never becomes an input to lifecycle decisions.

With `board_surfaces: [inline]`, every change operation in this slice includes the board in its
closed mutation plan, even if the resulting bytes are unchanged. With `board_surfaces: []`, the
renderer is not invoked and any historical board file is preserved. `github` remains an
unsupported active request, not a fallback.

### Change artifact block

The artifact renderer derives links from typed change fields, the candidate repository snapshot,
and explicit link context. It is the sole writer of the managed `docket:artifacts` block. The
forward block is informational: validation and operations continue to use typed frontmatter and
repository paths as authority.

### Artifact backlink

The backlink renderer is the sole writer of the managed backlink at the top of a spec created or
moved by this slice. Its target is the change's current canonical metadata path, never a line
number. Plans and results are not created or attached here; later slices may reuse the renderer.

### ADR index

The ADR-index renderer consumes the complete candidate ADR snapshot and reproduces the established
status grouping, group order, links, and empty-group behavior. It is output only. ADR source files
remain authoritative.

## Atomic file plans

The minimum closed file sets are:

| Operation | Primary files | Derived files in the same plan |
| --- | --- | --- |
| change create | new active change | artifact block in record, inline board when enabled |
| change groom, spec | change and new spec | artifact block, spec backlink, inline board |
| change groom, trivial | change | artifact block, inline board |
| change block/defer | change | artifact block, inline board |
| change kill | archived replacement and active-path deletion | artifact block, linked spec backlink when present, inline board |
| learning record/update | learning record | none; learning index is preserved |
| adr record | new ADR, optional producing change | ADR index, optional change artifact block |
| adr supersede/reverse | new ADR, old ADR, optional producing change | ADR index, optional change artifact block |

An affected file is omitted only when the operation contract says the surface is disabled or the
file is outside this slice. Byte equality may yield a transaction-level no-op, but the planner does
not deliberately leave a v1-owned view stale.

## Concurrency and idempotency

Allocation, lifecycle validation, section patching, and rendering all run again from freshly loaded
state on every transaction retry. Derived-view overlap with an unrelated writer therefore causes a
replan, not authored-content contention. A changed decision input, moved target, or incompatible
lifecycle transition returns `contended` or `invalid-state` according to the transaction and domain
contracts; it is never silently overwritten.

Creation receipts use the 0309 bounded canonical JSON format and closed structs. A replayed stable
request returns the original semantic identity and committed result. Reusing a request ID with a
different digest is an invalid input. Operations that allocate nothing do not require an
idempotency key, but they do require exact submitted entity versions.

## Results and failures

The CLI follows the versioned protocol settled by the program architecture. In JSON mode it emits
exactly one document on stdout; diagnostics go to stderr. The application maps outcomes into the
shared result kinds: `applied`, `no-op`, `contended`, `invalid-input`, `invalid-state`, `blocked`,
`unsupported-config`, `gate-failed`, `external-failed`, `interrupted`, and `internal-error`.

Results expose only operation-specific closed fields such as change or ADR ID, slug, canonical
path, committed revision, and whether an idempotent result was replayed. They never expose local
transaction-worktree paths. Handled refusals do not rely on exit-code or prose parsing.

Preflight, request validation, renderer validation, and candidate repository validation all occur
before a commit. If any fails, the authoritative ref remains unchanged. Transaction worktree
cleanup and interrupted-push classification remain responsibilities of the 0309 engine rather than
being reimplemented here.

## Verification strategy

The implementation must prove the following at package and real-Git boundaries.

### Source-preserving mutation tests

- Each supported field and section edit changes only its owned bytes.
- Unknown fields, comments, quoting, headings, fenced-code headings, blank lines, and line endings
  survive byte-identically.
- Duplicate or malformed owned headings and unbalanced or out-of-order managed markers refuse the
  complete edit.
- Candidate documents are reparsed and repository-validated.
- Negative mutation tests remove each guard and demonstrate that the relevant test fails.

### Renderer tests

- Canonical change, learning, and ADR records match approved golden bytes.
- Board, artifact, backlink, and ADR-index renderers are deterministic and match the established
  semantic corpus already owned by the 0310 status slice.
- Empty and inline board configurations behave distinctly, and an active GitHub surface is fenced
  before mutation.
- The frozen compatibility corpus is copied with explicit provenance; tests do not scan live
  templates or silently update historical fixtures.
- The learning index is unchanged by both manual learning operations.

### Operation tests

- Capture followed by authored grooming produces one build-ready change with a linked spec and
  synchronized board.
- Trivial grooming produces no spec and remains independently build-ready.
- Block, defer, and kill exercise every domain-legal transition and reject illegal ones.
- Kill atomically archives the record and retargets a metadata-resident spec backlink without
  attempting external cleanup.
- Manual learning record/update preserves promotion metadata and never writes the learning index.
- ADR record, supersede, and reverse allocate monotonically, freeze old content, update the ADR
  index, and link an optional producing change atomically.
- Every creation operation replays after a simulated lost response and rejects request-ID reuse
  with a different digest.

### Concurrent repository tests

Using real temporary bare remotes in both repository modes:

- unrelated concurrent mutations retry and preserve both writers' authored decisions;
- two operations against the same submitted entity version produce one winner and one typed
  contention result;
- concurrent allocators never produce duplicate change or ADR IDs;
- derived views reflect the winning candidate snapshot and cannot trail their source records;
- a renderer or whole-repository validation failure produces no pushed commit;
- each successful operation produces one explicit-path metadata commit and leaves no transaction
  worktree behind.

The build gate runs the repository's full configured suite, not only the tests enumerated here.

## Acceptance criteria

Change 0312 is complete when:

1. All ten typed operations above are available through the Go application and one-shot CLI seams.
2. Existing records are edited only through owned fields, managed blocks, and structured authored
   sections; no operation accepts a whole replacement record for an existing document.
3. New records are canonical, creation is idempotent, and retries recompute allocation and derived
   bytes from fresh authoritative state.
4. Every source mutation and affected v1-owned board, artifact block, backlink, or ADR index lands
   in one validated transaction commit.
5. Manual learning writes leave the deferred learning index and promotion workflow untouched.
6. Kill performs only its metadata transition and archive move; downstream external cleanup and
   finalization behavior remain absent.
7. No behavior owned by changes 0313 through 0318 is implemented.
8. Compatibility, failure, idempotency, and concurrent-writer tests pass in both repository modes,
   followed by the full configured test suite.
