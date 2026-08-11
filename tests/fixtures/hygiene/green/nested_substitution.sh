#!/usr/bin/env bash
# GREEN FIXTURE: a command substitution restarts quoting from scratch, even inside double quotes.
# A scanner that does not model that reads the INNER opening quote below as the CLOSING quote of
# the outer one and runs inverted from there to end of file — so it loses real violations exactly
# as readily as it invents false ones. This is not hypothetical: the shape below is
# tests/test_sync_agents_runners.sh's own, and it desynced this scanner into reporting 61 phantom
# heredoc hits over the following 300 lines before the nesting was modeled.
between="$(awk -v q="'" '
  !her && index($0, "<<" q) { her=1; d=$0; sub(/^.*<</, "", d); next }
  { print }
' "$0")"
# The desync is only visible downstream, so the fixture needs something downstream to be wrong
# about: this prose anchor — `git checkout .` — is inert in a comment, and must stay inert. A
# line-oriented scanner that has swallowed the awk program as a live heredoc body reports it.
printf '%s\n' "$between"

# A parenthesized group nested inside the substitution needs a frame of its own, or its closing
# paren pops the SUBSTITUTION early and everything after it reads as double-quoted. The
# single-quoted awk-ish literal below is then lexed as double-quoted text, and its backtick — inert
# data here — is reported as a live one.
grouped="$( (cd / && pwd) ; printf '%s' '/^`/ { print }' )"
printf '%s\n' "$grouped"
