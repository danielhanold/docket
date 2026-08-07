#!/usr/bin/env bash
# tests/test_verify_run.sh — run: bash tests/test_verify_run.sh
# Hermetic: every case builds a sandbox repo with its own changes dir and passes
# --changes-dir, so nothing reads the developer's real docket state.
set -uo pipefail
unset XDG_CONFIG_HOME
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VR="$ROOT/scripts/verify-run.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

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
write_change 15 in-progress feat/slug15 "" "
Writing a \`## Run halted\` section is how a run clears the gate."
out="$( cd "$SBX" && vr 15 )"
assert "halted: a prose mention does not fire the section" \
  '! grep -q "^run-halted" <<<"$out"'

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
  'grep -F "## Run halted" "$CONV" | grep -qiF "presence-encoded"'

# PRODUCER — anchored on the halted disposition prose that performs the write, not on a section
# that merely defines what the write means.
assert "0237 prose: the halted disposition WRITES the section" \
  'flat "$IMPL" | grep -qiE "halted[^.]{0,250}(write|writing|append)[^.]{0,200}## Run halted|## Run halted[^.]{0,200}(commit|committed)"'
assert "0237 prose: the halted write is described as a COMMITTED git act" \
  'flat "$IMPL" | grep -qiE "## Run halted[^.]{0,250}commit"'

# REMOVAL — owned by Step 2's claim (presence-encoded-state: every transition out removes it).
step2="$(awk "/^### Step 2 — Claim/,/^### Step 3/" "$IMPL" | tr '\n' ' ' | tr -s ' ')"
assert "0237 prose: Step 2's claim removes a stale '## Run halted'" \
  'grep -qF "## Run halted" <<<"$step2"'
assert "0237 prose: and states removal, not merely mentions the section" \
  'grep -qiE "(remove|delete|strip)[^.]{0,120}## Run halted|## Run halted[^.]{0,120}(remove|delete|strip)" <<<"$step2"'

# board-checks.md gains the pointer sentence and NOTHING in board-checks.sh changed.
assert "0237 prose: board-checks.md points at verify-run" 'grep -qF "verify-run" "$BCMD"'
assert "0237 prose: the pointer says the check is floor-free at a dispatch seam" \
  'flat "$BCMD" | grep -qiE "verify-run[^.]{0,200}(floor-free|no floor|without a floor|dispatch seam)"'

exit $fail
