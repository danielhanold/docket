//go:build integration

package bashupgrade

import (
	"reflect"
	"strings"
	"testing"
)

// ownershipRefusals are the repository-setup finding codes that mean the binary
// refused (or could not prove) docket ownership of the Bash-made metadata branch.
// Any of them on a saved case is a binary defect per the spec's "When the binary is
// wrong" rule, never a guide step.
var ownershipRefusals = []string{
	"metadata-root-foreign", "metadata-root-unresolved", "metadata-ownership-unverified",
	"metadata-presence-unknown", "docket-dir-foreign",
}

func TestIntegrationBashUpgradeOwnership(t *testing.T) {
	for _, tag := range savedTags(t) {
		t.Run(tag, func(t *testing.T) {
			c := restoreCase(t, tag)
			if got, want := listRecords(t, c), readRecords(t, tag); !reflect.DeepEqual(got, want) {
				t.Fatalf("records.txt disagrees with the saved bundle\n got %v\nwant %v", got, want)
			}
			for _, args := range [][]string{{"repository", "prepare", "--json"}, {"repository", "check", "--json"}} {
				r := c.run(t, c.Clone, c.Docket, args...)
				if r.Stdout == "" {
					t.Fatalf("%s produced no JSON (code %d); stderr:\n%s", strings.Join(args, " "), r.Code, r.Stderr)
				}
				for _, f := range findingCodes(t, r.Stdout) {
					for _, bad := range ownershipRefusals {
						if f.Code == bad {
							t.Fatalf("BLOCKED: %s refused the Bash-made docket branch (%s)\nstdout:\n%s\nstderr:\n%s",
								strings.Join(args, " "), f.Code, r.Stdout, r.Stderr)
						}
					}
				}
				// prepare must adopt the Bash clone (applied or no-op); check exits
				// non-zero whenever it reports findings, which the guide handles later.
				if args[1] == "prepare" && r.Code != 0 {
					t.Fatalf("repository prepare did not adopt the restored Bash clone (code %d)\nstdout:\n%s\nstderr:\n%s", r.Code, r.Stdout, r.Stderr)
				}
				t.Logf("%s exit %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), r.Code, r.Stdout, r.Stderr)
			}
		})
	}
}
