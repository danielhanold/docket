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
- [ADR-0034](0034-repo-root-anchored-to-main-worktree.md) — docket scripts anchor the repo root to the main worktree, never the caller's CWD (Accepted) ← change #75 · relates to ADR-0068
- [ADR-0035](0035-cleanup-teardown-fail-closed.md) — docket's feature-branch teardown is fail-closed, never half-destructive (Accepted) ← change #75 · relates to ADR-0034
- [ADR-0036](0036-codex-agents-md-dispatch-block-committed-machine-neutral.md) — Codex AGENTS.md dispatch block is committed and machine-neutral (Accepted) ← change #77 · relates to ADR-0015, ADR-0017, ADR-0020
- [ADR-0037](0037-runner-delegation-explicit-runner-field.md) — Cross-harness runner delegation is switched by an explicit runner field, never model-ID sniffing (Accepted) ← change #79 · relates to ADR-0015, ADR-0012
- [ADR-0040](0040-terminal-publish-default-opt-in.md) — terminal_publish defaults to false — publishing is opt-in (Accepted) ← change #84 · relates to ADR-0027
- [ADR-0041](0041-learnings-findings-directory-and-promotion-valve.md) — Learnings ledger restructure — findings directory + derived index + human-gated promotion valve (Accepted) ← change #67 · relates to ADR-0005, ADR-0012, ADR-0019, ADR-0028, ADR-0030, ADR-0031, ADR-0032, ADR-0039
- [ADR-0043](0043-retire-bot-auto-approval-zero-approvals-branch-protection.md) — Retire bot auto-approval — branch protection with zero required approvals is the single-maintainer merge path (Accepted) ← change #95 → reverses ADR-0042 · relates to ADR-0011
- [ADR-0044](0044-autonomy-precedence-call-site-pre-specification.md) — Autonomy precedence is enforced by pre-specification at the call site (Accepted) ← change #96 · relates to ADR-0018, ADR-0008, ADR-0024
- [ADR-0045](0045-auto-capture-is-best-effort.md) — Auto-capture is best-effort — a failed stub mint never aborts the change being built (Accepted) ← change #91 · relates to ADR-0012
- [ADR-0046](0046-cas-reset-hard-shared-worktree-tracked-clean-tree-precondition.md) — A compare-and-swap reset --hard in a shared metadata worktree requires a tracked-files-only clean-tree precondition (Accepted) ← change #91 · relates to ADR-0004, ADR-0012
- [ADR-0047](0047-digest-only-read-tier-skips-preflight.md) — docket-status --digest-only is a read tier that deliberately skips docket_preflight (Accepted) ← change #94 · relates to ADR-0012
- [ADR-0049](0049-board-checks-findings-channel-structural-columns-only-validated-values.md) — board-checks.sh findings channel — structural columns carry only script-derived or shape-validated values (Accepted) ← change #104 · relates to ADR-0012
- [ADR-0050](0050-backstop-checks-must-compute-not-reenumerate.md) — A backstop check must compute the invariant it guards, never re-enumerate the causes it backs up (Accepted) ← change #104 · relates to ADR-0049
- [ADR-0051](0051-publish-deferred-marker-not-branch-diff-detector.md) — Make a deferred terminal publish visible with a presence-encoded marker, not a branch-diff detector (Accepted) ← change #83 · relates to ADR-0001
- [ADR-0052](0052-config-key-resolution-boundary.md) — A documented config key resolves through docket-config.sh; a model-read of .docket.yml is not a supported shape (Accepted) ← change #102 · relates to ADR-0048, ADR-0019, ADR-0012
- [ADR-0053](0053-readme-yaml-fences-guarded-by-default-opt-out-marker-grammar.md) — README yaml fences are guarded by default, with an opt-out marker grammar (Accepted) ← change #108 · relates to ADR-0048
- [ADR-0054](0054-cross-reference-anchor-style.md) — Cross-references in maintained source anchor on symbols or quoted clauses, never line numbers — and the guard is deliberately partial (Accepted) ← change #114 · relates to ADR-0031, ADR-0050
- [ADR-0055](0055-exhaustive-vocabulary-mappings-require-array-pinned-set-equality.md) — Exhaustive vocabulary mappings require array-pinned set equality (Accepted) ← change #116 · relates to ADR-0049, ADR-0050
- [ADR-0056](0056-config-manifest-key-scoping-follows-resolver-read-shape.md) — Config-manifest keys are qualified by their ancestor path; the duplicate-name floor is derived from the resolver's read shape (Accepted) ← change #127
- [ADR-0057](0057-frontmatter-read-must-be-anchored-when-key-may-be-absent.md) — A frontmatter read must be anchored when the key may be absent (Accepted) ← change #127
- [ADR-0058](0058-two-tier-frontmatter-scalar-readers-field-vs-field-raw.md) — Two-tier frontmatter scalar readers — field() (logical value) vs field_raw() (raw token) (Accepted) ← change #138
- [ADR-0059](0059-dispatch-capability-resolved-not-inferred-from-tool-name.md) — Dispatch capability is resolved, never inferred from a tool name; unavailability is tiered (Accepted) ← change #137 · relates to ADR-0008, ADR-0017, ADR-0024, ADR-0026
- [ADR-0060](0060-generated-wrapper-conforms-to-target-harness-contract.md) — A generated wrapper conforms to its target harness's own documented contract (Accepted) ← change #135 · relates to ADR-0008, ADR-0015, ADR-0017, ADR-0059
- [ADR-0061](0061-detect-vs-mark-a-missing-terminal-record.md) — Detect a missing terminal record where there is no marker seam; mark where the failure mode is a conscious human deferral (Accepted) ← change #117 · relates to ADR-0049, ADR-0051
- [ADR-0062](0062-in-repo-shell-yaml-readers-no-external-parser.md) — YAML and frontmatter are parsed by in-repo shell readers — no external YAML parser (Accepted) ← change #18 · relates to ADR-0057, ADR-0058
- [ADR-0063](0063-docket-owns-the-build-role-profile-routed-workers.md) — Docket owns the build role — profile-routed workers, model and effort on named agents (Accepted) ← change #167 → supersedes ADR-0023 · relates to ADR-0015, ADR-0016, ADR-0018, ADR-0059
- [ADR-0064](0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md) — Shipped agent model/effort defaults live in a harness-indexed sidecar; wrapper templates carry no model floor (Accepted) ← change #168 → supersedes ADR-0048 · relates to ADR-0015, ADR-0016, ADR-0060, ADR-0063
- [ADR-0065](0065-bare-scalar-validation-needs-an-explicit-quote-leg.md) — A bare-scalar validator needs an explicit quote leg — raw-vs-consumed comparison is a whitespace test, not a bare-scalar test (Accepted) ← change #173 · relates to ADR-0058, ADR-0015
- [ADR-0066](0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md) — Docket owns the review role — read-only rungs, and the suite runs in the build gate (Accepted) ← change #170 · relates to ADR-0012, ADR-0024, ADR-0063
- [ADR-0067](0067-runner-bearing-agent-requires-a-user-configured-model.md) — A runner-bearing agent must carry a user-configured model, runner-wide (Accepted) ← change #205 · relates to ADR-0015, ADR-0037, ADR-0038
- [ADR-0068](0068-delegated-run-anchor-is-an-explicit-argument.md) — A delegated run's anchor is an explicit argument defaulting to the main worktree (Accepted) ← change #206 · relates to ADR-0034
- [ADR-0069](0069-mode-conditioned-clause-discriminates-on-provenance.md) — A mode-conditioned clause in a loadable skill body discriminates on provenance, and the second person belongs to the continue branch (Accepted) ← change #212 · relates to ADR-0024, ADR-0044
- [ADR-0070](0070-fix-loop-profile-envelope-blocker-floor-and-max-ceiling.md) — The fix loop's profile envelope — a blocker floor at standard, a ceiling below max (Accepted) ← change #218 · relates to ADR-0066
- [ADR-0071](0071-writer-guarantees-yaml-validity-by-construction.md) — A writer guarantees YAML validity by construction; a checker's predicate is detection only (Accepted) ← change #235 · relates to ADR-0062, ADR-0065
- [ADR-0072](0072-leg-c-predicate-duplicated-by-value-across-two-scripts.md) — Leg C's predicate is duplicated by value across two scripts, never shared (Accepted) ← change #219
- [ADR-0073](0073-scalar-quote-predicate-has-no-flow-collection-exemption.md) — The needs-quoting predicate answers a scalar-domain question, so it carries no flow-collection exemption (Accepted) ← change #235 · relates to ADR-0065, ADR-0071
- [ADR-0074](0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md) — The build gate's verdict is tri-state — a runner-defined non-failure exit is a halt (Accepted) ← change #224
- [ADR-0075](0075-run-gate-attributes-a-claim-conservatively-and-reports-a-halt-with-its-own-exit-code.md) — The run gate attributes a claim conservatively and reports a halt with its own exit code (Accepted) ← change #237
- [ADR-0076](0076-quote-leg-rule-binds-by-role-not-reader-shape.md) — ADR-0065's quote-leg rule binds by role, not by reader shape (Accepted) ← change #255 · relates to ADR-0065, ADR-0072
- [ADR-0077](0077-orphan-effort-dropped-as-docket-policy-not-vendor-constraint.md) — An effort with no resolved model is dropped as docket policy, not because opencode would reject it (Accepted) ← change #245 · relates to ADR-0015, ADR-0060
- [ADR-0078](0078-parent-facing-gate-surface-for-claude-one-physical-instructions-file.md) — The parent-facing gate surface for Claude Code, and the one-physical-instructions-file symlink policy (Accepted) ← change #242 · relates to ADR-0024
- [ADR-0079](0079-shim-wrapper-frontmatter-pin-governs-the-parent-side-agent.md) — A shim wrapper's frontmatter pin governs the parent-side agent (Accepted) ← change #269 → supersedes ADR-0038 · relates to ADR-0015, ADR-0067

