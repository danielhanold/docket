<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0224 — The build gate contract never says green/red is the exit code, so an output-shape match passes as a gate](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0224-the-build-gate-contract-never-says-green-red-is-the-exit-code.md)**
<!-- docket:backlink:end -->

# Build-gate verdict is the exit status — Implementation Plan (change 0224)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** State normatively in `skills/docket-build/SKILL.md` § *The build gate* that the gate's green/red verdict is the suite command's **exit status** — never its output text — and guard that clause in `tests/test_docket_build.sh`.

**Architecture:** Three coupled edits across two tasks. Task 1 inserts one prose paragraph into the build-gate section and raises the file's size-budget row in the same diff (the file has only 8 lines / 62 words of headroom, so the clause does not fit without the raise). Task 2 adds a change-0224 banner to the existing `tests/test_docket_build.sh` — a `/^#+ /`-terminated slice of § *The build gate*, whitespace-flattened before phrase matching, with a non-vacuity companion and one independently mutation-tested assert per rule.

**Tech Stack:** Bash 4.4+, POSIX ERE via `grep -E`, `awk` section slicing, markdown contract prose. No application code — this is a contract-prose change with docs guards.

## Global Constraints

These bind **every** task. Values are copied verbatim from the spec and from the repo's own house rules (`AGENTS.md`, the learnings ledger).

- **Feature worktree:** `/Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code`, branch `feat/the-build-gate-contract-never-says-green-red-is-the-exit-code`. All paths below are relative to it. Never write docket metadata (`docs/changes/**`, `docs/adrs/**`, `BOARD.md`) from this worktree.
- **Section terminator is `/^#+ /`, never `/^## /`.** The level-2 form would swallow the `### Gate execution posture` subsection that `tests/test_gate_execution_posture.sh` separately owns, letting a bounded-gap assert match across sections and survive its own mutation. (learnings: `section-slice-needs-a-named-terminator`)
- **Flatten whitespace before any phrase match.** `tests/test_docket_build.sh` already defines `flat(){ tr -s '[:space:]' ' ' <<<"$1"; }` at line 16. Use it. A phrase grep over hard-wrapped prose otherwise doubles as a line-wrap guard. (learnings: `phrase-grep-over-wrapped-prose`)
- **At most ONE bounded gap (`[^.]{0,N}`) per ERE.** Two or more backtrack catastrophically on non-matching input, so the mutation test **hangs instead of reddening**. Where a claim has three anchored parts, write two asserts with one gap each. (learnings: `stacked-gap-regex-hangs-instead-of-failing`)
- **Mutation restore is `cp` backup, NEVER `git checkout -- <file>`.** `git checkout --` restores to HEAD or to the index, destroying the uncommitted edit under test and producing a large, meaningless red reading. Use `cp "$f" "$f.bak"` … `mv "$f.bak" "$f"`. (learnings: `mutation-restore-needs-a-backup-copy`)
- **Prove every mutation actually landed** with `grep -c` before and after, taken through a whitespace-flattened copy. A substitution that silently fails to match yields a green run with nothing mutated, which reads exactly like a robust guard. (learnings: `assert-detects-removal-not-replacement`)
- **A non-vacuity companion runs through the SAME extractor.** If the `awk` slice breaks or the heading is renamed, something must redden. (learnings: `assert-detects-removal-not-replacement` rule 5, `marker-scoped-guard-needs-a-population-floor`)
- **Never anchor an assert on a bare common noun** (`exit`, `gate`, `green`). Anchor on a verbatim slice of the claim.
- **Verification commands run under explicit `bash`, and greps under `command grep` or `git grep`.** The agent's interactive shell is zsh and its `grep` is ugrep; a sweep can iterate zero times and a grep can match nothing while both print success. (learnings: `agent-shell-noop-reads-as-success`)
- **Do not touch the `docket:build-evidence` record schema.** Change 0190 is open on that surface (spec assumption 3).
- **Do not edit `skills/docket-finalize-change/SKILL.md`.** Its `configured-bash-finalize` block already *is* the exit-status test; restating the rule there grows a second set of guards over the copy (spec assumption 2, learnings: `restatement-accumulates-its-own-guards`).
- **Do not state any rule about what a suite *runner* should exit** for a non-failure condition. The gate reads bare non-zero and must not learn an exit-code taxonomy (spec edit 1, *Deliberately not stated*).

