# Auto-create discovered stubs — design

- **Change:** #0091 (`auto-create-discovered-stubs`)
- **Date:** 2026-07-17 (UTC)
- **Author:** docket-auto-groom (autonomous, default-biased self-brainstorm)
- **Depends on:** #0090 (`discovered-from-provenance`) — this change populates the `discovered_from:` field #0090 defines
- **Relates to:** ADR-0012 (script-vs-model boundary), ADR-0019 (global-config fence), ADR-0021 (pipeline scripts author formulaic commits)

> This spec was groomed autonomously. Every design decision was defaulted conservatively; the
> `## Assumptions` block is the deferred human audit trail — read it first at the merge gate.

## Problem

In an unattended autonomous run, follow-up work that a skill discovers mid-task is routinely lost.
`docket-implement-next`'s reconcile/review notices adjacent gaps, a build surfaces a latent bug, a
close-out finding implies a next step — today the model either *asks the human* (no human present)
or drops it in prose that scrolls away. docket already generates this discovered work but has no
durable, no-friction capture path for it when no human is in the loop.

The fix is a config-gated posture: when enabled, an autonomous skill that identifies genuine
follow-up work mints a `proposed` needs-brainstorm stub directly (with `discovered_from:` set, per
#0090) instead of asking or mentioning. Stubs are cheap, reviewable markdown on the metadata
branch; the human still gates everything at groom time, so auto-capture adds **capture fidelity,
not autonomy** — nothing is groomed, built, or merged without the existing human gates.

## Design (settled)

### 1. The config knob — `auto_capture: true | false` (default `false`)

- A new boolean `.docket.yml` key `auto_capture`, default `false`. When `false`, today's
  ask-or-mention behavior is unchanged (byte-identical to pre-change for non-adopters and for the
  default-off majority). When `true`, the autonomous mint behavior below activates.
- Resolved by `docket-config.sh --export` using the **same layered read as `auto_groom`**
  (`.docket.local.yml` > repo `.docket.yml` > global `config.yml` > built-in `false`; see
  `docket-config.sh:206`), emitted as `AUTO_CAPTURE`. Skill bodies read the exported variable;
  they never re-parse YAML.
- **Fence classification: global-able** (may be set in the user-level global config and
  `.docket.local.yml`, not only per-repo). Justification under ADR-0019: the fence protects
  *coordination* keys whose per-machine divergence would split or corrupt the shared substrate
  (where the backlog lives, branch names, dirs, external objects). `auto_capture` gates an
  autonomous **local-run behavior** that produces ordinary needs-brainstorm backlog commits — it is
  directly analogous to `auto_groom`, which ADR-0019 **explicitly classifies global-able** despite
  also producing non-re-derivable commits (specs, change-file edits) on the shared `docket` branch.
  Per-machine divergence of `auto_capture` means "machine A captures discovered work, machine B
  does not" — the same benign divergence `auto_groom` already tolerates — never a split backlog.

### 2. The mint mechanism — model decides *what*, script does the *mint* (ADR-0012 boundary)

Auto-capture splits cleanly across the ADR-0012 script-vs-model boundary:

- **Model (the autonomous skill) decides *what* to capture** — the materiality judgment (§4) is a
  judgment call the agent makes; it is not mechanizable.
- **A deterministic helper does the *mint*** — id allocation, stub-file write from the change
  template, `discovered_from:` population, the cheap dedup check (§5), and the compare-and-swap push.
  This is the mechanical, repeatable operation `docket-new-change` performs today in its **Step 1
  (Allocate)** skill routine (scan max `id:` across `active/` + `archive/`, id = max + 1, finalize
  at push by CAS: on a non-fast-forward rejection re-preflight → re-read max → re-allocate → rename
  the stub file → re-push). Recommended shape: factor that routine into a deterministic
  **`mint-stub.sh`** helper reached through the `docket.sh` facade, so both `docket-new-change` and
  the autonomous skills call one CAS-correct implementation (consistent with ADR-0021, which lets
  deterministic pipeline scripts author formulaic commits). The exact factoring — new script vs
  shared skill routine — is a plan-time decision; the **contract** is fixed here: non-interactive,
  populates `discovered_from:`, honors dedup + cap, CAS-correct.
- The minted stub is an ordinary `proposed` needs-brainstorm change: `trivial: false`, no `spec:`,
  `auto_groomable` **unset** (inherits the repo default, exactly like a scan-mode stub), a
  PM-altitude why/what body distilled from the discovery, `discovered_from:` set to the current
  change id, `created`/`updated` = UTC today.
- Minting is a **metadata-worktree write only** — it must not disturb the running change's own
  claim/branch/PR state (same isolation as any `docket-new-change` allocation).

### 3. Which skills mint (with `auto_capture: true`)

The **autonomous, single-change** discovery surfaces:

- `docket-implement-next` — reconcile-pass and review-pass discoveries (one "run" = one build).
- `docket-finalize-change` / `docket-status` harvest — close-out findings that imply distinct
  follow-up work, as opposed to a learnings lesson (one "run" = one close-out).

**`docket-auto-groom` is deliberately NOT a mint site.** Two reasons: (a) it would break
auto-groom's own *provable-termination* invariant — the drain proves termination on "every exit
shrinks the queue," but a minted `proposed` needs-brainstorm stub (with `auto_groomable` unset →
inheriting an `auto_groom: true` repo default) is itself autonomous-eligible, so an exit could
*grow* the queue; (b) with `auto_groom` + `auto_capture` both on, auto-groom minting its own
feed-stock is a self-reinforcing backlog-growth loop. Implement-next's reconcile/review and
close-out findings are the strong, well-bounded discovery surfaces; auto-groom "spotting adjacent
work while grooming" is the weakest, and excluding it eliminates both hazards at no cost to the
core value.

