<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0219 — aborted-run's Step 7 seam — a fourth git-only leg, plus GitHub enrichment for leg C](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0219-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d.md)**
<!-- docket:backlink:end -->

# aborted-run's Step 7 Seam Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the `aborted-run` coverage gap at `docket-implement-next`'s Step 7 seam by adding a fourth git-only leg to `board-checks.sh` (manifest carries `pr:` while `status:` is still `in-progress`) and a GitHub enrichment leg to `docket-status.sh` that resolves the ambiguity leg C's `pr:`-unset finding leaves behind.

**Architecture:** Two independent pieces in two scripts, plus doc repairs. Leg D is a purely local frontmatter predicate inside `board-checks.sh`'s existing `status: in-progress` block, sharing a single hoisted `pr:` read with leg C so the cost-sensitive path (change 0176) pays for no second read. The GitHub enrichment is a new `detect_orphan_pr` function in `docket-status.sh` sitting beside `detect_merged` and reusing its best-effort posture verbatim — any `gh`/network/parse failure emits `sweep-skipped <reason>` and returns 0. `board-checks.sh`'s git-only/offline contract is untouched: the `gh` work goes where `gh` already lives.

**Tech Stack:** Bash (floor 4.0, needs `mapfile`/`declare -g`), `git` plumbing, `gh` + `jq` for the enrichment leg, docket's own hermetic shell test suite (`tests/test_*.sh`, run via `scripts/run-tests.sh`).

## Global Constraints

- **`scripts/board-checks.sh` stays git-only.** It shells no `gh` and makes no network call. Every new predicate in it reads frontmatter or git refs only.
- **Advisory posture, unconditionally.** No leg added here flips a status, releases a claim, or writes a file. Every message **hedges** and its remedy is a **verification**, never "push it" / "open the PR" — a prescriptive remedy acted on against a live run races the running agent on its own branch.
- **Anchored frontmatter reads only.** `pr:`, `branch:`, `plan:`, `results:` are all OPTIONAL keys. In `board-checks.sh` read them with `fm_field` (first frontmatter block only, ADR-0057), never `field`. An unanchored read falls through the closing `---` into body prose, and in THIS repo a change body discussing `pr:` is ordinary content — the failure is a silent FALSE NEGATIVE.
- **Reuse `ABORTED_RUN_IDLE_SECS`' 2h value; introduce no second horizon.** Legs B and C both hardcode their horizons with no config knob; that precedent holds. Using leg C's own floor guarantees the enrichment never fires on a change leg C stayed silent about.
- **No hardcoded fixture ids.** `tests/test_board_checks.sh` was at a max of 248 and is moving. Every new fixture id in this plan is allocated above the current maximum — re-check the file's actual maximum before writing and shift the whole block up if it has moved.
- **Best-effort `gh` posture is verbatim `detect_merged`'s:** any gh/network/parse failure ⇒ `echo "sweep-skipped <reason>"; return 0`. It never aborts the pass.
- Full suite command: `scripts/run-tests.sh`. Single file: `bash tests/test_board_checks.sh`.

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `scripts/board-checks.sh` | git-only health checks over change files | Modify: hoist `ar_pr`, add leg D, fix stale preamble comment |
| `scripts/board-checks.md` | `board-checks.sh`'s authoritative contract | Modify: document leg D, rewrite `## Not covered`, repoint the git-only deferral |
| `scripts/docket-status.sh` | the status orchestrator (sweep, health, board, digest) | Modify: add `detect_orphan_pr`, call it on the full path |
| `scripts/docket-status.md` | `docket-status.sh`'s authoritative contract | Modify: document the enrichment leg |
| `tests/test_board_checks.sh` | hermetic suite for `board-checks.sh` | Modify: leg-D fixtures + mutation arm |
| `tests/test_docket_status.sh` | hermetic suite for `docket-status.sh` | Modify: `detect_orphan_pr` fixtures with a stubbed `gh` |

Task 1 owns leg D end-to-end (code + contract section + tests). Task 2 owns the GitHub leg end-to-end. Task 3 owns the two cross-script doc repairs, which can only be written truthfully once both legs exist.

---

### Task 1: Leg D — `pr:` recorded, `status:` never advanced

