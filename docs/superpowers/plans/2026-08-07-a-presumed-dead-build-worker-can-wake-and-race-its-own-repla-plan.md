<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0231 — A presumed-dead build worker can wake and race its own replacement in one worktree](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0231-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla.md)**
<!-- docket:backlink:end -->

# Never Discard-and-Re-Dispatch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make "discard a dispatched worker's tree and dispatch a fresh worker for the same task" an explicit prohibition in both controller contracts and widen the worker's amend ban to its own commits, so a presumed-dead worker that wakes can never race its replacement in one worktree.

**Architecture:** Four prose edits to three normative skill files, each stated in its own file's vocabulary rather than pointed at, plus mutation-tested guard asserts in the existing `tests/test_docket_build.sh`. No new field, status, script, or mechanism — the rule is prose enforced by guards and review, which is docket's standing posture for contract rules. The trigger is an event a controller can actually observe (control returned without a schema-valid outcome), never elapsed time.

**Tech Stack:** Markdown skill contracts; Bash 4.4+ test guards (`set -uo pipefail`, `grep -E`, `awk`); `scripts/run-tests.sh` as the suite runner.

## Global Constraints

Copied from the spec and from `AGENTS.md`; every task below implicitly carries these.

- **A guard is code: mutation-test it** — strip the thing it guards, watch it redden, and confirm the mutation actually landed with `grep -c` before and after. A mutation that leaves an assert green is a defect until proven otherwise.
- **Assert the state you REMOVED, not the one you added**, wherever a change replaces existing wording. Every absence assert needs a live non-vacuity companion reading through the **same extractor**, so a broken extractor reddens something.
- **Anchor every prose assert on the producing region** — the bullet, the section, the paragraph that performs the write — never on a whole-file grep. A whole-file OR-grep for a concept matches body prose, headings, and examples.
- **Collapse whitespace before matching a phrase** against hard-wrapped markdown, so a pure re-flow cannot redden an assert about policy that never changed. Collapse runs of whitespace, not only newlines.
- **ERE repetition counts must stay at or below 255.** `/usr/bin/grep` rejects `{0,300}` with "maximum repetition exceeds 255" while the PATH `grep` (ugrep) accepts it, so a too-large bound passes locally and fails on a stock BSD toolchain.
- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`) under `set -o pipefail`. Capture into a variable first, then `grep <<<"$var"`.
- **No backticks in an `assert` description.** `tests/test_docket_build.sh`'s `assert` helper interpolates `$1` into a double-quoted string, so a backtick in the description is command substitution that runs.
- **A cross-reference in maintained source anchors on a symbol name or a verbatim-quoted clause**, never on a line number.
- **Raising a row in `tests/runtime-budgets.tsv` is not in scope.** `tests/test_docket_build.sh` is budgeted at 10s and these are grep-only asserts.
- **A size-budget raise in `tests/test_skill_size_budgets.sh` is permitted and ritualized.** It must (a) be an in-diff edit in the same commit that grows the file, (b) set lines to the next multiple of 5 and words to the next multiple of 50 above the re-measured actual — and if that lands within 25 words of the actual, the multiple **after** it, (c) carry a comment naming the `references/` file the new prose was considered for and stating why it cannot live there, and (d) leave real headroom: near-zero margin is the documented failure mode this ritual exists to prevent, not a tight fit to be proud of.
- **Do not claim the rule "reads onto" `docket-finalize-change`.** That skill neither loads nor references `docket-build`, and it has no discard-and-re-dispatch path. That sentence must not reach the built text.
- **Every edit is additive**, so it composes at rebase with the concurrently open changes touching the same files (0190, 0224, 0232).

## File Structure

| File | Responsibility in this change |
|---|---|
| `skills/docket-build/SKILL.md` | The controller contract. Two edits: the *A worker return is malformed or unverifiable* halting bullet gains the sibling prohibition; § *Dispatching a task* extends its concurrency ban to a controller that believes the first worker is gone. |
| `skills/docket-build-task/SKILL.md` | The worker contract. One edit: § *Scope*'s amend ban widens from "earlier task commits" to any commit, including one this worker just made. |
| `skills/docket-implement-next/references/fix-loop.md` | Step 6's fix loop, a second controller that dispatches workers directly and never loads `docket-build`. One edit: the same prohibition in **its own** abort-and-report vocabulary. |
| `tests/test_docket_build.sh` | All guards for all three surfaces. Gains one whitespace-collapse helper and three region extractors. Already defines `$CTRL`, `$WORKER`, and `$IMPL_FIX`. |
| `tests/test_skill_size_budgets.sh` | Budget rows for the two grown skill files, each raise carrying its rationale comment. |

**Task boundaries:** one task per contract surface, because a reviewer could reject the controller edit while approving the worker edit. Each task carries its own guards **and** its own size-budget raise — the raise must be in the same diff that grows the file, so it cannot be deferred to a fourth task.

---

### Task 1: The controller contract — `docket-build`

**Files:**
- Modify: `skills/docket-build/SKILL.md` (§ *Halting conditions*, the *A worker return is malformed or unverifiable* bullet; and § *Dispatching a task*)
- Modify: `tests/test_skill_size_budgets.sh` (the `skills/docket-build/SKILL.md` row, if the re-measured actual leaves under 5 lines or 50 words of headroom)
- Test: `tests/test_docket_build.sh`

**Interfaces:**
- Consumes: nothing from an earlier task — this is the first.
- Produces: the shell function `flat()` in `tests/test_docket_build.sh`, used by Tasks 2 and 3:
  `flat(){ tr -s '[:space:]' ' ' <<<"$1"; }` — takes one string argument, returns it with every run of whitespace collapsed to a single space. Also produces the convention that a region extractor is an `awk` section slice stored in `<file>_<region>` with a flattened twin `<file>_<region>_flat`.

- [ ] **Step 1: Write the failing guards**

Add to `tests/test_docket_build.sh`. Place the helper immediately below the existing `assert(){ ... }` definition near the top of the file:

```bash
# Collapse runs of whitespace so a phrase assert survives a pure re-flow of hard-wrapped markdown
# (learnings: phrase-grep-over-wrapped-prose). Runs, not only newlines: an indented list
# continuation leaves several spaces behind, and `tr '\n' ' '` alone would not close them up.
flat(){ tr -s '[:space:]' ' ' <<<"$1"; }
```

Append the following block to the end of the controller section of the file — after the escalation-ladder asserts and before the final `if [ "$fail" = 0 ]` line. It reads `$ctrl_body`, which the file already defines:

```bash
# ---------------------------------------------------------------------------
# Change 0231 — never discard a dispatched worker's tree and re-dispatch.
#
# A worker that did not return with a schema-valid outcome may still be RUNNING; discarding its
# tree and dispatching a replacement puts two workers in one worktree, which is how change 0223's
# double-write happened. Both asserts below are region-scoped rather than whole-file: the phrase
# "dispatch a fresh worker" would also match a future summary line or the frontmatter description,
# and a whole-file grep cannot observe the rule being removed from the bullet that owns it.
# ---------------------------------------------------------------------------

