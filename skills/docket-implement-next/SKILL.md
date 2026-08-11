---
name: docket-implement-next
description: Use when you want the next build-ready change in the docket backlog implemented end-to-end to an open PR with no human interaction — picking, claiming, reconciling against current reality, planning, building with TDD, reviewing, and stopping at the human merge gate. The autonomous backlog-drainer; runs solo per change.
context: fork
agent: docket-implement-next
---

# docket-implement-next — the implementer (autonomous)

## Overview

`docket-implement-next` runs with **no human interaction**: it picks the next build-ready change from the docket backlog and drives it all the way to an open PR, then stops at the human merge gate. One invocation handles one change — select, claim, reconcile, plan, build, review, PR, stop.

## When to use

- You want the backlog drained autonomously — pick the highest-priority build-ready change and ship it to a PR without human steering, or hand it an id set (`90,92,94`; a single id is the degenerate case) to scope the run to those changes.
- Do NOT use if you want to interact during brainstorm or design — that is `docket-new-change`'s job. This skill re-brainstorms nothing; the escape hatch for a fundamentally invalidated design is to STOP and hand back to the human.

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (build-ready, metadata working tree, terminal-publish, the `DOCKET`/`LIVE` bootstrap probes, …) without redefinition; no step below is executable without the convention loaded.

## Procedure

### Step 0 — Sync & sweep

Run the convention's **Step-0 preamble** (load the convention; run `docket.sh preflight` as its own Bash call; read the printed `KEY=value` block off stdout; act on the verdict). All bookkeeping in this skill (claim, reconcile, `status`, `pr:`, `plan:`, `adrs:`) lands in the metadata working tree on `metadata_branch`, pushed immediately; only the plan + results + code land on the feature branch.

Then, before selection, **dispatch the `docket-status` subagent** (foreground, at the model/effort its wrapper resolves), whose merge-sweep pass archives any `implemented` change whose PR has merged — the self-cleaning safety net. The dispatch is **unconditional**; its effects are commits on `origin/docket`, surfaced by the preamble's metadata re-sync (already run above) — the contract is **git state, not an in-context return**. If no dispatch mechanism resolves per the convention's *Dispatch-capability resolution* — never from a tool name — the `docket-status` dispatch is **Tier A**: run the same sweep inline, an equivalent path, neither a degradation nor a warning.

### Step 1 — Select

Build-readiness and ranking are defined by the convention's **Build-readiness & selection** section — that definition is the authority here, and this step only describes how to ACQUIRE the set it defines.

