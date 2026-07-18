# Backlog

**96 changes** — 🟡 11 proposed · 🔴 1 blocked · ⚪ 1 deferred · 🔵 3 implemented · ✅ 75 done · 🗑️ 5 killed

## 🟡 Proposed (11)

| # | Title | Priority | Readiness |
|---|-------|----------|-----------|
| [0007](active/0007-recurring-change-templates.md) | Recurring change templates — scheduled maintenance work that spawns proposed instances | `medium` | needs-brainstorm |
| [0008](active/0008-parallel-backlog-drain.md) | Parallel backlog drain — fan out concurrent implement-next runs over independent build-ready changes | `medium` | needs-brainstorm |
| [0009](active/0009-human-escalation-loop.md) | Human escalation loop — structured questions-for-you in the change file, answered asynchronously in git | `medium` | needs-brainstorm |
| [0010](active/0010-board-analytics.md) | Board analytics — throughput and cycle-time stats derived from git history, rendered on BOARD.md | `low` | needs-brainstorm |
| [0018](active/0018-yq-yaml-parsing.md) | Evaluate adopting yq for YAML parsing across docket scripts | `low` | needs-brainstorm |
| [0019](active/0019-finalize-ci-gate-functional-test.md) | Finalize ci/both gate — functional test against real GitHub CI (poll/retry) | `low` | needs-brainstorm |
| [0082](active/0082-global-harnesses-per-repo-generation.md) | Global agent_harnesses doesn't reach per-repo generation — silent no-op | `low` | needs-brainstorm |
| [0083](active/0083-terminal-publish-gap-detection.md) | A terminal record can silently never reach the integration branch — investigate #0043's gap and decide on detection | `medium` | auto-groom blocked — needs you |
| [0087](active/0087-headless-finalize-driver.md) | Headless finalize — the finalize-side disposition contract, mirroring 0088 | `high` | build-ready |
| [0091](active/0091-auto-create-discovered-stubs.md) | Auto-create discovered stubs — a config flag that turns mid-run findings into proposed changes | `medium` | build-ready |
| [0096](active/0096-suppress-plan-skill-execution-handoff.md) | An autonomous run can be halted by a sub-skill's interactive hand-off — neutralize writing-plans' execution choice | `high` | needs-brainstorm |

## 🔴 Blocked (1)

| # | Title | Priority | Blocked by |
|---|-------|----------|------------|
| [0044](active/0044-configurable-build-model.md) | Configurable SDD build models for docket-implement-next | `low` | PR #69 is stale (predates the 0068/0072 facade rework and later agent-layer changes) and #0079 (runner delegation) reshapes the design — the build roles should grow a runner field (build.<role>.runner codex, the mixed topology 0079 deferred). Needs a rebase plus redesign pass before merge. |

## ⚪ Deferred (1)

| # | Title | Priority |
|---|-------|----------|
| [0094](active/0094-docket-prime-context-digest.md) | docket-prime — a token-budgeted context digest skills load instead of walking docs/changes | `medium` |

## 🔵 Implemented — awaiting merge (3)

