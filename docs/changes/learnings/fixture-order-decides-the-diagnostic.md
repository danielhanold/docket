---
slug: fixture-order-decides-the-diagnostic
hook: "Under set -u a fixture that dereferences a value must run AFTER the assert that the value is emitted — otherwise the missing export kills the suite inside the harness and the assert that exists to name it never runs."
topics: [testing, errexit, fixtures]
changes: [218, 220]
created: 2026-08-06
updated: 2026-08-07
promotion_state: candidate
promoted_to:
---

## Apply
Order a test file so shape/existence asserts precede any fixture that dereferences what they
check. A deref under `set -u` aborts inside `assert()`'s own eval, so the diagnostic that would
have named the cause is one of the asserts that never ran. Prove a reorder is a reorder: keep the
`ok - ` name set byte-identical before and after, and re-letter block ids so they ascend in file
order (moving every in-repo cross-reference with them).

## War story
- 2026-08-06 (#218, PR #162) — With deref-ing fixtures first, deleting one `emit` line killed the
  run at the global-able fixture with `unbound variable`, lost 211 subsequent asserts, and the
  string "REVIEW_MIN_FIX_SEVERITY is emitted" appeared **zero** times in the output. After the
  reorder the same mutation reddens three named asserts first. The suite still aborts at the next
  deref — inherent to a missing export, not fixable by ordering — but the cause is named first.
- 2026-08-07 (#220, PR #164) — the same dependency one level out: not the order asserts run in, but
  the order the **fixture's own data** is walked. A guard proving a false-advisory line is emitted
  pinned its bad runner config to the `status` agent; the leg under test walks `docket-*.md`
  alphabetically, so `docket-adr` and fourteen others reach the advisory before the bad entry is
  ever read. The assert discriminates — but only because the fixture happens to sort that way. Move
  the bad entry onto the alphabetically-first agent and the run dies in a can't-happen assertion
  before any advisory prints, and the guard goes vacuous with nothing red. A reviewer called the
  assert unconditionally non-discriminating; that rationale was wrong and the complaint was right,
  which is the useful shape here. When a guard's subject is produced inside an ordered walk, state
  which position the fixture occupies and why — or place it where the walk reaches it **last**.
