<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0113 — A suppressed hand-off can silently end an autonomous run — make step completion verifiable, not narrated](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0113-suppressed-handoff-silently-ends-autonomous-run.md)**
<!-- docket:backlink:end -->

# Verifiable step completion — the `aborted-run` check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an external, deterministic, git-only `aborted-run` health check that detects an autonomous run that narrated success but dropped its bookkeeping write, plus the two prose riders that tighten the same failure mode.

**Architecture:** `aborted-run` becomes the fifteenth check-id in `scripts/board-checks.sh`, riding the existing health-check family (registry array, report rendering, fixture + mutation-test conventions — the exact path change 0191 used for `scalar-form`). Two independent legs, both gated on `status: in-progress`: **leg A** is manifest/git incoherence (the feature branch carries a plan or results file that the integration branch does not have, while the matching manifest field is empty) — the exact inverse of the existing `broken-plan-results` check; **leg B** is a run-scale stale claim (`claimed_at` older than 12h). Advisory only: it emits findings on stdout, flips no status, releases no claim, writes no file. Two riders adjust `skills/docket-implement-next/SKILL.md` prose.

**Tech Stack:** Bash 4.0-floor shell (`scripts/board-checks.sh`, `scripts/lib/docket-frontmatter.sh`), the hermetic temp-repo test harness in `tests/test_board_checks.sh`, markdown contracts in `scripts/*.md`.

## Global Constraints

- **Bash 4.0 floor.** No `${arr[-1]}`, no `declare -n`. Guard every possibly-empty array expansion as `${arr[@]+"${arr[@]}"}`. The file's prologue is `set -uo pipefail` (no `-e`).
- **Git-only, offline.** No `gh`, no network, no `curl`. Every git call goes through the `GIT="${GIT:-git}"` mock seam and is `git -C "$CHANGES_DIR" …`. Detecting an open PR is explicitly out of bounds.
- **Warn-only, pure reader.** `board-checks.sh` never mutates a change file, never auto-fixes, never marks `EXPLAINED`, and never feeds `board-row-dropped`.
- **Clock seam.** All time comparisons use `NOW="${NOW:-$(date +%s)}"`, never a bare `date`.
- **Anchored reads for optional keys.** `plan:`, `results:`, `branch:`, `claimed_at:` are all optional manifest fields. Read every one of them with `fm_field` (first `---…---` block only), never `field` — an unanchored read falls through into body prose. This is ADR-0057 / the `frontmatter-anchored-read` learning, and in this repo body prose about `plan:` is normal content, not a contrived fixture.
- **Literal check-ids at every emit site.** Write `emit aborted-run "$id" "…"`. An `emit "$var"` site is invisible to the whole correspondence-guard section of the test file and reddens the `emit_sites == emit_literal_sites` assert.
- **Every predicate gets a mutation test, and every mutation must be proven to have landed** (`grep -c` before and after) before its red/green result is believed. A completion check that cannot fail is the defect this change exists to fix, wearing a badge.
- **Hardcoded 12h window.** No config knob. House precedent: `FINALIZE_BLOCKED_STALE_SECS` and `stale-in-progress`'s 3-day branch-idle horizon are both hardcoded.
- Commit subjects use the docket scope form: `feat(0113): …`, `test(0113): …`, `docs(0113): …`.

---

## File Structure

| File | Responsibility in this change |
|---|---|
| `scripts/lib/docket-frontmatter.sh` | Declare `aborted-run` in `BOARD_CHECK_IDS` (alphabetically first). |
| `scripts/board-checks.sh` | New `--results-dir` flag, the `PLANS_DIR_REL` / `ABORTED_RUN_STALE_SECS` constants, the `branch_ref` + `branch_only_artifact` helpers, and the two-leg `aborted-run` block. Header comment's check-id list. |
| `scripts/docket-status.sh` | Pass `--results-dir` through from the resolved config. |
| `scripts/board-checks.md` | Contract entry for `aborted-run`; `--results-dir` in Usage. |
| `scripts/docket-status.md` | Report vocabulary — add `aborted-run` to the `check <check-id>` set. |
| `tests/test_board_checks.sh` | Fixtures (red + green), the registry count bump, mutation tests. |
| `skills/docket-implement-next/SKILL.md` | Rider 1: split the §5 fused sentence. Rider 2: densify the `claimed_at` heartbeat. |
| `tests/test_skill_size_budgets.sh` | Word-budget headroom for the two riders (currently 3939/3950 — 11 words). |

ADR-0044's dated `## Update` note is **deliberately not a task here**. ADRs live on the metadata branch and are written by the `docket-adr` skill, never by a feature-branch build worker; it ships atomically to `main` via this change's `adrs: [24, 44]` at terminal publish (the `adr-update-delivery` learning). Do not create or edit any file under `docs/adrs/` on this branch.

---

### Task 1: Leg A — manifest/git incoherence, plus the full check-id registration

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh:306-308` (the `BOARD_CHECK_IDS` array)
- Modify: `scripts/board-checks.sh` (argument parsing ~line 30-50; constants near `FINALIZE_BLOCKED_STALE_SECS` ~line 103; helpers near `git_has` ~line 67; the new check block after the `stale-in-progress` block ~line 315; header comment lines 8-15)
- Modify: `scripts/docket-status.sh:731-734` (the `board-checks.sh` invocation)
- Modify: `scripts/board-checks.md` (Usage section; a new check entry)
- Modify: `scripts/docket-status.md:364` (report vocabulary)
- Test: `tests/test_board_checks.sh` (new section after the `scalar-form` section, which ends at the `rm -rf "$mcopy"` line ~876; and the count literal at ~line 1808)

**Interfaces:**
- Consumes: `field`, `fm_field`, `int_field`, `iso_to_epoch`, `emit`, `git_has`, `GIT`, `NOW`, `CHANGES_DIR`, `INTEGRATION_BRANCH` — all already present in `board-checks.sh` / `lib/docket-frontmatter.sh`.
- Produces: `branch_ref BRANCH` → prints a resolvable ref name (`refs/heads/<b>` or `refs/remotes/origin/<b>`) and exits 0, else exits 1 with empty stdout. `branch_only_artifact REF DIR` → prints the first path under `DIR` present on `REF` but absent on `INTEGRATION_BRANCH` and exits 0, else exits 1 with empty stdout. `RESULTS_DIR_REL` (repo-relative results dir, default `docs/results`), `PLANS_DIR_REL` (constant `docs/superpowers/plans`), `ABORTED_RUN_STALE_SECS` (`12 * 3600`, consumed by Task 2). Task 2 adds leg B inside the same `# --- aborted-run:` block.

**Registration must land in this task, not before it.** `tests/test_board_checks.sh` set-compares the *declared* `BOARD_CHECK_IDS`, the ids `board-checks.sh` *actually emits*, `board-checks.md`, and `docket-status.md`. Adding the id to any one of those surfaces without the emitting code reddens the suite. All four move together, here.