| # | Title | Priority | PR |
|---|-------|----------|----|
| [0078](active/0078-codex-cli-validation-runbook.md) | Codex CLI live-validation runbook — prove docket works end-to-end under Codex | `high` | [#89](https://github.com/danielhanold/docket/pull/89) |
| [0089](active/0089-claim-leases-reclaim-script.md) | Claim leases + reclaim script — expired in-progress claims self-heal back to proposed | `medium` | [#99](https://github.com/danielhanold/docket/pull/99) |
| [0093](active/0093-archive-decay-digest.md) | Archive decay — a rolling one-line digest so board and context cost stay flat as the archive grows | `medium` | [#96](https://github.com/danielhanold/docket/pull/96) |

```mermaid
graph TD
  0007
  0008
  0009
  0010
  0016 --> 0018
  0015 --> 0019
  0044
  0077 --> 0078
  0082
  0083
  0087
  0089
  0090 --> 0091
  0093
  0094
  0096
  0001:::done
  0002:::done
  0003:::done
  0004:::done
  0005:::done
  0006:::done
  0011:::done
  0012:::done
  0013:::done
  0014:::done
  0015:::done
  0016:::done
  0017:::done
  0020:::done
  0021:::done
  0022:::done
  0023:::done
  0024:::done
  0025:::done
  0026:::done
  0027:::done
  0029:::done
  0030:::done
  0031:::done
  0032:::done
  0034:::done
  0035:::done
  0036:::done
  0037:::done
  0038:::done
  0039:::done
  0040:::done
  0041:::done
  0042:::done
  0045:::done
  0046:::done
  0047:::done
  0048:::done
  0049:::done
  0050:::done
  0051:::done
  0052:::done
  0053:::done
  0054:::done
  0055:::done
  0056:::done
  0057:::done
  0058:::done
  0059:::done
  0060:::done
  0061:::done
  0062:::done
  0063:::done
  0064:::done
  0065:::done
  0066:::done
  0067:::done
  0068:::done
  0069:::done
  0070:::done
  0071:::done
  0072:::done
  0073:::done
  0074:::done
  0075:::done
  0077:::done
  0079:::done
  0080:::done
  0081:::done
  0084:::done
  0085:::done
  0088:::done
  0090:::done
  0092:::done
  0095:::done
  classDef done fill:#d3f9d8;
```

<details><summary>✅🗑️ Archive — done + killed (80)</summary>

| # | Title | Merged |
|---|-------|--------|
| [0095](archive/2026-07-18-0095-retire-auto-approve-workflow.md) | Retire the auto-approve workflow — document the classifier and the single-maintainer branch-protection solution | 2026-07-18 |
| [0092](archive/2026-07-18-0092-orphan-detection-script.md) | Orphan detection script — cross-reference change ids in merged commits against archive state | 2026-07-18 |
| [0090](archive/2026-07-18-0090-discovered-from-provenance.md) | discovered-from provenance links — record which change's build surfaced a new stub | 2026-07-18 |
| [0088](archive/2026-07-18-0088-implement-next-loop-continuation.md) | Loop continuation — implement-next chains into the next ready change instead of stopping | 2026-07-18 |
| [0086](archive/2026-07-18-0086-attended-finalize-merge-path.md) | Attended finalize has no merge path under auto_approve — scope the --admin ban to autonomous runs | 2026-07-18 |
| [0085](archive/2026-07-17-0085-skill-slimming-round-two.md) | Second-round skill slimming — re-slim regrown skills + regrowth guard | 2026-07-17 |
| [0062](archive/2026-07-17-0062-autonomous-finalize-merge-authorization.md) | Autonomous finalize merge — clear the auto-mode Merge-Without-Review soft-deny | 2026-07-17 |
| [0084](archive/2026-07-16-0084-terminal-publish-opt-in-default.md) | Flip terminal_publish default to false — publishing to the integration branch becomes opt-in | 2026-07-16 |
| [0081](archive/2026-07-16-0081-first-run-setup-config-example.md) | First-run setup — committed starter config + install.sh scaffolding + README Install restructure | 2026-07-16 |
| [0079](archive/2026-07-16-0079-codex-runner-delegation.md) | Cross-harness runner delegation framework (first runner — OpenAI Codex) | 2026-07-16 |
| [0077](archive/2026-07-16-0077-codex-harness-toml-agents.md) | Codex harness — TOML agent generation + AGENTS.md dispatch block | 2026-07-16 |
| [0067](archive/2026-07-16-0067-learnings-promotion-destination.md) | Give the learnings ledger a promotion destination — it has no way to shrink | 2026-07-16 |
| [0033](archive/2026-07-16-0033-adr-index-main-maintenance.md) | Decide how the ADR index is maintained on the integration branch | 2026-07-16 |
| [0080](archive/2026-07-15-0080-link-skills-create-harness-dir.md) | link-skills.sh creates a missing skills subdir when the harness is present | 2026-07-15 |
| [0075](archive/2026-07-15-0075-cwd-independent-repo-root-anchor.md) | Anchor the repo root to the main worktree — CWD-independent scripts, a fail-closed cleanup guard, and a durable finalize posture | 2026-07-15 |
| [0076](archive/2026-07-14-0076-cwd-independent-repo-root-resolution.md) | Resolve the repo root independently of CWD — preflight run inside `.docket` mints a nested metadata worktree | 2026-07-14 |
| [0074](archive/2026-07-14-0074-bootstrap-facade-verb.md) | A `bootstrap` facade verb — retire the last direct-helper carve-out in Step-0 | 2026-07-14 |
| [0073](archive/2026-07-14-0073-cursor-sandbox-permissions-guide.md) | Cursor sandbox & permissions guide — copyable config, trust tiers, troubleshooting | 2026-07-14 |
| [0072](archive/2026-07-14-0072-facade-skill-rewiring.md) | Rewire the operating skills and Step-0 to the facade — retire the eval preamble | 2026-07-14 |
| [0071](archive/2026-07-14-0071-board-surfaces-unset-vs-empty.md) | Encode the disabled board positively — an empty surfaces value is a wiring bug, not a configuration | 2026-07-14 |
| [0070](archive/2026-07-14-0070-redirect-regex-board-write-guard.md) | Harden the BOARD.md write guard — REDIRECT_RE misses real redirect forms | 2026-07-14 |
| [0068](archive/2026-07-14-0068-docket-command-facade.md) | One executable docket facade — finite subcommands, config read from stdout, never eval'd | 2026-07-14 |
| [0069](archive/2026-07-13-0069-status-report-self-evidencing.md) | docket-status report is self-evidencing and board-independent — stop the board-off BOARD.md hunt | 2026-07-13 |
| [0066](archive/2026-07-13-0066-auto-groom-critic-recheck-foreground.md) | Auto-groom's critic re-check must be foreground — a forked skill that yields returns a half-done run to its caller | 2026-07-13 |
| [0065](archive/2026-07-13-0065-agent-model-pinning-docs.md) | Document the two invocation paths and per-agent model pinning; ADR the context:fork findings | 2026-07-13 |
| [0064](archive/2026-07-13-0064-optional-terminal-publish.md) | Optional terminal-publish — per-repo opt-out to keep metadata on docket | 2026-07-13 |
| [0063](archive/2026-07-11-0063-git-hook-coexistence.md) | Coexist with git-hook frameworks — docket bookkeeping commits skip hooks | 2026-07-11 |
| [0061](archive/2026-07-11-0061-claude-context-fork-dispatch.md) | Claude Code skill-invocation parity — context:fork dispatch to pinned wrappers | 2026-07-11 |
| [0060](archive/2026-07-11-0060-board-render-truncation-guard.md) | board-refresh.sh must not write an empty BOARD.md — non-empty guard on the atomic write | 2026-07-11 |
| [0059](archive/2026-07-11-0059-board-refresh-surface-gate.md) | board-refresh honors board_surfaces — gate BOARD.md regeneration on the resolved surface set | 2026-07-11 |
| [0058](archive/2026-07-11-0058-docket-status-orchestrator.md) | docket-status orchestrator — collapse the status pass into one script call | 2026-07-11 |
| [0057](archive/2026-07-11-0057-docket-owned-gitignore-consolidation.md) | Fold the migration-time .gitignore entries into the managed docket:generated block | 2026-07-11 |
| [0056](archive/2026-07-11-0056-consultant-brainstorm.md) | Consultant-authored brainstorm — opt-in pinned design agent for the brainstorm role | 2026-07-11 |
| [0055](archive/2026-07-11-0055-slim-remaining-skills.md) | Slim docket-implement-next + propagate Step-0 preamble to the small skills | 2026-07-11 |
| [0054](archive/2026-07-11-0054-slim-finalize-change-skill.md) | Slim docket-finalize-change — rewire close-out to the shared reference | 2026-07-11 |
| [0053](archive/2026-07-11-0053-slim-convention-status-skills.md) | Slim docket-convention + docket-status via progressive disclosure | 2026-07-11 |
| [0052](archive/2026-07-10-0052-readme-doc-critic-refresh.md) | README doc-critic refresh — accuracy, structure, newcomer clarity (post-0051) | 2026-07-10 |
| [0051](archive/2026-07-10-0051-global-agents-middle-layer.md) | Machine-local config layer (.docket.local.yml) + all-local agent generation | 2026-07-10 |
| [0050](archive/2026-07-09-0050-global-config-layer.md) | Global config layer — full-schema ~/.config/docket/config.yml with a coordination-key fence | 2026-07-09 |
| [0049](archive/2026-07-09-0049-pluggable-workflow-skills.md) | Pluggable workflow skills — make superpowers invocations configurable, with auto fallback | 2026-07-09 |
| [0048](archive/2026-07-09-0048-cursor-dispatch-rule-generation.md) | Generate Cursor dispatch rules; always write the full agent set per harness | 2026-07-09 |
| [0047](archive/2026-07-08-0047-readme-agent-config-discoverability.md) | Make the agent-config refresh workflow discoverable in the README | 2026-07-08 |
| [0046](archive/2026-07-08-0046-per-harness-agent-models.md) | Per-harness model overrides for docket agents | 2026-07-08 |
| [0045](archive/2026-07-08-0045-multi-harness-agent-generation.md) | Per-repo agent model config reaches Cursor via multi-harness generation | 2026-07-08 |
| [0043](archive/2026-07-08-0043-agent-model-tiers.md) | Model-tier indirection for agent model selection + config-driven advisories | 2026-07-08 |
| [0042](archive/2026-07-08-0042-retune-agent-model-defaults.md) | Re-tune default agent models for the Claude 5 lineup (pin explicit versions) | 2026-07-08 |
| [0024](archive/2026-07-08-0024-retire-board-source-drift-check.md) | Retire or downgrade the inline board/source-drift health check once rendering is deterministic | 2026-07-08 |
| [0041](archive/2026-06-24-0041-post-merge-sync-targets-consuming-repo.md) | Post-merge integration sync fast-forwards the docket clone, not the consuming repo where the merge landed | 2026-06-24 |
| [0040](archive/2026-06-23-0040-terminal-publish-refresh-adr-index.md) | terminal-publish leaves the integration-branch ADR index stale — regenerate it when an ADR is published | 2026-06-23 |
| [0039](archive/2026-06-21-0039-trim-docket-status-archive-prose.md) | Trim docket-status's residual archive-internals prose onto scripts/archive-change.md | 2026-06-21 |
| [0038](archive/2026-06-21-0038-test-grep-stray-dash-warning.md) | Test suite — drop over-escaped dashes in test_docket_metadata_branch.sh grep (silences "stray \ before -") | 2026-06-21 |
| [0037](archive/2026-06-21-0037-skill-fallback-progressive-disclosure.md) | Slim skills — move the per-skill manual-fallback / script-contract prose into on-demand sibling files | 2026-06-21 |
| [0036](archive/2026-06-21-0036-status-sweep-double-archive.md) | docket-status sweep — delegate archiving to archive-change.sh (remove the double-archive) | 2026-06-21 |
| [0035](archive/2026-06-21-0035-artifact-links.md) | Artifact links — a generated link block at the top of every change | 2026-06-21 |
| [0034](archive/2026-06-21-0034-consuming-repo-script-resolution.md) | Helper scripts unreachable in consuming repos — skills call repo-relative `scripts/…` that exists only in the docket source repo | 2026-06-21 |
| [0032](archive/2026-06-20-0032-frontmatter-id-validation.md) | Validate numeric id across the frontmatter script family | 2026-06-20 |
| [0031](archive/2026-06-20-0031-adr-status-check-verb-match.md) | ADR status-consistency check — match the supersede/reverse verb, not just the target id | 2026-06-20 |
| [0030](archive/2026-06-20-0030-script-adr-passes.md) | Script docket-adr's deterministic passes — index render, ledger checks, ADR-only publish | 2026-06-20 |
| [0029](archive/2026-06-20-0029-sync-integration-after-merge.md) | Fast-forward the local integration branch after a docket merge | 2026-06-20 |
| [0028](archive/2026-06-20-0028-wire-closeout-call-sites.md) | Wire the close-out call sites to the extracted scripts | 2026-06-20 |
| [0027](archive/2026-06-20-0027-claude-settings-publish-permission.md) | Auto-grant docket's integration-branch push permission via a per-repo Claude settings rule | 2026-06-20 |
| [0026](archive/2026-06-19-0026-config-resolution-script.md) | Extract config resolution + bootstrap guard into a deterministic script | 2026-06-19 |
| [0025](archive/2026-06-19-0025-closeout-scripts.md) | Extract the shared terminal-transition close-out mechanics into deterministic scripts | 2026-06-19 |
| [0023](archive/2026-06-19-0023-script-sweep-and-health-checks.md) | Decide and apply scripting vs model-driven for the merge sweep and health checks | 2026-06-19 |
| [0022](archive/2026-06-19-0022-render-board-script.md) | Extract inline board rendering into a deterministic script | 2026-06-19 |
| [0021](archive/2026-06-19-0021-finalize-consent-model.md) | Finalize consent model — ambiguity-only prompt + require_pr_approval policy gate | 2026-06-19 |
| [0020](archive/2026-06-17-0020-convention-progressive-disclosure.md) | Split the docket-convention skill via progressive disclosure — extract the GitHub board mirror first | 2026-06-17 |
| [0017](archive/2026-06-17-0017-docket-subagent-composition-wiring.md) | docket subagent composition — nested status/adr/critic dispatch | 2026-06-17 |
| [0015](archive/2026-06-17-0015-finalize-rebase-retest-gate.md) | finalize — rebase onto base + re-run tests before merge | 2026-06-17 |
| [0016](archive/2026-06-16-0016-docket-subagent-model-effort.md) | docket skills as model/effort-pinned subagents — foundation | 2026-06-16 |
| [0011](archive/2026-06-16-0011-github-issues-board-mirror.md) | GitHub board mirror — selectable board surfaces, one-way Issues + Projects mirror | 2026-06-16 |
| [0014](archive/2026-06-12-0014-docket-auto-groom.md) | docket-auto-groom — autonomous grooming drain over auto-groomable stubs | 2026-06-12 |
| [0013](archive/2026-06-12-0013-groom-next-stub-recap.md) | Groom-next recap — introduce the selected stub before the brainstorm starts | 2026-06-12 |
| [0012](archive/2026-06-12-0012-groom-next-skill.md) | Groom-next skill — pick the next needs-brainstorm stub and groom it to build-ready | 2026-06-12 |
| [0006](archive/2026-06-12-0006-learnings-ledger.md) | Learnings ledger — an append-only per-repo memory that builds feed and future builds read | 2026-06-12 |
| [0005](archive/2026-06-10-0005-convention-extraction-skill.md) | Extract the shared convention into a docket-convention skill — reference-loaded, not embedded | 2026-06-10 |
| [0004](archive/2026-06-10-0004-board-refresh-on-status-transition.md) | BOARD.md goes stale during a build — refresh it on status transitions (claim / implemented), not only at Step 0 | 2026-06-10 |
| [0003](archive/2026-06-05-0003-migration-tool-pwd-target.md) | migrate-to-docket.sh targets the invoking repo ($PWD) — usable for consuming repos | 2026-06-05 |
| [0002](archive/2026-06-04-0002-docket-metadata-branch.md) | docket metadata branch — separate planning state from code history | 2026-06-04 |
| [0001](archive/2026-06-02-0001-results-artifact.md) | Change results artifact — linked, optional close-out file | 2026-06-02 |

</details>
