# terminal_publish opt-in default — design

**Date:** 2026-07-16
**Change:** #0084
**Status:** validated with Daniel (brainstorm 2026-07-16)

## Problem

`terminal_publish` defaults to `true`: on every terminal transition (and every `docket-adr`
publish) docket writes **directly to the integration branch**. In practice this default is the
wrong fail-open posture:

- On a protected or PR-only integration branch the direct push trips branch protection —
  sometimes landing with a warning, sometimes failing.
- Auto-mode permission classifiers deny the push mid-run in autonomous sessions, hard-stopping
  loops (see LEARNINGS entries around finalize/ADR publishes).
- A failed publish can gap **silently** — change #0083 documents #0043's terminal record never
  reaching `main` for eight days with no health check noticing.

Publishing machine commits onto the code line should be a conscious, per-repo opt-in, not
something a repo gets because it never set a key.

## Decision summary (brainstormed)

1. **Built-in default flips to `false`** — publish becomes opt-in.
2. **This repo pins `true`** in its committed `.docket.yml` — its archive-parity practice
   (records mirrored on `main`) is unchanged, now explicit.
3. **Migration posture: docs-only** — no runtime notice when the key is unset; repos wanting
   the old behavior add `terminal_publish: true`.
4. **New ADR** records the flip (relates_to ADR-0027, back-linked `change: 84`). ADR-0027
   (per-repo fencing, script-gating) stays Accepted — it decided *where* the knob may be set
   and *where* the gate lives, not the default value.
5. **Amendment (Daniel):** `terminal-publish.sh` invoked with **no `--enabled` flag at all**
   must no-op **loudly** — a prominent stderr warning that nothing was published — because a
   caller that forgot the flag is a bug, not a decision. An explicit `--enabled false` stays a
   silent, intentional no-op (exit 0), exactly as today.

## Code changes (3 sites — every fallback flips fail-safe)

1. **`scripts/docket-config.sh`** — `TERMINAL_PUBLISH="${TERMINAL_PUBLISH:-true}"` →
   `"${TERMINAL_PUBLISH:-false}"`. The `true|false` validation and the coordination-key fence
   are unchanged.
2. **`scripts/terminal-publish.sh`** — replace `ENABLED="true"` with an *unset* sentinel:
   - `--enabled true` → publish (unchanged).
   - `--enabled false` → silent no-op, exit 0 (unchanged).
   - **flag omitted** → no-op, exit 0, plus a prominent stderr warning, e.g.:
     `WARNING: terminal-publish: --enabled not passed — defaulting to DISABLED; NOTHING was published. Pass --enabled true (from the resolved TERMINAL_PUBLISH) to publish.`
     Exit 0 is deliberate: skill callers trust the exit code, and an omitted flag must not
     abort a close-out — but the warning makes the skipped publish impossible to miss in run
     output (the silent-gap failure mode of #0083).
3. **`scripts/docket-status.sh`** (sweep, ~line 389) — `${TERMINAL_PUBLISH:-true}` →
   `${TERMINAL_PUBLISH:-false}`.

## Documentation changes

Invert the framing everywhere current docs say "default true / set false to opt out":
publish is now **opt-in**, and `true` carries a risks callout (direct pushes to a protected
branch, machine commits on the code line, classifier denials in autonomous runs, silent gaps
per #0083).

- **`README.md`** — config sample block default; rewrite the section
  "Keeping metadata off the integration branch (`terminal_publish`)" as
  "Publishing terminal records to the integration branch (`terminal_publish`, opt-in)" with
  the explicit risks-of-`true` callout.
- **`scripts/docket-config.md`** — default column `true` → `false`; behavior text inverted.
- **`scripts/terminal-publish.md`** — `--enabled` contract: new default `false`, the
  omitted-flag warning, the silent explicit-`false` no-op.
- **`skills/docket-convention/SKILL.md`** — the `.docket.yml` sample comment and the Branch
  model paragraph (`terminal_publish: false` is now the default; `true` = opt-in copy).
- **`skills/docket-convention/references/terminal-close-out.md`** — default mention.
- **`skills/docket-adr/SKILL.md`, `skills/docket-finalize-change/SKILL.md`,
  `skills/docket-status/SKILL.md`** — any "default true" mentions.
- **`.docket.yml` (this repo)** — uncomment the key to `terminal_publish: true` and adjust
  its comment (now an explicit opt-in, not "the default stands").

Historical artifacts are untouched: archived changes (0064, 0065), old specs/plans/results,
and ADR-0027's text stay as written.

## ADR

One new ADR: *terminal-publish default flips to opt-in* — `relates_to: [27]`, `change: 84`.
Context: the fail-open default, the classifier/branch-protection friction, the #0043 silent
gap. Decision: unset ⇒ `false` at every layer (config resolver, script flag fallback, sweep
fallback); publish requires the committed `.docket.yml` to say `terminal_publish: true`.
Consequences: upgrading repos that relied on the implicit default stop publishing until they
pin `true` (docs-only migration); hand invocations of `terminal-publish.sh` need an explicit
`--enabled true`.

## Tests

Update the four suites that assert the current default; add coverage for the new behaviors:

- **`tests/test_docket_config.sh`** — unset key resolves `TERMINAL_PUBLISH=false`; explicit
  `true`/`false` honored; invalid value still dies; fence warnings unchanged.
- **`tests/test_terminal_publish.sh`** — omitted `--enabled`: no-op, exit 0, warning on
  stderr; explicit `false`: silent no-op; explicit `true`: publishes.
- **`tests/test_closeout.sh`** / **`tests/test_docket_status.sh`** — fixtures that relied on
  the implicit default now pass/pin `--enabled true` (or set the key in the fixture's
  `.docket.yml`) so they keep testing the publish path; add/keep one case proving the
  unset-key sweep does NOT publish.

## Out of scope

- Change #0083's gap-detection health check (separate change; this flip narrows its blast
  radius but does not replace it).
- Any PR-based publish mechanism (publish-via-PR instead of direct push) — possible future
  work if a `true` repo wants records without direct pushes.
- Retroactive behavior: the knob remains never-retroactive (ADR-0027) — already-published
  records stay where they are.
