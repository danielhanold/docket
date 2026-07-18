#!/usr/bin/env bash
# tests/test_finalize_disposition.sh — guards change 0087 (headless finalize: the finalize-side
# disposition contract, mirroring 0088). Asserts the four-disposition terminal contract, id-set
# scoping, the mergeability ordering keys IN ORDER, the `## Finalize blocked` marker semantics,
# and the README drain-pattern doc.
# Sentinels are sampling, not parsing (learnings: foundational-test-discipline) — pair with the
# whole-branch review; this test does not replace it.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if ( eval "$2" ); then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

FIN="$REPO/skills/docket-finalize-change/SKILL.md"

# --- SKILL.md: the four-disposition terminal contract ---
assert "SKILL has a Terminal disposition section" 'grep -Eqi "Terminal disposition" "$FIN"'
for d in advanced contended drained halted; do
  tok="\`$d\`"
  assert "SKILL names disposition $d (code-formatted)" 'grep -qF "$tok" "$FIN"'
done
# The binary driver rule — both halves must be present (non-vacuous).
assert "SKILL states continue-on advanced/contended" 'grep -Eqi "continue on .{0,4}advanced" "$FIN"'
assert "SKILL states stop-on drained/halted" 'grep -Eqi "stop on .{0,4}drained" "$FIN"'
assert "SKILL enumerates skipped-with-reason" 'grep -Eqi "skipped with (its|the) reason" "$FIN"'

# --- SKILL.md: the finalize-specific disposition semantics ---
assert "SKILL ties every abort-and-report point to halted" \
  'grep -Eqi "abort-and-report point.{0,40}(is|are|maps to|→).{0,20}\`?halted" "$FIN"'
assert "SKILL states a blocked-but-non-empty set is halted, not drained" \
  'grep -Eqi "halted.{0,30}(never|not).{0,10}\`?drained" "$FIN"'
assert "SKILL states one merge per invocation" \
  'grep -Eqi "run merges.{0,20}exactly one.{0,20}change" "$FIN"'
assert "SKILL states it never batches" 'grep -Eqi "never batch" "$FIN"'
# The multi-candidate prompt is an interactive-BATCH guard; one-merge-per-invocation supersedes it,
# so the unscoped drain selects by Ordering instead of halting on an impossible prompt.
assert "SKILL states the multi-candidate prompt is superseded on the driver path" \
  'grep -Eqi "multi-candidate prompt is an interactive.{0,10}\*?batch\*? guard.{0,60}supersed" "$FIN"'
assert "SKILL states a driver/autonomous run never prompts and takes the Ordering head" \
  'grep -Eqi "(driver|autonomous) run selects by .{0,15}Ordering.{0,15} and never prompts" "$FIN"'
assert "the selection matrix scopes the multi-candidate prompt to attended runs" \
  'grep -Eqi "More than one eligible.{0,120}NO prompt.{0,140}Attended run" "$FIN"'
# Already-merged close-out is `advanced`, not `drained` — real work ran, so the driver must continue.
assert "SKILL maps an already-merged close-out to advanced" \
  'grep -Eqi "archived an already-merged PR" "$FIN"'
assert "SKILL exempts already-merged archiving from one-merge-per-invocation" \
  'grep -Eqi "already-merged.{0,40}changes in one run does not violate" "$FIN"'
# `contended` must not swallow a raced success this run actually merged.
assert "SKILL qualifies contended against a raced success" \
  'grep -Eqi "if .{0,5}this.{0,5} run performed the merge, it is .\`?advanced" "$FIN"'

# --- SKILL.md: id-set scoping ---
assert "SKILL documents an id allowlist" 'grep -Eqi "allowlist" "$FIN"'
assert "SKILL shows the comma-separated id-set form" 'grep -Eq "docket-finalize-change 90,92,94" "$FIN"'
assert "SKILL states naming the ids IS the authorization" \
  'grep -Eqi "naming the ids.{0,30}authorization" "$FIN"'
assert "SKILL ties the allowlist to the require_pr_approval override" \
  'grep -Eqi "allowlist never prompts.{0,60}require_pr_approval" "$FIN"'

