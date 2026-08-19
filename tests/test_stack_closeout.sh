#!/usr/bin/env bash
# tests/test_stack_closeout.sh — scripts/stack-closeout.sh, the idempotent stack close-out (change
# 0298), plus stack_descendants in scripts/lib/docket-stack.sh.
#
# NEW SUBSYSTEM, not an extension: the close-out drives the four shared terminal-close-out scripts
# per descendant and owns its own report vocabulary, so it gets its own shard rather than renting
# space in tests/test_docket_stack.sh (the library + resolver shard).
#
# THE ASSERT WITH THE TEETH is the idempotency probe. A run that promotes half a stack and dies
# leaves a DIRTY local tree behind, so "the tree is clean" is exactly the proxy a half-completed run
# falsifies; the probe therefore keys on the state the close-out PROMISED — the archived file on the
# metadata branch. The `remote-archived` fixture below is the only one that tells the two apart:
# every other fixture stays green with the probe mutated to a clean-tree check.
#
# The four close-out scripts are mocked through the SCRIPTS_DIR seam, as tests/test_docket_status.sh
# mocks the sweep's copies of the same four. archive-change.sh's mock is a real-ish one (it moves,
# commits and PUSHES) because the idempotency probe reads the metadata branch, and a mock that only
# logged would make that probe unobservable.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/stack-closeout.sh"
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

tmp="$(mktemp -d "${TMPDIR:-/tmp}/stack-closeout.XXXXXX")"; trap 'rm -rf "$tmp"' EXIT

# --- fixture: a bare origin plus a clone standing in for the metadata worktree -------------------
git init -q -b main "$tmp/seed"
git -C "$tmp/seed" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init
git clone -q --bare "$tmp/seed" "$tmp/origin.git"
git -C "$tmp/origin.git" config remote.origin.url "$tmp/seed"
git clone -q "$tmp/origin.git" "$tmp/work" 2>/dev/null
mw="$tmp/work"
CD="$mw/docs/changes"
mkdir -p "$CD/active" "$CD/archive" "$mw/docs/adrs"
git -C "$mw" config user.email t@t
git -C "$mw" config user.name t

# seed_active ID SLUG STATUS STACKED_ON TITLE — one active change file.
# Every body carries a prose `stacked_on:` line: an unanchored read would pick it up, so the whole
# fixture set doubles as a pin that the descendant scan reads frontmatter ONLY.
seed_active(){
  cat > "$CD/active/$(printf '%04d' "$1")-$2.md" <<EOF
---
id: $1
slug: $2
title: $5
status: $3
priority: high
depends_on: []
stacked_on: $4
branch: feat/$2
pr: $1
---

Body. Prose here may legitimately discuss stacked_on: 999 without being frontmatter.
EOF
}

# seed_archived ID SLUG DATE TITLE — an already-terminal change, the shape a stack ROOT is in by the
# time the close-out runs for it.
seed_archived(){
  cat > "$CD/archive/$3-$(printf '%04d' "$1")-$2.md" <<EOF
---
id: $1
slug: $2
title: $4
status: done
priority: high
depends_on: []
stacked_on:
branch: feat/$2
pr: $1
---

Body of the root record.
EOF
}

# --- family A: the happy path, rooted at 50 -------------------------------------------------------
seed_archived 50 root 2026-08-12 "Root change"
seed_active 51 child-one stacked-merged 50 "Child one"
seed_active 52 child-two stacked-merged 50 "Child two"
seed_active 53 grand-child stacked-merged 51 "Grand child"
seed_active 54 unrelated implemented "" "Unrelated change"
# A descendant that is NOT stacked-merged: this pass's business is the stacked-merged set only, and
# without this fixture the status gate could be deleted with every assert staying green.
seed_active 55 sibling-implemented implemented 50 "Sibling still implemented"
# A frontmatter `stacked_on:` written the way a human writes it — PADDED, extra spaces, trailing
# inline comment. The descendant scan prefilters on the KEY's shape before reading values, and this
# is the fixture that pins the prefilter as a superset of what fm_field can answer: narrow the
# prefilter to a value shape and this change disappears from the graph.
cat > "$CD/active/0057-padded-child.md" <<'EOF'
---
id: 57
slug: padded-child
title: Padded child
status: stacked-merged
priority: high
depends_on: []
stacked_on:   0050   # padded, and annotated by hand
branch: feat/padded-child
pr: 57
---

