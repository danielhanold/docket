# Design: script docket-adr's deterministic passes (change 0030)

**Status:** design (brainstormed 2026-06-20)
**Change:** 0030
**Builds on:** 0022 (`render-board.sh` + `lib/docket-frontmatter.sh`), 0023 (`board-checks.sh`), 0025 (`terminal-publish.sh`), 0026 (`docket-config.sh`).

---

## 1. Context

The scripting sweep (changes 0022/0023/0025/0026) lifted every deterministic
pass out of the `docket-status` / `docket-finalize-change` family into tested
shell, governed by **ADR-0012**'s rule: a pass moves into a script when it is
**mechanical** (no reading intent from free text) and **free of
terminal-transition side effects shared across callers** (or, if shared, owned by
*one* jointly-owned script). `docket-adr` is the one operating skill the sweep
never reached, so three of its passes are still model-prose even though each
passes ADR-0012's test cleanly. This change brings `docket-adr` to parity.

The three passes, each with a scripted sibling that already exists:

| Pass | Today (prose) | Sibling already scripted |
|---|---|---|
| ADR index render (`<adrs_dir>/README.md`) | `docket-adr` "Index / validate" | `render-board.sh` (0022) — `BOARD.md` |
| ADR ledger validation | `docket-adr` "Validate the ledger and flag" | `board-checks.sh` (0023) — 5 change checks |
| ADR-only terminal-publish (`T = adr-<NN>`) | `docket-finalize-change` "The mechanics" (run **by hand**) | `terminal-publish.sh` (0025) — the change-publish path |

The index render is the sharpest gap: `<adrs_dir>/README.md` literally carries
the line *"This index is generated — do not hand-edit"* yet has no generator, so
it is regenerated from prose on the model's tokens every time. The cost of
re-deriving it by hand also shows as **drift**: the published copy on `main`
currently lists only ADR-0001/0002 while the authoritative copy on `docket` lists
all twelve. A deterministic generator makes "same ADR files ⇒ byte-identical
index" true and lets the index re-render heal that drift on the next pass.

The ADR-only publish is the **last hand-run git sequence embedded in a skill
body**: `terminal-publish.sh` is keyed on `--id` (it builds its copy-set from an
archived *change* file), so the `T = adr-<NN>` path — copy-set = one ADR file,
step-1 archive skipped — is performed by the literal bash in
`docket-finalize-change`'s *The mechanics* section. The generic
provision→copy→CAS-push→teardown mechanics are byte-for-byte the same as the
scripted change-publish path; ADR-0012's guidance for shared terminal-transition
mechanics is to own them in **one** script, never duplicate per-caller.

This change ships **two new scripts**, **one extended script**, the **skill
wiring** that retires the replaced prose, and **one new ADR** recording the
boundary extension. No ADR or board *semantics* change — faithful
re-implementation only.

## 2. `render-adr-index.sh` — the renderer

**CLI:** `render-adr-index.sh --adrs-dir DIR`

- Sources `lib/docket-frontmatter.sh`; reads `DIR/*.md` excluding `README.md`.
- **Emits the index to stdout** — no git writes (caller redirects + commits, the
  same discipline as `render-board.sh`). Deterministic + idempotent: same ADR
  files ⇒ byte-identical output. **Offline** — pure filesystem, no `gh`, no `git`,
  no network (it reads the working-tree files in the metadata tree).

**Output = `docket-adr`'s *Index / validate* structure, pinned here so a golden
test locks it byte-for-byte:**

- **Header** — the fixed three-line preamble (`# Architecture Decision Records` +
  the immutability blurb + "This index is generated — do not hand-edit").
- **Three `##` groups, always emitted in this order**, each row sorted by
  **ascending numeric id**; an empty group renders `_None._`:
  1. **Active** — `status: Accepted` (and any draft/`Proposed`, which sort here too).
  2. **Superseded / Reversed** — `status` matching `Superseded by ADR-NN` / `Reversed by ADR-NN`.
  3. **Deprecated** — `status: Deprecated`.
