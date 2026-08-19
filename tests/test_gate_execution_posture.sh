#!/usr/bin/env bash
# tests/test_gate_execution_posture.sh — change 0223. Guards for the build gate's EXECUTION
# posture: the contract clauses in docket-build, finalize's citation-by-reference, the
# per-harness reference, and the default budget's agreement across every surface that states it.
# Run: bash tests/test_gate_execution_posture.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
BUILD="$REPO/skills/docket-build/SKILL.md"
REF="$REPO/skills/docket-build/references/gate-execution.md"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# Phrase asserts read a whitespace-FLATTENED haystack. grep matches within a line, so a
# phrase-spanning assert over hard-wrapped markdown silently doubles as a line-wrap guard: a pure
# re-flow reddens it with a message about a policy nobody changed. `-s` (squeeze) is load-bearing,
# not tidiness — a wrapped list item indents its continuation, so a plain newline-to-space swap
# leaves words four spaces apart and a single-space pattern misses.
flatten(){ tr -s '[:space:]' ' '; }

assert "build: SKILL.md exists" '[ -f "$BUILD" ]'
build_body="$(cat "$BUILD" 2>/dev/null)"
build_flat="$(flatten <<<"$build_body")"
# Non-vacuity floor: every assert below reads these variables, so an unreadable file must redden
# HERE rather than passing every negative grep by default.
assert "build: body is non-vacuous (>= 150 lines)" \
  '[ "$(grep <<<"$build_body" -c .)" -ge 150 ]'

# --- (1) the posture subsection exists, INSIDE the build gate -------------------
# LINE-anchored on purpose: the heading level is the signal — a `##` here would make the posture a
# sibling of the gate rather than part of it, and flattening erases the line start.
assert "posture: is a subsection heading" \
  'grep -qE "^### Gate execution posture" <<<"$build_body"'

# Groups (2)-(4) read a SECTION SLICE, not the whole file. The plan drafted them file-wide; that is
# too weak for at least one of them, and weak in the same way for the rest. The repair-task clause
# is the proof: `## Halting conditions`' pre-existing "Never convert this into a repair task"
# (the undetectable-suite case) already satisfies a file-wide match, so deleting the posture's own
# refusal would have reddened nothing — a guard that cannot fail on the mutation it exists to catch.
# This is the same reasoning tests/test_docket_build.sh records for its `halt_blk` slice: "a
# whole-file grep would be satisfied by the in-place rule that points here". Consequence, recorded
# rather than discovered later: deleting or re-levelling the heading reddens (1) AND every scoped
# assert together. That cascade is correct — a posture with no section states none of these rules —
# and it is why each clause is still mutation-tested individually, one clause at a time.
posture_blk="$(awk '/^### Gate execution posture$/{f=1;next} f && /^#+ /{f=0} f' <<<"$build_body")"
posture_flat="$(flatten <<<"$posture_blk")"
assert "posture: the subsection body is non-vacuous (>= 20 lines)" \
  '[ "$(grep -c . <<<"$posture_blk")" -ge 20 ]'

# --- (2) the load-bearing clauses ----------------------------------------------
# Keyed on the RULE, not on any wording introduced by this change alone, so a faithful rewrite
# stays green and a rewrite that drops the rule reddens.
assert "posture: must not depend on a single foreground call" \
  'grep -qiE "(not|never)[^.]{0,120}single foreground" <<<"$posture_flat"'
assert "posture: requires a durable result artifact" \
  'grep -qiE "durable[^.]{0,60}(result|artifact)" <<<"$posture_flat"'
assert "posture: completion is established from the artifact, not the caller signal" \
  'grep -qiE "(never|not)[^.]{0,140}completion signal" <<<"$posture_flat"'
assert "posture: observation is bounded by a finite budget" \
  'grep -qiE "(bounded|finite)[^.]{0,80}budget" <<<"$posture_flat"'
assert "posture: names the resolved budget export" \
  'grep -qF "GATE_OBSERVATION_BUDGET" <<<"$posture_blk"'
assert "posture: exhausting the budget FAILS CLOSED" \
  'grep -qiE "fail[s]? closed" <<<"$posture_flat"'
# The `0` reading lives on the CONFIG surfaces (`.docket.example.yml`, mirrored in the resolver):
# "0 is legal and means observe once, then fail closed". The contract the agent EXECUTES has to
# carry it too, or a zero budget reads as exhausted before any observation — zero observations,
# not the one observation the config promises. Keyed on the two poles of that reading (the value
# and the single observation), not on either surface's wording.
assert "posture: a zero budget still buys ONE observation" \
  'grep -qiE "0[^.]{0,140}observ(e|ation)[^.]{0,40}once|once[^.]{0,60}0[^.]{0,80}fail" <<<"$posture_flat"'
# Fail-closed is a HALT, never a red — the distinction that keeps an unfinished run from
# manufacturing an integration-repair task.
assert "posture: fail-closed must not mint a repair task" \
  'grep -qiE "(never|not|no)[^.]{0,120}repair task" <<<"$posture_flat"'

# --- (3) the false-completion rule ---------------------------------------------
assert "posture: a stale pre-yield report is not evidence of a crash" \
  'grep -qiE "stale[^.]{0,120}(crash|not evidence)" <<<"$posture_flat"'

# --- (4) the ADR-0024 boundary is NOT relaxed ----------------------------------
assert "posture: distinguishes a dispatched subagent from an external gate process" \
  'grep -qiE "dispatched subagent" <<<"$posture_flat"'