---

## File Structure

| File | Change | Responsibility after the change |
|---|---|---|
| `skills/docket-build/SKILL.md` | Modify — insert one paragraph between line 200 (`copy that fragment into this file.`) and line 202 (`**Green** →`) | Owns the build gate contract, now including **what decides** green/red, not only what green and red *do* |
| `tests/test_skill_size_budgets.sh` | Modify — one `BUDGETS` row + its in-diff justification comment | Regrowth guard; the raised row records why the prose cannot live in `references/` |
| `tests/test_docket_build.sh` | Modify — append a change-0224 banner block before the final `if [ "$fail" = 0 ]` line | Owns the build gate's contract guards, now including the verdict rule |

No new files. The spec's edit 4 ("per-file-loop confirmation") is not a separate deliverable — it is one sentence in Task 1 plus assert (d) in Task 2.

---

### Task 1: The verdict clause and its size-budget raise

Both edits land together because the clause does not fit the current row: `skills/docket-build/SKILL.md` measures **317 lines / 2938 words** against row `skills/docket-build/SKILL.md 325 3000`, i.e. 8 lines / 62 words of headroom against a ~11-line / ~140-word addition. Splitting them would leave `tests/test_skill_size_budgets.sh` red between two commits.

**Files:**
- Modify: `skills/docket-build/SKILL.md:200-202` (insert between them)
- Modify: `tests/test_skill_size_budgets.sh` — the `skills/docket-build/SKILL.md` row in `BUDGETS`, plus a justification comment appended to the block of `#` comments above `BUDGETS="`

**Interfaces:**
- Consumes: nothing from earlier tasks (this is Task 1).
- Produces: the prose Task 2's asserts key on. The exact phrases Task 2 will grep, verbatim and case-insensitively, are: `green if and only if the resolved suite command exits zero`; `diagnostic only`; `a gate that reads its verdict out of the output is not a gate`; `terminal result artifact`; `completed successfully`; `are not statuses`; `loop over per-file commands`; `aggregate`; `including the repair worker's post-fix re-run`.

- [ ] **Step 1: Confirm the insertion point is still where the plan says**

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
command grep -n '^## The build gate$\|^copy that fragment into this file\.$\|^\*\*Green\*\* →\|^### Gate execution posture$' skills/docket-build/SKILL.md
```

Expected, in this order: `185:## The build gate`, `200:copy that fragment into this file.`, `202:**Green** →`, `228:### Gate execution posture`.

If the line numbers differ but the four anchors still appear in that relative order, proceed using the anchors, not the numbers. If the anchors themselves are gone, STOP and report — the spec's assumption 7 escape hatch has fired and the placement must be re-derived.

