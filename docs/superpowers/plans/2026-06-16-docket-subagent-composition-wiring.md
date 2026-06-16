# docket subagent composition — nested status/adr/critic dispatch — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewire docket's three whole-skill sub-invocations to dispatch *named*, model/effort-pinned subagents (foreground, git-state-as-contract) instead of running inline at the parent's model, and add the dedicated `docket-auto-groom-critic` wrapper so the adversarial gate runs in genuine isolation.

**Architecture:** Pure docs/config change — no runtime code. Three surgical skill-body edits re-point existing invocations; one new committed agent wrapper (auto-discovered by `sync-agents.sh`'s `agents/docket-*.md` glob, no generator edit); the convention's forward-pointer becomes the present-tense contract. Tests are bash sentinel/structural assertions (`tests/*.sh`).

**Tech Stack:** Markdown skill bodies (`skills/docket-*/SKILL.md`), YAML-frontmatter agent wrappers (`agents/docket-*.md`), bash test harness (`tests/test_*.sh`), `sync-agents.sh` generator.

---

## File Structure

- **Create** `agents/docket-auto-groom-critic.md` — the sixth wrapper; wraps no skill, loads only `docket-convention`, pinned `opus/xhigh`. Auto-discovered by the generator (glob + `short_name` ⇒ config key `auto-groom-critic`).
- **Create** `tests/test_composition_wiring.sh` — sentinels for the `implement-next` step 0/step 6 rewirings and the convention's present-tense composition contract.
- **Modify** `tests/test_sync_agents.sh` — bump the two `= "5"` wrapper counts to `6` (lines ~17, ~61); add a critic-wrapper structural block + a per-repo `auto-groom-critic` override/`--check` block.
- **Modify** `tests/test_auto_groom.sh` — add an assertion that step 3 names `docket-auto-groom-critic` (keep the existing `"fresh subagent"` + designer→critic→exit order assertions green).
- **Modify** `skills/docket-implement-next/SKILL.md` — step 0 dispatches the `docket-status` subagent; step 6 dispatches the `docket-adr` subagent.
- **Modify** `skills/docket-auto-groom/SKILL.md` — step 3 dispatches the named `docket-auto-groom-critic` subagent (still a *fresh* subagent).
- **Modify** `skills/docket-convention/SKILL.md` — convert the "Composition (built in change 0017)" forward-pointer to the present-tense contract; introduce the sixth wrapper.

Two tasks. Task 1 = the critic wrapper + its generator/structural tests (self-contained). Task 2 = the three skill-body rewirings + convention + their sentinels, built as **one coupled unit** (LEARNINGS #1: don't fragment a tightly-coupled contract across subagents — the wiring, the convention prose, and the sentinels encode one contract).

---

### Task 1: The `docket-auto-groom-critic` wrapper + generator/structural tests

**Files:**
- Modify: `tests/test_sync_agents.sh`
- Create: `agents/docket-auto-groom-critic.md`

- [ ] **Step 1: Bump the two wrapper counts (5 → 6) — failing test**

In `tests/test_sync_agents.sh`, change line ~17 from `= "5"` to `= "6"`:

```bash
assert "exactly 6 built-in wrappers" '[ "$(find "$AGENTS" -maxdepth 1 -name "docket-*.md" | wc -l | tr -d " ")" = "6" ]'
```

And line ~61 from `= "5"` to `= "6"`:

```bash
assert "all 6 wrappers land in .claude/agents" '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "6" ]'
```

- [ ] **Step 2: Add the critic structural + override/check block — failing test**

Append, immediately AFTER the existing `# -- per-repo layer ...` block (after its `rm -rf "$SBX" "$HROOT"` at line ~104) and BEFORE the `# ---- Task 3: --check drift gate` section, this block:

```bash
# ---- Task 1b: the docket-auto-groom-critic wrapper (wraps NO skill) ---------
CRITIC="$AGENTS/docket-auto-groom-critic.md"
assert "critic wrapper exists" '[ -f "$CRITIC" ]'
assert "critic: name matches file" '[ "$(fm "$CRITIC" name)" = "docket-auto-groom-critic" ]'
assert "critic: has a description" '[ -n "$(fm "$CRITIC" description)" ]'
assert "critic: model is opus" '[ "$(fm "$CRITIC" model)" = "opus" ]'
assert "critic: effort is xhigh" '[ "$(fm "$CRITIC" effort)" = "xhigh" ]'
assert "critic: skills injects docket-convention" 'grep -Eq "^skills:.*docket-convention" "$CRITIC"'
# Isolation: the skills: line must NOT pull in the designer skill (would re-inject its bias).
# Scope the check to the skills: line — the name: line legitimately contains "docket-auto-groom".
crit_skills_line="$(grep -E "^skills:" "$CRITIC" || true)"
assert "critic: skills EXCLUDES the docket-auto-groom designer skill" '! grep -q "docket-auto-groom" <<<"$crit_skills_line"'
assert "critic: body carries abort-and-report directive" 'grep -qi "abort-and-report" "$CRITIC"'

# Per-repo override of the critic key (auto-groom-critic) resolves to this wrapper source,
# proving the precedence path + --check drift gate cover the critic.
make_sandbox                                        # SBX = the repo
HROOT2="$(mktemp -d)"; mkdir -p "$HROOT2/.claude"   # separate user-level harness root
printf 'agents:\n  auto-groom-critic: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT2" bash "$SYNC" >/dev/null )
assert "per-repo critic override writes project-level file" '[ -f "$SBX/.claude/agents/docket-auto-groom-critic.md" ]'
assert "per-repo critic override applies model" '[ "$(fm "$SBX/.claude/agents/docket-auto-groom-critic.md" model)" = "sonnet" ]'
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT2" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check passes for in-sync critic (rc=0)" '[ "$chk_rc" = "0" ]'
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-auto-groom-critic.md"; rm -f "$SBX/.claude/agents/docket-auto-groom-critic.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT2" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check flags critic drift (rc!=0)" '[ "$chk_rc" != "0" ]'
rm -rf "$SBX" "$HROOT2"
```

Note: `SYNC`, `AGENTS`, `fm`, `assert`, `make_sandbox` are already defined earlier in the file.

- [ ] **Step 3: Run the suite — verify it FAILS**

Run: `bash tests/test_sync_agents.sh`
Expected: FAIL — `NOT OK - exactly 6 built-in wrappers` (only 5 exist), `NOT OK - critic wrapper exists`, and the per-repo/`--check` critic asserts fail (no source wrapper yet).

- [ ] **Step 4: Create the critic wrapper**

Create `agents/docket-auto-groom-critic.md` with EXACTLY this content. The `description:` scalar must contain no `": "` colon-space (LEARNINGS #5 — unquoted YAML scalars break on colon-space; the em-dash is safe):

```markdown
---
name: docket-auto-groom-critic
description: Adversarial reviewer of an auto-groom draft spec or trivial verdict — attacks it, never improves it, and returns exactly one verdict per the dispatching skill's protocol.
model: opus
effort: xhigh
skills: [docket-convention]
---
You are an adversarial critic of the draft handed to you in your prompt. Attack it; do not defend or improve it. Return exactly one verdict per the dispatching skill's protocol.

You load only `docket-convention` (for vocabulary), never the `docket-auto-groom` designer skill — so you cannot inherit the designer's commit-to-the-conservative-default bias.

You run autonomously with no human to pause and ask: never prompt. If you cannot reach a verdict from the context provided, that IS the "needs human context" verdict (the groom abstains). Treat any unmet precondition or blocking ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
```

- [ ] **Step 5: Run the suite — verify it PASSES**

Run: `bash tests/test_sync_agents.sh`
Expected: every line `ok - ...`, exit 0. Specifically the count, critic structural, per-repo critic override, and critic `--check` drift assertions all pass.

- [ ] **Step 6: Commit**

```bash
git add agents/docket-auto-groom-critic.md tests/test_sync_agents.sh
git commit -m "feat(0017): add docket-auto-groom-critic wrapper; bump sync-agents wrapper count 5->6"
```

---

### Task 2: Rewire the three call sites + the convention, guarded by sentinels

Built as one coupled unit. Write the failing sentinels first, then make all three skill-body edits, then prove the whole suite (including the existing `test_auto_groom.sh` invariants) stays green.

**Files:**
- Create: `tests/test_composition_wiring.sh`
- Modify: `tests/test_auto_groom.sh`
- Modify: `skills/docket-implement-next/SKILL.md`
- Modify: `skills/docket-auto-groom/SKILL.md`
- Modify: `skills/docket-convention/SKILL.md`

- [ ] **Step 1: Write the new sentinel test — failing**

Create `tests/test_composition_wiring.sh` with EXACTLY this content:

```bash
#!/usr/bin/env bash
# tests/test_composition_wiring.sh — guards change 0017 (subagent composition wiring):
#   - implement-next step 0 dispatches the docket-status subagent
#   - implement-next step 6 dispatches the docket-adr subagent
#   - docket-convention's Composition section is the present-tense contract (no forward-pointer),
#     still references 0017, names the docket-auto-groom-critic wrapper, and states the isolation
# Sentinels are sampling, not parsing (LEARNINGS #5/#13) — pair with the whole-branch review.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

IMPL="$REPO/skills/docket-implement-next/SKILL.md"
CONV="$REPO/skills/docket-convention/SKILL.md"

# --- implement-next: the two dispatch sites ---
assert "implement-next step 0 dispatches the docket-status subagent" \
  'grep -Eqi "dispatch the .?docket-status.? subagent" "$IMPL"'
assert "implement-next step 6 dispatches the docket-adr subagent" \
  'grep -Eqi "dispatch the .?docket-adr.? subagent" "$IMPL"'

# --- convention: present-tense composition contract ---
# Non-vacuous: the forward-pointer wording must be GONE (deleting the conversion flips this red).
assert "convention: composition is present-tense (no 'will spawn')" '! grep -qi "will spawn" "$CONV"'
assert "convention: composition has no 'Until 0017 lands' forward-pointer" '! grep -qi "Until 0017 lands" "$CONV"'
assert "convention: composition still references change 0017" 'grep -q "0017" "$CONV"'
assert "convention: composition names the docket-auto-groom-critic wrapper" 'grep -qF "docket-auto-groom-critic" "$CONV"'
assert "convention: critic wraps no skill" 'grep -qi "no skill" "$CONV"'
assert "convention: critic loads only docket-convention" 'grep -Eqi "only .?docket-convention" "$CONV"'

exit $fail
```

- [ ] **Step 2: Add the auto-groom critic-naming assertion — failing**

In `tests/test_auto_groom.sh`, immediately AFTER the existing block:

```bash
assert "auto-groom: critic is a fresh subagent, not the designer" \
  'grep -qF "fresh subagent" "$AG"'
```

add:

```bash
assert "auto-groom: step 3 dispatches the named docket-auto-groom-critic subagent" \
  'grep -qF "docket-auto-groom-critic" "$AG"'
```

(Leave the existing `"fresh subagent"` and the designer→critic→exit order assertions untouched — both stay true after the edit.)

- [ ] **Step 3: Run both tests — verify they FAIL**

Run: `bash tests/test_composition_wiring.sh; bash tests/test_auto_groom.sh`
Expected:
- `test_composition_wiring.sh`: FAIL — `NOT OK` for the two implement-next dispatch asserts, `NOT OK - convention: composition is present-tense (no 'will spawn')` (the forward-pointer still says "will spawn"), and the critic/`no skill` asserts.
- `test_auto_groom.sh`: FAIL — `NOT OK - auto-groom: step 3 dispatches the named docket-auto-groom-critic subagent` (not named yet). All OTHER `test_auto_groom.sh` lines still `ok`.

- [ ] **Step 4: Rewire `implement-next` step 0**

In `skills/docket-implement-next/SKILL.md`, in the Step 0 paragraph, replace:

```
then invoke `docket-status` (whose merge-sweep pass archives any `implemented` change whose PR has merged) before selection — the self-cleaning safety net for changes not closed via `docket-finalize-change`.
```

with:

```
then **dispatch the `docket-status` subagent** (foreground — the parent suspends until it returns; pinned sonnet/medium via its wrapper), whose merge-sweep pass archives any `implemented` change whose PR has merged, before selection — the self-cleaning safety net for changes not closed via `docket-finalize-change`. The dispatch is **unconditional** (baked into the skill body, so it runs at sonnet/medium whether this skill was invoked as its own wrapper subagent or inline) and its effects are commits on `origin/docket`; the `.docket/` re-sync this step already performs before selection surfaces the swept state — the contract is **git state, not an in-context return**.
```

- [ ] **Step 5: Rewire `implement-next` step 6**

In the same file, in the Step 6 paragraph, replace:

```
For any non-obvious decision made during implementation, invoke `docket-adr` to record it (it assigns the number + updates the index); append the returned number to the change's `adrs:`.
```

with:

```
For any non-obvious decision made during implementation, **dispatch the `docket-adr` subagent** (foreground, pinned sonnet/medium via its wrapper) to record it (it assigns the number + updates the index) — once per decision; it commits the ADR on `origin/docket`, publishes it onto the integration branch on acceptance, and **returns the number**. After re-syncing `.docket/`, append that returned number to the change's `adrs:`.
```

- [ ] **Step 6: Rewire `auto-groom` step 3 (name the critic, keep "fresh subagent")**

In `skills/docket-auto-groom/SKILL.md`, in the Step 3 — Critic pass paragraph, replace:

```
Dispatch a **fresh subagent** (never the designer reviewing itself) to adversarially attack the draft — specs and trivial verdicts alike.
```

with:

```
Dispatch the dedicated **`docket-auto-groom-critic`** subagent (foreground, pinned opus/xhigh via its wrapper) — a fresh subagent (never the designer reviewing itself), isolated in its own context, loading only `docket-convention` and never this designer skill — to adversarially attack the draft — specs and trivial verdicts alike.
```

(Keeping "fresh subagent" is honest, not a contortion: the named critic genuinely IS a fresh, isolated subagent — naming it does not make it less fresh. It also keeps the existing `test_auto_groom.sh` assertion green.)

- [ ] **Step 7: Convert the convention's Composition paragraph to present tense**

In `skills/docket-convention/SKILL.md`, replace the entire paragraph that begins `**Composition (built in change 0017).**` with:

```
**Composition (change 0017).** Nesting lets each whole-skill sub-invocation run at its own model. `docket-implement-next` **dispatches the `docket-status` subagent** at step 0 (sonnet/medium) and the **`docket-adr` subagent** at step 6 (sonnet/medium); `docket-auto-groom` **dispatches the dedicated `docket-auto-groom-critic` subagent** (opus/xhigh) for its adversarial gate. All three are **foreground** (the parent suspends until the child returns) and **unconditional** (baked into the skill body, so the sub-call gets its own model whether the parent ran as its wrapper subagent or as a plain inline skill); the contract is **git state** on `origin/docket` (and, for adr, a published ADR on the integration branch), re-read after a re-sync — never an in-context return. The critic is a **sixth generated wrapper** (`agents/docket-auto-groom-critic.md`, config key `auto-groom-critic`) that wraps **no skill** — it loads only `docket-convention`, never the `docket-auto-groom` designer body, so the adversary cannot inherit the designer's commit-to-the-default bias. (The "Agent layer" line above stays exact: five *skills* get a wrapper; this sixth wrapper is attached to `auto-groom` and wraps no skill.)
```

- [ ] **Step 8: Sweep for stale wrapper-count language (LEARNINGS #5/#14)**

The wrapper set grew 5 → 6. Grep the repo for stale count words in NON-immutable files (ADR-0008 is immutable — its context shift is handled by the dated `## Update` at review step 6, never a body edit):

Run:
```bash
grep -rniE "five wrappers|5 wrappers|six wrappers|6 wrappers|exactly (five|5) " \
  README.md skills/ docs/changes/ 2>/dev/null
```
Expected: no hit implies "five wrappers"-style prose is absent (the convention says "Five *skills* get a wrapper", which stays correct — five skills do). If any hit refers to the *wrapper-file* count (not the skill count), update it to read "six wrappers (five skill wrappers + the `docket-auto-groom-critic` critic)". Do NOT touch `docs/adrs/`.

- [ ] **Step 9: Run the full affected test set — verify GREEN**

Run:
```bash
bash tests/test_composition_wiring.sh
bash tests/test_auto_groom.sh
bash tests/test_sync_agents.sh
bash tests/test_convention_extraction.sh
```
Expected: all four exit 0, every line `ok - ...`. In particular:
- `test_composition_wiring.sh`: all green (both dispatches present; convention present-tense, references 0017, names the critic, states isolation).
- `test_auto_groom.sh`: green — the new critic-naming assert passes AND the existing `"fresh subagent"` + designer→critic→exit order asserts still pass.
- `test_sync_agents.sh`: still green (the `grep -q "0017"` convention assert still passes — the present-tense text keeps "0017").
- `test_convention_extraction.sh`: still green (no count/agent-section assert broken).

- [ ] **Step 10: Commit**

```bash
git add skills/docket-implement-next/SKILL.md skills/docket-auto-groom/SKILL.md \
        skills/docket-convention/SKILL.md tests/test_composition_wiring.sh tests/test_auto_groom.sh
git commit -m "feat(0017): dispatch named subagents for status/adr/critic; present-tense composition contract"
```

---

## Notes for the build loop (not build tasks)

- **ADR (implement-next step 6).** Per spec §8, record the composition decision via the `docket-adr` subagent at review time. Lean: a dated `## Update` to **ADR-0008** for "composition landed (foreground + git-as-contract)", plus possibly a **new ADR** for the genuinely-new critic-isolation decision (critic loads only the convention, never the designer skill, to prevent self-agreement). Decide at build; append the number(s) to `adrs:` in the metadata working tree.
- **No `sync-agents.sh` edit.** The `agents/docket-*.md` glob + `short_name` auto-discover the critic (config key `auto-groom-critic`) — verified at reconcile (spec §9). The only mechanical must-do is the 5→6 count bump (Task 1).
- **The running skills are unaffected mid-build.** Edits land in the feature worktree; the installed `~/.claude/skills` symlinks point at the primary `main` worktree, so this run's loaded skills do not change under it.

## Self-review

- **Spec coverage:** §2 three call sites → Task 2 steps 4/5/6; §4 critic wrapper → Task 1 step 4; §5 rewiring table → Task 2; §6 convention → Task 2 step 7; §7 tests (count bump + critic asserts; rewiring sentinels) → Task 1 + Task 2 steps 1/2; §8 ADR → Notes. All covered.
- **Placeholder scan:** every step has exact paths, exact old/new text, exact commands + expected output. No TBD/TODO.
- **Type/name consistency:** wrapper file `agents/docket-auto-groom-critic.md`, `name: docket-auto-groom-critic`, config key `auto-groom-critic`, skill phrases "dispatch the `docket-status` subagent" / "dispatch the `docket-adr` subagent" — used identically in the skill edits and the sentinels.
```
