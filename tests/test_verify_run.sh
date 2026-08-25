#!/usr/bin/env bash
# tests/test_verify_run.sh — run: bash tests/test_verify_run.sh
# Hermetic: every case builds a sandbox repo with its own changes dir and passes
# --changes-dir, so nothing reads the developer's real docket state.
set -uo pipefail
unset XDG_CONFIG_HOME
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VR="$ROOT/scripts/verify-run.sh"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# --- fixture ------------------------------------------------------------------
make_sbx(){   # sets SBX (repo root) and CH (changes dir with active/ + archive/)
  SBX="$(mktemp -d)"; SBX="$(cd "$SBX" && pwd -P)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  ( cd "$SBX" && git commit --allow-empty -qm init )
  CH="$SBX/docs/changes"; mkdir -p "$CH/active" "$CH/archive"
}

# write_change ID STATUS BRANCH PR [BODY]
write_change(){
  local id="$1" status="$2" branch="$3" pr="$4" body="${5:-}"
  printf -v padded '%04d' "$id"
  cat > "$CH/active/$padded-slug$id.md" <<EOF
---
id: $id
slug: slug$id
title: "Change $id"
status: $status
branch: $branch
pr: $pr
---

## Why

body text
$body
EOF
}

# push_branch NAME — make refs/remotes/origin/NAME resolve in SBX
push_branch(){ git -C "$SBX" update-ref "refs/remotes/origin/$1" HEAD; }

vr(){ bash "$VR" "$@" --changes-dir "$CH" 2>/dev/null; }

# --- run-complete: all three conjuncts hold -----------------------------------
make_sbx
write_change 10 implemented feat/slug10 "https://github.com/o/r/pull/7"
push_branch feat/slug10
out="$( cd "$SBX" && vr 10 )"; rc=$?
assert "complete: verdict line is run-complete" '[ "$out" = "run-complete 10" ]'
assert "complete: exit 0" '[ "$rc" = "0" ]'

# --- run-incomplete: the 0235 signature (built, never delivered) --------------
write_change 11 in-progress feat/slug11 ""
out="$( cd "$SBX" && vr 11 )"; rc=$?
assert "incomplete: names the change and every unmet conjunct" \
  '[ "$out" = "run-incomplete 11 status pr branch" ]'
assert "incomplete: still exits 0 (a finding is not a script failure)" '[ "$rc" = "0" ]'

# --- run-incomplete: partial — pushed branch, PR recorded, status not advanced -
write_change 12 in-progress feat/slug12 "https://github.com/o/r/pull/8"
push_branch feat/slug12
out="$( cd "$SBX" && vr 12 )"
assert "incomplete: reports ONLY the unmet conjunct" '[ "$out" = "run-incomplete 12 status" ]'

# --- run-halted: the git-written escape ---------------------------------------
write_change 13 in-progress feat/slug13 "" "
## Run halted

### 2026-08-07 — halted

Design fundamentally invalidated; needs a human."
out="$( cd "$SBX" && vr 13 )"
assert "halted: presence of the section produces run-halted" '[ "$out" = "run-halted 13" ]'

# --- run-halted loses to a satisfied postcondition ----------------------------
# `## Run halted` is presence-encoded and its removal is owned by the Step 2 claim, which does
# NOT run on a resume — so a stale section can ride into a completed run. A satisfied
# postcondition is the stronger fact and must win.
write_change 14 implemented feat/slug14 "https://github.com/o/r/pull/9" "
## Run halted

### 2026-08-06 — halted

stale record from an earlier attempt."
push_branch feat/slug14
out="$( cd "$SBX" && vr 14 )"
assert "halted: a satisfied postcondition outranks a stale halt record" '[ "$out" = "run-complete 14" ]'

