# discovered-from provenance links — design

- **Change:** 0090 — discovered-from provenance links
- **Date:** 2026-07-17 (UTC)
- **Author:** docket-auto-groom (autonomous self-brainstorm; every decision audited below)
- **Status:** build-ready design
- **Related:** #0035 (Artifacts link block — done), #0091 (auto-create discovered stubs — separate consumer, being groomed concurrently), #0010 (provenance-graph analytics — out of scope)

## Problem

docket already *generates* follow-up work — implement-next's reconcile/review passes, groom
brainstorms, and close-out findings all surface new changes (this repo's own board shows chains
like 0075→0076 and 0086/0087 from 0062) — but the provenance of that work lives only in prose.
`related:` is symmetric and unordered; nothing in the manifest records "this stub exists *because*
building #NN surfaced it." Beads' `discovered-from` dependency type captures exactly this, and its
FAQ calls it "agent-specific" precisely because autonomous agents constantly spawn follow-ups whose
origin is valuable graph data (planned vs discovered ratio; which changes spawn the most work).

## Goal

Add one optional manifest field, `discovered_from:`, that records the change id(s) whose work
surfaced this change; document its semantics in the convention; seed it in the change template; and
wire it into the *human-attended* capture path (`docket-new-change`) so it is usable the day it
ships. The field is the deliverable — this change is the data layer.

## Scope boundary — standalone, not merged with #0091

This change delivers **only the provenance field** and its documentation/population in flows that
mint change stubs *today*. Autonomous mid-run stub-minting (implement-next reconcile, finalize's
harvest, auto-groom deciding to file a follow-up) does **not** exist yet — that behavior is #0091.
So #0090 defines and documents the field; #0091 later consumes it by populating it from the
autonomous minters. The two are cleanly layered (data field first, autonomous consumer second), and
#0090 is coherent whether #0091 is eventually merged into it, made to `depends_on` it, or never
ships. The "merge #0090 with #0091?" open question is therefore resolved here as **land #0090
standalone**; the fold-in decision remains the humans' (and #0091's groomer's) to make and is not
foreclosed by this design.

## Design

### The field

```yaml
discovered_from: [62]    # change id(s) whose work surfaced this one; empty/absent for planned work
```

- **Name:** `discovered_from` — snake_case, matching every other multi-word manifest key
  (`depends_on`, `blocked_by`, `auto_groomable`).
- **Shape:** a **list of ids**, parallel to `related:` / `depends_on:` / `adrs:` (every existing
  cross-reference field is a list). `list_field()` in `lib/docket-frontmatter.sh` already parses
  `[a, b]` → `a b`, so consumers read it for free with no parser change.
- **Semantics:** purely **informational**, exactly like `related:`. It is **never** a readiness
  gate (unlike `depends_on:`), never introduces blocking, and is directional (child → origin(s)).
  Empty or absent means deliberately planned work.
