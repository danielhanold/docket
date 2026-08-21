#!/usr/bin/env bash
# scripts/release-smoke.sh — native per-tuple smoke driver for a release-candidate bundle
# (change 0317). Contract: scripts/release-smoke.md
#
# WHAT THIS PROVES, on the HOST tuple and against the PACKAGED BYTES — never a rebuild:
#   1. The host-tuple archive verifies against the bundle's checksums.txt and extracts to exactly
#      one regular member `docket`.
#   2. That extracted binary reports the exact expected version (version --json) and the native
#      supported tuple (diagnostic runtime --json).
#   3. The bundle's rendered POSIX downloader (internal/release/downloader/install.sh, stamped)
#      installs into an isolated HOME across all four harnesses, a read-only install check is
#      clean, and a same-version rerun is idempotent (record + dest hashes unchanged).
#   4. With --base-bundle: installing the base then upgrading to head replaces ONLY the owned
#      binary bytes and the record — a foreign harness asset planted before the upgrade is
#      byte-identical after.
#   5. The Task-9 rename interruption (the downloader's `mv -f "$stage" "$dest"` step, replaced by
#      `exit 97` in a doctored copy) leaves no binary at dest, and a rerun of the REAL downloader
#      then CONVERGES to a real installed binary. The doctoring is proven to change exactly one
#      line, so the case cannot pass vacuously (learning assert-detects-removal-not-replacement).
#
# This is EVIDENCE FOR THE HOST TUPLE ONLY. A host process cannot execute foreign-tuple binaries;
# the other three tuples are proven by the workflow's native smoke matrix and by the four-harness
# live acceptance — both external truth. See scripts/release-smoke.md.
#
# Bash is fine here: this runs on CI runners and dev machines, NOT inside the downloader's POSIX
# constraint. `set -uo pipefail`, no `producer | early-exiting-consumer` pipelines. Every block
# ends non-zero via die() with a NAMED diagnostic on the first failure; on success the ONLY line on
# stdout is `SMOKE PASS <os>/<arch> <version>`, which the workflow summary greps for verbatim.
set -uo pipefail

# --- Diagnostics: every failure names its block; nothing but the PASS line reaches stdout --------
die() { printf 'SMOKE FAIL: %s\n' "$1" >&2; exit 1; }
usage_die() { printf 'release-smoke.sh: %s\n' "$1" >&2
	printf '%s\n' \
"Usage: release-smoke.sh --bundle <dir> --version <v> [--base-bundle <dir> --base-version <v>]" >&2
	exit 2; }
step() { printf 'release-smoke: %s\n' "$1" >&2; }

# --- Flag parsing --------------------------------------------------------------------------------
BUNDLE=''; VERSION=''; BASE_BUNDLE=''; BASE_VERSION=''
while [ $# -gt 0 ]; do
	case $1 in
		--bundle)       [ $# -ge 2 ] || usage_die "--bundle requires a value";       BUNDLE=$2; shift 2 ;;
		--version)      [ $# -ge 2 ] || usage_die "--version requires a value";      VERSION=$2; shift 2 ;;
		--base-bundle)  [ $# -ge 2 ] || usage_die "--base-bundle requires a value";  BASE_BUNDLE=$2; shift 2 ;;
		--base-version) [ $# -ge 2 ] || usage_die "--base-version requires a value"; BASE_VERSION=$2; shift 2 ;;
		-h|--help)      usage_die "help" ;;
		*)              usage_die "unknown argument: $1" ;;
	esac
done
[ -n "$BUNDLE" ]  || usage_die "--bundle is required"
[ -n "$VERSION" ] || usage_die "--version is required"
[ -d "$BUNDLE" ]  || usage_die "--bundle is not a directory: $BUNDLE"
# --base-bundle and --base-version are all-or-nothing: the upgrade block needs a real prior binary.
if [ -n "$BASE_BUNDLE" ] || [ -n "$BASE_VERSION" ]; then
	[ -n "$BASE_BUNDLE" ]  || usage_die "--base-version was given without --base-bundle"
	[ -n "$BASE_VERSION" ] || usage_die "--base-bundle was given without --base-version"
	[ -d "$BASE_BUNDLE" ]  || usage_die "--base-bundle is not a directory: $BASE_BUNDLE"
fi