# --- a prose MENTION of the marker is not the section (whole-line match) -------
# Fixture body carried in a SINGLE-quoted literal: the anchor it quotes has backticks, and a
# multi-line double-quoted block carrying one is the exact 0212 incident shape (0221).
write_change 15 in-progress feat/slug15 "" '
Writing a `## Run halted` section is how a run clears the gate.'
out="$( cd "$SBX" && vr 15 )"
assert "halted: a prose mention does not fire the section" \
  '! grep -q "^run-halted" <<<"$out"'

# --- a DATED heading is not the section either (the producer-instruction trap) -
# `has_section` is `grep -qxF`, so `## Run halted — 2026-08-07` is invisible to the reader and the
# run gate reads a deliberate halt as `run-incomplete`. This fixture is why every producer
# instruction must say the heading is BARE; the prose assert below pins that wording.
write_change 19 in-progress feat/slug19 "" "
## Run halted — 2026-08-07

Design fundamentally invalidated; needs a human."
out="$( cd "$SBX" && vr 19 )"
assert "halted: a DATED heading does not fire the section" \
  '! grep -q "^run-halted" <<<"$out"'
assert "halted: and the dated form degrades to run-incomplete" \
  '[ "$out" = "run-incomplete 19 status pr branch" ]'

# --- run-unclaimed: no live run to verify -------------------------------------
write_change 16 proposed "" ""
out="$( cd "$SBX" && vr 16 )"
assert "unclaimed: a proposed change has no run to verify" '[ "$out" = "run-unclaimed 16" ]'
write_change 17 deferred "" ""
out="$( cd "$SBX" && vr 17 )"
assert "unclaimed: a deferred change too" '[ "$out" = "run-unclaimed 17" ]'
cat > "$CH/archive/2026-08-01-0018-slug18.md" <<'EOF'
---
id: 18
slug: slug18
status: done
---

## Why
EOF
out="$( cd "$SBX" && vr 18 )"
assert "unclaimed: an archived change too" '[ "$out" = "run-unclaimed 18" ]'

# --- stacked-merged is a COMPLETED run, never unclaimed (change 0298) ---------
# A change merged into its stack PARENT carries `stacked-merged`: non-terminal, still in active/,
# with a merged PR and a pushed branch. Its implement-next run genuinely completed, so the claim
# gate must admit the status AND the `status` conjunct must accept it alongside `implemented` —
# otherwise a finished run reports as `run-unclaimed` and the run gate re-dispatches onto it.
write_change 81 stacked-merged feat/slug81 "https://github.com/o/r/pull/16"
push_branch feat/slug81
out="$( cd "$SBX" && vr 81 )"; rc=$?
assert "stacked-merged: a completed run reports run-complete" '[ "$out" = "run-complete 81" ]'
assert "stacked-merged: exit 0" '[ "$rc" = "0" ]'
assert "stacked-merged: is not reported unclaimed" '! grep -qF run-unclaimed <<<"$out"'
# The gate admitting the status is not the conjuncts waiving themselves: with the status conjunct
# met, an undelivered stacked-merged change still names the two it fails. This assert is what
# separates the two edits — a claim-gate-only fix leaves it reporting `status` as well.
write_change 82 stacked-merged "" ""
out="$( cd "$SBX" && vr 82 )"
assert "stacked-merged: the status conjunct is met; the others still apply" \
  '[ "$out" = "run-incomplete 82 pr branch" ]'

# --- ANCHORED READS: one absent-key fixture + one mutation arm per read -------
# frontmatter OMITS the key while the body opens a line with it. An unanchored read returns the
# prose; the anchored read returns empty. The natural fixture (key present) passes under BOTH
# implementations, so these absent-key fixtures are the whole guard.
printf -v p '%04d' 20
cat > "$CH/active/$p-slug20.md" <<'EOF'
---
id: 20
slug: slug20
status: in-progress
branch: feat/slug20
---

## Why

pr: https://example.test/not-a-real-field
EOF
push_branch feat/slug20
out="$( cd "$SBX" && vr 20 )"
assert "anchored pr: body prose opening 'pr:' is NOT read as the field" \
  '[ "$out" = "run-incomplete 20 status pr" ]'

