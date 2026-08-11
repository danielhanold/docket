#!/usr/bin/env bash
# RED FIXTURE + SIDE-EFFECT SENTINEL: if anything ever EXECUTES this file instead of scanning it,
# the marker file appears and tests/test_assert_hygiene.sh reddens. Detection must not require
# execution.
assert "sentinel" "touch \`printf %s "$HYGIENE_SENTINEL_DIR/EXECUTED"\`"
