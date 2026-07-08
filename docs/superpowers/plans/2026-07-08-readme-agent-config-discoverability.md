# README agent-config discoverability — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a discoverable, dedicated section to the repo-root `README.md` that answers "how do I change the model/effort an agent runs at, and how do I make it take effect?" — a pure discoverability fix (the facts already exist, buried in the Install prose).

**Architecture:** Single-artifact prose addition to `README.md`, guarded by section-scoped doc-sentinel tests appended to `tests/test_sync_agents.sh` (the topical home for agent-config generation). The section **references** docket-convention's "Agent layer" for the config *shape* rather than restating field examples, so #0046's harness-first `agents:` rework changes the shape in one place. Because this is a tightly-coupled single-artifact edit, it is built inline (per LEARNINGS #1), not fanned out to subagents.

**Tech Stack:** Markdown, bash doc-sentinel tests (`assert()` + `grep`/`awk`), the repo's standalone `bash tests/test_*.sh` convention.

## Global Constraints

- **Docs only.** No behavior change to `sync-agents.sh` or any script (change 0047 "Out of scope").
- **Do NOT restate the `agents:` config shape** — reference docket-convention's "Agent layer"; #0046 owns the shape (LEARNINGS #17: a config-overridable value must not be restated in prose).
- **Do NOT hardcode any per-skill model/effort literal** (e.g. `opus`/`xhigh`) in the README — those are config-overridable; the built-in defaults live only in `agents/docket-*.md` (LEARNINGS #17).
- Doc sentinels must be **non-vacuous** and **section-scoped**: each asserts against the *extracted new section body*, not the whole README (most individual facts already appear in the Install prose, so a whole-README grep would pass vacuously — LEARNINGS #2/#20/#21). Each assert anchors to exactly one clause it owns.
- Test convention: standalone `bash tests/test_sync_agents.sh`; `set -uo pipefail`; `assert(){ eval "$2" … }`; append before the final `exit $fail`.

---

### Task 1: Discoverable agent model/effort section + section-scoped doc sentinels

**Files:**
- Modify: `README.md` (add one new `## ` section between the `docket-mode: where metadata lives` section and `## The eight skills`)
- Test: `tests/test_sync_agents.sh` (append a sentinel block before `exit $fail`)

**Interfaces:**
- Consumes: nothing (leaf docs change).
- Produces: a README `## ` heading matching `.*[Aa]gent.*([Mm]odel|[Ee]ffort)` whose body contains the anchored substrings the sentinels assert.

- [ ] **Step 1: Write the failing sentinel block** — append to `tests/test_sync_agents.sh` immediately BEFORE the final `exit $fail` line:

```bash
# ---- README discoverability of the agent model/effort refresh workflow (change 0047) ----
# The facts already exist buried in the Install prose, so a whole-README grep would pass
# vacuously. Extract the NEW dedicated section (heading -> next `## `) and assert within it,
# so each sentinel is RED before the section exists and non-vacuous after.
READMEF="$REPO/README.md"
sec="$(awk '/^##[[:space:]].*[Aa]gent.*([Mm]odel|[Ee]ffort)/{f=1;print;next} f&&/^##[[:space:]]/{f=0} f{print}' "$READMEF")"

assert "0047: README has a discoverable agent model/effort section" '[ -n "$sec" ]'
assert "0047 §agent-cfg: names the global layer ~/.config/docket/agents.yaml" \
  'grep -qF "~/.config/docket/agents.yaml" <<<"$sec"'
assert "0047 §agent-cfg: names the per-repo .docket.yml agents: layer" \
  'grep -qF ".docket.yml" <<<"$sec" && grep -qi "per-repo" <<<"$sec"'
assert "0047 §agent-cfg: gives the refresh command (bash sync-agents.sh)" \
  'grep -qE "bash sync-agents\.sh" <<<"$sec"'
assert "0047 §agent-cfg: names the user-level target (every present harness)" \
  'grep -qiE "present.*harness" <<<"$sec"'
assert "0047 §agent-cfg: names the project-level target (agent_harnesses)" \
  'grep -qF "agent_harnesses" <<<"$sec"'
assert "0047 §agent-cfg: documents the --check drift gate" \
  'grep -qF "sync-agents.sh --check" <<<"$sec"'
assert "0047 §agent-cfg: references docket-convention Agent layer for the shape (not restated)" \
  'grep -qF "docket-convention" <<<"$sec" && grep -qi "agent layer" <<<"$sec"'
# Non-restatement guard: the section must NOT hardcode a per-skill model/effort literal
# (those are config-overridable; built-in defaults live only in agents/docket-*.md). LEARNINGS #17.
assert "0047 §agent-cfg: does NOT hardcode a model/effort literal (references the source instead)" \
  '! grep -qiE "\b(opus|sonnet|haiku|fable)\b.*\b(xhigh|high|medium|low)\b|model:[[:space:]]*(opus|sonnet|haiku|claude-)" <<<"$sec"'
```

- [ ] **Step 2: Run the test to verify it FAILS** — the section does not exist yet, so `$sec` is empty and every `§agent-cfg` assert plus the section-presence assert are NOT OK:

Run: `bash tests/test_sync_agents.sh`
Expected: pre-existing asserts `ok - …`; the new block prints `NOT OK - 0047: README has a discoverable agent model/effort section` (and the section-scoped ones); overall exit non-zero.

- [ ] **Step 3: Add the section to `README.md`** — insert this block after the `main`-mode opt-out section's trailing `---` (the divider that closes `## docket-mode: where metadata lives`, currently just above `## The eight skills`), and before `## The eight skills`:

```markdown
## Tuning an agent's model & effort

Each **autonomous** docket skill runs as a model/effort-pinned subagent (`docket-implement-next`, `docket-auto-groom`, `docket-finalize-change`, `docket-status`, `docket-adr`; the two interactive skills, `docket-new-change` and `docket-groom-next`, stay inline and only surface an advisory recommendation). To change the model or effort one of them runs at:

**1. Edit a config layer.** Two layers override the built-in defaults (precedence: per-repo > global > built-in):

- **Global** — `~/.config/docket/agents.yaml` (user-level; applies to every repo on your machine).
- **Per-repo** — the `agents:` block in a repo's committed `.docket.yml` (applies to that repo for every clone and agent, so an autonomous change builds on the same model everywhere).

The config **shape** — the `agents:` keys and how `model:`/`effort:` are written — is documented once in docket-convention's **"Agent layer"**; consult it there rather than copying field examples here, so the shape has a single source of truth and stays current as it evolves.

**2. Refresh the generated wrappers.** The resolved model/effort are baked into generated wrapper *copies* (not symlinks), so after editing any layer, regenerate them:

```bash
bash sync-agents.sh        # or re-run install.sh, which calls it for you
```

- A **global** edit rewrites user-level wrappers into every **present** harness root (`~/.<harness>/agents/`, e.g. `~/.claude/agents/`).
- A **per-repo** edit rewrites the committed **project-level** wrappers for each harness in that repo's `.docket.yml` `agent_harnesses:` list (default `[claude]`; e.g. `[claude, cursor]` for a repo that also drives Cursor).

**3. Guard drift in CI.** `sync-agents.sh --check` exits non-zero (with a diff) when the committed project-level wrappers have fallen out of sync with the resolved config — wire it into CI so a config edit that was never regenerated fails the build instead of silently drifting.
```

- [ ] **Step 4: Run the test to verify it PASSES**

Run: `bash tests/test_sync_agents.sh`
Expected: all asserts `ok - …`, including every `0047 …` line; exit 0.

- [ ] **Step 5: Mutation-check the strongest sentinel is non-vacuous** — temporarily delete the new README section heading line, re-run, confirm the `0047: README has a discoverable agent model/effort section` assert flips to NOT OK, then restore. (Manual guard per LEARNINGS #2 — do not commit the mutation.)

- [ ] **Step 6: Commit** (feature branch, in the feature worktree)

```bash
git add README.md tests/test_sync_agents.sh docs/superpowers/plans/2026-07-08-readme-agent-config-discoverability.md
git commit -m "docs(0047): discoverable agent model/effort section in README"
```

## Self-Review

**1. Spec coverage** — the change's "What changes" bullets map 1:1 to the section: two layers named (global + per-repo) ✓; refresh command `bash sync-agents.sh` ✓; targets (present harness user-level + `agent_harnesses` project-level) ✓; `sync-agents.sh --check` drift gate ✓; references docket-convention "Agent layer" for the shape ✓. Each has a sentinel.

**2. Placeholder scan** — README content and test code are given verbatim; no TBD/TODO.

**3. Type/name consistency** — the awk heading regex `.*[Aa]gent.*([Mm]odel|[Ee]ffort)` matches the chosen heading `## Tuning an agent's model & effort` ("agent" then "model"). Sentinel substrings (`~/.config/docket/agents.yaml`, `.docket.yml`, `per-repo`, `bash sync-agents.sh`, `present`+`harness`, `agent_harnesses`, `sync-agents.sh --check`, `docket-convention`+`agent layer`) each appear in the section body and are anchored to one clause each (no double-guard).
