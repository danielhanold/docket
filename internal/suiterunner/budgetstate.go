// This file owns the screen-then-confirm budget state machine (change 0318),
// mirroring the former Bash oracle's change-0251 machinery: a
// contended parallel measurement is only a SCREENING observation, never an
// authoritative breach; a five-overrun streak arms the first serial confirmation
// and a ten-overrun recheck counter arms a periodic one; at most ONE scheduled
// confirmation runs per qualifying run; --strict re-measures every current
// candidate immediately and fails closed (exit 4). Only a solo (uncontended)
// re-measurement over ceiling*3/2 is ever authoritative.
//
// The store speaks the oracle's exact v1 format so a human reads both files with
// one set of eyes — the ONLY deliberate divergence is the default PATH: the Go
// runner writes <git-common-dir>/docket/development-test-budget-state.tsv, never
// the Bash runner's <git-dir>/docket/run-tests-budget-state.tsv. Two independent
// writers on one advisory file would corrupt both histories, so the runners keep
// separate files with the same schema (a documented intentional deviation, pinned
// by TestStorePathIsNotTheBashRunners). Every store operation is ADVISORY and
// fail-open: a missing, corrupt, unlockable, or unwritable store never fails the
// run.
package suiterunner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// bsSchema is the budget-state store schema version, embedded in the context key
// (a schema bump invalidates every record through the key itself) and in the
// header. It matches the Bash oracle's BS_SCHEMA.
const bsSchema = 1

// storeBasename is the Go runner's OWN advisory store filename — deliberately not
// the Bash oracle's run-tests-budget-state.tsv (see the file header).
const storeBasename = "development-test-budget-state.tsv"

// storeHeader is the v1 format header, byte-identical to the Bash oracle's so a
// human reads both stores with one set of eyes.
const storeHeader = "# docket-run-tests-budget-state v1"

// lockAttempts / lockInterval bound the advisory mkdir-lock retry (the oracle's
// ~3s of 100ms attempts). Package vars, not consts, so the fail-open lock tests
// can shrink them without a real 3s wait.
var (
	lockAttempts = 30
	lockInterval = 100 * time.Millisecond
)

// ScreenObs is one parallel-executed target's screening observation for a run.
// The caller (the run entrypoint) computes Key via ContextKey so budgetstate.go
// never needs the execution-context dimensions itself. Over is the contended
// screening crossing (secs*2 > ceil*5 AND the target passed) — a screening
// signal only, never an authoritative breach.
type ScreenObs struct {
	Key     string
	Path    string
	Ceiling int
	Secs    int
	Over    bool
}

// bsRecord is one per-context record, mirroring the oracle's TSV columns. lastPar,
// lastSolo, confRes, and dueSeq keep the oracle's "-" sentinel as a string so the
// store round-trips byte-for-byte.
type bsRecord struct {
	state    string // unobserved | watching | parallel-sensitive | confirmed-breach
	streak   int    // consecutive initial overruns (arms at 5)
	since    int    // overruns since the last confirmation (arms a recheck at 10)
	lastPar  string // last contended parallel seconds, or "-"
	lastSolo string // last uncontended solo seconds, or "-"
	ceiling  int
	confRes  string // "-" | failed | breached | cleared
	dueSeq   string // monotonic due stamp, or "-" when not due
	path     string
}

// budgetState is one run's view of the store: the loaded records, the monotonic
// due sequence, the current run's -j (for report lines), this run's screening
// candidates (Over==true observations, for EmitScreenReport and the strict path),
// the keys observed this run (the current-execution-context filter for
// ScheduleConfirmation), and the fail-open store bookkeeping.
type budgetState struct {
	records    map[string]*bsRecord
	nextSeq    int
	jobs       int
	candidates []ScreenObs
	seen       map[string]bool

	path   string    // resolved store path ("" => no history this run)
	warn   io.Writer // advisory warnings (may be nil)
	locked bool      // whether this run holds the advisory lock
}

// newBudgetState builds an empty state primed with the current run's -j.
func newBudgetState(jobs int) *budgetState {
	return &budgetState{
		records: make(map[string]*bsRecord),
		nextSeq: 1,
		jobs:    jobs,
		seen:    make(map[string]bool),
	}
}

