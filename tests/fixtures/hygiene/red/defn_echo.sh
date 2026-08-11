#!/usr/bin/env bash
# RED FIXTURE: canonical one-line shape but the pre-0221 echo body — the drift the allowlist
# exists to catch after normalization lands.
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
