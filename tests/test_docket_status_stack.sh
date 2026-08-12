#!/usr/bin/env bash
# tests/test_docket_status_stack.sh — the docket-status sweep's STACKED legs (change 0298).
#
# SIBLING SHARD, not an extension: tests/test_docket_status.sh owns the sweep's ordinary close-out
# coverage but sits at the 60s ceiling of tests/runtime-budgets.tsv, and that table's header states
# a row at its ceiling has no next raise. New sweep legs therefore land here.
#
# What this file pins is spec §6's third bullet — the PRODUCER of the `stacked-merged` state. A PR
# merged into its stack parent's branch has not reached the integration branch, so by the governing
# invariant its change is not `done`: it flips in place and stays in active/. A PR merged into the
# integration branch is unaffected, and THAT is the assert with the teeth — it is what proves the
# base-ref comparison discriminates instead of firing on every stacked change.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/docket-status.sh"
DOCKET_BASH_PATH=""
for runtime_candidate in "$(command -v bash)" /opt/homebrew/bin/bash /usr/local/bin/bash; do
  [ -x "$runtime_candidate" ] || continue
  [ "$(LC_ALL=C "$runtime_candidate" --version 2>/dev/null | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')" -ge 4 ] 2>/dev/null || continue
  DOCKET_BASH_PATH="$runtime_candidate"; break
done
: "${DOCKET_BASH_PATH:?tests require an absolute GNU Bash 4+ runtime}"
export DOCKET_BASH_PATH
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

tmp="$(mktemp -d "${TMPDIR:-/tmp}/docket-status-stack.XXXXXX")"; trap 'rm -rf "$tmp"' EXIT

# --- fixture: a bare origin, a clone standing in for the metadata worktree -----------------------
# The clone IS $mw here (DOCKET_MODE=main resolves the metadata worktree to the repo root of the
# process CWD), so the sweep's in-place flip commits and pushes for real against this origin.
git init -q -b main "$tmp/seed"
git -C "$tmp/seed" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init
git clone -q --bare "$tmp/seed" "$tmp/origin.git"
git -C "$tmp/origin.git" config remote.origin.url "$tmp/seed"
git clone -q "$tmp/origin.git" "$tmp/work" 2>/dev/null
mw="$tmp/work"
mkdir -p "$mw/docs/changes/active" "$mw/docs/changes/archive" "$mw/docs/adrs"
git -C "$mw" config user.email t@t
git -C "$mw" config user.name t

seed_change(){
  # $1 id, $2 slug, $3 status, $4 stacked_on (may be empty)
  cat > "$mw/docs/changes/active/$(printf '%04d' "$1")-$2.md" <<EOF
---
id: $1
slug: $2
title: $2 change
status: $3
priority: high
depends_on: []
stacked_on: $4
branch: feat/$2
pr: $1
---

Body. Prose here may legitimately discuss status: done without being frontmatter.
EOF
}
# The parent is in-progress on purpose: detect_merged only considers implemented changes, so the
# parent is never itself a sweep candidate here and only its recorded branch: matters.
seed_change 40 parent in-progress ""
seed_change 41 child implemented 40
# 0042 is STACKED TOO, and that is the whole point of it: its PR merged into the INTEGRATION branch
# rather than into the parent's. An unstacked change here would make the discriminating assert below
# vacuous — it would stay green with the base-ref comparison deleted, because the absent stacked_on
# already short-circuits the leg. Being stacked-but-merged-onto-main is the only fixture the
# comparison itself decides, and it is a real situation: a child retargeted onto the integration
# branch when its parent lands first. Its code IS reachable from the integration branch, so the
# governing invariant makes it `done`.
seed_change 42 plain implemented 40
git -C "$mw" add docs/changes
git -C "$mw" commit -q -m "seed stack sweep changes"
git -C "$mw" push -q origin main

