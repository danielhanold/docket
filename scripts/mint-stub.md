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
   a live duplicate. On a match: exit 3, nothing written. Slugification is idempotent (trims trailing
   `-` both before AND after the 60-char cap), so a previously truncated slug and a freshly truncated
   one always compare equal.
4. **Allocate** — id = max `id:` across `active/` + `archive/`, plus one.
5. **Write** — recreates `active/` (`mkdir -p`) if a prior CAS reset pruned it, then renders the stub
   from the change template: the template's instructional `# comment` scaffolding is stripped from
   the frontmatter block, then frontmatter scalars are rewritten inside the first `---…---` block
   only, an empty `## Artifacts` marker block follows (the block's sole writer remains
   `render-change-links.sh`), then the caller's body verbatim. Every frontmatter value — most notably
   `--title`, which is model-authored English prose, not a script constant — is written back
   byte-for-byte; a literal `|`, `&`, or `\` in the title is never reinterpreted as replacement
   syntax. The stub is an ordinary needs-brainstorm change: `status: proposed`, no `spec:`,
   `trivial: false`, `auto_groomable` left **unset** so it inherits the repo default — exactly like a
   scan-mode stub. Any failure in this step (a bad field write, a directory that still can't be
   created, the render itself) aborts before anything is staged or committed.
6. **Commit + CAS push** — stages and commits the ONE new change file, then pushes with a bounded
   5-attempt retry. On every **lost race** (push rejected as non-fast-forward) it fetches, resets
   `--hard` to the fresh remote tip, and **re-derives both the dedup verdict and the next id from
   that origin state** — never from the working tree it just wrote. A concurrent writer that minted
   the same slug meanwhile turns the run into a duplicate skip. A push failure that is **not** a lost
   race (auth, network, remote gone, a rejecting hook, …) is a real error: it dies immediately with
   the captured git diagnostic instead of retrying. Either way — an immediate real-failure die, or
   exhausting all 5 retries without converging — the local branch is reset back to the fresh remote
   tip before the script exits, so it never leaves a dangling unpushed commit behind. That cleanup
   reset (like every `reset --hard` in this script) is itself gated by a clean-tree precondition: see
   Invariants.

Exactly one report line goes to stdout; the caller surfaces it.

## Exit codes

| Code | Meaning | Report line |
|---|---|---|
| 0 | stub minted and pushed | `minted <id> <slug>` |
| 3 | duplicate; nothing written | `skipped duplicate <slug> (matches #<id>)` |
| 4 | per-invocation cap reached; nothing written | `skipped cap-reached (cap <n>, minted <n>)` |
| 1 | usage/git error (diagnostic on stderr) | — |

Exit 1 covers several distinct git-level failures, all diagnosed on stderr: a non-race push failure,
retry exhaustion (5 lost races that never converged), and a refused CAS reset because the worktree
carried uncommitted changes this run did not itself create (see Invariants). In the first two cases
the local branch is left matching the remote before the script exits; in the third, the refusal
means the reset never ran, so nothing beyond this run's own (still-local, unpushed) commit changes.

## Invariants

- **Metadata-worktree writes only.** It touches exactly one new file under `active/` and never the
  originating change's `status:`/`branch:`/`pr:`, never `BOARD.md`, never a feature branch.
- **One stub per invocation.** Multi-stub capture is the caller looping, incrementing `--minted`.
- **Never merges or edits an existing change.** A near-duplicate is skipped, never amended.
- **No `gh`, no network beyond the git remote.** Offline-safe apart from the push.
- **The commit is formulaic** (`docket(<id>): auto-capture stub discovered from #<n>`) and touches a
  single path, keeping it trivially reviewable in history.
- **`reset --hard` never discards uncommitted work it did not itself create.** The script shares its
  metadata worktree with other autonomous agents. Immediately after this script's own commit the
  tree is clean by construction, so ANY `git status --porcelain` output right before a `reset --hard`
  can only be another writer's uncommitted work; every `reset --hard` in this script (mid-retry, on a
  non-race push failure, and on retry exhaustion) is preceded by this check and refused (exit 1) if
  the tree is dirty. This is what keeps the CAS's core promise intact: a reset only ever discards
  THIS run's own last commit, never anything it didn't write.
- **No unpushed commit is left behind on error**, except in the rare compound case where the
  clean-tree precondition above refuses the cleanup reset itself — that failure mode is surfaced on
  stderr rather than silently resolved, since silently discarding the other writer's work is exactly
  what the precondition exists to prevent.

## Mock seams

`GIT` (default `git`), `TODAY` (default `date -u +%Y-%m-%d`). `tests/test_mint_stub.sh` drives a real
temp repo with a bare origin so the CAS push genuinely lands.
