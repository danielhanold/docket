# Codex gate transport — collect the invocation before the receipt

There are two transport layers before a Docket receipt: `functions.exec` may yield a cell,
and `tools.exec_command` may yield a shell session. Neither yield completes the command.
Preserve each layer's identity until its terminal response has been collected.

For each catalog-resolved gate command, use this code-mode shape (substitute the command
and working directory; do not print private capabilities in commentary):

```javascript
const gateCall = await tools.exec_command({
  cmd: gateCommand,
  workdir: featureWorktree,
  yield_time_ms: 1000,
  max_output_tokens: 12000
});
text(gateCall);
```

Required completion sequence:

1. If `functions.exec` itself reports `Script running with cell ID`, retain that cell ID
   and use `functions.wait` for that same cell until it completes. Do not launch another
   command while its initial response is pending.
2. Retain the **whole** shell response. If it contains `session_id`, the command is live,
   even if `output` is empty. Continue that exact session using
   `tools.write_stdin({session_id: gateSession, chars: "", yield_time_ms: 1000,
   max_output_tokens: 12000})` through `functions.exec`, again emitting the whole result
   with `text(...)`. Collect every output chunk until the shell reports a terminal
   `exit_code` with no live session. Apply step 1 to each code-mode call as needed.
3. Validate accumulated stdout, stderr and terminal exit through `agent.check-receipt`
   using [checked receipt consumption](receipt-semantics.md). A checked `WAITING` authorizes
   advance or handoff; empty live output authorizes only step 2.

`text(gateCall.output)` discards the shell session and exit status and is not a valid
capture wrapper. A longer initial wait is not a repair: a slice plus fingerprinting and
launch overhead can exceed it. Partial JSON must be accumulated, not parsed or retried.
Terminal empty/malformed output, a lost session/cell, or a failed transport observation
remains a caller-contract failure. Never recover credentials by rereading private drive
files or repeating the gate operation. Do not terminate a live session merely to obtain
an exit code. A dispatched child stays in its turn while collecting transport completion;
this is distinct from the forbidden yield/return to its parent while owning a live gate.
