package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the read-only `context finalize` operation and the shared
// FinalizeDeps every terminal-half operation composes. `context finalize` pins
// the metadata corpus once, probes each finalize-population change's live pull
// request, and reports the authoritative finalize disposition of every
// candidate — the same deterministic queue domain.SelectFinalizeQueue derives —
// so the finalize workflow reasons from one internally consistent bundle. It
// decides no lifecycle policy of its own: ordering and skip reasons are the
// domain's; this layer probes the live facts, threads them in, and presents the
// result. It never mutates and never opens a transaction; an unresolved GitHub
// probe surfaces as unknown PR facts (never a clean absence), and a subsequent
// mutation runs its own preflight.

// OperationContextFinalize is the operation key `context finalize` records in
// its result envelope.
const OperationContextFinalize = "context.finalize"

// The stable machine reasons the finalize-context operation reports for its
// typed non-bundle outcomes on an explicit --id. Message text is explanatory
// and must not be parsed.
const (
	// ReasonFinalizeUnknownChange is returned when an explicit --id names no
	// record in the corpus.
	ReasonFinalizeUnknownChange = "unknown-change"
	// ReasonFinalizeAmbiguousID is returned when an explicit --id is claimed by
	// more than one record: the operation refuses to choose.
	ReasonFinalizeAmbiguousID = "ambiguous-change"
	// ReasonFinalizeMalformed is returned when the named change's identity is not
	// usable (a non-positive id or a slug outside the record-slug grammar).
	ReasonFinalizeMalformed = "malformed-record"
	// ReasonFinalizeNotFinalizable is returned when an explicit --id names a
	// change that is not in finalize's population — terminal, or carrying no PR
	// reference — so there is nothing to finalize.
	ReasonFinalizeNotFinalizable = "not-finalizable"
)

// FinalizeGitHub is the GitHub seam the terminal-half operations delegate their
// GitHub mechanics to. *githubcli.Client satisfies it; unit tests inject a
// recording fake. It names the exact set of probe/act calls finalize needs
// across its operations (Task 6-17), so later operations compose the same
// adapter rather than each holding a second client. The read-only context
// operation touches none of these directly — it reads live PR facts through
// FinalizePRProber — but the seam rides in FinalizeDeps so the mutating
// operations share one wiring.
type FinalizeGitHub interface {
	DiscoverRepository(ctx context.Context, dir string) (githubcli.Repository, error)
	ProbeMerged(ctx context.Context, repo githubcli.Repository, number int) (githubcli.MergeOutcome, githubcli.MergedFacts, error)
	FindOpenPullRequestsByHead(ctx context.Context, repo githubcli.Repository, headBranch string) ([]githubcli.PullRequest, error)
	RetargetPullRequest(ctx context.Context, repo githubcli.Repository, number int, expectedVersion, newBase string) (githubcli.RetargetOutcome, githubcli.PullRequest, error)
	EnsureComment(ctx context.Context, repo githubcli.Repository, number int, marker, body string) (githubcli.CommentOutcome, string, error)
	FindComment(ctx context.Context, repo githubcli.Repository, number int, marker string) (bool, string, error)
	MergePullRequest(ctx context.Context, repo githubcli.Repository, number int, expectedHead githubcli.ObjectRef, admin bool) (githubcli.MergeResult, error)
}