# The malformed-return halting bullet, sliced from its own "- **" line to the next one.
ctrl_malformed="$(awk '
  /^- \*\*A worker return is malformed/ {inb=1; print; next}
  inb && /^- \*\*/ {exit}
  inb {print}
' <<<"$ctrl_body")"
ctrl_malformed_flat="$(flat "$ctrl_malformed")"

# Non-vacuity through the SAME extractor: a renamed bullet, a reflowed heading, or a broken awk
# range would empty $ctrl_malformed and turn the negative assert below into a permanent green.
# The companion reads a clause that predates this change and must still be there.
assert "controller: the malformed-return halting bullet is extractable" \
  '[ -n "$ctrl_malformed_flat" ] &&
   grep -qF -- "Never re-dispatch a task to repair its own return" <<<"$ctrl_malformed_flat"'

assert "controller: that bullet also forbids discarding the worktree and dispatching a fresh worker" \
  'grep -qiF -- "never discard the worktree and dispatch a fresh worker" <<<"$ctrl_malformed_flat"'

assert "controller: the bullet gives the still-running worker as the reason" \
  'grep -qiE "did not observe return cleanly.{0,120}still be running" <<<"$ctrl_malformed_flat"'

assert "controller: the bullet preserves the worktree rather than cleaning it" \
  'grep -qiE "leave the worktree" <<<"$ctrl_malformed_flat"'

# The trigger is the observable event, never elapsed patience. An undefined time threshold in a
# normative contract is unactionable and invites exactly the improvisation this change closes.
assert "controller: the bullet keys on the return, not on elapsed time" \
  '! grep -qiE "(minutes|elapsed|timed out|timeout|too long|patience)" <<<"$ctrl_malformed_flat"'

