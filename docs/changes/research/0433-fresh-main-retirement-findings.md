# Change 433 — fresh-main restart and failed-approach retirement

Date: 2026-09-18. Human-directed backlog retirement; no implementation or merge.

## Decision

Abandon the implementation route represented by 425 and its direct follow-ups and acceptance repairs. Start [433](../active/0433-pilot-top-level-codex-coordinators-with-one-level-native-dis.md) afresh from a verified main checkout, with a top-level Codex coordinator and direct leaf agents. The old stack is research evidence only. No wholesale merge or cherry-pick is authorized.

The required invariant is **leave Claude Code and Cursor and opencode behavior unchanged**. It includes shared skills, generated instructions/assets, configuration, model selection, ownership, gates, cancellation/resume, persistence compatibility, review and publication—not just unchanged adapter files.

## Retirement inventory and findings

### 424 — model-capability registry and failed production dogfood

Killed and [archived](../archive/2026-09-18-0424-validate-codex-coordinator-models-against-a-versioned-capabi.md). The registry followed the assumption that a separately spawned coordinator needs a dispatch-capable model. The new topology instead needs authoritative verification of the actual root session plus valid leaf roles; none of 424's registry design is automatically inherited.

Native planning and Tasks 1–3 completed with focused red/green and mutation evidence. HEAD is 434ebe2aac3f96d572a813368b4bcf030e41330c; prior task commits are 9fac14eaff13f448e576cd8e244532507582a573 and 1410893297c80f52febb9a0d5343fc1e46beb112; the plan commit is 4870144f. Task 4 remains dirty in /Users/homer/dev/docket/.worktrees/validate-codex-coordinator-models-against-a-versioned-capabi on feat/validate-codex-coordinator-models-against-a-versioned-capabi. Its eight modified install/fixture paths are enumerated in 433. Preserve this worktree and backup/424-plan-attempt-20260917.

Operational failures included premature coordinator completion while children were live; lost shell handles and true exit codes; incomplete assignment/write-grant/resource payloads; confusing handoff generation with missing credentials; existing fingerprint discovery; delayed release of completed native thread slots; and cancelled-history admission ambiguity. Some validators correctly rejected bad inputs. Native nesting succeeded, so a categorical claim that Codex cannot nest is not supported. No controlled cross-harness comparison proved nesting alone causal.

Task 4 also exposed genuine policy/fixture defects, including temporary application to non-Codex selection and missing CLI/harness fixture metadata. Tasks 5–6, full final certification, independent review, results and reviewed PR were never reached. The prior 96s/90s serial timing finding remains unwaived. The final stop cancelled the undispatched replacement; current native-agent inventory shows completed children. Do not reuse old scope authority.

Local full chronology: /Users/homer/dev/docket-0424-restart.ZyKU6s/operational-evidence.MTtgDV/REPORT.md; private captures and reset evidence remain under that restart directory. 433 carries the portable substantive narrative. No private capability or credential-bearing capture is published here.

### 425 — native nested-coordinator production architecture

