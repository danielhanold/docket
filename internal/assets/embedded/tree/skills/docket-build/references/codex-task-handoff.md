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

Observe the original shell session to terminal. Validate with agent.check-receipt: pinned assignment/digest, operation, both files and --exit-code "$gate_rc". Never rerun an operation to recover output. Apply this to prepare-scope, start, advance, acknowledge, handoff, claim and takeover.

Prepare-scope fields are top-level scope_id, child_capability, parent_capability; drive fields are nested drive.drive_id, drive.generation, drive.outcome. Terminal start/advance/acknowledge require the assigned run root; PASSED includes contained raw_run_dir. Transfers omit run_root; PASSED transfers retain contained raw_run_dir. A diagnostic HALTED may omit generation, deadline and paths; it authorizes no continuation. WAITING omits paths; FAILED with nonzero exit is valid. Invalid receipts halt with original stdout/stderr/status retained.

WAITING continues one drive: handoff, claim, advance with new generation. Claim/takeover close the old scope. Record recovered terminal work separately. Later tests use fresh quiescent scopes without closed-scope predecessors. Commit-only continuations carry validated recovered evidence, not rerun tests. Final acknowledgement consumes the current terminal result.
