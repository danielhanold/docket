---
id: 84
slug: terminal-publish-opt-in-default
title: Flip terminal_publish default to false — publishing to the integration branch becomes opt-in
status: in-progress
priority: medium
created: 2026-07-16
updated: 2026-07-16
depends_on: []
related: [62, 64, 83]
adrs: [27, 40]
spec: docs/superpowers/specs/2026-07-16-terminal-publish-opt-in-default-design.md
plan: docs/superpowers/plans/2026-07-16-terminal-publish-opt-in-default.md
results:
trivial: false
auto_groomable:
branch: feat/terminal-publish-opt-in-default
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-16-terminal-publish-opt-in-default-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-16-terminal-publish-opt-in-default-design.md) |
| Plan | [2026-07-16-terminal-publish-opt-in-default.md](https://github.com/danielhanold/docket/blob/feat/terminal-publish-opt-in-default/docs/superpowers/plans/2026-07-16-terminal-publish-opt-in-default.md) |
| ADRs | [ADR-0027](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0027-terminal-publish-repo-scoped-script-gated.md), [ADR-0040](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0040-terminal-publish-default-opt-in.md) |
<!-- docket:artifacts:end -->

## Why

`terminal_publish` defaults to `true`, so every repo that never set the key gets direct
machine commits onto its integration branch at close-out. That default keeps causing real
friction: direct pushes trip branch protection on protected/PR-only branches, auto-mode
permission classifiers deny the push mid-run and hard-stop autonomous loops, and a failed
publish can gap silently (#0083 / #0043 — a terminal record missing from `main` for eight
days with nothing noticing). Writing to the code line should be a conscious, per-repo
opt-in, not a fail-open default.

## What changes

- The built-in default flips to `false` at every layer that encodes it: the config resolver
  (`docket-config.sh`), the `terminal-publish.sh` `--enabled` flag fallback, and the
  `docket-status.sh` sweep fallback — no code path defaults to publishing.
- `terminal-publish.sh` invoked with **no** `--enabled` flag no-ops loudly (prominent stderr
  warning that nothing was published, exit 0); an explicit `--enabled false` stays a silent,
  intentional no-op.
- Docs invert the framing to opt-in and call out the risks of `true`: README (rewritten
  `terminal_publish` section + config sample), `docket-config.md`, `terminal-publish.md`,
  the convention SKILL + terminal-close-out reference, and default-mentions in the adr /
  finalize / status skills.
- This repo pins `terminal_publish: true` in its committed `.docket.yml` — its
  archive-parity practice is unchanged, now explicit.
- A new ADR records the flip (`relates_to: [27]`; ADR-0027 stays Accepted — it decided the
  fencing and gating, not the default value).
- The four affected test suites are updated; fixtures that relied on the implicit default
  pin `true` so the publish path stays covered, plus new coverage that unset ⇒ no publish.
  The coordination-key **fence** fixtures additionally invert their probe value
  (`false` → `true`): once the default is `false`, an ignored machine-scoped `false` is
  indistinguishable from the default, so the assertion would pass vacuously — see the
  reconcile log.

## Out of scope

- #0083's gap-detection health check (separate change; this narrows its blast radius but
  does not replace it).
- Any PR-based publish mechanism for repos that want records on the integration branch
  without direct pushes.
- Retroactive behavior — the knob remains never-retroactive (ADR-0027).

## Open questions

None — design settled in the linked spec (brainstormed 2026-07-16).

## Reconcile log

### 2026-07-16 — reconciled at claim (implementer)

Verified the spec against current `origin/main` + `origin/docket`. **The design holds; no
scope dropped.** Findings:

- **All three code sites match the spec exactly**, including the cited line number:
  `docket-config.sh:199` (`${TERMINAL_PUBLISH:-true}`), `terminal-publish.sh:30`
  (`ENABLED="true"`), `docket-status.sh:389` (`--enabled "${TERMINAL_PUBLISH:-true}"`).
  Nothing was done elsewhere; change 0079's runner-delegation rework (ADRs 37/38) does not
  touch this surface.
