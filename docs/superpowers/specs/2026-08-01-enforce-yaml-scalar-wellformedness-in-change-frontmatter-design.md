<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0191 — Enforce YAML scalar well-formedness in change-file frontmatter](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0191-enforce-yaml-scalar-wellformedness-in-change-frontmatter.md)**
<!-- docket:backlink:end -->

# Enforce YAML scalar well-formedness in change-file frontmatter — design

**Change:** 0191 · **Date:** 2026-08-01 · **Groomed:** autonomous (docket-auto-groom) · **Type:** fix. No
`depends_on` gate. Discovered from 0190's review, where an **unquoted `title:` containing a colon-space**
shipped silently — the line-based readers (ADR-0062) tolerated it, the board rendered it, and nothing
reddened.

## Context

AGENTS.md's house rule requires quoting any hand-authored YAML scalar that carries a colon-space or a
bare boolean keyword — but it is a rule, not a guard. Change 0190 proved the gap: its `title:` was
written unquoted with a `: ` inside; every reader (`field()`, the board, the digest) consumed it as
text, and only a human reviewer noticed. The shared reading is blunt about this: *"Today's grep/awk
reader tolerating it is not evidence it is well-formed."*

The board's `field-domain` check already guards the one shape that breaks its *own* output — a `title`
containing `|` (markdown column injection) — but no check enforces YAML scalar well-formedness. This
change adds that enforcement: a new mechanical check in `board-checks.sh` that flags an **unquoted**
frontmatter scalar carrying a colon-space or resolving to a YAML 1.1 bare boolean keyword.

The enforcement is a natural extension of an accepted rule, not a new architecture decision. ADR-0065
already states that a bare-scalar validator carries two legs — the raw-vs-consumed comparison and an
explicit **quote leg** (a raw value opening with `"` or `'` is not a bare scalar) — and that the rule
applies to "every `field`/`field_raw` validator pair in docket, present and future." This check applies
that quote-leg reasoning to change frontmatter and adds the colon-space leg the house rule names. It is
warn-only, exactly like every `board-checks.sh` health check: it surfaces the finding for a human, it
never edits.

## Goals

- A malformed scalar in a change file's frontmatter reddens the board's health pass instead of surviving
  silently until a strict YAML consumer or a human notices.
- The guard is **shape, not spelling**: the covered field set is derived from what docket actually reads
  and each field's capacity to carry the hazard — never an enumerated list of bad values — and each
  detection leg is a lexical-shape test consistent with ADR-0065.
- Warn-only: no auto-fix, no reader rewrite, no strict-YAML parser (ADR-0062 stands).

## Non-goals

- Rewriting the line-based readers or adopting an external YAML parser (ADR-0062).
- Changing the board's rendering, any **existing** reader's behavior, or `field-domain`'s existing
  pipe/domain checks. The one read-side addition — the new anchored-raw `fm_field_raw` helper in
  `lib/docket-frontmatter.sh` — is in scope because the optional `blocked_by:` key needs an anchored
  read (ADR-0057); no existing reader changes shape.
- Auto-fixing flagged files.
- Enforcing *balanced* quotes or full YAML string decoding: a scalar that OPENS with a quote is treated
  as the author's remedy and its interior is never nagged about; an unclosed quote is a different defect
  class, out of scope.

## Decision — a new sibling check-id `scalar-form` in `board-checks.sh`

Add a new board-checks check-id, **`scalar-form`**, emitted beside `field-domain` from a brand-new probe
block in the per-file walk of `board-checks.sh`. For each covered field it reads the **raw** token —
never `field()`/`fm_field()`, which unwrap quotes and destroy the evidence — and applies three legs:

1. **Skip leg** — empty value, or the raw value opens with `"` or `'` (the ADR-0065 quote leg): quoted is
   well-formed by definition; do not inspect its interior.
2. **Colon-space leg** — the unquoted raw value contains `: ` (the YAML plain-scalar rule) → flag.
3. **Boolean leg** — the unquoted raw value, case-folding as YAML 1.1 does, is exactly one of
   `on off yes no true false` → flag.

