<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0257 — Clear the residual review findings from 0193 and 0201](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0257-clear-the-residual-review-findings-from-0193-and-0201.md)**
<!-- docket:backlink:end -->

# Clear the residual review findings from 0193 and 0201 — design

Change 0257 · groomed 2026-08-07 (auto-groom) · consolidates the surviving items of killed
#0197 (0193's five merge-time findings) and killed #0204 (0201's rationale losses + absorbed
0214's AGENTS.md half-rule).

All eight edit sites re-verified live on `main` 2026-08-07. Line numbers below are current
anchors, not contracts — re-locate each site by its quoted content at build time (several
in-flight changes touch the same files; see Assumptions A8/A9).

## The eight edits

### E1 — README roster cells (0197 finding 1)

`README.md` agent-roster table: the `docket-build` row (~:653) and `docket-review` row (~:655)
still open "Pluggable `build` role, opt-in —" / "Pluggable `review` role, opt-in —", contradicted
by the same file's binding-posture paragraph (~:759, "Change 0193 ended that: … shipped
cross-harness defaults"). Replace "opt-in" with "shipped default" in exactly those two cells.
The `docket-brainstorm` row's "opt-in" is correct and stays.

### E2 — Convention config-sketch comment (0197 finding 2)

`skills/docket-convention/SKILL.md` ~:45: the sketch comment
`skills: # pluggable workflow skills; unset key = the superpowers default shown` is falsified by
the `build:`/`review:` rows directly below it (docket-owned values). Reword to
`unset key = the shipped default shown`. This is the only live surface carrying the stale phrase
(the other three grep hits are archived specs/plans and 0193's own results record — historical
artifacts, untouched).

### E3 — Non-vacuity companion in the review test (0197 finding 3)

`tests/test_docket_review.sh` ~:857-860: the absence assert
`assert "this repo no longer pins skills.review …" '[ -z "$dy_skills" ]'` extracts via an inline
awk block-scanner and passes green on any extraction failure (missing file, renamed header,
broken awk). Add a live companion **through the same extractor**: run the identical awk shape
over a block known present in this repo's `.docket.yml` (`build:` — deliberately pinned there
for `checkpoint:`) and assert the slice is non-empty, mirroring
`tests/test_docket_build.sh:673` (`"repo's build: block extraction is non-vacuous"`).
"Same extractor" is load-bearing: factor the awk into one parameterized function used by *both*
asserts (the `dy_yaml_block_body` shape at `test_docket_build.sh:654-660` is the precedent) —
two copy-pasted awks would leave the absence extractor unguarded. Mutation-check at build:
break the shared extractor's block-start regex and the *companion* must redden.

### E4 — Remove the two entailed asserts (0197 finding 4)

`tests/test_docket_config.sh` ~:460-464: the two `!=` asserts
(`BUILD default is no longer SDD`, `REVIEW default is no longer superpowers review`) are strictly
entailed by the `=` asserts four lines above (`[ "$SKILL_BUILD" = docket-build ]` etc.) — a
shell string equal to `docket-build` cannot also equal the superpowers id, so they can never
redden independently. Remove both plus their now-orphaned comment. Disposition is **remove, not
re-aim**: the both-directions resolver guards already live in `test_docket_build.sh:679-686` and
`test_docket_review.sh:850-856`, where the haystack (a grepped source line) genuinely can
contain both strings.

### E5 — Anchor the opt-back-in guard (0197 finding 5)

`tests/test_docket_build.sh` ~:694-695: `"README says how to opt back into SDD"` greps the
whole-file `$rm_body` for `superpowers:subagent-driven-development`, already satisfied by
incidental prose — deleting the actual opt-out instructions leaves it green. Anchor it:
extract the `### docket-build` README section body (awk heading-to-heading range, the shape
`rvsec` already uses in `test_docket_review.sh`), add a non-vacuity anchor on the slice, then
assert the SDD id within it. No stacked gaps, bounds ≤ 255 (0253-compatible; see A8).

### E6 — Restore the anti-deadlock rationale (0204 item 1)

`skills/docket-finalize-change/references/gate-failure.md`, `## Finalize blocked` marker
section: 0201 relocated the override rule's mechanics but dropped its why. Note the reference
file's marker section carries **no explicit-id override discussion at all** (the rule lives only
in `SKILL.md`'s inline stub) — so the restoration minimally restates the override rule alongside
its why, anchored to the skip/clearing discussion: an explicitly named id overrides the
auto-detect skip because without it a change carrying the marker could never be finalized — the
skip excludes it and the clearing rule only fires on a successful finalize, a deadlock. `SKILL.md`'s inline stub (~:164, the "I looked at it, retry"
signal) is a *different, complementary* rationale and stays as is; the reference file is where
0201 should have relocated this one.

### E7 — Complete AGENTS.md's frontmatter-edit rule (0204 item 2, ex-0214)

`AGENTS.md` §"Frontmatter and generated blocks", the anchoring bullet (~:27-28): append a
clause (editorial default from the stub — a clause, not a new bullet, since anchoring and
whitespace-class are one operation with independent failure modes): match an empty value's
trailing run with `[[:blank:]]*`, never `\s*` (in Perl/PCRE `\s` includes `\n`, so on an
empty-valued field it eats the terminator and welds two fields into one line, exit 0), and read
the field back after writing. Grep re-confirmed 2026-08-07: no committed script carries the
`s/^<field>:\s*$/` shape — the clause targets agent-authored ad-hoc shell only.

