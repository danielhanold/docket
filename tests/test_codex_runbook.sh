#!/usr/bin/env bash
# tests/test_codex_runbook.sh — guards for the Codex live-validation runbook (change 0078).
# Structure only: these prove the runbook covers every phase, stamps a pass criterion on each,
# cites only real committed paths, and names the COMPLETE generated-agent set derived from the
# glob. They CANNOT prove an expected-outcome claim is TRUE — that is established by Daniel
# executing the runbook in Codex CLI and recording the results doc (LEARNINGS, verify-the-claim
# family: a doc sentinel proves a sentence still EXISTS, never that it is still TRUE).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
RUNBOOK="$REPO/docs/codex/validation-runbook.md"
CONV="$REPO/skills/docket-convention/SKILL.md"
fail=0
ok(){ echo "ok - $1"; }
no(){ echo "NOT OK - $1"; fail=1; }

# --- Assertion 0: the runbook exists ----------------------------------------------------------
if [ -f "$RUNBOOK" ]; then ok "runbook exists"; else no "runbook exists"; exit 1; fi

# --- Assertion 1: all six phases present, each stamped with a pass criterion -------------------
# Count equality, not presence: a phase silently dropped (or added unstamped) must redden.
NPHASES="$(grep -cE '^## Phase [1-6] — ' "$RUNBOOK")"
if [ "$NPHASES" = "6" ]; then ok "runbook covers 6 phases"; else no "runbook covers 6 phases (found $NPHASES)"; fi
# The count alone is identity-blind: renaming `## Phase 6 —` to `## Phase 1 —` keeps it at 6 and
# stays green. Assert each ordinal exists exactly once, so a duplicate/dropped phase reddens.
missing_phase=""
for n in 1 2 3 4 5 6; do
  c="$(grep -cE "^## Phase $n — " "$RUNBOOK")"
  [ "$c" = "1" ] || missing_phase="$missing_phase $n(x$c)"
done
if [ -z "$missing_phase" ]; then ok "each phase 1..6 appears exactly once"; else no "phase identity broken:$missing_phase"; fi
NSTAMPS="$(grep -cF -- '**Pass when:**' "$RUNBOOK")"
if [ "$NSTAMPS" = "$NPHASES" ] && [ "$NPHASES" != "0" ]; then ok "every phase ($NPHASES) carries a pass criterion ($NSTAMPS)"; else no "phase/pass-criterion mismatch: $NPHASES phases, $NSTAMPS criteria"; fi

# --- Assertion 2: the generated-agent set is COMPLETE, derived from the glob -------------------
# The enumerated-floor trap (LEARNINGS): a hand-listed agent set goes stale the moment a 10th
# wrapper lands. Derive from the authoritative glob and require the runbook to name every member.
AGENT_SET="$(cd "$REPO/agents" && ls docket-*.md 2>/dev/null | sed 's/\.md$//' | sort)"
NAGENTS="$(grep -c . <<<"$AGENT_SET")"
if [ "$NAGENTS" -ge 9 ]; then ok "agent set derivable from glob ($NAGENTS agents)"; else no "agent set derivable from glob (found $NAGENTS, expected >= 9)"; fi
missing=""
while IFS= read -r a; do
  [ -z "$a" ] && continue
  # Boundary-matched: bare -F would let `docket-auto-groom-critic` satisfy `docket-auto-groom`.
  grep -qE -- "${a}([^A-Za-z0-9_-]|$)" "$RUNBOOK" || missing="$missing $a"
done <<<"$AGENT_SET"
if [ -z "$missing" ]; then ok "runbook names every generated agent"; else no "runbook missing agents:$missing"; fi