Body.
EOF
# Frontmatter says unstacked; only the BODY mentions the root. It must never enter the graph.
cat > "$CD/active/0059-body-only.md" <<'EOF'
---
id: 59
slug: body-only
title: Body only mention
status: stacked-merged
priority: high
depends_on: []
stacked_on:
branch: feat/body-only
pr: 59
---

This change discusses the stack in prose:
stacked_on: 50
EOF

# --- family B: the partial run, rooted at 60 ------------------------------------------------------
seed_archived 60 second-root 2026-08-13 "Second root"
seed_active 61 b-child-one stacked-merged 60 "B child one"
seed_active 62 b-child-two stacked-merged 60 "B child two"
seed_active 63 b-grand-child stacked-merged 61 "B grand child"

# --- family C: a root whose Stack carried markers are malformed -----------------------------------
cat > "$CD/archive/2026-08-14-0070-marker-root.md" <<'EOF'
---
id: 70
slug: marker-root
title: Marker root
status: done
priority: high
depends_on: []
stacked_on:
branch: feat/marker-root
pr: 70
---

Body above.

<!-- docket:stack-carried:end -->
| #9999 | Hand-written | — |
<!-- docket:stack-carried:start (generated — do not hand-edit) -->

Body below, which an unbounded range would eat.
EOF
seed_active 71 c-child stacked-merged 70 "C child"

# --- family D: a cyclic stacked_on graph ----------------------------------------------------------
seed_active 80 d-root stacked-merged 82 "D root"
seed_active 81 d-child stacked-merged 80 "D child"
seed_active 82 d-grand stacked-merged 81 "D grand"

# --- family E: the render failure that abandons an expected publish -------------------------------
seed_archived 90 e-root 2026-08-15 "E root"
seed_active 91 e-child stacked-merged 90 "E child"

git -C "$mw" add docs
git -C "$mw" commit -q -m "seed stack close-out changes"
git -C "$mw" push -q origin main

# --- the close-out mocks, via the SCRIPTS_DIR seam ------------------------------------------------
mkdir -p "$tmp/mock-scripts"
calllog="$tmp/closeout-calls.log"
: > "$calllog"

# Real-ish: performs the dated move, commits and PUSHES, so the metadata branch actually carries the
# archived file the idempotency probe reads.
cat > "$tmp/mock-scripts/archive-change.sh" <<'EOF'
#!/usr/bin/env bash
echo "archive-change $*" >> "$CLOSEOUT_LOG"
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
# The real script's reuse-existing-archive probe: already archived is an idempotent no-op, which is
# what makes the whole sequence safe to re-run from the top on a resume.
existing="$(find "$changes_dir/archive" -maxdepth 1 -name "*-${pad}-*.md" | sed -n 1p)"
[ -z "$existing" ] || exit 0
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
git -C "$root" -c user.email=t@t -c user.name=t commit -q -m "mock archive $id" >/dev/null 2>&1
git -C "$root" push -q origin main >/dev/null 2>&1
exit 0
EOF
cat > "$tmp/mock-scripts/render-change-links.sh" <<'EOF'
#!/usr/bin/env bash
echo "render-change-links $*" >> "$CLOSEOUT_LOG"
cf=""
while [ $# -gt 0 ]; do
  case "$1" in --change-file) cf="$2"; shift ;; esac
  shift
done
case "$cf" in *"-${FAIL_RENDER_PAD:-nevermatches}-"*) exit 1 ;; esac
# Idempotent, like the real renderer: writes the block once, then is a no-diff no-op. A mock that
# wrote nothing would make the follow-on commit unobservable.
grep -qF "docket:artifacts:start" "$cf" || printf '\n%s\n%s\n' \
  "<!-- docket:artifacts:start (generated — do not hand-edit) -->" \
  "<!-- docket:artifacts:end -->" >> "$cf"
