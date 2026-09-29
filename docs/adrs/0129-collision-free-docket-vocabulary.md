---
id: 129
slug: 'collision-free-docket-vocabulary'
title: 'Collision-free docket vocabulary'
status: 'Accepted'
date: '2026-09-29'
supersedes: []
reverses: []
relates_to: []
change:
---

## Context

A review of `docs/reference/glossary.md` (2026-09-28) found docket terms that collide, either with each other or with unrelated concepts. The same word means different things depending on the page or the run log:

- **"gate"** names about ten things: the build, suite, test, task, finalize, policy and merge gates, the gate drive, the gate run, the gate key, the gate verdict, and the **run gate**. The run gate is not a checkpoint at all. It is bookkeeping for a launched run.
- **"merge gate"** names two different stops: the human merge gate at `implemented`, and an alias of the finalize gate.
- **"fence"** names two unrelated mechanisms: the config coordination-key fence and the run-cancel epoch fence. The glossary has to warn about this.
- **"terminal"** clashes with the shell terminal and names change-lifecycle end states.
- **`--version <blob>`** on a change clashes with the `docket version` command.
- **"arm"** (run gate) and **"re-arm"** (auto-groom) are unrelated mechanisms that share a verb.
- The strength of a worker is called a **profile**, a **rung** or a **tier** depending on the page.
- **"epoch"** suggests time, but it names a run and its id.

The glossary also still carries entries for retired features (runner delegation, the runner shim, `runtime.bash`, terminal publish).

Traced at grooming (2026-09-28, maintained source, `*_test.go` included):

| Term | Go occurrences | skills/agents | docs |
|---|---|---|---|
| `run-epoch` / `RunEpoch` | 594 in 66 files | 0 | 7 |
| `gate-verdict` | 289 in 26 files | 1 | 13 |
| `gate-before` | 154 in 21 files | 1 | 8 |
| `rung` | 132 in 40 files | 21 | 14 |
| `--version` (change revision) | 58; the flag is on 22 operations | 22 | 32 |
| `rearm` | 63 in 13 files | 8 | 8 |

Other facts found in the trace:

- The CLI has **no alias or deprecation mechanism** for flags, operation ids or tokens.
- `run.gate-before` and `run.gate-verdict` also appear in this repo's CLAUDE.md run-gate rules and in the generated dispatch material (`internal/harness/dispatch.go`, the embedded cursor rules), which `docket install` writes into every consumer repo.

A single PR for all of this would touch four subsystems (run gate, finalize, groom, review), rebase badly against concurrent loops, and be hard to review.

This decision is produced by change 0468 (spec: `docs/superpowers/specs/2026-09-29-rename-colliding-docket-terms-and-retire-obsolete-glossary-e-design.md`).

## Decision

This ADR records the vocabulary settled by change 0468. It is the single reference a family implementer, reviewer or reconcile pass checks a name against. Delivery: 0468 (umbrella) settles every name and does the prose-only renames; four family changes, each `depends_on: [468]`, hard-cut the wire tokens: (a) run tracker 0471, (b) revision 0472, (c) tiers 0473, (d) groom and lifecycle codes 0474.

### Naming rules

2. **Hard cut, no aliases.** When a family lands, the old spellings of its flags, operation ids, tokens and codes are refused. Docket's own consumers (skills, agent wrappers, generated dispatch material) ship with the binary through `docket install`, so they switch in one step. Consumer repos pick the new names up by re-running `docket install`. No alias or deprecation machinery is built.
3. **Persisted storage names stay unchanged.** On-disk directories, files and JSON keys in durable run-tracker, gate-drive and receipt state keep their current names (row 38, row 45). Renaming them would make every existing record unreadable at upgrade, including in-flight runs. No human or agent reads them.
4. **"epoch" splits by meaning.** Where the code means the identifier, it becomes **run id**. Where it means the stored file, it becomes **run record**. Where it means the run or its state, it becomes **run**. A uniform `epoch → run-id` substitution was rejected: it makes codes such as `epoch-io` and `epoch-corrupt` say the id itself is at fault.
5. **"gate" means a suite checkpoint only.** The build gate, finalize gate, suite gate, gate drive, gate run and the `docket gate …` CLI noun keep the word. The run gate becomes the **run tracker**, and its `gate-*` tokens become `run-*`.
6. **"final" names change-lifecycle end states.** That covers `done` / `killed`, and ADR statuses other than Accepted. **"terminal" stays** for the process-level meaning, "a run or process has finished" (`record-terminal`, `incumbent-nonterminal`, `terminal_receipt`, JSON `terminal`). That is the standard state-machine sense and does not collide.
7. **"fence" means only the run fence.** The config coordination-key fence becomes the **shared-setting guard**.
8. **`change.groom` outcome tokens stay in the requested-action form.** `rearm` becomes `re-enable`, matching the form of its siblings `revise` and `abstain`. A past tense would read as a result.
9. **Config keys, agent names and frontmatter fields are never renamed.** An existing `.docket.yml` with a renamed key would be silently ignored as unknown (0392 tolerates unknown keys as warnings), which is worse than a hard cut.
10. **No human-readable old→new mapping in the glossary.** The mapping lives in (i) this change's ADR, as the decision record, and (ii) a code-level **retired-vocabulary table** in `internal/repoguard`. That table maps each retired wire token to its replacement, drives the family absence seals, and names the replacement in every seal failure. The first family to land creates the table, and each later family appends its rows. The umbrella does not create an empty table, because a seal over an empty list cannot be mutation-tested.
11. **Retired features go to an "Obsolete terms" section of the glossary**, separate from renames: runner delegation, the runner shim / `runners` block, `runtime.bash`, terminal publish. The config-decode warnings for those keys stay.

