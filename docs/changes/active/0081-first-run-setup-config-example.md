---
id: 81
slug: first-run-setup-config-example
title: First-run setup — committed starter config + install.sh scaffolding + README Install restructure
status: in-progress
priority: medium
created: 2026-07-15
updated: 2026-07-15
depends_on: []
related: [50, 80]
adrs: [39]
spec: docs/superpowers/specs/2026-07-15-first-run-setup-config-example-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/first-run-setup-config-example
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-15-first-run-setup-config-example-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-15-first-run-setup-config-example-design.md) |
| ADRs | [ADR-0039](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0039-config-example-mirrors-wrapper-defaults.md) |
<!-- docket:artifacts:end -->

## Why

After a first `git clone` + machine install, docket's setup is not obvious. `install.sh`
links skills, generates agent wrappers, and exports `DOCKET_SCRIPTS_DIR`, but there is no
signposted next step — in particular the global `~/.config/docket/config.yml` that selects
harnesses and tunes per-skill models. Compounding this, docket's **default configuration is
invisible**: the per-harness, per-skill model/effort defaults are real (shipped in each
`agents/docket-*.md` wrapper) and known to the bootstrap, but a first user cannot see what
they are or the shape they'd edit to change them.

This change turns first-run setup into a short, explicit sequence and makes the default
configuration a concrete, editable artifact.

## What changes

- **`config.yml.example` (new, committed at repo root)** — a copy-me template for
  `~/.config/docket/config.yml`. Ships `agent_harnesses: [claude]` active; an
  `agents.claude:` block populated with docket's nine current claude built-in defaults
  (active and explicit, so the defaults are visible and tunable); and commented-out
  `codex:`/`cursor:` blocks carrying the provided example IDs, labeled as unvalidated
  examples to verify + uncomment (and add the harness to `agent_harnesses`) before use.
- **`install.sh` scaffolds the global config** — a new idempotent primitive,
  `scripts/ensure-global-config.sh` (with a co-located `.md` contract), copies
  `config.yml.example` to `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml` **only if it
  does not already exist**, creating the dir as needed and logging which happened; it never
  overwrites an existing file. Wired into `install.sh` as a fourth step (run before
  `sync-agents.sh`).
- **README Install restructure** — the single "The one-line install" subsection becomes a
  short numbered setup sequence: step 1 installs docket on the machine (unchanged one-liner
  + primitives, now naming the config-scaffold step); a new step 2 covers the global config
  (where the defaults are visible, and how to enable a harness: add to `agent_harnesses` +
  uncomment its `agents:` block, then re-run `install.sh`). The change-data / migration note
  stays as the section tail; the full schema stays in the Configuration section (linked, not
  duplicated).

See the linked spec for the exact file layout, the install-step behavior contract, and the
verification cases.

## Out of scope

- Any change to config *resolution* (`docket-config.sh`, four-layer precedence, the
  coordination-key fence). This change only adds a starter artifact + a scaffolding step.
- New config keys, or any change to `sync-agents.sh` generation behavior.
- An interactive install-time wizard (the starter is copy-then-edit, not a prompt flow).
- Automated drift-checking between the shipped `agents/docket-*.md` defaults and the mirror
  in `config.yml.example` — noted as a possible follow-up, not built here.

## Open questions

<!-- None outstanding; resolved during brainstorm. -->

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-15 — reconcile (docket-implement-next)

Verified the spec against current `origin/main`; the design holds unchanged.

- **Nine wrapper defaults match.** All nine `agents/docket-*.md` frontmatter values on
  `origin/main` equal the spec's `agents.claude` mirror (1b): status haiku-4-5/medium; adr
  sonnet-5/medium; brainstorm-consultant, auto-groom, auto-groom-critic, implement-next,
  rebase-resolver, integration-repair all opus-4-8/xhigh; finalize-change sonnet-5/medium.
- **Nothing done elsewhere.** `config.yml.example` and `scripts/ensure-global-config.sh` are
  both absent on `origin/main` — full scope remains to build.
- **`install.sh` structure accurate.** It runs three primitives (link-skills → sync-agents →
  ensure-docket-env); the plan adds `ensure-global-config.sh` before sync-agents (fourth step)
  and updates the header's "runs N primitives" list. Confirmed accurate.
- **README structure accurate.** `## Install` → `### Prerequisites` → `### The one-line
  install`; the Deliverable-3 restructure (rename to step 1, add step 2) matches.
- **ADR-0039 Accepted.** Records the documented-mirror coupling (wrappers are source of truth);
  the build-time equality check enforces it. No automated drift guard in scope.
- **Related changes done.** 0050 (global config layer) and 0080 (link-skills harness dir) both
  `done` — the `~/.config/docket/config.yml` layer and dir-creation behavior this builds on are
  present.
- **In-flight codex work (0077/0079) unmerged, no impact.** The `codex`/`cursor` blocks ship
  commented as unvalidated examples; no dependency on those changes.

No scope drift, no obsolescence, design not invalidated. Proceeding to plan + build.
