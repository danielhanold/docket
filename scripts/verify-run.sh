#!/usr/bin/env bash
# scripts/verify-run.sh — the mechanical consumer of docket's terminal-disposition contract
# (change 0237), behind `docket.sh verify-run`. Evaluates docket-implement-next's **Step 7
# postcondition** for one change and reports a verdict on stdout; or, in --in-progress-ids mode,
# prints the ids of every in-progress change so a caller can diff the set across a hand-off.
#
# PURE READER. Git and filesystem only — no network, no `gh`, no harness, no status writes, no
# file writes, no claim release. The only thing that ACTS on a verdict is runner-dispatch.sh.
#
# Usage: verify-run.sh <id> [--changes-dir DIR]
#        verify-run.sh --in-progress-ids [--with-claimed-at] [--changes-dir DIR]
#        verify-run.sh --build --worktree DIR --branch NAME --since SHA
#   Build-family verdict lines (change 0271; a build task's terminal state is a COMMIT, not a PR):
#     task-committed <branch>                 on-branch, tip advanced, tree clean
#     task-incomplete <branch> <unmet…>       tokens: branch tip tree
#     task-unverifiable <reason>              the worktree could not be read — never a guess
#   `task-committed` proves CLEAN COMPLETION, never semantic success.
#   --with-claimed-at widens each snapshot line to `<id> <claimed_at-epoch>`, or `<id> -` when the
#   change carries no `claimed_at:` or one that does not parse. This script stays the SINGLE owner
#   of frontmatter reading for the run gate: runner-dispatch.sh needs the claim instant to tell its
#   own claim from a foreign one, and it gets it from here rather than growing a second reader.
#   Verdict lines (one, on stdout):
#     run-complete <id>                    every conjunct holds
#     run-halted <id>                      a `## Run halted` record is present — deliberate stop
#     run-incomplete <id> <unmet…>         one or more conjuncts unmet (tokens: status pr branch)
#     run-unclaimed <id>                   not in-progress and not implemented — no run to verify
#   Exit 0 WHENEVER A VERDICT WAS PRODUCED. `run-incomplete` is a FINDING, not a script failure:
#   a bare non-zero consumer must not read it as one (LEARNINGS: exit-code-encodes-a-non-failure).
#   Non-zero (2) only when the check itself could not run — bad usage, unknown id, unreadable
#   change file, unresolvable config, not a repo.
#   NO TIME FLOOR. Sound only because of WHERE this is called: at a seam where the child process
#   has already returned, so "stopped" and "still working" are not ambiguous. board-checks.sh
#   cannot make that assumption and therefore keeps its floors — it is deliberately untouched.
#   Mock seams: GIT="${GIT:-git}", CONFIG_EXPORT_CMD (config resolution).
set -uo pipefail
GIT="${GIT:-git}"
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

die(){ printf 'verify-run: %s\n' "$*" >&2; exit 2; }

ID=""; CHANGES_DIR=""; MODE="verdict"; WITH_CLAIMED_AT=0
BUILD_WORKTREE=""; BUILD_BRANCH=""; BUILD_SINCE=""
while [ $# -gt 0 ]; do
  case "$1" in
    --in-progress-ids) MODE="ids" ;;
    --with-claimed-at) WITH_CLAIMED_AT=1 ;;
    --changes-dir) CHANGES_DIR="${2:-}"; shift ;;
    --build) MODE="build" ;;
    --worktree) BUILD_WORKTREE="${2:-}"; shift ;;
    --branch) BUILD_BRANCH="${2:-}"; shift ;;
    --since) BUILD_SINCE="${2:-}"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) die "unknown argument: $1" ;;
    *) [ -z "$ID" ] || die "unexpected extra argument: $1"; ID="$1" ;;
  esac
  shift
done
[ "$WITH_CLAIMED_AT" = 0 ] || [ "$MODE" = "ids" ] || die "--with-claimed-at is only valid with --in-progress-ids"
[ "$MODE" != "ids" ] || [ -z "$ID" ] || die "an <id> cannot be combined with --in-progress-ids"
[ "$MODE" != "build" ] || [ -z "$ID" ] || die "an <id> cannot be combined with --build"
if [ "$MODE" = "build" ]; then
  [ -n "$BUILD_WORKTREE" ] || die "--build requires --worktree"
  [ -n "$BUILD_BRANCH" ]   || die "--build requires --branch"
  [ -n "$BUILD_SINCE" ]    || die "--build requires --since"
fi

