#!/usr/bin/env bash
# tests/test_skill_size_budgets.sh — regrowth guard (change 0085): every skills/**/*.md stays
# within a per-file line/word budget (originally set ~10% above the 0085 post-slim actuals; see the
# BUDGETS comment for how a later raise is set). A future change that
# bloats a skill must slim elsewhere or consciously RAISE the budget in this table (an in-diff edit).
# Budgets are a DIRECTION made durable, not the slim's goal (learnings: size-target-is-direction).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# BUDGETS: one row per tracked file — "<relpath> <maxLines> <maxWords>". The ORIGINAL basis of these
# rows is the 0085 post-slim actuals + ~10% (ceil). A LATER RAISE (0102, 0127, 0137) does not re-apply
# that +10% to a grown file — it sets the row to the file's re-measured actual plus a small WORKING
# MARGIN, so a subsequent one-line edit does not redden CI on arrival. Change 0137's rule for the
# rows it raises: lines rounded up to the next multiple of 5, words to the next multiple of 50 —
# and if that lands within 25 words of the actual, the multiple AFTER it, because a rule that can
# leave a 4-word margin reproduces the failure mode this paragraph exists to forbid.
# Near-zero headroom is not the intent — it is the failure mode 0102 recorded below (1 word left),
# and 0137's first attempt repeated it (rounding words to the next multiple of 10 left +1 and +9).
# To raise a budget, edit the number here in the same diff that grows the file. A raise must
# additionally NAME the references/ file the new prose was considered for and STATE why it cannot
# live there (a rule that must intervene at the moment of action, a cross-skill contract quoted
# where it is produced, ...) — "no other home" is a claim argued in-diff, not asserted
# (change 0201).
# docket-convention/SKILL.md's word budget was raised 5689 -> 5850 by change 0127, which added a
# whole policy dimension to the Auto-capture shared definition (classify -> admit -> suppress, and
# the filtering-precedes-the-cap rule) plus the change_types / nested auto_capture config block and
# the manifest's type: field. The section was compressed by ~150 words first (the mint-sites,
# materiality, and deterministic-mint paragraphs); the residual is normative text with no other
# home. The line budget was NOT raised.
# docket-finalize-change/SKILL.md's word budget was raised 4060 -> 4200 by change 0102, which grew
# the file to 4059/4060 words (1 word of headroom) while wiring finalize.require_pr_approval
# through the resolver — the next edit to that file would have reddened CI on arrival.
# docket-convention/SKILL.md's budget was raised 354/5850 -> 365/6250 by change 0137, which added
# the dispatch-capability resolution rule + its A/B/C tier table. The rule must live in SKILL.md
# itself rather than a reference: it fires at the exact moment an agent is about to wrongly
# conclude dispatch is absent, and a rule sitting in an unread reference file cannot intervene at
# that moment. Set per the rule above from the measured actual: 361 lines -> 365, 6209 words -> 6250.
# docket-implement-next/SKILL.md's word budget was raised 3315 -> 3500 by change 0137, which names
# the Tier A/C posture at its four consuming dispatch sites (Step 0's docket-status sweep and Step
# 6's docket-adr dispatch are Tier A; Step 5's build invocation and Step 6's review invocation are
# Tier C) so the convention's dispatch-capability rule has a producer at every site that actually
# dispatches, not only a definition. Two later fix rounds grew it further: pairing each site's own
# noun with its tier in the same clause (so a proximity-scoped guard can distinguish sites sharing
# one paragraph, instead of a bare tier-literal presence check that a swapped-tier mutation could
# pass), and giving Step 6's two clauses the same *Dispatch-capability resolution* back-pointer the
# other sites carry — the citation, not the tier label, is what stops an agent concluding "no
# dispatch tool". Set from the measured actual: 3445 words -> 3500 (the next multiple of 50 is 3450,
# which would leave a 5-word margin — the within-25 clause above pushes it to 3500). The LINE budget was not raised
# by this change at all — the measured actual (135 lines) still fits the pre-existing 147.
# skills/docket-build-task/SKILL.md is a NEW row added by change 0167, which introduced the
# docket-build-task worker contract. Set per the rounding rule above from the measured actual:
# 99 lines -> 100, 805 words -> 850 (45 words of margin, above the within-25 threshold). The LINE
# budget was then raised 100 -> 105 in a fix round: 1 line of headroom is the exact near-zero
# failure mode this comment block warns against (see the 0102/0137 entries above), so one added
# line to SKILL.md would redden CI on arrival.
# skills/docket-build/SKILL.md is a NEW row added by change 0167, which introduced the docket-build
# controller skill. Set per the rounding rule above from the measured actual: 153 lines -> 155 (2
# lines of margin), 1177 words -> 1200 would leave only 23 words of margin (within the 25-word
# threshold), so the next multiple was taken instead: 1250 (73 words of margin). The LINE budget
# was then raised 155 -> 160 in a fix round, matching the docket-build-task precedent above: 2
# lines of headroom is still near-zero relative to a routine edit, so the next multiple of 5 was
# taken instead (7 lines of margin). Change 0167's final fix wave raised it again, 160/1250 ->
# 165/1300: the whole-branch review found the controller silent on the change's most likely
# first-run failure (a profile agent not yet registered, because install.sh has not re-run since
# the opt-in went live), and that rule has to live where the dispatch happens. Measured actual
# 160 lines -> the next multiple of 5 IS 160, i.e. zero margin, so 165; 1261 words -> 1300.
# skills/docket-convention/SKILL.md's WORD budget was raised 6250 -> 6300 by change 0167's final
# fix wave: the *Agent layer*'s "injected into every wrapper" clause became false when the three
# docket-build profile workers shipped without docket-convention, so the clause now names both
# no-convention exceptions and why. Measured actual 6248 words fits 6250 with 2 words of margin —
# the exact near-zero failure mode this block warns about — so the rule's next multiple was taken:
# 6300. The LINE budget was not raised (361 actual, 365 budget).
# Change 0167's INDEPENDENT WHOLE-BRANCH review raised both docket-build rows again:
# docket-build/SKILL.md 165/1300 -> 225/2050 and docket-build-task/SKILL.md 105/850 -> 115/1000.
# The review's framing was that the contracts state a PREDICATE where they owe a DISPOSITION, and
# its remedy is structural rather than a phrase edit: one `## Halting conditions` section in the
# controller enumerating all nine halts and owning the shared disposition, plus an `## Inputs`
# section, the COMPLETE-commit ancestry verification, the undetectable-suite configuration-gap
# branch, the stray-commit rule, and the plan-checkbox rule; and in the worker, the metadata
# boundary its wrappers cannot get from docket-convention (they do not load it) plus the same
# checkbox rule. Set per the rounding rule above from the measured actuals: controller 221 lines ->
# 225, 1989 words -> 2000 would leave 11 words (within the 25-word threshold) so the next multiple,
# 2050; worker 111 lines -> 115, 962 words -> 1000 (38 words of margin). For LINES this change read
# the same "within-25-of-a-50-step" proportion as a half-step of the 5-line step — 4 and 4 lines of
# margin both clear it, where the earlier 0/2-line margins did not.
# skills/docket-convention/SKILL.md was NOT raised by that review wave: its cardinal fix
# ("except two" -> "except four", plus the pointer at docket-build-task's own Scope) measured
# 361/6267, inside the existing 365/6300.
# references/agent-layer.md's budget was raised 168/1839 -> 175/2000 by change 0168, which moved the
# built-in layer from the twelve wrapper sources to the shipped, harness-indexed
# agents/harness-defaults.yml. The file gains the sidecar's own invariants (concrete harnesses only,
# both fields per entry, no runner:) and the unmapped-pair-ships-unpinned rule — normative text with
# no other home, since this reference IS the configuration mechanics. Set per the rule above from
# the measured actual: 172 lines -> 175, 1948 words -> 1950 is within 25, so the multiple after: 2000.
# Raised again 175/2000 -> 190/2150 by change 0205, which added the runner-wide model/effort
# provenance rules (a shipped default is never forwarded to a child harness; model: is therefore
# required, ADR-0067; effort: omitted defers to lower user layers while `auto` suppresses).
# WHERE ELSE IT WAS CONSIDERED, per the naming requirement above: the natural alternatives are
# scripts/runners/<name>.md (rejected — there are three of them, and a per-adapter home is exactly
# the "learn it twice" failure ADR-0067 exists to remove) and README's Runner delegation subsection
# (which DOES now carry the user-facing statement). It cannot live only in those, because this
# reference is what an agent loads while WRITING an `agents:` entry: the required-model rule has to
# intervene at the moment of action, or the agent emits a config that fails generation. That is the
# same "no other home" argument 0168 made, argued rather than asserted.
# The new prose was drafted long and compressed to a single paragraph before commit (the two-bullet
# form was ~250 words; the shipped paragraph is ~110) — note this is compression of the ADDITION,
# not of pre-existing prose, which the diff leaves untouched apart from the runner-list line.
# Set from the measured actual per the rule above: 184 lines -> 185 leaves 1 line, the near-zero
# failure mode, so the multiple after: 190. 2120 words -> the next multiple of 50 is 2150, whose
# 30-word margin is OUTSIDE the within-25 clause, so 2150 stands (not 2200).
# skills/docket-build/SKILL.md's budget was raised 225/2050 -> 250/2350 by change 0184, which
# retiered the three build profiles to four. The growth is almost entirely the `## Routing` rubric:
# a fourth tier adds a bullet, and the `max`/`high` boundary needed an ORGANIZING PRINCIPLE stated
# ("`max` is for mistakes this build's own correction machinery cannot walk back") rather than a
# longer trigger list, because a list is what the router over-applies. That principle plus the
# `low` tier's cross-file-reasoning condition are normative routing text with no other home — a
# reference file cannot intervene at the moment the router is choosing. Set per the rounding rule
# above from the measured actual: 247 lines -> 250, 2313 words -> 2350 (37 words of margin).
# skills/docket-review/SKILL.md is a NEW row added by change 0170, which introduced docket's own
# bounded read-only whole-branch review role. The file is a compact worker contract in the
# docket-build-task register: scope and metadata boundary, dispatch inputs, the read-only conduct
# rules, the build-evidence verification (present / result: green / head_sha equals branch HEAD,
# with the `unverified-build-state` blocker as the ONLY remedy available to a reviewer), what is in
# and out of review scope, the finding schema plus verdict line, and the abort-and-report halting
# posture. Set per the rounding rule above from the measured actuals: 96 lines -> the next multiple
# of 5 is 100, but 4 lines of margin is the near-zero mode this block warns about at a file this
# short, so the multiple after was taken (105), matching the docket-build-task precedent; 746 words
# -> 750 is within the 25-word threshold, so the multiple after: 800 (54 words of margin).
# skills/docket-convention/SKILL.md's WORD budget was raised 6300 -> 6350 by change 0170's rung
# wrappers. The roster grew thirteen -> sixteen, so the *Agent layer* opening now enumerates a
# seventh wrapped skill (`docket-review`, shared by its three rung wrappers), the convention-
# injection clause names an exception set of eight rather than the four it claimed while listing
# five, and the *Composition* tally gains the review rungs as their own class. All three are count
# prose the suite asserts on, so they have no other home. Set per the rounding rule above from the
# measured actual: 6301 words -> the next multiple of 50 is 6350 (49 words of margin). The LINE
# budget was not raised (361 actual, 365 budget) — every edit reflowed inside an existing line.
# skills/docket-build/SKILL.md's budget was raised 250/2350 -> 265/2450 by change 0170, which made
# the gate's GREEN path mint the build-evidence record and added it to the `## Output` list of
# stable emitted lines. The growth is the record's literal shape: a marker-bounded fenced block
# naming `command` / `result` / `head_sha` / `ran_at`. That shape is a cross-skill contract two
# consumers grep verbatim (`docket-implement-next` Step 6 validates it, `docket-finalize-change`
# reads it out of the PR body), so it has to be quoted where the producer writes it — a prose
# summary or a pointer to a reference file would leave the producer free to drift from the readers.
# Set per the rounding rule above from the measured actuals: 263 lines and 2417 words. 263 -> the
# next multiple of 5 is 265, which leaves 2 lines of margin — the near-zero mode this block warns
# about twice above (docket-build-task 100 -> 105, docket-build 155 -> 160) — so the multiple after
# was taken: 270 (7 lines of margin). 2417 words -> 2450 (33 words of margin, above the 25-word
# threshold). An earlier revision of this paragraph recorded the actuals as 262/2414 and set the
# line budget to 265; both numbers were wrong and the 2-line margin was the failure mode itself.
# skills/docket-implement-next/SKILL.md's WORD budget was raised 3500 -> 3900 by change 0170, which
# gave Step 6 the three things the review role cannot infer: the build-evidence validation (present,
# green, `head_sha` at HEAD, else re-run the suite once), the deterministic rung selection (the
# build's highest routed-or-escalated profile, one step up past 1500 changed lines, capped at deep),
# and the severity triage (blockers through the `docket-build-task` ladder plus one suite re-run,
# important/minor to the PR body, no re-review round). Step 7 gained the marker-bounded evidence
# block that `docket-finalize-change` reads. Every one of those is a rule another agent executes
# verbatim, so none of it compresses into a pointer. Set per the rounding rule above from the
# measured actual: 3839 words -> the next multiple of 50 is 3850, within the 25-word threshold, so
# the multiple after: 3900 (61 words of margin). The LINE budget was not raised (143 actual, 147
# budget) — the new prose is four paragraphs inside the existing 147-line ceiling.
# docket-finalize-change/SKILL.md's WORD budget was raised 4200 -> 4350 by change 0170, which gave
# the rebase-retest gate a conditional skip of its post-rebase LOCAL suite run. The growth is one
# numbered flow item, and it is all predicate: the skip is admissible only on the conjunction of a
# no-op rebase, a parseable `docket:build-evidence` block with `result: green`, and a `head_sha`
# equal to the branch HEAD being merged — plus the fail-toward-running posture, the audit line, and
# the scoping that leaves `ci`, `both`'s CI leg, and `off` alone. A shortened version would be a
# predicate an agent has to guess at, and guessing wrong here merges an untested branch onto the
# integration branch. Set per the rounding rule above from the measured actual: 4285 words -> the
# next multiple of 50 is 4300, within the 25-word threshold, so the multiple after: 4350 (60 words
# of margin). The LINE budget was not raised (191 actual, 193 budget) — the new item is two lines.
# docket-implement-next/SKILL.md's WORD budget was raised 3900 -> 3950 by change 0170's whole-branch
# review fixes, which closed two gaps in Step 6/7 prose that another agent executes verbatim: rung
# selection had no defined input when `skills.build` is not `docket-build` (the SHIPPED default
# emits no build record at all), so it now names `docket-review-standard` as the no-record rung; and
# Step 7's evidence block now says that a step-6.5 results commit — like any post-gate commit —
# moves HEAD past the minted `head_sha`, so staleness there is expected and finalize simply runs the
# suite. Both are rules, not commentary: an unstated default is a rule with no input, and an
# unexplained staleness reads as a defect to whoever meets it next. Set per the rounding rule above
# from the measured actual: 3923 words -> 3950 (27 words of margin, just above the 25-word
# threshold). The LINE budget was not raised (143 actual, 147 budget) — every edit reflowed inside
# an existing line.
# skills/docket-finalize-change/references/gate-failure.md is a NEW row added by change 0201's
# progressive-disclosure extraction: the merge-gate failure flows (the two-agent split, repair
# sign-off, the abort-and-report set + surfacing channels, and the `## Finalize blocked` write
# shape + lifecycle mechanics) moved behind blocking pointers at their trigger moments in the
# parent SKILL.md. Set per the rounding rule above from the measured actuals: 31 lines -> the next
# multiple of 5 is 35 (4 lines of margin — the same half-step proportion the 0167 line-margin
# reading accepts), 852 words -> 900 (48 words of margin, above the within-25 threshold).
# skills/docket-implement-next/references/edge-paths.md is a NEW row added by change 0201's
# progressive-disclosure extraction: the implementer's rare edges (reconcile-kill caller notes,
# the resume-safety rules, and Step 7's PR-body assembly mechanics) moved behind blocking
# pointers at their trigger moments. Set per the rounding rule above from the measured actuals:
# 28 lines -> the next multiple of 5 is 30, which leaves 2 lines of margin — the near-zero mode
# this block warns about — so the multiple after was taken: 35; 389 words -> 400 is within the
# 25-word threshold (11 words of margin), so the multiple after: 450.
# skills/docket-convention/references/auto-capture.md is a NEW row added by change 0201's
# progressive-disclosure extraction: the auto-capture shared definition's mechanics (the
# classify -> admit -> suppress sequence, the materiality bar, and the deterministic mint-stub
# invocation with its exit codes and count carry-forward) moved behind a blocking read trigger,
# with a summary + the 0127-pinned tokens kept inline (mirroring the learnings.md precedent).
# Set per the rounding rule above from the measured actuals: 38 lines -> the next multiple of 5
# is 40, which leaves 2 lines of margin — the near-zero mode — so the multiple after: 45;
# 376 words -> 400 is within the 25-word threshold (24 words of margin), so the multiple
# after: 450.
# skills/docket-convention/SKILL.md's WORD budget was raised 6350 -> 6400 by change 0194, which
# added the *Skill layer*'s role-self-description bullet: a role skill body names its skills.<role>
# binding key, never whether that binding is the shipped default. The bullet is the single home of
# a rule change 0193 proved is needed — that flip had to sweep eight files because the default was
# restated in each, and two role skill bodies still carried it. Stating it here is what stops the
# ninth and tenth accumulating, so the words are load-bearing rather than commentary. Set per the
# rounding rule above from the measured actual: 6349 words -> the next multiple of 50 is 6350,
# which leaves 1 word of margin (far under the 25-word threshold), so the multiple after was taken:
# 6400 (51 words of margin). The LINE budget was NOT raised (363 actual, 365 budget).
# Change 0201 RATCHETED the Big-4 rows DOWN to post-slim actuals (the first downward move since
# 0085) after its three progressive-disclosure extractions + in-place tightening:
# docket-convention/SKILL.md 365/6400 -> 345/5800 (measured 339/5773; 340 lines would leave 1 —
# near-zero — so 345; 5800 leaves 27 words, above the 25 threshold), docket-finalize-change
# 193/4350 -> 180/3450 (measured 174/3395; 175 leaves 1 line so 180; 3400 leaves 5 words so 3450),
# docket-implement-next 147/3950 -> 145/3700 (measured 139/3654; 140 leaves 1 line so 145; 3700
# leaves 46 words), docket-build 270/2450 -> 265/2400 (measured 260/2348; 260 is zero line margin
# so 265; 2350 leaves 2 words so 2400). The three new reference rows were added by their creating
# commits (gate-failure.md 35/900, edge-paths.md 35/450, auto-capture.md 45/450).
# skills/docket-implement-next/SKILL.md's WORD budget was raised 3700 -> 3800 by change 0113, whose
# two riders split the §5 fused proceed/stay-silent sentence into separately-stated obligations and
# densified the claimed_at heartbeat from two phase boundaries to every metadata commit. Both are
# additions to prose that two observed runs demonstrably misread; the words buy the disambiguation.
# The references/ file considered and rejected is skills/docket-implement-next/references/edge-paths.md
# (0201's rare-edges extraction): neither rider can live there. Both are rules that must fire on the
# COMMON path at the exact moment of action — the §5 rider governs every build invocation as the
# agent decides whether a suppressed hand-off ends the step, and the heartbeat rider governs every
# metadata commit the skill makes. edge-paths.md is read only when a rare edge is already known to
# have been hit, so a rule parked there is unread precisely when it must intervene; that is the same
# argument the 0137 dispatch-capability entry above records. Set per the rounding rule above from
# the re-measured MERGED file (0201's slim plus 0113's riders): 3728 words -> the next multiple of
# 50 is 3750, which leaves 22 words (within the 25-word threshold), so the multiple after: 3800
# (72 words of margin). The LINE budget was NOT raised — the riders reflowed inside existing lines
# (139 actual, 145 budget, which is 0201's ratcheted value). Neither pre-rebase number survives:
# 0113's 4050 was measured against the pre-slim file, and 0201's 3700 predates these riders.
# skills/docket-build/SKILL.md's budget was raised 265/2400 -> 270/2450 by change 0212, which added the
# mode-conditioned scoping clause beside the file's terminal stop. skills/docket-build/ has NO
# references/ tree, so change 0201's rule cannot be discharged by naming an existing file: the home
# that would have to be created is skills/docket-build/references/. Creating it is wrong here — the
# clause must fire at the exact moment a reader reads "Then you stop", and a rule sitting in an
# unread reference file cannot intervene at that moment. That is the same argument change 0137
# recorded for the convention's dispatch rule. The H1 paragraph was compressed first; set from the
# measured actual: 267 lines -> 270, 2421 words -> 2450.
# Extended by change 0212: 270/2450 -> 280/2500, for the SECOND scoping clause the same change owes —
# at ## Halting conditions, whose `halted` token collides with docket-implement-next's run
# disposition of the same name. The references/ argument above applies unchanged and with more force:
# this clause must fire as the reader decides whether `halted` ends the run. Set from the measured
# actual: 2460 words -> 2500 (40 words of margin, past the 25-word threshold); 274 lines -> the next
# multiple of 5 is 275, which leaves ONE line of margin — the near-zero headroom this table's header
# forbids — so the multiple after: 280.
# skills/docket-review/SKILL.md's budget was raised 105/800 -> 105/900 by change 0212, which scoped
# the file's ## Conduct prohibitions and its ## Halting stop to the review role. skills/docket-review/
# has NO references/ tree; the home that would have to be created is
# skills/docket-review/references/, and creating it is wrong for the same reason as docket-build's
# entry above — a prohibition-scoping rule must be read in the same breath as the prohibition it
# scopes. Set from the measured actual: 838 words -> 900 (850 lands within 25 words of the actual,
# so the next multiple is taken). The LINE budget was then raised 105 -> 110 in a fix round: the
# file measures 104 lines, so 105 left ONE line of headroom — the near-zero margin this block
# already records raising for twice (docket-build-task 100 -> 105, docket-build 155 -> 160), on a
# file this change edited twice.
# skills/docket-build-task/SKILL.md's budget was raised 115/1000 -> 125/1100 by change 0212, which
# scoped the worker contract's ## Scope prohibitions and its ## Outcomes return to the worker role.
# The body reaches a caller's context by wrapper preload (agents/docket-build-*.md carry
# skills: [docket-build-task]), so the hazard is real for this file. skills/docket-build-task/ has NO
# references/ tree; a created one could not intervene at the moment the return instruction is read.
# ## Scope's worktree bullet was compressed first. The derivation was re-measured in a fix round —
# later commits on this branch moved the file after the original comment was written — and set from
# the CURRENT measured actual: 119 lines -> 120 leaves ONE line of headroom, the near-zero margin
# this block forbids, so the multiple after (125); 1051 words -> 1100, unchanged.
# skills/docket-implement-next/SKILL.md's budget was raised 145/3800 -> 150/3850 by change 0212,
# which added the run-disposition obligation the agent must discharge when it decides the run is
# over. This change grew the file from 139/3728 (the actual the 0113 entry above records) to
# EXACTLY its 145/3800 budget on both axes — zero headroom — so the raise is consumed by this diff
# rather than prophylactic. That is the failure mode this comment block's 0102 and 0137 entries record: one
# word of margin means the next edit to the file reddens CI on arrival, so the raise is taken now
# rather than deferred. The considered home was skills/docket-implement-next/references/edge-paths.md,
# and the obligation cannot live there: edge-paths.md is read CONDITIONALLY, only when the run hits
# one of its named edges, whereas the closing obligation must already be in context on EVERY run at
# the moment the agent decides it is finished. The run that ends early is exactly the run that never
# reaches a conditional read, so a reference-file home would be absent precisely when it is needed.
# Set from the measured actual per the rule above: 145 lines -> 150, 3800 words -> 3850.
# skills/docket-build/references/task-routing.md is a NEW row added by change 0218, which extracted
# docket-build's `## Routing` rubric to a shared reference so docket-implement-next's Step 6 fix loop
# could classify a finding from the same source rather than restate it. The justification for the
# extraction is SHARED CONSUMPTION, not section weight — the file has two owners, and a restated
# rubric is the documented drift class (restatement-accumulates-its-own-guards). Set per the
# rounding rule above from the measured actual: 46 lines -> 50 (4 lines of margin, the same
# half-step proportion the 0167 line-margin reading and the gate-failure.md row accept), 464 words
# -> 500 (36 words of margin, above the within-25 threshold).
# skills/docket-build/SKILL.md's budget was NOT lowered by the extraction: a budget is a ceiling,
# and lowering it to the new actual would redden the next unrelated edit for no invariant.
# skills/docket-implement-next/references/fix-loop.md is a NEW row added by change 0218, which gave
# Step 6 a bounded in-branch fix loop for review findings. The mechanics are heavy AND conditionally
# read — a review that returns no findings never needs them — which is the skill-extraction-and-stub-
# pointer test, and the same shape as this skill's existing edge-paths.md and the convention's
# auto-capture.md. Step 6 keeps the RULE (fix in-branch, character-routed, never max, threshold knob)
# and the blocking pointer; the reference keeps rule + why. Set per the rounding rule above from the
# measured actual: 122 lines -> 125 (3 lines of margin, the accepted half-step proportion),
# 1053 words -> 1100 (47 words of margin, above the within-25 threshold). SKILL.md's own row was
# NOT raised: the rewritten triage paragraph measures 145/3840, inside the existing 150/3850.
# fix-loop.md's row was raised 125/1100 -> 140/1250 by a 0218 review fix: the blocker floor (a
# blocker's fix starts no lower than standard, the one exception to character/severity
# orthogonality) must live beside the routing table it excepts — this file IS the routing rule's
# home, and stating an exception anywhere else (the considered home was SKILL.md's Step 6 summary,
# which keeps only the rule + pointer) would leave the table it modifies contradicting it. Set per
# the rounding rule above from the measured actual: 135 lines -> 135 is the next multiple of 5 but
# leaves ZERO margin, so 140; 1215 words -> 1250 (35 words of margin, above the within-25 threshold).
# fix-loop.md's row was raised again, 140/1250 -> 150/1400, by a later 0218 review fix: the suite
# gate's revert step named no fix-task ORDER and no posture for a conflicted revert, and the whole
# "the branch can never end worse than the green build that entered it" guarantee rests on that
# revert succeeding. The ordering rule (blockers first, so non-blocker commits are the branch's
# tail) belongs in `## Tasks, batching, commits` and the conflict posture in the gate step that can
# hit it — both in THIS file, which owns the loop's mechanics end to end. The considered home was
# skills/docket-implement-next/SKILL.md's Step 6 summary, which deliberately keeps only the rule
# plus the blocking pointer; putting a dispatch-ordering constraint there would separate it from
# the revert whose safety it buys, and the revert step would then have to restate it — the drift
# class the task-routing.md entry above already records. Set per the rounding rule above from the
# measured actual: 144 lines -> 145 is the next multiple of 5 but leaves ONE line of margin, the
# near-zero failure mode this block records raising for three times already, so 150; 1353 words ->
# 1400 (47 words of margin, above the within-25 threshold).
# fix-loop.md's row was raised again, 150/1400 -> 160/1550, by a later 0218 review fix: the loop
# bounded escalations and suite runs but not the NUMBER of fix tasks, so a ten-plus-finding review
# could expand Step 6 without limit. The cap (at most five non-blocker fix tasks, blockers never
# counted, overflow deferred deterministically) belongs in `## Tasks, batching, commits` — this
# file owns the loop's task mechanics end to end, per the same considered-home reasoning as the
# two raises above. Set per the rounding rule above from the measured actual: 155 lines -> 155 is
# a multiple of 5 but leaves ZERO margin, so 160; 1509 words -> 1550 (41 words of margin, above
# the within-25 threshold).
# fix-loop.md's row was raised again, 160/1550 -> 175/1800, by a later 0218 review fix wave: three
# gaps in the loop's own mechanics. (a) The disposition table enumerated no state for a finding that
# took the surviving narrow mint path, so the "complete per-finding accounting" claim was false — a
# `minted` row plus the sentence that makes the claim explicit. (b) The `unverified-build-state`
# self re-run was never reconciled with the gate's two-run bound, and the first could make the second
# false — the re-run is now stated as outside the bound, with Step 6's real ceiling of three named.
# (c) The loop named no posture for unavailable profile dispatch, a case docket-build states for its
# own dispatches — now a tier plus a pointer at the convention's rule (pointed at, never restated).
# All three belong in THIS file for the same considered-home reasoning as the three raises above:
# the considered home was skills/docket-implement-next/SKILL.md's Step 6 summary, which deliberately
# keeps only the rule plus the blocking pointer, and each of these three qualifies a mechanism —
# the table, the gate bound, the dispatch — that exists only here; stating a qualifier apart from
# the mechanism it qualifies leaves the mechanism reading as unqualified, the restatement drift class
# the task-routing.md entry above records. Set per the rounding rule above from the measured actual:
# 172 lines -> 175 (3 lines of margin, the accepted half-step proportion this block already took at
# 122 -> 125); 1736 words -> 1750 is the next multiple of 50 but leaves 14 words (within the 25-word
# threshold), so the multiple after it, 1800.
# skills/docket-build/references/task-routing.md's row was NOT raised by the same wave: qualifying
# the `standard` bullet's override clause (which asserted a consumer capability the fix loop does not
# have) measures 48/485, inside the existing 50/500.
# skills/docket-convention/references/auto-capture.md's budget was raised 45/450 -> 50/550 by change
# 0218, which narrowed the materiality bar: work fixable by a small in-branch edit now FAILS the bar,
# so a review finding about the branch's own diff is never mintable. The prose has no other home —
# this file IS the bar's definition, and a rule about what the bar admits cannot live in a sibling
# reference (the considered home was skills/docket-implement-next/references/fix-loop.md, which is
# read only by the implementer's Step 6; the bar is applied by every auto-capture site, including
# docket-status's sweep, which never reads the implementer's references). Set per the rounding rule
# above from the measured actual: 45 lines -> 45 is the next multiple of 5 but leaves ZERO margin,
# so the multiple after it, 50; 478 words -> 500 would leave 22 (within the 25-word threshold), so
# 550.
# That same row was raised again, 50/550 -> 55/600, by a 0218 review fix: the narrowed bar was stated
# UNSCOPED in a reference shared by mint sites that have no branch and no fix loop (the
# docket-finalize-change / docket-status harvest), so "fixable by a small in-branch edit fails the
# bar" told exactly those sites to drop the follow-up nothing else will ever pick up. The scoping
# paragraph — which sites the clause binds at, and that the harvest is exempt because neither a
# branch nor a fix loop exists there — has no other home: this file IS the bar's definition and the
# one artifact every mint site reads before minting, and the considered alternative,
# skills/docket-implement-next/references/fix-loop.md, is read ONLY by the caller the clause already
# binds — putting the exemption there leaves the harvest reader with the unscoped rule and nothing
# to correct it. Set per the rounding rule above from the measured actual: 51 lines -> 55; 544 words
# -> 550 would leave 6 (within the 25-word threshold), so 600.
# skills/docket-implement-next/results-template.md's budget was raised 24/172 -> 25/250 by change
# 0218, which narrowed `## Verify (human)` to genuinely manual checks and said where a fixed
# finding's outcome IS read instead (the PR body's disposition table). The prose is a template
# comment: it must sit in the template itself, at the moment an author is filling the section in —
# the considered home, skills/docket-implement-next/references/edge-paths.md, is read only when a
# run hits a rare edge, and writing a results file is not one. Set per the rounding rule above from
# the measured actual: 23 lines -> 25 (the next multiple of 5; the pre-existing 24 would leave the
# 1-line margin this comment block twice records as the near-zero failure mode), 191 words -> 200
# would leave 9 (within the 25-word threshold), so 250.
# skills/docket-implement-next/references/edge-paths.md's row was NOT raised by change 0218: turning
# the PR-body findings clause into the disposition-table pointer measures 28/411, inside 35/450.
# skills/docket-convention/SKILL.md's WORD budget was raised 5800 -> 5850 by a later 0218 review fix:
# the human decided `skills.build: auto` keeps authorizing the Step 6 in-branch fix workers rather
# than growing a `skills.fix` twin, so the BORROWING has to be stated where the authorization is
# defined — the *Dispatch-capability resolution* Tier C row, whose dispatch cell enumerates that
# tier's consumers. A config reader who meets one knob named for one role, silently authorizing two
# kinds of inline work, is the surprise the words buy off. The considered home,
# skills/docket-convention/references/agent-layer.md, is read only when configuring `agents:` /
# running sync-agents.sh — never at the moment an agent is deciding whether an inline fix is
# authorized, which is precisely when the Tier C row fires (the same reasoning that put the tier
# table in SKILL.md at change 0137, above). The three satellite sites (README, .docket.example.yml,
# fix-loop.md) carry short pointers stating only what THAT reader needs, so the rule is not restated.
# Set per the rounding rule above from the measured actual: 5804 words -> 5850 (46 words of margin,
# above the within-25 threshold). The LINE budget was NOT raised (339 actual, 345 budget).
# skills/docket-implement-next/references/fix-loop.md's row was raised 175/1800 -> 180/1850 by that
# same fix: its Tier C paragraph already cited `skills.build: auto`, but incidentally — the sentence
# that makes the borrowing deliberate, and answers why no `skills.fix` knob exists (a fix worker runs
# the `docket-build-task` contract at `docket-build`'s own profiles), belongs beside the citation it
# qualifies. The considered home is the convention's Tier C row, which now owns the RULE; the
# "why is there no knob" question is asked by a reader of this file, at this paragraph, and an
# answer parked one file away leaves the citation reading as an accident here. The paragraph was also
# re-wrapped so `*Dispatch-capability resolution*` sits on ONE line: the check_site guard in
# tests/test_dispatch_capability.sh greps the citation as a literal, and a line-wrapped phrase is
# unfindable to it (AGENTS.md: a cross-reference anchors on a greppable verbatim clause). Set per the
# rounding rule above from the measured actual: 175 lines is a multiple of 5 but leaves ZERO margin,
# so the multiple after it, 180; 1779 words -> 1800 leaves 21 (within the 25-word threshold), so 1850.
# skills/docket-implement-next/SKILL.md's budget was raised 150/3850 -> 165/4250 by change 0203,
# which gave the file's orphan term `git-state postcondition` a referent: one `### Step
# postconditions` table stating, for each of Steps 2-7, the git condition that completes it (refs,
# commits, frontmatter fields, and the committed build-evidence record), prefaced by the governing
# sentence that these certify a STEP and never the run — only Step 7's postcondition also completes
# the run, so a satisfied intermediate row is never licence to stop. Change 0113 added the clause
# "the step is not complete until its git-state postcondition holds" and defined the term nowhere;
# the 0206 run stopped at a satisfied Step-5 condition, which is the failure the governing sentence
# is aimed at. COMPRESSION WAS TAKEN FIRST, per this block's compress-then-raise posture: the
# *Terminal disposition* pointer sentence ("that postcondition is Step 7's to state, not this
# section's") became "stated in *Step postconditions* above, not here" — but that recovered only
# 2 words and 0 lines, so essentially the whole raise is the new section. Deeper compression was
# rejected on inventory: eleven test files grep this SKILL.md's prose, and
# test_board_refresh_on_transition.sh, test_learnings_ledger.sh, test_closeout.sh and
# test_results_artifact.sh assert on sentences a cut would have taken (restatement-accumulates-its-
# own-guards; size-target-is-direction — take the raise rather than cut prose another guard holds).
# The considered home is skills/docket-implement-next/references/edge-paths.md, and the table cannot
# live there: edge-paths.md is read CONDITIONALLY, only once a run already knows it hit one of its
# named edges, whereas a postcondition table is read on the COMMON path at EVERY step boundary — the
# agent consults it precisely while deciding whether an ordinary step is finished, which is never an
# edge. A rule parked there is unread exactly when it must intervene; that is the same argument the
# 0113 and 0212 entries above record for this same file and reference pair.
# Set per the rounding rule above from the measured actuals: pre-edit 145/3844, post-edit 160/4186.
# 160 lines is itself a multiple of 5 and so leaves ZERO margin, the near-zero failure mode this
# block records raising for repeatedly, so the multiple after it: 165. 4186 words -> the next
# multiple of 50 is 4200, which leaves 14 words (within the 25-word threshold), so the multiple
# after it: 4250 (64 words of margin).
# Change 0203 REVIEW FIX, same file, word budget 4250 -> 4300 (lines unchanged at 165): the governing
# sentence claimed every row is read "from git, never from a sub-skill's report", which is false for
# rows 5-6 — docket-build EMITS the build-evidence record as output, its default BUILD_CHECKPOINT:
# false persists nothing, and the true-path ledger lives under the gitignored `.superpowers/`. The
# section now names that one exception and pins head_sha == HEAD as the record's only git fact.
# Compression was taken first and paid most of it: Step 5's parenthetical "(the conjunct that makes a
# sub-skill's report git-checkable)" was DELETED, the header now saying it once and for both rows.
# No references/ home applies — this is a qualifier on the governing sentence of a table that the
# rejection above already argued cannot leave SKILL.md; splitting a rule from its own exception is
# strictly worse than either whole. Set per the rounding rule from the measured actual 160/4243:
# 4243 -> 4250 leaves 7 words (within the 25-word threshold), so the multiple after it, 4300.
# skills/docket-build/references/gate-execution.md is a NEW row added by change 0223, which states
# the build gate's EXECUTION POSTURE by capability in docket-build's SKILL.md and quarantines every
# product-specific name, setting, and measured figure here. The quarantine is the point, not a size
# argument: the parent body is bound by a harness-neutrality rule the suite asserts negatively, so
# per-harness verdicts, launch shapes, and observed durations have no other home by construction —
# naming a tool in SKILL.md is the defect this file exists to prevent. It is also read on a
# different cadence (blocking, once, at gate start) and re-measured whenever a harness version
# moves, which SKILL.md is not. Set per the rounding rule above from the measured actuals: 143
# lines -> the next multiple of 5 is 145, which leaves 2 lines of margin — the near-zero mode this
# block warns about, and the same reading the edge-paths.md row took — so the multiple after: 150;
# 1290 words -> 1300 is within the 25-word threshold (10 words of margin), so the multiple after:
# 1350. This row is a build-time consequence of creating the file, not a discretionary raise: the
# completeness check below rejects any skills/**/*.md without one.
BUDGETS="
skills/docket-adr/SKILL.md                                  86 1408
skills/docket-adr/adr-template.md                           26   90
skills/docket-auto-groom/SKILL.md                           66 1237
skills/docket-brainstorm/SKILL.md                           84  692
skills/docket-build/SKILL.md                               280 2500
skills/docket-build/references/gate-execution.md            150 1350
skills/docket-build/references/task-routing.md              50  500
skills/docket-build-task/SKILL.md                          125 1100
skills/docket-convention/SKILL.md                          345 5850
skills/docket-convention/github-board-mirror.md             19  462
skills/docket-convention/references/agent-layer.md         190 2150
skills/docket-convention/references/auto-capture.md         55  600
skills/docket-convention/references/learnings.md            84  580
skills/docket-convention/references/terminal-close-out.md  173 1458
skills/docket-finalize-change/SKILL.md                     180 3450
skills/docket-finalize-change/references/gate-failure.md    35  900
skills/docket-groom-next/SKILL.md                           77 1484
skills/docket-implement-next/SKILL.md                      165 4300
skills/docket-implement-next/references/edge-paths.md       35  450
skills/docket-implement-next/references/fix-loop.md        180 1850
skills/docket-implement-next/results-template.md            25  250
skills/docket-review/SKILL.md                              110  900
skills/docket-new-change/SKILL.md                           61 1330
skills/docket-new-change/change-template.md                 51  203
skills/docket-status/SKILL.md                              118 2393
"

