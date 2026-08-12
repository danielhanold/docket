#!/usr/bin/env bash
# scripts/stack-closeout.sh — the idempotent STACK CLOSE-OUT (change 0298, spec §7 and §10). When a
# stack ROOT's code reaches the integration branch, every descendant that has been sitting at
# `stacked-merged` becomes reachable from it too, and by the governing invariant those changes are
# now `done`. This script promotes them — each through the SHARED terminal close-out sequence, in
# its documented order — and then writes the root's **Stack carried** table.
#
# Usage:
#   stack-closeout.sh --changes-dir DIR --root-id N --date YYYY-MM-DD
#                     --integration-branch B --metadata-branch M --adrs-dir REL
#                     --terminal-publish true|false [--remote R]
#   --changes-dir      the docket changes directory (the parent of active/ and archive/), in the
#                      METADATA working tree; the tree it lives in is where every commit lands.
#   --root-id          the stack root whose merge triggered this pass; padded or bare.
#   --date             the root merge date in UTC — the terminal date every descendant is archived
#                      with. Never now(): the close-out is re-run, and a date derived from the clock
#                      makes two runs disagree about the same change's archive filename.
#   --adrs-dir         REPO-RELATIVE adrs directory (terminal-publish.sh's shape); the artifacts
#                      re-render is handed the absolute form composed from the worktree root.
#   --terminal-publish the resolved TERMINAL_PUBLISH knob, passed straight through to
#                      terminal-publish.sh's --enabled. No default: this script never decides it.
#   Mock seams: GIT="${GIT:-git}", SCRIPTS_DIR (the close-out helper dir), DOCKET_BASH_PATH.
#
# Report lines on STDOUT, one per descendant, plus at most one table line:
#   promoted <id>                     — the full close-out ran
#   promote-skipped <id> <reason>     — already-archived | not-stacked-merged | change-file-missing
#   promote-failed <id> <reason>      — archive | archived-file-not-found | render-change-links |
#                                       terminal-publish
#   stack-carried <root> <count>      — the root's table was regenerated (committed if it changed)
#   stack-carried-failed <root> <why> — root-not-archived | markers-unbalanced | render-failed |
#                                       commit-failed | push-failed
#
# IDEMPOTENCY — the one thing this script must get right. The no-op probe keys on the state the
# close-out PROMISED: the descendant's archived file present on the METADATA BRANCH. It deliberately
# does NOT key on any local proxy — a clean working tree above all — because a run that promoted
# half a stack and died leaves precisely a DIRTY tree behind, so "the tree is clean" is false exactly
# when resumption matters and true exactly when it does not. The probe reads the branch after a
# read-only fetch, so a promotion another agent (or an earlier crashed run) already landed is seen
# even when this worktree has not pulled it.
#
# Exit codes: 0 every descendant reached a verdict (including failed ones — each promotion is
# independently re-runnable, so a per-descendant failure never abandons its siblings) · 1 the PASS
# could not run (no changes dir, not a git worktree, unknown root) · 2 usage. Full contract:
# scripts/stack-closeout.md.
set -uo pipefail

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="${SCRIPTS_DIR:-$SCRIPTDIR}"
GIT="${GIT:-git}"

START_MARKER='<!-- docket:stack-carried:start (generated — do not hand-edit) -->'
END_MARKER='<!-- docket:stack-carried:end -->'

die(){ printf 'stack-closeout: %s\n' "$1" >&2; exit 2; }
fatal(){ printf 'stack-closeout: %s\n' "$1" >&2; exit 1; }
log(){ printf 'stack-closeout: %s\n' "$1" >&2; }

