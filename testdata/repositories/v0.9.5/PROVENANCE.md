# v0.9.5 docket-self fixture

Source: docket's own committed `.docket.yml` and `xdg/docket/config.yml` as of change 0363
(Remove main-mode compatibility from Go v1), 2026-08-28.

Cut from `v0.9.4/docket-self` by change 0363. The ONLY file that changed from v0.9.4 is
`docket-self/repo/.docket.yml`: the obsolete `metadata_branch:` key was removed, because Go v1
supports one metadata topology (the fixed orphan `docket` branch) and the key is now a decode-only
obsolete tombstone rather than a setting. Everything else is carried verbatim. Older versioned
fixture trees (v0.9.2–v0.9.4) are immutable inputs and are never edited in place.
