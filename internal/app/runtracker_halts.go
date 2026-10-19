// The per-repository halt count: how many implement-next runs in a row ended in a
// halt. A tracked loop reads it to decide whether a halt on a change it picked from
// the queue lets it move on (run-next) or stops it (run-stop) once the count reaches
// run.max_consecutive_halts. A clean build (run-complete) and the run.reset-halts
// operation bring it back to 0.
//
// WHERE: <per-repo state folder>/run-tracker-halts/, beside — never inside — the
// run-tracker root. The scanners that walk run-key directories under run-tracker/
// therefore never see it, for the same reason acquireResumeLock keeps its locks under
// run-tracker-resume/. The folder holds:
//
//	count.json              {"schema":1,"halts_in_a_row":N}
//	counted/<key>.halt.json  the saved haltCounted outcome of that key's halt
//	counted/<key>.reset.json {"schema":1}: that key's run-complete already reset the count
//	lock                     the flock file serializing every read-modify-write
//
// ONCE PER KEY: a key's halt is counted at most once and its run-complete resets at
// most once. A replayed recordHalt returns the saved outcome unchanged, so re-running
// a verdict after the count moved (a reset, other halts) never flips run-next and
// run-stop; a replayed recordReset never re-zeroes a count that has since grown.
//
// FAULTS: a count that cannot be read is an error, never read as 0, and every
// failure leaves the files as they were. The lock wait is bounded by haltLockWait, so
// a holder that never lets go turns into errHaltCountBusy instead of a hung verdict.
// Callers treat any error here as "print today's line".
//
// CRASH ORDERS: every file is replaced through a same-directory temp file and a
// rename, so a reader sees a whole old or new document. Between two writes a crash
// can still leave one done without the other, and each order is chosen so the
// leftover state stops a loop sooner, never later:
//   - recordHalt writes the count first, then the marker. A crash between them
//     re-counts that key on replay: an over-count.
//   - recordReset writes the marker first, then the zero count. A crash between them
//     leaves the count high.
package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	runTrackerHaltsDirName = "run-tracker-halts"
	haltCountFileName      = "count.json"
	haltCountedDirName     = "counted"
	haltLockFileName       = "lock"
	haltMarkerSuffix       = ".halt.json"
	resetMarkerSuffix      = ".reset.json"
	haltStoreSchema        = 1
	haltLockPoll           = 20 * time.Millisecond
)

// haltLockWait bounds how long any halt-count operation waits for the lock before it
// gives up with errHaltCountBusy. Tests shorten it (restoring it in t.Cleanup) and
// never run in parallel while they do.
var haltLockWait = 2 * time.Second

// errHaltCountBusy reports that another process held the halt-count lock for longer
// than haltLockWait.
var errHaltCountBusy = errors.New("halt count busy")

// haltCountStore is the halt-count folder of one repository.
type haltCountStore struct{ dir string }

// haltCounted is the saved outcome of one key's halt. Decision is RunDecisionNext or
// RunDecisionStop for a loop-picked halt and empty when the run's mode is unknown;
// Noted marks a run-stop at the limit, which carries the reset note.
type haltCounted struct {
	Schema      int    `json:"schema"`
	Decision    string `json:"decision,omitempty"`
	HaltsInARow int    `json:"halts_in_a_row"`
	HaltLimit   int    `json:"halt_limit,omitempty"`
	Noted       bool   `json:"noted,omitempty"`
}

// haltCountFile is the on-disk shape of count.json. Pointer fields let readHaltCount
// tell a missing field from a zero one.
type haltCountFile struct {
	Schema      *int `json:"schema"`
	HaltsInARow *int `json:"halts_in_a_row"`
}

// haltResetMarker is the on-disk shape of a counted/<key>.reset.json marker.
type haltResetMarker struct {
	Schema int `json:"schema"`
}

// haltCountStoreAt returns the halt-count store under stateDir, a per-repo state
// folder. It touches nothing on disk.
func haltCountStoreAt(stateDir string) haltCountStore {
	return haltCountStore{dir: filepath.Join(stateDir, runTrackerHaltsDirName)}
}

// openHaltCountStore resolves the halt-count store for repoDir through
// runTrackerStateDir. It resolves, never creates: nothing is written until a
// recording operation runs.
func openHaltCountStore(repoDir string) (haltCountStore, error) {
	state, err := runTrackerStateDir(repoDir)
	if err != nil {
		return haltCountStore{}, err
	}
	return haltCountStoreAt(state), nil
}

// recordHalt counts key's halt once and returns its outcome. For a loop-picked halt
// the outcome carries the decision against limit: run-next while the new count stays
// under limit, otherwise run-stop with Noted. An unknown-mode halt (loopPicked false)
// still counts, with no decision. A key already counted returns its saved outcome
// unchanged and writes nothing.
func (s haltCountStore) recordHalt(key string, loopPicked bool, limit int) (haltCounted, error) {
	if err := validateRunKey(key); err != nil {
		return haltCounted{}, err
	}
	if loopPicked && limit < 1 {
		return haltCounted{}, fmt.Errorf("halt count: a loop-picked halt needs a limit of 1 or more, got %d", limit)
	}
	unlock, err := s.lock()
	if err != nil {
		return haltCounted{}, err
	}
	defer unlock()

	markerPath := s.markerPath(key, haltMarkerSuffix)
	saved, found, err := readHaltMarker(markerPath)
	if err != nil {
		return haltCounted{}, err
	}
	if found {
		return saved, nil
	}
	n, err := s.readCount()
	if err != nil {
		return haltCounted{}, err
	}
	n++
	out := haltCounted{Schema: haltStoreSchema, HaltsInARow: n}
	if loopPicked {
		out.HaltLimit = limit
		if n < limit {
			out.Decision = RunDecisionNext
		} else {
			out.Decision = RunDecisionStop
			out.Noted = true
		}
	}
	// Count first, then the marker: a crash between them over-counts on replay.
	if err := s.writeCount(n); err != nil {
		return haltCounted{}, err
	}
	if err := writeHaltJSONAtomic(markerPath, out); err != nil {
		return haltCounted{}, err
	}
	return out, nil
}

