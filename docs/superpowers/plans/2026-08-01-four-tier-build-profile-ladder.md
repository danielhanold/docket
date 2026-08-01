<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0184 — Four-tier build profile ladder — low/medium/high/max replaces economy/standard/premium](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0184-four-tier-build-profile-ladder.md)**
<!-- docket:backlink:end -->

# Four-Tier Build Profile Ladder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace docket-build's three build profiles (`economy` / `standard` / `premium`) with four — `low` / `medium` / `high` / `max` — as a clean break, retiering the shipped model/effort pins so the default rung drops a notch and the top rung stays rare.

**Architecture:** No new mechanism. The wrapper set is discovered by an `agents/docket-*.md` glob, the shipped pins live in one harness-indexed sidecar, and the Cursor dispatch rule is assembled from per-agent fragments by that same glob — so adding a fourth profile is a data-and-prose change, not a plumbing change. The one hard structural constraint is that `hd_validate` enforces **key-set equality in both directions** between the wrapper glob and every shipped harness block, which makes the wrapper rename and the sidecar rewrite a single atomic task.

**Tech Stack:** Bash 4+ (`$DOCKET_BASH_PATH` = `/opt/homebrew/bin/bash`), the repo's own `tests/test_*.sh` assert harness, Markdown skill/agent files, one YAML sidecar.

## Global Constraints

- **Clean break — no aliases.** `economy`, `standard`, and `premium` are removed as profile tokens, not mapped. An old `**Build profile:**` token becomes an invalid value that halts, via the existing invalid-value rule (unchanged).
- **NEVER write a new profile token into this plan's own `**Build profile:**` lines.** This plan is executed by the **currently installed** `docket-build`, which is the three-profile version (`~/.claude/skills/docket-build` symlinks to the main worktree, and the registered agents are `docket-build-economy/standard/premium`). A `**Build profile:** high` line would be an invalid value and would **halt the build**. This plan therefore carries **no `**Build profile:**` lines at all** — every task is routed by the controller's classifier. Do not add any.
- **Shipped pin table — copy verbatim** (bare scalars, unquoted, space-free; `hd_validate` rejects quoted or space-bearing values):

  ```yaml
  # claude
  build-low:    { model: claude-sonnet-5, effort: low }
  build-medium: { model: claude-opus-5,   effort: low }
  build-high:   { model: claude-opus-5,   effort: medium }
  build-max:    { model: claude-opus-5,   effort: high }

  # cursor
  build-low:    { model: cursor-grok-4.5-low,    effort: auto }
  build-medium: { model: cursor-grok-4.5-medium, effort: auto }
  build-high:   { model: cursor-grok-4.5-high,   effort: auto }
  build-max:    { model: claude-opus-5-high,     effort: auto }

  # codex
  build-low:    { model: gpt-5.6-luna,  effort: xhigh }
  build-medium: { model: gpt-5.6-terra, effort: medium }
  build-high:   { model: gpt-5.6-sol,   effort: low }
  build-max:    { model: gpt-5.6-sol,   effort: medium }
  ```