// FinalizeWorkspace is the workspace seam the terminal-half operations delegate
// their workspace mechanics to. *workspace.Service satisfies it; unit tests
// inject a fake. It names the read-only inspection, the rebase-receipt
// lifecycle, and the two publish primitives finalize composes.
type FinalizeWorkspace interface {
	Inspect(ctx context.Context, req workspace.InspectRequest) (workspace.Inspection, error)
	ReadRebaseReceipt(ctx context.Context, dir string) (workspace.RebaseReceipt, bool, error)
	WriteRebaseReceipt(ctx context.Context, dir string, r workspace.RebaseReceipt) error
	ClearRebaseReceipt(ctx context.Context, dir string) error
	PublishRewrite(ctx context.Context, req workspace.RewriteRequest) (workspace.RewriteOutcome, error)
	PublishHead(ctx context.Context, req workspace.PublishRequest) (workspace.PublishResult, error)
	// Cleanup is the landed, manifest-fact-driven, non-forcing removal of one
	// owned, ready, clean feature checkout. `finalize cleanup` (Task 14) drives it
	// as the workspace-removal leg of its ordered destructive suffix. The real
	// service satisfies it; unit tests inject a fake that faults the manifest/lock
	// probe to prove the leg is retained on an unprovable inspection.
	Cleanup(ctx context.Context, req workspace.CleanupRequest) (workspace.CleanupResult, error)
}

// FinalizeCleanupGit is the narrow Git seam `finalize cleanup` (Task 14) drives
// its ownership-proven branch deletion through: read-only tip/registration/
// ancestry/remote probes and the two checked ref-deletion primitives (Task 3).
// *gitcli.Client satisfies it; unit tests inject a fake that faults exactly one
// probe to prove the destructive leg fails closed (the resource is RETAINED,
// never destroyed, on any unknown — learning probe-error-is-not-clean-absence).
// It is a distinct seam from the concrete Planning.Client so a cleanup test can
// inject a probe error without a live-git condition; production wires it to the
// same client.
type FinalizeCleanupGit interface {
	ResolveRef(ctx context.Context, repo gitcli.Repository, ref gitcli.RefName) (gitcli.ObjectID, error)
	ProbeRemoteBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, ref gitcli.RefName) (gitcli.RemoteRef, error)
	ListWorktrees(ctx context.Context, repo gitcli.Repository) ([]gitcli.WorktreeInfo, error)
	IsAncestor(ctx context.Context, repo gitcli.Repository, ancestor, descendant gitcli.ObjectID) (bool, error)
	FetchBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, branch gitcli.RefName) (gitcli.Revision, error)
	DeleteLocalBranchChecked(ctx context.Context, repo gitcli.Repository, branch gitcli.RefName, expectedTip gitcli.ObjectID) error
	DeleteRemoteRefLease(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, ref gitcli.RefName, expectedTip gitcli.ObjectID) (gitcli.PushOutcome, error)
}

// FinalizePRProber reads one change's live pull-request facts as the domain
// finalize vocabulary. It is the read-only seam `context finalize` builds its
// facts map from; the production implementation (githubFinalizeProber) composes
// the existing githubcli probes, and unit tests inject a fake that scripts facts
// directly so the operation's delegation to domain.SelectFinalizeQueue is
// tested independently of the current probe's field coverage. A genuine probe
// error is returned as an error; the caller substitutes unknown facts and never
// a clean absence.
type FinalizePRProber interface {
	ProbePR(ctx context.Context, repoDir, prRef, headBranch string) (domain.PRFacts, error)
}

// FinalizeDeps is every seam the terminal-half operations compose. It layers the
// read-only planning seams (reader/engine/git client/clock) with the GitHub and
// workspace services and the PR-facts prober the finalize half needs. Tasks
// 7-17 reuse it; the read-only context operation touches only Planning.Reader
// and PRProber.
type FinalizeDeps struct {
	Planning  PlanningDeps
	GitHub    FinalizeGitHub
	Workspace FinalizeWorkspace
	PRProber  FinalizePRProber
	// Gate is the local-gate composition seam finalize rebase drives after a
	// completed rebase (Task 8): it launches the resolved suite in the feature
	// workspace, observes it to a terminal within the observation budget, and maps
	// the outcome. The read-only context operation and the retarget operation
	// never touch it, so it is nil in their wiring; the mutating rebase operation
	// requires it.
	Gate FinalizeGate
	// CleanupGit is the narrow branch-deletion Git seam `finalize cleanup` (Task
	// 14) drives its ownership-proven ref deletion through. It is nil in every
	// other operation's wiring; the cleanup operation falls back to
	// Planning.Client when it is nil, so production may leave it unset.
	CleanupGit FinalizeCleanupGit
}

