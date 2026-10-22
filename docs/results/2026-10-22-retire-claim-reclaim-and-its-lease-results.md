<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0575 — Retire claim reclaim and its lease](../changes/archive/2026-10-22-0575-retire-claim-reclaim-and-its-lease.md)**
<!-- docket:backlink:end -->

# Retire claim reclaim and its lease — Results

**Human action:** One important step: upgrade the `docket` binary on every machine that shares this repository's metadata before any of them claims another change. Nothing else needs a human beyond the normal PR review.

## Outcome

Docket used to give each claimed change a 72-hour "lease" (the `claimed_at` stamp). A `docket change reclaim` command, plus an optional sweep step, could hand an abandoned change back to `proposed` after the lease ran out. That machinery never fired in practice, and from another clone it could hand back a change that was still being built. This change removes it:

- `docket change reclaim` and the maintenance sweep's reclaim pass are gone.
- Claims, reconciles, and halt resumes no longer write `claimed_at`. Validation and repair neither require it nor complain about it, and old records that still carry the line load without findings.
- The claim result no longer reports a lease. Older stored claim receipts that still carry the lease fields decode as before.
- The `reclaim.lease_ttl` and `reclaim.auto` config keys still load in every config layer, but each now produces a warning that it is obsolete and is otherwise ignored.
- Skills and docs now describe one recovery path for a stranded run: resume it by id, with the run tracker deciding.
- ADR-0160 records the decision and supersedes ADR-0159.

One departure from the spec: the spec said to remove the `reclaim:` block from the `example-activated` sample config and move the `four-layer-collision` sample to a different key. `testdata/README.md` forbids editing those samples to make tests pass, and an old config carrying `reclaim:` is exactly what the tombstoned keys have to keep loading. The samples were left unchanged. The tests now check for the obsolete-key warnings against those real samples.

## Human actions and testing

### Important — upgrade every machine together

An older `docket` binary checks that a claimed change carries `claimed_at`. Records claimed by the new binary will not have it, so an older binary on another machine sharing the same metadata branch would report them as invalid. The check matters as soon as this merges and anyone claims a change. If you skip it, a machine still on the old binary will show validation findings for every newly claimed change until it is upgraded.

Prerequisites: the PR is merged to `main`.

1. On each machine that works on this repository, rebuild/install `docket` from `main` (see AGENTS.md, *Rebuild the binary after a merge to main*).
   Expected: the binary's `version` operation reports the merged `main` commit.
2. On one machine, run `docket status`.
   Expected: no `claimed_at` findings on in-progress changes.

### Optional — see the obsolete-key warning

1. In a scratch clone, add `reclaim:\n  auto: true` to `.docket.local.yml` and run `docket diagnostic config`.
   Expected: a warning that `reclaim.auto` is obsolete and ignored; the command still succeeds.

Cleanup: remove the lines you added.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed through the build gate after the five plan tasks, and passed again after the review fixes.
- Each new guard was mutation-tested by its worker: the retired-command pin, the sweep-ignores-in-progress pin, the claim-writes-no-stamp tests, the receipt-decoding test, the config tombstone test, and the skill prose pins (including a case-insensitive `reclaim` word check across the four affected skills).
- Review: deep tier, three minor findings, all fixed in one commit (2a9153782).
- The budget report showed only `BUDGET WATCH` screening lines (first overrun under parallel load), with no breaches.
- Backlog check: compared the follow-up below against all 19 proposed and deferred changes.

## Known issues and follow-ups

- **`go vet -tags integration,e2e ./...` does not compile.** It happens whenever both build tags are vetted together. It fails because `originHasBranch` is declared in both `internal/app/finalize_e2e_test.go` (e2e) and `internal/app/leak_check_private_integration_test.go` (integration). This was already true before this change. Impact is small: each tag vets cleanly on its own, and the suite does not use the combined form. Workaround: vet each tag separately. Confirmed. Suggested next action: rename or share the helper. No existing change fits (checked 19); a human can capture this as a new change.
