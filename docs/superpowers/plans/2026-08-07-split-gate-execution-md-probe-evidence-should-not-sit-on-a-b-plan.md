<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0234 — Split gate-execution.md: probe evidence should not sit on a blocking-read surface](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-07-0234-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b.md)**
<!-- docket:backlink:end -->

# Split `gate-execution.md`: probe evidence off the blocking-read surface — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: implement this plan task-by-task under docket's resolved build role (`docket-build`, which routes each task to a profile agent running the `docket-build-task` contract); `superpowers:subagent-driven-development` / `superpowers:executing-plans` are the generic equivalents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the probe *evidence* out of `skills/docket-build/references/gate-execution.md` — a file read blocking before every gate run — into a new non-blocking sibling, leaving only instruction and per-harness verdicts behind, and ratchet the size budget so the evidence cannot drift back.

**Architecture:** Two additive-then-subtractive tasks. Task 1 **creates** `references/gate-execution-evidence.md` carrying § *Method*, the one-variable-per-run ladder, and the four per-harness evidence narratives, plus its own `BUDGETS` row and a population-floor guard — the tree stays green with the content temporarily duplicated. Task 2 **removes** that content from the kept surface, compresses each `### <harness>` section to version + scope + verdict, repoints three § *Method* citations, adds the pointer + two more guards, and ratchets the kept row down to its new measured actual.

**Tech Stack:** Markdown (skill reference files), Bash test guards (`tests/test_gate_execution_posture.sh`, `tests/test_skill_size_budgets.sh`), suite runner `scripts/run-tests.sh`.

## Global Constraints

