#!/usr/bin/env bash
# tests/test_release_downloader_refusals.sh — refusal and ownership guards over the POSIX release
# downloader internal/release/downloader/install.sh (change 0317, Task 8).
#
# WHAT THIS PROVES. The downloader must NEVER move unverified bytes into the bin dir or run them,
# and must REFUSE to replace a binary it does not own — with no --force path. Every refusal case
# below asserts the same three invariants, plus a pinned diagnostic keyword:
#   (1) non-zero exit;
#   (2) NO byte changed at the destination or the ownership record (a before/after state signature
#       over both — an absent file stays absent, a present file keeps its exact bytes);
#   (3) NO `install` line reached the fake docket (unverified bytes are never executed).
# The one deliberate exception is the interrupted-completion case (dest already equals the freshly
# verified requested binary, record absent): it SUCCEEDS and publishes the record. The asset-install
# failure case is also special — there the bytes ARE verified and `docket install` DID run and fail,
# so its invariant is instead that the prior binary and record survive byte-for-byte and the stage
# is cleaned up.
#
# TWIN NOTE: the helper block below (the globals, the fake docket + tripwire fixtures,
# dl_build_sandbox / dl_mk_release / dl_case / dl_run, and dl_sha / pmode / have) is COPIED verbatim
# from tests/test_release_downloader.sh (Task 7) and is also carried by
# tests/test_release_downloader_converge.sh (Task 9). Each of those files is hermetic per
# tests/README.md — it sources nothing — so the block lives in each. Keep the copies in sync when the
# downloader contract moves.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SCRIPT="$REPO/internal/release/downloader/install.sh"
fail=0
ok(){ printf 'ok - %s\n' "$1"; }
nok(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

if [ ! -f "$SCRIPT" ]; then
  nok "downloader script missing at internal/release/downloader/install.sh"
  exit "$fail"
fi

# ================================================================================================
# TWIN NOTE helper block — verbatim copy of tests/test_release_downloader.sh Section B+ (Task 7).
# ================================================================================================

# Some hosts ship only one hashing provider; a case whose provider is absent is skipped, not failed.
have(){ command -v "$1" >/dev/null 2>&1; }

# Portable file-mode read: GNU stat first, then BSD stat. GNU-form MUST lead: on Linux `stat -f`
# means --file-system (not "format"), so a BSD-first `stat -f '%Lp'` prints a filesystem block to
# stdout AND exits non-zero, poisoning the captured mode so it never equals 755. BSD `stat -c` fails
# clean (rc!=0, no stdout), so the `|| stat -f '%Lp'` fallback is safe on macOS.
pmode(){ stat -c '%a' "$1" 2>/dev/null || stat -f '%Lp' "$1"; }

# Host-side SHA-256 — TEST SCAFFOLDING ONLY, not the script's seam. Prefer sha256sum, else openssl;
# both yield the same lowercase-hex digest, so the recomputed expectation is provider-independent.
dl_sha(){
  if have sha256sum; then _o=$(sha256sum "$1"); else _o=$(openssl dgst -sha256 -r "$1"); fi
  printf '%s' "${_o%% *}"
}

WORK=$(mktemp -d "${TMPDIR:-/tmp}/dl-downloader.XXXXXX") || { nok "cannot create scratch root"; exit "$fail"; }
trap 'rm -rf "$WORK"' EXIT
SANDBOX="$WORK/sandbox"
FAKE_DOCKET_SRC="$WORK/fake-docket"
TRIPWIRE_SRC="$WORK/tripwire"
FAKE_DOCKET_LOG="$WORK/fake-docket.log"
TRIP_LOG="$WORK/trip.log"
: > "$FAKE_DOCKET_LOG"; : > "$TRIP_LOG"

# The archive member: a fake `docket` that logs its argv to $FAKE_DOCKET_LOG, succeeds on install
# and install check, answers `version --json` with a canned line, and honors FAIL_ON (install|check)
# — the FAIL_ON hook is unused here but keeps the copied block ready for Tasks 8–9.
cat > "$FAKE_DOCKET_SRC" <<'FAKE'
#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_DOCKET_LOG"
case "$1 ${2:-}" in
  "version --json")
    printf '%s\n' '{"version":"v0.0.0-fake","commit":"0000000000000000000000000000000000000000","build_date":"1970-01-01T00:00:00Z"}'
    exit 0 ;;