// FinalizeContextRequest is the closed request. ID==0 applies the deterministic
// finalize selection policy over the whole population; a positive ID inspects
// that exact change (an attributed retry). Allowlist, when non-empty, bounds the
// selection membership without reordering survivors.
type FinalizeContextRequest struct {
	ID        int   `json:"id"`
	Allowlist []int `json:"allowlist"`
}

// FinalizePRReport is one change's live pull-request facts as the bundle carries
// them. Verdict is "probed" when the live facts were read, or "unknown" when the
// probe could not be resolved — never laundered into a clean absence. Ref is the
// canonical manifest PR reference the change records.
type FinalizePRReport struct {
	Ref          string `json:"ref"`
	Verdict      string `json:"verdict"` // "probed" | "unknown"
	Number       string `json:"number,omitempty"`
	Version      string `json:"version,omitempty"`
	State        string `json:"state,omitempty"`
	Draft        bool   `json:"draft,omitempty"`
	Approved     bool   `json:"approved,omitempty"`
	Mergeable    string `json:"mergeable,omitempty"`
	HeadOID      string `json:"head_oid,omitempty"`
	BaseRef      string `json:"base_ref,omitempty"`
	ChangedFiles int    `json:"changed_files,omitempty"`
	DiffLines    int    `json:"diff_lines,omitempty"`
	MergedAtUTC  string `json:"merged_at_utc,omitempty"`
	MergeCommit  string `json:"merge_commit,omitempty"`
}

// finalizeVerdictProbed and finalizeVerdictUnknown are the closed PR-facts
// verdict tokens.
const (
	finalizeVerdictProbed  = "probed"
	finalizeVerdictUnknown = "unknown"
)

// FinalizeRelation is one dependency of a candidate: its id and current
// lifecycle status, read from the snapshot graph.
type FinalizeRelation struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

// FinalizeDescendant is one transitive stack descendant of a candidate: its id,
// slug, current lifecycle status (from the stacked_on graph, never a rendered
// table), and the base its own PR targets (its stack destination) when that PR
// was probed.
type FinalizeDescendant struct {
	ID            int    `json:"id"`
	Slug          string `json:"slug"`
	Status        string `json:"status"`
	PRDestination string `json:"pr_destination,omitempty"`
}

// FinalizeCandidateReport is one change's authoritative finalize disposition:
// its identity and source version, resolved branch and effective base, live PR
// facts, dependency and stack relations, the set of open child PRs that must be
// retargeted before a stacked merge, and the typed candidate band or skip
// reason. OverrideNote is set when a skip reason (approval-required or
// finalize-blocked) is one an explicit --id may override at the mutation layer.
type FinalizeCandidateReport struct {
	ID            int                  `json:"id"`
	Slug          string               `json:"slug"`
	Path          string               `json:"path"`
	Version       string               `json:"version"`
	Status        string               `json:"status"`
	Branch        string               `json:"branch"`
	Band          string               `json:"band,omitempty"`
	SkipReason    string               `json:"skip_reason,omitempty"`
	OverrideNote  string               `json:"override_note,omitempty"`
	EffectiveBase ContextBase          `json:"effective_base"`
	PR            FinalizePRReport     `json:"pr"`
	Dependencies  []FinalizeRelation   `json:"dependencies"`
	Descendants   []FinalizeDescendant `json:"descendants"`
	OpenChildPRs  []int                `json:"open_child_prs"`
}

// FinalizePolicy is the repository-wide finalize configuration the bundle
// reports: repo mode, refs, and the resolved gate/suite/approval/reclaim policy.
type FinalizePolicy struct {
	RepoMode             string `json:"repo_mode"`
	IntegrationBranch    string `json:"integration_branch"`
	Remote               string `json:"remote"`
	Gate                 string `json:"gate"`
	TestCommand          string `json:"test_command,omitempty"`
	RequirePRApproval    bool   `json:"require_pr_approval"`
	ReclaimAuto          bool   `json:"reclaim_auto"`
	ReclaimLeaseTTLHours int    `json:"reclaim_lease_ttl_hours"`
}

