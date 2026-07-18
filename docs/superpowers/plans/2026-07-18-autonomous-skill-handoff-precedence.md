# Autonomous skill hand-off precedence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop an autonomous docket run from being halted by an invoked role skill's interactive hand-off, by pre-specifying the outcome at every autonomous call site and guarding that coverage with a test.

**Architecture:** Three prose surfaces plus one guard. The load-bearing element is call-site pre-specification in `skills/docket-implement-next/SKILL.md` (§4 is the live defect; §5/§6 are hardened against a future plugin release growing a hand-off; §7 already carries the reference shape). `skills/docket-convention/SKILL.md`'s *Skill layer* states the precedence rule once, for durability when a future role binding is added. `skills/docket-finalize-change/SKILL.md:124` makes its human-present condition explicit as the single permitted exception. `tests/test_skill_handoff_precedence.sh` asserts **coverage** — every autonomous role invocation carries a pre-specified outcome — deriving the sites from a whole-repo grep rather than a hand-list.

**Tech Stack:** Bash (POSIX-ish, `set -uo pipefail`), markdown skill prose, the repo's `assert`-style test idiom (see `tests/test_skill_fork_dispatch.sh`).

## Global Constraints

- **Do not edit the superpowers plugin.** `writing-plans` is vendored under `~/.claude/plugins/cache/claude-plugins-official/superpowers/6.1.1/`; a local edit is overwritten on upgrade. Every fix lands in docket's own prose.
- **Phrase directions by shape, never by citing a vendored heading.** Write "any execution-mode or option choice it poses", never "§Execution Handoff" — a reference to the heading goes stale at superpowers 6.2 while still looking correct.
- **The house marker for a pre-specified outcome is the literal `DIRECTED to:`** at the invocation, matching `docket-implement-next` §7's existing wording. The guard keys on it and the convention documents it.
- **Skill size budgets are a live gate.** `tests/test_skill_size_budgets.sh` pins every `skills/**/*.md`. Headroom at plan time: `docket-convention/SKILL.md` 294/317 lines, 4769/5104 words; `docket-implement-next/SKILL.md` 127/140 lines, 2641/2845 words; `docket-finalize-change/SKILL.md` 132/160 lines, 2266/2699 words. Prefer extending existing lines over adding new ones. If a budget must rise, edit its row in `tests/test_skill_size_budgets.sh` **in the same diff**.
- **Guards are code (AGENTS.md).** Every new assert must be mutation-proven: strip the thing it guards, watch it redden. Key on syntactic shape, never an enumerated list of spellings, and derive gated sites from a whole-repo grep.
- **Shell rules (AGENTS.md).** Never `producer | grep -q` under `pipefail` — capture into a variable and use `grep -q <<<"$var"`. A pattern with a leading `--` needs `grep -qF --`.
- **Interactive skills are out of scope.** `docket-new-change` and `docket-groom-next` are *supposed* to prompt; the guard must skip them by construction, not by name.
- The suite is discovered by glob — a new `tests/*.sh` needs no registration.

---

### Task 1: The coverage guard

**Files:**
- Create: `tests/test_skill_handoff_precedence.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the executable contract Tasks 2–4 satisfy. Two assertion groups: (1) the convention's *Skill layer* states the precedence rule and names the `DIRECTED to:` marker; (2) every `$SKILL_<ROLE>` invocation inside a skill that has a wrapper at `agents/<skill-name>.md` carries `DIRECTED to:` on the same line, with exactly one permitted exception — a line carrying an explicit human-present condition, which must be in `docket-finalize-change`.

This task lands the test **red**. That is the point: per the spec, assertion group 2 must fail against today's §4 on arrival. Tasks 2–4 turn it green.

- [ ] **Step 1: Write the failing test**

Create `tests/test_skill_handoff_precedence.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# tests/test_skill_handoff_precedence.sh — change 0096.
# Guards the autonomy-precedence contract: an invoked role skill's interactive hand-off must never
# halt an autonomous run. Two groups:
#   (1) docket-convention's *Skill layer* states the precedence rule and names the call-site marker.
#   (2) COVERAGE — every autonomous invocation of a resolved role skill pre-specifies its outcome
#       (`DIRECTED to:`), with docket-finalize-change's human-present close-out the one exception.
# Group (2) is the load-bearing one: a presence-only check would test the durability prose (which
# demonstrably does not win at the moment of invocation) while leaving the mechanism unguarded.
# Sites are DERIVED from a whole-repo grep, never hand-listed (AGENTS.md: enumerated floor).
# Run: bash tests/test_skill_handoff_precedence.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# The literal marker a call site uses to pre-specify its outcome. One house token, documented in the
# convention — a shape the guard can actually check, not an open-ended paraphrase.
MARKER='DIRECTED to:'