esac
if [ "${FAIL_ON:-}" = install ] && [ "$1" = install ] && [ "${2:-}" != check ]; then exit 3; fi
if [ "${FAIL_ON:-}" = check ] && [ "$1" = install ] && [ "${2:-}" = check ]; then exit 4; fi
exit 0
FAKE
chmod 755 "$FAKE_DOCKET_SRC"

# A tripwire fake for each banned tool: it records the call and exits 9. An empty $TRIP_LOG after a
# run is therefore proof the downloader reached no banned interpreter/helper — the SEMANTIC ban.
cat > "$TRIPWIRE_SRC" <<'TRIP'
#!/bin/sh
printf '%s %s\n' "$0" "$*" >> "${TRIP_LOG:-/dev/stderr}"
exit 9
TRIP
chmod 755 "$TRIPWIRE_SRC"

# The exact real tools a fresh /bin/sh run of the downloader needs (printf is a /bin/sh builtin, so
# it is not symlinked; dirname IS used by the record path and is required). gzip is required for
# tar's `-z`: GNU tar (Linux) execs the external gzip binary, so its absence from the sandbox reddens
# `tar -tzf`/`tar -xzf`; BSD tar (macOS) links libz internally and needs none, which is why omitting
# it only fails on Linux. The banned set gets a tripwire each.
DL_REAL_TOOLS='curl tar gzip uname mktemp mkdir mv cp chmod rm grep sed cat dirname'
DL_BANNED='bash python python3 perl shasum jq'

