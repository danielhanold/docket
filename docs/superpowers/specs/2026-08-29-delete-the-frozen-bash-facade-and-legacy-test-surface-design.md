<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0370 — Delete the frozen Bash facade and legacy test surface](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0370-delete-the-frozen-bash-facade-and-legacy-test-surface.md)**
<!-- docket:backlink:end -->

# Delete the Frozen Bash Facade and Legacy Test Surface

## Summary

Change 0370 completes the hard cutover after 0369 has migrated every maintained executable consumer to the PATH-resolved Go CLI. It removes the frozen Bash control plane, runtime and compatibility machinery, and tests whose only subject is that deleted implementation.

Deletion is proven through fresh, shape-derived reconciliation rather than familiar spellings or snapshot counts. Every substantive legacy assertion is classified before removal. Surviving product behavior moves to mutation-sensitive Go coverage, except behavior owned by the two remaining shell products: repository-root `install.sh` and the release downloader.

Finally, 0318's transitional `docket development test` runner contracts from the Go-plus-legacy corpus to the final topology: Go tests plus the declared POSIX suites for those two shell products, without weakening its execution or ADR-0074 gate semantics.

## Goals

- Prove no maintained executable consumer depends on the frozen architecture.
- Delete the production facade, runtime/helpers, compatibility launchers, and legacy runner.
- Remove mechanism-only tests while preserving every surviving product invariant.
- Contract the canonical runner to the final supported test topology.
- Add durable, mutation-tested guards against reintroduction.
- Regenerate affected products from canonical generators.
- Complete facade-era ADR status/index consequences that require physical deletion.

## Non-goals

0370 does not redo consumer migration; introduce a new shim or shell control plane; redesign unrelated configuration; publish/tag/release; conduct fresh-host self-hosting or v0.9.2 rollback; edit frozen release artifacts; rewrite archived changes, historical specs/results, or Accepted ADR history; or weaken runner and gate guarantees.

## Architectural end state

After 0370:

- the Go CLI is the sole maintained Docket control plane;
- maintained consumers use the direct PATH-resolved architecture from 0369;
- `scripts/docket.sh`, its production helper/runtime tree, and `scripts/run-tests.sh` are absent;
- compatibility launchers and environment setup used to locate/load the facade are absent;
- `DOCKET_SCRIPTS_DIR`, `DOCKET_BASH_PATH`, `runtime.bash`, and equivalent concepts are absent from active maintained execution;
- repository-root `install.sh` and the release downloader are the only supported shell products;
- `docket development test` runs Go tests plus their two explicit POSIX product suites;
- ADR-0036 machine neutrality, ADR-0074 gate semantics, and ADR-0099 topology remain true; and
- historical records may still name the retired system because they preserve point-in-time truth.

## Reconciliation against merged 0369

Implementation begins only from a base containing merged 0369 and derives candidate sites by syntactic/behavioral shape. Seeds such as `DOCKET_SCRIPTS_DIR`, `DOCKET_BASH_PATH`, `runtime.bash`, `scripts/docket.sh`, and `scripts/run-tests.sh` are useful but not proof of completeness.

Discovery covers direct execution/sourcing, shared runtime imports, environment-variable construction and forwarding, wrapper functions and command arrays, compatibility launchers, test helpers, generator/product output, workflow/setup behavior, and active operator/agent instructions.

Each candidate is classified as:

1. active maintained executable consumer, which blocks deletion;
2. active maintained test dependency, which enters assertion classification;
3. canonical generator;
4. generated product;
5. active maintained prose;
6. immutable historical record;
7. frozen release artifact;
8. false positive with a structural explanation; or
9. unknown/unclassified, which blocks deletion.

Failed probes, incomplete traversal, inaccessible inputs, parse ambiguity, and classification errors are not absence evidence. Counts and path lists remain review context, never architectural gates.

## Deletion surface

After reconciliation proves no maintained caller remains, delete production and compatibility mechanisms whose responsibility is the Bash control plane, including:

- `scripts/docket.sh`;
- its helper/runtime tree;
- `scripts/run-tests.sh`;
- compatibility launchers for the facade or runner;
- legacy runtime-loading and script-location setup;
- active configuration/environment plumbing for `DOCKET_SCRIPTS_DIR`, `DOCKET_BASH_PATH`, `runtime.bash`, and equivalents;
- harness utilities used only by the deleted implementation;
- generated products that encode the retired route, after generator correction; and
- active setup/operator material whose sole purpose is exposing the retired route.