printf -v p '%04d' 21
cat > "$CH/active/$p-slug21.md" <<'EOF'
---
id: 21
slug: slug21
status: in-progress
pr: https://github.com/o/r/pull/11
---

## Why

branch: feat/slug21
EOF
push_branch feat/slug21
out="$( cd "$SBX" && vr 21 )"
assert "anchored branch: body prose opening 'branch:' is NOT read as the field" \
  '[ "$out" = "run-incomplete 21 status branch" ]'

printf -v p '%04d' 22
cat > "$CH/active/$p-slug22.md" <<'EOF'
---
id: 22
slug: slug22
branch: feat/slug22
pr: https://github.com/o/r/pull/12
---

## Why

status: implemented
EOF
push_branch feat/slug22
out="$( cd "$SBX" && vr 22 )"
assert "anchored status: body prose opening 'status:' is NOT read as the field" \
  '[ "$out" = "run-unclaimed 22" ]'

# --- the branch conjunct is about ORIGIN, not a local ref ---------------------
write_change 23 implemented feat/slug23 "https://github.com/o/r/pull/13"
git -C "$SBX" branch feat/slug23 >/dev/null 2>&1
out="$( cd "$SBX" && vr 23 )"
assert "branch conjunct: a LOCAL branch does not satisfy 'delivered'" \
  '[ "$out" = "run-incomplete 23 branch" ]'

# --- a zero-padded id is the SAME id --------------------------------------------
# Docket displays the padded form everywhere (filenames, board, commit scopes, "change 0237"), and
# the validator admits it. Bash `printf %d`/`$((…))` read a leading 0 as OCTAL, so an uncanonicalized
# id silently resolves a DIFFERENT change (0237 -> 159) — and for a padded id containing 8 or 9 the
# arithmetic fails outright while the script, under `set -uo pipefail` with no `-e`, carries on.
# The verdict line echoes the CANONICAL id, so the answer can never name an id we did not read.
write_change 19 implemented feat/slug19 "https://github.com/o/r/pull/15"
push_branch feat/slug19
out="$( cd "$SBX" && vr 0010 )"; rc=$?
assert "padded id: 0010 resolves change 10, verdict echoes the canonical id" \
  '[ "$out" = "run-complete 10" ]'
assert "padded id: 0010 exits 0" '[ "$rc" = "0" ]'
out="$( cd "$SBX" && vr 0011 )"
assert "padded id: 0011 gives the same verdict as 11" \
  '[ "$out" = "run-incomplete 11 status pr branch" ]'
# 0019 is not a valid octal literal — the uncanonicalized form errors here.
err="$( cd "$SBX" && bash "$VR" 0019 --changes-dir "$CH" 2>&1 >/dev/null )"
out="$( cd "$SBX" && vr 0019 )"; rc=$?
assert "padded id with a 9: resolves change 19 rather than failing arithmetic" \
  '[ "$out" = "run-complete 19" ]'
assert "padded id with a 9: exits 0 on a real verdict, not a mangled one" '[ "$rc" = "0" ]'
assert "padded id with a 9: no arithmetic diagnostic on stderr" \
  '! grep -qiF "octal" <<<"$err"'

# --- snapshot mode ------------------------------------------------------------
make_sbx
write_change 30 in-progress feat/slug30 ""
write_change 31 proposed "" ""
write_change 32 in-progress feat/slug32 ""
ids="$( cd "$SBX" && vr --in-progress-ids )"
assert "snapshot: lists exactly the in-progress ids, numerically sorted" \
  '[ "$ids" = "$(printf "30\n32")" ]'
write_change 33 implemented feat/slug33 "https://github.com/o/r/pull/14"
ids="$( cd "$SBX" && vr --in-progress-ids )"
assert "snapshot: an implemented change is not in-progress" \
  '! grep -qx 33 <<<"$ids"'

