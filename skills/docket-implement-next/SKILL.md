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

Then, before selection, **dispatch the `docket-status` subagent** (foreground, at the model/effort its wrapper resolves), whose merge-sweep pass archives any `implemented` change whose PR has merged — the self-cleaning safety net for changes not closed via `docket-finalize-change`. The dispatch is **unconditional** and its effects are commits on `origin/docket`; the preamble's metadata re-sync (already run above, before selection) then surfaces the swept state — the contract is **git state, not an in-context return**.

### Step 1 — Select

Among `active/` changes, select per the convention's **Build-readiness & selection** definition: build-ready `proposed` changes only, ranked by its deterministic order — whose final tie-break is LOWEST `id`, so two implementers (if ever run concurrently) converge on the same winner and never claim the same change. Skip `in-progress`, `blocked`, `deferred`, and not-build-ready stubs.

**Scope (id allowlist).** With no argument the candidate set is the whole build-ready backlog (byte-identical to today). A caller may pass an **id allowlist** — `docket-implement-next 90,92,94` (a single id `90` is the degenerate case) — and selection is then **restricted to that set**, with the same deterministic order applied *within* it. The allowlist is a filter, **never a dependency override**: a scoped id that is not currently build-ready+claimable — needs-brainstorm, already `in-progress`, or waiting on an unmerged `depends_on` — is **skipped with its reason**, never force-built, and never aborts the run.

**Empty queue → `drained`.** If no candidate in scope is build-ready+claimable, build nothing and end the run with the **`drained`** disposition (see *Terminal disposition*) — the driver's stop signal.

### Step 2 — Claim (compare-and-swap)

Re-read the manifest after the sync, in the **metadata working tree**; if still `proposed`, set `status: in-progress` + `branch: feat/<slug>` + `updated: <UTC today>`; commit and push on `metadata_branch` (in `docket`-mode, `origin/docket` via `.docket/`). On a non-fast-forward rejection: DISCARD the pending local claim commit (it edits the same `status:`/`branch:` lines and would conflict on replay), re-sync (re-run `docket.sh preflight`), RE-READ (mandatory); if still `proposed`, re-claim and push — LOOP until the push lands (it can be rejected repeatedly under load). The arbiter is the re-read (abort if no longer `proposed`), not that any single push succeeds — and that abort is the **`contended`** disposition (see *Terminal disposition*): a lost claim CAS race is a normal, continue-able outcome a driver re-selects past, **never `halted`**. No worktree yet.

Then run the Board pass (best-effort — see *Best-effort board refresh*) as a separate commit, so the board reflects the change as `in-progress` rather than build-ready.

> Two agents must NOT share one local clone — each needs its own.

### Step 3 — Reconcile ⭐

In the **metadata working tree** (re-synced to its remote), re-read the change + its spec against `related` + recently-archived changes, cited + recent ADRs, and CURRENT code; refresh the change body and spec to what is true NOW (drop work done elsewhere, adjust scope, fold in new constraints), NON-INTERACTIVELY. The spec lives alongside the change on `metadata_branch` (in `docket`-mode, `.docket/docs/superpowers/specs/…`). A trivial change has no spec — refresh the body only. Append a dated `## Reconcile log` entry; set `reconciled: true`; commit and push on `metadata_branch` (in `docket`-mode, `origin/docket`).

Two escape hatches:

- Change now **OBSOLETE** → kill it via the convention's terminal close-out (**read `../docket-convention/references/terminal-close-out.md` now — blocking**) with `--outcome killed` and the UTC kill date — the reference owns invocations, ordering, and the `main`-mode degradation; this skill's posture is CALLER-side only: trust each exit code, a failure aborts the kill and is surfaced. The reference's cleanup step prunes any feature worktree/branch already created; its publish step is `terminal-publish` (a no-op in `main`-mode, or without the `terminal_publish: true` opt-in). After the kill is archived, run the Board pass (best-effort — see *Best-effort board refresh*) as a separate commit so the board drops the killed change, then loop back to Step 1.
- Design **FUNDAMENTALLY invalidated** (not just scope-adjustable) → STOP and escalate to the human — end the run with the **`halted`** disposition (see *Terminal disposition*), the driver's stop-and-surface signal. Any hard error that prevents reaching a PR is likewise `halted`. This skill cannot re-brainstorm alone; re-brainstorming is a human act handled by `superpowers:brainstorming` + `docket-new-change`.

### Step 4 — Worktree + plan

