<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0281 — Auto-groom's critic verdict return channel fails under background dispatch](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-11-0281-auto-groom-s-critic-verdict-return-channel-fails-under-backg.md)**
<!-- docket:backlink:end -->

# Auto-groom's critic verdict return channel — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Settle the `docket-auto-groom-critic` → `docket-auto-groom` verdict return-channel contract as foreground-only — the verdict travels solely as the critic's final report — and give the groom a bounded, terminating posture for when no verdict arrives.

**Architecture:** Three prose contracts are edited and one new prose-sentinel guard binds all three. The critic's agent source gains a *delivery* clause that binds at the moment the critic stops. The groom's Step 3 gains the *receiving* half plus a two-step bounded no-verdict posture that terminates in the Tier B abstain the skill already owns. The convention's *Composition* paragraph is re-ordered so the critic dispatch sits in the in-context-return family (with `docket-rebase-resolver` / `docket-integration-repair`) rather than inside the git-state-contract clause. No scripts change.

**Tech Stack:** Bash 3.2-compatible POSIX-ish shell test scripts (`tests/test_*.sh`, run by `scripts/run-tests.sh`); markdown skill/agent contracts.

## Global Constraints

Copied verbatim in effect from the spec and the repo's always-on rules — every task's requirements implicitly include these:

- **Harness-neutral normative prose.** Normative clauses phrase by capability/shape only; never name a product-specific tool, dispatch syntax, or agent-listing surface in a normative sentence. A *failure diagnostic* MAY name what was attempted.
- **A guard is code: mutation-test it.** Every assert added must be proven to redden when the thing it guards is stripped. **Deletion and inversion are different probes** — a comparison operator needs both probed.
- **Key a guard on syntactic shape, never an enumerated list of spellings.**
- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`) under `set -o pipefail` — capture into a variable first, then `grep <<<"$var"`.
- **`grep` for a pattern that leads with `--`** must declare it (`grep -qF --`).
- **Cross-references anchor on a symbol name or a verbatim-quoted clause, never a line number.**
- **Scope fence:** change 0260 is queued immediately after this one and also edits `skills/docket-convention/SKILL.md`. This diff touches **only** the *Composition* paragraph of that file. Do not opportunistically fix anything else in it.
- **Point-in-time records are never edited** — archived changes, results files, specs, plans, Accepted ADRs keep whatever was true when written.
- **Suite command** is whatever `finalize.test_command` resolves to: `scripts/run-tests.sh`. Run the **whole** suite at the build gate, never only the enumerated files. A trailing `OVER BUDGET:` line for a file this branch touched is a finding to act on.
- **Pre-existing and NOT this branch's problem:** `tests/test_sync_agents_runners.sh` runs ~190s against a 60s ceiling. It is tracked as change 0280. Leave it alone; do not raise its row.

## File Structure

| File | Disposition | Responsibility |
|---|---|---|
| `agents/docket-auto-groom-critic.md` | Modify | The critic's own contract. Gains the delivery clause — the only file the critic actually loads besides `docket-convention`, which is why the clause must live here and not in the groom's body. |
| `skills/docket-auto-groom/SKILL.md` | Modify (Step 3 only) | The groom's contract. Gains the receiving half and the bounded no-verdict posture. |
| `skills/docket-convention/SKILL.md` | Modify (*Composition* paragraph only) | The shared contract. Re-orders so the git-state-contract clause no longer has the critic as an antecedent. |
| `tests/test_critic_return_channel.sh` | Create | The single sentinel binding all three edits above. |
| `tests/runtime-budgets.tsv` | Modify (one row) | Every `tests/test_*.sh` must carry a ceiling row or `test_runtime_budgets.sh` fails. |
| `tests/test_runtime_budgets.sh` | Modify (one constant) | `EXPECTED_TOTAL` pins the SUM of every ceiling; a new file's row moves it. |

**Why one guard file rather than a fold-in:** the property spans three files in two populations (`agents/` and `skills/`) and would have to be split across `test_auto_groom.sh` and `test_composition_wiring.sh`, breaking the mutation story into halves that can pass independently. Settled at reconcile; recorded in the spec.

**Task ordering rationale:** Task 1 creates the guard file and carries the whole budget-table cost (setup folded into the task whose deliverable needs it), so Tasks 2 and 3 only append assert blocks. Each task is red→green on its own file pair and a reviewer can reject any one while approving its neighbors.

---

### Task 1: Critic delivery clause + guard scaffold + budget row

**Files:**
- Create: `tests/test_critic_return_channel.sh`
- Modify: `agents/docket-auto-groom-critic.md`
- Modify: `tests/runtime-budgets.tsv` (insert one row)
- Modify: `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL` constant)
- Test: `tests/test_critic_return_channel.sh` (the guard is the test)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the guard file with its shared preamble — `REPO`, `fail`, `assert(){...}`, `flat(){...}`, and the three path variables `CRITIC`, `AUTOGROOM`, `CONV` — plus the trailing `if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi; exit "$fail"` epilogue. Tasks 2 and 3 insert their assert blocks **above that epilogue** and reuse those exact names.

- [ ] **Step 1: Write the failing guard**

Create `tests/test_critic_return_channel.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# tests/test_critic_return_channel.sh — change 0281. The docket-auto-groom-critic verdict travels
# on exactly ONE channel: the critic's final report, read by the groom as the dispatch's return
# while it actively blocks. This guard binds the three prose contracts that make that true:
#
#   (a) agents/docket-auto-groom-critic.md — the DELIVERY half: the verdict IS the final report,
#       and the critic never addresses its dispatcher by name or via an agent-listing surface.
#   (b) skills/docket-auto-groom/SKILL.md Step 3 — the RECEIVING half, plus the bounded
#       no-verdict posture that terminates in the Tier B abstain.
#   (c) skills/docket-convention/SKILL.md *Composition* — the critic dispatch sits in the
#       in-context-return family, NOT inside the git-state-contract clause.
#
# Deliberate limits, named so a later reader does not over-trust this file:
#   * PHRASE-SCOPED. A contract reworded past these anchors escapes it. The anchors are verbatim
#     quoted clauses (ADR-0054), so drift is at least mechanically visible.
#   * The (c) assert is an ORDERING fact over one paragraph, not a parse of English antecedents.
# Sentinels are sampling, not parsing (learnings: foundational-test-discipline) — pair with the
# whole-branch review.
# Run: bash tests/test_critic_return_channel.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Whitespace-collapse: these contracts are hard-wrapped prose, so every proximity match below runs
# against a flattened copy or it would fail on a line break that means nothing.
flat(){ tr -s '[:space:]' ' ' <<<"$1"; }