One finding per violated leg per field, message naming the field and the shape, e.g.
`title: unquoted scalar contains ': ' — quote it or reword (well-formed YAML)`. The check inherits
board-checks.sh's warn-only posture, does not mark `EXPLAINED`, and never touches `board-row-dropped`
(a malformed scalar does not drop a row).

### The read must be anchored for the optional `blocked_by:` key

The two covered fields are read differently, and the difference is load-bearing:

- **`title`** — always present in every change file, so the first-match-anywhere raw reader
  (`field_raw`, `scripts/lib/docket-frontmatter.sh`) is safe exactly as ADR-0057 reasons: the frontmatter
  line always wins, the scan never reaches the body. Read via `field_raw`.
- **`blocked_by`** — an **optional** key (ADR-0057 lists it among the keys a first-match-anywhere read
  falls through for). `field_raw` scans the whole file, so for a change that omits `blocked_by:` while
  the body happens to open a `blocked_by:` line, the check would read body prose as the value — a false
  finding on a clean file, the exact defect class the `frontmatter-anchored-read` learning warns about.

The lib has no anchored read that keeps quotes intact: `fm_field` is frontmatter-scoped but unwraps the
quotes this check needs. So this change adds one read-side helper — **`fm_field_raw`**, the `fm_field`
twin that keeps surrounding quotes intact (anchored to the first `---…---` block, same inline-comment
strip as `fm_field`) — and routes `blocked_by` through it. `field_raw` for `title`, `fm_field_raw` for
`blocked_by`. This is a **new helper, not a change to existing reader behavior**; `field_raw`/`field`/
`fm_field` are untouched.

### Why a new check-id, not a `field-domain` arm

