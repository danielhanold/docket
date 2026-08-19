#!/usr/bin/env bash
# tests/test_configured_bash_finalize.sh — the finalize suite-command boundary is RETIRED (0316).
#
# History. Change 0132 gave the finalize skill a marker-delimited shell fragment
# (`<!-- configured-bash-finalize:start/end -->`) that published the suite command, and this file
# extracted that fragment and EXECUTED it hermetically — auto-detect routing, explicit-command
# passthrough, accumulate-past-failure, and the empty-suite refusal.
#
# Change 0316 removed the fragment. The finalize skill is now a sequencer over Go verbs: the local
# gate is composed into `docket finalize rebase`, which launches and observes a supervised run
# through `docket gate`. There is no shell fragment left to extract, so every executable assertion
# in the old file tested a boundary that no longer exists. Supporting evidence: the `runtime.bash`
# setting is classified `obsolete-setting` by the Go config reader ("selected the Bash
# implementation, which docket no longer ships; it is ignored").
#
# Why this file still exists rather than being deleted. Deleting a guard is how a regression hides,
# and 0316 does NOT remove Bash — its *Out of scope* defers "Bash fallback behavior", and change
# 0318 owns the hard cutover. So the guard is INVERTED instead: it now proves the boundary stayed
# retired. Re-introducing a published shell suite-command into the finalize skill reddens this file,
# which is exactly the regression worth catching while Bash still exists in the tree.
#
# When 0318 completes the Bash removal, this file should be deleted outright.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
FIN="$ROOT/skills/docket-finalize-change/SKILL.md"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# Non-vacuity first: an absent or empty SKILL.md would satisfy every `! grep` below while proving
# nothing. Anchor on a string the Go sequencer must carry.
assert "finalize SKILL.md exists and is non-empty" '[ -s "$FIN" ]'
assert "finalize SKILL.md is the Go sequencer (non-vacuity anchor)" \
  'grep -qF "docket finalize" "$FIN"'

# The retired boundary: no marker, in either half of the pair, and no executable fragment.
assert "finalize publishes no configured-bash start marker" \
  '! grep -qF -- "<!-- configured-bash-finalize:start -->" "$FIN"'
assert "finalize publishes no configured-bash end marker" \
  '! grep -qF -- "<!-- configured-bash-finalize:end -->" "$FIN"'
assert "finalize names no configured-bash marker family at all" \
  '! grep -qF -- "configured-bash-finalize" "$FIN"'

# The replacement is present: the gate is composed into the Go rebase verb, not a shell fragment.
assert "the local gate is composed into the Go rebase verb" \
  'grep -qiE "gate is composed into|finalize rebase" "$FIN"'

exit "$fail"
