// Package cli is Docket's inward-facing Cobra adapter: command tree,
// output-mode bootstrap, presenters, and exit mapping. It is not a supported
// Go API.
package cli

// DetectJSONMode is the output-transport fallback, not a second argument
// parser and not the mode input on a clean parse — Run reads the Cobra-bound
// --json flag whenever pflag managed to parse it. Output mode must still be
// known when ordinary parsing stops before Cobra reaches --json (e.g. `docket
// version --bogus --json`), and that case alone is what this deliberately
// narrow scan over raw arguments answers. Its bounded grammar:
//   - it recognizes only --json, --json=true, and --json=false;
//   - the last recognized value before a standalone -- selects the mode;
//   - it stops at the first standalone --;
//   - it neither validates, removes, reorders, nor interprets any other
//     argument. Cobra still performs all command and flag validation.
func DetectJSONMode(args []string) bool {
	mode := false
	for _, a := range args {
		switch a {
		case "--":
			return mode
		case "--json", "--json=true":
			mode = true
		case "--json=false":
			mode = false
		}
	}
	return mode
}
