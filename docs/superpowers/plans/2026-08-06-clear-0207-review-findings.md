<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0220 — clear the unfixed review findings from change 0207](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0220-clear-the-unfixed-review-findings-from-change-0207.md)**
<!-- docket:backlink:end -->

# Clear the unfixed review findings from change 0207 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Work all seven `docket-review-deep` findings from change 0207's PR #159 — one real
under-enumeration bug in `sync-agents.sh`'s `--check` path, one duplicate-diagnostic bug, one
unenforced calling contract, two test-coverage holes, and two comment corrections.

**Architecture:** Every edit lands in two files — `sync-agents.sh` (the generator) and
`tests/test_sync_agents.sh` (the suite that already owns every `runner:` fixture). The structural
fix is a single named predicate, `project_wrappers_generated`, shared by the three places that
decide whether per-repo wrappers exist: the writer (`project_level_pass`), the gate
(`validate_runner_config`'s project-level leg), and the `--check` drift loops
(`check_project_level`'s leg (c)). No new files, no new scripts, no signature changes.

**Tech Stack:** POSIX-ish bash (the script runs under `set -euo pipefail`), the repo's own
hand-rolled `assert` test harness in `tests/`.

## Global Constraints

Copied verbatim from the spec's *Assumptions* and the repo's promoted learnings:

- **bash 3.2-safe.** No associative arrays, no `declare -A`, no `${var^^}`. Dedup accumulates in a
  newline-delimited string.
- **`set -euo pipefail` is active in `sync-agents.sh`** (line 53). A bare failing command as the
  last element of a `&&`/`||` list triggers errexit — write every conditional grep as a full
  `if ... ; then ... fi`, never `grep ... && return 0`.
- **`/usr/bin/grep` portability.** The developer's PATH `grep` is ugrep and masks BSD bugs. New
  greps use BSD-safe expressions only: no `-P`, no `{n,m}` intervals above 255, no GNU-only flags.
  `-F -x -q` is safe.
- **No new suite file, no new script.** Every fixture lands in `tests/test_sync_agents.sh`.
- **No signature changes to `emit_wrapper`.** It keeps `$1..$6`; the contract is documented and
  asserted, not rerouted (spec D3 rejects the reroute explicitly).
- **Scope stays inside 0207's settled design.** The atomicity invariant and the all-or-nothing
  strictness trade are not reopened; Gate 2's pre-migration blind spot stays out.
- **Mutation-test every new assert.** Per learning `assert-detects-removal-not-replacement`: prove
  the assert reddens when the code it guards is removed, and prove the mutation actually landed —
  do not trust a green run as evidence the assert works.

## File Structure

- **Modify: `sync-agents.sh`** — one new predicate function; two guard sites changed; one
  dedup helper; one assertion added to `emit_wrapper`; four comment blocks corrected.
- **Modify: `tests/test_sync_agents.sh`** — one existing assert replaced (the vacuous
  `--check wrote no wrappers`), one existing assert made discriminating, three new fixtures
  appended to the change-0207 block.

Both files are large and established; follow their existing shape (long explanatory comments above
each non-obvious block, `assert "<change>/<tag>: <claim>" '<expression>'`).

---

### Task 1: Share one predicate between the gate and leg (c), and correct the comments it falsified

Spec decisions **D1** (findings 1 and 7) and **D5** (finding 5). The gate's project-level leg is
guarded by `per_repo_opted_in`; `check_project_level`'s leg (c) — the third `emit_wrapper` call
site — is guarded by the strictly weaker `gitignore_block_wanted`. A global `agent_harnesses:` list
without `claude`, plus a bad `agents.claude.*` `runner:`, in a repo that is not opted in, escapes
the gate and dies inside leg (c) on `emit_wrapper`'s can't-happen assertion: raw `ERROR` + `exit 1`,
skipping the remaining `--check` legs and leaking leg (c)'s `mktemp -d`.

D5 folds in here because the comment it corrects is `validate_runner_config`'s own header — the
same comment block D1 must edit.

**Files:**
- Modify: `sync-agents.sh` — add `project_wrappers_generated` next to `per_repo_opted_in`
  (~line 222-230); use it in `project_level_pass` (~line 1205), in `validate_runner_config`'s
  project-level leg (~line 648), and in `check_project_level`'s two drift loops (~line 1292 and
  ~line 1312); correct the header comment at ~line 607-610 (D5), the `emit_wrapper` can't-happen
  comment at ~line 945-950, and the `--check` dispatch comment at ~line 1431-1433.
- Test: `tests/test_sync_agents.sh` — replace the vacuous `0207: --check wrote no wrappers`
  assert (~line 1600) and append the D1 regression fixture after the change-0207 block.

**Interfaces:**
- Produces: `project_wrappers_generated()` — returns 0 when this run generates per-repo wrappers
  into `<repo>/.<harness>/agents`, 1 otherwise. Currently a pure delegation to `per_repo_opted_in`.
  Tasks 2–5 do not consume it.

- [ ] **Step 1: Write the failing test — the D1 regression fixture**

Append this to `tests/test_sync_agents.sh`, immediately after the change-0207 block's
`0205: and its wrapper is still native` fixture (the `rm -rf "$SBX"` at ~line 1625):

```bash
# ---- change 0220 / D1: the gate and leg (c) share ONE predicate -------------------------------
# The gap this closes: with a GLOBAL agent_harnesses: list that omits claude, $USER_TARGETS has no
# claude, so the gate's user-level leg never sees agents.claude.*; in a repo that is NOT opted in
# the gate's project-level leg `continue`s; but leg (c) iterated $HARNESSES (default claude) and
# called emit_wrapper, which died on its can't-happen assertion — raw ERROR + exit 1, skipping the
# remaining --check legs and leaking leg (c)'s mktemp -d.
#
# The assertion shape is INVERTED from the obvious one on purpose. In this fixture the gate itself
# is legitimately silent (runner_config_error returns 0 on the user leg because USER_TARGETS has no
# claude; the project leg continues), and on the --check path a FAILING gate exit 1s before
# check_project_level runs at all — so "the gate now catches it" and "the remaining legs still run
# after a gate failure" both describe unreachable paths. What is provable is that leg (c) no longer
# runs at all in a repo with no per-repo wrappers: no runner ERROR, no false advisory, and rc = 0.
#
# rc = 0 is the load-bearing assert: emit_wrapper's failure path is a hard `exit 1`, so rc can only
# be 0 if check_project_level reached its own `return $rc`. The .gitignore docket block is
# pre-written so leg (a) passes and rc = 0 is meaningful rather than vacuously non-zero.
mkgitrepo
mkdir -p "$SBX/.config/docket"
# global layer: harness list WITHOUT claude, plus a bad (model-less) claude runner
printf 'agent_harnesses: [codex]\nagents:\n  claude:\n    status: { runner: codex }\n' \
  > "$SBX/.config/docket/config.yml"
# NO .docket.yml and NO .docket.local.yml => per_repo_opted_in is false.
# gitignore_block_wanted is still TRUE (the block below), which is exactly the weaker predicate.
( . "$REPO/scripts/lib/docket-gitignore-block.sh" && emit_docket_gitignore_block ) > "$SBX/.gitignore"
d1_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1 >/dev/null )"; d1_rc=$?
assert "0220/D1: --check completes rather than aborting inside leg (c) (rc=0)" \
  '[ "$d1_rc" = "0" ]'
assert "0220/D1: no raw runner ERROR escapes from leg (c)" \
  '! grep -qF "requires an explicit model" <<<"$d1_err"'
# The false advisory D1 removes as a side effect: project_level_pass writes nothing in a
# non-opted-in repo, so leg (c) reporting "not generated on this machine" was always wrong.
assert "0220/D1: no false leg-(c) advisory for un-generated per-repo wrappers" \
  '! grep -qF "not generated on this machine" <<<"$d1_err"'
# Non-vacuity: the fixture really did put the weaker predicate in the TRUE state, so the assert
# above is about the shared predicate and not about check_project_level having returned early.
assert "0220/D1: (fixture) gitignore_block_wanted was true — leg (a) ran and passed" \
  '! grep -qF "nothing else to check" <<<"$d1_err"'
rm -rf "$SBX"
```

Then **replace** the vacuous assert at ~line 1600 (finding 7). Delete these three lines:

```bash
assert "0207: --check wrote no wrappers" \
  '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" 2>/dev/null | wc -l | tr -d " ")" = "0" ]'
```

and put this in their place:

```bash
# (This assert was vacuous as written — leg (c) redirects into a mktemp -d, so --check never wrote
# into .claude/agents even pre-0207. The property it was reaching for is pinned by the 0220/D1
# fixture below, which proves --check reaches its own return rather than exiting mid-leg.)
assert "0207: --check exits before check_project_level runs its legs" \
  '! grep -qF "nothing else to check" <<<"$err" && ! grep -qF "advisory" <<<"$err"'
```

- [ ] **Step 2: Run the tests to verify the new fixture fails**

Run: `cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0207 && bash tests/test_sync_agents.sh 2>&1 | grep -E "^NOT OK|0220/D1"`

Expected: the three `0220/D1` asserts about rc, the runner ERROR, and the false advisory all print
`NOT OK`. The `(fixture)` non-vacuity assert prints `ok` (leg (a) does run today). Confirm the
failure mode is the real one by eyeballing the captured output: it must contain
`ERROR claude/docket-status: runner 'codex' requires an explicit model`.

- [ ] **Step 3: Add the shared predicate**

In `sync-agents.sh`, immediately below `per_repo_opted_in()` (which ends at ~line 230), add:

```bash
# Whether THIS run generates per-repo wrappers into <repo>/.<harness>/agents. The single predicate
# shared by the writer (project_level_pass), the gate (validate_runner_config's project-level leg)
# and --check's wrapper/dispatch-rule drift loops (check_project_level's leg (c)), so the gate can
# never see fewer triples than a call site later resolves — the under-enumeration change 0220 fixed.
# It is a delegation, not an alias: the three call sites must move together, and naming the concept
# is what makes that reviewable. gitignore_block_wanted is deliberately NOT this predicate — it is
# strictly weaker (a .docket.local.yml, a docket branch, or a pre-existing block all satisfy it),
# and legs (a)/(b) keep it because they are about the .gitignore block and tracked leftovers, which
# exist independently of whether any wrapper was generated.
project_wrappers_generated() { per_repo_opted_in; }
```

- [ ] **Step 4: Route the three call sites through it**

In `project_level_pass` (~line 1205), change:

```bash
  per_repo_opted_in || return 0
```

to:

```bash
  project_wrappers_generated || return 0
```

In `validate_runner_config`'s project-level leg (~line 648), change:

```bash
    per_repo_opted_in || continue
```

to:

```bash
    project_wrappers_generated || continue
```

In `check_project_level`, guard **both** drift loops. The wrapper-drift loop currently opens with
(~line 1291):

```bash
  # leg (c) — local staleness (ADVISORY: reported, never fails CI; vacuous on a fresh clone).
  local src name got tmp d harness
  tmp="$(mktemp -d)"
```

Insert the guard above the `mktemp -d`, and change the comment, so the block reads:

```bash
  # leg (c) — local staleness (ADVISORY: reported, never fails CI; vacuous on a fresh clone).
  # Gated on project_wrappers_generated, the SAME predicate project_level_pass writes under. Two
  # reasons, one predicate: (1) this loop calls emit_wrapper, so a triple the gate skipped would
  # die here on the can't-happen assertion (change 0220); (2) diffing against wrappers this repo
  # never generates produced a "not generated on this machine" advisory for every agent, which was
  # simply false. A repo that WAS opted in, generated wrappers, then dropped its key keeps those
  # wrappers and stops having them diffed — accepted, because prune_orphans' legs are themselves
  # per-repo-opt-in gated, so leg (c) was the last survivor of a boundary drawn everywhere else.
  if ! project_wrappers_generated; then
    return $rc
  fi
  local src name got tmp d harness
  tmp="$(mktemp -d)"
```

This single early return covers the dispatch-rule drift loop and `prune_orphans per-repo` as well —
both sit below it, both are per-repo-wrapper concerns, and `prune_orphans` already re-checks the
same opt-in internally. Verify by reading the code between the guard and the function's final
`return $rc` that nothing else there must still run for a non-opted-in repo.

- [ ] **Step 5: Correct the three falsified comments**

**(5a) — D5, `validate_runner_config`'s header (~line 607-610).** Replace:

```bash
# Gate 3 (change 0207): every `runner:` rule, checked across every candidate triple, BEFORE the
# first wrapper write. Wrapper generation is atomic — a run regenerates every wrapper or changes
# nothing on disk, so a configuration error leaves the previously generated wrappers in place on
# the assumption that what was already there was working (nginx -t / nginx -s reload).
```

with:

```bash
# Gate 3 (change 0207): every `runner:` rule, checked across every candidate triple, BEFORE the
# first wrapper write. Wrapper generation is atomic — a run regenerates every WRAPPER or changes no
# wrapper on disk, so a configuration error leaves the previously generated wrappers in place on
# the assumption that what was already there was working (nginx -t / nginx -s reload).
#
# "No wrapper" is the exact claim, not "nothing" (change 0220): migrate_legacy_global runs ABOVE
# this gate and has two disk effects a failing run does not undo — it renames the user's legacy
# ~/.config/docket/agents.yaml to .migrated, and it APPENDS an indented agents: block to the user's
# live global config.yml (adding a trailing newline first if that file lacked one). Nothing else on
# the failure path writes: the .gitignore write, migrate_tracked_wrappers and prune_orphans all sit
# below a passing gate.
```

**(5b) — `emit_wrapper`'s can't-happen block (~line 945-950).** Replace the sentence
`there are three today (user_level_pass, project_level_pass, check_project_level's leg (c)) and
nothing structurally prevents a fourth.` with:

```bash
  # there are three today (user_level_pass, project_level_pass, check_project_level's leg (c)) and
  # nothing structurally prevents a fourth. The three are gated consistently BECAUSE the gate's
  # project-level leg and leg (c) share project_wrappers_generated (change 0220) — not as a
  # coincidence of two predicates that happen to agree. A fourth call site must adopt that
  # predicate too, or re-open the gap 0220 closed.
```

**(5c) — the `--check` dispatch comment (~line 1431-1433).** Replace:

```bash
    # This leg reads PRE-migration config. That is the same asymmetry the two gates above already
    # have, and it matches what check_project_level's leg (c) drift loop itself resolves, so no gap
    # opens between the gate and that loop's own emit_wrapper calls.
```

with:

```bash
    # This leg reads PRE-migration config — the same asymmetry the two gates above already have.
    # It matches what check_project_level's leg (c) drift loop resolves, and since change 0220 the
    # two are gated by the SAME predicate (project_wrappers_generated), so the gate cannot see
    # fewer triples than that loop later emits. Before 0220 the gate used per_repo_opted_in while
    # leg (c) used the strictly weaker gitignore_block_wanted, and a global agent_harnesses: list
    # omitting claude let a bad claude runner: reach leg (c)'s emit_wrapper unchecked.
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0207 && bash tests/test_sync_agents.sh 2>&1 | tail -30`

Expected: no `NOT OK` lines. In particular all four `0220/D1` asserts and the rewritten
`0207: --check exits before check_project_level runs its legs` assert pass.

Also run the sibling suites, which exercise the same `--check` legs for other harnesses:

Run: `for t in tests/test_sync_agents_codex.sh tests/test_sync_agents_cursor.sh tests/test_sync_agents_opencode.sh; do echo "== $t"; bash "$t" 2>&1 | grep -c "^NOT OK"; done`

Expected: `0` for each. If a codex/cursor/opencode `--check` advisory assert reddens, it is a
fixture that was relying on leg (c) running without opt-in — re-read it against D1's accepted cost
before changing anything, and report it rather than loosening the guard.

- [ ] **Step 7: Mutation-test the new fixture**

Temporarily revert the shared predicate at the leg-(c) guard only (change
`if ! project_wrappers_generated; then` back to `if ! gitignore_block_wanted; then`, which is the
pre-0220 behavior for that block), re-run `bash tests/test_sync_agents.sh`, and confirm the three
substantive `0220/D1` asserts go `NOT OK`. Confirm the mutation actually landed by grepping the
file for the mutated line before running. Then restore the guard and re-run to green.

- [ ] **Step 8: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "fix(0220): share one predicate between the runner gate and --check's leg (c)"
```

---

### Task 2: Fixture the gate's user-level leg through the global config layer

Spec decision **D2** (finding 2). Every `runner:` fixture in every suite writes `.docket.yml` — the
project layer — so the whole `for harness in $USER_TARGETS` block in `validate_runner_config`,
including the `set -u` resolution added specifically for `--check`, is mutation-survivable: strip
it and nothing reddens. That is the leg protecting `~/.claude/agents`, the widest blast radius of
the original bug.

**Files:**
- Test: `tests/test_sync_agents.sh` — append after Task 1's D1 fixture.
- No `sync-agents.sh` change. This task adds coverage only.

**Interfaces:**
- Consumes: nothing from Task 1. The fixture is independent and can be written first if preferred.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_sync_agents.sh`, after Task 1's D1 fixture:

```bash
# ---- change 0220 / D2: the gate's USER-LEVEL leg, exercised through the GLOBAL layer -----------
# Every other runner: fixture writes .docket.yml (the project layer), so the whole
# `for harness in $USER_TARGETS` block in validate_runner_config was mutation-survivable: delete it
# and nothing reddened. This is the leg that protects ~/.claude/agents — the widest blast radius of
# the original change-0079/0205 bug. The repo here has NO .docket.yml and NO .docket.local.yml, so
# per_repo_opted_in is false and the project-level leg `continue`s: only the user-level leg can
# catch this, and rc != 0 is therefore attributable to it alone.
D2REPO="$(mktemp -d)"; D2ROOT="$(mktemp -d)"
mkdir -p "$D2ROOT/.claude" "$D2ROOT/.config/docket"
printf 'agents:\n  claude:\n    status: { runner: codex }\n' > "$D2ROOT/.config/docket/config.yml"
d2_err="$( cd "$D2REPO" && DOCKET_HARNESS_ROOT="$D2ROOT" bash "$SYNC" 2>&1 >/dev/null )"; d2_rc=$?
assert "0220/D2: a bad runner in the GLOBAL layer fails the real run nonzero" '[ "$d2_rc" != "0" ]'
assert "0220/D2: the user-level diagnostic names the agent" 'grep -qF "docket-status" <<<"$d2_err"'
assert "0220/D2: and names the required-model rule" \
  'grep -qF "requires an explicit model" <<<"$d2_err"'
# The protected behavior, stated as behavior: ~/.claude/agents is never generated from bad config.
assert "0220/D2: NO user-level wrapper was written for any agent" \
  '[ "$(find "$D2ROOT/.claude/agents" -name "docket-*.md" 2>/dev/null | wc -l | tr -d " ")" = "0" ]'
# --check must reach the same verdict (this is the path where compute_user_targets has not run and
# USER_TARGETS/USER_HARNESSES_SET are unset under set -u).
d2c_err="$( cd "$D2REPO" && DOCKET_HARNESS_ROOT="$D2ROOT" bash "$SYNC" --check 2>&1 >/dev/null )"; d2c_rc=$?
assert "0220/D2: --check fails on the same global-layer config" '[ "$d2c_rc" != "0" ]'
assert "0220/D2: --check says a real run would refuse to write wrappers" \
  'grep -qiE "would refuse to write wrappers" <<<"$d2c_err"'
# Non-vacuity companion: the SAME shape with a model present must generate the full set, so the
# asserts above cannot be satisfied by sync-agents.sh failing for an unrelated reason.
rm -rf "$D2ROOT/.claude"; mkdir -p "$D2ROOT/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex, model: some/model-id }\n' \
  > "$D2ROOT/.config/docket/config.yml"
( cd "$D2REPO" && DOCKET_HARNESS_ROOT="$D2ROOT" bash "$SYNC" >/dev/null 2>&1 ); d2v_rc=$?
assert "0220/D2: a VALID global runner config still generates (not vacuous)" '[ "$d2v_rc" = "0" ]'
assert "0220/D2: and the full built-in set lands user-level" \
  '[ "$(find "$D2ROOT/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
rm -rf "$D2REPO" "$D2ROOT"
```

- [ ] **Step 2: Run the tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0207 && bash tests/test_sync_agents.sh 2>&1 | grep -E "^NOT OK|0220/D2"`

Expected: every `0220/D2` line reads `ok`. This fixture asserts existing correct behavior, so it is
green on first run — which is exactly why Step 3 is mandatory and not optional.

- [ ] **Step 3: Mutation-test — the whole point of this task**

The assert only has value if deleting the leg reddens it. In `sync-agents.sh`, temporarily comment
out the user-level leg inside `validate_runner_config`:

```bash
    # for harness in $USER_TARGETS; do
    #   resolve_agent_layers "$harness" "$name" "$GLOBAL_CFG"
    #   if ! err="$(runner_config_error "$harness" "$name" "$RES_RUNNER" "$(user_flag_model)")"; then
    #     log "ERROR $err"; rc=1
    #   fi
    # done
```

Confirm the mutation landed: `grep -c '^    # for harness in \$USER_TARGETS' sync-agents.sh` must
print `1`. Re-run `bash tests/test_sync_agents.sh` and confirm the four failure-path `0220/D2`
asserts go `NOT OK`. Restore the block and re-run to green.

- [ ] **Step 4: Commit**

```bash
git add tests/test_sync_agents.sh
git commit -m "test(0220): cover the runner gate's user-level leg via the global config layer"
```

---

### Task 3: Make `emit_wrapper`'s `$2 == $RES_MODEL` contract explicit and enforced

Spec decision **D3** (finding 3). `user_flag_model`'s comment claims the provenance filter is
"spelled once", but `emit_wrapper` does not call it — it keeps its own copy, computed from
positional `$2` rather than `$RES_MODEL`. They agree today only because all three call sites happen
to pass `$RES_MODEL`, a convention nothing documents or enforces. The spec explicitly **rejects**
rerouting `flag_model` through `user_flag_model` (`$2` is also passed straight to `emit_shim` as the
frontmatter pin; sourcing the baked flag from `$RES_MODEL` while the pin stays on `$2` would let a
future call site emit a wrapper whose frontmatter and baked flag disagree, silently).

**Files:**
- Modify: `sync-agents.sh` — `emit_wrapper`'s header comment and an assertion at the top of its
  body (~line 917-935); `user_flag_model`'s comment (~line 877-881).
- Test: `tests/test_sync_agents.sh` — append after Task 2's D2 fixture.

**Interfaces:**
- Consumes: nothing from Tasks 1–2.
- Produces: `emit_wrapper` now aborts when `$2` differs from `$RES_MODEL`. The assertion sits
  **above** the `[ -z "$runner" ]` short-circuit, so it covers native emission too — the header
  states the contract for every call, and enforcing it only on the delegated path would leave the
  documented rule unenforced on the native one.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_sync_agents.sh`, after Task 2's D2 fixture:

```bash
# ---- change 0220 / D3: emit_wrapper's $2 == $RES_MODEL calling contract ------------------------
# emit_wrapper keeps its OWN copy of the provenance filter (RES_MODEL_FROM_USER over positional $2)
# rather than calling user_flag_model. The two agree only because all three call sites pass
# $RES_MODEL immediately after resolve_agent_layers. Nothing documented or enforced that, so a
# future call site passing a post-processed model would silently reintroduce the mid-loop abort
# 0207 exists to prevent. The contract is now on the header AND asserted; this fixture pins the
# assertion by calling emit_wrapper directly with a mismatched $2.
d3_out="$(
  set +e
  # shellcheck source=/dev/null
  DOCKET_HARNESS_ROOT="$(mktemp -d)" bash -c '
    set -uo pipefail
    . "$1" 2>/dev/null || true
    RES_MODEL="the-resolved-one"; RES_EFFORT=""; RES_MODEL_FROM_USER=1; RES_EFFORT_FROM_USER=0
    emit_wrapper "$2" "a-DIFFERENT-model" "" "" "claude" "status"
  ' _ "$REPO/sync-agents.sh" "$REPO/agents/docket-status.md" 2>&1
  printf 'RC=%s' "$?"
)"
assert "0220/D3: emit_wrapper aborts when \$2 is not the resolved RES_MODEL" \
  '! grep -qF "RC=0" <<<"$d3_out"'
assert "0220/D3: and the abort names the contract" \
  'grep -qiE "RES_MODEL" <<<"$d3_out"'
# Non-vacuity: the SAME call with $2 == $RES_MODEL must succeed, so the assert above is about the
# mismatch and not about emit_wrapper failing to run at all in this harness.
d3_ok="$(
  set +e
  DOCKET_HARNESS_ROOT="$(mktemp -d)" bash -c '
    set -uo pipefail
    . "$1" 2>/dev/null || true
    RES_MODEL="the-resolved-one"; RES_EFFORT=""; RES_MODEL_FROM_USER=1; RES_EFFORT_FROM_USER=0
    emit_wrapper "$2" "the-resolved-one" "" "" "claude" "status"
  ' _ "$REPO/sync-agents.sh" "$REPO/agents/docket-status.md" >/dev/null 2>&1
  printf 'RC=%s' "$?"
)"
assert "0220/D3: (non-vacuity) a matching \$2 still emits successfully" '[ "$d3_ok" = "RC=0" ]'
# And the contract is stated where a caller reads it, not only enforced.
assert "0220/D3: emit_wrapper's header states the \$2 contract" \
  'within "$REPO/sync-agents.sh" "emit_wrapper(){" "RES_MODEL" 900'
```

> **Note for the implementer:** `sync-agents.sh` guards its main block with
> `if [ "${BASH_SOURCE[0]}" = "${0}" ]`, so sourcing it defines the functions without running the
> generator. Verify that the non-vacuity half of this fixture (`d3_ok` → `RC=0`) works **before**
> writing the rest — if sourcing aborts for an unrelated reason, both halves of the fixture would
> report `RC != 0` and the mismatch assert would pass while proving nothing. If the source path
> cannot be made to work, report the blocker and stop rather than routing around it; a fixture that
> cannot demonstrate its own success case is a vacuous assert.

- [ ] **Step 2: Run the tests to verify the new asserts fail**

Run: `cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0207 && bash tests/test_sync_agents.sh 2>&1 | grep "0220/D3"`

Expected: the mismatch assert and the header assert read `NOT OK`; the non-vacuity assert reads
`ok`. If the non-vacuity assert also fails, the sourcing harness is wrong — fix that before
touching `sync-agents.sh`, or the whole fixture is vacuous.

- [ ] **Step 3: State the contract on `emit_wrapper`'s header**

In `sync-agents.sh`, the comment block above `emit_wrapper` (~line 913-918) currently ends:

```bash
# with no USER-configured model — live in runner_config_error and are gated up front by
# validate_runner_config; the call below is only a can't-happen assertion.
```

Append to that block, immediately before the `emit_wrapper(){` line:

```bash
#
# CALLING CONTRACT (change 0220): $2 MUST be the RES_MODEL that resolve_agent_layers just resolved
# for this exact (harness, agent) pair, and $3 the matching RES_EFFORT. $2 is used TWICE and for
# two different things — as emit_shim's frontmatter pin, and (provenance-filtered through
# RES_MODEL_FROM_USER) as the baked --model flag — so a caller that passes a post-processed model
# would split the wrapper against itself. This is why the provenance filter here is a second
# spelling of user_flag_model's rather than a call to it: rerouting only the flag would leave the
# frontmatter pin on $2 and emit a wrapper whose two halves disagree, silently. The assertion at the
# top of the body is what makes the contract enforced rather than merely conventional.
```

- [ ] **Step 4: Add the enforcing assertion**

`emit_wrapper`'s body currently opens:

```bash
emit_wrapper(){  # $1=src $2=model $3=effort $4=runner $5=harness $6=agent-name  (stdout)
  local runner="$4"
  if [ -z "$runner" ]; then emit_for_harness "$1" "$5" "$2" "$3"; return 0; fi
```

Insert the assertion above the short-circuit — it must cover the native path too:

```bash
emit_wrapper(){  # $1=src $2=model $3=effort $4=runner $5=harness $6=agent-name  (stdout)
  # Enforce the calling contract stated in the header above. ABOVE the `[ -z "$runner" ]`
  # short-circuit deliberately: the header states the contract for EVERY call, so enforcing it only
  # on the delegated path would leave the documented rule unenforced on the native one.
  if [ "$2" != "${RES_MODEL:-}" ]; then
    log "ERROR emit_wrapper called for $5/docket-$6 with model '$2', which is not the resolved RES_MODEL '${RES_MODEL:-}' — see emit_wrapper's calling contract. No wrappers were written."
    exit 1
  fi
  local runner="$4"
  if [ -z "$runner" ]; then emit_for_harness "$1" "$5" "$2" "$3"; return 0; fi
```

- [ ] **Step 5: Correct `user_flag_model`'s comment**

The comment above `user_flag_model` (~line 877-881) ends:

```bash
# agents/harness-defaults.yml default must read as absent here. Spelled once: emit_wrapper and
# validate_runner_config must agree exactly, or the gate passes a triple the assertion then kills.
```

Replace those two lines with:

```bash
# agents/harness-defaults.yml default must read as absent here. NOT spelled once (change 0220):
# emit_wrapper deliberately keeps its own copy over positional $2, because $2 is also the
# frontmatter pin and rerouting only the flag would split the two. What keeps them from drifting is
# emit_wrapper's $2 == $RES_MODEL assertion, not a shared call — the two spellings must still agree
# exactly, or the gate passes a triple the assertion then kills.
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0207 && bash tests/test_sync_agents.sh 2>&1 | grep -E "^NOT OK|0220/D3"`

Expected: all four `0220/D3` asserts read `ok`, and no other assert regressed. A regression here
most likely means a real call site passes something other than `$RES_MODEL` — if so, that is a live
bug the assertion just found: report it rather than relaxing the assertion.

- [ ] **Step 7: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "fix(0220): document and enforce emit_wrapper's \$2 == \$RES_MODEL contract"
```

---

### Task 4: Make the ordering assert able to tell the two rules apart

Spec decision **D4** (finding 4). The `unregistered offender reports the registration rule` assert
matches only the runner name, which appears in **both** diagnostics — so swapping the two `if`
blocks in `runner_config_error` leaves it green. The line extraction is load-bearing: that fixture
also configures a second, legitimately model-less agent, so the required-model wording **is**
present in the accumulated `$err` and a whole-output negative assert would fail against a correct
implementation.

**Files:**
- Test: `tests/test_sync_agents.sh` — replace the assert at ~line 1587-1589 (fixture (3),
  "MULTIPLE offenders across different agents").
- No `sync-agents.sh` change.

**Interfaces:**
- Consumes: nothing from Tasks 1–3.

- [ ] **Step 1: Write the failing test**

In fixture (3) of the change-0207 block, replace:

```bash
# Accumulating, not short-circuiting: the unregistered one must report its OWN rule, not be
# swallowed by whichever offender the walk happened to reach first.
assert "0207: the unregistered offender reports the registration rule" \
  'grep -qF "gemini-cli" <<<"$err"'
```

with:

```bash
# Accumulating, not short-circuiting: the unregistered one must report its OWN rule, not be
# swallowed by whichever offender the walk happened to reach first.
#
# Extract the OFFENDING AGENT'S OWN LINE first (change 0220). Matching the runner name against the
# whole output could not tell the two rules apart — the name appears in both diagnostics, so
# swapping the if-blocks in runner_config_error left this green. A whole-output negative assert is
# not available either: docket-status in this same fixture is legitimately model-less, so the
# required-model wording IS in $err against a correct implementation.
adr_line="$(grep -F "docket-adr" <<<"$err" | head -n1)"
assert "0220/D4: (fixture) the unregistered offender produced a diagnostic line" '[ -n "$adr_line" ]'
assert "0220/D4: the unregistered offender's OWN line reports the REGISTRATION rule" \
  'grep -qF "is not a registered runner" <<<"$adr_line"'
assert "0220/D4: and that same line does NOT report the required-model rule" \
  '! grep -qF "requires an explicit model" <<<"$adr_line"'
# The companion direction: the registered-but-model-less agent reports the OTHER rule on its line.
status_line="$(grep -F "docket-status" <<<"$err" | head -n1)"
assert "0220/D4: the model-less offender's own line reports the required-model rule" \
  '[ -n "$status_line" ] && grep -qF "requires an explicit model" <<<"$status_line"'
```

- [ ] **Step 2: Run the tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0207 && bash tests/test_sync_agents.sh 2>&1 | grep -E "^NOT OK|0220/D4"`

Expected: all five `0220/D4` asserts read `ok` against the current (correct) implementation.

- [ ] **Step 3: Mutation-test — prove it now discriminates**

In `sync-agents.sh`'s `runner_config_error`, swap the two `if` blocks so the required-model check
runs before the registration check. Confirm the mutation landed by checking that the
`[ -z "$flag_model" ]` block now appears above the `! is_registered_runner "$runner"` block:

Run: `grep -n 'flag_model\" \]\|is_registered_runner \"\$runner\"' sync-agents.sh | head -4`

Re-run `bash tests/test_sync_agents.sh`. Expected: `0220/D4: the unregistered offender's OWN line
reports the REGISTRATION rule` and `...does NOT report the required-model rule` both go `NOT OK` —
the discrimination the old assert lacked. Restore the original order and re-run to green.

> Ordering itself stays pinned by the existing ORDERING FENCE fixture above; this assert is only
> being made able to tell the two rules apart. Do not delete or rewrite the ORDERING FENCE.

- [ ] **Step 4: Commit**

```bash
git add tests/test_sync_agents.sh
git commit -m "test(0220): make the runner rule-ordering assert discriminate between the two rules"
```

---

### Task 5: Report each distinct diagnostic once

Spec decision **D6** (finding 6). A bad `runner:` in the **global** layer is visible to both the
user-level leg (which resolves over `$GLOBAL_CFG`) and the project-level leg (which resolves over
local ⊕ committed ⊕ global), so it is reported twice, verbatim identically — against a README
promising every offender "in one pass". Dedupe on the **exact rendered diagnostic**, not on the
`(harness, agent)` triple: two genuinely different offenders stay separately reported even when
they share a harness and agent, and accumulation (never short-circuit) stays intact.

**Files:**
- Modify: `sync-agents.sh` — a `report_runner_error_once` helper above `validate_runner_config`
  (~line 620), and both `log "ERROR $err"; rc=1` sites inside it (~line 642 and ~line 652).
- Test: `tests/test_sync_agents.sh` — append after Task 3's D3 fixture.

**Interfaces:**
- Consumes: `project_wrappers_generated` from Task 1 is already in place at the project-level leg;
  this task does not change that guard.
- Produces: `report_runner_error_once <diagnostic>` — logs at most once per exact string,
  reading and appending to the caller's `seen` variable (bash dynamic scoping).

- [ ] **Step 1: Write the failing test**

Append to `tests/test_sync_agents.sh`, after Task 3's D3 fixture:

```bash
# ---- change 0220 / D6: each distinct diagnostic is reported exactly once ------------------------
# A bad runner: in the GLOBAL layer is visible to BOTH gate legs — the user-level leg resolves over
# $GLOBAL_CFG, the project-level leg over local ⊕ committed ⊕ global — so it was logged twice,
# verbatim identically, against a README that promises every offender "in one pass".
#
# This fixture is ALSO the over-dedupe guard, which is the failure mode the dedup code introduces.
# The two legs here yield DISTINCT diagnostics: .docket.yml sets an unregistered runner on status
# (project leg only — the global layer has no status entry), while the global config sets a
# registered-but-model-less runner on adr (visible to BOTH legs, hence the duplicate). Deduping on
# the rendered string must collapse adr's two identical copies while leaving status's different
# diagnostic untouched.
mkgitrepo
mkdir -p "$SBX/.claude" "$SBX/.config/docket"
printf 'agents:\n  claude:\n    status: { runner: gemini-cli }\n' > "$SBX/.docket.yml"
printf 'agents:\n  claude:\n    adr: { runner: codex }\n' > "$SBX/.config/docket/config.yml"
d6_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; d6_rc=$?
assert "0220/D6: (fixture) the run still fails nonzero" '[ "$d6_rc" != "0" ]'
# The dedup itself: the doubly-visible global offender is logged ONCE, not once per leg.
assert "0220/D6: a diagnostic visible to both legs is reported exactly once" \
  '[ "$(grep -cF "docket-adr" <<<"$d6_err")" = "1" ]'
# The OVER-dedupe guard: a genuinely different offender must survive the filter.
assert "0220/D6: a distinct offender is NOT suppressed by the dedup" \
  '[ "$(grep -cF "docket-status" <<<"$d6_err")" -ge 1 ]'
assert "0220/D6: and it keeps its own rule" \
  'grep -F "docket-status" <<<"$d6_err" | grep -qF "is not a registered runner"'
assert "0220/D6: while the deduped one keeps its own, different rule" \
  'grep -F "docket-adr" <<<"$d6_err" | grep -qF "requires an explicit model"'
rm -rf "$SBX"
```

- [ ] **Step 2: Run the tests to verify the dedup assert fails**

Run: `cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0207 && bash tests/test_sync_agents.sh 2>&1 | grep "0220/D6"`

Expected: `0220/D6: a diagnostic visible to both legs is reported exactly once` reads `NOT OK`
(today the count is 2). Every other `0220/D6` assert reads `ok` — they describe behavior that must
survive the change, and their job is to redden if the dedup over-suppresses.

- [ ] **Step 3: Add the dedup helper**

In `sync-agents.sh`, immediately above the `# Gate 3 (change 0207): ...` comment block that heads
`validate_runner_config` (~line 607), add:

```bash
# Log an accumulated gate diagnostic AT MOST ONCE per exact string (change 0220). Reads and appends
# to the caller's `seen` (bash dynamic scoping; bash-3.2-safe — no associative arrays).
#
# Keyed on the RENDERED DIAGNOSTIC, not on the (harness, agent) triple: a bad runner: in the GLOBAL
# layer is visible to both gate legs and produces two byte-identical lines, while two genuinely
# different offenders that happen to share a harness and agent produce two different lines and must
# both survive. Deduping by layer provenance was rejected — the gate's loops deliberately do not do
# provenance, and it would suppress a project diagnostic that merely happens to read identically.
# Suppressing a repeat never changes the caller's rc: the caller sets rc=1 unconditionally.
report_runner_error_once(){  # $1=diagnostic ; requires a caller-scoped `seen`
  if grep -F -x -q -- "$1" <<<"$seen"; then return 0; fi
  log "ERROR $1"
  seen="$seen$1"$'\n'
  return 0
}
```

> `set -e` is active: the `if ... ; then ... fi` form is mandatory. A `grep ... && return 0` list
> whose grep fails would abort the whole run.

- [ ] **Step 4: Route both log sites through it**

In `validate_runner_config`, extend the `local` declaration to seed the accumulator:

```bash
validate_runner_config() {
  local rc=0 src name harness err seen=""
```

Then change the user-level leg's report from:

```bash
      if ! err="$(runner_config_error "$harness" "$name" "$RES_RUNNER" "$(user_flag_model)")"; then
        log "ERROR $err"; rc=1
      fi
```

to:

```bash
      if ! err="$(runner_config_error "$harness" "$name" "$RES_RUNNER" "$(user_flag_model)")"; then
        report_runner_error_once "$err"; rc=1
      fi
```

and make the identical edit at the project-level leg's copy a few lines below. `rc=1` stays
**outside** the helper and unconditional — a suppressed duplicate is still a failure.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0207 && bash tests/test_sync_agents.sh 2>&1 | grep -E "^NOT OK|0220/D6"`

Expected: all five `0220/D6` asserts read `ok`, and no earlier assert regressed. Pay particular
attention to fixture (3)'s multi-offender asserts and Task 4's new per-line asserts — they are the
ones an over-eager dedup would break.

- [ ] **Step 6: Verify the accumulate-don't-short-circuit property survived**

Run: `cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0207 && bash tests/test_sync_agents.sh 2>&1 | grep -E "0207: (multiple offenders|the first offender|the SECOND offender)"`

Expected: all three read `ok`. If the second offender vanished, the dedup is keyed too broadly —
re-read the helper's comment rather than loosening the fixture.

- [ ] **Step 7: Run the full suite**

Run: `cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0207 && bash tests/test_sync_agents.sh 2>&1 | grep -c "^NOT OK"`

Expected: `0`.

- [ ] **Step 8: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "fix(0220): report each distinct runner diagnostic once per run"
```

---

## Verification

After the last task, the full repo suite must be green — not only `test_sync_agents.sh`. The
`--check` legs this change touches are exercised by the codex, cursor and opencode sibling suites,
and the comment edits are inside blocks that `tests/test_comment_anchor_style.sh` and
`tests/test_script_contracts_coverage.sh` may read.

Run the whole suite in ONE foreground call (it takes roughly ten minutes; do not background it):

```bash
cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0207 && \
  for t in tests/test_*.sh; do echo "== $t"; bash "$t" 2>&1 | grep -E "^NOT OK" ; done; echo DONE
```

Expected: no `NOT OK` lines between the `==` headers, and `DONE` at the end.

Then re-check the new greps under BSD grep, since the developer's PATH `grep` is ugrep and masks
portability bugs (repo learning `shell-portability`):

```bash
cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0207 && \
  PATH=/usr/bin:/bin bash tests/test_sync_agents.sh 2>&1 | grep -cE "^NOT OK"
```

Expected: `0`.

## Notes for the reviewer

Two places where this plan deliberately departs from the letter of the spec, both recorded so the
reviewer does not have to re-derive them:

1. **D1's third assertion clause.** The spec asks the regression fixture to show "the legs *after*
   leg (c) still emitting". In a non-opted-in repo there is no reachable post-leg-(c) emitter — the
   dispatch-rule loop needs a dispatch-rule harness in `$HARNESSES` (which requires the repo's own
   `agent_harnesses:`, i.e. opt-in) and `prune_orphans per-repo` is itself opt-in gated. The
   property is instead pinned by `rc = 0`, which is strictly stronger for this purpose:
   `emit_wrapper`'s failure path is a hard `exit 1`, so `rc = 0` is only reachable through
   `check_project_level`'s own `return $rc`.
2. **D5 is folded into Task 1** rather than standing alone. The comment it corrects *is*
   `validate_runner_config`'s header — the same block Task 1 edits — so splitting them would make
   two tasks touch one comment.
