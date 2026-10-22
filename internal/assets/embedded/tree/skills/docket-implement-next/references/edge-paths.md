# edge-paths — the implementer's rare edges

The rare edges of `docket-implement-next` — read at the trigger moment named in SKILL.md (a named
invocation waiting on its own dependency, a reconcile-kill, or a resume of an `in-progress`
change). Loaded on demand; sibling files are not auto-loaded with the skill.

## Named invocation's own merged dependencies (Step 0, bounded closeout)

Triggered when a named (single explicit id) run's `context.implementation` read refuses
`not-ready-waiting-dependency`. Read the named change's own unmet set — run the `status` operation
with `--json` (a write-free read) and take the named change's `unmet_dependencies` ids from its `changes[]` entry, keeping each id whose own `changes[]` entry has `status` `implemented`.

For **each** id in that set — and never any other change — run one `finalize.closeout` operation
with `--id <that id>` (resolve argv from the capability catalog). The operation itself re-proves the
merged PR and applies the one verified final shape, so run no cleanup suffix, and
never widen or recurse beyond this set. Map each envelope locally and key success on the envelope `result` `applied` or `no-op` — never on its `disposition` token:

- Every closeout succeeds → re-run the `repository.prepare` operation and re-run
  `context.implementation --id` once; if that single re-read still refuses (an applied closeout can
  leave the dependency not `done`), report that as the ordinary local refusal.
- A closeout refused with reason `pr-not-merged` → the dependency is genuinely unmerged; surface the
  ordinary waiting-dependency refusal naming that id (remedy: merge its PR, then
  `docket-finalize-change` or the explicit `finalize.closeout` operation).
- Any other outcome → a failure of the named change's **own** dependency; halt before claiming
  through the pre-claim run-reporting path, naming that dependency id and the same remedy.

An unrelated change's closeout is never attempted here, and a dependency unmet for any
non-`implemented` reason keeps its ordinary local refusal unchanged. A `not-ready-stack-base-unresolved` refusal is never a closeout trigger and keeps its ordinary local
refusal: the bounded closeout covers only the named change's own `depends_on` set, and an
unresolved stack base clears through the parent's own finalize or the next sweep.

## Reconcile-kill (Step 3, change OBSOLETE)

The convention's close-out reference owns invocations and ordering; this skill's posture is CALLER-side only: trust each exit code, a failure aborts the
kill and is surfaced. The cleanup step retains a killed change's worktree/branch (`killed-retained`, a `no-op`).
The kill archives on `docket` via the `change.kill` operation and copies nothing onto the
integration branch.

## Resume of an `in-progress` change

The `reconciled` flag is a **resume-safety guard**: on any resume of an `in-progress` change,
re-run the full reconcile pass if `reconciled` is still `false` (crash, interruption), and also
whenever `origin/<integration_branch>` has advanced since the last pass (idempotent,
non-interactive).

**A change carrying a `## Run halted` marker** — the `run.verify` operation reads it back as the closed
`run-halted` verdict — resumes only through the `change.resume-halted` operation with `--id <id>
--revision <revision> --acknowledge-quiescent`, never a fresh claim or a hand-deleted section.
The operation requires the exact marked record and the explicit acknowledgement that the prior
worker is quiescent, reprobes the branch/workspace/live gate, and removes
exactly the marker section while preserving every other byte and checkpoint. It refuses (writing
nothing) without the acknowledgement, on revision drift (`contended`), or on a live gate lock — it
never resets or adopts a workspace whose writer may still be live. Once resumed, the change re-enters
this resume path with its marker gone.

**Starting a resume over a live run.** A stranded run recovers only by resuming its change by id
(the `run.start` operation with `implement-next --resume <id>`); one worktree carries at most one
live run, so `run.start` refuses to open a second run over one that has not verifiably stopped:

- Prior run still **active** (an undispatched earlier resume start counts) → refused
  `resume-active-run`, naming the change and run key, with the remedy: cancel the prior run via the
  `run.cancel` operation (`--key <key> --reason <why>`) and resume after
  confirmed cancellation, or continue the live run via `run.verdict`. **Never** force a fresh
  claim over a possibly-live run — that is claim theft.
- Cancellation still finishing → refused `cancellation-pending`; the resume observes that cleanup
  only. Finish the cancel first, then resume.
- Prior run confirmed-cancelled and superseded → `run.start` reserves **exactly one** replacement
  dispatch and returns that reserved key; a repeat start (or one recovering a lost response) returns
  the same reservation (`resume-replacement-reserved`) rather than minting a second run. Resume thus
  admits one replacement, only after cancellation is confirmed.

**The plan seam.** An attributed caller-side re-dispatch — one naming the id and
the `run.verify` operation's unmet conditions — enters this resume path before ordinary ready-queue and
proposed-only allowlist filtering; a normal invocation that merely names an already-`in-progress`
id still skips it (it may belong to a live concurrent run — the run tracker's
before-set/dispatch attribution is what distinguishes a resume from claim theft). Then:

1. `plan:` set → planning is complete (the attach wrote file and field together); continue at
   Step 5; **never dispatch a second planner**.
2. `plan:` unset → re-dispatch the plan-writer.

Before building continues, re-run `workspace.commit-spec`: after a mid-flight spec revision it
refreshes the copy; otherwise it is a no-op.

**The results seam.** The results artifact is required for every change, so a resume
must not lose it. On resume, **load the attached results from the metadata worktree
(`metadata_worktree_path` in the `repository.prepare` context) at
`<metadata_worktree_path>/<results path>` before starting new work**, and
reuse the `results:` path whenever it is set — a changed authoring date **never mints a second file**; the canonical path is chosen once and reused.
**Never overwrite newer remote work** — a resume that finds the remote ahead re-reads authority
rather than force-writing its local view.

**Pre-workspace halt or unsafe-write pause.** When a run halts before the feature workspace exists,
or a checkpoint boundary is reached while the ownership or gate-drive contract forbids a safe write,
the results artifact cannot be captured yet. That case keeps its **existing disposition** (the halt
or the continuation it was already going to take) and **reports the capture limitation through the
existing halt/continuation channel** — no fabricated results path, no stolen claim, and no delayed
gate handoff to make room for a write. The missing capture is a reported limitation, never a reason
to force a workspace, a commit, or a HEAD move.