# --- Host-side SHA-256 — test scaffolding, not the downloader's seam. Same lowercase-hex digest --
# from either provider, so a recomputed expectation is provider-independent.
if command -v sha256sum >/dev/null 2>&1; then SHA_PROVIDER=sha256sum
elif command -v openssl >/dev/null 2>&1; then SHA_PROVIDER=openssl
else die "prereq: a SHA-256 tool is required but neither sha256sum nor openssl is on PATH"; fi
sha_file() {
	case $SHA_PROVIDER in
		sha256sum) sha256sum "$1" | { read -r _h _rest; printf '%s' "$_h"; } ;;
		openssl)   openssl dgst -sha256 -r "$1" | { read -r _h _rest; printf '%s' "$_h"; } ;;
	esac
}
command -v curl >/dev/null 2>&1 || die "prereq: curl is required but was not found on PATH"
command -v tar  >/dev/null 2>&1 || die "prereq: tar is required but was not found on PATH"

# --- Extract a flat JSON field. The result lines are compact single-line documents (one JSON
# object, no nesting around the fields we read), so a scalar field reads cleanly. -----------------
json_str() { # $1=field  $2=json line
	printf '%s' "$2" | sed -n 's/.*"'"$1"'":"\([^"]*\)".*/\1/p'
}
json_true() { # $1=field  $2=json line  -> 0 if "field":true present
	case $2 in *"\"$1\":true"*) return 0 ;; *) return 1 ;; esac
}

# --- Isolated roots the script owns, under a templated scratch dir (AGENTS.md Shell) --------------
ROOT=$(mktemp -d "${TMPDIR:-/tmp}/release-smoke.XXXXXX") || die "setup: cannot create scratch directory"
trap 'rm -rf "$ROOT"' EXIT

# A "session" is a self-contained HOME + XDG_* + bin root the downloader and the installed binary
# both run against, so nothing touches the invoking user's real dot-directories.
mk_session() { # $1 = name
	S_ROOT="$ROOT/session-$1"
	S_HOME="$S_ROOT/home"; S_STATE="$S_ROOT/state"; S_BIN="$S_ROOT/bin"
	S_DATA="$S_ROOT/data"; S_CONFIG="$S_ROOT/config"; S_TMP="$S_ROOT/tmp"
	mkdir -p "$S_HOME" "$S_STATE" "$S_BIN" "$S_DATA" "$S_CONFIG" "$S_TMP" \
		|| die "session: cannot create isolated roots for $1"
	S_DEST="$S_BIN/docket"
	S_RECORD="$S_STATE/docket/release-binary.record"
}

# Run the downloader (real or doctored) against the CURRENT session and a file:// base URL.
run_dl() { # $1=install.sh  $2=base_url  rest = downloader args
	local _script=$1 _base=$2; shift 2
	env HOME="$S_HOME" XDG_STATE_HOME="$S_STATE" XDG_BIN_HOME="$S_BIN" \
		XDG_DATA_HOME="$S_DATA" XDG_CONFIG_HOME="$S_CONFIG" TMPDIR="$S_TMP" \
		DOCKET_RELEASE_BASE_URL="$_base" \
		/bin/sh "$_script" "$@"
}

# Run the CURRENT session's installed binary with the session environment (so install check reads
# the state this session installed).
run_docket() {
	env HOME="$S_HOME" XDG_STATE_HOME="$S_STATE" XDG_BIN_HOME="$S_BIN" \
		XDG_DATA_HOME="$S_DATA" XDG_CONFIG_HOME="$S_CONFIG" TMPDIR="$S_TMP" \
		"$S_DEST" "$@"
}

# --- Block A: map the host tuple -----------------------------------------------------------------
step "block A: mapping the host tuple"
uname_s=$(uname -s) || die "tuple: cannot determine OS via uname -s"
case $uname_s in
	Darwin) HOST_OS=darwin ;;
	Linux)  HOST_OS=linux ;;
	*)      die "tuple: unsupported operating system: $uname_s (need Darwin or Linux)" ;;
esac
uname_m=$(uname -m) || die "tuple: cannot determine architecture via uname -m"
case $uname_m in
	x86_64|amd64)  HOST_ARCH=amd64 ;;
	arm64|aarch64) HOST_ARCH=arm64 ;;
	*)             die "tuple: unsupported architecture: $uname_m (need x86_64/amd64 or arm64/aarch64)" ;;
