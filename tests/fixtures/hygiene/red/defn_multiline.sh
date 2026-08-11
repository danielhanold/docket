#!/usr/bin/env bash
# RED FIXTURE: multiline declaration.
assert() {
  if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi
}
