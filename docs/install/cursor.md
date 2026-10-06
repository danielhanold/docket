# Cursor: running docket under Cursor

Cursor is a first-class docket harness. Running docket under Cursor's Auto-run in Sandbox needs a
small, stable permission configuration, because docket must run **outside** Cursor's sandbox.

### What an install writes for Cursor

- **User-level skills and agents.** docket's skills are linked under `~/.cursor/skills/`, and its
  17 agent wrappers are written to `~/.cursor/agents/docket-*.md`. Each carries `name`,
  `description`, and a `model:` line wherever a model resolves, with any reasoning effort encoded
  inside the model value. Agent overrides come from your global config only. Nothing is written
  per repository for these.
- **A `sessionStart` hook in `~/.cursor/hooks.json`.** This content-free hook runs
  `dckt instructions --hook cursor`, which prints nothing outside a private repository, so it is
  inert everywhere else. A `hooks.json` docket cannot edit in place (a symlink, or a file docket
  cannot parse, such as one with comments) is left untouched, and the run warns about it. Uninstall removes the hook only while it is
  unchanged.
- **The dispatch rule, on opt-in only.** When a repository's `.docket.yml` or `.docket.local.yml`
  lists `cursor` in `agent_harnesses`, the install writes `.cursor/rules/docket-dispatch.mdc` in that
  repository. It tells Cursor to dispatch a docket workflow to its matching agent instead of running
  it inline. Without the opt-in, the install touches no repository file.

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

docket's runtime needs the **network**: `docket repository prepare` and
`docket maintenance preflight` fetch from origin, and the workflows push. A sandboxed docket command
fails — typically the `git fetch` to origin dies and the command exits non-zero — **even when**
`sandbox.json` grants a read path and network access, because the command is still sandboxed.
The fix is not more sandbox permissions; it is a `terminalAllowlist` entry that runs docket
**outside** the sandbox.

### The fragments

docket ships as a native binary on your `PATH`, invoked as `docket <operation>`. That single,
stable command name is all Cursor needs to allowlist — no wrapper path, no environment variable, no
per-spelling entries.

**`~/.cursor/permissions.json`** — allowlist the `docket` binary. Cursor prefix-matches the literal
command string, so the one entry covers every `docket <operation>` invocation.

```json
{
  "terminalAllowlist": [
    "docket"
  ]
}
```

### What allowlisting the binary authorizes

Allowlisting `docket` authorizes, **unprompted**, every operation the binary can run — including
destructive and external-writing ones:

- `docket maintenance sweep` — closes out merged changes on the `docket` branch, pushes them, and
  deletes merged feature branches and worktrees under ownership proof.
- `docket finalize` — rebases a change's branch, pushes the rebased head, and merges its pull
  request.
- `docket gate launch -- <argv…>` — runs the command it is given as a supervised gate run. This is
  how the build and finalize gates run your test suite, and it means the allowlist entry also
  covers any command an agent passes after the `--`.
- Every write to the `docket` metadata branch, which is pushed to origin.

These are shared-history and external writes, and they are the deal you accept for one line of
config. Each is guarded or provenance-checked, which is a mitigation — not a reason to leave the
statement out.

### Why the broader workarounds are not acceptable

It is tempting to allowlist something broader — `eval`, a blanket `bash`, or a bootstrap-command
prefix. Each grants every command unconditionally, without even the supervised, recorded run that
`docket gate launch` wraps around the commands it starts. Allowlist `docket` and nothing broader.
(`docket run` is the run tracker: it records dispatched runs and executes nothing.)

### Scope — what this fragment does and does not cover

docket's binary stabilizes docket's own metadata and lifecycle operations. Your repo's **build-time**
commands run directly — feature-branch git, `gh`, a test command an agent runs outside
`docket gate` — are that repo's own permission surface. They are not covered by docket's fragment;
allowlist them separately according to your own trust policy. (For example, an agent compound that runs `docket status`
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
then eval true; fi; docket status`). Keep docket calls as standalone commands, and allowlist any other
leaf (e.g. `git status`) on its own. (Observed: Cursor 3.11.19 · 2026-07-14.)

**Invalid JSON silently disables the whole allowlist.** A malformed `permissions.json` (e.g. a
truncated trailing `}`) is silently ignored — the allowlist stops taking effect and every docket call
is demoted to the sandbox. Restoring valid JSON restores the allowlist within a second or two (file
watcher; no restart needed). Validate the file after editing. (Observed: Cursor 3.11.19 ·
2026-07-14.)

The full end-to-end Cursor validation — the CLI probe and the human IDE checklist — lives in
[the Cursor validation checklist](../reference/harness/validation.md).
