# v0.9.8 docket-self fixture

Source: docket's own committed `.docket.yml` and `xdg/docket/config.yml` as of change 0468
(rename colliding docket terms and retire obsolete glossary entries), 2026-09-29.

Cut from `v0.9.7/docket-self` by change 0468. The ONLY file that changed from v0.9.7 is
`docket-self/repo/.docket.yml`: two comments move off the change-lifecycle sense of "terminal"
(ADR-0129 row 64) — "published terminal records" becomes "published archived records", and
"mirrors its terminal records" becomes "mirrors its final records". No key or value changed.
Everything else is carried verbatim. Older versioned fixture trees (v0.9.2–v0.9.7) are immutable
inputs and are never edited in place.