**Files:**
- Modify: `scripts/board-checks.sh:398-402` (the stale `aborted-run` preamble comment) and `scripts/board-checks.sh:465` (hoist the `pr:` read, add leg D above leg C's `if`)
- Modify: `scripts/board-checks.md:200-203` (leg count prose) and the leg list after leg C's block
- Test: `tests/test_board_checks.sh`

**Interfaces:**
- Consumes: `fm_field FILE KEY` (anchored frontmatter read, from `scripts/lib/docket-frontmatter.sh`); `emit CHECK_ID CHANGE_ID MESSAGE`; the enclosing `if [ "$status" = "in-progress" ]` block; `$id`, `$f`.
- Produces: shell variable `ar_pr` — the anchored `pr:` value for the current change, read exactly once and consumed by both leg D and leg C. Leg C's own `[ -z "$(fm_field "$f" pr)" ]` becomes `[ -z "$ar_pr" ]`.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_board_checks.sh`, immediately after the existing leg-C block (before the mutation section that follows it). **First run `grep -oE '^id: [0-9]+' tests/test_board_checks.sh | sort -t' ' -k2 -n | tail -1`** and, if the maximum is at or above 260, shift every id below up by the same offset.

```bash
# ---------------- aborted-run, leg D: pr: recorded, status: never advanced (change 0219) ----------------
# The Step 7 seam. docket-implement-next writes `status: implemented` and `pr:` in ONE field-write,
# and no script under scripts/ writes pr: — so a manifest carrying pr: while still in-progress is an
# anomaly BY CONSTRUCTION. Time-free for that reason, exactly like leg A: there is no healthy window
# to wait out, so an idle floor would only delay a finding that is already certain.
#
# Leg D is the pr:-NON-empty arm of the same hoisted read whose pr:-empty arm is leg C, so the two
# are mutually exclusive by construction and one fixture can never produce both.

# --- RED: pr: set while status: is still in-progress ---
read -r AR_D1 _ < <(new_repo)
cat > "$AR_D1/docs/changes/active/0260-pr-recorded-status-stale.md" <<'EOF'
---
id: 260
slug: pr-recorded-status-stale
title: PR recorded, status never advanced
status: in-progress
priority: medium
depends_on: []
branch: feat/ar-d1
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr: 314
---
EOF
ard1out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR_D1/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg D fires when pr: is set but status: is still in-progress (id 260)" \
  'has_finding "$ard1out" aborted-run 260'
ard1line="$(grep -E "$(printf "^aborted-run\t260\t")" <<<"$ard1out")"
assert "the leg-D finding names the recorded PR and the missing status write (id 260)" \
  'grep -qF -- "pr: records 314" <<<"$ard1line" && grep -qF -- "status: is still in-progress" <<<"$ard1line"'
assert "the leg-D remedy is a VERIFICATION, and names the status it should reach (id 260)" \
  'grep -qF -- "verify the PR and set status: implemented" <<<"$ard1line"'
# Leg D must not borrow leg C's or leg B's exclusive clause: board-checks.md pins `pr: is unset` as
# leg-C-exclusive and `mid-step` as leg-B-exclusive so a message-shape assert can tell legs apart.
assert "leg D does not reuse leg C's exclusive 'pr: is unset' clause (id 260)" \
  '! grep -qF -- "pr: is unset" <<<"$ard1line"'
assert "leg D does not reuse leg B's exclusive 'mid-step' phrasing (id 260)" \
  '! grep -qF -- "mid-step" <<<"$ard1line"'
assert "aborted-run fires exactly ONCE for id 260 (leg B stays silent on an absent claimed_at)" \
  '[ "$(grep -cE "$(printf "^aborted-run\t260\t")" <<<"$ard1out")" = 1 ]'

# --- GREEN: pr: set AND status: implemented — the delivered change, the whole point of the gate ---
read -r AR_D2 _ < <(new_repo)
cat > "$AR_D2/docs/changes/active/0261-pr-recorded-delivered.md" <<'EOF'
---
id: 261
slug: pr-recorded-delivered
title: PR recorded and status advanced
status: implemented
priority: medium
depends_on: []
branch: feat/ar-d2
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr: 315
---
EOF
ard2out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR_D2/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT when pr: is set and status: is implemented (id 261, status gate)" \
  '! has_finding "$ard2out" aborted-run 261'

# --- GREEN: pr: empty — leg C's domain, and leg D must not poach it ---
read -r AR_D3 _ < <(new_repo)
cat > "$AR_D3/docs/changes/active/0262-pr-empty.md" <<'EOF'
---
id: 262
slug: pr-empty
title: In-progress with no PR recorded and no branch
status: in-progress
priority: medium
depends_on: []
branch:
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
---
EOF
ard3out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR_D3/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT for an in-progress change with an empty pr: and no branch (id 262)" \
  '! has_finding "$ard3out" aborted-run 262'

# --- RED: the ANCHORED read. Frontmatter OMITS pr: entirely while the body opens a `pr:` line.
# An unanchored read returns the body prose and fires leg D on a change that has no PR at all —
# the ADR-0057 shape, here producing a FALSE POSITIVE (the mirror of leg A's false negative).
# The natural fixture (a file that HAS pr:) passes under both implementations, so this
# absent-key one is the only fixture that discriminates. Paired with mutation N below: the fixture
# is inert without a mutation that reaches it, and the mutation is inert without this fixture.
read -r AR_D4 _ < <(new_repo)
cat > "$AR_D4/docs/changes/active/0263-pr-prose-only.md" <<'EOF'
---
id: 263
slug: pr-prose-only
title: Body prose mentions pr but frontmatter omits it
status: in-progress
priority: medium
depends_on: []
branch:
plan: docs/superpowers/plans/2026-06-01-present.md
results:
---

## Notes
pr: 999 was never opened for this change
EOF
ard4out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR_D4/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg D SILENT when only body prose mentions pr: (id 263, anchored read)" \
  '! has_finding "$ard4out" aborted-run 263'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_board_checks.sh`
Expected: FAIL — the two leg-D RED asserts on id 260 fail (no finding is emitted; `ard1line` is empty so the message asserts fail too). The three GREEN asserts (261, 262, 263) pass vacuously at this point, which is exactly why 263 needs its mutation arm in Step 5.

- [ ] **Step 3: Hoist the `pr:` read and add leg D**

In `scripts/board-checks.sh`, find leg C's opening line (currently line 465):

```bash
    if [ -z "$(fm_field "$f" pr)" ] && [ -n "$ar_ref" ]; then
```

Replace that single line with the hoist, leg D, and leg C's rewritten gate:

```bash
    # Leg D — THE STEP 7 SEAM: pr: recorded, status: never advanced (change 0219).
    # docket-implement-next writes `status: implemented` and `pr:` in ONE field-write, and no script
    # under scripts/ writes pr:. A manifest showing pr: set while status: is still in-progress is
    # therefore an anomaly BY CONSTRUCTION, not a run in flight — which is why this leg is TIME-FREE
    # with no idle floor. Leg A is the precedent and the reasoning is identical: there is no healthy
    # window to wait out, so a floor would only delay a finding that is already certain. The other
    # three legs are all blind here: leg A finds no incoherence (plan: and results: are both recorded
    # by then), leg C SHORT-CIRCUITS on a non-empty pr: by deliberate design, and leg B catches it
    # only at 12h — the same lag change 0211 exists to close.
    #
    # KNOWN RESIDUAL, and it is narrower than it looks: this script reads change files off the
    # FILESYSTEM, not out of a git blob. Combined with the single-stroke field-write, leg D's honest
    # yield is uncommitted partial edits in the shared .docket worktree, plus non-compliant drivers
    # that write the two fields separately. It is worth having as a cheap, additive completeness
    # guarantee over the Step 7 seam, NOT because it is a frequent signature. No idle floor is
    # constructible for an uncommitted edit, so no floor is correct here — but for that reason, not
    # for the reason a first draft reaches for.
    #
    # The read is HOISTED and shared with leg C below, whose gate is its exact complement. This path
    # is cost-sensitive (change 0176) and a second read of the same field here is a real regression.
    ar_pr="$(fm_field "$f" pr)"
    if [ -n "$ar_pr" ]; then
      emit aborted-run "$id" "pr: records $ar_pr but status: is still in-progress — the run stopped before its final status write; verify the PR and set status: implemented"
    fi

    if [ -z "$ar_pr" ] && [ -n "$ar_ref" ]; then
```

- [ ] **Step 4: Fix the stale preamble comment**

Still in `scripts/board-checks.sh`, the `aborted-run` preamble (line ~401) reads:

```
  # so the oracle has to be external and mechanical. Two INDEPENDENT legs; either emits, and both
  # can emit on one change (they describe different evidence, not two views of one).
```

Both halves have been stale since change 0211 made it three legs, and this change makes it four. Replace with the phrasing `board-checks.md` already uses:

```
  # so the oracle has to be external and mechanical. FOUR INDEPENDENT legs; any emits, and more than
  # one may emit on one change (they describe different evidence, not several views of one).
```

- [ ] **Step 5: Add the anchoring mutation arm**

Fixture 263 is inert without a mutation that reaches it. Append to the mutation section of `tests/test_board_checks.sh`, after the existing mutation M block, using the established `mreseed`/fresh-copy discipline. Note this mutation needs its OWN fixture repo (the shared `$MUT` tree carries no leg-D fixtures), so it reseeds a copy and runs it against `$AR_D4`:

```bash
# Mutation N (change 0219) — unanchor leg D's pr: read. Fixture 263 omits pr: from frontmatter while
# its body opens a `pr:` line, so an unanchored read returns the prose and leg D MISFIRES on a change
# that has no PR at all. This is the ADR-0057 shape in its false-POSITIVE direction; the fixture and
# the mutation only discriminate as a pair.
mreseed
mn_before="$(grep -cF -- 'ar_pr="$(fm_field "$f" pr)"' "$MUTSCRIPT")"
perl -pi -e 's/ar_pr="\$\(fm_field "\$f" pr\)"/ar_pr="\$(field "\$f" pr)"/' "$MUTSCRIPT"
mn_after="$(grep -cF -- 'ar_pr="$(fm_field "$f" pr)"' "$MUTSCRIPT")"
mnout="$(NOW=$NOW_EPOCH bash "$MUTSCRIPT" --changes-dir "$AR_D4/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "mutation N landed: leg D's pr: read is unanchored (fm_field count 1 -> 0)" \
  '[ "$mn_before" = 1 ] && [ "$mn_after" = 0 ]'
assert "mutation N landed: the mutated copy is still valid bash" 'bash -n "$MUTSCRIPT"'
assert "mutation N (unanchor leg D's pr: read): body prose 263 MISFIRES — proves the anchoring" \
  'has_finding "$mnout" aborted-run 263'
assert "mutation N: the misfire echoes the BODY value, not a frontmatter one" \
  'grep -qF -- "pr: records 999" <<<"$mnout"'
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `bash tests/test_board_checks.sh`
Expected: PASS — all leg-D asserts and mutation N green, and every pre-existing leg-A/B/C assert still green. Leg C's fixtures must be **unchanged byte-for-byte in outcome**: the hoist is a pure refactor of its gate, so any leg-C regression here means the hoist changed behaviour.

- [ ] **Step 7: Document leg D in `scripts/board-checks.md`**

In `scripts/board-checks.md`, change the leg count in the `aborted-run` preamble (line ~202-203) from:

```
Gated on `status: in-progress`. **Three**
independent legs; any emits, and more than one may emit on one change.
```

to:

```
Gated on `status: in-progress`. **Four**
independent legs; any emits, and more than one may emit on one change.
```

Then append a new leg-D bullet at the end of the leg list, after leg C's whole block (i.e. after its `**Not covered:**` paragraph — Task 3 rewrites that paragraph, and the two edits are adjacent but independent):

```markdown
- **Leg D — the Step 7 seam: `pr:` recorded, `status:` never advanced (time-free, change 0219).**
  `pr:` is non-empty while `status:` is still `in-progress`. `docket-implement-next` writes
  `status: implemented` **and** `pr:` in a single field-write and no script under `scripts/` writes
  `pr:`, so this state is an anomaly **by construction** rather than a run in flight — which is why
  the leg carries **no idle floor**. Leg A is the precedent, time-free for the same reason. The
  other three legs are all blind to it: leg A finds no incoherence (`plan:` and `results:` are both
  recorded by then), leg C **short-circuits on a non-empty `pr:`** by deliberate design, and leg B
  catches it only at 12h — the same lag change 0211 exists to close.

  The message names the recorded PR, the field that was never written, and a remedy that stays a
  verification: `pr: records <n> but status: is still in-progress — the run stopped before its final
  status write; verify the PR and set status: implemented`. It deliberately borrows neither leg C's
  exclusive `pr: is unset` clause nor leg B's exclusive `mid-step`, so message-shape asserts keep
  telling the four legs apart.

  **Cost: zero git invocations.** Leg D's predicate is a pure frontmatter test, and it shares its
  `pr:` read with leg C — one anchored read (`ar_pr`) serves both, since the two gates are exact
  complements. Adding a second read on this path would be a real regression (change 0176).

  **Known residual, and it is narrower than it looks.** This script reads change files off the
  filesystem, not out of a git blob. Combined with the single-stroke field-write, leg D's honest
  yield is *uncommitted partial edits in the shared `.docket` worktree, plus non-compliant drivers*
  that write the two fields separately — not a routine abort signature. It is worth having as a
  cheap, additive completeness guarantee over the Step 7 seam, not because it is frequent. No idle
  floor is constructible for an uncommitted edit, so no floor is correct here.
```

- [ ] **Step 8: Commit**

```bash
git add scripts/board-checks.sh scripts/board-checks.md tests/test_board_checks.sh
git commit -m "fix(0219): add aborted-run leg D — pr: recorded, status: never advanced"
```

---

### Task 2: The GitHub enrichment leg in `docket-status.sh`

**Files:**
- Modify: `scripts/docket-status.sh` — new `detect_orphan_pr` immediately after `detect_merged` (which ends at line ~562), plus one call site in `main()` (line ~955, after the health output is printed)
- Modify: `scripts/docket-status.md` — the new leg's contract
- Test: `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: `docket_metadata_worktree` (absolute path, change 0075); `field FILE KEY`; `int_field FILE KEY`; the `GIT`/`GH` mock seams; `$CHANGES_DIR`, `$REPO_FLAG`.
- Produces: `detect_orphan_pr` — takes no arguments, prints zero or more lines to stdout and always returns 0. Line shapes: `check aborted-run <id> <message>` (matching `health_checks`' own render, so consumers see one vocabulary) and `sweep-skipped <reason>` on any gh/network/parse failure. Also produces `ORPHAN_PR_IDLE_SECS`, a script-level constant.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_docket_status.sh`, after the existing `detect_merged` block (which ends around line 775). It follows that block's exact idiom: a fixture change-file tree, a stubbed `gh` script on the `GH` seam, and `bash -c '. "$SCRIPT"; detect_orphan_pr'` sourcing the script to call the function directly.

```bash
# ============ detect_orphan_pr — the GitHub enrichment leg (change 0219) ============
# Resolves the ambiguity board-checks.sh leg C leaves behind. Leg C is git-only by contract and can
# only say "verify the PR exists"; two very different situations produce that one finding — a PR that
# exists and merely went unrecorded (remedy: record it) versus a run that died before creating one
# (remedy: create it). This leg asks GitHub which one it is. The GATE is leg C's own, so the two
# findings always agree and the enrichment can never fire on a change leg C stayed silent about.
orphan_dir="$tmp/orphan-pr"
mkdir -p "$orphan_dir/docs/changes/active"
git -C "$orphan_dir" init -q 2>/dev/null || { git init -q "$orphan_dir"; }
git -C "$orphan_dir" commit -q --allow-empty -m base 2>/dev/null

# ORPHAN_NOW is the clock every case below is measured against; the branch tips are dated relative
# to it. 3h > the 2h floor, 1h < it — the floor is the axis under test, so it must never be an
# accident of the wall clock.
ORPHAN_NOW=1750000000
orphan_branch(){  # orphan_branch BRANCH AGE_SECS
  local ob_when="@$(( ORPHAN_NOW - $2 ))"
  git -C "$orphan_dir" checkout -q -b "$1" 2>/dev/null
  GIT_AUTHOR_DATE="$ob_when" GIT_COMMITTER_DATE="$ob_when" \
    git -C "$orphan_dir" commit -q --allow-empty -m "on $1"
  git -C "$orphan_dir" checkout -q - 2>/dev/null
}
orphan_branch feat/has-pr 10800
orphan_branch feat/no-pr   10800
orphan_branch feat/fresh    3600

cat > "$orphan_dir/docs/changes/active/0270-has-pr.md" <<'EOF'
---
id: 270
slug: has-pr
title: Pushed, PR open on GitHub, pr never recorded
status: in-progress
priority: high
depends_on: []
branch: feat/has-pr
pr:
---
EOF
cat > "$orphan_dir/docs/changes/active/0271-no-pr.md" <<'EOF'
---
id: 271
slug: no-pr
title: Pushed, no PR was ever opened
status: in-progress
priority: high
depends_on: []
branch: feat/no-pr
pr:
---
EOF
cat > "$orphan_dir/docs/changes/active/0272-fresh.md" <<'EOF'
---
id: 272
slug: fresh
title: Branch tip is fresher than the idle floor
status: in-progress
priority: high
depends_on: []
branch: feat/fresh
pr:
---
EOF
cat > "$orphan_dir/docs/changes/active/0273-recorded.md" <<'EOF'
---
id: 273
slug: recorded
title: pr already recorded — leg D's domain, not this one
status: in-progress
priority: high
depends_on: []
branch: feat/has-pr
pr: 500
---
EOF

cat > "$tmp/gh-orphan-ok.sh" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = repo ] && [ "$2" = view ]; then echo "x/y"; exit 0; fi
if [ "$1" = pr ] && [ "$2" = list ]; then
  # --head <branch> is the third/fourth arg pair; find it.
  head=""
  while [ $# -gt 0 ]; do [ "$1" = --head ] && head="$2"; shift; done
  case "$head" in
    feat/has-pr) echo '[{"number":777}]' ;;
    *)           echo '[]' ;;
  esac
  exit 0
fi
echo "gh-orphan-ok: unexpected args: $*" >&2
exit 1
EOF
chmod +x "$tmp/gh-orphan-ok.sh"

orphan_out="$( cd "$orphan_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes NOW=$ORPHAN_NOW GH="$tmp/gh-orphan-ok.sh" \
  bash -c '. "'"$SCRIPT"'"; detect_orphan_pr' )"

assert "detect_orphan_pr reports the OPEN PR it found, by number (id 270)" \
  'grep -q "^check aborted-run 270 " <<<"$orphan_out" && grep -qF "PR #777 is open" <<<"$orphan_out"'
assert "the found-PR remedy is to RECORD it (id 270)" \
  'grep -E "^check aborted-run 270 " <<<"$orphan_out" | grep -qF -- "pr: is unset — record it"'
assert "detect_orphan_pr reports the NO-PR case distinctly (id 271)" \
  'grep -q "^check aborted-run 271 " <<<"$orphan_out" && grep -E "^check aborted-run 271 " <<<"$orphan_out" | grep -qF "no PR on GitHub"'
assert "the two outcomes are DISTINGUISHABLE — 271 never claims a PR is open" \
  '! grep -E "^check aborted-run 271 " <<<"$orphan_out" | grep -qF "is open"'
assert "detect_orphan_pr SILENT below the 2h idle floor (id 272)" \
  '! grep -q "^check aborted-run 272 " <<<"$orphan_out"'
assert "detect_orphan_pr SILENT when pr: is already recorded (id 273 — leg D's domain)" \
  '! grep -q "^check aborted-run 273 " <<<"$orphan_out"'
assert "detect_orphan_pr emits no sweep-skipped when gh works" \
  '! grep -q "^sweep-skipped" <<<"$orphan_out"'

# Best-effort posture, verbatim detect_merged's: a failing gh must go QUIET, never loud and never
# fatal. This is what keeps board-checks.sh's offline guarantee intact — the offline-safe check keeps
# emitting leg C's finding, and only the enrichment stops.
cat > "$tmp/gh-orphan-fail.sh" <<'EOF'
#!/usr/bin/env bash
echo "gh-orphan-fail: boom" >&2
exit 1
EOF
chmod +x "$tmp/gh-orphan-fail.sh"

orphan_fail_out="$( cd "$orphan_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes NOW=$ORPHAN_NOW GH="$tmp/gh-orphan-fail.sh" \
  bash -c '. "'"$SCRIPT"'"; detect_orphan_pr' )"
orphan_fail_rc=$?
assert "detect_orphan_pr with a failing gh reports sweep-skipped" \
  'grep -q "^sweep-skipped" <<<"$orphan_fail_out"'
assert "detect_orphan_pr with a failing gh returns success (best-effort)" \
  '[ $orphan_fail_rc -eq 0 ]'
assert "detect_orphan_pr with a failing gh emits NO findings at all" \
  '! grep -q "^check aborted-run" <<<"$orphan_fail_out"'

# An ABSENT gh binary is a different failure path from a gh that runs and exits 1 — and it is the
# common one offline. A mock that omits the tool routes everything through the degrade branch, so
# both arms must be pinned separately.
orphan_absent_out="$( cd "$orphan_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes NOW=$ORPHAN_NOW GH="$tmp/definitely-not-a-real-gh" \
  bash -c '. "'"$SCRIPT"'"; detect_orphan_pr' )"
assert "detect_orphan_pr with an ABSENT gh reports sweep-skipped and no findings" \
  'grep -q "^sweep-skipped" <<<"$orphan_absent_out" && ! grep -q "^check aborted-run" <<<"$orphan_absent_out"'

# Malformed JSON is the third failure mode: gh exits 0 and prints something jq cannot parse.
cat > "$tmp/gh-orphan-garbage.sh" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = repo ] && [ "$2" = view ]; then echo "x/y"; exit 0; fi
echo 'not json at all'
exit 0
EOF
chmod +x "$tmp/gh-orphan-garbage.sh"
orphan_garbage_out="$( cd "$orphan_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes NOW=$ORPHAN_NOW GH="$tmp/gh-orphan-garbage.sh" \
  bash -c '. "'"$SCRIPT"'"; detect_orphan_pr' )"
assert "detect_orphan_pr treats unparseable gh output as a skip, not a finding" \
  '! grep -q "^check aborted-run" <<<"$orphan_garbage_out"'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_status.sh`
Expected: FAIL with `detect_orphan_pr: command not found` on every case.

- [ ] **Step 3: Add the `NOW` seam and the idle-floor constant**

`docket-status.sh` has no staleness clock today. Mirror `board-checks.sh`'s seam exactly (`board-checks.sh:29`). Add beside the existing `GH="${GH:-gh}"` at `scripts/docket-status.sh:35`:

```bash
NOW="${NOW:-$(date +%s)}"
```

and extend the mock-seam comment at line 31 to name it:

```bash
# Mock seams: GIT="${GIT:-git}", GH="${GH:-gh}", NOW="${NOW:-$(date +%s)}" (staleness clock),
# CONFIG_EXPORT_CMD (config export override).
```

Then, immediately above `detect_orphan_pr` (Step 4), add the constant:

```bash
# Branch-idle floor for the GitHub enrichment leg (change 0219). This is leg C's OWN floor
# (board-checks.sh's ABORTED_RUN_IDLE_SECS), reused rather than re-tuned, and the reuse is
# load-bearing: it guarantees the enrichment never fires on a change leg C stayed silent about, so
# the git-only finding and its GitHub resolution always agree. Hardcoded with no config knob — the
# same precedent ABORTED_RUN_STALE_SECS and ABORTED_RUN_IDLE_SECS set; a second magic number would
# need its own justification and this one has none to offer. Kept in sync BY VALUE, not by import:
# the two scripts share no library, and board-checks.sh must stay independently runnable.
ORPHAN_PR_IDLE_SECS=$(( 2 * 3600 ))
```

- [ ] **Step 4: Write `detect_orphan_pr`**

Insert immediately after `detect_merged`'s closing brace in `scripts/docket-status.sh`:

```bash
# detect_orphan_pr — the GitHub enrichment leg for board-checks.sh's aborted-run leg C (change 0219).
#
# Leg C fires on "branch has commits, pr: is unset, tip idle > 2h" and can only tell a human to "verify
# the PR exists": board-checks.sh is git-only BY CONTRACT and shells no gh. Two very different
# situations produce that one finding — a PR that exists and merely went unrecorded (remedy: record
# it) versus a run that died before opening one (remedy: open it) — and today only a manual check
# distinguishes them. This leg asks GitHub which it is. It adds NO detection; it RESOLVES an
# ambiguity, which is why it lives here (where gh already lives) rather than widening leg C.
#
# The gate is leg C's own, so the two findings always agree and this can never fire on a change leg C
# stayed silent about. Advisory like every aborted-run leg: it flips no status, releases no claim, and
# writes no file.
#
# Best-effort, VERBATIM detect_merged's posture: any gh/network/parse failure emits
# "sweep-skipped <reason>" and returns 0. That is what keeps board-checks.sh's offline guarantee
# intact — offline, the git-only check keeps emitting leg C's finding and only the enrichment goes
# quiet. Prints "check aborted-run <id> <message>" lines, matching health_checks' own render so
# consumers read one vocabulary.
detect_orphan_pr(){
  local mw
  mw="$(docket_metadata_worktree)"   # ABSOLUTE (change 0075) — see board_pass.
  local cd_dir="$mw/$CHANGES_DIR"

  local -a files
  mapfile -t files < <(find "$cd_dir/active" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
  [ ${#files[@]} -gt 0 ] || return 0

  # Collect candidates FIRST, and return before touching gh when there are none. A repo with no
  # candidate must pay nothing — not a `gh repo view`, not a subprocess.
  local -a ids=() branches=() idles=()
  local f id status pr slug branch tip
  for f in "${files[@]}"; do
    status="$(field "$f" status)"
    [ "$status" = in-progress ] || continue
    pr="$(field "$f" pr)"
    [ -z "$pr" ] || continue          # pr: recorded is leg D's domain, never this one
    id="$(int_field "$f" id)"
    [ -n "$id" ] || continue
    slug="$(field "$f" slug)"
    branch="$(field "$f" branch)"
    [ -n "$branch" ] || branch="feat/$slug"
    # Tip age off the LOCAL ref, then the remote-tracking one — branch_ref's order in
    # board-checks.sh. An unresolvable branch yields empty stdout and is silence, never a finding:
    # no positive evidence is the posture every aborted-run leg takes.
    tip="$("$GIT" -C "$cd_dir" log -1 --format=%ct "$branch" 2>/dev/null)"
    [ -n "$tip" ] || tip="$("$GIT" -C "$cd_dir" log -1 --format=%ct "origin/$branch" 2>/dev/null)"
    [ -n "$tip" ] || continue
    [ "$(( NOW - tip ))" -gt "$ORPHAN_PR_IDLE_SECS" ] || continue
    ids+=("$id"); branches+=("$branch"); idles+=("$(( (NOW - tip) / 3600 ))")
  done
  [ ${#ids[@]} -gt 0 ] || return 0

  local repo="${REPO_FLAG:-}"
  if [ -z "$repo" ]; then
    repo="$("$GH" repo view --json owner,name -q '(.owner.login)+"/"+(.name)' 2>/dev/null)" \
      || { echo "sweep-skipped gh-unavailable"; return 0; }
  fi
  [ -n "$repo" ] || { echo "sweep-skipped repo-unresolved"; return 0; }

  local i br pl_json pl_num
  for i in "${!ids[@]}"; do
    id="${ids[$i]}"; br="${branches[$i]}"
    pl_json="$("$GH" pr list --head "$br" --state open --json number 2>/dev/null)" || {
      echo "sweep-skipped gh-unavailable"
      return 0
    }
    # A gh that exits 0 and prints something jq cannot parse is a THIRD failure mode, distinct from
    # a non-zero exit and from an absent binary. Treat it as a skip for THIS change and keep going:
    # one unparseable response is not evidence about the others.
    if ! printf '%s' "$pl_json" | jq -e . >/dev/null 2>&1; then
      echo "sweep-skipped gh-unparseable"
      continue
    fi
    pl_num="$(printf '%s' "$pl_json" | jq -r '.[0].number // empty' 2>/dev/null)"
    # Both messages HEDGE nothing about the PR's existence — unlike leg C, this leg has ASKED, so it
    # states what it found as fact. The remedy stays a bookkeeping act on the manifest, never a
    # push or a merge: acting on the branch would race a run that is merely between commits.
    if [ -n "$pl_num" ]; then
      echo "check aborted-run $id PR #$pl_num is open on $br but pr: is unset — record it"
    else
      echo "check aborted-run $id $br is pushed (last commit ${idles[$i]}h ago) but no PR on GitHub — the run stopped before opening one"
    fi
  done
  return 0
}
```

- [ ] **Step 5: Call it on the full path**

In `main()` (`scripts/docket-status.sh`, currently line ~953-956), the health block reads:

```bash
  local health_out
  health_out="$(health_checks)"
  [ -n "$health_out" ] && printf '%s\n' "$health_out"
  reclaim_pass "$health_out"
```

Insert the enrichment call between the print and `reclaim_pass`:

```bash
  local health_out
  health_out="$(health_checks)"
  [ -n "$health_out" ] && printf '%s\n' "$health_out"
  # Change 0219: the GitHub enrichment for leg C, printed immediately after the git-only findings so
  # a leg-C finding and its resolution read together. Deliberately NOT folded into $health_out:
  # reclaim_pass keys a MUTATING gate on that blob (RECLAIMABLE_LINE_RE), and widening what feeds it
  # with network-derived lines would put a remote service inside a local mutation's trigger. This is
  # advisory output only. FULL PATH ONLY — never under --board-only, which early-exits above and is
  # invoked by many callers as a must-land board write.
  detect_orphan_pr
  reclaim_pass "$health_out"
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `bash tests/test_docket_status.sh`
Expected: PASS — every `detect_orphan_pr` assert green, and every pre-existing `detect_merged`, full-pass, and `reclaim_pass` assert still green. A regression in the full-pass fixtures means the `main()` call site changed what those fixtures print; the enrichment must be silent on them (their in-progress changes have no aged branch).

- [ ] **Step 7: Document the leg in `scripts/docket-status.md`**

Add a subsection in `scripts/docket-status.md` under the section describing the full pass's sweep/health behaviour, immediately after the `detect_merged` / sweep description:

```markdown
### The `aborted-run` GitHub enrichment leg (change 0219)

Full path only — never under `--board-only`. After the git-only health findings are printed and
before the reclaim gate, `detect_orphan_pr` resolves the ambiguity `board-checks.sh`'s `aborted-run`
**leg C** leaves behind.

**Gate — leg C's own, reused rather than re-tuned.** A change under `active/` with
`status: in-progress`, an empty `pr:`, and a branch whose newest commit is older than **2 hours**
(`ORPHAN_PR_IDLE_SECS`, the same value as `board-checks.sh`'s `ABORTED_RUN_IDLE_SECS`, kept in sync
by value — the two scripts share no library and `board-checks.sh` must stay independently runnable).
Reusing the floor guarantees the enrichment never fires on a change leg C stayed silent about, so
the two findings always agree. The branch is `branch:` when set, else `feat/<slug>`; an unresolvable
branch is **silence**, never a finding.

**Two outcomes, two remedies**, both rendered as `check aborted-run <id> <message>` — the same shape
`health_checks` prints, so consumers read one vocabulary:

- an open PR exists on the branch → `PR #<n> is open on <branch> but pr: is unset — record it`
- no open PR exists → `<branch> is pushed (last commit Nh ago) but no PR on GitHub — the run stopped
  before opening one`

Unlike leg C's messages these do **not** hedge about the PR's existence — this leg has asked GitHub,
so it states what it found. The remedy stays a bookkeeping act on the manifest and never a push or a
merge: acting on the branch would race a run that is merely between commits. Advisory like every
`aborted-run` leg — it flips no status, releases no claim, and writes no file.

**Best-effort, verbatim `detect_merged`'s posture.** Any gh/network/parse failure emits
`sweep-skipped <reason>` and returns 0; it never aborts the pass. A repo with no candidate change
pays nothing — not even a `gh repo view`. This is what keeps `board-checks.sh`'s offline guarantee
intact: offline, the git-only check keeps emitting leg C's finding and only the enrichment goes
quiet.

**Deliberately not folded into `health_checks`' output blob.** `reclaim_pass` keys a *mutating* gate
on that blob (`RECLAIMABLE_LINE_RE`); widening what feeds it with network-derived lines would put a
remote service inside a local mutation's trigger.
```

- [ ] **Step 8: Commit**

```bash
git add scripts/docket-status.sh scripts/docket-status.md tests/test_docket_status.sh
git commit -m "feat(0219): resolve leg C's pr-unset ambiguity with a GitHub enrichment leg"
```

---

### Task 3: Repair the cross-script documentation

Both repairs assert facts that only became true once Tasks 1 and 2 landed, which is why they are a
separate task rather than folded in.

**Files:**
- Modify: `scripts/board-checks.md:269-271` (the `## Not covered` paragraph, inside leg C's block)

**Interfaces:**
- Consumes: leg D (Task 1) and `detect_orphan_pr` (Task 2) both existing and documented.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Rewrite `Not covered` — rewrite, never delete**

This paragraph is the only written record of the gap; deleting it erases the reasoning. The current
text (`scripts/board-checks.md:269-271`) reads:

```markdown
  **Not covered:** the run that opens the PR, writes `pr:`, and dies before `status: implemented`.
  Leg C's `pr:`-empty gate makes it invisible and leg B catches it at 12h; its evidence is a
  manifest/GitHub comparison, and this script is git-only by contract.
```

Two claims in it are now false: that state is covered (leg D, Task 1), and its evidence is *not* a
manifest/GitHub comparison — it is manifest-internal and detectable git-only, which is exactly what
made leg D possible. Replace with:

```markdown
  **Now covered, by leg D (change 0219):** the run that opens the PR, writes `pr:`, and dies before
  `status: implemented`. Leg C's `pr:`-empty gate still makes it invisible *here* — that gate is
  deliberate — but the state is manifest-internal and detectable git-only, so it needed no widening
  of leg C and no relaxation of this script's contract. Leg D is its complement on the same hoisted
  `pr:` read.

  **The surviving residual is offline, or `gh` unavailable.** Leg C's `pr:`-unset finding is
  *ambiguous by construction*: a PR that exists and merely went unrecorded, and a run that died
  before opening one, produce the identical evidence in git. Resolving them requires asking GitHub,
  which this script will not do — so the resolution lives in `docket-status.sh`'s
  `detect_orphan_pr` (change 0219), beside `detect_merged` where `gh` already lives. That leg reuses
  this leg's own 2h floor, so the two findings always agree. When `gh` is unavailable it emits
  `sweep-skipped` and goes quiet; leg C's finding still fires, and a human still resolves the
  ambiguity by hand. **That degradation is the design, not a defect:** the offline-safe check stays
  offline-safe.
```

- [ ] **Step 2: Verify no stale cross-references remain**

Run:

```bash
grep -n "git-only by contract\|Two INDEPENDENT\|Three\s*$\|three legs" scripts/board-checks.md scripts/board-checks.sh
```

Expected: no hit that still describes `aborted-run` as having two or three legs, and no hit that
cites the git-only contract as the reason the Step 7 seam is *unresolvable* (as opposed to
unresolvable *in this script*, which remains true and is stated above). Fix any that survive.

- [ ] **Step 3: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: PASS — every test file green. Docs carry guard tests in this repo (correspondence guards
between a script and its `.md`), so a doc edit can legitimately redden a suite; a failure here is a
real finding, not noise.

- [ ] **Step 4: Commit**

```bash
git add scripts/board-checks.md
git commit -m "docs(0219): repair board-checks.md's Not covered paragraph for legs D and the gh enrichment"
```

---

## Self-Review

**1. Spec coverage.**

| Spec requirement | Task |
|---|---|
| Leg D in `board-checks.sh`, fires on `in-progress` + non-empty `pr:` | 1, Step 3 |
| Git-only; script contract untouched | 1 — leg D's predicate is a pure frontmatter test, zero git calls |
| Time-free, no idle floor, reasoning in the leg's comment | 1, Step 3 (comment) + Step 7 (contract) |
| Single hoisted `pr:` read shared with leg C (change 0176 cost) | 1, Step 3 — `ar_pr` hoisted, leg C's gate rewritten to consume it |
| Leg D message shape | 1, Step 3; asserted in Step 1 |
| Known residual stated in the leg's comment | 1, Step 3 + Step 7 |
| GitHub enrichment leg in `docket-status.sh` beside `detect_merged` | 2, Step 4 |
| Gate = leg C's (in-progress, `pr:` empty, 2h floor), floor reused not re-tuned | 2, Steps 3–4 |
| Two outcomes, two remedies | 2, Step 4; asserted in Step 1 |
| `sweep-skipped <reason>` + return 0 on any gh/network/parse failure | 2, Step 4; three failure arms asserted in Step 1 |
| Advisory — no status flip, no claim release, no file write | 2, Step 4 — the function only echoes |
| `board-checks.md` `## Not covered` rewritten, never deleted | 3, Step 1 |
| Its git-only deferral repointed at `docket-status.sh` | 3, Step 1 |
| `docket-status.md` contract for the new leg | 2, Step 7 |
| Stale "Two INDEPENDENT legs" preamble comment fixed, both halves | 1, Step 4; re-verified in 3, Step 2 |
| Test: leg D fires / silent when implemented / silent when `pr:` empty | 1, Step 1 (ids 260, 261, 262) |
| Test: leg C's existing behaviour unchanged by the hoist | 1, Step 6 — the whole pre-existing leg-C block must stay green |
| Test: stub `gh`, both outcomes, failing/absent `gh` ⇒ `sweep-skipped` + exit 0 | 2, Step 1 |
| No hardcoded fixture ids | Global Constraints + Task 1 Step 1's re-check instruction |

**2. Placeholder scan.** No `TBD`, no "add error handling", no "similar to Task N", no step without
its literal content. Every code step carries the exact text to insert and the exact line to replace.

**3. Type consistency.** `ar_pr` is introduced in Task 1 Step 3 and consumed by both leg D and leg
C's rewritten gate in that same step — no cross-task signature. `ORPHAN_PR_IDLE_SECS` is defined in
Task 2 Step 3 and read in Step 4. `detect_orphan_pr` takes no arguments, prints
`check aborted-run <id> <message>` / `sweep-skipped <reason>`, and always returns 0 — one contract,
used identically at its call site (Task 2 Step 5), in its tests (Step 1), and in its docs (Step 7).
The `NOW` seam added in Task 2 Step 3 matches `board-checks.sh:29`'s spelling exactly, so both
suites drive their clocks the same way.

**One gap found and closed during review:** the original task split put the `board-checks.md`
`## Not covered` rewrite in Task 1, where it would have had to claim `detect_orphan_pr` exists
before Task 2 wrote it. Moved to Task 3.
