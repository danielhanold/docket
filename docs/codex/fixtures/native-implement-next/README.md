# Native ImplementNext option-2 certification package

This is the manually delivered POC package for change 423. It contains pinned experimental role/skill source, a fresh-fixture generator, exact planner/worker handoff checks, deterministic mutation tests, sanitized historical evidence and a successful non-native backend rehearsal. Change 423 is completed as an accepted manual POC; see FINAL-RESULTS.md and evidence/final-run/. This is not installed production routing. Production implementation is specified by 425.

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
- `check-worker-scope.py`: controller-only read-only comparison of the actual minted scope against the immutable task input. It reads the pinned binary's v3 record because no catalog scope-inspection operation exists; unknown schemas halt. This is an experimental fixture check, not a production API or launcher.
- `dispatch-payload.js`: pure data validation of that verified scope, exact seed, and private outer context/epoch; emits exact ordinary native arguments. It is not an agent/tool wrapper.
- `check-results-template.py`: direct, read-only verification of the tracked template against the launch manifest in primary and the created feature worktree. The returned absolute feature path is the required results source.
- `validate-evidence.py` and ACCEPTANCE-SCHEMA.md: completeness, hashes and cross-stage consistency checks for sanitized receipts. Host authenticity and unavailable child telemetry require independent review.
- `evidence/`: immutable copied history, source references/hashes, and explicitly non-certifying backend rehearsal receipts.

## Verification and rehearsal

Run `python3 -B test-results-template.py` and `python3 -B test-missing-results-template.py` for hidden-directory discovery, missing/changed/redirected templates, and setup refusal before fixture allocation. Run `python3 test-handoff.py` and `node test-unverified-dispatch.cjs` for the scoped-identity and first-response regressions. Run `python3 test-evidence.py` for synthetic acceptance mutations. To exercise actual entry and planner paths, create a **separate** disposable rehearsal with setup.py, run `python3 rehearse-preparation.py <rehearsal-root>`, then the generated `verify-checks.py`, `verify-dispatch.cjs`, and `rehearse-entry.cjs`. The latter reproduces a0644 permission failure and restores0755. These mutation helpers are preparation-only and must not run during the live native fixture.

`rehearse-preparation.py` creates a clearly labeled synthetic plan and an actual outer gate in that separate rehearsal. It saves required live recovery authority only in a0600 private file. `rehearse-drivers.py <rehearsal-root>` exercises the normal scoped baseline/RED/GREEN, commit/ack, implementation suite, results commit/push/attachment, exact-final-HEAD suite, typed halt and keyed outer verdict. It uses the known tiny fixture transformation, not agents, and cannot certify native orchestration. Do not rerun either script against an already modified rehearsal. WAITING preserves private ownership for collecting the same drive; never restart after a lost/partial result.

Once metadata has been synchronized, `python3 test-planner.py <rehearsal-root>` mutation-tests the complete actual planner payload. None of the tests substitute for the final live run.

Current checks:45 fixed-input/runtime cases,41 native-payload cases including context/epoch,29 planner-payload cases,79 evidence-validator cases, exact Python argv execution, and permission failure/restoration. The added40 focused scope/receipt checks and dispatch verification checks pass. Removing the task-identity check or restoring the incorrect top-level receipt parser makes the regression fail. The original failed run's actual saved scope is now rejected before dispatch; see checks/original-failure-regression.json. The latest full gated backend rehearsal uses the EXACT worker capture block and the live scope-checker output with the actual payload body; it passed on the standard binary. See evidence/scope-rehearsal-binary-compatibility.json and checks/scope-receipt-mutations.json. Earlier rehearsal files remain historical. No specialized binary is required.

## Evidence and completion

After the live run, review evidence/run-report.md, coordinator-terminal.md, acceptance.json and their referenced raw/sanitized sources. Preserve implementation and final results-checkpoint commits separately. Any absent child-runtime or tool-path evidence remains an explicit limitation; a clean primary snapshot does not prove absence of transient restored writes. Accepting a named observability limit cannot fill a missing execution stage.

See MANUAL-CLOSEOUT.md for 423's truthful, user-authorized exception to the normal PR-based lifecycle. This package does not run production Docket build, implement-next or finalize on 423. The final continuous result passed and was accepted for manual closeout on2026-09-14; see FINAL-RESULTS.md.

## Run03 correction

Run02 proved the native chain through implementation. Its template was present; default file search omitted the hidden .agents directory. The new launch supplies the exact template path and hash, checks it in primary before any dispatch, checks it again in the new feature worktree before planner dispatch, and uses that path for results authoring. Setup refuses a missing packaged template. The parent now finalizes acceptance only after collecting its real outer verdict, preserving the coordinator draft. Full acceptance requirements are unchanged.

The fresh complete backend rehearsal consumed the actual included results template and passed through results commit/publication/attachment and the final checkpoint suite; see evidence/results-rehearsal-binary-compatibility.json and evidence/results-rehearsal-results-template-consumption.json. The preparation is not a native certification result. The fresh manual run is described in MANUAL-RUN.md.

## Final accepted outcome

The successful continuous run 04 and independent review are packaged in evidence/final-run/. Both implementation and final results-checkpoint suites passed; results were committed, locally published and attached. The original outer key was verified. The user authorized manual closeout with the documented observation limits. No further POC run is required. See FINAL-RESULTS.md for the complete outcome and lifecycle exception.