esac
ARCHIVE="docket_${VERSION}_${HOST_OS}_${HOST_ARCH}.tar.gz"

# Stage the host-tuple archive + checksums.txt into <releases>/<version>/, the layout the downloader
# fetches (base_url/<version>/…). Only the host archive is needed: the downloader fetches one tuple.
stage_release() { # $1=bundle dir  $2=version  $3=releases root
	local _b=$1 _v=$2 _rr=$3 _arc="docket_${2}_${HOST_OS}_${HOST_ARCH}.tar.gz"
	[ -f "$_b/$_arc" ]          || die "stage: bundle $_b is missing $_arc"
	[ -f "$_b/checksums.txt" ]  || die "stage: bundle $_b is missing checksums.txt"
	mkdir -p "$_rr/$_v"         || die "stage: cannot create release dir $_rr/$_v"
	cp "$_b/$_arc" "$_rr/$_v/"          || die "stage: cannot copy $_arc into the release dir"
	cp "$_b/checksums.txt" "$_rr/$_v/"  || die "stage: cannot copy checksums.txt into the release dir"
}

# --- Block B: verify the host archive against checksums.txt, then extract ------------------------
step "block B: verifying and extracting $ARCHIVE"
[ -f "$BUNDLE/$ARCHIVE" ]         || die "verify: bundle is missing the host archive $ARCHIVE"
[ -f "$BUNDLE/checksums.txt" ]    || die "verify: bundle is missing checksums.txt"
# Escape the dots so the fixed filename cannot over-match; require EXACTLY one syntactic line.
archive_re=$(printf '%s' "$ARCHIVE" | sed 's/\./\\./g')
manifest_count=$(grep -Ec "^[0-9a-f]{64}  ${archive_re}\$" "$BUNDLE/checksums.txt") || manifest_count=0
[ "$manifest_count" -eq 1 ] \
	|| die "verify: checksums.txt does not contain exactly one entry for $ARCHIVE (found $manifest_count)"
manifest_line=$(grep -E "^[0-9a-f]{64}  ${archive_re}\$" "$BUNDLE/checksums.txt") \
	|| die "verify: cannot read the checksum entry for $ARCHIVE"
expected_sha=${manifest_line%% *}
actual_sha=$(sha_file "$BUNDLE/$ARCHIVE")
[ -n "$actual_sha" ] && [ "$actual_sha" = "$expected_sha" ] \
	|| die "verify: checksum mismatch for $ARCHIVE (bundle bytes do not match checksums.txt)"
# The listing must be exactly the single member `docket` — nothing else may be extracted.
listing=$(tar -tzf "$BUNDLE/$ARCHIVE") || die "verify: cannot list the contents of $ARCHIVE"
[ "$listing" = "docket" ] \
	|| die "verify: $ARCHIVE members are not exactly {docket} (got: $(printf '%s' "$listing" | tr '\n' ' '))"
EXTRACT="$ROOT/extract"; mkdir -p "$EXTRACT" || die "verify: cannot create the extraction dir"
tar -xzf "$BUNDLE/$ARCHIVE" -C "$EXTRACT" || die "verify: extraction of $ARCHIVE failed"
BIN="$EXTRACT/docket"
{ [ -f "$BIN" ] && [ ! -L "$BIN" ]; } || die "verify: extracted docket is not a regular file"
chmod 755 "$BIN" || die "verify: cannot set mode 755 on the extracted binary"

# --- Block C: run the extracted binary directly — build identity + native tuple ------------------
step "block C: checking build identity and native tuple"
ver_json=$("$BIN" version --json) || die "identity: '$BIN version --json' exited non-zero"
got_version=$(json_str version "$ver_json")
[ "$got_version" = "$VERSION" ] \
	|| die "identity: version --json reported '$got_version', expected '$VERSION'"
rt_json=$("$BIN" diagnostic runtime --json) || die "identity: '$BIN diagnostic runtime --json' exited non-zero"
rt_os=$(json_str go_os "$rt_json")
rt_arch=$(json_str go_arch "$rt_json")
[ "$rt_os" = "$HOST_OS" ] \
	|| die "identity: diagnostic runtime go_os '$rt_os' does not match host OS '$HOST_OS'"
