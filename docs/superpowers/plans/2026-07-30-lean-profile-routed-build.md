<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0167 — Lean profile-routed build — fresh task workers without review loops](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-30-0167-lean-profile-routed-build.md)**
<!-- docket:backlink:end -->

# Lean Profile-Routed Build Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give docket its own build role — a controller skill that routes each plan task to one of three named Claude model/effort profile agents running a compact TDD worker contract — replacing SDD's per-task reviewer loops with one full-suite gate and docket's existing single whole-branch review.

**Architecture:** Two new skills (`docket-build`, the controller invoked inline through the existing `skills.build` role; `docket-build-task`, the compact worker contract preloaded into each profile agent) plus three new generated agent wrappers (`docket-build-economy` / `-standard` / `-premium`) that differ only in `model:` and `effort:`. One new config leaf (`build.checkpoint`) resolves through the existing layered resolver. Nothing about the existing dispatch, config, or review machinery is restructured — the profile agents are ordinary `agents/docket-*.md` wrappers auto-discovered by `sync-agents.sh`, and the full-suite gate reuses `finalize.test_command` rather than introducing a second test-command key.

**Tech Stack:** POSIX-ish Bash (`scripts/*.sh`, run under `$DOCKET_BASH_PATH`), awk/sed text processing, markdown skill and agent files with YAML frontmatter, hermetic `tests/test_*.sh` guard scripts.

## Global Constraints

Copied verbatim or by exact reference from the spec (`docs/superpowers/specs/2026-07-30-lean-profile-routed-build-design.md` on the `docket` branch) and from `AGENTS.md`, which every task must satisfy:

- The three profile entries live under `agents.claude`, **never** `agents.default` — placing Claude model IDs in the harness-neutral fallback would falsely present them as harness-portable.
- Accepted profile values are exactly `economy`, `standard`, `premium`. An invalid explicit plan value is a **plan contract error that halts**, never a silent fallback.
- Escalation ladder, exactly: `economy -> one standard retry`, `standard -> one premium retry`, `premium -> halt`. One escalation per task, total; a task that started at `economy` halts if its `standard` retry fails — it never climbs to `premium`.
- No profile emits `maxTurns`. There are no hard subagent turn caps anywhere in this change.
- `build.checkpoint` default is `false`. Values other than `true` or `false` are configuration errors (resolver `die`), not silent fallback.
- The shipped cross-harness default for `skills.build` stays `superpowers:subagent-driven-development`. Only this repo's own `.docket.yml` opts in to `docket-build`.
- The full-suite gate derives its command from `finalize.test_command` (when set) or finalize's existing auto-detection. **Do not introduce a second test-command config key**, and do not hand-maintain a duplicate copy of the suite-command fragment.
- `docket-build` performs **no** per-task independent review and **no** final review of its own. The worker's self-review is part of implementation, not a second agent.
- Every new `skills/**/*.md` file must gain a `BUDGETS` row in `tests/test_skill_size_budgets.sh` **in the same commit** — its completeness guard fails otherwise. Set the row from the measured actual: round lines up to the next multiple of 5 and words up to the next multiple of 50; if that leaves ≤25 words of margin, take the next multiple after that.
- `AGENTS.md` shell rules bind all new script and test code: never `producer | grep -q` under `set -o pipefail` (capture into a variable, then `grep <<<"$var"`); a pattern leading with `--` needs `grep -qF --` or `grep -E -e`; awk indent classes are `[^[:space:]]`, never `[^ ]`.
- `AGENTS.md` guard rules bind all new tests: a guard is code — mutation-test it or it is decoration; key guards on syntactic **shape**, not an enumerated list of spellings; run the **whole** suite at the build gate, never only the tests this plan enumerates.
- Cross-references in maintained source anchor on a **symbol name** or a **verbatim-quoted clause**, never a line number (`tests/test_comment_anchor_style.sh` enforces the filename-plus-line-number form).

**Out of scope for these tasks (handled elsewhere, do not build):** the new ADR superseding ADR-0023 (docket-implement-next Step 6 dispatches `docket-adr`); closing change 0044 (already killed 2026-07-30); minting the Cursor / Codex / lean-review follow-ups (already exist as changes 0168, 0169, 0170).

---

### Task 1: `build.checkpoint` config key, resolved end-to-end

The knob must ship end-to-end in one task: `tests/test_docket_example_yml.sh` drives its completeness check off the resolver's **actual export surface**, so adding the `BUILD_CHECKPOINT` export without documenting the key in `.docket.example.yml` turns that suite red. Resolver, contract doc, example file, and both test files therefore land together.