# --- SKILL.md: mergeability ordering, asserted IN ORDER (order is part of the contract) ---
# NOTE: never `grep … | head` under `set -o pipefail` (AGENTS.md) — the producer takes SIGPIPE and
# the 141 becomes an intermittent failure. Capture the whole match set, then take the first line
# with parameter expansion.
first_line_no(){ # first_line_no ERE -> line number of the first matching line, empty if none
  local m; m="$(grep -nEi -e "$1" "$FIN" || true)"
  [ -n "$m" ] || return 0
  m="${m%%$'\n'*}"        # first match only
  printf '%s' "${m%%:*}"  # strip everything from the first colon
}
p_dep="$(first_line_no '^[[:space:]]*1\..*depends_on')"
p_mrg="$(first_line_no '^[[:space:]]*2\..*mergeable')"
p_dif="$(first_line_no '^[[:space:]]*3\..*(smallest diff|changedFiles)')"
p_tie="$(first_line_no '^[[:space:]]*4\..*priority')"
assert "ordering key 1 is depends_on" '[ -n "$p_dep" ]'
assert "ordering key 2 is mergeable" '[ -n "$p_mrg" ]'
assert "ordering key 3 is diff size" '[ -n "$p_dif" ]'
assert "ordering key 4 is the priority tiebreak" '[ -n "$p_tie" ]'
assert "the four ordering keys appear in contract order" \
  '[ -n "$p_dep" ] && [ -n "$p_mrg" ] && [ -n "$p_dif" ] && [ -n "$p_tie" ] &&
   [ "$p_dep" -lt "$p_mrg" ] && [ "$p_mrg" -lt "$p_dif" ] && [ "$p_dif" -lt "$p_tie" ]'
# CONFLICTING DEPRIORITIZES, it is not excluded — line 54's delegation to the rebase-retest gate
# is preserved, so `docket-rebase-resolver` still owns resolution. A bare grep for "CONFLICTING"
# would be decorative (the word predates this change), so anchor on the deprioritize/never-exclude
# shape and on the delegation surviving.
assert "SKILL deprioritizes CONFLICTING rather than excluding it" \
  'grep -Eqi "CONFLICTING.{0,40}(deprioritize|sorts? last).{0,40}never excludes?|CONFLICTING[^.]{0,60}never excludes?" "$FIN"'
assert "SKILL keeps conflict resolution delegated to the rebase-resolver" \
  'grep -Eqi "(resolution|resolving).{0,60}delegated.{0,60}docket-rebase-resolver|delegated to the gate.{0,20}.?s .docket-rebase-resolver" "$FIN"'
assert "SKILL marks Finalize blocked only for a conflict the GATE can not act on" \
  'grep -Eqi "only a conflict the .{0,10}gate.{0,10}(can.t|cannot|can not) act on" "$FIN"'
assert "SKILL documents the lazy-mergeable poll" \
  'grep -q "UNKNOWN" "$FIN" && grep -Eqi "poll" "$FIN"'
assert "SKILL forbids pairwise file-overlap ranking" \
  'grep -Eqi "(not|never|do not|don.t) build pairwise|pairwise file-overlap" "$FIN"'

# --- SKILL.md: the `## Finalize blocked` marker (D4) ---
# NOTE: the three assertions below are anchored tighter than a bare substring grep for
# "## Finalize blocked" / "CONFLICTING…mark" / "metadata write" — Task 1 already left forward
# references containing those exact substrings (the ordering block's "*Finalize blocked* below",
# the skipped-with-reason list's "already carrying `## Finalize blocked`", and the durable-root
# paragraph's "the metadata writes"), so a bare version of each would pass before Task 2 adds
# anything. Anchoring on the actual heading / bullet phrasing keeps them non-vacuous.
assert "SKILL has the Finalize blocked marker subsection heading" \
  'grep -qF "### \`## Finalize blocked\` — marking a change that needs a human" "$FIN"'
