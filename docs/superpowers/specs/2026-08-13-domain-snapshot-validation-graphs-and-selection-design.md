<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0307 — Domain snapshot, validation, graphs, and selection](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0307-domain-snapshot-validation-graphs-and-selection.md)**
<!-- docket:backlink:end -->

# Domain snapshot, validation, graphs, and selection

**Change:** 0307 · **Type:** feat · **Priority:** critical · **Date:** 2026-08-13 · **Status:**
Approved design

## Purpose and boundary

This change gives Go one immutable, typed interpretation of a Docket repository and one pure source
of lifecycle policy. It decodes already-supplied documents into changes, ADRs, and learning
findings; validates the complete record set; builds dependency and stack graphs; evaluates legal
transitions, readiness, selection, claims, and reclaim; and returns typed policy outcomes without
reading a filesystem or invoking a subprocess.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) are fixed upstream constraints.
This spec resolves only change 0307. It does not reopen the hard-cutover, compatibility,
agent-first, transaction, or capability decisions in those documents, and it does not implement
behavior assigned to changes 0308–0318.

## Landed foundation and independently deliverable result

Changes 0305 and 0306 are complete.

- `internal/config` supplies a resolved `config.Snapshot` whose `Effective` aggregate contains only
  supported Go-v1 policy, including repository layout, integration branch, reclaim TTL, learning
  enablement, change types, and the retained priority-related defaults.
- `internal/document` supplies immutable source bytes, typed frontmatter decoding, exact field and
  managed-block locations, loss-preserving patches, and canonical typed values. It deliberately
  owns no change, ADR, learning, lifecycle, or graph vocabulary.

Change 0307 adds the inward policy layer between those foundations and later adapters. Its
independently reviewable deliverable is:

- an `internal/domain` package containing immutable Docket entities, repository policy, lifecycle
  actions, readiness, graph algorithms, selection, claims, reclaim, and typed policy outcomes;
- the read-model portion of `internal/repository`, which decodes an in-memory set of
  `document.Document` values into a complete immutable snapshot and deterministic validation
  report;
- before/after validation for policy that cannot be proven from one snapshot, especially frozen
  ADR content; and
- table-driven tests that reproduce retained `v0.9.2` semantic outcomes without filesystem,
  Git, GitHub, shell, or process dependencies.

Change 0308 later supplies authoritative Git objects and remote-ref facts. Change 0309 adds the
isolated transaction engine to `internal/repository`. Change 0310 composes config resolution,
document parsing, the snapshot builder, and the Git object source into status and health output.
This change defines the pure contracts those consumers need; it does not perform their work.

## Chosen architecture

Use two packages with one-way dependency flow:

```text
internal/config ───────┐
                      ├──> internal/repository/readmodel ───> internal/domain
internal/document ────┘
```

`internal/domain` imports neither `config` nor `document`. It knows typed repository policy and
typed entities, not how YAML or Markdown expressed them. The read-model files in
`internal/repository` are the anti-corruption boundary: they decode `document.Document`, translate
the supported leaves of `config.Effective` into domain policy, retain document identity needed for
evolution checks, and build the domain snapshot.

The repository package may be a flat package in Go; `readmodel` above names responsibility, not a
required subdirectory. Change 0309 extends the same `internal/repository` package with transactions
later. No package cycle and no outward import into `internal/domain` is permitted.

Two alternatives are rejected:

- Letting `internal/domain` import `internal/document` would make lifecycle policy depend on
  Markdown representation, contradicting the approved dependency direction and spreading YAML
  concerns into every later policy consumer.
- Deferring snapshot construction and complete validation to change 0310 would under-deliver this
  change and turn the status slice into the owner of repository semantics. The same policy must be
  reusable by transactions, planning, implementation, and finalize without importing status code.

## Domain vocabulary and immutability

### Closed value types

`internal/domain` defines closed types for values whose membership is policy:

```go
type ChangeID int
type ADRID int

type Status string
const (
    StatusInProgress   Status = "in-progress"
    StatusProposed     Status = "proposed"
    StatusBlocked      Status = "blocked"
    StatusDeferred     Status = "deferred"
    StatusImplemented  Status = "implemented"
    StatusStackedMerged Status = "stacked-merged"
    StatusDone         Status = "done"
    StatusKilled       Status = "killed"
)

type Priority string
const (
    PriorityCritical Priority = "critical"
    PriorityHigh     Priority = "high"
    PriorityMedium   Priority = "medium"
    PriorityLow      Priority = "low"
)
```

The declaration order above is not selection order by accident: one package-owned priority rank
table is the source for selection and validation tests. Unknown stored priorities are preserved for
diagnostics and use the retained `medium` fallback when a read-only selection is computed. Stored
change types are validated for the `[a-z][a-z0-9-]*` token shape but are not rejected merely because
the current machine's effective `change_types` list omits them; configuration governs creation,
not readability of shared history.

ADR status is parsed into a tagged value rather than retained as an unstructured string:

```text
Accepted | Deprecated | SupersededBy(ADR ID) | ReversedBy(ADR ID)
```

Learning promotion state is `retained`, `candidate`, or `promoted`; a missing legacy value resolves
to `retained` without rewriting the record.

### Entities

The domain model contains these repository record types and policy:

- `Change`: every retained manifest meaning, including identity, dates, relationships, artifact
  references, optional stack parent, optional claim/branch/PR state, `trivial`, `reconciled`, and
  the presence of Docket-owned state markers needed by policy;
- `ADR`: identity, status, date, `supersedes`, `reverses`, `relates_to`, optional producing change,
  and an opaque content identity supplied by the repository decoder;
- `Learning`: slug, hook, topics, related changes, dates, promotion state, promotion destination,
  and authored content;
- `Artifact`: a spec, plan, results file, or other authored Markdown artifact identified by path
  and kind, with opaque content identity and managed-marker presence but no domain interpretation of
  its prose;
- `DerivedView`: an inline board or index identified by path and kind, retained for accounting but
  never consulted as authority; and
- `RepositoryPolicy`: integration branch, effective change types, reclaim TTL, and learning
  enablement copied from resolved configuration.

Optional frontmatter stays optional in the decoded type. The decoder distinguishes absent, empty,
malformed, and present values long enough to report the right validation finding. It does not
launder malformed input into a zero value that later looks valid.

All aggregate fields are unexported or defensively copied. Constructors copy caller-owned slices,
maps, strings backed by byte buffers, and source identities. Snapshot accessors return values or
fresh collections; mutating an accessor result can never mutate the snapshot or another result.
Policy functions return a new value and never mutate their input.

## In-memory repository decoder

The repository read model consumes inputs shaped like:

```go
type RecordKind string // change | adr | learning | artifact | derived-view
type RecordLocation string // active | archive | ledger | artifact | derived

type InputDocument struct {
    Kind     RecordKind
    Location RecordLocation
    Path     string
    Document document.Document
}

type BuildInput struct {
    Config    config.Effective
    Documents []InputDocument
}
```

These names and the direction of the boundary are fixed for planning: callers supply already-read,
already-parsed documents and explicit repository-relative identity. The builder makes no path
discovery, file read, Git query, clock read, or branch probe.

The decoder uses record-specific private wire structs with YAML tags and calls
`Document.DecodeFrontmatter`. Unknown fields remain compatibility data in the document sidecar and
do not fail typed decoding. Lists preserve authored order because dependency diagnostics use that
order for deterministic tie-breaking. Dates and claim timestamps keep both their raw presence and
their parsed result so an invalid timestamp is never treated as evidence of expiry.

The result contains:

```go
type BuildResult struct {
    Snapshot domain.Snapshot
    Report   ValidationReport
}

func BuildSnapshot(input BuildInput) (BuildResult, error)
```

