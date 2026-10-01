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
| `--version` (change revision) | 58; a flag on 15 operations, a request key on 11 more, and three reads | 22 | 32 |
| `rearm` | 63 in 13 files | 8 | 8 |

Other facts found in the trace:

- The CLI has **no alias or deprecation mechanism** for flags, operation ids or tokens.
- `run.gate-before` and `run.gate-verdict` also appear in this repo's CLAUDE.md run-gate rules and in the generated dispatch material (`internal/harness/dispatch.go`, the embedded cursor rules), which `docket install` writes into every consumer repo.

A single PR for all of this would touch four subsystems (run gate, finalize, groom, review), rebase badly against concurrent loops, and be hard to review.

This decision is produced by change 0468 (spec: `docs/superpowers/specs/2026-09-29-rename-colliding-docket-terms-and-retire-obsolete-glossary-e-design.md`).

## Decision

This ADR records the vocabulary settled by change 0468. It is the single reference a family implementer, reviewer or reconcile pass checks a name against. Delivery: 0468 (umbrella) settles every name and does the prose-only renames; four family changes, each `depends_on: [468]`, hard-cut the wire tokens: (a) run tracker 0471, (b) revision 0472, (c) tiers 0473, (d) groom and lifecycle codes 0474. Change 0469 adds family (e), readability renames of terms that do not collide but are hard to read cold (rows 67-86), under the same naming rules.

### Naming rules

2. **Hard cut, no aliases.** When a family lands, the old spellings of its flags, operation ids, tokens and codes are refused. Docket's own consumers (skills, agent wrappers, generated dispatch material) ship with the binary through `docket install`, so they switch in one step. Consumer repos pick the new names up by re-running `docket install`. No alias or deprecation machinery is built.
3. **Local run-tracker storage is renamed by reset; committed state stays.** The run tracker's durable state under `.git/docket/` takes the new names (row 38). That covers the run records, the resume locks, and the gate-admission, gate-scope and gate-drive records that carry run ids or run contexts. It is not migrated. Each store whose persisted names change moves to a new root (a new directory name, or a `v1` → `v2` bump), so the new binary never reads the old state and starts empty. This is safe only when nothing is in flight at upgrade, which family (a) already requires. Old roots are left inert and may be deleted by hand. Committed state is not renamed, because git history on the metadata branch cannot be rewritten: the claim receipts' `gate_context_hash` key and the claim idempotency digest keep their spelling. Row 45 (`resolver_budget_version`, family (b)) stays unchanged.
4. **"epoch" splits by meaning.** Where the code means the identifier, it becomes **run id**. Where it means the stored file, it becomes **run record**. Where it means the run or its state, it becomes **run**. A uniform `epoch → run-id` substitution was rejected: it makes codes such as `epoch-io` and `epoch-corrupt` say the id itself is at fault.
5. **"gate" means a suite checkpoint only.** The build gate, finalize gate, suite gate, gate drive, gate run and the `docket gate …` CLI noun keep the word. The run gate becomes the **run tracker**, and its `gate-*` tokens become `run-*`.
6. **"final" names change-lifecycle end states.** That covers `done` / `killed`, and ADR statuses other than Accepted. **"terminal" stays** for the process-level meaning, "a run or process has finished" (`record-terminal`, `incumbent-nonterminal`, `terminal_receipt`, JSON `terminal`). That is the standard state-machine sense and does not collide.
7. **"fence" means only the run fence.** The config coordination-key fence becomes the **shared-setting guard**.
8. **`change.groom` outcome tokens stay in the requested-action form.** `rearm` becomes `re-enable`, matching the form of its siblings `revise` and `abstain`. A past tense would read as a result.
9. **Config keys, agent names and frontmatter fields are never renamed.** An existing `.docket.yml` with a renamed key would be silently ignored as unknown (0392 tolerates unknown keys as warnings), which is worse than a hard cut.
10. **No human-readable old→new mapping in the glossary.** The mapping lives in (i) this change's ADR, as the decision record, and (ii) a code-level **retired-vocabulary table** in `internal/repoguard`. That table maps each retired wire token to its replacement, drives the family absence seals, and names the replacement in every seal failure. The first family to land creates the table, and each later family appends its rows. The umbrella does not create an empty table, because a seal over an empty list cannot be mutation-tested.
11. **Retired features go to an "Obsolete terms" section of the glossary**, separate from renames: runner delegation, the runner shim / `runners` block, `runtime.bash`, terminal publish. The config-decode warnings for those keys stay.