CHANGES_DIR="" ROOT_ID="" DATE="" INTEGRATION_BRANCH="" METADATA_BRANCH="" ADRS_DIR=""
TERMINAL_PUBLISH="" REMOTE="origin"
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) [ $# -ge 2 ] || die "--changes-dir needs a value"; CHANGES_DIR="$2"; shift ;;
    --root-id) [ $# -ge 2 ] || die "--root-id needs a value"; ROOT_ID="$2"; shift ;;
    --date) [ $# -ge 2 ] || die "--date needs a value"; DATE="$2"; shift ;;
    --integration-branch) [ $# -ge 2 ] || die "--integration-branch needs a value"; INTEGRATION_BRANCH="$2"; shift ;;
    --metadata-branch) [ $# -ge 2 ] || die "--metadata-branch needs a value"; METADATA_BRANCH="$2"; shift ;;
    --adrs-dir) [ $# -ge 2 ] || die "--adrs-dir needs a value"; ADRS_DIR="$2"; shift ;;
    --terminal-publish) [ $# -ge 2 ] || die "--terminal-publish needs a value"; TERMINAL_PUBLISH="$2"; shift ;;
    --remote) [ $# -ge 2 ] || die "--remote needs a value"; REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done

# Validate the WHOLE flag set before doing any work, so a caller fixing one usage error is not sent
# back for the next one a call later.
[ -n "$CHANGES_DIR" ] || die "missing --changes-dir"
[ -n "$ROOT_ID" ] || die "missing --root-id"
[ -n "$DATE" ] || die "missing --date"
[ -n "$INTEGRATION_BRANCH" ] || die "missing --integration-branch"
[ -n "$METADATA_BRANCH" ] || die "missing --metadata-branch"
[ -n "$ADRS_DIR" ] || die "missing --adrs-dir"
case "$TERMINAL_PUBLISH" in
  true|false) ;;
  *) die "--terminal-publish must be true or false (pass the resolved knob through)" ;;
