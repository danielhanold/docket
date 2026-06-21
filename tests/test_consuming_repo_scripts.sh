#!/usr/bin/env bash
# tests/test_consuming_repo_scripts.sh — the DOCKET_SCRIPTS_DIR drift-guard (change 0034).
# The repo has no GitHub Actions CI; this test-suite file is the de-facto gate, mirroring
# test_change_links_coverage.sh and how test_sync_agents.sh exercises sync-agents.sh --check.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
ok(){ printf 'ok   - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# (1) STATIC AUDIT: no skill body invokes a bare, CWD-relative scripts/<concrete-name>.sh.
#     A concrete name has a lowercase letter right after the slash; the placeholder
#     scripts/<name>.sh (literal "<") is intentionally exempt.
audit_fail=0
while IFS= read -r f; do
  hits="$(grep -nE 'scripts/[a-z][a-z0-9-]*\.sh' "$f" || true)"
  if [ -n "$hits" ]; then
    no "bare scripts/<name>.sh in ${f#"$REPO"/}"; printf '%s\n' "$hits"; audit_fail=1
  fi
done < <(find "$REPO/skills" -name '*.md' | sort)
[ "$audit_fail" = 0 ] && ok "no skill body uses a bare scripts/<name>.sh path"

# (2) RESOLUTION: from a foreign CWD with DOCKET_SCRIPTS_DIR set, the resolved form locates a helper.
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
got="$(cd "$tmp" && DOCKET_SCRIPTS_DIR="$REPO/scripts" bash -c 'echo "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh')"
[ -x "$got" ] && ok "DOCKET_SCRIPTS_DIR resolves docket-config.sh from a foreign CWD" \
              || no "DOCKET_SCRIPTS_DIR resolves docket-config.sh from a foreign CWD ($got)"

# (3) FAIL-LOUD: unset DOCKET_SCRIPTS_DIR -> the :? form exits non-zero with the remedy.
err="$(cd "$tmp" && env -u DOCKET_SCRIPTS_DIR bash -c 'echo "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh' 2>&1)"; rc=$?  # env -u: exercise the unset path even when the dev shell exports the var (LEARNINGS #34)
[ "$rc" -ne 0 ] && ok "unset DOCKET_SCRIPTS_DIR fails loud (non-zero exit)" \
               || no "unset DOCKET_SCRIPTS_DIR fails loud (non-zero exit)"
printf '%s' "$err" | grep -qF "run docket/install.sh" && ok "fail-loud message carries the remedy" \
                                                       || no "fail-loud message carries the remedy"

# (3b) FAIL-LOUD at the real Step-0 eval shape: the :? fires inside the command
# substitution, so the outer eval does NOT exit non-zero — but the remedy still
# reaches stderr, which is what the executing agent sees and stops on.
evalerr="$(cd "$tmp" && env -u DOCKET_SCRIPTS_DIR bash -c 'eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"' 2>&1)"  # env -u: exercise the unset path even when the dev shell exports the var (LEARNINGS #34)
printf '%s' "$evalerr" | grep -qF "run docket/install.sh" && ok "eval-site: unset var surfaces the remedy on stderr" \
                                                           || no "eval-site: unset var surfaces the remedy on stderr"

exit $fail
