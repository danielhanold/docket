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
# Two frozen callers legitimately keep a legacy op INSIDE a migrated file, so this guard slices
# the migrated region with NAMED terminators (asserting each terminator exists, so a rename
# reddens instead of silently widening the slice to EOF) and bans the token only within it:
#   - terminal-close-out.md: the `killed`-outcome leg keeps `docket.sh archive-change` (finalize
#     closeout does not cover killed — Task 1 disposition); only the done-path slice is scanned.
#   - docket-adr/SKILL.md: the Index/validate section keeps a repair-only `docket.sh
#     render-adr-index`; only the transaction sections (Create..Index/validate) are scanned.
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

# --- Planning family: `render-artifact-backlink` -> `docket artifact backlink` -----------------
for s in docket-new-change docket-auto-groom docket-groom-next; do
  f="$REPO/skills/$s/SKILL.md"
  assert "no legacy backlink call in $s" '! banned "$f" "render-artifact-backlink"'
  assert "Go backlink verb present in $s (floor)" 'grep -qF "docket artifact backlink" "$f"'
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
# archive-change: the done path is migrated to `finalize closeout`, but the killed-outcome leg
# legitimately keeps `docket.sh archive-change` (Task 1 disposition). Slice the done-path region
# with named terminators (**Done drivers** .. **Kill drivers**) and ban the token inside it only.
assert "close-out done/kill terminators still exist (slice is bounded)" \
  'grep -qF "**Done drivers**" "$TCO" && grep -qF "**Kill drivers**" "$TCO"'
done_path_span(){ awk '/\*\*Done drivers\*\*/{f=1} /\*\*Kill drivers\*\*/{f=0} f' "$TCO"; }
assert "no legacy archive-change on the done path (killed leg stays frozen)" \
  '! done_path_span | grep -E -e "docket\.sh[[:space:]]+archive-change([^[:alnum:]_-]|$)"'

# --- ADR family (Class D): transactions on Go verbs; index-render follow-up removed ------------
ADR="$REPO/skills/docket-adr/SKILL.md"
assert "docket-adr transactions on Go verbs (floor)" \
  'grep -qF "docket adr record" "$ADR" && grep -qF "docket adr supersede" "$ADR" && grep -qF "docket adr reverse" "$ADR"'
# Class D: no `render-adr-index` follow-up inside the transaction sections. A repair-only mention
# in Index/validate is permitted (frozen — no Go verb). Slice Create..Index/validate with named
# terminators and assert each exists so a heading rename reddens rather than widening the slice.
assert "ADR Create + Index/validate terminators still exist (slice is bounded)" \
  'grep -qE "^### Create" "$ADR" && grep -qE "^### Index / validate" "$ADR"'
adr_txn_span(){ awk '/^### Create/{f=1} /^### Index \/ validate/{f=0} f' "$ADR"; }
assert "no index-render follow-up inside ADR transaction sections (Class D stays removed)" \
  '! adr_txn_span | grep -E -e "docket\.sh[[:space:]]+render-adr-index([^[:alnum:]_-]|$)"'

exit $fail
