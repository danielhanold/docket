# Auto-grant docket's integration-branch push permission Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pre-authorize *only* docket's terminal-publish push to the integration branch, per-repo and locally, by writing one Claude Code allow-rule into `<repo>/.claude/settings.local.json` — so close-out runs without a per-push permission prompt.

**Architecture:** A new idempotent, standalone-runnable helper `scripts/ensure-claude-settings.sh` writes the allow-rule `Bash(git -C * push origin HEAD:<integration_branch>)` (mirroring terminal-publish's real command shape, so no skill/convention edit is needed). The integration branch is resolved by consuming change #26's `scripts/docket-config.sh --export` (no second config-parsing site), with a `DOCKET_INTEGRATION_BRANCH` env override that doubles as the hermetic test seam. `migrate-to-docket.sh` invokes the helper at setup and also adds `.claude/settings.local.json` to `.gitignore` so the grant is never committed onto collaborators — independent of any per-user global ignore.

**Tech Stack:** Bash, `jq` 1.8.x (already a repo dependency — `scripts/github-mirror.sh` uses it), git. Hermetic bash tests in the style of `tests/test_docket_config.sh`.

## Global Constraints

- The allow-rule string is **exactly** `Bash(git -C * push origin HEAD:<integration_branch>)` — literal prefix `git -C `, single `*` absorbing the mktemp worktree path, literal tail ` push origin HEAD:<integration_branch>`. This mirrors `scripts/terminal-publish.sh:108` (`$GIT -C "$pub" push "$REMOTE" "HEAD:$INT_BRANCH"`, `REMOTE` defaults to `origin`). Force-push and other-branch pushes must NOT match.
- The settings file is `<repo>/.claude/settings.local.json`, key path `.permissions.allow` (a JSON array of strings).
- The helper writes that one file only and does **no git writes** (it is gitignored and standalone-runnable).
- The helper resolves the **target repo root** from `$PWD` via `git rev-parse --show-toplevel` (usable from any consuming repo), and finds its **sibling** `docket-config.sh` via its own script directory (`${BASH_SOURCE[0]}`) — the two differ when migrating an external repo.
- Idempotent: a second run adds no duplicate; every pre-existing key and allow-rule is preserved, order intact.
- Shell style matches `scripts/docket-config.sh`: `set -uo pipefail`, `GIT="${GIT:-git}"` mock seam, `die()` writing `scriptname: msg` to stderr and `exit 1`, header comment block.
- Tests match `tests/test_docket_config.sh`: `assert "<name>" '<eval-condition>'`, `ok - ` / `NOT OK - ` lines, final `PASS`/`FAIL` + `exit "$fail"`, hermetic temp repos, empty-bare-clone warning silenced (LEARNINGS #26).
- No `producer | grep -q` under pipefail (LEARNINGS #11/#16). Every new test assertion must be mutation-genuine (deleting the clause it guards flips it to NOT OK) (LEARNINGS #2/#5/#15).

---

### Task 1: The helper `scripts/ensure-claude-settings.sh` + its hermetic test

**Files:**
- Create: `scripts/ensure-claude-settings.sh`
- Test: `tests/test_ensure_claude_settings.sh`

**Interfaces:**
- Consumes: `scripts/docket-config.sh --export --repo-dir <dir>` (change #26) → eval-able `KEY=value` lines incl. `INTEGRATION_BRANCH`. Mock/override seam: `DOCKET_INTEGRATION_BRANCH` env (when set & non-empty, `docket-config.sh` is not called).
- Produces: `scripts/ensure-claude-settings.sh` — run with no args from inside (or pointing `$PWD` at) the target repo; writes `<repo>/.claude/settings.local.json` containing the allow-rule; prints a one-line summary; exit 0 on success, non-zero with a `ensure-claude-settings: …` stderr diagnostic on error.

- [ ] **Step 1: Write the failing test**

Create `tests/test_ensure_claude_settings.sh`:

```bash
#!/usr/bin/env bash
# tests/test_ensure_claude_settings.sh — hermetic tests for scripts/ensure-claude-settings.sh
# (change 0027). Run: bash tests/test_ensure_claude_settings.sh
# Env-seam cases need no network; one bare-origin fixture exercises the real docket-config.sh path.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SCRIPT="$REPO/scripts/ensure-claude-settings.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

RULE_MAIN='Bash(git -C * push origin HEAD:main)'
RULE_DEV='Bash(git -C * push origin HEAD:develop)'

# jq helpers (keep nested quoting out of the assert conditions)
has_rule(){   jq -e --arg r "$2" '(.permissions.allow // []) | index($r)' "$1" >/dev/null 2>&1; }
rule_count(){ jq --arg r "$2" '[(.permissions.allow // [])[] | select(. == $r)] | length' "$1"; }
has_key(){    jq -e --arg k "$2" 'has($k)' "$1" >/dev/null 2>&1; }

# plain git repo (one commit, no origin) — for the env-seam cases
mkgit(){
  local d="$1"; mkdir -p "$d"; git -C "$d" init --quiet
  git -C "$d" config user.email t@t.test; git -C "$d" config user.name Test
  : > "$d/README.md"; git -C "$d" add README.md; git -C "$d" commit --quiet -m init
}
# bare-origin clone on main (for the real-resolver case); silence empty-clone warning (LEARNINGS #26)
mkrepo(){
  local dir="$1" bare="$1.origin.git"
  git init --quiet --bare "$bare"
  git clone --quiet "$bare" "$dir" 2>/dev/null
  git -C "$dir" config user.email t@t.test; git -C "$dir" config user.name Test
  git -C "$dir" checkout --quiet -b main
  : > "$dir/README.md"; git -C "$dir" add README.md; git -C "$dir" commit --quiet -m init
  git -C "$dir" push --quiet -u origin main
  git -C "$dir" remote set-head origin -a >/dev/null 2>&1
}
# run helper in <dir>; optional <branch> sets the DOCKET_INTEGRATION_BRANCH env seam
run(){
  local d="$1" br="${2:-}"
  if [ -n "$br" ]; then ( cd "$d" && DOCKET_INTEGRATION_BRANCH="$br" bash "$SCRIPT" )
  else ( cd "$d" && bash "$SCRIPT" ); fi
}

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# --- (1) create-when-absent: no .claude/ -> file created with the rule ----------
mkgit "$tmp/a"
run "$tmp/a" main >/dev/null
S="$tmp/a/.claude/settings.local.json"
assert "create: settings.local.json exists" '[ -f "$S" ]'
assert "create: rule present (HEAD:main)"   'has_rule "$S" "$RULE_MAIN"'

# --- (2) idempotent: second run adds no duplicate (count EXACTLY 1) -------------
run "$tmp/a" main >/dev/null
assert "idempotent: rule count is exactly 1" '[ "$(rule_count "$S" "$RULE_MAIN")" -eq 1 ]'

# --- (3) preserve existing keys + unrelated rule --------------------------------
mkgit "$tmp/b"
mkdir -p "$tmp/b/.claude"
cat > "$tmp/b/.claude/settings.local.json" <<'JSON'
{ "permissions": { "allow": ["Bash(ls)"] }, "env": { "KEEP": "1" } }
JSON
run "$tmp/b" main >/dev/null
SB="$tmp/b/.claude/settings.local.json"
assert "preserve: pre-existing rule kept"        'has_rule "$SB" "Bash(ls)"'
assert "preserve: unrelated top-level key kept"  'has_key  "$SB" "env"'
assert "preserve: new rule added"                'has_rule "$SB" "$RULE_MAIN"'

# --- (4) branch resolution via env seam: develop tail --------------------------
mkgit "$tmp/c"
run "$tmp/c" develop >/dev/null
SC="$tmp/c/.claude/settings.local.json"
assert "branch resolution: develop tail"        'has_rule "$SC" "$RULE_DEV"'
assert "branch resolution: no stray main rule"  '! has_rule "$SC" "$RULE_MAIN"'

# --- (5a) no git writes: helper makes no commit and stages nothing -------------
mkgit "$tmp/d"
before="$(git -C "$tmp/d" rev-parse HEAD)"
run "$tmp/d" main >/dev/null
after="$(git -C "$tmp/d" rev-parse HEAD)"
assert "no git writes: HEAD unchanged (no commit)" '[ "$before" = "$after" ]'
assert "no git writes: nothing staged"             'git -C "$tmp/d" diff --cached --quiet'

# --- (5b) the migrate gitignore entry string actually ignores the file ----------
mkgit "$tmp/e"
printf '.claude/settings.local.json\n' > "$tmp/e/.gitignore"
git -C "$tmp/e" add .gitignore; git -C "$tmp/e" commit --quiet -m ignore
run "$tmp/e" main >/dev/null
assert "gitignore string ignores settings.local.json" \
  '[ -z "$(git -C "$tmp/e" status --porcelain -- .claude/settings.local.json)" ]'

# --- (6) REAL resolver path: helper consults docket-config.sh (no env seam) ----
# main-mode + integration_branch: develop -> docket-config.sh emits develop, no ref needed,
# bootstrap guard skipped (main-mode). Proves the #26 wiring is real, not a vacuous seam.
mkrepo "$tmp/f"
printf 'metadata_branch: main\nintegration_branch: develop\n' > "$tmp/f/.docket.yml"
git -C "$tmp/f" add .docket.yml; git -C "$tmp/f" commit --quiet -m cfg
git -C "$tmp/f" push --quiet origin main
run "$tmp/f" >/dev/null            # NO env seam -> exercises scripts/docket-config.sh
SF="$tmp/f/.claude/settings.local.json"
assert "real resolver: develop tail from docket-config.sh" 'has_rule "$SF" "$RULE_DEV"'

if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi
exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_ensure_claude_settings.sh`
Expected: FAIL — `bash: …/scripts/ensure-claude-settings.sh: No such file or directory` for each `run`, so the file-existence/rule asserts print `NOT OK`, final line `FAIL`, exit 1.

- [ ] **Step 3: Write the helper**

Create `scripts/ensure-claude-settings.sh`:

```bash
#!/usr/bin/env bash
# scripts/ensure-claude-settings.sh — grant docket's terminal-publish push to the integration
# branch, per-repo and LOCALLY, by writing one Claude Code allow-rule into the repo's
# <repo>/.claude/settings.local.json (change 0027). Idempotent + standalone-runnable: a fresh
# cloner of an already-migrated repo can run this directly to grant themselves the rule (the
# file is gitignored and per-user, so migrate's one-time run does not cover later clones).
#
# The rule pre-authorizes ONLY terminal-publish's exact command shape —
#   git -C <transient-worktree> push origin HEAD:<integration_branch>
# (force-push and other-branch pushes stay guarded). It mirrors the real command, so no
# skill/convention edit is needed.
#
# Usage: ensure-claude-settings.sh      # operate on the repo containing $PWD
#   The integration branch is resolved via the sibling scripts/docket-config.sh (change 0026),
#   unless DOCKET_INTEGRATION_BRANCH is set (manual override + hermetic test seam).
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

GIT="${GIT:-git}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"   # this script's dir -> sibling docket-config.sh
die() { printf 'ensure-claude-settings: %s\n' "$*" >&2; exit 1; }

# --- resolve the TARGET repo root from $PWD (like migrate-to-docket.sh) ---
REPO_ROOT="$("$GIT" rev-parse --show-toplevel 2>/dev/null)" \
  || die "not inside a git repo — cd into the repo you want to grant, then re-run."

# --- resolve the integration branch (env override > docket-config.sh) ---
if [ -n "${DOCKET_INTEGRATION_BRANCH:-}" ]; then
  INTEGRATION_BRANCH="$DOCKET_INTEGRATION_BRANCH"
else
  cfg="$("$HERE/docket-config.sh" --export --repo-dir "$REPO_ROOT")" \
    || die "could not resolve config via docket-config.sh (is origin reachable?)"
  eval "$cfg"
  [ -n "${INTEGRATION_BRANCH:-}" ] || die "docket-config.sh did not report INTEGRATION_BRANCH"
fi

RULE="Bash(git -C * push origin HEAD:$INTEGRATION_BRANCH)"
SETTINGS_DIR="$REPO_ROOT/.claude"
SETTINGS="$SETTINGS_DIR/settings.local.json"

# --- ensure the local Claude config exists (create the whole thing if absent) ---
created=0
if [ ! -f "$SETTINGS" ]; then
  mkdir -p "$SETTINGS_DIR"
  printf '{}\n' > "$SETTINGS"
  created=1
fi

# Refuse to clobber a corrupt pre-existing file.
jq empty "$SETTINGS" 2>/dev/null || die "$SETTINGS is not valid JSON — fix or remove it, then re-run."

# Was the rule already present? (for the idempotency report only)
already=0
if jq -e --arg r "$RULE" '(.permissions.allow // []) | index($r)' "$SETTINGS" >/dev/null 2>&1; then
  already=1
fi

# --- idempotently merge the rule, preserving every existing key/rule + order ---
tmp="$(mktemp)"
if jq --arg rule "$RULE" '
      .permissions //= {}
      | .permissions.allow //= []
      | if (.permissions.allow | index($rule)) == null
        then .permissions.allow += [$rule]
        else .
        end
    ' "$SETTINGS" > "$tmp"; then
  mv "$tmp" "$SETTINGS"
else
  rm -f "$tmp"; die "failed to update $SETTINGS"
fi

# --- one-line summary ---
rel="${SETTINGS#"$REPO_ROOT"/}"
if [ "$already" -eq 1 ]; then
  printf 'ensure-claude-settings: rule already present in %s — no change.\n' "$rel"
elif [ "$created" -eq 1 ]; then
  printf 'ensure-claude-settings: created %s and granted: %s\n' "$rel" "$RULE"
else
  printf 'ensure-claude-settings: added grant to %s: %s\n' "$rel" "$RULE"
fi
```

- [ ] **Step 4: Make the helper executable**

Run: `chmod +x scripts/ensure-claude-settings.sh`
(Matches the executable bit on `scripts/docket-config.sh` / `scripts/render-board.sh`.)

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_ensure_claude_settings.sh`
Expected: every line `ok - …`, final line `PASS`, exit 0.

- [ ] **Step 6: Verify mutation-genuineness of the key asserts (LEARNINGS #2/#5)**

Manually confirm (do NOT commit these mutations — revert after):
- Temporarily change the helper's `RULE` tail to a wrong branch (e.g. `HEAD:wrong`) → cases (1),(4),(6) flip to `NOT OK`. Revert.
- Temporarily make the jq merge append unconditionally (drop the `index(...) == null` guard) → case (2) `rule count is exactly 1` flips to `NOT OK` (count becomes 2). Revert.
Re-run `bash tests/test_ensure_claude_settings.sh`; confirm `PASS` again.

- [ ] **Step 7: Commit**

```bash
git add scripts/ensure-claude-settings.sh tests/test_ensure_claude_settings.sh
git commit -m "feat(0027): ensure-claude-settings.sh — per-repo local grant for terminal-publish push

Writes the allow-rule Bash(git -C * push origin HEAD:<integration_branch>) into
<repo>/.claude/settings.local.json; integration branch via docket-config.sh (#26) or the
DOCKET_INTEGRATION_BRANCH env seam. Idempotent, preserves existing keys, no git writes."
```

---

### Task 2: Wire `migrate-to-docket.sh` — invoke the helper + gitignore `.claude/settings.local.json`

**Files:**
- Modify: `migrate-to-docket.sh` (header comment line ~23; add `MIGRATE_DIR` near the output helpers; step 5 gitignore loop ~325; new grant step after step 5; step 6 "next steps" ~372)
- Test: `tests/test_ensure_claude_settings.sh` (append wiring sentinels)

**Interfaces:**
- Consumes: `scripts/ensure-claude-settings.sh` (Task 1), located relative to migrate's own dir (`MIGRATE_DIR`); migrate passes the already-resolved branch via `DOCKET_INTEGRATION_BRANCH="$INTEGRATION_BRANCH"` so the helper does not re-fetch.
- Produces: a migrated repo whose `.gitignore` ignores `.claude/settings.local.json` and whose `.claude/settings.local.json` carries the grant.

- [ ] **Step 1: Write the failing wiring sentinels**

Append to `tests/test_ensure_claude_settings.sh`, immediately **before** the final `if [ "$fail" = 0 ]…` block:

```bash
# --- wiring sentinels: migrate-to-docket.sh integrates the helper + the gitignore entry ---
MIG="$REPO/migrate-to-docket.sh"
assert "migrate gitignores .claude/settings.local.json" 'grep -qF ".claude/settings.local.json" "$MIG"'
assert "migrate invokes ensure-claude-settings.sh"       'grep -qF "ensure-claude-settings.sh" "$MIG"'
```

- [ ] **Step 2: Run the test to verify the new sentinels fail**

Run: `bash tests/test_ensure_claude_settings.sh`
Expected: the two new lines print `NOT OK - migrate …`, final line `FAIL`, exit 1 (Task-1 cases still `ok`).

- [ ] **Step 3: Add `MIGRATE_DIR` (compute before the `cd "$REPO_ROOT"`)**

In `migrate-to-docket.sh`, just after the output-helpers block (the `die()` definition, ~line 38), add:

```bash
# This script's own directory — resolve siblings (scripts/ensure-claude-settings.sh) relative
# to it, since we operate on the TARGET repo ($PWD), not docket's own checkout. Computed BEFORE
# the cd into the target repo so a relative invocation path still resolves.
MIGRATE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
```

- [ ] **Step 4: Add `.claude/settings.local.json` to the step-5 gitignore loop**

In step 5, change the loop header (currently `for entry in ".docket/" ".worktrees/"; do`) to:

```bash
for entry in ".docket/" ".worktrees/" ".claude/settings.local.json"; do
```

And update that step's `step` header and the commit message so they name the new entry:

- Change `step "Ensuring .gitignore ignores .docket/ and .worktrees/"` to
  `step "Ensuring .gitignore ignores .docket/, .worktrees/, .claude/settings.local.json"`
- Change the commit message `-m "docket: gitignore .docket/ and .worktrees/ (migrate-to-docket.sh)"` to
  `-m "docket: gitignore .docket/, .worktrees/, .claude/settings.local.json (migrate-to-docket.sh)"`

(The existing slash-tolerant matcher `pat="^${entry%/}/?$"` handles the no-trailing-slash file path: `^.claude/settings.local.json/?$` matches the literal line.)

- [ ] **Step 5: Invoke the helper as a new step (after the integration-branch push, before step 6)**

In `migrate-to-docket.sh`, after the prune-worktree teardown (the `PRUNE_WT=""` line, ~361) and before the `# 6. Next steps` block, insert:

```bash
# ---------------------------------------------------------------------------
# 5b. Grant docket's terminal-publish push permission (local Claude settings).
#     Best-effort: a failure here must not fail the migration (the grant is a convenience,
#     and is recoverable by running scripts/ensure-claude-settings.sh standalone).
# ---------------------------------------------------------------------------
step "Granting docket's integration-branch push permission (local Claude settings)"
if DOCKET_INTEGRATION_BRANCH="$INTEGRATION_BRANCH" bash "$MIGRATE_DIR/scripts/ensure-claude-settings.sh"; then
  :
else
  say "  (warning: could not write the local grant; run 'bash $MIGRATE_DIR/scripts/ensure-claude-settings.sh' from this repo later.)"
fi
```

(migrate's cwd is `$REPO_ROOT`, so the helper resolves the same target repo and writes `$REPO_ROOT/.claude/settings.local.json`. `set -e` is on in migrate, hence the explicit `if … else` to keep it best-effort.)

- [ ] **Step 6: Document the grant in the step-6 "next steps"**

In the `# 6. Next steps` heredoc, add a bullet under `Next steps:`:

```
    - A Claude Code allow-rule for docket's terminal-publish push to $INTEGRATION_BRANCH has been
      written to .claude/settings.local.json (gitignored, per-user). Anyone who later CLONES this
      repo can grant themselves the same rule by running:
          bash /path/to/docket/scripts/ensure-claude-settings.sh
```

- [ ] **Step 7: Update the file-header comment (step 5 line)**

Change the header comment line `#   5. Extend .gitignore with .docket/ + .worktrees/ (idempotent).` to:

```
#   5. Extend .gitignore with .docket/ + .worktrees/ + .claude/settings.local.json (idempotent),
#      then grant docket's terminal-publish push via scripts/ensure-claude-settings.sh.
```

- [ ] **Step 8: Syntax-check migrate + run the suite**

Run: `bash -n migrate-to-docket.sh && bash tests/test_ensure_claude_settings.sh`
Expected: no syntax error; every line `ok - …`; final `PASS`; exit 0.

- [ ] **Step 9: Commit**

```bash
git add migrate-to-docket.sh tests/test_ensure_claude_settings.sh
git commit -m "feat(0027): migrate-to-docket.sh grants the push permission + gitignores the local settings

Step 5 also ignores .claude/settings.local.json (committed, so the 'never committed onto
collaborators' guarantee holds without a per-user global ignore); a new step invokes
ensure-claude-settings.sh (best-effort). Wiring sentinels added to the test."
```

---

### Task 3: Document the standalone / fresh-cloner path in the README

**Files:**
- Modify: `README.md` (the `### Migrating an existing repo` section, ~line 187)
- Test: `tests/test_ensure_claude_settings.sh` (append a doc sentinel)

**Interfaces:**
- Consumes: nothing (documentation).
- Produces: README prose telling a fresh cloner to run `scripts/ensure-claude-settings.sh` to grant themselves the push permission.

- [ ] **Step 1: Write the failing doc sentinel**

Append to `tests/test_ensure_claude_settings.sh`, immediately before the final `if [ "$fail" = 0 ]…` block (after the wiring sentinels from Task 2):

```bash
# --- doc sentinel: README documents the standalone grant path ------------------
assert "README documents scripts/ensure-claude-settings.sh" \
  'grep -qF "scripts/ensure-claude-settings.sh" "$REPO/README.md"'
```

- [ ] **Step 2: Run the test to verify the sentinel fails**

Run: `bash tests/test_ensure_claude_settings.sh`
Expected: the new line prints `NOT OK - README …`, final `FAIL`, exit 1.

- [ ] **Step 3: Add the README paragraph**

In `README.md`, in the `### Migrating an existing repo` section, after the paragraph that begins "It prints the resolved target repo and prompts for confirmation…" (the one ending "…and adds `.docket/` + `.worktrees/` to `.gitignore`. Re-running it converges from any partial state."), insert a new paragraph:

```markdown
Migration also grants one **local, per-repo** Claude Code permission: an allow-rule for docket's terminal-publish push to the integration branch (written to `.claude/settings.local.json`, which migration adds to `.gitignore`). This pre-authorizes the one push the permission classifier guards on every close-out, narrowly and only in this repo — force-pushes and pushes to other branches stay guarded. Because `settings.local.json` is gitignored and per-user, anyone who later **clones** an already-migrated repo can grant themselves the same rule by running the helper standalone:

```bash
bash /path/to/docket/scripts/ensure-claude-settings.sh
```
```

- [ ] **Step 4: Run the suite to verify it passes**

Run: `bash tests/test_ensure_claude_settings.sh`
Expected: every line `ok - …`; final `PASS`; exit 0.

- [ ] **Step 5: Commit**

```bash
git add README.md tests/test_ensure_claude_settings.sh
git commit -m "docs(0027): document the standalone ensure-claude-settings.sh grant path"
```

---

## Self-Review

**1. Spec coverage**
- §3 the permission rule (exact form, force-push/other-branch guarded) → Task 1 helper `RULE`, Global Constraints, mutation-genuineness Step 6.
- §4 the helper (resolve repo root from `$PWD`; resolve branch via `docket-config.sh`; create config if absent; idempotent jq merge preserving keys; one-line summary; no git writes; test seam) → Task 1 in full.
- §5 migrate wiring (invoke helper adjacent to step 5; mention in next steps) + reconcile addition (gitignore `.claude/settings.local.json`) → Task 2.
- §6 standalone / fresh-cloner path documented → Task 3 + Task 2 next-steps bullet.
- §7 tests (create-when-absent, idempotent count==1, preserve existing, branch main/develop, no git writes, + real-resolver fixture) → Task 1 cases (1)–(6).
- §8 dependency on #26's resolver → Task 1 consumes `docket-config.sh --export`; case (6) proves it.
- §9 ADR optional, not required → no ADR task (Step 6 of docket-implement-next will judge; spec records the per-repo-scoping decision).

**2. Placeholder scan** — none; every step shows exact code/commands and expected output.

**3. Type consistency** — the rule string `Bash(git -C * push origin HEAD:<branch>)`, the file path `.claude/settings.local.json`, the env var `DOCKET_INTEGRATION_BRANCH`, and the jq key path `.permissions.allow` are identical across helper, tests, migrate, and README.
