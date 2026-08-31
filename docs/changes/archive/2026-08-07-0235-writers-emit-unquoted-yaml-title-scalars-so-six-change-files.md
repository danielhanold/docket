---
id: 235
slug: writers-emit-unquoted-yaml-title-scalars-so-six-change-files
title: Writers emit unquoted YAML title scalars, so six change files fail to parse
status: done
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: [234]
discovered_from: [234]
adrs: [71, 73]
spec: docs/superpowers/specs/2026-08-07-writers-emit-unquoted-yaml-title-scalars-so-six-change-files-design.md
plan: docs/superpowers/plans/2026-08-07-writers-emit-unquoted-yaml-title-scalars.md
results: docs/results/2026-08-07-writers-emit-unquoted-yaml-title-scalars-so-six-change-files-results.md
trivial: false
auto_groomable: true
branch: feat/writers-emit-unquoted-yaml-title-scalars-so-six-change-files
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/172
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-writers-emit-unquoted-yaml-title-scalars-so-six-change-files-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-writers-emit-unquoted-yaml-title-scalars-so-six-change-files-design.md) |
| Plan | [2026-08-07-writers-emit-unquoted-yaml-title-scalars.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-07-writers-emit-unquoted-yaml-title-scalars.md) |
| Results | [2026-08-07-writers-emit-unquoted-yaml-title-scalars-so-six-change-files-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-07-writers-emit-unquoted-yaml-title-scalars-so-six-change-files-results.md) |
| ADRs | [ADR-0071](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0071-writer-guarantees-yaml-validity-by-construction.md), [ADR-0073](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0073-scalar-quote-predicate-has-no-flow-collection-exemption.md) |
<!-- docket:artifacts:end -->

## Why

Five change files on `docket` have frontmatter that a real YAML parser rejects
(`mapping values are not allowed here`). Every one is an unquoted `title:` scalar carrying a colon:

| id | where | title fragment |
|---|---|---|
| 0121 | active | `The manifest's elsewhere: check proves...` |
| 0173 | archive | `...a model ID containing / or :` (trailing colon, end of value) |
| 0211 | archive | `...stops after the build: commits on an unpushed branch...` |
| 0217 | archive | `Clear change 0202's three minor findings: dead guard...` |
| 0234 | active | `Split gate-execution.md: probe evidence should not sit...` |

(Grooming counted six. 0219 was the sixth; its title has since been reworded to
`aborted-run's Step 7 seam — a fourth git-only leg, plus GitHub enrichment for leg C`, which carries
no colon and now parses. Confirmed at reconcile — the repair set is five, and the fact that a
violation left the tree by unrelated editing rather than by any guard is itself the argument for
fixing the writer.)

