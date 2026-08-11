<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0208 — Harden runner-dispatch — worktree membership gate, feature-scoped coverage, flag-parse guards](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0208-runner-dispatch-worktree-gate-3-proves-repo-containment-not.md)**
<!-- docket:backlink:end -->

# Harden runner-dispatch — worktree membership gate, feature-scoped coverage, flag-parse guards — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `scripts/runner-dispatch.sh` reject the three inputs it currently accepts or hangs on — a `--worktree` that is merely *inside* the repo rather than a worktree of it, a feature-scoped agent dispatched with no worktree (or into the main tree), and a value-taking flag in final position.

**Architecture:** Three independent hardenings of one script's input gates, plus the declaration mechanism the second one keys on. Gate 3 becomes a real membership test built from a single `git worktree list --porcelain` capture. Agent *worktree scope* becomes a declared frontmatter fact on every built-in agent source (`worktree-scope: feature|metadata`), validated loudly by `sync-agents.sh` at generation and read tolerantly by the facade at runtime — so neither the facade nor the shim emitter carries a name list. The parse loop gains a per-site value guard at the five `shift 2` sites.

**Tech Stack:** Bash (`set -uo pipefail`, no `-e`), git plumbing, the repo's hermetic shell test suite (`bash tests/test_*.sh`, run through `scripts/run-tests.sh`).

## Global Constraints

Copied from the spec and from `AGENTS.md`; every task's requirements implicitly include these.

