#!/usr/bin/env bash
# scripts/backfill-change-types.sh — apply a human-approved id->type mapping to ACTIVE change
# files (change 0127). Deterministic mechanics only: an interactive agent reads each active change
# and proposes a complete mapping, a human approves it as ONE decision, and this script validates
# and applies it (ADR-0012 — the model judges what, the script does the write). All files or none.
# Idempotent. It never reads or edits <changes-dir>/archive/: the archive is intentionally not
# backfilled.
#
# Usage: backfill-change-types.sh --changes-dir DIR --map ID=TYPE[,ID=TYPE...] [--dry-run]
#   --changes-dir DIR  the changes dir whose active/ holds the backlog (e.g. .docket/docs/changes)
#   --map PAIRS        comma-separated ID=TYPE assignments covering EVERY untyped active change
#   --dry-run          validate and report the file count; write nothing
#   -h, --help         print this header
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
# shellcheck source=/dev/null
. "$SELF_DIR/lib/docket-frontmatter.sh"   # fm_field + docket_change_type_is_reserved/_is_wellformed

CHANGES_DIR=""; MAP=""; DRY=0
die(){ printf 'backfill-change-types: %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir|--map)
      [ $# -ge 2 ] || die "$1 requires a value"
      case "$1" in
        --changes-dir) CHANGES_DIR="$2" ;;
        --map)         MAP="$2" ;;
      esac
      shift ;;
    --dry-run) DRY=1 ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done
[ -n "$CHANGES_DIR" ]        || die "missing --changes-dir"
[ -n "$MAP" ]                || die "missing --map"
[ -d "$CHANGES_DIR/active" ] || die "no active/ directory under $CHANGES_DIR"

# --- parse + validate the mapping -------------------------------------------------------------
# Every check below runs BEFORE any write. A helper that validated lazily would fail on entry N
# having already rewritten entries 1..N-1, which is precisely the half-migrated backlog the
# all-or-nothing contract exists to prevent.
declare -A WANT=()
# The mapping is ONE physical line by contract. `read` consumes a single line, so an embedded
# newline silently DISCARDS every assignment after it — validation included — and the loss is
# invisible: when the break lands inside the final type token the completeness check still passes
# and a wrong-but-well-formed value is written (`4=ch\nore` => `type: ch`), --dry-run reports it
# identically to a correct run, and the overwrite guard then refuses to repair it. A long map is
# exactly what a composing agent line-wraps, so reject the shape up front rather than downstream.
#
# Checking the WHOLE map rather than each parsed type is what makes the check reachable at all:
# every type token is a substring of $MAP, so a per-token control-character guard could only ever
# fire on input this one has already refused. There is deliberately no second copy below.
case "$MAP" in
  *[[:cntrl:]]*) die "--map contains a control character (an embedded newline?); pass the whole mapping as ONE physical line" ;;
