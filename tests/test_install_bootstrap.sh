#!/usr/bin/env bash
# tests/test_install_bootstrap.sh — run: bash tests/test_install_bootstrap.sh
#
# install.sh is a POSIX bootstrapper (change 0322): run from any CWD it resolves its own
# checkout directory and chooses one of three paths by probing `docket` —
#   (a) a compatible installed `docket`      -> delegate `docket development install`
#   (b) no `docket` on PATH                   -> `go run ./cmd/docket development install`
#   (c) a `docket` present but whose development-install probe errors/does not name the
#       operation                             -> refuse non-zero, mutating nothing (never
#                                                fall through to the go-run path)
# DOCKET_BOOTSTRAP_DRY_RUN=1 prints the resolved command to stdout and exits 0.
#
# Task A1 covers source-resolution + tri-state discovery (delegate path + the refuse state).
# Delegation argv-safety, the go-run path, and legacy-primitive removal are asserted by A2/A3.
set -uo pipefail

# pwd -P: install.sh canonicalises its checkout dir the same way, so compare physical to physical.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

_tmpdirs=()
trap 'rm -rf "${_tmpdirs[@]}"' EXIT
mk(){ d="$(mktemp -d "${TMPDIR:-/tmp}/itest.XXXXXX")"; _tmpdirs+=("$d"); printf '%s\n' "$d"; }

# A compatible stub `docket`: its `development install --help` exits 0 and names the operation.
compat_stub(){ # <dir>
  cat > "$1/docket" <<'EOF'
#!/bin/sh
case "$*" in
  "development install --help"|"development --help") echo "install  bootstrap the development binary"; exit 0;;
  *) echo "docket stub: $*"; exit 0;;
esac
EOF
  chmod +x "$1/docket"
}

# --- Case A1.1: a compatible installed `docket` selects the delegate path -----------------
# Run from `/` with the stub on PATH; the dry-run must print the delegate command carrying the
# checkout resolved from $0 (NOT the caller's CWD).
tmp="$(mk)"; compat_stub "$tmp"
out="$( cd / && PATH="$tmp:$PATH" DOCKET_BOOTSTRAP_DRY_RUN=1 sh "$REPO_ROOT/install.sh" )"; rc=$?
assert "A1.1 dry-run exits 0 on the delegate path" '[ "$rc" = "0" ]'
assert "A1.1 compatible docket -> dry-run prints delegate command with CWD-independent --source" \
  'grep -qF -- "docket development install --source $REPO_ROOT" <<<"$out"'

# --- Case A1.2: docket present but its probe errors -> refuse (third state) ----------------
# A found-but-incompatible docket must fail non-zero and must NOT fall through to the go-run path.
tmp="$(mk)"
cat > "$tmp/docket" <<'EOF'
#!/bin/sh
exit 3
EOF
chmod +x "$tmp/docket"
out="$( cd / && PATH="$tmp:$PATH" DOCKET_BOOTSTRAP_DRY_RUN=1 sh "$REPO_ROOT/install.sh" 2>&1 )"; rc=$?
assert "A1.2 incompatible docket -> non-zero exit" '[ "$rc" != "0" ]'
assert "A1.2 incompatible docket -> does not fall through to go run" \
  '! grep -q "go run" <<<"$out"'

if [ "$fail" -ne 0 ]; then exit 1; fi
