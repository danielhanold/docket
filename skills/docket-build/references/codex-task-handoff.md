# Codex task handoff

Construct a child in this order: complete controller-owned repository/workspace preparation. Pin a provisional assignment without `root_identity`; run `agent.check-inputs` at `prepare` for the root witness. Add it to the final assignment, freeze/hash that immutable assignment, and rerun `agent.check-inputs` at `prepare`. Then prepare the child scope and write the private payload with the assignment locator/digest, assignment-only `entry_argv`, child capability, scope identity, optional context/epoch, and predecessor. Run `agent.check-inputs` at `dispatch` with separate assignment and payload locators/digests. Store the parent capability and outer gate key privately.

Pass the assignment and private payload locators/digests unchanged through native dispatch. The child loads the executable's catalog and `schema --operation agent.check-inputs`, then runs its checker at `entry`. Append `--payload <path> --payload-sha256 <digest>` to the assignment-only `entry_argv`; the digest stays outside the payload bytes and cannot be circular. A missing, changed, or unvalidated private payload refuses entry.

Load `gate_argv`, `first_stdout`, and `first_stderr` from pinned private inputs in this same non-login shell call:

```bash
if "${gate_argv[@]}" >"$first_stdout" 2>"$first_stderr"; then
  gate_rc=0
else
  gate_rc=$?
fi
```

Retain `gate_rc` immediately and observe the original shell-tool session to terminal before reading the files. Run catalog-resolved `agent.check-receipt` with the pinned assignment/digest, operation, both files, and `--exit-code "$gate_rc"`. Never rerun a gate operation to recover output. Use this sequence for prepare-scope, start, advance, acknowledge, handoff, claim, and takeover.

Prepare-scope fields are top-level `scope_id`, `child_capability`, and `parent_capability`. Start, advance, and acknowledge fields are nested `drive.drive_id`, `drive.generation`, and `drive.outcome`; terminal responses require the assigned run root, and only PASSED carries `drive.raw_run_dir`. Handoff, claim, and takeover use `drive.generation` as the handoff or new-owner generation and omit run paths. WAITING may omit a run root. FAILED with nonzero exit is valid. Invalid JSON, refusal, incomplete fields, or mismatched roots halt with original stdout, stderr, and status retained.

WAITING continues one drive: hand off, then claim and advance with the new generation. Claim and takeover close the old scope. Record recovered terminal work separately. Later tests use a fresh quiescent scope without the closed scope's predecessor. A commit-only continuation carries validated recovered evidence and does not rerun a passing stage. Final acknowledgement consumes the current terminal result.
