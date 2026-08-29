<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0318 — Go-only source cutover](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0318-config-contraction-self-hosting-and-hard-cutover.md)**
<!-- docket:backlink:end -->

# Go-Native Whole-Suite Test Runner and Gate Cutover

## Summary

Change 0318 introduces `docket development test` as the Go-native owner of whole-repository test orchestration and moves every canonical source-validation gate to that command. It is the first stage of a sequential cutover: 0318 establishes the runner, 0369 migrates maintained consumers to direct Go, and 0370 removes the unused Bash facade and legacy test surface.

The merged intermediate state is intentionally complete and usable. The new Go runner owns scheduling, isolation, result validation, aggregation, signal behavior, and budget enforcement while the existing Bash facade, `scripts/run-tests.sh`, helper/runtime tree, callers, and test shards remain present and green as the compatibility corpus and parity oracle.

The public command is:

```text
docket development test
```

Repository gates validate the checkout under review through a branch-faithful source entry, normally:

```text
go run ./cmd/docket development test
```

An equivalent entry is acceptable only if it is documented, deterministic, forwards signals and status faithfully, and cannot silently select a previously installed binary.

## Goals

- Add `docket development test` as the Go-native whole-suite runner.
- Make it the sole canonical orchestration channel for repository-wide testing.
- Test the exact checkout under review rather than a stale installed executable.
- Preserve the observable correctness and diagnostic semantics of the existing runner.
- Continue executing all existing Bash test shards and Go targets during the transition.
- Cut `finalize.test_command`, contributor whole-suite instructions, and release-candidate source validation over to the new command.
- Establish differential, synthetic, and mutation evidence sufficient for later consumer migration and facade deletion.
- Leave a green, independently usable repository after merge.

## Non-goals

0318 does not migrate facade callers; remove or contract `DOCKET_SCRIPTS_DIR`, `DOCKET_BASH_PATH`, `runtime.bash`, or related configuration; delete `scripts/docket.sh`, `scripts/run-tests.sh`, helpers, runtimes, or test shards; rewrite the Bash corpus as Go tests; introduce a forwarding shim; broadly rewrite dispatch assets or active documentation; publish a release; perform fresh-host self-hosting; execute 0366; or implement 0367.

## Command and source-fidelity contract

`docket development test` is non-interactive and runs the complete configured suite. Unsupported arguments and options use the CLI's normal usage-error contract.

Source-validation gates must enter the command from the repository checkout. The entry must:

- build or invoke CLI source from the current checkout;
- preserve working-tree and branch fidelity;
- never fall back to an installed `docket`;
- forward signals and exit status without reinterpretation;
- derive repository paths from the checkout being tested; and
- avoid overwriting the user's installed binary or unrelated configuration.

After this change, `finalize.test_command`, primary contributor documentation, and release-candidate source validation all use that same authoritative entry. Focused test commands may remain documented, but `scripts/run-tests.sh` is no longer presented as an alternative whole-suite gate.

## Suite discovery and composition

The runner obtains the full suite from one authoritative definition or deterministic discovery rule. It must not create a second hand-copied target list. The transitional suite includes all Go targets, existing Bash shards, and any other targets already belonging to the authoritative suite.

Target identity must be explicit and stable. Discovery errors, invalid declarations, duplicate identities, unreadable inputs, and targets whose execution contracts cannot be constructed fail closed. Concurrency eligibility comes from maintained metadata or structural policy, never an inference that a target is safe because it once passed concurrently.

## Execution model

Each scheduled target receives an isolated context covering its temporary directory, durable result destination, runner-controlled environment, scratch files, and logs. Repository access remains available when the target contract requires it. Cleanup cannot erase diagnostic evidence before aggregation.

Parallel-safe targets may run concurrently under a bounded policy. Unsafe or exclusive targets remain serial. Scheduling and completion order may differ, but reporting order is deterministic.

For every target the runner tracks stable identity, scheduling and start state, process completion or interruption, durable result state, captured diagnostics, elapsed wall time, budget screening, and any serial confirmation.

## Durable result protocol

Exactly one valid durable result must be attributable to every scheduled target. The following are failures:

