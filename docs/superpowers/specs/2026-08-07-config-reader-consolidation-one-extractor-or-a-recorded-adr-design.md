<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0256 — Config-reader consolidation: one extractor or a recorded ADR](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-03-0256-config-reader-consolidation-one-extractor-or-a-recorded-adr.md)**
<!-- docket:backlink:end -->

# Config-reader consolidation: one extractor or a recorded ADR — design

**Change:** 0256 · **Date:** 2026-08-07 · **Type:** refactor

## Problem

Five hand-rolled `.docket.yml`/sidecar value readers exist across two YAML shapes, each
carrying its own copy of extraction rules that have already drifted wrong twice (0168,
0173). ADR-0062 makes shell readers the permanent implementation, so the duplication is
long-term, not a stopgap. The consolidated question (from killed #0179 + #0165): one
owner per reader family, or a recorded reason the copies stay separate.

Census (2026-08-07, against main):

| Reader | Shape | Class | Posture |
|---|---|---|---|
| `hd_field`/`hd_field_raw` — `scripts/lib/harness-defaults.sh:86-93,100-107` | flow map | `[^,}[:space:]]+` / `[^,}]*` | loud (hd_validate) |
| `field_of`/`field_of_raw` — `sync-agents.sh:407-411,420-427` | flow map | identical, by stated design ("the two readers deliberately match") | loud (validate_user_agent_values) |
| `scripts/runner-dispatch.sh:~111` value read | block mapping | rest-of-line, comment-stripped | deliberately tolerant (mid-handoff) |
| `yaml_get` — `migrate-to-docket.sh:73-78` | flat block scalars | `[^#]*` + dequote | tolerant (defaults fall through) |
| `config_*` family — `scripts/docket-config.sh:~100-160` | flat block scalars, 3 layers | `%%#*` + dequote, bash-native | loud |

## Ruling

**One extractor where readers must agree; a recorded ADR where divergence is
deliberate.** Concretely:

1. **Consolidate the flow-map pair.** `hd_field`/`field_of` (and their `_raw` twins)
   implement byte-identical extraction on the same shape and their own comments say they
   must never disagree — that is the strongest possible consolidation case, and the
   quote-leg one-sidedness (#0180's existence) proves comment-enforced correspondence
   fails. Factor two line-level helpers into `scripts/lib/harness-defaults.sh`:

   - `hd_line_field()` — `$1=line $2=field` → consumed value (`[^,}[:space:]]+` leg)
   - `hd_line_field_raw()` — `$1=line $2=field` → raw text (`[^,}]*`, trailing ws trimmed)

   `hd_field`/`hd_field_raw` become `_hd_entry_line` + delegate; `field_of`/
   `field_of_raw` delegate directly (sync-agents.sh already sources the lib at :59 —
   no new sourcing). The existing value-class rationale comments (ADR-0015 truncation
   story) move to the shared helpers; the wrappers keep one-line pointers.

   The lib's own header (`harness-defaults.sh:8-12`) currently records the opposite
   posture — "deliberately do NOT reuse sync-agents.sh's section_body/field_of …
   coupling the shipped-data reader to them would let a user-config change silently
   reshape program data." That concern is directional and stays satisfied: the shipped
   reader still consumes nothing from sync-agents; the delegation runs the other way
   (the user-config reader consumes the lib), and the loose-shape *lookup*
   (`section_body`/`harness_agent_line`) stays in sync-agents. Commit 1 MUST rewrite
   this header paragraph to say exactly that, and widen the lib's stated scope to
   "reader for agents/harness-defaults.yml + the shared flow-map value extractors" —
   left as-is it would name-check `field_of` as forbidden in the very file `field_of`
   now delegates into.

2. **The block-mapping and flat-scalar readers stay separate, recorded.** One new ADR
   (via docket-adr, at build time): "config value extraction — one owner per agreeing
   reader family; separateness is recorded per constraint, and a new reader either
   consumes an existing owner or adds its constraint to this record." Its Consequences
   name each surviving copy and its true constraint:
   - `runner-dispatch.sh`: different shape (block mapping), deliberately wider class,
     deliberately tolerant posture (dying mid-handoff converts a config typo into a
     failed dispatch — change 0173's recorded asymmetry).
   - `migrate-to-docket.sh` `yaml_get`: #0165's "standalone pre-install" premise is
     **dead** — that claim lived in the killed stub's body, not in the code (the actual
     comment at :71-72, "we intentionally avoid a YAML dependency", is true and stays),
     and the script already sources `scripts/lib/docket-gitignore-block.sh` at :47-48,
     so sourcing was never the obstacle. The true reason for separateness is contract
     divergence — a one-shot 4-key single-file read at migration time vs
     `docket-config.sh`'s layered committed/global/local snapshot reader, which is an
     entry-point script, not a sourceable lib. Consolidating would mean minting a new
     lib to dedupe ~6 lines while risking behavior drift in an install-time path. The
     ADR's Consequences record that the pre-install constraint was re-derived and
     found false.
   - `docket-config.sh` `config_*`: the layered reader; its own header already records
     that `runtime.bash` keeps a separate file reader.

3. **Cross-reference comments at every surviving copy.** Each stay-separate reader gets
   a 2-3 line comment naming the ADR and its own constraint. At `yaml_get` this is an
   **addition** beside the existing (still-true) no-YAML-dependency comment — there is
   no stale claim in the code to replace — so a fourth reader's author lands on the
   rule, not on an archaeology project.

4. **Behavior gates.** Extraction must be byte-identical after the refactor:
   - the existing pins stand as the gate — `tests/test_harness_defaults.sh` (19 call
     sites), `test_sync_agents.sh` (22), `test_harness_defaults_validator.sh`,
     `test_sync_agents_runners.sh` — suite green after each commit;
   - add one correspondence probe asserting `hd_line_field`/`hd_line_field_raw` output
     equals the pre-refactor expectations for the ADR-0065 table rows (quoted value,
     two-word value, provider-prefixed ID) — pinning the shared copy where the drift
     class lived.
   No new guard for the stay-separate readers: their classes are *deliberately
   different*, so a correspondence guard is wrong by construction; their behavior pins
   live in their existing suites.

## Out of scope (from the stub, confirmed)

- `docket-frontmatter.sh`'s `field`/`fm_field*` family — owned by #0244. Checked for
  collision: 0244's census guard greps `$(field ` / `$(field_raw ` fixed strings;
  `$(field_of ` and `$(field_of_raw ` match neither (no space after `field`, `_of`
  before `_raw`), so 0256 introduces nothing that trips it.
- Changing any reader's accepted-value semantics (ADR-0065 posture stands; the
  validators are #0255's territory).
- Any vendor model allowlist (ADR-0015).

## Assumptions

1. **Flow-map family: consolidate, not ADR-only.** Their own comments mandate byte
   agreement, they parse the same shape with the same classes, and the rule "every pair
   needs the quote leg" (ADR-0065) was applied one-sidedly exactly because there were
   two copies. Rejected: ADR-only (leaves the proven drift channel open); rejected:
   consolidating all five readers (the block-mapping and flat-scalar readers differ in
   shape, class, and posture *on purpose* — flattening them is the
   consolidation-flattens-caller-variance failure).
2. **Shared home = line-level helpers inside `harness-defaults.sh`, no new lib file.**
   sync-agents.sh already sources it (line 59), and the sharing extends within-pair
   precedent (`_hd_entry_line`: readers "can never disagree about WHICH line they are
   reading") to the value. The lib header's recorded non-reuse rationale (:8-12) is
   directional — shipped reader must not consume user-config machinery — and the
   delegation runs the other way; commit 1 rewrites that header (see Ruling 1) rather
   than leaving a contradiction. Rejected: a new `scripts/lib/flow-map-field.sh` (new
   file + new sourcing for exactly two consumers); rejected: sync-agents calling
   `hd_field` directly (signatures differ — `hd_field` does its own
   (file,harness,agent) lookup, `field_of` receives a line).
3. **`field_of`/`field_of_raw` keep their names as delegating wrappers** rather than
   being replaced at ~10+ sync-agents call sites. Minimizes diff and keeps the
   validator (`validate_user_agent_values`) untouched. Rejected: call-site rename
   (churn with no behavior payoff).
4. **Flat-scalar family stays separate, with the rationale corrected in writing.**
   The 0165 premise is stale in both directions: the copies are no longer identical
   AND the standalone constraint is no longer true. The honest artifact is the ADR +
   corrected comments, not a forced merge of two readers with different contracts.
   Rejected: extracting `config_line_scalar_get`/`config_normalize_scalar` into a lib
   both consume (new lib, behavior-change risk in migration and config paths, ~6 lines
   deduped); rejected: silence (the status quo that minted #0165).
5. **One ADR covers the whole ruling; no per-family ADRs.** The stub's premise is that
   one ruling covers both families; two ADRs would re-fragment it. The ADR relates_to
   ADR-0062 (shell readers permanent), ADR-0065 (pair rule), ADR-0015 (opaque IDs) and
   is recorded at build time via docket-adr (auto-groom never writes ADRs).
6. **Coupling with #0255 (same file, `hd_validate`'s quote leg).** Both changes edit
   `scripts/lib/harness-defaults.sh`; edits are disjoint (0255: validator legs +
   docs; 0256: extraction helpers) so either order merges with at most trivial
   conflicts. Recorded as `related`, not `depends_on` — neither needs the other's
   semantics. 0255 is proposed/ungroomed at this writing.
7. **Coupling with #0244 (frontmatter read shapes).** Boundary is clean by both stubs'
   out-of-scope clauses; 0244's spec puts its selection table in the frontmatter lib
   header, this ADR governs config readers — the ADR should cross-reference 0244's
   table as the sibling rule for the other family, nothing more. Recorded as `related`.
8. **Dependency state:** none blocking. #0179 and #0165 are killed (consolidated here);
   #0175 (the old blocker) merged 2026-08-01; #0180/#0181 folded into #0255.

## Build shape

Single branch, ~3 commits: (1) line-level helpers + delegation + the harness-defaults
header rewrite (Ruling 1), suite green, correspondence probe; (2) ADR via docket-adr +
cross-reference comment additions at the three surviving copies (runner-dispatch,
yaml_get, config_*); (3) docs touch-up if any other comment referenced the old
structure.
No behavior change anywhere: every existing test must pass unmodified, and the
correspondence probe pins the shared extraction on the ADR-0065 rows.
