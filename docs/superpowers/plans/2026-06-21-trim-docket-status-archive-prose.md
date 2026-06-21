# Trim docket-status's residual archive-internals prose Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Build-inline directive (LEARNINGS #1):** This is a *single-artifact* edit — one section of one file (`skills/docket-status/SKILL.md`). Per LEARNINGS.md #1 ("build inline when tasks share one artifact; fan out only for genuinely independent tasks"), execute this one task **inline in the controlling session**, not via a fanned-out subagent. There is exactly one task.

**Goal:** Trim `docket-status`'s merge-sweep **step c** so it stops re-narrating the internals that `archive-change.sh` owns (now fully documented in `scripts/archive-change.md`), keeping only the operational facts the skill needs — finishing the body↔contract boundary #37 deliberately deferred for this one file.

**Architecture:** Pure prose relocation in a single Markdown file. Replace step c's third sentence (the duplicated internals enumeration) with the same concise "trust the exit code → see the contract" phrasing #37 already applied to the sibling `docket-finalize-change` step 3 — but preserving the sweep's *own* `log-and-continue` failure posture (NOT finalize's `abort-and-report`). Every other line of the sweep is untouched. No script, code, or test changes.

**Tech Stack:** Markdown skill body; Bash sentinel tests under `tests/` (run with `bash tests/test_*.sh`, asserting `ok -`/`NOT OK -` lines, non-zero exit on any failure).

## Global Constraints

- **Edit exactly one file:** `skills/docket-status/SKILL.md`. No script, test, or other-skill edits.
- **Preserve verbatim (test-guarded by `tests/test_closeout.sh:279–296`):** the `/archive-change.sh` invocation; **no** raw `git mv active/` bash may appear; `render-change-links.sh` must remain ordered **before** `terminal-publish.sh`; the phrase **"abandon the remainder of this change"**; the phrase **"deliberately divergent from `docket-finalize-change`"**.
- **Preserve in meaning (review-guarded, LEARNINGS #5/#36/#37):** step d's #0035 re-render-before-publish ordering; the per-change `log-and-continue` failure-posture paragraph (steps c–e); the determinism invariant; the "identical to finalize" note. Keep must-preserve substrings in their **meaningful grammatical location** — never relocate a token merely to satisfy a grep (LEARNINGS #37 false-GREEN).
- **Keep the relied-upon operational fact:** step c must still state the archive primitive commits **the change file only** (the step-d re-render + board stay separate commits; the determinism invariant depends on it) and must keep a **pointer to `scripts/archive-change.md`** for the mechanics (LEARNINGS #20 progressive-disclosure: leave a pointer so cross-refs resolve).
- **Add no new doc sentinel.** The change is guarded by the *existing* sweep sentinels + whole-branch review (per the change body); a new `! grep "removed-token"` absence sentinel is the fragile anti-pattern LEARNINGS #36/#2 warn against. Do not add one.

---

### Task 1: Trim docket-status sweep step c's archive-internals enumeration

**Files:**
- Modify: `skills/docket-status/SKILL.md` (the "Merge sweep" section, step c — currently ~lines 141–145)
- Verify against (read-only, do not edit): `tests/test_closeout.sh:279–296`, `scripts/archive-change.md`, `skills/docket-finalize-change/SKILL.md:74–80` (the mirror)

**Interfaces:**
- Consumes: nothing from earlier tasks (sole task).
- Produces: nothing later tasks rely on (sole task).

- [ ] **Step 1: Capture the GREEN baseline**

The "tests" for this doc change are the existing sentinels, which are currently green and must *stay* green. Establish the baseline before editing so the trim is the only variable.

Run (from the feature worktree root):
```bash
bash tests/test_closeout.sh | grep -E 'NOT OK' || echo "closeout: ALL OK"
for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 && echo "PASS $t" || echo "FAIL $t"; done
```
Expected: `closeout: ALL OK`, and every line `PASS tests/...`. If anything is FAIL at baseline, STOP — the tree was already red before the edit (not caused by this change).

- [ ] **Step 2: Apply the exact edit**

In `skills/docket-status/SKILL.md`, replace step c's body. The change is: **drop the third sentence** (the internals enumeration "The script owns the dated `archive/…` move … plus fail-closed self-verification the old hand-rolled path lacked.") and **fold the trailing failure-posture pointer into a `Trust the exit code` sentence** that mirrors finalize step 3, keeping the change-file-only fact + the contract pointer.

Replace this exact block:
```
   c. **Archive (delegated to `archive-change.sh`).** Author the commit message, determine whether a `results:` file exists (it arrived via the PR merge → pass `--results <path>`), then invoke the archive primitive — the same call `docket-finalize-change`'s step 3 uses:
      ```
      "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/archive-change.sh --changes-dir .docket/<changes_dir> --id <id> --outcome done --date <merge-date> [--results <path>] --message "<msg>"
      ```
      The script owns the dated `archive/<merge-date>-<id>-<slug>.md` move (with reuse-existing-file idempotency, including across a day boundary), the `status: done` / `updated: <merge-date>` / `results:` writes, the **change-file-only** commit, and the push-with-rebase-retry on `origin/docket`, plus fail-closed self-verification the old hand-rolled path lacked. **Per-change failure posture (below): trust the exit code — `0` ⇒ archived; non-zero ⇒ log and move to the next change.**
```

With this exact block:
```
   c. **Archive (delegated to `archive-change.sh`).** Author the commit message, determine whether a `results:` file exists (it arrived via the PR merge → pass `--results <path>`), then invoke the archive primitive — the same call `docket-finalize-change`'s step 3 uses:
      ```
      "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/archive-change.sh --changes-dir .docket/<changes_dir> --id <id> --outcome done --date <merge-date> [--results <path>] --message "<msg>"
      ```
      **Trust the exit code:** `0` ⇒ archived (idempotent no-op if it was already archived — including across a day boundary, since it reuses the existing dated filename); non-zero ⇒ **per the per-change failure posture below, log and move to the next change.** The script owns the mechanics (see `scripts/archive-change.md`); the one fact the steps below rely on is that it commits **the change file only** on `origin/docket` — so the step-d re-render and the board stay separate commits and concurrent archivers converge tree-identically (the determinism invariant).
```

Rationale (do not paste into the file): this mirrors `docket-finalize-change/SKILL.md:80` near-verbatim, with three deliberate, correct divergences — (1) `abort-and-report` → the sweep's `log-and-continue` pointer; (2) `metadata_branch`/`step 5`/`re-point` → docket-status's `origin/docket`/`step-d re-render`/`board` vocabulary; (3) an explicit `(the determinism invariant)` tie to the invariant later in the section.

- [ ] **Step 3: Re-run the sentinels + full suite — confirm still GREEN**

Run:
```bash
bash tests/test_closeout.sh | grep -E 'NOT OK' || echo "closeout: ALL OK"
for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 && echo "PASS $t" || echo "FAIL $t"; done
```
Expected: identical to the baseline — `closeout: ALL OK` and every `PASS tests/...`. A prose-only edit must not flip any sentinel. If `test_closeout.sh` reports `NOT OK` for any of wiring(status) lines 279–296, the edit disturbed a guarded invariant — revert Step 2 and re-derive.

- [ ] **Step 4: Content-fidelity self-check (anchored, positive)**

Confirm the must-preserve anchors survive *in step c / the sweep* and the duplicated internals are gone. Run from the feature worktree root:
```bash
F=skills/docket-status/SKILL.md
echo "--- must PRESENT (positive anchors) ---"
grep -qF '/archive-change.sh' "$F"                                && echo "ok: archive-change.sh invocation"
grep -qF 'commits **the change file only**' "$F" || grep -qF 'change file only' "$F"  && echo "ok: change-file-only fact retained"
grep -qF 'see `scripts/archive-change.md`' "$F"                   && echo "ok: pointer to contract"
grep -qiE 'abandon the remainder of (this|THIS) change' "$F"      && echo "ok: log-and-continue posture phrase"
grep -qiE 'deliberately divergent from .?docket-finalize-change' "$F" && echo "ok: divergence framing"
awk '/render-change-links\.sh/{r=NR} /terminal-publish\.sh/{if(r && r<NR){print "ok: render-before-publish order"; exit}}' "$F"
echo "--- duplicated internals enumeration should be GONE from step c ---"
grep -nF 'push-with-rebase-retry on `origin/docket`, plus fail-closed self-verification the old hand-rolled path lacked' "$F" \
  && echo "STILL PRESENT — trim incomplete" || echo "ok: internals enumeration removed"
```
Expected: every `ok:` line prints, and the final check prints `ok: internals enumeration removed`. (These are *verification* greps run by hand, not new committed test sentinels — see Global Constraints.)

- [ ] **Step 5: Commit**

```bash
git add skills/docket-status/SKILL.md docs/superpowers/plans/2026-06-21-trim-docket-status-archive-prose.md
git commit -m "docs(docket-status): trim sweep step c archive-internals onto scripts/archive-change.md (#39)

Mirror #37's finalize step 3 trim: defer archive-change.sh's mechanics to its
contract, keep only the operational facts (invocation, args, trust-the-exit-code,
the change-file-only fact + contract pointer). Preserve the sweep's log-and-continue
posture, step d's #0035 re-render-before-publish ordering, and the determinism
invariant verbatim. No behaviour change; existing sweep sentinels stay green.

Claude-Session: https://claude.ai/code/session_01LvLsyayucMFFS1DwE6dtay"
```

(Commit message subject reads `docs(docket-status):` — the conventional-commit type for a documentation-only change. The plan file rides with the code commit as a build artifact, per the convention's metadata/code split.)
