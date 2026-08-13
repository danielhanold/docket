---
slug: frontmatter-anchored-read
hook: "A first-match-anywhere field read is safe only for keys that are ALWAYS present — for an optional key it falls through into the body and returns prose."
topics: [yaml, frontmatter, reads]
changes: [127, 202, 219, 244]
created: 2026-07-23
updated: 2026-08-08
promotion_state: candidate
promoted_to:
---

## Apply
The read twin of [[frontmatter-edit-anchor]]. A helper that returns the first `^key:` match
*anywhere in the file* is correct exactly as long as the key is always present in the frontmatter —
the first match is then necessarily the frontmatter one, and the scan never reaches the body. That
premise silently expires the moment the key becomes **optional**: for a file that omits it, the scan
runs past the closing `---` and returns whatever body line happens to start with that word.

So the question is never "is this read anchored?" but **"can this key be absent?"** Every optional
manifest field is exposed — in docket's own schema that is `spec:`, `plan:`, `results:`, `branch:`,
`pr:`, `issue:`, `blocked_by:`, `type:` — and the hazard is worst in a repo whose *subject matter*
is the field names, because body prose discussing `pr:` or `type:` is not a contrived fixture, it is
the normal content of a change file.

Use a frontmatter-scoped reader (stop at the first block's closing `---`) for any key that may be
absent. Lock it with a fixture that **omits** the key while the body opens a line with it: an
unanchored read returns the prose, an anchored one returns empty. Note that the natural test — a
file that *has* the field — passes under both implementations, so the fixture must be the
absent-key one or the guard is decoration.

And when one block performs **several** anchored reads, the fixture is per-key, not per-block. A
mutation that unanchors one read while the fixture population supplies body prose for only that
same key proves exactly one of them; its mirrors can be unanchored later with the suite still
green. One absent-key fixture and one mutation arm **per read**.

## War story
- 2026-07-23 (#127, PR #123) — `field()` returned the first match anywhere in the file. Safe for
  `status:`/`id:`, which every change carries; a real bug for the newly-optional `type:`. An untyped
  change whose body opened a line with `type:` rendered that prose as its Type, and
  `backfill-change-types` then **refused the record** as already-typed — so the migration would have
  silently skipped exactly the changes it existed to fix. Caught by the backfill's own anchor
  fixture during the build. Fixed by adding `fm_field` (first frontmatter block only) and routing
  every `type:` read through it; recorded as **ADR-0057**. The residual audit of the other `field()`
  call sites — every optional key listed above — was auto-captured as **#134**, which is the tell
  that the anchoring decision belongs at the *helper*, not at each call site.
- 2026-08-05 (#202, PR #158) — The `aborted-run` check performs **four** anchored reads
  (`plan`, `results`, `branch`, `pr`) and the suite pinned one. The mutation unanchored
  `fm_field "$f" plan` alone, and the only fixtures carrying body prose opened a `plan:` line — so
  swapping `fm_field "$f" results` to `field` reproduced the exact ADR-0057 false negative with a
  green suite. Both halves had to close together: a *mirror fixture* (frontmatter omits `results:`,
  body opens a `results:` line, branch carries an unrecorded results file) and a *second mutation
  arm*. Neither alone discriminates — the fixture is inert without a mutation that reaches it, and
  the mutation is inert without a fixture whose body prose it can fall through into.
- 2026-08-07 (#219, PR #171) — **Two legs on the same branch, one honoring the rule and one not.**
  `board-checks.sh`'s new `detect_orphan_pr` read the optional `pr:` and `branch:` keys with the
  unanchored `field`, while the leg added beside it in the same change used `fm_field` correctly —
  so the two legs could disagree about the same file, in a repo whose change bodies routinely open
  lines with `pr:`. Review caught it as a blocker, and again *no existing fixture could tell the two
  implementations apart*: every orphan fixture carried both keys in frontmatter, which is the
  natural way to author one. Fixed with one absent-key fixture and one mutation arm per read, per
  the per-key rule above. The recurrence inside a change whose author knew the rule is the argument
  for the anchoring decision living in the *helper*: knowing the hazard did not prevent reaching for
  `field` at a fresh call site.
- 2026-08-08 (#244, PR #184) — **The rule got written down, and the written criterion contradicted
  its own classification.** This change consolidated the four read shapes into one selection table
  in the library header. The first draft defined "guaranteed present" as *the template ships it* —
  but the change template ships `spec:`, `plan:`, `results:`, `branch:`, `pr:`, `blocked_by:` and
  `trivial:`, every one of which the rule correctly anchors, and all seven are present in 266 of
  266 change files today. Applying the header's own test would have promoted them straight back to
  the unanchored `field()`, and the census guard's failure message actively invited it. The
  criterion is now **semantic optionality** — *can any file this call site reads legitimately omit
  the key* — with template presence demoted to one sufficient reason a key is NOT guaranteed.
  Corpus presence today is a snapshot; the question is what a hand-edited file may legitimately do
  tomorrow. Second half of the same lesson: the census guard matched on **key name alone**, so
  `field "$f" priority` in an ADR walker or `field "$f" title` in a learnings walker would have
  passed while reading a key its corpus does not carry — the precise failure the change exists to
  close. The allowlist is now keyed on **(corpus class, key)** and fails closed when a call site's
  corpus cannot be determined. Also worth carrying: both `fm_field_verbatim` adoptions
  (`blocked_by` in `docket-status.sh` and `render-board.sh`) were unpinned by anything — every
  fixture used a `#`-free value, so both sites passed identically under `fm_field` and a later
  "consistency" swap would have silently restored the ` #…` truncation the verbatim read exists to
  prevent.
