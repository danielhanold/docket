package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the `maintenance sweep` operation: the batch driver that reclaims
// docket's terminal half over one pinned inventory. `docket status` stays
// read-only; every mutation this file dispatches goes through the same verified
// closeout, cleanup, and reclaim operations a human invokes one at a time
// (Tasks 12-14). The sweep composes them — it invents no new lifecycle policy,
// merges no open PR, overrides no approval, retargets no child, repairs no code,
// and edits no authored artifact. It pins one inventory to decide the worklist,
// processes items in a deterministic order, and reloads fresh authority before
// EVERY mutation (the dispatched operation reloads again and acts atomically).
// Per-item failures never stop independent items, and a destructive suffix for
// one item never runs after an unknown prerequisite for that item.

// OperationMaintenanceSweep is the operation key `maintenance sweep` records in
// its result envelope.
const OperationMaintenanceSweep = "maintenance.sweep"

// sweepBandMergedRecovery mirrors the domain finalize band string a
// FinalizeCandidate carries when its live PR is already merged and the change
// needs closeout. The domain constant is unexported; this app-layer copy is the
// same literal (the app already keys on PRFacts.State literals like "open").
const sweepBandMergedRecovery = "merged-recovery"

// The sweep item kinds. Each names which verified operation the item drives.
const (
	sweepKindCloseout = "closeout"
	sweepKindCleanup  = "cleanup"
	sweepKindReclaim  = "reclaim"
)

// SweepScope is the closed maintenance-sweep scope vocabulary (change 0389).
// full is the whole worklist — today's behavior, the default when the flag is
// omitted. implementation is the implementation-startup preflight: current
// merged-work closeouts (with their safe cleanup suffixes) and reclaim gating,
// with independent cleanup retries for records that were ALREADY terminal at the
// pinned inventory deferred to explicit full maintenance. The CLI resolves the
// scope once; the app layer never re-derives it from anything else.
type SweepScope string

const (
	SweepScopeFull           SweepScope = "full"
	SweepScopeImplementation SweepScope = "implementation"
)

// The closed vocabulary of per-item sweep dispositions. applied/noop/contended/
// blocked/unknown/failed map one dispatched operation's protocol result; skipped
// is the sweep's own pre-dispatch decision (policy declined, item vanished on
// reload, or a destructive suffix withheld after a non-successful prerequisite).
// Every item is reported with exactly one of these — never a collapsed boolean.
const (
	SweepDispApplied   = "applied"
	SweepDispNoOp      = "noop"
	SweepDispContended = "contended"
	SweepDispBlocked   = "blocked"
	SweepDispUnknown   = "unknown"
	SweepDispFailed    = "failed"
	SweepDispSkipped   = "skipped"
)

// The stable reasons the sweep reports for a skipped item or a whole-sweep
// refusal. Message text is explanatory and must not be parsed.
const (
	// ReasonSweepReclaimAutoDisabled: a reclaim-eligible record was left alone
	// because reclaim.auto is false; explicit `change reclaim` still applies.
	ReasonSweepReclaimAutoDisabled = "reclaim-auto-disabled"
	// ReasonSweepPrerequisiteUnresolved: a destructive suffix was withheld
	// because its prerequisite operation did not succeed (an unknown, contended,
	// blocked, or failed closeout). The resource is untouched.
	ReasonSweepPrerequisiteUnresolved = "prerequisite-unresolved"
	// ReasonSweepItemVanished: an item present in the pinned inventory was absent
	// or ambiguous on the fresh pre-mutation reload; the sweep dispatched nothing.
	ReasonSweepItemVanished = "item-vanished"
	// ReasonSweepReloadFailed: the fresh pre-mutation reload could not be read;
	// the sweep dispatched no mutation for this item and moved on.
	ReasonSweepReloadFailed = "reload-failed"
	// ReasonSweepReclaimVersionMissing: the reloaded record carried no usable
	// blob version to pin the exact-version reclaim; nothing was dispatched.
	ReasonSweepReclaimVersionMissing = "reclaim-version-missing"
	// ReasonSweepScopeInvalid: the typed scope was outside the closed
	// vocabulary; the sweep read nothing and dispatched nothing.
	ReasonSweepScopeInvalid = "sweep-scope-invalid"
)