- [ ] **Step 1: Write the failing fixtures**

Append this to `tests/test_board_checks.sh`, immediately after the `scalar-form` section's closing `rm -rf "$mcopy"` line and before the `# ======================= board-row-dropped` banner:

```bash
# ============================ aborted-run, leg A (change 0113) ============================
# An autonomous run that narrated success but dropped its bookkeeping write. Leg A is the
# TIME-FREE half: the feature branch carries an artifact file that is absent from the integration
# branch while the matching manifest field is EMPTY. This is the exact INVERSE of
# broken-plan-results (field set, file missing on the integration branch) — same two fields, same
# two trees, opposite direction; together they close a square that was half-open.
#
# Every optional field this check reads (plan, results, branch, claimed_at) goes through the
# ANCHORED fm_field: an unanchored read falls through the closing --- into body prose, and in this
# repo a change file whose body discusses `plan:` is normal content (ADR-0057). Fixture ar5 is what
# pins that; mutation 3 in Task 3 is what proves the pin can fail.
#
# Advisory only: warn-only, never EXPLAINED, never board-row-dropped, never mutates a file.
AR_PLAN_NEW="docs/superpowers/plans/2026-08-03-aborted.md"
AR_RESULTS_NEW="docs/results/2026-08-03-aborted-results.md"

# ar_branch REPO BRANCH PATH — cut BRANCH from main in REPO and commit PATH on it, so PATH exists
# on BRANCH and NOT on main. Leaves the repo parked back on docket (the metadata working tree).
ar_branch(){
  local arb_repo="$1" arb_br="$2" arb_path="$3"
  git -C "$arb_repo" checkout -b "$arb_br" main >/dev/null 2>&1
  mkdir -p "$arb_repo/$(dirname "$arb_path")"
  printf '# artifact\n' > "$arb_repo/$arb_path"
  git -C "$arb_repo" add "$arb_path"
  git_quiet -C "$arb_repo" commit -m "artifact on $arb_br"
  git -C "$arb_repo" checkout docket >/dev/null 2>&1
}

# --- RED: the branch carries a plan the manifest does not record ---
read -r AR1 _ < <(new_repo)
ar_branch "$AR1" feat/ar1 "$AR_PLAN_NEW"
cat > "$AR1/docs/changes/active/0201-plan-unrecorded.md" <<'EOF'
---
id: 201
slug: plan-unrecorded
title: Plan committed, plan field never written
status: in-progress
priority: medium
depends_on: []
branch: feat/ar1
plan:
results:
---
EOF
ar1out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR1/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg A fires for a committed plan with an empty plan: (id 201)" \
  'has_finding "$ar1out" aborted-run 201'
ar1line="$(grep -E "$(printf "^aborted-run\t201\t")" <<<"$ar1out")"
assert "the leg-A plan finding names the plan: field and the offending path (id 201)" \
  'grep -qF -- "plan: is unset" <<<"$ar1line" && grep -qF -- "$AR_PLAN_NEW" <<<"$ar1line"'
assert "aborted-run fires exactly ONCE for id 201 (leg B must stay silent on a fresh claim)" \
  '[ "$(grep -cE "$(printf "^aborted-run\t201\t")" <<<"$ar1out")" = 1 ]'

# --- RED: the branch carries a results file the manifest does not record ---
read -r AR2 _ < <(new_repo)
ar_branch "$AR2" feat/ar2 "$AR_RESULTS_NEW"
cat > "$AR2/docs/changes/active/0202-results-unrecorded.md" <<'EOF'
---
id: 202
slug: results-unrecorded
title: Results committed, results field never written
status: in-progress
priority: medium
depends_on: []
branch: feat/ar2
plan: docs/superpowers/plans/2026-06-01-present.md
results:
---
EOF
ar2out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR2/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg A fires for a committed results file with an empty results: (id 202)" \
  'has_finding "$ar2out" aborted-run 202'
ar2line="$(grep -E "$(printf "^aborted-run\t202\t")" <<<"$ar2out")"
assert "the leg-A results finding names the results: field and the offending path (id 202)" \
  'grep -qF -- "results: is unset" <<<"$ar2line" && grep -qF -- "$AR_RESULTS_NEW" <<<"$ar2line"'

# --- GREEN: the healthy in-flight build. Branch carries a plan AND the field records it. ---
read -r AR3 _ < <(new_repo)
ar_branch "$AR3" feat/ar3 "$AR_PLAN_NEW"
cat > "$AR3/docs/changes/active/0203-healthy.md" <<EOF
---
id: 203
slug: healthy
title: Healthy in-flight build
status: in-progress
priority: medium
depends_on: []
branch: feat/ar3
plan: $AR_PLAN_NEW
results:
claimed_at: $(iso $(( NOW_EPOCH - 3600 )))
---
EOF
ar3out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR3/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT for a healthy in-flight build: plan committed AND plan: set (id 203)" \
  '! has_finding "$ar3out" aborted-run 203'

# --- GREEN: an in-progress change with a branch that carries NO new artifact at all. Leg A is
# time-free and must not fire merely because a claim exists; leg B (Task 2) is what covers this
# shape, and its claim here is fresh.
read -r AR4 _ < <(new_repo)
git -C "$AR4" checkout -b feat/ar4 main >/dev/null 2>&1
git -C "$AR4" checkout docket >/dev/null 2>&1
cat > "$AR4/docs/changes/active/0204-nothing-yet.md" <<EOF
---
id: 204
slug: nothing-yet
title: Claimed, branch cut, nothing built yet
status: in-progress
priority: medium
depends_on: []
branch: feat/ar4
plan:
results:
claimed_at: $(iso $(( NOW_EPOCH - 3600 )))
---
EOF
ar4out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR4/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT for a fresh claim whose branch carries no new artifact (id 204)" \
  '! has_finding "$ar4out" aborted-run 204'

# --- THE LOAD-BEARING anchoring fixture: frontmatter OMITS plan: entirely while the BODY opens a
# plan: line. fm_field is anchored to the first ---...--- block, so plan: reads EMPTY and the check
# FIRES (the artifact is genuinely unrecorded). An UNANCHORED field() would read the body prose as
# a set plan: and go silently green — the false-negative this whole change exists to prevent.
read -r AR5 _ < <(new_repo)
ar_branch "$AR5" feat/ar5 "$AR_PLAN_NEW"
cat > "$AR5/docs/changes/active/0205-body-prose-plan.md" <<'EOF'
---
id: 205
slug: body-prose-plan
title: Omits plan in frontmatter, discusses it in the body
status: in-progress
priority: medium
depends_on: []
branch: feat/ar5
---

## Notes
plan: docs/superpowers/plans/2026-06-01-present.md
EOF
ar5out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR5/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg A FIRES when plan: is absent from frontmatter and only body prose mentions it (id 205, anchored read)" \
  'has_finding "$ar5out" aborted-run 205'

# --- GREEN: status gate. The identical incoherence on a NON-in-progress change is silent.
read -r AR6 _ < <(new_repo)
ar_branch "$AR6" feat/ar6 "$AR_PLAN_NEW"
cat > "$AR6/docs/changes/active/0206-proposed.md" <<'EOF'
---
id: 206
slug: proposed-incoherent
title: Same incoherence but not in-progress
status: proposed
priority: medium
depends_on: []
branch: feat/ar6
plan:
results:
EOF
ar6out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR6/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT on a 'proposed' change with the same incoherence (id 206, status gate)" \
  '! has_finding "$ar6out" aborted-run 206'

# --- GREEN: an artifact that is on the branch AND on the integration branch is already merged
# work, not an unrecorded artifact. The template's main carries
# docs/superpowers/plans/2026-06-01-present.md; a branch that merely inherits it must not fire.
read -r AR7 _ < <(new_repo)
git -C "$AR7" checkout -b feat/ar7 main >/dev/null 2>&1
echo unrelated > "$AR7/unrelated.txt"; git -C "$AR7" add unrelated.txt
git_quiet -C "$AR7" commit -m "unrelated code commit"
git -C "$AR7" checkout docket >/dev/null 2>&1
cat > "$AR7/docs/changes/active/0207-inherited.md" <<EOF
---
id: 207
slug: inherited-artifact
title: Branch inherits main's plan file only
status: in-progress
priority: medium
depends_on: []
branch: feat/ar7
plan:
results:
claimed_at: $(iso $(( NOW_EPOCH - 3600 )))
---
EOF
ar7out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR7/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT when the branch's only plan file is INHERITED from the integration branch (id 207)" \
  '! has_finding "$ar7out" aborted-run 207'

# --- The --results-dir flag: a repo whose results live somewhere else is honored.
read -r AR8 _ < <(new_repo)
ar_branch "$AR8" feat/ar8 "docs/custom-results/2026-08-03-x-results.md"
cat > "$AR8/docs/changes/active/0208-custom-results.md" <<'EOF'
---
id: 208
slug: custom-results
title: Custom results dir
status: in-progress
priority: medium
depends_on: []
branch: feat/ar8
plan: docs/superpowers/plans/2026-06-01-present.md
results:
---
EOF
ar8_default="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR8/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT for a custom results dir when --results-dir is NOT passed (default docs/results)" \
  '! has_finding "$ar8_default" aborted-run 208'
ar8_custom="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR8/docs/changes" --metadata-branch docket --integration-branch main --results-dir docs/custom-results 2>/dev/null)"
assert "aborted-run FIRES for a custom results dir when --results-dir names it (id 208)" \
  'has_finding "$ar8_custom" aborted-run 208'
```

