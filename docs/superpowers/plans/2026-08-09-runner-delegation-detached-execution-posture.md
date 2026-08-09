<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0271 — Runner delegation has no execution posture for a child that outlives its foreground call](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0271-runner-delegation-has-no-execution-posture-for-a-child-that.md)**
<!-- docket:backlink:end -->

# Runner delegation — detached execution posture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a delegated runner child outlive the single foreground call that launched it, by moving `runner-dispatch.sh` from a synchronous call-and-return to a **launch-then-observe** facade with a durable per-dispatch result directory, a sentinel, a bounded observation budget, and a git-read disposition that now covers `build-*` as well as `implement-next`.

**Architecture:** `runner-dispatch.sh` gains two verbs. `--launch` starts the adapter in its **own process group** with every stream redirected into a durable dir under the git common dir, and returns immediately with a dispatch key. `--observe <key>` performs one short, bounded, idempotent look at the sentinel file plus repo git state and returns a synthesized exit code. `verify-run.sh` stays a pure reader and grows a second, build-scoped verdict family. `emit_shim` stops baking "one foreground call, never background it" and bakes the launch-then-observe loop instead.

**Tech Stack:** POSIX-ish Bash 4+ (`scripts/`, `sync-agents.sh`), the repo's own `tests/test_*.sh` assert harness, `tests/runtime-budgets.tsv`.

## Global Constraints

- **Bash floor is 4+**, invoked through `"$DOCKET_BASH_PATH"`. No `setsid` — it is **absent on macOS** (measured, see Task 3). No `perl` dependency.
- **Never `producer | early-exiting-consumer`** under `pipefail` — capture into a variable, then `grep <<<"$var"`.
- **`grep` for a pattern leading with `--`** must use `grep -qF --` or `grep -E -e`.
- **awk indent classes are `[^[:space:]]`**, never `[^ ]`.
- **Always `mv -f`** on install/replace paths — BSD `mv` on an unwritable destination with a tty prompts, self-answers `n`, and **exits 0**.
- **`mktemp` must carry a template** — bare `mktemp` ignores `TMPDIR` on macOS. Exception: a temp file that must sit **beside its destination** for a same-filesystem atomic rename is templated there instead. The sentinel write uses that exception.
- **A guard is code: mutation-test it.** A mutation that leaves an assert green is a defect.
- **Key a guard on syntactic shape, never an enumerated list of spellings.**
- **Cross-references anchor on a symbol name or a verbatim-quoted clause — never a line number.**
- **Every new `tests/test_*.sh` needs a row in `tests/runtime-budgets.tsv`** or `tests/test_runtime_budgets.sh` fails.
- Run the whole suite at the build gate via `scripts/run-tests.sh` (the resolved `finalize.test_command`). A trailing `OVER BUDGET:` line is a finding to act on, not noise.
- ADR-0038's chokepoint property must survive: exactly **one** dispatch seam, **no** inline fallback, **no** silent retry.
- ADR-0015 unamended: model IDs stay opaque passthrough; no allowlist.
- ADR-0024 unamended: a dispatched child **observes by blocking** on short calls and never yields.

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `scripts/docket-config.sh` | resolve + emit `DELEGATION_OBSERVATION_BUDGET` | 1 |
| `scripts/docket-config.md` | document the key | 1 |
| `.docket.example.yml` | ship the commented default | 1 |
| `tests/test_docket_config.sh` | budget resolution chain + fail-closed | 1 |
| `scripts/verify-run.sh` | add the `--build` verdict family (pure reader) | 2 |
| `scripts/verify-run.md` | document the second family | 2 |
| `tests/test_verify_run.sh` | build-verdict fixtures + mutation arms | 2 |
| `scripts/lib/docket-dispatch-dir.sh` | **new** — durable dir resolution + key mint + sentinel read | 3 |
| `scripts/runner-dispatch.sh` | `--launch` detachment; `--observe` loop, budget, kill, exits | 3, 4, 5 |
| `scripts/runner-dispatch.md` | usage, exit codes, **delegation execution posture** | 3, 4, 5, 7 |
| `tests/test_runner_dispatch_detach.sh` | **new** — launch/observe/sentinel/kill/idempotence | 3, 4, 5 |
| `sync-agents.sh` | `emit_shim` launch-then-observe rewrite | 6 |
| `tests/test_sync_agents_runners.sh` | relocate the foreground guard; new shape asserts | 6 |
| `skills/docket-build/references/delegation-execution.md` | **new** — per-harness verdicts for the *adapter* launch shape | 7 |

---

### Task 1: `delegation_observation_budget` config key

**Files:**
- Modify: `scripts/docket-config.sh` (add after the `gate_observation_budget` block; add an `emit` line)
- Modify: `scripts/docket-config.md` (key table + exit-code table)
- Modify: `.docket.example.yml` (in the `Gate execution` section)
- Test: `tests/test_docket_config.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `DELEGATION_OBSERVATION_BUDGET` — a non-negative integer number of **minutes**, default `60`, exported by `docket-config.sh --export`. Task 4 reads it.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_config.sh`, following the existing `GOB-a … GOB-g` idiom (find those asserts and mirror their fixture style exactly — same `mkgitrepo`/`export_cfg` helpers the file already uses):

```bash
# ---- change 0271: delegation_observation_budget (DOB-a … DOB-f) ----------------
# Sibling of gate_observation_budget: same layering, same fail-closed integer check,
# different default (60) because a delegated AGENT RUN is a longer unit than a suite run.
mkgitrepo
out="$(export_cfg)"
assert "DOB-a: defaults to 60 with no config" \
  'grep -qxF "DELEGATION_OBSERVATION_BUDGET=60" <<<"$out"'

printf 'delegation_observation_budget: 15\n' > "$SBX/.docket.yml"
out="$(export_cfg)"
assert "DOB-b: committed layer is honored" \
  'grep -qxF "DELEGATION_OBSERVATION_BUDGET=15" <<<"$out"'

printf 'delegation_observation_budget: 5\n' > "$SBX/.docket.local.yml"
out="$(export_cfg)"
assert "DOB-c: repo-local outranks committed" \
  'grep -qxF "DELEGATION_OBSERVATION_BUDGET=5" <<<"$out"'
rm -f "$SBX/.docket.local.yml"

# 0 is legal and carries no magic — it means "observe once, then fail closed".
printf 'delegation_observation_budget: 0\n' > "$SBX/.docket.yml"
out="$(export_cfg)"
assert "DOB-d: 0 is legal, not a disabled gate" \
  'grep -qxF "DELEGATION_OBSERVATION_BUDGET=0" <<<"$out"'

# Fail CLOSED on garbage — a typo'd budget silently defaulting would make the
# fail-closed halt fire at a duration nobody chose.
printf 'delegation_observation_budget: soon\n' > "$SBX/.docket.yml"
err="$(export_cfg 2>&1 >/dev/null)"; rc=$?
assert "DOB-e: a non-integer budget is fatal" '[ "$rc" != "0" ]'
assert "DOB-f: the diagnostic names the key" \
  'grep -qF "delegation_observation_budget" <<<"$err"'
rm -rf "$SBX"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -E "DOB-"`
Expected: every `DOB-` line reads `NOT OK` (the variable is not emitted yet).

- [ ] **Step 3: Write minimal implementation**

In `scripts/docket-config.sh`, immediately after the `gate_observation_budget` `case … esac` (the block ending with the `unparseable config: gate_observation_budget` die), add:

```bash
# --- delegation_observation_budget: the delegation boundary's budget (change 0271) -----
# SIBLING of gate_observation_budget, deliberately a SEPARATE key rather than a reuse.
# The two bound different units: gate_observation_budget bounds awaiting one SUITE RUN
# started by an agent; this bounds awaiting a whole delegated AGENT RUN, which contains
# a plan task, its verification, and its commit. Folding them onto one number would force
# whichever unit is larger to set the ceiling for both.
# Integer MINUTES; default 60. Same fail-closed posture and the same full layering chain
# (repo-local > repo-committed > global > built-in) — local execution timing is
# legitimately per-machine, so it is global-able and NOT coordination-fenced.
# 0 is legal and carries no magic: "observe once, then fail closed".
# tests/test_docket_config.sh pins the chain and the boundary (DOB-a … DOB-f).
DELEGATION_OBSERVATION_BUDGET="$(lcl delegation_observation_budget)"
DELEGATION_OBSERVATION_BUDGET="${DELEGATION_OBSERVATION_BUDGET:-$(config_scalar_get committed delegation_observation_budget)}"
DELEGATION_OBSERVATION_BUDGET="${DELEGATION_OBSERVATION_BUDGET:-$(gbl delegation_observation_budget)}"
DELEGATION_OBSERVATION_BUDGET="${DELEGATION_OBSERVATION_BUDGET:-60}"
case "$DELEGATION_OBSERVATION_BUDGET" in
  ''|*[!0-9]*) die "unparseable config: delegation_observation_budget must be a non-negative integer (minutes), got '$DELEGATION_OBSERVATION_BUDGET'" ;;
esac
```

Then, in the `emit` block, add directly below the `GATE_OBSERVATION_BUDGET` line:

```bash
  emit DELEGATION_OBSERVATION_BUDGET "$DELEGATION_OBSERVATION_BUDGET"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -E "DOB-|NOT OK" | head -20`
Expected: all six `DOB-` asserts `ok`, no `NOT OK` anywhere in the file.

- [ ] **Step 5: Mutation-test the fail-closed leg**

Temporarily change the `case` guard's `*[!0-9]*` to `*[!0-9a-z]*` (admitting `soon`), re-run, and confirm `DOB-e`/`DOB-f` go red. **Restore with your own backup copy, never `git checkout --`** (that would restore to HEAD and destroy the task's other edits). Record the observed red/green in your commit message.

- [ ] **Step 6: Document the key**

In `scripts/docket-config.md`, add a row to the key table immediately after `gate_observation_budget`:

```markdown
| `delegation_observation_budget` | `60` | yes | flat top-level key; integer number of **minutes**; resolves repo-local > repo-committed > global; a non-integer aborts. Sibling of `gate_observation_budget` and deliberately separate — that key bounds awaiting one **suite run**, this one bounds awaiting a whole delegated **agent run**. Behavioral, not coordination-fenced. Bounds `runner-dispatch --observe`; `0` is legal and buys exactly one observation |
```

