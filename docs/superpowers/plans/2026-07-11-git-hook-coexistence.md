# git-hook coexistence — docket bookkeeping commits skip hooks — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make docket's machine-generated bookkeeping commits skip the repo's shared git-hook framework (pre-commit.com, husky, lefthook, …) by construction, so docket coexists with hook-using repos.

**Architecture:** A single idempotent helper (`scripts/disable-worktree-hooks.sh`) points a docket-owned worktree's hook lookup at an empty, docket-owned directory via git's worktree-scoped `core.hooksPath` (enabled by `extensions.worktreeConfig`). Every commit into that worktree — script- or agent-driven — then finds no hooks and proceeds. The helper is called at every site that creates a docket-owned worktree: the persistent `.docket` metadata worktree (`docket-status.sh`), and the transient worktrees used by `migrate-to-docket.sh` and `terminal-publish.sh`. Feature-branch code worktrees are untouched — the team's hooks still fire on real code.

**Tech Stack:** POSIX-ish bash, git ≥ 2.20 (`extensions.worktreeConfig`), the repo's hermetic `tests/test_*.sh` convention (`assert` helper, PASS/FAIL, exit code).

## Global Constraints

- **Scope = metadata bookkeeping only.** docket's own commits on `metadata_branch` (via `.docket`) and docket's own doc-management commits onto the integration branch (migrate's prune, terminal-publish's publish) skip hooks. Feature-branch **code** commits keep running the team's hooks — never touched.
- **Idempotent + self-healing.** Every helper call is a clean no-op when already applied (exit 0, no duplicate config), so it heals existing `.docket` worktrees on the next docket run.
- **Framework-agnostic.** Disable the hook *mechanism* (`core.hooksPath` → empty dir), never one framework's config env vars.
- **Local-only.** Never touches the remote, teammates' clones, or the committed `.docket.yml`. Not a coordination key.
- **Absolute empty-hooks path** under the git common dir: `<git-common-dir>/docket/empty-hooks`, a real (empty) directory, created idempotently. Absolute so `core.hooksPath` never resolves relative to a worktree root; a real dir avoids "hooksPath does not exist" surprises.
- **Mock seam:** every script uses `GIT="${GIT:-git}"`.
- **Tests are hermetic:** pin `GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null` and per-repo `user.name`/`user.email` so no ambient `core.hooksPath` leaks in and no dev global config is read or written (LEARNINGS: hermetic tests must not reach the dev's real config).
- **The whole suite is the gate** (no CI): the final task runs every `tests/test_*.sh`, not only the new one.
- **`worktreeConfig` safety caveat:** enabling `extensions.worktreeConfig` makes `core.worktree`/`core.bare` read per-worktree; a pre-existing value in the **common** config would silently stop applying to linked worktrees. The helper detects such a value before enabling and relocates it to the main worktree's per-worktree config (git's guidance); if it cannot do so safely, it warns loudly and fails closed (exit 1) rather than proceed blindly.

---

### Task 1: The `disable-worktree-hooks.sh` helper + contract + hermetic behavior test

**Files:**
- Create: `scripts/disable-worktree-hooks.sh`
- Create: `scripts/disable-worktree-hooks.md` (contract)
- Test: `tests/test_metadata_worktree_hooks.sh`

**Interfaces:**
- Consumes: nothing (leaf helper).
- Produces: CLI `disable-worktree-hooks.sh --worktree DIR`. Behavior: creates the absolute empty-hooks dir under the git common dir; (on first enable) handles the `core.worktree`/`core.bare` safety caveat, then enables `extensions.worktreeConfig`; sets worktree-scoped `core.hooksPath` on `DIR` to the empty dir. Idempotent (clean no-op when already applied). Exit 0 on success; 1 on a bad/missing worktree or an unsafe-to-relocate common config value; 2 on usage error. Mock seam `GIT="${GIT:-git}"`. Callers (Task 2) invoke it immediately after `git worktree add` of a docket-owned worktree.

