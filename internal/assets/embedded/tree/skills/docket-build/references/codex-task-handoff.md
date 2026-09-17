# Codex task handoff

Order: controller-owned repository/workspace preparation. Missing assignment or payload files are controller work. Use protected external storage. Create a dedicated absolute, clean allocation directory; pin assignment.run_root and pass it unchanged to prepare-scope and every start via --run-root, with --json. Keep assignments, payloads and receipt captures outside it. A missing root fails preparation; workers never substitute temporary directories.

Pin provisional assignment without root_identity; agent.check-inputs at prepare returns the witness. Add it to the immutable assignment, freeze/hash, and revalidate prepare. Next, prepare the child scope and private payload carrying assignment locator/digest, assignment-only entry_argv, child capability, context/epoch and predecessor. Validate at `dispatch` with both locators/digests. Parent capability and outer gate key stay private.

Pass both locators/digests unchanged through native dispatch. The child resolves the candidate catalog and `schema --operation agent.check-inputs`, then checks entry. Append --payload and --payload-sha256 to entry_argv; the payload digest stays outside its bytes, avoiding circular hashing. Missing, changed or unvalidated payloads refuse entry.

Load pinned gate_argv, first_stdout and first_stderr in one non-login shell:

```bash
if "${gate_argv[@]}" >"$first_stdout" 2>"$first_stderr"; then
  gate_rc=0
else
  gate_rc=$?
fi
```

Observe the original shell session to terminal. Follow [checked receipt consumption](receipt-semantics.md) for every operation, including facade continuation. Never rerun an operation to recover output. Prepare-scope returns top-level scope_id, child_capability and parent_capability. Drive receipts nest drive_id, generation and outcome under drive; run.gate-claim does not. Transfers omit run_root; terminal start/advance require the assigned run root. PASSED transfers retain contained raw_run_dir. A diagnostic HALTED may omit generation, deadline and paths; it authorizes no continuation.

A successful claim already grants fresh owner authority: advance, never claim twice. Claim/takeover closes the old scope; recovered controllers consume evidence without final acknowledgement on that scope. Later tests use fresh quiescent scopes. Uninterrupted workers commit, validate active inputs, then acknowledge; no writes follow acknowledgement.
