<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0242 — Close the Claude gap in the run-completion gate with a caller-side verify in the dispatch rules](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0242-close-the-claude-gap-in-the-run-completion-gate-with-a-comma.md)**
<!-- docket:backlink:end -->

# Close the Claude gap in the run-completion gate — caller-side verify Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver change 0237's `verify-run` gate to the *parent* session of every enabled harness — including Claude, which today has no generated parent-facing surface at all — by single-sourcing the gate text and having `sync-agents.sh` create and maintain a `CLAUDE.md` surface.

**Architecture:** Three seams, in order. (1) The gate procedure becomes one template file, `cursor-rules/run-gate.md`, emitted by one function and spliced into both existing assembly paths — `assemble_dispatch_rule()` (the Cursor `.mdc`) and `assemble_agents_md_dispatch()` (the committed `AGENTS.md` block) — so the gate cannot fork into diverging copies. (2) `sync-agents.sh` gains Claude-surface resolution: when `claude` is a targeted harness, resolve `CLAUDE.md` to a physical file (existing file's `realpath`; else a committed symlink to `AGENTS.md`; else a seeded real file), then write the managed `docket:dispatch` block **once per distinct physical file** across the AGENTS.md and Claude targets. (3) `--check` learns the same target set, and the convention gains a one-sentence pointer.

**Tech Stack:** Bash 4.4+ (`sync-agents.sh`, `scripts/lib/docket-gitignore-block.sh`), the repo's own hermetic test suite (`tests/test_*.sh`, run by `scripts/run-tests.sh`), markdown template fragments under `cursor-rules/`.

## Global Constraints

- **Bash floor: 4.4.** `sync-agents.sh` already runs under `DOCKET_BASH_PATH`; no feature above 4.4.
- **Shell portability (AGENTS.md, promoted learning):** no GNU-only flags. macOS BSD userland is the target. `realpath` is **not** available on stock macOS as a coreutils binary in all cases — resolve symlinks with a Bash helper (`cd -P`/`pwd -P`), never a bare `realpath` call.
- **`pipefail` (AGENTS.md, promoted learning):** every new pipeline in a script that sets `-o pipefail` must be checked; `sync-agents.sh` sets `set -uo pipefail`.
- **Marker-block range edits (AGENTS.md, promoted learning):** managed blocks are written only through `ensure_managed_block` / `remove_managed_block` in `scripts/lib/docket-gitignore-block.sh`. Never hand-splice markers.
- **Guards are code (AGENTS.md, promoted learning):** every assert added below must be mutation-proven — break the thing it guards, watch it go red, restore.
- **Brevity is a build requirement** (spec §1): the gate block rides always-loaded context in every enabled harness. The gate text is a **maximum of 14 rendered lines**.
- **The gate has exactly one template source** (spec §2). Two rendered copies of the gate text must be byte-identical; a test asserts this by comparison, not by reading.
- **Nothing in `verify-run.sh`, `runner-dispatch.sh`, `board-checks.sh`, or `context: fork` frontmatter is modified** (spec §4 / change `## Out of scope`).
- **`CLAUDE.md` is committed**, never added to the docket `.gitignore` block. `.cursor/rules/docket-dispatch.mdc` stays ignored.
- **New test files require a budget row** in `tests/runtime-budgets.tsv` or `tests/test_runtime_budgets.sh` fails.
- Run the suite with `scripts/run-tests.sh`; a single file with `bash tests/<file>.sh`.

---

## File Structure

| File | Responsibility |
|---|---|
| `cursor-rules/run-gate.md` | **Create.** The single source of the caller-side gate text. Harness-neutral prose, no path or product names. |
| `sync-agents.sh` | **Modify.** Add `assemble_run_gate()`; splice it into `assemble_dispatch_rule()` and `assemble_agents_md_dispatch()`. Add `resolve_physical_path()`, `repo_wants_claude_surface()`, `claude_surface_target()`, and `sync_dispatch_surfaces()` (which replaces `sync_agents_md_dispatch()`); extend `project_level_pass()` and `check_project_level()` with the deduped target set. |
| `tests/test_sync_agents_run_gate.sh` | **Create.** Gate text single-sourcing, presence in both rendered surfaces, byte-identity, the four behavioral claims, brevity. |
| `tests/test_sync_agents_claude_surface.sh` | **Create.** The four repo combos × harness sets, symlink creation, `realpath` dedupe writing exactly once, non-overwrite of an existing `CLAUDE.md`, `--check` parity. |
| `tests/runtime-budgets.tsv` | **Modify.** One budget row per new test file. |
| `skills/docket-convention/SKILL.md` | **Modify.** One sentence in *Composition* pointing at the managed-block gate. |
| `docs/results/2026-08-08-close-the-claude-gap-caller-side-run-gate-results.md` | **Create (Task 6).** Records the human-verification item (live parent compliance) that no in-repo test can be an oracle for. |

---

### Task 1: The gate text, single-sourced

Creates the one template source and proves it renders identically into both existing surfaces. This is the task the `duplicated-gate-copies-the-whole-predicate` and `consolidation-flattens-caller-variance` findings bear on directly: the gate is a *whole predicate*, and the two rendering callers must not be flattened into one another — only the gate text is shared, the surrounding per-harness prose is untouched.

**Files:**
- Create: `cursor-rules/run-gate.md`
- Modify: `sync-agents.sh` (add `assemble_run_gate()` next to `assemble_dispatch_rule()` at ~:1208; call it in `assemble_dispatch_rule()` and `assemble_agents_md_dispatch()`)
- Create: `tests/test_sync_agents_run_gate.sh`
- Modify: `tests/runtime-budgets.tsv`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `assemble_run_gate()` — a Bash function taking no arguments, printing the gate text to stdout with **no** leading or trailing blank line. Tasks 2–3 rely on the gate already being inside `assemble_agents_md_dispatch()`'s output, so the Claude surface inherits it for free.

- [ ] **Step 1: Write the failing test**

Create `tests/test_sync_agents_run_gate.sh`:

```bash
#!/usr/bin/env bash
# tests/test_sync_agents_run_gate.sh — the caller-side run gate is single-sourced and rendered
# identically into every parent-facing surface (change 0242).
# run: bash tests/test_sync_agents_run_gate.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
GATE_SRC="$REPO/cursor-rules/run-gate.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Collapse runs of whitespace so an assert about a CLAIM survives a pure re-flow of the prose
# (learnings: phrase-grep-over-wrapped-prose).
flat(){ tr '\n' ' ' < "$1" | tr -s '[:space:]' ' '; }

mk_repo(){  # $1 = agent_harnesses list body, e.g. "[claude, codex]"
  SBX="$(mktemp -d)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  printf 'agent_harnesses: %s\n' "$1" > "$SBX/.docket.yml"
  ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
}

# --- the template source exists and is the ONLY place the gate text is authored ---
assert "run-gate template exists" '[ -f "$GATE_SRC" ]'
# The gate's own commands must not be hand-copied into either assembler. Anchored on the
# distinctive flag, which appears in the template and nowhere else in the generator.
assert "sync-agents.sh does not restate the gate commands inline" \
  '[ "$(grep -c -- "--in-progress-ids" "$SYNC")" = "0" ]'

# --- brevity: the block rides always-loaded context in every harness (spec Risks) ---
assert "gate text is at most 14 lines" \
  '[ "$(grep -c "" "$GATE_SRC")" -le 14 ]'

# --- the four behavioral claims, each bound to what it is asserted ABOUT ---
# (learnings: prose-guard-binds-phrase-to-claim — never a bare phrase-presence grep)
G="$(flat "$GATE_SRC")"
assert "gate: snapshots the in-progress set BEFORE dispatching" \
  '[[ "$G" == *"Before dispatching"*"verify-run --in-progress-ids"* ]]'
assert "gate: verifies the attributed id after the return" \
  '[[ "$G" == *"After"*"verify-run <id>"* ]]'
assert "gate: run-halted never re-dispatches" \
  '[[ "$G" == *"run-halted"*"never re-dispatch"* ]]'
assert "gate: run-incomplete re-dispatches exactly ONCE, then stops" \
  '[[ "$G" == *"run-incomplete"*"once"* ]] && [[ "$G" == *"Never a third"* ]]'

# --- rendered into BOTH surfaces, byte-identically ---
mk_repo "[cursor, codex]"
CUR="$SBX/.cursor/rules/docket-dispatch.mdc"
AGM="$SBX/AGENTS.md"
assert "cursor rule was generated"   '[ -f "$CUR" ]'
assert "AGENTS.md block was written" '[ -f "$AGM" ]'

# Slice the gate out of each rendered surface by its own heading, terminated by a NAMED
# terminator (learnings: section-slice-needs-a-named-terminator).
slice_gate(){ awk '/^## Run gate/{g=1} g&&/^<!-- docket:dispatch:end/{exit} g{print}' "$1"; }
slice_gate "$CUR" > "$SBX/.gate-cursor"
slice_gate "$AGM" > "$SBX/.gate-agents"
assert "gate is present in the cursor rule"    '[ -s "$SBX/.gate-cursor" ]'
assert "gate is present in the AGENTS.md block" '[ -s "$SBX/.gate-agents" ]'
assert "the two rendered gates are byte-identical" \
  'diff -q "$SBX/.gate-cursor" "$SBX/.gate-agents" >/dev/null'
# The terminator the slice depends on must actually exist, or the slice silently runs to EOF.
assert "the AGENTS.md dispatch end marker exists" 'grep -q "docket:dispatch:end" "$AGM"'

echo; [ "$fail" = 0 ] && echo "ALL PASS" || echo "FAILURES"; exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_sync_agents_run_gate.sh`
Expected: FAIL — `NOT OK - run-gate template exists` (and the render asserts fail because no gate is emitted).

- [ ] **Step 3: Write the gate template**

Create `cursor-rules/run-gate.md` — harness-neutral, no product names, no paths. Exactly this content:

```markdown
## Run gate — verify a dispatched implement-next run before you report it

A dispatched run that stops early returns a report that reads as success. Do not trust it; read git.

1. **Before dispatching** `docket-implement-next`, snapshot the claimed set:
   `docket.sh verify-run --in-progress-ids`.
2. Dispatch and block on the return, as above.
3. **After the return**, re-run `docket.sh verify-run --in-progress-ids`. Any id absent from the
   snapshot is this run's claim; an empty diff (drained, or a lost claim race) ends the gate.
4. Run `docket.sh verify-run <id>` and key on its report line, never its exit code:
   - `run-complete` / `run-unclaimed` — done.
   - `run-halted` — done; **never re-dispatch** a halt, which means a human is needed.
   - `run-incomplete` — re-dispatch the same agent **once**, passing the id and the unmet
     conjuncts; verify again; if still incomplete, stop and report loudly. Never a third dispatch.
```

- [ ] **Step 4: Emit it from one function, spliced into both assemblers**

In `sync-agents.sh`, immediately **above** `assemble_dispatch_rule()` (currently line ~1208), add:

```bash
# The caller-side run gate (change 0242). ONE source — cursor-rules/run-gate.md — rendered verbatim
# into every parent-facing surface: the Cursor rule, the committed AGENTS.md block, and (change
# 0242) the Claude surface, which inherits it through the AGENTS.md block assembler. The gate is a
# whole predicate, not a threshold: a second hand-written copy would agree on the ordinary case and
# diverge on exactly the halted/incomplete states it exists to distinguish (LEARNINGS
# duplicated-gate-copies-the-whole-predicate). Printed with no surrounding blank line; each caller
# owns its own spacing.
assemble_run_gate() { cat "$CURSOR_RULES_SRC/run-gate.md"; }
```

In `assemble_dispatch_rule()`, after the `cat "$CURSOR_RULES_SRC/dispatch.head.md"` line (~:1212), insert:

```bash
  printf '\n'
  assemble_run_gate
```

In `assemble_agents_md_dispatch()`, replace the lone `printf '\n'` that currently follows the `HEAD` heredoc (~:1273) with:

```bash
  printf '\n'
  assemble_run_gate
  printf '\n'
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_sync_agents_run_gate.sh`
Expected: PASS — `ALL PASS`.

- [ ] **Step 6: Mutation-prove the two load-bearing asserts**

Guards are code. For each, break it, confirm RED, restore:

```bash
# (a) single-sourcing: hand-copy a gate command into the generator
cp sync-agents.sh /tmp/sync.bak
printf '\n# --in-progress-ids\n' >> sync-agents.sh
bash tests/test_sync_agents_run_gate.sh   # expect: NOT OK - sync-agents.sh does not restate ...
cp /tmp/sync.bak sync-agents.sh

# (b) byte-identity: make the cursor path render a second, edited copy
cp cursor-rules/run-gate.md /tmp/gate.bak
# temporarily change assemble_dispatch_rule to `sed 's/once/twice/' "$CURSOR_RULES_SRC/run-gate.md"`
# then:
bash tests/test_sync_agents_run_gate.sh   # expect: NOT OK - the two rendered gates are byte-identical
# restore sync-agents.sh from git
git checkout -- sync-agents.sh
```

Expected: each mutation reddens exactly the named assert; the restored tree is green.

- [ ] **Step 7: Add the budget row**

In `tests/runtime-budgets.tsv`, in the existing sorted position among the `tests/test_sync_agents_*` rows, add (TAB-separated):

```
tests/test_sync_agents_run_gate.sh	20	parallel
```

- [ ] **Step 8: Run the affected suites**

Run: `bash scripts/run-tests.sh tests/test_sync_agents_run_gate.sh tests/test_sync_agents_cursor.sh tests/test_sync_agents_codex.sh tests/test_sync_agents_opencode.sh tests/test_runtime_budgets.sh`
Expected: every file passes. If `test_sync_agents_codex.sh` or `test_sync_agents_opencode.sh` pinned the AGENTS.md block's exact body, update those asserts to accommodate the gate — they assert *claims*, not byte length; do not weaken them.

- [ ] **Step 9: Commit**

```bash
git add cursor-rules/run-gate.md sync-agents.sh tests/test_sync_agents_run_gate.sh tests/runtime-budgets.tsv
git commit -m "feat(0242): single-source the caller-side run gate into both dispatch surfaces"
```

---

### Task 2: Resolve and create the Claude parent surface

`sync-agents.sh` learns to find — or create — the one physical file a Claude parent session always loads. Pure resolution logic plus its write; wiring into the passes is Task 3, so this task's intermediate state is itself testable (learnings: `intermediate-task-state-buildable`).

**Files:**
- Modify: `sync-agents.sh` (add three functions after `repo_wants_agents_md_dispatch()`, ~:270)
- Create: `tests/test_sync_agents_claude_surface.sh`
- Modify: `tests/runtime-budgets.tsv`

**Interfaces:**
- Consumes: `assemble_agents_md_dispatch()` and `assemble_run_gate()` from Task 1; `ensure_managed_block` / `remove_managed_block` / `_docket_gi_current_block` from `scripts/lib/docket-gitignore-block.sh`; `DISPATCH_START` / `DISPATCH_END`; `HARNESSES`; `REPO`.
- Produces:
  - `resolve_physical_path <path>` — prints the fully symlink-resolved absolute path of `<path>`, or the path itself when it does not exist. Never fails on a missing file.
  - `repo_wants_claude_surface` — returns 0 when `claude` is in `$HARNESSES`.
  - `claude_surface_target` — prints the absolute path of the file the Claude block must be written **into**, creating a symlink or a seeded file as a side effect when neither `CLAUDE.md` nor a resolvable target exists. Prints nothing and returns 1 when the repo does not want the surface.

- [ ] **Step 1: Write the failing test**

Create `tests/test_sync_agents_claude_surface.sh`:

```bash
#!/usr/bin/env bash
# tests/test_sync_agents_claude_surface.sh — docket creates and maintains the Claude parent-facing
# instruction surface, one managed block per distinct PHYSICAL file (change 0242).
# run: bash tests/test_sync_agents_claude_surface.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# $1 = agent_harnesses body; $2 = combo: none|agents|claude|both|symlinked
mk_repo(){
  SBX="$(mktemp -d)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  printf 'agent_harnesses: %s\n' "$1" > "$SBX/.docket.yml"
  case "$2" in
    agents)    printf '# Repo instructions\n'  > "$SBX/AGENTS.md" ;;
    claude)    printf '# Repo instructions\n'  > "$SBX/CLAUDE.md" ;;
    both)      printf '# A\n' > "$SBX/AGENTS.md"; printf '# C\n' > "$SBX/CLAUDE.md" ;;
    symlinked) printf '# Repo instructions\n'  > "$SBX/AGENTS.md"
               ( cd "$SBX" && ln -s AGENTS.md CLAUDE.md ) ;;
    none)      : ;;
  esac
  ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
}
blocks_in(){ grep -c "docket:dispatch:start" "$1" 2>/dev/null || echo 0; }

# --- claude + AGENTS.md only: the symlink is created, ONE physical file, ONE block ---
mk_repo "[claude, codex]" agents
assert "AGENTS-only: CLAUDE.md is created"            '[ -e "$SBX/CLAUDE.md" ]'
assert "AGENTS-only: CLAUDE.md is a symlink"          '[ -L "$SBX/CLAUDE.md" ]'
assert "AGENTS-only: symlink points at AGENTS.md"     '[ "$(readlink "$SBX/CLAUDE.md")" = "AGENTS.md" ]'
assert "AGENTS-only: exactly ONE dispatch block"      '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'
assert "AGENTS-only: the block carries the run gate"  'grep -q "verify-run --in-progress-ids" "$SBX/AGENTS.md"'

# --- claude alone (no AGENTS.md-dispatch harness): a real CLAUDE.md is seeded ---
mk_repo "[claude]" none
assert "claude-only/neither: CLAUDE.md is created"       '[ -f "$SBX/CLAUDE.md" ]'
assert "claude-only/neither: it is a REAL file, no link" '[ ! -L "$SBX/CLAUDE.md" ]'
assert "claude-only/neither: it carries the gate"        'grep -q "verify-run --in-progress-ids" "$SBX/CLAUDE.md"'
assert "claude-only/neither: no AGENTS.md is created"    '[ ! -e "$SBX/AGENTS.md" ]'

# --- an existing CLAUDE.md is written INTO, never replaced ---
mk_repo "[claude]" claude
assert "existing CLAUDE.md: still a real file"     '[ -f "$SBX/CLAUDE.md" ] && [ ! -L "$SBX/CLAUDE.md" ]'
assert "existing CLAUDE.md: pre-existing content survives" 'grep -q "Repo instructions" "$SBX/CLAUDE.md"'
assert "existing CLAUDE.md: gains exactly one block" '[ "$(blocks_in "$SBX/CLAUDE.md")" = "1" ]'

# --- already symlinked: realpath dedupe writes the block exactly ONCE ---
mk_repo "[claude, codex]" symlinked
assert "symlinked: AGENTS.md has exactly one block" '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'
# CLAUDE.md IS AGENTS.md here; a second write would have appended a second block to the same inode.
assert "symlinked: reading through the link shows one block" '[ "$(blocks_in "$SBX/CLAUDE.md")" = "1" ]'
assert "symlinked: the link was not replaced by a file" '[ -L "$SBX/CLAUDE.md" ]'

# --- two DISTINCT physical files each get their own block ---
mk_repo "[claude, codex]" both
assert "distinct files: AGENTS.md has a block" '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'
assert "distinct files: CLAUDE.md has a block" '[ "$(blocks_in "$SBX/CLAUDE.md")" = "1" ]'
assert "distinct files: CLAUDE.md content survived" 'grep -q "^# C$" "$SBX/CLAUDE.md"'

# --- claude NOT targeted: no Claude surface is created or touched ---
mk_repo "[codex]" agents
assert "no claude: CLAUDE.md is not created" '[ ! -e "$SBX/CLAUDE.md" ]'
assert "no claude: AGENTS.md still gets its block" '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'

echo; [ "$fail" = 0 ] && echo "ALL PASS" || echo "FAILURES"; exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_sync_agents_claude_surface.sh`
Expected: FAIL — `NOT OK - AGENTS-only: CLAUDE.md is created` and every following surface assert.

- [ ] **Step 3: Write the resolution + creation logic**

In `sync-agents.sh`, immediately after `repo_wants_agents_md_dispatch()` and the `DISPATCH_START`/`DISPATCH_END` assignments (~:272), add:

```bash
# --- the Claude parent-facing surface (change 0242) ---------------------------
# ADR-0024 solved Claude's *routing* natively (context: fork — "no generated file"), so no
# parent-facing Claude surface was ever built. The run gate is not routing: it is what the parent
# does AFTER a routed run returns, and it needs an always-loaded surface to live on. Claude Code's
# documented always-loaded file is CLAUDE.md; we target that documented surface rather than betting
# on a given version also reading AGENTS.md (LEARNINGS harness-behavior-is-mode-and-version-scoped).
repo_wants_claude_surface(){ case " $HARNESSES " in *" claude "*) return 0;; *) return 1;; esac; }

# Print $1 with every symlink resolved, absolute. A missing path prints its own absolute form.
# Bash-only: stock macOS has no coreutils `realpath`, and `readlink -f` is GNU-only.
resolve_physical_path(){  # $1 = path
  local p="$1" d b
  d="$(dirname "$p")"; b="$(basename "$p")"
  d="$(cd "$d" 2>/dev/null && pwd -P)" || { printf '%s\n' "$p"; return 0; }
  # Walk the link chain by hand; bounded so a symlink cycle cannot hang the sync.
  local n=0
  while [ -L "$d/$b" ] && [ "$n" -lt 32 ]; do
    local t; t="$(readlink "$d/$b")"
    case "$t" in
      /*) d="$(dirname "$t")"; b="$(basename "$t")" ;;
      *)  d="$(cd "$d" && cd "$(dirname "$t")" 2>/dev/null && pwd -P)" || break
          b="$(basename "$t")" ;;
    esac
    n=$((n+1))
  done
  printf '%s/%s\n' "$d" "$b"
}

# Print the physical file the Claude block must be written into, creating the surface when absent.
# Three cases, in the spec's order:
#   CLAUDE.md exists (file or symlink) -> its physical path; never replaced, only written into.
#   absent, AGENTS.md present          -> create CLAUDE.md as a committed relative symlink to it,
#                                         so Claude loads ONE physical instructions file (the gate
#                                         AND everything else AGENTS.md carries, e.g. promoted
#                                         learnings), identically to codex/opencode.
#   neither                            -> create a real, empty CLAUDE.md to seed the block into.
claude_surface_target(){
  repo_wants_claude_surface || return 1
  local c="$REPO/CLAUDE.md"
  if [ ! -e "$c" ] && [ ! -L "$c" ]; then
    if [ -e "$REPO/AGENTS.md" ]; then
      ( cd "$REPO" && ln -s AGENTS.md CLAUDE.md ) || return 1
      log "created CLAUDE.md as a symlink to AGENTS.md (one physical instructions file) — COMMIT THIS."
    else
      : > "$c" || return 1
      log "created CLAUDE.md to carry the docket dispatch block — COMMIT THIS."
    fi
  fi
  resolve_physical_path "$c"
}
```

- [ ] **Step 4: Write the block into the deduped target set**

Replace `sync_agents_md_dispatch()` (~:1289) wholesale with a function that owns **both** targets and dedupes by physical path:

```bash
# Write the managed dispatch block into every parent-facing surface this repo targets, ONCE PER
# DISTINCT PHYSICAL FILE. A CLAUDE.md symlinked to AGENTS.md is one file, and writing it twice would
# append two diverging copies to the same inode (LEARNINGS decide-and-act-on-the-same-copy: decide
# from the copy you will actually write). Strip on de-list, per target, unchanged from change 0077.
sync_dispatch_surfaces(){
  local want block f phys seen="" status
  block="$(assemble_agents_md_dispatch)"

  local targets=""
  if repo_wants_agents_md_dispatch; then targets="$REPO/AGENTS.md"; fi
  if repo_wants_claude_surface; then
    f="$(claude_surface_target)" && targets="$targets${targets:+$'\n'}$f"
  fi

  # Write pass — deduped by physical path.
  if [ -n "$targets" ]; then
    while IFS= read -r f; do
      [ -n "$f" ] || continue
      phys="$(resolve_physical_path "$f")"
      case "$seen" in *"|$phys|"*) continue ;; esac
      seen="$seen|$phys|"
      status="$(ensure_managed_block "$phys" "$DISPATCH_START" "$DISPATCH_END" "$block")"
      case "$status" in
        wrote)   log "wrote/updated the docket dispatch block in $phys — COMMIT THIS (machine-neutral; no model IDs).";;
        refused) log "WARN $phys has a malformed docket:dispatch block — refusing to rewrite; repair the markers by hand and re-run.";;
      esac
    done <<<"$targets"
  fi

  # Strip pass — a surface whose harness is no longer targeted loses its block.
  if ! repo_wants_agents_md_dispatch && ! repo_wants_claude_surface; then
    for f in "$REPO/AGENTS.md" "$REPO/CLAUDE.md"; do
      [ -e "$f" ] || continue
      phys="$(resolve_physical_path "$f")"
      status="$(remove_managed_block "$phys" "$DISPATCH_START" "$DISPATCH_END")"
      case "$status" in
        removed) log "removed the docket dispatch block from $phys (no dispatch harness targeted) — COMMIT THIS.";;
        refused) log "WARN $phys has a malformed docket:dispatch block — refusing to strip; repair the markers by hand.";;
      esac
    done
  fi
}
```

Note: this task only *defines* the function. `project_level_pass()` still calls the old name until Task 3, so keep `sync_agents_md_dispatch(){ sync_dispatch_surfaces; }` as a one-line shim for now — the intermediate tree stays green.

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_sync_agents_claude_surface.sh`
Expected: PASS — `ALL PASS`.

- [ ] **Step 6: Mutation-prove the dedupe assert**

The dedupe is the whole point of the symlink combo. Break it and confirm RED:

```bash
cp sync-agents.sh /tmp/sync.bak
# In sync_dispatch_surfaces, delete the two lines:
#   case "$seen" in *"|$phys|"*) continue ;; esac
#   seen="$seen|$phys|"
bash tests/test_sync_agents_claude_surface.sh
# expect: NOT OK - symlinked: AGENTS.md has exactly one block
cp /tmp/sync.bak sync-agents.sh
```

Expected: the named assert reddens; restored tree is green.

- [ ] **Step 7: Add the budget row**

In `tests/runtime-budgets.tsv`, add in sorted position:

```
tests/test_sync_agents_claude_surface.sh	30	parallel
```

- [ ] **Step 8: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents_claude_surface.sh tests/runtime-budgets.tsv
git commit -m "feat(0242): create and maintain the Claude parent-facing dispatch surface"
```

---

### Task 3: Wire the surface into the passes and `--check`

Retires the shim, and teaches `--check` the same target set — otherwise CI asserts currency for `AGENTS.md` while the Claude surface drifts silently. This is the `correspondence-guard-runs-one-way` shape: the write pass and the check pass must iterate the *same* set.

**Files:**
- Modify: `sync-agents.sh` — `project_level_pass()` (~:1467), `check_project_level()` (~:1489-1505)
- Modify: `tests/test_sync_agents_claude_surface.sh` (append the `--check` block)

**Interfaces:**
- Consumes: `sync_dispatch_surfaces`, `claude_surface_target`, `resolve_physical_path`, `repo_wants_claude_surface` from Task 2.
- Produces: nothing new; `--check` exit code 1 on a stale or missing Claude-surface block.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_sync_agents_claude_surface.sh`, **before** the final `echo; [ "$fail" = 0 ]` line:

```bash
# --- --check covers the Claude surface, not only AGENTS.md ---
mk_repo "[claude]" none
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 )
assert "check: clean tree passes" '[ "$?" = "0" ]'

