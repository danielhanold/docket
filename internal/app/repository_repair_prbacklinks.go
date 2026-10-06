package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
)

// repository_repair_prbacklinks.go — `docket repository repair --pr-backlinks`:
// the one-time, human-confirmed repair of merged PRs whose docket:backlink block
// still names a change path that no longer exists (the active path of a change
// that has since been archived). It reads the pinned metadata corpus, reads the
// done changes' PR bodies in batches (≤25 per gh process), previews each PR with
// its current and corrected link, and — only under --yes, pinned to the previewed
// metadata revision — edits each one through the shared applyPRBacklinkRepoint,
// one PR at a time. A PR that cannot be read or edited is reported and skipped;
// it never aborts the batch. A second run finds nothing.

// RepairGitHub is the GitHub seam this repair reads and edits merged PR bodies
// through. *githubcli.Client satisfies it.
type RepairGitHub interface {
	DiscoverRepository(ctx context.Context, dir string) (githubcli.Repository, error)
	ViewPullRequestsBatch(ctx context.Context, repo githubcli.Repository, numbers []int) (map[int]githubcli.BatchPRResult, error)
	PRBodyEditor
}

var _ RepairGitHub = (*githubcli.Client)(nil)

// PRBacklinkRepair is one PR's row in the preview or the applied report. Current
// and Corrected are the backlink block's generated interior line (never authored
// prose).
type PRBacklinkRepair struct {
	ID        int    `json:"id"`
	PR        int    `json:"pr"`
	Current   string `json:"current,omitempty"`
	Corrected string `json:"corrected,omitempty"`
	Outcome   string `json:"outcome"`
	Message   string `json:"message,omitempty"`
}

// The closed per-PR outcomes.
const (
	PRBacklinkRepairPlanned    = "planned"
	PRBacklinkRepairRepointed  = "repointed"
	PRBacklinkRepairContended  = "contended"
	PRBacklinkRepairUnreadable = "unreadable"
	PRBacklinkRepairFailed     = "failed"
)

// prRepairCandidate is one done change with a PR and the archive interior its
// PR's block must carry.
type prRepairCandidate struct {
	id       int
	pr       int
	path     string
	interior string
}

// prRepairPlanned is a planned row plus the bytes and revision the edit needs
// (kept out of the published row: body bytes are never reported).
type prRepairPlanned struct {
	row      PRBacklinkRepair
	body     string
	revision string
	path     string
	interior string
}

func runPRBacklinkRepair(ctx context.Context, d SetupDeps, o RepairOptions) RepositoryRepairResult {
	sc, metadataTip, refusal := repairPreflight(ctx, d)
	if refusal != nil {
		return *refusal
	}
	if d.GitHub == nil {
		return repairInternalFailure(reposetup.StateHealthy, "wiring the GitHub client",
			errors.New("no GitHub client is wired for --pr-backlinks"))
	}
	corpus, err := readCheckCorpus(ctx, d.Git, sc)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "reading the metadata corpus", err)
	}
	snap, ok := buildCorpusSnapshot(sc.cfg, corpus.records)
	if !ok {
		return repairInternalFailure(reposetup.StateHealthy, "building the metadata snapshot",
			errors.New("the metadata corpus could not be built into a snapshot"))
	}
	cands, err := prRepairCandidates(snap, corpus.link)
	if err != nil {
		return repairInternalFailure(reposetup.StateHealthy, "rendering the archived backlinks", err)
	}
	if len(cands) == 0 {
		return prRepairNoOp(metadataTip, nil)
	}
	repo, err := d.GitHub.DiscoverRepository(ctx, sc.repo.PrimaryWorktree)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "resolving the GitHub repository", err)
	}
	planned, skipped := planPRRepairs(ctx, d.GitHub, repo, cands)
	if len(planned) == 0 {
		return prRepairNoOp(metadataTip, skipped)
	}
	if !o.Authorized {
		return prRepairConfirmationRequired(metadataTip, planned, skipped)
	}
	if migrateSourceMoved(o.ExpectedSource, metadataTip) {
		return repairContended(metadataTip, o.ExpectedSource)
	}
	rows := append([]PRBacklinkRepair(nil), skipped...)
	repointed, failed := 0, 0
	for _, p := range planned {
		r := applyPRBacklinkRepoint(ctx, d.GitHub, repo, p.row.PR, p.body, p.revision, p.path, p.interior)
		row := p.row
		switch r.outcome {
		case prBacklinkRepointed, prBacklinkAlready:
			row.Outcome = PRBacklinkRepairRepointed
			repointed++
		case prBacklinkContended:
			row.Outcome, row.Message = PRBacklinkRepairContended, r.detail
			failed++
		default:
			row.Outcome, row.Message = PRBacklinkRepairFailed, r.detail
			failed++
		}
		rows = append(rows, row)
	}
	return prRepairApplied(metadataTip, rows, repointed, failed)
}