# A5: the rule must not claim it reaches finalize, which neither loads nor references docket-build.
# Scoped by proximity rather than banning the word file-wide — the build gate legitimately cites
# skills/docket-finalize-change/SKILL.md as the single source of the suite-command block.
assert "controller: the prohibition does not claim to cover docket-finalize-change" \
  '! grep -qiE "never discard the worktree and dispatch a fresh worker.{0,200}finalize" <<<"$(flat "$ctrl_body")"'

# The concurrency ban in the dispatch section, sliced to that section.
ctrl_dispatch="$(awk '/^## Dispatching a task/{f=1;next} f&&/^## /{exit} f' <<<"$ctrl_body")"
ctrl_dispatch_flat="$(flat "$ctrl_dispatch")"

assert "controller: the Dispatching a task section is extractable" \
  '[ -n "$ctrl_dispatch_flat" ] &&
   grep -qF -- "Dispatch the profile agent" <<<"$ctrl_dispatch_flat"'

# Detect the REMOVED state: the bare concurrency ban that stopped at deliberate dispatch and did
# not reach a controller acting on a belief. Mutating the clause back to its bare form reddens this.
assert "controller: the concurrency ban binds a controller that believes the first worker is gone" \
  'grep -qiE "never dispatch two workers concurrently.{0,160}believes the first worker is gone" <<<"$ctrl_dispatch_flat"'
```

- [ ] **Step 2: Run the guards to verify they fail**

Run: `bash tests/test_docket_build.sh`
Expected: FAIL. Specifically `NOT OK` for "that bullet also forbids discarding the worktree and dispatching a fresh worker", "the bullet gives the still-running worker as the reason", "the bullet preserves the worktree rather than cleaning it", and "the concurrency ban binds a controller that believes the first worker is gone". The two extractability asserts and the two negative asserts (elapsed time, finalize) must already be `ok` — they pass on the unedited file by construction, which is why each has a companion.

- [ ] **Step 3: Make the two prose edits**

In `skills/docket-build/SKILL.md` § *Halting conditions*, replace the *A worker return is malformed or unverifiable* bullet. Current text:

```markdown
- **A worker return is malformed or unverifiable** — a missing or unparsable outcome, a `COMPLETE`
  whose commit is absent, unresolvable, or not an ancestor of the branch tip, or a
  `NEEDS_ESCALATION` with no concrete reason. Never re-dispatch a task to repair its own return.
```

Replacement:

```markdown
- **A worker return is malformed or unverifiable** — a missing or unparsable outcome, a `COMPLETE`
  whose commit is absent, unresolvable, or not an ancestor of the branch tip, or a
  `NEEDS_ESCALATION` with no concrete reason. Never re-dispatch a task to repair its own return,
  and never discard the worktree and dispatch a fresh worker for that task either: a worker you
  did not observe return cleanly may still be running, and it wakes into the same worktree its
  replacement is writing. Halt naming the task and the worktree, and leave the worktree exactly
  as it stands.
```

In § *Dispatching a task*, replace this sentence fragment:

```markdown
profile and routing reason, and the completion schema. Never dispatch a task reviewer, and
never dispatch two workers concurrently. Never preload a review skill either — for a **named**
```

with:

```markdown
profile and routing reason, and the completion schema. Never dispatch a task reviewer, and
never dispatch two workers concurrently — that binds a controller who *believes the first worker
is gone* exactly as it binds one dispatching deliberately. Never preload a review skill either —
for a **named**
```

Then re-flow only the lines you touched to the file's existing wrap width; do not re-flow the rest of the paragraph.

- [ ] **Step 4: Run the guards to verify they pass**

Run: `bash tests/test_docket_build.sh`
Expected: PASS.

- [ ] **Step 5: Mutation-test every new assert**

For each of the four positive asserts, delete the clause it names from `skills/docket-build/SKILL.md`, confirm the deletion landed with a count, re-run, and confirm the assert reddens. Then restore.

```bash
cd /Users/homer/dev/docket/.worktrees/a-presumed-dead-build-worker-can-wake-and-race-its-own-repla

# Mutation A — remove the new prohibition from the halting bullet.
grep -c "never discard the worktree and dispatch a fresh worker" skills/docket-build/SKILL.md   # expect 1
perl -0pi -e 's/,\n  and never discard the worktree.*?as it stands\./\./s' skills/docket-build/SKILL.md
grep -c "never discard the worktree and dispatch a fresh worker" skills/docket-build/SKILL.md   # expect 0 — the mutation LANDED
bash tests/test_docket_build.sh    # expect NOT OK on the discard, still-running, and worktree-preserving asserts
git checkout -- skills/docket-build/SKILL.md

