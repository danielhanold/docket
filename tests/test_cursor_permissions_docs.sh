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
# Full decorated canonical form (JSON-escaped quotes) — built from the derived token, not retyped.
CANON_FULL='\"'"$CANON"'\"/docket.sh'
if [ -n "$CANON" ] && grep -qF -- "$CANON_FULL" "$PERMS_JSON"; then ok "fragment carries full canonical decorated spelling"; else no "fragment carries full canonical decorated spelling"; fi
# The three remaining observed forms (short x2, absolute) have no feature-branch source of truth
# but the fragment itself — they are empirical observations from appendix §G. Assert each is present
# (mutation: drop one from the fragment -> reddens). $USER placeholder kept verbatim.
for form in \
  '\"${DOCKET_SCRIPTS_DIR:?}\"/docket.sh' \
  '\"${DOCKET_SCRIPTS_DIR:?}/docket.sh\"' \
  '/Users/$USER/dev/docket/scripts/docket.sh'; do
  if grep -qF -- "$form" "$PERMS_JSON"; then ok "fragment carries spelling: $form"; else no "fragment carries spelling: $form"; fi
done

# --- Assertion 3: the guide's fenced json blocks are byte-identical to the example files ------
# awk emits the body of the Nth ```json ... ``` fence (no fence lines). Reads to EOF (pipefail-safe).
extract_json_fence(){ # $1 = file, $2 = which (1-based)
  awk -v want="$2" '
    /^```json$/ { infence=1; n++; next }
    /^```$/ { if (infence) { infence=0 }; next }
    infence && n==want { print }
  ' "$1"
}
if diff <(extract_json_fence "$GUIDE" 1) "$PERMS_JSON" >/dev/null; then ok "guide permissions fence == permissions.example.json"; else no "guide permissions fence == permissions.example.json"; fi
if diff <(extract_json_fence "$GUIDE" 2) "$SANDBOX_JSON" >/dev/null; then ok "guide sandbox fence == sandbox.example.json"; else no "guide sandbox fence == sandbox.example.json"; fi

# --- Assertion 4: every troubleshooting entry carries a provenance stamp ----------------------
# Scope to the ## Troubleshooting section (awk reads to EOF; pipefail-safe).
TROUBLE="$(awk '/^## Troubleshooting$/{f=1;next} /^## /{f=0} f' "$GUIDE")"
ENTRIES="$(grep -cE '^### ' <<<"$TROUBLE")"
STAMPS="$(grep -cE '^\*\*Observed:\*\* Cursor 3\.11\.19' <<<"$TROUBLE")"
if [ "$ENTRIES" -gt 0 ] && [ "$ENTRIES" = "$STAMPS" ]; then ok "troubleshooting entries ($ENTRIES) all stamped ($STAMPS)"; else no "troubleshooting entries ($ENTRIES) all stamped ($STAMPS)"; fi

# --- Assertion 5: guide's never-allowlist set == scripts/docket.md Not-exposed set ------------
# Exposed op basenames from the Subcommand inventory rows: | `op` | ... |  -> op.sh
EXPOSED="$(grep -oE '^\| `[a-z-]+`' "$FACADE_DOC" | tr -d '|` ' | sed 's/$/.sh/' | sort -u)"
# Backtick *.sh tokens inside the ## Not exposed section (awk reads to EOF; pipefail-safe).
NOTEXP_SECTION="$(awk '/^## Not exposed$/{f=1;next} /^## /{f=0} f' "$FACADE_DOC")"
NOTEXP_RAW="$(grep -oE '`[a-z-]+\.sh`' <<<"$NOTEXP_SECTION" | tr -d '`' | sort -u)"
# Subtract exposed ops (drops board-refresh.sh) -> the true never-allowlist set.
NOTEXP="$(comm -23 <(printf '%s\n' "$NOTEXP_RAW") <(printf '%s\n' "$EXPOSED"))"
if [ -n "$NOTEXP" ]; then ok "not-exposed set derivable from docket.md"; else no "not-exposed set derivable from docket.md"; fi
missing=""
while IFS= read -r s; do
  [ -z "$s" ] && continue
  grep -qF -- "$s" "$GUIDE" || missing="$missing $s"
done <<<"$NOTEXP"
if [ -z "$missing" ]; then ok "guide names every not-exposed script"; else no "guide missing not-exposed scripts:$missing"; fi

# --- Assertion 6: README links the guide -----------------------------------------------------
if grep -qF -- '](docs/cursor/permissions.md)' "$README"; then ok "README links the guide"; else no "README links the guide"; fi

exit $fail
