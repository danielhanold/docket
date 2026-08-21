#!/usr/bin/env bash
# tests/test_release_downloader_converge.sh — interruption-point convergence and upgrade guards over
# the POSIX release downloader internal/release/downloader/install.sh (change 0317, Task 9).
#
# WHAT THIS PROVES. The downloader's install sequence has interruption points, and a rerun after any
# of them must CONVERGE — it must reach the requested version without ever replacing a binary it does
# not own. Two kinds of interruption are injected:
#   - FAIL_ON (install|check): the fake docket in the staged archive fails at a chosen point of its
#     OWN run — before the binary is moved into place (install), or at the final read-only check.
#   - A DOCTORED COPY of the downloader with a single filesystem-boundary line replaced by `exit 97`:
#     the rename `mv -f "$stage" "$dest"` (interrupt AFTER asset install, BEFORE rename) or the
#     record-publish `mv -f "$record_tmp" "$record"` (interrupt AFTER rename, BEFORE the record is
#     published). The copy is written by dl_doctor at the test's write boundary, and dl_doctor
#     REFUSES unless the doctoring changed exactly one line — a silent no-op doctoring would make
#     every interruption case vacuously run the happy path (learning assert-detects-removal-not-
#     replacement). Case (F) proves that refusal reddens.
#
# The convergence claim in each interruption case is: the interrupted run leaves the prior state
# exactly as the spec's ordering promises (nothing moved before its step ran), and a rerun of the
# REAL script then reaches the requested version and repairs the ownership record — via the
# owned-record branch when the old binary is still at dest, or the interrupted-completion branch when
# the new bytes already are. The upgrade case proves a run at a newer version replaces ONLY the owned
# binary bytes and the record, leaving an unrelated harness asset byte-for-byte intact.
#
# TWIN NOTE: the helper block below (the globals, the fake docket + tripwire fixtures,
# dl_build_sandbox / dl_mk_release / dl_case / dl_run, and dl_sha / pmode / have) is COPIED verbatim
# from tests/test_release_downloader.sh (Task 7) and is also carried by
# tests/test_release_downloader_refusals.sh (Task 8). Each of those files is hermetic per
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

# Portable file-mode read: BSD stat first, then GNU stat.
pmode(){ stat -f '%Lp' "$1" 2>/dev/null || stat -c '%a' "$1"; }

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
# it is not symlinked; dirname IS used by the record path and is required). The banned set gets a
# tripwire each.
DL_REAL_TOOLS='curl tar uname mktemp mkdir mv cp chmod rm grep sed cat dirname'
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
# Convergence-specific helpers (NOT part of the twin block).
# ================================================================================================

ERR="$WORK/stderr.log"

# A distinct "old" owned binary — different bytes from the fake docket member, so that "dest still
# the old binary" and "dest is now the requested bytes" are distinguishable hashes.
OLD_BIN="$WORK/old-owned-binary"
printf '#!/bin/sh\necho old-owned-docket\nexit 0\n' > "$OLD_BIN"
chmod 755 "$OLD_BIN"

# The hash the script computes for the verified fake-docket member (dl_mk_release copies
# FAKE_DOCKET_SRC as the archive member, so bin_sha == this).
FAKE_SHA=$(dl_sha "$FAKE_DOCKET_SRC")

# Plant an OWNED prior state at $DEST/$RECORD: dest = the bytes of $1, record naming dest with
# version $2 and the matching sha256. This is the state that makes the ownership decision "owned",
# so a run is ALLOWED to reach the install/rename/publish sequence where the interruption lands.
cv_plant_owned(){
  cp "$1" "$DEST"; chmod 755 "$DEST"
  mkdir -p "$(dirname "$RECORD")"
  { printf 'path=%s\n' "$DEST"; printf 'version=%s\n' "$2"; printf 'sha256=%s\n' "$(dl_sha "$1")"; } > "$RECORD"
}

