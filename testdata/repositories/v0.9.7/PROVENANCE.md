# v0.9.7 docket-self fixture

Source: docket's own committed `.docket.yml` and `xdg/docket/config.yml` as of change 0374
(separate build and finalize test configuration), 2026-08-31.

Cut from `v0.9.6/docket-self` by change 0374. The ONLY file that changed from v0.9.6 is
`docket-self/repo/.docket.yml`: it adds `build.gate` and `build.test_command` (the build role's own
whole-suite gate, an independent setting from finalize's) and drops the stale frozen-oracle sentence
from the `finalize.test_command` comment. Everything else is carried verbatim. Older versioned
fixture trees (v0.9.2–v0.9.6) are immutable inputs and are never edited in place.