# Mutation B — revert the concurrency ban to its bare pre-change form.
grep -c "believes the first worker is gone" skills/docket-build/SKILL.md                        # expect 1
perl -0pi -e 's/ — that binds a controller who \*\*believes the first worker\nis gone\*\* exactly as it binds one dispatching deliberately\./. /s' skills/docket-build/SKILL.md
grep -c "believes the first worker is gone" skills/docket-build/SKILL.md                        # expect 0
bash tests/test_docket_build.sh    # expect NOT OK on the believes-the-first-worker-is-gone assert
git checkout -- skills/docket-build/SKILL.md

# Mutation C — prove the extractor companion is live, not decorative.
perl -0pi -e 's/^- \*\*A worker return is malformed or unverifiable\*\*/- **A worker return is bad**/m' skills/docket-build/SKILL.md
bash tests/test_docket_build.sh    # expect NOT OK on "the malformed-return halting bullet is extractable"
git checkout -- skills/docket-build/SKILL.md
```

If a `perl -0pi` substitution does not change the count, the mutation did **not** land and the green run proves nothing — adjust the pattern to the text as it actually sits in the file and re-run. Do not accept a green mutation run without a count that moved.

- [ ] **Step 6: Re-measure and raise the size-budget row**

```bash
wc -l -w skills/docket-build/SKILL.md
grep -n "^skills/docket-build/SKILL.md" tests/test_skill_size_budgets.sh
bash tests/test_skill_size_budgets.sh
```

The pre-edit actual is **312 lines / 2876 words** against the row `320 2950`. The edits add roughly +5 lines and +70 words, which lands near 317/2946 — inside the row but with about 3 lines and 4 words of headroom. That is the near-zero margin the budget comment block names as its own failure mode, so raise the row. Apply the documented rounding to the **measured** actual, not to this estimate: lines to the next multiple of 5, words to the next multiple of 50, and if either lands within 25 words of the actual take the multiple after. From 317/2946 that gives `325 3000`.

Edit the row in `tests/test_skill_size_budgets.sh`:

```
skills/docket-build/SKILL.md                               325 3000
```

and append this rationale comment to the block above the `BUDGETS` table, immediately after the last existing entry:

```
# skills/docket-build/SKILL.md's budget was raised 320/2950 -> 325/3000 by change 0231, which
# extended the *A worker return is malformed or unverifiable* halting bullet with the sibling
# prohibition on discard-and-re-dispatch, and extended § *Dispatching a task*'s concurrency ban to
# a controller who believes the first worker is gone. The two references/ files that exist under
# skills/docket-build/ were both considered and neither can hold this prose. gate-execution.md is
# scoped to the build GATE's execution posture — how to run the suite and observe its result — and
# this rule fires at worker dispatch, before any gate runs. task-routing.md is the profile-selection
# rubric, shared with docket-implement-next's fix loop, and a dispatch-safety prohibition stated
# there would reach the fix loop in docket-build's disposition vocabulary, which is the exact
# mis-import change 0231 avoids by giving fix-loop.md its own sentence. A halting condition must
# also sit in the halting-conditions list a controller reads at the moment it decides what to do
# with a bad return; a rule in an unread reference cannot intervene at that moment. Set per the
# rounding rule above from the measured actual (317 lines -> 325, 2946 words -> 2950 would leave a
# 4-word margin, which is the 0102 near-zero failure mode this block warns against, so the multiple
# after: 3000).
```

Correct both numbers and the parenthetical to whatever you actually measured before committing.

- [ ] **Step 7: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: exit 0, `PASS` for every file. Act on any `OVER BUDGET:` line rather than treating it as noise.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-build/SKILL.md tests/test_docket_build.sh tests/test_skill_size_budgets.sh
git commit -m "fix(0231): forbid discard-and-re-dispatch in the docket-build controller

A worker that did not return with a schema-valid outcome may still be running.
Discarding its tree and dispatching a replacement puts two workers in one
worktree — change 0223's double-write. Extend the malformed-return halting
bullet with the sibling prohibition, keyed on the observable return rather than
elapsed time, and extend the concurrency ban to a controller who believes the
first worker is gone. Guards are region-scoped and mutation-tested."
```

---

### Task 2: The worker contract — `docket-build-task`

**Files:**
- Modify: `skills/docket-build-task/SKILL.md` (§ *Scope*, the amend bullet)
- Modify: `tests/test_skill_size_budgets.sh` (the `skills/docket-build-task/SKILL.md` row, only if the re-measured actual leaves under 5 lines or 50 words of headroom)
- Test: `tests/test_docket_build.sh`