- **Row:** `- [ADR-NNNN](<file>.md) — <title> (<status>)` followed by annotations
  in fixed order:
  - `← change #N` when `change:` is set;
  - `→ supersedes ADR-NN` / `→ reverses ADR-NN` when `supersedes:` / `reverses:` non-empty (Active rows);
  - `· relates to ADR-NN[, ADR-NN]` when `relates_to:` non-empty.
  - The `· ` / ` ` separators and ordering match the current committed index
    exactly (the golden fixture is the contract).
- **No generated-at timestamp** (consistent with `render-board.sh`).

The id→title/status map needed for the `ADR-NN` annotation text is built in the
same single scan (so an annotation can never disagree with the row it points at).

## 3. `adr-checks.sh` — the validator

**CLI:** `adr-checks.sh --adrs-dir DIR [--strict]` — the exact shape of
`board-checks.sh`.

- Sources `lib/docket-frontmatter.sh`; reads `DIR/*.md` (excluding `README.md`).
- Emits one finding per line on stdout, **TAB-separated
  `<check-id>\t<adr-id>\t<message>`**, sorted by `(check-id, adr-id)`. A clean
  ledger prints nothing and exits 0. `--strict` ⇒ exit 1 on any finding (a future
  CI gate). Pure filesystem — no `gh`, no network; **warn-only**, never auto-fixes.
- `check-id ∈ {adr-numbering-gap, adr-dangling-link, adr-status-inconsistent}`:
  - **`adr-numbering-gap`** — an id missing from the `1..max` sequence (one finding per gap).
  - **`adr-dangling-link`** — a `supersedes:` / `reverses:` / `relates_to:` value
    referencing an id with no corresponding file.
  - **`adr-status-inconsistent`** — either side of a broken supersession/reversal:
    (a) an ADR whose `status:` says `Superseded by ADR-NN` / `Reversed by ADR-NN`
    but no ADR NN exists; **or** (b) an ADR that `supersedes:`/`reverses:` another
    whose target's `status:` was **not** flipped to point back at it.

## 4. `terminal-publish.sh` — add an ADR-only mode

Extend the existing script with **`--adr <NN>`** as the alternative to `--id <N>`
(mutually exclusive; exactly one required). ADR mode reuses the generic
provision→copy→CAS-push→self-verify→teardown machinery unchanged; only the
copy-set construction and the throwaway-branch token differ:

- **Token** `T = adr-<NN>` → worktree branch `pub-adr-<NN>` (today's `--id` path
  stays `pub-<id>`); all `pub-$ID` references parameterize on `T`.
- **Copy-set** = the single ADR file, resolved on `origin/<metadata-branch>` by id
  (`<adrs_dir>/<NNNN>-*.md`). **Step-1 archive is skipped** (no change file).
- **No `Accepted` gate** in ADR mode — the caller (`docket-adr`) invokes it only
  when publishing is intended, *including* a status-line flip
  (Superseded/Reversed/Deprecated), so the script copies the ADR's current bytes
  as-is. (The `Accepted` gate stays on the `--id` path, which filters a change's
  `adrs:` list.)
- **Default message** `docket(adr-NN): publish ADR-NN`.
- The **main-mode no-op guard** and the **fail-closed self-verify** (re-fetch,
  assert the copy-set landed on `origin/<integration>`) apply identically.

This makes `terminal-publish.sh` the single executor of *both* publish shapes;
`docket-finalize-change`'s *The mechanics* section stops being a runbook the model
executes by hand and becomes documentation of what the script does.

## 5. Skill wiring (retire the replaced prose)