### Rename table (rows 1-66, plus 28a, 38a-38h, 40a, 41a-41b, 43a, 44a, 46a, 47a and 59a-59c; family (e) rows 67-86)

Row ownership: rows 1-38, 28a and 38a-38d -> change 0471; rows 38e-38h -> change 0477 (38h records a rename 0471 already made); rows 39-45, 40a, 41a-41b, 43a and 44a -> change 0472; rows 46-52, 46a and 47a -> change 0473; rows 53-59 and 59a-59c -> change 0474; rows 60-66 -> change 0468; rows 67-86 -> change 0469.

Kinds:
- **concept**: a word in docs, skills and agent text.
- **op / flag / token / key / code / env**: wire surfaces, hard-cut (`env` is an environment variable passed between docket processes).
- **label**: a fixed line format one docket skill writes and another reads (no Go code parses it). Hard-cut like a wire surface, but not sealed by the retired-vocabulary table.
- **stage**: the `failure.stage` label and the `run epoch <stage>: <kind>` error-text prefix.
- **disk**: persisted state (Decision 3).
- **path**: a maintained file path that other maintained files reference by name. Hard-cut and sealed like a token, because a stale reference fails only when an agent follows it.

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
| 28a | code | `replacement-epoch-unreadable` (`run.cancel` finding token, form `replacement-epoch-unreadable:<key>`) | `replacement-run-record-unreadable` |
| 29 | code | `incumbent-epoch-fenced` | `incumbent-run-fenced` |
| 30 | code | `resume-epoch-unreadable` | `resume-run-record-unreadable` |
| 31 | code | `stale-run-epoch` | `stale-run-id` |
| 32 | code | `unknown-run-epoch` | `unknown-run-id` |
| 33 | code | `gate-unavailable` (verdict outcome and store error) | `run-tracker-unavailable` |
| 34 | code | `gate-context-invalid` / `gate-context-conflict` | `run-context-invalid` / `run-context-conflict` |
| 35 | stage | `mint-epoch`, `find-epoch`, `complete-epoch`, `supersede-epoch` | `mint-run`, `find-run`, `complete-run`, `supersede-run` |
| 36 | stage | `write-epoch`, `load-epoch`, `epoch-cas` | `write-run-record`, `load-run-record`, `run-record-cas` |
| 37 | stage | `bind-epoch-change`, `bind-epoch-worktree`, `epoch-launch-gate`, `reserve-worktree-execution-epoch`, `retire-worktree-execution-epoch`; error-text prefix `run epoch` | `bind-run-change`, `bind-run-worktree`, `run-launch-gate`, `reserve-worktree-execution-run`, `retire-worktree-execution-run`; prefix `run` |
| 38 | disk | under `.git/docket/`: `rungate/<key>/` (files `epoch.json`, `epoch.lock`; keys `epoch_id`, `gate_key`, `dispatch_epoch`); `rungate-resume/<id>/epoch.lock`; `gate-admission/v1` (implicit key `RunEpochID`); `gate-scopes/v1` (keys `run_epoch_id`, `gate_context_hash`); `gate-drives/v1` (key `gate_context_hash`) | `run-tracker/<key>/` (files `run.json`, `run.lock`; keys `run_id`, `run_key`, `dispatched_at`); `run-tracker-resume/<id>/run.lock`; `gate-admission/v2` (explicit JSON tags on every field; key `run_id`); `gate-scopes/v2` (keys `run_id`, `run_context_hash`); `gate-drives/v2` (key `run_context_hash`). Reset, not migrated (Decision 3). `dispatch_epoch` is a Unix timestamp, so it becomes `dispatched_at` beside `created_at` |
| 38a | flag | `agent enter --run-gate-key` | `--run-key` |
| 38b | key | `run.gate-before` (row 7: `run.start`) result JSON `epoch` | `run_id` |
| 38c | key | `change.claim` request `gate_context` | `run_context` |
| 38d | env | `DOCKET_AGENT_GUARDIAN_EPOCH` | `DOCKET_AGENT_GUARDIAN_RUN_ID` |
| 38e | flag | gate drive `--gate-context` (`gate drive start`, `gate drive prepare-scope`) | `--run-context` |
| 38f | env | `DOCKET_AGENT_GUARDIAN_GATE_KEY` | `DOCKET_AGENT_GUARDIAN_RUN_KEY` |
| 38g | key | `run.start` (row 7) result JSON `dispatch_context` | `run_context` |
| 38h | stage | error-text prefix `rungate store` | `run-tracker store` |