// FinalizeContextResult is the protocol-v1 document `context finalize` returns.
// It carries the finalize policy and the surfaced candidate reports on a
// successful read, and a stable reason plus explanatory message on a typed
// explicit-id absence/refusal.
type FinalizeContextResult struct {
	Envelope
	Policy     FinalizePolicy            `json:"policy"`
	Candidates []FinalizeCandidateReport `json:"candidates"`
	Warnings   []string                  `json:"warnings"`
	Reason     string                    `json:"reason,omitempty"`
	Message    string                    `json:"message,omitempty"`
}

// HumanText renders the one-line human summary. It names counts and derived
// verdicts only — never an authored document body.
func (r FinalizeContextResult) HumanText() string {
	if r.Result == ResultApplied {
		first := ""
		if len(r.Candidates) > 0 {
			c := r.Candidates[0]
			disp := c.Band
			if disp == "" {
				disp = "skip:" + c.SkipReason
			}
			first = fmt.Sprintf("; first %04d %s (%s)", c.ID, c.Slug, disp)
		}
		return fmt.Sprintf("%s: %s — %d candidate(s)%s", r.Operation, r.Result, len(r.Candidates), first)
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newFinalizeContextResult stamps the envelope and normalizes the top-level
// collections so a nil never leaks into the protocol document.
func newFinalizeContextResult(result Result, reason, message string, policy FinalizePolicy, candidates []FinalizeCandidateReport, warnings []string) FinalizeContextResult {
	if candidates == nil {
		candidates = []FinalizeCandidateReport{}
	}
	if warnings == nil {
		warnings = []string{}
	}
	return FinalizeContextResult{
		Envelope:   NewEnvelope(OperationContextFinalize, result),
		Policy:     policy,
		Candidates: candidates,
		Warnings:   warnings,
		Reason:     reason,
		Message:    message,
	}
}

// ContextFinalize assembles the authoritative finalize-context bundle. It is
// read-only: one pin, one corpus read, one snapshot; live PR facts threaded from
// the prober; every ordering and skip decision delegated to the domain. It
// never opens a transaction.
func ContextFinalize(ctx context.Context, deps FinalizeDeps, repoDir string, req FinalizeContextRequest) FinalizeContextResult {
	reader := deps.Planning.Reader
	pin, err := reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newFinalizeContextResult(result, reason, err.Error(), FinalizePolicy{}, nil, nil)
	}
	eff := pin.Config.Effective
	policy := finalizePolicy(pin)

	blobs, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newFinalizeContextResult(result, reason, err.Error(), policy, nil, nil)
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		// A build error means the CALL was malformed — a contract violation.
		return newFinalizeContextResult(ResultInternalError, ReasonStatusInternalError, err.Error(), policy, nil, nil)
	}
	snap := build.Snapshot

	blobByPath := make(map[string]StatusBlob, len(blobs))
	for _, b := range blobs {
		blobByPath[b.Path] = b
	}

	// An explicit --id that names no usable, finalizable record is a typed
	// refusal that fabricates no candidate list. It also bounds the selection to
	// exactly that record so the report inspects it alone.
	selectIDs := req.Allowlist
	if req.ID > 0 {
		if refusal := finalizeExplicitGuard(snap, req.ID, policy); refusal != nil {
			return *refusal
		}
		selectIDs = []int{req.ID}
	}

	// Probe live PR facts for every change in finalize's population — non-terminal
	// and carrying a PR reference, bounded by the allowlist when one is given — so
	// the domain selector orders over authoritative facts. A probe error is
	// unknown facts, never a clean absence.
	warnings := []string{}
	allow := allowlistIDs(selectIDs)
	facts := make(map[domain.ChangeID]domain.PRFacts)
	probed := make(map[domain.ChangeID]bool)
	for _, c := range snap.Changes() {
		if len(allow) > 0 && !allow[c.ID()] {
			continue
		}
		if c.Status().Terminal() || !finalizeHasPRRef(c) {
			continue
		}
		f, unresolved := probeFinalizeFacts(ctx, deps.PRProber, repoDir, c)
		facts[c.ID()] = f
		probed[c.ID()] = !unresolved
		if unresolved {
			warnings = append(warnings, fmt.Sprintf("change %04d: pull-request facts could not be resolved; treated as unknown", int(c.ID())))
		}
	}

	queue := domain.SelectFinalizeQueue(snap, facts, finalizeBlockedMap(), allowlistChangeIDs(selectIDs))

	facts2 := domain.NewBranchFacts(nil)
	reports := make([]FinalizeCandidateReport, 0, len(queue))
	for _, cand := range queue {
		c, out := snap.Change(cand.ID)
		if out != domain.LookupFound {
			continue
		}
		reports = append(reports, buildCandidateReport(snap, c, cand, facts, probed, facts2, req.ID, blobByPath))
	}

	return newFinalizeContextResult(ResultApplied, "", "", policy, reports, warnings)
}