# --- snapshot mode: --with-claimed-at -----------------------------------------
# runner-dispatch's run gate must tell ITS claim from a foreign one, and a set diff cannot do that.
# This script owns the read (single frontmatter reader for the feature) and converts to epoch here
# through the shared `iso_to_epoch`, so the consumer compares integers and needs no `date` of its
# own. An absent or unparseable stamp prints `-` — no positive evidence, never a number.
make_sbx
write_claimed(){  # write_claimed ID CLAIMED_AT [BODY]
  printf -v padded '%04d' "$1"
  cat > "$CH/active/$padded-slug$1.md" <<EOF
---
id: $1
slug: slug$1
status: in-progress
branch: feat/slug$1
claimed_at: $2
---

## Why
${3:-}
EOF
}
write_claimed 40 "2026-08-07T12:00:00Z"
line="$( cd "$SBX" && vr --in-progress-ids --with-claimed-at )"
assert "with-claimed-at: line is '<id> <epoch>'" '[ "$line" = "40 1786104000" ]'
ids="$( cd "$SBX" && vr --in-progress-ids )"
assert "with-claimed-at: the PLAIN snapshot is unchanged (ids only)" '[ "$ids" = "40" ]'

write_claimed 41 "not-a-timestamp"
line="$( cd "$SBX" && vr --in-progress-ids --with-claimed-at | grep '^41 ' )"
assert "with-claimed-at: an unparseable claimed_at prints '-', never a number" '[ "$line" = "41 -" ]'

printf -v p '%04d' 42
cat > "$CH/active/$p-slug42.md" <<'EOF'
---
id: 42
slug: slug42
status: in-progress
branch: feat/slug42
---

## Why

claimed_at: 2026-08-07T12:00:00Z
EOF
line="$( cd "$SBX" && vr --in-progress-ids --with-claimed-at | grep '^42 ' )"
assert "with-claimed-at: an absent key is '-' and body prose is NOT read as the stamp" \
  '[ "$line" = "42 -" ]'

err="$( cd "$SBX" && bash "$VR" 40 --with-claimed-at --changes-dir "$CH" 2>&1 >/dev/null )"; rc=$?
assert "with-claimed-at: rejected outside snapshot mode" '[ "$rc" != "0" ] && [ -n "$err" ]'
rm -rf "$SBX"
make_sbx
write_change 30 in-progress feat/slug30 ""

# --- errors: the check could not run => non-zero, no verdict ------------------
out="$( cd "$SBX" && bash "$VR" 999 --changes-dir "$CH" 2>/dev/null )"; rc=$?
assert "missing id: non-zero" '[ "$rc" != "0" ]'
assert "missing id: emits no verdict line on stdout" '[ -z "$out" ]'
err="$( cd "$SBX" && bash "$VR" --changes-dir "$CH" 2>&1 >/dev/null )"; rc=$?
assert "no id and no mode: non-zero" '[ "$rc" != "0" ]'
assert "no id and no mode: diagnostic names the script" 'grep -qF "verify-run" <<<"$err"'
err="$( cd "$SBX" && bash "$VR" 10 --changes-dir "$SBX/nope" 2>&1 >/dev/null )"; rc=$?
assert "bad --changes-dir: non-zero with a diagnostic" '[ "$rc" != "0" ] && [ -n "$err" ]'
err="$( cd "$SBX" && bash "$VR" abc --changes-dir "$CH" 2>&1 >/dev/null )"; rc=$?
assert "non-numeric id is rejected up front" '[ "$rc" != "0" ]'
err="$( cd "$SBX" && bash "$VR" 30 --in-progress-ids --changes-dir "$CH" 2>&1 >/dev/null )"; rc=$?
assert "id combined with --in-progress-ids: non-zero with a diagnostic" '[ "$rc" != "0" ] && [ -n "$err" ]'
rm -rf "$SBX"