# --- Assertion 3: every committed repo path the runbook cites actually EXISTS ------------------
# This is the guard for the error class that bit this change's own spec: it cited
# `scripts/sync-agents.sh`, which does not exist (the script is repo-root).
#
# SCOPE (explicit, because the tokenizer cannot infer it): this scan checks citations of THIS
# repo's COMMITTED artifacts. The runbook also cites FIXTURE paths — paths inside the disposable
# repo the operator builds while running it — which are shaped identically (`docs/…`) but must
# NOT be resolved against this clone. They are listed in FIXTURE_PATHS below and excluded by
# name. Without that list, `docs/changes/` (cited in Phase 1/6 as a fixture path that does NOT
# exist until the Phase 6 stub lands) would be checked against the docket clone and pass only
# COINCIDENTALLY — this repo happens to have a docs/changes/ — while a future fixture-only
# citation would false-RED. Generated/user paths (`.codex/…`, `~/.codex/…`, `.docket.local.yml`)
# never enter: they lack a scanned prefix. Glob'd tokens carry `*`, excluded by the char class.
FIXTURE_PATHS='docs/changes/'
CITED_RAW="$(grep -oE '`[A-Za-z0-9_./-]+`' "$RUNBOOK" | tr -d '`' | sort -u | grep -E '^(scripts|agents|skills|docs|tests)/' )"
CITED="$(grep -vxF -e "$FIXTURE_PATHS" <<<"$CITED_RAW")"
NCITED="$(grep -c . <<<"$CITED")"
# Prove the tokenizer SEES the corpus before trusting its verdict (LEARNINGS: a scan that parses
# nothing passes everything). This is ONLY a blind-tokenizer tripwire: the identity loop below
# carries the real weight, so the floor needs no margin analysis — it exists so that a scan
# parsing nothing cannot pass everything.
if [ "$NCITED" -ge 4 ]; then ok "path scan found $NCITED cited repo paths"; else no "path scan found only $NCITED cited repo paths (tokenizer likely blind)"; fi
badpaths=""
while IFS= read -r p; do
  [ -z "$p" ] && continue
  [ -e "$REPO/$p" ] || badpaths="$badpaths $p"
done <<<"$CITED"
if [ -z "$badpaths" ]; then ok "every cited repo path exists"; else no "runbook cites nonexistent paths:$badpaths"; fi
# The count check above is identity-blind — it stays green with a required citation deleted
# (proven: dropping either `scripts/runners/codex.sh` or `docs/cursor/permissions.md` alone left
# the suite green). Task 1 Step 3 names these four paths explicitly, so assert each by IDENTITY —
# the same pattern Assertion 4 below uses for root scripts.
for p in docs/codex/setup.md docs/cursor/permissions.md scripts/runners/codex.sh docs/results/; do
  if grep -qF -- "\`$p\`" "$RUNBOOK"; then ok "runbook cites required path: $p"; else no "runbook cites required path: $p"; fi
done

# --- Assertion 4: root-level scripts the runbook drives are named at their REAL location -------
# The repo-root scripts have no `scripts/` prefix; assert each is cited and each exists.
for s in install.sh migrate-to-docket.sh sync-agents.sh link-skills.sh; do
  if [ ! -f "$REPO/$s" ]; then no "root script exists: $s"; continue; fi
  if grep -qF -- "\`$s\`" "$RUNBOOK"; then ok "runbook names root script: $s"; else no "runbook names root script: $s"; fi
done
# ...and never at a fabricated `scripts/` path.
if grep -qF -- 'scripts/sync-agents.sh' "$RUNBOOK"; then no "runbook cites fabricated scripts/sync-agents.sh"; else ok "runbook does not cite fabricated scripts/sync-agents.sh"; fi

# --- Assertion 5: canonical facade spelling, DERIVED from the convention -----------------------
# Phase 2 is the bash-under-sandbox smoke test; it must spell the facade the way the convention
# does. Derive the token rather than retyping it, so the assert cannot drift from the contract.
CANON="$(grep -oE '\$\{DOCKET_SCRIPTS_DIR:\?run docket/install\.sh\}' "$CONV")"
CANON="${CANON%%$'\n'*}"   # first match; no pipe-to-head (pipefail-safe)
if [ -n "$CANON" ]; then ok "canonical facade token derivable from convention"; else no "canonical facade token derivable from convention"; fi
if [ -n "$CANON" ] && grep -qF -- "$CANON" "$RUNBOOK"; then ok "runbook carries canonical facade token"; else no "runbook carries canonical facade token"; fi
# Full decorated canonical form (quotes around the expansion, `/docket.sh` outside them) — built
# from the derived token, not retyped, so a coordinated mangle (e.g. the surrounding quotes
# dropped) cannot pass by satisfying only the inner-token check above. This is the exact bug
# change 0073 found and fixed once already (commit bb4a792) in the sibling Cursor guard; guarding
# only the inner token here left `.../TOTALLY-WRONG.sh` fully green under mutation.
CANON_FULL='"'"$CANON"'"/docket.sh'
if [ -n "$CANON" ] && grep -qF -- "$CANON_FULL" "$RUNBOOK"; then ok "runbook carries full canonical decorated spelling"; else no "runbook carries full canonical decorated spelling"; fi