`run.cancel`, `run.verify`, `--key`, and the verdict reason tokens that do not carry "gate" or "epoch" (e.g. `run-waiting`, `takeover-ambiguous`, `no-attributable-claim`) are unchanged.

### Family (b) — revision (change 0472)

| # | Kind | Old | New |
|---|---|---|---|
| 39 | concept | change version / entity version | revision / record revision |
| 40 | flag | `--version <blob>` on `change.{attach-plan, attach-results, claim, halt, mark-implemented, reclaim, refresh-claim, resume-halted}`, `finalize.{block, clear-block, merge, rebase, retarget-children}`, `workspace.prepare` | `--revision` |
| 40a | flag | `change repair-identity --expect-version` | `--expect-revision` |
| 41 | key | request `version` on `change.{block, defer, groom, kill, reconcile, revive, unblock}` and `learning.update`, and on the requests the row-40 flags build; `status` `changes[].version`; `context.implementation` `change.version` / `spec.version`; `context.finalize` `candidates[].version` | `revision` |
| 41a | key | `status --records` `records[].version` | `revision` |
| 41b | key | `context.finalize` `candidates[].pr.version` (the PR snapshot hash) | `pr.revision` |
| 42 | key | `spec_version` (`change.groom`) | `spec_revision` |
| 43 | key | `target.version` (`adr.supersede`, `adr.reverse`) | `target.revision` |
| 43a | key | `change.version` (`adr.record`; `successor.change.version` on `adr.supersede`, `adr.reverse`) | `change.revision` |
| 44 | key | `pr_version` (`finalize.merge` result `merge.pr_version`; `finalize.retarget-children` authorized children) | `pr_revision` |
| 44a | code | `version-mismatch`, `version-drift`, `spec-version-mismatch`, `reclaim-version-missing`, `empty-version`, `empty-change-version`, `empty-target-version`, `empty-spec_version`, `invalid-spec_version`, `empty-child_pr_version` | `revision-mismatch`, `revision-drift`, `spec-revision-mismatch`, `reclaim-revision-missing`, `empty-revision`, `empty-change-revision`, `empty-target-revision`, `empty-spec_revision`, `invalid-spec_revision`, `empty-child_pr_revision` |
| 45 | disk | `resolver_budget_version` (rebase receipt) | unchanged (Decision 3) |

`protocol_version`, `schema_version`, `capability_version`, `format_version`, `go_version`, `product_version`, the `version` operation, `docket version` and the `capabilities` result's `binary.version` are unchanged. They name software or format versions, not record revisions.

"Revision" names the exact id of a pinned state: a record revision is a git blob id, a PR revision is a hash over the PR's mutable snapshot, and the existing `*_revision` keys (`committed_revision`, `metadata_revision`, `*_branch_revision`, the run tracker's `revision` / `bound_revision`) are commit ids. They share the word in that one sense and are not renamed.

### Family (c) — tiers (change 0473; prose, skill labels and test strings: no wire tokens, no Go identifiers)

