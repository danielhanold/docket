# docket command facade — design

- **Change:** 0068 (`docket-command-facade`)
- **Date:** 2026-07-13 (interactive groom; decisions settled with the human)
- **Follow-ups:** 0072 (skill/Step-0 rewiring), 0073 (Cursor guide + published config)

## Problem

Two forces, one root cause.

1. **Cursor's auto-run classifier.** Cursor classifies every leaf command of a submitted shell
   program before deciding whether the whole program may run outside the sandbox; a single
   non-allowlisted leaf — even an unreachable one — demotes the entire program. Docket's Step-0
   prose asks agents to compose `eval`, branching, worktree creation, hook setup, and fetch/pull
   into multiline programs, so every leaf is another permission failure mode and the allowlist
   can never be finite.
2. **The `eval` pattern stores resolved config in the shell.** `eval "$(docket-config.sh
   --export)"` treats the agent's shell as the persistence layer for resolved configuration.
   Shell-state persistence across agent tool calls is harness-dependent — Claude Code's Bash tool
   does not persist env vars across calls at all — and even where a shell persists, a restarted
   shell silently drops the exports with nothing to detect it. The only state guaranteed to
   persist across tool calls in **every** harness is the model's own context window.

The root cause: docket routes resolved values *through the shell* to reach the model, when the
model already reads every byte the resolver prints. The shell round-trip adds fragility and adds
the exact command shapes (`eval`, `source`, compound programs) that permission classifiers
cannot allowlist.

## Core decisions

1. **One executable facade, `scripts/docket.sh`.** It accepts only documented named subcommands
   and rejects unknown operations; it never evaluates caller-supplied shell text. No `run`,
   `exec`, `shell`, `eval`, or equivalent escape-hatch operation, ever.
2. **Config flows model-ward, not shell-ward.** `docket.sh env` prints fully resolved
   `KEY=value` lines to stdout (absolute paths where the value is a path, `BOOTSTRAP` verdict
   included). The agent **reads** the values from the tool result and interpolates them as
   **literals** into subsequent commands. Nothing docket emits is ever `eval`'d or `source`d by
   an agent. There is no source-only mode and no sourced-vs-executed detection.
3. **Side effects are a plain executable op.** `docket.sh preflight` performs today's Step-0
   side effects — resolve config, enforce the bootstrap verdict fail-closed, ensure the metadata
   worktree (state-specific, idempotent), disable its hooks (change 0063), fetch + pull the
   metadata branch — and on success ends by printing the same `KEY=value` block `env` prints, so
   one call serves a skill's Step-0. `env` remains as the cheap read-only re-read.
   Re-running `preflight` is the sanctioned mid-run re-sync verb.
4. **Self-sufficiency invariant.** Every facade operation requires only `DOCKET_SCRIPTS_DIR`
   (profile-injected by `install.sh`, so it survives fresh shells on every harness). Operations
   that read or write the metadata tree run the shared preflight implementation internally
   (idempotent, cheap when already synced). **No operation depends on previously exported
   environment or on a persistent shell.** Pure renderers taking explicit paths skip the
   internal preflight.
5. **Exactly one canonical spelling**, byte-identical everywhere docket emits or documents it:

   ```bash
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh <operation> [args...]
   ```

   This matches the existing convention spelling for helper invocation (quote closes before
   `/docket.sh`). Tests enforce the single spelling; permission entries match it plus the
   resolved-absolute form.
6. **`docket-status.sh` reuses the shared preflight implementation** instead of its private
   `ensure_and_sync_worktree` (docket-status.sh:40-56 today) — one sync implementation, no
   second copy to drift.

## Subcommand inventory

**Operation name = daily helper's script basename** (minus `.sh`). No alias table to drift, and
call sites stay grep-derivable across the whole repo (LEARNINGS 2026-07-13 #64: derive gated
call-site lists by grep, never by hand).

| Operation | Wraps | Notes |
|---|---|---|
| `preflight` | shared impl (extracted) | side effects + prints env block |
| `env` | `docket-config.sh` | read-only; prints resolved `KEY=value` lines |
| `docket-status` | `docket-status.sh` | orchestrator; internal sweep stays guarded as today |
| `board-refresh` | `board-refresh.sh` | gated BOARD.md writer |
| `archive-change` | `archive-change.sh` | |
| `terminal-publish` | `terminal-publish.sh` | ADR-0027 gating unchanged |
| `cleanup-feature-branch` | `cleanup-feature-branch.sh` | provenance guards unchanged |
| `github-mirror` | `github-mirror.sh` | |
| `sync-integration-branch` | `sync-integration-branch.sh` | |
| `render-change-links` | `render-change-links.sh` | pure renderer (explicit paths) |
| `render-adr-index` | `render-adr-index.sh` | pure renderer |
| `adr-checks` | `adr-checks.sh` | read-only checks |
| `board-checks` | `board-checks.sh` | read-only checks |

