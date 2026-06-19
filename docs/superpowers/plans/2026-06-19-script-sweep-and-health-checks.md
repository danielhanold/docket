# Script the mechanical health checks (`board-checks.sh`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the five *mechanical* `docket-status` health checks into a deterministic, git-only `scripts/board-checks.sh` (sourcing the existing `lib/docket-frontmatter.sh`), and rewire `docket-status`'s Health-checks section to invoke it — leaving the two judgment-bearing checks model-driven.

**Architecture:** `board-checks.sh` sources `scripts/lib/docket-frontmatter.sh` (change 0022 — `field`/`list_field`/`has_section`/`resolve_deps`/`readiness`), calls `resolve_deps` once, then walks every change file (`active/` + `archive/`) emitting one finding per line on stdout as TAB-separated `<check-id>\t<change-id>\t<message>`, sorted by `(check-id, change-id)`. Five checks: `broken-spec`, `broken-plan-results`, `dep-cycle`, `stale-in-progress`, `merge-gate-stall`. Git is the only external dependency (mock seam `GIT="${GIT:-git}"`); a `NOW` seam pins the staleness clock for deterministic tests. Clean tree ⇒ no output, exit 0; `--strict` ⇒ exit 1 on any finding. The skill edit replaces the five mechanical bullets with one script invocation and keeps `blocked_by:` re-examination and the inline board/source-drift bullet model-driven.

**Tech Stack:** Bash 4+ (associative arrays + `mapfile`, already the repo norm), `git`, the project's `tests/test_*.sh` hermetic-fixture convention (no `gh`, no network). Run a test with `bash tests/test_board_checks.sh`.

## Global Constraints

Copied verbatim from the spec + the LEARNINGS ledger; every task's requirements implicitly include these.

