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
#        verify-run.sh --in-progress-ids [--changes-dir DIR]
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

ID=""; CHANGES_DIR=""; MODE="verdict"
while [ $# -gt 0 ]; do
  case "$1" in
    --in-progress-ids) MODE="ids" ;;
    --changes-dir) CHANGES_DIR="${2:-}"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) die "unknown argument: $1" ;;
    *) [ -z "$ID" ] || die "unexpected extra argument: $1"; ID="$1" ;;
  esac
  shift
done

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
  for f in "$CHANGES_DIR"/active/*.md; do
    [ -f "$f" ] || continue
    [ "$(fm_field "$f" status)" = "in-progress" ] || continue
    id="$(int_field "$f" id)"; [ -n "$id" ] && printf '%s\n' "$id"
  done | sort -n
  exit 0
fi

[ -n "$ID" ] || die "an <id> is required (or --in-progress-ids)"
case "$ID" in ''|*[!0-9]*) die "invalid id: $ID (must be a non-negative integer)" ;; esac

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

# EVERY read is fm_field, never field: `pr:` and `branch:` are optional keys, and this repo's
# change bodies routinely open lines with them (LEARNINGS: frontmatter-anchored-read). One
# absent-key fixture and one mutation arm per read live in tests/test_verify_run.sh.
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
