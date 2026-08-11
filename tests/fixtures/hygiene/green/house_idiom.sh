#!/usr/bin/env bash
# GREEN FIXTURE: the suite's house idiom. eval sees the backslash and treats the backtick as a
# literal character, so this is safe and must stay legal.
assert "the block names the token" 'grep -qF "\`docket:backlink\`" "$f"'
