# docket skills as model/effort-pinned subagents — foundation (0016) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make each autonomous docket skill runnable as a model/effort-pinned subagent, generated from layered config, with sane built-in defaults — without editing skill bodies.

**Architecture:** Ship 5 committed built-in wrapper files under `agents/docket-*.md` (each pins its default model+effort and injects the skill via `skills:`). A new `sync-agents.sh` (sibling of `link-skills.sh`) resolves layered config — built-in ⊕ global (`~/.config/docket/agents.yaml`) ⊕ per-repo (`.docket.yml agents:`) — and writes **generated copies**: user-level wrappers (built-in ⊕ global) into each present harness `*/agents/` dir, and committed project-level wrappers (built-in ⊕ per-repo) into `<repo>/.claude/agents/`. Claude Code's project-over-user precedence applies the layers natively (no 3-way merge in the script). A `--check` mode is the CI drift gate. The 2 interactive skills stay inline with an advisory model/effort recommendation; `docket-convention` documents the whole contract.

**Tech Stack:** Bash (`set -euo pipefail`, `sed`/`awk`/`grep` only — no `yq`/python, matching `link-skills.sh` / `scripts/github-mirror.sh`); plain-bash test harness with the `DOCKET_HARNESS_ROOT` seam (matching `tests/test_link_skills.sh`); Markdown agent/skill/convention files.

---

## Background the engineer must know

- **This repo is docket itself**, run in docket-mode. Code + scripts + skills live on `main` (the integration branch). You are on a feature branch `feat/docket-subagent-model-effort` cut from `origin/main`, in worktree `.worktrees/docket-subagent-model-effort`. **Do all work here.** Never touch the `.docket/` worktree or the `docket` branch — that is docket's planning metadata, owned by the orchestrating skill, not this build.
- **The skills are the single source of behavior.** A wrapper does NOT restate skill logic — it pins model/effort, injects the skill via `skills:`, and adds a one-line directive. Skill bodies are NOT edited by this change, **except** the two interactive skills (`docket-new-change`, `docket-groom-next`), which gain a small advisory recommendation (Task 6).
- **Reference the spec** at `docs/superpowers/specs/2026-06-15-docket-subagent-model-effort-design.md` (read it from the `docket` branch / `.docket/` if needed; it does not exist on this feature branch). Key tables: §4 (built-in default table), §5 (layered config), §9 (testing strategy). The reconcile log in the change file resolved two open questions: (1) harness agent-dir list = mirror `link-skills.sh`'s `HARNESS_SKILL_DIRS` swapping `skills`→`agents`; (2) 0016 ships **5** wrapper files (no separate critic file — that is 0017).
- **The built-in default table (§4), made concrete:**

  | wrapper (`agents/<file>`) | skill injected | model | effort |
  |---|---|---|---|
  | `docket-implement-next.md` | docket-implement-next | `opus` | `xhigh` |
  | `docket-auto-groom.md` | docket-auto-groom | `opus` | `xhigh` |
  | `docket-finalize-change.md` | docket-finalize-change | `sonnet` | `medium` |
  | `docket-status.md` | docket-status | `sonnet` | `medium` |
  | `docket-adr.md` | docket-adr | `sonnet` | `medium` |

  The 2 interactive skills get **no** wrapper file (advisory only): `new-change` → recommend `sonnet` (effort: model default), `groom-next` → recommend `sonnet` / `high`.

- **Layer/precedence model (why two layers, not three):** committed project-level files must be clone-identical, so they must NOT bake the per-machine global layer. Therefore: user-level files = built-in ⊕ global; project-level files = built-in ⊕ per-repo. Claude Code picks project-over-user natively, yielding the effective precedence **per-repo > global > built-in** without the generator merging three layers per file.
- **Config shapes:**
  - Per-repo, in `<repo>/.docket.yml` (keys are skill SHORT names, under an `agents:` block):
    ```yaml
    agents:
      implement-next: { model: opus,   effort: xhigh }
      status:         { model: sonnet, effort: medium }
      # unlisted -> built-in default; effort: auto -> omit the frontmatter effort line
    ```
  - Global, in `~/.config/docket/agents.yaml` (same `name: { model, effort }` lines, but at TOP level — no `agents:` wrapper key, it is a dedicated file):
    ```yaml
    implement-next: { model: sonnet }
    status:         { model: haiku, effort: low }
    ```
- **`effort` rules:** allowed values `low|medium|high|xhigh|max`; there is **no `auto`** frontmatter value — `effort: auto` (or unset) in config means **omit** the `effort:` line (inherit model default). Built-in wrappers for the 5 autonomous skills always carry an explicit model+effort, so a config override only ever *replaces* (or, for `auto`, *removes*) those lines.
- **Relevant learnings (from `docs/changes/LEARNINGS.md`):**
  - Never `producer | grep -q` under `set -o pipefail` (SIGPIPE → 141 → intermittent failure). Capture output to a variable first, then grep the variable.
  - YAML frontmatter: an unquoted scalar cannot contain `": "` (colon-space). The skill descriptions you copy use em-dashes, not colon-space — keep them that way; do not introduce colon-space into any generated unquoted scalar.
  - Prove each test assertion non-vacuous: deleting the clause it guards must flip the test to NOT OK.
  - Sentinel greps are sampling, not parsing — pair doc-edit sentinels with the whole-branch review.

