# Architecture Decision Records

Immutable, numbered record of *why*. ADRs are never archived or rewritten; once `Accepted`, only the `status:` line changes (on supersession/reversal). This index is generated — do not hand-edit.

## Active

- [ADR-0001](0001-docket-metadata-branch-model.md) — Planning metadata on an orphan docket branch; publish terminal records by copy, not merge (Accepted) ← change #2
- [ADR-0002](0002-docket-mode-default-and-bootstrap.md) — docket-mode is the default; refuse-and-migrate bootstrap; terminal-publish single-sourced in finalize (Accepted) ← change #2 · relates to ADR-0001
- [ADR-0003](0003-convention-reference-loading.md) — The docket convention is reference-loaded from a docket-convention skill, not embedded per skill (Accepted) ← change #5 · relates to ADR-0002
- [ADR-0004](0004-grooming-takes-no-claim.md) — Grooming takes no claim — final-push CAS suffices for human-attended sessions (Accepted) ← change #12 · relates to ADR-0001
- [ADR-0005](0005-close-out-only-harvest.md) — Learnings are harvested only at close-out — one writer, one moment, ledger unpublished (Accepted) ← change #6 · relates to ADR-0001, ADR-0003
- [ADR-0006](0006-autonomous-grooming-bounds.md) — Autonomous grooming bounds — critic gates every build-ready exit; kill/defer never autonomous (Accepted) ← change #14 · relates to ADR-0004
- [ADR-0007](0007-github-board-mirror-boundary.md) — GitHub board mirror — one-way, change-files-authoritative, driven by a deterministic script (Accepted) ← change #11 · relates to ADR-0001
- [ADR-0008](0008-agent-layer-generated-subagents.md) — Agent layer — generated subagent wrappers from layered config (Accepted) ← change #16 · relates to ADR-0001, ADR-0003
- [ADR-0009](0009-auto-groom-critic-isolation.md) — Auto-groom critic isolation — the adversary loads only the convention (Accepted) ← change #17 · relates to ADR-0008
- [ADR-0010](0010-finalize-merge-gate-split-agents.md) — Finalize merge gate — split conflict-resolution from semantic-repair at the rebase-completion boundary (Accepted) ← change #15 · relates to ADR-0008, ADR-0009
- [ADR-0011](0011-finalize-consent-model.md) — Finalize consent model — ambiguity-only prompt + `require_pr_approval` policy gate (Accepted) ← change #21 · relates to ADR-0010
- [ADR-0012](0012-docket-status-script-vs-model-boundary.md) — docket-status script-vs-model boundary for skill passes (Accepted) ← change #23 · relates to ADR-0007
- [ADR-0013](0013-adr-0012-boundary-extends-to-docket-adr-surface.md) — ADR-0012's script-vs-model boundary extends to the docket-adr surface (Accepted) ← change #30 · relates to ADR-0012, ADR-0007, ADR-0002
- [ADR-0014](0014-consuming-repo-script-resolution.md) — Consuming-repo script resolution via `DOCKET_SCRIPTS_DIR` (Accepted) ← change #34 · relates to ADR-0012
- [ADR-0015](0015-harness-portable-agent-config.md) — Harness-portable agent model config — direct model IDs, per-repo generation to an explicit harness list (Accepted) ← change #45 · relates to ADR-0008, ADR-0001
- [ADR-0016](0016-harness-first-agent-config.md) — Harness-first `agents:` config — per-harness model/effort with field-level default fallback (Accepted) ← change #46 · relates to ADR-0015, ADR-0008
- [ADR-0018](0018-pluggable-skills-passthrough-degrade.md) — Pluggable workflow skills — unvalidated skill-name passthrough + degrade-to-auto (not abort) on a missing skill (Accepted) ← change #49 · relates to ADR-0015
- [ADR-0019](0019-global-config-fence-classification.md) — Global config layer — the coordination-key fence classification rule (Accepted) ← change #50 · relates to ADR-0008, ADR-0015, ADR-0016
- [ADR-0020](0020-generated-agent-artifacts-machine-local.md) — Generated agent artifacts are machine-local, never committed; `.docket.local.yml` completes the four-layer config (Accepted) ← change #51 → supersedes ADR-0017 · relates to ADR-0015, ADR-0019
- [ADR-0021](0021-pipeline-script-authored-mechanical-commits.md) — Deterministic pipeline scripts may author formulaic commits and mutate blessed-sequence state (Accepted) ← change #58 · relates to ADR-0012
- [ADR-0022](0022-consultant-authored-brainstorm.md) — Consultant-authored brainstorm — opt-in pinned design agent for the brainstorm role (Accepted) ← change #56 · relates to ADR-0008, ADR-0009, ADR-0018
- [ADR-0023](0023-configurable-sdd-build-model.md) — Configurable SDD build models — a `build:` surface of per-role direct model IDs (Accepted) ← change #44 · relates to ADR-0015, ADR-0016, ADR-0018
- [ADR-0024](0024-claude-context-fork-skill-dispatch.md) — Claude Code uses `context: fork` frontmatter as its inline-skill dispatch mechanism; fork only human-non-interactive skills (Accepted) ← change #61 · relates to ADR-0008, ADR-0017
- [ADR-0025](0025-docket-worktrees-disable-git-hooks.md) — docket bookkeeping commits skip shared git hooks via worktree-scoped core.hooksPath (Accepted) ← change #63 · relates to ADR-0001
- [ADR-0026](0026-fork-dispatch-opacity-two-invocation-paths.md) — Accept fork-dispatch opacity; document two invocation paths; add no tooling (Accepted) ← change #65 · relates to ADR-0008, ADR-0017, ADR-0020, ADR-0024
- [ADR-0027](0027-terminal-publish-repo-scoped-script-gated.md) — terminal_publish — a per-repo coordination key, gated once inside the script (Accepted) ← change #64 · relates to ADR-0012, ADR-0019
- [ADR-0028](0028-report-channel-is-not-a-board-surface.md) — A report channel is not a board surface — the backlog digest is ungated (Accepted) ← change #69 · relates to ADR-0012, ADR-0021
- [ADR-0029](0029-docket-facade-routing-and-config-presentation.md) — docket facade — routing-boundary dispatch and model-ward config presentation (Accepted) ← change #68 · relates to ADR-0012
- [ADR-0030](0030-facade-wiring-guard-discriminates-on-invocation-prefix.md) — The facade-wiring guard discriminates on the invocation prefix, not the bare presence of a `.sh` token (Accepted) ← change #72 · relates to ADR-0029
- [ADR-0031](0031-complementary-board-write-guards-and-the-bound-of-source-scanning.md) — Two complementary board-write guards, and the bound of source-syntax scanning (Accepted) ← change #70
- [ADR-0032](0032-positive-off-state-empty-is-a-wiring-bug.md) — A deliberate off-state is encoded positively — absence and emptiness are reserved for error (Accepted) ← change #71 · relates to ADR-0028, ADR-0030, ADR-0031
- [ADR-0033](0033-cursor-auto-run-trust-at-facade.md) — Cursor auto-run trust is granted at the facade, not per operation (Accepted) ← change #73 · relates to ADR-0029, ADR-0020, ADR-0027
- [ADR-0034](0034-repo-root-anchored-to-main-worktree.md) — docket scripts anchor the repo root to the main worktree, never the caller's CWD (Accepted) ← change #75
- [ADR-0035](0035-cleanup-teardown-fail-closed.md) — docket's feature-branch teardown is fail-closed, never half-destructive (Accepted) ← change #75 · relates to ADR-0034
- [ADR-0036](0036-codex-agents-md-dispatch-block-committed-machine-neutral.md) — Codex AGENTS.md dispatch block is committed and machine-neutral (Accepted) ← change #77 · relates to ADR-0015, ADR-0017, ADR-0020
- [ADR-0037](0037-runner-delegation-explicit-runner-field.md) — Cross-harness runner delegation is switched by an explicit runner field, never model-ID sniffing (Accepted) ← change #79 · relates to ADR-0015, ADR-0012
- [ADR-0038](0038-runner-shim-wrapper-single-dispatch-chokepoint.md) — Runner delegation rides a generated shim wrapper body, not per-skill dispatch branching (Accepted) ← change #79 · relates to ADR-0012, ADR-0015, ADR-0020, ADR-0024, ADR-0037
- [ADR-0039](0039-config-example-mirrors-wrapper-defaults.md) — config.yml.example is a documented mirror of the shipped wrapper defaults (Accepted) ← change #81
- [ADR-0040](0040-terminal-publish-default-opt-in.md) — terminal_publish defaults to false — publishing is opt-in (Accepted) ← change #84 · relates to ADR-0027
- [ADR-0041](0041-learnings-findings-directory-and-promotion-valve.md) — Learnings ledger restructure — findings directory + derived index + human-gated promotion valve (Accepted) ← change #67 · relates to ADR-0005, ADR-0012, ADR-0019, ADR-0028, ADR-0030, ADR-0031, ADR-0032, ADR-0039
- [ADR-0043](0043-retire-bot-auto-approval-zero-approvals-branch-protection.md) — Retire bot auto-approval — branch protection with zero required approvals is the single-maintainer merge path (Accepted) ← change #95 → reverses ADR-0042 · relates to ADR-0011
- [ADR-0044](0044-autonomy-precedence-call-site-pre-specification.md) — Autonomy precedence is enforced by pre-specification at the call site (Accepted) ← change #96 · relates to ADR-0018, ADR-0008, ADR-0024
- [ADR-0045](0045-auto-capture-is-best-effort.md) — Auto-capture is best-effort — a failed stub mint never aborts the change being built (Accepted) ← change #91 · relates to ADR-0012
- [ADR-0046](0046-cas-reset-hard-shared-worktree-tracked-clean-tree-precondition.md) — A compare-and-swap reset --hard in a shared metadata worktree requires a tracked-files-only clean-tree precondition (Accepted) ← change #91 · relates to ADR-0004, ADR-0012
- [ADR-0047](0047-digest-only-read-tier-skips-preflight.md) — docket-status --digest-only is a read tier that deliberately skips docket_preflight (Accepted) ← change #94 · relates to ADR-0012

## Superseded / Reversed

- [ADR-0017](0017-cursor-dispatch-rule-full-agent-set.md) — Per-repo agent generation goes always-full-set, opt-in, with a Cursor dispatch rule (Superseded by ADR-20) ← change #48 · relates to ADR-0015, ADR-0016
- [ADR-0042](0042-auto-approve-consent-model.md) — Auto-approve consent model — a bot approval proves docket's pipeline signed off, not human review (Reversed by ADR-0043) ← change #62 · relates to ADR-0011

## Deprecated

_None._
