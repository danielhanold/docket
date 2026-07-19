# Finalize-marker Clearing-rule Wording Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Re-phrase the `## Finalize blocked` clearing rule so it asserts the property that is actually true — every reader of the marker is scoped to a pre-`done` change — instead of an over-broad universal that demands strip-on-archive, and re-point the convention's independent restatement at the owning skill.

**Architecture:** Two prose edits on the `main` line, no code and no behavior change. (1) In `skills/docket-finalize-change/SKILL.md`, replace **only** the closing sentence of the clearing-rule bullet, keeping "A successful finalize removes the section" and its machine-verifiable-condition rationale intact. (2) In `skills/docket-convention/SKILL.md`, replace the clearing clause of the `## Finalize blocked` body-section entry with a pointer to the skill, keeping the skip-scoping and named-id-retry clauses intact in meaning. Verification is the existing sentinel suite plus a read for meaning — the suite must go green **with no test edit**.

**Tech Stack:** Markdown only. Bash sentinel tests (`tests/test_finalize_disposition.sh`, `tests/test_render_board.sh`, `tests/test_docket_frontmatter.sh`), run directly with `bash`.

## Global Constraints

- **Wording only — no behavior change.** `git diff` must touch `.md` files under `skills/` only (plus this plan file). No `.sh` file changes.
- **No test edit.** `tests/test_finalize_disposition.sh` must pass unmodified. A required edit there means the wording moved further than intended — re-read rather than re-anchoring the sentinel.
- **Keep verbatim:** the phrase `A successful finalize removes the section.` (sentinel at `tests/test_finalize_disposition.sh:120` matches `(remove|clear)s?.{0,40}section`).
- **Keep verbatim in the convention:** `later **auto-detect** finalize runs skip the change` and `retries a marked change by **naming its id**` (sentinels at `tests/test_finalize_disposition.sh:130-133`).
- **Do not** touch `scripts/archive-change.sh`, the stale-marker health check (change 0098), `README.md:184`, or `scripts/lib/docket-frontmatter.sh` — all explicitly out of scope.
- **Never hard-code a roster of readers** as the load-bearing claim. Change 0098 added a third reader the same day the spec was authored; readers may be named parenthetically as illustration only.
- The `presence-encoded-state` learning stays correct as written — the new text must *discharge* its enumeration, never weaken or contradict the finding.

---

## File Structure

- `skills/docket-finalize-change/SKILL.md` — owner of the clearing rule. Line 162, the third bullet under `### \`## Finalize blocked\` — marking a change that needs a human`. Only the bullet's closing sentence changes.
- `skills/docket-convention/SKILL.md` — line 171, the `## Finalize blocked` entry in the *Change body sections* list. Only the clearing clause changes, into a pointer.

Both files change together: they state one contract at two altitudes, and the convention's entry bundles three clauses of which only one is being re-pointed. A reviewer must read them as a pair to see that the skip-scoping and retry clauses survived — so this is one task, not two.

---

### Task 1: Re-phrase the clearing rule and re-point the convention

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md:162`
- Modify: `skills/docket-convention/SKILL.md:171`
- Test: `tests/test_finalize_disposition.sh` (existing, unmodified), `tests/test_render_board.sh`, `tests/test_docket_frontmatter.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks — this is the only task.
- Produces: the settled clearing-rule phrasing. No later task depends on it.

- [ ] **Step 1: Run the three guard suites on the untouched branch to establish the green baseline**

