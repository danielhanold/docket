# Provenance — canonical new-record goldens (change 0312, task 2)

These three goldens are the **canonical v1 shapes** emitted by
`internal/render`'s `ChangeRecord`, `LearningRecord`, and `ADRRecord`
serializers. They are **hand-authored**, not copied from any live record, and
they are a historical snapshot under docket's frozen-golden contract: they must
**not** silently track the source templates below. If a template changes, this
golden does not follow it automatically — a human decides whether the canonical
Go shape should change, and updates the golden and its serializer together.

## Source templates (shape reference only — NOT tracked)

- `change.golden.md` mirrors `skills/docket-new-change/change-template.md`
  (field names, order, and defaults) but **without** the template's trailing
  authoring-hint comments, and **without** the template's `## Open questions`
  and `## Reconcile log` body sections. The canonical serializer emits only
  `## Artifacts` (with an empty `docket:artifacts` managed block, exact marker
  spelling from `internal/document/markers.go`), `## Why`, `## What changes`,
  and `## Out of scope`.
- `learning.golden.md` mirrors the live findings under
  `docs/changes/learnings/` on the `docket` branch (e.g.
  `cached-runner-serves-a-mutated-tree.md`), with `promotion_state: retained`
  and an empty `promoted_to:` (a freshly recorded finding, not yet triaged).
- `adr.golden.md` mirrors `skills/docket-adr/adr-template.md` and the live
  `docs/adrs/*.md`, adding the `## Alternatives considered` body section that
  the canonical v1 ADR carries.

## Why these diverge from the Bash-era templates (canonical by construction)

Every frontmatter field is emitted through `document.New`
(`internal/document/builder.go`), whose closed value model has exactly one
text-scalar constructor — `document.String`, which **always single-quotes**
(ADR-0071, unconditional quoting of free-text scalars). Consequently the
canonical records quote enum/date scalars the Bash templates leave bare —
`status: 'proposed'`, `priority: 'medium'`, `type: 'feat'`,
`created: '2026-08-16'`, `promotion_state: 'retained'`, `status: 'Accepted'`,
`date: '2026-08-16'`, and string list elements such as
`topics: ['testing', 'mutation']`. This is intended: quoting is a construction
property, not a per-field decision. Flow collections of integers stay unquoted
sequences (`depends_on: []`, `relates_to: [62, 71]`, `change: 312`), and unset
scalars render as the bare `key:` form (`spec:`, `promoted_to:`, `change:` when
nil).

## Regenerating

There is no generator script. These goldens are produced by the serializers
under test; the byte-equality tests in `record_test.go` are the drift assert.
To intentionally change a canonical shape, edit the serializer and the matching
golden in the same commit.
