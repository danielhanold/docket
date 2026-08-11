#!/usr/bin/env bash
# RED FIXTURE: change 0212's actual incident shape — a MULTI-LINE double-quoted assignment.
# A per-line scanner reports nothing here; that is the regression this fixture pins.
SITES="
skills/docket-implement-next/SKILL.md
  anchor: run `git checkout .` to discard the pending claim
scripts/run-tests.sh
"
printf '%s\n' "$SITES"
