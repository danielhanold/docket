package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	CollectionStatusCollected  = "collected"
	CollectionStatusReferenced = "referenced"
	CollectionStatusUnverified = "unverified"
	CollectionStatusFailed     = "failed"
)

type CollectOptions struct {
	Roots  UserRoots
	FS     FSOps
	DryRun bool
}

type CollectionEntry struct {
	AssetSetID string
	Path       string
	Status     string
	Detail     string
}

type CollectionOutcome struct {
	Applied bool
	Entries []CollectionEntry
	Pending []string
	Err     error
}

// collectBeforeQuarantine is a deterministic race seam for proving that the
// final reference, containment, kind, and content checks carry deletion
// authority. Production leaves it inert.
var collectBeforeQuarantine = func(string) {}
var collectBeforeReferenceRefresh = func(string) {}

// Collect removes only version trees proven complete and proven unreachable.
// Dry-run takes the same lock when it already exists, but never creates the
// mutex or any other installation material.
func Collect(options CollectOptions) CollectionOutcome {
	if err := validateCollectOptions(options); err != nil {
		return CollectionOutcome{Err: err}
	}
	if options.DryRun {
		lock, _, err := acquireReadOnlyInstallLock(options.Roots)
		if err != nil {
			return CollectionOutcome{Err: err}
		}
		if lock == nil {
			return CollectionOutcome{}
		}
		defer lock.release()
		return collectPass(options, lock, false)
	}
	lock, err := acquireInstallLock(options.Roots)
	if err != nil {
		return CollectionOutcome{Err: err}
	}
	defer lock.release()
	return collectLocked(options, lock)
}

func validateCollectOptions(options CollectOptions) error {
	if options.FS == nil {
		return fmt.Errorf("%w: collection requires filesystem operations", ErrInvalidInput)
	}
	if options.Roots.DataRoot == "" || !filepath.IsAbs(options.Roots.DataRoot) {
		return fmt.Errorf("%w: roots carry no absolute data root", ErrInvalidInput)
	}
	return nil
}

// collectLocked is the mutating collector seam used by uninstall while it
// holds the installation-wide lock across state publication and collection.
func collectLocked(options CollectOptions, lock *installLock) CollectionOutcome {
	if !lock.held() {
		return CollectionOutcome{Err: errors.New("install: collection requires the installation lock")}
	}
	return collectPass(options, lock, true)
}

