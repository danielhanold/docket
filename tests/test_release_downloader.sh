#!/usr/bin/env bash
# tests/test_release_downloader.sh — guards over the POSIX release downloader
# internal/release/downloader/install.sh (change 0317).
#
# THIS FILE, RIGHT NOW, IS THE STATIC SECTION ONLY (Task 4): the script exists, has the #!/bin/sh
# shebang, carries the @DOCKET_DEFAULT_VERSION@ placeholder render.go stamps, and contains no
# forbidden-interpreter spelling. The hermetic behavior sections — happy paths under a sandboxed
# PATH, both hash providers, umask 077 — are added in Task 7 (Tasks 8-9 add the refusal and
# convergence files). Do not fold behavior assertions in here ahead of that.
#
# THE SPELLING BAN MATCHES SPELLINGS, NOT THE PROPERTY. The final assertion greps install.sh for
# the whole words bash|python|perl|shasum|jq|eval. That is a byte-pattern check: it proves those
# spellings are absent from the source, NOT that the script cannot reach a forbidden interpreter
# some other way (an aliased name, a $(command) built at runtime, a relative path). The SEMANTIC
# guarantee — that a genuinely fresh /bin/sh run touches none of them — is the PATH-sandbox with
# tripwire fakes in Tasks 7-8, where every banned tool is a fake on PATH that trips a log if the
# script ever calls it. This grep is the cheap first line; that sandbox is the real one. Because
# the ban is a pure spelling scan it fires on a comment too, which is exactly why the Task 4
# mutation step (a commented-out eval line must redden this) proves the scan runs at all.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SCRIPT="$REPO/internal/release/downloader/install.sh"
fail=0
ok(){ printf 'ok - %s\n' "$1"; }
nok(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

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
# the header: this is the spelling-level check; Tasks 7-8 own the semantic PATH-sandbox proof.
banned='bash|python|perl|shasum|jq|eval'
if hits="$(grep -nEw -e "$banned" "$SCRIPT")"; then
  nok "forbidden interpreter/helper spelling appears in install.sh (see header; PATH-sandbox is the semantic check):"
  printf '%s\n' "$hits" | sed 's/^/    /'
else
  ok "no forbidden spelling (bash|python|perl|shasum|jq|eval) appears in install.sh"
fi

exit "$fail"