**Files:**
- Modify: `scripts/docket-config.sh` (insert the `build:` block immediately after the `reclaim:` block, before the `change_types + auto_capture` section; extend that block's `trap`; add one `emit` line)
- Modify: `scripts/docket-config.md` (key table row + prose subsection)
- Modify: `.docket.example.yml` (new banner section between the `reclaim` and `Board surfaces` sections)
- Test: `tests/test_docket_config.sh` (new `BLD-a` … `BLD-e` block)
- Test: `tests/test_docket_example_yml.sh` (two `classify_key` arms + `expected_key_count`)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the exported variable **`BUILD_CHECKPOINT`** — value `true` or `false`, default `false` — emitted in the `--export` block after `RECLAIM_AUTO` and before `SKILL_BRAINSTORM`. Task 4's `docket-build` skill reads it as a Step-0 export literal.

- [ ] **Step 1: Write the failing resolver tests**

Append this block to `tests/test_docket_config.sh` immediately after the reclaim block's final assert (the one labelled `unparseable reclaim.auto: mentions reclaim.auto`) and before the `# --- Change 0091 — auto_capture` banner. It reuses the helpers already defined in that file (`mkrepo`, `run`, `rung`, `run_resolver_with`, `assert`).

```bash
# ============================================================================
# Change 0167 — the build: block (BUILD_CHECKPOINT)
# NOTE (guards-are-code (e)): clear the asserted vars BEFORE each eval — an aborting run emits
# NOTHING, and eval "" would silently leave the previous case's value in place.
# ============================================================================

# --- (BLD-a) default when no layer sets the block -----------------------------
unset BUILD_CHECKPOINT
mkrepo "$tmp/bld-a"
out="$(run "$tmp/bld-a" --export)"; eval "$out"
assert "BUILD_CHECKPOINT defaults to false" 'echo "$out" | grep -qxF "BUILD_CHECKPOINT=false"'

# --- (BLD-b) repo-committed block is honored ----------------------------------
unset BUILD_CHECKPOINT
mkrepo "$tmp/bld-b"
cat > "$tmp/bld-b/.docket.yml" <<'EOF'
metadata_branch: main
build:
  checkpoint: true
EOF
git -C "$tmp/bld-b" add .docket.yml; git -C "$tmp/bld-b" commit --quiet -m cfg
git -C "$tmp/bld-b" push --quiet origin main
out2="$(run "$tmp/bld-b" --export)"; eval "$out2"
assert "BUILD_CHECKPOINT reads the block" 'echo "$out2" | grep -qxF "BUILD_CHECKPOINT=true"'

# --- (BLD-c) global-able (ADR-0019 — NOT coordination-fenced) -----------------
unset BUILD_CHECKPOINT
mkrepo "$tmp/bld-c"
mkdir -p "$tmp/bld-c.xdg/docket"
cat > "$tmp/bld-c.xdg/docket/config.yml" <<'EOF'
build:
  checkpoint: true
EOF
bld_c_err="$(rung "$tmp/bld-c.xdg" "$tmp/bld-c" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/bld-c.xdg" "$tmp/bld-c" --export 2>/dev/null)"; eval "$out"
assert "build.checkpoint is global-able (not fenced)" '[ "$BUILD_CHECKPOINT" = "true" ]'
assert "no fence warning for build.checkpoint" '! printf "%s" "$bld_c_err" | grep -qi "build.*per-repo-only"'

# --- (BLD-d) repo-local layer wins over repo-committed ------------------------
unset BUILD_CHECKPOINT
mkrepo "$tmp/bld-d"
cat > "$tmp/bld-d/.docket.yml" <<'EOF'
metadata_branch: main
build:
  checkpoint: true
EOF
git -C "$tmp/bld-d" add .docket.yml; git -C "$tmp/bld-d" commit --quiet -m cfg
git -C "$tmp/bld-d" push --quiet origin main
printf 'build:\n  checkpoint: false\n' > "$tmp/bld-d/.docket.local.yml"
out="$(run "$tmp/bld-d" --export)"; eval "$out"
assert "local layer beats repo-committed for build.checkpoint" '[ "$BUILD_CHECKPOINT" = "false" ]'

# --- (BLD-e) SHADOW GUARD — a bare checkpoint: OUTSIDE the build: block --------
# must not leak in. This is the whole reason the block is read via yaml_block_body: `checkpoint`
# is a generic word another block could otherwise shadow.
unset BUILD_CHECKPOINT
mkrepo "$tmp/bld-e"
cat > "$tmp/bld-e/.docket.yml" <<'EOF'
metadata_branch: main
some_future_block:
  checkpoint: true
EOF
git -C "$tmp/bld-e" add .docket.yml; git -C "$tmp/bld-e" commit --quiet -m cfg
git -C "$tmp/bld-e" push --quiet origin main
out="$(run "$tmp/bld-e" --export)"; eval "$out"
assert "a foreign block's checkpoint: does not shadow build.checkpoint" '[ "$BUILD_CHECKPOINT" = "false" ]'

# --- (BLD-f) fail closed on garbage -------------------------------------------
assert "non-bool checkpoint aborts nonzero" '! run_resolver_with "build:\n  checkpoint: maybe\n" >/dev/null 2>&1'
bld_f_err="$(run_resolver_with "build:\n  checkpoint: maybe\n" 2>&1 >/dev/null)"
assert "unparseable build.checkpoint: mentions build.checkpoint" \
  'printf "%s" "$bld_f_err" | grep -qF "build.checkpoint"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -E "NOT OK|^(PASS|FAIL)"`
Expected: FAIL, with `NOT OK - BUILD_CHECKPOINT defaults to false` (and the other BLD asserts) — the resolver emits no such variable yet.

- [ ] **Step 3: Implement the resolver block**

In `scripts/docket-config.sh`, insert immediately after the reclaim block's closing `esac` (the one whose `die` message names `reclaim.auto`) and before the `# --- change_types + auto_capture: the typed-capture policy (change 0127) -------` banner:

```sh
# --- build: the build-role knobs (change 0167) -------------------------------
# Nested block parsed exactly like reclaim: — the leaf is read WITHIN the block via
# yaml_block_body, never as a bare top-level key: `checkpoint` is a generic word another block
# could shadow. Behavioral, NOT coordination-fenced: it resolves through the full per-field
# layering repo-local > repo-committed > global > built-in, like reclaim.* / learnings.*.
# checkpoint gates whether docket-build persists a resume ledger; false (the default) keeps the
# build's durability in the per-task code commits alone.
BUILD_BLK="$(mktemp)";  yaml_block_body "$CFG"  build >"$BUILD_BLK"
GBUILD_BLK="$(mktemp)"; yaml_block_body "$GCFG" build >"$GBUILD_BLK"
LBUILD_BLK="$(mktemp)"; yaml_block_body "$LCFG" build >"$LBUILD_BLK"
trap 'rm -f "$CFG" "$LEARN_BLK" "$GLEARN_BLK" "$LLEARN_BLK" "$RECLAIM_BLK" "$GRECLAIM_BLK" "$LRECLAIM_BLK" "$BUILD_BLK" "$GBUILD_BLK" "$LBUILD_BLK"' EXIT
build_key(){  # build_key <leaf> <default> -> resolved value on stdout
  local v; v="$(yaml_get "$LBUILD_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$BUILD_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$GBUILD_BLK" "$1")"
  printf '%s' "${v:-$2}"
}
BUILD_CHECKPOINT="$(build_key checkpoint false)"
case "$BUILD_CHECKPOINT" in
  true|false) ;;
  *) die "unparseable config: build.checkpoint must be 'true' or 'false', got '$BUILD_CHECKPOINT'" ;;
esac
```

Then, in the `if [ "$MODE" = export ]; then` block, add one line immediately after `emit RECLAIM_AUTO "$RECLAIM_AUTO"`:

```sh
  emit BUILD_CHECKPOINT "$BUILD_CHECKPOINT"
```

Note the `trap` line **replaces** the reclaim block's trap by re-issuing it with the three new temp files appended — that is the established idiom in this file (the `learnings:` and `reclaim:` blocks each re-issue it the same way). Do not add a second `trap` alongside the old one.

- [ ] **Step 4: Run the resolver test to verify it passes**

Run: `bash tests/test_docket_config.sh 2>&1 | tail -3`
Expected: `PASS`.

- [ ] **Step 5: Document the key in `.docket.example.yml`**

Insert a new banner section between the `reclaim` section (ending with `auto: false`) and the `# ═══ Board surfaces ═══` banner. Match the surrounding style exactly: banner line padded with `═` to the same column, a prose paragraph, a `# scope:` line directly above the key, and the key **active at its shipped default** (uncommenting nothing changes behavior, so it is not presence-sensitive).

```yaml
# ═══ build — the docket-owned build role ═══════════════════════════════════════════════════

# Knobs for docket's own build role, docket-build (change 0167) — the lean, profile-routed
# alternative to superpowers:subagent-driven-development, selected via `skills: build:` below.
# Inert unless that role is bound to docket-build.
build:
  # checkpoint — false (default) => the build persists NO resume ledger; completed work is durable
  # through the per-task code commits alone, and a resumed run reconstructs progress conservatively
  # from the plan, commits, code, and tests. true => docket-build writes a compact state ledger to
  # the gitignored .superpowers/docket-build/<change-id>/progress.md, and a resumed task is skipped
  # only when its entry is COMPLETE, the plan hash still matches, and its commit is an ancestor of
  # the branch. Anything other than true/false is a config error, not a silent fallback.
  # scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)
  checkpoint: false
```

- [ ] **Step 6: Update the example-yml guard**

In `tests/test_docket_example_yml.sh`:

Add one arm to `classify_key()` immediately after the `reclaim.auto` arm:

```sh
    build.checkpoint)             echo 'resolved:BUILD_CHECKPOINT' ;;
```

Add `build` to the block-header arm — change the line reading:

```sh
    finalize|learnings|reclaim|skills|runners|runners.codex|auto_capture) echo 'elsewhere:HEADER' ;;
```

to:

```sh
    finalize|learnings|reclaim|build|skills|runners|runners.codex|auto_capture) echo 'elsewhere:HEADER' ;;
```

Bump the count — two new keys (the `build` header and its `checkpoint` leaf):

```sh
expected_key_count=38
```

Update the comment immediately above `expected_key_count` so the growth log stays true; append to its existing sentence: `change 0167 took it from 36 to 38 (the build: block header and its checkpoint leaf).`

- [ ] **Step 7: Update the resolver contract doc**

In `scripts/docket-config.md`, add a row to the key table (immediately after the `reclaim.auto` row) reading `build.checkpoint` / default `false` / scope `any layer` — match the table's existing column shape exactly. Then add a short prose subsection alongside the existing `learnings:` / `reclaim:` ones:

```markdown
### `build:`

`build.checkpoint` (default `false`, any layer) gates whether `docket-build` persists a resume
ledger under `.superpowers/docket-build/<change-id>/progress.md`. Read within the block via
`yaml_block_body` so a bare `checkpoint:` elsewhere cannot shadow it; a value other than `true` or
`false` aborts the resolver rather than falling back.
```

If the exit-code table in that file enumerates the `die` paths by key, add `build.checkpoint` there too; if it only describes exit classes generically, leave it alone.

- [ ] **Step 8: Run both test files to verify they pass**

Run: `bash tests/test_docket_config.sh 2>&1 | tail -2 && bash tests/test_docket_example_yml.sh 2>&1 | tail -2`
Expected: `PASS` from both.

- [ ] **Step 9: Mutation-test the new guards**

Prove each guard bites, restoring the file after each probe:
1. Temporarily change the resolver default from `false` to `true` in the `build_key checkpoint false` call → `bash tests/test_docket_config.sh` must report `NOT OK - BUILD_CHECKPOINT defaults to false`. Restore.
2. Temporarily delete the `checkpoint: false` line from `.docket.example.yml` → `bash tests/test_docket_example_yml.sh` must redden (the documented-vs-exported correspondence check). Restore.
3. Temporarily change `expected_key_count` back to `36` → the example-yml suite must redden. Restore.

Record the three outputs; if any mutation leaves the suite green, the guard is decoration and must be fixed before committing.

- [ ] **Step 10: Commit**

```bash
git add scripts/docket-config.sh scripts/docket-config.md .docket.example.yml \
        tests/test_docket_config.sh tests/test_docket_example_yml.sh
git commit -m "feat(0167): resolve build.checkpoint through the layered config"
```

---

### Task 2: `docket-build-task` — the compact worker contract

**Files:**
- Create: `skills/docket-build-task/SKILL.md`
- Create: `tests/test_docket_build.sh`
- Modify: `tests/test_skill_size_budgets.sh` (one `BUDGETS` row)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the skill name **`docket-build-task`**, referenced verbatim in Task 3's three agent wrappers' `skills:` frontmatter list; and the three worker outcome tokens **`COMPLETE`**, **`NEEDS_ESCALATION`**, **`BLOCKED`**, which Task 4's controller keys on.

- [ ] **Step 1: Write the failing contract guard**

Create `tests/test_docket_build.sh`. This task adds only the worker half; Task 4 appends the controller half to the same file.

```bash
#!/usr/bin/env bash
# tests/test_docket_build.sh — change 0167. Contract guards for docket's own build role:
# the docket-build controller skill and the docket-build-task worker skill.
# Guards are keyed on the load-bearing CLAUSES of each contract, so a rewrite that keeps the
# rule stays green while a rewrite that drops the rule reddens. Run: bash tests/test_docket_build.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
WORKER="$REPO/skills/docket-build-task/SKILL.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# ---------------------------------------------------------------------------
# docket-build-task — the worker contract
# ---------------------------------------------------------------------------
assert "worker: SKILL.md exists" '[ -f "$WORKER" ]'
worker_body="$(cat "$WORKER" 2>/dev/null)"

# Non-vacuity floor: every negative/shape assert below reads $worker_body, so an empty or
# unreadable file must redden HERE rather than passing every grep by default.
assert "worker: contract is non-vacuous (>= 40 lines)" \
  '[ "$(printf "%s\n" "$worker_body" | grep -c .)" -ge 40 ]'

# The three outcome tokens are the controller's entire input vocabulary — each must be present
# as a standalone token, not merely as a word inside prose.
for tok in COMPLETE NEEDS_ESCALATION BLOCKED; do
  assert "worker: defines the $tok outcome" \
    'grep -qE "(^|[^A-Z_])'"$tok"'([^A-Z_]|$)" <<<"$worker_body"'
done

# Exactly-one-commit rule: the deliverable of a task is one commit, and only on success.
assert "worker: requires exactly one commit on success" \
  'grep -qiE "exactly one (successful )?(task )?commit" <<<"$worker_body"'
assert "worker: forbids committing on a non-COMPLETE outcome" \
  'grep -qiE "(only on success|no commit|does not commit|never commit)" <<<"$worker_body"'

# TDD default plus the evidence-bound exception with all three required statements.
assert "worker: states the focused TDD cycle" \
  'grep -qiE "fails for the intended reason" <<<"$worker_body"'
assert "worker: bug fixes require a failing regression test" \
  'grep -qiE "regression test" <<<"$worker_body"'
assert "worker: guards require mutation evidence" \
  'grep -qiE "mutation evidence|turns red" <<<"$worker_body"'
for clause in "why RED/GREEN was unsuitable" "what verification replaced it" "what residual risk"; do
  assert "worker: TDD exception must state — $clause" 'grep -qiF -- "$clause" <<<"$worker_body"'
done
# The insufficient-reason list is the teeth of the exception: without it "hard to test" walks.
assert "worker: names the insufficient reasons for skipping RED/GREEN" \
  'grep -qiF -- "hard to test" <<<"$worker_body" && grep -qiF -- "no existing tests" <<<"$worker_body"'

# NO REVIEW: the worker self-reviews; it must never dispatch a reviewer or fix agent.
assert "worker: forbids dispatching a reviewer or another agent" \
  'grep -qiE "(never|does not|do not|no)[^.]{0,80}(dispatch|subagent)" <<<"$worker_body"'
assert "worker: self-review is part of implementation, not a second agent" \
  'grep -qiE "self-review" <<<"$worker_body"'

# Escalation is a narrow door — an expected RED or one failed run is NOT an escalation.
assert "worker: excludes an expected RED / ordinary debugging from escalation" \
  'grep -qiE "expected RED" <<<"$worker_body"'
assert "worker: escalation needs a concrete reason" \
  'grep -qiE "concrete reason" <<<"$worker_body"'

# Scope: it owns ONE task and must not rewrite earlier task commits.
assert "worker: owns exactly one task" 'grep -qiE "exactly one task|only that task" <<<"$worker_body"'
assert "worker: must not rewrite earlier task commits" \
  'grep -qiE "not rewrite|never rewrite" <<<"$worker_body"'

# An escalated worker inherits the worktree — it must account for uncommitted changes.
assert "worker: escalated worker must not blindly discard existing uncommitted work" \
  'grep -qiE "uncommitted" <<<"$worker_body"'

# Repository instructions outrank this generic contract.
assert "worker: repository instructions override the generic contract" \
  'grep -qF -- "AGENTS.md" <<<"$worker_body"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_docket_build.sh`
Expected: FAIL, starting with `NOT OK - worker: SKILL.md exists`.

- [ ] **Step 3: Write the worker skill**

Create `skills/docket-build-task/SKILL.md`:

````markdown
---
name: docket-build-task
description: The compact per-task worker contract for docket's own build role — owns exactly one plan task from focused test through implementation, verification, self-review, and one commit, returning COMPLETE, NEEDS_ESCALATION, or BLOCKED. Preloaded into the docket-build profile agents; not invoked directly by a human.
---

# docket-build-task — one plan task, one commit

You own **exactly one task** from the implementation plan, handed to you in your prompt along with
the branch, the worktree, the selected build profile, and the routing reason. You are a fresh
worker: nothing carries over from earlier tasks except the code and commits already on the branch.

You do not review other tasks, and you do not dispatch anyone. Your self-review is part of
implementing this task, not a second agent — never dispatch a reviewer, a fix agent, or any other
subagent, and never load a review skill.

## Scope

- Implement only that task. Work outside its boundary belongs to another worker.
- Never rewrite, amend, or revert earlier task commits, and never touch unrelated user work.
- Repository instructions — `AGENTS.md`, `CLAUDE.md`, and any nested equivalents — **override**
  this generic contract wherever they conflict. Read them before you write code.
- If you were dispatched as an **escalated** worker, the worktree may already hold uncommitted
  changes from the weaker worker's attempt. Inspect and account for every one of them. You may
  revise or replace them, but never discard them blindly and never `git checkout .` over them.

## The cycle

Where a meaningful behavioral test is possible:

1. Run the narrowest relevant tests to establish the baseline.
2. Add or identify a test that **fails for the intended reason** — read the failure and confirm it
   is the one you meant, not a typo or an import error.
3. Implement the smallest change that makes it pass.
4. Re-run the focused test set. Focused, not the whole suite: the controller runs the full suite
   once after every task.
5. Self-review the diff, then commit.

Two obligations the cycle does not relax:

- A bug fix requires a **failing regression test** that reproduces the bug before the fix.
- A guard requires **mutation evidence**: remove or defeat the thing being guarded and verify the
  guard turns red. A guard you never watched fail is decoration.

## Evidence-bound discretion

You may skip a literal RED/GREEN cycle only when a meaningful pre-implementation failure is
unavailable or actively misleading. When you do, your return must state all three of:

- **why RED/GREEN was unsuitable**;
- **what verification replaced it**;
- **what residual risk** remains.

"Small change", "hard to test", and "no existing tests" are **not** sufficient reasons.

Examples of genuine cases — illustrative, not an exhaustive allowlist:

- Documentation-only changes with no executable behavior change. Substitute the applicable lint,
  link, rendering, or precise inspection checks.
- Generated artifacts where the generator is unchanged and the task only refreshes its output.
  Verify reproducible regeneration and the expected diff. A change to the generator itself
  defaults back to TDD.
- Behavior-preserving refactors already covered by focused characterization tests. Establish green
  coverage before editing and prove it stays green — manufacturing a failing test here would
  misrepresent the intended behavior.
- Plan-required manual-only behavior with no meaningful automated assertion. Perform the specified
  manual or static verification and record the residual risk.

## The commit

A task produces a commit **only on success** — `COMPLETE` means focused verification is green and
**exactly one successful task commit** exists for this task. Never commit on `NEEDS_ESCALATION` or
`BLOCKED`: leave the worktree as it stands so the next worker or the human can read it.

## Outcomes

Return exactly one of three outcomes. A missing or malformed outcome halts the build, so state it
plainly.

- **`COMPLETE`** — focused verification is green and exactly one task commit exists.
- **`NEEDS_ESCALATION`** — the task proves materially more complex or riskier than the assigned
  profile, with a **concrete reason** naming what exceeded it. An expected RED test, ordinary
  debugging, or a single failed test run is **not** an escalation condition. You get at most one
  escalation per task, so spend it on genuine under-capacity, not on friction.
- **`BLOCKED`** — a stronger model cannot resolve this: missing authority, contradictory
  requirements, an absent dependency, or an unsafe condition. Name which.

## Your return

Keep it short. The controller keeps only this; there are no brief files, task reports, or review
records.

```text
OUTCOME: COMPLETE | NEEDS_ESCALATION | BLOCKED
PROFILE: <economy|standard|premium> — <one-line routing reason as given to you>
VERIFICATION: <the focused command you ran> -> <result>
TDD: <RED/GREEN evidence, or the three-part exception: why unsuitable / what replaced it / residual risk>
COMMIT: <sha, or "none" for a non-COMPLETE outcome>
NOTES: <only what the next worker or the PR genuinely needs — omit when there is nothing>
```
````

- [ ] **Step 4: Add the budget row and measure it**

Run: `wc -l skills/docket-build-task/SKILL.md && wc -w skills/docket-build-task/SKILL.md`

Apply the rounding rule from Global Constraints to the measured values, then add the row to `BUDGETS` in `tests/test_skill_size_budgets.sh`, keeping the table's alphabetical-by-path ordering (it sorts after `skills/docket-brainstorm/SKILL.md` and before `skills/docket-convention/SKILL.md`) and its column alignment. Add a one-line justification comment above the `BUDGETS=` block in the same style as the existing ones, naming change 0167 and the measured actual.

- [ ] **Step 5: Run both tests to verify they pass**

Run: `bash tests/test_docket_build.sh 2>&1 | tail -2 && bash tests/test_skill_size_budgets.sh 2>&1 | tail -2`
Expected: `PASS` from both.

- [ ] **Step 6: Mutation-test the contract guard**

Restore the file after each probe:
1. Delete the `NEEDS_ESCALATION` bullet from the Outcomes section → `bash tests/test_docket_build.sh` must report `NOT OK - worker: defines the NEEDS_ESCALATION outcome`.
2. Delete the "what residual risk" bullet → the matching clause assert must redden.
3. Truncate the file to 10 lines → the non-vacuity floor must redden (proving the negative asserts are not passing on an empty read).

- [ ] **Step 7: Commit**

```bash
git add skills/docket-build-task/SKILL.md tests/test_docket_build.sh tests/test_skill_size_budgets.sh
git commit -m "feat(0167): add the docket-build-task worker contract"
```

---

### Task 3: the three Claude profile agents and every roster surface

Adding `agents/docket-*.md` files is self-registering for `sync-agents.sh` (it globs), and `link-skills.sh` globs `skills/*/` — neither needs an edit. But several surfaces enumerate the roster or its **size** by hand and go red or stale otherwise. This task closes all of them together.

**Files:**
- Create: `agents/docket-build-economy.md`, `agents/docket-build-standard.md`, `agents/docket-build-premium.md`
- Create: `cursor-rules/dispatch/docket-build-economy.md`, `cursor-rules/dispatch/docket-build-standard.md`, `cursor-rules/dispatch/docket-build-premium.md`
- Modify: `.docket.example.yml` (the `agents.claude` mirror, the two commented harness mirrors, and the "9 wrapper files ship" prose)
- Modify: `README.md` (the two count sentences)
- Modify: `skills/docket-convention/SKILL.md` (the wrapper-count clause)
- Modify: `tests/test_sync_agents.sh` (wrapper count 9 → 12)
- Modify: `tests/test_docket_example_yml.sh` (the mirror-equality agent list)
- Modify: `tests/test_finalize_gate.sh` (the convention count-prose assert)
- Modify: `tests/test_skill_size_budgets.sh` only if the convention edit exceeds its row

**Interfaces:**
- Consumes: the skill name `docket-build-task` from Task 2 (each wrapper's `skills:` list).
- Produces: three dispatchable agent names — **`docket-build-economy`**, **`docket-build-standard`**, **`docket-build-premium`** — which Task 4's controller dispatches by name.

- [ ] **Step 1: Write the failing roster tests**

In `tests/test_sync_agents.sh`, change the wrapper-count assert to 12:

```bash
assert "all 12 wrappers land in .claude/agents" '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "12" ]'
```

In `tests/test_docket_example_yml.sh`, extend the mirror-equality loop's agent list — it must read:

```bash
for a in status adr brainstorm-consultant auto-groom auto-groom-critic \
         implement-next rebase-resolver integration-repair finalize-change \
         build-economy build-standard build-premium; do
```

In `tests/test_finalize_gate.sh`, update the count-prose assert to the new number:

```bash
assert "convention count prose says twelve wrappers" 'grep -qi "twelve" "$CONV"'
```

Then append this block to `tests/test_docket_build.sh`, immediately before its final `if [ "$fail" = 0 ]` line:

```bash
# ---------------------------------------------------------------------------
# The three Claude build-profile wrappers (change 0167)
# ---------------------------------------------------------------------------
fmv(){ awk 'NR==1 && $0=="---"{f=1;next} f && $0=="---"{exit} f{print}' "$1" \
        | sed -n "s/^$2:[[:space:]]*//p" | head -n1 | sed 's/[[:space:]]*$//'; }

# The ladder is a triple, and effort is the ONLY thing that differs. Asserting the efforts
# pairwise-distinct is what stops a copy-paste that silently makes all three the same agent.
efforts=""
for p in economy:low standard:medium premium:high; do
  name="${p%%:*}"; want="${p##*:}"
  w="$REPO/agents/docket-build-$name.md"
  assert "profile $name: wrapper exists" '[ -f "$w" ]'
  [ -f "$w" ] || continue
  assert "profile $name: name field matches its filename" '[ "$(fmv "$w" name)" = "docket-build-'"$name"'" ]'
  assert "profile $name: effort is $want" '[ "$(fmv "$w" effort)" = "'"$want"'" ]'
  assert "profile $name: model is set" '[ -n "$(fmv "$w" model)" ]'
  assert "profile $name: preloads the shared worker skill" \
    'grep -qF -- "docket-build-task" <<<"$(fmv "$w" skills)"'
  assert "profile $name: emits no maxTurns" '! grep -qiE "^maxTurns[[:space:]]*:" "$w"'
  efforts="$efforts $(fmv "$w" effort)"
done
assert "the three profiles carry three DISTINCT efforts" \
  '[ "$(tr " " "\n" <<<"$efforts" | grep -c .)" = 3 ] && [ "$(tr " " "\n" <<<"$efforts" | grep -c . )" = "$(tr " " "\n" <<<"$efforts" | grep . | sort -u | wc -l | tr -d " ")" ]'

# All three share one model — the profile axis is effort, not model. If a future change
# deliberately splits models, this assert is the place that must be updated consciously.
models="$(for n in economy standard premium; do fmv "$REPO/agents/docket-build-$n.md" model; done | sort -u)"
assert "the three profiles share one model" '[ "$(grep -c . <<<"$models")" = 1 ]'

# The IDs must NOT appear under agents.default in the example — Claude model IDs there would
# falsely present themselves as harness-portable (spec: "never the harness-neutral fallback").
EX="$REPO/.docket.example.yml"
default_blk="$(awk '/^#[[:space:]]*default:[[:space:]]*$/{inblk=1;next} inblk && /^#[[:space:]]{0,3}[a-z]/{inblk=0} inblk{print}' "$EX")"
assert "no build profile is documented under agents.default" \
  '! grep -qE "build-(economy|standard|premium)" <<<"$default_blk"'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_build.sh 2>&1 | grep -c "NOT OK"` and `bash tests/test_sync_agents.sh 2>&1 | grep "NOT OK"`
Expected: several `NOT OK` lines — `profile economy: wrapper exists` and friends, plus the 12-wrapper count.

- [ ] **Step 3: Create the three agent wrappers**

`agents/docket-build-economy.md`:

```markdown
---
name: docket-build-economy
description: Economy build-profile worker for docket-build — implements one fully-specified, low-risk plan task under the docket-build-task contract at low reasoning effort.
model: claude-opus-5
effort: low
skills: [docket-build-task]
---
Implement the single plan task handed to you, following the docket-build-task skill exactly.

You were routed to the ECONOMY profile because the task was judged fully specified, localized, pattern-following, and without consequential risk. If that judgment proves wrong, return NEEDS_ESCALATION with a concrete reason rather than pushing through — you get exactly one escalation, to STANDARD.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as BLOCKED and surface what blocked you, never an interactive prompt.
```

`agents/docket-build-standard.md`:

```markdown
---
name: docket-build-standard
description: Standard build-profile worker for docket-build — implements one normal feature, integration, refactor, or debugging plan task under the docket-build-task contract at medium reasoning effort.
model: claude-opus-5
effort: medium
skills: [docket-build-task]
---
Implement the single plan task handed to you, following the docket-build-task skill exactly.

You were routed to the STANDARD profile — the default for ordinary feature, integration, refactor, and debugging work, and the destination of an economy escalation. If the task proves materially riskier or more complex than that, return NEEDS_ESCALATION with a concrete reason; whether an escalation to PREMIUM is still available depends on where this task started, and the controller decides that, not you.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as BLOCKED and surface what blocked you, never an interactive prompt.
```

`agents/docket-build-premium.md`:

```markdown
---
name: docket-build-premium
description: Premium build-profile worker for docket-build — implements one high-risk or architecturally unresolved plan task under the docket-build-task contract at high reasoning effort.
model: claude-opus-5
effort: high
skills: [docket-build-task]
---
Implement the single plan task handed to you, following the docket-build-task skill exactly.

You were routed to the PREMIUM profile because the task touches a named risk — authentication or a security boundary, a migration or irreversible data change, concurrency or locking, release infrastructure, or unresolved architecture — or because a weaker worker escalated to you. Premium means greater reasoning investment, not a stronger correctness guarantee: your testing and completion obligations are identical to every other profile.

There is no profile above you. If you cannot complete the task, return BLOCKED with a concrete reason and the build halts for a human — do not lower the bar to produce a commit.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as BLOCKED and surface what blocked you, never an interactive prompt.
```

- [ ] **Step 4: Create the three Cursor dispatch fragments**

A missing fragment only warns, but leaving it out ships an un-curated auto-block. Create each in the shape of `cursor-rules/dispatch/docket-adr.md`. `cursor-rules/dispatch/docket-build-economy.md`:

```markdown
## docket-build-economy — dispatch only

Trigger only from the `docket-build` controller, when it has routed a plan task to the ECONOMY
profile. Never trigger this agent from a human request directly.

Dispatch to the subagent `docket-build-economy`, foreground, using this mode's subagent-launch
mechanism. The prompt must carry the plan task, the branch and worktree, the selected profile and
its routing reason, and the completion schema from the docket-build-task skill.

Do NOT implement the task in the parent, and do NOT dispatch a reviewer after it.

One concrete call, as an illustration of the shape — not the contract:

    Task(subagent_type: "docket-build-economy", run_in_background: false,
         prompt: "Task 3 of <plan path>. Profile: economy (fully specified, one file, established pattern). <task text>")
```

Create `docket-build-standard.md` and `docket-build-premium.md` the same way, substituting the profile name in every position and its routing reason in the illustrative call (`standard (ordinary integration work)`, `premium (touches locking)`). Write each file out in full — do not abbreviate one as "same as above".

- [ ] **Step 5: Update `.docket.example.yml`**

Three edits:

1. In the `agents:` prose paragraph, change `9 wrapper files ship (agents/docket-*.md, mirrored below): 5 wrap one of the 5 autonomous skills (adr, auto-groom, finalize-change, implement-next, status); the other 4 (auto-groom-critic, brainstorm-consultant, integration-repair, rebase-resolver) are standalone agents those skills dispatch and wrap no skill of their own.` to:

```
# 12 wrapper files ship (agents/docket-*.md, mirrored below): 5 wrap one of the 5 autonomous
# skills (adr, auto-groom, finalize-change, implement-next, status); 3 (build-economy,
# build-standard, build-premium) are docket-build's task workers and all preload the same
# docket-build-task worker skill, differing only in model/effort; the other 4 (auto-groom-critic,
# brainstorm-consultant, integration-repair, rebase-resolver) are standalone agents those skills
# dispatch and wrap no skill of their own.
```

Re-wrap to the file's comment width; do not leave an over-long line.

2. Append three rows to the commented `claude:` mirror, keeping the column alignment:

```
#     build-economy:         { model: claude-opus-5,             effort: low }
#     build-standard:        { model: claude-opus-5,             effort: medium }
#     build-premium:         { model: claude-opus-5,             effort: high }
```

3. Append matching rows to the doubly-commented `codex:` and `cursor:` mirrors so the harness blocks stay complete, using each block's existing example ID style:

```
#   #   build-economy:         { model: gpt-5.6-sol, effort: low }
#   #   build-standard:        { model: gpt-5.6-sol, effort: medium }
#   #   build-premium:         { model: gpt-5.6-sol, effort: high }
```

```
#   #   build-economy:         { model: cursor-grok-4.5-low-fast, effort: auto }
#   #   build-standard:        { model: cursor-grok-4.5-medium, effort: auto }
#   #   build-premium:         { model: cursor-grok-4.5-high, effort: auto }
```

The mirror-equality assert only checks the `claude:` block against the wrappers; the other two are unvalidated examples, exactly as the surrounding comment already says.

- [ ] **Step 6: Update the count prose in README and the convention**

In `README.md`, the sentence containing `mirrors the shipped defaults for all nine subagents ... instead of opening nine wrapper files` becomes `... for all twelve subagents ... instead of opening twelve wrapper files`. The sentence containing `(the nine generated agents)` becomes `(the twelve generated agents)`. The note reading `it's why you'll find nine directories under skills/ even though that table lists eight` becomes `it's why you'll find eleven directories under skills/ even though that table lists eight — docket-brainstorm plus the two build-role skills, docket-build and docket-build-task.`

In `skills/docket-convention/SKILL.md`, the clause `Four of the **nine** generated wrappers wrap **no skill**` becomes `Four of the **twelve** generated wrappers wrap **no skill**`, and the closing parenthetical `(Five *skills* get a wrapper — nine wrappers, five skills.)` becomes `(Twelve wrappers: five wrap the five autonomous skills, three are docket-build's task workers sharing the `docket-build-task` contract, four wrap none.)`

- [ ] **Step 7: Re-measure the convention budget**

The convention file has very little margin. Run:

`wc -l skills/docket-convention/SKILL.md && wc -w skills/docket-convention/SKILL.md`

Compare against its `BUDGETS` row (365 lines / 6250 words). If either measurement now exceeds its limit, raise **that row only**, applying the rounding rule from Global Constraints, and add a justification comment in the existing style naming change 0167. If both still fit, change nothing.

- [ ] **Step 8: Run the affected tests to verify they pass**

Run: `bash tests/test_docket_build.sh 2>&1 | tail -2 && bash tests/test_sync_agents.sh 2>&1 | tail -2 && bash tests/test_docket_example_yml.sh 2>&1 | tail -2 && bash tests/test_finalize_gate.sh 2>&1 | tail -2 && bash tests/test_skill_size_budgets.sh 2>&1 | tail -2`
Expected: `PASS` from all five.

- [ ] **Step 9: Verify generation really works, and that it is idempotent**

Run `bash sync-agents.sh` then `bash sync-agents.sh --check`, and confirm the three new wrappers appear in the generated harness directory with the intended `model:`/`effort:`, that `--check` reports no drift on the second run, and that no `WARN`/`ERROR` line mentions a build profile. A generated file whose effort line is missing means an `effort: auto` leaked in — fix the wrapper, not the test.

- [ ] **Step 10: Mutation-test the new roster guards**

Restore after each probe:
1. Change `effort: low` to `effort: medium` in `agents/docket-build-economy.md` → both `profile economy: effort is low` and `the three profiles carry three DISTINCT efforts` must redden.
2. Remove `docket-build-task` from one wrapper's `skills:` list → the preload assert must redden.
3. Add `build-economy: { model: claude-opus-5, effort: low }` under the example's commented `default:` block → the `no build profile is documented under agents.default` assert must redden. This is the assert that encodes the spec's harness-portability rule, so prove it bites.

- [ ] **Step 11: Commit**

```bash
git add agents/docket-build-economy.md agents/docket-build-standard.md agents/docket-build-premium.md \
        cursor-rules/dispatch/docket-build-economy.md cursor-rules/dispatch/docket-build-standard.md \
        cursor-rules/dispatch/docket-build-premium.md \
        .docket.example.yml README.md skills/docket-convention/SKILL.md \
        tests/test_docket_build.sh tests/test_sync_agents.sh tests/test_docket_example_yml.sh \
        tests/test_finalize_gate.sh tests/test_skill_size_budgets.sh
git commit -m "feat(0167): ship the three Claude build-profile agents"
```

---

### Task 4: `docket-build` — the controller skill

**Files:**
- Create: `skills/docket-build/SKILL.md`
- Modify: `tests/test_docket_build.sh` (controller half)
- Modify: `tests/test_skill_size_budgets.sh` (one `BUDGETS` row)

**Interfaces:**
- Consumes: `BUILD_CHECKPOINT` (Task 1); the outcome tokens `COMPLETE` / `NEEDS_ESCALATION` / `BLOCKED` and the worker contract name `docket-build-task` (Task 2); the three agent names `docket-build-economy` / `-standard` / `-premium` (Task 3).
- Produces: the skill name **`docket-build`**, bound in Task 5's `.docket.yml` under `skills: build:`.

- [ ] **Step 1: Write the failing controller guard**

Insert this block into `tests/test_docket_build.sh` between the worker section and the profile-wrapper section:

```bash
# ---------------------------------------------------------------------------
# docket-build — the controller contract
# ---------------------------------------------------------------------------
CTRL="$REPO/skills/docket-build/SKILL.md"
assert "controller: SKILL.md exists" '[ -f "$CTRL" ]'
ctrl_body="$(cat "$CTRL" 2>/dev/null)"
assert "controller: contract is non-vacuous (>= 50 lines)" \
  '[ "$(printf "%s\n" "$ctrl_body" | grep -c .)" -ge 50 ]'

# It must dispatch by AGENT NAME — the whole point of the change is that model and effort are
# properties of a named agent rather than an ad-hoc per-dispatch argument.
for a in docket-build-economy docket-build-standard docket-build-premium; do
  assert "controller: names the $a agent" 'grep -qF -- "'"$a"'" <<<"$ctrl_body"'
done

# The routing rubric, with its deliberate asymmetry.
assert "controller: economy must be POSITIVELY established" \
  'grep -qiE "economy[^.]{0,120}(only when|positively)" <<<"$ctrl_body"'
assert "controller: named risk selects premium" \
  'grep -qiE "premium[^.]{0,200}(authentication|security boundar)" <<<"$ctrl_body"'
assert "controller: uncertainty defaults to standard" \
  'grep -qiE "(uncertainty|remaining|otherwise)[^.]{0,80}standard|standard[^.]{0,80}(default|remaining)" <<<"$ctrl_body"'

# The plan override and its fail-loud contract.
assert "controller: honors an explicit plan Build profile override" \
  'grep -qF -- "Build profile:" <<<"$ctrl_body"'
assert "controller: an invalid explicit profile HALTS rather than falling back" \
  'grep -qiE "invalid[^.]{0,120}halt" <<<"$ctrl_body"'

# The escalation ladder — all three edges, including the terminal one.
assert "controller: economy escalates to standard" \
  'grep -qiE "economy[^.]{0,40}(->|→|to)[^.]{0,20}standard" <<<"$ctrl_body"'
assert "controller: standard escalates to premium" \
  'grep -qiE "standard[^.]{0,40}(->|→|to)[^.]{0,20}premium" <<<"$ctrl_body"'
assert "controller: premium escalation halts" \
  'grep -qiE "premium[^.]{0,40}(->|→|to)?[^.]{0,20}halt" <<<"$ctrl_body"'
assert "controller: at most ONE escalation per task" \
  'grep -qiE "(at most once|one escalation|single .{0,20}escalation)" <<<"$ctrl_body"'
assert "controller: a retried task does not climb twice" \
  'grep -qiE "does not climb|never climbs|not climb again" <<<"$ctrl_body"'

# NO REVIEW inside the build — the defining property of this topology.
assert "controller: performs no per-task review" \
  'grep -qiE "no per-task[^.]{0,40}review" <<<"$ctrl_body"'
assert "controller: performs no final review of its own" \
  'grep -qiE "no final review|no whole-branch review of its own" <<<"$ctrl_body"'
assert "controller: hands the single review to docket-implement-next Step 6" \
  'grep -qiE "skills.review|Step 6" <<<"$ctrl_body"'

# The full-suite gate is DERIVED, never a second config key or a hand-copied fragment.
assert "controller: full-suite gate reads finalize.test_command" \
  'grep -qF -- "FINALIZE_TEST_COMMAND" <<<"$ctrl_body"'
assert "controller: falls back to finalize's existing auto-detection" \
  'grep -qiE "auto-detect" <<<"$ctrl_body"'
assert "controller: cites finalize's canonical suite-command block rather than copying it" \
  'grep -qF -- "configured-bash-finalize" <<<"$ctrl_body"'
# SINGLE SOURCE: the canonical fragment lives in finalize's SKILL.md and nowhere else. A second
# marker pair here would be the duplicate this change exists to avoid.
assert "controller: does not open a second configured-bash-finalize marker block" \
  '[ "$(grep -cF -- "<!-- configured-bash-finalize:start -->" "$CTRL")" = 0 ]'
assert "controller: introduces no second test-command config key" \
  '! grep -qiE "build\.test_command|BUILD_TEST_COMMAND" <<<"$ctrl_body"'

# A red suite becomes ONE synthetic repair task, not a repair/review loop.
assert "controller: a red suite does not invoke review" \
  'grep -qiE "red[^.]{0,80}(does not|never)[^.]{0,40}review" <<<"$ctrl_body"'
assert "controller: red suite becomes one integration-repair task" \
  'grep -qiE "integration.repair" <<<"$ctrl_body"'
assert "controller: repair ladder is standard -> premium -> halt" \
  'grep -qiE "standard[^.]{0,60}premium[^.]{0,60}halt" <<<"$ctrl_body"'

# Checkpointing: off by default, and the ledger path is exact.
assert "controller: reads BUILD_CHECKPOINT" 'grep -qF -- "BUILD_CHECKPOINT" <<<"$ctrl_body"'
assert "controller: names the ledger path" \
  'grep -qF -- ".superpowers/docket-build/" <<<"$ctrl_body"'
assert "controller: skips a resumed task only on COMPLETE + plan hash + ancestor commit" \
  'grep -qiE "ancestor" <<<"$ctrl_body"'

# Tier C: an un-dispatchable build halts unless the human explicitly configured auto.
assert "controller: un-dispatchable profile routing halts (Tier C)" \
  'grep -qiE "Tier C" <<<"$ctrl_body"'
assert "controller: cites the convention's dispatch-capability resolution" \
  'grep -qiF -- "Dispatch-capability resolution" <<<"$ctrl_body"'
assert "controller: forbids concluding unavailability from a tool name" \
  'grep -qF -- "never from a tool name" <<<"$ctrl_body"'

# A malformed worker return is never read as success.
assert "controller: a missing or malformed outcome halts" \
  'grep -qiE "(missing or malformed|malformed)[^.]{0,60}halt" <<<"$ctrl_body"'
assert "controller: never infers success from a child reporting it finished" \
  'grep -qiE "never infer" <<<"$ctrl_body"'
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_docket_build.sh 2>&1 | grep "NOT OK" | head -5`
Expected: `NOT OK - controller: SKILL.md exists` plus the rest of the controller block.

- [ ] **Step 3: Write the controller skill**

Create `skills/docket-build/SKILL.md`. It carries **no** `context: fork` and **no** `agent:` frontmatter — it is a role skill invoked inline by `docket-implement-next` Step 5 through the `skills.build` binding, exactly like the superpowers default it replaces.

````markdown
---
name: docket-build
description: Use as docket's build role (skills.build) — executes an implementation plan task-by-task by routing each task to a named economy/standard/premium Claude profile agent running the docket-build-task contract, with one bounded escalation per task, no per-task review, and a single full-suite gate at the end.
---

# docket-build — profile-routed plan execution

The lean alternative to `superpowers:subagent-driven-development`. You are already running inside
`docket-implement-next` Step 5 with the plan written and the feature worktree cut. You read the
plan, route each task to a profile, dispatch one fresh worker per task, apply the escalation
protocol, and run the build gate. Then you stop — review is not yours.

You are not a router subagent: routing is a decision you make in this context. Each selected task
gets exactly one fresh worker dispatch unless that worker requests its single allowed escalation.

## Profiles

Three named agents, all preloading the same `docket-build-task` worker skill and differing only in
model and effort:

| Agent | Effort | Use |
|---|---|---|
| `docket-build-economy` | low | fully specified, localized, pattern-following, no consequential risk |
| `docket-build-standard` | medium | normal feature, integration, refactor, and debugging work |
| `docket-build-premium` | high | named risk, or unresolved architecture |

`premium` means greater reasoning investment, **not** a stronger correctness guarantee — every
profile carries identical testing and completion obligations. Model and effort resolve through
docket's ordinary generated-agent layer and may be overridden at the global, repo-committed, or
repo-local layer; never restate literal model IDs in your dispatch prose.

## Routing

**Explicit override wins.** A plan task may carry a line of the form:

```markdown
**Build profile:** economy
```

A valid value (`economy`, `standard`, `premium`) is authoritative; record its use in that task's
routing line. An **invalid** value is a plan contract error: **halt** and surface it — never
silently fall back to a default.

**Otherwise classify**, with a deliberate asymmetry — `economy` must be *positively* established,
a named risk selects `premium`, and uncertainty defaults to `standard`:

- **`premium`** when the task involves authentication or security boundaries, migrations or
  irreversible data changes, concurrency or locking, release infrastructure, or unresolved
  architecture.
- **`economy`** *only when* the task is fully specified, localized to roughly one or two files,
  follows an established pattern, and carries no consequential risk.
- **`standard`** for everything remaining.

Emit one concise routing line per task naming both the profile and its reason.

## Dispatching a task

Dispatch the profile agent **by name**, foreground, one task at a time — later tasks build on
earlier task commits and share the worktree, so workers are strictly sequential. Give the worker:
the plan task text, the branch and worktree, the applicable repository instructions, the selected
profile and routing reason, and the completion schema. Never preload a review skill, never dispatch
a task reviewer, and never dispatch two workers concurrently.

If profile dispatch is genuinely unavailable — established only per the convention's
*Dispatch-capability resolution*, **never from a tool name** — this role is **Tier C,
authorized-or-halt**: only an explicitly configured `skills.build: auto` authorizes inline
execution. Selecting `docket-build` is not implicit authorization to discard its isolation or its
model/effort contract, so abort-and-report instead, leaving the change `in-progress`.

## Reading a worker's return

Valid outcomes are `COMPLETE`, `NEEDS_ESCALATION`, and `BLOCKED`. A **missing or malformed outcome
halts** the build. Never infer success from a child merely reporting that it finished — a
`COMPLETE` claim must come with the focused verification result and a commit SHA, and a task
without a commit is not complete.

## Escalation

Each task may escalate automatically **at most once**:

```text
initial economy  -> one standard retry
initial standard -> one premium retry
initial premium  -> halt
```

The retry consumes that task's whole escalation allowance: a task that started at `economy` and
whose `standard` retry still cannot complete **halts** — it does not climb again to `premium`.

Escalate only on a concrete reason that the task is materially more complex or riskier than the
assigned profile. An expected RED test, ordinary debugging, or a single failed test run is not an
escalation condition; a worker returning `NEEDS_ESCALATION` without such a reason is a malformed
return.

The stronger worker continues in the **same worktree** and must inspect and account for any
uncommitted changes the weaker worker left — revising them is allowed, discarding them blindly is
not. A successful escalation continues this run automatically.

Return `halted` — change still `in-progress`, worktree preserved for inspection or resume — when a
premium worker requests escalation, an escalated worker still cannot finish, requirements
contradict, authority or dependencies are missing, or continuation is unsafe.

## The build gate

Workers run focused tests only. After every plan task has committed, run the **whole suite once**:

1. Use the already-resolved `FINALIZE_TEST_COMMAND` when it is non-empty.
2. Otherwise reuse finalize's existing suite **auto-detection**.

The command boundary is the one finalize already publishes — its `configured-bash-finalize` marker
block in `skills/docket-finalize-change/SKILL.md` is the single source, and the awkward `finalize`
namespace is deliberately kept rather than introducing a second, driftable test command. Do not
copy that fragment into this file.

**Green** → the build is done; `docket-implement-next` Step 6 runs the resolved `skills.review`
role once over the whole branch.

**Red** → the build performs **no review**. Turn the failure into exactly one synthetic
integration-repair task, run through the same worker contract on the ladder
`standard -> premium -> halt`. The repair worker diagnoses the cross-task failure, adds regression
coverage where appropriate, fixes it, re-runs the full suite, and commits the repair. There is no
repeated repair/review loop; failure after the premium repair path returns `halted`.

## Review boundary

This build performs **no per-task independent review** and **no final review of its own**. The
worker's self-review is part of implementation, not a second agent or an adversarial gate. Docket's
single independent whole-branch review remains `docket-implement-next` Step 6's `skills.review`
role, which stays separately configurable.

## Checkpointing

Read `BUILD_CHECKPOINT` from the Step-0 config export.

**`false` (default)** — persist nothing. Completed work is durable through the per-task code
commits; keep only the compact in-context worker returns; write no `.superpowers/docket-build/`
files. A resumed run reconstructs progress conservatively from the plan, the commits, the code, and
the tests rather than trusting a formal receipt.

**`true`** — write a compact ledger to `.superpowers/docket-build/<change-id>/progress.md` (covered
by the committed `.superpowers/` ignore rule) recording branch, plan path and blob hash, task
identity and status, profile and reason, escalation, TDD evidence or exception, verification, and
commit SHA. It is a state ledger, not a prose task report. On resume, skip a task **only** when its
ledger entry is `COMPLETE`, the plan hash still matches, and its commit is an **ancestor** of the
current branch — missing, stale, malformed, or contradictory state never marks a task complete.

## Output

Emit concise, stable lines and nothing more: task-to-profile selection and reason; escalation and
reason; worker outcome and commit; focused verification; full-suite command and result; the
terminal build disposition. Write no verbose task artifact unless `BUILD_CHECKPOINT` is `true`.
Material TDD exceptions and residual risks flow into the PR description or the results artifact,
not into per-task files.
````

- [ ] **Step 4: Add the budget row**

Run `wc -l skills/docket-build/SKILL.md && wc -w skills/docket-build/SKILL.md`, apply the rounding rule, and add the row to `BUDGETS` in path order (it sorts after `skills/docket-brainstorm/SKILL.md` and before `skills/docket-build-task/SKILL.md`), with a justification comment naming change 0167 and the measured actual.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_docket_build.sh 2>&1 | tail -2 && bash tests/test_skill_size_budgets.sh 2>&1 | tail -2`
Expected: `PASS` from both.

- [ ] **Step 6: Mutation-test the controller guard**

Restore after each probe:
1. Delete the `initial premium -> halt` line from the escalation ladder → `controller: premium escalation halts` must redden.
2. Change "no per-task independent review" to "a per-task review" → the no-review assert must redden.
3. Add a `<!-- configured-bash-finalize:start -->` marker into the controller file → the single-source assert must redden. This is the assert that keeps the suite fragment from being duplicated, so prove it bites.
4. Delete the `Tier C` sentence → the Tier-C assert must redden.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-build/SKILL.md tests/test_docket_build.sh tests/test_skill_size_budgets.sh
git commit -m "feat(0167): add the docket-build controller skill"
```

---

### Task 5: dogfood the opt-in and document the feature

**Files:**
- Modify: `.docket.yml` (this repo's own opt-in)
- Modify: `README.md` (the build-role documentation)
- Test: `tests/test_docket_build.sh` (opt-in + docs guards)

**Interfaces:**
- Consumes: the skill name `docket-build` (Task 4), the config leaf `build.checkpoint` (Task 1), the three agent names (Task 3).
- Produces: nothing consumed by a later task — this is the terminal task.

- [ ] **Step 1: Write the failing opt-in guard**

Append to `tests/test_docket_build.sh`, immediately before its final `if [ "$fail" = 0 ]` line:

```bash
# ---------------------------------------------------------------------------
# Dogfood: this repo opts in, the shipped default does NOT change
# ---------------------------------------------------------------------------
DY="$REPO/.docket.yml"
dy_body="$(cat "$DY")"
assert "repo opts skills.build in to docket-build" \
  'grep -qE "^[[:space:]]+build:[[:space:]]+docket-build[[:space:]]*$" <<<"$dy_body"'
assert "repo pins build.checkpoint explicitly" \
  'grep -qE "^[[:space:]]+checkpoint:[[:space:]]+(true|false)[[:space:]]*$" <<<"$dy_body"'

# The SHIPPED cross-harness default must stay SDD — the opt-in is this repo's, not everyone's.
# Anchored on the resolver, which is what actually decides the default.
sdd_default="$(grep -E 'SKILL_BUILD=|skill_role build' "$REPO/scripts/docket-config.sh")"
assert "shipped skills.build default is still superpowers SDD" \
  'grep -qF -- "superpowers:subagent-driven-development" <<<"$sdd_default"'

# The knob is documented for users, not only implemented (config-knob-ship-end-to-end).
RM="$REPO/README.md"
rm_body="$(cat "$RM")"
assert "README documents the docket-build role" 'grep -qF -- "docket-build" <<<"$rm_body"'
assert "README documents the three profiles" \
  'grep -qF -- "economy" <<<"$rm_body" && grep -qF -- "premium" <<<"$rm_body"'
assert "README documents build.checkpoint" 'grep -qF -- "build.checkpoint" <<<"$rm_body"'
assert "README says how to opt back into SDD" \
  'grep -qF -- "superpowers:subagent-driven-development" <<<"$rm_body"'
assert "README states the Claude-only support boundary for the profiles" \
  'grep -qiE "docket-build[^.]{0,200}(claude-only|Claude Code only|only.{0,20}Claude)" <<<"$rm_body"'
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_docket_build.sh 2>&1 | grep "NOT OK"`
Expected: the opt-in and README asserts red; the earlier sections still green.

- [ ] **Step 3: Opt this repo in**

In `.docket.yml`, append after the `terminal_publish: true` stanza:

```yaml

# This repo dogfoods docket's own build role (change 0167): each plan task is routed to a named
# economy/standard/premium profile agent instead of SDD's implementer+reviewer pairs. The SHIPPED
# cross-harness default stays superpowers:subagent-driven-development — see .docket.example.yml.
skills:
  build: docket-build

build:
  checkpoint: false            # no resume ledger; per-task commits are the durability

# Mirrors the shipped profile defaults so they are visible and tunable in one place. Claude only —
# Cursor and Codex profile mappings land with changes 0168 and 0169.
agents:
  claude:
    build-economy:  { model: claude-opus-5, effort: low }
    build-standard: { model: claude-opus-5, effort: medium }
    build-premium:  { model: claude-opus-5, effort: high }
```

Note `agents:` is **presence-sensitive** — adding it opts this repo into per-repo wrapper generation. That is intended here (it is the dogfood), and `sync-agents.sh` regenerates the full set regardless.

- [ ] **Step 4: Verify the opt-in resolves**

Run: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh env 2>/dev/null | grep -E "SKILL_BUILD|BUILD_CHECKPOINT"`
Expected: `SKILL_BUILD=docket-build` and `BUILD_CHECKPOINT=false`.

Then run `bash sync-agents.sh --check` and confirm it reports no drift and no error for the three profile agents.

- [ ] **Step 5: Document the feature in README**

Add a subsection to the README section that already covers the pluggable `skills:` roles (near the `docket-brainstorm` role discussion), titled `#### docket-build — the lean, profile-routed build`. It must cover, in prose:

- what it replaces and why (SDD's per-task implementer+reviewer pairs and duplicate whole-branch review; roughly `T + 1` nested runs across build and review instead of SDD's `2T + 2` clean path);
- how to select it (`skills: build: docket-build`) and how to opt back out (`skills: build: superpowers:subagent-driven-development`, the shipped default, which stays unchanged for everyone who does nothing);
- the three profiles and the routing rubric, including that `economy` must be positively established and uncertainty defaults to `standard`;
- the plan-task override syntax `**Build profile:** economy` and that an invalid value halts;
- the one-escalation-per-task ladder and that a premium failure halts for a human;
- that the build runs **one** full-suite gate derived from `finalize.test_command` (no second config key) and performs no review of its own, leaving `skills.review` as the sole review gate;
- `build.checkpoint`, default `false`, and what `true` persists;
- the **Claude-only** support boundary: the three profile agents ship Claude model IDs under `agents.claude`, so Cursor and Codex users should stay on the default until changes 0168 and 0169 land.

Keep it consistent with the surrounding README voice; do not restate the routing rubric verbatim from the skill file — link to the skill for the full contract.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `bash tests/test_docket_build.sh 2>&1 | tail -2 && bash tests/test_docket_example_yml.sh 2>&1 | tail -2`
Expected: `PASS` from both.

- [ ] **Step 7: Run the WHOLE suite**

Per `AGENTS.md`, focused tests are not completion evidence. Run every test with the configured runtime, from the worktree root:

```bash
for t in tests/test_*.sh; do
  printf '%s: ' "$t"
  "${DOCKET_BASH_PATH:-bash}" "$t" 2>&1 | tail -1
done
```

Expected: `PASS` on every line. Investigate any `FAIL` before committing — a test that this change made stale must be repaired at the surface that actually owns the content, not by re-adding deleted prose to keep a grep green.

- [ ] **Step 8: Commit**

```bash
git add .docket.yml README.md tests/test_docket_build.sh
git commit -m "feat(0167): dogfood docket-build and document the build role"
```

---

## Self-review notes

**Spec coverage.** Build controller + worker skill → Tasks 2 and 4. Three Claude profile agents with model/effort and no `maxTurns` → Task 3. `agents.claude` placement rule → Task 3 (with its own negative assert). Explicit plan override and invalid-value halt → Task 4. Automatic classification rubric with its asymmetry → Task 4. Escalation ladder, one per task, same-worktree continuation, uncommitted-work rule → Tasks 2 and 4. Worker protocol, outcomes, malformed-outcome halt → Tasks 2 and 4. TDD default and evidence-bound discretion with all three required statements → Task 2. Full-suite gate from `finalize.test_command` / auto-detection, single-source fragment → Task 4. Red-suite integration repair on `standard -> premium -> halt`, no repair/review loop → Task 4. Review boundary → Tasks 2 and 4. `build.checkpoint` resolution, precedence, malformed-boolean failure, ledger path and resume rule → Tasks 1 and 4. Observability lines and cost bound → Task 4. Configuration + documentation changes, dogfood opt-in, shipped-default preservation → Tasks 1, 3, and 5. Generated-agent verification (generation, idempotent `--check`, no `maxTurns`, harness non-representation) → Task 3 Steps 9–10. Skill contract and mutation tests → the mutation step in every task. Whole-suite run → Task 5 Step 7.

Deliberately **not** tasks, per the reconcile: the ADR superseding ADR-0023 (docket-implement-next Step 6), closing change 0044 (already killed), and minting changes 0168/0169/0170 (already exist).

The spec's fourth verification level — a live Claude Code smoke test through all three profiles with a real escalation — cannot run inside this build (it requires a separate multi-task fixture run under the new controller). Task 5's full-suite run plus Task 3's real `sync-agents.sh` generation check are the automated substitute; the residual gap belongs in the results file as a manual check for the merge gate.
