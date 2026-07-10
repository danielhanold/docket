# Architecture Decision Records

Immutable, numbered record of *why*. ADRs are never archived or rewritten; once `Accepted`, only the `status:` line changes (on supersession/reversal). This index is generated — do not hand-edit.

## Active

- [ADR-0001](0001-docket-metadata-branch-model.md) — Planning metadata on an orphan docket branch; publish terminal records by copy, not merge (Accepted) ← change #2
- [ADR-0002](0002-docket-mode-default-and-bootstrap.md) — docket-mode is the default; refuse-and-migrate bootstrap; terminal-publish single-sourced in finalize (Accepted) ← change #2 · relates to ADR-0001
- [ADR-0003](0003-convention-reference-loading.md) — The docket convention is reference-loaded from a docket-convention skill, not embedded per skill (Accepted) ← change #5 · relates to ADR-0002
- [ADR-0004](0004-grooming-takes-no-claim.md) — Grooming takes no claim — final-push CAS suffices for human-attended sessions (Accepted) ← change #12 · relates to ADR-0001
- [ADR-0005](0005-close-out-only-harvest.md) — Learnings are harvested only at close-out — one writer, one moment, ledger unpublished (Accepted) ← change #6 · relates to ADR-0001, ADR-0003
- [ADR-0006](0006-autonomous-grooming-bounds.md) — Autonomous grooming bounds — critic gates every build-ready exit; kill/defer never autonomous (Accepted) ← change #14 · relates to ADR-0004
- [ADR-0007](0007-github-board-mirror-boundary.md) — GitHub board mirror — one-way, change-files-authoritative, driven by a deterministic script (Accepted) ← change #11 · relates to ADR-0001
- [ADR-0008](0008-agent-layer-generated-subagents.md) — Agent layer — generated subagent wrappers from layered config (Accepted) ← change #16 · relates to ADR-0001, ADR-0003
- [ADR-0009](0009-auto-groom-critic-isolation.md) — Auto-groom critic isolation — the adversary loads only the convention (Accepted) ← change #17 · relates to ADR-0008
- [ADR-0010](0010-finalize-merge-gate-split-agents.md) — Finalize merge gate — split conflict-resolution from semantic-repair at the rebase-completion boundary (Accepted) ← change #15 · relates to ADR-0008, ADR-0009
- [ADR-0011](0011-finalize-consent-model.md) — Finalize consent model — ambiguity-only prompt + `require_pr_approval` policy gate (Accepted) ← change #21 · relates to ADR-0010
- [ADR-0012](0012-docket-status-script-vs-model-boundary.md) — docket-status script-vs-model boundary for skill passes (Accepted) ← change #23 · relates to ADR-0007
- [ADR-0013](0013-adr-0012-boundary-extends-to-docket-adr-surface.md) — ADR-0012's script-vs-model boundary extends to the docket-adr surface (Accepted) ← change #30 · relates to ADR-0012, ADR-0007, ADR-0002
- [ADR-0014](0014-consuming-repo-script-resolution.md) — Consuming-repo script resolution via `DOCKET_SCRIPTS_DIR` (Accepted) ← change #34 · relates to ADR-0012
- [ADR-0015](0015-harness-portable-agent-config.md) — Harness-portable agent model config — direct model IDs, per-repo generation to an explicit harness list (Accepted) ← change #45 · relates to ADR-0008, ADR-0001
- [ADR-0016](0016-harness-first-agent-config.md) — Harness-first `agents:` config — per-harness model/effort with field-level default fallback (Accepted) ← change #46 · relates to ADR-0015, ADR-0008
- [ADR-0018](0018-pluggable-skills-passthrough-degrade.md) — Pluggable workflow skills — unvalidated skill-name passthrough + degrade-to-auto (not abort) on a missing skill (Accepted) ← change #49 · relates to ADR-0015
- [ADR-0019](0019-global-config-fence-classification.md) — Global config layer — the coordination-key fence classification rule (Accepted) ← change #50 · relates to ADR-0008, ADR-0015, ADR-0016
- [ADR-0020](0020-generated-agent-artifacts-machine-local.md) — Generated agent artifacts are machine-local, never committed; `.docket.local.yml` completes the four-layer config (Accepted) ← change #51 → supersedes ADR-0017 · relates to ADR-0015, ADR-0019

## Superseded / Reversed

- [ADR-0017](0017-cursor-dispatch-rule-full-agent-set.md) — Per-repo agent generation goes always-full-set, opt-in, with a Cursor dispatch rule (Superseded by ADR-20) ← change #48 · relates to ADR-0015, ADR-0016

## Deprecated

_None._
