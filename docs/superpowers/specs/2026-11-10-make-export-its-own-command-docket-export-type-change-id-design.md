# `docket export` — export as its own command

Change 0589 — Make export its own command: docket export <type> <change_id>

Change #589, groomed interactively on 2026-11-10.

## Why

Exporting an artifact is a different job from opening one, but today it is a flag on open: `docket open spec 9 --export`. That reads as "open this, and also export it", and `open` has to police flags (`--output-path`, `--warnings`) that only mean anything with `--export`. The ticket work (changes 581, 582, and the link-ticket and update-ticket skills) now routes ticket bodies and attachments through the export, so export is a first-class job and gets a command named for it.

## Goals

- `docket export <type> [<id>]` does everything `docket open <type> [<id>] --export` does today, except the one opening change settled below (`--output-path` no longer opens the copy).
- `docket open` only opens.
- The `docket-export` skill and the docs name the new command.

## Non-goals

- Changing what the export writes: kept sections, link removal, reference rewriting, the self-check, its warnings, the file name.
- An export for the board or pull requests.
- Any other change to `docket open`.

## Settled at grooming

- **Command and catalog id.** A new top-level command `docket export <type> [<id>]`, `<type>` one of `change`, `spec`, `plan`, `results`. Its capability-catalog id is `export`, following the house pattern that a top-level command's id is its name (`open`, `status`). Without an id it resolves the change whose recorded branch is checked out, exactly as `open` does.
- **Opening behavior.**
  - No `--output-path`: write the copy to the default destination (`~/Downloads` on macOS, the home directory elsewhere — unchanged) and **open the copy**, as `open --export` does today.
  - `--output-path <dir-or-file>`: write the copy there and **print its path; open nothing**. The person named the destination, so they know where the file is.
  - `--print`: write the copy and print its path; open nothing, whatever the destination. Kept because the `docket-export` skill exports to the default destination for file requests and must never pop a viewer at a person who may not be at the screen.
- **`docket open` loses `--export`, `--output-path`, and `--warnings` outright.** No alias, no message naming the new command — the same posture change 582 took for the `issue` → `ticket` kind rename. `open --export` fails with the CLI's ordinary unknown-flag error.
- **Flags of `docket export`:** `--output-path`, `--warnings all|leftovers|mentions` (default `all`), `--print`, `--repo-dir`, and the global `--json`. Their meaning, validation, and messages are today's, and so is where they are checked: usage mistakes stay **CLI usage errors** (`invalid-arguments`, exit 2, no operation document), caught in the command's argument check before the operation runs — a type outside the four (`board`, `pr`, anything else), a bad `--warnings` value, an empty `--output-path` (builder's call made at grooming: this matches today's `open --export`, whose `Args` hook rejects the same mistakes). Everything else about the flags is unchanged (`--output-path` given empty is refused; the destination rules — an existing directory takes the copy under its generated name, any other path is the exact file and its parent must exist; the source artifact and anything inside the metadata checkout are refused).

## Design

### Behavior that moves unchanged

Everything below is today's `open --export` behavior and keeps working byte-for-byte; only its entry point changes. The one deliberate difference is that `--output-path` without `--print` no longer opens the copy (today it does — `openExport` ends in the shared open step, which launches unless `--print` is set); a moved test expecting that launch changes with it.