Note `iso()` is already defined at line ~309 of the test file (the `stale-in-progress` section), which runs before this block — reuse it, do not redefine it.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E "NOT OK|^FAIL|^PASS"`
Expected: `NOT OK` lines for every `fires`/`FIRES` assert above (the check does not exist yet), and `FAIL` at the end. The `SILENT` asserts pass vacuously at this point — that is expected and is exactly why Task 3's mutation tests exist.

- [ ] **Step 3: Declare the check-id in the registry**

In `scripts/lib/docket-frontmatter.sh`, replace lines 306-308:

```bash
BOARD_CHECK_IDS=(aborted-run adr-unpublished board-row-dropped broken-plan-results broken-spec
                 dep-cycle field-domain malformed-id merge-gate-stall merged-orphan
                 publish-deferred scalar-form stale-finalize-blocked stale-in-progress
                 unknown-commit-ref)
```

- [ ] **Step 4: Add the `--results-dir` flag and the two constants to `board-checks.sh`**

In the argument loop (after the `--adrs-dir` case, ~line 37), add:

```bash
    --results-dir) RESULTS_DIR_REL="$2"; shift ;;
```

After the `LEASE_TTL_HOURS="${LEASE_TTL_HOURS:-72}"` line (~line 44), add:

```bash
# Repo-RELATIVE artifact directories for the aborted-run leg-A probe (change 0113). Unlike
# --changes-dir/--adrs-dir (filesystem paths), these are addressed as `<ref>:<path>` and
# `ls-tree --full-tree`, which are always worktree-root-relative. --results-dir defaults to the
# convention's own default so a standalone hand-run stays sane; docket-status.sh passes the
# resolved RESULTS_DIR. The plans dir has no config knob in the convention (the plan path is fixed
# by the plan role's own default), so it is a constant here rather than a flag nobody would set.
RESULTS_DIR_REL="${RESULTS_DIR_REL:-docs/results}"
PLANS_DIR_REL="docs/superpowers/plans"
```

Beside `FINALIZE_BLOCKED_STALE_SECS` (~line 103), add:

```bash
# Run-scale staleness horizon for aborted-run's leg B (change 0113). Hardcoded, no config knob —
# same precedent as FINALIZE_BLOCKED_STALE_SECS above and stale-in-progress's 3*86400 branch-idle
# threshold. 12h is six times tighter than the 72h lease default: tight enough that a /loop drain
# trips over an abort on its next iteration, loose enough to leave room for a marathon build. When
# a genuinely long build does trip it the finding is free, self-clearing, and worth a glance.
ABORTED_RUN_STALE_SECS=$(( 12 * 3600 ))
```

- [ ] **Step 5: Add the two helpers**

Immediately after `git_has` (~line 67) in `scripts/board-checks.sh`:

```bash
# branch_ref BRANCH — print the first ref name that resolves for BRANCH (local first, then the
# origin remote-tracking ref) and exit 0; exit 1 with empty stdout when neither resolves or BRANCH
# is empty. Single source for "does this change's feature branch exist at all", shared by
# stale-in-progress's has_branch test and aborted-run's leg A.
branch_ref(){
  local br="$1"
  [ -n "$br" ] || return 1
  if "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/heads/$br"; then
    printf '%s' "refs/heads/$br"; return 0
  fi
  if "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$br"; then
    printf '%s' "refs/remotes/origin/$br"; return 0
  fi
  return 1
}

