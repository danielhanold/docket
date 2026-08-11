---
slug: plan-supplied-test-code-is-unverified
hook: "Test code a plan hands you is unverified code, not an oracle — prove the assert CAN pass, and mutation-test its own key."
topics: [testing, plan, guards]
changes: [94, 104, 112, 130, 133, 157, 168, 170, 173, 174, 194, 113, 212, 211, 203, 228, 226, 234, 237, 200, 244, 242, 286, 260, 284, 247]
created: 2026-07-19
updated: 2026-08-11
promotion_state: candidate
promoted_to:
---

## Apply
A plan's asserts arrive with the authority of the plan and none of its scrutiny: the plan author
wrote them against an implementation that did not exist yet, and nothing has ever executed them.
Treat every supplied assert as a draft under test until you have shown two things:

1. **It can pass at all.** An assert that is unsatisfiable by *any* correct implementation reads as
   a real regression, and the honest response — stop, report BLOCKED — burns a cycle chasing a
   defect in the test rather than the code. Before debugging the implementation against a red
   supplied assert, check the assert's own field indices, ranges, and expected values against the
   real output format.
2. **Its key is load-bearing.** Mutation-test the assert by deleting the thing it exists to check.
   If it stays green, it is decoration — and a *fixture* can hide this as easily as the assert can:
   a fixture set too narrow to distinguish two orderings makes a sort-key check pass under a
   coincidence rather than under the rule.

This is the defect class the plan author structurally cannot see, which is why it survives to the
implementer. It is not a reason to distrust plans — it is a reason to run the plan's tests as code.