This change's suite must be green *before* and *after* — a pre-existing red would otherwise read as damage from the edit.

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/finalize-marker-clearing-rule-wording
bash tests/test_finalize_disposition.sh; echo "exit=$?"
bash tests/test_render_board.sh; echo "exit=$?"
bash tests/test_docket_frontmatter.sh; echo "exit=$?"
```
Expected: all three print `exit=0` with no `NOT OK` lines.

- [ ] **Step 2: Confirm the target sentence is byte-for-byte what the spec quotes**

Run:
```bash
grep -n "State encoded by an artifact's presence must be cleared by every transition out of that state." skills/docket-finalize-change/SKILL.md
```
Expected: exactly one hit, on line 162. If zero hits or more than one, STOP — the base moved and the edit must be re-derived against current text.

- [ ] **Step 3: Replace the closing universal in the finalize skill**

In `skills/docket-finalize-change/SKILL.md`, the bullet currently reads:

```markdown
- **A successful finalize removes the section.** Unlike auto-groom's human-only re-arm, the condition is machine-verifiable (the gate passed), so requiring a human to delete it would strand stale markers on changes that are fine. State encoded by an artifact's presence must be cleared by every transition out of that state.
```

Replace that whole bullet with:

```markdown
- **A successful finalize removes the section.** Unlike auto-groom's human-only re-arm, the condition is machine-verifiable (the gate passed), so requiring a human to delete it would strand stale markers on changes that are fine. That removal is a **live-path obligation** — an `implemented` change that stays `implemented` must not carry a stale needs-you cell — and **not** a guarantee about archived files: nothing strips the section at close-out, so on an out-of-band human merge it rides into `archive/` verbatim. It does not need to. **Every reader of the marker is scoped to a change that has not yet reached `done`** (today: the board cell, the auto-detect selection skip, and the `stale-finalize-blocked` health check — each `implemented`-or-unmerged-only), so archiving retires the marker's meaning whether or not the section is physically present. This is the presence-encoded-state rule discharged, not waived: removal is its usual *means*, and the *end* is that no reader is left misinformed — where every reader has already stopped consulting the artifact, its presence encodes nothing. Do **not** add strip-on-archive to satisfy the rule literally; it would destroy the record of why a change stalled, and put body surgery into the shared terminal primitive for zero observable gain.
```

Note the reader list is parenthetical and prefixed `today:` — it illustrates, it does not carry the claim.

- [ ] **Step 4: Verify the finalize-skill sentinels still pass, with no test edit**

Run:
```bash
bash tests/test_finalize_disposition.sh; echo "exit=$?"
git diff --name-only tests/
```
Expected: `exit=0`, and `git diff --name-only tests/` prints **nothing** (no test was modified).

- [ ] **Step 5: Re-point the convention's restatement**

In `skills/docket-convention/SKILL.md`, line 171 currently reads:

```markdown
- `## Finalize blocked` — dated record appended by `docket-finalize-change` when a gate failure leaves a change needing a human; presence drives the board's `finalize blocked — needs you` cell and makes later **auto-detect** finalize runs skip the change. Cleared automatically by a successful finalize; a human retries a marked change by **naming its id**, which overrides the skip (no manual delete needed).
```

Replace it with:

```markdown
- `## Finalize blocked` — dated record appended by `docket-finalize-change` when a gate failure leaves a change needing a human; presence drives the board's `finalize blocked — needs you` cell and makes later **auto-detect** finalize runs skip the change. A human retries a marked change by **naming its id**, which overrides the skip (no manual delete needed). The clearing rule — when the section is removed, and why archiving does not require it — is owned by `docket-finalize-change`'s `## Finalize blocked` section and is not restated here.
```

Two clauses are load-bearing and survive **verbatim**: `later **auto-detect** finalize runs skip the change` and `retries a marked change by **naming its id**`. Only the `Cleared automatically by a successful finalize;` clause becomes a pointer.

- [ ] **Step 6: Verify the convention sentinels still pass**

Run:
```bash
bash tests/test_finalize_disposition.sh; echo "exit=$?"
bash tests/test_render_board.sh; echo "exit=$?"
bash tests/test_docket_frontmatter.sh; echo "exit=$?"
```
Expected: all three `exit=0`, no `NOT OK` lines.

- [ ] **Step 7: Prove the diff is wording-only**

Run:
```bash
git diff --name-only
```
Expected: exactly two paths, both `.md` under `skills/`:
```
skills/docket-convention/SKILL.md
skills/docket-finalize-change/SKILL.md
```
(The plan file itself is committed separately in Step 9.) If any `.sh` path appears, the change exceeded its scope — revert it.

- [ ] **Step 8: Read the edited bullet end-to-end for meaning**

Per `learnings/foundational-test-discipline.md`, a green sentinel proves a phrase still *exists*, not that it is still *true* — the read is the verification for this change.

Run:
```bash
sed -n '156,165p' skills/docket-finalize-change/SKILL.md
sed -n '171p' skills/docket-convention/SKILL.md
```
Confirm all four hold, and fix inline if any fails:
1. No sentence asserts a guarantee about archived files.
2. The finalize-path removal still reads as a live-path obligation, not an optional nicety.
3. The load-bearing claim is the scoping *property*; no count or roster carries it.
4. The convention no longer states the clearing rule independently, and still carries the auto-detect skip scoping and the named-id retry.

- [ ] **Step 9: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md skills/docket-convention/SKILL.md docs/superpowers/plans/2026-07-19-finalize-marker-clearing-rule-wording.md
git commit -m "docs(0099): scope the Finalize-blocked clearing rule to its readers

Replace the closing universal ('State encoded by an artifact's presence must
be cleared by every transition out of that state') with the property that is
actually true: every reader of the marker is scoped to a pre-done change, so
archiving retires its meaning whether or not the section is present.

Keeps 'A successful finalize removes the section' — a real live-path cleanup,
and the phrase tests/test_finalize_disposition.sh:120 is anchored on. Re-points
the convention's independent restatement at the owning skill.

Wording only, no behavior change."
```

---

## Self-Review

**1. Spec coverage.** Spec *Scope — In* lists three items: the closing sentence of the clearing-rule bullet (Step 3), the convention's clearing clause re-pointed rather than restated (Step 5), and "a spec-derived note only if the edit moves sentinel-matched text (it should not)" — Steps 4 and 6 confirm no sentinel-matched text moved, so no note is required. Spec *Verification* items 1–4 map to Steps 4/6, 6, 8, and 7 respectively. The reconcile addendum ("phrase the property, not a roster") is enforced by Step 3's `today:` framing and Step 8's check 3. The spec's *Explicitly decided against* section is carried into the new text's final sentence, so a future reader does not re-derive strip-on-archive as an open thread.

**2. Placeholder scan.** No TBD/TODO; both replacement texts are given in full, verbatim and complete; every command has an expected result.

**3. Type consistency.** No code, so no signatures. The two anchored phrases are quoted identically in Global Constraints, Step 3, and Step 5.

**Scope check:** single subsystem (docket's own prose contract), one task — no sub-project split warranted.
