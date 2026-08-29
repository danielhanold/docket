<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0318 — Go-only source cutover](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0318-config-contraction-self-hosting-and-hard-cutover.md)**
<!-- docket:backlink:end -->

# Go-only source cutover

## Summary

Change 0318 completes Docket's source-level hard cutover to Go in one reviewable code pull request.
After this change, the `docket` executable resolved from `PATH` is the only maintained workflow and
control path. Maintained agents, skills, generated dispatch material, workflows, setup checks,
configuration, tests, and operator documentation use public Go CLI or JSON contracts directly.

The cutover removes the production Bash facade, its helper/runtime tree, compatibility launchers,
environment bridges, hidden fallbacks, and lifecycle dependencies on Python or Perl. The only
surviving shell products are repository-root POSIX `install.sh` and the POSIX release downloader.
Their genuine `/bin/sh` behavior remains tested.

This change also introduces `docket development test` as the sole whole-suite implementation. It
tests the exact source copy under review, runs Go coverage plus the two retained POSIX product
suites, and preserves the repository's isolation, completeness, interruption, and
budget-confirmation guarantees.

Change 0318 stops at an open code PR. Publication, fresh-host self-hosting, rollback rehearsal,
release evidence, and active-backlog closeout belong to change 0366, a deliberately ungroomed
human-attended successor that depends on this change.

## Goals

- Make the PATH-resolved Go binary Docket's sole maintained workflow and control implementation.
- Remove all active production dependencies on the Bash facade and legacy helper/runtime
  mechanisms.
- Preserve the behaviorally meaningful invariants previously protected by legacy tests.
- Establish one Go-native, branch-faithful whole-suite command for contributors, CI,
  finalization, and release-candidate source gating.
- Rewrite active configuration, generated assets, workflows, and documentation so they state the
  post-cutover architecture truthfully.
- Produce acceptance evidence that can be established autonomously from the source checkout before
  merge.

## Non-goals

Change 0318 does not:

- Publish, tag, or create GitHub release `v1.0.0-rc1`.
- Assert that release URLs, downloadable assets, or public clean-install paths already exist.
- Perform the whole-active-backlog migration-ledger disposition or create its manual Go learning
  records.
- Run native binary smoke tests across every Darwin/Linux and amd64/arm64 tuple.
- Perform complete mutating lifecycle, restart, or resume scenarios on genuinely fresh Claude,
  Codex, Cursor, or OpenCode hosts.
- Rehearse rollback using the frozen `v0.9.2` distribution.
- Collate immutable release evidence or close release metadata.
- Deliver stable `v1.0.0`, Homebrew distribution, Windows support, signing/notarization, SBOM or
  provenance signing, uninstall support, or version-tree garbage collection.
- Redesign Markdown/Git storage, the JSON protocol, harness topology, or Git/GitHub adapters.
- Change capabilities unrelated to removing obsolete mechanisms unless the existing capability
  cannot truthfully operate through the Go-only path.
- Rewrite historical specs, results, archived changes, Accepted ADRs, or frozen `v0.9.2` fixtures.
- Treat the existing run-halted lifecycle marker or its later removal as product scope.

All post-merge, irreversible, public-distribution, fresh-host, and human-verified work is owned by
0366. A failure there must not retroactively expand this PR's scope.

## Governing design constraints

- ADR-0099's single metadata topology remains authoritative for Go v1.
- The Go migration architecture and program map referenced by the prior design remain governing
  context where they do not conflict with this narrowed scope.
- No new ADR is required merely to execute the settled hard cutover. If implementation discovers a
  durable architectural choice rather than a mechanical consequence of retiring Bash, it records
  the decision need instead of silently establishing it.
- Inventories are derived from the reconciled branch under review. Counts and file lists in plans,
  prior specs, or halted reports are snapshots, never authority.
- Maintained-source cross-references use symbol names or verbatim quoted clauses, never line
  numbers.
- Frozen fixtures remain frozen. An active generated input that needs a changed baseline gets a
  newly versioned baseline with provenance.

## Repository-wide inventory and classification

Implementation begins with derived whole-repository inventories for legacy mechanisms and their
dependent guards. Searches key on syntactic shape and executable behavior, not on a hand-maintained
list of expected filenames or spellings.

At minimum, inventory:

- Bash entry points, sourced helpers, compatibility launchers, runtime libraries, and shell-command
  construction.
- `runtime.bash`, `DOCKET_SCRIPTS_DIR`, `DOCKET_BASH_PATH`, helper-path setup, and equivalent
  indirect runtime selection.
