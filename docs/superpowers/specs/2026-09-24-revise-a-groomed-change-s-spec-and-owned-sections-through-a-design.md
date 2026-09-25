<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0445 — Revise a groomed change's spec and owned sections through a typed operation](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-25-0445-revise-a-groomed-change-s-spec-and-owned-sections-through-a.md)**
<!-- docket:backlink:end -->

# Revise a groomed change's spec and owned sections through a typed operation

## Intent and scope

Add a typed way to adjust an already-groomed `proposed` change — its linked spec's body, its
owned proposal sections (`## Why` / `## What changes` / `## Out of scope` / `## Open questions`),
or both — in one exact-version-pinned metadata transaction, without the hand-edit-plus-plain-git-commit
workaround `docket-groom-next` currently documents. Repeatable: a change may be revised any number
of times while it stays `proposed` — each call is a standard exact-version CAS write, no one-shot
marker.

Out of scope: editing frozen build records (merged plans/results), Accepted ADRs, or terminal
(`done`/`killed`) changes; changing the spec path or relinking a different spec file; flipping a
change between spec'd and trivial or vice versa; revising an `in-progress` change (that is
`change.reconcile`'s existing job); any automatic or autonomous revision; spec review/approval
workflow.

## Investigation and prior decisions

Design baseline: main `9d4cb1fe22c4f11bc6bf4b2d2454468e5015a20d`, inspected 2026-09-24.

- `internal/app/change_groom.go` (`change.groom`) already owns the proposed/needs-design→build-ready
  transition. Its `Plan()` gate is one condition:
  `c.Status() != domain.StatusProposed || c.Spec().Value != "" || c.Trivial()` → refuse
  `not-groomable`. This is the exact complement of "already groomed" — a `revise` outcome needs the
  inverse gate, not new machinery, to select its target changes.
- The same file already has every mechanical piece a revision needs: `render.ApplySectionEdits`
  splices the owned proposal sections over exact source bytes (`toSectionEdits`/`validateGroomSections`,
  scoped to `render.ChangeOwnedHeadings`); `assembleSpecFile` + `render.BacklinkContent` assemble a
  spec file (backlink block, blank line, authored markdown); `buildGroomCandidate` +
  `render.ArtifactBlockContent` re-render the `## Artifacts` block; `includeBoard` re-renders the
  inline board when enabled; the whole thing runs under the engine's exact-blob CAS
  (`transaction.EntityExpectation`) via `deps.Engine.Execute`.
- `internal/app/change_reconcile.go` (`change.reconcile`) already revises a linked spec's body for
  `in-progress` changes: `ChangeReconcileRequest.SpecSections map[string]string` and
  `planSpecPatch` read the spec's exact bytes from the tree, section-splice the named headings via
  the same `render.ApplySectionEdits`, and `MutationReplace` the existing path — never minting a new
  path, never touching the `spec:` field. Its gate (`c.Status() != domain.StatusInProgress` →
  refuse, remapped to `contended`) is deliberately narrower than groom's: reconcile is tied to
  claim/reconcile-log machinery (`domain.RefreshClaim`, the appended `## Reconcile log` entry,
  `reconciled: true`) that has no meaning before a change is claimed. Pre-claim revision (this
  change's scope) and in-progress revision (reconcile's existing scope) are different lifecycle
  states with different obligations; they should stay on their existing respective operations rather
  than merge into one, per YAGNI — reconcile's claim/log obligations would be dead weight on a
  pre-claim revise, and groom's board/artifact-render path has no claim to refresh.
- Observed motivating case: change 0444 (`done`). A YAGNI review after grooming narrowed its spec;
  both the spec file and the change's `## What changes`/`## Out of scope` were hand-edited with
  plain git in the `.docket` worktree — bypassing the exact-version CAS and the `updated:` stamp
  every other metadata write gets.
- `docket-groom-next`'s skill body (`~/.claude/skills/docket-groom-next/SKILL.md`) already documents
  the gap this closes, in its own "When to use" section: *"Do NOT use to re-groom a change that
  already has a spec — drift against current reality is the reconcile pass's job in
  `docket-implement-next`. A human who wants to redo a design can clear `spec:` by hand first."*
  This is precisely the hand-edit workaround being replaced.