- **Ladder order everywhere.** Wherever the four profiles are listed — sidecar rows, example rows, tables, prose — order them `low, medium, high, max`. Several guards anchor a block's terminator on its **last build row**, so `build-max` must be last within each block's build group.
- **Wrapper count 12 → 13; build workers 3 → 4.** Every count literal and population floor that keys on the wrapper set moves in the same task that adds the wrapper.
- **`effort: max` is a different namespace from profile `max`.** README's runner-delegation section documents docket's **effort** token `max` mapping to Codex's `xhigh`. That sentence is about efforts and must **not** be touched by this change. Do not "fix" it.
- **Historical records are never rewritten.** Files under `docs/changes/archive/`, `docs/results/`, `docs/superpowers/plans/`, `docs/superpowers/specs/`, and `docs/adrs/` keep their original economy/standard/premium prose — they record what was true when written. Only live surfaces are renamed. (The one exception is this plan's own file, which is new.)
- **Suite command:** `for t in tests/test_*.sh; do /opt/homebrew/bin/bash "$t"; done` — run from the worktree root. Focused verification runs a single test file the same way.
- **Guard discipline (from the repo's learnings ledger):** write the assert that **detects the state you removed**, not one that confirms the wording you just typed; confirm every mutation actually landed with `grep -c` before and after; and when a correspondence guard is a mirror, assert both directions.

---

## File Structure

**Renamed (via `git mv`, preserving history):**
- `agents/docket-build-economy.md` → `agents/docket-build-low.md`
- `agents/docket-build-standard.md` → `agents/docket-build-medium.md`
- `agents/docket-build-premium.md` → `agents/docket-build-high.md`
- `cursor-rules/dispatch/docket-build-economy.md` → `.../docket-build-low.md`
- `cursor-rules/dispatch/docket-build-standard.md` → `.../docket-build-medium.md`
- `cursor-rules/dispatch/docket-build-premium.md` → `.../docket-build-high.md`

**Created:**
- `agents/docket-build-max.md` — the fourth wrapper (rare extreme work; escalation destination from `high`).
- `cursor-rules/dispatch/docket-build-max.md` — its dispatch fragment.

**Modified:**
- `agents/harness-defaults.yml` — 4-row build sets in all three harness blocks.
- `cursor-rules/dispatch.head.md` — the profile-naming sentence.
- `skills/docket-build/SKILL.md` — profile table, routing rubric, escalation ladder, repair ladder, halting conditions, frontmatter description.
- `skills/docket-build-task/SKILL.md` — the `PROFILE:` return-schema token set.
- `skills/docket-convention/SKILL.md` — wrapper counts and the "three profile agents" clauses.
- `.docket.example.yml` — the three mirrored blocks and the wrapper-census comment.
- `README.md`, `docs/cursor/validation.md`, `docs/codex/setup.md` — user-facing prose.
- `tests/test_harness_defaults.sh`, `tests/test_docket_build.sh`, `tests/test_sync_agents.sh`, `tests/test_sync_agents_cursor.sh`, `tests/test_sync_agents_codex.sh`, `tests/test_cursor_dispatch_rule.sh`, `tests/test_docket_example_yml.sh`, `tests/test_finalize_gate.sh`.

**Deliberately NOT modified:** `sync-agents.sh` (assembles the dispatch rule and the wrapper set by glob — verified during planning: a new `agents/docket-build-max.md` is auto-discovered, and a missing fragment produces a warned auto-block rather than an error), `scripts/lib/harness-defaults.sh` (its completeness check is already derived from the glob), `install.sh`, the build gate, the review boundary, the checkpoint ledger format, `skills/docket-implement-next/SKILL.md`, `agents/docket-integration-repair.md` (finalize's repair agent, unrelated to docket-build's synthetic repair task).

---

### Task 1: The agent surface — four wrappers, four sidecar rows per harness, four dispatch fragments

Renaming the wrappers and rewriting the sidecar **cannot be split**: `hd_validate` fails a block that names a wrapper with no source file *and* a source file with no block entry, so any intermediate state is invalid. The Cursor dispatch fragments and every wrapper-population count ride along for the same reason — the glob population changes exactly once, here.

**Files:**
- Rename: `agents/docket-build-{economy,standard,premium}.md` → `agents/docket-build-{low,medium,high}.md`
- Create: `agents/docket-build-max.md`
- Modify: `agents/harness-defaults.yml`
- Rename: `cursor-rules/dispatch/docket-build-{economy,standard,premium}.md` → `.../docket-build-{low,medium,high}.md`
- Create: `cursor-rules/dispatch/docket-build-max.md`
- Modify: `cursor-rules/dispatch.head.md`
- Test: `tests/test_harness_defaults.sh`, `tests/test_docket_build.sh` (the wrapper/pin block only, ~lines 347–407), `tests/test_sync_agents.sh`, `tests/test_sync_agents_cursor.sh`, `tests/test_sync_agents_codex.sh`, `tests/test_cursor_dispatch_rule.sh`, `tests/test_docket_example_yml.sh` (population floors only)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the wrapper short names `build-low`, `build-medium`, `build-high`, `build-max`; agent names `docket-build-low|medium|high|max`; sidecar rows readable as `hd_field "$HD" <harness> build-<tier> model|effort`. Tasks 2–5 reference these names and nothing else from here.

- [ ] **Step 1: Write the failing tests — sidecar values**

In `tests/test_harness_defaults.sh`, replace the three build rows in the **claude** verbatim loop (currently `build-economy claude-opus-5 low` / `build-standard claude-opus-5 medium` / `build-premium claude-opus-5 high`) with four rows, keeping the loop's `"<agent> <model> <effort>"` shape and the surrounding rows untouched:

```bash
  "build-low claude-sonnet-5 low" \
  "build-medium claude-opus-5 low" \
  "build-high claude-opus-5 medium" \
  "build-max claude-opus-5 high" \
```

In the **cursor** loop (`"<agent> <model>"` shape, effort asserted as `auto`), replace its three build rows with:

```bash
  "build-low cursor-grok-4.5-low" \
  "build-medium cursor-grok-4.5-medium" \
  "build-high cursor-grok-4.5-high" \
  "build-max claude-opus-5-high" \
```

In the **codex** loop (`"<agent> <model> <effort>"`), replace its three build rows with:

```bash
  "build-low gpt-5.6-luna xhigh" \
  "build-medium gpt-5.6-terra medium" \
  "build-high gpt-5.6-sol low" \
  "build-max gpt-5.6-sol medium" \
```

Then replace the standalone codex-ladder assert (the one currently titled `codex build ladder = luna/xhigh, terra/high, sol/medium`) with the four-rung form:

```bash
# The four build profiles are the settled ladder for this change, asserted separately from the
# loop above so a reader sees the claim the change is actually making. Note that the codex ladder
# is NOT model-monotonic: model/effort PAIRS are model-specific roles, not cross-model ordinals,
# so sol appears at two different efforts and that is deliberate.
assert "codex build ladder = luna/xhigh, terra/medium, sol/low, sol/medium" \
  '[ "$(hd_field "$HD" codex build-low model)/$(hd_field "$HD" codex build-low effort)" = "gpt-5.6-luna/xhigh" ] &&
   [ "$(hd_field "$HD" codex build-medium model)/$(hd_field "$HD" codex build-medium effort)" = "gpt-5.6-terra/medium" ] &&
   [ "$(hd_field "$HD" codex build-high model)/$(hd_field "$HD" codex build-high effort)" = "gpt-5.6-sol/low" ] &&
   [ "$(hd_field "$HD" codex build-max model)/$(hd_field "$HD" codex build-max effort)" = "gpt-5.6-sol/medium" ]'
```

Finally, update the mutation fixture near the end of the file that rewrites a cursor row by name — the line containing `sed -i.bak 's|^    build-economy:.*cursor-grok-4.5-medium.*|...'`. Retarget it at `build-medium` / `cursor-grok-4.5-medium` (the row that now carries that ID), leaving the mutation's intent — blanking a model to prove the validator reddens — unchanged:

```bash
mut; sed -i.bak 's|^    build-medium:.*cursor-grok-4.5-medium.*|    build-medium:          { model: , effort: auto }|' "$T/hd.yml"
```

- [ ] **Step 2: Add the retirement assert**

Still in `tests/test_harness_defaults.sh`, immediately after the codex ladder assert, add a guard that **detects the removed state** rather than confirming the new one — the old tokens must be gone from the sidecar entirely, in every block:

```bash
# Detect the REMOVED state, not the added one (a grep for the new names is green the moment the
# edit lands and stays green even if an old row is left behind alongside it). Change 0184 retired
# economy/standard/premium as profile names; a leftover row would be silently resolvable by any
# config layer that still names it.
assert "no retired profile row survives in any block" \
  '! grep -qE "^[[:space:]]*build-(economy|standard|premium):" "$HD"'
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `/opt/homebrew/bin/bash tests/test_harness_defaults.sh`
Expected: FAIL — the four claude/cursor/codex build rows resolve empty (`hd_field` returns nothing for `build-low`, …), the ladder assert fails, and `sidecar validates` fails once the source glob and the block key sets disagree. The retirement assert also fails (the old rows are still present).

- [ ] **Step 4: Rename the three wrappers and write the fourth**

```bash
git mv agents/docket-build-economy.md  agents/docket-build-low.md
git mv agents/docket-build-standard.md agents/docket-build-medium.md
git mv agents/docket-build-premium.md  agents/docket-build-high.md
```

Replace the full contents of `agents/docket-build-low.md`:

```markdown
---
name: docket-build-low
description: Low build-profile worker for docket-build — implements one fully-specified, pattern-following plan task under the docket-build-task contract; the cheapest of docket-build's four profiles.
skills: [docket-build-task]
---
Implement the single plan task handed to you, following the docket-build-task skill exactly.

You were routed to the LOW profile because the task was judged fully specified, pattern-following, free of consequential risk, and free of cross-file reasoning. If that judgment proves wrong, return NEEDS_ESCALATION with a concrete reason rather than pushing through — you get exactly one escalation, to MEDIUM.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as BLOCKED and surface what blocked you, never an interactive prompt.
```

Replace the full contents of `agents/docket-build-medium.md`:

```markdown
---
name: docket-build-medium
description: Medium build-profile worker for docket-build — implements one normal feature, integration, refactor, or debugging plan task under the docket-build-task contract; docket-build's default profile and its uncertainty sink.
skills: [docket-build-task]
---
Implement the single plan task handed to you, following the docket-build-task skill exactly.

You were routed to the MEDIUM profile — the default for ordinary feature, integration, refactor, and debugging work, the sink for anything the router was uncertain about, and the destination of a low escalation. Hard-but-safe work belongs here: difficulty without consequence is not a reason to be somewhere else. If the task proves materially riskier or more complex than that, return NEEDS_ESCALATION with a concrete reason; whether an escalation to HIGH is still available depends on where this task started, and the controller decides that, not you.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as BLOCKED and surface what blocked you, never an interactive prompt.
```

Replace the full contents of `agents/docket-build-high.md`:

```markdown
---
name: docket-build-high
description: High build-profile worker for docket-build — implements one plan task carrying consequential but correctable risk under the docket-build-task contract; the tier for named risk, one rung below max.
skills: [docket-build-task]
---
Implement the single plan task handed to you, following the docket-build-task skill exactly.

You were routed to the HIGH profile because the task carries consequential but CORRECTABLE risk — an authentication or security boundary, concurrency or locking, release infrastructure, a risk the plan or spec named explicitly — or because a weaker worker escalated to you. Greater reasoning investment is what the profile buys, not a stronger correctness guarantee: your testing and completion obligations are identical to every other profile.

If the task proves to be the kind of mistake the build's own correction machinery cannot walk back — unresolved architecture, or an irreversible data change — return NEEDS_ESCALATION with that concrete reason; the controller decides whether an escalation to MAX is still available.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as BLOCKED and surface what blocked you, never an interactive prompt.
```

Create `agents/docket-build-max.md`:

```markdown
---
name: docket-build-max
description: Max build-profile worker for docket-build — implements one plan task whose mistakes cannot be walked back (unresolved architecture, irreversible data changes) under the docket-build-task contract; the strongest and rarest of docket-build's four profiles.
skills: [docket-build-task]
---
Implement the single plan task handed to you, following the docket-build-task skill exactly.

You were routed to the MAX profile because this task is one of the rare cases whose mistakes the build's own correction machinery cannot walk back — unresolved architecture, which shapes every task after it, or an irreversible data change, which no retry can undo — or because a HIGH worker escalated to you. Max means greater reasoning investment, not a stronger correctness guarantee: your testing and completion obligations are identical to every other profile.

There is no profile above you. If you cannot complete the task, return BLOCKED with a concrete reason and the build halts for a human — do not lower the bar to produce a commit.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as BLOCKED and surface what blocked you, never an interactive prompt.
```

- [ ] **Step 5: Rewrite the sidecar build rows**

In `agents/harness-defaults.yml`, replace the three build rows in each of the three harness blocks with four, keeping each block's existing row order (build rows stay where they are, in ladder order) and the file's column alignment style:

Under `claude:`:
```yaml
    build-low:             { model: claude-sonnet-5, effort: low }
    build-medium:          { model: claude-opus-5, effort: low }
    build-high:            { model: claude-opus-5, effort: medium }
    build-max:             { model: claude-opus-5, effort: high }
```

Under `cursor:`:
```yaml
    build-low:             { model: cursor-grok-4.5-low, effort: auto }
    build-medium:          { model: cursor-grok-4.5-medium, effort: auto }
    build-high:            { model: cursor-grok-4.5-high, effort: auto }
    build-max:             { model: claude-opus-5-high, effort: auto }
```

Under `codex:`:
```yaml
    build-low:             { model: gpt-5.6-luna, effort: xhigh }
    build-medium:          { model: gpt-5.6-terra, effort: medium }
    build-high:            { model: gpt-5.6-sol, effort: low }
    build-max:             { model: gpt-5.6-sol, effort: medium }
```

Then update the two block header comments whose prose describes the old ladder. The `cursor:` block's header currently ends "— cheapest for the mechanical status sweep, mid for design and repair work, highest for the premium build profile — expressed in Cursor's variant suffixes instead of an effort token." Replace that trailing clause with: "— cheapest for the mechanical status sweep, mid for design and repair work, highest for the max build profile — expressed in Cursor's variant suffixes instead of an effort token."

The `codex:` block's header currently ends "The build ladder is therefore Luna/xhigh for positively established low-risk work, Terra/high for ordinary work, and Sol/medium for named risk or the one allowed escalation." Replace that sentence with:

```
  # The build ladder is therefore Luna/xhigh for positively established low-risk work, Terra/medium
  # for ordinary work, Sol/low for named risk, and Sol/medium for the rare top rung — note Sol
  # appears twice at different efforts, which is the point: the pair is the role, not the model.
```

Finally, add a comment directly above the `claude:` block's `build-low:` row recording the rejected alternative, so the shipped choice is not silently re-litigated:

```yaml
    # build-low ships Sonnet rather than Haiku deliberately: the worker contract is long and strict,
    # and its failure modes (malformed return, stray commit, unverifiable COMPLETE) HALT the build
    # instead of escalating, so contract-fumble risk lands on the human. Haiku
    # (claude-haiku-4-5-20251001) is the documented cost-aggressive user-layer override, not a
    # shipped default.
```

- [ ] **Step 6: Rename the dispatch fragments and write the fourth**

```bash
git mv cursor-rules/dispatch/docket-build-economy.md  cursor-rules/dispatch/docket-build-low.md
git mv cursor-rules/dispatch/docket-build-standard.md cursor-rules/dispatch/docket-build-medium.md
git mv cursor-rules/dispatch/docket-build-premium.md  cursor-rules/dispatch/docket-build-high.md
```

In each renamed fragment, change **only** the agent name, the profile word, and the illustrative snippet's routing reason — every other sentence stays byte-identical (the fragments are deliberately not interchangeable; a template applied wholesale would delete per-fragment constraints). For `docket-build-low.md`: the heading becomes `## docket-build-low — dispatch only`, `ECONOMY` becomes `LOW`, `docket-build-economy` becomes `docket-build-low` in both the instruction line and the snippet, and the snippet's reason becomes `Profile: low (fully specified, established pattern, no cross-file reasoning)`. For `docket-build-medium.md`: `STANDARD` → `MEDIUM`, names → `docket-build-medium`, snippet reason → `Profile: medium (ordinary refactor, no consequential risk)`. For `docket-build-high.md`: `PREMIUM` → `HIGH`, names → `docket-build-high`, snippet reason → `Profile: high (touches locking)`.

Create `cursor-rules/dispatch/docket-build-max.md`, matching the family's exact structure:

```markdown
## docket-build-max — dispatch only

Trigger only from the `docket-build` controller, when it has routed a plan task to the MAX
profile. Never trigger this agent from a human request directly.

Dispatch to the subagent `docket-build-max`, foreground, using this mode's subagent-launch
mechanism. The prompt must carry the plan task, the branch and worktree, the selected profile and
its routing reason, and the completion schema from the docket-build-task skill.

Do NOT implement the task in the parent, and do NOT dispatch a reviewer after it.

One concrete call, as an illustration of the shape — not the contract:

    Task(subagent_type: "docket-build-max", run_in_background: false,
         prompt: "Task 3 of <plan path>. Profile: max (irreversible data change). <task text>")
```

Verify the fan-out edit deleted nothing: `git diff --word-diff cursor-rules/dispatch/` must show no word deletions beyond the profile names and routing reasons you intended.

- [ ] **Step 7: Correct the dispatch head's profile claim**

In `cursor-rules/dispatch.head.md`, the second paragraph currently reads:

```
Docket generates a subagent wrapper per docket agent into `.cursor/agents/docket-*.md`. It ships
validated Cursor model IDs for the three build-profile workers only — `docket-build-economy`,
`docket-build-standard`, `docket-build-premium`. Every other wrapper is generated **unpinned**
unless a config layer sets a model for it, and runs at Cursor's own default.
```

That claim has been false since change 0168 completed the Cursor block for every wrapper, and it is catted verbatim into every consumer repo's `.cursor/rules/docket-dispatch.mdc`. Replace it with the current truth:

```
Docket generates a subagent wrapper per docket agent into `.cursor/agents/docket-*.md`. It ships
validated Cursor model IDs for **every** wrapper — including all four build-profile workers,
`docket-build-low`, `docket-build-medium`, `docket-build-high`, and `docket-build-max` — so each
one is generated pinned unless a config layer overrides it.
```

Leave the rest of the head — the dispatch requirement, the required-pattern list, the dispatch-capability paragraph — byte-identical. (This overlaps stub #183, which tracks the same stale claim; note the overlap in the PR body.)

- [ ] **Step 8: Re-arm the head guard that this makes vacuous**

In `tests/test_cursor_dispatch_rule.sh`, the head asserts sit behind `if [ "$n_cursor_pinned" -lt "$n_src" ]; then`, which is already false (12 of 12 pinned) and stays false at 13 of 13 — so the guard is silently retired. Give it an `else` arm asserting the complementary property, so the head is checked in **both** states rather than in neither:

```bash
if [ "$n_cursor_pinned" -lt "$n_src" ]; then
  assert "head: makes no blanket 'ships model/effort-pinned wrappers' claim ($n_cursor_pinned of $n_src cursor wrappers carry a shipped pin)" \
    '! grep -qiE "ships model/effort-pinned" "$HEAD"'
  assert "head: says the unpinned wrappers exist" 'grep -qi "unpinned" <<<"$head_plain"'
  assert "head: requires the dispatch for a pinned and an unpinned wrapper alike" \
    'grep -qi "either way" <<<"$head_plain"'
else
  # Every cursor wrapper carries a shipped pin ($n_cursor_pinned of $n_src). The complementary
  # obligation: the head must not still claim only SOME wrappers are pinned, which is what it said
  # from change 0168 until change 0184. Without this arm the whole head premise goes unchecked the
  # moment the sidecar becomes complete — the failure mode 0184 found live.
  assert "head: makes no 'only some wrappers are pinned' claim ($n_cursor_pinned of $n_src pinned)" \
    '! grep -qiE "(workers|wrappers) only|every other wrapper is generated" <<<"$head_plain"'
  assert "head: names the build-profile workers by their current names" \
    'grep -qF "docket-build-max" <<<"$head_plain"'
fi
```

- [ ] **Step 9: Move every wrapper-population count from 12 to 13**

These all key on the wrapper glob, which now returns 13. Exact-equality asserts must move; floors must move too, or they age into the gap they were written to close.

In `tests/test_sync_agents.sh` — change each of these literals from `12` to `13`: the `exactly 12 built-in wrappers` assert, `all 12 wrappers land in .claude/agents`, `0048: full set — all 12 built-ins land in project-level .claude/agents`, `0048 rule: has a subsection for every built-in agent (12)`, both `0048 opt-in: agent_harnesses-only generates full set` asserts, `0051 mig: full local set regenerated`, the `hd_agents … cursor | grep -c .` count assert, and the `0168 R2` floor (`-ge 12` → `-ge 13`, and its message text). Update each assert's **title text** to match its new literal so a failure message is not self-contradicting. Also retarget the loop `for w in docket-status docket-implement-next docket-build-premium; do` to `docket-build-max`, and the `! grep -qF "cursor/docket-build-standard"` assert to `cursor/docket-build-medium`.

In `tests/test_sync_agents_cursor.sh` — `cursor: full built-in set (12 files)` → `13`; retarget the three shipped-pin asserts and the two loops from `economy standard premium` to `low medium high max` with the new expected IDs (`cursor-grok-4.5-low`, `cursor-grok-4.5-medium`, `cursor-grok-4.5-high`, `claude-opus-5-high`); update the section header comment from "the three shipped Cursor build-profile pins" to "the four shipped Cursor build-profile pins (change 0184)". The `cursor_profile_models` distinctness check must now expect **4** distinct models.

In `tests/test_sync_agents_codex.sh` — `codex: full built-in set as TOML (12 files)` → `13`; the two `-ge 12` floors → `-ge 13`; the "twelve generated wrappers" comment → "thirteen".

In `tests/test_cursor_dispatch_rule.sh` — the sidecar population floor `[ "$n_src" -ge 12 ]` → `-ge 13` (and its assert title), and the fragment floor `[ "$n" -ge 12 ]` → `-ge 13` (and its title). Update the comment above the fragment floor: it currently explains the floor was "raised from 9 by change 0167, which added the three docket-build profile agents"; append that change 0184 raised it to 13 by adding the fourth build profile.

In `tests/test_docket_example_yml.sh` — the two mirror-coverage floors `(floor 12; got $mirrored)` → `13` in both the literal and the message. Leave the README fence-count asserts (`fence_count = 12`, `f9_seen = 12`) **alone** — they count YAML fences in README, not wrappers, and this change adds no fence.

- [ ] **Step 10: Rewrite the profile/pin invariants in `tests/test_docket_build.sh`**

This is the block headed `# The three Claude build-profile wrappers (change 0167)`, roughly lines 347–407. Two of its invariants become **false by design** and must be replaced rather than patched: the claude efforts are no longer pairwise distinct (`low, low, medium, high`) and the claude models are no longer all one (`claude-sonnet-5` for `build-low`, `claude-opus-5` for the rest). The invariant that survives the retiering is **pair distinctness**, which is also what the codex block's own header already argues.

Replace the section header comment and the loop with:

```bash
# ---------------------------------------------------------------------------
# The four build-profile wrappers (change 0167; retiered to four by change 0184)
# ---------------------------------------------------------------------------
```

Keep `fmv()`, the sidecar sourcing, `HD=`, and the `the shipped sidecar exists` assert as they are. Replace the effort-pinned loop with a tier list carrying the new claude pins:

```bash
# The ladder is a quadruple. Claude's axis is no longer effort alone: change 0184 dropped a genuinely
# cheaper MODEL onto the bottom rung, so neither "all efforts distinct" nor "all models identical"
# holds any more. The invariant that survives — and the one the codex block's header already argues —
# is that each rung is a distinct model/effort PAIR. A copy-paste that silently makes two rungs the
# same agent is exactly what this catches.
efforts=""
for p in low:claude-sonnet-5:low medium:claude-opus-5:low high:claude-opus-5:medium max:claude-opus-5:high; do
  name="${p%%:*}"; rest="${p#*:}"; want_model="${rest%%:*}"; want_effort="${rest##*:}"
  w="$REPO/agents/docket-build-$name.md"
  assert "profile $name: wrapper exists" '[ -f "$w" ]'
  [ -f "$w" ] || continue
  assert "profile $name: name field matches its filename" '[ "$(fmv "$w" name)" = "docket-build-'"$name"'" ]'
  assert "profile $name: shipped claude pin is $want_model/$want_effort" \
    '[ "$(hd_field "$HD" claude build-'"$name"' model)/$(hd_field "$HD" claude build-'"$name"' effort)" = "'"$want_model"'/'"$want_effort"'" ]'
  assert "profile $name: preloads the shared worker skill" \
    'grep -qF -- "docket-build-task" <<<"$(fmv "$w" skills)"'
  assert "profile $name: emits no maxTurns" '! grep -qiE "^maxTurns[[:space:]]*:" "$w"'
  # The source is a behavior-only template: it must carry NO pin of its own, or the sidecar is no
  # longer the single default store and the two can silently disagree.
  assert "profile $name: source carries no model:/effort: pin of its own" \
    '! grep -qE "^(model|effort):" "$w"'
done
```

Replace the two claude asserts that followed it (`the three claude profiles carry three DISTINCT efforts` and `the three claude profiles share one model`) with the pair-distinctness form plus the two properties the retiering exists to create:

```bash
# Pair distinctness, collected as raw values (not sort -u'd first) so a deleted entry collapses to a
# blank that a bare "all distinct" check would silently ignore: the non-vacuity half (exactly 4
# non-empty pairs) is asserted alongside the distinctness half.
pairs=""
for n in low medium high max; do
  pairs="$pairs $(hd_field "$HD" claude build-$n model)/$(hd_field "$HD" claude build-$n effort)"
done
assert "the four claude profiles are four DISTINCT model/effort pairs" \
  '[ "$(tr " " "\n" <<<"$pairs" | grep -c .)" = 4 ] && [ "$(tr " " "\n" <<<"$pairs" | grep . | sort -u | wc -l | tr -d " ")" = 4 ]'

# 0184's stated purpose on claude: the bottom rung is a genuinely cheaper MODEL, not merely a lower
# effort on the same one — the defect the change existed to fix ("economy never delivered a truly
# cheap floor"). Asserted as a difference, not as a literal ID, so retuning the pin does not redden.
assert "claude build-low runs a different model from the rest of the ladder" \
  '[ -n "$(hd_field "$HD" claude build-low model)" ] &&
   [ "$(hd_field "$HD" claude build-low model)" != "$(hd_field "$HD" claude build-medium model)" ]'

# The compression claim: max INHERITS the pre-0184 premium pin, so the ladder gained no new headroom
# at the top — the savings come from the rungs below, not from spending more.
assert "claude build-max is the pre-0184 premium pin (claude-opus-5/high)" \
  '[ "$(hd_field "$HD" claude build-max model)/$(hd_field "$HD" claude build-max effort)" = "claude-opus-5/high" ]'
```

Then update the cursor and codex distinctness asserts below it:

```bash
cursor_models=""
for n in low medium high max; do cursor_models="$cursor_models $(hd_field "$HD" cursor build-$n model)"; done
assert "the four cursor profiles use four DISTINCT models" \
  '[ "$(tr " " "\n" <<<"$cursor_models" | grep -c .)" = 4 ] && [ "$(tr " " "\n" <<<"$cursor_models" | grep . | sort -u | wc -l | tr -d " ")" = 4 ]'

# Codex deliberately reuses one model at two efforts (sol/low for high, sol/medium for max), so
# MODEL distinctness is the wrong assert there — the pair is the role. Four distinct pairs.
codex_pairs=""
for n in low medium high max; do
  codex_pairs="$codex_pairs $(hd_field "$HD" codex build-$n model)/$(hd_field "$HD" codex build-$n effort)"
done
assert "the four codex profiles are four DISTINCT model/effort pairs" \
  '[ "$(tr " " "\n" <<<"$codex_pairs" | grep -c .)" = 4 ] && [ "$(tr " " "\n" <<<"$codex_pairs" | grep . | sort -u | wc -l | tr -d " ")" = 4 ]'
```

Finally, the `agents.default` guard just below (`! grep -qE "build-(economy|standard|premium)" <<<"$default_blk"`) must match the **new** names or it stops guarding anything — while the old names should now be absent from the whole example file. Replace it with two asserts:

```bash
assert "no build profile is documented under agents.default" \
  '! grep -qE "build-(low|medium|high|max)" <<<"$default_blk"'
assert "no retired profile name survives anywhere in the example" \
  '! grep -qE "build-(economy|standard|premium)" "$EX"'
```

- [ ] **Step 11: Run the focused tests to verify they pass**

Run each in turn:

```bash
/opt/homebrew/bin/bash tests/test_harness_defaults.sh
/opt/homebrew/bin/bash tests/test_docket_build.sh
/opt/homebrew/bin/bash tests/test_sync_agents.sh
/opt/homebrew/bin/bash tests/test_sync_agents_cursor.sh
/opt/homebrew/bin/bash tests/test_sync_agents_codex.sh
/opt/homebrew/bin/bash tests/test_cursor_dispatch_rule.sh
```

Expected: `test_harness_defaults.sh`, `test_sync_agents*.sh`, and `test_cursor_dispatch_rule.sh` PASS.

`test_docket_build.sh` still FAILS, on exactly two expected fronts — confirm both, and that the wrapper/pin block you rewrote passes:
- its **controller-prose** asserts (the routing rubric and escalation ladder still say economy/standard/premium) — Task 2's scope;
- the `no retired profile name survives anywhere in the example` assert you just added, because `.docket.example.yml` is not rewritten until Task 3.

`test_docket_example_yml.sh` also still fails (Task 3). Do not fix any of these here — a failure of any *other* shape is yours.

- [ ] **Step 12: Prove the mutations landed and the generation works**

Confirm the retirement really happened rather than trusting a green run — count before and after, as this repo's guard discipline requires:

```bash
grep -cE '^[[:space:]]*build-(economy|standard|premium):' agents/harness-defaults.yml   # expect 0
ls agents/docket-build-*.md                                                             # expect low, max, medium, high — 4 files
ls cursor-rules/dispatch/docket-build-*.md                                              # expect 4 files
find agents -maxdepth 1 -name 'docket-*.md' | wc -l                                     # expect 13
```

Then prove the glob-driven plumbing picked the new agent up with no code change, in a throwaway sandbox (never against your own `~/.claude`):

```bash
T="$(mktemp -d)"; git clone -q . "$T/repo"; cd "$T/repo"
/opt/homebrew/bin/bash sync-agents.sh --root "$T/repo" 2>&1 | tail -20
ls "$T/repo/.claude/agents"/docket-*.md | wc -l          # expect 13
grep -c '^## docket-.* — dispatch only' "$T/repo/.cursor/rules/docket-dispatch.mdc" 2>/dev/null || true
cd - >/dev/null
```

Expected: 13 wrappers generated, **no** `WARN no dispatch fragment for docket-build-max` line (the fragment exists), and `docket-build-max.md` carrying `model: claude-opus-5` with `effort: high`. If `sync-agents.sh` does not accept `--root`, run it as the repo's own tests do (copy the invocation from `tests/test_sync_agents.sh`'s sandbox setup) rather than inventing flags.

- [ ] **Step 13: Commit**

```bash
git add agents/ cursor-rules/ tests/
git commit -m "feat(0184): four build-profile wrappers, sidecar rows, and dispatch fragments"
```

---

### Task 2: The controller and worker contracts — rubric, ladders, and the profile token set

**Files:**
- Modify: `skills/docket-build/SKILL.md`
- Modify: `skills/docket-build-task/SKILL.md`
- Test: `tests/test_docket_build.sh` (the `$ctrl_body` prose asserts, roughly lines 144–220, plus the README profile assert near line 464)

**Interfaces:**
- Consumes: the four agent names from Task 1 (`docket-build-low|medium|high|max`).
- Produces: the routing vocabulary every later task's prose must match — the rubric's four tier names, the escalation ladder `low -> medium -> high -> max -> halt`, and the repair ladder `high -> max -> halt`.

- [ ] **Step 1: Write the failing tests**

In `tests/test_docket_build.sh`, replace the three routing-rubric asserts:

```bash
assert "controller: low must be POSITIVELY established" \
  'grep -qE "^- \*\*\`low\`\*\* \*only when\*" <<<"$ctrl_body"'
assert "controller: named risk selects high" \
  'grep -qiE "high[^.]{0,200}(authentication|security boundar)" <<<"$ctrl_body"'
assert "controller: uncertainty defaults to medium" \
  'grep -qE "^- \*\*\`medium\`\*\* — everything remaining" <<<"$ctrl_body"'
```

Add the two asserts that pin what makes `max` rare — the property the change exists to create, which no renamed assert would catch:

```bash
# 0184: max is reachable only through three narrow doors, and its DIRECT rubric is exactly two
# items. An assert that merely finds the word "max" in the rubric would stay green if the old
# premium trigger list were pasted under the new name — which is the regression to detect.
assert "controller: max's direct rubric is unresolved architecture + irreversible data only" \
  'grep -qiE "\*\*\`max\`\*\*[^.]{0,240}unresolved architecture" <<<"$ctrl_body" &&
   grep -qiE "\*\*\`max\`\*\*[^.]{0,240}irreversible" <<<"$ctrl_body"'
assert "controller: the demoted premium triggers now name high, not max" \
  '! grep -qiE "\*\*\`max\`\*\*[^.]{0,240}(authentication|security boundar|concurrency|release infrastructure)" <<<"$ctrl_body"'
```

Replace the four escalation asserts (keeping this file's established `initial <tier>` anchoring, which exists because the repair-ladder literal decoy-matches bare anchors):

```bash
assert "controller: low escalates to medium" \
  'grep -qiE "initial low[^.]{0,40}(->|→|to)[^.]{0,20}medium" <<<"$ctrl_body"'
assert "controller: medium escalates to high" \
  'grep -qiE "initial medium[^.]{0,40}(->|→|to)[^.]{0,20}high" <<<"$ctrl_body"'
assert "controller: high escalates to max" \
  'grep -qiE "initial high[^.]{0,40}(->|→|to)[^.]{0,20}max" <<<"$ctrl_body"'
assert "controller: max escalation halts" \
  'grep -qiE "initial max[^.]{0,20}(->|→|to)?[^.]{0,20}halt" <<<"$ctrl_body"'
```

Replace the repair-ladder assert:

```bash
assert "controller: repair ladder is high -> max -> halt" \
  'grep -qiE "high[^.]{0,60}max[^.]{0,60}halt" <<<"$ctrl_body"'
```

Replace the README profile assert (near line 464):

```bash
assert "README documents the four profiles" \
  'grep -qF -- "docket-build-low" <<<"$rm_body" && grep -qF -- "docket-build-max" <<<"$rm_body"'
```

Add one file-scoped retirement assert for the two skill bodies:

```bash
# Detect the removed state. The clean break means these tokens must not survive as PROFILE names in
# either contract; anchored on the two skill files rather than a whole-repo grep, because historical
# records under docs/ legitimately keep the old vocabulary.
assert "controller + worker carry no retired profile token" \
  '! grep -qiE "\b(economy|premium)\b" "$CTRL" "$WORKER"'
```

Use whatever variable this file already binds for the controller path (it binds `WORKER="$REPO/skills/docket-build-task/SKILL.md"` near the top; add a matching `CTRL="$REPO/skills/docket-build/SKILL.md"` beside it if one is not already defined — check first, do not shadow an existing binding).

- [ ] **Step 2: Run the test to verify it fails**

Run: `/opt/homebrew/bin/bash tests/test_docket_build.sh`
Expected: FAIL on the new rubric, escalation, repair-ladder, retirement, and README asserts. The wrapper/pin block from Task 1 must still PASS — if it does not, stop and fix Task 1's work before continuing.

- [ ] **Step 3: Rewrite the controller skill**

In `skills/docket-build/SKILL.md`:

Frontmatter `description:` — replace `routing each task to a named economy/standard/premium profile agent` with `routing each task to a named low/medium/high/max profile agent`.

The `## Profiles` section — replace the intro line "Three named agents" with "Four named agents", and replace the table with:

```markdown
| Agent | Use |
|---|---|
| `docket-build-low` | fully specified, pattern-following, no consequential risk, no cross-file reasoning |
| `docket-build-medium` | normal feature, integration, refactor, and debugging work — the default |
| `docket-build-high` | consequential but correctable risk, or a risk the plan names |
| `docket-build-max` | unresolved architecture, or an irreversible data change |
```

Replace the sentence beginning "`premium` means greater reasoning investment" with: "A higher rung means greater reasoning investment, **not** a stronger correctness guarantee — every profile carries identical testing and completion obligations." Leave the rest of that paragraph (model/effort resolution through the generated-agent layer, never restating literal model IDs) byte-identical.

The `## Routing` section — the override example becomes `**Build profile:** low`, and the valid-value sentence becomes: "A valid value (`low`, `medium`, `high`, `max`) is authoritative; record its use in that task's routing line." Keep the invalid-value rule exactly as written.

Replace the classification paragraph and its three bullets with the four-tier rubric, including the organizing principle that makes edge cases resolvable without extending lists:

```markdown
**Otherwise classify**, with a deliberate asymmetry — `low` must be *positively* established,
named risk selects upward, and uncertainty defaults to `medium`.

The `max`/`high` boundary has an organizing principle, not just a list: **`max` is for mistakes this
build's own correction machinery cannot walk back.** An auth bug is serious but patch-correctable and
caught at the suite gate or in review; destroyed data cannot be un-destroyed by a retry, and a wrong
architectural call shapes every task after it. Resolve edge cases by applying that test, not by
extending the lists below.

- **`max`** — **unresolved architecture** or an **irreversible data change** (a destructive
  migration, a backfill, anything that cannot be rolled back). Nothing else classifies here.
  Irreversibility is the test: a reversible or purely additive migration is *not* `max` — it is
  `high`, or `medium` if it carries no consequential risk at all.
- **`high`** — authentication or security boundaries, concurrency or locking, release
  infrastructure, or any consequential risk **explicitly named in the plan or spec text**. That last
  door is honored, not inferred: never articulate a new risk on your own — your classification is
  this closed list, so uncertainty still sinks to `medium`.
- **`medium`** — everything remaining; the default and the uncertainty sink. Deliberately including
  hard-but-safe work: difficulty without consequence stays here, because the plan override covers
  difficulty known at plan time and the `medium -> high` escalation covers difficulty discovered at
  build time.
- **`low`** — *only when* the task is fully specified, follows an established pattern, carries no
  consequential risk, and requires **no cross-file reasoning** — either localized to a couple of
  implementation files (tests do not count against locality), or a mechanical, pattern-identical
  edit repeated across many files whose instances do not interact and where a missed instance fails
  loudly (a grep, a validator) rather than silently. All four conditions must hold; doubt about any
  one of them means `medium`.

`max` is rare by construction: the two-item rubric above, an explicit plan override, and a `high`
escalation are its only three doors.
```

The `## Escalation` fence becomes:

```text
initial low    -> one medium retry
initial medium -> one high retry
initial high   -> one max retry
initial max    -> halt
```

The paragraph after it becomes: "The retry consumes that task's whole escalation allowance: a task that started at `low` and whose `medium` retry still cannot complete **halts** — it does not climb again to `high`."

In `## Halting conditions`, the escalation-exhausted bullet becomes: "**A task's escalation allowance is exhausted** — an initial `max` worker requests escalation, or an escalated worker still cannot finish." The repair bullet becomes: "**The suite is still red after the max repair** — there is no second repair round." The un-dispatchable bullet's example agent name (`docket-build-economy`, in the *Dispatching a task* section's stale-install paragraph) becomes `docket-build-low`.

In `## The build gate`, the red branch's ladder becomes `high -> max -> halt`, and "failure after the premium repair path halts" becomes "failure after the max repair path halts". Keep the rationale sentence's meaning but state it: the repair task is cross-task diagnosis, never routine work, which is why it starts at `high` rather than at the default rung.

- [ ] **Step 4: Update the worker contract**

In `skills/docket-build-task/SKILL.md`, the return-schema line becomes:

```text
PROFILE: <low|medium|high|max> — <one-line routing reason as given to you>
```

Then re-read the file for any other profile-name or "three profiles" reference and correct it — the frontmatter `description:` says "Preloaded into the docket-build profile agents", which needs no change, but verify rather than assume.

- [ ] **Step 5: Run the test to verify it passes**

Run: `/opt/homebrew/bin/bash tests/test_docket_build.sh`
Expected: PASS except the `README documents the four profiles` assert, which stays RED until Task 4 rewrites the README. Confirm that is the **only** remaining failure; anything else is yours to fix here.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-build/SKILL.md skills/docket-build-task/SKILL.md tests/test_docket_build.sh
git commit -m "feat(0184): four-tier routing rubric, escalation ladder, and repair ladder"
```

---

### Task 3: The example config — mirrored rows and the slice anchor

**Files:**
- Modify: `.docket.example.yml`
- Test: `tests/test_docket_example_yml.sh`

**Interfaces:**
- Consumes: the sidecar rows from Task 1 (the example mirrors them value for value).
- Produces: nothing later tasks depend on.

- [ ] **Step 1: Write the failing test**

`tests/test_docket_example_yml.sh` isolates each harness's slice of the example by anchoring the slice **terminator** on that block's `build-premium` model — see the loop that computes `bp_model="$(hd_field "$HD" "$h" build-premium model)"` and `slice="$(ex_slice "$h" "build-premium:.*$(ere_escape "$bp_model")")"`. Repoint every one of those three references from `build-premium` to `build-max`, including the assert titles and the `grep -q "build-premium:"` terminator check:

```bash
  bm_model="$(hd_field "$HD" "$h" build-max model)"
  assert "$h mirror: the sidecar supplies a build-max model to anchor the slice on" '[ -n "$bm_model" ]'
  slice="$(ex_slice "$h" "build-max:.*$(ere_escape "$bm_model")")"
```

and

```bash
  assert "$h mirror: the $h slice was isolated and terminates at its build-max anchor" \
    '[ -n "$slice" ] && [ "$first" = "'"$h"':" ] && grep -q "build-max:" <<<"$last"'
```

Rename the local variable consistently (`bp_model` → `bm_model`) so no stale binding survives. Update the block comment above the loop that explains the anchor — it currently says "Each block's terminator is its own build-premium MODEL" — to name `build-max` and to state **why** it is the terminator: it is the last build row in ladder order, so the anchor moves whenever the ladder's top rung is renamed.

- [ ] **Step 2: Run the test to verify it fails**

Run: `/opt/homebrew/bin/bash tests/test_docket_example_yml.sh`
Expected: FAIL — the `build-max` anchor finds nothing in the example (still mirroring the old three rows), so each slice is empty and the mirror asserts fail.

- [ ] **Step 3: Rewrite the example's mirrored rows**

In `.docket.example.yml`, replace the three build rows at the end of each of the `claude:`, `codex:`, and `cursor:` commented blocks with four, in ladder order and keeping each block's existing comment prefix and column alignment:

```yaml
#     build-low:             { model: claude-sonnet-5,            effort: low }
#     build-medium:          { model: claude-opus-5,              effort: low }
#     build-high:            { model: claude-opus-5,              effort: medium }
#     build-max:             { model: claude-opus-5,              effort: high }
```

```yaml
#     build-low:             { model: gpt-5.6-luna, effort: xhigh }
#     build-medium:          { model: gpt-5.6-terra, effort: medium }
#     build-high:            { model: gpt-5.6-sol, effort: low }
#     build-max:             { model: gpt-5.6-sol, effort: medium }
```

```yaml
#     build-low:             { model: cursor-grok-4.5-low,        effort: auto }
#     build-medium:          { model: cursor-grok-4.5-medium,     effort: auto }
#     build-high:            { model: cursor-grok-4.5-high,       effort: auto }
#     build-max:             { model: claude-opus-5-high,         effort: auto }
```

- [ ] **Step 4: Update the wrapper census comment**

The header comment above the `agents:` block currently reads, in part: "12 wrapper files ship (agents/docket-*.md; …): 5 wrap one of the 5 autonomous skills (adr, auto-groom, finalize-change, implement-next, status); 3 (build-economy, build-standard, build-premium) are docket-build's task workers …". Replace the count and the build-worker clause:

```
# `effort:` key instead keeps the built-in effort — auto and omitted are NOT equivalent. 13 wrapper
# files ship (agents/docket-*.md; their shipped model/effort live in agents/harness-defaults.yml,
# mirrored below): 5 wrap one of the 5 autonomous skills (adr,
# auto-groom, finalize-change, implement-next, status); 4 (build-low, build-medium, build-high,
# build-max) are docket-build's task workers and all preload the same docket-build-task
# worker skill, differing only in model/effort; the other 4 (auto-groom-critic,
```

Also correct the nearby line reading "are complete — every one of the twelve agents carries a model and an effort under each." to "…every one of the thirteen agents…".

- [ ] **Step 5: Run the test to verify it passes**

Run: `/opt/homebrew/bin/bash tests/test_docket_example_yml.sh`
Expected: PASS. Then re-run `/opt/homebrew/bin/bash tests/test_docket_build.sh` — its `no retired profile name survives anywhere in the example` assert (added in Task 1, Step 10) now has real content to check and must be green.

- [ ] **Step 6: Prove the mutation landed**

```bash
grep -c 'build-premium' .docket.example.yml   # expect 0
grep -c 'build-max' .docket.example.yml       # expect 3 — one per harness block
```

- [ ] **Step 7: Commit**

```bash
git add .docket.example.yml tests/test_docket_example_yml.sh
git commit -m "docs(0184): mirror the four-tier build rows in the example config"
```

---

### Task 4: User-facing prose — convention, README, and the harness setup guides

**Files:**
- Modify: `skills/docket-convention/SKILL.md`
- Modify: `README.md`
- Modify: `docs/cursor/validation.md`
- Modify: `docs/codex/setup.md`
- Test: `tests/test_finalize_gate.sh`

**Interfaces:**
- Consumes: the vocabulary fixed in Task 2 and the counts fixed in Task 1.
- Produces: nothing later tasks depend on.

- [ ] **Step 1: Write the failing test**

`tests/test_finalize_gate.sh` carries an assert that reaches into the convention's prose for its wrapper count — a restatement that has quietly become load-bearing in a file about something else entirely. Repoint it at the new count and, per this repo's guard discipline, make it detect the removed state too:

```bash
assert "convention count prose says thirteen wrappers" 'grep -qi "thirteen" "$CONV"'
assert "convention count prose no longer says twelve wrappers" '! grep -qi "twelve" "$CONV"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `/opt/homebrew/bin/bash tests/test_finalize_gate.sh`
Expected: FAIL on both new asserts — the convention still says "twelve".

- [ ] **Step 3: Update the convention**

In `skills/docket-convention/SKILL.md`, two sentences in the *Agent layer* section:

In the paragraph beginning "Each **autonomous** docket skill can run as a model/effort-pinned **subagent**", replace `and `docket-build-task` (shared by its three profile agents)` with `and `docket-build-task` (shared by its four profile agents)`, and replace `and the three `docket-build-*` profile workers` with `and the four `docket-build-*` profile workers`, and the trailing `since that contract is the only thing those three wrappers load` with `…those four wrappers load`.

In the *Composition* paragraph, replace `Four of the **twelve** generated wrappers wrap **no skill**` with `Four of the **thirteen** generated wrappers wrap **no skill**`, and the closing parenthetical `(Twelve wrappers: five wrap the five autonomous skills, three are docket-build's task workers sharing the `docket-build-task` contract, four wrap none.)` with `(Thirteen wrappers: five wrap the five autonomous skills, four are docket-build's task workers sharing the `docket-build-task` contract, four wrap none.)`.

Check `skills/docket-convention/references/agent-layer.md` for any count or profile-name reference and correct it if present — a planning grep found none, but verify rather than assume.

- [ ] **Step 4: Update the README**

Four edits in `README.md`, all in the `docket-build` section:

(a) The sentence "the installer is what generates the three profile agents and links the two build skills onto this machine" → "…generates the four profile agents…".

(b) The paragraph beginning "Each task is routed to one of three named profile agents" — replace the agent list and the classifier description:

```
Each task is routed to one of four named profile agents (`docket-build-low`, `docket-build-medium`, `docket-build-high`, `docket-build-max`), sharing one worker contract and differing only in model and effort. The classifier is deliberately asymmetric: `low` must be *positively* established (fully specified, pattern-following, no consequential risk, no cross-file reasoning), while genuine uncertainty defaults to `medium` rather than the cheaper tier. `max` is deliberately rare — reachable only for unresolved architecture or an irreversible data change, by an explicit plan override, or by escalation from `high` — so the tier meant for extreme cases does not normalize. A plan task can override the classifier outright with a `**Build profile:** low` line on that task; an invalid value halts the build rather than silently falling back. The full routing rubric and worker protocol are the [`docket-build`](skills/docket-build/SKILL.md) skill's, not restated here.
```

(c) The escalation paragraph — replace its first sentence and its closing ladder:

```
Each task carries at most one automatic escalation — a `low` worker retries once at `medium`, a `medium` worker once at `high`, a `high` worker once at `max` — never a second climb; a `max` worker that still can't finish halts the build for a human.
```

and, in the same paragraph, `red becomes one synthetic integration-repair task on the same `standard -> premium -> halt` ladder` → `red becomes one synthetic integration-repair task on the `high -> max -> halt` ladder`.

(d) The shipped-defaults paragraph: `twelve agents each, the three build profiles among them` → `thirteen agents each, the four build profiles among them`. In the delegation bullet further down, `(the twelve generated agents)` → `(the thirteen generated agents)`.

**Do not touch** the sentence mapping docket's `effort:` token `max` to Codex's `model_reasoning_effort` `xhigh` — that is the effort namespace, not a profile name.

While in this section, add one sentence documenting the Haiku override, since a knob is not shipped until it is surfaced — append to the shipped-defaults paragraph:

```
On Claude, `build-low` ships Sonnet rather than Haiku deliberately: the worker contract is long and strict, and its failure modes halt the build instead of escalating. If you want a more cost-aggressive floor, set `build-low` to `claude-haiku-4-5-20251001` in a config layer.
```

- [ ] **Step 5: Update the harness guides**

In `docs/cursor/validation.md`:
- "maps Cursor for **all twelve wrappers**" → "**all thirteen wrappers**".
- "The one deliberate exception is `docket-build-premium`, whose …" → "…is `docket-build-max`, whose …".
- Phase 7's opening, "Docket ships Cursor model IDs for every wrapper, the three build profiles among them" → "…the four build profiles among them".
- Phase 7's step 1, "**Explicit routing, all three profiles.** A task carrying `**Build profile:** economy` lands on `docket-build-economy`; likewise `standard` and `premium` on their own workers. Observable outcome: three dispatches, three distinct agent names…" → "**Explicit routing, all four profiles.** A task carrying `**Build profile:** low` lands on `docket-build-low`; likewise `medium`, `high`, and `max` on their own workers. Observable outcome: four dispatches, four distinct agent names…". Also change the run instruction "on a plan with at least three tasks" to "at least four tasks".

In `docs/codex/setup.md`: "ships a complete twelve-agent `codex:` block, so all twelve `.toml` wrappers" → "…complete thirteen-agent `codex:` block, so all thirteen `.toml` wrappers".

- [ ] **Step 6: Run the tests to verify they pass**

```bash
/opt/homebrew/bin/bash tests/test_finalize_gate.sh
/opt/homebrew/bin/bash tests/test_docket_build.sh
```

Expected: both PASS — including `README documents the four profiles`, which was the last assert left red by Task 2.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/ README.md docs/cursor/validation.md docs/codex/setup.md tests/test_finalize_gate.sh
git commit -m "docs(0184): retier the profile vocabulary across convention, README, and harness guides"
```

---

### Task 5: The retirement guard and the whole-suite gate

The preceding tasks each detect their own removed state within their own files. This task adds the one guard nothing else can give: that **no live surface anywhere** still carries a retired profile name, with the historical record explicitly and visibly exempt.

**Files:**
- Modify: `tests/test_docket_build.sh` (append a new section)

**Interfaces:**
- Consumes: everything from Tasks 1–4.
- Produces: the final green suite.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_build.sh`:

```bash
# ---------------------------------------------------------------------------
# Change 0184: the clean break is repo-wide, and the historical record is exempt
# ---------------------------------------------------------------------------
# A per-file assert cannot see a surface nobody thought to list. This walks the LIVE tree and fails
# on any surviving profile token, with the exemption stated as a path filter rather than an
# allowlist of known files — an allowlist would be an enumerated floor that ages into the gap.
#
# Exempt by design: docs/changes/archive, docs/results, docs/superpowers, docs/adrs. Those record
# what was true when written; rewriting them would falsify the history. This plan's own file lives
# under docs/superpowers/plans and is exempt for the same reason.
live_hits="$(git -C "$REPO" grep -InE 'build-(economy|standard|premium)|docket-build-(economy|standard|premium)' -- \
  ':!docs/changes/archive' ':!docs/results' ':!docs/superpowers' ':!docs/adrs' ':!tests/test_docket_build.sh' || true)"
assert "no live surface names a retired build profile" '[ -z "$live_hits" ]'

# Non-vacuity: the search must be capable of finding something. Run the same pattern WITHOUT the
# exemptions and require hits — the historical record genuinely contains these tokens, so an empty
# result here means the grep itself is broken (bad pathspec, wrong repo root) and the assert above
# passed for the wrong reason.
all_hits="$(git -C "$REPO" grep -IlE 'build-(economy|standard|premium)' || true)"
assert "retirement grep is armed (the historical record still contains the tokens)" \
  '[ -n "$all_hits" ]'
```

Note the `:!tests/test_docket_build.sh` exemption: this file necessarily contains the retired tokens inside the guard's own pattern. That is a deliberate, single-file carve-out — the retirement of the tokens *within this file* is covered by the `controller + worker carry no retired profile token` assert added in Task 2.

- [ ] **Step 2: Run the test to verify it can fail**

Temporarily reintroduce the defect, confirm the mutation actually landed, and watch it redden:

```bash
grep -c 'build-economy' README.md            # expect 0
printf '\n<!-- build-economy -->\n' >> README.md
grep -c 'build-economy' README.md            # expect 1 — the mutation landed
/opt/homebrew/bin/bash tests/test_docket_build.sh   # expect FAIL on the new assert
git checkout -- README.md
grep -c 'build-economy' README.md            # expect 0 again
```

Expected: RED with the mutation, and the failure message naming `README.md`. If it stays green, the pathspec or the repo root is wrong — fix the guard, not the assert's expectation.

- [ ] **Step 3: Run the guard clean**

Run: `/opt/homebrew/bin/bash tests/test_docket_build.sh`
Expected: PASS. If `live_hits` is non-empty, it names a real surface an earlier task missed — go fix that surface rather than widening the exemption list.

- [ ] **Step 4: Run the whole suite**

```bash
for t in tests/test_*.sh; do /opt/homebrew/bin/bash "$t"; done
```

Expected: every file green. Any failure here is a cross-task interaction — fix it in place rather than deferring it to the build gate.

- [ ] **Step 5: Commit**

```bash
git add tests/test_docket_build.sh
git commit -m "test(0184): guard the retired profile names out of every live surface"
```

---

## Merge-gate notes for the results file

Record these at close-out; they are consequences of the clean break that no test can observe.

1. **Re-run `install.sh` and start a fresh session before the next build.** The four profile agents are registered by the harness only at process start, and the machine currently has `docket-build-economy/standard/premium` registered. Until then, a `docket-build` run halts on the harness rejecting a dispatch to `docket-build-medium` — which is the correct, documented behavior, not a defect.
2. **Outer config layers were checked and are clean.** A repo-committed rename cannot reach `~/.config/docket/config.yml` or `.docket.local.yml`; both were inspected during planning and neither sets any `build-*` agent key, so no machine-local override is stranded by this change. Any *other* clone with such an override falls back to shipped defaults until its owner renames the keys.
3. **A pre-existing plan carrying an old `**Build profile:**` token halts** with the existing invalid-value diagnostic. Remedy: edit the plan line. This is the intended clean-break behavior.
4. **The compression claim is review-verified, not machine-verified.** "Every rung below is at or below today's cost" has no oracle in the suite — there is no cost model to assert against. The tests pin the *structural* claims (four distinct pairs, a genuinely different model on the bottom rung, `max` equal to the pre-change premium pin); the cost direction was checked by reading the table.
5. **`cursor-rules/dispatch.head.md` overlaps stub #183.** This change corrects the head's stale "three build-profile workers only … every other wrapper is generated unpinned" claim and re-arms the guard that went vacuous when the Cursor block was completed. #183 should be re-read against this branch before it is groomed.
