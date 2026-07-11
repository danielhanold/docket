# docket-status orchestrator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse the entire deterministic `docket-status` pipeline into one `scripts/docket-status.sh` invocation that emits a single compact machine-parseable report, then rewrite the `docket-status` skill around it so a status pass costs 2–5 model turns instead of 10–35+.

**Architecture:** A new orchestrator script sequences the already-existing, jointly-owned docket scripts (`docket-config.sh`, `render-board.sh`, `github-mirror.sh`, `archive-change.sh`, `render-change-links.sh`, `terminal-publish.sh`, `cleanup-feature-branch.sh`, `board-checks.sh`, `sync-integration-branch.sh`) in one process. It duplicates none of their mechanics — it only orders them, applies each surface's failure posture (board best-effort, sweep log-and-continue, checks warn-only), and translates their output into a line-oriented report on stdout. The model reads that one report and does only the judgment follow-ups (harvest-learnings, `blocked_by:` review, mint write-backs, human summary). A new ADR (relates to ADR-0012) legitimizes the script authoring formulaic templated commit messages and mutating state along the blessed sequence.

**Tech Stack:** Bash (same style as the existing `scripts/*.sh`), `git`, `gh` (GitHub CLI, mocked in tests), the shared `scripts/lib/docket-frontmatter.sh` helper. Hermetic bash tests under `tests/` following the `test_render_board.sh` pattern.

## Global Constraints

- Language & style: POSIX-ish Bash matching the existing scripts — `set -uo pipefail`, `emit(){ printf '%s=%q\n' … }`-style helpers, `GIT="${GIT:-git}"` mock seam. Add a parallel `GH="${GH:-gh}"` seam for every `gh` invocation so tests can mock it.
- The orchestrator NEVER duplicates a shared script's internals; it only invokes `"$SELF_DIR"/<script>.sh` (where `SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`).
- Determinism invariant (ADR-0035 ordering; ADR-0012 boundary): archive commits touch only the change file; UTC dates only (never `now()`/local time); concurrent runs converge byte-identically; two runs over the same change files produce identical `BOARD.md` bytes.
- Failure postures are surface-specific and MUST NOT be unified: **board** = best-effort (log + continue, push-rebase-retry, regenerate-never-3-way-merge on `BOARD.md` conflict); **sweep** = log-and-continue per change (a failed step abandons the rest of THAT change's close-out — a failed re-render skips publish — and moves to the next change); **health checks** = warn-only (never auto-fix). These mirror the current skill prose exactly.
- Exit codes: `0` = pass completed (findings/warnings allowed on stdout); non-zero = hard error ONLY (config/bootstrap failure, metadata worktree unusable). Stderr carries diagnostics; stdout stays machine-parseable.
- Output contract (stdout, line-oriented — this is the model's ONE tool result). Emit exactly these line shapes:
  ```
  board    inline   changed|clean   pushed|push-failed
  board    github   ok|skipped|failed
  minted   issue    <change-id> <issue-number>
  minted   project  <owner> <number>
  swept    <id>     <merge-date>
  sweep-failed <id> <step> <one-line reason>
  sweep-skipped <reason>
  check    <check-id>  <change-id>  <message>
  harvest  <id>    <archived-path>
  judgment blocked <id> <blocked_by text>
  ```
- Config comes from `eval "$("$SELF_DIR"/docket-config.sh --export)"`; the emitted vars this script consumes are: `DOCKET_MODE`, `METADATA_BRANCH`, `INTEGRATION_BRANCH`, `METADATA_WORKTREE`, `CHANGES_DIR`, `ADRS_DIR`, `RESULTS_DIR`, `BOARD_SURFACES`, `BOOTSTRAP`. `docket-config.sh --export` does NOT emit `github_project`; the github surface's project handling is driven entirely by the `--project`/`--auto-create-project`/`--project-owner` passthrough flags.
- `--board-only` runs steps 1–3 (config+bootstrap, worktree sync, board pass) and exits 0; it MUST skip sweep, health checks, and integration sync.
- Repo skill source lives at `skills/docket-status/SKILL.md` (NOT the installed `~/.claude` copy). Script + contract live at `scripts/docket-status.sh` + `scripts/docket-status.md`.

---

### Task 1: Orchestrator skeleton — arg parsing, config eval, bootstrap gate

**Files:**
- Create: `scripts/docket-status.sh`
- Create: `scripts/docket-status.md` (contract stub — Purpose/Usage/Behavior/Exit codes/Invariants headers, filled incrementally)
- Test: `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: `docket-config.sh --export` vars (see Global Constraints).
- Produces: an executable `scripts/docket-status.sh` accepting flags `--board-only`, `--repo OWNER/REPO`, `--project OWNER/NUMBER`, `--auto-create-project`, `--project-owner OWNER`, `--help`; sets `SELF_DIR`, `GIT="${GIT:-git}"`, `GH="${GH:-gh}"`; a function `main()` invoked at end.

- [ ] **Step 1: Write the failing test** (`tests/test_docket_status.sh`)

```bash
#!/usr/bin/env bash
# tests/test_docket_status.sh — verifies change 0058: the docket-status orchestrator.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/docket-status.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "script exists and is executable" '[ -x "$SCRIPT" ]'
assert "--help exits 0 and prints usage" '"$SCRIPT" --help 2>&1 | grep -qi "usage"'

# Bootstrap gate: a STOP_MIGRATE verdict must exit non-zero and print the migrate remedy.
# Mock docket-config.sh by shadowing it on PATH-independent SELF_DIR is not possible;
# instead assert the gate logic via a stubbed config export (see Step 3 mock seam).

exit $fail
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_status.sh`
Expected: FAIL — "script exists and is executable" NOT OK (file absent).

- [ ] **Step 3: Write minimal implementation**

Create `scripts/docket-status.sh`:

```bash
#!/usr/bin/env bash
# scripts/docket-status.sh — deterministic orchestrator for the docket-status pass (change 0058).
# Sequences the shared docket scripts in one process; emits one line-oriented report on stdout.
# Contract: scripts/docket-status.md.
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GIT="${GIT:-git}"
GH="${GH:-gh}"

BOARD_ONLY=0 REPO_FLAG="" PROJECT_FLAG="" AUTO_CREATE_PROJECT=0 PROJECT_OWNER=""
usage(){ sed -n '2,12p' "${BASH_SOURCE[0]}"; }
while [ $# -gt 0 ]; do
  case "$1" in
    --board-only) BOARD_ONLY=1 ;;
    --repo) REPO_FLAG="$2"; shift ;;
    --project) PROJECT_FLAG="$2"; shift ;;
    --auto-create-project) AUTO_CREATE_PROJECT=1 ;;
    --project-owner) PROJECT_OWNER="$2"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "docket-status: unknown argument: $1" >&2; exit 2 ;;
  esac; shift
