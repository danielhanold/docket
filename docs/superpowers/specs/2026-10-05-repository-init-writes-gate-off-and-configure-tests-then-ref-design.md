<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0526 — repository configure-tests takes the test command as input](../../changes/archive/2026-10-05-0526-repository-init-writes-gate-off-and-configure-tests-then-ref.md)**
<!-- docket:backlink:end -->

# configure-tests takes the test command as input — design

Change: 0526. Related: 0374 (separate build/finalize test configuration — the discovery planner and
`configure-tests`), 0352 (native repository init), 0366 (the acceptance run that hit this), 0523
(configure-tests refusing on a behind-only `.docket`, a separate fix).

## Problem

On a repository whose only test is a root `test.sh`, `docket repository init` writes
`build.gate: "off"` and `finalize.gate: "off"`. No command path then turns the gates on.

Traced cause (not the stub's original diagnosis):

- `configure-tests` does not treat `off` as a decision. It re-runs the same suite discovery as init
  (`reposetup.DiscoverTests` over `detectorRegistry`). A root `test.sh` matches no registered family
  (the shell family is `tests/test_*.sh`), so the outcome is `none`. `RenderTestConfigEdit` for `none`
  plans `gate: "off"` (preserve-explicit), the file already says that, so `changed` is false and
  `RunRepositoryConfigureTests` falls into its single not-wrote message: "the test policy is already
  configured; nothing to write". That message is false for `none` and for `ambiguous`.
- `configure-tests` accepts no input (only `--repo-dir`). Whenever discovery cannot produce the
  command, the operator's only path is a hand edit of `.docket.yml`.
- Sibling dead ends with the same cause:
  - init's ambiguous note (`testDiscoveryNote`) says "run `docket repository configure-tests` to choose
    one", but configure-tests re-discovers, gets the same ambiguity, and writes nothing.
  - `reposetup.ConfigureTestsGapNote` (one gate configured, the other `local` with no command) tells
    the operator to set the key by hand.
  - `AmbiguousTestDiscoveryError` (migrate's pre-mutation refusal on a legacy repository) names
    `docket repository configure-tests` as the follow-up, but configure-tests refuses every
    non-healthy state, including legacy — the remedy loops.

0374 deliberately chose `none → gate: "off"` so a repository without a test suite records truthful
skipped evidence instead of halting every build. That decision stands.

## Decision

### 1. `configure-tests --command <cmd>`

Add a `--command` string flag to `docket repository configure-tests`.

- With `--command`, discovery is skipped entirely (no detector probes). The operation plans an
  explicit policy: in BOTH `build:` and `finalize:`, `gate: local` and `test_command: <cmd>`.
- The explicit command **overwrites**: an existing `gate` of any value (`off`, or any other explicit
  value) becomes `local`, and an existing different `test_command` is replaced. This is unlike the
  discovery path, whose gate pair is preserve-explicit (`kvPair.preserveExplicit`) and whose
  configured short-circuit never re-plans. The explicit flag is the human's choice, so it wins; the
  result is still only a pending edit the human reviews.
- Posture is unchanged from today's configure-tests: healthy topology only (`configureTestsGuard`),
  pending unstaged `.docket.yml` edit in the primary worktree, never stages, never commits, the
  repository reported `needs-review` with the pending path.
- Idempotent: when the file already carries `gate: local` + exactly `<cmd>` under both owners, the
  result is `no-op` with a message naming the command; nothing is written.
- Input validation, refused as `invalid-input` with nothing written: an empty value, a
  whitespace-only value, and the legacy sentinel `auto` (it never survives resolution as a command —
  `isConfiguredCommand`). The command string is written as given (trimmed of surrounding whitespace)
  through the existing renderer, which owns YAML quoting.
- No per-gate flags. Divergent build/finalize commands remain a hand edit — rare, and 0374 already
  supports it in config; a second flag is YAGNI.
- Malformed existing YAML keeps today's posture: an error, file untouched.

Implementation steer (not binding on shape): extend the existing planner rather than adding a second
renderer — e.g. an explicit-command input to `TestPolicyEdit`/`RenderTestConfigEdit` that yields the
`{gate: local, test_command: cmd}` pairs with gate NOT preserve-explicit, reusing `planOwnerBlock`,
the splice/append machinery, and `ensureTestPolicyConfig`'s pending write. The request reaches the
app layer through the existing `SetupDeps`/runner seam (`repositoryConfigureTestsRunner`); the
capability catalog entry `repository.configure-tests` keeps its `local-write` effect.

### 2. Truthful no-change messages

Without `--command`, the not-wrote message in `RunRepositoryConfigureTests` is chosen by the
discovery outcome instead of one generic line:

- `none` — "no supported test suite was found; the build and finalize gates are off. Re-run with
  `--command "<cmd>"` to turn them on with your suite command."
- `ambiguous` — names each candidate family with its command (from `DetectedSuite`) and says to
  re-run with `--command` set to the one to use. Nothing is written (unchanged).
- `configured` — "the test policy is already configured" and names the resolved build and finalize
  commands. When `ConfigureTestsGapNote` applies (one gate configured, the other `local` with no
  command), its remedy names `docket repository configure-tests --command "<cmd>"` instead of a hand
  edit.
- `detected` with no change (the file already carries the detected settings) — "already configured"
  naming the command.

init's messages align: `testDiscoveryNote`'s ambiguous note names
`docket repository configure-tests --command "<cmd>"`, and init adds a `none` note saying the gates
were written `off` because no supported suite was found, with the same `--command` remedy.

The `repository check` finding `test-config-missing` and the build/finalize halt remedies keep naming
`docket repository configure-tests` — it now actually resolves every case.

### 3. Migrate's ambiguity remedy

`AmbiguousTestDiscoveryError.Error()` keeps naming the candidate families, but its remedy becomes:
set `finalize.test_command` in `.docket.yml`, commit and push it to the default branch, then re-run
`docket repository migrate` (migrate reads the committed config, and `migrateTestOutcome` preserves
an explicit `finalize.test_command` and copies it to build). It no longer names configure-tests,
which refuses a legacy repository.

### 4. Docs

Describe only current behavior (no change citations):

- `docs/reference/glossary.md` — the *Suite command / configure-tests* entry and the
  repository-setup example show `configure-tests --command "<cmd>"`, and say plain
  `configure-tests` re-runs discovery.
- `docs/guide/proving-the-build.md` and `docs/concepts/build-tiers-and-gate.md` — where the
  `configure-tests` remedy is described, note that `--command` sets both gates to `local` with the
  given command when discovery can't find the suite.
- The shipped skills (`skills/docket-build`, `skills/docket-convention`, and their embedded copies)
  keep naming `docket repository configure-tests` as the remedy; no skill edit is required unless a
  sentence there claims configure-tests only discovers.

## Out of scope

- init's `none → gate: "off"` policy (kept, per 0374).
- New detector families, including a root `test.sh` — `--command` covers non-standard layouts.
- Per-gate command flags; a `--command` flag on `init` or `migrate`.
- Gate semantics, evidence, and the `test-config-missing` finding's trigger.
- configure-tests' healthy-only guard (0523 handles the behind-only `.docket` refusal).

## Testing

- Planner/renderer unit tests (`internal/reposetup`): explicit command over (a) no file, (b)
  `gate: "off"` both owners, (c) a different configured command, (d) one owner configured and the
  other `local` with no command, (e) a divergent explicit gate on one owner — each yields `local` +
  the command under both owners; (f) already-equal file → no change. Mutation check: restoring
  preserve-explicit on the explicit path must redden (b) and (e).
- Input validation: empty, whitespace-only, and `auto` → `invalid-input`, file byte-identical.
- App-layer message tests: each discovery outcome (`none`, `ambiguous`, `configured`,
  configured-with-gap) renders its own message; a test fails if `none` or `ambiguous` render the
  "already configured" text. The ambiguous message lists every candidate's command.
- init: the `none` and `ambiguous` notes name `--command`.
- Migrate: the ambiguity refusal names re-running `docket repository migrate` and does not name
  configure-tests.
- Integration (the 0366 fixture shape): a fresh repository whose only test is an executable root
  `test.sh` → `repository init` (gates `off`) → commit → `configure-tests --command "sh ./test.sh"` →
  pending edit carries both local gates and the command → commit → `repository check` reports no
  `test-config-missing` finding and the resolved config shows both commands. A repeat
  `--command` run is a no-op.
- CLI: `--command` appears in `configure-tests --help`; the capability catalog entry is unchanged in
  id and effect.
