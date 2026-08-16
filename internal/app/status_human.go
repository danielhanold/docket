package app

import (
	"fmt"
	"strconv"
	"strings"
)

// StatusResult is a fully-computed operation outcome the presenter renders, so
// it must satisfy OperationResult. HumanText below supplies the second method;
// Env comes from the embedded Envelope.
var _ OperationResult = StatusResult{}

// HumanText renders the deterministic, compact status report (spec §Human
// report), in fixed section order: (1) repository mode and short authoritative
// revisions; (2) complete and displayed counts; (3) the ordered ready queue;
// (4) one line per displayed active change; (5) health totals followed by the
// ordered error and warning findings. Every empty state — an empty ready queue,
// an empty displayed projection, and a healthy repository — is an explicit line
// rather than a missing section. Only branch names and revisions carry the
// repository's identity; no host-absolute path is printed. Matches
// ConfigInspectionResult.HumanText's strings.Builder / TrimRight register.
func (r StatusResult) HumanText() string {
	var b strings.Builder

	// 1. mode + short revisions. The metadata branch line is docket-mode only.
	fmt.Fprintf(&b, "mode: %s\n", r.Context.MetadataMode)
	fmt.Fprintf(&b, "default branch: %s @ %s\n", r.Context.DefaultBranch, shortRevision(r.Context.DefaultBranchRevision))
	fmt.Fprintf(&b, "integration branch: %s @ %s\n", r.Context.IntegrationBranch, shortRevision(r.Context.IntegrationRevision))
	if r.Context.MetadataBranch != "" {
		fmt.Fprintf(&b, "metadata branch: %s @ %s\n", r.Context.MetadataBranch, shortRevision(r.Context.MetadataRevision))
	}

	// Failure results carry no report body — surface the classification instead.
	if r.Reason != "" {
		fmt.Fprintf(&b, "\nreason: %s\n", r.Reason)
		if r.Message != "" {
			fmt.Fprintf(&b, "message: %s\n", r.Message)
		}
	}

	// 2. complete and displayed counts.
	fmt.Fprintf(&b, "\nchanges: %d total, %d active, %d displayed\n",
		r.Summary.TotalChanges, r.Summary.ActiveChanges, r.Summary.DisplayedChanges)
	fmt.Fprintf(&b, "records: %d adrs, %d learnings\n", r.Summary.ADRs, r.Summary.Learnings)

	// 3. ordered ready queue — an empty queue is an explicit line.
	if len(r.Ready) == 0 {
		b.WriteString("\nready queue: (empty)\n")
	} else {
		ids := make([]string, 0, len(r.Ready))
		for _, id := range r.Ready {
			ids = append(ids, strconv.Itoa(id))
		}
		fmt.Fprintf(&b, "\nready queue: %s\n", strings.Join(ids, ", "))
	}

	// 4. one line per displayed active change — an empty projection is explicit.
	if len(r.Changes) == 0 {
		b.WriteString("\ndisplayed changes: (none)\n")
	} else {
		b.WriteString("\ndisplayed changes:\n")
		for _, c := range r.Changes {
			fmt.Fprintf(&b, "  #%d %s — %s; unmet deps: %s; base: %s\n",
				c.ID, c.Title, c.Readiness, formatDeps(c.UnmetDeps), effectiveBase(c.EffectiveBase))
		}
	}

	// 5. health totals, then error findings then warning findings, each group in
	// landed report order. Counts come from the findings themselves so the header
	// can never disagree with the rows beneath it (the ConfigInspectionResult
	// blockerCount pattern). A repository with neither is an explicit ok line.
	errs, warns := countFindings(r.Findings)
	if errs == 0 && warns == 0 {
		b.WriteString("\nhealth: ok (0 errors, 0 warnings)\n")
	} else {
		fmt.Fprintf(&b, "\nhealth: %d %s, %d %s\n",
			errs, pluralize(errs, "error", "errors"),
			warns, pluralize(warns, "warning", "warnings"))
		for _, f := range r.Findings {
			if f.Severity == "error" {
				writeFinding(&b, f)
			}
		}
		for _, f := range r.Findings {
			if f.Severity == "warning" {
				writeFinding(&b, f)
			}
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// shortRevision truncates a full object id to the leading 12 characters, the
// stable display length the report uses. A shorter id is returned unchanged.
func shortRevision(rev string) string {
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}

// formatDeps renders the unmet-dependency ids, or "none" when there are none —
// an unmet dependency is a fact worth a visible word rather than a blank.
func formatDeps(deps []int) string {
	if len(deps) == 0 {
		return "none"
	}
	ids := make([]string, 0, len(deps))
	for _, d := range deps {
		ids = append(ids, strconv.Itoa(d))
	}
	return strings.Join(ids, ", ")
}

// effectiveBase spells the resolved stack base, or "(default)" when the change
// declares no stack parent and inherits the integration/default base.
func effectiveBase(base string) string {
	if base == "" {
		return "(default)"
	}
	return base
}

// countFindings tallies the error and warning severities the human report
// surfaces; other severities (e.g. notice) are not part of the health totals.
func countFindings(findings []StatusFinding) (errs, warns int) {
	for _, f := range findings {
		switch f.Severity {
		case "error":
			errs++
		case "warning":
			warns++
		}
	}
	return errs, warns
}

// writeFinding renders one finding row: the padded severity, its code, a
// locator when the finding names an entity or a path, and the explanatory
// message — mirroring ConfigInspectionResult's diagnostics rows.
func writeFinding(b *strings.Builder, f StatusFinding) {
	fmt.Fprintf(b, "  %-7s %s", f.Severity, f.Code)
	if loc := findingLocator(f); loc != "" {
		fmt.Fprintf(b, " %s", loc)
	}
	fmt.Fprintf(b, " — %s\n", f.Message)
}

// findingLocator names what a finding is about: an entity plus its identity and
// optional field, else a repo-relative path, else nothing.
func findingLocator(f StatusFinding) string {
	switch {
	case f.Entity != "" && f.Identity != "":
		loc := f.Entity + " " + f.Identity
		if f.Field != "" {
			loc += " (" + f.Field + ")"
		}
		return loc
	case f.Entity != "":
		return f.Entity
	case f.Path != "":
		return f.Path
	default:
		return ""
	}
}
