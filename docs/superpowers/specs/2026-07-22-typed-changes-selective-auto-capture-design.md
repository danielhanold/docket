# Typed changes, selective auto-capture, and backlog filters — design

- **Change:** #0127 (`typed-changes-selective-auto-capture`)
- **Date:** 2026-07-22 (UTC)
- **Type:** `feat`
- **Relates to:** #0090 (discovery provenance), #0091 (auto-capture), #0094 (selection-order digest), #0124 (backlog triage), ADR-0012 (script/model boundary), ADR-0019 (configuration layers), ADR-0045 (best-effort auto-capture), ADR-0052 (resolver boundary)

## Problem

`auto_capture: true` currently treats every material discovery alike. In this repository, that has
made narrow guard, documentation, and bug-fix follow-ups compete in the same needs-brainstorm queue
as product features. At design time, 16 of the 23 proposed changes carried `discovered_from:`
provenance, and the existing triage change (#0124) measured 10 auto-captured stubs among 17
needs-brainstorm changes. The capture path is doing what it was built to do—preserve discoveries—
but Docket has no vocabulary or policy for deciding which kinds of work belong in that path.

Every record is currently just a “change.” Without an explicit type, Docket cannot distinguish a
feature from a fix, refactor, documentation task, performance improvement, or chore; users cannot
limit auto-capture by category; and backlog reports cannot isolate the work they intend to review.

## Goals

- Give every newly created change an explicit, configurable type.
- Let auto-capture admit only selected types while preserving its best-effort posture.
- Make type and priority useful as read-time filters without letting a filter alter lifecycle work
  or the canonical board.
- Categorize this repository's active backlog once, without rewriting archived history.
- Give other repositories the same explicit, human-approved active-backlog migration path.
- Resolve all new settings through Docket's complete configuration stack.

## Non-goals

- Interactive filtering inside GitHub-rendered Markdown. `BOARD.md` remains a static derived view.
- A GitHub Projects integration, custom field, plugin, or new GitHub mirror behavior.
- Reclassifying archived changes.
- Inferring types from Conventional Commit prefixes in git history.
- Changing the seven-state change lifecycle or the build-ready selection order.
- Making type a new readiness gate for legacy archived records.

## Design

### 1. Manifest type and default taxonomy

Add a singular `type:` field to the change manifest. The shipped taxonomy is:

```yaml
change_types: [chore, docs, feat, fix, refactor, perf]
```

The defaults borrow the familiar Conventional Commit vocabulary without making Docket commit
messages or lifecycle semantics depend on Conventional Commits. `change_types` is ordered,
non-empty, duplicate-free, and contains lowercase tokens matching `[a-z][a-z0-9-]*`.

Every new change path writes `type:`:

- Interactive `docket-new-change` infers the best configured type from the request and presents it
  as part of design approval. It asks a separate type question only when classification is
  genuinely ambiguous or the requested type is unavailable.
- Trivial and scan paths use the same configured taxonomy; scan mode classifies each candidate.
- Auto-capture classifies each discovery before deciding whether to mint it.
- The deterministic mint helper accepts an explicit type and writes it into the first frontmatter
  block. It never performs the semantic classification itself (ADR-0012).

The value `type: feat` is not an implicit fallback in stored records. The creating agent must make
and expose the classification. Existing archived records may remain without `type:` forever.

### 2. Four-layer configuration

All new settings are supported in every Docket configuration layer, with the existing precedence:

```text
repo-local > repo-committed > user/global > built-in
```

The built-in layer is defined consistently in the resolver, convention skill, examples, and user
documentation. The two human-controlled machine/repository layers and the committed repository
layer can override every setting:

```yaml
change_types: [chore, docs, feat, fix, refactor, perf]

auto_capture:
  enabled: false
  types: all
```

`auto_capture` becomes a map. This is an intentional breaking change: the old scalar
`auto_capture: true|false` form is invalid, has no compatibility shim, and fails closed with a
migration-oriented diagnostic. Omitting the entire map is equivalent to:

```yaml
auto_capture:
  enabled: false
  types: all
```

Map leaves resolve independently, so a high-precedence layer can override `enabled` while
inheriting `types`. List values never merge: a higher-layer `change_types` or
`auto_capture.types` replaces the complete lower-layer list. That replacement rule is required so
a user can remove a built-in value instead of only adding values forever.

`auto_capture.types` is either the scalar `all` or a duplicate-free list drawn from the effective
`change_types`. Its built-in default is explicitly `all`, meaning every effective change type is
eligible. A higher-layer list such as `[feat, fix]` replaces `all` and narrows admission;
`enabled: false` suppresses all minting regardless of the selector.

The resolver removes `AUTO_CAPTURE` and emits three explicit values for downstream consumers:
`CHANGE_TYPES`, `AUTO_CAPTURE_ENABLED`, and `AUTO_CAPTURE_TYPES`. Their shell-safe serialization is
a plan-time detail, but it must preserve configured order and distinguish the literal `all` from a
list. Every skill consumes only resolver exports and never reparses YAML (ADR-0052).

Because higher layers may deliberately differ between machines, a consumer must still render and
filter a type already stored in a change file even when that value is not in its current effective
`change_types`. Configuration governs creation on this run; it cannot make shared historical data
unreadable. Manifest values `all` and `untyped` are forbidden: `all` is a configuration selector
and query pseudo-value, while `untyped` is only a query/migration pseudo-value.

### 3. Selective auto-capture

The existing materiality bar remains unchanged: only distinct, actionable follow-up work that
would deserve its own change/PR reaches classification. Learnings still go to the learnings ledger,
current-change drift stays in current work or the reconcile log, and observations stay in the run
report.

For each material discovery:

1. The autonomous skill assigns one effective configured type.
2. If `AUTO_CAPTURE_ENABLED` is false, it follows the existing disabled posture and reports the
   discovery without minting.
3. If enabled but the assigned type is outside `AUTO_CAPTURE_TYPES`, it does not mint and reports
   the proposed title and type as policy-suppressed.
4. If enabled and admitted, it calls the deterministic mint helper with the type.
5. Mint success, dedup, cap overflow, and hard failure retain ADR-0045's best-effort reporting
   posture and never abort the change being built.

The existing mint sites and exclusions remain unchanged. `docket-implement-next` and the
finalize/status close-out harvest can mint; `docket-auto-groom` cannot mint because doing so would
break its termination invariant; interactive skills already have a human decision point.

Filtering by type happens before the existing per-invocation cap is consumed. A suppressed `docs`
candidate must not use one of a `[feat, fix]` run's three mint slots. Existing active-title/slug
dedup remains after admission, immediately before minting.

### 4. Canonical board and read-time filters

`BOARD.md` stays the complete, static, canonical inline surface. Each active-state table gains a
`Type` column. Missing active values render as `untyped` during migration. Archive tables remain
unchanged because the archive is intentionally not backfilled.

`docket-status` gains two orthogonal report filters:

```text
docket-status --type all
docket-status --type fix
docket-status --type untyped
docket-status --priority all
docket-status --priority high
docket-status --type fix --priority high
```

Omitting either option is equivalent to its `all` pseudo-value. `type` accepts an effective
configured type, an active type already present in the repository, `untyped`, or `all`. Priority
accepts the existing four values or `all`. Combined filters use logical AND.

The filters affect only the displayed active backlog projection: the report's `change` lines and
its `ready` queue. They do not narrow the input set for merge detection, sweep, harvesting,
archiving, publishing, health checks, reclaim, or canonical board regeneration. Lifecycle/action
report lines remain visible even when they describe a change outside the backlog filter, because
they report work the command actually performed.

The implementation must preserve a hard boundary between the filtered digest projection and the
unfiltered writer. A filtered `--board-only` invocation may return filtered `change`/`ready` lines,
but the `BOARD.md` it refreshes still contains every active change.

### 5. One-time active-backlog categorization

Missing `type:` in an active change is a migration finding, not an immediate global blocker.
`docket-status --type untyped` is the exact inventory. Normal operations remain available during
the transition, while every creation path writes a type, so the untyped set can only shrink.

The rollout adds a deterministic `backfill-change-types` facade operation. Its input is a complete
human-approved mapping of untyped active change ids to configured types. The semantic division is:

- An interactive agent reads each active change and proposes a complete mapping.
- The human reviews and approves that mapping as a single decision.
- The helper validates and applies it mechanically.

The helper:

- scans only `<changes_dir>/active/`;
- anchors every edit to the first balanced frontmatter block;
- refuses unknown ids, duplicate assignments, malformed types, missing assignments for the
  migration set, and any assignment that conflicts with an existing non-empty type;
- writes all validated files or none of them;
- is idempotent when rerun with the already-applied mapping;
- commits and pushes the migration as one metadata-branch change through the normal preflight/CAS
  discipline; and
- never reads or edits `<changes_dir>/archive/`.

This repository's rollout includes a human-approved assignment for every active change present at
the migration point, including #0127. Other repositories receive the same inventory → proposal →
approval → deterministic-apply workflow when they adopt the release. Archived records stay
byte-identical. Its scalar `auto_capture: true` configuration migrates without a policy change to:

```yaml
auto_capture:
  enabled: true
  types: all
```

### 6. Validation and failure behavior

Configuration fails closed when:

- `change_types` is empty, not a list, contains duplicates, or contains a malformed token;
- `auto_capture` is not a map;
- `auto_capture.enabled` is not boolean;
- `auto_capture.types` is neither the scalar `all` nor a list, contains duplicates, or names a type
  absent from the effective `change_types`; or
- either list has a malformed YAML shape at any layer.

The diagnostic names the offending layer and key. A legacy scalar `auto_capture` diagnostic shows
the new nested shape. Whole-list replacement and per-leaf map inheritance must be visible in the
resolver's documented export contract.

Change-file validation treats a missing active `type:` as the migration finding described above.
It rejects `all`, `untyped`, empty values on newly written records, and malformed tokens. It does
not drop a board row because of a type problem: title/id/status row visibility remains independent
of type validation, following the existing board-row safety posture.

### 7. Documentation surfaces

Ship the schema and breaking migration end-to-end:

- `.docket.example.yml`, README configuration examples, global-config setup, and the convention's
  configuration block;
- the manifest contract, new-change template, new-change/auto-capture operating prose, and
  mint-stub contract;
- resolver and `docket-status` contracts, including layer precedence and filter semantics;
- board and health-check contracts; and
- a migration note showing the scalar-to-map rewrite and active categorization workflow.

The GitHub board mirror and GitHub Projects remain out of scope. A later change may add a
`docket:type/<type>` label or custom field if users opt into that surface.

## Verification

### Configuration

- Default, global, repo-committed, and repo-local fixtures cover each of `change_types`,
  `auto_capture.enabled`, and `auto_capture.types`.
- Cross-layer fixtures prove repo-local > repo-committed > global > built-in.
- Nested-map fixtures prove per-leaf fallback; list fixtures prove whole-list replacement rather
  than concatenation.
- Default and explicit `all` fixtures prove that every effective change type is admitted, while a
  higher-layer list replaces `all` and narrows admission.
- Legacy scalar booleans fail with the migration diagnostic.
- Malformed, duplicate, empty, and out-of-taxonomy values fail closed.
- Export ordering and documentation are guarded against drift.

### Creation and capture

- New-change template, scan, trivial, and auto-capture fixtures cannot create an untyped file.
- Mutation tests remove the type write from each executable mint path and make the relevant test
  fail.
- Capture-disabled, type-excluded, admitted, deduped, capped, and hard-failure outcomes are each
  reported correctly.
- A type-excluded discovery does not consume the cap.

### Board and status

- All active board tables show Type; archive output remains byte-compatible except for unrelated
  count/link movement.
- Omitted/`all`, configured type, observed type, `untyped`, priority, and combined filters produce
  the expected `change` and `ready` lines.
- A filtered full or board-only run still sweeps an out-of-filter merged change and writes a board
  containing in-filter and out-of-filter active rows.
- Invalid filter values fail with the accepted values and do not mutate state.

### Migration

- A complete valid assignment updates only active first-frontmatter blocks in one operation.
- Partial, duplicate, unknown-id, malformed-type, and conflicting-overwrite mappings leave every file
  untouched.
- Rerunning an applied mapping is a no-op.
- A snapshot/hash assertion proves all archive files are byte-identical.
- This repository's live active inventory reaches zero `untyped` records after its approved pass.

Run the entire repository suite at the build gate, not only the tests named here.

## Alternatives considered

### Lazy migration

Leave active records untyped until they are naturally edited. This reduces rollout work but leaves
the current backlog—the motivating problem—unfilterable for an indefinite period. Rejected.

### Capture-only classification

Type only auto-captured stubs and use the value only as a capture gate. This is smaller, but manual
changes remain undifferentiated and the board never gains a coherent type model. Rejected.

### Implicit type inference from titles or commit history

Compute type at read time instead of storing it. This makes classifications unstable as heuristics
change, hides the human decision, and cannot reliably distinguish a feature from a fix. Rejected.

### Interactive Markdown or GitHub Projects filtering

Move the filtering experience to browser behavior or a GitHub Project. GitHub-rendered Markdown
does not provide table filtering, and the repository's canonical surface should remain the existing
portable `BOARD.md`. Rejected for this change.

## Open questions

None. The design choices were approved interactively on 2026-07-22.