CRITIC="$REPO/agents/docket-auto-groom-critic.md"
AUTOGROOM="$REPO/skills/docket-auto-groom/SKILL.md"
CONV="$REPO/skills/docket-convention/SKILL.md"

# --- (a) the critic's delivery clause -----------------------------------------------------------
# Non-vacuity anchor: the file must exist and be non-empty, or every assert below passes for
# reasons unrelated to the property.
assert "critic source exists and is non-empty" '[ -s "$CRITIC" ]'
# Non-vacuity anchor: a live PRESENCE assert through the same read, so a rename reddens here
# rather than silently greening the rest.
assert "critic source is the adversarial critic contract" 'grep -qi "adversarial critic" "$CRITIC"'

critic_flat="$(flat "$(cat "$CRITIC")")"

# The verdict is bound to the final report. Bounded gap ([^.]{0,80}) keeps the match inside one
# sentence, so a stray "verdict" and a stray "final report" in different clauses cannot satisfy it.
assert "critic: the verdict IS the critic's final report" \
  'grep -qE "verdict[^.]{0,80}final report|final report[^.]{0,80}verdict" <<<"$critic_flat"'

# The never-address-your-dispatcher clause, pinned on its two load-bearing halves: the prohibition
# itself, and the REASON (which is what stops a critic from inventing a workaround channel).
assert "critic: never message, address, or resolve the dispatcher" \
  'grep -qE "[Nn]ever[^.]{0,120}(message|address|resolve)[^.]{0,120}dispatcher" <<<"$critic_flat"'
