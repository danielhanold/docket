<!-- LEARNINGS.md — the learnings ledger (contract: docket-convention, "Learnings ledger").
     Build-loop memory: lessons harvested at change close-out, read at groom, plan, and review
     time. Durable project conventions belong in CLAUDE.md — promotion during a distill removes
     the entry here. Newest first. Soft cap ~300 lines; the first harvest past the cap also
     distills (compression, not destruction — git history keeps whatever is dropped). -->

- 2026-07-13 (#69, PR #77) — Making a new channel the SOLE source of some state turned a previously
  benign staleness into a self-contradiction. The digest was spec'd to emit BEFORE the merge sweep,
  which was fine while `BOARD.md` existed as a second channel — but the same change forbade the skill
  from ever opening the board, so a full pass now printed `change 60 implemented` and `swept 60` in
  one report with no corrective path left. Apply: when a change removes the fallback channel for some
  state, re-audit the surviving channel's ORDERING against every pass that MUTATES that state — a
  snapshot taken before a mutating pass is only tolerable while something downstream can correct it.
  (Shipped: emit once per path — before the early exit that runs no sweep, after the sweep on a full
  pass. The spec's placement sentence described a mechanism for a single-call implementation, not a
  boundary, so the deviation was disclosed in the results file, documented at three sites, and locked
  by tests that redden if the call moves back — never silently "fixed".)

- 2026-07-13 (#64, PR #75) — Adding a knob that GATES an existing operation, the plan enumerated the
  call sites by hand — and listed the skill/reference prose while missing `scripts/docket-status.sh`,
  the one *executable* invocation, in the headless merge sweep. That is precisely the agent the gate
  exists to serve, so the miss would have left `terminal_publish: false` still publishing to `main`
  on every sweep — the exact failure the change exists to prevent. Apply: never hand-list the call
  sites of an operation you are gating — derive them from a grep of the invocation across the whole
  repo, then sort them into prose vs executable, because only the executable ones can actually
  violate the gate and they are the ones a docs-shaped reading of the change skips right past. Guard
  the completeness of that list with a structural sentinel in the suite, not with review attention.

- 2026-07-13 (#64 PR #75; #69 PR #77 — merged, one sentinel-as-code family) — Structural sentinels
  shipped FALSE-PASS holes that surfaced only under mutation (strip the feature, watch the test go
  red). (a) A sentinel grepped per LINE, so a logical line carrying a gated `--id` invocation and an
  ungated `--adr` invocation side by side — which `docket-finalize-change/SKILL.md` actually has —
  was whitewashed by the single `--enabled` present; it now splits per invocation before filtering.
  (b) Fence tests `eval`'d the config export without clearing the asserted variable first, and an
  aborting run emits NOTHING, so `eval ""` silently left the previous case's value in place and the
  assertion passed on stale state. (c) #69 had to NARROW an existing sentinel (0059's "never mentions
  `render-board.sh`") rather than keep it, because the new read-only `--format digest` call
  necessarily violates it — and one guard could not replace it: a per-invocation flag-tokenizer
  proved (by mutation) to miss `render-board.sh --format digest > BOARD.md`, so a SECOND, independent
  scan asserting the orchestrator never redirects the renderer into `BOARD.md` ships beside it.
  Apply: a guard is code — mutation-test it before trusting it, or it is decoration. A grep sentinel
  must tokenize at the unit it claims to guard (the invocation, not the line), and one guard covers
  one hole — when a mutation slips past it, add an independent scan rather than widening the first.
  Any test that `eval`s a command's output must clear the variables it asserts on first, because
  "emitted nothing" and "emitted the expected thing" are otherwise indistinguishable. And when a new
  feature legitimately violates an old absolutist sentinel, narrow the sentinel to the property that
  is actually load-bearing — deleting it is how the guarded hole reopens.

- 2026-06-21 / 2026-07-13 (#34 PR #45; #66 PR #73 — merged, one environment family) — Twice a suite
  ran RED where the failure was NOT a regression. (a) finalize's local gate reddened on the change's
  own drift-guard test only because `DOCKET_SCRIPTS_DIR` was *exported* in the interactive shell (that
  very change's `install.sh` writes it to the shell profile), and the ambient export was inherited by
  the test's sub-shells, masking its `${VAR:?}` fail-loud assertions; a clean-env re-run
  (`env -u DOCKET_SCRIPTS_DIR`) was all-green. (b) The build sandbox failed 5 tests (`origin/HEAD`
  unresolvable behind a proxied remote, a umask-dependent file mode, one timeout). The implementer
  proved they were environment-bound rather than waving them through — running the identical suite on
  unmodified `origin/main` and byte-comparing the failing set and exit codes — and said so in the
  results file; finalize's gate on the dev machine then confirmed it (35/35 green). Apply: a RED suite
  in a build sandbox or in a dev shell that has the feature installed is a hypothesis, not a verdict —
  before calling it a regression OR dismissing it, re-run the identical suite against the unmodified
  base (or under `env -u VAR`) and byte-compare the failing sets; record that differential in the
  results file, and let the merge gate's clean-environment run be the confirmation. Also author
  fail-loud tests to `env -u VAR` their own sub-shells so an installed dev shell never false-REDs them.

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

- 2026-07-13 (#65, PR #74) — A doc sentinel proves a sentence still EXISTS; it can never prove the
  sentence is still TRUE. This change's README teaching prose asserted which model tier each built-in
  agent runs at, and shipped factually false on the first pass (it claimed a design pass sits at a
  mid tier between build and sweep — `docket-auto-groom` actually ships at opus/xhigh, the same top
  tier as `docket-implement-next`, and `docket-status` was called "low effort" when it ships at
  `medium`). Every sentinel was green: they pin the framing, not the claims. The whole-branch review
  caught it by re-deriving each tier from the shipped `agents/docket-*.md`. Apply: when prose asserts
  a fact whose source of truth is another file (a tier, a count, a default), a grep sentinel is NOT
  coverage — either derive the assertion from that source in the test, or accept that only a review
  reading against the source can validate it; and treat prose restating a configurable value as a
  drift surface from the day it ships.

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

- 2026-07-11 (#59 PR #64; #60 PR #70 — merged, two sides of one rule) — 0059 was designed around a
  still-`proposed` sibling (0058) "later" composing its board-refresh gate, but 0058 merged first and
  built the same gate independently, inverting 0059's scope twice; and because 0059 touched every
  skill file, three slim PRs merged mid-flight and re-slimmed its exact target lines. Conversely,
  0060's spec was ~90% delivered by sibling 0059, and reconcile correctly folded 0060 down to the one
  residual sub-case rather than killing it or rebuilding the overlap. Apply: never design a change
  around what a still-proposed sibling WILL do — reconcile against what has actually merged, not the
  planned backlog; hold the most conflict-prone change in flight (touches many shared files) and
  rebase it LAST, once, onto the settled base; and when a sibling ships the bulk of an unstarted
  change's scope, reconcile down to the residual — kill only if it is genuinely covered.

- 2026-06-12 / 2026-07-11 (#14 PR #10; #56 PR #68 — merged, same lesson) — Adding a member to an
  enumerated set left stale literal counts across prose AND tests: the 9th generated wrapper turned
  the convention's "eight wrappers / three no-skill" line, one `test_finalize_gate` assertion, and
  seven "8 built-in wrappers" assertions in `test_sync_agents` stale at once (the full suite, not the
  change's own sentinels, proved none were missed); earlier, a new skill left "six skills" in README
  and "six operating skills" in the convention. Apply: a literal count of an enumerated set
  (wrappers, states, skills) repeated across prose and test assertions is a cross-cutting invariant —
  before adding a member, grep the WHOLE repo for the current number and update every copy in the
  same change.

- 2026-07-11 (#55, PR #67) — A behavior-neutral skill slim landed all five files modestly over the
  spec's line-count targets, and review confirmed the residual was load-bearing/test-anchored content,
  not un-cut prose — the spec's size estimates were simply optimistic. Apply: on a behavior-neutral
  slim, the size target is a direction, not a gate — once review shows the remaining lines are
  load-bearing, accept the size and stop trimming; behavior-neutrality outranks hitting the number.

- 2026-07-10/11 (#52 PR #61; #54 PR #66 — merged, one pattern) — A goal-scoped rewrite only examines
  the dimensions in its goal set; anything OUTSIDE it passes unaudited even when every claim it makes
  is TRUE. (a) A README rewrite audited hard against its three named goals (accuracy, structure,
  newcomer clarity) shipped clean — yet stayed Claude-centric for a tool that first-classes three
  agent harnesses; the owner caught it at the merge gate. An accuracy audit verifies each claim is
  true, which a single-harness framing can be. (b) A behavior-neutral skill slim passed its own
  goal-scoped review, yet finalize's FULL suite caught a regression its 7 enumerated sentinels did
  not — the slim had dropped finalize's required `render-change-links.sh` mention, reddening
  `test_change_links_coverage`, a test outside the anticipated set. Apply: name every dimension you
  need audited as an explicit rewrite goal (for a multi-harness tool, "neutral across the supported
  set" is one — default narrative examples to the NEUTRAL term); and run the WHOLE suite at the
  merge/build gate, never only the tests the spec enumerated — the sentinel list is a floor, and an
  out-of-goal regression is exactly what the tests outside it exist to catch.

- 2026-07-11/13 (#58 PR #65; #69 PR #77 — merged, one mock-fidelity family) — Twice a mock shaped to
  the CODE PATH rather than to the real tool made a suite pass against a fiction. (a) A `gh api
  graphql` jq path read one level too shallow (`.data.pN.mergedAt` vs
  `.data.pN.pullRequest.mergedAt`); the bug hid because the mock returned a *flattened* JSON shape
  `gh` never emits. (b) Worse, every full-pass `docket-status` fixture pointed `SCRIPTS_DIR` at a mock
  dir containing no `render-board.sh` at all — and because the new digest call is **best-effort**, the
  missing tool degraded silently on every full-pass test. The change's two headline claims (the
  backlog pass is *ungated*; `main()` *always* closes with `pass ok`) therefore had ZERO real
  coverage: deleting the full-path `pass ok`, or re-gating the pass, both left the suite green. Apply:
  a tool-output mock must mirror the real tool's exact response shape, nesting and all — and when the
  code under test has a best-effort/degrade branch, a mock that OMITS the tool silently routes every
  test through that branch, so at least one fixture must carry the REAL tool (and its `lib/`) or the
  happy path is untested while the suite reads green. Same root as the coverage family below.

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

- 2026-07-09 (#50, PR #59) — A feature that added a write path to a shared per-user location
  (`sync-agents.sh`'s auto-migration writes `~/.config/docket/config.yml`) upgraded every
  non-hermetic test that reaches it from read-leak to write hazard: `tests/test_install.sh`
  inherited `XDG_CONFIG_HOME`, so on machines exporting it (common on Linux) the suite would have
  **rewritten the developer's real global config** as a test side effect — caught by the final
  whole-branch review, fixed pre-merge (`585b5ae`). Apply: when a change adds a write path to a
  shared user-level location, audit every test that can transitively reach that path and pin the
  relevant env (XDG/HOME) hermetically — tests that merely read-leaked before the change become
  data-loss hazards after it.

- 2026-07-09 (#50, PR #59) — The new global config layer passed every unit fixture, yet in live use
  its `agents:` values were fully shadowed in any repo opted into per-repo generation: the committed
  full wrapper set (change 0048) resolves from `.docket.yml` + built-ins only and takes harness
  precedence over the user-level wrappers carrying the global values. Found only by live-testing on
  a real repo *after* the build (a loud causal warning landed as a stopgap; real semantics deferred
  to #0051). Apply: when adding a lower-precedence config layer, enumerate every higher-precedence
  **generated artifact** that can shadow it — not just the direct readers — and live-test the new
  layer's effect in a repo where those artifacts exist; a value can resolve correctly and still
  never take effect.

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

- 2026-07-08 (#46, PR #56) — A latent bug rode in from the plan's own awk and surfaced only at task
  review, not at planning: `ind()` used `[^ ]` (a literal-space class), so a **tab-indented** config
  layer was silently dropped — fixed to `[^[:space:]]` in both awk copies, with a tab-indented
  regression test. (Its sibling bug, a `printf … | section_body` SIGPIPE, is folded into the pipefail
  entry below.) Apply: when a plan hands you awk/shell it authored, treat whitespace character
  classes as suspect and test tab-indented input.

- 2026-07-08 (#47, PR #55) — A docs change whose whole job was to document an *existing* behavior
  (how `effort: auto` affects a generated agent) nearly documented the wrong thing: the neighboring
  docket-convention "Agent layer" prose says "`effort: auto` (or omitted) → omit the effort line,"
  but `sync-agents.sh:145` omits the line only for `auto` — omitting the *key* keeps the built-in
  effort. The README was salvaged only by writing it against the code, not the sibling prose. Apply:
  when a change's job is to document existing behavior, treat the code (cite the line) as the source
  of truth — sibling prose describing the same behavior may have drifted; don't copy it forward.

- 2026-07-08 (#45, PR #54) — A plan that split multi-harness generation across two tasks left a
  Task-1 seam: Task 1 removed the `PROJECT_AGENT_DIR` variable, but `check_project_level` (only
  rewritten in Task 2) still referenced it, an unbound-variable crash under `set -euo pipefail` that
  would have reddened the `--check` tests had the tasks landed in isolation. Apply: when a plan
  splits one function's rewrite across sequential tasks, treat the intermediate (Task N of M) state
  as itself buildable and testable — don't assume the earlier task's leftover references are safe
  because a later task will delete them.

- 2026-07-08 (#42, PR #52; also #32, PR #43 — merged near-duplicates) — Twice, a spec's enumerated
  touch-point/call-site list **undercounted**: 4 extra test assertions hardcoding the old model
  aliases, and two un-named id-read sites, all surfaced only by the reconcile pass's exhaustive grep,
  not the spec's enumeration. Apply: a spec's enumerated list is a floor, not the complete set — grep
  the whole repo/suite for every occurrence of the old literal or pattern before a codebase-wide
  swap, and let reconcile pin the full inventory in the change before the build starts.

- 2026-06-21 (#37, PR #48) — A change stripping prose from `docket-status/SKILL.md` was mid-build when
  PR #47 (#36) merged into `origin/main` with newer fixes to the *same* file. Rebasing onto the new
  base, the reflexive "keep my side" of the conflict would have **silently reverted** #36's ordering +
  failure-posture fixes — the branch's version simply predated them. Apply: when the integration branch
  advances mid-build into a file you're editing, resolve the rebase by *intent* against the landed
  version, and recognize when a same-file change that merged *after* you diverged **supersedes** your
  edit (drop yours) rather than treating it as a conflict to win.

- 2026-06-17/21 · 2026-07-13 (#15 PR #32; #21 PR #34; #36 PR #47; #37 PR #48; #65 PR #74; #69 PR #77
  — merged, one anchoring family) —
  Four ways a doc sentinel passed while guarding nothing. (a) **Broad keyword OR-set:** an ordering
  assert grepping `run the suite|validate|local` latched onto an unintended EARLIER line. (b)
  **Double-guarded:** one grep satisfied by two independent clauses (a YAML config comment AND a
  prose sentence), so deleting the substantive prose left it green — while a whole-region mutation
  test still "proved" it non-vacuous. #65 then shipped two MORE double-guarded sentinels, and in both
  the implementer's own mutation test had already gone green-on-mutation — its report rationalized
  that away as benign; the reviewer caught them by re-deriving the substring's occurrence count by
  hand. #69 shipped two more still (`grep -qF "digest"` satisfied 3× over, `"board off"` 4×): the
  reviewer rewrote the skill's Final summary to literally say *"read from `BOARD.md`"* — the exact
  posture that change abolishes — and the assert stayed green. The fix each time is the same one this
  family already prescribes: re-anchor to the unique phrase the clause owns and confirm `grep -c` == 1.
  (c) **False-RED:** a `! grep "must-not-say-X"` fired on X in a
  legitimately *contrastive* clause, which a blunt absence-grep can't tell from the forbidden
  *adopted* posture. (d) **False-GREEN:** a `grep -q "literal"` stayed green when a prose strip
  **relocated** the must-preserve substring into an unrelated bullet, producing a false sentence.
  Apply: anchor to the UNIQUE phrase the target clause owns ("before any push"), never a keyword set;
  one assert owns exactly ONE clause — if two independent locations satisfy it, split and
  mutation-test each in isolation; assert intent with a POSITIVE anchor on the meaningful framing;
  keep a must-preserve substring in its grammatical location (never relocate it to pass); never rely
  on a blunt `! grep`/`grep -q` over a literal that can legitimately appear elsewhere in the doc. And
  a mutation that leaves an assert GREEN is a defect until proven otherwise, never a fact to explain
  away — re-derive the anchor's occurrence count yourself rather than trusting an implementer's
  narrative that a surviving mutant is harmless.

- 2026-06-21 (#38, PR #46) — A test grepped for a CLI flag with `grep -qE "\-\-yes\b|\b-y\b"`; the
  `\-` over-escaping silenced one trap (a bare ERE that *leads* with `--` is parsed as a grep
  **option**, not a pattern — `unrecognized option`, exit 2) only by springing another (GNU grep's
  `stray \ before -` stderr warning; BSD grep stays silent, so it hides on macOS). Naively dropping
  the backslashes re-opens the option-parse trap. Apply: to grep for a `--flag`, declare the pattern
  with `grep -E -e "<pat>"` — POSIX `-e` makes the next arg a pattern, never an option; clean (exit 0,
  empty stderr) on **both** GNU and BSD grep. Never lead a bare ERE with `--`.

- 2026-06-16/21 (#16 PR #30; #22 PR #35; #25 PR #36; #26 PR #38; #35 PR #44 — merged, one coverage
  family) — Five green suites that never exercised the branch they existed to cover. (a) A golden
  fixture used `pr: 142` where real changes store a full URL, and had a single `done` change — so
  neither the URL-format path nor the multi-id concatenation bug was hit. (b) A generator test set
  `DOCKET_HARNESS_ROOT` to the repo root, so the user-level and project-level passes wrote ONE dir and
  an "unlisted skill gets no project file" assertion passed vacuously. (c) A CAS conflict-retry branch
  shipped uncovered because the competing-writer test touched an *unrelated* file, hitting only the
  clean if-branch. (d) A renderer branching on git-remote resolution was smoked in a `/tmp` fixture,
  which has no origin — only the degraded bare-path fallback ran. (e) Fixtures cloning a fresh
  `init --bare` origin emit `warning: You appear to have cloned an empty repository`, leaving noisy
  stderr and no meaningful "0-byte stderr" assertion. Apply: fixtures need real-SHAPED field values
  and PLURALITY (≥2 of every kind rendered as a list); smoke against real data, inside a real
  worktree, before merge; to cover a conflict/CAS path the competing writer must DIVERGE the same
  contended path (mutation-confirmed); give a tool writing to BOTH a user and a project location
  SEPARATE dirs; keep fixture stderr 0-byte. Green tests ≠ the hard branch was exercised.

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

- 2026-06-19 (#25, PR #36) — A script resolving a worktree-relative path built from `mktemp` matched
  nothing on macOS: `mktemp` yields `/var/…` but git reports `/private/var/…`, so stripping the
  worktree prefix failed. Apply: `pwd -P` (resolve symlinks) on both the path and the prefix before
  stripping a git-worktree prefix — the same discipline the `cleanup` provenance guard needs.

- 2026-06-19 (#21, PR #34) — A spec's stated *rationale* for a scope boundary was factually wrong
  (§3 claimed the convention's `.docket.yml` example "does not enumerate `finalize:`" — it does),
  yet the boundary it justified (keep the new knob's doc in finalize's own SKILL, not the convention)
  was still sound on other grounds (the gate/test_command doc-ownership precedent). Apply: when a
  build finds a spec's reason false but its conclusion defensible, record the discrepancy in the
  results file — do NOT silently "fix" (override) an explicit spec scope boundary mid-build; leave
  the re-scope to the human.

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

- 2026-06-17 (#15, PR #32) — A config enum value (`gate: off`) collides with a YAML 1.1 boolean —
  safe under docket's grep/awk reads (it stays the literal string "off"), but it would parse as
  `false` under a real YAML loader. Apply: a config value that is a YAML boolean keyword
  (on/off/yes/no/true/false) must be quoted or avoided once a YAML library is in play (flagged for #0018/yq).

- 2026-06-17 (#17, PR #31) — An `## Update` to an already-published, immutable ADR (0008) had to
  reach the integration branch alongside a NEW ADR (0009) it cross-references, without a premature
  direct-to-`main` push (which would dangle the `[[0009]]` link until the new ADR merged). Apply:
  to deliver an ADR body update onto the integration branch atomically, list that ADR id in the
  producing change's `adrs:` so terminal-publish re-copies it on merge — never a standalone
  push that races the cross-referenced ADR's own publish.

- 2026-06-16 / 2026-07-08 (#11 PR #11; #16 PR #30; #46 PR #56 — merged, one pipefail family) — A test
  piped a live-producing script straight into `grep -q`; grep exits on first match, the still-writing
  producer takes SIGPIPE, and `pipefail` turned that 141 into an intermittent failure — review later
  found the same shape with `head`, and #46 hit it again in production code (`printf … | section_body`,
  whose consumer `exit`s early; guarded with `|| true`). Apply: never `producer |
  early-exiting-consumer` (`grep -q`, `head`, `head -n1`, or any reader that may stop before EOF)
  under `set -o pipefail` — capture into a variable first, then grep/`head <<<"$var"`.

- 2026-06-16 (#11, PR #11) — Two idempotency bugs in one derived-surface mirror. It keyed idempotency
  on a persisted change-file field but did no git writes itself, so a bare run (outside the
  orchestrating pass that records the field) re-created every item — and it read the integration
  checkout where `active/` is pruned, so it only saw archived changes. Separately, its first-sync
  close-state keyed on the *pre-existing* id field (empty on a fresh mint), so an already-terminal
  item was created open and closed only on a later pass. Apply: a script that reads change files must
  read the metadata working tree (guard the pruned tree) and is idempotent only via the orchestrating
  pass's write-back — drive it through that pass, never bare; and when a create-and-set-state pass
  mints an id, key the state write on the EFFECTIVE id (existing OR just-minted), not the stored field.

- 2026-06-12 (#14, PR #10) — A plan's order assertion compared `grep -n` line numbers, which
  cannot order two phrases inside one paragraph; the implementer "satisfied" it by splitting a
  sentence mid-paragraph. Apply: order-assert with byte offsets (`grep -ob`) when both anchors
  can share a line — and treat an implementer contorting the artifact to pass a test as a signal
  the assertion itself is wrong.

- 2026-06-12 (#14, PR #10) — Two views keyed off a body section's *presence* (board cell,
  selection band), but the state transition out (re-arm) didn't remove the section — a re-armed
  stub stayed mislabeled. Apply: when state is encoded by an artifact's presence, every
  transition out of that state must remove the artifact.

- 2026-06-12 (#6, PR #8) — The spec asked for a test asserting a metadata-branch file exists,
  but the suite runs against the integration-branch checkout where that file never lives. Apply:
  when specifying tests for metadata-branch artifacts, verify them at build time and record in
  the results file instead — repo tests can only see the integration branch.

- 2026-06-12 (#12, PR #7) — A code-review finding cited a sentence that did not exist in the
  reviewed file. Apply: verify review claims against the artifact (byte-diff against canonical
  content) before implementing fixes; reject false positives with evidence.

- 2026-06-12 (#12, PR #7) — link-skills.sh needed no edit for a new skill — it globs skills/*/.
  Apply: at reconcile, check whether plumbing auto-discovers before planning an edit to it.

- 2026-06-10 (#5, PR #6) — YAML frontmatter: an unquoted scalar value cannot contain ": "
  (colon-space). Apply: reword with an em-dash or quote the scalar in skill descriptions.

- 2026-06-02–12 (#1, #2, #5, #13) — Foundational sentinel/test discipline (consolidated; richer,
  more specific restatements live in the anchoring and coverage families above): sentinel greps are
  sampling, not parsing — pair them with a whole-branch review that reads for meaning; prove each
  assertion non-vacuous (deleting the clause it guards must flip the test to NOT OK); when order is
  part of the contract, assert it explicitly rather than inferring it from presence; and build inline
  when tasks share one artifact, fanning out only for genuinely independent work.
