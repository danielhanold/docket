#!/usr/bin/env bash
# Configured-Bash routing witnesses for the public facade and Docket-owned helper launchers.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
FACADE="$REPO/scripts/docket.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

REAL_BASH="$(command -v bash)"
for candidate in "$REAL_BASH" /opt/homebrew/bin/bash /usr/local/bin/bash; do
  [ -x "$candidate" ] || continue
  candidate_major="$(LC_ALL=C "$candidate" --version 2>/dev/null | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')"
  if [ -n "$candidate_major" ] && [ "$candidate_major" -ge 4 ]; then REAL_BASH="$candidate"; break; fi
done
CONFIGURED_LOG="$tmp/configured.log"
PATH_LOG="$tmp/path.log"
LEGACY_LOG="$tmp/legacy.log"
export REAL_BASH CONFIGURED_LOG PATH_LOG LEGACY_LOG

mkdir -p "$tmp/configured" "$tmp/legacy" "$tmp/path-bin" "$tmp/stub-scripts" "$tmp/runners"
cat > "$tmp/configured/bash" <<'SH'
#!/bin/sh
printf 'configured:' >> "$CONFIGURED_LOG"
printf ' <%s>' "$@" >> "$CONFIGURED_LOG"
printf '\n' >> "$CONFIGURED_LOG"
exec "$REAL_BASH" "$@"
SH
cat > "$tmp/path-bin/bash" <<'SH'
#!/bin/sh
printf 'path:' >> "$PATH_LOG"
printf ' <%s>' "$@" >> "$PATH_LOG"
printf '\n' >> "$PATH_LOG"
exec "$REAL_BASH" "$@"
SH
cat > "$tmp/legacy/bash" <<'SH'
#!/bin/sh
if [ "${1:-}" = --version ]; then
  printf '%s\n' 'GNU bash, version 3.2.57(1)-release (fake-legacy)'
  exit 0
fi
printf 'unexpected delegation:' >> "$LEGACY_LOG"
printf ' <%s>' "$@" >> "$LEGACY_LOG"
printf '\n' >> "$LEGACY_LOG"
exec "$REAL_BASH" "$@"
SH
chmod +x "$tmp/configured/bash" "$tmp/legacy/bash" "$tmp/path-bin/bash"
RUNTIME="$tmp/configured/bash"
LEGACY_RUNTIME="$tmp/legacy/bash"

cat > "$tmp/stub-scripts/board-refresh.sh" <<'SH'
#!/usr/bin/env bash
printf 'BOARD'
printf ' <%s>' "$@"
printf '\n'
SH
cat > "$tmp/stub-scripts/docket-config.sh" <<SH
#!/usr/bin/env bash
printf '%s\n' \
  'DOCKET_MODE=docket' \
  'METADATA_BRANCH=docket' \
  'METADATA_WORKTREE=$tmp/metadata' \
  'BOOTSTRAP=PROCEED' \
  'DOCKET_BASH_PATH=$RUNTIME'
SH
cat > "$tmp/stub-scripts/disable-worktree-hooks.sh" <<'SH'
#!/usr/bin/env bash
printf 'HOOKS <%s>\n' "$*" >&2
SH
cat > "$tmp/git" <<'SH'
#!/bin/sh
exit 0
SH
cat > "$tmp/runners/probe.sh" <<'SH'
#!/usr/bin/env bash
printf 'ADAPTER'
printf ' <%s>' "$@"
printf '\n'
SH
chmod +x "$tmp/stub-scripts/"*.sh "$tmp/git" "$tmp/runners/probe.sh"
mkdir -p "$tmp/metadata"

: > "$CONFIGURED_LOG"; : > "$PATH_LOG"
out="$(PATH="$tmp/path-bin:/usr/bin:/bin" DOCKET_BASH_PATH="$RUNTIME" \
  SCRIPTS_DIR="$tmp/stub-scripts" "$FACADE" board-refresh 'two words' tail 2>/dev/null)"; rc=$?
assert "facade preserves helper argv through the configured runtime" \
  '[ "$rc" -eq 0 ] && [ "$out" = "BOARD <two words> <tail>" ]'
