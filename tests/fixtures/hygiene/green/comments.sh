#!/usr/bin/env bash
# GREEN FIXTURE: backticks in comments are inert. The repo's prose is full of them — `$SKILL_PLAN`,
# `## Run halted`, `assert(){` — and flagging them would make the guard unusable.
# A comment mid-line is also fine: see below.
printf 'x\n'   # trailing prose about `git checkout .`