- **Repo root for every path below:** `/Users/homer/dev/docket/.worktrees/split-gate-execution-md-probe-evidence-should-not-sit-on-a-b`. All `git` and test commands run from there.
- **No existing assert in `tests/test_gate_execution_posture.sh` may be rewritten.** New guards are additive (spec A6). If an existing assert reddens *because a re-worded citation broke a `[^.]`-bounded window*, that is a wording defect in the new prose — **fix the prose, never the pattern.** If an assert reddens for any other reason, the split was drawn wrong: stop and report, do not loosen the assert.
- **Do not edit** `skills/docket-build/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `README.md`, `docs/results/**`, or `scripts/lib/harness-defaults.sh`. The kept file keeps its filename `gate-execution.md`, so every existing pointer at it stays valid — verified by sweep: `README.md:723`, `tests/test_gate_execution_posture.sh:157`, `skills/docket-build/SKILL.md:274`, `skills/docket-finalize-change/SKILL.md:143` all name `gate-execution.md` and none names moved prose.
- **Version strings stay** on the kept surface (spec A3): each `### <harness>` section keeps its version token and any invocation flags it needs to run at all.
- **Budget rows are set from measured actuals** by the rule in `tests/test_skill_size_budgets.sh:12-19`: lines rounded up to the next multiple of 5, words to the next multiple of 50; **if that lands within 25 words of the actual, use the multiple after it.** The same near-zero-headroom caution applies to the line figure (see the 0102/0137/0167 entries in that comment block).
- **Shell portability:** BSD `grep` ERE has no `\b`; use bounded character classes. Do not stack two `[^.]{0,N}` gaps in one new ERE (learnings: `stacked-gap-regex-hangs-instead-of-failing`) — the new guards below use none.
- **Concurrency:** change 0231 is being built in parallel from the same `origin/main` tip and may add or raise an adjacent `BUDGETS` row plus a justification-comment entry. **Append** this change's justification entry at the **end** of the comment block (immediately above `BUDGETS="`), never mid-block, and touch only the two rows this change owns. Whichever PR merges second rebases.

---

## File Structure

| File | Action | Responsibility after this change |
|---|---|---|
| `skills/docket-build/references/gate-execution.md` | **Modify** | Blocking-read **instruction**: six capabilities, the mitigation + its precondition, `## Reading a verdict`, one compact `### <harness>` section per shipped harness, and a non-blocking pointer to the evidence. |
| `skills/docket-build/references/gate-execution-evidence.md` | **Create** | Non-blocking **evidence**: § *Method*, the one-variable-per-run ladder, per-harness measurement narratives, and a back-pointer to 0223's results file. |
| `tests/test_gate_execution_posture.sh` | **Modify** (append only) | Four new asserts pinning the split: the evidence file exists and is non-vacuous, the kept file points at it, and the kept file carries no `## Method`. |
| `tests/test_skill_size_budgets.sh` | **Modify** | New row for the evidence file; `gate-execution.md` row ratcheted down; one appended justification entry covering both. |

---

### Task 1: Create the evidence file, its budget row, and its population-floor guard

**Files:**
- Create: `skills/docket-build/references/gate-execution-evidence.md`
- Modify: `tests/test_gate_execution_posture.sh` (append a new group before the final `exit $fail`)
- Modify: `tests/test_skill_size_budgets.sh` (append a justification entry; add one `BUDGETS` row)
- Test: `tests/test_gate_execution_posture.sh`, `tests/test_skill_size_budgets.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the file `skills/docket-build/references/gate-execution-evidence.md` (Task 2's pointer target, and the file whose *name* Task 2's `grep -qF "gate-execution-evidence.md"` assert looks for in the kept file); the shell variable `EVID` and `evid_body` in `tests/test_gate_execution_posture.sh`, which Task 2's added asserts sit beside but do not reuse.

**Why the intermediate state is green:** after this task the evidence exists in *both* files. That duplication is temporary and deliberate — every existing assert reads the kept file, which is untouched here, so the suite stays green and Task 2's deletion is reviewable on its own.

- [ ] **Step 1: Write the failing guard for the evidence file**

Open `tests/test_gate_execution_posture.sh` and insert this block **immediately before** the final line `exit $fail` (currently line 387). It must come after `ref_body` is defined (line 159) — appending at the end satisfies that.

```bash
# --- (11) the SPLIT: probe evidence lives OFF the blocking-read surface (change 0234) ---
# `gate-execution.md` is read blocking before every gate run (docket-build § Gate execution
# posture). The probe design, the launch-duration ladder, and the per-harness measurement
# narratives are a measurement report, not instruction, and they rot on an external schedule —
# so they live in a sibling that no gate run loads. These four asserts pin that split.
EVID="$REPO/skills/docket-build/references/gate-execution-evidence.md"
assert "evidence: the file exists" '[ -f "$EVID" ]'
evid_body="$(cat "$EVID" 2>/dev/null)"
# Population floor, same shape as the kept file's: without it the split could silently collapse
# back into one file with the evidence DELETED rather than moved, and every assert here would
# still pass on an empty sibling.
assert "evidence: is non-vacuous (>= 40 lines)" \
  '[ "$(printf "%s\n" "$evid_body" | grep -c .)" -ge 40 ]'
```

- [ ] **Step 2: Run it to verify it fails**

```bash
bash tests/test_gate_execution_posture.sh 2>&1 | grep -E "^(NOT )?ok - evidence"
```

Expected: two lines, both `NOT OK` — `NOT OK - evidence: the file exists` and `NOT OK - evidence: is non-vacuous (>= 40 lines)`. If either says `ok`, the file already exists — stop and report.

- [ ] **Step 3: Create the evidence file**

Create `skills/docket-build/references/gate-execution-evidence.md` with exactly this content. Every figure, version, and flag is **moved verbatim** from `skills/docket-build/references/gate-execution.md` — do not re-measure, re-word a number, or add a verdict token.

```markdown
# Gate execution — probe evidence

**This file is evidence, not instruction, and is not read before a gate run.** The rules an agent
needs at gate time — the six required capabilities, the mitigation, and each harness's verdict —
are in [`gate-execution.md`](gate-execution.md). This file records how those verdicts were
obtained, so a reader can judge what they are worth and re-probe when a version moves.

The same version scoping and re-probe caveats are carried as human verification items in
`docs/results/2026-08-07-the-build-gate-contract-never-states-an-execution-posture-for-results.md`,
the close-out record of change 0223, which produced these measurements.

## Method

Every verdict was measured on macOS 26.6.1 (arm64) with a stand-in gate that emits progress,
records that it started, sleeps 180s past any plausible turn boundary, and writes its terminal
sentinel **last**. Three properties of the probe are load-bearing, each having caught a false
verdict:

- **The sentinel is written last.** A gate that records its result first proves nothing about
  survival.
- **Observation happens from outside the harness**, after the harness process has exited. The
  harness's own report is never the evidence: on the failing runs below, every harness reported the
  launch as succeeding.
- **The launch call's duration is measured**, not only whether the artifact appears. A launch that
  blocks for the gate's full duration is a contract failure even though every artifact assertion
  passes. Survival is credited only when the launch call exited *before* the gate wrote its
  sentinel — a fixed threshold would not distinguish survival from a gate that merely finished
  first.

**`setsid(1)` is not installed on macOS**, so the launch shape cannot use it. The equivalent is
`POSIX::setsid(2)` called after a `fork` — the fork is mandatory, because a process-group leader
gets `EPERM`, which is the exact failure that made two of grooming's Codex runs inconclusive. All
four verdicts were measured with one identical launch shape: a `nohup`'d, fully-redirected,
backgrounded helper that forks, calls `setsid(2)` in the child, reopens all three streams onto a
durable log, and **does not let the parent return until the child has finished detaching**.

That last clause is the operative variable — the non-obvious precondition the kept surface states
as instruction — and it was established by a one-variable-per-run ladder rather than guessed:

- Same shape but with the parent returning immediately after the fork (a race): on Codex the gate
  **never started** — the child was killed within milliseconds, before `setsid(2)` could complete.
- Same shape with the new-session detach removed but the redirection kept: on Codex and on cursor
  the gate **started, then was killed mid-run**. Both harnesses reported success.
- Redirection additionally removed (streams left attached): on cursor, identical outcome — gate
  killed mid-run, and the launch call did **not** block (20s). This is the run that failed to
  reproduce the design spec's blocking claim, which is why capability 2 rests on durability alone.

So on both Codex and cursor the operative variable is the **new session**, not the redirection:
redirection alone does not save the gate, and a race-free new session does.

## Per-harness evidence

### claude — measured

`2.1.223 (Claude Code)`. Launch call returned in **0s**; the gate ran its full 180s and wrote the
sentinel, observed in a later, separate foreground call. Launch exited before the gate's terminal
write, and the gate ran as the leader of its own session, detached from the launching call.

The launch and the observation were performed as two separate foreground calls of one **interactive
live session**, so this row additionally measures capability 4 — in that mode, and only there.
Docket's own default path is not that mode: the gate runs inside `docket-build`, which is invoked
inline by the forked `docket-implement-next`, and a forked or dispatched agent has no channel on
which a resumption signal can arrive. On change 0223 itself, three dispatched build workers
backgrounded the suite and yielded, and none was resumed by the completion event.

The stricter variant that would have measured that mode — a non-interactive `claude -p` child
observed from a shell outside it — was **not obtainable on this machine**: the permission classifier
denied granting the child process Bash access (both `--allowedTools Bash` and the bypass flag),
while a plain `claude -p` with no tool grant runs fine. That denial is why the forked mode is
unmeasured.

### cursor — measured

`2026.08.04-aaa8809`, invoked with `--print --force --sandbox disabled`. Launch call returned in
**19s**; the gate ran its full 180s and wrote the sentinel; launch exited before the gate's terminal
write.

Two disambiguating runs: without a new session the gate is started and then killed mid-run whether
streams are redirected (17s launch) or left attached (20s launch). The attached-stream run is what
establishes that this version does **not** block the initiating call on an attached stream,
correcting the claim inherited from the design spec. This verdict supersedes the spec's `2026.01.23`
row, which was not re-usable.

### codex — measured

`codex-cli 0.146.1`, invoked with `exec --skip-git-repo-check --sandbox danger-full-access` (it
refuses to run outside a trusted directory otherwise). Launch call returned in **11s**; the gate ran
its full 180s and wrote the sentinel; launch exited before the gate's terminal write.

This is the harness that motivates capability 1's stronger reading. Codex runs the command under
`/bin/zsh -lc` and tears down that call's process group on return, reporting `succeeded in 0ms`
either way. With the racy detach the gate never started at all; with no detach it started and was
killed mid-run. Only the race-free new session survives.

### opencode — measured

`1.18.14`, invoked with `run`. Launch call returned in **5s** — the fastest of the four; the gate ran
its full 180s and wrote the sentinel; launch exited before the gate's terminal write. No
disambiguating run was needed: the standard shape succeeded first time, so nothing here establishes
whether opencode would also kill an un-detached gate the way Codex and cursor do.
```

- [ ] **Step 4: Run the guard to verify it now passes**

```bash
bash tests/test_gate_execution_posture.sh 2>&1 | grep -E "^(NOT )?ok - evidence"
```

Expected: both lines say `ok`.

- [ ] **Step 5: Add the budget row and its justification entry**

First measure the new file:

```bash
wc -l -w skills/docket-build/references/gate-execution-evidence.md
```

Apply the Global-Constraints rounding rule to those two actuals to get `<maxL>` and `<maxW>`.

Then in `tests/test_skill_size_budgets.sh`, append this entry **at the very end of the justification
comment block, on the line immediately above `BUDGETS="`** (currently line 631), filling in the
measured actuals and the numbers you derived:

```bash
# skills/docket-build/references/gate-execution-evidence.md is a NEW row added by change 0234, which
# split skills/docket-build/references/gate-execution.md along the instruction-vs-evidence axis: the
# § *Method* probe design, the one-variable-per-run ladder, the four launch durations, and the
# per-harness measurement narratives moved off a file that is read BLOCKING before every gate run.
# The WHERE-ELSE clause is not required for this row — that rule binds a RAISE only (see the top of
# this block), and this is a new file, the same reading change 0223 recorded for exactly this case.
# Recorded anyway: the two homes considered were 0223's docs/results/ record (rejected — a results
# file is a close-out record of a completed change, while this content must be rewritten whenever a
# harness version moves) and a new ADR (rejected — an Accepted ADR is immutable except its status
# line, the wrong lifecycle for a measurement). Set per the rounding rule above from the measured
# actuals: <L> lines -> <maxL>, <W> words -> <maxW>.
```

Then add the row to the `BUDGETS` block, keeping the alphabetical-by-path ordering (it goes
immediately **above** the `gate-execution.md` row, since `gate-execution-evidence.md` sorts first)
and the column alignment of its neighbours:

```
skills/docket-build/references/gate-execution-evidence.md   <maxL> <maxW>
```

- [ ] **Step 6: Run both test files**

```bash
bash tests/test_skill_size_budgets.sh 2>&1 | tail -5
bash tests/test_gate_execution_posture.sh 2>&1 | grep -c "^NOT OK"
```

Expected: `PASS` from the budgets file (in particular `ok - every skills/**/*.md has a budget row (unbudgeted:[])` and `ok - skills/docket-build/references/gate-execution-evidence.md within line budget`), and `0` from the posture file.

- [ ] **Step 7: Prove the new guards bite (mutation check)**

```bash
cp skills/docket-build/references/gate-execution-evidence.md /tmp/evid.bak
: > skills/docket-build/references/gate-execution-evidence.md
bash tests/test_gate_execution_posture.sh 2>&1 | grep -E "^(NOT )?ok - evidence"
cp /tmp/evid.bak skills/docket-build/references/gate-execution-evidence.md && rm -f /tmp/evid.bak
```

Expected: the emptied file leaves `ok - evidence: the file exists` but reddens
`NOT OK - evidence: is non-vacuous (>= 40 lines)` — the floor is what makes the pair non-vacuous.
Confirm the restore with `wc -l skills/docket-build/references/gate-execution-evidence.md` before
committing. (Restore by **copy**, not `git checkout --` — the file is untracked at this point and
`git checkout` would not bring it back; learnings: `mutation-restore-needs-a-backup-copy`.)

- [ ] **Step 8: Commit**

```bash
git add skills/docket-build/references/gate-execution-evidence.md tests/test_gate_execution_posture.sh tests/test_skill_size_budgets.sh
git commit -m "docs(0234): move gate-execution probe evidence to a non-blocking sibling"
```

---

### Task 2: Slim the kept surface, repoint its citations, and ratchet its budget

**Files:**
- Modify: `skills/docket-build/references/gate-execution.md` (delete `## Method`; compress the four `### <harness>` sections; repoint three citations; add the pointer)
- Modify: `tests/test_gate_execution_posture.sh` (append two more asserts to the group added in Task 1)
- Modify: `tests/test_skill_size_budgets.sh` (ratchet the `gate-execution.md` row; extend the justification entry)
- Test: `tests/test_gate_execution_posture.sh`, `tests/test_skill_size_budgets.sh`, full suite