- Python or Perl invocations participating in Docket lifecycle behavior.
- Maintained callers that bypass the public Go CLI or approved Go adapters.
- Skills, native agent definitions, generated dispatch blocks, workflows, setup checks,
  validators, examples, configuration, and operator instructions encoding the old mechanism.
- Tests and guards coupled to anything being removed.
- Historical records and frozen fixtures that mention the old mechanism but remain immutable.

Every occurrence is classified as exactly one of:

1. **Removable production mechanism** — active code, configuration, generated material, or
   instruction implementing or selecting the legacy path.
2. **Retained POSIX distribution/bootstrap surface** — repository-root `install.sh` or the release
   downloader, including genuine `/bin/sh` behavior.
3. **Retained product-invariant test** — a test whose premise remains required after its legacy
   subject disappears.
4. **Removed implementation-detail test** — a test of obsolete Bash spelling, quoting, pipelines,
   helper protocols, or other deleted implementation details.
5. **Immutable historical evidence** — point-in-time records, Accepted ADRs, archived material, or
   frozen `v0.9.2` fixtures.

The PR records the derived inventory and disposition in reviewable form, distinguishes executable
sites from prose, and shows that every active executable site has a disposition. Searches are rerun
after implementation; the initial counts are not reused as acceptance evidence.

## Production implementation cutover

The maintained production path resolves `docket` through `PATH` and invokes documented Go commands
or JSON contracts. There is no repository-relative facade path and no environment-selected alternate
implementation.

The PR removes:

- The production Bash facade and its production helper/runtime tree.
- Compatibility launchers and shims whose purpose is to select or emulate the removed
  implementation.
- Active `runtime.bash`, `DOCKET_SCRIPTS_DIR`, `DOCKET_BASH_PATH`, and equivalent helper-path
  bridges.
- Hidden fallbacks from Go to Bash or another legacy implementation.
- Python or Perl lifecycle dependencies.
- Setup logic, validation, or examples whose only purpose is to locate the removed runtime.
- Duplicate implementations of operations now owned by the Go CLI.

The only retained shell products are repository-root POSIX `install.sh`, whose source-bootstrap and
legacy-adoption ownership was settled by 0322, and the POSIX release downloader established by
0317. They remain `/bin/sh` surfaces and may not conceal a route back to a Bash lifecycle.

Direct Git and GitHub effects remain encapsulated by approved Go adapters. Maintained agents,
skills, workflows, or instructions must not compensate for facade removal by scripting raw
repository mutations externally.

## Maintained callers and generated assets

Every maintained integration surface moves to the direct-Go model:

- Skills invoke the PATH-resolved `docket` binary.
- Native agent definitions use public Go CLI or JSON contracts.
- Generated dispatch blocks describe and execute the direct-Go path.
- Workflows use the Go command surface and canonical whole-suite command.
- Setup and health checks verify the binary and supported contracts rather than helper paths.
- Operator instructions and examples show PATH-based invocation.
- Validators reject obsolete runtime configuration rather than teaching or accepting it.

Generated files are changed through their generators. Acceptance covers generator output and drift
detection. Tests that guarded duplicated prose or generated copies move to the canonical source and
new generated contract; they do not simply disappear with the old copy.

Process-start-loaded assets cannot be validated reliably by the session that edited them. This PR
therefore proves that generators emit the direct-Go contract, checked-in generated output matches,
and locally automatable hermetic fresh-process checks pass. Genuinely fresh external harness loading
is 0366 evidence.

## Configuration contraction

Configuration cleanup is limited to obsolete mechanisms whose consumers disappear here. Remove
Bash runtime selection, executable/helper discovery, facade examples and validators, environment
bridges used only by the removed implementation, and obsolete compatibility toggles with no Go
consumer.

Preserve global model/reasoning pins, unrelated machine overrides, surviving Go capability
configuration, frozen versioned fixtures and historical examples, and compatibility data still
needed by the bounded adoption behavior of repository-root `install.sh`.

If removing a field changes an independent product capability rather than deleting a dead
mechanism, defer it unless the field makes the Go-only claim false. A required active baseline
change gets a new versioned baseline and provenance; prior-version material is not edited.

## Test disposition and invariant preservation

Production Bash and mechanism-only Bash tests leave together in this PR. Test deletion requires
assertion-level classification.

Tests leave with their subject when they verify only Bash spelling/quoting, pipeline construction,
helper sourcing/path selection, shell-specific transport, legacy runtime variables, launcher
routing, or deleted helper protocols.

Tests are retained or replaced when they protect a surviving property. The inventory must consider:

- Atomic installation and replacement.
- Managed-marker balance, order, and preservation of unmanaged content.
- Concurrent Git-write safety.
- External-effect recovery and resumability.
- Process and lifecycle state transitions.
- Ownership and metadata correctness.
- Installer and downloader integrity.
- Generated-output determinism and drift detection.
- Missing-result detection, interruption propagation, and budget confirmation in suite execution.

A surviving invariant receives mutation-sensitive Go coverage or, only for behavior owned by one of
the two retained shell products, POSIX coverage. Each guard must fail when its protected premise is
removed or violated. A green mutation is a defect.

## Canonical whole-suite command

The public contributor command is:

```text
docket development test
```

It is the sole whole-suite implementation. Contributor documentation and the release-candidate
source gate reach the same command logic. The committed `finalize.test_command` is the one source of
truth for the build gate.

### Source-copy fidelity

The suite must test the branch under review, not the globally installed binary. The committed gate
uses a branch-faithful Go entry—preferably `go run ./cmd/docket development test` from the checkout—
to build and enter the current source implementation before orchestration. The resulting runner:

- Builds or resolves the exact executable from the current checkout.
- Places that executable first in an isolated test `PATH`, or passes its explicit path internally.
- Records enough build identity to prove the tested executable matches the action target.
- Does not overwrite the user's installed binary or mutate unrelated user configuration.
- Does not depend on the legacy workflow being removed.

The source bootstrap exists only to enter the current Go implementation. It is not a compatibility
facade or second lifecycle implementation. If a direct `go run` spelling cannot satisfy a settled
runner constraint, implementation may use an equivalent Go-native source bootstrap, but
`finalize.test_command`, active documentation, and the acceptance evidence must name the one chosen
form consistently.

### Suite composition

The canonical suite runs all applicable Go packages, the retained POSIX behavioral tests for
repository-root `install.sh` and the release downloader, and the generated-artifact,
documentation/configuration, absence, and mutation guards required by this design.

It rejects Bash-dependent targets, targets requiring the deleted helper/runtime tree, legacy
compatibility runners, undeclared shell suites outside the two retained products, and configuration
that routes tests through a stale installed binary. Plain `go test ./...` is not an acceptable
whole-suite replacement because it omits POSIX product coverage and orchestration guarantees.

### Orchestration guarantees

The Go-native runner preserves the prior runner's semantic guarantees without preserving its
implementation:

- Per-target isolation for mutable filesystem, environment, repository, and process state.
- Parallel execution where targets declare it safe.
- One durable, identity-matched result for every scheduled target.
- Failure on missing, malformed, duplicated, or misattributed results.
- Prompt cancellation and correct failure reporting on interruption or termination.
- Cleanup that preserves the diagnostic record for interrupted or failed work.
- Deterministic aggregation and nonzero failure when any required target fails.
- Per-target wall-clock budget screening and serial confirmation before machine-sensitive parallel
  overages become authoritative.
- Explicit distinction between screening findings and authoritative serial breaches.

`BUDGET WATCH:` and `PARALLEL-SENSITIVE:` remain screening findings.
`SERIAL CONFIRMED OVER BUDGET:` remains an authoritative breach that must be dispositioned even if
the runner does not fail by default. Output clauses may remain stable where maintained contracts
depend on them, but legacy structure is not retained solely for textual compatibility.

## Documentation design

Active documentation states that source contributors bootstrap and test the current Go checkout;
operators invoke `docket` from `PATH`; repositories use 0352's native operations; all four harnesses
dispatch directly to native Docket agents and generated assets; repository-root `install.sh` and the
release downloader are the only POSIX products; rollback to `v0.9.2` is separate rather than an
active fallback; and the whole suite is surfaced through `docket development test` from the
committed gate.

`v1.0.0-rc1` is described as the upcoming first public Go candidate. Any public-URL example remains
future-facing or templated until publication. The bounded interval between this merge and 0366's
publication is intentional and is not presented as release failure.

Historical specs, results, archived changes, Accepted ADRs, and `v0.9.2` fixtures remain unchanged
even when their instructions no longer describe current operation.

## Failure handling and follow-up boundary

- A legacy caller, fallback, or surviving invariant needed for the Go-only claim is in scope.
- A missing Go behavior already promised by the product is in scope only to the minimum needed to
  preserve it through cutover.
- A new capability, architecture redesign, unrelated cleanup, or broader configuration contraction
  becomes follow-up work.
- Irreversible release action, public infrastructure, subjective backlog disposition, or fresh-host
  manual proof belongs to 0366.
- A durable new architecture choice is surfaced as an ADR need rather than decided silently.
- If source testing still depends on the frozen prior workflow, redesign the branch-under-review
  bootstrap. Do not modify `v0.9.2` fixtures or retain a hidden legacy runtime.