# Build a file:// release at <parent>/<version>/ whose single archive member is the binary <3>
# (dl_mk_release always uses FAKE_DOCKET_SRC; the upgrade case needs a DIFFERENT member for release
# B). Real tar + real hashing, host tuple, zero network — same shape as dl_mk_release.
cv_mk_release_member(){
  _parent="$1"; _ver="$2"; _member="$3"
  _os=$(uname -s); case "$_os" in Darwin) _os=darwin ;; Linux) _os=linux ;; esac
  _arch=$(uname -m); case "$_arch" in x86_64|amd64) _arch=amd64 ;; arm64|aarch64) _arch=arm64 ;; esac
  _rel="$_parent/$_ver"; mkdir -p "$_rel"
  _arc="docket_${_ver}_${_os}_${_arch}.tar.gz"
  _mem=$(mktemp -d "${TMPDIR:-/tmp}/cv-member.XXXXXX")
  cp "$_member" "$_mem/docket"; chmod 755 "$_mem/docket"
  tar -czf "$_rel/$_arc" -C "$_mem" docket
  rm -rf "$_mem"
  _h=$(dl_sha "$_rel/$_arc")
  printf '%s  %s\n' "$_h" "$_arc" > "$_rel/checksums.txt"
}

# Write a DOCTORED copy of the downloader to $1 with the single line CONTAINING the literal needle
# $2 replaced by `exit 97`. awk index() matches the needle LITERALLY — the target lines carry
# $stage/$dest/$record_tmp, every one of which is regex-significant to sed. Returns 0 ONLY if exactly
# one line matched AND the copy differs from the original by exactly that one line: a one-line
# replacement yields exactly one `<` and one `>` in diff, so a diff of 2 marker lines is the proof
# the mutation landed and is not a silent no-op (learning assert-detects-removal-not-replacement).
# Emits no ok/nok of its own; every caller asserts on the return status, so case (F) can exercise the
# refusal path without tripping a failure.
dl_doctor(){
  _out="$1"; _needle="$2"
  awk -v needle="$_needle" '
    index($0, needle) { print "exit 97"; n++; next }
    { print }
    END { exit (n == 1 ? 0 : 1) }
  ' "$SCRIPT" > "$_out" || return 1
  _n=$(diff "$SCRIPT" "$_out" | grep -Ec '^[<>]') || true
  [ "$_n" = 2 ] || return 1
  chmod 755 "$_out" 2>/dev/null || true
  return 0
}

# Pick whichever hashing provider the host has; provider-specific coverage is Task 7's concern.
if have sha256sum; then PROV=sha256sum
elif have openssl; then PROV=openssl
else nok "host has neither sha256sum nor openssl — cannot run the convergence suite"; exit "$fail"
fi

# ================================================================================================
# CASE 1 — fail BEFORE asset install (FAIL_ON=install). The staged binary's own `install` fails
# before the rename, so the prior owned state must be fully restored: old binary + old record
# byte-identical, no stage left behind.
# ================================================================================================
dl_build_sandbox "$PROV" || nok "1: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
cv_plant_owned "$OLD_BIN" "v0.0.9-old"
old_dest=$(dl_sha "$DEST"); old_rec=$(dl_sha "$RECORD")
FAIL_ON=install
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
if [ "$rc" != 0 ]; then ok "fail-before-install: non-zero exit ($rc)"; else nok "fail-before-install: expected non-zero exit, got 0"; fi
if [ "$(dl_sha "$DEST")" = "$old_dest" ]; then ok "fail-before-install: old binary at dest is byte-identical"; else nok "fail-before-install: dest bytes moved"; fi
if [ "$(dl_sha "$RECORD")" = "$old_rec" ]; then ok "fail-before-install: old ownership record is byte-identical"; else nok "fail-before-install: record moved"; fi
if ls "$RUN_BIN"/.docket-stage.* >/dev/null 2>&1; then nok "fail-before-install: a staging file was left behind"; else ok "fail-before-install: no staging file left behind"; fi
if grep -qF install "$FAKE_DOCKET_LOG"; then ok "fail-before-install: install was attempted (the failure is mid-transaction, not pre-verification)"; else nok "fail-before-install: install never ran — the fixture did not reach the failing step"; fi