esac
case "$ROOT_ID" in (*[!0-9]*) die "--root-id must be a change id, got: $ROOT_ID" ;; esac
# Canonicalize at the ARGUMENT boundary: ids arrive zero-padded from every docket surface, and bash
# reads a leading `0` as an octal prefix — `0237` would silently become 159 and `0008` would not
# parse at all. Same precedent as scripts/board-checks.sh and scripts/adr-checks.sh.
ROOT_ID=$(( 10#$ROOT_ID ))
ROOT_PAD="$(printf '%04d' "$ROOT_ID")"
[ -d "$CHANGES_DIR" ] || fatal "changes dir not found: $CHANGES_DIR"

# shellcheck source=lib/docket-frontmatter.sh
source "$SCRIPTDIR/lib/docket-frontmatter.sh"
# shellcheck source=lib/docket-stack.sh
source "$SCRIPTDIR/lib/docket-stack.sh"

WT="$("$GIT" -C "$CHANGES_DIR" rev-parse --show-toplevel)" || fatal "not a git worktree: $CHANGES_DIR"
# changes-dir path relative to the worktree root (git add/commit want worktree-relative paths).
# `pwd -P` because macOS mktemp yields /var/... where git rev-parse yields /private/var/...
REL_ABS="$(cd "$CHANGES_DIR" && pwd -P)"
REL="${REL_ABS#"$WT"/}"

stack_find_file "$CHANGES_DIR" "$ROOT_ID" >/dev/null || fatal "no change file for root id $ROOT_ID"

# A read-only fetch, so the idempotency probe below reads the metadata branch as it actually is —
# not as this worktree last saw it. Deliberately NOT a pull: a pull would mutate the working tree,
# and the half-completed-run state this script exists to resume is precisely a tree that cannot take
# one. Best-effort: an offline run falls back to whatever ref is already present.
"$GIT" -C "$WT" fetch --quiet "$REMOTE" "$METADATA_BRANCH" >/dev/null 2>&1 || \
  log "could not fetch $REMOTE/$METADATA_BRANCH — the archived-file probe reads the last-known ref"

META_REF=""
if "$GIT" -C "$WT" show-ref --verify --quiet "refs/remotes/$REMOTE/$METADATA_BRANCH"; then
  META_REF="$REMOTE/$METADATA_BRANCH"
elif "$GIT" -C "$WT" show-ref --verify --quiet "refs/heads/$METADATA_BRANCH"; then
  META_REF="$METADATA_BRANCH"
fi

# archived_on_metadata PAD — exit 0 iff an archived change file for PAD exists on the metadata
# branch. THE idempotency probe; see the header. With no resolvable metadata ref (a fresh repo with
# nothing pushed) it degrades to the archived path on disk — still the promised artifact, just read
# from the only place left, and never a clean-tree proxy.
archived_on_metadata(){
  local pad="$1" names
  if [ -z "$META_REF" ]; then
    names="$(find "$CHANGES_DIR/archive" -maxdepth 1 -name "*-$pad-*.md" 2>/dev/null)"
  else
    names="$("$GIT" -C "$WT" ls-tree -r --name-only "$META_REF" -- "$REL/archive" 2>/dev/null)"
  fi
  [ -n "$names" ] || return 1
  grep -qF -- "-$pad-" <<<"$names"
}

# publish_outstanding PAD — exit 0 iff the archived record carries the durable `## Publish deferred`
# marker, i.e. an EXPECTED publish this stack's close-out abandoned.
#
# This is the second half of the idempotency key, and it is what makes a half-finished promotion
# RESUMABLE rather than merely detectable. Archiving is the first step of the sequence, so a run
# that died at the publish leaves the change archived — indistinguishable, on the archived file
# alone, from one that completed. The marker is the difference, and it is real state on the metadata
# branch rather than another local proxy: ADR-0051's presence-encoding, written by the driver that
# abandoned the publish and stripped by terminal-publish.sh's own success path.
#
# Under suppression there is no expected publish, so nothing is ever marked and a re-run correctly
# skips: the residual there is a stale `## Artifacts` block, which is what the sweep documents too.
publish_outstanding(){
  local pad="$1" names name body
  if [ -z "$META_REF" ]; then
    name="$(find "$CHANGES_DIR/archive" -maxdepth 1 -name "*-$pad-*.md" 2>/dev/null | sed -n 1p)"
    [ -n "$name" ] || return 1
    body="$(cat "$name" 2>/dev/null)"
  else
    names="$("$GIT" -C "$WT" ls-tree -r --name-only "$META_REF" -- "$REL/archive" 2>/dev/null)"
    name="$(grep -F -- "-$pad-" <<<"$names" | sed -n 1p)"
    [ -n "$name" ] || return 1
    body="$("$GIT" -C "$WT" show "$META_REF:$name" 2>/dev/null)"
  fi
  grep -qxF '## Publish deferred' <<<"$body"
}

# mark_deferred ARCHIVED ID DETAIL — write the durable `## Publish deferred` marker on a change
# whose EXPECTED publish this pass abandoned, and land it on the metadata branch. The obligation is
# the shared close-out's ("a failed step-2 re-render abandons the publish for every driver, and
# every driver is required by this contract to mark there"); like the sweep, this driver discharges
# it in code rather than leaving it to a report line nobody reads.
#
# BEST-EFFORT toward the report stream — always returns 0 and never writes to stdout, so a failed
# mark degrades to exactly the pre-mark observable behavior. TRANSACTIONAL toward the shared
# worktree: on a failed add/commit the path is restored to HEAD (a dirty shared tree fails every
# later pass's `pull --rebase`, for every change), while a failed PUSH retains the commit, which is
# clean and self-heals on the next pass.
#
# NEVER UNDER SUPPRESSION (ADR-0051): when `terminal_publish` is false, or metadata and integration
# are the same branch (main-mode), the publish is a no-op that exits 0 — that is success, not a
# deferral, and marking it would manufacture a gap that does not exist.
mark_deferred(){
  local archived="$1" id="$2" detail="$3"
  [ "$TERMINAL_PUBLISH" = true ] || return 0
  [ "$METADATA_BRANCH" != "$INTEGRATION_BRANCH" ] || return 0
  "${DOCKET_BASH_PATH:?run docket/install.sh}" "$SCRIPTS_DIR"/mark-publish-deferred.sh --mode add \
    --change-file "$archived" --reason blocked --detail "$detail" \
    --integration-branch "$INTEGRATION_BRANCH" --id "$id" >/dev/null 2>&1 || return 0
  if "$GIT" -C "$WT" add -- "$archived" >/dev/null 2>&1 \
     && "$GIT" -C "$WT" commit -q -m "docket($id): mark terminal publish deferred (blocked)" -- "$archived" >/dev/null 2>&1; then
    "$GIT" -C "$WT" push >/dev/null 2>&1 || true
  else
    "$GIT" -C "$WT" checkout HEAD -- "$archived" >/dev/null 2>&1 || true
  fi
  return 0
}

# promote_one ID — run the shared terminal close-out for one descendant and print its verdict.
# ALWAYS returns 0: a per-descendant failure is a report line, never an abandonment of the rest of
# the stack, because each promotion is independently re-runnable and a stalled sibling helps nobody.
promote_one(){
  local id="$1" pad f slug status archived spec resume=0
  pad="$(printf '%04d' "$id")"
  if ! f="$(stack_find_file "$CHANGES_DIR" "$id")"; then
    printf 'promote-skipped %s change-file-missing\n' "$id"
    return 0
  fi
  # The probe comes FIRST, before the status read: a descendant archived by an earlier run (or by
  # another agent) may still be sitting in this worktree's active/ carrying `stacked-merged`, and
  # re-promoting it would re-archive a change that is already terminal on the metadata branch. An
  # archived record with an outstanding publish is the RESUMABLE case: the sequence runs again from
  # the top, which is safe because every step of it is idempotent (archive-change.sh's own
  # reuse-existing probe makes step 1 a no-op), and the status gate below is skipped because the
  # change is legitimately `done` on disk by then.
  if archived_on_metadata "$pad"; then
    if publish_outstanding "$pad"; then
      resume=1
    else
      printf 'promote-skipped %s already-archived\n' "$id"
      return 0
    fi
  fi
  if [ "$resume" -eq 0 ]; then
    status="$(field "$f" status)"
    if [ "$status" != stacked-merged ]; then
      printf 'promote-skipped %s not-stacked-merged\n' "$id"
      return 0
    fi
  fi
  slug="$(field "$f" slug)"

  # (1) Archive on the metadata branch, with the ROOT's merge date. archive-change.sh commits the
  # change file ONLY and pushes, which is what keeps two concurrent archivers tree-identical.
  if ! "${DOCKET_BASH_PATH:?run docket/install.sh}" "$SCRIPTS_DIR"/archive-change.sh \
        --changes-dir "$CHANGES_DIR" --id "$id" --outcome done --date "$DATE" \
        --message "docket($id): done — archived (status done, $DATE)" >&2; then
    printf 'promote-failed %s archive\n' "$id"
    return 0
  fi
  # Glob on the PAD, not on the date: archive-change.sh reuses an existing dated filename when the
  # change was already archived across a day boundary, so a date-pinned glob would miss it.
  archived="$(find "$CHANGES_DIR/archive" -maxdepth 1 -name "*-$pad-*.md" 2>/dev/null | sed -n 1p)"
  if [ -z "$archived" ]; then
    printf 'promote-failed %s archived-file-not-found\n' "$id"
    return 0
  fi

  # (2) Re-render the `## Artifacts` block on the ARCHIVED file, as its own follow-on commit pushed
  # BEFORE the publish. ORDERING IS LOAD-BEARING: terminal-publish.sh copies the change file from
  # the metadata branch, so publishing first would publish the stale block on the exact surface the
  # re-render targets.
  if ! "${DOCKET_BASH_PATH:?run docket/install.sh}" "$SCRIPTS_DIR"/render-change-links.sh \
        --change-file "$archived" --adrs-dir "$WT/$ADRS_DIR" >&2; then
    # The publish is now abandoned for this descendant and no later pass resumes it — once archived,
    # the change leaves active/ and every sweep scans active/ only. Mark it, so the gap is durable
    # instead of living in a report line. The clean-path precondition is this leg's alone: here the
    # path is provably clean of anything this run did (archive-change.sh committed it moments ago),
    # so a dirty path is some other actor's uncommitted state and must not be marked over.
    if [ -z "$("$GIT" -C "$WT" status --porcelain -- "$archived" 2>/dev/null)" ]; then
      mark_deferred "$archived" "$id" \
        "stack close-out: the artifacts re-render failed, so the publish was never attempted — re-render before publishing"
    fi
    printf 'promote-failed %s render-change-links\n' "$id"
    return 0
  fi
  # The spec's back-link goes stale on the active/ -> archive/ move, and the shared sequence
  # re-stamps it in this SAME follow-on commit. Skipped when there is no spec:, or when the recorded
  # path does not resolve in this worktree.
  spec="$(fm_field "$archived" spec)"
  if [ -n "$spec" ] && [ -f "$WT/$spec" ]; then
    "${DOCKET_BASH_PATH:?run docket/install.sh}" "$SCRIPTS_DIR"/render-artifact-backlink.sh \
      --artifact-file "$WT/$spec" --change-file "$archived" >&2 || \
      log "spec back-link re-render failed for $id — cosmetic, self-heals on the next render"
  fi
  # Commit only what changed, by explicit path: the metadata worktree is SHARED, so an unscoped add
  # would sweep a concurrent agent's staged work in under this run's message. A commit/push failure
  # here is COSMETIC — a stale link block, regenerable — and must not abandon the publish and the
  # cleanup, which is the sweep's carve-out in the shared sequence and this driver's posture too.
  local -a refresh_paths=("$archived")
  [ -n "$spec" ] && [ -f "$WT/$spec" ] && refresh_paths+=("$WT/$spec")
  if [ -n "$("$GIT" -C "$WT" status --porcelain -- "${refresh_paths[@]}" 2>/dev/null)" ]; then
    if ! "$GIT" -C "$WT" add -- "${refresh_paths[@]}" >&2 \
       || ! "$GIT" -C "$WT" commit -q -m "docket($id): refresh artifacts links" -- "${refresh_paths[@]}" >&2; then
      log "artifacts refresh commit failed for $id — cosmetic; continuing the close-out"
    elif ! "$GIT" -C "$WT" push >&2; then
      log "artifacts refresh push failed for $id — cosmetic; continuing the close-out"
    fi
  fi

  # (3) Publish the terminal record. Gated by the knob, passed straight through; a suppressed
  # publish is an exit-0 no-op and therefore SUCCESS, so steps 4 and 5 still run.
  if ! "${DOCKET_BASH_PATH:?run docket/install.sh}" "$SCRIPTS_DIR"/terminal-publish.sh \
        --id "$id" --outcome done --enabled "$TERMINAL_PUBLISH" \
        --integration-branch "$INTEGRATION_BRANCH" --metadata-branch "$METADATA_BRANCH" \
        --changes-dir "$REL" --adrs-dir "$ADRS_DIR" --metadata-worktree "$WT" \
        --message "docket($id): publish terminal record (done)" >&2; then
    # Reachable only on a NON-ZERO exit, which neither suppression can produce (both are exit-0
    # no-ops), so the never-mark-under-suppression rule holds without a second gate — mark_deferred
    # re-checks anyway. No clean-path precondition here, unlike the re-render leg: that script
    # strips the marker in this worktree before publishing and documents that the driver's defer
    # path re-marks, so refusing on a dirty path would leave the removal uncommitted.
    mark_deferred "$archived" "$id" "stack close-out: the publish step exited non-zero"
    printf 'promote-failed %s terminal-publish\n' "$id"
    return 0
  fi

  # (4) Tear the feature branch + worktree down. A failure here is reported to stderr and does not
  # unmake the promotion: the change IS done, and an orphaned branch is a janitorial residue.
  if ! "${DOCKET_BASH_PATH:?run docket/install.sh}" "$SCRIPTS_DIR"/cleanup-feature-branch.sh \
        --slug "$slug" >&2; then
    log "cleanup failed for $id ($slug) — the promotion stands; the branch is left behind"
  fi

  printf 'promoted %s\n' "$id"
  return 0
}

# markers_wellformed FILE — exit 0 iff the file carries AT MOST ONE well-formed stack-carried block:
# start before end, balanced, never nested. Presence is not enough (AGENTS.md): an unbounded range
# consumes to EOF, so a dangling start would make the rewrite eat the rest of the record.
markers_wellformed(){
  local f="$1" line depth=0 blocks=0
  while IFS= read -r line || [ -n "$line" ]; do
    case "$line" in
      '<!-- docket:stack-carried:start'*)
        depth=$(( depth + 1 )); blocks=$(( blocks + 1 ))
        [ "$depth" -le 1 ] || return 1 ;;
      '<!-- docket:stack-carried:end'*)
        depth=$(( depth - 1 ))
        [ "$depth" -ge 0 ] || return 1 ;;
    esac
  done < "$f"
  [ "$depth" -eq 0 ] && [ "$blocks" -le 1 ]
}

