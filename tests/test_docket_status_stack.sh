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
assert "the flip landed as a commit and the shared worktree is clean" \
  '[ -z "$(git -C "$mw" status --porcelain -- docs/changes/active/0041-child.md)" ] && \
   git -C "$mw" show "origin/main:docs/changes/active/0041-child.md" 2>/dev/null | grep -q "^status: stacked-merged$"'

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

printf '%s\n' "--- done"
exit "$fail"