// recordReset zeroes the count once for key's run-complete. A key that already reset
// the count is a no-op, so an old run-complete replayed never re-zeroes a count that
// has grown since.
func (s haltCountStore) recordReset(key string) error {
	if err := validateRunKey(key); err != nil {
		return err
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()

	markerPath := s.markerPath(key, resetMarkerSuffix)
	found, err := readResetMarker(markerPath)
	if err != nil {
		return err
	}
	if found {
		return nil
	}
	// Marker first, then the zero count: a crash between them leaves the count high.
	if err := writeHaltJSONAtomic(markerPath, haltResetMarker{Schema: haltStoreSchema}); err != nil {
		return err
	}
	return s.writeCount(0)
}

// resetAll sets the count to 0 and reports what it was. A count that cannot be read
// reports unreadable (previous 0) and is repaired by the same write. Markers stay, so
// keys already counted stay counted.
func (s haltCountStore) resetAll() (previous int, unreadable bool, err error) {
	unlock, err := s.lock()
	if err != nil {
		return 0, false, err
	}
	defer unlock()

	previous, rerr := s.readCount()
	if rerr != nil {
		previous, unreadable = 0, true
	}
	if err := s.writeCount(0); err != nil {
		return 0, unreadable, err
	}
	return previous, unreadable, nil
}

// count reads the current count without taking the lock (tests and diagnostics). A
// missing count is 0; an unreadable one is an error.
func (s haltCountStore) count() (int, error) {
	return s.readCount()
}

// lock takes the exclusive halt-count lock, polling a non-blocking flock until
// haltLockWait elapses. The returned func releases it.
func (s haltCountStore) lock() (func(), error) {
	if err := os.MkdirAll(filepath.Join(s.dir, haltCountedDirName), 0o700); err != nil {
		return nil, fmt.Errorf("halt count: create folder: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(s.dir, haltLockFileName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("halt count: open lock: %w", err)
	}
	deadline := time.Now().Add(haltLockWait)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EINTR) {
			f.Close()
			return nil, fmt.Errorf("halt count: lock: %w", err)
		}
		if !time.Now().Before(deadline) {
			f.Close()
			return nil, errHaltCountBusy
		}
		time.Sleep(haltLockPoll)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}

func (s haltCountStore) markerPath(key, suffix string) string {
	return filepath.Join(s.dir, haltCountedDirName, key+suffix)
}

// readCount reads count.json: missing is 0; unparseable, a wrong or missing schema,
// or a missing or negative count is an error, never 0.
func (s haltCountStore) readCount() (int, error) {
	buf, err := os.ReadFile(filepath.Join(s.dir, haltCountFileName))
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("halt count: read: %w", err)
	}
	var c haltCountFile
	if err := decodeHaltJSON(buf, &c); err != nil {
		return 0, fmt.Errorf("halt count: unreadable %s: %w", haltCountFileName, err)
	}
	if c.Schema == nil || *c.Schema != haltStoreSchema {
		return 0, fmt.Errorf("halt count: %s has an unknown schema", haltCountFileName)
	}
	if c.HaltsInARow == nil || *c.HaltsInARow < 0 {
		return 0, fmt.Errorf("halt count: %s has no valid count", haltCountFileName)
	}
	return *c.HaltsInARow, nil
}

func (s haltCountStore) writeCount(n int) error {
	schema := haltStoreSchema
	return writeHaltJSONAtomic(filepath.Join(s.dir, haltCountFileName), haltCountFile{Schema: &schema, HaltsInARow: &n})
}

// readHaltMarker reads a saved halt outcome. A missing marker is (zero, false, nil);
// an unreadable one is an error, never treated as absent.
func readHaltMarker(path string) (haltCounted, bool, error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return haltCounted{}, false, nil
	}
	if err != nil {
		return haltCounted{}, false, fmt.Errorf("halt count: read marker: %w", err)
	}
	var out haltCounted
	if err := decodeHaltJSON(buf, &out); err != nil {
		return haltCounted{}, false, fmt.Errorf("halt count: unreadable marker %s: %w", filepath.Base(path), err)
	}
	if out.Schema != haltStoreSchema {
		return haltCounted{}, false, fmt.Errorf("halt count: marker %s has an unknown schema", filepath.Base(path))
	}
	return out, true, nil
}

// readResetMarker reports whether a reset marker exists. An unreadable one is an
// error, never treated as absent.
func readResetMarker(path string) (bool, error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("halt count: read marker: %w", err)
	}
	var m haltResetMarker
	if err := decodeHaltJSON(buf, &m); err != nil {
		return false, fmt.Errorf("halt count: unreadable marker %s: %w", filepath.Base(path), err)
	}
	if m.Schema != haltStoreSchema {
		return false, fmt.Errorf("halt count: marker %s has an unknown schema", filepath.Base(path))
	}
	return true, nil
}

// decodeHaltJSON decodes exactly one JSON document into v, refusing trailing data.
func decodeHaltJSON(buf []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(buf))
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data")
	}
	return nil
}

// writeHaltJSONAtomic replaces path with v's JSON through a same-directory temp file
// and a rename, so a reader sees a whole old or new document.
func writeHaltJSONAtomic(path string, v any) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("halt count: encode: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("halt count: write: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return fmt.Errorf("halt count: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("halt count: write: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("halt count: write: %w", err)
	}
	return nil
}