# (4a)/(4b) read TWO slices tighter than $posture_blk — the numbered clause list, and the "does not
# relax" paragraph — for the same reason groups (2)-(4) read a section slice at all. The scoping
# below is deliberately stated in BOTH places (the clause is where the rule is performed; the
# paragraph is where a reader goes to resolve the ADR-0024 conflict), so a section-wide grep would
# let either statement be deleted while the other kept every assert green — a guard that cannot fail
# on the mutation it exists to catch. The clause slice ends at the first column-0 `**` line, which is
# the list's own terminator shape: continuation lines are indented, so none can close it early.
clauses_blk="$(awk '/^1\. /{f=1} f && /^\*\*/{f=0} f' <<<"$posture_blk")"
clauses_flat="$(flatten <<<"$clauses_blk")"
assert "posture: the numbered clause list was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$clauses_blk")" -ge 12 ]'
# Anchored on the RULE NAME (`never-yield`) in a bolded lead-in, not on the sentence's wording, so a
# faithful rewrite of the paragraph still locates it.
relax_blk="$(awk '/^\*\*.*never-yield/{f=1} f && /^[[:space:]]*$/{f=0} f' <<<"$posture_blk")"
relax_flat="$(flatten <<<"$relax_blk")"
assert "posture: the 'does not relax' paragraph was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$relax_blk")" -ge 4 ]'

# (4a) The yield permission is SCOPED BY THE OBSERVING AGENT'S OWN DISPATCH POSTURE. An unqualified
# "you may yield" contradicts ADR-0024 on docket's DEFAULT path, not an exotic one: this role is
# invoked inside `docket-implement-next` Step 5, and that role is itself dispatched — so the agent
# doing the yielding is a dispatched agent, which by ADR-0024 has no channel to receive the
# resumption signal it is waiting on and comes back half-done reading as `completed`. Observed live
# on change 0223: three dispatched build workers yielded to await a gate completion event, none was
# resumed by it, and one run had to be discarded. Keyed on the rule's two poles — who may yield, and
# what the one who may not does instead — never on a single phrasing.
assert "posture: the yield is available only to a top-level session agent" \
  'grep -qiE "(top-level|top level)[^.]{0,200}yield|yield[^.]{0,200}(top-level|top level)" <<<"$clauses_flat"'
# Boundary class is `[^[:alnum:]]`, not `\b` (absent from BSD grep's ERE, AGENTS.md § Shell) and not
# `[[:space:]]` — the negation is bolded in the body, so `**never**` has no space on its left. The
# negation-to-`yield` window is TIGHT (12) where the other windows are loose, and that is mutation
# evidence rather than taste: at 120 the semantic inversion "a dispatched or forked child has no
# such channel, so it may yield" stayed GREEN, because the sentence's own "has **no** such channel"
# sat 24 characters from the un-negated verb and satisfied the pair. The negation has to attach to
# the verb it negates, so only a short gap counts.
assert "posture: a dispatched or forked build role may NEVER yield" \
  'grep -qiE "(dispatched|forked)[^.]{0,200}[^[:alnum:]](never|not|no)[^[:alnum:]][^.]{0,12}yield" <<<"$clauses_flat"'
assert "posture: the non-yielding path observes by BLOCKING instead" \
  'grep -qiE "block(ing|s)?[^.]{0,160}observ|observ[^.]{0,160}block(ing|s)?" <<<"$clauses_flat"'

# (4b) ...and the conflict-resolving paragraph states the same scoping, since that is where a reader
# lands when clause 4 and ADR-0024 appear to disagree. It must also name docket's own default path
# as the dispatched case, or the reader reads the blocking branch as the exotic one.
assert "relax: scopes the yield to a top-level session agent" \
  'grep -qiE "(top-level|top level)[^.]{0,200}yield|yield[^.]{0,200}(top-level|top level)" <<<"$relax_flat"'
assert "relax: names docket's own default path as the dispatched case" \
  'grep -qF "docket-implement-next" <<<"$relax_blk"'

# --- (5) HARNESS NEUTRALITY: the body names no mechanism -----------------------
# This is the negative that keeps the quarantine real, and it is deliberately WHOLE-FILE: the
# neutrality invariant binds the skill body everywhere, not only inside the posture section.
# Anchored word-wise so ordinary prose ("background" as an English word) cannot trip it while a
# real mechanic does.
for mech in nohup setsid "&>" "process group" "shell id"; do
  assert "neutrality: build body does not name the mechanism '$mech'" \
    '! grep -qiF -- "'"$mech"'" <<<"$build_body"'
done
# No harness FIGURE either. The one number the body may carry is docket's own default budget, so
# the pattern requires a UNIT and therefore cannot catch a bare `30` — but it keys on the duration
# SHAPE (digits, optional space or hyphen, a time unit, a non-alphanumeric boundary) rather than on
# an enumerated list of spellings, per AGENTS.md § Guards and tests. The list this replaced was the
# failure that rule names: `000 ms` was subsumed by `ms`, and `minute foreground` / `s timeout` were
# ad-hoc literals, so the obvious leaks — `180s`, `5 minutes`, `a 2-minute ceiling`, `20 seconds` —
# every one of them passed, and the invariant read as guarded while guarding almost nothing.
# The trailing `([^[:alnum:]]|$)` is what keeps the bare `s`/`m` units honest: it is why "5 steps"
# and "5 more" do not match. `[^[:alnum:]]` rather than `\b`, absent from BSD grep's ERE
# (AGENTS.md § Shell). Docket's own "default 30, in minutes" is separated by a comma, so the unit
# never abuts the digits and the policy default stays sayable.
assert "neutrality: build body states no harness timeout figure" \
  '! grep -qiE "[0-9]+[[:space:]]*-?[[:space:]]*(milliseconds?|minutes?|seconds?|hours?|ms|secs?|mins?|hrs?|[sm])([^[:alnum:]]|$)" <<<"$build_body"'

