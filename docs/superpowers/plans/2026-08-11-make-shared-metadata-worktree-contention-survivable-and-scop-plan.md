<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0247 — Make shared metadata worktree contention survivable and scope its commits](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-11-0247-make-shared-metadata-worktree-contention-survivable-and-scop.md)**
<!-- docket:backlink:end -->

# Make shared metadata worktree contention survivable and scope its commits — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a collision in the shared `.docket` metadata worktree survivable (a bounded, discriminating preflight retry) and bound its blast radius (every commit into that tree — script-issued *and* agent-issued — stages only what it wrote).

**Architecture:** Three halves of one invariant, built in dependency order. Half 1 replaces `docket-preflight.sh`'s bare `fetch && pull --rebase || return 1` with a fetch-first fast path plus a classified, bounded retry. Half 2 adds `--` pathspecs to the two exposed `docket-status.sh` commits and introduces a `blocked-wedged-tree` report token for a tree that is mid-rebase. Half 3 states the pathspec rule in `docket-convention`'s direct-git grant and carries the marker `Stage by explicit path` into all seven metadata-writing skill bodies. One new guard file, `tests/test_shared_worktree_commit_scope.sh`, covers both channels — one invariant, one guard, one exception list.

**Tech Stack:** Bash 4+ (`DOCKET_BASH_PATH`), POSIX awk, git plumbing, docket's own `tests/*.sh` assert-harness convention (`assert "<name>" '<eval-string>'`), `tests/run-tests.sh` as the suite gate.

**Spec:** `.docket/docs/superpowers/specs/2026-08-09-make-shared-metadata-worktree-contention-survivable-and-scop-design.md`, reconciled 2026-08-11. Its *Requirements checklist* is the normative summary; its `## Assumptions` 1–16 are the decision record. **Read Assumption 16 before Task 2 and Task 5** — it carries this build's four hard constraints.

## Global Constraints