# --- the gh stub: 41 merged into the PARENT's branch, 42 merged into the integration branch ------
cat > "$tmp/gh-stack.sh" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = repo ] && [ "$2" = view ]; then echo "x/y"; exit 0; fi
if [ "$1" = api ] && [ "$2" = graphql ]; then
  cat <<'JSON'
{"data":{"p41":{"pullRequest":{"number":41,"mergedAt":"2026-08-01T09:00:00Z","state":"MERGED","baseRefName":"feat/parent"}},"p42":{"pullRequest":{"number":42,"mergedAt":"2026-08-02T09:00:00Z","state":"MERGED","baseRefName":"main"}}}}
JSON
  exit 0
fi
echo "gh-stack: unexpected args: $*" >&2
exit 1
EOF
chmod +x "$tmp/gh-stack.sh"

# --- the close-out mocks, via the SCRIPTS_DIR seam ------------------------------------------------
mkdir -p "$tmp/mock-scripts"
calllog="$tmp/closeout-calls.log"
: > "$calllog"

cat > "$tmp/mock-scripts/archive-change.sh" <<'EOF'
#!/usr/bin/env bash
echo "archive-change $*" >> "$SWEEP_LOG"
changes_dir="" id="" date=""
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) changes_dir="$2"; shift ;;
    --id) id="$2"; shift ;;
    --date) date="$2"; shift ;;
  esac
  shift
done
pad="$(printf '%04d' "$id")"
active="$(find "$changes_dir/active" -maxdepth 1 -name "${pad}-*.md" | sed -n 1p)"
[ -n "$active" ] || exit 1
base="$(basename "$active")"
slug="${base#"${pad}"-}"; slug="${slug%.md}"
mkdir -p "$changes_dir/archive"
dest="$changes_dir/archive/${date}-${pad}-${slug}.md"
root="$(git -C "$changes_dir" rev-parse --show-toplevel)"
git -C "$root" mv "$active" "$dest"
sed -i.bak "s/^status:.*/status: done/" "$dest" && rm -f "$dest.bak"
git -C "$root" add -- "$dest" 2>/dev/null
git -C "$root" -c user.email=t@t -c user.name=t commit -q -m "mock archive" >/dev/null 2>&1
git -C "$root" push -q origin main >/dev/null 2>&1
exit 0
EOF
cat > "$tmp/mock-scripts/render-change-links.sh" <<'EOF'
#!/usr/bin/env bash
echo "render-change-links $*" >> "$SWEEP_LOG"
exit 0
EOF
cat > "$tmp/mock-scripts/terminal-publish.sh" <<'EOF'
#!/usr/bin/env bash
echo "terminal-publish $*" >> "$SWEEP_LOG"
exit 0
EOF
cat > "$tmp/mock-scripts/cleanup-feature-branch.sh" <<'EOF'
#!/usr/bin/env bash
echo "cleanup-feature-branch $*" >> "$SWEEP_LOG"
exit 0
EOF
chmod +x "$tmp/mock-scripts/"*.sh

# --- detection alone, BEFORE the sweep -----------------------------------------------------------
# Order is load-bearing: the sweep below flips 0041 out of `implemented`, and detect_merged would
# then legitimately stop emitting it — an assert placed after the sweep would read empty and fail
# for a reason unrelated to the field it is pinning.
dm="$( cd "$mw" && DOCKET_MODE=main CHANGES_DIR=docs/changes GH="$tmp/gh-stack.sh" \
  bash -c '. "'"$SCRIPT"'"; detect_merged' )"
assert "detect_merged carries the PR's base ref as a fifth field" \
  'grep -qF "$(printf "41\tchild\t41\t2026-08-01\tfeat/parent")" <<<"$dm"'
assert "the integration-branch merge carries its own base ref" \
  'grep -qF "$(printf "42\tplain\t42\t2026-08-02\tmain")" <<<"$dm"'

# --- the run: detection and close-out composed, so the baseRefName plumbing is end-to-end ---------
out="$tmp/sweep-out.txt"
( cd "$mw" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=main \
  GH="$tmp/gh-stack.sh" SCRIPTS_DIR="$tmp/mock-scripts" SWEEP_LOG="$calllog" \
  bash -c '. "'"$SCRIPT"'"; detect_merged | sweep_execute' ) > "$out" 2>"$tmp/sweep-err.txt"

