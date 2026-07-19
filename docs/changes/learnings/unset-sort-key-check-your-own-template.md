---
slug: unset-sort-key-check-your-own-template
hook: "Decide a sort key's unset case explicitly — then check whether your own template makes unset the COMMON case rather than the rare one."
topics: [design, sorting, templates]
changes: [94]
created: 2026-07-19
updated: 2026-07-19
promotion_state: retained
promoted_to:
---

## Apply
A spec that fixes an ordering as `a → b → c` has usually specified only the case where every key is
present. The unset case then falls to whatever the sort tool does by default — and for text sorts
the default is that **empty sorts before everything**, so the record with the *missing* key silently
preempts the whole band it should have trailed. Decide the unset case deliberately, and cover
**malformed** alongside unset: a placeholder left in place is neither empty nor valid, and its first
character decides where it lands (`#` collates below every digit).

Then ask the question that turns this from theoretical to urgent: **does anything you ship make the
unset case the common one?** A template that emits the field with a comment instead of a value, a
form that defaults it blank, a migration that backfills nothing — any of these mean the "edge" case
is what most records actually look like on day one. Check your own template before deciding the
trigger is rare.

When the same ordering is implemented in more than one place, write the resolved rule into the
shared definition rather than only into the code — otherwise the model-executed rankers and the
script-executed ranker drift apart on exactly this unspecified case.

## War story
- 2026-07-19 (#94, PR #108) — The selection order was specified as `priority → created → id` with
  nothing said about a missing `created:`. Empty sorts before every date, so an unstamped change
  preempted its entire priority band. This was **reachable, not theoretical**:
  `skills/docket-new-change/change-template.md` ships `created:                  # YYYY-MM-DD (UTC)`
  and the frontmatter reader returns that literal comment as the value — so under the naive rule
  *every freshly created change* would head its band until a human stamped it. Resolved as: unset,
  empty, or malformed sorts **last**. The opposite convention (unknown age = oldest) was considered
  and declined for exactly that reason. The rule was mechanized in `render-board.sh`, documented in
  `scripts/render-board.md`, covered for both the unset and the template-placeholder case in
  `tests/test_render_board.sh`, and written into the convention's *Build-readiness & selection*
  shared definition so the in-model rankers rank the same way. An unset or unrecognized `priority`
  was settled in the same pass: treat as `medium`, matching the documented default.
