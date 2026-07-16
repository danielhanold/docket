#!/usr/bin/env bash
# tests/test_docket_approve_template.sh — structural checks on the shipped approve-workflow
# template (change 0062). No network; a static-content audit (grep sentinels). Run: bash tests/test_docket_approve_template.sh
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
TPL="$ROOT/scripts/templates/docket-approve.yml"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "template file exists"                 '[ -f "$TPL" ]'
assert "workflow_dispatch trigger"            'grep -q "workflow_dispatch" "$TPL"'
assert "required pr input"                    'grep -Eq "pr:" "$TPL" && grep -q "required: true" "$TPL"'
assert "job-scoped pull-requests: write"      'grep -Eq "pull-requests:[[:space:]]*write" "$TPL"'
assert "guard: open state"                    'grep -q "OPEN" "$TPL"'
assert "guard: draft rejected"                'grep -qi "draft" "$TPL"'
assert "guard: fork rejected"                 'grep -qiE "fork|isCrossRepository" "$TPL"'
assert "guard: feat/* head shape"             'grep -q "feat/\*" "$TPL"'
assert "approves via gh pr review --approve"  'grep -q "gh pr review" "$TPL" && grep -q -- "--approve" "$TPL"'
assert "uses GITHUB_TOKEN"                    'grep -q "GITHUB_TOKEN" "$TPL"'
# a static template must NOT hardcode a specific repo/owner (kept byte-identical across installs)
assert "no hardcoded repo owner"              '! grep -qiE "danielhanold/docket" "$TPL"'

exit $fail