# --- (6) the blocking pointer to the quarantine --------------------------------
assert "posture: points at the per-harness reference" \
  'grep -qF "references/gate-execution.md" <<<"$posture_blk"'
assert "reference: the file exists" '[ -f "$REF" ]'
ref_body="$(cat "$REF" 2>/dev/null)"
assert "reference: is non-vacuous (>= 40 lines)" \
  '[ "$(grep <<<"$ref_body" -c .)" -ge 40 ]'
# Six capabilities, counted rather than spot-checked: dropping one is the drift that matters. The
# count is SLICED to the section that owns the enumeration, using the same awk pattern group (10)
# uses. A whole-file count over `^[0-9]+\. ` is the same defect the slices elsewhere in this file
# exist to avoid, in both directions: a future numbered list under `## Method` breaks it with a
# message about capabilities, and six numbered lines split across two sections would satisfy it.
caps_blk="$(awk '$0 == "## The six required capabilities" {f=1;next} f && /^#+ /{f=0} f' <<<"$ref_body")"
assert "reference: the capabilities section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$caps_blk")" -ge 12 ]'
assert "reference: enumerates exactly 6 required capabilities" \
  '[ "$(grep -cE "^[0-9]+\. " <<<"$caps_blk")" -eq 6 ]'

# --- (7) the halting condition --------------------------------------------------
# LINE-anchored, and scoped to the section that owns the enumeration: `## Halting conditions` is
# where a halt becomes a disposition rather than a mention, so a budget bullet anywhere else is
# not this rule. The `^- **` shape is the section's own bullet form.
halt_blk="$(awk '/^## Halting conditions$/{f=1;next} f && /^## /{f=0} f' <<<"$build_body")"
assert "build: the Halting conditions section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$halt_blk")" -ge 15 ]'
assert "build: Halting conditions carries the exhausted-budget bullet" \
  'grep -qiE "^- \*\*.*budget" <<<"$halt_blk"'

# --- (8) finalize CITES docket-build for the yield rule; the run mechanics are the binary's -------
# RE-KEYED BY CHANGE 0316 (plan Task 20, category (c): behavior preserved, locators brittle — do
# NOT edit the skill to satisfy these). Change 0316 rewrote docket-finalize-change/SKILL.md from a
# Bash procedure into a Go-verb sequencer, and commit 8c74c1c8 NARROWED what finalize cites from
# docket-build's *Gate execution posture* to point 4 ALONE — the yield-vs-block rule, a property of
# the CALLER's own dispatch posture that `docket gate` cannot own. The run mechanics docket-build's
# posture used to be the sole home for — the supervised run, the durable run directory, artifact-
# based completion, and the fail-closed observation budget — are now owned by the `docket gate` verb
# (plan authority #2: "`docket gate` owns the gate's mechanics") and are legitimately DESCRIBED as
# its ownership in the skill. So this group is re-keyed, not deleted:
#   * the CITATION positives survive, re-pointed from the old `- `local` runs the suite` per-step
#     bullet (gone with the Bash procedure) to step 4's gate paragraph, located by its own subject;
#   * the two no-restatement NEGATIVES are RETIRED. They forbade the durable-artifact and fail-closed
#     clauses as docket-build restatements, but those clauses are no longer docket-build's to
#     restate — they are the binary's, and the skill states so on purpose (Authority #3: the skill's
#     positive "`docket gate` owns the gate's mechanics … One clause … it cannot own is yours to
#     obey"). Deleting a guard is how a regression hides, so each is replaced by the POSITIVE that
#     pins the new split: mechanics attributed to `docket gate`, the ONE cited clause the yield rule.
#     Undoing the split reddens the replacements.
FIN="$REPO/skills/docket-finalize-change/SKILL.md"
assert "finalize: SKILL.md exists" '[ -f "$FIN" ]'
fin_body="$(cat "$FIN" 2>/dev/null)"
fin_flat="$(flatten <<<"$fin_body")"
# Non-vacuity floor, re-baselined to the Go sequencer's real size (79 non-empty lines at 0316); an
# unreadable or truncated file must redden HERE rather than passing every positive grep by default.
assert "finalize: body is non-vacuous (>= 70 lines)" \
  '[ "$(grep <<<"$fin_body" -c .)" -ge 70 ]'
# The POSITIVES read step 4's gate paragraph — the one run this posture governs — not the whole
# file, so a citation parked in an unrelated paragraph does not satisfy them. The sequencer states
# the same governance the old item-5 bullet did, in one paragraph located by its own subject phrase
# ("owns the gate's mechanics"), which is unique to that paragraph.
fin_gate="$(grep -E "owns the gate.s mechanics" <<<"$fin_body")"
gate_flat="$(flatten <<<"$fin_gate")"
assert "finalize: the gate-posture paragraph was located (non-vacuity anchor)" \
  '[ -n "$fin_gate" ]'
# The citation names the OWNER, so a reader lands on the single source.
assert "finalize: local gate cites the gate execution posture" \
  'grep -qiE "gate execution posture" <<<"$gate_flat"'
assert "finalize: the citation names docket-build as the owner" \
  'grep -qiE "gate execution posture[^.]{0,120}docket-build|docket-build[^.]{0,120}gate execution posture" <<<"$gate_flat"'
# RETIREMENT REPLACEMENTS for the two no-restatement negatives (see the header). These pin the split
# 8c74c1c8 drew, keyed on SHAPE within the located paragraph (whose non-vacuity is anchored above),
# never on character distance. Re-attributing the durable dir / fail-closed budget to docket-build,
# or dropping the yield-clause cite, reddens them. `.` stands for the backtick around `docket gate`
# — a literal backtick in the assert expression would be command-substituted by `assert`'s `eval`.
assert "finalize: the run mechanics (durable dir, fail-closed budget) are attributed to docket gate" \
  'grep -qiE "docket gate. owns the gate" <<<"$gate_flat" && grep -qiE "durable run directory" <<<"$gate_flat" && grep -qiE "fails? closed" <<<"$gate_flat"'
