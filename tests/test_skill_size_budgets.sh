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
# skills/docket-convention/references/auto-capture.md's budget was raised 55/600 -> 125/1200 by
# change 0226, which reframed the file from a suppression rule into a capability-discovery pipeline:
# it ADDS the positive half the file never had — six discovery categories to search for, six
# admission gates the discovery must clear, the five capture fields a minted body carries, and a
# per-site routing table — on top of every suppression rule, which is carried forward unweakened.
# The prose has no other home. This file IS the definition every mint site reads before minting, and
# each added part is a rule applied at that same moment: the categories are what the reader searches
# with, the gates are what admits, the fields are what the body must contain to survive
# mint-stub.sh's `## Why` contract, and the routing table is what tells a site whether fix-in-branch
# even exists for it. The considered home was skills/docket-implement-next/references/fix-loop.md
# (already the considered-and-rejected home for the 0218 raises above): it is read ONLY by the
# implementer's Step 6, so the finalize/status harvest — a mint site that never reads the
# implementer's references — would get the reframe's gates and none of its site-C carve-out. The
# convention's SKILL.md summary was considered and rejected under progressive disclosure: it is
# loaded on every skill invocation, and the detail here is read only when a discovery is in hand.
# Set per the rounding rule above from the measured actual: 119 lines -> 120 is the next multiple of
# 5 but leaves ONE line of margin, the same near-zero headroom the docket-build-task row above was
# corrected for in a fix round, so the multiple after it, 125; 1142 words -> 1150 would leave 8
# (within the 25-word threshold), so 1200.
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
# 1296 words -> 1300 is within the 25-word threshold (4 words of margin), so the multiple after:
# 1350. This row is a build-time consequence of creating the file, not a discretionary raise: the
# completeness check below rejects any skills/**/*.md without one. (The word actual read `1290` as
# first recorded — a mis-measurement, corrected here by change 0223's second review wave against
# `wc -w`. The conclusion is unchanged: 1290 and 1296 both land inside the 25-word threshold and
# both round on to 1350. Recorded rather than silently overwritten, because a justification block
# whose stated basis does not reproduce is the thing that makes the next raise unauditable.)
# skills/docket-build/SKILL.md's budget was raised 280/2500 -> 305/2750 by that same change 0223,
# which added the `### Gate execution posture` subsection to `## The build gate` plus one
# `## Halting conditions` bullet for the exhausted observation budget. WHERE ELSE IT WAS CONSIDERED,
# per the naming requirement above: skills/docket-build/references/gate-execution.md, created by
# this same change one commit earlier. The posture cannot live there, for two reasons that are not
# size arguments. (a) That file is the QUARANTINE for product-specific evidence — per-harness launch
# shapes, versions, measured durations — and it is re-measured and rewritten whenever a harness
# version moves; parking the contract inside it would make docket's own rule editable as a side
# effect of refreshing evidence, and what it enumerates is what a HARNESS must provide, not what
# this role must do. (b) The plan's next task has skills/docket-finalize-change/SKILL.md cite this
# posture by name as `docket-build`'s, and a cross-skill citation must anchor in the owning skill
# body — a citation pointing into a reference file names no owner. The halting bullet is fixed in
# place for a mechanical reason on top of that: tests/test_docket_build.sh enumerates the halts
# inside a `## Halting conditions` section slice precisely so a halt stated anywhere else does not
# count. COMPRESSION WAS TAKEN FIRST on the addition itself, per this block's posture: the drafted
# standalone "the observation interval is an implementation detail" paragraph was folded into
# clause 5, and the false-completion rule's provenance sentence became a one-clause pointer at this
# file's own `## Reading a worker's return` section rather than a restatement of it. Pre-existing
# prose is untouched by the diff. Set per the rounding rule above from the measured actuals: 299
# lines -> the next multiple of 5 is 300, which leaves ONE line of margin — the near-zero failure
# mode this block records raising for repeatedly — so the multiple after it: 305. 2689 words -> the
# next multiple of 50 is 2700, which leaves 11 words (within the 25-word threshold), so the multiple
# after it: 2750.
# skills/docket-build/SKILL.md's budget was raised again 305/2750 -> 315/2900 by change 0223's
# review-fix wave, which SCOPED the posture's yield permission to the observing agent's own dispatch
# posture: clause 4 now grants the yield only to a top-level session agent able to receive a
# resumption signal, and requires bounded BLOCKING observation from a dispatched or forked build
# role; the "does not relax" paragraph states the same scoping, because that is where a reader goes
# to resolve the apparent ADR-0024 conflict. The unqualified permission it replaces was wrong on
# docket's DEFAULT path rather than an exotic one — this role is invoked inside docket-implement-next
# Step 5, and that role is itself dispatched — and wrong empirically: dispatched build workers on
# this very change yielded to await a gate completion event and none was resumed by it.
# WHERE ELSE IT WAS CONSIDERED, per the naming requirement above:
# skills/docket-build/references/gate-execution.md. It cannot live there, for the two reasons the
# rows above record, both sharpened here. (a) That file is the per-HARNESS quarantine — what a
# harness must provide, re-measured whenever a harness version moves — whereas this states what THIS
# ROLE must do given its own dispatch posture: a fact about docket's own composition that no harness
# measurement can change. Parking it there would make an ADR-0024 boundary condition editable as a
# side effect of refreshing evidence. (b) It is a QUALIFIER on clause 4 and on the paragraph that
# resolves the conflict; splitting a rule from its own scope leaves the unqualified rule standing at
# the point of action, which is precisely the defect being fixed. COMPRESSION WAS TAKEN FIRST on the
# addition: clause 4's "each returning on its own" was deleted (subsumed by "short foreground
# reads"), and the empirical sentence dropped its restatement of the half-done/`completed`
# consequence, which the false-completion rule immediately above already states. Pre-existing prose
# is otherwise untouched. Set per the rounding rule above from the measured actuals: 308 lines -> the
# next multiple of 5 is 310, which leaves 2 lines — the near-zero mode this block warns about, and
# the same reading the gate-execution.md row took — so the multiple after: 315. 2831 words -> the
# next multiple of 50 is 2850, which leaves 19 words (within the 25-word threshold), so the multiple
# after it: 2900.
# skills/docket-build/SKILL.md's budget was raised a third time 315/2900 -> 320/2950 by change
# 0223's third review wave, which pulled the `0` budget's SEMANTICS onto the executing contract.
# `.docket.example.yml` (mirrored in scripts/docket-config.sh) already said "0 is legal and means
# observe once, then fail closed", but the posture said only that an EXHAUSTED budget fails closed —
# and a 0-minute budget is exhausted before any observation, so the contract the agent runs delivered
# zero observations where its own config promises one. Clause 5 now states the reading and clause 6
# names where the verdict lands under it.
# WHERE ELSE IT WAS CONSIDERED, per the naming requirement above:
# skills/docket-build/references/gate-execution.md, and the config surfaces it was left on. Neither
# can host it. (a) The reference is the per-HARNESS quarantine, re-measured when a harness version
# moves; a boundary reading of docket's own POLICY value is not a harness fact and must not become
# editable as a side effect of refreshing evidence. (b) The config surfaces are where the value is
# declared, not where it is spent — that is the whole finding: a rule stated only on a surface the
# executing agent does not read is a rule the agent does not execute, and this one must intervene at
# the moment the budget is checked. Set per the rounding rule above from the measured actuals: 312
# lines -> the next multiple of 5 is 315, which leaves 3 lines — the near-zero mode this block warns
# about — so the multiple after: 320. 2876 words -> the next multiple of 50 is 2900, which leaves 24
# words (within the 25-word threshold), so the multiple after it: 2950.
# skills/docket-build/references/gate-execution.md's budget was raised 150/1350 -> 175/1650 by change
# 0223's second review wave, which stopped each `supported` verdict claiming more than the probe
# beneath it measured. Two coupled defects: § *Method* establishes only capabilities 1-3 (survival,
# redirection, terminal sentinel) while the token was defined against all six — capability 5's
# four-state distinction is never produced at all, since the stand-in gate always succeeds, and
# capability 6 was never probed; and the `claude` row called its measured mode "the boundary the
# posture is about" when that mode is an INTERACTIVE session, which is precisely the mode docket does
# not run in (this gate runs inside docket-build, invoked inline by the forked docket-implement-next,
# and the section itself records that the non-interactive variant was unobtainable on this machine).
# WHERE ELSE IT WAS CONSIDERED, per the naming requirement above: the parent
# skills/docket-build/SKILL.md, and a further sub-reference under skills/docket-build/references/.
# Neither can host it. (a) SKILL.md is bound by the harness-neutrality rule the suite asserts
# NEGATIVELY — a per-row mode scope cannot be stated without naming the harness whose row it scopes,
# which is the exact defect this quarantine file exists to prevent. (b) A sub-reference splits a
# verdict from the bound on what that verdict means; the whole finding is that a reader takes the
# token at face value, and a qualifier one file further away is read even less often than one three
# paragraphs up — which is why the per-row limits sit ON the verdict line and the guard now enforces
# that shape. COMPRESSION WAS TAKEN FIRST, and it also chose between the two available fixes: rather
# than a coverage line per harness section (four repetitions of one bound), the bound is stated ONCE
# in § *Reading a verdict* where the token is defined, and only the two rows narrower than it — the
# mode scope on `claude`, the launch-shape scope on `opencode` — say anything further. The `claude`
# paragraph's closing "The capability is measured; the outside-observer variant is not." was DELETED
# as subsumed by that shared bound. Set per the rounding rule above from the measured actuals: 168
# lines -> the next multiple of 5 is 170, which leaves 2 lines — the near-zero mode this block warns
# about, and the same reading both rows above took — so the multiple after: 175. 1612 words -> 1650
# leaves 38 words, clear of the 25-word threshold, so 1650 stands.
# skills/docket-finalize-change/SKILL.md's WORD budget was raised 3450 -> 3500 by change 0223, which
# added the gate-execution-posture citation to item 5's `local` leg: finalize's own suite run obeys
# the posture `docket-build` owns, including the `GATE_OBSERVATION_BUDGET` bound on observing it.
# The prose is already the minimum form — a CITATION, not a restatement (it is the deliberate mirror
# of build citing this file's `configured-bash-finalize` block for the suite command), so the
# considered home question resolves the other way from the usual: the rule's home IS
# skills/docket-build/references/gate-execution.md, and only the pointer lives here. A pointer
# cannot live in a reference file, because it must be read at the moment the agent is about to run
# the gate. Every other row this change touched was re-set and this one was not: the file measures
# 3437 words against 3450, thirteen words of headroom, which is the near-zero failure mode this
# block's 0102 and 0137 entries exist to forbid. Set per the rounding rule above from the measured
# actual: 3437 words -> the next multiple of 50 is 3450, within the 25-word threshold, so the
# multiple after: 3500 (63 words of margin). The LINE budget was NOT raised (174 actual, 180 budget).
# skills/docket-convention/SKILL.md's WORD budget was raised 5850 -> 5900 by a change 0226 review fix
# (2026-08-07): the drill-down trigger for the auto-capture reference read "Discovered follow-up work
# mid-run -> read ... now (blocking)", which fires only AFTER something has surfaced — so the
# reference's headline addition, the active "What to look for" discovery pass, was reachable by
# nobody who still needed it. The trigger is now scoped to the mint site itself ("At each mint site —
# on arrival, before anything has surfaced — and again on any discovered follow-up work mid-run").
# The considered home, skills/docket-convention/references/auto-capture.md, CANNOT hold it: it is the
# very file the trigger decides whether to open, so a rule stating when to open it, parked inside it,
# is read only by someone who already opened it — the same unreachability this fix exists to remove.
# The convention is injected into every mint site's wrapper, which is why widening it here (rather
# than rewording the three mint-site skill bodies) is what makes the pass reachable. Set per the
# rounding rule above from the measured actual 341/5853: 5853 -> 5900 (47 words of margin, above the
# within-25 threshold). The LINE budget was NOT raised (341 actual, 345 budget, 4 lines of margin).
# skills/docket-convention/references/auto-capture.md's budget was raised 125/1200 -> 130/1250 by a
# change 0226 review fix (2026-08-07): the `## Admission gates` section stated the six gates as
# universal, with the site-C carve-out standing two sections downstream in *Routing* and no forward
# pointer. Gates 1, 2, 3 and 6 are written against "the active change" / the active branch, neither of
# which exists at the docket-finalize-change / docket-status harvest, so a reader who stops at the
# gates and applies them literally suppresses precisely the cheap-to-fix follow-up the *Materiality
# bar*'s 0218 exemption exists to protect. The section now carries the scoping clause and a pointer to
# *Routing*, and `## Per discovery`'s parenthetical says WHICH bar the site applies rather than naming
# both unconditionally. The prose has no other home for the same reason every raise above records for
# this file: it IS the definition every mint site reads before minting, and an exception parked away
# from the rule it qualifies is unread exactly when it must intervene — that is the failure being
# fixed here, one section's distance already being enough. The considered home,
# skills/docket-implement-next/references/fix-loop.md, is read ONLY by the implementer's Step 6, i.e.
# by sites A and B, the two the unscoped gates already bind correctly; the harvest reader who needs
# the carve-out never opens it. Compression was considered and rejected: every other paragraph in the
# section is held by an assert in tests/test_docket_review.sh's change 0226 block (six numbered gates,
# six gate phrases, four never-mint clauses), so there is nothing to cut that is not guarded. Set per
# the rounding rule above from the measured actual 123/1202 (the `## Per discovery` paragraph was
# re-wrapped, which recovered one line): 125 is the next multiple of 5 but leaves TWO lines of
# margin, the near-zero headroom this block records raising past twice for this very file, so the
# multiple after it, 130; 1200 is already exceeded and 1250 leaves 48 words (above the within-25
# threshold).
# skills/docket-build/references/gate-execution-evidence.md is a NEW row added by change 0234, which
# split skills/docket-build/references/gate-execution.md along the instruction-vs-evidence axis: the
# § *Method* probe design, the one-variable-per-run ladder, the four launch durations, and the
# per-harness measurement narratives moved off a file that is read BLOCKING before every gate run.
# The WHERE-ELSE clause is not required for this row — that rule binds a RAISE only (see the top of
# this block), and this is a new file, the same reading change 0223 recorded for exactly this case.
# Recorded anyway: the two homes considered were 0223's docs/results/ record (rejected — a results
# file is a close-out record of a completed change, while this content must be rewritten whenever a
# harness version moves) and a new ADR (rejected — an Accepted ADR is immutable except its status
# line, the wrong lifecycle for a measurement). Set per the rounding rule above from the measured
# actuals: 100 lines -> 110, 971 words -> 1000. 100 is already a multiple of 5, so rounding up leaves
# ZERO lines of margin — the near-zero-headroom failure mode this block records twice — and 105
# leaves five, which is the same near-zero reading the sibling row below took when it rejected 115;
# hence the multiple after it, 110. 1000 leaves 29 words (above the within-25 threshold). This row is
# expected to be RAISED again on a re-probe or a fifth harness — each per-harness narrative runs
# 8-12 lines — unlike the ratcheted instruction row below, which is frozen instruction and is
# expected to hold.
# That WORD budget was then RE-SET within the same change, 1000 -> 1050. The 1000 above was derived
# from a 971-word actual; an in-branch review fix then added a sentence to the file, taking it to
# 102/999 — one word of headroom, which is precisely the near-zero failure mode this block records
# having been burned by three times (the 0102, 0137, and 0167 entries above): the next word added
# reddens CI on arrival. Re-applying the rounding rule to the ACTUAL 999: the next multiple of 50 is
# 1000, which leaves 1 word (inside the within-25 threshold), so the multiple after it, 1050. The
# LINE budget is untouched — 102 against 110 is eight lines of working margin, not near-zero. No
# where-else clause is owed: the raise buys headroom against a rule this block itself imposes, and
# moves no content.
# The same change RATCHETED skills/docket-build/references/gate-execution.md DOWN, 175/1650 ->
# 120/1000. A lowering needs no where-else clause either (that rule binds a raise), but the
# ratchet is the discretionary half of 0234 and so is argued here: the file's defect was ACCUMULATED
# evidence on a blocking-read surface, and leaving ~90 lines of headroom would leave the split
# unenforced — the evidence would simply drift back. Per size-target-is-direction the number is a
# direction, and the working margin the rounding rule leaves is the intended slack; a later change
# that genuinely needs the room raises the row in-diff with its own justification, which is exactly
# the audit trail wanted. Set per the rounding rule above from the measured actuals: 112 lines ->
# 120, 957 words -> 1000. The next multiple of 5 is 115, but that leaves THREE lines of margin, which
# this block has twice recorded raising past (TWO lines above, ZERO lines in the entry immediately
# preceding) — so the multiple after it, 120. 1000 leaves 43 words (above the within-25 threshold).
# skills/docket-build/SKILL.md's budget was raised 320/2950 -> 325/3000 by change 0231, which
# extended the *A worker return is malformed or unverifiable* halting bullet with the sibling
# prohibition on discard-and-re-dispatch, and extended § *Dispatching a task*'s concurrency ban to
# a controller who believes the first worker is gone. The two references/ files that exist under
# skills/docket-build/ were both considered and neither can hold this prose. gate-execution.md is
# scoped to the build GATE's execution posture — how to run the suite and observe its result — and
# this rule fires at worker dispatch, before any gate runs. task-routing.md is the profile-selection
# rubric, shared with docket-implement-next's fix loop, and a dispatch-safety prohibition stated
# there would reach the fix loop in docket-build's disposition vocabulary, which is the exact
# mis-import change 0231 avoids by giving fix-loop.md its own sentence. A halting condition must
# also sit in the halting-conditions list a controller reads at the moment it decides what to do
# with a bad return; a rule in an unread reference cannot intervene at that moment. A later 0231
# review fix then trimmed that same bullet's restatement of the section's shared worktree-preserved
# disposition, so the file measures BELOW where the raise was set; the row is a ceiling and was not
# lowered, only these figures were corrected to the post-trim actuals. Set per the rounding rule
# above from the measured actuals: 317 lines -> the next multiple of 5 is 320, which leaves 3
# lines — the near-zero mode this block warns about, and the same reading the rows above took — so
# the multiple after: 325. 2938 words -> the next multiple of 50 is 2950, which leaves a 12-word
# margin (inside the 25-word threshold), so the multiple after: 3000.
# skills/docket-build-task/SKILL.md's budget was raised 125/1100 -> 130/1150 by change 0231, which
# widened ## Scope's amend ban from "earlier task commits" to ANY commit — including one this
# worker just made — and added the correct-by-adding-another-commit direction. (The raise also
# covered a second copy of the plan-wins-on-commit-count escape in ## Scope; a later 0231 review
# fix deleted that copy as a non-sequitur under an amend ban and a duplicate of the one in
# ## The commit, so the file measures below where the raise was set. The row is a ceiling and was
# not lowered — only these figures were corrected to the post-deletion actuals.)
# skills/docket-build-task/ has NO references/ tree, so the only candidate home
# is one that would have to be created, and creating it is wrong here for the same reason the 0212
# entry above records for this file: the body reaches a worker's context by wrapper preload
# (agents/docket-build-*.md carry skills: [docket-build-task]), and a rule that must bind a worker
# at the moment it is about to amend cannot sit in a file the wrapper does not preload. Set per the
# rounding rule above from the measured actuals: 122 lines -> the next multiple of 5 is 125, which
# leaves 3 lines — the near-zero mode this block warns about, and the same reading 0212 took on
# this very file (119 -> 120 left one line, so 125) and Task 1 of this change took on
# docket-build/SKILL.md — so the multiple after: 130. 1087 words -> the next multiple of 50 is
# 1100, which leaves a 13-word margin (inside the 25-word threshold), so the multiple after: 1150.
# skills/docket-implement-next/references/fix-loop.md's row was raised 180/1850 -> 185/1900 by
# change 0231, which states the never-discard-and-re-dispatch prohibition in the fix loop's OWN
# disposition vocabulary. This row BREACHED on lines rather than merely tightening: 181 measured
# against a 180 budget, with the word count landing exactly on 1850 for zero margin. The considered
# home is skills/docket-build/SKILL.md, which owns the controller-side rule and states it there in
# the same change. It cannot be the only home: docket-implement-next Step 6 dispatches
# docket-build-task workers itself and never loads docket-build's SKILL.md, so a pointer would
# import docket-build's `halted` BUILD outcome where the fix loop's disposition is abort-and-report
# with the change left in-progress and claimed_at refreshed. That is one sentence duplicated into
# two vocabularies, which is the shape this file's owner already uses for shared rules, rather than
# a restatement of the same sentence. Set per the rounding rule above from the measured actuals:
# 181 lines -> the next multiple of 5 is 185, leaving 4 lines, which clears the near-zero reading
# the two rows above took (2 and 3 lines) and is the proportional analogue of the 25-word threshold
# on a 50-word step, so 185 stands. 1850 words -> the next multiple of 50 strictly above the actual
# is 1900 (50 words of margin); leaving the row at 1850 would sit exactly on the actual, which is
# the 0102 near-zero failure mode in its purest form.
# skills/docket-finalize-change/SKILL.md's WORD budget was raised 3500 -> 3700 by change 0190, which
# extended item 4's third condition from bare SHA equality to a disjunction: equality, OR a strict
# ancestor `head_sha` whose whole `head_sha..HEAD` delta lies under the configured `<results_dir>/`.
# The prose grew because every word of it is PREDICATE — the allowlist prefix and where it is read
# from, the fresh null-delimited `git diff --name-only -z` derivation, tracked paths only, the
# empty-diff-is-doubt rule, the enlarged anything-else list, the permit named in the skip log, and
# the degrade-off tie to the invisibility guard. A shortened form is a predicate the agent guesses
# at, and guessing wrong here merges an untested branch onto the integration branch.
# WHERE ELSE IT WAS CONSIDERED, per the naming requirement above:
# skills/docket-finalize-change/references/gate-failure.md, the only reference this skill has. It
# cannot host this. That file owns the FAILURE posture — what finalize does once the merge gate has
# already gone wrong (the two-agent split, repair sign-off, the abort-and-report set, the
# `## Finalize blocked` write) — and it is read only after that has happened. This is the opposite
# moment: a merge-GATING predicate evaluated on the ordinary green path, at the instant the agent
# decides whether the suite runs at all. A predicate sitting in a reference nobody reads on that
# path does not gate the merge — it silently permits it, which is the same argument the 0137 and
# 0113 entries above record. Set per the rounding rule above from the measured actual: 3625 words ->
# the next multiple of 50 is 3650, which leaves exactly 25 words (within the 25-word threshold), so
# the multiple after it: 3700 (75 words of margin). The LINE budget was NOT raised (176 actual, 180
# budget) — the growth is all inside one numbered item's single line.
# skills/docket-implement-next/references/edge-paths.md's WORD budget was raised 450 -> 500 by change
# 0190, which rewrote Step 7's build-evidence paragraph: a post-gate results commit no longer always
# defeats finalize's skip, so the prose now states the rule (write the block with the pre-commit
# `head_sha`; expect a skip when the delta is docs-only under `<results_dir>/`, a suite run when it
# is not) instead of the flat "the suite simply runs" it replaced. The row is raised even though the
# test currently PASSES: the file measures 448 words against 450, TWO words of headroom, which is
# precisely the near-zero failure mode this comment block's 0102 and 0137 entries exist to forbid —
# the next one-word edit to this file reddens CI on arrival. The raise is taken now rather than
# deferred onto whoever makes that edit.
# WHERE ELSE IT WAS CONSIDERED, per the naming requirement above: there is no further reference to
# consider, because this file IS the reference. The prose already lives at its extracted home —
# change 0201 moved the implementer's rare edges here out of skills/docket-implement-next/SKILL.md,
# and pushing it back into SKILL.md is the reverse of the extraction (and would raise the parent's
# budget instead, for prose that is conditionally read by construction). The naming rule's question
# is answered by exhaustion rather than by argument against a candidate. Set per the rounding rule
# above from the measured actual: 448 words -> the next multiple of 50 is 450, which leaves 2 words
# (far inside the 25-word threshold), so the multiple after it: 500 (52 words of margin). The LINE
# budget was NOT raised (28 actual, 35 budget).
# skills/docket-implement-next/SKILL.md was NOT raised by change 0190: it measures 162/4285, inside
# the existing 165/4300.
# docket-finalize-change/SKILL.md's budget was raised again, 180/3700 -> 185/3800, by change 0190's
# whole-branch review fix (finding 2): the skip's second limb had shipped ON by default everywhere,
# with a trailing "degrade off" sentence an agent was expected to self-apply. It is now armed by a
# real, default-off, coordination-fenced config key, so step 4 states the arming rule
# (FINALIZE_SKIP_RESULTS_ONLY_DELTA, read from the Step-0 export block; unset or false means change
# 0170's equality-only predicate and the disjunct is not evaluated at all; the key is
# repo-committed only because arming it asserts a property of that repo's own suite), and the
# gate's configured-block sample plus its export-block sentence name the key beside its siblings.
# WHERE ELSE IT WAS CONSIDERED, per the naming requirement above: the same and only candidate the
# entry above names, skills/docket-finalize-change/references/gate-failure.md, and it is rejected
# for the same reason with more force. That file is read only AFTER the gate has already gone
# wrong; the arming rule is evaluated on the ordinary GREEN path, at the instant the agent decides
# whether the suite runs at all — the one run that never opens it. An arming rule parked in the
# failure reference does not withhold the skip from an unverified repo, it silently grants it.
# Set per the rounding rule above from the measured actuals: 3750 words -> the next multiple of 50
# IS 3750, i.e. ZERO margin, so the multiple after it: 3800 (50 words of margin). 178 lines -> the next multiple of 5 is 180, which leaves TWO lines — the near-zero headroom
# this block's 0102/0137 entries and three later fix rounds all record raising for — so the
# multiple after it: 185.
# skills/docket-build/SKILL.md's budget was raised 325/3000 -> 335/3150 by change 0224, which states
# the build gate's verdict rule normatively: green iff the resolved suite command exits zero, with
# output text diagnostic only, the deciding status read from the terminal result artifact, the
# per-file-loop aggregate named, and the repair re-run bound by the same rule. The two references/
# files under skills/docket-build/ were both considered and neither can hold it.
# references/gate-execution.md is quarantined per-harness capability and probe evidence, read ONCE
# before the gate starts; this rule must be in hand at the moment the verdict is formed, in the
# section that already states what green and red DO. Splitting "what decides green" from "what green
# does" across two files is precisely the drift that produced the gap this change closes — the
# section defined both meanings and never their determinant. references/task-routing.md is the
# profile-selection rubric shared with docket-implement-next's fix loop and has nothing to do with
# the gate. Set per the rounding rule above from the measured actuals: 328 lines -> the next multiple
# of 5 is 330, which leaves 2 lines — the near-zero mode this block repeatedly records raising past —
# so the multiple after: 335. 3092 words -> the next multiple of 50 is 3100, an 8-word margin inside
# the 25-word threshold, so the multiple after: 3150.
# Change 0237 raised TWO word budgets to give the `## Run halted` section a definition and a
# PRODUCER. docket-convention/SKILL.md 5900 -> 6000: one bullet added to the *Change body sections*
# list, which is the enumeration `verify-run` now reads against — a section defined nowhere is a
# section no author writes. The references/ file considered and rejected is
# skills/docket-convention/references/terminal-close-out.md: the body-sections list is a single
# enumeration in SKILL.md and half of it living in a reference makes the list unreadable as a list.
# Measured 5942 words -> the next multiple of 50 is 5950, which leaves 8 words (within the 25-word
# threshold), so the multiple after: 6000. The LINE budget was NOT raised (343 actual, 345 budget).
# docket-implement-next/SKILL.md 4300 -> 4500: the producer (Step 3's halted escape must WRITE and
# COMMIT the section) plus its removal rule (Step 2's claim deletes a stale one). The references/
# file considered and rejected is skills/docket-implement-next/references/edge-paths.md: both are
# rules that must fire at the exact moment of action — the write is what makes a `halted`
# disposition verifiable in git instead of an untrusted self-report, and the removal is a step of
# the claim commit itself. A rule parked in a rare-edges reference is unread precisely when it must
# intervene (the same argument the 0113 and 0137 entries above record). Measured 4467 words -> 4500
# (33 words of margin, above the 25-word threshold). The LINE budget was NOT raised.
# Change 0200 raised docket-convention/SKILL.md 345/6000 -> 355/6100 to record that merged
# plan and results artifacts are FROZEN build records. The references/ file considered and rejected
# is skills/docket-convention/references/terminal-close-out.md — it is the natural topical home
# (it already owns what happens to artifacts at close-out), and it is the wrong one: a references/
# file is read ON DEMAND, and this rule has to be in hand BEFORE an agent decides to touch a merged
# plan, which is a decision taken while doing something else entirely. Step 0 loads SKILL.md
# unconditionally for every workflow skill, so that is the only surface where the rule fires before
# the action it forbids. The rule's own origin is the proof: change 0217 surfaced it because a
# merged plan's verification grep had gone stale and the reflex was to edit the plan. Set per the
# rounding rule above from the measured actuals: 350 lines -> the next multiple of 5 is 350 itself,
# leaving ZERO margin — the near-zero mode this block repeatedly records raising past — so the
# multiple after: 355. 6064 words -> the next multiple of 50 is 6100, a 36-word margin, above the
# 25-word threshold, so 6100 stands.
# skills/docket-build-task/SKILL.md's budget was raised 130/1150 -> 145/1350 by change 0249, which
# added two normative clauses to the worker contract: a pointer in ## The cycle to the gate's
# execution capabilities plus the worker-shaped consequence inline (never yield, observe by
# blocking, finite observation, unfinished is not green), and a ## Scope bullet requiring staging by
# explicit path with its escalation carve-out.
# WHERE ELSE IT WAS CONSIDERED, per the naming requirement above: skills/docket-build-task/ has NO
# references/ tree, so the only candidate home is one that would have to be created, and creating it
# is wrong here for the reason the 0212 and 0231 entries above already record for this same file —
# the body reaches a worker's context by wrapper preload (agents/docket-build-*.md carry
# skills: [docket-build-task]), and a rule that must bind a worker at the moment it is about to
# stage or about to yield cannot sit in a file the wrapper does not preload. The other candidate,
# skills/docket-build/references/gate-execution.md, is where the pointer POINTS and already holds
# everything extractable: it states the harness capabilities, not the worker's conduct, so the
# never-yield / finite-observation / fail-closed consequence has no home there. Set per the rounding
# rule above from the measured actuals: 139 lines -> the next multiple of 5 is 140, which leaves ONE
# line — the near-zero mode this block's 0102 and 0137 entries exist to forbid — so the multiple
# after: 145. 1319 words -> the next multiple of 50 is 1350, a 31-word margin, above the 25-word
# threshold, so 1350 stands.
BUDGETS="
skills/docket-adr/SKILL.md                                  86 1408
skills/docket-adr/adr-template.md                           26   90
skills/docket-auto-groom/SKILL.md                           66 1237
skills/docket-brainstorm/SKILL.md                           84  692
skills/docket-build/SKILL.md                               335 3150
skills/docket-build/references/gate-execution-evidence.md  110 1050
skills/docket-build/references/gate-execution.md            120 1000
skills/docket-build/references/task-routing.md              50  500
skills/docket-build-task/SKILL.md                          145 1350
skills/docket-convention/SKILL.md                          355 6100
skills/docket-convention/github-board-mirror.md             19  462
skills/docket-convention/references/agent-layer.md         190 2150
skills/docket-convention/references/auto-capture.md        130 1250
skills/docket-convention/references/learnings.md            84  580
skills/docket-convention/references/terminal-close-out.md  173 1458
skills/docket-finalize-change/SKILL.md                     185 3800
skills/docket-finalize-change/references/gate-failure.md    35  900
skills/docket-groom-next/SKILL.md                           77 1484
skills/docket-implement-next/SKILL.md                      165 4500
skills/docket-implement-next/references/edge-paths.md       35  500
skills/docket-implement-next/references/fix-loop.md        185 1900
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