And to the exit-code table, after the `gate_observation_budget` row:

```markdown
| `delegation_observation_budget` is not a non-negative integer | 1 |
```

- [ ] **Step 7: Ship the commented default**

In `.docket.example.yml`, inside the `═══ Gate execution ═══` section, directly below the `gate_observation_budget: 30` line, add:

```yaml

# delegation_observation_budget — (change 0271) how long, in MINUTES, docket is willing to await a
# terminal result from a DELEGATED RUNNER CHILD it launched. Runner delegation detaches the child
# so it can outlive the single foreground call that started it; the facade then observes a durable
# sentinel rather than blocking on the child. This budget bounds that observation. When it expires
# with no sentinel, the facade KILLS the detached process group and reports the run unavailable —
# no unwatched agent keeps working on the repo after its run was declared failed.
# Distinct from gate_observation_budget on purpose: that one bounds awaiting a suite run started by
# an agent, this one bounds awaiting a whole agent run. It is docket execution policy, distinct from
# any foreground-call timeout a particular harness imposes; no harness figure belongs here.
# 0 is legal and means "observe once, then fail closed".
# Anything that is not a non-negative integer is a config error, not a silent fallback.
# scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)
delegation_observation_budget: 60
```

- [ ] **Step 8: Verify the example file still parses and is self-consistent**

Run: `bash tests/test_docket_example_yml.sh`
Expected: all `ok`. This file asserts every shipped key is documented and resolvable; if it has a "every emitted variable appears in the example" leg, this step is what satisfies it.

- [ ] **Step 9: Commit**

```bash
git add scripts/docket-config.sh scripts/docket-config.md .docket.example.yml tests/test_docket_config.sh
git commit -m "feat(0271): add delegation_observation_budget, sibling of the gate budget"
```

---

### Task 2: `verify-run.sh` build verdict family

**Files:**
- Modify: `scripts/verify-run.sh` (new `--build` mode; header usage block)
- Modify: `scripts/verify-run.md`
- Test: `tests/test_verify_run.sh`

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces: a second verdict family on stdout, invoked as
  `verify-run.sh --build --worktree <abs path> --branch <name> --since <sha>`:
  - `task-committed <branch>` — all three conjuncts hold
  - `task-incomplete <branch> <unmet…>` — tokens, in this fixed order: `branch tip tree`
  - `task-unverifiable <reason>` — the check itself could not run against the worktree
  Exit **0 whenever a verdict was produced** (`task-incomplete` is a **finding, not a failure**); exit 2 only on bad usage. Task 5 consumes these.

**Naming is load-bearing:** `task-committed`, never `task-complete`. The verdict proves the task ran to its commit and left no stranded state; it does **not** certify the commit implements the plan task correctly. Semantic judgment stays with `docket-build`'s suite gate and the review role.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_verify_run.sh`, mirroring its existing fixture helpers:

```bash
# ---- change 0271: the build verdict family --------------------------------------
# A SECOND family, never the implement-next conjuncts stretched to fit: a build task's
# terminal state is a commit on the feature branch, not a PR.
mkbuildfixture(){   # sets BWT (a repo with a feature branch), BASE (the dispatch-time sha)
  BWT="$(mktemp -d "${TMPDIR:-/tmp}/docket-bwt.XXXXXX")"; BWT="$(cd "$BWT" && pwd -P)"
  git -C "$BWT" init --quiet
  git -C "$BWT" config user.email t@t.test
  git -C "$BWT" config user.name Test
  ( cd "$BWT" && git commit --allow-empty -qm base )
  git -C "$BWT" checkout -q -b feat/thing
  BASE="$(git -C "$BWT" rev-parse HEAD)"
}
vr_build(){ bash "$ROOT/scripts/verify-run.sh" --build --worktree "$BWT" --branch feat/thing --since "$BASE" 2>&1; }

# (a) nothing committed yet -> tip unmet
mkbuildfixture
v="$(vr_build)"; rc=$?
assert "0271-a: no commit yet is task-incomplete" '[[ "$v" == task-incomplete* ]]'
assert "0271-b: the unmet token is tip" '[[ "$v" == *"tip"* ]]'
assert "0271-c: task-incomplete is a FINDING, exit 0" '[ "$rc" = "0" ]'

# (b) a commit lands -> task-committed
( cd "$BWT" && git commit --allow-empty -qm "task work" )
v="$(vr_build)"; rc=$?
assert "0271-d: an advanced tip on a clean tree is task-committed" '[[ "$v" == "task-committed feat/thing" ]]'
assert "0271-e: task-committed exits 0" '[ "$rc" = "0" ]'

# (c) a dirty tree -> tree unmet (the STRANDED-WORK case this change exists for:
#     change 0258 left +64 lines uncommitted and the caller saw only exit 143)
printf 'stranded\n' > "$BWT/stranded.txt"
v="$(vr_build)"
assert "0271-f: an untracked stranded file is task-incomplete" '[[ "$v" == task-incomplete* ]]'
assert "0271-g: the unmet token is tree" '[[ "$v" == *"tree"* ]]'
rm -f "$BWT/stranded.txt"

# (d) wrong branch -> branch unmet
git -C "$BWT" checkout -q -b feat/other
v="$(vr_build)"
assert "0271-h: the wrong branch is task-incomplete" '[[ "$v" == task-incomplete* ]]'
assert "0271-i: the unmet token is branch" '[[ "$v" == *"branch"* ]]'

# (e) a worktree that is not a repo -> task-unverifiable, never a synthesized failure
BWT="$(mktemp -d "${TMPDIR:-/tmp}/docket-norepo.XXXXXX")"
v="$(vr_build)"; rc=$?
assert "0271-j: a non-repo worktree is task-unverifiable" '[[ "$v" == task-unverifiable* ]]'
assert "0271-k: task-unverifiable still exits 0 (a verdict was produced)" '[ "$rc" = "0" ]'

# (f) usage errors are exit 2, distinct from every verdict
v="$(bash "$ROOT/scripts/verify-run.sh" --build --branch feat/thing 2>&1)"; rc=$?
assert "0271-l: --build without --worktree is a usage error (2)" '[ "$rc" = "2" ]'
assert "0271-m: the usage diagnostic is not a verdict line" '[[ "$v" != task-* ]]'

# (g) the two families never collide: --build must not accept an <id>
v="$(bash "$ROOT/scripts/verify-run.sh" --build 7 --worktree "$BWT" 2>&1)"; rc=$?
assert "0271-n: --build rejects an id (families stay separate)" '[ "$rc" = "2" ]'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_verify_run.sh 2>&1 | grep -E "0271-"`
Expected: every `0271-` assert `NOT OK` — `--build` is an unknown argument today.

- [ ] **Step 3: Write minimal implementation**

In `scripts/verify-run.sh`, extend the argument parser. Add to the variable initialisation line:

```bash
ID=""; CHANGES_DIR=""; MODE="verdict"; WITH_CLAIMED_AT=0
BUILD_WORKTREE=""; BUILD_BRANCH=""; BUILD_SINCE=""
```

Add these cases to the `while` loop, before the `-*) die` catch-all:

```bash
    --build) MODE="build" ;;
    --worktree) BUILD_WORKTREE="${2:-}"; shift ;;
    --branch) BUILD_BRANCH="${2:-}"; shift ;;
    --since) BUILD_SINCE="${2:-}"; shift ;;
```

Add these usage guards next to the existing ones:

```bash
[ "$MODE" != "build" ] || [ -z "$ID" ] || die "an <id> cannot be combined with --build"
if [ "$MODE" = "build" ]; then
  [ -n "$BUILD_WORKTREE" ] || die "--build requires --worktree"
  [ -n "$BUILD_BRANCH" ]   || die "--build requires --branch"
  [ -n "$BUILD_SINCE" ]    || die "--build requires --since"
fi
```

Insert the build family **before** the `if [ "$MODE" = "ids" ]` block, so it never falls through to the changes-dir resolution it does not need (a build verdict reads a worktree, not the metadata tree):

```bash
# --- the BUILD verdict family (change 0271) -----------------------------------
# A SECOND family, deliberately not the implement-next conjuncts stretched to fit: a build
# task's terminal state is a commit on its feature branch, never a PR. Reads a WORKTREE; it
# needs no changes dir, which is why it returns above the resolver.
#
# NAMING: `task-committed`, never `task-complete`. These three conjuncts prove the task ran
# to its commit and stranded nothing. They do NOT certify the commit implements the plan task
# correctly — that judgment stays with docket-build's suite gate and the review role, and the
# verdict name must not let a caller read more into it than it claims.
if [ "$MODE" = "build" ]; then
  # Every failure to READ is `task-unverifiable`, never a synthesized incompleteness: a
  # missing worktree is an absence of evidence, and reporting it as unmet conjuncts would
  # be a guess wearing a verdict's clothes.
  [ -d "$BUILD_WORKTREE" ] \
    || { printf 'task-unverifiable worktree-missing\n'; exit 0; }
  "$GIT" -C "$BUILD_WORKTREE" rev-parse --git-dir >/dev/null 2>&1 \
    || { printf 'task-unverifiable not-a-repo\n'; exit 0; }

  bunmet=()
  # 1. on the expected branch
  cur="$("$GIT" -C "$BUILD_WORKTREE" rev-parse --abbrev-ref HEAD 2>/dev/null)"
  [ "$cur" = "$BUILD_BRANCH" ] || bunmet+=(branch)
  # 2. the tip advanced past the dispatch-time sha. `--since` is the direct analogue of
  #    DISPATCH_EPOCH: captured after the before-read, so a commit landing in the gap is
  #    excluded either way. An unresolvable since-sha is unverifiable, not "advanced".
  tip="$("$GIT" -C "$BUILD_WORKTREE" rev-parse HEAD 2>/dev/null)"
  if ! "$GIT" -C "$BUILD_WORKTREE" cat-file -e "${BUILD_SINCE}^{commit}" 2>/dev/null; then
    printf 'task-unverifiable unknown-since-sha\n'; exit 0
  fi
  [ -n "$tip" ] && [ "$tip" != "$BUILD_SINCE" ] || bunmet+=(tip)
  # 3. the working tree is clean — INCLUDING untracked files. The stranded-work case this
  #    whole change exists for (change 0258) left its +64 lines UNTRACKED, so a
  #    tracked-only check would have called that run clean.
  dirty="$("$GIT" -C "$BUILD_WORKTREE" status --porcelain 2>/dev/null)"
  [ -z "$dirty" ] || bunmet+=(tree)

  if [ "${#bunmet[@]}" -eq 0 ]; then
    printf 'task-committed %s\n' "$BUILD_BRANCH"; exit 0
  fi
  printf 'task-incomplete %s %s\n' "$BUILD_BRANCH" "${bunmet[*]}"
  exit 0
fi
```

