<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0542 — Open the board, a change, and its artifacts from the terminal](../../changes/archive/2026-10-08-0542-open-the-board-a-change-and-its-artifacts-from-the-terminal.md)**
<!-- docket:backlink:end -->

# Open docket artifacts from the terminal: design

Change #542, groomed interactively on 2026-10-08. Related: #136 (artifact backlinks, which introduced the GitHub link builder this reuses), #531 (private visibility, which decides where private metadata lives), #534 (the `dckt` alias, so `dckt open …` works the same way). ADRs: 0019 (config-key layer classification), 0141 (plan, results, and evidence live on the metadata branch), 0142 (private visibility layout).

## Summary

Getting from the terminal to a change's spec, plan, results, or PR, or to the board, takes several clicks today. You open GitHub, switch to the `docket` branch, find the record under `active/` or `archive/`, and follow its Artifacts table. A private repository has no web page at all: the files live in a checkout under `~/.local/share/dckt/…`.

This change adds one command, `docket open <what> [id]`, where `<what>` is `board`, `change`, `spec`, `plan`, `results`, or `pr`. Run with no id inside a checkout whose branch belongs to a change, it opens that change's artifact.

- In a shared repository, artifacts open as GitHub pages in the browser by default. A new layered config key, `open.artifacts: github | local`, lets a user open the file from the local metadata checkout in their default Markdown app instead.
- Private repositories always open locally.
- `pr` always opens the PR's URL.

## Evidence gathered at grooming

- **The GitHub link builder already exists.**
  - `githubWebURL` (`internal/app/link_context.go`) turns origin into `https://github.com/<owner>/<repo>` for `git@github.com:`, `https://github.com/`, and `ssh://git@github.com/` remotes only. Any other host gives `""`.
  - `linkContextOf(pin)` is the only `render.LinkContext` constructor, which `link_context_guard_test.go` enforces.
  - `LinkContext.BlobURL(path)` (`internal/render/link.go`) returns `RepoWebURL + "/blob/<metadata branch>/" + path`. It returns `""` when `RepoWebURL` is empty, or when `PrivateMetadata` is set and the branch is the metadata branch. The PR-body backlinks use it today.