Every supplied path remains accounted for. A record whose semantics cannot be indexed is retained
as an invalid record in the snapshot report rather than silently dropped. Duplicate IDs retain both
record identities and make lookup ambiguous; no winner is selected by file order. Queries that
need an unambiguous entity return a typed `inconclusive` outcome until the data is repaired.

The builder returns a usable snapshot alongside findings whenever it can safely account for the
input set. Programmer errors in the call shape may return a Go error; repository defects are data
and travel through `ValidationReport`.

## Validation model

### Findings and mutation gate

Validation findings are structured data, not rendered prose:

```go
type Severity string // error | warning

type Finding struct {
    Code      string
    Severity  Severity
    Entity    EntityRef
    Field     string
    Related   []EntityRef
    Detail    map[string]string
}
```

The code and structural parameters are stable; change 0310 owns human text and protocol mapping.
Findings sort deterministically by code, entity kind, numeric identity or slug, path, and field.
`ValidationReport.HasErrors()` is the one repository-validity gate future mutating operations call
after configuration preflight and before applying a semantic operation. Warnings remain visible but
do not masquerade as errors.

Read-only inspection is not denied merely because a repository is damaged. It receives the
snapshot and complete report so it can name every repair in one pass. Policy queries exclude
ambiguous or structurally unusable records and say `inconclusive`; they never guess.

### Single-snapshot validation

The complete-set pass covers:

- usable numeric IDs, slug/title/status/priority/date domains, path grammar, filename identity, and
  unique IDs, slugs, and paths;
- active versus archive placement, terminal meanings, and archive filename dates;
- state-coherence rules guaranteed by the final `v0.9.2` writers: `in-progress` carries a branch
  and valid claim stamp; `blocked` carries `blocked_by`; `implemented` carries branch, claim stamp,
  plan, PR, and `reconciled: true`; `stacked-merged` additionally carries a usable stack parent;
  terminal records carry terminal status in the archive and no claim stamp. Fields that are
  legitimately absent from older valid records remain tolerated where no lifecycle state requires
  them;
- validity of `depends_on`, `related`, `discovered_from`, `adrs`, `stacked_on`, ADR relationship,
  ADR `change`, learning `changes`, and supplied artifact references;
- dependency and stack cycles, including self-cycles, with every cycle member identified;
- exact ADR status/edge back-pointers and dangling relationships;
- ADR numbering gaps as warnings, preserving the existing ledger posture; and
- malformed learning identities, topics, dates, promotion states, and promotion destinations.

Validation keys on semantic shape, never enumerated bad spellings. Stored optional historical
fields are not retroactively required simply because today's template contains them. The named
transition validators below enforce the stronger postconditions for newly produced states.

### Before/after evolution validation

Some rules require two authoritative snapshots. `ValidateEvolution(before, after)` performs those
checks without writing either snapshot.

An ADR that existed in `before` has frozen existing content after acceptance. The only permitted
evolutions are:

1. replace the `status:` value through a legal deprecate, supersede, or reverse action while every
   other existing byte stays identical; or
2. while the ADR remains `Accepted`, append one or more `## Update` sections at EOF while every
   pre-existing byte remains identical.

The repository decoder uses `document.Document` field spans to mask only the status value during
the comparison; it never normalizes or re-encodes the ADR. An update may use the retained legacy
`## Update` heading or the current `## Update — …` shape. Existing `## Decision` bytes and every
other prefix byte remain frozen. A superseded, reversed, or deprecated ADR cannot receive a later
body update.

The evolution pass also rejects identity reuse or mutation of an existing record's immutable
identity. Transaction-level entity versions, expected refs, retry, and commit atomicity remain
change 0309.

## Lifecycle policy

### Named actions, not generic setters

The domain exports a closed action vocabulary and validates actions rather than exposing a generic
`SetStatus` policy API. It covers:

- claim `proposed → in-progress`;
- refresh an active claim lease without changing status;
- block `in-progress → blocked` and clear `blocked → in-progress` with a non-empty reason on the
  block action;