---

## File structure

- **Create** `agents/docket-implement-next.md`, `agents/docket-auto-groom.md`, `agents/docket-finalize-change.md`, `agents/docket-status.md`, `agents/docket-adr.md` — committed built-in wrappers (source of truth for the default table).
- **Create** `sync-agents.sh` — the generator/installer (`--check` mode included).
- **Create** `tests/test_sync_agents.sh` — the test suite (grows across Tasks 1–3, 5, 6).
- **Modify** `.docket.yml` — add a commented `agents:` schema block.
- **Modify** `skills/docket-convention/SKILL.md` — document the agent layer, config, precedence, generator, abort-and-report, composition pointer.
- **Modify** `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md` — advisory model/effort recommendation at startup.
- **Modify** `README.md` — add `sync-agents.sh` to install; add `agents:` to the `.docket.yml` example.

---

## Task 1: Built-in agent wrapper source files

**Files:**
- Create: `agents/docket-implement-next.md`, `agents/docket-auto-groom.md`, `agents/docket-finalize-change.md`, `agents/docket-status.md`, `agents/docket-adr.md`
- Test: `tests/test_sync_agents.sh`

- [ ] **Step 1: Write the failing test**

Create `tests/test_sync_agents.sh`:

```bash
#!/usr/bin/env bash
# tests/test_sync_agents.sh — run: bash tests/test_sync_agents.sh
set -uo pipefail
unset XDG_CONFIG_HOME   # hermetic: the script reads ${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}; pin global to the sandbox
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Extract a single-line frontmatter scalar value from a markdown file.
fm(){ sed -n "s/^$2:[[:space:]]*//p" "$1" | head -n1 | sed 's/[[:space:]]*$//'; }

# ---- Task 1: built-in wrapper source files ---------------------------------
AGENTS="$REPO/agents"
AUTONOMOUS="docket-implement-next docket-auto-groom docket-finalize-change docket-status docket-adr"

assert "agents/ source dir exists" '[ -d "$AGENTS" ]'
assert "exactly 5 built-in wrappers" '[ "$(find "$AGENTS" -maxdepth 1 -name "docket-*.md" | wc -l | tr -d " ")" = "5" ]'

for w in $AUTONOMOUS; do
  f="$AGENTS/$w.md"
  assert "$w: file exists" '[ -f "$f" ]'
  assert "$w: name matches file" '[ "$(fm "$f" name)" = "$w" ]'
  assert "$w: has a description" '[ -n "$(fm "$f" description)" ]'
  assert "$w: description matches the skill (single source)" \
    '[ "$(fm "$f" description)" = "$(fm "$REPO/skills/$w/SKILL.md" description)" ]'
  assert "$w: model is a known alias" '[[ "$(fm "$f" model)" =~ ^(opus|sonnet|haiku|fable)$ ]]'
  assert "$w: effort in allowed set" '[[ "$(fm "$f" effort)" =~ ^(low|medium|high|xhigh|max)$ ]]'
  assert "$w: skills: injects the skill itself" 'grep -Eq "^skills:.*\b'"$w"'\b" "$f"'
  assert "$w: skills: injects docket-convention" 'grep -Eq "^skills:.*docket-convention" "$f"'
  assert "$w: body carries abort-and-report directive" 'grep -qi "abort-and-report" "$f"'
done

# Built-in model/effort match the §4 default table.
assert "implement-next built-in = opus/xhigh" \
  '[ "$(fm "$AGENTS/docket-implement-next.md" model)/$(fm "$AGENTS/docket-implement-next.md" effort)" = "opus/xhigh" ]'
assert "auto-groom built-in = opus/xhigh" \
  '[ "$(fm "$AGENTS/docket-auto-groom.md" model)/$(fm "$AGENTS/docket-auto-groom.md" effort)" = "opus/xhigh" ]'
assert "finalize-change built-in = sonnet/medium" \
  '[ "$(fm "$AGENTS/docket-finalize-change.md" model)/$(fm "$AGENTS/docket-finalize-change.md" effort)" = "sonnet/medium" ]'
assert "status built-in = sonnet/medium" \
  '[ "$(fm "$AGENTS/docket-status.md" model)/$(fm "$AGENTS/docket-status.md" effort)" = "sonnet/medium" ]'
assert "adr built-in = sonnet/medium" \
  '[ "$(fm "$AGENTS/docket-adr.md" model)/$(fm "$AGENTS/docket-adr.md" effort)" = "sonnet/medium" ]'

# Advisory/interactive skills must NOT have a wrapper file.
assert "no wrapper for new-change (advisory)" '[ ! -f "$AGENTS/docket-new-change.md" ]'
assert "no wrapper for groom-next (advisory)" '[ ! -f "$AGENTS/docket-groom-next.md" ]'

exit $fail
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_sync_agents.sh`
Expected: FAIL — `NOT OK - agents/ source dir exists` (and the per-wrapper assertions).

