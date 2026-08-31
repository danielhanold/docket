#!/usr/bin/env bash
# tests/test_go_consumer_migration_guard.sh — change 0369: the migrated Class A/D consumer
# surface stays on the typed Go CLI.
#
# STAGE-LOCAL BY DESIGN: scans ONLY the files 0369 migrated. It asserts nothing about
# preflight, board-only, render-change-links, terminal-publish, mint-stub, runner-dispatch,
# stack-*, adr-checks, render-learnings-index, backfill-change-types, mark-publish-deferred,
# README.md's frozen digest example, scripts/, docs/, or internal/assets/embedded/ — those are
# 0370/0371/0372's (or have no Go verb) and MUST keep passing here untouched. The final
# no-callers seal is explicitly NOT this guard's claim: it never asserts a repo-wide zero
# `docket.sh` count.
#
# HISTORICAL (0369): two frozen callers once kept a legacy op INSIDE a migrated file, and this
# guard sliced the migrated region with named terminators to ban the token only within the migrated
# slice. Change 0377 RETIRED both slice exemptions in one commit — the legacy op is now gone from
# BOTH files entirely, so each is scanned whole-file with a plain ban plus its new Go-verb floor:
#   - terminal-close-out.md: the `killed`-outcome leg migrated `docket.sh archive-change` ->
#     `docket change kill` (Task 10), which archives killed changes atomically. Whole-file ban.
#   - docket-adr/SKILL.md: the repair-only `docket.sh render-adr-index` migrated ->
#     `docket repository migrate` repair (Task 9). Whole-file ban.
#
# SHAPE, NOT SPELLINGS: the legacy-invocation discriminator is `docket.sh <op-token>` with both
# sides of the op token bounded — any path or variable prefix reaching `docket.sh` matches, so a
# `${DOCKET_SCRIPTS_DIR…}/docket.sh` house idiom cannot slip past. The op-token list is the 0369
# migration map (scope data — the migrated-surface definition), NOT a spelling enumeration.
#
# POSITIVE FLOOR per migrated file (marker-scoped-guard-needs-a-population-floor): each file must
# carry its new Go invocation, so deleting a file or rewriting it away from the Go verb reddens.
# Absence asserts alone go vacuously green on an empty file.
#
# RESIDUAL (named, per byte-pattern-guard-matches-a-spelling): a prose paraphrase that re-teaches
# a legacy op WITHOUT the literal `docket.sh` token ("run the archive helper", "regenerate the ADR
# index") survives this byte guard; whole-branch review owns that, not this net.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# The legacy-invocation discriminator: `docket.sh <op-token>`, both sides of the token bounded.
# $2 is an ERE alternation of op tokens. The trailing class excludes [alnum_-] so a longer op
# (bootstrap-foo) is not caught by a shorter token (bootstrap). No -q — read the whole file so a
# producer|early-exiting-consumer SIGPIPE cannot manufacture an intermittent 141 under pipefail.
banned(){ # $1=file  $2=op-token alternation
  grep -E -e "docket\.sh[[:space:]]+($2)([^[:alnum:]_-]|\$)" "$1"
}

# --- Run-gate family: parent instructions on `docket run gate-before` / `gate-verdict` ---------
GATE_SRC="$REPO/cursor-rules/run-gate.md"
AGM="$REPO/AGENTS.md"
for f in "$GATE_SRC" "$AGM"; do
  assert "no legacy gate facade call in ${f##*/}" '! banned "$f" "gate-before|gate-verdict"'
  assert "Go gate pair present in ${f##*/} (floor)" \
    'grep -qF "docket run gate-before implement-next" "$f" && grep -qF "docket run gate-verdict" "$f"'
done

# --- Planning family: `render-artifact-backlink` -> the atomic `docket change groom` transaction --
# change 0377 Task 11: the new-change/grooming skills' spec exit migrated onto `docket change create`
# / `docket change groom`, which stamp the spec's reciprocal `docket:backlink` block ATOMICALLY
# inside the same metadata commit — so the skills no longer make a standalone `docket artifact
# backlink` call. The guarantee relocated, not dropped: ban the legacy facade renderer and floor on
# the `docket change groom` transaction that now owns the backlink stamp.
for s in docket-new-change docket-auto-groom docket-groom-next; do
  f="$REPO/skills/$s/SKILL.md"
  assert "no legacy backlink call in $s" '! banned "$f" "render-artifact-backlink"'
  assert "Go groom transaction present in $s (backlink-owning floor)" 'grep -qF "docket change groom" "$f"'
done

# --- Implementation family: digest -> `docket status --json` -----------------------------------
# The op token is the two-word `docket-status --digest-only` — NOT bare `docket-status`: the
# frozen best-effort reconcile-kill board pass keeps `docket.sh docket-status --board-only`, a
# different op that must stay permitted here (Task 4 frozen boundary). Both sides bounded.
IMPL="$REPO/skills/docket-implement-next/SKILL.md"
assert "no legacy digest call in implement-next" \
  '! grep -E -e "docket\.sh[[:space:]]+docket-status[[:space:]]+--digest-only([^[:alnum:]_-]|$)" "$IMPL"'
assert "Go status read present in implement-next (floor)" 'grep -qF "docket status --json" "$IMPL"'

# --- Convention CREATE_ORPHAN: `bootstrap` -> `docket repository init` -------------------------
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "no legacy bootstrap call in the convention" '! banned "$CONV" "bootstrap"'
assert "Go repository init present in the convention (floor)" 'grep -qF "docket repository init" "$CONV"'

# --- Finalize family: close-out done path on `docket finalize closeout|cleanup` ----------------
TCO="$REPO/skills/docket-convention/references/terminal-close-out.md"
assert "no legacy cleanup call in terminal-close-out" '! banned "$TCO" "cleanup-feature-branch"'
assert "no legacy backlink call in terminal-close-out" '! banned "$TCO" "render-artifact-backlink"'
assert "Go finalize pair present in terminal-close-out (floor)" \
  'grep -qF "docket finalize closeout --id" "$TCO" && grep -qF "docket finalize cleanup --id" "$TCO"'
# archive-change: 0377 Task 10 retired the killed-leg exemption — the done path is on `finalize
# closeout` and the killed leg is on `docket change kill`, so `docket.sh archive-change` is banned
# WHOLE-FILE now (no slice). Floor on the new killed-leg Go verb so deleting it reddens.
assert "no legacy archive-change anywhere in terminal-close-out (killed leg now on change kill)" \
  '! banned "$TCO" "archive-change"'
assert "Go change kill present in terminal-close-out (killed-leg floor)" \
  'grep -qF "docket change kill" "$TCO"'

# --- ADR family (Class D): transactions on Go verbs; index-render follow-up removed ------------
ADR="$REPO/skills/docket-adr/SKILL.md"
assert "docket-adr transactions on Go verbs (floor)" \
  'grep -qF "docket adr record" "$ADR" && grep -qF "docket adr supersede" "$ADR" && grep -qF "docket adr reverse" "$ADR"'
# Class D: 0377 Task 9 retired the repair-only `docket.sh render-adr-index` exemption — the repair
# leg migrated to `docket repository migrate`, so the token is banned WHOLE-FILE now (no slice).
assert "no legacy render-adr-index anywhere in docket-adr (repair leg now on repository migrate)" \
  '! banned "$ADR" "render-adr-index"'

exit $fail