- **Private metadata has no web view.** `layout.PrivateLayout` puts the bare remote at `<DataHome>/dckt/<owner-repo>/remote.git` and the metadata checkout at `…/checkouts/<CloneID>`, outside the clone. A private PR body carries no backlinks; `PRPublish` skips them.
- **Record resolution is an established path.**
  - The path is `PinContext` (which fetches, via `loadOperationalContext`) → `ReadCorpus` → `parseCorpus` → `repository.BuildSnapshot` → `snap.Change(id)` (`LookupFound` / `LookupAbsent` / `LookupAmbiguous`). `resolvePRChange` and `loadWorkspaceContext` use it.
  - `domain.Change` exposes `Path()` (the record's real `active/` or `archive/` path), `Spec()`, `Plan()`, `Results()`, `PR()`, `Branch()`, and `Trivial()`.
  - No existing read operation returns spec, plan, results, and PR together.
- **Nothing infers a change from the current directory.** Every operation takes `--id`. The building blocks exist:
  - `gitcli.(*Client).WorktreeCheckoutState` (`internal/gitcli/checkoutstate.go`) returns the checked-out branch's full ref, or `Detached`.
  - `recordedBranch(c)` (`internal/app/branch_identity.go`) reads a record's branch.
- **No opener code exists.** Nothing in the Go tree runs `open`, `xdg-open`, or `gh browse`.
- **The metadata checkout can lag the metadata tip,** because operations read pinned blobs, not the checkout. `RunRepositoryPrepare` is the established sync:
  - a clean, behind checkout is fast-forwarded
  - a missing checkout is attached
  - a dirty, ahead, or diverged one is refused with a finding
- **One command keeps the catalog small.** Every public executable leaf must be a catalog entry whose id equals its dotted command path (`TestProductionCapabilityCorrespondence`), so one `open` leaf with a positional `<what>` costs one entry, not six.
- **The board path is fixed.** It is `boardCorpusPath(cfg)`, which is `<changes_dir>/BOARD.md`.
- **A template for the new key exists.** The `visibility` key (`internal/config/schema.go`) is a string enum leaf with `merge: mergeScalar`, `scope: scopeAny`, `disp: dispSupported`, and `validate: enumLeaf(…)`. That is exactly the shape `open.artifacts` needs.
- **Browsers show Markdown as source.** Chrome, Safari, and Firefox don't render a local `.md` file. GitHub renders it on its servers. That is why local opens go to the operating system's default handler for the file (on the development machine, a Markdown viewer app) rather than to a browser.

## Design

### 1. Command

```
docket open <what> [id] [--print] [--repo-dir <dir>] [--json]
```

- `<what>` is one of `board`, `change`, `spec`, `plan`, `results`, `pr`. Any other value is a usage error that lists the six.
- `board` takes no id. Passing one is a usage error.
- The id is a decimal change id with optional leading zeros (`541`, `0541`). `#541` is not accepted, because an interactive shell can treat a word that starts with `#` as a comment.
- With no id, the change is inferred from the checkout (section 3).
- On success the command prints the opened target on one line: the URL, or the absolute file path.
- `--print` prints the target without launching anything. It is for SSH sessions, scripts, and agents that want to hand a human a link.
- `--json` emits the protocol-v1 envelope with an `OpenResult`:
  - `what`
  - `change_id` (omitted for `board`)
  - `target`
  - `target_kind`: `url` or `file`
  - `launched`: false under `--print`
  - `notes`: a list of one-line notes, such as a fallback reason or a stale-checkout warning
- `dckt open …` works identically through the existing alias.

### 2. What each target opens

| `<what>` | GitHub target | Local target (file opened with the OS default app) |
|---|---|---|
| `board` | `BlobURL(<changes_dir>/BOARD.md)` | `<metadata checkout>/<changes_dir>/BOARD.md` |
| `change` | `BlobURL(change.Path())`, active or archived | `<metadata checkout>/<change.Path()>` |
| `spec` / `plan` / `results` | `BlobURL(<field value>)` | `<metadata checkout>/<field value>` |
| `pr` | the record's `pr:` value | the same `pr:` URL, because the PR lives on GitHub in both modes |

**Choosing GitHub or local** (applies to every `<what>` except `pr`):

1. Private repository: local. `open.artifacts` is not consulted, and no note is added.
2. Otherwise `open.artifacts: local`: local.
3. Otherwise (`github`, the default): use `linkContextOf(pin).BlobURL(path)`. When it returns `""` (origin is not a GitHub remote), open locally and add the note `origin is not a GitHub remote — opened the local file instead`.

No new visibility or host logic is added. The existing builder's empty result is the fallback trigger.

**Spec copy.** `spec` always opens the copy on the metadata branch, which exists from grooming onward. The copy on the integration branch, present only after merge, is not used.

**Board availability.** When the resolved `board_surfaces` has no `inline` surface, `open board` fails with `this repository has no board (board_surfaces has no inline surface)`, even if a stale `BOARD.md` is still present.

**PR value.** The `pr:` value opens as-is when it is an `http://` or `https://` URL. Anything else fails with an error that names the value. There is no shorthand expansion.

### 3. Inferring the change

When `<what>` is not `board` and no id is given:

1. Read the checkout state of the resolved repo dir (`--repo-dir`, defaulting to the current directory) with `WorktreeCheckoutState`.
2. Detached HEAD fails: `HEAD is detached — pass a change id`.
3. Compare the branch's short name with every record's recorded branch (`recordedBranch`). Check active records first, and archived records only when no active record matches.
4. Exactly one match: that change. No match: `branch <name> belongs to no change — pass a change id`. That covers the metadata checkout and the main checkout on `main`. More than one active match: `branch <name> is recorded by changes <a>, <b> — pass a change id`.

Matching is by branch, not by folder path, so it works in `.worktrees/<slug>` and in the main checkout when a feature branch is checked out there.

### 4. Local opens and freshness

Before any local open, the command runs the same work as `repository.prepare` (`RunRepositoryPrepare`) against the repository:

- `applied` / `no-op`: the checkout is current, or was just fast-forwarded or attached. Open the file.
- `refused` / `error`: the checkout may be stale, for example because of uncommitted edits. Open the file on disk anyway and add a note naming prepare's finding code: `metadata checkout not synced (<code>) — file may be stale`. This never blocks.
- If the file does not exist on disk, fail and name both the path and prepare's finding.

The path opened is `<metadata_worktree_path>/<repo-relative path>`, using the prepare context's `metadata_worktree_path`. The command never assumes `.docket` or a private store path.

### 5. The `open.artifacts` key

- New schema entry modelled on `visibility`: `{path: "open.artifacts", kind: kindString, enum: []string{"github", "local"}, def: "github", merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: enumLeaf("github", "local")}`.
- New `Open.Artifacts Value[string]` on `config.Effective`, assigned in `resolve.go`.
- It is an ordinary layered key: repo-local > repo-committed (or `.git/dckt/config.yml` in a private repo) > global > built-in. It is not a repository-identity key, so under ADR-0019 it is honoured in every layer.
- In a private repository it resolves normally but has no effect, because private metadata has no GitHub copy. This is deliberately silent. A global `github` would otherwise produce a note on every open in every private repository.
- An unknown value fails resolution like every other enum key.

### 6. Opener

- A small seam injected into the app deps, never called directly. `open` on darwin, `xdg-open` on linux, and no opener on any other platform.
- The opener receives the URL or absolute path as a single argv element, with no shell. The command waits for the opener to exit and reports a non-zero exit as an error.
- With no opener available (an unsupported platform, or `xdg-open` not on `PATH`), the command prints the target and fails with `no opener available on <os> — open it yourself, or use --print`.
- Tests always inject a fake. The suite never launches a real application.

### 7. Catalog, schema, docs

- CLI: `internal/cli/open.go`, registered in `root.go`. Annotation `capability("open", EffectRead, EffectLocalWrite, EffectProcessControl)`. `local-write` covers the prepare sync, and `process-control` covers launching the opener.
- Schema: `{ID: "open", Request: nil, Result: OpenResult{}}` in `operationBindings` (`internal/app/schema_registry.go`), the shape `status` and `run.verify` use.
- Update the asset-independent allowlist in `internal/cli/install.go` (`TestAssetIndependentSetExact`) and the catalog and schema production tests (`capability_production_test.go`, `schema_production_test.go`) for the new entry.
- Docs:
  - `docs/reference/cli.md`: one noun line for `docket open`, naming the six targets and the no-id inference. That page is a pointer list and never lists flags; `docket open --help` owns those.
  - `.docket.example.yml`: the `open.artifacts` key with its description. ADR-0048 makes this file canonical, and an existing test keeps it in step with the schema, so that test reddens until the key is added.
  - `docs/reference/config-keys.md`: the key's row (default `github`, scope any layer).

### 8. Errors

Every error is one line with a non-zero exit:

| Situation | Message |
|---|---|
| unknown `<what>`, or an id with `board` | usage error listing the six targets |
| malformed id | `invalid change id <arg>` |
| no such change | `no change <id>` |
| field not set | `change <id> has no plan yet` (likewise `results`, `PR`, and `spec` on a non-trivial change) |
| trivial change, `spec` | `change <id> is trivial — it has no spec` |
| board disabled | `this repository has no board (board_surfaces has no inline surface)` |
| `pr:` not a URL | `change <id> pr: <value> is not a URL` |
| inference failures | section 3 messages |
| local file missing | names the path and prepare's finding |
| no opener | section 6 message, after printing the target |

## Tests

- **Target table (unit, fake opener, fake reader).** Cover {shared + `github`, shared + `local`, shared with a non-GitHub origin, private} × {`board`, `change` (active and archived), `spec`, `plan`, `results`, `pr`}. Assert `target`, `target_kind`, and notes. Assert that private mode ignores `open.artifacts: github` silently, and that the non-GitHub fallback adds its note.
- **Inference.** Cover a match in a worktree, the main checkout on a feature branch, archived fallback when no active record matches, no match, detached HEAD, the metadata checkout, and two active records recording one branch. Each error message is asserted.
- **Errors.** Each row of section 8.
- **Output.** `--print` launches nothing and prints the target. `--json` carries the `OpenResult` fields. The success path prints exactly one line.
- **Opener selection** by GOOS, the missing-`xdg-open` path, and argv passed as one element.
- **Config.** `open.artifacts` resolves per layer with normal precedence, defaults to `github`, and refuses an unknown value. The registry-default parity tests pick it up.
- **Integration (real fixture repository, private layout).** A behind metadata checkout is fast-forwarded before a local open. A dirty one opens with the stale note. A missing file fails naming the path.
- **Catalog and schema production tests** updated for the `open` entry.
- **Mutation checks (AGENTS.md).** Strip the active-before-archived ordering, the `BlobURL`-empty fallback, and the private short-circuit in turn, and confirm a test reddens for each.

## Out of scope

- Web links for hosts other than GitHub (GitHub Enterprise, GitLab, and others). Those fall back to local.
- Rendering Markdown to HTML or serving a local preview site.
- Opening ADRs, learnings, or the integration-branch copy of a spec.
- Shell completion for change ids.
- Changing the board or the `## Artifacts` table to advertise the command.