assert "critic: states why no such address resolves" \
  'grep -qF -- "not registered under its skill name" <<<"$critic_flat"'
# The belief-changes-nothing clause: a critic that concludes the channel is broken must still write
# the verdict as its final report. Without this, a critic reasons its way into silence.
assert "critic: a believed-unavailable channel changes nothing about what it does" \
  'grep -qE "believe[^.]{0,160}changes nothing" <<<"$critic_flat"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_critic_return_channel.sh`

Expected: FAIL. The two non-vacuity anchors print `ok`; the four property asserts print `NOT OK` (`critic: the verdict IS the critic's final report`, `critic: never message, address, or resolve the dispatcher`, `critic: states why no such address resolves`, `critic: a believed-unavailable channel changes nothing about what it does`). Exit status 1.

- [ ] **Step 3: Add the delivery clause to the critic source**

In `agents/docket-auto-groom-critic.md`, append these two paragraphs **at the end of the file**, after the existing paragraph that begins `You run autonomously with no human to pause and ask`. The clause must bind at the point the critic is reading when it stops (learnings: `prohibition-needs-a-return-value` — the mapping lives in the clause, not a distant section), which is why it goes last rather than into the opening paragraph.

```markdown
**Your verdict is your final report** — the text you end your run with. That return is the only
channel your verdict travels on, and your dispatcher is blocking on it. Write the verdict there and
stop.

Never attempt to message, address, or resolve your dispatcher by name, and never try to look it up
through an agent-listing surface: a dispatched groom is not registered under its skill name, so no
such address resolves and a verdict sent to one is stranded. If you come to believe the return
channel itself is unavailable, that belief changes nothing about what you do — write the verdict as
your final report and stop.
```

- [ ] **Step 4: Run the guard to verify it passes**

Run: `bash tests/test_critic_return_channel.sh`

Expected: PASS, six `ok` lines, exit 0.

- [ ] **Step 5: Mutation-test the four new asserts (deletion probe)**

For each assert, delete the clause it binds, confirm the guard reddens, then restore. Do this with `git stash`/`git checkout --` or an editor undo — do **not** leave a mutation committed.

```bash
# Probe 1: strip the "final report" binding
perl -0pi -e 's/\*\*Your verdict is your final report\*\*/Your verdict/' agents/docket-auto-groom-critic.md
bash tests/test_critic_return_channel.sh   # expect: NOT OK - critic: the verdict IS ... ; FAIL
git checkout -- agents/docket-auto-groom-critic.md

# Probe 2: strip the never-address prohibition sentence
perl -0pi -e 's/Never attempt to message[^\n]*(\n(?!\n)[^\n]*)*//' agents/docket-auto-groom-critic.md
bash tests/test_critic_return_channel.sh   # expect: NOT OK on the never-address, the reason, and the belief asserts; FAIL
git checkout -- agents/docket-auto-groom-critic.md
```

Record in the commit message which probes were run and that each reddened. If any probe leaves the guard **green**, that assert is decoration — fix the matcher before proceeding.

- [ ] **Step 6: Add the runtime-budget row and re-pin the total**

In `tests/runtime-budgets.tsv`, insert the new row in the table's existing `LC_ALL=C` sort order — between `tests/test_convention_extraction.sh` and `tests/test_cursor_contract_docs.sh` (`c-o` < `c-r` < `c-u`). The separator is a literal **TAB**, not spaces:

```
tests/test_critic_return_channel.sh	10	parallel
```

In `tests/test_runtime_budgets.sh`, change the pinned sum on the `EXPECTED_TOTAL=` line from `1670` to `1680`, keeping the trailing comment. This is the table header's sanctioned case — "a new test file brings its own row."

- [ ] **Step 7: Verify the budget guard and measure the real cost**

```bash
bash tests/test_runtime_budgets.sh
scripts/run-tests.sh -j 1 --timings tests/test_critic_return_channel.sh
```