- [ ] **Step 2: Record the pre-edit measurement**

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
wc -lw skills/docket-build/SKILL.md
command grep -n 'skills/docket-build/SKILL.md  *[0-9]' tests/test_skill_size_budgets.sh
```

Expected: `317 2938 skills/docket-build/SKILL.md`, and the row `skills/docket-build/SKILL.md                               325 3000`.

Write both numbers down. Step 6 sets the new row from the file's **own re-measured actual**, never from this plan's estimate.

- [ ] **Step 3: Insert the verdict clause**

In `skills/docket-build/SKILL.md`, insert this paragraph — preceded and followed by exactly one blank line — between the paragraph ending `copy that fragment into this file.` and the paragraph beginning `**Green** →`:

```markdown
The verdict is an **exit status, never output text**. A run is **green if and only if the resolved
suite command exits zero**; any non-zero status is not green. A `PASS`/`FAIL` line, a summary count,
or a progress ticker is **diagnostic only** — a gate that reads its verdict out of the output is not
a gate. The deciding status is the one recorded in the **terminal result artifact** that *Gate
execution posture* requires, and that recorded status is what **completed successfully** means:
*still running* and *result unavailable* are not statuses at all, so they stay budget halts and are
never red. When the resolved command is a **loop over per-file commands** — the shape finalize's
`configured-bash-finalize` block takes when `FINALIZE_TEST_COMMAND` is unset — the deciding status
is the **aggregate** that block exits with, never any individual file's. This rule binds every
full-suite run this role performs, including the repair worker's post-fix re-run below.
```

Change nothing else in the file. In particular, do **not** reword the configuration-gap item (item 3, line ~192) or the observation-budget clauses in `### Gate execution posture` — neither is a suite run that produced a status, so neither is red, and the new wording must not reclassify them.

- [ ] **Step 4: Verify the clause landed in the right slice**

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
bash -c 'awk "/^## The build gate\$/{f=1;next} f && /^#+ /{f=0} f" skills/docket-build/SKILL.md \
  | tr -s "[:space:]" " " \
  | command grep -c "green if and only if the resolved suite command exits zero"'
```

Expected: `1`. A `0` means the paragraph landed outside the section slice (most likely below `### Gate execution posture`) — move it and re-run.

- [ ] **Step 5: Confirm the adjacent guard is undisturbed**

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
bash tests/test_gate_execution_posture.sh; echo "rc=$?"
```

Expected: `PASS` and `rc=0`. This suite owns the `### Gate execution posture` slice that sits immediately after the insertion point; a paragraph that drifted into it would redden here.

- [ ] **Step 6: Re-measure and set the new budget row**

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
wc -lw skills/docket-build/SKILL.md
```

Apply `tests/test_skill_size_budgets.sh`'s own documented raise rule to **these** numbers:

1. Round **lines** up to the next multiple of 5.
2. Round **words** up to the next multiple of 50.
3. **If either rounded value lands within 25 of the actual, take the multiple after it.** (For lines, the file's own history reads a 3-line margin as the near-zero failure mode and steps up; apply the same reading.)

Worked example against the expected post-edit actuals of ~328 lines / ~3080 words: 328 → next multiple of 5 is 330 (2 lines of margin — near-zero, so step to **335**); 3080 → next multiple of 50 is 3100 (20 words, inside the 25-word threshold, so step to **3150**). **Use your own measurement, not this example.**

Edit the row in `BUDGETS`, preserving the file's column alignment:

```
skills/docket-build/SKILL.md                               335 3150
```

- [ ] **Step 7: Append the raise justification comment**

The file's rule (change 0201) requires a raise to **name** the `references/` file the prose was considered for and **argue** why it cannot live there. Append this comment immediately after the last existing `# skills/…` entry and before `BUDGETS="`, with the numbers corrected to your Step 6 measurement:

```bash
# skills/docket-build/SKILL.md's budget was raised 325/3000 -> 335/3150 by change 0224, which states
# the build gate's verdict rule normatively: green iff the resolved suite command exits zero, with
# output text diagnostic only, the deciding status read from the terminal result artifact, the
# per-file-loop aggregate named, and the repair re-run bound by the same rule. The two references/
# files under skills/docket-build/ were both considered and neither can hold it.
# references/gate-execution.md is quarantined per-harness capability and probe evidence, read ONCE
# before the gate starts; this rule must be in hand at the moment the verdict is formed, in the
# section that already states what green and red DO. Splitting "what decides green" from "what green
# does" across two files is precisely the drift that produced the gap this change closes — the
# section defined both meanings and never their determinant. references/task-routing.md is the
# profile-selection rubric shared with docket-implement-next's fix loop and has nothing to do with
# the gate. Set per the rounding rule above from the measured actuals: 328 lines -> the next multiple
# of 5 is 330, which leaves 2 lines — the near-zero mode this block repeatedly records raising past —
# so the multiple after: 335. 3080 words -> the next multiple of 50 is 3100, a 20-word margin inside
# the 25-word threshold, so the multiple after: 3150.
```