- **Never `--autostash`** in any metadata-tree sync path. Half 1 asserts this with a repo grep.
- **Untracked-only files never count as dirty.** Every dirty probe uses `--untracked-files=no`.
- **Every new env var is `DOCKET_`-namespaced** (ADR-0014); follow change 0284's `DOCKET_RUNNER_DISPATCH_TEST_BARRIER` precedent.
- **Cross-references anchor on a symbol name or a verbatim-quoted clause, never a line number** (ADR-0054). This binds the guard's exception keys too: key them `<basename>:<-C target var>`.
- **No producer piped into an early-exiting consumer** under `set -o pipefail`. Capture into a variable, then `grep <<<"$var"`.
- **`grep` for a pattern leading with `--`** must use `grep -qF --` or `grep -E -e`.
- **A new `tests/test_*.sh` with no row in `tests/runtime-budgets.tsv` fails `tests/test_runtime_budgets.sh`.** Task 3 adds the row.
- **Do NOT run `scripts/run-tests.sh --timings <test path>` against a real test file** — it truncates the named file to zero bytes (#0290, unfixed). Measure with `time bash tests/<file>` instead.
- **Suite command:** whatever `finalize.test_command` resolves to — `scripts/run-tests.sh`. Run the whole suite at the gate, never only the files a task names. A trailing `OVER BUDGET:` line is a finding to act on.
- **Known flake, not yours:** `tests/test_gate_run_stop.sh` at "the stop is held where the completed marker would be written" is #0293 and reproduces on clean `origin/main`. Re-run and proceed. `tests/test_sync_agents_runners.sh` at ~190s is #0280, pre-existing.

### Plan-supplied code is a draft under test

Every code block below was written against an implementation that does not exist yet and **has never been executed**. This is the dominant failure mode of the current drain (learnings: `plan-supplied-test-code-is-unverified`). For every assert you take from this plan:

1. **Prove it can pass at all** before debugging the implementation against it. Check field indices, ranges and expected values against the real output format.
2. **Mutation-test its key** — delete the thing it exists to check and watch it redden. A mutation that leaves an assert green is a defect until proven otherwise.
3. **Confirm the mutation actually changed bytes.** `grep -c` before and after, or compare `wc -l`. Against hard-wrapped prose a single-line `perl -pi` pattern silently no-ops, and a mutation that never applied is indistinguishable from a guard that failed to catch.
4. **Check exact removed/added line counts, never a token count**, when a mutation is supposed to delete something.
5. **Deletion and inversion are different probes.** A comparison operator needs both.

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `scripts/lib/docket-preflight.sh` | Modify — retry budget constants, the injectable sleep seam, the dirty/wedged predicates, and `_docket_sync_metadata` replacing the inline sync in both mode branches | 1 |
| `tests/test_docket_preflight.sh` | Modify — five fixture sections (H)–(L) for Half 1, plus the `--autostash` repo grep | 1 |
| `scripts/docket-status.sh` | Modify — pathspecs on the two exposed commits, the wedged probe, the `blocked-wedged-tree` token, and the explicit `case` arms at both consumers | 2 |
| `scripts/docket-status.md` | Modify — report-line vocabulary and exit-code mapping for the new token | 2 |
| `tests/test_docket_status.sh` | Modify — token behaviour and pathspec scoping fixtures | 2 |
| `tests/test_shared_worktree_commit_scope.sh` | **Create** — Group A (script channel, Task 3); Groups B1/B2 (agent channel, Task 5) | 3, 5 |
| `tests/runtime-budgets.tsv` | Modify — a row for the new file; re-measure any row this change pushes | 3 |
| `skills/docket-convention/SKILL.md` | Modify — the pathspec rule at the direct-git grant sentence | 4 |
| `skills/docket-{implement-next,groom-next,auto-groom,status,new-change,finalize-change,adr}/SKILL.md` | Modify — the marker at each commit instruction | 4 |
| `tests/test_skill_size_budgets.sh` | Modify — re-measured budget raises with the change-0201 in-diff argument | 4 |

**Build order rationale.** Task 1 is independent. Task 2 consumes Task 1's `_docket_tree_wedged`. Task 3's guard must run *after* Task 2 fixes the two sites, or it lands red. Task 4 writes the prose Task 5's guard checks; splitting them keeps a reviewer able to reject the prose without rejecting the guard.

**File-collision note.** Changes #0118 and #0268 are queued immediately after this one and both edit `scripts/docket-status.sh`. Keep every edit here confined to the two commit sites, the two result `case` statements, and the report vocabulary. Do not opportunistically fix anything else in that file (learnings: `concurrent-edits-compose-at-rebase` — keep each edit additive and funnel through the shared chokepoint).

---

### Task 1: Half 1 — the bounded, discriminating preflight sync

**Files:**
- Modify: `scripts/lib/docket-preflight.sh` (the sync lines inside `docket_preflight`, and new helpers above it)
- Test: `tests/test_docket_preflight.sh` (append sections H–L before the final exit)

**Interfaces:**
- Produces: `_docket_tree_wedged <git> <dir>` — returns 0 when a rebase or merge is in progress in `<dir>`. **Task 2 consumes this by name.**
- Produces: `_docket_tree_dirty <git> <dir>` — returns 0 when TRACKED files are modified.
- Produces: `_docket_sync_metadata <git> <dir> <remote> <branch>` — returns 0 on success, 1 on exhaustion or a terminal failure, diagnostic on stderr.
- Produces: env seam `DOCKET_PREFLIGHT_TEST_SLEEP_CMD` — when set, its value is `eval`ed in place of the real `sleep`.

**Invariant to review against (spec, Half 1):** the sync may report success when local metadata is already current with, or ahead of, the fetched remote; it integrates remote changes only when the tracked tree is clean and no git operation is in progress; it never mutates another agent's in-flight state to get there.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_docket_preflight.sh`, immediately before its final `exit` line. These reuse `$bare`, `$work`, `$tmp`, `$LIB`, `$SCRIPTS`, `mkexport` and `assert` already defined at the top of that file.

```bash
# --- (H) change 0247: a dirty tree with NOTHING to pull must never fail the sync ----------------
# The most common collision by far: the other agent is mid-edit and has not pushed. There is
# nothing to integrate, so no rebase is needed, so a dirty tree is irrelevant. Pre-0247 this was a
# hard failure — `pull --rebase` refuses on unstaged changes regardless of whether it has work.
echo dirty >> "$work/.docket/README.md" 2>/dev/null || : > "$work/.docket/dirtyfile"
git -C "$work/.docket" add -A >/dev/null 2>&1
git -C "$work/.docket" commit -q -m seed >/dev/null 2>&1
git -C "$work/.docket" push -q origin HEAD:docket >/dev/null 2>&1
printf 'tracked\n' > "$work/.docket/tracked.txt"
git -C "$work/.docket" add tracked.txt >/dev/null 2>&1
git -C "$work/.docket" commit -q -m tracked >/dev/null 2>&1
git -C "$work/.docket" push -q origin HEAD:docket >/dev/null 2>&1
printf 'modified\n' >> "$work/.docket/tracked.txt"          # tracked + dirty, remote NOT moved
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD=':' docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/h.err"; rc=$?
assert "H: dirty tracked tree with no remote movement syncs successfully" '[ "$rc" -eq 0 ]'
assert "H: the dirty edit was NOT stashed, reset, or committed away" \
  'grep -q "^modified$" "$work/.docket/tracked.txt"'

# --- (I) untracked-only files never count as dirty ---------------------------------------------
git -C "$work/.docket" checkout -- tracked.txt
: > "$work/.docket/stray-untracked.txt"
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD=':' docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/i.err"; rc=$?
assert "I: an untracked-only file never fails the sync" '[ "$rc" -eq 0 ]'
assert "I: the untracked file survives the sync" '[ -f "$work/.docket/stray-untracked.txt" ]'
rm -f "$work/.docket/stray-untracked.txt"

# --- (J) dirty tree + remote MOVED: retries, then succeeds once the other agent finishes --------
# The sleep seam is the ONLY point in the loop where a second actor could have acted, so the
# fixture models "the other agent committed and the tree went clean" from inside it. This also
# proves the retry loop actually re-evaluates state between attempts rather than re-running a
# decision it made once.
git -C "$work" push -q origin HEAD:main >/dev/null 2>&1 || :
other="$tmp/other"; git clone --quiet "$bare" "$other" 2>/dev/null
git -C "$other" config user.email t@t.test; git -C "$other" config user.name Test
git -C "$other" checkout --quiet -B docket origin/docket
printf 'remote moved\n' > "$other/remote-moved.txt"
git -C "$other" add remote-moved.txt >/dev/null 2>&1
git -C "$other" commit -q -m "other agent" >/dev/null 2>&1
git -C "$other" push -q origin HEAD:docket >/dev/null 2>&1
printf 'mid-edit\n' >> "$work/.docket/tracked.txt"          # our tree is dirty AND remote moved
cat > "$tmp/heal.sh" <<HEAL
#!/usr/bin/env bash
n=\$(cat "$tmp/heal.count" 2>/dev/null || echo 0); n=\$((n+1)); echo "\$n" > "$tmp/heal.count"
# On the 2nd backoff, the "other agent" finishes: our tree goes clean.
[ "\$n" -ge 2 ] && git -C "$work/.docket" checkout -- tracked.txt
exit 0
HEAL
chmod +x "$tmp/heal.sh"; rm -f "$tmp/heal.count"
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/heal.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/j.err"; rc=$?
assert "J: a dirty tree with remote movement retries and then succeeds" '[ "$rc" -eq 0 ]'
assert "J: it actually spent at least two backoffs before succeeding" \
  '[ "$(cat "$tmp/heal.count" 2>/dev/null || echo 0)" -ge 2 ]'
assert "J: the remote commit was integrated" '[ -f "$work/.docket/remote-moved.txt" ]'

# --- (K) exhaustion: non-zero, and the diagnostic NAMES the last failure class ------------------
# The point of a discriminating retry is that the caller learns WHAT blocked it, not merely that
# five attempts died. Keeping the tree dirty for every attempt forces the `dirty` class.
printf 'still mid-edit\n' >> "$work/.docket/tracked.txt"
git -C "$other" pull -q --rebase origin docket >/dev/null 2>&1
printf 'remote moved again\n' > "$other/remote-moved-2.txt"
git -C "$other" add remote-moved-2.txt >/dev/null 2>&1
git -C "$other" commit -q -m "other agent 2" >/dev/null 2>&1
git -C "$other" push -q origin HEAD:docket >/dev/null 2>&1
rm -f "$tmp/exh.count"
cat > "$tmp/count.sh" <<CNT
#!/usr/bin/env bash
n=\$(cat "$tmp/exh.count" 2>/dev/null || echo 0); echo "\$((n+1))" > "$tmp/exh.count"; exit 0
CNT
chmod +x "$tmp/count.sh"
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/count.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/k.err"; rc=$?
assert "K: exhaustion returns non-zero (fail-closed, as before)" '[ "$rc" -ne 0 ]'
assert "K: the exhaustion diagnostic names the dirty-tracked-tree class" \
  'grep -qi "dirty" "$tmp/k.err"'
assert "K: the exhaustion diagnostic distinguishes it from a wedged tree" \
  '! grep -qi "rebase or merge is in progress" "$tmp/k.err"'
assert "K: the budget is bounded — exactly 4 backoffs across 5 attempts" \
  '[ "$(cat "$tmp/exh.count" 2>/dev/null || echo 0)" -eq 4 ]'
git -C "$work/.docket" checkout -- tracked.txt

# --- (L) a conflicting local commit aborts IMMEDIATELY without burning the retry budget ---------
# A content conflict raised by THIS attempt's own rebase is deterministic: it fails identically on
# every retry, so spending budget on it is pure latency. The tree must be restored to its
# pre-attempt state (rebase --abort), never left mid-rebase for the next agent to find.
git -C "$work/.docket" pull -q --rebase origin docket >/dev/null 2>&1
printf 'ours\n' > "$work/.docket/conflict.txt"
git -C "$work/.docket" add conflict.txt >/dev/null 2>&1
git -C "$work/.docket" commit -q -m ours >/dev/null 2>&1
git -C "$other" pull -q --rebase origin docket >/dev/null 2>&1
printf 'theirs\n' > "$other/conflict.txt"
git -C "$other" add conflict.txt >/dev/null 2>&1
git -C "$other" commit -q -m theirs >/dev/null 2>&1
git -C "$other" push -q origin HEAD:docket >/dev/null 2>&1
rm -f "$tmp/conf.count"
cat > "$tmp/count2.sh" <<CNT2
#!/usr/bin/env bash
n=\$(cat "$tmp/conf.count" 2>/dev/null || echo 0); echo "\$((n+1))" > "$tmp/conf.count"; exit 0
CNT2
chmod +x "$tmp/count2.sh"
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/count2.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/l.err"; rc=$?
assert "L: a content conflict fails immediately (non-zero)" '[ "$rc" -ne 0 ]'
assert "L: it spent NO retry budget (zero backoffs)" \
  '[ "$(cat "$tmp/conf.count" 2>/dev/null || echo 0)" -eq 0 ]'
assert "L: the diagnostic names the conflict class" 'grep -qi "conflict" "$tmp/l.err"'
assert "L: the tree was restored — no rebase left in progress" \
  '[ ! -d "$work/.docket/.git/rebase-merge" ] && [ ! -d "$work/.docket/.git/rebase-apply" ] && ! git -C "$work/.docket" status --porcelain 2>/dev/null | grep -q "^UU"'

# --- (M) the never-autostash rule, asserted by repo grep ----------------------------------------
# A shared tree makes --autostash a data-loss bug: it stashes ANOTHER agent's in-flight edits.
# Shape-keyed over the whole sync library, not an enumerated list of call sites.
assert "M: no --autostash anywhere in the metadata sync library" \
  '! grep -qF -- "--autostash" "$LIB"'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_preflight.sh`
Expected: sections H–L report `NOT OK`. H and J fail because today's bare `pull --rebase` refuses on a dirty tree; K and L fail because no diagnostic or budget exists. M passes already (that is fine — it is a regression guard, and its own mutation proof is Step 6).

- [ ] **Step 3: Add the constants, the seam, and the predicates**

In `scripts/lib/docket-preflight.sh`, insert after the `. "$(cd ... )/docket-root.sh"` source line:

```bash
# --- metadata sync: bounded, discriminating retry (change 0247) ----------------------------------
# 5 attempts with 2/4/8/8s backoff (~22s total). The RATIONALE, not just the number: the collision
# this retries is "another agent is between its edit and its push" in the SHARED .docket worktree.
# Most such windows close in seconds once that agent commits, and an autonomous caller re-running
# preflight later covers the long tail — so a longer budget buys little while blocking every
# skill's Step 0 for it. Calibrated on the live collisions observed in changes 0109/0110, on one
# machine: treat it as a starting tolerance, not a measured constant.
DOCKET_SYNC_ATTEMPTS=5
DOCKET_SYNC_BACKOFF=(2 4 8 8)

# _docket_sync_sleep SECONDS — the injectable backoff seam. DOCKET_PREFLIGHT_TEST_SLEEP_CMD, when
# set, REPLACES the real sleep: fixtures drive all five attempts at zero wall-clock cost (the
# suite's per-file budgets make ~22s of real sleeping in a test a defect, not a style choice), and
# a fixture modelling "the other agent finished" mutates its repo from inside that command — the
# only point in the loop where a second actor could have acted. DOCKET_-namespaced per ADR-0014.
_docket_sync_sleep(){
  if [ -n "${DOCKET_PREFLIGHT_TEST_SLEEP_CMD:-}" ]; then
    eval "$DOCKET_PREFLIGHT_TEST_SLEEP_CMD" >&2 2>/dev/null || true
    return 0
  fi
  sleep "$1"
}

# _docket_tree_dirty GIT DIR — true (0) when TRACKED files are modified in DIR. Untracked-only
# files never count as dirty (ADR-0046's two-sided lesson): a stray untracked file in the shared
# worktree must never fail another agent's sync.
_docket_tree_dirty(){
  [ -n "$("$1" -C "$2" status --porcelain --untracked-files=no 2>/dev/null)" ]
}

# _docket_tree_wedged GIT DIR — true (0) when a rebase or merge is already in progress in DIR.
# Consumed by docket-status.sh's commit_and_push_generated as well as by the sync below: a commit
# into a mid-rebase shared tree commits that rebase's staged content under the caller's message.
_docket_tree_wedged(){
  local gd
  gd="$("$1" -C "$2" rev-parse --git-dir 2>/dev/null)" || return 1
  case "$gd" in /*) ;; *) gd="$2/$gd" ;; esac
  [ -d "$gd/rebase-merge" ] || [ -d "$gd/rebase-apply" ] || [ -f "$gd/MERGE_HEAD" ]
}
```

- [ ] **Step 4: Add the sync function**

Append immediately after `_docket_tree_wedged`:

```bash
# _docket_sync_metadata GIT DIR REMOTE BRANCH — the ONE metadata sync (change 0247), used by both
# branches of docket_preflight so they cannot drift apart.
#
# INVARIANT: it may report success when local metadata is already current with, or AHEAD of, the
# fetched remote; it integrates remote changes ONLY when the tracked tree is clean and no git
# operation is in progress; and it NEVER mutates another agent's in-flight state to get there — no
# --autostash, no reset, no stash. Review any change to this function against those three clauses.
#
# Returns 0 on success; 1 on a terminal failure or retry exhaustion, with a stderr diagnostic that
# NAMES the last failure class (dirty / wedged / fetch / conflict), so the caller learns what
# blocked the sync rather than merely that attempts died.
_docket_sync_metadata(){
  local git="$1" dir="$2" remote="$3" branch="$4"
  local attempt=0 last=fetch head remote_sha nap
  while [ "$attempt" -lt "$DOCKET_SYNC_ATTEMPTS" ]; do
    attempt=$((attempt + 1))
    if ! "$git" -C "$dir" fetch "$remote" "$branch" >&2; then
      # Fetch failures retry UNDISCRIMINATED, deliberately: git's exit codes do not portably
      # separate an auth or bad-remote failure from a transient network one, and stderr-pattern
      # matching is locale- and version-fragile. Accepted limit — the diagnostic carries the class
      # and git's own stderr is already on the caller's channel.
      last=fetch
    else
      head="$("$git" -C "$dir" rev-parse HEAD 2>/dev/null)"
      remote_sha="$("$git" -C "$dir" rev-parse FETCH_HEAD 2>/dev/null)"
      # FAST PATH — up to date, or ahead only. The remote is an ancestor of HEAD, so there is
      # nothing to integrate and no rebase is needed. This is the single most common collision
      # (the other agent has not pushed yet), and it must succeed on a dirty tree.
      if [ -n "$head" ] && [ -n "$remote_sha" ] \
         && "$git" -C "$dir" merge-base --is-ancestor "$remote_sha" "$head" 2>/dev/null; then
        return 0
      fi
      # The remote moved (or history diverged: local commits AND remote movement). Both cases take
      # this one path — local commits rebase onto the fetched remote under the same precondition.
      if _docket_tree_wedged "$git" "$dir"; then
        # A rebase/merge that PREDATES this attempt is another agent mid-sync: transient, so it
        # spends budget. Only the exhaustion diagnostic gets to call it wedged.
        last=wedged
      elif _docket_tree_dirty "$git" "$dir"; then
        last=dirty
      elif "$git" -C "$dir" rebase "$remote_sha" >&2; then
        return 0
      elif _docket_tree_wedged "$git" "$dir"; then
        # A content conflict raised by THIS attempt's own rebase is deterministic — it fails
        # identically on every retry — so abort (restoring the pre-attempt state) and fail now,
        # spending no further budget.
        "$git" -C "$dir" rebase --abort >&2 2>/dev/null || true
        echo "docket-preflight: metadata sync failed — this attempt's own rebase hit a content conflict in $dir. Deterministic, so it was not retried; the tree was restored. Resolve it by hand." >&2
        return 1
      else
        # An unrecognized rebase failure is TERMINAL, never a retry arm (change 0286's fail-closed
        # doctrine): a loop whose default arm is "try again" is the shape that never terminates,
        # and item 2's own rule is to spend retries only on classes that can self-heal.
        echo "docket-preflight: metadata sync failed — the rebase in $dir failed for an unrecognized reason (git's output is above). Failing closed rather than retrying." >&2
        return 1
      fi
    fi
    nap="${DOCKET_SYNC_BACKOFF[$((attempt - 1))]:-}"
    [ -n "$nap" ] && _docket_sync_sleep "$nap"
  done

  case "$last" in
    dirty) echo "docket-preflight: metadata sync failed after $DOCKET_SYNC_ATTEMPTS attempts — the tracked tree in $dir stayed dirty throughout (another agent mid-write, or a human's leftover edit). Retry later, or inspect it." >&2 ;;
    wedged) echo "docket-preflight: metadata sync failed after $DOCKET_SYNC_ATTEMPTS attempts — a rebase or merge is in progress in $dir and never cleared. This one needs a human: finish or abort it." >&2 ;;
    *)     echo "docket-preflight: metadata sync failed after $DOCKET_SYNC_ATTEMPTS attempts — the last failure was fetching $branch from $remote (git's output is above)." >&2 ;;
  esac
  return 1
}
```

- [ ] **Step 5: Route both mode branches through it**

In `docket_preflight`, replace the docket-mode sync:

```bash
    "$git" -C "$wt" fetch origin "$METADATA_BRANCH" >&2 \
      && "$git" -C "$wt" pull --rebase origin "$METADATA_BRANCH" >&2 \
      || { echo "docket-preflight: metadata worktree sync failed" >&2; return 1; }
```

with:

```bash
    _docket_sync_metadata "$git" "$wt" origin "$METADATA_BRANCH" || return 1
```

and the main-mode sync:

```bash
    "$git" -C "${root:-.}" pull --rebase >&2 || { echo "docket-preflight: metadata sync failed" >&2; return 1; }
```

with:

```bash
    # Main-mode takes the IDENTICAL path (spec, Half 1 item 1: "both branches of the sync function
    # must behave identically here; leaving it implicit is how they drift apart"). Naming the
    # remote and branch explicitly, rather than relying on the checked-out branch's upstream, is
    # part of that: the two branches now differ only in which directory they sync.
    _docket_sync_metadata "$git" "${root:-.}" origin "$METADATA_BRANCH" || return 1
```

- [ ] **Step 6: Run the tests and mutation-prove them**

Run: `bash tests/test_docket_preflight.sh`
Expected: every assert `ok`, including the pre-existing sections A–G.

Then prove each new assert's key is load-bearing. **Confirm each mutation changed bytes before trusting its result** (`grep -c` before and after):

| Mutation | Must redden |
|---|---|
| Delete the fast-path `if … merge-base --is-ancestor … return 0` block | H, I |
| Change `--untracked-files=no` to `--untracked-files=normal` in `_docket_tree_dirty` | I |
| Change `DOCKET_SYNC_ATTEMPTS=5` to `1` | J, K's backoff count |
| Delete the `nap=…; _docket_sync_sleep` lines | J, K's backoff count |
| Replace the whole `case "$last"` block with one generic message | K's class asserts |
| Move the conflict arm's `return 1` so it falls through to the retry | L's zero-backoff assert |
| Add `--autostash` to the `rebase` invocation | M |

Both directions on the bound: `[ "$attempt" -lt "$DOCKET_SYNC_ATTEMPTS" ]` must be probed by **deletion** (remove the bound → the loop must not silently pass) and by **inversion** (`-ge` → the loop body never runs, K must redden).

- [ ] **Step 7: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: green apart from the two known-pre-existing items in Global Constraints. Then measure this file: `time bash tests/test_docket_preflight.sh`. Its row is `10 parallel`. **If the measured time exceeds it, raise the row in `tests/runtime-budgets.tsv` with the measured number** — and report the remaining margin as a number, never as "did not trip the check" (learnings: `budget-headroom-is-spent-before-it-is-breached`).

- [ ] **Step 8: Commit**

```bash
git add scripts/lib/docket-preflight.sh tests/test_docket_preflight.sh tests/runtime-budgets.tsv
git commit -m "fix(0247): preflight sync survives contention — fast path, classified bounded retry

Replaces the bare fetch && pull --rebase. A dirty tree with nothing to pull
now succeeds without rebasing (the most common collision); the rebase path
retries 5 times on 2/4/8/8s backoff, spending budget ONLY on classes that can
self-heal. This attempt's own content conflict aborts and fails immediately;
an unrecognized rebase failure fails closed rather than retrying. Never
--autostash: on a shared tree that stashes another agent's edits."
```

---

### Task 2: Half 2 — scope the two commits; the `blocked-wedged-tree` token

**Read spec Assumption 16(a) before starting.** The new token needs an explicit `case` arm at *both* consumers, not just a new return value out of `commit_and_push_generated` — both existing `case` statements end in a `*)` catch-all that would silently relabel it `changed push-failed`, which is the retryable token, reintroducing the exact overloading Assumption 4 forbids.

**Files:**
- Modify: `scripts/docket-status.sh` — `commit_and_push_generated`, `board_pass_inline`, `learnings_pass`, `board_classify`, and the sweep's refresh-artifacts-links pair
- Modify: `scripts/docket-status.md` — report-line vocabulary + exit-code mapping
- Test: `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: `_docket_tree_wedged <git> <dir>` from Task 1 (`docket-status.sh` already sources `lib/docket-preflight.sh`).
- Produces: `commit_and_push_generated` gains a fourth return token, `blocked-wedged-tree`, joining `clean` / `changed-pushed` / `changed-push-failed`.
- Produces: report lines `board inline blocked-wedged-tree`, `learnings index blocked-wedged-tree`, `sweep-failed <id> render-change-links blocked-wedged-tree`.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_docket_status.sh`. Adapt the fixture-repo setup to whatever that file already provides — read its existing helpers first and reuse them rather than minting a parallel fixture.

```bash
# --- change 0247: pathspec scoping and the wedged-tree token -------------------------------------
# (1) A concurrent agent's staged file must NOT be swallowed by the board commit.
# Stage an unrelated file in the shared worktree, run a board pass that changes BOARD.md, and
# assert the board commit touched BOARD.md ONLY. This is the whole point of Half 2: the observed
# defect is another agent's staged work landing under this run's message.
assert "0247: the board commit touches BOARD.md only, never a concurrently-staged file" \
  '[ "$(git -C "$MW" show --name-only --format= HEAD | grep -vc "^$")" = "1" ] && git -C "$MW" show --name-only --format= HEAD | grep -qx "docs/changes/BOARD.md"'
assert "0247: the concurrently-staged file is still staged and uncommitted" \
  'git -C "$MW" diff --cached --name-only | grep -qx "other-agent.txt"'

# (2) A wedged tree yields the new token and attempts NO commit.
# Simulate mid-rebase by creating the rebase-merge directory git itself uses as the marker.
mkdir -p "$MW/.git/rebase-merge" 2>/dev/null || mkdir -p "$(git -C "$MW" rev-parse --git-dir)/rebase-merge"
before_sha="$(git -C "$MW" rev-parse HEAD)"
out="$(cd "$MW/.." && bash "$SCRIPTS/docket-status.sh" --board-only 2>/dev/null)"
assert "0247: a wedged tree reports blocked-wedged-tree" \
  'grep -qx "board inline blocked-wedged-tree" <<<"$out"'
assert "0247: a wedged tree is never mislabelled as the retryable push-failed token" \
  '! grep -q "board inline changed push-failed" <<<"$out"'
assert "0247: no commit was attempted on a wedged tree" \
  '[ "$(git -C "$MW" rev-parse HEAD)" = "$before_sha" ]'

# (3) --must-land treats it as NOT LANDED (a halt), never as a retryable outcome.
( cd "$MW/.." && bash "$SCRIPTS/docket-status.sh" --board-only --must-land >/dev/null 2>&1 ); rc=$?
assert "0247: --must-land exits non-zero on blocked-wedged-tree" '[ "$rc" -ne 0 ]'
rm -rf "$(git -C "$MW" rev-parse --git-dir)/rebase-merge"

# (4) changed-push-failed keeps its exact current meaning — the new token did not steal it.
assert "0247: changed-push-failed is still the sole RETRYABLE board outcome" \
  'grep -q "board inline changed push-failed" "$SCRIPTS/docket-status.sh"'
```

- [ ] **Step 2: Run to verify they fail**

Run: `bash tests/test_docket_status.sh`
Expected: the 0247 asserts report `NOT OK` — the token does not exist, and the board commit currently sweeps up `other-agent.txt`.

- [ ] **Step 3: Add the wedged probe and the pathspec in `commit_and_push_generated`**

Extend the function's contract comment — its last line currently reads `# Echoes exactly one of: clean | changed-pushed | changed-push-failed`:

```bash
# Echoes exactly one of: clean | changed-pushed | changed-push-failed | blocked-wedged-tree
#
# blocked-wedged-tree (change 0247): the shared metadata worktree has a rebase or merge in
# progress, so NOTHING is committed. Scoping the commit with a `--` pathspec makes it exit 128 in
# that state where the old pathspec-less form exited 0 — but that old exit 0 committed an
# interrupted rebase's staged content under a board-refresh message, which is corruption, not
# availability. A distinct token, never an overload of changed-push-failed: `--must-land` must
# treat this as not-landed and halt, while push-failed stays the retryable outcome.
```

Insert as the first statement of the function body, before the `status --porcelain` check:

```bash
  if _docket_tree_wedged "$GIT" "$mw"; then
    printf 'blocked-wedged-tree\n'
    return 0
  fi
```

Scope the commit itself:

```bash
    "$GIT" -C "$mw" add -- "$rel" >&2
    "$GIT" -C "$mw" commit -q -m "$commit_msg" -- "$rel" >&2 || true
```

(the `add` gains `--` too — same #0083 mark-path idiom, and it guards a `$rel` that could begin with a dash).

- [ ] **Step 4: Add the explicit `case` arms at both consumers**

In `board_pass_inline`, ahead of the catch-all:

```bash
  case "$result" in
    clean)               echo "board inline clean" ;;
    changed-pushed)      echo "board inline changed pushed" ;;
    blocked-wedged-tree) echo "board inline blocked-wedged-tree" ;;
    *)                   echo "board inline changed push-failed" ;;
  esac
```

In `learnings_pass`, the same shape:

```bash
  case "$result" in
    clean)               echo "learnings index clean" ;;
    changed-pushed)      echo "learnings index changed pushed" ;;
    blocked-wedged-tree) echo "learnings index blocked-wedged-tree" ;;
    *)                   echo "learnings index changed push-failed" ;;
  esac
```

Read the existing arms in the file first and preserve their exact strings — only the new arm is added. In `board_classify`, name the token explicitly rather than leaving it to the catch-all:

```bash
      "board inline changed pushed"|"board inline clean"|"board off"|"board github ok") ;;
      "board inline changed push-failed") has_retryable=1 ;;
      "board inline blocked-wedged-tree") has_failed=1 ;;   # 0247: NOT retryable — a human must
                                                            # clear the rebase; retrying is latency.
      *) has_failed=1 ;;   # board inline failed | board github failed | board <tok> unknown | …
```

- [ ] **Step 5: Scope the sweep's refresh-artifacts-links pair**

Replace the `if [ -n "$("$GIT" -C "$mw" status --porcelain -- "$archived" …` block's inner add/commit with:

```bash
    if _docket_tree_wedged "$GIT" "$mw"; then
      echo "sweep-failed $id render-change-links blocked-wedged-tree"
    elif ! "$GIT" -C "$mw" add -- "$archived" >&2 \
      || ! "$GIT" -C "$mw" commit -q -m "docket($id): refresh artifacts links" -- "$archived" >&2; then
      echo "sweep-failed $id render-change-links commit-failed"
    elif ! "$GIT" -C "$mw" push >&2; then
      echo "sweep-failed $id render-change-links push-failed"
    fi
```

This keeps step 6a's report-and-continue posture: like `commit-failed`, the new reason is cosmetic-and-non-terminal here — `terminal-publish.sh` and `cleanup-feature-branch.sh` still run.

- [ ] **Step 6: Update `scripts/docket-status.md`**

Add rows to the report-line vocabulary table, next to the existing `board inline changed push-failed` and `learnings index changed push-failed` rows:

```markdown
| `board inline blocked-wedged-tree` | The shared metadata worktree has a rebase or merge in progress, so the board pass committed **nothing** (change 0247). Distinct from `changed push-failed` and deliberately not retryable: a commit into a mid-rebase tree would commit that rebase's staged content under a board-refresh message. `--must-land` treats it as **not landed** (non-zero exit → the autonomous caller STOPs and abort-reports); a best-effort caller logs it and continues. Clearing it is a human act — finish or abort the in-progress operation. |
| `learnings index blocked-wedged-tree` | As above, for the learnings-index pass. |
| `sweep-failed <id> render-change-links blocked-wedged-tree` | Step 6a found the shared worktree mid-rebase and committed nothing. **Report-and-continue**, exactly like `commit-failed`: `terminal-publish.sh` and `cleanup-feature-branch.sh` still ran and the change is still reported `swept`. The `## Artifacts` block self-heals next pass. |
```

Amend the `--must-land` row so its success set stays exhaustive and the new token is explicitly outside it, and note in the exit-code mapping that `blocked-wedged-tree` is a **terminal** (non-retryable) outcome. `changed-push-failed` keeps its exact current meaning — say so, so a future reader cannot infer the token was split off from it.

- [ ] **Step 7: Run the tests and mutation-prove them**

Run: `bash tests/test_docket_status.sh`, then `scripts/run-tests.sh`.

| Mutation | Must redden |
|---|---|
| Remove `-- "$rel"` from the commit in `commit_and_push_generated` | test (1) — the staged file is swallowed |
| Delete the `_docket_tree_wedged` early return | tests (2), (3) |
| Delete only the `blocked-wedged-tree)` arm in `board_pass_inline` (leaving the return value) | test (2)'s "never mislabelled" assert — **this is the Assumption 16(a) probe; if it stays green the arm is decoration** |
| Change `board_classify`'s new arm from `has_failed=1` to `has_retryable=1` | test (3) |

The third row is the one to run most carefully: it is the exact defect Assumption 16(a) predicts, and a test that only checks for the presence of `board inline blocked-wedged-tree` while the catch-all still fires would pass it. Confirm the mutation changed bytes.

- [ ] **Step 8: Commit**

```bash
git add scripts/docket-status.sh scripts/docket-status.md tests/test_docket_status.sh
git commit -m "fix(0247): scope both shared-tree commits; add the blocked-wedged-tree token

The two pathspec-less commits in the shared metadata worktree swept up
whatever another agent had staged at that instant. Both now carry -- pathspecs.
A pathspec commit exits 128 mid-rebase where the old form exited 0 committing
an interrupted rebase's staged content, so a wedged tree is probed first and
reported as a NEW token — never overloading the retryable push-failed. Both
result case statements gain an explicit arm: their catch-alls would otherwise
silently relabel it as retryable."
```

---

### Task 3: The shape-keyed guard — Group A, the script channel

**Files:**
- Create: `tests/test_shared_worktree_commit_scope.sh`
- Modify: `tests/runtime-budgets.tsv`

**Interfaces:**
- Produces: shell functions `mask_quoted`, `logical_lines`, `scan_commits` — Task 5 extends the same file and reuses `assert`, `flatten`, and the file's header.
- Produces: findings on stdout as `<basename>:<driver-C-var>:<scoped|unscoped>` records.

**Contract boundary (spec, Half 2 item 3) — state it in the header and do not exceed it:** the guard detects `commit` as an exact-token git subcommand under an explicit driver set. It is not, and must not grow into, a general shell parser. A commit issued through a driver spelling outside the set is outside the guard's contract; introducing a new driver means extending the set in the same change — a review obligation, not something the guard infers.

**Site list — derived, never hand-listed.** Derive it fresh, do not copy this table:

```bash
grep -rnE '\bcommit\b' scripts/ --include='*.sh' | grep -E 'commit (-|--|"|\$|q)'
```

As of `main` @ a97c1542 that yields eight executable sites: `terminal-publish.sh` ×2, `mint-stub.sh`, `archive-change.sh`, `reclaim-claims.sh`, `docket-status.sh` ×3. After Task 2 exactly **one** is unscoped: `terminal-publish.sh`'s `$pub` commit. **Re-derive and reconcile against your own grep** — if the count differs, the difference is the finding.

**Two non-vacuity facts this guard must get right, both already live in the repo:**
- `docket-config.sh:91` runs `g commit-tree …` through the local `g` wrapper. `commit-tree` is **not** `commit`; a substring match false-positives on it. This is why the subcommand match is exact-token, and it is a free positive control — assert the scanner does not report it.
- `reclaim-claims.sh`'s commit message contains a `;` and `mint-stub.sh`'s contains ` #`. Unmasked text splits mid-message and false-positives on both. This is why detection runs on quote-masked text.

- [ ] **Step 1: Write the guard's scanner and its self-tests**

Create `tests/test_shared_worktree_commit_scope.sh`:

```bash
#!/usr/bin/env bash
# tests/test_shared_worktree_commit_scope.sh — change 0247.
# ONE invariant, two channels: no commit into the SHARED .docket metadata worktree may stage
# anything it did not write. The tree is dirty for another agent's whole multi-tool-call
# edit->commit window, so a pathspec-less commit lands that agent's staged work under this run's
# message (observed live 2026-08-09: an interactive groom's three staged files were swallowed by
# two concurrent autonomous commits, and the groom's own commit reported "nothing to commit").
#
#   Group A (script channel) — every `git … commit` in scripts/**/*.sh carries a `--` pathspec.
#                              Default-deny, with a keyed exception list for exclusive-worktree
#                              sites. Task-3 scope.
#   Group B (agent channel)  — the convention states the rule at the direct-git grant, and every
#                              metadata-writing skill body carries the marker. Task-5 scope.
# Both live in ONE file on purpose: a second file would split the exception lists for one invariant.
#
# CONTRACT BOUNDARY: this detects `commit` as an exact-token subcommand under an EXPLICIT driver
# set. It is not, and must not become, a general shell parser. A commit issued through a driver
# spelling outside the set is outside the contract (accepted limit — the set is small because the
# repo's metadata-writing drivers are). Introducing a new driver means extending DRIVERS in the
# same change: a review obligation, not something the guard infers.
# Run: bash tests/test_shared_worktree_commit_scope.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# The explicit driver set. `g` is docket-config.sh's local wrapper `g(){ "$GIT" -C "$REPO_DIR" "$@"; }`
# — it writes the metadata branch, so it is a metadata-writing driver even though it is one letter.
DRIVERS='git $GIT ${GIT} "$GIT" "${GIT}" g'

# mask_quoted — replace the CONTENTS of quoted runs with X, preserving quotes and length.
# Detection runs on masked text because commit MESSAGES routinely carry the very characters the
# segment splitter and the pathspec check key on: reclaim-claims.sh's message contains a `;`,
# mint-stub.sh's contains ` #`. Raw-text detection false-positives on both (#0119, settled).
mask_quoted(){
  awk '{
    out=""; inq="";
    for (i = 1; i <= length($0); i++) {
      ch = substr($0, i, 1)
      if (inq == "") { out = out ch; if (ch == "\"" || ch == "'"'"'") inq = ch }
      else if (ch == inq) { out = out ch; inq = "" }
      else { out = out "X" }
    }
    print out
  }'
}

# logical_lines FILE — join backslash-continued lines into one logical line, emitting
# "<first-line-number><TAB><joined text>". Every multi-line commit in scripts/ (mint-stub.sh,
# reclaim-claims.sh, terminal-publish.sh:$change_path) puts its `--` pathspec on a CONTINUATION
# line, so a per-physical-line scan reports every one of them as unscoped.
logical_lines(){
  awk '{
    if (buf == "") { start = NR; buf = $0 } else { buf = buf " " $0 }
    if (buf ~ /\\$/) { sub(/\\$/, "", buf); next }
    printf "%d\t%s\n", start, buf; buf = ""
  }
  END { if (buf != "") printf "%d\t%s\n", start, buf }'
}
```

- [ ] **Step 2: Add the per-segment predicate and the scanner**

```bash
# segment_is_unscoped_commit SEGMENT — true (0) when SEGMENT is a git commit lacking a `--`
# pathspec. Evaluated PER `;`/`&&`/`||`/`|` SEGMENT, never whole-line: a whole-line predicate has a
# demonstrated false NEGATIVE on the #0083 mark path, where an unscoped commit shares a line with a
# scoped neighbour whose `--` satisfies the line-level check (#0119, settled).
segment_is_unscoped_commit(){
  local seg="$1" tok driver_seen=0 subcmd=""
  for tok in $seg; do
    if [ "$driver_seen" -eq 0 ]; then
      case " $DRIVERS " in *" $tok "*) driver_seen=1 ;; esac
      continue
    fi
    # First non-flag, non-"-C <arg>" token after the driver is the subcommand.
    case "$tok" in
      -C) subcmd="__skip__"; continue ;;
      -*) continue ;;
    esac
    if [ "$subcmd" = "__skip__" ]; then subcmd=""; continue; fi
    subcmd="$tok"; break
  done
  [ "$driver_seen" -eq 1 ] || return 1
  # EXACT-TOKEN match. `commit-tree` (docket-config.sh's orphan bootstrap, via the `g` wrapper) is
  # NOT a commit: it writes a commit object from a tree with no index and no pathspec concept, so a
  # substring match would report a permanent false positive there.
  [ "$subcmd" = commit ] || return 1
  ! grep -qE '(^| )-- ' <<<"$seg"
}