**Interfaces:**
- Consumes: `flat()` and the region-extractor convention from Task 1.
- Produces: nothing later tasks depend on.

- [ ] **Step 1: Write the failing guards**

Append to the worker section of `tests/test_docket_build.sh` (the block that reads `$worker_body`, above the controller section):

```bash
# ---------------------------------------------------------------------------
# Change 0231 — the amend ban covers the worker's OWN commit, not only earlier tasks'.
#
# The 0223 incident had the woken worker commit and then amend inside its own turn, sweeping the
# replacement's work in. A "never amend after emitting your return" clause would not have reached
# that, and "after a rival has written to the same files" is not worker-observable. Widening the
# existing Scope line to any commit is observable, absolute, and binds the woken worker.
# ---------------------------------------------------------------------------
worker_scope="$(awk '/^## Scope/{f=1;next} f&&/^## /{exit} f' <<<"$worker_body")"
worker_scope_flat="$(flat "$worker_scope")"

# Non-vacuity through the SAME extractor, so a renamed heading cannot green the negative below.
assert "worker: the Scope section is extractable" \
  '[ -n "$worker_scope_flat" ] &&
   grep -qF -- "Implement only that task" <<<"$worker_scope_flat"'

assert "worker: the amend ban covers any commit, including one this worker just made" \
  'grep -qiE "never rewrite, amend, or revert \*\*any\*\* commit" <<<"$worker_scope_flat"'

assert "worker: directs correcting by adding a commit rather than amending" \
  'grep -qiE "adding another commit, never by amending" <<<"$worker_scope_flat"'

# Detect the REMOVED state. The pre-0231 wording scoped the ban to earlier task commits, which is
# exactly the gap the woken worker walked through; its return must redden this.
assert "worker: the narrow earlier-task-only amend ban is gone" \
  '! grep -qiE "amend, or revert earlier task commits" <<<"$worker_scope_flat"'

# The escalated-worker allowance is a deliberate carve-out and must survive the widening: an
# escalated worker still revises the weaker worker's UNCOMMITTED changes. Widening the COMMIT ban
# into a ban on touching uncommitted work would break escalation, so pin that it did not.
assert "worker: the escalated-worker allowance for uncommitted work survives" \
  'grep -qiF -- "You may revise or replace them" <<<"$worker_scope_flat"'
```

- [ ] **Step 2: Run the guards to verify they fail**

Run: `bash tests/test_docket_build.sh`
Expected: FAIL with `NOT OK` for "the amend ban covers any commit, including one this worker just made", "directs correcting by adding a commit rather than amending", and "the narrow earlier-task-only amend ban is gone". The extractability, escalation-allowance, and Task 1 asserts stay `ok`.

- [ ] **Step 3: Widen the Scope bullet**

In `skills/docket-build-task/SKILL.md` § *Scope*, replace:

```markdown
- Never rewrite, amend, or revert earlier task commits, and never touch unrelated user work.
```

with:

```markdown
- Never rewrite, amend, or revert **any** commit — an earlier task's, or one you just made
  yourself — and never touch unrelated user work. Correct a commit of your own by adding another
  commit, never by amending: another agent's work may already be inside it, and you cannot
  observe that. If the task text prescribes more than one commit, the plan wins.
```

- [ ] **Step 4: Run the guards to verify they pass**

Run: `bash tests/test_docket_build.sh`
Expected: PASS.

- [ ] **Step 5: Mutation-test the new asserts**

```bash
cd /Users/homer/dev/docket/.worktrees/a-presumed-dead-build-worker-can-wake-and-race-its-own-repla

# Mutation A — restore the narrow pre-change wording. The removal assert must redden.
grep -c "amend, or revert \*\*any\*\* commit" skills/docket-build-task/SKILL.md   # expect 1
perl -0pi -e 's/- Never rewrite, amend, or revert \*\*any\*\* commit.*?the plan wins\./- Never rewrite, amend, or revert earlier task commits, and never touch unrelated user work./s' skills/docket-build-task/SKILL.md
grep -c "amend, or revert \*\*any\*\* commit" skills/docket-build-task/SKILL.md   # expect 0 — the mutation LANDED
grep -c "amend, or revert earlier task commits" skills/docket-build-task/SKILL.md # expect 1
bash tests/test_docket_build.sh   # expect NOT OK on all three new positive/negative asserts
git checkout -- skills/docket-build-task/SKILL.md

# Mutation B — drop only the correct-by-adding sentence.
perl -0pi -e 's/ Correct a commit of your own by adding another\n  commit, never by amending:.*?observe that\.//s' skills/docket-build-task/SKILL.md
bash tests/test_docket_build.sh   # expect NOT OK on "directs correcting by adding a commit rather than amending"
git checkout -- skills/docket-build-task/SKILL.md

# Mutation C — prove the Scope extractor companion is live.
perl -0pi -e 's/^## Scope$/## Boundaries/m' skills/docket-build-task/SKILL.md
bash tests/test_docket_build.sh   # expect NOT OK on "the Scope section is extractable"
git checkout -- skills/docket-build-task/SKILL.md
```