- [ ] **Step 8: Run the budget suite**

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
bash tests/test_skill_size_budgets.sh; echo "rc=$?"
```

Expected: `PASS` and `rc=0`. A `FAIL` naming `skills/docket-build/SKILL.md` means the row is still below the actual — re-measure and redo Step 6.

- [ ] **Step 9: Run the build-contract suite (should still be green — no asserts added yet)**

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
bash tests/test_docket_build.sh; echo "rc=$?"
```

Expected: `PASS` and `rc=0`. The clause is additive prose; nothing existing keys on its absence.

- [ ] **Step 10: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
git add skills/docket-build/SKILL.md tests/test_skill_size_budgets.sh
git commit -m "docs(0224): the build gate's verdict is the exit status, never output text

State normatively in skills/docket-build/SKILL.md § The build gate that a run is
green iff the resolved suite command exits zero; output text is diagnostic only.
The deciding status is the one recorded in the terminal result artifact, which is
where 'completed successfully' is settled — still running and result unavailable
stay budget halts, never red. Under a per-file loop the aggregate decides. The
rule binds every full-suite run this role performs, including the repair worker's
post-fix re-run.

Raise the file's size-budget row in the same diff; the clause does not fit the
prior row, and the raise names references/gate-execution.md as the considered
home and argues why the rule cannot live there."
```

---

### Task 2: Guard the verdict clause in `tests/test_docket_build.sh`

The guard goes in the **existing** file, not a new one: `test_docket_build.sh` already owns the build gate's contract prose (the `FINALIZE_TEST_COMMAND` derivation, the `configured-bash-finalize` citation, the configuration-gap classification, the repair ladder), and this is one clause in that same section. A new file would add a suite entry and a runtime-budget row for nothing (spec assumption 1).

**Files:**
- Modify: `tests/test_docket_build.sh` — append a banner block immediately before the final `if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi` line
- Read-only: `skills/docket-build/SKILL.md` (the artifact under guard, written in Task 1)

**Interfaces:**
- Consumes from Task 1: the verdict paragraph in § *The build gate*, and the phrase list in Task 1's *Produces* block. Also consumes two pre-existing definitions already in the test file — `ctrl_body` (line 177, the full `skills/docket-build/SKILL.md` text) and `flat()` (line 16, the whitespace collapser). Do **not** redefine either.
- Produces: nothing later tasks depend on. This is the last task.

- [ ] **Step 1: Write the failing guard block**

Append this block to `tests/test_docket_build.sh`, immediately **before** the final `if [ "$fail" = 0 ]; …` line:

```bash
# ---------------------------------------------------------------------------
# Change 0224: the gate's VERDICT is the exit status, never output text
# ---------------------------------------------------------------------------
# § The build gate defined what green and red MEAN but never what DETERMINES which one a run is,
# so a gate keyed on an output-shape match (`tail -1 | grep PASS`) satisfied the contract and could
# mint a valid-looking build-evidence record for a branch nobody verified. These asserts pin the
# determinant.
#
# Terminator is /^#+ /, NOT /^## /: the level-2 form would swallow ### Gate execution posture,
# which tests/test_gate_execution_posture.sh separately owns, and a bounded-gap assert over that
# wider slice could match across sections and survive its own mutation
# (learnings: section-slice-needs-a-named-terminator).
gate_blk="$(awk '/^## The build gate$/{f=1;next} f && /^#+ /{f=0} f' <<<"$ctrl_body")"
# Flatten before phrase matching so a pure re-flow of the hard-wrapped paragraph does not redden
# asserts about policy that never changed (learnings: phrase-grep-over-wrapped-prose).
gate_flat="$(flat "$gate_blk")"