done

# Config export mock seam: CONFIG_EXPORT_CMD lets tests inject a stub export.
config_export(){ ${CONFIG_EXPORT_CMD:-"$SELF_DIR"/docket-config.sh --export}; }

main(){
  local cfg; cfg="$(config_export)" || { echo "docket-status: config export failed" >&2; exit 1; }
  eval "$cfg"
  case "${BOOTSTRAP:-}" in
    PROCEED) : ;;
    STOP_MIGRATE) echo "docket-status: repo not migrated — run migrate-to-docket.sh" >&2; exit 1 ;;
    CREATE_ORPHAN) echo "docket-status: fresh repo — bootstrap is opt-in; run a docket skill to create the docket branch" >&2; exit 1 ;;
    *) echo "docket-status: unknown bootstrap verdict '${BOOTSTRAP:-}'" >&2; exit 1 ;;
  esac
  # Steps 2..7 wired in later tasks.
}
main "$@"
```

Make it executable: `chmod +x scripts/docket-status.sh`.

Create `scripts/docket-status.md` with the section headers (Purpose, Usage, Behavior, Exit codes, Invariants) and a one-line stub under each; fill in later tasks.

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_docket_status.sh`
Expected: PASS — both assertions ok.

- [ ] **Step 5: Add bootstrap-gate assertions with a stubbed config export**

Extend the test to exercise the gate via `CONFIG_EXPORT_CMD`:

```bash
stub_cfg(){ printf 'echo %q' "BOOTSTRAP=$1
METADATA_BRANCH=docket
INTEGRATION_BRANCH=main
DOCKET_MODE=docket
METADATA_WORKTREE=.docket
CHANGES_DIR=docs/changes
ADRS_DIR=docs/adrs
RESULTS_DIR=docs/results
BOARD_SURFACES=inline"; }

CONFIG_EXPORT_CMD="$(stub_cfg STOP_MIGRATE)" bash "$SCRIPT" --board-only >/dev/null 2>err.txt
assert "STOP_MIGRATE exits non-zero" '[ $? -ne 0 ]'
assert "STOP_MIGRATE prints migrate remedy" 'grep -qi "migrate" err.txt'
CONFIG_EXPORT_CMD="$(stub_cfg CREATE_ORPHAN)" bash "$SCRIPT" --board-only >/dev/null 2>&1
assert "CREATE_ORPHAN exits non-zero" '[ $? -ne 0 ]'
```

Adjust the stub mechanism so `CONFIG_EXPORT_CMD` yields a command whose stdout is the KEY=value block (e.g. set `CONFIG_EXPORT_CMD="cat fixture-export.sh"` where the fixture holds `printf '%s\n' KEY=value…`, matching how `eval "$cfg"` consumes it). Run, verify PASS, then commit.

- [ ] **Step 6: Commit**

```bash
git add scripts/docket-status.sh scripts/docket-status.md tests/test_docket_status.sh
git commit -m "feat(0058): docket-status orchestrator skeleton — args, config eval, bootstrap gate"
```

---

### Task 2: Metadata worktree ensure + sync

**Files:**
- Modify: `scripts/docket-status.sh` (add `ensure_and_sync_worktree`)
- Test: `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: `DOCKET_MODE`, `METADATA_BRANCH`, `METADATA_WORKTREE`.
- Produces: function `ensure_and_sync_worktree()` — in `docket`-mode ensures `$METADATA_WORKTREE` (default `.docket`) exists (idempotent `git worktree add` if missing), then `fetch` + `pull --rebase origin "$METADATA_BRANCH"`; in `main`-mode degrades to `pull --rebase` on the primary tree. A hard failure (worktree unusable) exits non-zero.

- [ ] **Step 1: Write the failing test** — assert that in a temp `main`-mode repo the sync step is a no-op-safe pull that exits 0, and that a missing metadata worktree in `docket`-mode is created. Use a throwaway git repo fixture (init, commit, branch) and the `GIT` seam. Expected FAIL (function absent).

- [ ] **Step 2: Run test to verify it fails** — `bash tests/test_docket_status.sh` → NOT OK for the new sync assertions.

- [ ] **Step 3: Implement `ensure_and_sync_worktree`** and call it from `main()` after the bootstrap gate:

```bash
ensure_and_sync_worktree(){
  if [ "${DOCKET_MODE:-}" = docket ]; then
    local wt="${METADATA_WORKTREE:-.docket}"
    if [ ! -d "$wt" ]; then
      "$GIT" worktree add "$wt" "$METADATA_BRANCH" >&2 2>/dev/null \
        || "$GIT" worktree add "$wt" "origin/$METADATA_BRANCH" >&2 \
        || { echo "docket-status: cannot create metadata worktree $wt" >&2; exit 1; }
    fi
    "$GIT" -C "$wt" fetch origin "$METADATA_BRANCH" >&2 \
      && "$GIT" -C "$wt" pull --rebase origin "$METADATA_BRANCH" >&2 \
      || { echo "docket-status: metadata worktree sync failed" >&2; exit 1; }
  else
    "$GIT" pull --rebase >&2 || { echo "docket-status: metadata sync failed" >&2; exit 1; }
  fi
}
```

- [ ] **Step 4: Run test to verify it passes** — `bash tests/test_docket_status.sh` → all ok.

- [ ] **Step 5: Commit**

```bash
git add scripts/docket-status.sh tests/test_docket_status.sh
git commit -m "feat(0058): orchestrator metadata worktree ensure + sync"
```

---

### Task 3: Board pass — inline render + push-rebase-retry, github passthrough

**Files:**
- Modify: `scripts/docket-status.sh` (add `board_pass`)
- Test: `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: `BOARD_SURFACES`, `CHANGES_DIR`, `ADRS_DIR`, `METADATA_WORKTREE`, `METADATA_BRANCH`, the `--repo`/`--project`/`--auto-create-project`/`--project-owner` flags, `render-board.sh`, `github-mirror.sh`.
- Produces: function `board_pass()` emitting `board inline changed|clean pushed|push-failed`, `board github ok|skipped|failed`, and pass-through `minted issue <id> <n>` / `minted project <owner> <n>` lines. `board_surfaces` empty ⇒ no-op (emit nothing). Unknown token ⇒ warn on stderr, ignore.

