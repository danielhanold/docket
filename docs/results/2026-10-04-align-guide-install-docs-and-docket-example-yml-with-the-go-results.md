<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0464 — Align the human-facing docs and example config with the docket binary](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0464-align-guide-install-docs-and-docket-example-yml-with-the-go.md)**
<!-- docket:backlink:end -->
# Align the human-facing docs and example config with the docket binary — Results

**Human action:** No action is required to merge. One optional check is listed: reading a few rewritten pages to confirm they match how you use docket.

## Outcome

docket's human-facing documentation described how docket worked before the Go binary: copying `.docket.example.yml` as `.docket.yml` blocked every repository write, and many pages taught removed settings, commands, and checks. This change brings the living docs (`README.md`, `docs/README.md`, `docs/guide/`, `docs/install/`, `docs/concepts/`, `docs/reference/`, and the release acceptance checklist) in line with what the binary does today.

- **Example config is safe to copy.** `.docket.example.yml` lists only supported keys with their defaults; agent model pins are commented and marked global-config-only. A verbatim copy passes `docket diagnostic config --for-mutation` and resolves to the built-in defaults. Its test now requires every supported key, forbids every unsupported key (derived from the schema, including inside commented blocks), and keeps the commented agents table equal to the built-in table.
- **Pages deleted:** `docs/install/workflow-roles.md` (its default-roles table moved to `docs/reference/skills-and-agents.md`), `docs/install/delegating-across-harnesses.md`, the Codex validation runbook and its nested-launch fixtures, and the two Cursor example JSON files.
- **Renamed:** "the daily loop" page is now `docs/guide/five-steps.md`, "Using docket: the five steps"; "loop" is kept for `/loop` only, and "fix loop" is "fix pass".
- **New guard:** `internal/repoguard/docs_alignment_test.go` fails when a living doc cites a change or PR, or spells a config key the schema marks unsupported. It scans 42 files with a population floor.
- **Other cleanup:** the dead `scripts/runners/` files are deleted; `agents/harness-defaults.yml` comments were rewritten (values unchanged) with a new frozen fixture `testdata/repositories/v0.9.11/`; `docket run`'s short help no longer claims to be read-only.
- **Correction surfaced during build:** an `agents:` model pin in `.docket.yml` or `.docket.local.yml` does not merely get ignored — docket refuses to change the repository until it is moved to the global config. The docs now say so.

## Human actions and testing

### Optional — read the rewritten pages you rely on

Why: the guard catches change citations and unsupported keys, but not every factual sentence. Workers checked claims against the code and `--help`, and a deep review spot-checked them; plain-prose accuracy is still a judgement call.

1. Open `docs/guide/five-steps.md`, `docs/install/config-layers.md`, and `docs/reference/config-keys.md` on the PR branch.
   Expected: each describes the commands and settings you use today, with no mention of single-branch mode, `skills:` rebinding, delegation, dummy mode, or auto-capture.
2. Copy `.docket.example.yml` to `.docket.yml` in a scratch git repository and run `docket diagnostic config --for-mutation --repo-dir <scratch>`.
   Expected: `configuration: valid` and `mutation: allowed`.

Cleanup: delete the scratch repository.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed at the build head; it is re-run at the final head before the PR, with the evidence in the PR body.
- One-off relative-link and anchor check over the 43 in-scope files: 0 problems. No living doc links to a deleted or renamed page.
- `.docket.example.yml` copied verbatim into a scratch repository passed `docket diagnostic config --for-mutation` (valid, mutation allowed, every value at its built-in default).
- Each new or reworked guard was mutation-tested: planted citations and unsupported keys, a shrunken root list, deleted or altered example keys, and an altered agents-table row each turned the tests red.
- Whole-branch review (deep tier): 8 findings (2 important, 6 minor), all fixed in-branch; full table in the PR body.

## Known issues and follow-ups

### Skills and agent files still describe retired behaviour

The skills (for example `docket-convention` and `docket-status`) still mention stale-claim and dependency-stall checks, `learnings.cap`, the `skills:` merge, the `agents.yaml` migration, and "Go v1". `docket-finalize-change`'s gate-failure reference also contradicts its SKILL.md on whether re-running finalize clears a repair sign-off block. This is confirmed and out of scope here; change #502 owns it.

### Two hand-synced copies of the unsupported-key matcher

The example-config test keeps its own copy of the docs guard's unsupported-key matcher (an import cycle prevents sharing it). If one copy changes and the other does not, the two guards can disagree. Neither copy matches a comment nested inside a comment (`#   # key: value`). Suggested next action: move the matcher into a small shared package.

### Refused values of supported keys are not guarded

The new guard catches unsupported keys, not refused values such as `finalize.gate: ci`. Review is the only check for those, as the design intended.

### `.gitignore` managed block still lists `.kiro` and `.windsurf`

`internal/reposetup/gitignore.go` still writes ignore globs for two harnesses docket no longer supports. This is binary behaviour, out of scope for a docs change; worth a small cleanup change.