- **`docket-adr` → *Index / validate*.** Replace the hand-render prose with
  "invoke `scripts/render-adr-index.sh --adrs-dir <metadata tree>/<adrs_dir>
  > …/README.md`"; keep the **separate-index-commit** discipline and the
  **regenerate-don't-3-way-merge** conflict rule (now literally "re-run the
  script"). Replace the "Validate the ledger and flag …" prose with "invoke
  `scripts/adr-checks.sh --adrs-dir <metadata tree>/<adrs_dir>` and surface each
  finding line"; keep the human-readable description of *what* the three checks
  cover (as `docket-status` kept its check descriptions atop `board-checks.sh`).
- **`docket-adr` → publish references.** The two "this skill's own ADR-only
  terminal-publish invocation" references (standalone ADR; status-change
  re-publish) now name `scripts/terminal-publish.sh --adr <NN> …`.
- **`docket-finalize-change` → *Terminal publish* / *The mechanics*.** The
  ADR-only path's by-hand provision/copy/CAS/teardown block becomes a
  `terminal-publish.sh --adr <NN>` call; the generic-mechanics prose remains as
  the documented contract the script implements (parallel to how the change-publish
  mechanics are documented even though `terminal-publish.sh --id` automates them).
- **Mode handling unchanged** — the skills already point the metadata tree at
  `.docket/` (docket-mode) or the primary tree (main-mode); the scripts only need
  the dir / the existing `--metadata-branch`/`--integration-branch` args.

## 6. The new ADR (produced at build time)

A short ADR recording that **ADR-0012's script-vs-model boundary now extends to
the `docket-adr` surface**: the index render and ledger validation are mechanical
and script-owned (analogs of `render-board.sh` / `board-checks.sh`), and the
ADR-only terminal-publish is folded into the **shared** `terminal-publish.sh`
rather than duplicated per-caller — the literal application of ADR-0012's
"shared terminal-transition mechanics owned by one script." `relates_to:
[12, 7, 2]`. Authored by the implementer's `docket-adr` step-6 dispatch and
appended to this change's `adrs:`.

## 7. Scope

**In scope:** `scripts/render-adr-index.sh`; `scripts/adr-checks.sh`;
`terminal-publish.sh` `--adr` mode; the `docket-adr` + `docket-finalize-change`
SKILL edits; the new ADR; `tests/test_render_adr_index.sh` (golden + idempotence),
`tests/test_adr_checks.sh` (one fixture per check + a clean ledger), and a
`terminal-publish.sh --adr` test in the existing terminal-publish harness.

**Out of scope:**
- **ADR or board *semantics*** — the index grouping, row format, the three checks,
  and the publish mechanics are reproduced exactly from `docket-adr` /
  `docket-finalize-change`, not redesigned.
- **The metadata-worktree ensure-and-sync ritual** (the cross-cutting Tier-2
  dedup) — a separate extraction, not this change.
- **Next-id allocation** — stays in-model (welded to the CAS-push-rename retry);
  at most a future `lib/` helper.
- **`yq`** (0018) — these passes read flat frontmatter only; `field`/`list_field`
  already cover 100%, same finding as 0022 §4. `related` to 0018, not `depends_on`.
- **Back-filling the stale `main` ADR index** beyond what the next normal
  index-render pass heals — no manual history rewrite.

## 8. Test plan

- **`tests/test_render_adr_index.sh`** — a fixture `adrs/` tree spanning Active
  (with `change:`, `relates_to:`, and `→ supersedes`/`→ reverses` annotations),
  a Superseded and a Reversed entry, and a Deprecated entry, rendered and compared
  **byte-for-byte to a committed golden `README.md`**; plus an idempotence
  assertion (re-render ⇒ identical bytes) and an empty-group (`_None._`) case.
- **`tests/test_adr_checks.sh`** — fixtures triggering each check
  (`adr-numbering-gap`, `adr-dangling-link`, both arms of `adr-status-inconsistent`)
  and a clean ledger asserting **no output, exit 0**; plus a `--strict` exit-1
  assertion.
- **`terminal-publish.sh --adr`** — extend `tests/` with a hermetic bare-origin
  fixture: a standalone Accepted ADR on `docket` publishes to the integration
  branch (copy-set lands, self-verify passes, `pub-adr-NN` worktree torn down),
  the run is idempotent (re-run = no-op), and main-mode is a no-op. The existing
  `--id` change-publish tests stay green (regression gate).