`docket-new-change` and `docket-groom-next` are **interactive** and already mint stubs by their
nature — they are out of scope for auto-capture (the human is present to decide).

### 4. Materiality bar (guardrail against noise)

Mint a stub **only** for *actionable follow-up work that would be its own change / PR* — the test
is "would a human file this as a `docket-new-change`?" Explicitly **not** a stub:

- A build-loop lesson about *how to build* → the **learnings** harvest.
- In-scope drift for the *current* change → the **reconcile log**, or folded into current work.
- A mere observation with no distinct actionable follow-up → prose / run report only.

### 5. Deduplication (cheap check)

Before minting, compare the proposed slug (and title) against existing **active** change files
(case-insensitive slug match). On a near-match, **skip the mint** and note the skip in the run
report. This is the "cheap check against existing active titles/slugs" the stub scopes in; anything
deeper (semantic dedup, archive scan) is out of scope.

### 6. Per-run cap (guardrail against runaway)

A **small hardcoded cap** (recommended: **3**) bounds how many stubs one autonomous **skill
invocation** may mint. The cap scope is **per-invocation on a single change**: for
`docket-implement-next`, per build; for the `docket-finalize-change` / `docket-status` harvest, per
close-out. (There is no drain-loop ambiguity to resolve, because `docket-auto-groom` is not a mint
site — §3.) Overflow is **surfaced, not silently dropped**: the run reports "N additional discovered
items suppressed by the auto-capture cap" so a human can act on them. The cap stays a constant (not
a new config key) to keep the config surface a clean boolean; promoting it to config is a deferred
follow-up if the constant proves too rigid in practice.

> Non-blocking plan note: a `docket-status` sweep can close multiple changes in one pass, so the
> per-close-out cap scales as up to 3 × N stubs across N swept changes. This is still bounded,
> surfaced in the report, human-gated (each stub is inert needs-brainstorm), and non-looping (the
> swept set is already-merged and fixed before minting) — the plan should just size the cap with the
> sweep case in mind rather than assume one close-out at a time.

