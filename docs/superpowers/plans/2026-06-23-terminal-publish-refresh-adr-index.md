# terminal-publish: refresh the integration-branch ADR index — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `scripts/terminal-publish.sh` regenerate the integration-branch ADR index (`<adrs_dir>/README.md`) **from the integration branch's own ADR files** and include it in the **same publish commit**, whenever (and only when) the publish copies ≥1 ADR file onto the branch. A no-op in `main`-mode. This closes the silent drift where ADR files accumulate on the integration branch while its index keeps pointing at an old high-water mark. Change #0040, spec `docs/superpowers/specs/2026-06-23-terminal-publish-refresh-adr-index-design.md`.

**Architecture:** Three insertions confined to the copy-and-push region of `terminal-publish.sh` (the spec's "Target flow"), plus a fourth optional hardening assertion (A9). No change to copy-set assembly, the mode guard, or argument parsing. The renderer `scripts/render-adr-index.sh` is reused **as-is** (`--adrs-dir DIR`, reads a local dir, emits to stdout, excludes `README.md`). The contract `scripts/terminal-publish.md` gains a behavior subsection + an invariant. Optionally one accuracy touch to `docket-convention`'s terminal-publish copy-set description (build-time call).

**Tech Stack:** Bash (the docket scripts + the `tests/*.sh` harness with real local-bare-origin git fixtures). No external deps. The suite is the de-facto gate (no GitHub Actions CI).

## Global Constraints

- **Render from the integration branch's own ADR set, never the metadata superset** (spec A4, Open Q2). After the copy-set checkout, `$pub/<adrs_dir>` holds main's pre-existing ADR files **plus** this publish's ADR(s); rendering from there keeps every index link resolvable. Copying `metadata:README.md` verbatim would list ADRs whose files are not yet on the branch → dangling links (the #0033 footgun). This is the load-bearing correctness constraint and gets a dedicated mutation test.
- **Same publish commit, not a separate index commit** (spec A6). The integration-branch publish is a single serialized CAS push where atomicity (ADR file + its index row landing together) is the goal; the existing guarded `diff --cached --quiet || commit` captures copy-set **and** index in one commit. (The "index in a separate commit" rule from `render-adr-index.md`/`docket-adr` guards concurrent ADR *creates on `metadata_branch`* — a different context.)
- **Fire only when an ADR is actually published** (`adr_published`, spec A5). True in `--adr` mode (the lone copy-set entry is an ADR); true in `--id` mode iff ≥1 ADR passed the Accepted gate. No spurious index commit on a no-ADR change-publish. Each fire is a *full* re-render, so prior drift self-heals incrementally.
- **Re-render in the CAS retry path too** (spec A7, LEARNINGS #25). The push-reject `else` branch already re-checkouts the copy-set before `rebase --continue`; mirror the render+add there (same `adr_published` guard) so a concurrent push is resolved by deterministic regeneration, never a hand-merge.
- **`main`-mode is a no-op** (spec A8). The existing mode guard (`META_BRANCH == INT_BRANCH`) early-exits before the copy/push region; `docket-adr` already maintains the index in place there. No new branch, no new flag behavior.
- **No new ADR** (spec, "What the implementer edits"). Additive, reversible tooling applying existing decisions (ADR-0012 script-vs-model boundary; render-adr-index's sole-writer/regenerate rules). `adrs: []` stays.
- **Test discipline (LEARNINGS):** capture producer output into a var before `grep` — never `producer | grep -q` under `pipefail` (#11/#16); mutation-test each new assertion for non-vacuity (#20/#2); cover the CAS-retry render by **diverging the same contended path**, not an unrelated file (#25); fixtures use real local-bare-origin git repos, not `/tmp` smoke (#22/#35). Do **not** run a real `terminal-publish.sh` against this repo's `origin/main` — it would publish for real; all coverage is hermetic.
- **`render-adr-index.sh` requires its `--adrs-dir` to exist** — guaranteed at render time because the render only fires after ≥1 ADR file was checked out into `$pub/<adrs_dir>` (verified in reconcile).
- **Commit-message trailers:** every commit ends with
  `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>` and
  `Claude-Session: https://claude.ai/code/session_01YDEJQBf11FQvK3aBAiGcfA`.

---

## File Structure

- `scripts/terminal-publish.sh` — the three insertions (`adr_published` flag during copy-set assembly; render+add after the `pub` checkout guarded by the flag; the mirrored render+add in the CAS retry `else` branch) + the optional A9 self-verify assertion for `<adrs_dir>/README.md` when `adr_published`.
- `scripts/terminal-publish.md` — a behavior subsection documenting the index refresh (when it fires, render-from-branch-set rationale, same-commit, `main`-mode no-op) + an invariant line mirroring `BOARD.md is never published`.
- `tests/test_closeout.sh` — extend the functional suite with the spec's 6-test floor (+ a CAS-retry index assertion folded into the existing conflict test).
- `docs/superpowers/specs/.../...design.md` — read-only (the source of truth).
- *Optional, build-time call:* `skills/docket-convention/SKILL.md` — one-line accuracy touch to the terminal-publish copy-set description (the Branch model line listing "the change file, spec, Accepted ADRs"). Not required for correctness.

The script and its tests change together; the contract `.md` rides the same task.

---

## Task 1: Add the failing functional tests (TDD red)

**Files:**
- Test: `tests/test_closeout.sh` — append after the existing `--adr` blocks, before the `cleanup-feature-branch.sh` section (the `new_repo` fixture + `PUBLISH` var are already in scope).

**Interfaces:**
- Consumes: the existing `new_repo` fixture (docket branch carries `0003-accepted.md` Accepted, `0005-proposed.md` Proposed; `main` baseline has **no** `docs/adrs/`), `ARCHIVE`, `PUBLISH`, `assert`, and the `ls_main` helper pattern.
- Produces: nothing for a later task — the assertions are the deliverable's guard.

Write these assertions FIRST; they must FAIL against the current (un-rendered) script. Capture command output into vars before grepping (#11/#16). The index README is **not** in the copy-set, so the existing tests never assert it — these are all new.

Floor (spec "Testing approach"):

1. **Change-publish (`--id`) with an Accepted ADR** → after publishing change 7 (whose `adrs: [3,5]`, only 3 Accepted), `origin/main:docs/adrs/README.md` exists and lists `ADR-0003`; **every** linked file in the index resolves on `origin/main` (no dangling row); the index and the ADR file landed in the **same** commit (one commit touches both `docs/adrs/0003-accepted.md` and `docs/adrs/README.md`).
2. **ADR-only publish (`--adr 3`)** → `origin/main:docs/adrs/README.md` lists `ADR-0003` and only branch-present ADRs; every linked file resolves.
3. **Renders from the branch set, not the metadata superset (dangling-link guard)** → publish `--adr 5` (Proposed-but-`--adr` ignores the gate). Index on `origin/main` lists `ADR-0005` and does **NOT** list `ADR-0003` (0003's file was never published to `main`, though it is Accepted on docket). This is the mutation test for A4.
4. **No-ADR change-publish** → with a change whose `adrs:` is empty (or whose ADRs all fail the gate so the copy-set has no ADR), `origin/main:docs/adrs/README.md` is unchanged / not created — no spurious back-fill commit. (Build a fixture variant: a change with `adrs: []`; assert publish creates no `docs/adrs/README.md` and the integration tip is byte-stable vs a no-ADR publish.)
5. **`main`-mode** (`--metadata-branch main == --integration-branch main`) → early no-op; no index write (reuse the existing main-mode block, add an assertion that no `README.md` was written).
6. **Idempotent re-run** → re-publishing the same `--id 7` (or `--adr 3`) creates no new integration commit (byte-stable index — `before == after` on `origin/main`).

Also extend the existing **CAS conflict ELSE-branch** test (the one that diverges the same archived change-file path): publish `--id 7` so the copy-set includes `0003-accepted.md`, and assert the landed `origin/main:docs/adrs/README.md` lists `ADR-0003` with no conflict markers — proving the retry-path render fired (A7, #25 same-path divergence).

- [ ] **Step 1: Author the assertions above in `tests/test_closeout.sh`.**
- [ ] **Step 2: Run `bash tests/test_closeout.sh`; confirm the NEW assertions are `NOT OK` (red) and all pre-existing ones stay `ok`.** Capture which fail — these define done.

**Verification:** `bash tests/test_closeout.sh 2>&1 | grep -iE 'index|README|dangling|ADR-000'` shows the new lines failing; the rest of the suite is unchanged.

---

## Task 2: Implement the three insertions in `terminal-publish.sh` (TDD green)

**Files:**
- Modify: `scripts/terminal-publish.sh` — copy/push region only (after line ~108 copy-set assembly through the self-verify loop).
- Test: `tests/test_closeout.sh` (from Task 1).

**Interfaces:**
- Consumes: `render-adr-index.sh --adrs-dir "$pub/$ADRS_DIR"` (unchanged renderer), the existing `$pub`, `$metaref`, `$copyset`, `$ADRS_DIR`, `$REMOTE`, `$INT_BRANCH` locals.
- Produces: a refreshed `$ADRS_DIR/README.md` staged into the same publish commit; the `adr_published` boolean.

Insertions (spec "Target flow inside `terminal-publish.sh`"):

1. **`adr_published` flag during copy-set assembly.** In `--adr` mode set `adr_published=true` (the lone entry is an ADR). In `--id` mode initialize `adr_published=false` and set it `true` inside the Accepted-gate loop each time an entry is appended to `copyset` under `<adrs_dir>` (i.e., when `copyset+=("$apath")` fires for an Accepted ADR). Initialize `adr_published=false` once up top so it is always defined under `set -u`.
2. **Render after the `pub` checkout, into the same staged commit.** Immediately after `git -C "$pub" checkout "$metaref" -- "${copyset[@]}"` (line ~125) and **only when** `adr_published` is true:
   ```bash
   if [ "$adr_published" = true ]; then
     "$(dirname "$0")/render-adr-index.sh" --adrs-dir "$pub/$ADRS_DIR" > "$pub/$ADRS_DIR/README.md" \
       || { teardown; die "adr index render failed"; }
     $GIT -C "$pub" add "$ADRS_DIR/README.md"
   fi
   ```
   The render reads `$pub/<adrs_dir>` (branch ADRs + this publish's ADR overlaid). The existing guarded `diff --cached --quiet || commit` then captures copy-set + index in one commit; a true no-op still creates no commit.
3. **Mirror the render+add in the CAS retry `else` branch.** After the retry `git -C "$pub" checkout "$metaref" -- "${copyset[@]}"` (line ~132) and before `rebase --continue`, repeat the same guarded render+add so a concurrent push is resolved by regeneration, not a stale/conflicted index.

Then (spec A9 hardening, recommended): when `adr_published` is true, add a self-verify assertion that `$ADRS_DIR/README.md` is present on `$REMOTE/$INT_BRANCH` after the push (a separate one-line check — the README is **not** in `copyset`, so this does not alter copy-set semantics).

Keep `set -uo pipefail` semantics intact: `adr_published` always initialized; the render redirection failure is caught (`||`).

- [ ] **Step 1: Apply insertions 1–3 + the A9 assertion.**
- [ ] **Step 2: Run `bash tests/test_closeout.sh`; all assertions (old + new) must be `ok`.**
- [ ] **Step 3: Mutation-check each new assertion (#20):** temporarily neutralize the render line (e.g. comment out insertion 2) and confirm tests 1–3 flip to `NOT OK`; neutralize the `adr_published` guard and confirm test 4 (no spurious back-fill) flips; restore.

**Verification:** `bash tests/test_closeout.sh` is fully green; `bash tests/test_terminal_publish.sh` (arg-validation) still green; mutation runs prove non-vacuity.

---

## Task 3: Document the refresh in `terminal-publish.md` + optional convention touch

**Files:**
- Modify: `scripts/terminal-publish.md` — add an "ADR index refresh" subsection under **Behavior** and an invariant under **Invariants**.
- Test: `tests/test_script_contracts_coverage.sh` (existence audit — already passes; the contract stays co-located). No new sentinel required, but verify the suite stays green.
- *Optional:* `skills/docket-convention/SKILL.md` — append "(and refreshes the integration ADR index)" to the Branch-model sentence listing the terminal-publish copy-set, iff it reads cleanly; skip if it risks tripping an existing convention sentinel.

Document: the refresh fires only when `adr_published` (an ADR is actually copied); it renders from the **integration branch's own** ADR set (the dangling-link rationale); it rides the **same** publish commit; and it is a **`main`-mode no-op**. Add an invariant mirroring the existing `BOARD.md is never published` line, e.g. *the ADR index is refreshed only from the integration branch's published ADR files, and only when an ADR is published.*

- [ ] **Step 1: Write the `.md` subsection + invariant.**
- [ ] **Step 2: (optional) the one-line convention touch, only if clean.**
- [ ] **Step 3: Run `bash tests/test_script_contracts_coverage.sh` and (if convention touched) `bash tests/test_convention_extraction.sh` + any convention sentinel suites — all green.**

**Verification:** the contract reads accurately against the new behavior; the full suite is green.

---

## Task 4: Whole-branch review + full suite

**Files:** all changed files.

- [ ] Run the **entire** `tests/test_*.sh` suite (not just `test_closeout.sh`) — a script edit can ripple. Confirm 0-byte-meaningful stderr where the suite asserts it.
- [ ] `superpowers:requesting-code-review` (whole-branch), re-reading LEARNINGS first. Watch specifically for: a sentinel satisfied from two locations (#21), an ordering assertion latching the wrong line (#15), and any `producer | grep -q` (#16).
- [ ] Record any non-obvious decision as an ADR via the `docket-adr` subagent (expected: none — this change makes no new architectural decision; `adrs: []` stays).

**Verification:** whole suite green; review clean; no ADR warranted.

---

## Out of scope (spec scope guards)

- **Back-filling already-drifted branches** (e.g. docket's own `main` index, currently listing only through ADR-0002 while files reach 0014). This change prevents *future* drift; each fire is a full re-render so the present gap self-heals on the **next** ADR publish, but an on-the-spot back-fill stays a manual `render-adr-index.sh` + push.
- **Publishing any other derived view** (`BOARD.md` stays `metadata_branch`-only).
- **Changing which records terminal-publish copies** beyond adding the rendered index.
- **Touching #0033** (its kill/narrow disposition is the owner's call; never autonomous).
