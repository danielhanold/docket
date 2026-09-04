# The CLI by noun and verb

This page is a map, not a transcript. It names the `docket` commands and, for each, where the
current, authoritative detail lives — so nothing here can drift out of step with the binary you
actually have installed.

**How to read the current value:**

- **The noun list** — every top-level command — is owned by `docket --help`. Run it to see what
  ships in your build.
- **The verbs and flags under a noun** are owned by `docket <noun> --help` (for example,
  `docket change --help`). This page never lists flags: they change with the binary, and the help
  output is the one place that is always right.
- **The capability catalog** — the machine-readable list of every operation the `docket` binary
  offers, which skills read instead of hard-coding commands (a skill is a named, reusable
  instruction set an agent loads for one job) — is owned by `docket capabilities --json`. That JSON,
  not this page, is what the workflows resolve their commands from.

## Nouns

Each line is a pointer: the command, what its verbs govern, and where to read the verbs.

- **`docket adr`** — record, supersede, and reverse architecture decisions. Verbs: `docket adr --help`.
- **`docket artifact`** — render docket-managed blocks into workflow artifacts. Verbs: `docket artifact --help`.
- **`docket capabilities`** — emit the binary's complete executable command catalog (read-only,
  repository-independent). Verbs: `docket capabilities --help`.
- **`docket change`** — create and transition changes in the backlog. Verbs: `docket change --help`.
- **`docket context`** — assemble read-only context bundles for the implementation workflow. Verbs: `docket context --help`.
- **`docket development`** — contributor operations against a docket checkout, including the test
  suite gate. Verbs: `docket development --help`.
- **`docket diagnostic`** — read-only diagnostics. Verbs: `docket diagnostic --help`.
- **`docket evidence`** — record and verify the build evidence that certifies an exact tested
  commit. Verbs: `docket evidence --help`.
- **`docket finalize`** — sequence a change's terminal half: rebase, publish, merge, and closeout. Verbs: `docket finalize --help`.
- **`docket gate`** — launch, observe, stop, and recover supervised local gate runs. Verbs: `docket gate --help`.
- **`docket install`** — install docket's skills, agents, and dispatch material into your
  harnesses. Verbs: `docket install --help`.
- **`docket learning`** — record and update manual learning findings. Verbs: `docket learning --help`.
- **`docket maintenance`** — reclaim docket's terminal half in batch (status stays read-only). Verbs: `docket maintenance --help`.
- **`docket pr`** — publish the ready-for-review pull request for an in-progress change's tested
  head. Verbs: `docket pr --help`.
- **`docket repository`** — initialize, migrate, and check the docket repository topology. Verbs: `docket repository --help`.
- **`docket run`** — report on a change's claim-to-implemented run (read-only). Verbs: `docket run --help`.
- **`docket schema`** — emit request/result payload schemas and closed vocabularies (read-only,
  repository-independent). Verbs: `docket schema --help`.
- **`docket status`** — report backlog status, readiness, selection, and repository health
  (read-only). Verbs: `docket status --help`.
- **`docket version`** — report the binary's build identity. Verbs: `docket version --help`.
- **`docket workspace`** — prepare, inspect, and publish feature workspaces for in-progress
  changes. Verbs: `docket workspace --help`.
