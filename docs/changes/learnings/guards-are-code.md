---
slug: guards-are-code
hook: "A guard is code — mutation-test it (strip the feature, watch it go red) or it is decoration."
topics: [testing, sentinels, mutation]
changes: [14, 15, 21, 36, 37, 64, 65, 67, 68, 69, 70, 71, 72, 73, 74, 84, 88, 91, 96, 101, 106, 107]
created: 2026-06-17
updated: 2026-07-20
promotion_state: promoted
promoted_to: AGENTS.md
---

## Apply
Anchor to the UNIQUE phrase the target clause owns ("before any push") and confirm
`grep -c` == 1 — never a keyword set, never a blunt `! grep`/`grep -q` over a literal that can
legitimately appear elsewhere. One assert owns exactly ONE clause; if two locations satisfy it,
split and mutation-test each in isolation, and when a mutation slips past, add an INDEPENDENT scan
rather than widening the first. Tokenize at the unit you claim to guard (the invocation, not the
line), prove the tokenizer SEES the whole document before trusting it (assert the unit COUNT it
found — a guard that parses nothing passes everything), and order-assert with byte offsets
(`grep -ob`) when both anchors can share a line. Prove an assert can FIRE before trusting that it
passes: run it against a tree where the guarded thing IS present. Never fence an assert behind a
condition DERIVED from the thing under test (`if grep -q "<the success line>"; then …`) — that fence
goes false exactly when the branch degrades, so the mutation makes the assert VANISH rather than
fail; assert unconditionally, and read the ok COUNT as part of the contract, because a mutation that
lowers it while producing 0 NOT OK is a vacuous guard announcing itself. Three ways it is vacuous by
construction — a pattern leading with `--` (always `grep -qF -- "$pat"`); an expected string you
INVENTED rather than copied from the producer's real output (`%q`-formatted, quoted, escaped); and
a probe value that COINCIDES with the default, so a fence/ignore test must always probe with the
NON-default value (and a change that moves a default must re-check every guard whose expected value
it just made ambient). Key a guard on SYNTACTIC SHAPE, never an enumerated spelling list — the
spelling it misses is the target file's own house idiom; when the load-bearing thing is a compound
spelling, assert ALL its bytes and DERIVE the literal from the token you already hold. Mutate the
REAL tree, not only fixtures — a fixture battery samples the shapes you already thought of. Any test
that `eval`s a command's output must clear the variables it asserts on first. An assert helper that
`eval`s the assertion BODY must run it in a SUBSHELL (`( eval "$2" )`) — a body carrying its own
control flow (`exit`, `for … || exit 1`) run in the current shell aborts the whole harness on first
failure instead of recording one NOT OK and continuing, hiding every later assert. Treat a surviving
mutant as a defect until proven otherwise (re-derive the anchor's count yourself, never trust an
implementer's narrative), and read an implementer contorting the artifact to pass an assert as a
signal the assert itself is wrong. Never RETIRE a guard on the claim that a new one subsumes it —
prove subsumption by mutation in BOTH directions, or keep both; when a new feature legitimately
violates an old absolutist sentinel, NARROW it to the load-bearing property (#74: retiring the last
`/docket-config.sh` spelling reddened a sentinel keyed on it — the fix was to narrow the pattern,
not drop the assert), and FOLLOW a call-site-pinned audit when the code is extracted into a shared
lib. A snippet the PLAN hands you is unvetted code: mutation-test it like any assert you wrote.

## War story
- 2026-06-17 → 2026-07-16 (#15 PR #32; #21 PR #34; #36 PR #47; #37 PR #48; #64 PR #75; #65 PR #74;
  #69 PR #77; #68 PR #78; #72 PR #79; #70 PR #80; #71 PR #81; #74 PR #82; #73 PR #83; #84 PR #90;
  #67 PR #91 — merged, one guards-are-code family) — A guard is code: mutation-test it (strip the feature, watch it
  go red) or it is decoration. Every way one has shipped GREEN while guarding nothing:
  (a) **Wrong anchor** — a broad keyword OR-set (`run the suite|validate|local`) latched onto an
  unintended EARLIER line.
  (b) **Double-guarded** — one grep satisfied by two independent clauses (a YAML config comment AND a
  prose sentence), so deleting the substantive prose left it green. #65 shipped two more, and in both
  the implementer's own mutation test had already gone green — its report rationalized that away as
  benign. #69 shipped two more still (`grep -qF "digest"` satisfied 3× over, `"board off"` 4×): the
  reviewer rewrote the skill's Final summary to literally say *"read from `BOARD.md`"* — the exact
  posture that change abolishes — and the assert stayed green.
  (c) **Wrong unit** — a sentinel grepped per LINE, so a logical line carrying a gated `--id` and an
  ungated `--adr` invocation side by side was whitewashed by the single `--enabled` present. #68's
  escape-hatch scan anchored `^\s*NAME)` and so missed pipe-combined `case` arms (`run|shell|eval)`),
  searched for a token that was a typo of the real one, and missed input laundered through a variable.
  #72's prose-invocation guard tokenized code units with an awk fence toggle anchored at COLUMN 0, so
  every fenced block nested inside a numbered list (indented fences — two whole reference files' worth)
  was silently dropped from the sweep: the guard read green over prose it had never parsed. An
  order-assert built on `grep -n` LINE NUMBERS is the same error one level up — it cannot order two
  phrases inside one paragraph, and the implementer "satisfied" it by splitting a sentence
  mid-paragraph (#14).
  (d) **Wrong surface** — #68's inventory sentinel derived its op set from the `WRAPPED_OPS` array,
  proving "the array matches the doc" but never "the DISPATCH matches the doc": a `case` arm
  hand-added outside the loop would route while set-equality still held. A structural guard over a
  single-source list does not guard the surface that CONSUMES that list.
  (e) **Stale state** — fence tests `eval`'d the config export without clearing the asserted variable
  first, and an aborting run emits NOTHING, so `eval ""` silently left the previous case's value in
  place and the assertion passed.
  (f) **False-RED / False-GREEN** — a `! grep "must-not-say-X"` fired on X in a legitimately
  *contrastive* clause, which a blunt absence-grep can't tell from the forbidden *adopted* posture;
  and a `grep -q "literal"` stayed green when a prose strip RELOCATED the must-preserve substring into
  an unrelated bullet, producing a false sentence.
  (g) **A list of spellings, not a shape** — #70's write sentinel enumerated the *spellings* of a
  tainted variable (`$out`, `${out}`) instead of describing its *shape*, so it was green on `${out:-}`
  — the guarded file's own house idiom, 14 occurrences in `docket-status.sh`, i.e. the single most
  likely real regression. Four rounds of fixture-driven hardening all shipped green; round five found
  the hole in minutes by injecting the regression into the REAL script. #70 also disproved its own
  spec's premise that the new sentinel SUBSUMED the older `REDIRECT_RE` scan — mutation testing showed
  the two are complementary, so neither may be deleted (ADR-0031).
  (h) **An assert that could never fire at all** — #71 shipped two. `! grep -qxF "BOARD_SURFACES="`
  asserted on a string the producer never emits (bash's `%q` renders an empty value as
  `BOARD_SURFACES=''`, never a bare trailing `=`), and `! grep -qF "$FLAG"` with `FLAG='--surfaces'`
  makes grep parse the *pattern* as an **option** and exit 2 — which the leading `!` then inverts into
  a green `ok`. Neither could redden under any mutation. The second was copied VERBATIM from the plan's
  own snippet. #73 re-hit it from the byte side: the plan's search literals omitted the JSON-escaped
  `\"`, so they could never byte-match the fragment they searched.
  (i) **A proper SUBSTRING of the thing you claim to guard** — #73's canonical-spelling assert anchored
  only the inner `${DOCKET_SCRIPTS_DIR:?…}` token, leaving the surrounding decoration
  (`"…"/docket.sh`) unguarded, so a mangle coordinated across the JSON fragment AND the guide's fence
  shipped green. The bytes that carry the meaning (the quotes, the `/docket.sh` suffix) were never
  asserted. Fixed to assert the full decorated spelling, CONSTRUCTED from the derived token rather
  than retyped.
  (j) **Vacuous once the default moves** — #84's coordination-key FENCE tests probed by writing
  `terminal_publish: false` into a machine-scoped layer and asserting the resolved value "stays true".
  That change flipped the default to `false`, so the *ignored* value and the *default* coincided: the
  assert would then pass whether or not the fence worked. The fixtures now probe with `true`, and a
  reviewer mutation-test confirmed they redden when the fence is defeated.
  (k) **Self-disabling fence — the asserts VANISH instead of failing** — #67 found a pre-existing test
  whose strong asserts sat inside `if grep -q "board inline changed pushed"; then …`. That fence is
  derived from the very branch under test, so it goes false *exactly* when the branch degrades:
  breaking the regen callback yielded **234 ok → 232 ok with 0 NOT OK**. The guard did not fail, it
  ceased to exist, and a pass/fail-only reading of the suite called that green. Two defenses, both
  cheap: assert UNCONDITIONALLY (the fixed test now yields 3 real reddens and also catches a sibling
  conflict-path mutation), and treat the **ok COUNT** as part of the contract — a mutation that lowers
  it without producing a NOT OK is a vacuous guard announcing itself. Never fence an assert behind a
  precondition the mutation you are testing can falsify.
- 2026-07-18 (#88, PR #100) — Two sentinel-harness defects caught in task review before merge, one
  per class this finding names. (a) **Harness could abort mid-run** — the `assert()` helper ran
  `eval "$2"` in the *current* shell, so an assert body containing `for … || exit 1` terminated the
  whole test on first failure instead of recording one NOT OK and continuing; fixed by isolating the
  eval in a subshell `( eval "$2" )` (`fail=1` still propagates from the parent-scope else branch).
  (b) **Wrong anchor** (class (a)) — the whole-backlog `/loop docket-implement-next` drain assert
  regex `…next$|…next[^0-9]` also matched the id-set bullet (the space after `next` is a non-digit),
  so it gave no independent regression protection over the pre-existing line — retargeted to a fixed
  string requiring the trailing backtick immediately after `next`. A Minor "never merges" assert that
  matched a pre-existing Quickstart sentence was likewise scoped to the new subsection's phrasing.
- 2026-07-19 (#96, PR #102) — **A guard keyed on a SPELLING silently stops discovering sites, and
  nothing reddens.** The call-site guard found its sites by grepping the bare `$SKILL_X` sigil, so
  rewriting one site to the equally-valid `${SKILL_X}` dropped it from discovery entirely — no assert
  failed, because a site that is never discovered is never asserted against. The only thing that
  noticed was the `checked >= 5` vacuity floor, which is the weakest possible detector: it protects
  only while the real site count stays at 5, and stops the moment a legitimate 6th site lands. Two
  rules. (a) `AGENTS.md` already carried this exact class — *"key a guard on shape, never a
  spelling"* — and it still slipped through, so a promoted rule is not self-enforcing; the mutation
  test is what enforces it. (b) The mutation to run on a *discovery* guard is not "strip the feature"
  but **"rewrite a site into an equivalent spelling and watch `checked` drop"** — for any guard that
  finds its own inputs, site-count is part of the contract and a silent decrease is the failure mode.
  Fixed by widening the pattern to both spellings with asserts pinning each. Documented in the same
  pass, unfixed: a token-presence marker is satisfied from any position (a parenthetical would pass),
  and `checked` counts matching *lines*, so one line invoking two role skills passes on one marker.
- 2026-07-19 (#91, PR #104) — **A gate whose over-correction is also a bug needs a TWO-SIDED proof.**
  `mint-stub.sh`'s clean-tree precondition guards a `reset --hard` in the shared `.docket` worktree.
  Both directions are real defects: no gate wipes another agent's uncommitted work (reproduced, with
  the script still exiting 0), and an over-broad gate keyed on plain `git status --porcelain` counts
  untracked files, so a stray `.DS_Store` hard-fails the mint on exactly the contended path the
  feature exists for (also reproduced). A one-sided test — assert dirty fails, or assert clean passes
  — blesses whichever error it does not probe. The rule generalizes past clean-tree checks: whenever
  a guard can fail by being too *loose* AND by being too *tight*, mutation-test both directions, and
  read "the assert passes" as evidence about one side only.
- 2026-07-20 (#101, PR #109) — **RETARGETING an assert to a new file makes it a new, unproven assert —
  and re-opens two vacuity modes this finding already names.** Four asserts were moved off this repo's
  `.docket.yml` onto `.docket.yml.example` when the former stopped being the user-facing documentation
  surface. Two went vacuous the moment they landed: one asserted `finalize.require_pr_approval` was
  `"false"`, but the reader returns the DEFAULT `"false"` for an ABSENT key — the
  probe-coincides-with-the-default trap, re-entered through a *move* rather than a fresh write; the
  other two matched the `learnings` `enabled`/`cap` values with UNANCHORED regexes that hit anywhere in
  a file whose content had entirely changed. Fixed with an explicit active-key assert and a
  block-scoped awk; the review independently re-derived and mutation-confirmed all four. The rule:
  a retargeted assert inherits none of its passing history, because the new target has different
  absent-key defaults and different surrounding bytes — re-run the mutation against the NEW file.
- 2026-07-20 (#107, PR #110) — **The EXTRACTOR is part of the guard, and it fails by silently
  narrowing the corpus rather than by erroring.** A README-vs-example correspondence guard fed six
  asserts from a helper that took the FIRST ```` ```yaml ```` fence in a section, with nothing
  asserting the section held only one. A reviewer added a second fence carrying
  `metadata_branch: BOGUS` and a `nonexistent_key` — **all six asserts stayed green**, because the
  bogus content was never in the corpus the guard parsed. This is class (c) *wrong unit* arriving
  through selection rather than tokenization: the fix is an explicit **occurrence-count assert on
  the extraction site itself** (`section has exactly one yaml fence … got 2`), not a wider pattern.
  Two neighbours of the same shape in the same guard: (a) the flattener's key regex rejected
  anything outside `[A-Za-z_][A-Za-z0-9_]*`, and the vacuity floor counted its POST-filter output —
  so `some-new-key: yes` was dropped by the filter and invisible to both the floor and the loop
  (closed by a raw-vs-flattened line-count cross-check: **whenever a count is taken downstream of a
  filter, also assert filtered-vs-raw, or the filter's own misses are unobservable**); and (b) the
  section boundary was bounded on `^### ` only, so a compound edit let a LATER section's content
  satisfy both pointer asserts — a sentinel satisfied by a neighbour's window, which the guard's own
  comment claimed to defend against. Widening that boundary to `^#{1,3} ` then broke on the YAML
  sample's own leading `# .docket.yml — …` comment line, simultaneously valid YAML-comment and
  valid markdown-H1, truncating the section to **zero** keys until the heading-exit check was gated
  off inside fenced blocks. General rule: before trusting a guard that extracts its input, prove the
  extraction is UNIQUE, prove its boundaries close on what you aimed at, and count the corpus at
  every stage it is narrowed.
- 2026-07-20 (#106, PR #111) — **One mutation does not prove a SET of asserts. Record which ones
  redden, and derive from the code's own structure which mutation each assert can even respond
  to.** Five asserts pin the `finalize.test_command` `auto` sentinel's cross-layer masking.
  Mutation 1 — collapse the sentinel per-layer instead of after the precedence chain — reddened
  `s4`/`s5` and left `s6` GREEN. Not decoration: precedence is `local > committed > global`,
  first non-empty wins, so `s6`'s sentinel sits *below* the winning rung and `:-` short-circuits
  before it is ever consulted. `s6` is structurally IMMUNE to that mutation and needed its own
  (Mutation 2, blanket "any layer says `auto` ⇒ unset"), under which `s6` alone reddens and all
  four `s4`/`s5` asserts stay green. Reading the run as "the mutation went red, the guard works"
  would have shipped one of the two properties unproven — the asymmetry is forced by precedence,
  so it is derivable up front rather than discoverable by luck. Two practices made it legible and
  belong in any mutation record: **assert-total conservation** (219+2 = 221; 220+1 = 221 — proving
  no assert VANISHED rather than failed, class (k)) and naming WHICH asserts redden per mutation
  instead of only the count. The change existed at all because a *comment* was the sole assertion
  of this property and had already shipped it backwards once (`a9da1e2`, caught only at review) —
  a comment is a claim, not a guard (see [[verify-the-claim]]).