- [ ] **Step 3: Create the 5 wrapper files**

The `description:` value for each MUST be copied verbatim from the matching `skills/<name>/SKILL.md` (the test enforces this). Get each with:
`sed -n 's/^description:[[:space:]]*//p' skills/docket-implement-next/SKILL.md | head -n1`

Create `agents/docket-implement-next.md`:

```markdown
---
name: docket-implement-next
description: Use when you want the next build-ready change in the docket backlog implemented end-to-end to an open PR with no human interaction — picking, claiming, reconciling against current reality, planning, building with TDD, reviewing, and stopping at the human merge gate. The autonomous backlog-drainer; runs solo per change.
model: opus
effort: xhigh
skills: [docket-implement-next, docket-convention]
---
Execute docket-implement-next to drain the next build-ready change. Follow the skill exactly.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
```

Create `agents/docket-auto-groom.md`:

```markdown
---
name: docket-auto-groom
description: Use when a repo (or individual stubs) opted into autonomous grooming and you want the auto-groomable needs-brainstorm queue drained with no human — selecting each autonomous-eligible stub deterministically and designing it via a default-biased self-brainstorm gated by an adversarial critic, exiting each stub with a linked spec, a trivial verdict, or an abstain back to the human queue. Kill and defer are never autonomous. Writes markdown only — never branches, worktrees, or code.
model: opus
effort: xhigh
skills: [docket-auto-groom, docket-convention]
---
Execute docket-auto-groom to drain the autonomous grooming queue. Follow the skill exactly.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
```

Create `agents/docket-finalize-change.md`:

```markdown
---
name: docket-finalize-change
description: Use when a change's PR is approved or merged and you want to close it out to done promptly rather than waiting for the safety-net sweep — merging if approved, verifying the merge landed, archiving the change, cleaning up its branch and worktree, and refreshing the board. The human's closing bookend; mirrors docket-new-change.
model: sonnet
effort: medium
skills: [docket-finalize-change, docket-convention]
---
Execute docket-finalize-change to close out the change. Follow the skill exactly.

You run autonomously with no human to pause and ask: treat any unmet precondition (PR not actually approved, merge conflict, dirty worktree) or blocking ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
```

Create `agents/docket-status.md`:

```markdown
---
name: docket-status
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by regenerating the BOARD.md board, sweeping merged changes to done, or running health checks for stale claims, broken spec/plan/results links, and dependency stalls.
model: sonnet
effort: medium
skills: [docket-status, docket-convention]
---
Execute docket-status to refresh the board and run the sweep + health checks. Follow the skill exactly.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
```

Create `agents/docket-adr.md`:

```markdown
---
name: docket-adr
description: Use when recording, superseding, reversing, or indexing an architecture decision (ADR) — capturing why a non-obvious technical decision was made into the immutable docs/adrs ledger, or regenerating and validating the ADR index. Invoked by docket-implement-next, or directly any time a decision must be recorded or changed.
model: sonnet
effort: medium
skills: [docket-adr, docket-convention]
---
Execute docket-adr to record or re-index an architecture decision. Follow the skill exactly.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
```

> Note: if any `description:` differs from its skill's, the test's "description matches the skill" assertion fails — re-copy from the live `skills/<name>/SKILL.md` rather than retyping.

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_sync_agents.sh`
Expected: PASS — all `ok -` lines, exit 0.

- [ ] **Step 5: Commit**

```bash
git add agents tests/test_sync_agents.sh
git commit -m "0016: built-in agent wrapper source files + validity tests"
```

---

## Task 2: sync-agents.sh — generator (install + global + per-repo passes)

**Files:**
- Create: `sync-agents.sh`
- Test: `tests/test_sync_agents.sh` (append)

- [ ] **Step 1: Append the failing tests**

Append BEFORE the final `exit $fail` line in `tests/test_sync_agents.sh`:

```bash
# ---- Task 2: sync-agents.sh generator --------------------------------------
SYNC="$REPO/sync-agents.sh"
assert "sync-agents.sh exists and is executable-by-bash" '[ -f "$SYNC" ]'

# Helper: a fresh fake harness root + repo for an isolated generator run.
make_sandbox(){ SBX="$(mktemp -d)"; mkdir -p "$SBX/.claude" "$SBX/.agents"; }   # .cursor/.codex/.kiro/.windsurf absent on purpose

# -- user-level install: built-in wrappers, verbatim, into present harnesses --
make_sandbox
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "writes into present .claude/agents" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "writes into present .agents/agents" '[ -f "$SBX/.agents/agents/docket-status.md" ]'
assert "all 5 wrappers land in .claude/agents" '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "5" ]'
assert "does NOT create an absent harness (.cursor)" '[ ! -d "$SBX/.cursor/agents" ]'
assert "no override => byte-identical to built-in source" 'diff -q "$REPO/agents/docket-status.md" "$SBX/.claude/agents/docket-status.md" >/dev/null'