# Staleness on the CLAUDE-only surface must FAIL the check. Corrupt the block's body, keeping
# the markers intact so this exercises staleness, not the malformed-marker path.
perl -0pi -e 's/(docket:dispatch:start[^\n]*\n)/$1STALE\n/' "$SBX/CLAUDE.md" 2>/dev/null \
  || sed -i '' -e "/docket:dispatch:start/a\\
STALE" "$SBX/CLAUDE.md"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 )
assert "check: a stale Claude-surface block fails the check" '[ "$?" = "1" ]'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_sync_agents_claude_surface.sh`
Expected: FAIL — `NOT OK - check: a stale Claude-surface block fails the check` (the check leg only looks at `AGENTS.md` today).

- [ ] **Step 3: Retire the shim in the write pass**

In `project_level_pass()` (~:1467), replace:

```bash
  sync_agents_md_dispatch
```

with:

```bash
  sync_dispatch_surfaces
```

Then delete the one-line `sync_agents_md_dispatch(){ sync_dispatch_surfaces; }` shim added in Task 2, and confirm no other caller remains: `grep -n 'sync_agents_md_dispatch' sync-agents.sh` must print nothing.

- [ ] **Step 4: Extend the check leg to the same target set**

In `check_project_level()`, replace the `am_want`/`am_have` block (~:1492-1505) with a loop over the same deduped set the write pass uses:

```bash
  # Dispatch-surface currency (changes 0077, 0242) — CI-meaningful, symmetric with the .gitignore
  # leg. Iterates the SAME target set the write pass does: a check that walked only AGENTS.md would
  # certify a repo whose Claude surface had drifted (LEARNINGS correspondence-guard-runs-one-way).
  local am_want phys seen="" f
  am_want="$(assemble_agents_md_dispatch)"
  if repo_wants_agents_md_dispatch || repo_wants_claude_surface; then
    local targets=""
    repo_wants_agents_md_dispatch && targets="$REPO/AGENTS.md"
    if repo_wants_claude_surface; then targets="$targets${targets:+$'\n'}$REPO/CLAUDE.md"; fi
    while IFS= read -r f; do
      [ -n "$f" ] || continue
      phys="$(resolve_physical_path "$f")"
      case "$seen" in *"|$phys|"*) continue ;; esac
      seen="$seen|$phys|"
      if [ "$am_want" != "$(_docket_gi_current_block "$phys" "$DISPATCH_START" "$DISPATCH_END")" ]; then
        log "check: docket dispatch block in $phys is missing or stale — run: bash sync-agents.sh and commit it"
        rc=1
      fi
    done <<<"$targets"
  else
    for f in "$REPO/AGENTS.md" "$REPO/CLAUDE.md"; do
      [ -e "$f" ] || continue
      phys="$(resolve_physical_path "$f")"
      if [ -n "$(_docket_gi_current_block "$phys" "$DISPATCH_START" "$DISPATCH_END")" ]; then
        log "check: $phys carries a docket dispatch block but no dispatch harness (claude, $(printf '%s' "$AGENTS_MD_DISPATCH_HARNESSES" | sed 's/ /, /g')) is in agent_harnesses — run: bash sync-agents.sh and commit it"
        rc=1
      fi
    done
  fi
