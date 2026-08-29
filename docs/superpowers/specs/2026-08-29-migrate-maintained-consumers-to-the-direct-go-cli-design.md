<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0369 — Migrate maintained consumers to the direct Go CLI](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0369-migrate-maintained-consumers-to-the-direct-go-cli.md)**
<!-- docket:backlink:end -->

# Migrate Maintained Consumers to the Direct Go CLI

## Summary

Change 0369 is stage two of the source cutover. After 0318 establishes the Go-owned transitional test runner, this change migrates every maintained executable consumer from the Bash facade and helper paths to the PATH-resolved Go `docket` CLI and its public JSON contracts. The legacy implementation remains present, frozen, and testable, but no maintained consumer may depend on it. Change 0370 performs physical deletion afterward.

The merged intermediate state must be independently green and usable. This change introduces no replacement forwarding shim.

## Goals

- Make the PATH-resolved Go executable the direct control plane for every maintained consumer.
- Preserve the behavior and machine-readable contracts each consumer requires.
- Update canonical generators before regenerating checked-in products.
- Test generated and process-start-loaded integrations honestly.
- Leave the old Bash implementation behaviorally frozen and runnable under 0318's runner.
- Add a shape-derived, mutation-tested no-new-callers guard.
- Record the direct-Go architectural transition and disposition facade-era ADRs.

## Non-goals

0369 does not delete or simplify the Bash facade, helper/runtime tree, old runner, compatibility configuration, or mechanism-only tests; introduce any forwarding shim; replace Docket commands with raw Git/GitHub/filesystem mutations; perform release/publication/rollback/four-host acceptance; implement 0367; or rewrite immutable historical records.

## Architectural decision

This change owns a new ADR establishing the PATH-resolved Go `docket` CLI as the sole maintained control plane. Maintained automation and instructions invoke public commands directly and consume documented structured output where machines interpret results. Repository-relative facade paths and helper/runtime internals cease to be maintained integration contracts even though their files remain temporarily for 0370.

The ADR must explicitly disposition:

- **ADR-0014:** superseded for maintained consumers; `DOCKET_SCRIPTS_DIR` is no longer the route to Docket behavior.
- **ADR-0029:** reversed for maintained consumers; direct public CLI invocation replaces facade routing.
- **ADR-0030:** superseded; the invariant becomes “maintained callers must not use the facade or helper/runtime internals.”
- **ADR-0033:** superseded or formally updated so Cursor trust attaches to the PATH-resolved Go executable rather than the facade.
- **ADR-0036:** preserved; committed, machine-neutral generated dispatch remains authoritative.
- **ADR-0074:** preserved; invocation migration cannot weaken gate attribution, retry, halt, waiting, or observation semantics.
- **ADR-0099:** preserved as the authoritative metadata-topology decision.

Formal supersede/reverse/update work uses the `docket-adr` workflow. Code and the ADR ledger may not describe different control planes.

## Reconciliation and inventory

Implementation begins from merged 0318 and derives a fresh whole-repository inventory by syntactic and executable shape. Halt-report counts are snapshots, not thresholds or allowlists.

Discovery covers direct and variable-composed facade calls, helper/runtime sourcing, environment-built commands, shell snippets embedded in Markdown/YAML/JSON/templates/Go strings, generator templates and products, validators, process-start-loaded assets, workflows, setup/health logic, and copyable examples.

Every site is classified as exactly one of:

1. maintained executable consumer to migrate;
2. canonical generator or template;
3. generated product;
4. active prose prescribing executable behavior;
5. frozen legacy implementation or test retained for 0370;
6. immutable point-in-time history;
7. false positive with a structural explanation; or
8. unknown/unclassified, which fails closed.

Classification follows role and behavior rather than directory alone. Markdown may be executable instruction; a test may be an active validator; a generated artifact may be loaded at process start.

## Public Go boundary

Every maintained consumer invokes `docket` by command name from `PATH` and uses the public subcommand owning the operation. Machine consumers request and validate structured output where available. They may not infer authority from human prose, parse private files, source helpers, reconstruct internal paths, duplicate lifecycle policy, introduce a new binary-location variable, or bypass Docket with lower-level repository mutation.

If a consumer requires behavior not exposed by the public Go CLI, implementation halts for reconciliation rather than recreating it in a consumer or shim. Gate commands preserve ADR-0074 exactly even though their entry path changes.

## Generators and generated products

For each generated or managed artifact:

1. identify its canonical generator/template;
2. migrate that source;
3. run the supported regeneration path;
4. review the product;
5. verify product/source correspondence; and
6. verify repeat generation is clean.

Generated products are not hand-edited as the primary source. Managed blocks retain marker balance/order validation. Output remains machine-neutral: it may assume `docket` is on `PATH`, but cannot embed a checkout, host-specific shell, or developer path.

## Process-start-loaded assets

Skills, native agent definitions, dispatch material, and similar files may be loaded only at host startup. Acceptance therefore distinguishes source correctness, generator correctness, checked-in product correctness, hermetic fresh-process behavior, and behavior of already-running external hosts.

Where possible, clean isolated subprocesses load representative assets and prove direct-Go invocation. The implementation must not claim that current Claude, Codex, Cursor, or OpenCode sessions reloaded changed assets. Full native-host reload and mutating self-host proof remain in 0366.

## Frozen legacy boundary

The facade, helper/runtime tree, old runner, compatibility seams, and mechanism tests remain present and green. They are behaviorally frozen:

- no new forwarding layer;
- no new production behavior;
- no opportunistic cleanup/refactor;
- no weakened tests;
- all applicable legacy tests still run under 0318's transitional runner.

