# Harness runbooks and examples

Reference material for validating docket on a specific harness (the tool that runs the agent:
Claude Code, Cursor, Codex, or opencode). These are the live-execution checklists and example
files that back up the setup prose. For how to install, configure, and tune docket on your
harness, start at the guide page [Running on your harness](../../guide/running-on-your-harness.md);
these files are what it points you to when you need the exact procedure.

- [`validation.md`](validation.md) — the Cursor validation runbook: the best-effort `cursor-agent`
  CLI probe and the human-executed IDE checklist that must pass before a Cursor wrapper change
  merges.
- [`validation-runbook.md`](validation-runbook.md) — the Codex CLI live-validation runbook: the
  phase-by-phase checklist a human runs inside a real Codex session to confirm skills load, agents
  dispatch, and pins reach the spawned agent.
- [`permissions.example.json`](permissions.example.json) — an example Cursor permissions allowlist
  to copy and adapt (see the guide's Cursor section for what each tier grants).
- [`sandbox.example.json`](sandbox.example.json) — an example Cursor sandbox/network configuration
  to copy and adapt.
- [`fixtures/nested-launch/`](fixtures/nested-launch/README.md) — the committed two-role probe
  fixture (a synthetic `probe-coordinator` and `probe-leaf`) the Codex runbook's nested-dispatch
  phase drives; its README owns the install, staging, and teardown.