CONFIRM the step-3 reconcile push has landed on the **metadata branch** before continuing — by **SHA-compare**, not "the push exited 0": after a re-sync, the local metadata tip must equal the remote tip (`docket`-mode: re-run `docket.sh preflight`, assert `git -C .docket rev-parse @ == git rev-parse origin/docket`; `main`-mode: the primary tree's tip equals `origin/<integration_branch>`). If they differ (a concurrent writer rejected the push): re-sync, re-push — loop until the SHAs match, so the build never reads bytes older than origin. Then cut the feature branch — **ALWAYS from `origin/<integration_branch>`**, in both modes — after a direct `git fetch` of `<integration_branch>` for freshness (plain git plumbing on the feature line, NOT the metadata tree `preflight` syncs):

```
git worktree add .worktrees/<slug> -b feat/<slug> origin/<integration_branch>
```

`metadata_branch` only redirects bookkeeping commits — it NEVER determines where code branches start. The reconciled **spec is read from the metadata working tree** (**re-sync `.docket/` immediately before reading it**); alongside it, read the learnings index `<changes_dir>/learnings/README.md`, then the individual finding files whose hook + topics bear on this change — past lessons inform the plan. Skip both reads entirely when `learnings.enabled` is `false`. Run the **resolved plan skill** — `$SKILL_PLAN` from the Step-0 config export (default `superpowers:writing-plans`; on `auto` or unavailability, apply the plan auto-fallback per the convention's *Skill layer* — author the plan file yourself, warning prominently). This is an intentional **cross-tree** step — the spec is read from the metadata working tree, the plan is written into `.worktrees/<slug>` (`docs/superpowers/plans/` ON THE FEATURE BRANCH); the feature tree never carries the spec. Record the plan path in `plan:` per the **field-write rule**. The plan **file** merges to the integration branch with the code, so the `plan:` link resolves there only after the PR merges (why `docket-status` ignores a missing `plan:` on an `implemented` change).

### Step 5 — Build

The **resolved build skill** — `$SKILL_BUILD` from the Step-0 config export (default `superpowers:subagent-driven-development`) — executes the plan task-by-task; SDD does TDD + per-task review. On `auto` or unavailability, apply the build auto-fallback per the convention's *Skill layer* (execute the plan on the feature branch, warning prominently) — the artifact is the executed plan; method is the agent's choice.

### Step 6 — Review + ADRs

The **resolved review skill** — `$SKILL_REVIEW` from the Step-0 config export (default `superpowers:requesting-code-review`) — whole-branch; on `auto` or unavailability, apply the review auto-fallback per the convention's *Skill layer* (a whole-branch review before the PR opens, warning prominently). Re-read the learnings index `<changes_dir>/learnings/README.md` first and pull the findings relevant to what this change touched (skipped entirely when `learnings.enabled` is `false`). For any non-obvious decision made during implementation, **dispatch the `docket-adr` subagent** (foreground, at the model/effort its wrapper resolves) — once per decision; it assigns the number, updates the index, commits the ADR on `origin/docket`, publishes it onto the integration branch on acceptance if the repo has opted in, and **returns the number**. After re-syncing `.docket/`, append that number to the change's `adrs:` per the **field-write rule**.

### Step 6.5 — Results close-out (optional)

Write a results file ONLY if: **(a)** the human must run interactive/manual checks at the merge gate beyond automated tests, **(b)** the build surfaced findings worth recording (including any that became ADRs), or **(c)** there are follow-ups or notable plan deviations to capture. Otherwise SKIP it — the PR description + green CI are the receipt. When warranted: author `<results_dir>/<YYYY-MM-DD>-<slug>-results.md` from `results-template.md` **IN THE FEATURE WORKTREE** and commit it on `feat/<slug>` with the code — a build artifact, like the plan; the `results:` FIELD is set in the metadata working tree in step 7 (same split as `plan:`).

### Step 7 — PR + stop

Invoke the **resolved finish skill** — `$SKILL_FINISH` from the Step-0 config export (default `superpowers:finishing-a-development-branch`) — DIRECTED to: push the feature branch and open a PR — do NOT merge — then stop. On `auto` or unavailability, apply the finish auto-fallback per the convention's *Skill layer* (push the branch and open the PR, never merging, then stop) and note the degrade in the PR body. Pre-specifying the outcome keeps it non-interactive while reusing its push/PR mechanics.

**Best-effort PR→issue reference (when the `github` board surface is enabled).** If the change carries an `issue:`, add a plain `#<issue>` reference to the PR body — but **never `Closes #N`**: the mirror sync stays the sole writer of issue state and close reason. Skip silently when `issue:` is unset — the reference is a one-time courtesy, not a build gate.

Then, BACK IN THE **METADATA WORKING TREE** (in `docket`-mode, `.docket/`), set `status: implemented` + `pr:` (and `results:` if a results file was written in step 6.5) per the **field-write rule** — this is also what lets the sweep read `pr:`. Then run the Board pass (best-effort — see *Best-effort board refresh*) as a separate commit, so the board shows the change as `implemented` — needs your merge.

**STOP.** The change stays in `active/` as `implemented` until a human merges it, or approves `docket-finalize-change` to merge it.

### Terminal disposition (driver contract)

Every run ends by declaring exactly **one** of four dispositions, so any driver — a human re-typing the command, the built-in `/loop`, a cron/scheduled agent, or #0008's fan-out — keys on the outcome instead of parsing prose:

| Disposition | Meaning | Driver action |
|---|---|---|
| `advanced` | Built a change → PR opened (Step 7 reached). | continue |
| `contended` | Selected a change but lost the claim CAS (Step 2); **nothing built**. | continue — re-select next |
| `drained` | No build-ready+claimable change in scope (Step 1's empty queue). | **stop** |
| `halted` | Stopped needing a human — fundamentally-invalidated design (Step 3) or a hard error. | **stop + surface** |

The driver's decision is binary: **continue on `advanced`/`contended`, stop on `drained`/`halted`.** The contract is **driver-agnostic** — it names run outcomes, not any one driver's mechanics; `/loop` is *recommended*, not required (see the README drain-pattern doc).

The final report **enumerates** what happened: the change built (if any), each change **skipped with its reason** (needs-brainstorm / already `in-progress` / waiting on an unmerged `depends_on` / outside the id allowlist), and which disposition ended the run.

### Best-effort board refresh

The Board pass this skill runs after its own status writes (claim, reconcile-kill, `implemented`) is **best-effort**: invoke `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only` — the single Board-pass entry point, which renders, commits, and pushes `BOARD.md` itself — then **log whatever report line it prints and continue**; never abort the build for it. The build's correctness rests on the change-file CAS, not the board; residual staleness self-heals at the next must-land Board pass. The board is always a **separate commit** from the `status:` write (keeping the claim CAS byte-identical across concurrent agents).

## The reconcile pass and the `reconciled` flag

A change is drafted against a *snapshot* of the codebase, ADRs, and other in-flight changes; reconcile is the antidote, run at the **last responsible moment**: after claim (the change is ours) but before planning (nothing is committed to yet).

`reconciled: false` at birth; set to `true` only after the reconcile pass completes and commits. It is **(1) an audit signal** — paired with the dated `## Reconcile log` entry, it proves the change was freshened against current reality; **(2) a resume-safety guard** — on any resume of an `in-progress` change, re-run the full pass if `reconciled` is still `false` (crash, interruption), and also whenever `origin/<integration_branch>` has advanced since the last pass (idempotent, non-interactive).

`reconciled` is **NOT a selection criterion** — build-readiness is `spec:`-or-`trivial: true` plus satisfied `depends_on`; a change sitting at `reconciled: false` is still build-ready, and reconcile happens in step 3, after selection and claim.

## Branch & metadata discipline

### The field-write rule

Every change-file field write this skill makes (claim's `status:`/`branch:`, reconcile, `status: implemented`, `plan:`, `adrs:`, `pr:`, `results:`) is a **metadata commit in the metadata working tree on `metadata_branch`** — never in the feature worktree — pushed to its remote immediately. A write to a **link-bearing field** (`spec:`/`plan:`/`adrs:`/`pr:`/`results:`) additionally regenerates the `## Artifacts` block IN THE SAME COMMIT: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-change-links --change-file .docket/<changes_dir>/active/<id>-<slug>.md --adrs-dir .docket/<adrs_dir>` (the renderer is the sole writer of the block). Scope note: the **claim** (step 2) writes `status:`/`branch:` only — metadata discipline applies, but Artifacts regen does NOT (neither field is link-bearing).

### Feature branch invariants

New change ⇒ `git worktree add .worktrees/<slug> -b feat/<slug> origin/<integration_branch>` — in BOTH modes. The feature branch is cut AFTER claim + reconcile, adds only plan + results + code, and **never modifies** docket metadata (the change file, `BOARD.md`, ADRs) — at merge, the 3-way merge takes the integration branch's side for the change file unconditionally, so there is no conflict and no revert needed. (In `docket`-mode the change file does not even exist on `origin/<integration_branch>` unless terminal-publish copied it there under the `terminal_publish: true` opt-in; change 0084.)

Metadata commits (per the **field-write rule**) happen in the metadata working tree; code and plan/results *file* commits happen in the **feature worktree** on `feat/<slug>` — the `plan:`/`results:` *fields* are always written on `metadata_branch`, never the feature worktree. Never cross these streams — a metadata write landing in the feature worktree silently diverges or conflicts at merge. (The single deliberate cross-tree touch is step 4's spec **read** — a read across trees, never a metadata write into the feature tree.)

This skill is safe to invoke from any branch: it `git fetch`es and operates against `origin/<integration_branch>` and `metadata_branch` explicitly; the branch the human happened to be on when they typed the command is irrelevant.
