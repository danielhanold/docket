<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0170 — Lean Docket-owned whole-branch review skill](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0170-lean-whole-branch-review-skill.md)**
<!-- docket:backlink:end -->

# Lean whole-branch review skill + suite-once evidence chain — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `docket-review` — docket's own bounded, read-only whole-branch review role behind three pinned rung wrappers — plus the build-evidence chain that makes the full test suite run exactly once during implementation and conditionally once more at finalize.

**Architecture:** Three new generated wrappers (`docket-review-lean/-standard/-deep`) wrap one new `skills/docket-review/SKILL.md`, joining the roster through the existing `agents/docket-*.md` glob — no generator code changes. `docket-build`'s existing full-suite gate additionally emits a structured **build-evidence** record (command, result, head SHA, timestamp). `docket-implement-next` Step 6 validates that evidence, picks a reviewer rung deterministically from the build's highest routed profile, dispatches it foreground, triages severity-tiered findings, and Step 7 writes the evidence block durably into the PR body. `docket-finalize-change` reads that block and skips its post-rebase suite run only when the rebase was a no-op and the evidence is green at the exact HEAD being merged.

**Tech Stack:** Bash 3.2-compatible shell, markdown skill/agent contracts, the repo's flat `tests/test_*.sh` suite.

## Global Constraints

- **This is a prose-and-config change with guard tests.** There is no application code. The deliverables are markdown contracts, one YAML sidecar, and Bash guards. A guard is code: mutation-test every new assert (strip the thing it guards, watch it redden) or it is decoration.
- **Repo instructions bind.** `AGENTS.md` at the repo root is always in context. In particular: never `producer | early-exiting-consumer` under `set -o pipefail`; `grep` for a leading `--` pattern must use `-e`/`-F --`; awk indent classes are `[^[:space:]]`; anchor frontmatter edits to the first `---…---` block; validate marker order and balance before rewriting a marker-delimited block; never hand-list the sites of a literal you are gating; cross-references anchor on a symbol name or a verbatim-quoted clause, **never a line number**.
- **Portability.** The interactive shell's `grep` is ugrep and accepts constructs BSD `grep` rejects. Any new regex in a test must be re-checked under `/usr/bin/grep` before the task is called complete.
- **Run tests with the configured runtime:** `"$DOCKET_BASH_PATH" tests/test_<name>.sh`. The whole suite is `for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done`.
- **The wrapper roster count changes 13 → 16 exactly once**, in Task 2. Every count surface and every count guard moves in that one commit, or the suite is red between commits.
- **Banned literals in wrapper/fragment prose:** `tests/test_docket_build.sh` bans the bare words `low`, `medium`, and `high` from every `agents/docket-*.md` and `cursor-rules/dispatch/docket-*.md`. The three review wrappers and fragments must describe their rung without those words.
- **Banned literals in finalize prose:** `tests/test_finalize_gate.sh` forbids `opus`, `sonnet`, `haiku`, `fable`, and `xhigh` anywhere in `skills/docket-finalize-change/SKILL.md`.
- **Size budgets are enforced.** `tests/test_skill_size_budgets.sh` auto-discovers every `skills/**/*.md` and fails on any file with no budget row. Its rounding rule: lines → next multiple of 5, words → next multiple of 50; if that leaves within 25 words of the measured actual, take the multiple *after*. Every budget change ships with a prose paragraph in that file's header comment explaining the raise, following the existing house style.
- **The reviewer pin table introduces no new model ID or pair.** Every value reuses a pair already shipped in `agents/harness-defaults.yml`. This is deliberate — per the learnings finding `external-truth-needs-a-human-checkpoint`, a genuinely new vendor ID would need a human verification item because no in-repo test can be its oracle. Do not substitute a different ID.

---

## File Structure

**New files**

| Path | Responsibility |
|---|---|
| `skills/docket-review/SKILL.md` | The review role contract: read-only conduct, evidence verification, finding schema, one-shot no-escalation rule. |
| `agents/docket-review-lean.md` | Rung wrapper — cheapest reviewer. Wraps `docket-review` only. |
| `agents/docket-review-standard.md` | Rung wrapper — common case. |
| `agents/docket-review-deep.md` | Rung wrapper — cap rung; matches the build-max pin on every harness. |
| `cursor-rules/dispatch/docket-review-lean.md` | Cursor dispatch fragment. |
| `cursor-rules/dispatch/docket-review-standard.md` | Cursor dispatch fragment. |
| `cursor-rules/dispatch/docket-review-deep.md` | Cursor dispatch fragment. |
| `tests/test_docket_review.sh` | All new guards: skill contract, rung table, evidence producer + consumers, finalize skip predicate. |

**Modified files**

| Path | Change |
|---|---|
| `agents/harness-defaults.yml` | +9 rows (3 rungs × claude/cursor/codex). |
| `.docket.example.yml` | Count prose + 9 mirrored rows across the three commented blocks. |
| `.docket.yml` | `skills: review: docket-review` — this repo's dogfood opt-in. |
| `skills/docket-build/SKILL.md` | The gate emits the build-evidence record. |
| `skills/docket-implement-next/SKILL.md` | Step 6 rung selection + evidence validation + triage; Step 7 PR-body block. |
| `skills/docket-finalize-change/SKILL.md` | Conditional post-rebase suite skip. |
| `skills/docket-convention/SKILL.md` | Wrapper counts/enumeration (+ the 0184 off-by-one fix), Skill-layer review row. |
| `README.md` | Two counts, `## Skills` catalog row, new `### docket-review` section, suite-placement rationale. |
| `tests/test_skill_size_budgets.sh` | One new row + four raises + header paragraphs. |
| `tests/test_sync_agents.sh` | Two `13` → `16`. |
| `tests/test_sync_agents_cursor.sh` | One `13` → `16`. |
| `tests/test_sync_agents_codex.sh` | One equality count + two `-ge` floors. |
| `tests/test_cursor_dispatch_rule.sh` | Fragment population floor. |
| `tests/test_finalize_gate.sh` | Convention count asserts; "six skills" → "seven skills". |
| `tests/test_skill_fork_dispatch.sh` | A comment recording why `docket-review` is in neither list. |

**Deliberate non-changes**

- `sync-agents.sh`, `scripts/lib/harness-defaults.sh`, `link-skills.sh` — all glob-driven. Reconcile verified this (learnings: `check-plumbing-auto-discovery`). If a task finds itself editing a generator, stop and re-read: the premise is wrong.
- `tests/test_dispatch_capability.sh`'s `check_site` roster, its `-eq 5` floor, and `PENDING_TIER` — see Task 4's Step 1 for why the rung dispatch must not add a site, and how to prove it.

---

### Task 1: The `docket-review` skill contract

**Files:**
- Create: `skills/docket-review/SKILL.md`
- Modify: `tests/test_skill_size_budgets.sh` (new BUDGETS row + header paragraph)
- Modify: `README.md` (`## Skills` catalog row)
- Modify: `tests/test_skill_fork_dispatch.sh` (decision comment)
- Test: `tests/test_docket_review.sh` (create)

