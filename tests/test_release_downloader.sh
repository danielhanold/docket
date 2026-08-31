#!/usr/bin/env bash
# docket-suite: posix-downloader
# tests/test_release_downloader.sh — guards over the POSIX release downloader
# internal/release/downloader/install.sh (change 0317).
#
# STRUCTURE. Section A is the static source scan from Task 4: the script exists, has the #!/bin/sh
# shebang, carries the @DOCKET_DEFAULT_VERSION@ placeholder render.go stamps, and contains no
# forbidden-interpreter spelling. Sections B–F are the Task 7 hermetic behavior tests — a genuinely
# fresh /bin/sh runs the downloader under a locked-down PATH sandbox against a file:// "release"
# built inline, proving the checksum-verified install path with BOTH hash providers (sha256sum and
# OpenSSL), under umask 077, with an argument-capturing fake docket. Tasks 8–9 add the refusal and
# convergence sibling FILES.
#
# THE SPELLING BAN (Section A) MATCHES SPELLINGS, NOT THE PROPERTY. Section A's final assertion greps
# install.sh for the whole words bash|python|perl|shasum|jq|eval. That is a byte-pattern check: it
# proves those spellings are absent from the source, NOT that the script cannot reach a forbidden
# interpreter some other way (an aliased name, a $(command) built at runtime, a relative path). The
# SEMANTIC guarantee — that a genuinely fresh /bin/sh run touches none of them — is Section B+'s
# PATH-sandbox with tripwire fakes, where every banned tool is a fake on PATH that trips a log if the
# script ever calls it. This grep is the cheap first line; that sandbox is the real one. Because the
# ban is a pure spelling scan it fires on a comment too, which is exactly why the Task 4 mutation
# step (a commented-out eval line must redden this) proves the scan runs at all.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SCRIPT="$REPO/internal/release/downloader/install.sh"
fail=0
ok(){ printf 'ok - %s\n' "$1"; }
nok(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# ================================================================================================
# SECTION A — static source scan (Task 4)
# ================================================================================================

# --- (1) the script exists ----------------------------------------------------------------------
if [ -f "$SCRIPT" ]; then
  ok "downloader script exists at internal/release/downloader/install.sh"
else
  nok "downloader script missing at internal/release/downloader/install.sh"
  # Nothing below can run without the file; report what we have and stop.
  exit "$fail"
fi

# --- (2) POSIX shebang, not bash ----------------------------------------------------------------
# Read the first line directly from the file (no pipe, so no producer SIGPIPE under pipefail).
IFS= read -r first_line < "$SCRIPT" || first_line=""
if [ "$first_line" = "#!/bin/sh" ]; then
  ok "shebang is #!/bin/sh"
else
  nok "first line is not the #!/bin/sh shebang (got: ${first_line})"
fi

# --- (3) the render placeholder is present ------------------------------------------------------
# render.go (Task 5) stamps this exact token with the bundle version; its absence would make every
# rendered bundle unversioned. Pattern does not lead with '-', so a plain -F match is safe.
if grep -qF -- '@DOCKET_DEFAULT_VERSION@' "$SCRIPT"; then
  ok "carries the @DOCKET_DEFAULT_VERSION@ render placeholder"
else
  nok "missing the @DOCKET_DEFAULT_VERSION@ render placeholder"
fi

# --- (4) no forbidden-interpreter spelling ------------------------------------------------------
# Whole-word scan (-w) for each forbidden spelling anywhere in the source, comments included. See
# the header: this is the spelling-level check; Sections B+ own the semantic PATH-sandbox proof.
banned='bash|python|perl|shasum|jq|eval'
if hits="$(grep -nEw -e "$banned" "$SCRIPT")"; then
  nok "forbidden interpreter/helper spelling appears in install.sh (see header; PATH-sandbox is the semantic check):"
  printf '%s\n' "$hits" | sed 's/^/    /'
else
  ok "no forbidden spelling (bash|python|perl|shasum|jq|eval) appears in install.sh"
fi

# --- (5) render.go embeds this downloader (the embed tie) ---------------------------------------
# Relocated class-C invariant: this assertion previously lived in tests/test_release_package.sh
# (SECTION H), a file whose surviving invariants are decomposed and whose body Task 8 deletes. The
# downloader reaches users ONLY as render.go's embedded copy, so a dropped go:embed directive would
# silently unship it — that makes the embed tie a downloader-product invariant, homed here.
# SPELLING LIMIT: this greps render.go for the go:embed directive naming downloader/install.sh; it
# does not compile the package (the Go build is the semantic proof that the embedded path resolves).
RENDER="$REPO/internal/release/render.go"
if [ -f "$RENDER" ] && grep -Eq '^//go:embed[[:space:]]+downloader/install\.sh' "$RENDER"; then
  ok "internal/release/render.go embeds downloader/install.sh"
else
  nok "internal/release/render.go does not embed downloader/install.sh"
fi

# ================================================================================================
# SECTION B+ — hermetic behavior (Task 7)
#
# TWIN NOTE: the helper block below (the globals, the fake docket + tripwire fixtures,
# dl_build_sandbox / dl_mk_release / dl_case / dl_run, and dl_sha / pmode / have) is COPIED
# verbatim into tests/test_release_downloader_refusals.sh (Task 8) and
# tests/test_release_downloader_converge.sh (Task 9). Each of those files is hermetic per
# tests/README.md — it sources nothing — so the block lives in each. Keep the copies in sync when
# the downloader contract moves.
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

# --- (B) fresh install, sha256sum provider ------------------------------------------------------
if have sha256sum; then
  dl_build_sandbox sha256sum || nok "B: sha256sum sandbox build failed"
  dl_case
  VER=v0.1.0
  dl_mk_release "$RELEASES" "$VER"
  EXP_SHA=$(dl_sha "$FAKE_DOCKET_SRC")
  dl_run --version "$VER" --harness claude --harness opencode; rc=$?

  if [ "$rc" = 0 ]; then ok "sha256sum: fresh install exits 0"; else nok "sha256sum: fresh install exit $rc"; fi
  if [ -f "$DEST" ]; then ok "sha256sum: binary installed at XDG_BIN_HOME/docket"; else nok "sha256sum: binary missing at $DEST"; fi
  if [ "$(pmode "$DEST")" = 755 ]; then ok "sha256sum: installed binary is mode 755 under umask 077"; else nok "sha256sum: dest mode is $(pmode "$DEST" 2>/dev/null), want 755"; fi

  # install (with the two forwarded --harness pairs) THEN install check, in that order.
  log_want="install --harness claude --harness opencode
install check"
  if [ "$(cat "$FAKE_DOCKET_LOG")" = "$log_want" ]; then
    ok "sha256sum: fake docket saw 'install --harness claude --harness opencode' then 'install check', in order"
  else
    nok "sha256sum: fake docket log mismatch: $(tr '\n' '|' < "$FAKE_DOCKET_LOG")"
  fi

  if grep -qxF "path=$DEST" "$RECORD" && grep -qxF "version=$VER" "$RECORD" && grep -qxF "sha256=$EXP_SHA" "$RECORD"; then
    ok "sha256sum: ownership record has correct path=/version=/sha256="
  else
    nok "sha256sum: ownership record wrong: $(tr '\n' '|' < "$RECORD" 2>/dev/null)"
  fi

  if ls "$RUN_BIN"/.docket-stage.* >/dev/null 2>&1; then nok "sha256sum: a staging file was left behind"; else ok "sha256sum: no staging file left behind"; fi
  if [ -s "$TRIP_LOG" ]; then nok "sha256sum: a banned tool was invoked: $(cat "$TRIP_LOG")"; else ok "sha256sum: no banned interpreter/helper invoked (tripwire log empty)"; fi
else
  ok "sha256sum absent on host — sha256sum happy-path case skipped"
fi

# --- (C) fresh install, OpenSSL provider (sha256sum removed from the sandbox) --------------------
# Same outcome via the other hashing branch, so neither branch is a green-but-unexecuted path.
if have openssl; then
  dl_build_sandbox openssl || nok "C: openssl sandbox build failed"
  dl_case
  VER=v0.1.0
  dl_mk_release "$RELEASES" "$VER"
  EXP_SHA=$(dl_sha "$FAKE_DOCKET_SRC")
  dl_run --version "$VER" --harness claude --harness opencode; rc=$?

  if [ ! -e "$SANDBOX/bin/sha256sum" ]; then ok "openssl: sha256sum is absent from the sandbox, so the OpenSSL hashing branch is exercised"; else nok "openssl: sha256sum should not be in the sandbox for this case"; fi
  if [ "$rc" = 0 ]; then ok "openssl: fresh install exits 0"; else nok "openssl: fresh install exit $rc"; fi
  if [ -f "$DEST" ]; then ok "openssl: binary installed at XDG_BIN_HOME/docket"; else nok "openssl: binary missing at $DEST"; fi
  if [ "$(pmode "$DEST")" = 755 ]; then ok "openssl: installed binary is mode 755 under umask 077"; else nok "openssl: dest mode is $(pmode "$DEST" 2>/dev/null), want 755"; fi

  log_want="install --harness claude --harness opencode
install check"
  if [ "$(cat "$FAKE_DOCKET_LOG")" = "$log_want" ]; then
    ok "openssl: fake docket saw install then install check, in order"
  else
    nok "openssl: fake docket log mismatch: $(tr '\n' '|' < "$FAKE_DOCKET_LOG")"
  fi

  if grep -qxF "path=$DEST" "$RECORD" && grep -qxF "version=$VER" "$RECORD" && grep -qxF "sha256=$EXP_SHA" "$RECORD"; then
    ok "openssl: ownership record has correct path=/version=/sha256="
  else
    nok "openssl: ownership record wrong: $(tr '\n' '|' < "$RECORD" 2>/dev/null)"
  fi

  if ls "$RUN_BIN"/.docket-stage.* >/dev/null 2>&1; then nok "openssl: a staging file was left behind"; else ok "openssl: no staging file left behind"; fi
  if [ -s "$TRIP_LOG" ]; then nok "openssl: a banned tool was invoked: $(cat "$TRIP_LOG")"; else ok "openssl: no banned interpreter/helper invoked (tripwire log empty)"; fi
else
  ok "openssl absent on host — openssl happy-path case skipped"
fi

# --- (D) default version: no --version uses the rendered @DOCKET_DEFAULT_VERSION@ stamp ----------
if have sha256sum; then
  dl_build_sandbox sha256sum || nok "D: sandbox build failed"
  dl_case
  VER=v0.0.1-default
  # Render a copy with the placeholder replaced by the fixture version — one sed at the write boundary.
  RENDERED="$WORK/install.rendered.sh"
  sed "s/@DOCKET_DEFAULT_VERSION@/$VER/" "$SCRIPT" > "$RENDERED"
  # Prove the render actually landed (a silent no-op would leave the case vacuous).
  if grep -qF -- '@DOCKET_DEFAULT_VERSION@' "$RENDERED"; then
    nok "default-version: placeholder still present after render"
  else
    ok "default-version: render replaced the @DOCKET_DEFAULT_VERSION@ placeholder with $VER"
  fi
  DL_SCRIPT="$RENDERED"
  dl_mk_release "$RELEASES" "$VER"
  dl_run --harness claude; rc=$?

  if [ "$rc" = 0 ]; then ok "default-version: run with no --version exits 0"; else nok "default-version: exit $rc"; fi
  if grep -qxF "version=$VER" "$RECORD"; then
    ok "default-version: fetched and recorded the stamped default version $VER"
  else
    nok "default-version: recorded version wrong: $(grep '^version=' "$RECORD" 2>/dev/null)"
  fi
  if [ -s "$TRIP_LOG" ]; then nok "default-version: a banned tool was invoked"; else ok "default-version: tripwire log empty"; fi
else
  ok "sha256sum absent on host — default-version case skipped"
fi

# --- (E) idempotent rerun: the same version again is allowed and leaves the record unchanged -----
if have sha256sum; then
  dl_build_sandbox sha256sum || nok "E: sandbox build failed"
  dl_case
  VER=v0.1.0
  dl_mk_release "$RELEASES" "$VER"
  dl_run --version "$VER" --harness claude; rc1=$?
  rec_before=$(cat "$RECORD" 2>/dev/null)
  : > "$FAKE_DOCKET_LOG"; : > "$TRIP_LOG"
  dl_run --version "$VER" --harness claude; rc2=$?
  rec_after=$(cat "$RECORD" 2>/dev/null)

  if [ "$rc1" = 0 ] && [ "$rc2" = 0 ]; then ok "idempotent: both runs of the same version exit 0"; else nok "idempotent: exits $rc1 then $rc2"; fi
  if [ -n "$rec_before" ] && [ "$rec_before" = "$rec_after" ]; then ok "idempotent: ownership record unchanged across the rerun"; else nok "idempotent: record changed across rerun"; fi
  if [ -s "$TRIP_LOG" ]; then nok "idempotent: a banned tool was invoked on rerun"; else ok "idempotent: tripwire log empty on rerun"; fi
else
  ok "sha256sum absent on host — idempotent-rerun case skipped"
fi

# --- (F) mutation pass: the tripwire assert is non-vacuous, and verification precedes install ----
# (F1) Directly invoke a banned-tool tripwire once. TRIP_LOG must become non-empty and the tool must
# exit 9 — so every "tripwire log empty" assert above is a real guard, able to redden, not decoration.
if have sha256sum; then dl_build_sandbox sha256sum >/dev/null 2>&1 || true; else dl_build_sandbox openssl >/dev/null 2>&1 || true; fi
: > "$TRIP_LOG"
env -i PATH="$SANDBOX/bin" TRIP_LOG="$TRIP_LOG" "$SANDBOX/bin/bash" -c ':'; trc=$?
if [ "$trc" = 9 ] && [ -s "$TRIP_LOG" ]; then
  ok "mutation: invoking a banned tool trips the wire (exit 9, TRIP_LOG non-empty) — the empty-log assert can redden"
else
  nok "mutation: tripwire did not fire (exit $trc; log-empty=$([ -s "$TRIP_LOG" ] && echo no || echo yes))"
fi

# (F2) Flip one byte in the release archive. Checksum verification must fail BEFORE extraction, so
# the run is non-zero AND the fake docket's install is never reached — no 'install' line is logged.
if have sha256sum; then
  dl_build_sandbox sha256sum || nok "F2: sandbox build failed"
  dl_case
  VER=v0.1.0
  dl_mk_release "$RELEASES" "$VER"
  _os=$(uname -s); case "$_os" in Darwin) _os=darwin ;; Linux) _os=linux ;; esac
  _arch=$(uname -m); case "$_arch" in x86_64|amd64) _arch=amd64 ;; arm64|aarch64) _arch=arm64 ;; esac
  ARC="$RELEASES/$VER/docket_${VER}_${_os}_${_arch}.tar.gz"
  # Flip the byte at offset 40 to a guaranteed-different value — any change moves the archive SHA-256.
  orig=$(dd if="$ARC" bs=1 skip=40 count=1 2>/dev/null | od -An -tu1 | tr -d ' ')
  new=$(( (orig + 1) % 256 ))
  printf "$(printf '\\%03o' "$new")" | dd of="$ARC" bs=1 seek=40 count=1 conv=notrunc 2>/dev/null
  : > "$FAKE_DOCKET_LOG"; : > "$TRIP_LOG"
  dl_run --version "$VER" --harness claude; rc=$?

  if [ "$rc" != 0 ]; then ok "mutation: corrupted archive is rejected (non-zero exit $rc)"; else nok "mutation: corrupted archive was accepted (exit 0)"; fi
  if grep -qF install "$FAKE_DOCKET_LOG"; then
    nok "mutation: install ran on unverified bytes (an install line reached the fake docket)"
  else
    ok "mutation: no 'install' reached the fake docket — verification precedes install"
  fi
  if [ ! -e "$DEST" ]; then ok "mutation: no binary was installed from the corrupt archive"; else nok "mutation: a binary was installed despite the checksum mismatch"; fi
  if [ -s "$TRIP_LOG" ]; then nok "mutation: a banned tool was invoked on the corrupt-archive path"; else ok "mutation: tripwire log empty on the corrupt-archive path"; fi
else
  ok "sha256sum absent on host — corrupt-archive mutation case skipped"
fi

exit "$fail"
