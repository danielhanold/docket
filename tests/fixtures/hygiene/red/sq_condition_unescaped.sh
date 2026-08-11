#!/usr/bin/env bash
# RED FIXTURE: source quoting protects the FIRST evaluation; eval strips that protection.
assert "demo" 'printf "%s\n" "`printf EXECUTED`"'
