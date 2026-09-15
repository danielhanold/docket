<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0419 — Make finalize repair attempts configurable with a default of six](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0419-make-finalize-repair-attempts-configurable-with-a-default-of.md)**
<!-- docket:backlink:end -->
# Make finalize repair attempts configurable with a default of six — Results

## Outcome

Finalize's integration-repair budget is now configurable instead of a hardcoded two-attempt limit.

- Added the `finalize.repair_max_attempts` configuration leaf: a positive integer with a built-in default of 6 and a minimum of 1, resolved through the standard global → repository → machine-local per-field precedence, following the existing `finalize.resolver_max_attempts` pattern (`internal/config`: `config.go`, `defaults.go`, `resolve.go`, `schema.go`).
- Raised the built-in default of `finalize.resolver_max_attempts` from 3 to 10. Explicit overrides, the positive-integer validation floor, and the durable reservation/exhaustion semantics are unchanged; existing owned rebase receipts keep their snapshotted budget.
- Exposed the resolved repair value through the typed prepared/finalize context (`internal/app/repository_prepare.go`) and through the effective-configuration diagnostics (`internal/app/config.go`), and threaded it into the integration-repair agent's dispatch payload so the repair workflow uses the configured cap. The finalize skill now passes "Repair-attempt budget: <finalize.repair_max_attempts> (initial attempt included)" to the agent.
- The repair cap is enforced by the dispatched integration-repair agent's contract (initial attempt counts as attempt 1, stop early on green, use the existing stuck/halted path on exhaustion), deliberately kept as an agent-honored budget rather than a durable Go reservation — unlike the resolver budget, which owns durable reservations. Repair attempts remain accounted separately from conflict-resolver dispatches.
- Updated the maintained finalize skill (`SKILL.md`, `references/gate-failure.md`), the integration-repair agent markdown, the Cursor dispatch rule, configuration docs (`.docket.example.yml`), and regenerated the embedded asset tree, manifest, and all four harness goldens consistently.
- Recorded the replacement of ADR-0010's fixed ≤2 repair cap as a dated `## Update` note on ADR-0010 (on the metadata branch), preserving its Accepted body verbatim and leaving its status and the ①/② rebase-completion split intact.

## Verification performed

- Build gate (`go run ./cmd/docket development test`, the configured whole-suite command) ran green via the native gate driver against the certified feature head.
- Added/updated automated coverage: config precedence, the positive-integer validation floor and type rejections (zero/negative → invalid-value; fraction/string/bool/list → invalid-type), decode acceptance, registry path-set/defaults, the resolver default bump (3→10) across diagnostics and the successive-conflict integration test, prepared-context propagation of the repair cap (built-in 6 plus non-default values), effective-config diagnostics on both the JSON and human halves, and the skill-budget ceilings for the finalize skill and gate-failure reference.
- Whole-branch review (docket-review-standard) confirmed the config/validation wiring, the resolver default bump with no stale "default 3" prose, the repair-cap propagation into the agent dispatch payload, byte-identical source-markdown ↔ embedded-goldens consistency (manifest sizes and sha256 recomputed), and that ADR-0010's Accepted text was not falsified. Its one important finding — the missing ADR replacement record — was resolved by the dated `## Update` note on ADR-0010 (commit on the metadata branch), which the reviewer could not see from the feature branch.
