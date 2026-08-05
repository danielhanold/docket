<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0212 — An inlined role skill's terminal stop ends the whole run — scope docket-build's stop and enforce the run disposition](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0212-an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco.md)**
<!-- docket:backlink:end -->

# An inlined role skill's terminal stop ends the whole run — results

Change: #0212 · Branch: feat/an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco · PR: (see change `pr:`) · Plan: docs/superpowers/plans/2026-08-05-inlined-role-terminal-stop-scoping.md · ADRs: 0069

## Verify (human)

The suite proves the clause is *present, two-sided, and adjacent to its stop site*. It cannot prove
how a model resolves the pronoun, which is the property the change actually exists to secure. These
are the checks only you can make.

- [ ] Read the landed clause at `skills/docket-build/SKILL.md`'s `Then you stop — review` site and
      satisfy yourself it reads correctly to an agent that is *both* a forked subagent and an
      inlining caller — the double-antecedent case that made the first attempt a blocker.
- [ ] Confirm the same at the five other sites (`docket-review` ×2, `docket-status`, `docket-build-task` ×2).
- [ ] Decide the ten unfixed review findings below (five `important`, five `minor`) — fix in-branch
      before merge, or merge and let them ride.
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

**Ten review findings were left unfixed by contract** (the skill auto-fixes blockers only). Full text
is in the PR body. The five `important` ones, in the reviewer's own framing:

1. `docket-build-task`'s prohibition exemption can be read by the profile worker itself, because that
   body reaches the worker by wrapper preload — potentially voiding the one metadata boundary whose
   violation is unrecoverable.
2. The `docket-brainstorm` no-hazard verdict rests on a claim the prose does not support: its stop
   names the owner of *planning*, not the owner of its caller's next step, and it is the one swept
   body with no `context: fork` — always inlined.
3. `tests/test_inline_role_stop_scoping.sh`'s `docket-adr` no-hazard assert matches `docket-review`'s
   third-person vocabulary, not `docket-adr`'s imperative forms, so it cannot fire.
4. Commit `8c971fa4` reworded `docket-build-task`'s metadata bullet to `The controller owns that.`,
   deleting the only correct owner — `docket-build` holds no metadata authority; `docket-implement-next` does.
5. `docket-build`'s `## Halting conditions` stop got a `role-scoped` *label* rather than the clause,
   and has no guard row — so the file's second and arguably more dangerous terminal stop is
   unenforced. (It returns `halted`, which collides with a run disposition of the same spelling.)

**Two guards in `tests/test_docket_build.sh` are phrase-and-line-anchored** and reddened purely from
reflow during Task 3/4. They were repaired at the source in `8c971fa4`, but any future reflow of
those two sentences will redden CI the same way.

## Follow-ups

- **The ten findings were deliberately not minted as stubs.** Change **0218** ("fix review findings
  in branch instead of minting a stub for them") is open and records exactly that preference, so
  minting here would have cut against it. They live in the PR body and in this file instead.
- **Changes 0203 and 0211 both collide with this one.** 0203 edits the same *Terminal disposition*
  section and the same `tests/test_skill_size_budgets.sh` row for
  `skills/docket-implement-next/SKILL.md`; 0211 is the deterministic `aborted-run` half. A budget row
  is a **semantic** conflict even when git merges the line cleanly — whichever lands second must
  re-measure the merged file and re-derive the row from the post-rebase actual, not keep the number
  it computed pre-rebase.
- **`skills/docket-implement-next/SKILL.md` had zero margin** on both axes after Task 4 (145/3800
  against a `145 3800` row). Raised to `150 3850` in Task 6 rather than shipping the near-zero
  headroom the budget file's own comment block warns against.
- **`docket-build-task`'s `## Scope` clause sits exactly at `anchor + WINDOW`** in the new guard. One
  inserted line reddens it for a formatting reason. The fix is to move the clause or re-anchor the
  site — never to widen `WINDOW`.
