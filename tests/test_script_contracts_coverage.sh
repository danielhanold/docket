#!/usr/bin/env bash
# tests/test_script_contracts_coverage.sh — every top-level scripts/<name>.sh has a co-located
# scripts/<name>.md contract, and every scripts/<name>.md has a live scripts/<name>.sh (change
# 0037). Existence audit ONLY — content fidelity rests on co-location + review + the convention's
# "prose is the contract" rule (mechanical prose-vs-bash checking is out of scope: flaky/gameable).
# Mirrors test_change_links_coverage.sh; the suite is the de-facto gate (no GitHub Actions CI).
# Scope: TOP-LEVEL scripts/*.sh only (the glob's * never matches /, so scripts/lib/*.sh sourced
# helpers are out — they are documented within their callers' contracts).
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
ok(){ printf 'ok   - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# (1) every top-level scripts/<name>.sh has a co-located scripts/<name>.md
for sh in "$ROOT"/scripts/*.sh; do
  [ -e "$sh" ] || continue
  base="$(basename "$sh" .sh)"
  if [ -f "$ROOT/scripts/$base.md" ]; then ok "contract present for $base.sh"; else no "missing scripts/$base.md for $base.sh"; fi
done

# (2) every scripts/<name>.md has a live scripts/<name>.sh (no orphaned contract)
for md in "$ROOT"/scripts/*.md; do
  [ -e "$md" ] || continue
  base="$(basename "$md" .md)"
  if [ -f "$ROOT/scripts/$base.sh" ]; then ok "script present for $base.md"; else no "orphaned scripts/$base.md (no $base.sh)"; fi
done

exit $fail