- [ ] **Step 1: Write the failing test** — build a temp metadata tree with a `changes/active` fixture (reuse the shape from `test_render_board.sh`). Assert:
  - clean tree (BOARD.md already current) ⇒ stdout contains `board inline clean`.
  - a changed tree ⇒ `board inline changed` and (with the `GIT` seam pointed at a local bare remote) `pushed`.
  - `BOARD_SURFACES=""` ⇒ no `board` line emitted.
  Expected FAIL.

- [ ] **Step 2: Run to verify fail.**

- [ ] **Step 3: Implement `board_pass`.** Iterate `$BOARD_SURFACES` tokens. Resolve the metadata tree root `MW` (`$METADATA_WORKTREE` in docket-mode, `.` in main-mode) and `CD="$MW/$CHANGES_DIR"`.
  - `inline`: derive `--repo` (from `$REPO_FLAG` else omit), run `"$SELF_DIR"/render-board.sh --changes-dir "$CD" ${REPO_FLAG:+--repo "$REPO_FLAG"} > "$CD/BOARD.md.tmp"`; if it exits non-zero, emit `board inline failed` on stderr + keep the old BOARD.md (do NOT truncate — write to `.tmp` then `mv`), and skip commit. Compare `.tmp` to `BOARD.md`; identical ⇒ `rm .tmp; echo "board inline clean"`. Differ ⇒ `mv .tmp BOARD.md`, `git -C "$MW" add`, commit with the fixed template `docket: board refresh`, then the push-rebase-retry loop: on push rejection `git -C "$MW" pull --rebase`; if the rebase conflicts on `BOARD.md`, **re-run render-board.sh into BOARD.md, `git add`, `git rebase --continue`** (regenerate, never 3-way merge); bounded retries (e.g. 5); emit `board inline changed pushed` or `board inline changed push-failed`.
    > **LEARNINGS #57/#51 guard:** never `> BOARD.md` directly — a failed render must not truncate the committed board. Render to `.tmp`, verify non-empty + exit 0, then `mv`.
  - `github`: run `"$SELF_DIR"/github-mirror.sh --changes-dir "$CD" ${REPO_FLAG:+--repo "$REPO_FLAG"} ${PROJECT_FLAG:+--project "$PROJECT_FLAG"} $([ "$AUTO_CREATE_PROJECT" = 1 ] && echo --auto-create-project) ${PROJECT_OWNER:+--project-owner "$PROJECT_OWNER"}` best-effort (never exit non-zero); grep its stdout for `issue-minted`/`project-minted` and re-emit as `minted issue …`/`minted project …`; emit `board github ok|failed`.
  - unknown token: `echo "docket-status: unknown board surface '$tok'" >&2`.

- [ ] **Step 4: Run to verify pass.**

- [ ] **Step 5: Commit** — `git commit -m "feat(0058): orchestrator board pass — inline render + push retry, github passthrough"`.

---

### Task 4: Sweep detection (batched gh)

