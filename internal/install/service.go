package install

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/config"
)

// The operation services are where the pieces below them become the three
// things a user can ask for: install, check, and development install. Each one
// is a single function over values — no globals, no ambient filesystem, no
// configuration loading — so the CLI layer above can compose them and a test
// can run a whole installation inside a temporary directory.
//
// Two invariants hold across all three:
//
//   - Nothing is written until every target has been classified. A conflict
//     anywhere refuses the whole operation, because a half-installed harness is
//     worse than an uninstalled one and the user's own files are what a partial
//     write would be gambling with.
//   - Check writes nothing at all. It shares the planning path with Install so
//     that what it reports is what Install would do, and it stops before the
//     transaction rather than reimplementing the comparison.

// Planner is the installer's view of one harness: detect it, and render the
// targets an installation would own.
//
// It is a closure seam rather than an interface because internal/harness
// imports this package — an installer holding harness.Adapter would close an
// import cycle. internal/app adapts each adapter into one of these, closing
// over the resolved configuration the adapter reads. The catalog arrives per
// call rather than in the closure because a development install renders from
// the contributor's checkout, not from the bytes this binary shipped with.
type Planner struct {
	Name   string
	Detect func(UserRoots) (present bool, root string)
	Plan   func(mode Mode, assetsDir string, catalog assets.Catalog) ([]Target, error)
}

// Options is everything a release install or a check reads.
type Options struct {
	Roots     UserRoots
	Planners  []Planner // in the caller's fixed harness order
	Harnesses []string  // explicit selection; empty means detect
	Catalog   assets.Catalog
	Config    *config.Snapshot // built-in ⊕ global only; no repository layer
	Info      buildinfo.Info
	FS        FSOps
	// AgentDigest identifies the resolved agent settings the plan was rendered
	// from. The caller computes it, because the caller owns the table.
	AgentDigest string
}

// Action is one thing an operation did, or would have to do.
type Action struct {
	Op     string `json:"op"`
	Path   string `json:"path"`
	Detail string `json:"detail,omitempty"`
}

// The operation vocabulary. It is closed: the app layer renders these, so a new
// verb is a protocol change rather than a wording choice.
const (
	OpCreate   = "create"
	OpUpdate   = "update"
	OpRemove   = "remove"
	OpKeep     = "keep"
	OpConflict = "conflict"
	OpDrift    = "drift"
	OpRecover  = "recover"
	OpState    = "state"
)

// The stable machine reasons. Everything above the service layer keys on these
// strings, never on error text.
const (
	ReasonNoHarnessDetected           = "no-harness-detected"
	ReasonInstallationRequired        = "installation-required"
	ReasonInstallationDrift           = "installation-drift"
	ReasonTransactionRecoveryRequired = "transaction-recovery-required"
	ReasonAssetManifestInvalid        = "asset-manifest-invalid"
	ReasonAssetProtocolMismatch       = "asset-protocol-mismatch"
	ReasonSourceAssetsDrifted         = "source-assets-drifted"
	ReasonDeferredCapability          = config.CodeDeferredCapRequested

	// Reasons this change adds beyond the spec's list, for states the spec's
	// vocabulary does not name.
	ReasonUnknownHarness    = "unknown-harness"
	ReasonInvalidOptions    = "invalid-options"
	ReasonInvalidSourceRoot = "invalid-source-root"
	ReasonBuildFailed       = "build-failed"
	ReasonStateInvalid      = "installed-state-invalid"
	ReasonFilesystemFailed  = "filesystem-failed"
	ReasonInternal          = "internal-error"
)

var (
	// ErrInvalidInput is every caller-side defect: an unknown harness name, a
	// source root that is not a checkout, options missing a seam.
	ErrInvalidInput = errors.New("install: invalid input")
	// ErrNoHarness is detection finding nothing to install into.
	ErrNoHarness = errors.New("install: no supported harness detected")
	// ErrNotInstalled is a check finding no installation record.
	ErrNotInstalled = errors.New("install: no installation record")
	// ErrDrifted is a check finding the installation no longer matches what
	// this binary and configuration would produce.
	ErrDrifted = errors.New("install: installation has drifted")
	// ErrBuildFailed wraps a failure of the injected Go toolchain runner.
	ErrBuildFailed = errors.New("install: building the development binary failed")
)

// Outcome is one operation's complete result. Err is set on every failure and
// Reason carries the machine-readable half; the app layer classifies from both.
type Outcome struct {
	Applied       bool
	Mode          Mode
	Harnesses     []string
	AssetProtocol int
	AssetSetID    string
	StatePath     string
	Actions       []Action
	Reason        string
	Err           error
}