# Build $SANDBOX/bin: real symlinks for the needed tools + a tripwire for each banned tool. $1
# selects the hash provider exposed (sha256sum | openssl); the other is deliberately absent so the
# script's provider probe actually exercises the selected branch.
dl_build_sandbox(){
  _prov="$1"
  rm -rf "$SANDBOX/bin"; mkdir -p "$SANDBOX/bin"
  for _t in $DL_REAL_TOOLS "$_prov"; do
    _r=$(command -v "$_t" 2>/dev/null) || _r=''
    case "$_r" in
      /*) ln -s "$_r" "$SANDBOX/bin/$_t" ;;
      *)  nok "sandbox: host is missing required tool $_t"; return 1 ;;
    esac
  done
  for _b in $DL_BANNED; do cp "$TRIPWIRE_SRC" "$SANDBOX/bin/$_b"; chmod 755 "$SANDBOX/bin/$_b"; done
  return 0
}

# Build a file:// release at <parent>/<version>/ holding the host-tuple archive (single member
# `docket`) and a two-space sha256sum-format checksums.txt. Real tar + real hashing; zero network.
dl_mk_release(){
  _parent="$1"; _ver="$2"
  _os=$(uname -s); case "$_os" in Darwin) _os=darwin ;; Linux) _os=linux ;; esac
  _arch=$(uname -m); case "$_arch" in x86_64|amd64) _arch=amd64 ;; arm64|aarch64) _arch=arm64 ;; esac
  _rel="$_parent/$_ver"; mkdir -p "$_rel"
  _arc="docket_${_ver}_${_os}_${_arch}.tar.gz"
  _mem=$(mktemp -d "${TMPDIR:-/tmp}/dl-member.XXXXXX")
  cp "$FAKE_DOCKET_SRC" "$_mem/docket"; chmod 755 "$_mem/docket"
  tar -czf "$_rel/$_arc" -C "$_mem" docket
  rm -rf "$_mem"
  _h=$(dl_sha "$_rel/$_arc")
  printf '%s  %s\n' "$_h" "$_arc" > "$_rel/checksums.txt"
}

# Fresh per-case roots: isolated HOME / XDG_STATE_HOME / XDG_BIN_HOME / TMPDIR, an empty releases
# dir, and reset logs. DL_SCRIPT defaults to the real script; a case may point it at a rendered copy.
dl_case(){
  _d=$(mktemp -d "${TMPDIR:-/tmp}/dl-case.XXXXXX")
  RUN_HOME="$_d/home"; RUN_STATE="$_d/state"; RUN_BIN="$_d/bin"; RUN_TMP="$_d/tmp"
  RELEASES="$_d/releases"
  mkdir -p "$RUN_HOME" "$RUN_STATE" "$RUN_BIN" "$RUN_TMP" "$RELEASES"
  BASE_URL="file://$RELEASES"
  RECORD="$RUN_STATE/docket/release-binary.record"
  DEST="$RUN_BIN/docket"
  : > "$FAKE_DOCKET_LOG"; : > "$TRIP_LOG"
  FAIL_ON=''
  DL_SCRIPT="$SCRIPT"
}

# Run the downloader in a genuinely fresh environment: env -i wipes everything, PATH is the sandbox,
# and only the documented seams are handed in. This is the semantic ban's enforcement point.
dl_run(){
  env -i \
    PATH="$SANDBOX/bin" \
    HOME="$RUN_HOME" \
    XDG_STATE_HOME="$RUN_STATE" \
    XDG_BIN_HOME="$RUN_BIN" \
    TMPDIR="$RUN_TMP" \
    DOCKET_RELEASE_BASE_URL="$BASE_URL" \
    FAKE_DOCKET_LOG="$FAKE_DOCKET_LOG" \
    TRIP_LOG="$TRIP_LOG" \
    FAIL_ON="$FAIL_ON" \
    /bin/sh "$DL_SCRIPT" "$@"
}

# Every install must land mode 755 even when the ambient umask would forbid it — so run the whole
# behavior suite under a restrictive umask and assert the explicit chmod 755 wins.
umask 077

# ================================================================================================
# Refusal-specific helpers (NOT part of the twin block).
# ================================================================================================

ERR="$WORK/stderr.log"

# The host-tuple archive name for a given version — the exact file the downloader will request.
dl_arc_name(){
  _o=$(uname -s); case "$_o" in Darwin) _o=darwin ;; Linux) _o=linux ;; esac
  _a=$(uname -m); case "$_a" in x86_64|amd64) _a=amd64 ;; arm64|aarch64) _a=arm64 ;; esac
  printf '%s' "docket_${1}_${_o}_${_a}.tar.gz"
}

# Write a valid two-space sha256sum-format checksums.txt for archive <2> in release dir <1>, so a
# hostile or tampered archive still PASSES the checksum gate and the refusal is proven to come from a
# LATER guard (listing / regular-file / ownership), not from a checksum mismatch.
dl_checksums_for(){
  _h=$(dl_sha "$1/$2")
  printf '%s  %s\n' "$_h" "$2" > "$1/checksums.txt"
}

# A before/after signature over the two bytes-that-must-not-move: the installed binary and the
# ownership record. An absent file reads as ABSENT so "stayed absent" is distinguishable from "same
# bytes". dl_sha uses a host tool (outer shell), independent of the sandboxed run.
dl_state(){
  if [ -e "$DEST" ]; then printf 'dest=%s\n' "$(dl_sha "$DEST")"; else printf 'dest=ABSENT\n'; fi
  if [ -e "$RECORD" ]; then printf 'record=%s\n' "$(dl_sha "$RECORD")"; else printf 'record=ABSENT\n'; fi
}

# The three shared refusal invariants: non-zero exit, no byte moved at dest/record, no install line.
# $1 = case label, $2 = the state signature captured BEFORE the run (in "$before").
assert_refused(){
  _after=$(dl_state)
  if [ "$rc" != 0 ]; then ok "$1: non-zero exit ($rc)"; else nok "$1: expected non-zero exit, got 0"; fi
  if [ "$_after" = "$2" ]; then ok "$1: destination and record unchanged byte-for-byte"; else nok "$1: state moved: before=[$(printf '%s' "$2" | tr '\n' ' ')] after=[$(printf '%s' "$_after" | tr '\n' ' ')]"; fi
  if grep -qF install "$FAKE_DOCKET_LOG"; then nok "$1: an install line reached the fake docket (unverified bytes executed)"; else ok "$1: no install line reached the fake docket"; fi
}

# Pin the actionable diagnostic keyword the failure must carry on stderr.
assert_diag(){
  if grep -qF -- "$2" "$ERR"; then ok "$1: diagnostic mentions '$2'"; else nok "$1: diagnostic missing '$2' (got: $(tr '\n' ' ' < "$ERR"))"; fi
}

# Only the sha256sum/openssl split matters for the ownership branches; pick whichever the host has
# so the whole file runs. Provider-specific coverage is Task 7's concern.
if have sha256sum; then PROV=sha256sum
elif have openssl; then PROV=openssl
else nok "host has neither sha256sum nor openssl — cannot run the refusal suite"; exit "$fail"
fi

FAKE_SHA=$(dl_sha "$FAKE_DOCKET_SRC")   # the hash the script computes for the verified member

# ================================================================================================
# GROUP 1 — checksum and manifest refusals (verification precedes extraction and install)
# ================================================================================================

# --- (1a) corrupted archive: a flipped byte moves the archive hash; refuse with a "checksum" word.
dl_build_sandbox "$PROV" || nok "1a: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
ARC=$(dl_arc_name "$VER"); AP="$RELEASES/$VER/$ARC"
orig=$(dd if="$AP" bs=1 skip=40 count=1 2>/dev/null | od -An -tu1 | tr -d ' ')
newb=$(( (orig + 1) % 256 ))
printf "$(printf '\\%03o' "$newb")" | dd of="$AP" bs=1 seek=40 count=1 conv=notrunc 2>/dev/null
before=$(dl_state)
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
assert_refused "corrupt-archive" "$before"
assert_diag "corrupt-archive" "checksum"

# --- (1b) missing manifest entry: checksums.txt names a DIFFERENT file, so the count is 0.
dl_build_sandbox "$PROV" || nok "1b: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
printf '%s  %s\n' "0000000000000000000000000000000000000000000000000000000000000000" "docket_other.tar.gz" > "$RELEASES/$VER/checksums.txt"
before=$(dl_state)
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
assert_refused "missing-manifest" "$before"
assert_diag "missing-manifest" "exactly one entry"

# --- (1c) duplicate manifest entry: two valid lines for the archive, so the count is 2, not 1.
dl_build_sandbox "$PROV" || nok "1c: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
ARC=$(dl_arc_name "$VER"); DUP=$(dl_sha "$RELEASES/$VER/$ARC")
{ printf '%s  %s\n' "$DUP" "$ARC"; printf '%s  %s\n' "$DUP" "$ARC"; } > "$RELEASES/$VER/checksums.txt"
before=$(dl_state)
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
assert_refused "duplicate-manifest" "$before"
assert_diag "duplicate-manifest" "exactly one entry"

# --- (1d) malformed manifest line: 63 hex digits and a single space — neither the width nor the
# two-space separator the checker requires, so the strict regex matches zero lines.
dl_build_sandbox "$PROV" || nok "1d: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
ARC=$(dl_arc_name "$VER")
printf '%s %s\n' "000000000000000000000000000000000000000000000000000000000000000" "$ARC" > "$RELEASES/$VER/checksums.txt"
before=$(dl_state)
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
assert_refused "malformed-manifest" "$before"
assert_diag "malformed-manifest" "exactly one entry"

# ================================================================================================
# GROUP 2 — hostile archives: the tar listing / regular-file guard refuses before the bin dir is
# ever touched. Each archive carries a VALID checksum, so the refusal is proven to be the listing
# guard, not the checksum gate.
# ================================================================================================

# --- (2a) extra member: `docket` plus a second member. Listing is two lines, not the single line.
dl_build_sandbox "$PROV" || nok "2a: sandbox build failed"
dl_case
VER=v0.1.0
mkdir -p "$RELEASES/$VER"
ARC=$(dl_arc_name "$VER")
mem=$(mktemp -d "$WORK/mem.XXXXXX")
cp "$FAKE_DOCKET_SRC" "$mem/docket"; chmod 755 "$mem/docket"; printf 'x\n' > "$mem/extra"
tar -czf "$RELEASES/$VER/$ARC" -C "$mem" docket extra
dl_checksums_for "$RELEASES/$VER" "$ARC"
list=$(tar -tzf "$RELEASES/$VER/$ARC")
if [ "$list" != "docket" ]; then ok "extra-member: fixture archive lists more than docket (non-vacuous)"; else nok "extra-member: fixture archive lists only docket — hostile member was lost"; fi
before=$(dl_state)
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
assert_refused "extra-member" "$before"
assert_diag "extra-member" "exactly one member named docket"

# --- (2b) symlink member: `docket` is a symlink, not a regular file. Listing is the single line
# `docket`, so it passes the listing gate and is refused at the regular-file check after extraction
# (still before the bin dir is touched).
dl_build_sandbox "$PROV" || nok "2b: sandbox build failed"
dl_case
VER=v0.1.0
mkdir -p "$RELEASES/$VER"
ARC=$(dl_arc_name "$VER")
mem=$(mktemp -d "$WORK/mem.XXXXXX")
ln -s /etc/hosts "$mem/docket"
tar -czf "$RELEASES/$VER/$ARC" -C "$mem" docket
dl_checksums_for "$RELEASES/$VER" "$ARC"
before=$(dl_state)
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
assert_refused "symlink-member" "$before"
assert_diag "symlink-member" "regular file"

# --- (2c) traversal member `../docket`: built with -P so the leading ../ survives on both bsdtar
# and GNU tar. Listing is `../docket`, not `docket`, so the listing guard refuses it.
dl_build_sandbox "$PROV" || nok "2c: sandbox build failed"
dl_case
VER=v0.1.0
mkdir -p "$RELEASES/$VER"
ARC=$(dl_arc_name "$VER")
mem=$(mktemp -d "$WORK/mem.XXXXXX"); mkdir -p "$mem/deeper"
cp "$FAKE_DOCKET_SRC" "$mem/docket"; chmod 755 "$mem/docket"
tar -czf "$RELEASES/$VER/$ARC" -P -C "$mem/deeper" ../docket 2>/dev/null
dl_checksums_for "$RELEASES/$VER" "$ARC"
list=$(tar -tzf "$RELEASES/$VER/$ARC")
if [ "$list" != "docket" ]; then ok "traversal-member: fixture archive lists a non-docket name (non-vacuous)"; else nok "traversal-member: leading ../ was stripped — fixture is not hostile"; fi
before=$(dl_state)
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
assert_refused "traversal-member" "$before"
assert_diag "traversal-member" "exactly one member named docket"

# --- (2d) member named `evil`: the sole member is not `docket`, so the listing guard refuses it.
dl_build_sandbox "$PROV" || nok "2d: sandbox build failed"
dl_case
VER=v0.1.0
mkdir -p "$RELEASES/$VER"
ARC=$(dl_arc_name "$VER")
mem=$(mktemp -d "$WORK/mem.XXXXXX")
cp "$FAKE_DOCKET_SRC" "$mem/evil"; chmod 755 "$mem/evil"
tar -czf "$RELEASES/$VER/$ARC" -C "$mem" evil
dl_checksums_for "$RELEASES/$VER" "$ARC"
before=$(dl_state)
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
assert_refused "evil-member" "$before"
assert_diag "evil-member" "exactly one member named docket"

# ================================================================================================
# GROUP 3 — unsupported tuple: fail BEFORE any network request. A tripwire curl proves curl is never
# invoked (TRIP_LOG stays empty), and a fake uname supplies the unsupported values.
# ================================================================================================

# --- (3a) unsupported OS: uname -s reports SunOS. The OS map fails before the prereq probe or curl.
dl_build_sandbox "$PROV" || nok "3a: sandbox build failed"
rm -f "$SANDBOX/bin/uname" "$SANDBOX/bin/curl"
cat > "$SANDBOX/bin/uname" <<'UNAME'
#!/bin/sh
case "$1" in -s) echo SunOS ;; -m) echo mips ;; *) echo unknown ;; esac
UNAME
chmod 755 "$SANDBOX/bin/uname"
cp "$TRIPWIRE_SRC" "$SANDBOX/bin/curl"; chmod 755 "$SANDBOX/bin/curl"
dl_case
before=$(dl_state)
dl_run --version v0.1.0 --harness claude 2>"$ERR"; rc=$?
assert_refused "unsupported-os" "$before"
assert_diag "unsupported-os" "unsupported operating system"
if [ -s "$TRIP_LOG" ]; then nok "unsupported-os: a network/banned tool was invoked before the tuple check: $(cat "$TRIP_LOG")"; else ok "unsupported-os: curl never invoked (tripwire log empty)"; fi

# --- (3b) unsupported architecture: OS is Linux but uname -m reports mips. The arch map fails,
# still before any curl.
dl_build_sandbox "$PROV" || nok "3b: sandbox build failed"
rm -f "$SANDBOX/bin/uname" "$SANDBOX/bin/curl"
cat > "$SANDBOX/bin/uname" <<'UNAME'
#!/bin/sh
case "$1" in -s) echo Linux ;; -m) echo mips ;; *) echo unknown ;; esac
UNAME
chmod 755 "$SANDBOX/bin/uname"
cp "$TRIPWIRE_SRC" "$SANDBOX/bin/curl"; chmod 755 "$SANDBOX/bin/curl"
dl_case
before=$(dl_state)
dl_run --version v0.1.0 --harness claude 2>"$ERR"; rc=$?
assert_refused "unsupported-arch" "$before"
assert_diag "unsupported-arch" "unsupported architecture"
if [ -s "$TRIP_LOG" ]; then nok "unsupported-arch: curl was invoked before the tuple check: $(cat "$TRIP_LOG")"; else ok "unsupported-arch: curl never invoked (tripwire log empty)"; fi

# ================================================================================================
# GROUP 4 — missing prerequisites: an actionable named failure BEFORE any destination change.
# ================================================================================================

# --- (4a) curl absent.
dl_build_sandbox "$PROV" || nok "4a: sandbox build failed"
rm -f "$SANDBOX/bin/curl"
dl_case
dl_mk_release "$RELEASES" v0.1.0
before=$(dl_state)
dl_run --version v0.1.0 --harness claude 2>"$ERR"; rc=$?
assert_refused "no-curl" "$before"
assert_diag "no-curl" "curl is required"

# --- (4b) tar absent.
dl_build_sandbox "$PROV" || nok "4b: sandbox build failed"
rm -f "$SANDBOX/bin/tar"
dl_case
dl_mk_release "$RELEASES" v0.1.0
before=$(dl_state)
dl_run --version v0.1.0 --harness claude 2>"$ERR"; rc=$?
assert_refused "no-tar" "$before"
assert_diag "no-tar" "tar is required"

# --- (4c) neither sha256sum nor openssl. Build with the provider, then remove it; the other was
# never added by dl_build_sandbox, so no SHA-256 tool remains.
dl_build_sandbox "$PROV" || nok "4c: sandbox build failed"
rm -f "$SANDBOX/bin/sha256sum" "$SANDBOX/bin/openssl"
dl_case
dl_mk_release "$RELEASES" v0.1.0
before=$(dl_state)
dl_run --version v0.1.0 --harness claude 2>"$ERR"; rc=$?
assert_refused "no-sha-tool" "$before"
assert_diag "no-sha-tool" "SHA-256 tool is required"

# ================================================================================================
# GROUP 5 — download failure: the base URL points at an empty release dir, so curl -f fails on the
# very first fetch. Clean refusal, and the scratch dir is cleaned up on exit.
# ================================================================================================
dl_build_sandbox "$PROV" || nok "5: sandbox build failed"
dl_case
# RELEASES is created empty by dl_case and left empty here: no <version>/ dir, no checksums.txt.
before=$(dl_state)
dl_run --version v0.1.0 --harness claude 2>"$ERR"; rc=$?
assert_refused "download-fail" "$before"
assert_diag "download-fail" "failed to download"
if ls "$RUN_TMP"/docket-release.* >/dev/null 2>&1; then nok "download-fail: scratch dir left behind under TMPDIR"; else ok "download-fail: scratch dir cleaned up on exit"; fi

# ================================================================================================
# GROUP 6 — usage errors: exit 2 BEFORE any probe. No sandbox tool is even reachable.
# ================================================================================================

# --- (6a) non-absolute --bin-dir.
dl_build_sandbox "$PROV" || nok "6a: sandbox build failed"
dl_case
before=$(dl_state)
dl_run --version v0.1.0 --bin-dir relative/bin --harness claude 2>"$ERR"; rc=$?
if [ "$rc" = 2 ]; then ok "bad-bin-dir: exits 2 (usage)"; else nok "bad-bin-dir: expected exit 2, got $rc"; fi
assert_refused "bad-bin-dir" "$before"
assert_diag "bad-bin-dir" "absolute path"

# --- (6b) unknown --harness value.
dl_build_sandbox "$PROV" || nok "6b: sandbox build failed"
dl_case
before=$(dl_state)
dl_run --version v0.1.0 --harness bogus 2>"$ERR"; rc=$?
if [ "$rc" = 2 ]; then ok "bad-harness: exits 2 (usage)"; else nok "bad-harness: expected exit 2, got $rc"; fi
assert_refused "bad-harness" "$before"
assert_diag "bad-harness" "unknown harness"

# --- (6c) bad --version (no leading v).
dl_build_sandbox "$PROV" || nok "6c: sandbox build failed"
dl_case
before=$(dl_state)
dl_run --version 1.0.0 --harness claude 2>"$ERR"; rc=$?
if [ "$rc" = 2 ]; then ok "bad-version: exits 2 (usage)"; else nok "bad-version: expected exit 2, got $rc"; fi
assert_refused "bad-version" "$before"
assert_diag "bad-version" "invalid version"

# ================================================================================================
# GROUP 7 — ownership: the downloader replaces only a binary it owns. Foreign bytes at $DEST are
# preserved byte-for-byte in every refusal. A prior binary is written directly (never via a run), so
# the fake log is clean going in.
# ================================================================================================

FOREIGN="$WORK/foreign-bin"
printf '#!/bin/sh\necho foreign\n' > "$FOREIGN"   # not the fake docket, so its hash != $FAKE_SHA

# --- (7a) foreign binary at dest, no record: refuse and preserve it.
dl_build_sandbox "$PROV" || nok "7a: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
cp "$FOREIGN" "$DEST"; chmod 755 "$DEST"
before=$(dl_state)
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
assert_refused "foreign-no-record" "$before"
assert_diag "foreign-no-record" "refusing to replace"

# --- (7b) record present but dest bytes drifted: record names dest with a recorded hash that
# matches neither the current (drifted) bytes nor the requested binary. Refuse.
# THIS is the case the drifted-bytes mutation must redden (see the Task 8 mutation note).
dl_build_sandbox "$PROV" || nok "7b: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
cp "$FOREIGN" "$DEST"; chmod 755 "$DEST"
mkdir -p "$(dirname "$RECORD")"
{ printf 'path=%s\n' "$DEST"; printf 'version=%s\n' "v0.0.9-old"; printf 'sha256=%s\n' "1111111111111111111111111111111111111111111111111111111111111111"; } > "$RECORD"
before=$(dl_state)
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
assert_refused "drifted-owned" "$before"
assert_diag "drifted-owned" "refusing to replace"

# --- (7c) record naming a DIFFERENT bin dir than --bin-dir: the record points elsewhere, and the
# current dest is foreign, so neither the owned nor the converge branch applies. Refuse.
dl_build_sandbox "$PROV" || nok "7c: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
cp "$FOREIGN" "$DEST"; chmod 755 "$DEST"
mkdir -p "$(dirname "$RECORD")"
{ printf 'path=%s\n' "/some/other/dir/docket"; printf 'version=%s\n' "$VER"; printf 'sha256=%s\n' "$(dl_sha "$FOREIGN")"; } > "$RECORD"
before=$(dl_state)
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
assert_refused "record-other-dir" "$before"
assert_diag "record-other-dir" "refusing to replace"

# --- (7d) malformed record: a record too incomplete to establish ownership must not grant it. Two
# variants, each over foreign dest bytes, each preserving record and dest.
#   (7d-i) missing sha256 line.
dl_build_sandbox "$PROV" || nok "7d-i: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
cp "$FOREIGN" "$DEST"; chmod 755 "$DEST"
mkdir -p "$(dirname "$RECORD")"
{ printf 'path=%s\n' "$DEST"; printf 'version=%s\n' "$VER"; } > "$RECORD"   # truncated: no sha256=
before=$(dl_state)
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
assert_refused "malformed-record-truncated" "$before"
assert_diag "malformed-record-truncated" "refusing to replace"

#   (7d-ii) missing path line (a stray extra key present instead) — rec_path empty, ownership fails.
dl_build_sandbox "$PROV" || nok "7d-ii: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
cp "$FOREIGN" "$DEST"; chmod 755 "$DEST"
mkdir -p "$(dirname "$RECORD")"
{ printf 'bogus=%s\n' "x"; printf 'version=%s\n' "$VER"; printf 'sha256=%s\n' "$(dl_sha "$FOREIGN")"; } > "$RECORD"
before=$(dl_state)
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
assert_refused "malformed-record-nopath" "$before"
assert_diag "malformed-record-nopath" "refusing to replace"

# --- (7e) interrupted completion — THE ONE ALLOWED EXCEPTION. Dest already equals the freshly
# verified requested binary and there is no record: the run SUCCEEDS and publishes the record.
dl_build_sandbox "$PROV" || nok "7e: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
cp "$FAKE_DOCKET_SRC" "$DEST"; chmod 755 "$DEST"   # dest == the verified member bytes
[ ! -e "$RECORD" ] || nok "7e: record should be absent going in"
dest_before=$(dl_sha "$DEST")
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
if [ "$rc" = 0 ]; then ok "interrupted-completion: run succeeds (exit 0)"; else nok "interrupted-completion: expected exit 0, got $rc: $(tr '\n' ' ' < "$ERR")"; fi
if [ "$(dl_sha "$DEST")" = "$dest_before" ]; then ok "interrupted-completion: dest bytes unchanged (already the requested binary)"; else nok "interrupted-completion: dest bytes changed"; fi
if grep -qxF "path=$DEST" "$RECORD" 2>/dev/null && grep -qxF "version=$VER" "$RECORD" 2>/dev/null && grep -qxF "sha256=$FAKE_SHA" "$RECORD" 2>/dev/null; then
  ok "interrupted-completion: ownership record published with correct path/version/sha256"
else
  nok "interrupted-completion: record not published correctly: $(tr '\n' '|' < "$RECORD" 2>/dev/null)"
fi
if grep -qF install "$FAKE_DOCKET_LOG"; then ok "interrupted-completion: install ran (converge branch installs the verified binary)"; else nok "interrupted-completion: install never ran"; fi

# ================================================================================================
# GROUP 8 — asset-install failure: `docket install` exits 3 AFTER the bytes are verified. Bytes were
# verified and install DID run, so the invariant here is different: the prior owned binary and its
# record survive byte-for-byte and the stage file is cleaned up. (The fake log DOES carry an install
# line — that is the failed attempt, not an unverified execution.)
# ================================================================================================
dl_build_sandbox "$PROV" || nok "8: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
# A genuinely OWNED prior binary so replacement is ALLOWED and the failure lands mid-transaction.
OWNED="$WORK/owned-bin"; printf '#!/bin/sh\necho owned-prior\n' > "$OWNED"
cp "$OWNED" "$DEST"; chmod 755 "$DEST"
mkdir -p "$(dirname "$RECORD")"
{ printf 'path=%s\n' "$DEST"; printf 'version=%s\n' "v0.0.9-prior"; printf 'sha256=%s\n' "$(dl_sha "$OWNED")"; } > "$RECORD"
before=$(dl_state)
FAIL_ON=install
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
if [ "$rc" != 0 ]; then ok "install-fail: non-zero exit ($rc)"; else nok "install-fail: expected non-zero exit, got 0"; fi
if [ "$(dl_state)" = "$before" ]; then ok "install-fail: prior owned binary and record preserved byte-for-byte"; else nok "install-fail: prior state moved"; fi
if ls "$RUN_BIN"/.docket-stage.* >/dev/null 2>&1; then nok "install-fail: a staging file was left behind"; else ok "install-fail: staging file cleaned up"; fi
assert_diag "install-fail" "docket install failed"

exit "$fail"
