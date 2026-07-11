# Spec — docket coexists with git-hook frameworks

**Change:** 0063
**Date:** 2026-07-11
**Status:** approved (brainstormed with the human, 2026-07-11)

## Problem

Docket is unusable in a repo that uses a git-hook framework (pre-commit.com, husky, lefthook, …) with a `pre-commit` hook. Git hooks are **shared across all worktrees** via the common git dir, and docket **never skips hooks** on any commit. So every one of docket's many machine-generated bookkeeping commits into the `.docket/` worktree — which sits on the **orphan `docket` branch** — fires the repo's shared `pre-commit` shim, which hard-fails because the orphan branch has no `.pre-commit-config.yaml`:

```
$ git commit -m "claim(0061): ..."
No .pre-commit-config.yaml file was found
Check the README ...            (exit 1)
```

The failure was observed in practice (Cursor + a pre-commit repo): a metadata commit was blocked, and the agent only recovered because it happened to catch the failure and improvise a per-commit env-var workaround (pre-commit's `PRE_COMMIT_ALLOW_NO_CONFIG=1`). That recovery is fragile and non-deterministic — a helper-script commit, or an autonomous run that does not catch the failure, simply hard-fails. Docket has **no systematic handling** today (verified: no `--no-verify`, `core.hooksPath`, or hook logic anywhere in `scripts/`, `skills/`, or the top-level `*.sh`).

Root cause is **not** worktree hook *initialization* — hooks are shared, not per-worktree. It is a shared hook running against a branch (and commits) it was never meant to guard.

## Approach

**Disable git hooks on the `.docket` metadata worktree, once, at creation/ensure time** — scoped so nothing else changes:

```
git config extensions.worktreeConfig true                 # common config; local, idempotent
git -C .docket config --worktree core.hooksPath <ABS-EMPTY-DIR>
```

`core.hooksPath` set **worktree-scoped** (in `.git/worktrees/.docket/config.worktree`, enabled by `extensions.worktreeConfig`) points the `.docket` worktree's hook lookup at an empty, docket-owned directory. Every commit into `.docket` — from a **helper script** or the **agent following skill prose** — then finds no hooks and proceeds. This is **enforced by construction**: there is no per-commit flag for a script or an agent to forget (the exact failure path already seen). It is **framework-agnostic**: it disables the hook *mechanism*, not one framework's config.

The main working tree and every feature worktree are **untouched** — their commits still run the team's hooks. This satisfies the scope decision below.

### Scope — metadata bookkeeping only (human decision)

- **Skip hooks:** docket's own bookkeeping commits — every commit on `metadata_branch` via the `.docket` worktree (claim, reconcile, `status`, board, ADR, spec, `plan:`/`adrs:`/`pr:`/`results:` field writes, artifacts re-render), **plus** the `terminal-publish` doc-publish commit (below).
- **Keep hooks:** feature-branch **code** commits during autonomous builds (plan/results/code on `feat/<slug>`). These are real code headed to a PR; the team's code-quality gates must still fire. The feature branch is cut from `origin/<integration_branch>`, which carries the real `.pre-commit-config.yaml`, so hooks resolve normally there — no change needed, and none made.

### The `.docket` create/ensure sites

The hook-disable is applied at **every** site that creates or ensures the `.docket` worktree, **idempotently** — so it is set for fresh installs and **self-heals** existing `.docket` worktrees on the next docket run:

- `scripts/docket-status.sh` → `ensure_and_sync_worktree()` (the shared ensure path).
- `migrate-to-docket.sh` (each `.docket` worktree-add path).
- The convention **Step-0 preamble** "ensure + sync the metadata working tree" — so interactive skills (`docket-new-change`, `docket-groom-next`) that ensure the worktree inline also apply it.

> **Reconcile refinement (2026-07-11):** `scripts/docket-config.sh --bootstrap` was listed here, but `create_orphan()` is **worktree-free** (builds the orphan via `commit-tree` + push; no `.docket` worktree exists yet), so a worktree-scoped `core.hooksPath` has nothing to attach to at bootstrap. The `.docket` worktree is created immediately afterward by the Step-0 preamble / `ensure_and_sync_worktree` — a helper site that self-heals — so **bootstrap needs no helper call**. Worktree-creation helper sites are `docket-status.sh` and `migrate-to-docket.sh` only.

To keep this DRY and single-sourced, factor the disable into one small idempotent helper — proposed `scripts/disable-worktree-hooks.sh` (with a co-located `scripts/disable-worktree-hooks.md` contract, per docket's script-contract convention) — that every site calls after `worktree add`. The helper owns: creating the empty hooks dir, enabling `extensions.worktreeConfig`, and setting the worktree-scoped `core.hooksPath`; it is a no-op when already applied.

### Terminal-publish's bookkeeping commit (human decision: include)

`terminal-publish` copies the archived change file + its `spec:` + `Accepted` ADRs onto the integration branch through a **temporary** worktree (`pub-$T`, added from `origin/<integration_branch>`) and commits there. That branch **has** the team's config, so the shared hook would run and could block the publish — the same class of failure at change close-out. Because the worktree-config trick does not reach an ad-hoc temp worktree, skip hooks on that specific commit **per-invocation**:

```
git -C "$pub" -c core.hooksPath=<ABS-EMPTY-DIR> commit -m "<message>"
```

This is docket bookkeeping (its own docs), not the team's code, consistent with the scope decision.

> **Reconcile refinement (2026-07-11):** terminal-publish is **not** a single commit site — beyond the L147 publish commit it **replays via `rebase --continue`** inside the CAS push-retry loop (L150-156), which a lone per-invocation `-c core.hooksPath` on the L147 commit would not cover. Preferred mechanism: apply the **worktree-scoped disable** (reusing `disable-worktree-hooks.sh`) to the transient `pub-$T` worktree right after `git worktree add` (L119), so **every** commit in it — publish and rebase replay alike — skips hooks; it is torn down with the worktree. Confirm exact wiring at plan time.

### The empty hooks directory

An **absolute** path to an empty, docket-owned directory inside the git common dir (e.g. `"$(git rev-parse --git-common-dir)/docket/empty-hooks"`), created idempotently. Living under `.git/`, it is never tracked and never leaks into a commit. An absolute path avoids `core.hooksPath` resolving relative to a worktree root. A real (empty) directory — rather than a nonexistent path — avoids any "hooksPath does not exist" surprises in git or in the framework's own `core.hooksPath` checks.

## Safety — `extensions.worktreeConfig`

Enabling `extensions.worktreeConfig` is a **local, idempotent** `.git/config` change (git ≥ 2.20, 2018). Its one documented caveat: after it is enabled, `core.worktree` and `core.bare` are read **per-worktree**, so a value pre-existing in the **common** config would silently stop applying to linked worktrees. In virtually all normal repos these are unset. The helper handles this defensively: **detect** a pre-existing common-config `core.worktree`/`core.bare` before enabling and, per git's own guidance, relocate it to the main worktree's `config.worktree`; if that cannot be done safely, **warn loudly** with remediation guidance rather than proceed blindly. (Plan-time: whether to hard-degrade to per-invocation `-c core.hooksPath` on the rare unsafe path, accepting it does not cover agent-prose commits, is a plan decision — see Open questions.)

The change is **local-only**: it never touches the remote, teammates' clones, or the committed `.docket.yml`. It is not a coordination key.

## Harness-agnostic

The fix is entirely at the git layer, so it serves **Cursor, Claude Code, and Codex** identically. Nothing harness-specific is added; the reporter's Cursor setup is incidental to the root cause.

## Files touched (approximate — the plan finalizes)

- **New:** `scripts/disable-worktree-hooks.sh` + `scripts/disable-worktree-hooks.md` (the idempotent helper + contract).
- `scripts/docket-status.sh` — call the helper in `ensure_and_sync_worktree()` after worktree add.
- `migrate-to-docket.sh` — call the helper at each `.docket` create path.
- `scripts/terminal-publish.sh` — apply the helper (worktree-scoped disable) to the transient `pub-$T` worktree after `worktree add`, covering both the publish commit and the rebase-continue replay. (Reconcile refinement — supersedes the earlier per-invocation `-c` on `docket-config.sh --bootstrap`, dropped as worktree-free.)
- `skills/docket-convention/SKILL.md` — Step-0 preamble + Branch-model: note that ensuring the `.docket` worktree also disables its hooks (single-sentence, pointing at the helper).
- `README.md` — a short "git-hook frameworks" note (docket's bookkeeping commits skip your hooks; your code commits still run them).
- **Tests:** `tests/test_metadata_worktree_hooks.sh` (below); `tests/test_script_contracts_coverage.sh` picks up the new contract automatically.

## Testing

A **hermetic** test (repo convention: standalone `bash tests/test_*.sh`, `assert` helper, PASS/FAIL + exit code):

1. Build a throwaway git repo with a `.docket`-style second worktree on an orphan branch, and install an **always-failing** hook (`exit 1`) in the common hooks dir.
2. Assert a commit in the **main** working tree **fails** (hook runs) — proving the hook is real and the disable is not global.
3. Apply the helper; assert a commit in the **metadata** worktree **succeeds** (hook skipped).
4. Assert the disable is **worktree-scoped**: after the helper, the main working tree's commit still **fails** (hooks not globally disabled).
5. Assert **idempotence**: running the helper twice is a clean no-op (exit 0, no duplicate config).
6. Non-vacuousness: removing the helper call flips step 3 back to a failing commit.

The `terminal-publish` per-invocation skip is covered by asserting its commit path carries `-c core.hooksPath` (structural) — a full publish-with-failing-hook integration test is optional if cheap.

## Out of scope

- Feature-branch **code** commits — keep running the team's hooks (human decision).
- Framework-specific handling (`PRE_COMMIT_ALLOW_NO_CONFIG`, `SKIP=…`, husky/lefthook config) — the mechanism-level disable is framework-agnostic and needs none.
- Any change to which commits docket makes, or to the branch model (ADR-0001) itself.
- Configurable per-repo hook policy (a `.docket.yml` `hooks:` knob) — YAGNI; metadata bookkeeping never wants the team's hooks, and feature code always does. Revisit only if a real need appears.

## Open questions

- **Unsafe-`worktreeConfig` degrade path.** In the rare case a pre-existing common-config `core.worktree`/`core.bare` blocks safe enablement, does docket (a) relocate them per git's guidance and proceed, or (b) warn and degrade to per-invocation `-c core.hooksPath` on script commit sites only (not agent-prose commits)? Lean (a); confirm at plan time.
- **README placement.** Standalone "git-hook frameworks" subsection vs. a note folded into the migration / branch-model section. Cosmetic; decide at plan time.