esac
IFS=',' read -r -a _pairs <<< "$MAP"
for _p in "${_pairs[@]}"; do
  case "$_p" in
    *=*) : ;;
    *)   die "malformed --map entry '$_p' (expected ID=TYPE)" ;;
  esac
  _id="${_p%%=*}"; _ty="${_p#*=}"
  case "$_id" in ''|*[!0-9]*) die "malformed change id '$_id' in --map" ;; esac
  # `0007` and `7` name the same change. Filenames (`0007-slug.md`) and BOARD.md rows (`[0007]`)
  # both show the PADDED form, so an operator or agent building the map from either supplies it,
  # while `id:` frontmatter carries the bare integer — keying the two sides differently made a
  # live active change report as "not an active change (archived records are never reclassified)".
  # Canonicalize here, matching the zero-padding tolerance board-checks.sh already documents.
  _id=$((10#$_id))
  # The type arrives as an argument that an agent composed, so it is untrusted input: reject by
  # SHAPE (the token grammar), never by enumerating bad spellings. Control characters are already
  # refused for the whole map above — see that comment for why the check belongs there and not here.
  if docket_change_type_is_reserved "$_ty"; then
    die "type for id $_id is the reserved value '$_ty' (a config selector / query pseudo-value, never a stored type)"
  fi
  docket_change_type_is_wellformed "$_ty" \
    || die "type for id $_id must match [a-z][a-z0-9-]*, got '$_ty'"
  [ -z "${WANT[$_id]:-}" ] || die "duplicate assignment for id $_id"
  WANT["$_id"]="$_ty"
done

# --- resolve the ACTIVE population and the migration set --------------------------------------
declare -A FILE_OF=()
migration_ids=""
for f in "$CHANGES_DIR"/active/*.md; do
  [ -e "$f" ] || continue
  fid="$(field "$f" id)"
  case "$fid" in ''|*[!0-9]*) continue ;; esac
  fid=$((10#$fid))                                # canonical bare integer — see the --map parse
  FILE_OF["$fid"]="$f"
  [ -z "$(fm_field "$f" type)" ] && migration_ids="$migration_ids $fid"
done

for id in "${!WANT[@]}"; do
  [ -n "${FILE_OF[$id]:-}" ] \
    || die "id $id is not an active change (archived records are never reclassified)"
  existing="$(fm_field "${FILE_OF[$id]}" type)"
  if [ -n "$existing" ] && [ "$existing" != "${WANT[$id]}" ]; then
    die "id $id already has type '$existing'; refusing to overwrite it with '${WANT[$id]}'"
  fi
done
for id in $migration_ids; do
  [ -n "${WANT[$id]:-}" ] \
    || die "incomplete mapping: active change $id has no type and no assignment"
done

# --- apply: stage every rewrite, then install ---------------------------------------------------
# Staged into a scratch dir first and only moved into place once every file rewrote cleanly, so a
# failure partway through cannot leave the backlog half-migrated.
stage="$(mktemp -d)"; trap 'rm -rf "$stage"' EXIT
wrote=0
for id in "${!WANT[@]}"; do
  src="${FILE_OF[$id]}"
  [ "$(fm_field "$src" type)" = "${WANT[$id]}" ] && continue    # already applied — idempotent
  out="$stage/$(basename "$src")"
  # The value is written through awk's ENVIRON, never interpolated into a sed replacement, so `&`
  # and backreferences in it can never be reinterpreted. The edit is anchored to the FIRST
  # ---...--- block (AGENTS.md): `n` counts frontmatter delimiters, so a body line beginning
  # `type:` is untouchable. An existing EMPTY `type:` placeholder inside that block is filled in
  # place; otherwise the field is inserted just before the block's closing `---`.
  BF_TYPE="${WANT[$id]}" awk '
    BEGIN { val = ENVIRON["BF_TYPE"]; n = 0; done = 0 }
    /^---[[:space:]]*$/ {
      n++
      if (n == 2 && !done) { print "type: " val; done = 1 }
      print; next
    }
    n == 1 && !done && /^type:[[:space:]]*$/ { print "type: " val; done = 1; next }
    { print }
  ' "$src" > "$out" || die "rewrite failed for id $id"
  # Prove the write landed where it was meant to before trusting the staged file.
  [ "$(fm_field "$out" type)" = "${WANT[$id]}" ] \
    || die "post-write verification failed for id $id (type not set in the first frontmatter block)"
  wrote=$((wrote + 1))
done

if [ "$DRY" = 1 ]; then
  printf 'backfill-change-types: dry-run — %s file(s) would change\n' "$wrote"
  exit 0
fi
# Install is the only phase that mutates the backlog, and a bare loop of `mv` is NOT all-or-none:
# a failure at file k leaves 1..k-1 installed and k..N not — precisely the half-migrated backlog
# this script's contract, and its documented exit-code semantics ("install failure. Nothing was
# installed."), promise is impossible on ANY failure path. Staging protects the REWRITE phase only;
# nothing protected the install. So the install now carries its own undo: the current bytes of
# every target are copied aside first, and a failed `mv` restores whatever already landed before
# dying. `.backup` is a dot-directory, so the `*.md` globs below never pick it up.
backup="$stage/.backup"; mkdir -p "$backup" || die "could not create the rollback staging dir"
for out in "$stage"/*.md; do
  [ -e "$out" ] || continue
  base="$(basename "$out")"
  cp -p "$CHANGES_DIR/active/$base" "$backup/$base" \
    || die "could not stage a rollback copy of $base; nothing was installed"
done
installed=""
for out in "$stage"/*.md; do
  [ -e "$out" ] || continue
  base="$(basename "$out")"
  if ! mv "$out" "$CHANGES_DIR/active/$base"; then
    for undo in $installed; do
      cp -p "$backup/$undo" "$CHANGES_DIR/active/$undo" 2>/dev/null \
        || printf 'backfill-change-types: WARNING: rollback failed for %s — restore it from git\n' "$undo" >&2
    done
    die "install failed for $base; rolled back to the pre-run state"
  fi
  installed="$installed $base"
done
printf 'backfill-change-types: applied %s file(s)\n' "$wrote"