- **A FOURTH default-encoding site the spec's code section missed** — found only by the
  whole-repo grep the LEARNINGS ledger mandates (0064 itself hand-listed prose sites and
  missed the executable sweep site, so `terminal_publish: false` would have kept publishing
  on every sweep). `tests/test_docket_status.sh` Case B (~L919-950) mocks a config export
  that omits the key and asserts *"defaults to enabled — archived record DOES reach the
  integration branch"*. It must be **inverted, not deleted**: the block exists to guard a
  real `set -u` unbound-variable crash in `sweep_execute_one`, which `${TERMINAL_PUBLISH:-false}`
  still guards. Its other two asserts stay verbatim. Full inventory is now pinned in the spec:
  exactly **four** sites encode the default.
- **Refinement — the fence tests would go vacuous.** `tests/test_docket_config.sh:578,594`
  probe the coordination-key fence by writing `terminal_publish: false` into the global /
  `.docket.local.yml` layer and asserting the resolved value "stays true". Once the default
  is `false`, the *ignored* value and the *default* coincide, so the assertion can no longer
  distinguish "fence ignored it" from "fence honored it". The fixtures must invert their
  probe to `terminal_publish: true` and assert the value stays `false`. The spec's "fence
  warnings unchanged" understates this — folded into *What changes*.
- **One extra site the spec omits:** `tests/test_closeout.sh:41-42` excludes `tests/` from
  its 0064 call-site sentinel because tests "deliberately exercise the back-compat
  default-omitted-enabled path". That framing dies with the flip (omitted ⇒ loud no-op, not
  publish); the comment needs rewording. Small, folded in.
- **Verified as needing NO change** (so the spec's omission is correct, not an oversight):
  `scripts/docket-status.md` mentions the knob only conditionally ("under
  `terminal_publish: false` that step is a no-op") and already spells `--enabled true`
  explicitly — it encodes no default. `config.yml.example` correctly omits the key (it is
  per-repo fenced), so `tests/test_config_example.sh` is unaffected.
- **The loud-warning path is a safety net, not a caller path:** `test_closeout.sh`'s 0064
  sentinel already forces every real call site (skills + scripts) to pass `--enabled`
  explicitly, and the skills pass the resolved `TERMINAL_PUBLISH` through. So the flip
  cannot silently change any in-repo caller's behavior — only hand invocations.
- `.docket.yml` carries the key commented out (`# terminal_publish: true`) with the comment
  "This repo publishes its terminal records, so the default stands" — exactly the state the
  spec anticipates; it gets uncommented and the comment reworded to an explicit opt-in.
- ADR-0027 confirmed `Accepted` and correctly left alone (it decided the fence + gate
  location, not the default). Next free ADR number is 0040.

### 2026-07-16 — correction (appended at build close-out)

**The "Verified as needing NO change" bullet above was WRONG about `scripts/docket-status.md`,
and the enumerated-floor lesson beat this reconcile too.** Recording it here because the bullet
would otherwise mislead the next reader.

- `scripts/docket-status.md:130` quoted the literal `--enabled "${TERMINAL_PUBLISH:-true}"` —
  an expression this change deletes — and its `swept` report-line definition (L209) claimed the
  record is "published", which the branch's own Case B test disproves for an opted-out repo.
  Both were fixed during the build.
- **How I missed it:** the inventory grep used `--include="*.sh"` for the `TERMINAL_PUBLISH:-`
  pattern, and a case-sensitive lowercase `terminal_publish` for the `.md` files. An *uppercase*
  `${TERMINAL_PUBLISH:-true}` inside a `.md` file fell through both filters. The spec had in
  fact listed the finalize/status skills; I overrode it on the strength of a bad grep.
- **The real defect shape** (found by the final whole-branch review, not by any grep): prose
  naming **`main`-mode as the SOLE no-op exception** to publishing. That was exhaustive *before*
  this change — the mode guard genuinely was the only unconditional no-op — and is exactly half
  the gate after it. Such lines contain neither "default" nor "true", so every keyword search was
  structurally blind to them. Nine live sites carried it, including one instruction
  (`docket-convention/SKILL.md:84`) that would have made a parent agent read a healthy opted-out
  run as a failure.
- Net: 4 executable sites (correctly inventoried) + ~10 prose sites (badly inventoried). The
  code inventory held; the prose inventory was a floor, not the set — the fourth recurrence of
  that lesson in this repo.