assert "finalize: the ONE clause cited from docket-build is the yield-vs-block rule (the narrowing)" \
  'grep -qiE "cannot own" <<<"$gate_flat" && grep -qiE "yield" <<<"$gate_flat"'

# --- (9) the default budget agrees across every surface that states it ---------
# Four independent statements of one value drift silently. Derive each from its own file with its
# own extractor keyed on that file's own idiom, then compare — the number is hardcoded in exactly
# one place (the resolver), which is the place that seeds it.
EX="$REPO/.docket.example.yml"
RM="$REPO/README.md"
CFG="$REPO/scripts/docket-config.sh"

# First line of an already-captured blob. Deliberately NOT `producer | head -n1`: head exits early
# and SIGPIPEs the producer under `pipefail` (AGENTS.md § Shell), and a 141 here would arrive as an
# empty extraction — i.e. as a drift report about a value nobody changed.
first(){ sed -n '1p' <<<"$1"; }

# Each extractor peels the digits off the END of its own match rather than splitting on a
# separator. The plan drafted the resolver one as `sed 's/.*://'` over `GATE_OBSERVATION_BUDGET:-30`;
# the greedy `.*:` stops at the parameter-expansion colon and yields `-30`, so the comparison tested
# a value that appears on no surface and would have stayed green through a real drift on any of the
# other three. `[0-9]+$` has no such seam.
gob_resolver="$(first "$(grep -oE 'GATE_OBSERVATION_BUDGET:-[0-9]+' "$CFG")" | grep -oE '[0-9]+$')"
gob_example="$(first "$(grep -oE '^gate_observation_budget:[[:space:]]*[0-9]+' "$EX")" | grep -oE '[0-9]+$')"
gob_readme="$(first "$(grep -oE 'gate_observation_budget` \(default `[0-9]+' "$RM")" | grep -oE '[0-9]+$')"
# The skill body states it in prose, so this one reads the flattened haystack; `[^.]{0,120}` cannot
# cross a sentence end, which is what keeps it off the § Halting conditions mention of the same
# export (that sentence carries no default).
gob_skill="$(first "$(grep -oE 'GATE_OBSERVATION_BUDGET[^.]{0,120}default [0-9]+' <<<"$build_flat")" | grep -oE '[0-9]+$')"

# NON-VACUITY first: each extractor must actually have extracted something. Without this, every
# extraction failure — wrong path, renamed file, broken pattern — reads as the property holding,
# and two empty strings compare equal.
for pair in "resolver:$gob_resolver" "example:$gob_example" "readme:$gob_readme" "skill:$gob_skill"; do
  assert "budget: the ${pair%%:*} extractor found a value (got '${pair#*:}')" \
    '[ -n "'"${pair#*:}"'" ]'
done
assert "budget: resolver and example agree ($gob_resolver vs $gob_example)" \
  '[ "$gob_resolver" = "$gob_example" ]'
assert "budget: resolver and README agree ($gob_resolver vs $gob_readme)" \
  '[ "$gob_resolver" = "$gob_readme" ]'
assert "budget: resolver and skill body agree ($gob_resolver vs $gob_skill)" \
  '[ "$gob_resolver" = "$gob_skill" ]'

# --- (10) EVERY shipped harness has a recorded verdict ------------------------
# The population is DERIVED from HD_SHIPPED_HARNESSES, never hand-listed: a fifth harness reddens
# this automatically, which is the whole point. An allowlist here would be the enumerated floor
# that ages directly into the gap it was written to close.
HD="$REPO/scripts/lib/harness-defaults.sh"
assert "verdicts: harness-defaults.sh exists" '[ -f "$HD" ]'
# Sourced tolerantly and read through `:-` so a missing or broken sidecar reddens the population
# floor below with a readable message, instead of aborting the whole file on `set -u` before the
# remaining asserts ever run.
# shellcheck source=/dev/null
. "$HD" 2>/dev/null || true
shipped="${HD_SHIPPED_HARNESSES:-}"
# Floor on the POPULATION itself: a failed source would leave it empty and the loop below vacuous —
# zero iterations are indistinguishable from success.
n_shipped="$(grep -c . <<<"$(printf '%s\n' $shipped)")"
assert "verdicts: HD_SHIPPED_HARNESSES is non-empty (got $n_shipped)" '[ "$n_shipped" -ge 4 ]'
for h in $shipped; do
  # Presence is a whole-FILE property, so this one is deliberately unscoped. `[[:space:]]*$` rather
  # than `\b`: BSD grep's ERE has no `\b`, and a bounded trailing-space class is portable.
  assert "verdicts: reference has a section for '$h'" \
    'grep -qE "^### '"$h"'[[:space:]]*$" <<<"$ref_body"'
  # The verdict must be one of the three legal tokens AND must belong to THIS harness's section. A
  # file-wide grep would be satisfied by a neighbour's verdict line — the same defect the group
  # (2)-(4) slices exist to avoid — so the haystack is a section slice. The slice is captured into a
  # variable first: `awk … | grep -q` would SIGPIPE the awk under `pipefail`.
  #
  # GRAMMAR (widened by change 0223's second review wave): the token may be followed by an em-dash
  # SCOPE clause — `**Verdict:** `token` — <scope>` — which is how a row whose evidence is narrower
  # than the capability list declares that limit where the verdict is read rather than three
  # paragraphs above it. The clause is optional but its SHAPE is not: the separator is required and
  # the scope must be non-empty, so `token` followed by loose trailing prose still reddens. Keeping
  # `$` anchored is the whole point — without it the token check degenerates into "contains a legal
  # word somewhere on the line".
  h_blk="$(awk -v h="$h" '$0 == "### " h {f=1;next} f && /^#+ /{f=0} f' <<<"$ref_body")"
  assert "verdicts: '$h' records a legal verdict token" \
    'grep -qE "^\*\*Verdict:\*\* .(supported|unverified|incompatible).( — .+)?$" <<<"$h_blk"'