- no result;
- duplicate results;
- wrong-target, unknown-target, or unscheduled-target identity;
- malformed, truncated, incomplete, or unsupported state;
- conflict between the result and runner-observed execution; or
- inability to determine whether publication completed durably.

Publication is atomic from the aggregator's perspective. A successful child status cannot substitute for a missing result, and a nominal result cannot conceal launch, execution, or termination failure. Validation covers the complete scheduled set rather than stopping after the first failure.

## Deterministic aggregation and observability

The aggregate is stable for the same suite and outcomes, independent of process completion order. Stable target order governs summaries, failures, invalid-result diagnostics, budget findings, and final state.

The report distinguishes ordinary assertions, launch/infrastructure failures, invalid or missing results, interruptions, screening findings, authoritative budget breaches, and aggregate gate outcome. All independently discoverable failures are reported. Per-target output remains attributable; raw interleaving is not the only record.

Stable diagnostic clauses relied upon by repository gates, including the budget classifications, remain machine-detectable. Verbose output must not expose unrelated environment secrets.

## Interruption and process lifecycle

On supported termination or interruption signals the runner:

1. stops scheduling;
2. forwards the signal to running process groups;
3. allows only a bounded cleanup interval;
4. prevents orphaned children;
5. collects already durable results and diagnostics;
6. marks incomplete targets interrupted or missing; and
7. exits non-successfully with the expected interruption meaning.

Escalation for children that ignore the first signal is bounded and tested. The contract covers parallel execution, serial execution, serial budget confirmation, and interruption between scheduling and launch. No required interruption can produce a clean pass.

## Budget and gate semantics

The runner preserves the screen-then-confirm model:

- A parallel overage emits a `BUDGET WATCH:`-equivalent screening finding. Machine-sensitive screening is not authoritative failure.
- A screened target is rerun under defined serial conditions.
- A confirmed breach emits a `SERIAL CONFIRMED OVER BUDGET:`-equivalent authoritative finding.
- Confirmation launch failure, test failure, interruption, within-budget confirmation, and confirmed overage remain distinct.

Thresholds, units, measurement boundaries, and rounding remain sourced from authoritative existing policy or are changed only with explicit parity evidence.

ADR-0074 remains normative. The runner preserves clean success, completed-with-observation, and authoritative failure/indeterminate states; callers must not collapse this into a warning grep or a binary child exit interpretation.

## Frozen prior workflow boundary

The Bash facade, helper/runtime tree, `scripts/run-tests.sh`, current callers, and test corpus remain present and green. They serve as a stable prior workflow, parity oracle, and migration corpus for 0369 and 0370. No success criterion depends on weakening a test because its mechanism will later be removed.

0318 introduces no forwarding shim and does not modify the legacy architecture beyond narrowly necessary accommodations to execute its targets under the Go runner. Go owns canonical orchestration; Bash remains temporary compatibility residue, not a second documented canonical command.

## Parity and mutation strategy

A differential harness feeds equivalent controlled suites to the Bash and Go runners and compares normalized observations for discovery, stable order, success, ordinary failure, launch failure, concurrency classification, isolation, missing/malformed/duplicate/wrong-target results, multi-failure aggregation, interruption, budget screening, serial confirmation, and tri-state interpretation.

Normalization may remove temporary paths, PIDs, and timing jitter, but not target identity, failure category, scheduling class, result validity, budget authority, or gate outcome. Any intentional deviation from accidental or unsafe Bash behavior is documented and receives a focused contract test.

Synthetic targets deterministically model success, failure, delays, result omission/corruption/duplication/mismatch, child process trees, signals, parallel overlap, and budget outcomes. Timing tests use synchronization or controllable thresholds rather than narrow sleeps alone.

Mutation-sensitive evidence must redden when the implementation is changed to:

- omit a scheduled target from validation;
- accept zero, duplicate, malformed, or wrong-target results;
- aggregate in completion order;
- schedule an unsafe target concurrently;
- skip signal propagation or orphan cleanup;
- treat parallel screening as authoritative;
- skip required serial confirmation;
- collapse ADR-0074 states; or
- invoke an installed binary at a source-validation gate.