- `docket-new-change`'s skill body lands a change's first spec through the same `change.groom`
  operation (its own step 5, `outcome: spec`). It has no post-groom adjust step of its own — a
  human wanting to tweak a just-landed spec before moving on hits the same gap.
- No open or archived change proposes this mechanism (checked via a repo-wide grep for
  revise/respec/regroom/spec-revision language across `docs/adrs` and `docs/changes/active`); no
  ADR addresses it.

## Alternatives and decision

1. A new standalone operation (e.g. `change.revise-spec`) with its own gate and catalog id, separate
   from `change.groom`. Rejected: it would duplicate the gate check, the section-splice call, the
   spec-file assembly, and the candidate/artifact-render/board plumbing that already exist in
   `change_groom.go`, for a target state (`proposed`) `change.groom` already owns. A second
   operation over the same record kind and directory also doubles the schema-registry/CLI-verb
   surface for no behavioral gain.
2. A new `revise` outcome on the existing `change.groom` operation, gated on the complement of the
   current groom gate (`status==proposed && (spec!="" || trivial)`), reusing `SpecMarkdown` (whole
   spec-body replace, same field the `spec` outcome already uses) and `Sections` (the same owned
   proposal-section splice) unchanged. Selected: `change.groom` already owns "proposed, has a
   design decision to make or adjust" as its lifecycle scope; `revise` is the same scope's second
   half, not a new one.
3. Extend `change.reconcile`'s existing `SpecSections` section-patch machinery to also accept
   `proposed` changes. Rejected: reconcile's gate, receipt, and Plan steps are structurally tied to
   claim refresh (`domain.RefreshClaim`) and the mandatory dated `## Reconcile log` append — neither
   applies pre-claim, and stretching the gate to admit `proposed` would require conditionally
   skipping half of what the operation does, which is worse than the operation staying single-purpose.
4. Section-level patch for the spec body (mirroring `change.reconcile`'s `SpecSections` map, patching
   only named headings and leaving the rest of the spec untouched) instead of whole-body replace.
   Rejected for this change: the stub's own scope is "replace its linked spec's authored body,"
   matching `change.groom`'s existing `spec`-outcome shape (`SpecMarkdown`, a single field, no
   partial-edit ambiguity about which headings were meant to survive). A future change can add
   section-level spec patching to `revise` if a real caller needs partial edits; nothing here forecloses
   it, and no current caller has asked for it.

## Design

### Gate: the complement of the groom gate, unchanged inputs

`change_groom.go`'s `Plan()` gains a case on `o.req.Outcome`:

- `revise`: refuse `not-revisable` unless `c.Status() == domain.StatusProposed && (c.Spec().Value != "" || c.Trivial())`.
  This is exactly the negation of the existing `spec`/`trivial` gate — a change is either
  needs-design (groomable) or already-groomed (revisable) while `proposed`, never both, never
  neither.
- `revise` never sets `spec` or `trivial` on the record. Flipping a change's groom outcome (spec ↔
  trivial) stays impossible by construction, not by convention: no code path under the `revise`
  branch calls `ps.SetField("spec", ...)` or `ps.SetField("trivial", ...)`.

`ChangeGroomRequest` gains no new fields. `Outcome` becomes a three-value enum (`spec` | `trivial` |
`revise`); `FCInvalidOutcome`'s message updates to name all three. `SpecMarkdown` and `Sections` are
both already optional-by-omission fields on the wire (empty string / empty slice); `revise`
validation requires **at least one** of them non-empty-in-effect (a non-empty `SpecMarkdown`, or a
`Sections` list containing at least one `replace`/`remove` edit) — an all-preserve or fully-empty
revise request is refused `FCEmptyRevise` before the engine call, mirroring how `GroomTrivial`
already requires `hasAuthoredRationale`.

### Spec-body replace: reuse the existing spec-outcome assembly, target the existing path

When `SpecMarkdown` is non-empty under `revise`:

- Refuse `spec-not-linked` if `c.Spec().Value == ""` (a trivial-only change has no spec to revise;
  submitting `SpecMarkdown` against one is a caller error, not a partial no-op).