# write_stack_carried ROOT_FILE ID... — regenerate the root's marker-bounded Stack carried block and
# print the report line. Deterministic: same descendants, byte-identical block.
write_stack_carried(){
  local root_file="$1"; shift
  local id pad f title pr rows="" tmpf
  if ! markers_wellformed "$root_file"; then
    printf 'stack-carried-failed %s markers-unbalanced\n' "$ROOT_ID"
    return 0
  fi
  for id in "$@"; do
    pad="$(printf '%04d' "$id")"
    if ! f="$(stack_find_file "$CHANGES_DIR" "$id")"; then
      rows="$rows| #$pad | (change file missing) | — |"$'\n'
      continue
    fi
    title="$(field "$f" title)"
    pr="$(fm_field "$f" pr)"
    case "$pr" in (''|*[!0-9]*) pr="—" ;; (*) pr="#$pr" ;; esac
    rows="$rows| #$pad | ${title:-(untitled)} | $pr |"$'\n'
  done
  # Render to a temp file BESIDE the destination (same filesystem, so the rename is atomic) and gate
  # on exit status AND non-empty output before `mv -f` — never redirect a renderer into the file it
  # generates. The dot-prefixed name keeps a crashed run's residue out of every `*.md` glob.
  tmpf="$(mktemp "${root_file%/*}/.stack-carried.XXXXXX")" || {
    printf 'stack-carried-failed %s render-failed\n' "$ROOT_ID"; return 0; }
  # The block travels to awk in a FILE, never in `-v`: a `-v` assignment carrying a newline is
  # rejected outright by BSD awk ("newline in string"), and this block is inherently multi-line.
  local blockf
  blockf="$(mktemp "${TMPDIR:-/tmp}/stack-carried-block.XXXXXX")" || {
    rm -f "$tmpf"; printf 'stack-carried-failed %s render-failed\n' "$ROOT_ID"; return 0; }
  printf '%s\n\n## Stack carried\n\n| Change | Title | PR |\n|---|---|---|\n%s%s\n' \
    "$START_MARKER" "$rows" "$END_MARKER" > "$blockf"
  if ! awk -v bf="$blockf" '
       function emit(  line) { while ((getline line < bf) > 0) print line; close(bf) }
       index($0, "<!-- docket:stack-carried:start") == 1 { inblock = 1; emit(); seen = 1; next }
       index($0, "<!-- docket:stack-carried:end") == 1 { inblock = 0; next }
       inblock { next }
       { print }
       END { if (!seen) { print ""; emit() } }
     ' "$root_file" > "$tmpf" || [ ! -s "$tmpf" ]; then
    rm -f "$tmpf" "$blockf"
    printf 'stack-carried-failed %s render-failed\n' "$ROOT_ID"
    return 0
  fi
  rm -f "$blockf"
  mv -f "$tmpf" "$root_file"
  if [ -n "$("$GIT" -C "$WT" status --porcelain -- "$root_file" 2>/dev/null)" ]; then
    if ! "$GIT" -C "$WT" add -- "$root_file" >&2 \
       || ! "$GIT" -C "$WT" commit -q -m "docket($ROOT_ID): stack carried table" -- "$root_file" >&2; then
      "$GIT" -C "$WT" checkout HEAD -- "$root_file" >/dev/null 2>&1 || true
      printf 'stack-carried-failed %s commit-failed\n' "$ROOT_ID"
      return 0
    fi
    if ! "$GIT" -C "$WT" push >&2; then
      # The commit is clean and the next pass's `pull --rebase` carries it; resetting it would
      # re-open the write and re-report it forever.
      printf 'stack-carried-failed %s push-failed\n' "$ROOT_ID"
      return 0
    fi
  fi
  printf 'stack-carried %s %s\n' "$ROOT_ID" "$#"
}

# The graph is snapshotted ONCE, at call time and BEFORE any promotion — never from an earlier scan.
# That is what lets a child swept to `stacked-merged` moments ago, in the same sweep pass, be
# promoted in that same pass.
descendants=()
while IFS= read -r line; do
  [ -n "$line" ] || continue
  descendants+=("$line")
done < <(stack_descendants "$CHANGES_DIR" "$ROOT_ID")

for d in "${descendants[@]+"${descendants[@]}"}"; do
  promote_one "$d"
done

# The table describes what the root CARRIED, so a root with no stack has nothing to say and its
# record is left untouched rather than stamped with an empty block.
if [ "${#descendants[@]}" -gt 0 ]; then
  root_file="$(stack_find_file "$CHANGES_DIR" "$ROOT_ID")"
  case "$root_file" in
    */archive/*) write_stack_carried "$root_file" "${descendants[@]}" ;;
    *) printf 'stack-carried-failed %s root-not-archived\n' "$ROOT_ID" ;;
  esac
fi

exit 0
