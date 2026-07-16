# Learnings promotion destination — results
Change: #67 · Branch: feat/learnings-promotion-destination · PR: <url> · Plan: docs/superpowers/plans/2026-07-16-learnings-promotion-destination.md · ADRs: 5, 41

## Verify (human)

The migration and the promotion flip are **metadata-branch** operations, structurally invisible to the
integration-branch suite (LEARNINGS #6). This section is their only gate.

- [ ] **The ledger migration landed and lost nothing.** Already on `origin/docket` (commit `aa267d5`),
      *before* this PR merges — see *The migration ordering window* below. Spot-check:
      `git show origin/docket:docs/changes/learnings/README.md` (the index) and a couple of findings,
      e.g. `guards-are-code.md`, `enumerated-floor.md`. Confirm the war stories read as they did.
- [ ] **Land the promotions — the shrink valve currently ships at ZERO.** All **7** `candidate` findings
      map onto rules already written into `AGENTS.md` by this change, but none is `promoted` yet, because
      `AGENTS.md` cannot exist on `main` until this PR merges. That ordering is by design (the harvest
      never touches the integration branch — ADR-0005), but it means that **on merge**: every one of
      those 7 rules is double-paid (always-in-context *and* on the retrieval surface),
      `learnings promotion-pending 7 — needs you` fires on **every** full `docket-status` pass, all 34
      findings still count against the cap, and `## Promoted` renders never. **Post-merge, flip the 7**
      (`promotion_state: promoted`, `promoted_to: AGENTS.md`) on `origin/docket` — the next
      `docket-status` pass re-renders the index and the valve starts working:
      `guards-are-code`, `enumerated-floor`, `pipefail`, `shell-portability`, `yaml-scalar`,
      `frontmatter-edit-anchor`, `marker-block-range-edit`.
- [ ] **A real harvest.** Close the next change and confirm the harvest creates/extends a finding,
      re-renders the index, commits it as its own commit on `docket`, and that the idempotency probe
      no-ops on a second run.
- [ ] **A real disabled sweep.** Set `learnings: {enabled: false}` (`.docket.local.yml` works — both keys
      are global-able) and run a full `docket-status`: expect exactly one `learnings disabled` line, no
      advisories, no render, and `learnings/` byte-untouched. Then re-enable and confirm it resumes.
- [ ] **`AGENTS.md` is yours now.** It ships with 10 rules distilled from this repo's own findings, each
      verified against real code at review. It becomes always-in-context for every agent in this repo —
      read it and cut anything you disagree with.

## Findings

**Recorded as ADR-0041** (`relates_to: [5]`, not supersedes — ADR-0005's decision stands unchanged; only
its founding *consequence*, "short enough to actually be read", is what failed). ADR-0005 gains a dated
`## Update` note pointing forward, delivered atomically by keeping `5` in this change's `adrs:`.

**Build-time decision worth its own record (ADR-0041, decision 7).** The `learnings:` block mirrors
`finalize:`'s *shape* but the `skills:` block's *parsing*. `finalize.gate` is read by bare leaf-key
(`yaml_get "$CFG" gate`) — safe only because `gate`/`test_command` are unusual words. `enabled` and `cap`
are generic: a bare read would let **any** other block's (or a future top-level) leaf shadow them. Each
leaf is therefore read *within* the block via `yaml_block_body`. Mutation-verified: swapping in bare reads
reddens a dedicated shadow-guard.

**Defects this build found in its own new code:**
- **The renderer's `dequote` was escape-blind.** A valid double-quoted `hook:` containing `\"` leaked
  literal backslashes into the index. Found by the migration hitting it in real data. Fixed (single-pass
  unescape, matched-pair-only stripping); the real migrated index re-renders byte-identical.
- **…and the fix's own comment was false.** The final review replaced the single-pass with the "naive
  two-pass" it warns against and the suite stayed green: `_dq_unescape_dquote` is called with the closing
  delimiter already stripped, so the comment's scenario is unreachable and, for well-formed YAML, the two
  passes are provably equivalent. The single-pass is still right (it degrades predictably on *malformed*
  input), but the guard was decoration. Fixed: added the one discriminating fixture
  (`"path C:\\" and more"` — a bare unescaped quote inside a double-quoted scalar) and corrected the
  comments. A `verify-the-claim` violation inside the change that ships `verify-the-claim`.
- **The harvest could truncate the index.** Step 2.5's `render … > README.md` truncates on open, so a
  render failure left an **empty** index — and then committed it, since "commit only if bytes changed" is
  satisfied by truncation. `docket-status.sh`'s `learnings_regen_index` exists precisely to prevent this,
  but that primitive is private to it. Fixed: the harvest now renders to a temp and `mv`s on success.
- **A failed render silently muted both needs-you advisories.** Over-cap and promotion-pending are
  computed from the finding files and are independent of the render, so the human lost the escalation
  exactly when something was already wrong. Fixed; the advisories now fire independently.
- **A pre-existing self-disabling conditional** hid the board's regen branch: the strong asserts sat
  inside `if grep -q "board inline changed pushed"`, which goes false exactly when the branch degrades.
  Breaking the regen callback gave 234 ok → 232 ok with **0 NOT OK** (asserts vanished rather than
  failed). Fixed to assert unconditionally; the mutation now yields 3 real reddens and also catches the
  sibling conflict-path mutation.

**Plan defects (recorded because the plan is an artifact too).** Four of this plan's own given asserts
were **double-guarded** — anchored on phrases the target file already carried 2-4 times, so deleting the
guarded clause left them green. All were caught by the mandatory `grep -c == 1` singularity check and
re-anchored. One given assert was **case-sensitive** (`read` vs the real prose's `Read`) and missed what
it guarded. The `enumerated-floor` family landed on this plan three separate times: the reader set (I
listed 3 sites; a whole-repo grep found **5** — `docket-auto-groom` and `docket-brainstorm` would have
silently read a dead stub), the family list for the migration (listed 12; the real derivation found
**14**), and the co-located script contracts (the plan listed none; **three** — `docket-config.md`,
`docket.md`, `docket-status.md` — were stale and had to be fixed).

**The board pass is provably intact.** This branch extracted a shared `commit_and_push_generated` helper
out of `board_pass_inline` — the riskiest edit in the change, in docket's most-run orchestrator. Review
proved preservation by mechanically inlining the extraction and diffing against the pre-refactor script:
exactly three deltas, all justified (one is a genuine pipefail fix — `grep -q` on a pipeline could
SIGPIPE git and make a *matching* conflict take the abort branch). Every board report line stays reachable
and identical; the no-op probe still keys on "it reached the remote"
(`git rev-list --count @{u}..HEAD -- <path>`), never a clean-tree proxy.

**The base moved mid-build.** Two concurrent events: change 0084's PR #90 merged ~10 min after this run's
Step-0 sweep read it as unmerged (so the feature branch was cut from a `main` that already contained its
prose — the flagged overlap risk was eliminated rather than managed), and a concurrent session harvested
0084 and **distilled the ledger** (490/33 → 485/34) mid-build. The migration correctly derived from
current state, not from the reconcile's reading.

### Spec deviations

1. **`config.yml.example` deliberately NOT touched**, against spec §4.8/§6.13. The instruction is wrong
   about the real file: its header states it shows *"Only the two harness/model keys … see README ->
   Configuration for every other key"*, and **ADR-0039** pins it as a documented **mirror of the
   `agents/docket-*.md` wrapper defaults**, nothing else — `finalize:`, `board_surfaces`,
   `terminal_publish`, and `skills:` are all likewise absent from it. Adding `learnings:` would contradict
   both. The spec's *intent* (LEARNINGS #49 — a knob ships end-to-end) is honored through the surfaces
   that actually carry knobs: the commented `.docket.yml` sample, the convention's schema block, and
   README. **Flagged for the human: this is a scope call, not a silent override — re-scope if you
   disagree.**
2. **Counts:** the ledger measured **485 lines / 34 entries** at migration (the spec says 491, the plan
   490/33 — both dated by the concurrent distill). Cosmetic; the cap-breach premise is unaffected.

### The migration ordering window

The migration landed on `origin/docket` **at build time** (per spec §4.7/§4.8, it is the acceptance
proof), which is *before* this PR merges. Until merge, the skills installed from `main` still read
`LEARNINGS.md` — now a pointer stub. This is exactly why the design keeps the stub rather than deleting
it (LEARNINGS #20: leave a stub + pointer so name-based cross-refs still resolve, for a human **or an
older skill copy**). The window is real but bounded and self-healing on merge; merging promptly closes it.

## Follow-ups

- **Flip the 7 promotion candidates post-merge** — see *Verify (human)*. Until then the valve is dormant
  and `promotion-pending 7 — needs you` fires every pass. This is the single highest-value follow-up:
  it is what makes the change's headline feature actually do anything.
- **`learnings-refresh.sh` (deferred by design).** The harvest and `docket-status` each own their own
  render→temp→move→commit-if-changed logic. `BOARD.md` solved this with a single gated writer
  (`board-refresh.sh`) that is the sole writer, guarded by a `REDIRECT_RE` scan forbidding anyone else
  from redirecting into it. Learnings has no equivalent, so nothing structurally stops a future caller
  from re-introducing the truncating redirect that was just fixed. Worth a change.
- **`hook:` quality is unlinted.** The index is a hint surface; a hook that under-describes its finding
  means the finding never gets pulled (spec §7). Consider a length/shape lint.
- **`topics:` must be single-token**, silently. `list_field` splits on whitespace, so a multi-word topic
  fragments without warning. Either lint it or teach the renderer.
- **Disabled-machine harvest is lossy by design** (spec §7): because the harvest is one-shot at close-out
  and archived changes are never re-swept, a change closing on a machine with `learnings.enabled: false`
  is permanently un-harvested. Accepted (a strict subset of ADR-0005's omission envelope); revisit only if
  it bites.
- **`docket-status`'s learnings-reader status.** The convention's Readers line names implement-next,
  groom-next, and auto-groom. `docket-status` and `docket-finalize-change` touch the index only as
  writer/self-heal, not as consumers — correct today, but worth re-checking if either grows a read.
