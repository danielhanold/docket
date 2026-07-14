# Running docket under Cursor Auto-run — sandbox & permissions

docket's runtime is now two command shapes behind one facade (`docket.sh`), which finally makes a
small, stable Cursor permission configuration possible. This guide shows the configuration, explains
why docket must run **outside** Cursor's sandbox, states plainly what one allowlist entry authorizes,
and troubleshoots the failure modes observed in a live session.

> **Provenance.** Every classifier claim below was observed in **Cursor 3.11.19** on **2026-07-14**
> under **Allowlist (with Sandbox)**. Cursor's auto-run classifier is not a documented contract, so
> treat these as empirical claims about that version, and re-verify if your Cursor differs.

## The three gates, and why they are independent

Cursor decides whether an agent command runs, and how, through three independent gates:

1. **Command approval** (`permissions.json` → `terminalAllowlist`) — whether a command auto-runs at
   all, and whether it runs **outside** the sandbox.
2. **Filesystem access** (`sandbox.json` → `additionalReadonlyPaths`) — what a **sandboxed** command
   may read.
3. **Network** — whether a **sandboxed** command may reach the network.

They do not substitute for one another. Granting filesystem or network access to a sandboxed command
does **not** move it outside the sandbox; only a `terminalAllowlist` match does. Config edits to
`~/.cursor/permissions.json` are picked up within a second or two without restarting Cursor (file
watcher).

**Run Modes and the allowlist lock.** When `~/.cursor/permissions.json` defines a non-empty
`terminalAllowlist` (or `mcpAllowlist`), Cursor constrains the selectable Run Modes to **Allowlist**
and **Allowlist (with Sandbox)** only. **Run Everything** is disabled (a banner says so), and
**Auto-review (with Sandbox)** — though still shown — becomes non-selectable. Publishing a facade
allowlist therefore commits you to an Allowlist mode; it does **not** compose with Auto-review the way
the public docs imply. Do **not** try to escape this with an `approvalMode` key — writing
`approvalMode: "unrestricted"` alongside allowlists emptied the Run Mode dropdown entirely (no options
selectable) and had to be removed. The recommended operator mode is **Allowlist (with Sandbox)**.

## Why docket must run outside the sandbox

docket's runtime needs two things a sandbox denies: an **out-of-workspace program**
(`$DOCKET_SCRIPTS_DIR/docket.sh`, in your docket clone outside the repo) and the **network**
(`preflight` fetches and rebases; skills push). A sandboxed docket command fails — typically the
`git fetch` to origin dies and `preflight` exits non-zero — **even when** `sandbox.json` grants a read
path and network access, because the command is still sandboxed. The fix is not more sandbox
permissions; it is a `terminalAllowlist` entry that runs the facade **outside** the sandbox.

## The fragments

Copy these into your Cursor config. Replace `$USER` with your username (or keep it if your shell
expands it), and adjust the absolute path to your actual docket clone.

**`~/.cursor/permissions.json`** — allowlist the facade. Four spellings are listed because agents emit
the facade invocation in all four forms in practice; Cursor prefix-matches the literal command string,
and the short guarded forms are not prefixes of the canonical one, so each must be listed explicitly.