Any necessary behavior change inside the legacy implementation is a reconciliation exception requiring explicit justification. The desired intermediate topology is direct-Go maintained consumers plus an unused, tested, frozen Bash implementation awaiting 0370.

## No-new-callers guard

Replace the old facade-wiring guard with a repository-wide guard derived from prohibited invocation shapes rather than fixed counts or file lists. It detects maintained execution/source of the facade, helper/runtime interfaces, `DOCKET_SCRIPTS_DIR`-based invocation, and equivalent composed routes.

It distinguishes prohibited consumers from the frozen legacy implementation, tests that genuinely exercise it, immutable history, and descriptive migration prose. Exclusions are narrow, role-based, and fail closed; broad directory exemptions are not acceptable.

Mutation tests inject representative violations into a skill/instruction, agent or generator, workflow/setup check, and validator/example. Alternate spellings and composed calls must be detected. Missing inputs, parse failures, unavailable discovery, or unclassifiable executable shapes are failures, not absence evidence.

## Consumer requirements

### Skills and agents

- Commands use `docket` from `PATH`.
- Machine decisions consume public JSON where applicable.
- Gate authority continues to come from public gate contracts, not child prose or exit-code inference.
- Definitions remain machine-neutral and are regenerated from canonical sources.

### Workflows, setup, and health checks

- Workflows find `docket` through `PATH` and fail clearly when unavailable.
- Setup verifies the direct-Go prerequisite instead of facade environment state.
- Health does not treat compatibility residue as an active dependency.
- Compatibility variables/config remain physically present for 0370 when deletion would broaden this stage.

### Active documentation and examples

- Current operator/contributor instructions prescribe direct public commands.
- Copyable examples are treated as executable consumers.
- Historical records retain the command true at their recorded time.
- Documentation accurately notes that the unused facade still exists until 0370 and that human native-host proof remains in 0366.

## Testing strategy

Use 0318's canonical whole-suite command from resolved repository configuration. Focused coverage includes:

- inventory classification, including fail-closed unknowns;
- PATH fixture invocation without repository-local facade paths;
- structured-output validation for machine consumers;
- deterministic generator/product correspondence and managed-marker safety;
- hermetic fresh-process loading of representative generated assets;
- mutation-tested no-new-callers violations across several maintained shapes;
- legacy freeze verification; and
- the complete configured suite plus required budget-signal review.

## Failure handling

Stop rather than silently expand or narrow if a consumer needs missing Go behavior, a canonical generator cannot be found, regeneration is nondeterministic, direct invocation would weaken ADR-0074, the guard requires a broad exemption, or keeping the legacy tree green requires behavioral modification. Such findings do not authorize a shim, duplicated policy, raw mutation, or pulling 0370 into this PR.

## Acceptance criteria

1. A new ADR establishes PATH-resolved Go as the sole maintained control plane and formally dispositions ADRs 0014, 0029, 0030, and 0033.
2. ADR-0036, ADR-0074, and ADR-0099 remain satisfied.
3. Every active executable-shaped candidate on the reconciled base is classified, with no unknown/error state.
4. Every maintained consumer invokes the public CLI directly from `PATH`.
5. Machine consumers use documented structured output where interpretation is required.
6. No maintained consumer sources, executes, or constructs a route to facade/helper internals.
7. No consumer replaces Docket lifecycle behavior with raw Git/GitHub/filesystem mutation or copied policy.
8. Canonical generators are updated before products; repeat generation is clean.
9. Generated artifacts remain deterministic, machine-neutral, and free of developer paths.
10. Representative process-start-loaded assets pass hermetic fresh-process tests without claiming live external-host reload.
11. The Bash facade, runtime/helpers, old runner, and mechanism tests remain present and behaviorally frozen.
12. No forwarding shim is introduced.
13. The frozen implementation has no maintained executable caller.
14. A shape-derived no-new-callers guard covers direct, indirect, sourced, and generated invocation.
15. Discovery, parse, and classification uncertainty fail closed.
16. Mutations prove violations in skills/instructions, agents/generators, workflows/setup, and validators/examples are caught.
17. History and the explicitly retained legacy tree do not create false positives.
18. Active docs and setup accurately describe the direct-Go intermediate topology.
19. Gate attribution, halt, waiting, observation, and retry semantics remain unchanged.
20. 0318's canonical runner executes the complete suite and retained legacy tests successfully, with no unresolved authoritative budget breach.
21. The merged state is independently green and usable without 0370.
22. No deletion, release, native-host acceptance, rollback, or 0367 work is performed.
23. A final whole-repository scan finds no unclassified executable-shaped facade/helper invocation.

## Dependencies and sequencing

0369 depends on merged 0318 and precedes 0370. Human acceptance 0366 and post-cutover board work 0367 follow 0370. These are sequential merged-main dependencies, not stacked branches.

## Assumptions

- 0318 supplies the transitional Go runner and complete legacy corpus.
- The public Go CLI already exposes the lifecycle operations and structured contracts maintained consumers need.
- `docket` on `PATH` is the supported machine-neutral availability contract.
- Canonical generators exist for generated dispatch and agent assets.
- Hermetic subprocesses can validate representative process-start-loaded output.
- Temporary presence of an unused, frozen legacy implementation does not make it a supported alternate control plane.
- Compatibility environment/configuration may remain until 0370 without being used by maintained consumers.
- The ADR workflow can record one transition decision and its supersede/reverse consequences.
- Current reference counts and path sets are not architectural invariants.
- 0366 remains sole owner of fresh real harness and public-release proof.
