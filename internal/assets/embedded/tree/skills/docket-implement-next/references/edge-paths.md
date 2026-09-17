# edge-paths — the implementer's rare edges

The rare edges of `docket-implement-next` — read at the trigger moment named in SKILL.md (a
reconcile-kill, a resume of an `in-progress` change, or Step 7's PR-body assembly). Loaded on
demand; sibling files are not auto-loaded with the skill.

## Reconcile-kill (Step 3, change OBSOLETE)

The convention's terminal close-out reference owns invocations and ordering; this skill's posture is CALLER-side only: trust each exit code, a failure aborts the
kill and is surfaced. The reference's cleanup step prunes any feature worktree/branch already
created. Terminal publication is deferred from Go v1 — the kill archives on `docket` via the `change.kill`
operation and copies nothing onto the integration branch.

## Resume of an `in-progress` change

### Resume context acquisition

An explicitly attributed parent resume enters here before proposed-only queue/allowlist
filtering; a plain id allowlist still skips in-progress work. Handoff continuations
retain the first-act `run.gate-claim` requirement. Read [checked receipt consumption](../../docket-build/references/receipt-semantics.md): `gate-claimed` returns fresh owner authority in top-level `generation`; advance top-level `drive_id`, never claim again or demand predecessor credentials.

Resolve `context.implementation` and call `--id <id> --resume --json`. Require protocol 1,
matching operation, `result: applied`, and the dispatched id with `in-progress` status.
Take the exact version from `context.change.version` and the existing branch from
`context.workflow.feature_branch`, never a reminted name. `readiness: not-proposed`
and `claim_eligible: false` are expected fresh-claim refusals, not resume rejection.

Inspection grants no ownership. Require supported parent attribution and confirmed
prior-run quiescence. If `context.halt.run_halted` is true, invoke
`change.resume-halted --id <id> --version <context.change.version> --acknowledge-quiescent`.
The transaction reprobes branch/workspace/live gate, refreshes the claim and removes
only the halt section. Missing acknowledgement, version drift or a live writer refuses
without mutation. After success, re-read the same resume context for the new version
and metadata revision; refresh workspace binding before scopes or assignments.
If the marker is absent, a parent-authorized replacement resume, keyed retry, or valid
continuation inspects ownership and continues verified checkpoints without replaying
resume-halted. A retry has no continuation id and does not invoke run.gate-claim;
an actual handoff retains that first-act requirement. Absent markers grant no authority.
Refused/malformed/foreign/missing
context halts; never substitute fresh claim, a hand-read version or a hand-deleted marker.

### Gate epoch and checkpoints

One worktree carries at most one live run. The parent's `run.gate-before … --resume <id>`
admits one replacement only after confirmed cancellation:

- `resume-active-run`: use its locator with `run.cancel --key <key> --epoch <id>
  --reason <why>`, or continue the live run through `run.gate-verdict`. Never force a claim.
- `cancellation-pending`: finish the same cancellation before arming again.
- `resume-replacement-reserved`: retain the single reserved replacement key. Repeated
  arms recover that reservation, never authorize parallel dispatch.

Reconcile again if `reconciled: false` or `origin/<integration_branch>` advanced since
the last pass. Preserve verified plan and worker checkpoints:

1. Linked plan, commit and backlink verify → reuse; continue at Step 5, no second planner.
2. No `plan:`, but latest feature commit is a clean single-file plan commit whose
   `Docket-Plan-Path:` trailer and backlink agree → recover and attach under the field-write rule.
3. Ambiguous or inconsistent path/delta/trailer/backlink → halt; never guess or re-plan.

Load existing committed results before new work; a changed date never creates a second file:

1. Linked results and committed artifact/backlink verify → reuse that path.
2. No `results:`, but exactly one safe backlinked candidate at
   `<results_dir>/<YYYY-MM-DD>-<slug>-results.md` → recover and attach it.
3. No `results:` or candidates because the run ended before first creation → create the
   first artifact at the next safe Step-6.5 checkpoint, through author/commit/publish/attach
   before final certification. Record only verified work; absence is not evidence.
4. Missing linked artifact, multiple candidates, failed path/backlink checks or uncertain
   prior creation → halt with the mismatch; never fabricate prior results or guess a path.

Never overwrite newer remote work: re-read authority instead of force-writing. If no
workspace exists or live work/drive ownership prevents a safe checkpoint, retain the
existing halt/continuation and report the capture limitation through its channel.
Do not steal a lease, invent a results path, delay a gate handoff or move HEAD unsafely.

## PR-body assembly (Step 7)

**Best-effort PR→issue reference.** If the change carries an `issue:` value, add a plain `#<issue>` reference to the PR body — but **never `Closes #N`**, so merging the PR never auto-closes the referenced issue. Skip silently when `issue:` is unset — the reference is a one-time courtesy, not a build gate.

**PR-body back-link (change 0136).** When docket authors the PR body, prepend a **back-link line** pointing home to the change on `metadata_branch` — a first body line of the shape `↩ Change <padded-id> — <title>` linking to the change file on `docket` (built with the same blob-or-bare-path logic; skill-side, since the renderer's contract excludes the PR body). Best-effort — never block the PR on it.

**Build-evidence block (change 0170).** Write the current evidence record into the PR body, marker-bounded, alongside the review outcome — the rung that reviewed, and the **findings disposition table** (change 0218): one row per finding, each marked fixed (with its commit SHA), deferred, reverted, or recorded. The table's states are defined in `fix-loop.md`; do not redefine them here. The PR body is the block's durable home: `docket-finalize-change` reads it to decide whether its post-rebase suite run can be skipped. Validate marker order and balance before rewriting an existing block. A results checkpoint that moves HEAD invalidates earlier evidence. Consolidate material results before final certification, then record and verify evidence for exact HEAD before publication. Never publish stale evidence or edit results solely to paste gate timestamps. The deferred results-only skip never permits stale certification.
