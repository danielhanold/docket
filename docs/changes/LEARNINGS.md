<!-- LEARNINGS.md — the learnings ledger (contract: docket-convention, "Learnings ledger").
     Build-loop memory: lessons harvested at change close-out, read at groom, plan, and review
     time. Durable project conventions belong in CLAUDE.md — promotion during a distill removes
     the entry here. Newest first. Soft cap ~300 lines; the first harvest past the cap also
     distills (compression, not destruction — git history keeps whatever is dropped). -->

- 2026-07-15 (#80, PR #87) — Under `set -euo pipefail`, `[ -d "$dir" ] || mkdir -p "$dir"` inside a
  loop does not just skip a bad target — a pre-existing NON-directory at `$dir` (stray file or
  dangling symlink) makes `mkdir -p` fail, and `set -e` aborts the ENTIRE script, leaving a partial
  install across every remaining harness. Whole-branch review reproduced both triggers empirically.
  Apply: a conditional `mkdir` in a per-item loop needs a `|| continue` (fail one item, not the run),
  and the regression test must assert a LATER item still processes and the run exits 0 — not just
  that the bad item is skipped.
- 2026-07-15 (#75, PR #84) — A spec that frames code as "dead/dormant today, comes alive only once
  this change lands" can already be LIVE mid-branch, because an earlier task in the same branch
  flipped its precondition. Here Task 3 made `METADATA_WORKTREE` absolute, so `docket-status.sh`'s
  artifacts-refresh block (which just reads `mw="${METADATA_WORKTREE:-.docket}"`) was live from
  commit `c42ae5b` onward — and its `return 0` on a failed push had been silently abandoning
  `terminal-publish` AND `cleanup` the whole time, not merely from the final commit. Apply: when a
  premise is "X is dead today," re-probe X's liveness at the task that flips its precondition, not
  against the pre-branch tree; a `return`/early-exit in a block you're activating is abandoning
  every close-out step downstream of it until proven otherwise.
- 2026-06-17 → 2026-07-14 (#15 PR #32; #21 PR #34; #36 PR #47; #37 PR #48; #64 PR #75; #65 PR #74;
  #69 PR #77; #68 PR #78; #72 PR #79; #70 PR #80; #71 PR #81; #74 PR #82; #73 PR #83 — merged, one
  guards-are-code family) — A guard is code: mutation-test it
  (strip the feature, watch it go red) or it is decoration. Every way one has shipped GREEN while
  guarding nothing:
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
  was silently dropped from the sweep: the guard read green over prose it had never parsed, and a
  missed invocation would have shipped falsely green. Caught by the whole-branch review, fixed to allow
  leading whitespace, and mutation-confirmed (reverting one indented-fence invocation now reddens; it
  was invisible before). An order-assert built on `grep -n` LINE NUMBERS is the same error one level up
  — it cannot order two phrases inside one paragraph, and the implementer "satisfied" it by splitting a
  sentence mid-paragraph (#14).
  (d) **Wrong surface** — #68's inventory sentinel derived its op set from the `WRAPPED_OPS` array,
  proving "the array matches the doc" but never "the DISPATCH matches the doc": a `case` arm
  hand-added outside the loop would route while set-equality still held. A structural guard over a
  single-source list does not guard the surface that CONSUMES that list — assert the consuming
  surface directly.
  (e) **Stale state** — fence tests `eval`'d the config export without clearing the asserted variable
  first, and an aborting run emits NOTHING, so `eval ""` silently left the previous case's value in
  place and the assertion passed.
  (f) **False-RED / False-GREEN** — a `! grep "must-not-say-X"` fired on X in a legitimately
  *contrastive* clause, which a blunt absence-grep can't tell from the forbidden *adopted* posture;
  and a `grep -q "literal"` stayed green when a prose strip RELOCATED the must-preserve substring into
  an unrelated bullet, producing a false sentence.
  (h) **An assert that could never fire at all** — #71 shipped two. `! grep -qxF "BOARD_SURFACES="`
  was asserting on a string the producer never emits (bash's `%q` renders an empty value as
  `BOARD_SURFACES=''`, never a bare trailing `=`), and `! grep -qF "$FLAG"` with `FLAG='--surfaces'`
  makes grep parse the *pattern* as an **option** and exit 2 — which the leading `!` then inverts into
  a green `ok`. Neither could redden under any mutation. The second was copied VERBATIM from the plan's
  own snippet.
  (g) **A list of spellings, not a shape** — #70's write sentinel enumerated the *spellings* of a
  tainted variable (`$out`, `${out}`) instead of describing its *shape*, so it was green on `${out:-}`
  — the guarded file's own house idiom, 14 occurrences in `docket-status.sh`, i.e. the single most
  likely real regression. Four rounds of fixture-driven hardening all shipped green; round five found
  the hole in minutes by injecting the regression into the REAL script. #70 also disproved its own
  spec's premise that the new sentinel SUBSUMED the older `REDIRECT_RE` scan — mutation testing showed
  the two are complementary (the token-scoped sentinel is structurally blind to a write crossing a
  statement boundary carrying the bytes in no variable), so neither may be deleted (ADR-0031).
  (i) **A proper SUBSTRING of the thing you claim to guard** — #73's canonical-spelling assert anchored
  only the inner `${DOCKET_SCRIPTS_DIR:?…}` token, leaving the surrounding decoration
  (`"…"/docket.sh`) unguarded, so a mangle coordinated across the JSON fragment AND the guide's fence
  shipped green. The guarded string was a substring of the load-bearing one; the bytes that carry the
  meaning (the quotes and the `/docket.sh` suffix) were never asserted. Fixed to assert the full
  decorated spelling, CONSTRUCTED from the derived token rather than retyped, and mutation-confirmed.
  #73 also re-hit (h)'s plan-authored-test-code hazard from the byte side: the plan's own search
  literals for two short-form spellings omitted the JSON-escaped `\"`, so they could never byte-match
  the fragment they searched — a guard that would have passed only by never matching anything.
  Apply: anchor to the UNIQUE phrase the target clause owns ("before any push") and confirm
  `grep -c` == 1 — never a keyword set, never a blunt `! grep`/`grep -q` over a literal that can
  legitimately appear elsewhere in the doc. Tokenize at the unit you claim to guard (the invocation,
  not the line) — and prove the tokenizer SEES the whole document before trusting what it reports: an
  extractor whose anchor is stricter than the format allows (a column-0 fence in a format that indents
  fences) silently shrinks the corpus, and a guard that parses nothing passes everything, so assert the
  unit count it found, not just its verdict. Order-assert with byte offsets (`grep -ob`) when both
  anchors can share a line, and read an implementer contorting the artifact to pass an assert as a
  signal the assert itself is wrong. One assert owns exactly ONE clause — if two locations satisfy it, split and
  mutation-test each in isolation; when a mutation slips past a guard, add an INDEPENDENT scan rather
  than widening the first (#69 needed a second scan asserting the orchestrator never redirects the
  renderer into `BOARD.md`). Any test that `eval`s a command's output must clear the variables it
  asserts on first — "emitted nothing" and "emitted the expected thing" are otherwise
  indistinguishable. When a new feature legitimately violates an old absolutist sentinel, NARROW it to
  the property that is actually load-bearing; deleting it is how the guarded hole reopens — #74
  re-hit this from the deletion side: RETIRING the last `docket-config.sh --bootstrap` invocation
  removed the only slash-prefixed `/docket-config.sh` string in the convention and so reddened a
  sentinel keyed on that spelling, even though the convention still NAMES the resolver descriptively
  (exactly what ADR-0030 permits), so the fix was to narrow the pattern to `docket-config.sh`, never
  to drop the assert. A
  call-site-pinned audit must FOLLOW the call when the code is later extracted into a shared lib
  (0063's hooks audit correctly reddened when 0068 moved the call site into `lib/docket-preflight.sh`
  — the fix is to follow the extraction, never to loosen the audit). And a mutation that leaves an
  assert GREEN is a defect until proven otherwise, never a fact to explain away — re-derive the
  anchor's occurrence count yourself rather than trusting an implementer's narrative that a surviving
  mutant is harmless. Key a guard on SYNTACTIC SHAPE, never on an enumerated list of spellings — a
  spelling list is always one spelling short, and the spelling it misses is the target file's own
  house idiom. Mutate the REAL tree, not only fixtures: a fixture battery samples the shapes you
  already thought of, so inject the regression into the actual guarded file (its idioms are the
  adversary) before believing a green guard. And never RETIRE an existing guard on the claim that a
  new one subsumes it — prove subsumption by mutation in BOTH directions, or keep both. Prove an assert
  can FIRE before you trust that it passes: one whose pattern leads with `--` (always `grep -qF --
  "$pat"`), or whose expected string you INVENTED rather than copied from the producer's real output
  (`%q`-formatted, quoted, escaped), is vacuous by construction — run it against a tree where the
  guarded thing IS present and watch it redden. And a snippet the PLAN hands you is unvetted code:
  mutation-test it like any assert you wrote yourself. When the load-bearing thing is a compound
  spelling, assert ALL of its bytes — a recognizable substring is not the string, and the decoration
  you leave out is exactly what a coordinated edit is free to break; DERIVE the full literal from the
  token you already hold rather than retyping it, so the assert cannot drift from the artifact.

- 2026-07-13/14 (#69 PR #77; #71 PR #81 — merged, one sole-channel family) — When a channel becomes the
  SOLE source of some state, every property you used to get for free from the fallback has to be
  re-proven on the survivor. Both changes shipped a hole here, in both directions.
  (a) **Ordering** — #69's digest was spec'd to emit BEFORE the merge sweep, fine while `BOARD.md` was
  a second channel, but the same change forbade the skill from ever opening the board: a full pass then
  printed `change 60 implemented` and `swept 60` in one report with no corrective path left.
  (b) **Totality** — #71 collapsed six duplicated Board-pass call sites onto a stdout report line
  ("key on the LINE, never the exit code"), and the whole-branch review found two exit-0 paths through
  `docket-status.sh` that emit **no `board …` line at all** (an unknown/typo'd surface token; an inline
  render failure). A must-land caller seeing no line concludes "terminal → it landed" and proceeds on a
  silently stale board — the very defect class the change exists to kill, relocated from the script
  boundary into the caller contract and made QUIETER than before (a direct `board-refresh.sh` call had
  at least surfaced a non-zero exit).
  (c) **Terminality** — #71's first retry contract listed the three lines meaning "done" and retried on
  everything else, so a legitimate `board_surfaces: [github]` repo (which prints only `board github ok`)
  would have re-invoked forever.
  Apply: when a change removes the fallback channel for some state, re-audit the survivor's ORDERING
  against every pass that MUTATES that state (a snapshot taken before a mutating pass is only tolerable
  while something downstream can correct it), and prove the channel is TOTAL — every path, including the
  warn-and-ignore and failure paths, emits exactly one line — because "no line" is otherwise
  indistinguishable from success and you have merely moved the silence somewhere quieter. Enumerate a
  retry contract by its RETRYABLE set, never its terminal set: the terminal set is open-ended, and the
  legitimate line you forgot becomes an infinite loop.

- 2026-06-12 → 2026-07-14 (#14 PR #10; #32 PR #43; #42 PR #52; #56 PR #68; #64 PR #75; #52 PR #61;
  #54 PR #66; #71 PR #81; #74 PR #82 — merged, one enumerated-floor family) — **Every hand-written enumeration is a
  floor, not the set** — of sites, of audit dimensions, of tests — and the miss always lands in the
  surface that mattered most.
  (a) **Sites.** A hand-listed "everywhere X appears" undercounted again and again: 4 test assertions
  still hardcoding old model aliases plus two un-named id-read sites (surfaced only by reconcile's
  exhaustive grep, not the spec); a 9th generated wrapper leaving the convention's "eight wrappers"
  line, one `test_finalize_gate` assertion and seven `test_sync_agents` assertions stale at once; an
  earlier skill leaving "six skills" in README and the convention; and 0064's gating knob listing every
  *prose* site while missing `scripts/docket-status.sh`, the one **executable** invocation, in the
  headless merge sweep — precisely the agent the gate exists to serve, so `terminal_publish: false`
  would have kept publishing to `main` on every sweep. The same undercount hits a sentinel's own
  CORPUS: #71's structural sentinel scanned a file set that omitted
  `skills/docket-convention/github-board-mirror.md` — the one reference doc *about* board surfaces, and
  the likeliest place for a 9th call site to appear (widened in review to `skills/*/*.md` +
  `skills/*/references/*.md`).
  (b) **Audit dimensions.** A goal-scoped rewrite only examines the dimensions in its goal set; anything
  outside it passes unaudited even when every claim it makes is TRUE — a README rewrite audited hard
  against its three named goals shipped clean yet stayed Claude-centric for a tool that first-classes
  three harnesses (an accuracy audit verifies each claim is true, which a single-harness framing can
  be); the owner caught it at the merge gate.
  (c) **Tests.** A behavior-neutral slim passed its own goal-scoped review, yet finalize's FULL suite
  caught a regression its 7 enumerated sentinels missed. #74 re-hit it from the other direction: its
  edits reddened a *pre-existing* sentinel in `tests/test_docket_config.sh` — a file its plan never
  enumerated, in a change whose subject was a different file entirely — and only the whole-suite run
  saw it. The blast radius of retiring a string is every guard keyed on that string, repo-wide.
  Apply: never hand-list the sites of a literal, a count, or an operation you are gating — derive them
  from a grep of the WHOLE repo, then sort them into prose vs executable, because only the executable
  ones can violate a gate and they are the ones a docs-shaped reading skips right past. Let reconcile
  pin the full inventory before the build starts, and guard the list's completeness with a structural
  sentinel — whose corpus is itself an enumeration, so derive that too. Name every dimension you need
  audited as an explicit goal (for a multi-harness tool, "neutral across the supported set" is one). And
  run the WHOLE suite at the merge/build gate, never only the tests the spec enumerated — an
  out-of-goal regression is exactly what the tests outside the goal set exist to catch.

- 2026-06-21 / 2026-07-11 / 2026-07-14 (#37 PR #48; #59 PR #64; #60 PR #70; #74 PR #82 — merged, one
  moving-base family) — A change is designed against a SNAPSHOT and the base moves under it.
  (a) **Design.** 0059 was designed around what a still-`proposed` sibling (0058) would "later"
  compose; 0058 merged first and built the same gate independently, inverting 0059's scope twice (and
  since 0059 touched every skill file, three slim PRs merged mid-flight and re-slimmed its exact
  target lines). Conversely 0060's spec was ~90% delivered by that same sibling, and reconcile
  correctly folded 0060 to its one residual sub-case rather than killing it or rebuilding the overlap.
  (b) **Coordinates.** #74's spec pinned its two edit sites by LINE NUMBER (`~78`/`~110`) in a file
  sibling #71 reshaped before the build even began — stale on arrival; reconcile re-anchored them to
  shape-descriptions and the edits then coexisted cleanly.
  (c) **Conflict.** #37 was mid-build when a PR merged newer fixes into the very file it was
  stripping; the reflexive "keep my side" would have **silently reverted** them — the branch's version
  simply predated them.
  Apply: reconcile against what has actually MERGED, never against what a proposed sibling will do,
  and fold a change whose scope a sibling already shipped down to its residual (kill only if genuinely
  covered). Anchor a spec's edit sites to STRUCTURE the reader can re-find (the clause, the shape) —
  never to line numbers, which a sibling merge invalidates without touching your change. Rebase the
  most conflict-prone change (the one touching many shared files) LAST, once, onto the settled base,
  and resolve every hunk by INTENT: a same-file change that merged AFTER you diverged SUPERSEDES your
  edit (drop yours) rather than being a conflict to win.

- 2026-06-12 → 2026-07-14 (#12 PR #7; #21 PR #34; #47 PR #55; #65 PR #74; #74 PR #82 — merged, one
  verify-the-claim family) — A document that asserts a fact about another artifact — a code review, a
  spec, teaching prose — is not an oracle, and it has been flatly false four times. (a) A review
  finding cited a sentence that did not exist in the reviewed file. (b) A spec's stated *rationale*
  for a scope boundary was wrong (§3 claimed the convention's `.docket.yml` example "does not
  enumerate `finalize:`" — it does), though the boundary itself was sound on other grounds. (c) #74's
  spec claimed `docket.sh bootstrap` in a `STOP_MIGRATE`-shaped repo "exits non-zero and writes
  nothing"; against the real resolver it exits **0**, emits `BOOTSTRAP=STOP_MIGRATE`, and writes
  nothing — failing closed on a non-`PROCEED` verdict is `preflight`'s job, not the verb's — so an
  assert written FROM the spec would have pinned fiction and gone green doing it. (d) Prose restating
  a fact owned by another file drifts, and no sentinel can catch it: #65's README asserted which model
  tier each built-in agent runs at and shipped factually FALSE (it placed `docket-auto-groom` at a mid
  tier — it ships at the same top tier as `docket-implement-next` — and called `docket-status` "low
  effort" when it ships at `medium`) with every grep green, because a doc sentinel proves a sentence
  still EXISTS and can never prove it is still TRUE; #47 nearly copied a drifted sibling forward (the
  convention says "`effort: auto` (or omitted) → omit the effort line," but `sync-agents.sh` omits it
  only for `auto` — omitting the KEY keeps the built-in effort). Apply: verify a claim against the
  artifact or the RUNNING CODE before acting on it — byte-diff a review's quoted sentence; RUN the
  command whose behavior a spec describes before encoding that behavior in an assert; and write prose
  asserting a tier, count, default, or behavior against the CODE (cite the line), never against the
  sibling prose that describes the same thing and may already have drifted. Treat prose restating a
  configurable value as a drift surface from the day it ships. When a claim is false but its
  conclusion still defensible (here: the bootstrap cell guard does hold through the facade — no write
  outside `CREATE_ORPHAN`), keep the conclusion, write the test to the OBSERVED behavior, and record
  the discrepancy in the results file — never silently override a spec's scope boundary mid-build;
  leave the re-scope to the human. Reject false positives with evidence.

- 2026-06-21 / 2026-07-13 (#34 PR #45; #66 PR #73 — merged, one environment family) — Twice a suite
  ran RED where the failure was NOT a regression: (a) an ambient `DOCKET_SCRIPTS_DIR` export in the
  dev shell (written there by that very change's `install.sh`) was inherited by the test's sub-shells
  and masked their `${VAR:?}` fail-loud assertions; (b) a build sandbox failed 5 tests on environment
  facts (`origin/HEAD` unresolvable behind a proxied remote, a umask-dependent file mode, a timeout).
  Both were proven environment-bound by re-running the identical suite against unmodified `origin/main`
  and byte-comparing the failing sets. Apply: a RED suite in a build sandbox, or in a dev shell that
  has the feature installed, is a hypothesis, not a verdict — before calling it a regression OR waving
  it through, re-run the identical suite on the unmodified base (or under `env -u VAR`), byte-compare
  the failing sets, record the differential in the results file, and let the merge gate's clean-env run
  confirm. Author fail-loud tests to `env -u VAR` their own sub-shells so an installed shell can't
  false-RED them.

- 2026-07-13 (#66, PR #73) — The entire build ran under the Skill-layer `auto` fallback:
  `superpowers:writing-plans`, `subagent-driven-development`, and `requesting-code-review` were not
  invocable inside the implementer subagent's session, so plan, build, AND review all degraded —
  correctly per the Missing-skill rule, and disclosed in the results file and PR body. The artifacts
  all still look right, but it means docket's own autonomous builds are not actually running the
  SDD/TDD/review discipline their `skills:` bindings name. Apply: read a fallback warning as a
  build-loop defect to investigate, never as boilerplate — when a role degrades, check whether the
  skill is installed in the harness the SUBAGENT runs in (not merely the parent session), because a
  degraded binding silently drops the discipline while every artifact it should have produced is
  still there.

- 2026-07-11 (#63, PR #72) — 0063 disabled hooks on docket worktrees by relocating a conflicting
  common-config git value, and an early draft relocated `core.bare` unconditionally. Because
  `git init`/`clone` write `core.bare=false` into common config on essentially every repo, that
  fired on docket-status's most-run path — and since docket runs concurrent autonomous loops, a
  concurrent `--unset core.bare` raced one loop into the rollback branch, transiently re-enabling
  hooks. Apply: when a helper mutates SHARED git config (common/global) on a frequently-run path,
  only touch a value when the tool's own rule requires it (here: relocate `core.bare` only when
  `true`, per git); leave harmless defaults untouched, and assume a concurrent loop may be
  mutating the same key. Also: enabling `extensions.worktreeConfig` must precede any `--worktree`
  write and roll back on a failed follow-on write, so the extension is never left on with a value
  stranded — fail-closed ordering for multi-step config changes.

- 2026-07-11/13 (#58 PR #65; #69 PR #77; #16 PR #30; #22 PR #35; #25 PR #36; #26 PR #38; #35 PR #44
  — merged, one green-suite-untested-branch family) — Seven green suites that never exercised the
  branch they existed to cover. **Mock fidelity:** (a) a `gh api graphql` jq path read one level too
  shallow (`.data.pN.mergedAt` vs `.data.pN.pullRequest.mergedAt`), and the bug hid because the mock
  returned a *flattened* JSON shape `gh` never emits; (b) worse, every full-pass `docket-status`
  fixture pointed `SCRIPTS_DIR` at a mock dir containing **no `render-board.sh` at all** — and because
  the new digest call is best-effort, the missing tool degraded silently on every full-pass test, so
  the change's two headline claims (the backlog pass is ungated; `main()` always closes with `pass ok`)
  had ZERO real coverage: deleting the full-path `pass ok`, or re-gating the pass, both left the suite
  green. **Fixture realism:** (c) a golden fixture used `pr: 142` where real changes store a full URL
  and had a single `done` change, so neither the URL-format path nor the multi-id concatenation bug was
  hit; (d) a generator test set `DOCKET_HARNESS_ROOT` to the repo root, so the user-level and
  project-level passes wrote ONE dir and an "unlisted skill gets no project file" assertion passed
  vacuously; (e) a CAS conflict-retry branch shipped uncovered because the competing-writer test
  touched an *unrelated* file, hitting only the clean if-branch; (f) a renderer branching on
  git-remote resolution was smoked in a `/tmp` fixture with no origin, so only the degraded bare-path
  fallback ran; (g) fixtures cloning a fresh `init --bare` origin emit `warning: You appear to have
  cloned an empty repository`, leaving noisy stderr and no meaningful "0-byte stderr" assertion.
  Apply: a tool-output mock must mirror the real tool's response shape, nesting and all — and when the
  code under test has a best-effort/degrade branch, a mock that OMITS the tool silently routes every
  test through that branch, so at least one fixture must carry the REAL tool (and its `lib/`) or the
  happy path is untested while the suite reads green. Fixtures need real-SHAPED field values and
  PLURALITY (≥2 of every kind rendered as a list); smoke against real data inside a real worktree
  before merge; to cover a conflict/CAS path the competing writer must DIVERGE the same contended path
  (mutation-confirmed); give a tool writing to BOTH a user and a project location SEPARATE dirs; keep
  fixture stderr 0-byte. Green tests ≠ the hard branch was exercised.

- 2026-07-11 (#55, PR #67) — A behavior-neutral skill slim landed all five files modestly over the
  spec's line-count targets, and review confirmed the residual was load-bearing/test-anchored content,
  not un-cut prose — the spec's size estimates were simply optimistic. Apply: on a behavior-neutral
  slim, the size target is a direction, not a gate — once review shows the remaining lines are
  load-bearing, accept the size and stop trimming; behavior-neutrality outranks hitting the number.

- 2026-07-10/11 (#51 PR #60; #57 PR #63 — merged, re-hit class) — An awk/sed **range** edit
  (`/start/,/end/`) over a marker-bounded "do-not-hand-edit" managed block is a data-loss hazard
  whenever the end marker is lost (truncation / bad merge) or the markers are out of order
  (END-before-START, same spelling): the range runs to EOF and silently deletes the user's own
  content after the dangling start (`.gitignore` bytes here). A guard checking marker *presence*
  alone is bypassed by the corrupted block. Apply: before stripping/rewriting a marker-delimited
  block, validate marker *order and balance* — refuse-and-warn on dangling / out-of-order / nested /
  unbalanced markers (either spelling) and leave the file untouched; never presence alone, never let
  the range consume to EOF.

- 2026-07-10 (#51, PR #60) — A printed migration remedy chained `git add .gitignore && git commit`
  unconditionally, but in a repo with stale tracked wrappers and no current opt-in no block is
  written, so the command failed as-run — the remedy was valid only in the state the author
  pictured, not the state that triggered it (fixed to make the `git add` clause conditional on the
  block actually being maintained, `41d9815`). Apply: a remedy command you print for a user to run
  verbatim must be valid in the *exact* repo state that produced it — branch the printed text on the
  same condition that gates the underlying write, never emit one fixed command for divergent states.

- 2026-07-09 (#50, PR #59 — two hazards from one new config layer) — (a) **The write path.** Adding a
  write to a shared per-user location (`sync-agents.sh`'s auto-migration writes
  `~/.config/docket/config.yml`) upgraded every non-hermetic test that reaches it from read-leak to
  write hazard: `tests/test_install.sh` inherited `XDG_CONFIG_HOME`, so on machines exporting it (common
  on Linux) the suite would have **rewritten the developer's real global config** as a test side effect
  — caught by the final whole-branch review, fixed pre-merge (`585b5ae`). (b) **The read path.** The new
  layer passed every unit fixture, yet in live use its `agents:` values were fully shadowed in any repo
  opted into per-repo generation: the committed full wrapper set (change 0048) resolves from
  `.docket.yml` + built-ins only and takes harness precedence over the user-level wrappers carrying the
  global values. Found only by live-testing a real repo *after* the build (a loud causal warning landed
  as a stopgap; real semantics deferred to #0051). Apply: when a change adds a write path to a shared
  user-level location, audit every test that can transitively reach it and pin the relevant env
  (XDG/HOME) hermetically — tests that merely read-leaked before the change become data-loss hazards
  after it. And when adding a LOWER-precedence config layer, enumerate every higher-precedence
  **generated artifact** that can shadow it — not just the direct readers — then live-test the new
  layer in a repo where those artifacts exist: a value can resolve correctly and still never take
  effect.

- 2026-07-09 (#49, PR #58) — A change that added a new user-facing config knob (the role-keyed
  `skills:` map) shipped its resolution logic and skill-body wiring but NOT its surfacing: the
  commented sample `.docket.yml` never gained the new keys, README still framed superpowers as a
  hard requirement rather than a configurable default, and the option went undocumented — all caught
  by the human at the merge gate, not the build. Apply: a new config knob is not done when it merely
  *works* — ship it end-to-end in the same change: add it (commented, with every option) to the sample
  `.docket.yml`, document it in README, and update any prose that stated the now-relaxed requirement
  as absolute.

- 2026-07-09 (#48, PR #57) — A new per-repo generation behavior (committed agent wrappers + a Cursor
  dispatch rule) was gated on merely `.docket.yml` being *present*, which silently broke tracking-only
  adopters: `install.sh`'s `sync-agents.sh` run littered 8 untracked `.claude/agents/docket-*.md` into
  any change-tracking-only repo and flipped its `sync-agents.sh --check` from a no-op to failing — a
  backward-incompatible break caught only by the whole-branch review, not planning. Apply: when adding
  output-generating behavior to a tool that has a minimal "tracking-only" adoption mode, gate it on an
  explicit opt-in signal (a dedicated config key), never on the mere presence of the config file — and
  add a regression test asserting the minimal adopter generates zero files and keeps `--check` a no-op.

- 2026-07-08 (#45, PR #54) — A plan that split multi-harness generation across two tasks left a
  Task-1 seam: Task 1 removed the `PROJECT_AGENT_DIR` variable, but `check_project_level` (only
  rewritten in Task 2) still referenced it, an unbound-variable crash under `set -euo pipefail` that
  would have reddened the `--check` tests had the tasks landed in isolation. Apply: when a plan
  splits one function's rewrite across sequential tasks, treat the intermediate (Task N of M) state
  as itself buildable and testable — don't assume the earlier task's leftover references are safe
  because a later task will delete them.

- 2026-06-19 / 2026-06-21 / 2026-07-08 / 2026-07-14 (#25 PR #36; #38 PR #46; #46 PR #56; #71 PR #81 —
  merged, one shell-portability family) — Portability traps in tooling the plan itself authored. (a)
  **grep for a `--flag`:** a bare ERE that *leads* with `--` is parsed as a grep **option**
  (`unrecognized option`, exit 2); over-escaping to dodge that (`\-\-yes\b`) springs GNU grep's
  `stray \ before -` stderr warning, which BSD grep stays silent about — so it hides on macOS. Declare
  the pattern with `grep -E -e "<pat>"` or `grep -qF -- "<pat>"`: POSIX `-e`/`--` makes the next arg a
  pattern, never an option. #71 re-hit this inside a NEGATED assert (`! grep -qF "$FLAG"`,
  `FLAG='--surfaces'`), where the leading `!` inverted grep's exit-2 error into a green `ok` — the trap
  stops being a loud crash and becomes a permanently vacuous guard (guards family, (h)).
  (b) **awk whitespace class:** `ind()` used `[^ ]` (a literal-space class), so a **tab-indented**
  config layer was silently dropped — use `[^[:space:]]` and test tab-indented input. (c) **macOS path
  resolution:** `mktemp` yields `/var/…` but git reports `/private/var/…`, so stripping a worktree
  prefix matched nothing — `pwd -P` both the path and the prefix before stripping. Apply: when a plan
  hands you awk/shell it authored, treat whitespace classes, `--`-leading patterns, and symlinked
  temp paths as suspect, and test each on both GNU and BSD.

- 2026-06-19 (#27, PR #39) — A change promised its locally-written file (`.claude/settings.local.json`)
  would "never be committed onto collaborators," but on the build machine that guarantee only held
  because a *user-global* excludesfile (`~/.config/git/ignore`) ignored it — the repo `.gitignore` did
  not. Reconcile caught it; unfixed, a collaborator without that global ignore could have committed the
  file, defeating the change's whole point. Apply: an "every clone / never committed" guarantee must
  rest on a committed repo `.gitignore` entry, never a per-machine user-global ignore — and when a
  change *generates* such a file, add the ignore in the same change (the migrate step here) so the
  guarantee ships with the feature instead of silently depending on each dev's box.

- 2026-06-19 (#26, PR #38) — A `.docket.yml` reader interpolated the lookup key straight into an ERE
  (`^[[:space:]]*<key>`), so any future key carrying a regex metacharacter would match unintended
  lines; the same unescaped helper is still copy-pasted in `migrate-to-docket.sh`. Apply: escape ERE
  metacharacters in a key before building a `grep -E`/regex match from it — and when you fix one copy
  of a duplicated shell helper, note the un-fixed twin so the divergence is tracked, not silent.

- 2026-06-19 (#25, PR #36) — An in-place `sed` that sets a frontmatter field (`status:`/`updated:`/
  `results:`) was unanchored, so it would have rewritten *any* column-0 match — including body prose,
  a live risk for docket's own change/ADR files (which discuss those field names). Apply: anchor a
  frontmatter-field edit to the first `---…---` block, never a bare line match — and lock it with a
  test where a body `status:` line survives verbatim while the frontmatter field is set.

- 2026-06-17 (#20, PR #33) — Invoking a skill presents only its `SKILL.md`; sibling files are NOT
  auto-loaded, so a section moved out for progressive disclosure leaves every consumer's context
  unless something Reads it. Apply: extract only a section that is heavy AND off the common path
  (opt-in, or its work is script-delegated — like the GitHub mirror → `github-mirror.sh`); leave a
  stub + pointer under the original heading so name-based cross-refs still resolve, and add a pointer
  in the one consumer that needs the mechanics. Verify the MOVE by byte-diffing the sibling against
  the base section and mutation-testing each new grep assertion.

- 2026-06-17 (#15, PR #32) — A read-only review subagent ran `git checkout <sha>` in the SHARED
  feature worktree to inspect a diff, detaching HEAD; the controller's later commits (plan, results)
  landed on the detached HEAD, the branch ref stayed put, and a plain `git push` of the branch
  silently published only the pre-detach tip (the PR was briefly missing files). Apply: review/
  inspection subagents must NOT `git checkout` in a shared worktree (use `git show`/`git diff <sha>`,
  or a throwaway worktree); after every push the controller SHA-compares local vs origin AND checks
  `git symbolic-ref -q HEAD` is the feature branch — never trust the push exit code alone.

- 2026-06-10 / 2026-06-17 (#5 PR #6; #15 PR #32 — merged, one YAML-scalar family) — Two ways a value
  docket writes by hand parses differently once a real YAML loader is in play: an unquoted frontmatter
  scalar cannot contain ": " (colon-space), and a config enum colliding with a YAML 1.1 boolean keyword
  (`gate: off`) is safe under docket's grep/awk reads — it stays the literal string "off" — but would
  load as `false`. Apply: quote (or reword around) any hand-authored scalar carrying a colon-space or a
  boolean keyword (on/off/yes/no/true/false); today's reader tolerating it is not evidence the value is
  well-formed (flagged for #0018/yq).

- 2026-06-17 / 2026-07-14 (#17 PR #31; #74 PR #82 — merged, one ADR-update-delivery family) — An
  `## Update` to an already-published, immutable ADR (0008) had to reach the integration branch
  alongside a NEW ADR (0009) it cross-references, without a premature direct-to-`main` push (which
  would dangle the `[[0009]]` link until the new ADR merged). #74 hit the same shape without the
  cross-reference: narrowing the facade wiring guard dated a *supporting detail* of ADR-0030's
  Decision (the carve-out it named is gone) while leaving the decision itself — the invocation-prefix
  discriminator — intact. Apply: to deliver an ADR body update onto the integration branch atomically,
  list that ADR id in the producing change's `adrs:` so terminal-publish re-copies it on merge — never
  a standalone push that races the cross-referenced ADR's own publish. An Accepted ADR is immutable
  except its `status:` line, so a detail the world has since dated is appended to as a dated
  `## Update`, never rewritten as a Decision edit.

- 2026-06-16 / 2026-07-08 (#11 PR #11; #16 PR #30; #46 PR #56 — merged, one pipefail family) — A test
  piped a live-producing script straight into `grep -q`; grep exits on first match, the still-writing
  producer takes SIGPIPE, and `pipefail` turned that 141 into an intermittent failure — review later
  found the same shape with `head`, and #46 hit it again in production code (`printf … | section_body`,
  whose consumer `exit`s early; guarded with `|| true`). Apply: never `producer |
  early-exiting-consumer` (`grep -q`, `head`, `head -n1`, or any reader that may stop before EOF)
  under `set -o pipefail` — capture into a variable first, then grep/`head <<<"$var"`.

- 2026-06-16 / 2026-07-14 (#11 PR #11; #71 PR #81 — merged, one idempotency-keying family) — Three
  no-op/idempotency probes, each keyed on a proxy that a PARTIAL FAILURE also satisfies. #11's
  derived-surface mirror keyed idempotency on a persisted change-file field but did no git writes
  itself, so a bare run (outside the orchestrating pass that records the field) re-created every item —
  and it read the integration checkout where `active/` is pruned, so it only saw archived changes; its
  first-sync close-state keyed on the *pre-existing* id field (empty on a fresh mint), so an
  already-terminal item was created open and closed only on a later pass. #71 found the same shape in
  the board orchestrator: `board inline clean` was keyed on a CLEAN WORKING TREE, but after a failed
  push the board commit already exists locally and the tree is clean — so the must-land remedy
  (re-invoke) re-rendered, found no diff, and reported the terminal-success line while the board had
  never reached the remote. `board inline changed pushed` was unreachable after any push failure. Apply:
  key a "nothing to do" probe on the state you actually PROMISED (it reached the remote), never on a
  local proxy — clean tree, no diff, a stored field — because the proxy is precisely what a
  half-completed run leaves behind, and the probe then certifies the failure as success. (Fixed by also
  counting unpushed commits touching the path, `git rev-list --count @{u}..HEAD -- <path>`, degrading to
  "nothing to push" when there is no upstream, and falling through into the existing push/rebase loop.)
  A script that reads change files must read the metadata working tree (guard the pruned tree) and is
  idempotent only via the orchestrating pass's write-back — drive it through that pass, never bare; and
  when a create-and-set-state pass mints an id, key the state write on the EFFECTIVE id (existing OR
  just-minted), not the stored field.

- 2026-06-12 (#14, PR #10) — Two views keyed off a body section's *presence* (board cell,
  selection band), but the state transition out (re-arm) didn't remove the section — a re-armed
  stub stayed mislabeled. Apply: when state is encoded by an artifact's presence, every
  transition out of that state must remove the artifact.

- 2026-06-12 (#6, PR #8) — The spec asked for a test asserting a metadata-branch file exists,
  but the suite runs against the integration-branch checkout where that file never lives. Apply:
  when specifying tests for metadata-branch artifacts, verify them at build time and record in
  the results file instead — repo tests can only see the integration branch.

- 2026-06-12 (#12, PR #7) — link-skills.sh needed no edit for a new skill — it globs skills/*/.
  Apply: at reconcile, check whether plumbing auto-discovers before planning an edit to it.

- 2026-06-02–12 (#1, #2, #5, #13) — Foundational sentinel/test discipline (consolidated; richer,
  more specific restatements live in the guards-are-code and green-suite families above): sentinel
  greps are sampling, not parsing — pair them with a whole-branch review that reads for meaning; prove
  each assertion non-vacuous (deleting the clause it guards must flip the test to NOT OK); when order
  is part of the contract, assert it explicitly rather than inferring it from presence; and build
  inline when tasks share one artifact, fanning out only for genuinely independent work.