| # | Kind | Old | New |
|---|---|---|---|
| 46 | concept | build profile (economy / standard / premium / max) | build tier (the tier names are unchanged) |
| 46a | label | plan-task override line `**Build profile:** <tier>` (plan writer → docket-build); worker return line `PROFILE: <tier> — <reason>` (docket-build-task → docket-build); dispatch prompt labels `Profile: <tier>` and `Rung: <tier>` | `**Build tier:** <tier>`; `TIER: <tier> — <reason>`; `Tier: <tier>` |
| 47 | concept | review rung (lean / standard / deep) | review tier (the tier names are unchanged) |
| 47a | concept | finding severity called "tiers": "severity-tiered findings", "the tiers a reviewer assigns", "tiered by severity" | severity levels: "severity-ranked findings", "the severity levels a reviewer assigns", "ranked by severity" |
| 48 | concept | dispatch tiers A / B / C and the carve-out | dispatch fallbacks, rows 49–52 |
| 49 | concept | Tier A: deterministic (the `docket-status` and `docket-adr` dispatches) | `inline`: run inline as a first-class equivalent path |
| 50 | concept | Tier B: adversarial (the `docket-auto-groom-critic` gate) | `abstain` |
| 51 | concept | Tier C: discipline (plan writer, build, review, in-branch fix workers), and its posture name "authorized-or-halt" | `auto-or-halt`: inline only when the role is explicitly `auto`, otherwise abort-and-report |
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
| 59 | code | `not-terminal` (finalize cleanup), `skipped-terminal` (retarget), `adr-update-after-terminal` | `not-final`, `skipped-final` (also emitted for stacked-merged children, which are non-final: the child no longer needs its PR retargeted), `adr-update-after-final` |
| 59a | path | `skills/docket-convention/references/terminal-close-out.md` | `skills/docket-convention/references/close-out.md` |
| 59b | concept | change- or ADR-lifecycle "terminal" beyond row 64's four phrases: Go identifiers (`Status.Terminal()`, `boardTerminalStatuses`, `isTerminalADRStatus`, …), messages ("change 0412 is terminal"), commit subjects ("terminal backlinks …"), skill text and comments | "final" ("terminal record" -> "archived record", per row 64); "terminal half" -> **closing half**, because that half leads to an end state and is not one |
| 59c | concept | "fence" for internal checks other than the run fence: the board-surface check (`fenceBoardSurface`), the learnings check (`fenceLearningsEnabled`), `haltPinAndFence`, the deferred-capability, owned-section and owned-ref checks, and the config guard's Go names (`applyFence`, `scopeRepoFenced`, `CodeFencedIgnored`, …) | check / refuse wording (`resolveBoardSurface`, `requireLearningsEnabled`, `haltPreflight`); the config guard's names follow row 56 (`applySharedSettingGuard`, `scopeRepoOnly`, `CodeSharedSettingIgnored`) |

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

### Family (e) — readability renames (change 0469)

These terms do not collide; they are hard to read without the glossary entry. The wire rows (67-73) are hard-cut and sealed through the retired-vocabulary table (Decisions 2 and 10). Rows 74-84 are prose only. Rows 85-86 retire names that no longer match any code (Decision 11). After this family, "identity" names only *which* record, scope or process something is, never a thing that changed.