# ---- 0237: `## Run halted` — producer coverage, not just definition ----------------
# LEARNINGS specified-but-unreachable: a contract can be fully specified and ship INERT because
# nothing writes it. Consumer-side asserts pass identically in both worlds, so at least one assert
# must anchor on the paragraph that performs the WRITE.
CONV="$ROOT/skills/docket-convention/SKILL.md"
IMPL="$ROOT/skills/docket-implement-next/SKILL.md"
BCMD="$ROOT/scripts/board-checks.md"

# collapse wrapped prose before matching, so a pure re-flow never reddens a policy assert
flat(){ tr '\n' ' ' < "$1" | tr -s ' '; }

assert "0237 prose: the convention lists '## Run halted' as a body section" \
  'grep -qF -- "- \`## Run halted\`" "$CONV"'
assert "0237 prose: the convention's entry names it presence-encoded" \
  'grep -F "## Run halted" "$CONV" | grep >/dev/null -iF "presence-encoded"'

# PRODUCER — anchored on the halted disposition prose that performs the write, not on a section
# that merely defines what the write means. Scoped to Step 3 with awk FIRST, so each pattern needs
# at most ONE bounded gap: stacking two bounded gaps in one alternation backtracks catastrophically
# on non-matching input, so the assert would HANG instead of reddening on exactly the prose reflow
# it exists to catch (learning `stacked-gap-regex-hangs-instead-of-failing`).
step3="$(awk "/^### Step 3 — Reconcile/,/^### Step 4/" "$IMPL" | tr '\n' ' ' | tr -s ' ')"
assert "0237 prose: Step 3's halted disposition WRITES the section" \
  'grep -qiE "(write|writing|append)[^.]{0,250}## Run halted" <<<"$step3"'
# WRITE SHAPE — RETIRED (0316 plan Task 20 follow-up, category (a)). Under the Bash procedure the
# agent hand-authored the `## Run halted` heading, so the producer prose had to name it bare/undated:
# `has_section` is a whole-line match, and a dated H2 the agent typed would be invisible to the
# reader (the `## Run halted — <date>` fixtures above pin that behavior and STAY guarded by them —
# this retires only the PROSE instruction, never that behavior). The Go verb now OWNS the heading:
# `internal/app/change_halt.go` sets `runHaltedSectionHeading = domain.RunHaltedSection` (the bare
# `## Run halted` H2), writes the section through `render.ApplySectionEdits`, and `haltReportBody`
# puts the date in a `### <date> — halted` SUB-heading inside the body — so an agent calling
# `docket change halt` cannot emit a dated H2 and the producer prose need not name the heading shape.
# Authority #2: change_halt.go's `runHaltedSectionHeading` / `domain.RunHaltedSection` own the
# heading. Guard re-pointed at the surviving substance: the producer delegates the write to that Go
# verb rather than hand-authoring the heading.
assert "0237 prose: the producer delegates the '## Run halted' write to the Go verb that owns the heading" \
  'grep -qiE "docket change halt[^.]{0,250}## Run halted|## Run halted[^.]{0,250}docket change halt" <<<"$step3"'
# GIT-WRITE SHAPE — RE-KEYED (0316 plan Task 20 follow-up, category (c)). The halted write is still a
# committed git act, only the vocabulary moved: the producer says "**write that halt into git before
# stopping**" and `docket change halt` "records ... into a `## Run halted` section ... in one
# exact-version transaction" (change_halt.go commits it, subject "change NNNN run halted"). The
# literal token "commit" left the window — key on the git-write shape (into-git / exact-version
# transaction), not the word. The property is unchanged: a `halted` disposition is a COMMITTED git
# fact `verify-run` reads, never an untrusted sentence in a report. The old `commit` spelling stays
# as a third alternative so a future reword back to it still passes.
assert "0237 prose: the halted write is described as a committed git act (into-git / a git transaction)" \
  'grep -qiE "into git[^.]{0,200}## Run halted|## Run halted[^.]{0,250}(exact-version transaction|transaction|commit)" <<<"$step3"'

