#!/usr/bin/env bash
# GREEN FIXTURE: a backslash-newline continuation inside a double-quoted string, and a `#` that is
# NOT a comment because it does not begin a word. Neither may desynchronize the state machine.
msg="first line \
second line"
url='https://example.invalid/path#fragment'
printf '%s %s\n' "$msg" "$url"