Confirm each count moved before believing any run.

- [ ] **Step 6: Re-measure the size budget**

```bash
wc -l -w skills/docket-build-task/SKILL.md
bash tests/test_skill_size_budgets.sh
```

Pre-edit actual is **119 lines / 1051 words** against the row `125 1100`; the edit adds roughly +3 lines and +40 words, landing near 122/1091. That breaches nothing but leaves about 9 words of headroom, which is the near-zero failure mode. If the measured margin is under 5 lines or 50 words, raise the row per the rounding rule and append a rationale comment naming the considered home. `skills/docket-build-task/` has **no** `references/` directory, so the comment must state that the only candidate is one that would have to be created — and that creating it is wrong here for the same reason the existing entries for this file record: the body reaches a worker's context by wrapper preload (`agents/docket-build-*.md` carry `skills: [docket-build-task]`), and a rule that must bind a worker at the moment it is about to amend cannot sit in a file the wrapper does not preload. From 122/1091 the rule gives `125 1150`; the line row already holds at 125, so raise the word row only if the measurement says so.

- [ ] **Step 7: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: exit 0.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-build-task/SKILL.md tests/test_docket_build.sh tests/test_skill_size_budgets.sh
git commit -m "fix(0231): widen the worker amend ban to any commit, including its own

The woken worker in change 0223 committed and then amended inside its own turn,
rewriting a commit a rival's work was already inside. Widen the Scope line from
earlier task commits to any commit and direct correcting by adding another —
worker-observable and absolute, where a post-return clause would not have
reached the incident. The escalated-worker uncommitted-work allowance is
unaffected and now pinned."
```

---

### Task 3: The fix loop — `docket-implement-next`

**Files:**
- Modify: `skills/docket-implement-next/references/fix-loop.md` (§ *Tasks, batching, commits*)
- Modify: `tests/test_skill_size_budgets.sh` (the `skills/docket-implement-next/references/fix-loop.md` row)
- Test: `tests/test_docket_build.sh`

**Interfaces:**
- Consumes: `flat()` from Task 1. Also consumes `IMPL_FIX`, already defined in `tests/test_docket_build.sh` as `IMPL_FIX="$REPO/skills/docket-implement-next/references/fix-loop.md"` — today that variable is declared for the record and never read, and this task is what puts it to work.
- Produces: nothing.

- [ ] **Step 1: Write the failing guards**

Append to `tests/test_docket_build.sh`, after the block that defines `IMPL_FIX`:

```bash
# ---------------------------------------------------------------------------
# Change 0231 — the same prohibition in the fix loop's OWN vocabulary.
#
# docket-implement-next Step 6 dispatches docket-build-task workers directly and never loads
# docket-build's SKILL.md, so it cannot inherit the controller rule and must not import
# docket-build's `halted` BUILD outcome. Its disposition is abort-and-report with the change left
# in-progress and claimed_at refreshed. These asserts pin that it says so in its own terms.
# ---------------------------------------------------------------------------
impl_fix_body="$(cat "$IMPL_FIX" 2>/dev/null)"
impl_fix_flat="$(flat "$impl_fix_body")"

# Non-vacuity floor: every assert below reads $impl_fix_flat, so an unreadable or moved file must
# redden HERE rather than passing every negative grep by default.
assert "fix loop: the reference is non-vacuous (at least 100 lines)" \
  '[ "$(grep -c . <<<"$impl_fix_body")" -ge 100 ]'

assert "fix loop: forbids discarding the worktree and dispatching a fresh worker" \
  'grep -qiF -- "never discard the worktree and dispatch a fresh worker" <<<"$impl_fix_flat"'

# The disposition must be the fix loop's own, stated with the prohibition rather than imported.
assert "fix loop: gives that prohibition the abort-and-report disposition" \
  'grep -qiE "never discard the worktree and dispatch a fresh worker.{0,240}abort-and-report" <<<"$impl_fix_flat"'