done
# Reverse direction: no verdict section for a harness docket does not ship. The mirror check — a
# guard over a correspondence proves only the direction it iterates, so the forward loop above would
# pass unchanged with a phantom fifth section sitting in the file.
ref_sections="$(grep -oE "^### [a-z-]+" <<<"$ref_body" | sed 's/^### //' | sort)"
shipped_sorted="$(printf '%s\n' $shipped | sort)"
assert "verdicts: the reference's harness sections EQUAL HD_SHIPPED_HARNESSES" \
  '[ "$ref_sections" = "$shipped_sorted" ]'

# --- (10b) a verdict TOKEN claims no more than the probe measured ---------------
# Group (10) proves every shipped harness HAS a verdict; it says nothing about what that verdict is
# entitled to mean. § *Method* measures three of the six capabilities — survival past the initiating
# call, redirection to a durable location, and a terminal sentinel — so a token defined against all
# six overclaims on every row at once. The bound therefore has to sit in `## Reading a verdict`,
# where the token is DEFINED: a bound stated only inside one harness's prose leaves the definition
# itself unbounded for the other rows.
#
# Sliced to that section for the same reason groups (2)-(4) slice at all. § *Method* already
# discusses at length what it did and did not do, so a file-wide grep for "unmeasured" is satisfied
# by prose that was there before this guard existed — a guard that cannot fail on the mutation it
# exists to catch.
verdict_def="$(awk '/^## Reading a verdict$/{f=1;next} f && /^## /{f=0} f' <<<"$ref_body")"
verdict_flat="$(flatten <<<"$verdict_def")"
assert "verdict scope: the 'Reading a verdict' section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$verdict_def")" -ge 12 ]'
assert "verdict scope: a verdict covers ONLY what Method measured" \
  'grep -qiE "verdict[^.]{0,80}only[^.]{0,80}measur" <<<"$verdict_flat"'
# Per capability, because they are unmeasured for three DIFFERENT reasons (4: the observer sits
# outside the harness; 5: the stand-in gate only ever succeeds; 6: never probed), and a single
# "some capabilities are unmeasured" sentence would let any two of them be quietly re-claimed. The
# numbers are the reference's own structure — an assert above already counts exactly six — not an
# enumeration of spellings. The negation must attach NEAR the capability it qualifies: `[^.]` cannot
# cross a sentence end, so a neighbouring bullet's "not" cannot stand in for a missing one.
for c in 4 5 6; do
  assert "verdict scope: capability $c is declared unmeasured" \
    'grep -qiE "capability \**'"$c"'\**[^.]{0,120}[^[:alnum:]](not|never|un(measured|observed|probed))" <<<"$verdict_flat"'
done
# ...and the escape hatch is named where it is used: a row narrower than the general bound puts the
# limit ON its verdict line, which is what the widened grammar in group (10) accepts.
assert "verdict scope: a narrower row carries its limit on the verdict line" \
  'grep -qiE "verdict line" <<<"$verdict_flat"'

# --- (10c) the mode a row was measured in, where docket does not run in it -------
# The `supported` row for the harness docket itself runs under was measured as two foreground calls
# of an INTERACTIVE session. That is not docket's execution mode: this gate runs inside
# `docket-build`, invoked inline by the forked `docket-implement-next`, and a forked child has no
# channel on which a resumption signal can arrive — which is exactly why the posture's clause 4
# forbids that child to yield. So the one mode matching docket's real topology is the mode NOT
# measured, and an unqualified verdict reads as though it were.
#
# DERIVED, never hand-listed: the loop asks which section addresses the forked/dispatched mode
# rather than naming a harness, because which product docket runs under can change while the claim
# being guarded — that the measured mode is not the running mode — does not. The counter is the
# non-vacuity floor: deleting the mode discussion outright would otherwise leave the loop with zero
# iterations, which is indistinguishable from success.
mode_secs=0
for h in $shipped; do
  h_blk="$(awk -v h="$h" '$0 == "### " h {f=1;next} f && /^#+ /{f=0} f' <<<"$ref_body")"
  # The PROSE asserts read the section with its verdict line REMOVED, and that is mutation evidence
  # rather than tidiness: at full-section scope both of them were satisfied by the scope clause on
  # the verdict line itself, so gutting the evidence paragraph — the statement they exist to
  # protect — left them green while only the verdict-line assert reddened. Three asserts that
  # collapse onto one line are one assert. The verdict-line assert below keeps the full slice,
  # because the line is exactly what it is about.
  h_prose="$(grep -v '^\*\*Verdict:\*\*' <<<"$h_blk")"
  h_flat="$(flatten <<<"$h_prose")"
  grep -qiE "forked|dispatched" <<<"$h_flat" || continue
  mode_secs=$((mode_secs + 1))
  # `[^-]` is load-bearing, not decoration: the same paragraph names the stricter variant it could
  # NOT obtain as a "non-interactive `claude -p` child", so a bare `interactive` stayed GREEN through
  # a mutation that deleted the measured mode's name outright — the negative spelling of the word
  # stood in for the positive one. Change 0234 moved that negative spelling off this surface into
  # `gate-execution-evidence.md`, so the compressed `### claude` section here no longer contains
  # `non-interactive` at all; `[^-]` is retained against its return, not as live evidence.
  assert "modes: '$h' names the mode its evidence WAS measured in" \
    'grep -qiE "(^|[^-])interactive" <<<"$h_flat"'
  assert "modes: '$h' records the forked/dispatched mode as UNMEASURED" \
    'grep -qiE "(forked|dispatched)[^.]{0,120}(unmeasured|not measured)|(unmeasured|not measured)[^.]{0,120}(forked|dispatched)" <<<"$h_flat"'
  # The scope has to be on the VERDICT line: a caveat buried in the evidence paragraph is not what
  # a reader scanning rows sees, and the row is the artifact other skills cite.
  assert "modes: '$h' carries the mode scope on its verdict line" \
    'grep -qE "^\*\*Verdict:\*\* .(supported|unverified|incompatible). — .+$" <<<"$h_blk"'
