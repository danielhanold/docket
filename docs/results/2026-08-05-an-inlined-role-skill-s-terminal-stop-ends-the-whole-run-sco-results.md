<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0212 — An inlined role skill's terminal stop ends the whole run — scope docket-build's stop and enforce the run disposition](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0212-an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco.md)**
<!-- docket:backlink:end -->

# An inlined role skill's terminal stop ends the whole run — results

Change: #0212 · Branch: feat/an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco · PR: https://github.com/danielhanold/docket/pull/161 · Plan: docs/superpowers/plans/2026-08-05-inlined-role-terminal-stop-scoping.md · ADRs: 0069

## Verify (human)

The suite proves the clause is *present, two-sided, and adjacent to its stop site*. It cannot prove
how a model resolves the pronoun, which is the property the change actually exists to secure. These
are the checks only you can make.

- [ ] Read the landed clause at `skills/docket-build/SKILL.md`'s `Then you stop — review` site and
      satisfy yourself it reads correctly to an agent that is *both* a forked subagent and an
      inlining caller — the double-antecedent case that made the first attempt a blocker.
- [ ] Confirm the same at the six other sites (`docket-review` ×2, `docket-status`,
      `docket-build-task` ×2, `docket-brainstorm`) and at `docket-build`'s `## Halting conditions`.
- [ ] Satisfy yourself that `docket-build-task`'s *preload is not self-invocation* wording really
      closes the worker-side read — that exemption guards the one boundary whose violation is
      unrecoverable, so it is the single riskiest sentence on the branch.
- [ ] Harden the shared `assert` helper against backticked descriptions (see Follow-ups) — it
      executed `git checkout .` during this build.
- [ ] Sanity-check ADR-0069's rule against how you want future docket skill bodies written; it is
      stated to bind every future instance of this class, not just this change.

## Findings

**The change's own fix reproduced its own defect class, and the review caught it.**
The first scoping clause conditioned on the reader's *employment status* — "loaded inline into a
caller's context … dispatched as a subagent, your turn ends here." Both antecedents are
simultaneously true of a `docket-implement-next` instance (it is a forked subagent *and* an inlining
caller), and the continue branch was third-person while the abort branch was second-person — so the
branch surviving a pronoun read is the aborting one. Landed instead: discriminate on **provenance**
("if you invoked this skill yourself"), with the **second person on the continue branch**. Recorded
as **ADR-0069**, which generalizes the rule past this change. This is
`fix-reintroduces-its-own-defect-class` landing exactly where that finding predicts.

**Routing `premium` at Task 5 paid for itself.** The plan handed the worker a finished guard script;
it was not an oracle and would have failed twice against the landed files. (1) The SITES anchor
`Then you stop — review is not yours.` matched nothing, because Task 1's compression wrapped that
sentence across lines. (2) `clause_near` was line-literal, so `docket-status`'s clause — wrapped
between `dispatched as a` and `subagent,` — read as absent. Both found by execution, not by reading.
`plan-supplied-test-code-is-unverified`, confirmed.

**All eleven review findings were fixed on this branch** at the human's instruction — one commit per
`important`, one combined commit for the minors. Full text is in the PR body. The five `important`
ones and their fixes:

1. `docket-build-task`'s prohibition exemption was readable by the profile worker itself, because
   that body reaches the worker by wrapper preload — potentially voiding the one metadata boundary
   whose violation is unrecoverable. Both clauses now state affirmatively that **preload is not
   self-invocation**, with a new per-site guard assert (`7ab48675`).
2. The `docket-brainstorm` no-hazard verdict rested on a claim the prose did not support: its stop
   names the owner of *planning*, not of its caller's next step, while `docket-new-change` has three
   steps after it. Now a real clause plus a SITES row (`04028d04`).
3. The `docket-adr` no-hazard assert matched `docket-review`'s third-person vocabulary, so it could
   not fire. Rebuilt on two shape-based matchers and mutation-proved against three injected
   constructs the old regex scored `0` on (`ed979ae6`).
4. Commit `8c971fa4` had reworded `docket-build-task`'s metadata bullet to `The controller owns
   that.`, deleting the only correct owner — `docket-build` holds no metadata authority.
   `docket-implement-next` restored (`03fac7b9`).
5. `docket-build`'s `## Halting conditions` stop had a `role-scoped` *label* rather than the clause
   and no guard row, leaving the file's second terminal stop unenforced — it returns `halted`, which
   collides with a run disposition of the same spelling. Full clause plus a SITES row (`5e5f4b7b`).

**One finding was itself partly wrong, and the fix worker caught it.** Finding 2 asserted
`docket-brainstorm` is the only swept body without `context: fork`; in fact `docket-build`,
`docket-review`, and `docket-build-task` also lack it. The load-bearing half survives — it is the
only body whose *sole* invocation path is inline loading — so the fix stands on the corrected basis.
`verify-the-claim` applies to review findings too, not just to specs.

**Two guards in `tests/test_docket_build.sh` are phrase-and-line-anchored** and reddened purely from
reflow during Task 3/4. They were repaired at the source in `8c971fa4`, but any future reflow of
those two sentences will redden CI the same way.

## Follow-ups

- **`assert` executes backticks in its description — a live latent defect, not fixed here.**
  `tests/test_inline_role_stop_scoping.sh`'s `assert` helper interpolates `$1` into a double-quoted
  string, so a backtick in an anchor is command-substituted. A fix worker hit this for real: an
  anchor containing a backticked `git checkout .` caused the test run to **execute** it and revert
  the worker's own uncommitted edits. Mitigated locally by forbidding backticks in anchors (stated
  in the SITES comment), but the `assert` idiom is shared across sibling test files and should be
  hardened at the source — `printf '%s'` on the description rather than interpolation.
- **The findings were fixed in-branch rather than minted as stubs**, matching change **0218** ("fix
  review findings in branch instead of minting a stub for them").
- **Changes 0203 and 0211 both collide with this one.** 0203 edits the same *Terminal disposition*
  section and the same `tests/test_skill_size_budgets.sh` row for
  `skills/docket-implement-next/SKILL.md`; 0211 is the deterministic `aborted-run` half. A budget row
  is a **semantic** conflict even when git merges the line cleanly — whichever lands second must
  re-measure the merged file and re-derive the row from the post-rebase actual, not keep the number
  it computed pre-rebase.
- **`skills/docket-implement-next/SKILL.md` had zero margin** on both axes after Task 4 (145/3800
  against a `145 3800` row). Raised to `150 3850` in Task 6 rather than shipping the near-zero
  headroom the budget file's own comment block warns against.
- **`docket-build-task`'s `## Scope` site was re-anchored**, not widened — `WINDOW` stays 6 and the
  site now has 2 lines of slack. The neighbouring `Return exactly one of three outcomes` site still
  has only 1 line of slack and is the next one that will redden on a reflow.