# ================================================================================================
# CASE 2 — fail AFTER asset install, BEFORE rename (doctored copy: the rename line -> exit 97). The
# staged install succeeds but the binary is never moved into place, so dest + record are still the
# OLD owned state. A rerun of the REAL script then converges via the owned-record branch.
# ================================================================================================
dl_build_sandbox "$PROV" || nok "2: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
cv_plant_owned "$OLD_BIN" "v0.0.9-old"
old_dest=$(dl_sha "$DEST"); old_rec=$(dl_sha "$RECORD")
DOCTORED="$WORK/install.doctored-rename.sh"
if dl_doctor "$DOCTORED" 'mv -f "$stage" "$dest"'; then ok "before-rename: doctored copy replaces exactly the rename line with exit 97 (one line changed)"; else nok "before-rename: doctoring the rename line did not change exactly one line"; fi
DL_SCRIPT="$DOCTORED"
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
if [ "$rc" = 97 ]; then ok "before-rename: doctored run stops at the injected exit 97"; else nok "before-rename: expected exit 97, got $rc"; fi
if grep -qF install "$FAKE_DOCKET_LOG"; then ok "before-rename: asset install DID run before the interruption"; else nok "before-rename: asset install did not run"; fi
if [ "$(dl_sha "$DEST")" = "$old_dest" ]; then ok "before-rename: dest is still the old binary (rename never happened)"; else nok "before-rename: dest bytes moved despite the interrupted rename"; fi
if [ "$(dl_sha "$RECORD")" = "$old_rec" ]; then ok "before-rename: record is still the old record"; else nok "before-rename: record moved before the rename"; fi
if ls "$RUN_BIN"/.docket-stage.* >/dev/null 2>&1; then nok "before-rename: the stage file was left behind by the interrupted run"; else ok "before-rename: interrupted run cleaned up its stage file"; fi
# Rerun the REAL script — it must converge to the requested version via the owned-record branch.
DL_SCRIPT="$SCRIPT"
: > "$FAKE_DOCKET_LOG"; : > "$TRIP_LOG"
dl_run --version "$VER" --harness claude 2>"$ERR"; rc2=$?
if [ "$rc2" = 0 ]; then ok "before-rename: rerun of the real script converges (exit 0)"; else nok "before-rename: rerun did not converge, exit $rc2: $(tr '\n' ' ' < "$ERR")"; fi
if [ "$(dl_sha "$DEST")" = "$FAKE_SHA" ]; then ok "before-rename: after rerun, dest is the requested binary bytes"; else nok "before-rename: dest is not the requested binary after rerun"; fi
if grep -qxF "version=$VER" "$RECORD" 2>/dev/null && grep -qxF "sha256=$FAKE_SHA" "$RECORD" 2>/dev/null; then ok "before-rename: record updated to the requested version and hash"; else nok "before-rename: record not updated: $(tr '\n' '|' < "$RECORD" 2>/dev/null)"; fi
if [ -s "$TRIP_LOG" ]; then nok "before-rename: a banned tool was invoked on the converging rerun"; else ok "before-rename: no banned interpreter/helper invoked on the rerun (tripwire log empty)"; fi

