#!/usr/bin/env bash
# tests/test_auto_approve_docs.sh — ship-the-knob-end-to-end wiring for finalize.auto_approve
# (change 0062). Run: bash tests/test_auto_approve_docs.sh
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "setup guide exists"                    '[ -f "$ROOT/docs/auto-approve-setup.md" ]'
assert "README links the setup guide"          'grep -q "auto-approve-setup.md" "$ROOT/README.md"'
assert ".docket.yml documents auto_approve"    'grep -q "auto_approve" "$ROOT/.docket.yml"'
assert "guide covers setup-auto-approve run"   'grep -q "setup-auto-approve" "$ROOT/docs/auto-approve-setup.md"'
assert "guide covers CODEOWNERS limitation"    'grep -qi "CODEOWNERS" "$ROOT/docs/auto-approve-setup.md"'
assert "guide covers workflow OAuth scope"     'grep -qi "workflow" "$ROOT/docs/auto-approve-setup.md"'

exit $fail
