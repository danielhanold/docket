<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0266 — Deterministic frontmatter field writer — no skill hand-rolls a manifest edit](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-03-0266-deterministic-frontmatter-field-writer-no-skill-hand-rolls-a.md)**
<!-- docket:backlink:end -->

# Deterministic frontmatter field writer — design (change 0266)

## Problem

Every operating skill writes manifest frontmatter fields (`status:`, `branch:`, `claimed_at:`,
`plan:`, `adrs:`, `pr:`, `results:`, …) on essentially every run, and each write is hand-rolled
per run, per agent, in whatever regex dialect the agent reaches for. Change 0140's Step 7 showed
the failure mode: a `perl -0pi` with `/s` made `.*$` greedy to EOF and truncated the file below
the frontmatter — committed and pushed before anyone noticed, because the spot-check read only the
edited lines. Meanwhile two scripts (`reclaim-claims.sh`, `archive-change.sh`) each carry their own
private `set_field()` duplicate, and two AGENTS.md prose rules exist only because no writer
enforces them mechanically. This is squarely ADR-0012 territory: deterministic mechanics belong in
a script.

## Design

One new helper, `scripts/set-field.sh`, with co-located contract `scripts/set-field.md`, reached
through the facade as `docket.sh set-field` (new row in `docket.md`'s subcommand inventory; the
facade test derives both sets by grep, so the row and the dispatch entry land together).

### CLI

```
set-field.sh --file FILE --kind {change|adr} [--raw KEY=VALUE ...] KEY=VALUE [KEY=VALUE ...]
```

- `FILE` is a change file or an ADR file — any markdown file opening with a `---…---` frontmatter
  block. The writer is **key-agnostic about policy**: which keys may move to which values, and
  when, stays owned by the skills.
- `--kind` is **caller-declared, required** — the writer never infers file kind from content
  (shape inference is the defect class; the caller always knows which corpus it is editing). Kind
  scopes the append gate below.
- A plain `KEY=VALUE` pair is always a **scalar** (or an empty clear). A pair passed via
  `--raw KEY=VALUE` is written **verbatim** — the caller's explicit declaration that the value is
  a flow collection (`adrs=[24, 71]`, `depends_on=[3]`); a raw value must start with `[`, end
  with `]`, and carry no control characters, or the invocation refuses.
- Each positional arg splits on the **first** `=`; `KEY=` (empty value) clears the field to the
  bare `key:` form with no trailing space (an empty scalar is well-formed YAML null). This
  deliberately **normalizes** today's `key: ` colon-space-trailing form that the two private
  `set_field()` duplicates emit — readers tolerate both, and the writer picks the
  trailing-whitespace-free spelling.
- `KEY` must match `^[a-z][a-z0-9_]*$` — a key is data, never a pattern, so it is gated before it
  ever meets the anchor regex; anything else (spaces, regex metacharacters, uppercase) ⇒ refuse.
- Multiple pairs per invocation; **all pairs are validated before any write** (key shape, key
  resolvable per the presence rules below, values carry no control characters, no duplicate key
  in the arg list), then the file is rewritten **once** to a sibling temp file and atomically
  `mv -f`-renamed over the original.

### Behavior and refusals (all leave the file byte-untouched, non-zero exit)

1. **Fence validation first**: line 1 must be exactly `---` and a closing `---` line must exist.
   Malformed or missing fence ⇒ refuse. All edits are confined to that first block; everything
   below the closing fence is emitted byte-identical.