- [ ] **Step 1: Write the failing test**

Create `tests/test_metadata_worktree_hooks.sh`:

```bash
#!/usr/bin/env bash
# tests/test_metadata_worktree_hooks.sh — change 0063: disable-worktree-hooks.sh makes commits in a
# docket-owned worktree skip the repo's SHARED git hooks — worktree-scoped, idempotent, not global —
# without disabling hooks anywhere else. Hermetic: throwaway repo + an always-failing common
# pre-commit hook; ambient user/system git config ignored. Run: bash tests/test_metadata_worktree_hooks.sh
set -uo pipefail
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null   # no ambient core.hooksPath leaks in
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
HELPER="$REPO/scripts/disable-worktree-hooks.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# setup: prints the main worktree path of a fresh repo that has (a) a second docket-owned worktree
# at .docket on branch `docket`, and (b) an always-failing pre-commit hook in the COMMON hooks dir
# (shared by every worktree). No helper applied yet.
setup(){
  local root work hooks
  root="$(mktemp -d)"; root="$(cd "$root" && pwd -P)"
  work="$root/work"
  git init -q "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  git -C "$work" config commit.gpgsign false
  echo root > "$work/f.txt"; git -C "$work" add f.txt; git -C "$work" commit -qm C0
  hooks="$(cd "$work" && cd "$(git rev-parse --git-common-dir)" && pwd -P)/hooks"
  mkdir -p "$hooks"; printf '#!/bin/sh\nexit 1\n' > "$hooks/pre-commit"; chmod +x "$hooks/pre-commit"
  git -C "$work" worktree add -q "$work/.docket" -b docket
  printf '%s' "$work"
}

n=0
# try_commit DIR → echoes the exit code of a commit attempt in worktree DIR (unique file each call).
try_commit(){
  n=$((n+1))
  ( cd "$1" && printf '%s\n' "$n" > "c$n.txt" && git add "c$n.txt" && git commit -qm "c$n" ) >/dev/null 2>&1
  echo $?
}

# --- Case 1: the hook is real and active (main-worktree commit FAILS) ---
W="$(setup)"
assert "hook active: main-worktree commit fails" "[ \"\$(try_commit \"$W\")\" -ne 0 ]"

# --- Case 2: after the helper, a .docket commit SUCCEEDS (hook skipped) ---
"$HELPER" --worktree "$W/.docket" >/dev/null 2>&1
assert "helper exit 0"                            "[ $? -eq 0 ]"
assert "skip: .docket commit succeeds"            "[ \"\$(try_commit \"$W/.docket\")\" -eq 0 ]"

# --- Case 3: worktree-scoped, not global (main-worktree commit STILL fails) ---
assert "scoped: main-worktree commit still fails" "[ \"\$(try_commit \"$W\")\" -ne 0 ]"

# --- Case 4: idempotent — a second run is a clean no-op, single hooksPath entry ---
"$HELPER" --worktree "$W/.docket" >/dev/null 2>&1; rc=$?
count="$(git -C "$W/.docket" config --worktree --get-all core.hooksPath | wc -l | tr -d ' ')"
assert "idempotent: second run exit 0"            "[ $rc -eq 0 ]"
assert "idempotent: single core.hooksPath value"  "[ \"$count\" -eq 1 ]"
assert "idempotent: .docket commit still skips"   "[ \"\$(try_commit \"$W/.docket\")\" -eq 0 ]"

# --- Case 5: non-vacuous — WITHOUT the helper, a fresh .docket commit fails ---
W2="$(setup)"
assert "non-vacuous: unpatched .docket commit fails" "[ \"\$(try_commit \"$W2/.docket\")\" -ne 0 ]"

if [ "$fail" -eq 0 ]; then echo "ALL PASS"; else echo "FAILURES"; fi
exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_metadata_worktree_hooks.sh`
Expected: FAIL — `$HELPER` does not exist yet, so Case 2's commit still hits the failing hook (`NOT OK - skip: .docket commit succeeds`), exit 1. (Cases 1 and 5 should already pass — they prove the fixture's hook is real.)

