# v0.9.12 docket-self fixture

Source: docket's own committed `.docket.yml` and `xdg/docket/config.yml` as of the change that
keeps plan, results, and build evidence on the metadata branch, 2026-10-06.

Cut from `v0.9.10/docket-self`. The ONLY file that changed from v0.9.10 is
`docket-self/repo/.docket.yml`: it drops the `finalize.skip_results_only_delta: false` line and its
comment block, because that setting is now an obsolete tombstone. An explicit `false` requested no
deferred capability, so the preflight blocker set is unchanged: the machine-global
`auto_capture.enabled` request remains the sole blocker. Everything else is carried verbatim.
Older versioned fixture trees (v0.9.2–v0.9.11) are immutable inputs and are never edited in place.
