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
assert(){ if ( eval "$2" ); then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# A literal backtick, held in a SINGLE-quoted literal, for the code-span tokens built below beside a
# `$d` expansion. No backtick may sit inside double quotes in test source: bare, the shell runs it
# when it reads the line; backslash-escaped, the escape is consumed there and a bare backtick
# travels on to the next evaluation (change 0221, scripts/check-test-source-hygiene.sh).
BT='`'

FIN="$REPO/skills/docket-finalize-change/SKILL.md"
# Change 0201 moved the marker's write shape + lifecycle mechanics (re-mark, CONFLICTING-at-
# selection, clearing, the abort surfacing channels) behind a blocking pointer in
# references/gate-failure.md; the asserts guarding those mechanics read GF, while everything
# selection consults on the hot path (skip / override / already-merged / board cell) stays on FIN.
GF="$REPO/skills/docket-finalize-change/references/gate-failure.md"
assert "gate-failure reference exists" '[ -f "$GF" ]'
# gate-failure.md is a hard-wrapped reference doc (unlike the sequencer SKILL's unwrapped
# paragraphs), so a phrase-spanning `grep` over it silently doubles as a line-wrap guard: a clause
# that lands a wrap between two words reddens on a pure re-flow. Several 0316 marker/abort asserts
# below key on phrases the rewrite wrapped, so they read this FLATTENED copy. `-s` (squeeze) is
# load-bearing — a wrapped bullet indents its continuation, so a bare newline-to-space swap leaves
# words several spaces apart and a single-space pattern misses.
GF_FLAT="$(tr '\n' ' ' < "$GF" | tr -s '[:space:]' ' ')"
assert "gate-failure flattened haystack is non-vacuous (>= 2000 chars)" '[ "${#GF_FLAT}" -ge 2000 ]'
# RE-KEYED (0316, category (c)): the Go-sequencer rewrite states the same blocking pointer as
# "…live in `references/gate-failure.md` — read it at any abort" rather than the old
# "read `references/gate-failure.md` now (blocking)". Behavior preserved (GF is still the blocking
# reference for the failure flows); locator keyed on shape.
assert "SKILL points at the gate-failure reference (blocking read)" \
  'grep -Eqi "references/gate-failure\.md.{0,30}read it at any abort" "$FIN"'

# --- SKILL.md: the four-disposition terminal contract ---
assert "SKILL has a Terminal disposition section" 'grep -Eqi "Terminal disposition" "$FIN"'
for d in advanced contended drained halted; do
  tok="$BT$d$BT"
  assert "SKILL names disposition $d (code-formatted)" 'grep -qF "$tok" "$FIN"'
done
# The binary driver rule — both halves must be present (non-vacuous).
assert "SKILL states continue-on advanced/contended" 'grep -Eqi "continue on .{0,4}advanced" "$FIN"'
assert "SKILL states stop-on drained/halted" 'grep -Eqi "stop on .{0,4}drained" "$FIN"'
# RE-KEYED (0316, category (c)): "each change skipped with its closed reason" (was "…with its
# reason"). The final-report enumeration is preserved; the locator now tolerates the qualifier.
assert "SKILL enumerates skipped-with-reason" 'grep -Eqi "skipped with (its|the) [a-z]* ?reason" "$FIN"'

# --- SKILL.md: the finalize-specific disposition semantics ---
assert "SKILL ties every abort-and-report point to halted" \
  'grep -Eqi "abort-and-report point.{0,40}(is|are|maps to|→).{0,20}\`?halted" "$FIN"'
assert "SKILL states a blocked-but-non-empty set is halted, not drained" \
  'grep -Eqi "halted.{0,30}(never|not).{0,10}\`?drained" "$FIN"'
assert "SKILL states one merge per invocation" \
  'grep -Eqi "run merges.{0,20}exactly one.{0,20}change" "$FIN"'
assert "SKILL states it never batches" 'grep -Eqi "never batch" "$FIN"'
# --- RETIRED (0316, category (a)): the interactive multi-candidate PROMPT and the selection MATRIX
# were the old Bash procedure's attended batch-disambiguation UI. Selection is now owned by
# `internal/app/finalize_context.go` — `SelectFinalizeQueue` returns candidates already ordered and
# the sequencer TAKES THE HEAD; there is no interactive prompt to supersede and no matrix to scope.
# Authority #2: the Go symbol `SelectFinalizeQueue`/`FinalizePolicy` owns selection eligibility,
# ordering, and skip reasons. The guard is re-pointed at the surviving substance rather than
# deleted: the skill names the Go ordering owner and takes its ordered head deterministically.
assert "SKILL selection is owned by SelectFinalizeQueue and takes the ordered head" \
  'grep -qF -- "SelectFinalizeQueue" "$FIN" && grep -Eqi "[Tt]ake the head" "$FIN"'
# Already-merged close-out is `advanced`, not `drained` — real work ran, so the driver must continue.
assert "SKILL maps an already-merged close-out to advanced" \
  'grep -Eqi "archived an already-merged PR" "$FIN"'
# RE-KEYED (0316, category (c) — the plan's canonical example): the rewrite inserted a parenthetical
# ("(each a merged-recovery candidate with no merge to perform)") between "already-merged" and "does
# not violate", overrunning the old `.{0,40}` window. Nothing changed but the spacing — key on the
# shape, not the character distance.
assert "SKILL exempts already-merged archiving from one-merge-per-invocation" \
  'grep -Eqi "already-merged.{0,120}does not violate this rule" "$FIN"'
# `contended` must not swallow a raced success this run actually merged.
assert "SKILL qualifies contended against a raced success" \
  'grep -Eqi "if .{0,5}this.{0,5} run performed the merge, it is .\`?advanced" "$FIN"'

# --- SKILL.md: id-set scoping ---
assert "SKILL documents an id allowlist" 'grep -Eqi "allowlist" "$FIN"'
# RE-KEYED (0316, category (c)): the old concrete invocation `docket-finalize-change 90,92,94`
# became the `--allowlist <ids>` flag form on `docket context finalize`; the id-set capability is
# preserved (the concrete comma-separated example still lives in the README, asserted below).
assert "SKILL shows the id-set (allowlist) form" 'grep -qF -- "--allowlist <ids>" "$FIN"'
assert "SKILL states naming the ids IS the authorization" \
  'grep -Eqi "naming the ids.{0,30}authorization" "$FIN"'
# RE-KEYED (0316, category (c)/(a)): the `require_pr_approval` override is now owned by
# `internal/app/finalize_merge.go` (`ApprovalSatisfied: in.explicitID || !in.requireApproval`). The
# skill expresses the same tie as the allowlist authorization OVERRIDING the `approval-required`
# skip reason — preserved substance, keyed on the sequencer's vocabulary.
assert "SKILL ties the allowlist authorization to the approval override" \
  'grep -Eqi "naming the ids is the same authorization" "$FIN" && grep -Eqi "overrides the .approval-required" "$FIN"'

# --- SKILL.md: mergeability ordering (now the Go owner's contract) ---
# RETIRED (0316, category (a)): the mergeability ORDERING was a hand-numbered 1./2./3./4. list whose
# contract order this block verified by LINE NUMBER; the lazy-mergeable POLL and the
# no-pairwise-file-overlap prohibition were the Bash procedure's own selection logic. All three are
# now owned by `internal/app/finalize_context.go` — `SelectFinalizeQueue` orders the queue and
# `FinalizePolicy`/`FinalizeCandidateReport.Band` classify mergeability; the skill states the order
# as the Go owner's contract, not a re-derivable numbered list, so the `first_line_no` order
# machinery is retired with it. Authority #2: SelectFinalizeQueue owns ordering and the mergeability
# band. The guard is re-pointed at the ordering contract the skill still states, keyed on shape
# (the sequence within the SelectFinalizeQueue sentence), and on CONFLICTING being DEPRIORITIZED
# (ordered after MERGEABLE) rather than excluded.
sfq_line="$(grep -F "SelectFinalizeQueue" "$FIN" || true)"
assert "SKILL names the Go ordering owner (SelectFinalizeQueue)" '[ -n "$sfq_line" ]'
assert "SKILL states the ordering contract in the Go owner's terms (depends-eligible, mergeable, diff, priority)" \
  'grep -Eqi "dependency-eligible.{0,80}MERGEABLE. before .CONFLICTING.{0,120}changed-files and diff.{0,40}priority" <<<"$sfq_line"'
assert "SKILL deprioritizes CONFLICTING (ordered after MERGEABLE) rather than excluding it" \
  'grep -Eqi "MERGEABLE. before .CONFLICTING" <<<"$sfq_line"'
# RE-KEYED (0316, category (c)): conflict resolution is still delegated to docket-rebase-resolver —
# step 3's resolver loop dispatches it on a `conflicted` rebase. Preserved behavior, keyed on the
# sequencer's own dispatch sentence.
assert "SKILL keeps conflict resolution delegated to the rebase-resolver" \
  'grep -Eqi "conflicted.{0,40}dispatch .docket-rebase-resolver" "$FIN"'
# RE-KEYED (0316): the "mark only when the gate cannot act on the conflict" rule moved with the rest
# of the marker lifecycle into gate-failure.md, phrased as "Marking happens only at an abort-and-
# report point" (the resolver resolves an ordinary CONFLICTING PR, so it is not marked up front).
assert "gate-failure marks Finalize blocked only at an abort-and-report point (not an ordinary conflict)" \
  'grep -Eqi "[Mm]arking happens only at an abort-and-report point" "$GF"'

# --- the `## Finalize blocked` marker (D4) — write shape + lifecycle now in gate-failure.md ---
# RE-KEYED (0316, category (c)): the marker's WRITE SHAPE and LIFECYCLE moved out of the SKILL body
# into references/gate-failure.md's "## The `## Finalize blocked` marker — write shape and lifecycle"
# section (the Go-sequencer SKILL points at GF as a blocking read at every abort, asserted above).
# The behavior is preserved verbatim there — heading, "not a new status / not a reuse of blocked",
# the auto-detect skip scoped to unmerged changes already carrying the section, the named-id
# override, and the metadata-write shape — only these locators move from FIN to GF. The board-cell
# wording moved to the convention (and to `internal/render/board.go`, which now renders the board —
# the skill no longer does), so that assert reads the convention.
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "gate-failure has the Finalize blocked marker subsection heading" \
  'grep -qF "## The \`## Finalize blocked\` marker — write shape and lifecycle" "$GF"'
assert "gate-failure states it is NOT a new status" \
  'grep -Eqi "not.{0,5}a new lifecycle status|not.{0,5}an eighth status" "$GF"'
assert "gate-failure states it is not a reuse of blocked" \
  'grep -Eqi "(not|never).{0,5}a reuse of .{0,3}\`?blocked" "$GF"'
assert "gate-failure states selection SKIPS a marked change" \
  'grep -Eqi "selection skips.{0,40}(carrying|marked|section)" "$GF"'
# The skip must be scoped to auto-detect and overridable by a named id, or the marker deadlocks:
# a permanently-skipped change can never be finalized, so the clearing rule below can never fire.
assert "gate-failure scopes the marker skip to the auto-detect path" \
  'grep -Eqi "[Aa]uto-detect selection skips" "$GF"'
assert "gate-failure states a named id or allowlist member OVERRIDES the marker skip" \
  'grep -Eqi "named id or allowlist member overrides.{0,20}skip|overrides the skip.{0,60}named id" <<<"$GF_FLAT"'
assert "the marker skip is scoped to auto-detect over changes already carrying the section" \
  'grep -Eqi "auto-detect selection skips.{0,4}any unmerged change already carrying the section" "$GF"'
assert "gate-failure states a CONFLICTING PR is NOT marked at selection time" \
  'grep -Eqi "CONFLICTING.{0,10}PR is .{0,4}NOT marked at selection time" "$GF"'
assert "gate-failure states a successful finalize CLEARS the section" \
  'grep -Eqi "(remove|clear)s?.{0,40}section|section.{0,40}(removed|cleared)" "$GF"'
assert "convention names the board cell wording (board render owned by Go)" \
  'grep -qF "finalize blocked — needs you" "$CONV"'
assert "gate-failure says the marker is a metadata write" \
  'grep -Eqi "metadata write" "$GF"'

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
  tok="$BT$d$BT"
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
# RE-KEYED (0316, category (c)): the surfacing step is preserved — `docket finalize block` records
# the reason durably, first as the owned PR comment, then as the `## Finalize blocked` marker — but
# reworded (no longer "appends the …") and hard-wrapped, so it reads the flattened GF.
assert "gate-failure wires the marker write into the abort-and-report surfacing step" \
  'grep -Eqi "[Ww]here the reason surfaces.{0,220}docket finalize block. records it durably.{0,220}## Finalize blocked. marker" <<<"$GF_FLAT"'
# A retry that fails again must not accrete a second heading — the marker is state, not a log.
# RE-KEYED (0316, category (c)): GF says "never a second heading" (was "never appends a second
# heading"); preserved behavior, keyed on shape over the flattened GF.
assert "gate-failure states a re-mark REPLACES the section rather than appending a second heading" \
  'grep -Eqi "re-mark.{0,60}replaces.{0,140}never a second heading" <<<"$GF_FLAT"'
# The transition-out gap: a human-merged PR carrying a stale marker must still be recovered.
# RE-KEYED (0316, category (c)): the rule moved into GF and is phrased "an already-merged PR is a
# merged-recovery candidate regardless of the marker" (was "archived regardless"); flattened GF.
assert "gate-failure states an already-merged PR is a recovery candidate regardless of the marker" \
  'grep -Eqi "already-merged PR is a merged-recovery candidate regardless.{0,10}of the marker" <<<"$GF_FLAT"'
# The skip is scoped to UNMERGED changes; an unscoped "skips any change carrying it" strands them.
# RE-KEYED (0316, category (c)): moved into GF, unbolded ("skips any unmerged change"); flattened GF.
assert "gate-failure scopes the auto-detect marker skip to unmerged changes" \
  'grep -Eqi "selection skips.{0,4}any unmerged change" <<<"$GF_FLAT"'
# The drained/halted boundary must be decidable, not inferred — same backlog, same disposition.
assert "SKILL resolves the drained boundary: in-scope-but-human-requiring counts as halted" \
  'grep -Eqi "counts toward the non-empty set and yields .{0,4}halted" "$FIN"'
# RE-KEYED (0316, category (c)): "drained requires that context finalize surfaced no implemented
# candidate at all" (was "requires that no implemented change was in scope"); preserved boundary.
assert "SKILL states drained requires nothing in scope at all" \
  'grep -Eqi "drained. requires that .{0,25}surfaced no .{0,4}implemented.{0,4}(candidate|change)" "$FIN"'
# RE-KEYED (0316, category (c)/(a)): a merge denial is no longer framed as a harness/classifier
# decision — the Go `docket finalize merge` verb returns `merge-denied`/`denied`. The abort-and-
# report mapping is preserved: an authoritatively denied merge maps to halted. Flattened GF.
assert "gate-failure maps a denied merge (merge-denied) into the abort-and-report set" \
  'grep -Eqi "authoritatively .{0,4}denied.{0,4} merge.{0,120}merge-denied" <<<"$GF_FLAT"'

# --- Non-vacuity / mutation proof: the code-formatted disposition grep actually bites. ---
probe="$(mktemp)"; printf 'plain advanced word, no code formatting\n' > "$probe"
assert "the code-formatted disposition grep is non-vacuous" '! grep -qF "\`advanced\`" "$probe"'
# Non-vacuity for the ordering comparison: a reversed pair must fail the same test.
assert "the ordering comparison is non-vacuous (9 < 3 is caught)" '! [ 9 -lt 3 ]'
rm -f "$probe"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