**Acquisition.** Run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --digest-only` — a write-free read, and only AFTER Step 0's `docket-status` dispatch and metadata re-sync (the digest is a snapshot; taken pre-sweep it lists already-merged changes). Its final `ready <id> <id> …` line is the build-ready queue already in the convention's deterministic order — final tie-break LOWEST `id`, so concurrent implementers converge on one winner. The `change <id> <status> <readiness> <slug>` lines carry the skip reasons (`needs-brainstorm`, `auto-groom-blocked`, `waiting-on-<N>-unbuilt`, `waiting-on-<N>-needs-merge`).

The digest is an **accelerator, not the sole channel**: the change files stay authoritative. Read the top candidate's change file and CONFIRM build-readiness before claiming — a stale digest then costs a re-pick, never a bad build. If the file disagrees with the digest, drop that candidate, take the next, and REPORT the disagreement. **The exit status governs what a missing or bare `ready` line means.** A **non-zero exit** from `docket-status --digest-only` is a hard error (config export failure, non-`PROCEED` bootstrap verdict, missing metadata worktree, failed render) — surface its stderr diagnostic and STOP with the **`halted`** disposition; never fall back to `active/` for a hard error, and never report `drained` for one. On **exit 0**, a **bare `ready`** line still means the queue is genuinely empty → `drained`; **no `ready` line at all** means an older `render-board` that predates the queue — fall back to walking `active/` yourself, applying the convention's definition and order, and say so in the run report as a degradation to investigate.

**Scope (id allowlist).** With no argument the candidate set is the whole `ready` queue. A caller may pass an **id allowlist** — `docket-implement-next 90,92,94` (a single id `90` is the degenerate case) — and selection is then **restricted to that set**, preserving the queue's order *within* it. The allowlist is a filter, **never a dependency override**: a scoped id that is not currently build-ready+claimable — needs-brainstorm, already `in-progress`, or waiting on an unmerged `depends_on` — is **skipped with its reason** (read it off that id's `change` line — a non-`proposed` status there is skipped for that status — or from the change file if it has none — already archived, or no such id), never force-built, and never aborts the run.

**Empty queue → `drained`.** If no candidate in scope is build-ready+claimable, build nothing and end the run with the **`drained`** disposition (see *Terminal disposition*) — the driver's stop signal.

### Step 2 — Claim (compare-and-swap)

Re-read the manifest after the sync, in the **metadata working tree**; if still `proposed`, set `status: in-progress` + `branch: feat/<slug>` + `updated: <UTC today>` + `claimed_at: <UTC ISO-8601 now>` (`date -u +%Y-%m-%dT%H:%M:%SZ`; add the field if the change predates it) — the claim lease `reclaim-claims` keys on; commit and push on `metadata_branch` (in `docket`-mode, `origin/docket` via `.docket/`). On a non-fast-forward rejection: DISCARD the pending local claim commit (it would conflict on replay), re-sync (re-run `docket.sh preflight`), RE-READ (mandatory); if still `proposed`, re-claim and push — LOOP until the push lands. The arbiter is the re-read (abort if no longer `proposed`), not that any single push succeeds — and that abort is the **`contended`** disposition (see *Terminal disposition*): a lost claim CAS race is a normal, continue-able outcome a driver re-selects past, **never `halted`**. No worktree yet. The claim also **removes any `## Run halted` section** the change carries: the section is presence-encoded state recording that a previous run stopped needing a human, and a fresh claim is the transition back into a live run, so a stale record left in place would tell `verify-run` that an actively-running change had deliberately stopped. Delete the whole section in the same claim commit; git history keeps it.

Then run the Board pass (best-effort — see *Best-effort board refresh*) as a separate commit, so the board reflects the change as `in-progress` rather than build-ready.

> Two agents must NOT share one local clone — each needs its own.

### Step 3 — Reconcile ⭐

In the **metadata working tree** (re-synced to its remote), re-read the change + its spec against `related` + recently-archived changes, cited + recent ADRs, and CURRENT code; refresh the change body and spec to what is true NOW (drop work done elsewhere, adjust scope, fold in new constraints), NON-INTERACTIVELY. The spec lives alongside the change on `metadata_branch` (in `docket`-mode, `.docket/docs/superpowers/specs/…`). A trivial change has no spec — refresh the body only. Append a dated `## Reconcile log` entry; set `reconciled: true`; commit and push on `metadata_branch` (in `docket`-mode, `origin/docket`).

When `AUTO_CAPTURE_ENABLED` is `true` (Step-0 export), adjacent follow-up work this pass surfaces is classified and minted per the convention's *Auto-capture* shared definition instead of only being noted; a discovery whose type falls outside `AUTO_CAPTURE_TYPES` is reported as policy-suppressed and does NOT consume a mint slot.

Two escape hatches:

- Change now **OBSOLETE** → kill it via the convention's terminal close-out (**read `../docket-convention/references/terminal-close-out.md` now — blocking**) with `--outcome killed` and the UTC kill date; its publish step is `terminal-publish`. Caller posture and the cleanup/publish notes: **read `references/edge-paths.md` now (blocking)**. After the kill is archived, run the Board pass (best-effort — see *Best-effort board refresh*) as a separate commit so the board drops the killed change, then loop back to Step 1.
- Design **FUNDAMENTALLY invalidated** (not just scope-adjustable) → STOP and escalate to the human — end the run with the **`halted`** disposition (see *Terminal disposition*), the driver's stop-and-surface signal — and **write that halt into git before stopping**: append a `## Run halted` section — heading **bare**, never dated (`has_section` matches the whole line, so a dated heading is invisible); date it inside the body — to the change file naming what stopped the run and what a human must decide, and **commit and push it on `metadata_branch`** with the rest of the metadata discipline. This is the producer half of the section the convention defines: a `halted` disposition that exists only as a sentence in a completion report is exactly the untrusted self-report `verify-run` was built to stop trusting. The same write is required for **any** hard error that ends the run `halted`, wherever it occurs. Any hard error that prevents reaching a PR is likewise `halted`. This skill cannot re-brainstorm alone; re-brainstorming is a human act handled by `superpowers:brainstorming` + `docket-new-change`.

### Step 4 — Worktree + plan

