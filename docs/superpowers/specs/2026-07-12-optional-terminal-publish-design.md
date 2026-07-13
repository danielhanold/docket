# Optional terminal-publish — the `terminal_publish` knob

> Design spec for change 0064. Makes docket's terminal-publish step opt-out per repo, so
> repositories whose integration branch requires pull requests can keep every metadata record on
> `docket` instead of auto-copying it onto the integration branch.

## Problem

On a terminal transition (`done` or `killed`), docket's shared close-out sequence runs
**terminal-publish** (step 3 of `references/terminal-close-out.md`): it copies the archived change
file + its `spec:` + the `Accepted` ADRs in `adrs:` from `origin/docket` onto the integration
branch via `git checkout origin/docket -- <paths>` — a direct commit to the integration branch,
never a PR.

In private repos where direct commits to the integration branch are tolerable, this is the
desired behavior — the closed change's record lands on `main` automatically. But in repos where
the integration branch is `main` and **every merge to `main` is expected to go through a pull
request**, this auto-copy fights the workflow: docket writes to `main` behind the PR gate.

The code the change actually produces is unaffected — the feature branch's code + `plan:` +
`results:` reach the integration branch through the normal PR merge. Only the *metadata record*
(change file, spec, ADRs) is force-copied by terminal-publish. So the need is narrow: let a repo
suppress the metadata copy while keeping everything else.

## Solution overview