Expected: `test_runtime_budgets.sh` PASSes. The timed run reports well under 10s (this guard only reads three files). If the measured time exceeds 10s, raise the row to the measured number rounded up to the next multiple of 5 plus 5s and re-pin `EXPECTED_TOTAL` to match — never leave an `OVER BUDGET:` line for a file this branch introduced.

- [ ] **Step 8: Commit**

```bash
git add tests/test_critic_return_channel.sh agents/docket-auto-groom-critic.md tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "fix(0281): the critic's verdict is its final report, never a name-addressed message"
```

---

### Task 2: Groom Step 3 — the receiving half and the bounded no-verdict posture

**Files:**
- Modify: `skills/docket-auto-groom/SKILL.md` (the `### Step 3 — Critic pass` section only)
- Modify: `tests/test_critic_return_channel.sh` (append the `(b)` assert block)
- Test: `tests/test_critic_return_channel.sh`

**Interfaces:**
- Consumes: `REPO`, `fail`, `assert`, `flat`, `AUTOGROOM` from Task 1's preamble.
- Produces: nothing later tasks consume.

- [ ] **Step 1: Write the failing asserts**

In `tests/test_critic_return_channel.sh`, insert this block **immediately above** the closing `if [ "$fail" = 0 ]; then echo "PASS"; ...` epilogue:

```bash
# --- (b) the groom's receiving half + bounded no-verdict posture ---------------------------------
assert "auto-groom skill exists and is non-empty" '[ -s "$AUTOGROOM" ]'
assert "auto-groom has a Step 3 critic pass" 'grep -qF -- "### Step 3 — Critic pass" "$AUTOGROOM"'

# Scope every match below to Step 3 alone: awk from the Step 3 heading to the next "### " heading,
# then whitespace-collapse. A match anywhere else in the file must NOT satisfy these.
step3="$(awk '/^### Step 3 — Critic pass/{f=1;next} f&&/^### /{exit} f{print}' "$AUTOGROOM")"
assert "Step 3 section is non-empty (slice anchor holds)" '[ -n "$step3" ]'
step3_flat="$(flat "$step3")"

# The receiving half: the verdict comes from the critic's RETURN, and out-of-band delivery is
# refused. Two separate asserts — the positive channel and the prohibition are separately
# deletable, so binding them in one match would let either deletion pass.
assert "Step 3: the verdict is read from the critic's return" \
  'grep -qE "verdict is read from[^.]{0,60}return" <<<"$step3_flat"'
assert "Step 3: the groom never waits for out-of-band delivery" \
  'grep -qE "never waits for[^.]{0,120}out-of-band" <<<"$step3_flat"'

# The bounded posture, pinned on each of its three bounds. All three are load-bearing: drop the
# collect and a transient plumbing fault junks a sound draft; drop the re-dispatch likewise; drop
# the third-dispatch ban and termination is no longer provable.
assert "Step 3: exactly one collect attempt" \
  'grep -qE "one collect attempt" <<<"$step3_flat"'
assert "Step 3: exactly one fresh foreground re-dispatch" \
  'grep -qE "one fresh foreground re-dispatch" <<<"$step3_flat"'
assert "Step 3: never a third dispatch" \
  'grep -qiE "[Nn]ever a third dispatch" <<<"$step3_flat"'
assert "Step 3: never an indefinite wait" \
  'grep -qiE "never an indefinite wait" <<<"$step3_flat"'

# The mapping the whole posture exists to supply (learnings: prohibition-needs-a-return-value):
# no verdict maps to the Tier B abstain, which is a value the exit vocabulary already has.
# Bounded gap keeps antecedent and consequent inside one sentence.
assert "Step 3: no verdict maps to the Tier B abstain" \
  'grep -qE "[Ss]till no verdict[^.]{0,200}Tier B" <<<"$step3_flat"'
assert "Step 3: the Tier B outcome is the abstain exit" \
  'grep -qE "Tier B[^.]{0,60}abstain" <<<"$step3_flat"'

# Regression anchor: the pre-existing never-yield qualifier on the SECOND critic round must
# survive this edit (it is what tests/test_composition_wiring.sh binds).
assert "Step 3: the critic re-check is still dispatched foreground" \
  'grep -qi "re-check is dispatched foreground" <<<"$step3_flat"'
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_critic_return_channel.sh`