# -- idempotency: second run is byte-identical ----
before="$(cat "$SBX/.claude/agents/docket-implement-next.md")"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
after="$(cat "$SBX/.claude/agents/docket-implement-next.md")"
assert "second run idempotent (byte-identical)" '[ "$before" = "$after" ]'
rm -rf "$SBX"

# -- global layer: ~/.config/docket/agents.yaml overrides model/effort (user-level) --
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'status: { model: haiku, effort: low }\nimplement-next: { effort: auto }\n' > "$SBX/.config/docket/agents.yaml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "global override sets model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
assert "global override sets effort" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "low" ]'
assert "effort: auto drops the effort line" '! grep -q "^effort:" "$SBX/.claude/agents/docket-implement-next.md"'
assert "auto keeps the built-in model" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "opus" ]'
assert "unlisted skill keeps built-in model+effort" '[ "$(fm "$SBX/.claude/agents/docket-adr.md" model)/$(fm "$SBX/.claude/agents/docket-adr.md" effort)" = "sonnet/medium" ]'
rm -rf "$SBX"

# -- per-repo layer: .docket.yml agents: => committed project-level files --
make_sandbox
printf 'agents:\n  status: { model: sonnet, effort: high }\n  new-change: { model: opus }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "per-repo override writes project-level file" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "per-repo override applies model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "per-repo override applies effort" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "high" ]'
assert "no project-level file for unlisted skill (implement-next)" '[ ! -f "$SBX/.claude/agents/docket-implement-next.md" ] || diff -q "$REPO/agents/docket-implement-next.md" "$SBX/.claude/agents/docket-implement-next.md" >/dev/null'
assert "advisory skill in agents: produces NO file (new-change)" '[ ! -f "$SBX/.claude/agents/docket-new-change.md" ]'
rm -rf "$SBX"
```

> Note on the "unlisted skill" assertion: when `.docket.yml` exists, the user-level pass still runs (no global file ⇒ verbatim built-in) AND the project-level pass writes only listed skills into the SAME `<repo>/.claude/agents/` dir. Since the sandbox root == repo root in the test, `docket-implement-next.md` may be present as the verbatim user-level copy; the assertion accepts "absent OR byte-identical to built-in" so it stays true either way and never flags a non-override.

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_sync_agents.sh`
Expected: FAIL — `NOT OK - sync-agents.sh exists ...` and all Task 2 assertions.

- [ ] **Step 3: Create `sync-agents.sh`**

