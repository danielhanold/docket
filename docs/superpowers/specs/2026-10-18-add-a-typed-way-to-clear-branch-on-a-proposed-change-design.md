<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0568 — Stop deferring a change that is already being built](../../changes/active/0568-add-a-typed-way-to-clear-branch-on-a-proposed-change.md)**
<!-- docket:backlink:end -->

# Stop deferring a change that is already being built — design

Change #568, groomed 2026-10-18.

## Intent

A change that is being built (`in-progress`) can today be deferred. `domain.Defer` accepts
`proposed` and `in-progress`, and on purpose touches only `status:`. So a deferred
in-progress change keeps `branch:`, `claimed_at:`, `plan:`, and `results:`. When it is
revived (`deferred` → `proposed`), those leftovers stay on a proposed record. That is the
only normal path to a proposed change that carries a `branch:`, and it leads to two dead ends
in `change.groom`:

- `not-restackable` (change 563): you cannot change `stacked_on` while `branch:` is set. No
  typed operation clears `branch:`, and deleting the git branch does not lift the refusal.
- `not-retitleable` (change 0461): you cannot retitle while `branch:`, `plan:`, or `results:`
  is set.

The stub proposed a typed way to clear `branch:`. Tracing showed the cause is upstream. No
docket workflow defers an in-progress change; only a human typing `docket change defer` does.
In about 570 changes it has never happened: all 17 deferrals were of unclaimed changes, and
no active or deferred record carries a `branch:` today. So the fix is to remove the path:
**a change that is being built can only be finished, blocked, or killed — never deferred.**

## Behavior

- `change.defer` on an `in-progress` change refuses and writes nothing. Deferring a
  `proposed` change is unchanged.
- The refusal uses a dedicated finding code (the builder names it; for example
  `not-deferrable`), not the generic `illegal-source-status`, because it needs its own
  wording. Following the house finding-text rule (what happened, whether it matters, the
  remedy, in plain words), its message says:
  - what: change NNNN is being built (in progress), and a change being built cannot be
    deferred;
  - remedy, two choices:
    - to **pause it and keep its branch and work**, block it with a reason (the
      `change.block` operation); `change.unblock` resumes it later;
    - to **drop it**, kill it (the `change.kill` operation). The change is archived and its
      `branch:` cleared, but the git branch and its worktree stay in place; delete them
      yourself if you don't need them. It can be captured again later.
- **Delivery (decided).** The text rides in the refusal finding's message, built in the app
  layer, following the `not-restackable` precedent (`Detail["message"]` on the refusal; the
  domain layer stays text-free). No new impact/remedy pipeline for typed-operation refusals.
- **Terminal output (decided).** `ChangeLifecycleResult.HumanText` today prints only
  `<operation>: <result>` on any non-applied result, so a person typing the command never
  sees the explanation. Change it so a refused lifecycle command also prints each finding's
  message (one per line) under that line. This applies uniformly to all four lifecycle
  commands — block, defer, unblock, revive — so their other refusals get clearer too. JSON
  output is unchanged apart from the new code and message.
- Every other defer refusal (deferred, blocked, implemented, final statuses) keeps today's
  `illegal-source-status`.


### Kill says what it leaves behind (folded in during grooming)

Since the refusal sends people to kill, kill must be honest about what it leaves. Today
`change.kill` clears `branch:` from the record and inspects no branch or worktree, and
finalize cleanup deliberately retains a killed change's branches and worktree
(`killed-retained`: it deletes only provably merged work). So after a kill nothing in docket
names the branch any more. Keeping the branch is correct and stays. The fix is wording only:

- When the record carried a non-empty `branch:` at kill time, the applied kill result carries
  a warning-severity finding (the builder names the code; for example `branch-left-behind`)
  whose message names that exact branch and says its local and remote branch and any feature
  worktree are left in place: look them over, then delete them by hand if you don't need them.
