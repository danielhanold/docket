<!-- LEARNINGS.md — the learnings ledger (contract: docket-convention, "Learnings ledger").
     Build-loop memory: lessons harvested at change close-out, read at groom, plan, and review
     time. Durable project conventions belong in CLAUDE.md — promotion during a distill removes
     the entry here. Newest first. Soft cap ~300 lines; the first harvest past the cap also
     distills (compression, not destruction — git history keeps whatever is dropped). -->

- 2026-06-19 (#22, PR #35) — A shell-helper refactor claimed "byte-identical" but had dropped a
  trailing newline (`printf '%s'` replacing a final `sed`); every `$(field …)` caller masked it
  (command substitution strips trailing newlines), and only a **direct-pipe** caller (`field … | sort`)
  exposed it — there the missing `\n` concatenated multiple ids into one token (`printf: Result too
  large`). Apply: a "byte-identical" claim for a shell helper must be validated against a direct-pipe
  caller, not only `$(…)` callers — `$()` silently hides a dropped trailing newline.
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
- 2026-06-17 (#17, PR #31) — Skill-body prose pinned literal model/effort tiers ("dispatch the
  critic, pinned opus/xhigh via its wrapper") for the dispatched subagents — a second source of
  truth for a value whose home is the wrapper frontmatter + layered `.docket.yml`/global config,
  so it drifts silently the moment a repo overrides the tier (the whole point of the agent layer).
  Caught by the human at the merge gate, not the build. Apply: a layer that makes a value
  config-overridable must NOT restate that value in prose — name the source ("at the model/effort
  its wrapper resolves") and guard with a regex test that no `alias/effort` literal appears in the
  dispatch prose.
- 2026-06-17 (#17, PR #31) — An `## Update` to an already-published, immutable ADR (0008) had to
  reach the integration branch alongside a NEW ADR (0009) it cross-references, without a premature
  direct-to-`main` push (which would dangle the `[[0009]]` link until the new ADR merged). Apply:
  to deliver an ADR body update onto the integration branch atomically, list that ADR id in the
  producing change's `adrs:` so terminal-publish re-copies it on merge — never a standalone
  push that races the cross-referenced ADR's own publish.
  repo root, so the user-level pass and the project-level pass both wrote one `.claude/agents/`
  and an "unlisted skill gets no project file" assertion passed vacuously through a weak `|| diff`
  arm. Apply: when a tool writes to BOTH a user/harness location and a project/repo location, give
  the test SEPARATE dirs — a shared dir lets one pass's output mask the other's bugs.
- 2026-06-16 (#16, PR #30) — review found a `producer | head` (not just `grep -q`) that could 141
  under `pipefail` — `head` closes the pipe early too. Apply: the no-`producer | grep -q` rule
  generalizes to ANY early-closing consumer (`head`, `head -n1`) — capture into a var, then
  `head -n1 <<<"$var"`.
- 2026-06-16 (#11, PR #11) — A test piped a live-producing script straight into `grep -q`; grep
  exits on first match, the still-writing producer takes SIGPIPE, and `pipefail` turned that 141
  into an intermittent failure. Apply: capture a producer's output into a variable first, then
  grep the variable — never `producer | grep -q` under `set -o pipefail`.
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
- 2026-06-12 (#13, PR #9) — A sentinel test guarded every clause of an ordered procedure but
  would have false-passed had the clauses been reordered; review caught it. Apply: when order is
  part of the contract, assert it — compare `grep -n` line numbers, don't just grep for presence.
- 2026-06-12 (#6, PR #8) — The spec asked for a test asserting a metadata-branch file exists,
  but the suite runs against the integration-branch checkout where that file never lives. Apply:
  when specifying tests for metadata-branch artifacts, verify them at build time and record in
  the results file instead — repo tests can only see the integration branch.
- 2026-06-12 (#12, PR #7) — A code-review finding cited a sentence that did not exist in the
  reviewed file. Apply: verify review claims against the artifact (byte-diff against canonical
  content) before implementing fixes; reject false positives with evidence.
- 2026-06-12 (#12, PR #7) — link-skills.sh needed no edit for a new skill — it globs skills/*/.
  Apply: at reconcile, check whether plumbing auto-discovers before planning an edit to it.
- 2026-06-10 (#5, PR #6) — A full convention restatement hid in paraphrase ("satisfied = done")
  where fixed-string sentinels could not see it. Apply: sentinel greps are sampling, not parsing;
  pair them with a whole-branch review that reads for meaning.
- 2026-06-10 (#5, PR #6) — YAML frontmatter: an unquoted scalar value cannot contain ": "
  (colon-space). Apply: reword with an em-dash or quote the scalar in skill descriptions.
- 2026-06-04 (#2) — A backward-compat test assertion was vacuous (any "main-mode" mention
  satisfied it). Apply: prove each assertion non-vacuous — deleting the clause it guards must
  flip the test to NOT OK.
- 2026-06-02 (#1) — Fragmenting a tightly-coupled single-artifact edit across subagents risks
  inconsistent edits to shared content. Apply: build inline when tasks share one artifact; fan
  out only for genuinely independent tasks.