```bash
#!/usr/bin/env bash
# sync-agents.sh — generate docket's model/effort-pinned subagent wrappers into each PRESENT
# agent-harness dir, resolving layered config (built-in ⊕ global ⊕ per-repo).
#
# Unlike link-skills.sh (which SYMLINKS skills/<name>), agent files bake resolved model/effort,
# so they are GENERATED COPIES this script owns and OVERWRITES on every run.
#
# Layers & precedence — per-repo > global > built-in:
#   built-in  agents/docket-*.md in this repo (each ships its default model/effort)
#   global    ~/.config/docket/agents.yaml        -> user-level    ~/.claude/agents/docket-*.md
#   per-repo  <repo>/.docket.yml `agents:` block  -> project-level <repo>/.claude/agents/docket-*.md (committed)
# Claude Code applies project-over-user precedence natively, so the generator writes two layers
# (user = built-in⊕global, project = built-in⊕per-repo) and never hand-merges all three.
#
# Usage:
#   bash sync-agents.sh           # write user-level (built-in ⊕ global); and, if <repo>/.docket.yml
#                                 # has an `agents:` block, project-level (built-in ⊕ per-repo)
#   bash sync-agents.sh --check   # CI gate: exit non-zero (with a diff) if committed project-level
#                                 # files drift from what the resolved config would generate
#
# Test seam: DOCKET_HARNESS_ROOT overrides $HOME (harness dirs AND the global-config root).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_SRC="$SCRIPT_DIR/agents"
REPO="$PWD"

HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"
GLOBAL_CFG="${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}/docket/agents.yaml"
DOCKET_YML="$REPO/.docket.yml"
PROJECT_AGENT_DIR="$REPO/.claude/agents"

# Mirror link-skills.sh's HARNESS_SKILL_DIRS, swapping skills -> agents.
HARNESS_AGENT_DIRS=(
  "$HARNESS_ROOT/.claude/agents"
  "$HARNESS_ROOT/.codex/agents"
  "$HARNESS_ROOT/.cursor/agents"
  "$HARNESS_ROOT/.agents/agents"
  "$HARNESS_ROOT/.kiro/agents"
  "$HARNESS_ROOT/.windsurf/agents"
)

CHECK=0
[ "${1:-}" = "--check" ] && CHECK=1

log(){ printf '%s\n' "sync-agents: $*" >&2; }

short_name(){ local b; b="$(basename "$1")"; b="${b#docket-}"; printf '%s' "${b%.md}"; }

# --- config helpers ----------------------------------------------------------
# Print the single config line for <name> from <file>, optionally only within an `agents:` block.
# Captures each pipeline stage into a variable (never `producer | grep -q`) to stay SIGPIPE-safe.
entry_line() {  # $1=file  $2=name  $3=under_block(0|1)
  local file="$1" name="$2" under="$3" body stripped matched
  [ -f "$file" ] || return 0
  if [ "$under" = "1" ]; then
    body="$(awk '/^agents:[[:space:]]*$/{f=1;next} f&&/^[^[:space:]#]/{f=0} f{print}' "$file")"
  else
    body="$(cat "$file")"
  fi
  stripped="$(printf '%s\n' "$body" | sed 's/#.*//')"
  matched="$(printf '%s\n' "$stripped" | grep -E "^[[:space:]]*${name}[[:space:]]*:" || true)"
  printf '%s\n' "$matched" | head -n1
}

# Extract one field value (model/effort) from a config entry line. Empty if absent.
field_of() {  # $1=line  $2=field
  printf '%s' "$1" | sed -nE "s/.*[{,[:space:]]${2}[[:space:]]*:[[:space:]]*([A-Za-z0-9._-]+).*/\1/p" | head -n1
}

# Names listed under <file>'s `agents:` block, one per line.
block_names() {  # $1=file
  [ -f "$1" ] || return 0
  awk '
    /^agents:[[:space:]]*$/{f=1;next}
    f&&/^[^[:space:]#]/{f=0}
    f&&/^[[:space:]]+[A-Za-z0-9._-]+[[:space:]]*:/{
      line=$0; sub(/#.*/,"",line); sub(/^[[:space:]]+/,"",line); sub(/[[:space:]]*:.*/,"",line);
      if(line!="") print line
    }' "$1"
}

# Resolve an override for <name> from <file>/<under_block> into RES_MODEL / RES_EFFORT (empty = none).
resolve_from() {  # $1=file  $2=name  $3=under_block
  local line; line="$(entry_line "$1" "$2" "$3")"
  RES_MODEL="$(field_of "$line" model)"
  RES_EFFORT="$(field_of "$line" effort)"
}

# --- emit a resolved wrapper to stdout ---------------------------------------
# Rewrites model:/effort: lines inside the frontmatter. Empty override => keep built-in.
# effort override "auto" => drop the effort line entirely (inherit model default).
emit() {  # $1=src file  $2=model  $3=effort
  awk -v model="$2" -v effort="$3" '
    /^---[[:space:]]*$/ { d++; print; infm=(d==1); next }
    {
      if (infm && model!=""  && $0 ~ /^model[[:space:]]*:/)  { print "model: " model; next }
      if (infm && effort!="" && $0 ~ /^effort[[:space:]]*:/) { if (effort!="auto") print "effort: " effort; next }
      print
    }' "$1"
}

# --- passes ------------------------------------------------------------------
user_level_pass() {  # built-in ⊕ global -> each present harness */agents dir
  local src dir name
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    resolve_from "$GLOBAL_CFG" "$name" 0
    for dir in "${HARNESS_AGENT_DIRS[@]}"; do
      [ -d "$(dirname "$dir")" ] || continue   # only into harnesses that exist (parent root present)
      mkdir -p "$dir"
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$dir/$(basename "$src")"
    done
  done
}

project_level_pass() {  # built-in ⊕ per-repo -> <repo>/.claude/agents (committed)
  [ -f "$DOCKET_YML" ] || return 0
  local names name src
  names="$(block_names "$DOCKET_YML")"
  [ -n "$names" ] || return 0
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    src="$AGENTS_SRC/docket-$name.md"
    if [ ! -f "$src" ]; then
      log "skip '$name' — no built-in wrapper (advisory/interactive skills have no agent file)"
      continue
    fi
    resolve_from "$DOCKET_YML" "$name" 1
    mkdir -p "$PROJECT_AGENT_DIR"
    emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$PROJECT_AGENT_DIR/docket-$name.md"
  done <<EOF
$names
EOF
}

user_level_pass
project_level_pass
log "done"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_sync_agents.sh`
Expected: PASS — all `ok -`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "0016: sync-agents.sh generator — built-in/global/per-repo layered wrappers"
```

---

## Task 3: sync-agents.sh — `--check` drift gate

**Files:**
- Modify: `sync-agents.sh`
- Test: `tests/test_sync_agents.sh` (append)

- [ ] **Step 1: Append the failing tests**

Append BEFORE the final `exit $fail`:

```bash
# ---- Task 3: --check drift gate --------------------------------------------
make_sandbox
printf 'agents:\n  status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )   # generate committed project file
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check passes when committed agents match config (rc=0)" '[ "$chk_rc" = "0" ]'