# driver_target SEGMENT — the `-C` target variable name, normalized: "$mw" -> mw, "${pub}" -> pub.
# The exception KEY is <basename>:<this>, never a line number (ADR-0054): line numbers rot fastest
# in exactly the files that move most, and nothing can check them.
driver_target(){
  local seg="$1" prev="" tok
  for tok in $seg; do
    if [ "$prev" = "-C" ]; then
      tok="${tok#\"}"; tok="${tok%\"}"; tok="${tok#\$}"; tok="${tok#\{}"; tok="${tok%\}}"
      printf '%s' "$tok"; return 0
    fi
    prev="$tok"
  done
  printf '%s' "-"
}

# scan_commits — emit one record per commit call site: "<basename>:<target> <scoped|unscoped> <lineno>"
scan_commits(){
  local f rel base lno text seg IFS_SAVE
  while IFS= read -r f; do
    base="$(basename "$f")"
    while IFS=$'\t' read -r lno text; do
      text="$(mask_quoted <<<"$text")"
      # Split on ; && || | — the masked text makes this safe, since no message content survives.
      while IFS= read -r seg; do
        [ -n "$seg" ] || continue
        if segment_is_unscoped_commit "$seg"; then
          printf '%s:%s unscoped %s\n' "$base" "$(driver_target "$seg")" "$lno"
        elif grep -qE '(^| )commit( |$)' <<<"$seg" && segment_has_driver "$seg"; then
          printf '%s:%s scoped %s\n' "$base" "$(driver_target "$seg")" "$lno"
        fi
      done < <(sed 's/&&/\n/g; s/||/\n/g; s/[;|]/\n/g' <<<"$text")
    done < <(logical_lines "$f")
  done < <(find "$REPO/scripts" -name '*.sh' -type f | sort)
}

