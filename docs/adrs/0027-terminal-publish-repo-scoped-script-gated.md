---
id: 27
slug: terminal-publish-repo-scoped-script-gated
title: terminal_publish — a per-repo coordination key, gated once inside the script
status: Accepted
date: 2026-07-13
supersedes: []
reverses: []
relates_to: [12, 19]
change: 64
---

## Context

Terminal-publish is docket's only write from the metadata branch onto the integration branch: on a
change's terminal transition it copies the archived change file, its spec, and the `Accepted` ADRs
in `adrs:` from `origin/docket` onto `origin/<integration_branch>`; `docket-adr` performs the same
write for a standalone or status-changed ADR. Some repos do not want it — a team whose integration
branch is protected, or whose reviewers do not want machine commits on the code line, wants the
record to stay on `docket`. Change 0064 adds the per-repo `terminal_publish` knob (default `true`)
to suppress it.

Two questions had non-obvious answers.

**Where may the knob be set?** It reads like a personal preference — "I don't want docket committing
to main *on my machine*" — which would put it in the global `~/.config/docket/config.yml` or the
machine-local `.docket.local.yml`, the layers ADR-0019 opens to workflow and policy knobs.

**Where does the gate live?** Four drivers reach the publish (the two close-out drivers,
`docket-finalize-change` and `docket-status`'s merge sweep; the kill origin; and `docket-adr`'s
standalone/status-change ADR publish), so the knob could be branched on in each calling skill body.

## Decision

**`terminal_publish` is a coordination key — per-repo-only, honored solely in the committed
`.docket.yml`, warned-and-ignored in both machine-scoped layers.** This is ADR-0019's rule applied,
not extended: the write it governs lands on a shared branch and is not re-derivable, so it is fenced
like `metadata_branch`. The decisive fact is that publishing is *not* driven only from a developer's
machine — the headless `docket-status` merge sweep may run on CI or another clone, where machine-scoped
files do not exist. A machine-scoped value would therefore make the repo publish or not publish
depending on which agent happened to run the sweep, splitting the integration branch's record. **A
policy over shared history must be a property of the repo, seen identically by every agent.** The
general rule for classifying any future knob: ask not whether the value *feels* personal, but whether
its effect writes state another agent will read.

**The gate lives in `terminal-publish.sh`, before the `--id`/`--adr` mode dispatch, and a suppressed
publish is SUCCESS.** The script takes `--enabled <true|false>`; when false it returns a clean no-op —
exit 0, no fetch, no temp worktree, no commit, no push. One guard placed ahead of the mode dispatch
covers *both* publish shapes (the change close-out and the standalone ADR publish). Every call site
passes the flag unconditionally and trusts the exit code; **no skill branches on the knob** — the
ADR-0012 script-vs-model boundary, where deterministic mechanics live in scripts rather than in agent
prose that each of four drivers would have to re-derive identically. Exit 0 is load-bearing: all four
drivers treat a non-zero publish as an abort, so a suppressed publish that failed would read as a
failed close-out. Invalid `--enabled` values are **fail-closed** (the script dies rather than
defaulting to publish — a typo must never write to a branch the repo asked docket to keep off).

## Consequences

A developer cannot locally opt out of publishing; that is the intended trade — per-repo consistency
beats per-machine convenience for a key that writes shared history. Adding the knob costs no new
branch in any skill body, and a fifth driver would inherit the gate for free.

Because a single ungated call site would silently publish under `terminal_publish: false` — the exact
failure the knob exists to prevent — the invariant is enforced by a build-failing sentinel rather than
by review: `find_ungated_terminal_publish_call_sites` (tests/test_closeout.sh) fails if any
`terminal-publish.sh` invocation omits `--enabled`. A wiring invariant that cannot be seen in one file
must be asserted by a test.

The knob is **inert in `main`-mode** (the metadata working tree *is* the integration branch, so there
is nothing to publish) and **non-retroactive**: records already published stay put; flipping it to
`false` only stops future publishes, and flipping it back does not backfill. A repo that runs with
`terminal_publish: false` has an integration branch that is deliberately not a complete record — the
`docket` branch is the only place the full ledger exists, and readers on the code line must be
expected to look there.
