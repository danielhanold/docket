---
slug: fix-reintroduces-its-own-defect-class
hook: "New code added by a change that fixes a defect class is the likeliest place for that class to reappear — audit the change's OWN additions against its thesis before review, and check the twin it did not touch."
topics: [review, refactoring, contracts]
changes: [135, 173, 113, 212, 220, 228, 259, 254]
created: 2026-07-28
updated: 2026-08-08
promotion_state: candidate
promoted_to:
---

## Apply
A change whose whole purpose is "stop emitting X in shape A when the consumer expects shape B"
usually also *adds* a new producer or a new adapter. That new code was written by the same hands,
against the same mental model, in the same session — and it is routinely the one place the fix does
not reach, because the audit is aimed at the code being repaired, not at the code being introduced.

Two moves, both cheap:

- **Run the change's own thesis over its diff.** Take the one-sentence defect statement and apply it
  literally to every file the branch adds. If the thesis is "a sentinel value must be normalized
  before it reaches the harness," grep the additions for that sentinel.
- **Find the twin.** A copy-paste sibling (the adapter for the *other* harness, the second emitter,
  the upstream helper both call) almost always carries the same gap. Fix it, or mint it explicitly
  as a follow-up with the root cause named — never leave it as an unrecorded observation.

Watch especially for a **defensive warning that keys on the wrong condition**: a guard testing
`-z "$VAR"` never fires for a sentinel like `inherit`, which is non-empty and equally invalid, so
the code both mis-handles the value and suppresses its own diagnostic.

Related: [[escape-ere-metacharacters-in-key]] (the un-fixed twin of a duplicated helper),
[[correspondence-guard-runs-one-way]], [[verify-the-claim]].