- `ChangeKillResult.HumanText` prints that finding's message under the applied line, so a
  person typing `docket change kill` sees it. A kill of a change with no `branch:` (a never-
  claimed proposed change) prints exactly what it prints today.
- Kill stays probe-free: the warning is decided from the record alone, so its wording says
  "any" worktree / "if they exist" rather than asserting what is on disk.

## Where it changes

- `internal/domain/actions.go`: `Defer` accepts `proposed` only and returns the dedicated
  refusal for `in-progress`. Rewrite its doc comment, which today explains why an
  in-progress deferral keeps branch and claim stamp.
- `internal/app/change_lifecycle.go` / `internal/app/finding_codes.go`: register the new
  finding code the same way the other refusal codes are registered (it must appear wherever
  the closed code list and schema vocabulary require it), attach its message on the refusal,
  and extend `ChangeLifecycleResult.HumanText` to print refusal finding messages.
- Code comments that explain a gate by the removed path — reword them so they no longer
  describe deferral out of in-progress as a live path (the gates' behavior and refusal
  wording stay): the retitle-gate and re-stack-gate comments in
  `internal/app/change_groom.go`, the `revivedReviseFiles` fixture comment in
  `internal/app/change_groom_test.go`, and the helper comment in
  `internal/app/change_lifecycle_test.go` about the claim stamp an in-progress deferral
  leaves behind. These gates now guard hand-edited or legacy records.
- Docs that describe the lifecycle, current behavior only (no change or PR citations):
  - `skills/docket-convention/SKILL.md`: the lifecycle diagram (the `deferred` loop currently
    comes off `in-progress`) and the *Rules* sentence "`deferred` may be entered from
    `proposed` or `in-progress`".
  - Any other current-behavior doc or skill that a whole-repo grep shows stating that
    in-progress changes can be deferred. Archived changes, plans, results, specs, and
    Accepted ADRs are point-in-time records and stay as written.
  - Regenerate embedded assets after skill edits (`go generate ./internal/assets/`).
- `internal/app/change_kill.go`: the branch-left-behind warning finding (registered like
  other finding codes) and `HumanText` printing it.
- Tests: kill of a record with `branch:` returns the warning naming the branch and its
  HumanText shows it; kill of a record without `branch:` has no such finding.
- Tests: domain and app tests for the new refusal (in-progress refused with the new code,
  its message present, nothing written; proposed still defers); a HumanText test showing a
  refused lifecycle command prints the finding message; update the existing tests that treat
  in-progress defer as legal (the source-status tables in
  `internal/app/change_lifecycle_test.go` list `{"in-progress", true}`).

## Non-goals

- No new impact/remedy pipeline for refusals of typed operations (`findingsToStatus` keeps
  clearing `Remedy`).
- No typed operation to clear `branch:`. With the path closed, it has nothing left to fix.
- `not-restackable` and `not-retitleable` stay exactly as they are, wording included, as a
  cheap safety net for a hand-edited or legacy record.
- `change.revive` is unchanged, including for any legacy deferred record that still carries
  `branch:` (none exists today). No migration.
- No change to what `block`, `unblock`, `kill`, `reclaim`, or `complete` do; only the
  lifecycle commands' terminal output on refusal and kill's warning/output change. Kill still
  leaves the git branch and worktree in place, never deletes them, never records the branch
  name in the archived record, and never probes git.
- The killed-stack-parent policy (change 565) is separate.

## Anti-goals

- Refusing without telling the human what to do instead. The refusal must name both block
  and kill and what each does to the branch.
- A new status, field, or gate. This only narrows an existing transition.

## Left to the builder

- The exact finding-code name and wording, within the content above.
- The exact layout of the terminal refusal output.

## Calls made during grooming (not explicitly decided by the human)

- The not-restackable / not-retitleable gates and their wording are left as-is.
- The kill remedy states plainly that the branch and worktree are left behind.
- Stale code comments are reworded rather than left.
