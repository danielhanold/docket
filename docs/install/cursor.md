# Cursor: running docket under Cursor

Cursor is a first-class harness both as a host and as a delegation target. Running docket under
Cursor's Auto-run in Sandbox needs a small, stable permission configuration, because docket must run
**outside** Cursor's sandbox.

> **Provenance.** Every classifier claim below was observed in **Cursor 3.11.19** on
> **2026-07-14** under **Allowlist (with Sandbox)**. Cursor's auto-run classifier is not a documented
> contract, so treat these as empirical claims about that version, and re-verify if your Cursor
> differs.

### The three gates, and why they are independent

Cursor decides whether an agent command runs, and how, through three independent gates:

1. **Command approval** (`permissions.json` → `terminalAllowlist`) — whether a command auto-runs at
   all, and whether it runs **outside** the sandbox.
2. **Filesystem access** (`sandbox.json` → `additionalReadonlyPaths`) — what a **sandboxed** command
   may read.
3. **Network** — whether a **sandboxed** command may reach the network.

They do not substitute for one another. Granting filesystem or network access to a sandboxed command
does **not** move it outside the sandbox; only a `terminalAllowlist` match does. Edits to
`~/.cursor/permissions.json` are picked up within a second or two without restarting Cursor (file
watcher).

**Run Modes and the allowlist lock.** When `~/.cursor/permissions.json` defines a non-empty
`terminalAllowlist` (or `mcpAllowlist`), Cursor constrains the selectable Run Modes to **Allowlist**
and **Allowlist (with Sandbox)** only. **Run Everything** is disabled (a banner says so), and
**Auto-review (with Sandbox)** — though still shown — becomes non-selectable. Do **not** try to
escape this with an `approvalMode` key — writing `approvalMode: "unrestricted"` alongside allowlists
emptied the Run Mode dropdown entirely and had to be removed. The recommended operator mode is
**Allowlist (with Sandbox)**.

### Why docket must run outside the sandbox

docket's runtime needs the **network**: `preflight` fetches and rebases, and skills push. A
sandboxed docket command fails — typically the `git fetch` to origin dies and `preflight` exits
non-zero — **even when** `sandbox.json` grants a read path and network access, because the command
is still sandboxed. The fix is not more sandbox permissions; it is a `terminalAllowlist` entry that
runs docket **outside** the sandbox.

### The fragments

docket now ships as a native binary on your `PATH`, invoked as `docket <operation>`. That single,
stable command name is all Cursor needs to allowlist — no wrapper path, no environment variable, no
per-spelling entries. Copy these into your Cursor config (ready-made copies ship as
[`permissions.example.json`](../reference/harness/permissions.example.json) and
[`sandbox.example.json`](../reference/harness/sandbox.example.json) in the harness reference; replace
`$USER` with your username and adjust the path to your actual docket clone).

**`~/.cursor/permissions.json`** — allowlist the `docket` binary. Cursor prefix-matches the literal
command string, so the one entry covers every `docket <operation>` invocation.

```json
{
  "terminalAllowlist": [
    "docket"
  ]
}
```

**`~/.cursor/sandbox.json`** — grant a read path to the docket clone (complementary; it does **not**
move docket out of the sandbox — see above).

```json
{
  "additionalReadonlyPaths": [
    "/Users/$USER/dev/docket"
  ]
}
```

### What allowlisting the binary authorizes

Allowlisting `docket` authorizes, **unprompted**, every operation the binary can run — including
destructive and external-writing ones:

- `docket-status`'s guarded sweep — archives merged changes, publishes terminal records onto the
  **integration branch** (the branch code lands on, usually `main`), and deletes merged feature
  branches and worktrees.
- terminal-publish's direct push to the integration branch.
- github-mirror's external writes to GitHub Issues and Projects.
- cleanup-feature-branch's provenance-guarded branch and worktree deletion.

These are shared-history and external writes, and they are the deal you accept for one line of
config. Each is guarded or provenance-checked, which is a mitigation — not a reason to leave the
statement out.

### Why the broader workarounds are not acceptable

It is tempting to allowlist something broader — `eval`, a blanket `bash`, or a bootstrap-command
prefix. Each erases the trust boundary the binary draws and returns the permission surface to
unbounded. The `docket` binary deliberately has **no** `run`/`exec`/`shell`/`eval` operation for
exactly this reason; do not reintroduce one at the permission layer.

### Scope — what this fragment does and does not cover

docket's binary stabilizes docket's own metadata and lifecycle operations. Your repo's **build-time**
commands — feature-branch git, the test suite, `gh` — are that repo's own permission surface. They
are not covered by docket's fragment and not silently granted by it; allowlist them separately
according to your own trust policy. (For example, an agent compound that runs `docket board-refresh`
alongside `git status` needs `git status` allowlisted on its own — the docket entry does not cover
it.)

### Troubleshooting

**A sandbox grant did not make docket work.** You added a read path and network access in
`sandbox.json`, but a docket command still fails (often `git fetch` to origin). Sandbox permissions
govern **sandboxed** commands; they do not move a command outside the sandbox. Only a
`terminalAllowlist` match runs docket unsandboxed. Add the `docket` entry to `permissions.json`.
(Observed: Cursor 3.11.19 · 2026-07-14.)

**One unmatched command in a compound sandboxes the whole program.** A compound command is demoted to
the sandbox **as a whole** if any leaf is unmatched — even a leaf that can never execute (`if false;
then eval true; fi; docket env`). Keep docket calls as standalone commands, and allowlist any other
leaf (e.g. `git status`) on its own. (Observed: Cursor 3.11.19 · 2026-07-14.)

**Invalid JSON silently disables the whole allowlist.** A malformed `permissions.json` (e.g. a
truncated trailing `}`) is silently ignored — the allowlist stops taking effect and every docket call
is demoted to the sandbox. Restoring valid JSON restores the allowlist within a second or two (file
watcher; no restart needed). Validate the file after editing. (Observed: Cursor 3.11.19 ·
2026-07-14.)

The full end-to-end Cursor validation — the CLI probe and the human IDE checklist — lives in
[the Cursor validation runbook](../reference/harness/validation.md).