**Interfaces:**
- Consumes: `skills/docket-build/references/gate-execution-evidence.md` from Task 1 — this task adds the pointer at it and deletes the content it now duplicates.
- Produces: the final kept surface. No later task depends on it.

- [ ] **Step 1: Write the two remaining failing asserts**

In `tests/test_gate_execution_posture.sh`, append these two asserts to the end of the group (11)
block added in Task 1 (still before the final `exit $fail`):

```bash
assert "reference: points at the evidence file" \
  'grep -qF "gate-execution-evidence.md" <<<"$ref_body"'
# ABSENCE assert, deliberately: a guard asserting a removed class is ABSENT cannot go stale, because
# the only way to redden it is to reintroduce the thing (learnings:
# restatement-accumulates-its-own-guards, the 0194 entry). A positive assert that the evidence file
# still CONTAINS the method section would instead pin a copy and rot on its next rewrite.
assert "reference: carries no Method section (evidence stays off the blocking surface)" \
  '! grep -qE "^## Method" <<<"$ref_body"'
```

- [ ] **Step 2: Run them to verify they fail**

```bash
bash tests/test_gate_execution_posture.sh 2>&1 | grep -E "^(NOT )?ok - reference: (points|carries)"
```

Expected: both `NOT OK` — the kept file neither names the sibling nor has lost its `## Method`
heading yet.

