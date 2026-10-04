// Package bashupgrade holds the temporary proof that docs/release/upgrading-from-bash.md
// works: integration tests restore saved Bash docket v0.9.2/v0.9.3 installs from
// testdata/bash-upgrade/ and run the guide's own marked steps against the binary
// built from this checkout. The package has no non-test code beyond this file and is
// deleted, with testdata/bash-upgrade/, when stable v1.0.0 ships.
package bashupgrade
