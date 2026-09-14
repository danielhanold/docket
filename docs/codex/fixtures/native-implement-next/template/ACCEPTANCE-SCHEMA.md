# Sanitized acceptance bundle v1

Write evidence/acceptance.json with schema_version 1, explicit evidence_gaps list, and the objects below. Every object requires evidence: a nonempty array of {path, sha256} references relative to evidence/. Copy only selected sanitized receipts, never raw credential-bearing records. Validate with `python3 ../validate-evidence.py acceptance.json` from evidence/. Missing facts must stay missing and result in certification-incomplete. Do not guess a field to satisfy validation.

- initial: unclaimed, plan_absent, feature_absent (true, from initial status/Git evidence).
- workspace: created_in_run, registered (true), primary and feature canonical absolute roots.
- native: roles [docket-implement-next, docket-plan-writer, docket-build-standard], single_coordinator true, dispatches 3, terminal_collected true, agent_enter_calls 0, proof_kind host-observed or mixed-host-and-child-receipts. Preserve concrete lineage/receipt identifiers in referenced evidence. Mixed proof requires explicit gaps.
- plan: newly_authored, verified_attached true, task_count 1, full commit.
- worker: consumed_plan_commit, outcomes [PASSED,FAILED,PASSED], red_assertion true, scope_closed and final_acked true, changed_files [greeting.go,greeting_test.go], input_hash_valid, entry_argv_valid, outer_context_preserved true, cwd, run_root, expected_run_root, full implementation commit. Verify against Git, immutable input and sanitized durable driver/scope records. Outer context is compared privately; never copy it here.
- final_gate: head, command go test -count=1 ./..., outcome PASSED, clean true, cwd feature. This is the exact FINAL results checkpoint commit, distinct from implementation-stage evidence when HEAD changed.
- results: full checkpoint commit, attached true, published_to_local_origin true.
- primary: status PRIMARY_UNCHANGED, references to original and post-stage audits.
- halt: result applied, disposition halted from typed change.halt.
- outer_verdict: report gate-stop, verdict run-halted, same_key true; reference sanitized terminal facade output with key omitted or consistently hashed.

The validator checks completeness, receipt hashes and cross-stage consistency. It cannot authenticate host events or prove that an authored attestation is true. Independent review checks source receipts and distinguishes configuration, child claims, actual observations and unavailable telemetry. A passing validator never marks production423 done.