assert "a PR merged into the parent branch reports stacked-merged" \
  'grep -qF "stacked-merged 41 40" "$out"'
assert "the child is not archived" \
  '[ -f "$mw/docs/changes/active/0041-child.md" ]'
st="$(sed -n 's/^status: //p' "$mw/docs/changes/active/0041-child.md")"
assert "the child status is stacked-merged" \
  '[ "$(printf "%s\n" "$st" | sed -n 1p)" = stacked-merged ]'
assert "the child never reports swept" \
  '! grep -qE "^swept 41 " "$out"'
assert "no close-out step at all ran for the child" \
  '! grep -qF -- "--id 41 " "$calllog" && ! grep -qF -- "--slug child" "$calllog"'
assert "the child's branch is NOT cleaned up — it still carries code the root needs" \
  '! grep -qF "cleanup-feature-branch" "$calllog" || ! grep -qF -- "--slug child" "$calllog"'

# THE DISCRIMINATING ASSERT — 0042 carries a stacked_on: of its own, so the base-ref comparison is
# the ONLY thing standing between it and the flip. Delete that comparison in sweep_stacked_parent
# and this reddens: every stacked change would flip on any merge, including one retargeted onto and
# merged into the integration branch, whose code IS reachable from it and which is therefore `done`.
assert "a stacked PR merged into the INTEGRATION branch still sweeps to done" \
  'grep -qF "swept 42 " "$out"'
assert "the integration-branch merge never reports stacked-merged" \
  '! grep -qE "^stacked-merged 42 " "$out"'
assert "the integration-branch merge did archive and publish" \
  '[ -f "$mw/docs/changes/archive/2026-08-02-0042-plain.md" ] && grep -qF -- "--id 42 " "$calllog"'

# The flip is a real commit on the metadata branch, not a working-tree scribble: an uncommitted
# status change would be reverted by the very next pass's `pull --rebase`.
# `git show` is CAPTURED before it is matched, never piped into `grep -q`: under `set -o pipefail`
# the early-exiting consumer SIGPIPEs the producer and the resulting 141 is an intermittent failure
# (AGENTS.md, and tests/test_pipe_shapes.sh enforces it).
remote_child="$(git -C "$mw" show "origin/main:docs/changes/active/0041-child.md" 2>/dev/null)"
assert "the flip landed as a commit and the shared worktree is clean" \
  '[ -z "$(git -C "$mw" status --porcelain -- docs/changes/active/0041-child.md)" ] && \
   grep -q "^status: stacked-merged$" <<<"$remote_child"'

# --- idempotency: re-feeding the SAME close-out record is a silent no-op --------------------------
# detect_merged would not re-emit this change (it is no longer `implemented`), so the guard is
# exercised by feeding sweep_execute the record directly — otherwise the assert would pass for a
# reason that has nothing to do with the guard.
rerun="$tmp/rerun-out.txt"
printf '41\tchild\t41\t2026-08-01\tfeat/parent\n' > "$tmp/rerun-input.tsv"
( cd "$mw" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=main \
  SCRIPTS_DIR="$tmp/mock-scripts" SWEEP_LOG="$calllog" SWEEP_INPUT="$tmp/rerun-input.tsv" \
  bash -c '. "'"$SCRIPT"'"; sweep_execute < "$SWEEP_INPUT"' ) > "$rerun" 2>>"$tmp/sweep-err.txt"
assert "re-sweeping a stacked-merged change emits nothing at all" \
  '[ -z "$(tr -d "[:space:]" < "$rerun")" ]'
assert "re-sweeping a stacked-merged change leaves it in active/ untouched" \
  '[ -f "$mw/docs/changes/active/0041-child.md" ] && \
   [ -z "$(git -C "$mw" status --porcelain -- docs/changes/active/0041-child.md)" ]'

