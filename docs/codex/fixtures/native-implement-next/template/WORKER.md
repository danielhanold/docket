# Codex worker: exact entry and fixed task values

Your native JSON message contains task_input_file, task_input_sha256, entry_checker, entry_argv, scope_id, child_capability, gate_context, run_epoch and assignment. The child capability is your actual authorized token. No parent capability is supplied. Missing dynamic fields mean BLOCKED.

## Entry and reads

Execute entry_argv exactly, as the FIRST shell call with login=false and NO workdir override. The array starts with python3 followed by the absolute checker path and complete arguments. Use ordinary shell quoting to preserve argument boundaries; do not improvise the invocation. This read-only check launches nothing, confirms input hash/worktree identity and requires the primary startup cwd. Preserve BINDING_OK and the observed startup cwd.

Then read the fixed input's feature-root operation_reference, AGENTS.md, skill_files and plan_file. Resolve canonical paths. Every subsequent feature shell has explicit feature workdir and login=false; all file-tool paths are absolute inside feature. The exact external task-input file is an authorized repeated read. The entry checker, its boundary module and exact workspace receipt are permitted entry reads. Task-owned logs under worker_run_root are the only other authorized external reads/writes. No searching the fixture parent, primary, home, other fixtures, host logs, metadata or driver-capability storage.

Fetch your OWN docket capabilities --json as a standalone shell call and validate it before other Docket calls. Resolve operation argv/schema from that catalog. Do NOT run repository.prepare or controller/metadata duties even if the generic convention describes them. Your remaining Docket calls belong to the actual task gate driver.

## Every test invocation

Follow the actual docket-build-task baseline/RED/GREEN, predecessor, WAITING/handoff and final-acknowledgement contract. Reload fixed values in EACH independent shell call. Use this ordinary JSON-loading form; the only file locator to insert is task_input_file from your message:

```sh
task_inputs='<exact task_input_file>'
task_json=$(cat "$task_inputs") || exit 1
worker_run_root=$(jq -er '.worker_run_root' <<< "$task_json") || exit 1
feature_root=$(jq -er '.feature_worktree' <<< "$task_json") || exit 1
task_branch=$(jq -er '.branch' <<< "$task_json") || exit 1
task_change=$(jq -er '.change_id' <<< "$task_json") || exit 1
task_name=$(jq -er '.task_id' <<< "$task_json") || exit 1
task_phase=$(jq -er '.phase' <<< "$task_json") || exit 1
test_args=()
while IFS= read -r test_arg; do test_args+=("$test_arg"); done < <(jq -er '.test_argv[]' <<< "$task_json")
```

Use your live catalog's gate.drive.start argv with --owner task, --run-root "$worker_run_root", --repo-dir "$feature_root", --change-id "$task_change", --task-id "$task_name", --phase "$task_phase", --branch "$task_branch", your exact dynamic scope/child token and any captured predecessor pair, then -- "${test_args[@]}". Pass the exact dynamic gate_context from your native message with --gate-context on EVERY start. Unlike the earlier focused test, this continuous run has an outer implement-next gate context. If run_epoch is a nonempty string, also pass that exact --run-epoch on every start; if null, omit it. Never save tokens in the fixed input file or public evidence. These are ordinary native calls, not a new wrapper. Build start_argv as the complete array for that ordinary catalog-resolved call. Use the exact capture block below. The operation response has a nested drive object: drive.drive_id, drive.generation, drive.outcome and drive.run_root. There is no top-level drive_id or owner_generation. Do not type an independent literal run-root, eval the test string, or expect variables from a prior tool call to persist.

## Preserve the first response, including errors

After loading task JSON and forming start_argv, execute this block in that SAME shell call. The array includes the command and every argument, including the test command after --. Raw response files are private ownership evidence under the authorized worker_run_root; never copy them unredacted into public reports. A nonzero process exit is not itself a malformed receipt: a genuine assertion RED may return a valid FAILED drive. Inspect outcome under the real ownership contract. Never repeat a start to recover a lost response.

<!-- drive-capture:start -->
```sh
umask 077
mkdir -p "$worker_run_root" || exit 1
drive_capture=$(mktemp -d "$worker_run_root/start-response.XXXXXX") || exit 1
# Preserve stdout, stderr and status BEFORE any parsing or early exit.
if "${start_argv[@]}" >"$drive_capture/stdout.json" 2>"$drive_capture/stderr.txt"; then
  drive_process_exit=0
else
  drive_process_exit=$?
fi
printf '%s\n' "$drive_process_exit" >"$drive_capture/exit-code.txt"
if ! jq -e --arg root "$worker_run_root" '
  .protocol_version == 1 and .operation == "gate.drive.start" and .result == "applied"
  and (.drive | type == "object")
  and (.drive.drive_id | type == "string" and length > 0)
  and (.drive.generation | type == "string" and length > 0)
  and (.drive.outcome | . == "PASSED" or . == "FAILED" or . == "WAITING" or . == "HALTED")
  and .drive.run_root == $root
' "$drive_capture/stdout.json" >/dev/null; then
  printf 'BLOCKED: first gate response retained privately at %s (process exit %s). Do not restart.\n' "$drive_capture" "$drive_process_exit"
  exit 1
fi
# This is private operational output, carrying the actual ownership generation.
cat "$drive_capture/stdout.json"
```
<!-- drive-capture:end -->

Retain the parsed nested drive object in private memory immediately. Use its generation unchanged for predecessor-owner-gen and final owner-gen acknowledgement. Preserve the capture directory reference even on invalid-input/non-JSON output; diagnose the original stored response without running a second start. WAITING retains the SAME drive for the normal handoff/claim path. Do not treat FAILED as COMPLETE or HALTED as PASSED.

On WAITING hand off and return. On violation return BLOCKED under the ownership contract, with no commit. On successful GREEN self-review, commit exactly greeting.go/greeting_test.go, acknowledge the final scope, and return the normal COMPLETE receipt. Preserve the plan and all three baseline test cases; no checkbox edits or subagents from plan boilerplate.

Keep tokens out of reusable reports. Include input hash, actual startup cwd, boundary/catalog outcomes, task commit and terminal drive/run-root in the receipt. Controller independently audits those claims against native and driver evidence.