done
assert "modes: some harness section addresses the forked/dispatched mode (got $mode_secs)" \
  '[ "$mode_secs" -ge 1 ]'

# --- (11) the SPLIT: probe evidence lives OFF the blocking-read surface (change 0234) ---
# `gate-execution.md` is read blocking before every gate run (docket-build § Gate execution
# posture). The probe design, the launch-duration ladder, and the per-harness measurement
# narratives are a measurement report, not instruction, and they rot on an external schedule —
# so they live in a sibling that no gate run loads. These four asserts pin that split.
EVID="$REPO/skills/docket-build/references/gate-execution-evidence.md"
assert "evidence: the file exists" '[ -f "$EVID" ]'
evid_body="$(cat "$EVID" 2>/dev/null)"
# Population floor, same shape as the kept file's: without it the split could silently collapse
# back into one file with the evidence DELETED rather than moved, and every assert here would
# still pass on an empty sibling.
assert "evidence: is non-vacuous (>= 40 lines)" \
  '[ "$(grep <<<"$evid_body" -c .)" -ge 40 ]'
# Closes the reverse direction of the absence assert below: that one proves the Method section LEFT
# the kept file, this one proves it ARRIVED here rather than being deleted outright. Keyed on the
# heading, so it pins structure, not prose — a rewrite of the evidence text does not rot it.
assert "evidence: carries the Method section" 'grep -qE "^## Method" <<<"$evid_body"'
assert "reference: points at the evidence file" \
  'grep -qF "gate-execution-evidence.md" <<<"$ref_body"'
# ABSENCE assert, deliberately: a guard asserting a removed class is ABSENT cannot go stale, because
# the only way to redden it is to reintroduce the thing (learnings:
# restatement-accumulates-its-own-guards, the 0194 entry). A positive assert that the evidence file
# still CONTAINS the method section would instead pin a copy and rot on its next rewrite.
assert "reference: carries no Method section (evidence stays off the blocking surface)" \
  '! grep -qE "^## Method" <<<"$ref_body"'
# The heading assert above detects ONE heading, not the CLASS of content the split removed
# (learnings: assert-detects-removal-not-replacement). What moved out is measured probe FIGURES, and
# the likeliest regrowth path is the launch-duration sentences pasted back under the `### <harness>`
# headings they lived under before 0223's structure settled — which leaves the heading assert green
# and is caught only once regrowth exceeds the ratcheted budget. So pair it with a SHAPE assert on
# the figures themselves: a bold duration figure, `**<digits>s**`.
# Verified in BOTH directions against live content at the time of writing:
#   (a) it does NOT match the kept file — `gate-execution.md` contains no `[0-9]+s` at all; its only
#       reference to the timings is the prose pointer "measured launch durations", which carries no
#       figure.
#   (b) it DOES match every one of the four harness narratives in `gate-execution-evidence.md`
#       (`**0s**`, `**19s**`, `**11s**`, `**5s**`). The reviewer's proposed `returned in \*\*[0-9]+s\*\*`
#       was checked and REJECTED: the phrasing is uniform but the `claude -p` narrative wraps between
#       "returned in" and "**19s**", and grep is line-oriented, so that form matches only 3 of 4 and
#       would miss a paste of the wrapped sentence. Keyed on the figure's shape rather than on the
#       four literal values, which would age straight into the gap this closes.
assert "reference: carries no measured launch-duration figures (shape: bold **Ns**)" \
  '! grep -qE "\*\*[0-9]+s\*\*" <<<"$ref_body"'

# --- (12) the posture is wired to the SHIPPED helper (change 0282) --------------
# Three rules joined the posture beside clauses 1-6: the helper plus the liveness-keyed wait, the
# `died` posture with its ONE `--stop`-gated relaunch for an idempotent child, and the rule for
# abandoning a child that is still running. Each assert keys on the RULE's shape — a negation or a
# co-occurrence inside a window `[^.]` cannot carry across a sentence end — never on an enumerated
# list of spellings, and never on wording this change alone introduced.
#
# No pattern below may contain a backtick: `assert` runs its argument through `eval`, so a
# backtick inside the pattern would be command substitution rather than a literal. `[^.]` already
# covers the code spans these rules are written with.
CONTRACT="$REPO/scripts/gate-run.md"
assert "helper: the contract file exists" '[ -f "$CONTRACT" ]'
contract_body="$(cat "$CONTRACT" 2>/dev/null)"
assert "helper: the contract is non-vacuous (>= 100 lines)" \
  '[ "$(grep <<<"$contract_body" -c .)" -ge 100 ]'