# branch_only_artifact REF DIR — print the first path under DIR that exists on REF but NOT on
# INTEGRATION_BRANCH, and exit 0; exit 1 with empty stdout when DIR is empty on REF or every path
# under it is already on the integration branch (inherited, i.e. already-merged work).
# --full-tree makes DIR worktree-root-relative regardless of the `-C "$CHANGES_DIR"` cwd, which is
# a subdirectory. Captured into a variable and consumed from a here-string rather than piped: this
# file runs under `set -uo pipefail`, where an early `return` out of a piped consumer races the
# producer.
branch_only_artifact(){
  local boa_ref="$1" boa_dir="$2" boa_list boa_p
  boa_list="$("$GIT" -C "$CHANGES_DIR" ls-tree -r --name-only --full-tree "$boa_ref" -- "$boa_dir" 2>/dev/null)"
  [ -n "$boa_list" ] || return 1
  while IFS= read -r boa_p; do
    [ -n "$boa_p" ] || continue
    git_has "$INTEGRATION_BRANCH" "$boa_p" || { printf '%s' "$boa_p"; return 0; }
  done <<<"$boa_list"
  return 1
}
```

- [ ] **Step 6: Route `stale-in-progress`'s branch test through the shared helper**

In the `stale-in-progress` block (~lines 287-296), replace the inline `has_branch` resolution:

```bash
    branch="$(field "$f" branch)"
    claimed="$(field "$f" claimed_at)"
    has_branch=0
    if [ -n "$branch" ]; then
      if "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/heads/$branch" \
         || "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$branch"; then
        has_branch=1
      fi
    fi
```

with:

```bash
    branch="$(field "$f" branch)"
    claimed="$(field "$f" claimed_at)"
    has_branch=0
    if branch_ref "$branch" >/dev/null; then has_branch=1; fi
```

Behavior is identical (same two refs, same order, same empty-branch short-circuit); this is a single-source move, not a semantic change. The existing `stale-in-progress` asserts are the regression gate.

- [ ] **Step 7: Add the `aborted-run` block (leg A only for now)**

In `scripts/board-checks.sh`, immediately after the closing `fi` of the `stale-in-progress` block (~line 315) and before the `# --- merge-gate-stall:` comment, insert:

```bash
  # --- aborted-run: an in-progress change whose autonomous run stopped mid-step (change 0113).
  # An agent that dropped its bookkeeping write is the least reliable narrator of whether it
  # dropped it — both observed incidents produced confident, specific, WRONG completion reports —
  # so the oracle has to be external and mechanical. Two INDEPENDENT legs; either emits, and both
  # can emit on one change (they describe different evidence, not two views of one).
  #
  # Advisory only. It flips no status, releases no claim, and touches no file: the originating
  # incident left a real written plan a naive claim release would have stranded, and this script is
  # a pure reader by contract. Never marks EXPLAINED and never feeds board-row-dropped — a dropped
  # metadata write does not drop a board row.
  #
  # Every field here is read with the ANCHORED fm_field, never field(): plan/results/branch/
  # claimed_at are all OPTIONAL, and an unanchored read falls through the closing --- into body
  # prose (ADR-0057). In THIS repo that is not a contrived hazard — a change file whose body
  # discusses `plan:` is ordinary content, and the failure it would cause is a silent FALSE
  # NEGATIVE: prose read as a set plan: makes the check certify the exact abort it exists to catch.
  if [ "$status" = "in-progress" ]; then
    ar_branch="$(fm_field "$f" branch)"

    # Leg A — manifest/git incoherence, time-free. The feature branch carries an artifact file the
    # integration branch does not have, while the manifest field that should record it is empty.
    # The exact INVERSE of broken-plan-results (field set, file missing on the integration branch):
    # same two fields, same two trees, opposite direction. Its only false-positive window is the
    # seconds between an artifact commit and its field write, and since the finding is advisory and
    # self-clearing that race costs nothing.
    if ar_ref="$(branch_ref "$ar_branch")"; then
      if [ -z "$(fm_field "$f" plan)" ] && ar_hit="$(branch_only_artifact "$ar_ref" "$PLANS_DIR_REL")"; then
        emit aborted-run "$id" "plan committed on $ar_branch ($ar_hit) but plan: is unset — the run stopped before its metadata write; record it or re-run the step"
      fi
      if [ -z "$(fm_field "$f" results)" ] && ar_hit="$(branch_only_artifact "$ar_ref" "$RESULTS_DIR_REL")"; then
        emit aborted-run "$id" "results committed on $ar_branch ($ar_hit) but results: is unset — the run stopped before its metadata write; record it or re-run the step"
      fi
    fi
  fi
```

- [ ] **Step 8: Update the script's own header comment**

In `scripts/board-checks.sh`, replace lines 8-15's usage/check-id block so the flag and the id are both documented:

```bash
# Usage: board-checks.sh --changes-dir DIR --metadata-branch BR --integration-branch BR [--strict]
#                         [--lease-ttl-hours N] [--adrs-dir DIR] [--terminal-publish]
#                         [--results-dir REPO-RELATIVE-DIR]
#   Findings: TAB-separated  <check-id>\t<change-id>\t<message>  on stdout, sorted by (check-id, change-id).
#     check-id ∈ {aborted-run, adr-unpublished, board-row-dropped, broken-spec, broken-plan-results,
#                 dep-cycle, field-domain, malformed-id, publish-deferred, scalar-form,
#                 stale-in-progress, merge-gate-stall, stale-finalize-blocked, merged-orphan,
#                 unknown-commit-ref}
#     The set above is declared in lib/docket-frontmatter.sh as BOARD_CHECK_IDS and pinned to it,
#     to board-checks.md, and to docket-status.md by tests/test_board_checks.sh — edit all four.
```

- [ ] **Step 9: Pass `--results-dir` from `docket-status.sh`**

In `scripts/docket-status.sh`, in `health_checks`, change the invocation (lines 731-734) to add the flag:

```bash
  out="$("$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/board-checks.sh \
    --changes-dir "$cd_dir" --metadata-branch "$metadata_branch" \
    --integration-branch "origin/$INTEGRATION_BRANCH" \
    --results-dir "${RESULTS_DIR:-docs/results}" \
    --lease-ttl-hours "${RECLAIM_LEASE_TTL:-72}" ${adr_args[@]+"${adr_args[@]}"} 2>&2)" || rc=$?
```

The `:-docs/results` default guards a stale or mocked config export that does not emit the key — the same shape as the `${TERMINAL_PUBLISH:-false}` guard eight lines above.

- [ ] **Step 10: Bump the registry count literal in the test**

In `tests/test_board_checks.sh`, replace lines 1806-1809:

```bash
# 15 since change 0113 added aborted-run (14 at 0191's scalar-form; 13 at 0117's adr-unpublished).
# This literal is the ONE hand-edit the derived set-compares below do not absorb — bump it with
# every new id.
assert "BOARD_CHECK_IDS holds the 15 check-ids board-checks.sh emits" \
  '[ "${#BOARD_CHECK_IDS[@]}" = 15 ]'
```

- [ ] **Step 11: Document the check in `scripts/board-checks.md`**

Add `[--results-dir REPO-RELATIVE-DIR]` to the Usage block, with a line in the options list reading:

```markdown
`--results-dir` — repo-relative results directory scanned by `aborted-run`'s leg A. Optional;
defaults to `docs/results` (the convention's own default) so a hand-run stays sane. Unlike
`--changes-dir` and `--adrs-dir` this is a **repo-relative** path, not a filesystem path: it is
addressed through `<ref>:<path>` and `ls-tree --full-tree`, which are worktree-root-relative.
```

And insert a full check entry immediately before the `**`board-row-dropped`**` entry (~line 196):

```markdown
**`aborted-run`** — An `in-progress` change whose autonomous run stopped mid-step: it completed the
visible artifact, narrated success, and dropped the metadata write. The oracle is deliberately
**external** — the agent that dropped the bookkeeping write is the least reliable narrator of
whether it dropped it, and the observed incidents produced confident, specific, wrong reports, so a
check keyed on hedging in the report catches nothing. Gated on `status: in-progress`. Two
independent legs; either emits, and both may emit on one change.

- **Leg A — manifest/git incoherence (time-free).** The change's `branch:` carries a file under
  `docs/superpowers/plans/` (or under `--results-dir`) that is **absent from the integration
  branch**, while `plan:` (resp. `results:`) is empty. This is the exact **inverse of
  `broken-plan-results`**, which catches *field set, file missing*; leg A catches *file present,
  field empty*. Same two fields, same two trees, opposite direction. An artifact the branch merely
  **inherits** from the integration branch is already-merged work and never fires. The only
  false-positive window is the seconds between an artifact commit and its field write — advisory
  and self-clearing, so the race costs nothing.
- **Leg B — run-scale stale claim (time-based).** `claimed_at:` older than **12 hours**. This
  catches the abort that leaves nothing in git at all (a plan written but never committed), which
  leg A structurally cannot see.

**A separate check-id, not a widened `stale-in-progress`.** That check keys on the same
`claimed_at:` field but at a *human-scale abandonment* horizon (the 72h lease TTL, plus a 3-day
branch-idle signal), with a different remedy — "this looks abandoned, reclaim it" — and a machine
contract `docket-status` keys on, the trailing `[reclaimable]` marker. `aborted-run`'s remedy is
"this run stopped mid-step, go look". Widening the incumbent predicate would silently change what
an already-written consumer sees.

The 12h window is **hardcoded**, matching `stale-finalize-blocked`'s 72h and `stale-in-progress`'s
3-day branch-idle horizon; only the lease TTL is a knob.

**Not detected, deliberately:** *"a PR is open but `pr:` is empty"* would need a network probe,
which this script forbids by contract. *"Build commits present while `in-progress`"* is what a
healthy in-flight build looks like, not incoherence.

Warn-only, like every check here: it never marks `EXPLAINED`, never feeds `board-row-dropped`, and
never mutates the change file. Every field it reads (`branch`, `plan`, `results`, `claimed_at`) is
optional, so all four go through the **anchored** `fm_field` (ADR-0057) — an unanchored read would
take body prose for a set field and certify the very abort the check exists to catch.
```

Also add the row to the check-id table at the top of the file if one exists there (search for `broken-plan-results` near the head of the document and mirror whatever list shape you find; the test set-compares `board-checks.md` against the emitted set, so the id must appear).

- [ ] **Step 12: Add the id to the report vocabulary**

In `scripts/docket-status.md:364`, add `aborted-run` to the `<check-id>` set so it reads
`… `<check-id>` ∈ {aborted-run, adr-unpublished, board-row-dropped, …}`.

- [ ] **Step 13: Run the board-checks suite to verify green**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E "NOT OK|^FAIL|^PASS"`
Expected: `PASS`, with no `NOT OK` lines.

- [ ] **Step 14: Run the two adjacent suites for regressions**

Run: `bash tests/test_docket_status.sh 2>&1 | tail -5` and `bash tests/test_render_board.sh 2>&1 | tail -5`
Expected: `PASS` from each. (`stale-in-progress` was refactored in Step 6; `docket-status.sh`'s invocation changed in Step 9.)

- [ ] **Step 15: Commit**

```bash
git add scripts/board-checks.sh scripts/board-checks.md scripts/docket-status.sh scripts/docket-status.md scripts/lib/docket-frontmatter.sh tests/test_board_checks.sh
git commit -m "feat(0113): aborted-run check-id, leg A (manifest/git incoherence)"
```

---

### Task 2: Leg B — the run-scale stale claim

**Files:**
- Modify: `scripts/board-checks.sh` (inside the `# --- aborted-run:` block added in Task 1)
- Test: `tests/test_board_checks.sh` (append to the `aborted-run` section)

**Interfaces:**
- Consumes: `ABORTED_RUN_STALE_SECS`, `iso_to_epoch`, `NOW`, `emit`, and the `if [ "$status" = "in-progress" ]` block from Task 1.
- Produces: nothing new for later tasks; Task 3 mutates both legs.

- [ ] **Step 1: Write the failing tests**

Append to the `aborted-run` section of `tests/test_board_checks.sh` (after the Task 1 fixtures, before the `board-row-dropped` banner):

