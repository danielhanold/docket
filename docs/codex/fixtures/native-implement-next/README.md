# Native ImplementNext option-2 certification package

This is the manually delivered POC package for change423. It contains pinned experimental role/skill source, a fresh-fixture generator, exact planner/worker handoff checks, deterministic mutation tests, sanitized historical evidence and a successful non-native backend rehearsal. It is not installed production routing and it does not mark423 complete. Production implementation is specified by425.

## Create and launch

Requirements: Python3.11+, Node, jq, Git, Go and the standard docket binary on PATH. The current verified binary is v0.9.3-1016-g06ebb52c, commit06ebb52c058894b564ac2a8432922ddf4b2d56b3. Codex must expose native named-agent dispatch and the exact registered model assignments.

Run `python3 setup.py --destination /absolute/new/disposable-directory`. The destination must not exist. Setup uses only this package's source snapshots; it does not depend on the earlier fixture directories or global skill files. It creates a primary repo and local bare origin, one build-ready unclaimed candidate, baseline tests, a primary-integrity snapshot and a hash/mode manifest. No agents, claim, worktree, plan, scope or outer gate start during setup.

Open a NEW **Local** Codex app task at `<destination>/repo`, set **gpt-5.6-terra / low**, and send:

```text
Read and execute <destination>/LAUNCH.md once. This is the authorized continuous native option-2 fixture. Use the registered Terra/low ImplementNext coordinator, Sol/medium planner and Terra/medium standard worker. Preserve the exact outer gate context and supplied epoch, complete planner payload and corrected worker entry/capability payload. Stop at the bounded results checkpoint with typed halt and the parent's keyed verdict. No agent.enter, Luna, replacement agents, review, PR, merge or production changes.
```

Do not run two tasks against the same fixture. Setup a separate destination for any separate run. Distinct-fixture parallel execution is not certified by this POC. Preserve executed fixtures; setup always creates a new one.

## What is reusable

- `template/`: complete pinned snapshots and parameterized run instructions. `@@FIXTURE_ROOT@@` is replaced only by setup. The snapshot is deliberately experimental; see PROVENANCE.json and role-changes.diff. Only the three bounded native roles are registered.
- `setup.py`: setup only, with live capability/schema checks and synchronized metadata; no agent transport.
- `prepare-worker-inputs.py` in each generated fixture: serializes credential-free fixed inputs from the actual new plan; no Docket mutations or scope creation.
- `dispatch-payload.js`: pure data validation of a real scope plus the private outer context/epoch; emits exact ordinary native arguments. It is not an agent/tool wrapper.
- `validate-evidence.py` and ACCEPTANCE-SCHEMA.md: completeness, hashes and cross-stage consistency checks for sanitized receipts. Host authenticity and unavailable child telemetry require independent review.
- `evidence/`: immutable copied history, source references/hashes, and explicitly non-certifying backend rehearsal receipts.

## Verification and rehearsal

Run `python3 test-evidence.py` for synthetic acceptance mutations. To exercise actual entry and planner paths, create a **separate** disposable rehearsal with setup.py, run `python3 rehearse-preparation.py <rehearsal-root>`, then the generated `verify-checks.py`, `verify-dispatch.cjs`, and `rehearse-entry.cjs`. The latter reproduces a0644 permission failure and restores0755. These mutation helpers are preparation-only and must not run during the live native fixture.

`rehearse-preparation.py` creates a clearly labeled synthetic plan and an actual outer gate in that separate rehearsal. It saves required live recovery authority only in a0600 private file. `rehearse-drivers.py <rehearsal-root>` exercises the normal scoped baseline/RED/GREEN, commit/ack, implementation suite, results commit/push/attachment, exact-final-HEAD suite, typed halt and keyed outer verdict. It uses the known tiny fixture transformation, not agents, and cannot certify native orchestration. Do not rerun either script against an already modified rehearsal. WAITING preserves private ownership for collecting the same drive; never restart after a lost/partial result.

Once metadata has been synchronized, `python3 test-planner.py <rehearsal-root>` mutation-tests the complete actual planner payload. None of the tests substitute for the final live run.

Current checks:45 fixed-input/runtime cases,41 native-payload cases including context/epoch,29 planner-payload cases,79 evidence-validator cases, exact Python argv execution, and permission failure/restoration. The full gated backend rehearsal passed on the standard binary; see evidence/rehearsal-binary-compatibility.json. No specialized binary is required.

## Evidence and completion

After the live run, review evidence/run-report.md, coordinator-terminal.md, acceptance.json and their referenced raw/sanitized sources. Preserve implementation and final results-checkpoint commits separately. Any absent child-runtime or tool-path evidence remains an explicit limitation; a clean primary snapshot does not prove absence of transient restored writes. Accepting a named observability limit cannot fill a missing execution stage.

See MANUAL-CLOSEOUT.md for423's truthful, user-authorized exception to the normal PR-based lifecycle. This package does not run production Docket build, implement-next or finalize on423. The final continuous result is still pending.