# PARAGRAPH slices, the same shape `$relax_blk` above uses: a column-0 bolded lead-in opens the
# slice, the next column-0 bolded lead-in or heading closes it. Section-wide scope is too weak
# here for the reason groups (2)-(4) slice at all — all three rules discuss the helper and its
# states, so any one of them could be deleted while its neighbours kept a section-wide grep green.
para(){ awk -v pat="$1" 'index($0,pat)==1{f=1;print;next} f && (/^\*\*/ || /^#/){f=0} f' <<<"$posture_blk"; }
helper_blk="$(para '**The shipped implementation')"
died_blk="$(para '**On the died state')"
abandon_blk="$(para '**Abandoning a live child')"
helper_flat="$(flatten <<<"$helper_blk")"
died_flat="$(flatten <<<"$died_blk")"
abandon_flat="$(flatten <<<"$abandon_blk")"
# Non-vacuity anchors FIRST: an unlocated slice is empty, and every positive grep below would then
# read a deleted rule as a missing paragraph rather than passing silently — but only if the anchor
# reddens with a message that says so.
assert "helper: the shipped-implementation paragraph was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$helper_blk")" -ge 3 ]'
assert "died: the died-posture paragraph was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$died_blk")" -ge 6 ]'
assert "abandon: the abandon paragraph was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$abandon_blk")" -ge 2 ]'

# (12a) the helper, and the wait predicate it exists for.
# A bare `grep -F gate-run` is NOT enough, measured: replacing the invocation with the prose "the
# helper" left it green off the `gate-run.md` pointer two sentences later. So the posture must name
# the INVOCATION a worker actually runs — the facade op — and every verb the contract publishes.
# The verb set is DERIVED from the contract's own Usage block, so a fourth verb reddens this
# automatically instead of aging into a hand-written list.
assert "helper: the posture names the facade invocation" \
  'grep -qE "docket\.sh gate-run" <<<"$helper_blk"'
verbs="$(grep -oE '^gate-run\.sh --[a-z-]+' <<<"$contract_body" | sed 's/.*--/--/' | sort -u)"
n_verbs="$(grep -c . <<<"$verbs")"
assert "helper: the contract's verb set was located (got $n_verbs)" '[ "$n_verbs" -ge 3 ]'
for v in $verbs; do
  assert "helper: the posture gives the '$v' verb a role" 'grep -qF -- "'"$v"'" <<<"$helper_blk"'
done
assert "helper: the posture points at the helper contract for the state vocabulary" \
  'grep -qF -- "gate-run.md" <<<"$helper_blk"'
# THE headline rule. Keyed on its two poles — what the wait IS keyed on (the observed state) and
# what it is NOT keyed on (a marker) — with the negation required to attach to the marker, because
# the paragraph's own explanatory sentence also contains the word "marker" and must not be able to
# stand in for the rule it explains. Mutation: delete the liveness-keyed sentence -> red.
assert "helper: the wait is keyed on the observed state, never on a marker" \
  'grep -qiE "(state|observ)[^.]{0,160}[^[:alnum:]](never|not)[^[:alnum:]][^.]{0,40}marker" <<<"$helper_flat"'
assert "helper: only running is retryable" \
  'grep -qiE "only[^.]{0,20}running[^.]{0,20}retryable" <<<"$helper_flat"'

# (12a-ii) THE CALLER'S LOOP IS NOT REINVENTED PER CALL SITE (change 0286). A live loop matched
# bare state names against a line whose first field is the printed "state=passed" form, so a
# finished gate read as unfinished until its budget burned. This is where loops are actually
# authored, so the keying rule is restated here — bound to what it is asserted ABOUT, not merely
# present (learnings: prose-guard-binds-phrase-to-claim). Mutation: delete the added sentence ->
# all three redden. The asserted sentence lives inside `$helper_blk`, and `para()` closes its slice
# at the next column-0 `**` — so the sentence must stay MID-LINE in SKILL.md: reflowing it so
# `**Reuse the canonical loop**` starts a line truncates the slice before it and reddens all three
# against a file where the sentence is plainly present.
assert "helper: the posture points at the contract's canonical loop rather than inviting a new one" \
  'grep -qiE "canonical[^.]{0,80}loop" <<<"$helper_flat"'
assert "helper: the keying rule is bound to the full printed state= form" \
  'grep -qiE "(key|match)[^.]{0,120}state=[^.]{0,60}(form|printed|line)" <<<"$helper_flat"'
assert "helper: and the bare-token loop is named as the thing that never terminates" \
  'grep -qiE "bare[^.]{0,60}(never terminat|does not terminat)" <<<"$helper_flat"'

# (12b) the `died` posture. The three legs are DERIVED from the contract's own token table, never
# hand-listed: a fourth token added there reddens this automatically, which an allowlist could not.
stop_tokens="$(awk -F'|' '
  /^\| *Token *\| *Produced when *\|/ {f=1; next}
  !f {next}
  /^\|[ -]*\|[ -]*\|/ {next}
  /^\|/ {t=$2; gsub(/[^a-z-]/, "", t); if (t != "") print t; next}
  {f=0}' <<<"$contract_body")"
n_tokens="$(grep -c . <<<"$stop_tokens")"
assert "died: the contract's stop-token table was located (got $n_tokens)" '[ "$n_tokens" -ge 3 ]'
# Presence alone is NOT the property, and that is measured rather than assumed: deleting the whole
# `unavailable` bullet left a bare `grep -F unavailable` GREEN, because the neighbouring bullet
# names the token in passing ("`stopped` and `unavailable` never relaunch"). So each token must
# open a disposition of its own, keyed on the list's own bullet SHAPE — a column-0 `- ` followed by
# the token in a code span — never on a mention anywhere in the paragraph.
for t in $stop_tokens; do
  assert "died: the '$t' leg carries a disposition bullet of its own" \
    'grep -qE "^- .'"$t"'. " <<<"$died_blk"'
