<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0374 — Separate build and finalize test configuration](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-01-0374-separate-build-and-finalize-test-configuration.md)**
<!-- docket:backlink:end -->

# Separate build and finalize test configuration — design

## Problem

`docket-build` and `docket-finalize-change` currently share `finalize.test_command`. That coupling
was deliberate in ADR-0063, but it makes two different workflow decisions indistinguishable:

- the build role needs to decide whether and how to certify a completed implementation before
  review and PR publication;
- finalize needs to decide whether and how to re-certify a rebased PR before merge.

The shared key leaves build with no supported opt-out. A repository with no tests can set
`finalize.gate: off`, but build still requires a command. Its only practical workaround is
`finalize.test_command: "true"`, which falsely records a passing test command and also changes
finalize policy.

The `auto` spelling makes the behavior harder to understand. The current Go resolver turns
`finalize.test_command: auto` into an empty command. The current Go gate then halts because no
command resolved; it performs no discovery. The `docket-build` skill still refers to a removed
Bash-era fallback that looped over `tests/test_*.sh`. Repository initialization and migration do
not currently discover or generate test configuration.

This change separates the two runtime policies, removes runtime `auto`, and moves any inference to
the attended repository-setup boundary where the generated configuration is reviewed before it
becomes authoritative.

## Decision

### Independent runtime configuration

Build and finalize each own a gate and a command:

```yaml
build:
  gate: local                 # local | off
  test_command: "go test ./..."

finalize:
  gate: local                 # existing finalize gate vocabulary remains otherwise unchanged
  test_command: "go test ./..."
```

`build.gate` is a new supported leaf. `local` runs the resolved build command once after all plan
tasks; `off` is the explicit declaration that this repository requires no build test gate. A build
command is required only when `build.gate` is `local`.

`finalize.gate` keeps its existing meaning and ownership. A finalize command is required whenever
the selected finalize mode has a local leg and is irrelevant when the gate is `off`.

`build.test_command` and `finalize.test_command` resolve independently through the ordinary config
layers. Neither falls back to the other. The initial generated values may be identical, but they
remain separate settings and may diverge later.

Both gates default internally to `local`; both commands default to the empty, unconfigured state.
That combination is valid input for repository setup but unsupported for a runtime local gate.
The canonical example spells the empty commands as quoted empty strings, not `auto`.

### Remove `auto` as runtime behavior

`auto` is removed from the public runtime contract for both commands. Missing commands and legacy
`auto` values are represented internally as unconfigured setup state, not as permission to guess
during a build or finalize run.

The parser may recognize the literal `auto` only as legacy migration input so setup can produce an
actionable upgrade. It is never a valid resolved gate command. Operational commands that require a
local gate fail before launch with a typed configuration disposition and point to the repository
test-configuration operation. They never manufacture a red suite or enter a repair ladder.

Configuration resolution itself must remain available to repository setup even when the test
leaves are absent or legacy-shaped. Otherwise the operation responsible for repairing the state
could not run.

### Build-gate evidence

A local build gate that exits zero produces the existing exact-head green evidence. An explicitly
disabled build gate produces truthful skipped evidence instead of executing `true`:

```text
<!-- docket:build-evidence:start -->
result:   skipped
reason:   build-gate-off
head_sha: <40-char SHA>
ran_at:   <UTC ISO-8601>
<!-- docket:build-evidence:end -->
```

The exact field encoding may follow the evidence package's established codec, but these semantics
are fixed:

- `green` means a configured build command ran successfully at `head_sha`;
- `skipped` means the repository explicitly set `build.gate: off`;
- an unconfigured command, launch failure, unfinished drive, or ambiguous verdict produces no
  certifying evidence.

`docket-implement-next` may proceed to review and PR publication with exact-head `green` or
`skipped` build evidence. The PR body remains truthful about which occurred.

Finalize may reuse build evidence only when all existing exact-head conditions hold **and** the
evidence is `green` **and** the recorded build command is byte-equal to the currently resolved
`finalize.test_command`. Differing commands are different assertions, even at the same SHA. Skipped
build evidence never waives finalize's local gate.

### Gate ownership

The native gate driver remains the common execution mechanism, but command selection becomes an
explicit domain boundary:

- build-drive construction reads only `build.gate` and `build.test_command`;
- finalize-drive construction reads only `finalize.gate` and `finalize.test_command`;
- persisted provenance names the exact owning config path;
- no agent or CLI caller may substitute an arbitrary command around authoritative configuration.

The build evidence operation validates the passed drive's recorded command and provenance against
build configuration. It no longer re-resolves `finalize.test_command`. Finalize's in-process gate
continues to validate against finalize configuration.

## Repository test discovery

### One shared pure planner

Initialization, migration, and upgrades use one deterministic, side-effect-free planner. Given a
pinned repository tree plus already-declared test settings, it returns:

- `configured` — preserve explicit valid settings;
- `detected` — exactly one supported suite command was found;
- `none` — no supported executable test suite was found;
- `ambiguous` — multiple plausible commands were found and no safe choice exists.

The detector registry owns the supported repository shapes and commands in one place. The initial
registry covers exactly these families: a `Makefile`/`makefile` test target; a Go module containing
`*_test.go`; a `package.json` with a non-placeholder `scripts.test` and one unambiguous recognized
package-manager launcher; an explicit pytest configuration plus Python test files; a Rust crate;
and the established `tests/test_*.sh` shell-suite shape. Each detector owns its exact command and
the executable shape that makes the command valid. If more than one family matches, the result is
`ambiguous`; no priority list guesses which command is the repository's complete suite. Each
detector reports its evidence so setup previews can explain the choice.

Discovery is a setup-time generator, not a runtime fallback. Adding or changing a repository's
tests later requires an explicit configuration edit or a re-run of the configuration operation.

### Fresh repository initialization

`docket repository init` runs discovery against the same pinned, clean repository state used for
the rest of initialization. It prepares `.docket.yml` as another pending primary-worktree path:

- one detected suite: write separate local gates and explicit commands for build and finalize;
- no detected suite: write `build.gate: "off"` and `finalize.gate: "off"`, with no fake command;
- ambiguous suites: make no test-policy write, return the candidates, and stop for a human choice.

Like the existing `.gitignore` and authorized harness surfaces, generated configuration is left
uncommitted and unstaged. The repository remains `needs-review` until the human reviews and commits
the pending paths.

### Legacy repository migration

`docket repository migrate` incorporates the same planner into its existing pinned, two-pass
preview/authorization transaction:

- an explicit legacy `finalize.test_command` is preserved and copied into the missing
  `build.test_command`, retaining behavior while separating ownership;
- missing or legacy-`auto` commands are discovered;
- one detected suite generates explicit independent settings;
- no detected suite proposes both gates as the quoted scalar `"off"`;
- ambiguity stops before any remote mutation and reports the candidates and exact remedy;
- already-explicit new-style settings are preserved.

Every generated change appears in the migration preview. The authorized migration commits exactly
the reviewed config bytes along with its existing migration transaction.

### Already-initialized repository upgrade

Repositories already on the Docket metadata topology do not pass through init, and the current
migration command correctly treats their topology as already migrated. A new
`docket repository configure-tests` operation covers this upgrade case.

It uses the same planner and prepares a pending, unstaged `.docket.yml` edit in the primary
worktree. It never commits configuration. `docket repository check` detects missing commands and
legacy `auto` values, reports a closed test-configuration finding, and names
`docket repository configure-tests` as the remedy. An ambiguous result returns candidates and
leaves the file untouched.

The operation is idempotent: valid explicit settings are a no-op; a repeated run over its own
pending bytes produces no further change.

## Compatibility and rollout

This is an intentional configuration migration, not a silent fallback:

- new repositories receive explicit reviewed settings from init;
- legacy topology migrations receive explicit previewed settings from migrate;
- already-initialized repositories receive a health finding and a deterministic upgrade command;
- runtime workflows refuse missing or legacy test policy with the exact remedy;
- no installer or background status pass edits repository configuration.

Docket's own committed `.docket.yml` will carry both explicit commands, initially set to
`go run ./cmd/docket development test`. Its build and finalize policies can then evolve
independently.

ADR-0063 must be superseded by a new ADR that carries forward its unaffected build-role decisions
and replaces Decision 5's shared-command rule. ADR-0074's tri-state verdict rule remains in force:
configuration gaps, launch failures, and unfinished runs are halts, not red suites.

