<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0194 — Retire the retired-default framing from the docket-owned role skill bodies](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-03-0194-retire-the-retired-default-framing-from-the-docket-owned-rol.md)**
<!-- docket:backlink:end -->

# Retire the retired-default framing from the docket-owned role skill bodies — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the two default-status claims from docket's own role skill bodies, ship a negative guard that stops the construct coming back, and record the rule once in the convention's *Skill layer*.

**Architecture:** Three edits and one new test. The guard is written **first inside Task 1** and must go red against the three occurrences that exist today, then green once the two prose edits land — the assert must detect the state being removed, never confirm the wording being introduced. Task 2 adds the owning rule to the convention and handles that file's size budget.

**Tech Stack:** Bash test scripts under `tests/`, run as `bash tests/test_*.sh`. Markdown skill bodies under `skills/`. No build system, no test registry — the suite is a bare `tests/test_*.sh` glob, so a new test file self-registers.

## Global Constraints

- **The rule being enforced:** a docket-owned role skill body may name its role and the `skills.<role>` key that binds it, but never whether that binding is the shipped default, and never positions itself as an "alternative" to another role skill.
- **No behavior change** inside `docket-build`, `docket-review`, or `docket-brainstorm`. Prose and one test only.
- **Do not touch `README.md`.** It owns the brainstorm opt-in posture and `tests/test_consultant_brainstorm.sh` reads that prose from there (`assert "README states off-by-default"`). Repointing it is out of scope and would break a green assert for nothing.
- **Do not touch `skills/docket-review/SKILL.md`.** It already conforms — it contains no `superpowers:` reference at all.
- **A guard is code: mutation-test it** (AGENTS.md). Every assert in the new file must be shown red against the real defect before it is believed.
- **Confirm a mutation landed with a count** (`grep -c`) before trusting a green run.
- **Anchor on syntactic shape, never a line number** (ADR-0054, AGENTS.md). No assert in this change may reference a file:line.
- **Shell rules (AGENTS.md):** never `producer | grep -q` under `pipefail`; a pattern leading with `--` must use `-e`/`-F --`.
- **Size budgets** (`tests/test_skill_size_budgets.sh`) are `wc -l` / `wc -w` per file. Current actuals and ceilings, measured on this branch:
  - `skills/docket-build/SKILL.md` — 263/270 lines, 2417/2450 words (this change shrinks it)
  - `skills/docket-brainstorm/SKILL.md` — 76/84 lines, 629/692 words (this change shrinks it)
  - `skills/docket-convention/SKILL.md` — **361/365 lines, 6321/6350 words** (this change grows it; see Task 2 Step 4)

---

### Task 1: The negative guard, and the two prose deletions that make it pass