# REMOVAL — owned by Step 2's claim (presence-encoded-state: every transition out removes it).
step2="$(awk "/^### Step 2 — Claim/,/^### Step 3/" "$IMPL" | tr '\n' ' ' | tr -s ' ')"
assert "0237 prose: Step 2's claim removes a stale '## Run halted'" \
  'grep -qF "## Run halted" <<<"$step2"'
assert "0237 prose: and states removal, not merely mentions the section" \
  'grep -qiE "(remove|delete|strip)[^.]{0,120}## Run halted|## Run halted[^.]{0,120}(remove|delete|strip)" <<<"$step2"'

# board-checks.md gains the pointer sentence and NOTHING in board-checks.sh changed.
assert "0237 prose: board-checks.md points at verify-run" 'grep -qF "verify-run" "$BCMD"'
assert "0237 prose: the pointer says the check is floor-free at a dispatch seam" \
  'flat "$BCMD" | grep >/dev/null -iE "verify-run[^.]{0,200}(floor-free|no floor|without a floor|dispatch seam)"'

# ---- change 0271: the build verdict family --------------------------------------
# A SECOND family, never the implement-next conjuncts stretched to fit: a build task's
# terminal state is a commit on the feature branch, not a PR.
mkbuildfixture(){   # sets BWT (a repo with a feature branch), BASE (the dispatch-time sha)
  BWT="$(mktemp -d "${TMPDIR:-/tmp}/docket-bwt.XXXXXX")"; BWT="$(cd "$BWT" && pwd -P)"
  git -C "$BWT" init --quiet
  git -C "$BWT" config user.email t@t.test
  git -C "$BWT" config user.name Test
  ( cd "$BWT" && git commit --allow-empty -qm base )
  git -C "$BWT" checkout -q -b feat/thing
  BASE="$(git -C "$BWT" rev-parse HEAD)"
}
vr_build(){ bash "$ROOT/scripts/verify-run.sh" --build --worktree "$BWT" --branch feat/thing --since "$BASE" 2>&1; }

# (a) nothing committed yet -> tip unmet
mkbuildfixture
v="$(vr_build)"; rc=$?
assert "0271-a: no commit yet is task-incomplete" '[[ "$v" == task-incomplete* ]]'
assert "0271-b: the unmet token is tip" '[[ "$v" == *"tip"* ]]'
assert "0271-c: task-incomplete is a FINDING, exit 0" '[ "$rc" = "0" ]'

# (b) a commit lands -> task-committed
( cd "$BWT" && git commit --allow-empty -qm "task work" )
v="$(vr_build)"; rc=$?
assert "0271-d: an advanced tip on a clean tree is task-committed" '[[ "$v" == "task-committed feat/thing" ]]'
assert "0271-e: task-committed exits 0" '[ "$rc" = "0" ]'

# (c) a dirty tree -> tree unmet (the STRANDED-WORK case this change exists for:
#     change 0258 left +64 lines uncommitted and the caller saw only exit 143)
printf 'stranded\n' > "$BWT/stranded.txt"
v="$(vr_build)"
assert "0271-f: an untracked stranded file is task-incomplete" '[[ "$v" == task-incomplete* ]]'
assert "0271-g: the unmet token is tree" '[[ "$v" == *"tree"* ]]'
rm -f "$BWT/stranded.txt"

# (d) wrong branch -> branch unmet
git -C "$BWT" checkout -q -b feat/other
v="$(vr_build)"
assert "0271-h: the wrong branch is task-incomplete" '[[ "$v" == task-incomplete* ]]'
assert "0271-i: the unmet token is branch" '[[ "$v" == *"branch"* ]]'

# (d2) `tip` is a DESCENDANCY check, not an inequality. A tip that merely DIFFERS from the
#      dispatch-time sha — a bad rebase, a reset onto an unrelated history — is not evidence that
#      the task's work landed ON TOP of what was dispatched, and an inequality reports it as met.
mkbuildfixture
( cd "$BWT" && git checkout -q --orphan feat/rewritten \
    && git commit --allow-empty -qm "unrelated history" \
    && git branch -qM feat/thing )