# --- a four-field record (every pre-0298 caller and fixture) still closes out normally ------------
seed_change 43 legacy implemented ""
git -C "$mw" add docs/changes
git -C "$mw" commit -q -m "seed legacy record change"
git -C "$mw" push -q origin main
legacy="$tmp/legacy-out.txt"
printf '43\tlegacy\t43\t2026-08-03\n' > "$tmp/legacy-input.tsv"
( cd "$mw" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=main \
  SCRIPTS_DIR="$tmp/mock-scripts" SWEEP_LOG="$calllog" SWEEP_INPUT="$tmp/legacy-input.tsv" \
  bash -c '. "'"$SCRIPT"'"; sweep_execute < "$SWEEP_INPUT"' ) > "$legacy" 2>>"$tmp/sweep-err.txt"
assert "a four-field close-out record (no base ref) still sweeps to done" \
  'grep -qF "swept 43 " "$legacy"'

# --- the detection query actually ASKS for the base ref -------------------------------------------
# stdout cannot witness a graphql selection set, and a stub that answers with baseRefName would keep
# every assert above green even if the real query never requested it. Read the source.
assert "the aliased graphql selection requests baseRefName" \
  'q="$(grep -F "pullRequest(number:" "$SCRIPT")"; grep -qF "baseRefName" <<<"$q"'
assert "the pr list fallback requests baseRefName too" \
  'p="$(grep -F -- "--json number,mergedAt" "$SCRIPT")"; grep -qF "baseRefName" <<<"$p"'

# --- change 0298 task 8: a ROOT merge drives the stack close-out from inside the sweep -------------
# 60 is the root, merged into the integration branch. 61 and 62 already sit at `stacked-merged` from
# earlier passes. 63 is the RACE: it is still `implemented` when this pass starts and its PR merged
# into the root's own branch, so the pass must flip it AND then promote it — which is only possible
# because the descendant snapshot is taken after the per-change loop has run, not before it.
seed_change 60 root implemented ""
seed_change 61 childone stacked-merged 60
seed_change 62 childtwo stacked-merged 60
seed_change 63 racer implemented 60
git -C "$mw" add docs/changes
git -C "$mw" commit -q -m "seed root stack changes"
git -C "$mw" push -q origin main

cat > "$tmp/gh-root.sh" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = repo ] && [ "$2" = view ]; then echo "x/y"; exit 0; fi
if [ "$1" = api ] && [ "$2" = graphql ]; then
  cat <<'JSON'
{"data":{"p60":{"pullRequest":{"number":60,"mergedAt":"2026-08-05T09:00:00Z","state":"MERGED","baseRefName":"main"}},"p63":{"pullRequest":{"number":63,"mergedAt":"2026-08-05T10:00:00Z","state":"MERGED","baseRefName":"feat/root"}}}}
JSON
  exit 0
fi
echo "gh-root: unexpected args: $*" >&2
exit 1
EOF
chmod +x "$tmp/gh-root.sh"

# The close-out reached through the SCRIPTS_DIR seam is the REAL scripts/stack-closeout.sh — a shim
# that logs its argv and then execs it, so it runs against the mocked close-out helpers above (it
# resolves those through the same exported SCRIPTS_DIR). A stub echoing `promoted` lines would make
# every assert below vacuous: what is under test is that the sweep drives the real close-out with a
# usable argument set, not that a stub can print.
cat > "$tmp/mock-scripts/stack-closeout.sh" <<EOF
#!/usr/bin/env bash
echo "stack-closeout \$*" >> "\$SWEEP_LOG"
exec "\$DOCKET_BASH_PATH" "$REPO/scripts/stack-closeout.sh" "\$@"
EOF
chmod +x "$tmp/mock-scripts/stack-closeout.sh"

rootout="$tmp/root-out.txt"
( cd "$mw" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=main \
  GH="$tmp/gh-root.sh" SCRIPTS_DIR="$tmp/mock-scripts" SWEEP_LOG="$calllog" \
  bash -c '. "'"$SCRIPT"'"; detect_merged | sweep_execute' ) > "$rootout" 2>>"$tmp/sweep-err.txt"

assert "a root merge promotes its descendants" \
  'grep -qF "promoted 61" "$rootout" && grep -qF "promoted 62" "$rootout"'
assert "the root is still swept to done" \
  'grep -qF "swept 60 " "$rootout"'