# --- Assertion 6: the runbook teaches deriving Codex model slugs, not just naming one ----------
# config.yml.example labels its codex IDs "UNVALIDATED examples"; setup.md points at the live
# source. This only checks that `codex debug models` is PRESENT (derivation is taught) — it does
# NOT check that no slug is ALSO hardcoded elsewhere as fact; no such violation exists today.
if grep -qF -- 'codex debug models' "$RUNBOOK"; then ok "runbook derives model slugs via codex debug models"; else no "runbook derives model slugs via codex debug models"; fi

# --- Assertion 6b: the pin is established BEFORE the first Codex-side agent phase --------------
# The one structural claim that would have caught the C2 defect: slug discovery and the
# `agents.codex` pin originally sat in Phase 5, so Phases 3 and 4 observed wrappers carrying
# model IDs Codex cannot run (the built-in Claude IDs, or — if the operator's global config.yml
# carries the shipped block — its UNVALIDATED example slugs, silently and with no warning). A
# refusal observed there is indistinguishable from "Codex does not honor the AGENTS.md dispatch
# block", which would settle ADR-0036 on a FALSE finding. Phase 3 is the first phase that
# observes an agent, so both anchors must appear ABOVE its heading.
# Anchors are resolved independently and their absence REDDENS (never silently skips) — an
# ordering assert whose anchors vanished would otherwise pass vacuously.
first_line_of(){ local out; out="$(grep -nE -- "$1" "$RUNBOOK" | cut -d: -f1)"; printf '%s' "${out%%$'\n'*}"; }
P3_LINE="$(first_line_of '^## Phase 3 — ')"
SLUG_LINE="$(first_line_of 'codex debug models')"
PIN_LINE="$(first_line_of '^[[:space:]]*codex:[[:space:]]*$')"
if [ -z "$P3_LINE" ] || [ -z "$SLUG_LINE" ] || [ -z "$PIN_LINE" ]; then
  no "pin-ordering anchors all present (phase3='$P3_LINE' slug='$SLUG_LINE' pin='$PIN_LINE')"
else
  ok "pin-ordering anchors all present (phase3=$P3_LINE slug=$SLUG_LINE pin=$PIN_LINE)"
  if [ "$SLUG_LINE" -lt "$P3_LINE" ] && [ "$PIN_LINE" -lt "$P3_LINE" ]; then
    ok "model pin established before the first Codex-side agent phase (Phase 3)"
  else
    no "model pin lands at/after ## Phase 3 (slug=$SLUG_LINE pin=$PIN_LINE phase3=$P3_LINE) — Phases 3/4 would observe wrappers Codex cannot spawn"
  fi
fi
# The pin must cover the WHOLE agents.codex block: Phase 3 lists all nine agents, and a
# status-only pin leaves eight unrunnable. Derive the required keys from the same authoritative
# glob Assertion 2 uses (agent name minus the `docket-` prefix), and require each as a YAML key.
unpinned=""
while IFS= read -r a; do
  [ -z "$a" ] && continue
  k="${a#docket-}"
  grep -qE -- "^[[:space:]]+${k}:[[:space:]]*\{" "$RUNBOOK" || unpinned="$unpinned $k"
done <<<"$AGENT_SET"
if [ -z "$unpinned" ]; then ok "runbook pins every codex agent key"; else no "runbook leaves codex agent keys unpinned:$unpinned"; fi

# --- Assertion 7: the native/runner-delegation boundary is stated ------------------------------
# 0079's runner delegation is the opposite direction and out of scope; conflating them is the
# single likeliest misreading of this runbook.
if grep -qF -- 'runner-dispatch' "$RUNBOOK"; then ok "runbook states the runner-dispatch boundary"; else no "runbook states the runner-dispatch boundary"; fi

# --- Assertion 8: the runbook is discoverable -------------------------------------------------
SETUP="$REPO/docs/codex/setup.md"
README="$REPO/README.md"
# setup.md is the runbook's sibling; link it relatively from the section it deepens.
if grep -qF -- '](validation-runbook.md)' "$SETUP"; then ok "setup.md links the runbook"; else no "setup.md links the runbook"; fi
if grep -qF -- '](docs/codex/validation-runbook.md)' "$README"; then ok "README links the runbook"; else no "README links the runbook"; fi

exit $fail
