# Backlog

**236 changes** — 🟢 4 in progress · 🟡 54 proposed · ⚪ 4 deferred · ✅ 146 done · 🗑️ 28 killed

## 🟢 In progress (4)

| # | Title | Priority | Type | Spec | Branch |
|---|-------|----------|------|------|--------|
| [0190](active/0190-close-the-build-evidence-value-gap-a-post-gate-results-commi.md) | Close the build-evidence value gap: a post-gate results commit always defeats finalize's suite skip | `medium` | `feat` | [spec](../superpowers/specs/2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi-design.md) | `feat/close-the-build-evidence-value-gap-a-post-gate-results-commi` |
| [0219](active/0219-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d.md) | aborted-run's Step 7 seam — a fourth git-only leg, plus GitHub enrichment for leg C | `high` | `fix` | [spec](../superpowers/specs/2026-08-07-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d-design.md) | `feat/aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d` |
| [0231](active/0231-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla.md) | A presumed-dead build worker can wake and race its own replacement in one worktree | `medium` | `fix` | [spec](../superpowers/specs/2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-design.md) | `feat/a-presumed-dead-build-worker-can-wake-and-race-its-own-repla` |
| [0235](active/0235-writers-emit-unquoted-yaml-title-scalars-so-six-change-files.md) | Writers emit unquoted YAML title scalars, so six change files fail to parse | `medium` | `fix` | [spec](../superpowers/specs/2026-08-07-writers-emit-unquoted-yaml-title-scalars-so-six-change-files-design.md) | `feat/writers-emit-unquoted-yaml-title-scalars-so-six-change-files` |

## 🟡 Proposed (54)

