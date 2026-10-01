<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0479 — Refuse an unfiltered integration-tagged run of internal/app before go test's 10-minute timeout](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0479-ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s.md)**
<!-- docket:backlink:end -->
# Refuse an unfiltered integration-tagged run of internal/app before go test's 10-minute timeout — Results

**Human action:** None required. The optional walkthrough below shows the new refusal message if you want to see it.

## Outcome

Running `go test -tags integration ./internal/app/` with no `-run` filter used to start the whole integration corpus of `internal/app`. That takes about 19 minutes, so go test's default 10-minute timeout killed it with a goroutine-dump panic. Plans kept prescribing this command, and change 0472's build lost a verification step to it.

The `internal/app` integration test binary now refuses that run right after compile. It exits non-zero within seconds and prints the supported forms:

- run one shard script: `bash <one of tests/test_go_integration_app_*.sh>`
- filter by test-name prefix: `go test -tags integration -count=1 -run '^<Prefix>' ./internal/app/`
- run the whole package deliberately: add `-timeout 30m`

The guard refuses only when `-run` and `-list` are both empty and the timeout is the 10-minute default. Any other timeout is allowed, including `-timeout 0` and running a binary built with `go test -c`. `-skip` does not count as a filter. Untagged and e2e builds compile a no-op, so the suite and the shard runners behave as before. `tests/README.md` gains a section, "Running integration-tagged Go tests by hand", that documents these forms.

The guard lives in `internal/testsupport` and uses the same build-tag split as the existing no-real-git guard. Only `internal/app` calls it.

## Human actions and testing

### Optional — see the refusal

Use this if you want to see the message a developer gets.

Run this from a checkout of this branch:

1. Run `go test -tags integration -count=1 -skip . ./internal/app/`
   Expected: within seconds the command exits non-zero. It prints `internal/app: the integration-tagged corpus outlasts go test's default 10m timeout when run whole (change 0479).` followed by the three supported forms.
2. Run `go test -tags integration -count=1 -run '^TestIntegrationRunRecord' ./internal/app/`
   Expected: `ok`. A filtered run is not affected.

Do not drop `-skip .` in step 1 on a tree that lacks this change. Without the guard, that command runs for about 19 minutes and then times out.

## Verification performed

- Unit tests pin every clause of the decision, the flag reading, setup errors (a missing or mistyped testing flag is an error, never a silent allow), and the remedy text. They pass in both the default and the integration build, and `go vet` is clean under the default, integration and e2e tags.
- Mutation tests:
  - Deleting each of the three decision clauses turned at least one unit test red. This was checked during the earlier, halted run. The resumed run committed the same code unchanged.
  - Removing the `TestMain` call, or making the guard return nil, turned the new contract check (11) red within seconds. This ran in a scratch copy outside the worktree.
- `tests/test_go_integration_app_gatelifecycle.sh` passes. Its guardian tests re-exec the test binary, which shows the re-exec routing still runs before the guard. `tests/test_go_integration_app_runrecord.sh` passes as an ordinary filtered shard.
- `tests/test_go_integration_contract.sh` takes 12.8s before this change and 12.9s after, against its 15s budget. The fallback sibling test file was not needed.
- Whole-branch review (standard tier) found no blocker or important issues and two minor ones. Both are fixed in commit e8e3022cf:
  - The remedy text now takes its "10m" from the `goTestDefaultTimeout` constant, so the message can no longer drift from the decision. The text itself is unchanged.
  - The comment above `WholeCorpusTimeout` now names `tests/README.md` as a second place to update when the value is recomputed.
- The whole-suite build gate result is recorded in the PR's build-evidence block.
