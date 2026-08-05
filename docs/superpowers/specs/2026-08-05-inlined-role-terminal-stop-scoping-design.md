<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0212 — An inlined role skill's terminal stop ends the whole run — scope docket-build's stop and enforce the run disposition](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-05-0212-an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco.md)**
<!-- docket:backlink:end -->

# Design — scope the inlined skill's terminal stop, and bind the run disposition

Change 0212. Autonomously groomed (docket-auto-groom, 2026-08-05) — every decision below is a
default committed without a human; the `## Assumptions` block is the audit trail. Revised once after
the adversarial critic pass.

## Problem

`docket-implement-next` Step 5 invokes the resolved build skill **inline via the Skill tool**. An
inline invocation loads the sub-skill's body into the *same context* as the driver's step sequence,
so its second-person terminal sentence — `skills/docket-build/SKILL.md:11`, *"Then you stop — review
is not yours."* — resolves "you" to the driver. On 2026-08-05 the 0206 run executed Steps 0–5 in
full and then closed with docket-build's own output contract verbatim, ending the turn at the Step
5/6 boundary with the branch unpushed, no review, and no PR.

Two independent surfaces failed:

1. **The sub-skill's stop is unscoped.** It is correct for a dispatched role (a subagent whose turn
   genuinely ends) and hazardous for an inlined one, and the skill body cannot know which it is.
2. **The run's terminal disposition is unenforced.** `docket-implement-next` names four run
   dispositions (`advanced` / `contended` / `drained` / `halted`) as a *driver contract* — guidance
   on how a driver interprets an outcome. Nothing binds the running agent to declare one. The 0206
   run declared a **build** disposition instead, and that wrong vocabulary is by itself proof the run
   never reached its terminal step. It would have caught all four observed incidents (0109, 0194
   twice, 0206).

0113's Step 5 rider did not prevent this: Step 5's postcondition *was* satisfied. The rider guards
**within** a step; the failure was the **transition out** of it.

**Why this fix is not the one that already failed.** 0113's rider is prose in the driver's own body,
at the moment of action, and it still lost to `docket-build/SKILL.md:11`. Adding a third call-site
rider would be the same move a third time. Lever 1 is different in kind: it **removes the conflicting
instruction at its source** rather than trying to out-rank it. That is the design's load-bearing
argument, and lever 2 is deliberately the *smaller* half — a closing obligation, not the mechanism.

## Design

### Lever 1 — scope the stop to the role

**The hazardous construct is broader than a stop sentence.** Inline loading makes *any* second-person
directive bind the driver. Two classes:

- **Terminal stops** — "Then you stop", "STOP AT THE SPEC", "return". The class that fired.
- **Second-person prohibitions** — `docket-review/SKILL.md`'s `## Conduct` ("A reviewer **never
  writes** files… **never commits**… **never dispatches** subagents"). Inline-loaded at Step 6 these
  would forbid the driver's own blocker-fix dispatch, the `docket-adr` dispatch, and Step 7's
  metadata writes. Strictly the worse hazard, and it must be in scope.