# --- the BUILD verdict family (change 0271) -----------------------------------
# A SECOND family, deliberately not the implement-next conjuncts stretched to fit: a build
# task's terminal state is a commit on its feature branch, never a PR. Reads a WORKTREE; it
# needs no changes dir, which is why it returns above the resolver.
#
# NAMING: `task-committed`, never `task-complete`. These three conjuncts prove the task ran
# to its commit and stranded nothing. They do NOT certify the commit implements the plan task
# correctly — that judgment stays with docket-build's suite gate and the review role, and the
# verdict name must not let a caller read more into it than it claims.
if [ "$MODE" = "build" ]; then
  # Every failure to READ is `task-unverifiable`, never a synthesized incompleteness: a
  # missing worktree is an absence of evidence, and reporting it as unmet conjuncts would
  # be a guess wearing a verdict's clothes.
  [ -d "$BUILD_WORKTREE" ] \
    || { printf 'task-unverifiable worktree-missing\n'; exit 0; }
  "$GIT" -C "$BUILD_WORKTREE" rev-parse --git-dir >/dev/null 2>&1 \
    || { printf 'task-unverifiable not-a-repo\n'; exit 0; }

  bunmet=()
  # 1. on the expected branch
  cur="$("$GIT" -C "$BUILD_WORKTREE" rev-parse --abbrev-ref HEAD 2>/dev/null)"
  [ "$cur" = "$BUILD_BRANCH" ] || bunmet+=(branch)
  # 2. the tip advanced past the dispatch-time sha. `--since` is the direct analogue of
  #    DISPATCH_EPOCH: captured after the before-read, so a commit landing in the gap is
  #    excluded either way. An unresolvable since-sha is unverifiable, not "advanced".
  tip="$("$GIT" -C "$BUILD_WORKTREE" rev-parse HEAD 2>/dev/null)"
  if ! "$GIT" -C "$BUILD_WORKTREE" cat-file -e "${BUILD_SINCE}^{commit}" 2>/dev/null; then
    printf 'task-unverifiable unknown-since-sha\n'; exit 0
  fi
  [ -n "$tip" ] && [ "$tip" != "$BUILD_SINCE" ] || bunmet+=(tip)
  # 3. the working tree is clean — INCLUDING untracked files. The stranded-work case this
  #    whole change exists for (change 0258) left its +64 lines UNTRACKED, so a
  #    tracked-only check would have called that run clean.
  dirty="$("$GIT" -C "$BUILD_WORKTREE" status --porcelain 2>/dev/null)"
  [ -z "$dirty" ] || bunmet+=(tree)

  if [ "${#bunmet[@]}" -eq 0 ]; then
    printf 'task-committed %s\n' "$BUILD_BRANCH"; exit 0
  fi
  printf 'task-incomplete %s %s\n' "$BUILD_BRANCH" "${bunmet[*]}"
  exit 0
fi

# --- changes dir: an explicit flag, else the resolver -------------------------
# Resolving here (rather than making every caller pass it) is what makes `docket.sh verify-run <id>`
# a usable hand command. The flag exists for hermetic tests and for runner-dispatch.sh, which has
# already resolved a repo root of its own.
if [ -z "$CHANGES_DIR" ]; then
  cfg="$(${CONFIG_EXPORT_CMD:-"${DOCKET_BASH_PATH:-bash}" "$SELF_DIR/docket-config.sh" --export})" \
    || die "config export failed"
  eval "$cfg"
  case "${BOOTSTRAP:-}" in
    PROCEED) : ;;
    STOP_MIGRATE)  die "repo not migrated — run migrate-to-docket.sh" ;;
    CREATE_ORPHAN) die "fresh repo — run docket.sh bootstrap to create the docket branch" ;;
    *) die "unknown bootstrap verdict '${BOOTSTRAP:-}'" ;;
  esac
  # The resolver exports `CHANGES_DIR` as a REPO-RELATIVE value (verified against
  # `docket-config.sh --export`) — it CLOBBERS the flag variable, so capture it before use. There
  # is no `REPO_ROOT` in the export; `docket_metadata_worktree` is the one anchor that turns the
  # relative `METADATA_WORKTREE` into an absolute tree and handles the non-`docket` mode too.
  rel="${CHANGES_DIR:-docs/changes}"
  # shellcheck source=/dev/null
  source "$SELF_DIR/lib/docket-root.sh"
  base="$(docket_metadata_worktree)"
  [ -n "$base" ] || die "could not resolve the metadata worktree"
  CHANGES_DIR="$base/$rel"
fi
[ -d "$CHANGES_DIR" ] || die "changes dir not found: $CHANGES_DIR"

# shellcheck source=/dev/null
source "$SELF_DIR/lib/docket-frontmatter.sh"