- [ ] **Step 3: Write the helper**

Create `scripts/disable-worktree-hooks.sh`:

```bash
#!/usr/bin/env bash
# scripts/disable-worktree-hooks.sh — disable git hooks on a docket-owned worktree, idempotently, so
# docket's bookkeeping commits skip the repo's shared hook framework (pre-commit/husky/lefthook).
# Change 0063. Contract: scripts/disable-worktree-hooks.md. Mock seam: GIT="${GIT:-git}".
set -uo pipefail
GIT="${GIT:-git}"
die(){ echo "disable-worktree-hooks: $1" >&2; exit "${2:-1}"; }
usage(){ echo "usage: disable-worktree-hooks.sh --worktree DIR" >&2; }

WT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --worktree) WT="${2:-}"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "disable-worktree-hooks: unknown argument: $1" >&2; usage; exit 2 ;;
  esac; shift
done
[ -n "$WT" ] || { usage; exit 2; }
[ -d "$WT" ] || die "worktree dir not found: $WT"

# Absolute empty, docket-owned hooks dir inside the common git dir. Under .git/, never tracked,
# never leaks into a commit. Absolute (via pwd -P) so core.hooksPath never resolves relative to a
# worktree root; a real (empty) dir avoids "hooksPath does not exist" surprises.
common="$(cd "$WT" && cd "$("$GIT" rev-parse --git-common-dir 2>/dev/null)" && pwd -P)" \
  || die "cannot resolve git common dir for $WT"
empty="$common/docket/empty-hooks"
mkdir -p "$empty"

# worktreeConfig safety (git >=2.20): once enabled, core.worktree/core.bare read per-worktree, so a
# value in the COMMON config would silently stop applying to linked worktrees. Detect before enabling
# and relocate to the MAIN worktree's per-worktree config (git's guidance); if that cannot be done
# safely, warn loudly and fail closed rather than proceed blindly.
if [ "$("$GIT" -C "$WT" config --local --get extensions.worktreeConfig 2>/dev/null || true)" != "true" ]; then
  main_wt="$("$GIT" -C "$WT" worktree list --porcelain 2>/dev/null | awk '/^worktree /{print $2; exit}')"
  for key in core.worktree core.bare; do
    val="$("$GIT" -C "$WT" config --local --get "$key" 2>/dev/null || true)"
    [ -n "$val" ] || continue
    if [ -n "$main_wt" ] \
       && "$GIT" -C "$WT" config extensions.worktreeConfig true \
       && "$GIT" -C "$main_wt" config --worktree "$key" "$val" \
       && "$GIT" -C "$WT" config --local --unset "$key"; then
      echo "disable-worktree-hooks: relocated common $key='$val' to $main_wt (worktreeConfig safety)" >&2
    else
      die "refusing to enable worktreeConfig — common $key='$val' present and could not be relocated safely; set core.hooksPath per-invocation instead"
    fi
  done
  "$GIT" -C "$WT" config extensions.worktreeConfig true
fi

# Point THIS worktree's hook lookup at the empty dir (worktree-scoped). Idempotent: a repeat write is
# the same value, and --worktree replaces rather than appends, so there is never a duplicate entry.
"$GIT" -C "$WT" config --worktree core.hooksPath "$empty"
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_metadata_worktree_hooks.sh`
Expected: `ALL PASS`, exit 0 (all five cases green).

- [ ] **Step 5: Make the helper executable**

Run: `chmod +x scripts/disable-worktree-hooks.sh`

- [ ] **Step 6: Write the contract**

Create `scripts/disable-worktree-hooks.md`:

```markdown
# disable-worktree-hooks.sh — skip git hooks on a docket-owned worktree, idempotently

## Purpose

Points a docket-owned worktree's git-hook lookup at an empty, docket-owned directory, so every
commit into that worktree skips the repo's shared hook framework (pre-commit.com, husky, lefthook).
docket makes many machine-generated bookkeeping commits into worktrees on the orphan `docket` branch
(no `.pre-commit-config.yaml`) and onto the integration branch (its own docs); a shared `pre-commit`
hook would hard-fail or run against commits it was never meant to guard. This helper disables the
hook *mechanism* — framework-agnostically — by construction, so no per-commit flag can be forgotten.

Scope is metadata bookkeeping only. Feature-branch code worktrees are never passed to this helper,
so the team's code-quality hooks still fire on real code headed to a PR (change 0063).

Invoked by `docket-status.sh` (the persistent `.docket` worktree), `migrate-to-docket.sh` (its
transient seed/prune worktrees), and `terminal-publish.sh` (its transient publish worktree),
immediately after each `git worktree add`. Idempotent and self-healing — a repeat call is a clean
no-op, so existing installs are fixed on the next docket run.

## Usage

```
disable-worktree-hooks.sh --worktree DIR
```

- `--worktree DIR` — the docket-owned worktree to disable hooks on (required).

**Mock seam:** `GIT="${GIT:-git}"`.

## Behavior

1. **Resolve the empty hooks dir.** `<git-common-dir>/docket/empty-hooks`, resolved to an absolute
   path (`cd DIR && cd "$(git rev-parse --git-common-dir)" && pwd -P`) and created with `mkdir -p`.
   Absolute so `core.hooksPath` never resolves relative to a worktree root; a real empty directory
   avoids "hooksPath does not exist" surprises in git and in a framework's own `core.hooksPath`
   checks. Living under `.git/`, it is never tracked and never leaks into a commit.
2. **worktreeConfig safety (first enable only).** If `extensions.worktreeConfig` is not already
   `true`, detect a pre-existing **common-config** `core.worktree`/`core.bare` value: relocate it to
   the main worktree's per-worktree config, then enable `extensions.worktreeConfig`. If a value is
   present and cannot be relocated safely, warn loudly and exit 1 (fail-closed) — never enable
   blindly. In virtually all repos these keys are unset, so this is a no-op path.
3. **Set the worktree-scoped hooks path.** `git -C DIR config --worktree core.hooksPath <empty>`.
   `--worktree` replaces rather than appends, so re-running never duplicates the entry.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Hooks disabled on DIR — or already disabled (idempotent no-op). |
| 1 | DIR missing/not a worktree, common git dir unresolvable, or an unsafe-to-relocate `core.worktree`/`core.bare` blocked enabling. |
| 2 | Usage error (missing `--worktree`, unknown flag). |

## Invariants

- **Worktree-scoped, never global.** Only the passed worktree's `core.hooksPath` is set; the main
  working tree and every feature worktree keep running the team's hooks. The behavior test asserts a
  main-worktree commit still fails after the helper runs.
- **Idempotent.** A repeat call re-writes the same value under `--worktree`; there is never a
  duplicate `core.hooksPath` entry and no error. This is what makes it self-healing at every
  create/ensure site.
- **Local-only.** Touches only `.git/config` and `.git/worktrees/<wt>/config.worktree` plus a dir
  under `.git/`. Never the remote, teammates' clones, or the committed `.docket.yml`.
- **Fail-closed on the worktreeConfig caveat.** Rather than risk silently unsetting `core.worktree`/
  `core.bare` for linked worktrees, it relocates-or-refuses.
```

- [ ] **Step 7: Verify the contract-coverage test stays green**

Run: `bash tests/test_script_contracts_coverage.sh`
Expected: `ok - contract present for disable-worktree-hooks.sh` and `ok - script present for disable-worktree-hooks.md` among the output; exit 0. (The test auto-discovers the new `scripts/*.sh`↔`scripts/*.md` pair.)

- [ ] **Step 8: Commit**