# Non-vacuity companion through the SAME extractor. Without it, a renamed heading or a broken awk
# range empties $gate_blk and turns every assert below into a permanent green. The anchor is a
# clause that PREDATES this change and sits inside the slice, so it cannot be satisfied by the
# prose this change just added (learnings: assert-detects-removal-not-replacement, rule 5).
assert "0224: the build gate section slice is non-vacuous" \
  '[ "$(grep -c . <<<"$gate_blk")" -ge 20 ]'
assert "0224: the build gate slice extractor still resolves (pre-existing clause present)" \
  'grep -qiF -- "configuration gap, not a red suite" <<<"$gate_flat"'

# (a) The iff — an exit status decides green. Anchored on a verbatim slice of the claim, never on a
# bare common noun like "exit" or "gate" (learnings: assert-detects-removal-not-replacement, #226).
assert "0224: green is defined as the suite command exiting zero" \
  'grep -qiF -- "green if and only if the resolved suite command exits zero" <<<"$gate_flat"'

# (b) The negative — output text is not the verdict. Two separate asserts rather than one ERE with
# two bounded gaps: stacked gaps backtrack catastrophically on NON-matching input, so the mutation
# test hangs instead of reddening (learnings: stacked-gap-regex-hangs-instead-of-failing).
assert "0224: output text is classified as diagnostic, not the verdict" \
  'grep -qiF -- "diagnostic only" <<<"$gate_flat"'
assert "0224: reading the verdict out of the output is named as not a gate" \
  'grep -qiF -- "reads its verdict out of the output is not a gate" <<<"$gate_flat"'

# (c) The verdict is read from the terminal result artifact — which is also where the definition of
# "completed successfully" that references/gate-execution.md capability 5 requires finally lands —
# and the two non-statuses stay halts. One bounded gap in the ERE, not two.
assert "0224: the deciding status is the one in the terminal result artifact" \
  'grep -qiE "deciding status[^.]{0,120}terminal result artifact" <<<"$gate_flat"'
assert "0224: the recorded status is what completed successfully means" \
  'grep -qiF -- "completed successfully" <<<"$gate_flat"'
assert "0224: still running and result unavailable are not statuses" \
  'grep -qiE "result unavailable are not statuses[^.]{0,120}never red" <<<"$gate_flat"'

# (d) The per-file-loop aggregate. Confirmed against finalize's configured-bash-finalize block,
# which accumulates suite_status=1 per failing file and exits on [ "$suite_status" -eq 0 ] — the
# aggregate IS the status, so the wording holds unchanged.
assert "0224: under a per-file loop the aggregate is the deciding status" \
  'grep -qiE "loop over per-file commands[^.]{0,220}aggregate" <<<"$gate_flat"'

# (e) The rule binds the repair worker's post-fix re-run, whose green is what ends the ladder.
assert "0224: the rule binds the repair worker's post-fix re-run" \
  'grep -qiF -- "including the repair worker'"'"'s post-fix re-run" <<<"$gate_flat"'
```

- [ ] **Step 2: Run the suite to verify the new asserts pass against the Task 1 prose**

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
bash tests/test_docket_build.sh; echo "rc=$?"
```

Expected: `PASS`, `rc=0`, and nine new `ok - 0224:` lines.

If any `0224:` assert reports `NOT OK`, the phrase in `skills/docket-build/SKILL.md` does not match the pattern. Fix the **assert** to match the prose Task 1 actually wrote — do not reword the contract to satisfy a grep, except where the assert has found a genuine omission.