# --- group 1: the convention states the rule ----------------------------------------------------
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "convention SKILL.md exists" '[ -f "$CONV" ]'
# Scope to the *Skill layer* section so a stray mention elsewhere cannot satisfy these.
LAYER="$(awk '/^### Skill layer/{f=1;next} f&&/^### /{exit} f' "$CONV")"
assert "the Skill layer section is non-empty" '[ -n "$LAYER" ]'
assert "Skill layer states an invoked skill never outranks the caller" 'grep -qi "never outranks" <<<"$LAYER"'
assert "Skill layer names pre-specification as the mechanism" 'grep -qi "pre-specif" <<<"$LAYER"'
assert "Skill layer names the call-site marker" 'grep -qF -- "$MARKER" <<<"$LAYER"'

# --- group 2: coverage over every autonomous role invocation ------------------------------------
# A skill is AUTONOMOUS iff sync-agents.sh generates a wrapper for it (agents/<skill>.md) — the same
# wrapper that carries the abort-and-report rule. Interactive skills have no wrapper and are skipped
# by construction, never by name: their prompts are the product.
SITES="$(grep -rn -e '\$SKILL_[A-Z]\{4,\}' "$REPO/skills" 2>/dev/null)"
assert "role-skill invocation sites were discovered" '[ -n "$SITES" ]'

checked=0
exceptions=0
while IFS= read -r entry; do
  [ -n "$entry" ] || continue
  file="${entry%%:*}"; rest="${entry#*:}"; lno="${rest%%:*}"; text="${rest#*:}"
  skill="$(basename "$(dirname "$file")")"
  # Skip interactive skills (no generated wrapper).
  [ -f "$REPO/agents/$skill.md" ] || continue
  rel="${file#"$REPO"/}"
  checked=$((checked+1))
  if grep -qi "human is present" <<<"$text"; then
    exceptions=$((exceptions+1))
    assert "$rel:$lno human-present exception belongs to docket-finalize-change" \
      '[ "$skill" = "docket-finalize-change" ]'
    continue
  fi
  assert "$rel:$lno autonomous role invocation pre-specifies its outcome" \
    'grep -qF -- "$MARKER" <<<"$text"'
done <<<"$SITES"

# The classifier must not go vacuous: if wrapper detection broke, every site would be skipped and
# every assert above would silently vanish.
assert "autonomous role invocations were actually checked (checked=$checked >= 5)" '[ "$checked" -ge 5 ]'
assert "exactly one human-present exception exists (found $exceptions)" '[ "$exceptions" -eq 1 ]'

# --- non-vacuity / mutation proof ---------------------------------------------------------------
# The marker check must reject an unmarked invocation line — the exact shape of today's defective §4.
UNMARKED='Run the **resolved plan skill** — `$SKILL_PLAN` from the Step-0 config export.'
assert "the marker check is non-vacuous (an unmarked invocation is caught)" \
  '! grep -qF -- "$MARKER" <<<"$UNMARKED"'
# The exception classifier must not match an ordinary invocation line.
assert "the exception classifier is non-vacuous (a plain line is not an exception)" \
  '! grep -qi "human is present" <<<"$UNMARKED"'

