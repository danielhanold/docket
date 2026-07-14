#!/usr/bin/env bash
# tests/test_worktree_hooks_wiring.sh — change 0063: every docket-owned worktree-creation site calls
# disable-worktree-hooks.sh, and the worktree-free bootstrap does NOT. Structural (grep) audit, in the
# spirit of the spec's terminal-publish structural check. Run: bash tests/test_worktree_hooks_wiring.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Change 0068: the docket-status ensure path was extracted into the shared preflight lib
# (scripts/lib/docket-preflight.sh); the hook-disable now lives THERE, and both docket-status.sh and
# the docket.sh facade reach it by sourcing that lib — so the audit follows the call into the lib.
assert "shared preflight lib calls the helper" \
  "grep -q 'disable-worktree-hooks.sh' \"$REPO/scripts/lib/docket-preflight.sh\""
assert "docket-status ensure reaches the helper via the shared preflight lib" \
  "grep -q 'lib/docket-preflight.sh' \"$REPO/scripts/docket-status.sh\""
assert "docket.sh facade reaches the helper via the shared preflight lib" \
  "grep -q 'lib/docket-preflight.sh' \"$REPO/scripts/docket.sh\""
assert "migrate calls the helper" \
  "grep -q 'disable-worktree-hooks.sh' \"$REPO/migrate-to-docket.sh\""
assert "terminal-publish calls the helper" \
  "grep -q 'disable-worktree-hooks.sh' \"$REPO/scripts/terminal-publish.sh\""
# The worktree-free bootstrap must NOT wire it (there is no worktree to scope).
assert "docket-config bootstrap does NOT call the helper" \
  "! grep -q 'disable-worktree-hooks.sh' \"$REPO/scripts/docket-config.sh\""

if [ "$fail" -eq 0 ]; then echo "ALL PASS"; else echo "FAILURES"; fi
exit "$fail"
