---
id: 71
slug: board-surfaces-unset-vs-empty
title: Encode the disabled board positively — an empty surfaces value is a wiring bug, not a configuration
status: in-progress
priority: medium
created: 2026-07-13
updated: 2026-07-14
depends_on: [72]
related: [59, 68, 69, 70, 72]
adrs: [28]
spec: docs/superpowers/specs/2026-07-13-board-surfaces-unset-vs-empty-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/board-surfaces-unset-vs-empty
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-13-board-surfaces-unset-vs-empty-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-13-board-surfaces-unset-vs-empty-design.md) |
| ADRs | [ADR-0028](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0028-report-channel-is-not-a-board-surface.md) |
<!-- docket:artifacts:end -->

## Why

`board-refresh.sh` cannot tell "this repo disabled the board" from "the caller passed me nothing" — both arrive as an empty `--surfaces` value. `docket-config.sh` maps `board_surfaces: []` to exactly `BOARD_SURFACES=""`, so at the script boundary a **disabled repo and a mis-wired caller are byte-identical**. The script prints `inline disabled — no-op` and exits 0; the caller sees an unchanged `BOARD.md`, treats it as the genuine no-op the prose licenses, commits nothing, and proceeds believing the board is current. The board goes stale with a success exit code the whole way down. This was hit live while filing change 0070.

Change 0059 defended the adjacent hole — a caller who *forgets the flag* exits 2, and `board-refresh.sh` tracks `SURFACES_SET` precisely so the two cases stay distinct. What it did not anticipate is a present flag carrying an unresolved value, which lands in the legitimate-empty branch. It is the same defect class ADR-0028 and change 0069 eliminated on the report channel: an exit-0 no-op indistinguishable from success.

Changes 0068 and 0072 were expected to remove the **trigger**. Re-verified against merged `main` on 2026-07-14, **they did not.** 0068 diagnosed the root cause independently — resolved config stored in an agent's shell, which no harness guarantees to persist across tool calls — and retired the `eval` pattern in favour of a facade that *prints* config; 0072 rewrote the Step-0 preamble to carry those printed values forward as literals. But **the Board-pass call sites were never re-spelled**: grep on `origin/main` finds **8 sites across 6 skill/reference files still spelling `--surfaces "$BOARD_SURFACES"`, and zero literal spellings**. 0072 replaced a mechanism that *guaranteed* the variable was unset with an instruction an agent must remember to apply — against prose that still shows the variable. The failure is now less likely and no more detectable.

So the trigger survives, and the **encoding is still ambiguous**: the next mis-wiring — from any cause — is still silent rather than loud. That makes the sentinel below load-bearing, not defence-in-depth. The duplicated Board-pass prose blocks also remain, and `board_surfaces: []` stays the sole legitimate empty, keeping the defect class permanently latent.

## What changes

Invert the polarity: failing to resolve config must produce a loud error, and disabling the board must require positively saying so.

- **A positive sentinel.** `docket-config.sh` never emits an empty `BOARD_SURFACES`; `board_surfaces: []` resolves to the reserved token `none`. `board-refresh.sh` keeps `--surfaces` required as a flag (0059's guard intact), **exits 2 on an empty value**, and no-ops on `none` — which never creates or truncates `BOARD.md`. `none` is reserved and exclusive (`none inline` exits 2). `docket-status.sh`'s `board_pass` gains a `none` arm mapping to its existing `board off` line, so a disabled repo's output is unchanged.
- **One Board pass.** The six duplicated Board-pass prose blocks collapse into a single facade call, `docket.sh docket-status --board-only` — the orchestrator already self-resolves config, gates, renders, commits, and pushes with retry. No surfaces value crosses the skill/script boundary anymore; the sentinel then defends only script-to-script and future callers. `docket-new-change`'s board commit splits out from its change+spec commit, aligning it with the separate-board-commit rule every other skill already follows. Must-land skills key their retry on the stdout report line (`board inline changed pushed`), not an exit code.
- **A structural sentinel**, grep-derived and mutation-tested: no skill or reference prose may contain `$BOARD_SURFACES`, `--surfaces`, or a direct `board-refresh` invocation.
- **One ADR**, recording the reversal of 0059's "an explicit empty value means no surfaces" and the durable rule behind it: *a deliberate off-state must be encoded positively; absence and emptiness are reserved for error.*

## Out of scope

- Changing `board_surfaces: []` **semantics**. A disabled repo keeps a no-op that never truncates a prior `BOARD.md`; only its encoding changes (`""` → `none`).
- The `$SKILL_*` family — the same unresolved-variable hazard on the model-consumed side. **Change 0072 owns it** (it rewrites the Step-0 preamble to read printed values and interpolate them as literals). Deliberately not duplicated here.
- The Step-0 preamble rewrite and the `eval` retirement — changes 0068 and 0072.
- The `github` surface and `github-mirror.sh`. Same caller pattern, but that surface is best-effort by design; this scopes to the `inline` write decision.
- Retrofitting stale boards. Any board left stale by this bug self-heals at the next correctly wired Board pass.

## Open questions

<!-- Resolved at grooming, 2026-07-13. The design is settled in the linked spec. -->

## Reconcile log

### 2026-07-14 — reconciled against `origin/main` @ `edb37f9`

Re-derived every call-site claim by grep rather than trusting the spec. **Build as specified** — the
design is intact, nothing obsolete.

- **The trigger survives 0072, confirmed.** 8 `--surfaces "$BOARD_SURFACES"` call sites across 6
  skill/reference files; zero literal spellings. 0072 retired the `eval` and told agents to carry
  printed config forward as literals, but never re-spelled the Board-pass call sites — so the spec's
  corrected reading is right, and the sentinel is load-bearing rather than defence-in-depth.
- **Part 2 shrank.** `docket-status.sh` already parses `--board-only`, and `docket-status` is already
  a facade op — `docket.sh docket-status --board-only` works today. Part 2 is therefore prose
  rewiring plus the report-line contract, not new script surface. `board_pass_inline` already passes
  a literal `--surfaces inline`, so only `board_pass`'s empty-guard needs part 1's `none` arm.
- **Part 3 tightened by two ADRs that landed after grooming.** ADR-0030 fixes the sentinel's
  discrimination rule (forbid **invocations**, not nouns — the convention and docket-status prose
  legitimately *describe* `board-refresh.sh` and must stay green). ADR-0031 forbids collapsing or
  deleting the existing board-write guards, so this adds an independent scan and extends
  `tests/test_skill_facade_wiring.sh` in its established idiom rather than widening `REDIRECT_RE`.
- **Dependency discharged.** 0072 is `done` (PR #79); 0068 is `done` (PR #78).
- Scope, out-of-scope, and success criteria unchanged. `adrs:` will gain the new polarity-reversal
  ADR at step 6.
