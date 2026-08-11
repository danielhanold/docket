#!/usr/bin/env bash
# RED FIXTURE: the backslash is consumed at source evaluation, so $2 carries a BARE backtick
# into eval and it executes there.
assert "demo" "grep \`printf pattern\` file"
