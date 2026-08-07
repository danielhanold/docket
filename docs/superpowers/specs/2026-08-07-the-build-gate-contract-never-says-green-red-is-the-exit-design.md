<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0224 — The build gate contract never says green/red is the exit code, so an output-shape match passes as a gate](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0224-the-build-gate-contract-never-says-green-red-is-the-exit-code.md)**
<!-- docket:backlink:end -->

# Design — the build gate's green/red verdict is the exit code (change 0224)

## Problem

`skills/docket-build/SKILL.md` § *The build gate* says what green and red **mean** — green mints the
build-evidence record, red enters the `premium -> max -> halt` repair ladder — but never says what
**determines** which one a run is. The determinant is the resolved suite command's exit status, and
the contract is silent on it.

On the change 0203 run (2026-08-06) the gate was implemented as a `tail -1` string match for
`"PASS"`, printing `### RED rc=0` for nearly every file in the suite. That instance was caught only
because the output was self-contradictory enough to notice. The failure that matters is the quiet
one: a shape-matching gate that happens to agree with the exit code mints a valid-looking
`docket:build-evidence` record for a branch nobody verified. `docket-implement-next` Step 6
validates the record's presence and `head_sha`, not the reasoning that produced it, and
`docket-finalize-change` step 4 uses a matching green record to **skip** its own local suite run —
so one false green propagates all the way to a merge.

There is a second, narrower hole the same clause closes. `skills/docket-build/references/gate-execution.md`
capability 5 already requires the gate to distinguish four states — *still running*, *completed
successfully*, *completed unsuccessfully*, *result unavailable* — but nothing anywhere defines
**successfully**. § *Gate execution posture* clause 3 forbids reading completion from the caller-visible
signal of the command that started the gate, so on the detached posture the agent never observes the
suite command's exit status directly; the verdict has to come out of the durable result artifact.

This is `AGENTS.md`'s own house rule — key a guard on shape, never an enumerated list of spellings —
applied to the gate itself.

## Decision

Three edits: a clause in `skills/docket-build/SKILL.md`, its guard in `tests/test_docket_build.sh`,
and the size-budget row raise those two require.

### 1. State exit-code keying normatively in § *The build gate*

Insert the clause **between the `configured-bash-finalize` command-boundary paragraph and the
`**Green** →` paragraph** — the hinge between resolving the command and acting on the verdict. Not a
new `###` subsection: a heading of its own would create the same all-at-once assert cascade
`tests/test_gate_execution_posture.sh` documents, for a clause this short.

Content, in substance (wording is the implementer's, the rules are not):

> The verdict is an **exit status**, never output text. The run is **green if and only if the
> resolved suite command exits zero**; any non-zero status is not green. A `PASS`/`FAIL` line, a
> summary count, or a progress ticker is diagnostic only — a gate that reads the verdict out of the
> output is not a gate. The deciding status is the one **recorded in the terminal result artifact**
> § *Gate execution posture* requires, which is where *completed successfully* is settled: *still
> running* and *result unavailable* are not statuses and stay budget halts, never red. When the
> resolved command is a **loop over per-file commands** — the shape finalize's
> `configured-bash-finalize` block takes with `FINALIZE_TEST_COMMAND` unset — the deciding status is
> the **aggregate** the block exits with, not any individual file's.

Plus one sentence binding the repair path: the rule governs **every** full-suite run this role
performs, including the repair worker's post-fix re-run, whose green is what ends the ladder.

Existing carve-outs are untouched and must not be reclassified by the new wording: the
**configuration gap** (item 3) and the **observation budget** exhaustion are not suite runs that
produced a status, so neither is red.

**Deliberately not stated:** any rule about what a *suite runner* should exit for a non-failure
condition. That is a rule about runners, not about this role's gate; it is absent from the stub's
`## What changes`; and `scripts/run-tests.md`'s exit-code table already defers the semantics here
("Exit-code semantics beyond this are change 0224's, not this script's") while itself exiting **4**
— non-zero — for a non-failure behind `--strict-budget`. Writing the runner rule here would
contradict its own best example. The gate's side of that contract is already fully expressed by
*green iff zero*.

### 2. Guard it in `tests/test_docket_build.sh`

New change-0224 banner in the existing file, not a new test file: `test_docket_build.sh` already
owns the build gate's contract prose (the `FINALIZE_TEST_COMMAND` derivation, the
`configured-bash-finalize` citation, the single-marker rule, the configuration-gap classification,
the repair ladder), and this is one clause in that same section.

Shape, following that file's established discipline:

- Slice § *The build gate* with the terminator `/^#+ /`, **not** `/^## /` — the level-2 form would
  swallow the `### Gate execution posture` subsection that `test_gate_execution_posture.sh`
  separately owns, and a bounded-gap assert over the wider slice could then match across sections
  and survive its own mutation (`section-slice-needs-a-named-terminator`). `/^#+ /` is the
  terminator `test_gate_execution_posture.sh` already uses for the sibling slice.
- Flatten whitespace (`tr -s '[:space:]' ' '`) before phrase matching — hard-wrapped prose otherwise
  turns every phrase assert into an accidental line-wrap guard (`phrase-grep-over-wrapped-prose`).
- A **non-vacuity companion** through the same extractor: assert the slice is non-empty and still
  carries a clause that predates this change — `configuration gap, not a red suite`, verified to sit
  inside the slice and to survive flattening. Without it a renamed heading or a broken `awk` range
  turns every scoped assert into a permanent green.
- Assert each rule separately, so each is independently deletable-and-red: (a) the iff — exit status
  decides green; (b) the negative — output text is not the verdict; (c) the verdict is read from the
  terminal result artifact, and *still running* / *result unavailable* are not red; (d) the
  per-file-loop aggregate; (e) the repair re-run is bound by the same rule.
- At most **one** bounded gap per ERE. Two or more backtrack catastrophically on non-matching input,
  so the mutation test **hangs instead of reddening** (`stacked-gap-regex-hangs-instead-of-failing`).
