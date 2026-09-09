# edge-paths — the implementer's rare edges

The rare edges of `docket-implement-next` — read at the trigger moment named in SKILL.md (a
reconcile-kill, a resume of an `in-progress` change, or Step 7's PR-body assembly). Loaded on
demand; sibling files are not auto-loaded with the skill.

## Reconcile-kill (Step 3, change OBSOLETE)

The convention's terminal close-out reference owns invocations and ordering; this skill's posture is CALLER-side only: trust each exit code, a failure aborts the
kill and is surfaced. The reference's cleanup step prunes any feature worktree/branch already
created. Terminal publication is deferred from Go v1 — the kill archives on `docket` via the frozen
`archive-change` leg and copies nothing onto the integration branch.

## Resume of an `in-progress` change

The `reconciled` flag is a **resume-safety guard**: on any resume of an `in-progress` change,
re-run the full reconcile pass if `reconciled` is still `false` (crash, interruption), and also
whenever `origin/<integration_branch>` has advanced since the last pass (idempotent,
non-interactive).

**A change carrying a `## Run halted` marker** — the `run.verify` operation reads it back as the closed
`run-halted` verdict — resumes only through the `change.resume-halted` operation with `--id <id>
--version <entity-version> --acknowledge-quiescent`, never a fresh claim or a hand-deleted section.
The operation requires the exact marked record and the explicit acknowledgement that the prior
worker is quiescent, reprobes the branch/workspace/live gate, refreshes the claim, and removes
exactly the marker section while preserving every other byte and checkpoint. It refuses (writing
nothing) without the acknowledgement, on version drift (`contended`), or on a live gate lock — it
never resets or adopts a workspace whose writer may still be live. Once resumed, the change re-enters
this resume path with its marker gone.

**The plan seam (change 0324).** An attributed caller-side re-dispatch — one naming the id and
`verify-run`'s unmet conjuncts — enters this resume path before ordinary ready-queue and
proposed-only allowlist filtering; a normal invocation that merely names an already-`in-progress`
id still skips it (it may belong to a live concurrent run — the caller gate's
before-set/dispatch attribution is what distinguishes a resume from claim theft). Then:

1. `plan:` already set and its committed artifact + backlink verify → reuse it and continue at
   Step 5; **never dispatch a second planner**.
2. `plan:` empty, but the feature branch's latest commit is a clean, single-file plan commit whose
   `Docket-Plan-Path:` trailer and backlink agree → recover that path, land it under the normal
   field-write rule, and continue at Step 5.
3. The persisted path, commit delta, backlink, and manifest disagree or are ambiguous → halt with
   the exact mismatch. **Never guess a custom plan location and never re-plan** merely because the
   parent stopped after the child returned. The trailer is evidence only — subject it to the same
   git and backlink verification as a live return.

**The results seam (change 0410).** The results artifact is required for every change, so a resume
must not lose it. On resume, **load the committed results before starting new work**, and reuse the
`results:` field whenever it is already set — a changed authoring date **never mints a second
file**; the canonical path is chosen once and reused.

1. `results:` already set and its committed artifact + backlink verify → reuse that path and
   continue; **never author a second results file**.
2. `results:` empty, but the owned feature branch holds **exactly one** safe, correctly
   backlinked results candidate at the canonical `<results_dir>/<YYYY-MM-DD>-<slug>-results.md`
   location (a commit-before-attach interruption) → recover that path and reattach it under the
   normal field-write rule.
3. Zero such candidates, more than one, or any that fails path/backlink verification → **surface
   the ambiguity and halt**; never guess a path and never fabricate a file. **Never overwrite newer
   remote work** — a resume that finds the remote ahead re-reads authority rather than force-writing
   its local view.

**Pre-workspace halt or unsafe-write pause.** When a run halts before the feature workspace exists,
or a checkpoint boundary is reached while the ownership or gate-drive contract forbids a safe write,
the results artifact cannot be captured yet. That case keeps its **existing disposition** (the halt
or the continuation it was already going to take) and **reports the capture limitation through the
existing halt/continuation channel** — no fabricated results path, no stolen lease, and no delayed
gate handoff to make room for a write. The missing capture is a reported limitation, never a reason
to force a workspace, a commit, or a HEAD move.

## PR-body assembly (Step 7)

**Best-effort PR→issue reference.** If the change carries an `issue:` value (historical data — the `github` mirror is retired and mints none), add a plain `#<issue>` reference to the PR body — but **never `Closes #N`**, so merging the PR never auto-closes the referenced issue. Skip silently when `issue:` is unset — the reference is a one-time courtesy, not a build gate.

**PR-body back-link (change 0136).** When docket authors the PR body, prepend a **back-link line** pointing home to the change on `metadata_branch` — a first body line of the shape `↩ Change <padded-id> — <title>` linking to the change file on `docket` (built with the same blob-or-bare-path logic; skill-side, since the renderer's contract excludes the PR body). Best-effort — never block the PR on it.

**Build-evidence block (change 0170).** Write the current evidence record into the PR body, marker-bounded, alongside the review outcome — the rung that reviewed, and the **findings disposition table** (change 0218): one row per finding, each marked fixed (with its commit SHA), deferred, reverted, or recorded. The table's states are defined in `fix-loop.md`; do not redefine them here. The PR body is the block's durable home: `docket-finalize-change` reads it to decide whether its post-rebase suite run can be skipped. Validate marker order and balance before rewriting an existing block. A step-6.5 results commit — now **required** for every change, like any post-gate commit — moves branch HEAD after the evidence was minted, so a stale `head_sha` on that path is EXPECTED, not a defect: write the block anyway with that stale SHA. A `finalize.skip_results_only_delta` **permit exists in config** (repo-fenced, off by default), but it is a **deferred capability that current finalize does not act on**: `docket-finalize-change`'s gate skips its suite run **only** when the rebase was a no-op **and** the PR body carries green build-evidence for the exact current head recorded against the resolved `finalize.test_command`. As that skill states, "**There is no strict-ancestor or results-only skip**." A post-gate results commit moves HEAD past the evidence head, so **expect finalize's suite to run** for it exactly as for any other post-gate commit — never read the deferred permit as an active skip.