- [ ] **Step 3: Stage the work so it survives the mutation cycle**

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
git add tests/test_docket_build.sh
git status --short
```

Expected: `M  tests/test_docket_build.sh` staged. Task 1's edits are already committed.

This is belt-and-braces only. Step 4 restores from a `cp` backup regardless — `git checkout -- <file>` is **never** the restore step, because it reads HEAD or the index and would silently destroy the uncommitted edit under test (learnings: `mutation-restore-needs-a-backup-copy`).

- [ ] **Step 4: Mutation-test each assert, one clause at a time**

Run this loop. It backs the file up, deletes exactly one guarded phrase, **proves the deletion landed** with a flattened `grep -c` before and after, runs the suite, and restores from the backup copy.

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
bash -c '
F=skills/docket-build/SKILL.md
flatcount(){ tr -s "[:space:]" " " < "$F" | command grep -c "$1" || true; }
while IFS="|" read -r label phrase; do
  [ -n "$label" ] || continue
  cp "$F" "$F.bak"
  before="$(flatcount "$phrase")"
  # Delete the whole paragraph the phrase lives in, then confirm the phrase is gone.
  perl -0pi -e "s/\Qgreen if and only if\E/GREEN-IF-ONLY-IF-REMOVED/g if 0" "$F"
  perl -0pi -e "my \$p = quotemeta(q{$phrase}); s/\$p//gs" "$F" 2>/dev/null || true
  after="$(flatcount "$phrase")"
  echo "=== $label : flattened count before=$before after=$after"
  if [ "$before" -eq 0 ] || [ "$after" -ne 0 ]; then
    echo "!!! MUTATION DID NOT LAND for: $label — reading is meaningless, fix the probe"
  else
    bash tests/test_docket_build.sh > /tmp/mut.out 2>&1
    echo "    suite rc=$? ; matching asserts:"
    command grep "NOT OK - 0224" /tmp/mut.out || echo "    !!! NOTHING REDDENED — this assert is decoration"
  fi
  mv "$F.bak" "$F"
done <<EOF
(a) iff exit zero|green if and only if the resolved suite command exits zero
(b1) diagnostic only|diagnostic only
(b2) output is not a gate|reads its verdict out of the output is not
(c1) terminal result artifact|terminal result artifact
(c2) completed successfully|completed successfully
(c3) not statuses|are not statuses at all
(d) per-file loop aggregate|loop over per-file commands
(e) repair re-run|including the repair worker
EOF
'
```

Note the phrase for (b2) and (e) is deliberately truncated before the apostrophe — `perl`'s `quotemeta` handles it, but keeping the probe phrase apostrophe-free avoids a quoting trap in the heredoc.

**For each row, exactly one thing must be true:** the mutation landed (`before` non-zero, `after` zero) **and** at least one `NOT OK - 0224:` line appeared. Any row printing `MUTATION DID NOT LAND` or `NOTHING REDDENED` is a defect in the probe or in the assert — fix it and re-run that row. An assert never seen red against its own mutation is decoration.

Because the paragraph is one block, deleting one phrase may redden more than one `0224:` assert. That is fine. What is not fine is a row reddening **zero** of them.

- [ ] **Step 5: Confirm the file is fully restored**

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
git status --short
ls skills/docket-build/SKILL.md.bak 2>/dev/null && echo "!!! leftover backup — remove it"
bash tests/test_docket_build.sh; echo "rc=$?"
bash tests/test_skill_size_budgets.sh; echo "rc=$?"
```

Expected: `git status --short` shows only `M  tests/test_docket_build.sh` (staged), no `.bak` file, and both suites `PASS` with `rc=0`. A modified `skills/docket-build/SKILL.md` here means a mutation was not restored — restore it with `git checkout -- skills/docket-build/SKILL.md` (safe **now**, because Task 1 committed that file; it was unsafe during Step 4 only).

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
git add tests/test_docket_build.sh
git commit -m "test(0224): guard the build gate's exit-status verdict rule

Add a change-0224 banner to tests/test_docket_build.sh: a /^#+ /-terminated
slice of § The build gate, whitespace-flattened before phrase matching, with a
non-vacuity companion through the same extractor anchored on a pre-existing
clause, and one independently mutation-tested assert per rule — the iff, the
output-is-not-the-verdict negative, the terminal-result-artifact source and the
two non-statuses, the per-file-loop aggregate, and the repair re-run binding.

At most one bounded gap per ERE: stacked gaps backtrack catastrophically on
non-matching input, so the mutation test would hang instead of reddening.
Each assert was mutation-tested individually with grep -c confirming the
mutation landed, restoring from a cp backup rather than git checkout --."
```