```bash
# ---------------- aborted-run, leg B: run-scale stale claim (12h, hardcoded) ----------------
# The abort that leaves NOTHING in git — the originating incident, where the plan was written but
# never committed, so leg A has no artifact to see. Deliberately a separate check-id from
# stale-in-progress rather than a retuned one: that check's 72h lease / 3-day branch-idle horizons
# are human-scale abandonment signals with a different remedy and a machine contract
# (the trailing [reclaimable] marker) that docket-status already keys on.
AR_STALE_CLAIM="$(iso $(( NOW_EPOCH - 13*3600 )))"   # 13h  > 12h  => fires
AR_FRESH_CLAIM="$(iso $(( NOW_EPOCH - 11*3600 )))"   # 11h  < 12h  => silent

read -r AR10 _ < <(new_repo)
cat > "$AR10/docs/changes/active/0210-stale-claim.md" <<EOF
---
id: 210
slug: stale-claim
title: Claim older than the run-scale window
status: in-progress
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at: $AR_STALE_CLAIM
---
EOF
ar10out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR10/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg B fires for a claim older than 12h (id 210)" \
  'has_finding "$ar10out" aborted-run 210'
ar10line="$(grep -E "$(printf "^aborted-run\t210\t")" <<<"$ar10out")"
assert "the leg-B finding reports the claim age in hours (id 210)" \
  'grep -qE "13h" <<<"$ar10line"'
# stale-in-progress must stay SILENT here: 13h is far inside its 72h lease TTL. This is the whole
# point of the separate check-id — the two predicates must not have become one.
assert "stale-in-progress SILENT on the same 13h claim (id 210, the two horizons stay distinct)" \
  '! has_finding "$ar10out" stale-in-progress 210'

read -r AR11 _ < <(new_repo)
cat > "$AR11/docs/changes/active/0211-fresh-claim.md" <<EOF
---
id: 211
slug: fresh-claim
title: Claim inside the run-scale window
status: in-progress
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar11out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR11/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg B SILENT for a claim 11h old (id 211, just inside the window)" \
  '! has_finding "$ar11out" aborted-run 211'

# No claimed_at at all => no positive evidence => silent (never treated as infinitely old).
read -r AR12 _ < <(new_repo)
cat > "$AR12/docs/changes/active/0212-no-claim.md" <<'EOF'
---
id: 212
slug: no-claim
title: In-progress with no claimed_at
status: in-progress
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at:
---
EOF
ar12out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR12/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg B SILENT when claimed_at is absent (id 212, no positive evidence)" \
  '! has_finding "$ar12out" aborted-run 212'

# An unparseable claimed_at is also no positive evidence — never an exception, never "expired".
read -r AR13 _ < <(new_repo)
cat > "$AR13/docs/changes/active/0213-bad-claim.md" <<'EOF'
---
id: 213
slug: bad-claim
title: In-progress with a malformed claimed_at
status: in-progress
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at: not-a-timestamp
---
EOF
ar13out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR13/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg B SILENT for an unparseable claimed_at (id 213)" \
  '! has_finding "$ar13out" aborted-run 213'

# BOTH legs on one change: an unrecorded plan AND a stale claim => TWO findings, not one.
# The legs are independent evidence, and collapsing them would hide whichever fired second.
read -r AR14 _ < <(new_repo)
ar_branch "$AR14" feat/ar14 "$AR_PLAN_NEW"
cat > "$AR14/docs/changes/active/0214-both-legs.md" <<EOF
---
id: 214
slug: both-legs
title: Unrecorded plan and a stale claim
status: in-progress
priority: medium
depends_on: []
branch: feat/ar14
plan:
results:
claimed_at: $AR_STALE_CLAIM
---
EOF
ar14out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR14/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "both aborted-run legs fire independently on one change (id 214, exactly 2 findings)" \
  '[ "$(grep -cE "$(printf "^aborted-run\t214\t")" <<<"$ar14out")" = 2 ]'

# Status gate again, on leg B this time: a 'proposed' change with an ancient claimed_at is silent.
read -r AR15 _ < <(new_repo)
cat > "$AR15/docs/changes/active/0215-proposed-stale.md" <<EOF
---
id: 215
slug: proposed-stale
title: Proposed with an old claimed_at
status: proposed
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at: $AR_STALE_CLAIM
---
EOF
ar15out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR15/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg B SILENT on a 'proposed' change with an old claimed_at (id 215, status gate)" \
  '! has_finding "$ar15out" aborted-run 215'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E "NOT OK|^PASS|^FAIL"`
Expected: `NOT OK` for the two leg-B `fires` asserts and the `exactly 2 findings` assert; `FAIL` at the end.

- [ ] **Step 3: Implement leg B**

In `scripts/board-checks.sh`, inside the `# --- aborted-run:` block, after the leg-A `fi` and before the block's closing `fi`, add:

```bash
    # Leg B — run-scale stale claim. Catches the abort that leaves NOTHING in git at all: the plan
    # written to the worktree but never committed, so leg A has no artifact to see. An absent or
    # unparseable claimed_at is NO POSITIVE EVIDENCE and stays silent — never treated as expired
    # (the same posture iso_to_epoch's contract states and stale-in-progress already takes).
    ar_claimed="$(fm_field "$f" claimed_at)"
    if [ -n "$ar_claimed" ]; then
      ar_epoch="$(iso_to_epoch "$ar_claimed")" || ar_epoch=""
      if [ -n "$ar_epoch" ] && [ "$(( NOW - ar_epoch ))" -gt "$ABORTED_RUN_STALE_SECS" ]; then
        emit aborted-run "$id" "claim stamped $(( (NOW - ar_epoch) / 3600 ))h ago, past the 12h run-scale window — a run may have stopped mid-step; verify it reached its PR"
      fi
    fi
```

- [ ] **Step 4: Run the suite to verify it passes**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E "NOT OK|^PASS|^FAIL"`
Expected: `PASS`, no `NOT OK`.

- [ ] **Step 5: Commit**

```bash
git add scripts/board-checks.sh tests/test_board_checks.sh
git commit -m "feat(0113): aborted-run leg B — the run-scale stale claim"
```

---

### Task 3: Mutation tests — prove every predicate can fail

**Files:**
- Test: `tests/test_board_checks.sh` (append to the `aborted-run` section, after Task 2's fixtures)

**Interfaces:**
- Consumes: the `mreseed` / `MUTSCRIPT` pattern from the `scalar-form` mutation block (~lines 789-876). Note `mreseed` and `mcopy` are already defined there and `rm -rf "$mcopy"` runs at the end of that block; re-declare a fresh reseeder scoped to this section rather than reusing a torn-down one.
- Produces: nothing consumed later.

**This task is not optional rigor.** Three predicates ship (the two leg-A pairs and leg B); every one gets a mutation that breaks it and a proof the mutation *landed*. A mutation that silently fails to match yields a green run with nothing mutated, which reads exactly like a robust guard.

- [ ] **Step 1: Write the mutation block**

Append to the `aborted-run` section of `tests/test_board_checks.sh`:

```bash
# ---------------- aborted-run mutation tests (guards-are-code) ----------------
# Each predicate is broken in a throwaway COPY of board-checks.sh and watched change the fixtures'
# outcome. Every mutation runs against a FRESH pristine copy (never a cumulative chain) and is
# CONFIRMED LANDED with a grep -c before/after before its result is believed.
read -r ARM _ < <(new_repo)
ar_branch "$ARM" feat/arm-plan    "$AR_PLAN_NEW"
git -C "$ARM" checkout -b feat/arm-results main >/dev/null 2>&1
mkdir -p "$ARM/docs/results"
printf '# artifact\n' > "$ARM/$AR_RESULTS_NEW"
git -C "$ARM" add "$AR_RESULTS_NEW"; git_quiet -C "$ARM" commit -m "results on feat/arm-results"
git -C "$ARM" checkout docket >/dev/null 2>&1
# 220: unrecorded plan, FRESH claim  -> leg A (plan) only
cat > "$ARM/docs/changes/active/0220-mplan.md" <<EOF
---
id: 220
slug: mplan
title: Unrecorded plan
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-plan
plan:
results:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 221: unrecorded results, FRESH claim -> leg A (results) only
cat > "$ARM/docs/changes/active/0221-mresults.md" <<EOF
---
id: 221
slug: mresults
title: Unrecorded results
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-results
plan: docs/superpowers/plans/2026-06-01-present.md
results:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 222: STALE claim, no branch -> leg B only
cat > "$ARM/docs/changes/active/0222-mclaim.md" <<EOF
---
id: 222
slug: mclaim
title: Stale claim only
status: in-progress
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at: $AR_STALE_CLAIM
---
EOF
# 223: plan absent from FRONTMATTER, present in BODY prose, unrecorded plan on the branch, fresh
# claim -> leg A fires under the ANCHORED read and goes silent under an unanchored one.
cat > "$ARM/docs/changes/active/0223-manchor.md" <<EOF
---
id: 223
slug: manchor
title: Body prose mentions plan
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-plan
results:
claimed_at: $AR_FRESH_CLAIM
---