Update the script's header usage comment to name the second family:

```bash
#        verify-run.sh --build --worktree DIR --branch NAME --since SHA
#   Build-family verdict lines (change 0271; a build task's terminal state is a COMMIT, not a PR):
#     task-committed <branch>                 on-branch, tip advanced, tree clean
#     task-incomplete <branch> <unmet…>       tokens: branch tip tree
#     task-unverifiable <reason>              the worktree could not be read — never a guess
#   `task-committed` proves CLEAN COMPLETION, never semantic success.
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_verify_run.sh`
Expected: every assert `ok`, including the pre-existing implement-next family (the two families must not interfere).

- [ ] **Step 5: Mutation-test the untracked-files conjunct**

Change `status --porcelain` to `status --porcelain --untracked-files=no`, re-run, and confirm `0271-f`/`0271-g` go red. That mutation is the exact defect the change exists to catch, so a green assert there would be a decoration. Restore from your own backup copy.

- [ ] **Step 6: Document the family**

In `scripts/verify-run.md`, add a `## Build verdicts (change 0271)` section stating: the three conjuncts; the fixed token order `branch tip tree`; that `task-incomplete` is a finding at exit 0; that `task-unverifiable` is returned for every failure to read rather than a synthesized incompleteness; and — verbatim, because Task 5 depends on it — that **`task-committed` proves clean completion, not semantic success.**

- [ ] **Step 7: Commit**

```bash
git add scripts/verify-run.sh scripts/verify-run.md tests/test_verify_run.sh
git commit -m "feat(0271): add the build verdict family to verify-run, still a pure reader"
```

---

### Task 3: durable dispatch dir, detached launch, and the sentinel

**Files:**
- Create: `scripts/lib/docket-dispatch-dir.sh`
- Modify: `scripts/runner-dispatch.sh` (add `--launch`; keep the legacy synchronous path intact for now)
- Modify: `scripts/runner-dispatch.md`
- Create test: `tests/test_runner_dispatch_detach.sh`
- Modify: `tests/runtime-budgets.tsv`

**Interfaces:**
- Consumes: nothing from Tasks 1–2.
- Produces, sourced by `runner-dispatch.sh`:
  - `docket_dispatch_root <worktree>` → absolute `<git-common-dir>/docket/dispatch`, created.
  - `docket_dispatch_mint <root> <agent>` → prints a new unique key `<agent>-<YYYYMMDDTHHMMSSZ>-<pid>` and creates `<root>/<key>/`.
  - `docket_dispatch_dir <root> <key>` → prints the absolute per-dispatch dir; returns 1 if absent.
  - `docket_sentinel_field <dir> <key>` → prints one field from `<dir>/done`, empty when absent/malformed.
  - Per-dispatch files: `launch` (KEY=value: `pgid`, `started_at`, `agent`, `runner`, `worktree`, `since_sha`), `stdout.log`, `stderr.log`, `done` (the sentinel), `killed` (written only on budget exhaustion).
  - Sentinel schema (flat KEY=value): `exit_code`, `started_at`, `finished_at`, `pid`, `dispatch_key`.
  Tasks 4 and 5 consume all of these.

**The measured mechanism.** `setsid` is **absent on macOS** and there is no `perl` dependency budget, so detachment uses Bash **job control**: `set -m` makes a background job a **process-group leader**. Measured on this machine, 2026-08-09, one variable changed between arms in a single run: a launcher started two children, one under `set -m` and one not, then the launcher's whole process group received `TERM`. The `set -m` child (own PGID) **survived**; the non-`set -m` child (launcher's PGID) was **killed**. That is capability 1's stronger reading — survival of the harness's teardown of the initiating call's **process group**, not merely its parent's exit.

- [ ] **Step 1: Write the failing test**

Create `tests/test_runner_dispatch_detach.sh`:

```bash
#!/usr/bin/env bash
# tests/test_runner_dispatch_detach.sh — the launch/observe detachment posture (change 0271).
# Run: bash tests/test_runner_dispatch_detach.sh
# Hermetic: a FAKE adapter script stands in for every runner, so nothing here needs a child
# harness CLI. The fake sleeps for a caller-controlled duration and writes a marker, which is
# what makes "survived the group teardown" observable rather than asserted.
set -uo pipefail
unset XDG_CONFIG_HOME
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCKET_BASH_PATH=""
for runtime_candidate in "$(command -v bash)" /opt/homebrew/bin/bash /usr/local/bin/bash; do
  [ -x "$runtime_candidate" ] || continue
  [ "$(LC_ALL=C "$runtime_candidate" --version 2>/dev/null | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')" -ge 4 ] 2>/dev/null || continue
  DOCKET_BASH_PATH="$runtime_candidate"; break
done
: "${DOCKET_BASH_PATH:?tests require an absolute GNU Bash 4+ runtime}"
export DOCKET_BASH_PATH
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

FACADE="$ROOT/scripts/runner-dispatch.sh"

make_fixture(){  # sets SBX (repo), RDIR (fake runners dir)
  SBX="$(mktemp -d "${TMPDIR:-/tmp}/docket-detach.XXXXXX")"; SBX="$(cd "$SBX" && pwd -P)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  ( cd "$SBX" && git commit --allow-empty -qm init )
  RDIR="$SBX/fake-runners"; mkdir -p "$RDIR"
  # The fake adapter: sleeps FAKE_SLEEP, then writes a marker and exits FAKE_RC.
  cat > "$RDIR/fake.sh" <<'FAKE'
#!/usr/bin/env bash
sleep "${FAKE_SLEEP:-0}"
printf 'adapter-ran\n' > "$FAKE_MARKER"
printf 'fake adapter stdout\n'
printf 'fake adapter stderr\n' >&2
exit "${FAKE_RC:-0}"
FAKE
  chmod +x "$RDIR/fake.sh"
}
launch(){ ( cd "$SBX" && RUNNERS_DIR="$RDIR" FAKE_MARKER="$SBX/marker" \
    FAKE_SLEEP="${FAKE_SLEEP:-0}" FAKE_RC="${FAKE_RC:-0}" \
    bash "$FACADE" --launch --runner fake --agent "${1:-status}" ); }

# ---- launch returns immediately with a key -------------------------------------
make_fixture
FAKE_SLEEP=5
start=$(date +%s)
KEY="$(launch status)"; rc=$?
elapsed=$(( $(date +%s) - start ))
assert "launch exits 0" '[ "$rc" = "0" ]'
assert "launch prints a dispatch key" '[ -n "$KEY" ]'
assert "the key names the agent" '[[ "$KEY" == status-* ]]'
# THE POINT OF THE CHANGE: launch must NOT block for the child's duration.
assert "launch returned well before the child finished" '[ "$elapsed" -lt 3 ]'

DDIR="$(cd "$SBX" && git rev-parse --git-common-dir)"
DDIR="$(cd "$SBX/$DDIR" 2>/dev/null || cd "$DDIR"; pwd -P)/docket/dispatch/$KEY"
assert "the per-dispatch dir exists" '[ -d "$DDIR" ]'
assert "a launch record was written" '[ -f "$DDIR/launch" ]'
assert "the launch record carries a pgid" 'grep -qE "^pgid=[0-9]+$" "$DDIR/launch"'
assert "the launch record carries started_at" 'grep -qE "^started_at=[0-9TZ:-]+$" "$DDIR/launch"'

# The child's OWN process group — the whole detachment property, asserted on the MECHANISM
# and not merely on an outcome that an unrelated pass could satisfy.
lpgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
mypgid="$(ps -o pgid= -p $$ | tr -d ' ')"
assert "the child is in its OWN process group, not the test's" '[ "$lpgid" != "$mypgid" ]'

# ---- the sentinel is written as the LAST act, by the wrapper, not the agent -----
assert "no sentinel while the child is still running" '[ ! -f "$DDIR/done" ]'
for _ in $(seq 1 40); do [ -f "$DDIR/done" ] && break; sleep 1; done
assert "the sentinel appears after the child finishes" '[ -f "$DDIR/done" ]'
assert "the adapter actually ran" '[ -f "$SBX/marker" ]'
assert "sentinel carries exit_code" 'grep -qE "^exit_code=0$" "$DDIR/done"'
assert "sentinel carries finished_at" 'grep -qE "^finished_at=[0-9TZ:-]+$" "$DDIR/done"'
assert "sentinel carries the dispatch key" 'grep -qxF "dispatch_key=$KEY" "$DDIR/done"'

# ---- every stream redirected to a durable location -----------------------------
assert "stdout was captured durably" 'grep -qF "fake adapter stdout" "$DDIR/stdout.log"'
assert "stderr was captured durably" 'grep -qF "fake adapter stderr" "$DDIR/stderr.log"'

# ---- a failing adapter records its code, and the sentinel is still well-formed --
make_fixture
FAKE_SLEEP=0 FAKE_RC=7
KEY="$(launch status)"
DDIR="$(cd "$SBX" && git rev-parse --git-common-dir)"
DDIR="$(cd "$SBX/$DDIR" 2>/dev/null || cd "$DDIR"; pwd -P)/docket/dispatch/$KEY"
for _ in $(seq 1 30); do [ -f "$DDIR/done" ] && break; sleep 1; done
assert "a failing adapter still writes a sentinel" '[ -f "$DDIR/done" ]'
assert "the sentinel records the adapter's real code" 'grep -qxF "exit_code=7" "$DDIR/done"'

# ---- build-* still requires --worktree at launch time --------------------------
make_fixture
err="$( ( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --launch --runner fake --agent build-standard ) 2>&1 >/dev/null )"; rc=$?
assert "build-* without --worktree is still a loud abort at launch" '[ "$rc" != "0" ]'
assert "the diagnostic still names --worktree" 'grep -qF -- "--worktree" <<<"$err"'

rm -rf "$SBX"
exit "$fail"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_runner_dispatch_detach.sh 2>&1 | head -20`
Expected: fails at the first assert — `--launch` is an unknown argument today.