Killed and [archived](../archive/2026-09-18-0425-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor.md). [PR #303](https://github.com/danielhanold/docket/pull/303) was OPEN at initial inspection and was then CLOSED without merging, with mergedAt null verified; its exact published head remains 70de2fc251ea9720a84bfe3a02c0f79bf15ed21e, branch codex/restore-native-codex-dispatch-for-multi-agent-v2-docket-coor. That branch, its worktree, backup/425-before-main-rebase-20260917, candidate runtime and source evidence are preserved.

The production route replaced automatic shell/app-server entry with native named coordinator/feature children and explicit feature-worktree binding. It accumulated assignment/payload/resource-closure checks, gate transport/receipt validation, diagnostics, fixture generation and follow-on operational repairs. Source review caught and repaired resolver authored/generated conflict handling and an overly broad runtime-manifest path rejection. These fixes and passing source tests are useful evidence, not proof of sustained unattended orchestration.

Against main 3ccf9fac511f370c200315674a1edf97766b0a5b, the published aggregate spans 194 files, 11,971 additions and 695 deletions, including generated mirrors, tests and historical documents. Crucially, it changes shared gate admission, cancellation/replacement recovery, scope schema 3 to 4, metadata/artifact diagnosis, finalization and common workflow prose. Cursor's run-gate instructions also change. Unchanged Claude/Cursor/OpenCode adapter files therefore do not establish unchanged behavior.

PR source, packaging and platform-smoke CI was green at inspection. The release-candidate workflow explicitly leaves live four-harness acceptance outstanding. There is regression risk, not demonstrated corruption of other harnesses. Older binaries reject new scope records: upgrade/rollback compatibility needs explicit evidence before any comparable shared-format change is considered for 433.

[Source results at the retained commit](https://github.com/danielhanold/docket/blob/70de2fc251ea9720a84bfe3a02c0f79bf15ed21e/docs/results/2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-results.md) remain point-in-time evidence.

### 426 — legacy agent.enter retirement after 425

Killed and [archived](../archive/2026-09-18-0426-constrain-agent-enter-to-explicit-legacy-and-feature-worktre.md). This proposed follow-up depended on 425's native nested-coordinator route becoming production. That prerequisite is abandoned. No branch or implementation is recorded. Its useful distinction survives as a design question for 433: root launch compatibility and feature-worktree safety are different concerns. Do not delete feature binding just because the intermediate coordinator disappears. Assess the APIs that actually exist on main rather than carrying forward 426's migration plan.

### 430 — first durable-review acceptance attempt

Already killed and archived before this retirement; preserve that outcome. Native worker commit 6aa8e33f727dd2df581d6cd04d3e6f56cb685f62 passed focused red/green but acknowledged its scope before the required active-input recheck. The closed-scope check then refused. Native review, complete build certification, evidence/results publication and PR were not reached. This is a meaningful ordering failure, not a successful acceptance result.

[Archived 430](../archive/2026-09-17-0430-native-codex-acceptance-for-durable-review-evidence.md) and its fixture remain research input.

### 431 — repaired active-worker acceptance

Killed and [archived](../archive/2026-09-18-0431-native-codex-acceptance-for-active-worker-validation.md) under the explicit human-approved metadata exception. Its real [PR #309](https://github.com/danielhanold/docket/pull/309) merged into 425, NOT main, on 2026-09-17. Recorded head f6695e5f8fbb198ba9458d40cfeb215a2af928bc; merge commit 8982b2873580f5677a0900abc94bfaef22403028; branch chore/native-codex-acceptance-for-active-worker-validation.

The small Value/Double fixture reached native planner, worker and reviewer, results and final certification after repairs and human intervention. The final gate at that head and the recorded 56-file/460-assertion suite are successful evidence for that fixture. A 61s/60s serial timing exception was explicitly authorized only for that closeout; it does not waive 424's separate budget finding.

Preserve the achieved behavior without generalizing to unattended operation or other harnesses. Marking the abandoned delivery killed must not erase the actual GitHub merge event or relabel it as never merged. It never reached integration through 425.

### 432 — complete-native-runner repairs and documentation

Killed and [archived](../archive/2026-09-18-0432-complete-native-codex-runner.md) under the explicit human-approved metadata exception. Its repairs were carried through 431/PR #309; documentation-only [PR #310](https://github.com/danielhanold/docket/pull/310) then merged into 425, NOT main, on 2026-09-17. Recorded head bc78d999d32069e828ba401f26b203b46864dbac; merge commit 8deef57f6ff8764d06e786b4a27bda504c202e25; branch fix/complete-native-codex-runner.

The history records misread continuation receipts, native-child completion/transport problems, exact-head evidence handoff and human-assisted recovery. Its documentation closeout used an explicitly authorized bookkeeping exception, not a fabricated managed-workspace or full-suite receipt. Original plans/results and the shared-orchestration/supervisor research remain frozen.

[Retained completion research](https://github.com/danielhanold/docket/blob/70de2fc251ea9720a84bfe3a02c0f79bf15ed21e/docs/research/native-codex-runner/README.md) and [shared simplification findings](https://github.com/danielhanold/docket/blob/70de2fc251ea9720a84bfe3a02c0f79bf15ed21e/docs/research/shared-orchestration-simplification.md) are references, not implementation prerequisites.

## Scope exclusions: do not rewrite unrelated or completed history

- 423 is a completed, explicitly accepted POC with a deliberate bounded halt, no production PR, no hard-isolation/parallel-safety certification and an incomplete evidence audit disclosed at acceptance. Its positive native-dispatch observations stand; they do not certify the abandoned production architecture.
- 412 is a separate harness-neutral gate-supervisor proposal motivated by earlier 349/323 incidents. 422 fixes a genuine repeated-observation retry-accounting defect from 421 and was independently branched from main. Neither is stacked on 425. Keep both separate; 433 inherits neither, and retirement is not a fix or waiver of their underlying issues.
- 427, 428 and 429 are already completed shared recovery/stack fixes; their records are not retroactively killed and main is not reverted by this request.
- Other records mentioning Codex or a related change number are not automatically part of the abandoned architecture. No unrelated backlog item is cancelled by keyword matching.

## Lessons and acceptance boundary for 433

1. Start with main's actual contracts. Minimize the Codex-specific implementation; do not import the failed stack and rename its layers.
2. Expected model/effort in config is not the actual session's identity. Define authoritative matched/mismatched/unverified handling without self-attestation.
3. One-level dispatch does not provide worktree isolation, child cancellation, or guaranteed root persistence on its own.
4. Preserve useful safety guarantees while moving exact payload/receipt/session bookkeeping out of model judgment wherever necessary.
5. Distinguish a transport yield, workflow WAITING, terminal failure, and unknown outcome. Preserve the original operation identity and exact exit status.
6. Keep cancellation, live-writer quiescence, continuation single use and exact-head verification. Do not erase checks simply because they exposed a bad caller.
7. Protect Claude Code, Cursor and OpenCode at shared-instruction and persisted-state boundaries. Test real workflow and interruption/resume behavior on the exact candidate; an unavailable harness leaves an outstanding item.
8. Record interventions and stop criteria. A successful probe, a repaired fixture, green CI or repeated manual rescue is not evidence of unattended reliability.
9. Preserve predecessor evidence and accepted ADR bodies. Any future architecture reversal/supersession is a new reviewed ADR, not a rewrite of historical findings.

## Execution disposition

Findings were recorded and pushed before terminal transitions. The supported change.kill transactions archived 431, 432, 424, 426 and 425 in that order and regenerated their metadata backlinks and the board. For 431/432 only, the explicit human exception permitted a documented temporary administrative in-progress status so the existing archival transaction could run; no implementation was resumed and no new claim or run authority was acquired. The resulting killed status abandons delivery without rewriting either GitHub merge event. 430 was already killed.

PR #303 is verified closed and unmerged. Remote 425/431/432 branches retain their inspected heads; 424 retains its exact Task 3 head and eight dirty Task 4 paths. No source changes, installation, model-pin changes, branch/worktree deletion, run-state rewriting, or merge was performed. Cleanup was deliberately omitted under the preservation instruction. 433 stays proposed/needs-brainstorm with effective base main. Original generated artifact links may not locate unshipped feature files after terminal rendering; the exact-commit evidence links in the retirement rationales and this review preserve their real locations without pretending they shipped to main.