### E8 — Bounded rationale-loss sweep (0204 item 3)

Read the four SKILL.md files 0201 compressed (`docket-convention`, `docket-finalize-change`,
`docket-implement-next`, `docket-build`) against their reference files for further instances of
the class *rule kept, rationale dropped instead of relocated*. Fix in-change only same-class,
one-sentence restorations; record anything larger in the results file as discovered follow-up
(no minting from the build). The 0204 item-2 instance (auto-capture backlog-growth loop) is
confirmed already restored by 0226 (`skills/docket-convention/SKILL.md:276-278`) — excluded.

## Verification

- Full suite green (`scripts/run-tests.sh`); the budget rows for the touched SKILL.md/reference
  files must absorb E2/E6 (a handful of words — re-measure, don't assume headroom, per 0204's
  out-of-scope note).
- E3/E5 mutation checks as above; E4 is a deletion — confirm the surviving `=` asserts still
  redden on a resolver default flip (poison-var harness already proves this shape).
- No new guard machinery beyond E3/E5 (stub's out-of-scope, honored).

## Assumptions

- **A1 (E1 wording).** "shipped default" replaces "opt-in" in the two roster cells, minimal edit.
  Rejected: rewriting the full cells (churn without content) and deleting the qualifier outright
  (loses the posture statement the row exists to carry). The :759 paragraph's own phrase
  ("shipped cross-harness defaults") is the vocabulary anchor.
- **A2 (E2 scope).** Only the live SKILL.md comment changes; archived specs/plans and the 0193
  results record keep the stale phrase as historical artifacts. Rejected: repo-wide rewrite —
  editing archives falsifies records.
- **A3 (E3 shape).** Companion runs the *same extractor* — factored per E3, never copy-pasted —
  over the `build:` block of the same `.docket.yml`. Rejected: asserting file existence only (doesn't exercise the extractor);
  hoisting the build test's `dy_yaml_block_body` helper into shared lib (real, but that is
  0252's fixture/hermeticity territory — a one-site copy of the proven shape is the bounded fix).
- **A4 (E4 disposition).** Remove, not re-aim — the stub's stated default, and the re-aimed
  targets already exist in the two role tests. Rejected: re-aiming at the resolver source here
  (duplicate coverage) and keeping them as documentation (asserts that cannot redden are
  decoration per AGENTS.md's mutation rule).
- **A5 (E5 anchor).** Section-scoped extraction + non-vacuity anchor + fixed-string grep within.
  Rejected: proximity-gap ERE tying "opt back" to the SDD id (adds a prose-anchored gap pattern
  to exactly the population 0253 is draining); leaving it whole-file with a longer needle
  (still incidental-prose-satisfiable).
- **A6 (E6 placement).** The sentence lands in `references/gate-failure.md`, not SKILL.md — the
  reference file is the relocation target 0201 defined; SKILL.md's inline stub stays pointer-thin
  and already carries its own (different) retry-signal rationale. Rejected: both sites (regrows
  the duplication 0201 removed).
- **A7 (E7 form).** One clause on the existing anchoring bullet, per the stub's editorial
  default. Rejected: a separate bullet — defensible (independent failure modes, 0206 satisfied
  anchoring while violating whitespace-class) but the two govern the same edit operation and the
  section is a checklist read as a unit; the clause keeps them co-located.
- **A8 (coupling — 0253, written into the stub's `related:` frontmatter as `[253]` in this
  groom's commit, not prose-only).** 0253 (proposed, build-ready) converts
  and rewrites prose-anchored guards in `test_docket_build.sh` and `test_docket_review.sh` — the
  same files, and its build-time site re-derivation would sweep the very guard E5 rewrites. No
  hard ordering: whichever lands second reconciles, and E3/E5 are written 0253-compatible
  (flattenable, single-gap-free, bounds ≤ 255). Mirrors 0253's own treatment of its 0172
  collision (`related:`, not `depends_on:`).
- **A9 (couplings left in prose).** 0249 (spec'd, appends new guards under a banner to
  `test_docket_build.sh`) and 0224 (`implemented`, PR #174 open, appends to the same file):
  plain append-adjacency with no shared guard and no ordering constraint — 0249's own critic-gated
  spec classified its 0257 collision exactly this way and left it in prose; symmetric treatment
  here. 0172 (producer-pipe shape, rewrites `fmv()` there): textual collision only, none of
  0257's edits touch pipes or `fmv()`.
- **A10 (E8 bound).** The sweep is the four SKILL.mds vs their reference files, read-only unless
  a same-class one-sentence fix applies; larger finds go to results prose. Rejected: skipping the
  sweep (0204 carried it for the `fix-reintroduces-its-own-defect-class` learning) and widening
  to all compressed files (unbounded).
- **A11 (dependency state).** `depends_on:` stays empty — nothing here requires another change
  to land first; all eight sites exist on current `main`.

## Out of scope

- 0200's board-checks bundle (separate change).
- Any new guard machinery beyond E3/E5; hoisting shared test helpers (0252); guard-pattern
  policy (0253); producer-pipe hygiene (0172).
- Editing archived specs/plans/results that quote the stale phrases.
