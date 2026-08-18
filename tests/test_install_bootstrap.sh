#!/usr/bin/env bash
# tests/test_install_bootstrap.sh — run: bash tests/test_install_bootstrap.sh
#
# install.sh is a POSIX bootstrapper (change 0322): run from any CWD it resolves its own
# checkout directory and chooses one of three paths by probing `docket` —
#   (a) a compatible installed `docket`      -> delegate `docket development install`
#   (b) no `docket` on PATH                   -> `go run -C <checkout> ./cmd/docket development install`
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
assert "A2.a absent docket + go present -> go run delegate is anchored with -C <checkout> and carries --source <checkout>" \
  'grep -qF -- "go run -C $REPO_ROOT ./cmd/docket development install --source $REPO_ROOT" <<<"$out"'

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

# ============================================================================
# Task A3 — the go-run path resolves ./cmd/docket from any CWD (the anchor),
# and the bootstrapper runs NONE of the legacy install primitives.
# ============================================================================

# --- Case A3.a: the go-run path resolves ./cmd/docket when invoked from an unrelated CWD -----
# `go run ./cmd/docket` resolves the package RELATIVE TO the process CWD, so an unanchored
# bootstrapper run from any directory other than the checkout cannot find the package. install.sh
# must anchor the run to the checkout (`go run -C <checkout>`). This is a REAL (non-dry-run)
# invocation from `/`: a stub `go` emulates resolution — it honors the -C anchor by cd'ing there
# and proving the package path exists from that directory, and fails non-zero otherwise (exactly
# how the real toolchain fails when the package cannot be resolved). The property under test is
# resolution, so the stub checks the package DIRECTORY resolves, never an argv spelling.
resolve_go_dir="$(mk)"
ln -s "$(command -v dirname)" "$resolve_go_dir/dirname"
cat > "$resolve_go_dir/go" <<'EOF'
#!/bin/sh
[ "$1" = run ] || { echo "stub go: expected 'run', got '$1'" >&2; exit 2; }
shift
anchor=
if [ "$1" = -C ]; then anchor="$2"; shift 2; fi
if [ -z "$anchor" ]; then
  echo "stub go: no -C anchor; ./cmd/docket cannot resolve from $(pwd)" >&2
  exit 3
fi
cd "$anchor" || { echo "stub go: cannot enter anchor '$anchor'" >&2; exit 4; }
# "$1" is the package spec, e.g. ./cmd/docket — prove it resolves from the anchored CWD.
[ -d "$1" ] || { echo "stub go: package '$1' does not resolve under '$anchor'" >&2; exit 5; }
echo "RESOLVED $1 under $anchor"
EOF
chmod +x "$resolve_go_dir/go"
out="$( cd / && PATH="$resolve_go_dir" /bin/sh "$REPO_ROOT/install.sh" 2>&1 )"; rc=$?
assert "A3.a go-run path invoked from an unrelated CWD resolves ./cmd/docket -> exit 0" '[ "$rc" = "0" ]'
assert "A3.a go-run package resolves under the checkout, not the caller's CWD" \
  'grep -qF "RESOLVED ./cmd/docket under $REPO_ROOT" <<<"$out"'

# --- Case A3.b: the bootstrapper runs NONE of the four legacy install primitives -------------
# The legacy install.sh ran ensure-global-config.sh / link-skills.sh / sync-agents.sh /
# ensure-docket-env.sh. Guard on the PROPERTY that none executes — an executable shim for each on
# PATH TOUCHES a per-name sentinel when run, and we assert no sentinel appears — not by grepping
# install.sh for their names (repo learning byte-pattern-guard-matches-a-spelling: a name grep
# misses the file's own idiom for an indirect or aliased call).
LEGACY_PRIMS="ensure-global-config.sh link-skills.sh sync-agents.sh ensure-docket-env.sh"
plant_legacy_shims(){ # <bin-dir> <sentinel-dir>
  # Bake the sentinel path into each shim at write time — the shim then depends on nothing but a
  # shell builtin at run time, since the guard runs install.sh on a stripped PATH where even
  # basename is absent (a runtime $(basename "$0") would fail and mask the very execution we test).
  for prim in $LEGACY_PRIMS; do
    cat > "$1/$prim" <<EOF
#!/bin/sh
: > "$2/$prim.ran"
EOF
    chmod +x "$1/$prim"
  done
}

# Delegate path: a compatible docket on PATH selects delegation; dry-run so nothing is exec'd —
# the bootstrapper's whole control flow (resolve, discover, parse, build) runs and must touch none.
legacy_bin="$(mk)"; legacy_sent="$(mk)"
ln -s "$(command -v dirname)" "$legacy_bin/dirname"
plant_legacy_shims "$legacy_bin" "$legacy_sent"
compat_stub "$legacy_bin"
out="$( cd / && PATH="$legacy_bin" DOCKET_BOOTSTRAP_DRY_RUN=1 /bin/sh "$REPO_ROOT/install.sh" )"; rc=$?
ran="$(ls -A "$legacy_sent" 2>/dev/null)"
assert "A3.b bootstrapper (delegate path) runs no legacy install primitive" \
  '[ -z "$ran" ] || { echo "  legacy primitive(s) executed: $ran" >&2; false; }'

# No-binary go-run path: docket absent, a stub go so state=2 is reached; dry-run again.
legacy_bin2="$(mk)"; legacy_sent2="$(mk)"
ln -s "$(command -v dirname)" "$legacy_bin2/dirname"
ln -s "$(command -v go)" "$legacy_bin2/go"
plant_legacy_shims "$legacy_bin2" "$legacy_sent2"
out="$( cd / && PATH="$legacy_bin2" DOCKET_BOOTSTRAP_DRY_RUN=1 /bin/sh "$REPO_ROOT/install.sh" )"; rc=$?
ran2="$(ls -A "$legacy_sent2" 2>/dev/null)"
assert "A3.b bootstrapper (go-run path) runs no legacy install primitive" \
  '[ -z "$ran2" ] || { echo "  legacy primitive(s) executed: $ran2" >&2; false; }'

if [ "$fail" -ne 0 ]; then exit 1; fi