| # | Title | Priority | Type | Readiness |
|---|-------|----------|------|-----------|
| [0019](active/0019-finalize-ci-gate-functional-test.md) | Finalize ci/both gate — functional test against real GitHub CI (poll/retry) | `low` | `chore` | needs-brainstorm |
| [0082](active/0082-global-harnesses-per-repo-generation.md) | Global agent_harnesses doesn't reach per-repo generation — silent no-op | `low` | `fix` | needs-brainstorm |
| [0100](active/0100-force-push-lease-classifier-denial.md) | Force-push-with-lease denied by the auto-mode classifier — unblock finalize's merge gate | `medium` | `fix` | needs-brainstorm |
| [0103](active/0103-wire-the-github-project-config-read-documented-but-unwired-k.md) | Wire the github_project config read (documented-but-unwired key) | `low` | `fix` | needs-brainstorm |
| [0110](active/0110-shared-metadata-worktree-contention.md) | Concurrent agents collide on the shared .docket worktree's dirty-tree window | `high` | `fix` | needs-brainstorm |
| [0118](active/0118-decide-whether-the-sweep-s-skip-publish-path-should-also-mar.md) | Decide whether the sweep's skip-publish path should also mark an unpublished terminal record | `medium` | `fix` | needs-brainstorm |
| [0119](active/0119-scope-the-metadata-worktree-git-commit-calls-to-the-paths-th.md) | Scope the metadata-worktree git commit calls to the paths they own | `medium` | `fix` | auto-groom blocked — needs you |
| [0121](active/0121-the-manifest-s-elsewhere-check-proves-a-word-occurrence-not.md) | The manifest's elsewhere: check proves a word occurrence, not a real config read | `medium` | `fix` | needs-brainstorm |
| [0123](active/0123-machine-check-the-docket-config-md-export-list-order-against.md) | Machine-check the docket-config.md export list order against the resolver | `medium` | `chore` | needs-brainstorm |
| [0125](active/0125-decide-whether-the-rung-pair-completeness-claim-should-be-me.md) | Decide whether the rung-pair completeness claim should be mechanically enforced | `medium` | `chore` | needs-brainstorm |
| [0134](active/0134-audit-field-call-sites-for-frontmatter-anchored-reads.md) | Audit field() call sites for frontmatter-anchored reads | `medium` | `fix` | needs-brainstorm |
| [0139](active/0139-extend-the-tiered-dispatch-unavailability-posture-to-finaliz.md) | Extend the tiered dispatch-unavailability posture to finalize's two in-context-gating dispatches | `medium` | `fix` | needs-brainstorm |
| [0140](active/0140-normalize-the-inherit-model-sentinel-once-for-every-runner-a.md) | Normalize the inherit model sentinel once for every runner adapter | `medium` | `fix` | needs-brainstorm |
| [0141](active/0141-factor-the-shared-wrapper-source-parse-out-of-the-named-harn.md) | Factor the shared wrapper-source parse out of the named harness emitters | `medium` | `refactor` | needs-brainstorm |
| [0142](active/0142-make-the-unmapped-harness-wrapper-gap-loud-at-generation-tim.md) | Make the unmapped-harness wrapper gap loud at generation time | `medium` | `fix` | needs-brainstorm |
| [0147](active/0147-extend-the-2c-orphan-key-check-past-its-column-0-anchor-to-n.md) | Extend the (2c) orphan-key check past its column-0 anchor to nested keys | `medium` | `fix` | auto-groom blocked — needs you |
| [0150](active/0150-pin-or-report-the-resolved-shell-toolchain-across-the-test-s.md) | Pin or report the resolved shell toolchain across the test suite | `medium` | `chore` | auto-groom blocked — needs you |
| [0151](active/0151-vacuous-docket-bash-path-asserts-sit-in-eval-free-blocks-out.md) | Vacuous DOCKET_BASH_PATH asserts sit in eval-free blocks, out of the poison-prelude guard's reach | `medium` | `fix` | auto-groom blocked — needs you |
| [0154](active/0154-audit-skill-bodies-for-the-stale-restatement-class-change-01.md) | Audit skill bodies for the stale-restatement class change 0145 closed in one file | `medium` | `docs` | needs-brainstorm |
| [0155](active/0155-interior-tabs-in-a-frontmatter-value-shift-the-render-board.md) | Interior TABs in a frontmatter value shift the render-board sort feeder's fields | `medium` | `fix` | needs-brainstorm |
| [0156](active/0156-render-board-sh-exits-0-on-malformed-input-and-commits-a-cor.md) | render-board.sh exits 0 on malformed input and commits a corrupt board | `medium` | `fix` | needs-brainstorm |
| [0158](active/0158-batch-mode-for-docket-implement-next-build-several-coupled-c.md) | Batch mode for docket-implement-next — build several coupled changes on one branch | `medium` | `feat` | needs-brainstorm |
| [0159](active/0159-docket-status-skill-md-s-normal-outcomes-list-omits-the-heal.md) | docket-status SKILL.md's normal-outcomes list omits the 'health checks failed <exit>' line | `medium` | `docs` | needs-brainstorm |
| [0160](active/0160-a-committed-too-deep-runtime-bash-lost-its-machine-local-ign.md) | A committed too-deep runtime.bash lost its machine-local ignored advisory | `medium` | `fix` | needs-brainstorm |
| [0163](active/0163-six-phase-tier-3-guard-counts-headings-instead-of-asserting.md) | Six-phase Tier 3 guard counts headings instead of asserting the phase set | `medium` | `fix` | needs-brainstorm |
| [0165](active/0165-consolidate-or-document-the-duplicated-flat-scalar-docket-ym.md) | Consolidate or document the duplicated flat-scalar .docket.yml reader in migrate-to-docket.sh | `medium` | `refactor` | needs-brainstorm |
| [0166](active/0166-retune-the-interactive-skills-advisory-session-model-recomme.md) | Retune the interactive skills' advisory session-model recommendation | `medium` | `chore` | needs-brainstorm |
| [0171](active/0171-settle-a-reflow-tolerant-house-pattern-for-prose-anchored-gu.md) | Settle a reflow-tolerant house pattern for prose-anchored guards | `medium` | `refactor` | needs-brainstorm |
| [0172](active/0172-normalize-the-banned-producer-pipe-shape-across-tests-and-he.md) | Normalize the banned producer-pipe shape across tests and helpers | `medium` | `chore` | needs-brainstorm |
| [0177](active/0177-harden-the-0174-fixture-template-helpers-sticky-failure-ungu.md) | Harden the 0174 fixture-template helpers (sticky failure, unguarded mktemp, destructive pre-clean, leaked root) | `medium` | `chore` | needs-brainstorm |
| [0178](active/0178-fix-the-bsd-grep-parse-error-truncating-test-docket-example.md) | Fix the BSD-grep parse error truncating test_docket_example_yml.sh | `medium` | `fix` | needs-brainstorm |
| [0179](active/0179-revisit-factoring-a-shared-config-value-extractor-across-the.md) | Revisit factoring a shared config value extractor across the three readers | `medium` | `refactor` | needs-brainstorm |
| [0180](active/0180-apply-adr-0065-s-quote-leg-to-hd-validate-and-the-remaining.md) | Apply ADR-0065's quote leg to hd_validate and the remaining flow-map truncation corners | `medium` | `fix` | needs-brainstorm |
| [0181](active/0181-document-the-unquoted-space-free-rule-for-agent-model-effort.md) | Document the unquoted, space-free rule for agent model/effort config values | `medium` | `docs` | needs-brainstorm |
| [0182](active/0182-facade-tests-read-the-developer-s-real-global-config-instead.md) | Facade tests read the developer's real global config instead of a sandbox | `medium` | `fix` | needs-brainstorm |
| [0187](active/0187-harden-the-docket-example-yml-mirror-guards-one-directional.md) | Harden the .docket.example.yml mirror guards — one-directional coverage, an unexercised round-trip slice, and a prefix-weak terminator | `medium` | `chore` | needs-brainstorm |
| [0188](active/0188-backfill-change-types-sh-calls-mktemp-d-with-no-template-so.md) | backfill-change-types.sh calls mktemp -d with no template, so TMPDIR is ignored on macOS and uchg fixtures leak undeletable dirs | `medium` | `fix` | needs-brainstorm |
| [0189](active/0189-sweep-the-15-remaining-bare-mv-install-sites-a-tty-prompt-ma.md) | Sweep the 15 remaining bare-mv install sites — a tty prompt makes their || die guards unreachable | `medium` | `fix` | needs-brainstorm |
| [0195](active/0195-retune-the-opencode-shipped-model-defaults-for-cost.md) | Retune the opencode shipped model defaults for cost | `medium` | `chore` | needs-brainstorm |
| [0196](active/0196-shared-agents-md-dispatch-block-restate-and-test-the-single.md) | Shared AGENTS.md dispatch block — restate and test the single-owner assumptions | `medium` | `fix` | needs-brainstorm |
| [0197](active/0197-clear-the-unfixed-review-findings-from-change-0193.md) | Clear the unfixed review findings from change 0193 | `medium` | `chore` | needs-brainstorm |
| [0198](active/0198-settle-the-role-self-description-rule-s-positive-half-docket.md) | Settle the role-self-description rule's positive half — docket-review names no skills.review binding | `medium` | `docs` | needs-brainstorm |
| [0199](active/0199-harden-the-role-self-description-guard-co-occurrence-gap-har.md) | Harden the role-self-description guard — co-occurrence gap, hardcoded population, broad default matcher | `medium` | `chore` | needs-brainstorm |
| [0200](active/0200-clear-the-unfixed-review-findings-from-change-0191.md) | Board-checks and test-suite hardening — sanitize LF escape, mutation G, minor-finding clearance, mapfile floor | `medium` | `fix` | needs-brainstorm |
| [0204](active/0204-restore-rationale-dropped-by-round-three-compression-finaliz.md) | Restore dropped doc rationale (compression losses) and complete AGENTS.md's frontmatter-edit rule | `medium` | `docs` | needs-brainstorm |
| [0208](active/0208-runner-dispatch-worktree-gate-3-proves-repo-containment-not.md) | Harden runner-dispatch — worktree membership gate, feature-scoped coverage, flag-parse guards | `medium` | `fix` | needs-brainstorm |
| [0221](active/0221-assert-executes-backticks-in-its-test-description-so-a-verba.md) | assert() executes backticks in its test description, so a verbatim-quoted anchor can run shell | `medium` | `fix` | needs-brainstorm |
| [0222](active/0222-raise-docket-s-minimum-bash-from-4-to-4-4.md) | Raise docket's minimum Bash from 4+ to 4.4 | `medium` | `chore` | needs-brainstorm |
| [0224](active/0224-the-build-gate-contract-never-says-green-red-is-the-exit-code.md) | The build gate contract never says green/red is the exit code, so an output-shape match passes as a gate | `high` | `docs` | needs-brainstorm |
| [0229](active/0229-the-runner-s-budget-slack-factor-is-a-hardware-dependent-con.md) | the runner's budget slack factor is a hardware-dependent constant | `medium` | `refactor` | needs-brainstorm |
| [0230](active/0230-a-self-scanning-population-floor-pins-test-docket-config-sh.md) | a self-scanning population floor pins test_docket_config.sh's size and blocks sharding | `medium` | `refactor` | needs-brainstorm |
| [0232](active/0232-the-gate-execution-posture-never-reaches-the-build-workers-t.md) | The gate execution posture never reaches the build workers that also run the full suite | `medium` | `docs` | needs-brainstorm |
| [0233](active/0233-guard-against-stacked-gap-ere-patterns-that-hang-instead-of.md) | Guard against stacked-gap ERE patterns that hang instead of failing | `medium` | `chore` | needs-brainstorm |
| [0236](active/0236-suppressed-execution-handoff-still-ends-run-at-plan.md) | A suppressed execution hand-off still ends the run at the plan — 0113 recurrence | `high` | `fix` | auto-groom blocked — needs you |