**Not exposed** (facade must reject): one-time human-initiated tools (`install.sh`,
`migrate-to-docket.sh`, `sync-agents.sh`, `ensure-docket-env.sh`, `ensure-claude-settings.sh`),
internals reached only through an op (`docket-config.sh`, `disable-worktree-hooks.sh`,
`render-board.sh` — internal to `board-refresh` per its gated-writer contract), `lib/`, and
tests. The facade's subcommand table (in its `scripts/docket.md` contract) **is** the permission
inventory.

## Dispatch semantics

- Arguments after the operation name are forwarded verbatim to the helper; existing helper
  argument validation and provenance guards remain authoritative.
- The helper's exit code and stderr pass through unmasked.
- Unknown operations and missing operation names exit non-zero and list the supported
  operations.
- The facade is a routing boundary, not a second implementation — behavior stays in the helpers.

## `env` output format

- One `KEY=value` per line, no `export ` prefix, no quoting for shell re-consumption — the
  consumer is the model, not a shell. Reuses `docket-config.sh`'s resolver; the facade owns only
  the presentation.
- Path-valued keys are absolute (resolved against the repo the command ran in), so interpolated
  literals are unambiguous regardless of later cwd.
- `BOOTSTRAP` is always present; agents act on the verdict exactly as today (`PROCEED` /
  `STOP_MIGRATE` / `CREATE_ORPHAN`).
- An aborting resolver run emits nothing and exits non-zero — the empty-output-plus-success
  false-pass mode (LEARNINGS 2026-07-13 #64b) cannot occur.

## Error handling

- Config and bootstrap failures fail closed with the existing actionable diagnostics.
- Worktree creation follows the current state-specific remote/local branch behavior, idempotent.
- Metadata fetch or rebase failure stops preflight before any metadata read or write.
- A failed helper preserves its exit code and stderr; the facade adds nothing and masks nothing.

## Verification

- Hermetic preflight tests: docket-mode and main-mode, existing/missing worktree, hook setup,
  sync failure, all three bootstrap verdicts, env-block emission on success only.
- Dispatch tests: every inventory operation routes to its helper, argument forwarding, exit-code
  preservation, rejection of unknown/arbitrary operations, rejection of the not-exposed scripts.
- `env` output tests: parseable `KEY=value`, absolute paths, no `export ` prefix, non-zero +
  empty on resolver abort.
- Inventory sentinel: a test derives (by grep, tokenized per invocation) every runtime helper
  invocation across the repo and fails when a daily helper is reachable outside the facade or an
  operation is undocumented in the contract table. Mutation-test the sentinel before trusting it
  (LEARNINGS 2026-07-13 #64).
- `docket-status.sh` tests prove it shares the preflight implementation and performs no second
  private sync.
- Co-located `scripts/docket.md` contract documents the table as the permission inventory.

## Deferred to follow-ups

- **0072** — rewire the seven operating skills and the convention's Step-0 preamble to
  `preflight` + literal interpolation; wiring tests (no inline `eval`/`if`/worktree/fetch-pull
  programs in skill prose; facade-only runtime invocations, with a carve-out for prose
  references to the human-initiated tier); mid-run re-sync guidance.
- **0073** — Cursor-focused guide: `~/.cursor/permissions.json` / `sandbox.json`, reload
  behavior, command-approval vs filesystem vs network gates, the copyable permission fragment
  matching the canonical forms, the trust-tier classification, the security consequences of
  allowlisting `docket-status` (its guarded sweep archives, publishes, cleans branches), the
  observed classifier failure modes **with provenance** (Cursor version + session date).

## Rejected alternatives

- **Source-only preflight exporting into "the persistent agent shell"** — rests on a
  harness-dependent premise (false in Claude Code, unverified in Cursor across shell restarts);
  requires a `source` builtin permission entry; needs bash/zsh sourced-vs-executed detection;
  a stale shell reproduces exactly the continue-without-environment state it exists to prevent.
- **Allowlisting individual helper leaves** — 57 scripts with heterogeneous risk profiles; every
  new helper is a new permission failure mode.
- **Blanket `bash`/`eval` approval or a generic command-runner op** — erases the trust boundary
  the facade exists to draw.
