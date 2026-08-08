<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0249 — Build-worker contract: gate-execution pointer and staging discipline](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0249-build-worker-contract-gate-execution-pointer-and-staging-dis.md)**
<!-- docket:backlink:end -->

# Build-worker contract: gate-execution pointer and staging discipline — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the two remaining change-0223 mechanisms in `skills/docket-build-task/SKILL.md` — the gate execution posture never reaches a dispatched worker, and nothing constrains what a worker stages — with contract prose, mutation-proven guards, and the budget raise the prose requires.

**Architecture:** Prose-only contract change plus guards. Four edits in one diff: a pointer paragraph in the worker contract's `## The cycle` at the reference file `skills/docket-build/references/gate-execution.md` (with the worker-shaped consequence stated inline in harness-neutral words); a new `## Scope` bullet stating staging discipline and its escalation carve-out; a change-0249 banner of asserts in `tests/test_docket_build.sh` reusing the existing 0231 `worker_scope_flat` extractor plus a new section extractor for `## The cycle`; and the `tests/test_skill_size_budgets.sh` row raise the growth forces, set in-diff from the measured post-edit actual.

**Tech Stack:** Bash 4.4+, POSIX-portable `grep -E` / `awk`, markdown skill contracts, `scripts/run-tests.sh` as the suite runner.

## Global Constraints

