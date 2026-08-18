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

# ============================================================================
# Task A2 — delegation, argument passing, and exit propagation.
# ============================================================================

# A stub `docket` that advertises compatibility for the --help probe, then on the real
# delegation call echoes its argv so a test can prove word boundaries were preserved.
echo_stub(){ # <dir>
  cat > "$1/docket" <<'EOF'
#!/bin/sh
case "$*" in
  "development install --help"|"development --help") echo "install  bootstrap the development binary"; exit 0;;
esac
printf 'ARGC=%s\n' "$#"
for a in "$@"; do printf 'ARG=[%s]\n' "$a"; done
EOF
  chmod +x "$1/docket"
}

# A compatible stub whose real delegation call exits non-zero, to prove exit propagation.
exit7_stub(){ # <dir>
  cat > "$1/docket" <<'EOF'
#!/bin/sh
case "$*" in
  "development install --help"|"development --help") echo "install"; exit 0;;
esac
exit 7
EOF
  chmod +x "$1/docket"
}

# A minimal PATH bin dir so `docket` is genuinely ABSENT (the real one is on the ambient
# PATH and must never be shadowed-behind or executed). install.sh needs only `dirname`
# externally to resolve its own checkout dir; `go` is added when the test wants it present.
minbin(){ # <yes|no for go>
  d="$(mk)"
  ln -s "$(command -v dirname)" "$d/dirname"
  if [ "$1" = yes ]; then ln -s "$(command -v go)" "$d/go"; fi
  printf '%s\n' "$d"
}

# --- Case A2.a: absent docket + go present selects the go-run delegate ----------------------
# /bin/sh by absolute path: PATH is stripped to `minbin`, so it must not govern finding sh itself.
mb="$(minbin yes)"
out="$( cd / && PATH="$mb" DOCKET_BOOTSTRAP_DRY_RUN=1 /bin/sh "$REPO_ROOT/install.sh" )"; rc=$?
assert "A2.a go-run dry-run exits 0" '[ "$rc" = "0" ]'
assert "A2.a absent docket + go present -> go run delegate carries --source <checkout>" \
  'grep -qF -- "go run ./cmd/docket development install --source $REPO_ROOT" <<<"$out"'

# --- Case A2.b: a checkout path containing a space is passed as ONE argv word ---------------
sp="$(mktemp -d "${TMPDIR:-/tmp}/it sp.XXXXXX")"; _tmpdirs+=("$sp")
cp "$REPO_ROOT/install.sh" "$sp/install.sh"
sp_phys="$(cd "$sp" && pwd -P)"   # install.sh canonicalises its own dir the same way
tmp="$(mk)"; echo_stub "$tmp"
out="$( PATH="$tmp:$PATH" sh "$sp/install.sh" )"; rc=$?
assert "A2.b spaced checkout -> exactly 4 delegated argv words" \
  'grep -qF "ARGC=4" <<<"$out"'
assert "A2.b spaced checkout -> --source value is one word with the space intact" \
  'grep -qF "ARG=[$sp_phys]" <<<"$out"'

# --- Case A2.c: no docket and no go -> non-zero exit with an actionable Go message ----------
mb="$(minbin no)"
out="$( cd / && PATH="$mb" /bin/sh "$REPO_ROOT/install.sh" 2>&1 )"; rc=$?
assert "A2.c missing go on the no-binary path -> non-zero exit" '[ "$rc" != "0" ]'
assert "A2.c missing go -> actionable 'Go toolchain' message" \
  'grep -qi "go toolchain" <<<"$out"'

# --- Case A2.d: the delegated command's non-zero exit is propagated unchanged ---------------
tmp="$(mk)"; exit7_stub "$tmp"
out="$( cd / && PATH="$tmp:$PATH" sh "$REPO_ROOT/install.sh" 2>&1 )"; rc=$?
assert "A2.d delegated exit 7 is propagated as install.sh's exit" '[ "$rc" = "7" ]'

# --- Case A2.e: supported passthrough flags forwarded, in order, after --source -------------
tmp="$(mk)"; compat_stub "$tmp"
out="$( cd / && PATH="$tmp:$PATH" DOCKET_BOOTSTRAP_DRY_RUN=1 sh "$REPO_ROOT/install.sh" \
        --bin-dir /opt/docket/bin --harness claude --harness codex )"; rc=$?
assert "A2.e passthrough dry-run exits 0" '[ "$rc" = "0" ]'
assert "A2.e --bin-dir and repeatable --harness forwarded in order after --source" \
  'grep -qF -- "docket development install --source $REPO_ROOT --bin-dir /opt/docket/bin --harness claude --harness codex" <<<"$out"'

# --- Case A2.f: an unsupported flag is refused, not forwarded blind ------------------------
tmp="$(mk)"; compat_stub "$tmp"
out="$( cd / && PATH="$tmp:$PATH" DOCKET_BOOTSTRAP_DRY_RUN=1 sh "$REPO_ROOT/install.sh" --frobnicate 2>&1 )"; rc=$?
assert "A2.f unsupported flag -> non-zero exit" '[ "$rc" != "0" ]'
assert "A2.f unsupported flag -> the offending flag is named" \
  'grep -qF -- "--frobnicate" <<<"$out"'

if [ "$fail" -ne 0 ]; then exit 1; fi
