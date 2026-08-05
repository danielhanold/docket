<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0211 — aborted-run is blind to a run that stops after the build: commits on an unpushed branch, every field coherent](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-05-0211-aborted-run-is-blind-to-a-run-that-stops-after-the-build-com.md)**
<!-- docket:backlink:end -->

# aborted-run leg C — built but not delivered: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a third leg to `aborted-run` in `scripts/board-checks.sh` that detects an `in-progress` change whose feature branch carries commits ahead of the integration branch, has gone quiet, and has no PR recorded — the "built but not delivered" abort signature both existing legs are structurally blind to.

**Architecture:** One new leg inside the existing `if [ "$status" = "in-progress" ]` block, after leg B, reusing leg A's `ar_ref` (with an explicit non-empty re-guard). No new check-id — `aborted-run` stays the single id, so the four-place `BOARD_CHECK_IDS` pinning is untouched. One emit site with a two-way message branch on whether the branch was ever pushed. Advisory and warn-only like the rest of the file: no status flip, no claim release, no file write. Tests are hermetic fixtures in `tests/test_board_checks.sh`, each predicate mutation-tested against the shared `$ARM` repo.

**Tech Stack:** bash (`set -uo pipefail`, no `errexit`), git plumbing through the file's `GIT` mock seam, the `NOW` epoch mock seam, `tests/test_board_checks.sh`'s `assert`/`has_finding`/`new_repo` harness.

## Global Constraints

- **Every git call goes through the `GIT` seam and the `-C "$CHANGES_DIR"` anchor** — `"$GIT" -C "$CHANGES_DIR" …`, never a bare `git`. This is how `branch_ref`, `git_has`, and `stale-in-progress` already call git, and how the tests inject a mock.
- **Every frontmatter read uses the ANCHORED `fm_field`, never `field`** (ADR-0057). `pr:` is an OPTIONAL key; an unanchored read falls through the closing `---` into body prose. In this repo a change body discussing `pr:` is ordinary content.
- **`scripts/board-checks.sh` runs under `set -uo pipefail` with NO `errexit`.** `"${ar_bases[@]}"` on an empty array is a `set -u` error under older bash, so the empty case must be gated explicitly before the expansion is reached.
- **`board-checks.sh` is a pure reader by contract** — git-only, no `gh`, no network, no file writes, never marks `EXPLAINED`, never feeds `board-row-dropped`.
- **The idle floor is keyed on the branch's newest COMMIT timestamp, never on `claimed_at:`** — the heartbeat rider re-stamps `claimed_at`, which is precisely why leg B misses this signature.
- **`ABORTED_RUN_IDLE_SECS=$(( 2 * 3600 ))`** — hardcoded, no config knob, placed beside `ABORTED_RUN_STALE_SECS`.
- **The integration-branch comparison excludes BOTH bases** — `refs/heads/$INTEGRATION_BRANCH` and `refs/remotes/origin/$INTEGRATION_BRANCH` — each independently `show-ref`-verified. No base resolving ⇒ SILENT.
- **Fixture ids start at 232, not 227.** The spec's assumption 8 said "227, and the build's reconcile re-checks the actual high-water mark" — the reconcile found `tests/test_board_checks.sh` already uses **230 and 231** (the ARQ1/ARQ2 non-ASCII fixtures from change 0202), so 227-229 is only a three-id gap. 232+ is the real high-water start. This is the spec's own instruction, not a deviation from it.
- **New test asserts use literal `grep -qF`/`-cF` or `grep -cE` with an embedded TAB only** — no bounded repetition, no construct BSD grep rejects. The machine's PATH `grep` is ugrep and would mask a portability bug; this change introduces no such construct, so no `/usr/bin/grep` double-run is needed.
- **Every mutation is CONFIRMED LANDED** with a `grep -c` before/after transition assert *and* a `bash -n` validity assert before its outcome is believed — the file's house rule.
- Run the file with: `bash tests/test_board_checks.sh`.

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `scripts/board-checks.sh` | The check implementation | Modify: add `ABORTED_RUN_IDLE_SECS` beside `ABORTED_RUN_STALE_SECS` (~line 172); add leg C after leg B inside the `in-progress` block (~line 425) |
| `tests/test_board_checks.sh` | Hermetic fixtures + mutation tests | Modify: add the `ar_branch_at` dated-commit helper beside `ar_branch` (~line 896); add standalone fixtures 232-238; add ARM fixtures 240-246 and mutations G-L; pin the existing ARM fixtures' leg-C silence |
| `scripts/board-checks.md` | The script's authoritative contract | Modify: extend the `aborted-run` section (~line 198) with leg C and the new constant |

---

### Task 1: Leg C — the predicate and its two messages

The firing half. Two standalone fixtures drive the two message branches; the implementation lands both gates the fixtures discriminate (idle floor, ahead-of-both-bases) plus the `pr:` gate, because the predicate is one conjunction and splitting it across tasks would leave an intermediate state that fires on healthy runs.

**Files:**
- Modify: `scripts/board-checks.sh:167-172` (constant), `scripts/board-checks.sh:415-426` (leg C after leg B)
- Test: `tests/test_board_checks.sh` — new helper beside `ar_branch` (~line 896); fixtures appended after the leg-B `ar15out` block (~line 1302), before the `# ---------------- aborted-run mutation tests` header at line 1304

