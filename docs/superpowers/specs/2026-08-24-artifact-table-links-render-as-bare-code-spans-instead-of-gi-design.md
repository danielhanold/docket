<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0341 — Artifact-table links render as bare code spans instead of GitHub links** — `docs/changes/archive/2026-08-24-0341-artifact-table-links-render-as-bare-code-spans-instead-of-gi.md`
<!-- docket:backlink:end -->

# Artifact-table links render as bare code spans instead of GitHub links — design (change 0341)

## Problem

The generated `## Artifacts` link block on change files (and the reciprocal `docket:backlink`
block on specs/plans/results, and the artifact/PR links in PR bodies) renders artifact paths as
bare inline code spans (`` `docs/…` ``) instead of clickable `https://github.com/…/blob/…` links.
Affected files are all recent: 2 active and 9 archived change files at time of writing, e.g. 0317,
0342, 0339, 0340, 0251, 0330, 0335–0338.

## Root cause (code-proven)

Rendering has been ported to the Go runtime. The pure render package
(`internal/render/link.go`, `internal/render/artifacts.go`) is correct: it emits a blob URL when
its `LinkContext.RepoWebURL` is non-empty and a bare code span when it is empty — faithfully
rendering whatever it is handed, and never deriving the URL itself (an explicit purity contract in
the package doc comment).

The defect is in the **Go app layer that constructs `LinkContext`**. Every one of the ~18
construction sites builds it as `render.LinkContext{MetadataBranch: metadataBranchOf(pin)}` and
**never sets `RepoWebURL`** (change_create, change_groom, change_claim, change_attach,
change_implemented, change_reconcile, change_kill, change_lifecycle, change_reclaim,
finalize_closeout, adr_ops, pr_publish). Nothing in the Go code derives the repository web URL from
the origin remote: `gitcli`'s only `remote get-url origin` call lives inside `ensureRemoteConfigured`,
which checks that a remote is configured and **discards the URL**. There is no web-URL parser
(`git@github.com:owner/repo(.git)` → `https://github.com/owner/repo`) anywhere in the Go tree.

Consequence: every artifact block, backlink, and PR link the `docket` **Go binary** renders comes
out as a bare code span, unconditionally — independent of environment or cwd. The "good" files were
last rendered by the **legacy bash renderers** (`scripts/render-change-links.sh`,
`scripts/render-artifact-backlink.sh`), which do derive origin correctly; those are still what the
`docket.sh render-change-links` / `render-artifact-backlink` facade ops shell to, and what the
grooming skills invoke. The observed "backlink works but artifact table doesn't" asymmetry within a
single change is a per-file coincidence of which runtime ran that file's **last** render (e.g. 0317's
backlink was stamped locally by the bash script at spec-write, while its artifact block was last
rewritten by the Go binary at `→ implemented`); it is not a difference in detection logic — both
runtimes' detection idioms are otherwise equivalent.

## What changes

1. **Derive `RepoWebURL` once in the Go app layer and pass it through every `LinkContext`.**
   - Add a `gitcli` getter that returns the configured URL of a named remote (`git remote get-url
     <remote>`), reading local config only, offline. (Today the URL is fetched only to be thrown
     away inside `ensureRemoteConfigured`.)
   - Add a **pure** web-URL parser converting a GitHub remote URL to its `https://github.com/owner/repo`
     web base, accepting exactly the three forms the bash renderers accept —
     `git@github.com:owner/repo(.git)`, `https://github.com/owner/repo(.git)`,
     `ssh://git@github.com/owner/repo(.git)` — stripping a trailing `.git`. Any non-GitHub or
     unparseable URL yields the empty string (the existing bare-path fallback, unchanged).
   - Resolve the web base **once** (at pin/config resolution for `origin`) and thread it into a
     single shared `LinkContext` constructor (companion to `metadataBranchOf`), replacing all ~18
     inline `render.LinkContext{MetadataBranch: …}` literals so no call site can forget it again.
   - This fixes all three surfaces uniformly — the `## Artifacts` block, the `docket:backlink`
     block, and PR-body links — because all three read `RepoWebURL` from the same context.

2. **One-time re-render sweep of the already-broken files — two channels, because the artifacts
   live on two branches.**
   - **Metadata branch (`docket`).** Re-render the `## Artifacts` block on every change file
     (`active/` + `archive/`) and the `docket:backlink` block on every **spec** file — specs live on
     `metadata_branch` — in one metadata commit on `docket`, pushed. Safe against both the
     sole-writer and frozen-record rules: the renderers are the only sanctioned writers of these
     generated marker blocks (re-rendering an archived change's `## Artifacts` block is what the
     `docket-status` sweep step 6a already does), and only the marker blocks change.
   - **Integration branch (`main`).** Merged **plan** and **results** files live on `main` — they
     are feature-branch build artifacts merged by each change's PR, not metadata — so their
     `docket:backlink` re-stamps **cannot** ride a metadata commit. Re-stamp them on this fix
     change's **own feature branch**, landing on `main` through its PR (the normal channel for
     touching the code line); a direct machine write to protected `main` is never used.
     `render-artifact-backlink.sh` re-stamping a merged artifact's backlink block is the
     convention's one permitted post-merge writer, so this respects the frozen-build-record rule and
     never edits authored content. (`terminal_publish: false` here, so there are no additional
     integration-branch copies of change files or specs to fix.)
   - Because the renderers are same-input→byte-identical, files that were already correct are no-ops.