Exact paths are derived from the merged tree. Mixed-responsibility files are split or rewritten so surviving responsibilities move to Go or the retained POSIX owner. Deletion never crosses into immutable history or frozen artifacts merely because their text matches.

## Assertion-level test classification

Test files are not disposed by filename. Every substantive assertion is classified:

### Surviving product invariant

Behavior still promised by Go or the repository—lifecycle state, configuration meaning, generated-content contracts, atomicity, safety, recovery, or public command behavior—moves to Go coverage. Relocation is complete only when a targeted mutation makes the replacement test fail.

### Installer invariant

Behavior specifically owned by repository-root `install.sh` remains or is rewritten in its POSIX suite. Shared helpers may remain only when they serve this product without reconstructing the deleted runtime.

### Release-downloader invariant

Behavior specifically owned by the release downloader remains or is rewritten in its POSIX suite under the same boundary.

### Deleted implementation mechanism

Assertions about helper routing, runtime sourcing, facade dispatch internals, legacy environment propagation, compatibility wrappers, Bash quoting, or other behavior with no surviving product meaning are deleted with their subject.

### Mixed or uncertain

Mixed assertions are decomposed before disposition. Uncertain product significance blocks removal. Classification may live in plan/build evidence, but the resulting mapping must remain auditable.

## Final runner topology

Contract `docket development test` to discover and execute:

1. all required Go targets;
2. the declared POSIX product suite for repository-root `install.sh`; and
3. the declared POSIX product suite for the release downloader.

There is no generic Bash/facade category and no dormant compatibility execution branch. The two POSIX categories are explicit product owners; their individual targets are derived rather than pinned to a snapshot count.

The contraction preserves:

- source-copy fidelity;
- per-target isolation;
- fail-closed target/result completeness;
- safe bounded parallelism;
- explicit non-success on interruption;
- deterministic multi-failure aggregation;
- budget screening followed by serial confirmation;
- ADR-0074's distinct success, observation, authoritative failure, and operational uncertainty; and
- fail-closed suite discovery.

Tests cover both removal of the legacy category and retention of each guarantee. Dormant legacy paths are defects even when unreachable in the current suite.

## Final absence guards

Guards derive executable candidates by shape and ownership rather than fixed strings, counts, or file lists. They inspect generators and products; reject direct execution, indirect delegation, sourcing, environment-mediated resolution, and wrapper reconstruction; reject dependencies on retired runtime/config concepts; fail on unknowns; and provide actionable diagnostics.

Exclusions are categorical and location/ownership-aware, not a growing per-file allowlist. Immutable history and frozen v0.9.2 artifacts remain permitted.

Mutation evidence introduces:

- direct facade invocation;
- variable-composed invocation;
- runtime sourcing;
- retired environment dependence;
- a generator-emitted forbidden command;
- an unclassified candidate; and
- a forbidden candidate in active material.

Each must redden. Companion tests prove legitimate history and frozen artifacts remain accepted.

## Generated products

Update canonical generators before products, use normal deterministic regeneration, and prove repeat generation is clean. Generated dispatch remains machine-neutral under ADR-0036 and cannot substitute a checkout-specific or host-specific path for the deleted facade.

## ADR treatment

0369 owns the direct-Go decision. 0370 consumes it and records that its physical consequences are complete. Reconcile ADRs 0014, 0029, 0030, and 0033 through the formal ADR workflow. If 0369 already changed status, verify the promised deletion condition; if a status could not truthfully change until deletion, complete it now. Accepted records are not silently rewritten. ADRs 0036, 0074, and 0099 remain authoritative.

## Implementation gates

1. **Reconcile and classify:** confirm merged 0369, inventory every candidate, and stop on maintained consumers or unknowns.
2. **Build replacement coverage:** classify assertions, add Go/retained-POSIX coverage, and prove mutations before deleting old tests.
3. **Contract the runner:** remove the legacy corpus category while preserving all runner guarantees.
4. **Update generators and active integration material:** change canonical sources, regenerate, and verify determinism.
5. **Delete:** remove the facade/runtime/runner/config and mechanism-only tests, then repeat discovery.
6. **Install final guards:** mutation-test forbidden shapes and permitted historical categories.
7. **Complete ADR consequences and verify:** render ADR/index artifacts, run deterministic generators and the canonical suite, inspect budget signals, and prove final absence.

These are build gates inside one reviewable change, not permission for partial merge points.