- **Placement:** immediately after `related:` in the manifest frontmatter block (both the
  convention's manifest section and the change template), grouped with the other informational
  cross-refs.

### No automatic back-link

Setting `discovered_from: [62]` on this change does **not** auto-edit change #62 to add
`related: [90]`. The writer sets one field on the new stub; nothing reaches out to mutate another
change file. (Reverse queries — "what did building #62 spawn?" — are a cheap derived scan over
`discovered_from` across `active/` + `archive/`, needing no stored back-link.)

### Backward compatibility

Frontmatter is parsed field-by-field (`field`/`list_field`/`int_field` in
`lib/docket-frontmatter.sh`, all `sed`-based); there is no enumerated key allowlist or schema
anywhere in the scripts. Consumers read only the fields they know about, so adding
`discovered_from` is transparent to every existing reader (board render, health checks, mirror,
Artifacts renderer). Existing change files without the field are unaffected.

### Touchpoints (for the implementer)

1. **`docket-convention` SKILL.md** — add `discovered_from:` to the change-manifest frontmatter
   block (after `related:`) with the comment above, plus one sentence noting it is informational
   like `related:` and never a readiness gate. Describe it generically ("set by whichever flow
   mints the stub when an origin is known") so the prose stays accurate whether or not #0091 ships.
2. **`skills/docket-new-change/change-template.md`** — add the seeded empty `discovered_from:` line
   (after `related:`) with the same comment.
3. **`skills/docket-new-change/SKILL.md`** — extend step 3 ("Scan related context", which already
   pre-fills `related`/`depends_on`/`adrs`) so that when the human names an originating change
   (brainstorm mode) or scan mode infers one, it records `discovered_from`. Prose only — new-change
   writes frontmatter by hand.

Keep additions minimal: all three touched files carry size budgets in
`tests/test_skill_size_budgets.sh` (both SKILL.md files *and* `change-template.md`), so favor a
single manifest line + one clarifying comment/sentence per file over new paragraphs. Headroom is
tight but sufficient for one line + one comment each.

### Rendering — deferred, deliberately

This change adds **no new render surface**, for two reasons:

- The per-change `## Artifacts` block (`render-change-links.sh`) renders *document* artifacts only
  — `spec`/`plan`/`results`/`pr`/`adrs`. Cross-reference fields `related:` and `depends_on:` are
  deliberately **not** in that block; putting `discovered_from` there would break its semantic.
- A board surface (a column, annotation, or mermaid provenance edge in `render-board.sh`) is a
  heavier change — renderer + `board-checks.sh` + tests — for an informational field, and provenance
  analytics is explicitly #0010's territory.

The field lives in frontmatter (visible in the raw change file) and is fully documented, queryable,
and populated without a render surface — the same posture `related:`/`depends_on:` have in the
Artifacts block. A future change (or #0091 / #0010) may add a surface; this one keeps the blast
radius to the data layer.

## Out of scope

- Auto-creating stubs and autonomous mid-run population (#0091).
- Provenance-graph analytics / reporting (#0010).
- Any new blocking or readiness semantics — `discovered_from` is informational, full stop.
- A render surface (board column / Artifacts row / mermaid edge) — deferred, see above.
- Back-filling `discovered_from` onto existing stubs whose provenance is currently in prose — that
  needs per-change human judgment and is not required for the field to work going forward. Optional
  human follow-up.
- A health check for dangling `discovered_from` ids — `related:` has no such check either; skip to
  keep the change minimal (possible follow-up).

## Testing considerations

- Any test asserting the change template's exact frontmatter field set (if one exists) must be
  updated to include `discovered_from`; the implementer's reconcile pass will surface it.
- No parser change is needed, so no board/mirror/health-check test should regress; confirm the
  existing suite stays green after the template + convention edits.
- Respect the skill size-budget guard (`tests/test_skill_size_budgets.sh`) when editing the two
  SKILL.md files **and** `change-template.md` — all three have a budget row.

## Dependency state

`depends_on: []` — nothing gates this change. `related: [35, 91]`: #0035 (the Artifacts block) is
`done`; #0091 is `proposed` and being groomed concurrently as a separate consumer. This change does
not depend on #0091 and must not wait on it.

## Assumptions (deferred-human audit trail)

Each decision below is one an interactive brainstorm would have raised; the chosen default is
conservative and rooted in existing docket convention, and every rejected alternative is recorded.

1. **Land #0090 standalone; do not merge with #0091.** *Chosen* because the field is a coherent,
   valuable unit on its own (humans can record provenance the day it ships; #0091/#0010 build on the
   data model) and #0091 is being groomed concurrently — its state is not knowable here. *Rejected:*
   merging the two (couples this design to #0091's undecided scope and to a sibling groom in flight);
   making #0090 `depends_on` #0091 (backwards — the field is the dependency, not the dependent). The
   fold-in remains a human decision and is not foreclosed.

2. **Field name `discovered_from`.** *Chosen* for snake_case consistency with `depends_on` /
   `blocked_by` / `auto_groomable` and it is the stub's own suggestion. *Rejected:* `discovered-from`
   (hyphen matches beads but not docket YAML keys); `origin` / `spawned_by` (less precise).

3. **List shape `[ids]`, not a single id.** *Chosen:* every existing cross-ref field is a list;
   `list_field()` parses it for free; a stub can legitimately be surfaced by work spanning multiple
   changes. *Rejected:* single scalar id — simpler but inconsistent with the manifest and unable to
   express multi-origin.

4. **No automatic `related:` back-link on the origin.** *Chosen:* auto-editing another change's
   frontmatter is a cross-file mutation with concurrency and terminal-state hazards (the origin is
   usually an archived `done` change), and collapsing provenance back into symmetric `related:`
   defeats the directional point of the field. *Rejected:* auto back-link (couples writes across
   files, risks touching archived changes, re-introduces the symmetry this field exists to escape).
   Reverse queries are a cheap derived scan.

5. **Standalone population only in `docket-new-change` (human-attended).** *Chosen:* no autonomous
   skill mints change stubs today, so new-change is the sole minter that can know an origin; wiring
   it into new-change's existing step 3 is a minimal, real, usable capability. *Rejected:* wiring
   population into implement-next / finalize-harvest / auto-groom now — those don't mint stubs until
   #0091, so the code would be dead. The convention documents the field generically so #0091 slots in
   without rework.

6. **Defer all rendering.** *Chosen:* the Artifacts block is document-only (cross-refs `related` /
   `depends_on` are absent from it by precedent) and a board surface is heavier than an informational
   field warrants; the field works fully without a surface. *Rejected:* an Artifacts-block row
   (semantically wrong — it is not a document artifact); a board column / mermaid edge now (scope
   creep into #0010's analytics territory).

7. **No back-fill, no dangling-id health check.** *Chosen:* both add scope and human-judgment burden
   for no forward-going benefit; `related:` sets the precedent (no dangling check). *Rejected:*
   back-filling existing chains (needs per-change human judgment on fuzzy prose provenance);
   validating references (scope creep). Both noted as optional follow-ups.