```json
{
  "terminalAllowlist": [
    "\"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}\"/docket.sh",
    "\"${DOCKET_SCRIPTS_DIR:?}\"/docket.sh",
    "\"${DOCKET_SCRIPTS_DIR:?}/docket.sh\"",
    "/Users/$USER/dev/docket/scripts/docket.sh"
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

## Trust tiers — docket's shell surface, classified

docket's operations sort into three tiers. The classification is derived from `scripts/docket.md`,
which declares itself the permission inventory — it is not hand-copied here.

- **Daily operations, behind the facade — allowlisted.** Everything an agent runs day to day goes
  through `docket.sh <operation>`. That is the one entry above. The facade is the trust boundary.
- **The human-initiated tier — never allowlisted.** `install.sh`, `migrate-to-docket.sh`,
  `sync-agents.sh`, `ensure-docket-env.sh`, `ensure-claude-settings.sh` — one-time setup and migration
  tools a human runs deliberately. They are never invoked by an agent and never belong in the
  allowlist.
- **Internals the facade must not expose** — `docket-config.sh`, `disable-worktree-hooks.sh`,
  `render-board.sh` (plus `lib/` and the tests). They are reached only indirectly, through the facade
  verbs; naming them directly in an allowlist would route around the boundary.

## What one allowlist entry authorizes

Allowlisting the facade authorizes, **unprompted**, every operation the facade can run — including
destructive and external-writing ones:

- `docket-status`'s guarded sweep — archives merged changes, publishes terminal records onto the
  integration branch, and deletes merged feature branches and worktrees.
- `terminal-publish`'s direct push to the integration branch.
- `github-mirror`'s external writes to GitHub Issues and Projects.
- `cleanup-feature-branch`'s provenance-guarded branch and worktree deletion.

These are shared-history and external writes, and they are the deal you accept for one line of config.
Each is guarded or provenance-checked, which is a mitigation — not a reason to leave the statement out.

## Why the workarounds are not acceptable

It is tempting to allowlist something broader — `eval`, a blanket `bash`, a bootstrap-command prefix,
or a generic command-runner subcommand. Each erases the trust boundary the facade exists to draw and
returns the permission surface to unbounded. The facade deliberately has **no** `run`/`exec`/`shell`/
`eval` operation for exactly this reason; do not reintroduce one at the permission layer.

## Scope — what this fragment does and does not cover

The facade stabilizes docket's own metadata and lifecycle operations. Your repo's **build-time**
commands — feature-branch git, the test suite, `gh` — are that repo's own permission surface. They are
not covered by docket's fragment and not silently granted by it; allowlist them separately according to
your own trust policy. (For example, an agent compound that runs `docket.sh board-refresh` alongside
`git status` needs `git status` allowlisted on its own — the facade entry does not cover it.)

## Troubleshooting

Each entry was reproduced in the live session; the stamp records the Cursor version and date.

### A sandbox grant did not make docket work

You added a read path and network access in `sandbox.json`, but `docket.sh preflight` still fails
(often `git fetch` to origin). Sandbox permissions govern **sandboxed** commands; they do not move a
command outside the sandbox. Only a `terminalAllowlist` match runs the facade unsandboxed. Add the
facade entry to `permissions.json`.

**Observed:** Cursor 3.11.19 · 2026-07-14

### A typo in the guard message breaks the match

An allowlist entry with a typo in the guard text — e.g. `docket/instal.sh` instead of
`docket/install.sh` — does not match the real command; Cursor prefix-matches the literal string, so the
command is demoted to the sandbox and origin fetch fails. Copy the spellings exactly.

**Observed:** Cursor 3.11.19 · 2026-07-14

### A short `:?}` spelling is not matched by the canonical entry

If your allowlist has only the canonical `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh`
form but an agent emits a short guarded form like `"${DOCKET_SCRIPTS_DIR:?}"/docket.sh` or
`"${DOCKET_SCRIPTS_DIR:?}/docket.sh"`, the command still requires approval or is demoted — the short
guard is not a prefix of the canonical one. List all four spellings.

**Observed:** Cursor 3.11.19 · 2026-07-14

### One unmatched command in a compound sandboxes the whole program

A compound command (`"…"/docket.sh env; eval true`) is demoted to the sandbox **as a whole** if any
leaf is unmatched — even a leaf that can never execute (`if false; then eval true; fi; "…"/docket.sh
env`). Keep facade calls as standalone commands, and allowlist any other leaf (e.g. `git status`) on
its own.

**Observed:** Cursor 3.11.19 · 2026-07-14

### Invalid JSON silently disables the whole allowlist

A malformed `permissions.json` (e.g. a truncated trailing `}`) is silently ignored — the allowlist
stops taking effect and every facade call is demoted to the sandbox. Restoring valid JSON restores the
allowlist within a second or two (file watcher; no restart needed). Validate the file after editing.

**Observed:** Cursor 3.11.19 · 2026-07-14

### Allowlisting a helper directly does not work (and should not)

Naming an internal like `docket-config.sh` directly in the allowlist — instead of going through the
facade — leaves it sandboxed (origin fetch fails), because the facade-only allowlist does not cover
per-helper invocations. This is by design: the facade is the boundary. Run everything through
`docket.sh`.

**Observed:** Cursor 3.11.19 · 2026-07-14