- defer `proposed|in-progress → deferred` and revive `deferred → proposed`;
- kill `proposed|in-progress → killed`;
- mark `in-progress → implemented` after the PR postconditions are supplied;
- mark `implemented → stacked-merged` when the verified merge destination is the stack parent;
- mark a root `implemented → done` or promote `stacked-merged → done` only when reachability from
  the integration branch is supplied as a verified fact; and
- reclaim `in-progress → proposed` only through the reclaim predicate below.

Killing a stack parent is a distinct graph action, not a widening of the ordinary block action. It
identifies every non-terminal descendant and returns `blocked` with the retained re-scope,
re-parent, or kill reason; an already-blocked descendant is a semantic no-op. The later finalize
operation owns persistence and recovery of that result.

Each action accepts all external facts it needs as explicit immutable input: an injected UTC time,
verified merge destination, PR identity, or branch-presence set. No policy function calls
`time.Now`, Git, or GitHub. A successful evaluation returns the next semantic `Change` plus a typed
description of fields that changed. It does not patch Markdown, author body sections, render links,
commit, push, open a PR, or clean a branch.

Illegal source state, unmet precondition, or malformed supplied fact returns a typed policy failure
whose kind can later map to protocol `invalid-state`, `blocked`, or `invalid-input` without parsing
prose.

## Readiness, dependencies, and deterministic selection

### Dependency state

`depends_on` and `stacked_on` remain orthogonal graphs. A dependency is satisfied only when the
referenced change is `done`; `implemented` and `stacked-merged` do not satisfy it.

For a proposed change, dependency evaluation returns every unmet edge and the retained summary:

- any `implemented` dependency yields `needs-merge`, which outranks `not-built` for the summary;
- all other non-`done` or missing dependencies yield `not-built`; and
- within the same reason, the first dependency in authored `depends_on` order is the display
  representative.

Missing references are also validation errors. Read-only dependency evaluation still reports them
as unmet rather than treating absence as `done`.

### Readiness

Readiness is a typed outcome, not a Boolean. Its cases are `build-ready`,
`needs-brainstorm`, `auto-groom-blocked` for retained historical markers, `waiting-dependency`,
`stack-base-unresolved`, `invalid`, and `not-proposed`.

A change is build-ready exactly when it is:

1. `proposed`;
2. carrying a non-empty `spec:` or `trivial: true`;
3. clear of unmet dependencies; and
4. unstacked or carrying a resolved effective base.

The build/readiness projection preserves the retained precedence: an unmet dependency reports
`waiting-dependency` before missing design is considered; missing design reports
`needs-brainstorm` or `auto-groom-blocked`; and stack resolution is consulted only for a change that
would otherwise be build-ready. A separate `NeedsDesign` predicate ignores dependency satisfaction
so interactive grooming can design ahead exactly as the convention permits. Autonomous grooming
itself is deferred from Go v1; retaining the marker distinction does not reintroduce its selector
or critic.

### Selection

Selection filters to unambiguous build-ready changes, then sorts by:

1. priority: `critical`, `high`, `medium`, `low`;
2. well-formed `created` date ascending; malformed or absent dates sort after every valid date in
   the same priority band; and
3. numeric change ID ascending.

An unknown stored priority produces a validation finding but uses the retained `medium` rank for a
read-only queue. Selection accepts optional type and priority filters as typed values, but report
formatting and CLI argument parsing belong to change 0310.

## Stack graph and effective bases

The stack graph indexes the single optional `stacked_on` edge separately from dependencies. It
provides parent, ancestor, child, and parent-before-child descendant walks without recursion that
can loop forever on bad input. Missing parents and every cycle member are validation findings.

Effective-base resolution consumes a precomputed set of remote branch names plus the integration
branch. It returns a tagged result with the resolved branch or an exact non-resolution reason; it
does not collapse all failures into an empty string.

The retained rules, in precedence order, are:

1. an unstacked change resolves to the integration branch;
2. a killed parent returns `parent-killed`, naming the exact ancestor reached even when a branch
   with its recorded name still exists;
