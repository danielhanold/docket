---
id: 351
slug: complete-0334-retire-global-instruction-writes-and-deploy-recursion-guard
title: "Complete change 0334: stop writing global instruction files and actually deploy the recursion guard"
status: proposed
priority: high
type: fix
created: 2026-08-26
updated: 2026-08-26
depends_on: []
stacked_on:
related: [334, 294]
discovered_from: [334]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0334 ("Make Docket dispatch minimal, non-recursive, and mechanically gated") merged, but
two parts of its stated scope did not actually land in a real install. Both were found while
verifying 0334 after merge (2026-08-26).

**1. Docket still writes personal GLOBAL instruction files.** 0334 §5 states plainly:
*"Docket will not inspect, edit, truncate, or delete `~/.claude/CLAUDE.md`,"* leaving the generated
**per-repository** surface authoritative. The spec framed the global block as hand-authored and its
removal as a manual maintainer step. That premise is false: the live installer **emits** a
docket-managed dispatch block into the user-level instruction files.
- `internal/harness/claude/claude.go:32` — `dispatchFile = "CLAUDE.md"`; lines ~123-129 emit a
  `roleDispatch` target at `Path: filepath.Join(root, dispatchFile)` with annotation
  `managed by docket — do not hand-edit`, where `root` is the user home
  (`internal/cli/install.go:174` → `ResolveRoots(os.UserHomeDir, …)`).
- The same pattern in the codex / cursor / opencode adapters writes `~/.codex/AGENTS.md`,
  `~/.cursor/rules/docket-dispatch.mdc`, `~/.config/opencode/AGENTS.md`.
- Confirmed by an isolated install into a temp `HOME`: docket creates `.claude/CLAUDE.md` carrying
  a `docket:dispatch` block. So manually deleting the block never sticks — the next
  `docket development install` re-writes it. That is exactly the reported symptom.

Consequence: docket edits the user's personal, cross-repo instructions (the thing §5 forbade), and
the global `~/.claude/CLAUDE.md` still carries the full 17-agent roster (its content had not been
re-synced to the compact rule; the per-repo `<repo>/CLAUDE.md` IS compact).

**2. The recursion guard — 0334's actual purpose — is not deployed to already-installed wrappers.**
0334 exists to kill self-recursion (0317's three-deep `docket-status` dispatch tree). The fix is
correct in code and fully tested: `internal/harness/guard.go`'s `RecursionGuard` is wired into all
four adapters (`claude.go:191`, `codex.go:178`, `cursor.go:249`, `opencode.go:205`), all 17 claude
goldens carry it, and `tests/test_sync_agents_recursion_guard.sh` guards it. A **fresh** install
renders it correctly.

But the **live** installed wrappers do not have it:
- `~/.claude/agents/docket-*.md`: 0 of 17 contain the guard paragraph. A diff of the live
  `docket-status.md` against the current binary's fresh render shows the guard is the *only*
  missing content.
- The live wrapper's mtime is `10:56 UTC`, **before** the 0334 merge (`12:33 UTC`). A post-merge
  install *did* run — it recorded the new `asset_set_id` (`sha256:36fcca…`) and refreshed the
  dispatch instruction files + binary — but it **did not re-render the agent wrappers**.

Net: on any machine that already had docket installed before 0334, the recursion guard ships in the
binary and the tests but never reaches the files that run. The bug 0334 set out to fix is still
live for existing installs.

**How 294 framed the adjacent scope (context).** Change 0294 (killed, absorbed into 0334) owned the
"shrink the always-loaded dispatch footprint" work. It flagged that the dispatch table restates
each agent's `description:` verbatim, duplicating what harnesses like Claude Code inject natively —
and it was explicit that dropping the per-agent descriptions must be **verified per harness**
(claude, cursor, codex, opencode) *before dropping anything*, because the compact rule only works
where the harness surfaces agent descriptions natively. That per-harness verification and the
seeding question below carry into this change.

## What changes

- **Stop writing personal global instruction files.** Remove the user-root `roleDispatch`
  (dispatch-block) target from all four harness install plans. Docket keeps installing agent
  *wrappers* and *skills* globally (needed for cross-repo dispatch) but writes **no** dispatch block
  into `~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md`, `~/.config/opencode/AGENTS.md`, or
  `~/.cursor/rules/docket-dispatch.mdc`. The dispatch surface lives per-repository — committed in
  each repo's `CLAUDE.md`/`AGENTS.md`, exactly as docket's own repo already carries it.
- **Seed the per-repo dispatch surface** for a repo adopting docket (likely `migrate-to-docket.sh`,
  or a first-run writer) so the rule is present where docket is actually used.
- **One-time cleanup of the previously-written global block** — have the installer remove a
  docket-managed dispatch block it previously wrote to a personal instruction file (bounded to the
  managed markers), or document the manual removal; decide which during design.
- **Fix the installer so renderer-level changes reach already-installed wrappers.** A change to a
  renderer (like the recursion guard) that does not alter the embedded asset-set hash must still be
  detected and re-rendered into existing installed wrappers. Add a regression test: an install whose
  wrappers lack the guard is updated to include it.

## Out of scope

- The dispatch-rule *semantics* and the recursion-guard *content* — both are correct as written.
  This change is about DEPLOYMENT and about retiring the global writes, not re-designing either.
- The 0349 finalize-resolver cap work (separate change).

## Open questions

- **Split or single change?** The global-write retirement and the installer wrapper-staleness fix
  are distinct failure modes that happen to share the installer. Groom may split them.
- **Root-cause the wrapper-staleness** precisely: confirm whether the installer's up-to-date check
  keys on the embedded asset-set hash (which a renderer-only change like the guard does not bump),
  vs. a per-target content compare that should have caught the diff. `internal/install/inspect.go`
  (`bytes.Equal` at :99; `interiorDigest`/`SHA256` records) and `internal/install/service.go`'s
  asset-set/source-digest drift path (~:282-295) are the places to look.
- **Per-harness verification (from 0294):** which harnesses inject agent descriptions natively, and
  what must the dispatch block retain on those that don't — before the roster is dropped anywhere.
- **Seeding mechanism** for the per-repo dispatch surface in a non-docket repo.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
