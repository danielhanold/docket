<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0195 — Retune the opencode shipped model defaults for cost](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0195-retune-the-opencode-shipped-model-defaults-for-cost.md)**
<!-- docket:backlink:end -->

# Retune the opencode shipped model defaults for cost — design

Change: 0195 · Type: chore · Priority: low · Depends on: 0192 (done)

## Problem

Change 0192 shipped the `opencode:` block in `agents/harness-defaults.yml` with Kimi K3
(`openrouter/moonshotai/kimi-k3`, ~$0.86/task) on seven of sixteen rows. A single live review run
measured ~$0.80 (2026-08-02), matching the published per-task figure — the cost is the model
choice, not a fault. Kimi is well priced for the top of the ladder but is applied to rows that do
not need it. This change is a data-only cost retune of working defaults.

## Design

Edit six of the sixteen `opencode:` rows in `agents/harness-defaults.yml`. All sixteen `effort:`
values are preserved exactly as shipped. Kimi drops from seven rows to two (the cap rungs).

```yaml
  opencode:
    adr:                   { model: openrouter/openai/gpt-5.6-luna, effort: medium }        # ← was kimi-k3
    auto-groom:            { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: medium }
    auto-groom-critic:     { model: openrouter/openai/gpt-5.6-luna, effort: high }
    brainstorm-consultant: { model: openrouter/openai/gpt-5.6-luna, effort: medium }        # ← was kimi-k3
    build-economy:         { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: medium }
    build-standard:        { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    build-premium:         { model: openrouter/x-ai/grok-4.5, effort: medium }              # ← was kimi-k3
    build-max:             { model: openrouter/moonshotai/kimi-k3, effort: high }
    finalize-change:       { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    implement-next:        { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    integration-repair:    { model: openrouter/x-ai/grok-4.5, effort: high }                # ← was kimi-k3
    rebase-resolver:       { model: openrouter/x-ai/grok-4.5, effort: high }                # ← was kimi-k3
    review-lean:           { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    review-standard:       { model: openrouter/openai/gpt-5.6-luna, effort: medium }        # ← was kimi-k3
    review-deep:           { model: openrouter/moonshotai/kimi-k3, effort: high }
    status:                { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: low }
```

Resulting spread: DeepSeek Flash ×7, Luna ×4, Grok ×3, Kimi ×2 — four families, up from three.

### Surfaces that move with the sidecar

1. **`agents/harness-defaults.yml`** — the six model swaps above. Model IDs stay bare, unquoted,
   space-free scalars (change 0181); the block stays complete at sixteen rows
   (`HD_SHIPPED_HARNESSES` requires completeness in both directions).
2. **`tests/test_harness_defaults.sh`** — two value-keyed blocks, both mutation-visible guards:
   (a) the sixteen-row fixture assertion for the `opencode` harness (the block beginning
   `"adr openrouter/moonshotai/kimi-k3 medium"`) updates to the new six values; (b) the
   "opencode build ladder" asserts, whose `build-premium is Kimi/medium` assert reddens
   independently on the Grok swap and whose comment prose ("Kimi carries the two judgment
   rungs") goes false with it — update the assert to Grok/medium and reword the comment to the
   new ladder shape. Edit the sidecar first, watch both blocks redden, then update.
3. **`.docket.example.yml`** — the commented `opencode:` mirror block (role-ordered rows; its
   comment covers `reasoningEffort:`/OpenRouter mechanics) updates to the same six values.
   Round-trip evidence is mechanical, not manual: `tests/test_docket_example_yml.sh` derives a
   per-key sidecar↔mirror comparison (its slice terminator is the sidecar's `build-max` model,
   which stays Kimi, so the derivation survives) — the guard enforces the round trip.
4. **The block's explanatory comment** in `agents/harness-defaults.yml` (and any echo in the
   example file) — the 0192 rationale "three models, chosen for cheap high intelligence" becomes
   false at four families; reword to state the four-family, cost-tiered rationale.