- [ ] **Step 3: Delete `## Method` from the kept surface**

In `skills/docket-build/references/gate-execution.md`, delete the whole `## Method` section —
from the line `## Method` (currently line 69) through the line immediately before `### claude`
(currently line 107), inclusive of the trailing blank line. Nothing else in that range survives; its
content is already in the sibling from Task 1.

Then replace the now-parentless run of harness sections with a heading of its own. Insert this line
where `## Method` was, followed by a blank line:

```markdown
## Per-harness verdicts
```

Do **not** use a `###` heading here: `tests/test_gate_execution_posture.sh` asserts that the set of
`^### [a-z-]+` headings in this file **equals** `HD_SHIPPED_HARNESSES`, so any additional `###`
heading reddens it.

- [ ] **Step 4: Replace the four harness sections with their compact forms**

Replace each `### <harness>` section body (heading through its `**Verdict:**` line) with exactly the
text below. Each keeps its version string and invocation flags (Global Constraints / spec A3) and
drops the durations, the ladder cross-references, and the narrative.

```markdown
### claude

`2.1.223 (Claude Code)`. Measured as two separate foreground calls of one **interactive** live
session, which is also the only row that establishes capability 4 — in that mode alone. Docket's own
default path is the **forked**/dispatched one, and that mode is **unmeasured** here; the verdict
claims nothing about it.

**Verdict:** `supported` — interactive session, two foreground calls; forked mode unmeasured

### cursor

`2026.08.04-aaa8809`, invoked with `--print --force --sandbox disabled`. Measured with the standard
race-free detaching launch shape; two further runs establish that this version does not block the
initiating call on an attached stream.

**Verdict:** `supported`

### codex

`codex-cli 0.146.1`, invoked with `exec --skip-git-repo-check --sandbox danger-full-access` (it
refuses to run outside a trusted directory otherwise). This is the harness that motivates capability
1's stronger reading: without a race-free new session the gate does not survive, so the verdict is
conditional on the launch shape rather than on the harness alone.

**Verdict:** `supported` — race-free new-session launch shape only

### opencode

`1.18.14`, invoked with `run`. Only the standard launch shape was probed, so whether opencode would
also kill an un-detached gate is unmeasured.

**Verdict:** `supported` — standard launch shape only; un-detached behavior unmeasured
```

