<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0203 — Define the per-step git-state postcondition docket-implement-next now names but never states](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0203-define-the-per-step-git-state-postcondition-docket-implement.md)**
<!-- docket:backlink:end -->

# Design — the per-step git-state postcondition for `docket-implement-next`

Change 0203. Autonomously groomed (docket-auto-groom, 2026-08-05) — every decision below is a
default committed without a human; the `## Assumptions` block is the audit trail.

## Problem

Change 0113's prose rider added to `docket-implement-next` §5:

> the step is not complete until its git-state postcondition holds

`git-state postcondition` occurs **exactly once** across `skills/`. No step states one, the
*Terminal disposition* section does not, and `docket-convention` does not. An agent that already
misread §5 is now told to satisfy a named condition with no referent — weaker than the enumerated
obligations around it.

**New evidence that reframes the task (2026-08-05, the 0206 run).** Steps 0–5 completed: four build
commits on `feat/…`, suite green, `plan:` recorded and pushed. Then the turn ended — branch
unpushed, no review, no PR. **Step 5's postcondition was fully satisfied at the moment the run
died.** A postcondition set that states only *what each step must have achieved* would have
certified that run as healthy.

That is the stub's core assumption contradicted by observation, and it changes the deliverable: a
per-step postcondition is a **step certificate**, never a **run certificate**, and the design must
say so out loud or it manufactures exactly the false confidence the 0206 run displayed.

## Design

### 1. One table, not six inline riders

The postconditions are stated **once**, as a table in a new `### Step postconditions` subsection
placed immediately **before** *Terminal disposition* in `skills/docket-implement-next/SKILL.md`.
Steps 2–7 are not each given a pointer sentence; only the §5 clause (which already names the term)
is repaired, by pointing at the table.

Content, stated by shape — exact wording is a build-time choice under a hard word budget:

| Step | Complete only when, read from git |
|---|---|
| 2 Claim | `status: in-progress`, `branch:`, `claimed_at:` committed **and landed** on `metadata_branch` (local tip == remote tip). |
| 3 Reconcile | `reconciled: true` + a dated `## Reconcile log` entry landed on `metadata_branch`; or, on the kill path, the change archived. |
| 4 Worktree + plan | Step 3's push SHA-confirmed (local metadata tip == remote tip) **before** the branch is cut; then the plan file **and its `docket:backlink` stamp** committed on `feat/<slug>` **and** `plan:` landed on `metadata_branch` — a two-tree conjunction, verified by reading both refs, never by the plan skill's report. |
| 5 Build | the executed plan committed on `feat/<slug>` with a green build-evidence record whose `head_sha` equals the branch HEAD. |
| 6 Review + ADRs | the evidence record still green with `head_sha` == HEAD **after** any blocker-fix commits, and every ADR the run produced landed in `adrs:` on `metadata_branch`. On a clean review this is Step 5's condition unchanged — see the residual below. |
| 7 PR + stop | the branch pushed (`refs/remotes/origin/feat/<slug>` resolves), the PR open, and `status: implemented` + `pr:` landed on `metadata_branch`; `results:` set **iff** a results file and its backlink stamp are committed on `feat/<slug>` (the Step 6.5 conjunct). |

Every cell is a condition over refs, commits, frontmatter fields, and the committed build-evidence
record. No new state, field, or status is introduced — but the claim is *no new state*, **not** "all
of it is already read by `scripts/board-checks.sh`": that script reads neither the evidence record
nor `## Reconcile log` presence, so rows 3, 5 and 6 name state no existing check consumes.

**Named residual — Step 6 is the weak row.** On a clean-review path (no ADRs, no blockers) Step 6
produces no git state of its own, so its row reduces to Step 5's and is vacuously satisfied by a run
that never invoked the reviewer. That is not fixable within this change's means: whether a reviewer
ran is not a fact about git. It is stated in the spec rather than papered over, and it is the one
row whose satisfaction is genuinely weaker than the prose it certifies. The backstop is the same one
the transition problem has — 0212's disposition obligation and 0211's leg C.

### 2. The governing sentence — a step certificate is not a run certificate

The table is prefaced by one normative sentence, and it is the part the 0206 evidence buys:

> These certify a **step**, never the run. The conditions are cumulative — each step's holds in
> addition to every earlier step's. **Once a change is claimed (Step 2), and absent a `halted`
> disposition or a Step-3 kill, the only postcondition that also completes the run is Step 7's**; a
> satisfied intermediate one is never evidence the run may end. A run that ends any other way ends
> on a disposition, not on a postcondition.

The scoping matters and two drafts got it wrong. `drained` and `contended` end before Step 2, but
`halted` is defined at `SKILL.md:109` as reachable from **Step 3** (fundamentally-invalidated
design) or any hard error, and Step 3's OBSOLETE escape hatch archives a claimed change and loops
back to Step 1, where the run may end `drained`. Both are post-claim endings with no Step 7
postcondition, so a carve-out for pre-Step-2 runs alone would contradict the *Terminal disposition*
table three lines below. The inserted sentence names both exceptions explicitly.

This composes exactly with 0212's lever 2, which obliges the run to declare one of four dispositions
and states that `advanced` is claimable **only when Step 7's postcondition holds**, *by pointing at
Step 7*. After 0203 lands, that pointer resolves to the table's Step 7 row instead of Step 7's
enumerated prose. Additive in either landing order — which is the seam 0212 already recorded.

**Why the transition out of a step gets no per-step clause.** Three shapes were weighed for making
each step's *exit* verifiable, and all three are rejected in favour of the cumulative rule:

1. A per-row "and the next step has begun" conjunct — not git-readable, and a step that has begun
   has produced nothing to read. It would be prose that cannot be checked, which is the defect 0203
   exists to remove.
2. A run-record line on `metadata_branch` naming the current step — rejected on the same two grounds
   0212 rejected it for lever 2: it duplicates the `aborted-run` ledger, and the convention's Tier-C
   precedent is explicitly "no new status, no new field."
3. A new runtime mechanism (a wrapper- or driver-side check) — rejected identically to 0212's
   options 2 and 3: the vendored driver is not docket's to change, and a wrapper has no channel to
   read a subagent's final message.

