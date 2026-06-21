# docket-status sweep: delegate archiving to archive-change.sh — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite `docket-status`'s merge-sweep per-change archive sub-procedure so it delegates the archive entirely to `scripts/archive-change.sh` (byte-aligned with `docket-finalize-change` step 3), removing today's hand-rolled double-archive — a documentation-only change to `skills/docket-status/SKILL.md`, guarded by skill-body sentinel tests.

**Architecture:** This is a skill-body (Markdown procedure) edit plus matching sentinel tests. No runtime/script code changes — `archive-change.sh`, `terminal-publish.sh`, and `render-change-links.sh` are unchanged; their behavior is reused, not modified. The sweep's steps a/b (pre-gate) and g/h (cleanup/harvest) and the once-per-run integration sync are untouched; only steps c–f collapse into a three-step delegated flow: (1) `archive-change.sh`, (2) `render-change-links.sh` follow-on commit pushed to `origin/docket` before publish, (3) `terminal-publish.sh`. The sweep keeps its own **log-and-continue** failure posture (deliberately divergent from finalize step 3's abort-and-report).

**Tech Stack:** Bash (the docket scripts + the test harness in `tests/*.sh`, which use a `grep -Eqi`/`grep -q` sentinel pattern over the SKILL.md prose). No external deps.

## Global Constraints