// ContextKey embeds every dimension that makes two measurements incomparable, in
// the oracle's exact rendering "%s|j%d|c%d|%s|%s|b%d|m%s|s%d": a -j16 contention
// profile says nothing about -j4, and a ceiling, mode, or schema change is simply
// a different key, so the old record is neither read nor advanced.
func ContextKey(path string, jobs, cpus int, osName, arch string, ceiling int, mode Mode) string {
	return fmt.Sprintf("%s|j%d|c%d|%s|%s|b%d|m%s|s%d", path, jobs, cpus, osName, arch, ceiling, string(mode), bsSchema)
}

// DefaultStatePath resolves the Go runner's OWN advisory budget-state store:
// <git-common-dir>/docket/development-test-budget-state.tsv. It is deliberately
// NOT the Bash oracle's <git-dir>/docket/run-tests-budget-state.tsv — see the
// file header and TestStorePathIsNotTheBashRunners. A relative git-common-dir is
// anchored to repoRoot, mirroring the oracle's anchoring of a relative git dir.
func DefaultStatePath(repoRoot string) (string, error) {
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("suiterunner: resolve git-common-dir for %q: %w", repoRoot, err)
	}
	gd := strings.TrimSpace(string(out))
	if gd == "" {
		return "", fmt.Errorf("suiterunner: empty git-common-dir for %q", repoRoot)
	}
	if !filepath.IsAbs(gd) {
		gd = filepath.Join(repoRoot, gd)
	}
	return filepath.Join(gd, "docket", storeBasename), nil
}

// ApplyScreenObservations folds this run's contended parallel measurements into
// the loaded state (the oracle's apply_screen_observations), called once per
// qualifying run. It records every observation's key as seen this run (the
// current-context filter ScheduleConfirmation relies on) and collects the Over
// observations as this run's screening candidates.
//
//	overrun (Over):
//	  unobserved/watching       -> streak+1; at 5 a due stamp is minted
//	  parallel-sensitive/breach -> since+1;  at 10 a due stamp is minted
//	clean (not Over):
//	  unobserved/watching       -> streak resets to 0, dropping any due stamp
//	  parallel-sensitive/breach -> the since-counter is neither incremented NOR
//	                               reset (the asymmetry is deliberate)
func (s *budgetState) ApplyScreenObservations(obs []ScreenObs) {
	for _, o := range obs {
		s.seen[o.Key] = true
		if o.Over {
			s.candidates = append(s.candidates, o)
		}
		rec, exists := s.records[o.Key]
		if !o.Over && !exists {
			continue // a clean measurement with no prior record has no history worth a row
		}
		if !exists {
			rec = &bsRecord{state: "unobserved", lastPar: "-", lastSolo: "-", ceiling: o.Ceiling, confRes: "-", dueSeq: "-", path: o.Path}
			s.records[o.Key] = rec
		}
		if o.Over {
			switch rec.state {
			case "unobserved", "watching":
				rec.state = "watching"
				rec.streak++
				if rec.streak >= 5 && rec.dueSeq == "-" {
					rec.dueSeq = strconv.Itoa(s.nextSeq)
					s.nextSeq++
				}
			case "parallel-sensitive", "confirmed-breach":
				rec.since++
				if rec.since >= 10 && rec.dueSeq == "-" {
					rec.dueSeq = strconv.Itoa(s.nextSeq)
					s.nextSeq++
				}
			}
		} else {
			switch rec.state {
			case "unobserved", "watching":
				// Resetting the streak drops any stale due stamp with it.
				rec.streak = 0
				rec.dueSeq = "-"
				// parallel-sensitive/confirmed-breach: leave the since-counter exactly where it is.
			}
		}
		// lastPar tracks this run's contended measurement for both branches.
		rec.lastPar = strconv.Itoa(o.Secs)
		rec.ceiling = o.Ceiling
		rec.path = o.Path
	}
}

// dueItem is one confirmation-eligible record with its tie-break keys.
type dueItem struct {
	overdue int
	dueSeq  int
	path    string
	key     string
}