v="$(vr_build)"
assert "0271-o: a tip that is not a descendant of the dispatch sha is task-incomplete" \
  '[[ "$v" == task-incomplete* ]]'
assert "0271-p: the unmet token is tip" '[[ "$v" == *"tip"* ]]'

# (e) a worktree that is not a repo -> task-unverifiable, never a synthesized failure
BWT="$(mktemp -d "${TMPDIR:-/tmp}/docket-norepo.XXXXXX")"
v="$(vr_build)"; rc=$?
assert "0271-j: a non-repo worktree is task-unverifiable" '[[ "$v" == task-unverifiable* ]]'
assert "0271-k: task-unverifiable still exits 0 (a verdict was produced)" '[ "$rc" = "0" ]'

# (f) usage errors are exit 2, distinct from every verdict
v="$(bash "$ROOT/scripts/verify-run.sh" --build --branch feat/thing 2>&1)"; rc=$?
assert "0271-l: --build without --worktree is a usage error (2)" '[ "$rc" = "2" ]'
assert "0271-m: the usage diagnostic is not a verdict line" '[[ "$v" != task-* ]]'

# (g) the two families never collide: --build must not accept an <id>
v="$(bash "$ROOT/scripts/verify-run.sh" --build 7 --worktree "$BWT" 2>&1)"; rc=$?
assert "0271-n: --build rejects an id (families stay separate)" '[ "$rc" = "2" ]'

# ---- change 0324: a plan-only stop is the external gate's run-incomplete -------------
# implement-next Step 4 dispatches the model-pinned plan-writer, which COMMITS a plan artifact on
# the feature branch (marked with a `Docket-Plan-Path:` trailer) before any build work. A run that
# stops right there — plan committed and pushed, status still in-progress, no PR — is exactly the
# stopped-after-planning shape the caller's run gate must re-dispatch. This is the distinct
# committed-plan variant of "in-progress, no PR": verify-run reads status/pr/branch and never the
# plan artifact, so a plan commit (trailer and all) must NOT flip the verdict off run-incomplete.
make_sbx
write_change 50 in-progress feat/slug50 ""
mkdir -p "$SBX/docs/superpowers/plans"
printf '# Plan for change 50\n' > "$SBX/docs/superpowers/plans/2026-08-15-slug50.md"
git -C "$SBX" add docs/superpowers/plans/2026-08-15-slug50.md
git -C "$SBX" commit -qm "plan(0050): author implementation plan

Docket-Plan-Path: docs/superpowers/plans/2026-08-15-slug50.md"
push_branch feat/slug50
out="$( cd "$SBX" && vr 50 )"
assert "plan-only stop: a committed plan (with Docket-Plan-Path: trailer) does NOT flip the verdict off run-incomplete" \
  '[ "$out" = "run-incomplete 50 status pr" ]'

# ---- change 0342: run-waiting is DELEGATED to the Go authority ----------------------
# The run-waiting verdict is derived from agreeing LOCAL gate-drive receipts that live OUTSIDE the
# change file, so this script cannot read it from frontmatter — it shells out to `docket run verify`
# (internal/app RunVerify's evaluateRunWaiting) on the run-incomplete fallthrough ONLY and relays a
# run-waiting verdict when the Go op reports one. Before this leg existed, every `run-waiting*`
# consumer in runner-dispatch.sh was dead under the default VERIFY_RUN and a genuine WAITING
# continuation was reported as `run-incomplete`, re-dispatched as a FRESH run. The mock DOCKET_BIN
# stub stands in for the Go binary so the case is hermetic; it also RECORDS whether it was called,
# which is how the precedence/short-circuit asserts prove the Go op is not consulted where the
# frontmatter already decides the verdict.
make_sbx
STUBLOG="$SBX/stub.calls"
cat > "$SBX/stub-docket.sh" <<EOF
#!/usr/bin/env bash
# mock \`docket\` — records the call and emits whatever JSON STUB_JSON carries, ignoring args.
printf 'called\n' >> "$STUBLOG"
printf '%s\n' "\${STUB_JSON:-}"
EOF
chmod +x "$SBX/stub-docket.sh"
STUB="$SBX/stub-docket.sh"
vrw(){ DOCKET_BIN="$STUB" STUB_JSON="$1" bash "$VR" "$2" --changes-dir "$CH" 2>/dev/null; }

