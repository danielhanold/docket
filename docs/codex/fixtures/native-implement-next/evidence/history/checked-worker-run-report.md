# Focused worker regression — run report

## Outcome

`worker-contract-passed`: the single dispatched `docket-build-standard` worker completed the prepared Task 1 and the controller's one required build-owned suite passed. This is a focused fixture result only; it does not certify hard isolation, parallel safety, production readiness, or completion of change 423.

## Runtime and lineage

- Controller runtime checker: `RUNTIME_OK`, current task `01a0a048-8107-7101-817e-31b31568a5b1`, `gpt-5.6-terra` / `low`, startup cwd `/Users/homer/dev/docket-poc-0423-checked.xlmwfwc4/repo` (checked before Docket operations and immediately before scope preparation).
- The one native child was dispatched as registered `docket-build-standard`, task `/root/focused_worker`, with `fork_turns: none`. The resolved live Codex profile configuration identifies that role as `gpt-5.6-terra` / `medium`.
- The controller validated the prepared payload against the live child capability and excluded the parent capability. The native message body was operationally opaque after dispatch; it was not plaintext-audited from host records.
- Child receipt reports `BINDING_OK` from the controller startup cwd and an independent live Docket catalog. This controller did not claim unavailable child-host log inspection.

## Fixture and controller checks

- The initial and final manifest hash/mode validations passed for every listed immutable file.
- Live Docket catalog binary was `v0.9.3-1016-g06ebb52c`, commit `06ebb52c058894b564ac2a8432922ddf4b2d56b3`, matching the verified standard build.
- `repository.prepare` was applied by the controller; status located change 0001 as `in-progress` with version `af1f4005ae4341da6e193fa2b158a3067982a1a8`, and the controller refreshed its claim.
- The known `artifact-missing` finding remains: the change's metadata plan reference is absent from primary while the imported plan remains feature-only. No artifact was moved.
- Scope `0a3d0a289a190f4beaa60e4d9167b95a` was prepared exactly once; its parent capability stayed private.

## Worker evidence

- Fixed input SHA-256: `1c011222d260379394d1e81f02e3f521f56fda23f27095dc9e53dd8e02c2007d`.
- The child reports fixed input reloads for every test, genuine baseline/RED/GREEN, and final scope acknowledgement on drive `abaede87c3959cd7b41a13b3bc2e4c8d`.
- Task commit: `978e9236b155ee4dc089a327ff85ff7c7c2d70eb` (`feat: trim whitespace in greeting`). It is the sole commit after prepared head `903c581180ade136c78a68d139ceb00edb93ab0d` and modifies only `greeting.go` and `greeting_test.go`.
- The imported plan is unchanged and the feature worktree was clean before and after the controller suite gate.

## Final build gate and integrity

- `diagnostic.config` at the feature root resolved `effective.build.test_command.value` to `go test -count=1 ./...`, matching fixed worker input.
- The controller invoked the configured suite once through `gate.drive.start --owner build`, with no explicit test-command override. Drive `a62448e075403bfb697c6f8f38663c53` returned `PASSED`; raw run directory: `evidence/final-build-gates/1611181d7e6ed3c6128e2d1cb2369796`.
- Implementation HEAD remained `978e9236b155ee4dc089a327ff85ff7c7c2d70eb` and the feature tree remained clean after that suite.
- Both primary audits returned `PRIMARY_UNCHANGED`. The audit's documented limitation remains: it cannot detect transient writes that were restored.

## Limitations

No private capability is included here. The controller validated its retained parent capability against the generated dispatch data and did not publish it. Native message contents are treated as opaque after delivery, so successful child driver credential use is evidenced by the child terminal receipt and gate records, not by a plaintext host-log audit.