# Every tracked file is within budget.
budgeted=""
while read -r rel maxL maxW; do
  [ -n "$rel" ] || continue
  budgeted="$budgeted $rel"
  f="$REPO/$rel"
  assert "budgeted file exists: $rel" '[ -f "$f" ]'
  [ -f "$f" ] || continue
  L=$(wc -l < "$f" | tr -d ' '); W=$(wc -w < "$f" | tr -d ' ')
  assert "$rel within line budget ($L <= $maxL)" '[ "$L" -le "$maxL" ]'
  assert "$rel within word budget ($W <= $maxW)" '[ "$W" -le "$maxW" ]'
done <<EOF
$BUDGETS
EOF

# Completeness (auto-discovery guard, finding #12): every skills/**/*.md has a budget row, so a
# newly-added skill file can never go silently un-budgeted.
missing=""
while IFS= read -r f; do
  rel="${f#"$REPO"/}"
  printf '%s' "$budgeted" | grep -qF -- " $rel" || missing="$missing $rel"
done < <(find "$REPO/skills" -name '*.md' | sort)
assert "every skills/**/*.md has a budget row (unbudgeted:[$missing])" '[ -z "$missing" ]'

# Non-vacuity / mutation proof: the guard actually bites. A synthetic file 1 line over a 1-line
# budget must be caught by the same comparison.
probe="$(mktemp)"; printf 'a\nb\n' > "$probe"
pL=$(wc -l < "$probe" | tr -d ' ')
assert "the line-budget comparison is non-vacuous (2 > 1 is caught)" '[ ! "$pL" -le 1 ]'
rm -f "$probe"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
