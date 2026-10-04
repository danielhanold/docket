# Results, dispositions, reason tokens, and health codes

docket reports what happened in a small set of fixed words. This page names each vocabulary and
the surface that owns its current members. The machine-readable source for all of them is
`docket schema` (read-only, repository-independent): `docket schema --json` lists every closed
vocabulary, and `docket schema --operation <id>` shows one operation's result shape. An operation
that reads a JSON file (`--request`, `--input`, or `--body`) also shows that file's request shape.
A flag-only operation shows no request; `docket capabilities --json` lists its flags in its
`signature`. A required field inside an optional object is required only when that object is sent.

## Results

Every JSON result envelope (`--json`) carries a `result` field. It takes exactly one of:

- `applied` — the operation did its work.
- `no-op` — there was nothing to do; the state already matched.
- `contended` — another writer got there first; re-read and try again.
- `invalid-input` — the request itself was malformed.
- `invalid-state` — the request was well formed, but the repository is not in a state that
  allows it.
- `blocked` — something must be resolved first; the finding names it.
- `unsupported-config` — the resolved configuration is invalid or asks for something docket does
  not support; `docket diagnostic config --repo-dir .` shows why.
- `gate-failed` — a test suite ran and was red.
- `external-failed` — an outside tool (Git, the GitHub CLI, the network) failed or timed out.
- `interrupted` — the operation was stopped before it finished.
- `internal-error` — docket itself failed.

## Dispositions

A disposition is an operation's own, more specific outcome, reported beside `result`. Each
operation has its own disposition vocabulary — for example, `docket run cancel` reports
`cancelled`, `cancellation-pending`, `already-cancelled`, or `refused`. `docket schema --json`
lists them all under `vocabularies`.

`applied` | `no-op` | `refused` | `error` is the disposition set of `repository prepare` (the
startup check every workflow runs first) and of nothing else. The `docket-convention` skill
([`../../skills/docket-convention/SKILL.md`](../../skills/docket-convention/SKILL.md)) states what
each of those four obliges a caller to do.

## Reason tokens

When finalize refuses or halts, it names a typed reason token (for example, a mismatched PR head or
an unresolved review state). The token vocabulary and each token's remedy are owned by the finalize
skill's failure reference,
[`../../skills/docket-finalize-change/references/gate-failure.md`](../../skills/docket-finalize-change/references/gate-failure.md).
Read the current token and its recovery there, not from memory.

## Health codes

The health findings a status scan raises are structural: a linked spec, plan, or results file
that is missing (`artifact-missing`), a change that references one that does not exist
(`change-reference-dangling`), a dependency cycle (`change-dependency-cycle`), a malformed branch
name (`branch-malformed`), and configuration and parse diagnostics. `docket status --json` carries
the live findings for your repository; `docket status` is the human-readable form.