assert "fix loop: that disposition refreshes the claim lease" \
  'grep -qiE "never discard the worktree and dispatch a fresh worker.{0,240}claimed_at" <<<"$impl_fix_flat"'

# It must NOT import docket-build's build-outcome vocabulary, which is the mis-import this
# separate sentence exists to avoid.
assert "fix loop: does not import the build role halted outcome for this rule" \
  '! grep -qiE "never discard the worktree and dispatch a fresh worker.{0,240}(return .halted.|halted build outcome)" <<<"$impl_fix_flat"'

# A5: the prohibition must not claim to reach finalize, which has no discard-and-re-dispatch path.
assert "fix loop: the prohibition does not claim to cover docket-finalize-change" \
  '! grep -qiE "never discard the worktree and dispatch a fresh worker.{0,200}finalize" <<<"$impl_fix_flat"'
```

Note the `.{0,240}` bounds: every repetition count in this file stays at or below 255 so `/usr/bin/grep` accepts it.

- [ ] **Step 2: Run the guards to verify they fail**

Run: `bash tests/test_docket_build.sh`
Expected: FAIL with `NOT OK` for "forbids discarding the worktree and dispatching a fresh worker", "gives that prohibition the abort-and-report disposition", and "that disposition refreshes the claim lease". The non-vacuity floor and the three negative asserts pass on the unedited file.

- [ ] **Step 3: Add the sentence**

In `skills/docket-implement-next/references/fix-loop.md` § *Tasks, batching, commits*, the opening paragraph currently reads:

```markdown
Every fix runs the **`docket-build-task`** contract (focused test → implement → verify →
self-review → one commit), dispatched by profile name, **foreground and sequential** — fixes share
one worktree, so two concurrent workers would collide.
```

Extend it to:

```markdown
Every fix runs the **`docket-build-task`** contract (focused test → implement → verify →
self-review → one commit), dispatched by profile name, **foreground and sequential** — fixes share
one worktree, so two concurrent workers would collide.

A fix worker that returns without a schema-valid outcome may still be **running**: never discard
the worktree and dispatch a fresh worker for that finding, however dead the first one looks. Halt
instead — abort-and-report, the change staying `in-progress` with `claimed_at` refreshed and the
reason recorded, the worktree left exactly as it stands. The trigger is the malformed return you
observed, never elapsed time; a blocked foreground controller has no clock.
```

- [ ] **Step 4: Run the guards to verify they pass**

Run: `bash tests/test_docket_build.sh`
Expected: PASS.

- [ ] **Step 5: Mutation-test the new asserts**

```bash
cd /Users/homer/dev/docket/.worktrees/a-presumed-dead-build-worker-can-wake-and-race-its-own-repla

# Mutation A — delete the whole new paragraph.
grep -c "never discard the worktree and dispatch a fresh worker" skills/docket-implement-next/references/fix-loop.md   # expect 1
perl -0pi -e 's/\nA fix worker that returns without a schema-valid outcome.*?has no clock\.\n//s' skills/docket-implement-next/references/fix-loop.md
grep -c "never discard the worktree and dispatch a fresh worker" skills/docket-implement-next/references/fix-loop.md   # expect 0 — the mutation LANDED
bash tests/test_docket_build.sh   # expect NOT OK on all three positive asserts
git checkout -- skills/docket-implement-next/references/fix-loop.md

# Mutation B — keep the prohibition but strip its disposition, the exact mis-import this guards.
perl -0pi -e 's/ Halt\ninstead — abort-and-report, the change staying `in-progress` with `claimed_at` refreshed and the\nreason recorded, the worktree left exactly as it stands\./ Halt instead./s' skills/docket-implement-next/references/fix-loop.md
bash tests/test_docket_build.sh   # expect NOT OK on the abort-and-report and claimed_at asserts
git checkout -- skills/docket-implement-next/references/fix-loop.md