Expected: FAIL. The four anchor/regression asserts (`auto-groom skill exists…`, `auto-groom has a Step 3 critic pass`, `Step 3 section is non-empty…`, `Step 3: the critic re-check is still dispatched foreground`) print `ok`; the eight property asserts print `NOT OK`.

- [ ] **Step 3: Add the receiving half and the no-verdict posture**

In `skills/docket-auto-groom/SKILL.md`, inside `### Step 3 — Critic pass`, append these two paragraphs **after** the existing single body paragraph (the one ending `— an author cannot be their own adversarial gate.`) and **before** the `### Step 4 — Exit (one of three)` heading. Leave the existing paragraph byte-identical.

```markdown
**Receiving the verdict.** The verdict is read from the critic's **return** — its final report, which
the groom is actively blocking on. The groom never waits for a message, a notification, or any other
out-of-band delivery: nothing is registered to deliver one to it, so a groom that waits for one waits
forever.

**No-verdict posture (bounded — two steps, then out).** If the dispatch comes back with no legible
verdict — a malformed return, pre-yield prose, or a backgrounded child's bare completion — make
**one collect attempt** (read the child's completed final report, where the harness surfaces it),
and failing that **one fresh foreground re-dispatch** of the critic over the same draft. Still no
verdict ⇒ treat it as a failed dispatch attempt under the convention's *Dispatch-capability
resolution*: **Tier B**, so the groom **abstains** for this stub (→ Step 4's **Abstain** exit),
recording the return-channel diagnostic in the `## Auto-groom blocked` section. Never a third
dispatch; never an indefinite wait. Re-dispatching a critic is safe where re-dispatching a build
worker is not — the critic is read-only over prose, holds no worktree, and writes no git state, so
the closed-doors analysis in `yielded-worker-return-closes-every-door` does not bind here.
```

- [ ] **Step 4: Run the guard to verify it passes**

Run: `bash tests/test_critic_return_channel.sh`

Expected: PASS, exit 0.

- [ ] **Step 5: Run the neighbouring guards for regressions**

Run: `bash tests/test_auto_groom.sh && bash tests/test_composition_wiring.sh && bash tests/test_skill_size_budgets.sh`

Expected: all PASS. `test_skill_size_budgets.sh` matters here — this task adds prose to a skill body that has a size ceiling. If it reddens, that is a real finding: report it rather than silently raising the ceiling.

- [ ] **Step 6: Mutation-test the new asserts (deletion probe)**

```bash
# Probe 1: strip the receiving-half paragraph
perl -0pi -e 's/\*\*Receiving the verdict\.\*\*.*?\n\n//s' skills/docket-auto-groom/SKILL.md
bash tests/test_critic_return_channel.sh   # expect: NOT OK on both receiving-half asserts; FAIL
git checkout -- skills/docket-auto-groom/SKILL.md

# Probe 2: strip the whole no-verdict posture paragraph
perl -0pi -e 's/\*\*No-verdict posture.*?\n\n//s' skills/docket-auto-groom/SKILL.md
bash tests/test_critic_return_channel.sh   # expect: NOT OK on all six posture/mapping asserts; FAIL
git checkout -- skills/docket-auto-groom/SKILL.md

