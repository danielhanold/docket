---
id: 57
slug: frontmatter-read-must-be-anchored-when-key-may-be-absent
title: A frontmatter read must be anchored when the key may be absent
status: Accepted
date: 2026-07-22
supersedes: []
reverses: []
relates_to: []
change: 127
---

## Context

The existing `field()` helper reads a change manifest field by taking the first line matching
`<key>:` anywhere in the file. That is safe only for keys guaranteed to appear in the frontmatter
block for every change (e.g. `id:`, `status:`), because there is nowhere else in a well-formed
file such a line could occur.

`typed-changes-selective-auto-capture` (change 127) introduced `type:` as an **optional** manifest
field. An optional key breaks `field()`'s safety assumption: when `type:` is absent from
frontmatter, a body prose line that happens to begin `type:` (e.g. inside a sentence broken across
lines, or a literal example) is returned as the value instead of "absent." Every call site reading
`type:` inherited this hazard.

## Decision

A frontmatter read must be **anchored to the frontmatter block** whenever the key it reads may be
legitimately absent — it must not scan the whole file the way `field()` does for always-present
keys. The new `fm_field` helper reads only the first `---`...`---` block (the frontmatter delimiter
pair at the top of the file) and returns absent if the key is not found within it, never falling
through to body prose. Every `type:` read in the change 127 implementation goes through `fm_field`,
not `field()`.

The audit of `field()`'s other existing call sites — to determine which, if any, read keys that
are similarly optional and should be migrated to `fm_field` — is out of scope for change 127 and is
tracked as follow-up change 134.

## Consequences

`type:` (and any future optional manifest field) is read correctly whether present or absent, with
no risk of a body line masquerading as the field's value. The codebase now carries two frontmatter
readers (`field()` for always-present keys, `fm_field` for optional ones) rather than one, so a new
call site must consciously choose the right one based on whether its key can be absent. Until change
134 lands, other `field()` call sites reading keys that are actually optional remain
un-audited and could share the same latent hazard `fm_field` was built to close.

## Update — 2026-08-08

The follow-up this ADR's Decision and Consequences track as **change 134** is **resolved by change
244**, not by 134. Change 134 ("audit field call sites for frontmatter anchored reads") and its
near-duplicate change 240 were both **killed into 244** during the 2026-08-07 triage
(`docs/changes/archive/2026-08-07-0134-audit-field-call-sites-for-frontmatter-anchored-reads.md`,
`docs/changes/archive/2026-08-07-0240-audit-which-frontmatter-accessor-each-call-site-should-use-n.md`).
Change 244 ("One selection rule for the four frontmatter read shapes", PR #184) merged onto `main`
on 2026-08-08 (merge commit `afbe0888`) and is archived at
`docs/changes/archive/2026-08-08-0244-one-selection-rule-for-the-four-frontmatter-read-shapes.md`.

What 244 delivered: it re-took the whole-repo census of `field()` / `field_raw()` / `fm_field*` /
`int_field` / `list_field` call sites; migrated the remaining optional-key `field()` reads to the
anchored tier (`fm_field` for structured values, `fm_field_verbatim` for the free-prose `blocked_by`,
whose ` #…` is data, not a comment); recorded a single selection rule as a decision table in the
`scripts/lib/docket-frontmatter.sh` header (canonical) with a cross-reference in
`scripts/board-checks.md`; kept `fm_field_raw` as a documented orphan (zero production callers,
pinned in both directions); and added a static (corpus class, key) census-allowlist guard plus an
absent-key behavioral fixture.

The durable refinement to this ADR's rule: the selection criterion is **semantic optionality** — can
any file this call site reads legitimately omit the key — **not** "the template ships it." Template
presence is only one sufficient reason a key is *not* guaranteed. And the guaranteed-key sets are
**per corpus** (change files, ADRs, learnings findings), so a guard keyed on key name alone is
insufficient.

No new ADR was produced, deliberately. ADR-0057 (anchor when the key may be absent) and ADR-0058
(the logical/raw two-tier split) remain the decision record; change 244 consolidated them into an
operational selection table rather than adding a third overlapping ADR.