// sweepOps is the injection seam for the three verified operations the sweep
// composes. Production wires each to the real Task 12-14 entry point over the
// live seams; unit tests inject recording fakes so the orchestration — order,
// per-item isolation, the unknown-prerequisite rule, reclaim gating, and the
// fresh reload before every mutation — is proved without a real repository.
type sweepOps struct {
	closeout func(ctx context.Context, id int) CloseoutResult
	cleanup  func(ctx context.Context, id int) CleanupOpResult
	reclaim  func(ctx context.Context, id int, version string) ChangeReclaimResult
	// probeFacts reads the finalize population's live PR facts for the pinned
	// snapshot in one batched pass (replacing the old per-change probe), returning
	// the facts map the domain selector bands over plus one finding per failed
	// batch (silent omission is not success). Production wires it to
	// sweepSelectPRFacts over deps.PRBatch; tests inject the seam directly.
	probeFacts func(ctx context.Context, snap domain.Snapshot) (map[domain.ChangeID]domain.PRFacts, []StatusFinding)
}

// MaintenanceEntry is one item's structured outcome. Disposition is a closed
// sweep token; Operation names the op key that produced it (empty for a
// pre-dispatch skip). It leaks no authored artifact bytes.
type MaintenanceEntry struct {
	ID          int    `json:"id"`
	Kind        string `json:"kind"`
	Disposition string `json:"disposition"`
	Operation   string `json:"operation,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Message     string `json:"message,omitempty"`
	CarriedIDs  []int  `json:"carried_ids,omitempty"`
}

// MaintenanceResult is the protocol-v1 document `maintenance sweep` returns. The
// per-item entries carry the real story; the envelope result reports only
// whether the sweep ran and whether it mutated anything, never a collapsed
// per-item verdict. A whole-sweep refusal (unreadable inventory, a deferred
// capability) carries a stable reason and message and no partial entries.
type MaintenanceResult struct {
	Envelope
	Entries                    []MaintenanceEntry `json:"entries"`
	Reason                     string             `json:"reason,omitempty"`
	Message                    string             `json:"message,omitempty"`
	Findings                   []StatusFinding    `json:"findings"`
	Scope                      string             `json:"scope"`
	DeferredHistoricalCleanups int                `json:"deferred_historical_cleanups"`
}

// HumanText renders a one-line summary naming the entry count and the mutated
// count only — never an authored document body.
func (r MaintenanceResult) HumanText() string {
	op := r.Operation
	if r.Scope != "" {
		op = fmt.Sprintf("%s (scope %s)", r.Operation, r.Scope)
	}
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		applied := 0
		for _, e := range r.Entries {
			if e.Disposition == SweepDispApplied {
				applied++
			}
		}
		out := fmt.Sprintf("%s: %d item(s), %d applied", op, len(r.Entries), applied)
		if r.Scope == string(SweepScopeImplementation) {
			// A count of candidates deliberately NOT probed — never a claim
			// they are dirty or blocked; explicit full maintenance owns them.
			out += fmt.Sprintf("; %d historical cleanup(s) deferred to `docket maintenance sweep --scope full`", r.DeferredHistoricalCleanups)
		}
		return out
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", op, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", op, r.Result)
}

// newMaintenanceResult stamps the envelope and normalizes nil collections so the
// arrays marshal as [] on every path.
func newMaintenanceResult(result Result, out MaintenanceResult) MaintenanceResult {
	out.Envelope = NewEnvelope(OperationMaintenanceSweep, result)
	if out.Entries == nil {
		out.Entries = []MaintenanceEntry{}
	}
	if out.Findings == nil {
		out.Findings = []StatusFinding{}
	}
	return out
}

// maintenanceRefusal builds a whole-sweep refusal carrying a stable reason and
// message with no partial entries.
func maintenanceRefusal(result Result, reason, message string) MaintenanceResult {
	return newMaintenanceResult(result, MaintenanceResult{Reason: reason, Message: message})
}

// sweepWorkItem is one unit of the pinned inventory: which change, which kind of
// verified operation it drives, and the stack depth the closeout order sorts on.
type sweepWorkItem struct {
	id    int
	kind  string
	depth int // stack-ancestor count; only meaningful for closeout items
}

// MaintenanceSweep composes the verified closeout, cleanup, and reclaim
// operations over one pinned inventory. It is the production entry point; the
// CLI wires it, and it delegates to maintenanceSweep with real operation
// closures over the live seams.
func MaintenanceSweep(ctx context.Context, deps FinalizeDeps, repoDir string, scope SweepScope) MaintenanceResult {
	// The reclaim leg needs a full workspace service (Prepare/Inspect/Publish);
	// FinalizeDeps carries only the narrower finalize workspace seam, so build the
	// reclaim workspace over the same git client the deps already hold. A build
	// failure is not fatal to the whole sweep: only reclaim items need it, and
	// each such item then fails closed with a failed entry rather than mutating.
	var wdeps WorkspaceDeps
	var wsErr error
	if deps.Planning.Client != nil {
		svc, err := workspace.NewService(deps.Planning.Client)
		if err != nil {
			wsErr = err
		} else {
			wdeps = WorkspaceDeps{Service: svc}
		}
	} else {
		wsErr = fmt.Errorf("no git client wired for the reclaim workspace")
	}

	ops := sweepOps{
		closeout: func(ctx context.Context, id int) CloseoutResult {
			// The safety-net sweep carries no authored notes.
			return FinalizeCloseout(ctx, deps, repoDir, id, CloseoutNotes{})
		},
		cleanup: func(ctx context.Context, id int) CleanupOpResult {
			return FinalizeCleanup(ctx, deps, repoDir, id)
		},
		reclaim: func(ctx context.Context, id int, version string) ChangeReclaimResult {
			if wsErr != nil {
				return newChangeReclaimResult(ResultInternalError, ChangeReclaimResult{
					ID: id, Findings: []StatusFinding{lifecycleFinding("workspace-service-unavailable", wsErr.Error())},
				})
			}
			return ChangeReclaim(ctx, deps.Planning, wdeps, repoDir, ChangeReclaimRequest{ID: id, Version: version})
		},
		probeFacts: func(ctx context.Context, snap domain.Snapshot) (map[domain.ChangeID]domain.PRFacts, []StatusFinding) {
			// One shared GitHub identity, batched exact-number reads over the whole
			// finalize population — never a probe per change.
			return sweepSelectPRFacts(ctx, deps.PRBatch, repoDir, snap)
		},
	}
	return maintenanceSweep(ctx, deps, repoDir, ops, scope)
}

// maintenanceSweep is the orchestration under test. It pins one inventory,
// derives the deterministic worklist, and processes each item — reloading fresh
// authority before every mutation and dispatching through the injected ops.
func maintenanceSweep(ctx context.Context, deps FinalizeDeps, repoDir string, ops sweepOps, scope SweepScope) MaintenanceResult {
	reader := deps.Planning.Reader

	// The resolved scope is stamped onto every envelope this sweep returns —
	// refusals and successes alike — so the caller always sees which scope ran.
	stamp := func(r MaintenanceResult, deferred int) MaintenanceResult {
		r.Scope = string(scope)
		r.DeferredHistoricalCleanups = deferred
		return r
	}
	if scope != SweepScopeFull && scope != SweepScopeImplementation {
		return stamp(maintenanceRefusal(ResultInvalidInput, ReasonSweepScopeInvalid,
			fmt.Sprintf("unknown sweep scope %q: must be full or implementation", scope)), 0)
	}

	// One pinned inventory. This read decides the worklist; every mutation below
	// reloads its own fresh authority (and the dispatched op reloads once more).
	pin, err := reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return stamp(maintenanceRefusal(result, reason, err.Error()), 0)
	}
	// Capability preflight before any external effect: a deferred capability
	// request refuses the whole sweep before it dispatches a single mutation.
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		return stamp(maintenanceRefusal(ResultUnsupportedConfig, ReasonDeferredCapRequested,
			"configuration actively requests a deferred capability docket does not ship in this version ("+
				joinPaths(blockerPaths(decision.Blockers))+"); withdraw it before any mutation"), 0)
	}
	eff := pin.Config.Effective

	inv, refusal := sweepBuildSnapshot(ctx, reader, pin, eff)
	if refusal != nil {
		return stamp(*refusal, 0)
	}

	// Read every finalize-population change's live PR facts (the same
	// non-terminal, PR-bearing population `context finalize` reads) in one batched
	// pass over a shared GitHub identity, so the domain selector bands merged PRs
	// into the merged-recovery closeout work. A failed batch is unknown facts —
	// never a clean absence — surfaced as a finding rather than silently omitted.
	var facts map[domain.ChangeID]domain.PRFacts
	var factFindings []StatusFinding
	if ops.probeFacts != nil {
		facts, factFindings = ops.probeFacts(ctx, inv.snap)
	}
	queue := domain.SelectFinalizeQueue(inv.snap, facts, finalizeBlockedMap(), nil)

	items, deferredHistorical := sweepWorklist(inv.snap, queue, eff, deps.Planning.Clock.Now(), scope)

	entries := make([]MaintenanceEntry, 0, len(items))
	for _, it := range items {
		switch it.kind {
		case sweepKindCloseout:
			entries = append(entries, sweepRunCloseout(ctx, reader, repoDir, eff, ops, it.id)...)
		case sweepKindCleanup:
			entries = append(entries, sweepRunCleanup(ctx, reader, repoDir, eff, ops, it.id))
		case sweepKindReclaim:
			entries = append(entries, sweepRunReclaim(ctx, reader, repoDir, eff, ops, it.id))
		}
	}

	// Structured, sorted entries — deterministic regardless of dispatch order.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].ID != entries[j].ID {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].Kind < entries[j].Kind
	})

	result := ResultNoOp
	for _, e := range entries {
		if e.Disposition == SweepDispApplied {
			result = ResultApplied
			break
		}
	}
	// Discovery diagnostics ride the result even when no operation was selected —
	// silent omission is not success.
	return stamp(newMaintenanceResult(result, MaintenanceResult{Entries: entries, Findings: factFindings}), deferredHistorical)
}

// sweepInventory is one authoritative read: the built snapshot plus the exact
// blob version of every change record, keyed by active path.
type sweepInventory struct {
	snap          domain.Snapshot
	versionByPath map[string]string
}

// sweepBuildSnapshot reads the corpus for pin and builds the snapshot. A read or
// build failure is a whole-sweep refusal (the inventory could not be pinned).
func sweepBuildSnapshot(ctx context.Context, reader StatusReader, pin StatusPin, eff config.Effective) (sweepInventory, *MaintenanceResult) {
	blobs, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := maintenanceRefusal(result, reason, err.Error())
		return sweepInventory{}, &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		r := maintenanceRefusal(ResultInternalError, ReasonStatusInternalError, err.Error())
		return sweepInventory{}, &r
	}
	versions := make(map[string]string, len(blobs))
	for _, b := range blobs {
		versions[b.Path] = b.Version
	}
	return sweepInventory{snap: build.Snapshot, versionByPath: versions}, nil
}

// sweepWorklist derives the deterministic ordered worklist from one pinned
// snapshot: closeout items (active implemented changes with a merged PR),
// ordered children-before-ancestors so a root can carry its descendants; then
// cleanup items (done or stacked-merged records — archived changes and completed
// stacks) by id; then reclaim items (in-progress records whose lease is strictly
// expired) by id. The three status sets are disjoint, so no id is double-listed.
func sweepWorklist(snap domain.Snapshot, queue []domain.FinalizeCandidate, eff config.Effective, now time.Time, scope SweepScope) (items []sweepWorkItem, deferredHistorical int) {
	var closeouts []sweepWorkItem
	for _, cand := range queue {
		if cand.Band != sweepBandMergedRecovery {
			continue
		}
		c, out := snap.Change(cand.ID)
		if out != domain.LookupFound || c.Status() != domain.StatusImplemented {
			continue
		}
		closeouts = append(closeouts, sweepWorkItem{
			id:    int(c.ID()),
			kind:  sweepKindCloseout,
			depth: len(domain.StackAncestors(snap, c)),
		})
	}
	// Children before ancestors: a descendant has strictly more stack ancestors
	// than any of its ancestors, so deeper-first guarantees the order. Id breaks
	// ties deterministically.
	sort.SliceStable(closeouts, func(i, j int) bool {
		if closeouts[i].depth != closeouts[j].depth {
			return closeouts[i].depth > closeouts[j].depth
		}
		return closeouts[i].id < closeouts[j].id
	})

	var cleanups, reclaims []sweepWorkItem
	ttl := eff.Reclaim.LeaseTTL.Value
	for _, c := range snap.Changes() {
		if _, out := snap.Change(c.ID()); out != domain.LookupFound {
			continue
		}
		switch {
		case c.Status() == domain.StatusDone || c.Status() == domain.StatusStackedMerged:
			// Implementation scope defers records that were already terminal at
			// the pinned inventory: they are counted, never enqueued, so the
			// worklist stays independent of the historical population. A record
			// a closeout archives DURING this invocation is untouched by this
			// filter — its cleanup rides sweepRunCloseout's suffix, not this list.
			if scope == SweepScopeImplementation {
				deferredHistorical++
				continue
			}
			cleanups = append(cleanups, sweepWorkItem{id: int(c.ID()), kind: sweepKindCleanup})
		case domain.EvaluateLease(c, now, ttl) == domain.LeaseExpired:
			reclaims = append(reclaims, sweepWorkItem{id: int(c.ID()), kind: sweepKindReclaim})
		}
	}
	sort.SliceStable(cleanups, func(i, j int) bool { return cleanups[i].id < cleanups[j].id })
	sort.SliceStable(reclaims, func(i, j int) bool { return reclaims[i].id < reclaims[j].id })

	out := make([]sweepWorkItem, 0, len(closeouts)+len(cleanups)+len(reclaims))
	out = append(out, closeouts...)
	out = append(out, cleanups...)
	out = append(out, reclaims...)
	return out, deferredHistorical
}

// sweepRunCloseout reloads fresh authority, dispatches the verified closeout,
// and — only when the closeout SUCCEEDED (applied or a verified no-op) — reloads
// again and runs the ownership-safe cleanup suffix. A closeout that is unknown,
// contended, blocked, or failed withholds the destructive suffix entirely: the
// suffix never runs after an unresolved prerequisite.
func sweepRunCloseout(ctx context.Context, reader StatusReader, repoDir string, eff config.Effective, ops sweepOps, id int) []MaintenanceEntry {
	var out []MaintenanceEntry

	present, refusal := sweepReloadPresent(ctx, reader, repoDir, eff, id)
	if refusal != nil {
		return []MaintenanceEntry{sweepEntry(id, sweepKindCloseout, SweepDispSkipped, "", refusal.reason, refusal.message)}
	}
	if !present {
		return []MaintenanceEntry{sweepEntry(id, sweepKindCloseout, SweepDispSkipped, "", ReasonSweepItemVanished, "record absent or ambiguous on reload")}
	}

	res := ops.closeout(ctx, id)
	disp := sweepDispositionForResult(res.Env().Result)
	out = append(out, MaintenanceEntry{
		ID: id, Kind: sweepKindCloseout, Disposition: disp,
		Operation: res.Env().Operation, Reason: res.Reason, Message: res.Message, CarriedIDs: res.CarriedIDs,
	})

	if disp != SweepDispApplied && disp != SweepDispNoOp {
		// The prerequisite did not resolve; withhold the destructive suffix.
		out = append(out, sweepEntry(id, sweepKindCleanup, SweepDispSkipped, "",
			ReasonSweepPrerequisiteUnresolved, "cleanup withheld: closeout was "+disp))
		return out
	}

	out = append(out, sweepRunCleanup(ctx, reader, repoDir, eff, ops, id))
	return out
}

// sweepRunCleanup reloads fresh authority and dispatches the ownership-safe
// cleanup for one terminal (or completed-stack) record.
func sweepRunCleanup(ctx context.Context, reader StatusReader, repoDir string, eff config.Effective, ops sweepOps, id int) MaintenanceEntry {
	present, refusal := sweepReloadPresent(ctx, reader, repoDir, eff, id)
	if refusal != nil {
		return sweepEntry(id, sweepKindCleanup, SweepDispSkipped, "", refusal.reason, refusal.message)
	}
	if !present {
		return sweepEntry(id, sweepKindCleanup, SweepDispSkipped, "", ReasonSweepItemVanished, "record absent or ambiguous on reload")
	}
	res := ops.cleanup(ctx, id)
	return MaintenanceEntry{
		ID: id, Kind: sweepKindCleanup, Disposition: sweepDispositionForResult(res.Env().Result),
		Operation: res.Env().Operation, Reason: res.Reason, Message: res.Message,
	}
}

// sweepRunReclaim gates the reclaim on reclaim.auto: when it is off the eligible
// record is surfaced as skipped and nothing is dispatched or reloaded; when it
// is on the sweep reloads fresh authority to pin the exact blob version and
// dispatches the verified reclaim.
func sweepRunReclaim(ctx context.Context, reader StatusReader, repoDir string, eff config.Effective, ops sweepOps, id int) MaintenanceEntry {
	if !eff.Reclaim.Auto.Value {
		return sweepEntry(id, sweepKindReclaim, SweepDispSkipped, "", ReasonSweepReclaimAutoDisabled,
			"reclaim.auto is false; run `docket change reclaim` explicitly")
	}
	version, present, refusal := sweepReloadVersion(ctx, reader, repoDir, eff, id)
	if refusal != nil {
		return sweepEntry(id, sweepKindReclaim, SweepDispSkipped, "", refusal.reason, refusal.message)
	}
	if !present {
		return sweepEntry(id, sweepKindReclaim, SweepDispSkipped, "", ReasonSweepItemVanished, "record absent or ambiguous on reload")
	}
	if version == "" {
		return sweepEntry(id, sweepKindReclaim, SweepDispSkipped, "", ReasonSweepReclaimVersionMissing, "reloaded record carried no blob version")
	}
	res := ops.reclaim(ctx, id, version)
	return MaintenanceEntry{
		ID: id, Kind: sweepKindReclaim, Disposition: sweepDispositionForResult(res.Env().Result),
		Operation: res.Env().Operation, Reason: res.Reason, Message: res.Message,
	}
}

// sweepReloadError carries a fresh-reload refusal: the record could not be read
// (reload-failed) so the sweep dispatched nothing for this item.
type sweepReloadError struct {
	reason  string
	message string
}

// sweepReloadPresent re-pins fresh authority and reports whether the change is
// present and unambiguous on the reloaded snapshot. Re-pinning is the fresh
// reload the spec requires before every mutation; it is why the item is
// re-confirmed rather than trusted from the pinned inventory.
func sweepReloadPresent(ctx context.Context, reader StatusReader, repoDir string, eff config.Effective, id int) (bool, *sweepReloadError) {
	_, present, refusal := sweepReloadVersion(ctx, reader, repoDir, eff, id)
	return present, refusal
}

// sweepReloadVersion re-pins fresh authority, rebuilds the snapshot, and returns
// the reloaded record's exact blob version. present is false when the record is
// absent or ambiguous now; a read/build failure returns a reload error.
func sweepReloadVersion(ctx context.Context, reader StatusReader, repoDir string, eff config.Effective, id int) (string, bool, *sweepReloadError) {
	fresh, err := reader.PinContext(ctx, repoDir)
	if err != nil {
		return "", false, &sweepReloadError{reason: ReasonSweepReloadFailed, message: err.Error()}
	}
	inv, refusal := sweepBuildSnapshot(ctx, reader, fresh, eff)
	if refusal != nil {
		return "", false, &sweepReloadError{reason: ReasonSweepReloadFailed, message: refusal.Message}
	}
	c, out := inv.snap.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		return "", false, nil
	}
	return inv.versionByPath[c.Path()], true, nil
}

// sweepEntry builds one MaintenanceEntry.
func sweepEntry(id int, kind, disposition, operation, reason, message string) MaintenanceEntry {
	return MaintenanceEntry{ID: id, Kind: kind, Disposition: disposition, Operation: operation, Reason: reason, Message: message}
}

// sweepDispositionForResult maps a dispatched operation's protocol-v1 result
// onto the closed sweep disposition vocabulary. external-failed is the unknown
// external-effect verdict (a probe or reachability error, never a clean
// absence); every other refusal that is not contended folds onto blocked; a
// gate/interrupt/internal failure is a hard failed.
func sweepDispositionForResult(r Result) string {
	switch r {
	case ResultApplied:
		return SweepDispApplied
	case ResultNoOp:
		return SweepDispNoOp
	case ResultContended:
		return SweepDispContended
	case ResultExternalFailed:
		return SweepDispUnknown
	case ResultBlocked, ResultInvalidState, ResultInvalidInput, ResultUnsupportedConfig:
		return SweepDispBlocked
	default: // gate-failed, interrupted, internal-error
		return SweepDispFailed
	}
}

// joinPaths joins blocker config paths for a refusal message without importing
// strings solely for one Join.
func joinPaths(paths []string) string {
	out := ""
	for i, p := range paths {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
