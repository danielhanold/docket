# Change results artifact — results
Change: #0001 · Branch: `feat/results-artifact` · Plan: `docs/superpowers/plans/2026-06-02-results-artifact.md` · Spec: `docs/superpowers/specs/2026-06-02-results-artifact-design.md` · ADRs: none

First exercise of the convention this change introduces — docket's first results artifact, dogfooded on the change that created the feature.

## Verify (human)

- [ ] Installed skills auto-pick up the edits: `ls -l ~/.claude/skills/docket-*` are symlinks into this repo (per `link-skills.sh`), so the merged convention changes apply with no re-link. If any are plain copies rather than symlinks, re-run `bash link-skills.sh`.
- [ ] Post-merge sanity on `main`: `bash sync-convention.sh --check` prints "convention in sync" and `bash tests/test_results_artifact.sh` is all green.

## Findings

- **Build executed inline** (superpowers:executing-plans style) rather than via subagent-driven-development. The change is one tightly-coupled docs+invariant edit where fragmenting across subagents risked landing prose inside the `<!-- docket:convention:* -->` markers (which the next sync would silently overwrite). The whole-branch code review (Step 6) still ran and passed.
- **`results-template.md` lives with its consumer** (`skills/docket-implement-next/`), not with `change-template.md` (`skills/docket-new-change/`) — the implementer authors results files, the producer authors changes.
- **Line-401 is history, not intent.** The design spec's "results folded into the body" describes the one-time Markhaus migration; reconcile added a go-forward note + decision #15 rather than rewriting it.

## Follow-ups

- **Migrate Markhaus's orphans** (separate, markhaus-side change): move `markhaus/docs/2026-05-31-onboarding-results.md` (and the pre-docket `plan-N-results.md`, `preferences-pane-results.md`, `spike-results.md`) into `docs/results/`, and back-link the docket-era ones via the new `results:` field on their changes. Out of scope here (spec §9); this change only defines the convention.
