#!/usr/bin/env bash
# RED FIXTURE: legacy backtick substitution in unquoted code position. Banned suite-wide; the
# repo has zero live uses and `$(…)` is the house style.
now=`date -u +%s`
printf '%s\n' "$now"
