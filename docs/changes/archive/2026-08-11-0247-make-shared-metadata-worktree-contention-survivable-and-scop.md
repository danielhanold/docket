---
id: 247
slug: make-shared-metadata-worktree-contention-survivable-and-scop
title: 'Make shared metadata worktree contention survivable and scope its commits'
status: done
priority: high
type: fix
created: 2026-08-07
updated: 2026-08-11
depends_on: []
related: [8, 118, 253]
discovered_from: [110, 119]
adrs: [89]
spec: docs/superpowers/specs/2026-08-09-make-shared-metadata-worktree-contention-survivable-and-scop-design.md
plan: docs/superpowers/plans/2026-08-11-make-shared-metadata-worktree-contention-survivable-and-scop-plan.md
results: docs/results/2026-08-11-make-shared-metadata-worktree-contention-survivable-and-scop-results.md
trivial: false
auto_groomable: true
branch: feat/make-shared-metadata-worktree-contention-survivable-and-scop
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/200
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-make-shared-metadata-worktree-contention-survivable-and-scop-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-make-shared-metadata-worktree-contention-survivable-and-scop-design.md) |
| Plan | [2026-08-11-make-shared-metadata-worktree-contention-survivable-and-scop-plan.md](https://github.com/danielhanold/docket/blob/feat/make-shared-metadata-worktree-contention-survivable-and-scop/docs/superpowers/plans/2026-08-11-make-shared-metadata-worktree-contention-survivable-and-scop-plan.md) |
| Results | [2026-08-11-make-shared-metadata-worktree-contention-survivable-and-scop-results.md](https://github.com/danielhanold/docket/blob/feat/make-shared-metadata-worktree-contention-survivable-and-scop/docs/results/2026-08-11-make-shared-metadata-worktree-contention-survivable-and-scop-results.md) |
| PR | [#200](https://github.com/danielhanold/docket/pull/200) |
| ADRs | [ADR-0089](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0089-shared-metadata-worktree-contention-survivable-not-impossible.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0110 and #0119 (2026-08-07 triage): the two halves of the shared-`.docket`-worktree concurrency problem. #0119's own auto-groom abstain said its blocking policy decision "should probably be settled alongside #0110" — a per-session-worktree answer to #0110 would delete the shared-vs-exclusive framing #0119's guard is built on. One design conversation, one change.

Verified 2026-08-07:

- **Contention (#0110, priority high).** `scripts/lib/docket-preflight.sh:70-72` is unchanged: `fetch && pull --rebase || return 1` — no retry, no backoff, no dirty-tree discrimination. `grep -rn "flock\|lockfile\|\.lock" scripts/` returns zero hits. The shared worktree is dirty for the whole multi-tool-call edit→commit window of any agent, so a concurrent agent's preflight hard-fails. Observed live during 0109; interactive sessions racing autonomous loops hit it routinely.
- **Blast radius (#0119).** Two pathspec-less commits inside the shared worktree sweep up another agent's staged work: `scripts/docket-status.sh:282` (`commit_and_push_generated`) and `:846` (refresh-artifacts-links). The scoped precedent exists in the same file (`:880`, `-- "$archived"`). `terminal-publish.sh:327` is pathspec-less but in its own dedicated worktree (safe); `:206` is already scoped.
- **The policy fork #0119 abstained on:** a pathspec-scoped commit exits 128 mid-rebase (e.g. during another agent's preflight rebase), converting a today-succeeding board pass into a hard halt of every autonomous skill — availability vs correctness. The answer depends on #0110's architecture choice.

**Third half added 2026-08-09 — the agent-authored commit channel.** The blast-radius defect reaches the shared tree a second way that the #0119 audit did not cover: git commands an agent runs directly from **skill prose**. No skill body instructs staging by pathspec. `docket-convention`'s Step-0 preamble grants direct git plumbing and constrains it not at all; the six metadata-writing skills say only "Commit the change-file edit + spec together in the metadata working tree" or equivalent. The one place the discipline is written down — `docket-build-task`'s "Stage by explicit path — only paths your task changed" — governs *feature-branch* commits, a private worktree where the hazard does not exist.

Observed live during an interactive groom of #0270 (2026-08-09): two concurrent autonomous commits each swept up the session's staged files — the staged rename into `docs(0279): auto-groom to build-ready…`, then all three files into `docket: arm 0195 0265 0272 for auto-groom (wave 5, triage…)`. The groomer's own `git commit` returned "nothing to commit, working tree clean"; content survived, the commit message and its rationale did not. **Neither swallowing commit was a script commit, so the guard in half 2 would have stayed green through both.** The same session then hit half 1's contention defect on a later `preflight` — both halves corroborated live in one run.

## What changes

Architecture settled (2026-08-09 auto-groom, critic-gated; half 3 added 2026-08-09 by human decision; full decision trail in the linked spec's `## Assumptions`): collisions become **survivable, not impossible** — the shared `.docket` worktree stays; per-session worktrees and advisory locks are rejected (heavyweight lifecycle machinery ahead of a workload #0008 defers; a lock cannot span tool calls). Recorded as an ADR at build time.

- **Preflight sync** (`scripts/lib/docket-preflight.sh`): fetch first and skip the rebase entirely when the remote has not moved (a dirty tree with nothing to pull never fails); otherwise a bounded discriminating retry (5 attempts, 2/4/8/8s backoff); never `--autostash`; on exhaustion fail closed with a diagnostic distinguishing dirty-tracked-tree vs wedged (mid-rebase/merge) vs ordinary fetch failure. Untracked-only files never count as dirty.
- **Scope the two `docket-status.sh` commits** (`commit_and_push_generated` and the sweep's refresh-artifacts-links pair) with `--` pathspecs, per the #0083 mark-path idiom. Wedged-tree posture: probe for an in-progress rebase/merge before committing and return a new report token `blocked-wedged-tree` (never overload `push-failed`); `--must-land` treats it as not-landed (halt), best-effort callers log and continue. Update `scripts/docket-status.md`'s vocabulary.
- **Shape-keyed guard** `tests/test_shared_worktree_commit_scope.sh`: default-deny on pathspec-less `git … commit` in `scripts/**`, built to #0119's critic-settled requirements (masked quoted strings, per-segment predicate, explicit driver set including `docket-config.sh`'s local `g` wrapper), keyed exception list with existence floor, mutation-tested.
- **Scope the agent-authored commits (half 3).** State the pathspec rule in `docket-convention`'s Step-0 preamble — at the sentence that grants direct git plumbing — and carry the existing house marker `Stage by explicit path` at each metadata-writing skill's commit step. Both, not either: a standing rule already in context demonstrably loses to a specific instruction at the moment of action (the finding `tests/test_skill_handoff_precedence.sh` was built on). The same guard file gains a two-group coverage check on that guard's model, with in-scope skills **derived** by the invoked command `docket.sh preflight` — the Step-0 preamble, a literal command string rather than a rewordable phrase — verified to yield exactly the seven metadata writers and to exclude the feature-worktree skills. Includes the `tests/test_skill_size_budgets.sh` raises this forces (three skills have ≤14 words of headroom, and change 0201's rule requires arguing in-diff why the prose cannot live in a `references/` file) and a reflow-proof marker match (#0253's `flatten()` when it has merged, else the same shape locally).

## Out of scope

- The sweep's skip-publish marking question — #0118, separate (collides on adjacent `docket-status.sh` lines; see `related:`).
- Parallel backlog drain design (#0008, deferred feat) — this change only has to make today's interactive+autonomous overlap safe; #0008's revival is the trigger to revisit per-session worktrees.
- The push-side CAS loops (already correct) and feature-branch commit paths (not shared).

## Open questions

None — resolved in the linked spec. The core fork (retry vs per-session worktrees) was committed to retry as the conservative default with the reasoning and the reversal path recorded in the spec's `## Assumptions` block for human audit.

## Reconcile log

### 2026-08-11 — reconciled at claim, against `main` @ a97c1542

Eight changes merged between the 2026-08-09 spec and this build. **The design survives intact — all three halves, the architecture decision, and every assumption still hold.** What moved is measurement and one silent-relabelling hazard. Six findings, recorded in full as spec Assumption 16:

1. **A new report token needs an explicit `case` arm, not just a new return value.** `board_pass_inline`'s result `case` ends in a `*)` catch-all printing `board inline changed push-failed`, and `learnings_pass` carries the identical shape. A `blocked-wedged-tree` that only travels out of `commit_and_push_generated` would be *silently relabelled by that catch-all* into the retryable push-failed token — reintroducing one layer up the exact overloading the spec's Assumption 4 forbids. Both sites get an explicit arm ahead of the catch-all. `board_classify`'s own `*)` already maps an unrecognized `board …` line to `failed`, so must-land halts correctly by construction; the token is still named there explicitly, because inheriting correct behaviour from a catch-all is not the same as documenting it.

2. **Reconciled with #0208's `worktree-scope:` (ADR-0083) rather than minting a parallel scope notion.** That is a declared frontmatter fact on `agents/docket-*.md` with two values, `feature` and `metadata`. Half 3's Group 2 predicate is a *coverage* derivation over skill bodies, not a scope notion, and will not be described as one — but it gains a **cross-check floor** from 0208's fact: every agent source declaring `worktree-scope: metadata` whose `skills:` names a docket operating skill must appear in Group 2's derived set. Verified: `docket-adr`, `docket-auto-groom`, `docket-finalize-change`, `docket-implement-next`, `docket-status` — five of the seven; the other two are interactive and wrapper-less by construction.

3. **The `docket.sh preflight` derivation re-verified — still exactly the seven**, plus `docket-convention` excluded as the rule's home. Assumption 12 unchanged.

4. **#0253 has not merged** — no `tests/lib/prose_guard.sh` on `main`. Assumption 14's second branch is live: define `flatten(){ tr -s '[:space:]' ' '; }` locally, byte-identical to the three existing copies, commented with #0253 as the consolidation target.

5. **The spec's skill-headroom measurements were stale and the ranking had inverted.** `docket-auto-groom` is now the tightest at 14 words (the spec said 32), while `docket-implement-next` went from 11 to 30 and `docket-convention` from 14 to 50 — several of the eight merged changes raised these very rows. Spec updated with the 2026-08-11 figures and an explicit instruction that the build re-measures again before setting any row.

6. **Change 0286's caller poll-loop read and ruled not applicable** — recorded so it is not re-litigated. It governs observing a launched child through printed `state=` lines under a minute-denominated budget; Half 1's retry has no child, no report line, and no observation budget. One doctrine does transfer and is adopted: **the unknown arm is terminal, never a retry** — a rebase failure matching none of the named classes fails closed immediately instead of spending budget, which is what the spec's own "spend retries only on classes that can self-heal" already implied.

**Scope unchanged; no work dropped as done-elsewhere; no auto-capture mints** (nothing surfaced clearing the six admission gates — findings 1 and 5 are in-scope build constraints, not independently valuable follow-ups). Build-order note for the file collision with #0118 and #0268, both queued next on `docket-status.sh`: this change's edits there are confined to the two commit sites, the two result `case` statements, and the report vocabulary.