done
assert "died: a died run is not red and mints no repair work" \
  'grep -qiE "died[^.]{0,120}[^[:alnum:]](not|never|no)[^[:alnum:]][^.]{0,80}(red|repair)" <<<"$died_flat"'
# The ONE relaunch is licensed by IDEMPOTENCE, not by the state — so the two must co-occur, and a
# bare mention of either word is not the rule.
assert "died: the one relaunch is scoped to an idempotent child" \
  'grep -qiE "idempotent[^.]{0,200}(one|single)[^.]{0,80}relaunch|(one|single)[^.]{0,80}relaunch[^.]{0,200}idempotent" <<<"$died_flat"'
assert "died: a non-idempotent child keeps its site's existing posture" \
  'grep -qiE "non-idempotent[^.]{0,140}(existing|its site|unchanged)" <<<"$died_flat"'
assert "died: the relaunch is gated on the stop report" \
  'grep -qiE "(gated|keyed)[^.]{0,60}(on|by)[^.]{0,60}(stop|report)" <<<"$died_flat"'
# MEASURED against the shipped script, and the reason this assert exists: the ordinary stop of a
# live child reports `already-terminal`, not `stopped` (the wrapper survives the TERM, records the
# child's exit, and step 6 finds that record). Prose that implies `stopped` is the common case
# contradicts the shipped behavior, so the ordinary-case naming is pinned here rather than left to
# a reader to rediscover.
assert "died: already-terminal is named as the ordinary live-child stop" \
  'grep -qiE "already-terminal[^.]{0,100}ordinary|ordinary[^.]{0,100}already-terminal" <<<"$died_flat"'
assert "died: the already-terminal leg re-observes and keys on what returns" \
  'grep -qiE "already-terminal[^.]{0,220}observ" <<<"$died_flat"'
# `abort` is required inside the same sentence, and that is mutation evidence too: without it the
# assert was satisfied by the already-terminal bullet's "`stopped` and `unavailable` never
# relaunch" — a true but different statement — so deleting the unavailable leg outright stayed
# green. The rule is that this leg ABORTS and does not relaunch, so both halves must attach.
assert "died: the unavailable leg aborts WITHOUT relaunching" \
  'grep -qiE "unavailable[^.]{0,80}abort[^.]{0,120}(without|never|no)[^.]{0,60}relaunch" <<<"$died_flat"'
assert "died: a second died is abort-and-report" \
  'grep -qiE "second[^.]{0,60}died[^.]{0,80}abort" <<<"$died_flat"'

# (12c) abandoning a child that is still running.
assert "abandon: a run left in the running state is stopped BEFORE the report" \
  'grep -qiE "running[^.]{0,200}stop[^.]{0,100}before[^.]{0,60}report" <<<"$abandon_flat"'
assert "abandon: every leg still halts" \
  'grep -qiE "(every|each)[^.]{0,40}leg[^.]{0,60}halt" <<<"$abandon_flat"'
# The one leg where the human inherits a live process is the one leg that must be loud.
assert "abandon: the unavailable leg halts LOUDLY" \
  'grep -qiE "unavailable[^.]{0,140}loud|loud[^.]{0,140}unavailable" <<<"$abandon_flat"'

# (12d) the reference gains a POINTER at capability 5 and a named mitigation — and NO verdict is
# rewritten. Sliced to the numbered item that owns the four-state requirement: a pointer parked
# anywhere else in the file leaves capability 5 itself still claiming to own the vocabulary.
cap5_blk="$(awk '/^5\. /{f=1} f && /^[0-9]+\. /&&!/^5\. /{f=0} f && /^#/{f=0} f' <<<"$caps_blk")"
assert "reference: capability 5's item was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$cap5_blk")" -ge 3 ]'
assert "reference: capability 5 points at the contract that owns the state vocabulary" \
  'grep -qF -- "gate-run.md" <<<"$cap5_blk"'
mitigation_blk="$(awk '/^One mitigation/{f=1} f && /^[[:space:]]*$/{f=0} f' <<<"$ref_body")"
assert "reference: the mitigation paragraph was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$mitigation_blk")" -ge 4 ]'
# The mitigation must name the FACADE INVOCATION, not merely mention the helper. Drafted as a bare
# `grep -qF gate-run`, this assert PULLED AGAINST tests/test_consuming_repo_scripts.sh: the shortest
# way to satisfy "name the shipped implementation" is a repo-relative `scripts/gate-run.sh`, which
# that guard forbids in every skill body — a skill ships into a consuming repo that has no
# `scripts/` directory of its own. Measured: with `scripts/gate-run.sh` in the paragraph the whole
# of this file stayed GREEN (111 asserts) while the consuming-repo audit went red. Requiring the
# facade spelling makes the two guards agree — the only way to satisfy this one is now a spelling
# the other permits, and the path-shaped alternatives (`scripts/gate-run.sh`, `scripts/docket.sh
# gate-run`) are both caught over there rather than restated here. Same anchor as (12a) uses on the
# skill body, deliberately: the reference and the body name the helper the same way.
assert "reference: the mitigation names the facade invocation of its shipped implementation" \
  'grep -qE "docket\.sh gate-run" <<<"$mitigation_blk"'
# ...and the helper stays OUT of every harness row. This is the mechanical form of "rewrite no
# verdict": a row edited to name the helper is a row whose measured claim moved. Population is
# derived from HD_SHIPPED_HARNESSES for the same reason group (10) derives it.
for h in $shipped; do
  h_blk="$(awk -v h="$h" '$0 == "### " h {f=1;next} f && /^#+ /{f=0} f' <<<"$ref_body")"
  assert "verdicts: '$h' row names no helper — no verdict was rewritten or re-probed" \
    '! grep -qF -- "gate-run" <<<"$h_blk"'
done

exit $fail
