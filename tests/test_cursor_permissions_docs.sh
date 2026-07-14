#!/usr/bin/env bash
# tests/test_cursor_permissions_docs.sh — guards for the Cursor permissions guide (change 0073).
# Structure only: these prove a claim is stamped / a set is complete / JSON parses. They CANNOT
# prove a classifier claim is TRUE — that is validated by the human reading docs/cursor/permissions.md
# against the spec's verification-log appendix at the merge gate (LEARNINGS, verify-the-claim family).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
PERMS_JSON="$REPO/docs/cursor/permissions.example.json"
SANDBOX_JSON="$REPO/docs/cursor/sandbox.example.json"
GUIDE="$REPO/docs/cursor/permissions.md"
CONV="$REPO/skills/docket-convention/SKILL.md"
FACADE_DOC="$REPO/scripts/docket.md"
README="$REPO/README.md"
fail=0
ok(){ echo "ok - $1"; }
no(){ echo "NOT OK - $1"; fail=1; }

# JSON parser seam: prefer jq, fall back to python3.
json_ok(){ # $1 = file
  if command -v jq >/dev/null 2>&1; then jq -e . "$1" >/dev/null 2>&1
  else python3 -m json.tool "$1" >/dev/null 2>&1; fi
}

# --- Assertion 1: both example JSON fragments parse -------------------------------------------
if json_ok "$PERMS_JSON"; then ok "permissions.example.json parses"; else no "permissions.example.json parses"; fi
if json_ok "$SANDBOX_JSON"; then ok "sandbox.example.json parses"; else no "sandbox.example.json parses"; fi

# --- Assertion 2: the fragment carries all four observed facade spellings ---------------------
# Canonical guard token is DERIVED from the convention (authoritative), never retyped here.
CANON="$(grep -oE '\$\{DOCKET_SCRIPTS_DIR:\?run docket/install\.sh\}' "$CONV")"
CANON="${CANON%%$'\n'*}"   # first match; no pipe-to-head (pipefail-safe)
if [ -n "$CANON" ]; then ok "canonical guard token derivable from convention"; else no "canonical guard token derivable from convention"; fi
if [ -n "$CANON" ] && grep -qF -- "$CANON" "$PERMS_JSON"; then ok "fragment carries canonical guarded spelling"; else no "fragment carries canonical guarded spelling"; fi
# The three remaining observed forms (short x2, absolute) have no feature-branch source of truth
# but the fragment itself — they are empirical observations from appendix §G. Assert each is present
# (mutation: drop one from the fragment -> reddens). $USER placeholder kept verbatim.
for form in \
  '\"${DOCKET_SCRIPTS_DIR:?}\"/docket.sh' \
  '\"${DOCKET_SCRIPTS_DIR:?}/docket.sh\"' \
  '/Users/$USER/dev/docket/scripts/docket.sh'; do
  if grep -qF -- "$form" "$PERMS_JSON"; then ok "fragment carries spelling: $form"; else no "fragment carries spelling: $form"; fi
done

exit $fail