- reading the artifact from the local metadata checkout whatever the repository's open routing (shared, private, `open.artifacts`), with the same stale-checkout note when preparing the checkout fails;
- the file name `<date>-<kind>-<name>.md` (kinds `ticket`, `spec`, `plan`, `results`), the transform, the self-check, the atomic write, the `replaced` note;
- the refusal reasons and result classes: `unknown-change`, `ambiguous-change`, `artifact-unset`, `artifact-path-invalid`, the branch-inference reasons (`head-detached`, `branch-unmatched`, `branch-ambiguous`, `checkout-unreadable`), `file-missing`, `output-path-invalid`, `output-path-protected`, `export-failed`, `no-opener`, and `opener-failed`. A type outside the four is the CLI usage error above, not an operation reason; if the app-layer entry point is called directly with one it refuses as `invalid-target` (today's direct-call reason is `invalid-option`, which goes away with `ValidateOpenExport`).

### Result document

The result's envelope carries operation `export`. Its fields are today's export result: `what`, `change_id`, `exported_path`, `replaced`, `launched`, `notes` (always a list), `leftovers`, `mentions` (both following `--warnings`), and `reason` / `message` on a refusal or failure. Today's `target` and `target_kind` are dropped from it — on an export they always equal `exported_path` and `file` (builder's call made at grooming; nothing reads them for an export — the `docket-export` skill reads `result`, `exported_path`, `replaced`, `leftovers`, `mentions`, and `message`). Because the fields differ from `open`'s, the export has its own result type, registered in `internal/app/schema_registry.go` under `export` beside `open`'s `OpenResult` entry; its human text prints `exported_path` (`OpenResult.HumanText` prints `target`). Human output is unchanged: the path alone on success, so `"$(docket export spec 9 --print)"` composes; notes, then warnings, on stderr; the message on a refusal.

`launched` is `true` only when the copy was handed to the opener; it is `false` under `--print` or `--output-path`.

### Effects

The `export` entry declares `read`, `local-write`, and `process-control` (it still launches the opener on the default path). `open` keeps the same three: it still prepares the metadata checkout and launches the opener.

### `docket open` after the removal

`OpenOptions` loses its export fields; `ValidateOpenExport` and the `invalid-option` reason go with them (no remaining use). Open's targets, routing, `--print`, and result are otherwise unchanged; `open`'s help text drops the export paragraphs. The change-resolution and local-artifact helpers `Open` and the export both need (target and change resolution, branch inference, `localArtifact`, `finishOpen`'s opener call) are shared, not copied — how to factor them is the builder's call.

### Callers and docs

Derive the site list from a whole-repo grep for `--export`, `--output-path`, `--warnings`, and the `open` operation, sorting hits into maintained source vs point-in-time records; only maintained source changes. Known sites today:

- `skills/docket-export/SKILL.md`: section 1 takes the `export` entry's argv instead of `open`'s; section 3 runs `<export argv> <what> [<id>] --print --json --repo-dir <repo root>`, always `--print --json`, with the same file-request and outside-tool rules (`--output-path` as today).
- `skills/docket-link-ticket/references/create.md` uses `open … spec <id> --print --json` to get the spec's URL — that is a real open, not an export, and stays.
- `docs/reference/cli.md`: a `docket export` entry; the `docket open` entry drops its export sentence.
- `docs/reference/glossary.md`: the `docket-export` entry names the `export` operation.
- `internal/repoguard/prose_contracts_test.go`: the `change_0578_export_skill` sentinel's `` `--export --print --json` `` clause follows the skill's new spelling; mutation-test it.
- `internal/cli/capability_production_test.go`: the `open` usage string loses the three flags; an `export` usage string is added.
- `internal/app/schema_registry.go`: an `export` entry with the new result type.
- The embedded asset tree (`go generate ./internal/assets/`) after the skill edit.

Point-in-time records (release notes, archived changes, results, specs, plans) keep `open --export` as written.

## Acceptance criteria

- `docket export spec <id> --print --json` writes the same file, with the same bytes, that `docket open spec <id> --export --print --json` writes before this change, and reports the same `exported_path`, `replaced`, `leftovers`, and `mentions`.
- `docket export spec <id>` with no `--output-path` and no `--print` opens the copy (`launched: true`); with `--output-path` or `--print` it opens nothing (`launched: false`) and prints the path.
- `docket export board`, `docket export pr 3`, `--warnings bogus`, and an empty `--output-path` are CLI usage errors (`invalid-arguments`, exit 2), as `open --export`'s are today.
- `docket open` rejects `--export`, `--output-path`, and `--warnings` as unknown flags.
- The capability catalog lists `export`, and the `docket-export` skill resolves its argv from that entry.
- The whole suite is green.

## Left to the builder

- How to factor the shared resolution code between `Open` and the export, and where the export's app-layer code lives.
- Test placement; the existing `open --export` tests move to the new command rather than being rewritten from scratch.
