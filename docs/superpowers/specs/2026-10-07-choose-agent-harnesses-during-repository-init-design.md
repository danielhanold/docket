# Choose agent harnesses during init and with configure-harnesses: design

Change 0538 — Choose agent harnesses during init and with configure-harnesses

Change #538, groomed interactively on 2026-10-06. Discovered while hand-checking #535. Related: #351 (only a repository-level `agent_harnesses` authorizes repository writes), #531 (private mode), #533 (switching visibility).

## Summary

`docket repository init` writes the parent-facing dispatch instructions only when the repository declares `agent_harnesses`, but nothing ever asks for that setting, and nothing says when it is missing. This change adds one shared "choose harnesses" step with two entry points:

- `docket repository init` runs it on first setup: from `--harnesses`, from an interactive checkbox picker on a terminal, or, with neither, it succeeds and warns.
- A new `docket repository configure-harnesses` runs it any time afterwards, without init's clean-checkout and at-remote-tip precondition.

The step writes `agent_harnesses` to the repository-level config and refreshes the instructions in the same run, through the machinery `docket install` already uses. `docket repository check` gains a non-blocking warning while the key is absent.

## Evidence gathered at grooming

- **Init already writes the instructions when authorized.** `RunRepositoryInit` calls `installAuthorizedSurfaces` (shared mode only when `facts.SurfacesAuthorized`; private mode unconditionally, after `ensurePrivateConfig`). That function runs `ResolveRepoPhase` and `applyRepoPhaseSurfaces`, the same planning and proof-gated removals as `docket install`. The gap is only that the setting is never asked for and its absence is silent.
- **Authority rule (#351).** `ResolveRepoPhase`, `gatherRepoFacts` and `reposetup.authorizedHarnesses` each authorize only `Explicit && isRepositoryLayer(...)`: `.docket.yml`, `.docket.local.yml`, or the private `.git/dckt/config.yml`. A global value resolves but never authorizes. Absent means touch nothing; explicit `[]` retires every surface docket owns.
- **Shared init's authorization is decided too early.** `facts.SurfacesAuthorized` is computed in `GatherSetupFacts` before init writes anything, so a key written during init would not authorize that same run's surface install.
- **The clean-checkout check ignores docket-managed paths.** `primaryCleanPresence` skips `docketManagedWorktreePaths` (`.gitignore`, `.docket.yml`, `CLAUDE.md`, `AGENTS.md`, `.cursor/rules/docket-dispatch.mdc`), so an uncommitted `.docket.yml` edit does not block init.
- **`configure-tests` is the pattern to follow.** `RunRepositoryConfigureTests` and init share `ensureTestPolicyConfig`; writes go through `writeTargetConfig` → `repoConfigTarget` → `writeRepoConfig` (shared: `.docket.yml`, unstaged, a pending path; private: `.git/dckt/config.yml`, nothing pending). Its guard `configureTestsGuard` admits only `StateHealthy`; a docket-managed uncommitted edit classifies as `needs-review`.
- **No writer emits a YAML sequence.** `reposetup/testconfigedit.go` (`renderOwnerPairs`, `verifyOwnerPairs`) is hard-wired to the `build:`/`finalize:` blocks; `configedit.go` (`RemoveMetadataBranchKey`) removes a top-level key by line splice using `topLevelMapping`, `maxNodeLine`, `spliceOutLines`.
- **Silence everywhere.** `repository check`, `status`, `diagnostic config` and the preflight say nothing about a missing key; `docket install` only lists a `notAuthorizedAction` keep line.
- **Detection exists.** `harness.Adapter.Detect` stats `~/.claude`, `~/.codex`, `~/.cursor`, `$XDG_CONFIG_HOME/opencode`.
- **No prompt UI exists.** The only prompt is the hand-rolled y/N for `migrate`/`repair` (`repositoryConfirmInteractive` checks `os.ModeCharDevice` on stdin). `go.mod` has four direct dependencies and no terminal library.
- **Binary-size measurement** (current `main`, a reachable call to each option):

  | Build | Unchanged | + `golang.org/x/term` | + `charmbracelet/huh` |
  |---|---|---|---|
  | default `go build` | 14,751 KB | 14,769 KB (+17 KB) | 16,556 KB (+1,762 KB, +12%) |
  | stripped (`-s -w`) | 10,583 KB | 10,583 KB (+0.3 KB) | 11,950 KB (+1,335 KB, +13%) |
  | new modules in the graph | | 2 | 42 |

## Decisions

1. **A dedicated command for later changes.** `docket repository configure-harnesses` changes the selection on an already set-up repository. Re-running `init` to change harnesses was rejected because it inherits init's clean and at-tip precondition; leaving later changes as a hand edit plus `docket install` was rejected as the status quo the change exists to fix.
2. **One shared step, two entry points.** Init does not invoke the command; both call the same internal function, the way init and `configure-tests` share `ensureTestPolicyConfig`. Flag parsing, validation, the picker, the write, the local-override warning and the refresh are identical; only two caller-supplied behaviours differ (below).
3. **An arrow-key checkbox picker built on `charmbracelet/huh`.** Chosen over a numbered line prompt and over a hand-rolled `golang.org/x/term` picker, with the size cost above known. It is docket's first interactive UI dependency, and later prompts reuse it.
4. **`repository check` warns while the key is absent.** "No agents" is a valid setup, so the warning fires only while no repository-level layer declares the key; writing `[]` or any list silences it permanently. It never blocks.
5. **Choosing none is a recorded decision.** `--harnesses none`, or confirming the picker with nothing checked, writes `agent_harnesses: []`.

## Design

### The shared step

One app-layer function (working name `chooseHarnesses`) takes:

- the explicit flag value, if `--harnesses` was given;
- a chooser: a function the CLI supplies when interactive, `nil` otherwise;
- two caller policies:

  | | `init` | `configure-harnesses` |
  |---|---|---|
  | a repository-level value exists and no flag | skip; keep the value | show the picker, pre-checked with it |
  | no flag and no chooser | succeed and warn | refuse |

It then:

1. **Resolves the selection.**
   - The flag wins. `--harnesses claude,cursor` (a comma list; repeating the flag also works) or `--harnesses none`. Every token is validated against the `agent_harnesses` allowed set (`claude`, `codex`, `cursor`, `opencode`) and duplicates are refused, before anything is written. `none` cannot be combined with a name.
   - Otherwise, with a chooser, the picker runs, pre-checked with the first of: the repository-level value, the global `agent_harnesses`, the harnesses `harness.Adapter.Detect` finds. Options are listed in `harness.Order`. Confirming with nothing checked selects `[]`; cancelling (Ctrl-C) writes nothing and the command reports a cancellation.
   - Otherwise the caller policy applies. Init's warning names the exact line to add (`agent_harnesses: [claude]` in `.docket.yml`, or in `.git/dckt/config.yml` for a private repository) and the command `docket repository configure-harnesses`. The command's refusal says to pass `--harnesses <list>`.
2. **Writes the key** through `writeTargetConfig` / `repoConfigTarget`, unchanged: shared → `.docket.yml`, unstaged, reported as a pending path; private → `.git/dckt/config.yml`, nothing pending. Writing the value already present is a no-op.
3. **Refreshes the instructions** by calling `installAuthorizedSurfaces`, which re-resolves config from disk. Dropped harnesses lose their surfaces through the existing proof-gated removals; `[]` retires everything docket owns there. A surface conflict refuses exactly as it does for init and install today, mapped by `mapSurfaceFailure`.
4. **Warns on a local override.** In a shared repository whose `.docket.local.yml` also declares `agent_harnesses`, the write still goes to `.docket.yml`, and the result warns that `.docket.local.yml` overrides it and names the value that was actually applied. (Private mode already refuses a `.docket.local.yml` beside the private config.)

### `docket repository init`

- New flag `--harnesses`.
- The shared step runs after the managed `.gitignore` write and **before** the surface install. The shared-mode `if facts.SurfacesAuthorized` gate is replaced by authorization re-evaluated after the step, so a key written in this run authorizes this run's surfaces. Private mode keeps its unconditional call, now after the step.
- The picker is offered only when there is no flag, the CLI is interactive, and no repository-level layer declares the key.
- Init's guard (clean checkout, at the remote integration tip, layout states) is unchanged.

### `docket repository configure-harnesses` (new)

- Flags: `--harnesses`, plus the standard `--repo-dir` and `--json`.
- Guard: like `configureTestsGuard`, but it admits `needs-review` as well as `healthy`, so it can run right after `init` or twice in a row before the human commits. Fresh → "run `docket repository init`"; legacy → "run `docket repository migrate`"; any other state → "run `docket repository check`".
- No clean-checkout or at-tip precondition.
- Result: the standard `RepositoryOpResult`, with the written value, pending paths, surface actions and any warnings.

### The config writer

A new `reposetup` render function (working name `RenderAgentHarnessesEdit(existing []byte, harnesses []string)`):

- Emits one flow-sequence line, `agent_harnesses: [claude, cursor]` or `agent_harnesses: []`.
- If the top-level key exists, in block or flow form, it replaces that key's lines (located with `topLevelMapping`, `maxNodeLine`, `spliceOutLines`); otherwise it appends the line at the end of the file. Every other byte is preserved.
- It refuses multi-document files, non-mapping roots and undecodable YAML, as `topLevelMapping` does.
- It re-parses the result and refuses unless the only change is `agent_harnesses` holding exactly the requested list, mirroring `verifyOwnerPairs`.
- It is given nothing to write when the value already matches.

### Interactive UI

- The picker lives in `internal/cli` as a thin adapter over `huh.NewMultiSelect`. The app layer never imports huh.
- The CLI supplies the chooser only when stdin **and** stdout are terminals and `--json` is off; otherwise it passes `nil`. Detection follows the existing `repositoryConfirmInteractive` seam style, so tests can override it.

### `repository check` warning

- A new warning condition in `reposetup/health.go`, code `harnesses-unset`, raised when `SurfacesAuthorized` is false (no repository-level layer declares `agent_harnesses`). It works for shared and private layouts alike.
- Message: docket writes no dispatch instructions here because `agent_harnesses` is unset. Remedy: `docket repository configure-harnesses` (or `--harnesses none` to record that no agents are used).
- Severity warning; it does not change the classified state and never blocks prepare, implement or any other operation.

### Catalog and documentation

- `repository.configure-harnesses` joins the capability catalog with effect `local-write`, plus its request/result schema.
- Command help for `init` and `configure-harnesses`, the `agent_harnesses` section of `.docket.example.yml`, and the convention's configuration prose name the command and the picker. Documentation describes current behavior only.

## Acceptance criteria

1. Writer: key absent → appended; block-form and flow-form key → replaced in place; neighbouring comments and keys byte-identical; `[]` written; multi-document and non-mapping input refused; an unchanged value writes nothing.
2. `init --harnesses claude` on a fresh shared repo writes `agent_harnesses: [claude]` to `.docket.yml` and the Claude dispatch surface, both reported as pending paths, in one run.
3. `init --harnesses claude` on a fresh private repo writes `.git/dckt/config.yml` and the dispatch block in `.git/dckt/AGENTS.md`, with nothing pending.
4. Interactive `init` with no repository-level value calls the chooser with the documented pre-check order (repo, then global, then detected); a fake chooser's selection is what gets written; an empty selection writes `[]`; cancellation writes nothing.
5. Non-interactive `init` with no flag and no value succeeds and its output names the exact line and `docket repository configure-harnesses`.
6. `init` on a repo that already declares the key never calls the chooser and leaves the value unchanged.
7. `configure-harnesses --harnesses claude` on a repo with `[claude, cursor]` removes `.cursor/rules/docket-dispatch.mdc` through the proof-gated removal; `--harnesses none` retires every owned surface.
8. `configure-harnesses` succeeds with a dirty checkout, behind origin, and in `needs-review`; it refuses a fresh and a legacy repository with the documented remedies.
9. `configure-harnesses` with no flag and no terminal refuses and writes nothing.
10. An unknown or duplicate token, or `none` mixed with a name, is refused before any write, for both commands.
11. With `.docket.local.yml` declaring the key, the result warns and names the applied value.
12. `repository check` reports `harnesses-unset` when the key is absent, and does not for `[]` or a list, in both shared and private layouts; the classified state is unaffected.
13. Every new assert is mutation-checked: removing the behavior it guards turns it red.

## ADRs expected

One new ADR: docket adopts `charmbracelet/huh` as its interactive terminal UI dependency, chosen over a hand-rolled `golang.org/x/term` picker despite about 1.3–1.8 MB (around 12%) more binary and 42 modules in the graph; later interactive prompts reuse it rather than adding another library.

## Out of scope

- Making init refuse to run until harnesses are chosen.
- Copying the global `agent_harnesses` into the repository config without asking.
- Changing #351's rule that only a repository-level `agent_harnesses` authorizes repository writes.
- Relaxing init's clean-checkout and at-remote-tip precondition.
- Converting the existing y/N prompts or any other command to huh.
- A huh accessible-mode switch.
