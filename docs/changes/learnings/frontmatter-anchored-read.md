---
slug: frontmatter-anchored-read
hook: "A first-match-anywhere field read is safe only for keys that are ALWAYS present — for an optional key it falls through into the body and returns prose."
topics: [yaml, frontmatter, reads]
changes: [127]
created: 2026-07-23
updated: 2026-07-23
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
