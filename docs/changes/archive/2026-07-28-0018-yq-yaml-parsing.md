---
id: 18
slug: yq-yaml-parsing
title: Record the pure-bash YAML/frontmatter parsing stance as an ADR
status: done
priority: low
created: 2026-06-16
updated: 2026-07-28
depends_on: [16]
related: [11, 127]
adrs: [57, 58, 62]
spec:
plan: docs/superpowers/plans/2026-07-28-yq-yaml-parsing.md
results: docs/results/2026-07-28-yq-yaml-parsing-results.md
trivial: true
auto_groomable: true
branch: feat/yq-yaml-parsing
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/137
blocked_by:
reconciled: true
type: docs
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-07-28-yq-yaml-parsing.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-28-yq-yaml-parsing.md) |
| Results | [2026-07-28-yq-yaml-parsing-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-28-yq-yaml-parsing-results.md) |
| ADRs | [ADR-0057](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0057-frontmatter-read-must-be-anchored-when-key-may-be-absent.md), [ADR-0058](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0058-two-tier-frontmatter-scalar-readers-field-vs-field-raw.md), [ADR-0062](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0062-in-repo-shell-yaml-readers-no-external-parser.md) |
<!-- docket:artifacts:end -->

## Why

docket's shell scripts parse YAML and markdown frontmatter with hand-rolled
`sed`/`awk`/`grep` — `sync-agents.sh` (change 0016), `scripts/github-mirror.sh`
(change 0011), and the frontmatter readers sprinkled through the tooling. In
`sync-agents.sh` the config-reading helpers (`entry_line`/`field_of`/`block_names`)
are dense regex over YAML, raised as a readability concern (it is run manually by a
human). `yq` would make those ~40 lines more readable and more *robust* (real YAML:
flow-vs-block mappings, quoting, spacing — the hand parser only handles the
documented block-style subset, and silently ignores top-level flow-style
`agents: {…}`).

The decision today (2026-06-16) was to **keep the scripts as-is** — but the
tradeoff is worth a deliberate future review rather than leaving it implicit. This
stub captures that.

## Re-scoped 2026-07-26 (change 0124 triage)

**The evaluate-yq question is closed — decided "no" by conduct, never written down.** Since this
stub was filed, docket did not merely decline `yq`; it invested in the opposite direction and then
formalized that investment:

- Frontmatter reading was **centralized** into `scripts/lib/docket-frontmatter.sh`
  (`field` / `field_raw` / `fm_field` / `list_field` / `int_field`).
- **ADR-0057** (anchored reads for optionally-absent keys) and **ADR-0058** (the two-tier
  `field` vs `field_raw` reader split) are Accepted decisions *about the hand-rolled readers* —
  design work that would be nonsense had adoption still been open.
- `scripts/docket-config.sh` states "(no yq)" as a standing property (line has since drifted to
  `:100`; cite the file, not the line).
- Zero `yq` invocations exist repo-wide.

So the "if yes" branch is foreclosed, and this change is reduced to the "if no" branch's single
unmet deliverable: **write the ADR**. The original title and `type: refactor` were retired
accordingly — nothing is being refactored.

One premise of the original framing is also stale and should not be repeated in the ADR: change
0132 established that docket *does* validate a runtime dependency (a configured Bash 4+ runtime),
so "zero external deps" is no longer literally true. The honest stance is narrower — no external
*YAML parser* — and the ADR should say that rather than overclaiming.

## What changes

Record a short ADR stating that docket parses YAML and markdown frontmatter with in-repo shell
readers and does not take a dependency on `yq` or any external YAML parser.

- State the decision and its real boundary (no external YAML parser — not "zero dependencies",
  per 0132).
- Give the reasons that actually held: the install pitch is clone-and-run; the two `yq` forks
  (`mikefarah` Go vs `kislyuk` Python) are incompatible binaries, so adoption means pinning one;
  and the parsed surface is a documented block-style subset, not arbitrary YAML.
- Name the accepted consequence: the hand readers handle a subset, and top-level flow-style
  mappings (e.g. `agents: {…}`) are silently ignored rather than parsed.
- Relate it to ADR-0057 and ADR-0058, which decide *how* the in-repo readers behave and only make
  sense under this stance. The link is one-way by construction: both are Accepted and immutable
  except their `status:` line, so their `relates_to: []` is NOT to be edited back.

The new ADR's id is whatever the `docket-adr` flow allocates at author time — do not pre-reserve
one here (other in-flight changes mint ADRs concurrently). Frontmatter: `change: 18`,
`relates_to: [57, 58]`, no `supersedes`/`reverses`.

Name the reader modules as they exist today — `sync-agents.sh` (repo root), `scripts/docket-config.sh`,
`scripts/lib/docket-frontmatter.sh`, and `scripts/lib/docket-runtime.sh` (the `runtime.bash`
declaration reader, changes 0133/0152). The helper names in `## Why` above (`entry_line`, `block_names`)
are pre-refactor history and must not be transcribed into the ADR; the current parser is
`section_body()` / `harness_agent_line()` / `field_of()`. The flow-style consequence is still true:
`section_body()` matches only a bare header, so a top-level `agents: {…}` never enters the block.

