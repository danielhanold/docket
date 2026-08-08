---
slug: verify-the-claim
hook: "A document asserting a fact about another artifact is not an oracle — verify it against the artifact or the RUNNING CODE before acting on it."
topics: [process, review, spec]
changes: [12, 21, 47, 65, 67, 74, 96, 101, 102, 109, 112, 138, 130, 157, 164, 170, 212, 211, 200]
created: 2026-06-12
updated: 2026-08-08
promotion_state: retained
promoted_to:
---

## Apply
Verify a claim against the artifact or the RUNNING CODE before acting on it — byte-diff a
review's quoted sentence; RUN the command whose behavior a spec describes before encoding it in an
assert; and write prose asserting a tier, count, default, or behavior against the CODE (cite the
line), never against sibling prose that may already have drifted. Treat prose restating a
configurable value as a drift surface from the day it ships. When a claim is false but its
conclusion still defensible, keep the conclusion, write the test to the OBSERVED behavior, and
record the discrepancy in the results file — never silently override a spec's scope boundary
mid-build; leave the re-scope to the human. Reject false positives with evidence.

## War story
- 2026-06-12 → 2026-07-16 (#12 PR #7; #21 PR #34; #47 PR #55; #65 PR #74; #74 PR #82; #67 PR #91 —
  merged, one verify-the-claim family) — A document that asserts a fact about another artifact — a code
  review, a spec, teaching prose — is not an oracle, and it has been flatly false five times. (a) A review
  finding cited a sentence that did not exist in the reviewed file. (b) A spec's stated *rationale*
  for a scope boundary was wrong (it claimed the convention's `.docket.yml` example "does not
  enumerate `finalize:`" — it does), though the boundary itself was sound on other grounds. (c) #74's
  spec claimed `docket.sh bootstrap` in a `STOP_MIGRATE`-shaped repo "exits non-zero and writes
  nothing"; against the real resolver it exits **0**, emits `BOOTSTRAP=STOP_MIGRATE`, and writes
  nothing — so an assert written FROM the spec would have pinned fiction and gone green doing it.
  (d) Prose restating a fact owned by another file drifts, and no sentinel can catch it: #65's README
  asserted which model tier each built-in agent runs at and shipped factually FALSE, with every grep
  green, because a doc sentinel proves a sentence still EXISTS and can never prove it is still TRUE.
  (e) **A CODE COMMENT is a claim too, and #67 broke this rule inside the change that ships it.** A
  fix's own comment warned against a "naive two-pass" unescape and explained why the single-pass was
  required; review swapped the single-pass *for* the two-pass it warns against and the suite stayed
  green. The comment's scenario was unreachable — `_dq_unescape_dquote` is called with the closing
  delimiter already stripped — so for well-formed YAML the two passes are provably equivalent and the
  comment was asserting a danger that could not occur. The single-pass is still correct (it degrades
  predictably on *malformed* input), but the justification was decoration and the fixture set could
  not tell the two apart. Fixed by adding the one discriminating fixture (`"path C:\\" and more"` — a
  bare unescaped quote inside a double-quoted scalar) and correcting the comment. Treat a comment
  explaining why code is correct as an unverified claim: find the input that distinguishes it from the
  alternative it rejects, or delete the claim.
- 2026-07-19 (#96, PR #102) — **A FALSE PARALLEL is the shape this family takes in prose, and the
  guard structurally cannot catch it.** The change shipping the autonomy-precedence rule wrote, in
  `docket-finalize-change`, that on the autonomous path finalize "pre-specifies its outcome exactly as
  `docket-implement-next` §7 does". It does not — autonomous finalize never invokes `$SKILL_FINISH` at
  all; it merges via `gh` and runs its own steps 1–6. The sentence invited the exact inverse drift it
  meant to prevent (a future autonomous finalize concluding it *should* invoke the finish skill with a
  directed outcome). Two compounding lessons. (a) The claim was about a SIBLING SKILL's control flow,
  the drift surface this finding already names — an "exactly as X does" analogy is an assertion about
  X, verifiable only by reading X's actual path. (b) **The guard could not see it**, and not by
  accident: that line already satisfied the token check through the human-present exception branch, so
  a prose sentinel keyed on presence is blind to a line that carries the token and lies anyway. The
  repo's own documented false-prose mode reappeared *inside the change that ships the rule against
  it* — a claim's provenance ("we just wrote the rule") is not evidence. Recorded as a follow-up:
  requiring the exception line to also carry a shape assertion would close the blind spot.
- 2026-07-20 (#101, PR #109) — **A CONSOLIDATION copies from the surfaces it is replacing, and those
  are the drifted ones.** The change whose entire deliverable was a canonical config reference shipped
  two Critical-severity false claims, both the same one: that the first `github` sync mints a Projects
  v2 board and writes the resolved `{owner, number}` back over `auto`. Nothing reads `github_project`
  from config at all — `github-mirror.sh` resolves its board only from `--project` /
  `--auto-create-project`, and `docket-status.sh` populates those only from CLI flags no skill passes.
  The correct fact was already written down **in this change's own plan** ("documented-but-unwired key
  … the `auto` sentinel is documentation-only"); the prose was then written against the *old*
  `.docket.yml`'s claim instead. So: when consolidating N drifting surfaces into one canonical one,
  every sentence carried over is an unverified claim inherited from a known-drifted source — the
  artifacts being replaced are the *least* trustworthy input available, and your own plan's
  established findings outrank them. Second consecutive change (after #96) in which this family
  reappeared inside the change built to end it.
- 2026-07-20 (#109, PR #112) — **The family's mirror image: a plan's RISK claim, wrong in the safe
  direction.** The plan warned that a missed escaped-ERE replacement would leave the `(8)` guard
  "silently vacuous while still reporting ok". Review mutation-tested exactly that — regressed the ERE
  to the old form — and both `(8)` pointer asserts reddened with `<no link>`: the guard's existing
  `[ -n "$sn_ptr" ]` floor already covered the hazard. The hazard was real, the *consequence* was not.
  Worth recording because a plan's risk claims get a pass that its factual claims do not (over-caution
  costs nothing, so nobody checks), and an unchecked one silently sets the bar for how much guarding
  the change ships. Mutation-testing the claim is what separated "the guard needs work" from "the
  guard already holds" — and it is the same move whether the claim turns out too generous or too dire.
- 2026-07-21 (#102, PR #115) — **A contract's stated INVARIANT was already false before the change
  touched it, and the change was about to inherit it.** `scripts/docket-config.md` asserted, in both
  the global-layer and machine-local-layer bullets, that layer problems "never abort" — warned and
  ignored. Untrue at the time it was written: a malformed value for any fail-closed boolean
  (`auto_capture` then, `require_pr_approval` now) aborts every docket command on that machine. The
  new key would have quietly become the second counterexample to a rule the doc still claimed was
  absolute. Two things worth reusing. (a) **A doc's *invariants* deserve the same suspicion as its
  *facts*** — a sentence saying "X never happens" is a claim about every code path, which is the
  hardest kind to keep true and the least likely to be re-checked when a path is added. (b) The
  audit that found it also found two table rows (`learnings.enabled`, `reclaim.auto`) that failed to
  document their own abort behavior at all — so the drift was systemic, not a one-off. When adding
  an entry to a table whose other rows make a behavioral claim, verify the claim for the *existing*
  rows before matching their format; copying the format silently copies the assertion.
- 2026-07-22 (#112, PR #118) — **Re-derive an ACCURATE report too — you cannot know it was accurate
  until you have.** The whole-branch reviewer did not read the implementer's 18-cell mutation matrix
  and check it for internal consistency; it copied the resolver and test file into an isolated tree,
  re-applied all three mutations by content-anchoring, and re-ran every state, confirming the deltas
  suite-wide rather than only inside the section under test. The report turned out to be entirely
  correct. That is the point worth recording: the verification's value did not depend on finding an
  error, and a reviewer who reads a plausible matrix and agrees has produced no evidence at all.
  This is `guards-are-code`'s "never trust an implementer's narrative" applied where the narrative
  happened to be true — the only condition under which you learn whether the discipline is real.
  Also here, a **near-collision ruled out explicitly rather than assumed away**: the new `s7` assert
  looks like a re-pin of the pre-existing `L2` fixture (both resolve `make local-test` from the
  local rung), and the reviewer distinguished them by running the mutation — `L2` has the committed
  key *absent* rather than set to `auto`, so M3 leaves it untouched while `s7` reddens. "This is
  probably already covered" is a claim about another test's behavior, verifiable only by running it.
- 2026-07-24 (#138, PR #125) — **An ENUMERATION of affected call sites is itself a claim — verify
  its COMPLETENESS against every call site, not just the ones it names.** The change had `field()`
  strip a matched surrounding quote pair, so board titles render unquoted. The spec enumerated six
  *title* consumers of `field()` and asserted "every `field()` consumer benefits"; reconcile
  confirmed the enumerated six but trusted the enumeration as the complete set instead of
  independently auditing *all* `field()` call sites for what they do with the result. The one
  consumer outside the list is exactly where it broke: `render-learnings-index.sh` reads the finding
  `hook` via `field()` and then runs its OWN full YAML unescaper (`dequote`), which needs the RAW
  quoted scalar — outer quotes intact — to detect the quote style and run its escaped-closer guard;
  stripping the quotes in `field()` broke it. The whole-branch review didn't catch it either — the
  full-suite **finish gate** did, reddening `tests/test_render_learnings_index.sh`. Resolved by
  ADR-0058 (two-tier readers: `field_raw()` for consumers that do their own decode, `field()` for
  plain reads). The move that was skipped is cheap and mechanical: `grep` every `field()`/`fm_field()`
  call site and check each for post-decode behavior, rather than accepting the spec's list of which
  ones matter. A spec that says "the consumers affected are X, Y, Z" is asserting a closed set;
  closure is the part that fails, and only auditing the open set proves it.
- 2026-07-28 (#130, PR #133 — merged) — **A regex claim asserted in both the spec and the plan,
  never executed, and false.** Spec §A3 and the plan both stated that the ERE extraction pattern
  `\{[0-9]+(,[0-9]*)?\}` "also matches BRE `\{m,n\}`", calling the over-match harmless. It matches
  it not at all — against `\{0,600\}` the pattern consumes `{`, then `600`, then requires `}` and
  finds `\` — so the guard shipped covering roughly half the surface its own header comment claimed,
  with 21 tracked non-`docs/` files using BRE intervals and the identical `maximum repetition
  exceeds 255` hazard live in `sed` and plain `grep`. One execution against the real source text
  would have shown it; three per-task reviews did not, and only the whole-branch review did. The
  repair's own helper was then a **silent fail-open**: without the backslash strip in `offenders()`,
  a genuine `\{0,600\}` planted in a tracked file was found by the scan and never reported, while
  the check printed `ok`. Mutation-verified in both directions, with a BRE positive control and a
  legal-BRE negative control.

  The counter-example in the same change is the model: `{,600}` (no lower bound) was flagged as a
  silent-escape risk, then **measured** — `/usr/bin/grep -E 'a{,600}' f` exits 1 with no error,
  because BSD does not parse that form as an interval at all — and ruled out of scope with the
  measurement recorded rather than with an assumption.
- 2026-07-28 (#157, PR #136 — merged) — **A spec's "already covered" is a coverage claim, and it
  measured zero.** Change 0152's spec asserted that `install.sh` was "genuinely covered already" for
  the Bash-major mutation, so its acceptance bar ("every surviving caller must redden") looked
  partially pre-satisfied. Direct measurement found **0 reddens** for that caller — the claim was not
  merely optimistic, it was inverted, and taking it at face value would have shipped the change
  declaring an acceptance bar it did not meet. The gap was in scope, so it was closed in
  `tests/test_install.sh`; all four callers then reddened (1/2/4/3), independently re-measured rather
  than inferred from the fix. Generalize: **"X is already covered" is the cheapest claim to make and
  the cheapest to check** — one mutation run answers it — and it is exactly the claim a spec author
  writes from memory of the file rather than from running it. When a change's acceptance bar is
  stated as coverage, measure the baseline before trusting any part of it as already paid.
- 2026-07-29 (#164, PR #138 — merged) — **Third instance of the same sub-class: README prose
  asserting agent model/effort tiers, factually false, every grep green.** A values-only retune moved
  `docket-implement-next` to `claude-opus-5`/**medium** and `docket-auto-groom` to
  `claude-opus-5`/**low**. Two `README.md` sites carrying the literals were edited; a paragraph 140
  lines below still said a build runs "at the top tier, **at high effort**" and that autonomous design
  "earns that same tier, **because deciding what to build is no cheaper than building it**" — the
  first clause false outright, the second asserting as justification the exact opposite of the new
  defaults. Every literal-based verification on the branch was green throughout and **none could have
  caught it: the sentence names no model ID and no effort value.** Generalize: prose that describes a
  configurable relationship *qualitatively* is a drift surface with **no grep signature**, so a
  stale-literal sweep provably cannot cover it — when a change moves a value, read the surrounding
  prose for claims about that value's *relationship* to others, not just for the value itself. Same
  branch, same sweep: excluding all of `docs/` from a stale-literal sweep is a trap — `docs/` also
  holds live user-facing documentation, not only archived records; narrow such exclusions to the
  archive subtrees.
- 2026-08-01 (#170, PR #149 — merged) — **A false prose claim pinned green by a guard written to
  match it.** The README asserted the new review design meant "one full-suite run on the clean path,
  two in the worst case, and never three" — false by the change's own contract, since a blocker fix's
  commits trigger a re-run (two) and a real rebase at finalize adds a third. `tests/test_docket_review.sh`
  was asserting `grep -qiE "never three"`, so the guard had promoted a prose error into a *guarded*
  prose error: every run green, forever, on a sentence that was wrong the day it was written.
  This is the failure mode's worst shape, and it is worth naming separately from the plain
  stale-prose case above. A guard keyed on a claim's own wording cannot distinguish "this claim is
  true" from "this claim is still present" — it tests presence and reads as correctness. When prose
  states a *count* or an *arithmetic property* of the system, derive it from the contract before
  writing either the sentence or the assert, and key the assert on surviving phrases rather than on
  the number itself. The correction here restated the honest arithmetic (one / two / three, with the
  condition for each) and re-pointed the assert.
  Two more in the same read, neither greppable: a reworded convention clause claimed a contract was
  "the only thing those wrappers load" across all eight exceptions — false for
  `docket-brainstorm-consultant`, which wraps no skill at all, a fact stated two lines below it; and
  the same sentence read "every wrapper except **four**" while naming five, left behind by #184's
  fourth build profile. Both are prose asserting a *count of siblings*, the drift surface #164
  recorded, and both were found by the whole-branch read that the suite structurally could not
  perform — which is the argument for keeping that read, made by the change that ships it.
- 2026-08-05 (#212, PR #161) — **A review finding is a claim, and gets the same treatment.** Of
  eleven findings on this branch, one asserted that `docket-brainstorm` was "the only swept body
  without `context: fork`". The worker fixing it checked: `docket-build`, `docket-review`, and
  `docket-build-task` also lack it. The finding's load-bearing half survived — `docket-brainstorm`
  is the only body whose *sole* invocation path is inline loading, which is what the fix actually
  needed — so the fix landed on the corrected basis rather than the stated one.
  The extension to this finding: reviewers produce documents asserting facts about artifacts, which
  is precisely the shape this rule already covers, but the review context invites deference — a
  finding arrives with the authority of having been *looked for*, and the natural response is to
  fix it rather than to check it. Verify the finding's premise against the running code before
  implementing its remedy. The cost of not doing so is a fix that is correct by luck, resting on a
  false rationale that then gets written into the commit message and outlives everyone's memory of
  the review.
- 2026-08-05 (#211, PR #160) — **A review finding was real; its stated failure mode was wrong, and
  writing the test the reviewer described would have measured the wrong thing.** The finding said an
  untested empty-collection short-circuit in `board-checks.sh` would, if removed, abort the walk
  under `set -u`, truncate every later change's findings, and exit non-zero. Measured, it does not:
  the expansion sits inside `$( … )`, so the `set -u` failure kills only the command-substitution
  subshell — the parent walk continues and the script exits 0. The build measured this, then
  re-verified it independently before accepting the deviation, and wrote the mutation to assert the
  walk **survives** so that a future bash which *does* abort reddens here rather than silently
  changing what the mutation measures. The generalization: **accept a review finding's existence
  claim and its causal claim separately.** The reviewer saw a genuine coverage gap by reading; the
  mechanism they attributed to it was a prediction about runtime behavior, and a test built on that
  prediction inherits it as an unexamined premise — passing for reasons neither the reviewer nor the
  implementer ever checked.
- 2026-08-08 (#200, PR #181) — A new convention rule ("build artifacts are frozen after merge") was
  written unconditionally and was falsified by docket's own publisher: `terminal-publish.sh`'s
  `restamp_build_artifacts` re-renders the `docket:backlink` block on merged `plan:`/`results:`
  files *after* the merge. The rule read as obviously true and had a counterexample in the same
  repo, in code the change's own close-out path would run. Fixed by carving out the generated block,
  mirroring how `## Artifacts` is already scoped. **Before stating an unconditional rule about your
  own system's artifacts, grep for the code that writes them** — the exception is usually already
  shipped, and prose is the only place it is missing.