# Probe 3 (SLICE probe): the Step 3 slice must really be a slice. Move the posture paragraph out of
# Step 3 into Step 4 and confirm the guard still reddens — otherwise the asserts are matching the
# whole file and the awk anchor is decorative.
```

For probe 3, cut the `**No-verdict posture…**` paragraph and paste it under `### Step 4 — Exit (one of three)`, run the guard, confirm FAIL, then restore with `git checkout --`.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-auto-groom/SKILL.md tests/test_critic_return_channel.sh
git commit -m "fix(0281): Step 3 reads the verdict from the critic's return, with a bounded no-verdict abstain"
```

---

### Task 3: Convention *Composition* — move the critic into the in-context-return family

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (the paragraph beginning `**Composition (change 0017).**` — and nothing else in the file)
- Modify: `tests/test_critic_return_channel.sh` (append the `(c)` assert block)
- Test: `tests/test_critic_return_channel.sh`

**Interfaces:**
- Consumes: `REPO`, `fail`, `assert`, `flat`, `CONV` from Task 1's preamble.
- Produces: nothing.

- [ ] **Step 1: Write the failing asserts**

In `tests/test_critic_return_channel.sh`, insert this block **immediately above** the closing epilogue (i.e. below Task 2's block):

```bash
# --- (c) the convention reclassifies the critic dispatch ----------------------------------------
assert "convention exists and is non-empty" '[ -s "$CONV" ]'

# Slice the Composition paragraph — it is one physical line beginning with the bolded marker.
comp="$(grep -F -- '**Composition (change 0017).**' "$CONV")"
assert "Composition paragraph located" '[ -n "$comp" ]'
comp_flat="$(flat "$comp")"
# Non-vacuity: the paragraph still names the critic at all (test_composition_wiring.sh also binds
# this; asserting it here keeps the ORDERING check below from passing on an absent needle).
assert "Composition still names docket-auto-groom-critic" \
  'grep -qF -- "docket-auto-groom-critic" <<<"$comp_flat"'
# Non-vacuity: the git-state clause is still present and still says what it says.
assert "Composition still carries the git-state-contract clause" \
  'grep -qF -- "contract is **git state**" <<<"$comp_flat"'

# THE PROPERTY. The git-state-contract clause must be CLOSED before the critic is introduced, so
# the critic can no longer be an antecedent of "These dispatches … their contract is git state".
# Expressed as a byte-offset ordering over the flattened paragraph: a mechanical fact, not a parse
# of English. `awk index()` is 1-based and returns 0 when absent — both needles are asserted
# present above, so a 0 here would be a bug in the slice, not a legitimate ordering.
offset_of(){ awk -v s="$1" 'BEGIN{ }{ print index($0, s) }' <<<"$comp_flat"; }
gs_at="$(offset_of 'contract is **git state**')"
critic_at="$(offset_of 'docket-auto-groom-critic')"
assert "both offsets resolved (git-state=$gs_at critic=$critic_at)" \
  '[ "$gs_at" -gt 0 ] && [ "$critic_at" -gt 0 ]'
assert "the git-state clause closes BEFORE the critic is introduced" \
  '[ "$gs_at" -lt "$critic_at" ]'

# The positive half of the reclassification: the critic's verdict is an in-context return, and
# neither git state nor agent messaging. Bounded gaps keep each inside one sentence.
assert "Composition: the critic's verdict flows back in-context as the dispatch's return" \
  'grep -qE "in-context as the dispatch.{0,3}s return" <<<"$comp_flat"'
assert "Composition: never via git state and never via agent messaging" \
  'grep -qE "never via git state[^.]{0,60}never via agent messaging" <<<"$comp_flat"'

# Regression anchor: the never-yield rule and the caller's reciprocal reading are untouched by
# this edit. If the re-order dropped either, that is a silent contract loss.
assert "Composition: the never-yield rule survives" \
  'grep -qF -- "to await a task-notification" <<<"$comp_flat"'
assert "Composition: the never-adopt-a-child's-files rule survives" \
  'grep -qF -- "never adopts or commits a child" <<<"$comp_flat"'

