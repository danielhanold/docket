# v0.9.6 docket-self fixture

Source: docket's own committed `.docket.yml` and `xdg/docket/config.yml` as of change 0318
(Go-native whole-suite test runner and gate cutover), 2026-08-29.

Cut from `v0.9.5/docket-self` by change 0318. The ONLY file that changed from v0.9.5 is
`docket-self/repo/.docket.yml`: `finalize.test_command` moved from `scripts/run-tests.sh` to the
branch-faithful source entry `go run ./cmd/docket development test`, because the merge gate now runs
the Go-native whole-suite runner and must test the exact checkout under review rather than a stale
installed binary. Everything else is carried verbatim. Older versioned fixture trees (v0.9.2–v0.9.5)
are immutable inputs and are never edited in place.
