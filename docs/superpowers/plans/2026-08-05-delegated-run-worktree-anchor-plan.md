<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0206 — Delegated runner runs are anchored at the main worktree, not the feature worktree](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0206-delegated-runner-runs-are-anchored-at-the-main-worktree-not.md)**
<!-- docket:backlink:end -->

# Delegated Run Worktree Anchor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `runner-dispatch.sh` an optional `--worktree <path>` whose default is the main worktree, gate it loudly for `build-*` agents, and bake it into generated `build-*` delegation shims — so a delegated build worker can no longer start in the shared primary checkout on the integration branch.

**Architecture:** The facade owns the anchor, so all three adapters stay code-identical: they keep reading `DOCKET_REPO_ROOT` verbatim into their own directory flag, and only their *contracts* (plus their header comments) change to stop claiming that value is always the primary checkout. The new argument is resolved through `docket_anchor_path`, which joins a relative value to the **main worktree** — so the flag inherits ADR-0034's cwd-independence rather than reintroducing the CWD hazard ADR-0034 was written against. Three loud `die` gates make an omitted or bogus anchor an abort instead of a silent main-tree run. `sync-agents.sh`'s `emit_shim` bakes the flag into `build-*` shims only, leaving every other shim byte-identical.

**Tech Stack:** POSIX-ish Bash 4+ (`$DOCKET_BASH_PATH`), `sed`/`awk`/`grep` text processing, the repo's hand-rolled `assert`-based test suite (`tests/test_*.sh`).

## Global Constraints

Copied verbatim from `AGENTS.md` and the spec — every task's requirements implicitly include these.