### 7. Board & lifecycle — no new state

Auto-created stubs are ordinary `proposed` needs-brainstorm changes. They already surface on the
board as **needs-brainstorm** and flow into `docket-groom-next`'s queue; the human gates them at
groom time. This change adds **no new board state or "unreviewed" flag**. Provenance visibility
("discovered from #NN") is #0090's rendering territory (its `discovered_from:` field + `## Artifacts`
/ board render), not this change's.

### 8. Surfacing (ship the knob end-to-end)

Per the "a config knob is not done when it merely works" discipline, the same change ships:

- `auto_capture` (commented, with its default) added to `config.yml.example` and to the
  `docket-convention` `.docket.yml` schema block.
- **`auto_capture: global-able` recorded in the authoritative per-key fence classification table in
  `scripts/docket-config.md`** — the convention names that table authoritative, so the classification
  must land there, not only in this spec's prose.
- README documentation of the knob and the auto-capture posture.
- The `docket-convention` *Autonomous grooming* / discovery prose updated so the now-relaxed
  "the model asks the human" framing reflects the configurable capture path.
- The two autonomous single-change skill bodies (§3 — `docket-implement-next` and the
  `docket-finalize-change`/`docket-status` harvest) wired to the mint contract behind the
  `AUTO_CAPTURE` gate.

## Out of scope

- Auto-grooming or auto-implementing the created stubs — governed by the existing `auto_groom`
  machinery (an auto-captured stub in an `auto_groom: true` repo becomes autonomously groomable via
  the *existing* rules; this change adds no new grooming behavior).
