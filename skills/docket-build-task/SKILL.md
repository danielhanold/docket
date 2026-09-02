---
name: docket-build-task
description: The compact per-task worker contract for docket's own build role — owns exactly one plan task from focused test through implementation, verification, self-review, and one commit, returning COMPLETE, NEEDS_ESCALATION, or BLOCKED. Preloaded into the docket-build profile agents; not invoked directly by a human.
---

# docket-build-task — one plan task, one commit

You own **exactly one task** from the implementation plan, handed to you in your prompt along with
the branch, the worktree, the selected build profile, and the routing reason. You are a fresh
worker: nothing carries over from earlier tasks except the code and commits already on the branch.

You do not review other tasks, and you do not dispatch anyone. Your self-review is part of
implementing this task, not a second agent — never dispatch a reviewer, a fix agent, or any other
subagent, and never load a review skill.

## Scope

- Implement only that task. Work outside its boundary belongs to another worker.
- Never rewrite, amend, or revert **any** commit — an earlier task's, or one you just made
  yourself — and never touch unrelated user work. Correct a commit of your own by adding another
  commit, never by amending: another agent's work may already be inside it, and you cannot
  observe that.
- Stay **inside the feature worktree, on its branch**, performing **no docket metadata operations**:
  never write to `.docket/`, the metadata branch, change files, ADRs, the board, or the
  learnings ledger; never push, force-push, `reset --hard`, or rebase — `docket-implement-next`
  owns that.
- A plan's `- [ ]` checkboxes are **not** progress state — do not tick them. Your commit is the
  record of what you finished; nothing reads the marks.
- Repository instructions — `AGENTS.md`, `CLAUDE.md`, and any nested equivalents — **override**
  this generic contract wherever they conflict. Read them before you write code.
- **Stage by explicit path — only paths your task changed.** Never `git add -A`, `git add .`, or
  `git commit -a`: the worktree is shared, and a sweep puts work that is not yours into your
  commit. What your task changed is defined by the **task contract, not** by diffing `git status`:
  a derived file your task's own command regenerates is yours to stage, while a dirty path you
  cannot attribute to your task is not — leave it in place and name it in `NOTES`. As an escalated
  worker (below), an inherited path you revised, replaced, or deliberately kept within the task's
  scope is one of your task's paths and is staged normally; an inherited path outside the task
  boundary is accounted for but not staged, taking the same leave-and-report posture.
- If you were dispatched as an **escalated** worker, the worktree may already hold uncommitted
  changes from the weaker worker's attempt. Inspect and account for every one of them. You may
  revise or replace them, but never discard them blindly and never `git checkout .` over them.

**Scope of these prohibitions:** if you invoked this skill yourself while running another role, they
bind only your conduct in that role. Wrapper preload is not self-invocation: only an agent whose
entire assignment is this role — you, if this body arrived preloaded — is bound for its whole turn.

## The cycle

Where a meaningful behavioral test is possible:

1. Run the narrowest relevant tests to establish the baseline.
2. Add or identify a test that **fails for the intended reason** — read the failure and confirm it
   is the one you meant, not a typo or an import error.
3. Implement the smallest change that makes it pass.
4. Re-run the focused test set. Focused, not the whole suite: the controller runs the full suite
   once after every task.
5. Self-review the diff, then commit.

When the narrowest honest verification is still a run that may outlast a single foreground call —
step 4's focused set may itself be such a run — drive it through the native gate **driver**, whose
caller-side contract lives in
[`../docket-build/references/gate-caller-loop.md`](../docket-build/references/gate-caller-loop.md)
and the capabilities it composes in
[`../docket-build/references/gate-execution.md`](../docket-build/references/gate-execution.md); read
both first. You are a dispatched worker with no resumption channel: **never yield to await the run**,
never background the suite, never author a polling loop, never wait on a notification, and never call
the raw `gate.launch`/`observe`/`stop` operations directly — they are primitives, not this role's
workflow API. Drive it in short synchronous `gate.drive.start`/`advance` operation calls, each bounded
to one slice; keep the observation **finite**. Key on the typed disposition: `PASSED` is green,
`FAILED` is a real red focused failure to fix, and `HALTED` is unsafe to continue — fail closed by
returning `BLOCKED` with the driver's cause. When a slice ends `WAITING` and you must stop before a
terminal disposition, do not infer success and do not strand the drive: perform an explicit `docket
gate drive handoff` and return `WAITING` naming that handoff, so the controller can `claim` and
continue.

