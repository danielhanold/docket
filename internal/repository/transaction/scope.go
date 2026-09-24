package transaction

import (
	"cmp"
	"encoding/json"
	"maps"
	"slices"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

// This file is the pure decision core of scoped validation (change 0449): how a
// finding is identified, which record paths it touches, and which candidate
// errors a scoped after-gate must refuse. Nothing here performs I/O. Every rule
// fails closed — an unresolvable reference, an empty scope, or an unprovable
// "unchanged" makes a finding relevant or refused, never grandfathered.

// canonicalRef is the serialized identity of one EntityRef: every field, so two
// refs share a canonical form iff they are equal.
type canonicalRef struct {
	Kind string `json:"k"`
	ID   int    `json:"i"`
	Slug string `json:"s"`
	Path string `json:"p"`
}

// canonicalFinding is the complete, deterministically ordered serialization of
// one finding that canonicalFindingKey encodes.
type canonicalFinding struct {
	Code     string         `json:"c"`
	Severity string         `json:"v"`
	Field    string         `json:"f"`
	Entity   canonicalRef   `json:"e"`
	Related  []canonicalRef `json:"r"`
	Detail   [][2]string    `json:"d"`
}

// toCanonicalRef copies r into its serialized form.
func toCanonicalRef(r domain.EntityRef) canonicalRef {
	return canonicalRef{Kind: string(r.Kind), ID: r.ID, Slug: r.Slug, Path: r.Path}
}

// compareEntityRefs is a total order over every EntityRef field, so sorting a
// Related list canonicalizes it without dropping or merging members.
func compareEntityRefs(a, b domain.EntityRef) int {
	return cmp.Or(
		cmp.Compare(a.Kind, b.Kind),
		cmp.Compare(a.ID, b.ID),
		cmp.Compare(a.Slug, b.Slug),
		cmp.Compare(a.Path, b.Path),
	)
}

// canonicalFindingKey serializes one finding completely and deterministically:
// Code, Severity, Field, Entity (all four fields), Related sorted by a total
// EntityRef key (multiplicity kept), and Detail as sorted key/value pairs. Two
// findings are "the same" iff their keys are equal. domain's compareFindings
// orders only by Code/Entity/Field and ignores Related/Detail, so it cannot
// serve as this equality test and must not be used here. JSON encoding quotes
// every string, so no value can inject a delimiter and collide two findings.
func canonicalFindingKey(f domain.Finding) string {
	related := slices.Clone(f.Related)
	slices.SortFunc(related, compareEntityRefs)
	c := canonicalFinding{
		Code:     f.Code,
		Severity: string(f.Severity),
		Field:    f.Field,
		Entity:   toCanonicalRef(f.Entity),
		Related:  make([]canonicalRef, 0, len(related)),
		Detail:   make([][2]string, 0, len(f.Detail)),
	}
	for _, r := range related {
		c.Related = append(c.Related, toCanonicalRef(r))
	}
	for _, k := range slices.Sorted(maps.Keys(f.Detail)) {
		c.Detail = append(c.Detail, [2]string{k, f.Detail[k]})
	}
	b, err := json.Marshal(c)
	if err != nil {
		// Strings, ints, and slices of them always marshal; reaching here is a
		// programmer error, never a domain outcome.
		panic("transaction: canonicalFindingKey: " + err.Error())
	}
	return string(b)
}

// resolveRef maps one reference to its record path in snap: a non-empty Path is
// itself; otherwise a change or ADR resolves by (positive) ID and a learning by
// Slug, and only an exactly-one-record lookup with a non-empty path resolves.
// Any other shape — absent, ambiguous, kind-only, non-numeric identity with no
// path — is unresolvable.
func resolveRef(r domain.EntityRef, snap domain.Snapshot) (gitcli.RepoPath, bool) {
	if r.Path != "" {
		return gitcli.RepoPath(r.Path), true
	}
	var path string
	switch r.Kind {
	case domain.EntityChange:
		if r.ID <= 0 {
			return "", false
		}
		c, out := snap.Change(domain.ChangeID(r.ID))
		if out != domain.LookupFound {
			return "", false
		}
		path = c.Path()
	case domain.EntityADR:
		if r.ID <= 0 {
			return "", false
		}
		a, out := snap.ADR(domain.ADRID(r.ID))
		if out != domain.LookupFound {
			return "", false
		}
		path = a.Path()
	case domain.EntityLearning:
		if r.Slug == "" {
			return "", false
		}
		l, out := snap.Learning(r.Slug)
		if out != domain.LookupFound {
			return "", false
		}
		path = l.Path()
	default:
		return "", false
	}
	if path == "" {
		return "", false
	}
	return gitcli.RepoPath(path), true
}

// findingPaths resolves every EntityRef a finding carries (Entity + all
// Related) to record paths in st: a non-empty Path is itself; a change ID
// resolves through st.Snapshot.Change, an ADR ID through st.Snapshot.ADR, a
// learning slug through st.Snapshot.Learning. ok=false when ANY ref fails to
// resolve — the caller must then treat the finding as relevant (fail closed).
// The returned paths are de-duplicated and sorted.
func findingPaths(f domain.Finding, st LoadedState) (paths []gitcli.RepoPath, ok bool) {
	refs := make([]domain.EntityRef, 0, 1+len(f.Related))
	refs = append(refs, f.Entity)
	refs = append(refs, f.Related...)
	for _, r := range refs {
		p, ok := resolveRef(r, st.Snapshot)
		if !ok {
			return nil, false
		}
		paths = append(paths, p)
	}
	slices.Sort(paths)
	return slices.Compact(paths), true
}

// findingRelevant reports whether an error finding touches the scope in EITHER
// state: any resolved path (Entity or Related, resolved in before AND in
// candidate where given) is a subject, or resolution fails anywhere. An empty
// scope, or no state to resolve against, makes every finding relevant — the
// absence of a subject set never means "ignore every error".
func findingRelevant(f domain.Finding, scope map[gitcli.RepoPath]bool, before, after *LoadedState) bool {
	if len(scope) == 0 {
		return true
	}
	states := make([]*LoadedState, 0, 2)
	for _, st := range []*LoadedState{before, after} {
		if st != nil {
			states = append(states, st)
		}
	}
	if len(states) == 0 {
		return true
	}
	for _, st := range states {
		paths, ok := findingPaths(f, *st)
		if !ok {
			return true
		}
		for _, p := range paths {
			if scope[p] {
				return true
			}
		}
	}
	return false
}

// errorBaseline is the before state's grandfather candidates: the multiset
// (canonical key → count) of unrelated error findings, and the union of record
// paths each key's findings resolved to.
type errorBaseline struct {
	counts map[string]int
	paths  map[string][]gitcli.RepoPath
}

// baselineErrors captures the before state's grandfather candidates: the
// multiset (key → count) of canonical keys of error-severity findings that are
// NOT relevant to scope in the before state, alongside the union of paths each
// key's findings resolved to. It is derived fresh from each attempt's base and
// never persisted.
func baselineErrors(before LoadedState, scope map[gitcli.RepoPath]bool) errorBaseline {
	base := errorBaseline{counts: make(map[string]int), paths: make(map[string][]gitcli.RepoPath)}
	for _, f := range before.Report.Findings() {
		if f.Severity != domain.SeverityError || findingRelevant(f, scope, &before, nil) {
			continue
		}
		key := canonicalFindingKey(f)
		base.counts[key]++
		paths, _ := findingPaths(f, before) // resolvable: irrelevance implies it
		merged := append(slices.Clone(base.paths[key]), paths...)
		slices.Sort(merged)
		base.paths[key] = slices.Compact(merged)
	}
	return base
}

// scopedAfterErrors decides the after gate: it returns the error findings that
// must refuse. An after-state error finding survives (is grandfathered) ONLY
// when all of: (1) not relevant to scope in either state; (2) its canonical key
// has remaining count in the baseline (each match decrements — multiset
// semantics, so a count change refuses the surplus); (3) every path it resolves
// to (in both states) has equal, non-empty Blobs ids in before and after.
// Everything else — new keys, changed Detail/Related (a different key), a
// touched path, an unresolvable ref — is returned as a refusal finding. A
// disappeared before-state error licenses nothing: only an exact key match
// against remaining baseline count can grandfather.
func scopedAfterErrors(before, after LoadedState, scope map[gitcli.RepoPath]bool) []domain.Finding {
	base := baselineErrors(before, scope)
	var refused []domain.Finding
	for _, f := range after.Report.Findings() {
		if f.Severity != domain.SeverityError {
			continue
		}
		if !grandfathered(f, base, before, after, scope) {
			refused = append(refused, f)
		}
	}
	return refused
}

// grandfathered applies scopedAfterErrors' three conditions to one after-state
// error finding, consuming one baseline count on success.
func grandfathered(f domain.Finding, base errorBaseline, before, after LoadedState, scope map[gitcli.RepoPath]bool) bool {
	if findingRelevant(f, scope, &before, &after) {
		return false
	}
	key := canonicalFindingKey(f)
	if base.counts[key] <= 0 {
		return false
	}
	beforePaths, _ := findingPaths(f, before) // resolvable: irrelevance implies it
	afterPaths, _ := findingPaths(f, after)
	paths := slices.Concat(base.paths[key], beforePaths, afterPaths)
	if len(paths) == 0 {
		return false
	}
	for _, p := range paths {
		b, a := before.Blobs[string(p)], after.Blobs[string(p)]
		if b == "" || a == "" || b != a {
			return false
		}
	}
	base.counts[key]--
	return true
}