Four guard shapes are load-bearing in the `claude` section and must survive any re-wording:

1. it contains the literal token `forked` — without it the (10c) loop `continue`s, `mode_secs` stays
   `0`, and the population floor reddens;
2. `forked` and `unmeasured` **co-occur inside one sentence** (`[^.]{0,120}`, either order) — no
   period may fall between them;
3. the word `interactive` appears **not** preceded by a hyphen (`(^|[^-])interactive`) — here it is
   preceded by `**`, which satisfies it;
4. its verdict line carries a **non-empty** ` — <scope>` clause after the backticked token.

- [ ] **Step 5: Repoint the three § *Method* citations**

Three references to § *Method* sit on the kept surface. Repoint each; a fourth (inside `### cursor`,
"Two disambiguating runs are recorded under *Method*") is already gone with Step 4's rewrite.

**(a) In the mitigation paragraph** (currently line 34) replace:

```
call returns**, or the harness's teardown wins the race.
```

…leaving the sentence's opening intact but changing the citation. The full repaired sentence reads:

```markdown
call returns**, or the harness's teardown wins the race. That precondition was measured, not
reasoned about; the measurement is in the evidence file linked at the end of this reference.
```

and the words `measured and recorded under *Method* below` earlier in that sentence become
`measured and recorded as evidence`.