**Files:**
- Modify: `scripts/docket-status.sh` (add `detect_merged`)
- Test: `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: `CHANGES_DIR`, `METADATA_WORKTREE`, `field()`/`list_field()` from `lib/docket-frontmatter.sh` (source it), the `GH` seam.
- Produces: function `detect_merged()` that prints TAB-separated `<id>\t<slug>\t<pr>\t<merged-date>` lines for every `implemented` change whose PR is merged, using ONE batched `gh` call. On any `gh`/network failure it emits `sweep-skipped <reason>` to stdout and returns without failing the pass.

- [ ] **Step 1: Write the failing test** — fixture with two `implemented` changes (one merged PR, one open) plus a `GH` mock script that returns a canned batched response; assert exactly the merged change is detected, and that a failing `GH` mock yields `sweep-skipped`. Expected FAIL.

- [ ] **Step 2: Run to verify fail.**

- [ ] **Step 3: Implement `detect_merged`.** Source `"$SELF_DIR"/lib/docket-frontmatter.sh`. Collect `implemented` change files under `$CD/active`. Build the batched query: prefer a single `gh api graphql` aliased query keyed on each change's `pr:` number; for changes with `pr:` unset, fall back to `gh pr list --head "feat/$slug" --state merged --json number,mergedAt`. Decide graphql-vs-N-calls at implementation time by testing `gh api graphql` ergonomics (spec Open Question) — either way it is ONE model turn since it is inside the script; prefer the single aliased graphql query if it parses cleanly. Wrap all `gh` calls with the `GH` seam and a failure trap ⇒ `echo "sweep-skipped gh-unavailable"`. Compute the merge date in UTC from `mergedAt` (`date -u -d` / `TZ=UTC`), never `now()`.

- [ ] **Step 4: Run to verify pass.**

- [ ] **Step 5: Commit** — `git commit -m "feat(0058): orchestrator batched sweep detection"`.

---

### Task 5: Sweep execution (chain shared close-out, log-and-continue)

**Files:**
- Modify: `scripts/docket-status.sh` (add `sweep_execute`)
- Test: `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: `detect_merged` output, `archive-change.sh`, `render-change-links.sh`, `terminal-publish.sh`, `cleanup-feature-branch.sh`, `INTEGRATION_BRANCH`, `METADATA_BRANCH`, `CHANGES_DIR`, `ADRS_DIR`.
- Produces: function `sweep_execute()` that, per merged change, runs the ADR-0035-guarded close-out order and emits `swept <id> <date>` on full success, `harvest <id> <archived-path>` (so the model harvests learnings), and `sweep-failed <id> <step> <reason>` on any per-change failure — abandoning the rest of THAT change's close-out and continuing to the next.

- [ ] **Step 1: Write the failing test** — with mocked shared scripts (shadow `archive-change.sh` etc. via a temp `SELF_DIR` override, or a `SCRIPTS_DIR` seam) assert: a clean merged change emits `swept` + `harvest`; a change whose `render-change-links.sh` mock exits non-zero emits `sweep-failed <id> render-change-links …` AND does NOT call `terminal-publish.sh` (failed re-render skips publish); the loop continues to the next change. Expected FAIL.
  > Add a `SCRIPTS_DIR="${SCRIPTS_DIR:-$SELF_DIR}"` seam so tests can point sub-script calls at mocks.

- [ ] **Step 2: Run to verify fail.**

- [ ] **Step 3: Implement `sweep_execute`.** For each detected merged change, in order:
  1. `git -C "$MW" pull --rebase`; re-read `status`; already `done`/archived ⇒ no-op continue (idempotent).
  2. `"$SCRIPTS_DIR"/archive-change.sh --changes-dir "$CD" --id "$id" --outcome done --date "$merged_date" --message "docket($id): done — archived (status done, $merged_date)"` — on non-zero: `echo "sweep-failed $id archive <reason>"`, continue.
  3. `"$SCRIPTS_DIR"/render-change-links.sh --change-file <archived path> --adrs-dir "$MW/$ADRS_DIR"` as a **separate follow-on commit**, pushed — on non-zero: `echo "sweep-failed $id render-change-links skipped-publish"`, continue (skip publish).
  4. `"$SCRIPTS_DIR"/terminal-publish.sh --id "$id" --outcome done --integration-branch "$INTEGRATION_BRANCH" --metadata-branch "$METADATA_BRANCH" --changes-dir "$CHANGES_DIR" --adrs-dir "$ADRS_DIR" --message "…"` — on non-zero: `echo "sweep-failed $id terminal-publish <reason>"`, continue.
  5. `"$SCRIPTS_DIR"/cleanup-feature-branch.sh --slug "$slug"` — on non-zero: `echo "sweep-failed $id cleanup <reason>"` (non-fatal), continue.
  6. On full success: `echo "swept $id $merged_date"` and `echo "harvest $id <archived-path>"`.
  Wrap the whole per-change body so one change's failure never aborts the loop.

- [ ] **Step 4: Run to verify pass.**

- [ ] **Step 5: Commit** — `git commit -m "feat(0058): orchestrator sweep execution — chained close-out, log-and-continue"`.

---

### Task 6: Health checks + judgment lines + integration sync