- Otherwise emit `transaction.FileMutation{Path: gitcli.RepoPath(c.Spec().Value), Kind:
  transaction.MutationReplace, Bytes: assembleSpecFile(backlink, o.req.SpecMarkdown)}` — the same
  `assembleSpecFile` and `render.BacklinkContent(gc, o.link)` calls the `spec` outcome already
  makes, targeted at the change's *existing* spec path instead of a newly minted dated path. No
  `spec-path-taken` check applies (there is no new path); no `treeHasPath` probe is needed.

### Change-record sections, fields, and derived views: exactly the groom path, minus spec/trivial

`Sections` splices via the existing `render.ApplySectionEdits(src, render.ChangeOwnedHeadings,
toSectionEdits(o.req.Sections))` call, unchanged. `updated:` is upserted unconditionally (as it
already is for `spec`/`trivial`). Requested relationship fields (`DependsOn`/`Related`/
`DiscoveredFrom`/`ADRs`/`StackedOn`) patch with their existing nil-unchanged/explicit-empty-clears
semantics, unchanged. The candidate-snapshot rebuild, `## Artifacts` block re-render, and
`includeBoard` inline-board re-render are the same calls the `spec`/`trivial` outcomes already make
— `revise` differs from them only in which fields it does *not* touch (`spec`, `trivial`) and which
file mutation it emits for the spec (replace-existing vs. create-new).

### Receipt and result

`changeGroomReceipt.Outcome` carries `"revise"`; `SpecPath` is populated with the existing spec path
when `SpecMarkdown` was applied, empty otherwise (mirroring the trivial outcome's empty `SpecPath`).
`ChangeGroomResult.HumanText` gains a `revise` case: `"change %04d revised — %s"`.

### Skill-doc wiring

`docket-groom-next`'s Step 1 explicit-id rule ("an explicit id that is not needs-brainstorm is an
error to report") gains a carve-out: an explicit id naming an already-groomed `proposed` change (has
`spec:` or `trivial: true`) routes to a **revise** flow instead of erroring — recap the change's
current spec/sections, run the resolved brainstorm skill seeded with what's there today framed as
"what would you like to adjust," and exit via `change.groom` with `outcome: revise` (the same request
shape as the `spec`/`trivial` exits, carrying whichever of `spec_markdown`/`sections` the adjustment
touched). The skill's current line documenting the hand-edit workaround ("clear `spec:` by hand
first") is removed — the typed path replaces it. `docket-new-change`'s brainstorm-mode step 5 gains a
pointer: after landing a change's first spec, a human who wants to adjust it immediately can run
`docket-groom-next <id>` to reach the same revise flow, rather than re-running `docket-new-change`
or hand-editing.

## Acceptance and verification

Extends the existing `internal/app/change_groom_test.go` fixture family; no new test file needed.

1. `revise` on a `proposed`, spec'd change with `sections` only → change record's sections and
   `updated:` change; the spec file is byte-identical; the board/artifacts block re-render is
   asserted.
2. `revise` on a `proposed`, spec'd change with `spec_markdown` only → the spec file's body changes
   (backlink block preserved), the change record's sections are byte-identical apart from `updated:`.
3. `revise` on a `proposed`, spec'd change with both → both land in the same commit/receipt.
4. `revise` on a `proposed`, trivial-verdicted change with `sections` only (adjusting the trivial
   rationale) → succeeds, no spec file touched.
5. `revise` with non-empty `spec_markdown` against a trivial-only (no-spec) change → refused
   `spec-not-linked`, nothing written.
6. `revise` against a needs-brainstorm change (no spec, not trivial) → refused `not-revisable`
   (this is `change.groom`'s existing target state, not revise's).
7. `revise` against an `in-progress`, `blocked`, `deferred`, `implemented`, or terminal change →
   refused `not-revisable`.
8. `revise` with an empty/all-preserve request (no `spec_markdown`, no effective section edit) →
   refused `FCEmptyRevise` before any engine call.
9. `revise` never writes `spec:` or `trivial:` — assert the groomed-outcome field values are
   byte-identical pre/post across every case above.
10. Stale/contended version on a `revise` call → the existing exact-version CAS refusal path,
    unchanged, writes nothing.
11. Repeated `revise` calls in sequence (each against the freshly-landed version) both succeed,
    proving no one-shot marker exists.
