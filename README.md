# docket

docket keeps a backlog of planned work as plain markdown files inside your repo and ships
agent skills that drain that backlog to open pull requests. It is a repository-level
implementation of the Plan, Design, Build, Test, and Deploy stages of Anthropic's
[AI-Native SDLC Playbook](https://claude.com/blog/the-ai-native-sdlc-playbook)
([stage-by-stage comparison](docs/comparison/ai-native-sdlc-playbook.md)) — git-native,
harness-neutral, with the human at the merge. Each unit of work is a **change**: one markdown
file, roughly one pull request's worth of work, that moves through a fixed lifecycle from idea
to archived record — coordinated entirely through git, with no service, no database, and no
CLI to install.

## What you get

- **A durable backlog that outlives the session.** Planned work is tracked in-repo as
  markdown, so a change you brainstorm today is still there, with its full context, when an
  agent picks it up next week.
- **Hands-off implementation.** An autonomous skill claims the next ready change, refreshes it
  against the current state of the code, builds it with test-driven development, and opens a
  PR — with no supervision in between.
- **You stay at the merge gate.** Agents never merge on their own authority. Your review of
  the pull request is the one required human checkpoint on the way to `done`.
- **No new infrastructure.** Markdown files, git, and skills any supported harness can run —
  Claude Code, Cursor, Codex, and opencode are first-class.
- **The right model for each step.** Every autonomous skill is pinned to its own model and
  effort, so a board refresh runs at a cheap tier while a build runs at a top one — see
  [Models: tuning model and effort per task](docs/install/models-and-effort.md).

## The committed artifact chain

Every stage ends in an artifact committed to git before the next stage reads it. The chain is
the audit trail — and the whole interface between you and the autonomous skills.

| Stage | What docket commits | Where it lives |
|---|---|---|
| Plan | The change file: why, what changes, out of scope, dependencies, priority, type | `docs/changes/active/` on the `docket` metadata branch; rendered on `BOARD.md` |
| Design | The spec, linked from the change (or `trivial: true` for small mechanical work) | `docs/superpowers/specs/` on the metadata branch |
| Build | A dated reconcile log, the implementation plan, one verified commit per task | The change body; the plan on the feature branch |
| Test | The build-evidence record — suite command, result, head SHA — plus the results file and the review's disposition table | The PR body; `docs/results/` |
| Deploy | The merge (proven reachable), the archived change record, a cleaned branch and worktree, a re-rendered board | `docs/changes/archive/`; `BOARD.md` |

## Why docket: plans rot

A change is drafted against a snapshot — the codebase, the decision ledger, and the other
in-flight changes as they stood the day you designed it. In an async backlog the implementer
may not pick it up for weeks. By then another change has shipped half of it, an architecture
decision has settled an open question the other way, or an interface it assumed has moved.
Most backlog systems build the ticket as written and let the implementer discover the
mismatch halfway through.

docket instead **reconciles at the last responsible moment**: after a change is claimed but
before any build work starts, the implementer re-reads it against related and archived
changes, recent decisions, and the current code — then rewrites its scope to what is true
now, records a dated reconcile log entry, kills it if it has become obsolete, or stops and
escalates if the design itself no longer holds.

The stance: plans rot, so refresh them just-in-time and never trust a stale backlog. The
`reconciled` flag on every built change is the visible proof it happened.

## Where you decide

- **Creating and grooming changes.** Capturing an idea and designing it to build-ready are
  interactive; autonomous grooming is opt-in per stub, and killing or deferring a change is
  never autonomous.
- **Merging the PR.** The one required checkpoint — the implementer stops at an open pull
  request every time.
- **Finalize's confirmations.** Close-out merges only with your authorization, and unattended
  repair at the merge gate blocks for your sign-off.
- **Promoting a learning.** Findings graduate into the always-loaded instructions file only by
  your hand.
- **Filing discovered work.** Runs report follow-up work; a human decides what enters the
  backlog.

Plan approval is deliberately **not** a human point: your checkpoint is the PR, where the plan,
the diff, and the evidence arrive together. And docket ends at the merge — it does not deploy to production,
monitor production, or feed incidents back into the backlog.

## Install and the daily loop

```bash
cd ~/dev/docket
git fetch --tags && git pull
bash install.sh
```

Re-run `install.sh` after every update — it is idempotent and machine-global. Full
prerequisites and what an install run does: [Installing docket](docs/install/install.md). To adopt
docket in an existing repo, run `docket repository migrate` from inside it
([Migration](docs/guide/where-the-metadata-lives.md)).

The daily loop, one skill per step
([Quickstart](docs/guide/daily-loop.md)):

1. **Capture** an idea into the backlog — `docket-new-change`.
2. **Groom** rough stubs to build-ready — `docket-groom-next` (or `docket-auto-groom`).
3. **Drain** the backlog to open PRs — `docket-implement-next` (or `/loop` it hands-free).
4. **Review and merge** the PR — you.
5. **Close out** merged work — `docket-finalize-change`; `docket-status` keeps the board
   honest in between.

## Documentation map

Start at **[the documentation index](docs/README.md)** — it lays out every section below and a
start-here path through them.

- **[Install and configure](docs/install/README.md)** — get docket onto your machine, decide where
  each setting lives, and set up your harness (one page each for Claude Code, Cursor, Codex, and
  opencode).
- **[Guide](docs/guide/README.md)** — how do I do the thing: one goal per page, end to end, each
  titled by the docket component it is about.
- **[Concepts](docs/concepts/README.md)** — what each piece is and why it is shaped the way it is,
  linking the decisions behind it.
- **[Reference](docs/reference/README.md)** — exact fields, keys, verbs, and outcomes, each
  pointing at the surface that owns the current value; includes the
  [harness runbooks and examples](docs/reference/harness/README.md).

## Status

docket-mode — planning metadata on its own orphan `docket` branch — is the supported default;
`main`-mode (`metadata_branch: main`) remains a simple opt-out that keeps everything on one
branch. Five documented features are deferred from Go v1 and activate nothing today:
`auto_capture`, `terminal_publish`, the automated learnings harvest/index/promotion,
`dummy_mode`, and `github_project`.

## License

docket is open source under the [Apache License 2.0](LICENSE). The
[NOTICE](NOTICE) file carries the attribution that section 4(d) of the license
asks redistributors to preserve.

- **Files docket generates in your repository are yours.** Change documents,
  specs, plans, results, decision records, and configuration belong to you; the
  license places no condition on them.
- **The license grants no rights in the docket name.** Use the software freely;
  do not present a modified version as docket itself.
- **Contributions** are accepted under the Developer Certificate of Origin; see
  [CONTRIBUTING.md](CONTRIBUTING.md).

The license applies to the whole history of this repository, including every
commit made before it was added.