```

**Important:** `check_project_level` must NOT create the surface — `--check` is read-only. That is why this loop reads `$REPO/CLAUDE.md` directly rather than calling `claude_surface_target` (which creates). A missing `CLAUDE.md` in a claude-targeted repo therefore reads as an empty current block ≠ `am_want`, and correctly fails the check.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_sync_agents_claude_surface.sh`
Expected: PASS — `ALL PASS`.

- [ ] **Step 6: Verify `--check` created nothing**

```bash
SBX="$(mktemp -d)"; git -C "$SBX" init --quiet
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash sync-agents.sh --check >/dev/null 2>&1 )
ls "$SBX/CLAUDE.md"
```

Expected: `ls` reports no such file — `--check` is read-only.

- [ ] **Step 7: Run the full suite**

Run: `bash scripts/run-tests.sh`
Expected: every file passes. Any red file here is a real regression in a neighboring assert about the AGENTS.md block — fix the code or update the assert to the new, correct claim; never delete an assert to get green.

- [ ] **Step 8: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents_claude_surface.sh
git commit -m "feat(0242): iterate the same dispatch-surface set in the write pass and --check"
```

---

### Task 4: Regenerate this repo's own surface and the convention pointer

Docket's own root is the spec's `AGENTS.md`-only combo, so building this change creates the committed symlink here. The convention gains the one sentence naming the gate as *Composition*'s mechanical form for interactive dispatch.

**Files:**
- Modify: `skills/docket-convention/SKILL.md:105` (the *Composition* paragraph)
- Create: `CLAUDE.md` (symlink → `AGENTS.md`, generated)
- Modify: `AGENTS.md` (regenerated `docket:dispatch` block)

**Interfaces:**
- Consumes: everything from Tasks 1–3.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Add the convention pointer**

In `skills/docket-convention/SKILL.md`, in the *Composition* paragraph, immediately after the sentence ending `**never adopts or commits a child's uncommitted working-tree files**.`, insert:

```
For an interactive dispatch, that verification obligation has a mechanical form: the managed `docket:dispatch` block each harness's parent-facing instructions file carries runs `docket.sh verify-run` on the returning run and re-dispatches an incomplete one exactly once (change 0242).
```

One sentence — a pointer, not a restatement of the gate.

- [ ] **Step 2: Regenerate this repo's surfaces**

```bash
bash sync-agents.sh
git status --short
```

Expected: `CLAUDE.md` appears as a new symlink and `AGENTS.md` shows the regenerated block carrying the gate. The generated `.claude/agents/docket-*.md` wrappers are gitignored and must NOT appear as untracked additions — if they do, stop: the `.gitignore` block is stale.

- [ ] **Step 3: Verify the symlink and the block**

```bash
readlink CLAUDE.md                                   # expect: AGENTS.md
grep -c "docket:dispatch:start" AGENTS.md            # expect: 1
grep -q "verify-run --in-progress-ids" CLAUDE.md && echo gate-reaches-claude
bash sync-agents.sh --check; echo "check rc=$?"      # expect: rc=0
```

Expected: `AGENTS.md`, `1`, `gate-reaches-claude`, `check rc=0`.

- [ ] **Step 4: Confirm CLAUDE.md is not ignored**

```bash
git check-ignore -v CLAUDE.md; echo "rc=$?"
```

