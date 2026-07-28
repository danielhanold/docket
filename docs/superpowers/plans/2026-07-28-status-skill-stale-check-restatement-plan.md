<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0145 — docket-status SKILL.md restates a stale check count and list the 0111 guard does not pin](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0145-docket-status-skill-md-restates-a-stale-check-count-and-list.md)**
<!-- docket:backlink:end -->

# docket-status SKILL.md stale check restatement — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the stale check-id restatement from `skills/docket-status/SKILL.md`'s `### Health checks` section, and add one section-scoped guard so it cannot silently return.

**Architecture:** Removal, not a fifth pinned surface. Change 0111's correspondence guard pins the check-id vocabulary across four surfaces; `skills/docket-status/SKILL.md` is not one of them, so every new check-id drifts there silently while the guard stays green. Adding a fifth surface would make every future check-id a five-file edit; deleting the restatement makes the drift *impossible* rather than *detected*. The section keeps only what the skill genuinely owns — the warn-only posture and the one-line pointer to `## Judgment follow-ups` — and points at the authoritative enumeration for the rest.

**Tech Stack:** Bash (`set -uo pipefail`), `awk`, `grep`, the repo's hand-rolled `assert` harness in `tests/test_board_checks.sh`. No new dependencies.

## Global Constraints

- **Everything in this plan is a single task.** The SKILL.md edit and the test-file edits *must* land in one commit: the edit reddens three existing asserts, and the new guard is red until the edit lands. Any split leaves an intermediate state with a red suite.
- **Cross-references anchor on a symbol name or a verbatim-quoted clause, never a line number** (`AGENTS.md` → *Comments and cross-references*; ADR-0054). `tests/test_comment_anchor_style.sh` rejects the filename-plus-line-number form. Every comment written by this plan obeys this — placement below is stated **structurally** ("immediately before the `PASS`/`exit "$fail"` epilogue"), never as a line number.
- **A guard is code: mutation-test it** (`AGENTS.md` → *Guards and tests*). The mutation matrix in Step 6 is mandatory, not optional.
- **Run the whole suite at the build gate**, never only the tests this plan enumerates (`AGENTS.md`).
- **Never pipe a producer into an early-exiting consumer.** This file runs under `set -uo pipefail`, where `printf … | grep -q` is a real hazard, not a style preference — `tests/test_board_checks.sh` documents this at its own `has_finding` helper. Use here-strings (`<<<`), which is the file's house idiom.
- **Do not "fix" the count and stop.** The vocabulary is 13 ids today (`BOARD_CHECK_IDS`, `scripts/lib/docket-frontmatter.sh`), but this change *removes* the number rather than correcting it. Correcting it would re-create the same drift with fresher wrong values.

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `skills/docket-status/SKILL.md` | The docket-status skill body. Owns the *posture* of the health pass, not the check-id vocabulary. | Modify: rewrite the `### Health checks` section (the file's last section). |
| `tests/test_board_checks.sh` | Home of the check-id correspondence-guard family; already derives `$emitted` from `scripts/board-checks.sh`'s real `emit` sites, and already defines `$SKILL`. | Modify: delete three now-false asserts in the *docket-status wiring sentinels* block; add the new section-scoped guard immediately before the `PASS`/`exit "$fail"` epilogue. |

Nothing else changes. No script behavior, no check-id vocabulary, no ADR.

---

## Verified starting state

Every fact below was checked against the running code on this branch's base (`origin/main` at `f804c7b2`), not inferred from the spec. Two of them **correct the spec**, so read this section before starting.

1. **`BOARD_CHECK_IDS` holds 13 ids**, not the spec's "12 on `main`, 13 after #129" — change 0117 (PR #129) has **merged**. Not load-bearing (the fix removes the number), but the spec's conditional phrasing is stale.
2. **⚠️ The spec's "No other test changes" is wrong.** Three existing asserts in `tests/test_board_checks.sh` pin the exact invocation block this change deletes, and they *will* redden. Confirmed empirically by applying the edit and running the suite:
   ```
   NOT OK - docket-status Health checks invoke board-checks (via the docket.sh facade)
   NOT OK - the board-checks invocation passes --changes-dir
   NOT OK - the board-checks invocation passes --metadata-branch and --integration-branch
   ```
   Step 4 retires them, with justification.
3. **The three phrases those asserts pin occur *only* inside the doomed section** — `docket.sh board-checks`, `--changes-dir`, `--metadata-branch`/`--integration-branch` appear nowhere else in SKILL.md. So they cannot be salvaged by rewording; the premise is deleted, not re-gated.
4. **The three *other* sentinels in that block survive untouched**, because each phrase also occurs outside the section: `do not auto-fix` (also in `## Overview`), `blocked_by:` and `mirror reachability` (both stated in full under `## Judgment follow-ups`). Do not delete these three.
5. **`### Health checks` is the file's last section**, so the extractor's **EOF arm is the live path** — a two-heading-match extractor yields the empty set and the negative assert passes vacuously forever. The positive anchor is what catches that.
6. **`publish-deferred` legitimately appears in the `### Merge sweep` section**, and is SKILL.md's only check-id occurrence outside the target section. The negative assert **must** stay section-scoped; a file-wide ban would redden honest prose.
7. **The invocation block is genuinely stale** — `scripts/board-checks.sh` accepts `--adrs-dir`, `--lease-ttl-hours`, `--strict`, and `--terminal-publish` in addition to the three flags the block shows.
8. **`$SKILL` and `$emitted` already exist** in `tests/test_board_checks.sh` (`$SKILL` near the top with `$REPO`/`$SCRIPT`; `$emitted` derived in the 0111 block from `scripts/board-checks.sh`'s `emit <id> "` sites). The new guard consumes both — it must **not** re-derive either, and must be placed *after* `$emitted` is assigned.
9. **The baseline suite is green** before any edit.

---

## Task 1: Remove the restatement and guard its return

**Files:**
- Modify: `skills/docket-status/SKILL.md` — the `### Health checks` section
- Modify: `tests/test_board_checks.sh` — the *docket-status wiring sentinels* block, and a new guard before the `PASS`/`exit` epilogue

**Interfaces:**
- Consumes: `$SKILL` (absolute path to `skills/docket-status/SKILL.md`) and `$emitted` (newline-separated, sorted check-ids derived from `scripts/board-checks.sh`), both already defined in `tests/test_board_checks.sh`; the `assert NAME COND` helper, which `eval`s `COND`.
- Produces: shell variables `hc_section` (the extracted section text) and `hc_restated` (newline-separated restated ids, empty when clean). Nothing later in the file consumes these — this guard is file-final.

---

- [ ] **Step 1: Confirm the baseline is green**

Run from the worktree root:

```bash
bash tests/test_board_checks.sh 2>&1 | tail -2
```

Expected: last line `PASS`.

If it is already `FAIL`, stop and report — something upstream is broken and nothing below will be interpretable.

---

- [ ] **Step 2: Write the failing guard**

Open `tests/test_board_checks.sh`. Append the block below **immediately before** the file's final two lines, which are the epilogue:

```bash
if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

This end-of-file placement is deliberate and structural: it sits after `$emitted` is assigned (which the guard consumes) and outside the count-assert region that change 0117 recently rewrote.

Block to append:

```bash

# ============ SKILL.md must not restate the check-id vocabulary (change 0145) ============
# Change 0145 DELETED the count word, the five-item check-id list, and the hand-run invocation
# block from skills/docket-status/SKILL.md's `### Health checks` section. The 0111 correspondence
# guard above pins the check-id set across four surfaces, and SKILL.md was never one of them — so
# every new check-id drifted there silently while this suite stayed green. Rather than add a fifth
# pinned surface (which taxes every future check-id with another edit), the restatement was
# removed; this guard is what keeps it removed.
#
# SCOPED TO THE SECTION, NOT THE FILE. The `### Merge sweep` section legitimately names
# `publish-deferred` while explaining what that mark drives — it is the file's only check-id
# occurrence outside the section below, and a file-wide ban would redden honest prose.
#
# The extractor terminates on the next `^(#|##|###) ` heading OR EOF. The EOF arm is the LIVE
# path, not a fallback: `### Health checks` is currently the file's LAST section, so an extractor
# written as "lines between two heading matches" would yield the empty set and the negative assert
# would pass vacuously forever. The non-vacuity anchor below is what catches that — and catches a
# rename of the heading, which would otherwise silently disable this whole guard.
hc_section="$(awk '/^### Health checks[[:space:]]*$/{inhc=1;next} inhc && /^(#|##|###) /{exit} inhc' "$SKILL")"

assert "SKILL.md's '### Health checks' section is extractable and non-empty (a heading rename must redden, not pass vacuously)" \
  '[ -n "${hc_section//[[:space:]]/}" ]'
assert "SKILL.md's '### Health checks' section points at the authoritative enumeration (scripts/board-checks.md)" \
  'grep -qF "scripts/board-checks.md" <<<"$hc_section"'

# Word-boundary match, deliberately NOT backtick-anchored. A backtick-only matcher would miss a
# list re-added in bare form (`- broken-spec — ...`) AND would pass a mutation check written by
# copying the old backticked list — passing its own test while leaving the hole open. Every
# emitted id is a hyphenated compound that cannot occur in ordinary prose, so word-boundary
# matching costs no false positives. Consumed from a here-string, never a pipe: this file runs
# under `set -uo pipefail`, where piping into an early-exiting `grep -q` is a real hazard.
hc_restated="$(while IFS= read -r cid; do
                 [ -n "$cid" ] || continue
                 grep -qw -- "$cid" <<<"$hc_section" && printf "%s\n" "$cid"
               done <<<"$emitted")"
assert "no check-id is restated in SKILL.md's '### Health checks' section (point at scripts/board-checks.md, never a list)" \
  '[ -z "$hc_restated" ] || { echo "restated check-ids: $(echo $hc_restated)" >&2; false; }'
```

**Known limitation, recorded deliberately:** section scoping means this stops the restatement returning *in this section only*. An editor who re-adds the list under a **new** heading escapes it — the non-vacuity anchor catches a *rename of* `### Health checks`, not a *new* section elsewhere. The file-wide alternative was rejected: it pins prose position and is more brittle than what it buys.

---

- [ ] **Step 3: Run the guard to verify it fails**

```bash
bash tests/test_board_checks.sh 2>&1 | grep -E "SKILL.md's '### Health|restated check|^PASS|^FAIL"
```

Expected — red on both arms, naming the five ids currently restated:

```
ok - SKILL.md's '### Health checks' section is extractable and non-empty (a heading rename must redden, not pass vacuously)
NOT OK - SKILL.md's '### Health checks' section points at the authoritative enumeration (scripts/board-checks.md)
restated check-ids: broken-plan-results broken-spec dep-cycle merge-gate-stall stale-in-progress
NOT OK - no check-id is restated in SKILL.md's '### Health checks' section (point at scripts/board-checks.md, never a list)
FAIL
```

The *extractable and non-empty* assert passing here is the point: it proves the EOF arm works and that the negative assert below it is running against real text rather than the empty string.

---

- [ ] **Step 4: Retire the three asserts whose premise this change deletes**

In `tests/test_board_checks.sh`, inside the block headed `docket-status wiring sentinels (SKILL is code on main)`, **delete** these three asserts:

```bash
assert "docket-status Health checks invoke board-checks (via the docket.sh facade)" \
  'grep -qF "docket.sh board-checks" "$SKILL"'
```

```bash
# Mutation guard: the board-checks invocation passes the changes-dir + both branch refs.
assert "the board-checks invocation passes --changes-dir" 'grep -qF -- "--changes-dir" "$SKILL"'
assert "the board-checks invocation passes --metadata-branch and --integration-branch" \
  'grep -qF -- "--metadata-branch" "$SKILL" && grep -qF -- "--integration-branch" "$SKILL"'
```

Delete the `# Mutation guard: …` comment line with them — it describes only the two asserts it heads.

**Why delete rather than re-gate:** ask what the block *guards*, not what it asserts. These three pin a **copied invocation and its flags** — the same restatement class this change exists to eliminate, and already stale (the real script also accepts `--adrs-dir`, `--lease-ttl-hours`, `--strict`, `--terminal-publish`). The first assert's claim is also simply false after this change, and arguably was before: the skill does not invoke `board-checks` directly, it invokes `docket.sh docket-status`, which runs the checker itself. Their premise is deleted, not re-gated.

**Do NOT touch the other three asserts in that block** — `do not auto-fix`, `blocked_by:`, and `mirror reachability` all match text that survives outside the rewritten section, and all three must stay green.

Then **replace** the now-stale explanatory comment that sits above the surviving asserts. Delete this comment:

```bash
# The five mechanical checks are now delegated — their old standalone bullets are gone as bullets,
# but the SKILL still names them so a reader knows what the script covers. Assert the surviving
# model-driven signals, each anchored to a phrase it owns: the blocked_by re-examination
# (judgment) and the github mirror-reachability visibility flag. Change 0024 retired the inline
# board/source-drift check (deterministic render + the unconditional Board-pass re-render make it
# vacuous); its removed tripwire lives in tests/test_board_refresh_on_transition.sh.
```

and put this in its place:

```bash
# The SKILL no longer names the check-ids at all (change 0145 — see the section-scoped guard at
# the end of this file, which now forbids it). What remains here are the surviving model-driven
# signals, each anchored to a phrase it owns: the blocked_by re-examination (judgment) and the
# github mirror-reachability visibility flag. Change 0024 retired the inline board/source-drift
# check (deterministic render + the unconditional Board-pass re-render make it vacuous); its
# removed tripwire lives in tests/test_board_refresh_on_transition.sh.
```

---

- [ ] **Step 5: Rewrite the SKILL.md section**

In `skills/docket-status/SKILL.md`, replace everything from the line after `### Health checks` down to — **but not including** — the file's final line (the `Two judgment checks stay in-model…` paragraph, which is preserved **byte-for-byte**).

Delete: the count sentence, the fenced invocation block, and the five bullets. The resulting section, in full:

```markdown
### Health checks

Flag what the pass reports (do not auto-fix unless asked): mechanical, git-only, warn-only checks over stale claims, broken spec/plan/results links, and dependency stalls. This skill never runs the checker directly — it invokes `docket.sh docket-status`, which runs it. The closed check-id set and each check's meaning live where they are owned: the per-check sections of `scripts/board-checks.md`, and the `check <check-id>` report-line row in `scripts/docket-status.md`.

Two judgment checks stay in-model, on top of the script: `blocked_by:` re-examination and `github` mirror reachability (see *Judgment follow-ups* above) — both warn-only, never auto-fix.
```

Three properties of that replacement paragraph are load-bearing — preserve them if you reword:

1. It contains the literal `do not auto-fix` (posture, and one of the surviving sentinels).
2. It contains the literal `scripts/board-checks.md` (the guard's non-vacuity anchor).
3. It contains **no** emitted check-id. `check-id` and `check <check-id>` are safe — neither is an id.

The wording "stale claims, broken spec/plan/results links, and dependency stalls" is taken verbatim from SKILL.md's own frontmatter `description:` and `## Overview`, so the three surfaces agree.

The resulting file is 96 lines, down from 107.

---

- [ ] **Step 6: Run the guard to verify it passes, then mutation-test it**

First, the target state:

```bash
bash tests/test_board_checks.sh 2>&1 | tail -2
```

Expected: `PASS`. If any of the three deleted asserts still reddens, Step 4 was incomplete.

Now prove the guard is load-bearing rather than decoration. Each mutation edits `skills/docket-status/SKILL.md`, runs the suite, then reverts with `git checkout -- skills/docket-status/SKILL.md`. **Revert after every cell**, and confirm `git status --porcelain` is clean before the commit.

| # | Mutation | Required result |
|---|---|---|
| A | none (target state) | all three asserts `ok`, suite `PASS` |
| B | rename the heading to `### Health checks and hygiene` | **NOT OK** on *extractable and non-empty* **and** on *points at the authoritative enumeration* |
| C | add a bullet `- broken-spec — spec: set but the path does not resolve.` (**unbackticked**) | **NOT OK** on *no check-id is restated*, reporting `restated check-ids: broken-spec` |
| D | add a bullet ``- **`dep-cycle`** — a depends_on cycle.`` (**backticked**) | **NOT OK** on *no check-id is restated*, reporting `restated check-ids: dep-cycle` |
| E | delete `the per-check sections of `scripts/board-checks.md`, and ` from the paragraph | **NOT OK** on *points at the authoritative enumeration* |

Cell C is the one that matters most: it is the bare-form re-add a backtick-anchored matcher would miss. Cell B is the vacuity trap — note that in B the negative assert goes **`ok`** (the section extracts empty, so no id is found); that is precisely why the non-vacuity anchor exists and why B must be checked on the anchor, not on the negative.

All five cells were run against the real suite during planning and matched these expectations.

---

- [ ] **Step 7: Run the whole suite**

Per `AGENTS.md`, the build gate is the whole suite, not just the tests this plan names. Run every test **in one foreground invocation** and let it finish:

```bash
for t in tests/test_*.sh; do
  printf '%s: ' "$t"
  bash "$t" >/tmp/out.$$ 2>&1 && tail -1 /tmp/out.$$ || { echo "FAIL"; tail -20 /tmp/out.$$; }
done; rm -f /tmp/out.$$
```

Expected: every test reports `PASS`. `tests/test_skill_size_budgets.sh` covers SKILL.md's size and can only benefit from an 11-line reduction.

If a test unrelated to this change fails, check whether it also fails on the unmodified base before assuming this change caused it.

---

- [ ] **Step 8: Commit**

```bash
git add skills/docket-status/SKILL.md tests/test_board_checks.sh
git commit -m "docs(0145): drop the stale check-id restatement from docket-status SKILL.md

The ### Health checks section restated a count ("Five") and a five-item
check-id list against a vocabulary that is now thirteen, plus a hand-run
invocation block missing four flags the script has since gained. Change
0111's correspondence guard pins that vocabulary across four surfaces and
SKILL.md was never one of them, so every new check-id drifted there
silently while the suite stayed green.

Remove the restatement rather than add a fifth pinned surface: removal
makes the drift impossible instead of merely detected, and does not tax
every future check-id with another edit. The section keeps the warn-only
posture and the pointer to ## Judgment follow-ups, and now points at
scripts/board-checks.md and scripts/docket-status.md for the closed set.

Guarded by a section-scoped assert in tests/test_board_checks.sh: a
positive non-vacuity anchor (so a heading rename reddens rather than
passing silently) plus a word-boundary ban on every emitted check-id,
matched unbackticked so a bare-form re-add cannot slip through.

Retires three asserts that pinned the deleted invocation block and its
flags — the same restatement class, and already stale."
```

---

## Self-review

**Spec coverage.** Spec §1 (what SKILL.md keeps) → Step 5, with the three load-bearing properties enumerated. §2 (what it drops) → Step 5, all three items. §3 (the guard: placement, scoped negative, positive anchor, `grep -w`, named limitation) → Step 2, all five elements, limitation recorded verbatim. §Test (three mutation checks) → Step 6, extended from three cells to five (added the backticked re-add and the pointer deletion).

**Spec corrections carried.** §Assumption 9's conditional count → settled at 13 (*Verified starting state* 1). §Test's "No other test changes" → **wrong**, corrected in *Verified starting state* 2 and handled by Step 4. §Assumption 6's file-collision risk → retired, 0117 merged; the end-of-file placement rule is kept on its independent merit and Step 2 says so.

**Placeholder scan.** No TBD/TODO, no "add appropriate…", no "similar to Task N". Every code step carries literal content; every expected output is a real captured run.

**Consistency.** `hc_section` and `hc_restated` are used with the same names throughout. `$SKILL`/`$emitted` are consumed, never redefined, and the guard is placed after `$emitted` is assigned. `awk` uses `/^(#|##|###) /` (plain ERE alternation) rather than `{1,3}` interval syntax, which older BSD `awk` does not support.
