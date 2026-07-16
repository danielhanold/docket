---
slug: guards-are-code
hook: "A guard is code — mutation-test it (strip the feature, watch it go red) or it is decoration."
topics: [testing, sentinels, mutation]
changes: [14, 15, 21, 36, 37, 64, 65, 68, 69, 70, 71, 72, 73, 74, 84]
created: 2026-06-17
updated: 2026-07-16
promotion_state: candidate
promoted_to:
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
passes: run it against a tree where the guarded thing IS present. Three ways it is vacuous by
construction — a pattern leading with `--` (always `grep -qF -- "$pat"`); an expected string you
INVENTED rather than copied from the producer's real output (`%q`-formatted, quoted, escaped); and
a probe value that COINCIDES with the default, so a fence/ignore test must always probe with the
NON-default value (and a change that moves a default must re-check every guard whose expected value
it just made ambient). Key a guard on SYNTACTIC SHAPE, never an enumerated spelling list — the
spelling it misses is the target file's own house idiom; when the load-bearing thing is a compound
spelling, assert ALL its bytes and DERIVE the literal from the token you already hold. Mutate the
REAL tree, not only fixtures — a fixture battery samples the shapes you already thought of. Any test
that `eval`s a command's output must clear the variables it asserts on first. Treat a surviving
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
  #69 PR #77; #68 PR #78; #72 PR #79; #70 PR #80; #71 PR #81; #74 PR #82; #73 PR #83; #84 PR #90 —
  merged, one guards-are-code family) — A guard is code: mutation-test it (strip the feature, watch it
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