[ "$rt_arch" = "$HOST_ARCH" ] \
	|| die "identity: diagnostic runtime go_arch '$rt_arch' does not match host arch '$HOST_ARCH'"
json_true supported_target "$rt_json" \
	|| die "identity: diagnostic runtime does not report the host tuple as a supported_target"

# --- Block D: drive the bundle's downloader to install across all four harnesses ------------------
step "block D: installing via the bundle downloader (claude, codex, cursor, opencode)"
HEAD_REL="$ROOT/releases-head"
stage_release "$BUNDLE" "$VERSION" "$HEAD_REL"
mk_session main
if ! run_dl "$BUNDLE/install.sh" "file://$HEAD_REL" --version "$VERSION" \
		--harness claude --harness codex --harness cursor --harness opencode >"$ROOT/install.out" 2>&1; then
	die "install: the bundle downloader exited non-zero: $(tail -n 3 "$ROOT/install.out" | tr '\n' ' ')"
fi
[ -f "$S_DEST" ]   || die "install: no binary at dest after a successful install"
[ -f "$S_RECORD" ] || die "install: no ownership record after a successful install"
grep -qxF "version=$VERSION" "$S_RECORD" \
	|| die "install: ownership record does not name the installed version: $(tr '\n' '|' < "$S_RECORD")"

# --- Block E: read-only install check must be clean ----------------------------------------------
step "block E: install check --json"
check_json=$(run_docket install check --json); check_rc=$?
[ "$check_rc" -eq 0 ] || die "check: install check --json exited $check_rc (expected 0/clean)"
check_result=$(json_str result "$check_json")
case $check_result in
	applied|no-op) ;;
	*) die "check: install check result '$check_result' is not clean (expected applied or no-op)" ;;
esac

# --- Block F: same-version rerun is idempotent ---------------------------------------------------
step "block F: same-version rerun idempotence"
dest_before=$(sha_file "$S_DEST")
record_before=$(sha_file "$S_RECORD")
if ! run_dl "$BUNDLE/install.sh" "file://$HEAD_REL" --version "$VERSION" \
		--harness claude --harness codex --harness cursor --harness opencode >"$ROOT/rerun.out" 2>&1; then
	die "idempotent: same-version rerun exited non-zero: $(tail -n 3 "$ROOT/rerun.out" | tr '\n' ' ')"
fi
[ "$(sha_file "$S_DEST")" = "$dest_before" ] \
	|| die "idempotent: dest binary hash changed on a same-version rerun"
[ "$(sha_file "$S_RECORD")" = "$record_before" ] \
	|| die "idempotent: ownership record hash changed on a same-version rerun"

# --- Block G: upgrade base -> head replaces only the owned bytes (only with --base-bundle) --------
if [ -n "$BASE_BUNDLE" ]; then
	step "block G: upgrade $BASE_VERSION -> $VERSION preserves foreign harness bytes"
	# One releases root serving both versions, so a single base URL drives base install and upgrade.
	UP_REL="$ROOT/releases-upgrade"
	stage_release "$BASE_BUNDLE" "$BASE_VERSION" "$UP_REL"
	stage_release "$BUNDLE" "$VERSION" "$UP_REL"
	mk_session upgrade
	# Install the base candidate.
	if ! run_dl "$BASE_BUNDLE/install.sh" "file://$UP_REL" --version "$BASE_VERSION" \
			--harness claude --harness codex --harness cursor --harness opencode >"$ROOT/base.out" 2>&1; then
		die "upgrade: base install of $BASE_VERSION exited non-zero: $(tail -n 3 "$ROOT/base.out" | tr '\n' ' ')"
	fi
	base_dest_sha=$(sha_file "$S_DEST")
	base_ver=$(json_str version "$(run_docket version --json)")
	[ "$base_ver" = "$BASE_VERSION" ] \
		|| die "upgrade: base install reports version '$base_ver', expected '$BASE_VERSION'"
	# Plant a FOREIGN sentinel under a real harness dir (.claude exists after the base install).
	sentinel="$S_HOME/.claude/docket-smoke-sentinel.bin"
	printf 'foreign-harness-asset for release-smoke pid %s\n' "$$" > "$sentinel" \
		|| die "upgrade: cannot plant the foreign harness sentinel"
	sentinel_before=$(sha_file "$sentinel")
	# Upgrade to head (owned-record branch: dest is base, record names dest with base's hash).
	if ! run_dl "$BUNDLE/install.sh" "file://$UP_REL" --version "$VERSION" \
			--harness claude --harness codex --harness cursor --harness opencode >"$ROOT/upgrade.out" 2>&1; then
		die "upgrade: upgrade to $VERSION exited non-zero: $(tail -n 3 "$ROOT/upgrade.out" | tr '\n' ' ')"
	fi
	head_ver=$(json_str version "$(run_docket version --json)")
	[ "$head_ver" = "$VERSION" ] \
		|| die "upgrade: after upgrade the installed binary reports '$head_ver', expected '$VERSION'"
	[ "$(sha_file "$S_DEST")" != "$base_dest_sha" ] \
		|| die "upgrade: dest bytes unchanged across the upgrade — base and head bundles built an identical binary (vacuous upgrade)"
	grep -qxF "version=$VERSION" "$S_RECORD" \
		|| die "upgrade: ownership record was not updated to the head version: $(tr '\n' '|' < "$S_RECORD")"
	[ "$(sha_file "$sentinel")" = "$sentinel_before" ] \
		|| die "upgrade: the foreign harness sentinel changed across the upgrade (the downloader replaced bytes it does not own)"