### Rename table (rows 1-66)

Row ownership: rows 1-38 -> change 0471; rows 39-45 -> change 0472; rows 46-52 -> change 0473; rows 53-59 -> change 0474; rows 60-66 -> change 0468.

Kinds:
- **concept**: a word in docs, skills and agent text.
- **op / flag / token / key / code**: wire surfaces, hard-cut.
- **stage**: the `failure.stage` label and the `run epoch <stage>: <kind>` error-text prefix.
- **disk**: persisted state (Decision 3).

Go identifiers follow their row's term (e.g. `EpochRecord` → `RunRecord`, `reviewRung` → `reviewTier`). Each family derives them with a whole-repo grep, never from a hand list.

### Family (a) — run tracker (change 0471)

| # | Kind | Old | New |
|---|---|---|---|
| 1 | concept | run gate / gate facade | run tracker |
| 2 | concept | arm / arming | start / starting a run |
| 3 | concept | run epoch (the run) / epoch id | run / run id |
| 4 | concept | epoch fence | run fence |
| 5 | concept | gate key | run key |
| 6 | concept | dispatch context | run context |
| 7 | op + CLI | `run.gate-before` / `docket run gate-before` | `run.start` / `docket run start` |
| 8 | op + CLI | `run.gate-verdict` / `docket run gate-verdict` | `run.verdict` / `docket run verdict` |
| 9 | op + CLI | `run.gate-claim` / `docket run gate-claim` | `run.continue` / `docket run continue` |
| 10 | flag | `--run-epoch` (`agent.enter`, `gate drive start`, `gate drive prepare-scope`) | `--run-id` |
| 11 | flag | `run cancel --epoch` | `run cancel --run-id` |
| 12 | flag | `change claim --gate-context` | `--run-context` |
| 13 | token | `gate-armed` / `gate-unarmed` lines; JSON `armed` | `run-started` / `run-untracked`; `started` |
| 14 | token | verdict lines `gate-retry-once`, `gate-continue`, `gate-done`, `gate-stop`, `gate-observe` | `run-retry-once`, `run-continue`, `run-done`, `run-stop`, `run-observe` |
| 15 | token | `gate-claimed` (continue decision) | `run-continued` |
| 16 | code | `epoch-not-found` | `run-not-found` |
| 17 | code | `epoch-corrupt` | `run-record-corrupt` |
| 18 | code | `epoch-exists` | `run-exists` |
| 19 | code | `epoch-not-active` | `run-not-active` |
| 20 | code | `epoch-mismatch` | `run-id-mismatch` |
| 21 | code | `epoch-not-cancelled` | `run-not-cancelled` |
| 22 | code | `epoch-ambiguous` | `run-ambiguous` |
| 23 | code | `epoch-owner-ambiguous` | `run-owner-ambiguous` |
| 24 | code | `epoch-owner-unresolved` | `run-owner-unresolved` |
| 25 | code | `epoch-io` | `run-record-io` |
| 26 | code | `epoch-participant-unknown` | `run-participant-unknown` |
| 27 | code | `epoch-state-unknown` | `run-state-unknown` |
| 28 | code | `epoch-unreadable` | `run-record-unreadable` |
| 29 | code | `incumbent-epoch-fenced` | `incumbent-run-fenced` |
| 30 | code | `resume-epoch-unreadable` | `resume-run-record-unreadable` |
| 31 | code | `stale-run-epoch` | `stale-run-id` |
| 32 | code | `unknown-run-epoch` | `unknown-run-id` |
| 33 | code | `gate-unavailable` (verdict outcome and store error) | `run-tracker-unavailable` |
| 34 | code | `gate-context-invalid` / `gate-context-conflict` | `run-context-invalid` / `run-context-conflict` |
| 35 | stage | `mint-epoch`, `find-epoch`, `complete-epoch`, `supersede-epoch` | `mint-run`, `find-run`, `complete-run`, `supersede-run` |
| 36 | stage | `write-epoch`, `load-epoch`, `epoch-cas` | `write-run-record`, `load-run-record`, `run-record-cas` |
| 37 | stage | `bind-epoch-change`, `bind-epoch-worktree`, `epoch-launch-gate`, `reserve-worktree-execution-epoch`, `retire-worktree-execution-epoch`; error-text prefix `run epoch` | `bind-run-change`, `bind-run-worktree`, `run-launch-gate`, `reserve-worktree-execution-run`, `retire-worktree-execution-run`; prefix `run` |
| 38 | disk | dirs `rungate/`, `rungate-resume/`; files `epoch.json`, `epoch.lock`; keys `epoch_id`, `dispatch_epoch`, `run_epoch_id` | unchanged (Decision 3) |