```bash
git add scripts/disable-worktree-hooks.sh scripts/disable-worktree-hooks.md tests/test_metadata_worktree_hooks.sh
git commit -m "feat(0063): disable-worktree-hooks.sh helper + contract + hermetic test"
```

---

### Task 2: Wire the helper into every docket-owned worktree-creation site

**Files:**
- Modify: `scripts/docket-status.sh` — `ensure_and_sync_worktree()` (after the `.docket` worktree exists).
- Modify: `migrate-to-docket.sh` — after each transient worktree add (`DOCKET_WT` seed/top-up; `PRUNE_WT` prune).
- Modify: `scripts/terminal-publish.sh` — after the transient `pub-$T` worktree add.
- Test: `tests/test_worktree_hooks_wiring.sh` (structural — each site invokes the helper).

**Interfaces:**
- Consumes: `disable-worktree-hooks.sh --worktree DIR` from Task 1.
- Produces: nothing new; each call site becomes hook-safe. Callers resolve the helper next to themselves — `docket-status.sh` via `"$SELF_DIR"/disable-worktree-hooks.sh`, `terminal-publish.sh` via `"$(dirname "$0")/disable-worktree-hooks.sh"`, `migrate-to-docket.sh` via `"$(dirname "$0")/scripts/disable-worktree-hooks.sh"` (migrate lives at repo root; the helper lives in `scripts/`).

> **Why these exact sites (from reconcile):** `docket-config.sh --bootstrap` is intentionally NOT wired — `create_orphan()` is worktree-free (builds the orphan via `commit-tree` + push), so there is no worktree to scope config to; the `.docket` worktree it precedes is created and disabled immediately afterward by `ensure_and_sync_worktree`. `migrate-to-docket.sh` and `terminal-publish.sh` use **transient** worktrees (not `.docket`): migrate's `DOCKET_WT` (orphan `docket`, seed L254 / top-up L271) and `PRUNE_WT` (integration branch, prune L321), and terminal-publish's `pub-$T` (integration branch, publish commit + `rebase --continue` replay). Applying the helper to the transient worktree right after `worktree add` covers **every** commit in it — including the rebase replay a per-invocation `-c core.hooksPath` on a single commit line would miss.

- [ ] **Step 1: Write the failing structural test**

Create `tests/test_worktree_hooks_wiring.sh`:

```bash
#!/usr/bin/env bash
# tests/test_worktree_hooks_wiring.sh — change 0063: every docket-owned worktree-creation site calls
# disable-worktree-hooks.sh, and the worktree-free bootstrap does NOT. Structural (grep) audit, in the
# spirit of the spec's terminal-publish structural check. Run: bash tests/test_worktree_hooks_wiring.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "docket-status ensure calls the helper" \
  "grep -q 'disable-worktree-hooks.sh' \"$REPO/scripts/docket-status.sh\""
assert "migrate calls the helper" \
  "grep -q 'disable-worktree-hooks.sh' \"$REPO/migrate-to-docket.sh\""
assert "terminal-publish calls the helper" \
  "grep -q 'disable-worktree-hooks.sh' \"$REPO/scripts/terminal-publish.sh\""
# The worktree-free bootstrap must NOT wire it (there is no worktree to scope).
assert "docket-config bootstrap does NOT call the helper" \
  "! grep -q 'disable-worktree-hooks.sh' \"$REPO/scripts/docket-config.sh\""

if [ "$fail" -eq 0 ]; then echo "ALL PASS"; else echo "FAILURES"; fi
exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_worktree_hooks_wiring.sh`
Expected: FAIL — the three positive assertions fail (no call sites yet); the negative one passes.

- [ ] **Step 3: Wire `docket-status.sh` `ensure_and_sync_worktree()`**

In `scripts/docket-status.sh`, inside `ensure_and_sync_worktree()`, after the worktree exists (i.e. after the `if [ ! -d "$wt" ]; then … fi` block that adds it, and before/after the fetch+pull — hooks only affect commits, so anywhere after creation is fine; place it right after the create block). Replace:

```bash
    if [ ! -d "$wt" ]; then
      "$GIT" worktree add "$wt" "$METADATA_BRANCH" >&2 2>/dev/null \
        || "$GIT" worktree add "$wt" "origin/$METADATA_BRANCH" >&2 \
        || { echo "docket-status: cannot create metadata worktree $wt" >&2; exit 1; }
    fi
    "$GIT" -C "$wt" fetch origin "$METADATA_BRANCH" >&2 \
```

with:

```bash
    if [ ! -d "$wt" ]; then
      "$GIT" worktree add "$wt" "$METADATA_BRANCH" >&2 2>/dev/null \
        || "$GIT" worktree add "$wt" "origin/$METADATA_BRANCH" >&2 \
        || { echo "docket-status: cannot create metadata worktree $wt" >&2; exit 1; }
    fi
    # Change 0063: skip the repo's shared git hooks on the metadata worktree (idempotent; self-heals
    # existing installs). Best-effort — a failure here must not block the status pass.
    "$SELF_DIR"/disable-worktree-hooks.sh --worktree "$wt" >&2 \
      || echo "docket-status: warning — could not disable hooks on $wt (continuing)" >&2
    "$GIT" -C "$wt" fetch origin "$METADATA_BRANCH" >&2 \
```

- [ ] **Step 4: Wire `migrate-to-docket.sh` (both transient worktrees)**

In `migrate-to-docket.sh`, after the `DOCKET_WT` worktree is added (both the orphan-create path ~L251 and the top-up path ~L260), and after the `PRUNE_WT` add (~L305), call the helper. Add after the orphan create (`git worktree add --orphan -b docket "$DOCKET_WT" >/dev/null`):

```bash
  git worktree add --orphan -b docket "$DOCKET_WT" >/dev/null
  "$(dirname "$0")/scripts/disable-worktree-hooks.sh" --worktree "$DOCKET_WT" >/dev/null 2>&1 || true
```

Add after the top-up worktree add (`git worktree add "$DOCKET_WT" docket >/dev/null`):

```bash
  git worktree add "$DOCKET_WT" docket >/dev/null
  "$(dirname "$0")/scripts/disable-worktree-hooks.sh" --worktree "$DOCKET_WT" >/dev/null 2>&1 || true
```

Add after the prune worktree add (`git worktree add -B "migrate-prune-$INTEGRATION_BRANCH" "$PRUNE_WT" "$INTEGRATION_REF" >/dev/null`):

```bash
git worktree add -B "migrate-prune-$INTEGRATION_BRANCH" "$PRUNE_WT" "$INTEGRATION_REF" >/dev/null
"$(dirname "$0")/scripts/disable-worktree-hooks.sh" --worktree "$PRUNE_WT" >/dev/null 2>&1 || true
```