**Interfaces:**
- Produces: the skill file `skills/docket-review/SKILL.md`, whose `name:` is `docket-review` and whose `description:` is copied **byte-for-byte** into the three wrappers in Task 2. The finding schema fields (`severity`, `location`, `summary`, `rationale`, `suggested_fix`) and the verdict line format are consumed by Task 4's triage prose.
- Consumes: nothing from earlier tasks.

- [ ] **Step 1: Write the failing test**

Create `tests/test_docket_review.sh`. Copy the header/harness shape from an existing guard (`tests/test_docket_build.sh` is the closest sibling — same `assert` helper, same `REPO` resolution). This first cut guards only the skill contract; later tasks append sections to this same file.

```bash
#!/usr/bin/env bash
# tests/test_docket_review.sh — guards change 0170's review role: the docket-review skill
# contract, the three rung wrappers, the build-evidence chain, and finalize's conditional skip.
# Run: bash tests/test_docket_review.sh
set -u

REPO="$(cd "$(dirname "$0")/.." && pwd)"
fails=0
assert(){ if eval "$2"; then echo "ok   - $1"; else echo "FAIL - $1"; fails=$((fails+1)); fi; }

REV="$REPO/skills/docket-review/SKILL.md"

# --- the skill exists and declares itself -------------------------------------------------
assert "docket-review skill exists" '[ -f "$REV" ]'
assert "docket-review frontmatter name is docket-review" \
  'awk "/^---$/{n++; next} n==1" "$REV" | grep -qE "^name: docket-review$"'

# --- read-only conduct: the properties that make the verdict trustworthy -------------------
# Each is a distinct promise; a single "read-only" mention would not prove any of them.
assert "conduct: forbids running the test suite" \
  'grep -qiE "never runs? the (full )?(test )?suite" "$REV"'
assert "conduct: forbids writing, committing, or checking out" \
  'grep -qiE "never (writes|commits|checks out)" "$REV"'
assert "conduct: forbids dispatching subagents" \
  'grep -qiE "never dispatches" "$REV"'
assert "conduct: no reviewer escalation ladder" \
  'grep -qiE "never re-dispatches itself|no .{0,20}escalation" "$REV"'

# --- the finding schema: every field a triaging controller must be able to read ------------
for f in severity location summary rationale suggested_fix; do
  assert "finding schema names the '$f' field" 'grep -qF -- "$f" "$REV"'
done
for s in blocker important minor; do
  assert "finding schema names the '$s' severity" 'grep -qE "\`$s\`|\*\*$s\*\*" "$REV"'
done

# --- the evidence backstop finding --------------------------------------------------------
# The reviewer's ONLY answer to bad evidence is a finding; it must never run the suite itself.
assert "reviewer reports unverified-build-state rather than running the suite" \
  'grep -qF -- "unverified-build-state" "$REV"'
assert "reviewer verifies the evidence head_sha against the branch HEAD" \
  'grep -qF -- "head_sha" "$REV"'

echo "---"; [ "$fails" -eq 0 ] && echo "PASS" || { echo "FAIL ($fails)"; exit 1; }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `"$DOCKET_BASH_PATH" tests/test_docket_review.sh`
Expected: FAIL — `docket-review skill exists` reddens first, and every subsequent assert fails because `$REV` does not exist.

- [ ] **Step 3: Write the skill**

Create `skills/docket-review/SKILL.md`. Target **≤ 110 lines / ≤ 950 words** (budget row in Step 4 is 115/1000). Model the register on `skills/docket-build-task/SKILL.md` — a compact worker contract, normative, no restatement of the convention (the wrappers do not load it).

Required content, in this order:

1. **Frontmatter** — `name: docket-review`, and a `description:` written as one sentence in the house style, e.g.:
   `Bounded read-only whole-branch reviewer for docket's review role — reads the branch diff and the build-evidence record, returns severity-tiered findings, and never fixes, dispatches, or runs the test suite.`
   Record this exact string; Task 2 copies it verbatim into all three wrappers.
2. **`## Scope`** — one paragraph stating the metadata boundary the wrappers cannot get from `docket-convention` (they do not load it): this agent touches no docket metadata, no change file, no board, no ADR. It reads the feature branch and returns findings.
3. **`## Inputs`** — what the dispatch prompt carries: branch name and base ref, the change's PM-altitude context (title, `## Why`, `## What changes`), the relevant learnings hooks the controller pulled, and the current build-evidence record.
4. **`## Conduct`** — the read-only rules, phrased so the Step-1 asserts hit:
   - may run read-only commands (`git diff`, `git log`, `git show`, greps);
   - **never writes** files, **never commits**, **never checks out** (the worktree is shared — see the learnings finding `no-checkout-in-shared-worktree`), **never dispatches** subagents, and **never runs the test suite**;
   - one shot at the dispatched rung: a reviewer that cannot complete aborts and reports; it **never re-dispatches itself** upward, and there is no escalation ladder.