3. a `done` parent resolves directly and terminally to the integration branch, because `done`
   means its code is reachable there, even when its old branch still exists;
4. any other parent whose recorded branch is present in the supplied remote-branch set resolves to
   that branch;
5. a branchless `stacked-merged` parent recursively resolves its own effective base, because its
   commits merged into its parent rather than the integration branch; and
6. a missing parent, cycle, malformed parent edge, or any other live parent with no supplied remote
   branch returns a distinct invalid/unresolved cause.

Rule 3 is ADR-0092 and must have a discriminating test: a `done` parent above a killed grandparent
still resolves to the integration branch and never recurses into the killed ancestor. No caller may
fall back to the integration branch for a killed or invalid chain.

Remote-ref discovery belongs to 0308. Status and health presentation of these typed outcomes
belongs to 0310. Feature-branch creation and PR bases belong to 0313. Stack merge and terminal
close-out execution belong to 0316.

## Claims and reclaim

Claim eligibility is build-readiness plus an unambiguous current snapshot. A claim action requires
an injected second-precision UTC timestamp and returns `in-progress`, the deterministic
`feat/<slug>` branch name, that timestamp as `claimed_at`, and `reconciled: false`. Removing a
historical `## Run halted` body section is a later document-operation responsibility; the domain
action identifies that marker as a required owned removal.

Lease evaluation has typed results for `fresh`, `expired`, `missing`, `malformed`, and
`not-in-progress`. Missing or malformed timestamps are never positive evidence of expiry.

Reclaim eligibility retains all three conjuncts from `v0.9.2`:

1. status is `in-progress`;
2. `claimed_at` parses and `now - claimed_at` is strictly greater than the configured TTL; and
3. neither the recorded branch nor the conventional `feat/<slug>` branch exists in the supplied
   local-or-remote branch-presence facts.

The strict `>` boundary is intentional: equality is still fresh. A branch of either name makes the
claim non-reclaimable regardless of lease age. A successful reclaim returns `proposed`, clears
`branch` and `claimed_at`, and sets `reconciled: false`; the dated reclaim-log prose, document patch,
CAS retry, and commit belong to later operations.

## ADR and learning policy

### ADRs

The ADR graph validates `supersedes`, `reverses`, and `relates_to` references. A supersede edge from
ADR X to ADR Y requires Y's status to be exactly `Superseded by ADR-X`; a reverse edge requires the
verb-matched `Reversed by ADR-X`. A status target must exist. Supersede and reverse are distinct
named actions that create a new `Accepted` ADR and flip only the target's status in one semantic
result; no generic body editor is exposed.

Next-ID calculation is pure `max(existing IDs)+1`; gaps remain warnings rather than allocation
targets. Slug rendering, canonical new-record bytes, ADR-index output, metadata commits, and
publication belong to change 0312.

### Learnings

The snapshot decodes existing learning findings so later context queries can return stable slug,
hook, topics, promotion state, and authored content. With learning consumption disabled in policy,
domain learning-candidate queries return an explicit disabled result and no findings. The stronger
zero-I/O guarantee is enforced by the future source composer: it must not supply learning documents
when `learnings.enabled` is false.

When enabled, the domain can return the catalog and filter by explicit topic/slug inputs. It does
not decide semantic relevance from prose; the calling skill owns that judgment. Promoted findings
remain readable historical records but do not count as active. Automatic harvest, index rendering,
capacity checks, promotion, and autonomous capture remain deferred. Manual learning record/update
operations and canonical document rendering belong to change 0312.

## Error and result boundary

Domain failures use a closed package-local kind plus structural fields such as change ID, ADR ID,
edge, state, or missing fact. They contain no shell exit code and do not import `internal/app`.
Application operations later map them to the already-landed protocol result taxonomy.

Expected policy outcomes are values, not Go errors: a waiting dependency, contended claim,
unresolved stack base, fresh lease, or invalid repository finding is ordinary domain state. Go
errors are reserved for violated API preconditions or impossible internal conditions. This keeps
future CLI code from deciding behavior through error-string parsing.