So the transition is covered at the **run** level (0212's disposition obligation) and detected
**after the fact** by the `aborted-run` ledger — leg C, once 0211 lands, is the deterministic oracle
for precisely the 0206 signature.

**Stated honestly: 0203's own contribution to the transition problem is small.** The cumulative rule
plus the explicit denial that an intermediate certificate ends a run removes the *false confidence*
a bare postcondition set would create; it does not detect or prevent the stop. Anything stronger
requires a mechanism this change has ruled out on 0212's inherited grounds. The value of stating it
here is that a reader of the table cannot come away believing a green Step 5 row means a healthy
run — which, absent the sentence, is exactly what the table would imply.

### 3. Enforceability — prose, plus a positive-presence guard

The postconditions are **not** enforced by anything other than the agent reading them. That is the
settled answer, not an omission:

- A step boundary is **not a fact about git**. No git-only checker can observe that a step ended.
  `board-checks.sh` gets close without ever getting there: its leg A is explicitly *time-free* and
  fires the instant a plan or results file is committed while `plan:`/`results:` is unset — but it
  keys on **artifact/field incoherence**, which is a fact about state, not about a step having
  ended. The signatures that leave no incoherence (0206's among them) need an idle floor precisely
  because nothing observable marks the seam.
- 0212 weighed and rejected a git-readable run record and a new runtime mechanism for the closely
  adjacent lever 2, on redundancy with the `aborted-run` ledger and the convention's Tier-C "no new
  status, no new field" precedent. Those grounds bind here unchanged, and are restated above rather
  than assumed.

The mechanical companion is therefore a **prose-presence guard**, extending
`tests/test_loop_continuation.sh` (which already asserts the four-disposition contract over this
same SKILL.md and is the file 0212 also extends): assert the `### Step postconditions` heading
exists, that the table names each of Steps 2–7, and that the cumulative/Step-7 sentence is present.

**The §5 assert must be proximity-scoped, not a file-level count.** Asserting that
`git-state postcondition` occurs more than once in the file is the 0199 co-occurrence weakness —
it proves nothing about §5. Anchor instead on the §5 sentence itself: within the Step 5 region,
assert the clause co-occurs with its pointer to the new section. Non-vacuity anchors follow
`tests/test_role_skill_self_description.sh` (the file 0212's guard also copies) rather than
`test_loop_continuation.sh`'s current style, which has one `mktemp` probe over one matcher and no
file-exists/non-empty anchor: add a file-exists + non-empty anchor and a mutation probe **per new
matcher**, per `assert-detects-removal-not-replacement` and `guards-are-code`. Copying a matcher is
not inheriting its property — probe by execution (`mirrored-guard-enforces-its-own-property`).

The `specified-but-unreachable` learning applies and is met: this contract's **producer** is §5's
repaired clause plus each step's own procedure, and the proximity-scoped assert anchors on that
producer, not only on the defining section.

### 4. Step 4 gets a row, and the uniformity *is* its special treatment

Step 4 received no 0113 rider despite being where two of four observed stops happened. It gets no
distinguished mechanism here — it gets the same row shape as every other step, which is the point:
the stub's own argument is that the check must be "per-step and uniform," and the reason Step 4 was
missed is that 0113 chose riders per-site instead of a uniform set. Its row is unusual only in being
a **two-tree conjunction** (plan file on `feat/<slug>` AND `plan:` on `metadata_branch`), and the
row states the conjunction explicitly along with the "read git, not the sub-skill's report" clause
the stub asks for.

### 5. The §5 clause is repaired, not deleted

`the step is not complete until its git-state postcondition holds` gains a referent by pointing at
the table. Deleting it was the stub's stated alternative and is rejected: the sentence is the only
producer-side occurrence of the term on the common path, and 0113's budget rationale in
`tests/test_skill_size_budgets.sh` argues in-diff that this rider must fire at the moment of action.
Deleting it would discard that argument to save a clause.

### 6. Size budget

`tests/test_skill_size_budgets.sh` row: `skills/docket-implement-next/SKILL.md 145 3800`; measured
2026-08-05 at **139 lines / 3728 words** — 6 lines and 72 words of margin. The table plus the
governing sentence will not fit. Sequence: **compress the touched region first (Step 7's enumerated
prose and §5's clause both shorten once the table exists), re-measure, then raise only if the
post-compression actual exceeds the row.** A raise is not pre-committed.

If a raise is needed it is set from the *measured* actual per the file's documented rounding rule
(lines → next multiple of 5, words → next multiple of 50; within 25 words of the actual ⇒ the
multiple after). The in-diff justification must name
`skills/docket-implement-next/references/edge-paths.md` and state why the prose cannot live there —
the same argument 0113's own entry records: edge-paths.md is read only when a rare edge is already
known to have been hit, and a postcondition table is read on the **common** path at every step
boundary, so a rule parked there is unread precisely when it must intervene.

## Sequencing and couplings

- **0113** — origin. This settles the term its rider introduced.
- **0212** — the closest coupling, and a **semantic conflict a clean git merge will not resolve**,
  on two surfaces: `skills/docket-implement-next/SKILL.md` (0212 adds the disposition obligation to
  *Terminal disposition*; 0203 adds the table immediately before it) and the
  `tests/test_skill_size_budgets.sh` row for that file (the budget file records against itself that
  0113's 4050 and 0201's 3700 were both measured pre-rebase and neither survived). Both also extend
  `tests/test_loop_continuation.sh`.
  **Recommended order: 0212 first.** 0212's pointer resolves to existing Step 7 prose today and
  degrades gracefully; 0203's table then attaches to a section whose obligation already exists,
  which is the direction 0212's own spec describes. Recorded as a **recommendation, not a
  `depends_on`** — see assumption 7.
  **Whichever lands second must re-measure the merged file and re-derive the budget row from the
  post-rebase actual**; `concurrent-edits-compose-at-rebase` covers the prose, not a derived number.
- **0211** — the deterministic oracle for the 0206 signature (`aborted-run` leg C). Disjoint files
  (`scripts/board-checks.sh` + its tests). This spec cites leg C as the after-the-fact backstop for
  the transition problem, which is a citation, not a dependency.
- **0202** — touches `tests/test_skill_size_budgets.sh` only to *verify* 0113's rationale comment
  and make no edit (its finding 5 is verify-then-no-op). It does **not** touch the
  `docket-implement-next` budget row, so the surface does not actually collide. Recorded in
  `related:` for the shared 0113 origin, not for a file collision.

## Out of scope

- Reversing ADR-0044 or re-litigating call-site pre-specification.
- Changing the `aborted-run` check or its predicates (0211).
- The run-level disposition obligation and the inlined-role stop scoping (0212).
- Any new field, status, run record, or runtime mechanism.
- Postconditions for Steps 0, 1, and 6.5 — see assumption 3.

## Assumptions

Every decision below was defaulted autonomously. Each names the alternatives weighed and why the
chosen one is the conservative default.

1. **One table the §5 clause points at, not six inline riders.** *Rejected:* stating a postcondition
   inline at each step — per-site riders are exactly the shape that skipped Step 4 in 0113, and that
   is the whole argument; no word-cost claim is made either way, since a 6-row table plus a governing
   sentence may well cost *more* than six short riders. *Rejected:* a `references/` file — a common-path
   rule parked in a rare-edge reference is unread when it must fire, the argument 0113's and 0137's
   budget entries both record. The table is placed adjacent to *Terminal disposition* because that is
   where 0212's pointer resolves.

2. **The governing sentence removes the false confidence a bare postcondition set would create — and
   that is all it does.** The stub's framing (state the condition that must hold *within* each step)
   is insufficient on its own: Step 5's held at the moment the 0206 run died. But the sentence is a
   disclaimer, not a detector; the transition is genuinely 0212's and 0211's to cover, and this spec
   says so in §2 rather than claiming more. Its scope is **a run that has claimed a change** —
   `drained`, `contended` and `halted` are legitimate complete runs that never reach Step 7, so an
   unscoped "only Step 7 completes the run" would contradict the *Terminal disposition* table three
   lines below it. *Rejected:* per-row "the next step has begun" conjuncts (not git-readable).
   *Rejected:* leaving the run-level problem wholly to 0212 (the table would then read as six
   independent green lights). The stub body's framing is updated accordingly.

3. **Rows for Steps 2–7; Steps 0, 1 and 6.5 get no row of their own.** The stub scopes the ask to
   "Steps 2 through 7" and this follows it. The honest reasons, corrected: Step 0 *does* produce git
   state (sweep archives, a Board pass) but none of it is scoped to the change being built, and Step
   1 is a pure read — a run ending in either ends on a **disposition**, which is 0212's surface, not
   a postcondition. Step 6.5 is optional, so it gets no row but is **not dropped**: its artifact is
   folded into Step 7's row as an `iff` conjunct (`results:` set exactly when the results file and
   its backlink stamp are committed on the branch), which is the coherence property that matters.
   *Rejected:* a vacuously-satisfiable Step 6.5 row. Step 6's analogous vacuity on a clean-review
   path is **not** dismissed — it is recorded as a named residual in §1, because unlike 6.5 it is a
   step that always runs.

4. **No enforcement beyond the agent reading its own prose, plus a presence guard.** *Rejected:* a
   step-level checker — a step boundary is not a fact about git. (Nor is `board-checks.sh` purely
   time-gated: its leg A is time-free and fires at the artifact/field seam. It keys on incoherence,
   not on a step ending — the distinction the second draft blurred.) The first draft argued this
   from a claim that
   `board-checks.sh` already reads every row's state; that claim is **false** (it reads neither the
   build-evidence record nor `## Reconcile log` presence) and has been removed rather than leaned on.
   *Rejected:* a git-readable run record — 0212 rejected the identical option for lever 2 on
   redundancy with the `aborted-run` ledger and the Tier-C "no new status, no new field" precedent,
   and both grounds hold verbatim here. *Rejected:* a negative vocabulary guard, per the
   role-self-description guard's own documented escape-by-paraphrase weakness.

5. **The guard extends `tests/test_loop_continuation.sh` rather than adding a file — but borrows its
   anchors from `tests/test_role_skill_self_description.sh`.** The home choice is right: that file
   already owns the terminal-contract prose asserts over this same SKILL.md and is where 0212's
   lever-2 asserts go, so the two changes' asserts land side by side as one contract. Its *style* is
   not a model to copy — it carries one `mktemp` probe over one matcher and no file-exists/non-empty
   anchor — so the new asserts take the fuller anchor set from the role-self-description guard.
   The §5 assert is **proximity-scoped**; a file-level "occurs more than once" count would be the
   0199 co-occurrence weakness and would not anchor on §5 at all, despite §3's claim that it does.
   *Rejected:* a new test file (splits one contract across two homes).
   `restatement-accumulates-its-own-guards` applies to the compression pass: grep the suite for any
   Step 7 prose the compression removes before removing it.

6. **Step 4 gets a row like every other step, not a distinguished mechanism — and the row is stated
   at full strength.** *Rejected:* a Step 4 rider in 0113's style — that per-site shape is why Step 4
   was missed. *Rejected:* the first draft's row, which named only the plan-file/`plan:` conjunction
   and was therefore **weaker than the prose it certifies** (the stub's own "worse than no rule"):
   the step also requires the opening SHA-compare of Step 3's push and the plan's
   `render-artifact-backlink` stamp committed on `feat/<slug>` — separable bookkeeping, exactly the
   class the stub flags (the SHA-compare is `SKILL.md:64`, the backlink stamp `SKILL.md:70`). Both
   are now in the row.

7. **`related: [113, 202, 211, 212]`, `depends_on:` empty.** *Rejected:* `depends_on: [212]` — a hard
   `depends_on` is a readiness gate satisfied only when 0212 reaches `done`, and 0212 is an
   unstarted `medium`-priority `proposed` change, so a hard gate would park 0203 indefinitely for an
   ordering preference that degrades gracefully in both directions (0212's own spec says so). The
   preference is recorded as a written recommendation plus a mandatory post-rebase re-measure of the
   budget row, which is the same remedy 0212 chose from its side. Forward link only — the
   reciprocals on 0113/0202/0211/0212 are not written. Dependency state at groom time: none
   unsatisfied. 0202 is included in `related:` for the shared 0113 origin even though the budget-row
   surface does **not** in fact collide (0202's finding 5 is verify-then-no-op) — recorded so a
   later reader does not re-derive the collision claim. One residual: 0202's finding-5 verification
   *reads* the same 0113 rationale block a 0203 budget raise would append to. Still a no-op on both
   sides (0202 makes no edit), so no conflict — recorded so the re-measure obligation covers it.

8. **Compress first, raise only what still exceeds.** *Rejected:* pre-committing a budget raise (the
   compression of Step 7's enumerated prose, which the table now states, may absorb the addition).
   *Rejected:* behaviour-neutral slimming of unrelated prose as the primary lever — it couples a fix
   to a refactor, and `size-target-is-direction` records that the number is a direction, not a gate.

9. **Priority stays `medium`, type stays `docs`.** *Rejected:* raising to `high` on the strength of
   the 0206 incident — 0211 (detection) and 0212 (prevention at source) are the halves that act on
   that incident; 0203 is the definitional half. Priority is a human's call over backlog composition;
   an autonomous groom does not raise it.

10. **Every row is verified by reading git, never by a sub-skill's report.** Stated once as a
    property of the whole table rather than repeated per row (word budget), and stated explicitly in
    Step 4's row because the stub names it there. `capability-absence-needs-a-failed-attempt` and
    the convention's "a caller must not read a bare `completed` as proof the child finished" are the
    same rule from two directions. **Tension named:** Step 5's row rests on the build-evidence
    record, which *is* a sub-skill's report — committed, and made git-checkable only by the
    `head_sha == HEAD` conjunct. That conjunct is load-bearing, not decorative, and must survive any
    compression of the row.

11. **The table is a document asserting facts about a procedure, so the build re-verifies it.**
    `verify-the-claim`. Re-check at build time, against the then-current files: the measured
    139/3728 and the budget row; the six-row scope; the collision claims about 0212's and 0202's
    touched surfaces; that `git-state postcondition` still occurs exactly once; **what
    `scripts/board-checks.sh` actually reads** (the first draft asserted it reads every row's state —
    false — and a second draft then mis-stated leg A as idle-gated when it is explicitly time-free);
    and **what `tests/test_loop_continuation.sh` actually asserts and which anchors it
    carries** (the first draft mis-described its style). Both corrections came from the critic
    verifying against the files, which is this assumption earning itself twice in one round.