| # | Kind | Old | New |
|---|---|---|---|
| 67 | token | readiness `needs-brainstorm` (`ReadyNeedsBrainstorm`; the board cell and the `status` readiness value): proposed, no spec, not trivial | `needs-grooming` |
| 68 | code | gate-drive halt cause `identity-mismatch`: the worktree fingerprint (HEAD, index, status, live file bytes) moved since the drive started | `worktree-changed` |
| 69 | code | `unresolved-execution` (gate-drive halt cause and ownership error kind): nothing proved whether a launch happened (a lost launch response, a crash between reserving the worktree and attaching the process) | `launch-unconfirmed` |
| 70 | op + CLI | `change.repair-identity` / `docket change repair-identity`: re-points a record's `branch:` at the PR's head, or its `pr:` at a PR the human supplies | `change.relink` / `docket change relink` |
| 71 | token | row 70's result tokens `repaired-branch` / `repaired-pr` | `relinked-branch` / `relinked-pr` |
| 72 | code | finalize merge condition `pr-identity-mismatch`: the record's PR does not match the real PR (row 70 fixes it) | `pr-link-mismatch` |
| 73 | code | `evidence.recertify` reason `identity-drift`: the PR head or the build command moved after the gate passed | `certified-input-changed` |
| 74 | concept | unmet conjuncts / conjunct / conjunction (CLAUDE.md and AGENTS.md dispatch rules, generated dispatch material, comments; Go `MergeConjuncts`) | unmet conditions / condition (`MergeConditions`) |
| 75 | concept | admission slot / worktree admission slot | worktree slot |
| 76 | concept | liveness transition (a harness moving a still-running command into the background) | moved to background (liveness probe unchanged) |
| 77 | concept | native supervisor | gate supervisor ("run supervisor" rejected: "run" names a tracked implement-next run, rows 1-3) |
| 78 | concept | gate execution | gate run (the same concept) |
| 79 | concept | presence-encoded section / marker | marker section |
| 80 | concept | Step-0 preamble | startup check |
| 81 | concept | closed vocabulary | allowed values |
| 82 | concept | compare-and-swap (prose) | conflict-checked write (the `contended` token is unchanged) |
| 83 | concept | pay per relevance | read on demand |
| 84 | concept | identity repair; finalize's identity checkpoint | relink; finalize's link check |
| 85 | glossary | bootstrap verdicts `BOOTSTRAP=` / `PROCEED` / `STOP_MIGRATE` / `CREATE_ORPHAN` (Bash-era; no Go sites) | prose describes `repository.prepare`'s dispositions; the names move to "Obsolete terms" |
| 86 | glossary | `docket status --digest-only` / digest-only read (the flag does not exist) | `docket status`; the name moves to "Obsolete terms" |

Kept in family (e): the worktree slot state `unresolved` and the stage `mark-worktree-execution-unresolved`; the `--adopt-pr-head`, `--adopt-pr` and `--expect-*` flags and the other repair result tokens (`stale-evidence`, `workspace-conflict`, `candidate-branch-absent`, `pr-unknown`, `invalid-request`); "identity" in its which-record sense (`scope-identity-mismatch`, `results-identity-broken`, `identity-reused`, `identity-mutated`, the `status` JSON `identity` key, the process-lock `identity` stage); the `skills.brainstorm` key and the `docket-brainstorm` / `docket-brainstorm-consultant` names (Decision 9).

Considered for family (e) and not renamed:

- **abstain** ("hand back"): Decision 8 and row 50 chose it.
- **dummy mode / persona**, **metadata branch**, **reconcile**: they are the config keys `dummy_mode` and `metadata_branch` and the frontmatter field `reconciled:` (Decision 9); renaming only the prose would split it from the key.
- **inert**: the config classification `inert` and code `inert-setting`, and plain English.
- **disposition** ("outcome"): the protocol-v1 envelope key of every operation (about 2,500 Go sites and every skill).
- **owner generation / `--owner-gen`**: an opaque fencing token, not a counter; "generation" is the standard term.
- **continuation id**: settled by row 9 (`run.continue <key> <continuation-id>`).
- **sync integration** ("fast-forward main"): the operation id `repository.sync-integration`, and the integration branch is not always `main`.
- **coordination key**: already row 60. **scope tag**: clear and consistent in `docs/reference/config-keys.md`.

### Explicitly not renamed

