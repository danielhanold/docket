# finalize.require_pr_approval layer resolution — results
Change: #102 · Branch: feat/finalize-require-pr-approval-has-no-layer-resolution · PR: <url> · Plan: docs/superpowers/plans/2026-07-21-require-pr-approval-layer-resolution.md · ADRs: 52

## Verify (human)

- [ ] **Word-budget headroom was consumed and then raised — confirm the new number.** This change
      grew `skills/docket-finalize-change/SKILL.md` past its enforced budget (4126 > 4060,
      `tests/test_skill_size_budgets.sh`). It was slimmed back to 4059 — **one word under** — and
      only then was the budget raised 4060 → 4200. Pre-change the file was at 4057, so the original
      margin was 3 words. Confirm 4200 is the headroom you want; the alternative is accepting that
      the next edit to that file reddens CI on arrival.
- [ ] **Confirm the scope call on `require_pr_approval`.** It is now **global-able** — settable in
      `.docket.local.yml` and the global `config.yml` — and deliberately NOT coordination-fenced,
      even though it gates an irreversible shared write (a merge onto the integration branch).
      The precedent is `finalize.gate`, already global-able and gating the same merge. The practical
      consequence: "machine A refuses to merge an unapproved PR, machine B merges it" is now a
      reachable state you chose per machine. ADR-0052 records the reasoning; this is the one
      product decision in the change worth a second look.
- [ ] **Nothing to exercise interactively.** The full suite (53 files) is green and the resolver was
      verified live (`./scripts/docket-config.sh --export` → 25 lines, `FINALIZE_REQUIRE_PR_APPROVAL=false`
      directly after `FINALIZE_TEST_COMMAND`). No manual run needed beyond the two judgment calls above.

## Findings

**The guard's own credibility was the hard part, not the wiring.** The four-rung resolution chain
was straightforward. What consumed the build was proving the drift guard could actually fail —
five successive review rounds each found a way to land a documented-but-unwired key with a fully
green suite. Recording them because the pattern is the lesson:

1. **The sole-channel assert was vacuous** (Task 3). It required a *single line* to contain both
   the key name and the framing string `Configured by .docket.yml` — no line ever did, before or
   after. It stayed green under both mutations it existed to catch: reverting the framing sentence,
   and bolting on an explicit fallback. Replaced with a positive anchor on `never by parsing` plus
   a negative fallback guard, both independently mutation-verified.
2. **`elsewhere:HEADER` was an unverified escape hatch** (Task 4). Nothing checked that a
   HEADER-classified key was actually a block opener, so relabeling `require_pr_approval` as
   `elsewhere:HEADER` left the suite fully green — re-opening the bug class through a one-word
   edit. Now asserts a real bare block opener *with a more-indented child*, which also closed a
   childless-nested-key escape.
3. **`elsewhere:` targets were unconstrained** (whole-branch review). The check grepped an
   arbitrary path, so a new key could anchor on `.docket.example.yml` itself — the very file
   documenting it — making the assert unconditionally true. Targets are now constrained to a
   declared consumer allowlist.
4. **`sort -u` absorbed leaf-name collisions** (whole-branch review). A new key whose leaf name
   matched an already-classified one was invisible: `classify_key` answered for the other key and
   the count never moved. `learnings.gate` (colliding with the flat-read `finalize.gate`) passed
   green — also a real mis-resolution hazard for `yaml_get`'s `head -n1`. Closed by a
   no-duplicate-leaf-names assert.
5. **`resolved:` proved the export existed, never that it belonged to the key** (final review).
   The sharpest one: renaming `finalize.require_pr_approval` → `finalize.require_approval` in the
   example and copy-pasting the classify arm reproduced **the original bug verbatim** with a green
   suite — the old export still emitted, so the assert passed. This is the spec's own
   "manifest gone stale against the script" direction. Closed by tying the export name back to its
   leaf key in the resolver; **independently mutation-verified at HEAD**, not just reported.

**Why this matters beyond this change:** every one of these was found by review, never by the
suite, and each individually looked like a working guard. `guards-are-code` and
`correspondence-guard-runs-one-way` both fired here, repeatedly.

**Other findings:**

- **The manifest caught an error in its own plan.** The plan specified `elsewhere:scripts/sync-agents.sh`;
  that file does not exist (it lives at the repo root). The consumer-anchored check reddened on it —
  good evidence the anchor is real rather than an allowlist.
- **`docket-config.md`'s "never aborts" invariants were already false pre-change.** Both the global
  and machine-local bullets claimed layer problems never abort, but a malformed value for any
  fail-closed boolean (`auto_capture` already, now `require_pr_approval` too) aborts every docket
  command on that machine. Corrected with a carve-out, and the two table rows that failed to
  document their own abort behavior (`learnings.enabled`, `reclaim.auto`) were annotated.
- **Removed a vestigial YAML parser.** `rpa_of()` in `tests/test_finalize_gate.sh` encoded the exact
  "parse `require_pr_approval` out of a yaml file" contract this change deletes, and tested no
  product code. Its replacement is strictly stronger: `rpa_of` returned the default `false` for an
  absent or commented-out key and would have stayed green on both.
- **Five stale `docket-config.sh:NNN` line anchors** were corrected, one of which this change had
  itself written born-stale.
- **ADR-0052** records the decision and its known residual honestly.

## Follow-ups

**Minted as stubs (auto-capture):**

- **#120** — `docket-finalize-change` claims `integration_branch` is read from `.docket.yml`, but it
  is an exported resolver key. The same bug class as this change, one key over. Less severe
  (`integration_branch` IS fenced, so a machine-scoped value is warned rather than silently
  dropped), but the prose is wrong about its own read channel and ADR-0052 now states the rule it
  violates.
- **#121** — the manifest's `elsewhere:` check proves a *word occurrence*, not a real config read.
  Demonstrated: a key classified `elsewhere:sync-agents.sh` passed on `\btimeout\b` matching English
  prose inside an embedded dispatch prompt. Narrowed here (targets constrained to five declared
  consumers) but not closed; documented as a residual in ADR-0052.
- **#122** — nested keys' scope tags in `.docket.example.yml` are unguarded. The scope-tag checker
  walks top-level keys only, and a block header's window is satisfied by any one child's tag. This
  change's two bespoke `require_pr_approval` asserts are currently the only guard on any nested key.

**Capped overflow (mint cap of 3 reached — surfaced, not filed):**

- **Machine-check `scripts/docket-config.md`'s export-LIST order against the resolver's runtime
  order.** `R7` anchors the runtime emission order and `R8` anchors the doc's presence, but nothing
  verifies the fenced list's *sequence* matches emission. Pre-existing; widened by one entry here.
  File this one by hand if you want it tracked.

**Accepted residuals (deliberate, not deferred work):**

- The negative fallback guard is phrasing-bound: it catches the `fall back` family, but a
  semantically identical fallback worded outside that family would pass. The positive `never by
  parsing` anchor is the real guard and is live.
- Hyphenated key names (`notify-slack:`) are invisible to the manifest extractor. Contrived under
  docket's underscore convention.
- The commented-key discriminator extracts exactly `agents` / `agent_harnesses` today. It is now
  rule-based (a commented key sits under its own `# scope:` tag) rather than a hardcoded pair, so a
  new commented key *with* a scope tag is caught — one without a tag still is not.

**Plan deviations:**

- The plan's Global Constraints asked the new rung chain to be a "visual sibling" of the packed
  one-liners above it; the spec's §1 prescribes the four-line form verbatim. Spec governs — kept
  four lines, no churn.
- The plan assumed Task 5 would be a formality. It caught the word-budget regression.