The change depends on change 0370 so it designs against the retained Go control plane rather than
adding another legacy Bash configuration surface that 0370 immediately deletes. Reconcile must
confirm 0370's landed source shape before implementation.

## Touch points

Implementation must derive the exact site inventory from the post-0370 tree and classify each
executable consumer versus historical prose. At minimum, the following maintained surfaces are in
scope:

1. **Configuration model and presentation** — schema registry, effective config structures,
   resolution/provenance, config inspection output, `.docket.example.yml`, repository config
   fixtures, capability classification, and layer-resolution tests.
2. **Repository setup** — init planning/execution, migration planning/preview/execution, health
   checks, the new configure-tests operation and CLI, pinned-tree detector fixtures, pending-path
   reporting, and idempotency/concurrency tests.
3. **Workflow contexts** — implementation context exposes build gate/command; finalize context
   exposes finalize gate/command. Generic `test_command` fields that obscure ownership are retired.
4. **Gate services** — build and finalize constructors, CLI routing, config provenance, command
   validation, typed no-command dispositions, and process-launch tests.
5. **Evidence** — record, codec, verification, PR publication, review preconditions,
   implement-next re-drive, and finalize's exact-head reuse predicate.
6. **Workflow skills and bundled assets** — `docket-build`, `docket-implement-next`,
   `docket-finalize-change`, `docket-convention`, embedded skill copies, and generated harness
   artifacts whose source text mentions the shared command or removed auto-detection.
7. **Maintained documentation and self-hosting policy** — README, AGENTS.md, test documentation,
   the canonical example, and comments/guards that currently call `finalize.test_command` the one
   whole-suite source. Build-gate guidance moves to `build.test_command`. The release-candidate
   workflow may remain deliberately bound to finalize policy, but its documentation must state
   that choice rather than claiming the key is universally shared.
8. **Regression and drift guards** — tests that currently require build to read
   `FINALIZE_TEST_COMMAND`, forbid `BUILD_TEST_COMMAND`, or assert the `auto` sentinel/masking
   matrix must be replaced with mutation-sensitive assertions for independent ownership.

Historical change records, accepted ADR bodies, frozen plans/results, versioned legacy fixtures,
and archived specs remain unchanged except through their established generated or supersession
mechanisms.

## Error handling

- A local gate with no explicit command is `unsupported-config`/halt with the setup remedy, never a
  test failure.
- `gate: off` is successful policy resolution and produces truthful skipped build evidence; it is
  not represented by a shell command.
- Ambiguous discovery performs no config write or migration mutation.
- Probe errors are unknown, never clean absence and never `gate: off`.
- A command that launches and exits nonzero remains a red suite according to the runner's verdict
  contract; a launch/observation failure remains a halt.
- Setup edits remain pinned to the exact tree that discovery inspected and follow each operation's
  existing contention and human-review posture.

## Testing

The implementation uses TDD and mutation-tests each guard. Required coverage includes:

- independent layer resolution and provenance for both gate/command pairs;
- legacy `auto`, missing, explicit command, and explicit off matrices;
- detector outcomes for one suite, no suite, multiple suites, and probe failure;
- init pending config generation without commit or staging;
- migration preview fidelity, exact authorized bytes, preservation/copy rules, and ambiguity with
  zero remote writes;
- configure-tests idempotency and repository-check remedies;
- build drive can read only build configuration and finalize drive can read only finalize
  configuration, proven by divergent-command fixtures;
- build-off skipped evidence accepted by implement-next but rejected as a finalize permit;
- green build evidence reused by finalize only when head and command both match;
- unconfigured and launch-failure paths cannot enter repair;
- embedded-asset and maintained-document drift guards; and
- the complete repository suite at the build gate.

## Out of scope

- Inferring a command during an ordinary build or finalize run.
- Silently changing configuration during install, status, or autonomous implementation.
- Designing a universal test orchestrator or combining every test framework into one command.
- Changing focused tests selected by individual build workers.
- Changing CI-only finalize semantics, approval policy, merge authorization, or repair bounds.
- Rewriting historical records to use the new key names.