Expected: `rc=1` (no ignore rule matches) — the surface is committed, unlike the Cursor rule.

- [ ] **Step 5: Run the full suite**

Run: `bash scripts/run-tests.sh`
Expected: all pass. `tests/test_sync_agents_drift_docs.sh` may assert the repo's own generated artifacts are current — this step is what makes it so.

- [ ] **Step 6: Commit**

```bash
git add CLAUDE.md AGENTS.md skills/docket-convention/SKILL.md
git commit -m "feat(0242): create docket's own Claude surface and point the convention at the gate"
```

---

### Task 5: Guard the claims the prose now makes

Two claims shipped in Tasks 1–4 are load-bearing and currently unguarded: that the gate is *reachable* from a Claude parent (not merely present in a template), and that the convention's pointer actually names the gate. Sentinels over prose assert presence, never reachability — so anchor one assert on the **producer** (learnings: `specified-but-unreachable`).

**Files:**
- Modify: `tests/test_sync_agents_run_gate.sh`

**Interfaces:**
- Consumes: `claude_surface_target` (via a generated sandbox repo), the convention file.
- Produces: nothing.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_sync_agents_run_gate.sh`, before the final `echo` line:

```bash
# --- reachability: the gate arrives at a Claude parent, not merely at a template ---
mk_repo "[claude]"
assert "reachability: a claude-only repo has a Claude surface" '[ -e "$SBX/CLAUDE.md" ]'
assert "reachability: the gate is readable through that surface" \
  'grep -q "verify-run --in-progress-ids" "$SBX/CLAUDE.md"'