// prRepairCandidates lists every done change carrying a PR reference, sorted by
// id, with the archive interior its PR's block must carry.
func prRepairCandidates(snap domain.Snapshot, link render.LinkContext) ([]prRepairCandidate, error) {
	var out []prRepairCandidate
	for _, c := range snap.Changes() {
		if c.Status() != domain.StatusDone || !finalizeHasPRRef(c) {
			continue
		}
		n, ok := parsePRNumber(c.PR().Value)
		if !ok {
			continue
		}
		block, err := render.BacklinkContent(c, link)
		if err != nil {
			return nil, fmt.Errorf("change %04d: %w", int(c.ID()), err)
		}
		out = append(out, prRepairCandidate{id: int(c.ID()), pr: n, path: c.Path(), interior: backlinkInterior(block)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out, nil
}

// planPRRepairs reads the candidates' PR bodies in ≤25-number batches and keeps
// the ones whose block does not name the archive path. A failed batch, an
// unresolved slot, or malformed markers is an `unreadable` row, never a silent drop.
func planPRRepairs(ctx context.Context, gh RepairGitHub, repo githubcli.Repository, cands []prRepairCandidate) ([]prRepairPlanned, []PRBacklinkRepair) {
	byPR := map[int][]prRepairCandidate{}
	var numbers []int
	for _, c := range cands {
		byPR[c.pr] = append(byPR[c.pr], c)
		numbers = append(numbers, c.pr)
	}
	read := map[int]githubcli.BatchPRResult{}
	failMsg := map[int]string{}
	for _, chunk := range sweepChunkInts(sweepDedupeSortAsc(numbers), sweepPRBatchCap) {
		res, err := gh.ViewPullRequestsBatch(ctx, repo, chunk)
		for _, n := range chunk {
			switch {
			case err != nil:
				failMsg[n] = "the pull-request batch could not be read: " + err.Error()
			case !res[n].Found:
				failMsg[n] = "the pull request could not be resolved"
			default:
				read[n] = res[n]
			}
		}
	}
	var planned []prRepairPlanned
	var skipped []PRBacklinkRepair
	for _, c := range cands {
		br, ok := read[c.pr]
		if !ok {
			skipped = append(skipped, PRBacklinkRepair{ID: c.id, PR: c.pr, Outcome: PRBacklinkRepairUnreadable, Message: failMsg[c.pr]})
			continue
		}
		_, current, present, needs, err := planPRBacklinkRepoint([]byte(br.PR.Body), c.path, c.interior)
		if err != nil {
			skipped = append(skipped, PRBacklinkRepair{ID: c.id, PR: c.pr, Outcome: PRBacklinkRepairUnreadable,
				Message: "the pull-request body carries a malformed docket:backlink block"})
			continue
		}
		if !present || !needs {
			continue
		}
		planned = append(planned, prRepairPlanned{
			row:      PRBacklinkRepair{ID: c.id, PR: c.pr, Current: current, Corrected: c.interior, Outcome: PRBacklinkRepairPlanned},
			body:     br.PR.Body,
			revision: br.PR.Revision,
			path:     c.path,
			interior: c.interior,
		})
	}
	return planned, skipped
}

func prRepairRows(planned []prRepairPlanned, skipped []PRBacklinkRepair) []PRBacklinkRepair {
	rows := make([]PRBacklinkRepair, 0, len(planned)+len(skipped))
	for _, p := range planned {
		rows = append(rows, p.row)
	}
	return append(rows, skipped...)
}

func writePRRepairRows(b *strings.Builder, rows []PRBacklinkRepair) {
	for _, r := range rows {
		fmt.Fprintf(b, "  PR #%d (change %04d): %s\n", r.PR, r.ID, r.Outcome)
		if r.Current != "" {
			fmt.Fprintf(b, "    current:   %s\n", r.Current)
		}
		if r.Corrected != "" {
			fmt.Fprintf(b, "    corrected: %s\n", r.Corrected)
		}
		if r.Message != "" {
			fmt.Fprintf(b, "    %s\n", r.Message)
		}
	}
}

func prRepairConfirmationRequired(metadataTip string, planned []prRepairPlanned, skipped []PRBacklinkRepair) RepositoryRepairResult {
	rows := prRepairRows(planned, skipped)
	out := newRepairResult(ResultInvalidState, RepositoryRepairResult{
		RepositoryState: repairConfirmationRequiredState,
		SourceRevision:  metadataTip,
		PRBacklinks:     rows,
	})
	var b strings.Builder
	fmt.Fprintf(&b, "docket repository repair --pr-backlinks — preview (metadata %s)\n", metadataTip)
	writePRRepairRows(&b, rows)
	fmt.Fprintf(&b, "confirmation required: re-run with --yes to repoint these %d pull-request backlink(s)", len(planned))
	out.human = b.String()
	return out
}

func prRepairNoOp(metadataTip string, skipped []PRBacklinkRepair) RepositoryRepairResult {
	out := newRepairResult(ResultNoOp, RepositoryRepairResult{
		RepositoryState: string(reposetup.StateHealthy),
		SourceRevision:  metadataTip,
		PRBacklinks:     skipped,
	})
	var b strings.Builder
	fmt.Fprintf(&b, "repository repair --pr-backlinks: no pull-request backlink to repoint at metadata %s\n", metadataTip)
	writePRRepairRows(&b, skipped)
	out.human = strings.TrimRight(b.String(), "\n")
	return out
}

func prRepairApplied(metadataTip string, rows []PRBacklinkRepair, repointed, failed int) RepositoryRepairResult {
	result := ResultApplied
	if repointed == 0 && failed > 0 {
		result = ResultExternalFailed
	}
	out := newRepairResult(result, RepositoryRepairResult{
		RepositoryState: string(reposetup.StateHealthy),
		SourceRevision:  metadataTip,
		PRBacklinks:     rows,
	})
	var b strings.Builder
	fmt.Fprintf(&b, "repository repair --pr-backlinks: %d repointed, %d not repointed\n", repointed, failed)
	writePRRepairRows(&b, rows)
	out.human = strings.TrimRight(b.String(), "\n")
	return out
}