- **The shared helper is consumed, never re-implemented.** Source `scripts/lib/docket-frontmatter.sh`; use `field`/`list_field`/`has_section`/`resolve_deps`/`readiness`. If a check needs a helper not present, add it to the lib — do not re-roll a local parser. (Spec §3.) `github-mirror.sh` already sources the helper — **do not** touch it; its migration was 0022's work.
- **Frontmatter stays hand-rolled** — no `yq` (spec §4).
- **Git-only, hermetic.** No `gh`, no network. The only external dependency is `git`, via the mock seam `GIT="${GIT:-git}"`. (Spec §5b.)
- **Warn-only — never auto-fix.** The script only prints findings; the caller surfaces them. (Spec §5a.)
- **The merge sweep is out of scope.** Change 0025 already scripted its close-out and rewired the sweep. Do not touch the sweep prose or the close-out scripts. (Reconcile log.)
- **SIGPIPE discipline (LEARNINGS #25/#22/#11/#16):** under `set -o pipefail`, never pipe a producer straight into an early-closing consumer (`grep -q`, `head`, `head -n1`). Capture into a variable first, then grep/sort the variable. (`sort` reads all input, so `printf "$var" | sort` is safe.)
- **`field` trailing-newline (LEARNINGS #22):** `$(field …)` strips the trailing newline (safe); a *direct pipe* of `field` would expose a dropped newline. This script consumes `field` only via `$(…)` — keep it that way. Tests capture the script's stdout into a `$(…)` var before asserting.
- **Mutation-test every assertion (LEARNINGS #25/#20/#15/#2):** green tests ≠ the hard branch ran. Each check needs a positive fixture (finding fires) **and** a clean fixture (silent), and **each carve-out** must be exercised by a fixture that would flip if the carve-out were removed.
- **Deterministic, pinned clock:** the staleness check reads `NOW="${NOW:-$(date +%s)}"`; tests pin `NOW` to a fixed epoch and age commits with `GIT_COMMITTER_DATE="@<epoch> +0000"` so the result never depends on wall-clock.

---

### Task 1: `board-checks.sh` scaffold + `broken-spec` check

Establishes the script: CLI parsing, helper sourcing, the single `resolve_deps` call, the change-file walk with labelled anchor comments for later tasks, the `FINDINGS`/`emit`/sorted-output/exit-code framework, and the first check (`broken-spec`). Also lays down the hermetic `new_repo` test harness later tasks reuse.

**Files:**
- Create: `scripts/board-checks.sh`
- Test: `tests/test_board_checks.sh`

**Interfaces:**
- Consumes: `scripts/lib/docket-frontmatter.sh` — `field FILE KEY`, `list_field FILE KEY`, `resolve_deps DIR` (populates globals `STATUS_OF`/`DEP_STATE`/`DEP_REASON`/`DEP_ON`).
- Produces (for later tasks):
  - `emit <check-id> <change-id> <message>` — appends one TAB-separated finding line to the global `FINDINGS` string.
  - `git_has <ref> <path>` — exit 0 iff `<ref>:<path>` resolves in the changes-dir's repo (via `$GIT -C "$CHANGES_DIR" cat-file -e`).
  - Globals in scope inside the walk loop: `$f` (file path), `$id`, `$status`, `$spec`, `$trivial`.
  - Anchor comments inside the loop (`# >>> broken-plan-results`, `# >>> stale-in-progress`, `# >>> merge-gate-stall`) and after it (`# >>> dep-cycle pass`) mark exactly where Tasks 2–5 insert code.
  - CLI: `board-checks.sh --changes-dir DIR --metadata-branch BR --integration-branch BR [--strict]`.
  - Test harness function `new_repo` (printed `"<work> <origin>"`) reused by every later task's fixtures.

- [ ] **Step 1: Write the failing test** — create `tests/test_board_checks.sh` with the harness, the `new_repo` builder, and the `broken-spec` asserts.

```bash
#!/usr/bin/env bash
# tests/test_board_checks.sh — verifies change 0023: scripts/board-checks.sh, the mechanical
# docket-status health checks (broken-spec, broken-plan-results, dep-cycle, stale-in-progress,
# merge-gate-stall). Hermetic: a temp repo with a local *bare* origin carrying docket + main and
# a few feature branches; no gh, no network. Run: bash tests/test_board_checks.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/board-checks.sh"
SKILL="$REPO/skills/docket-status/SKILL.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
git_quiet(){ git "$@" >/dev/null 2>&1; }

# has_finding OUTPUT CHECK-ID CHANGE-ID — exit 0 iff OUTPUT has a "<check>\t<id>\t…" line.
# Builds a literal-TAB ERE pattern via printf (portable: avoids grep -P, which BSD grep lacks).
has_finding(){ printf '%s' "$1" | grep -qE "$(printf '^%s\t%s\t' "$2" "$3")"; }

# A fixed reference "now"; tests age commits relative to it and pass NOW=$NOW_EPOCH to the script
# so staleness never depends on wall-clock. (2026-06-15T13:20:00Z-ish; the exact value is irrelevant.)
NOW_EPOCH=1750000000

# new_repo: prints "<work> <origin>" — a fresh clone with a bare origin holding docket + main.
#   docket: docs/changes/active|archive + docs/superpowers/specs (committed specs).
#   main:   docs/superpowers/plans + docs/results (committed build artifacts).
# Callers add change files under $work/docs/changes/{active,archive}/ on the docket checkout,
# create feature branches as needed, then invoke the script against $work/docs/changes.
new_repo(){
  local root work origin
  root="$(mktemp -d)"; origin="$root/origin.git"; work="$root/work"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  # --- main branch: build artifacts that 'done' changes link to ---
  git -C "$work" checkout -b main >/dev/null 2>&1
  mkdir -p "$work/docs/superpowers/plans" "$work/docs/results"
  echo "# plan"    > "$work/docs/superpowers/plans/2026-06-01-present.md"
  echo "# results" > "$work/docs/results/2026-06-01-present-results.md"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "main artifacts"
  git_quiet -C "$work" push -u origin main
  # --- docket branch: orphan metadata ---
  git -C "$work" checkout --orphan docket >/dev/null 2>&1
  git -C "$work" rm -rf . >/dev/null 2>&1 || true
  mkdir -p "$work/docs/changes/active" "$work/docs/changes/archive" "$work/docs/superpowers/specs"
  echo "# present spec" > "$work/docs/superpowers/specs/2026-06-01-present.md"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "docket metadata baseline"
  git_quiet -C "$work" push -u origin docket
  # leave the work clone parked on docket (the metadata working tree)
  printf '%s %s\n' "$work" "$origin"
}

# commit_present_spec_change: a helper used across tasks — writes a change file into active/.
# (Inline cat in each task is fine too; this keeps fixtures short.)

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

# ============================ broken-spec ============================
# A change citing a spec absent on the metadata branch ⇒ one broken-spec finding.
# A change citing a present spec ⇒ silent. A trivial change with no spec ⇒ silent (carve-out).
read -r W _ < <(new_repo)
cat > "$W/docs/changes/active/0001-good.md" <<'EOF'
---
id: 1
slug: good
title: Good
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-06-01-present.md
trivial: false
EOF
cat > "$W/docs/changes/active/0002-missing.md" <<'EOF'
---
id: 2
slug: missing
title: Missing spec
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-06-01-ABSENT.md
trivial: false
EOF
cat > "$W/docs/changes/active/0003-trivial.md" <<'EOF'
---
id: 3
slug: trivial
title: Trivial, no spec
status: proposed
priority: medium
depends_on: []
spec:
trivial: true
EOF
out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$W/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "broken-spec fires for a missing spec path (id 2)" 'has_finding "$out" broken-spec 2'
assert "broken-spec silent for a present spec (id 1)" '! has_finding "$out" broken-spec 1'
assert "broken-spec silent for a trivial change with no spec (id 3, carve-out)" '! has_finding "$out" broken-spec 3'

# ============================ clean tree + exit codes ============================
# A repo whose only change cites a present spec ⇒ no output, exit 0; --strict still exit 0.
read -r C _ < <(new_repo)
cat > "$C/docs/changes/active/0001-good.md" <<'EOF'
---
id: 1
slug: good
title: Good
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-06-01-present.md
trivial: false
EOF
clean="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$C/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "clean tree ⇒ empty stdout" '[ -z "$clean" ]'
NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$C/docs/changes" --metadata-branch docket --integration-branch main >/dev/null 2>&1; rc=$?
assert "clean tree ⇒ exit 0" '[ "$rc" = 0 ]'
NOW=$NOW_EPOCH bash "$SCRIPT" --strict --changes-dir "$C/docs/changes" --metadata-branch docket --integration-branch main >/dev/null 2>&1; rc=$?
assert "clean tree ⇒ --strict exit 0" '[ "$rc" = 0 ]'
# --strict on a finding ⇒ exit 1
NOW=$NOW_EPOCH bash "$SCRIPT" --strict --changes-dir "$W/docs/changes" --metadata-branch docket --integration-branch main >/dev/null 2>&1; rc=$?
assert "finding present ⇒ --strict exit 1" '[ "$rc" = 1 ]'
# without --strict, a finding still exits 0 (findings go to stdout; caller surfaces them)
NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$W/docs/changes" --metadata-branch docket --integration-branch main >/dev/null 2>&1; rc=$?
assert "finding present without --strict ⇒ exit 0" '[ "$rc" = 0 ]'

# ============================ usage errors ============================
bash "$SCRIPT" --metadata-branch docket --integration-branch main >/dev/null 2>&1; rc=$?
assert "missing --changes-dir ⇒ exit 2" '[ "$rc" = 2 ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_board_checks.sh`
Expected: FAIL — `NOT OK - script exists and is executable` (the script does not exist yet).

- [ ] **Step 3: Write the minimal `scripts/board-checks.sh`**

```bash
#!/usr/bin/env bash
# scripts/board-checks.sh — the mechanical docket-status health checks (change 0023). Sources the
# shared frontmatter/dependency-resolution helper (change 0022) and walks the change files, emitting
# one finding per line on stdout. Git-only (no gh, no network) and warn-only (never auto-fixes); the
# caller (docket-status) surfaces the lines. The one judgment-bearing check (blocked_by: re-examination)
# stays model-driven in the skill — it is NOT here.
#
# Usage: board-checks.sh --changes-dir DIR --metadata-branch BR --integration-branch BR [--strict]
#   Findings: TAB-separated  <check-id>\t<change-id>\t<message>  on stdout, sorted by (check-id, change-id).
#     check-id ∈ {broken-spec, broken-plan-results, dep-cycle, stale-in-progress, merge-gate-stall}
#   Clean tree ⇒ no output, exit 0. --strict ⇒ exit 1 if any finding (for a future CI gate).
#   Branch args are passed to `git cat-file -e <ref>:<path>` verbatim; in main-mode the two refs
#   coincide and both link checks resolve on the same branch with no special-casing.
#   Mock seams: GIT="${GIT:-git}"  (the only external dependency); NOW="${NOW:-$(date +%s)}" (staleness clock).
set -uo pipefail

GIT="${GIT:-git}"
NOW="${NOW:-$(date +%s)}"
CHANGES_DIR=""; METADATA_BRANCH=""; INTEGRATION_BRANCH=""; STRICT=0
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --metadata-branch) METADATA_BRANCH="$2"; shift ;;
    --integration-branch) INTEGRATION_BRANCH="$2"; shift ;;
    --strict) STRICT=1 ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'board-checks: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$CHANGES_DIR" ]        || { printf 'board-checks: missing --changes-dir\n' >&2; exit 2; }
[ -d "$CHANGES_DIR" ]        || { printf 'board-checks: changes dir not found: %s\n' "$CHANGES_DIR" >&2; exit 2; }
[ -n "$METADATA_BRANCH" ]    || { printf 'board-checks: missing --metadata-branch\n' >&2; exit 2; }
[ -n "$INTEGRATION_BRANCH" ] || { printf 'board-checks: missing --integration-branch\n' >&2; exit 2; }

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"

resolve_deps "$CHANGES_DIR"            # populates STATUS_OF / DEP_STATE / DEP_REASON / DEP_ON

# git_has REF PATH — exit 0 iff REF:PATH resolves in the changes-dir's repo (no network).
git_has(){ "$GIT" -C "$CHANGES_DIR" cat-file -e "$1:$2" 2>/dev/null; }

FINDINGS=""                            # accumulate "<check>\t<id>\t<msg>\n"; sorted + printed at the end
emit(){ FINDINGS+="$1"$'\t'"$2"$'\t'"$3"$'\n'; }

# Walk every change file (active + archive); per-check filters apply inside.
mapfile -t FILES < <(find "$CHANGES_DIR/active" "$CHANGES_DIR/archive" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
for f in "${FILES[@]}"; do
  id="$(field "$f" id)"; [ -n "$id" ] || continue
  status="$(field "$f" status)"
  spec="$(field "$f" spec)"; trivial="$(field "$f" trivial)"

  # --- broken-spec: spec set, not trivial, path absent on the metadata branch ---
  if [ -n "$spec" ] && [ "$trivial" != "true" ]; then
    git_has "$METADATA_BRANCH" "$spec" || emit broken-spec "$id" "spec not found on $METADATA_BRANCH: $spec"
  fi

  # >>> broken-plan-results  (Task 2 inserts here)

  # >>> stale-in-progress    (Task 4 inserts here)

  # >>> merge-gate-stall     (Task 5 inserts here)
done

# >>> dep-cycle pass         (Task 3 inserts here)

# Emit findings sorted by (check-id asc, change-id numeric asc) for determinism.
if [ -n "$FINDINGS" ]; then
  printf '%s' "$FINDINGS" | sort -t"$(printf '\t')" -k1,1 -k2,2n
fi

if [ "$STRICT" = 1 ] && [ -n "$FINDINGS" ]; then exit 1; fi
exit 0
```

- [ ] **Step 4: Make the script executable**

Run: `chmod +x scripts/board-checks.sh`

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_board_checks.sh`
Expected: PASS (every `ok - …`, final `PASS`).

- [ ] **Step 6: Commit**

```bash
git add scripts/board-checks.sh tests/test_board_checks.sh
git commit -m "feat(0023): board-checks.sh scaffold + broken-spec check"
```

---

### Task 2: `broken-plan-results` check

Flags a `done` change whose set `plan:`/`results:` does not resolve on the integration branch (link rot). Carve-out: an `implemented` change is never flagged — those files legitimately still live on the unmerged feature branch (the loop only inspects `status: done`, so this falls out naturally; the test proves it).

**Files:**
- Modify: `scripts/board-checks.sh` (insert at the `# >>> broken-plan-results` anchor)
- Test: `tests/test_board_checks.sh` (append a fixture + asserts)

**Interfaces:**
- Consumes: `git_has`, `emit`, `$f`, `$id`, `$status` from Task 1; `INTEGRATION_BRANCH`.
- Produces: `broken-plan-results` findings.

- [ ] **Step 1: Write the failing test** — append before the final `if [ "$fail" = 0 ]` line of `tests/test_board_checks.sh`:

```bash
# ============================ broken-plan-results ============================
# A 'done' change whose results: path is absent on the integration branch ⇒ one finding.
# The SAME missing field on an 'implemented' change ⇒ silent (carve-out). Present links ⇒ silent.
read -r P _ < <(new_repo)
cat > "$P/docs/changes/archive/2026-06-02-0010-donegood.md" <<'EOF'
---
id: 10
slug: donegood
title: Done, links present
status: done
priority: medium
depends_on: []
plan: docs/superpowers/plans/2026-06-01-present.md
results: docs/results/2026-06-01-present-results.md
EOF
cat > "$P/docs/changes/archive/2026-06-02-0011-donerot.md" <<'EOF'
---
id: 11
slug: donerot
title: Done, results link rotted
status: done
priority: medium
depends_on: []
plan: docs/superpowers/plans/2026-06-01-present.md
results: docs/results/2026-06-01-ABSENT-results.md
EOF
cat > "$P/docs/changes/active/0012-implmissing.md" <<'EOF'
---
id: 12
slug: implmissing
title: Implemented, plan not on integration yet
status: implemented
priority: medium
depends_on: []
plan: docs/superpowers/plans/2026-06-01-ABSENT.md
results:
EOF
pout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$P/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "broken-plan-results fires for a done change with a rotted results link (id 11)" \
  'has_finding "$pout" broken-plan-results 11'
assert "broken-plan-results silent for a done change with present links (id 10)" \
  '! has_finding "$pout" broken-plan-results 10'
assert "broken-plan-results silent for an implemented change with an absent plan (id 12, carve-out)" \
  '! has_finding "$pout" broken-plan-results 12'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_board_checks.sh`
Expected: FAIL — `NOT OK - broken-plan-results fires …` (the check is not implemented).

- [ ] **Step 3: Implement the check** — replace the `# >>> broken-plan-results  (Task 2 inserts here)` line in `scripts/board-checks.sh` with:

```bash
  # --- broken-plan-results: a done change's set plan:/results: must resolve on the integration branch ---
  # Carve-out: never flag an 'implemented' change — those files still live on the unmerged feature branch.
  if [ "$status" = "done" ]; then
    for key in plan results; do
      val="$(field "$f" "$key")"
      [ -n "$val" ] || continue
      git_has "$INTEGRATION_BRANCH" "$val" || emit broken-plan-results "$id" "$key not found on $INTEGRATION_BRANCH: $val"
    done
  fi
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_board_checks.sh`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/board-checks.sh tests/test_board_checks.sh
git commit -m "feat(0023): broken-plan-results check (done-only, implemented carve-out)"
```

---

### Task 3: `dep-cycle` check

DFS over the `depends_on` graph; on a cycle, emit one finding **per node in the cycle** so the human sees the whole loop. Edges to ids that are not themselves changes are skipped (a dangling `depends_on` is not a cycle). Bash-4.0-safe (slice-based stack pop, no `unset 'arr[-1]'`).

**Files:**
- Modify: `scripts/board-checks.sh` (insert at the `# >>> dep-cycle pass` anchor, after the per-file loop)
- Test: `tests/test_board_checks.sh` (append fixtures + asserts)

**Interfaces:**
- Consumes: `list_field`, `emit`, `FILES` from Task 1.
- Produces: `dep-cycle` findings (one per node on a cycle).

- [ ] **Step 1: Write the failing test** — append before the final `if [ "$fail" = 0 ]` line:

```bash
# ============================ dep-cycle ============================
# A→B→A ⇒ a finding for EACH node (1 and 2). A self-loop C→C ⇒ a finding for C.
# A clean DAG (D→E, no back edge) ⇒ silent. A dangling depends_on (F→99 missing) ⇒ silent.
read -r G _ < <(new_repo)
mk(){ # mk ID SLUG DEPS  — minimal proposed change with a present spec (so broken-spec stays quiet)
  cat > "$G/docs/changes/active/$(printf '%04d' "$1")-$2.md" <<EOF
---
id: $1
slug: $2
title: $2
status: proposed
priority: medium
depends_on: [$3]
spec: docs/superpowers/specs/2026-06-01-present.md
trivial: false
EOF
}
mk 1 a 2
mk 2 b 1
mk 3 c 3
mk 4 d 5
mk 5 e ""
mk 6 f 99
gout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$G/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "dep-cycle fires for both nodes of A→B→A (id 1)" 'has_finding "$gout" dep-cycle 1'
assert "dep-cycle fires for both nodes of A→B→A (id 2)" 'has_finding "$gout" dep-cycle 2'
assert "dep-cycle fires for a self-loop (id 3)" 'has_finding "$gout" dep-cycle 3'
assert "dep-cycle silent for a DAG node (id 4)" '! has_finding "$gout" dep-cycle 4'
assert "dep-cycle silent for a DAG leaf (id 5)" '! has_finding "$gout" dep-cycle 5'
assert "dep-cycle silent for a dangling depends_on (id 6 → missing 99)" '! has_finding "$gout" dep-cycle 6'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_board_checks.sh`
Expected: FAIL — `NOT OK - dep-cycle fires …`.

- [ ] **Step 3: Implement the check** — replace the `# >>> dep-cycle pass  (Task 3 inserts here)` line with:

```bash
# --- dep-cycle: DFS over depends_on; mark every node that lies on a cycle ---
declare -A ADJ COLOR INSTACK ONCYCLE
for f in "${FILES[@]}"; do
  cid="$(field "$f" id)"; [ -n "$cid" ] || continue
  ADJ["$cid"]="$(list_field "$f" depends_on)"
done
PATH_STACK=()
dfs(){ # dfs NODE — colors: white(unset) / gray(on stack) / black(done)
  local node="$1" dep i seen
  COLOR["$node"]=gray; INSTACK["$node"]=1; PATH_STACK+=("$node")
  for dep in ${ADJ["$node"]:-}; do
    [ -n "${ADJ[$dep]+x}" ] || continue            # dep is not a known change ⇒ not a graph edge
    if [ "${INSTACK[$dep]:-0}" = 1 ]; then
      seen=0                                        # back edge: mark dep..top-of-stack
      for i in "${PATH_STACK[@]}"; do
        [ "$i" = "$dep" ] && seen=1
        [ "$seen" = 1 ] && ONCYCLE["$i"]=1
      done
    elif [ "${COLOR[$dep]:-white}" = white ]; then
      dfs "$dep"
    fi
  done
  COLOR["$node"]=black; INSTACK["$node"]=0
  PATH_STACK=("${PATH_STACK[@]:0:${#PATH_STACK[@]}-1}")   # pop (bash-4.0-safe; no unset arr[-1])
}
for node in "${!ADJ[@]}"; do
  [ "${COLOR[$node]:-white}" = white ] && dfs "$node"
done
for node in "${!ONCYCLE[@]}"; do
  emit dep-cycle "$node" "participates in a depends_on cycle"
done
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_board_checks.sh`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/board-checks.sh tests/test_board_checks.sh
git commit -m "feat(0023): dep-cycle check (per-node cycle marking, dangling-edge safe)"
```

---

### Task 4: `stale-in-progress` check

Flags an `in-progress` change whose feature branch exists but has had no commit in over 3 days. Two carve-outs: (a) `branch:` set but the branch does not exist ⇒ silent (a just-claimed change, or "gone after creation" — indistinguishable via git alone, so conservatively not flagged); (b) a recent commit ⇒ silent. The clock is the `NOW` seam (default `date +%s`).

**Files:**
- Modify: `scripts/board-checks.sh` (insert at the `# >>> stale-in-progress` anchor)
- Test: `tests/test_board_checks.sh` (append fixtures + asserts)

**Interfaces:**
- Consumes: `field`, `emit`, `$f`, `$id`, `$status` from Task 1; `GIT`, `CHANGES_DIR`, `NOW`.
- Produces: `stale-in-progress` findings.

- [ ] **Step 1: Write the failing test** — append before the final `if [ "$fail" = 0 ]` line:

```bash
# ============================ stale-in-progress ============================
# in-progress + branch with last commit 4 days old ⇒ finding. branch with a commit today ⇒ silent.
# in-progress + branch: set but branch absent ⇒ silent (carve-out).
read -r S _ < <(new_repo)
STALE_EPOCH=$(( NOW_EPOCH - 4*86400 ))
FRESH_EPOCH=$(( NOW_EPOCH - 3600 ))
# feat/stale — aged commit
git -C "$S" checkout -b feat/stale >/dev/null 2>&1
echo x > "$S/x"; git -C "$S" add x
GIT_AUTHOR_DATE="@$STALE_EPOCH +0000" GIT_COMMITTER_DATE="@$STALE_EPOCH +0000" git_quiet -C "$S" commit -m "aged"
# feat/fresh — commit "now"
git -C "$S" checkout -b feat/fresh docket >/dev/null 2>&1
echo y > "$S/y"; git -C "$S" add y
GIT_AUTHOR_DATE="@$FRESH_EPOCH +0000" GIT_COMMITTER_DATE="@$FRESH_EPOCH +0000" git_quiet -C "$S" commit -m "fresh"
git -C "$S" checkout docket >/dev/null 2>&1
cat > "$S/docs/changes/active/0020-stale.md" <<'EOF'
---
id: 20
slug: stale
title: Stale claim
status: in-progress
priority: medium
depends_on: []
branch: feat/stale
EOF
cat > "$S/docs/changes/active/0021-fresh.md" <<'EOF'
---
id: 21
slug: fresh
title: Fresh claim
status: in-progress
priority: medium
depends_on: []
branch: feat/fresh
EOF
cat > "$S/docs/changes/active/0022-justclaimed.md" <<'EOF'
---
id: 22
slug: justclaimed
title: Just claimed, no branch yet
status: in-progress
priority: medium
depends_on: []
branch: feat/justclaimed
EOF
sout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "stale-in-progress fires for a branch idle >3 days (id 20)" \
  'has_finding "$sout" stale-in-progress 20'
assert "stale-in-progress silent for a branch with a recent commit (id 21)" \
  '! has_finding "$sout" stale-in-progress 21'
assert "stale-in-progress silent when branch: set but branch absent (id 22, carve-out)" \
  '! has_finding "$sout" stale-in-progress 22'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_board_checks.sh`
Expected: FAIL — `NOT OK - stale-in-progress fires …`.

- [ ] **Step 3: Implement the check** — replace the `# >>> stale-in-progress  (Task 4 inserts here)` line with:

```bash
  # --- stale-in-progress: in-progress, branch exists, newest commit older than 3 days ---
  # Carve-out: branch: set but the branch does not exist ⇒ not stale (just-claimed / indistinguishable).
  if [ "$status" = "in-progress" ]; then
    branch="$(field "$f" branch)"
    if [ -n "$branch" ] && "$GIT" -C "$CHANGES_DIR" rev-parse --verify --quiet "$branch" >/dev/null 2>&1; then
      ts="$("$GIT" -C "$CHANGES_DIR" log -1 --format=%ct "$branch" 2>/dev/null)"
      if [ -n "$ts" ] && [ "$(( NOW - ts ))" -gt "$(( 3*86400 ))" ]; then
        emit stale-in-progress "$id" "branch $branch idle >3 days (last commit $(( (NOW - ts) / 86400 ))d ago)"
      fi
    fi
  fi
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_board_checks.sh`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/board-checks.sh tests/test_board_checks.sh
git commit -m "feat(0023): stale-in-progress check (3d threshold, NOW seam, branch-absent carve-out)"
```

---

### Task 5: `merge-gate-stall` check

A build-ready change (`proposed`, has `spec:` or `trivial: true`) whose worst-unmet dependency is stuck at `implemented` — surfaced from `resolve_deps` (`DEP_REASON[id] == "needs your merge"`), naming the blocking dep straight from `DEP_ON[id]` (the reconcile-confirmed bonus the helper provides — no re-walk).

**Files:**
- Modify: `scripts/board-checks.sh` (insert at the `# >>> merge-gate-stall` anchor)
- Test: `tests/test_board_checks.sh` (append fixtures + asserts)

**Interfaces:**
- Consumes: `emit`, `$id`, `$status`, `$spec`, `$trivial` from Task 1; globals `DEP_REASON`, `DEP_ON` from `resolve_deps`.
- Produces: `merge-gate-stall` findings.

- [ ] **Step 1: Write the failing test** — append before the final `if [ "$fail" = 0 ]` line:

```bash
# ============================ merge-gate-stall ============================
# A build-ready change depends_on a change at 'implemented' ⇒ finding naming that dep.
# A build-ready change depends_on a change still 'proposed' (not yet built) ⇒ NOT a merge-gate-stall.
read -r M _ < <(new_repo)
cat > "$M/docs/changes/active/0030-impl.md" <<'EOF'
---
id: 30
slug: impl
title: Implemented dep
status: implemented
priority: medium
depends_on: []
pr: https://github.com/o/r/pull/9
EOF
cat > "$M/docs/changes/active/0031-waiter.md" <<'EOF'
---
id: 31
slug: waiter
title: Build-ready, waiting on a merge
status: proposed
priority: medium
depends_on: [30]
spec: docs/superpowers/specs/2026-06-01-present.md
trivial: false
EOF
cat > "$M/docs/changes/active/0032-unbuilt.md" <<'EOF'
---
id: 32
slug: unbuilt
title: Unbuilt dep
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-06-01-present.md
trivial: false
EOF
cat > "$M/docs/changes/active/0033-waiter2.md" <<'EOF'
---
id: 33
slug: waiter2
title: Waiting on a not-yet-built dep
status: proposed
priority: medium
depends_on: [32]
spec: docs/superpowers/specs/2026-06-01-present.md
trivial: false
EOF
mout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$M/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "merge-gate-stall fires for a build-ready change waiting on an implemented dep (id 31)" \
  'has_finding "$mout" merge-gate-stall 31'
assert "merge-gate-stall names the blocking dep #30" \
  'printf "%s" "$mout" | grep -E "$(printf "^merge-gate-stall\t31\t")" | grep -qF "#30"'
assert "merge-gate-stall silent for a change waiting on a not-yet-built dep (id 33)" \
  '! has_finding "$mout" merge-gate-stall 33'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_board_checks.sh`
Expected: FAIL — `NOT OK - merge-gate-stall fires …`.

- [ ] **Step 3: Implement the check** — replace the `# >>> merge-gate-stall  (Task 5 inserts here)` line with:

```bash
  # --- merge-gate-stall: build-ready, but its worst-unmet dep is stuck at 'implemented' ---
  if [ "$status" = "proposed" ] && { [ -n "$spec" ] || [ "$trivial" = "true" ]; }; then
    if [ "${DEP_REASON[$id]:-}" = "needs your merge" ]; then
      emit merge-gate-stall "$id" "build-ready but waiting on #${DEP_ON[$id]} — needs your merge"
    fi
  fi
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_board_checks.sh`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/board-checks.sh tests/test_board_checks.sh
git commit -m "feat(0023): merge-gate-stall check (names blocking dep via DEP_ON)"
```

---

### Task 6: Wire `docket-status`'s Health-checks section to invoke the script

Replace the five mechanical bullets in `skills/docket-status/SKILL.md` with one instruction to invoke `scripts/board-checks.sh` and surface each finding line as a warning. **Keep model-driven, unchanged:** the `blocked_by:` re-examination bullet and the inline board/source-drift bullet (owned by the still-open change 0024). **Keep:** the "do not auto-fix unless asked" stance and the "share the one dependency-resolution pass" note (now literally `resolve_deps`, run by the script). The `github`-surface mirror-reachability flag is unaffected (it is part of the board/source-drift bullet, which stays).

**Files:**
- Modify: `skills/docket-status/SKILL.md` (the `## Health checks` section)
- Test: `tests/test_board_checks.sh` (append skill-wiring sentinels)

**Interfaces:**
- Consumes: nothing new.
- Produces: the documented call-site for `board-checks.sh`.

- [ ] **Step 1: Write the failing test** — append before the final `if [ "$fail" = 0 ]` line:

```bash
# ============================ docket-status wiring sentinels (SKILL is code on main) ============================
assert "docket-status Health checks invoke board-checks.sh" \
  'grep -qF "scripts/board-checks.sh" "$SKILL"'
# The five mechanical checks are now delegated — their old standalone bullets are gone as bullets,
# but the SKILL still names them so a reader knows what the script covers. Assert the two
# judgment/0024 checks remain explicitly model-driven, each anchored to a phrase it owns.
assert "docket-status keeps blocked_by re-examination model-driven" \
  'grep -qiF "blocked_by:" "$SKILL"'
assert "docket-status keeps the inline board/source drift check (owned by change 0024)" \
  'grep -qiF "board/source drift" "$SKILL" || grep -qiF "board/source-drift" "$SKILL"'
assert "docket-status keeps the do-not-auto-fix stance" \
  'grep -qiF "do not auto-fix" "$SKILL"'
# Mutation guard: the board-checks invocation passes the changes-dir + both branch refs.
assert "the board-checks invocation passes --changes-dir" 'grep -qF -- "--changes-dir" "$SKILL"'
assert "the board-checks invocation passes --metadata-branch and --integration-branch" \
  'grep -qF -- "--metadata-branch" "$SKILL" && grep -qF -- "--integration-branch" "$SKILL"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_board_checks.sh`
Expected: FAIL — `NOT OK - docket-status Health checks invoke board-checks.sh` (the SKILL still lists the five bullets).

- [ ] **Step 3: Make the edit** — in `skills/docket-status/SKILL.md`, replace the entire `## Health checks` section body. Replace this exact current block:

```markdown
Flag the following (do not auto-fix unless asked). Board and health checks share the one dependency-resolution pass computed above — it is not re-run.

- **Stale `in-progress` past the build step** — the planned branch is gone, or exists but has had no commits in **3 days** (3 is the current fixed default; promoting it to a `.docket.yml` knob is a future enhancement). A just-claimed change with a `branch:` value but no branch yet created is **not** stale.
- **Broken `spec:` link** — `spec:` is set but the path does not resolve against `metadata_branch` (in `docket`-mode, against `docket` — where the spec lives). Skip `trivial: true` changes; they have no spec.
- **Broken `plan:`/`results:` link on `done` changes** — resolve `plan:` and `results:` against **`origin/<integration_branch>`, NOT `docket`** (those files never live on `docket` — they are feature-branch build artifacts that reach the integration branch via the PR merge; resolving them against `docket` would flag every `done` change as a permanent broken link). A `done` change's `plan:` and `results:` paths must resolve there (link rot check). Ignore a missing `plan:` or `results:` on an `implemented` change — those files legitimately still live on the unmerged feature branch (pre-merge they don't resolve on the integration branch yet; that is tolerated until merge). In `main`-mode `metadata_branch == integration_branch`, so both resolve on the same branch.
- **Human-merge gate stall** — a build-ready change whose only unsatisfied dependency is stuck at `implemented` (from the shared pass, reason = `"needs your merge"`). Surfaces the dependency so the human knows a single merge unblocks downstream work.
- **`blocked` changes whose blocker may have cleared** — re-examine `blocked_by:` text; flag if the referenced issue/PR/event appears resolved.
- **`depends_on` cycles** — detect circular dependency chains; flag every change in the cycle.
- **Board/source drift** — runs **per enabled surface** (skipped entirely when `board_surfaces: []`). For `inline`: render the board in-memory from the change files (reusing the shared dependency-resolution pass) and compare it to the committed `BOARD.md`; if any change's rendered status or placement disagrees, **warn** naming the change(s) (a writer skipped the board-refresh invariant). For `github`: warn on a change carrying an `issue:` whose mirror is unreachable (the upsert is best-effort and self-heals; this is only a visibility flag). Like the other checks it only warns — it never auto-fixes; the Board pass in this same `docket-status` run re-renders the enabled surfaces and heals the drift regardless. A best-effort refresh is allowed to lose a race.
```

with this new block:

```markdown
Flag the following (do not auto-fix unless asked). Board and health checks share the one dependency-resolution pass computed above — it is not re-run (it is now literally `resolve_deps`, run inside the script below).

**Mechanical checks → `scripts/board-checks.sh` (change 0023).** The five mechanical checks are deterministic git probes, so they live in a script, not in prose. Invoke:

```
scripts/board-checks.sh --changes-dir <metadata working tree>/<changes_dir> \
  --metadata-branch <metadata_branch> --integration-branch origin/<integration_branch>
```

(in `docket`-mode the metadata working tree is `.docket/`, so `--changes-dir .docket/<changes_dir> --metadata-branch docket`; resolve `plan:`/`results:` against `origin/<integration_branch>` — those files never live on `docket`. In `main`-mode pass `--metadata-branch <integration_branch> --integration-branch origin/<integration_branch>`; both link checks then resolve on the same content). The script sources the shared helper, calls `resolve_deps` once, and prints one finding per line on stdout — TAB-separated `<check-id>\t<change-id>\t<message>`, `check-id` ∈ `{broken-spec, broken-plan-results, dep-cycle, stale-in-progress, merge-gate-stall}`. **Surface each finding line as a warning.** A clean tree prints nothing. The script is **git-only** (no `gh`, no network) and **warn-only** (it never auto-fixes); `--strict` makes it exit non-zero on any finding, for a future CI gate. What the five cover:

- **`broken-spec`** — `spec:` set (and not `trivial: true`) but the path does not resolve on the metadata branch.
- **`broken-plan-results`** — a `done` change's set `plan:`/`results:` does not resolve on the integration branch (link rot). An `implemented` change is never flagged — those files legitimately still live on the unmerged feature branch.
- **`dep-cycle`** — a `depends_on` cycle; one finding per change in the loop.
- **`stale-in-progress`** — an `in-progress` change whose feature branch exists but has had no commit in **3 days** (the current fixed default). A just-claimed change with a `branch:` value but no branch yet created is **not** stale.
- **`merge-gate-stall`** — a build-ready change whose worst-unmet dependency is stuck at `implemented` (reason `"needs your merge"`), naming the blocking `#N`. Surfaces that a single merge unblocks downstream work.

**Model-driven checks (judgment — stay in-model, on top of the script):**

- **`blocked` changes whose blocker may have cleared** — re-examine `blocked_by:` free text; flag if the referenced issue/PR/event appears resolved. Judgment, not a git probe — never scripted.
- **Board/source drift** — runs **per enabled surface** (skipped entirely when `board_surfaces: []`). For `inline`: render the board in-memory from the change files (reusing the shared dependency-resolution pass) and compare it to the committed `BOARD.md`; if any change's rendered status or placement disagrees, **warn** naming the change(s) (a writer skipped the board-refresh invariant). For `github`: warn on a change carrying an `issue:` whose mirror is unreachable (the upsert is best-effort and self-heals; this is only a visibility flag). Like the other checks it only warns — it never auto-fixes; the Board pass in this same `docket-status` run re-renders the enabled surfaces and heals the drift regardless. A best-effort refresh is allowed to lose a race. (Retiring/downgrading this `inline` drift check once rendering is deterministic is change **0024**.)
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_board_checks.sh`
Expected: PASS.

- [ ] **Step 5: Run the full test suite to confirm no regressions**

Run: `for t in tests/test_*.sh; do echo "== $t =="; bash "$t" | tail -1; done`
Expected: every test ends `PASS` (in particular `tests/test_render_board.sh` and `tests/test_github_mirror.sh`, which share the helper, stay green).

- [ ] **Step 6: Commit**

```bash
git add skills/docket-status/SKILL.md tests/test_board_checks.sh
git commit -m "docs(0023): rewire docket-status health checks to invoke board-checks.sh"
```

---

## Notes for the implementer & reviewer

- **The boundary ADR is NOT a build task.** The §2 guiding principle ("mechanical & side-effect-free ⇒ script; judgment or shared terminal-transition ⇒ agent-prose", generalizing ADR-0007) is recorded via the `docket-adr` subagent during the implementer's review step (docket Step 6), committed on the `docket` branch — it is metadata, never a feature-branch file. Do not create an ADR file here.
- **Do not touch the merge sweep or the close-out scripts.** Change 0025 already scripted the sweep's close-out and rewired the call-site; this change is health-checks-only (see the change's reconcile log).
- **Do not touch `github-mirror.sh`.** It already sources the shared helper (0022's migration); 0023 only consumes the helper.
- **Task 6's wiring sentinels** assert only that the SKILL names `scripts/board-checks.sh` and passes `--changes-dir`/`--metadata-branch`/`--integration-branch`; the exact invocation wording is allowed to evolve as long as the flags are passed.
