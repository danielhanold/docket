<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0167 — Lean profile-routed build — fresh task workers without review loops](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-30-0167-lean-profile-routed-build.md)**
<!-- docket:backlink:end -->

# Lean profile-routed build — results

Change: #0167 · Branch: feat/lean-profile-routed-build · PR: (opened at close of this run) · Plan: docs/superpowers/plans/2026-07-30-lean-profile-routed-build.md · ADRs: 63 (supersedes 23)

## Verify (human)

The automated suite (71 files) is green, but this change alters how docket builds itself, and its
opt-in activates the moment the PR merges. These checks cannot be automated from the branch.

- [ ] **Install before merging, not after.** Run `bash ~/dev/docket/install.sh` and start a **fresh**
      Claude Code session. Skills and agents register at process start, so a running session will not
      see them. Then confirm all five artifacts exist: `~/.claude/skills/docket-build`,
      `~/.claude/skills/docket-build-task`, and `~/.claude/agents/docket-build-{economy,standard,premium}.md`.
- [ ] **Understand that the opt-in is latent until merge.** `docket-config.sh` reads `.docket.yml` via
      `git show origin/HEAD:.docket.yml`, so `SKILL_BUILD` stays `superpowers:subagent-driven-development`
      for this repo until the PR lands — then flips to `docket-build` with no gradual rollout. Any
      `docket-implement-next` already in flight picks it up at its next Step-0 export.
- [ ] **Watch the first real build.** Confirm one routing line per task (profile + reason), exactly one
      worker dispatch per task with no task reviewer, and the full suite running **once** at the end.
      The suite running per-task would mean a worker ignored the focused-tests rule; no routing lines
      would mean the controller never reached its rubric.
- [ ] **Know the failure shape.** If a prerequisite was missed, the build halts with the change left
      `in-progress` and the worktree preserved. That is safe, but it presents as an unexplained stall
      rather than a loud error.

## Findings

- **The change's own thesis was validated mid-build, unintentionally.** SDD's five task-scoped reviews
  plus its whole-branch review all passed, and docket's *separate* Step-6 review — the one this change
  preserves — then found five Important contract defects none of them could see. Every one was an
  off-happy-path **disposition** gap: the contract stated a predicate ("a task without a commit is not
  complete") where it owed an action. The difference was method: the task reviews read the skills as
  grep targets, the Step-6 review read them as an operator executing from a cold start. That is
  concrete evidence for keeping one independent whole-branch review as the sole review gate while
  removing the per-task reviewer loop.

- **Guard anchoring was the dominant defect class, in both directions.** Across four fix rounds the same
  failure recurred: a presence-anywhere grep survives deletion of the very rule it guards, because the
  phrase also appears in the frontmatter `description:`, a summary line, or a template block. Deleting
  all three `## Outcomes` bullets once left all three outcome asserts green; an unanchored `no`
  alternative matched inside `Nothing`, letting a full inversion of the no-review rule pass. Each was
  found only by mutation, never by reading. The `agents.default` guard was additionally **vacuous** —
  its extraction returned the empty string on the real file, so the negative assert could not fail.

- **A count-based guard nearly laundered a real defect.** `.docket.yml`'s slim-file budget was raised
  40→45 to fit an `agents.claude` block that turned out to be a pure mirror of the shipped built-ins,
  resolving nothing new and unguarded by the ADR-0039 mirror-equality loop. Deleting it put the file at
  34 lines. The budget existed precisely to stop that accretion; bumping it was the cheapest path to
  green. Removed rather than re-budgeted.

- **The plan's enumeration of count sites was a floor, not a census.** The roster moved 9 → 12; the plan
  named five surfaces and a whole-repo grep found four more (`test_sync_agents.sh`'s second assert,
  `test_sync_agents_cursor.sh`, `test_sync_agents_codex.sh`, a prose comment in
  `test_docket_example_yml.sh`). One brief-named README site was additionally reported as edited when it
  had not been — caught only because the reviewer re-read the file instead of trusting the report.

- **ADR-0063** records the decision and supersedes ADR-0023, whose premise this change reverses:
  ADR-0023 aimed to name the models *inside* SDD's topology, but no choice of model IDs reduces a
  review loop. The lever was the topology. ADR-0023's status change was published to `main` during
  this run.

## Follow-ups

Auto-captured as stubs (both `discovered_from: 167`):

- **#0171** (`refactor`) — settle a reflow-tolerant house pattern for prose-anchored guards. Several
  asserts are line-scoped, so a cosmetic rewrap false-reddens them while the rule is untouched — the
  mirror of the too-loose presence grep, and the same root cause: anchoring on prose layout rather than
  on a stable syntactic feature.
- **#0172** (`chore`) — normalize the banned `producer | grep -q` / `| head` shape under `pipefail`
  across tests and helpers. Every known instance is benign today; the risk is that the banned shape
  reads as sanctioned house style and gets copied somewhere it matters.

Parked with rulings, deliberately not fixed here:

- Declined to restate AGENTS.md's "key guards on syntactic shape" rule inside `docket-build-task`'s
  contract — duplicating a rule that already has an owner is the pattern this change exists to remove,
  and the worker is told AGENTS.md overrides it.
- `README`'s "eleven directories under `skills/`" was written in Task 3 as a forward reference and only
  became true when Task 4 added `skills/docket-build/`. Verified true at the final commit.

Deferred to the follow-up changes that already existed before this build:

- **#0168** (Cursor) and **#0169** (Codex) profile-dispatch support. The dispatch plumbing generates for
  every configured harness, but only the Claude model IDs are shipped and validated; the commented
  `codex:`/`cursor:` rows in `.docket.example.yml` are unvalidated examples and say so.
- **#0170** — a lean Docket-owned `skills.review` replacement. Deliberately not attempted here so the
  build redesign could be measured without simultaneously changing the sole remaining review gate.

## Notable plan deviations

- **Change 0044 and the three follow-up changes were already closed** before the build began (0044
  killed as superseded the same day; 0168/0169/0170 already minted), so both scope items were dropped at
  reconcile rather than executed.
- **The spec's fourth verification level — a live Claude Code smoke test** through all three profiles
  with a real escalation — was not run. It requires executing a multi-task fixture *under* the new
  controller, which cannot happen from this branch (the opt-in is latent until merge). The whole-suite
  run plus a real `sync-agents.sh` generation-and-idempotence check are the automated substitute; the
  residual gap is the first-build check in **Verify (human)** above.
- **`skills.build` for this repo is the only opt-in.** The shipped cross-harness default remains
  `superpowers:subagent-driven-development`, so a user who does nothing sees no behavior change.
