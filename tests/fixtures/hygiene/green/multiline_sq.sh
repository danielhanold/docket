#!/usr/bin/env bash
# GREEN FIXTURE: a multi-line SINGLE-quoted region carrying backticks as data (an awk program,
# the shape scripts/ uses everywhere). Inert, and not an assert condition.
prog='
/^`/ { print "fenced" }
{ next }
'
printf '%s\n' "$prog"
