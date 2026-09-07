<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0406 — Flaky TestIntegrationReleasePackageDeterministic — linux_arm64 bundle nondeterminism reddens the suite](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-07-0406-flaky-testintegrationreleasepackagedeterministic-linux-arm64.md)**
<!-- docket:backlink:end -->

# Deterministic release bundles — change 0406

## Approved scope and timing

The human approved this scope on 2026-09-06. Reproduction under isolated and suite-load conditions, root-cause diagnosis, and selection of the repair happen during implementation. Grooming establishes the investigation and acceptance contract; it does not claim to have reproduced or diagnosed the defect.

## Problem and objective

During change 0403's build gate, TestIntegrationReleasePackageDeterministic reported different checksums for the Linux ARM64 archive between two builds of the same source. The other three target archives and install.sh matched. Three later serial reruns passed, so those passes do not establish a repair.

For identical source, declared packaging inputs, and toolchain, repeated packaging must produce byte-identical release bundles. Preserve strict checksum comparison for Darwin/Linux on amd64/arm64 and the installer.

## Current implementation and uncertainties

Package in internal/release/package.go builds every tuple through the same loop with CGO disabled, -trimpath, fixed injected build identity, and cleared GOFLAGS. WriteArchive in internal/release/archive.go writes exactly one USTAR member and explicitly fixes tar and gzip timestamps, ownership, mode, and gzip OS identity.

Therefore archive timestamp leakage and file ordering are hypotheses, not established causes. The difference may originate in the compiled binary, packaging, or changing effective build inputs. Do not prescribe an ARM64-specific workaround from the symptom alone.

TestIntegrationReleasePackageDeterministic in internal/release/package_integration_test.go compares checksums.txt after two Package calls. Its current failure output identifies manifest differences without locating the differing bytes.

## Investigation and repair

1. Reconcile against current source and reproduce on the unmodified base using repeated uncached executions, both in isolation and through the configured whole-suite runner under its normal parallel load. Use separate output directories and record source revision, declared packaging inputs, toolchain identity, relevant build settings, cache conditions, commands, and outcomes. Bound the experiment and record its actual extent; do not retry indefinitely until green.
2. On a mismatch, capture enough evidence before temporary outputs disappear to distinguish compressed archive differences, gzip header differences, decompressed tar stream/header differences, and differences in the embedded docket binary. Report affected tuple, hashes and sizes, and useful byte offsets or structural differences. If binaries differ, inspect their build identity and relevant binary metadata to distinguish varying inputs from nondeterministic output. Do not dump entire binaries or the full ambient environment into logs.
3. Fix the demonstrated cause at the earliest responsible release build or packaging boundary. Keep any change to shared behavior limited to what the evidence requires, and verify all four tuples. Preserve archive format, member contract, bundle naming, installer behavior, and injected release identity.
4. Improve mismatch diagnostics so a future failure names the affected artifact and differing layer while still failing the strict comparison. Prefer failure-only analysis using the existing two builds; do not multiply expensive release builds in the permanent test unnecessarily.

If bounded investigation cannot reproduce the defect or establish its cause, document that limitation explicitly in the results and PR. Passing reruns or diagnostics alone are not a demonstrated repair. Do not introduce a speculative production fix or claim that the flake is fixed; return the unresolved cause for follow-up.

## Regression and acceptance evidence

- Add a focused regression test for the demonstrated mechanism. Prove that removing or reverting the repair makes that test fail for the intended reason, and that restoring the repair makes it pass. Defeat Go's result cache for these observations.
- Exercise mismatch diagnostics with controlled differences so gzip/tar metadata changes can be distinguished from payload changes and strict comparison remains red. Use small fixtures where possible.
- Preserve the existing end-to-end comparison across all four target archives and install.sh without retries that mask failures, relaxed comparison, quarantine, or new skipping.
- Repeat the diagnosed reproduction conditions after the repair and record the counts and outcomes, including both isolated execution and suite-load execution. Finite passing repetitions support the mechanism-based regression; they are not proof by themselves.
- Run the complete build gate from the resolved build.test_command configuration. At grooming time it is go run ./cmd/docket development test. Finalize independently resolves finalize.test_command. Follow tests/README.md and act on budget findings under the repository's standing rules.
- The results must connect the original mismatch, observed differing layer, demonstrated cause, minimal repair, regression evidence, and whole-suite outcome. A non-reproduction report must be plainly distinguishable from repair evidence.

## Scope boundaries and relationships

Likely implementation owners are internal/release/package.go, internal/release/archive.go, and the corresponding release integration tests; change only the owners the diagnosis establishes. No suite-runner or gate redesign, general flake tolerance, unrelated refactoring, release publication, or change to 0403's config-diagnostics work is included.

Change 0317 introduced release packaging and is done. Change 0366 covers release acceptance and remains separate. Change 0403 is the discovery source. These are context links, not blocking dependencies; change 0406 has no depends_on entries and does not stack on another branch.

## Alternatives considered

A speculative timestamp or ordering fix is rejected because the current writer already pins those properties. Weakening or retrying away the checksum assertion is rejected because it conceals differing release bytes. Evidence-led diagnosis followed by a focused source repair preserves the determinism contract while avoiding a cause inferred solely from one tuple's name.