**(b) In `## Reading a verdict`** (currently line 43) replace:

```markdown
**A verdict covers only what § *Method* measured — capabilities 1, 2 and 3**
```

with:

```markdown
**A verdict covers only what the probe measured — capabilities 1, 2 and 3**
```

> **Regex trap — do not put the filename in this sentence.** The assert is
> `grep -qiE "verdict[^.]{0,80}only[^.]{0,80}measur"` (`tests/test_gate_execution_posture.sh:328`).
> `[^.]` cannot cross a period, and `gate-execution-evidence.md` contains two of them, so writing
> the filename into this sentence breaks the window and reddens the assert. The pointer belongs in
> its own section (Step 6).

**(c) In the capability-4 bullet** (currently line 49) replace:

```markdown
- **Capability 4** is *not* measured by the standard probe — § *Method* observes from **outside**
```

with:

```markdown
- **Capability 4** is *not* measured by the standard probe — the probe observes from **outside**
```

Leave the capability 5 and 6 bullets byte-untouched: their
`capability N … (not|never|un(measured|observed|probed))` shapes are asserted individually.

- [ ] **Step 6: Add the non-blocking evidence pointer**

Append this section at the **end** of `skills/docket-build/references/gate-execution.md`:

```markdown
## Evidence

How each verdict above was obtained — the probe design, the one-variable-per-run ladder, the
measured launch durations, and the per-harness narratives — is in
[`gate-execution-evidence.md`](gate-execution-evidence.md). That file is **not read before a gate
run**: an agent about to start a suite needs the capabilities, the mitigation, and the verdicts on
this page, and nothing on that one. Read it when re-probing a harness whose version has moved, or
when judging what a verdict is worth.
```

- [ ] **Step 7: Run the posture guards — every assert, not only the new ones**

```bash
bash tests/test_gate_execution_posture.sh 2>&1 | grep "^NOT OK" ; echo "exit=$?"
```

Expected: no `NOT OK` lines (so `grep` prints nothing and `echo` reports `exit=1`, grep's
no-match status). If any assert reddens, apply the Global Constraints rule: a broken
`[^.]`-bounded citation window is a **prose** defect — repair the wording; anything else means the
split was drawn wrong — stop and report. In particular confirm these stay green:

