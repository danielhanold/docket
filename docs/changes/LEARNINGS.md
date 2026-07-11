<!-- LEARNINGS.md — the learnings ledger (contract: docket-convention, "Learnings ledger").
     Build-loop memory: lessons harvested at change close-out, read at groom, plan, and review
     time. Durable project conventions belong in CLAUDE.md — promotion during a distill removes
     the entry here. Newest first. Soft cap ~300 lines; the first harvest past the cap also
     distills (compression, not destruction — git history keeps whatever is dropped). -->

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

- 2026-07-11 (#60, PR #70) — 0060's original spec (a `render-board.sh --out` atomic-write mode +
  migrating every Board-pass call site) was ~90% delivered by sibling 0059, which merged first;
  reconcile correctly folded 0060 down to the single residual sub-case 0059 left — the non-empty
  half of the guard — instead of killing it or rebuilding the overlap. Apply: when a sibling ships
  the bulk of an unstarted change's scope, don't reflexively kill it — reconcile it down to the
  residual (here: one defensive assertion), and if the residual is genuinely covered, kill; but a
  real unimplemented sub-case is a legitimate small change worth keeping. (Counterpart to the #59
  lesson below on designing around unmerged siblings.)

- 2026-07-11 (#59, PR #64) — 0059 was designed around a still-`proposed` sibling (0058) "later"
  composing its board-refresh gate, but 0058 merged first and independently built the same gate,
  inverting 0059's scope twice; and because 0059 touched every skill file, three slim PRs
  (0054/0055/0056) merged mid-flight and re-slimmed its exact target lines. Apply: never design a
  change around what a still-proposed sibling will do — reconcile against what has actually merged,
  not the planned backlog; and for the most conflict-prone change in flight (touches many shared
  files), hold it and rebase it LAST, once, onto the settled base rather than reconciling repeatedly.

- 2026-07-11 (#56, PR #68) — Adding the 9th generated wrapper (docket-brainstorm-consultant)
  turned a hardcoded count into stale prose+tests in many places at once: the convention's
  "eight wrappers / three no-skill" line, one assertion in test_finalize_gate, and seven
  "8 built-in wrappers" assertions in test_sync_agents — all had to move to nine / four-no-skill,
  and the full suite (not the change's enumerated sentinels) was what proved none were missed.
  Apply: a literal count of an enumerated set (wrappers, states, skills) repeated across
  convention prose and test assertions is a cross-cutting invariant — before adding a member,
  grep the whole repo for the current number and update every copy in the same change.

- 2026-07-11 (#55, PR #67) — A behavior-neutral skill slim (docket-implement-next + the four
  small skills) landed all five files modestly over the spec's line-count targets
  (implement-next 137→107 vs ≤~100, groom-next 77→75 vs ~65, adr 88→86 vs ~78); the whole-branch
  review confirmed the residual was load-bearing/test-anchored content (selection bands, the recap
  contract, the four ADR publish contracts, SHA-compare narration), not un-cut prose — the spec's
  size estimates were simply optimistic. Apply: on a behavior-neutral slim, the size target is a
  direction, not a gate — once review shows the remaining lines are load-bearing or test-anchored,
  accept the size and stop trimming; behavior-neutrality outranks hitting the number.

- 2026-07-11 (#54, PR #66) — A behavior-neutral skill slim (docket-finalize-change, 234 → 114
  lines) passed its own goal-scoped whole-branch review, yet the finalize gate's **full** suite
  caught a regression the spec's 7 enumerated sentinel tests did not: the slim had dropped
  finalize's required `render-change-links.sh` mention, reddening `test_change_links_coverage` — a
  test outside the anticipated set. The #52 pattern (a goal-scoped rewrite passes its own audit
  while an out-of-goal dimension slips), caught here by the gate machinery, not a human. Apply: run
  the WHOLE suite at the merge/build gate, never only the tests the spec enumerated as touched — the
  sentinel list is a floor (cf. #42), and a regression in an out-of-goal dimension is exactly what
  the tests *outside* that list exist to catch.

- 2026-07-11 (#58, PR #65) — A `gh api graphql` jq path read one level too shallow
  (`.data.pN.mergedAt` vs `.data.pN.pullRequest.mergedAt`); the bug was masked because the test
  mock returned a *flattened* JSON shape `gh` never actually emits, so the test passed against a
  fiction. Apply: a tool-output mock must mirror the real tool's exact response shape (nesting and
  all) — a mock shaped to match the code under test rather than the real tool validates nothing and
  hides production parse bugs.

- 2026-07-10/11 (#51 PR #60; #57 PR #63 — merged, re-hit class) — An awk/sed **range** edit
  (`/start/,/end/`) over a marker-bounded "do-not-hand-edit" managed block is a data-loss hazard
  whenever the end marker is lost (truncation / bad merge) or the markers are out of order
  (END-before-START, same spelling): the range runs to EOF and silently deletes the user's own
  content after the dangling start (`.gitignore` bytes here). A guard checking marker *presence*
  alone is bypassed by the corrupted block. Apply: before stripping/rewriting a marker-delimited
  block, validate marker *order and balance* — refuse-and-warn on dangling / out-of-order / nested /
  unbalanced markers (either spelling) and leave the file untouched; never presence alone, never let
  the range consume to EOF.

- 2026-07-10 (#52, PR #61) — A critique-driven README rewrite audited hard against its three named
  goals (accuracy, structure, newcomer clarity) and shipped clean — but a dimension *outside* that
  goal set slipped through: the prose stayed Claude-centric for a tool that first-classes three
  agent harnesses (Claude Code, Cursor, Codex), and the owner caught it at the merge gate, not the
  build. The accuracy audit verified every claim was *true*, which a single-harness framing can be.
  Apply: a goal-scoped rewrite only examines the dimensions in its goal set — anything outside it
  (here, harness-neutrality) passes through unaudited even when every claim is correct. When
  documenting a multi-harness/multi-backend tool, add "neutral across the supported set" as an
  explicit rewrite goal, and default narrative examples to the *neutral* term, naming a specific
  harness only when the mechanic is genuinely specific to it.

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

- 2026-07-08 (#46, PR #56) — Two latent bugs rode in from the plan's own awk and surfaced only at
  task review, not at planning: `ind()` used `[^ ]` (a literal-space class) so a **tab-indented**
  config layer was silently dropped — fixed to `[^[:space:]]` (both awk copies) with a tab-indented
  regression test; and `printf … | section_body` SIGPIPE'd because the consumer `exit`s early —
  guarded with `|| true`. Apply: when a plan hands you awk/shell it authored, treat whitespace
  character classes and any producer piped into an early-exiting consumer as suspect — test
  tab-indented input, and guard pipes whose reader may stop reading before EOF.

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

- 2026-06-21 (#36 PR #47; #37 PR #48 — twin sentinel-failure modes, merged) — Two dual failures of
  literal-substring doc-sentinels, both caught in review: (false-RED) a `! grep "must-not-say-X"`
  absence-sentinel fired on X appearing legitimately in a *contrastive* clause ("…deliberately
  divergent from finalize's `non-zero ⇒ abort-and-report`…"), which a blunt absence-grep can't tell
  from the forbidden *adopted* posture; (false-GREEN) a `grep -q "literal"` presence-sentinel stayed
  green when a prose strip **relocated** the must-preserve substring into an unrelated docket-mode
  bullet, producing a factually-wrong sentence. Apply: assert doc intent with a POSITIVE anchor on the
  meaningful framing (e.g. `grep -qE "deliberately divergent from .?docket-finalize-change"`), keep a
  must-preserve substring in its grammatical location (never relocate/jam it to pass), and never rely
  on a blunt `! grep`/`grep -q` over a literal that can legitimately appear elsewhere in the same doc.

- 2026-06-21 (#38, PR #46) — A test grepped for a CLI flag with `grep -qE "\-\-yes\b|\b-y\b"`; the
  `\-` over-escaping silenced one trap (a bare ERE that *leads* with `--` is parsed as a grep
  **option**, not a pattern — `unrecognized option`, exit 2) only by springing another (GNU grep's
  `stray \ before -` stderr warning; BSD grep stays silent, so it hides on macOS). Naively dropping
  the backslashes re-opens the option-parse trap. Apply: to grep for a `--flag`, declare the pattern
  with `grep -E -e "<pat>"` — POSIX `-e` makes the next arg a pattern, never an option; clean (exit 0,
  empty stderr) on **both** GNU and BSD grep. Never lead a bare ERE with `--`.

- 2026-06-21 (#34, PR #45) — finalize's local merge gate ran the suite RED on the change's own
  drift-guard test — but only because `DOCKET_SCRIPTS_DIR` was *exported* in the interactive shell
  (this change's `install.sh` writes it to the shell profile). The test's `${VAR:?}` fail-loud
  assertions assume the var is unset; the ambient export was inherited by the test's sub-shells and
  masked them. A clean-env re-run (`env -u DOCKET_SCRIPTS_DIR`) was all-green — the merged result
  was never broken. Apply: when the gate's suite tests a `${VAR:?}`/fail-loud path for a var the
  feature itself injects into the dev shell, re-run RED tests under `env -u VAR` before trusting the
  failure — and author such tests to `env -u VAR` their own fail-loud sub-shells so a dev shell with
  the feature installed never false-REDs them.

- 2026-06-21 (#35, PR #44) — A close-out feature that re-renders derived content *into the change
  file* (here the `## Artifacts` block re-pointing plan/results after the feature branch is gone) was
  wired to run terminal-publish **before** the re-render committed to `origin/docket`. Since
  terminal-publish copies the archived change file *from `origin/docket`*, it would have published
  the **stale** block onto the integration branch — defeating the re-point on the exact public
  surface it targets. The `docket-status` sweep was already ordered correctly; finalize had to
  converge to it (commit `2d74b88`). Apply: any close-out step that mutates a change file's derived
  content must commit+push to `origin/docket` **before** terminal-publish copies from it —
  terminal-publish publishes whatever is on `origin/docket` at copy time, never the working tree.

- 2026-06-21 (#35, PR #44) — A `/tmp`-fixture smoke of a renderer whose output branches on git
  remote resolution silently exercised only the no-remote (bare-path) fallback — `/tmp` has no
  origin, so the GitHub-mode link path was never hit until the smoke was re-run *inside a real
  worktree*. Apply: when a script's behavior depends on `git remote`/origin-URL resolution, run its
  real-data smoke inside an actual repo worktree, not a `/tmp` fixture dir — the fixture path passes
  while covering only the degraded branch (sharpens #22's "smoke against real data").

- 2026-06-19/20 (#22 PR #35; #32 PR #43 — merged near-duplicates) — Twice, a shell accessor that
  dropped the trailing newline (`printf '%s'` where the original ended `'%s\n'`) shipped masked:
  every `$(…)` command-substitution caller strips trailing newlines anyway, and only a **direct-pipe**
  caller (`field … | sort`) exposed it — the missing `\n` concatenating ids (`10`,`12` → `1012`).
  Apply: a "byte-identical"/drop-in claim for a shell helper must be validated against a direct-pipe
  caller, not only `$(…)` callers — audit for un-captured pipe/concatenation uses before the swap.

- 2026-06-20 (#30, PR #41) — Scripting a "this file is generated — do not hand-edit" ADR index
  surfaced that the published `README.md` had silently accreted richer text than the source `title:`
  fields (backticks on one row, trailing clauses on two more) — the new generator emits each
  frontmatter field verbatim, so all of it is dropped on the first regen. The plan predicted one
  drifted row; the real-data run found three. Apply: a generated index/board renders its source field
  verbatim, so any embellishment a human wants to keep must live in that source-of-truth field (the
  ADR/change frontmatter), never typed into the generated file — and expect a fresh generator's first
  real run to surface more pre-existing hand-drift than a code read predicts.

- 2026-06-19 (#27, PR #39) — A change promised its locally-written file (`.claude/settings.local.json`)
  would "never be committed onto collaborators," but on the build machine that guarantee only held
  because a *user-global* excludesfile (`~/.config/git/ignore`) ignored it — the repo `.gitignore` did
  not. Reconcile caught it; unfixed, a collaborator without that global ignore could have committed the
  file, defeating the change's whole point. Apply: an "every clone / never committed" guarantee must
  rest on a committed repo `.gitignore` entry, never a per-machine user-global ignore — and when a
  change *generates* such a file, add the ignore in the same change (the migrate step here) so the
  guarantee ships with the feature instead of silently depending on each dev's box.
- 2026-06-19 (#26, PR #38 close-out) — One botched hand-rolled close-out, two lessons, both now
  structurally prevented. (a) A **stale** symlinked finalize skill ran: `~/.claude/skills/*` symlink
  into the docket clone's primary checkout, which in docket-mode is never fast-forwarded (work lives
  in `.docket/` + worktrees), so it sat 39 commits behind `origin/main` and loaded the *pre-0025*
  manual-archive skill — and a grep of that stale tree wrongly "proved" the rewire never landed (it
  had). (b) The hand-rolled archive staged the `git mv` rename but DROPPED the follow-on `status: done`
  edit: the `git add` listed the already-moved `active/` path beside the `archive/` path, the
  non-matching pathspec aborted the whole `git add` (staging nothing), and the rename-only commit
  (still `status: implemented`) rode terminal-publish onto `main`. Apply: (a) verify a skill's content
  against `origin/<integration_branch>`, never the local symlinked working copy, and keep the clone
  current (fixed by change 0029's FF at the two merge sites); (b) stage the `archive/` file alone,
  gated on a `git diff --cached` showing `status:` actually changed — the extracted `archive-change.sh`
  exists precisely to remove this hand-staging failure mode.
- 2026-06-19 (#26, PR #38) — A `.docket.yml` reader interpolated the lookup key straight into an ERE
  (`^[[:space:]]*<key>`), so any future key carrying a regex metacharacter would match unintended
  lines; the same unescaped helper is still copy-pasted in `migrate-to-docket.sh`. Apply: escape ERE
  metacharacters in a key before building a `grep -E`/regex match from it — and when you fix one copy
  of a duplicated shell helper, note the un-fixed twin so the divergence is tracked, not silent.
- 2026-06-19 (#26, PR #38) — Hermetic shell fixtures that `git clone` a freshly-`init --bare` origin
  each emit `warning: You appear to have cloned an empty repository` to stderr, so a passing suite
  still had noisy non-empty stderr (16× here). Apply: silence the empty-bare-clone warning in fixture
  builders so a green run leaves stderr 0-byte — pristine stderr is what makes a real-data smoke test's
  "0-byte stderr" assertion meaningful (extends #22).
- 2026-06-19 (#25, PR #36) — An in-place `sed` that sets a frontmatter field (`status:`/`updated:`/
  `results:`) was unanchored, so it would have rewritten *any* column-0 match — including body prose,
  a live risk for docket's own change/ADR files (which discuss those field names). Apply: anchor a
  frontmatter-field edit to the first `---…---` block, never a bare line match — and lock it with a
  test where a body `status:` line survives verbatim while the frontmatter field is set.
- 2026-06-19 (#25, PR #36) — A script resolving a worktree-relative path built from `mktemp` matched
  nothing on macOS: `mktemp` yields `/var/…` but git reports `/private/var/…`, so stripping the
  worktree prefix failed. Apply: `pwd -P` (resolve symlinks) on both the path and the prefix before
  stripping a git-worktree prefix — the same discipline the `cleanup` provenance guard needs.
- 2026-06-19 (#25, PR #36) — Extracting an intricate git dance into a script is meant to make its hard
  branch testable, yet the CAS conflict-retry branch shipped green-but-uncovered: the competing-writer
  test touched an *unrelated* file, hitting only the clean if-branch. Apply: to cover a conflict/CAS-
  retry path the competing writer must DIVERGE the *same* contended path, mutation-confirmed by
  neutralizing the re-checkout (extends #22 — green tests ≠ the hard branch was exercised).
- 2026-06-19 (#22, PR #35) — A green golden-fixture test still shipped two real-data bugs: the fixture
  used `pr: 142` (real changes store a full `pr:` URL) and had a single `done` change, so neither the
  URL-format path nor the multi-id concatenation bug was exercised; a live smoke test against the real
  backlog caught both. Apply: a golden fixture for a deterministic renderer must (a) use **real-shaped
  field values** (full-URL `pr:`, not a bare number) and (b) include **plurality** — ≥2 of every kind
  it renders as a list — and the renderer must be **smoke-tested against real data before merge**; the
  fixture is necessary but not sufficient (extends #20/#15 mutation testing to the data dimension).

- 2026-06-19 (#21, PR #34) — A spec's stated *rationale* for a scope boundary was factually wrong
  (§3 claimed the convention's `.docket.yml` example "does not enumerate `finalize:`" — it does),
  yet the boundary it justified (keep the new knob's doc in finalize's own SKILL, not the convention)
  was still sound on other grounds (the gate/test_command doc-ownership precedent). Apply: when a
  build finds a spec's reason false but its conclusion defensible, record the discrepancy in the
  results file — do NOT silently "fix" (override) an explicit spec scope boundary mid-build; leave
  the re-scope to the human.
- 2026-06-19 (#21, PR #34) — A doc sentinel was non-vacuous yet *double-guarded*: one grep was
  satisfied by two independent clauses (a YAML config-comment line AND a prose sentence), so deleting
  the substantive prose left it green while a whole-region mutation test still "proved" it
  non-vacuous. Apply: one assert anchors to exactly ONE clause it owns; if a pattern can be satisfied
  from two independent locations, split into separately-anchored asserts and mutation-test EACH clause
  in isolation (the per-clause refinement of #2's non-vacuity rule and #20/#15's mutation testing).
- 2026-06-17 (#20, PR #33) — Invoking a skill presents only its `SKILL.md`; sibling files are NOT
  auto-loaded, so a section moved out for progressive disclosure leaves every consumer's context
  unless something Reads it. Apply: extract only a section that is heavy AND off the common path
  (opt-in, or its work is script-delegated — like the GitHub mirror → `github-mirror.sh`); leave a
  stub + pointer under the original heading so name-based cross-refs still resolve, and add a pointer
  in the one consumer that needs the mechanics. Verify the MOVE by byte-diffing the sibling against
  the base section and mutation-testing each new grep assertion (extends #5).
- 2026-06-17 (#15, PR #32) — A read-only review subagent ran `git checkout <sha>` in the SHARED
  feature worktree to inspect a diff, detaching HEAD; the controller's later commits (plan, results)
  landed on the detached HEAD, the branch ref stayed put, and a plain `git push` of the branch
  silently published only the pre-detach tip (the PR was briefly missing files). Apply: review/
  inspection subagents must NOT `git checkout` in a shared worktree (use `git show`/`git diff <sha>`,
  or a throwaway worktree); after every push the controller SHA-compares local vs origin AND checks
  `git symbolic-ref -q HEAD` is the feature branch — never trust the push exit code alone.
- 2026-06-17 (#15, PR #32) — An ordering sentinel grepped a broad keyword OR-set
  (`run the suite|validate|local` + a separate `before` filter) and latched onto an unintended
  EARLIER line (step-1 prose), passing vacuously no matter what the real clause said. Apply: anchor
  an order/presence assert to the UNIQUE phrase its target clause owns (e.g. "before any push"), not
  a broad keyword set — and mutation-test it by deleting the real clause (must flip to NOT OK).
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
- 2026-06-16 (#16, PR #30) — A generator test set `DOCKET_HARNESS_ROOT` to the same dir as the
  repo root, so the user-level pass and the project-level pass both wrote one `.claude/agents/`
  and an "unlisted skill gets no project file" assertion passed vacuously through a weak `|| diff`
  arm. Apply: when a tool writes to BOTH a user/harness location and a project/repo location, give
  the test SEPARATE dirs — a shared dir lets one pass's output mask the other's bugs.
- 2026-06-16 (#11 PR #11; #16 PR #30 — merged near-duplicates) — A test piped a live-producing
  script straight into `grep -q`; grep exits on first match, the still-writing producer takes
  SIGPIPE, and `pipefail` turned that 141 into an intermittent failure — and review later found the
  same shape with `head`. Apply: never `producer | early-exiting-consumer` (`grep -q`, `head`,
  `head -n1`) under `set -o pipefail` — capture into a variable first, then grep/`head <<<"$var"`.
- 2026-06-16 (#11, PR #11) — A derived-surface mirror keyed idempotency on a persisted change-file
  field but did no git writes itself, so a bare run (outside the orchestrating pass that records
  the field) re-created every time — and it was pointed at the integration checkout where `active/`
  is pruned, so it only saw archived changes. Apply: a script that reads change files must read the
  metadata working tree (guard the pruned tree) and is idempotent only via the orchestrating pass's
  write-back — drive it through that pass, never bare.
- 2026-06-16 (#11, PR #11) — First-sync close-state keyed on the *pre-existing* id field (empty on
  a fresh mint), so an already-terminal item was created open and only closed on a later pass.
  Apply: when a create-and-set-state pass mints an id, key the state write on the effective id
  (existing OR just-minted), not the stored field.
- 2026-06-12 (#14, PR #10) — A plan's order assertion compared `grep -n` line numbers, which
  cannot order two phrases inside one paragraph; the implementer "satisfied" it by splitting a
  sentence mid-paragraph. Apply: order-assert with byte offsets (`grep -ob`) when both anchors
  can share a line — and treat an implementer contorting the artifact to pass a test as a signal
  the assertion itself is wrong.
- 2026-06-12 (#14, PR #10) — Two views keyed off a body section's *presence* (board cell,
  selection band), but the state transition out (re-arm) didn't remove the section — a re-armed
  stub stayed mislabeled. Apply: when state is encoded by an artifact's presence, every
  transition out of that state must remove the artifact.
- 2026-06-12 (#14, PR #10) — Adding a member to an enumerated set left two stale counts ("six
  skills" in README from 0012, "six operating skills" in the convention from 0014's own edit).
  Apply: when a set gains a member, grep the repo for the old count word and the enumeration.
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
  more specific restatements live in #15, #21, #36, #37 above): sentinel greps are sampling, not
  parsing — pair them with a whole-branch review that reads for meaning; prove each assertion
  non-vacuous (deleting the clause it guards must flip the test to NOT OK); when order is part of the
  contract, assert it explicitly rather than inferring it from presence; and build inline when tasks
  share one artifact, fanning out only for genuinely independent work.
