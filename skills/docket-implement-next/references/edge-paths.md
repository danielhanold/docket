# edge-paths — the implementer's rare edges

The rare edges of `docket-implement-next` — read at the trigger moment named in SKILL.md (a
reconcile-kill, a resume of an `in-progress` change, or Step 7's PR-body assembly). Loaded on
demand; sibling files are not auto-loaded with the skill.

## Reconcile-kill (Step 3, change OBSOLETE)

The convention's terminal close-out reference owns invocations, ordering, and the `main`-mode
degradation; this skill's posture is CALLER-side only: trust each exit code, a failure aborts the
kill and is surfaced. The reference's cleanup step prunes any feature worktree/branch already
created; its publish step is `terminal-publish` (a no-op in `main`-mode, or without the
`terminal_publish: true` opt-in).

## Resume of an `in-progress` change

The `reconciled` flag is a **resume-safety guard**: on any resume of an `in-progress` change,
re-run the full reconcile pass if `reconciled` is still `false` (crash, interruption), and also
whenever `origin/<integration_branch>` has advanced since the last pass (idempotent,
non-interactive).

## PR-body assembly (Step 7)

**Best-effort PR→issue reference (when the `github` board surface is enabled).** If the change carries an `issue:`, add a plain `#<issue>` reference to the PR body — but **never `Closes #N`**: the mirror sync stays the sole writer of issue state and close reason. Skip silently when `issue:` is unset — the reference is a one-time courtesy, not a build gate.

**PR-body back-link (change 0136).** When docket authors the PR body, prepend a **back-link line** pointing home to the change on `metadata_branch` — a first body line of the shape `↩ Change <padded-id> — <title>` linking to the change file on `docket` (built with the same blob-or-bare-path logic; skill-side, since the renderer's contract excludes the PR body). Best-effort — never block the PR on it.

**Build-evidence block (change 0170).** Write the current evidence record into the PR body, marker-bounded, alongside the review outcome — the rung that reviewed, and the **findings disposition table** (change 0218): one row per finding, each marked fixed (with its commit SHA), deferred, reverted, or recorded. The table's states are defined in `fix-loop.md`; do not redefine them here. The PR body is the block's durable home: `docket-finalize-change` reads it to decide whether its post-rebase suite run can be skipped. Validate marker order and balance before rewriting an existing block. A step-6.5 results commit — like any post-gate commit — moves branch HEAD after the evidence was minted, so a stale `head_sha` on that path is EXPECTED, not a defect: write the block anyway with that stale SHA. Only **where the ancestor permit is armed** — `docket-finalize-change`'s conditional-skip step, whose second limb is "**ARMED BY CONFIG, never by your judgement**" and off by default — expect finalize to skip its suite run when the post-gate delta is docs-only: `head_sha` a strict ancestor of HEAD and every path changed in `head_sha..HEAD` under the repo's configured `<results_dir>/`. Unarmed, or for any other post-gate commit, expect the suite to run.
