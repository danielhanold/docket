#!/usr/bin/env bash
# RED FIXTURE: the suite's own house idiom — an assert whose condition sits on a BACKSLASH-
# CONTINUATION line, indented. A backslash-newline is a SPLICE: both characters are removed and the
# next character is ordinary. A scanner that instead treats the continuation as an escape of that
# next character swallows the leading indent space, opens a spurious word, and the condition then
# lands at word index 3 — so the eval rule, which arms on index 2, never fires. Roughly 55% of the
# assert call sites in tests/ are written in exactly this two-line form, so that miss is not an edge
# case; it is the majority of the suite. The identical hazard on one line is
# tests/fixtures/hygiene/red/sq_condition_unescaped.sh, and it was always caught.
assert "the continuation form the suite writes everywhere" \
  'printf "%s\n" "`printf EXECUTED`"'