# segment_has_driver SEGMENT — the driver half of the predicate, factored out so the `scoped`
# branch above cannot drift from the `unscoped` one.
segment_has_driver(){
  local tok
  for tok in $1; do
    case " $DRIVERS " in *" $tok "*) return 0 ;; esac
  done
  return 1
}
```

**Note for the implementer:** `segment_has_driver` is referenced above its definition. In bash that is fine (functions resolve at call time, and nothing calls `scan_commits` until after sourcing completes), but move it above `scan_commits` for readability. Also verify the `sed 's/&&/\n/g'` GNU-vs-BSD behaviour on this machine — BSD `sed` does not expand `\n` in a replacement. If it does not work, use `awk '{gsub(/&&|\|\||;|\|/, "\n"); print}'` instead. **Test this before building on it.**

- [ ] **Step 3: Add Group A's asserts**

```bash
# --- group A: the script channel -----------------------------------------------------------------
FINDINGS="$(scan_commits)"
assert "A: the scanner found commit call sites at all (non-vacuity floor)" \
  '[ "$(grep -c . <<<"$FINDINGS")" -ge 6 ]'

# EXCEPTIONS — keyed <basename>:<-C target var>, each with the reason it is exempt. An exception is
# an argued exemption, never a place to park a defect.
#   terminal-publish.sh:pub — an EXCLUSIVE `mktemp -d` worktree, and the commit is index-driven
#     (the caller has built the index deliberately). A pathspec there would CHANGE BEHAVIOUR rather
#     than harden it, and the shared-tree hazard does not exist in a worktree only this process
#     holds (#0119, settled; spec Assumption 6).
EXCEPTIONS='terminal-publish.sh:pub'