# --- the convention's pointer names the gate and binds it to the verification obligation ---
CONV="$REPO/skills/docket-convention/SKILL.md"
C="$(tr '\n' ' ' < "$CONV" | tr -s '[:space:]' ' ')"
assert "convention: Composition points at the managed-block gate" \
  '[[ "$C" == *"uncommitted working-tree files"*"verify-run"*"once"* ]]'
```

- [ ] **Step 2: Run the test to verify it fails**

Temporarily revert the convention sentence (`git stash push skills/docket-convention/SKILL.md`), run `bash tests/test_sync_agents_run_gate.sh`.
Expected: FAIL — `NOT OK - convention: Composition points at the managed-block gate`. Restore with `git stash pop`.

- [ ] **Step 3: Run the test to verify it passes**

Run: `bash tests/test_sync_agents_run_gate.sh`
Expected: PASS — `ALL PASS`.

- [ ] **Step 4: Commit**

```bash
git add tests/test_sync_agents_run_gate.sh
git commit -m "test(0242): guard gate reachability at the Claude surface and the convention pointer"
```

---

### Task 6: Results file — the human-verification item

Live parent compliance is external truth with no in-repo oracle (spec *Scope*; learnings: `external-truth-needs-a-human-checkpoint`). No assert can be its judge, so it is routed to a named human item instead of a test that could only ever pass.

**Files:**
- Create: `docs/results/2026-08-08-close-the-claude-gap-caller-side-run-gate-results.md`

**Interfaces:**
- Consumes: the outcomes of Tasks 1–5.
- Produces: nothing.

- [ ] **Step 1: Write the results file**

Create `docs/results/2026-08-08-close-the-claude-gap-caller-side-run-gate-results.md` from `docs/changes/results-template.md`, filling in: what was built (single-sourced gate, Claude surface creation with `realpath` dedupe, `--check` parity), the mutation-test evidence from Tasks 1–3, and — under a **Human verification** heading — this item, verbatim:

> **Does a live Claude parent session actually run the gate?** No in-repo test can answer this: the gate is prose a model follows, and the only oracle is a real session. After merging, run `/docket-implement-next` in an interactive Claude session in this repo and confirm the transcript shows `docket.sh verify-run --in-progress-ids` **before** the dispatch and `docket.sh verify-run <id>` **after** the fork returns. A missing command is the degradation signal the spec's §3 predicted; if it recurs, the recorded escalation path is the `Stop`/`SubagentStop` hook preserved in the spec's *Rejected* section.

Also record: this repo now carries a committed `CLAUDE.md` symlink, which a Windows checkout without symlink support would materialize as a text file (spec *Risks*, accepted).

- [ ] **Step 2: Commit**

```bash
git add docs/results/2026-08-08-close-the-claude-gap-caller-side-run-gate-results.md
git commit -m "docs(0242): results — the live-parent-compliance human verification item"
```

---

## Self-Review

**Spec coverage.** Decision 1 (caller-side gate carried by the parent-facing block) → Task 1. Decision 2 (docket creates the Claude surface; three cases; one physical file; `realpath` dedupe) → Tasks 2–3. Decision 3 (routing untouched; a new parallel ADR) → Task 4's convention pointer; the **ADR itself is produced by the `docket-adr` dispatch at review time**, per docket's own flow, not authored by a plan task. Decision 4 (hook stays rejected) → no work; recorded in the results file's escalation note. Decision 5 (no loop mechanism, no headless adapter, no re-scope) → no work. Decision 6 (mirrors 0237's shape) → Task 1's gate text, asserted by the four behavioral claims. Spec §2 delivery table → Tasks 2–3. Spec *Scope*'s named test items — generated-block coverage (snapshot, verify, one-re-dispatch bound, run-halted rule) → Task 1 Step 1; surface logic per repo combo × harness set, symlink creation, dedupe-writes-once → Task 2 Step 1; the human-verification item → Task 6.

**Placeholder scan.** No TBDs; every code step carries the actual content. The one deliberate deferral — the ADR — is named with its producer, not left as a TODO.

**Type consistency.** `assemble_run_gate` (Task 1) is called by name in Tasks 1 and referenced in Task 2's comment only. `resolve_physical_path`, `repo_wants_claude_surface`, `claude_surface_target`, and `sync_dispatch_surfaces` are defined in Task 2 and called under exactly those names in Tasks 2 and 3. The Task 2 shim `sync_agents_md_dispatch(){ sync_dispatch_surfaces; }` is explicitly deleted in Task 3 Step 3, with a grep to prove no caller survives.