**Files:**
- Create: `tests/test_role_skill_self_description.sh`
- Modify: `skills/docket-build/SKILL.md` (the `# docket-build — profile-routed plan execution` heading's first paragraph)
- Modify: `skills/docket-brainstorm/SKILL.md` (frontmatter `description:`, and the `## Overview` first sentence)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `tests/test_role_skill_self_description.sh`, which Task 2 extends with one more assert. Its variable names are `REPO`, `fail`, and the function `assert(){ ... }` with signature `assert "<label>" '<shell expression>'` — the house harness copied from `tests/test_consultant_brainstorm.sh`.

**Why the guard and the edits are one task:** the guard's whole value is that it is red before the edits and green after. Splitting them would land a task whose deliverable is a failing suite.

- [ ] **Step 1: Write the failing test**

Create `tests/test_role_skill_self_description.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# tests/test_role_skill_self_description.sh — change 0194. A docket-owned role skill body names its
# role and its `skills.<role>` binding key; it never states whether that binding is the shipped
# default, and never positions itself as an "alternative" to another role skill. Defaults are owned
# by the docket-convention *Skill layer* role table and by README.md.
#
# NEGATIVE guard, deliberately limited — named here so a later reader does not over-trust it:
#   * LINE-SCOPED. A default claim split across two lines escapes it.
#   * VOCABULARY-SCOPED. A claim phrased without `alternative|default|instead of|opt-in` escapes it.
# The job is to catch recurrence of the exact construct change 0194 removed, not to prove the
# absence of every paraphrase. The non-vacuity block below is what keeps it from going quietly
# green if the pattern is ever broken.
# Run: bash tests/test_role_skill_self_description.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# The forbidden shape: a `superpowers:` reference co-occurring on one line with a
# default/alternative word. Kept in one place so the guard and its own non-vacuity probe below
# cannot drift apart.
CLAIM='superpowers:'
WORDS='alternative|default|instead of|opt-in'

# grep -c under `set -o pipefail` with no early-exiting consumer: capture, never pipe into head/-q.
claim_hits(){ grep -inE "$CLAIM" "$1" 2>/dev/null | grep -icE "$WORDS" || true; }

ROLE_SKILLS="docket-build docket-review docket-brainstorm"

for s in $ROLE_SKILLS; do
  f="$REPO/skills/$s/SKILL.md"
  # Non-vacuity anchor #1: the file the guard reads must exist and be non-empty, or every
  # absence assert below passes for reasons that have nothing to do with the property.
  assert "role skill exists and is non-empty: skills/$s/SKILL.md" '[ -s "$f" ]'
  [ -s "$f" ] || continue
  # Non-vacuity anchor #2: a live PRESENCE assert through the same file read. If the path is
  # wrong or the file is renamed, this reddens instead of the absence asserts going green.
  assert "skills/$s/SKILL.md names its own skill" 'grep -qF -- "$s" "$f"'
  n="$(claim_hits "$f")"
  assert "skills/$s/SKILL.md asserts no default status (found $n such lines)" '[ "$n" -eq 0 ]'
done

if [ "$fail" != 0 ]; then
  echo "REMEDY: a docket-owned role skill body names its role and its skills.<role> binding key,"
  echo "        never whether that binding is the shipped default. Delete the claim. Which skill a"
  echo "        role resolves to by default is owned by the docket-convention *Skill layer* role"
  echo "        table and by README.md — state it there, not here."
fi

# Non-vacuity anchor #3 (mutation-in-fixture): the matcher must actually FIRE on the shape it
# claims to reject. A typo in CLAIM or WORDS would otherwise make every assert above permanently
# green. This is the inversion mirrored-guard-enforces-its-own-property warns about.
probe="$(mktemp)"
printf '%s\n' 'The lean alternative to `superpowers:subagent-driven-development`.' > "$probe"
pn="$(claim_hits "$probe")"
assert "the matcher fires on a synthetic forbidden line (got $pn)" '[ "$pn" -eq 1 ]'
# And it must NOT fire on a conforming line that merely mentions another skill operationally.
printf '%s\n' 'Do NOT continue to `superpowers:writing-plans`; stop at the executed plan.' > "$probe"
cn="$(claim_hits "$probe")"
assert "the matcher ignores a bare operational reference (got $cn)" '[ "$cn" -eq 0 ]'
rm -f "$probe"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails against the real defect**

Run: `bash tests/test_role_skill_self_description.sh`

Expected: **FAIL**, with exactly these two asserts red and no others:
- `NOT OK - skills/docket-build/SKILL.md asserts no default status (found 1 such lines)`
- `NOT OK - skills/docket-brainstorm/SKILL.md asserts no default status (found 2 such lines)`

`docket-review` must be green already (it contains no `superpowers:` reference), and both synthetic-probe asserts must be green. If the counts differ from 1 and 2, stop and re-derive the inventory before editing anything — the guard is reporting a tree that does not match the plan.

Record the red output; it is the mutation evidence for this guard.

- [ ] **Step 3: Delete the claim in `skills/docket-build/SKILL.md`**

The first paragraph under `# docket-build — profile-routed plan execution` currently opens:

```markdown
The lean alternative to `superpowers:subagent-driven-development`. You are already running inside
`docket-implement-next` Step 5 with the plan written and the feature worktree cut. You read the
```

Replace the leading sentence so the paragraph opens on the role and its binding key, leaving the rest of the paragraph byte-identical:

```markdown
docket's build role, bound by `skills.build`. You are already running inside
`docket-implement-next` Step 5 with the plan written and the feature worktree cut. You read the
```

Constraints on the wording (the exact phrasing is yours, these are not):
- The first clause names the role and `skills.build`.
- No occurrence of `superpowers:subagent-driven-development` anywhere in the file.
- No *default*, *alternative*, *opt-in*, or *instead of* applied to this skill's own binding.
- Do not touch the frontmatter `description:` — it already conforms (`Use as docket's build role (skills.build) — …`).
- Do not touch the rest of the paragraph. The substantive contrast with SDD is already carried operationally further down the file by "no per-task review", which stays.

- [ ] **Step 4: Delete the claim in `skills/docket-brainstorm/SKILL.md` — two sites**

Site A — the frontmatter `description:`. It currently ends:

```
… Bindable via `skills: brainstorm:` (the 0049 passthrough); invoked by docket-new-change / docket-groom-next in place of the default `superpowers:brainstorming`.
```

Drop the trailing clause, keep the binding:

```
… Bindable via `skills: brainstorm:` (the 0049 passthrough); invoked by docket-new-change / docket-groom-next.
```

Site B — the `## Overview` opening sentence. It currently reads:

```markdown
`docket-brainstorm` is an opt-in alternative to the built-in `superpowers:brainstorming`
role. It keeps the ADR-0006 boundary — the design dialogue stays with the real human,
```

Replace the first sentence with one that opens on what the flow *is* and how it is bound, leaving the `It keeps the ADR-0006 boundary …` sentence and everything after it byte-identical:

```markdown
`docket-brainstorm` is docket's own brainstorm role, bound by `skills.brainstorm`.
It keeps the ADR-0006 boundary — the design dialogue stays with the real human,
```

Constraints:
- No *opt-in*, *alternative*, *default*, or *instead of* applied to this skill's own binding.
- **Leave lines 30 and 66's `superpowers:` references alone** — "identical in spirit to `superpowers:brainstorming`'s own dialogue" and "Do NOT continue to `superpowers:writing-plans`" are operational references carrying no default word, and the guard is written to pass them. Removing them is out of scope.
- This sentence is *accurate today*. It goes anyway: truth-at-time-of-writing is the property 0193's eight-file sweep proved worthless.

- [ ] **Step 5: Run the guard to verify it passes**

Run: `bash tests/test_role_skill_self_description.sh`

Expected: **PASS**, all asserts `ok`, including both synthetic probes.

- [ ] **Step 6: Prove the mutation, with counts**

Confirm the guard still bites by putting the defect back, running red, and reverting:

```bash
cd "$(git rev-parse --show-toplevel)"
grep -c 'lean alternative' skills/docket-build/SKILL.md || true   # expect 0
perl -0pi -e "s/^docket's build role, bound by \`skills\.build\`\./The lean alternative to \`superpowers:subagent-driven-development\`./m" skills/docket-build/SKILL.md
grep -c 'lean alternative' skills/docket-build/SKILL.md           # expect 1 — the mutation LANDED
bash tests/test_role_skill_self_description.sh; echo "exit=$?"    # expect FAIL, exit=1
git checkout -- skills/docket-build/SKILL.md
grep -c 'lean alternative' skills/docket-build/SKILL.md || true   # expect 0 again
bash tests/test_role_skill_self_description.sh; echo "exit=$?"    # expect PASS, exit=0
```

If the middle `grep -c` prints `0`, the substitution silently did not match — the "red" run proves nothing. Fix the substitution and redo. Do not proceed on an unlanded mutation.

- [ ] **Step 7: Run the whole suite**

Run: `for t in tests/test_*.sh; do echo "== $t"; bash "$t" | tail -1; done`

Expected: every file prints `PASS`. Pay particular attention to `tests/test_consultant_brainstorm.sh` (its README assert must still be green — README was not touched) and `tests/test_skill_size_budgets.sh` (both edited files shrank).

- [ ] **Step 8: Commit**

```bash
git add tests/test_role_skill_self_description.sh skills/docket-build/SKILL.md skills/docket-brainstorm/SKILL.md
git commit -m "docs(0194): drop default-status claims from the role skill bodies

Adds a line-scoped negative guard (mutation-proven red against both
occurrences) and deletes them: docket-build's 'lean alternative to
superpowers:subagent-driven-development' and docket-brainstorm's
'opt-in alternative' opener plus its description's 'in place of the
default' clause. Defaults stay owned by the convention and README."
```

---

### Task 2: Record the rule where it is owned, and pay for the words

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (the *Skill layer* bullet list, after the `- **Autonomy precedence …**` bullet)
- Modify: `tests/test_skill_size_budgets.sh` (the `BUDGETS` table row for `skills/docket-convention/SKILL.md`, plus its comment block)
- Modify: `tests/test_role_skill_self_description.sh` (one added assert)

**Interfaces:**
- Consumes: `tests/test_role_skill_self_description.sh` from Task 1 — same `REPO` / `fail` / `assert` harness.
- Produces: nothing later tasks rely on. This is the last task.

- [ ] **Step 1: Write the failing test**

Append this block to `tests/test_role_skill_self_description.sh`, immediately **before** the final `if [ "$fail" = 0 ]` line. It pins the *owner* — not a copy — so the remedy string printed above points at something that actually exists:

```bash
# The rule has a single home. The remedy above sends readers to the convention's *Skill layer*;
# assert it is really stated there, so the remedy can never become a pointer to nothing.
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "convention exists and is non-empty" '[ -s "$CONV" ]'
assert "convention *Skill layer* owns the role-self-description rule" \
  'grep -qiE "role skill (body )?(self-)?description" "$CONV" && grep -qF -- "skills.<role>" "$CONV"'
```

- [ ] **Step 2: Run the test to verify the new assert fails**

Run: `bash tests/test_role_skill_self_description.sh`

Expected: **FAIL**, with exactly one red assert:
- `NOT OK - convention *Skill layer* owns the role-self-description rule`

Everything from Task 1 must still be green. If `convention exists and is non-empty` is the one that reddens, the path is wrong — fix it rather than the assert.

- [ ] **Step 3: Add the bullet to the convention's *Skill layer***

In `skills/docket-convention/SKILL.md`, in the *Skill layer* section's bullet list, add this as the last bullet — immediately after the bullet beginning `- **Autonomy precedence — pre-specified at the call site.**`:

```markdown
- **Role skill self-description.** A role skill body names its `skills.<role>` binding key, never
  whether that binding is the default — this section's table and `README.md` own that.
```

This is exactly 2 lines and 28 words as written. It is a **single-home** addition, not a restatement: the *Skill layer* already owns role binding, and no other file carries a copy of this rule. Do not add a matching sentence to `README.md`, to the role skills, or to `docket-implement-next` — doing so recreates the exact class this change exists to remove.

- [ ] **Step 4: Pay for the words in the size budget, in the same diff**

The convention file measured 361 lines / 6321 words before this bullet, against a ceiling of 365/6350. The bullet takes it to 363/6349 — inside the line budget, but **1 word of margin**, which is the near-zero mode `tests/test_skill_size_budgets.sh`'s own comment block warns about repeatedly. Raise the **word** budget per that file's documented rounding rule (next multiple of 50; if that leaves under 25 words of margin, take the multiple after), and record why — a budget raise is a claim that the growth earned its words, and the comment is where that claim is made:

Measure first, do not predict:

```bash
wc -l -w skills/docket-convention/SKILL.md
```

Then, in `tests/test_skill_size_budgets.sh`, append to the comment block directly above the `BUDGETS=` assignment:

```
# skills/docket-convention/SKILL.md's WORD budget was raised 6350 -> 6400 by change 0194, which
# added the *Skill layer*'s role-self-description bullet: a role skill body names its skills.<role>
# binding key, never whether that binding is the shipped default. The bullet is the single home of
# a rule change 0193 proved is needed — that flip had to sweep eight files because the default was
# restated in each, and two role skill bodies still carried it. Stating it here is what stops the
# ninth and tenth accumulating, so the words are load-bearing rather than commentary. Set per the
# rounding rule above from the measured actual: 6349 words -> the next multiple of 50 is 6350,
# which leaves 1 word of margin (far under the 25-word threshold), so the multiple after was taken:
# 6400 (51 words of margin). The LINE budget was NOT raised (363 actual, 365 budget).
```

and change the table row from:

```
skills/docket-convention/SKILL.md                          365 6350
```

to:

```
skills/docket-convention/SKILL.md                          365 6400
```

Leave every other row untouched. If your measured word count differs from 6349, recompute the raise from *your* number using the same rule rather than keeping 6400.

- [ ] **Step 5: Run both tests to verify they pass**

Run: `bash tests/test_role_skill_self_description.sh && bash tests/test_skill_size_budgets.sh`

Expected: both print `PASS`. In the budget output specifically, confirm the line reads `ok - skills/docket-convention/SKILL.md within word budget (6349 <= 6400)` — a number other than your measured actual means the edit did not land where you thought.

- [ ] **Step 6: Prove the new assert bites, with counts**

```bash
cd "$(git rev-parse --show-toplevel)"
grep -c 'Role skill self-description' skills/docket-convention/SKILL.md   # expect 1
perl -0pi -e 's/- \*\*Role skill self-description\.\*\*/- **Something else entirely.**/' skills/docket-convention/SKILL.md
grep -c 'Role skill self-description' skills/docket-convention/SKILL.md   # expect 0 — mutation LANDED
bash tests/test_role_skill_self_description.sh; echo "exit=$?"            # expect FAIL, exit=1
git checkout -- skills/docket-convention/SKILL.md
grep -c 'Role skill self-description' skills/docket-convention/SKILL.md   # expect 1 again
bash tests/test_role_skill_self_description.sh; echo "exit=$?"            # expect PASS, exit=0
```

- [ ] **Step 7: Run the whole suite**

Run: `for t in tests/test_*.sh; do echo "== $t"; bash "$t" | tail -1; done`

Expected: every file prints `PASS`. `tests/test_convention_extraction.sh` and `tests/test_skill_size_budgets.sh` are the two most likely to react to a convention edit — read their output, do not just count PASS lines.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_skill_size_budgets.sh tests/test_role_skill_self_description.sh
git commit -m "docs(0194): give the role-self-description rule its single home

Adds the *Skill layer* bullet that owns the rule, an assert pinning the
owner so the guard's remedy points somewhere real, and the convention
word-budget raise (6350 -> 6400) that pays for it, per the budget file's
documented rounding rule."
```

---

## Notes for the reviewer

- **Nothing here is a behavior change.** Three prose edits and one new test file.
- **The guard's limits are stated in its own header comment**, deliberately: it is line-scoped and vocabulary-scoped, so a default claim spread over two lines or phrased without any anchor word escapes it. That is the accepted trade — the guard catches recurrence of the construct removed here, and the three non-vacuity anchors are what stop it going quietly green if its pattern breaks.
- **The budget raise in Task 2 is not the evasion** `guard-remedy-must-not-teach-the-evasion` warns about. That warning is about laundering a count to admit growth the guard existed to stop. Here the guard is a regrowth budget, the growth is a deliberate single-home rule addition, and the raise is an in-diff edit with its justification recorded in the file's own comment block, per the procedure that block documents. If a reviewer disagrees, the alternative is to trim the bullet to fit 6350 — not to leave a 1-word margin.
- **`README.md` and `skills/docket-review/SKILL.md` are untouched by design.** README owns the brainstorm opt-in posture (and a green assert reads it there); docket-review already conformed.
