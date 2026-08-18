<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0326 — Pre-Go mutation configuration contraction](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0326-pre-go-mutation-configuration-contraction.md)**
<!-- docket:backlink:end -->

# Pre-Go mutation configuration contraction

**Change:** 0326 · **Type:** chore · **Priority:** critical · **Date:** 2026-08-18 ·
**Status:** Approved design

## Purpose and boundary

This change moves only the configuration contraction required before Docket's first Go-managed
metadata mutation. It resolves the circular precondition discovered while launching change 0316:
Go correctly refuses the repository's active deferred capabilities, but the original program map
left their contraction in change 0318, after 0316 and release acceptance.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) remain governing constraints. This
change changes no supported/deferred classification and adds no bypass. It simply turns off the
requests Go v1 already classifies as unavailable. Change 0318 retains the broader self-hosting,
Bash-removal, documentation, release-publication, and hard-cutover work.

## Independently deliverable result

The migration repository and the migration host resolve inside Go v1's supported mutation envelope
before the installed Go binary is asked to claim change 0316.

The committed `.docket.yml` explicitly turns off these deferred switches:

- `build.checkpoint`;
- `finalize.skip_results_only_delta`; and
- `terminal_publish`.

The migration host's uncommitted `.docket.local.yml`:

- removes its repository-local `agents.*` mapping, because any repository-layer model/effort pin
  requests the per-repository routing Go v1 defers; and
- removes `auto_capture` or sets `auto_capture.enabled: false`.

Global `~/.config/docket/config.yml` model/effort pins are supported and remain untouched. They are
the intended Go-v1 machine-wide override layer. The earlier launch diagnosis that grouped global
pins with repository-layer blockers was overbroad; only repository and repository-local agent pins
block mutation.

## Delivery and one-time bridge

The tracked config edit is one ordinary reviewable PR. The machine-local edit cannot ride that PR,
so the implementation evidence records that an operator applied it on the migration host without
copying private configuration contents into the repository.

This change is implemented and finalized using Docket's immutable `v0.9.2` Bash workflow from a
separate clean checkout. That is the sanctioned transition runtime already named as the migration
baseline; it understands the still-active configuration and does not depend on the not-yet-installed
Go transaction CLI. This spec does not authorize a temporary Go build to claim, reconcile,
publish, or close this change.

After the PR lands and the local edit is applied, a Go binary built from reviewed source may run
the read-only check:

```text
docket diagnostic config --repo-dir <checkout> --for-mutation --json
```

The check must report that mutation is allowed and must identify no active unsupported capability
from any of the four layers. It is verification only. Change 0322 separately installs the reviewed
binary and harness assets before any Go metadata transaction occurs.

## Preservation and failure rules

- Set the three committed booleans explicitly to `false` so the migration decision is visible and
  cannot change if a future built-in default moves.
- Preserve unrelated `.docket.yml` keys and comments. Do not normalize or reorder the file.
- Preserve global agent pins. If a repository-local pin is still desired, move its value to the
  global file manually only when it is not already represented there; never copy machine-specific
  values into the committed repository.
- Keep `agent_harnesses`, runner settings, and inert compatibility data unless the diagnostic names
  them as an active blocker. This change is not a general configuration cleanup.
- A malformed layer, unknown provenance, or remaining active blocker fails the verification. It is
  not converted to a warning and does not authorize Go mutation.

## Testing and verification

- A focused config fixture reproduces the pre-change four-layer state and proves the same active
  blocker paths reported by the current Go classifier.
- The tracked diff changes only the three owned committed switches and their explanatory comments.
- A supported post-change fixture retains global agent pins, removes repository-layer pins, turns
  off automatic capture, and proves `MutationAllowed` is true.
- Negative fixtures leave each blocker active one at a time and prove mutation remains refused.
- The implementation records the exact reviewed Go commit used for the read-only diagnostic and a
  redacted result summary; it records no private config values.
- The whole resolved suite remains green. This change does not weaken or delete capability-fence
  tests.

## Explicit exclusions

This change does not:

- modify `internal/config`, add a migration override, ignore `.docket.local.yml`, or reclassify a
  deferred capability;
- install or build the permanent Go binary, adopt legacy harness artifacts, or change `install.sh`;
- implement any metadata, Git, GitHub, workspace, gate, evidence, finalize, recovery, or cleanup
  operation;
- remove global model/effort pins or require them to equal shipped defaults;
- remove production Bash, Bash tests, migration rollback material, or active documentation; or
- perform the full self-hosting rehearsal, release publication, or hard cutover assigned to 0318.

## Acceptance criteria

1. Committed `.docket.yml` explicitly disables `build.checkpoint`,
   `finalize.skip_results_only_delta`, and `terminal_publish` without unrelated normalization.
2. The migration host no longer requests repository-local agent routing or active automatic
   capture, while global model/effort overrides remain available.
3. Go's unmodified four-layer diagnostic reports mutation allowed from the reviewed post-change
   checkout, and one-at-a-time negative fixtures still fail closed.
4. The change is implemented and finalized with the frozen Bash baseline; no uninstalled or
   from-source Go transaction command writes shared metadata.
5. Installation/adoption remains 0322, finalize/recovery remains 0316, and remaining self-hosting
   and hard-cutover work remains 0318.