## Testing strategy

### Pure domain tables

Table-driven tests construct values entirely in memory and cover:

- all lifecycle action source states, legal results, required facts, and illegal transitions;
- spec/trivial/dependency/stack readiness precedence;
- dependency satisfaction at `done` only and `implemented` summary precedence;
- selection across all priorities, malformed-priority fallback, malformed dates, and ID ties;
- dependency self-cycles, multi-node cycles, dangling edges, and deterministic member reporting;
- every stack-base rule, including remote-branch presence, branchless `stacked-merged` recursion,
  killed ancestors, missing parents, cycles, and ADR-0092's `done` terminal arm;
- claim and lease states, the strict TTL boundary, missing/malformed timestamps, recorded versus
  conventional branches, and reclaim output; and
- ADR status parsing, exact verb back-pointers, dangling relationships, gap warnings, and legal
  versus illegal evolution.

### Decoder and snapshot tables

Repository tests feed `document.Document` values parsed from in-memory literals and embedded frozen
`v0.9.2` corpus bytes. They cover every change/ADR/learning field, optional-key absence with
key-shaped body prose, unknown-field preservation, duplicate IDs/slugs/paths, active/archive
placement, archive dates, artifact and derived-view accounting, reference validity, and complete
finding aggregation. No test treats a derived view's content as authoritative input.

Tests prove immutability by mutating every input slice/map after construction and every collection
returned by an accessor. The original snapshot and a separately computed policy result must remain
unchanged.

The accepted-ADR evolution tests compare bytes, not re-encoded YAML: status-only change passes;
append-only `## Update` passes; editing `## Decision`, an earlier update, a comment, an unknown
field, or any other existing byte fails. Removing the status mask or prefix check must redden a
focused test.

No production path in these packages imports `os`, `os/exec`, Git, GitHub, CLI, render, process, or
harness packages. A dependency-direction test pins that boundary. Every safety guard has a
mutation probe that removes the guarded predicate and observes a focused failure. Manual Go reruns
use `-count=1`, and the build gate runs the whole resolved suite.

## Explicit exclusions

Change 0307 does not implement:

- filesystem or authoritative Git discovery, fetch/ref/object reads, blob entity versions, remote
  branch enumeration, or Git error handling (0308);
- transaction worktrees, expected-ref leases, entity-version contention, commits, pushes, retry,
  request-ID idempotency, or recovery cleanup (0309);
- status/health context assembly, human text, protocol JSON, board-health compatibility, or
  mutating maintenance (0310);
- installation, assets, or harness rendering (0311);
- create/groom/block/defer/kill commands, document patches, board/artifact/backlink/ADR-index
  rendering, ADR writes, or manual-learning writes (0312);
- feature workspaces, effective-base branch creation, GitHub PRs, or build evidence (0313);
- process supervision or gates (0314);
- agent workflow orchestration, reconciliation judgment, plan/results attachment, run verification,
  or the transition-driving application commands (0315);
- merge/rebase execution, archive moves, stack close-out, reclaim sweeps, halted/finalize-blocked
  recovery, or cleanup (0316); or
- packaging, self-hosting, configuration contraction, or Bash retirement (0317–0318).

It also does not introduce a database, daemon, public Go API, repository schema version, generic
frontmatter setter, generic lifecycle status setter, autonomous grooming, learning relevance model,
or replacement rendering format.

## Acceptance boundary

Change 0307 is complete when in-memory `document.Document` values plus resolved configuration can
produce an immutable typed snapshot and complete deterministic validation report; pure queries
reproduce retained lifecycle, readiness, selection, dependency, stack, claim, reclaim, ADR, and
learning-consumption outcomes; invalid or ambiguous records are reported without silent loss;
future mutations have one error-level repository-validity gate; and table-driven tests prove the
behavior without filesystem or subprocess access.

No Git adapter, transaction, status command, renderer, workflow operation, archive/reclaim sweep,
or other behavior owned by changes 0308–0318 is required for that proof.