func fail(out Outcome, reason string, err error) Outcome {
	// Applied is deliberately preserved: a recovery may already have changed
	// the world before the step that failed, and reporting otherwise would
	// describe a filesystem that does not exist.
	out.Reason = reason
	out.Err = err
	return out
}

// Install brings a release installation to what this binary's bundle and the
// resolved configuration describe.
func Install(o Options) Outcome {
	out := Outcome{
		Mode:          ModeRelease,
		StatePath:     o.Roots.StatePath(),
		AssetProtocol: o.Catalog.Manifest.AssetProtocol,
		AssetSetID:    o.Catalog.Manifest.AssetSetID,
	}
	if err := requireOptions(o); err != nil {
		return fail(out, ReasonInvalidOptions, err)
	}
	if decision := config.PreflightMutation(o.Config); !decision.Allowed {
		return fail(out, ReasonDeferredCapability, fmt.Errorf("%w: %d blocker(s), first: %s",
			config.ErrUnsupportedConfig, len(decision.Blockers), decision.Blockers[0].Path))
	}
	if err := validateCatalog(o.Catalog); err != nil {
		return fail(out, ReasonAssetManifestInvalid, err)
	}
	if o.Catalog.Manifest.AssetProtocol != assets.AssetProtocol {
		return fail(out, ReasonAssetProtocolMismatch, fmt.Errorf(
			"%w: the bundle declares asset protocol %d, this binary speaks %d",
			assets.ErrManifestInvalid, o.Catalog.Manifest.AssetProtocol, assets.AssetProtocol))
	}

	selected, err := selectPlanners(o.Planners, o.Harnesses, o.Roots)
	if err != nil {
		return fail(out, selectionReason(err), err)
	}
	out.Harnesses = plannerNames(selected)

	out, err = recoverPending(o, out)
	if err != nil {
		return fail(out, ReasonTransactionRecoveryRequired, err)
	}

	// The version tree is extracted before planning because the plan's links
	// point into it; extraction is idempotent and outside the user's harness
	// directories, so an operation that later refuses has still touched nothing
	// a harness reads.
	assetsDir, _, err := EnsureVersionTree(o.Roots, o.Catalog.Manifest, o.Catalog.Bytes)
	if err != nil {
		return fail(out, versionTreeReason(err), err)
	}

	targets, owner, err := planTargets(selected, ModeRelease, assetsDir, o.Catalog)
	if err != nil {
		return fail(out, ReasonInternal, err)
	}

	return applyPlan(o, plannedInstallation{
		mode:          ModeRelease,
		harnesses:     out.Harnesses,
		targets:       targets,
		owner:         owner,
		assetSetID:    o.Catalog.Manifest.AssetSetID,
		assetProtocol: o.Catalog.Manifest.AssetProtocol,
	}, out)
}