exit 0
EOF
cat > "$tmp/mock-scripts/terminal-publish.sh" <<'EOF'
#!/usr/bin/env bash
echo "terminal-publish $*" >> "$CLOSEOUT_LOG"
id=""
while [ $# -gt 0 ]; do
  case "$1" in --id) id="$2"; shift ;; esac
  shift
done
[ "$id" = "${FAIL_PUBLISH_ID:-none}" ] && exit 1
exit 0
EOF
cat > "$tmp/mock-scripts/cleanup-feature-branch.sh" <<'EOF'
#!/usr/bin/env bash
echo "cleanup-feature-branch $*" >> "$CLOSEOUT_LOG"
exit 0
EOF
cat > "$tmp/mock-scripts/mark-publish-deferred.sh" <<'EOF'
#!/usr/bin/env bash
echo "mark-publish-deferred $*" >> "$CLOSEOUT_LOG"
cf=""
while [ $# -gt 0 ]; do
  case "$1" in --change-file) cf="$2"; shift ;; esac
  shift
done
[ -n "$cf" ] || exit 1
# The real script REPLACES an existing section rather than appending a second one; the marker is
# presence-encoded state, so the mock has to actually write it or the resume path is untestable.
grep -qxF '## Publish deferred' "$cf" || printf '\n## Publish deferred\n\nblocked\n' >> "$cf"
exit 0
EOF
cat > "$tmp/mock-scripts/render-artifact-backlink.sh" <<'EOF'
#!/usr/bin/env bash
echo "render-artifact-backlink $*" >> "$CLOSEOUT_LOG"
exit 0
EOF
chmod +x "$tmp/mock-scripts/"*.sh

# run_closeout ROOT_ID DATE [extra args...] — stdout captured by the caller, stderr to the log file.
run_closeout(){
  local root="$1" date="$2"; shift 2
  ( cd "$mw" && SCRIPTS_DIR="$tmp/mock-scripts" CLOSEOUT_LOG="$calllog" \
    "$DOCKET_BASH_PATH" "$SCRIPT" \
      --changes-dir "$CD" --root-id "$root" --date "$date" \
      --integration-branch main --metadata-branch main \
      --adrs-dir docs/adrs --terminal-publish false "$@" ) 2>>"$tmp/closeout-err.txt"
}

# --- run 1: the happy path ------------------------------------------------------------------------
out="$tmp/out1.txt"
run_closeout 50 2026-08-12 > "$out"; rc1=$?
root_archived="$CD/archive/2026-08-12-0050-root.md"

assert "every stacked-merged descendant is promoted" \
  'grep -qF "promoted 51" "$out" && grep -qF "promoted 52" "$out" && grep -qF "promoted 53" "$out"'
assert "the happy-path pass exits 0" '[ "$rc1" = 0 ]'
assert "a padded, inline-commented stacked_on still joins the graph" \
  'grep -qF "promoted 57" "$out"'
assert "a descendant that is not stacked-merged is skipped, not promoted" \
  'grep -qF "promote-skipped 55" "$out" && ! grep -qF "promoted 55" "$out"'
assert "a body-prose stacked_on mention never enters the graph" \
  '! grep -qE "^promote(d|-skipped|-failed) 59( |$)" "$out"'
assert "an unrelated change is untouched" \
  '[ -f "$CD/active/0054-unrelated.md" ]'
assert "a promoted descendant is archived with the root merge date" \
  '[ -f "$CD/archive/2026-08-12-0051-child-one.md" ]'
assert "a still-implemented descendant stays in active/" \
  '[ -f "$CD/active/0055-sibling-implemented.md" ]'