- Repository instructions in `/Users/homer/dev/docket/AGENTS.md` override this plan wherever they conflict. Read it before writing code.
- **Guards are code — mutation-test every new assert.** Strip or invert the guarded clause, watch the assert redden, restore. An assert never seen red is decoration.
- **Mutation restore uses a backup copy, never `git checkout --`** (learnings: `mutation-restore-needs-a-backup-copy`): `cp "$f" "$f.bak"; <mutate>; <run>; mv "$f.bak" "$f"`. The files under mutation are edited-and-uncommitted; `git checkout --` would destroy the work under test and produce a large meaningless red reading.
- **Confirm every mutation actually landed** with a `grep -c` before/after taken through a whitespace-flattened copy (learnings: `assert-detects-removal-not-replacement`, `phrase-grep-over-wrapped-prose`). A substitution that silently did not match yields a green run with nothing mutated, which reads exactly like a robust guard.
- **One bounded gap per ERE, never two** (learnings: `stacked-gap-regex-hangs-instead-of-failing`). Two stacked gaps backtrack catastrophically on non-matching input, so the mutation test hangs instead of reddening. Where a claim has three parts, write three asserts.
- **Bind a phrase to its claim, not to the file** (learnings: `prose-guard-binds-phrase-to-claim`). A bare presence grep survives a rewrite that keeps the words and drops the rule.
- **Match a whitespace-collapsed haystack** for every phrase assert (learnings: `phrase-grep-over-wrapped-prose`) — the file's existing `flat()` helper. Never make a phrase assert double as a line-wrap guard.
- **Never `producer | early-exiting-consumer` under `set -o pipefail`** (AGENTS.md). Capture into a variable, then `grep <<<"$var"`.
- **`grep` for a pattern that leads with `--` must declare it** — `grep -qF -- "<pat>"` (AGENTS.md).
- **Comments anchor on a symbol name or a verbatim-quoted clause, never a line number** (AGENTS.md; `tests/test_comment_anchor_style.sh` enforces the filename-plus-line-number form).
- **Do not edit** `skills/docket-build/SKILL.md`, `skills/docket-implement-next/SKILL.md`, or any file under `skills/docket-build/references/`. The change is scoped to the worker contract and the two test files (spec § *Out of scope*).
- **Do not restate** any gate capability or per-harness verdict outside `skills/docket-build/references/gate-execution.md`, and do not import `docket-build`'s controller vocabulary (`GATE_OBSERVATION_BUDGET`, *Halting conditions*, the `halted` BUILD outcome) into the worker contract. That import is the exact hazard the pointer exists to avoid (spec A1).
- **Full suite command:** `scripts/run-tests.sh` (this repo's resolved `finalize.test_command`). Exit `0` green, `1` a test failed, `3` green but some target produced no result.

---

## File Structure

| File | Responsibility in this change |
|---|---|
| `skills/docket-build-task/SKILL.md` | Modify. The worker contract. Gains one paragraph at the end of `## The cycle` (the pointer + inline consequence) and one bullet at the end of `## Scope` (staging discipline + escalation carve-out). Nothing else in the file changes. |
| `tests/test_docket_build.sh` | Modify. Gains a change-0249 banner block appended after the existing change-0224 block and before the terminal `if [ "$fail" = 0 ]` lines. Reuses `flat()`, `worker_body`, and the 0231 `worker_scope`/`worker_scope_flat` extractor; adds one new `worker_cycle`/`worker_cycle_flat` extractor. |
| `tests/test_skill_size_budgets.sh` | Modify. Two edits: one appended rationale comment paragraph in the `BUDGETS` comment block (immediately before the `BUDGETS="` assignment), and the `skills/docket-build-task/SKILL.md` row raised from `130 1150`. |

No files are created. `skills/docket-build-task/` deliberately gains no `references/` tree — see Task 1 Step 9's rationale.

---

### Task 1: Worker-contract clauses, their guards, and the budget raise

One task: the guards are the RED evidence for the prose, and the budget row breaches the moment the prose lands, so all four edits share one test cycle and one commit.

**Files:**
- Modify: `skills/docket-build-task/SKILL.md` — append a paragraph to `## The cycle`; append a bullet to `## Scope`
- Modify: `tests/test_docket_build.sh` — append the change-0249 banner block before the terminal `if [ "$fail" = 0 ]` line
- Modify: `tests/test_skill_size_budgets.sh` — append a rationale comment; raise the `skills/docket-build-task/SKILL.md` row
- Test: `tests/test_docket_build.sh`, `tests/test_skill_size_budgets.sh`, then the full suite

**Interfaces:**
- Consumes (already present in `tests/test_docket_build.sh`, do not redefine): `assert(){ ... }`; `flat(){ tr -s '[:space:]' ' ' <<<"$1"; }`; `worker_body="$(cat "$WORKER" 2>/dev/null)"`; `worker_scope="$(awk '/^## Scope/{f=1;next} f&&/^## /{exit} f' <<<"$worker_body")"`; `worker_scope_flat="$(flat "$worker_scope")"`.
- Produces: `worker_cycle` and `worker_cycle_flat` — a `## The cycle` section slice and its flattened form, available to any later assert in that file.

---

- [ ] **Step 1: Add the change-0249 guard block to `tests/test_docket_build.sh`**

Insert the block below **immediately before** the file's terminal two lines:

```bash
if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

The block to insert:

```bash
# ---------------------------------------------------------------------------
# Change 0249 — the worker contract carries the gate-execution pointer and a
# staging rule.
#
# Two mechanisms of the change-0223 incident that change 0231 did not close. (1) The gate execution
# posture lives in docket-build's SKILL.md and its references/gate-execution.md — files a dispatched
# worker never loads, because a worker is dispatched with its task, not with its controller's
# contract — so workers running the full suite as their honest focused verification each re-invented
# background-the-suite-and-yield. (2) "## Scope" forbade EDITING unrelated work but said nothing
# about STAGING, so `git add -A` could sweep another agent's dirty paths into the worker's one
# commit, which is how the 0223 double-write started.
# ---------------------------------------------------------------------------

# The pointer lives in "## The cycle", beside step 4's focused-not-the-whole-suite note. Slice to
# that section rather than grepping the file: the reference path and the never-yield words would
# also match a future summary line or a frontmatter description, and a whole-file grep cannot
# observe the rule being removed from the section that owns it.
worker_cycle="$(awk '/^## The cycle$/{f=1;next} f&&/^## /{exit} f' <<<"$worker_body")"
worker_cycle_flat="$(flat "$worker_cycle")"

# Non-vacuity through the SAME extractor, anchored on a clause that PREDATES this change, so a
# renamed heading or a broken awk range reddens HERE instead of greening every assert below.
assert "0249: the worker ## The cycle section is extractable" \
  '[ -n "$worker_cycle" ] &&
   grep -qF -- "fails for the intended reason" <<<"$worker_cycle_flat"'

# (1a) The pointer names the reference file by path. -F because the path carries regex
# metacharacters, and the whole point is that the worker is sent to the harness-neutral capability
# file rather than to docket-build's controller-vocabulary posture section.
assert "0249: the cycle points at docket-build/references/gate-execution.md" \
  'grep -qF -- "docket-build/references/gate-execution.md" <<<"$worker_cycle_flat"'

# (1b) ...and the worker-shaped consequence, not the path alone: a rewrite that keeps the pointer
# while inverting the conduct must redden. Word-anchored so the negation cannot match inside
# "whenever" or "however".
assert "0249: the cycle forbids yielding to await the run" \
  'grep -qiE "\b(never|do not)\b[^.]{0,60}yield to await" <<<"$worker_cycle_flat"'

# (1c) Fail-closed, bound to its subject with ONE gap: it is the UNFINISHED run that is not green.
# A bare presence grep for "not green" survives a rewrite that keeps the words and drops the rule.
assert "0249: an unfinished run at the observation bound is not green" \
  'grep -qiE "unfinished[^.]{0,80}not green" <<<"$worker_cycle_flat"'

# (2) The staging prohibition, scoped through the 0231 "## Scope" extractor above. Three asserts
# with ONE bounded gap each, never one ERE with three: stacked gaps backtrack catastrophically on
# non-matching input, so the mutation test hangs instead of reddening (learnings:
# stacked-gap-regex-hangs-instead-of-failing). The gap class excludes the colon that terminates the
# prohibition clause, so no gap can bind across the sentence into unrelated Scope prose.
assert "0249: Scope forbids git add -A" \
  'grep -qiE "\bnever\b[^:]{0,80}git add -A" <<<"$worker_scope_flat"'
assert "0249: Scope forbids git add ." \
  'grep -qiE "\bnever\b[^:]{0,80}git add \." <<<"$worker_scope_flat"'
assert "0249: Scope forbids git commit -a" \
  'grep -qiE "\bnever\b[^:]{0,80}git commit -a" <<<"$worker_scope_flat"'

# (3) The positive rule the three prohibitions implement. Without it the bullet is a list of banned
# spellings, and the next sweep idiom nobody enumerated walks straight through.
assert "0249: Scope states the explicit-path staging rule" \
  'grep -qiF -- "Stage by explicit path" <<<"$worker_scope_flat"'

# (4) The observability half — the part a lazy rewrite drops first — bound to what it displaces, so
# a rewrite that redefines the rule back onto `git status` diffing reddens.
assert "0249: what the task changed is defined by the task contract, not git status" \
  'grep -qiE "task contract, not[^.]{0,60}git status" <<<"$worker_scope_flat"'

# (5) The escalation carve-out, pinned in BOTH directions with one gap each. A single assert on the
# permissive half alone would stay green through a rewrite that re-licensed the sweep for exactly
# the worker most likely to be in a dirty shared tree — the first-draft wording the critic gate
# rejected. These are companions to, never replacements of, the 0231 pin
# "You may revise or replace them", which must stay green above.
assert "0249: an inherited path within the task boundary is staged normally" \
  'grep -qiE "within the task[^.]{0,80}staged normally" <<<"$worker_scope_flat"'
assert "0249: an inherited path outside the task boundary is not staged" \
  'grep -qiE "outside the task boundary[^.]{0,60}not staged" <<<"$worker_scope_flat"'
```

- [ ] **Step 2: Run the guard file and verify it fails for the intended reason**

Run: `bash tests/test_docket_build.sh`

Expected: **FAIL**. Exactly the nine new `0249:` asserts print `NOT OK` (the extractability assert passes — `## The cycle` and its "fails for the intended reason" clause already exist). Every pre-existing assert, including all five 0231 worker-scope pins, still prints `ok`. If any pre-existing assert reddens, stop: the block was inserted in the wrong place or clobbered an existing definition.

- [ ] **Step 3: Add the gate-execution pointer to `skills/docket-build-task/SKILL.md`**

In `## The cycle`, insert a blank line and this paragraph **after** the numbered list item `5. Self-review the diff, then commit.` and **before** the line `Two obligations the cycle does not relax:`:

```markdown
When the narrowest honest verification is still a run that may outlast a single foreground call —
on this repo, often the full suite — run it under the capabilities in
[`../docket-build/references/gate-execution.md`](../docket-build/references/gate-execution.md), and
read that file before you start such a run. You are a dispatched worker with no resumption channel:
**never yield to await the run.** Observe it by blocking — short foreground reads of the durable
result — keep the observation **finite**, and treat a run still unfinished when you stop observing
as **not green**: report it unverified and fail closed, never infer success.
```

Copy it verbatim, including the wrap points: assert (1b)'s gap is 60 characters and (1c)'s is 80, both measured against this flattened text.

- [ ] **Step 4: Add the staging-discipline bullet to `skills/docket-build-task/SKILL.md`**

In `## Scope`, append this bullet **immediately after** the existing escalated-worker bullet (the one ending `never discard them blindly and never `git checkout .` over them.`) and **before** the `**Scope of these prohibitions:**` paragraph:

```markdown
- **Stage by explicit path — only paths your task changed.** Never `git add -A`, `git add .`, or
  `git commit -a`: the worktree is shared, and a sweep puts work that is not yours into your
  commit. What your task changed is defined by the **task contract, not** by diffing `git status`:
  a derived file your task's own command regenerates is yours to stage, while a dirty path you
  cannot attribute to your task is not — leave it in place and name it in `NOTES`. If you were
  dispatched as an escalated worker, an inherited path you revised, replaced, or deliberately kept
  within the task's scope is one of your task's paths and is staged normally; an inherited path
  outside the task boundary is accounted for but not staged, taking the same leave-and-report
  posture.
```

Copy it verbatim. Placing it adjacent to the escalated-worker bullet is deliberate (spec A4): the carve-out must sit beside the bullet it qualifies.

- [ ] **Step 5: Run the guard file and verify it passes**

Run: `bash tests/test_docket_build.sh`

Expected: **PASS**, with all nine `0249:` asserts `ok` and every 0231 assert still `ok`.

- [ ] **Step 6: Measure the worker contract and derive the new budget row**

Run:

```bash
wc -l < skills/docket-build-task/SKILL.md
wc -w < skills/docket-build-task/SKILL.md
```

Apply the documented rule from the `BUDGETS` comment block in `tests/test_skill_size_budgets.sh`, verbatim:

- **Lines** → the next multiple of 5 strictly above the actual; if that leaves near-zero headroom (the block records raising past margins of 0, 1, 2, and 3 lines), take the multiple after it.
- **Words** → the next multiple of 50 strictly above the actual; if that lands **within 25 words** of the actual, take the multiple after it.

With the verbatim prose of Steps 3 and 4 the actuals are **139 lines / 1321 words**, giving:

- 139 → next multiple of 5 is 140, which leaves **1 line** — the near-zero mode — so the multiple after: **145**.
- 1321 → next multiple of 50 is 1350, which leaves **29 words**, above the 25-word threshold, so **1350** stands.

If your measured actuals differ from 139/1321, re-derive from **your** measurement and use those numbers in Steps 7 and 8 instead — the rule is authoritative, these figures are its expected output.

- [ ] **Step 7: Raise the budget row in `tests/test_skill_size_budgets.sh`**

In the `BUDGETS` string, change the row

```
skills/docket-build-task/SKILL.md                          130 1150
```

to

```
skills/docket-build-task/SKILL.md                          145 1350
```

Keep the existing column alignment of the surrounding rows.

- [ ] **Step 8: Append the in-diff raise rationale**

The `BUDGETS` comment block requires every raise to NAME the `references/` file the new prose was considered for and STATE why it cannot live there — argued in-diff, not asserted (change 0201). Append this paragraph to the end of the comment block, **immediately before** the `BUDGETS="` assignment line:

```bash
# skills/docket-build-task/SKILL.md's budget was raised 130/1150 -> 145/1350 by change 0249, which
# added two normative clauses to the worker contract: a pointer in ## The cycle to the gate's
# execution capabilities plus the worker-shaped consequence inline (never yield, observe by
# blocking, finite observation, unfinished is not green), and a ## Scope bullet requiring staging by
# explicit path with its escalation carve-out.
# WHERE ELSE IT WAS CONSIDERED, per the naming requirement above: skills/docket-build-task/ has NO
# references/ tree, so the only candidate home is one that would have to be created, and creating it
# is wrong here for the reason the 0212 and 0231 entries above already record for this same file —
# the body reaches a worker's context by wrapper preload (agents/docket-build-*.md carry
# skills: [docket-build-task]), and a rule that must bind a worker at the moment it is about to
# stage or about to yield cannot sit in a file the wrapper does not preload. The other candidate,
# skills/docket-build/references/gate-execution.md, is where the pointer POINTS and already holds
# everything extractable: it states the harness capabilities, not the worker's conduct, so the
# never-yield / finite-observation / fail-closed consequence has no home there. Set per the rounding
# rule above from the measured actuals: 139 lines -> the next multiple of 5 is 140, which leaves ONE
# line — the near-zero mode this block's 0102 and 0137 entries exist to forbid — so the multiple
# after: 145. 1321 words -> the next multiple of 50 is 1350, a 29-word margin, above the 25-word
# threshold, so 1350 stands.
```

If Step 6 produced different actuals, edit this paragraph's final two sentences to state **your** measured numbers and the derivation from them.

- [ ] **Step 9: Run the budget suite and verify it passes**

Run: `bash tests/test_skill_size_budgets.sh`

Expected: **PASS**. Every row, including the raised one, prints `ok`.

- [ ] **Step 10: Mutation-test the four `## The cycle` asserts**

For each mutation: back up with `cp`, mutate, run `bash tests/test_docket_build.sh`, confirm the named assert prints `NOT OK`, then restore with `mv`. Take a `grep -c` count through a flattened copy **before and after** each mutation and confirm it changed — a substitution that never matched reads exactly like a robust guard.

```bash
f=skills/docket-build-task/SKILL.md
flatf(){ tr -s '[:space:]' ' ' < "$1"; }

# M1 — delete the reference path. Expect "0249: the cycle points at ..." to redden.
cp "$f" "$f.bak"
before=$(flatf "$f" | grep -c -o -F -- "docket-build/references/gate-execution.md" || true)
perl -0pi -e 's{\Q../docket-build/references/gate-execution.md\E}{THE-REFERENCE}g' "$f"
after=$(flatf "$f" | grep -c -o -F -- "docket-build/references/gate-execution.md" || true)
echo "M1 before=$before after=$after"; [ "$before" -gt 0 ] && [ "$after" -eq 0 ] || echo "MUTATION DID NOT LAND"
bash tests/test_docket_build.sh | grep -F -- "0249:"
mv "$f.bak" "$f"

# M2 — INVERT the never-yield consequence (keep the words, drop the rule). Expect
# "0249: the cycle forbids yielding to await the run" to redden.
cp "$f" "$f.bak"
before=$(flatf "$f" | grep -c -o -iE "\bnever\b[^.]{0,60}yield to await" || true)
perl -0pi -e 's/\*\*never yield to await the run\.\*\*/**you may yield to await the run.**/' "$f"
after=$(flatf "$f" | grep -c -o -iE "\bnever\b[^.]{0,60}yield to await" || true)
echo "M2 before=$before after=$after"; [ "$before" -gt 0 ] && [ "$after" -eq 0 ] || echo "MUTATION DID NOT LAND"
bash tests/test_docket_build.sh | grep -F -- "0249:"
mv "$f.bak" "$f"

# M3 — sever the fail-closed binding: keep "not green", drop its subject. Expect
# "0249: an unfinished run at the observation bound is not green" to redden.
cp "$f" "$f.bak"
before=$(flatf "$f" | grep -c -o -iE "unfinished[^.]{0,80}not green" || true)
perl -0pi -e 's/a run still unfinished when you stop observing\s+as \*\*not green\*\*/the report as **not green**/s' "$f"
after=$(flatf "$f" | grep -c -o -iE "unfinished[^.]{0,80}not green" || true)
echo "M3 before=$before after=$after"; [ "$before" -gt 0 ] && [ "$after" -eq 0 ] || echo "MUTATION DID NOT LAND"
bash tests/test_docket_build.sh | grep -F -- "0249:"
mv "$f.bak" "$f"

# M4 — rename the heading, to prove the non-vacuity companion is load-bearing. Expect
# "0249: the worker ## The cycle section is extractable" to redden (and the three asserts that
# read the slice to redden with it, which is the point: a dead extractor must not read as green).
cp "$f" "$f.bak"
perl -0pi -e 's/^## The cycle$/## The loop/m' "$f"
grep -c -E "^## The cycle$" "$f" || echo "heading renamed"
bash tests/test_docket_build.sh | grep -F -- "0249:"
mv "$f.bak" "$f"
```

Every `echo` must report a changed count, and the named assert must appear as `NOT OK`. If a count did not change, the mutation did not land — fix the substitution and rerun before believing anything.

- [ ] **Step 11: Mutation-test the six `## Scope` asserts**

```bash
f=skills/docket-build-task/SKILL.md
flatf(){ tr -s '[:space:]' ' ' < "$1"; }

# M5 — delete the whole staging bullet. Expect all six staging asserts (2a/2b/2c, 3, 4, 5a/5b) to
# redden, and the 0231 pin "You may revise or replace them" to stay green.
cp "$f" "$f.bak"
before=$(flatf "$f" | grep -c -o -iF -- "Stage by explicit path" || true)
perl -0pi -e 's/^- \*\*Stage by explicit path.*?\n  posture\.\n//ms' "$f"
after=$(flatf "$f" | grep -c -o -iF -- "Stage by explicit path" || true)
echo "M5 before=$before after=$after"; [ "$before" -gt 0 ] && [ "$after" -eq 0 ] || echo "MUTATION DID NOT LAND"
bash tests/test_docket_build.sh | grep -E -- "0249:|revise or replace"
mv "$f.bak" "$f"

# M6 — keep the bullet, drop only the `git commit -a` form. Expect ONLY
# "0249: Scope forbids git commit -a" to redden — proof each form is pinned on its own rather than
# riding on one list-presence grep.
cp "$f" "$f.bak"
before=$(flatf "$f" | grep -c -o -iE "\bnever\b[^:]{0,80}git commit -a" || true)
perl -0pi -e 's/, or `git commit -a`:/:/' "$f"
after=$(flatf "$f" | grep -c -o -iE "\bnever\b[^:]{0,80}git commit -a" || true)
echo "M6 before=$before after=$after"; [ "$before" -gt 0 ] && [ "$after" -eq 0 ] || echo "MUTATION DID NOT LAND"
bash tests/test_docket_build.sh | grep -F -- "0249:"
mv "$f.bak" "$f"

# M7 — redefine observability back onto git status diffing. Expect
# "0249: what the task changed is defined by the task contract, not git status" to redden.
cp "$f" "$f.bak"
before=$(flatf "$f" | grep -c -o -iE "task contract, not[^.]{0,60}git status" || true)
perl -0pi -e 's/\*\*task contract, not\*\* by diffing `git status`/whatever `git status` reports as dirty/' "$f"
after=$(flatf "$f" | grep -c -o -iE "task contract, not[^.]{0,60}git status" || true)
echo "M7 before=$before after=$after"; [ "$before" -gt 0 ] && [ "$after" -eq 0 ] || echo "MUTATION DID NOT LAND"
bash tests/test_docket_build.sh | grep -F -- "0249:"
mv "$f.bak" "$f"

# M8 — re-license the sweep for the escalated worker: drop the out-of-task limb. Expect
# "0249: an inherited path outside the task boundary is not staged" to redden while
# "...within the task boundary is staged normally" stays green. This is the mutation that proves
# the carve-out is bounded in BOTH directions rather than merely present.
cp "$f" "$f.bak"
before=$(flatf "$f" | grep -c -o -iE "outside the task boundary[^.]{0,60}not staged" || true)
perl -0pi -e 's/; an inherited path\s+outside the task boundary is accounted for but not staged, taking the same leave-and-report\s+posture\./, as is any other inherited path./s' "$f"
after=$(flatf "$f" | grep -c -o -iE "outside the task boundary[^.]{0,60}not staged" || true)
echo "M8 before=$before after=$after"; [ "$before" -gt 0 ] && [ "$after" -eq 0 ] || echo "MUTATION DID NOT LAND"
bash tests/test_docket_build.sh | grep -F -- "0249:"
mv "$f.bak" "$f"
```

- [ ] **Step 12: Confirm the tree is restored and unmutated**

Run: `git diff --stat` and `bash tests/test_docket_build.sh`

Expected: exactly three modified files (`skills/docket-build-task/SKILL.md`, `tests/test_docket_build.sh`, `tests/test_skill_size_budgets.sh`), no `.bak` files left behind (`ls skills/docket-build-task/`), and the guard file **PASS**. If a `.bak` survives, a mutation block exited early — restore it and re-run the affected mutation.

- [ ] **Step 13: Run the full suite**

Run: `scripts/run-tests.sh`

Expected: exit `0`. Watch specifically for `tests/test_docket_build.sh`, `tests/test_skill_size_budgets.sh`, `tests/test_gate_execution_posture.sh`, `tests/test_comment_anchor_style.sh`, `tests/test_grep_portability.sh`, and `tests/test_runtime_budgets.sh`. A trailing `OVER BUDGET:` block does not fail the run but is a finding to report in the return, not noise.

This run may outlast a single foreground call. Run it under the gate-execution capabilities — never background it and yield; observe by blocking on short foreground reads of a durable result, keep the observation finite, and treat an unfinished run as unverified rather than green. (This plan step is the first consumer of the very clause it is landing.)

- [ ] **Step 14: Commit**

Stage by explicit path — the three files this task changed, nothing else:

```bash
git add skills/docket-build-task/SKILL.md tests/test_docket_build.sh tests/test_skill_size_budgets.sh
git commit -m "fix(0249): give the worker contract the gate-execution pointer and a staging rule

The gate execution posture lived only in docket-build's SKILL.md and its
references/gate-execution.md — files a dispatched worker never loads — so
workers running the full suite re-invented background-and-yield. And ## Scope
forbade editing unrelated work but never constrained staging, so git add -A
could sweep another agent's dirty paths into the worker's one commit.

Adds a pointer at the harness-neutral capability file plus the worker-shaped
consequence inline, and a staging-discipline bullet with an escalation
carve-out bounded by the task boundary. Guards under a change-0249 banner
reuse the 0231 Scope extractor; every assert mutation-proven. Budget row
raised 130/1150 -> 145/1350 from the in-diff measured actual."
```

---

## Verification checklist (for the whole-branch review)

- `skills/docket-build/SKILL.md`, `skills/docket-implement-next/SKILL.md`, and everything under `skills/docket-build/references/` are untouched in the branch diff.
- The worker contract contains no `GATE_OBSERVATION_BUDGET`, no *Halting conditions*, and no `halted` BUILD outcome — the controller vocabulary the pointer exists to keep out.
- All five change-0231 worker-scope asserts still print `ok`.
- Every new assert has a recorded mutation with a before/after count that changed.
- The budget rationale paragraph names both considered homes and argues each, rather than asserting "no other home".