# ================================================================================================
# CASE 3 — fail AFTER rename, BEFORE record publication (doctored copy: the record-publish line ->
# exit 97). The binary IS moved into place but the record is never published, so dest = new bytes
# while record = old. A rerun then converges via the interrupted-completion branch and repairs the
# record.
# ================================================================================================
dl_build_sandbox "$PROV" || nok "3: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
cv_plant_owned "$OLD_BIN" "v0.0.9-old"
old_rec=$(dl_sha "$RECORD")
DOCTORED="$WORK/install.doctored-record.sh"
if dl_doctor "$DOCTORED" 'mv -f "$record_tmp" "$record"'; then ok "before-record: doctored copy replaces exactly the record-publish line with exit 97 (one line changed)"; else nok "before-record: doctoring the record-publish line did not change exactly one line"; fi
DL_SCRIPT="$DOCTORED"
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
if [ "$rc" = 97 ]; then ok "before-record: doctored run stops at the injected exit 97"; else nok "before-record: expected exit 97, got $rc"; fi
if [ "$(dl_sha "$DEST")" = "$FAKE_SHA" ]; then ok "before-record: dest is already the new bytes (rename ran before the interruption)"; else nok "before-record: dest is not the new bytes despite the rename having run"; fi
if [ "$(dl_sha "$RECORD")" = "$old_rec" ]; then ok "before-record: record is still the old record (publication never happened)"; else nok "before-record: record moved despite the interrupted publication"; fi
# Rerun the REAL script — dest already equals the verified requested binary, so it converges via the
# interrupted-completion branch and repairs the record.
DL_SCRIPT="$SCRIPT"
: > "$FAKE_DOCKET_LOG"; : > "$TRIP_LOG"
dl_run --version "$VER" --harness claude 2>"$ERR"; rc2=$?
if [ "$rc2" = 0 ]; then ok "before-record: rerun converges via interrupted-completion (exit 0)"; else nok "before-record: rerun did not converge, exit $rc2: $(tr '\n' ' ' < "$ERR")"; fi
if [ "$(dl_sha "$DEST")" = "$FAKE_SHA" ]; then ok "before-record: dest remains the requested binary bytes after rerun"; else nok "before-record: dest changed away from the requested binary"; fi
if grep -qxF "version=$VER" "$RECORD" 2>/dev/null && grep -qxF "sha256=$FAKE_SHA" "$RECORD" 2>/dev/null; then ok "before-record: record repaired to the requested version and hash"; else nok "before-record: record not repaired: $(tr '\n' '|' < "$RECORD" 2>/dev/null)"; fi
if [ -s "$TRIP_LOG" ]; then nok "before-record: a banned tool was invoked on the converging rerun"; else ok "before-record: no banned interpreter/helper invoked on the rerun (tripwire log empty)"; fi

# ================================================================================================
# CASE 4 — fail at the FINAL check (FAIL_ON=check). Steps 3-4 (rename + record publish) precede step
# 5 (exec docket install check), so the binary and record are ALREADY published when the check
# fails; the script exits with the check's status. A rerun with a passing check exits 0.
# ================================================================================================
dl_build_sandbox "$PROV" || nok "4: sandbox build failed"
dl_case
VER=v0.1.0
dl_mk_release "$RELEASES" "$VER"
FAIL_ON=check
dl_run --version "$VER" --harness claude 2>"$ERR"; rc=$?
if [ "$rc" = 4 ]; then ok "fail-at-check: script exits with the check's own status (4)"; else nok "fail-at-check: expected the check's status 4, got $rc"; fi
if [ "$(dl_sha "$DEST")" = "$FAKE_SHA" ]; then ok "fail-at-check: binary was already published before the check ran"; else nok "fail-at-check: binary not published at dest"; fi
if grep -qxF "version=$VER" "$RECORD" 2>/dev/null && grep -qxF "sha256=$FAKE_SHA" "$RECORD" 2>/dev/null; then ok "fail-at-check: record was already published before the check ran"; else nok "fail-at-check: record not published: $(tr '\n' '|' < "$RECORD" 2>/dev/null)"; fi
# Rerun with the check passing — converges to exit 0.
FAIL_ON=''
: > "$FAKE_DOCKET_LOG"; : > "$TRIP_LOG"
dl_run --version "$VER" --harness claude 2>"$ERR"; rc2=$?
if [ "$rc2" = 0 ]; then ok "fail-at-check: rerun exits 0 once the check passes"; else nok "fail-at-check: rerun did not converge, exit $rc2: $(tr '\n' ' ' < "$ERR")"; fi