# The shared close-out sequence, in order, for one descendant. Ordering is load-bearing: the
# ## Artifacts re-render must land BEFORE the publish, or the publish copies a stale block.
seq51="$(grep -F -- "-0051-" "$calllog"; grep -F -- "--id 51 " "$calllog"; grep -F -- "--slug child-one" "$calllog")"
assert "the descendant ran archive, re-render, publish and cleanup" \
  'grep -qF "archive-change" <<<"$seq51" && grep -qF "render-change-links" <<<"$seq51" && \
   grep -qF "terminal-publish" <<<"$seq51" && grep -qF "cleanup-feature-branch" <<<"$seq51"'
ord="$(grep -nE "render-change-links .*-0051-|terminal-publish .*--id 51 " "$calllog")"
assert "the artifacts re-render precedes the publish for that descendant" \
  'r="$(grep -F "render-change-links" <<<"$ord" | sed -n 1p | cut -d: -f1)"; \
   p="$(grep -F "terminal-publish" <<<"$ord" | sed -n 1p | cut -d: -f1)"; \
   [ -n "$r" ] && [ -n "$p" ] && [ "$r" -lt "$p" ]'
assert "the publish is gated by --enabled, never invoked bare" \
  'pub="$(grep -F "terminal-publish " "$calllog")"; grep -qF -- "--enabled false" <<<"$pub"'
assert "the re-render is committed as its own follow-on commit" \
  'subjects="$(git -C "$mw" log --format=%s -20)"; grep -qF "refresh artifacts links" <<<"$subjects"'

# --- the root's Stack carried table ---------------------------------------------------------------
assert "the root record carries the Stack carried table" \
  'grep -qF "| Stack carried |" "$root_archived" || grep -qF "## Stack carried" "$root_archived"'
assert "the table lists every descendant" \
  'grep -qF "#0051" "$root_archived" && grep -qF "#0053" "$root_archived"'
assert "the table carries each descendant title" \
  'grep -qF "Grand child" "$root_archived"'
assert "the table is marker-bounded" \
  'grep -qF "docket:stack-carried:start" "$root_archived" && grep -qF "docket:stack-carried:end" "$root_archived"'
assert "the table write is reported" 'grep -qE "^stack-carried 50 " "$out"'
assert "the table landed as a commit on the metadata branch" \
  'blob="$(git -C "$mw" show "origin/main:docs/changes/archive/2026-08-12-0050-root.md")"; \
   [ -z "$(git -C "$mw" status --porcelain -- "$root_archived")" ] && \
   grep -qF "docket:stack-carried:start" <<<"$blob"'
assert "no stray temp file is left beside the record" \
  '[ -z "$(find "$CD/archive" -maxdepth 1 -name ".stack-carried.*" 2>/dev/null)" ]'

# --- run 2: idempotency ---------------------------------------------------------------------------
out2="$tmp/out2.txt"
run_closeout 50 2026-08-12 > "$out2"; rc2=$?
assert "a second run is a no-op" \
  '[ -z "$(grep -F "promoted 51" "$out2")" ]'
assert "a second run still exits 0" '[ "$rc2" = 0 ]'
assert "a second run names the archived file as the reason it skipped" \
  'grep -qF "promote-skipped 51 already-archived" "$out2"'
assert "a second run does not duplicate the Stack carried block" \
  '[ "$(grep -cF "docket:stack-carried:start" "$root_archived")" = 1 ]'
assert "a second run re-runs no close-out step for an archived descendant" \
  'a="$(grep -F "archive-change " "$calllog")"; [ "$(grep -cF -- "--id 51 " <<<"$a")" = 1 ]'

# --- THE DISCRIMINATING FIXTURE: archived on the metadata branch, dirty local tree -----------------
# A half-completed run is exactly this state, and it is the ONLY fixture that separates the promised
# state (the archived file on the metadata branch) from the local proxy (a clean tree). Key the probe
# on a clean tree instead and this reddens: the tree is dirty, so the mutant concludes "not done yet"
# and promotes a change that is already archived on the remote.
seed_active 56 remote-archived stacked-merged 50 "Remote archived child"
git -C "$mw" add docs/changes/active/0056-remote-archived.md
git -C "$mw" commit -q -m "seed the remote-archived fixture"
git -C "$mw" push -q origin main
git clone -q "$tmp/origin.git" "$tmp/other" 2>/dev/null
git -C "$tmp/other" config user.email t@t
git -C "$tmp/other" config user.name t
git -C "$tmp/other" mv docs/changes/active/0056-remote-archived.md \
                      docs/changes/archive/2026-08-12-0056-remote-archived.md