func collectPass(options CollectOptions, lock *installLock, mutate bool) CollectionOutcome {
	out := CollectionOutcome{}
	if !lock.held() {
		out.Err = errors.New("install: collection consistency probe is not held")
		return out
	}
	dataRoot, err := canonicalPath(options.Roots.DataRoot)
	if err != nil {
		out.Err = fmt.Errorf("install: canonicalising data root: %w", err)
		return out
	}
	options.Roots.DataRoot = dataRoot

	journal, err := loadCollectionJournal(options.Roots)
	if err != nil {
		out.Pending = []string{options.Roots.CollectionJournalPath()}
		out.Err = err
		return out
	}
	if journal != nil {
		if !mutate {
			out.Pending = []string{options.Roots.CollectionJournalPath()}
			out.Err = fmt.Errorf("%w: collection journal requires locked recovery", ErrCollectionPending)
			return out
		}
		if err := reconcileCollectionJournal(options.FS, options.Roots); err != nil {
			out.Pending = []string{options.Roots.CollectionJournalPath()}
			out.Err = err
			return out
		}
		out.Applied = true
	}

	txnID, found, err := DetectRecovery(options.Roots)
	if err != nil {
		out.Err = err
		return out
	}
	if found {
		out.Pending = []string{txnID}
		out.Err = fmt.Errorf("install: rollback transaction %s is pending", txnID)
		return out
	}

	state, stateErr := LoadState(options.Roots.StatePath())
	paths, versions, enumErr := collectionCandidates(options.Roots)
	if enumErr != nil {
		out.Err = enumErr
		return out
	}
	if stateErr != nil || state == nil {
		if len(paths) == 0 && stateErr == nil {
			return out
		}
		detail := "installed state is absent"
		if stateErr != nil {
			detail = stateErr.Error()
		}
		for _, path := range paths {
			out.Entries = append(out.Entries, CollectionEntry{AssetSetID: filepath.Base(path), Path: path, Status: CollectionStatusUnverified, Detail: detail})
		}
		if stateErr != nil {
			out.Err = stateErr
		} else {
			out.Err = fmt.Errorf("%w: cannot collect existing versions without installed state", ErrStateInvalid)
		}
		return sortedCollection(out)
	}

	references, err := DeriveVersionReferences(options.Roots, state)
	if err != nil {
		out.Err = err
		return out
	}
	eligible := make([]int, 0, len(paths))
	initialProofs := make(map[string]ProvenVersion, len(paths))
	for _, path := range paths {
		entry := CollectionEntry{AssetSetID: filepath.Base(path), Path: path}
		if !strictVersionChild(versions, path) {
			entry.Status = CollectionStatusUnverified
			entry.Detail = "candidate is not strictly contained by the versions root"
			out.Entries = append(out.Entries, entry)
			continue
		}
		info, err := os.Lstat(path)
		if err != nil {
			entry.Status = CollectionStatusUnverified
			entry.Detail = fmt.Sprintf("inspecting candidate: %v", err)
			out.Entries = append(out.Entries, entry)
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			entry.Status = CollectionStatusUnverified
			entry.Detail = "candidate is not a non-symlink directory"
			out.Entries = append(out.Entries, entry)
			continue
		}
		canonical, err := canonicalPath(path)
		if err != nil {
			entry.Status = CollectionStatusUnverified
			entry.Detail = fmt.Sprintf("canonicalising candidate: %v", err)
			out.Entries = append(out.Entries, entry)
			continue
		}
		entry.Path = canonical
		if !strictVersionChild(versions, entry.Path) {
			entry.Status = CollectionStatusUnverified
			entry.Detail = "candidate is not strictly contained by the versions root"
			out.Entries = append(out.Entries, entry)
			continue
		}
		if _, ok := references[path]; ok {
			entry.Status = CollectionStatusReferenced
			entry.Detail = "retained by installed state or a live target"
			out.Entries = append(out.Entries, entry)
			continue
		}
		proven, err := ProveVersionTree(path)
		if err != nil {
			entry.Status = CollectionStatusUnverified
			entry.Detail = err.Error()
			out.Entries = append(out.Entries, entry)
			continue
		}
		entry.AssetSetID = proven.Manifest.AssetSetID
		entry.Status = CollectionStatusCollected
		if !mutate {
			entry.Detail = "would collect verified unreferenced tree"
		}
		out.Entries = append(out.Entries, entry)
		eligible = append(eligible, len(out.Entries)-1)
		initialProofs[entry.Path] = proven
	}

	if !mutate {
		return sortedCollection(out)
	}
	for _, index := range eligible {
		entry := &out.Entries[index]
		collectBeforeReferenceRefresh(entry.Path)
		state, err := LoadState(options.Roots.StatePath())
		if err != nil || state == nil {
			entry.Status = CollectionStatusFailed
			if err == nil {
				err = fmt.Errorf("%w: installed state disappeared", ErrStateInvalid)
			}
			entry.Detail = err.Error()
			out.Err = err
			break
		}
		freshReferences, err := DeriveVersionReferences(options.Roots, state)
		if err != nil {
			entry.Status = CollectionStatusFailed
			entry.Detail = err.Error()
			out.Err = err
			break
		}
		if _, nowReferenced := freshReferences[entry.Path]; nowReferenced {
			entry.Status = CollectionStatusReferenced
			entry.Detail = "became referenced before quarantine"
			continue
		}
		collectBeforeQuarantine(entry.Path)
		canonical, err := canonicalPath(entry.Path)
		if err != nil {
			entry.Status = CollectionStatusFailed
			entry.Detail = err.Error()
			out.Err = err
			break
		}
		if !strictVersionChild(versions, canonical) {
			entry.Status = CollectionStatusFailed
			entry.Detail = "candidate escaped versions root before quarantine"
			out.Err = fmt.Errorf("install: refusing escaped collection candidate %s", canonical)
			break
		}
		info, err := os.Lstat(entry.Path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			if err == nil {
				err = fmt.Errorf("candidate changed kind before quarantine")
			}
			entry.Status = CollectionStatusFailed
			entry.Detail = err.Error()
			out.Err = err
			break
		}
		proven, err := ProveVersionTree(entry.Path)
		initial := initialProofs[entry.Path]
		if err != nil || proven.Manifest.AssetSetID != entry.AssetSetID || proven.Legacy != initial.Legacy {
			if err == nil {
				err = fmt.Errorf("candidate identity changed before quarantine")
			}
			entry.Status = CollectionStatusFailed
			entry.Detail = err.Error()
			out.Err = err
			break
		}
		journal := collectionJournal{
			FormatVersion:      collectionJournalFormatVersion,
			OriginalAssetSetID: proven.Manifest.AssetSetID,
			Manifest:           proven.Manifest,
			SourcePath:         entry.Path,
			QuarantinePath:     options.Roots.CollectionQuarantineDir(),
			Legacy:             proven.Legacy,
			Phase:              collectionPhasePrepared,
		}
		err = writeCollectionJournal(options.FS, options.Roots, &journal)
		if err == nil {
			err = options.FS.Rename(journal.SourcePath, journal.QuarantinePath)
			if err != nil {
				err = pending("quarantining verified source", err)
			}
		}
		if err == nil {
			journal.Phase = collectionPhaseQuarantined
			err = writeCollectionJournal(options.FS, options.Roots, &journal)
			if err != nil {
				err = pending("publishing quarantined phase", err)
			}
		}
		if err == nil {
			err = reconcileCollectionJournal(options.FS, options.Roots)
		}
		if err != nil {
			entry.Status = CollectionStatusFailed
			entry.Detail = err.Error()
			out.Pending = []string{options.Roots.CollectionJournalPath()}
			out.Err = err
			break
		}
		out.Applied = true
	}
	return sortedCollection(out)
}

func collectionCandidates(roots UserRoots) ([]string, string, error) {
	versions, err := canonicalPath(roots.VersionsDir())
	if err != nil {
		return nil, "", fmt.Errorf("install: canonicalising versions root: %w", err)
	}
	entries, err := os.ReadDir(versions)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, versions, nil
	}
	if err != nil {
		return nil, versions, fmt.Errorf("install: reading versions root %s: %w", versions, err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		// Canonicalise the container, not the child: following a child symlink
		// would erase the very kind information that makes it ineligible.
		paths = append(paths, filepath.Join(versions, entry.Name()))
	}
	sort.Strings(paths)
	return paths, versions, nil
}

func strictVersionChild(versions, candidate string) bool {
	rel, err := filepath.Rel(versions, candidate)
	return err == nil && rel != "." && rel != ".." && !filepath.IsAbs(rel) &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && filepath.Dir(rel) == "."
}

func sortedCollection(out CollectionOutcome) CollectionOutcome {
	sort.Slice(out.Entries, func(i, j int) bool { return out.Entries[i].Path < out.Entries[j].Path })
	sort.Strings(out.Pending)
	return out
}