- [ ] **Step 7: Run the full suite**

```bash
cd /Users/homer/dev/docket/.worktrees/the-build-gate-contract-never-says-green-red-is-the-exit-code
bash scripts/run-tests.sh
```

Expected: green. Note for the gate: the verdict is this command's **exit status**, not any line of its output — which is the rule this change exists to state.

---

## Self-Review

**1. Spec coverage.**

| Spec item | Task |
|---|---|
| Edit 1 — verdict clause in § *The build gate*, in the named slot | Task 1, Steps 1–4 |
| Edit 1 — placement between the `configured-bash-finalize` paragraph and `**Green** →`, not a new `###` | Task 1, Steps 1 and 3 (paragraph, no heading) |
| Edit 1 — existing carve-outs (configuration gap, observation budget) untouched | Task 1, Step 3 closing paragraph; Task 1, Step 5 |
| Edit 1 — *Deliberately not stated*: no runner exit-code taxonomy | Global Constraints, final bullet |
| Edit 2 — guard in the existing `tests/test_docket_build.sh` under a 0224 banner | Task 2, Step 1 |
| Edit 2 — `/^#+ /` terminator | Global Constraints; Task 2, Step 1 |
| Edit 2 — flatten before phrase matching | Global Constraints; Task 2, Step 1 (`gate_flat`) |
| Edit 2 — non-vacuity companion through the same extractor | Task 2, Step 1 (two asserts, one anchored on `configuration gap, not a red suite`) |
| Edit 2 — five rules asserted separately (a)–(e) | Task 2, Step 1 (nine asserts across the five rules) |
| Edit 2 — at most one bounded gap per ERE | Global Constraints; Task 2, Step 1 (b split into two asserts) |
| Edit 2 — no bare-common-noun anchors | Task 2, Step 1 comments |
| Edit 2 — mutation-test each assert, `grep -c` before/after, `cp` backup restore | Task 2, Steps 3–5 |
| Edit 3 — size-budget raise in the same diff | Task 1, Steps 2, 6, 8 (same commit as the prose) |
| Edit 3 — documented rounding rule with the within-25 clause | Task 1, Step 6 |
| Edit 3 — name the `references/` candidate and argue against it | Task 1, Step 7 |
| Edit 4 — per-file-loop confirmation | Task 1, Step 3 (loop sentence) + Task 2, Step 1 assert (d) |
| Verification — `test_docket_build.sh`, `test_skill_size_budgets.sh`, `test_gate_execution_posture.sh`, full suite | Task 1 Steps 5/8/9; Task 2 Steps 2/5/7 |

No gaps.

**2. Placeholder scan.** No `TBD`, no "handle edge cases", no "similar to Task N". Every code step carries the literal text to write. The one deliberate deferral — the exact budget numbers — is not a placeholder but the spec's own instruction that the implementer sets the row from **its own** measurement; the procedure, the rounding rule, and a worked example are all given.

**3. Type consistency.** The nine assert patterns in Task 2 are each a substring of the paragraph written in Task 1, cross-checked phrase by phrase against Task 1's *Produces* block. Shell identifiers: `gate_blk` and `gate_flat` are new and unique in the file; `ctrl_body`, `flat`, `assert`, `fail`, and `REPO` are pre-existing and are consumed, never redefined.
