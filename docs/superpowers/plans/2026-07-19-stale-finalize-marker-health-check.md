# Stale `## Finalize blocked` marker health check — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a git-only, warn-only, time-based health check `stale-finalize-blocked` to `scripts/board-checks.sh` that flags an `implemented` change whose `## Finalize blocked` marker has outlived a hardcoded 72 h horizon.

**Architecture:** The check rides the existing per-file `FILES` walk in `board-checks.sh`, gated on `status == implemented` AND the `finalize_blocked` helper. Marker age is the change file's last-commit timestamp (`git log -1 --format=%ct -- <file>`) — tamper-proof, unlike the model-authored in-body date. It `emit`s through the same accumulator every other finding uses; no new plumbing, no config knob, no `gh`/network. The horizon is a hardcoded named constant mirroring `stale-in-progress`'s own hardcoded `3*86400` branch-idle horizon.

**Tech Stack:** POSIX-ish bash (`set -uo pipefail`), the sourced `scripts/lib/docket-frontmatter.sh` helpers (`field`, `finalize_blocked`), git plumbing, the existing hermetic bash test harness `tests/test_board_checks.sh` (temp repo + bare origin, `GIT`/`NOW` mock seams).

## Global Constraints

- **Git-only, offline.** No `gh`, no network — `board-checks.sh`'s stated core invariant (script header lines 4–6). Marker age comes only from `git log`.
- **Warn-only.** The check `emit`s a finding and never mutates the change file, never auto-clears the marker. Mutation stays with `docket-finalize-change`.
- **No new config knob.** The 72 h horizon is a hardcoded named constant `FINALIZE_BLOCKED_STALE_SECS=$(( 72 * 3600 ))` (spec A4). Do NOT reuse `--lease-ttl-hours` (that flag's meaning stays "claim-lease TTL").
- **Determinism.** Findings stay sorted `(check-id asc, change-id numeric asc)` — the existing final `sort` handles the new check-id for free.
- **Marker age = git commit timestamp, never the in-body date** (spec A2; learning `model-authored-values-are-untrusted-input`). The `## Finalize blocked` heading is deliberately bare; its body date is model-authored prose.
- **Fire on any marker past the horizon** (spec A3) — a git-only check cannot know whether the underlying cause still holds, and a still-blocked marker past 72 h is itself worth a human glance.
- Run the **whole** test suite at the build gate, not only `test_board_checks.sh` (AGENTS.md).

---

### Task 1: The `stale-finalize-blocked` check + tests + contract doc

**Files:**
- Modify: `scripts/board-checks.sh` (add the named constant near the top; add the check block inside the per-file `FILES` walk; add `stale-finalize-blocked` to the header's documented `check-id ∈ {…}` set)
- Modify: `tests/test_board_checks.sh` (new `stale-finalize-blocked` section)
- Modify: `scripts/board-checks.md` (document the new check under "Check enumeration")

**Interfaces:**
- Consumes: `field FILE status`, `finalize_blocked FILE` (whole-line `## Finalize blocked` presence, `implemented`-only by contract), the `GIT`/`NOW` mock seams, `id` (already resolved at the top of the loop) — all already present in the script.
- Produces: a TAB-separated finding line `stale-finalize-blocked\t<id>\t<message>` on stdout, where `<message>` names the marker age in hours and the remedy. No new function, no new flag, no exported symbol.

- [ ] **Step 1: Write the failing tests**

Append a new section to `tests/test_board_checks.sh`, immediately before the final `# ==== docket-status wiring sentinels ====` section (i.e. after the `merged-orphan / unknown-commit-ref` section). It reuses the file's existing helpers (`new_repo`, `has_finding`, `git_quiet`, `NOW_EPOCH`, `assert`). Marker age is driven by committing the fixture files with `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE` seams, exactly as the `stale-in-progress` section ages its branch commits.

```bash
# ============================ stale-finalize-blocked ============================
# An 'implemented' change carrying `## Finalize blocked` whose change-file's last commit is older
# than the hardcoded 72h horizon ⇒ one stale-finalize-blocked finding. A recent last commit ⇒
# silent. No marker ⇒ silent. A non-implemented status carrying a stray marker ⇒ silent (the
# status==implemented gate). Marker age is the change file's git commit timestamp (the
# GIT_AUTHOR/COMMITTER_DATE seams below), never a model-authored in-body date. Hermetic: NOW pinned.
read -r FB _ < <(new_repo)
FB_STALE_EPOCH=$(( NOW_EPOCH - 100*3600 ))   # 100h old  > 72h horizon => stale
FB_FRESH_EPOCH=$(( NOW_EPOCH -   1*3600 ))   #   1h old  < 72h horizon => fresh
# id 40: implemented + marker, file committed 100h ago ⇒ fires.
cat > "$FB/docs/changes/active/0040-staleblocked.md" <<'EOF'
---
id: 40
slug: staleblocked
title: Implemented, finalize-blocked, stale marker
status: implemented
priority: medium
depends_on: []
pr: https://github.com/o/r/pull/40
---

## Finalize blocked

### 2026-01-01 — gate failure
Rebase onto main hit a conflict; a human must resolve.
EOF
# id 41: implemented + marker, file committed 1h ago ⇒ silent.
cat > "$FB/docs/changes/active/0041-freshblocked.md" <<'EOF'
---
id: 41
slug: freshblocked
title: Implemented, finalize-blocked, fresh marker
status: implemented
priority: medium
depends_on: []
pr: https://github.com/o/r/pull/41
---

## Finalize blocked

### 2026-07-19 — gate failure
Rebase onto main hit a conflict; just marked.
EOF
# id 42: implemented, NO marker ⇒ silent (even though committed 100h ago).
cat > "$FB/docs/changes/active/0042-nomarker.md" <<'EOF'
---
id: 42
slug: nomarker
title: Implemented, no finalize-blocked marker
status: implemented
priority: medium
depends_on: []
pr: https://github.com/o/r/pull/42
---

## Why
Nothing blocked here.
EOF
# id 43: in-progress carrying a STRAY marker, file committed 100h ago ⇒ silent (status gate).
cat > "$FB/docs/changes/active/0043-wrongstatus.md" <<'EOF'
---
id: 43
slug: wrongstatus
title: In-progress with a stray finalize-blocked marker
status: in-progress
priority: medium
depends_on: []
branch: feat/wrongstatus
---

## Finalize blocked

### 2026-01-01 — stray
Should not fire — status is not implemented.
EOF
# Commit the stale-dated fixtures (40/42/43) at 100h-old, then the fresh fixture (41) at 1h-old.
# `git log -1 --format=%ct -- <file>` resolves each file's own last-touching commit, so the two
# commits' dates attach per-file regardless of global commit ordering.
git -C "$FB" add docs/changes/active/0040-staleblocked.md \
                 docs/changes/active/0042-nomarker.md \
                 docs/changes/active/0043-wrongstatus.md
GIT_AUTHOR_DATE="@$FB_STALE_EPOCH +0000" GIT_COMMITTER_DATE="@$FB_STALE_EPOCH +0000" \
  git_quiet -C "$FB" commit -m "fb: stale fixtures"
git -C "$FB" add docs/changes/active/0041-freshblocked.md
GIT_AUTHOR_DATE="@$FB_FRESH_EPOCH +0000" GIT_COMMITTER_DATE="@$FB_FRESH_EPOCH +0000" \
  git_quiet -C "$FB" commit -m "fb: fresh fixture"
fbout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$FB/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "stale-finalize-blocked fires for an implemented change with a stale marker (id 40)" \
  'has_finding "$fbout" stale-finalize-blocked 40'
assert "stale-finalize-blocked message reports the marker age in hours (100h)" \
  'printf "%s" "$fbout" | grep -E "$(printf "^stale-finalize-blocked\t40\t")" | grep -qF "100h"'
assert "stale-finalize-blocked message names re-run finalize with the id (id 40)" \
  'printf "%s" "$fbout" | grep -E "$(printf "^stale-finalize-blocked\t40\t")" | grep -qF "finalize 40"'
assert "stale-finalize-blocked silent for a recent marker (id 41)" \
  '! has_finding "$fbout" stale-finalize-blocked 41'
assert "stale-finalize-blocked silent for an implemented change without the marker (id 42)" \
  '! has_finding "$fbout" stale-finalize-blocked 42'
assert "stale-finalize-blocked silent for a non-implemented change carrying a stray marker (id 43, status gate)" \
  '! has_finding "$fbout" stale-finalize-blocked 43'
```

- [ ] **Step 2: Run the tests to verify the "fires" assertions FAIL (check not yet implemented)**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E 'stale-finalize-blocked|FAIL|NOT OK'`
Expected: the `id 40` "fires" / "100h" / "finalize 40" asserts print `NOT OK` (no finding emitted yet); the silent asserts print `ok` (vacuously — nothing fires). Overall run ends `FAIL`. This confirms the tests actually exercise the new check rather than passing vacuously.

- [ ] **Step 3: Add the horizon constant to `scripts/board-checks.sh`**

Insert the named constant just above the `mapfile -t FILES` line (right after the `emit(){ … }` definition, before the `# Walk every change file` comment):

```bash
# Staleness horizon for the stale-finalize-blocked check (change 0098): an 'implemented' change's
# `## Finalize blocked` marker older than this fires the advisory. Hardcoded, no config knob —
# mirrors stale-in-progress's own hardcoded 3*86400 branch-idle horizon; 72h matches the lease-TTL
# default's sense of "a few days is normal, longer is suspicious". Promote to a flag only if
# independent tuning is ever wanted.
FINALIZE_BLOCKED_STALE_SECS=$(( 72 * 3600 ))
```

- [ ] **Step 4: Add the check block inside the per-file `FILES` walk**

In `scripts/board-checks.sh`, immediately after the `merge-gate-stall` block and before the loop's closing `done`, add:

```bash
  # --- stale-finalize-blocked: an 'implemented' change carrying the `## Finalize blocked` marker
  # whose marker has outlived FINALIZE_BLOCKED_STALE_SECS (change 0098). The marker's only clearing
  # path is a docket-finalize-change run; when a human resolves the underlying cause out of band
  # (without re-running finalize with the id named) the marker sits on the board indefinitely. This
  # is a git-only, time-based advisory: it cannot know whether the cause still holds (that needs a
  # network probe this script forbids), so it fires on ANY marker past the horizon — a marker still
  # genuinely blocked that long is itself worth a human glance. Marker age = the change file's
  # last-commit timestamp (git ct is tamper-proof; the in-body date is model-authored prose). Never
  # mutates the file / auto-clears the marker — that stays docket-finalize-change's job.
  if [ "$status" = "implemented" ] && finalize_blocked "$f"; then
    fbts="$("$GIT" -C "$CHANGES_DIR" log -1 --format=%ct -- "$f" 2>/dev/null)"
    if [ -n "$fbts" ] && [ "$(( NOW - fbts ))" -gt "$FINALIZE_BLOCKED_STALE_SECS" ]; then
      emit stale-finalize-blocked "$id" "## Finalize blocked marker set $(( (NOW - fbts) / 3600 ))h ago — resolve the cause and re-run finalize $id, or it will sit on the board"
    fi
  fi
```

- [ ] **Step 5: Add `stale-finalize-blocked` to the header's documented check-id set**

In `scripts/board-checks.sh`, update the header comment's `check-id ∈ {…}` line so the new id is listed (keep the two-line wrap tidy):

```bash
#     check-id ∈ {broken-spec, broken-plan-results, dep-cycle, stale-in-progress, merge-gate-stall,
#                 stale-finalize-blocked, merged-orphan, unknown-commit-ref}
```

- [ ] **Step 6: Run the targeted test to verify it now PASSES**

Run: `bash tests/test_board_checks.sh 2>&1 | tail -5`
Expected: the run ends `PASS` (exit 0). All `stale-finalize-blocked` asserts print `ok`.

- [ ] **Step 7: Document the new check in `scripts/board-checks.md`**

Under "### Check enumeration", add a `**stale-finalize-blocked**` entry after the `merge-gate-stall` paragraph:

```markdown
**`stale-finalize-blocked`** — The change has `status: implemented` and carries the
`## Finalize blocked` body section (`finalize_blocked`), and that marker has outlived a fixed
staleness horizon (`FINALIZE_BLOCKED_STALE_SECS`, hardcoded 72 h). Marker age is the change file's
last-commit timestamp (`git log -1 --format=%ct -- <file>`) — the marker heading is deliberately
undated and its in-body date is model-authored, so git's clock is the tamper-proof signal. The
finding names the age in hours and advises re-running finalize with the id. Git-only and warn-only:
it cannot probe whether the underlying cause still holds (that needs `gh`/network, forbidden here),
so it fires on **any** marker past the horizon — a still-blocked marker that old is itself worth a
human glance. It never mutates the change file or auto-clears the marker; that stays
`docket-finalize-change`'s job. The horizon is a hardcoded constant (mirroring `stale-in-progress`'s
own `3*86400` branch-idle horizon), not a config knob.
```

- [ ] **Step 8: Run the whole suite (build gate)**

Run: `for f in tests/test_*.sh; do bash "$f" >/tmp/dsuite.$$ 2>&1 || { echo "FAILED: $f"; tail -20 /tmp/dsuite.$$; }; done; echo "suite done"`
Expected: no `FAILED:` lines; `suite done` prints. (AGENTS.md: run the whole suite, not only the enumerated test.)

- [ ] **Step 9: Commit**

```bash
git add scripts/board-checks.sh scripts/board-checks.md tests/test_board_checks.sh
git commit -m "feat(0098): stale-finalize-blocked health check in board-checks.sh"
```
