<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0473 — Rename build profile and review rung to tiers, and dispatch tiers to dispatch fallbacks](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0473-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t.md)**
<!-- docket:backlink:end -->
# Rename build profile and review rung to tiers, and dispatch tiers to dispatch fallbacks — Results

**Human action:** No action is required before merging, but plan the merge for a quiet moment. Merge only when no docket repository has a change mid-build or a plan that is committed but not yet built. After merging, you can optionally re-run `docket install` in consumer repos so the generated agent descriptions show the new wording.

## Outcome

Docket used three names for how strong a worker is: "profile", "rung" and "tier". It also named its fallbacks for an unavailable agent dispatch with the letters "Tier A / B / C" plus "the carve-out". This change applies ADR-0129 family (c). The names are now:

- **build tier**: economy, standard, premium or max.
- **review tier**: lean, standard or deep.
- **dispatch fallbacks**, one per kind of dispatch: `inline`, `abstain`, `auto-or-halt` and `no-fallback`.

Nothing changes in meaning. No agent names, config keys, tier names or frontmatter fields change.

Three labels that one skill writes and another skill reads were cut over with no alias:

- A plan task's routing override `**Build profile:** <tier>` is now `**Build tier:** <tier>`.
- A worker's return line `PROFILE:` is now `TIER:`.
- The routing lines `Profile:` and `Rung:` are now `Tier:`.

The edits cover the skills, the agent wrappers, the config comments in `agents/harness-defaults.yml` and `.docket.example.yml`, the docs and the glossary. The concept page `docs/concepts/build-profiles-and-gate.md` is renamed to `docs/concepts/build-tiers-and-gate.md`, and every link to it is updated.

### Deviation from the plan, authorized by a human

The spec treats `testdata/repositories/` as frozen. A test, `TestBuiltinAgentsParityWithFrozenSidecar`, requires the live `agents/harness-defaults.yml` to be byte-identical to a frozen copy under `testdata/repositories/v0.9.3/`. Even comment-only edits to the live file break that test. The first build run halted on this conflict at Task 4.

On 2026-09-30 Daniel chose option (b). It follows change 0468's precedent, which added `v0.9.8`:

- The existing tree `v0.9.3` is untouched. Other tests still read its status corpus.
- A new tree, `testdata/repositories/v0.9.9/`, holds a byte-identical copy of the edited file. Its `PROVENANCE.md` records that only comments changed.
- The test now compares the live file against the new copy. The byte-equality check itself is unchanged.
- The matching reference comment in `internal/config/defaults.go` now names `v0.9.9`.
- `internal/repoguard` needed no change. It skips every `testdata` directory as a category and has no executable list of fixture versions.

Two smaller departures from the spec's site list:

- `docs/reference/harness/validation-runbook.md` needed no edit. Its only matches are the unrelated phrase "shell profile".
- The closing grep's permitted residue is a little wider than the spec listed. The "profile" residue also includes that runbook file and two frozen ADR link targets. The "carve" residue also includes the embedded copy of `gate-execution.md`.

## Human actions and testing

### Important — merge only after a drain

This matters because a plan written before this merge still carries the old `**Build profile:**` line, and the new docket-build does not read it. It skips no build: an affected task is routed by the normal rubric instead of by the override. That can drop a task to a cheaper tier than its author wanted.

1. Before merging, check every docket repository's board for changes that are `in-progress` or hold a committed plan that has not been built yet.
   Expected: there are none, or you accept that those builds ignore their `**Build profile:**` override lines.

### Optional — refresh generated agent wrappers

Generated agent descriptions and Cursor rules are loaded when a session starts. Without a refresh they keep the old wording, but nothing breaks.

1. After the merge and the binary rebuild, run `docket install` in each consumer repository.
2. Restart your agent sessions.
   Expected: the build agents' descriptions say "tier" rather than "profile", and the review agents' descriptions say "severity-ranked findings".

## Verification performed

- Each task ran focused Go tests through the gate driver: `internal/repoguard`, `internal/assets`, `internal/harness/...` and `internal/config`. All passed.
- After every regeneration, the goldens and the embedded tree were checked against the rename map.
- Mutation evidence:
  - The updated label pins in Tasks 1 and 2 went red when their label was removed.
  - The repointed parity test went red when one comment byte in the `v0.9.9` copy was altered. It went green again once the byte was restored.
- The whole-repo closing grep (plan Task 5 Step 6) found no rung words and no retired phrases. The "profile" and "carve" residue is only the permitted, unrelated senses listed above.
- The frozen-path check found that the only frozen path added is `testdata/repositories/v0.9.9/`, as authorized. No existing tree changed. `internal/repoguard/budgets_test.go` is byte-identical to the base, so no size ceiling was raised.
- The full suite (`go run ./cmd/docket development test`) passed at the build gate. It ran again after the review fixes, and the evidence for the final head is in the PR body.
- A deep whole-branch review returned no blockers, no important findings and four minor findings. All four are fixed in commit `c55454602`:
  1. The review guide said "model tier" where it now says "build tier".
  2. "never restate literal tiers" in the convention and in the finalize gate-failure reference now names model IDs and efforts. This removes a clash with the new "tier" noun.
  3. `testdata/repositories/v0.9.9/PROVENANCE.md` now records the source commit and "Redaction: none".
  4. A comment in `internal/repoguard/absence_test.go` now names v0.9.9 as the parity copy.

## Known issues and follow-ups

### Stale header comment in `agents/harness-defaults.yml`

The file's header says it is "enforced by scripts/lib/harness-defaults.sh", but that script no longer exists. Nothing breaks; a reader looking for the enforcing code would find the wrong pointer. This is confirmed and predates this change. Fixing it requires another frozen-sidecar copy, which is out of scope for a rename. Suggested next action: fix it at the next sidecar re-cut, or capture it with `docket change create`.

### One integration test is over its time budget

The suite runner confirmed, running serially, that `tests/test_go_integration_app_rebaserecovery.sh` takes 63s against a 60s solo budget. The suite still passes. This change is prose-only and does not touch that test. Suggested next action: triage the budget separately. It may overlap with the budget follow-up change 0472 recorded.