`run.cancel`, `run.verify`, `--key`, and the verdict reason tokens that do not carry "gate" or "epoch" (e.g. `run-waiting`, `takeover-ambiguous`, `no-attributable-claim`) are unchanged.

### Family (b) — revision (change 0472)

| # | Kind | Old | New |
|---|---|---|---|
| 39 | concept | change version / entity version | revision / record revision |
| 40 | flag | `--version <blob>` on `change.{attach-plan, attach-results, block, claim, defer, groom, halt, kill, mark-implemented, reclaim, reconcile, refresh-claim, resume-halted, revive, unblock}`, `finalize.{block, clear-block, merge, rebase, retarget-children}`, `workspace.{inspect, prepare}` | `--revision` |
| 41 | key | request/response `version` on those operations and `learning.update`; `status` `changes[].version`; `context.implementation` / `context.finalize` | `revision` |
| 42 | key | `spec_version` (`change.groom`) | `spec_revision` |
| 43 | key | `target.version` (`adr.record`, `adr.supersede`, `adr.reverse`) | `target.revision` |
| 44 | key | `pr_version` (finalize merge, retarget) | `pr_revision` |
| 45 | disk | `resolver_budget_version` (rebase receipt) | unchanged (Decision 3) |

`protocol_version`, `schema_version`, `capability_version`, `format_version`, `go_version`, `product_version`, the `version` operation, and `docket version` are unchanged. They name software or format versions, not record revisions.

### Family (c) — tiers (change 0473; prose and Go identifiers, no wire tokens)

| # | Kind | Old | New |
|---|---|---|---|
| 46 | concept | build profile (economy / standard / premium / max) | build tier (the tier names are unchanged) |
| 47 | concept | review rung (lean / standard / deep) | review tier (the tier names are unchanged) |
| 48 | concept | dispatch tiers A / B / C and the carve-out | dispatch fallbacks, rows 49–52 |
| 49 | concept | Tier A: deterministic (the `docket-status` and `docket-adr` dispatches) | `inline`: run inline as a first-class equivalent path |
| 50 | concept | Tier B: adversarial (the `docket-auto-groom-critic` gate) | `abstain` |
| 51 | concept | Tier C: discipline (plan writer, build, review, in-branch fix workers) | `auto-or-halt`: inline only when the role is explicitly `auto`, otherwise abort-and-report |
| 52 | concept | carve-out (`docket-rebase-resolver`, `docket-integration-repair`) | `no-fallback`: abort-and-report, inline substitution forbidden |

Agent names (`docket-build-economy` … `docket-review-deep`) are unchanged.

### Family (d) — groom and lifecycle codes (change 0474)