# (a) the fallthrough case: an in-progress change with no PR (run-incomplete: status pr branch),
#     and the Go op reports a waiting continuation -> verify-run relays it in the consumer shape.
: > "$STUBLOG"
write_change 60 in-progress feat/slug60 ""
out="$( cd "$SBX" && vrw '{"verdict":"run-waiting","handoff_id":"drive-abc123","phase":"gate"}' 60 )"; rc=$?
assert "run-waiting: verify-run emits 'run-waiting <id> <handoff-id> <phase>'" \
  '[ "$out" = "run-waiting 60 drive-abc123 gate" ]'
assert "run-waiting: a produced verdict exits 0" '[ "$rc" = "0" ]'
assert "run-waiting: the Go authority was actually consulted on the fallthrough" \
  '[ -s "$STUBLOG" ]'

# (b) precedence: run-halted still wins where it applies — a change carrying a `## Run halted`
#     section returns run-halted BEFORE the waiting probe, and the Go op is never consulted.
: > "$STUBLOG"
write_change 61 in-progress feat/slug61 "" "
## Run halted

### 2026-08-24 — halted

needs a human."
out="$( cd "$SBX" && vrw '{"verdict":"run-waiting","handoff_id":"drive-x","phase":"gate"}' 61 )"
assert "run-waiting: run-halted takes precedence over a waiting continuation" '[ "$out" = "run-halted 61" ]'
assert "run-waiting: the Go op is NOT consulted once run-halted decides" '[ ! -s "$STUBLOG" ]'

# (c) short-circuit: a run-complete change never reaches the probe, so the Go op is not consulted
#     even when it would report waiting — the existing verdict is unchanged.
: > "$STUBLOG"
write_change 62 implemented feat/slug62 "https://github.com/o/r/pull/20"
push_branch feat/slug62
out="$( cd "$SBX" && vrw '{"verdict":"run-waiting","handoff_id":"drive-y","phase":"gate"}' 62 )"
assert "run-waiting: run-complete is unchanged and never consults the Go op" \
  '[ "$out" = "run-complete 62" ] && [ ! -s "$STUBLOG" ]'

# (d) the Go op reports a NON-waiting verdict -> verify-run's own reading stands (run-incomplete);
#     the Go op never overrides the conjuncts this script read.
out="$( cd "$SBX" && vrw '{"verdict":"run-incomplete","unmet":[]}' 60 )"
assert "run-waiting: a non-waiting Go verdict does not override run-incomplete" \
  '[ "$out" = "run-incomplete 60 status pr branch" ]'

# (e) a partial waiting report (handoff present, phase empty) is a MALFORMED verdict, not waiting —
#     the consumers parse three fields, so an incomplete line must degrade to run-incomplete.
out="$( cd "$SBX" && vrw '{"verdict":"run-waiting","handoff_id":"drive-z","phase":""}' 60 )"
assert "run-waiting: a partial waiting report degrades to run-incomplete" \
  '[ "$out" = "run-incomplete 60 status pr branch" ]'

# (f) a missing/erroring binary is swallowed and the run-incomplete finding stands — the delegation
#     never turns a finding into a script failure.
out="$( cd "$SBX" && DOCKET_BIN="$SBX/does-not-exist" bash "$VR" 60 --changes-dir "$CH" 2>/dev/null )"; rc=$?
assert "run-waiting: a missing Go binary falls through to run-incomplete" \
  '[ "$out" = "run-incomplete 60 status pr branch" ]'
assert "run-waiting: a missing Go binary still exits 0 (a finding is not a failure)" '[ "$rc" = "0" ]'

exit $fail