- No assert keyed on a bare common noun or identifier (`exit`, `gate`) — anchor on a verbatim slice
  of the claim (`assert-detects-removal-not-replacement`, war story #226).

**Mutation-test each assert one clause at a time**, per `AGENTS.md`: delete or invert that clause,
confirm the edit landed with `grep -c` before and after, watch that single assert redden, restore
from a backup **copy** — never `git checkout --`, which restores to HEAD and destroys the
uncommitted work under test (`mutation-restore-needs-a-backup-copy`). An assert never seen red
against its own mutation is decoration.

### 3. Raise the `skills/docket-build/SKILL.md` size-budget row in the same diff

Measured 2026-08-07: the file is **317 lines / 2938 words** against row `325 3000` in
`tests/test_skill_size_budgets.sh` — **8 lines / 62 words** of headroom. The clause above does not
fit, so the raise is part of this change, not a surprise at build time.

Follow that file's documented raise rule exactly: re-measure the grown file, round **lines up to the
next multiple of 5** and **words up to the next multiple of 50**, and if either lands within 25 of
the actual, take the multiple after it. The drafted prose measures ~+11 lines / ~+159 words, landing
near 328/3097 — where 3100 leaves a 3-word margin and the within-25 clause therefore forces `3150`,
giving approximately `330 3150`. The implementer sets the row from **its own** measurement of the
file it actually wrote, never from this estimate.

The raise must also **name the `references/` file the prose was considered for and argue why it
cannot live there** (the file's own rule, change 0201). The argument for the record:
`skills/docket-build/references/gate-execution.md` is the candidate and is the wrong home — it is
quarantined per-harness capability and probe evidence, read once before the gate starts, whereas
this is the rule that must be in hand at the moment the verdict is formed, in the section that
already states what green and red *do*. Splitting *what decides green* from *what green does* across
two files is precisely the drift that produced the gap.

If the addition can be tightened to fit within the existing row without losing a rule, that is
better than a raise — but never trim unrelated prose to make room: deleting a restatement is never a
one-file edit (`restatement-accumulates-its-own-guards`).

### 4. Per-file-loop confirmation

Not a separate deliverable: it is the loop sentence in edit 1 plus assert (d) in edit 2. The
confirmation is already discharged by reading finalize's `configured-bash-finalize` block against
the drafted wording — it accumulates `suite_status=1` per failing file and exits on
`[ "$suite_status" -eq 0 ]`, so the aggregate *is* the status and the wording holds unchanged.

## Assumptions

Every decision below was defaulted without a human. This is the audit trail. Items 1–7 survived an
adversarial critic pass; items 6 and 8 record what that pass corrected.

1. **Guard placement — existing `tests/test_docket_build.sh`, not a new file.**
   *Rejected:* a new `tests/test_build_gate_verdict.sh`, mirroring change 0223's
   `tests/test_gate_execution_posture.sh`. *Why:* 0223 spanned four surfaces (the build skill,
   finalize's citation, a reference file, and the budget default's agreement across all of them),
   which earned a file; 0224 is one clause in one section whose guards already live in
   `test_docket_build.sh`. A new file adds a suite entry and a runtime-budget row for nothing.
   *Reversible:* trivially — the asserts move as a block.

2. **Scope — the `docket-build` gate (including its repair re-run) only; `docket-finalize-change`
   is not edited.** *Rejected:* also binding finalize's step-5 post-rebase local run and
   `references/gate-failure.md`. *Why:* the stub's `## Why` is entirely about the build gate and the
   build-evidence record; finalize's verdict already runs through the `configured-bash-finalize`
   block, whose last line **is** the exit-status test, so the mechanism is present there even with
   the prose silent; and restating one rule in two skills grows a second set of guards over the copy
   (`restatement-accumulates-its-own-guards`). *Residual:* finalize's prose stays silent on the
   verdict rule — a candidate follow-up, not scope creep here.

3. **No runtime assertion; contract prose plus a docs guard is the whole of it.**
   *Rejected:* extending the `docket:build-evidence` record with an `exit_code:` field so a false
   green is structurally detectable. *Why:* that changes a schema with three consumers
   (docket-build's emitter, `docket-implement-next` Step 6's validator, finalize step 4's skip
   predicate) while an open change already works that surface (0190, still in `active/`). A
   cross-change schema decision belongs to a human and is not what this stub asked for.
   *Also rejected:* having the gate echo `rc=$?` — a self-report by the same agent proves nothing.

4. **The rule is stated as `green iff exit zero`, with no exit-code taxonomy.**
   *Rejected:* enumerating meanings for specific codes (`run-tests.sh`'s exit 3 = "certified
   nothing", exit 4 = strict budget breach). *Why:* an enumerated list of spellings is the exact
   anti-pattern this change closes, one level up. *Residual, stated plainly:* under this rule
   `run-tests.sh`'s exit 3 (a harness failure that certified nothing) reads as red and mints a
   repair task against zero failing assertions. That is fail-closed and identical to today's
   behavior — this change makes it no better and no worse. Fixing it needs either a taxonomy or a
   runner change, both out of scope. The runner-side obligation is deliberately **not** stated here;
   see edit 1's *Deliberately not stated*.

5. **Placement inside § *The build gate*, in one named slot: after the `configured-bash-finalize`
   command-boundary paragraph, before `**Green** →`.** *Rejected:* a new `### Gate verdict`
   subsection parallel to `### Gate execution posture`. *Why:* a heading of its own invites the
   re-levelling cascade `test_gate_execution_posture.sh` records, and the verdict reads in sequence
   where it sits — after the command is resolved, before Green and Red act on it.

6. **The size-budget row must be raised in the same diff — the file does not have room.**
   *Corrected from the draft's "has room", which was false.* Measured: 317/2938 against `325 3000`,
   i.e. 8 lines / 62 words of headroom for a ~10-line / ~130-word addition. Edit 3 takes the file's
   own documented raise path (re-measure, round, name the `references/` candidate and argue against
   it) rather than the draft's original instruction to halt — a halt would have been the near-certain
   outcome of building this spec.

7. **Dependency state.** `depends_on:` is empty and nothing here is gated on 0190, 0223, or 0227.
   0223 and 0227 are both archived (2026-08-07): `### Gate execution posture` and
   `scripts/run-tests.sh` are read as **current tree state**, not assumptions. The implementer's
   reconcile re-validates — if § *The build gate* has been restructured since 2026-08-07, re-derive
   the placement rather than forcing this one.

8. **The verdict is sourced from the terminal result artifact, not from the caller-visible exit of
   the command that started the gate.** *Added after the critic pass.* Phrasing the rule purely as
   "the resolved suite command exits zero" is under-specified against § *Gate execution posture*
   clause 3, which forbids reading completion from that signal. This also supplies the definition of
   *completed successfully* that `references/gate-execution.md` capability 5 requires and never had.
   *Rejected:* leaving it implicit — the two clauses would read as contradicting each other, and the
   contradiction lands on an autonomous builder.

## Out of scope

- The gate's execution posture and timeouts (change 0223 — landed; this clause plugs into it).
- Suite runtime and the budget table (change 0227 — landed).
- What green and red *do* — the evidence record, the repair ladder. Only what decides them.
- Any change to the `docket:build-evidence` record schema (assumption 3; adjacent to change 0190).
- Any rule about what a suite runner should exit for a non-failure condition.
- `docket-finalize-change`'s prose (assumption 2).

## Verification

- `bash tests/test_docket_build.sh` green.
- Each new assert individually mutation-tested red, with `grep -c` before/after confirming the
  mutation landed.
- `bash tests/test_skill_size_budgets.sh` green, with the raised row and its in-diff argument.
- `bash tests/test_gate_execution_posture.sh` green — the new clause sits adjacent to that slice and
  must not disturb it.
- Full suite via `scripts/run-tests.sh`.