unscoped="$(grep ' unscoped ' <<<"$FINDINGS" | awk '{print $1}' | sort -u)"
for key in $unscoped; do
  case " $EXCEPTIONS " in
    *" $key "*) continue ;;
  esac
  assert "A: pathspec-less commit at $key — every commit in the shared metadata worktree must stage by explicit path" 'false'
done

# EXISTENCE FLOOR — a stale exception must redden, not sit forever. This is the direction a
# forward-only loop is structurally blind to (learnings: correspondence-guard-runs-one-way).
for key in $EXCEPTIONS; do
  assert "A: exception '$key' still matches a real unscoped site (stale exceptions must redden)" \
    'grep -q "^$key unscoped " <<<"$FINDINGS"'
done

# POSITIVE CONTROLS — the two live shapes that make the design decisions load-bearing.
assert "A: commit-tree is NOT reported as a commit (exact-token subcommand match)" \
  '! grep -q "^docket-config.sh:" <<<"$FINDINGS"'
assert "A: a `;`-bearing commit MESSAGE does not split into a false positive (masked text)" \
  '! grep -q "^reclaim-claims.sh:[^ ]* unscoped " <<<"$FINDINGS"'
assert "A: a ` #`-bearing commit MESSAGE does not become a false positive (masked text)" \
  '! grep -q "^mint-stub.sh:[^ ]* unscoped " <<<"$FINDINGS"'
