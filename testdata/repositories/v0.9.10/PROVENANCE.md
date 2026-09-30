# v0.9.10 docket-self fixture

Source: docket's own committed `.docket.yml` and `xdg/docket/config.yml` as of commit
`a96558229` (config: enable claude and opencode agent harnesses), 2026-09-30.

Cut from `v0.9.8/docket-self`. The ONLY file that changed from v0.9.8 is
`docket-self/repo/.docket.yml`: it gains an explicit `agent_harnesses: [claude, opencode]`
declaration. `agent_harnesses` is not a deferred capability, so the preflight blocker set is
unchanged: the machine-global `auto_capture.enabled` request remains the sole blocker.
Everything else is carried verbatim. Older versioned fixture trees (v0.9.2–v0.9.9) are immutable
inputs and are never edited in place.