// finalizeExplicitGuard returns a typed refusal when an explicit --id cannot be
// inspected as a finalize candidate: absent, ambiguous, malformed, or outside
// the finalize population. It returns nil when the id names one usable,
// finalizable record — a skip-reasoned candidate (approval-required,
// finalize-blocked, and the rest) is NOT refused here, so an explicit id
// surfaces it with its reason for the mutation layer to override.
func finalizeExplicitGuard(snap domain.Snapshot, id int, policy FinalizePolicy) *FinalizeContextResult {
	c, out := snap.Change(domain.ChangeID(id))
	switch out {
	case domain.LookupAbsent:
		r := newFinalizeContextResult(ResultInvalidInput, ReasonFinalizeUnknownChange,
			fmt.Sprintf("no change %04d is present in the corpus", id), policy, nil, nil)
		return &r
	case domain.LookupAmbiguous:
		r := newFinalizeContextResult(ResultInvalidState, ReasonFinalizeAmbiguousID,
			fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id), policy, nil, nil)
		return &r
	}
	if c.ID() <= 0 || !domain.ValidSlugToken(c.Slug()) {
		r := newFinalizeContextResult(ResultInvalidState, ReasonFinalizeMalformed,
			fmt.Sprintf("change %04d has an unusable identity", id), policy, nil, nil)
		return &r
	}
	if c.Status().Terminal() || !finalizeHasPRRef(c) {
		r := newFinalizeContextResult(ResultInvalidState, ReasonFinalizeNotFinalizable,
			fmt.Sprintf("change %04d is not in finalize's population (terminal or without a PR reference)", id), policy, nil, nil)
		return &r
	}
	return nil
}

// buildCandidateReport assembles one candidate's report from the snapshot graph
// and the probed facts. explicitID is the request id (0 for the selection path);
// a skip reason an explicit id may override carries an override note.
func buildCandidateReport(snap domain.Snapshot, c domain.Change, cand domain.FinalizeCandidate, facts map[domain.ChangeID]domain.PRFacts, probed map[domain.ChangeID]bool, branchFacts domain.BranchFacts, explicitID int, blobByPath map[string]StatusBlob) FinalizeCandidateReport {
	rep := FinalizeCandidateReport{
		ID:            int(c.ID()),
		Slug:          c.Slug(),
		Path:          c.Path(),
		Version:       blobByPath[c.Path()].Version,
		Status:        c.RawStatus(),
		Branch:        domain.BranchForSlug(c.Slug()),
		Band:          cand.Band,
		SkipReason:    cand.SkipReason,
		EffectiveBase: contextBase(domain.ResolveEffectiveBase(snap, c, branchFacts)),
		PR:            finalizePRReport(c, facts[c.ID()], probed[c.ID()]),
		Dependencies:  finalizeDependencies(snap, c),
		Descendants:   finalizeDescendants(snap, c, facts),
		OpenChildPRs:  finalizeOpenChildPRs(snap, c, facts),
	}
	if explicitID == int(c.ID()) && overridableSkip(cand.SkipReason) {
		rep.OverrideNote = "explicit --id overrides skip reason " + cand.SkipReason
	}
	return rep
}

