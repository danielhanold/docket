# finalize — rebase-onto-base + re-run-tests gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a rebase-onto-base + re-run-tests gate to `docket-finalize-change`'s merge step so a behind-base PR cannot land a semantically-broken integration branch, with two pinned judgment-tier subagents that resolve rebase conflicts and repair red tests, and a sign-off rule so auto-authored repairs never merge unseen.

**Architecture:** docket is a markdown-and-bash convention system. The "code" here is (a) two new agent **wrapper** files (`agents/docket-*.md`, frontmatter + a short charter, auto-discovered by `sync-agents.sh`'s glob — no generator edit), (b) **skill prose** (`skills/*/SKILL.md`) the agent follows, (c) one **config** file (`.docket.yml`), and (d) **shell sentinel/structural tests** (`tests/test_*.sh`) that assert the prose/wrappers/config say what they must. Tests are standalone: `bash tests/test_<name>.sh` prints `ok -`/`NOT OK -` lines and exits non-zero on any failure. TDD here = write the failing sentinel/structural assert, watch it fail, write the prose/wrapper/config to satisfy it.

**Tech Stack:** Bash (POSIX-ish, `set -euo pipefail` in scripts; `set -uo pipefail` in tests), awk/sed for YAML scalar/block parsing, Markdown skills + YAML frontmatter, `git` + `gh`.

**Key invariants pulled from the spec + LEARNINGS (read before starting):**
- The two new wrappers **wrap no skill** (like `agents/docket-auto-groom-critic.md`): `skills: [docket-convention]` only, pinned `opus`/`xhigh`, carry an abort-and-report directive. Config keys are `rebase-resolver` and `integration-repair` (the `docket-` prefix is stripped by `short_name`).
- **Stale-count trap (LEARNINGS 2026-06-12 #14 / 2026-06-10 #5):** wrappers go **6 → 8** (5 skill-wrappers + critic + these two). The exact spots are `tests/test_sync_agents.sh:17` and `:61` (`= "6"`) and the `skills/docket-convention/SKILL.md` "sixth generated wrapper" prose at line ~60. The line **"Five skills get a wrapper"** (convention line ~37) and the parenthetical **"five *skills* get a wrapper"** stay verbatim — these wrap no skill. README's "eight skills" and the convention's "seven operating skills" are about **skills** (none added) — do **not** touch. `docs/results/*` are immutable historical artifacts — do **not** touch.
- **Don't restate model/effort literals in dispatch prose (LEARNINGS 2026-06-17 #17):** the finalize SKILL body must dispatch the two agents by name + "at the model/effort its wrapper resolves" — never the literal `opus`/`xhigh`. A regex assert guards this.
- **SIGPIPE-safe shell (LEARNINGS 2026-06-16 #11/#16):** never `producer | grep -q` / `producer | head` under `pipefail`; capture into a var, then `grep`/`head -n1 <<<"$var"`. Use `--force-with-lease` for the gate's force-push.
- **Prove every assert non-vacuous (LEARNINGS 2026-06-04 #2):** deleting the clause an assert guards must flip it to `NOT OK`. Sentinels are sampling, not parsing — the whole-branch review (step 6) is the real net (LEARNINGS 2026-06-10 #5).
- **YAML frontmatter scalars can't contain `": "` unquoted (LEARNINGS 2026-06-10 #5):** keep `description:` lines em-dash/comma separated, no colon-space.

---

## File Structure

| File | Responsibility | Create/Modify |
|------|----------------|---------------|
| `agents/docket-rebase-resolver.md` | Wrapper ①: resolve rebase conflicts during the gate's rebase; never tests | **Create** |
| `agents/docket-integration-repair.md` | Wrapper ②: make the suite pass after the rebase; ≤2 attempts; sign-off-gated | **Create** |
| `tests/test_sync_agents.sh` | Wrapper count 6→8 + structural asserts for the two no-skill wrappers | Modify (`:17`, `:61`, append Task-1c block) |
| `skills/docket-finalize-change/SKILL.md` | The gate flow inside the merge step, the two dispatches, §6 sign-off, §7 abort-and-report set | Modify (step 1 + new gate section) |
| `.docket.yml` | Example `finalize:` block + docket's own `gate: local` (dogfood) | Modify |
| `tests/test_finalize_gate.sh` | Sentinels for the gate mechanics, config-parse for the 4 modes + off path, no-tier-literal guard, convention/status doc sentinels | **Create** |
| `skills/docket-convention/SKILL.md` | Document `finalize.gate`/`test_command`; extend Agent-layer + Composition for the two wrappers; bump count prose 6→8 | Modify |
| `skills/docket-status/SKILL.md` | One-line "the gate is finalize-only; the sweep only archives already-merged PRs" note | Modify |

---

## Task 1: Two no-skill agent wrappers + sync-agents count/structural tests

**Files:**
- Create: `agents/docket-rebase-resolver.md`
- Create: `agents/docket-integration-repair.md`
- Modify: `tests/test_sync_agents.sh` (lines 17 and 61; append a "Task 1c" block after the Task-1b critic block, before `# ---- Task 3:`)

- [ ] **Step 1: Update the wrapper-count asserts to 8 and add the structural block (RED)**

In `tests/test_sync_agents.sh`, change line 17 from:
```bash
assert "exactly 6 built-in wrappers" '[ "$(find "$AGENTS" -maxdepth 1 -name "docket-*.md" | wc -l | tr -d " ")" = "6" ]'
```
to:
```bash
assert "exactly 8 built-in wrappers" '[ "$(find "$AGENTS" -maxdepth 1 -name "docket-*.md" | wc -l | tr -d " ")" = "8" ]'
```

Change line 61 from:
```bash
assert "all 6 wrappers land in .claude/agents" '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "6" ]'
```
to:
```bash
assert "all 8 wrappers land in .claude/agents" '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "8" ]'
```

Then add this block immediately after the Task-1b critic block (after the `rm -rf "$SBX" "$HROOT2"` that closes the critic per-repo-override fixture, around line 133), before `# ---- Task 3:`:
```bash
# ---- Task 1c: the two finalize-gate wrappers (wrap NO skill) ----------------
# docket-rebase-resolver (①) and docket-integration-repair (②): like the critic,
# they inject ONLY docket-convention, pin opus/xhigh, and carry abort-and-report.
for nw in docket-rebase-resolver docket-integration-repair; do
  f="$AGENTS/$nw.md"
  assert "$nw: wrapper exists" '[ -f "$f" ]'
  assert "$nw: name matches file" '[ "$(fm "$f" name)" = "$nw" ]'
  assert "$nw: has a description" '[ -n "$(fm "$f" description)" ]'
  assert "$nw: model is opus" '[ "$(fm "$f" model)" = "opus" ]'
  assert "$nw: effort is xhigh" '[ "$(fm "$f" effort)" = "xhigh" ]'
  assert "$nw: skills injects docket-convention" 'grep -Eq "^skills:.*docket-convention" "$f"'
  # Isolation: the skills: line wraps NO docket skill (only the convention).
  nw_skills_line="$(grep -E "^skills:" "$f" || true)"
  assert "$nw: skills EXCLUDES any wrapped docket skill" \
    '! grep -Eq "docket-(finalize-change|implement-next|auto-groom|status|adr|groom-next|new-change)" <<<"$nw_skills_line"'
  assert "$nw: body carries abort-and-report directive" 'grep -qi "abort-and-report" "$f"'
done

# Per-repo override of a new key (rebase-resolver) resolves to its wrapper source,
# proving the precedence path + --check drift gate cover the new wrappers.
make_sandbox                                        # SBX = the repo
HROOT3="$(mktemp -d)"; mkdir -p "$HROOT3/.claude"   # separate user-level harness root
printf 'agents:\n  rebase-resolver: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT3" bash "$SYNC" >/dev/null )
assert "per-repo rebase-resolver override writes project-level file" '[ -f "$SBX/.claude/agents/docket-rebase-resolver.md" ]'
assert "per-repo rebase-resolver override applies model" '[ "$(fm "$SBX/.claude/agents/docket-rebase-resolver.md" model)" = "sonnet" ]'
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT3" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check passes for in-sync rebase-resolver (rc=0)" '[ "$chk_rc" = "0" ]'
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-rebase-resolver.md"; rm -f "$SBX/.claude/agents/docket-rebase-resolver.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT3" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check flags rebase-resolver drift (rc!=0)" '[ "$chk_rc" != "0" ]'
rm -rf "$SBX" "$HROOT3"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_sync_agents.sh`
Expected: FAIL — `NOT OK - exactly 8 built-in wrappers` (6 found), `NOT OK - all 8 wrappers land in .claude/agents`, and every `docket-rebase-resolver`/`docket-integration-repair` assert `NOT OK` (files don't exist). Non-zero exit.

- [ ] **Step 3: Create `agents/docket-rebase-resolver.md` (①)**

```markdown
---
name: docket-rebase-resolver
description: Resolves rebase conflicts during finalize's rebase-onto-base gate — reconciles each conflicted hunk by merge intent and continues the rebase to completion; never runs tests.
model: opus
effort: xhigh
skills: [docket-convention]
---
You resolve the conflicts of an in-progress `git rebase` of a feature branch onto its integration base, handed to you by `docket-finalize-change`'s merge gate. You load only `docket-convention` for vocabulary — you wrap no skill.

Charter: for each conflicted hunk, reconcile it with merge-intent judgment — work out what base changed and what the PR intends, then keep one side or synthesize both. `git add` the resolved paths and `git rebase --continue` through every conflicted commit until the rebase completes. Confine edits to the conflicted regions. You do NOT run tests — making the suite pass after the rebase lands is the integration-repair agent's job, not yours.

Report your work as conflicts resolved, never an authored repair — pure conflict resolution completes the merge the human already intended and does not trigger the gate's auto-repair sign-off.

You run autonomously with no human to pause and ask: never prompt. When a conflict is genuinely ambiguous — you cannot tell which intent is correct without guessing — treat it as abort-and-report: run `git rebase --abort`, stop, and surface exactly which hunk blocked you and why.
```

- [ ] **Step 4: Create `agents/docket-integration-repair.md` (②)**

```markdown
---
name: docket-integration-repair
description: Makes the test suite pass after finalize's rebase lands — root-causes the red tests, writes a minimal fix in at most two attempts, never weakens tests, and reports an authored repair the dispatcher gates behind sign-off.
model: opus
effort: xhigh
skills: [docket-convention]
---
You make the test suite pass after `docket-finalize-change` has rebased a feature branch onto its integration base and the suite came up red. You load only `docket-convention` for vocabulary — you wrap no skill.

Charter: own every red-test outcome regardless of cause — genuine base drift, or a bad conflict resolution you can see in the git state. Apply systematic-debugging discipline: find the root cause, write a MINIMAL fix, never game or weaken the tests, then re-run the suite. You are bounded to at most two repair attempts.

Because your output is code the human's PR review never saw, a successful repair must never merge unseen: report it as an authored repair, including the diff and a plain account of what broke and how you fixed it. The dispatching skill gates the merge on that report — interactive sign-off, or autonomous abort-and-report.

You run autonomously with no human to pause and ask: never prompt. If you cannot reach green within two attempts, treat it as abort-and-report: stop and surface your diagnosis — what is still failing, your hypothesis, and what you tried.
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_sync_agents.sh`
Expected: PASS — all `ok -`, including `ok - exactly 8 built-in wrappers`, `ok - all 8 wrappers land in .claude/agents`, and every Task-1c assert. Exit 0.

- [ ] **Step 6: Sanity-check the generator picks up the new wrappers**

Run: `bash -c 'D=$(mktemp -d); mkdir -p "$D/.claude"; DOCKET_HARNESS_ROOT="$D" bash sync-agents.sh >/dev/null; ls "$D/.claude/agents" | grep -E "rebase-resolver|integration-repair"; rm -rf "$D"'`
Expected: prints `docket-integration-repair.md` and `docket-rebase-resolver.md` — confirms the `agents/docket-*.md` glob auto-discovers them with no generator edit.

- [ ] **Step 7: Commit**

```bash
git add agents/docket-rebase-resolver.md agents/docket-integration-repair.md tests/test_sync_agents.sh
git commit -m "feat(0015): add rebase-resolver + integration-repair wrappers; wrappers 6->8"
```

---

## Task 2: The merge gate in finalize + config + finalize-gate tests

**Files:**
- Create: `tests/test_finalize_gate.sh`
- Modify: `skills/docket-finalize-change/SKILL.md` (step 1 merge action + a new gate section after *Per-change steps*)
- Modify: `.docket.yml` (add a `finalize:` block; set docket's own `gate: local`)

- [ ] **Step 1: Write `tests/test_finalize_gate.sh` (RED)**

Create the file exactly:
```bash
#!/usr/bin/env bash
# tests/test_finalize_gate.sh — run: bash tests/test_finalize_gate.sh
# Sentinels for the finalize rebase-retest merge gate (change 0015). Sentinels are
# sampling, not parsing — paired with the whole-branch review. Each assert is written
# to flip to NOT OK if the clause it guards is removed (non-vacuous).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

FIN="$REPO/skills/docket-finalize-change/SKILL.md"
CONV="$REPO/skills/docket-convention/SKILL.md"
STAT="$REPO/skills/docket-status/SKILL.md"
DYML="$REPO/.docket.yml"

# ---- Config parse: the nested finalize.gate key, four modes + default ----------
# Block-scoped awk (the sync-agents.sh idiom), SIGPIPE-safe (capture, no producer|grep).
# Default is `local` (gate on by default); `off` is the documented opt-out.
gate_of(){  # $1 = path to a .docket.yml
  local v
  v="$(awk '
    /^finalize:[[:space:]]*$/{f=1;next}
    f&&/^[^[:space:]#]/{f=0}
    f&&/^[[:space:]]+gate[[:space:]]*:/{
      line=$0; sub(/#.*/,"",line); sub(/.*gate[[:space:]]*:[[:space:]]*/,"",line);
      gsub(/[[:space:]]/,"",line); print line; exit
    }' "$1" 2>/dev/null)"
  printf '%s' "${v:-local}"
}
TMPC="$(mktemp -d)"
printf 'finalize:\n  gate: local\n'  > "$TMPC/local.yml"
printf 'finalize:\n  gate: ci\n'     > "$TMPC/ci.yml"
printf 'finalize:\n  gate: both\n'   > "$TMPC/both.yml"
printf 'finalize:\n  gate: off\n'    > "$TMPC/off.yml"
printf 'metadata_branch: docket\n'   > "$TMPC/absent.yml"   # no finalize: block
assert "config-parse: gate local"            '[ "$(gate_of "$TMPC/local.yml")" = "local" ]'
assert "config-parse: gate ci"               '[ "$(gate_of "$TMPC/ci.yml")"    = "ci" ]'
assert "config-parse: gate both"             '[ "$(gate_of "$TMPC/both.yml")"  = "both" ]'
assert "config-parse: gate off (opt-out)"    '[ "$(gate_of "$TMPC/off.yml")"   = "off" ]'
assert "config-parse: absent block => local" '[ "$(gate_of "$TMPC/absent.yml")" = "local" ]'
rm -rf "$TMPC"

# ---- finalize SKILL gates on finalize.gate ------------------------------------
assert "finalize references the finalize.gate config" 'grep -Eq "finalize\.gate|finalize:" "$FIN"'
assert "finalize names all four gate modes" \
  'grep -q "local" "$FIN" && grep -q "ci" "$FIN" && grep -q "both" "$FIN" && grep -qE "\boff\b" "$FIN"'
assert "finalize: off restores today's no-rebase behavior" 'grep -Eqi "off[^.]*(today|no rebase|no re-test|trust)" "$FIN"'

# ---- dispatches the two agents at the right triggers --------------------------
assert "finalize dispatches docket-rebase-resolver on conflict" 'grep -q "docket-rebase-resolver" "$FIN"'
assert "rebase-resolver dispatch is tied to a rebase conflict" \
  'grep -Eqi "conflict[^.]*docket-rebase-resolver|docket-rebase-resolver[^.]*conflict" "$FIN"'
assert "finalize dispatches docket-integration-repair on red tests" 'grep -q "docket-integration-repair" "$FIN"'
assert "integration-repair dispatch is tied to a red/failed suite" \
  'grep -Eqi "(red|fail)[^.]*docket-integration-repair|docket-integration-repair[^.]*(red|fail)" "$FIN"'

# ---- local validation runs BEFORE the force-push (ordering is the contract) ----
assert "finalize force-pushes with --force-with-lease" 'grep -q "force-with-lease" "$FIN"'
local_ln="$(grep -nEi "run the suite|validate|local" "$FIN" | grep -i "before" | head -n1 | cut -d: -f1)"
push_ln="$(grep -ni "force-with-lease" "$FIN" | head -n1 | cut -d: -f1)"
assert "finalize states local validation precedes the push" '[ -n "$local_ln" ] && [ -n "$push_ln" ] && [ "$local_ln" -lt "$push_ln" ]'

# ---- §6 sign-off: interactive prompt vs autonomous abort-and-report -----------
assert "finalize documents repair sign-off" 'grep -qi "sign-off" "$FIN"'
assert "finalize: interactive sign-off prompts before merge" 'grep -Eqi "interactive[^.]*(prompt|sign-off)" "$FIN"'
assert "finalize: autonomous repair aborts-and-reports" 'grep -Eqi "autonomous[^.]*abort-and-report" "$FIN"'

# ---- §7 abort-and-report set (the full list of stop points) -------------------
ab="$(grep -ci "abort-and-report" "$FIN")"
assert "finalize names abort-and-report multiple times" '[ "$ab" -ge 3 ]'
assert "abort path: ambiguous rebase conflict"     'grep -Eqi "ambiguous[^.]*conflict|conflict[^.]*ambiguous" "$FIN"'
assert "abort path: no detectable test suite"      'grep -Eqi "no[^.]*suite|suite[^.]*not[^.]*found|no[^.]*test_command" "$FIN"'
assert "abort path: cannot reach green in <=2"      'grep -Eqi "two attempts|<=2|cannot reach green|stuck" "$FIN"'
assert "abort path: force-with-lease rejected"      'grep -Eqi "lease[^.]*reject|reject[^.]*lease|concurrent push" "$FIN"'

# ---- LEARNINGS #17: no model/effort literal in the dispatch prose -------------
assert "finalize body restates NO model alias literal" '! grep -qiE "\b(opus|sonnet|haiku|fable)\b" "$FIN"'
assert "finalize body restates NO effort literal" '! grep -qiE "\bxhigh\b" "$FIN"'
assert "finalize names the wrapper as the tier source" 'grep -Eqi "model/effort its wrapper resolves|its wrapper resolves" "$FIN"'

# ---- docket repo dogfoods the gate -------------------------------------------
assert "repo .docket.yml sets finalize gate to local" '[ "$(gate_of "$DYML")" = "local" ]'

exit $fail
```
(The `CONV`/`STAT` vars are declared here but exercised by Tasks 3 and 4, which append their asserts to this same file.)

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_finalize_gate.sh`
Expected: the `config-parse:` asserts PASS (the helper is self-contained), but every `finalize …` assert is `NOT OK` (the SKILL has no gate yet) and `repo .docket.yml sets finalize gate to local` is `NOT OK` (no `finalize:` block yet). Non-zero exit.

- [ ] **Step 3: Rewrite the finalize merge step (step 1) to invoke the gate**

In `skills/docket-finalize-change/SKILL.md`, replace the parenthetical at the end of step 1 (the sentence beginning "(Merging the PR is the only thing…") so the merge is preceded by the gate. Change step 1's final lines from:
```
… under **auto-detect**, PROMPT first per the Selection rules above before merging. Then continue. (Merging the PR is the only thing that lands plan + results + code on the integration branch — they ride the merge, not the terminal-publish.)
```
to:
```
… under **auto-detect**, PROMPT first per the Selection rules above before merging. **Before the merge lands, run *The rebase-retest merge gate* below** (unless `finalize.gate` is `off`) — it brings the feature branch up to base, validates the integrated result, and only then proceeds to `gh pr merge`. Then continue. (Merging the PR is the only thing that lands plan + results + code on the integration branch — they ride the merge, not the terminal-publish.)
```

- [ ] **Step 4: Add the dedicated gate section to the finalize SKILL**

Insert this section immediately after the *Per-change steps* block's closing **Note** (the "must not diverge" note ending step 5), before `## Where finishing-a-development-branch fits`:
```markdown
## The rebase-retest merge gate

Guards step 1's merge — the **only** place docket itself merges. It validates the
*merged result*, not just the PR head: a PR that is behind base can pass its own CI
and still break the integration branch on a semantic conflict git auto-merges cleanly.
Configured by `.docket.yml`:

```yaml
finalize:
  gate: local          # local (default) | ci | both | off
  test_command:        # OPTIONAL override; unset => the agent auto-detects the suite
```

`gate` defaults to **`local`** (gate on, validating against the repo's local suite);
`ci` validates GitHub checks; `both` requires local **and** CI green; **`off`** restores
today's behavior exactly — merge trusting the PR's own CI, no rebase and no re-test.
`test_command` is normally unset — auto-detect the suite by inspecting the repo
(Makefile, `package.json` scripts, a `tests/` dir, CI config); the override is used
verbatim only when auto-detection guesses wrong.

The gate operates in the change's feature worktree (`.worktrees/<slug>`) if it still
exists, else a transient worktree on `feat/<slug>` provisioned and torn down like
terminal-publish's `pub-<T>` tree.

**Flow** (runs before `gh pr merge`):

1. `gate == off` → merge as today (no rebase, no re-test); skip the rest of the gate.
2. **Rebase** `feat/<slug>` onto `origin/<integration_branch>`. On a clean rebase,
   continue. On conflict, **dispatch the `docket-rebase-resolver` subagent**
   (foreground, at the model/effort its wrapper resolves) to reconcile every hunk
   until the rebase completes; if it reports an **ambiguous conflict** it cannot
   resolve, the rebase is aborted and the gate **aborts-and-reports**.
3. **Determine the suite:** the `test_command` override, else auto-detect. Under
   `local`/`both` with **no detectable suite and no `test_command`**, **abort-and-report**.
4. **Validate per `gate`:**
   - `local` → run the suite in the worktree **before any push**.
   - `ci` → push `--force-with-lease`, then poll `gh pr checks`.
   - `both` → local first, then push + CI.
   On green, continue. On **red**, **dispatch the `docket-integration-repair` subagent**
   (foreground, at the model/effort its wrapper resolves) — it owns every red-test
   outcome, root-causes it, and writes a minimal fix in **at most two attempts**. If it
   reaches green, apply the **sign-off rule** below. If it is **stuck / cannot reach
   green**, **abort-and-report**. A `ci`/`both` run with **red or absent CI checks**
   also **aborts-and-reports**.
5. **Push** `--force-with-lease` if the branch was rebased and not already pushed; a
   **lease rejected by a concurrent push** → **abort-and-report**.
6. `gh pr merge` → the existing close-out (harvest → archive → terminal-publish →
   cleanup → board).

The rebase makes the feature sit on top of base, so the eventual `gh pr merge` is
conflict-free — validating the rebased branch validates what actually lands. `local`
runs the suite **before** the force-push so a broken rebase is never force-pushed;
`ci` validates after the push (CI runs on the pushed branch); `both` does both.

### The two agents (split at rebase-completion)

Conflict resolution and semantic repair are different shapes — a bounded reconciliation
versus open-ended debugging — so they are two dedicated wrappers
(`agents/docket-rebase-resolver.md` ①, `agents/docket-integration-repair.md` ②), each
wrapping **no skill** (loading only `docket-convention`), both carrying abort-and-report,
each dispatched **foreground at the model/effort its wrapper resolves** (never a literal
tier in this prose — the wrapper + layered config are the single source). The boundary is
**the rebase completing**: ① resolves conflicts *during* the rebase and never runs tests;
② owns the **red suite** *after* the rebase lands, regardless of cause (base drift or a
bad ① resolution). ①'s report is **conflicts resolved**; ②'s is an **authored repair** —
and an authored repair is what fires the sign-off rule.

### Sign-off on auto-authored repairs

A ② repair is code the human's approval predated, so it **never merges unseen** —
reconciling with the agent layer's abort-and-report rule for autonomous subagents:

- **Interactive finalize** (a human is attending the session): force-push
  `--force-with-lease` the repaired branch, **report the repair diff + what broke**, and
  **prompt** for go-ahead before `gh pr merge`.
- **Autonomous finalize** (running as its own subagent, no human to ask): it **cannot**
  prompt, so it **force-pushes the repair and aborts-and-reports** — STOP, do not merge.
  The human reviews the pushed repair on the PR and re-runs finalize to merge.

Pure ① conflict resolution does **not** trigger sign-off — it completes the merge the
human already intended and flows through the normal merge path.

### abort-and-report points (the full set)

Each leaves the **PR open** and the change **`implemented`**, surfacing a clear reason:
an ambiguous rebase conflict ① gives up on · `local`/`both` with no detectable suite and
no `test_command` override · ② cannot reach green in ≤2 attempts · `ci`/`both` with red or
absent CI checks · a `--force-with-lease` rejected by a concurrent push · any ② repair
under **autonomous** finalize (sign-off).
```

- [ ] **Step 5: Add the `finalize:` block to `.docket.yml`**

In `.docket.yml`, after the `results_dir: docs/results` line (before the `board_surfaces` block), insert:
```yaml

# Finalize merge gate (change 0015): rebase the feature branch onto the integration
# branch and re-validate the merged result before docket merges, so a behind-base PR
# can't land a semantically-broken integration branch. gate: local (default, on) |
# ci | both | off. `off` restores pre-0015 behavior (merge trusting the PR's own CI).
# test_command is normally unset — finalize auto-detects the suite; set it only to
# override a wrong guess.
finalize:
  gate: local
  # test_command:
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `bash tests/test_finalize_gate.sh`
Expected: PASS — all `ok -` through `repo .docket.yml sets finalize gate to local`. Exit 0.
If `finalize body restates NO model alias literal` is `NOT OK`, you wrote a literal tier (e.g. "opus") in the gate prose — replace it with "at the model/effort its wrapper resolves".

- [ ] **Step 7: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md .docket.yml tests/test_finalize_gate.sh
git commit -m "feat(0015): add rebase-retest merge gate to finalize + config + tests"
```

---

## Task 3: Document the gate + the two wrappers in the convention

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (Configuration section: `finalize.gate`/`test_command`; Composition section: the two finalize dispatches + count prose 6→8)
- Modify: `tests/test_finalize_gate.sh` (append convention sentinels before `exit $fail`)

- [ ] **Step 1: Append convention sentinels to `tests/test_finalize_gate.sh` (RED)**

Insert immediately before the final `exit $fail` line:
```bash
# ---- convention documents the gate + the two new wrappers --------------------
assert "convention documents finalize.gate" 'grep -Eqi "finalize\.gate|finalize:" "$CONV" && grep -qi "gate" "$CONV"'
assert "convention names the four gate modes" \
  'grep -Eqi "local[^.]*ci[^.]*both[^.]*off|gate.*off.*opt" "$CONV"'
assert "convention names docket-rebase-resolver" 'grep -q "docket-rebase-resolver" "$CONV"'
assert "convention names docket-integration-repair" 'grep -q "docket-integration-repair" "$CONV"'
assert "convention count prose says eight wrappers" 'grep -qi "eight" "$CONV"'
# Non-vacuous count guard: the "five skills get a wrapper" language must stay exact.
assert "convention keeps 'five skills get a wrapper' exact" 'grep -qi "five .*skills.* get a wrapper" "$CONV"'
```

- [ ] **Step 2: Run the test to verify the new asserts fail**

Run: `bash tests/test_finalize_gate.sh`
Expected: the six new `convention …` asserts are `NOT OK` (convention not edited yet); earlier asserts still PASS. Non-zero exit.

- [ ] **Step 3: Document `finalize.gate`/`test_command` in the convention Configuration section**

In `skills/docket-convention/SKILL.md`, inside the `### Configuration — .docket.yml` block, add to the example YAML (after the `agents:` line, keeping the existing comments intact) a `finalize:` entry, and add one explanatory sentence after the `board_surfaces` paragraph. Add to the YAML example:
```yaml
finalize:                    # merge gate (change 0015): rebase onto base + re-test before merge
  gate: local                # local (default, on) | ci | both | off  — off = pre-0015 (trust the PR's CI)
  test_command:              # OPTIONAL; unset => finalize auto-detects the suite
```
And add this paragraph after the `board_surfaces` description (before the Agent layer section):
```markdown
**`finalize` — the rebase-retest merge gate.** `finalize.gate` (`local` default · `ci` ·
`both` · `off`) governs `docket-finalize-change`'s merge step: before docket merges, it
rebases the feature branch onto `origin/<integration_branch>` and re-validates the merged
result, merging only if green. `local` runs the repo's suite locally (auto-detected, or the
`finalize.test_command` override); `ci` polls GitHub checks; `both` requires both; **`off`**
restores pre-gate behavior (merge trusting the PR's own CI). The gate is **finalize-only** —
the `docket-status` sweep never merges, so it has nothing to gate. Details: the gate flow and
its two judgment-tier agents live in `docket-finalize-change`.
```

- [ ] **Step 4: Extend the Composition section + bump the count prose 6 → 8**

In the **Composition (change 0017)** paragraph (line ~60), do two edits.

(a) After the sentence describing `auto-groom`'s critic dispatch, add the finalize dispatch:
```markdown
`docket-finalize-change` **dispatches the `docket-rebase-resolver` subagent** when its merge gate hits a rebase conflict and the **`docket-integration-repair` subagent** when the rebased suite is red (change 0015) — each, like every dispatch here, at the model/effort its own wrapper resolves (the literal tiers are **never restated** in the dispatch prose).
```

(b) Replace the closing sentence about the critic being a "sixth generated wrapper" with text that covers all eight. Change:
```
The critic is a **sixth generated wrapper** (`agents/docket-auto-groom-critic.md`, config key `auto-groom-critic`) that wraps **no skill** — it loads only `docket-convention`, never the `docket-auto-groom` designer body, so the adversary cannot inherit the designer's commit-to-the-default bias. (The "Agent layer" line above stays exact: five *skills* get a wrapper; this sixth wrapper is attached to `auto-groom` and wraps no skill.)
```
to:
```
Three of the **eight** generated wrappers wrap **no skill** — `agents/docket-auto-groom-critic.md` (config key `auto-groom-critic`, attached to `auto-groom`), and `agents/docket-rebase-resolver.md` + `agents/docket-integration-repair.md` (config keys `rebase-resolver` / `integration-repair`, attached to `finalize-change`'s gate). Each loads only `docket-convention`, never a designer/driver skill body, so it inherits no caller bias; all are auto-discovered by `sync-agents.sh`'s `agents/docket-*.md` glob (no generator edit). (The "Agent layer" line above stays exact: **five *skills* get a wrapper**; these three are wrappers that wrap no skill — eight wrappers, five skills.)
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_finalize_gate.sh`
Expected: PASS — all asserts including the six `convention …` asserts. Exit 0.

- [ ] **Step 6: Re-run the wrapper-count test (no regression)**

Run: `bash tests/test_sync_agents.sh`
Expected: PASS (the convention edits don't affect it; confirms no collateral breakage). Exit 0.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_finalize_gate.sh
git commit -m "docs(0015): document finalize.gate + the two gate wrappers in convention; count 6->8"
```

---

## Task 4: One-line finalize-only note in docket-status

**Files:**
- Modify: `skills/docket-status/SKILL.md` (the `## Merge sweep` section)
- Modify: `tests/test_finalize_gate.sh` (append a status sentinel before `exit $fail`)

- [ ] **Step 1: Append the status sentinel to `tests/test_finalize_gate.sh` (RED)**

Insert before the final `exit $fail`:
```bash
# ---- docket-status notes the gate is finalize-only ---------------------------
assert "status notes the rebase-retest gate is finalize-only" \
  'grep -Eqi "finalize-only|the sweep[^.]*never merges|only archives already-merged" "$STAT"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_finalize_gate.sh`
Expected: `NOT OK - status notes the rebase-retest gate is finalize-only`; everything else PASS. Non-zero exit.

- [ ] **Step 3: Add the one-line note to the Merge sweep section**

In `skills/docket-status/SKILL.md`, in the `## Merge sweep` section, after the opening paragraph (the one ending "…on each swept change it both archives on `metadata_branch` and, in `docket`-mode, publishes the terminal record onto the integration branch."), add:
```markdown
> **Note (the gate is finalize-only).** The rebase-onto-base + re-run-tests gate (change 0015) lives in `docket-finalize-change`'s merge step — the only place docket itself merges. The sweep **only archives PRs that are already merged**; it never performs a merge, so a pre-merge gate has nothing to act on here. A PR merged via the GitHub button bypasses the gate by nature — outside docket's control.
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_finalize_gate.sh`
Expected: PASS — all asserts including the status sentinel. Exit 0.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-status/SKILL.md tests/test_finalize_gate.sh
git commit -m "docs(0015): note the rebase-retest gate is finalize-only in docket-status sweep"
```

---

## Task 5: Whole-suite verification + stale-count sweep

**Files:** none (verification only)

- [ ] **Step 1: Run the full shell test suite**

Run: `for t in tests/test_*.sh; do echo "== $t =="; bash "$t" || echo "FAILED: $t"; done`
Expected: every test ends with no `NOT OK -` lines and no `FAILED:`. Pay special attention to `test_sync_agents.sh`, `test_finalize_gate.sh`, `test_convention_extraction.sh`, and `test_composition_wiring.sh`.

- [ ] **Step 2: Run the drift gate**

Run: `bash sync-agents.sh --check; echo "rc=$?"`
Expected: `rc=0` — the repo's `.docket.yml` has no `agents:` block, so there are no committed project-level wrapper files to drift (the generator reports "no agents: block — nothing to check").

- [ ] **Step 3: Prove two structural asserts non-vacuous (spot check)**

Temporarily break one new assert's target and confirm it flips, then revert:
```bash
# wrapper count: temporarily hide one new wrapper
mv agents/docket-rebase-resolver.md /tmp/rr.md
bash tests/test_sync_agents.sh | grep "exactly 8 built-in wrappers"   # expect: NOT OK
mv /tmp/rr.md agents/docket-rebase-resolver.md
bash tests/test_sync_agents.sh | grep "exactly 8 built-in wrappers"   # expect: ok
```
Expected: `NOT OK` then `ok`. (Documents the assert is real; no commit.)

- [ ] **Step 4: Stale-count sweep (LEARNINGS #14/#5)**

Run: `grep -rniE "\bsix(th)?\b|= \"6\"" skills/ agents/ tests/ README.md | grep -iE "wrapper|agent"`
Expected: **no lines** that refer to the wrapper count as six (the `test_convention_extraction.sh` "sixth skill" hits refer to a **skill**, not a wrapper — those are correct and may remain; confirm by reading each hit). If any wrapper-count "six"/`= "6"` remains, fix it and re-run the affected test.

- [ ] **Step 5: Confirm the working tree is clean and all work is committed**

Run: `git -C "$(git rev-parse --show-toplevel)" status --short`
Expected: empty (every change committed across Tasks 1–4).

---

## Notes for the implementer / reviewer

- **This is the docket repo dogfooding its own change.** The feature worktree is `.worktrees/finalize-rebase-retest-gate` on `feat/finalize-rebase-retest-gate`, cut from `origin/main`. Plan/results/code commit here; the change file, board, ADRs, and spec are metadata on the `docket` branch (the `.docket/` worktree) and must **never** be touched from this feature worktree.
- **ADR at review (step 6 of docket-implement-next):** two decisions are ADR-worthy — (1) the gate splits conflict-resolution from semantic-repair into two pinned no-skill agents at a rebase-completion boundary (likely a **new** ADR, `relates_to: [8, 9]`, reusing change 0017's named-subagent-dispatch + git-state-contract pattern); (2) finalize may author a repair gated by sign-off — possibly folded into the same ADR, or recorded as an **Update to ADR-0008** if it reads as an extension of that abort-and-report rule. Decide new-vs-update at review per the #16/#17 precedent. Append the resulting number(s) to change 0015's `adrs:` on the `docket` branch.
- **Sentinels are sampling, not parsing** — the whole-branch code review is the real correctness net for the gate prose. Read the gate section for meaning, not just for the greps.
```