git -C "$tmp/other" commit -q -m "another agent archived 0056"
git -C "$tmp/other" push -q origin main
# This worktree never pulled, so locally 0056 is still active — and the tree is dirty BOTH globally
# (an untracked scratch file) and on the change's own path (an uncommitted edit), so neither a
# whole-tree nor a path-scoped clean-tree proxy survives this fixture.
printf 'scratch\n' > "$mw/half-finished-run.tmp"
printf '\nA half-written edit.\n' >> "$CD/active/0056-remote-archived.md"
out4="$tmp/out4.txt"
run_closeout 50 2026-08-12 > "$out4"; rc4=$?
assert "a descendant archived on the metadata branch is not re-promoted from a dirty tree" \
  '! grep -qF "promoted 56" "$out4"'
assert "that descendant is reported as already archived" \
  'grep -qF "promote-skipped 56 already-archived" "$out4"'
assert "the dirty-tree pass still exits 0" '[ "$rc4" = 0 ]'
rm -f "$mw/half-finished-run.tmp"
git -C "$mw" checkout -- docs/changes/active/0056-remote-archived.md
git -C "$mw" pull -q --rebase >/dev/null 2>&1 || true

# --- family B: the partial run is completed by the next -------------------------------------------
# Publishing is EXPECTED here (a distinct integration branch, the knob on), which is what makes the
# abandoned publish leave the durable `## Publish deferred` marker — and the marker is what tells the
# next pass that this archived record is half-finished rather than closed out. Under suppression
# there is no expected publish, hence nothing to resume; that asymmetry is the contract, not a gap.
out3a="$tmp/out3a.txt"
FAIL_PUBLISH_ID=62 run_closeout 60 2026-08-13 --terminal-publish true \
  --integration-branch mainline > "$out3a"; rc3a=$?
assert "a per-descendant failure does not abandon its siblings" \
  'grep -qF "promoted 61" "$out3a" && grep -qF "promoted 63" "$out3a"'
assert "the failing descendant is reported failed, naming the step" \
  'grep -qF "promote-failed 62 terminal-publish" "$out3a"'
assert "a pass in which every descendant reached a verdict exits 0" '[ "$rc3a" = 0 ]'
assert "the failed descendant's branch is NOT cleaned up" \
  '! grep -qF -- "--slug b-child-two" "$calllog"'
assert "the abandoned publish left the durable marker on the archived record" \
  'grep -qxF "## Publish deferred" "$CD/archive/2026-08-13-0062-b-child-two.md"'
out3="$tmp/out3.txt"
run_closeout 60 2026-08-13 --terminal-publish true --integration-branch mainline > "$out3"
assert "a partial run is completed by the next" \
  'grep -qF "promoted 62" "$out3"'
# THE DISCRIMINATING HALF of the resume rule: 61 is archived too, and the ONLY thing separating it
# from 62 is the marker. Resume on the archived file alone and this reddens — every completed
# promotion would be re-run, forever.
assert "the completing run leaves its already-promoted siblings alone" \
  '! grep -qF "promoted 61" "$out3" && grep -qF "promote-skipped 61 already-archived" "$out3"'

# --- family C: malformed markers refuse, and leave the file untouched -----------------------------
marker_root="$CD/archive/2026-08-14-0070-marker-root.md"
before="$(cksum < "$marker_root")"
out5="$tmp/out5.txt"
run_closeout 70 2026-08-14 > "$out5"; rc5=$?
after="$(cksum < "$marker_root")"
assert "out-of-order markers leave the record byte-identical" '[ "$before" = "$after" ]'
assert "the marker refusal is reported" 'grep -qF "stack-carried-failed 70 markers-unbalanced" "$out5"'
assert "the refusal does not abandon the promotions" 'grep -qF "promoted 71" "$out5"'
assert "the refusing pass still exits 0" '[ "$rc5" = 0 ]'