else
	step "block G: skipped (no --base-bundle given)"
fi

# --- Block H: rename-interruption convergence on the real tuple ----------------------------------
# Doctored-copy technique inlined from Task 9: replace the downloader's single rename line with
# `exit 97`. The run installs assets and stages, then stops BEFORE the binary is moved into place;
# dest is never created. A rerun of the REAL downloader then converges to a real installed binary.
step "block H: rename-interruption convergence"
mk_session converge
DOCTORED="$ROOT/install.doctored.sh"
awk -v needle='mv -f "$stage" "$dest"' '
	index($0, needle) { print "exit 97"; n++; next }
	{ print }
	END { exit (n == 1 ? 0 : 1) }
' "$BUNDLE/install.sh" > "$DOCTORED" \
	|| die "converge: doctoring did not replace exactly one rename line in the downloader"
# Prove the mutation landed and is a one-line replacement (one `<` and one `>` marker): a silent
# no-op would make convergence pass vacuously (learning assert-detects-removal-not-replacement).
doctor_markers=$(diff "$BUNDLE/install.sh" "$DOCTORED" | grep -Ec '^[<>]') || true
[ "$doctor_markers" = 2 ] \
	|| die "converge: doctored copy differs from the source by $doctor_markers marker lines (want 2) — the injection is vacuous"
chmod 755 "$DOCTORED" 2>/dev/null || true
# Doctored run: fresh install, so the ownership decision allows it; it exits 97 at the rename.
run_dl "$DOCTORED" "file://$HEAD_REL" --version "$VERSION" --harness claude >"$ROOT/converge1.out" 2>&1
converge_rc=$?
[ "$converge_rc" -eq 97 ] \
	|| die "converge: the doctored run exited $converge_rc, expected 97 (the injected interruption was not reached)"
[ ! -e "$S_DEST" ] \
	|| die "converge: the interrupted run left a binary at dest (the rename should not have run)"
if ls "$S_BIN"/.docket-stage.* >/dev/null 2>&1; then
	die "converge: the interrupted run left a staging file behind"
fi
# Rerun the REAL downloader — it must converge to a real installed binary.
if ! run_dl "$BUNDLE/install.sh" "file://$HEAD_REL" --version "$VERSION" --harness claude >"$ROOT/converge2.out" 2>&1; then
	die "converge: rerun of the real downloader failed to converge: $(tail -n 3 "$ROOT/converge2.out" | tr '\n' ' ')"
fi
[ -f "$S_DEST" ] || die "converge: no binary at dest after the converging rerun"
converge_ver=$(json_str version "$(run_docket version --json)")
[ "$converge_ver" = "$VERSION" ] \
	|| die "converge: the converged binary reports '$converge_ver', expected '$VERSION'"
grep -qxF "version=$VERSION" "$S_RECORD" \
	|| die "converge: ownership record was not published after convergence: $(tr '\n' '|' < "$S_RECORD")"

# --- All blocks green ----------------------------------------------------------------------------
printf 'SMOKE PASS %s/%s %s\n' "$HOST_OS" "$HOST_ARCH" "$VERSION"