## ⚪ Deferred (4)

| # | Title | Priority | Type |
|---|-------|----------|------|
| [0007](active/0007-recurring-change-templates.md) | Recurring change templates — scheduled maintenance work that spawns proposed instances | `medium` | `feat` |
| [0008](active/0008-parallel-backlog-drain.md) | Parallel backlog drain — fan out concurrent implement-next runs over independent build-ready changes | `medium` | `feat` |
| [0009](active/0009-human-escalation-loop.md) | Human escalation loop — structured questions-for-you in the change file, answered asynchronously in git | `medium` | `feat` |
| [0010](active/0010-board-analytics.md) | Board analytics — throughput and cycle-time stats derived from git history, rendered on BOARD.md | `low` | `feat` |

```mermaid
graph TD
  0007
  0008
  0009
  0010
  0015 --> 0019
  0082
  0100
  0103
  0110
  0118
  0119
  0121
  0123
  0125
  0134
  0139
  0140
  0141
  0142
  0147
  0150
  0151
  0154
  0155
  0156
  0158
  0159
  0160
  0163
  0165
  0166
  0171
  0172
  0177
  0178
  0175 --> 0179
  0180
  0181
  0182
  0187
  0188
  0189
  0190
  0192 --> 0195
  0196
  0197
  0198
  0199
  0200
  0204
  0208
  0211 --> 0219
  0221
  0211 --> 0222
  0224
  0229
  0230
  0223 --> 0231
  0232
  0233
  0235
  0236
  0015:::done
  0175:::done
  0192:::done
  0211:::done
  0223:::done
  classDef done fill:#d3f9d8;
```

