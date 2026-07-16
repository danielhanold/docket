# Design: learnings promotion destination — the ledger gets a shape that can shrink

**Status:** design (groomed 2026-07-16 via `docket-brainstorm`; reconciled 2026-07-16 at claim — every
factual claim re-verified against current reality and holding; ledger measures 490 lines / 33 top-level
entries, next free ADR id is 0041)
**Change:** 0067
**Related:** change 0006 (the original ledger — this reshapes what it built), change 0065 (the harvest that surfaced the 491-line wall), ADR-0005 (close-out-only harvest — extended, not superseded), ADR-0012/0022/0030/0031 (script-vs-model boundary and the derived-view family this joins), ADR-0019 (config-fence classification for the new `learnings.cap`/`learnings.enabled` keys), ADR-0028 (the ungated report channel the escalations ride), ADR-0032 (positive off-state — the `promotion_state` default and the disable-note discipline)

## 1. Problem

The convention's *Learnings ledger* names a two-lever distill rule for `<changes_dir>/LEARNINGS.md`: past ~300 lines, (a) merge near-duplicates and (b) promote durable conventions to CLAUDE.md. **Lever (b) has never existed** — this repo has no CLAUDE.md, and nothing writes one. So every distill has only lever (a), and lever (a) has run dry: at change 0065's harvest the ledger was 382 lines; merging five near-duplicate families won ~31 lines net; it now stands at 491 lines carrying ~43 genuinely distinct lessons across ~30 top-level entries. **Merging distinct lessons is destruction, not distillation** — the next "distill" would start deleting knowledge.

Second-order cost: the ledger is read *whole* at three hot moments — `docket-groom-next` before a brainstorm, `docket-implement-next` at plan time and again at review. The `guards-are-code` family alone is ~100 lines. Every line is a context tax on every run, whether or not it bears on the change at hand. ADR-0005's founding consequence — "the ledger stays short enough to actually be read" — has quietly failed, and single-file append-with-caps cannot recover it (the converged production pattern for agent memory in 2025–26 — OpenAI Codex's always-in-prompt index over a grepped handbook, Claude Code's 200-line `MEMORY.md` index over one-file-per-memory — is index-plus-detail, not one brutally-capped file).

The ledger needs a structure with a real shrink valve: a way to *retrieve by relevance* instead of *pay by history*, and a way to *graduate* the rules that must fire unprompted out of the retrieval tier entirely. It also needs a wholesale off switch, for repos and operators who would rather not pay the token cost at all.

## 2. Goals

1. Turn the ledger into an **index + detail** structure: one curated finding-file per lesson (or per consolidated family), plus a **rendered, derived** index — so readers load a hint surface and pull only the findings relevant to the change at hand.
2. Make **promotion real**: the rules that must fire unprompted graduate to the repo's always-in-context agent-instructions file on the integration branch; the finding then leaves the retrieval surface, so the ledger genuinely shrinks.
3. Keep **consolidation human-gated**: past a configurable cap the loop *flags* — it never auto-merges its own memory.
4. Give operators a **wholesale off switch** (`learnings.enabled`) that zeroes the token cost without destroying accumulated findings.
5. **Migrate** the current 491-line ledger into the new structure as the build's acceptance proof, and ship the change **end-to-end** — every prose/skill/convention site, the sample config, an integration-branch README section, and ADR-0005's territory all describing what actually exists.
6. Preserve ADR-0005 unchanged in substance: harvest-only-at-close-out, single writer, single moment, ledger-never-published, and the idempotency probe all stand.

## 3. Non-goals

