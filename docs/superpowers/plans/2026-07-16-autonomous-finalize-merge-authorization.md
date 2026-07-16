# Autonomous finalize merge — Action-approved PRs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let `docket-finalize-change` merge a docket feature PR headless by having a repo-controlled GitHub Actions workflow genuinely approve the PR (built-in `GITHUB_TOKEN`), so branch protection's required review is satisfied without `--admin`.

**Architecture:** Opt-in per repo via a coordination-key-fenced `finalize.auto_approve` knob (config resolver) + a human-run setup script that installs a static approve-workflow template onto the integration branch and flips the repo Actions setting. At merge time, finalize dispatches the workflow *after* its rebase-retest gate's force-push (so the approval covers the exact merged SHA), polls, verifies `reviewDecision: APPROVED`, and merges without `--admin`; any failure is abort-and-report, never an `--admin` fallback.

**Tech Stack:** POSIX/bash shell scripts (docket's `scripts/`), GitHub Actions YAML, `gh` CLI, hermetic bash test scripts under `tests/`, markdown skill/contract/doc files.

## Global Constraints

- **Spec:** `docs/superpowers/specs/2026-07-16-autonomous-finalize-merge-authorization-design.md` (read on the `docket` metadata branch / `.docket/` worktree). This plan implements spec **Tasks 2, 3, 4, 6** on the feature branch.
- **Spec Task 1 (go/no-go spike) is DONE with a GO verdict** (attended, 2026-07-16, scratch PR #93, Claude Code 2.1.211). **Do NOT re-run the spike and write NO spike code.** Its findings: under CC 2.1.211 the merge classifier does not fire headless, the Actions-bot approval satisfies branch protection, and a direct records-push to `main` is not denied headless (Arm C) — the publish-degradation path is version-defense, currently dormant.
- **Spec Task 5 (ADR) is NOT a build task in this plan.** It is recorded by `docket-implement-next`'s review step via the `docket-adr` subagent (committed on the metadata branch, published via `adrs:`). Do not author an ADR file on the feature branch.
- **Default is byte-identical to today:** `finalize.auto_approve` defaults to `false`; with it unset/false the merge step and config output must be unchanged. (learnings: `opt-in-signal-not-file-presence`)
- **`auto_approve` is coordination-key fenced** (per-repo-only, ADR-0019): it writes shared GitHub state (an approval + an unattended merge). Read from the repo-committed `.docket.yml` ONLY (no global / `.docket.local.yml` rungs); a machine-scoped value is warned-and-ignored. Mirror the existing `terminal_publish` precedent exactly.
- **Ship the knob end-to-end:** sample `.docket.yml` (commented), README/doc, and any now-relaxed prose land in the same change. (learnings: `config-knob-ship-end-to-end`)
- **Never fall back to `--admin`** on any auto-approve failure — that would silently reintroduce the bypass this design retires.
- **Test harness:** each test is a standalone `tests/test_<name>.sh` run as `bash tests/test_<name>.sh`; it prints `ok - <label>` / `NOT OK - <label>` lines and `exit`s non-zero on any failure. No network, no `gh`, no real pushes — stub via env seams (`GIT`, `SCRIPTS_DIR`, `XDG_CONFIG_HOME`, and a stubbed `gh` on `PATH`). The whole suite (every `tests/test_*.sh`) is the de-facto CI gate.
- **Shell rules (promoted learnings, always apply):** `set -uo pipefail` (or `-euo` where the file already uses it); quote expansions; `|| continue` on a conditional `mkdir` inside a loop; escape ERE metacharacters before building a `grep -E`/`sed -E` from a key; never redirect a renderer straight into the file it generates.

---

### Task 1: `finalize.auto_approve` config knob — resolver + contract + tests

Adds the coordination-key-fenced `finalize.auto_approve` knob to the config resolver, emitted as `FINALIZE_AUTO_APPROVE`. This is spec Task 4's config half; finalize's behavior wiring is Task 4 of this plan.

**Files:**
- Modify: `scripts/docket-config.sh` (Stage 2c fence loop ~line 169; a new parse block after `FINALIZE_TEST_COMMAND` ~line 194; the emit list ~line 367)
- Modify: `scripts/docket-config.md` (resolved-keys table; coordination-key fence prose; emitted-keys list)
- Test: `tests/test_docket_config.sh`

**Interfaces:**
- Produces: resolver export `FINALIZE_AUTO_APPROVE` (`true`|`false`, default `false`), consumed by `docket-finalize-change` (this plan's Task 4) and read off the `docket.sh preflight`/`env` block.

- [ ] **Step 1: Write the failing tests**

Add these assertions to `tests/test_docket_config.sh`. Put the default-value assertion in the "absent .docket.yml -> all defaults" block (near the existing `TERMINAL_PUBLISH default false` line), and add a new fixture block for the explicit / garbage / fence cases (mirror the existing `terminal_publish` fixtures in that file — grep it for `terminal_publish` to find the pattern to copy):

```bash
# --- finalize.auto_approve (change 0062) — coordination-key fenced, repo-committed only ---
# default: absent => false  (add near the other "absent cfg:" asserts, after eval of $out)
assert "absent cfg: FINALIZE_AUTO_APPROVE default false" '[ "$FINALIZE_AUTO_APPROVE" = false ]'

# explicit true in the repo-committed .docket.yml is honored
mkrepo "$tmp/aa"
printf 'finalize:\n  auto_approve: true\n' > "$tmp/aa/.docket.yml"
git -C "$tmp/aa" add .docket.yml; git -C "$tmp/aa" commit --quiet -m aa; git -C "$tmp/aa" push --quiet origin main
out="$(run "$tmp/aa" --export)"; eval "$out"
assert "repo cfg: finalize.auto_approve true honored" '[ "$FINALIZE_AUTO_APPROVE" = true ]'

# garbage value fails closed (non-zero exit)
mkrepo "$tmp/aabad"
printf 'finalize:\n  auto_approve: yes\n' > "$tmp/aabad/.docket.yml"
git -C "$tmp/aabad" add .docket.yml; git -C "$tmp/aabad" commit --quiet -m bad; git -C "$tmp/aabad" push --quiet origin main
assert "repo cfg: finalize.auto_approve garbage exits non-zero" '[ "$(rung_rc "$tmp/xdg-void" "$tmp/aabad" --export)" -ne 0 ]'

# machine-scoped value is FENCED (warned + ignored): a global config auto_approve: true does NOT flip it
mkrepo "$tmp/aafence"
gx="$tmp/aa-xdg"; mkdir -p "$gx/docket"
printf 'finalize:\n  auto_approve: true\n' > "$gx/docket/config.yml"
out="$(rung "$gx" "$tmp/aafence" --export 2>/dev/null)"; eval "$out"
assert "fence: global finalize.auto_approve ignored (stays false)" '[ "$FINALIZE_AUTO_APPROVE" = false ]'
assert "fence: global finalize.auto_approve warns on stderr" \
  'rung "$gx" "$tmp/aafence" --export 2>&1 >/dev/null | grep -qi "auto_approve.*per-repo-only"'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_config.sh`
Expected: the new `FINALIZE_AUTO_APPROVE` / fence assertions print `NOT OK` (the variable is unset/empty; no fence warning emitted). Pre-existing assertions still pass.

- [ ] **Step 3: Add the parse block to `scripts/docket-config.sh`**

Immediately after the `FINALIZE_TEST_COMMAND=...` line (~line 194), insert:

```bash
# change 0062: finalize.auto_approve — coordination-key fenced (ADR-0019), like terminal_publish.
# It grants standing permission to APPROVE + MERGE unreviewed code — shared, non-re-derivable
# GitHub state — so it is repo-committed ONLY (no lcl/gbl rungs; a machine-scoped value is
# warned-and-ignored by the Stage 2c fence below). Read as a bare leaf (safe like gate/
# test_command: `auto_approve` appears only under finalize:). Fail closed on garbage: silently
# defaulting a typo to `true` would auto-merge unreviewed code against intent.
FINALIZE_AUTO_APPROVE="$(yaml_get "$CFG" auto_approve)"; FINALIZE_AUTO_APPROVE="${FINALIZE_AUTO_APPROVE:-false}"
case "$FINALIZE_AUTO_APPROVE" in
  true|false) ;;
  *) die "unparseable .docket.yml: finalize.auto_approve must be 'true' or 'false', got '$FINALIZE_AUTO_APPROVE'" ;;
esac
```

- [ ] **Step 4: Add `auto_approve` to the Stage 2c coordination-key fence loop**

In the `for _fkey in ...` loop (~line 169), append `auto_approve` to the key list:

```bash
for _fkey in metadata_branch integration_branch changes_dir adrs_dir results_dir github_project terminal_publish auto_approve; do
```

(The loop's bare `yaml_get "$GCFG"/"$LCFG" "$_fkey"` finds a nested `finalize.auto_approve` in a machine-scoped file and prints the existing `... is per-repo-only ...` warning — no new code needed.)

- [ ] **Step 5: Emit the new key**

In the `--export` emit block, after `emit FINALIZE_TEST_COMMAND "$FINALIZE_TEST_COMMAND"` (~line 367), add:

```bash
  emit FINALIZE_AUTO_APPROVE "$FINALIZE_AUTO_APPROVE"
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `bash tests/test_docket_config.sh`
Expected: all `ok -` (including the new auto_approve default / explicit / garbage / fence assertions). No `NOT OK`.

- [ ] **Step 7: Update the contract `scripts/docket-config.md`**

1. In the resolved-keys table (the block with the `gate`/`test_command` rows ~lines 103-104), add a row:

```
| `auto_approve` (finalize) | `false` | **no (per-repo-only)** | read from `finalize.auto_approve` leaf key; repo-committed `.docket.yml` ONLY — coordination-key fenced (writes shared GitHub state); `true`/`false`, fails closed |
```

2. In the coordination-key fence prose list (~lines 152-154, the sentence enumerating `metadata_branch`, `integration_branch`, `changes_dir`, ... `terminal_publish`), add `finalize.auto_approve` to the enumerated set.

3. In the emitted-keys list (the block listing `FINALIZE_GATE` / `FINALIZE_TEST_COMMAND` ~lines 252-253), add `FINALIZE_AUTO_APPROVE`.

- [ ] **Step 8: Run the config-contract coverage + example tests**

Run: `bash tests/test_docket_config.sh && bash tests/test_config_example.sh`
Expected: both exit 0 (all `ok -`). If `test_config_example.sh` asserts the emitted-key set, it now includes `FINALIZE_AUTO_APPROVE`.

- [ ] **Step 9: Commit**

```bash
git add scripts/docket-config.sh scripts/docket-config.md tests/test_docket_config.sh
git commit -m "feat(0062): finalize.auto_approve config knob (coordination-key fenced)"
```

---

### Task 2: `docket-approve.yml` workflow template + template test

The static GitHub Actions workflow that approves a docket feature PR with the built-in `GITHUB_TOKEN`. Shipped in the docket clone; the setup script (Task 3) copies it onto a repo's integration branch.

**Files:**
- Create: `scripts/templates/docket-approve.yml`
- Test: `tests/test_docket_approve_template.sh`

**Interfaces:**
- Produces: the template at `scripts/templates/docket-approve.yml`, resolved by `setup-auto-approve.sh` (Task 3) as `<script-dir>/templates/docket-approve.yml`, installed to `<repo>/.github/workflows/docket-approve.yml`. Dispatched by finalize (Task 4) as `gh workflow run docket-approve.yml -f pr=<N>`.

- [ ] **Step 1: Write the failing test**

Create `tests/test_docket_approve_template.sh`:

```bash
#!/usr/bin/env bash
# tests/test_docket_approve_template.sh — structural checks on the shipped approve-workflow
# template (change 0062). No network; a static-content audit (grep sentinels). Run: bash tests/test_docket_approve_template.sh
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
TPL="$ROOT/scripts/templates/docket-approve.yml"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "template file exists"                 '[ -f "$TPL" ]'
assert "workflow_dispatch trigger"            'grep -q "workflow_dispatch" "$TPL"'
assert "required pr input"                    'grep -Eq "pr:" "$TPL" && grep -q "required: true" "$TPL"'
assert "job-scoped pull-requests: write"      'grep -Eq "pull-requests:[[:space:]]*write" "$TPL"'
assert "guard: open state"                    'grep -q "OPEN" "$TPL"'
assert "guard: draft rejected"                'grep -qi "draft" "$TPL"'
assert "guard: fork rejected"                 'grep -qiE "fork|isCrossRepository" "$TPL"'
assert "guard: feat/* head shape"             'grep -q "feat/\*" "$TPL"'
assert "approves via gh pr review --approve"  'grep -q "gh pr review" "$TPL" && grep -q -- "--approve" "$TPL"'
assert "uses GITHUB_TOKEN"                    'grep -q "GITHUB_TOKEN" "$TPL"'
# a static template must NOT hardcode a specific repo/owner (kept byte-identical across installs)
assert "no hardcoded repo owner"              '! grep -qiE "danielhanold/docket" "$TPL"'

exit $fail
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_approve_template.sh`
Expected: FAIL — `NOT OK - template file exists` (and the rest), because the template does not exist yet.

- [ ] **Step 3: Create the template**

Create `scripts/templates/docket-approve.yml`:

```yaml
# docket-approve.yml — installed by `docket.sh setup-auto-approve` (docket change 0062).
# Approves a docket feature PR with the built-in GITHUB_TOKEN so branch protection's required
# review is satisfied WITHOUT --admin, and the merge is no longer "without review". A github-
# actions[bot] review counts toward required approvals (it does NOT satisfy CODEOWNERS).
# STATIC TEMPLATE: keep byte-identical across installs so re-running setup updates it cleanly.
# Guards below fail LOUD so a denial is diagnosable from the run log.
name: docket-approve

on:
  workflow_dispatch:
    inputs:
      pr:
        description: "PR number to approve"
        required: true
        type: string

permissions:
  pull-requests: write

jobs:
  approve:
    runs-on: ubuntu-latest
    steps:
      - name: Guard eligibility and approve
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          PR: ${{ inputs.pr }}
          REPO: ${{ github.repository }}
        run: |
          set -euo pipefail
          data="$(gh pr view "$PR" --repo "$REPO" \
            --json state,isDraft,isCrossRepository,headRefName)"
          state="$(printf '%s' "$data" | jq -r '.state')"
          draft="$(printf '%s' "$data" | jq -r '.isDraft')"
          cross="$(printf '%s' "$data" | jq -r '.isCrossRepository')"
          head="$(printf '%s' "$data" | jq -r '.headRefName')"
          # Guard 1: PR must be OPEN.
          [ "$state" = "OPEN" ]  || { echo "::error::PR #$PR is not open (state=$state)"; exit 1; }
          # Guard 2: not a draft.
          [ "$draft" = "false" ] || { echo "::error::PR #$PR is a draft"; exit 1; }
          # Guard 3: head repo == base repo (never bot-approve a fork PR).
          [ "$cross" = "false" ] || { echo "::error::PR #$PR is from a fork"; exit 1; }
          # Guard 4: head branch matches docket's feat/* shape.
          case "$head" in
            feat/*) : ;;
            *) echo "::error::PR #$PR head '$head' is not a docket feat/* branch"; exit 1 ;;
          esac
          gh pr review "$PR" --repo "$REPO" --approve \
            --body "docket auto-approve: rebase-retest gate passed (workflow_dispatch)"
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_docket_approve_template.sh`
Expected: PASS — all `ok -`, no `NOT OK`.

- [ ] **Step 5: Commit**

```bash
git add scripts/templates/docket-approve.yml tests/test_docket_approve_template.sh
git commit -m "feat(0062): ship docket-approve.yml workflow template"
```

---

### Task 3: `setup-auto-approve.sh` + contract + facade wiring + tests

The one-time, human-attended setup: install the workflow onto the integration branch and flip the repo Actions setting. Reached as `docket.sh setup-auto-approve`. NEVER invoked by an autonomous skill.

**Files:**
- Create: `scripts/setup-auto-approve.sh`
- Create: `scripts/setup-auto-approve.md`
- Modify: `scripts/docket.sh` (`WRAPPED_OPS` array ~line 37; the Usage comment block ~lines 9-26)
- Modify: `scripts/docket.md` (Usage comment ~lines 24-26; the Subcommand inventory table ~lines 39-56)
- Test: `tests/test_setup_auto_approve.sh`
- Test (extend): `tests/test_docket_facade.sh` (the inventory sentinel already derives both sides by grep — verify it stays green; add a routing assertion)

**Interfaces:**
- Consumes: `scripts/templates/docket-approve.yml` (Task 2).
- Produces: facade op `setup-auto-approve` → `scripts/setup-auto-approve.sh`. Flags: `--integration-branch <B>` (optional; default = `origin/HEAD`'s branch), `--remote <R>` (default `origin`). Mock seams: `GIT` (git binary), `GH` (gh binary).

- [ ] **Step 1: Write the failing tests**

Create `tests/test_setup_auto_approve.sh` (hermetic — stub `gh`, use a real local bare-origin repo, never touch the network):

```bash
#!/usr/bin/env bash
# tests/test_setup_auto_approve.sh — hermetic tests for scripts/setup-auto-approve.sh (change
# 0062). Real local git (bare origin + clone); gh stubbed via the GH seam. No network.
# Run: bash tests/test_setup_auto_approve.sh
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SCRIPT="$ROOT/scripts/setup-auto-approve.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# --- a gh stub that records calls and serves a fixed actions-permissions payload -------------
ghstub="$tmp/ghbin"; mkdir -p "$ghstub"
cat > "$ghstub/gh" <<'STUB'
#!/usr/bin/env bash
echo "gh $*" >> "$GH_LOG"
case "$1 $2" in
  "api -X")  # a PUT — record and succeed
    exit 0 ;;
esac
# a plain `gh api repos/.../actions/permissions/workflow` GET: return current settings
if [ "$1" = "api" ] && printf '%s' "$*" | grep -q "permissions/workflow"; then
  echo '{"default_workflow_permissions":"read","can_approve_pull_request_reviews":false}'
  exit 0
fi
if [ "$1" = "repo" ] && [ "$2" = "view" ]; then echo "acme/widget"; exit 0; fi
exit 0
STUB
chmod +x "$ghstub/gh"

# --- a bare origin + clone with main + a docket orphan ---------------------------------------
mkrepo(){
  local dir="$1" bare="$1.origin.git"
  git init --quiet --bare "$bare"
  git clone --quiet "$bare" "$dir" 2>/dev/null
  git -C "$dir" config user.email t@t.test; git -C "$dir" config user.name Test
  git -C "$dir" checkout --quiet -b main; : > "$dir/README.md"
  git -C "$dir" add README.md; git -C "$dir" commit --quiet -m init
  git -C "$dir" push --quiet -u origin main
  git -C "$dir" remote set-head origin -a >/dev/null 2>&1
}
mkrepo "$tmp/r"
export GH_LOG="$tmp/gh.log"; : > "$GH_LOG"
runsetup(){ ( cd "$tmp/r" && GH="$ghstub/gh" GH_LOG="$GH_LOG" bash "$SCRIPT" "$@" ); }

# (A) installs the workflow file onto the integration branch (pushed to origin/main)
out="$(runsetup --integration-branch main 2>&1)"; rc=$?
assert "setup exits 0" '[ "$rc" -eq 0 ]'
assert "workflow landed on origin/main" \
  'git -C "$tmp/r" ls-tree -r --name-only origin/main | grep -qx ".github/workflows/docket-approve.yml"'

# (B) read-modify-write: PUT preserves default_workflow_permissions=read, sets approve=true
assert "PUT sends can_approve_pull_request_reviews=true" 'grep -q "can_approve_pull_request_reviews=true" "$GH_LOG"'
assert "PUT preserves default_workflow_permissions=read" 'grep -q "default_workflow_permissions=read" "$GH_LOG"'

# (C) prints the reminder to set finalize.auto_approve in .docket.yml
assert "reminds about finalize.auto_approve knob" 'printf "%s" "$out" | grep -q "finalize.auto_approve"'

# (D) idempotent: a second run still exits 0 and leaves exactly one workflow file
out2="$(runsetup --integration-branch main 2>&1)"; rc2=$?
assert "second run idempotent (exit 0)" '[ "$rc2" -eq 0 ]'
assert "still exactly one workflow file" \
  '[ "$(git -C "$tmp/r" ls-tree -r --name-only origin/main | grep -c "docket-approve.yml")" -eq 1 ]'

# (E) leaves no leftover setup worktree
assert "no leftover setup worktree" '! git -C "$tmp/r" worktree list | grep -q "setup-approve"'

exit $fail
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_setup_auto_approve.sh`
Expected: FAIL — the script does not exist yet, so every assertion after `runsetup` fails.

- [ ] **Step 3: Create `scripts/setup-auto-approve.sh`**

```bash
#!/usr/bin/env bash
# scripts/setup-auto-approve.sh — one-time, HUMAN-ATTENDED setup for finalize's auto-approve
# (change 0062). (1) Installs scripts/templates/docket-approve.yml onto the integration branch as
# .github/workflows/docket-approve.yml (direct admin push — same posture as terminal-publish);
# (2) flips the repo Actions setting can_approve_pull_request_reviews=true via `gh api` PUT,
# preserving default_workflow_permissions (read-modify-write, never blind-set); (3) prints what it
# changed and reminds the human to set finalize.auto_approve: true in .docket.yml.
# NEVER invoked by an autonomous skill. Idempotent. Contract: scripts/setup-auto-approve.md.
# Mock seams: GIT, GH.
set -uo pipefail

GIT="${GIT:-git}"
GH="${GH:-gh}"
REMOTE="origin"
INT_BRANCH=""
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE="$SELF_DIR/templates/docket-approve.yml"

die(){ printf 'setup-auto-approve: %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --integration-branch) [ $# -ge 2 ] || die "--integration-branch needs an arg"; INT_BRANCH="$2"; shift ;;
    --remote)             [ $# -ge 2 ] || die "--remote needs an arg"; REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done

[ -f "$TEMPLATE" ] || die "template not found: $TEMPLATE (broken install?)"
$GIT rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not a git repo"

# Resolve the integration branch from origin/HEAD when not given.
if [ -z "$INT_BRANCH" ]; then
  $GIT remote set-head "$REMOTE" -a >/dev/null 2>&1 || true
  INT_BRANCH="$($GIT symbolic-ref --quiet --short "refs/remotes/$REMOTE/HEAD" 2>/dev/null | sed "s#^$REMOTE/##")"
  [ -n "$INT_BRANCH" ] || die "could not resolve integration branch from $REMOTE/HEAD — pass --integration-branch"
fi

$GIT fetch "$REMOTE" "$INT_BRANCH" >/dev/null 2>&1 || die "fetch $REMOTE/$INT_BRANCH failed"

# --- (1) install the workflow onto the integration branch via a transient worktree -----------
pub="$($GIT rev-parse --show-toplevel)/.setup-approve-wt"
teardown(){
  $GIT -C "$pub" checkout --detach >/dev/null 2>&1 || true
  $GIT worktree remove --force "$pub" >/dev/null 2>&1 || true
  $GIT branch -D setup-approve >/dev/null 2>&1 || true
}
$GIT worktree prune
$GIT worktree add -B setup-approve "$pub" "$REMOTE/$INT_BRANCH" >/dev/null 2>&1 \
  || die "could not provision setup-approve worktree"
# Skip the team's shared hooks on docket's own asset commit (best-effort).
"$SELF_DIR/disable-worktree-hooks.sh" --worktree "$pub" >/dev/null 2>&1 || true

mkdir -p "$pub/.github/workflows" || { teardown; die "mkdir .github/workflows failed"; }
cp "$TEMPLATE" "$pub/.github/workflows/docket-approve.yml" || { teardown; die "copy template failed"; }
$GIT -C "$pub" add .github/workflows/docket-approve.yml

if $GIT -C "$pub" diff --cached --quiet; then
  echo "setup-auto-approve: workflow already up to date on $INT_BRANCH (no commit needed)"
else
  $GIT -C "$pub" commit -m "chore(docket): install docket-approve.yml auto-approve workflow" >/dev/null \
    || { teardown; die "commit failed"; }
  # Push HEAD explicitly; surface the workflow-scope caveat on rejection (HTTPS token auth needs it).
  if ! $GIT -C "$pub" push "$REMOTE" "HEAD:$INT_BRANCH" 2>"$pub/.push.err"; then
    if grep -qi "workflow" "$pub/.push.err"; then
      teardown
      die "push rejected — pushing .github/workflows/ over HTTPS needs the 'workflow' OAuth scope; re-auth with that scope (gh auth refresh -s workflow) or use an SSH remote, then re-run"
    fi
    teardown; die "push to $REMOTE/$INT_BRANCH failed: $(cat "$pub/.push.err")"
  fi
  echo "setup-auto-approve: installed .github/workflows/docket-approve.yml on $INT_BRANCH"
fi
teardown
$GIT worktree prune

# --- (2) flip the repo Actions setting (read-modify-write) ------------------------------------
slug="$($GH repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || $GH repo view 2>/dev/null | head -n1)"
[ -n "$slug" ] || die "could not resolve owner/repo via gh"
cur="$($GH api "repos/$slug/actions/permissions/workflow" 2>/dev/null)" \
  || die "could not read Actions permissions (need repo admin + a token with 'repo' scope)"
dwp="$(printf '%s' "$cur" | sed -n 's/.*"default_workflow_permissions":"\([^"]*\)".*/\1/p')"
dwp="${dwp:-read}"
$GH api -X PUT "repos/$slug/actions/permissions/workflow" \
  -f "default_workflow_permissions=$dwp" \
  -F "can_approve_pull_request_reviews=true" >/dev/null \
  || die "could not set can_approve_pull_request_reviews (org policy may override the repo setting)"
echo "setup-auto-approve: set can_approve_pull_request_reviews=true on $slug (default_workflow_permissions=$dwp preserved)"

# --- (3) reminder ----------------------------------------------------------------------------
cat <<EOF
setup-auto-approve: done. Next:
  - Set 'finalize.auto_approve: true' in this repo's committed .docket.yml (this script never edits committed config).
  - Verify: gh api repos/$slug/actions/permissions/workflow
EOF
```

Notes for the implementer: match the exact stderr/stdout token strings the test greps for (`finalize.auto_approve`, `can_approve_pull_request_reviews=true`, `default_workflow_permissions=read`, `workflow`). The `-f`/`-F` distinction in the `gh api` PUT is intentional — the test only checks the two substrings appear in the recorded call, so either flag is fine as long as both `key=value` strings are emitted.

- [ ] **Step 4: Wire the facade — `scripts/docket.sh`**

Append `setup-auto-approve` to the `WRAPPED_OPS` array (~line 37):

```bash
WRAPPED_OPS="docket-status board-refresh archive-change terminal-publish cleanup-feature-branch github-mirror sync-integration-branch render-change-links render-adr-index render-learnings-index adr-checks board-checks runner-dispatch setup-auto-approve"
```

Add a Usage-comment line (in the `#   ...` block ~lines 10-26, after `runner-dispatch`):

```bash
#   setup-auto-approve        one-time, human-attended install of the auto-approve workflow + repo setting
```

- [ ] **Step 5: Document the op — `scripts/docket.md`**

Add a matching Usage-comment line (mirroring the docket.sh one) and a Subcommand-inventory table row (after the `runner-dispatch` row ~line 56):

```
| `setup-auto-approve` | `setup-auto-approve.sh` | one-time, human-attended install of the auto-approve workflow onto the integration branch + the repo Actions setting (change 0062) |
```

(The `test_docket_facade.sh` inventory sentinel derives both op sets by grep — `WRAPPED_OPS` vs the `| \`op\` |` cells — so both edits together keep it green.)

- [ ] **Step 6: Create the contract `scripts/setup-auto-approve.md`**

Author a Purpose / Usage / Behavior / Exit codes / Invariants contract (match the shape of `scripts/terminal-publish.md`). It MUST state: human-attended only / never autonomous; the two writes (workflow file onto the integration branch; the `can_approve_pull_request_reviews` PUT via read-modify-write preserving `default_workflow_permissions`); idempotency; the workflow-OAuth-scope push caveat and its surfaced hint; and that it never edits committed `.docket.yml` (prints a reminder instead). (`test_script_contracts_coverage.sh` requires this file to exist alongside the `.sh`.)

- [ ] **Step 7: Add a facade routing assertion + run the facade tests**

In `tests/test_docket_facade.sh`, add `setup-auto-approve` to the stub-helper `for h in ...` list so a routing assertion can exercise it, then add near the other routing asserts:

```bash
out="$(SCRIPTS_DIR="$stub" bash "$FACADE" setup-auto-approve --integration-branch main 2>/dev/null)"
assert "setup-auto-approve routes to its helper with args" \
  '[ "$out" = "CALLED setup-auto-approve --integration-branch main" ]'
```

Run: `bash tests/test_docket_facade.sh && bash tests/test_script_contracts_coverage.sh && bash tests/test_setup_auto_approve.sh`
Expected: all three exit 0. The inventory sentinel (`docket.sh op set == docket.md documented op set`) passes with the new op on both sides.

- [ ] **Step 8: Commit**

```bash
git add scripts/setup-auto-approve.sh scripts/setup-auto-approve.md scripts/docket.sh scripts/docket.md tests/test_setup_auto_approve.sh tests/test_docket_facade.sh
git commit -m "feat(0062): setup-auto-approve script + facade op + contract"
```

---

### Task 4: finalize integration — merge-gate wiring + publish degradation

Wire `finalize.auto_approve` into `docket-finalize-change`'s rebase-retest merge gate and document the `terminal_publish` headless publish-degradation. This is a prose (SKILL.md) change; tests are grep sentinels, matching the existing `tests/test_finalize_gate.sh` pattern.

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (the rebase-retest gate Flow, ~lines 98-108; the `.docket.yml` snippet ~lines 86-92; the Terminal publish section ~lines 131-137)
- Test: `tests/test_finalize_gate.sh`

**Interfaces:**
- Consumes: `FINALIZE_AUTO_APPROVE` (Task 1); the `docket-approve.yml` workflow (Tasks 2-3); `terminal-publish` degradation.

- [ ] **Step 1: Write the failing sentinels**

Add to `tests/test_finalize_gate.sh` (it already sets `FIN="$REPO/skills/docket-finalize-change/SKILL.md"` — reuse that variable; grep it to confirm the name):

```bash
# --- auto_approve merge path (change 0062) ---
assert "finalize documents finalize.auto_approve knob" \
  'grep -q "auto_approve" "$FIN"'
assert "auto_approve dispatches the approve workflow after the gate's push, before merge" \
  'grep -Eqi "docket-approve|gh workflow run" "$FIN"'
assert "auto_approve merges WITHOUT --admin" \
  'grep -Eqi "without .*--admin|no .*--admin|not .*--admin" "$FIN"'
assert "auto_approve re-checks reviewDecision == APPROVED" \
  'grep -q "reviewDecision" "$FIN" && grep -q "APPROVED" "$FIN"'
assert "auto_approve failure is abort-and-report, never an --admin fallback" \
  'grep -Eqi "never .*--admin|no --admin fallback|abort-and-report" "$FIN"'
assert "publish degradation: terminal_publish headless push denial degrades, not fails" \
  'grep -qi "terminal_publish" "$FIN" && grep -Eqi "degrad|surface.*manual|run .*terminal-publish" "$FIN"'
```

- [ ] **Step 2: Run to verify failure**

Run: `bash tests/test_finalize_gate.sh`
Expected: the six new assertions print `NOT OK` (the SKILL.md has no auto_approve wiring yet); pre-existing assertions pass.

- [ ] **Step 3: Wire the merge gate in `skills/docket-finalize-change/SKILL.md`**

In *The rebase-retest merge gate* Flow (the numbered list ~lines 98-108), replace step 6 (`gh pr merge → the existing close-out ...`) with an auto_approve-aware step. Insert after step 5 (Push `--force-with-lease`):

```markdown
6. **Approve, if `finalize.auto_approve` is `true` and the PR is not already `APPROVED`.** After
   the gate's push (step 5), so the approval covers the exact rebased SHA:
   1. `gh workflow run docket-approve.yml --ref <integration_branch> -f pr=<N>`.
   2. Poll the dispatched run to completion (bounded; identify it by workflow name + `--ref`
      branch + recency — `gh run list` returns no id from `workflow run`).
   3. Re-check `reviewDecision == APPROVED` (bounded retries — the bot review lands a beat after
      the run finishes).
   4. Merge **without** `--admin`.
   Any failure in 6.1–6.3 (dispatch rejected, run failed/timed out, approval never materialized)
   is **abort-and-report**: leave the PR open, surface the reason (and record it as a PR comment),
   and **never** fall back to `--admin` — that would silently reintroduce the bypass this retires.
   When `auto_approve` is `false` (default), or the PR is already approved, this step is a no-op and
   the merge proceeds exactly as today.
7. `gh pr merge` (with `--admin` only on the pre-existing explicit-id / attended paths, never under
   auto_approve) → the existing close-out (harvest → archive → terminal-publish → cleanup → board).
```

- [ ] **Step 4: Update the `.docket.yml` snippet + add the require_pr_approval note**

In the gate's `.docket.yml` code block (~lines 86-92), add the knob under `finalize:`:

```yaml
  auto_approve: false         # default false. true => headless finalize dispatches docket-approve.yml
                              #   after the rebase-retest gate's push, verifies reviewDecision:APPROVED,
                              #   and merges WITHOUT --admin. Requires `docket.sh setup-auto-approve`.
                              #   Coordination-key fenced (per-repo-only). Any failure aborts; never --admin.
```

And add a sentence after the `require_pr_approval` prose (~line 96): under `auto_approve: true` a passing approval proves *docket's pipeline signed off* (the review step + rebase-retest gate), not human review — so `require_pr_approval: true` combined with `auto_approve: true` is satisfiable by the bot's own review; cross-reference the new ADR (recorded at review time).

- [ ] **Step 5: Document the publish degradation in the Terminal publish section**

In *Terminal publish (docket-mode)* (~lines 131-137), append: on a **headless** run with `terminal_publish: true`, a records-push denied by an agent permission classifier does **not** fail the run — finalize completes archive + cleanup + board and surfaces `terminal-publish blocked (auto-mode push denial) — run docket.sh terminal-publish --id <id> (and --adr <NN> for any published ADR) manually` in its report. Attended runs are unaffected (the human's conversational intent clears the push as today). Note this is version-defense: the 2026-07-16 spike (CC 2.1.211) found the push arm did not fire headless.

- [ ] **Step 6: Run to verify pass**

Run: `bash tests/test_finalize_gate.sh`
Expected: all `ok -`, no `NOT OK`.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md tests/test_finalize_gate.sh
git commit -m "feat(0062): wire finalize.auto_approve into the merge gate + publish degradation"
```

---

### Task 5: setup documentation + README link + sample `.docket.yml` knob

Ship the knob end-to-end (learnings: `config-knob-ship-end-to-end`): a setup guide, its README link, and the commented knob in this repo's `.docket.yml`.

**Files:**
- Create: `docs/auto-approve-setup.md`
- Modify: `README.md` (link the guide)
- Modify: `.docket.yml` (commented `finalize.auto_approve` entry in the finalize block)
- Test: `tests/test_config_example.sh` (extend if it audits `.docket.yml` finalize keys) — otherwise a small new grep test

**Interfaces:** none (docs only).

- [ ] **Step 1: Write the failing checks**

Add a small doc-wiring test. If `tests/test_config_example.sh` already reads `.docket.yml`/README, extend it; otherwise create `tests/test_auto_approve_docs.sh`:

```bash
#!/usr/bin/env bash
# tests/test_auto_approve_docs.sh — ship-the-knob-end-to-end wiring for finalize.auto_approve
# (change 0062). Run: bash tests/test_auto_approve_docs.sh
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "setup guide exists"                    '[ -f "$ROOT/docs/auto-approve-setup.md" ]'
assert "README links the setup guide"          'grep -q "auto-approve-setup.md" "$ROOT/README.md"'
assert ".docket.yml documents auto_approve"    'grep -q "auto_approve" "$ROOT/.docket.yml"'
assert "guide covers setup-auto-approve run"   'grep -q "setup-auto-approve" "$ROOT/docs/auto-approve-setup.md"'
assert "guide covers CODEOWNERS limitation"    'grep -qi "CODEOWNERS" "$ROOT/docs/auto-approve-setup.md"'
assert "guide covers workflow OAuth scope"     'grep -qi "workflow" "$ROOT/docs/auto-approve-setup.md"'

exit $fail
```

- [ ] **Step 2: Run to verify failure**

Run: `bash tests/test_auto_approve_docs.sh`
Expected: FAIL — the guide/README link/`.docket.yml` entry don't exist yet.

- [ ] **Step 3: Create `docs/auto-approve-setup.md`**

Write the guide covering (from spec Task 6): prerequisites (repo admin; a classic `repo`-scoped token, fine-grained equivalents; the SSH-vs-HTTPS `workflow`-scope caveat; the "Allow Actions to create and approve pull requests" setting); the one-time `docket.sh setup-auto-approve` run (what it changes, how to verify with the `gh api` read-back, idempotency); enabling `finalize.auto_approve: true` and what finalize then does at merge; and honest limitations (CODEOWNERS unsupported; org Actions policy can override the repo setting; `require_pr_approval: true` becomes bot-satisfiable — link the ADR; the `terminal_publish: true` headless degradation).

- [ ] **Step 4: Link it from `README.md`**

Add a line to the README's docs/links section pointing at `docs/auto-approve-setup.md` (e.g., under a "Headless / autonomous finalize" bullet). Follow the README's existing link style.

- [ ] **Step 5: Add the commented knob to `.docket.yml`**

In the `finalize:` block of the repo-root `.docket.yml` (next to the commented `require_pr_approval` line), add:

```yaml
  # auto_approve: false  # true => headless finalize dispatches .github/workflows/docket-approve.yml
  #                      #   (install once via `docket.sh setup-auto-approve`) to approve the PR, then
  #                      #   merges WITHOUT --admin. Coordination-key fenced (per-repo-only). See
  #                      #   docs/auto-approve-setup.md.
```

- [ ] **Step 6: Run to verify pass**

Run: `bash tests/test_auto_approve_docs.sh`
Expected: all `ok -`.

- [ ] **Step 7: Commit**

```bash
git add docs/auto-approve-setup.md README.md .docket.yml tests/test_auto_approve_docs.sh
git commit -m "docs(0062): auto-approve setup guide + README link + sample knob"
```

---

## Self-Review (completed by the plan author)

**Spec coverage:**
- Spec Task 1 (spike) — DONE-GO; explicitly not re-run (Global Constraints). ✓
- Spec Task 2 (`docket-approve.yml` template) — plan Task 2. ✓
- Spec Task 3 (`setup-auto-approve.sh` + facade) — plan Task 3. ✓
- Spec Task 4 (finalize integration + `auto_approve` knob + publish degradation) — plan Task 1 (config) + Task 4 (finalize wiring). ✓
- Spec Task 5 (ADR) — produced by docket-implement-next's review step via the `docket-adr` subagent, NOT a feature-branch build task (Global Constraints). ✓
- Spec Task 6 (setup docs + README) — plan Task 5. ✓

**Placeholder scan:** all code/test steps carry concrete content; no TBD/TODO. Prose skill/doc/contract steps (Tasks 3 §6, 4 §3-5, 5 §3-4) describe the required content precisely and are gated by grep-sentinel tests. ✓

**Type/name consistency:** the facade op name `setup-auto-approve` == helper basename `setup-auto-approve.sh`; the resolver export `FINALIZE_AUTO_APPROVE`; the workflow file `docket-approve.yml` and its dispatch input `pr`; the template path `scripts/templates/docket-approve.yml` — consistent across Tasks 1-5. ✓

**Ordering:** Task 1 (config export) precedes Task 4 (finalize reads it); Task 2 (template) precedes Task 3 (setup installs it). Independent tasks otherwise. ✓