// Check reports whether the installation on disk is still the one this binary
// and configuration describe. It writes nothing: it never opens a transaction,
// never extracts a version tree, and never publishes state.
func Check(o Options) Outcome {
	out := Outcome{
		Mode:          ModeRelease,
		StatePath:     o.Roots.StatePath(),
		AssetProtocol: o.Catalog.Manifest.AssetProtocol,
		AssetSetID:    o.Catalog.Manifest.AssetSetID,
	}
	if len(o.Planners) == 0 {
		return fail(out, ReasonInvalidOptions, fmt.Errorf("%w: no planners", ErrInvalidInput))
	}
	if err := validateCatalog(o.Catalog); err != nil {
		return fail(out, ReasonAssetManifestInvalid, err)
	}

	// A journal nobody published means the world is mid-mutation; every other
	// answer would be read from a filesystem that is not yet settled.
	txnID, pending, err := DetectRecovery(o.Roots)
	if err != nil {
		return fail(out, ReasonTransactionRecoveryRequired, err)
	}
	if pending {
		out.Actions = append(out.Actions, Action{
			Op:     OpRecover,
			Path:   filepath.Join(o.Roots.TransactionsDir(), txnID),
			Detail: "an interrupted transaction is waiting to be rolled back",
		})
		return fail(out, ReasonTransactionRecoveryRequired,
			fmt.Errorf("%w: transaction %s was never published", ErrJournalInvalid, txnID))
	}

	state, err := LoadState(o.Roots.StatePath())
	if err != nil {
		return fail(out, ReasonStateInvalid, err)
	}
	if state == nil {
		return fail(out, ReasonInstallationRequired, ErrNotInstalled)
	}
	out.Mode = state.Mode
	out.AssetSetID = state.AssetSetID
	out.AssetProtocol = state.AssetProtocol
	out.Harnesses = append([]string(nil), state.Harnesses...)

	if state.AssetProtocol != assets.AssetProtocol {
		return fail(out, ReasonAssetProtocolMismatch, fmt.Errorf(
			"%w: the installation speaks asset protocol %d, this binary speaks %d",
			ErrDrifted, state.AssetProtocol, assets.AssetProtocol))
	}

	catalog := o.Catalog
	assetsDir := ""
	switch state.Mode {
	case ModeDevelopment:
		sourceCatalog, digest, err := generateSourceCatalog(state.SourceRoot)
		if err != nil {
			return fail(out, ReasonInstallationDrift, err)
		}
		if digest != state.SourceDigest {
			return fail(out, ReasonSourceAssetsDrifted, fmt.Errorf(
				"%w: %s now digests %s, the installation recorded %s",
				ErrDrifted, state.SourceRoot, digest, state.SourceDigest))
		}
		catalog = sourceCatalog
		assetsDir = state.SourceRoot
	default:
		if state.AssetSetID != catalog.Manifest.AssetSetID {
			return fail(out, ReasonInstallationDrift, fmt.Errorf(
				"%w: the installation carries asset set %s, this binary carries %s",
				ErrDrifted, state.AssetSetID, catalog.Manifest.AssetSetID))
		}
		assetsDir = o.Roots.VersionDir(state.AssetSetID)
		if err := verifyVersionTree(assetsDir, catalog.Manifest); err != nil {
			return fail(out, ReasonInstallationDrift, err)
		}
	}

	// A check verifies what is installed, which is what the state records —
	// not whatever this invocation's flags happen to select.
	selected, missing := plannersFor(o.Planners, state.Harnesses)
	var drift []Action
	for _, name := range missing {
		drift = append(drift, Action{Op: OpDrift, Path: o.Roots.StatePath(),
			Detail: "the installation records harness " + name + ", which this binary cannot plan"})
	}

	targets, _, err := planTargets(selected, state.Mode, assetsDir, catalog)
	if err != nil {
		return fail(out, ReasonInternal, err)
	}
	// Drift is a statement about the filesystem: a target that is not already
	// what this binary and configuration would write. The record's own identity
	// fields — the product version it was written by, the agent digest — are
	// deliberately not compared, because a rebuilt binary that would install
	// byte-identical material has not drifted.
	for _, t := range targets {
		inspection, err := InspectTarget(t, state, nil)
		if err != nil {
			return fail(out, ReasonFilesystemFailed, err)
		}
		if inspection.Disposition == DispositionNoop {
			continue
		}
		detail := string(inspection.Disposition)
		if inspection.Reason != "" {
			// A conflict seen from `check` is the same dead end it is from
			// `install`, so it is reported with the same remedy rather than
			// with the bare reason.
			detail = inspection.ConflictDetail()
		}
		drift = append(drift, Action{Op: OpDrift, Path: t.Path, Detail: detail})
	}

	prunes, err := PruneCandidates(scopedTo(state, plannerNames(selected)), targets)
	if err != nil {
		return fail(out, ReasonStateInvalid, err)
	}
	for _, prune := range prunes {
		if prune.Record.Kind == KindManagedBlock {
			continue // retained by design; see applyPlan
		}
		drift = append(drift, Action{Op: OpDrift, Path: prune.Record.Path, Detail: "recorded but no longer planned"})
	}

	if len(drift) > 0 {
		out.Actions = append(out.Actions, drift...)
		return fail(out, ReasonInstallationDrift, fmt.Errorf("%w: %d difference(s)", ErrDrifted, len(drift)))
	}
	return out
}

// plannedInstallation is what an operation decided to install, before anything
// on disk has been consulted about it.
type plannedInstallation struct {
	mode          Mode
	harnesses     []string
	targets       []Target
	owner         map[string]string // cleaned target path -> harness name
	assetSetID    string
	assetProtocol int
	sourceRoot    string
	sourceDigest  string
}