# ================================================================================================
# CASE 5 — upgrade. Install release A (v0.0.1-a), then run --version v0.0.2-b from a second release
# dir whose member is DIFFERENT bytes. The owned binary is replaced with B's bytes and the record
# names B; an unrelated harness asset planted before the upgrade is byte-identical after — the
# downloader replaces only the bytes it owns.
# ================================================================================================
dl_build_sandbox "$PROV" || nok "5: sandbox build failed"
dl_case
VERA=v0.0.1-a
VERB=v0.0.2-b
# Member B: behaviorally identical to the fake docket, different bytes (an appended comment).
FAKE2="$WORK/fake-docket-b"
cp "$FAKE_DOCKET_SRC" "$FAKE2"; printf '%s\n' '# release B build — distinct bytes' >> "$FAKE2"; chmod 755 "$FAKE2"
SHA_A=$(dl_sha "$FAKE_DOCKET_SRC")
SHA_B=$(dl_sha "$FAKE2")
if [ "$SHA_A" != "$SHA_B" ]; then ok "upgrade: release A and B members differ in bytes (non-vacuous upgrade)"; else nok "upgrade: members A and B have the same bytes"; fi
dl_mk_release "$RELEASES" "$VERA"
cv_mk_release_member "$RELEASES" "$VERB" "$FAKE2"
# Install A.
dl_run --version "$VERA" --harness claude 2>"$ERR"; rca=$?
if [ "$rca" = 0 ]; then ok "upgrade: base install of $VERA exits 0"; else nok "upgrade: base install failed, exit $rca: $(tr '\n' ' ' < "$ERR")"; fi
if [ "$(dl_sha "$DEST")" = "$SHA_A" ]; then ok "upgrade: dest is A's bytes after the base install"; else nok "upgrade: dest is not A's bytes after the base install"; fi
# Plant an unrelated harness asset and record its bytes.
HARNESS_DIR="$RUN_HOME/.local/share/docket/harness-assets"
mkdir -p "$HARNESS_DIR"
SENTINEL="$HARNESS_DIR/asset.bin"
printf '%s\n' 'harness-asset-payload-v1' > "$SENTINEL"
sent_before=$(dl_sha "$SENTINEL")
# Upgrade to B (owned-record branch: dest = A, record names dest with A's hash).
: > "$FAKE_DOCKET_LOG"; : > "$TRIP_LOG"
dl_run --version "$VERB" --harness claude 2>"$ERR"; rcb=$?
if [ "$rcb" = 0 ]; then ok "upgrade: run --version $VERB exits 0"; else nok "upgrade: upgrade run failed, exit $rcb: $(tr '\n' ' ' < "$ERR")"; fi
if grep -qxF "version=$VERB" "$RECORD" 2>/dev/null && grep -qxF "sha256=$SHA_B" "$RECORD" 2>/dev/null; then ok "upgrade: record now names B's version and hash"; else nok "upgrade: record does not name B: $(tr '\n' '|' < "$RECORD" 2>/dev/null)"; fi
if [ "$(dl_sha "$DEST")" = "$SHA_B" ]; then ok "upgrade: dest is now B's bytes"; else nok "upgrade: dest is not B's bytes after the upgrade"; fi
if [ "$(dl_sha "$SENTINEL")" = "$sent_before" ]; then ok "upgrade: the planted harness asset is byte-identical after the upgrade"; else nok "upgrade: the harness asset changed across the upgrade"; fi
if [ -s "$TRIP_LOG" ]; then nok "upgrade: a banned tool was invoked during the upgrade"; else ok "upgrade: no banned interpreter/helper invoked during the upgrade (tripwire log empty)"; fi

# ================================================================================================
# CASE F — mutation: dl_doctor's exactly-one-line guard is non-vacuous. A needle that matches NO line
# must be REJECTED — otherwise a silent no-op copy would make cases 2-3 run the happy path and pass
# vacuously. We exercise dl_doctor on a guaranteed-absent needle and confirm it neither claims
# success nor injects an `exit 97` line (an empty diff).
# ================================================================================================
MUT="$WORK/install.mutation-noop.sh"
if dl_doctor "$MUT" 'this-literal-needle-appears-in-no-line-of-the-downloader-source'; then
  nok "mutation: dl_doctor accepted a no-op doctoring (a needle matching no line) — the interruption cases could be vacuous"
else
  ok "mutation: dl_doctor rejects a no-op doctoring (a needle matching no line changes nothing)"
fi
if [ -f "$MUT" ] && grep -qxF 'exit 97' "$MUT"; then
  nok "mutation: a no-op doctoring still injected an exit 97 line"
else
  ok "mutation: a no-op doctoring injected no exit 97 line (its diff against the source is empty)"
fi

exit "$fail"