5. **`## Verifying the build evidence`** — the reviewer checks that the evidence record is present, `result: green`, and `head_sha` equal to the branch HEAD it is reviewing. A missing, malformed, or stale record is returned as a **blocker** finding with the summary `unverified-build-state`. State explicitly that running the suite is **not** an available remedy — the controller owns that.
6. **`## What to review`** — the whole-branch diff for correctness, design soundness, contract violations, and test-coverage gaps the suite cannot see. Explicitly out of scope: re-litigating profile routing or TDD mechanics (the build's own discipline), and anything the suite already proves.
7. **`## Return schema`** — the table with `severity` / `location` / `summary` / `rationale` / `suggested_fix`, the severity definitions (**blocker** = would ship a real defect; **important** = should be addressed but survivable in an open PR; **minor** = style/polish), and the closing rule: return the finding list plus a one-line verdict — `clean`, or `N findings: B blocker, I important, M minor` — and nothing else. No prose report. An empty list is a valid, expected return.
8. **`## Halting`** — abort-and-report posture: an unmet precondition or blocking ambiguity is surfaced and stopped on, never turned into an interactive prompt.

Do **not** add `context: fork` or `agent:` frontmatter — this skill is reached by wrapper dispatch, exactly like `docket-build-task`.

- [ ] **Step 4: Add the budget row**

In `tests/test_skill_size_budgets.sh`, add to `BUDGETS` (keep the block's alphabetical-by-path ordering, so the row lands after `skills/docket-implement-next/results-template.md` and before `skills/docket-new-change/SKILL.md`):

```
skills/docket-review/SKILL.md                              115 1000
```

Measure first — `wc -l` and `wc -w` on the file you just wrote — then apply the rounding rule and use the real numbers if they differ from 115/1000. Append a paragraph to the header comment block in that file's existing style, naming change 0170, what the file contains, and the measured actuals the budget was derived from.

- [ ] **Step 5: Add the README catalog row**

`tests/test_readme_skill_catalog.sh` runs a forward **and** reverse correspondence between `skills/*/SKILL.md` directories and the README `## Skills` table. Add a `docket-review` row to that table matching the surrounding rows' shape and column count. Read the neighbouring rows first and match them exactly.

- [ ] **Step 6: Record the fork-list decision**

`tests/test_skill_fork_dispatch.sh` hand-lists `FORKED` and `EXCLUDED`. `docket-review` belongs to **neither** — it is wrapper-dispatched, not forked, exactly like `docket-build` and `docket-build-task`, which are also in neither list. Add a comment immediately above the `EXCLUDED` assignment recording that placement as a decision rather than an omission:

```bash
# Wrapper-dispatched skills (docket-build, docket-build-task, docket-review) are in NEITHER list
# by design: they are reached through a pinned wrapper, never through Claude Code's context: fork
# path, so neither the forked-frontmatter assert nor the must-not-fork assert applies to them.
```

- [ ] **Step 7: Run the new test and the affected guards**

Run:
```bash
"$DOCKET_BASH_PATH" tests/test_docket_review.sh
"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh
"$DOCKET_BASH_PATH" tests/test_readme_skill_catalog.sh
"$DOCKET_BASH_PATH" tests/test_skill_fork_dispatch.sh
```
Expected: all PASS.

- [ ] **Step 8: Mutation-test the new guards**

For at least three of the new asserts, break the thing they guard and confirm the assert reddens, then restore. Concretely: delete the `never runs the test suite` clause (the suite assert must redden); delete the `unverified-build-state` string (its assert must redden); remove the `suggested_fix` row from the schema table (its assert must redden). Restore the file to green after each. Record the outcome in the commit message.

- [ ] **Step 9: Commit**

```bash
git add skills/docket-review/SKILL.md tests/test_docket_review.sh \
        tests/test_skill_size_budgets.sh tests/test_skill_fork_dispatch.sh README.md
git commit -m "feat(0170): the docket-review skill — bounded read-only whole-branch reviewer"
```

---

### Task 2: The three rung wrappers, their pins, and the 13 → 16 roster move

**Files:**
- Create: `agents/docket-review-lean.md`, `agents/docket-review-standard.md`, `agents/docket-review-deep.md`
- Create: `cursor-rules/dispatch/docket-review-lean.md`, `-standard.md`, `-deep.md`
- Modify: `agents/harness-defaults.yml`
- Modify: `.docket.example.yml`
- Modify: `skills/docket-convention/SKILL.md` (count + enumeration prose only)
- Modify: `README.md` (the two count sentences)
- Modify: `tests/test_sync_agents.sh`, `tests/test_sync_agents_cursor.sh`, `tests/test_sync_agents_codex.sh`, `tests/test_cursor_dispatch_rule.sh`, `tests/test_finalize_gate.sh`
- Test: `tests/test_docket_review.sh` (append a rung-table section)

**Interfaces:**
- Consumes: Task 1's `docket-review` skill and its exact `description:` string.
- Produces: three registered wrapper agents named `docket-review-lean`, `docket-review-standard`, `docket-review-deep`. Task 4's rung-selection rule maps build profiles onto exactly these three names.

**Why this is one task and not five:** the roster count is a single atomic fact. `hd_validate` refuses to generate any wrapper unless every shipped harness block covers every `agents/docket-*.md`, and five separate guards assert the count literally. Splitting this leaves the suite red between commits.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_review.sh`, before the trailing `echo "---"` summary block:

```bash
# --- the three rung wrappers ---------------------------------------------------------------
HD="$REPO/agents/harness-defaults.yml"
REV_DESC="$(awk "/^---$/{n++; next} n==1" "$REV" | sed -n 's/^description: //p')"
assert "the skill's description is non-empty (anchor for the wrapper compare)" '[ -n "$REV_DESC" ]'

for rung in lean standard deep; do
  W="$REPO/agents/docket-review-$rung.md"
  assert "wrapper exists: docket-review-$rung" '[ -f "$W" ]'
  [ -f "$W" ] || continue
  assert "docket-review-$rung: name matches its filename" \
    'grep -qE "^name: docket-review-'"$rung"'$" "$W"'
  # Byte-equality with the skill's own description is the house rule for wrappers.
  assert "docket-review-$rung: description matches the skill's" \
    'wd="$(sed -n "s/^description: //p" "$W")"; [ "$wd" = "$REV_DESC" ]'
  # The wrapper wraps the review skill ONLY — no docket-convention, mirroring the build workers.
  assert "docket-review-$rung: injects docket-review" 'grep -qF -- "skills: [docket-review]" "$W"'
  assert "docket-review-$rung: does NOT inject docket-convention" \
    '! grep -qF -- "docket-convention" "$W"'
  assert "docket-review-$rung: carries the abort-and-report posture" \
    'grep -qF -- "abort-and-report" "$W"'
  # No pins live in wrapper files since change 0168 — they live in the sidecar.
  assert "docket-review-$rung: carries no model/effort pin" \
    '! grep -qE "^(model|effort):" "$W"'
  # Every shipped harness must supply a pair, or generation fails outright.
  for h in claude cursor codex; do
    assert "harness-defaults: $h supplies a pair for review-$rung" \
      'grep -qE "^ *review-'"$rung"': *\{ *model: *[^ ,}]+, *effort: *[^ ,}]+ *\}" "$HD"'
  done
  F="$REPO/cursor-rules/dispatch/docket-review-$rung.md"
  assert "cursor dispatch fragment exists: docket-review-$rung" '[ -f "$F" ]'
done

# The cap-rung invariant, asserted per harness rather than asserted in prose: review-deep is
# pinned exactly where build-max is, so the cap rung never reviews below the strength the
# riskiest build work was built with.
for h in claude cursor codex; do
  assert "$h: the review-deep pin equals the build-max pin" \
    'blk="$(awk "/^  '"$h"':/{f=1;next} /^  [a-z]+:/{f=0} f" "$HD")";
     d="$(printf "%s" "$blk" | sed -n "s/^ *review-deep: *//p")";
     m="$(printf "%s" "$blk" | sed -n "s/^ *build-max: *//p")";
     [ -n "$d" ] && [ "$d" = "$m" ]'
done

# The rung wrappers must NOT introduce a new dispatch site into test_dispatch_capability.sh's
# reverse-correspondence population. The four docket-build-* workers set the precedent: they are
# referred to as profile agents, never in the `name`-near-"subagent" shape that guard derives on.
assert "rung dispatch prose avoids the derived-dispatch-site shape" \
  '! grep -rohE --include="*.md" "\`docket-review-[a-z]+\`[^\`]{0,20}subagent" "$REPO/skills/" | grep -q .'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `"$DOCKET_BASH_PATH" tests/test_docket_review.sh`
Expected: FAIL on every `wrapper exists` assert and every `harness-defaults` pair assert.

- [ ] **Step 3: Write the three wrapper files**

Each is a thin file with exactly three frontmatter keys — `name`, `description`, `skills` — mirroring `agents/docket-build-economy.md`. **`description:` must be byte-identical to the skill's** (Task 1 Step 3). No `model:` or `effort:` line. Remember the banned bare words `low`/`medium`/`high`.

`agents/docket-review-lean.md`:

```markdown
---
name: docket-review-lean
description: <PASTE the exact description: string from skills/docket-review/SKILL.md>
skills: [docket-review]
---
Review the whole feature branch handed to you, following the docket-review skill exactly.

You were routed to the LEAN rung because the build it reviews stayed on its cheapest profile throughout — no task escalated, and the branch diff is small. Read the diff, verify the build evidence, and return findings. Do not fix anything, do not run the test suite, and do not re-dispatch yourself to a stronger rung: one rung, one pass.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
```

`agents/docket-review-standard.md` — same shape; second paragraph opens: `You were routed to the STANDARD rung because the build routed or escalated a task to its standard profile.` Keep the same three closing rules.

`agents/docket-review-deep.md` — same shape; second paragraph opens: `You were routed to the DEEP rung because the build reached one of its two strongest profiles, or because the branch diff crossed the size threshold.` Add: `There is no rung above you — findings you miss reach the human as a merged defect.`

- [ ] **Step 4: Add the nine sidecar rows**

In `agents/harness-defaults.yml`, add three rows to each of the `claude:`, `cursor:`, and `codex:` blocks. Place them **alphabetically among the existing keys** where each block's ordering makes that natural (after `rebase-resolver`, before `status`) — match the surrounding block's alignment style exactly. Values, verbatim:

```yaml
  # claude:
    review-lean:           { model: claude-sonnet-5, effort: high }
    review-standard:       { model: claude-opus-5, effort: medium }
    review-deep:           { model: claude-opus-5, effort: high }

  # cursor:
    review-lean:           { model: cursor-grok-4.5-medium, effort: auto }
    review-standard:       { model: cursor-grok-4.5-high, effort: auto }
    review-deep:           { model: claude-opus-5-high, effort: auto }

  # codex:
    review-lean:           { model: gpt-5.6-terra, effort: medium }
    review-standard:       { model: gpt-5.6-terra, effort: high }
    review-deep:           { model: gpt-5.6-sol, effort: medium }
```

Add a short comment above the claude review rows recording the ladder's reasoning: the rungs price *review* work directly rather than shifting the build ladder by one, and on every harness **review-deep equals the build-max pin**. Note that the lean rung deliberately departs from a literal one-above-the-build on claude, because the build-economy comment's reason for avoiding a smaller model (a contract fumble halts the build) does not apply to a read-and-return reviewer.

Values are bare scalars — unquoted and space-free — or `hd_validate` rejects them.

- [ ] **Step 5: Write the three Cursor dispatch fragments**

Model each on `cursor-rules/dispatch/docket-build-economy.md`. Fragment shape rules enforced by `tests/test_cursor_dispatch_rule.sh`: non-indented instruction prose must **not** contain the words `Task` or `Agent`; the file must match `dispatch (to|the)` case-insensitively; and if it contains an indented `^    Word(` code block, it must also contain the word `illustration`. Banned bare words `low`/`medium`/`high` apply here too.

`cursor-rules/dispatch/docket-review-lean.md`:

```markdown
## docket-review-lean — dispatch only

Trigger only from the `docket-implement-next` controller at its review step, when it has selected
the LEAN reviewer rung. Never trigger this agent from a human request directly.

Dispatch to the subagent `docket-review-lean`, foreground, using this mode's subagent-launch
mechanism. The prompt must carry the branch and its base ref, the change's title and scope
sections, the relevant learnings hooks, and the current build-evidence record.

The reviewer is read-only: it returns findings and never fixes, never commits, and never runs the
test suite. Do NOT dispatch a second reviewer afterwards.

One concrete call, as an illustration of the shape — not the contract:

    Task(subagent_type: "docket-review-lean", run_in_background: false,
         prompt: "Review branch feat/<slug> against origin/main. Rung: lean (build stayed on its cheapest profile). Evidence: <block>. <context>")
```

Write `-standard.md` and `-deep.md` the same way, changing only the agent name, the rung word, and the parenthesized selection reason.

- [ ] **Step 6: Update the count prose (three files)**

Each of these is a restatement with its own guard — per the learnings finding `restatement-accumulates-its-own-guards`, grep the suite for the prose you are changing before changing it.

1. `skills/docket-convention/SKILL.md`, the *Composition* paragraph: **thirteen → sixteen**, and the parenthetical roster tally. The new tally: *five wrap the five autonomous skills, four are docket-build's task workers sharing the `docket-build-task` contract, three are docket-review's rung wrappers sharing the `docket-review` contract, four wrap none.*
2. The same file's *Agent layer* opening: **"Six skills get a wrapper"** becomes **seven**, adding `docket-review` to the enumeration. `tests/test_finalize_gate.sh` asserts this phrase exactly.
3. The same file's convention-injection sentence. It currently reads *"injected via `skills:` into every wrapper except **four**"* while naming five — an off-by-one left by change 0184's fourth build profile. Correct it and extend it: with the three review wrappers the exception set is **eight** (`docket-brainstorm-consultant`, the four `docket-build-*` workers, and the three `docket-review-*` rung wrappers). State the shared reason once: these wrappers perform no docket metadata operations.
4. `README.md`: both count sentences (the `harness-defaults.yml` completeness sentence and the delegatable-wrappers bullet) **thirteen → sixteen**.
5. `.docket.example.yml`: the mirror-block prose sentence **thirteen → sixteen**.

Check the convention's line/word budget after editing (`wc -l`, `wc -w`) — it sits at 361/365 lines and roughly 33 words of headroom. If either is exceeded, raise the row in `tests/test_skill_size_budgets.sh` per the rounding rule and add the explaining header paragraph in the same commit.

- [ ] **Step 7: Mirror the nine rows into `.docket.example.yml`**

Add the same three rows to each of the three commented blocks (`#   claude:`, `#   codex:`, `#   cursor:`), keeping each block's `#     ` comment prefix and its column alignment. `tests/test_docket_example_yml.sh` compares these values against the sidecar; a misaligned or mistyped value reddens the equality leg. Place them consistently with the existing per-block ordering.

- [ ] **Step 8: Update the count guards**

- `tests/test_sync_agents.sh`: `exactly 13 built-in wrappers` → 16, and `all 13 wrappers land in .claude/agents` → 16. Update both assert *labels* alongside their expressions so a failure message stays truthful.
- `tests/test_sync_agents_cursor.sh`: `cursor: full built-in set (13 files)` → 16.
- `tests/test_sync_agents_codex.sh`: the `(13 files)` equality → 16, and raise both `-ge 13` floors to `-ge 16`.
- `tests/test_cursor_dispatch_rule.sh`: raise the fragment population floor from `>= 13` to `>= 16`.
- `tests/test_finalize_gate.sh`: the convention count assert `thirteen` → `sixteen`, and its companion negative assert `no longer says twelve` → `no longer says thirteen`. Update the assert labels to match. Also update the exact-phrase assert for `six skills get a wrapper` → `seven skills get a wrapper`.

Also check `tests/test_sync_agents.sh`'s parser-subprocess budget (`FORK_COUNT < 400`) after generation — three more wrappers add roughly a quarter more per-agent parser work. If it trips, raise the budget with a comment naming change 0170 and the measured count; do **not** silently loosen it.

- [ ] **Step 9: Regenerate and verify**

Run:
```bash
"$DOCKET_BASH_PATH" ./sync-agents.sh --check
"$DOCKET_BASH_PATH" tests/test_harness_defaults.sh
"$DOCKET_BASH_PATH" tests/test_sync_agents.sh
"$DOCKET_BASH_PATH" tests/test_sync_agents_cursor.sh
"$DOCKET_BASH_PATH" tests/test_sync_agents_codex.sh
"$DOCKET_BASH_PATH" tests/test_cursor_dispatch_rule.sh
"$DOCKET_BASH_PATH" tests/test_docket_example_yml.sh
"$DOCKET_BASH_PATH" tests/test_docket_build.sh
"$DOCKET_BASH_PATH" tests/test_finalize_gate.sh
"$DOCKET_BASH_PATH" tests/test_docket_review.sh
```
Expected: all PASS. `test_docket_build.sh` is in this list specifically because of its banned bare-word scan over `agents/` and `cursor-rules/dispatch/` — if it reddens, a rung wrapper or fragment used `low`, `medium`, or `high` as a bare word.

- [ ] **Step 10: Mutation-test the roster guards**

Delete one sidecar row (say `codex: review-deep`) and confirm generation fails with the `block is incomplete` diagnostic; restore. Change one `review-deep` pin so it no longer equals `build-max` and confirm the cap-rung invariant assert reddens; restore. Add a phantom `` `docket-review-lean` subagent `` mention into a skill file and confirm the derived-dispatch-shape assert reddens; remove it.

- [ ] **Step 11: Commit**

```bash
git add agents/ cursor-rules/dispatch/ .docket.example.yml README.md \
        skills/docket-convention/SKILL.md tests/
git commit -m "feat(0170): three review rung wrappers, their pins, and the 13 -> 16 roster move"
```

---

### Task 3: The build-evidence record (producer)

**Files:**
- Modify: `skills/docket-build/SKILL.md`
- Modify: `tests/test_skill_size_budgets.sh` (raise the docket-build row)
- Test: `tests/test_docket_review.sh` (append an evidence-producer section)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the **build-evidence record** — the contract Tasks 4 and 5 read. Exact shape, fixed here and referenced by both consumers:

```
<!-- docket:build-evidence:start -->
command:  <the exact full-suite command that was run>
result:   green
head_sha: <the 40-char branch HEAD the run tested>
ran_at:   <UTC ISO-8601, e.g. 2026-08-01T19:40:00Z>
<!-- docket:build-evidence:end -->
```

`result:` is only ever `green` — a red suite never mints a record; it enters the repair path.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_review.sh`:

```bash
# --- the build-evidence chain: producer ----------------------------------------------------
# Per the learnings finding `specified-but-unreachable`: a contract with a producer and a consumer
# needs at least one assert anchored on the paragraph that PERFORMS the write, not only on the
# section that defines what the write means. The producer is docket-build's gate.
BUILD="$REPO/skills/docket-build/SKILL.md"
assert "producer: docket-build names the build-evidence record" \
  'grep -qF -- "build-evidence" "$BUILD"'
assert "producer: the evidence markers are defined where the gate emits them" \
  'grep -qF -- "docket:build-evidence:start" "$BUILD"'
for f in command result head_sha ran_at; do
  assert "producer: the evidence record carries the '$f' field" 'grep -qF -- "$f" "$BUILD"'
done
# The emission must be attached to the GREEN path of the gate, never to the section that merely
# describes the record: scope the search to the gate section's own text.
gate_sec="$(awk "/^## The build gate/{f=1;next} /^## /{f=0} f" "$BUILD")"
assert "producer: the gate section itself emits the evidence (not just a definition elsewhere)" \
  'grep -qF -- "build-evidence" <<<"$gate_sec"'
assert "producer: a red suite mints no evidence record" \
  'grep -qiE "red .{0,60}(never|no) .{0,30}evidence|evidence .{0,40}only .{0,20}green" "$BUILD"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `"$DOCKET_BASH_PATH" tests/test_docket_review.sh`
Expected: FAIL on all six producer asserts.

- [ ] **Step 3: Edit `skills/docket-build/SKILL.md`**

In `## The build gate`, extend the **Green** paragraph so the gate emits the record. Replace the existing single Green line with prose of this shape (keep it tight — the file has 3 lines of headroom before its budget raise in Step 4):

> **Green** → the build is done. Emit the **build-evidence** record — a marker-bounded block carrying `command` (the exact full-suite command run), `result: green`, `head_sha` (the branch HEAD the run tested, from `git rev-parse HEAD`), and `ran_at` (UTC ISO-8601):
>
> ```text
> <!-- docket:build-evidence:start -->
> command:  <full-suite command>
> result:   green
> head_sha: <40-char SHA>
> ran_at:   <UTC ISO-8601>
> <!-- docket:build-evidence:end -->
> ```
>
> The record certifies the branch so the review step need not re-run the suite; `docket-implement-next` Step 6 validates it and Step 7 writes it into the PR body. Only a green run mints a record — a red suite mints nothing and enters the repair path below.

Also add `the build-evidence record` to the `## Output` section's list of stable emitted lines, so the controller has a defined place to read it from.

- [ ] **Step 4: Raise the budget**

Measure `wc -l` and `wc -w` on `skills/docket-build/SKILL.md`. It starts at 247 lines against a 250 budget, so this edit will exceed it. Apply the rounding rule and update the row, then append a header paragraph in the file's existing style explaining that change 0170 added the build-evidence emission to the gate and the output list, and naming the measured actuals.

- [ ] **Step 5: Run the tests**

Run:
```bash
"$DOCKET_BASH_PATH" tests/test_docket_review.sh
"$DOCKET_BASH_PATH" tests/test_docket_build.sh
"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh
```
Expected: all PASS.

- [ ] **Step 6: Mutation-test**

Move the evidence block out of `## The build gate` into a new section at the end of the file, and confirm the gate-scoped assert (`the gate section itself emits the evidence`) reddens while the plain presence asserts stay green — that inversion is the whole reason the scoped assert exists. Restore.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-build/SKILL.md tests/test_docket_review.sh tests/test_skill_size_budgets.sh
git commit -m "feat(0170): the build gate mints a build-evidence record on green"
```

---

### Task 4: Controller — rung selection, evidence validation, triage, and the PR-body block

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Steps 6 and 7)
- Modify: `skills/docket-convention/SKILL.md` (Skill-layer role table row for `review`)
- Modify: `tests/test_skill_size_budgets.sh` (raise the implement-next row; convention row if needed)
- Test: `tests/test_docket_review.sh` (append a controller section)

**Interfaces:**
- Consumes: Task 2's three wrapper names; Task 3's evidence record shape.
- Produces: the PR-body evidence block that Task 5's finalize skip predicate parses — same markers, same four fields.

**The dispatch-shape constraint (read before writing prose).** `tests/test_dispatch_capability.sh` derives its dispatch-site population by grepping all of `skills/` for `` `<name>` ``-within-20-chars-of-`subagent`, and its site-coverage floor is an **exact** `-eq 5`. Naming the rung wrappers in that shape would add three uncovered sites and redden it. The established precedent is `docket-build`, which dispatches four profile workers and refers to them as *profile agents* — never in that shape — so its dispatch is covered by the single `resolved build skill` Tier C site. Do the same here: the existing `resolved review skill` Tier C paragraph stays the one tiered site, and the rung wrappers are referred to as **rung wrappers** or **the selected reviewer rung**. This is a deliberate, precedent-backed choice, not an evasion — Task 2 Step 1 already added the assert that proves the shape stays absent.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_review.sh`:

```bash
# --- the build-evidence chain: controller (consumer #1) ------------------------------------
IMPL="$REPO/skills/docket-implement-next/SKILL.md"
step6="$(awk "/^### Step 6 — Review/{f=1;next} /^### Step 6.5/{f=0} f" "$IMPL")"
assert "controller: Step 6 was located (non-vacuity anchor)" '[ -n "$step6" ]'
assert "controller: Step 6 validates the evidence before dispatching review" \
  'grep -qF -- "build-evidence" <<<"$step6"'
assert "controller: uncertified evidence re-runs the gate rather than reviewing blind" \
  'grep -qiE "re-run.{0,40}(gate|suite)" <<<"$step6"'
assert "controller: names all three reviewer rungs" \
  'for r in lean standard deep; do grep -qF -- "docket-review-$r" <<<"$step6" || exit 1; done'
assert "controller: rung selection is deterministic, from the build's highest profile" \
  'grep -qiE "highest .{0,40}profile" <<<"$step6"'
assert "controller: blockers route through the docket-build-task contract" \
  'grep -qF -- "docket-build-task" <<<"$step6"'
assert "controller: important/minor findings go to the PR body, never auto-fixed" \
  'grep -qE "important" <<<"$step6" && grep -qiE "PR body" <<<"$step6"'
assert "controller: no re-review round after fixes" \
  'grep -qiE "no re-review|never re-review" <<<"$step6"'
assert "controller: a red re-run halts" 'grep -qiE "red .{0,40}halt|halt" <<<"$step6"'

step7="$(awk "/^### Step 7 — PR/{f=1;next} /^### Terminal disposition/{f=0} f" "$IMPL")"
assert "controller: Step 7 was located (non-vacuity anchor)" '[ -n "$step7" ]'
assert "controller: Step 7 writes the evidence block into the PR body" \
  'grep -qF -- "docket:build-evidence:start" <<<"$step7"'

# The Tier C review site must survive untouched — this change adds rungs, not a new posture.
assert "controller: the review role keeps its Tier C dispatch paragraph" \
  'grep -qF -- "resolved review skill" "$IMPL"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `"$DOCKET_BASH_PATH" tests/test_docket_review.sh`
Expected: FAIL on the Step 6 evidence/rung/triage asserts and the Step 7 block assert.

- [ ] **Step 3: Rewrite Step 6**

Keep the existing paragraph's Tier C sentence, the learnings-index read, the `docket-adr` dispatch, and the auto-capture sentence intact — they are all guarded elsewhere. Insert, before the review dispatch:

> **Validate the build evidence.** Read the build-evidence record `$SKILL_BUILD`'s gate emitted: it must be present, `result: green`, and its `head_sha` equal to the branch HEAD. If it is missing, malformed, or stale — a build-contract violation — re-run the full suite once to mint fresh evidence rather than dispatching a review of an uncertified branch.
>
> **Select the reviewer rung** deterministically from the build record: take the **highest profile any task routed or escalated to** (an escalation counts as the tier escalated *to*), then map `economy` → `docket-review-lean`, `standard` → `docket-review-standard`, `premium` or `max` → `docket-review-deep`. One modifier: a whole-branch diff of more than **1500 changed lines** (`git diff --shortstat origin/<integration_branch>...HEAD`) bumps the rung one step, capped at deep — the single selection signal independent of the build's own self-assessment. Log the chosen rung and its reason as one line. Selection is a rule over the build record, never model judgment.

Then, in the dispatch sentence, direct the reviewer by rung wrapper (**not** in the `` `name` ``-near-`subagent` shape):

> …invoked **DIRECTED to:** review the whole branch against its base and return its findings, then stop. Dispatch the selected rung wrapper by name, foreground, and pass it the branch and base ref, the change's title and scope sections, the relevant learnings hooks, and the current evidence record.

Then add the triage paragraph:

> **Triage the returned findings.** **Blocker** → one synthetic fix task covering all blockers, run through the `docket-build-task` contract on the ladder `standard → premium → halt`; if its commits land, re-run the full suite once and refresh the evidence. A red re-run **halts** (abort-and-report, the change stays `in-progress` with the reason recorded) — no second repair chain. The reviewer's `unverified-build-state` blocker is the one exception: it is resolved by the controller re-running the suite, never by a worker task. **Important / minor** → recorded in the PR body for the human's merge-time judgment; never auto-fixed. **Distinct follow-up work** → the existing auto-capture path, unchanged. There is **no re-review** after fixes: remediation is verified by the worker's self-review plus the green suite re-run, and both findings and fixes are visible in the PR body.

- [ ] **Step 4: Extend Step 7**

Add one paragraph to Step 7, before the metadata write:

> **Build-evidence block (change 0170).** Write the current evidence record into the PR body, marker-bounded (`<!-- docket:build-evidence:start -->` / `<!-- docket:build-evidence:end -->`), alongside the review outcome — the rung that reviewed, blockers fixed, and any important/minor findings left for merge-time judgment. The PR body is the block's durable home: `docket-finalize-change` reads it to decide whether its post-rebase suite run can be skipped. Validate marker order and balance before rewriting an existing block.

- [ ] **Step 5: Update the convention's Skill-layer role table**

In `skills/docket-convention/SKILL.md`, the `review` row's `auto` / fallback column currently reads *a whole-branch review before the PR opens*. Leave the shipped default binding (`superpowers:requesting-code-review`) unchanged — `tests/test_docket_example_yml.sh` pins it. Extend the fallback cell to name the evidence obligation, so the `auto` path does not silently drop it: *a whole-branch review before the PR opens, over a branch whose build evidence is green*.

- [ ] **Step 6: Raise the budgets**

Measure both files. `skills/docket-implement-next/SKILL.md` starts at 135/147 lines with ~55 words of headroom and will exceed the word budget. Apply the rounding rule, update the rows, and append header paragraphs naming change 0170 and the measured actuals. Check the convention row too.

- [ ] **Step 7: Run the tests**

Run:
```bash
"$DOCKET_BASH_PATH" tests/test_docket_review.sh
"$DOCKET_BASH_PATH" tests/test_dispatch_capability.sh
"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh
"$DOCKET_BASH_PATH" tests/test_skill_handoff_precedence.sh
"$DOCKET_BASH_PATH" tests/test_composition_wiring.sh
"$DOCKET_BASH_PATH" tests/test_loop_continuation.sh
```
Expected: all PASS. If `test_dispatch_capability.sh` reddens on its `-eq 5` floor or its reverse-correspondence loop, the Step 3 prose used the forbidden `` `name` ``-near-`subagent` shape — reword to *rung wrapper*, do not extend `PENDING_TIER` (the file forbids that explicitly).

- [ ] **Step 8: Mutation-test**

Delete the `head_sha` validation sentence from Step 6 and confirm the evidence assert reddens. Rename the Step 6 heading and confirm the non-vacuity anchor (`Step 6 was located`) reddens rather than the whole section silently passing with an empty haystack — that anchor exists precisely because an awk range over a renamed heading yields an empty string that every `grep -q` would then fail on confusingly. Restore.

- [ ] **Step 9: Commit**

```bash
git add skills/docket-implement-next/SKILL.md skills/docket-convention/SKILL.md \
        tests/test_docket_review.sh tests/test_skill_size_budgets.sh
git commit -m "feat(0170): Step 6 rung selection, evidence validation, and finding triage"
```

---

### Task 5: Finalize's conditional post-rebase suite skip

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md`
- Modify: `tests/test_skill_size_budgets.sh` (raise the finalize row)
- Test: `tests/test_docket_review.sh` (append a finalize section)

**Interfaces:**
- Consumes: Task 4's PR-body evidence block (same markers, same four fields).
- Produces: nothing downstream.

**Constraints specific to this file:**
- The `<!-- configured-bash-finalize:start/end -->` fenced fragment is **executed verbatim** by `tests/test_configured_bash_finalize.sh`. The skip condition goes in the surrounding prose, **never inside that fence**.
- `tests/test_finalize_gate.sh` asserts local validation precedes the push by comparing the line-order of two greps. Do not reorder those two paragraphs.
- No `opus`/`sonnet`/`haiku`/`fable`/`xhigh` literals anywhere in this file.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_review.sh`:

```bash
# --- the build-evidence chain: finalize (consumer #2) --------------------------------------
FIN="$REPO/skills/docket-finalize-change/SKILL.md"
assert "finalize: reads the PR body's build-evidence block" \
  'grep -qF -- "build-evidence" "$FIN"'
# All three skip conditions must be stated; any one missing turns "fails toward running" into
# "fails toward merging an untested branch".
assert "finalize: skip requires a no-op rebase" \
  'grep -qiE "no-op rebase|rebase was a no-op" "$FIN"'
assert "finalize: skip requires result green" 'grep -qF -- "result: green" "$FIN"'
assert "finalize: skip requires the head_sha to match the branch HEAD" \
  'grep -qF -- "head_sha" "$FIN"'
# The posture is the safety property, not a nicety.
assert "finalize: any doubt runs the suite (fails toward running)" \
  'grep -qiE "fails? toward running|any doubt .{0,40}runs" "$FIN"'
assert "finalize: a skip is logged so the decision is auditable" \
  'grep -qiE "log.{0,60}skip|skip .{0,40}logged" "$FIN"'
assert "finalize: only the local gate path is affected" \
  'grep -qE "\`ci\`.{0,60}untouched|untouched.{0,60}\`ci\`" "$FIN"'
# The skip must NOT live inside the executable fragment, which the suite runs verbatim.
frag="$(awk "/configured-bash-finalize:start/{f=1;next} /configured-bash-finalize:end/{f=0} f" "$FIN")"
assert "finalize: the executable bash fragment is untouched by the skip logic" \
  '! grep -qiE "evidence|skip|head_sha" <<<"$frag"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `"$DOCKET_BASH_PATH" tests/test_docket_review.sh`
Expected: FAIL on the finalize asserts (the fragment assert passes vacuously-correctly, since the fragment is untouched — that one is a regression guard, not a new-behavior guard).

- [ ] **Step 3: Add the skip predicate**

In `## The rebase-retest merge gate`, insert a new numbered item **between** the current step 3 (determine the suite) and step 4 (validate per `gate`), renumbering the rest. It must sit after the rebase and before validation:

> 4. **Conditional skip (`local`/`both` only).** Skip the post-rebase suite run **only when all three hold**: the rebase was a **no-op** (the branch was already based on the current `origin/<integration_branch>` tip, and HEAD is unchanged by the rebase); the PR body carries a parseable `docket:build-evidence` block with `result: green`; and that block's `head_sha` equals the branch HEAD being merged. Anything else — a missing or malformed block, a SHA mismatch, an actual rebase — runs the suite exactly as before. **The posture fails toward running:** any doubt costs one suite run, never a broken integration branch. Log a skip loudly as one line naming the matched SHA, so the decision is auditable. `ci`, `both`'s CI leg, and `off` are untouched.

Note that `both` skips only its *local* leg — its CI leg is unaffected — so the sentence must scope the skip to the local run rather than to the gate as a whole.

- [ ] **Step 4: Raise the budget**

The file starts at 189/193 lines. Measure, apply the rounding rule, update the row, and append the explaining header paragraph.

- [ ] **Step 5: Run the tests**

Run:
```bash
"$DOCKET_BASH_PATH" tests/test_docket_review.sh
"$DOCKET_BASH_PATH" tests/test_finalize_gate.sh
"$DOCKET_BASH_PATH" tests/test_configured_bash_finalize.sh
"$DOCKET_BASH_PATH" tests/test_config_read_channel.sh
"$DOCKET_BASH_PATH" tests/test_finalize_disposition.sh
"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh
```
Expected: all PASS.

- [ ] **Step 6: Mutation-test**

Delete the `head_sha` condition from the skip predicate and confirm its assert reddens. Move the word `evidence` inside the `configured-bash-finalize` fence and confirm the fragment-purity assert reddens **and** `test_configured_bash_finalize.sh` still passes — proving the two guards catch different things. Restore.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md tests/test_docket_review.sh \
        tests/test_skill_size_budgets.sh
git commit -m "feat(0170): finalize skips its post-rebase suite run on certified, unmoved branches"
```

---

### Task 6: Documentation and the dogfood binding

**Files:**
- Modify: `README.md`
- Modify: `.docket.yml`
- Test: `tests/test_docket_review.sh` (append a docs section)

**Interfaces:**
- Consumes: everything above.
- Produces: nothing downstream.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_review.sh`:

```bash
# --- documentation + the dogfood binding ---------------------------------------------------
RM="$REPO/README.md"
assert "README documents the docket-review role" 'grep -qF -- "docket-review" "$RM"'
assert "README explains why the suite lives in the build gate, not the reviewer" \
  'grep -qiE "build gate" "$RM" && grep -qF -- "build-evidence" "$RM"'
assert "README states the suite-run count the change delivers" \
  'grep -qiE "once|one run" "$RM"'
DY="$REPO/.docket.yml"
assert "this repo dogfoods docket-review via .docket.yml" \
  'awk "/^skills:/{f=1;next} /^[a-z_]+:/{f=0} f" "$DY" | grep -qE "^ +review: +docket-review$"'
# The SHIPPED default must NOT move — the example config is the cross-harness default surface.
assert "the shipped default review binding is unchanged in the example config" \
  'grep -qE "^ +review: +superpowers:requesting-code-review$" "$REPO/.docket.example.yml"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `"$DOCKET_BASH_PATH" tests/test_docket_review.sh`
Expected: FAIL on the README asserts and the `.docket.yml` binding assert.

- [ ] **Step 3: Write the README section**

Add a `### docket-review — the bounded whole-branch reviewer` section immediately after the existing `### docket-build` section, mirroring its structure and length. Cover:

- What it is: one read-only reviewer behind three pinned rungs, selected deterministically as *one above the build* from the highest profile the build routed or escalated to, with a diff-size bump.
- The finding tiers and where each goes (blockers fixed pre-PR through the build-task contract; important/minor to the PR body; follow-ups to auto-capture).
- **Why the suite lives in the build gate, not the reviewer** — the design's central placement decision, as four short points: the suite answers the *build's* question ("does what I assembled work together?") while review asks "is this good?"; the repair machinery already lives on the build side, so a suite inside a reviewer forbidden to fix would have to hand failures back out and re-enter build machinery, recreating the build→review→build loop this design exists to kill; gate-first ordering is cheaper on failure (a red suite found after an expensive whole-branch read wastes the review); and the evidence chain then follows naturally, because the thing that last mutated the branch is what certifies it. Land the mental model: the suite is the **boundary between build and review**, owned by the side that can fix a failure — like CI status checks on a PR, with the reviewer as the human-style reviewer who reads the diff and trusts the green check.
- The evidence chain and the resulting arithmetic: one full-suite run on the clean path, two worst-case, never three.
- The binding posture: the shipped cross-harness default stays `superpowers:requesting-code-review`; this repository opts in via its committed `.docket.yml`.

Also update the `skills:` role table row for `review` if the surrounding rows carry a description that this change makes stale.

- [ ] **Step 4: Add the dogfood binding**

In `.docket.yml`, extend the existing `skills:` block:

```yaml
skills:
  build: docket-build
  review: docket-review
```

Extend the comment above that block so it names both roles rather than only the build one, and keeps stating that the shipped cross-harness defaults are unchanged.

- [ ] **Step 5: Run the tests**

Run:
```bash
"$DOCKET_BASH_PATH" tests/test_docket_review.sh
"$DOCKET_BASH_PATH" tests/test_readme_skill_catalog.sh
"$DOCKET_BASH_PATH" tests/test_readme_finalize_docs.sh
"$DOCKET_BASH_PATH" tests/test_docket_config.sh
"$DOCKET_BASH_PATH" tests/test_docket_example_yml.sh
"$DOCKET_BASH_PATH" tests/test_finalize_gate.sh
```
Expected: all PASS.

- [ ] **Step 6: Verify the resolver actually reports the binding**

The `.docket.yml` edit is only real if the resolver emits it. Run:

```bash
"${DOCKET_SCRIPTS_DIR:?}"/docket.sh env | grep SKILL_REVIEW
```
Expected: `SKILL_REVIEW=docket-review`. If it still reports the superpowers default, the YAML edit landed in the wrong block or the file is not on the branch being read — resolve before committing.

- [ ] **Step 7: Commit**

```bash
git add README.md .docket.yml tests/test_docket_review.sh
git commit -m "docs(0170): document the review role, the suite-once rationale, and dogfood it"
```

---

### Task 7: Whole-suite gate and portability sweep

**Files:**
- Modify: any file the sweep reveals as broken.
- Test: the whole suite.

- [ ] **Step 1: Run the whole suite**

Run, as one foreground command:
```bash
cd /Users/homer/dev/docket/.worktrees/lean-whole-branch-review-skill && \
  for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test" || echo "SUITE-FAIL: $test"; done
```
Expected: no `SUITE-FAIL` lines. Per AGENTS.md, run the whole suite here — never only the tests this plan enumerated. Tests not named anywhere in this plan can still redden: `test_convention_extraction.sh`, `test_composition_wiring.sh`, `test_script_contracts_coverage.sh`, `test_comment_anchor_style.sh`, and `test_typed_changes_docs.sh` all read the files this change touches.

- [ ] **Step 2: Re-check every new regex under BSD grep**

The interactive shell's `grep` is ugrep and accepts constructs `/usr/bin/grep` rejects, so a green local run is not proof of portability. For every regex added to `tests/test_docket_review.sh`, re-run the file with BSD grep first on `PATH`:

```bash
PATH=/usr/bin:/bin "$DOCKET_BASH_PATH" tests/test_docket_review.sh
```
Expected: PASS, with no `grep: ... repetition` or `illegal option` diagnostics on stderr. Fix any bounded-repetition counts above 255 and any GNU-only escape.

- [ ] **Step 3: Verify wrapper generation end to end**

Run:
```bash
"$DOCKET_BASH_PATH" ./sync-agents.sh --check
```
Expected: clean, reporting the full sixteen-wrapper set with no drift and no `WARN no dispatch fragment` lines for the three new agents.

- [ ] **Step 4: Fix and re-run**

Fix anything red, then re-run Steps 1-3 until clean. Do not weaken an assert to make it pass — if a guard is genuinely wrong, correct it and say why in the commit message.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "test(0170): whole-suite gate — portability sweep and generation verification"
```

---

## Self-Review

**1. Spec coverage.** Component 1 (the `docket-review` skill) → Task 1. Component 2 (rungs + pins, the "one above the build" rule) → Tasks 2 and 4. Component 3 (the build-evidence block) → Task 3 produces it, Task 4 validates and publishes it, Task 5 consumes it. Component 4 (controller changes) → Task 4. Component 5 (finalize's conditional skip) → Task 5. The spec's *Finding schema* → Task 1 Step 3 item 7. The spec's ripple list → Tasks 2 and 6, plus the reconciled hard-gate list distributed across every task's test step. The spec's "record the suite-placement rationale in README.md" → Task 6 Step 3. The spec's "ADR at build time" is **not** a plan task — it is `docket-implement-next` Step 6's own `docket-adr` dispatch, performed by the controller after the build, not by a build worker.

**2. Placeholder scan.** The only intentional placeholder is `<PASTE the exact description: string from skills/docket-review/SKILL.md>` in Task 2 Step 3, which is a deliberate cross-task reference to a string Task 1 fixes and Task 2's own byte-equality assert verifies. Every budget number is marked "measure first, then apply the stated rounding rule" rather than guessed, because the actuals depend on prose written in the same step.

**3. Type consistency.** The evidence record's four field names (`command`, `result`, `head_sha`, `ran_at`) and its marker pair (`docket:build-evidence:start` / `:end`) are spelled identically in Task 3 (producer), Task 4 (validator and PR-body writer), and Task 5 (finalize consumer), and each task's asserts grep those exact strings. The three wrapper names (`docket-review-lean`, `-standard`, `-deep`) and the three sidecar keys (`review-lean`, `review-standard`, `review-deep`) are consistent between Task 2's files and Task 4's selection mapping — note the deliberate asymmetry: wrapper *files* carry the `docket-` prefix, sidecar *keys* do not, because `sync-agents.sh` derives the short name by stripping that prefix. The three severity words (`blocker`, `important`, `minor`) are consistent between Task 1's schema and Task 4's triage.