// applyPlan is the shared tail of Install and DevelopmentInstall: classify,
// refuse or apply, publish. Everything before it differs by mode; nothing after
// it does.
func applyPlan(o Options, p plannedInstallation, out Outcome) Outcome {
	out.Mode = p.mode
	out.AssetSetID = p.assetSetID
	out.AssetProtocol = p.assetProtocol
	out.Harnesses = p.harnesses

	prior, err := LoadState(o.Roots.StatePath())
	if err != nil {
		// Corruption is not "not installed": overwriting here would adopt
		// targets nothing can prove docket owns.
		return fail(out, ReasonStateInvalid, err)
	}

	inspections := make([]Inspection, 0, len(p.targets))
	var conflicts []Inspection
	for _, t := range p.targets {
		inspection, err := InspectTarget(t, prior, nil)
		if err != nil {
			return fail(out, ReasonFilesystemFailed, err)
		}
		if inspection.Disposition == DispositionConflict {
			conflicts = append(conflicts, inspection)
		}
		inspections = append(inspections, inspection)
	}
	if len(conflicts) > 0 {
		for _, c := range conflicts {
			// The detail carries the remedy as well as the reason: there is no
			// --force, so this report is the whole of what the user has to act
			// on.
			out.Actions = append(out.Actions, Action{Op: OpConflict, Path: c.Target.Path, Detail: c.ConflictDetail()})
		}
		return fail(out, conflictReason(conflicts), fmt.Errorf(
			"%w: %d target(s) are not provably docket's", ErrPlanConflict, len(conflicts)))
	}

	prunes, err := PruneCandidates(scopedTo(prior, p.harnesses), p.targets)
	if err != nil {
		return fail(out, ReasonStateInvalid, err)
	}
	var (
		removals  []TargetRecord
		retained  []TargetRecord
		keptNotes []Action
		blocked   []Prune
	)
	for _, prune := range prunes {
		switch {
		case prune.Record.Kind == KindManagedBlock:
			// The file around the block is the user's. Retiring one block by
			// deleting its file would take content docket was never given, so
			// the record is carried forward and the file is left alone.
			retained = append(retained, prune.Record)
			keptNotes = append(keptNotes, Action{Op: OpKeep, Path: prune.Record.Path,
				Detail: "managed block retired from the plan; the file is the user's and is left in place"})
		case !prune.Removable:
			blocked = append(blocked, prune)
		default:
			removals = append(removals, prune.Record)
		}
	}
	if len(blocked) > 0 {
		for _, b := range blocked {
			out.Actions = append(out.Actions, Action{Op: OpDrift, Path: b.Record.Path,
				Detail: "recorded target no longer matches what docket wrote"})
		}
		return fail(out, ReasonInstallationDrift, fmt.Errorf(
			"%w: %d recorded target(s) have drifted and will not be removed", ErrDrifted, len(blocked)))
	}

	desired, err := desiredState(o, p, prior, retained)
	if err != nil {
		return fail(out, ReasonInternal, err)
	}

	steps := 0
	for _, inspection := range inspections {
		if inspection.Disposition != DispositionNoop {
			steps++
		}
	}
	settled, err := stateSettled(prior, desired)
	if err != nil {
		return fail(out, ReasonInternal, err)
	}
	if steps == 0 && len(removals) == 0 && settled {
		out.Actions = append(out.Actions, keptNotes...)
		return out
	}

	txn, err := BeginTxnWithRemovals(o.FS, o.Roots, inspections, removals)
	if err != nil {
		return fail(out, transactionReason(err), err)
	}
	if err := txn.Apply(); err != nil {
		return fail(out, ReasonFilesystemFailed, err)
	}
	if err := txn.Commit(o.Roots.StatePath(), desired); err != nil {
		return fail(out, ReasonFilesystemFailed, err)
	}

	out.Applied = true
	for _, inspection := range inspections {
		switch inspection.Disposition {
		case DispositionCreate:
			out.Actions = append(out.Actions, Action{Op: OpCreate, Path: inspection.Target.Path,
				Detail: p.owner[filepath.Clean(inspection.Target.Path)]})
		case DispositionUpdate:
			out.Actions = append(out.Actions, Action{Op: OpUpdate, Path: inspection.Target.Path,
				Detail: p.owner[filepath.Clean(inspection.Target.Path)]})
		}
	}
	for _, rec := range removals {
		out.Actions = append(out.Actions, Action{Op: OpRemove, Path: rec.Path, Detail: rec.Harness})
	}
	out.Actions = append(out.Actions, keptNotes...)
	if steps == 0 && len(removals) == 0 {
		out.Actions = append(out.Actions, Action{Op: OpState, Path: o.Roots.StatePath(),
			Detail: "installation record refreshed"})
	}
	return out
}