## Superseded / Reversed

- [ADR-0017](0017-cursor-dispatch-rule-full-agent-set.md) — Per-repo agent generation goes always-full-set, opt-in, with a Cursor dispatch rule (Superseded by ADR-20) ← change #48 · relates to ADR-0015, ADR-0016
- [ADR-0023](0023-configurable-sdd-build-model.md) — Configurable SDD build models — a `build:` surface of per-role direct model IDs (Superseded by ADR-63) ← change #44 · relates to ADR-0015, ADR-0016, ADR-0018
- [ADR-0038](0038-runner-shim-wrapper-single-dispatch-chokepoint.md) — Runner delegation rides a generated shim wrapper body, not per-skill dispatch branching (Superseded by ADR-79) ← change #79 · relates to ADR-0012, ADR-0015, ADR-0020, ADR-0024, ADR-0037
- [ADR-0039](0039-config-example-mirrors-wrapper-defaults.md) — config.yml.example is a documented mirror of the shipped wrapper defaults (Superseded by ADR-48) ← change #81
- [ADR-0042](0042-auto-approve-consent-model.md) — Auto-approve consent model — a bot approval proves docket's pipeline signed off, not human review (Reversed by ADR-0043) ← change #62 · relates to ADR-0011
- [ADR-0048](0048-docket-yml-example-invariants.md) — .docket.yml.example is a tested canonical config reference — mirror, fidelity, must-update (Superseded by ADR-64) ← change #101 → supersedes ADR-0039 · relates to ADR-0019

## Deprecated

_None._