# Mutation C — prove the non-vacuity floor is live.
head -20 skills/docket-implement-next/references/fix-loop.md > /tmp/ff && cp /tmp/ff skills/docket-implement-next/references/fix-loop.md
bash tests/test_docket_build.sh   # expect NOT OK on "the reference is non-vacuous (at least 100 lines)"
git checkout -- skills/docket-implement-next/references/fix-loop.md
```

Do not use a fixed `/tmp/<name>` path if you are running inside the parallel runner; run these probes directly with `bash tests/test_docket_build.sh` as shown, and prefer `mktemp` if you script them.

- [ ] **Step 6: Re-measure and raise the size-budget row**

```bash
wc -l -w skills/docket-implement-next/references/fix-loop.md
bash tests/test_skill_size_budgets.sh
```

Pre-edit actual is **175 lines / 1779 words** against the row `180 1850`. The paragraph adds roughly +6 lines and +50 words, landing near 181/1829 — a **line breach**. Raise the row. From 181/1829 the rounding rule gives lines 185 and words 1850; 1850 leaves only 21 words over the measured actual, inside the 25-word clause, so take the multiple after: `185 1900`.

```
skills/docket-implement-next/references/fix-loop.md        185 1900
```

Append the rationale comment after the last existing entry in the block above the table:

```
# fix-loop.md's row was raised 180/1850 -> 185/1900 by change 0231, which states the
# never-discard-and-re-dispatch prohibition in the fix loop's OWN disposition vocabulary. The
# considered home is skills/docket-build/SKILL.md, which owns the controller-side rule and states
# it there in the same change. It cannot be the only home: docket-implement-next Step 6 dispatches
# docket-build-task workers itself and never loads docket-build's SKILL.md, so a pointer would
# import docket-build's `halted` BUILD outcome where the fix loop's disposition is
# abort-and-report with the change left in-progress and claimed_at refreshed. That is one sentence
# duplicated into two vocabularies, which is the shape this file's owner already uses for shared
# rules, rather than a restatement of the same sentence. Set per the rounding rule above from the
# measured actual (181 lines -> 185; 1829 words -> 1850 would leave a 21-word margin, inside the
# 25-word clause, so the multiple after: 1900).
```

Correct all numbers to what you measured.

- [ ] **Step 7: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: exit 0, `PASS` for every file.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-implement-next/references/fix-loop.md tests/test_docket_build.sh tests/test_skill_size_budgets.sh
git commit -m "fix(0231): state the no-discard-and-re-dispatch rule in the fix loop's vocabulary

Step 6's fix loop dispatches docket-build-task workers directly and never loads
docket-build's SKILL.md, so it cannot inherit the controller rule and must not
import docket-build's halted build outcome. One sentence in the fix loop's own
disposition — abort-and-report, in-progress, claimed_at refreshed — plus guards
pinning the co-occurrence rather than the bare phrase."
```

---

## Self-Review

**Spec coverage.** Spec § *What changes* item 1 (halting bullet) → Task 1 Step 3. Item 2 (§ *Dispatching a task*) → Task 1 Step 3. Item 3 (worker § *Scope*) → Task 2 Step 3. Item 4 (`fix-loop.md`) → Task 3 Step 3. Item 5 (guards in `tests/test_docket_build.sh`, mutation-tested) → Steps 1/5 of all three tasks. A1's observable trigger → the elapsed-time negative assert in Task 1 and the closing clause of Task 3's paragraph. A2 (prose, no mechanism) → no new field, file, or script appears anywhere in this plan. A3 (own vocabulary, not a pointer) → Task 3's abort-and-report and no-halted-import asserts. A4's stated trade (a worker loses ordinary amend-cleanup) and its preserved escape ("if the task text prescribes more than one commit, the plan wins") → both in Task 2's replacement text. A5 (finalize out of scope, and no claim it reads onto finalize) → the proximity-scoped negative asserts in Tasks 1 and 3. A8's size-budget hazard → Step 6 of every task, with the rationale ritual. A9 (no detection) → nothing in this plan adds a stray-commit detector.

**Placeholder scan.** Every prose edit is given as literal before/after text; every assert is given as runnable Bash; every mutation is given as a runnable command with its expected count. The three size-budget steps deliberately say "correct the numbers to what you measured" — that is a measurement instruction with the exact rule, the exact pre-edit actual, and the computed expected value, not a placeholder.

**Type consistency.** `flat()` is defined once in Task 1 and consumed by Tasks 2 and 3 with the same one-argument signature. `$ctrl_body`, `$worker_body`, `$CTRL`, `$WORKER`, and `$IMPL_FIX` all use the names `tests/test_docket_build.sh` already defines. The new variables follow one convention throughout: `<file>_<region>` for the raw slice and `<file>_<region>_flat` for its collapsed twin.

**One correction made during review.** Task 2's guard set initially had no assert protecting the escalated-worker allowance. Widening a *commit* ban next to a bullet that explicitly permits revising a weaker worker's *uncommitted* changes is exactly where an over-broad rewrite would land, and nothing would have caught it — so the allowance assert was added.