- [ ] **Step 3: Write the dispatch-dir library**

Create `scripts/lib/docket-dispatch-dir.sh`:

```bash
#!/usr/bin/env bash
# scripts/lib/docket-dispatch-dir.sh — the durable per-dispatch result area (change 0271).
# Sourced by runner-dispatch.sh; never executed directly.
#
# WHERE: <git-common-dir>/docket/dispatch/<key>/ — the same family as
# disable-worktree-hooks.sh's docket-owned dir inside the common git dir. Under .git/, so it is
# never tracked, never leaks into a commit, and needs no .gitignore entry. It is NOT in the
# feature worktree: a dispatch result must outlive `git worktree remove`, and the whole point of
# this change is that the child's work can be inspected after the run was declared over.
#
# KEY: <agent>-<UTC timestamp>-<pid>. Keyed on agent + a mint rather than on change id or
# worktree, so two concurrent dispatches FOR THE SAME CHANGE never collide.

docket_dispatch_root(){  # $1 = a path inside the repo -> absolute dispatch root (created)
  local wt="${1:-.}" common
  common="$(cd "$wt" && cd "$("${GIT:-git}" rev-parse --git-common-dir 2>/dev/null)" && pwd -P)" || return 1
  printf '%s/docket/dispatch' "$common"
}

docket_dispatch_mint(){  # $1 = root, $2 = agent -> prints a fresh unique key, creates its dir
  local root="$1" agent="$2" stamp key
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  key="$agent-$stamp-$$"
  # A same-second, same-pid collision is not reachable in practice, but a silent reuse would
  # overwrite a live dispatch's sentinel — so refuse rather than clobber.
  [ -e "$root/$key" ] && return 1
  mkdir -p "$root/$key" || return 1
  printf '%s' "$key"
}

docket_dispatch_dir(){  # $1 = root, $2 = key -> prints the dir, 1 when absent
  local d="$1/$2"
  [ -d "$d" ] || return 1
  printf '%s' "$d"
}

docket_sentinel_field(){  # $1 = dispatch dir, $2 = field -> value, empty when absent/malformed
  [ -f "$1/done" ] || return 0
  sed -n "s/^$2=//p" "$1/done" | sed -n 1p
}
```

- [ ] **Step 4: Add the `--launch` verb to the facade**

In `scripts/runner-dispatch.sh`, source the library next to the existing `docket-root.sh` source:

```bash
# shellcheck source=lib/docket-dispatch-dir.sh
. "$SELF_DIR/lib/docket-dispatch-dir.sh"
```

Add `--launch` and `--observe` to the argument parser (the `--observe` branch is filled in by Task 4; parse it here so the two verbs are validated together):

```bash
    --launch)  VERB="launch" ;;
    --observe) VERB="observe"; OBSERVE_KEY="${2:-}"; shift 2; continue ;;
```

and initialise alongside the other variables:

```bash
VERB=""; OBSERVE_KEY=""
```

Then, **after** the existing anchor gates and the `runners.<name>:` config resolution (so `--launch` inherits every gate `build-*` already gets — this is why the verb branch sits here and not at the top), and **before** the legacy synchronous handoff, insert:

```bash
# --- verb: --launch (change 0271) ---------------------------------------------------
# Detach the adapter so the delegated run can OUTLIVE this call, then return at once with the
# dispatch key. The posture and its six required capabilities are cited, never restated:
# skills/docket-build/references/gate-execution.md, plus the adapter-launch verdicts in
# skills/docket-build/references/delegation-execution.md.
if [ "$VERB" = "launch" ]; then
  DROOT="$(docket_dispatch_root "$ANCHOR")" || die "cannot resolve the dispatch root for $ANCHOR"
  mkdir -p "$DROOT" || die "cannot create the dispatch root $DROOT"
  KEY="$(docket_dispatch_mint "$DROOT" "$AGENT")" || die "cannot mint a dispatch key under $DROOT"
  DDIR="$DROOT/$KEY"
  STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  # The dispatch-time SHA: the direct analogue of DISPATCH_EPOCH, captured BEFORE the child can
  # commit anything, so a commit landing in the gap is excluded either way. Empty on a repo with
  # no commits — the build verdict then reports unknown-since-sha rather than guessing.
  SINCE_SHA="$("$GIT" -C "$ANCHOR" rev-parse HEAD 2>/dev/null || true)"

  # DETACHMENT, measured (2026-08-09): `set -m` makes a background job a PROCESS-GROUP LEADER,
  # so it survives the harness's teardown of THIS call's process group — capability 1's stronger
  # reading. Measured with one variable changed between two arms of a single run: the set -m
  # child survived a group-directed TERM, the non-set-m child did not. `setsid` is ABSENT on
  # macOS, which is why job control rather than a new session is the mechanism.
  # Every stream is redirected to the durable dir and stdin is closed, so nothing remains
  # attached to the initiating call (capability 2).
  # THE WRAPPER WRITES THE SENTINEL, NEVER THE AGENT: "done" must not be a claim by the party
  # being judged. The write is atomic — a temp file BESIDE its destination (the one licensed
  # exception to templating temp files into TMPDIR, because the rename must be same-filesystem)
  # then `mv -f`; a reader therefore never sees a half-written sentinel.
  set -m
  {
    "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@"
    ec=$?
    printf 'exit_code=%s\nstarted_at=%s\nfinished_at=%s\npid=%s\ndispatch_key=%s\n' \
      "$ec" "$STARTED_AT" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$$" "$KEY" > "$DDIR/done.partial"
    mv -f "$DDIR/done.partial" "$DDIR/done"
  } >"$DDIR/stdout.log" 2>"$DDIR/stderr.log" </dev/null &
  CHILD_PID=$!
  set +m

  # RACE PRECONDITION (0223's measured one): the new group must be fully established before this
  # call returns, or the harness's teardown wins the race. Poll briefly and FAIL CLOSED rather
  # than return a key for a child that is still in our group and about to be reaped with us.
  MY_PGID="$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ')"
  CHILD_PGID=""
  for _ in $(seq 1 50); do
    CHILD_PGID="$(ps -o pgid= -p "$CHILD_PID" 2>/dev/null | tr -d ' ')"
    # The child may already have finished — that is establishment too, and the sentinel proves it.
    [ -f "$DDIR/done" ] && break
    [ -n "$CHILD_PGID" ] && [ "$CHILD_PGID" != "$MY_PGID" ] && break
    sleep 0.1
  done
  if [ -z "$CHILD_PGID" ] && [ ! -f "$DDIR/done" ]; then
    die "launch failed — the detached child never appeared (key $KEY)"
  fi
  if [ -n "$CHILD_PGID" ] && [ "$CHILD_PGID" = "$MY_PGID" ] && [ ! -f "$DDIR/done" ]; then
    kill -TERM "$CHILD_PID" 2>/dev/null
    die "launch failed — the child did not separate into its own process group; refusing to report a dispatch that a teardown would kill (key $KEY)"
  fi

  printf 'pgid=%s\nchild_pid=%s\nstarted_at=%s\nagent=%s\nrunner=%s\nworktree=%s\nsince_sha=%s\n' \
    "${CHILD_PGID:-$CHILD_PID}" "$CHILD_PID" "$STARTED_AT" "$AGENT" "$RUNNER" "$ANCHOR" "${SINCE_SHA:-}" \
    > "$DDIR/launch.partial"
  mv -f "$DDIR/launch.partial" "$DDIR/launch"

  printf '%s\n' "$KEY"
  exit 0
fi
```

Note: `runner-dispatch.sh` does not currently define `GIT`. Add `GIT="${GIT:-git}"` next to the other seam definitions at the top, and name it in the contract's mock-seams list.

- [ ] **Step 5: Run test to verify it passes**

Run: `bash tests/test_runner_dispatch_detach.sh`
Expected: every assert `ok`.

- [ ] **Step 6: Mutation-test the detachment itself**

This is the assert that must not be decoration. Remove the `set -m` line (leaving `set +m`), re-run, and confirm **"the child is in its OWN process group, not the test's"** goes red. Then restore from your own backup copy and re-run to green. If that assert stays green without `set -m`, the guard is measuring nothing and must be rewritten before you proceed.

- [ ] **Step 7: Register the runtime budget**

Add a row to `tests/runtime-budgets.tsv` (a new `tests/test_*.sh` with no row fails `tests/test_runtime_budgets.sh`). This file sleeps deliberately, so budget it generously:

```
tests/test_runner_dispatch_detach.sh	60	parallel
```

Use a literal TAB between fields. Then run `bash tests/test_runtime_budgets.sh` and expect all `ok`.

- [ ] **Step 8: Document the launch verb**

In `scripts/runner-dispatch.md`, add `--launch` to the Usage block and a `### Launch` subsection under Behavior covering: the durable dir location and why it is under the git common dir; the key shape and why it is agent + mint; the `set -m` mechanism with its measured justification and the macOS `setsid` absence; the race precondition and its fail-closed refusal; the sentinel schema; and that **the wrapper writes the sentinel, never the agent**. Add `GIT` to the mock-seams list.

- [ ] **Step 9: Commit**

```bash
git add scripts/lib/docket-dispatch-dir.sh scripts/runner-dispatch.sh scripts/runner-dispatch.md \
        tests/test_runner_dispatch_detach.sh tests/runtime-budgets.tsv
git commit -m "feat(0271): detach the adapter into its own process group with a durable sentinel"
```

---

### Task 4: `--observe` — sentinel read, budget, kill-on-giving-up, synthesized exits

**Files:**
- Modify: `scripts/runner-dispatch.sh` (the `--observe` branch)
- Modify: `scripts/runner-dispatch.md` (exit codes + observation)
- Test: `tests/test_runner_dispatch_detach.sh`

**Interfaces:**
- Consumes: Task 1's `DELEGATION_OBSERVATION_BUDGET`; Task 3's dir layout, `launch` record, and sentinel.
- Produces: `runner-dispatch.sh --observe <key>` — one **short, idempotent** observation. Exit codes, consumed by Task 5 and by Task 6's shim:

| Exit | Meaning |
|---|---|
| `0` | terminal, complete |
| `1` | terminal, failed — or result unavailable (distinct stderr diagnostic) |
| `3` | terminal, halted (a human is needed) |
| `4` | **not terminal — still running; observe again** |

**Exit-code discipline.** Per `LEARNINGS: exit-code-encodes-a-non-failure`, enumerate the consumers before minting `4`. There is exactly one: the generated shim wrapper, whose current rule is bare-non-zero → abort-and-report. `4` is **not** a failure, so that consumer is **changed in this same change** (Task 6) to loop on `4` and abort on any other non-zero. No other caller reads this facade's code.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_runner_dispatch_detach.sh`, above the final `exit "$fail"`:

```bash
# ---- observe: still running -> 4, terminal -> 0 ---------------------------------
observe(){ ( cd "$SBX" && RUNNERS_DIR="$RDIR" \
    DELEGATION_OBSERVATION_BUDGET="${BUDGET:-60}" \
    bash "$FACADE" --observe "$1" --runner fake --agent "${2:-status}" ); }

make_fixture
FAKE_SLEEP=6 FAKE_RC=0
KEY="$(launch status)"
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "observe on a live child exits 4 (still running)" '[ "$rc" = "4" ]'
assert "observe says still running" 'grep -qi "still running" <<<"$out"'
# The observation must be SHORT — it may not become the long foreground call all over again.
start=$(date +%s)
observe "$KEY" >/dev/null 2>&1
assert "an observation is short-lived" '[ $(( $(date +%s) - start )) -lt 10 ]'

for _ in $(seq 1 40); do
  observe "$KEY" >/dev/null 2>&1; [ "$?" != "4" ] && break; sleep 1
done
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "observe after a clean child exits 0" '[ "$rc" = "0" ]'

# IDEMPOTENCE: same inputs, same verdict and code, every time.
out2="$(observe "$KEY" 2>&1)"; rc2=$?
assert "observe is idempotent in code" '[ "$rc2" = "$rc" ]'
assert "observe is idempotent in output" '[ "$out2" = "$out" ]'

# ---- a failed child -> 1 --------------------------------------------------------
make_fixture
FAKE_SLEEP=0 FAKE_RC=9
KEY="$(launch status)"
for _ in $(seq 1 30); do observe "$KEY" >/dev/null 2>&1; [ "$?" != "4" ] && break; sleep 1; done
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "a non-zero adapter code observes as failed (1)" '[ "$rc" = "1" ]'
assert "the failure diagnostic reports the child's code" 'grep -qF "9" <<<"$out"'

# ---- budget exhaustion kills the GROUP and reports unavailable ------------------
# The orphan policy (honors change 0231): no unwatched agent keeps working after the run was
# declared failed.
make_fixture
FAKE_SLEEP=120 FAKE_RC=0
BUDGET=0                       # legal, and buys exactly ONE observation
KEY="$(launch status)"
DDIR="$(cd "$SBX" && git rev-parse --git-common-dir)"
DDIR="$(cd "$SBX/$DDIR" 2>/dev/null || cd "$DDIR"; pwd -P)/docket/dispatch/$KEY"
lpgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "budget exhaustion exits 1" '[ "$rc" = "1" ]'
assert "the diagnostic distinguishes unavailable from failed" 'grep -qi "unavailable" <<<"$out"'
sleep 2
assert "the detached process group was killed" '! kill -0 -"$lpgid" 2>/dev/null'
assert "a killed marker was recorded" '[ -f "$DDIR/killed" ]'
assert "the adapter never completed its work" '[ ! -f "$SBX/marker" ]'
# Deterministic re-observation AFTER the terminal kill.
out2="$(observe "$KEY" 2>&1)"; rc2=$?
assert "re-observing a killed dispatch stays unavailable (1)" '[ "$rc2" = "1" ]'
assert "re-observation after the kill is deterministic" 'grep -qi "unavailable" <<<"$out2"'
BUDGET=60

# ---- a malformed sentinel with a dead child is UNAVAILABLE, never a fake failure -
make_fixture
FAKE_SLEEP=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(cd "$SBX" && git rev-parse --git-common-dir)"
DDIR="$(cd "$SBX/$DDIR" 2>/dev/null || cd "$DDIR"; pwd -P)/docket/dispatch/$KEY"
for _ in $(seq 1 30); do [ -f "$DDIR/done" ] && break; sleep 1; done
printf 'garbage-not-a-schema\n' > "$DDIR/done"
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "a malformed sentinel is unavailable, not a synthesized failure" '[ "$rc" = "1" ]'
assert "the malformed-sentinel diagnostic says unavailable" 'grep -qi "unavailable" <<<"$out"'
assert "no exit code was read out of garbage" '! grep -qi "exited garbage" <<<"$out"'

# ---- an unknown key is a usage error, never a verdict ---------------------------
out="$(observe "no-such-key-0000" 2>&1)"; rc=$?
assert "an unknown dispatch key aborts" '[ "$rc" != "0" ]'
assert "the unknown-key diagnostic names the key" 'grep -qF "no-such-key-0000" <<<"$out"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_runner_dispatch_detach.sh 2>&1 | grep "NOT OK" | head`
Expected: the new observe asserts fail; Task 3's launch asserts still pass.

- [ ] **Step 3: Write the observe branch**

In `scripts/runner-dispatch.sh`, directly after the `--launch` block, add:

```bash
# --- verb: --observe (change 0271) --------------------------------------------------
# ONE short, idempotent look. Never a long foreground call — that ceiling is the defect this
# change removes, so re-introducing it here would defeat the whole design.
#
# LIVENESS comes from the sentinel; CORRECTNESS comes from git (the verify-run verdicts wired in
# Task 5). A sentinel claiming success with no matching git evidence is a FAILURE — correctness
# wins. That split is why the facade may observe while the child is alive without ever inferring
# liveness from git state.
if [ "$VERB" = "observe" ]; then
  [ -n "$OBSERVE_KEY" ] || die "--observe requires a dispatch key"
  DROOT="$(docket_dispatch_root "$ANCHOR")" || die "cannot resolve the dispatch root for $ANCHOR"
  DDIR="$(docket_dispatch_dir "$DROOT" "$OBSERVE_KEY")" \
    || die "unknown dispatch key '$OBSERVE_KEY' (no result directory under $DROOT)"

  LPGID="$(sed -n 's/^pgid=//p' "$DDIR/launch" 2>/dev/null | sed -n 1p)"
  LSTART="$(sed -n 's/^started_at=//p' "$DDIR/launch" 2>/dev/null | sed -n 1p)"

  # 1. A prior budget kill is TERMINAL and re-reports identically forever (idempotence).
  if [ -f "$DDIR/killed" ]; then
    printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (the detached run was killed at budget exhaustion)\n' "$OBSERVE_KEY" >&2
    exit 1
  fi

  # 2. A sentinel means the child is DONE. Well-formed vs malformed is the difference between
  #    a clean adapter exit and a wrapper crash — the latter is `unavailable`, NEVER an exit
  #    code read out of garbage.
  if [ -f "$DDIR/done" ]; then
    SEC="$(docket_sentinel_field "$DDIR" exit_code)"
    case "$SEC" in
      ''|*[!0-9]*)
        printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (the sentinel is malformed; the launcher did not finish cleanly)\n' "$OBSERVE_KEY" >&2
        exit 1 ;;
    esac
    if [ "$SEC" = "0" ]; then
      printf 'runner-dispatch: observe %s — complete (child exited 0)\n' "$OBSERVE_KEY" >&2
      exit 0
    fi
    printf 'runner-dispatch: observe %s — FAILED (child exited %s); see %s/stderr.log\n' \
      "$OBSERVE_KEY" "$SEC" "$DDIR" >&2
    exit 1
  fi

  # 3. No sentinel: still running, unless the budget is spent. `0` is legal and buys exactly ONE
  #    observation — this one — so the comparison is `>=`, evaluated AFTER the sentinel read
  #    above, which is what makes the single observation a real one.
  NOW="$(date -u +%s)"
  START_EPOCH=""
  [ -n "$LSTART" ] && START_EPOCH="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" --iso-to-epoch "$LSTART" 2>/dev/null)"
  case "${START_EPOCH:-}" in
    ''|*[!0-9]*)
      # No readable start time is no positive evidence that the budget is spent — keep observing
      # rather than kill a healthy child on a guess.
      printf 'runner-dispatch: observe %s — still running (start time unreadable; budget not enforced this pass)\n' "$OBSERVE_KEY" >&2
      exit 4 ;;
  esac
  ELAPSED_MIN=$(( (NOW - START_EPOCH) / 60 ))
  if [ "$ELAPSED_MIN" -lt "${DELEGATION_OBSERVATION_BUDGET:-60}" ]; then
    printf 'runner-dispatch: observe %s — still running (%sm of %sm budget)\n' \
      "$OBSERVE_KEY" "$ELAPSED_MIN" "${DELEGATION_OBSERVATION_BUDGET:-60}" >&2
    exit 4
  fi

  # 4. Budget exhausted: KILL THE WHOLE SESSION/GROUP, never a single pid. A single-PID kill
  #    reaps the launcher shell and ORPHANS the adapter and its children — precisely the
  #    half-dead state this change exists to eliminate. Honors change 0231: no presumed-dead
  #    worker wakes to race its replacement. Partial work stays in the worktree for a human.
  if [ -n "$LPGID" ]; then
    kill -TERM -"$LPGID" 2>/dev/null
    for _ in $(seq 1 20); do kill -0 -"$LPGID" 2>/dev/null || break; sleep 0.5; done
    kill -KILL -"$LPGID" 2>/dev/null
  fi
  printf 'killed_at=%s\nreason=budget-exhausted\nbudget_minutes=%s\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${DELEGATION_OBSERVATION_BUDGET:-60}" > "$DDIR/killed.partial"
  mv -f "$DDIR/killed.partial" "$DDIR/killed"
  printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (budget of %sm exhausted; the detached process group was terminated)\n' \
    "$OBSERVE_KEY" "${DELEGATION_OBSERVATION_BUDGET:-60}" >&2
  exit 1
fi
```

`--observe` needs `DELEGATION_OBSERVATION_BUDGET`. The facade does not currently resolve config. Take it from the environment when set (as the test does) and otherwise resolve it once, near the top of the observe branch:

```bash
  if [ -z "${DELEGATION_OBSERVATION_BUDGET:-}" ]; then
    _cfg="$("$DOCKET_BASH_PATH" "$SELF_DIR/docket-config.sh" --export 2>/dev/null)" \
      && eval "$(grep -E '^DELEGATION_OBSERVATION_BUDGET=' <<<"$_cfg")"
    DELEGATION_OBSERVATION_BUDGET="${DELEGATION_OBSERVATION_BUDGET:-60}"
  fi
```

- [ ] **Step 4: Add the `--iso-to-epoch` helper to `verify-run.sh`**

The observe branch above needs a portable ISO→epoch conversion, and `verify-run.sh` already owns that on the shared `iso_to_epoch` (via `lib/docket-frontmatter.sh`) — reuse it rather than growing a second portable timestamp parse. Add to `verify-run.sh`'s parser:

```bash
    --iso-to-epoch) MODE="iso"; ISO_IN="${2:-}"; shift ;;
```

initialise `ISO_IN=""`, and add, immediately after the frontmatter library is sourced:

```bash
# A tiny exported utility so runner-dispatch.sh does not grow a SECOND portable ISO->epoch parse
# (this script is already the single owner of that conversion for the run gate). Pure: prints an
# integer or nothing, writes nothing.
if [ "$MODE" = "iso" ]; then
  [ -n "$ISO_IN" ] || die "--iso-to-epoch requires a timestamp"
  iso_to_epoch "$ISO_IN"
  exit 0
fi
```

Note that `--iso-to-epoch` must be reachable **without** the changes-dir resolution, so place this block above that resolver — the same placement rule Task 2's build family follows.

- [ ] **Step 5: Run test to verify it passes**

Run: `bash tests/test_runner_dispatch_detach.sh`
Expected: every assert `ok`.

- [ ] **Step 6: Mutation-test the group kill**

Change `kill -TERM -"$LPGID"` to `kill -TERM "$LPGID"` (dropping the negation, so only the launcher pid is signalled) and re-run. Expect **"the detached process group was killed"** and **"the adapter never completed its work"** to go red — the adapter survives as an orphan. This mutation is the exact orphan hazard the spec names, so a green assert here would prove the guard decorative. Restore from your own backup copy.

- [ ] **Step 7: Document the exit codes**

In `scripts/runner-dispatch.md`, add an `### Observation` subsection and extend the Exit codes section with the four `--observe` codes in a table, stating explicitly that **`4` is not a failure** — it means observe again — that its **only** consumer is the generated shim, and that the shim was changed in the same change to loop on it. Record the kill-on-giving-up policy and the idempotence guarantee.

- [ ] **Step 8: Commit**

```bash
git add scripts/runner-dispatch.sh scripts/verify-run.sh scripts/runner-dispatch.md \
        tests/test_runner_dispatch_detach.sh
git commit -m "feat(0271): add --observe with a bounded budget, group kill, and synthesized exits"
```

---

### Task 5: widen the run gate past `implement-next`