CONFIRM the step-3 reconcile push has landed on the **metadata branch** before continuing — by **SHA-compare**, not "the push exited 0": after a re-sync, the local metadata tip must equal the remote tip (`docket`-mode: re-run `docket.sh preflight`, assert `git -C .docket rev-parse @ == git rev-parse origin/docket`; `main`-mode: the primary tree's tip equals `origin/<integration_branch>`). If they differ (a concurrent writer rejected the push): re-sync, re-push — loop until the SHAs match, so the build never reads bytes older than origin. Then cut the feature branch — **ALWAYS from `origin/<integration_branch>`**, in both modes — after a direct `git fetch` of `<integration_branch>` for freshness (plain git plumbing on the feature line, NOT the metadata tree `preflight` syncs):

```
git worktree add .worktrees/<slug> -b feat/<slug> origin/<integration_branch>
```

`metadata_branch` only redirects bookkeeping commits — it NEVER determines where code branches start. The reconciled **spec is read from the metadata working tree** (**re-sync `.docket/` immediately before reading it**); alongside it, read the learnings index `<changes_dir>/learnings/README.md`, then the individual finding files whose hook + topics bear on this change. Skip both reads entirely when `learnings.enabled` is `false`. Run the **resolved plan skill** — `$SKILL_PLAN` from the Step-0 config export (default `superpowers:writing-plans`) — **DIRECTED to:** write the plan file and stop there. Any execution-mode or option choice it poses is answered internally from the already-resolved config — step 5 runs `$SKILL_BUILD` — and never surfaced; log one line naming the role and skill if you suppressed one. On `auto` or unavailability, apply the plan auto-fallback per the convention's *Skill layer* — author the plan file yourself, warning prominently. This is an intentional **cross-tree** step — the spec is read from the metadata working tree, the plan is written into `.worktrees/<slug>` (`docs/superpowers/plans/` ON THE FEATURE BRANCH); the feature tree never carries the spec. Record the plan path in `plan:` per the **field-write rule**. The plan **file** merges with the code, so the `plan:` link resolves on the integration branch only after the PR merges (why `docket-status` ignores a missing `plan:` on an `implemented` change). Then stamp the plan's back-link home **on the feature branch** so it rides the PR: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-artifact-backlink --artifact-file .worktrees/<slug>/<plan-path> --change-file .docket/<changes_dir>/active/<id>-<slug>.md` — committed with the plan file on `feat/<slug>` (a feature-worktree write; the renderer is the sole writer of the `docket:backlink` block). Close-out re-renders it durably inside `terminal-publish` under `terminal_publish: true`; otherwise stamp-once.

### Step 5 — Build

The **resolved build skill** — `$SKILL_BUILD` from the Step-0 config export (default `docket-build`) — is invoked **DIRECTED to:** execute the plan task-by-task and stop at the executed plan. **Proceed through the build — the deliverable is the executed plan, never the decision about how to execute it.** Separately, and without ever relaxing the first: answer any choice it poses from resolved config, surface none, and log one line naming the role and skill if you suppressed a hand-off. Emitting that log line discharges the suppression obligation only; the step is not complete until its git-state postcondition holds — see *Step postconditions*. docket-build routes each task to a profile agent and gates on one full-suite run. On `auto` or unavailability, apply the build auto-fallback per the convention's *Skill layer* (execute the plan on the feature branch, warning prominently) — the artifact is the executed plan; method is the agent's choice. If the resolved skill is invocable but **cannot dispatch** (established only per the convention's *Dispatch-capability resolution* — never from a tool name), the build role is **Tier C, authorized-or-halt**: an explicitly configured `auto` authorizes the inline path above; any other resolved value is abort-and-report, leaving the change `in-progress` with `claimed_at` refreshed and the halt reason recorded.

### Step 6 — Review + ADRs

**Validate the build evidence (change 0170).** Read the build-evidence record step 5's gate emitted — it must be present, `result: green`, and its `head_sha` equal to the branch HEAD. Missing, malformed, or stale is a build-contract violation: re-run the full suite once to mint the record yourself rather than reviewing an uncertified branch.

**Select the reviewer rung** deterministically from the build record — take the **highest profile any task routed or escalated to** (an escalation counts as the tier escalated *to*), then map `economy` to `docket-review-lean`, `standard` to `docket-review-standard`, and `premium` or `max` to `docket-review-deep`. When the resolved build skill emits **no build record at all** — as any build role rebound away from `docket-build` may well do, `superpowers:subagent-driven-development` among them — the rung defaults to `docket-review-standard`, matching the uncertainty sink `standard` is in docket-build's own routing. One modifier: a whole-branch diff of more than **1500 changed lines** — insertions + deletions from `git diff --shortstat origin/<integration_branch>...HEAD` — bumps the rung one step, capped at deep — the only selection signal independent of the build's own self-assessment. Log the chosen rung and its reason as one line. Selection is a rule over the build record, never model judgment.

The **resolved review skill** — `$SKILL_REVIEW` from the Step-0 config export (default `docket-review`) — is invoked **DIRECTED to:** review the whole branch against its base and return its findings, then stop, answering any choice it poses from resolved config and never surfacing one — log one line naming the role and skill if you suppressed a hand-off; on `auto` or unavailability, apply the review auto-fallback per the convention's *Skill layer* (a whole-branch review before the PR opens, warning prominently). Dispatch the selected rung wrapper by name, foreground, passing it the branch and base ref, the change's title and scope, the relevant learnings hooks, and the evidence record. Name the **feature worktree** in that dispatch payload: a reviewer reached through a runner delegation receives its worktree through the facade's `--worktree` flag, and a delegated dispatch that names none is refused. Re-read the learnings index `<changes_dir>/learnings/README.md` first and pull the findings relevant to what this change touched (skipped entirely when `learnings.enabled` is `false`). For any non-obvious decision made during implementation, **dispatch the `docket-adr` subagent** (foreground, at the model/effort its wrapper resolves) — once per decision; it assigns the number, updates the index, commits the ADR on `origin/docket`, publishes it onto the integration branch on acceptance if the repo has opted in, and **returns the number**. After re-syncing `.docket/`, append that number to the change's `adrs:` per the **field-write rule**. Review findings that are distinct follow-up work — not this change's own fixes — are likewise classified and minted per *Auto-capture* when `AUTO_CAPTURE_ENABLED` is `true`, carrying the running `--minted` count forward from the reconcile pass; a policy-suppressed candidate is reported and does not increment it. On unavailable dispatch — established only per the convention's *Dispatch-capability resolution*, never from a tool name — the review role is **Tier C** on the same authorized-or-halt terms as step 5; the `docket-adr` dispatch is **Tier A**, running inline instead with its git-state contract unchanged.

**Triage the returned findings, then FIX them in-branch.** Findings are repaired on the open branch before the PR opens — a bounded **fix loop**, not a stub for every one. `REVIEW_MIN_FIX_SEVERITY` (Step-0 export; `minor` by default) is the lowest severity that enters it, and blockers are fixed regardless of it. Routing is by the fix's CHARACTER via the shared rubric, never by its severity, and never reaches the `max` profile; every fix runs the `docket-build-task` contract. Before dispatching the first fix task, **read `references/fix-loop.md` now (blocking)** — it owns the routing table, the non-blocker task cap, the per-finding and batched task shapes, the revert-and-record suite gate (bounded at two runs; still-red halts), and the PR-body disposition table. The reviewer's `unverified-build-state` blocker is the one exception you resolve yourself, by re-running the suite. A finding that is genuinely distinct beyond-the-branch work still takes the auto-capture path above; a finding about this branch's own diff never does. There is **no re-review** round after fixes — remediation is carried by the worker's own self-review, the suite re-run, and the human reading every fix in the PR diff.

### Step 6.5 — Results close-out (optional)

Write a results file ONLY if: **(a)** the human must run interactive/manual checks at the merge gate beyond automated tests, **(b)** the build surfaced findings worth recording (including any that became ADRs), or **(c)** there are follow-ups or notable plan deviations to capture. Otherwise SKIP it — the PR description + green CI are the receipt. When warranted: author `<results_dir>/<YYYY-MM-DD>-<slug>-results.md` from `results-template.md` **IN THE FEATURE WORKTREE** and commit it on `feat/<slug>` with the code — a build artifact, like the plan; the `results:` FIELD is set in the metadata working tree in step 7 (same split as `plan:`). When a results file is written, stamp its back-link home the same way — `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-artifact-backlink --artifact-file .worktrees/<slug>/<results-path> --change-file .docket/<changes_dir>/active/<id>-<slug>.md` — committed with the results file on the feature branch (re-rendered durably at close-out via `terminal-publish` under `terminal_publish: true`).

### Step 7 — PR + stop

Invoke the **resolved finish skill** — `$SKILL_FINISH` from the Step-0 config export (default `superpowers:finishing-a-development-branch`) — DIRECTED to: push the feature branch and open a PR — do NOT merge — then stop. On `auto` or unavailability, apply the finish auto-fallback per the convention's *Skill layer* (push the branch and open the PR, never merging, then stop) and note the degrade in the PR body. Pre-specifying the outcome keeps it non-interactive while reusing its push/PR mechanics — this is the reference shape for the convention's *Skill layer* autonomy-precedence rule.

**PR-body assembly.** The authored PR body carries three docket elements: the best-effort `#<issue>` reference (never `Closes #N`), the **back-link line** (`↩ Change <padded-id> — <title>`, change 0136), and the **build-evidence block** (change 0170) — the current evidence record written marker-bounded (`<!-- docket:build-evidence:start -->` / `<!-- docket:build-evidence:end -->`) alongside the review outcome, the block `docket-finalize-change` reads to decide whether its post-rebase suite run can be skipped. Under dummy mode the body carries an authored plain-language block alongside them — see *Terminal disposition*. Before assembling any of the three, **read `references/edge-paths.md` now (blocking)** — it owns the mechanics, including marker validation and the expected-staleness rule for a post-gate `head_sha`.

Then, BACK IN THE **METADATA WORKING TREE** (in `docket`-mode, `.docket/`), set `status: implemented` + `pr:` (and `results:` if a results file was written in step 6.5) per the **field-write rule** — this is also what lets the sweep read `pr:`. Then run the Board pass (best-effort — see *Best-effort board refresh*) as a separate commit, so the board shows the change as `implemented` — needs your merge.

**STOP.** The change stays in `active/` as `implemented` until a human merges it, or approves `docket-finalize-change` to merge it.

### Step postconditions

Each step below is complete only when its row holds — read from **git**, never from a sub-skill's report or its own narration. One qualification: the build-evidence record of rows 5–6 is an in-context artifact, not a git object, so only its `head_sha` == HEAD conjunct is a git fact — load-bearing, not decorative. The conditions are **cumulative**: each holds in addition to every earlier step's, each read **as of the close of its own step** — a later commit moving branch HEAD (Step 6.5's results file) leaves an earlier row's `head_sha` stale, which `references/edge-paths.md` calls expected, not a defect. These certify a **step**, never the run. **Once a change is claimed, and absent a `halted` disposition or a Step-3 kill, the only postcondition that also completes the run is Step 7's** — a satisfied intermediate row is never licence to stop. A run that ends any other way ends on a **disposition**, not on a postcondition.

| Step | Complete only when |
|---|---|
| 2 Claim | `status: in-progress` + `branch:` + `updated:` + `claimed_at:` committed on `metadata_branch` **and landed** (local tip == remote tip). |
| 3 Reconcile | `reconciled: true` and a dated `## Reconcile log` entry landed on `metadata_branch` — or, on the kill path, the change archived. |
| 4 Worktree + plan | Step 3's push SHA-confirmed **before** the branch is cut; then the plan file **and** its `docket:backlink` stamp committed on `feat/<slug>`, **and** `plan:` landed on `metadata_branch` — a two-tree conjunction, both refs read. |
| 5 Build | the executed plan committed on `feat/<slug>`, with a build-evidence record at `result: green` whose `head_sha` **equals branch HEAD**. |
| 6 Review + ADRs | that record still green at `head_sha` == HEAD **after** any fix commits, and every ADR the run produced landed in `adrs:`. Known-weak row: on a clean review this reduces to Step 5's, because whether a reviewer ran is not a fact about git. |
| 7 PR + stop | the branch pushed (`origin/feat/<slug>` resolves), the PR open, and `status: implemented` + `pr:` landed on `metadata_branch`; `results:` set **iff** a results file and its backlink stamp are committed on `feat/<slug>`. |

Rows 3, 4, 6 and 7 each carry, in addition, the **field-write rule**'s conjuncts on every metadata commit they name: the `claimed_at` re-stamp, and — for any link-bearing field write — the regenerated `## Artifacts` block in the same commit.

Steps 0, 1 and 6.5 get no row: 0 produces nothing scoped to this change, 1 is a pure read, and 6.5 is optional — its artifact rides in Step 7's `iff` conjunct.

### Terminal disposition (driver contract)

Every run ends by declaring exactly **one** of four dispositions, so any driver keys on the outcome instead of parsing prose:

| Disposition | Meaning | Driver action |
|---|---|---|
| `advanced` | Built a change → PR opened (Step 7 reached). | continue |
| `contended` | Selected a change but lost the claim CAS (Step 2); **nothing built**. | continue — re-select next |
| `drained` | No build-ready+claimable change in scope (Step 1's empty queue). | **stop** |
| `halted` | Stopped needing a human — fundamentally-invalidated design (Step 3) or a hard error. Committed on `metadata_branch` as a bare, undated `## Run halted` section — see Step 3 for the write. | **stop + surface** |

The driver's decision is binary: **continue on `advanced`/`contended`, stop on `drained`/`halted`.** The contract is **driver-agnostic** — it names run outcomes, not any one driver's mechanics; `/loop` is *recommended*, not required (see the README drain-pattern doc).

**The obligation is on the agent, not only the driver** — the run does not end until exactly one of
the four is declared. A final report that declares a step-scoped or invented disposition — a build
disposition, a review outcome, "complete" — is by construction an aborted run, whatever else it
reports. `advanced` is claimable only when **Step 7's postcondition** holds — stated in
*Step postconditions* above, not here.

The final report **enumerates** what happened: the change built (if any), each change **skipped with its reason** (needs-brainstorm / already `in-progress` / waiting on an unmerged `depends_on` / outside the id allowlist), any stubs **auto-captured** (plus every dedup skip and any cap overflow), and which disposition ended the run.

**Dummy mode:** when `DUMMY_MODE_ENABLED` is `true` (Step-0 export), write this run's `reports` calibrated to `DUMMY_MODE_PERSONA`, and give its `pr` body, its close-out `results` file, and any `change-sections` it writes (`## Run halted`) an authored `### In plain terms` block alongside the full technical content — the convention's *Dummy mode* shared definition owns the mechanics, each block is written as its own artifact is authored so it rides that artifact's commit and is never retro-added, and the plain block is never a decision input.

### Best-effort board refresh

The Board pass this skill runs after its own status writes (claim, reconcile-kill, `implemented`) is **best-effort**: invoke `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only` — the single Board-pass entry point, which renders, commits, and pushes `BOARD.md` itself — then **log whatever report line it prints and continue**; never abort the build for it. The build's correctness rests on the change-file CAS, not the board; residual staleness self-heals at the next must-land Board pass. The board is always a **separate commit** from the `status:` write (keeping the claim CAS byte-identical across concurrent agents).

## The reconcile pass and the `reconciled` flag

A change is drafted against a *snapshot* of the codebase, ADRs, and other in-flight changes; reconcile is the antidote, run at the **last responsible moment**: after claim (the change is ours) but before planning (nothing is committed to yet).

`reconciled: false` at birth; set to `true` only after the reconcile pass completes and commits. It is **(1) an audit signal** — paired with the dated `## Reconcile log` entry, it proves the change was freshened against current reality; **(2) a resume-safety guard** — on any resume of an `in-progress` change, **read `references/edge-paths.md` now (blocking)** for the resume rules.

`reconciled` is **NOT a selection criterion** — build-readiness is `spec:`-or-`trivial: true` plus satisfied `depends_on`; a change sitting at `reconciled: false` is still build-ready, and reconcile happens in step 3, after selection and claim.

## Branch & metadata discipline

### The field-write rule

Every change-file field write this skill makes (claim's `status:`/`branch:`, reconcile, `status: implemented`, `plan:`, `adrs:`, `pr:`, `results:`) is a **metadata commit in the metadata working tree on `metadata_branch`** — never in the feature worktree — pushed to its remote immediately. EVERY later metadata commit this skill makes — reconcile, `plan:`, `adrs:`, `pr:`, `results:`, `implemented` — also RE-STAMPS `claimed_at: <UTC ISO-8601 now>`. The commits are already happening, so the heartbeat is free, and it puts a stamp at the plan→build seam where a stopped run is otherwise invisible for the whole build span. A write to a **link-bearing field** (`spec:`/`plan:`/`adrs:`/`pr:`/`results:`) additionally regenerates the `## Artifacts` block IN THE SAME COMMIT: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-change-links --change-file .docket/<changes_dir>/active/<id>-<slug>.md --adrs-dir .docket/<adrs_dir>` (the renderer is the sole writer of the block). Scope note: the **claim** (step 2) writes `status:`/`branch:` only — metadata discipline applies, but Artifacts regen does NOT (neither field is link-bearing).

### Feature branch invariants

New change ⇒ `git worktree add .worktrees/<slug> -b feat/<slug> origin/<integration_branch>` — in BOTH modes. The feature branch is cut AFTER claim + reconcile, adds only plan + results + code, and **never modifies** docket metadata (the change file, `BOARD.md`, ADRs) — at merge, the 3-way merge takes the integration branch's side for the change file unconditionally, so there is no conflict and no revert needed. (In `docket`-mode the change file may not even exist on `origin/<integration_branch>`; change 0084.)

Metadata commits (per the **field-write rule**) happen in the metadata working tree; code and plan/results *file* commits happen in the **feature worktree** on `feat/<slug>` — the `plan:`/`results:` *fields* are always written on `metadata_branch`, never the feature worktree. Never cross these streams — a metadata write landing in the feature worktree silently diverges or conflicts at merge. (The single deliberate cross-tree touch is step 4's spec **read** — a read across trees, never a metadata write into the feature tree.)

This skill is safe to invoke from any branch: it `git fetch`es and operates against `origin/<integration_branch>` and `metadata_branch` explicitly; the branch the human happened to be on when they typed the command is irrelevant.