| # | Kind | Old | New |
|---|---|---|---|
| 53 | concept | re-arm (auto-groom) | re-enable |
| 54 | token | `change.groom` `outcome: rearm` (`groom_outcomes` vocabulary) | `re-enable` |
| 55 | code | `nothing-to-rearm` | `nothing-to-re-enable` |
| 56 | code | `fenced-setting-ignored` | `shared-setting-ignored` |
| 57 | code | `terminal-backlink-pending`, `terminal-notes-frozen` | `final-backlink-pending`, `final-notes-frozen` |
| 58 | code | `change-terminal-claim-stamp`, `drop-terminal-claimed-at` | `change-final-claim-stamp`, `drop-final-claimed-at` |
| 59 | code | `not-terminal` (finalize cleanup), `skipped-terminal` (retarget), `adr-update-after-terminal` | `not-final`, `skipped-final`, `adr-update-after-final` |

### 0468 itself — prose only

| # | Kind | Old | New |
|---|---|---|---|
| 60 | concept | coordination-key fence | shared-setting guard |
| 61 | concept | human merge gate | PR handoff |
| 62 | concept | "merge gate", "rebase-retest gate" (aliases of the finalize gate) | dropped; **finalize gate** stays (it matches config key `finalize.gate`) |
| 63 | concept | "test gate" (alias of the suite gate) | dropped; **suite gate** stays |
| 64 | concept | terminal status / terminal record / terminal sweep / terminal close-out (change lifecycle) | final status / archived record / merged-PR sweep / close-out |
| 65 | concept | autonomous-eligible | folded into auto-groomable |
| 66 | glossary | runner delegation, runner shim / `runners` block, `runtime.bash`, terminal publish | moved to the glossary's "Obsolete terms" section (Decision 11) |

### Explicitly not renamed

- The `docket gate …` CLI noun, the gate drive, the gate run, `gatedrive-*`, `idempotent-suite-gate`.
- Config keys, including `finalize.gate`, `build.gate`, `terminal_publish`, `gate_observation_budget` (Decision 9).
- Process-level "terminal" (Decision 6).
- Agent names, the tier names inside each tier, frontmatter fields.
- `cmd/releasepkg --source-epoch`, which is a real Unix epoch (`SOURCE_DATE_EPOCH`).

### Deviations

A family change that has to deviate from a row records the deviation through the normal ADR supersede/update path (a new superseding ADR, or a dated `## Update` note where the decision still stands), never by silently diverging from this table.

## Consequences

- **Hard cut, no aliases.** When a family lands, the old spellings of its flags, operation ids, tokens and codes are refused outright; no alias, deprecation-window or dual-spelling machinery is built. Docket's own skills, agent wrappers and generated dispatch material ship with the binary and switch in one step.
- **Consumer repos must re-run `docket install` per family.** Their generated dispatch material names the old tokens until reinstalled; each family's results `**Human action:**` says so.
- **Family (a) (0471) must land with no dispatched run in flight** (drain or cancel first), and the post-merge binary rebuild must run immediately. Between merge and rebuild, CLAUDE.md names `run.start` while the installed binary only knows `run.gate-before`; catalog resolution stops the coordinator loudly (safe, but blocking until the rebuild).
- **Change 0469 must drop rows 5, 6 and 48-52** (gate key, dispatch context, dispatch tiers), which this ADR now owns; its remaining run-tracker items touch family (a)'s files, so its grooming should consider `depends_on: [471]`.
- Persisted storage names (rows 38, 45), config keys, agent names and frontmatter fields stay, so existing records and `.docket.yml` files remain readable.
- A code-level retired-vocabulary table in `internal/repoguard`, created by the first family to land and appended by later ones, drives the absence seals and names each replacement; the glossary carries no old->new mapping.
- Cost: four family PRs touching many files (e.g. `run-epoch` in 594 Go sites), golden output churn, and a one-time consumer reinstall per family.

## Alternatives considered

- **One PR for all renames.** Rejected: touches four subsystems (run tracker, finalize, groom, review), rebases badly against concurrent loops, and is hard to review.
- **Aliases / deprecation window.** Rejected: the CLI has no alias mechanism and docket's own consumers switch in one step via `docket install`.
- **Uniform `epoch -> run-id` substitution.** Rejected: makes codes like `epoch-io` / `epoch-corrupt` say the id itself is at fault; split by meaning instead (Decision 4).
- **Renaming persisted storage names or config keys.** Rejected: would make existing records unreadable at upgrade, and a renamed config key would be silently ignored as unknown.
- **Past-tense groom outcome (`rearmed`).** Rejected: reads as a result; `re-enable` matches the requested-action form of `revise`/`abstain`.
- **Human-readable old->new mapping in the glossary.** Rejected: the ADR plus the repoguard retired-vocabulary table carry it.
