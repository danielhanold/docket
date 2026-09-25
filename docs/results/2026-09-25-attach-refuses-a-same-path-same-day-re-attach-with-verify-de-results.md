# Attach refuses a same-path same-day re-attach with verify-delta invalid-state — Results

**Human action:** None required. The change is ready to merge.

## Outcome

Re-attaching the same plan or results path to a change on the same day used to fail with `invalid-state` at stage `verify-delta` ("a declared path is not an actual change"). The record only stores the artifact path, so a same-day re-attach re-rendered identical bytes, and the transaction engine rejected the unchanged file that the operation had still declared.

`changeAttachOp.Plan` (`internal/app/change_attach.go`) now declares the change record only when its bytes actually change, using the same guard that `change_groom.go` already has. A same-path, same-day re-attach now produces an empty plan, and the engine returns `no-op` with no commit. A first attach, a different path, or a re-attach on a later day still changes the bytes and commits as before. The engine's delta guard, attach idempotency keying and the board declaration are unchanged.

A new regression test, `TestChangeAttachIdenticalReattachIsNoOp` in `internal/app/change_attach_test.go`, attaches the same path twice for both `attach-plan` and `attach-results` with the inline board on and requires an empty plan on the second call. It failed before the fix, with the change record declared, and passes after it.

## Verification performed

- Red before the fix: both subtests failed with "identical re-attach declared files [docs/changes/active/0003-widget.md], want an empty (no-op) plan".
- Green after the fix: `go test ./internal/app/ -run 'TestChangeAttach|TestChangeGroom' -count=1` and `go test ./internal/app/ -count=1` both passed.
- The full suite runs at the build gate on the head that contains this file.
