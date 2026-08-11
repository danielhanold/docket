#!/usr/bin/env bash
# RED FIXTURE: spaced declaration. Shape-tolerant discovery must catch it; the allowlist must
# then reject it, because its body still interpolates through echo.
assert () { if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