`docket-brainstorm` Step 3 is the counter-example that defines the safe form: it stops, then names
the owner of the next step (*"Do NOT continue to `superpowers:writing-plans`; planning is build-time,
owned by `docket-implement-next`"*).

**Sweep criterion — any docket-owned skill body that can be loaded into a caller's context**, whether
by a `skills:` role binding, by a Tier-A inline fallback, or by preload into a generated wrapper.
*Not* "role skill": `docket-adr` and `docket-status` are not role skills (no `skills:` key binds
them; `tests/test_role_skill_self_description.sh` fixes docket's role-skill set as exactly
`docket-build docket-review docket-brainstorm`), yet both are explicitly run **inline** under the
convention's Tier A.

**The sweep's deliverable is a per-file verdict, not an assumed edit set.** Current reading — to be
re-verified at build time, since this table is itself a document asserting facts about other
artifacts:

| File | Verdict |
|---|---|
| `skills/docket-build/SKILL.md` | **Edit.** Line 11's stop and `## Output`'s "the terminal build disposition" are the exact construct that fired. |
| `skills/docket-review/SKILL.md` | **Edit, and larger than "lighter".** The H1 at line 6 ("read the branch, return findings, stop") and `## Halting`'s abort-and-report paragraph are terminal; `## Conduct`'s never-writes / never-commits / never-dispatches prohibitions are the second class above. `## Scope` already says the dispatching controller owns every following write, so some scoping is present. |
| `skills/docket-adr/SKILL.md` | **Likely no-hazard.** No terminal stop sentence; ends on a validation invocation. Record the finding. |
| `skills/docket-brainstorm/SKILL.md` | **Likely no-hazard.** Its stop names the next step's owner. Record the finding, and treat its phrasing as the house pattern. |
| `skills/docket-status/SKILL.md` | **Verdict required.** Carries an unscoped second-person stop ("surface the stderr diagnostic and stop rather than improvising a fix") and is inlined at `docket-implement-next` Step 0 under Tier A. Included by the criterion; edit-or-no-hazard is a build-time call. |
| `skills/docket-build-task/SKILL.md` | **Verdict required.** Admitted by the same preload relation as the `docket-review-*` rungs (`agents/docket-build-*.md` carry `skills: [docket-build-task]`) and it carries both hazard classes — never-dispatch / never-write prohibitions and a terminal "Return exactly one of three outcomes". Tightest budget in the set (111/959 against 115/1000). |

Excluded: **vendored** skills (`superpowers:writing-plans`,
`superpowers:finishing-a-development-branch`) — docket does not own those files, a plugin upgrade
silently reverts an edit, and ADR-0044's call-site pre-specification is the sanctioned remedy there.
**Nothing is excluded by being "always dispatched":** a generated wrapper is a thin separate file
that *injects* a skill body (`agents/docket-adr.md` carries `skills: [docket-adr,
docket-convention]`; `agents/docket-review-lean.md` carries `skills: [docket-review]`), so the
wrapper and the dispatched subagent read the **same body** — a clause added to a body is therefore
also read by a subagent whose turn really does end.

**The clause must therefore be two-sided and conditioned on invocation mode**, stated by shape (exact
wording is a build-time choice under a hard word budget): *if your caller's run continues past this
role, your stop ends this role only and the caller continues to its own next step; if you were
dispatched as a subagent, your turn ends here.* Same for the prohibition class: the prohibitions bind
this role's conduct, not a caller that loaded this body inline. `docket-build`'s `## Output`
additionally labels its disposition **role-scoped** — a build disposition is not a run disposition.

**Mechanical companion:** a new **positive-presence** guard test in the style of
`tests/test_role_skill_self_description.sh` (changes 0194/0198/0199) — for each swept file that
carries a terminal stop or a second-person prohibition, assert the body also carries the
mode-conditioned scoping clause; with the same non-vacuity anchors (file exists and is non-empty; a
live presence assert through the same read) and a mutation-in-fixture probe proving the matcher
fires. Two obligations inherited from 0199: the guard's file list is hand-maintained, so it must
match whatever sweep set the build lands on; and **presence of the clause anywhere in the file is not
presence of it at the stop** — 0199's co-occurrence lesson applies, so the assertion needs proximity
or per-site scoping. A *negative* vocabulary guard is rejected: the existing guard's own header
documents that line- and vocabulary-scoped negative greps escape by paraphrase.

### Lever 2 — bind the run disposition, and where it stops

**The split:** 0212 adds a **run-level closing obligation**; 0203 settles **per-step git state**.

- **0212 (here):** `docket-implement-next`'s *Terminal disposition* section gains an obligation on
  the **agent**, not only guidance to a driver — the run does not end until exactly one of the four
  dispositions is declared, and **a final report declaring any other disposition vocabulary is by
  construction an aborted run**.
- **0203 (not here):** what git state each step must reach. 0212 states that `advanced` is claimable
  only when **Step 7's postcondition holds** — the PR URL, plus `status: implemented`, `pr:` and
  (when a results file exists) `results:` written — **by pointing at Step 7**, never by defining
  Step 7's postcondition.

**The seam is a pointer rule, not a surface partition.** 0203's `## Why` names the *Terminal
disposition* section verbatim, and 0203 explicitly leaves open whether its postconditions are stated
inline per step or as one table the steps point at — so the two changes may well end up editing the
same section. What keeps them from colliding in *design space* is the pointer-not-definition rule
above: 0212 adds an obligation to declare a disposition; 0203 may later attach the postcondition
table that obligation points into. Both additive. Today the pointer resolves to Step 7's enumerated
prose; after 0203 it resolves to a named postcondition. It degrades gracefully in either landing
order.

**No new runtime mechanism for lever 2** — the settled answer to the stub's open question. Four
options weighed:

1. A check in `board-checks.sh` — impossible: the final report is model output and that script is
   git-only by contract.
2. A driver-side check in `/loop` — rejected: vendored plugin, not docket's to change.
3. A wrapper-level check — rejected: no channel to read a subagent's final message.
4. **Require the declared disposition to leave a git trace** (a run-record line on
   `metadata_branch`), which `board-checks.sh` *could* read within its git-only contract. This is the
   option consistent with docket's own "the contract is git state, not an in-context return" stance,
   and it is **rejected on two grounds**: it duplicates 0211's leg C for the one signature leg C
   already covers, and the convention's Tier-C precedent is explicit that the remedy for an abandoned
   autonomous run is "**no new status, no new field**" — the reclaim lease self-heals it.

So lever 2 is **prose plus a prose-presence guard**: extend `tests/test_loop_continuation.sh` (which
already asserts the four-disposition contract) with asserts for the closing obligation and the
wrong-vocabulary rule.

**Mechanical coverage, stated accurately.** 0211's leg C is gated on `pr:` empty plus a 2h
branch-idle floor and fires only on **built-but-not-delivered** — the 0206 signature. The other three
(0109, 0194's first, 0194's second) are the signatures 0113's existing incoherence leg was designed
against, so all four observed signatures already have a deterministic backstop once 0211 lands. That
*strengthens* the rejection of option 4 below: a new git trace would add nothing the ledger does not
already detect. The one signature with no backstop is 0211's own explicit non-goal — a **sixth**
shape where the PR is opened and `pr:` **is written** before the run dies — which leg B catches at
12h, advisory.

### Size budgets

`tests/test_skill_size_budgets.sh` is a live constraint:

| File | Actual (2026-08-05) | Budget | Margin |
|---|---|---|---|
| `skills/docket-build/SKILL.md` | 260 lines / 2348 words | 265 / 2400 | 5 lines, 52 words |
| `skills/docket-implement-next/SKILL.md` | 139 lines / 3728 words | 145 / 3800 | 6 lines, 72 words |
| `skills/docket-review/SKILL.md` | 96 / 758 | 105 / 800 | 9 lines, 42 words |
| `skills/docket-status/SKILL.md` | 96 / 2260 | 118 / 2393 | 22 lines, 133 words |
| `skills/docket-build-task/SKILL.md` | 111 / 959 | 115 / 1000 | 4 lines, 41 words |

Sequence: **compress the touched section, re-measure, then raise only the rows the post-compression
actual exceeds.** A raise is not pre-committed here — `docket-build`'s 52-word margin may absorb the
clause outright. Rows that do need raising are set from the *measured* actual per the file's
documented rounding rule (lines → next multiple of 5, words → next multiple of 50; if that lands
within 25 words of the actual, take the multiple after it).

The in-diff raise justification (change 0201's rule) must **name the `references/` file the prose was
considered for and state why it cannot live there**. Note that `skills/docket-build/` and
`skills/docket-review/` have **no `references/` tree at all**, unlike every precedent cited
(0127/0137 raised `docket-convention` and `docket-implement-next`, which do). For those files the
argument must name the home that *would have to be created* and argue why creating it is wrong —
here, that the clause must fire at the exact moment the stop is read, and a rule in an unread
reference file cannot intervene there. Behaviour-neutral slimming of *unrelated* prose is rejected as
the primary lever: it couples a fix to a refactor, and `size-target-is-direction` records that the
number is a direction, not a gate.

## Sequencing and couplings

- **0113** — origin; this is its prose half, and the surface its Step 5 rider could not reach.
- **0211** — the mechanical half (`aborted-run` leg C). Disjoint files (`scripts/board-checks.sh` +
  its tests vs `skills/` + guard tests). No dependency in either direction.
- **0203** — adjacent normative design task. Collides on `skills/docket-implement-next/SKILL.md`
  (likely the *same section*) and on the `tests/test_skill_size_budgets.sh` budget row. Not a
  dependency: neither blocks the other. **But a budget row is a semantic conflict even when git
  merges the line cleanly** — the file records exactly this against itself (0113's 4050 and 0201's
  3700 both measured pre-rebase, neither survived). Whichever change lands second **must re-measure
  the merged file and re-derive the row from the post-rebase actual**;
  `concurrent-edits-compose-at-rebase` covers the prose, not a derived number.
- **0096** — the earlier instance of this class (the plan skill's execution-handoff prompt).
- **0154** — the skill-body sweep pattern (enumerate, then decide per hit).
- **0194 / 0198 / 0199** — the role-self-description guard whose test shape lever 1's guard copies.
  `mirrored-guard-enforces-its-own-property`: probe the copied matcher by execution before claiming
  it enforces the new property.

## Out of scope

- 0211's `aborted-run` leg (the deterministic oracle).
- Defining per-step git-state postconditions (0203).
- Editing any vendored skill body.
- Changing the 0049 role-invocation contract — see assumption 12.
- Reversing ADR-0044 or re-litigating ADR-0024.

## Assumptions

Every decision below was defaulted autonomously. Each names the alternatives weighed and why the
chosen one is the conservative default.

1. **Sweep set = docket-owned skill bodies loadable into a caller's context — six files.** The
   criterion is loadability, not role-skill membership; `docket-adr` and `docket-status` are not role
   skills but are inlined under Tier A, and `docket-status` is included on exactly the footing that
   admits `docket-adr`. *Rejected:* scoping to the three role skills (misses the Tier-A inline
   paths, which are first-class equivalent paths by the convention's own words). *Rejected:* every
   `skills/**/SKILL.md` (adds prose against the size budget for files no caller loads). *Rejected:*
   editing vendored skills — see out-of-scope.

2. **The deliverable is a per-file verdict, including recorded no-hazard findings.** *Rejected:* a
   pre-committed edit list. `verify-the-claim` — the table above is a document asserting facts about
   other artifacts, and the build re-verifies against the files. The critic already found one such
   error in the draft (a heading reference that does not resolve), which is the rule earning itself.

3. **The scoping clause is stated by shape, two-sided and conditioned on invocation mode.**
   *Rejected:* an unconditional "your stop ends this role, the caller continues" — a generated
   wrapper is a separate file that injects the **same** skill body, so a one-sided clause is read by
   dispatched subagents whose turn genuinely ends, where it is inert at best and invites continuing
   past the return at worst. *Rejected:* pinning exact sentences here (the build owns phrasing under
   a hard word budget).

4. **Lever 1 gets a positive-presence guard, not a negative vocabulary guard.** *Rejected:* a
   negative grep forbidding unqualified "you stop" — the existing role-self-description guard's own
   header documents that such guards are line- and vocabulary-scoped and escape by paraphrase.
   *Rejected:* no guard at all — `fix-reintroduces-its-own-defect-class` and
   `restatement-accumulates-its-own-guards` both point the other way. The guard must be
   proximity-scoped (0199's co-occurrence lesson), not a bare file-level presence check.

5. **Lever 2 stays in 0212, split from 0203 by a pointer-not-definition rule.** *Rejected:* the
   draft's justification that *Terminal disposition* is a surface 0203 does not name — **factually
   false**: 0203's `## Why` names it verbatim, and 0203's own scope reaches Steps 6/7 bookkeeping.
   The split survives on the pointer rule alone: 0212 obliges a disposition to be declared, 0203
   defines the git state it points into; both additive to the same section. *Rejected:* handing
   lever 2 wholly to 0203 (the obligation that would have caught all four incidents would wait on an
   unstarted design task). *Rejected:* 0212 defining Step 7's postcondition itself (duplication the
   stub forbids).

6. **No new runtime mechanism for lever 2 — and the git-trace option is named and rejected, not
   omitted.** All four options are enumerated in the Design section. Option 4 (a git-readable run
   record) is the one consistent with docket's git-state-is-the-contract stance and is rejected on
   two named grounds: redundancy with the `aborted-run` ledger (leg C covers the 0206 signature; the
   other three observed signatures are what 0113's existing incoherence leg was built against, so all
   four are covered once 0211 lands), and the convention's Tier-C "no new status, no new field"
   precedent. The only uncovered shape is 0211's own declared non-goal — PR opened *and* `pr:`
   written before the run dies — which leg B catches at 12h.

7. **Budgets: compress, re-measure, raise only what still exceeds.** *Rejected:* pre-committing a
   raise for both files (docket-build's 52-word margin may absorb the clause). *Rejected:* offsetting
   cuts to unrelated prose as the primary lever. Note the 0201 rule's awkward fit: `docket-build/`
   and `docket-review/` have no `references/` tree, so the required argument names the home that
   would have to be created and argues against creating it.

8. **No `depends_on`; 0203 and 0211 are `related:` only — with an explicit re-measure obligation.**
   *Rejected:* `depends_on: [203]` (0212's pointer resolves to existing Step 7 prose today).
   *Rejected:* `depends_on: [211]` (disjoint files, independently valuable). The 0203 budget-row
   collision is **semantic, not textual** — a clean git merge still yields a wrong number, exactly as
   the budget file records of 0113 and 0201 — so whichever lands second re-derives the row from the
   post-rebase actual. Dependency state at groom time: none unsatisfied.

9. **Priority stays `medium`, type stays `fix`.** *Rejected:* raising to `high` to match 0211 — 0211
   is the detection half; 0212 is prevention over prose already partially mitigated at Steps 4 and 5.
   Priority is a human's call over backlog composition; an autonomous groom does not raise it.

10. **The fix works by removing the conflicting instruction at its source, not by out-ranking it.**
    Surfaced by the critic as the design's central unargued claim, and now stated in `## Problem`.
    0113's rider was already driver-body prose at the moment of action and still lost. Lever 1 edits
    the sub-skill body for that reason; lever 2 is deliberately the smaller half. *Rejected:* a third
    call-site rider as the primary remedy — that is the move that already failed twice.

11. **The hazardous class includes second-person prohibitions, not only terminal stops.**
    `docket-review`'s `## Conduct` inline-loaded at Step 6 would forbid the driver's own dispatches
    and Step 7's metadata writes — a worse hazard than the stop sentence. *Rejected:* the draft's
    narrower "bare second-person terminal stop" shape, which would have let the sweep pass over it.

12. **The build role stays inline-invoked; dispatching it as a subagent is out of scope.** The hazard
    exists only because Step 5 loads a controller into the driver's context, and dispatching would
    remove the mechanism rather than counter-instruct it. *Rejected* because `skills:` passthrough
    binds a **skill** invoked via the Skill tool, not an agent; docket-build must dispatch profile
    workers sequentially into a shared feature worktree; and changing the 0049 role-invocation
    contract is a structural redesign, not a fix. Recorded here so the option is on the record rather
    than unconsidered — if this class recurs after 0212 lands, this is the next thing to weigh.