2. **Presence rules — replace, append-if-absent-capable (change files only), else refuse.** A
   `KEY` matching an existing `key:` line inside the block is replaced in place. A `KEY` absent
   from the block is **appended** immediately before the closing fence only when `--kind change`
   AND the key belongs to the **change-file absent-capable roster**. That roster today exists
   only as a prose comment in `scripts/lib/docket-frontmatter.sh`; this change **promotes it to a
   declared, sourceable array in that lib** (e.g. `DOCKET_CHANGE_ABSENT_CAPABLE_KEYS`, alongside
   `DOCKET_STATUSES`), scoped to the change-file corpus — so `promotion_state`/`promoted_to`,
   which are learnings-finding keys, are **excluded**, and `reconciled` is **included**: it is
   unclassified in the lib's prose today, yet `reclaim-claims.sh` writes it at a migrated site
   and it is semantically optional and hand-authorable, the lib's own test for the set. The
   roster's content is derived at build time from a whole-repo grep of migrated-site writes
   reconciled against the prose list (per AGENTS.md's never-hand-list rule), not copied
   verbatim. The prose comment is repointed at the array for the change-corpus keys while
   keeping the learnings-corpus keys documented in prose (the array is change-only, so a plain
   repoint would drop them); the census test is updated if it parses the prose today. The lib stays the single owner
   of the schema fact; the writer sources the array, never restates it. `--kind adr` **refuses
   every absent key** — ADR keys are always present in that corpus, so an absent one is a typo or
   the wrong file. This keeps a typo'd key from silently minting a field while legacy
   pre-template change files (hundreds predate template revisions) and the `issue:` mint flow
   (`github-mirror.sh` emits `issue-minted` for the caller to persist into a field 0266's own
   manifest lacks) remain writable through the one path. (The stub's boundary — "introduces no
   new frontmatter field" — governs the *schema*, not a documented existing key absent from an
   old file.)
3. **Duplicate `key:` lines inside the fence** (malformed but reachable) ⇒ refuse — replacing
   "the first" would leave a second, contradictory line the next reader may hit.
4. **Control characters in a value** ⇒ refuse (the `mint-stub.sh` precedent: no legitimate field
   value carries one, and a newline in a value is a frontmatter injection).
5. Key matching anchors on `^key:` **inside the fence range only**, with `[[:blank:]]*` (never
   `\s`) after the colon — the AGENTS.md anchoring rule, now enforced by construction.

### Value serialization

Three shapes, decided by **caller declaration, never shape inference**:

- **Flow collection** (`--raw` pairs only): written **verbatim** — quoting would change the
  parsed type from sequence to string (the AGENTS.md flow-collection carve-out). This is how
  `adrs: [24, 71]` and `depends_on: [3]` are written. A **plain** pair whose value is
  bracket-wrapped ⇒ **refuse** with a message pointing at `--raw`: the lib's own
  `docket_scalar_quote_reason` commentary documents why bracket-sniffing is unsound
  (`[a title: with colon]`, `[WIP] rework]`) — sequence-vs-string intent is a call-site
  decision, so the writer demands the declaration rather than inferring.
- **Empty**: written as `key:` (see above).
- **Scalar**: single-quoted **unconditionally** via the existing `docket_yaml_single_quote`
  (interior `'` doubled) — the house rule (ADR-0071, restated in the lib's own comments: "a
  writer must quote unconditionally, never predicate on this"). This is safe because docket's
  production read tier is uniformly quote-stripping: `field()`/`fm_field()` route through
  `_docket_unwrap_quotes`, and a repo census found zero production greps of bare
  `status:`/`trivial:` values in `scripts/*.sh` or skill bodies (bare spellings live only in
  test fixtures, which the writer never touches). Any fixture or ad-hoc grep the migration does
  surface is updated in the same change — a bounded, mechanical cost.

### Implementation home and dedup

The write function (`docket_set_frontmatter_fields`, name illustrative) lives in
`scripts/lib/docket-frontmatter.sh` beside the read accessors it must round-trip with
(`field()` / `_docket_unwrap_quotes` already strip a matched quote pair, so quoted output reads
back as the logical value). `set-field.sh` is the thin CLI over it. The two in-tree duplicates —
`reclaim-claims.sh`'s and `archive-change.sh`'s private `set_field()` — are migrated to source the
shared function, so the repo ends with exactly one frontmatter write path. **Deliberate behavior
change at those sites**: the duplicates silently no-op on a missing key (a legacy archived file
lacking `claimed_at:`/`results:` today loses the write without a trace); under the shared
function those keys are in the absent-capable set and are **appended**, so the value actually
lands — an improvement, called out in the contract and covered by a legacy-file test so the
archive path is proven, not assumed. Two further migrated-site knock-ons the contract names
explicitly: (i) on a file with **duplicate `key:` lines** the old sed rewrote every match while
the shared function refuses — a second behavior change, from silent rewrite-all to loud refusal
on a malformed file; (ii) `reclaim-claims.sh`'s sweep contract is per-item skip, so a refusal
from the shared function inside `reclaim_file` must surface as a **per-item skip, never a sweep
abort** — one malformed file must not halt every other reclaim. To make that skip atomic, the
migrated sites **batch all their field writes into a single `set-field` invocation** —
`reclaim_file`'s five sequential `set_field` calls become one call, and `archive-change.sh`'s
likewise — so validate-all-then-write-once means a refusal lands **before any byte moves** and a
skipped item is byte-untouched, never a torn half-transition sitting uncommitted in the shared
worktree; the commit is skipped on refusal. Both knock-ons carried by tests at the migrated
sites, with the refusal fixture's defect placed on a **later** key in the batch plus a
file-byte-untouched assertion, so a torn-write implementation reddens.

### Repointing the skills

The field-write rule in `docket-implement-next` (and the analogous field-write sites in
`docket-groom-next`, `docket-auto-groom`, `docket-finalize-change`, `docket-adr`,
`docket-new-change`, `docket-status`) is repointed: a frontmatter field write is performed by
`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh set-field --file <path> KEY=VALUE ...`,
never a hand-rolled edit. The rule's *policy* content (which fields, when, `claimed_at` re-stamp,
Artifacts regen in the same commit) is unchanged — only the *mechanics* sentence changes. The
AGENTS.md frontmatter bullets stay (they still govern hand edits in non-docket contexts and
body-prose hazards), gaining at most a pointer to the writer; note the overlap with change 0257's
finding 2, which edits the same bullets.

### Out of scope (unchanged from the stub)

- `## Artifacts` regeneration — `render-change-links.sh` stays the sole writer of that block;
  skills keep calling it after link-bearing writes.
- Archiving, status policy, and any new frontmatter field.
- Body-section writes (`## Auto-groom blocked`, `## Reconcile log`, …) — multi-line body appends
  are a different shape and a different change if ever scripted.

### Tests

- Unit tests for `set-field.sh`: multi-field write, unconditional scalar quoting (round-trip
  through `field()`), `--raw` flow-collection verbatim, clear-to-empty, append-if-absent-capable
  on a legacy change file (the `archive-change.sh` path), refusals (absent non-capable key,
  absent key under `--kind adr`, plain bracket-wrapped scalar, malformed key shape, malformed
  fence, control char, duplicate key arg, duplicate `key:` line in the fence, non-bracketed
  `--raw` value), and a body-byte-identical assertion (compare everything below the closing
  fence before/after). Migrated-site tests: legacy-file archive lands the append;
  reclaim-claims per-item-skips a refusing file (defect on a later key in the batch, file
  asserted byte-untouched) and continues the sweep without committing the skipped item.
- Mutation-test the guards per AGENTS.md: strip the fence validation, watch the refusal test
  redden; strip the quote call, watch the round-trip test redden; strip the absent-capable gate,
  watch the typo'd-key refusal redden.
- A prose guard extending the existing skill-text test family: the field-write rule sections
  reference `docket.sh set-field` (shape-keyed, not an enumerated spelling list).

## Assumptions

Every decision an interactive brainstorm would have raised, the committed default, and the
rejected alternatives.

1. **Multi-field per invocation** (the stub's first named fork). Chosen: multiple `KEY=VALUE`
   pairs, validate-all-then-write-once, atomic rename. Rejected: one field per call — simpler CLI
   but N partial-failure windows for writes that are semantically one transition (a claim writes
   `status` + `branch` + `claimed_at` together), and the learnings ledger's
   *validate-the-whole-input-set-first* finding argues directly for batch validation. Rejected: a
   stdin/JSON batch protocol — heavier than any caller needs.
2. **ADR files in scope** (the stub's second named fork). Chosen: both change files and ADR files,
   because the writer is key-agnostic and the mechanics (first fence block, scalar quoting) are
   identical; `docket-adr`'s `status:`-line supersede/reverse edit is exactly the 0140 defect
   class. Rejected: change-files-only — would leave the ADR `status:` write hand-rolled for no
   mechanical reason; the stub's own boundary sentence says "a change or ADR file".
3. **Presence rules: replace when present; append only under `--kind change` for keys in a new
   declared lib array (`DOCKET_CHANGE_ABSENT_CAPABLE_KEYS`, promoted from the lib's prose
   comment, learnings-only keys excluded); `--kind adr` refuses every absent key; any other
   absent key refuses.** (Revised twice after critic review: replace-existing-only contradicted
   the lib's absent-capable documentation and the `issue:` mint flow; then the roster's
   sourceable form and file-kind scoping were pinned — the prose comment is not sourceable as
   written, and a corpus-mixed roster on a kind-blind writer would launder `claimed_at:` onto an
   ADR. Rejected: blanket append-if-missing — a typo'd key silently mints a field. Rejected:
   pure refuse — forces the `issue:` persist and legacy-file writes back to hand-rolled edits.
   Rejected: inferring file kind from content — shape inference is the defect class; the caller
   declares `--kind`. Final round: the critic's prescribed fixes were applied verbatim —
   `reconciled` classified into the roster, roster content derived by grep not copied, and the
   prose repoint keeps the learnings-corpus keys.)
4. **Unconditional single-quoting for scalars; flow collections only via caller-declared
   `--raw`; a plain bracket-wrapped scalar refuses.** (Revised twice after critic review: the
   draft's fail-closed bare-allowlist rested on a falsified bare-grep migration cost and shipped
   the bare-boolean hazard against ADR-0071; then the bracket shape-sniff was dropped — the
   lib's `docket_scalar_quote_reason` commentary documents that sequence-vs-string intent is a
   call-site decision a shape test cannot infer. Rejected: the allowlist. Rejected: per-key
   quoting table — an enumeration by another name. Rejected: bracket inference — `[WIP] rework]`
   and `[a title: with colon]` are the counterexamples in the lib's own comments.)
5. **Consolidate the two script-internal `set_field()` duplicates now**, accepting one named,
   tested behavior change: missing-key writes at those sites go from silent no-op to append (for
   absent-capable keys), so `archive-change.sh` on a legacy file now lands the value instead of
   dropping it. Rejected: leave the duplicates — the change's thesis is "one write path".
   Rejected: preserving the silent no-op in the shared function — it is the defect class itself.
   `mint-stub.sh`'s creation-time template write is *not* migrated — it writes a whole new file,
   not a field edit.
6. **Facade verb named `set-field`** matching the script basename, per `docket.md`'s
   operation-name-equals-basename rule; no new env vars.
7. **`## Artifacts` regen stays with the skills** (per the stub's explicit boundary) rather than
   being auto-invoked by the writer on link-bearing keys. Rejected: auto-invoke — attractive
   (one less conjunct to forget) but it couples the writer to change-file schema knowledge
   (which keys are link-bearing) and to a second script's CLI, and the stub rules it out.
8. **Dependency state**: none. `depends_on` is empty; 0140 is done/archived and this change was
   scoped to stand with 0140 reverted. Couplings recorded as `related:` (forward link only):
   0256 (sibling consolidation of the *config*-reader families — no file collision, same
   ADR-0012 theme; whichever lands second reconciles prose) and 0257 (its finding 2 edits the
   same AGENTS.md frontmatter bullets this change touches — a small file collision the build-time
   reconcile pass should check).
9. **Key shape is gated** (`^[a-z][a-z0-9_]*$`) before the key ever meets the anchor pattern — a
   key is data, never a regex. (Added after critic review flagged the unstated interpolation.)
10. **Duplicate `key:` lines inside the fence ⇒ refuse** — replacing one of two contradictory
    lines is repair-by-guess; a malformed file needs a human. Two named knock-ons at the
    migrated sites (see the dedup section): the old sed rewrote all duplicates while the shared
    function refuses, and `reclaim-claims.sh` must surface that refusal as a per-item skip, not
    a sweep abort — made atomic by batching each site's writes into a single invocation so a
    refusal precedes any mutation. (Added, knock-ons named, then the batching requirement and
    torn-write test fixed per the critic's prescription.)
11. **Clearing emits `key:` with no trailing space**, deliberately normalizing the duplicates'
    `key: ` form; readers tolerate both. (Added after critic review corrected the "matches
    today's clearing" claim.)

## Open questions

None — resolved above.