- **No script changes.** Only `skills/docket-status/SKILL.md` (prose) and `tests/test_closeout.sh` (sentinels) are edited. `archive-change.sh`, `terminal-publish.sh`, `render-change-links.sh`, `cleanup-feature-branch.sh`, and the convention are not touched.
- **Renderer ordering is correctness-critical (LEARNINGS #0035).** The `render-change-links.sh` re-render of the `## Artifacts` block must be a **follow-on commit pushed to `origin/docket` BEFORE** `terminal-publish.sh` runs — terminal-publish copies the archived change file *from `origin/docket`*, so a stale block would be published. A failed re-render must **skip** publish.
- **The sweep's failure posture is log-and-continue, NOT abort-and-report** (spec A6). On a non-zero exit from any of the three delegated steps: log it, abandon the remainder of THIS change's close-out, continue to the NEXT change. This is deliberately divergent from finalize step 3 (whose posture is abort-and-report). "Abandon the remainder" carries the #0035 guard: a failed renderer commit skips publish.
- **The archive commit stays change-file-only and byte-identical across concurrent archivers** (the determinism invariant) — which is why the renderer must be a SEPARATE follow-on commit, never bundled into the script-owned archive commit.
- **The two "must not diverge" notes** (one each in `skills/docket-status/SKILL.md` and `skills/docket-finalize-change/SKILL.md` — NOT the convention) must remain present and accurate after the edit.
- **Presentation decision (spec A9, resolved in reconcile):** byte-align the sweep's prose to finalize step 3's sequence while keeping the sweep's own explicit log-and-continue posture in its own words — do NOT replace the sweep prose with a bare "see finalize step 3" reference. Rationale: the sweep's failure handling legitimately diverges and its surrounding best-effort-safety-net framing is self-contained.
- **Commit-message trailer:** every commit ends with `Claude-Session: https://claude.ai/code/session_017yfnCscGdanqv4LBbUGwEo`.

---

## File Structure

- `skills/docket-status/SKILL.md` — the `## Merge sweep` section, step `3. Merged → ARCHIVE IDEMPOTENTLY:`. Replace sub-steps **c, d, e, f** with the delegated three-step flow + a per-change failure-posture paragraph. Sub-steps **a, b, g, h**, the *Sync the integration checkout* paragraph, the *Determinism invariant* paragraph, and the *Note (must not diverge)* paragraph stay (the Note's wording is verified accurate against the new flow).
- `tests/test_closeout.sh` — the call-site wiring sentinels block (near the end). Add sweep-specific assertions for: delegation present, manual `git mv active/` gone, renderer ordered after archive and before terminal-publish, and the log-and-continue failure posture distinct from abort-and-report. The existing three `wiring(status): sweep invokes …` assertions stay (still true).

The two files change together (prose + its guard) and are committed together per task.

---

## Task 1: Rewrite the sweep's archive sub-procedure (steps c–f) to delegate to archive-change.sh

**Files:**
- Modify: `skills/docket-status/SKILL.md` — `## Merge sweep` section, step 3 sub-steps c–f
- Test: `tests/test_closeout.sh` — call-site wiring sentinels block (append sweep assertions)

**Interfaces:**
- Consumes: the existing `archive-change.sh` CLI (`--changes-dir --id --outcome done --date <merge-date> [--results <path>] --message "<msg>"`), `render-change-links.sh --change-file <path> --adrs-dir <dir>`, and `terminal-publish.sh --id <id> --outcome done --integration-branch … --metadata-branch docket --changes-dir … --adrs-dir … --message "<msg>"` — all unchanged, invoked verbatim as finalize step 3 invokes them.
- Produces: nothing consumed by a later task (single-task change). The edited prose is the deliverable.

This is a documentation change; TDD here means: write the sentinel test first (it must FAIL against the current double-archive prose for the *new* assertions), then edit the prose to make it pass, mutation-test each new assertion for non-vacuity.

- [ ] **Step 1: Write the failing sentinel tests**

In `tests/test_closeout.sh`, locate the existing call-site wiring sentinels block (the three `wiring(status): sweep invokes …` lines near the end of the file). The `STATUS="$REPO/skills/docket-status/SKILL.md"` variable is already defined there. Append these four NEW assertions immediately after the existing `wiring(status): sweep invokes cleanup-feature-branch.sh` line (keep the three existing `wiring(status)` lines unchanged):

```bash
# --- change 0036: the sweep delegates archiving to archive-change.sh (no manual double-archive) ---
# The sweep's per-change archive must NOT hand-roll the move any more (mirrors the finalize sentinel).
assert "wiring(status): sweep has no leftover raw archive bash (git mv active/)" \
  '! grep -qE "git mv .*active/" "$STATUS"'
# The renderer re-render must be ordered AFTER archive-change.sh and BEFORE terminal-publish
# (LEARNINGS #0035 — anchor to the unique "before … terminal-publish" phrasing, assert order not presence).
assert "wiring(status): sweep re-renders the Artifacts block before terminal-publish" \
  'awk "/render-change-links\\.sh/{r=NR} /terminal-publish\\.sh/{if(r && r<NR){print \"ok\"; exit}}" "$STATUS" | grep -q ok'
assert "wiring(status): sweep names render-change-links.sh in the delegated archive flow" \
  'grep -q "render-change-links.sh" "$STATUS"'
# The sweep's failure posture is log-and-continue (its own unique phrasing), NOT abort-and-report.
assert "wiring(status): sweep failure posture is log-and-continue (abandon the remainder of this change)" \
  'grep -qiE "abandon the remainder of (this|THIS) change" "$STATUS"'
assert "wiring(status): sweep does NOT adopt finalize abort-and-report for the archive flow" \
  '! grep -qiE "non-zero . abort-and-report" "$STATUS"'
```

Note the `awk` ordering assertion: it records the line number of the LAST `render-change-links.sh` mention (`r`) and, on encountering `terminal-publish.sh`, prints `ok` only if a render line preceded it. This asserts *order*, not mere presence (per spec testing-approach point 2 and the #0021/#0015 ordering-sentinel lesson).

- [ ] **Step 2: Run the tests to verify the NEW assertions fail (and old ones still pass)**

Run: `bash /Users/homer/dev/docket/.worktrees/status-sweep-double-archive/tests/test_closeout.sh 2>&1 | grep -E 'status\):'`

Expected: the five existing-plus-new `wiring(status)` lines. The NEW ones must report **NOT OK** because the current prose still has `git mv active/` (so the no-leftover assertion fails), has no `abandon the remainder of this change` phrasing (failure-posture assertion fails), and the render→publish ordering inside the sweep step is currently a render BEFORE the archive script (step d renders, step f publishes — but the current step d render is on the active→archive done-transition commit BEFORE step f's archive-change.sh, and there is a terminal-publish in step f; the awk may currently pass by luck, so verify). Specifically confirm at minimum:
- `wiring(status): sweep has no leftover raw archive bash (git mv active/)` → **NOT OK** (current prose has `git mv active/` in step c).
- `wiring(status): sweep failure posture is log-and-continue …` → **NOT OK** (phrase absent).

If the ordering or render-presence assertion already passes on the current prose, that is acceptable (those behaviors partly pre-exist via #0035); the load-bearing new failures are the `git mv` removal and the explicit log-and-continue posture. Record which assertions are red.

- [ ] **Step 3: Rewrite the sweep prose (steps c–f → delegated flow)**

In `skills/docket-status/SKILL.md`, replace the block from sub-step `c.` through the `In main-mode …` line that closes sub-step `f.` (the lines beginning `c. \`git mv active/…\`` down to and including `In main-mode the metadata working tree *is* the integration branch, so the archive commit is itself the terminal record and \`terminal-publish.sh\` is a no-op (its own mode-guard fires).`) with the following. Keep sub-steps `a.` and `b.` above it unchanged, and `g.` / `h.` below it unchanged.

````markdown
   c. **Archive (delegated to `archive-change.sh`).** Author the commit message, determine whether a `results:` file exists (it arrived via the PR merge → pass `--results <path>`), then invoke the archive primitive — the same call `docket-finalize-change`'s step 3 uses:
      ```
      "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/archive-change.sh --changes-dir .docket/<changes_dir> --id <id> --outcome done --date <merge-date> [--results <path>] --message "<msg>"
      ```
      The script owns the dated `archive/<merge-date>-<id>-<slug>.md` move (with reuse-existing-file idempotency, including across a day boundary), the `status: done` / `updated: <merge-date>` / `results:` writes, the **change-file-only** commit, and the push-with-rebase-retry on `origin/docket`, plus fail-closed self-verification the old hand-rolled path lacked. **Per-change failure posture (below): trust the exit code — `0` ⇒ archived; non-zero ⇒ log and move to the next change.**

   d. **Re-render the `## Artifacts` block (follow-on commit, before publish).** After `archive-change.sh` returns `0`, regenerate the block on the **archived** file: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-change-links.sh --change-file .docket/<changes_dir>/archive/<merge-date>-<id>-<slug>.md --adrs-dir .docket/<adrs_dir>` (plan/results re-point to the integration branch at `done`; the renderer is the sole writer of the block). Commit this as a **separate follow-on metadata commit** on `docket` and **push `origin/docket`** — it must land on `origin/docket` **before** the publish below, because `terminal-publish.sh` copies the change file *from `origin/docket`*; publishing before the re-render lands would copy the stale block (the #0035 footgun). It is a separate commit, never bundled into the script-owned archive commit, which must stay change-file-only and byte-identical across concurrent archivers (the determinism invariant).

   e. **Publish the terminal record (`docket`-mode).** Reached **only if the step-d re-render commit landed on `origin/docket`**: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/terminal-publish.sh --id <id> --outcome done --integration-branch <integration_branch> --metadata-branch docket --changes-dir <changes_dir> --adrs-dir <adrs_dir> --message "<msg>"` — copies the now-re-pointed terminal records from `origin/docket` onto the integration branch. The script's reuse-existing-file idempotency makes a sweep racing `docket-finalize-change` on the same change a safe no-op. In `main`-mode the metadata working tree *is* the integration branch, so the step-c archive commit is itself the terminal record and `terminal-publish.sh` is a no-op (its own mode-guard fires); the step-d renderer still runs once to re-point the block in place.

   **Per-change failure posture (steps c–e).** The sweep is a **bulk best-effort safety net** run unattended — its other steps (cleanup `g`, harvest `h`, the integration sync) are already explicitly log-and-continue. The three delegated archive steps take the **same** posture: on a non-zero exit from `archive-change.sh`, the renderer follow-on commit/push, **or** `terminal-publish.sh`, **log it, abandon the remainder of this change's close-out, and continue to the next change.** "Abandon the remainder" carries the #0035 guard — a failed archive skips the re-render and publish, and a **failed re-render commit skips publish**, so a stale `## Artifacts` block is never published. The next sweep self-heals idempotently (each script is a reuse-existing / byte-identical no-op on the already-done portion and re-attempts the rest). This is **deliberately divergent from `docket-finalize-change`'s step 3**, whose `non-zero ⇒ abort-and-report` fits a single-change close-out, not a janitor draining N changes — the sequence is shared, the failure posture is not.
````

- [ ] **Step 4: Verify the Note (must not diverge) is still accurate; adjust only if needed**

Re-read the trailing `**Note:** This archive procedure is **identical** to \`docket-finalize-change\`'s per-change archive …` paragraph. It must still read true. After this edit the sweep and finalize step 3 describe the identical archive+render+publish *sequence* (differing only in failure posture, which the Note does not claim identical). The Note as written says "same UTC merge date, same change-file-only commit, same reuse-existing-file idempotency, same terminal-publish invocation. Both skills describe the same operation; they must not diverge." This stays accurate and needs **no edit** — but confirm the words "must not diverge" remain present (the convention's *and the test's* anchor). Do not weaken it.

- [ ] **Step 5: Run the full test_closeout.sh suite to verify all assertions pass**

Run: `bash /Users/homer/dev/docket/.worktrees/status-sweep-double-archive/tests/test_closeout.sh 2>&1 | grep -E 'NOT OK|status\):' ; echo "exit: $?"`

Expected: every `wiring(status)` line reports `ok - …`; no `NOT OK` lines anywhere. The three pre-existing `wiring(status): sweep invokes …` assertions still pass (the delegated flow still names all three scripts). The five new assertions now pass.

- [ ] **Step 6: Mutation-test each NEW assertion for non-vacuity**

For each new assertion, temporarily mutate the prose to confirm the assertion flips to NOT OK, then revert. Use a scratch copy so the real file is never left mutated:

```bash
cd /Users/homer/dev/docket/.worktrees/status-sweep-double-archive
cp skills/docket-status/SKILL.md /tmp/STATUS.bak
# (1) re-introduce a manual git mv -> the no-leftover assertion must flip to NOT OK
printf '\n   x. `git mv active/<id>-<slug>.md archive/<d>.md`\n' >> skills/docket-status/SKILL.md
bash tests/test_closeout.sh 2>&1 | grep -E 'no leftover raw archive'   # expect NOT OK
cp /tmp/STATUS.bak skills/docket-status/SKILL.md
# (2) swap the posture phrasing to abort-and-report -> posture assertions must flip
perl -0pi -e 's/abandon the remainder of this change/abort-and-report on this change/' skills/docket-status/SKILL.md
bash tests/test_closeout.sh 2>&1 | grep -E 'log-and-continue'          # expect NOT OK
cp /tmp/STATUS.bak skills/docket-status/SKILL.md
# (3) remove the renderer mention -> ordering + presence assertions must flip
perl -0pi -e 's/render-change-links\.sh/REMOVED-RENDERER/g' skills/docket-status/SKILL.md
bash tests/test_closeout.sh 2>&1 | grep -E 'before terminal-publish|names render-change-links'  # expect NOT OK
cp /tmp/STATUS.bak skills/docket-status/SKILL.md
# confirm fully reverted and green
bash tests/test_closeout.sh 2>&1 | grep -E 'NOT OK' && echo "STILL RED — investigate" || echo "all green after revert"
rm -f /tmp/STATUS.bak
```

Expected: each mutation prints the corresponding NOT OK line; the final check prints `all green after revert`.

- [ ] **Step 7: Run the broader sentinel suites that touch the sweep prose (regression guard)**

Run each and confirm no new failures (some are unrelated and may have pre-existing skips — compare to the base if unsure):

```bash
cd /Users/homer/dev/docket/.worktrees/status-sweep-double-archive
for t in test_closeout test_composition_wiring test_convention_extraction test_change_links_coverage test_learnings_ledger; do
  echo "=== $t ==="; bash tests/$t.sh >/tmp/$t.out 2>&1; grep -c '^ok' /tmp/$t.out | sed 's/^/ok: /'; grep '^NOT OK' /tmp/$t.out || echo "  (no failures)"
done
```

Expected: no `NOT OK` lines in any suite. (These suites grep the SKILL bodies; the edit must not break an unrelated sentinel.)

- [ ] **Step 8: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/status-sweep-double-archive
git add skills/docket-status/SKILL.md tests/test_closeout.sh
git commit -m "feat(0036): sweep delegates archive to archive-change.sh (remove double-archive)

Collapse the merge-sweep steps c-f into the delegated three-step flow
byte-aligned with docket-finalize-change step 3: archive-change.sh, then a
render-change-links.sh follow-on commit pushed to origin/docket before
terminal-publish, then terminal-publish. Keep the sweep's own
log-and-continue failure posture (deliberately divergent from finalize's
abort-and-report). Sentinel tests assert delegation, the no-leftover-git-mv
collapse, the renderer-before-publish ordering (#0035), and the posture.

Claude-Session: https://claude.ai/code/session_017yfnCscGdanqv4LBbUGwEo"
```

---

## Self-Review

**1. Spec coverage:**
- Spec "Target sweep per-change flow" (archive delegated / re-render follow-on / publish) → Task 1 Step 3. ✓
- Spec "Per-change failure posture" (log-and-continue, distinct from finalize) → Task 1 Step 3 posture paragraph + Steps 1/6 assertions. ✓
- Spec Open Q1 (nothing dropped) → handled by *using* the script (the field writes the script owns are unchanged); the only manual behavior not in the script (the #0035 renderer call) is preserved in step d. ✓
- Spec Open Q2 (renderer ordering) → Task 1 Step 3 step d + ordering assertion in Step 1. ✓
- Spec testing approach points 1–5 → Step 1 assertions (1 delegation, 2 ordering, 3 git-mv-gone, 4 must-not-diverge note, 5 posture) + Step 6 mutation tests. ✓
- Spec "two must not diverge notes accurate" → Task 1 Step 4. ✓
- Spec "No ADR / no script change" → Global Constraints; no task touches scripts or the convention. ✓

**2. Placeholder scan:** No TBD/TODO/"handle edge cases". All prose is verbatim; all commands are exact with expected output. ✓

**3. Type consistency:** No code symbols; the script flags used (`--changes-dir/--id/--outcome/--date/--results/--message` for archive; `--change-file/--adrs-dir` for render; the terminal-publish flag set) match the verified live interfaces and finalize step 3's invocations exactly. The sub-step relabel (old c–f → new c,d,e + posture paragraph) is internally consistent; a/b/g/h labels are unchanged. ✓