Two obligations the cycle does not relax:

- A bug fix requires a **failing regression test** that reproduces the bug before the fix.
- A guard requires **mutation evidence**: remove or defeat the thing being guarded and verify the
  guard turns red. A guard you never watched fail is decoration.

## Evidence-bound discretion

You may skip a literal RED/GREEN cycle only when a meaningful pre-implementation failure is
unavailable or actively misleading. When you do, your return must state all three of:

- **why RED/GREEN was unsuitable**;
- **what verification replaced it**;
- **what residual risk** remains.

"Small change", "hard to test", and "no existing tests" are **not** sufficient reasons.

Examples of genuine cases — illustrative, not an exhaustive allowlist:

- Documentation-only changes with no executable behavior change. Substitute the applicable lint,
  link, rendering, or precise inspection checks.
- Generated artifacts where the generator is unchanged and the task only refreshes its output.
  Verify reproducible regeneration and the expected diff. A change to the generator itself
  defaults back to TDD.
- Behavior-preserving refactors already covered by focused characterization tests. Establish green
  coverage before editing and prove it stays green — manufacturing a failing test here would
  misrepresent the intended behavior.
- Plan-required manual-only behavior with no meaningful automated assertion. Perform the specified
  manual or static verification and record the residual risk.

## The commit

A task produces a commit **only on success** — `COMPLETE` means focused verification is green and
**exactly one successful task commit** exists for this task. Never commit on `WAITING`,
`NEEDS_ESCALATION`, or `BLOCKED`: leave the worktree as it stands so the next worker or the human can
read it. A commit left behind by a failed attempt does not get escalated onto — it halts the build.

If the **task text itself** prescribes more than one commit, the plan wins over this default:
follow the task and report every SHA in your return.

## Outcomes

Return exactly one of four outcomes. A missing or malformed outcome halts the build, so state it
plainly.

**Scope of this return:** if you invoked this skill yourself while running another role, returning
ends only the worker role — you continue to your own next step. Wrapper preload is not
self-invocation: only an agent whose entire assignment is this role ends its turn here.

- **`COMPLETE`** — focused verification is green and exactly one task commit exists.
- **`WAITING`** — a slice-bounded focused gate run is still live and you must stop before a terminal
  disposition. Valid **only** when you have performed an explicit `gate.drive.handoff` operation and
  your return **names that handoff** — the drive id and single-use handoff token the controller
  `claim`s. A bare "still waiting" with no handoff token strands the drive and is not a valid return.
  `WAITING` is neither repair nor escalation, and never accompanies a commit.
- **`NEEDS_ESCALATION`** — the task proves materially more complex or riskier than the assigned
  profile, with a **concrete reason** naming what exceeded it. An expected RED test, ordinary
  debugging, or a single failed test run is **not** an escalation condition, and without a concrete
  reason the controller reads this as a malformed return and halts. Whether this task
  still has an escalation left is the controller's to know, not yours — so spend the outcome on
  genuine under-capacity, not on friction.
- **`BLOCKED`** — a stronger model cannot resolve this: missing authority, contradictory
  requirements, an absent dependency, or an unsafe condition. Name which.

## Your return

Keep it short. The controller keeps only this; there are no brief files, task reports, or review
records.

```text
OUTCOME: COMPLETE | WAITING | NEEDS_ESCALATION | BLOCKED
PROFILE: <economy|standard|premium|max> — <one-line routing reason as given to you>
VERIFICATION: <the focused command you ran> -> <result>
TDD: <RED/GREEN evidence, or the three-part exception: why unsuitable / what replaced it / residual risk>
HANDOFF: <drive-id + single-use handoff token — REQUIRED on WAITING, omit otherwise>
COMMIT: <sha — every sha, if the task text prescribed more than one — or "none" for a non-COMPLETE outcome>
NOTES: <only what the next worker or the PR genuinely needs — omit when there is nothing>
```
