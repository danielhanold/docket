# Codex task handoff

Keep the static assignment separate from live gate authority. Prepare a scope, capture the first JSON response and exit code once, then write the final private worker payload with the actual child capability, scope identity, unchanged optional context/epoch, and applicable predecessor. Validate its bytes and digest immediately before native dispatch. Keep the parent capability and outer gate key in controller-private storage.

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

WAITING continues the same drive: hand it off, then claim and advance with the new generation. Claim and takeover close the old child scope. Record recovered terminal work separately; later tests use a fresh quiescent scope with no predecessor from the closed scope. A commit-only continuation carries validated recovered evidence and does not rerun a passing stage. Final acknowledgement consumes the current terminal result.