# --- family D: a cyclic graph terminates and emits each id once -----------------------------------
cyc="$tmp/cycle-out.txt"
( "$DOCKET_BASH_PATH" -c '
    . "$1"/scripts/lib/docket-frontmatter.sh
    . "$1"/scripts/lib/docket-stack.sh
    stack_descendants "$2" 80
  ' _ "$REPO" "$CD" > "$cyc" 2>/dev/null; printf 'done\n' > "$cyc.done" ) &
cyc_waited=0
while [ ! -f "$cyc.done" ] && [ "$cyc_waited" -lt 40 ]; do sleep 0.5; cyc_waited=$((cyc_waited + 1)); done
assert "a cyclic stacked_on graph terminates" '[ -f "$cyc.done" ]'
assert "a cycle emits each descendant exactly once, and never the root" \
  '[ "$(sort "$cyc" | uniq | grep -c .)" = 2 ] && ! grep -qx "80" "$cyc"'

# --- family E: a failed re-render abandons an expected publish, and says so durably ----------------
out6="$tmp/out6.txt"
FAIL_RENDER_PAD=0091 run_closeout 90 2026-08-15 --terminal-publish true \
  --integration-branch mainline > "$out6"; rc6=$?
assert "a failed artifacts re-render skips the publish" \
  'pub="$(grep -F "terminal-publish " "$calllog")"; \
   grep -qF "promote-failed 91 render-change-links" "$out6" && ! grep -qF -- "--id 91 " <<<"$pub"'
assert "the abandoned publish is marked durably" \
  'grep -qF "mark-publish-deferred" "$calllog"'
assert "the marker names the descendant's archived file" \
  'm="$(grep -F "mark-publish-deferred" "$calllog")"; grep -qF -- "-0091-" <<<"$m"'
assert "the abandoning pass still exits 0" '[ "$rc6" = 0 ]'

# --- usage and pass-level failure -----------------------------------------------------------------
( cd "$mw" && "$DOCKET_BASH_PATH" "$SCRIPT" --changes-dir "$CD" --root-id 50 ) >/dev/null 2>&1
assert "a missing required flag is a usage error (exit 2)" '[ $? -eq 2 ]'
( cd "$mw" && "$DOCKET_BASH_PATH" "$SCRIPT" --changes-dir "$CD" --root-id 999 --date 2026-08-12 \
    --integration-branch main --metadata-branch main --adrs-dir docs/adrs \
    --terminal-publish false ) >/dev/null 2>&1
assert "an unknown root is a pass-level failure (exit 1)" '[ $? -eq 1 ]'
help_txt="$("$DOCKET_BASH_PATH" "$SCRIPT" --help 2>&1)"
assert "--help prints its own header" '[ -n "$(grep -F stack-closeout.sh <<<"$help_txt")" ]'

# --- the facade exposes the op --------------------------------------------------------------------
assert "the docket.sh facade wraps stack-closeout" \
  'grep -qE "^WRAPPED_OPS=.*[\" ]stack-closeout[\" ]" "$REPO/scripts/docket.sh"'
assert "the facade contract documents the op" \
  'grep -qF "| stack-closeout |" "$REPO/scripts/docket.md" || grep -q "stack-closeout" "$REPO/scripts/docket.md"'

# --- every DESIGNED INVOKER of the op is wired ------------------------------------------------------
# Spec §7 names TWO invokers of this pass — the `docket-status` merge sweep AND
# `docket-finalize-change` — and the sweep provably cannot cover for finalize: `detect_merged`
# enumerates `active/` changes at `implemented`, so once finalize archives a root that root is never
# re-enumerated, and a `stacked-merged` descendant has no merged PR of its own to detect. A finalize
# that never runs the op therefore strands every descendant PERMANENTLY — the single worst outcome
# this script exists to prevent — so the wiring is guarded here, in the op's own shard, alongside the
# facade asserts above. The sweep's own invoker (`sweep_stack_closeout`) is executable and is covered
# by tests/test_docket_status_stack.sh; finalize's is PROSE, which nothing else in the suite reads.
#
# KEYED ON SHAPE, DERIVED, NOT ENUMERATED: the required flag set comes from stack-closeout.sh's own
# whole-flag-set validation block, and the report vocabulary from its own header table. A flag or a
# report line added to the script that the documented invocation never learns about reddens these
# asserts on arrival, which a hand-listed copy here could not do.
FIN="$REPO/skills/docket-finalize-change/SKILL.md"
STACKREF="$REPO/skills/docket-convention/references/stacked-changes.md"
# Required flags: the `missing --X` refusals plus the --terminal-publish value check. Optional flags
# (--remote, --help) name themselves nowhere in that block, which is exactly the discriminator.
closeout_validation="$(grep -E 'die "missing --|must be true or false' "$SCRIPT")"
REQUIRED_FLAGS="$(grep -oE -e '--[a-z][a-z-]+' <<<"$closeout_validation" | sort -u)"
# Report vocabulary: the `#   <token> <...>` rows of the script's own report table.
REPORT_TOKENS="$(awk '/^#   [a-z][a-z-]+ </{print $2}' "$SCRIPT" | sort -u)"
assert "the required-flag derivation found a plausible flag set" \
  '[ "$(grep -c . <<<"$REQUIRED_FLAGS")" -ge 6 ]'
assert "the report-vocabulary derivation found a plausible token set" \
  '[ "$(grep -c . <<<"$REPORT_TOKENS")" -ge 4 ]'

# The reference section that OWNS the invocation, whitespace-collapsed so a re-flow of the prose
# cannot redden a phrase match (the section runs to the next `## ` heading or EOF).
closeout_section="$(awk '/^## The stack close-out is idempotent/{s=1;next} s&&/^## /{exit} s' "$STACKREF")"
closeout_flat="$(tr -s '[:space:]' ' ' <<<"$closeout_section")"
assert "the reference's close-out section exists and is non-empty" '[ -n "${closeout_flat// /}" ]'
# The fence literal is held in a SINGLE-quoted awk variable: no backtick may sit inside double
# quotes in test source (change 0221, scripts/check-test-source-hygiene.sh).
closeout_block="$(awk -v f='```' 'index($0,f)==1{b=!b;next} b' <<<"$closeout_section")"
assert "the reference's close-out section carries a fenced facade invocation of the op" \
  'grep -qF "docket.sh stack-closeout" <<<"$closeout_block"'
for flag in $REQUIRED_FLAGS; do
  assert "the reference's close-out command names $flag" \
    'grep -qF -- "$flag" <<<"$closeout_block"'
done
for token in $REPORT_TOKENS; do
  assert "the reference's close-out section names the report line $token" \
    'grep -qF -- "$token" <<<"$closeout_flat"'
done

# Finalize: RETIRED (0316, category (a)). The stack close-out was a separate `docket.sh stack-closeout`
# facade call the finalize skill sequenced BETWEEN `docket.sh archive-change` and
# `docket.sh cleanup-feature-branch` (this block asserted that line order). Root-carry archiving is
# now absorbed into the Go `docket finalize closeout` verb: its `root-archived` disposition "archives
# the root and every descendant using the root's merge date" in one transaction, and its
# `stacked-merged` disposition marks a child in place until the root lands. Authority #2:
# `docket finalize closeout` owns root-carry archiving — the skill no longer sequences bash facade
# calls, so the archive→stack-closeout→cleanup line-order machinery is retired with them. Guard
# re-pointed at the closeout verb owning the stack dispositions.
fin_flat="$(tr -s '[:space:]' ' ' < "$FIN")"
assert "finalize's Go closeout verb owns the stack root-carry (root-archived)" \
  'grep -qF "docket finalize closeout" <<<"$fin_flat" && grep -qF "root-archived" <<<"$fin_flat"'
assert "finalize's closeout retains a stacked-merged child until the root lands" \
  'grep -qF "stacked-merged" <<<"$fin_flat"'

printf '%s\n' "--- done"
exit "$fail"