// overridableSkip reports whether a skip reason is one an explicit --id may
// override at the mutation layer (Task 10 merge policy). Approval and a finalize
// block are human-overridable; every other skip reflects a state the merge path
// cannot be authorized past.
func overridableSkip(reason string) bool {
	return reason == "approval-required" || reason == "finalize-blocked"
}

// finalizePRReport renders the live PR facts. An unresolved probe reports the
// unknown verdict carrying only the canonical reference, never a fabricated
// clean state.
func finalizePRReport(c domain.Change, f domain.PRFacts, wasProbed bool) FinalizePRReport {
	ref := c.PR().Value
	if !wasProbed {
		return FinalizePRReport{Ref: ref, Verdict: finalizeVerdictUnknown, Number: f.Number, State: "unknown"}
	}
	return FinalizePRReport{
		Ref:          ref,
		Verdict:      finalizeVerdictProbed,
		Number:       f.Number,
		Version:      f.Version,
		State:        f.State,
		Draft:        f.Draft,
		Approved:     f.Approved,
		Mergeable:    f.Mergeable,
		HeadOID:      f.HeadOID,
		BaseRef:      f.BaseRef,
		ChangedFiles: f.ChangedFiles,
		DiffLines:    f.DiffLines,
		MergedAtUTC:  f.MergedAtUTC,
		MergeCommit:  f.MergeCommit,
	}
}

// finalizeDependencies reads the candidate's dependency relations from the
// snapshot graph, ascending by id, resolving each to its current lifecycle.
func finalizeDependencies(snap domain.Snapshot, c domain.Change) []FinalizeRelation {
	out := make([]FinalizeRelation, 0)
	for _, dep := range c.DependsOn() {
		other, lookup := snap.Change(dep)
		if lookup != domain.LookupFound {
			continue
		}
		out = append(out, FinalizeRelation{ID: int(other.ID()), Status: other.RawStatus()})
	}
	return out
}

// finalizeDescendants reads the candidate's transitive stack descendants from
// the stacked_on graph (parent-first, never a rendered table), reporting each
// descendant's lifecycle and — when its PR was probed — the base its PR targets.
func finalizeDescendants(snap domain.Snapshot, c domain.Change, facts map[domain.ChangeID]domain.PRFacts) []FinalizeDescendant {
	out := make([]FinalizeDescendant, 0)
	for _, id := range domain.StackDescendantsParentFirst(snap, c.ID()) {
		d, lookup := snap.Change(id)
		if lookup != domain.LookupFound {
			continue
		}
		out = append(out, FinalizeDescendant{
			ID:            int(d.ID()),
			Slug:          d.Slug(),
			Status:        d.RawStatus(),
			PRDestination: facts[id].BaseRef,
		})
	}
	return out
}

// finalizeOpenChildPRs is the set of direct stack children whose live PR is
// open — the exact set a stacked merge of the candidate must first retarget. It
// reads the children from the graph and their open-ness from the probed facts.
func finalizeOpenChildPRs(snap domain.Snapshot, c domain.Change, facts map[domain.ChangeID]domain.PRFacts) []int {
	out := make([]int, 0)
	for _, id := range domain.StackChildren(snap, c.ID()) {
		if facts[id].State == "open" {
			out = append(out, int(id))
		}
	}
	return out
}

// probeFinalizeFacts reads one change's live PR facts through the prober. A probe
// error is folded into unknown facts (carrying only the canonical number when it
// parses) and reported as unresolved, so the domain selector reads pr-unknown
// rather than a clean absence.
func probeFinalizeFacts(ctx context.Context, prober FinalizePRProber, repoDir string, c domain.Change) (domain.PRFacts, bool) {
	ref := c.PR().Value
	head := domain.BranchForSlug(c.Slug())
	f, err := prober.ProbePR(ctx, repoDir, ref, head)
	if err != nil {
		return domain.PRFacts{Number: prNumberToken(ref), State: "unknown"}, true
	}
	return f, false
}