assert "A: both fixed docket-status.sh sites are reported scoped" \
  '[ "$(grep -c "^docket-status.sh:mw scoped " <<<"$FINDINGS")" -ge 3 ]'

exit $fail
```

- [ ] **Step 4: Run it — expect green, then prove it can go red**

Run: `bash tests/test_shared_worktree_commit_scope.sh`
Expected: all `ok`. A green run here proves nothing yet — Task 2 already fixed both sites, so this guard is green by construction and could be entirely vacuous.

**Mutation-prove it (this step is the deliverable, not the green run):**

| Mutation | Must redden |
|---|---|
| Strip `-- "$rel"` from `commit_and_push_generated`'s commit | the `docket-status.sh:mw` unscoped assert |
| Strip `-- "$archived"` from the sweep's refresh commit | same |
| Strip `-- "$REL/active/$pad-$SLUG.md"` from `mint-stub.sh` | a `mint-stub.sh:WT` unscoped assert |
| Delete the `terminal-publish.sh:pub` entry from `EXCEPTIONS` | the default-deny assert fires for it |
| Change `EXCEPTIONS` to a key that matches nothing (`foo.sh:bar`) | the existence-floor assert |
| Change the subcommand test to `[[ "$subcmd" == commit* ]]` | the `commit-tree` positive control |
| Remove the `mask_quoted` call from `scan_commits` | the `;`/` #` message controls |
| Replace the per-segment split with a whole-line predicate | the #0083 mark-path false negative — verify by checking `docket-status.sh:mw` findings drop |

For each: **confirm the mutation landed** (`grep -c` before/after) and check exact line counts, then restore from a **backup copy**, never `git checkout -- <file>` (learnings: `mutation-restore-needs-a-backup-copy` — the file has uncommitted work at this point).

- [ ] **Step 5: Add the runtime-budget row**

Measure: `time bash tests/test_shared_worktree_commit_scope.sh` (three runs; take the slowest). Add to `tests/runtime-budgets.tsv`, in the file's existing sort position:

```
tests/test_shared_worktree_commit_scope.sh	<measured, rounded up to the next multiple of 5, min 10>	parallel
```