3. **Regression guard.** Add a Go test at the wiring boundary asserting that, given a GitHub origin
   remote, the app-layer resolver produces a non-empty `RepoWebURL` and the rendered `## Artifacts`
   / backlink output contains blob URLs rather than bare code spans. It must be mutation-tested:
   reverting the `LinkContext` wiring (dropping `RepoWebURL`) reddens it.

## Out of scope

- Redesigning the `## Artifacts` block format or its columns.
- Non-GitHub remote link styles (GitLab, Bitbucket, self-hosted) — the fallback stays bare paths.
- Other renderers (board, ADR index, learnings index) beyond sharing the same GitHub-detection fix
  where they construct a `LinkContext`.
- Changes to the legacy bash renderers, which already behave correctly. (The builder should confirm
  the Go output byte-matches the bash output for the same inputs, but no bash change is expected.)

## Assumptions

Every decision below was defaulted autonomously (no human in the loop); each records the chosen
default, the rejected alternatives, and why.

- **A1 — The fix belongs in the Go app layer, not the pure render package.** Chosen: derive
  `RepoWebURL` in the app layer (new `gitcli` getter + pure parser) and pass it via `LinkContext`.
  Rejected: (a) making the render package impure by having it invoke Git — it has an explicit,
  documented purity invariant, and breaking it would be a far larger architectural change; (b)
  deriving inside `gitcli` — that package returns primitives, not formatted web URLs. The chosen
  path mirrors the existing bash division of labor and the render package's stated contract, so it
  is the conservative choice.

- **A2 — Accept exactly the three GitHub remote forms the bash renderers already accept.** Chosen:
  match `scripts/render-change-links.sh` / `render-artifact-backlink.sh` byte-for-byte
  (`git@…`, `https://…`, `ssh://git@…`, strip `.git`), GitHub-only. Rejected: a broader host-agnostic
  parser (out of scope; would change fallback behavior for non-GitHub repos). Rationale: the two
  runtimes must agree on output, and this repo's remote is GitHub; anything else keeps the existing
  bare-path fallback.

- **A3 — `origin` is the remote, read from local config only.** Chosen: `git remote get-url origin`,
  offline, matching the bash renderers and `ensureRemoteConfigured`. Rejected: probing multiple
  remotes or the network. Rationale: deterministic, offline, and consistent with every existing
  detection site.

- **A4 — Resolve the web base once and thread it through one shared constructor.** Chosen: a single
  `LinkContext` builder (companion to `metadataBranchOf`) so all ~18 sites are fixed at once and a
  future site cannot silently omit the URL. Rejected: patching each site inline (invites the exact
  regression this change fixes). Rationale: the defect is precisely that a construction detail was
  repeated 18 times and forgotten; centralizing removes the failure mode.

- **A5 — The existing files are healed by a one-time sweep, not left to self-heal.** Chosen: an
  explicit sweep, because archived change files and merged artifacts are never rewritten by normal
  lifecycle writes, so the 9 archived files would otherwise stay broken forever. Rejected:
  self-heal-on-next-write (leaves archive permanently broken); hand-editing (violates the
  sole-writer boundary and is not reproducible). Rationale: only the sanctioned renderers touch the
  generated blocks, so the sweep is safe against the frozen-record rule.

- **A6 — The sweep reuses the correct renderers; no new permanent public verb is required.** Chosen:
  invoke the existing per-file render entrypoints over the corpus once (a maintenance invocation).
  Whether that is a throwaway loop or a small internal helper is an implementation choice left to
  the plan; it need not become a documented public `docket` verb. Rejected: minting a permanent
  "re-render everything" command as part of this fix (larger surface than the bug warrants).
  Rationale: keep the blast radius proportional to a rendering bugfix.

- **A7 — The guard is an executable, mutation-tested Go test at the wiring boundary, not a
  committed-file grep.** Chosen: assert the resolver + rendered output for a GitHub origin. Rejected:
  a repo-wide grep over committed files for code-span rows — brittle (false positives on legitimately
  non-GitHub or historical records) and awkward during the transition window before the sweep runs;
  a `docket-status` health check that flags code-span rows is noted as a possible **follow-up**, not
  a blocker. Rationale: AGENTS.md requires guards keyed on shape and mutation-tested; the wiring
  test is the executable form that reddens when the fix is reverted.

- **A8 — Coupling with the Go-runtime cutover (informational, non-blocking).** This change lives
  entirely in the Go runtime introduced by the config-contraction / self-hosting cutover work
  (e.g. 0318) and the Go render port. There is no `depends_on` gate: the Go render path already
  exists and ships, so the fix can land against current `main`. `related: [35]` (the original
  artifact-links change) is retained. No new frontmatter couplings are wired (not instructed to);
  the coupling is recorded here for the human's awareness.

## Open questions

Resolved at design time:

- *Is this trivial?* No — it carries genuine design decisions (architectural placement of the
  derivation, the sweep's interaction with the frozen-record rule, the guard's form). Spec, not
  trivial verdict.
- *Do the bash scripts also need fixing?* No — they already derive origin correctly and are still
  what the `docket.sh` facade ops and grooming skills use. The builder confirms Go/bash output
  parity but changes only the Go side.
- *Does the fix reach backlinks and PR links, not just the artifact table?* Yes — all three read
  `RepoWebURL` from the same `LinkContext`, so the single app-layer fix covers all three.
