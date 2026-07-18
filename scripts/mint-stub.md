# scripts/mint-stub.sh — contract

## Purpose

The deterministic, mechanical half of **auto-capture** (change 0091). When an autonomous skill
judges that it has discovered genuine follow-up work, this script performs the mint: a cheap
active-slug dedup check, id allocation, the stub write from the change template with
`discovered_from:` provenance, and a compare-and-swap push onto the metadata branch.

The split is the ADR-0012 script-vs-model boundary: the **model decides what is material** (and
gates on `AUTO_CAPTURE`); the **script does the mint**. Every mint site therefore shares one
CAS-correct implementation instead of N hand-rolled copies. Per ADR-0021 it authors its own
formulaic commit.

## Usage

```
mint-stub.sh --changes-dir DIR --title TITLE --body-file FILE --discovered-from ID
             [--slug SLUG] [--minted N] [--cap N] [--remote R] [--template PATH]
```

Reached from a skill through the facade: `docket.sh mint-stub …`.

| Flag | Required | Meaning |
|---|---|---|
| `--changes-dir` | yes | the metadata worktree's changes dir (e.g. `.docket/docs/changes`) |
| `--title` | yes | the stub's title |
| `--body-file` | yes | file whose contents become the stub body verbatim; **must start with `## Why`** |
| `--discovered-from` | yes | originating change id; populates `discovered_from: [ID]` |
| `--slug` | no | derived from `--title` when omitted |
| `--minted` | no | stubs already minted by THIS skill invocation (default `0`) |
| `--cap` | no | per-invocation cap (default `3`) |
| `--remote` | no | default `origin` |
| `--template` | no | default `../skills/docket-new-change/change-template.md` |

## Behavior

1. **Validate** every argument; a malformed body (no leading `## Why`) is rejected before any write.
2. **Cap check** — `--minted >= --cap` refuses immediately (exit 4), before touching repo state.
3. **Dedup** — case-insensitive slugified match of the proposed slug against every **active** change's
   `slug:` and `title:`. Archived changes are deliberately NOT scanned: archived work is history, not
   a live duplicate. On a match: exit 3, nothing written.
4. **Allocate** — id = max `id:` across `active/` + `archive/`, plus one.
5. **Write** — render the stub from the change template: the template's instructional `# comment`
   scaffolding is stripped from the frontmatter block, then frontmatter scalars are rewritten inside
   the first `---…---` block only, an empty `## Artifacts` marker block follows (the block's sole
   writer remains `render-change-links.sh`), then the caller's body verbatim. The stub is an ordinary
   needs-brainstorm change: `status: proposed`, no `spec:`, `trivial: false`, `auto_groomable` left
   **unset** so it inherits the repo default — exactly like a scan-mode stub.
6. **Commit + CAS push** — stages and commits the ONE new change file, then pushes with a bounded
   5-attempt retry. On every non-fast-forward it fetches, `reset --hard`s to the fresh remote tip,
   and **re-derives both the dedup verdict and the next id from that origin state** — never from the
   working tree it just wrote. A concurrent writer that minted the same slug meanwhile turns the run
   into a duplicate skip. `reset --hard` is safe only because the script pushes per mint, so the local
   branch never carries more than this one commit.

Exactly one report line goes to stdout; the caller surfaces it.

## Exit codes

| Code | Meaning | Report line |
|---|---|---|
| 0 | stub minted and pushed | `minted <id> <slug>` |
| 3 | duplicate; nothing written | `skipped duplicate <slug> (matches #<id>)` |
| 4 | per-invocation cap reached; nothing written | `skipped cap-reached (cap <n>, minted <n>)` |
| 1 | usage/git error (diagnostic on stderr) | — |

## Invariants

- **Metadata-worktree writes only.** It touches exactly one new file under `active/` and never the
  originating change's `status:`/`branch:`/`pr:`, never `BOARD.md`, never a feature branch.
- **One stub per invocation.** Multi-stub capture is the caller looping, incrementing `--minted`.
- **Never merges or edits an existing change.** A near-duplicate is skipped, never amended.
- **No `gh`, no network beyond the git remote.** Offline-safe apart from the push.
- **The commit is formulaic** (`docket(<id>): auto-capture stub discovered from #<n>`) and touches a
  single path, keeping it trivially reviewable in history.

## Mock seams

`GIT` (default `git`), `TODAY` (default `date -u +%Y-%m-%d`). `tests/test_mint_stub.sh` drives a real
temp repo with a bare origin so the CAS push genuinely lands.