# THE RACE LEG. `stacked-merged 63 60` is printed by the per-change loop and `promoted 63` by the
# close-out that runs after it. Move the close-out invocation inside the loop and this reddens: at
# the moment the root is swept, 63 is still `implemented` and no descendant scan can see it.
assert "a root sweep racing a just-merged child handles both in one pass" \
  'grep -qF "stacked-merged 63 60" "$rootout" && grep -qF "promoted 63" "$rootout"'
assert "each promoted descendant is archived with the ROOT's merge date" \
  '[ -f "$mw/docs/changes/archive/2026-08-05-0061-childone.md" ] && \
   [ -f "$mw/docs/changes/archive/2026-08-05-0063-racer.md" ]'
carried="$(cat "$mw/docs/changes/archive/2026-08-05-0060-root.md" 2>/dev/null)"
assert "the root record carries the Stack carried table" \
  'grep -qF "## Stack carried" <<<"$carried" && grep -qF "#0061" <<<"$carried" && grep -qF "#0063" <<<"$carried"'
assert "the sweep relays the close-out's stack-carried line" \
  'grep -qE "^stack-carried 60 3$" "$rootout"'
# Scoping: 0041 is `stacked-merged` too, but on a DIFFERENT root. A close-out that promoted every
# stacked-merged change in the tree instead of this root's descendants would archive it.
assert "a stacked-merged change outside this root's stack is untouched" \
  '! grep -qF "promoted 41" "$rootout" && [ -f "$mw/docs/changes/active/0041-child.md" ]'
# The close-out is invoked once per swept ROOT — never for a swept change that carries no stack, so
# the ordinary close-out pays nothing for this feature.
assert "no close-out was invoked for the stackless changes swept earlier" \
  'n="$(grep -cF "stack-closeout " "$calllog")"; [ "$n" = 1 ] && ! grep -qF "stack-closeout" "$out" && ! grep -qF "stack-closeout" "$legacy"'

# --- the close-out's failure posture is the sweep's own: log, and continue to the next root --------
# Two independent stacks, both roots merged into the integration branch, with the close-out failing
# outright. The second root's line is what has teeth: it can only appear if the first failure was
# reported rather than propagated out of the drain loop.
seed_change 70 rootone implemented ""
seed_change 71 kidone stacked-merged 70
seed_change 72 roottwo implemented ""
seed_change 73 kidtwo stacked-merged 72
git -C "$mw" add docs/changes
git -C "$mw" commit -q -m "seed failing close-out stacks"
git -C "$mw" push -q origin main

cp -R "$tmp/mock-scripts" "$tmp/mock-scripts-fail"
cat > "$tmp/mock-scripts-fail/stack-closeout.sh" <<'EOF'
#!/usr/bin/env bash
echo "stack-closeout: could not run" >&2
exit 1
EOF
chmod +x "$tmp/mock-scripts-fail/stack-closeout.sh"

cat > "$tmp/gh-fail.sh" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = repo ] && [ "$2" = view ]; then echo "x/y"; exit 0; fi
if [ "$1" = api ] && [ "$2" = graphql ]; then
  cat <<'JSON'
{"data":{"p70":{"pullRequest":{"number":70,"mergedAt":"2026-08-06T09:00:00Z","state":"MERGED","baseRefName":"main"}},"p72":{"pullRequest":{"number":72,"mergedAt":"2026-08-06T09:00:00Z","state":"MERGED","baseRefName":"main"}}}}
JSON
  exit 0
fi
echo "gh-fail: unexpected args: $*" >&2
exit 1
EOF
chmod +x "$tmp/gh-fail.sh"

failout="$tmp/fail-out.txt"
( cd "$mw" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=main \
  GH="$tmp/gh-fail.sh" SCRIPTS_DIR="$tmp/mock-scripts-fail" SWEEP_LOG="$calllog" \
  bash -c '. "'"$SCRIPT"'"; detect_merged | sweep_execute' ) > "$failout" 2>>"$tmp/sweep-err.txt"

assert "a close-out that cannot run is reported, not swallowed" \
  'grep -qF "sweep-failed 70 stack-closeout script-error" "$failout"'