// desiredState is the record this operation would publish: one entry per
// planned target, plus every record belonging to a harness this run did not
// plan for.
func desiredState(o Options, p plannedInstallation, prior *State, retained []TargetRecord) (*State, error) {
	records := make([]TargetRecord, 0, len(p.targets))
	claimed := make(map[string]bool, len(p.targets))
	for _, t := range p.targets {
		rec, err := RecordFor(t)
		if err != nil {
			return nil, err
		}
		rec.Harness = p.owner[filepath.Clean(t.Path)]
		records = append(records, rec)
		claimed[rec.Path] = true
	}

	selected := make(map[string]bool, len(p.harnesses))
	for _, name := range p.harnesses {
		selected[name] = true
	}
	carry := func(rec TargetRecord) {
		path := filepath.Clean(rec.Path)
		if claimed[path] {
			return
		}
		claimed[path] = true
		rec.Path = path
		records = append(records, rec)
	}
	if prior != nil {
		for _, rec := range prior.Targets {
			if !selected[rec.Harness] {
				carry(rec)
			}
		}
	}
	for _, rec := range retained {
		carry(rec)
	}

	harnesses := map[string]bool{}
	for _, name := range p.harnesses {
		harnesses[name] = true
	}
	for _, rec := range records {
		if rec.Harness != "" {
			harnesses[rec.Harness] = true
		}
	}
	names := make([]string, 0, len(harnesses))
	for name := range harnesses {
		names = append(names, name)
	}
	sort.Strings(names)

	return &State{
		FormatVersion:  StateFormatVersion,
		ProductVersion: o.Info.Version,
		AssetProtocol:  p.assetProtocol,
		AssetSetID:     p.assetSetID,
		Mode:           p.mode,
		SourceRoot:     p.sourceRoot,
		SourceDigest:   p.sourceDigest,
		Harnesses:      names,
		AgentDigest:    o.AgentDigest,
		Targets:        records,
	}, nil
}

// stateSettled reports whether the published record already says exactly what
// this operation would publish. Both sides are compared through the canonical
// encoding, so field order and slice order cannot make an unchanged
// installation look changed.
func stateSettled(prior, desired *State) (bool, error) {
	if prior == nil {
		return false, nil
	}
	was, err := encodeState(prior)
	if err != nil {
		return false, err
	}
	now, err := encodeState(desired)
	if err != nil {
		return false, err
	}
	return string(was) == string(now), nil
}

// scopedTo narrows a prior state to the harnesses this run planned for. Prune
// decisions read the narrowed view — a run that installs one harness must never
// sweep another's targets — while ownership proofs read the full state, because
// what docket wrote is what docket wrote whoever asked for it.
func scopedTo(prior *State, harnesses []string) *State {
	if prior == nil {
		return nil
	}
	selected := make(map[string]bool, len(harnesses))
	for _, name := range harnesses {
		selected[name] = true
	}
	scoped := *prior
	scoped.Targets = nil
	for _, rec := range prior.Targets {
		if rec.Harness != "" && selected[rec.Harness] {
			scoped.Targets = append(scoped.Targets, rec)
		}
	}
	return &scoped
}

// recoverPending rolls back an interrupted transaction before planning. It is
// the one mutation that happens before the plan is known, and it only ever
// restores what a previous run had already recorded.
func recoverPending(o Options, out Outcome) (Outcome, error) {
	txnID, found, err := DetectRecovery(o.Roots)
	if err != nil || !found {
		return out, err
	}
	if err := Recover(o.FS, o.Roots, txnID); err != nil {
		return out, err
	}
	out.Actions = append(out.Actions, Action{
		Op:     OpRecover,
		Path:   filepath.Join(o.Roots.TransactionsDir(), txnID),
		Detail: "rolled back an interrupted transaction",
	})
	out.Applied = true
	return out, nil
}

// planTargets renders every selected harness's targets in the caller's fixed
// order and records which harness produced each one.
func planTargets(selected []Planner, mode Mode, assetsDir string, catalog assets.Catalog) ([]Target, map[string]string, error) {
	var targets []Target
	owner := map[string]string{}
	for _, p := range selected {
		rendered, err := p.Plan(mode, assetsDir, catalog)
		if err != nil {
			return nil, nil, fmt.Errorf("install: planning %s: %w", p.Name, err)
		}
		for _, t := range rendered {
			path := filepath.Clean(t.Path)
			if prior, taken := owner[path]; taken && prior != p.Name {
				return nil, nil, fmt.Errorf("%w: %s is claimed by both %s and %s",
					ErrInvalidTarget, path, prior, p.Name)
			}
			owner[path] = p.Name
			targets = append(targets, t)
		}
	}
	return targets, owner, nil
}