- **Never `producer | early-exiting-consumer` under `pipefail`.** Capture into a variable first, then `grep <<<"$var"`. This is an explicit spec requirement for the new gate.
- **`grep` for a pattern that leads with `--` must declare it:** `grep -qF -- "--worktree"`. Inside a negated assert a bare leading `--` is exit 2, which inverts into a permanently green, vacuous guard.
- **The facade's error posture is `set -uo pipefail` with NO `-e`.** Do not add `set -e`; every failure is an explicit `die`.
- **The facade's failure posture is a loud `die`, never a hang and never a silent degrade** — except where an existing comment states a deliberately tolerant posture (`runners.<name>:` value parsing, unknown-agent probing).
- **Retain the diagnostic phrase `not a worktree of this repository`** on the membership failure; it becomes accurate, and keeping it minimizes assert churn (`tests/test_runner_dispatch.sh` leg (g) greps it).
- **Frontmatter key is spelled `worktree-scope`** with values exactly `feature` or `metadata`.
- **A cross-reference in maintained source anchors on a symbol name or a verbatim-quoted clause — never a line number.**
- **A guard is code: mutation-test it.** Every new assert in this plan carries an explicit mutation check in its task. A mutation that leaves an assert green is a defect.
- **Run the whole suite at the build gate**, via `scripts/run-tests.sh` (the resolved `finalize.test_command`), never only the files this plan names. A trailing `OVER BUDGET:` line is a finding to act on.
- **Budget:** `tests/test_runner_dispatch.sh` has a 20s row in `tests/runtime-budgets.tsv`. If the additions push the measured wall clock past it, re-measure and raise the row with the measured number. `tests/test_sync_agents_runners.sh` at ~184s against its 60s row is **pre-existing and out of scope** (tracked as change #0280) — do not touch that row.

## File Structure

| File | Responsibility in this change |
|---|---|
| `scripts/runner-dispatch.sh` | Parse-loop value guards (5 sites); the scope probe + `AGENTS_SRC` seam; gate 1 keyed on scope; gate 3 rewritten as a membership test; the feature-scoped main-tree rejection |
| `scripts/runner-dispatch.md` | Contract prose: the gate list, the `build-*`-vs-feature-scoped wording, the new `AGENTS_SRC` seam, the parse-guard posture |
| `agents/docket-*.md` (16 files) | One new frontmatter line each: `worktree-scope: feature|metadata` |
| `sync-agents.sh` | `agent_worktree_scope()` reader; `validate_agent_scopes()` generation gate; `emit_shim`'s `--worktree` slot keyed on the declaration with generalized rule text |
| `scripts/runners/opencode.md` | One sentence: `build-*` → feature-scoped |
| `skills/docket-implement-next/SKILL.md` | §6 review dispatch gains the feature-worktree sentence |
| `skills/docket-finalize-change/SKILL.md`, `.../references/gate-failure.md` | Resolver/repair dispatch prose gains the feature-worktree sentence |
| `tests/test_runner_dispatch.sh` | Parse-guard legs, membership legs, scope-coverage legs, the exit-code conjunct on 0270's success path |
| `tests/test_sync_agents_runners.sh` | Scope-keyed shim asserts + the missing-key generation-failure fixture |

Task order is dependency order: Task 1 is fully independent; Task 2 is fully independent; Task 3 introduces the declaration that Task 4 reads; Task 5 is prose that describes Tasks 2–4.

---

### Task 1: Flag-parse value guards (spec §3)

A value-taking flag in final position makes `shift 2` fail (it shifts *nothing* at `$# = 1`) and, with no `set -e`, `while [ $# -gt 0 ]` spins forever. Measured: `timeout 3 bash scripts/runner-dispatch.sh --runner` returns 124.

**Files:**
- Modify: `scripts/runner-dispatch.sh` — the five `shift 2` arms of the `while [ $# -gt 0 ]` parse loop
- Test: `tests/test_runner_dispatch.sh` — a new section after the existing `# ---- facade: validation ----` section

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: a test helper `run_bounded <seconds> <facade args...>` that later tasks may reuse. It prints either the numeric exit code or the literal `HUNG`, and leaves the run's stderr in the file named by `$BOUND_ERR`.

**Site derivation — do this, do not trust the list below.** Run `grep -n 'shift 2' scripts/runner-dispatch.sh` and treat its output as the authoritative site list. At the time of writing it yields exactly five: `--runner`, `--agent`, `--model`, `--effort`, `--worktree`. `--observe` (change 0271) and `--brief-file` (change 0277) do **not** appear — each already carries its own last-argument guard in a different but equally non-hanging shape (`shift; [ $# -gt 0 ] && shift`) with an in-file comment explaining it. **Leave those two byte-identical.** Re-shaping them is out of scope and would churn a just-merged diff for no behavior change.

- [ ] **Step 1: Write the failing test**

Add to `tests/test_runner_dispatch.sh`, immediately after the `# ---- facade: validation ---------` section's last assert.

The helper's hard stop is deliberately **independent of the guard under test** (LEARNINGS: `mutation-target-needs-a-forced-exit`): completion is signalled by a sentinel *file* the subshell writes, never by `kill -0` on the pid (a reaped-but-unwaited child is a zombie, and `kill -0` on a zombie succeeds — polling liveness that way would report `HUNG` for every healthy run). The job is started under `set -m` so it leads its own process group and the give-up path can signal the whole tree, rather than orphaning a spinning `bash` that burns CPU for the rest of the suite.

```bash
# ---- 0208 leg (c): a valueless trailing flag must die, never hang ---------------------
# Every value-taking flag whose arm ends in `shift 2` hangs when the flag is the FINAL argument:
# bash's `shift` FAILS rather than truncating at `$# = 1`, the loop has no trailing shift, and the
# facade runs under `set -uo pipefail` with no `-e`. Measured before the fix:
# `timeout 3 bash scripts/runner-dispatch.sh --runner` returned 124.
#
# The bound is a background job plus a SENTINEL FILE, and both halves are load-bearing:
#   * The stop must be INDEPENDENT of the guard under test, or deleting the guard deletes the stop
#     and the mutation hangs instead of reddening (LEARNINGS: mutation-target-needs-a-forced-exit).
#   * Completion is the sentinel FILE, never `kill -0` on the pid: a finished-but-unwaited child is
#     a zombie whose pid still answers `kill -0`, so a liveness poll would report HUNG for every
#     healthy run — the assert would pass for the wrong reason and go vacuous the moment it is fixed.
#   * `set -m` makes the job a process-group LEADER so the give-up path can signal the whole tree.
#     Without it the subshell dies and the spinning facade is orphaned into the rest of the suite.
# `timeout(1)` is deliberately not used: stock macOS ships none and no existing test depends on one.
BOUND_ERR=""
run_bounded(){  # $1 = seconds to wait; $2... = args to the facade -> prints the exit code, or HUNG
  local secs="$1"; shift
  local rcf="$SBX/bounded.rc"
  BOUND_ERR="$SBX/bounded.err"
  rm -f "$rcf" "$rcf.partial"; : > "$BOUND_ERR"
  set -m
  ( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" "$@" >/dev/null 2>"$BOUND_ERR"
    printf '%s' "$?" > "$rcf.partial"; mv -f "$rcf.partial" "$rcf" ) &
  local p=$! i=0
  set +m
  while [ "$i" -lt $(( secs * 10 )) ] && [ ! -f "$rcf" ]; do sleep 0.1; i=$(( i + 1 )); done
  if [ ! -f "$rcf" ]; then
    kill -TERM "-$p" 2>/dev/null || kill -TERM "$p" 2>/dev/null
    wait "$p" 2>/dev/null
    printf 'HUNG'
    return 0
  fi
  wait "$p" 2>/dev/null
  cat "$rcf"
}

make_fixture
for f in runner agent model effort worktree; do
  rc="$(run_bounded 3 --"$f")"
  # Pinned on the MECHANISM, not merely on "it failed" (LEARNINGS: assert-pins-outcome-not-mechanism):
  # `HUNG` and a non-zero code are different outcomes, and only one of them is this leg's subject.
  assert "0208(c): trailing --$f exits rather than hanging" '[ "$rc" != "HUNG" ]'
  assert "0208(c): trailing --$f exits nonzero" '[ "$rc" != "0" ]'
  assert "0208(c): trailing --$f says it requires a value" \
    'grep -qF -- "--'"$f"' requires a value" "$BOUND_ERR"'
done
rm -rf "$SBX"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep -c '^NOT OK'`

Expected: at least 10 `NOT OK` lines — for each of the five flags, `HUNG` is reported (the exits-rather-than-hanging assert fails) and the diagnostic grep finds nothing. The whole file must still complete in a few seconds; if it wedges, the helper is wrong, not the code.

- [ ] **Step 3: Write the minimal implementation**

In `scripts/runner-dispatch.sh`, replace the five `shift 2` arms. Keep the arms' existing alignment style. Delete the now-stale `"${2:-}"` defaulting — the guard proves `$2` exists, so the `:-` fallback would only hide a future regression.

```bash
    --runner) [ $# -ge 2 ] || die "--runner requires a value"; RUNNER="$2"; shift 2 ;;
    --agent)  [ $# -ge 2 ] || die "--agent requires a value";  AGENT="$2";  shift 2 ;;
    --model)  [ $# -ge 2 ] || die "--model requires a value";  MODEL="$2";  shift 2 ;;
    --effort) [ $# -ge 2 ] || die "--effort requires a value"; EFFORT="$2"; shift 2 ;;
    --worktree) [ $# -ge 2 ] || die "--worktree requires a value"; WORKTREE="$2"; shift 2 ;;
```

Replace the comment that currently sits above the `--observe` arm (the one beginning "`shift 2` is this parser's house form") with a version that no longer describes `shift 2` as unguarded, since it now is guarded — while keeping the explanation of why `--observe` and `--brief-file` use the other shape:

```bash
    # `--observe` and `--brief-file` keep the shift-then-conditional-shift shape they were written
    # with; the arms above use the `[ $# -ge 2 ] || die` shape instead. Both are guards against the
    # same hazard — bash's `shift` FAILS rather than truncating when the flag is the last argument,
    # and this loop has no trailing shift, so an unguarded value-taking flag in final position spins
    # here forever. `--observe` additionally needs the value to stay OPTIONAL at parse time so its
    # own "--observe requires a dispatch key" refusal below is the one a caller sees.
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep -E '0208\(c\)|^NOT OK'`

Expected: 15 `ok - 0208(c): …` lines and no `NOT OK`.

- [ ] **Step 5: Mutation-test the guard**

Delete the `[ $# -ge 2 ] || die "--runner requires a value";` clause from the `--runner` arm only (edit in place; keep a copy of the original line to restore — `git checkout --` would discard this task's other uncommitted work, LEARNINGS: `mutation-restore-needs-a-backup-copy`). Re-run:

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep '0208(c): trailing --runner'`

Expected: `NOT OK - 0208(c): trailing --runner exits rather than hanging`, reported within ~3 seconds — not a wedged run. Restore the line and re-confirm green before committing.

- [ ] **Step 6: Commit**

```bash
git add scripts/runner-dispatch.sh tests/test_runner_dispatch.sh
git commit -m "fix(0208): a valueless trailing flag dies instead of hanging the parse loop"
```

---

### Task 2: Gate 3 becomes a worktree membership test (spec §1, first half)

Today's gate is `[ "$(docket_main_worktree "$ANCHOR")" = "$REPO_ROOT" ]`, which proves *containment in the repo*: it passes for the main worktree itself and for every ordinary subdirectory of it. A `build-*` delegation handed `<repo>/docs` clears all three gates.

**Files:**
- Modify: `scripts/runner-dispatch.sh` — the `# Gate 3 —` block
- Test: `tests/test_runner_dispatch.sh` — the `# ---- change 0206: --worktree, the explicit run anchor ----` section (legs (b) and (c) fixtures) plus new membership legs

**Interfaces:**
- Consumes: `run_bounded` exists from Task 1 but is not needed here.
- Produces: `ANCHOR` is `pwd -P`-normalized from this point on — every later reader in the file (the dispatch dir, the launch record's `worktree=`, `DOCKET_REPO_ROOT`) sees the physical path. Task 4's main-tree rejection compares that normalized value against `$REPO_ROOT`.

**Why the exact shapes are what they are** — carry these into the code comment, they are the design's whole content:

- **Same-repo is the FIRST `worktree` line equalling `$REPO_ROOT`, never an anywhere-in-list match.** `git worktree list` retains stale records for deleted-and-recreated directories, so a *foreign* repo's list can contain a `worktree $REPO_ROOT` line for a path that is no longer its worktree; an anywhere-match would hand a delegated run a tree docket does not own, regressing the guarantee today's gate provides. git lists the main worktree first — the same property `docket_main_worktree` already rests on.
- **Membership is the exact `worktree $ANCHOR` line** — top-level, not merely contained.
- **A non-repo path yields empty output and fails the first-line comparison**, so the not-a-repo case still falls out of this one check, exactly as today.
- **`pwd -P` normalization is load-bearing on macOS.** `mktemp -d` and user-supplied `/tmp/...` paths are symlinked (`/tmp` → `/private/tmp`) while git prints physical paths; without it the new exact-line match would falsely reject valid worktrees the old containment check accepted. `$REPO_ROOT` needs no normalization — it is git's own output already.
- **Capture into a variable, never a pipe into `grep -q`** — under `pipefail` grep's early exit races git's SIGPIPE status.

- [ ] **Step 1: Write the failing test**

Two edits in `tests/test_runner_dispatch.sh`.

**(1a)** In the change-0206 section, replace the bare `mkdir` fixture with real linked worktrees, because legs (b) and (c) must now name an actual member. Change:

```bash
mkdir -p "$SBX/.worktrees/featslug" "$SBX/sub/dir"
```

to:

```bash
# REAL linked worktrees, not `mkdir -p` directories: since 0208 the anchor gate is a MEMBERSHIP
# test, so a bare subdirectory of the main worktree is exactly the value it now rejects. A `mkdir`
# fixture here would make legs (b) and (c) assert that a rejected value is accepted.
git -C "$SBX" worktree add -q -b featslug "$SBX/.worktrees/featslug" >/dev/null 2>&1
mkdir -p "$SBX/sub/dir"
assert "0206: fixture sanity — .worktrees/featslug is a REAL linked worktree" \
  '[ -f "$SBX/.worktrees/featslug/.git" ]'
```

**(1b)** Add the new membership legs at the end of that section, immediately before its trailing `rm -rf "$SBX"`:

```bash
# (h) 0208: an ordinary subdirectory of the main worktree is CONTAINED but not a MEMBER.
# This is the value the pre-0208 gate wrongly admitted: docket_main_worktree("$SBX/sub/dir")
# returns $SBX, so containment passed and a delegated run anchored inside the primary checkout.
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$SBX/sub/dir" 2>&1 >/dev/null )"; rc=$?
assert "0208(a): an ordinary subdirectory is rejected" '[ "$rc" != "0" ]'
assert "0208(a): the subdirectory rejection names worktree top-level" \
  'grep -qiF "worktree top-level" <<<"$err"'

# (i) 0208: a real linked worktree is still accepted — the positive half of the membership pair.
# Without it every assert above is satisfied by a gate that rejects everything.
: > "$LOG"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$SBX/.worktrees/featslug" >/dev/null 2>&1 ); rc=$?
assert "0208(a): a real linked worktree is accepted" '[ "$rc" = "0" ]'
assert "0208(a): and the accepted worktree is the anchor handed to the adapter" \
  'grep -qxF -- "$SBX/.worktrees/featslug" "$LOG"'

# (j) 0208: a SYMLINKED alias of a real member is accepted — the pwd -P normalization leg.
# On macOS /tmp is a symlink to /private/tmp, so an un-normalized exact-line match would reject
# valid worktrees the old containment check accepted. This leg reproduces that shape locally.
ln -s "$SBX/.worktrees/featslug" "$SBX/featlink"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$SBX/featlink" >/dev/null 2>&1 ); rc=$?
assert "0208(a): a symlinked alias of a real worktree is accepted" '[ "$rc" = "0" ]'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep '0208(a)'`

Expected: leg (h)'s two asserts are `NOT OK` — the subdirectory is accepted today and the diagnostic phrase does not exist. Legs (i) and (j) pass already (they describe behavior the containment gate also allows); they are the non-vacuity floor for (h), not new behavior.

- [ ] **Step 3: Write the minimal implementation**

In `scripts/runner-dispatch.sh`, replace the two-line Gate 3 block:

```bash
# Gate 3 — and belong to THIS repo's worktree set, so a child harness running under an
# auto-approve permission grant is never handed a tree docket does not own. A non-repo path makes
# docket_main_worktree print nothing, so the not-a-repo case falls out of this same comparison.
[ "$(docket_main_worktree "$ANCHOR")" = "$REPO_ROOT" ] || die "--worktree $ANCHOR is not a worktree of this repository"
```

with:

```bash
# Gate 3 — MEMBERSHIP, not containment. The pre-0208 test asked docket_main_worktree "$ANCHOR",
# which answers "is this path INSIDE some worktree of this repo" — true for the main worktree
# itself and for every ordinary subdirectory of it. So the one value the gate most needs to reject,
# the repo root handed to a build worker, cleared it while the diagnostic asserted a membership
# nothing had checked.
#
# One `worktree list --porcelain` capture from $ANCHOR yields BOTH facts:
#   * same repo — the FIRST `worktree` line equals $REPO_ROOT. git lists the main worktree first,
#     the exact property docket_main_worktree already rests on. NEVER an anywhere-in-list match:
#     `worktree list` retains stale records for deleted-and-recreated directories, so a FOREIGN
#     repo's list can carry a `worktree $REPO_ROOT` line for a path that is no longer its worktree,
#     and an anywhere-match would hand a delegated run a tree docket does not own — regressing the
#     very guarantee this gate provides.
#   * membership — an exact `worktree $ANCHOR` line, i.e. a worktree TOP-LEVEL rather than merely a
#     path contained in one.
# A non-repo path yields empty output and fails the first-line comparison, so the not-a-repo case
# still falls out of this same check, as it did before.
# CAPTURED INTO A VARIABLE, never piped into `grep -q`: under `pipefail` grep's early exit races
# git's SIGPIPE status (AGENTS.md, "Shell").
# `pwd -P` runs FIRST and is load-bearing on macOS: `mktemp -d` and user-supplied /tmp paths are
# symlinked (/tmp -> /private/tmp) while git prints physical paths, so without it this exact-line
# match would falsely reject valid worktrees the old containment check accepted. It runs after the
# -d gate above, so the `cd` cannot fail. $REPO_ROOT needs no normalization — it IS git's output.
ANCHOR="$(cd "$ANCHOR" && pwd -P)"
wt_list="$("$GIT" -C "$ANCHOR" worktree list --porcelain 2>/dev/null)"
[ "$(sed -n '1s/^worktree //p' <<<"$wt_list")" = "$REPO_ROOT" ] \
  || die "--worktree $ANCHOR is not a worktree of this repository"
grep -qxF -- "worktree $ANCHOR" <<<"$wt_list" \
  || die "--worktree $ANCHOR is not a worktree of this repository (it is inside one, but a run anchor must be a worktree top-level)"
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep -E '0206|0208|0270|^NOT OK'`

Expected: every `0206:`, `0208(a):` and `0270:` line is `ok`, and there is no `NOT OK` anywhere in the file. Legs (f) (non-directory) and (g) (outside-repo) must still be green — (g) proves the retained diagnostic phrase.

- [ ] **Step 5: Mutation-test the gate**

Two mutations, restored between runs from a saved copy of the block:

1. Delete the `grep -qxF -- "worktree $ANCHOR"` line. Expected: `NOT OK - 0208(a): an ordinary subdirectory is rejected`.
2. Change the first-line test to an anywhere match — `grep -qxF -- "worktree $REPO_ROOT" <<<"$wt_list"`. Expected: leg (g) (`--worktree outside the repo worktree set is rejected`) must still be `ok`, because an unrelated `mktemp -d` is not a repo at all and its list is empty. **This mutation is expected to stay green**, and that is a residual to state, not a defect to chase: reproducing a foreign repo whose worktree list carries a stale `worktree $REPO_ROOT` record requires deleting and recreating a directory across two repos, which is more fixture than the risk warrants. Record the residual in the results file, naming the mutation and why it is not covered.

- [ ] **Step 6: Commit**

```bash
git add scripts/runner-dispatch.sh tests/test_runner_dispatch.sh
git commit -m "fix(0208): gate 3 proves worktree membership, not repo containment"
```

---

### Task 3: `worktree-scope` as a declared frontmatter fact (spec §2, generation half)

The `--worktree` requirement covers `build-*` only, while `rebase-resolver`, `integration-repair`, and the three `review-*` rungs are equally feature-scoped — two of them commit. The fix is a **declaration**, not a second name list: a name list in the facade plus a twin in `emit_shim` is the `duplicated-gate-copies-the-whole-predicate` shape, and the stub itself calls a name list "an enumerated floor that ages into the gap".

**Files:**
- Modify: `agents/docket-*.md` — 16 files, one frontmatter line each
- Modify: `sync-agents.sh` — add `agent_worktree_scope()`, add `validate_agent_scopes()`, key `emit_shim`'s slot on the declaration
- Test: `tests/test_sync_agents_runners.sh` — extend the change-0206 block

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `agent_worktree_scope <src-file>` → prints `feature`, `metadata`, or empty. Task 4's facade implements its **own** one-line probe against the same key (a different script, deliberately not a shared helper — `sync-agents.sh` and `runner-dispatch.sh` share no library today and the spec's §2 pins the runtime read as a `sed -n` frontmatter probe).

**The scope assignment, in full** — feature: `build-economy`, `build-standard`, `build-premium`, `build-max`, `rebase-resolver`, `integration-repair`, `review-lean`, `review-standard`, `review-deep`. Metadata: `adr`, `auto-groom`, `auto-groom-critic`, `brainstorm-consultant`, `finalize-change`, `implement-next`, `status`. That is 9 + 7 = 16.

- [ ] **Step 1: Write the failing test**

Add to `tests/test_sync_agents_runners.sh`, inside the change-0206 block (after the existing Direction 2 asserts, before its `rm -rf "$SBX"`), plus a new block after it.

```bash
# ---- 0208: the --worktree slot keys on the DECLARED scope, not on a name shape --------
# The 0206 asserts above are still the mirror correspondence; this widens the population they run
# over. `review-lean` is feature-scoped and matches no `build-*` name shape, so it is the leg that
# distinguishes a scope-keyed gate from the old case statement — under the old rule its shim
# carries no slot, which is exactly the silent main-tree anchor 0206 exists to eliminate.
rm -rf "$SBX"
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    review-lean: { model: test-model-x, runner: codex }\n    adr: { model: test-model-y, runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
R="$SBX/.claude/agents/docket-review-lean.md"
A="$SBX/.claude/agents/docket-adr.md"
assert "0208(b): fixture sanity — review-lean generated a real shim" \
  'grep -qF "docket.sh runner-dispatch" "$R"'
assert "0208(b): fixture sanity — adr generated a real shim" \
  'grep -qF "docket.sh runner-dispatch" "$A"'
assert "0208(b): a feature-scoped NON-build shim bakes --worktree" \
  'grep -qF -- "--worktree" "$R"'
assert "0208(b): the feature-scoped rule text is generic, not build-specific" \
  '! grep -qiF "this is a BUILD worker" "$R"'
assert "0208(b): a metadata-scoped shim still carries no --worktree" \
  '! grep -qF -- "--worktree" "$A"'
rm -rf "$SBX"

# ---- 0208: a source with no worktree-scope FAILS generation loudly --------------------
# The generation gate is where absence is PREVENTABLE — a future feature-scoped agent must not be
# able to ship undeclared. AGENTS_SRC in sync-agents.sh is hardcoded ($SCRIPT_DIR/agents, no seam),
# so the fixture copies the whole script tree and strips the key from one source in the COPY — the
# mutation-fixture pattern tests/test_docket_status.sh already uses — rather than adding a
# generator seam that exists only for this test.
mkgitrepo
mkdir -p "$SBX/.claude"
COPY="$SBX/docketcopy"
mkdir -p "$COPY"
cp "$REPO/sync-agents.sh" "$COPY/"
cp -R "$REPO/agents" "$COPY/agents"
cp -R "$REPO/scripts" "$COPY/scripts"
cp -R "$REPO/skills" "$COPY/skills"
[ -d "$REPO/cursor-rules" ] && cp -R "$REPO/cursor-rules" "$COPY/cursor-rules"
# Strip the key from ONE source. `sed -i` is not portable to BSD without an argument, so rewrite
# through a temp file beside the destination.
sed '/^worktree-scope:/d' "$COPY/agents/docket-review-lean.md" > "$COPY/agents/.tmp" \
  && mv -f "$COPY/agents/.tmp" "$COPY/agents/docket-review-lean.md"
assert "0208(b): fixture sanity — the key really was stripped from the copy" \
  '! grep -q "^worktree-scope:" "$COPY/agents/docket-review-lean.md"'
out="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$COPY/sync-agents.sh" 2>&1 )"; rc=$?
assert "0208(b): a missing worktree-scope fails generation" '[ "$rc" != "0" ]'
assert "0208(b): the refusal names the key and the agent" \
  'grep -qF "worktree-scope" <<<"$out" && grep -qF "review-lean" <<<"$out"'
assert "0208(b): and no wrappers were written" '[ ! -e "$SBX/.claude/agents/docket-adr.md" ]'
rm -rf "$SBX"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_sync_agents_runners.sh 2>&1 | grep '0208(b)'`

Expected: `0208(b): a feature-scoped NON-build shim bakes --worktree` is `NOT OK` (review-lean matches no `build-*` shape today), and every assert in the missing-key block is `NOT OK` (there is no key to strip and no gate to fail).

- [ ] **Step 3: Write the minimal implementation**

**(3a)** Add the frontmatter line to each of the 16 sources, immediately below the `skills:` line where one exists, otherwise below `description:`. For `agents/docket-build-economy.md`:

```yaml
---
name: docket-build-economy
description: Economy build-profile worker for docket-build — implements one fully-specified, pattern-following plan task under the docket-build-task contract; the cheapest of docket-build's four profiles.
skills: [docket-build-task]
worktree-scope: feature
---
```

and for `agents/docket-status.md`:

```yaml
---
name: docket-status
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by refreshing docket state, sweeping merged changes to done, and running health checks for stale claims, broken spec/plan/results links, and dependency stalls.
skills: [docket-status, docket-convention]
worktree-scope: metadata
---
```

Apply `feature` to the nine listed above and `metadata` to the seven. Verify the split mechanically before moving on:

```bash
grep -L '^worktree-scope:' agents/docket-*.md        # must print nothing
grep -c '^worktree-scope: feature'  agents/docket-*.md | grep -c ':1$'   # must be 9
grep -c '^worktree-scope: metadata' agents/docket-*.md | grep -c ':1$'   # must be 7
```

Note the side effect, and do not try to suppress it: `emit()` passes source frontmatter through verbatim (only `model:`/`effort:` are rewritten), so `worktree-scope:` also appears in every generated **Claude** wrapper. Claude Code tolerates unknown frontmatter keys, so this is harmless — but wrapper bytes change, so an existing install must re-run `sync-agents.sh` once before `--check` drift assertions settle. The Cursor/codex/opencode emitters build frontmatter from whitelists and are unaffected.

**(3b)** In `sync-agents.sh`, add the reader beside `agent_description` (same idiom, same neighbourhood):

```bash
# Extract the single-line `worktree-scope:` frontmatter value from a wrapper source file (change
# 0208). An agent's worktree scope is a DECLARED FACT, not a name shape: the delegation gates —
# emit_shim's required --worktree slot below, and runner-dispatch.sh's runtime gate — both key on
# this declaration, so a future feature-scoped agent cannot ship ungated by not matching a pattern.
agent_worktree_scope(){ sed -n '/^worktree-scope:/{s/^worktree-scope:[[:space:]]*//;p;q;}' "$1"; }
```

**(3c)** Add the generation gate, beside `validate_harness_defaults`:

```bash
# Every built-in agent source must DECLARE its worktree scope (change 0208). Loud and fatal, and
# deliberately at GENERATION time rather than at runtime: this is where new agents get wired, so it
# is the seam at which an undeclared agent is still preventable. The facade's runtime read is
# tolerant by design — a missing file or key there must keep the adapter's more specific
# unknown-agent diagnostic rather than shadowing it.
validate_agent_scopes(){  # $1 = sources dir
  local src name scope bad=0
  for src in "$1"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    scope="$(agent_worktree_scope "$src")"
    case "$scope" in
      feature|metadata) ;;
      '') log "ERROR agent '$name' declares no worktree-scope: — add 'worktree-scope: feature' or 'worktree-scope: metadata' to $src"; bad=1 ;;
      *)  log "ERROR agent '$name' declares an invalid worktree-scope '$scope' — the only values are 'feature' and 'metadata' ($src)"; bad=1 ;;
    esac
  done
  [ "$bad" = "0" ]
}
```

Call it on **both** paths, immediately after each `validate_harness_defaults` call — the `--check` leg:

```bash
    if ! validate_agent_scopes "$AGENTS_SRC"; then
      log "check: an agent source declares no valid worktree-scope — a real run would refuse to write wrappers."
      exit 1
    fi
```

and the real leg, above any `mkdir -p` or wrapper redirection, for the reason the sidecar gate states — a refusal past that point has already left a half-regenerated agent directory:

```bash
  if ! validate_agent_scopes "$AGENTS_SRC"; then
    log "ERROR an agent source declares no valid worktree-scope — no wrappers were written."
    exit 1
  fi
```

**(3d)** Key `emit_shim`'s slot on the declaration. Replace the `case "$5" in build-*)` block with:

```bash
  # change 0206, generalized by change 0208: a FEATURE-SCOPED worker must run INSIDE the worktree it
  # serves, on that branch. Keyed on the source's DECLARED `worktree-scope:` ($1 is the source file)
  # rather than on a `build-*` name shape — `rebase-resolver`, `integration-repair` and the three
  # `review-*` rungs are equally feature-scoped and match no build shape, and a second name list
  # here would be the twin of the facade's that drifts
  # (LEARNINGS: duplicated-gate-copies-the-whole-predicate). A metadata-scoped shim stays
  # byte-identical, which keeps 0206's bidirectional guard intact — now scope-keyed.
  local wt_slot="" wt_rule=""
  case "$(agent_worktree_scope "$1")" in
    feature)
      wt_slot=" --worktree <feature worktree>"
      wt_rule="This agent is FEATURE-SCOPED: it must run INSIDE the feature worktree it serves, on
that worktree's branch — never the main tree on the integration branch. Replace \`<feature worktree>\`
with the absolute path of the feature worktree your caller named (drop the angle brackets). If your
caller named no worktree, abort-and-report — never guess a path, and never omit the flag."
      ;;
  esac
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_sync_agents_runners.sh 2>&1 | grep -E '0206|0208\(b\)|^NOT OK'`

Expected: every `0206:` and `0208(b):` line is `ok`, and no `NOT OK` anywhere in the file. In particular `0206: build-* shim tells the caller to abort rather than guess a path` (which greps `abort-and-report`) must still pass under the generalized wording.

- [ ] **Step 5: Mutation-test both directions**

The correspondence here is a **mirror**, so probe both (LEARNINGS: `correspondence-guard-runs-one-way`), restoring from a saved copy between runs:

1. Revert `emit_shim`'s `case` to `case "$5" in build-*)`. Expected: `NOT OK - 0208(b): a feature-scoped NON-build shim bakes --worktree`.
2. Change `agents/docket-adr.md` to `worktree-scope: feature`. Expected: `NOT OK - 0208(b): a metadata-scoped shim still carries no --worktree` **and** `NOT OK - 0206: non-build shim carries no --worktree`.
3. Delete the `validate_agent_scopes` call from the real (non-`--check`) path. Expected: `NOT OK - 0208(b): a missing worktree-scope fails generation`.

- [ ] **Step 6: Commit**

```bash
git add agents sync-agents.sh tests/test_sync_agents_runners.sh
git commit -m "feat(0208): declare each agent's worktree scope; the shim slot keys on the declaration"
```

---

### Task 4: The facade reads the declaration and gates on it (spec §1 second half, §2 runtime half)

**Files:**
- Modify: `scripts/runner-dispatch.sh` — the header `# Mock seams:` line, a new `AGENTS_SRC` resolution beside `RUNNERS_DIR`, the scope probe, Gate 1, and the feature-scoped main-tree rejection after Gate 3
- Test: `tests/test_runner_dispatch.sh` — scope-coverage legs and the main-tree rejection leg; plus the exit-code conjunct on 0270's existing success path

**Interfaces:**
- Consumes: `worktree-scope:` on every `agents/docket-*.md` (Task 3); the normalized `ANCHOR` and the membership gate (Task 2).
- Produces: `AGENT_SCOPE` — `feature`, or empty for everything else.

**Four decisions this task must honor, each with its reason:**

1. **Only the `--worktree` requirement moves to scope.** The `case "$AGENT" in build-*)` block also carries change 0277's empty-payload refusal. That refusal stays keyed on `build-*`: its reasoning ("a build worker launched with no task improvises from whatever it finds in the worktree") is build-specific, and widening it would refuse legitimately payload-free dispatches of the newly gated set.
2. **The main-tree rejection exempts the observe anchor fallback.** `--observe` on a dispatch whose worktree was removed deliberately reassigns `ANCHOR="$REPO_ROOT"` and sets `ANCHOR_FALLBACK=1` so the durable record stays readable; the build leg then reports `task-unverifiable worktree-removed`. Rejecting the main tree unconditionally would `die` there and convert a reported non-verdict into a failed observation.
3. **The probe is shape-guarded, because `$AGENT` becomes a path component.** `$AGENT` has no shape validation today (only `$RUNNER` does). The probe runs only for a name in the same safe class `--runner` is held to; any other name yields no declared scope and falls to the tolerant metadata default — the same outcome as a missing file or key.
4. **A missing file or key is metadata-scope, tolerantly.** Generation is the loud seam. Dying here would shadow the adapter's more specific unknown-agent diagnostic and break probes of non-built-in agents.

- [ ] **Step 1: Write the failing test**

Two edits in `tests/test_runner_dispatch.sh`.

**(1a)** Extend 0270's existing config-locality block rather than authoring a duplicate success-path fixture — it already builds a real linked worktree and dispatches `build-economy --worktree "$WT"`, and asserts the anchor handed to the adapter is that worktree and not the main tree. It is missing only an exit-code conjunct. Capture the status of the existing dispatch by changing:

```bash
( cd "$WT" && PATH="$BIN:$PATH" DOCKET_HARNESS_ROOT="$SBX" \
    bash "$FACADE" --runner codex --agent build-economy --worktree "$WT" -- "0270 fixture task" >/dev/null 2>&1 )
argv="$(cat "$LOG")"
```

to:

```bash
( cd "$WT" && PATH="$BIN:$PATH" DOCKET_HARNESS_ROOT="$SBX" \
    bash "$FACADE" --runner codex --agent build-economy --worktree "$WT" -- "0270 fixture task" >/dev/null 2>&1 ); rc=$?
argv="$(cat "$LOG")"
# 0208: the SUCCESS-path conjunct the 0206 review asked for — a feature-scoped agent WITH a real
# --worktree exits 0. The argv asserts below are satisfied by any run that reached the adapter, so
# they do not by themselves distinguish "succeeded" from "succeeded then failed afterwards"; and
# without an exit-code leg the mutation where a feature-scoped dispatch aborts unconditionally
# would leave this block red only by accident. It rides HERE rather than in a second fixture
# because this block already builds the exact shape the leg needs.
assert "0208(a): a feature-scoped agent WITH a real --worktree exits 0" '[ "$rc" = "0" ]'
```

**(1b)** Add a new section after the change-0206 section:

```bash
# ---- 0208 leg (b): the --worktree requirement keys on DECLARED scope ------------------
# The pre-0208 gate matched `build-*` only, leaving `rebase-resolver`, `integration-repair` and the
# three `review-*` rungs — two of which COMMIT — able to anchor silently in the main tree on the
# integration branch. The facade reads `worktree-scope:` from the agent source, so this section
# drives REAL agent names against the REAL agents/ directory; a fabricated name would test the
# tolerant fallback instead of the gate.
make_fixture
git -C "$SBX" worktree add -q -b featslug "$SBX/.worktrees/featslug" >/dev/null 2>&1
WT="$SBX/.worktrees/featslug"

for a in rebase-resolver review-lean integration-repair; do
  err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent "$a" 2>&1 >/dev/null )"; rc=$?
  assert "0208(b): feature-scoped $a without --worktree is rejected" '[ "$rc" != "0" ]'
  assert "0208(b): the $a rejection names --worktree" 'grep -qF -- "--worktree" <<<"$err"'
done

for a in status adr; do
  ( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent "$a" >/dev/null 2>&1 ); rc=$?
  assert "0208(b): metadata-scoped $a without --worktree still succeeds" '[ "$rc" = "0" ]'
done

# A feature-scoped agent WITH a worktree reaches the adapter — the non-vacuity floor for the
# refusals above, which are otherwise satisfied by a gate that rejects every dispatch.
: > "$LOG"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent review-lean \
    --worktree "$WT" >/dev/null 2>&1 ); rc=$?
assert "0208(b): feature-scoped review-lean WITH --worktree succeeds" '[ "$rc" = "0" ]'
assert "0208(b): and its anchor is the feature worktree" 'grep -qxF -- "$WT" "$LOG"'

# The MAIN-TREE rejection: membership alone still admits the repo root, and the repo root is the
# one value the whole gate exists to reject — a feature-scoped worker anchored in the primary
# checkout on the integration branch.
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent review-lean \
    --worktree "$SBX" 2>&1 >/dev/null )"; rc=$?
assert "0208(a): a feature-scoped agent anchored at the main worktree is rejected" '[ "$rc" != "0" ]'
assert "0208(a): the main-tree rejection names the integration branch hazard" \
  'grep -qiF "integration branch" <<<"$err"'

# ...and it is SCOPED: a metadata-scoped agent may legitimately anchor at the main worktree, which
# is the default anchor for every one of them. Without this leg the rejection could be widened to
# every agent and nothing would redden.
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$SBX" >/dev/null 2>&1 ); rc=$?
assert "0208(a): a metadata-scoped agent at the main worktree is still accepted" '[ "$rc" = "0" ]'

# The tolerant fallback: an agent with no source file keeps the ADAPTER's more specific
# unknown-agent diagnostic instead of dying at the facade's scope probe. Generation is the loud
# seam for absence; the facade must not shadow the better message.
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent no-such-agent >/dev/null 2>&1 ); rc=$?
assert "0208(b): an agent with no source file is not rejected by the scope probe" '[ "$rc" = "0" ]'
rm -rf "$SBX"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep -E '0208\(b\)|main worktree|integration branch'`

Expected: all three `feature-scoped <agent> without --worktree is rejected` asserts are `NOT OK`, and both main-tree-rejection asserts are `NOT OK`. The metadata-scoped and tolerant-fallback legs pass already — they are the floors.

- [ ] **Step 3: Write the minimal implementation**

**(3a)** Header. Change the `# Mock seams:` line from `# Mock seams: RUNNERS_DIR, GIT.` to:

```bash
# Mock seams: RUNNERS_DIR, GIT, AGENTS_SRC.
```

**(3b)** Beside the existing `RUNNERS_DIR` resolution, add:

```bash
# The built-in agent sources, read at runtime for one field only: `worktree-scope:` (change 0208).
# The resolution path mirrors the adapters' own (`AGENTS_SRC="$SELF_DIR/../../agents"` in
# scripts/runners/codex.sh), with the depth adjusted because the facade sits one level shallower.
# Consumer repos run the facade out of DOCKET_SCRIPTS_DIR, so ../agents exists wherever it runs.
# The env override is a MOCK SEAM this change introduces — the adapters have no such override.
AGENTS_SRC="${AGENTS_SRC:-$SELF_DIR/../agents}"
```

**(3c)** After the `case "$RUNNER" in` shape guard (so the two path-component guards read together), add the probe:

```bash
# --- the agent's DECLARED worktree scope (change 0208) -------------------------------
# `feature` means the agent must run inside the feature worktree it serves; anything else — including
# a source file or key that cannot be read — is metadata scope. The read is TOLERANT by design and
# the loud seam is elsewhere: sync-agents.sh's validate_agent_scopes refuses to generate a wrapper
# for an undeclared agent, which is where absence is preventable. Dying here instead would shadow
# the adapter's more specific unknown-agent diagnostic and would break a probe of any agent that is
# not a built-in.
# $AGENT becomes a PATH COMPONENT below, so it earns the same shape-keyed treatment `--runner` gets
# above. It is skipped rather than fatal, for the tolerance reason just given: an off-shape name has
# no declared scope and reaches the adapter, which names it precisely.
AGENT_SCOPE=""
case "$AGENT" in
  *[!A-Za-z0-9._-]*|*..*) ;;
  *) case "$(sed -n '/^worktree-scope:/{s/^worktree-scope:[[:space:]]*//;p;q;}' \
              "$AGENTS_SRC/docket-$AGENT.md" 2>/dev/null)" in
       feature) AGENT_SCOPE="feature" ;;
     esac ;;
esac
```

**(3d)** Gate 1. Split the existing `case "$AGENT" in build-*)` block so the `--worktree` requirement is scope-keyed and 0277's payload refusal stays build-keyed:

```bash
# Gate 1 — a FEATURE-SCOPED worker must run INSIDE the worktree it serves. Keyed on the agent's
# DECLARED scope (change 0208) rather than on a `build-*` name shape: `rebase-resolver`,
# `integration-repair` and the three `review-*` rungs are equally feature-scoped and match no build
# shape, and two of them commit. It is a RUNTIME requirement (the path is runtime data), so
# sync-agents.sh's generation-time slot cannot substitute for it. Loud, matching the facade's posture
# for an unknown --runner rather than its tolerant posture for a runners.<name>: value: that
# tolerance exists so a cosmetic config typo cannot fail a live dispatch, whereas this is a request
# the facade cannot serve correctly.
if [ "$AGENT_SCOPE" = "feature" ]; then
  [ -n "$WORKTREE" ] || die "--worktree is required for feature-scoped agents (agent '$AGENT' declares worktree-scope: feature — it must run in its feature worktree, not the main tree)"
fi
# The empty-payload refusal stays keyed on `build-*` and is NOT widened to the feature-scoped set
# (change 0277, scope confirmed at 0208's reconcile). Its reason is build-specific: a build worker
# with no task at all does not error, it invents work from whatever it can see in the worktree, and
# the dispatch still looks successful. The other feature-scoped agents legitimately dispatch
# payload-free, so widening this would refuse correct dispatches.
case "$AGENT" in
  build-*)
    # EXEMPT: `--observe`, which starts no child at all — it reads a result the matching `--launch`
    # already recorded. A payload there would have nothing to carry it to, and the generated shim's
    # observe line deliberately has no brief slot, so requiring one would refuse every second half
    # of the launch/observe pair. The gate stays pre-verb for the two verbs that DO start a child:
    # `--launch` and the legacy synchronous call.
    # CONTENT, not arity: `[ $# -gt 0 ]` counts arguments, so `-- ""` satisfied it while the adapter's
    # payload came out empty and the task context vanished. The argv channel is measured by the same
    # whitespace-stripped predicate the brief-file channel is measured by above (a brief file that
    # reached this line has already been proven to carry content).
    argv_body="$*"
    [ "$VERB" = "observe" ] || [ -n "$BRIEF_FILE" ] || [ -n "${argv_body//[[:space:]]/}" ] || die "a build-* dispatch carries no task: pass the brief with --brief-file <path> (preferred) or after '--'. A build worker launched with no task does not error — it improvises from whatever it finds in the worktree and the dispatch still looks successful" ;;
esac
```

**(3e)** Immediately after Gate 3's two membership checks, add the main-tree rejection:

```bash
# Gate 3b — and for a FEATURE-SCOPED agent it must not be the MAIN worktree. Membership alone still
# admits `$REPO_ROOT`, which is the precise wrong value this whole gate exists to reject: the primary
# checkout, sitting on the integration branch, handed to a worker that commits.
# EXEMPT when the observe anchor fallback fired. `--observe` on a dispatch whose worktree has since
# been removed deliberately reassigns ANCHOR to $REPO_ROOT so the durable record stays readable, and
# the build leg then reports `task-unverifiable worktree-removed`. Refusing here would turn that
# honest non-verdict into a failed observation — the record would be durable in storage and not in
# service, the exact failure ANCHOR_FALLBACK was added to prevent.
if [ "$AGENT_SCOPE" = "feature" ] && [ "$ANCHOR_FALLBACK" != 1 ]; then
  [ "$ANCHOR" != "$REPO_ROOT" ] \
    || die "--worktree resolves to the main worktree; a feature-scoped agent must not run in the primary checkout on the integration branch"
fi
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep -c '^NOT OK'`

Expected: `0`. Then run the three sibling facade files, which exercise the same gates from other angles:

Run: `for t in tests/test_runner_dispatch_build_gate.sh tests/test_runner_dispatch_detach.sh tests/test_runner_dispatch_observe.sh; do echo "== $t"; bash "$t" 2>&1 | grep -c '^NOT OK'; done`

Expected: `0` from each. If `test_runner_dispatch_observe.sh` reddens, the anchor-fallback exemption is the first thing to check — that file drives the removed-worktree observe path.

- [ ] **Step 5: Mutation-test the two new gates**

Restoring from a saved copy between runs:

1. Change Gate 1's condition to `case "$AGENT" in build-*)`. Expected: `NOT OK - 0208(b): feature-scoped rebase-resolver without --worktree is rejected` (and the review-lean and integration-repair twins).
2. Delete the `[ "$ANCHOR" != "$REPO_ROOT" ]` line. Expected: `NOT OK - 0208(a): a feature-scoped agent anchored at the main worktree is rejected`.
3. Drop the `&& [ "$ANCHOR_FALLBACK" != 1 ]` conjunct. Expected: `tests/test_runner_dispatch_observe.sh` reddens on its removed-worktree leg. If it does **not**, that path is unguarded by the suite — say so in the results file rather than assuming coverage.

- [ ] **Step 6: Commit**

```bash
git add scripts/runner-dispatch.sh tests/test_runner_dispatch.sh
git commit -m "fix(0208): the facade gates on declared worktree scope, and refuses the main tree"
```

---

### Task 5: Contract and dispatcher prose

Widening the shim rule without editing the callers would make every runner-delegated `review-*` / `rebase-resolver` / `integration-repair` dispatch deterministically abort on the shim's "caller named no worktree" rule — their dispatch prose names no worktree today. A native (non-runner) dispatch is unaffected; the rule exists only in delegation shims.

**Files:**
- Modify: `scripts/runner-dispatch.md`
- Modify: `scripts/runners/opencode.md`
- Modify: `skills/docket-implement-next/SKILL.md`
- Modify: `skills/docket-finalize-change/SKILL.md`, `skills/docket-finalize-change/references/gate-failure.md`

**Interfaces:**
- Consumes: the behavior built in Tasks 1–4. This task adds no behavior.
- Produces: nothing later tasks read.

- [ ] **Step 1: Derive the site list, do not hand-list it**

Run and read all of it before editing — a docs-shaped reading skips executable sites and vice versa:

```bash
grep -n 'build-\*' scripts/runner-dispatch.md scripts/runners/*.md
grep -rn 'shift 2\|Mock seams' scripts/runner-dispatch.md
```

Every hit is either (i) a statement about the `--worktree` requirement or the shim slot, which becomes "feature-scoped agents (declared `worktree-scope: feature`)"; or (ii) a statement about the empty-payload gate or the `build-*` observe verdict, which stays `build-*` verbatim. Classify each hit before changing it — the two families are interleaved in that file and only the first is in scope.

- [ ] **Step 2: Update `scripts/runner-dispatch.md`**

Three edits, each anchored on a verbatim-quoted clause rather than a line number:

1. The gate description under the argument-validation section — the clause reading "`--worktree` is **required** for a `build-*` agent" becomes "`--worktree` is **required** for a **feature-scoped** agent (one whose `agents/docket-<name>.md` source declares `worktree-scope: feature`)", and the sentence gains the membership and main-tree facts: the resolved anchor must be a **directory**, must be a **worktree top-level of this repo** (membership, not containment — an ordinary subdirectory of the main worktree is refused), and for a feature-scoped agent must not be the **main worktree** itself.
2. The paragraph beginning "**`build-*` agents**: the `docket-build-task` contract requires a build worker to run inside its feature worktree" — generalize to feature-scoped agents, name the declaration as the mechanism, and list the nine feature-scoped agents once. State explicitly that the declaration is validated at generation by `sync-agents.sh` and read tolerantly at runtime.
3. The `Mock seams` documentation — add `AGENTS_SRC`, describing it as the directory the facade reads `worktree-scope:` from, defaulting to `$SELF_DIR/../agents`.

Leave untouched: "**`build-*` agents require a payload**", the `build-*` observe verdict section, and the "**`build-*` is observe-only — never re-dispatched**" rule. All three are about the payload/verdict families, not the worktree requirement.

Add one new bullet to the argument-validation section recording the parse-guard posture:

> - **A value-taking flag in final position is a loud refusal, never a hang.** Each of `--runner`, `--agent`, `--model`, `--effort`, `--worktree` guards `[ $# -ge 2 ]` before consuming its value and dies with `<flag> requires a value`. `--observe` and `--brief-file` reach the same outcome through a shift-then-conditional-shift arm, because `--observe` must keep its own "requires a dispatch key" refusal reachable.

- [ ] **Step 3: Update `scripts/runners/opencode.md`**

The sentence reading "`build-*` agent and aborts loudly when it names none (change 0206)" becomes a feature-scoped statement: the adapter's caller must name the worktree for any **feature-scoped** agent (declared `worktree-scope: feature`), and the facade aborts loudly when it names none (changes 0206, 0208).

- [ ] **Step 4: Add the dispatch sentence to the three skill sites**

Shape it on `skills/docket-build/SKILL.md`'s existing sentence, quoted verbatim as the model:

> A worker reached through a runner delegation receives its worktree through the facade's `--worktree` flag, not through the prompt body alone.

**`skills/docket-implement-next/SKILL.md`, §6** — the sentence beginning "Dispatch the selected rung wrapper by name, foreground, passing it the branch and base ref…" gains, immediately after it:

> Name the **feature worktree** in that dispatch payload: a reviewer reached through a runner delegation receives its worktree through the facade's `--worktree` flag, and a delegated dispatch that names none is refused.

**`skills/docket-finalize-change/SKILL.md`** — the rebase step's sentence "On conflict, dispatch the `docket-rebase-resolver` subagent (foreground, at the model/effort its wrapper resolves)" and the red-suite step's "dispatch `docket-integration-repair` (foreground, at the model/effort its wrapper resolves)" each gain:

> Name the **feature worktree** in the dispatch payload — reached through a runner delegation it receives that worktree through the facade's `--worktree` flag, and a delegated dispatch that names none is refused.

**`skills/docket-finalize-change/references/gate-failure.md`** — the paragraph describing the resolver/repair split gains the same sentence once, covering both agents.

- [ ] **Step 5: Verify the prose against the code**

Run: `bash tests/test_script_contracts_coverage.sh 2>&1 | tail -5` and any doc-fence suite the repo carries for skills:

Run: `bash scripts/run-tests.sh 2>&1 | tail -25`

Expected: the full suite is green. If a prose guard reddens on a phrase this task reworded, fix the *guard's* anchor to the new verbatim clause rather than restoring stale prose — but only where the guard's claim is unchanged.

- [ ] **Step 6: Commit**

```bash
git add scripts/runner-dispatch.md scripts/runners/opencode.md skills/docket-implement-next/SKILL.md skills/docket-finalize-change
git commit -m "docs(0208): feature-scoped worktree requirement in the contract and dispatch prose"
```

---

### Task 6: Full-suite gate and budget

**Files:**
- Modify (only if measurement requires it): `tests/runtime-budgets.tsv`

**Interfaces:**
- Consumes: everything above.
- Produces: the build-evidence record the review step reads.

- [ ] **Step 1: Run the whole suite**

Run: `bash scripts/run-tests.sh 2>&1 | tail -40`

Expected: every file passes. Read the tail for a trailing `OVER BUDGET:` line — it does not fail the run, so nothing else will catch it.

- [ ] **Step 2: Measure this change's own files if a budget line names them**

Run: `time bash tests/test_runner_dispatch.sh >/dev/null 2>&1`

Expected: under the 20s row. The new legs are cheap in the green case — every parse-guard leg exits immediately, and the 3-second bound is spent only when a guard is broken.

- [ ] **Step 3: Raise the row only with a measured number**

If and only if `tests/test_runner_dispatch.sh` exceeds 20s, take the median of three serial runs, round up to the next whole second with headroom, and update its row in `tests/runtime-budgets.tsv`. Do **not** touch `tests/test_sync_agents_runners.sh`'s row: its ~184s-against-60s breach is pre-existing, untouched by this change, and tracked as change #0280.

- [ ] **Step 4: Commit (only if the budget row changed)**

```bash
git add tests/runtime-budgets.tsv
git commit -m "chore(0208): re-measure the runner-dispatch test budget"
```

---

## Out of scope for this plan

- **The two ADRs.** The spec calls for one new ADR (an agent's worktree scope is a declared frontmatter fact; the delegation gates key on the declaration, never on a name list — `relates_to: [34, 68]`) plus a dated `## Update` on ADR-0068, whose Consequences assert two things this change falsifies ("the `build-*` gate is the one piece of agent-family knowledge the facade gains" and "leaving every other shim byte-identical"). Both are ledger writes on the metadata branch, minted through `docket-adr` after the build, not build tasks. Do not create ADR files on the feature branch.
- **Flag semantics and value resolution**; arg-shaped flag values (`--model --effort high`) — a wrong-value problem, not a hang.
- **Change 0207's `sync-agents.sh` gate findings** — change 0220's territory. The `worktree-scope` validation added here is a new gate, not one of those findings.
- **Change #0280's `tests/test_sync_agents_runners.sh` budget breach.**
