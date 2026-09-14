// The gate.history.cleanup application operation. It composes the SAME native
// gate seam the commandless drive service composes (gatedrive.OpenStore over the
// repository's Git common directory + process.NewService over the resolved
// executable + gatedrive.NewSystemDriver), runs the shared manual legacy-history
// assessment (Driver.CleanupHistory), and maps its outcome into a bounded
// protocol-v1 document. The result carries ONLY id/class/reason per finding — no
// command, env, head oid, generation, argv, or credential ever crosses this seam.
package app

import (
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/process"
)

// OperationGateHistoryCleanup is the fixed protocol identifier for the manual
// legacy-history assessment operation.
const OperationGateHistoryCleanup = "gate.history.cleanup"

// GateHistoryCleanupRequest selects the manual assessment's scope. An empty
// DriveID scans the whole registry; a non-empty DriveID assesses exactly that one
// record (a malformed or traversal id is refused). DryRun previews without
// writing any abandoned marker.
type GateHistoryCleanupRequest struct {
	RepoDir string `json:"repo_dir"`
	DriveID string `json:"drive_id,omitempty"`
	DryRun  bool   `json:"dry_run,omitempty"`
}

// HistoryCleanupFinding is one drive's assessment, projected onto the bounded
// application shape. It carries ONLY the validated drive id (empty for an
// unrecognized registry entry), the class, and the bounded reason — never
// command, env, head oid, generation, or any credential.
type HistoryCleanupFinding struct {
	DriveID string `json:"drive_id,omitempty"`
	Class   string `json:"class"`
	Reason  string `json:"reason"`
}

// GateHistoryCleanupResult is the protocol document for gate.history.cleanup. On
// the applied path it mirrors the outcome's counts and findings; Reason is set
// only on a command failure (a bounded safe token, never record content). Because
// the findings carry only id/class/reason, no mapping here can leak a command,
// env, head oid, or generation.
type GateHistoryCleanupResult struct {
	Envelope
	DryRun      bool                    `json:"dry_run"`
	Checked     int                     `json:"checked"`
	Recovered   int                     `json:"recovered"`
	Recoverable int                     `json:"recoverable"`
	Retained    int                     `json:"retained"`
	Findings    []HistoryCleanupFinding `json:"findings"`
	Reason      string                  `json:"reason,omitempty"`
}

// GateHistoryCleanup runs the shared legacy assessment without starting any
// execution. gitCommonDir/exePath resolve exactly as the commandless drive
// service resolves them; no metadata preparation, remote fetch, or worktree
// existence is required.
func GateHistoryCleanup(gitCommonDir, exePath string, req GateHistoryCleanupRequest) GateHistoryCleanupResult {
	proc, err := process.NewService(exePath)
	if err != nil {
		res, reason := mapGateFailure(err)
		return GateHistoryCleanupResult{
			Envelope: NewEnvelope(OperationGateHistoryCleanup, res),
			DryRun:   req.DryRun,
			Findings: []HistoryCleanupFinding{},
			Reason:   reason,
		}
	}
	store := gatedrive.OpenStore(gitCommonDir)
	driver := gatedrive.NewSystemDriver(store, proc)
	outcome, err := driver.CleanupHistory(gatedrive.HistoryCleanupRequest{
		DriveID: req.DriveID,
		DryRun:  req.DryRun,
	})
	if err != nil {
		res, reason := mapDriveFailure(err)
		return GateHistoryCleanupResult{
			Envelope: NewEnvelope(OperationGateHistoryCleanup, res),
			DryRun:   req.DryRun,
			Findings: []HistoryCleanupFinding{},
			Reason:   reason,
		}
	}
	findings := make([]HistoryCleanupFinding, 0, len(outcome.Findings))
	for _, f := range outcome.Findings {
		findings = append(findings, HistoryCleanupFinding{
			DriveID: f.DriveID,
			Class:   f.Class,
			Reason:  f.Reason,
		})
	}
	return GateHistoryCleanupResult{
		Envelope:    NewEnvelope(OperationGateHistoryCleanup, ResultApplied),
		DryRun:      req.DryRun,
		Checked:     outcome.Checked,
		Recovered:   outcome.Recovered,
		Recoverable: outcome.Recoverable,
		Retained:    outcome.Retained,
		Findings:    findings,
	}
}

// HumanText renders the compact assessment summary. The applied path prints a
// counts header (the nonblocking count is derived from the findings, since the
// four recovery counters plus nonblocking partition them), one indented line per
// finding, and — when retained records remain — a plain not-complete line that
// never claims repository readiness. A non-applied path names the result and its
// bounded reason.
func (r GateHistoryCleanupResult) HumanText() string {
	if r.Result != ResultApplied {
		if r.Reason != "" {
			return "history cleanup — " + string(r.Result) + " (" + r.Reason + ")"
		}
		return "history cleanup — " + string(r.Result)
	}
	nonblocking := 0
	for _, f := range r.Findings {
		if f.Class == gatedrive.LegacyNonblocking {
			nonblocking++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "history cleanup — checked %d: %d nonblocking, %d recovered, %d recoverable, %d retained",
		r.Checked, nonblocking, r.Recovered, r.Recoverable, r.Retained)
	for _, f := range r.Findings {
		id := f.DriveID
		if id == "" {
			id = "(registry)"
		}
		fmt.Fprintf(&b, "\n  %s  %s  %s", id, f.Class, f.Reason)
	}
	if r.Retained > 0 {
		b.WriteString("\nretained records remain; recovery is not complete")
	}
	return b.String()
}
