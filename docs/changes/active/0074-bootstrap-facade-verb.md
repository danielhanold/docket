---
id: 74
slug: bootstrap-facade-verb
title: A `bootstrap` facade verb — retire the last direct-helper carve-out in Step-0
status: in-progress
priority: medium
created: 2026-07-14
updated: 2026-07-14
depends_on: [68, 72]
related: [68, 72]
adrs: [29, 30]
spec: docs/superpowers/specs/2026-07-14-bootstrap-facade-verb-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/bootstrap-facade-verb
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-14-bootstrap-facade-verb-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-14-bootstrap-facade-verb-design.md) |
| ADRs | [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md), [ADR-0030](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0030-facade-wiring-guard-discriminates-on-invocation-prefix.md) |
<!-- docket:artifacts:end -->

## Why

Change 0068 introduced the `docket.sh` facade (finite subcommands, config read from stdout, never
`eval`'d) and change 0072 rewired the seven operating skills and the convention's Step-0 preamble
onto it. After 0072, exactly **one** direct-helper invocation survives in the convention: the
`CREATE_ORPHAN` bootstrap path still calls `docket-config.sh --bootstrap` directly, because the
facade has no verb for it.

That single carve-out is the whole cost. It is the one place a reader of Step-0 must learn a second
command shape, and it is the one hole in the claim change 0073 (Cursor sandbox & permissions guide)
wants to make — that docket's entire runtime surface is two command shapes, and therefore that a
small, stable, copyable permission configuration is possible. A one-verb exception forces the
permission config, and the guide explaining it, to enumerate a second binary.

0072 left this deliberately out of scope (0068 owns facade behavior); ADR-0029 and the 0072 spec's
§Decisions both record it as a future candidate, and the 0072 results file re-raised it at the merge
gate.

## What changes

- Add a `bootstrap` verb to the `docket.sh` facade, routing the `CREATE_ORPHAN` path (the guarded
  orphan-`docket`-branch create) through the facade like every other operation.
- Rewire the convention's Step-0 preamble to invoke the verb, retiring the last
  `docket-config.sh --bootstrap` direct-helper mention from skill prose.
- Extend the 0072 skill-facade wiring guard so the bootstrap carve-out it currently tolerates is no
  longer an accepted exception — the guard should redden if a direct-helper bootstrap invocation
  reappears in skill prose.
- Check whether change 0073's "two command shapes" framing can then be stated without the carve-out
  caveat.

## Out of scope

- Any change to what bootstrap actually *does* (the `¬DOCKET ∧ ¬LIVE` guard, the orphan create, the
  push). This is a routing/surface change, not a behavior change.
- Broadening the facade beyond this one verb, or revisiting ADR-0029's finite-subcommand posture.
- Tightening the 0072 prose guard to forbid all `.sh` tokens — ADR-0030 explicitly rejects that
  over-scope, and it must stay rejected.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-14 — reconcile (docket-implement-next)

Design verified against current code (integration branch `origin/main` @ `233a853`); it **holds
unchanged in scope** — no obsolescence, no fundamental invalidation. Details:

- **Dependencies satisfied.** #68 (facade) and #72 (skill rewiring) are both archived `done`
  (`archive/2026-07-14-0068-…`, `archive/2026-07-14-0072-…`). Build-ready confirmed.
- **Verb flag equivalence confirmed.** `docket-config.sh --bootstrap` (bare) == `--export
  --bootstrap`: in `docket-config.sh`, `--export` is the *default* mode and `--bootstrap` is an
  independent `DO_BOOTSTRAP=1` flag, so the bare form both performs the CREATE_ORPHAN write and
  emits the `KEY=value` env block. The spec §1 verb snippet `exec docket-config.sh --bootstrap
  "$@"` is correct; the Step-0 rewire still needs the trailing `then re-run docket.sh preflight`
  (the verb is not a composite — it does not sync the worktree).
- **Guard file drifted since the spec was groomed (folded in).** Change #71 (PR #81) merged
  *after* the 0074 spec was written, reshaping `tests/test_skill_facade_wiring.sh`: it added the
  Layer-3 board-surfaces sentinel (`SCOPE3`, `BOARD_SURFACES`/`--surfaces`/`board-refresh`
  asserts). The two 0074 edit sites moved — the byte-exact **bootstrap strip** clause is now at
  ~L94 (the `s#…/docket-config\.sh --bootstrap##g` sed clause in `strip_canonical`), and the
  **`carve == 1`** assertion is now at ~L126–128. The spec's `~78`/`~110` refs are pre-#71 and
  stale; the spec §Context/§3 have been updated to anchor by *shape* (per the enumerated-set
  learning), not line number. The 0074 edits (delete the bootstrap strip clause; replace `carve
  == 1` with a prefixed `== 0` assert keyed on `"${DOCKET_SCRIPTS_DIR:?…}"/docket-config.sh`)
  are independent of Layer 3 and coexist with it cleanly.
- **Single skill-prose invocation site confirmed.** A whole-repo grep found exactly one
  invocation-position `docket-config.sh --bootstrap` in skill prose:
  `skills/docket-convention/SKILL.md:74` (the CREATE_ORPHAN clause) — the site §2 rewires. All
  other occurrences are legitimate NOUN/contract mentions out of the wiring guard's scope
  (`scripts/lib/docket-gitignore-block.sh` comment, `scripts/lib/docket-preflight.sh:26`
  diagnostic, `scripts/docket-config.md` usage examples, `tests/test_consuming_repo_scripts.sh`
  resolvability tests). The build must not touch these; the `docket-preflight.sh:26` human-facing
  diagnostic is an *optional* facade-consistency nicety, not a required or guard-enforced site,
  and does not affect the §7 skill-surface "two command shapes" claim.
- **Doc-accuracy floor for the build (spec's site list is a floor, not a ceiling).** Adding the
  verb also touches, for accuracy: `docket.sh` usage header + `reject()` supported-ops line;
  `docket.md` Behavior/"Not exposed"/Exit-code prose that today says `docket-config.sh` is reached
  "through the `env` and `preflight` verbs" (→ add `bootstrap`); `test_docket_facade.sh` `sh_ops`
  (add `bootstrap` to the verb set) and `expected_labels` (add the `bootstrap)` case arm); and the
  wiring test's header/`strip_canonical` comments that describe the carve-out as present.
- **ADR hygiene (§6) unchanged and needed.** ADR-0030 (Accepted) has two now-stale statements —
  "two byte-exact canonical forms … and the single `…/docket-config.sh --bootstrap` carve-out"
  (Decision) and the "one convention-only carve-out" Consequence — so it takes a dated `## Update`
  note (never a Decision edit) recording the retirement; its invocation-prefix discriminator is
  unchanged. ADR-0029 untouched. Both ids are already in `adrs: [29, 30]`, so terminal-publish
  re-copies the updated ADR-0030 at merge.