- Never `producer | early-exiting-consumer` (`grep -q`, `head`, `head -n1`) under `set -o pipefail`. Capture into a variable first, then `grep <<<"$var"`.
- `grep` for a pattern that leads with `--` must declare it: `grep -E -e "<pat>"` or `grep -qF -- "<pat>"`. A bare leading `--` is parsed as an option (exit 2) — and inside a negated assert (`! grep …`), that error inverts into a permanently green, vacuous guard. **Every new assert in this plan greps for `--worktree`, so this rule applies to all of them.**
- awk indent classes are `[^[:space:]]`, never `[^ ]`.
- A guard is code: mutation-test it — strip the thing it guards, watch it redden — or it is decoration.
- Key a guard on syntactic **shape**, never an enumerated list of spellings.
- A cross-reference in maintained source anchors on a **symbol name** or a **verbatim-quoted clause** — never on a line number.
- Run the whole suite at the build gate, never only the tests this plan enumerates.
- ADR-0034 stands **unamended**: nothing may resolve an anchor from the caller's CWD. The main worktree remains the default; the only way off it is an argument someone deliberately wrote.
- Absent `--worktree`, facade behavior must be **byte-identical to today**. Absent a `build-*` agent, generated shim bytes must be **byte-identical to today**.

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `scripts/runner-dispatch.sh` | Modify — parse `--worktree`, resolve the anchor, three gates | 1 |
| `tests/test_runner_dispatch.sh` | Modify — seven new facade asserts | 1 |
| `scripts/runner-dispatch.md` | Modify — Usage, Behavior step 2, Exit codes, Invariants | 2 |
| `scripts/runners/codex.md` · `cursor.md` · `opencode.md` | Modify — env-table row (and `opencode.md`'s Purpose prose) | 2 |
| `scripts/runners/codex.sh` · `cursor.sh` · `opencode.sh` | Modify — **header comment only**, no executable change | 2 |
| `sync-agents.sh` | Modify — `emit_shim` bakes the `build-*` slot | 3 |
| `tests/test_sync_agents.sh` | Modify — bidirectional shim asserts | 3 |
| `skills/docket-build/SKILL.md` | Modify — one sentence in *Dispatching a task* | 3 |

**Out of band — do NOT do in this plan.** The spec's §4 *Ledger* (a new ADR with `relates_to: [34]`, plus a dated `## Update` on ADR-0034) is authored on the `docket` branch by the `docket-adr` dispatch in `docket-implement-next` step 6. It is **not** a feature-branch commit. Do not create ADR files in this worktree.

---

### Task 1: Facade — `--worktree` flag, anchor resolution, three gates

The whole behavioral change lives here. Everything downstream is documentation or a generated-shim slot, so this task is the one a reviewer could reject on its own.

**Files:**
- Modify: `scripts/runner-dispatch.sh`
- Test: `tests/test_runner_dispatch.sh`

**Interfaces:**
- Consumes: `docket_main_worktree [dir]` and `docket_anchor_path <path> [dir]` from `scripts/lib/docket-root.sh` (already sourced by the facade). `docket_anchor_path` passes an absolute path through untouched, maps `""`/`.` to the main worktree root, and joins a relative path to that root; when `dir` is not in a repo it returns `<path>` unchanged (soft fallback).
- Produces: the exported env var `DOCKET_REPO_ROOT` — now "the run anchor", still absolute, still cwd-independent. Adapters (Task 2) and the shim slot (Task 3) rely on the flag being spelled exactly `--worktree` and taking one path argument.

- [ ] **Step 1: Write the failing tests**

Open `tests/test_runner_dispatch.sh` and find the existing block whose header comment is `# ---- facade: repo-root anchor + adapter handoff -----`. It ends with `rm -rf "$SBX"` just before the header comment `# ---- facade: runners.<name> config resolution across layers ----`. Insert this **entire new block** between those two — after that `rm -rf "$SBX"`, before that next header comment.

The fixture helper `make_fixture` (already defined near the top of the file) sets `SBX` (a git repo root, already `pwd -P`-normalized), `BIN` (fake-`codex` dir), and `LOG` (the argv log the fake codex appends to). `$FACADE` and `$ADAPTER` are already defined. Reuse them; define nothing new.

```bash
# ---- change 0206: --worktree, the explicit run anchor -------------------------------
# The facade's anchor is an ARGUMENT defaulting to the main worktree. ADR-0034 is unamended:
# nothing resolves the anchor from the caller's CWD, so a RELATIVE --worktree joins to the main
# worktree, not to $PWD. That relative-from-a-subdir assert is the one that distinguishes this
# design from the rejected resolve-the-caller's-CWD option — do not drop it.
make_fixture
mkdir -p "$SBX/.worktrees/featslug" "$SBX/sub/dir"

# (a) flag absent => main worktree (regression fence on today's behavior)
: > "$LOG"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status >/dev/null 2>&1 )
argv="$(cat "$LOG")"
assert "0206: no --worktree => anchor is the main worktree" 'grep -qxF -- "$SBX" <<<"$argv"'

# (b) absolute --worktree => that path verbatim
: > "$LOG"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$SBX/.worktrees/featslug" >/dev/null 2>&1 )
argv="$(cat "$LOG")"
assert "0206: absolute --worktree becomes the anchor" \
  'grep -qxF -- "$SBX/.worktrees/featslug" <<<"$argv"'
assert "0206: absolute --worktree displaces the main worktree" '! grep -qxF -- "$SBX" <<<"$argv"'

# (c) relative --worktree from a FOREIGN cwd inside the repo => joins to the MAIN worktree.
# This is the ADR-0034 discriminator: CWD-resolution would yield $SBX/sub/dir/.worktrees/featslug.
: > "$LOG"
( cd "$SBX/sub/dir" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree ".worktrees/featslug" >/dev/null 2>&1 )
argv="$(cat "$LOG")"
assert "0206: relative --worktree joins to the main worktree, not the cwd" \
  'grep -qxF -- "$SBX/.worktrees/featslug" <<<"$argv"'
assert "0206: relative --worktree did NOT resolve against the caller cwd" \
  '! grep -qF -- "$SBX/sub/dir/.worktrees" <<<"$argv"'

# (d) build-* agent with no --worktree => loud nonzero naming the flag
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent build-economy 2>&1 >/dev/null )"; rc=$?
assert "0206: build-* without --worktree is rejected" '[ "$rc" != "0" ]'
assert "0206: build-* rejection names --worktree" 'grep -qF -- "--worktree" <<<"$err"'

# (e) the gate is SCOPED to build-* — a metadata-scoped agent still needs no flag
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status >/dev/null 2>&1 ); rc=$?
assert "0206: non-build agent without --worktree still succeeds" '[ "$rc" = "0" ]'

# (f) resolved anchor is not a directory => nonzero
printf 'x\n' > "$SBX/notadir"
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$SBX/notadir" 2>&1 >/dev/null )"; rc=$?
assert "0206: --worktree pointing at a non-directory is rejected" '[ "$rc" != "0" ]'
assert "0206: non-directory rejection says directory" 'grep -qiF "not a directory" <<<"$err"'

# (g) resolved anchor outside this repo's worktree set => nonzero
OUTSIDE="$(mktemp -d)"; OUTSIDE="$(cd "$OUTSIDE" && pwd -P)"
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$OUTSIDE" 2>&1 >/dev/null )"; rc=$?
assert "0206: --worktree outside the repo worktree set is rejected" '[ "$rc" != "0" ]'
assert "0206: outside-repo rejection says worktree of this repository" \
  'grep -qiF "not a worktree of this repository" <<<"$err"'
rm -rf "$OUTSIDE"
rm -rf "$SBX"
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd /Users/homer/dev/docket/.worktrees/delegated-runner-runs-are-anchored-at-the-main-worktree-not && "$DOCKET_BASH_PATH" tests/test_runner_dispatch.sh`

Expected: FAIL. Specifically the `0206:` asserts — `absolute --worktree becomes the anchor`, the two relative-path asserts, `build-* without --worktree is rejected`, and both rejection-message asserts. Today `--worktree` is an unknown argument, so the facade dies with `unknown argument: --worktree`; the `rc != 0` asserts in (d)/(f)/(g) will pass for the *wrong reason* while their message asserts fail — that is expected at this step and is exactly why each has a message assert beside it.

- [ ] **Step 3: Add the flag to the parser**

In `scripts/runner-dispatch.sh`, change the variable initialization line:

```sh
RUNNER=""; AGENT=""; MODEL=""; EFFORT=""
```

to:

```sh
RUNNER=""; AGENT=""; MODEL=""; EFFORT=""; WORKTREE=""
```

Then add one case arm to the `while` parse loop, immediately after the `--effort` arm and before the `--)` arm:

```sh
    --worktree) WORKTREE="${2:-}"; shift 2 ;;
```

- [ ] **Step 4: Replace the anchor block with resolution plus the three gates**

Still in `scripts/runner-dispatch.sh`, replace this existing three-line block (it sits between the unknown-runner `die` and the `# --- runners.<name>: config …` comment):

```sh
REPO_ROOT="$(docket_main_worktree)"
[ -n "$REPO_ROOT" ] || die "not inside a git repository"
export DOCKET_REPO_ROOT="$REPO_ROOT"
```

with:

```sh
# --- anchor: an explicit argument defaulting to the main worktree (change 0206) -----
# ADR-0034 is UNAMENDED. Routing --worktree through docket_anchor_path rather than using it raw
# is the whole point: a relative value joins to the MAIN worktree, so it resolves identically from
# any cwd, and the new argument inherits ADR-0034's cwd-independence instead of quietly
# reintroducing the hazard ADR-0034 was written against. Absent --worktree the expression is
# docket_anchor_path "." — the main worktree — so every currently-shipped shim is unaffected.
REPO_ROOT="$(docket_main_worktree)"
[ -n "$REPO_ROOT" ] || die "not inside a git repository"
# Gate 1 — a build worker must run INSIDE its feature worktree. This is the one piece of
# agent-family knowledge the facade gains; it is a RUNTIME requirement (the path is runtime data),
# so sync-agents.sh's generation-time slot cannot substitute for it. Loud, matching the facade's
# posture for an unknown --runner rather than its tolerant posture for a runners.<name>: value:
# that tolerance exists so a cosmetic config typo cannot fail a live dispatch, whereas this is a
# request the facade cannot serve correctly.
case "$AGENT" in
  build-*) [ -n "$WORKTREE" ] || die "--worktree is required for build-* agents (a build worker must run in its feature worktree, not the main tree)" ;;
esac
ANCHOR="$(docket_anchor_path "${WORKTREE:-.}")"
# Gate 2 — the resolved anchor must exist as a directory.
[ -d "$ANCHOR" ] || die "--worktree $ANCHOR is not a directory"
# Gate 3 — and belong to THIS repo's worktree set, so a child harness running under an
# auto-approve permission grant is never handed a tree docket does not own. A non-repo path makes
# docket_main_worktree print nothing, so the not-a-repo case falls out of this same comparison.
[ "$(docket_main_worktree "$ANCHOR")" = "$REPO_ROOT" ] || die "--worktree $ANCHOR is not a worktree of this repository"
export DOCKET_REPO_ROOT="$ANCHOR"
```

Note the gates sit **before** the `runners.<name>:` config read, so an anchor failure is reported without depending on config parsing.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/delegated-runner-runs-are-anchored-at-the-main-worktree-not && "$DOCKET_BASH_PATH" tests/test_runner_dispatch.sh`

Expected: PASS — every line `ok - …`, no `NOT OK`. In particular the pre-existing asserts `facade: repo root anchored to the main worktree` and `facade: -C is the main worktree even from a subdir` must still pass: they are the byte-identical-behavior fence.

- [ ] **Step 6: Mutation-test the three gates**

A guard is code. Prove each gate reddens, one at a time, reverting after each:

1. Comment out the `case "$AGENT" in build-*)` gate → assert `0206: build-* without --worktree is rejected` must go `NOT OK`. Restore.
2. Comment out the `[ -d "$ANCHOR" ]` gate → `0206: --worktree pointing at a non-directory is rejected` must go `NOT OK`. Restore.
3. Comment out the `docket_main_worktree "$ANCHOR"` comparison gate → `0206: --worktree outside the repo worktree set is rejected` must go `NOT OK`. Restore.
4. Change `docket_anchor_path "${WORKTREE:-.}"` to `printf '%s\n' "${WORKTREE:-$REPO_ROOT}"` (the rejected CWD-naive shape) → `0206: relative --worktree joins to the main worktree, not the cwd` must go `NOT OK`. Restore.

Run `"$DOCKET_BASH_PATH" tests/test_runner_dispatch.sh` after each mutation and after each restore. All four mutations must redden; the final restored run must be fully green. If any mutation stays green, the assert is decoration — fix the assert before continuing.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/delegated-runner-runs-are-anchored-at-the-main-worktree-not
git add scripts/runner-dispatch.sh tests/test_runner_dispatch.sh
git commit -m "fix(0206): make the delegated run anchor an explicit --worktree argument

The anchor defaults to the main worktree, so every shipped shim is unaffected.
Three loud gates: --worktree is required for build-* agents, the resolved anchor
must be a directory, and it must belong to this repo's worktree set. Resolution
runs through docket_anchor_path, so a relative value joins to the main worktree
and the new argument inherits ADR-0034's cwd-independence."
```