## Notes
plan: docs/superpowers/plans/2026-06-01-present.md
EOF

armcopy=""
armreseed(){
  [ -n "$armcopy" ] && rm -rf "$armcopy"; armcopy="$(mktemp -d)"
  mkdir -p "$armcopy/scripts/lib"
  cp "$SCRIPT" "$armcopy/scripts/board-checks.sh"
  cp "$REPO/scripts/lib/docket-frontmatter.sh" "$armcopy/scripts/lib/"
  ARMSCRIPT="$armcopy/scripts/board-checks.sh"
}
armrun(){ NOW=$NOW_EPOCH bash "$ARMSCRIPT" --changes-dir "$ARM/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null; }

# Baseline: the un-mutated copy fires exactly the three expected findings.
armreseed
arm0out="$(armrun)"
assert "mutation baseline: unmutated copy fires leg A on 220 (plan)" 'has_finding "$arm0out" aborted-run 220'
assert "mutation baseline: unmutated copy fires leg A on 221 (results)" 'has_finding "$arm0out" aborted-run 221'
assert "mutation baseline: unmutated copy fires leg B on 222 (stale claim)" 'has_finding "$arm0out" aborted-run 222'
assert "mutation baseline: unmutated copy fires leg A on 223 (anchored read)" 'has_finding "$arm0out" aborted-run 223'

# Mutation A — invert leg A's plan emptiness test (-z becomes -n): the unrecorded-plan fixture 220
# goes GREEN and the healthy-field fixture 221 (plan: SET) starts misfiring. Both directions.
armreseed
armA_before="$(grep -cF 'if [ -z "$(fm_field "$f" plan)" ]' "$ARMSCRIPT")"
awk '{ if ($0 ~ /fm_field "\$f" plan/) sub(/-z /, "-n "); print }' "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armA_after="$(grep -cF 'if [ -z "$(fm_field "$f" plan)" ]' "$ARMSCRIPT")"
armAout="$(armrun)"
assert "mutation A landed: leg A's plan emptiness test is inverted (count 1 -> 0)" \
  '[ "$armA_before" = 1 ] && [ "$armA_after" = 0 ]'
assert "mutation A (invert plan emptiness): the unrecorded-plan fixture 220 goes GREEN" \
  '! has_finding "$armAout" aborted-run 220'
assert "mutation A: the stale-claim fixture 222 still fires (leg B is independent)" \
  'has_finding "$armAout" aborted-run 222'

# Mutation B — strip leg A's results emit arm: 221 goes GREEN, the plan arm survives on 220.
armreseed
armB_before="$(grep -cF 'but results: is unset' "$ARMSCRIPT")"
awk '!/but results: is unset/' "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armB_after="$(grep -cF 'but results: is unset' "$ARMSCRIPT")"
armBout="$(armrun)"
assert "mutation B landed: leg A's results emit arm is gone (count 1 -> 0)" \
  '[ "$armB_before" = 1 ] && [ "$armB_after" = 0 ]'
assert "mutation B (strip results arm): the unrecorded-results fixture 221 goes GREEN" \
  '! has_finding "$armBout" aborted-run 221'
assert "mutation B: the unrecorded-plan fixture 220 still fires (arm survives)" \
  'has_finding "$armBout" aborted-run 220'

# Mutation C — widen leg B's window from 12h to 1000h: the stale-claim fixture 222 goes GREEN,
# proving the finding is produced by the THRESHOLD and not by the mere presence of claimed_at.
armreseed
armC_before="$(grep -cF 'ABORTED_RUN_STALE_SECS=$(( 12 * 3600 ))' "$ARMSCRIPT")"
awk '{ sub(/ABORTED_RUN_STALE_SECS=\$\(\( 12 \* 3600 \)\)/, "ABORTED_RUN_STALE_SECS=$(( 1000 * 3600 ))"); print }' "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armC_after="$(grep -cF 'ABORTED_RUN_STALE_SECS=$(( 12 * 3600 ))' "$ARMSCRIPT")"
armCout="$(armrun)"
assert "mutation C landed: leg B's window widened to 1000h (12h literal count 1 -> 0)" \
  '[ "$armC_before" = 1 ] && [ "$armC_after" = 0 ]'
assert "mutation C (widen leg B window): the 13h stale-claim fixture 222 goes GREEN" \
  '! has_finding "$armCout" aborted-run 222'
assert "mutation C: the unrecorded-plan fixture 220 still fires (leg A is independent)" \
  'has_finding "$armCout" aborted-run 220'

# Mutation D — unanchor the plan read (fm_field -> field): the body-prose fixture 223 goes GREEN,
# because the unanchored read takes `plan: …` from the body as a set field and certifies the abort.
# This is the FALSE-NEGATIVE direction, and it is the reason every read here is anchored.
armreseed
armD_before="$(grep -cF 'fm_field "$f" plan' "$ARMSCRIPT")"
awk '{ if ($0 ~ /fm_field "\$f" plan/) sub(/fm_field/, "field"); print }' "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armD_after="$(grep -cF 'fm_field "$f" plan' "$ARMSCRIPT")"
armDout="$(armrun)"
assert "mutation D landed: the plan read is unanchored (fm_field count 1 -> 0)" \
  '[ "$armD_before" = 1 ] && [ "$armD_after" = 0 ]'
assert "mutation D (unanchor the plan read): the body-prose fixture 223 goes GREEN — proves the anchoring" \
  '! has_finding "$armDout" aborted-run 223'
assert "mutation D: fixture 220, which has no body plan: line, still fires" \
  'has_finding "$armDout" aborted-run 220'

# Mutation E — drop the whole aborted-run block: every red fixture goes GREEN, and
# stale-in-progress must stay unaffected (the two checks are genuinely separate code).
armreseed
armE_before="$(grep -c 'aborted-run' "$ARMSCRIPT")"
awk '/# --- aborted-run:/{inar=1} inar && /# --- merge-gate-stall:/{inar=0} !inar' "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armE_after="$(grep -c 'aborted-run' "$ARMSCRIPT")"
armEout="$(armrun)"
assert "mutation E landed: the aborted-run block is gone (aborted-run occurrences dropped)" \
  '[ "$armE_before" -ge 3 ] && [ "$armE_after" -lt "$armE_before" ]'
assert "mutation E (drop whole block): fixture 220 goes GREEN" '! has_finding "$armEout" aborted-run 220'
assert "mutation E (drop whole block): fixture 221 goes GREEN" '! has_finding "$armEout" aborted-run 221'
assert "mutation E (drop whole block): fixture 222 goes GREEN" '! has_finding "$armEout" aborted-run 222'
assert "mutation E (drop whole block): fixture 223 goes GREEN" '! has_finding "$armEout" aborted-run 223'
rm -rf "$armcopy"
```

- [ ] **Step 2: Run the suite and confirm every mutation landed**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E "NOT OK|^PASS|^FAIL"`
Expected: `PASS`. If a `mutation X landed` assert is `NOT OK`, the substitution did not match — **fix the `awk` pattern against the real file text, never the assert.** A mutation that did not land makes every downstream assert in that mutation meaningless.

Mutation E's `awk` range depends on the `# --- aborted-run:` block sitting immediately before `# --- merge-gate-stall:` in `board-checks.sh`. If Task 1 Step 7 placed the block elsewhere, adjust the terminating pattern to whatever comment actually follows it and re-verify the `landed` assert.

- [ ] **Step 3: Commit**

```bash
git add tests/test_board_checks.sh
git commit -m "test(0113): mutation tests for both aborted-run legs"
```

---

### Task 4: The two prose riders

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (the `### The field-write rule` paragraph; the `### Step 5 — Build` paragraph)
- Modify: `tests/test_skill_size_budgets.sh:187` (the word budget)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: nothing for later tasks. Independent of Tasks 1-3; a reviewer could reject this and keep the check.

The skill file is at **3939 words against a 3950 budget** — 11 words of headroom. Both edits add words, so the budget bump is part of this task, not a follow-up.

- [ ] **Step 1: Densify the claim heartbeat**

In `skills/docket-implement-next/SKILL.md`, in `### The field-write rule`, replace:

```
Each LATER phase-boundary metadata commit (reconcile, `implemented`) also RE-STAMPS `claimed_at: <UTC ISO-8601 now>` — a zero-cost heartbeat for the claim lease.
```

with:

```
EVERY later metadata commit this skill makes — reconcile, `plan:`, `adrs:`, `pr:`, `results:`, `implemented` — also RE-STAMPS `claimed_at: <UTC ISO-8601 now>`. The commits are already happening, so the heartbeat is free, and it puts a stamp at the plan→build seam where a stopped run is otherwise invisible for the whole build span.
```

- [ ] **Step 2: Split the fused §5 sentence**

In `### Step 5 — Build`, replace the fused clause:

```
is invoked **DIRECTED to:** execute the plan task-by-task and stop at the executed plan, answering any choice it poses from resolved config and never surfacing one — log one line naming the role and skill if you suppressed a hand-off; docket-build routes each task to a profile agent and gates on one full-suite run.
```

with two separately-stated obligations:

```
is invoked **DIRECTED to:** execute the plan task-by-task and stop at the executed plan. **Proceed through the build — the deliverable is the executed plan, never the decision about how to execute it.** Separately, and without ever relaxing the first: answer any choice it poses from resolved config, surface none, and log one line naming the role and skill if you suppressed a hand-off. Emitting that log line discharges the suppression obligation only; the step is not complete until its git-state postcondition holds. docket-build routes each task to a profile agent and gates on one full-suite run.
```

- [ ] **Step 3: Verify the size budget fails, then raise it**

Run: `bash tests/test_skill_size_budgets.sh 2>&1 | grep -E "NOT OK|^PASS|^FAIL"`
Expected: `NOT OK` naming `skills/docket-implement-next/SKILL.md` over budget.

Then in `tests/test_skill_size_budgets.sh`, add a comment above the table beside the existing 0137/0170 bump notes and raise the word figure on line 187:

```
# skills/docket-implement-next/SKILL.md's WORD budget was raised 3950 -> 4050 by change 0113, whose
# two riders split the §5 fused proceed/stay-silent sentence into separately-stated obligations and
# densified the claimed_at heartbeat from two phase boundaries to every metadata commit. Both are
# additions to prose that two observed runs demonstrably misread; the words buy the disambiguation.
```

and change the row to:

```
skills/docket-implement-next/SKILL.md                      147 4050
```

Set the LINE figure only if the line count actually exceeds 147 — check with `wc -lw skills/docket-implement-next/SKILL.md` and use the real number plus a small margin, matching the table's existing style.

- [ ] **Step 4: Run the size-budget and skill-wiring suites**

Run: `bash tests/test_skill_size_budgets.sh 2>&1 | tail -3` and `bash tests/test_skill_facade_wiring.sh 2>&1 | tail -3`
Expected: `PASS` from each.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-implement-next/SKILL.md tests/test_skill_size_budgets.sh
git commit -m "docs(0113): densify the claim heartbeat and split the fused build-step sentence"
```

---

## Final gate

- [ ] **Run the full suite in ONE foreground call** and confirm it is green before the branch is reviewed.

The repo has no single runner script; the suite is the `tests/test_*.sh` glob (the same set
`scripts/profile-asserts.sh` defaults to). Run it in ONE foreground call:

```bash
cd "$(git rev-parse --show-toplevel)" && \
for t in tests/test_*.sh; do
  out="$(bash "$t" 2>&1)"; printf '%s %s\n' "$(printf '%s' "$out" | tail -1)" "$t"
done
```

Expected: every line reads `PASS tests/test_<name>.sh`. Any `FAIL` line names the suite to fix.

## Self-review notes

- **Spec coverage.** Leg A's two pairs → Task 1. Leg B → Task 2. Mutation tests for all three predicates plus the negative fixtures the spec names (healthy in-flight build; `plan:` correctly set alongside its committed plan) → Tasks 1-3. Heartbeat densification and the §5 split → Task 4. Ripple list: `docket-frontmatter.sh`, `board-checks.sh`, `board-checks.md`, `docket-status.md`, `test_board_checks.sh`, `docket-implement-next/SKILL.md` all covered; `docs/adrs/0044-*.md` is intentionally excluded from the feature branch and routed through the metadata branch instead.
- **Two surfaces the spec's ripple list did not name** and this plan adds: `scripts/docket-status.sh` (must pass the new `--results-dir`, or the check silently uses the default in a repo that configured another results dir) and `tests/test_skill_size_budgets.sh` (11 words of headroom cannot absorb the riders).
- **`shared-resource-keeps-first-owner-assumptions`** applies to Task 1 Step 6: `branch_ref` gains a second owner. The incumbent's behavior is pinned by the existing `stale-in-progress` asserts, which must stay green; the multi-owner property — both checks resolving the same branch — is exercised by fixture 214, where leg A and leg B fire on one change.