- Changing **who** writes the ledger or **when** (ADR-0005's close-out-only harvest, single-writer rule, and idempotency probe are load-bearing and untouched).
- Publishing the ledger — finding files or index — to the integration branch. The harvest still writes `metadata_branch` only.
- **Automating the promotion or the consolidation judgment.** The harvest proposes; a human disposes. LLM self-summarization of its own memory is the operation this design deliberately refuses to automate (Stanford ACE 2025, "context collapse").
- Automating **merges of two existing distinct findings.** The harvest may create a finding or extend an existing family finding; collapsing two distinct findings into one is human-gated curation, never a harvest action.
- **Purging findings.** Disabling the subsystem is a read/write gate, never a delete; existing `learnings/` files are left untouched.
- Any new lifecycle status, or any change to the change-manifest schema.

## 4. Design

### 4.1 Structure — a findings directory replacing the single file

`LEARNINGS.md` becomes a directory:

```
<changes_dir>/learnings/          # default docs/changes/learnings/  (metadata_branch only, never published)
  <slug>.md                       # one curated finding per file (a lesson, or a consolidated family)
  README.md                       # GENERATED index — sole writer render-learnings-index.sh; never hand-edited
```

This mirrors `<adrs_dir>/` (flat files + a generated `README.md` index) **in every respect but one deliberate difference — the filename.** A **finding** is one lesson or one consolidated family — exactly the granularity today's family entries (`guards-are-code`, `enumerated-floor`, …) already use by hand; the design formalizes it.

**Finding-file frontmatter** (line-oriented, parseable by the existing `scripts/lib/docket-frontmatter.sh` — `field`/`list_field`/`int_field`; **no real YAML loader**, per the YAML-scalar family learning):

```yaml
---
slug: guards-are-code
hook: "A guard is code — mutation-test it (strip the feature, watch it redden) or it is decoration."
topics: [testing, sentinels]        # flat list (list_field); first tag is the PRIMARY grouping topic
changes: [14, 15, 21, 36, 37, 64, 65, 68, 69, 70, 71, 72, 73, 74]   # int list (list_field) — provenance + idempotency key
created: 2026-06-17
updated: 2026-07-16
promotion_state: retained           # retained | candidate | promoted   (positive off-state, ADR-0032)
promoted_to:                        # set only when promoted: path of the agent-instructions file it graduated into
---

## Apply
<the distilled, actionable rule — the load-bearing part a retriever wants first>

## War story
- 2026-07-14 (#72, PR #79) — <what happened>. …
- 2026-06-17 (#15, PR #32) — <what happened>. …
```

Decision records:
- **Filename is a bare `<slug>.md` — no index number, deliberately unlike ADRs.** An ADR number encodes an *immutable, ordered record*: a new decision is a new number, and numbers never move. A finding is the opposite — a **living file** extended every time a later change re-hits the same class (see `guards-are-code`'s 14 cited changes). Three concrete reasons the bare slug is the right key: (1) it *is* the harvest's natural **dedup key** — "does a finding about this class already exist?" is answered by the slug, not by scanning for a free number; (2) it avoids a **number-minting race** between concurrent autonomous sweeps (docket runs concurrent loops; two sweeps minting `0044-…` at once is a collision the slug sidesteps entirely); (3) chronology has a better home already — `created:`/`updated:` and the dated `## War story` entries carry it, so a filename ordinal would be redundant *and* fight the living-file model. The index (§4.2) supplies ordering and grouping as a derived view; the files themselves need none.
- **`created:`/`updated:` reuse the change manifest's existing field spellings** — the same date vocabulary every docket reader already parses (`field FILE created`), rejecting `creation_date`/`last_updated`. One vocabulary, no new parse.
- **`hook`** is the one-line index text — the "hint surface" line. It carries a colon-space and so **must be quoted** (YAML-scalar family learning; the reader tolerating an unquoted scalar is not evidence it is well-formed).
- **`changes:`** is a flat int list (same shape as `depends_on:`/`related:`/`adrs:`), which `list_field` already parses. It is both provenance and the idempotency key (§4.4). PR numbers live in the dated `## War story` entries, as they do today — keeping frontmatter to shapes the grep/awk reader already handles, rather than a list-of-maps no docket reader can parse.
- **`promotion_state`** uses the underscore spelling of docket's frontmatter convention (`auto_groomable`, `depends_on`), normalizing the brainstorm's hyphenated `promotion-state`. Default `retained` is a **positive** off-state (ADR-0032: absence/emptiness are reserved for error).
- Body is **Apply-first** (the rule the retriever acts on), then dated war-story entries newest-first with full `(#<id>, PR #<n>)` provenance — preserving today's `Apply:` convention and dated provenance.

The finding **files** stay curated prose — edited only by the single writer (the harvest) and by human curation, **never regenerated**. Only the index is derived. This is the precise boundary the convention's current "the ledger is prose, never regenerated" sentence must be rewritten to draw.

### 4.2 Index — a rendered derived view

`<changes_dir>/learnings/README.md` is generated by a new script in docket's **derived-view family** — the exact analog of `render-adr-index.sh` (change 0030):

**Deliverable: `scripts/render-learnings-index.sh` + contract `scripts/render-learnings-index.md`.**
- Reads every `learnings/*.md` (excluding `README.md`) via `lib/docket-frontmatter.sh`; emits the index to **STDOUT**; performs **no git writes** (the caller redirects + commits). Offline, deterministic, idempotent — same finding files ⇒ byte-identical output.
- **Sole writer** of `README.md`; skills never construct or patch it by hand (ADR-0012 script-vs-model boundary, exactly as for `BOARD.md` and the ADR index). The script itself needs no enabled/disabled awareness — the *callers* gate on `learnings.enabled` (§4.5), keeping the sole-writer renderer pure, exactly as `render-board.sh` stays pure while `board-refresh.sh`/the orchestrator gate it.
- **Content**: one line per finding — the `hook` plus a link — grouped under the finding's **primary topic** (first tag in `topics:`), remaining tags rendered inline (`· also: <tags>`) for cross-topic discoverability. A finding appears **once** (so it is counted once against the cap). A `candidate` finding carries a marker (e.g. `⟨needs promotion⟩`). Grouping and ordering are fully derived from frontmatter — no embellishment (the `render-adr-index.sh` contract's discipline).
- **Promoted findings** are removed from the topic-grouped hint surface and listed compressed in a trailing **`## Promoted`** group (`- [slug](slug.md) → <promoted_to>`), mirroring the ADR index's `## Superseded / Reversed` group — decay via status metadata, **not deletion**. The human's removal instinct is honored exactly where it should be — the **paid surface**: a promoted finding leaves the topic groups and the cap's view (§4.5), so it stops taxing every read. But the file persists, and that persistence buys three specific things (§4.6).

This is the deliberate departure from the convention's "never regenerated" sentence, and it is what §5 (ADR impact) records: the finding **files** stay curated prose; the **index** joins the derived-view family.

### 4.3 Read contract — pay-per-relevance

The three hot readers change from "read `LEARNINGS.md`" to a two-step (gated by `learnings.enabled` — §4.5):

1. Load `learnings/README.md` (the index) **always** — a small, grouped hint surface.
2. Read the individual finding files whose index line (hook + topics) looks relevant to the change at hand.

- `docket-implement-next`: at plan time and at review.
- `docket-groom-next`: before a brainstorm.

When `learnings.enabled` is `false`, both steps are skipped entirely — the reader performs **zero** learnings reads and pays zero tokens (the whole point of the knob). No reader gains write access; the change is the reader wording in those three skills (plus the convention subsection that single-sources it).

### 4.4 Harvest — writes finding files (ADR-0005 write moments unchanged)

The harvest still runs **only at close-out**, single-sourced in `docket-finalize-change`'s *Harvest learnings* step and invoked **by reference** from `docket-status`'s sweep. What changes is the write target and shape (the whole step is gated by `learnings.enabled` — §4.5):

- For each lesson, the harvest either **creates a new finding file** (`learnings/<slug>.md`) or **extends an existing family finding** when the lesson belongs to one — append a dated `## War story` entry, add this change id to `changes:`, bump `updated:`. The bare-slug filename (§4.1) is the dedup key that makes "does a finding about this already exist?" a direct lookup. This formalizes the by-hand family consolidation the ledger already does.
- The harvest **never merges two existing distinct findings** — that is human-gated curation (§4.5). It only creates or extends.
- After writing/updating a finding file, the harvest **re-renders the index** (`render-learnings-index.sh > learnings/README.md`) and commits the finding file + index together, as **its own commit on `metadata_branch`** — separate from the archive commit (ADR-0005's rule, preserved), pushed immediately, and **committed only if the render actually changed bytes** (the `BOARD.md` commit-only-if-changed discipline; idempotency-keying family).

**Idempotency probe (survives the restructure).** The probe is now: **some finding file's `changes:` list already contains this change id** — read through the frontmatter lib (`list_field`), *not* a bare numeric grep (guards-are-code / shell-portability families: key on shape, not a spelling that can match elsewhere). Because the id is written into the *committed* finding file, the probe keys on the state the harvest actually promised — a crash before commit leaves nothing, and the next run re-harvests cleanly (idempotency-keying family: never key a no-op probe on a local proxy a half-run also satisfies).

### 4.5 Config surface — enable/disable, cap + escalation

**Deliverable: a nested `learnings:` config block** (mirrors `finalize:`), with two keys resolved through docket's standard layered resolution (`.docket.local.yml` > repo-committed `.docket.yml` > global `~/.config/docket/config.yml` > built-in) via `docket-config.sh --export`, which emits `LEARNINGS_ENABLED` and `LEARNINGS_CAP` (defaulted); skill bodies read the variables and never re-parse YAML (the `BOARD_SURFACES`/`SKILL_*` precedent):

```yaml
learnings:
  enabled: true    # default. false = the whole subsystem is a no-op read/write gate (files untouched)
  cap: 300         # default. active-finding count past which the harvest flags "needs curation"
```

**Enable gate (`learnings.enabled`, default `true`) — end to end.** The gate wraps the entire subsystem and short-circuits before any other learnings logic (including `cap`). When `false`:
- **Hot readers** (§4.3) skip the index and finding reads entirely — zero token cost.
- **The harvest** (§4.4, both drivers) **no-ops with a one-line note** — not silent. In `docket-finalize-change` the step prints one line (`learnings disabled — harvest skipped`); in the `docket-status` sweep the pass emits a single `learnings disabled` note (once per pass, not per change). The note is deliberate, not silence: the sole-channel/totality learning is that "no line" is indistinguishable from success, and ADR-0032's positive-off-state principle says a deliberate off-state is stated, not inferred from absence — a reader must be able to tell "harvested zero" from "skipped because disabled."
- **`docket-status`** skips the two learnings advisories (over-cap, promotion-pending — §4.5/§4.6) and the index self-heal render (§4.6); `render-learnings-index.sh` is simply never invoked.
- **Existing `learnings/` files are left untouched — never deleted.** Disabling is a read/write gate, not a purge. **Re-enabling picks up where it left off**: readers read the existing index again, the harvest resumes writing, and the next render re-derives the index from whatever finding files exist.

**Fence classification (ADR-0019): both keys are global-able.**
- `learnings.cap` gates only a human-facing *advisory*; nothing committed to a shared branch depends on its value, and the "over cap" signal is a self-healing derived view. Same class as `finalize.gate` and `board_surfaces` minus `github`.
- `learnings.enabled` is **global-able as well**, and the tension is worth stating honestly rather than waving through. A machine-locally disabled harvest means that machine's close-out sweeps skip an *enrichment write* other collaborators might expect — and a harvest entry is a curated, non-deterministically-re-derivable write, which sounds fenced. It is nonetheless global-able, because ADR-0019's fence protects **coordination integrity** (a global `metadata_branch` splits the backlog; a global generated-file value fails `--check` everywhere), and a skipped harvest has no analog to that failure: it *omits* enrichment, never writes conflicting or corrupt state, so there is never a "which ledger is authoritative" question. The resulting ledger is a strict subset of the enabled path's, and that subset is already inside ADR-0005's accepted envelope — zero-entry harvests are normal, unmerged-change lessons already die in the results file, kills are not harvested; the ledger was never a complete or guaranteed function of history. The one edge to name plainly (§7): because the harvest is a one-shot at close-out and archived changes are never re-swept, a change that *happens to close on a disabled machine* is permanently un-harvested — not caught up by a later enabled machine. That is a real, bounded loss of enrichment, accepted as the coherent consequence of a deliberate per-machine opt-out, and it degrades the hint surface without ever corrupting coordination state. The index, being deterministically re-derivable, self-heals the moment any enabled machine renders (the `BOARD.md`-staleness precedent exactly).

**Cap semantics + escalation (`learnings.cap`, default 300), evaluated only when enabled.**
- Measured against the count of **active findings** (`retained` + `candidate`) — equivalently the active hint-lines of the index, since the index is one line per finding. **Promoted findings do not count** — this is what makes promotion (§4.6) a real shrink valve: graduating a finding drops it below the cap's view. Measured as finding-count, not raw byte-lines, so topic-group headers don't inflate it.
- **Behavior at cap: the harvest never auto-merges.** Past the cap it surfaces `learnings over-cap — needs curation` through the digest/board **needs-you** channel (ADR-0028's ungated report channel). A human then curates — merge genuine near-duplicates, or promote must-fire rules. Rationale: LLM self-consolidation of its own memory is the dangerous operation (ACE "context collapse"); consolidation is human-gated by construction.

### 4.6 Promotion valve — harvest proposes, human applies

**Tiering criterion (verbatim):** *"will the agent know to search for this?"* A rule that must fire **unprompted** belongs always-in-context; a war story belongs in retrieval.

- The **target** is the repo's always-in-context agent-instructions file at the integration-branch root: **`AGENTS.md` or `CLAUDE.md`** — harness-agnostic, symlink-aware (if one symlinks to the other, one write covers both). **Neither exists on `main` in this repo today** (verified), so the migration's graduation candidates have no destination yet: the design **recommends creating `AGENTS.md`** as the neutral spelling when neither exists — a human decision, surfaced, not taken by the loop.
- **The harvest never touches the integration branch** (ADR-0005's never-publish rule holds). When a lesson meets the tiering criterion, the harvest sets `promotion_state: candidate` in the finding's frontmatter on `metadata_branch`. The digest surfaces `learnings promotion-pending <count> — needs you`, naming the resolved target file (docket probes the integration-branch root for `AGENTS.md`/`CLAUDE.md`, symlink-aware, to give a precise pointer; recommends `AGENTS.md` when neither is found).
- **A human lands the graduation**: edits `AGENTS.md`/`CLAUDE.md` on the integration branch via a normal commit/PR, then flips the finding's `promotion_state: promoted` and sets `promoted_to:` on `metadata_branch`. The index then drops the finding from the topic groups into the compressed `## Promoted` appendix (§4.2), and it stops counting against the cap.

**The promoted finding file is kept, not deleted.** The human's instinct to *remove* the graduated lesson is right about the **paid surface** and wrong about the file — the two are separated here precisely so the removal is free of cost. Keeping the file buys three concrete things:
- **(a) The receipt.** The finding is the linked provenance for *why* the `AGENTS.md` rule exists — the war story and the changes that earned it. A one-line always-in-context rule with no traceable origin is exactly the kind of instruction that later gets "cleaned up" by someone who doesn't know what it cost.
- **(b) Harvest memory.** A future change that re-hits the same failure class **extends the existing finding** (adds its change id to `changes:`, appends a war-story entry) instead of minting a **duplicate** for a lesson that has *already graduated*. Delete the file and the dedup key (§4.4) is gone, so the harvest can't tell the class is already known — it re-mints, and the graduated rule silently sprouts a shadow in the retrieval tier.
- **(c) Reversibility.** If the always-in-context rule proves noisy or wrong, **demotion is a one-line frontmatter flip** (`promoted → retained`, clear `promoted_to:`) that returns the finding to the hint surface — not git archaeology to reconstruct a deleted file.

- **Self-heal for the human flip.** The harvest re-renders the index when *it* writes; a human's manual `promotion_state` flip is refreshed by **`docket-status`'s pass**, which re-renders `learnings/README.md` as a derived view each pass (commit-only-if-changed), exactly as it self-heals `BOARD.md` — so the index never trails the finding files regardless of which writer moved them. Both the advisory-surfacing and this self-heal render are inside the `learnings.enabled` gate (§4.5) — disabled, docket-status does neither. (Whether the render rides the existing Board pass or a sibling step in the `docket-status` orchestrator is build-time detail; the contract is "the index is a derived view and must never trail the finding files.")

### 4.7 Migration — the build's acceptance proof

The build converts the current 491-line `LEARNINGS.md` into finding files + a rendered index:

- Each existing consolidated **family** entry becomes one finding file: `guards-are-code`, `enumerated-floor`, `moving-base`, `verify-the-claim`, `green-suite-untested-branch`, `sole-channel`, `pipefail`, `idempotency-keying`, `shell-portability`, `yaml-scalar`, `adr-update-delivery`, `environment` — its `changes:` list carrying every cited id, its `## War story` carrying the dated `(#<id>, PR #<n>)` sub-entries verbatim.
- Each standalone dated entry becomes its own finding file.
- **Obvious graduation candidates** get `promotion_state: candidate` for the human to review (criterion-first, not a fixed list; e.g. the always-fire `Apply:` rules such as pipefail's "never `producer | early-exiting-consumer` under `pipefail`" read as must-fire-unprompted). Specific candidate selection is a build-time judgment.
- **`LEARNINGS.md` is left as a minimal pointer stub** ("→ moved to `learnings/`; see `learnings/README.md`"), not deleted — following docket's own learned convention (LEARNINGS #20, 2026-06-17: *leave a stub + pointer under the original heading so name-based cross-refs still resolve*). Git history keeps the full pre-migration ledger; the stub costs one line and prevents a "where did the ledger go" moment for a human or an older skill copy.
- Because metadata-branch files are invisible to the integration-branch test suite, the migration is verified at build time and recorded in the results file (LEARNINGS #6, 2026-06-12).

### 4.8 Prose / skill / test / README updates in scope

**Where each artifact lands.** This change spans both branches, which the build must plan for. The migrated **finding files, the rendered index, and the `LEARNINGS.md` stub** are runtime data on `metadata_branch` (like `BOARD.md` and today's ledger) — written at build time, invisible to the integration-branch suite (verify per LEARNINGS #6). Everything else — the **skill and convention edits, `render-learnings-index.sh` + contract, `config.yml.example`, the sample `.docket.yml` block, the README section, tests, and the recommended `AGENTS.md`** — is product code/docs on the **integration branch**, shipping via the change's normal feature-branch PR.

- **`docket-convention`** — rewrite the *Learnings ledger* subsection: directory path, finding-file frontmatter schema, the index-is-derived rule (and the finding-files-stay-prose boundary), the read contract, the harvest write shape + idempotency probe, the `learnings.enabled`/`learnings.cap` keys + fence classes + disabled semantics, the promotion states + target, and the derived-view family membership of `render-learnings-index.sh`. Add `learnings/` to the Directory-layout block. Single source per ADR-0003.
- **`docket-finalize-change`** — rewrite the *Harvest learnings* step (§4.4) as the single source, including the `learnings.enabled` gate + one-line disabled note.
- **`docket-status`** — the sweep invokes the harvest by reference (unchanged pattern); add the learnings-index derived-view refresh and the two needs-you advisories (`over-cap`, `promotion-pending`), all inside the enable gate, plus the once-per-pass `learnings disabled` note.
- **`docket-implement-next`** / **`docket-groom-next`** — the two-step read contract (§4.3), gated on `learnings.enabled`.
- **`config.yml.example`** / commented `.docket.yml` — add the `learnings:` block with both keys (commented, with defaults and the disabled meaning). LEARNINGS #49: a new knob ships end-to-end — sample config + README + relaxed-requirement prose in the same change.
- **Integration-branch `README.md` — a new first-class feature section (Addition 2).** The README currently says nothing about the learnings system; add a section presenting it as a real capability: *the repo gets smarter as changes ship.* **Altitude/sketch (not final prose — authored at build time):** a heading in the factual register (e.g. `## Learnings — the loop's memory`), then a short paragraph and a compact list covering (1) the **close-out harvest** — every change reaching `done` distills its PR-review/merge-gate/results lessons into a curated finding (zero is normal); (2) the **findings + rendered index** structure — one file per lesson/family plus a generated index, on the metadata branch; (3) **pay-per-relevance reads** — groom/plan/review load the index and pull only relevant findings; (4) **human-gated promotion** — must-fire rules graduate into `AGENTS.md`/`CLAUDE.md`, never automatically; (5) the **controls** — `learnings.enabled` to turn it off wholesale and `learnings.cap` for the curation threshold. Tone: factual and concrete, let the mechanism sell itself, no marketing-speak. It should **point to the convention's *Learnings ledger* subsection for mechanics rather than restating them** — prose restating a fact owned by another file is a drift surface (verify-the-claim family); the README sells and orients, the convention remains the single source.
- **Tests** — structural sentinels following the suite's pattern: the harvest procedure's single source; the two reader references; the convention subsection; `render-learnings-index.sh` determinism/idempotency (byte-identical on identical inputs, like the `render-adr-index.sh` tests); the promoted-appendix and candidate-marker rendering; the idempotency probe over `changes:`; and the **`learnings.enabled: false` path** — readers skip, harvest no-ops with its note, `render-learnings-index.sh` is not invoked, and existing `learnings/` files are left byte-untouched (assert a pre-seeded finding survives a disabled sweep). Every new grep sentinel must be mutation-tested (guards-are-code family), keyed on syntactic shape not an enumerated spelling, and the corpus itself derived not hand-listed (enumerated-floor family). Behavioral verification of a real harvest/promotion/disable is a merge-gate concern recorded in the results file.

## 5. ADR impact

**A new ADR (next free id — assigned at `docket-adr` time) is required, and it stays a single ADR.** It records six decisions:

1. **The learnings ledger is a findings directory plus a rendered, derived index** — the finding *files* stay curated prose (single-writer, never regenerated); the *index* joins the derived-view family with its own sole-writer renderer (`render-learnings-index.sh`). This is the explicit departure from the convention's former "the ledger is prose, never regenerated" statement, and from the single-file structure change 0006 shipped.
2. **Finding files are named by bare `<slug>.md`, with no index ordinal — deliberately unlike ADRs.** ADR numbers encode immutable, ordered records; findings are living files extended on every re-hit of a class. The slug is the harvest's dedup key and avoids a number-minting race between concurrent autonomous sweeps; chronology lives in `created:`/`updated:` and the dated war-story entries. (Human-requested first-class decision.)
3. **The promotion valve**: harvest sets `candidate` on `metadata_branch`; a human graduates must-fire-unprompted rules to the integration-branch agent-instructions file (`AGENTS.md`/`CLAUDE.md`, symlink-aware, `AGENTS.md` the neutral spelling when neither exists) and flips `promoted`; the harvest never touches the integration branch — preserving ADR-0005's never-publish rule.
4. **A promoted finding keeps its file** — it leaves the paid surface (topic groups + cap view) but is never deleted, because the file is the graduated rule's receipt/provenance, the harvest's dedup memory against re-minting a duplicate for an already-graduated class, and a one-line-reversible demotion path. Decay via status metadata, not deletion (ADR/postmortem practice). (Human-requested first-class decision.)
5. **Consolidation is human-gated, never autonomous**: the cap flags (`learnings over-cap — needs you`); the harvest never auto-merges its own memory (ACE context-collapse rationale). The tiering criterion is *"will the agent know to search for this?"*, with states `retained | candidate | promoted`.
6. **Config controls and their fence classifications**: `promotion_state`'s positive off-state (`retained`) per ADR-0032; and the `learnings:` block per ADR-0019 — `learnings.cap` (default 300) **global-able**, and `learnings.enabled` (default `true`) **global-able and defined as a read/write gate, not a purge**. The enable knob is global-able because a machine-local disable *omits* enrichment writes without corrupting coordination state (no divergence, no "which ledger is authoritative" question, the index self-heals on any enabled render), with the accepted edge that a change closing on a disabled machine is permanently un-harvested — a strict subset of ADR-0005's already-accepted omission envelope.

**One ADR, not two.** All six are facets of a single "learnings ledger restructure" decision with no independent-reversal seam: you cannot reverse the bare-slug naming (#2) without unwinding the living-file model that the index (#1), the harvest dedup (#4b), and the promotion valve (#3) all rest on; the keep-promoted-file rule (#4) is only coherent given the promotion valve (#3) and the derived-index/cap split (#1, #6); and the enable gate (#6) is an on/off switch for the whole subsystem, meaningless to reverse apart from it. Splitting them would create ADRs that can only ever be superseded together — the anti-pattern ADR granularity exists to avoid. If a later change reverses just one facet (say, promotion), that is its own superseding ADR at that time.

**Relationship to ADR-0005: relates_to, not supersedes.** ADR-0005's decision — harvest only at close-out, single writer, single moment, ledger unpublished, idempotency probe — stands **unchanged**; only its founding *consequence* ("short enough to actually be read") is what failed, and the new ADR's Context records that failure as its motivation. The new ADR sets `relates_to: [5]` (and `change: 67`). ADR-0005 stays `Accepted`; optionally a dated `## Update` note is appended to ADR-0005 pointing forward to the new ADR, delivered by keeping ADR-0005 in this change's `adrs:` so terminal-publish re-copies it onto the integration branch atomically (ADR-update-delivery family). Because ADR-0005 stays `Accepted`, the finalize Accepted-gate copies it normally — no `terminal-publish --adr` follow-up is needed.

## 6. Assumptions

Every judgment call made in place of asking:

1. **Directory + names.** The findings directory is `<changes_dir>/learnings/` (default `docs/changes/learnings/`); finding files are **bare `<slug>.md` with no index ordinal** (rationale in §4.1/§5.2: living files, slug = harvest dedup key, no number-minting race between concurrent sweeps, chronology in `created:`/`updated:` + war story); the index is `learnings/README.md`. The renderer is `scripts/render-learnings-index.sh` with contract `scripts/render-learnings-index.md`, in the derived-view family. Directory shape chosen for symmetry with `<adrs_dir>/` + `render-adr-index.sh`; the filename deliberately diverges.
2. **Config keys.** A nested `learnings:` block (mirrors `finalize:`) with `enabled` (bool, default `true`) and `cap` (int, default 300); resolved via the standard `.docket.local.yml` > `.docket.yml` > global > built-in chain, exported as `LEARNINGS_ENABLED`/`LEARNINGS_CAP`. Both **global-able** (ADR-0019). Chosen over flat `learnings_enabled:`/`learnings_cap:` for room to grow and `finalize:` symmetry.
3. **Enable gate is a read/write gate, not a purge.** Disabled ⇒ readers skip, harvest no-ops with a **one-line note** (per close-out / once per status pass — not silent, not per-change; justified by ADR-0032 positive-off-state + the sole-channel totality learning), advisories + self-heal render skipped, renderer never invoked, existing `learnings/` files byte-untouched, re-enable resumes from the existing files. The gate short-circuits before `cap` is consulted.
4. **Cap counts active findings only** (`retained` + `candidate`), not promoted ones — so promotion is a genuine shrink valve. Measured as finding-count (≈ index active-lines by construction), not raw byte-lines.
5. **`promotion_state` enum** is `retained | candidate | promoted`, underscore-spelled per docket frontmatter convention, default `retained` (positive off-state, ADR-0032).
6. **Frontmatter schema** keeps provenance as a flat int `changes:` list (idempotency key, reuses `list_field`) plus PR numbers in the dated body, rather than a list-of-maps the grep/awk reader can't parse. The `hook` scalar is quoted (colon-space, YAML-scalar family). Date fields reuse the change manifest's `created:`/`updated:` spellings (one vocabulary, no new parse).
7. **Idempotency probe** keys on `changes:` containing the id in a committed finding file, read via the frontmatter lib (shape, not a bare numeric grep).
8. **Promoted findings keep their file and drop to a compressed `## Promoted` appendix** (with `→ promoted_to`), never deleted and never left inline — decay via status metadata, mirroring the ADR index's superseded group. Removal is honored at the paid surface (topic groups + cap view) only; the file persists for three concrete buys (§4.6): (a) the receipt/provenance for the graduated `AGENTS.md` rule, (b) harvest dedup memory so a re-hit extends rather than re-mints, (c) one-line-flip reversibility of demotion.
9. **`LEARNINGS.md` is left as a pointer stub**, not deleted (docket's own stub-and-pointer convention, LEARNINGS #20).
10. **Promotion target** is `AGENTS.md`/`CLAUDE.md` at the integration-branch root, symlink-aware; **recommend creating `AGENTS.md`** when neither exists (this repo's current state). The loop only *recommends and points*; the human makes and lands the edit and the branch flip.
11. **Self-heal.** `docket-status`'s pass re-renders the index each pass (commit-only-if-changed) when enabled, so a human's manual `promotion_state` flip refreshes the index without a dedicated command — matching how `BOARD.md` self-heals. Whether this folds into the Board pass or a sibling orchestrator step is left to build-time.
12. **Index grouping** is by the finding's **primary topic** (first `topics:` tag); a finding appears once; remaining tags render inline. Chosen so cap-counting and the ADR-index "each item once" discipline both hold.
13. **README section** is a first-class feature section on the integration-branch `README.md` (ships via the feature-branch PR), factual register, sketched at §4.8 altitude — it **points to** the convention subsection for mechanics rather than restating them (verify-the-claim drift avoidance). Final prose is build-time.
14. **The restructure is one ADR** (§5), relating to ADR-0005 rather than superseding it, because ADR-0005's decision is unchanged; only its consequence dated. A forward `## Update` on ADR-0005 is advised-but-optional.
15. **The harvest may create or extend a finding, but never merges two distinct findings** — that stays human-gated curation.

## 7. Risks & open follow-ups

- **Disabled-machine harvest is lossy, by design.** Because the harvest is a one-shot at close-out and archived changes are never re-swept, a change that closes on a machine with `learnings.enabled: false` is **permanently un-harvested** — no later enabled machine catches it up. This is the honest cost of making the knob global-able (§4.5). Accepted because it is a strict subset of ADR-0005's already-normal omissions (zero-entry harvests, results-file-only lessons, unharvested kills) and degrades enrichment without ever corrupting coordination state — but a team that runs mixed enabled/disabled machines should know their disabled machines' close-outs won't enrich the shared ledger. If this proves painful, a follow-up could let an enabled sweep re-harvest recently-archived changes lacking a finding — deliberately out of scope here (it reopens ADR-0005's single-moment rule).
- **Hook quality gates retrieval.** The index is a hint surface; if a finding's `hook` under-describes it, a relevant finding won't be pulled. Mitigation: hooks are the distilled `Apply:` rule (already the ledger's sharpest line), and topics render inline. A genuine bet on the index-as-hint pattern — worth a later look at whether hooks need a length/quality lint.
- **Promotion depends on a human loop.** If graduation candidates pile up unpromoted, the cap can still trip with an all-`retained`/`candidate` surface — which is the designed escalation (`needs you`), not a failure. The shrink valve is only as live as the human's attention to the promotion-pending advisory.
- **README as a drift surface.** A README section describing the mechanics would drift from the convention (verify-the-claim family) — mitigated by keeping it orientation + a pointer, with the convention as the single source of mechanics.
- **Two derived views to keep from trailing.** Adding `learnings/README.md` to the set `docket-status` keeps fresh doubles the "derived view must never trail" surface (ADR-0032 territory). The commit-only-if-changed discipline and a determinism test are the guardrails; the sole-writer renderer must not be invoked by any skill by hand.
- **Follow-up (out of scope):** if `AGENTS.md`/`CLAUDE.md` graduation proves frequent, a later change could add a `docket learnings promote <slug>` helper that renders the appendix flip and stages the pointer — but promotion stays human-driven for now; the manual-flip + status self-heal path is deliberately v1.
- **Follow-up:** whether promoted findings should *ever* be prunable from `metadata_branch` is left open — this design keeps them indefinitely (§4.6 buys (a)/(b)/(c) all argue for retention), matching ADR/postmortem decay-not-delete practice.
