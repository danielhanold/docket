// Command releasepkg builds one deterministic release-candidate bundle by
// wrapping internal/release.Package with a plain stdlib-flag surface. It is
// development and CI tooling — a second cmd/ like cmd/genassets, never a shipped
// product executable and never a public API. It accepts, never infers: the
// source root, safe version, full commit, and source epoch all arrive as
// explicit flags and are validated inside release.Package.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/danielhanold/docket/internal/release"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// requiredFlags is the exact set of flags every run must provide, in the order
// a usage error names them.
var requiredFlags = []string{"source", "version", "commit", "source-epoch", "out"}

// run parses args, fills release.Inputs, and delegates to release.Package.
// Exit codes: 2 for a usage error (missing/malformed flags), 1 for a packaging
// failure, 0 on success.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("releasepkg", flag.ContinueOnError)
	fs.SetOutput(stderr)
	source := fs.String("source", "", "absolute path to the source checkout to build")
	version := fs.String("version", "", "safe release version (vX.Y.Z[-prerelease])")
	commit := fs.String("commit", "", "full 40-character lowercase-hex source commit")
	epoch := fs.Int64("source-epoch", 0, "source epoch, decimal unix seconds")
	out := fs.String("out", "", "destination directory for the bundle")

	if err := fs.Parse(args); err != nil {
		// ContinueOnError already wrote the parse error and usage to stderr; a
		// malformed --source-epoch lands here as a usage error.
		return 2
	}

	// Name every missing required flag at once, rather than one build at a time
	// (learning validate-the-whole-input-set-first). fs.Visit reports only the
	// flags actually present on the command line.
	seen := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { seen[f.Name] = true })
	var missing []string
	for _, name := range requiredFlags {
		if !seen[name] {
			missing = append(missing, "--"+name)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(stderr, "releasepkg: missing required flags: %s\n", strings.Join(missing, ", "))
		return 2
	}

	in := release.Inputs{
		SourceRoot:  *source,
		Version:     *version,
		Commit:      *commit,
		SourceEpoch: *epoch,
		OutDir:      *out,
	}
	if err := release.Package(in, "go"); err != nil {
		fmt.Fprintf(stderr, "releasepkg: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "packaged %s -> %s\n", *version, *out)
	return 0
}