<details><summary>✅🗑️ Archive — done + killed (174)</summary>

| # | Title | Merged |
|---|-------|--------|
| [0234](archive/2026-08-07-0234-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b.md) | Split gate-execution.md: probe evidence should not sit on a blocking-read surface | 2026-08-07 |
| [0228](archive/2026-08-07-0228-finalize-s-auto-detect-suite-loop-has-no-failure-accumulator.md) | finalize's auto-detect suite loop has no failure accumulator, so a mid-suite red merges | 2026-08-07 |
| [0227](archive/2026-08-07-0227-parallel-test-suite-runner.md) | Parallel test-suite runner — 4x+ wall-clock speedup | 2026-08-07 |
| [0226](archive/2026-08-07-0226-reframe-auto-capture-as-capability-discovery-with-strict-adm.md) | Reframe auto-capture as capability discovery with strict admission gates | 2026-08-07 |
| [0225](archive/2026-08-07-0225-the-test-suite-has-grown-into-the-harness-s-foreground-ceilin.md) | The test suite has grown into the harness's foreground ceiling — cut its wall-clock runtime | 2026-08-07 |
| [0223](archive/2026-08-07-0223-the-build-gate-contract-never-states-an-execution-posture-for.md) | The build gate contract never states an execution posture for a suite that outgrows a single foreground call | 2026-08-07 |
| [0220](archive/2026-08-07-0220-clear-the-unfixed-review-findings-from-change-0207.md) | clear the unfixed review findings from change 0207 | 2026-08-07 |
| [0203](archive/2026-08-07-0203-define-the-per-step-git-state-postcondition-docket-implement.md) | Define the per-step git-state postcondition docket-implement-next now names but never states | 2026-08-07 |
| [0218](archive/2026-08-06-0218-fix-review-findings-in-branch-instead-of-minting-a-stub-for.md) | Fix review findings in-branch instead of minting a stub for every one | 2026-08-06 |
| [0217](archive/2026-08-05-0217-clear-change-0202-s-three-minor-findings-dead-guard-stale-ba.md) | Clear change 0202's three minor findings: dead guard, stale baseline comment, wrong plan pattern | 2026-08-05 |
| [0216](archive/2026-08-05-0216-guard-the-capture-shape-constraint-in-branch-only-artifact-w.md) | Guard the capture-shape constraint in branch_only_artifact with a mutation G | 2026-08-05 |
| [0215](archive/2026-08-05-0215-escape-newlines-in-board-checks-sanitize-now-that-z-can-deli.md) | Escape newlines in board-checks sanitize now that -z can deliver a raw LF | 2026-08-05 |
| [0214](archive/2026-08-05-0214-agents-md-s-promoted-frontmatter-rule-omits-the-whitespace-c.md) | AGENTS.md's promoted frontmatter rule omits the whitespace-class half that corrupted two field writes | 2026-08-05 |
| [0213](archive/2026-08-05-0213-settle-the-bash-4-4-mapfile-d-floor-inconsistency-between-te.md) | Settle the bash 4.4 mapfile -d floor inconsistency between tests and shipped scripts | 2026-08-05 |
| [0212](archive/2026-08-05-0212-an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco.md) | An inlined role skill's terminal stop ends the whole run — scope docket-build's stop and enforce the run disposition | 2026-08-05 |
| [0211](archive/2026-08-05-0211-aborted-run-is-blind-to-a-run-that-stops-after-the-build-com.md) | aborted-run is blind to a run that stops after the build: commits on an unpushed branch, every field coherent | 2026-08-05 |
| [0210](archive/2026-08-05-0210-a-valueless-trailing-flag-hangs-runner-dispatch-forever-inst.md) | A valueless trailing flag hangs runner-dispatch forever instead of aborting | 2026-08-05 |
| [0209](archive/2026-08-05-0209-the-worktree-requirement-covers-build-only-leaving-three-fea.md) | The --worktree requirement covers build-* only, leaving three feature-scoped agent families ungated | 2026-08-05 |
| [0207](archive/2026-08-05-0207-sync-agents-aborts-mid-loop-on-a-bad-runner-config-leaving-a.md) | sync-agents aborts mid-loop on a bad runner config, leaving a zero-length wrapper and stale siblings | 2026-08-05 |
| [0206](archive/2026-08-05-0206-delegated-runner-runs-are-anchored-at-the-main-worktree-not.md) | Delegated runner runs are anchored at the main worktree, not the feature worktree | 2026-08-05 |
| [0205](archive/2026-08-05-0205-opencode-runner-adapter.md) | opencode runner adapter — delegate build workers to OpenRouter models | 2026-08-05 |
| [0202](archive/2026-08-05-0202-clear-the-unfixed-review-findings-from-change-0113.md) | Clear the unfixed review findings from change 0113 | 2026-08-05 |
| [0078](archive/2026-08-05-0078-codex-cli-validation-runbook.md) | Codex CLI live-validation runbook — prove docket works end-to-end under Codex | 2026-08-05 |
| [0183](archive/2026-08-01-0183-cursor-dispatch-head-ships-a-stale-unpinned-claim-its-guard.md) | Cursor dispatch head ships a stale unpinned claim; its guard retired itself | 2026-08-01 |
| [0044](archive/2026-07-30-0044-configurable-build-model.md) | Configurable SDD build models for docket-implement-next | 2026-07-30 |
| [0162](archive/2026-07-28-0162-restore-the-machine-local-ignored-advisory-for-a-committed-t.md) | Restore the machine-local-ignored advisory for a committed too-deep runtime.bash | 2026-07-28 |
| [0161](archive/2026-07-28-0161-enumerate-the-health-checks-failed-outcome-in-docket-status.md) | Enumerate the health-checks-failed outcome in docket-status SKILL.md | 2026-07-28 |
| [0153](archive/2026-07-28-0153-decide-whether-the-runtime-bash-leaf-match-should-be-depth-a.md) | Decide whether the runtime.bash leaf match should be depth-anchored | 2026-07-28 |
| [0152](archive/2026-07-28-0152-consolidate-the-two-surviving-hand-rolled-gnu-bash-4-validat.md) | Consolidate the two surviving hand-rolled GNU Bash 4+ validator copies | 2026-07-28 |
| [0149](archive/2026-07-28-0149-make-the-prelude-guard-s-exemption-bound-proportional-and-cl.md) | Make the prelude guard's exemption bound proportional, and close the partial-rename gap | 2026-07-28 |
| [0148](archive/2026-07-28-0148-two-unfalsifiable-z-asserts-in-the-config-suite-sit-in-eval.md) | Two unfalsifiable -z asserts in the config suite sit in eval-free blocks | 2026-07-28 |
| [0146](archive/2026-07-28-0146-widen-the-config-read-channel-guard-to-the-sibling-config-la.md) | Widen the config read-channel guard to the sibling config layers it does not match | 2026-07-28 |
| [0144](archive/2026-07-28-0144-a-board-checks-sh-non-zero-exit-silently-voids-the-entire-he.md) | A board-checks.sh non-zero exit silently voids the entire health pass | 2026-07-28 |
| [0143](archive/2026-07-28-0143-empty-id-collapses-the-archive-sort-feeder-s-tab-joined-fiel.md) | Empty id collapses the archive sort feeder's TAB-joined fields in render-board.sh | 2026-07-28 |
| [0131](archive/2026-07-26-0131-make-board-conflict-rebase-continuation-noninteractive.md) | Make board-conflict rebase continuation noninteractive | 2026-07-26 |
| [0129](archive/2026-07-26-0129-fix-the-pipefail-unsafe-plain-format-config-assertion.md) | Fix the pipefail-unsafe plain-format config assertion | 2026-07-26 |
| [0124](archive/2026-07-26-0124-backlog-triage-pass.md) | Backlog triage pass — kill, defer, or arm each needs-brainstorm stub | 2026-07-26 |
| [0105](archive/2026-07-20-0105-pin-docket-mode-main-coverage-for-docket-status-digest-only.md) | Pin DOCKET_MODE=main coverage for docket-status --digest-only | 2026-07-20 |
| [0086](archive/2026-07-18-0086-attended-finalize-merge-path.md) | Attended finalize has no merge path under auto_approve — scope the --admin ban to autonomous runs | 2026-07-18 |
| [0033](archive/2026-07-16-0033-adr-index-main-maintenance.md) | Decide how the ADR index is maintained on the integration branch | 2026-07-16 |
| [0076](archive/2026-07-14-0076-cwd-independent-repo-root-resolution.md) | Resolve the repo root independently of CWD — preflight run inside `.docket` mints a nested metadata worktree | 2026-07-14 |
| [0043](archive/2026-07-08-0043-agent-model-tiers.md) | Model-tier indirection for agent model selection + config-driven advisories | 2026-07-08 |
| [0028](archive/2026-06-20-0028-wire-closeout-call-sites.md) | Wire the close-out call sites to the extracted scripts | 2026-06-20 |

**Older done (collapsed)**

| Month | Done |
|-------|------|
| [2026-08](archive/) | 13 done |
| [2026-07](archive/) | 86 done |
| [2026-06](archive/) | 32 done |

</details>
