---
slug: specified-but-unreachable
hook: "Sentinels over prose assert a claim is PRESENT, never that it is REACHABLE — where a contract has a producer and a consumer, anchor one assert on the producer."
topics: [testing, sentinels, review]
changes: [87, 94, 203, 220, 226, 259, 271, 277]
created: 2026-07-19
updated: 2026-08-11
promotion_state: candidate
promoted_to:
---

## Apply
A contract can be fully specified — semantics, clearing rule, board cell, convention entry — and
still ship **inert**, because nothing ever writes it. Every individual sentinel can be a valid,
mutation-provable guard and the *set* still miss this: they all anchor on the **definition**, and a
definition is present whether or not any procedural path produces it. Consumer-side asserts are the
trap, because they pass identically in both worlds — "selection SKIPS a marked change" is green
whether or not anything can ever mark one.

So: when a feature has a **producer** (the step that writes the artifact) and a **consumer** (the
step that reads it), audit the sentinel set for **producer coverage** specifically, and anchor at
least one assert on the paragraph that performs the write — not on the section that defines what
the write means. Ask of any prose deliverable: *which numbered step, in which procedure, emits
this?* If the answer is only "the section that describes it," the feature is decoration.

## War story
- 2026-07-19 (#87, PR #103) — The `## Finalize blocked` marker was specified end to end and never
  written. The gate Flow didn't write it, the abort-and-report set didn't, and *Where the reason
  surfaces* enumerated exactly what happens on an abort (relay in-context, comment on the PR) and
  stopped there. Every marker sentinel passed on the definition alone; the whole-branch review
  caught it, not the suite. Fixed by wiring the write into the surfacing step and adding a sentinel
  anchored on **that paragraph** — the pre-existing consumer assert ("selection SKIPS a marked
  change") passes whether or not anything writes the marker, which is exactly how the gap survived
  to review.
- 2026-07-19 (#94, PR #108) — The same trap one layer down, in the *range expression* of a producer
  assert. A sentinel scoped itself with `awk "/^main\(\)/,/docket_preflight/"` to prove that
  `--digest-only` short-circuits **before** the preflight call. The range closed on the explanatory
  **comment** that merely contains the string `docket_preflight`, so the window never reached the
  code it was meant to cover — the assert was anchored on the producer by name and still never read
  it. It presented as a false BLOCKED (the implementer correctly stopped rather than proceeding past
  an apparently-regressed producer assert). Fixed by replacing the text range with a line-number
  **order** comparison anchored on the executable line `docket_preflight "$SCRIPTS_DIR"`. Lesson:
  a sentinel that names the producer is not yet a sentinel that *reaches* it — when the anchor
  string also occurs in prose, the region is bounded by the comment, not the code.
- 2026-08-06 (#203, PR #163) — the unreachability was in the **contract prose itself**, not a
  sentinel. A new `### Step postconditions` table made each row cumulative and gave row 6 a
  `head_sha == HEAD` conjunct; because the Step-6.5 results commit moves HEAD after the evidence is
  minted, that row could never hold at Step 7 — and Step 7's row is the sole licence for the
  `advanced` disposition. The skill's own `references/edge-paths.md` already declared that
  staleness EXPECTED, so the new table contradicted a rule in the same file, and the defect would
  have fired on the very run that shipped it (this change wrote a results file). The reviewer
  proposed scoping or exempting row 6; the fix found row 5 carried the identical defect and
  qualified the governing sentence once instead. Lesson: a specification with a *satisfaction*
  condition needs the same reachability question a sentinel does — walk the real path that must
  satisfy it, including the path this very change takes.
- 2026-08-07 (#220, PR #164) — the anchor pointed the wrong **direction**. An assert meant to prove
  a function's header comment states its calling contract used `within()`, whose window runs
  *forward* from the anchor — and the header sits **above** `emit_wrapper(){`. The window therefore
  covered the function body, where an unrelated `${RES_MODEL:-}` about 200 characters down
  satisfied the match: deleting the entire header comment left the assert green. Re-anchored on a
  verbatim clause of the comment itself. Lesson: a directional search helper makes "above the
  anchor" structurally unreachable, so an assert about a *header* can never be anchored on the
  declaration it heads. Mutation-test by deleting the thing the assert names, not by editing near it.
- 2026-08-07 (#226, PR #168) — The change's headline addition was an instruction telling agents to
  *actively search* for capability discoveries, backed by six categories in a reference file. Every
  test asserted the categories were present; the suite was fully green and the implementation was
  faithful to the spec. Review found the real defect: the only trigger loading that reference fired
  **after** something had already surfaced, so the six categories were read only by a reader who no
  longer needed them. The change's central instruction was dead prose. Fix: the convention's
  drill-down trigger now fires at **each mint site on arrival**. Lesson for prose deliverables
  specifically — reachability is not just "does some path write this," it is **does the path that
  reads it fire early enough to change behavior**. A reference loaded after the decision it exists
  to inform is as inert as one nothing loads at all.
- 2026-08-08 (#259, PR #177 — merged) — Reachability lost, then disclosed rather than papered over.
  Review asked for a fixture exercising M4's shifted-tuple conjuncts; the M5 fix (rejecting any
  TAB/CR-bearing path upfront) made that **structurally impossible** — M5 pre-empts the path case
  and M3 subsumes the interior-TAB status, so M4's conjuncts and the SECTION-loop `BAD` gate became
  unreachable by construction. Mutation-tested to zero failures, which is the diagnostic signature:
  a clause you can break without reddening anything. Kept deliberately as defence in depth, labelled
  as such in both the code and `scripts/render-board.md`, and the residual risk — three guard
  clauses with no assert that can go red — written into the results file as **disclosure, not
  coverage**. When a later fix strands an earlier guard, the honest options are delete it or say so
  in writing; silently retaining it as if tested is the failure ([[guards-are-code]]).
- 2026-08-09 (#271, PR #188 — merged) — **Unreachable in the most literal sense: after `exit`.**
  Change 0237's run gate lived as `GATE=0; [ "$AGENT" = "implement-next" ] && GATE=1` in the generated
  shim. Rewriting the shim to always `--launch` left those lines sitting *after both verbs' `exit`* —
  dead for every delegated run — while `runner-dispatch.md` still asserted "`implement-next` —
  unchanged." A delegated run that halted, or stopped before opening its PR, would have exited 0 at
  the adapter and been observed as `complete`: precisely the prose-level false-success 0237 exists to
  eliminate, silently restored by a refactor that moved the control flow out from under it. The
  producer/consumer audit generalizes to **control flow**, not just to who writes the artifact: when
  a rewrite changes where a script `exit`s, every guard downstream of the new exit is a candidate for
  having gone inert, and the documentation asserting it still works is the thing least likely to
  notice. `--observe` now carries the attribution snapshot captured at launch and synthesizes `3` on
  `run-halted`.
- 2026-08-11 (#277, PR #194 — merged) — **A generated recipe whose slots were right and which could
  not execute.** The shim `emit_shim` writes taught a two-call recipe for delegated dispatch: call 1
  creates the brief with `mktemp` plus a quoted heredoc, call 2 launches with
  `--brief-file <the path call 1 wrote>`. Harness Bash calls share no shell state and `mktemp`'s
  suffix is random, so in call 2's fresh shell `$BRIEF` is unset, expands empty, and the facade dies
  — on the sole taught channel, for every runner and every delegated agent. The shim sentinels were
  green throughout: they asserted the **slot's shape** (the flag is present, the placeholder looks
  right) and never that the recipe **executes to a usable value**. Caught at whole-branch review.
  Fixed by emitting the write and the launch as ONE harness call with a live `--brief-file "$BRIEF"`,
  which deletes the brief path as a model-substituted slot; recorded as **ADR-0082**. What this adds:
  for generated *instructions* the reachability question is not "does something write this" but
  "does the recipe run" — and the state a recipe silently depends on (a live shell between two steps)
  is exactly what a shape assert cannot see. Anchor at least one assert on the recipe producing a
  value, not on its placeholders. Related: [[generated-artifact-loaded-at-process-start]] — nothing
  in-session can validate the regenerated shim as the harness will read it, so the live dispatch
  stayed a human verification item.