[ "$fail" -eq 0 ] && echo "ALL OK" || echo "FAILURES"
exit "$fail"
```

- [ ] **Step 2: Run it to verify it fails**

```bash
cd /Users/homer/dev/docket/.worktrees/suppress-plan-skill-execution-handoff
bash tests/test_skill_handoff_precedence.sh
```

Expected: `FAILURES`, exit 1. Specifically these `NOT OK` lines:
- `NOT OK - Skill layer states an invoked skill never outranks the caller`
- `NOT OK - Skill layer names pre-specification as the mechanism`
- `NOT OK - Skill layer names the call-site marker`
- `NOT OK - skills/docket-implement-next/SKILL.md:64 autonomous role invocation pre-specifies its outcome`
- `NOT OK - skills/docket-implement-next/SKILL.md:68 autonomous role invocation pre-specifies its outcome`
- `NOT OK - skills/docket-implement-next/SKILL.md:72 autonomous role invocation pre-specifies its outcome`

Line 80 (§7) and line 124 of `docket-finalize-change` should already be `ok` — §7 already carries `DIRECTED to:`, and :124 already opens "When a human is present". If either is `NOT OK`, stop: the premise the whole change rests on has moved.

- [ ] **Step 3: Mutation-prove the guard bites**

Temporarily strip the marker from the one site that already has it, confirm the guard reddens, then restore:

```bash
cp skills/docket-implement-next/SKILL.md /tmp/0096-mutation-backup.md
sed -i '' '80s/DIRECTED to: //' skills/docket-implement-next/SKILL.md
bash tests/test_skill_handoff_precedence.sh | grep -c 'NOT OK'
cp /tmp/0096-mutation-backup.md skills/docket-implement-next/SKILL.md
rm /tmp/0096-mutation-backup.md
git diff --stat skills/docket-implement-next/SKILL.md
```

Expected: the `NOT OK` count rises by exactly 1 versus Step 2 (line 80 joins the failures), and the final `git diff --stat` prints nothing — the file is byte-restored.

Also mutation-prove the exception classifier:

```bash
cp skills/docket-finalize-change/SKILL.md /tmp/0096-mutation-backup2.md
sed -i '' '124s/When a human is present, the/The/' skills/docket-finalize-change/SKILL.md
bash tests/test_skill_handoff_precedence.sh | grep -E 'NOT OK.*(docket-finalize-change|exactly one human-present)'
cp /tmp/0096-mutation-backup2.md skills/docket-finalize-change/SKILL.md
rm /tmp/0096-mutation-backup2.md
git diff --stat skills/docket-finalize-change/SKILL.md
```

Expected: at least two new `NOT OK` lines (the :124 site now demands the marker, and `exactly one human-present exception exists (found 0)` fails), and the file is byte-restored.

- [ ] **Step 4: Make it executable and commit**

```bash
chmod +x tests/test_skill_handoff_precedence.sh
git add tests/test_skill_handoff_precedence.sh
git commit -m "test(0096): guard autonomy precedence — coverage over autonomous role invocations"
```

---

### Task 2: The convention's precedence rule

**Files:**
- Modify: `skills/docket-convention/SKILL.md` — the *Skill layer* bullet list (currently ends with the `Resolution` bullet, around line 107)

**Interfaces:**
- Consumes: the `DIRECTED to:` marker defined by Task 1's guard.
- Produces: the durability prose that group-1 assertions check. It must contain the substrings `never outranks`, `pre-specif`, and the literal `DIRECTED to:` inside the `### Skill layer` section.

Budget discipline: `docket-convention/SKILL.md` has 23 spare lines and 335 spare words. This bullet must fit inside that — target ≤ 8 lines and ≤ 150 words.

- [ ] **Step 1: Add the precedence bullet**

In `skills/docket-convention/SKILL.md`, append one bullet to the *Skill layer* bullet list, immediately after the existing `- **Resolution** is deterministic via …` bullet and before the `### Directory layout` heading. Insert exactly:

```markdown
- **Autonomy precedence — pre-specified at the call site.** An invoked skill's interactive step never outranks the caller's autonomy contract. An **autonomous** caller (a skill with a generated wrapper, carrying its abort-and-report rule) states the outcome up front in its direction to a role skill — the house marker is `DIRECTED to:` — and answers any choice the sub-skill poses internally from already-resolved config, emitting one run-output line naming the role and skill **only when** a hand-off was actually met and suppressed. Phrase the direction by **shape** ("any execution-mode or option choice it poses"), never by citing a vendored heading a plugin upgrade would silently stale. This paragraph is durability for future bindings, **not** the enforcement — what beats a specific instruction read at the moment of invocation is a specific counter-instruction at that same moment, so a future slim must not keep it and drop the call-site directions. Interactive skills (`docket-new-change`, `docket-groom-next`) are unaffected — their prompts are the product — and `docket-finalize-change`'s human-present close-out is the one autonomous-file exception, stated as an explicit condition.
```