# Non-vacuity anchor (mutation-in-fixture): the ordering matcher must actually FIRE on the shape it
# rejects — the pre-0281 wording, where the critic is enumerated inside the git-state clause. A
# typo in either needle would otherwise make the ordering assert permanently, vacuously green.
probe_flat="$(flat 'docket-auto-groom dispatches the docket-auto-groom-critic subagent; their contract is **git state** on origin/docket.')"
p_gs="$(awk -v s='contract is **git state**' '{ print index($0, s) }' <<<"$probe_flat")"
p_cr="$(awk -v s='docket-auto-groom-critic' '{ print index($0, s) }' <<<"$probe_flat")"
assert "the ordering matcher rejects the pre-0281 shape (git-state=$p_gs critic=$p_cr)" \
  '[ "$p_gs" -gt 0 ] && [ "$p_cr" -gt 0 ] && [ "$p_gs" -gt "$p_cr" ]'
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_critic_return_channel.sh`

Expected: FAIL. Specifically `the git-state clause closes BEFORE the critic is introduced` prints `NOT OK` (today the critic is named first), as do the two positive reclassification asserts. Every anchor and regression assert prints `ok`, including the fixture probe.

- [ ] **Step 3: Re-order the Composition paragraph**

In `skills/docket-convention/SKILL.md`, the *Composition* paragraph currently opens with this text (verbatim — everything from `**Composition (change 0017).**` through `never an in-context return.`):

```
**Composition (change 0017).** `docket-implement-next` dispatches the `docket-status` subagent (step 0) and the `docket-adr` subagent (step 6); `docket-auto-groom` dispatches the `docket-auto-groom-critic` subagent for its adversarial gate. These dispatches are **foreground** (the parent suspends until the child returns) and **unconditional**; their contract is **git state** on `origin/docket`, re-read after a re-sync — never an in-context return.
```

Replace **only that leading portion** with:

```
**Composition (change 0017).** `docket-implement-next` dispatches the `docket-status` subagent (step 0) and the `docket-adr` subagent (step 6). These dispatches are **foreground** (the parent suspends until the child returns) and **unconditional**; their contract is **git state** on `origin/docket`, re-read after a re-sync — never an in-context return. `docket-auto-groom` dispatches the `docket-auto-groom-critic` subagent for its adversarial gate — foreground and unconditional on the same terms, but its verdict flows back to the groom **in-context as the dispatch's return** — never via git state and never via agent messaging, because a dispatched groom is not registered under its skill name and no name-addressed delivery to it resolves (change 0281).
```

Everything from `**Foreground means the parent *actively blocks*…** ` to the end of the paragraph is left **byte-identical**. Do not touch any other paragraph, section, or line of this file — change 0260 is queued against it.

- [ ] **Step 4: Run the guard to verify it passes**

Run: `bash tests/test_critic_return_channel.sh`

Expected: PASS, exit 0.

- [ ] **Step 5: Run the neighbouring guards for regressions**

Run: `bash tests/test_composition_wiring.sh && bash tests/test_dispatch_capability.sh && bash tests/test_convention_extraction.sh && bash tests/test_skill_size_budgets.sh`

Expected: all PASS. `test_composition_wiring.sh` is the sharpest check — it asserts the paragraph still names `docket-auto-groom-critic`, still says `no skill` and `only docket-convention`, still carries the never-yield rule, and still pins no literal model/effort tier.

- [ ] **Step 6: Mutation-test the ordering assert — BOTH deletion and inversion**

This assert is a comparison, so it needs two different probes. A deletion probe reddening it says nothing about whether the operator is the right way round.

```bash
# Probe A — DELETION/REVERSION: restore the pre-0281 wording (critic enumerated inside the
# git-state clause). The ordering assert must go red.
git stash list >/dev/null  # (use an editor undo or a scratch copy; restore with git checkout --)
```

Perform probe A by hand: re-join the two sentences back into the original single sentence shown in Step 3, run `bash tests/test_critic_return_channel.sh`, confirm `NOT OK - the git-state clause closes BEFORE the critic is introduced`, then `git checkout -- skills/docket-convention/SKILL.md`.

```bash
# Probe B — INVERSION: flip the comparison operator in the guard itself and confirm it goes red
# against the CORRECT prose. If it stays green, the assert is not testing what it claims.
perl -pi -e "s/\Q[ \"\$gs_at\" -lt \"\$critic_at\" ]\E/[ \"\$gs_at\" -gt \"\$critic_at\" ]/" tests/test_critic_return_channel.sh
bash tests/test_critic_return_channel.sh   # expect: NOT OK - the git-state clause closes BEFORE ...; FAIL
git checkout -- tests/test_critic_return_channel.sh
```

Also probe the fixture: change one needle in the `probe_flat` block to a typo, confirm `the ordering matcher rejects the pre-0281 shape` reddens, restore.

Record every probe and its outcome in the commit message.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_critic_return_channel.sh
git commit -m "fix(0281): Composition moves the critic dispatch into the in-context-return family"
```