// finalizePolicy reads the resolved repository-wide finalize policy from the pin.
func finalizePolicy(pin StatusPin) FinalizePolicy {
	eff := pin.Config.Effective
	return FinalizePolicy{
		RepoMode:             pin.Mode,
		IntegrationBranch:    pin.IntegrationBranch,
		Remote:               string(originRemote),
		Gate:                 eff.Finalize.Gate.Value,
		TestCommand:          eff.Finalize.TestCommand.Value,
		RequirePRApproval:    eff.Finalize.RequirePRApproval.Value,
		ReclaimAuto:          eff.Reclaim.Auto.Value,
		ReclaimLeaseTTLHours: eff.Reclaim.LeaseTTL.Value,
	}
}

// finalizeBlockedMap is the finalize-blocked marker set the domain selector
// consults. Reading durable "## Finalize blocked" markers is Task 11's job;
// until then no change is marked blocked, so this is empty. It is a named seam so
// a later task wires marker reading in one place.
func finalizeBlockedMap() map[domain.ChangeID]bool { return map[domain.ChangeID]bool{} }

// finalizeHasPRRef reports whether c carries a usable PR reference — the
// manifest signal that a change is in finalize's population.
func finalizeHasPRRef(c domain.Change) bool {
	pr := c.PR()
	return pr.State == domain.FieldPresent && pr.Value != ""
}

// allowlistIDs builds a membership set of change ids from a request allowlist,
// returning nil for an empty list so "no allowlist" reads as "match everything".
func allowlistIDs(ids []int) map[domain.ChangeID]bool {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[domain.ChangeID]bool, len(ids))
	for _, id := range ids {
		set[domain.ChangeID(id)] = true
	}
	return set
}

// allowlistChangeIDs converts a request allowlist to the domain id slice the
// selector bounds membership by.
func allowlistChangeIDs(ids []int) []domain.ChangeID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]domain.ChangeID, 0, len(ids))
	for _, id := range ids {
		out = append(out, domain.ChangeID(id))
	}
	return out
}

// prNumberToken extracts the canonical PR number from a PR reference in either
// accepted form (see parsePRRef), returning "" when the reference has no
// parseable positive number. It never fabricates a number.
func prNumberToken(ref string) string {
	n, ok := parsePRRef(ref)
	if !ok {
		return ""
	}
	return strconv.Itoa(n)
}

// githubFinalizeProber is the production FinalizePRProber. It composes the
// existing githubcli probes: the authoritative merged reprobe first (a merged PR
// carries full facts and is the finalize recovery band), then the open-PR probe
// by feature head for a non-merged PR.
//
// Field coverage: the current githubcli probes do not carry a PR's review
// decision, mergeability, or diff size for an OPEN pull request, so those fields
// (Approved, Mergeable, ChangedFiles, DiffLines) stay zero for an open PR — a
// conservative reading (an open PR bands as UNKNOWN mergeability and unapproved)
// that never over-permits. A later task enriching githubcli with a full open-PR
// facts view (reviewDecision, mergeStateStatus, changedFiles/additions) slots in
// here without changing the operation. The merged path is fully faithful.
type githubFinalizeProber struct {
	gh FinalizeGitHub
}