**docket already knows this rule and still writes violations.** AGENTS.md carries the promoted
`yaml-scalar` finding ("Quote any hand-authored YAML scalar carrying a colon-space or a boolean
keyword. Today's grep/awk reader tolerating it is not evidence it is well-formed"), ADR-0065 records
it, and `board-checks.sh`'s `scalar-form` check (change 0191) *detects* it. The loop mints a
malformed file and then reports the violation back to itself, warn-only, forever.

The gap is that the rule is scoped to **hand-authored** scalars while the highest-volume writer is a
**script**. `mint-stub.sh`'s `set_field` writes `print key ": " val` — byte-for-byte by deliberate
design, so that a model-authored title is never corrupted by sed/awk metacharacters (its B1 comment
is explicit about this). That protection is against *shell and regex* injection. It does nothing
about *YAML syntax*: a title containing `: ` produces a file no YAML consumer can read. The write
path has no quoting step at all.

`scalar-form` also has a hole of its own, which is why the count above is five and the check reports
four: it tests for `': '` (colon-space) and for bare booleans, but **not for a colon at the end of
the value**. 0173's title ends in `/ or :` — invalid YAML, undetected.

Impact today is bounded but not zero: docket's own readers are grep/awk (`docket-frontmatter.sh`)
and tolerate it, which is exactly the false comfort AGENTS.md warns about. Any consumer that
actually parses — the GitHub mirror, an external tool, a future reader — sees a broken file, and the
failure mode is a parse abort over the whole manifest rather than one bad field.

## What changes

- **Quote at the write boundary.** `set_field` (and any sibling writer of a free-text scalar) emits
  a properly quoted YAML scalar when the value needs it. Byte-for-byte fidelity of the value must
  survive — the B1 property is not to be traded away, and quoting must handle a value that itself
  contains quotes.
- **Repair the five existing files**, active and archive alike — and republish the three archived
  ones onto `main`, where `terminal_publish: true` already put the broken copies. An archived file
  is immutable as a *record*, but a syntactically broken one is not a record anyone can read.
- **Close the `scalar-form` gap**: a value ending in `:` is as invalid as one containing `': '` —
  along with a value containing ` #` or opening with a YAML indicator, which truncate *silently*.
- **Guard it** with one predicate shared by the writer and the checker, so the two can never drift
  again: hermetic unit + round-trip tests over the predicate and `set_field`, with `board-checks`
  over the live metadata branch as the backstop. No YAML library is introduced anywhere.

## Out of scope

- The `field-domain` finding on change 0189 (`title contains '|'`, which injects board columns).
  Adjacent and also a title-sanitization issue, but a different failure — a rendering corruption,
  not a parse error — with its own existing check.
- Rewording any title for style. This is a syntax fix; titles keep their words.
- Introducing a YAML library dependency into the runtime read path. The readers stay grep/awk;
  only the *writer* and the *guard* are in question.

## Open questions

Settled by the linked spec (autonomous groom; every decision and its rejected alternatives are in
the spec's `## Assumptions`):

- **Quote-vs-reject:** `mint-stub` quotes **silently**. It runs unattended via auto-capture, where a
  refusal turns a valid capture into an aborted run.
- **Quoting form:** **single-quoted**, with `''` doubling done in bash before the ENVIRON export —
  and `_docket_unwrap_quotes` gains the exact inverse leg, without which every apostrophe-bearing
  title would render as `manifest''s` in `BOARD.md`.
- **Where the guard lives:** both, by construction. One shared needs-quoting predicate in
  `lib/docket-frontmatter.sh` serves the writer and `board-checks`'s `scalar-form`; the hermetic
  suite covers the predicate and the `set_field` round-trip, while the live-tree backstop stays
  `board-checks`. No YAML-library dependency is added — a parser guard gated on an optional install
  goes silently vacuous, which is the failure class this change exists to fix.
- **Other write paths:** only `mint-stub`'s `set_field` writes free text; the `archive-change` and
  `reclaim-claims` copies write generated constants. Model-authored frontmatter edits stay covered
  by the AGENTS.md rule, whose wording is widened from "hand-authored" to any writer.

Scope grew in two places during design: the predicate also covers a value containing ` #` or opening
with a YAML indicator character (silent truncation, not just a loud abort), and the three archived
repairs are **republished onto `main`**, where `terminal_publish: true` already put the broken
copies.

## Reconcile log

### 2026-08-07

Re-read the change, its spec (including the **2026-08-07 always-quote revision / ADR-0071**),
`related: [234]`, and the current code on `origin/main` (035e8eba). **The design holds; one factual
correction to the repair set.**

**Repair set is five, not six.** 0219's title was reworded since grooming — it now reads
`aborted-run's Step 7 seam — a fourth git-only leg, plus GitHub enrichment for leg C`, carrying no
colon, so it parses. Verified in the metadata tree. Remaining targets, all confirmed still broken
verbatim as the spec describes them:

| id | tree | shape |
|---|---|---|
| 0121 | `active/` | colon-space |
| 0234 | `active/` | colon-space |
| 0173 | `archive/` | **trailing colon** (the leg `scalar-form` is missing) |
| 0211 | `archive/` | colon-space |
| 0217 | `archive/` | colon-space |

The three archived copies were re-verified on `origin/main` — all three still carry the broken line,
so §3's republish leg is still required and still has work to do.

Code anchors re-verified against current `main`; every one still reads as the spec describes:

- `scripts/mint-stub.sh` — `set_field` at line 138 still writes `print key ": " val` with no quoting
  step; the seven `set_field` calls sit at lines 202–208 with `title` at 204, and `title` is still
  the only free-text prose among them (`slug` slugified, `id`/`created`/`updated`/`type` generated,
  `discovered_from` a `[…]` list). The control-character gate is at line 77 as cited.
- `scripts/lib/docket-frontmatter.sh` — `_docket_unwrap_quotes` (line 44) still strips exactly one
  matched pair and does **no** unescaping, so the `''`-undoubling leg ADR-0071 makes load-bearing is
  genuinely absent today. `field_raw`/`fm_field_raw` still preserve quotes for the checker.
- `scripts/board-checks.sh` — `scalar_form_check` at line 336 carries the skip leg, the `': '` leg,
  and the boolean leg, and **no trailing-colon leg**; it is called for `title` and `blocked_by` at
  lines 351–352. The predicate does not exist yet in the library.
- `AGENTS.md:29` and `docs/changes/learnings/yaml-scalar.md` both still say **"hand-authored"** — §4's
  widening is unstarted.

Scope unchanged otherwise. `depends_on: []`; ADR-0071 is present and `Accepted` on `docket`.

**Concurrency note for the build:** 0234 is `in-progress` under another autonomous run right now,
and its change file is one of the two `active/` repair targets. Its title line is not something that
run edits, but the repair must re-read the file immediately before writing and stage only that path.

Auto-capture: the discovery pass surfaced no adjacent work clearing the six admission gates. The
`|`-in-title sibling is already change 0189, and A7's re-confirmation of the `archive-change.sh` /
`reclaim-claims.sh` `set_field` copies is in-scope verification work for this branch, not a separate
change. Nothing minted.