- The `discovered_from:` field itself — that is #0090; this change consumes it (`depends_on: [90]`).
- Deduplication beyond the cheap active-slug check (§5).
- Analytics over the provenance graph (#0010's territory).
- Making the per-run cap configurable (deferred follow-up, §6).

## Dependencies & reconcile notes

- **`depends_on: [90]`.** The mint contract populates `discovered_from:`, a manifest field #0090
  defines; the field must exist (template + manifest + `docket-config`/render awareness) before this
  behavior populates it. Design-ahead is legitimate for grooming; the implementer's reconcile pass
  re-validates that #0090 has landed (or folds in the field) before building.
- **Merge-vs-separate (the stub's open question) is resolved as *separate*.** #0090 = the field
  (provenance data), #0091 = the behavior (auto-capture). Keeping them separate keeps each a small,
  independent PR and lets the field land first. (This spec does not and cannot decide a *merge* of
  the two changes — that is a cross-change call; it designs #0091 as an independent consumer.)

## Verification notes (for the plan)

- A default-off (`auto_capture` unset/false) repo mints **zero** stubs and keeps `--check` a no-op —
  a regression test asserting the minimal/non-adopter path is byte-unchanged (the
  opt-in-signal-not-file-presence discipline).
- Live-test the global-layer read in a repo that also has committed generated artifacts, to confirm
  the global `auto_capture` value actually takes effect and is not shadowed (config-layer read
  hazard).
- The mint is CAS-correct under concurrency: two runs minting at once do not collide on an id
  (re-preflight → re-read max → re-allocate → rename → re-push), mirroring `docket-new-change`.
- Metadata-branch artifacts (the minted stubs) are verified at build time and recorded in the
  results file (repo tests only see the integration branch).

## Assumptions (deferred human audit trail)

Each row: the decision, the conservative default chosen, the alternatives rejected, and why the
default is safe to auto-commit.

1. **Merge #0090 + #0091 vs keep separate** → **Keep separate; #0091 `depends_on: [90]`.**
   Rejected: merging into one change. Why safe: this is the smaller, independent-PR choice; the
   field can land first; a cross-change *merge* is not a call this groom is permitted to make, and
   designing #0091 as a standalone consumer needs no such call.

2. **Single global switch vs per-skill granularity** → **Single boolean `auto_capture`.**
   Rejected: per-skill flags (e.g. allow implement-next but not auto-groom). Why safe: YAGNI — one
   switch is the minimal surface, matches the stub's own `true|false` framing, and granularity is a
   purely additive, reversible follow-up if a real need appears.

3. **Config key name/default** → **`auto_capture`, default `false`.** Rejected: `auto_create_stubs`,
   `capture_discovered`. Why safe: matches the stub's proposed name; default-off preserves current
   behavior for everyone; a key name is reversible before it ships and the human can rename at the
   merge gate.

4. **Fence classification** → **Global-able**, recorded in the authoritative fence table in
   `scripts/docket-config.md` (§8). Rejected: per-repo-only (fenced). Why safe: direct analogy to
   `auto_groom`, which ADR-0019 explicitly lists as global-able despite producing non-re-derivable
   commits on the shared branch; `auto_capture`'s per-machine divergence is the same benign
   "captures / doesn't capture" divergence, never a split backlog or corrupted coordination state.
   The classification is landed where it governs (the authoritative table), not left as spec prose.

5. **Mint mechanism** → **Reuse `docket-new-change`'s allocate/CAS routine via a deterministic
   helper (recommended `mint-stub.sh`); model decides materiality, script does the mint (ADR-0012).**
   Rejected: each skill hand-rolls id allocation + push (CAS-correctness would drift across N
   copies). Why safe: it reuses a proven CAS path and respects the established script-vs-model
   boundary; exact factoring (script vs shared routine) is left to the plan, only the contract is
   fixed.

6. **Materiality bar** → **"Would a human file this as a `docket-new-change`?" — distinct actionable
   follow-up only; lessons → learnings, current-change drift → reconcile log.** Rejected: mint for
   any surfaced observation (noise), or no bar (backlog flood). Why safe: it is a principled,
   conservative (mint-fewer) bar with clear alternative homes for non-work findings; combined with
   the cap it bounds noise.

7. **Per-run cap** → **Hardcoded small constant (recommend 3), scoped per skill invocation on a
   single change (per build; per close-out); overflow surfaced in the run report.** Rejected: no cap
   (runaway), a new `auto_capture_cap` config key (extra surface), or an unscoped "per run" that a
   drain loop could evade. Why safe: the scope is unambiguous now that auto-groom's drain is not a
   mint site (§3/#10); it bounds runaway while keeping the config a clean boolean; nothing is
   silently dropped; config-ifying is a trivial reversible follow-up.

8. **Deduplication** → **Cheap case-insensitive active-slug/title check; skip + report on match.**
   Rejected: semantic dedup or archive scanning. Why safe: it is exactly the cheap check the stub
   scopes in; a false-negative just yields a reviewable near-duplicate stub the human dismisses at
   groom time (low cost, no autonomy risk).

9. **Board / lifecycle treatment** → **No new board state; auto-created stubs are plain
   needs-brainstorm.** Rejected: a new "discovered — unreviewed" board flag/state. Why safe: minimal
   surface; the human already sees and gates needs-brainstorm stubs; provenance rendering is #0090's
   territory, so a board flag here would duplicate or pre-empt it.

10. **Mint sites** → **The two autonomous single-change skills (implement-next; finalize/status
    harvest). `docket-auto-groom` and the interactive skills are excluded.** Rejected: including
    auto-groom as a mint site; also wiring new-change/groom-next. Why safe: (a) interactive skills
    already mint with a human present, so wiring them adds no value and risks double-capture; (b)
    auto-groom minting would break its own provable-termination invariant (a minted needs-brainstorm
    stub, `auto_groomable` unset → autonomous-eligible in an `auto_groom: true` repo, grows the
    queue an exit is meant to shrink) and create an `auto_groom` × `auto_capture` self-reinforcing
    backlog-growth loop — excluding it removes both hazards at no cost to the core value, since
    implement-next reconcile/review and close-out findings are the strong discovery surfaces.
