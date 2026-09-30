<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0469 — Replace opaque docket terms with clearer names](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0469-replace-opaque-docket-terms-with-clearer-names.md)**
<!-- docket:backlink:end -->
# Replace opaque docket terms with clearer names — Results

**Human action:** Assessment pending — the build is complete and the whole-branch review has not run yet.

## Outcome

Docket's run logs, verdict lines, skills and docs used several terms a newcomer could not read without the glossary. This change renames them to plain names (ADR-0129 family (e), rows 67-86), as one hard cut with no aliases:

- Readiness `needs-brainstorm` is now `needs-grooming`, matching the project's own word for that step.
- Gate-drive halt causes: `identity-mismatch` is now `worktree-changed`, and `unresolved-execution` is now `launch-unconfirmed`.
- The CLI verb `docket change repair-identity` is now `docket change relink`, and finalize's "repair checkpoint" is now its "link check".
- Finalize merge conditions: `pr-identity-mismatch` is now `pr-link-mismatch`, and recertify's `identity-drift` is now `certified-input-changed`.
- "Conjunct" is now "condition" everywhere, including the Go type `MergeConditions` and the generated AGENTS.md dispatch block ("unmet conditions").
- Concept renames in prose and comments: worktree slot, moved to background, gate supervisor, gate run, marker section, allowed values, conflict-checked write, read on demand.
- "Step-0 preamble" is now the "startup check". The bootstrap verdict names and `--digest-only` are retired to the glossary's Obsolete terms.
- The retired-vocabulary seal now covers the family (e) wire tokens, so an old spelling that creeps back fails the suite.

Anyone running an old, already-loaded skill that passes a retired token or `change repair-identity` gets a loud unknown-command or unknown-value failure, never a silent fallback.

## Known issues and follow-ups

### One halt token now covers two takeover conditions

When a gate-drive takeover finds that its scope no longer matches, it halts with `worktree-changed`, the same token as a worktree that changed mid-run. This keeps the old behavior, where both used `identity-mismatch`, so nothing gets worse. It does mean a reader cannot tell the two cases apart from the token. Confirmed. Suggested next action: a human decides whether the scope-mismatch case deserves its own token, which would need an ADR-0129 `## Update`.

### "CAS" kept in Go comments

Around 150 Go comments still say "CAS". There it names the `Store.CAS` / `scopeCAS` / `admissionCAS` functions, which no rename row covers. The prose "CAS" in skills, docs and the glossary now reads "conflict-checked write". Confirmed as a deliberate choice; no action needed unless a later change renames those functions.

### Skill word budgets are nearly full

Several skills now sit within a few words of their size ceilings, for example `skills/docket-build/references/gate-caller-loop.md`, which is exactly at its limit. The next edit to these files will have to cut words somewhere else. Confirmed; no action needed now.