### Execution shape (metadata-only change)

This change has no code deliverable, which is unusual and must not be discovered cold at build time:

- The ADR is authored on `metadata_branch` by the `docket-adr` dispatch, and doing so is this
  change's **acceptance criterion — unconditional**. Do not let it ride step 6's "non-obvious
  decision made during implementation" condition: no decision is made during this build, the ADR
  *is* the deliverable. The step-4 plan MUST carry an explicit "author the ADR via the `docket-adr`
  dispatch" task. A PR that opens with no new ADR id recorded in `adrs:` is an incomplete build,
  not a clean one.
- The feature branch never carries metadata (change file, `BOARD.md`, ADRs), so it holds only the
  plan file plus a short results file — enough for a non-empty PR at step 7.
- Delivery to the integration branch is the change's own terminal publish, triggered by that PR's
  merge, with the new ADR id appended to this change's `adrs:` (becoming `[57, 58, <new>]`).
  Re-copying 57 and 58 is idempotent and harmless.
- Do NOT use `docket-adr`'s standalone `terminal-publish --adr NN` path: it would publish the ADR
  while leaving this change with no route to `done`.

## Out of scope

- Re-opening adoption. If it is ever re-opened, that is a new ADR superseding this one, not an
  edit — and it would be all-or-nothing, never one bilingual script.
- Changing any reader's behavior, or touching `sync-agents.sh` / `scripts/github-mirror.sh`
  parsing. This change writes a record; it moves no code.
- Prose in the repo `README.md` or any `scripts/*.md` contract. (Regenerating `docs/adrs/README.md`
  and the integration-branch ADR index at publish are mandatory parts of the `docket-adr` flow, not
  scope creep.)
- `sync-agents.sh`'s `emit()` frontmatter rewrite — markdown-with-frontmatter is not pure YAML and
  stays `awk` under any stance.

## Open questions

- None. The decision is made; this records it.

## Auto-groom verdict — trivial (2026-07-27)

Groomed autonomously to `trivial: true` (build-ready, no spec). Reasoning: the sole deliverable is
one ADR whose stance, boundary, three reasons, accepted consequence, and re-open rule are already
written in this stub — a spec would restate it verbatim. The adversarial critic verified against the
tree that zero `yq` invocations exist repo-wide (only the `(no yq)` comment at
`scripts/docket-config.sh:97`), that ADR-0057/0058 are Accepted and about the in-repo readers, that
change 0132 makes "zero external deps" an overclaim, that `depends_on: [16]` is satisfied (0016
archived `done`), and that no existing ADR states this stance. Its three corrections — do not
pre-reserve an ADR id, do not transcribe the stale helper names, and state the metadata-only
execution shape — are folded into `## What changes` above. No decision needed human context.

## Reconcile log

### 2026-07-28 — reconciled at claim, no scope change

Re-verified every premise of the 2026-07-26 re-scope against the tree at `origin/main`; all hold,
and the change stays exactly what the auto-groom verdict left it: author one ADR, nothing else.

- **Zero `yq` invocations repo-wide.** The only hits are prose in archived changes, plans, and one
  results file — no script calls it.
- **ADR-0057 and ADR-0058 are both `Accepted`**, both about the in-repo readers, and both carry
  `relates_to: []` — left untouched per `## What changes` (the link is one-way by construction).
- **`depends_on: [16]` satisfied** — `0016-docket-subagent-model-effort` is archived `done`.
- **No existing ADR states this stance** (no ADR in `docs/adrs/` mentions `yq`), so this is a new
  decision record, not a restatement. Highest allocated id today is 0061; the new id is still to be
  allocated by the `docket-adr` dispatch, not reserved here.
- **`sync-agents.sh`'s current parser is `section_body()` / `field_of()` / `harness_agent_line()`**,
  as the re-scope said; the pre-refactor names stay out of the ADR.
- **`scripts/lib/docket-frontmatter.sh` exports `field_raw` / `field` / `fm_field` / `list_field` /
  `int_field`** — unchanged.

Two drifts folded in, neither affecting scope:

1. The `(no yq)` comment in `scripts/docket-config.sh` has moved from line 97 to line 100. The ADR
   cites the file, never a line number.
2. `scripts/lib/docket-runtime.sh` (changes 0133/0152) is a **fourth** hand-rolled reader — it parses
   `runtime.bash` declarations under the same no-external-parser stance, and under a stricter
   constraint still (it must run on macOS system Bash 3.2). Added to the reader modules the ADR
   names, and it strengthens rather than complicates the decision.

Execution shape confirmed unchanged: metadata-only, the ADR is the unconditional acceptance
criterion, the feature branch carries only plan + results, and delivery rides this change's own
terminal publish at merge — never `terminal-publish --adr NN`.
