---
slug: frozen-fixture-corpus-trips-repo-wide-scans
hook: "A frozen fixture corpus is DATA, not source — a repo-wide pattern guard will match inside it and must exclude it, with the exclusion itself bounded and mutation-tested."
topics: [testing, guards, fixtures]
changes: [307]
created: 2026-08-14
updated: 2026-08-14
promotion_state: retained
promoted_to:
---

## Apply
Repo-wide scans (portability greps, banned-literal gates, style guards) are written against
*maintained source* but run against the *whole tree*. The moment the repo grows a frozen corpus —
recorded fixtures, golden files, a captured historical snapshot — the scan starts reporting the
corpus's content as if the repo had authored it, and the corpus cannot be edited to satisfy it
without destroying the thing that makes it a fixture. Exclude the corpus path explicitly, and pair
the exclusion with the two controls that keep it honest: a **bounded** pattern (the exclusion names
the fixture directory, never a wildcard that would swallow future real source) and a
**mutation test** proving the scan still reddens for a violation planted just outside it. The same
pass also has to separate *false positives in real source* from the fixture case — a language
construct that merely looks like the banned pattern needs its own narrowed rule, not a second
exclusion.

## War story
- 2026-08-14 (#307, PR #208) — the repo-wide grep-portability scan flagged `{0,600}` inside a
  v0.9.2 frozen corpus record (a real ERE bound BSD grep rejects, but the corpus is a verbatim
  historical artifact), and separately false-positived on Go slice literals like `{305}`. Fixed by
  excluding `internal/repository/testdata/corpus/*` and constraining the Go-literal case, with
  mutation-verified boundedness controls so neither exclusion can silently widen.