5. **Delegation-recipe prose** — `README.md`'s and `docs/opencode/setup.md`'s runner-recipe
   examples pin `build-premium … kimi-k3, effort: medium` to mirror the shipped ladder
   (README ~"build-premium … kimi-k3" row; setup.md's matching row). Disposition: **update both
   `build-premium` recipe rows to Grok/medium** so the recipes keep mirroring the shipped
   ladder; the `build-max` recipe rows (setup.md's "no effort" and `effort: auto` examples)
   stay Kimi and are untouched.
6. **Whole-repo grep sweep** for `kimi-k3`, sorted prose vs executable, before finalizing — never
   a hand-listed site enumeration (promoted repo rule). Items 2–5 are the sites known at design
   time; the sweep is the completeness check, not the discovery mechanism for them.

### Verification

- Full suite at the build gate (`scripts/run-tests.sh` via `finalize.test_command`).
- Verify item carried to build: `opencode models` must still list `openrouter/x-ai/grok-4.5` and
  `openrouter/openai/gpt-5.6-luna` at build time (external truth; no in-repo test can catch a
  wrong ID — every mirror assert compares generated output against the sidecar that generated it).
- Verify item carried to build: a **live dispatch** re-certification of the premium rung — the
  same certification class 0192 used, not merely `opencode debug agent` (which observes what
  docket emits, not what OpenRouter accepts for x-ai) — since `build-premium` changes model and
  0192's certification was against the shipped pins. Cheap, bounded, and the only rung whose
  model changes below the cap.

### Invariants that survive

- `review-deep == build-max` (the cap-rung invariant) holds unchanged, without a family break —
  so this change needs **no ADR**.
- No substrate edits: registration, emitter, dispatch, routing, and `REGISTERED_RUNNERS` are
  untouched. If the retune appears to need one, the design is wrong.
- All sixteen efforts unchanged (the benchmark cannot price effort).

## Assumptions

Decisions an interactive brainstorm would have raised, the committed default, and why. Audit
trail for the human; every one is a reversible one-line data edit if overruled.

1. **`review-standard`: Luna, not Grok.** The one row where cheap and coherent conflict. Chosen:
   **Luna at medium** ($0.066). Rejected: Grok at $0.450 (a "real" 49.9/53.8/56.9 ladder).
   Why: (a) the change's stated purpose is cost, and this is the review rung hit most often — it
   is where the observed ~$0.80 becomes ~$0.07; (b) the stub's own table and savings arithmetic
   assume Luna; (c) the risk is bounded — a reviewer's misses are caught at the human merge gate,
   and `review-deep` keeps Kimi; (d) the ladder-compression concern (standard only 1.4 pts above
   lean, at lower effort) is real but argues the middle rung is cheap insurance, not that it must
   cost 7×. If standard reviews prove indistinguishable from lean in practice, that is evidence
   for a future consolidation, not for paying Grok now. Overruling this is a one-line edit plus
   the fixture row.
2. **Kimi stays at both cap rungs.** Adopted from the stub's own arithmetic: Grok at the top
   compresses the ladder to <4 pts; Terra pays +58% over Grok for +1.1 pts; the next genuine step
   up (Sol) is 2.1× Kimi for +1.9 pts. Rejected alternatives: Grok-at-top, Terra-at-top,
   Sol/Opus-at-top (all dominated on $/marginal-point or ladder shape).
3. **`openrouter/x-ai/grok-4.5` is a valid ID** — settled, no longer an assumption: verified
   2026-08-09 against the local `opencode models` listing, which includes it verbatim (alongside
   4.20/4.3 variants). Re-verified at build time per the verify item, since model catalogs drift.
4. **Effort rides out-of-band of the model ID, never inside it.** Settled in-repo, anchored on
   the path the sidecar actually feeds: the sync-agents **native emitter** consumes each shipped
   row's `effort:` and writes it as `reasoningEffort:` agent frontmatter (`agents/harness-defaults.yml`
   header: "docket emits the effort as `reasoningEffort:`"; asserted in
   `tests/test_sync_agents_opencode.sh`). The runner's `--effort` → `opencode run --variant`
   mapping is the *other* out-of-band carrier, serving only user-config `runner: opencode` rows —
   a shipped default is never forwarded there (`scripts/runners/opencode.md`). Both carriers are
   flags/frontmatter, and the live `opencode models` listing shows no effort-suffixed Grok IDs,
   so the three Grok rows' `effort:` values are the right shape as drafted; the shipped Kimi
   rows were live-certified through the emitter path in 0192. Cursor's ID-encoded variants
   (`cursor-grok-4.5-high`) are a cursor idiom, not an OpenRouter one.
5. **Grok-4.5 at `medium` cost is unmeasured but bounded.** The benchmark has one row per model
   (Grok measured at high, $0.450). Assumption: medium ≤ high in cost, so `build-premium`'s
   per-task cost is ≤ $0.450 vs Kimi's $0.860 — the savings direction holds even though the −48%
   figure is exact only for the two high rows. No effort retune is smuggled in to compensate.
6. **Carry a live premium-rung re-certification verify item** rather than waiving it (0192 waived
   `build-max` certification, but that row does not change here; `build-premium` does). Rejected:
   shipping on mirror-round-trip evidence alone — a wrong live pin is exactly the failure class
   no in-repo test can see.
7. **Dependency state**: `depends_on: [192]` is satisfied — 0192 is archived done (2026-08-02),
   so this design is against merged reality, not a snapshot.
8. **Couplings**: no new `depends_on`/`related` beyond what the stub already carries
   ([164, 166, 181] related, [192] dependency); the surfaces in this spec are files, not open
   changes, and no active stub touches the `opencode:` block.

## Out of scope

Unchanged from the stub: the claude/cursor/codex blocks; any substrate change; effort retuning;
reopening 0192's cleared merge-gate items; a user-layer override recipe.