// ProbePR reads one change's live PR facts. It resolves the repository from
// repoDir, reprobes the exact PR number for a merged outcome, and otherwise
// reads the open PR by feature head. A repository-resolution or probe failure is
// a returned error; the caller substitutes unknown facts.
func (p *githubFinalizeProber) ProbePR(ctx context.Context, repoDir, prRef, headBranch string) (domain.PRFacts, error) {
	number, ok := parsePRNumber(prRef)
	if !ok {
		return domain.PRFacts{}, fmt.Errorf("pull-request reference %q carries no parseable number", prRef)
	}
	repo, err := p.gh.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return domain.PRFacts{}, err
	}
	outcome, mf, err := p.gh.ProbeMerged(ctx, repo, number)
	if err != nil {
		return domain.PRFacts{}, err
	}
	if outcome == githubcli.MergeMerged || outcome == githubcli.MergeAlreadyMerged {
		return domain.PRFacts{
			Number:      strconv.Itoa(number),
			Version:     mf.Version,
			State:       "merged",
			HeadOID:     mf.HeadOID,
			BaseRef:     mf.BaseRef,
			MergedAtUTC: mf.MergedAtUTC,
			MergeCommit: mf.MergeCommit,
		}, nil
	}
	prs, err := p.gh.FindOpenPullRequestsByHead(ctx, repo, headBranch)
	if err != nil {
		return domain.PRFacts{}, err
	}
	for _, pr := range prs {
		if pr.Number != number {
			continue
		}
		return domain.PRFacts{
			Number:  strconv.Itoa(number),
			Version: pr.Version,
			State:   string(pr.State),
			Draft:   pr.Draft,
			HeadOID: pr.HeadCommit,
			BaseRef: pr.BaseBranch,
		}, nil
	}
	// Cleanly observed as not merged and not open for its head: closed unmerged.
	return domain.PRFacts{Number: strconv.Itoa(number), State: "closed"}, nil
}

// parsePRNumber extracts the positive PR number from a canonical reference in
// either accepted form (see parsePRRef). It returns false for a reference with
// no parseable positive number.
func parsePRNumber(ref string) (int, bool) { return parsePRRef(ref) }

// parsePRRef is the single source of truth for reading a PR number out of a
// pr: reference. It accepts both canonical forms:
//
//   - the full GitHub URL — ".../pull/N", tolerating a trailing slash, a
//     "?query", a "#fragment", or a deeper sub-page (".../pull/N/files"),
//     because the number immediately after "/pull/" is unambiguous in every
//     one of those shapes;
//   - the "owner/repo#N" shorthand — the integer after the last "#".
//
// The "/pull/" check runs before the "#" fallback so a URL fragment is never
// mistaken for the number. Both forms require a positive integer; anything
// else — a non-numeric segment, a missing number, zero or negative — returns
// (0, false). Both parsePRNumber and prNumberToken delegate here so the two
// can never diverge on which forms they accept.
func parsePRRef(ref string) (int, bool) {
	if i := strings.Index(ref, "/pull/"); i >= 0 {
		seg := ref[i+len("/pull/"):]
		if j := strings.IndexAny(seg, "/?#"); j >= 0 {
			seg = seg[:j]
		}
		return parsePositiveInt(seg)
	}
	i := strings.LastIndex(ref, "#")
	if i < 0 || i+1 >= len(ref) {
		return 0, false
	}
	return parsePositiveInt(ref[i+1:])
}

// parsePositiveInt reads a strictly positive base-10 integer from seg,
// returning (0, false) unless every character is a digit and the value is > 0.
// The all-digit guard rejects a signed segment ("+42", "-1") identically in
// both parsePRRef branches, so a leading sign never slips through Atoi.
func parsePositiveInt(seg string) (int, bool) {
	if seg == "" {
		return 0, false
	}
	for _, r := range seg {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(seg)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// NewGitHubFinalizeProber builds the production PR-facts prober over a GitHub
// seam. The CLI wires it into FinalizeDeps.
func NewGitHubFinalizeProber(gh FinalizeGitHub) FinalizePRProber {
	return &githubFinalizeProber{gh: gh}
}

// Compile-time seam assertions: the production clients satisfy the finalize
// seams, so FinalizeDeps can carry the real wiring.
var (
	_ FinalizeGitHub     = (*githubcli.Client)(nil)
	_ FinalizeWorkspace  = (*workspace.Service)(nil)
	_ FinalizePRProber   = (*githubFinalizeProber)(nil)
	_ FinalizeCleanupGit = (*gitcli.Client)(nil)
)
