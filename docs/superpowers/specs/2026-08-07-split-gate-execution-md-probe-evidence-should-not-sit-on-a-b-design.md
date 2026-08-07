<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0234 — Split gate-execution.md: probe evidence should not sit on a blocking-read surface](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0234-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b.md)**
<!-- docket:backlink:end -->

# Split `gate-execution.md`: probe evidence off the blocking-read surface — design

Change [#0234](../../changes/active/0234-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b.md).
Groomed autonomously (`docket-auto-groom`); every decision below was defaulted, not discussed with a
human — see § *Assumptions* for the audit trail.

## Problem

`skills/docket-build/references/gate-execution.md` is read **blocking before every gate run** (per
`skills/docket-build/SKILL.md` § *Gate execution posture*). At 168 lines / 1612 words against a
175/1650 budget, most of what an agent loads there is not instruction: § *Method*, the
one-variable-per-run ladder, the four launch durations, the 180s stand-in gate, the
failed-to-reproduce blocking claim, and the permission-classifier denial are a **measurement
report**. A build agent about to start a suite needs the capabilities, the mitigation, and each
harness's verdict; it does not need the probe design that produced them.

## The split axis

**Instruction vs. evidence** — not product-specificity, which is the axis change 0223 drew. The
quarantine 0223 built (product names out of `SKILL.md`) stays exactly as it is; this change draws a
second boundary *inside* the quarantine.

### Stays on the blocking-read surface — `references/gate-execution.md`

1. `## The six required capabilities` — unchanged.
2. The mitigation paragraph, including its non-obvious precondition ("the new session must be fully
   established before the initiating call returns"). The precondition **stays**: it is the operative
   variable, and an agent that reads the mitigation without it launches a racy detach.
3. `## Reading a verdict` — unchanged in substance, with its references to § *Method* repointed at
   the evidence file. **Three** references sit on the kept surface, not one: two inside
   `## Reading a verdict` (`gate-execution.md:43` "what § *Method* measured" and `:49-50`
   "§ *Method* observes from **outside** the harness") and a third inside the mitigation paragraph
   item 2 keeps (`:34`, "measured and recorded under *Method* below"). Repoint all three; the word
   "below" in the third becomes a cross-file pointer. A fourth occurrence (`:136`, "Two
   disambiguating runs are recorded under *Method*") sits inside `### cursor` and is part of the
   moved evidence — it must not be carried forward into the compressed section.
4. One compact `### <harness>` section per harness in `HD_SHIPPED_HARNESSES`, each carrying exactly:
   the **version string** (and invocation flags where the harness needs them to run at all), a
   one-or-two-sentence statement of the **mode/shape the evidence covers and what it does not**, and
   the `**Verdict:**` line with its ` — <scope>` clause. That clause is optional in general but
   **mandatory** for any section whose prose mentions `forked`/`dispatched` — i.e. for `claude` —
   enforced by `tests/test_gate_execution_posture.sh:381-382`.
5. A pointer to the evidence file, marked explicitly **non-blocking** ("not read before a gate run").

Target shape: ~85–95 lines (measured from the kept content: capabilities 24, mitigation 6,
`## Reading a verdict` 24, header + pointer ~8, four harness sections 3-5 each with blanks ~24). The
win is roughly half the file, not two-thirds. Measured harness sections today (heading through
verdict line, raw / non-blank): claude 21/18, cursor 13/10, codex 13/10, opencode 11/8 — so three of
the four are already well under 15 lines and only `claude` sheds real bulk. Expect the kept file to
land near **93 lines**, the top of the band, not the middle: the component figures above under-count
`## Reading a verdict`, which is 32 raw lines (`:37-68`), not 24. The `>= 40` non-blank floor at
`tests/test_gate_execution_posture.sh:160-161` is safe with wide margin either way.

### Moves to `references/gate-execution-evidence.md` (new, non-blocking)

- All of `## Method`: the stand-in-gate design and its three load-bearing properties, the
  `setsid(1)`/`POSIX::setsid(2)`/fork mechanics, the one-variable-per-run ladder and its three rungs.
- Per-harness evidence narratives: measured launch durations (0s / 19s / 11s / 5s), the
  disambiguating runs, the cursor run that failed to reproduce the inherited blocking claim, the
  Codex `succeeded in 0ms` teardown behavior, and the `claude -p` permission-classifier denial that
  left the forked mode unmeasured.
- A back-pointer to
  `docs/results/2026-08-07-the-build-gate-contract-never-states-an-execution-posture-for-results.md`,
  which already carries the same version scoping and re-probe caveats as human verification items.

The new file opens with a line stating it is **evidence, not instruction, and is not read before a
gate run** — the property this whole change exists to establish.

## Structure: sections, not a table

The change stub proposed "a compact verdict table (harness → token → version → scope qualifier)".
**Rejected on guard structure.** `tests/test_gate_execution_posture.sh` does more than assert a
verdict per harness:

- group (10) requires `^### <harness>$` headings whose set **equals** `HD_SHIPPED_HARNESSES`, and a
  `^**Verdict:** \`token\`( — scope)?$` line **inside that harness's section slice**;
- group (10c) slices each harness section, strips its verdict line, and — for any section whose
  residual prose contains the literal token `forked` or `dispatched` — requires that prose to name
  the mode the evidence was measured in and to record the forked/dispatched mode as unmeasured. The
  per-harness asserts are **conditional** (`grep -qiE "forked|dispatched" … || continue`,
  `tests/test_gate_execution_posture.sh:369`), backstopped by a population floor requiring at least
  one such section (`:384-385`).

A markdown table satisfies neither: it has no `###` headings and no per-harness prose slice. Turning
it into one would mean rewriting a guard file that is mutation-tested clause by clause and whose
comments record, for several asserts, the exact mutation that survived a looser form. That is a much
larger and riskier diff than the one this change is for.

The kept shape is therefore **one short section per harness** — which is the compact row the stub
wanted, expressed in the structure the guards already enforce. Each section becomes 3-5 lines; the
largest one today (`claude`, 21 lines) is where nearly all of the saving comes from.

Consequence for group (10c): the `claude` section keeps a short prose statement that its evidence is
an interactive two-foreground-call measurement and that the forked/dispatched mode is unmeasured.
That is **verdict scope**, which the stub itself puts on the kept surface — the *reason* it could not
be measured (the classifier denial) is what moves.

Four concrete constraints on how that section may be compressed, all of them guard shapes rather
than taste:

- it must contain the literal token `forked` or `dispatched`, or the (10c) loop skips the section
  entirely, `mode_secs` stays 0, and the population floor reddens;
- `forked`/`dispatched` and `unmeasured`/`not measured` must **co-occur inside one sentence**
  (`[^.]{0,120}`, either order);
- the word `interactive` must appear **not** preceded by a hyphen (`(^|[^-])interactive`), so the
  section cannot state only the non-interactive variant it failed to obtain;
- and the section's verdict line must carry a non-empty ` — <scope>` clause (`:381-382`).

Sentences that satisfy all four exist comfortably; they just are not free, which is why the section
budget below is 3-5 lines rather than 1-2.

## Version strings stay

The stub's *Why* counts "four external version strings" among what rots on the blocking-read
surface. They stay anyway: the file's own rule is that a verdict is an observation about the version
named in its section, so a verdict stripped of its version becomes an unfalsifiable claim about the
product and loses the only signal that says when to re-probe. Four short tokens is the price of the
rule; the *durations, flags rationale, and ladder* around them are what leave.

## Guards

Existing asserts in `tests/test_gate_execution_posture.sh` are expected to stay green unchanged, and
that expectation is itself a build-time check: run the file before and after the move. Three asserts
to watch, because their haystack shrinks:

- `reference: is non-vacuous (>= 40 lines)` — the kept file stays well above 40.
- (10b) `verdict scope: a verdict covers ONLY what Method measured` and the three
  `capability N is declared unmeasured` asserts read the `## Reading a verdict` slice, which is kept
  in full. Re-word the § *Method* citations **without** disturbing the `verdict … only … measur` and
  `capability N … not/unmeasured` shapes. **The repoint has a regex trap**: that assert is
  `grep -qiE "verdict[^.]{0,80}only[^.]{0,80}measur"` (`:328-329`) and `[^.]` cannot cross a period,
  so writing the new **filename** into that sentence (`gate-execution-evidence.md`) breaks the
  window and reddens it. Keep the sentence filename-free — "covers only what the probe measured" —
  and put the file pointer in an adjacent sentence.
- (10c)'s three per-harness asserts, per the constraints listed above.

Add to the same test file:

1. `evidence: the file exists` and is non-vacuous (`>= 40` lines) — a population floor, so the split
   cannot silently collapse back into one file with the evidence deleted.
2. `reference: points at the evidence file` — a literal `grep -qF "gate-execution-evidence.md"` over
   the kept file.
3. An **absence** assert on the kept file: no `^## Method` heading. Per the
   `restatement-accumulates-its-own-guards` learning's 0194 entry, an absence assert over a class
   being removed cannot go stale — the only way to redden it is to reintroduce the thing.

Before writing (1)–(3), grep the whole suite for prose being moved (the
`restatement-accumulates-its-own-guards` sweep — asserts reach into whichever copy was nearest).
Known dependents to expect: `tests/test_skill_size_budgets.sh` cites this file heavily **in comments**
and also carries a live `BUDGETS` data row for it (`:578`) driving three asserts plus the
completeness assert — a data row, not a comment, and it is one of the two rows this change edits.

## Budgets

`tests/test_skill_size_budgets.sh` rejects any `skills/**/*.md` without a row, so the new file needs
one regardless. Both rows are set from **measured actuals** by the file's own rounding rule (lines to
the next multiple of 5, words to the next multiple of 50; if that lands within 25 words of the
actual, the multiple after it), with an in-diff justification block in the same shape as the existing
entries. The *where else it was considered* clause is **not** required here: that rule binds a
**raise** only (`tests/test_skill_size_budgets.sh:21-25`), and neither row is one — the new file's row
is a build-time consequence of creating the file (the precedent 0223 recorded for exactly this case
at `:460-461`) and `gate-execution.md`'s moves downward. Write the reasoning anyway, briefly, because
the ratchet in the next paragraph is the discretionary half of this change.

The `gate-execution.md` row is **ratcheted down** to its new actual rather than left at 175/1650. A
ratchet is the only mechanism that stops the evidence drifting back onto the blocking-read surface;
leaving ~90 lines of headroom on a file whose whole defect was accumulated evidence would leave the
change unenforced. Per `size-target-is-direction`, the number is a direction — the working margin the
rounding rule produces is the intended slack, and a later change that genuinely needs the room raises
the row in-diff with its own justification, which is exactly the audit trail wanted here.

## Out of scope

Unchanged from the stub: no re-probing, no measuring the `claude` forked mode, no edit to
`docket-build` § *Gate execution posture* or the `GATE_OBSERVATION_BUDGET` contract, and the kept
surface is still blocking-read. Also out of scope: editing the archived results file or
`docs/results/` in any way.

## Assumptions

Every decision below was defaulted by the autonomous groom. Reverse any of them freely at build
time; none is load-bearing for the others except A1.

**A1 — Where the evidence lands: a new non-blocking sibling reference,
`skills/docket-build/references/gate-execution-evidence.md`.**
Rejected: *append to 0223's results file* — `docs/results/` records are terminal artifacts of a
completed change, and this content is live: the file's own rule is that verdicts are re-probed when
a version moves, so its evidence must be editable on that cadence. Rejected: *a new ADR* — ADRs
record decisions and are immutable once Accepted except for their status line, which is the wrong
lifecycle for a measurement that is expected to be rewritten. The sibling keeps the evidence one
relative link from the verdict it supports, and picks up a size-budget row (a cost, but also a
regrowth guard).

**A2 — Keep `### <harness>` sections; do not build the table the stub named.**
Weighed against rewriting `tests/test_gate_execution_posture.sh` groups (10) and (10c) to read a
table. Rejected: those asserts carry recorded mutation evidence for their current shape (the
`[^-]interactive` boundary, the verdict-line `$` anchor, the h_prose slice that excludes the verdict
line), and re-deriving them against a new structure risks silently weakening guards while the visible
goal — a smaller file — is met either way. The short-section shape delivers the size win without
touching a guard's semantics.

**A3 — Version strings stay on the kept surface.** Contra the stub's framing, which counts them among
the rot. Rejected removal because the version is what makes the verdict falsifiable and what triggers
the re-probe rule stated three paragraphs above it. Alternative considered and rejected: keep a bare
"see evidence for versions" pointer — that puts the re-probe trigger one file away from the rule that
names it.

**A4 — Ratchet the `gate-execution.md` budget row down to the new actual.** The stub asks the
question and this defaults to yes, for the reason the stub itself gives: a ratchet is the only thing
preventing regrowth. The alternative (leave 175/1650 as headroom) was rejected as leaving the change
unenforced.

**A5 — The mitigation's non-obvious precondition stays on the kept surface**, even though it was
"measured and recorded under *Method*". Rejected moving it with its measurement: it is an
instruction (how to launch), and its absence is the failure mode the Codex rung demonstrates. Only
the *measurement that established it* moves.

**A6 — New guards are additive; no existing assert is rewritten.** If the build finds an existing
assert that genuinely cannot hold against the new shape, that is a signal the split was drawn wrong —
re-draw the split, do not loosen the assert. Per `test-premise-deleted-not-regated`: narrow a guard
whose property still holds, delete one whose subject is gone; re-gating to green is out of bounds.
One exception is pre-diagnosed so it is not mistaken for that signal: an assert that reddens purely
because a **re-worded citation** broke a `[^.]`-bounded window (the (10b) trap above, or the (10c)
co-occurrence windows) is a wording defect in the new prose, not evidence of a wrong split — fix the
prose, never the pattern.

**A7 — `docs/results/` is not touched.** The duplication the stub notes (results file carries the
same version scoping) is left as-is, with the evidence file pointing at it. Rejected consolidating
the two — but **not** on an immutability rule: docket makes no such rule for `<results_dir>/` (the
convention says only that results files are never archived; immutability is an `Accepted`-ADR
property, which A1 cites and this assumption must not borrow). The real reason is narrower and
sufficient: a results file records what one change's close-out verified, its content is already
published onto the integration branch, and rewriting it to de-duplicate would edit a historical
statement to serve a later refactor. Leave it; point at it.

**A8 — Dependency state: `depends_on: []`, `related: [231]`.** The change is
`discovered_from: [223]`, whose work is merged and archived — no unsatisfied dependency, nothing to
design around. The coupling to change 0231 ("a presumed-dead build worker can wake and race its own
replacement") is recorded in the **`related:` field**, not only in this prose: 0231 shares
`discovered_from: [223]`, already has a spec, and its `## What changes` targets `docket-build`'s
halting conditions plus `docket-implement-next`'s fix loop. The basis is **one probable file
collision**: 0231's spec anticipates raising a `tests/test_skill_size_budgets.sh` row with its own
rationale, which would land an **adjacent** `BUDGETS` row (`docket-build/SKILL.md`) and a new entry
in the same justification-comment block that 0234 also edits. Probable, not certain — 0231's
`## What changes` lists no budgets edit, and its guards go in `tests/test_docket_build.sh`.
0234 does **not** edit `skills/docket-build/SKILL.md` at all (see § *Out of scope*; the posture's
pointer names `references/gate-execution.md`, a filename this split keeps), so there is no collision
there. Forward link only (0231 is not edited to point back). At build time, check 0231's state and
rebase the budgets edit against it rather than assuming either row.

## Open questions

None left open for the human. Both questions the stub raised (evidence home; budget ratchet) are
settled above as A1 and A4.
