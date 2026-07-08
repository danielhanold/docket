# Re-tune default agent models for the Claude 5 lineup — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pin every built-in docket subagent wrapper (and both interactive-skill advisories) to an explicit Claude 5-lineup model ID instead of a bare alias, re-tuning `docket-status` down to Haiku 4.5, so a clone runs the exact model the commit records.

**Architecture:** Pure value edits in existing files — 8 agent frontmatters (`agents/docket-*.md`), 2 advisory prose lines (`skills/docket-{new-change,groom-next}/SKILL.md`), and the assertions in `tests/test_sync_agents.sh` that hardcode the built-in defaults. No new mechanism (that is #0043); bare aliases remain legal *config input* (`.docket.yml`/global override path), so every override test stays unchanged.

**Tech Stack:** Bash test harness (`tests/*.sh`, custom `assert`), markdown frontmatter, Claude Code agent-layer wrappers + `sync-agents.sh` generator.

## Global Constraints

- **Explicit model IDs replace bare aliases in built-in defaults** — `opus` → `claude-opus-4-8`, `sonnet` → `claude-sonnet-5`, and `docket-status` demotes to `claude-haiku-4-5-20251001`. Copy these IDs verbatim.
- **The re-tuned built-in table (authoritative):**
  - `docket-implement-next` → `claude-opus-4-8` / `xhigh`
  - `docket-auto-groom` → `claude-opus-4-8` / `xhigh`
  - `docket-auto-groom-critic` → `claude-opus-4-8` / `xhigh`
  - `docket-integration-repair` → `claude-opus-4-8` / `xhigh`
  - `docket-rebase-resolver` → `claude-opus-4-8` / `xhigh`
  - `docket-adr` → `claude-sonnet-5` / `medium`
  - `docket-finalize-change` → `claude-sonnet-5` / `medium`
  - `docket-status` → `claude-haiku-4-5-20251001` / `medium` (**demoted**)
- **Effort values are unchanged everywhere** — only `model:` lines change (status stays `medium`). No effort re-tuning.
- **Bare aliases remain legal config INPUT** — the `.docket.yml`/global `agents:` override tests (`{ model: haiku }`, `{ model: fable }`, `{ model: sonnet }`) stay valid and unedited; only the shipped *defaults* pin full IDs.
- **Advisories pin the model, keep the framing + effort** — `docket-new-change` → `claude-sonnet-5` (effort: "model default", kept); `docket-groom-next` → `claude-sonnet-5` / `high` (keep `/ high`). Both stay advisory ("the human owns the session").
- **Never restate config-overridable tiers in dispatch prose** (LEARNINGS #17) — this change touches only wrapper frontmatter + advisories; `tests/test_composition_wiring.sh` guards the prose and must stay green (its `(opus|sonnet|haiku|fable)/(effort)` regex cannot match a full ID, so no change is expected there).
- **Reconcile finding — the spec's `test_sync_agents.sh` edit list is a floor, not a ceiling** (LEARNINGS #32). Beyond the alias regex + 5 built-in-table assertions the spec names, four more assertions hardcode the built-in bare aliases and go RED under pinning: L80 (`auto keeps the built-in model`), L81 (`unlisted skill keeps built-in model+effort`), L111 (`critic: model is opus`), L143 (loop `model is opus` for rebase-resolver + integration-repair). All must move to the pinned full IDs.
- **Every new/changed test assertion must be non-vacuous** (LEARNINGS #2) — a pinned-value assertion must flip to NOT OK if the model line is wrong.

---

### Task 1: Build-time gate — confirm agent `model:` frontmatter accepts full model IDs

**Load-bearing.** The whole change assumes Claude Code's agent-layer `model:` field resolves a full ID like `claude-sonnet-5` (not only the `opus|sonnet|haiku|fable` aliases). If it does NOT, explicit pinning is unachievable this way — **abort-and-report and STOP**, do not silently fall back to aliases (that defeats the change).

**Files:** none modified (verification only).

- [ ] **Step 1: Confirm full-ID acceptance authoritatively.**

Dispatch the `claude-code-guide` agent with: "Does Claude Code's subagent frontmatter `model:` field accept a full model ID such as `claude-sonnet-5`, `claude-opus-4-8`, or `claude-haiku-4-5-20251001`, in addition to the short aliases `opus|sonnet|haiku|fable`? I need to pin explicit model IDs in agent definition files (`.claude/agents/*.md`). Cite the docs/behavior." Expected: full model IDs are accepted (the `model` setting and `--model` accept full IDs; agent frontmatter uses the same resolver).

- [ ] **Step 2: Empirical confirmation via a throwaway wrapper.**

Create a scratch agent file pinned to a full ID and dispatch it to confirm no resolver error:

```bash
mkdir -p ~/.claude/agents
cat > ~/.claude/agents/docket-fullid-probe.md <<'EOF'
---
name: docket-fullid-probe
description: throwaway probe confirming full-ID model frontmatter resolves
model: claude-haiku-4-5-20251001
---
Reply with exactly: PROBE-OK
EOF
```

Note: the subagent registry loads at process start, so a brand-new agent type may not be dispatchable mid-session (see the `sync-agents.sh` restart caveat). If a dispatch of `docket-fullid-probe` is not available, treat Step 1 (claude-code-guide, authoritative on the harness) as the gate and record that the registry-reload limitation — not a full-ID rejection — is why the empirical dispatch was skipped. Remove the probe file regardless:

```bash
rm -f ~/.claude/agents/docket-fullid-probe.md
```

- [ ] **Step 3: Decision.**

If full IDs are confirmed accepted → proceed to Task 2. If the harness *rejects* full IDs in `model:` frontmatter → **STOP**: report to the human that explicit pinning is not achievable on this harness and the change should not silently revert to aliases. (No commit either way — verification only.)

---

### Task 2: Pin the 8 built-in wrappers to full IDs (status → Haiku) — TDD via `test_sync_agents.sh`

The built-in-default assertions in `tests/test_sync_agents.sh` are the RED test: they currently pin bare aliases. Update them to the re-tuned table first (RED against the still-bare agents), then pin the agent frontmatters (GREEN).

**Files:**
- Modify: `tests/test_sync_agents.sh` (L26 alias regex; L34–43 built-in table; L80, L81, L111, L143 reconcile-found built-in assertions)
- Modify: `agents/docket-implement-next.md`, `agents/docket-auto-groom.md`, `agents/docket-auto-groom-critic.md`, `agents/docket-integration-repair.md`, `agents/docket-rebase-resolver.md`, `agents/docket-adr.md`, `agents/docket-finalize-change.md`, `agents/docket-status.md` (the `model:` line only)
- Test: `tests/test_sync_agents.sh`

**Interfaces:**
- Consumes: nothing (first code task).
- Produces: agent files whose `model:` is a full ID; test assertions pinned to the re-tuned table. Task 3 relies on `test_sync_agents.sh` running green so its new advisory assertions are the only remaining RED.

- [ ] **Step 1: Relax the alias regex (L26) to also accept full `claude-*` IDs, non-vacuously.**

In `tests/test_sync_agents.sh`, change:

```bash
  assert "$w: model is a known alias" '[[ "$(fm "$f" model)" =~ ^(opus|sonnet|haiku|fable)$ ]]'
```

to (accepts a bare alias OR a full `claude-<name>` ID; still rejects an empty/garbage value):

```bash
  assert "$w: model is a known alias or full id" '[[ "$(fm "$f" model)" =~ ^(opus|sonnet|haiku|fable|claude-[a-z0-9]+(-[a-z0-9]+)*)$ ]]'
```

- [ ] **Step 2: Update the five built-in-table assertions (L34–43) to the re-tuned values.**

```bash
assert "implement-next built-in = claude-opus-4-8/xhigh" \
  '[ "$(fm "$AGENTS/docket-implement-next.md" model)/$(fm "$AGENTS/docket-implement-next.md" effort)" = "claude-opus-4-8/xhigh" ]'
assert "auto-groom built-in = claude-opus-4-8/xhigh" \
  '[ "$(fm "$AGENTS/docket-auto-groom.md" model)/$(fm "$AGENTS/docket-auto-groom.md" effort)" = "claude-opus-4-8/xhigh" ]'
assert "finalize-change built-in = claude-sonnet-5/medium" \
  '[ "$(fm "$AGENTS/docket-finalize-change.md" model)/$(fm "$AGENTS/docket-finalize-change.md" effort)" = "claude-sonnet-5/medium" ]'
assert "status built-in = claude-haiku-4-5-20251001/medium" \
  '[ "$(fm "$AGENTS/docket-status.md" model)/$(fm "$AGENTS/docket-status.md" effort)" = "claude-haiku-4-5-20251001/medium" ]'
assert "adr built-in = claude-sonnet-5/medium" \
  '[ "$(fm "$AGENTS/docket-adr.md" model)/$(fm "$AGENTS/docket-adr.md" effort)" = "claude-sonnet-5/medium" ]'
```

- [ ] **Step 3: Update the four reconcile-found built-in assertions to the pinned IDs.**

L80 (`auto keeps the built-in model` — implement-next built-in via effort-only override):

```bash
assert "auto keeps the built-in model" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-4-8" ]'
```

L81 (`unlisted skill keeps built-in model+effort` — adr built-in):

```bash
assert "unlisted skill keeps built-in model+effort" '[ "$(fm "$SBX/.claude/agents/docket-adr.md" model)/$(fm "$SBX/.claude/agents/docket-adr.md" effort)" = "claude-sonnet-5/medium" ]'
```

L111 (`critic: model is opus`):

```bash
assert "critic: model is claude-opus-4-8" '[ "$(fm "$CRITIC" model)" = "claude-opus-4-8" ]'
```

L143 (loop `$nw: model is opus` — rebase-resolver + integration-repair):

```bash
  assert "$nw: model is claude-opus-4-8" '[ "$(fm "$f" model)" = "claude-opus-4-8" ]'
```

- [ ] **Step 4: Run the test to verify the built-in assertions now FAIL (RED).**

Run: `bash tests/test_sync_agents.sh`
Expected: several `NOT OK` lines for the built-in assertions (agents still carry bare aliases), e.g. `NOT OK - implement-next built-in = claude-opus-4-8/xhigh`, `NOT OK - status built-in = claude-haiku-4-5-20251001/medium`, `NOT OK - critic: model is claude-opus-4-8`. The override/config-input and advisory assertions still print `ok`. Non-zero exit.

- [ ] **Step 5: Pin the eight agent `model:` lines to the re-tuned table.**

Edit each `model:` line (leave `effort:` untouched):

```
agents/docket-implement-next.md    : model: claude-opus-4-8
agents/docket-auto-groom.md        : model: claude-opus-4-8
agents/docket-auto-groom-critic.md : model: claude-opus-4-8
agents/docket-integration-repair.md: model: claude-opus-4-8
agents/docket-rebase-resolver.md   : model: claude-opus-4-8
agents/docket-adr.md               : model: claude-sonnet-5
agents/docket-finalize-change.md   : model: claude-sonnet-5
agents/docket-status.md            : model: claude-haiku-4-5-20251001
```

- [ ] **Step 6: Run the test to verify the built-in assertions now PASS (GREEN).**

Run: `bash tests/test_sync_agents.sh`
Expected: every built-in assertion prints `ok`. The only remaining `NOT OK`, if any, would be Task 3's not-yet-added advisory assertions — none exist yet, so expect a fully green run and exit 0.

- [ ] **Step 7: Commit test + agents together.**

```bash
git add tests/test_sync_agents.sh agents/docket-*.md
git commit -m "feat(0042): pin built-in agent wrappers to explicit Claude 5 model IDs

Pin all 8 built-in wrappers to full model IDs and demote docket-status to
Haiku 4.5. Update test_sync_agents.sh built-in-default assertions (incl. the
four beyond the spec's enumeration: L80/L81/critic/finalize-gate loop);
relax the alias regex to accept full claude-* IDs. Bare aliases remain legal
config input, so override tests are unchanged."
```

---

### Task 3: Pin both interactive-skill advisories to `claude-sonnet-5` — TDD via new assertions

The existing advisory assertions (`grep -qi "sonnet"`, `grep -qiE "sonnet…high"`) are satisfied by both `sonnet` and `claude-sonnet-5`, so they do not discriminate this change. Add two narrow assertions that the advisory prose names the **explicit** ID (RED now, GREEN after the edit); leave the existing asserts intact (they stay green per spec).

**Files:**
- Modify: `tests/test_sync_agents.sh` (add two assertions in the Task 6 advisory block, ~after L212)
- Modify: `skills/docket-new-change/SKILL.md` (the "Recommended model/effort (advisory)" line)
- Modify: `skills/docket-groom-next/SKILL.md` (the "Recommended model/effort (advisory)" line)
- Test: `tests/test_sync_agents.sh`

**Interfaces:**
- Consumes: `test_sync_agents.sh` green from Task 2.
- Produces: advisories naming `claude-sonnet-5`; full green suite.

- [ ] **Step 1: Add the two explicit-ID advisory assertions (RED).**

In `tests/test_sync_agents.sh`, in the `# ---- Task 6: advisory recommendation` block, after the existing `groom-next recommends sonnet/high` assertion, add:

```bash
# Explicit pin (change 0042): the advisory must name the full model ID, not the bare alias.
assert "new-change advisory pins claude-sonnet-5" 'grep -q "claude-sonnet-5" "$NEWC"'
assert "groom-next advisory pins claude-sonnet-5" 'grep -q "claude-sonnet-5" "$GROOM"'
```

- [ ] **Step 2: Run the test to verify the two new assertions FAIL (RED).**

Run: `bash tests/test_sync_agents.sh`
Expected: `NOT OK - new-change advisory pins claude-sonnet-5` and `NOT OK - groom-next advisory pins claude-sonnet-5` (advisories still say bare `sonnet`); everything else `ok`. Non-zero exit.

- [ ] **Step 3: Update the `docket-new-change` advisory line.**

In `skills/docket-new-change/SKILL.md`, change the recommendation from bare `sonnet` to the explicit ID, keeping the advisory framing and the "effort: model default" note. Replace:

```
This skill brainstorms with a human, so it cannot be a fire-and-forget subagent and cannot force the session model. **Recommended: `sonnet`, effort: model default** (wide variance from a trivial stub to a full brainstorm). Set `/model sonnet` to match; this is advisory only — the human owns the session.
```

with:

```
This skill brainstorms with a human, so it cannot be a fire-and-forget subagent and cannot force the session model. **Recommended: `claude-sonnet-5`, effort: model default** (wide variance from a trivial stub to a full brainstorm). Set `/model claude-sonnet-5` to match; this is advisory only — the human owns the session.
```

- [ ] **Step 4: Update the `docket-groom-next` advisory line.**

In `skills/docket-groom-next/SKILL.md`, change the recommendation from bare `sonnet` to the explicit ID, keeping `/ high` and the advisory framing. Replace:

```
This skill grooms interactively with a human, so it cannot be a fire-and-forget subagent and cannot force the session model. **Recommended: `sonnet` / `high`** (the cold-start recap is genuine synthesis). Set `/model sonnet` and `/effort high` to match; this is advisory only — the human owns the session.
```

with:

```
This skill grooms interactively with a human, so it cannot be a fire-and-forget subagent and cannot force the session model. **Recommended: `claude-sonnet-5` / `high`** (the cold-start recap is genuine synthesis). Set `/model claude-sonnet-5` and `/effort high` to match; this is advisory only — the human owns the session.
```

- [ ] **Step 5: Run the test to verify all advisory assertions PASS (GREEN).**

Run: `bash tests/test_sync_agents.sh`
Expected: fully green (exit 0) — including the pre-existing `new-change recommends sonnet` / `groom-next recommends sonnet/high` (both still match, since `claude-sonnet-5` contains `sonnet` and `/ high` is retained) and the two new explicit-ID assertions.

- [ ] **Step 6: Commit.**

```bash
git add tests/test_sync_agents.sh skills/docket-new-change/SKILL.md skills/docket-groom-next/SKILL.md
git commit -m "feat(0042): pin interactive-skill advisories to claude-sonnet-5

Update both advisory recommendation lines from the bare 'sonnet' alias to the
explicit claude-sonnet-5 ID (groom-next keeps / high). Add two non-vacuous
assertions that each advisory names the full ID. Framing stays advisory."
```

---

### Task 4: Whole-suite verification + `sync-agents.sh --check`

Confirm the full test surface is green and the generator's drift gate is satisfied — no partial pin, no composition-wiring regression, no committed project-level agent drift.

**Files:** none modified (verification only; fold any fix back into the owning task).

- [ ] **Step 1: `test_sync_agents.sh` fully green.**

Run: `bash tests/test_sync_agents.sh`
Expected: every line `ok`, exit 0.

- [ ] **Step 2: `test_composition_wiring.sh` still green (no change expected).**

Run: `bash tests/test_composition_wiring.sh`
Expected: green, exit 0 — full IDs cannot match its `(opus|sonnet|haiku|fable)/(low|medium|high|xhigh|max)` regex, so the "pins no literal model/effort tier" asserts are unaffected.

- [ ] **Step 3: Run any repo-wide test runner, if present.**

Run: `ls tests/ && (bash tests/run-all.sh 2>/dev/null || for t in tests/test_*.sh; do echo "== $t =="; bash "$t" || echo "FAILED: $t"; done)`
Expected: no `FAILED:` / no `NOT OK`. Investigate any red before proceeding (a red unrelated suite may be pre-existing — compare against `origin/main` if unsure).

- [ ] **Step 4: `sync-agents.sh --check` drift gate.**

Run: `bash sync-agents.sh --check; echo "rc=$?"`
Expected: `rc=0`. If the docket repo commits project-level `.claude/agents/docket-*.md` files (a `.docket.yml` `agents:` block), pinning the built-ins changes their resolved output; regenerate them with `bash sync-agents.sh` and commit the regenerated files in this task, then re-run `--check` to confirm `rc=0`. If there is no `agents:` block, `--check` passes with nothing to check — no regeneration needed.

- [ ] **Step 5: Commit any regenerated project-level files (only if Step 4 required them).**

```bash
git add .claude/agents/docket-*.md
git commit -m "chore(0042): regenerate committed project-level agent files for pinned defaults"
```

(Skip this commit entirely if Step 4 found nothing to regenerate.)

---

## Self-Review

**Spec coverage:**
- Explicit versions in 8 wrappers → Task 2 (Steps 5, table). ✓
- `status` demotes to Haiku 4.5 → Task 2 (table + L40/L41 assertion). ✓
- 2 advisories → `claude-sonnet-5` → Task 3. ✓
- `test_sync_agents.sh` alias-regex relaxation + built-in assertions → Task 2 (Steps 1–3), incl. the four reconcile-found ones. ✓
- `test_composition_wiring.sh` verify-only stays green → Task 4 Step 2. ✓
- Build-time gate (full-ID acceptance, abort-and-report on rejection) → Task 1. ✓
- Bare aliases remain legal config input (override tests untouched) → Global Constraints + Task 2 leaves L77/L89/L100/L127/L160 unedited. ✓
- Out of scope (no new mechanism, `.docket.yml` example unchanged, TDD build model untouched, no effort re-tune) → nothing in the plan touches those. ✓

**Placeholder scan:** No TBD/TODO/"handle edge cases" — every step shows exact lines, IDs, and commands. ✓

**Type consistency:** Model IDs are byte-identical everywhere: `claude-opus-4-8`, `claude-sonnet-5`, `claude-haiku-4-5-20251001`. Assertion names updated to match their new expected values. ✓

**No ADR:** This applies the existing agent-layer decisions (ADR-0008 lineage / change #0016); it records no new architectural decision. `adrs: []` stays empty.
