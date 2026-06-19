# Close-out scripts — results
Change: #25 · Branch: feat/closeout-scripts · PR: (opened at Step 7) · Plan: docs/superpowers/plans/2026-06-19-closeout-scripts.md · ADRs: 1, 2, 7

## Verify (human)

No interactive check is required beyond the merge gate's automated suite — the three
scripts are covered hermetically (`tests/test_closeout.sh`, 53 asserts, real bare-origin
git, no `gh`/network) and all 16 docket suites are green. Two soft watch items:

- [ ] **First live exercise.** These scripts go live for close-outs that happen *after*
  this change merges (the skills are re-linked from the merged source). The first real
  close-out that runs `archive-change.sh` → `terminal-publish.sh` → `cleanup-feature-branch.sh`
  against the real backlog is worth eyeballing once (it was deliberately not smoke-tested
  against live data — the test seam is hermetic, per spec §6). Compare its archived file +
  integration-branch copy-set against what the old prose would have produced.

## Findings

- **ADR-0002 `## Update` (rides this merge).** Appended a dated `## Update — 2026-06-19
  (change 0025)` to ADR-0002: Decision 3 ("terminal-publish single-sourced in finalize")
  still stands — finalize remains the documented **owner**; the *mechanics* now live in
  the three scripts, the same owner-keeps-*when* / script-owns-*how* split as
  `docket-status` ↔ `render-board.sh` (ADR-0007). Committed on `origin/docket`; **not**
  standalone-republished — ADR-0002 is in this change's `adrs:` so its terminal-publish
  re-copies the updated file onto `main` atomically on merge (avoids the premature
  direct-to-`main` push that would dangle the script references). No new ADR was needed
  (faithful extraction, as 0022 needed none).
- **macOS symlink path (reusable).** `archive-change.sh` needs `pwd -P` to resolve
  `mktemp`'s `/var/…` vs git's `/private/var/…` before stripping the worktree prefix;
  without it the `active/<pad>-*` glob matched nothing. Same `pwd -P` discipline used in
  `cleanup-feature-branch.sh`'s provenance guard.
- **SIGPIPE footgun caught (LEARNINGS #11/#16).** `terminal-publish.sh` first used
  `printf … | grep -m1` to pick one path from `ls-tree` output — the project's recurring
  early-pipe-close shape. Replaced with capture-then-first-line (`…grep -E…` then
  `${x%%$'\n'*}`). The surviving `printf '%s\n' "$var" | grep -q` postcondition checks are
  the project's sanctioned safe form (static captured var, not a live producer).
- **`set_field` frontmatter anchor (hardening beyond strict extraction — recorded per
  LEARNINGS #21).** `archive-change.sh`'s in-place frontmatter sed was unanchored, so it
  would have rewritten *any* column-0 `status:`/`updated:`/`results:` line — including
  body prose, a real risk for docket's own change files (which discuss these fields). I
  anchored the substitution to the first `---…---` block. This is *more* faithful to the
  original hand-procedure's intent (edit the field, not the body), changes no behavior for
  valid inputs, and is locked by a new test (a body `status:` line survives verbatim while
  the frontmatter is set). Flagged here rather than silently shipped.
- **Stale sentinel re-anchored (plan-scope deviation).** Rewiring the main-mode kill prose
  to script invocations made `tests/test_docket_metadata_branch.sh` K3/K4 (which grepped
  the old literal `"): do the archive move"`) go red. The invariant (main-mode degrades to
  an in-place archive move, terminal-publish skipped) is fully preserved, so I re-anchored
  both asserts to `"the integration branch), performing the archive move"` (mutation-
  confirmed: each flips when the main-mode clause is deleted). This edited one file outside
  the plan's stated file set — a necessary regression fix, not a scope expansion.
- **Test rigor caught two of its own gaps.** The provenance-guard test initially didn't
  drive the guard (worktree at the wrong path → an unrelated postcondition produced the
  non-zero exit); corrected so the guard genuinely fires, mutation-confirmed. A brittle
  "exactly-2-pathnames" change-file-only assert was made robust to git rename-detection.
- **Hygiene:** `.superpowers/` (the subagent-driven-development scratch dir) was not
  gitignored; two scratch reports leaked into intermediate commits. Added `.superpowers/`
  to `.gitignore` and untracked them — the branch's net diff vs `main` carries no scratch.
- **CAS conflict else-branch now covered (was a deferred follow-up; done in this PR).**
  `terminal-publish.sh`'s most intricate path — `pull --rebase` conflicts on a copy-set
  path → re-checkout `origin/docket`'s authoritative bytes → `rebase --continue` — is now
  exercised by a `publish(conflict)` test whose competing writer DIVERGES the archived
  change file (the existing competing-push test only hit the clean-rebase if-branch via
  `README`). Mutation-confirmed: neutralizing the re-checkout flips 3 of its 4 asserts.
  Scripting-to-test-the-intricate-git-dance is this change's whole thesis (spec §3), so
  leaving it uncovered was the wrong call — closed here.
- **Tree-identity precondition documented (was a deferred follow-up; done in this PR).**
  `archive-change.sh`'s header now states callers MUST derive `--date`/`--results`
  deterministically from the manifest, so two concurrent archivers stage a tree-identical
  change-file-only commit — making the invariant defensive rather than incidental.

## Follow-ups

- **Unblocks 0023.** Its deferred §5b "script the merge sweep" piece can now route the
  sweep's archive through these shared scripts instead of a divergent copy.
