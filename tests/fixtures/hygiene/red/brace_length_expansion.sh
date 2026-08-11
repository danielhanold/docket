#!/usr/bin/env bash
# RED FIXTURE: an unquoted ${#…} must not truncate the scan of the rest of its physical line.
# A brace expansion is word text, not a command separator — but a lexer that hands `{` to the
# separator branch closes the word, and the very next character of ${#FILES[@]} is a `#`, which
# then reads as a comment start and abandons the line. Everything after it on that line goes
# unscanned, so the double-quoted description below — whose backtick the shell RUNS at source
# evaluation — is reported by nothing. The live tree writes n=${#FILES[@]} today
# (tests/test_comment_anchor_style.sh, tests/test_grep_portability.sh); nothing follows it on
# those lines yet, which is the only reason there is no live miss.
FILES=(a b)
n=${#FILES[@]}; assert "the anchor says `printf HAZ` verbatim" "[ $n -gt 0 ]"