---

### Task 2: Contracts — stop asserting the anchor is always the primary checkout

Pure documentation, but it is the deliverable: a contributor reading `codex.md` alone must not conclude `DOCKET_REPO_ROOT` is always the main worktree. Splitting it from Task 1 lets a reviewer reject the wording without rejecting the behavior.

**Files:**
- Modify: `scripts/runner-dispatch.md`
- Modify: `scripts/runners/codex.md`, `scripts/runners/cursor.md`, `scripts/runners/opencode.md`
- Modify: `scripts/runners/codex.sh`, `scripts/runners/cursor.sh`, `scripts/runners/opencode.sh` — **header comments only**

**Interfaces:**
- Consumes: the flag name and gate diagnostics established in Task 1.
- Produces: nothing executable. `tests/test_script_contracts_coverage.sh` already requires each `scripts/**/*.sh` to have a co-located `.md`; this task adds no file, so that parity is unchanged.

**Scope note (from the change's reconcile pass).** The spec says "one env-table row each". The actual restatement surface is slightly wider, and all of it must move or a surface is left lying: `opencode.md`'s Purpose prose also says the value comes from `docket_main_worktree()`, and all three adapter **scripts** repeat the claim in their header comments. Comment-only edits keep the adapters' executable code byte-identical, which is what the spec means by "the adapters are unchanged".

- [ ] **Step 1: Update `scripts/runner-dispatch.md` — Usage**

Replace the Usage code block:

```
docket.sh runner-dispatch --runner <name> --agent <agent> [--model <m>] [--effort <e>] [--] [<args…>]
```

with:

```
docket.sh runner-dispatch --runner <name> --agent <agent> [--model <m>] [--effort <e>] [--worktree <path>] [--] [<args…>]
```

Then add this bullet to the flag list, immediately after the `--model` / `--effort` bullet and before the `-- <args…>` bullet:

```markdown
- `--worktree <path>` (optional) — the run anchor. Resolved through `docket_anchor_path`, so an
  absolute path passes through and a **relative** one joins to the main worktree (never to the
  caller's cwd). Absent ⇒ the main worktree, byte-identical to pre-0206 behavior. **Required for
  `build-*` agents**: the `docket-build-task` contract requires a build worker to run inside its
  feature worktree, on its branch, so a `build-*` delegation without it is a loud abort rather
  than a silent run in the primary checkout on the integration branch.
```

- [ ] **Step 2: Update `scripts/runner-dispatch.md` — Behavior step 2, Exit codes, Invariants**

Replace Behavior step 2 in full:

```markdown
2. **Anchor** — `DOCKET_REPO_ROOT` = `docket_anchor_path "${worktree:-.}"`
   (`scripts/lib/docket-root.sh`, ADR-0034). With no `--worktree` that is the repo's primary
   checkout, cwd-independent — correct even when invoked from `.docket/` or a `.worktrees/<slug>`
   feature worktree. With `--worktree` it is the named tree, and a relative value joins to the
   main worktree so it too resolves identically from any cwd. Not in a repo ⇒ abort. Three loud
   gates follow, all before the config read so an anchor failure never depends on config parsing:
   `--worktree` is **required** for a `build-*` agent; the resolved anchor must be a **directory**;
   and it must be a **worktree of this repository** (compared via `docket_main_worktree "$anchor"`,
   whose empty result for a non-repo path fails the same comparison).
```

In **Exit codes**, replace the `1` line with:

```markdown
- `1` — validation failure, unknown runner, not inside a git repository, or a rejected
  `--worktree` (missing for a `build-*` agent, not a directory, or not a worktree of this repo).
```

In **Invariants**, add as the first bullet:

```markdown
- The anchor is **never** resolved from the caller's CWD; absent `--worktree` it is the main
  worktree (ADR-0034 unamended). A relative `--worktree` joins to the main worktree, so the
  argument inherits that cwd-independence rather than reintroducing the hazard.
```

- [ ] **Step 3: Update the three adapter contracts**

In `scripts/runners/codex.md`, replace the `DOCKET_REPO_ROOT` env-table row:

```markdown
| `DOCKET_REPO_ROOT` | absolute run anchor — the main worktree unless the caller named a feature worktree; becomes `codex exec -C` | required |
```

In `scripts/runners/cursor.md`, replace its row:

```markdown
| `DOCKET_REPO_ROOT` | absolute run anchor — the main worktree unless the caller named a feature worktree; the run's repo anchor | required |
```

In `scripts/runners/opencode.md`, replace its row:

```markdown
| `DOCKET_REPO_ROOT` | absolute run anchor — the main worktree unless the caller named a feature worktree; becomes `opencode run --dir` | required |
```

Also in `scripts/runners/opencode.md`, find the Purpose prose sentence stating that the facade `sets DOCKET_REPO_ROOT from docket_main_worktree()` (cwd-independent by design, ADR-0034). Rewrite that clause to:

```markdown
sets `DOCKET_REPO_ROOT` to the run anchor — the main worktree by default, or the tree named by
`--worktree` (both cwd-independent by design, ADR-0034) — and this
```

Keep the surrounding sentence intact; only the clause changes. Do not renumber or reflow neighboring content.

- [ ] **Step 4: Update the three adapter header comments**

These are `#` comments at the top of each script. No executable line changes.

- `scripts/runners/codex.sh` — the header line reading `# Mock seam: CODEX_BIN. Env in (from the facade): DOCKET_REPO_ROOT (absolute, required),` keeps its shape; append to the `DOCKET_REPO_ROOT` mention the parenthetical so it reads `DOCKET_REPO_ROOT (absolute run anchor — main worktree unless the caller named one, required),`.
- `scripts/runners/cursor.sh` — the header line `# DOCKET_REPO_ROOT (absolute, required).` becomes `# DOCKET_REPO_ROOT (absolute run anchor — main worktree unless the caller named one, required).`
- `scripts/runners/opencode.sh` — the header line `# DOCKET_REPO_ROOT (absolute, required), DOCKET_RUNNER_CFG_PERMISSIONS (default \`ask\`).` becomes `# DOCKET_REPO_ROOT (absolute run anchor — main worktree unless the caller named one, required), DOCKET_RUNNER_CFG_PERMISSIONS (default \`ask\`).`

- [ ] **Step 5: Verify no surface still claims the anchor is always the main worktree**

Run this whole-repo sweep (never a hand-listed set of files):

```bash
cd /Users/homer/dev/docket/.worktrees/delegated-runner-runs-are-anchored-at-the-main-worktree-not
hits="$(grep -rn "DOCKET_REPO_ROOT" scripts/ | grep -i "main.worktree")"
printf '%s\n' "$hits"
```

Expected: every remaining hit is one that is *correct* — the facade's own comment and `runner-dispatch.md` step 2, both of which now say "unless the caller named a feature worktree" or equivalent. **No adapter file (`scripts/runners/*`) may appear in the output** describing the value as unconditionally the main worktree. If one does, it was missed in Step 3 or 4.

- [ ] **Step 6: Run the affected tests**

Run: `cd /Users/homer/dev/docket/.worktrees/delegated-runner-runs-are-anchored-at-the-main-worktree-not && "$DOCKET_BASH_PATH" tests/test_runner_opencode.sh && "$DOCKET_BASH_PATH" tests/test_runner_cursor.sh && "$DOCKET_BASH_PATH" tests/test_runner_dispatch.sh && "$DOCKET_BASH_PATH" tests/test_comment_anchor_style.sh && "$DOCKET_BASH_PATH" tests/test_script_contracts_coverage.sh`

Expected: PASS on all five, no `NOT OK` lines.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/delegated-runner-runs-are-anchored-at-the-main-worktree-not
git add scripts/runner-dispatch.md scripts/runners/
git commit -m "docs(0206): describe DOCKET_REPO_ROOT as the run anchor, not the main worktree

runner-dispatch.md gains --worktree in Usage, a rewritten Behavior step 2 covering
the default and the three gates, the widened exit-code note, and an invariant that
the anchor is never resolved from the caller's CWD. The three adapters keep their
executable code byte-identical; only their env-table rows and header comments move,
so no contract is left asserting the value is always the primary checkout."
```

---

### Task 3: Generated `build-*` shims carry the flag as a required slot

**Files:**
- Modify: `sync-agents.sh` (the `emit_shim` function)
- Modify: `skills/docket-build/SKILL.md`
- Test: `tests/test_sync_agents.sh`

**Interfaces:**
- Consumes: `emit_shim`'s existing positional contract — `$1=src $2=model $3=effort $4=runner $5=agent-name $6=flag-model $7=flag-effort`. `$5` is the short agent name (`status`, `adr`, `build-economy`, …), derived by `short_name` from `agents/docket-<name>.md`. Task 1's flag spelling `--worktree`.
- Produces: generated files under `<root>/.claude/agents/docket-*.md`. Non-`build-*` shims must be **byte-identical** to today.

- [ ] **Step 1: Write the failing tests**

Open `tests/test_sync_agents.sh`. Find the block starting `# ---- change 0079: runner delegation shims -----` and, within it, the sub-block that begins `# shim generation: agents.claude.<agent>.runner: codex swaps the BODY for the shim`. Locate the assert whose label is `0079: effort auto omits --effort from the shim`. Insert this new block immediately **after** that assert's line.

```bash
# ---- change 0206: build-* shims bake --worktree as a required slot -------------------
# BIDIRECTIONAL by construction (LEARNINGS: correspondence-guard-runs-one-way). This is a MIRROR
# correspondence, not a subset: build-* shims must carry the flag AND non-build shims must not, so
# a future change that widens the flag to every shim reddens just as a change that drops it does.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    build-economy: { model: test-model-x, runner: codex }\n    status: { model: test-model-y, runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
B="$SBX/.claude/agents/docket-build-economy.md"
S="$SBX/.claude/agents/docket-status.md"

# NON-VACUITY FLOOR: every assert below reads these two files, so prove BOTH are real shims first.
# Without this, a generation failure leaves absent files and the negative asserts pass by default —
# which reads exactly like the property holding.
assert "0206: fixture sanity — build-economy generated a real shim" \
  'grep -qF "docket.sh runner-dispatch" "$B"'
assert "0206: fixture sanity — status generated a real shim" \
  'grep -qF "docket.sh runner-dispatch" "$S"'

# Direction 1: a build-* shim CARRIES the slot.
assert "0206: build-* shim bakes --worktree into the dispatch line" \
  'grep -qF -- "--worktree" "$B"'
assert "0206: build-* shim keeps the slot on the runner-dispatch line itself" \
  'grep -F "docket.sh runner-dispatch" "$B" | grep -qF -- "--worktree"'
assert "0206: build-* shim tells the caller to abort rather than guess a path" \
  'grep -qiF "abort-and-report" "$B"'

# Direction 2: a non-build shim does NOT. (grep -qF -- is mandatory: a bare leading `--` is parsed
# as an option, exit 2, which inside this negation would be permanently, vacuously green.)
assert "0206: non-build shim carries no --worktree" '! grep -qF -- "--worktree" "$S"'
assert "0206: exactly one dispatch invocation in the build-* shim" \
  '[ "$(grep -cF "docket.sh runner-dispatch" "$B")" = "1" ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd /Users/homer/dev/docket/.worktrees/delegated-runner-runs-are-anchored-at-the-main-worktree-not && "$DOCKET_BASH_PATH" tests/test_sync_agents.sh 2>&1 | grep -E "^(NOT OK|ok - 0206)"`

Expected: the three Direction-1 asserts (`build-* shim bakes --worktree…`, `…keeps the slot on the runner-dispatch line itself`, `…abort rather than guess a path`) appear as `NOT OK`. The two fixture-sanity asserts and both Direction-2 asserts should already be `ok` — if a fixture-sanity assert is `NOT OK`, the fixture is wrong (most likely the agent key spelling); fix the fixture before touching `emit_shim`.

- [ ] **Step 3: Bake the slot into `emit_shim`**

In `sync-agents.sh`, inside `emit_shim`, find these three lines:

```sh
  local flags="--runner $4 --agent $5"
  [ -n "${6:-}" ] && flags="$flags --model $6"
  [ -n "${7:-}" ] && [ "${7:-}" != "auto" ] && flags="$flags --effort $7"
```

Add immediately after them:

```sh
  # change 0206: a build worker must run INSIDE its feature worktree, on its branch — the
  # docket-build-task contract's own requirement. Baked PER AGENT (emit_shim receives the name as
  # $5) rather than as generic prose, so a status/adr shim stays byte-identical. The VALUE is still
  # prose-supplied one level up; what makes that acceptable is the facade's build-* gate, which
  # turns an omission into a loud abort instead of a silent main-tree run on the integration branch.
  local wt_slot="" wt_rule=""
  case "$5" in
    build-*)
      wt_slot=" --worktree <feature worktree>"
      wt_rule="This is a BUILD worker: it must run INSIDE its feature worktree, on its branch —
never the main tree on the integration branch. Replace \`<feature worktree>\` with the absolute
path of the feature worktree your caller named (drop the angle brackets). If your caller named no
worktree, abort-and-report — never guess a path, and never omit the flag."
      ;;
  esac
```

Then, in the `cat <<SHIM` heredoc body, change the dispatch line from:

```
    "\${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh runner-dispatch $flags [-- <caller args>]
```

to:

```
    "\${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh runner-dispatch $flags$wt_slot [-- <caller args>]
```

Finally, append the conditional paragraph **after** the closing `SHIM` line of the heredoc, as the function's last statement:

```sh
  if [ -n "$wt_rule" ]; then printf '\n%s\n' "$wt_rule"; fi
```

Three details that matter and are easy to get wrong:

1. Use `if … then … fi`, **not** `[ -n "$wt_rule" ] && printf …`. As the function's last command the `&&` form returns 1 when `wt_rule` is empty, and `emit_wrapper` returns `emit_shim`'s status — a spurious nonzero on every non-build shim.
2. The paragraph is emitted **outside** the heredoc precisely so a non-build shim gains no trailing blank line. Byte-identical output for other shims is a hard requirement — `tests/test_sync_agents.sh`'s `0079: --check flags a de-shimmed wrapper as drift` assert depends on stable bytes.
3. `wt_slot` and `wt_rule` are interpolated into an **unquoted** heredoc, but backticks inside a *variable's value* are not re-parsed for command substitution, so the `` `<feature worktree>` `` markup is safe as written.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/delegated-runner-runs-are-anchored-at-the-main-worktree-not && "$DOCKET_BASH_PATH" tests/test_sync_agents.sh`

Expected: PASS — no `NOT OK` lines anywhere in the file, including every pre-existing `0079:`, `0168:`, and `0205:` assert.

- [ ] **Step 5: Mutation-test both directions**

The mirror correspondence must redden either way:

1. Delete the `build-*)` case arm (so no shim gets the slot) → `0206: build-* shim bakes --worktree into the dispatch line` must go `NOT OK`.
2. Restore, then change the case pattern from `build-*)` to `*)` (so every shim gets it) → `0206: non-build shim carries no --worktree` must go `NOT OK`.
3. Restore and re-run: fully green.

Run `"$DOCKET_BASH_PATH" tests/test_sync_agents.sh` after each. If either mutation stays green, the guard is one-directional — fix it before continuing.

- [ ] **Step 6: Add the `docket-build` dispatch sentence**

In `skills/docket-build/SKILL.md`, find the `## Dispatching a task` section's first paragraph — the one beginning `Dispatch the profile agent **by name**, foreground, one task at a time`. Append this sentence to the end of that paragraph:

```markdown
A worker reached through a runner delegation receives its worktree through the facade's
`--worktree` flag, not through the prompt body alone.
```

- [ ] **Step 7: Check the size budget before committing**

`skills/docket-build/SKILL.md` is budgeted at **265 lines / 2400 words** in `tests/test_skill_size_budgets.sh` and measured 260/2348 before this edit — roughly 5 lines and 52 words of headroom. Verify:

```bash
cd /Users/homer/dev/docket/.worktrees/delegated-runner-runs-are-anchored-at-the-main-worktree-not
wc -l -w skills/docket-build/SKILL.md
"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh
```

Expected: PASS, with the file at or under 265 lines and 2400 words. If the budget is exceeded, do **not** silently trim the sentence's meaning and do **not** raise the number as a reflex — the table's own rule requires a raise to name the `references/` file the prose was considered for and argue in-diff why it cannot live there. Prefer shortening the sentence to fit; only if it cannot carry the rule in the remaining budget, raise the row per the file's stated rounding rule (lines to the next multiple of 5, words to the next multiple of 50, pushed one multiple further if that lands within 25 words of the actual) with the argument written into the comment block.

- [ ] **Step 8: Run the affected tests**

Run: `cd /Users/homer/dev/docket/.worktrees/delegated-runner-runs-are-anchored-at-the-main-worktree-not && "$DOCKET_BASH_PATH" tests/test_sync_agents.sh && "$DOCKET_BASH_PATH" tests/test_docket_build.sh && "$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh`

Expected: PASS on all three.

- [ ] **Step 9: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/delegated-runner-runs-are-anchored-at-the-main-worktree-not
git add sync-agents.sh tests/test_sync_agents.sh skills/docket-build/SKILL.md
git commit -m "feat(0206): bake --worktree into generated build-* delegation shims

emit_shim receives the agent name, so the requirement lands in the generated file
itself rather than as prose applying equally to every shim: a build-* shim carries
the flag as a required slot plus an abort-and-report rule for a caller that named
no worktree, and every other shim is byte-identical to before. The generation guard
runs in both directions, so widening the flag to all shims reddens as loudly as
dropping it from build shims."
```

---

## Self-Review

**1. Spec coverage.**

| Spec section | Task |
|---|---|
| §1 Facade — `--worktree`, `docket_anchor_path` routing, three gates, gate ordering before config | Task 1 Steps 3–4 |
| §2 Adapters — code unchanged, contracts' env rows reworded | Task 2 Steps 3–4 |
| §2 `runner-dispatch.md` — Usage, Behavior step 2, Invariants | Task 2 Steps 1–2 |
| §3 `emit_shim` build-* slot + abort-and-report rule; other shims untouched | Task 3 Step 3 |
| §3 `docket-build` SKILL.md sentence | Task 3 Step 6 |
| §4 Ledger (new ADR + ADR-0034 `## Update`) | **Out of band** — `docket-adr` dispatch on the `docket` branch, flagged under *File Structure* |
| §5 Testing — all seven facade cases | Task 1 Step 1 (a)–(g) |
| §5 Testing — sync-agents both directions | Task 3 Step 1 |

The spec's seven facade test cases map one-to-one onto (a) flag absent, (b) absolute, (c) relative from a foreign cwd, (d) build-* without the flag, (f) non-directory, (g) outside the repo set, (e) non-build agent still succeeds. All present.

**2. Placeholder scan.** No `TBD`, no "add appropriate error handling", no "similar to Task N". Every code step carries the literal text to write; every test step carries the exact command and its expected result. The one deliberately deferred decision — whether the size budget needs raising — is a measured branch with both outcomes specified, not a placeholder.

**3. Type consistency.** The flag is spelled `--worktree` in the facade parser (Task 1 Step 3), the contract Usage block (Task 2 Step 1), the shim slot (Task 3 Step 3), and every assert. The shell variable is `WORKTREE` in the facade and `wt_slot`/`wt_rule` in `emit_shim` — different files, no collision. `ANCHOR` is local to the facade; `REPO_ROOT` keeps its existing name and now holds the main worktree used as the Gate-3 comparison base, which is what the surrounding pre-existing not-a-repo abort already assumed. `docket_anchor_path` and `docket_main_worktree` are used with the signatures documented in `scripts/lib/docket-root.sh`.