## War story
- 2026-07-28 (#135, PR #127) — The change existed because docket emitted Claude-shaped wrappers for
  Cursor, silently pinning one thing while the harness honored another. The branch's **new**
  `scripts/runners/cursor.sh` adapter did not normalize docket's `inherit` model sentinel, though
  `emit_cursor_md` in the same branch does. `emit_shim` bakes `--model $2` whenever the resolved
  override is non-empty, so `runner: cursor` plus an explicit `model: inherit` would have sent
  `--model inherit[effort=xhigh]` to `cursor-agent` — a non-existent model ID handed to a CLI with a
  documented compatible-model fallback, the effort pin destroyed with it. The adapter's own WARN was
  unreachable because it keyed on `-z "$MODEL"`. Caught at final whole-branch review, not by the
  suite; fixed by normalizing the sentinel before the flag mapping so it routes into the existing
  correct WARN. The Codex adapter's identical twin, rooted upstream in `emit_shim`, was still live
  and became **#0140**.
- 2026-07-31 (#173, PR #142 — merged) — The change existed to widen three over-narrow value classes
  that silently truncated config values. Its **own new** block-mapping reader in
  `scripts/runner-dispatch.sh` then over-captured in the opposite direction: for a comment-only line
  (`sandbox:   # TODO decide later`) it exported the comment text as the value, because the
  capture's trailing `[[:space:]]*` is greedy and eats the space the strip-comment step keys on, so
  that strip could never fire. `scripts/runners/codex.sh` would have run
  `codex exec --sandbox '# TODO decide later'`, or `die`d outright on a commented-out `network:` —
  turning a cosmetic comment into a failed dispatch, the exact harm the reader's deliberately
  tolerant posture exists to prevent. Caught at whole-branch review, not by the suite. Alongside it,
  the change's **new gate** was self-inconsistent: it exempted one already-warned-and-dropped config
  shape while hard-failing two others, so a quoted value in dead config blocked *all* wrapper
  generation. Both fixed in `ff9f0962`. The generalizable addition to this finding: when the defect
  class is "the pattern is the wrong width," the replacement pattern is wrong in the *other*
  direction just as easily — widening is not a safe direction, it is a second chance to be wrong.
- 2026-08-03 (#113, PR #154 — merged) — **The purest instance yet: the change's thesis, applied to
  the change's own additions, is the review finding.** 0113 exists because a first-match-anywhere
  frontmatter read falls through into the body for an *optional* key (the
  [[frontmatter-anchored-read]] class). Its new `aborted-run` check makes four such optional reads
  — `plan`, `results`, `branch`, `claimed_at` — and all four correctly use the anchored `fm_field`.
  But the guard proving the anchoring is load-bearing was written for **one** of them: swapping
  `fm_field "$f" results` back to the unanchored `field` reproduces the original silent false
  negative with the suite still fully green. Three of the four legs are correct by authorship and
  unprotected by any assert — indistinguishable, from the suite's point of view, from three that
  are wrong.
  What this adds to the finding: the audit move here is not "did the fix reach the new code" (it
  did, on all four legs) but **"does the evidence reach the new code"** — a change that ships a
  defect class's antidote must mutation-test *every* site it applied it to, because a single proven
  leg reads in a review as the property being established while it only establishes one instance of
  it. A guard over an N-site invariant that exercises one site is the same shape as
  [[correspondence-guard-runs-one-way]], one layer in. Captured as **#0202**.
- 2026-08-05 (#212, PR #161) — **The defect class reappeared inside the sentence written to abolish
  it.** 0212 exists because a terminal stop in a role-skill body ("your turn ends here") is read by
  an *inlining* caller as ending the whole run. The fix was one scoping clause, swept across seven
  stop sites. The first version conditioned on the reader's **employment status**: "loaded inline
  into a caller's context … dispatched as a subagent, your turn ends here." Both antecedents are
  simultaneously true of a `docket-implement-next` instance — it *is* a forked subagent and it *is*
  an inlining caller — so the clause reproduces exactly the ambiguous-antecedent read it was written
  to remove, and the continue branch was third-person while the abort branch was second-person, so
  the branch that survives a careless pronoun read is the aborting one. Landed instead:
  discriminate on **provenance** ("if you invoked this skill yourself"), with the second person on
  the **continue** branch. Recorded as ADR-0069 so the rule binds future bodies, not just these
  seven.
  What this adds: when the defect class is *ambiguous reference*, the antidote is prose, and prose
  is where the class lives — so the fix is written in the defect's own medium and inherits its
  failure mode for free. The audit move is mechanical and cheap: enumerate the readers of the new
  sentence, and check the discriminator is a property on which they actually **differ**. Employment
  status was not one; provenance was.
- 2026-08-07 (#220, PR #164) — a change whose whole purpose was clearing **false claims made in
  source comments and diagnostics** shipped a new one in its own additions. The abort message added
  to `emit_wrapper` said "No wrappers were written." All three call sites invoke it under an output
  redirection, and bash truncates the target before the function body runs — so the abort leaves a
  zero-length wrapper (the precise artifact 0207's atomicity design exists to prevent), and past the
  first loop iteration earlier wrappers are already on disk. Caught at whole-branch review, not by
  the suite. In the same branch, two `AGENTS.md` pipefail violations arrived from plan-supplied test
  code in a change that elsewhere cited that same rule to *reject* plan-supplied test code: one
  worker refused the plan's `grep … | grep -qF …` spelling while another had already accepted
  `grep -F … | head -n1` from the same plan.
  What this adds: when the defect class is a false statement rather than a false value, the audit
  target is every string the branch **writes for a human to read** — comment, diagnostic, commit
  message — checked against what the code actually does at that point. And a repo rule honored by
  one worker does not propagate to the block a different worker wrote: fan-out makes intra-branch
  self-contradiction the default, not the exception. Related:
  [[plan-supplied-test-code-is-unverified]], [[pipefail]].
- 2026-08-07 (#228, PR #167) — the change fixed an **uninspected exit status** (a `for` loop with
  no failure accumulator, so the block's status was the last test's). Its own new guard then
  reproduced that exact class: the assert checked only the failure direction (`-ne 0`) and threw
  away the status of its one all-green run, so a regression making the block *always* non-zero
  would have kept all 6123 asserts green while turning every green suite into a red gate at both
  consumers. Caught at whole-branch review; the fix's mutation (`suite_status=1` as the
  initializer) reddens the new success-direction assert and nothing else.
  What this adds: the audit target generalizes past *code* to the change's **guards** — when the
  defect class is "a status nobody looked at," the new assert that looks at only one direction is
  the same defect wearing a test's clothes. Both directions, or it is decoration
  ([[guards-are-code]], [[assert-pins-outcome-not-mechanism]]).
- 2026-08-08 (#259, PR #177 — merged) — The twin the change did not touch had the identical hole,
  undisclosed. `render-board.sh`'s archive feeder got M4 to guard a "future control-character path";
  the **ACTIVE** feeder (`SECTION`) uses the same `id<TAB>file` join, the same two-variable split,
  and had **four consumers and no guard at all** — so a TAB in an active filename rendered a raw TAB
  into `BOARD.md` at exit 0. The hazard M4 was written against was already reachable one function
  away. Found at whole-branch review, not by the change that wrote M4. The fix chose an **upfront
  rejection class (M5)** over a per-consumer guard precisely because guarding one of four sites
  leaves three open plus any fifth added later ([[enumerated-floor]]).
- 2026-08-08 (#254, PR #180 — merged) — **A guard against "a default that silently defeats a guard"
  was itself silently defeated — by a path.** The change hardened bare `mv` (which self-answers its
  prompt and exits 0, making every `|| die` unreachable). Its new `tests/test_bsd_tool_defaults.sh`
  carried a `git mv` carve-out applied to the whole `path:lineno:content` match string, and
  `[^|]*` spans both `/` and `:` — so any file whose **path** contained `git` was exempt.
  `scripts/lib/docket-gitignore-block.sh` is in scope and matched; worse, the entire mv guard
  collapsed for any checkout living under `~/git/…` or `~/github/…`. A second finding in the same
  branch: the predicate keyed on `mv "`, so `mv -i "$t" "$f"` — the precise interactive behavior the
  change exists to prevent — was invisible, and the downstream `mv -f` filter was dead code.
  What this adds: when the fix ships a **guard whose input is a joined string**, the carve-out must
  be anchored to the field it means (the content), not applied to the join — the other fields are
  attacker-controlled in the mundane sense that a developer chooses their checkout path. And the
  predicate must be keyed on the **command**, not on an incidental token of one common spelling.
  Related: [[guards-are-code]], [[agent-executed-markdown-is-code]] (the third finding in the same
  review — the sweep's own missed surface).
