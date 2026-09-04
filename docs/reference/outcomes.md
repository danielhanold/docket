# Dispositions, reason tokens, and health codes

The one-word outcomes docket reports, the reason tokens a blocked finalize prints, and the health
codes a status scan raises are each defined in exactly one owning surface. This page points at those
owners and copies none of the vocabularies, so it can never list a token the current binary no
longer emits.

## Dispositions

A disposition is the one-word outcome an operation reports: applied, no-op, refused, or error. This
closed vocabulary is owned by the `docket-convention` skill's Step-0 preamble
([`../../skills/docket-convention/SKILL.md`](../../skills/docket-convention/SKILL.md)), which states
it as **"the closed vocabulary `applied` | `no-op` | `refused` | `error`"** and defines what each
one obliges a caller to do. The exact closed set the binary emits is also machine-readable via
`docket schema` (the schema/vocabulary command).

## Reason tokens

When finalize refuses or halts, it names a typed reason token (for example, a mismatched PR head or
an unresolved review state). The token vocabulary and each token's remedy are owned by the finalize
skill's failure reference,
[`../../skills/docket-finalize-change/references/gate-failure.md`](../../skills/docket-finalize-change/references/gate-failure.md).
Read the current token and its recovery there, not from memory.

## Health codes

A health check is a status-time scan for things a human should look at: stale claims, broken links,
stalled dependencies. The health codes a scan can raise, and their current data, are owned by
`docket status --json` — its output carries the live health section for your repo. The human-readable
form is `docket status`. The convention skill's lifecycle and learnings-ledger sections give the
background each code is checking against.
