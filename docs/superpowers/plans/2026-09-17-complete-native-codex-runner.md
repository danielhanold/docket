# Complete native Codex runner implementation plan

> **For agentic workers:** Use the test-driven-development discipline task by task. This is the explicitly authorized human-directed, plain Git bootstrap for 432; do not invoke docket-implement-next or docket-build to implement it.

**Goal:** Consume real native continuation receipts correctly and rehearse the remaining completion path before separately authorized 431 acceptance.

**Architecture:** Extend the read-only receipt checker with operation-specific interpretation and expected identity checks. Preserve all producer envelopes and backend ownership semantics. Teach callers to collect their original transport invocation, advance a redeemed drive, and certify each required current HEAD.

**Tech Stack:** Go, public Docket CLI, real temporary Git repositories and supervised processes, generated Markdown assets.

**Spec:** https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-16-complete-native-codex-runner-design.md

## Global constraints

- Preserve the existing branch and its starting commit ed80a72a33ce535ce8c3ab639b6090cb3ba9abc8; its PR base remains 431 while that parent is live.
- Plain Git planning and repair are explicitly authorized after managed workspace preparation refused adoption. Do not fabricate a workspace manifest or claim managed artifact publication succeeded.
- Preserve wire fields, ownership, stale-token rejection, single-use redemption, counters, deadlines and closed-scope acknowledgement rules.
- No private record credential recovery; capture original public stdout/stderr/status in protected external storage.
- No 431 mutation, acceptance, global reinstall, or merge without the separate authorization required by the spec.
- Mocked host dispatch and GitHub effects are deterministic rehearsal limits, not native acceptance evidence.
- Run the configured build suite from source: `go run ./cmd/docket development test`; inspect its budget report.

## Task 1: Public receipt reproduction and interpretation

Files: `internal/cli/agent_receipt_test.go`, `internal/codexcontract/receipt.go`, `internal/codexcontract/receipt_test.go`, `internal/app/agent_receipt.go`, `internal/cli/agent_receipt.go`.

- [ ] Extend the existing real CLI gate fixtures to produce a handoff and facade claim, retaining the actual serialized output. Submit those bytes to `agent.check-receipt` and assert successful claim means owner authority and `gate.drive.advance`, preserving the drive id. A facade refusal must classify as halt even at exit zero.
- [ ] Run `go test ./internal/cli -run TestAgentReceipt -count=1`; record the failing consumer boundary, not a manufactured producer failure.
- [ ] Add `ReceiptExpectation{DriveID, Phase}` and operation-specific parsing for the facade envelope. Keep the facade fields top-level in the checked receipt. Add `authority` (`owner` or `handoff`) and `next_operation`; a successful claim uses `gate.drive.advance`, never another claim. Reject missing fields, duplicate keys, wrong operation/identity, inconsistent result/exit/terminal and malformed output.
- [ ] Expose expected-drive/phase flags on the checker. A coordinator may supply its established run root without a worker assignment; require explicit context and reject conflicting assignment/run-root inputs. This remains a read-only shape/identity check; backend calls alone establish ownership.
- [ ] Preserve existing `ParseReceipt` callers with an optional expectation argument. Test ordinary terminal generations and acknowledgement independently of recovery.
- [ ] Run checker and CLI tests, including mutated identity/envelope fields; commit the repair only after GREEN.

```go
// Core assertion at the public checker boundary:
if checked.Receipt.Authority != "owner" || checked.Receipt.NextOperation != "gate.drive.advance" {
    t.Fatal("redeemed continuation must advance under fresh owner authority")
}
```

## Task 2: Caller instructions and transport regression coverage

Files: `skills/docket-build/references/codex-task-handoff.md`, `skills/docket-build/references/codex-gate-transport.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-implement-next/references/edge-paths.md`, existing `internal/harness` contract tests and generated assets.

- [ ] Derive all maintained receipt and continuation instruction sites using `rg -n 'gate-claim|check-receipt|generation|handoff' skills agents internal/harness`.
- [ ] State exact operation-specific JSON paths and the successful facade claim's next operation. Separate uninterrupted final acknowledgement from recovered evidence consumption.
- [ ] Teach checked coordinator receipts using established drive/phase/run-root context, including collecting a terminal claim's full receipt by advance without relaunch.
- [ ] Keep shell-session and code-mode cell identities separate; accumulate all chunks through terminal return. Test initial completion, live empty output, split JSON, terminal missing/malformed output, lost/wrong handles and no mutation replay through the same example collector taught to callers.
- [ ] Preserve actual native child identity and require terminal return before consuming child disposition. Guard the instruction producer, regenerate with `go generate ./internal/assets/`, and run cross-harness contract tests. Mutate new guards and record their expected failure.

## Task 3: Recovered completion rehearsal

Files: extend `cmd/nativefixture` integration fixtures and the topical CLI gate tests; use existing input, review, evidence and publication helpers.

- [ ] Exercise direct handoff/claim/advance and facade verdict/claim/advance using public producers. Assert owner rotation, same execution identity and unchanged retry usage; prove stale owners and consumed tokens are refused.
- [ ] Cover WAITING and terminal PASSED/FAILED/HALTED, including process completion while the caller is absent. Advance recovered terminals to obtain full evidence; prove old-scope acknowledgement is still rejected.
- [ ] Rehearse worker active-input checks and commit/ack order. Commit results to move HEAD and prove stale review evidence is rejected; certify the new head, enter review, consolidate material findings, then certify final HEAD.
- [ ] Record durable evidence into a new external file and verify exact HEAD. Use fake external GitHub effects only for publication and prove wrong base/head/results refuse completion; require correctly based PR and run-complete in the fixture.
- [ ] Document the fixture's simulated host/review/GitHub limits. Do not report deterministic fixture dispatch as real native review.

```bash
go test -tags integration ./cmd/nativefixture -run TestIntegrationNativeFixture -count=1
go test ./internal/cli ./internal/codexcontract -count=1
```

## Task 4: Candidate verification and acceptance handoff

Files: `docs/results/2026-09-17-complete-native-codex-runner-results.md`; existing formatting defect in `internal/githubcli/comment_integration_test.go`.

- [ ] Apply gofmt to the baseline's existing formatting failure and verify the diff is formatting only.
- [ ] Record RED/GREEN, mutation results, rehearsal coverage and honest remaining limitations in the results artifact. Commit substantive results before final suite verification.
- [ ] Run the whole resolved build suite from the feature checkout; read budget findings and investigate serially confirmed breaches.
- [ ] Pin a clean source candidate, matching embedded assets and binary identity in an isolated output directory; preserve global runtime and role pins. Verify generated role aliases against the pinned assets.
- [ ] Prepare a concrete 431 acceptance proposal distinguishing external runtime delivery from source-owned suite code and branch integration. Preserve all 431 checkpoints and require supported old-run inspection/cancellation and new admission under separate human authorization.
- [ ] Stop at the acceptance authorization boundary. Managed attachment/publication remains pending supported workspace adoption; do not mark 432 implemented or compatible merely because rehearsal and suite pass.