# Out-of-band edit to a committed project-level file -> drift.
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-status.md"; rm -f "$SBX/.claude/agents/docket-status.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check fails on drift (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "--check reports a diff" 'printf "%s" "$chk_out" | grep -q "drift"'

# A repo with no agents: block has nothing to check -> passes.
rm -f "$SBX/.docket.yml"; : > "$SBX/.docket.yml"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check passes when no agents: block (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_sync_agents.sh`
Expected: FAIL — `--check` currently does nothing (the script ignores `$CHECK`), so `--check fails on drift` is NOT OK.

- [ ] **Step 3: Add the `--check` function and branch the main block**

In `sync-agents.sh`, ADD this function immediately after `project_level_pass() { ... }`:

```bash
check_project_level() {  # diff committed project-level files against freshly-resolved config
  local rc=0 names name src got tmp d
  [ -f "$DOCKET_YML" ] || { log "no .docket.yml in $REPO — nothing to check"; return 0; }
  names="$(block_names "$DOCKET_YML")"
  [ -n "$names" ] || { log "no agents: block — nothing to check"; return 0; }
  tmp="$(mktemp -d)"
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    src="$AGENTS_SRC/docket-$name.md"
    [ -f "$src" ] || continue
    resolve_from "$DOCKET_YML" "$name" 1
    emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$tmp/docket-$name.md"
    got="$PROJECT_AGENT_DIR/docket-$name.md"
    if [ ! -f "$got" ]; then
      log "drift: missing $got (run: bash sync-agents.sh)"; rc=1; continue
    fi
    d="$(diff -u "$got" "$tmp/docket-$name.md" || true)"   # capture (SIGPIPE-safe), do not pipe to grep -q
    if [ -n "$d" ]; then log "drift in docket-$name.md:"; printf '%s\n' "$d" >&2; rc=1; fi
  done <<EOF
$names
EOF
  rm -rf "$tmp"
  return $rc
}
```

Then REPLACE the final three lines of the script:

```bash
user_level_pass
project_level_pass
log "done"
```

with:

```bash
if [ "$CHECK" = "1" ]; then
  if check_project_level; then exit 0; else exit 1; fi
fi

user_level_pass
project_level_pass
log "done"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_sync_agents.sh`
Expected: PASS — all `ok -`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "0016: sync-agents.sh --check drift gate for committed project-level agents"
```

---

## Task 4: Document the `agents:` schema in `.docket.yml`

**Files:**
- Modify: `.docket.yml`

- [ ] **Step 1: Add the commented schema block**

Append to the end of `.docket.yml` (this repo keeps the built-in defaults — no active override, so the block is documentation only):

```yaml

# Per-skill subagent model/effort (change 0016). Generated into agent wrappers by sync-agents.sh.
# Keys are skill SHORT names; precedence is per-repo (this block) > global
# (~/.config/docket/agents.yaml) > built-in (agents/docket-*.md). An `effort: auto` (or omitted)
# means inherit the model default (the frontmatter effort line is dropped). Only the 5 autonomous
# skills have wrappers; new-change/groom-next are advisory-only and ignore this block. Listing a
# skill here generates a COMMITTED project-level .claude/agents/docket-<skill>.md — run
# `sync-agents.sh` after editing, and `sync-agents.sh --check` in CI to catch drift.
#
# agents:
#   implement-next: { model: opus,   effort: xhigh }
#   status:         { model: sonnet, effort: medium }
```

- [ ] **Step 2: Verify it parses as a comment (no active key)**

Run: `bash sync-agents.sh --check`
Expected: `sync-agents: no agents: block — nothing to check` on stderr, exit 0 (the block is commented, so `block_names` finds nothing).

- [ ] **Step 3: Commit**

```bash
git add .docket.yml
git commit -m "0016: document the agents: model/effort schema in .docket.yml"
```

---

## Task 5: Document the agent layer in `docket-convention`

**Files:**
- Modify: `skills/docket-convention/SKILL.md`
- Test: `tests/test_sync_agents.sh` (append)

- [ ] **Step 1: Append the failing sentinel tests**

Append BEFORE the final `exit $fail`:

```bash
# ---- Task 5: docket-convention documents the agent layer -------------------
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "convention documents the agents: config block" 'grep -q "agents:" "$CONV"'
assert "convention names the generator sync-agents.sh" 'grep -q "sync-agents.sh" "$CONV"'
assert "convention states the precedence" 'grep -qi "per-repo > global > built-in" "$CONV"'
assert "convention states auto => omit effort" 'grep -qi "auto" "$CONV" && grep -qi "omit" "$CONV"'
assert "convention states abort-and-report for autonomous subagents" 'grep -qi "abort-and-report" "$CONV"'
assert "convention points at composition (0017)" 'grep -q "0017" "$CONV"'
# Non-vacuous guard: the agent section must be a distinct heading, not an incidental word.
assert "convention has an agent-layer section heading" 'grep -qiE "^#+ .*(agent layer|model/effort|subagent)" "$CONV"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_sync_agents.sh`
Expected: FAIL — `convention names the generator sync-agents.sh` etc. are NOT OK.

- [ ] **Step 3: Add the documentation**

First, in the `.docket.yml` example inside `skills/docket-convention/SKILL.md` (the fenced `yaml` config block near the top), add an `agents:` line so the documented schema includes it. Find the line `github_project:` in that block and add directly below it:

```yaml
agents:                      # per-skill subagent model/effort (change 0016); see "Agent layer" below
```

Then add a new `###` section. Place it immediately AFTER the `### Configuration — .docket.yml (optional, committed on the default branch)` subsection (before `### Directory layout`). Use this content:

```markdown
### Agent layer — model/effort-pinned subagents (change 0016)

Each **autonomous** docket skill can run as a model/effort-pinned **subagent** instead of inline at the session model. Five skills get a wrapper — `docket-implement-next`, `docket-auto-groom`, `docket-finalize-change`, `docket-status`, `docket-adr`; the two **interactive** skills (`docket-new-change`, `docket-groom-next`) stay inline and only surface an **advisory** recommended model/effort at startup (a skill cannot force the session model). `docket-convention` is not an agent — it is injected into every wrapper via `skills:`.

A wrapper is a thin file: it pins `model` + `effort`, injects the skill via `skills: [<skill>, docket-convention]`, and adds a one-line directive. The skill body stays the single source of behavior. Because a subagent cannot pause to ask a human, every autonomous wrapper carries an **abort-and-report** rule: an unmet precondition or blocking ambiguity (e.g. finalize finding a PR not actually approved, a merge conflict, or a dirty worktree) is surfaced and stopped on — never turned into an interactive prompt.

**Layered config (precedence: per-repo > global > built-in).** Frontmatter is static, so configurability is a **generator** — `sync-agents.sh` — that resolves layers and writes agent files (generated copies it owns and overwrites, unlike `link-skills.sh`'s symlinks):

| Layer | Source | Generates |
|---|---|---|
| Built-in | `agents/docket-*.md` shipped in docket (each ships its default model/effort) | — |
| Global | `~/.config/docket/agents.yaml` (optional, XDG) | user-level `~/.claude/agents/docket-*.md` |
| Per-repo | `.docket.yml` `agents:` block (committed) | **project-level** `<repo>/.claude/agents/docket-*.md` |

```yaml
agents:
  implement-next: { model: opus,   effort: xhigh }
  status:         { model: sonnet, effort: medium }
  # unlisted -> built-in default; effort: auto (or omitted) -> omit the effort line (inherit model default)
```

User-level files are built-in ⊕ global; project-level files are built-in ⊕ per-repo. Claude Code applies **project-over-user precedence natively**, so the effective order is per-repo > global > built-in without the generator merging three layers per file — and because the per-repo overrides generate **committed** project-level files, the same autonomous change builds on the same model for every clone (the reproducibility guarantee). An agent with neither a built-in nor a config entry defaults to `model: inherit` with no `effort`.

`sync-agents.sh` runs **on demand** (install time, and after editing any config layer) — the same mental model as `link-skills.sh`; it does NOT hook session start (silently regenerating committed files out of band would race the commits that make overrides clone-identical). The drift backstop is **`sync-agents.sh --check`**, a CI gate that exits non-zero with a diff when committed project-level files fall out of sync with the resolved config.

**Composition (built in change 0017).** Nesting lets sub-invocations run at their own models: `docket-implement-next` will spawn `docket-status` (step 0) and `docket-adr` (step 6) as nested subagents, and `docket-auto-groom` will spawn its critic. Until 0017 lands those sub-invocations still run inline at the parent's model; 0016 only establishes the wrappers, config, and generator so standalone invocation works.
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_sync_agents.sh`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_sync_agents.sh
git commit -m "0016: document the agent layer + agents: config in docket-convention"
```

---

## Task 6: Advisory model/effort for the interactive skills

**Files:**
- Modify: `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`
- Test: `tests/test_sync_agents.sh` (append)

- [ ] **Step 1: Append the failing tests**

Append BEFORE the final `exit $fail`:

```bash
# ---- Task 6: advisory recommendation in the interactive skills -------------
NEWC="$REPO/skills/docket-new-change/SKILL.md"
GROOM="$REPO/skills/docket-groom-next/SKILL.md"
assert "new-change carries an advisory recommendation" 'grep -qi "[Rr]ecommended model" "$NEWC"'
assert "new-change recommends sonnet" 'grep -qi "sonnet" "$NEWC"'
assert "groom-next carries an advisory recommendation" 'grep -qi "[Rr]ecommended model" "$GROOM"'
assert "groom-next recommends sonnet/high" 'grep -qiE "sonnet[^A-Za-z]+high|high[^A-Za-z]+sonnet" "$GROOM"'
# Non-vacuous: it must be advisory, not a hard requirement (we cannot force the session model).
assert "new-change frames it as advisory" 'grep -qi "advisory" "$NEWC"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_sync_agents.sh`
Expected: FAIL — the recommendation lines do not exist yet.

- [ ] **Step 3: Add the advisory note to each interactive skill**

Both skills open with an `## Overview` (or similar) then a `## Convention (load first — blocking)` section. Insert the advisory note as a new short block immediately AFTER the `## Overview` section heading content and BEFORE `## Convention` in each file.

In `skills/docket-new-change/SKILL.md`, insert:

```markdown
## Recommended model/effort (advisory)

This skill brainstorms with a human, so it cannot be a fire-and-forget subagent and cannot force the session model. **Recommended: `sonnet`, effort: model default** (wide variance from a trivial stub to a full brainstorm). Set `/model sonnet` to match; this is advisory only — the human owns the session.
```

In `skills/docket-groom-next/SKILL.md`, insert:

```markdown
## Recommended model/effort (advisory)

This skill grooms interactively with a human, so it cannot be a fire-and-forget subagent and cannot force the session model. **Recommended: `sonnet` / `high`** (the cold-start recap is genuine synthesis). Set `/model sonnet` and `/effort high` to match; this is advisory only — the human owns the session.
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_sync_agents.sh`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-new-change/SKILL.md skills/docket-groom-next/SKILL.md tests/test_sync_agents.sh
git commit -m "0016: advisory model/effort recommendation for the interactive skills"
```

---

## Task 7: README install + stale-enumeration sweep + full suite

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Sweep for stale enumerations**

Run these and read the output (learning: adding a member to an enumerated set leaves stale counts/lists):

```bash
grep -rnE "link-skills\.sh|migrate-to-docket\.sh" README.md
grep -rniE "two scripts|both scripts|install|\.sh" README.md | head -40
```

Confirm whether any sentence enumerates docket's scripts (e.g. "link-skills.sh and migrate-to-docket.sh"). If one does, it must also mention `sync-agents.sh` after the edit below.

- [ ] **Step 2: Add `sync-agents.sh` to the Install section**

In `README.md`, the Install section currently shows a single `bash ~/dev/docket/link-skills.sh` block followed by a paragraph. Update the command block to:

```bash
bash ~/dev/docket/link-skills.sh
bash ~/dev/docket/sync-agents.sh
```

And add this paragraph immediately after the existing `link-skills.sh` paragraph (the one ending "available in every project you open."):

```markdown
`sync-agents.sh` generates docket's model/effort-pinned subagent wrappers from layered config (built-in defaults ⊕ `~/.config/docket/agents.yaml` ⊕ a repo's `.docket.yml agents:` block) into each present harness's `agents/` directory, and writes committed project-level wrappers for any repo that pins per-skill overrides. Unlike the symlinks `link-skills.sh` creates, these are generated copies — re-run it after editing any config layer, and run `sync-agents.sh --check` in CI to fail on drift.
```

- [ ] **Step 3: Add `agents:` to the README `.docket.yml` example**

In the fenced `.docket.yml` example in `README.md`, add (after the `github_project:` line, matching the convention doc):

```yaml
agents:                      # per-skill subagent model/effort (see "Agent layer" in docket-convention)
```

- [ ] **Step 4: Run the FULL test suite**

```bash
for t in tests/test_*.sh; do echo "== $t =="; bash "$t" || echo "SUITE FAIL: $t"; done
```

Expected: every test file ends with all `ok -` lines and no `SUITE FAIL:` / `NOT OK` lines. The new `tests/test_sync_agents.sh` and all pre-existing suites must pass.

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "0016: README — sync-agents.sh install step + agents: config example"
```

---

## Self-Review (run after all tasks)

**1. Spec coverage** (against `2026-06-15-docket-subagent-model-effort-design.md`):
- §3 two mechanisms (autonomous subagent vs advisory interactive) → Tasks 1, 6.
- §4 wrapper form + built-in default table → Task 1 (+ tests assert the table verbatim).
- §5 layered config + precedence + reproducibility → Tasks 2, 4, 5.
- §6 composition pointer (built in 0017, not here) → documented in Task 5; no code.
- §7 docket-convention changes → Task 5.
- §8 generator + `--check` + on-demand lifecycle → Tasks 2, 3 (+ doc in Tasks 4, 5).
- §9 testing strategy (generation / precedence / idempotency / `--check` / wrapper validity / advisory-skip) → Tasks 1–3, 6.
- §11 scope: 5 wrappers, config schema + precedence, generator, advisory, convention update → all tasks.

**2. Placeholder scan:** none — every step ships complete file content or exact insert anchors.

**3. Type/name consistency:** wrapper filenames `docket-<skill>.md`; config short-names `<skill>` (no `docket-` prefix); functions `entry_line`/`field_of`/`block_names`/`resolve_from`/`emit`/`user_level_pass`/`project_level_pass`/`check_project_level`; seam `DOCKET_HARNESS_ROOT`; resolved vars `RES_MODEL`/`RES_EFFORT` — used identically across Tasks 2–3.

**Out of scope (do NOT do here):** rewiring sub-invocations to nested subagents (that is 0017); editing the autonomous skill BODIES (only wrappers + the 2 interactive skills' advisory notes); changing `link-skills.sh`; wiring a real CI workflow (none exists in the repo; `--check` is provided for when one is added).
