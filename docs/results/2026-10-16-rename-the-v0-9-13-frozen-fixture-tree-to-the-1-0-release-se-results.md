<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0548 — Name test fixture folders by what they hold](../changes/active/0548-rename-the-v0-9-13-frozen-fixture-tree-to-the-1-0-release-se.md)**
<!-- docket:backlink:end -->

# Name test fixture folders by what they hold — results

**Human action:** None required. The change only moves test samples, rewrites test inputs, and adds a test guard; the full suite is green at the PR head.

## Outcome

`testdata/repositories/` now holds exactly `configs/`, `documents/`, `status-corpus/`, and `.gitignore`. The fake-version folders `v0.9.4` through `v0.9.13` and every byte copy of a shipped file are gone.

- The agent-defaults parity test (now `TestBuiltinAgentsParityWithShippedDefaults`) reads `agents/harness-defaults.yml` directly.
- `TestFixtureDocketSelf` reads docket's own `.docket.yml` at test time (only that file, so a developer's `.docket.local.yml` cannot reach it). `TestFixtureDeferredActive` builds its configuration from an inline constant, `deferredActiveYAML`, and names the missing path and the constant when a requestable deferred capability is absent.
- Each sample folder has one `PROVENANCE.md`; the document and status-corpus loaders skip it.
- `testdata/README.md` states the new rule: a sample is never edited to make a test pass, and is removed only when docket deliberately drops support for it.
- A new guard, `TestSampleFoldersNamedForWhatTheyHold`, fails on any folder under `testdata/repositories/` whose name starts like a version or a date (`^v?[0-9]`).
- On the metadata branch, two learnings that taught the old "cut a new versioned tree" protocol were rewritten: `config-edit-trips-its-own-frozen-drift-guard` (as the spec asked) and `frozen-copy-needs-a-drift-assert` (found by review).

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green through the gate driver at the final head.
- Every moved sample is an `R100` rename (40 files); the only added files are the three `PROVENANCE.md` files. `ls -A testdata/repositories` shows `.gitignore configs documents status-corpus`.
- The spec §5 grep (`repositories/v0.`, `sidecarPath`, "frozen sidecar", `FrozenSidecar`, "versioned tree", "versioned fixture tree", `assertFrozenCopyMatchesLive`) is clean outside the excluded frozen records, `internal/install/`, and the bash-upgrade cases.
- Throwaway probes run by the build workers, each reverted afterward:
  - changing one model in `agents/harness-defaults.yml` failed the parity test with the "must change together" message;
  - changing `finalize.test_command` in `.docket.yml` failed `TestFixtureDocketSelf` with the re-derive message;
  - a root `.docket.local.yml` requesting a deferred capability left `TestFixtureDocketSelf` passing;
  - deleting `auto_groom: true` from `deferredActiveYAML` failed `TestFixtureDeferredActive`, naming the path and the constant;
  - an empty `testdata/repositories/v0.9.14/` failed the folder-name guard, and a guard function forced to always pass failed all four reject cases.
- Review ran at the deep tier (the build used the standard tier; the 2,088-line diff, mostly deletions, bumped it one step). It returned two findings, both fixed: the second learning above, and a retired `finalize.skip_results_only_delta` key removed from `deferredActiveYAML`.
- The suite's budget report printed `BUDGET WATCH:` and `PARALLEL-SENSITIVE:` screening lines for integration tests this change does not touch, and no `SERIAL CONFIRMED OVER BUDGET:` line. These are timing notes from a loaded machine, not failures.