assert "a failed close-out does not abandon the next root" \
  'grep -qF "sweep-failed 72 stack-closeout script-error" "$failout"'
assert "both roots still swept to done" \
  'grep -qF "swept 70 " "$failout" && grep -qF "swept 72 " "$failout"'
assert "an unpromoted descendant is left exactly where it was, for the next pass" \
  '[ -f "$mw/docs/changes/active/0071-kidone.md" ] && [ -f "$mw/docs/changes/active/0073-kidtwo.md" ]'
# THE DEDUPE LEG. The resumption scan (below) runs at the end of this very pass and would find 0071
# and 0073 still parked at `stacked-merged` under roots that are already `done` and archived. Strip
# the seeding of its `seen` set from SWEEP_STACK_ROOTS and each root is driven a SECOND time in the
# same pass, so this failure line appears twice — a count a caller keys on, doubled.
assert "a root driven from the queue is not driven again by the resumption scan" \
  'n="$(grep -cF "sweep-failed 70 stack-closeout script-error" "$failout")"; \
   m="$(grep -cF "sweep-failed 72 stack-closeout script-error" "$failout")"; \
   [ "$n" = 1 ] && [ "$m" = 1 ]'

# --- the RESUMPTION scan: a stack stranded by an EARLIER pass is re-driven, not abandoned ---------
# The contract and the sweep both promise that a `sweep-failed <id> stack-closeout script-error` (and
# a `promote-failed`) self-heals on the next pass. Nothing made that true: SWEEP_STACK_ROOTS is
# filled only by a root swept in the SAME pass, and detection never sees an archived root again — so
# one transient failure stranded a stack permanently, under a line saying the opposite.
#
# The fixture is the state the FAILED run above actually left behind — 0071 and 0073 still parked at
# `stacked-merged` under roots 0070/0072 that are `done` and archived — plus a two-level stack under
# an archived root, and the two controls that must NOT move. Nothing merges in this pass at all: the
# gh stub answers with no merged PRs, so the only thing that can move any of them is a scan of
# active/, which is exactly what is under test.
cat > "$mw/docs/changes/archive/2026-08-07-0080-rootrec.md" <<'EOF'
---
id: 80
slug: rootrec
title: rootrec change
status: done
priority: high
depends_on: []
stacked_on:
branch: feat/rootrec
pr: 80
---

Body.
EOF
seed_change 81 kidrec stacked-merged 80
# 0082 is stacked on 0081, NOT on the root — a two-level stranded stack, which must drain WHOLE.
# Two independent mechanisms cover it (the scan walks up through the `stacked-merged` 0081 to the
# root that actually merged, and stack-closeout.sh promotes TRANSITIVE descendants), so what this
# pins is the outcome, not either mechanism on its own.
seed_change 82 grandkidrec stacked-merged 81
# CONTROL: a live root. Its code has not reached the integration branch, so 0085 must stay put — a
# scan that keyed on "is parked at stacked-merged" alone would promote it and falsify the invariant.
seed_change 84 liveroot in-progress ""
seed_change 85 orphanrec stacked-merged 84
# CONTROL: a KILLED root, ARCHIVED — the archived form is what gives this leg teeth. Killed is
# terminal, so a scan gated on "terminal" rather than on `done` promotes 0087; gated on `done` it
# does not. An UNARCHIVED killed root would make the leg vacuous instead: it would be turned away by
# the date probe (no date in the file name) no matter what the status gate said.
cat > "$mw/docs/changes/archive/2026-08-07-0086-deadroot.md" <<'EOF'
---
id: 86
slug: deadroot
title: deadroot change
status: killed
priority: high
depends_on: []
stacked_on:
branch: feat/deadroot
pr: 86
---

Body.
EOF
seed_change 87 orphantwo stacked-merged 86
git -C "$mw" add docs/changes
git -C "$mw" commit -q -m "seed stranded stacks"
git -C "$mw" push -q origin main