## Gate and documentation cutover

`finalize.test_command` is the sole configuration source for the build gate and resolves to the branch-faithful Go entry. Tests and documentation derive it rather than introduce another copy.

Contributor documentation names the same full-suite command and distinguishes source-checkout validation from running an arbitrary installed binary. Release-candidate source validation uses the same source entry without requiring the candidate to have installed itself. Tagging, publication, public installation, rollback, and fresh-host proof remain in 0366.

## Failure handling

Runner uncertainty fails closed. Discovery/parse failure, isolation failure, launch failure, result publication/read failure, identity contradiction, child-reaping failure, serial-confirmation failure, missing aggregation input, and unsupported gate state all yield attributable non-clean results distinct from ordinary test assertions.

If parity exposes behavior that cannot safely be preserved, the implementation documents the difference and proves the replacement contract. It may not silently narrow the suite or bypass the old behavior.

## ADR impact

ADR-0074 remains normative. ADRs 0014, 0029, 0030, 0033, 0036, and 0099 remain operationally relevant because their mechanisms still exist after 0318. Their facade-free disposition belongs to 0369 and 0370. Any durable new architectural choice discovered during implementation is recorded separately rather than hidden in code.

## Acceptance criteria

1. `docket development test` exists as a non-interactive whole-suite command.
2. It executes the complete authoritative suite, not a separately maintained subset.
3. `finalize.test_command`, contributor instructions, and release-candidate source validation use one branch-faithful source entry that cannot select a stale installed binary.
4. All existing required Go targets and Bash shards run through the new runner.
5. The Bash facade, old runner, helpers/runtime, callers, and tests remain present and green.
6. No forwarding shim, caller migration, facade deletion, or broad configuration contraction is introduced.
7. Every target receives isolated runner-controlled temporary and result state.
8. Only explicitly safe targets overlap; concurrency is bounded.
9. Exactly one attributable durable result is required for every scheduled target.
10. Missing, malformed, duplicate, incomplete, unknown-target, and wrong-target results fail with attributable diagnostics.
11. Result publication is atomic to the aggregator.
12. Aggregation and reporting are deterministic and preserve all independently discoverable failures.
13. Interruption stops scheduling, reaches child trees, prevents orphans, preserves diagnostics, and cannot pass.
14. Parallel budget overages remain screening findings with a `BUDGET WATCH:`-equivalent clause.
15. Required serial confirmation precedes authoritative budget failure, which retains a `SERIAL CONFIRMED OVER BUDGET:`-equivalent clause.
16. Confirmation launch, test, interruption, within-budget, and confirmed-overage outcomes remain distinguishable.
17. ADR-0074's clean, observed, and authoritative/indeterminate states have focused contract coverage.
18. Differential tests cover discovery, scheduling, isolation, results, aggregation, interruption, budgets, and gate state.
19. Synthetic fixtures deterministically cover invalid-result, scheduling, signal, and budget conditions.
20. Mutation tests prove exact-result validation, stable aggregation, safe scheduling, signals, serial confirmation, tri-state interpretation, and source fidelity are load-bearing.
21. Internal runner failures fail closed and remain distinct from product assertion failures.
22. The complete suite passes through the new canonical runner with every authoritative serial budget breach resolved or explicitly dispositioned.
23. The merged state is usable without 0369, 0370, or 0366.
24. No release, fresh-host proof, rollback, facade deletion, post-cutover board configuration, or unrelated capability work is included.

## Assumptions

- `docket development` is the accepted maintainer namespace.
- The current suite can expose one authoritative deterministic discovery mechanism without redesigning individual tests.
- Existing Bash shards can run as child targets before their product-facing callers migrate.
- The existing result and budget behaviors are intentional unless differential analysis documents a defect.
- ADR-0074 sufficiently defines the runner-facing tri-state contract.
- A Go toolchain is available in source-development environments.
- The Bash runner remains stable enough to serve as a parity oracle.
- 0369 and 0370 merge sequentially on `main`, not as stacked unmerged branches.
- 0366 and 0367 will depend on the final deletion stage.
- Historical records, Accepted ADRs, archived material, and frozen fixtures are not rewritten.