assert "facade and wrapped helper launch through configured Bash" \
  'grep -qF "<$tmp/stub-scripts/board-refresh.sh> <two words> <tail>" "$CONFIGURED_LOG"'
assert "facade routing never selects bash from PATH" '[ ! -s "$PATH_LOG" ]'

: > "$CONFIGURED_LOG"; : > "$PATH_LOG"
pf="$(PATH="$tmp/path-bin:/usr/bin:/bin" DOCKET_BASH_PATH="$RUNTIME" \
  SCRIPTS_DIR="$tmp/stub-scripts" GIT="$tmp/git" "$FACADE" preflight 2>/dev/null)"; rc=$?
assert "preflight preserves raw config output" \
  '[ "$rc" -eq 0 ] && grep -qxF "BOOTSTRAP=PROCEED" <<<"$pf"'
assert "preflight config resolver launches through configured Bash" \
  'grep -qF "<$tmp/stub-scripts/docket-config.sh> <--export>" "$CONFIGURED_LOG"'
assert "preflight internal helper launches through configured Bash" \
  'grep -qF "<$tmp/stub-scripts/disable-worktree-hooks.sh> <--worktree> <$tmp/metadata>" "$CONFIGURED_LOG"'
assert "preflight routing never selects bash from PATH" '[ ! -s "$PATH_LOG" ]'

: > "$CONFIGURED_LOG"; : > "$PATH_LOG"
adapter_out="$(cd "$REPO" && PATH="$tmp/path-bin:/usr/bin:/bin" DOCKET_BASH_PATH="$RUNTIME" \
  RUNNERS_DIR="$tmp/runners" "$RUNTIME" "$REPO/scripts/runner-dispatch.sh" \
  --runner probe --agent alpha --model beta -- payload 2>/dev/null)"; rc=$?
assert "runner launcher preserves adapter argv and exit status" \
  '[ "$rc" -eq 0 ] && [ "$adapter_out" = "ADAPTER <--agent> <alpha> <--model> <beta> <--> <payload>" ]'
assert "runner adapter launches through configured Bash" \
  'grep -qF "<$tmp/runners/probe.sh> <--agent> <alpha> <--model> <beta> <--> <payload>" "$CONFIGURED_LOG"'
assert "runner launcher never selects bash from PATH" '[ ! -s "$PATH_LOG" ]'

unset_out="$(env -u DOCKET_BASH_PATH "$FACADE" env 2>&1)"; unset_rc=$?
assert "missing configured Bash fails before facade work" '[ "$unset_rc" -ne 0 ]'
assert "missing configured Bash gives install remediation" \
  'grep -qF "run docket/install.sh after installing Bash 4+ (on macOS: brew install bash)" <<<"$unset_out"'

missing="$tmp/does-not-exist"
invalid_out="$(DOCKET_BASH_PATH="$missing" "$FACADE" env 2>&1)"; invalid_rc=$?
assert "unsupported configured Bash path fails before facade work" '[ "$invalid_rc" -ne 0 ]'
assert "unsupported path gives the same install remediation" \
  'grep -qF "run docket/install.sh after installing Bash 4+ (on macOS: brew install bash)" <<<"$invalid_out"'

: > "$LEGACY_LOG"
legacy_out="$(DOCKET_BASH_PATH="$LEGACY_RUNTIME" SCRIPTS_DIR="$tmp/stub-scripts" "$FACADE" env 2>&1)"; legacy_rc=$?
assert "executable GNU Bash 3 runtime is rejected nonzero" '[ "$legacy_rc" -ne 0 ]'
assert "legacy runtime gets the version-specific diagnosis" \
  'grep -qF "runtime.bash must be Bash 4 or newer, got '\''GNU bash, version 3.2.57(1)-release (fake-legacy)'\'' from $LEGACY_RUNTIME" <<<"$legacy_out"'
assert "legacy runtime gets the actionable install remediation" \
  'grep -qF "run docket/install.sh after installing Bash 4+ (on macOS: brew install bash)" <<<"$legacy_out"'
assert "legacy runtime is rejected before facade implementation work" '[ ! -s "$LEGACY_LOG" ]'

exit "$fail"