(One physical line, matching the file's existing one-paragraph-per-line style.)

- [ ] **Step 2: Run the guard to verify group 1 passes**

```bash
bash tests/test_skill_handoff_precedence.sh
```

Expected: all three `Skill layer …` assertions now print `ok`. The three `docket-implement-next` coverage assertions (lines 64/68/72) still print `NOT OK` — Task 3 fixes those. Overall still `FAILURES`.

- [ ] **Step 3: Verify the size budget still holds**

```bash
bash tests/test_skill_size_budgets.sh | grep -E 'docket-convention/SKILL.md|NOT OK'
```

Expected: both `docket-convention/SKILL.md within line budget` and `… within word budget` print `ok`, and no `NOT OK` lines appear anywhere. If either budget is now exceeded, tighten the bullet's wording first; only if it is genuinely load-bearing at that length, raise that file's row in `tests/test_skill_size_budgets.sh` in this same commit.

- [ ] **Step 4: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_skill_size_budgets.sh
git commit -m "docs(0096): state autonomy precedence once in the convention's Skill layer"
```

---

### Task 3: Pre-specify at every `docket-implement-next` call site

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md:64` (§4, plan — **the live defect**), `:68` (§5, build), `:72` (§6, review), `:80` (§7, finish — add the cross-reference only)

**Interfaces:**
- Consumes: the `DIRECTED to:` marker (Task 1) and the convention rule it cites (Task 2).
- Produces: coverage for all four `docket-implement-next` sites, turning group-2 assertions green for this file.

Per the `consolidation-flattens-caller-variance` finding: these four sites carry **real variance** — different artifacts, different stop-points, different fallbacks. Add the directive clause to each; do **not** template one sentence over all four and flatten what differs.

- [ ] **Step 1: §4 — direct the plan skill to write the plan and stop**

In `skills/docket-implement-next/SKILL.md` line 64, replace this exact substring:

```
Run the **resolved plan skill** — `$SKILL_PLAN` from the Step-0 config export (default `superpowers:writing-plans`; on `auto` or unavailability, apply the plan auto-fallback per the convention's *Skill layer* — author the plan file yourself, warning prominently).
```

with:

```
Run the **resolved plan skill** — `$SKILL_PLAN` from the Step-0 config export (default `superpowers:writing-plans`) — **DIRECTED to:** write the plan file and stop there. Any execution-mode or option choice it poses is answered internally from the already-resolved config — step 5 runs `$SKILL_BUILD` — and never surfaced; log one line naming the role and skill if you suppressed one. On `auto` or unavailability, apply the plan auto-fallback per the convention's *Skill layer* — author the plan file yourself, warning prominently.
```

- [ ] **Step 2: §5 — direct the build skill**

In the same file, line 68, replace this exact substring:

```
The **resolved build skill** — `$SKILL_BUILD` from the Step-0 config export (default `superpowers:subagent-driven-development`) — executes the plan task-by-task; SDD does TDD + per-task review.
```

with:

```
The **resolved build skill** — `$SKILL_BUILD` from the Step-0 config export (default `superpowers:subagent-driven-development`) — is invoked **DIRECTED to:** execute the plan task-by-task and stop at the executed plan, answering any choice it poses from resolved config; SDD does TDD + per-task review.
```

- [ ] **Step 3: §6 — direct the review skill**

In the same file, line 72, replace this exact substring:

```
The **resolved review skill** — `$SKILL_REVIEW` from the Step-0 config export (default `superpowers:requesting-code-review`) — whole-branch;
```

with:

```
The **resolved review skill** — `$SKILL_REVIEW` from the Step-0 config export (default `superpowers:requesting-code-review`) — is invoked **DIRECTED to:** review the whole branch and return its findings, then stop, answering any choice it poses from resolved config;
```

- [ ] **Step 4: §7 — cite the general rule**

In the same file, line 80, replace this exact substring:

```
Pre-specifying the outcome keeps it non-interactive while reusing its push/PR mechanics.
```

with:

```
Pre-specifying the outcome keeps it non-interactive while reusing its push/PR mechanics — this is the reference shape for the convention's *Skill layer* autonomy-precedence rule.
```

- [ ] **Step 5: Run the guard — group 2 must be fully green for this file**

```bash
bash tests/test_skill_handoff_precedence.sh
```

Expected: every `skills/docket-implement-next/SKILL.md:<n> autonomous role invocation pre-specifies its outcome` line prints `ok` (four of them). Note the line numbers may have shifted if any replacement spanned a newline — it should not; every replacement above stays within its existing physical line.

- [ ] **Step 6: Verify the size budget**

```bash
bash tests/test_skill_size_budgets.sh | grep -E 'docket-implement-next/SKILL.md|NOT OK'
```

Expected: both `docket-implement-next/SKILL.md` budget lines print `ok` and no `NOT OK` appears. Line count must be unchanged (127) since every edit stayed inline; the word count rises by roughly 80.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-implement-next/SKILL.md tests/test_skill_size_budgets.sh
git commit -m "fix(0096): pre-specify the outcome at every implement-next role call site"
```

---

### Task 4: The finalize exception, made explicit — and the whole-suite gate

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md:124`

**Interfaces:**
- Consumes: the exception classifier from Task 1 (a line matching `human is present`, which must belong to `docket-finalize-change`).
- Produces: a green `tests/test_skill_handoff_precedence.sh` and a green whole suite.

- [ ] **Step 1: Make the human-present condition read as *the* exception**

In `skills/docket-finalize-change/SKILL.md` line 124, replace this exact substring:

```
When a human is present, the resolved finish skill — `$SKILL_FINISH` (default `superpowers:finishing-a-development-branch`) — can drive a non-standard close-out (keep, discard, or merge locally without a PR); its chooser fits at step 4.
```

with:

```
When a human is present — the **one** exception to the *Skill layer*'s autonomy-precedence rule, and conditional on exactly that — the resolved finish skill — `$SKILL_FINISH` (default `superpowers:finishing-a-development-branch`) — can drive a non-standard close-out (keep, discard, or merge locally without a PR); its chooser fits at step 4. On the autonomous path there is no chooser: finalize pre-specifies its outcome exactly as `docket-implement-next` §7 does.
```

- [ ] **Step 2: Run the guard — it must now be fully green**

```bash
bash tests/test_skill_handoff_precedence.sh
```

Expected: `ALL OK`, exit 0. Confirm `autonomous role invocations were actually checked (checked=5 >= 5)` and `exactly one human-present exception exists (found 1)` both print `ok`.

- [ ] **Step 3: Verify the size budget**

```bash
bash tests/test_skill_size_budgets.sh
```

Expected: `ALL OK` / no `NOT OK` lines — every budgeted file, not just the three this change touched.

- [ ] **Step 4: Run the WHOLE suite (build gate)**

Per AGENTS.md: run the whole suite at the build gate, never only the tests the spec enumerated. Run it in ONE foreground call:

```bash
cd /Users/homer/dev/docket/.worktrees/suppress-plan-skill-execution-handoff
for t in tests/test_*.sh; do
  out="$(bash "$t" 2>&1)"
  if grep -q 'NOT OK' <<<"$out"; then echo "=== FAIL: $t"; grep 'NOT OK' <<<"$out"; else echo "ok - $t"; fi
done
```

Expected: every line prints `ok - tests/…`. Any `=== FAIL:` is a real regression to fix before committing — a prose edit to a skill can trip a sentinel in `test_convention_extraction.sh`, `test_composition_wiring.sh`, or `test_skill_facade_wiring.sh` that pinned a phrase this change reworded.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md
git commit -m "docs(0096): name finalize's human-present close-out as the one precedence exception"
```

---

## Self-review notes

**Spec coverage.** §1 call-site pre-specification → Task 3 (implement-next §4/§5/§6/§7) + Task 4 (finalize:124). §2 convention precedence rule → Task 2. §3 conditional suppression trace → the "log one line naming the role and skill if you suppressed one" clause in Task 3 Step 1, plus the convention bullet's "only when a hand-off was actually met and suppressed" in Task 2; deliberately unguarded, per the spec (it depends on model judgment and is a breadcrumb, not enforcement). §4 guard → Task 1, with the mutation proofs AGENTS.md requires.

**Deliberate scope note.** The spec's design section names only §4, §7 and finalize:124 as edited, but its guard is coverage-shaped ("every autonomous invocation"). §5 and §6 therefore get the marker too — otherwise the guard as specified fails on arrival at sites the design never mentions. This hardens them against a future superpowers release growing a hand-off in `subagent-driven-development` or `requesting-code-review`, which is the stated reason the trace is conditional rather than unconditional.

**Intermediate states are red on purpose.** Task 1 lands a failing test; Tasks 2–4 each turn a slice green. Only after Task 4 is the suite green.
