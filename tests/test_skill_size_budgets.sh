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
BUDGETS="
skills/docket-adr/SKILL.md                                  86 1408
skills/docket-adr/adr-template.md                           26   90
skills/docket-auto-groom/SKILL.md                           66 1237
skills/docket-brainstorm/SKILL.md                           84  692
skills/docket-build/SKILL.md                               280 2500
skills/docket-build/references/task-routing.md              50  500
skills/docket-build-task/SKILL.md                          125 1100
skills/docket-convention/SKILL.md                          345 5800
skills/docket-convention/github-board-mirror.md             19  462
skills/docket-convention/references/agent-layer.md         190 2150
skills/docket-convention/references/auto-capture.md         45  450
skills/docket-convention/references/learnings.md            84  580
skills/docket-convention/references/terminal-close-out.md  173 1458
skills/docket-finalize-change/SKILL.md                     180 3450
skills/docket-finalize-change/references/gate-failure.md    35  900
skills/docket-groom-next/SKILL.md                           77 1484
skills/docket-implement-next/SKILL.md                      150 3850
skills/docket-implement-next/references/edge-paths.md       35  450
skills/docket-implement-next/results-template.md            24  172
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