The open PR must remain independently reviewable and mergeable without successor work.

## Acceptance criteria

Change 0318 reaches its open-PR acceptance gate only when:

1. A fresh whole-repository inventory classifies all relevant active and historical occurrences
   into the five categories.
2. Every active executable site has a recorded disposition, and post-change searches find no
   undispositioned site.
3. No maintained production caller invokes the Bash facade, helper/runtime tree, compatibility
   launcher, environment bridge, hidden fallback, or Python/Perl lifecycle logic.
4. Repository-root POSIX `install.sh` and the POSIX release downloader are the only surviving shell
   products, and their tests run under `/bin/sh` without the removed runtime.
5. Maintained skills, agents, generated dispatch blocks, workflows, setup checks, validators, and
   instructions resolve `docket` from `PATH` and use public Go contracts.
6. Generated assets reproduce from canonical generators and match the direct-Go contracts.
7. Generator and hermetic fresh-process checks pass without claiming external harness reloads.
8. Active `runtime.bash`, `DOCKET_SCRIPTS_DIR`, `DOCKET_BASH_PATH`, helper-path setup, facade
   examples, and obsolete validators are absent where their consumers disappeared.
9. Global pins, unrelated machine overrides, historical records, Accepted ADRs, and frozen
   `v0.9.2` fixtures remain intact.
10. Every removed legacy assertion has a disposition, and every surviving invariant has
    mutation-sensitive Go or retained POSIX coverage.
11. Mutation checks redden when protected premises are stripped or violated.
12. `docket development test` exists as the sole whole-suite implementation and the committed
    `finalize.test_command` enters that implementation from current source.
13. The suite tests the checkout-built executable, not a stale installed binary.
14. The suite runs all Go tests and only the declared POSIX product suites, rejecting Bash and
    legacy-runtime targets.
15. Isolation, result completeness, interruption handling, deterministic aggregation, and
    screen-then-serial-confirm budgets are covered by tests.
16. The complete canonical suite passes.
17. Every authoritative `SERIAL CONFIRMED OVER BUDGET:` breach is corrected or explicitly
    dispositioned in acceptance evidence.
18. Active contributor, install, release, troubleshooting, agent, and setup documentation agrees on
    the Go-only source model and two POSIX exceptions.
19. `v1.0.0-rc1` is described as upcoming unless immutable publication evidence already exists.
20. Source/dev installation and suite execution are demonstrated hermetically without mutating the
    user's live installation or configuration.
21. The PR contains no tag, GitHub Release, public assets, fresh-host four-harness lifecycle claim,
    rollback rehearsal, backlog closeout, or other 0366 evidence.
22. The result is one reviewable code PR that can stop at the human merge gate with no post-merge
    action required to establish its source-level claims.

## Successor handoff

Change 0366 is deliberately `proposed`, unspecced, `auto_groomable: false`, and dependent on 0318.
Its detailed stub preserves the whole-active-backlog audit and manual learnings; exact once-only
candidate packaging; four native tuple smokes; complete fresh-session lifecycles through Claude,
Codex, Cursor, and OpenCode; isolated `v0.9.2` rollback; immutable tag/Release/assets/checksums;
public installation; and final evidence and metadata closeout.

Change 0318 supplies the mergeable source state that 0366 will package and verify. It does not
pre-authorize, simulate, or claim those human and post-merge outcomes.

## Assumptions

- The existing change identity, slug, recorded branch, and claim continuity remain; this spec
  narrows the deliverable without minting a replacement source-cutover change.
- Changes 0317, 0322, 0326, 0352, 0361, and 0363 are complete and their established contracts are
  present on the reconciled base.
- The public command name is `docket development test` unless implementation finds a direct naming
  conflict. Any alternate preserves the single-command contract and appears identically in the
  committed gate and active documentation.
- A small Go-native source bootstrap used only to enter the checkout's runner is acceptable and is
  not a second lifecycle implementation.
- The repository can express retained POSIX tests as an explicit finite set owned by `install.sh`
  and the release downloader.
- Existing budget vocabulary may remain stable while orchestration moves to Go.
- Internal Go runner packages and fixtures are in scope; a general-purpose task runner is not.
- Direct Git/GitHub behavior needed by cutover already exists in approved Go adapters; independent
  adapter redesign is follow-up work.
- Naming `v1.0.0-rc1` before download availability is acceptable when prose calls it upcoming.
- No fresh external harness, public release, irreversible tag, or subjective backlog-disposition
  evidence is required for this source-only PR.
