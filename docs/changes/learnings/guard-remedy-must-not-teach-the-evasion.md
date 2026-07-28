---
slug: guard-remedy-must-not-teach-the-evasion
hook: "A count-based guard whose failure message says 'bump the expected count' teaches the evasion — budget the coverage-granting path with its own counter."
topics: [guards, testing, invariants]
changes: [122]
created: 2026-07-28
updated: 2026-07-28
promotion_state: candidate
promoted_to:
---

## Apply
When a guard's assertion is a **count** and its failure message tells the next author how to make the
count agree, read the message as an attack: is there a way to add the very thing the guard exists to
catch that leaves only the count moving? If yes, the remedy text *is* the exploit, because it is the
cheapest path back to green.

Two halves close it, and only the second actually does:

1. Rewrite the message so the remedy leads with the substantive check ("confirm the new key carries
   its own tag") before it mentions the mechanical bump.
2. **Emit and assert a second, independent counter over the coverage-*granting* path** — the
   inheritance rule, the adjacency window, the default arm — so laundering the primary count still
   reddens. Confirm by mutation that the new assert fails *independently of* the primary count.

The general shape: an implicit-coverage rule (inheritance, adjacency, "nearby comment counts") is a
silent grant. Make it a **budgeted, loud** event with its own number, or every future item takes the
free ride. See [[marker-scoped-guard-needs-a-population-floor]] for the sibling failure where the
guard's *unit of admission* was the hole.

## War story
- 2026-07-28 (#122, PR #131 — merged) — `.docket.example.yml`'s scope-tag guard proved every nested
  key carries a sanctioned scope tag, with an exact floor of 17 keys. Rule 4 granted coverage to a
  scalar whose comment window was genuinely empty when it sat immediately below a same-depth key —
  needed for the `changes_dir`/`adrs_dir`/`results_dir` shared-comment group. Consequence: a new
  nested key added with **no comment at all** beneath a tagged sibling inherited coverage silently,
  and the only thing that moved was `COUNT`, 17 → 18. The exact-17 floor then failed with a message
  whose remedy was "bump `expected_nested_key_count` in the same commit" — doing exactly that
  returned the suite to green **with an untagged key shipped**. The cheapest way to add a nested key
  was the one that evaded the guard.

  Closed by emitting `ADJINHERIT`, a second counter for keys covered *via* the adjacency rule,
  asserted to equal exactly 2 (`adrs_dir`, `results_dir`); the re-review confirmed it reddens on the
  mutation independently of `COUNT`, so laundering the count to 18 still fails. The reworded message
  alone would not have.

  Two provenance notes worth keeping. Three clean **task-level** reviews did not mean a clean branch
  — this defect exists only in the *interaction* between two tasks and was surfaced solely by the
  whole-branch review ([[foundational-test-discipline]]). And the whole guard program was executed
  against the real file during the plan pass, so every expected value was measured, not predicted;
  all three mutation outputs matched on the implementer's first run
  ([[plan-supplied-test-code-is-unverified]]).