**Files:**
- Modify: `scripts/docket-status.sh` (add `health_checks`, `emit_judgment`, `integration_sync`; wire `main()`)
- Test: `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: `board-checks.sh`, `sync-integration-branch.sh`, change files (for `blocked` review lines).
- Produces: `health_checks()` — runs `board-checks.sh --changes-dir "$CD" --metadata-branch "$METADATA_BRANCH" --integration-branch "origin/$INTEGRATION_BRANCH"` and prefixes each TSV finding line as `check <check-id> <change-id> <message>`; `emit_judgment()` — one `judgment blocked <id> <blocked_by text>` per `blocked` change; `integration_sync()` — runs `sync-integration-branch.sh --integration-branch "$INTEGRATION_BRANCH"` once, best-effort, ONLY when ≥1 change was swept.

- [ ] **Step 1: Write the failing test** — fixture with a broken-spec change ⇒ `check broken-spec <id> …` line; a `blocked` change ⇒ `judgment blocked <id> …` line; assert integration sync is invoked only when a sweep occurred (mock `sync-integration-branch.sh`, count calls). Expected FAIL.

- [ ] **Step 2: Run to verify fail.**

- [ ] **Step 3: Implement the three functions and finish `main()`:**

```bash
main(){
  # … config + bootstrap (Task 1) …
  ensure_and_sync_worktree            # Task 2
  board_pass                          # Task 3
  [ "$BOARD_ONLY" = 1 ] && exit 0     # --board-only stops here
  local swept_count=0
  detect_merged | { … feed sweep_execute …; }   # Tasks 4–5; count `swept` lines into swept_count
  health_checks                       # this task
  emit_judgment                       # this task
  [ "$swept_count" -gt 0 ] && integration_sync   # this task
  exit 0
}
```

  Implementation note: capture `detect_merged`/`sweep_execute` stdout, tee it through, and count `^swept ` lines to set `swept_count` (a subshell pipe loses the variable — collect into a temp file or use process substitution).

- [ ] **Step 4: Run to verify pass.**

- [ ] **Step 5: Commit** — `git commit -m "feat(0058): orchestrator health checks, judgment lines, integration sync"`.

---

### Task 7: `--board-only` fast mode end-to-end

**Files:**
- Modify: `scripts/docket-status.sh` (verify the early exit; no new code expected)
- Test: `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: the wired `main()`.
- Produces: an assertion that `--board-only` emits `board` lines but NEVER `swept`/`check`/`harvest`/`judgment` lines and never invokes `board-checks.sh`/`sync-integration-branch.sh` (mock them, assert zero calls).

- [ ] **Step 1: Write the failing test** — run `--board-only` over a fixture that WOULD sweep in a full run; assert no `swept`/`check` lines and zero calls to the sweep/checks mocks. Expected FAIL only if the early exit is misplaced; otherwise this locks the behavior.

- [ ] **Step 2: Run to verify fail (or pass if already correct).**

- [ ] **Step 3: Fix the early-exit placement if needed** (it must sit immediately after `board_pass`).

- [ ] **Step 4: Run to verify pass.**

- [ ] **Step 5: Commit** — `git commit -m "test(0058): lock --board-only fast-mode skips sweep + checks"`.

---

### Task 8: Determinism, main-mode degradation, idempotence tests