## War story
- 2026-07-19 (#94, PR #108) — Three distinct defects in one plan's supplied test code, all caught
  during the build:
  - The membership-parity assert used awk field indices off by one against
    `change <id> <status> <readiness> <slug>`, leaving `exp_ready` unconditionally empty — the
    assert was **unsatisfiable by any correct implementation**.
  - A producer sentinel scoped itself with `awk "/^main\(\)/,/docket_preflight/"`, whose range
    closed on the explanatory **comment** containing that string, so it never reached the code.
    Presented as a false BLOCKED. (See [[specified-but-unreachable]].)
  - The lowest-`id` tie-break assert could not detect deletion of its own `-k3,3n` sort key: every
    fixture id was two digits, so `sort`'s lexicographic fallback happened to agree with numeric
    order. Fixed with a tie crossing a digit-width boundary (ids 9/10), then confirmed red under
    mutation — the fixture, not the assert, was what had been hiding the hole. (See
    [[guards-are-code]].)
- 2026-07-20 (#104, PR #113) — Three more, in one plan, caught by running the supplied tests as code:
  - `has_finding "$out" malformed-id "?"` was **vacuous**: the helper built an unescaped ERE and `?`
    is a quantifier, collapsing the pattern to `^malformed-id\t` and matching any line of that
    check-id. It was green even against the **pre-implementation baseline**. Fixed at the helper's
    *definition* (literal `case` match + here-string, which also removed a `printf | grep -q`
    pipefail hazard) rather than at the call site — `cid` can legitimately be `?`, so a call-site
    patch would have been re-hit by later tasks. **Fix a shared helper's hazard where it is
    defined; a call-site fix leaves the trap armed for every other caller.** (See
    [[escape-ere-metacharacters-in-key]].)
  - Two asserts used a literal `\t` inside `grep -E`, which **BSD grep does not interpret** —
    rewritten to the repo's portable `grep -E "$(printf '^x\ty\t')"` idiom.
  - The plan's own Step-2 *verification command* was broken: anchored to line start, it missed
    `emit` calls following `||` guards and found only 9 of 11 check-ids. A plan's verification
    commands are unverified code too — an under-counting verifier reports a gap that does not exist
    (or, reversed, misses one that does). A corrected unanchored derivation confirmed zero gaps.
- 2026-07-22 (#112, PR #118) — **The control case: what it looks like when this rule is paid up
  front.** All three per-task reviews and the whole-branch review came back with zero Critical or
  Important findings — an outlier in this backlog, and the change's own results file explains it
  rather than celebrating it. Two things were done before any subagent was dispatched: the fixtures
  were **fully specified in the plan**, and **the plan's own values were checked against the running
  code**. The consequence is a shift in where the effort lands — verification went into *proving the
  guards fire* (an 18-cell mutation matrix, every cell matching prediction) instead of into
  repairing asserts mid-build. The three preceding changes in this family each spent multiple review
  rounds discovering that supplied test code was wrong; this one spent that budget on mutation
  evidence and shipped clean.
  Two smaller instances of the same discipline in the same change. (a) A **forward claim** in Task
  1's header asserted how `s8`/`s9` would behave under mutations — a claim about fixtures that did
  not yet exist — and it was checked against the completed matrix at Task 3 rather than left
  standing. (b) A **fixture comment's stated reason** was verified both ways: the comment says
  `.docket.yml` is kept (key absent) for main-mode shape consistency, *not* because omitting it
  breaks resolution, so the reviewer built the fixture both ways and ran the resolver to confirm
  both halves. A comment encoding a false reason is what [[verify-the-claim]] exists to stop, and
  the cheap version of that check is running the alternative once.
  The generalizable claim is narrow but real: this family's defects are **front-loadable**. The cost
  of checking a plan's asserts against running code before dispatch is bounded and paid once; the
  cost of discovering them at review is a round per defect, and #102 shows that reaching five.
- 2026-07-28 (#130, PR #133 — merged) — **The plan's own guard source violated the invariant that
  guard enforces.** The plan embedded the complete guard text for verbatim transcription, and that
  text wrote `{0,600}` literally inside four header comments. The guard is deliberately not
  self-excluded, so transcribing it faithfully made the plan's own success criterion (`exit=0`, zero
  `NOT OK`) unreachable — staging the file produced a self-scan failure naming those exact four
  lines. The implementer diagnosed it, reworded only those four comments, and reported the deviation
  rather than silently diverging; the reviewer diffed the shipped file against the plan's block
  line-for-line to confirm nothing else moved.

  Generalize past the incident: **a guard that scans its own population makes the documentation of
  the pattern it forbids a live constraint on the guard's source.** The plan author has to write the
  header comment without using the very literal the header is about — and a plan that hands over
  verbatim guard text has to have been run against the tree it will land in, not just read.
- 2026-07-28 (#133, PR #134 — merged) — **A rc-capture idiom that made every rc assertion vacuous.**
  The plan's `probe()` helper wrote `out="$(cmd; printf 'x')"; rc=$?` — `$?` is `printf`'s status,
  not the target function's, so `rc` was always 0 and the whole validator block asserted nothing.
  The implementer diagnosed it, fixed the **test** only, and left the library exactly as specified;
  the reviewer independently reproduced the diagnosis and confirmed the library had been right all
  along. The sharp edge: the *same* `printf 'x'` idiom appears in the shipped resolver and is
  correct there, because that caller branches on a returned reason token and never on `$?` — the
  idiom's real job is guarding an empty trailing line from command substitution's newline strip.
  Same three characters, load-bearing in one place and silently fatal in the other; which it is
  depends entirely on whether the caller reads `$?`.
- 2026-07-28 (#157, PR #136 — merged) — **A plan's mutation command is test code too, and this one
  inverted its own oracle.** The plan supplied
  `perl -pi -e 's/.../[ "$_major" -ge 0 ]/'` to relax a Bash-major floor and prove the guard reddens.
  `$_major` is interpolated by *Perl*, not the shell, so the substitution collapsed the line to
  `[ "" -ge 0 ]` — which breaks validation outright instead of relaxing it, and left the targeted
  asserts green. Run literally the recorded oracle was invalid; re-run with the shell variable
  escaped, the property held (3 reddens, both targets). The general shape: a mutation command is the
  *only* evidence a guard is load-bearing, so a broken one produces a confident, fully-documented
  false negative — and it is the one piece of plan-supplied code nobody re-reads, because its output
  is the reassurance you were looking for. Escape-level bugs are the common failure (a shell `$` read
  by `perl`/`sed`/`awk`); the cheap check is diffing the mutated file, not just reading the exit code.
  Same change, same family: 0146's overlap-invariant guard **always inserted separators**, so it could
  never exercise the unseparated adjacency it existed to catch — vacuous by construction. Fixed with
  vacuity evidence before shipping rather than after.
- 2026-07-31 (#174, PR #141 — merged) — **The plan's scripted RED step could never have gone red.**
  Each task was to begin by proving the new assertions fail against the un-refactored helper, on the
  premise that an undefined template global would abort the file under `set -u`. It does not: every
  dereference sat inside a command substitution, so `set -u` killed only the **subshell** and the
  parent continued with an empty string — the file ran to completion, green. The implementer
  substituted real mutation evidence against the implemented helper (delete the `remote set-url`
  line; symlink the origin; symlink the work tree; drop a seeded ADR) and confirmed each new
  assertion reddens under at least one mutation. Two honesty notes the same review surfaced, worth
  copying: the mutation's blast radius was reported per file rather than in aggregate — the
  URL-rewrite mutation reddens 121 assertions in one file and exactly **1** in another, and that one
  merely restates the implementation line, so "the URL rewrite is the correctness core" was true of
  three files, not four; and two assertions per block were flagged as unfalsifiable given `cp -R`
  semantics — harmless, but they inflate the apparent strength of the block they sit in. Counting
  the assertions that *cannot* fail is part of reading a mutation matrix honestly. The same build's
  inert-optimization defect is in [[optimization-needs-a-measured-oracle]].
- 2026-07-31 (#168, PR #140 — merged) — **A supplied awk helper that returned failure
  unconditionally, aimed at the change's central negative invariant.** The plan's `fm_has` used
  `exit 0` inside a rule, but awk still runs `END` after a rule-body `exit`, and that `END` block's
  `exit 1` overrode it — so the helper returned 1 no matter what the frontmatter contained. Every
  `! fm_has` assert would therefore have been permanently green: a decoration guard sitting exactly
  on the "no cross-harness model pin leaked into this wrapper" property the whole change existed to
  establish. Note the asymmetry that makes this class dangerous — a helper stuck at *failure* is
  loud under positive asserts and **silent under negated ones**, so the same bug is caught instantly
  in one position and invisible in the other. Two further plan defects in the same build: test
  snippets called sandbox helpers (`mk_repo_cfg`, `run_sync`) that do not exist in the target test
  file, with a wrong generated-path shape; and Task 6 was told to leave the Codex TOML value asserts
  unchanged when they could not stay — Codex has no sidecar block, so all twelve Codex wrappers now
  emit no pin and the asserts had to be re-pointed at absence. (Change #0169 must flip them back to
  value asserts when it lands the Codex mapping.)
- 2026-07-31 (#173, PR #142 — merged) — **Two supplied snippets were flagged defective by the plan
  itself; the implementers found a third that was not.** Task 3's `probe()` helper wrote its fixture
  under `runners: codex:` while dispatching `--runner probe`, so every value assert would have read
  `<unset>` regardless of whether the fix worked — permanently red, aimed at the change's own
  central behavior. The sharper half of this entry is not plan-authored at all: a *review-fix*
  assert written during the build over-escaped an apostrophe inside a double-quoted string
  (`typo'"'"'d`), producing an unterminated quote that aborted `tests/test_sync_agents.sh` at line
  1832 with **zero `NOT OK` lines and `rc=2`**. The file looked clean by the metric anyone scans
  first; what gave it away was the *assert count* (526) coming in lower than expected. A suite whose
  pass/fail summary is derived from failure lines cannot distinguish "nothing failed" from "the
  interpreter stopped reading" — so when a test file is edited, compare the assert total against the
  previous run, not just the failure count. Same discipline as the plan-supplied case, applied to
  the code you just wrote yourself.
- 2026-08-01 (#170, PR #149 — merged) — **The pair extension: a plan that supplies both the assert
  and the prose it greps for has verified neither against the other.** Four tasks in one plan (3, 4,
  5, 6) shipped a hand-written assert and a suggested sentence written in the same pass, mismatched
  every time: an `evidence`-within-30-characters proximity regex the suggested wording could not
  satisfy; a case-sensitive `important` against prose that capitalized it; a bolded `**no-op**`
  whose own asterisks broke the pattern meant to find it; and a whole-file README grep loose enough
  to match anywhere. Each was resolved by keeping the assert and fixing the prose, or tightening the
  assert where it was genuinely wrong — never by weakening one to meet the other.
  The generalization is the sharp part: the existing rule says treat a supplied assert as a draft
  under test. When the plan also supplies its **haystack**, there are *two* unverified artifacts and
  the only evidence either is right is that they agree — which the plan author never checked,
  because writing both in one pass feels like writing one thing. Run the supplied grep against the
  supplied prose before writing any implementation; it costs one command and it is the whole check.
  Same branch, adjacent class: the whole-branch review found three near-vacuous asserts — a bare
  `|halt` alternation that passed before the prose it guarded existed, a `grep -qiE "once|one run"`
  over a 900-line README, and a negated grep over an `awk` range that goes permanently green if the
  fence markers it depends on are renamed. Each was tightened with a paired non-vacuity anchor and
  mutation-proven. A negated grep over a derived haystack is the same silent-under-negation
  asymmetry #168 recorded above, one layer up: the haystack, not the helper, is what empties.
- 2026-08-02 (#194, PR #153 — merged) — **Neither defect was in an assert; both were in the
  plan's *operating instructions* around the asserts, and both fired on every task.** (a) The
  verification step piped each suite to `tail -1` and expected the literal `PASS`. 46 of this
  repo's 76 suites legitimately end with `ALL PASS`, `ALL OK`, or their last assert line, so run
  verbatim the command reports 46 failures against a fully green tree — a *false red* across most
  of the suite, which is the reading that gets a correct build abandoned as broken. Every run in
  this build was keyed on **exit status** instead. (b) The mutation-proof step undid its mutation
  with `git checkout -- <file>` while the task's own edit was still uncommitted, so running it
  literally discards the work, not the mutation. Both implementers hit it live and both recovered
  independently — one staged first and re-applied, the other kept a `cp` backup.
  The generalization extends the rule past test code: **a plan's harness — how to run the suite,
  how to read its verdict, how to undo a mutation — is unverified code with no assert of its own
  and no reviewer, because its output is procedure rather than a result.** Both defects here are
  in the class that cannot redden anything: one manufactures failure where there is none, the other
  destroys the evidence. Cheap checks, in order: run the verdict command once against the untouched
  tree before trusting a single red it reports, and undo a mutation from a copy you made yourself,
  never from git, whenever your own edit is uncommitted.
- 2026-08-03 (#113, PR #154 — merged) — **A supplied mutation that produced an unparseable script,
  which would have shown up as a fully green run.** The plan's mutation B deleted only the `emit`
  line from leg A's results arm, leaving `if …; then` immediately followed by `fi` — a bash
  **syntax error**. The mutated `board-checks.sh` would have died before running any check, so
  every fixture goes green *for the wrong reason* and the "this arm survives" assert fails
  confusingly while the ones that matter never execute. The worker caught it, replaced the mutation
  with one removing the whole `if`/`emit`/`fi` arm, and added `bash -n "$ARMSCRIPT"` guards to
  mutations B and E.
  The generalization sharpens #157's entry above. A broken mutation command is dangerous because it
  fabricates evidence; a mutation that yields a **syntactically invalid** script is worse, because
  the failure mode inverts with the assert's polarity — a script that cannot parse produces no
  output at all, which reads as "no findings emitted," which is precisely what a
  *deletion-shaped* mutation is supposed to prove. The mutation and its oracle agree, and both are
  measuring nothing. Cheap check, now house practice here: **`bash -n` the mutated artifact before
  reading its result**, for every mutation whose shape is a deletion.
- 2026-08-05 (#212, PR #161) — The plan handed the worker a finished guard script for the seven
  swept stop sites. It failed **twice** against the landed files, both times invisibly-as-written.
  (1) The `SITES` anchor `Then you stop — review is not yours.` matched nothing, because an earlier
  task in the same plan compressed that body and wrapped the sentence across two lines. (2) The
  `clause_near` matcher was line-literal, so `docket-status`'s clause — wrapped between
  `dispatched as a` and `subagent,` — read as **absent** on a file where it was present and
  correct. Both were found by execution, neither by reading.
  The sharpening this adds: a plan-supplied guard is written against the file as it exists **at
  plan time**, and a multi-task plan is a machine for invalidating exactly that. When earlier tasks
  in the same plan reflow, compress, or move the prose the later guard anchors on, the guard's
  staleness is *caused by the plan it came in*. Prose anchors are the fragile case — run the guard
  against the real post-task tree before trusting either polarity, and prefer shape-based matchers
  over line-literal ones anywhere text can wrap.
- 2026-08-05 (#211, PR #160) — the plan's fixtures for `aborted-run`'s new leg C advanced a fixture
  origin with a commit under `docs/results/`. That path sits inside `RESULTS_DIR_REL`, so the
  advancing file rode onto the feature branch and fired **leg A** — breaking an id-scoped silence
  assert in one fixture, and in the mutation repo doing something worse: it would have made the
  mutation guarding this change's *central* design decision (both integration bases excluded) pass
  on a leg-A finding even with that predicate deleted. **Vacuously true, and green.** A second
  plan defect in the same fixture family — a single-commit advance whose tip carried a real
  wall-clock date — left another mutation unable to fire at all against the harness's deliberate
  clock skew. The generalization this adds: when a fixture must be *neutral*, check its paths
  against every directory the system under test already treats as meaningful — a plan author
  picking an "obvious" filename has no way to know which paths are load-bearing. And a mutation
  that passes proves nothing until you have shown it can *fail*: neutering each mutation's `sed`
  to `cat` and re-running is what surfaced both defects here.
- 2026-08-06 (#203, PR #163) — the results file names this finding as "earning itself," and three
  distinct plan-supplied defects fired in one branch. The plan's `## Verification` block asserted
  `grep -c 'git-state postcondition'` would return **2**; the delivered design deliberately never
  repeats the term, so the true value is **1** — an expected value derived from an imagined
  implementation, not a measured one. The plan also supplied its guard as literal shell, two lines
  of which were unusable as written: `flatten < f | grep -q` is the producer-piped-into-an-
  early-exiting-consumer hazard under `set -o pipefail` (see [[pipefail]]) and goes intermittently
  red at 141, and a comment anchored as `path:193` is exactly the filename-plus-line form
  `tests/test_comment_anchor_style.sh` rejects. **Both were caught by execution, neither by
  reading** — the plan's shell was syntactically fine and locally plausible in each case. The
  generalization: a plan author writes guard shell without running it against the repo's own
  meta-suite, so plan-supplied test code must clear the *project's* conventions checks, not just
  produce the right answer.
- 2026-08-07 (#228, PR #167) — the plan's Task 2 shipped `assert "empty suite runs zero tests"
  '[ ! -s "$empty_execution_log" ]'` over a fixture directory created empty with the log truncated
  immediately before the run. Nothing could ever write to that log, so **no reachable mutation
  could redden the assert** — it read as coverage while carrying none, and was deleted at review in
  favor of a mechanism pin. The tell was written down in the plan itself: it predicted the assert
  would "stay `ok` under this mutation" and shipped it anyway.
  What this adds: alongside "can it pass at all," ask the mirror question — **can it fail at all?**
  A supplied assert whose own plan text concedes it stays green under the mutation it is paired
  with is green-by-construction, and the concession is the evidence. See
  [[assert-pins-outcome-not-mechanism]].
- 2026-08-07 (#226, PR #168) — A *new shape* of this: the plan supplied the site-C assert
  `grep -qiE "unavailable|\*\*no\*\*" <<<"$c_row"` for a routing-table row. The row's own
  *Branch + fix loop* column independently says `**no**`, so the alternation stayed satisfied with
  the fix-in-branch exemption — the thing the assert existed to guard — deleted. The mutation
  proved it. Fix: split into two cell-scoped asserts using `[^|]` so each pattern stays inside the
  cell that owns its claim. The generalization: an alternation is only as strong as its weakest
  branch, and a *plan* author writing against a table that does not exist yet cannot see which
  branch some other cell will satisfy for free. Same session also caught a plan-supplied generic
  `/^## /` awk terminator that sliced a section short (see
  [[section-slice-needs-a-named-terminator]]) and a plan-supplied `git checkout -- <file>` restore
  idiom that destroyed an uncommitted edit mid-mutation-test (see
  [[mutation-restore-needs-a-backup-copy]]) — three defects, one plan, all in supplied *procedure*
  rather than supplied code, which is the part that reads least like code.
- 2026-08-07 (#234, PR #169) — **The rule extends past the plan to the *reviewer*: a pattern
  suggested in a review finding is unverified code with the same authority problem.** A minor
  finding proposed `returned in \*\*[0-9]+s\*\*` to catch duration figures regrowing on the kept
  surface. Validated against the real prose it matched only three of the four narratives — one
  narrative line-wraps between "returned in" and `**19s**`, and `grep` is line-oriented, so a paste
  of exactly the sentence the guard was written to catch would have slipped past it. Adopted instead
  a pattern keyed on the figure shape alone (`\*\*[0-9]+s\*\*`). Same wrap-fragility as #212's
  prose anchors, arriving through the review channel rather than the plan.
  Two smaller procedure defects in the same branch, both in the class that fabricates a reading:
  a `perl -0pi -e 's/^…$/…/m'` mutation silently substituted **nothing** and produced a false "the
  guard did not redden" — two workers hit it independently, so *confirm the mutation landed (diff
  the file) before believing the guard's response*; and the plan's own verification greps were
  written `^(NOT )?ok` while this runner prints `NOT OK` uppercase, so a RED line was invisible to
  the filter rather than reported.
- 2026-08-08 (#237, PR #176) — **The plan was docket's own, and its supplied guard was vacuous by
  quoting.** The assert proving `runner-dispatch.sh` no longer `exec`s was written with a `\$`
  inside an `eval`'d double-quoted string; the escape collapsed to a bare `$`, which `grep -E` then
  read as an **end-of-line anchor**, so the pattern matched the *unchanged* file and the guard was
  green before the change existed. Replaced with a shape-anchored static guard plus a runtime
  `$PPID`-vs-`$$` check, both red-before / green-after. Nothing new in the mechanism — what this
  entry adds is the provenance: this finding is already `promotion_state: candidate` and the plan
  that tripped it was authored inside the same repo that carries the finding. A rule that must fire
  unprompted does not fire because it is written down nearby.
- 2026-08-08 (#200, PR #181) — Three plan-supplied test defects in one branch, each caught by the
  builder that owned the task. (a) Mutation O's expected count of the inserted capture line was
  `2`; the real count is `1`, because the awk **replaces** the `done < <(…)` line rather than
  inserting beside it — derived empirically on a hand-built mutant, and deliberately not weakened
  to `-ge 1`, since the sibling arm reads `0` through the same grep and the exact count is the only
  thing separating them. (b) Mutation 4's anchor assumed a single top-level `for f in ` walk; the
  script has two, so the assert was re-pinned as a before/after pair rather than a bare literal.
  (c) Mutation 4's own **non-vacuity** assert was wrong for its arm: the plan wrote `[ -n "$m4out" ]`,
  but the fixture repo is built so scalar-form is the only check that fires — empty output is the
  *correct* result, and the assert failed on first run. The replacement re-runs capturing stderr
  (`2>&1 >/dev/null`) and demands empty stderr plus rc 0 — exactly what the runner's `2>/dev/null`
  was hiding. The general shape: a plan's non-vacuity assert is as unverified as the assert it
  guards, and it is written by the same author who could not run either.
- 2026-08-08 (#244, PR #184) — **The anti-vacuity floor was itself vacuous.** The plan supplied a
  fixture asserting the renderer had run by grepping the output for `docket:artifacts:end` — but
  the fixture ships that marker in its own body, and with every optional read empty the rendered
  block is byte-identical to the pre-render one. The assert could not distinguish "rendered" from
  "never touched". Worse, the render call discarded its exit status, so a renderer crashing with
  exit 2 left all three asserts green. Carried over verbatim from the plan because it *looked* like
  a floor. A floor has to be a claim the un-run case fails.

- 2026-08-08 (#242, PR #186) — five of six tasks had to correct plan-supplied code before it could
  be trusted: a slice helper with no terminator on one surface, a path predicate that trusted a
  symlink target's spelling, a `--check` leg gated on the weaker of two predicates, and an assert
  that flattened a whole SKILL.md and matched its keywords from unrelated paragraphs (mutation-proven
  vacuous). Each was found by building the thing, never by reading the plan.

- 2026-08-10 (#286, PR #192) — both defects in one plan were in its supplied test code, and both were
  found only by running it. The `loop_sec` slicer terminated on `/^#/`, which closes on the fenced
  block's own first *comment* line, so the slice was ~3 lines and its own `>= 20` non-vacuity anchor
  could not pass against any correct implementation. The supplied fixture shadowed only `sleep`,
  which would have made one named mutation spin for the fixture's real 5-minute budget — twice —
  against a plan claiming "milliseconds"; the harness now shadows `sleep` and `date` both.

- 2026-08-11 (#260, PR #198) — **The same defect three times in one plan, and the frequency is the
  finding.** The plan's mutation-probe harness carried a `grep -c` landing-check whose counter
  literal cannot change across its own mutation — so the probe reported `MUTATION DID NOT LAND`
  against a mutation that had landed perfectly. Three separate workers hit three separate instances
  of it in this one branch; each diagnosed and corrected its own independently, which is the tell
  that the defect is structural rather than authorial.
  Count it across the drain: **#286** (two defects in its supplied test code), **#281** (Task 1
  Probe 1 was not a valid probe), and **#260** (three probe-harness instances) — five recurrences of
  the plan-supplied-probe class across four changes, all inside one drain. Every previous entry in
  this family generalizes to *how the implementer should treat supplied code*; this one generalizes
  the other way, to **where the code should come from**. A landing-check counter is not a thing each
  plan author should be re-deriving per probe: it is a fixed, mechanical part of what a mutation
  probe *is*, and the plan author is structurally the person least able to verify it — no
  implementation exists yet to run it against. The conclusion the build run drew, recorded here as
  the rule: **the counter belongs in the probe TEMPLATE, not in each plan author's care** — a
  shared, tested mutation-probe harness the plan cites by name and parameterises, so the class stops
  recurring instead of being caught five times. That is a systemic tooling gap, not a per-plan slip.
  See [[guards-are-code]], [[mutation-target-needs-a-forced-exit]],
  [[mutation-restore-needs-a-backup-copy]].

- 2026-08-11 (#284, PR #199) — **Seven distinct defects in one plan's supplied probe code, and the
  sharpest one gives the class a mechanical remedy.** All seven were caught by the workers and none
  shipped. The one that matters: a probe *passed its own landing check while silently deleting 128
  lines*. Its check was a **token count** — the mutation's marker token was present the expected
  number of times, so the probe declared the mutation landed — but the edit that produced that token
  state had also removed most of the file, and nothing in the check could see it. A probe that
  cannot distinguish "the intended one-line mutation landed" from "the file was gutted" is not
  measuring the mutation at all; the assert that then reddens is evidence about a file that no
  longer exists, not about the guard under test. The branch-wide remedy adopted, and the
  generalizable rule: **a mutation probe checks exact removed/added LINE COUNTS, never a token
  count.** Line counts bound the edit in both directions — an over-broad edit fails the check even
  when the token arithmetic works out, which is exactly the failure a token count is blind to.
  This is the sixth-plus change in this drain hit by the plan-supplied-probe class, and the build
  called it the dominant failure mode of the drain. It is directly actionable by **#0292** (the
  shared, tested mutation-probe harness): the line-count landing check is the harness's job, not
  each plan author's — the same conclusion #260 reached one entry above, now with the specific
  predicate the harness should implement.

- 2026-08-11 (#247, PR #200) — **Roughly twenty defects in one plan's supplied test code, and one of
  them was not merely wrong but destructive.** The running tally for this class is now six changes in
  this drain — **#286, #281, #260, #284, #247** — and this is the highest single-change count yet.
  **The instance that changes the risk profile: a fixture cleanup that deleted a path outside the
  fixture.** It ran
  `rm -rf "$(git -C "$MW" rev-parse --git-dir)/rebase-merge"`. Under `-C`, `rev-parse --git-dir`
  prints a **relative** `.git`, so the argument resolved against **the test's own cwd**, not `$MW`.
  Run from the repo root while the developer's real tree was mid-rebase, it would have deleted their
  live rebase state. Every previous entry in this family costs a wasted diagnostic cycle; this one
  costs the developer's working tree. The rule that falls out is narrow and mechanical: **a fixture
  cleanup's `rm -rf` argument must be an absolute path derived from the fixture root, never from a
  command whose output can be relative** — and `git -C <dir> rev-parse --git-dir` is exactly such a
  command. Pair it with `--absolute-git-dir`, or build the path from `$MW` yourself.
  The other severe one: `logical_lines` never passed its `FILE` argument to awk, so it read stdin —
  the Group A scanner would have shipped **permanently vacuous**, green and checking nothing. Also in
  the same plan: a permanently-vacuous rebase-state assert whose `mkdir -p "$wt/.git/rebase-merge"`
  silently does nothing because a linked worktree's `.git` is a *pointer file* (this same trap was
  hit and fixed **three separate times** on this one surface, which is the tell that it belongs in a
  shared helper rather than in each author's care), a `pipefail` producer-into-`grep -q` violation, an
  unsatisfiable mutation-table row, a `--autostash` repo grep failed by the implementation's own
  explanatory comment, a wrong branch variable in main-mode, and inverted redirections that discarded
  `rebase --abort`'s errors.
  **Four mutation probes were green on first run** — the guards they tested were decoration — and
  each was repaired by adding a *control*, never by explaining the green away. A fifth was green until
  a `nosync` fixture was added; a sixth silently failed to apply and produced a meaningless green,
  caught only by the before/after `grep -c` this repo's rules already require.
  This change is strong evidence for **#0292** (shared, tested mutation-probe harness — now priority
  **high**): the vacuity controls, the landing check, and an absolute-path cleanup contract are all
  harness properties, and this branch re-derived every one of them by hand.