cat > "$tmp/gh-none.sh" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = repo ] && [ "$2" = view ]; then echo "x/y"; exit 0; fi
if [ "$1" = api ] && [ "$2" = graphql ]; then echo '{"data":{}}'; exit 0; fi
if [ "$1" = pr ] && [ "$2" = list ]; then echo '[]'; exit 0; fi
echo "gh-none: unexpected args: $*" >&2
exit 1
EOF
chmod +x "$tmp/gh-none.sh"

recout="$tmp/recover-out.txt"
reclog="$tmp/recover-calls.log"
: > "$reclog"
( cd "$mw" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=main \
  GH="$tmp/gh-none.sh" SCRIPTS_DIR="$tmp/mock-scripts" SWEEP_LOG="$reclog" \
  bash -c '. "'"$SCRIPT"'"; detect_merged | sweep_execute' ) > "$recout" 2>>"$tmp/sweep-err.txt"

assert "a stack stranded by an earlier failed close-out is re-driven with nothing merging this pass" \
  'grep -qF "promoted 71" "$recout" && grep -qF "promoted 73" "$recout"'
assert "a two-level stranded stack drains whole" \
  'grep -qF "promoted 81" "$recout" && grep -qF "promoted 82" "$recout"'
assert "a resumed descendant is archived under the ROOT's merge date, never today's" \
  '[ -f "$mw/docs/changes/archive/2026-08-06-0071-kidone.md" ] && \
   [ -f "$mw/docs/changes/archive/2026-08-07-0081-kidrec.md" ]'
assert "a stack whose root is still live is left alone" \
  '! grep -qF "promoted 85" "$recout" && [ -f "$mw/docs/changes/active/0085-orphanrec.md" ]'
# …and left alone SILENTLY. The status gate has to run BEFORE the date probe: a live root is never
# archived, so a scan that reached the probe would turn it away with a `date-unresolved` failure
# line on every single pass — a permanent false finding about a change that is simply not ready.
assert "a live root is turned away by the status gate, not by the date probe" \
  '! grep -qF "sweep-failed 84 " "$recout"'
assert "a stack whose root is KILLED is never promoted" \
  '! grep -qF "promoted 87" "$recout" && [ -f "$mw/docs/changes/active/0087-orphantwo.md" ]'
# 0041 sits at `stacked-merged` under a parent that is still `in-progress` — the same shape as 0085,
# from the first fixture, and it must be as untouched now as it was then.
assert "the scan does not disturb an unrelated parked change" \
  '[ -f "$mw/docs/changes/active/0041-child.md" ]'
# BOUNDED: three distinct roots (0070, 0072, 0080), one close-out each — 0081 and 0082 resolve to the
# SAME root and must not drive it twice. Drop the dedupe set and this reads 4.
assert "the scan drives each stranded root exactly once" \
  'n="$(grep -cF "stack-closeout " "$reclog")"; [ "$n" = 3 ]'

# --- the contract documents what the sweep now relays ---------------------------------------------
# Every line the sweep emits is a machine contract a caller keys on, so an undocumented one is a
# defect. Read the contract file, not the script.
CONTRACT="$REPO/scripts/docket-status.md"
assert "the contract documents the relayed close-out report lines" \
  'for tok in "promoted <id>" "promote-skipped <id> <reason>" "promote-failed <id> <reason>" \
              "stack-carried <root> <count>" "stack-carried-failed <root> <reason>"; do
     grep -qF -- "$tok" "$CONTRACT" || exit 1
   done'
assert "the contract documents the close-out failure line" \
  'grep -qF -- "sweep-failed <id> stack-closeout script-error" "$CONTRACT"'
assert "the contract states the after-the-loop placement" \
  'grep -qF -- "after the whole per-change loop" "$CONTRACT"'
# The self-heal the contract promises is now a named pass with its own report line, so the contract
# has to name both. A prose promise with no mechanism behind it is the defect this leg exists for.
assert "the contract documents the resumption scan and its failure line" \
  'grep -qF -- "6-recover" "$CONTRACT" && \
   grep -qF -- "sweep-failed <id> stack-closeout date-unresolved" "$CONTRACT"'

printf '%s\n' "--- done"
exit "$fail"