(All three are docket's own bookkeeping commits — the orphan seed has no `.pre-commit-config.yaml`; the prune is docket removing its own planning surface from the integration branch. Best-effort `|| true`: migration must not hard-fail if hook-disable is unavailable, and if it is skipped the commit either succeeds anyway or fails exactly as today — no regression.)

- [ ] **Step 5: Wire `terminal-publish.sh` (the transient `pub-$T` worktree)**

In `scripts/terminal-publish.sh`, immediately after the `pub-$T` worktree is provisioned (the `$GIT worktree add -B "pub-$T" "$pub" "$REMOTE/$INT_BRANCH"` block), disable hooks on `$pub` so both the publish commit (L147) and the CAS `rebase --continue` replay (L154) skip the team's hooks. Replace:

```bash
$GIT worktree add -B "pub-$T" "$pub" "$REMOTE/$INT_BRANCH" >/dev/null 2>&1 \
  || die "could not provision pub-$T worktree"
```

with:

```bash
$GIT worktree add -B "pub-$T" "$pub" "$REMOTE/$INT_BRANCH" >/dev/null 2>&1 \
  || die "could not provision pub-$T worktree"
# Change 0063: this is docket's own doc-publish commit (its archived change/spec/ADRs), not the
# team's code — skip the integration branch's shared hooks on it. Covers the publish commit AND the
# CAS rebase --continue replay below (worktree-scoped, torn down with the worktree). Best-effort.
"$(dirname "$0")/disable-worktree-hooks.sh" --worktree "$pub" >/dev/null 2>&1 || true
```

- [ ] **Step 6: Run the structural test to verify it passes**

Run: `bash tests/test_worktree_hooks_wiring.sh`
Expected: `ALL PASS`, exit 0.

- [ ] **Step 7: Re-run the behavior + contract-coverage tests (no regressions)**

Run: `bash tests/test_metadata_worktree_hooks.sh && bash tests/test_docket_status.sh && bash tests/test_terminal_publish.sh`
Expected: each prints `ALL PASS` (or its suite's pass marker) and exits 0. (Confirms the inserted helper calls didn't break the existing docket-status / terminal-publish flows — their tests use the `GIT` mock seam, and the helper is a sibling script the mock does not intercept, so the call is a real, idempotent no-op against the fixture worktree or a harmless best-effort warning.)

> If `test_docket_status.sh` or `test_terminal_publish.sh` exercises the wired code path with a `GIT` mock that does not provide `worktree`/`config`, the helper call may emit a warning but must not fail the flow (guarded by `|| echo …`/`|| true`). If a test asserts exact stderr/PASS and the warning perturbs it, adjust the test fixture to tolerate the best-effort warning line — do not weaken the guard.

- [ ] **Step 8: Commit**

```bash
git add scripts/docket-status.sh migrate-to-docket.sh scripts/terminal-publish.sh tests/test_worktree_hooks_wiring.sh
git commit -m "feat(0063): wire disable-worktree-hooks into ensure/migrate/publish worktree sites"
```

---

### Task 3: Documentation — convention Step-0 note + README git-hook-frameworks section

**Files:**
- Modify: `skills/docket-convention/SKILL.md` — Step-0 preamble ("ensure + sync the metadata working tree") + Branch-model.
- Modify: `README.md` — a short "git-hook frameworks" note.

**Interfaces:**
- Consumes: nothing.
- Produces: prose only — tells interactive skills (`docket-new-change`, `docket-groom-next`) that ensure the `.docket` worktree inline to also run the helper, and tells users docket's bookkeeping commits skip their hooks while their code commits do not.

- [ ] **Step 1: Add the Step-0 preamble note in `skills/docket-convention/SKILL.md`**

In the Step-0 preamble item that says to ensure + sync the metadata working tree in `docket`-mode, append one sentence after the create/sync instruction:

```markdown
   In `docket`-mode: the persistent `.docket/` worktree parked on `docket` (state-specific create per *Branch model*, idempotent); after ensuring it exists, run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/disable-worktree-hooks.sh --worktree .docket` so docket's bookkeeping commits skip the repo's shared git hooks (idempotent; self-heals existing worktrees — change 0063). **Sync before any read** — `git -C .docket fetch origin docket && git -C .docket pull --rebase origin docket`; pushes target `origin/docket`.
```

(Match the exact surrounding wording in the file; the added clause is the `disable-worktree-hooks.sh` sentence. Keep it to that single clause so the diff is minimal.)

- [ ] **Step 2: Add a Branch-model sentence in `skills/docket-convention/SKILL.md`**

In the Branch model section where the metadata working tree / `.docket` worktree is described, add one sentence noting the hook-skip is a property of the metadata worktree:

```markdown
The `.docket` metadata worktree has the repo's shared git hooks disabled (worktree-scoped `core.hooksPath` → an empty docket-owned dir, via `disable-worktree-hooks.sh`), so docket's many machine-generated bookkeeping commits coexist with a hook framework (pre-commit/husky/lefthook) on the integration branch; feature-branch code commits are untouched and still run the team's hooks (change 0063).
```

- [ ] **Step 3: Add the README "git-hook frameworks" note**

In `README.md`, add a short subsection (placement per reconcile Open question — a standalone "git-hook frameworks" note near the branch-model / migration material):

```markdown
### git-hook frameworks (pre-commit, husky, lefthook)

docket makes many small machine-generated bookkeeping commits (claims, board refreshes, status
writes, ADRs) on its metadata branch. Those commits **skip your repo's git hooks** by construction —
the `.docket` metadata worktree (and docket's transient publish/migration worktrees) have
`core.hooksPath` pointed at an empty directory, so a shared `pre-commit` hook never fires against
docket's own commits (which live on the orphan `docket` branch with no hook config anyway). Your
**code** commits on feature branches are untouched — the team's hooks still run on everything headed
to a PR. Nothing to configure; it is applied and self-heals on every docket run.
```

- [ ] **Step 4: Verify the docs render and the convention-extraction test stays green**

Run: `bash tests/test_convention_extraction.sh`
Expected: its pass marker, exit 0. (If this test pins exact convention prose/line structure, confirm the added sentences don't break an assertion; adjust the added prose to fit the section rather than restructuring the file.)

- [ ] **Step 5: Commit**

```bash
git add skills/docket-convention/SKILL.md README.md
git commit -m "docs(0063): note metadata-worktree hook-skip in convention + README"
```

---

### Task 4: Whole-suite gate

**Files:** none (verification only).

**Interfaces:** none.

- [ ] **Step 1: Run the entire test suite**

Run: `for t in tests/test_*.sh; do echo "== $t =="; bash "$t" || echo "FAILED: $t"; done`
Expected: every test prints its pass marker; no `FAILED:` line. The whole suite is the de-facto gate (LEARNINGS: run the WHOLE suite, never only the enumerated tests — tests outside the touched set exist to catch exactly the regressions a scoped run misses).

- [ ] **Step 2: Fix any regression, then re-run the full suite**

If any test fails, root-cause and fix minimally (never weaken a test), then re-run Step 1 until clean.

- [ ] **Step 3: Final commit (only if Step 2 changed anything)**

```bash
git add -A
git commit -m "fix(0063): resolve suite regressions from hook-skip wiring"
```

---

## Self-Review

**1. Spec coverage:**
- Helper `disable-worktree-hooks.sh` + contract → Task 1. ✅ (approach; empty-hooks dir; worktreeConfig safety)
- Hermetic hook test (spec's 6 test points: real hook, skip, worktree-scoped, idempotent, non-vacuous) → Task 1 test Cases 1–5. ✅ (spec step "removing the helper call flips step 3 back" = Case 5 non-vacuous)
- Create/ensure sites (docket-status ensure, migrate) → Task 2. ✅ Bootstrap dropped as worktree-free per reconcile → Task 2 negative assertion. ✅
- terminal-publish per-invocation skip → Task 2 Step 5, upgraded to worktree-scoped on `pub-$T` per reconcile (covers rebase replay). ✅
- Convention SKILL.md + README notes → Task 3. ✅
- `test_script_contracts_coverage.sh` auto-picks up the new contract → Task 1 Step 7. ✅
- Whole-suite gate → Task 4. ✅

**2. Placeholder scan:** No TBD/TODO; every code step shows complete code; every run step shows the command + expected output. ✅

**3. Type/name consistency:** The helper's CLI is `disable-worktree-hooks.sh --worktree DIR` everywhere (Task 1 defines it; Tasks 2 and 3 call it with `--worktree`). The empty-hooks path `<git-common-dir>/docket/empty-hooks` is identical in the helper, contract, and README. ✅

**Open questions resolved at plan time:**
- Unsafe-`worktreeConfig` degrade: chose **relocate-and-proceed, else fail-closed** (helper Step 3, contract Invariants) — not a silent degrade, matching the change's "by construction" intent.
- README placement: chose a **standalone "git-hook frameworks" subsection** (Task 3 Step 3).
