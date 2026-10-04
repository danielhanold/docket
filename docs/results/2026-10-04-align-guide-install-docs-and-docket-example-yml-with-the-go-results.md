<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0464 — Align the human-facing docs and example config with the docket binary](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0464-align-guide-install-docs-and-docket-example-yml-with-the-go.md)**
<!-- docket:backlink:end -->
# Align the human-facing docs and example config with the docket binary — Results

**Human action:** Assessment pending; review fixes are in progress.

## Outcome

The living human-facing docs (README, guide, install, concepts, reference, release checklist) were rewritten to describe only what the `docket` binary does today. `.docket.example.yml` now lists only supported keys and is safe to copy verbatim. A new repo guard blocks change/PR citations and unsupported config keys in the living docs. Final consolidation pending.

## Known issues and follow-ups

### Whole-branch review findings (deep tier), pending fixes

1. important: repository-layer `agents:` pins block every write, but the example and docs say only "honoured only from the global config".
2. important: `.docket.example.yml` `review.max_fix_tasks` comment over-states batching (only minors batch).
3. minor: stale index blurbs (README harness link, docs/reference/README harness entry, docs/README Global config and config-keys entries).
4. minor: example correspondence test cannot see unsupported keys inside commented blocks.
5. minor: no test keeps the example's commented `agents` table equal to the built-in table.
6. minor: finalize-sequencer words `final-backlink-pending` like a status; it is a finding on a `done` change.
7. minor: glossary gate-run sample names the retired `run-tests.sh`.
8. minor: v0.9.11 fixture PROVENANCE.md has no concrete commit id.
