# Config Resolution + Bootstrap Guard Script Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lift docket's per-skill startup boilerplate (config resolution + the docket-mode bootstrap guard) out of model-token prose into one deterministic, fixture-tested `scripts/docket-config.sh` that emits eval-able `KEY=value` output, and rewire the skills to consume it.

**Architecture:** A single bash script with three internal stages sharing one fetch — (1) resolve `origin/HEAD` + default branch, (2) read & resolve `.docket.yml`, (3) evaluate the `DOCKET`/`LIVE` 2×2 — emitting `KEY=value` lines for `eval "$(scripts/docket-config.sh --export)"`. Read-only by default; the lone write (create+push the empty orphan `docket` on a fresh repo) is opt-in via `--bootstrap`, guarded to the `¬DOCKET ∧ ¬LIVE` cell. Fail-closed: non-zero exit + stderr diagnostic on any hard error, keyed on `fetch`/`set-head` return codes — never on `git show` (a cached `origin/HEAD` lets `git show` succeed with stale bytes). Semantics are reproduced verbatim from ADR-0002 + the convention; no new ADR.

**Tech Stack:** POSIX-ish bash (matches the repo's existing `scripts/*.sh`), git plumbing, `sed`/`grep`. No `yq`, no network beyond git fetch. Tests are standalone `bash tests/test_docket_config.sh` with hermetic temp-repo + bare-origin fixtures (no real network), matching the existing suite.

## Global Constraints

- **Script conventions (copy the existing house style):** `#!/usr/bin/env bash`; `set -uo pipefail`; mock seam `GIT="${GIT:-git}"`; `-h|--help` prints the header via `grep '^#' "$0" | sed 's/^# \{0,1\}//'`; unknown arg → `exit 2`; helper `die(){ printf 'docket-config: %s\n' "$*" >&2; exit 1; }`.
- **Read-only by default.** The default `--export` invocation performs only the benign `git fetch` + `git remote set-head` (and reads); it NEVER mutates branches. The single write (orphan `docket` create+push) fires ONLY under `--bootstrap` AND ONLY in the `¬DOCKET ∧ ¬LIVE` cell.
- **Fail-closed, keyed on return codes.** Hard errors → non-zero exit + stderr diagnostic, NO `KEY=value` emitted: unreachable `origin` (fetch rc≠0), unresolvable `origin/HEAD` (set-head rc≠0 / empty symref), ref-absent integration branch (`git ls-tree` rc≠0), `metadata_branch` neither `docket` nor `main`. Abort decisions key on `fetch`/`set-head` rc, NEVER on `git show` succeeding.
- **`.docket.yml` is parsed with a flat scalar reader** (ported verbatim from `migrate-to-docket.sh`'s `yaml_get`); no YAML dependency. Nested `finalize.gate` / `finalize.test_command` are read by their unique leaf-key name (they appear nowhere else in the file). `agents:` is out of scope.
- **Semantics are ADR-0002 + convention, reproduced — never redesigned.** Defaults: `metadata_branch: docket`, `integration_branch: auto`, `changes_dir: docs/changes`, `adrs_dir: docs/adrs`, `results_dir: docs/results`, `finalize.gate: local`, `board_surfaces: [inline]`, `auto_groom: false`. `integration_branch: auto → origin/HEAD → main`.
- **Output contract (exact keys):** `DOCKET_MODE`, `DEFAULT_BRANCH`, `METADATA_BRANCH`, `INTEGRATION_BRANCH`, `METADATA_WORKTREE`, `CHANGES_DIR`, `ADRS_DIR`, `RESULTS_DIR`, `FINALIZE_GATE`, `FINALIZE_TEST_COMMAND`, `BOARD_SURFACES`, `AUTO_GROOM`, `BOOTSTRAP`. Values are shell-escaped via `printf '%s=%q\n'` so `eval` is safe with spaces/empties. `BOOTSTRAP ∈ {PROCEED, STOP_MIGRATE, CREATE_ORPHAN}`.
- **Commit discipline:** frequent commits, one per task. Commit messages end with the trailer `Claude-Session: https://claude.ai/code/session_01E9K1NsUS9bNuhcUFTF1HNh`.
- **Test discipline (from LEARNINGS):** validate `KEY=value` output against a **direct-pipe** caller, not only `$(…)` (`$()` strips a dropped trailing newline — #22). Fixtures use **real-shaped values** and **plurality** (#22). Cover the **guarded write branch** by mutation-confirming the write does NOT fire in the wrong cells (#25). One assert anchors exactly one clause (#21/#2). When editing skill prose, leave name-based cross-refs intact (#20).

---

## File Structure

- **Create `scripts/docket-config.sh`** — the resolver. Sole responsibility: resolve config + bootstrap verdict, emit `KEY=value`, optionally perform the opt-in orphan write. Self-contained (does NOT source `scripts/lib/docket-frontmatter.sh` — that lib reads `---`-delimited change-file frontmatter, not a plain `.docket.yml`; this script carries its own flat `yaml_get`).
- **Create `tests/test_docket_config.sh`** — hermetic fixture suite for every resolution permutation, all four bootstrap cells, the opt-in write (and its guard), the fail-closed error paths, and the skill-wiring sentinels.
- **Modify `skills/docket-convention/SKILL.md`** — the *Configuration* and *Bootstrap guard* sections gain a line naming `scripts/docket-config.sh --export` as the implementation of the resolution they specify (the prose stays as the contract).
- **Modify each operating skill's Step 0** — `skills/docket-implement-next/SKILL.md`, `skills/docket-status/SKILL.md`, `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-adr/SKILL.md`, `skills/docket-auto-groom/SKILL.md` — add a one-line directive to resolve config via the script.

---

### Task 1: Config resolution + `--export` contract (scaffold)

Builds the script through stage 2 (origin/HEAD + `.docket.yml` resolution) and the full `KEY=value` emit. Bootstrap is stubbed to `PROCEED` (correct for main-mode; the docket-mode 2×2 lands in Task 2). Tests assert config-key correctness via **main-mode** and explicit-config fixtures (where `BOOTSTRAP=PROCEED` is unconditionally correct), plus the absent-file defaults.

**Files:**
- Create: `scripts/docket-config.sh`
- Test: `tests/test_docket_config.sh`

**Interfaces:**
- Consumes: nothing (entry point).
- Produces: the executable `scripts/docket-config.sh` and these emitted keys (consumed by Tasks 2–5 and the skills): `DOCKET_MODE`, `DEFAULT_BRANCH`, `METADATA_BRANCH`, `INTEGRATION_BRANCH`, `METADATA_WORKTREE`, `CHANGES_DIR`, `ADRS_DIR`, `RESULTS_DIR`, `FINALIZE_GATE`, `FINALIZE_TEST_COMMAND`, `BOARD_SURFACES`, `AUTO_GROOM`, `BOOTSTRAP`. Helpers later tasks extend: `die()`, `g()` (= `"$GIT" -C "$REPO_DIR"`), `yaml_get()`, `emit()`.

- [ ] **Step 1: Write the failing test (config resolution — defaults, main-mode, explicit, escaping)**

Create `tests/test_docket_config.sh`:

```bash
#!/usr/bin/env bash
# tests/test_docket_config.sh — hermetic fixtures for scripts/docket-config.sh (change 0026).
# Run: bash tests/test_docket_config.sh   (no network; temp repos + bare origins)
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SCRIPT="$REPO/scripts/docket-config.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# --- fixture builder: a clone with a bare origin -----------------------------
# mkrepo <dir> : create a bare origin + a working clone at <dir>, identity set,
#   one commit on `main` (origin/HEAD -> main). Echoes nothing; populates $dir.
mkrepo(){
  local dir="$1" bare="$1.origin.git"
  git init --quiet --bare "$bare"
  git clone --quiet "$bare" "$dir"
  git -C "$dir" config user.email t@t.test
  git -C "$dir" config user.name  Test
  git -C "$dir" checkout --quiet -b main
  : > "$dir/README.md"
  git -C "$dir" add README.md
  git -C "$dir" commit --quiet -m init
  git -C "$dir" push --quiet -u origin main
  git -C "$dir" remote set-head origin -a >/dev/null 2>&1
}
# run <dir> [args...] : run the resolver against <dir>, echo stdout
run(){ local d="$1"; shift; bash "$SCRIPT" --repo-dir "$d" "$@"; }

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# --- (A) absent .docket.yml -> all defaults (docket-mode) --------------------
mkrepo "$tmp/a"
out="$(run "$tmp/a" --export)"; eval "$out"
assert "absent cfg: METADATA_BRANCH default docket"    '[ "$METADATA_BRANCH" = docket ]'
assert "absent cfg: DOCKET_MODE docket"                '[ "$DOCKET_MODE" = docket ]'
assert "absent cfg: METADATA_WORKTREE .docket"         '[ "$METADATA_WORKTREE" = .docket ]'
assert "absent cfg: INTEGRATION_BRANCH auto->main"     '[ "$INTEGRATION_BRANCH" = main ]'
assert "absent cfg: DEFAULT_BRANCH main"               '[ "$DEFAULT_BRANCH" = main ]'
assert "absent cfg: CHANGES_DIR default"               '[ "$CHANGES_DIR" = docs/changes ]'
assert "absent cfg: ADRS_DIR default"                  '[ "$ADRS_DIR" = docs/adrs ]'
assert "absent cfg: RESULTS_DIR default"               '[ "$RESULTS_DIR" = docs/results ]'
assert "absent cfg: FINALIZE_GATE default local"       '[ "$FINALIZE_GATE" = local ]'
assert "absent cfg: FINALIZE_TEST_COMMAND empty"       '[ -z "$FINALIZE_TEST_COMMAND" ]'
assert "absent cfg: BOARD_SURFACES default inline"     '[ "$BOARD_SURFACES" = inline ]'
assert "absent cfg: AUTO_GROOM default false"          '[ "$AUTO_GROOM" = false ]'

# --- (B) main-mode pin -> METADATA_WORKTREE '.', BOOTSTRAP PROCEED -----------
mkrepo "$tmp/b"
cat > "$tmp/b/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
EOF
git -C "$tmp/b" add .docket.yml; git -C "$tmp/b" commit --quiet -m cfg
git -C "$tmp/b" push --quiet origin main
out="$(run "$tmp/b" --export)"; eval "$out"
assert "main-mode: METADATA_BRANCH main"               '[ "$METADATA_BRANCH" = main ]'
assert "main-mode: DOCKET_MODE main"                   '[ "$DOCKET_MODE" = main ]'
assert "main-mode: METADATA_WORKTREE dot"              '[ "$METADATA_WORKTREE" = . ]'
assert "main-mode: BOOTSTRAP PROCEED"                  '[ "$BOOTSTRAP" = PROCEED ]'

# --- (C) explicit config (main-mode to skip bootstrap): dirs, gate, surfaces, escaping
mkrepo "$tmp/c"
cat > "$tmp/c/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: develop
changes_dir: planning/changes
adrs_dir: planning/adrs
results_dir: planning/results
auto_groom: true
board_surfaces: [inline, github]
finalize:
  gate: ci
  test_command: go test ./... -count=1
EOF
git -C "$tmp/c" add .docket.yml; git -C "$tmp/c" commit --quiet -m cfg
git -C "$tmp/c" push --quiet origin main
out="$(run "$tmp/c" --export)"; eval "$out"
assert "explicit: INTEGRATION_BRANCH verbatim develop" '[ "$INTEGRATION_BRANCH" = develop ]'
assert "explicit: CHANGES_DIR override"                '[ "$CHANGES_DIR" = planning/changes ]'
assert "explicit: ADRS_DIR override"                   '[ "$ADRS_DIR" = planning/adrs ]'
assert "explicit: RESULTS_DIR override"                '[ "$RESULTS_DIR" = planning/results ]'
assert "explicit: AUTO_GROOM true"                     '[ "$AUTO_GROOM" = true ]'
assert "explicit: FINALIZE_GATE ci"                    '[ "$FINALIZE_GATE" = ci ]'
assert "explicit: BOARD_SURFACES two (plurality)"      '[ "$BOARD_SURFACES" = "inline github" ]'
assert "explicit: FINALIZE_TEST_COMMAND w/ spaces"     '[ "$FINALIZE_TEST_COMMAND" = "go test ./... -count=1" ]'

# --- (D) board_surfaces: [] -> disabled (empty), distinct from unset ---------
mkrepo "$tmp/d"
printf 'metadata_branch: main\nboard_surfaces: []\n' > "$tmp/d/.docket.yml"
git -C "$tmp/d" add .docket.yml; git -C "$tmp/d" commit --quiet -m cfg
git -C "$tmp/d" push --quiet origin main
out="$(run "$tmp/d" --export)"; eval "$out"
assert "board []: BOARD_SURFACES empty"                '[ -z "$BOARD_SURFACES" ]'

# --- (E) direct-pipe caller (LEARNINGS #22: $() hides a dropped trailing \n) -
n="$(run "$tmp/c" --export | grep -c '=')"
assert "direct-pipe: 13 KEY=value lines emitted"       '[ "$n" -eq 13 ]'
last="$(run "$tmp/c" --export | tail -n1)"
assert "direct-pipe: last line is BOOTSTRAP"           'case "$last" in BOOTSTRAP=*) true;; *) false;; esac'

if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi
exit "$fail"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_config.sh`
Expected: FAIL — `scripts/docket-config.sh` does not exist yet (every `run` errors; assertions report NOT OK).

- [ ] **Step 3: Write minimal implementation (config resolution stage)**

Create `scripts/docket-config.sh`:

```bash
#!/usr/bin/env bash
# scripts/docket-config.sh — deterministic resolver for docket's startup config + bootstrap
# guard (change 0026). Emits eval-able KEY=value lines a skill consumes in one turn:
#   eval "$(scripts/docket-config.sh --export)"
# Read-only by default (only the benign git fetch + set-head); the lone write — create+push
# the empty orphan `docket` on a fresh repo — is opt-in (--bootstrap), guarded to the
# ¬DOCKET ∧ ¬LIVE cell. Fail-closed: non-zero + stderr diagnostic on a hard error
# (unreachable origin, unresolvable origin/HEAD, ref-absent integration branch, bad
# metadata_branch). Abort keys on the fetch/set-head return code, NEVER on git show
# (a cached origin/HEAD lets git show succeed with stale bytes). Semantics are ADR-0002 +
# the convention's Configuration / Bootstrap guard, implemented verbatim — no new ADR.
#
# Usage: docket-config.sh [--export] [--bootstrap] [--repo-dir DIR]
#   --export        emit resolved KEY=value lines (default mode)
#   --bootstrap     additionally perform the CREATE_ORPHAN write when the verdict is
#                   CREATE_ORPHAN (fresh repo); a no-op in every other cell
#   --repo-dir DIR  operate on the git repo at DIR (default: .) — the test/mock seam
#   -h, --help      print this header
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

GIT="${GIT:-git}"
MODE=export
DO_BOOTSTRAP=0
REPO_DIR="."
while [ $# -gt 0 ]; do
  case "$1" in
    --export)    MODE=export ;;
    --bootstrap) DO_BOOTSTRAP=1 ;;
    --repo-dir)  REPO_DIR="$2"; shift ;;
    -h|--help)   grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'docket-config: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done

die() { printf 'docket-config: %s\n' "$*" >&2; exit 1; }
g()   { "$GIT" -C "$REPO_DIR" "$@"; }
emit(){ printf '%s=%q\n' "$1" "$2"; }

# Minimal flat scalar reader for `key: value` (strips inline #comments, quotes, whitespace).
# Ported from migrate-to-docket.sh — .docket.yml is intentionally a flat scalar file (no yq).
# Nested finalize.gate / finalize.test_command are read by their unique leaf-key name.
yaml_get() {  # yaml_get <file> <key>  -> value on stdout (empty if key absent)
  [ -f "$1" ] || return 1
  sed -n -E "s/^[[:space:]]*$2[[:space:]]*:[[:space:]]*([^#]*).*/\1/p" "$1" \
    | head -n1 | sed -E 's/[[:space:]]+$//; s/^"(.*)"$/\1/; s/^'\''(.*)'\''$/\1/'
}

# --- Stage 1: resolve origin/HEAD + default branch (keyed on fetch/set-head rc) ---
g rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not a git repo: $REPO_DIR"
g fetch --quiet origin 2>/dev/null || die "cannot reach origin (git fetch failed) — check the remote/network"
g remote set-head origin -a >/dev/null 2>&1 || die "cannot resolve origin/HEAD (git remote set-head failed)"
DEFAULT_BRANCH="$(g symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null || true)"
DEFAULT_BRANCH="${DEFAULT_BRANCH#origin/}"
[ -n "$DEFAULT_BRANCH" ] || die "origin/HEAD is unresolvable after set-head"

# --- Stage 2: read + resolve .docket.yml (authoritative via git show origin/HEAD) ---
CFG="$(mktemp)"; trap 'rm -f "$CFG"' EXIT
g show "origin/HEAD:.docket.yml" >"$CFG" 2>/dev/null || : >"$CFG"   # absent file => defaults (NOT an error)

METADATA_BRANCH="$(yaml_get "$CFG" metadata_branch)"; METADATA_BRANCH="${METADATA_BRANCH:-docket}"
case "$METADATA_BRANCH" in
  docket) DOCKET_MODE=docket; METADATA_WORKTREE=.docket ;;
  main)   DOCKET_MODE=main;   METADATA_WORKTREE=. ;;
  *) die "unparseable .docket.yml: metadata_branch must be 'docket' or 'main', got '$METADATA_BRANCH'" ;;
esac

INTEGRATION_BRANCH="$(yaml_get "$CFG" integration_branch)"
if [ -z "$INTEGRATION_BRANCH" ] || [ "$INTEGRATION_BRANCH" = auto ]; then
  INTEGRATION_BRANCH="$DEFAULT_BRANCH"
fi

CHANGES_DIR="$(yaml_get "$CFG" changes_dir)"; CHANGES_DIR="${CHANGES_DIR:-docs/changes}"
ADRS_DIR="$(yaml_get "$CFG" adrs_dir)";       ADRS_DIR="${ADRS_DIR:-docs/adrs}"
RESULTS_DIR="$(yaml_get "$CFG" results_dir)"; RESULTS_DIR="${RESULTS_DIR:-docs/results}"
FINALIZE_GATE="$(yaml_get "$CFG" gate)";      FINALIZE_GATE="${FINALIZE_GATE:-local}"
FINALIZE_TEST_COMMAND="$(yaml_get "$CFG" test_command)"
AUTO_GROOM="$(yaml_get "$CFG" auto_groom)";   AUTO_GROOM="${AUTO_GROOM:-false}"

bs_raw="$(yaml_get "$CFG" board_surfaces)"
if [ -z "$bs_raw" ]; then
  BOARD_SURFACES="inline"                                  # unset => default [inline]
else
  bs="${bs_raw#[}"; bs="${bs%]}"; bs="${bs//,/ }"
  BOARD_SURFACES="$(echo $bs)"                             # trim/collapse; "[]" => ""
fi

# --- Stage 3: bootstrap verdict (Task 2 replaces the docket-mode branch with the 2×2) ---
BOOTSTRAP=PROCEED

# --- emit ---
if [ "$MODE" = export ]; then
  emit DOCKET_MODE "$DOCKET_MODE"
  emit DEFAULT_BRANCH "$DEFAULT_BRANCH"
  emit METADATA_BRANCH "$METADATA_BRANCH"
  emit INTEGRATION_BRANCH "$INTEGRATION_BRANCH"
  emit METADATA_WORKTREE "$METADATA_WORKTREE"
  emit CHANGES_DIR "$CHANGES_DIR"
  emit ADRS_DIR "$ADRS_DIR"
  emit RESULTS_DIR "$RESULTS_DIR"
  emit FINALIZE_GATE "$FINALIZE_GATE"
  emit FINALIZE_TEST_COMMAND "$FINALIZE_TEST_COMMAND"
  emit BOARD_SURFACES "$BOARD_SURFACES"
  emit AUTO_GROOM "$AUTO_GROOM"
  emit BOOTSTRAP "$BOOTSTRAP"
fi
```

Then make it executable: `chmod +x scripts/docket-config.sh`.

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_docket_config.sh`
Expected: PASS (all `ok -` lines; final `PASS`). Fixtures A–E exercise defaults, main-mode, explicit overrides, `[]`-disabled, and the direct-pipe line count.

- [ ] **Step 5: Commit**

```bash
git add scripts/docket-config.sh tests/test_docket_config.sh
git commit -m "feat(0026): docket-config.sh — config resolution + KEY=value export

Claude-Session: https://claude.ai/code/session_01E9K1NsUS9bNuhcUFTF1HNh"
```

---

### Task 2: Bootstrap guard 2×2 (docket-mode)

Replaces the `BOOTSTRAP=PROCEED` stub with the real `DOCKET`/`LIVE` evaluation in docket-mode (main-mode stays `PROCEED`). Read-only — it only evaluates and reports the verdict.

**Files:**
- Modify: `scripts/docket-config.sh` (Stage 3)
- Test: `tests/test_docket_config.sh`

**Interfaces:**
- Consumes: `DOCKET_MODE`, `INTEGRATION_BRANCH`, `CHANGES_DIR` from Task 1.
- Produces: `BOOTSTRAP ∈ {PROCEED, STOP_MIGRATE, CREATE_ORPHAN}`; the fixture helpers `seed_live`, `make_docket`, `make_fresh` reused by Tasks 3–4.

- [ ] **Step 1: Write the failing test (all four cells)**

Add to `tests/test_docket_config.sh`, immediately before the final `if [ "$fail" = 0 ]` block. These helpers build the four bootstrap states on top of `mkrepo` (which already leaves a docket-mode repo with `origin/main` carrying only `README.md`):

```bash
# --- bootstrap 2×2 fixtures (docket-mode; mkrepo leaves origin/main = README only) ---
# seed_live <dir> : put the live planning surface on origin/main (=> LIVE=1)
seed_live(){
  local d="$1"
  mkdir -p "$d/docs/changes/active"
  : > "$d/docs/changes/active/0001-x.md"
  : > "$d/docs/changes/README.md"
  : > "$d/docs/changes/BOARD.md"
  git -C "$d" add docs; git -C "$d" commit --quiet -m live
  git -C "$d" push --quiet origin main
}
# make_docket <dir> : create an empty origin/docket (=> DOCKET=1) without a local branch
make_docket(){
  local d="$1" t c
  t="$(git -C "$d" mktree </dev/null)"
  c="$(git -C "$d" commit-tree "$t" -m seed)"
  git -C "$d" push --quiet origin "$c:refs/heads/docket"
  git -C "$d" fetch --quiet origin docket
}

# (B1) migrated: DOCKET ∧ ¬LIVE -> PROCEED
mkrepo "$tmp/b1"; make_docket "$tmp/b1"
out="$(run "$tmp/b1" --export)"; eval "$out"
assert "2x2 migrated -> PROCEED"            '[ "$BOOTSTRAP" = PROCEED ]'

# (B2) fresh: ¬DOCKET ∧ ¬LIVE -> CREATE_ORPHAN
mkrepo "$tmp/b2"
out="$(run "$tmp/b2" --export)"; eval "$out"
assert "2x2 fresh -> CREATE_ORPHAN"         '[ "$BOOTSTRAP" = CREATE_ORPHAN ]'

# (B3) existing single-branch: ¬DOCKET ∧ LIVE -> STOP_MIGRATE
mkrepo "$tmp/b3"; seed_live "$tmp/b3"
out="$(run "$tmp/b3" --export)"; eval "$out"
assert "2x2 single-branch -> STOP_MIGRATE"  '[ "$BOOTSTRAP" = STOP_MIGRATE ]'

# (B4) half-migrated: DOCKET ∧ LIVE -> STOP_MIGRATE
mkrepo "$tmp/b4"; seed_live "$tmp/b4"; make_docket "$tmp/b4"
out="$(run "$tmp/b4" --export)"; eval "$out"
assert "2x2 half-migrated -> STOP_MIGRATE"  '[ "$BOOTSTRAP" = STOP_MIGRATE ]'
```

Also update fixture (E)'s expected line count if needed — it stays 13 (the key set is unchanged). No edit required.

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_config.sh`
Expected: FAIL — B2/B3/B4 report NOT OK (current stub always emits `PROCEED`). B1 happens to pass (migrated is `PROCEED`).

- [ ] **Step 3: Write minimal implementation (the 2×2)**

In `scripts/docket-config.sh`, replace the Stage 3 stub:

```bash
# --- Stage 3: bootstrap verdict (Task 2 replaces the docket-mode branch with the 2×2) ---
BOOTSTRAP=PROCEED
```

with:

```bash
# --- Stage 3: bootstrap guard — evaluate the DOCKET/LIVE 2×2 (docket-mode only) ---
BOOTSTRAP=PROCEED
if [ "$DOCKET_MODE" = docket ]; then
  # DOCKET = the docket branch exists (origin OR local)
  if g rev-parse --verify --quiet refs/remotes/origin/docket >/dev/null 2>&1 \
     || g rev-parse --verify --quiet refs/heads/docket >/dev/null 2>&1; then
    DOCKET=1; else DOCKET=0; fi
  # LIVE = the pruned live planning surface still sits on the integration branch.
  # ls-tree exit≠0 => the ref is absent/unreadable => HARD config error, NOT ¬LIVE.
  live_out="$(g ls-tree "origin/$INTEGRATION_BRANCH" -- \
              "$CHANGES_DIR/active" "$CHANGES_DIR/README.md" "$CHANGES_DIR/BOARD.md" 2>/dev/null)"
  rc=$?
  [ "$rc" -eq 0 ] || die "cannot read origin/$INTEGRATION_BRANCH (git ls-tree exit $rc) — integration_branch ref absent/unreadable (config error, not ¬LIVE)"
  [ -n "$live_out" ] && LIVE=1 || LIVE=0
  if   [ "$DOCKET" -eq 1 ] && [ "$LIVE" -eq 0 ]; then BOOTSTRAP=PROCEED        # migrated
  elif [ "$DOCKET" -eq 0 ] && [ "$LIVE" -eq 0 ]; then BOOTSTRAP=CREATE_ORPHAN  # fresh
  else BOOTSTRAP=STOP_MIGRATE   # ¬DOCKET∧LIVE (single-branch) | DOCKET∧LIVE (half-migrated)
  fi
fi
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_docket_config.sh`
Expected: PASS (all four cells now report `ok -`, plus the Task 1 assertions).

- [ ] **Step 5: Commit**

```bash
git add scripts/docket-config.sh tests/test_docket_config.sh
git commit -m "feat(0026): bootstrap guard 2×2 (DOCKET/LIVE) verdict

Claude-Session: https://claude.ai/code/session_01E9K1NsUS9bNuhcUFTF1HNh"
```

---

### Task 3: Opt-in `--bootstrap` orphan-create write

Adds the single mutation: under `--bootstrap`, in the `CREATE_ORPHAN` cell only, create + push an empty orphan `docket` and re-report `PROCEED`. Mutation-tested both ways (it fires in the fresh cell; it does NOT fire by default, nor in any other cell — LEARNINGS #25).

**Files:**
- Modify: `scripts/docket-config.sh` (add `create_orphan`, wire it into Stage 3)
- Test: `tests/test_docket_config.sh`

**Interfaces:**
- Consumes: `BOOTSTRAP`, `DO_BOOTSTRAP`, `g()` from Tasks 1–2.
- Produces: `create_orphan()` (pushes `<empty-commit>:refs/heads/docket` to origin, leaves no local branch); post-write `BOOTSTRAP=PROCEED`.

- [ ] **Step 1: Write the failing test (write fires only in the fresh cell)**

Add to `tests/test_docket_config.sh`, before the final block. `origin_has_docket <dir>` checks the bare origin directly:

```bash
# --- opt-in --bootstrap write (the only mutation; guarded to ¬DOCKET ∧ ¬LIVE) ---
origin_has_docket(){ git -C "$1.origin.git" rev-parse --verify --quiet refs/heads/docket >/dev/null 2>&1; }

# (W1) default --export in fresh cell: NO write, verdict CREATE_ORPHAN
mkrepo "$tmp/w1"
out="$(run "$tmp/w1" --export)"; eval "$out"
assert "read-only default: no orphan created" '! origin_has_docket "$tmp/w1"'
assert "read-only default: verdict CREATE_ORPHAN" '[ "$BOOTSTRAP" = CREATE_ORPHAN ]'

# (W2) --bootstrap in fresh cell: creates origin/docket, re-reports PROCEED
mkrepo "$tmp/w2"
out="$(run "$tmp/w2" --bootstrap --export)"; eval "$out"
assert "bootstrap fresh: origin/docket created" 'origin_has_docket "$tmp/w2"'
assert "bootstrap fresh: verdict now PROCEED"   '[ "$BOOTSTRAP" = PROCEED ]'

# (W3) --bootstrap in STOP_MIGRATE cell: GUARD holds — no orphan written
mkrepo "$tmp/w3"; seed_live "$tmp/w3"
out="$(run "$tmp/w3" --bootstrap --export)"; eval "$out"
assert "bootstrap guard: no write in single-branch cell" '! origin_has_docket "$tmp/w3"'
assert "bootstrap guard: verdict stays STOP_MIGRATE"     '[ "$BOOTSTRAP" = STOP_MIGRATE ]'

# (W4) --bootstrap in migrated cell: idempotent no-op, PROCEED
mkrepo "$tmp/w4"; make_docket "$tmp/w4"
out="$(run "$tmp/w4" --bootstrap --export)"; eval "$out"
assert "bootstrap migrated: PROCEED"            '[ "$BOOTSTRAP" = PROCEED ]'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_config.sh`
Expected: FAIL — W2 reports NOT OK (no write performed yet; `origin/docket` absent, verdict still `CREATE_ORPHAN`). W1/W3/W4 pass.

- [ ] **Step 3: Write minimal implementation (the orphan write)**

In `scripts/docket-config.sh`, add `create_orphan` next to the other helpers (after `yaml_get`):

```bash
# Create an empty orphan `docket` and push to origin. Worktree-free (empty-tree root
# commit via plumbing) and leaves NO local branch: we push the commit straight to
# origin's refs/heads/docket, then fetch so refs/remotes/origin/docket is populated.
create_orphan() {
  local tree commit
  tree="$(g mktree </dev/null)" || die "mktree failed"
  commit="$(g commit-tree "$tree" -m 'docket: initialize empty orphan metadata branch')" \
    || die "commit-tree failed — is git user.name/email set?"
  g push origin "$commit:refs/heads/docket" >/dev/null 2>&1 \
    || die "could not push orphan docket to origin"
  g fetch --quiet origin docket 2>/dev/null || true
}
```

Then, inside the `if [ "$DOCKET_MODE" = docket ]; then` block, after the 2×2 assigns `BOOTSTRAP`, append:

```bash
  if [ "$DO_BOOTSTRAP" -eq 1 ] && [ "$BOOTSTRAP" = CREATE_ORPHAN ]; then
    create_orphan
    BOOTSTRAP=PROCEED   # the repo is now migrated; the caller may proceed
  fi
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_docket_config.sh`
Expected: PASS — W1 (read-only default, no write), W2 (write fires, PROCEED), W3 (guard holds, no write), W4 (idempotent) all `ok -`.

- [ ] **Step 5: Commit**

```bash
git add scripts/docket-config.sh tests/test_docket_config.sh
git commit -m "feat(0026): opt-in --bootstrap orphan-create write (guarded to fresh cell)

Claude-Session: https://claude.ai/code/session_01E9K1NsUS9bNuhcUFTF1HNh"
```

---

### Task 4: Fail-closed error paths

Locks the hard-error behavior: non-zero exit + stderr diagnostic + NO `KEY=value` on unreachable origin, ref-absent integration branch, and bad `metadata_branch` — and proves the abort keys on the `fetch`/`set-head` return code, not on `git show` (a cached `origin/HEAD` must NOT mask an unreachable origin).

**Files:**
- Test: `tests/test_docket_config.sh` (the implementation from Tasks 1–2 already fails closed; this task verifies and, if a gap is found, tightens it)

**Interfaces:**
- Consumes: the `die()` paths from Tasks 1–2.
- Produces: no new code expected (verification task); if a case leaks, add the minimal guard.

- [ ] **Step 1: Write the failing test (error paths)**

Add to `tests/test_docket_config.sh`, before the final block. `run_rc` captures exit code; `run_err` captures stderr:

```bash
# --- fail-closed error paths (non-zero exit, stderr diagnostic, no KEY=value) ----
run_rc(){ local d="$1"; shift; bash "$SCRIPT" --repo-dir "$d" "$@" >/dev/null 2>&1; echo $?; }

# (F1) unreachable origin -> exit≠0, no output
mkrepo "$tmp/f1"
rm -rf "$tmp/f1.origin.git"                       # destroy the remote
assert "unreachable origin: nonzero exit" '[ "$(run_rc "$tmp/f1" --export)" -ne 0 ]'
assert "unreachable origin: emits nothing" '[ -z "$(bash "$SCRIPT" --repo-dir "$tmp/f1" --export 2>/dev/null)" ]'

# (F2) cached-but-stale origin/HEAD must NOT mask an unreachable origin (keys on fetch rc,
#      not git show — LEARNINGS / spec §7). origin/HEAD + .docket.yml are cached locally,
#      so `git show origin/HEAD:.docket.yml` would still succeed with stale bytes.
mkrepo "$tmp/f2"
echo 'metadata_branch: docket' > "$tmp/f2/.docket.yml"
git -C "$tmp/f2" add .docket.yml; git -C "$tmp/f2" commit --quiet -m cfg
git -C "$tmp/f2" push --quiet origin main
git -C "$tmp/f2" fetch --quiet origin              # populate caches
rm -rf "$tmp/f2.origin.git"                         # now unreachable
assert "stale cache does not mask unreachable origin" '[ "$(run_rc "$tmp/f2" --export)" -ne 0 ]'

# (F3) integration ref absent (docket-mode) -> ls-tree rc≠0 -> hard error
mkrepo "$tmp/f3"
printf 'metadata_branch: docket\nintegration_branch: nope\n' > "$tmp/f3/.docket.yml"
git -C "$tmp/f3" add .docket.yml; git -C "$tmp/f3" commit --quiet -m cfg
git -C "$tmp/f3" push --quiet origin main
assert "absent integration ref: nonzero exit" '[ "$(run_rc "$tmp/f3" --export)" -ne 0 ]'

# (F4) bad metadata_branch -> unparseable -> hard error
mkrepo "$tmp/f4"
echo 'metadata_branch: banana' > "$tmp/f4/.docket.yml"
git -C "$tmp/f4" add .docket.yml; git -C "$tmp/f4" commit --quiet -m cfg
git -C "$tmp/f4" push --quiet origin main
assert "bad metadata_branch: nonzero exit" '[ "$(run_rc "$tmp/f4" --export)" -ne 0 ]'
err="$(bash "$SCRIPT" --repo-dir "$tmp/f4" --export 2>&1 >/dev/null)"
assert "bad metadata_branch: diagnostic mentions metadata_branch" 'printf "%s" "$err" | grep -q metadata_branch'
```

- [ ] **Step 2: Run test to verify it fails or passes**

Run: `bash tests/test_docket_config.sh`
Expected: the Task 1–2 implementation should already make F1–F4 PASS (it fails closed by construction). If any reports NOT OK, that is a real gap — proceed to Step 3. If all PASS, note "no code change needed" and skip to Step 5.

- [ ] **Step 3: Tighten only if a case leaked**

If F1/F2 leaked (origin unreachable but exit 0): confirm Stage 1 runs `g fetch --quiet origin 2>/dev/null || die ...` BEFORE any `git show`, so the abort keys on fetch rc. If F3 leaked: confirm the `ls-tree` rc check `[ "$rc" -eq 0 ] || die ...` is present. If F4 leaked: confirm the `case "$METADATA_BRANCH"` default arm calls `die`. Apply the minimal missing guard; do not restructure passing paths.

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_docket_config.sh`
Expected: PASS (F1–F4 all `ok -`).

- [ ] **Step 5: Commit**

```bash
git add tests/test_docket_config.sh scripts/docket-config.sh
git commit -m "test(0026): fail-closed error paths (unreachable origin, stale cache, ref-absent, bad knob)

Claude-Session: https://claude.ai/code/session_01E9K1NsUS9bNuhcUFTF1HNh"
```

---

### Task 5: Rewire the skills' Step 0 to invoke the resolver

The canonical resolution + bootstrap prose is centralized in `docket-convention`; each operating skill loads it at Step 0. This task names `scripts/docket-config.sh --export` as the implementation in the convention's two sections and adds a one-line directive to each operating skill's Step 0 — exactly the inline-naming pattern `docket-status` already uses for `render-board.sh` / `github-mirror.sh`. Doc-sentinel tests lock the wiring; the existing doc-sentinel suites are re-run so an edit can't silently invalidate a neighbor's sentinel (LEARNINGS #20).

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (*Configuration* + *Bootstrap guard* sections)
- Modify: `skills/docket-implement-next/SKILL.md`, `skills/docket-status/SKILL.md`, `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-adr/SKILL.md`, `skills/docket-auto-groom/SKILL.md` (Step 0 / startup)
- Test: `tests/test_docket_config.sh` (wiring sentinels)

**Interfaces:**
- Consumes: the `--export` contract + `BOOTSTRAP` verdict from Tasks 1–3 (the prose references them).
- Produces: skill prose that names `scripts/docket-config.sh`; grep sentinels asserting it.

- [ ] **Step 1: Write the failing test (wiring sentinels)**

Add to `tests/test_docket_config.sh`, before the final block. One assert per file, each anchored to the single clause it owns (LEARNINGS #21/#2):

```bash
# --- skill-wiring sentinels (the SKILLs are code on the integration branch) ------
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "convention names docket-config.sh" 'grep -qF "scripts/docket-config.sh" "$CONV"'
for s in docket-implement-next docket-status docket-new-change docket-groom-next \
         docket-finalize-change docket-adr docket-auto-groom; do
  f="$REPO/skills/$s/SKILL.md"
  assert "$s Step 0 invokes docket-config.sh" 'grep -qF "scripts/docket-config.sh" "$f"'
done
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_config.sh`
Expected: FAIL — all 8 wiring sentinels report NOT OK (no skill names the script yet).

- [ ] **Step 3: Rewire the convention (single source of the contract)**

In `skills/docket-convention/SKILL.md`, in the **Configuration — `.docket.yml`** section, after the sentence describing authoritative reads (`Read config authoritatively via git show origin/HEAD:.docket.yml ...`), add:

```markdown
This resolution — repair `origin/HEAD`, read `.docket.yml`, apply every default, and resolve `integration_branch` — is performed by the deterministic **`scripts/docket-config.sh --export`**, which a skill consumes in one turn: `eval "$(scripts/docket-config.sh --export)"`. It emits the resolved knobs as eval-able `KEY=value` lines (`DOCKET_MODE`, `DEFAULT_BRANCH`, `METADATA_BRANCH`, `INTEGRATION_BRANCH`, `METADATA_WORKTREE`, `CHANGES_DIR`, `ADRS_DIR`, `RESULTS_DIR`, `FINALIZE_GATE`, `FINALIZE_TEST_COMMAND`, `BOARD_SURFACES`, `AUTO_GROOM`, `BOOTSTRAP`) and is **fail-closed** (non-zero exit + diagnostic on a hard error — unreachable `origin`, unresolvable `origin/HEAD`, ref-absent `integration_branch`, or a `metadata_branch` that is neither `docket` nor `main`). The prose in this section is the contract the script implements verbatim; the script is the single implementation the skills run instead of re-deriving it each session.
```

In the **Bootstrap guard (`docket`-mode first-run safety)** section, after the 2×2 table, add:

```markdown
This 2×2 is evaluated by the same `scripts/docket-config.sh` and reported as its `BOOTSTRAP=` verdict — `PROCEED` (migrated or main-mode), `STOP_MIGRATE` (existing single-branch or half-migrated), or `CREATE_ORPHAN` (fresh). The default `--export` invocation is **read-only** (it reports the verdict); the skill acts on it — STOP and point at `migrate-to-docket.sh` on `STOP_MIGRATE`, or opt into the orphan-create write (`scripts/docket-config.sh --bootstrap`, guarded to the `¬DOCKET ∧ ¬LIVE` cell) on `CREATE_ORPHAN`. The probe definitions above remain the contract the script implements.
```

- [ ] **Step 4: Rewire each operating skill's Step 0**

In each of the seven operating skill files, add the directive at the point where the skill resolves config at startup (its blocking Step 0 / convention-load step — e.g. just after "invoke the `docket-convention` skill"). Use this exact sentence (the `grep -qF "scripts/docket-config.sh"` sentinel keys on it):

```markdown
Resolve config + the bootstrap verdict deterministically: `eval "$(scripts/docket-config.sh --export)"` (fail-closed; read-only). Act on `BOOTSTRAP` — `PROCEED` to continue; `STOP_MIGRATE` to refuse-and-point at `migrate-to-docket.sh`; `CREATE_ORPHAN` to opt into `scripts/docket-config.sh --bootstrap` (fresh repo only).
```

For each file, place it where startup resolution belongs:
- `skills/docket-implement-next/SKILL.md` — in **Step 0 — Sync & sweep**, before the `.docket/` sync.
- `skills/docket-status/SKILL.md` — in the startup/config step before the three passes.
- `skills/docket-new-change/SKILL.md` — in the startup/config step before "Where everything is read and written".
- `skills/docket-groom-next/SKILL.md` — in its Step 0 (config & dependencies).
- `skills/docket-finalize-change/SKILL.md` — in its startup/config step before the merge gate.
- `skills/docket-adr/SKILL.md` — in its startup step.
- `skills/docket-auto-groom/SKILL.md` — in its startup/config step.

Do not delete the existing convention-load line or the `.docket/` sync prose — this directive sits alongside them (the resolver replaces the *resolution* derivation, not the worktree sync or the convention load). Preserve every existing name-based cross-reference (LEARNINGS #20).

- [ ] **Step 5: Run the wiring test AND the full doc-sentinel suite**

Run:
```bash
bash tests/test_docket_config.sh
bash tests/test_convention_extraction.sh
bash tests/test_composition_wiring.sh
bash tests/test_render_board.sh
bash tests/test_closeout.sh
bash tests/test_finalize_gate.sh
```
Expected: all PASS. `tests/test_docket_config.sh` now passes its 8 wiring sentinels. The other suites assert convention/skill prose that my edits sit alongside — they must stay green. If any sentinel in those suites breaks, my edit displaced prose it anchored to: restore the displaced text (add my sentence adjacent, do not overwrite) until green. Do NOT weaken another test to pass.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-convention/SKILL.md skills/docket-implement-next/SKILL.md \
        skills/docket-status/SKILL.md skills/docket-new-change/SKILL.md \
        skills/docket-groom-next/SKILL.md skills/docket-finalize-change/SKILL.md \
        skills/docket-adr/SKILL.md skills/docket-auto-groom/SKILL.md \
        tests/test_docket_config.sh
git commit -m "feat(0026): rewire skills' Step 0 to invoke docket-config.sh resolver

Claude-Session: https://claude.ai/code/session_01E9K1NsUS9bNuhcUFTF1HNh"
```

---

## Final verification (after all tasks)

- [ ] Run the new suite: `bash tests/test_docket_config.sh` → `PASS`.
- [ ] Run the full suite to catch regressions (each is standalone; there is no runner):
  ```bash
  for t in tests/test_*.sh; do echo "== $t =="; bash "$t" | tail -1; done
  ```
  Every file must end in `PASS`.
- [ ] **Real-data smoke test (LEARNINGS #22 — fixture is necessary, not sufficient).** Run the resolver against THIS repo (docket-mode, migrated) and eyeball the output:
  ```bash
  scripts/docket-config.sh --repo-dir . --export
  ```
  Expected (this repo): `DOCKET_MODE=docket`, `METADATA_BRANCH=docket`, `INTEGRATION_BRANCH=main`, `METADATA_WORKTREE=.docket`, `CHANGES_DIR=docs/changes`, `FINALIZE_GATE=local`, `BOARD_SURFACES=inline`, `BOOTSTRAP=PROCEED`. Record the literal output in the results file.
- [ ] Confirm `scripts/docket-config.sh` is executable (`test -x scripts/docket-config.sh`).

## Self-review notes (plan vs. spec)

- **Spec §3 output contract** → Task 1 emits all 13 keys; §3 read-only default → Tasks 1–3 never write under `--export`. ✓
- **Spec §4 three stages** → Task 1 (stages 1–2), Task 2 (stage 3 evaluate), Task 3 (stage 3 `--bootstrap` write). ✓
- **Spec §4 `.docket.yml` reconcile note (nested `finalize:`, no `---`)** → `yaml_get` reads leaf keys `gate`/`test_command`; self-contained, no frontmatter-lib reuse. ✓
- **Spec §5 skills keep owning STOP_MIGRATE / opt-in write** → Task 5 prose keeps those as skill actions; script only reports/optionally writes. ✓
- **Spec §7 testing** (resolution permutations, 4 bootstrap cells, `--bootstrap` only in fresh cell, error paths incl. stale-ref) → Tasks 1–4. ✓
- **Spec §8 risks** (blast radius → heaviest fixture coverage; lone write guarded + per-cell test; verbatim semantics; one `KEY=value` contract) → covered by Tasks 2–4 + the real-data smoke test. ✓