**Files:**
- Test: `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: the full script.
- Produces: assertions that (a) two consecutive full runs over unchanged change files produce byte-identical `BOARD.md` and an idempotent no-op second run (`board inline clean`, no re-sweep); (b) `main`-mode (`DOCKET_MODE=main`, no `.docket` worktree) degrades correctly — board renders against the primary tree, integration sync is a no-op.

- [ ] **Step 1: Write the failing/locking tests** for determinism + main-mode. Expected: they encode behavior that should already hold; fix any divergence surfaced.

- [ ] **Step 2: Run.** Fix any real divergence (e.g. a stray timestamp) in `scripts/docket-status.sh`.

- [ ] **Step 3: Run full suite** `bash tests/test_docket_status.sh` → all ok.

- [ ] **Step 4: Commit** — `git commit -m "test(0058): determinism + main-mode degradation for orchestrator"`.

---

### Task 9: Fill the `scripts/docket-status.md` contract

**Files:**
- Modify: `scripts/docket-status.md`

**Interfaces:**
- Consumes: the finished script behavior.
- Produces: a complete contract matching the co-located-`.md` pattern of the other scripts — **Purpose** (one-invocation orchestrator), **Usage** (all flags), **Behavior** (the 7-step sequence + the three failure postures + `--board-only`), **Output contract** (the exact line shapes, copied from Global Constraints), **Exit codes** (0 pass / non-zero hard error), **Invariants** (determinism, no-duplication-of-mechanics, UTC dates, surface-specific postures).

- [ ] **Step 1: Write the contract** with the sections above; no code, spec-level prose.

- [ ] **Step 2: Cross-check** every flag and output line against the implemented script (grep the script for each `echo` shape and each `--flag`).

- [ ] **Step 3: Commit** — `git commit -m "docs(0058): scripts/docket-status.md contract"`.

---

### Task 10: Rewrite `docket-status` SKILL.md around the orchestrator

**Files:**
- Modify: `skills/docket-status/SKILL.md`
- Test: `tests/test_render_board.sh` (it asserts docket-status inline-surface wiring — keep it green) and `tests/test_docket_status.sh`.

**Interfaces:**
- Consumes: `scripts/docket-status.sh`, `scripts/docket-status.md`.
- Produces: a slimmed skill body: Step-0 preamble (convention pointer + config eval) → **mode choice** (see-only ⇒ `--board-only`, else full pass) → invoke `docket-status.sh`, trust exit code, surface the report → judgment follow-ups driven off `harvest`/`judgment`/`minted` lines (harvest-learnings via the finalize harvest procedure; `blocked_by:` re-examination; `issue:`/`github_project` write-backs) → final human summary. The Board/Sweep/Health-check prose sections reduce to short descriptions pointing at `scripts/docket-status.md` as the executable source (the same pattern #0053 used for `render-board.sh`'s Structure section). Preserve the `## Convention (load first — blocking)` and `## When to use` sections.

- [ ] **Step 1: Rewrite the skill body** per the interface. Keep the `name:`/`description:` frontmatter. Do NOT restate mechanics the contract owns — point at `scripts/docket-status.md`.

- [ ] **Step 2: Verify the skill-wiring test still passes** — `bash tests/test_render_board.sh` (it greps `SKILL` for the inline-surface wiring; update the grep target in the test only if the wiring reference genuinely moved, and justify it). Expected PASS.

- [ ] **Step 3: Add a skill-body assertion** to `tests/test_docket_status.sh`: the skill references `docket-status.sh` and no longer inlines the full sweep loop prose (grep for the invocation; grep-absent a now-removed marker phrase). Run → PASS.

- [ ] **Step 4: Commit** — `git commit -m "refactor(0058): rewrite docket-status skill around the orchestrator"`.

---

### Task 11: End-to-end smoke run + wire into the suite

**Files:**
- Modify: whatever aggregates the test suite (e.g. a `tests/run-all.sh` or the CI test list) — add `test_docket_status.sh` if such an aggregator exists.

**Interfaces:**
- Consumes: the full change.
- Produces: `test_docket_status.sh` runs as part of the repo suite; a real `--board-only` smoke run in this repo produces a valid board and exit 0.

- [ ] **Step 1: Locate the suite aggregator** (`grep -rl "test_render_board" tests/ *.sh Makefile 2>/dev/null`) and add the new test if changes are listed explicitly there.

- [ ] **Step 2: Smoke run** `scripts/docket-status.sh --board-only` from a clean checkout of this repo; confirm `board inline changed|clean` and exit 0, and that `BOARD.md` is well-formed (non-empty, parses).

- [ ] **Step 3: Run the whole suite** once (foreground, single call) to confirm nothing regressed.

- [ ] **Step 4: Commit** — `git commit -m "test(0058): wire orchestrator test into the suite + smoke run"`.

---

## Notes for the implementer

- **The new ADR** (relates to ADR-0012 — templated commit messages + state mutation along a blessed script sequence in deterministic pipelines) is recorded at **review time via the `docket-adr` subagent**, not in this plan's tasks. When you author the formulaic commit-message templates in Tasks 3 & 5, you are relying on exactly the decision that ADR ratifies — flag it for the review step.
- **Do not weaken or duplicate** any shared script. If a shared script lacks a flag the orchestrator needs, prefer adding the flag to that script (with its own test) over reimplementing its logic here.
- **`gh` graphql vs N calls** (spec Open Question, Task 4): decide by testing `gh api graphql` aliased-query ergonomics; either is one model turn. Prefer the single aliased query if it parses cleanly; otherwise the `gh pr list --head` fallback loop is acceptable — both must go through the `GH` seam.
- **Run the full suite in ONE foreground Bash call** with `timeout 600000` (LEARNINGS: backgrounding the suite deadlocks the loop).
