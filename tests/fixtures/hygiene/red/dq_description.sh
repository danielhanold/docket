#!/usr/bin/env bash
# RED FIXTURE: bare backtick inside a double-quoted string executes at source evaluation.
assert "the skill says `git checkout .` in its guard anchor" 'true'