// dueCandidates gathers the records that are due in THIS run's execution context
// (observed this run, so their key matches the current -j/ceiling/mode/arch),
// whose file still exists, sorted by the oracle's deterministic order: largest
// overdue first, then lowest due_sequence, then C-collated path.
func (s *budgetState) dueCandidates() []dueItem {
	var due []dueItem
	for key, rec := range s.records {
		if !s.seen[key] || rec.path == "" || rec.dueSeq == "-" {
			continue
		}
		if _, err := os.Stat(rec.path); err != nil {
			continue // cannot confirm a vanished file
		}
		dseq, err := strconv.Atoi(rec.dueSeq)
		if err != nil {
			continue
		}
		var overdue int
		switch rec.state {
		case "unobserved", "watching":
			overdue = rec.streak - 5
		case "parallel-sensitive", "confirmed-breach":
			overdue = rec.since - 10
		default:
			continue
		}
		due = append(due, dueItem{overdue: overdue, dueSeq: dseq, path: rec.path, key: key})
	}
	sort.Slice(due, func(i, j int) bool {
		if due[i].overdue != due[j].overdue {
			return due[i].overdue > due[j].overdue
		}
		if due[i].dueSeq != due[j].dueSeq {
			return due[i].dueSeq < due[j].dueSeq
		}
		return due[i].path < due[j].path
	})
	return due
}

// ScheduleConfirmation is the bounded confirmation tail (the oracle's
// schedule_confirmation): a qualifying run performs AT MOST ONE scheduled solo
// confirmation. It announces SERIAL CONFIRMATION DUE for the chosen record,
// re-runs it solo, and classifies the uncontended measurement — rc!=0 FAILED
// (clears nothing, stays due); solo over ceiling*3/2 a confirmed breach (SERIAL
// CONFIRMED OVER BUDGET); else parallel-sensitive/cleared. Every OTHER due record
// prints SERIAL CONFIRMATION DEFERRED. Called between ApplyScreenObservations and
// Save so its outcome lands in the same write.
func (s *budgetState) ScheduleConfirmation(ctx context.Context, cfg Config, w io.Writer) {
	due := s.dueCandidates()
	if len(due) == 0 {
		return
	}
	chosen := due[0]
	rec := s.records[chosen.key]

	// DUE announces that a confirmation ran, whatever its outcome — and it prints
	// BEFORE the solo run, so an authoritative breach can never precede it.
	fmt.Fprintf(w, "SERIAL CONFIRMATION DUE: %s\n", chosen.path)
	rc, secs := s.soloConfirm(ctx, cfg, chosen.path, rec.ceiling)
	threshold := SoloThreshold(rec.ceiling)
	if rc != 0 {
		// A crashed confirm yields a spuriously low time, so it clears nothing,
		// resets no counter, and leaves the candidate due.
		rec.confRes = "failed"
		fmt.Fprintf(w, "SERIAL CONFIRMATION FAILED: %s\n", chosen.path)
	} else {
		rec.since = 0
		rec.lastSolo = strconv.Itoa(secs)
		rec.dueSeq = "-"
		if SoloOver(secs, rec.ceiling) {
			rec.state = "confirmed-breach"
			rec.confRes = "breached"
			fmt.Fprintf(w, "SERIAL CONFIRMED OVER BUDGET: %s — %ss under -j%d; %ds solo; solo threshold %ss\n",
				chosen.path, rec.lastPar, s.jobs, secs, threshold)
		} else {
			rec.state = "parallel-sensitive"
			rec.confRes = "cleared"
		}
	}
	for _, d := range due[1:] {
		fmt.Fprintf(w, "SERIAL CONFIRMATION DEFERRED: %s — Recheck is due; another test consumed this run's confirmation slot\n", d.path)
	}
}

// StrictConfirmCandidates is the --strict path (the oracle's
// strict_confirm_candidates): it re-measures EVERY current screening candidate
// immediately and individually, ignoring streak/recheck history and the
// one-per-run bound, reads no stored history to decide what to confirm, and fails
// closed. A solo over ceiling*3/2 is a confirmed breach and a non-zero
// confirmation cannot clear the candidate; either arms strict (exit 4). Only
// confirmation outcomes are persisted — never the screening counters — so a
// targeted strict run advances no streak.
func (s *budgetState) StrictConfirmCandidates(ctx context.Context, cfg Config, w io.Writer) (armed bool) {
	for _, o := range s.candidates {
		rec, ok := s.records[o.Key]
		if !ok {
			rec = &bsRecord{state: "watching", lastPar: "-", lastSolo: "-", ceiling: o.Ceiling, confRes: "-", dueSeq: "-", path: o.Path}
			s.records[o.Key] = rec
		}
		fmt.Fprintf(w, "SERIAL CONFIRMATION DUE: %s\n", o.Path)
		rc, secs := s.soloConfirm(ctx, cfg, o.Path, o.Ceiling)
		threshold := SoloThreshold(o.Ceiling)
		rec.lastPar = strconv.Itoa(o.Secs)
		rec.ceiling = o.Ceiling
		rec.path = o.Path
		if rc != 0 {
			rec.confRes = "failed"
			fmt.Fprintf(w, "SERIAL CONFIRMATION FAILED: %s\n", o.Path)
			armed = true
			continue
		}
		rec.lastSolo = strconv.Itoa(secs)
		if SoloOver(secs, o.Ceiling) {
			rec.state = "confirmed-breach"
			rec.confRes = "breached"
			fmt.Fprintf(w, "SERIAL CONFIRMED OVER BUDGET: %s — %ds under -j%d; %ds solo; solo threshold %ss\n",
				o.Path, o.Secs, s.jobs, secs, threshold)
			armed = true
		} else {
			rec.state = "parallel-sensitive"
			rec.confRes = "cleared"
		}
	}
	return armed
}