---

### Task 4: Full-suite gate

**Files:** none modified unless the suite surfaces a finding.

**Interfaces:**
- Consumes: the three committed prose edits and the completed guard.
- Produces: the build-evidence record the review step validates.

- [ ] **Step 1: Run the whole suite**

Run the command `finalize.test_command` resolves to — read it from there, never a second copy. It is currently `scripts/run-tests.sh`. Run the **entire** suite, not only the files this branch touched. It takes several minutes and runs files in parallel with per-job isolation; background it to a stable log and block on the exit code rather than holding a foreground call open.

```bash
scripts/run-tests.sh
```

Expected: green.

- [ ] **Step 2: Read the trailing OVER BUDGET report**

A trailing `OVER BUDGET:` line does **not** fail the run, so nothing else will catch it. Read it and act:

- `tests/test_sync_agents_runners.sh` (~190s vs a 60s ceiling) is **PRE-EXISTING**, tracked as change 0280, and **not this branch's**. Leave it. Do not raise its row.
- `tests/test_critic_return_channel.sh` over its 10s row **is** this branch's. Re-measure with `scripts/run-tests.sh -j 1 --timings tests/test_critic_return_channel.sh` and raise the row to the measured number rounded up to the next multiple of 5 plus 5s, re-pinning `EXPECTED_TOTAL` to match.
- Any other file this branch touched going over is likewise this branch's to fix.

- [ ] **Step 3: Commit any budget correction**

Only if Step 2 required one:

```bash
git add tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "chore(0281): re-measure the critic return-channel guard's runtime ceiling"
```

If no correction was needed, this task produces no commit — that is the expected outcome.

---

## Self-Review

**1. Spec coverage.** Spec §1 (critic agent source delivery clause) → Task 1 Step 3, both bullets, with the belief-changes-nothing sentence carried verbatim. Spec §2 (Step 3 receiving half + bounded no-verdict posture: one collect, one re-dispatch, Tier B abstain, never a third, never indefinite, plus the re-dispatch-is-safe rationale) → Task 2 Step 3. Spec §3 (Composition reclassification, one-sentence surgical edit, never-yield and the rest standing) → Task 3 Step 3, with the never-yield rule asserted as a regression anchor. Spec §4 (one mutation-tested sentinel binding all three: (a) final-report + never-address, (b) no-verdict→abstain with a bounded-gap whitespace-collapsed match, (c) the critic no longer inside the git-state clause) → the guard built across Tasks 1-3, mutation-probed in Task 1 Step 5, Task 2 Step 6, and Task 3 Step 6. Spec assumption 4 (prose + one test, no scripts) → no task touches `scripts/`. No gaps.

**2. Placeholder scan.** Every prose insertion is quoted in full; every assert is given as literal shell; every mutation probe names the command and the expected `NOT OK` line. No "TBD", no "handle edge cases", no "similar to Task N" (the guard preamble is reproduced once in Task 1 and referenced by exact variable name in the Interfaces blocks of Tasks 2 and 3, which is a definition, not an elision).

**3. Type consistency.** `REPO`, `fail`, `assert`, `flat`, `CRITIC`, `AUTOGROOM`, `CONV` are defined in Task 1 and used with those exact names in Tasks 2 and 3. `step3`/`step3_flat` are local to Task 2; `comp`/`comp_flat`/`offset_of`/`gs_at`/`critic_at` local to Task 3 — no collisions. The assert description strings quoted in the "expected FAIL" steps match the strings in the assert calls character-for-character. The budget row path `tests/test_critic_return_channel.sh` matches the created filename and the `EXPECTED_TOTAL` arithmetic (1670 + 10 = 1680).
