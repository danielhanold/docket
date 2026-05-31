#!/usr/bin/env bash
# sync-convention.sh — keep the embedded "## Convention" block byte-identical across
# all docket skills. Canonical source: docket-new-change/SKILL.md.
#
#   bash sync-convention.sh           # propagate the canonical block into the others
#   bash sync-convention.sh --check   # exit 0 if all in sync; 1 (and list drift) if not
#
# Exit codes: 0 ok · 1 drift (--check) · 2 setup error (canonical/markers missing).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_DIR="$SCRIPT_DIR/skills"
CANONICAL="${DOCKET_CANONICAL_SKILL:-docket-new-change}"
BEGIN='<!-- docket:convention:begin -->'
END='<!-- docket:convention:end -->'

extract() {  # print the block inclusive of its markers from file $1
  awk -v b="$BEGIN" -v e="$END" '$0==b{g=1} g{print} $0==e{g=0}' "$1"
}

src="$SKILLS_DIR/$CANONICAL/SKILL.md"
[ -f "$src" ] || { echo "canonical not found: $src" >&2; exit 2; }

blockfile="$(mktemp)"; cleanup_extra=""; trap 'rm -f "$blockfile" $cleanup_extra' EXIT
extract "$src" > "$blockfile"
[ -s "$blockfile" ] || { echo "no convention block in canonical $src" >&2; exit 2; }

mode="${1:-sync}"
status=0
for f in "$SKILLS_DIR"/*/SKILL.md; do
  [ "$f" = "$src" ] && continue
  cur="$(extract "$f")"
  if [ "$cur" = "$(cat "$blockfile")" ]; then
    continue
  fi
  if [ "$mode" = "--check" ]; then
    echo "DRIFT: $f"
    status=1
    continue
  fi
  # sync mode: the markers must already be present (skills are authored with them).
  if ! grep -qF "$BEGIN" "$f" || ! grep -qF "$END" "$f"; then
    echo "markers missing in $f — add the convention markers before syncing" >&2
    exit 2
  fi
  tmp="$(mktemp "$f.XXXXXX")"; cleanup_extra="$tmp"
  awk -v b="$BEGIN" -v e="$END" -v rf="$blockfile" '
    $0==b { while ((getline line < rf) > 0) print line; close(rf); skip=1; next }
    $0==e { skip=0; next }
    !skip { print }
  ' "$f" > "$tmp"
  mv "$tmp" "$f"; cleanup_extra=""
  echo "synced $f"
done

if [ "$mode" = "--check" ] && [ "$status" -eq 0 ]; then
  echo "convention in sync"
fi
exit $status