A new per-repo configuration knob, `terminal_publish`, defaulting to `true` (today's behavior).
Setting it to `false` in a repo's committed `.docket.yml` makes the terminal-publish step a no-op:
the archived change file, its spec, its Accepted ADRs, and the integration-branch ADR-index
refresh all stay on `docket` only. Everything else in the close-out sequence — archive, artifact
re-render, feature-branch cleanup, board refresh — is unchanged.

### Decisions (settled in brainstorm)

1. **Built-in default `true`.** Preserves today's behavior for every existing repo with no config
   change (docket's standard backward-compatible-opt-out posture, mirroring `metadata_branch`).
   Repos wanting PR-only opt out explicitly with `terminal_publish: false`.

2. **Per-repo-only (coordination-key fenced).** Honored only in the repo-committed `.docket.yml`;
   set in the global `~/.config/docket/config.yml` or the machine-local `.docket.local.yml` it is
   **warned-and-ignored** — the same posture as the `github` board surface (ADR-0019 fence). The
   rationale: terminal-publish is run by four close-out drivers, one of which
   (`docket-status`'s merge sweep) can run headless/cloud where the user's global and machine-local
   config files do not exist. If the knob lived in a machine-scoped layer, that autonomous sweep
   would not see it and would publish anyway, diverging the integration branch from what the human
   intended. Committing the value in `.docket.yml` is the only placement that makes the policy hold
   for **every** agent touching the repo.

3. **`false` suppresses the entire publish step.** Not a partial suppression — change file, spec,
   Accepted ADRs, and the main-branch ADR-index refresh are all skipped. Matches the "keep
   everything on the docket branch" intent and adds no special-casing to the publish script.

4. **Gate lives in `terminal-publish.sh`.** All four close-out drivers already funnel through this
   one script, which already self-guards to a no-op in `main`-mode. Adding the knob-guard there —
   rather than branching in each of the four skill bodies — covers every caller with a single
   guard and keeps the skill prose unchanged. The script stays pure and testable: it receives the
   value as an explicit `--enabled <true|false>` flag rather than sourcing config itself.

5. **The gate covers BOTH publish shapes — `--id` and `--adr` (added at reconcile, 2026-07-12).**
   `terminal-publish.sh` is the executor of two shapes: the close-out change publish (`--id`) and
   `docket-adr`'s standalone ADR publish (`--adr`, used both when an ADR is Accepted and when an
   already-published ADR's `status:` flips). Both write metadata directly onto the integration
   branch outside the PR gate — so both are in the knob's remit, and gating only the close-out
   shape would leave `docket-adr` still committing ADRs to `main`, contradicting this change's own
   promise that "Accepted ADRs stay on `docket` only". The guard is therefore placed **before the
   `--id`/`--adr` mode dispatch**, beside the existing `main`-mode guard, so one guard covers both
   shapes; the two `docket-adr` call sites pass `--enabled "$TERMINAL_PUBLISH"` like every close-out
   driver.

## What is and isn't affected

**Affected (suppressed when `false`):**

- terminal-publish's copy of the archived change file onto the integration branch
- terminal-publish's copy of the `spec:` file onto the integration branch
- terminal-publish's copy of the `Accepted` ADRs in `adrs:` onto the integration branch
- the integration-branch ADR-index refresh that rides the publish commit (changes 0033/0040)
- **`docket-adr`'s standalone `--adr` publish** — both the on-acceptance publish and the
  status-change (`Superseded by`/`Reversed by`/`Deprecated`) re-publish (reconcile, 2026-07-12).
  Same rationale: it is a direct commit of metadata onto the integration branch. Leaving it live
  would defeat the knob — ADRs would keep landing on `main` even with `terminal_publish: false`.

**Not affected (unchanged regardless of the knob):**

- The feature branch's **code + `plan:` + `results:`** reaching the integration branch via the
  normal PR merge — the knob never touches the PR flow.
- Close-out steps 1–2 (archive on `docket`, re-render `## Artifacts`) and 4–5 (feature-branch
  cleanup, board refresh) — all still run on `docket` exactly as today.
- `sync-integration-branch.sh`'s post-merge FF of the local integration checkout — that follows
  the PR merge, not the metadata publish.

**Resulting end-state in a `terminal_publish: false` repo:** the integration branch accumulates
code / plans / results through PRs; change files, specs, and ADRs live on `docket` only. This is
the deliberate, documented consequence — the metadata backlog remains fully browsable on
`origin/docket`, and a human may still merge `docket` → integration manually if they ever want the
records there.

## Mechanism (component by component)

### `docket-config.sh` + contract (`scripts/docket-config.md`)

- Read the `terminal_publish` leaf key from the resolved config; default `true`.
- Classify it as a **coordination key**: add it to the fence set alongside `metadata_branch`,
  `integration_branch`, `changes_dir`, `adrs_dir`, `results_dir`, `github_project`. When present in
  the global layer or `.docket.local.yml`, warn ("per-repo-only", naming the file) and ignore —
  reusing the existing Stage 2b/2c fence machinery, not a new code path.
- Emit `TERMINAL_PUBLISH=true|false` in `--export`.
- Update the classification table in `scripts/docket-config.md`: new row, **Global-able = no**.

### `terminal-publish.sh` + contract (`scripts/terminal-publish.md`)

- New flag `--enabled <true|false>`, default `true` (back-compat: omitting it behaves exactly as
  today). An unparseable value is a hard `die` (fail-closed, consistent with the script's other
  argument validation) — never a silent coerce to `true`.
- When `--enabled false`: no-op with a clear, single log line explaining the skip — a second guard
  beside the existing `main`-mode no-op guard, placed **before the `--id`/`--adr` mode dispatch** so
  it covers both publish shapes. Exit `0` (a suppressed publish is success, not failure).
- Argument *validation* still runs before the guard, so a malformed call fails loudly whether or not
  publishing is enabled — a disabled publish must not mask a broken call site.
- Update the `.md` contract's Usage / Behavior / Exit-codes / Invariants sections.

### `references/terminal-close-out.md`

- Document step 3 as **gated by `TERMINAL_PUBLISH`**, passed to `terminal-publish.sh` as
  `--enabled "$TERMINAL_PUBLISH"`. Note that a suppressed publish is a success (exit 0) and does
  not trip the skip-publish guard on steps 4–5.

### `docket-adr` SKILL.md (added at reconcile, 2026-07-12)

- Both `--adr` publish call sites (the on-acceptance publish and the status-change re-publish) pass
  `--enabled "$TERMINAL_PUBLISH"`, so the knob holds on the ADR path too.
- Add one sentence noting that under `terminal_publish: false` the ADR ledger lives on `docket`
  only — the integration branch carries no ADR files and no ADR index.

### `docket-convention` SKILL.md + `docket-config.md`

- Add `terminal_publish: true` to the documented `.docket.yml` schema block, with an inline note
  that it is per-repo-only.
- Add one sentence in the Branch model / terminal close-out prose describing the opt-out and its
  end-state.

### `main`-mode

- The knob is **inert** in `main`-mode: terminal-publish is already a no-op there (the metadata
  working tree *is* the integration branch). Documented as inert; no warning is emitted for setting
  it in a `main`-mode repo (it simply has no surface to act on).

## ADR

This change (a) classifies a new key under the ADR-0019 coordination-key fence and (b) introduces a
conditional-publish rule on the sole metadata-onto-integration flow. That is an architecture
decision worth recording — an ADR authored at **build time** via `docket-adr` (per the normal
docket-implement-next flow), not minted in this proposal. Expected to relate to ADR-0019 (fence
classification) and ADR-0012/0013 (script-vs-model boundary — the guard living in the script).

## Testing

- **`docket-config.sh`:** `TERMINAL_PUBLISH=false` when set in the repo `.docket.yml`;
  `TERMINAL_PUBLISH=true` **with a warning** when set only in the global `config.yml` or
  `.docket.local.yml`; `TERMINAL_PUBLISH=true` when unset anywhere.
- **`terminal-publish.sh`:** `--enabled false` leaves the integration branch untouched and exits 0
  (asserted against a fixture repo); `--enabled true` (and omitting the flag) publishes exactly as
  today; an unparseable `--enabled` value exits non-zero.
- **`terminal-publish.sh --adr` (reconcile, 2026-07-12):** `--adr N --enabled false` leaves the
  integration branch untouched and exits 0 — the guard covers the ADR shape, not just `--id`.
- **Close-out integration:** a `done` close-out in a `terminal_publish: false` fixture archives on
  `docket`, refreshes the board, and cleans up — with no new commit on the integration branch.
- **Call-site wiring (reconcile, 2026-07-12):** a structural check that every `terminal-publish.sh`
  invocation in the skill/reference prose passes `--enabled` — the close-out reference's step 3 and
  both `docket-adr` sites — so a future call site can't silently reintroduce an ungated publish.

## Out of scope

- Per-artifact granularity (e.g. "suppress the change file but still publish ADRs") — explicitly
  rejected in brainstorm in favor of all-or-nothing.
- Any change to how code / plans / results reach the integration branch (the PR flow).
- A retroactive un-publish of records already copied to the integration branch by prior runs.
- Making the knob settable from the global or machine-local layers — deliberately fenced out.