assert "SKILL states it is NOT a new status" \
  'grep -Eqi "not (a new|an eighth) status|never an eighth status" "$FIN"'
assert "SKILL states it is not a reuse of blocked" \
  'grep -Eqi "(not|never) a reuse of .{0,3}\`?blocked" "$FIN"'
assert "SKILL states selection SKIPS a marked change" \
  'grep -Eqi "skip.{0,40}(carrying|marked|section)" "$FIN"'
# The skip must be scoped to auto-detect and overridable by a named id, or the marker deadlocks:
# a permanently-skipped change can never be finalized, so the clearing rule below can never fire.
assert "SKILL scopes the marker skip to the auto-detect path" \
  'grep -Eqi "auto-detect selection skips" "$FIN"'
assert "SKILL states a named id or allowlist member OVERRIDES the marker skip" \
  'grep -Eqi "(explicitly named id|named id).{0,60}overrides the skip|overrides the skip.{0,60}named id" "$FIN"'
assert "the skipped-with-reason list scopes the marker skip to auto-detect" \
  'grep -Eqi "already carrying .\`?## Finalize blocked.{0,80}(auto-detect|named id)" "$FIN"'
assert "SKILL states a CONFLICTING PR is NOT marked at selection time" \
  'grep -Eqi "CONFLICTING.{0,10}PR is .{0,4}NOT marked at selection time" "$FIN"'
assert "SKILL states a successful finalize CLEARS the section" \
  'grep -Eqi "(remove|clear)s?.{0,40}section|section.{0,40}(removed|cleared)" "$FIN"'
assert "SKILL names the board cell wording" 'grep -qF "finalize blocked — needs you" "$FIN"'
assert "SKILL says the marker is a metadata write" \
  'grep -qF "**metadata write**" "$FIN"'

CONV="$REPO/skills/docket-convention/SKILL.md"
assert "convention lists the Finalize blocked body section" 'grep -qF "## Finalize blocked" "$CONV"'
# The convention entry must not foreclose a human retry (it used to say "not a human re-arm",
# which combined with an unconditional skip made a marked change permanently unfinalizable).
assert "convention scopes the marker skip to auto-detect runs" \
  'grep -Eqi "later .{0,4}\*{0,2}auto-detect\*{0,2}.{0,4} finalize runs skip" "$CONV"'
assert "convention says naming the id is how a human retries a marked change" \
  'grep -Eqi "retries a marked change by .{0,4}\*{0,2}naming its id" "$CONV"'

# --- README: the /loop finalize drain-pattern doc ---
README="$REPO/README.md"
fb='`/loop docket-finalize-change`'
assert "README documents the /loop finalize drain" 'grep -qF "$fb" "$README"'
assert "README documents the /loop finalize id-set drain" \
  'grep -Eq "/loop docket-finalize-change 90,92,94" "$README"'
# Retargeted (learnings: sentinel-passed-on-pre-existing-text): grepping the WHOLE README for the
# four words passes on the base revision — the implement-side /loop section already contains all
# four. Anchor to THIS section's own lead-in line, the way the neighbour assert below does, so
# deleting a disposition from the finalize paragraph reddens it.
# Anchoring on the LINE is still too loose: its trailing "continue on `advanced`/`contended`, stop
# on `drained`/`halted`" clause supplies all four tokens on its own, so deleting them from the
# ENUMERATION stayed green. Cut the line at that clause and assert over the enumeration alone.
fin_lead="$(grep -F 'same four dispositions' "$README" || true)"
fin_enum="${fin_lead%%so a single driver*}"
assert "README has the finalize four-disposition lead-in" '[ -n "$fin_lead" ]'
assert "the finalize enumeration is separable from the binary-rule clause" \
  '[ -n "$fin_enum" ] && [ "$fin_enum" != "$fin_lead" ]'
for d in advanced contended drained halted; do
  tok="\`$d\`"
  assert "README finalize enumeration names $d (code-formatted)" 'grep -qF "$tok" <<<"$fin_enum"'