**Files:**
- Modify: `scripts/runner-dispatch.sh` (the `GATE=` fence and the observe branch's git leg)
- Modify: `scripts/runner-dispatch.md`
- Test: `tests/test_runner_dispatch_detach.sh`

**Interfaces:**
- Consumes: Task 2's build verdicts; Task 4's observe branch.
- Produces: a terminal `--observe` disposition that combines the sentinel with a git verdict — for `implement-next` via `verify-run <id>`, and for `build-*` via `verify-run --build`.

**The disagreement rule** is the deliverable: a sentinel claiming success (`exit_code=0`) with no matching git evidence is a **failure**. Correctness wins over liveness. **No auto re-dispatch for `build-*`** — observe-only; a build task may have left partial commits, and re-running on top of them is the "never escalate onto a stray commit" hazard. `implement-next` keeps its existing one-re-dispatch policy and its 1/3/0 codes unchanged.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_runner_dispatch_detach.sh`:

```bash
# ---- the gate now covers build-*, via the BUILD verdict family ------------------
mkbuildrepo(){   # a repo + feature worktree the fake adapter can commit into
  make_fixture
  git -C "$SBX" checkout -q -b feat/thing
  WT="$SBX"     # the fake adapter runs with --worktree $SBX
}

# (a) the child commits cleanly -> task-committed -> observe 0
mkbuildrepo
cat > "$RDIR/fake.sh" <<'FAKE'
#!/usr/bin/env bash
cd "$DOCKET_REPO_ROOT" || exit 1
git commit --allow-empty -qm "task work"
exit 0
FAKE
chmod +x "$RDIR/fake.sh"
KEY="$( ( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --launch --runner fake \
    --agent build-standard --worktree "$WT" ) )"
for _ in $(seq 1 30); do
  ( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --observe "$KEY" --runner fake \
      --agent build-standard --worktree "$WT" ) >/dev/null 2>&1
  [ "$?" != "4" ] && break; sleep 1
done
out="$( ( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --observe "$KEY" --runner fake \
    --agent build-standard --worktree "$WT" ) 2>&1 )"; rc=$?
assert "0271: a clean build task observes as complete (0)" '[ "$rc" = "0" ]'
assert "0271: the build gate reports task-committed" 'grep -qF "task-committed" <<<"$out"'

# (b) THE DISAGREEMENT RULE — the child exits 0 but strands uncommitted work.
#     This is change 0258's exact failure, and the sentinel alone would call it success.
mkbuildrepo
cat > "$RDIR/fake.sh" <<'FAKE'
#!/usr/bin/env bash
cd "$DOCKET_REPO_ROOT" || exit 1
printf 'stranded work\n' > stranded.txt   # never committed
exit 0
FAKE
chmod +x "$RDIR/fake.sh"
KEY="$( ( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --launch --runner fake \
    --agent build-standard --worktree "$WT" ) )"
for _ in $(seq 1 30); do
  ( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --observe "$KEY" --runner fake \
      --agent build-standard --worktree "$WT" ) >/dev/null 2>&1
  [ "$?" != "4" ] && break; sleep 1
done
out="$( ( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --observe "$KEY" --runner fake \
    --agent build-standard --worktree "$WT" ) 2>&1 )"; rc=$?
assert "0271: a sentinel-success with stranded work FAILS (correctness wins)" '[ "$rc" = "1" ]'
assert "0271: the disagreement diagnostic names the git verdict" 'grep -qF "task-incomplete" <<<"$out"'
assert "0271: the stranded file is still there for a human" '[ -f "$SBX/stranded.txt" ]'

# (c) build-* is OBSERVE-ONLY: never a second adapter run on top of partial commits.
assert "0271: no re-dispatch happened for a build agent" \
  '[ "$(git -C "$SBX" rev-list --count HEAD)" -le 2 ]'

# (d) a non-build, non-implement-next agent keeps the sentinel-only disposition
make_fixture
FAKE_SLEEP=0 FAKE_RC=0
KEY="$(launch status)"
for _ in $(seq 1 30); do observe "$KEY" >/dev/null 2>&1; [ "$?" != "4" ] && break; sleep 1; done
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "0271: a status agent still observes as complete on the sentinel alone" '[ "$rc" = "0" ]'
assert "0271: no build verdict is claimed for a status agent" '! grep -qF "task-committed" <<<"$out"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_runner_dispatch_detach.sh 2>&1 | grep "NOT OK"`
Expected: the disagreement asserts fail — today a clean sentinel exits 0 regardless of git.

- [ ] **Step 3: Wire the git leg into the observe branch**

In `scripts/runner-dispatch.sh`, replace the observe branch's `if [ "$SEC" = "0" ]` success leg with a git-consulting version:

```bash
    if [ "$SEC" = "0" ]; then
      # LIVENESS said done; now CORRECTNESS decides. A sentinel claiming success with no matching
      # git evidence is a FAILURE — the delegated run is the party being judged, so its own exit
      # code can never be the last word. Change 0258's stranded +64 lines exited 0 at the adapter.
      GITV=""
      case "$AGENT" in
        build-*)
          LSINCE="$(sed -n 's/^since_sha=//p' "$DDIR/launch" 2>/dev/null | sed -n 1p)"
          LBRANCH="$("$GIT" -C "$ANCHOR" rev-parse --abbrev-ref HEAD 2>/dev/null)"
          GITV="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" --build --worktree "$ANCHOR" \
                    --branch "${LBRANCH:-HEAD}" --since "${LSINCE:-}" 2>/dev/null)"
          case "$GITV" in
            task-committed*) : ;;
            # OBSERVE-ONLY for build-*: never a re-dispatch. A build task may have left partial
            # commits, and re-running on top of them is docket-build's "never escalate onto a
            # stray commit" hazard. Report and stop.
            *) printf 'runner-dispatch: observe %s — FAILED (the child exited 0 but git disagrees: %s); work left in %s for inspection\n' \
                 "$OBSERVE_KEY" "${GITV:-no-verdict}" "$ANCHOR" >&2
               exit 1 ;;
          esac ;;
      esac
      printf 'runner-dispatch: observe %s — complete (child exited 0%s)\n' \
        "$OBSERVE_KEY" "${GITV:+; $GITV}" >&2
      exit 0
    fi
```

Then widen the legacy `GATE=` fence so the comment stops asserting a scope that is no longer true:

```bash
# The synchronous path's run gate stays scoped to implement-next (change 0237). The BUILD
# disposition is NOT bolted on here: it lives on the --observe seam (change 0271), where a
# detached child's terminal state is actually knowable. A build-* delegation leaves its change
# `in-progress` by design, which is why this fence never grew a build leg.
GATE=0; [ "$AGENT" = "implement-next" ] && GATE=1
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_runner_dispatch_detach.sh`
Expected: every assert `ok`.

- [ ] **Step 5: Verify the implement-next path is untouched**

Run: `bash tests/test_runner_dispatch.sh`
Expected: every assert `ok` — the synchronous gate, its one re-dispatch, and its 1/3/0 codes are unchanged. If anything reddens here, the widening leaked into the legacy path and must be pulled back out.

- [ ] **Step 6: Mutation-test the disagreement rule**

Delete the `build-*` case's `*)` failure leg (so a bad git verdict falls through to success) and re-run. Expect **"a sentinel-success with stranded work FAILS"** to go red. Restore from your own backup copy.

- [ ] **Step 7: Document the widening**

In `scripts/runner-dispatch.md`, extend the run-gate section: the observe seam now yields a disposition for `build-*` as well; the disagreement rule stated verbatim; `build-*` is **observe-only, never re-dispatched**, with the stray-commit reason; and `task-committed` proves clean completion, **not** semantic success.

- [ ] **Step 8: Commit**

```bash
git add scripts/runner-dispatch.sh scripts/runner-dispatch.md tests/test_runner_dispatch_detach.sh
git commit -m "feat(0271): give build-* a durable disposition on the observe seam"
```

---

### Task 6: `emit_shim` — launch-then-observe

**Files:**
- Modify: `sync-agents.sh` (`emit_shim`, the `SHIM` heredoc only)
- Modify: `tests/test_sync_agents_runners.sh`
- Test: `tests/test_sync_agents.sh`, `tests/test_sync_agents_run_gate.sh` (regression only)

**Interfaces:**
- Consumes: Tasks 3–5's `--launch` / `--observe` verbs and exit codes.
- Produces: the generated shim body. The baked flag string (`--runner … --agent … [--model …] [--effort …]`), the `--worktree` slot, and the `wt_rule` block stay **byte-stable**; only the instruction prose changes.

**Scope warning — read before grepping.** The phrase "never background it and never poll" also appears in `cursor-rules/run-gate.md`, `cursor-rules/dispatch.head.md`, `AGENTS.md`, and is asserted by `tests/test_sync_agents_run_gate.sh`. Those are the **caller-side native subagent dispatch** (change 0242) — a different boundary that legitimately keeps that language. **Do not touch them, and do not write a repo-wide ban.** The only site in scope is `emit_shim`'s `SHIM` heredoc.

- [ ] **Step 1: Write the failing test**

In `tests/test_sync_agents_runners.sh`, **replace** this line (the restatement guard, relocated rather than restored per `LEARNINGS: restatement-accumulates-its-own-guards`):

```bash
assert "0079: shim body demands ONE foreground call" 'grep -qi "one foreground" "$G"'
```

with:

```bash
# change 0271: the shim no longer bounds the whole delegated run inside one foreground call —
# that ceiling (600000 ms, the Bash tool's maximum and not a tunable) is the defect. The
# instruction is now launch-then-observe. This assert REPLACES 0079's "one foreground call"
# guard: the old one is not restored, because keeping it green would reinstate the defect.
assert "0271: shim bakes the --launch verb" 'grep -qF -- "--launch" "$G"'
assert "0271: shim bakes the --observe verb" 'grep -qF -- "--observe" "$G"'
# DETECTS THE REMOVAL, not the addition (LEARNINGS: assert-detects-removal-not-replacement).
assert "0271: shim no longer forbids backgrounding the dispatch" \
  '! grep -qi "never background it" "$G"'
assert "0271: shim no longer demands ONE foreground call" \
  '! grep -qi "one foreground" "$G"'
assert "0271: shim no longer bakes the 600000 ceiling" '! grep -qF "600000" "$G"'
# Exit 4 is NOT a failure, and this shim is its ONLY consumer — so the wrapper must name it
# explicitly rather than inheriting the bare-non-zero rule
# (LEARNINGS: exit-code-encodes-a-non-failure).
assert "0271: shim names exit 4 as the observe-again code" 'grep -qE "\b4\b" "$G"'
assert "0271: shim still forbids the inline fallback" 'grep -qiE "never.*inline" "$G"'
assert "0271: shim still forbids a silent retry" 'grep -qiE "never retry silently" "$G"'
# ADR-0038's chokepoint: still exactly ONE dispatch seam, even with two verbs.
assert "0271: both verbs go through the one facade" \
  '[ "$(grep -cF "docket.sh runner-dispatch" "$G")" = "2" ]'
```

Also update the pre-existing seam-count assert further down the same file, which pins one invocation:

```bash
assert "0079: exactly one dispatch invocation in the shim" '[ "$(grep -cF "docket.sh runner-dispatch" "$G")" = "1" ]'
```

becomes:

```bash
# 0271: two INVOCATIONS (launch + observe), still exactly ONE dispatch SEAM — both are the same
# facade. ADR-0038's chokepoint property is about the seam, not the call count.
assert "0271: exactly two dispatch invocations in the shim" \
  '[ "$(grep -cF "docket.sh runner-dispatch" "$G")" = "2" ]'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_sync_agents_runners.sh 2>&1 | grep -E "0271-|NOT OK" | head -20`
Expected: the new `0271` asserts fail against the current shim.

- [ ] **Step 3: Rewrite the heredoc**

In `sync-agents.sh`'s `emit_shim`, replace the `cat <<SHIM … SHIM` body with:

```bash
  cat <<SHIM
This agent is DELEGATED to the \`$4\` runner (cross-harness runner delegation, change 0079).
Do NOT execute the skill inline and do NOT load its skills yourself.

The delegated run MAY OUTLIVE the call that starts it, so this is a launch-then-observe
dispatch, not one blocking call (change 0271). Both steps go through the same facade — one
dispatch seam, no inline fallback, no silent retry.

STEP 1 — launch. Make ONE foreground Bash call:

    "\${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh runner-dispatch --launch $flags$wt_slot [-- <caller args>]

appending any caller-supplied task arguments after \`--\` (drop the brackets; omit entirely
when there are none). It detaches the child and returns immediately, printing a DISPATCH KEY
on stdout. A non-zero exit here is a failed launch: abort-and-report its stderr diagnostic.

STEP 2 — observe. Using that key, make repeated SHORT foreground Bash calls:

    "\${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh runner-dispatch --observe <key> $flags$wt_slot

Read the EXIT CODE, not the prose:
  - 4 — still running. This is NOT a failure. Observe again; keep going until another code.
  - 0 — the run completed. Relay the child's final message as your result, and verify its
        contract exactly as a native caller would: git state on origin/docket for
        state-contract agents (status, adr); the relayed report for in-context-report agents.
  - 3 — the run halted and needs a human. Abort-and-report its stderr diagnostic.
  - any other non-zero — the run failed or its result is unavailable. Abort-and-report its
        stderr diagnostic.

Block on each observe call and never yield between them — never background the dispatch and
never hand control back to your caller mid-run. The facade owns the observation budget and
stops on its own; never retry a failed dispatch silently, and never run the skill inline on
this harness as a fallback.
SHIM
```

The `flags`, `wt_slot`, and `wt_rule` computations above the heredoc are **unchanged**.

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_sync_agents_runners.sh`
Expected: every assert `ok`, including the untouched 0079/0206/0269 asserts (baked flags, `--worktree` slot, frontmatter pins).

- [ ] **Step 5: Verify non-build shims stayed byte-stable in shape**

Run: `bash tests/test_sync_agents.sh && bash tests/test_sync_agents_run_gate.sh && bash tests/test_sync_agents_codex_dispatch.sh && bash tests/test_sync_agents_opencode.sh`
Expected: all `ok`. `test_sync_agents_run_gate.sh` in particular must stay green — it guards the caller-side gate's own "never background it" language, which this change deliberately does **not** touch. A red there means the edit escaped `emit_shim`.

- [ ] **Step 6: Mutation-test the removal assert**

Re-add the sentence `Block until it completes — never background it, never poll.` to the heredoc, re-run `tests/test_sync_agents_runners.sh`, and confirm **"shim no longer forbids backgrounding the dispatch"** goes red. A guard that only confirms the new wording would detect nothing; this proves it detects the state that was removed. Restore from your own backup copy.

- [ ] **Step 7: Regenerate this repo's own wrappers and eyeball one**

```bash
bash sync-agents.sh
sed -n '1,60p' .claude/agents/docket-build-standard.md
```
Expected: frontmatter pin intact; `--launch` and `--observe` both present; the `--worktree <feature worktree>` slot and its rule block intact; no `600000`.

- [ ] **Step 8: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents_runners.sh
git commit -m "feat(0271): rewrite emit_shim to launch-then-observe, retiring the 600000 ceiling"
```

---

### Task 7: the delegation execution posture and its per-harness verdicts

**Files:**
- Create: `skills/docket-build/references/delegation-execution.md`
- Modify: `scripts/runner-dispatch.md` (the posture section)
- Test: `tests/test_runner_dispatch_detach.sh` (prose guards)

**Interfaces:**
- Consumes: everything above.
- Produces: the normative posture, and the per-harness verdict table for the **adapter** launch shape.

**Verdicts must be honest.** `gate-execution.md`'s verdicts were measured for a **gate** launch (a test command) and are explicitly version- and scope-scoped, so they **do not transfer** to an adapter launch. An autonomous build cannot install and authenticate the codex / cursor / opencode CLIs, so **all four harness rows ship as `unverified`** with the probe recipe recorded — `unverified` means "treat as unknown, never as working", which is exactly the state of knowledge. The **mechanism** (process-group survival) *was* measured hermetically and that measurement is recorded separately from the per-harness rows. Flag the re-probe as a human verify item in the results file.

- [ ] **Step 1: Write the failing prose guards**

Append to `tests/test_runner_dispatch_detach.sh`:

```bash
# ---- the posture cites gate-execution.md, never restates the six capabilities ----
DOC="$ROOT/scripts/runner-dispatch.md"
DEL="$ROOT/skills/docket-build/references/delegation-execution.md"
assert "0271: the delegation verdicts reference exists" '[ -f "$DEL" ]'
assert "0271: runner-dispatch.md states a delegation execution posture" \
  'grep -qi "delegation execution posture" "$DOC"'
# CITES rather than RESTATES (the change-0154 discipline): the posture must point at the
# quarantine file, and must NOT grow its own copy of the numbered capability list.
assert "0271: the posture cites gate-execution.md" 'grep -qF "gate-execution.md" "$DOC"'
assert "0271: the posture does not restate the six capabilities" \
  '[ "$(grep -ciE "six (required )?capabilities" "$DOC")" -le 1 ]'
assert "0271: the posture says a delegated run may outlive its launching call" \
  'grep -qiE "outlive (the|its) call" "$DOC"'

# Per-harness rows are DERIVED from the shipped roster, never hand-listed — and the population
# is floored so an extractor returning nothing cannot read as parity holding
# (LEARNINGS: marker-scoped-guard-needs-a-population-floor).
SHIPPED="$(sed -n 's/^HD_SHIPPED_HARNESSES="\([^"]*\)".*/\1/p' "$ROOT/sync-agents.sh" | sed -n 1p)"
n_h=0
for h in $SHIPPED; do
  assert "0271: delegation verdicts cover harness '$h'" 'grep -qiF "'"$h"'" "$DEL"'
  n_h=$((n_h+1))
done
assert "0271: the harness loop actually enumerated the roster (got $n_h)" '[ "$n_h" -ge 4 ]'
# Honesty: an unmeasured adapter launch shape must read `unverified`, never inherit the gate's
# `supported`. gate-execution.md's own rule — a verdict is version- and scope-scoped.
assert "0271: unmeasured adapter launches are recorded as unverified" 'grep -qF "unverified" "$DEL"'
assert "0271: the reference says the gate verdicts do NOT transfer" \
  'grep -qiE "do(es)? not transfer|never inherit" "$DEL"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_runner_dispatch_detach.sh 2>&1 | grep "NOT OK"`
Expected: the posture asserts fail — neither the section nor the reference file exists.

- [ ] **Step 3: Write the verdicts reference**

Create `skills/docket-build/references/delegation-execution.md`:

```markdown
# Delegation execution — required capabilities and per-harness evidence

Reference for `scripts/runner-dispatch.md` § *Delegation execution posture*. The **six required
capabilities are defined once**, in
[`gate-execution.md`](gate-execution.md) — this file does not restate them. What it adds is the
evidence for a *different launch shape*: the **adapter** launch that starts a whole delegated agent
run, rather than the **gate** launch that starts a test command.

## Why the gate verdicts do not transfer

`gate-execution.md`'s verdicts are explicitly **version- and scope-scoped**, and every one names the
shape it measured — a gate launch. A delegated adapter launch differs in duration, in what the child
does to the repo, and in which CLI subcommand is invoked. A verdict measured for one is **not**
evidence for the other, so these rows are re-derived rather than inherited. Never inherit a row on
faith.

Read a verdict exactly as `gate-execution.md` defines it: `supported` — measured, with evidence and
version recorded. `unverified` — not measured, or measured inconclusively; **treat as unknown, never
as working.** `incompatible` — measured and established as unable to meet a required capability.
A probe that changes two variables at once proves nothing about either.

## The mechanism — measured hermetically

Independent of any harness, the facade's detachment mechanism was measured on **macOS (darwin
25.6.0), GNU Bash 5.x**, 2026-08-09:

- `setsid` is **not present** on macOS, and docket takes no `perl` dependency. Detachment therefore
  uses Bash **job control**: under `set -m`, a background job becomes a **process-group leader**.
- One run, two arms, one variable changed: a launcher started two children, one under `set -m` and
  one not, then the launcher's whole **process group** received `TERM`. The `set -m` child (own
  PGID) **survived**; the non-`set -m` child (launcher's PGID) was **killed**.
- That is capability 1's stronger reading — survival of the teardown of the initiating call's
  process group, not merely its parent's exit. `tests/test_runner_dispatch_detach.sh` pins it with
  a fake adapter, and the assert is mutation-tested by removing `set -m`.

This measures the **facade**, not any harness. It says nothing about whether a given child CLI
tolerates being started that way.

## Per-harness verdicts — the adapter launch shape

Every row below is **`unverified`**: the adapter launch shape has not been measured on any harness.
Re-probing requires each CLI installed and authenticated, which the change that introduced this file
could not do. This is a recorded gap, not an implied pass.

| Harness | Adapter launch shape | Verdict |
|---|---|---|
| claude | `docket.sh runner-dispatch --launch` → `runners/claude`-family adapter | `unverified` — adapter launch shape unmeasured |
| cursor | `cursor … --print --force --sandbox disabled` | `unverified` — adapter launch shape unmeasured |
| codex | `codex exec --skip-git-repo-check --sandbox danger-full-access` | `unverified` — adapter launch shape unmeasured |
| opencode | `opencode run --dir <anchor>` (+ `--auto` under `permissions: auto-approve`) | `unverified` — adapter launch shape unmeasured |

## Probe recipe

For each harness, with its CLI installed and authenticated, changing **one** variable per run:

1. Launch a delegated agent whose task is deliberately longer than the parent harness's foreground
   ceiling: `docket.sh runner-dispatch --launch --runner <h> --agent status`.
2. Confirm the call returns in seconds with a dispatch key, and that
   `<git-common-dir>/docket/dispatch/<key>/launch` records a `pgid` different from the launching
   shell's.
3. Tear down the launching call's **process group** (`kill -TERM -<launcher pgid>`), not just its
   pid. Isolate the launcher into its own group first, or the probe kills the harness running it.
4. Wait past the child's expected duration, then check for `<key>/done` and a non-empty
   `<key>/stdout.log`.
5. Sentinel present with a well-formed `exit_code` ⇒ capabilities 1–3 hold for this shape; record
   `supported` with the CLI version. Sentinel absent ⇒ **inconclusive**, not `incompatible`, unless
   the run establishes the child was killed by the teardown.

Record the version in the row. Re-probe when it moves.
```

- [ ] **Step 4: Write the posture into the contract**

In `scripts/runner-dispatch.md`, add before `## Exit codes`:

```markdown
## Delegation execution posture

A delegated run **may outlive the call that launched it**. That is the contract, not an
implementation detail: the parent harness's foreground-call ceiling does not bound a delegated
agent run, and nothing on this path may re-introduce a bound that it does.

The required capabilities are the same six the build gate needs, defined once in
`skills/docket-build/references/gate-execution.md` — **read them there; this contract does not
restate them.** What is specific here is the division of labour:

- **The shim launches and observes.** It makes one `--launch` call and then bounded, short
  `--observe` calls. It never blocks for the child's duration and never yields between
  observations (ADR-0024 unamended: a dispatched child observes by *blocking* on short calls; only
  a top-level session agent may background-and-await).
- **The facade owns detachment, observation, and disposition.** It starts the adapter in its own
  process group with every stream redirected to a durable per-dispatch directory, records a
  sentinel as the launcher's last act, bounds observation by
  `delegation_observation_budget`, and kills the whole detached group before reporting a run
  unavailable.
- **The agent owns none of it.** The delegated agent has no sentinel obligation and no knowledge of
  the result directory — a sentinel written by the party being judged would make "done" a claim
  rather than evidence.

ADR-0038's chokepoint property is unchanged: two verbs, still exactly **one** dispatch seam, still
no inline fallback and no silent retry.

Evidence for the **adapter** launch shape — which is a different shape from the gate launch and
therefore does not inherit `gate-execution.md`'s verdicts — is in
`skills/docket-build/references/delegation-execution.md`.
```

- [ ] **Step 5: Run test to verify it passes**

Run: `bash tests/test_runner_dispatch_detach.sh`
Expected: every assert `ok`.

- [ ] **Step 6: Mutation-test the citation guard**

Paste the six-capability numbered list from `gate-execution.md` into `runner-dispatch.md`, re-run, and confirm **"the posture does not restate the six capabilities"** goes red. That guard exists to stop exactly the restatement class change 0154 tracks. Restore from your own backup copy.

- [ ] **Step 7: Run the whole suite**

Run: `scripts/run-tests.sh`
Expected: every file passes. Act on any trailing `OVER BUDGET:` line — it does not fail the run, so nothing else will catch it. If `tests/test_runner_dispatch_detach.sh` is over its 60s row, either raise the row with a comment explaining the sleeps or shorten the fake adapter's durations; do not pass `--no-budget-check`.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-build/references/delegation-execution.md scripts/runner-dispatch.md \
        tests/test_runner_dispatch_detach.sh
git commit -m "docs(0271): state the delegation execution posture, citing the gate's capabilities"
```

---

## Self-review

**Spec coverage.**

| Spec decision | Task |
|---|---|
| 1. Delegation execution posture (prose, cites not restates) | 7 |
| 2. Detachment owned by the facade; re-probe harnesses | 3 (mechanism, measured); 7 (per-harness rows, honestly `unverified`) |
| 3. Done-file sentinel; wrapper writes it; disagreement rule | 3 (sentinel); 5 (disagreement) |
| 4. Watcher loop, budget, orphan kill, idempotence, no resumability | 1 (key); 4 (loop, kill, idempotence) |
| 5. Run gate widened to `build-*`; second verdict family; `task-committed` naming; no auto re-dispatch | 2 (family); 5 (widening, observe-only) |
| 6. Synthesized exit codes | 4 (table + the `4` consumer changed in Task 6) |
| 7. Per-dispatch result key (agent + mint, docket-owned durable dir) | 3 |
| 8. `emit_shim` launch-then-observe rewrite | 6 |
| 9. Sequencing on 0269 | Discharged at reconcile — 0269 is `done` and merged; the branch is cut from `origin/main`, so there is no rebase step |

**Spec guards, all placed:** wrapper no longer contains the foreground instruction (6); build shim flags unchanged / non-build bytes stable (6); run gate fires for a `build-*` agent (5); each new `verify-run` verdict produced by a fixture (2); sentinel disagreement rule fixture (5); malformed/absent sentinel with a dead session yields `result unavailable` (4); observe idempotence (4); per-harness population from `HD_SHIPPED_HARNESSES` with a non-vacuity floor (7).

**Placeholder scan:** every code step carries real code; no "TBD", no "handle edge cases", no "similar to Task N".

**Type consistency:** the verdict tokens (`task-committed` / `task-incomplete` / `task-unverifiable`, unmet order `branch tip tree`) are defined in Task 2 and consumed verbatim in Task 5. The exit codes (0/1/3/4) are defined in Task 4 and consumed verbatim in Task 6. The dispatch-dir function names and the `launch` / `done` / `killed` file names are defined in Task 3 and consumed verbatim in Tasks 4 and 5. `DELEGATION_OBSERVATION_BUDGET` is defined in Task 1 and read in Task 4.

**Two risks worth naming for the reviewer.**

1. **`--observe` resolves config on a live path.** Task 4 shells out to `docket-config.sh --export` when the variable is unset. The facade's standing posture on live dispatch paths is *tolerant*, so this degrades to the `60` default rather than dying — matching how an unparseable `runners.<name>:` value is skipped rather than fatal.
2. **The per-harness verdicts ship `unverified`.** That is honest, not a gap papered over, but it means this change lands the mechanism without proving any child CLI tolerates it. Task 7 records the probe recipe and it belongs in the results file as a human verify item.