**Interfaces:**
- Consumes: `branch_ref` (already assigns `ar_ref` in leg A's `if` at line 406), `fm_field`, `emit`, `$GIT`, `$CHANGES_DIR`, `$NOW`, `$INTEGRATION_BRANCH`, `$id`, `$ar_branch` — all already in scope at the insertion point.
- Produces: `ABORTED_RUN_IDLE_SECS` (integer seconds, read by nothing else); the test helper `ar_branch_at REPO BRANCH BASE AGE_SECS [PATH]` (used by Tasks 2 and 3); two `aborted-run` message shapes matched literally by later tasks:
  - never-pushed: `"<N> commits on <branch> ahead of <integration>, branch never pushed and pr: is unset (last commit <H>h ago) — the run stopped before it opened its PR; push and open it, or re-run the step"`
  - push/PR seam: `"<branch> is pushed but pr: is unset (last commit <H>h ago) — the run stopped between the push and the PR record; open the PR or record it"`

- [ ] **Step 1: Add the dated-commit fixture helper**

`ar_branch` (line 896) is called by every existing ARM fixture, so it must stay byte-identical — assumption 10. Add a sibling immediately after it. It needs two capabilities `ar_branch` lacks: a controlled commit date (derived from `NOW_EPOCH`, exactly the way `AR_STALE_CLAIM`/`AR_FRESH_CLAIM` are) and a caller-chosen base ref (the stale-local-`main` fixture cuts from `refs/remotes/origin/main`).

Insert after line 907 (the closing `}` of `ar_branch`):

```bash
# ar_branch_at REPO BRANCH BASE AGE_SECS [PATH] — cut BRANCH from BASE in REPO and, when PATH is
# given, commit PATH on it with BOTH author and committer dates set to AGE_SECS before NOW_EPOCH.
# Leaves the repo parked back on docket, like ar_branch.
#
# A SIBLING of ar_branch, not a widening of it (change 0211): ar_branch is called by every existing
# ARM fixture, and giving it date control would change what those fixtures measure. Byte-identical
# is not the same as unaffected — leg C changes what they COULD emit, which is pinned separately.
#
# The dates are load-bearing, not decoration. ar_branch's commits carry real wall-clock dates while
# NOW_EPOCH is 1750000000 (2025-06), so `NOW - ts` is hugely NEGATIVE for them and they are silent
# for leg C's idle floor only by accident. A leg-C fixture must never inherit that accident: its
# age has to be the thing under test.
ar_branch_at(){
  local aba_repo="$1" aba_br="$2" aba_base="$3" aba_age="$4" aba_path="${5:-}" aba_when
  aba_when="@$(( NOW_EPOCH - aba_age ))"
  git -C "$aba_repo" checkout -b "$aba_br" "$aba_base" >/dev/null 2>&1
  if [ -n "$aba_path" ]; then
    mkdir -p "$aba_repo/$(dirname "$aba_path")"
    printf '# artifact\n' > "$aba_repo/$aba_path"
    git -C "$aba_repo" add "$aba_path"
    GIT_AUTHOR_DATE="$aba_when" GIT_COMMITTER_DATE="$aba_when" \
      git -C "$aba_repo" commit -m "commit on $aba_br" >/dev/null 2>&1
  fi
  git -C "$aba_repo" checkout docket >/dev/null 2>&1
}

# ar_push REPO BRANCH — publish BRANCH to the fixture's own bare origin, which is what creates
# refs/remotes/origin/<BRANCH> — the ref leg C's message branch probes. Separate from ar_branch_at
# because "was it pushed" is exactly the axis the two leg-C messages split on.
ar_push(){ git -C "$1" push -q origin "$2" >/dev/null 2>&1; }
```

- [ ] **Step 2: Write the failing fixtures for both message branches**

Append after the leg-B `ar15out` assert (line 1302), before the `# ---------------- aborted-run mutation tests` header:

```bash
# ---------------- aborted-run leg C: built but not delivered (change 0211) ----------------
# The signature legs A and B are both blind to: the build finished, the delivery did not. Fixture
# ids start at 232 — 220-226 are the ARM mutation repo and 230-231 are the ARQ non-ASCII fixtures.

# --- RED: commits on an UNPUSHED branch, quiet 3h, pr: unset -> leg C, "never pushed" message ---
read -r AR16 _ < <(new_repo)
ar_branch_at "$AR16" feat/ar16 main $(( 3*3600 )) "$AR_PLAN_NEW"
# Sanity: the branch really is unpushed. Without this the "never pushed" assert below could pass
# because the push silently failed rather than because the message branch is right.
assert "leg C fixture 232 precondition: feat/ar16 has NO remote-tracking ref" \
  '! git -C "$AR16" show-ref --verify --quiet refs/remotes/origin/feat/ar16'
cat > "$AR16/docs/changes/active/0232-built-unpushed.md" <<EOF
---
id: 232
slug: built-unpushed
title: Build finished, branch never pushed
status: in-progress
priority: medium
depends_on: []
branch: feat/ar16
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar16out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR16/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg C FIRES on an unpushed branch ahead of main, quiet 3h, pr: unset (id 232)" \
  'has_finding "$ar16out" aborted-run 232'
assert "leg C names the NEVER-PUSHED remedy on 232" \
  'grep -qF "branch never pushed and pr: is unset" <<<"$ar16out"'
assert "leg C reports the commit count ahead of the integration branch on 232" \
  'grep -qF "1 commits on feat/ar16 ahead of main" <<<"$ar16out"'
assert "leg C reports the branch idle age in hours on 232" \
  'grep -qF "(last commit 3h ago)" <<<"$ar16out"'
assert "leg C is SILENT for leg A on 232 — plan: is recorded, so the legs did not double-count" \
  '! grep -qF "but plan: is unset" <<<"$ar16out"'

# --- RED: the same branch PUSHED, pr: still unset -> leg C, push/PR-seam message ---
read -r AR17 _ < <(new_repo)
ar_branch_at "$AR17" feat/ar17 main $(( 3*3600 )) "$AR_PLAN_NEW"
ar_push "$AR17" feat/ar17
assert "leg C fixture 233 precondition: feat/ar17 HAS a remote-tracking ref (the push landed)" \
  'git -C "$AR17" show-ref --verify --quiet refs/remotes/origin/feat/ar17'
cat > "$AR17/docs/changes/active/0233-built-pushed.md" <<EOF
---
id: 233
slug: built-pushed
title: Branch pushed, PR never recorded
status: in-progress
priority: medium
depends_on: []
branch: feat/ar17
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar17out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR17/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg C FIRES on a PUSHED branch with pr: unset (id 233)" \
  'has_finding "$ar17out" aborted-run 233'
assert "leg C names the PUSH/PR-SEAM remedy on 233" \
  'grep -qF "feat/ar17 is pushed but pr: is unset" <<<"$ar17out"'
assert "leg C does NOT claim 233 was never pushed — the two messages are exclusive" \
  '! grep -qF "branch never pushed" <<<"$ar17out"'
```

- [ ] **Step 3: Run the tests and watch them fail**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E "NOT OK|^ok - aborted-run leg C|^ok - leg C"`

Expected: the two `precondition` asserts pass (they test the fixture, not the feature); every `leg C FIRES` / `leg C names…` / `leg C reports…` assert prints `NOT OK`. If a `precondition` assert is `NOT OK`, stop and fix the helper — the feature asserts below it are meaningless until the fixture is real.

- [ ] **Step 4: Add the idle-floor constant**

In `scripts/board-checks.sh`, immediately after the `ABORTED_RUN_STALE_SECS=$(( 12 * 3600 ))` line (line 172), add:

```bash
# Branch-idle floor for aborted-run's leg C (change 0211). Hardcoded, no config knob — the same
# precedent as ABORTED_RUN_STALE_SECS above, FINALIZE_BLOCKED_STALE_SECS, and stale-in-progress's
# 3*86400. Keyed on the branch's newest COMMIT, never on claimed_at: the heartbeat rider re-stamps
# claimed_at at every phase boundary, which is exactly why leg B is blind to this signature.
#
# 2h, derived: after the last build commit a healthy run still has review, any ADR, the ~10-minute
# suite, and the push to get through — and a review-driven fix COMMITS, resetting this clock, so the
# real exposure is that tail, not the whole build span. 2h covers it with room and is 6x tighter
# than leg B's 12h (the same ratio leg B took against the 72h lease).
#
# The residual, stated rather than hidden: a marathon tail with no post-review commit WILL fire leg
# C on a healthy run. That finding is free, advisory, and self-clearing the moment the PR is
# recorded — and a floor loose enough never to misfire would be loose enough to stop detecting.
ABORTED_RUN_IDLE_SECS=$(( 2 * 3600 ))
```

- [ ] **Step 5: Implement leg C**

In `scripts/board-checks.sh`, insert after leg B's closing `fi` (line 425) and before the `fi` that closes the `in-progress` block (line 426):

```bash

    # Leg C — BUILT BUT NOT DELIVERED (change 0211). The run finished its build and stopped before
    # delivering it: commits on the feature branch, no PR recorded. Legs A and B are both
    # STRUCTURALLY blind to this, which is why it is a third leg and not a widening:
    #   - leg A keys on manifest/git INCOHERENCE, and here every field is coherent (plan: recorded,
    #     no results file written yet). The run dropped no bookkeeping write — it dropped two steps.
    #   - leg B keys on claimed_at, which the heartbeat rider re-stamps at every phase boundary, so
    #     a run that dies just AFTER a metadata commit starts leg B's countdown from the freshest
    #     possible stamp. Leg B is at its blindest exactly when a run has just completed a step.
    # Same check-id: this is more evidence for the same conclusion ("this run stopped mid-step"), so
    # a new id would buy a four-place BOARD_CHECK_IDS edit and a second remedy vocabulary for nothing.
    #
    # Gates are ordered CHEAPEST FIRST, and the ordering is a cost contract, not a style choice:
    # the FREE frontmatter read decides the common case (a change with a recorded PR costs ZERO git
    # calls), a non-firing path costs at most three, and the remote-ref probe runs only once the leg
    # has already decided to fire. This path is cost-sensitive (change 0176).
    #
    # A non-empty pr: short-circuits the WHOLE leg. A change whose PR is recorded has delivered;
    # "unpushed branch with a recorded PR" means the PR record and the remote disagree, which is a
    # different defect with a different remedy that leg C would be a misleading oracle for.
    if [ -z "$(fm_field "$f" pr)" ] && [ -n "$ar_ref" ]; then
      # ar_ref is REUSED from leg A but RE-GUARDED, and the guard is not optional: leg C runs
      # OUTSIDE leg A's `if ar_ref="$(branch_ref …)"`, and a failed branch_ref leaves ar_ref SET BUT
      # EMPTY. Without the -n test, `log -1 --format=%ct ""` returns empty, `NOW - ""` is NOW, and
      # the idle floor evaluates TRUE for a change with no branch at all.
      ar_tip="$("$GIT" -C "$CHANGES_DIR" log -1 --format=%ct "$ar_ref" 2>/dev/null)"
      if [ -n "$ar_tip" ] && [ "$(( NOW - ar_tip ))" -gt "$ABORTED_RUN_IDLE_SECS" ]; then
        # Ahead of BOTH bases. Feature branches are cut from origin/<integration_branch> while
        # INTEGRATION_BRANCH names the LOCAL ref, and a local integration ref routinely LAGS origin
        # (sync-integration-branch.sh is FF-only and best-effort). Comparing against the local ref
        # alone makes a freshly-cut, NOTHING-BUILT branch look arbitrarily far ahead with
        # arbitrarily old commits — it would sail through the idle floor and fire leg C on the
        # exact signature (0109: stopped with nothing built) that belongs to leg B.
        #
        # BOTH bases are show-ref-verified, symmetrically. An absent refs/heads/<integration> makes
        # rev-list exit 128 with EMPTY stdout, and since the predicate reads "empty => not ahead",
        # guarding only the remote one would silently turn the whole leg into a no-op with no
        # diagnostic. No base resolving at all is SILENCE (no positive evidence) — the same posture
        # leg B takes for an unparseable claimed_at — never "ahead of nothing".
        ar_bases=()
        for ar_b in "refs/heads/$INTEGRATION_BRANCH" "refs/remotes/origin/$INTEGRATION_BRANCH"; do
          "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "$ar_b" && ar_bases+=( "$ar_b" )
        done
        # The count gate must come FIRST and short-circuit: expanding "${ar_bases[@]}" on an empty
        # array is a set -u error under older bash.
        if [ "${#ar_bases[@]}" -gt 0 ] && \
           [ -n "$("$GIT" -C "$CHANGES_DIR" rev-list -n 1 "$ar_ref" --not "${ar_bases[@]}" 2>/dev/null)" ]; then
          # Display values only, computed ONLY on the firing path where their cost is irrelevant and
          # they are what make the finding actionable.
          ar_ahead="$("$GIT" -C "$CHANGES_DIR" rev-list --count "$ar_ref" --not "${ar_bases[@]}" 2>/dev/null)"
          ar_idle_h=$(( (NOW - ar_tip) / 3600 ))
          # One emit site, two mutually-exclusive messages. `origin` is hardcoded, inheriting
          # branch_ref's existing convention rather than inventing a second one. A STALE
          # remote-tracking ref left by a remote-side branch deletion reads as "pushed" and yields
          # the other message — acceptable for an advisory finding whose remedy in both cases is
          # "go look at this run".
          if "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$ar_branch"; then
            emit aborted-run "$id" "$ar_branch is pushed but pr: is unset (last commit ${ar_idle_h}h ago) — the run stopped between the push and the PR record; open the PR or record it"
          else
            emit aborted-run "$id" "$ar_ahead commits on $ar_branch ahead of $INTEGRATION_BRANCH, branch never pushed and pr: is unset (last commit ${ar_idle_h}h ago) — the run stopped before it opened its PR; push and open it, or re-run the step"
          fi
        fi
      fi
    fi
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -c "NOT OK"`
Expected: `0`.

Then confirm the new asserts actually ran (a typo'd fixture name yields zero matching lines, which `grep -c "NOT OK"` cannot distinguish from success — see the `agent-shell-noop-reads-as-success` learning):

Run: `bash tests/test_board_checks.sh 2>&1 | grep -c "leg C"`
Expected: `9` or more (2 preconditions + 7 leg-C asserts from Step 2).

- [ ] **Step 7: Commit**

```bash
git add scripts/board-checks.sh tests/test_board_checks.sh
git commit -m "feat(0211): add aborted-run leg C — built but not delivered"
```

---

### Task 2: The four SILENT fixtures — every gate exercised at baseline

Task 1's fixtures only prove leg C fires. These prove each gate declines, and they are the fixtures Task 3's mutations flip.

**Files:**
- Test: `tests/test_board_checks.sh` — append after Task 1's fixtures, still before the `# ---------------- aborted-run mutation tests` header

**Interfaces:**
- Consumes: `ar_branch_at`, `ar_push` (Task 1), `new_repo`, `assert`, `has_finding`, `NOW_EPOCH`, `AR_FRESH_CLAIM`, `AR_PLAN_NEW`
- Produces: no new API — fixture ids 234, 235, 236, 237, 238 and the `$AR18`-`$AR22` repo variables

- [ ] **Step 1: Write the live-run-window fixture (the floor is real)**

```bash
# --- GREEN: the LIVE-RUN WINDOW. Identical to 232 except the branch is 30m old, not 3h. This is
# the fixture that proves the idle floor is real: board-checks.sh runs on every Board pass,
# INCLUDING the passes inside the very run being built, so without a floor leg C would fire on
# every healthy build for the whole build span.
read -r AR18 _ < <(new_repo)
ar_branch_at "$AR18" feat/ar18 main $(( 1800 )) "$AR_PLAN_NEW"
cat > "$AR18/docs/changes/active/0234-live-run.md" <<EOF
---
id: 234
slug: live-run
title: Build commits 30 minutes old, run still live
status: in-progress
priority: medium
depends_on: []
branch: feat/ar18
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar18out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR18/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg C SILENT inside the live-run window — branch quiet only 30m (id 234)" \
  '! has_finding "$ar18out" aborted-run 234'
```

- [ ] **Step 2: Write the delivered-state fixture (the `pr:` gate)**

```bash
# --- GREEN: DELIVERED. Pushed, quiet 3h, ahead — every other conjunct true — but pr: is SET, so
# the free frontmatter read short-circuits the leg before a single git call.
read -r AR19 _ < <(new_repo)
ar_branch_at "$AR19" feat/ar19 main $(( 3*3600 )) "$AR_PLAN_NEW"
ar_push "$AR19" feat/ar19
cat > "$AR19/docs/changes/active/0235-delivered.md" <<EOF
---
id: 235
slug: delivered
title: Pushed and the PR is recorded
status: in-progress
priority: medium
depends_on: []
branch: feat/ar19
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr: https://github.com/o/r/pull/7
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar19out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR19/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg C SILENT when pr: is recorded — the delivered state (id 235)" \
  '! has_finding "$ar19out" aborted-run 235'
```

- [ ] **Step 3: Write the nothing-built fixture (the ahead-of-bases gate)**

```bash
# --- GREEN: NOTHING BUILT. The branch exists and is old, but carries ZERO commits of its own.
# This is the 0109 signature — a run that stopped with nothing built — which is leg B's territory,
# not leg C's. Leg C claims "built but not delivered"; with nothing built it must stay silent.
read -r AR20 _ < <(new_repo)
ar_branch_at "$AR20" feat/ar20 main $(( 3*3600 ))
cat > "$AR20/docs/changes/active/0236-nothing-built.md" <<EOF
---
id: 236
slug: nothing-built
title: Branch cut, nothing committed on it
status: in-progress
priority: medium
depends_on: []
branch: feat/ar20
plan:
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar20out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR20/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg C SILENT on a branch with zero commits ahead — the 0109 signature (id 236)" \
  '! has_finding "$ar20out" aborted-run 236'
```

- [ ] **Step 4: Write the stale-local-`main` fixture (the both-bases gate)**

This is the fixture the single-base predicate would have failed, and no existing fixture reaches this state — `new_repo`'s template leaves local `main` and `origin/main` identical, so it has to be built deliberately.

```bash
# --- GREEN: a STALE LOCAL integration ref. Advance the fixture's own bare origin (and therefore
# refs/remotes/origin/main) WITHOUT moving local main, then cut the change's branch from
# origin/main with no commits of its own. `push origin tmp:main` does both in one call — no fetch
# needed, since new_repo's template already carries both refs.
#
# The advancing commits MUST be dated relative to NOW_EPOCH: the change's branch tip IS one of
# them, so with real wall-clock dates the idle floor would be false and this fixture could never
# discriminate — it would be green for the wrong reason, and mutation I could never fire.
#
# Under the correct BOTH-bases predicate this branch is ahead of nothing: SILENT. Under a
# local-ref-only predicate it inherits every commit origin/main gained, looks arbitrarily far
# ahead with an arbitrarily old tip, and fires.
read -r AR21 _ < <(new_repo)
ar_branch_at "$AR21" tmp-advance main $(( 3*3600 )) "docs/results/2026-06-02-advance-results.md"
git -C "$AR21" push -q origin tmp-advance:main >/dev/null 2>&1
assert "leg C fixture 237 precondition: origin/main is AHEAD of local main (the stale local ref)" \
  '[ "$(git -C "$AR21" rev-parse refs/remotes/origin/main)" != "$(git -C "$AR21" rev-parse refs/heads/main)" ]'
ar_branch_at "$AR21" feat/ar21 refs/remotes/origin/main 0
cat > "$AR21/docs/changes/active/0237-stale-local-base.md" <<EOF
---
id: 237
slug: stale-local-base
title: Branch cut from origin/main while local main lags
status: in-progress
priority: medium
depends_on: []
branch: feat/ar21
plan:
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar21out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR21/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg C SILENT when the branch is ahead of a STALE LOCAL main but not of origin/main (id 237)" \
  '! has_finding "$ar21out" aborted-run 237'
```

- [ ] **Step 5: Write the legs-A-and-C-together fixture**

```bash
# --- RED x2: leg A and leg C on ONE change, proving the legs stayed independent. The branch
# carries an unrecorded PLAN (leg A) and is quiet, ahead, unpushed with pr: unset (leg C). Same
# shape as the existing "BOTH legs" fixture for A+B; both messages are self-contained, so
# docket-status printing two lines with different remedies for one change needs no caller change.
read -r AR22 _ < <(new_repo)
ar_branch_at "$AR22" feat/ar22 main $(( 3*3600 )) "$AR_PLAN_NEW"
cat > "$AR22/docs/changes/active/0238-both-a-and-c.md" <<EOF
---
id: 238
slug: both-a-and-c
title: Unrecorded plan on a quiet unpushed branch
status: in-progress
priority: medium
depends_on: []
branch: feat/ar22
plan:
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar22out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR22/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run fires on 238 (legs A and C together)" 'has_finding "$ar22out" aborted-run 238'
assert "legs A and C are INDEPENDENT: leg A's unrecorded-plan message is present on 238" \
  'grep -qF "but plan: is unset" <<<"$ar22out"'
assert "legs A and C are INDEPENDENT: leg C's never-pushed message is ALSO present on 238" \
  'grep -qF "branch never pushed and pr: is unset" <<<"$ar22out"'
# Computed OUTSIDE the assert: the assert argument is eval'd, and burying a TAB-bearing pattern in
# it means escaping out of and back into single quotes for every field separator.
ar22n="$(grep -cE "^aborted-run"$'\t'"238"$'\t' <<<"$ar22out")"
assert "238 emits exactly TWO aborted-run lines — one per leg, not one merged or three" \
  '[ "$ar22n" = 2 ]'
```

- [ ] **Step 6: Run the tests**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E "NOT OK"`
Expected: no output.

If fixture 237's `precondition` assert fails, the `push origin tmp-advance:main` did not advance the bare origin — stop and fix it; every assert under it is vacuous otherwise.

- [ ] **Step 7: Commit**

```bash
git add tests/test_board_checks.sh
git commit -m "test(0211): pin leg C's three gates and its independence from leg A"
```

---

### Task 3: Mutation tests — every predicate proven load-bearing

A silent fixture proves nothing until a mutation makes it speak. Six mutations, one per predicate, each against a fresh pristine copy via `armreseed`, each confirmed landed.

**Files:**
- Test: `tests/test_board_checks.sh` — ARM fixtures appended after fixture 226's heredoc (line 1432, before `armcopy=""` at line 1434); baseline asserts appended to the `arm0out` block (after line 1459); mutations G-L appended after mutation F's `rm -rf "$armcopy"` (~line 1596)

**Interfaces:**
- Consumes: `ar_branch_at`, `ar_push` (Task 1), `$ARM`, `armreseed`, `armrun`, `armrun_at`, `$ARMSCRIPT`, `AR_FRESH_CLAIM`, `AR_PLAN_NEW`
- Produces: ARM fixture ids 240-246; no new API

- [ ] **Step 1: Duplicate the leg-C fixtures into the shared `$ARM` repo**

Mutation asserts run `armrun` against the single shared `$ARM` repo, so every fixture a mutation references must exist there — the same duplication the 220-223 fixtures already do. Insert after fixture 226's `EOF` (line 1432):

```bash
# --- leg C fixtures in the shared mutation repo (change 0211) ---------------------------------
# Advance $ARM's OWN origin main first, WITHOUT moving local main: fixture 245 needs a stale local
# integration ref, and doing it here keeps it a property of the repo rather than of one fixture.
# Safe for every other check in this repo — leg A, merged-orphan and unknown-commit-ref all read
# the LOCAL INTEGRATION_BRANCH, which does not move.
ar_branch_at "$ARM" tmp-advance main $(( 3*3600 )) "docs/results/2026-06-02-arm-advance-results.md"
git -C "$ARM" push -q origin tmp-advance:main >/dev/null 2>&1
assert "ARM precondition: origin/main is AHEAD of local main (fixture 245's stale local ref)" \
  '[ "$(git -C "$ARM" rev-parse refs/remotes/origin/main)" != "$(git -C "$ARM" rev-parse refs/heads/main)" ]'

ar_branch_at "$ARM" feat/arm-c-unpushed main $(( 3*3600 )) "$AR_PLAN_NEW"
ar_branch_at "$ARM" feat/arm-c-pushed   main $(( 3*3600 )) "$AR_PLAN_NEW"
ar_push "$ARM" feat/arm-c-pushed
ar_branch_at "$ARM" feat/arm-c-live     main $(( 1800 ))   "$AR_PLAN_NEW"
ar_branch_at "$ARM" feat/arm-c-empty    main $(( 3*3600 ))
ar_branch_at "$ARM" feat/arm-c-frombase refs/remotes/origin/main 0
ar_branch_at "$ARM" feat/arm-c-prose    main $(( 3*3600 )) "$AR_PLAN_NEW"

# 240: unpushed, quiet 3h, ahead, pr: unset -> leg C fires, "never pushed"
cat > "$ARM/docs/changes/active/0240-mc-unpushed.md" <<EOF
---
id: 240
slug: mc-unpushed
title: Built on an unpushed branch
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-unpushed
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 241: identical but only 30m quiet -> SILENT. Mutation G's fixture (the idle floor).
cat > "$ARM/docs/changes/active/0241-mc-live.md" <<EOF
---
id: 241
slug: mc-live
title: Build commits 30 minutes old
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-live
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 242: PUSHED, pr: unset -> leg C fires, push/PR seam. Mutation J's fixture (the message branch).
cat > "$ARM/docs/changes/active/0242-mc-pushed.md" <<EOF
---
id: 242
slug: mc-pushed
title: Pushed with no PR recorded
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-pushed
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 243: pushed AND pr: set -> SILENT. Mutation I's fixture (the pr: gate).
cat > "$ARM/docs/changes/active/0243-mc-delivered.md" <<EOF
---
id: 243
slug: mc-delivered
title: Pushed and delivered
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-pushed
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr: https://github.com/o/r/pull/9
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 244: branch with ZERO commits ahead -> SILENT. Mutation H's fixture (the ahead-of-bases test).
cat > "$ARM/docs/changes/active/0244-mc-empty.md" <<EOF
---
id: 244
slug: mc-empty
title: Branch cut, nothing built
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-empty
plan:
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 245: cut from origin/main while LOCAL main lags -> SILENT. Mutation K's fixture (both bases).
cat > "$ARM/docs/changes/active/0245-mc-frombase.md" <<EOF
---
id: 245
slug: mc-frombase
title: Cut from origin/main, local main stale
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-frombase
plan:
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 246: pr: absent from FRONTMATTER, present in BODY prose -> leg C fires under the ANCHORED read
# and goes silent under an unanchored one. Mutation L's fixture (ADR-0057), the exact shape 223/224
# pin for plan: and results:.
cat > "$ARM/docs/changes/active/0246-mc-prose.md" <<EOF
---
id: 246
slug: mc-prose
title: Body prose mentions pr
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-prose
plan: docs/superpowers/plans/2026-06-01-present.md
results:
claimed_at: $AR_FRESH_CLAIM
---

## Notes
pr: https://github.com/o/r/pull/11
EOF
```

- [ ] **Step 2: Extend the baseline asserts, and pin the EXISTING fixtures' leg-C silence**

Append to the `arm0out` baseline block, after line 1459:

```bash
assert "mutation baseline: leg C fires on 240 (unpushed, quiet, ahead)" 'has_finding "$arm0out" aborted-run 240'
assert "mutation baseline: leg C fires on 242 (pushed, no PR)" 'has_finding "$arm0out" aborted-run 242'
assert "mutation baseline: leg C fires on 246 (pr: only in body prose)" 'has_finding "$arm0out" aborted-run 246'
assert "mutation baseline: leg C SILENT on 241 (live-run window)"   '! has_finding "$arm0out" aborted-run 241'
assert "mutation baseline: leg C SILENT on 243 (pr: recorded)"      '! has_finding "$arm0out" aborted-run 243'
assert "mutation baseline: leg C SILENT on 244 (nothing built)"     '! has_finding "$arm0out" aborted-run 244'
assert "mutation baseline: leg C SILENT on 245 (stale local main)"  '! has_finding "$arm0out" aborted-run 245'

# The EXISTING leg-A/B fixtures (220, 221, 223, 225) all have a branch that is ahead of main with
# pr: absent — three of leg C's four conjuncts. They stay leg-C-silent ONLY because ar_branch dates
# its commits with the real wall clock (2026-08) while NOW_EPOCH is 1750000000 (2025-06), making
# `NOW - ts` NEGATIVE and the idle floor false. That is an ACCIDENT of the harness, not an intent,
# and it is exactly what leg C's arrival makes load-bearing: the single-finding asserts above are
# otherwise guarded by nothing but the sign of that delta. Pin the intent explicitly, so that
# re-dating those fixtures later reddens HERE with a message that says why, instead of silently
# changing what mutations A-F measure.
# Computed outside the asserts, for the same reason as 238's count above.
arm0_legc="$(grep -cF "branch never pushed and pr: is unset" <<<"$arm0out")"
arm0_220="$(grep -E "^aborted-run"$'\t'"220"$'\t' <<<"$arm0out")"
arm0_221="$(grep -E "^aborted-run"$'\t'"221"$'\t' <<<"$arm0out")"
arm0_223="$(grep -E "^aborted-run"$'\t'"223"$'\t' <<<"$arm0out")"
assert "leg C does not reach the existing leg-A fixtures: no leg-C message on 220" \
  '! grep -qF "branch never pushed and pr: is unset" <<<"$arm0_220"'
assert "leg C does not reach the existing leg-A fixtures: no leg-C message on 221" \
  '! grep -qF "branch never pushed and pr: is unset" <<<"$arm0_221"'
assert "leg C does not reach the existing leg-A fixtures: no leg-C message on 223" \
  '! grep -qF "branch never pushed and pr: is unset" <<<"$arm0_223"'
# Non-vacuity companion for the three absence asserts above (they would all pass if leg C's
# never-pushed message never appeared at all): the SAME string must be present somewhere in this
# very output, on the fixtures that are supposed to have it.
assert "the leg-C never-pushed message IS present in this run — the three absence asserts are not vacuous" \
  '[ "$arm0_legc" -ge 1 ]'
assert "leg C SILENT on 225 (healthy fields) — the fixture stays a pure leg-A mutation target" \
  '! has_finding "$arm0out" aborted-run 225'
```

- [ ] **Step 3: Re-check every count-based and single-finding assert in the ARM repo**

Mutation G (Step 4) removes the idle floor, which makes **every** skewed ARM branch fixture a leg-C candidate at once — its blast radius is wider than the one fixture it names.

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E "NOT OK"`
Expected: no output. Then read every assert between lines 1445 and the end of mutation F for one that counts findings or says "exactly ONCE"; each one now shares the `aborted-run` id with a third leg. Where such an assert exists and its fixture's branch is ahead with `pr:` absent, add the leg-C-specific message exclusion shown in Step 2 rather than leaving the outcome to the date delta. Record in the commit message which asserts you re-checked.

- [ ] **Step 4: Mutation G — drop the idle floor**

Append after mutation F's `rm -rf "$armcopy"`:

```bash
# ---------------- leg C mutations (change 0211) ----------------
# Mutation G — neutralize leg C's idle floor (the > comparison becomes a tautology): the live-run
# fixture 241 starts firing. NOTE the blast radius: without the floor, every ARM branch fixture
# whose branch is ahead with pr: absent becomes a leg-C candidate, which is why the baseline block
# above pins those fixtures explicitly rather than trusting the date delta.
armreseed
armG_before="$(grep -cF '"$(( NOW - ar_tip ))" -gt "$ABORTED_RUN_IDLE_SECS"' "$ARMSCRIPT")"
sed 's/"$(( NOW - ar_tip ))" -gt "$ABORTED_RUN_IDLE_SECS"/-n "$ar_tip"/' "$ARMSCRIPT" > "$ARMSCRIPT.t"
mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armG_after="$(grep -cF '"$(( NOW - ar_tip ))" -gt "$ABORTED_RUN_IDLE_SECS"' "$ARMSCRIPT")"
assert "mutation G landed: the idle-floor comparison is gone (count 1 -> 0)" \
  '[ "$armG_before" = 1 ] && [ "$armG_after" = 0 ]'
assert "mutation G landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armGout="$(armrun)"
assert "mutation G (drop the idle floor): the live-run fixture 241 starts firing — the floor is real" \
  'has_finding "$armGout" aborted-run 241'
assert "mutation G: the quiet fixture 240 still fires (the leg itself survives)" \
  'has_finding "$armGout" aborted-run 240'
rm -rf "$armcopy"
```

- [ ] **Step 5: Mutation H — drop the ahead-of-bases test**

```bash
# Mutation H — make the ahead-of-bases probe unconditionally true: the nothing-built fixture 244
# starts firing, i.e. leg C would claim "built but not delivered" about a branch with no build.
armreseed
armH_before="$(grep -cF 'rev-list -n 1 "$ar_ref" --not' "$ARMSCRIPT")"
sed 's|\[ -n "$("$GIT" -C "$CHANGES_DIR" rev-list -n 1 "$ar_ref" --not "${ar_bases\[@\]}" 2>/dev/null)" \]|true|' \
  "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armH_after="$(grep -cF 'rev-list -n 1 "$ar_ref" --not' "$ARMSCRIPT")"
assert "mutation H landed: the ahead-of-bases probe is gone (count 1 -> 0)" \
  '[ "$armH_before" = 1 ] && [ "$armH_after" = 0 ]'
assert "mutation H landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armHout="$(armrun)"
assert "mutation H (drop the ahead test): the nothing-built fixture 244 starts firing" \
  'has_finding "$armHout" aborted-run 244'
assert "mutation H: the genuinely-built fixture 240 still fires" 'has_finding "$armHout" aborted-run 240'
rm -rf "$armcopy"
```

- [ ] **Step 6: Mutation I — drop the `pr:`-empty gate**

```bash
# Mutation I — drop leg C's pr:-empty gate: the delivered fixture 243 starts firing.
armreseed
armI_before="$(grep -cF 'if [ -z "$(fm_field "$f" pr)" ] && [ -n "$ar_ref" ]; then' "$ARMSCRIPT")"
sed 's|if \[ -z "$(fm_field "$f" pr)" \] && \[ -n "$ar_ref" \]; then|if [ -n "$ar_ref" ]; then|' \
  "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armI_after="$(grep -cF 'if [ -z "$(fm_field "$f" pr)" ] && [ -n "$ar_ref" ]; then' "$ARMSCRIPT")"
assert "mutation I landed: leg C's pr:-empty gate is gone (count 1 -> 0)" \
  '[ "$armI_before" = 1 ] && [ "$armI_after" = 0 ]'
assert "mutation I landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armIout="$(armrun)"
assert "mutation I (drop the pr: gate): the delivered fixture 243 starts firing" \
  'has_finding "$armIout" aborted-run 243'
rm -rf "$armcopy"
```

- [ ] **Step 7: Mutation J — swap the remote-ref probe's sense**

```bash
# Mutation J — invert the message-selecting remote-ref probe: the two firing fixtures SWAP
# messages. This is the mutation that proves the branch is a real discriminator and not a coin
# flip that happens to be right for one of them.
armreseed
armJ_before="$(grep -cF 'show-ref --verify --quiet "refs/remotes/origin/$ar_branch"' "$ARMSCRIPT")"
sed 's|if "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$ar_branch"; then|if ! "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$ar_branch"; then|' \
  "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armJ_after="$(grep -cF 'if ! "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$ar_branch"; then' "$ARMSCRIPT")"
assert "mutation J landed: the remote-ref probe is negated (count 0 -> 1)" \
  '[ "$armJ_before" = 1 ] && [ "$armJ_after" = 1 ]'
assert "mutation J landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armJout="$(armrun)"
armJ240="$(grep -E "^aborted-run"$'\t'"240"$'\t' <<<"$armJout")"
armJ242="$(grep -E "^aborted-run"$'\t'"242"$'\t' <<<"$armJout")"
assert "mutation J (swap the probe): the UNPUSHED fixture 240 now gets the pushed message" \
  'grep -qF "is pushed but pr: is unset" <<<"$armJ240"'
assert "mutation J (swap the probe): the PUSHED fixture 242 now gets the never-pushed message" \
  'grep -qF "branch never pushed and pr: is unset" <<<"$armJ242"'
rm -rf "$armcopy"
```

- [ ] **Step 8: Mutation K — compare against the local base only**

```bash
# Mutation K — drop the remote-tracking base from ar_bases (the single-base predicate an earlier
# draft used): fixture 245, cut from origin/main while local main lags, starts firing. This is the
# false positive the both-bases design exists to prevent — and note the idle floor does NOT catch
# it, because the inherited commits are genuinely old.
armreseed
armK_before="$(grep -cF 'for ar_b in "refs/heads/$INTEGRATION_BRANCH" "refs/remotes/origin/$INTEGRATION_BRANCH"; do' "$ARMSCRIPT")"
sed 's|for ar_b in "refs/heads/$INTEGRATION_BRANCH" "refs/remotes/origin/$INTEGRATION_BRANCH"; do|for ar_b in "refs/heads/$INTEGRATION_BRANCH"; do|' \
  "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armK_after="$(grep -cF 'for ar_b in "refs/heads/$INTEGRATION_BRANCH" "refs/remotes/origin/$INTEGRATION_BRANCH"; do' "$ARMSCRIPT")"
assert "mutation K landed: the remote-tracking base is gone from ar_bases (count 1 -> 0)" \
  '[ "$armK_before" = 1 ] && [ "$armK_after" = 0 ]'
assert "mutation K landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armKout="$(armrun)"
assert "mutation K (local base only): the stale-local-main fixture 245 starts firing" \
  'has_finding "$armKout" aborted-run 245'
rm -rf "$armcopy"
```

- [ ] **Step 9: Mutation L — unanchor leg C's `pr:` read**

```bash
# Mutation L — unanchor leg C's pr: read (fm_field -> field): the body-prose fixture 246 goes
# GREEN, because the unanchored read falls through the closing --- and returns the prose line as
# if it were a recorded PR. ADR-0057, the same property 223 and 224 pin for plan: and results:.
# A FALSE NEGATIVE is the dangerous direction here: it makes the check certify the exact abort it
# exists to catch.
armreseed
armL_before="$(grep -cF 'fm_field "$f" pr' "$ARMSCRIPT")"
sed 's|\[ -z "$(fm_field "$f" pr)" \]|[ -z "$(field "$f" pr)" ]|' "$ARMSCRIPT" > "$ARMSCRIPT.t"
mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armL_after="$(grep -cF 'fm_field "$f" pr' "$ARMSCRIPT")"
assert "mutation L landed: leg C's pr: read is unanchored (count 1 -> 0)" \
  '[ "$armL_before" = 1 ] && [ "$armL_after" = 0 ]'
assert "mutation L landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armLout="$(armrun)"
assert "mutation L (unanchor the pr: read): the body-prose fixture 246 goes GREEN — proves the anchoring" \
  '! has_finding "$armLout" aborted-run 246'
assert "mutation L: fixture 240, which has no body pr: line, still fires" \
  'has_finding "$armLout" aborted-run 240'
rm -rf "$armcopy"
```

- [ ] **Step 10: Run the full test file**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E "NOT OK"`
Expected: no output.

Then prove every mutation actually landed rather than silently no-op'ing on a `sed` pattern that no longer matches:

Run: `bash tests/test_board_checks.sh 2>&1 | grep -cE "^ok - mutation [GHIJKL] landed"`
Expected: `12` (two landed-asserts per mutation).

- [ ] **Step 11: Commit**

```bash
git add tests/test_board_checks.sh
git commit -m "test(0211): mutation-test each leg C predicate and pin the existing fixtures"
```

---

### Task 4: Document leg C in the script contract

**Files:**
- Modify: `scripts/board-checks.md` — the `aborted-run` section (lines 198-225)

**Interfaces:**
- Consumes: the constant name `ABORTED_RUN_IDLE_SECS` and the two message shapes from Task 1
- Produces: nothing consumed by other tasks

- [ ] **Step 1: Update the section's opening**

Change the sentence at line 202-203 from "Gated on `status: in-progress`. Two independent legs; either emits, and both may emit on one change." to:

```markdown
Gated on `status: in-progress`. **Three** independent legs; any emits, and more than one may emit
on one change.
```

- [ ] **Step 2: Add the leg C bullet**

Insert after the leg B bullet (line 215), before the "**A separate check-id…**" paragraph:

```markdown
- **Leg C — built but not delivered (time-based, change 0211).** The branch named in `branch:`
  carries commits reachable from **neither** `refs/heads/<integration-branch>` **nor**
  `refs/remotes/origin/<integration-branch>`, its newest commit is older than **2 hours**, and
  `pr:` is empty. Catches the run that finished its build and stopped before delivering it —
  invisible to leg A (every field is coherent: `plan:` recorded, no results file yet) and to leg B
  (the `claimed_at` heartbeat was re-stamped at that very metadata write, so leg B's countdown
  starts from the freshest possible stamp). One leg, two messages, chosen by whether
  `refs/remotes/origin/<branch>` resolves: *branch never pushed* (the run stopped before its push)
  or *pushed but `pr:` unset* (the run stopped between the push and the PR record).

  Three design points worth stating, because each is a predicate someone will later be tempted to
  simplify:

  - **Both integration bases are excluded, each `show-ref`-verified.** Feature branches are cut
    from `origin/<integration-branch>` while a local integration ref routinely lags it, so a
    local-only comparison makes a freshly-cut, nothing-built branch look arbitrarily far ahead with
    arbitrarily old commits — firing leg C on a signature that belongs to leg B. No base resolving
    is **silence**, never "ahead of nothing".
  - **The idle floor is keyed on the branch's newest commit, never on `claimed_at`** — the
    heartbeat rider makes `claimed_at` unusable here, which is precisely why leg B is blind.
  - **A non-empty `pr:` short-circuits the whole leg** before any git call. That keeps the common
    case free, and "unpushed branch with a recorded PR" is a different defect with a different
    remedy that leg C would be a misleading oracle for.

  **Cost:** at most three `git` invocations on a non-firing path and five on the firing path, and
  only for `in-progress` changes with an empty `pr:` — the population legs A and B already walk.
  A change with a recorded PR adds **zero**.

  **Known residual:** a marathon post-build tail with no further commit fires leg C on a healthy
  run. The finding is advisory and self-clearing once the PR is recorded; a floor loose enough
  never to misfire would be loose enough to stop detecting.

  **Not covered:** the run that opens the PR, writes `pr:`, and dies before `status: implemented`.
  Leg C's `pr:`-empty gate makes it invisible and leg B catches it at 12h; its evidence is a
  manifest/GitHub comparison, and this script is git-only by contract.
```

- [ ] **Step 3: Update the hardcoded-horizons paragraph**

Change lines 224-225 from "The 12h window is **hardcoded**, matching…" to:

```markdown
The 12h leg-B window and the 2h leg-C branch-idle floor (`ABORTED_RUN_STALE_SECS` and
`ABORTED_RUN_IDLE_SECS`) are both **hardcoded**, matching `stale-finalize-blocked`'s 72h and
`stale-in-progress`'s 3-day branch-idle horizon; only the lease TTL is a knob.
```

- [ ] **Step 4: Verify the doc matches the code**

Run: `grep -c "ABORTED_RUN_IDLE_SECS" scripts/board-checks.sh scripts/board-checks.md`
Expected: `scripts/board-checks.sh:2` and `scripts/board-checks.md:1`.

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E "NOT OK"`
Expected: no output (this file has sentinel asserts over `SKILL.md`; confirm none of them cover the changed doc text).

- [ ] **Step 5: Commit**

```bash
git add scripts/board-checks.md
git commit -m "docs(0211): document aborted-run leg C and its idle floor"
```

---

## Final verification

- [ ] Run the full suite (foreground, ONE call — never backgrounded). This repo has no aggregate runner script; the detected Bash-suite shape is the loop `docket-finalize-change` documents:

```bash
for test in tests/test_*.sh; do "${DOCKET_BASH_PATH:-bash}" "$test"; done
```

Expected: green (no `NOT OK` lines).
- [ ] Run `bash tests/test_board_checks.sh 2>&1 | grep -c "^ok"` and confirm the count rose by the number of asserts this plan adds — a suite that reports green while silently running zero new asserts is the failure mode this step exists to catch.