// selectPlanners resolves the harnesses this run acts on: the explicit names
// when the caller gave any, else whatever detection finds. Explicit selection
// never consults detection — installing into a harness whose directory does not
// exist yet is exactly what an explicit flag is for.
func selectPlanners(planners []Planner, explicit []string, roots UserRoots) ([]Planner, error) {
	known := make(map[string]bool, len(planners))
	for _, p := range planners {
		if p.Name == "" || p.Plan == nil {
			return nil, fmt.Errorf("%w: a planner is missing its name or its plan function", ErrInvalidInput)
		}
		if known[p.Name] {
			return nil, fmt.Errorf("%w: two planners are named %q", ErrInvalidInput, p.Name)
		}
		known[p.Name] = true
	}

	if len(explicit) > 0 {
		requested := make(map[string]bool, len(explicit))
		for _, name := range explicit {
			if !known[name] {
				return nil, fmt.Errorf("%w: unknown harness %q", ErrInvalidInput, name)
			}
			requested[name] = true
		}
		selected, _ := plannersFor(planners, keysOf(requested))
		return selected, nil
	}

	var selected []Planner
	for _, p := range planners {
		if p.Detect == nil {
			continue
		}
		if present, _ := p.Detect(roots); present {
			selected = append(selected, p)
		}
	}
	if len(selected) == 0 {
		return nil, ErrNoHarness
	}
	return selected, nil
}

// plannersFor returns the named planners in the caller's fixed order, plus the
// names it has no planner for.
func plannersFor(planners []Planner, names []string) ([]Planner, []string) {
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}
	var selected []Planner
	for _, p := range planners {
		if wanted[p.Name] {
			selected = append(selected, p)
			delete(wanted, p.Name)
		}
	}
	return selected, keysOf(wanted)
}

func keysOf(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func plannerNames(planners []Planner) []string {
	out := make([]string, 0, len(planners))
	for _, p := range planners {
		out = append(out, p.Name)
	}
	return out
}

// requireOptions refuses an operation whose seams are missing, before it can
// half-run.
func requireOptions(o Options) error {
	if o.FS == nil {
		return fmt.Errorf("%w: no filesystem", ErrInvalidInput)
	}
	if len(o.Planners) == 0 {
		return fmt.Errorf("%w: no planners", ErrInvalidInput)
	}
	if o.Roots.Home == "" || !filepath.IsAbs(o.Roots.Home) {
		return fmt.Errorf("%w: roots carry no absolute home", ErrInvalidInput)
	}
	return nil
}

// validateCatalog proves the bundle describes itself consistently before any of
// its paths steer a write.
func validateCatalog(c assets.Catalog) error {
	if err := assets.ValidateManifest(c.Manifest); err != nil {
		return err
	}
	if len(c.Manifest.Entries) > 0 {
		// A manifest without a working payload accessor would fail much later,
		// halfway through extracting a version tree.
		if _, err := c.Bytes(c.Manifest.Entries[0].Path); err != nil {
			return err
		}
	}
	return nil
}

// conflictReason picks the reason a conflicted plan reports. A file docket
// cannot even parse is the more urgent diagnosis, so it wins over a plain
// ownership conflict however the plan was ordered.
func conflictReason(conflicts []Inspection) string {
	for _, c := range conflicts {
		if c.Reason == ReasonManagedBlockInvalid {
			return ReasonManagedBlockInvalid
		}
	}
	return ReasonOwnershipConflict
}

func selectionReason(err error) string {
	switch {
	case errors.Is(err, ErrNoHarness):
		return ReasonNoHarnessDetected
	case errors.Is(err, ErrInvalidInput):
		return ReasonUnknownHarness
	default:
		return ReasonInternal
	}
}

func versionTreeReason(err error) string {
	switch {
	case errors.Is(err, ErrVersionTreeInvalid):
		return ReasonInstallationDrift
	case errors.Is(err, assets.ErrManifestInvalid):
		return ReasonAssetManifestInvalid
	default:
		return ReasonFilesystemFailed
	}
}

func transactionReason(err error) string {
	switch {
	case errors.Is(err, ErrPlanConflict):
		return ReasonOwnershipConflict
	case errors.Is(err, ErrInvalidTarget):
		return ReasonInternal
	default:
		return ReasonFilesystemFailed
	}
}