`field-domain`'s contract is a value "well-formed text but outside its field's DOMAIN," scoped in its
own comment to "the four fields the board renderers consume" (`slug`/`priority`/`title`/`type`).
Extending it with a well-formedness arm would (a) claim `blocked_by` — which has never been a
`field-domain` field — under that id, silently widening the documented scope, and (b) mix two defect
taxonomies (domain violation vs. lexical form) under one id on the findings channel, which docket's
guard culture keeps one-finding-type-per-defect-class (ADR-0049). The `|` pipe arm lives in
`field-domain` because it is a *board-column injection* — a board-domain concern; a YAML well-formedness
violation is not. The cost of the separation is the bounded, well-trodden `BOARD_CHECK_IDS` pinning
(13→14: the array, the `--help` header, `board-checks.md`'s per-check sections, `docket-status.md`'s
report-line row, and `test_board_checks.sh`'s both-directions pin).

### Why `title` and `blocked_by` — the derived field set

The covered set is derived two ways, and both land on the same two fields. First, from what docket
actually reads: a whole-repo pass over the manifest keys consumed via `field`/`fm_field`/`field_raw`
yields `status`, `title`, `slug`, `priority`, `type`, `spec`, `branch`, `trivial`, `issue`,
`blocked_by`, `claimed_at`, `created`, `updated`, `id`, `pr`, `plan`, `results`, `reconciled`, plus the
list fields (`depends_on`, `related`, `discovered_from`, `adrs`). Second, from each field's capacity to
carry the hazard without its own existing gate:

- **Natively boolean** — `trivial`, `auto_groomable`, `reconciled`: a bare `true`/`false` there is the
  *correct, well-formed* YAML. A boolean-leg check on them is a false positive by construction —
  excluded.
- **Closed or shape-gated vocabularies** — `status`/`priority` (membership checks), `type` (shape
  `^[a-z][a-z0-9-]*$`), `slug` (`^[a-z0-9-]+$`): a `true` or `on` there already fires a `field-domain`
  finding, or (slug) cannot contain `: ` and a bare-boolean slug is a strict-YAML parse mismatch with no
  structural consequence — excluded to keep the guard the noise-free source.
- **Structurally incapable** — `id` (integer), `created`/`updated` (dates), `claimed_at` (ISO
  timestamp: colons but never `: `), `branch`/`pr`/`issue` (paths/URLs/numbers), the list fields (split
  on commas): none can hold a `: ` or equal a bare boolean.
- **Free-text string scalars** — `title` and `blocked_by`: the only two fields a hand-author can set to
  arbitrary text, both actually read (`render-board.sh` renders both; `docket-status.sh` reads
  `blocked_by` for its `judgment blocked` line), and neither is gated by an existing shape/domain check.
  Covered.

This is the shape-not-spelling discipline: the set is a derivation over the manifest's field types and
the reader inventory, never a hand-listed "these fields can be bad" enumeration.

## Candidate-form rationale (why three legs, and why over the raw token)

The raw token is the only lossless view: `field()` unwraps quotes (change 0138), so a quoted title with
a colon-space — exactly the 0190 shape that must NOT flag — is indistinguishable from a bare one after
unwrapping. `field_raw()` preserves the quotes, which is what makes the quote leg a one-character probe
instead of a parser. ADR-0065's lesson anchors the split: raw-vs-consumed comparison is a whitespace
test, so a bare-scalar claim needs the quote leg *beside* the comparison; this design keeps that
structure (skip-leg → colon-space → boolean) and adds the colon-space shape the house rule names.

## Ripple surfaces

| Surface | Edit |
|---|---|
| `scripts/board-checks.sh` | new `scalar-form` probe block in the per-file walk (title via `field_raw`, blocked_by via `fm_field_raw`; skip/colon-space/boolean legs; one emit per violated leg); add `scalar-form` to the `--help` header's check-id enumeration |
| `scripts/lib/docket-frontmatter.sh` | add `scalar-form` to `BOARD_CHECK_IDS` (the closed check-id vocabulary, 13 → 14); **add the new `fm_field_raw` helper** (the anchored raw twin of `fm_field` — quotes kept intact) that the `blocked_by` read requires |
| `scripts/board-checks.md` | new per-check section documenting the check-id, the derived field set, and the three legs; the existing "pinned against BOARD_CHECK_IDS" paragraph is unchanged |
| `scripts/docket-status.md` | add `scalar-form` to the `check <check-id>` report-line row enumeration |
| `tests/test_board_checks.sh` | `BOARD_CHECK_IDS` count 13 → 14; new `scalar-form` fixtures (red and green, see Assumptions); keep the emitted-set + help-header pins |
| `docs/changes/active/0191-….md` | this change: set `spec:`, resolve the `## Open questions`, set `updated:`; the field-write rule re-renders the `## Artifacts` block in the same commit, and the spec's back-link block is stamped by `render-artifact-backlink.sh` |

## Assumptions (deferred human audit trail)

1. **Check location — new sibling check-id `scalar-form`, not a `field-domain` arm.** Chosen because
   well-formedness is a distinct defect class from domain membership — the findings channel already
   separates defect classes into distinct check-ids (ADR-0049 governs that channel's *column
   integrity*: structural columns carry script-derived or shape-validated values, which `scalar-form`'s
   change-id column inherits) — and because a `field-domain` arm would re-scope that check's documented
   four-field contract to pull in `blocked_by`. Rejected: extending `field-domain` (cheap, and the `|`
   pipe arm is a form check inside it — but that arm is board-column injection, a *board-domain*
   concern, unlike a YAML-form violation) and a separate script (overkill; the findings channel is
   board-checks.sh's job). The cost — the four-surface `BOARD_CHECK_IDS` pinning — is the bounded,
   well-trodden 0104/0117 mechanics.
2. **Field coverage — `title` and `blocked_by` only.** Derived (see Decision) as the sole free-text
   string scalars that are read and not already gated. Rejected: "all four board-rendered fields"
   (status/priority/type booleans and slugs already trip their own membership/shape arms, so adding
   them double-reports), and "every reader-read scalar" (would flag `trivial: true` — a correct,
   natively-boolean value — as a violation: a false positive on every non-trivial change). This is the
   one decision that would most embarrass the design if it were wrong, and it is stated as a set with
   its derivation rather than as an appeal to coverage.
3. **Predicate shape — three legs over the raw token, quote-first, anchored per field.** `title` via
   `field_raw`, `blocked_by` via the new `fm_field_raw` — the raw view is mandatory (see the anchored
   read subsection). The quote leg is load-bearing the other way too: it must keep the **quoted
   colon-space title green** (the 0190 regression shape is the *accept* case, not the reject case). A
   raw value that OPENS with `"` or `'` is treated as quoted and skipped even if the closing quote is
   absent — balanced/unclosed-quote enforcement is out of scope (a different defect class, recorded
   here so the build does not re-derive it). Rejected: running the predicate over `field()` (unwraps
   quotes, so the accept case becomes indistinguishable) and a one-leg `colon-space-or-boolean` bundle
   (loses the diagnostic's shape-specific wording). Boolean matching is case-insensitive per YAML 1.1
   core (`Yes`/`TRUE` are booleans), against AGENTS.md's lowercase spellings.
4. **No auto-fix, no change to existing reader behavior** — inherits board-checks.sh's warn-only
   contract; `field_raw`/`field`/`fm_field` stay byte-for-byte; the sole read-side addition is the new
   `fm_field_raw` helper the anchored `blocked_by` read needs; ADR-0062 holds. The finding is surfaced
   for a human, the same way `field-domain`'s pipe finding is.
5. **No ADR, no ADR update.** The change codifies an existing house rule (AGENTS.md; the `yaml-scalar`
   finding; ADR-0065's already-"present and future" quote-leg rule) into a *guard*; it changes no
   accepted decision and adds no new one. Rejected: an ADR for a mechanical guard (ADR-0062/0058/0049
   already establish the reading, reader-pair, and findings-channel boundaries this check sits inside);
   a dated Update on ADR-0065 (its Decision already covers "every field/field_raw validator pair,
   present and future"). If the reviewer reads the colon-space leg as a new decision, that is follow-up,
   not a blocking ambiguity — recorded here so the build does not stall on it.
6. **Live first target is expected.** `0121`'s active change file carries an **unquoted colon-space
   title** (`The manifest's elsewhere: check proves a word occurrence, not a real config read`). The
   next board pass will surface it as the `scalar-form` check's first finding — warn-only by design,
   exactly the work the check exists to surface; not a regression, and a convenient arrival check that
   the guard fires on real history.
7. **Guard tests are mutation-tested, and pinned in both directions.** Broadly, per
   `guards-are-code` and `assert-detects-removal-not-replacement`: fixtures in `test_board_checks.sh`
   must include the **absent-value and body-prose** cases, not just the present-field ones (the natural
   green fixtures pass under a vacuous implementation; the absent/clean ones are what stop it being
   decoration). Two fixtures are non-negotiable for the anchoring: (a) a change whose frontmatter
   **omits** `blocked_by:` while its body opens a `blocked_by:` line must stay green — this proves the
   `fm_field_raw` read is anchored and returns empty (ADR-0057's absent-key fixture discipline; the
   unanchored `field_raw` would return the prose and misfire); (b) a change **without** a `title:`-key
   body-prose twin, because `title` is always present the anchored question does not arise. And each
   leg's removal must redden its own test (drop the colon-space leg → the colon-space fixture goes
   green; drop the quote-skip → the *quoted* colon-space fixture reddens, which is the wrong direction
   and proves the signature; drop the anchoring → the absent-`blocked_by` fixture misfires). The
   `BOARD_CHECK_IDS` pin (now 14) plus the emitted-set and help-header pins keep the new id from
   decaying.

## Open questions

None — the design commits every decision, including the two that would otherwise beg a human (field
coverage with the boolean-field trap; new check-id vs `field-domain` arm). The only live item at
build time — whether the current corpus renders the check green everywhere except the named `0121`
finding — has a defined answer in assumption 6 and is verified by the fixture suite.