done
# Retargeted (learnings: sentinel-passed-on-pre-existing-text): the bare continue/stop phrasing is
# byte-identical to the implement-side section by design (same four-disposition contract), so an
# unanchored grep for it is decorative here — it already passes on the pre-Task-4 README. Anchor to
# this section's own unique lead-in so the assertion actually depends on this prose landing.
assert "README states the binary continue/stop rule (finalize)" \
  'grep -Eqi "keys on both halves of the loop.{0,150}continue on .{0,4}advanced.{0,80}stop on .{0,4}drained" "$README"'
assert "README states naming the ids is the authorization" \
  'grep -Eqi "naming the ids.{0,40}authorization" "$README"'
assert "README names the finalize-blocked board cell" \
  'grep -qF "finalize blocked — needs you" "$README"'
# The implement-side driver never merges; THIS one does. The distinction must be explicit, or a
# reader carries the wrong mental model across the two subsections.
assert "README states the finalize driver DOES merge" \
  'grep -Eqi "this driver (does|merges)|unlike the implementer" "$README"'
# The two /loop sections must reconcile: the implement-side "never merges" guarantee stands, but a
# dependency can now also clear via a finalize drain, so that clause points at this section.
assert "README's implement-side never-merges clause points at the finalize drain" \
  'grep -Eqi "never merges\*\*[^|]{0,200}finalize drain.{0,80}#closing-out-hands-free-with-loop" "$README"'

# The unattended merge depends on a branch-protection setting documented ~430 lines below; a drain
# subsection that omits the prerequisite reads as "this just works" and halts on the first merge.
assert "README's drain subsection cross-links the branch-protection prerequisite" \
  'grep -Eqi "prerequisite.{0,200}hands-off-finalize" "$README"'

# --- The marker WRITE must be reachable from the procedure, not only from its own definition. ---
# Without this the whole marker/skip/clear apparatus is inert: every other marker assertion below
# passes on the *definition* alone, so nothing else catches "no code path ever writes it".
assert "SKILL wires the marker write into the abort-and-report surfacing step" \
  'grep -Eqi "where the reason surfaces.{0,600}appends the .{0,4}## Finalize blocked" "$FIN"'
# A retry that fails again must not accrete a second heading — the marker is state, not a log.
assert "SKILL states a re-mark REPLACES the section rather than appending a second heading" \
  'grep -Eqi "re-mark.{0,60}replaces.{0,120}never appends a second heading" "$FIN"'
# The transition-out gap: a human-merged PR carrying a stale marker must still be archived.
assert "SKILL states an already-merged PR is archived regardless of the marker" \
  'grep -Eqi "already-merged PR is archived regardless" "$FIN"'
# The skip is scoped to UNMERGED changes; an unscoped "skips any change carrying it" strands them.
assert "SKILL scopes the auto-detect marker skip to unmerged changes" \
  'grep -Eqi "selection skips\*\* any \*\*unmerged\*\*" "$FIN"'
# The drained/halted boundary must be decidable, not inferred — same backlog, same disposition.
assert "SKILL resolves the drained boundary: in-scope-but-human-requiring counts as halted" \
  'grep -Eqi "counts toward the non-empty set and yields .{0,4}halted" "$FIN"'
assert "SKILL states drained requires nothing in scope at all" \
  'grep -Eqi "drained.{0,40}requires that no .{0,4}implemented.{0,4} change was in scope" "$FIN"'
# A classifier/harness denial of the merge is on the critical path and mapped nowhere otherwise.
assert "SKILL maps a harness/classifier merge denial into the abort-and-report set" \
  'grep -Eqi "classifier denying the merge" "$FIN"'

# --- Non-vacuity / mutation proof: the code-formatted disposition grep actually bites. ---
probe="$(mktemp)"; printf 'plain advanced word, no code formatting\n' > "$probe"
assert "the code-formatted disposition grep is non-vacuous" '! grep -qF "\`advanced\`" "$probe"'
# Non-vacuity for the ordering comparison: a reversed pair must fail the same test.
assert "the ordering comparison is non-vacuous (9 < 3 is caught)" '! [ 9 -lt 3 ]'
rm -f "$probe"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
