#!/usr/bin/env bash
# RED FIXTURE: a continuation line whose first character — at COLUMN ZERO, with no indent to
# absorb the mistake — is the opening quote itself. Treating the spliced newline as an escape of
# that character eats the OPENING quote, so the CLOSING one opens a region instead and the machine
# runs INVERTED to end of file: every later violation of every class is then lost silently, which
# is a far worse failure than a false positive. The downstream double-quoted backtick below is what
# pins that the inversion did not happen — under the escape-and-consume reading it lands inside a
# phantom single-quoted region and is reported by nothing at all.
printf '%s %s\n' \
'a column-zero single-quoted continuation argument' \
"the anchor is `printf HAZARD` here"