- The `docket gate …` CLI noun, the gate drive, the gate run, `gatedrive-*`, `idempotent-suite-gate`.
- Config keys, including `finalize.gate`, `build.gate`, `terminal_publish`, `gate_observation_budget` (Decision 9).
- Process-level "terminal" (Decision 6): gate-run, supervisor, participant and drive terminals, `terminal_receipt`, the JSON `terminal`, `terminal_status`, `terminal_turn` and `terminal_observed_at` keys, a run's "Terminal disposition", a command's terminal result or envelope. Also a GitHub pull request's "terminal state" (closed or merged).
- The obsolete "terminal publish" / "terminal publication" feature name, which matches the kept config key `terminal_publish`.
- Markdown code fences and `---` frontmatter fences, which are standard Markdown terms, not docket mechanisms.
- The frozen fixture directory `testdata/repositories/v0.9.2/fenced-machine-keys/` and ADR-0019's filename.
- The `internal/suiterunner` signal-handler "re-arm", and "re-enabling" in its generic sense (learnings, recursive self-dispatch).
- Agent names, the tier names inside each tier, frontmatter fields.
- "tier" in its generic senses (model and effort cost tiers, the harness validation Tier 1/2/3, the learnings "tiering criterion"), the "ladder" metaphor for the ordered tiers, and "profile" in its unrelated senses (verification profile, remote-call profile, budget profile, codex's `--profile`).
- `cmd/releasepkg --source-epoch`, which is a real Unix epoch (`SOURCE_DATE_EPOCH`).
- `gate-failed` (the suite gate failed) and the `gate-scope` run participant kind (a gate-drive scope): both use "gate" in the checkpoint sense.
- The committed claim-receipt key `gate_context_hash` and the claim idempotency digest payload (Decision 3), including the payload's `version` key.
- The `gatelifecycle` integration shard, which tests gate launch/stop (the gate-run sense).
- The existing commit-id `*_revision` keys (see the note under family (b)).
- The release tools' `--version` flags (`cmd/releasepkg` and the release downloader), which name a software release.

### Deviations

A family change that has to deviate from a row records the deviation through the normal ADR supersede/update path (a new superseding ADR, or a dated `## Update` note where the decision still stands), never by silently diverging from this table.

## Consequences

- **Hard cut, no aliases.** When a family lands, the old spellings of its flags, operation ids, tokens and codes are refused outright; no alias, deprecation-window or dual-spelling machinery is built. Docket's own skills, agent wrappers and generated dispatch material ship with the binary and switch in one step.
- **Consumer repos must re-run `docket install` per family.** Their generated dispatch material names the old tokens until reinstalled; each family's results `**Human action:**` says so.
- **Family (a) (0471) must land with no dispatched run in flight** (drain or cancel first), and the post-merge binary rebuild must run immediately. Between merge and rebuild, CLAUDE.md names `run.start` while the installed binary only knows `run.gate-before`; catalog resolution stops the coordinator loudly (safe, but blocking until the rebuild). Every machine must also switch to the new binary with no implement-next or finalize run in flight in any docket repo, because the new binary starts the run tracker's local stores empty (Decision 3).
- **Family (c) (0473) must land with no change mid-build or holding a committed-but-unbuilt plan** (drain first). A plan written before the upgrade may carry the retired `**Build profile:**` override (row 46a), which the new docket-build does not read, so a risky task could silently route to a cheaper tier.
- **Change 0469 must drop rows 5, 6 and 48-52** (gate key, dispatch context, dispatch tiers), which this ADR now owns; its remaining run-tracker items touch family (a)'s files, so its grooming should consider `depends_on: [471]`.
- **Family (e) (0469)** needs a consumer `docket install` rerun like the other families, and must land with no gate drive in flight, because rows 68 and 69 rename halt causes that drive records carry. The committed `BOARD.md` shows `needs-brainstorm` cells until its next re-render; 0469's own close-out commit re-renders it.
- Config keys, agent names, frontmatter fields, committed claim-receipt keys and row 45 stay, so `.docket.yml` files and metadata history remain readable. The run tracker's local stores restart empty at upgrade under their new names (row 38), and their old roots are left inert.
- A code-level retired-vocabulary table in `internal/repoguard`, created by the first family to land and appended by later ones, drives the absence seals and names each replacement; the glossary carries no old->new mapping.
- Cost: four family PRs touching many files (e.g. `run-epoch` in 594 Go sites), golden output churn, and a one-time consumer reinstall per family.

## Alternatives considered

- **One PR for all renames.** Rejected: touches four subsystems (run tracker, finalize, groom, review), rebases badly against concurrent loops, and is hard to review.
- **Aliases / deprecation window.** Rejected: the CLI has no alias mechanism and docket's own consumers switch in one step via `docket install`.
- **Uniform `epoch -> run-id` substitution.** Rejected: makes codes like `epoch-io` / `epoch-corrupt` say the id itself is at fault; split by meaning instead (Decision 4).
- **Keeping the run tracker's storage names (this ADR's original Decision 3).** Rejected at change 0471's grooming: docket has a single user, nothing is in flight at upgrade, and a reset to new roots costs no migration code, so the storage layer need not keep the retired vocabulary.
- **Migrating run-tracker storage in place.** Rejected: a migration must lock out live writers and survive a crash mid-move to preserve state that nothing reads; with nothing in flight, a reset is enough.
- **Renaming config keys.** Rejected: a renamed config key would be silently ignored as unknown.
- **Past-tense groom outcome (`rearmed`).** Rejected: reads as a result; `re-enable` matches the requested-action form of `revise`/`abstain`.
- **Human-readable old->new mapping in the glossary.** Rejected: the ADR plus the repoguard retired-vocabulary table carry it.