// EmitScreenReport prints one classification line per CURRENT screening candidate
// in C-collated path order, reading the just-updated state (the oracle's
// emit_screen_report). A parallel-sensitive/confirmed-breach record renders
// PARALLEL-SENSITIVE; everything else renders BUDGET WATCH. Neither line is ever
// authoritative — a screening crossing is never labeled OVER BUDGET.
func (s *budgetState) EmitScreenReport(w io.Writer) {
	if len(s.candidates) == 0 {
		return
	}
	ordered := make([]ScreenObs, len(s.candidates))
	copy(ordered, s.candidates)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	for _, o := range ordered {
		state, streak, since, lastSolo := "unobserved", 0, 0, "-"
		if rec := s.records[o.Key]; rec != nil {
			state, streak, since, lastSolo = rec.state, rec.streak, rec.since, rec.lastSolo
		}
		switch state {
		case "parallel-sensitive", "confirmed-breach":
			fmt.Fprintf(w, "PARALLEL-SENSITIVE: %s — %ds under -j%d; last solo measurement %ss; recheck progress %d/10\n",
				o.Path, o.Secs, s.jobs, lastSolo, since)
		default:
			fmt.Fprintf(w, "BUDGET WATCH: %s — %ds under -j%d; consecutive parallel-overrun streak %d/5\n",
				o.Path, o.Secs, s.jobs, streak)
		}
	}
}

// soloConfirm re-runs one target solo in a fresh sandbox under <work>/solo (so the
// parallel run's own stat/log records stay the sole authority), exporting
// DOCKET_RUNTESTS_SOLO=1. It returns the child's rc and the solo seconds — the
// measured wall time unless the DOCKET_RUNTESTS_TEST_DURATIONS seam injects a
// column-3 value. An infrastructure failure is treated as a failed confirmation
// (fail closed on the confirm axis), never a silent clear.
func (s *budgetState) soloConfirm(ctx context.Context, cfg Config, path string, ceiling int) (rc, secs int) {
	base := filepath.Base(path)
	t := Target{Path: path, Base: base, Ceiling: ceiling, Mode: ModeParallel}
	soloWork := filepath.Join(cfg.Work, "solo")
	// goTestConcurrency 0: a solo confirmation runs this target uncontended to
	// measure it against its SOLO ceiling, which is seeded from a raw solo
	// `bash tests/<file>.sh` run (Go defaults, DOCKET_GO_TEST_CONCURRENCY absent).
	// Exporting the parallel-lane cap here would cap the very measurement it must
	// reproduce, so the solo re-run keeps Go's defaults (change 0373).
	res, err := ExecuteTarget(ctx, cfg.Bash, t, soloWork, newProcRegistry(), []string{"DOCKET_RUNTESTS_SOLO=1"}, 0)
	if err != nil {
		return 1, 0
	}
	rc, secs = res.RC, res.Seconds
	if inj, ok := injectedSolo(cfg.DurationsPath, base); ok {
		secs = inj
	}
	return rc, secs
}

// injectedSolo reads the solo seconds (column 3) for base from the durations seam,
// mirroring the oracle's `awk -F'\t' '$1==b{print $3}'`. Missing file, missing
// row, or a non-numeric field means "use the measured time".
func injectedSolo(durationsPath, base string) (int, bool) {
	if durationsPath == "" {
		return 0, false
	}
	f, err := os.Open(durationsPath)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), "\t")
		if len(fields) >= 3 && fields[0] == base {
			n, err := strconv.Atoi(strings.TrimSpace(fields[2]))
			if err != nil {
				return 0, false
			}
			return n, true
		}
	}
	return 0, false
}