```bash
bash tests/test_gate_execution_posture.sh 2>&1 | grep -E "^ok - (reference: is non-vacuous|reference: enumerates exactly 6|verdict scope: a verdict covers ONLY|verdict scope: capability|modes: |verdicts: )"
```

- [ ] **Step 8: Ratchet the `gate-execution.md` budget row**

Measure the slimmed file:

```bash
wc -l -w skills/docket-build/references/gate-execution.md
```

Apply the Global-Constraints rounding rule to both actuals. In `tests/test_skill_size_budgets.sh`,
edit the existing row (currently line 632, `175 1650`) to the new, **lower** numbers, preserving the
column alignment:

```
skills/docket-build/references/gate-execution.md            <maxL> <maxW>
```

Then extend this change's justification entry (added in Task 1, at the end of the comment block)
with the ratchet's reasoning, filling in the measured actuals and derived numbers:

```bash
# The same change RATCHETED skills/docket-build/references/gate-execution.md DOWN, 175/1650 ->
# <maxL>/<maxW>. A lowering needs no where-else clause either (that rule binds a raise), but the
# ratchet is the discretionary half of 0234 and so is argued here: the file's defect was ACCUMULATED
# evidence on a blocking-read surface, and leaving ~90 lines of headroom would leave the split
# unenforced — the evidence would simply drift back. Per size-target-is-direction the number is a
# direction, and the working margin the rounding rule leaves is the intended slack; a later change
# that genuinely needs the room raises the row in-diff with its own justification, which is exactly
# the audit trail wanted. Set per the rounding rule above from the measured actuals: <L> lines ->
# <maxL>, <W> words -> <maxW>.
```

- [ ] **Step 9: Prove the ratchet bites, then run the full suite**

```bash
cp skills/docket-build/references/gate-execution.md /tmp/ref.bak
cat /tmp/ref.bak /tmp/ref.bak > skills/docket-build/references/gate-execution.md
bash tests/test_skill_size_budgets.sh 2>&1 | grep "gate-execution.md within"
cp /tmp/ref.bak skills/docket-build/references/gate-execution.md && rm -f /tmp/ref.bak
```

Expected: the doubled file reddens `NOT OK - skills/docket-build/references/gate-execution.md within
line budget`. Confirm the restore with `wc -l skills/docket-build/references/gate-execution.md`
(it must match the Step 8 measurement) before continuing.

Then run the whole suite:

```bash
scripts/run-tests.sh
```

Expected: green. This is a docs-and-guards change; any red file outside the two edited here means an
unnoticed dependent — investigate before committing, and do not weaken the assert.

- [ ] **Step 10: Commit**

```bash
git add skills/docket-build/references/gate-execution.md tests/test_gate_execution_posture.sh tests/test_skill_size_budgets.sh
git commit -m "refactor(0234): slim gate-execution.md to instruction and ratchet its budget"
```

---

## Verification checklist

- [ ] `skills/docket-build/references/gate-execution.md` contains no `## Method` heading, no launch
      duration (`0s` / `19s` / `11s` / `5s`), no `setsid`/`nohup`/`fork` mechanics, and no
      permission-classifier narrative.
- [ ] It still contains: six numbered capabilities, the mitigation **with its "fully established
      before the initiating call returns" precondition**, the whole `## Reading a verdict` section,
      four `### <harness>` sections each with a version string and a legal verdict line, and the
      `## Evidence` pointer.
- [ ] The set of `^### ` headings in the kept file is exactly `claude cursor codex opencode`
      (`grep -n '^### ' skills/docket-build/references/gate-execution.md`).
- [ ] `bash tests/test_gate_execution_posture.sh` prints no `NOT OK`, and no pre-existing assert in
      that file was edited (`git diff origin/main -- tests/test_gate_execution_posture.sh` shows
      additions only).
- [ ] `bash tests/test_skill_size_budgets.sh` prints `PASS`, with both edited rows set from measured
      actuals by the rounding rule and each carrying an in-diff justification.
- [ ] `scripts/run-tests.sh` is green.
- [ ] No file outside the four in the File Structure table is modified
      (`git diff --name-only origin/main`).