if [ "$MODE" = "ids" ]; then
  # The snapshot half. Numerically sorted so a caller's `comm`/diff is stable.
  #
  # --with-claimed-at adds the claim instant as epoch seconds. It is converted HERE, through the
  # shared `iso_to_epoch` (which already handles GNU and BSD `date`), so no caller has to own a
  # portable timestamp parse of its own — the consumer compares two integers. `claimed_at` is read
  # with the ANCHORED `fm_field`: the key is optional, and a change body discussing `claimed_at:`
  # in prose would otherwise be read as the value (LEARNINGS: frontmatter-anchored-read).
  # An absent or unparseable stamp prints `-`, never a number: "no positive evidence", the same
  # posture reclaim-claims.sh and board-checks.sh take on an unreadable lease.
  for f in "$CHANGES_DIR"/active/*.md; do
    [ -f "$f" ] || continue
    [ "$(fm_field "$f" status)" = "in-progress" ] || continue
    id="$(int_field "$f" id)"; [ -n "$id" ] || continue
    if [ "$WITH_CLAIMED_AT" = 1 ]; then
      claimed="$(fm_field "$f" claimed_at)"; epoch=""
      [ -n "$claimed" ] && epoch="$(iso_to_epoch "$claimed")"
      printf '%s %s\n' "$id" "${epoch:--}"
    else
      printf '%s\n' "$id"
    fi
  done | sort -n
  exit 0
fi

[ -n "$ID" ] || die "an <id> is required (or --in-progress-ids)"
case "$ID" in ''|*[!0-9]*) die "invalid id: $ID (must be a non-negative integer)" ;; esac
# CANONICALIZE before ANY arithmetic. Docket displays the padded form everywhere (filenames, board,
# commit scopes, "change 0237") and the validator above admits it — but bash reads a leading `0` as
# octal, so `0237` becomes 159 and `0219` is not a number at all. Base-10 forced, matching
# board-checks.sh / adr-checks.sh. Every later use — the `%04d` glob, the die text, and the verdict
# line — is this canonical value, so the id we PRINT is always the id we READ.
ID=$((10#$ID))

# Locate the change: active/ first, then archive/. An archived change is a legitimate
# `run-unclaimed` (terminal — there is no run to verify); a change that exists NOWHERE is a caller
# error and must not be reported as a benign verdict.
printf -v padded '%04d' "$ID"
FILE=""
for cand in "$CHANGES_DIR/active/$padded-"*.md "$CHANGES_DIR"/archive/*-"$padded-"*.md; do
  [ -f "$cand" ] && { FILE="$cand"; break; }
done
[ -n "$FILE" ] || die "no change file for id $ID under $CHANGES_DIR"
[ -r "$FILE" ] || die "change file is unreadable: $FILE"

# Every CONJUNCT read here is fm_field, never field: `pr:` and `branch:` are optional keys, and
# this repo's change bodies routinely open lines with them (LEARNINGS: frontmatter-anchored-read).
# One absent-key fixture and one mutation arm per read live in tests/test_verify_run.sh. The `id`
# read above (`int_field`, via the shared `field`) is UNANCHORED and has no absent-key fixture —
# `id:` is mandatory and always line 2 of the template, so the exposure is low-likelihood but real;
# tracked, not fixed, by change 0237 (see also change 0134's repo-wide accessor audit).
status="$(fm_field "$FILE" status)"
pr="$(fm_field "$FILE" pr)"
branch="$(fm_field "$FILE" branch)"

case "$status" in
  in-progress|implemented) : ;;
  *) printf 'run-unclaimed %s\n' "$ID"; exit 0 ;;
esac

# --- Step 7's postcondition, as three conjuncts ------------------------------
unmet=()
[ "$status" = "implemented" ] || unmet+=(status)
[ -n "$pr" ]                  || unmet+=(pr)
if [ -z "$branch" ] || ! "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$branch"; then
  unmet+=(branch)
fi

# ORDER IS DELIBERATE: a satisfied postcondition outranks a `## Run halted` record. The section is
# presence-encoded state whose removal is owned by docket-implement-next's Step 2 claim — which
# does NOT run on a resume — so a stale record can ride into a genuinely completed run. Checking
# the conjuncts first means a stale marker can never downgrade a complete run
# (LEARNINGS: presence-encoded-state — enumerate the readers, then decide).
if [ "${#unmet[@]}" -eq 0 ]; then
  printf 'run-complete %s\n' "$ID"; exit 0
fi
if has_section "$FILE" "## Run halted"; then
  printf 'run-halted %s\n' "$ID"; exit 0
fi
printf 'run-incomplete %s %s\n' "$ID" "${unmet[*]}"
exit 0