// Load acquires the advisory lock and reads the store at path, primed with the
// current run's -j. It is fail-open everywhere: an empty path, an unacquirable
// lock, a missing file, or a corrupt header all yield an empty (no-history) state
// and never an error. A held lock and a corrupt header are the two ways the run
// proceeds with no history; the caller pairs Load with Save (which writes and
// releases the lock) or Release (which releases it without writing).
func Load(path string, jobs int, warn io.Writer) *budgetState {
	s := newBudgetState(jobs)
	s.path = path
	s.warn = warn
	if path == "" {
		return s
	}
	if !s.lock() {
		if warn != nil {
			fmt.Fprintf(warn, "run-tests: budget-state lock not acquired (%s.lock) — this run reads and writes no budget history. Remove the lock dir by hand if its owner is dead.\n", path)
		}
		return s
	}
	s.locked = true
	s.readFile()
	return s
}

// lock is the bounded advisory mkdir-lock (the oracle's state_lock): lockAttempts
// tries at lockInterval each. It first ensures the store's parent dir exists; an
// unwritable parent makes the lock mkdir fail, which is simply an unacquired lock.
func (s *budgetState) lock() bool {
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	lockDir := s.path + ".lock"
	for i := 0; i < lockAttempts; i++ {
		if err := os.Mkdir(lockDir, 0o755); err == nil {
			return true
		}
		time.Sleep(lockInterval)
	}
	return false
}

// readFile parses the store fail-open: a missing file or a wrong/corrupt header
// discards all state; malformed rows are skipped with at most one warning.
func (s *budgetState) readFile() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return // missing => empty state
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != storeHeader {
		return // wrong/corrupt header => discard state
	}
	if len(lines) > 1 {
		if n, ok := parseNextSeq(lines[1]); ok {
			s.nextSeq = n
		}
	}
	reported := false
	for _, line := range lines[2:] {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 10 || f[0] == "" || f[1] == "" || f[9] == "" {
			if !reported && s.warn != nil {
				fmt.Fprintf(s.warn, "run-tests: malformed budget-state record ignored in %s\n", s.path)
			}
			reported = true
			continue
		}
		streak, _ := strconv.Atoi(f[2])
		since, _ := strconv.Atoi(f[3])
		ceil, _ := strconv.Atoi(f[6])
		s.records[f[0]] = &bsRecord{
			state: f[1], streak: streak, since: since, lastPar: f[4], lastSolo: f[5],
			ceiling: ceil, confRes: f[7], dueSeq: f[8], path: f[9],
		}
	}
}

// parseNextSeq reads "# next_due_sequence N" from the store's second line.
func parseNextSeq(line string) (int, bool) {
	const p = "# next_due_sequence "
	if !strings.HasPrefix(line, p) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(line[len(p):]))
	if err != nil {
		return 0, false
	}
	return n, true
}

// Save writes the full store fail-open (temp-beside + atomic rename, chmod 0600)
// and releases the advisory lock. A run that holds no lock — an empty path or an
// unacquired lock — writes nothing (so it never clobbers a store it could not
// read), and any write failure is swallowed: the store is advisory.
func (s *budgetState) Save() {
	if !s.locked {
		return // no lock held: this run neither read nor writes history
	}
	defer s.Release()
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".development-test-budget-state.*")
	if err != nil {
		return
	}
	tmpName := tmp.Name()

	var b strings.Builder
	b.WriteString(storeHeader + "\n")
	fmt.Fprintf(&b, "# next_due_sequence %d\n", s.nextSeq)
	keys := make([]string, 0, len(s.records))
	for k := range s.records {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r := s.records[k]
		fmt.Fprintf(&b, "%s\t%s\t%d\t%d\t%s\t%s\t%d\t%s\t%s\t%s\n",
			k, r.state, r.streak, r.since, dash(r.lastPar), dash(r.lastSolo), r.ceiling, dash(r.confRes), dash(r.dueSeq), r.path)
	}

	if _, err := tmp.WriteString(b.String()); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
	}
}

// Release drops the advisory lock without writing, for an early exit that took no
// confirmation action.
func (s *budgetState) Release() {
	if s.locked {
		os.Remove(s.path + ".lock")
		s.locked = false
	}
}

// dash renders an empty field as the store's "-" sentinel.
func dash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