Do **not** guess the number, and do **not** use `scripts/run-tests.sh --timings` on this path (#0290 truncates the named file to zero bytes).

- [ ] **Step 6: Run the full suite and commit**

Run: `scripts/run-tests.sh`

```bash
git add tests/test_shared_worktree_commit_scope.sh tests/runtime-budgets.tsv
git commit -m "test(0247): default-deny guard on pathspec-less commits in scripts/

Shape-keyed, per #0119's critic-settled requirements: detection runs on
quote-masked text (commit messages carry ; and #), the predicate is evaluated
per shell segment (a whole-line predicate has a demonstrated false negative on
the 0083 mark path), and `commit` matches as an exact-token subcommand under an
explicit driver set including docket-config.sh's local g wrapper — so
commit-tree does not false-positive. One keyed exception with an existence
floor, so a stale exemption reddens."
```

---

### Task 4: Half 3 — state the rule at the grant, carry the marker at every call site

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (the Step-0 preamble's direct-git grant sentence)
- Modify: `skills/docket-implement-next/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-auto-groom/SKILL.md`, `skills/docket-status/SKILL.md`, `skills/docket-new-change/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-adr/SKILL.md`
- Modify: `tests/test_skill_size_budgets.sh`

**Marker (verbatim, one house token):** `Stage by explicit path` — reused from `docket-build-task`, not a new idiom. The Task-5 guard keys on this exact literal.

**Why both, not either** (spec Half 3 item 2): a standing instruction already in context demonstrably loses to a specific instruction at the moment of action. That is the finding `tests/test_skill_handoff_precedence.sh` was built on — its header records run 40, where the wrapper's abort-and-report rule and §5's resolved-build statement were *both* in context and the sub-skill's prompt still won. A convention-only fix repeats the mistake that guard exists to prevent.

- [ ] **Step 1: State the rule at the grant**

In `skills/docket-convention/SKILL.md`, the *Step-0 preamble* section, the sentence ending `plain git plumbing (\`git add\`/\`commit\`/\`push\`, \`git -C\` forms) stays direct.` — extend it:

```markdown
plain git plumbing (`git add`/`commit`/`push`, `git -C` forms) stays direct — and **stages by explicit path**: the metadata working tree is SHARED, so `git add -A`, `git add .`, or `commit -a` there commits whatever another agent had staged at that instant, under your message and your push (observed live, change 0247 — a groom's three staged files landed in two unrelated autonomous commits, and its own commit reported "nothing to commit"). Stage by explicit path.
```

The clause naming the observed consequence is load-bearing, not padding: a rule whose cost is stated survives a slim; a bare imperative does not.

- [ ] **Step 2: Carry the marker at each of the seven commit instructions**

Append to each skill's commit instruction. **Re-derive the seven yourself** — do not trust this list:

```bash
grep -rl 'docket.sh preflight' skills/*/SKILL.md | grep -v docket-convention
```

Verified against `main` @ a97c1542 this yields exactly: `docket-implement-next`, `docket-groom-next`, `docket-auto-groom`, `docket-status`, `docket-new-change`, `docket-finalize-change`, `docket-adr`. Anchors, one per file — attach the clause to the sentence that actually instructs a commit:

- `docket-implement-next` — the *field-write rule* paragraph (`Every change-file field write this skill makes … is a **metadata commit in the metadata working tree**`).
- `docket-groom-next` — Step 5's `Commit the change-file edit + spec together in the metadata working tree`.
- `docket-auto-groom` — its `Commit the stub's outcome (change-file edit + spec when emitted) in the metadata working tree`.
- `docket-new-change` — Brainstorm mode's `committed to metadata_branch` sentence (its Scan mode's `commit them together (NOT BOARD.md)` is the second site; the file-level check covers both, and the parenthetical there already carries the intent).
- `docket-adr` — step 3's `**Commit the new ADR file only** in \`.docket/\``.
- `docket-finalize-change` — the harvest's `commit the finding file(s) + index together as **its own commit**`.
- `docket-status` — the `minted issue` write-back's `re-run \`docket.sh preflight\`, commit, push` sequence.

The clause to append, adapted to each sentence's grammar but always carrying the literal marker:

```markdown
Stage by explicit path — the metadata tree is shared, so a bare `add -A` commits another agent's staged work under your message.
```

`docket-status` is in scope **on purpose** even though its commits are made by `docket-status.sh`: the convention's Tier-A rule has the agent run that same work **inline** when dispatch is unavailable, so the prose must carry the discipline the script does.

- [ ] **Step 3: Re-measure every touched skill and set the budget rows**

**Measure first — do not reuse any number from the spec or this plan.** The spec's 2026-08-09 figures were already stale at reconcile time (`docket-auto-groom` moved from 32 words of headroom to 14, `docket-implement-next` from 11 to 30), and Steps 1–2 have now changed the files again:

```bash
for s in docket-adr docket-auto-groom docket-convention docket-finalize-change \
         docket-groom-next docket-implement-next docket-new-change docket-status; do
  f="skills/$s/SKILL.md"
  printf '%-26s %4d lines %6d words\n' "$s" "$(wc -l < "$f")" "$(wc -w < "$f")"
done
```

Compare each against its `BUDGETS` row in `tests/test_skill_size_budgets.sh`. For every row that now fails, apply **change 0137's rounding rule verbatim**: lines up to the next multiple of 5, words up to the next multiple of 50 — **and if that lands within 25 words of the actual, take the multiple after it.** Near-zero headroom is the failure mode the table exists to forbid, not a tight fit to aim for.

Add one comment paragraph per raised row to the `BUDGETS` comment block, in the established style. Each must satisfy **change 0201's rule**: name the `references/` file the new prose was considered for and argue in-diff why it cannot live there. The argument here is available and must be made explicitly rather than waved at — draft:

```
# skills/<name>/SKILL.md's budget was raised <old> -> <new> by change 0247, which states the
# shared-metadata-worktree staging rule at this skill's commit instruction. It was considered for
# skills/docket-convention/references/ (the convention's own reference directory, where the
# Step-0 preamble's longer mechanics already live) and cannot go there: the marker is a rule that
# must intervene AT THE MOMENT OF ACTION — the test header's own first example of prose that
# cannot live behind a pointer — and the whole evidential basis for stating it per-site is that a
# standing rule already in context loses to the specific instruction at that moment (run 40, the
# finding tests/test_skill_handoff_precedence.sh was built on). A rule sitting in an unread
# reference file is exactly the convention-only fix this change rejects. Set per the rounding rule
# above from the measured actual: <L> lines -> <maxL>, <W> words -> <maxW>.
```

- [ ] **Step 4: Verify and commit**

Run: `bash tests/test_skill_size_budgets.sh` — expected all `ok`.
Run: `bash tests/test_skill_handoff_precedence.sh` and `bash tests/test_consuming_repo_scripts.sh` — the skill bodies changed; confirm no neighbouring prose guard reddened.
Run: `scripts/run-tests.sh`.

```bash
git add skills/ tests/test_skill_size_budgets.sh
git commit -m "docs(0247): state the pathspec rule at the grant and at all seven call sites

The blast-radius defect reaches the shared tree through a channel no shell
guard can see: git an agent runs from skill prose. The convention's Step-0
preamble grants direct git plumbing and constrained it not at all. Now the
grant states the rule with its observed cost, and every skill that invokes
docket.sh preflight carries the house marker at its commit instruction.
Both, deliberately: a standing rule in context loses to a specific
instruction at the moment of action (run 40)."
```

---

### Task 5: The guard's Group B — the agent channel

**Read spec Assumption 16(b) and 16(d) before starting.**

**Files:**
- Modify: `tests/test_shared_worktree_commit_scope.sh` (append Groups B1 and B2 before the final `exit $fail`)

**Interfaces:**
- Consumes: `assert`, `REPO`, `fail` from Task 3's header.
- Produces: `flatten` — a local, byte-identical copy of the house helper.

**Reflow-proofing (spec Assumption 14, reconciled).** #0253 has **not** merged: `tests/lib/prose_guard.sh` does not exist on `main`. So define `flatten` locally, byte-identical to the three existing copies in `test_docket_review.sh`, `test_gate_execution_posture.sh` and `test_loop_continuation.sh`, with a comment naming #0253 as the consolidation target. A fourth local copy is the house idiom until #0253 lands, not a deviation from it. A bare `grep -qF 'Stage by explicit path'` reddens the moment an editor rewraps the sentence across a line break.

**Bind the phrase to its claim, not merely to the file** (learnings: `prose-guard-binds-phrase-to-claim`). A guard asserting a phrase is *present* survives a rewrite that keeps the words and drops the claim. Group B1's asserts bind the marker to the shared-tree reason with a **bounded** gap over flattened text.

- [ ] **Step 1: Write Group B**

Append to `tests/test_shared_worktree_commit_scope.sh`, before `exit $fail`:

```bash
# --- the reflow-proof matcher --------------------------------------------------------------------
# Byte-identical to the three existing copies (test_docket_review.sh, test_gate_execution_posture.sh,
# test_loop_continuation.sh). #0253 is hoisting these into a sourced tests/lib/prose_guard.sh — when
# it merges, replace this definition with the source; it is the consolidation target for this copy.
# Without it, a pure re-flow of the guarded sentence across a line break reddens a policy assert
# about policy that never changed (learnings: phrase-grep-over-wrapped-prose).
flatten(){ tr -s '[:space:]' ' '; }

MARKER='Stage by explicit path'

# --- group B1: the convention states the rule at the grant ---------------------------------------
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "B1: convention SKILL.md exists" '[ -f "$CONV" ]'
# Scope to the Step-0 preamble section — the same awk-range idiom test_skill_handoff_precedence.sh
# uses — so a stray mention elsewhere in a 6000-word file cannot satisfy these.
PREAMBLE="$(awk '/^### Step-0 preamble/{f=1;next} f&&/^### /{exit} f' "$CONV" | flatten)"
assert "B1: the Step-0 preamble section is non-empty (extractor floor)" '[ -n "$PREAMBLE" ]'
assert "B1: the preamble still grants direct git plumbing (the sentence the rule attaches to)" \
  'grep -qF -- "stays direct" <<<"$PREAMBLE"'
assert "B1: the preamble carries the marker" 'grep -qF -- "$MARKER" <<<"$PREAMBLE"'
# BIND the rule to its subject with a bounded gap: a guard that only proves the words are present
# survives a rewrite that keeps them and severs them from the shared tree they are about.
assert "B1: the pathspec rule is bound to the SHARED tree, not floating" \
  'grep -qiE "shared[^.]{0,120}(stage|explicit path|add -A)|((stage|explicit path|add -A)[^.]{0,120}shared)" <<<"$PREAMBLE"'
assert "B1: the preamble names the bare-add spelling it forbids" \
  'grep -qE -e "add -A" <<<"$PREAMBLE"'

# --- group B2: coverage over every metadata-writing skill ----------------------------------------
# Sites are DERIVED, never hand-listed (AGENTS.md: enumerated floor). A skill is in scope iff its
# body INVOKES `docket.sh preflight` — the convention's Step-0 preamble, which is what MAKES a skill
# an operating skill that reads and writes on metadata_branch. docket-convention is excluded as the
# rule's home.
#
# Keyed on the COMMAND STRING, not on prose describing it. The obvious predicate — "the body names
# the metadata working tree" — yields the same set today but is keyed on a SPELLING, which AGENTS.md
# forbids for exactly the reason visible in docket-adr: it already uses the variant "metadata tree"
# more often than the canonical phrase, so an ordinary slim normalizing to its own house idiom would
# silently drop it from coverage — a false green in the one channel this group exists to guard.
# `docket.sh preflight` is a literal invoked command, immune to both reflow and rewording.
IN_SCOPE="$(grep -rl 'docket.sh preflight' "$REPO"/skills/*/SKILL.md 2>/dev/null \
            | grep -v '/docket-convention/' | sort)"
assert "B2: the derivation found in-scope skills (extractor floor)" '[ -n "$IN_SCOPE" ]'
assert "B2: the derivation yields exactly 7 metadata-writing skills (found $(grep -c . <<<"$IN_SCOPE"))" \
  '[ "$(grep -c . <<<"$IN_SCOPE")" -eq 7 ]'

covered=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  sk="$(basename "$(dirname "$f")")"
  covered=$((covered+1))
  assert "B2: $sk carries the marker at its commit instruction" \
    'grep -qF -- "$MARKER" <(flatten < "$f")'
done <<<"$IN_SCOPE"
assert "B2: every in-scope skill was actually checked (covered=$covered = 7)" '[ "$covered" -eq 7 ]'

# Skills that must NOT be in scope: their commits are feature-branch, in a per-change worktree that
# is not shared. Including them would imply the shared-tree hazard applies there and dilute the
# rule's reason (spec Assumption 13).
for out in docket-build docket-build-task docket-review docket-brainstorm; do
  assert "B2: $out is correctly OUT of scope (feature worktree, not the shared tree)" \
    '! grep -q "/$out/SKILL.md$" <<<"$IN_SCOPE"'
done

# --- group B2b: cross-check against change 0208's DECLARED worktree-scope (ADR-0083) -------------
# Do not mint a second notion of scope. 0208 already established `worktree-scope:` as a declared
# frontmatter fact on agents/docket-*.md, with exactly two values. This is the reverse direction the
# forward loop above is structurally blind to (learnings: correspondence-guard-runs-one-way): every
# agent source declaring metadata scope whose skills: list names a docket operating skill must
# appear in the derived set. Five of the seven have wrappers; the other two are interactive and
# wrapper-less by construction, which is why this is a floor and not an equality.
checked_scope=0
for src in "$REPO"/agents/docket-*.md; do
  scope="$(sed -n '/^worktree-scope:/{s/^worktree-scope:[[:space:]]*//;p;q;}' "$src")"
  [ "$scope" = metadata ] || continue
  wrapped="$(sed -n '/^skills:/{s/^skills:[[:space:]]*\[//;s/\].*//;p;q;}' "$src" | cut -d, -f1 | tr -d ' ')"
  [ -n "$wrapped" ] || continue
  [ "$wrapped" = docket-convention ] && continue     # wraps no operating skill
  [ -f "$REPO/skills/$wrapped/SKILL.md" ] || continue
  checked_scope=$((checked_scope+1))
  assert "B2b: '$wrapped' declares worktree-scope: metadata, so it must be in the derived set" \
    'grep -q "/$wrapped/SKILL.md$" <<<"$IN_SCOPE"'
done
assert "B2b: the 0208 cross-check reached its population (checked_scope=$checked_scope = 5)" \
  '[ "$checked_scope" -eq 5 ]'
```

**Two accepted limits, stated rather than papered over** — put both in the file header:

```
# (a) Only two of the seven skills have a commit-bearing heading, so B2 is a FILE-LEVEL token check
#     for the other five: the marker could sit anywhere in the file and pass. Scoping to a heading
#     would silently skip five of seven, which is worse; the realistic drift — a marker deleted or
#     reflowed away — is still caught.
# (b) A skill that grows a SECOND commit site is covered by the file's single marker.
# Both need contrived prose to exploit.
```

- [ ] **Step 2: Run it**

Run: `bash tests/test_shared_worktree_commit_scope.sh`
Expected: all `ok`. **Verify the process-substitution `<(flatten < "$f")` form actually works inside the `assert` eval string** — `assert` runs `eval "$2"` and the string is single-quoted, so `$f` and `$MARKER` expand at eval time inside the loop. If it misbehaves, capture first (`hay="$(flatten < "$f")"`) and match with `<<<"$hay"`. **Prove it can pass before debugging anything else** (learnings: `plan-supplied-test-code-is-unverified`).

- [ ] **Step 3: Mutation-prove both groups**

| Mutation | Must redden |
|---|---|
| Delete the marker sentence from **one** skill body (not the convention) | that skill's B2 assert — and confirm the deletion removed the expected number of lines, not a token count |
| Delete the marker clause from the convention's preamble | B1's marker assert |
| Keep the words in the convention but move them out of the Step-0 preamble section | B1's marker assert (this is the phrase-vs-claim probe — if it stays green the section scoping is decoration) |
| Rewrite the convention clause to keep `Stage by explicit path` but drop every mention of the tree being shared | B1's binding assert |
| Re-flow the convention's marker sentence across a line break with no wording change | **nothing** must redden — this is the `flatten` proof, and it is the one mutation where green is the pass |
| Change the B2 derivation grep to a phrase predicate (`metadata working tree`) | nothing today — **record this**: it is why the command-string key was chosen, and the drift it prevents is future, not present |
| Delete a skill's `docket.sh preflight` invocation entirely | the `-eq 7` count assert |
| Flip one agent source's `worktree-scope:` from `metadata` to `feature` | B2b's `checked_scope=5` assert |

For the re-flow row, apply the mutation with a real line-break edit and **confirm the bytes changed** — against hard-wrapped prose a single-line `perl -pi` pattern silently no-ops, and a no-op mutation is indistinguishable from a guard that caught nothing.

- [ ] **Step 4: Re-measure the budget row and run the full suite**

Group B roughly doubles the file. Re-measure `time bash tests/test_shared_worktree_commit_scope.sh` and raise its `tests/runtime-budgets.tsv` row if the measurement exceeds what Task 3 set. Report the remaining margin as a number.

Run: `scripts/run-tests.sh`

- [ ] **Step 5: Commit**

```bash
git add tests/test_shared_worktree_commit_scope.sh tests/runtime-budgets.tsv
git commit -m "test(0247): guard the agent channel — the rule at the grant, the marker at every site

Group B1 scopes to docket-convention's Step-0 preamble and binds the marker to
the shared tree with a bounded gap, so a rewrite that keeps the words and drops
the claim reddens. Group B2 derives its population from the invoked command
string \`docket.sh preflight\` rather than prose describing the metadata tree —
docket-adr already prefers a variant spelling, so a routine slim would silently
drop it from a spelling-keyed predicate. B2b cross-checks against 0208's
declared worktree-scope: rather than minting a second notion of scope.
flatten() is local: #0253 has not merged."
```

---

## Self-Review

**1. Spec coverage** — every line of the spec's *Requirements checklist* maps to a task:

| Requirement | Task |
|---|---|
| No rebase when the remote has not advanced; dirty tree with nothing to pull never fails | 1, steps 3–4; test H |
| Diverged history rebases only on a clean tracked tree; both sync branches identical | 1, steps 4–5 |
| Bounded retry spent only on self-healing classes; own conflict aborts immediately; diagnostic names the last class | 1, step 4; tests K, L |
| Never `--autostash`, repo-grep asserted | 1, test M |
| Untracked-only files never dirty | 1, `_docket_tree_dirty`; test I |
| Backoff sleep injectable; no real waiting | 1, `_docket_sync_sleep`; tests J, K |
| Both `docket-status.sh` sites carry pathspecs; wedged yields `blocked-wedged-tree`; `--must-land` treats it as not-landed | 2 |
| Default-deny guard: masked text, per-segment predicate, explicit driver set, keyed exceptions + existence floor, mutation-tested | 3 |
| Every metadata-writing skill carries the marker; convention states the rule; coverage derived from `docket.sh preflight`; both groups mutation-tested; reflow-proof | 4, 5 |
| Budget raises satisfy 0201's in-diff argument and 0137's rounding | 4, step 3 |
| ADR recording survivable-over-impossible + the halt posture | **Not a plan task** — `docket-implement-next` Step 6 dispatches `docket-adr`, which assigns the number and commits on `origin/docket`. Its number lands in `adrs:`. One ADR; Half 3 adds no second (spec Assumption 7), and its *Consequences* names **both** channels. |

Reconcile Assumption 16 items map: (a) → Task 2 steps 4, 7; (b) → Task 5 Group B2b; (c) → Task 4 step 2, Task 5 B2's count assert; (d) → Task 5 `flatten`; (e) → Task 4 step 3's re-measure instruction; (f) → Task 1 step 4's fail-closed unknown arm.

**2. Placeholder scan** — no "TBD", no "add error handling", no "similar to Task N". Every code step carries real code. Three places deliberately defer to measurement rather than asserting a number: the three `runtime-budgets.tsv` values and the `test_skill_size_budgets.sh` raises. That is not a placeholder — a budget number copied from a plan instead of measured is precisely the defect the reconcile found in the spec.

**3. Type consistency** — `_docket_tree_wedged` is defined in Task 1 and consumed by name in Task 2 (two call sites) and in Task 1's own sync. `_docket_tree_dirty` and `_docket_sync_sleep` are Task-1-internal. The token spelling `blocked-wedged-tree` is identical across `commit_and_push_generated`'s return, both `case` arms, `board_classify`, the sweep's report line, and `docket-status.md`. The marker `Stage by explicit path` is byte-identical in Task 4's prose and Task 5's `MARKER`. `flatten` matches the three existing copies exactly.

**Known-weak spot, flagged rather than hidden:** Task 3's `scan_commits` is the most intricate untested code in this plan — an awk masker, a logical-line joiner, a segment splitter, and a token predicate composed together, none of which has ever run. Budget real time for Step 4's mutation table, and treat any surprise there as a defect in the scanner rather than in the scripts it scans.