## Failure and recovery

A missed maintained consumer stops deletion. A small migration correction consistent with 0369 may be reconciled; a material new migration design requires regrooming. An unclassifiable assertion remains until understood. Runner contraction that loses targets, weakens isolation, changes gate semantics, or converts uncertainty into success is incomplete. Generated output is not committed until its generator is reproducible. A failed absence proof cannot be replaced by a manual belief statement.

## Verification strategy

- Base evidence proves 0369 is merged and all candidates are classified.
- Behavior evidence maps every surviving assertion to mutation-sensitive Go or one of the two POSIX suites.
- Runner evidence proves final topology and preservation of missing-result, interruption, aggregation, budget, and tri-state behavior.
- Deletion evidence proves no active source, generator, setup, or configuration can recreate the retired control plane.
- Guard evidence proves forbidden direct/indirect/generated/environmental shapes fail and immutable history remains allowed.
- Generation/ADR evidence proves deterministic products and truthful ledger/index state.
- Whole-suite evidence proves the final corpus through `docket development test` with no unresolved authoritative serial breach.

## Acceptance criteria

1. The base contains merged 0369.
2. Shape-derived reconciliation classifies all candidates with no unknown/error state or maintained consumer.
3. `scripts/docket.sh`, its production runtime/helpers, and `scripts/run-tests.sh` are removed.
4. Facade/runner compatibility launchers and active runtime-loading/script-location setup are removed.
5. Active `DOCKET_SCRIPTS_DIR`, `DOCKET_BASH_PATH`, `runtime.bash`, and equivalent environment/configuration machinery are removed.
6. Mixed-responsibility files retain their surviving behavior without dormant facade routes.
7. Every substantive removed-test assertion has an auditable individual classification.
8. Surviving Go product invariants have mutation-sensitive Go coverage.
9. Installer and downloader invariants have retained POSIX coverage owned by their respective products.
10. Mechanism-only tests are removed; unresolved mixed/uncertain assertions are not.
11. `docket development test` executes all Go targets plus exactly the two declared POSIX product categories.
12. No dormant generic legacy-facade category remains.
13. Source fidelity, isolation, completeness, interruption, deterministic aggregation, safe concurrency, budgets, and ADR-0074 semantics remain covered.
14. Discovery/result uncertainty fails closed.
15. Generators are corrected before products and repeat generation is clean.
16. Generated dispatch remains machine-neutral.
17. Final guards derive sites by shape, distinguish active material from immutable history, and fail on errors/unknowns.
18. Mutations prove direct, indirect, sourced, environmental, generated, and unclassified reintroductions are rejected.
19. Guard tests prove historical records and frozen v0.9.2 artifacts remain permitted and unchanged.
20. Facade-era ADR status/index consequences are completed through the ADR workflow without falsifying Accepted history.
21. ADR-0036, ADR-0074, and ADR-0099 remain satisfied.
22. The canonical whole suite passes over the final topology with no unresolved authoritative budget breach.
23. No release, tag, asset, fresh-host proof, rollback, replacement shim, or new shell control plane is introduced.

## Risks and mitigations

- **Hidden consumer:** search execution/dependency shape, classify all results, and fail closed.
- **Lost product coverage:** classify assertions individually and require red-on-mutation replacements.
- **Dormant runner coupling:** remove the category structurally and test absent/unknown targets.
- **Brittle guard:** use shape and ownership categories rather than snapshot inventory.
- **Historical falsification:** explicitly classify and test immutable/frozen exclusions.
- **Renamed compatibility layer:** forbid indirect delegation and runtime reconstruction, not only old names.
- **Moving base:** reconcile only against merged 0369 and repeat discovery after deletion.

## Assumptions

- 0369 merges first and establishes the authoritative direct-Go ADR.
- 0369 leaves the facade frozen, unused, and still tested.
- 0318 already defines the runner guarantees 0370 must preserve.
- Exact helper/runtime paths may change, so responsibility and dependency shape govern the inventory.
- Repository-root `install.sh` and the release downloader are the only surviving shell products.
- Shared POSIX helpers may remain only if they serve those products without reconstructing the retired runtime.
- Historical records, Accepted ADRs, and frozen v0.9.2 artifacts are immutable for this change.
- Active current-operation documentation may be corrected.
- A small missed consumer within 0369's settled architecture may be reconciled; material redesign requires regrooming.
- No transient count, current filename set, or line number is authoritative.