## Amendment — 2026-09-29 (change 0471 grooming)

Edited in place with the human's explicit authorization, before any family change was built. Decision 3 changed from "persisted storage names stay unchanged" to a reset of the run tracker's local storage. Row 38 was replaced and rows 38a-38d were added. "Explicitly not renamed", Consequences and Alternatives were updated to match.

## Update — 2026-09-29 (change 0471 build)

The decision stands. Building family (a) found one name the table missed: the `run.cancel` finding token `replacement-epoch-unreadable:<key>`. Change 0471 renamed it to `replacement-run-record-unreadable:<key>`, following row 28's pattern, and the table now records it as row 28a. Added with the human's explicit authorization, after the change was built and its PR opened.

## Update — 2026-09-29 (change 0477 grooming)

The decision stands. Change 0471 kept three run-tracker spellings because no row named them: the gate drive's own `--gate-context` flag, the guardian environment variable `DOCKET_AGENT_GUARDIAN_GATE_KEY`, and the `run.start` result key `dispatch_context`. It also renamed the store error prefix `rungate store` to `run-tracker store` without a row. Change 0477 renames the three kept spellings, following rows 12, 5 and 6, and the table now records all four as rows 38e-38h. Row 38h only records the rename 0471 already made. With row 38e, every `--gate-context` is retired, including the gate drive's, so the gate drive and `change claim` take the same run-context token under one flag name. The committed claim-receipt key `gate_context_hash` still stays (Decision 3). Added with the human's explicit authorization at change 0477's grooming, before it was built.

## Amendment — 2026-09-29 (change 0472 grooming)

Edited in place with the human's explicit authorization, before family (b) was built. Tracing the code showed rows 40-44 were incomplete: row 40 listed operations that take the revision in a request file (or not at all, `workspace.inspect`) as flag operations, and row 43 listed `adr.record`, which carries no target. Rows 40, 41, 43 and 44 were corrected, rows 40a, 41a, 41b, 43a and 44a were added, the note defining "revision" was added under family (b), and "Explicitly not renamed" gained the claim digest's `version` key, the commit-id `*_revision` keys and the release tools' `--version`.

## Update — 2026-10-01 (change 0481)

The decision stands. Rows 68 (`worktree-changed`) and 69 (`launch-unconfirmed`) now cover only the conditions they describe. A gate-drive takeover whose recorded scope identity (repo, branch, worktree, change, task, phase) no longer matches its scope halts with the retained family-(e) token `scope-identity-mismatch` (`ErrScopeIdentityMismatch`), not `worktree-changed`. When `resolveDriveRun` loses the drive's worktree-slot link to its run (the slot is unreadable or absent, or was reassigned to another reservation token), the drive halts with the new token `run-link-lost` (`CauseRunLinkLost`), not `launch-unconfirmed`. This is a token-only change: halt and recovery behavior is unchanged.
